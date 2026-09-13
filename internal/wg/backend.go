// Package wg abstracts the WireGuard data plane behind a small interface so the
// rest of ihasvpn does not care whether peers live in the kernel module, in a
// userspace wireguard-go process, or in an in-memory mock used by tests and
// UI development.
package wg

import (
	"context"
	"net"
	"net/netip"
	"time"
)

// Key is a 32-byte WireGuard key (public, private or preshared).
type Key [32]byte

// PeerState is one peer as the data plane currently sees it.
type PeerState struct {
	PublicKey         Key
	Endpoint          *net.UDPAddr
	LastHandshake     time.Time // zero when the peer has never completed a handshake
	ReceiveBytes      int64
	TransmitBytes     int64
	AllowedIPs        []netip.Prefix
	PersistentKeepalive time.Duration
}

// PeerConfig is what ihasvpn wants a peer to look like on the interface.
type PeerConfig struct {
	PublicKey           Key
	PresharedKey        *Key
	AllowedIPs          []netip.Prefix
	PersistentKeepalive time.Duration
}

// DeviceConfig is the interface-level configuration.
type DeviceConfig struct {
	PrivateKey Key
	ListenPort int
	// FirewallMark is applied to every packet the interface sends; zero means
	// none. Left at zero by ihasvpn, but exposed for completeness.
	FirewallMark int
}

// DeviceState is a snapshot of the interface.
type DeviceState struct {
	Name       string
	PublicKey  Key
	ListenPort int
	Peers      []PeerState
}

// Backend is the data plane ihasvpn drives.
type Backend interface {
	// Kind names the implementation: "kernel", "userspace" or "mock".
	Kind() string
	// Up creates the interface (if needed), applies the device configuration
	// and brings the link up with the given addresses and MTU.
	Up(ctx context.Context, cfg DeviceConfig, addrs []netip.Prefix, mtu int) error
	// Down tears the interface down and releases every resource Up acquired.
	Down(ctx context.Context) error
	// Device returns the current state of the interface and all of its peers.
	Device(ctx context.Context) (*DeviceState, error)
	// SetPeer adds or replaces a peer; AllowedIPs replace what was there.
	SetPeer(ctx context.Context, p PeerConfig) error
	// RemovePeer removes a peer. Removing a peer that is absent is not an error.
	RemovePeer(ctx context.Context, pub Key) error
	// ReplacePeers makes the interface's peer set exactly the given list.
	ReplacePeers(ctx context.Context, peers []PeerConfig) error
	// SetMTU changes the interface MTU without disturbing peers.
	SetMTU(ctx context.Context, mtu int) error
}
