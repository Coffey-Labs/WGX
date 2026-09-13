//go:build linux

package wg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// linuxBackend drives a real WireGuard interface. In kernel mode the link is a
// native `wireguard` netlink link and every packet is handled by the module;
// in userspace mode a wireguard-go process owns a TUN device with the same
// name and WGX talks to it over its UAPI socket. Both are configured through
// wgctrl, which picks the transport on its own.
type linuxBackend struct {
	name      string
	userspace bool
	client    *wgctrl.Client
	proc      *exec.Cmd
	log       *slog.Logger
}

// KernelAvailable reports whether the running kernel can create a WireGuard
// link. It tries to add and immediately delete a probe interface rather than
// trusting /sys/module, because a module that is loadable but not yet loaded
// is only discovered by asking for it.
func KernelAvailable() bool {
	const probe = "wgxprobe0"
	link := &netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: probe}}
	if err := netlink.LinkAdd(link); err != nil {
		return false
	}
	_ = netlink.LinkDel(link)
	return true
}

// NewKernel returns a backend that uses the kernel module.
func NewKernel(name string, log *slog.Logger) (Backend, error) {
	return newLinux(name, false, log)
}

// NewUserspace returns a backend that runs wireguard-go for the data plane.
func NewUserspace(name string, log *slog.Logger) (Backend, error) {
	if _, err := exec.LookPath("wireguard-go"); err != nil {
		return nil, errors.New("wireguard-go is not installed and the kernel has no WireGuard support")
	}
	return newLinux(name, true, log)
}

func newLinux(name string, userspace bool, log *slog.Logger) (Backend, error) {
	c, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("open wgctrl: %w", err)
	}
	return &linuxBackend{name: name, userspace: userspace, client: c, log: log}, nil
}

func (b *linuxBackend) Kind() string {
	if b.userspace {
		return "userspace"
	}
	return "kernel"
}

func (b *linuxBackend) Up(ctx context.Context, cfg DeviceConfig, addrs []netip.Prefix, mtu int) error {
	// A previous run that died without Down leaves the link behind. Start
	// clean rather than inheriting peers and addresses nobody remembers.
	if err := b.deleteLink(); err != nil {
		return err
	}
	if b.userspace {
		if err := b.startUserspace(ctx); err != nil {
			return err
		}
	} else {
		if err := netlink.LinkAdd(&netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: b.name}}); err != nil {
			return fmt.Errorf("create %s: %w (is the container running with NET_ADMIN?)", b.name, err)
		}
	}
	link, err := b.link()
	if err != nil {
		return err
	}
	priv := wgtypes.Key(cfg.PrivateKey)
	port := cfg.ListenPort
	wcfg := wgtypes.Config{PrivateKey: &priv, ListenPort: &port, ReplacePeers: true}
	if cfg.FirewallMark != 0 {
		fw := cfg.FirewallMark
		wcfg.FirewallMark = &fw
	}
	if err := b.client.ConfigureDevice(b.name, wcfg); err != nil {
		return fmt.Errorf("configure %s: %w", b.name, err)
	}
	for _, p := range addrs {
		// Not prefixToIPNet: that masks the host bits, and an interface
		// address must keep them (10.8.0.1/24, not 10.8.0.0/24).
		ipn := addrToIPNet(p)
		if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: &ipn}); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("add address %s: %w", p, err)
		}
	}
	if mtu > 0 {
		if err := netlink.LinkSetMTU(link, mtu); err != nil {
			return fmt.Errorf("set mtu %d: %w", mtu, err)
		}
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring up %s: %w", b.name, err)
	}
	return nil
}

func (b *linuxBackend) startUserspace(ctx context.Context) error {
	_ = os.MkdirAll("/var/run/wireguard", 0o700)
	cmd := exec.Command("wireguard-go", "-f", b.name)
	cmd.Env = append(os.Environ(), "WG_PROCESS_FOREGROUND=1", "LOG_LEVEL=error")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start wireguard-go: %w", err)
	}
	b.proc = cmd
	sock := filepath.Join("/var/run/wireguard", b.name+".sock")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			if _, err := netlink.LinkByName(b.name); err == nil {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("wireguard-go did not create its UAPI socket in time")
}

func (b *linuxBackend) Down(ctx context.Context) error {
	var errs []error
	if err := b.deleteLink(); err != nil {
		errs = append(errs, err)
	}
	if b.proc != nil && b.proc.Process != nil {
		_ = b.proc.Process.Kill()
		_ = b.proc.Wait()
		b.proc = nil
	}
	if err := b.client.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (b *linuxBackend) deleteLink() error {
	link, err := netlink.LinkByName(b.name)
	if err != nil {
		var nf netlink.LinkNotFoundError
		if errors.As(err, &nf) {
			return nil
		}
		return fmt.Errorf("look up %s: %w", b.name, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete stale %s: %w", b.name, err)
	}
	return nil
}

func (b *linuxBackend) link() (netlink.Link, error) {
	link, err := netlink.LinkByName(b.name)
	if err != nil {
		return nil, fmt.Errorf("look up %s: %w", b.name, err)
	}
	return link, nil
}

func (b *linuxBackend) Device(ctx context.Context) (*DeviceState, error) {
	d, err := b.client.Device(b.name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", b.name, err)
	}
	st := &DeviceState{Name: d.Name, PublicKey: Key(d.PublicKey), ListenPort: d.ListenPort}
	st.Peers = make([]PeerState, 0, len(d.Peers))
	for _, p := range d.Peers {
		ps := PeerState{
			PublicKey:           Key(p.PublicKey),
			Endpoint:            p.Endpoint,
			LastHandshake:       p.LastHandshakeTime,
			ReceiveBytes:        p.ReceiveBytes,
			TransmitBytes:       p.TransmitBytes,
			PersistentKeepalive: p.PersistentKeepaliveInterval,
		}
		for _, a := range p.AllowedIPs {
			if pfx, ok := ipNetToPrefix(a); ok {
				ps.AllowedIPs = append(ps.AllowedIPs, pfx)
			}
		}
		st.Peers = append(st.Peers, ps)
	}
	return st, nil
}

func (b *linuxBackend) SetPeer(ctx context.Context, p PeerConfig) error {
	return b.client.ConfigureDevice(b.name, wgtypes.Config{Peers: []wgtypes.PeerConfig{toPeerConfig(p)}})
}

func (b *linuxBackend) RemovePeer(ctx context.Context, pub Key) error {
	err := b.client.ConfigureDevice(b.name, wgtypes.Config{Peers: []wgtypes.PeerConfig{{PublicKey: wgtypes.Key(pub), Remove: true}}})
	if err != nil && strings.Contains(err.Error(), "no such") {
		return nil
	}
	return err
}

func (b *linuxBackend) ReplacePeers(ctx context.Context, peers []PeerConfig) error {
	cfg := wgtypes.Config{ReplacePeers: true}
	for _, p := range peers {
		cfg.Peers = append(cfg.Peers, toPeerConfig(p))
	}
	return b.client.ConfigureDevice(b.name, cfg)
}

func (b *linuxBackend) SetMTU(ctx context.Context, mtu int) error {
	link, err := b.link()
	if err != nil {
		return err
	}
	return netlink.LinkSetMTU(link, mtu)
}

func toPeerConfig(p PeerConfig) wgtypes.PeerConfig {
	pc := wgtypes.PeerConfig{PublicKey: wgtypes.Key(p.PublicKey), ReplaceAllowedIPs: true}
	if p.PresharedKey != nil {
		psk := wgtypes.Key(*p.PresharedKey)
		pc.PresharedKey = &psk
	}
	if p.PersistentKeepalive > 0 {
		ka := p.PersistentKeepalive
		pc.PersistentKeepaliveInterval = &ka
	}
	for _, a := range p.AllowedIPs {
		pc.AllowedIPs = append(pc.AllowedIPs, prefixToIPNet(a))
	}
	return pc
}

// prefixToIPNet converts a route prefix; host bits are cleared.
func prefixToIPNet(p netip.Prefix) net.IPNet {
	return addrToIPNet(p.Masked())
}

// addrToIPNet converts an interface address with its prefix length, keeping
// the host bits.
func addrToIPNet(p netip.Prefix) net.IPNet {
	ip := p.Addr()
	if ip.Is4() {
		a := ip.As4()
		return net.IPNet{IP: net.IP(a[:]), Mask: net.CIDRMask(p.Bits(), 32)}
	}
	a := ip.As16()
	return net.IPNet{IP: net.IP(a[:]), Mask: net.CIDRMask(p.Bits(), 128)}
}

func ipNetToPrefix(n net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap()
	ones, _ := n.Mask.Size()
	return netip.PrefixFrom(addr, ones), true
}
