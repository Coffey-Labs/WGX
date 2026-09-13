// Package netcfg owns everything around the WireGuard interface that is not
// WireGuard itself: IP forwarding, the nftables ruleset that NATs peers to the
// outside world, and the sysctls that keep throughput up.
package netcfg

import (
	"context"
	"bytes"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

// Rules describes the firewall WGX wants.
type Rules struct {
	// Iface is the WireGuard interface name, e.g. wg0.
	Iface string
	// Egress is the interface peers reach the outside world through. Empty
	// means "any interface that is not Iface", which is what most single-NIC
	// containers want.
	Egress string
	// ListenPort is the UDP port to accept WireGuard traffic on.
	ListenPort int
	// Subnets are the tunnel networks to masquerade (v4 and/or v6).
	Subnets []netip.Prefix
	// PeerIsolation drops traffic between peers when true.
	PeerIsolation bool
	// ClampMSS rewrites the MSS of forwarded SYNs to fit the path MTU. It
	// costs almost nothing and removes the single most common cause of
	// "the VPN connects but websites hang".
	ClampMSS bool
	// Table names the nftables table, so a host with its own rules never
	// collides with ours. Defaults to "wgx".
	Table string
}

// Ruleset renders the nftables script for the given rules. It is a pure
// function so tests can check the output without a kernel.
func Ruleset(r Rules) string {
	table := r.Table
	if table == "" {
		table = "wgx"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "table inet %s\n", table)
	fmt.Fprintf(&b, "delete table inet %s\n", table)
	fmt.Fprintf(&b, "table inet %s {\n", table)

	fmt.Fprintf(&b, "  chain input {\n")
	fmt.Fprintf(&b, "    type filter hook input priority filter; policy accept;\n")
	fmt.Fprintf(&b, "    udp dport %d accept comment \"wireguard\"\n", r.ListenPort)
	fmt.Fprintf(&b, "  }\n")

	fmt.Fprintf(&b, "  chain forward {\n")
	fmt.Fprintf(&b, "    type filter hook forward priority filter; policy accept;\n")
	if r.PeerIsolation {
		fmt.Fprintf(&b, "    iifname %q oifname %q drop comment \"peer isolation\"\n", r.Iface, r.Iface)
	}
	if r.ClampMSS {
		fmt.Fprintf(&b, "    iifname %q tcp flags syn tcp option maxseg size set rt mtu comment \"clamp mss\"\n", r.Iface)
		fmt.Fprintf(&b, "    oifname %q tcp flags syn tcp option maxseg size set rt mtu comment \"clamp mss\"\n", r.Iface)
	}
	fmt.Fprintf(&b, "    iifname %q accept\n", r.Iface)
	fmt.Fprintf(&b, "    oifname %q ct state related,established accept\n", r.Iface)
	fmt.Fprintf(&b, "  }\n")

	fmt.Fprintf(&b, "  chain postrouting {\n")
	fmt.Fprintf(&b, "    type nat hook postrouting priority srcnat; policy accept;\n")
	for _, s := range r.Subnets {
		fam := "ip"
		if s.Addr().Is6() {
			fam = "ip6"
		}
		if r.Egress != "" {
			fmt.Fprintf(&b, "    %s saddr %s oifname %q masquerade\n", fam, s.Masked(), r.Egress)
		} else {
			fmt.Fprintf(&b, "    %s saddr %s oifname != %q masquerade\n", fam, s.Masked(), r.Iface)
		}
	}
	fmt.Fprintf(&b, "  }\n")
	fmt.Fprintf(&b, "}\n")
	return b.String()
}

// Apply loads the ruleset with nft(8).
func Apply(ctx context.Context, r Rules) error {
	return runNFT(ctx, Ruleset(r))
}

// Remove deletes the WGX table, ignoring the case where it is already gone.
func Remove(ctx context.Context, table string) error {
	if table == "" {
		table = "wgx"
	}
	script := fmt.Sprintf("table inet %s\ndelete table inet %s\n", table, table)
	return runNFT(ctx, script)
}

func runNFT(ctx context.Context, script string) error {
	nft, err := exec.LookPath("nft")
	if err != nil {
		return fmt.Errorf("nft is not installed: %w", err)
	}
	cmd := exec.CommandContext(ctx, nft, "-f", "-")
	cmd.Stdin = strings.NewReader(script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
