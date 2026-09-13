package netcfg

import (
	"net/netip"
	"strings"
	"testing"
)

func TestRuleset(t *testing.T) {
	r := Rules{
		Iface:         "wg0",
		Egress:        "eth0",
		ListenPort:    51820,
		Subnets:       []netip.Prefix{netip.MustParsePrefix("10.8.0.0/24"), netip.MustParsePrefix("fd42::/64")},
		PeerIsolation: true,
		ClampMSS:      true,
	}
	out := Ruleset(r)
	for _, want := range []string{
		"table inet ihasvpn {",
		"udp dport 51820 accept",
		`iifname "wg0" oifname "wg0" drop`,
		`tcp option maxseg size set rt mtu`,
		`ip saddr 10.8.0.0/24 oifname "eth0" masquerade`,
		`ip6 saddr fd42::/64 oifname "eth0" masquerade`,
		`oifname "wg0" ct state related,established accept`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ruleset missing %q:\n%s", want, out)
		}
	}
	// Without an egress, masquerade on anything that is not the tunnel.
	r.Egress = ""
	r.PeerIsolation = false
	out = Ruleset(r)
	if !strings.Contains(out, `oifname != "wg0" masquerade`) {
		t.Errorf("expected wildcard masquerade:\n%s", out)
	}
	if strings.Contains(out, "peer isolation") {
		t.Error("isolation rule present when off")
	}
}

func TestWanted(t *testing.T) {
	v4 := Wanted(false)
	v6 := Wanted(true)
	if len(v6) != len(v4)+1 {
		t.Fatal("ipv6 forwarding not added")
	}
	if v4[0].Key != "net.ipv4.ip_forward" || !v4[0].Required {
		t.Fatal("ip_forward must be first and required")
	}
}
