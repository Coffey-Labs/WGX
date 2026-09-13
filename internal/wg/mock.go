package wg

import (
	"context"
	"math/rand/v2"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"
)

// Mock is an in-memory data plane. It needs no privileges, so it is what the
// tests use and what `IHASVPN_BACKEND=mock` gives a developer working on the UI.
// With Simulate on, peers randomly handshake, move traffic and go quiet so
// the dashboard has something to show.
type Mock struct {
	mu       sync.Mutex
	name     string
	cfg      DeviceConfig
	peers    map[Key]*mockPeer
	up       bool
	Simulate bool
	stop     chan struct{}
}

type mockPeer struct {
	cfg       PeerConfig
	endpoint  *net.UDPAddr
	handshake time.Time
	rx, tx    int64
	active    bool
}

// NewMock returns an empty mock backend for the named interface.
func NewMock(name string, simulate bool) *Mock {
	return &Mock{name: name, peers: map[Key]*mockPeer{}, Simulate: simulate}
}

func (m *Mock) Kind() string { return "mock" }

func (m *Mock) Up(ctx context.Context, cfg DeviceConfig, addrs []netip.Prefix, mtu int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	m.up = true
	if m.Simulate && m.stop == nil {
		m.stop = make(chan struct{})
		go m.simulate(m.stop)
	}
	return nil
}

func (m *Mock) Down(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.up = false
	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
	return nil
}

func (m *Mock) Device(ctx context.Context) (*DeviceState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := &DeviceState{Name: m.name, PublicKey: m.cfg.PrivateKey.PublicKey(), ListenPort: m.cfg.ListenPort}
	keys := make([]Key, 0, len(m.peers))
	for k := range m.peers {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	for _, k := range keys {
		p := m.peers[k]
		st.Peers = append(st.Peers, PeerState{
			PublicKey:           k,
			Endpoint:            p.endpoint,
			LastHandshake:       p.handshake,
			ReceiveBytes:        p.rx,
			TransmitBytes:       p.tx,
			AllowedIPs:          append([]netip.Prefix(nil), p.cfg.AllowedIPs...),
			PersistentKeepalive: p.cfg.PersistentKeepalive,
		})
	}
	return st, nil
}

func (m *Mock) SetPeer(ctx context.Context, p PeerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.peers[p.PublicKey]; ok {
		existing.cfg = p
		return nil
	}
	// Half of new peers start out busy so a fresh mock install has
	// something moving on the dashboard straight away.
	m.peers[p.PublicKey] = &mockPeer{cfg: p, active: m.Simulate && rand.IntN(2) == 0}
	return nil
}

func (m *Mock) RemovePeer(ctx context.Context, pub Key) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.peers, pub)
	return nil
}

func (m *Mock) ReplacePeers(ctx context.Context, peers []PeerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := map[Key]*mockPeer{}
	for _, p := range peers {
		if existing, ok := m.peers[p.PublicKey]; ok {
			existing.cfg = p
			next[p.PublicKey] = existing
		} else {
			next[p.PublicKey] = &mockPeer{cfg: p}
		}
	}
	m.peers = next
	return nil
}

func (m *Mock) SetMTU(ctx context.Context, mtu int) error { return nil }

// Touch fakes a handshake and some traffic for a peer. Tests use it to make
// a peer look connected without waiting on the simulator.
func (m *Mock) Touch(pub Key, rx, tx int64, endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.peers[pub]
	if !ok {
		return
	}
	p.handshake = time.Now()
	p.rx += rx
	p.tx += tx
	if endpoint != "" {
		if ap, err := netip.ParseAddrPort(endpoint); err == nil {
			p.endpoint = net.UDPAddrFromAddrPort(ap)
		}
	}
}

func (m *Mock) simulate(stop chan struct{}) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		m.mu.Lock()
		for _, p := range m.peers {
			// Peers flip between active and idle a few times an hour.
			if rand.IntN(60) == 0 {
				p.active = !p.active
			}
			if p.active {
				if p.endpoint == nil {
					p.endpoint = &net.UDPAddr{IP: net.IPv4(203, 0, 113, byte(1+rand.IntN(250))), Port: 30000 + rand.IntN(30000)}
				}
				if time.Since(p.handshake) > time.Duration(90+rand.IntN(40))*time.Second {
					p.handshake = time.Now()
				}
				p.rx += int64(rand.IntN(400_000))
				p.tx += int64(rand.IntN(3_000_000))
			}
		}
		m.mu.Unlock()
	}
}
