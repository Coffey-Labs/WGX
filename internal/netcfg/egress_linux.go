//go:build linux

package netcfg

import (
	"errors"
	"net"

	"github.com/vishvananda/netlink"
)

// DefaultEgress returns the interface the default IPv4 route leaves through.
// Masquerading on exactly that interface (rather than "anything that is not
// wg0") keeps the NAT rule from touching docker-internal traffic.
func DefaultEgress() (string, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}
	for _, r := range routes {
		if r.Dst == nil || (r.Dst.IP.Equal(net.IPv4zero) && isZeroMask(r.Dst.Mask)) {
			if r.LinkIndex == 0 {
				continue
			}
			link, err := netlink.LinkByIndex(r.LinkIndex)
			if err != nil {
				return "", err
			}
			return link.Attrs().Name, nil
		}
	}
	return "", errors.New("no default route")
}

func isZeroMask(m net.IPMask) bool {
	for _, b := range m {
		if b != 0 {
			return false
		}
	}
	return true
}
