// Package engine ties the pieces together: it owns the server key, brings the
// interface up, keeps the data plane in step with the database, reads
// counters, and answers the questions the API asks.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"github.com/Coffey-Labs/WGX/internal/config"
	"github.com/Coffey-Labs/WGX/internal/netcfg"
	"github.com/Coffey-Labs/WGX/internal/store"
	"github.com/Coffey-Labs/WGX/internal/wg"
)

const (
	serverKeySetting = "server_private_key"
	bucketSize       = 5 * time.Minute
	flushEvery       = 30 * time.Second
	houseEvery       = 30 * time.Second
)

// Engine is the long-running core.
type Engine struct {
	cfg *config.Config
	st  *store.Store
	be  wg.Backend
	log *slog.Logger

	mu        sync.RWMutex
	settings  Settings
	serverKey wg.Key
	startedAt time.Time
	sysctls   []netcfg.Result
	egress    string
	fwErr     string
	peers     map[string]*store.Peer // by id
	byKey     map[string]*store.Peer // by public key

	col *collector
	hub *Hub

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New wires an engine up without starting anything.
func New(cfg *config.Config, st *store.Store, be wg.Backend, log *slog.Logger) *Engine {
	e := &Engine{cfg: cfg, st: st, be: be, log: log, peers: map[string]*store.Peer{}, byKey: map[string]*store.Peer{}, hub: NewHub()}
	e.col = newCollector(e)
	return e
}

// Store exposes the database to the HTTP layer for users, sessions and audit.
func (e *Engine) Store() *store.Store { return e.st }

// Config exposes the process configuration.
func (e *Engine) Config() *config.Config { return e.cfg }

// Hub is the live-update fan-out.
func (e *Engine) Hub() *Hub { return e.hub }

// Backend names the data plane in use.
func (e *Engine) Backend() string { return e.be.Kind() }

// ServerPublicKey is what clients put in their [Peer] section.
func (e *Engine) ServerPublicKey() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.serverKey.PublicKey().String()
}

// Settings returns a copy of the current settings.
func (e *Engine) Settings() Settings {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.settings
}

// ServerAddresses are the interface's own tunnel addresses.
func (e *Engine) ServerAddresses() []netip.Prefix {
	var out []netip.Prefix
	out = append(out, netip.PrefixFrom(e.cfg.Subnet4.Masked().Addr().Next(), e.cfg.Subnet4.Bits()))
	if e.cfg.Subnet6.IsValid() {
		out = append(out, netip.PrefixFrom(e.cfg.Subnet6.Masked().Addr().Next(), e.cfg.Subnet6.Bits()))
	}
	return out
}

func (e *Engine) tunnelSubnets() []netip.Prefix {
	out := []netip.Prefix{e.cfg.Subnet4.Masked()}
	if e.cfg.Subnet6.IsValid() {
		out = append(out, e.cfg.Subnet6.Masked())
	}
	return out
}

// Start loads state, brings the interface up and starts the background
// loops. It is safe to call Stop after a failed Start.
func (e *Engine) Start(ctx context.Context) error {
	if err := e.loadSettings(ctx); err != nil {
		return err
	}
	if err := e.loadServerKey(ctx); err != nil {
		return err
	}
	if err := e.loadPeers(ctx); err != nil {
		return err
	}
	if e.cfg.ManageSysctl && e.be.Kind() != "mock" {
		results, err := netcfg.ApplyAll(netcfg.Wanted(e.cfg.Subnet6.IsValid()))
		e.mu.Lock()
		e.sysctls = results
		e.mu.Unlock()
		for _, r := range results {
			if !r.Applied {
				e.log.Warn("sysctl not applied", "key", r.Key, "wanted", r.Value, "current", r.Current, "error", r.Err, "why", r.Why)
			}
		}
		if err != nil {
			return err
		}
	}
	settings := e.Settings()
	dev := wg.DeviceConfig{PrivateKey: e.serverKey, ListenPort: e.cfg.ListenPort}
	if err := e.be.Up(ctx, dev, e.ServerAddresses(), settings.MTU); err != nil {
		return err
	}
	e.log.Info("interface up", "iface", e.cfg.Iface, "backend", e.be.Kind(), "port", e.cfg.ListenPort, "addresses", e.ServerAddresses(), "mtu", settings.MTU)
	if err := e.applyFirewall(ctx); err != nil {
		// Not fatal: the operator may run their own NAT. It is reported in
		// the UI and the log so nobody wonders why peers cannot reach out.
		e.log.Error("firewall rules not applied", "error", err)
		e.mu.Lock()
		e.fwErr = err.Error()
		e.mu.Unlock()
	}
	if err := e.reconcile(ctx); err != nil {
		return err
	}
	e.mu.Lock()
	e.startedAt = time.Now()
	e.mu.Unlock()

	loopCtx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.wg.Add(2)
	go func() { defer e.wg.Done(); e.col.run(loopCtx) }()
	go func() { defer e.wg.Done(); e.housekeeping(loopCtx) }()
	return nil
}

// Stop halts the loops, flushes counters and tears the interface down.
func (e *Engine) Stop(ctx context.Context) error {
	if e.cancel != nil {
		e.cancel()
		e.wg.Wait()
	}
	var errs []error
	if err := e.col.flush(ctx); err != nil {
		errs = append(errs, err)
	}
	if e.cfg.ManageFirewall && e.be.Kind() != "mock" {
		if err := netcfg.Remove(ctx, ""); err != nil {
			errs = append(errs, err)
		}
	}
	if err := e.be.Down(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *Engine) loadSettings(ctx context.Context) error {
	s, ok, err := loadSettings(ctx, e.st)
	if err != nil {
		return err
	}
	if !ok {
		def := DefaultSettings(e.cfg.InitialEndpoint, e.cfg.InitialDNS, e.cfg.ListenPort)
		s = &def
		if err := saveSettings(ctx, e.st, s); err != nil {
			return err
		}
	}
	e.mu.Lock()
	e.settings = *s
	e.mu.Unlock()
	return nil
}

func (e *Engine) loadServerKey(ctx context.Context) error {
	raw, err := e.st.GetSetting(ctx, serverKeySetting)
	if err != nil {
		return err
	}
	var key wg.Key
	if raw == "" {
		key, err = wg.GeneratePrivateKey()
		if err != nil {
			return err
		}
		if err := e.st.SetSetting(ctx, serverKeySetting, key.String()); err != nil {
			return err
		}
		e.log.Info("generated server key", "publicKey", key.PublicKey().String())
	} else {
		key, err = wg.ParseKey(raw)
		if err != nil {
			return fmt.Errorf("stored server key is invalid: %w", err)
		}
	}
	e.mu.Lock()
	e.serverKey = key
	e.mu.Unlock()
	return nil
}

func (e *Engine) loadPeers(ctx context.Context) error {
	peers, err := e.st.ListPeers(ctx)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.peers = make(map[string]*store.Peer, len(peers))
	e.byKey = make(map[string]*store.Peer, len(peers))
	for _, p := range peers {
		e.peers[p.ID] = p
		e.byKey[p.PublicKey] = p
	}
	return nil
}

func (e *Engine) applyFirewall(ctx context.Context) error {
	if !e.cfg.ManageFirewall || e.be.Kind() == "mock" {
		return nil
	}
	egress := e.cfg.Egress
	if egress == "" {
		if d, err := netcfg.DefaultEgress(); err == nil {
			egress = d
		} else {
			e.log.Warn("could not detect the egress interface; masquerading on every non-tunnel interface", "error", err)
		}
	}
	s := e.Settings()
	rules := netcfg.Rules{
		Iface:         e.cfg.Iface,
		Egress:        egress,
		ListenPort:    e.cfg.ListenPort,
		Subnets:       e.tunnelSubnets(),
		PeerIsolation: s.PeerIsolation,
		ClampMSS:      s.ClampMSS,
	}
	if err := netcfg.Apply(ctx, rules); err != nil {
		return err
	}
	e.mu.Lock()
	e.egress = egress
	e.fwErr = ""
	e.mu.Unlock()
	e.log.Info("firewall rules applied", "egress", egress, "peerIsolation", s.PeerIsolation, "clampMSS", s.ClampMSS)
	return nil
}

// UpdateSettings validates, persists and applies new settings.
func (e *Engine) UpdateSettings(ctx context.Context, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	old := e.Settings()
	if err := saveSettings(ctx, e.st, &s); err != nil {
		return err
	}
	e.mu.Lock()
	e.settings = s
	e.mu.Unlock()
	if s.MTU != old.MTU {
		if err := e.be.SetMTU(ctx, s.MTU); err != nil {
			e.log.Warn("could not change interface MTU", "error", err)
		}
	}
	if s.PeerIsolation != old.PeerIsolation || s.ClampMSS != old.ClampMSS {
		if err := e.applyFirewall(ctx); err != nil {
			e.mu.Lock()
			e.fwErr = err.Error()
			e.mu.Unlock()
			return fmt.Errorf("settings saved but firewall rules failed: %w", err)
		}
	}
	e.hub.Publish("settings", s)
	return nil
}

// active reports whether a peer should currently be on the interface.
func active(p *store.Peer, now time.Time) bool {
	if !p.Enabled {
		return false
	}
	if !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt) {
		return false
	}
	return true
}

func (e *Engine) peerConfig(p *store.Peer) (wg.PeerConfig, error) {
	pub, err := wg.ParseKey(p.PublicKey)
	if err != nil {
		return wg.PeerConfig{}, err
	}
	pc := wg.PeerConfig{PublicKey: pub}
	if p.PresharedKey != "" {
		psk, err := wg.ParseKey(p.PresharedKey)
		if err != nil {
			return wg.PeerConfig{}, err
		}
		pc.PresharedKey = &psk
	}
	if a, err := netip.ParseAddr(p.IPv4); err == nil {
		pc.AllowedIPs = append(pc.AllowedIPs, netip.PrefixFrom(a, 32))
	}
	if p.IPv6 != "" {
		if a, err := netip.ParseAddr(p.IPv6); err == nil {
			pc.AllowedIPs = append(pc.AllowedIPs, netip.PrefixFrom(a, 128))
		}
	}
	return pc, nil
}

// reconcile makes the interface's peer set match the database, one peer at
// a time. It never uses ReplacePeers on a running interface: that would
// reset every counter and drop every session for the sake of one change.
func (e *Engine) reconcile(ctx context.Context) error {
	dev, err := e.be.Device(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	e.mu.RLock()
	want := make(map[string]wg.PeerConfig, len(e.peers))
	for _, p := range e.peers {
		if active(p, now) {
			pc, err := e.peerConfig(p)
			if err != nil {
				e.log.Warn("skipping peer with bad key", "peer", p.ID, "error", err)
				continue
			}
			want[p.PublicKey] = pc
		}
	}
	e.mu.RUnlock()

	have := make(map[string]wg.PeerState, len(dev.Peers))
	for _, p := range dev.Peers {
		have[p.PublicKey.String()] = p
	}
	var errs []error
	for key, pc := range want {
		cur, ok := have[key]
		if ok && samePeer(cur, pc) {
			continue
		}
		if err := e.be.SetPeer(ctx, pc); err != nil {
			errs = append(errs, fmt.Errorf("add peer %s: %w", key, err))
		}
	}
	for key, cur := range have {
		if _, ok := want[key]; !ok {
			if err := e.be.RemovePeer(ctx, cur.PublicKey); err != nil {
				errs = append(errs, fmt.Errorf("remove peer %s: %w", key, err))
			}
		}
	}
	return errors.Join(errs...)
}

func samePeer(cur wg.PeerState, want wg.PeerConfig) bool {
	if len(cur.AllowedIPs) != len(want.AllowedIPs) {
		return false
	}
	set := map[netip.Prefix]bool{}
	for _, a := range cur.AllowedIPs {
		set[a] = true
	}
	for _, a := range want.AllowedIPs {
		if !set[a] {
			return false
		}
	}
	return cur.PersistentKeepalive == want.PersistentKeepalive
}

func (e *Engine) housekeeping(ctx context.Context) {
	t := time.NewTicker(houseEvery)
	defer t.Stop()
	prune := time.NewTicker(time.Hour)
	defer prune.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := e.reconcile(ctx); err != nil {
				e.log.Warn("reconcile", "error", err)
			}
			if err := e.col.flush(ctx); err != nil {
				e.log.Warn("flush counters", "error", err)
			}
			_ = e.st.PruneSessions(ctx)
		case <-prune.C:
			_ = e.st.PruneTraffic(ctx, time.Now().Add(-e.cfg.TrafficRetention))
			_ = e.st.PruneAudit(ctx, 5000)
		}
	}
}
