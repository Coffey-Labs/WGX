//go:build linux

package wg

import (
	"net/netip"
	"testing"
)

func TestIPNetConversions(t *testing.T) {
	// An interface address keeps its host bits; a route prefix loses them.
	// Getting this wrong once put 10.8.0.0/24 on the interface, and the
	// server answered nothing on 10.8.0.1.
	addr := addrToIPNet(netip.MustParsePrefix("10.8.0.1/24"))
	if addr.IP.String() != "10.8.0.1" {
		t.Fatalf("address lost host bits: %s", addr.IP)
	}
	if ones, _ := addr.Mask.Size(); ones != 24 {
		t.Fatalf("mask %d", ones)
	}
	route := prefixToIPNet(netip.MustParsePrefix("10.8.0.7/24"))
	if route.IP.String() != "10.8.0.0" {
		t.Fatalf("route kept host bits: %s", route.IP)
	}
	v6 := addrToIPNet(netip.MustParsePrefix("fd42::1/64"))
	if v6.IP.String() != "fd42::1" || len(v6.IP) != 16 {
		t.Fatalf("v6 %s", v6.IP)
	}
	back, ok := ipNetToPrefix(route)
	if !ok || back.String() != "10.8.0.0/24" {
		t.Fatalf("round trip %v %v", back, ok)
	}
}

func TestKeys(t *testing.T) {
	priv, err := GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	if priv[0]&7 != 0 || priv[31]&128 != 0 || priv[31]&64 == 0 {
		t.Fatal("private key not clamped")
	}
	pub := priv.PublicKey()
	parsed, err := ParseKey(pub.String())
	if err != nil || parsed != pub {
		t.Fatal("public key does not round-trip through base64")
	}
	if _, err := ParseKey("not base64!"); err == nil {
		t.Fatal("bad key accepted")
	}
	if _, err := ParseKey("AAAA"); err == nil {
		t.Fatal("short key accepted")
	}
}
