package engine

import (
	"errors"
	"fmt"
	"net/netip"
)

// allocate returns the lowest free host address in the subnet, skipping the
// network address, the server's own (first host) and, for IPv4, the
// broadcast address. `used` holds addresses already handed out.
func allocate(subnet netip.Prefix, used map[string]bool) (netip.Addr, error) {
	subnet = subnet.Masked()
	base := subnet.Addr()
	server := base.Next()
	var last netip.Addr
	if base.Is4() {
		// Broadcast: all host bits set.
		a := base.As4()
		hostBits := 32 - subnet.Bits()
		var n uint32 = uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
		n |= (1 << hostBits) - 1
		last = netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	}
	// Cap the scan: a /64 is not walked to the end, and nobody has 65k peers.
	const maxScan = 65536
	addr := server.Next()
	for i := 0; i < maxScan && subnet.Contains(addr); i++ {
		if base.Is4() && addr == last {
			break
		}
		if !used[addr.String()] {
			return addr, nil
		}
		addr = addr.Next()
	}
	return netip.Addr{}, errors.New("no free addresses left in " + subnet.String())
}

// checkAddress validates an operator-chosen address for a peer.
func checkAddress(subnet netip.Prefix, s string, used map[string]bool) (netip.Addr, error) {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%q is not an IP address", s)
	}
	if !subnet.Contains(a) {
		return netip.Addr{}, fmt.Errorf("%s is outside %s", a, subnet.Masked())
	}
	if a == subnet.Masked().Addr() || a == subnet.Masked().Addr().Next() {
		return netip.Addr{}, fmt.Errorf("%s is reserved for the server", a)
	}
	if used[a.String()] {
		return netip.Addr{}, fmt.Errorf("%s is already assigned", a)
	}
	return a, nil
}
