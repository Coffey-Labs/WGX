package engine

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Coffey-Labs/WGX/internal/config"
	"github.com/Coffey-Labs/WGX/internal/store"
	"github.com/Coffey-Labs/WGX/internal/wg"
)

func testConfig() *config.Config {
	return &config.Config{
		DBPath:           ":memory:",
		Backend:          "mock",
		Iface:            "wg0",
		ListenPort:       51820,
		Subnet4:          netip.MustParsePrefix("10.8.0.0/29"),
		Subnet6:          netip.MustParsePrefix("fd42::/64"),
		HTTP:             "127.0.0.1:0",
		SessionIdle:      time.Hour,
		SessionMax:       24 * time.Hour,
		TrafficRetention: 24 * time.Hour,
		PollInterval:     time.Hour, // tests drive polls by hand
		InitialEndpoint:  "vpn.example.com",
		InitialDNS:       "1.1.1.1",
		ManageFirewall:   true,
		ManageSysctl:     true,
	}
}

func newTestEngine(t *testing.T) (*Engine, *wg.Mock) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	mock := wg.NewMock("wg0", false)
	e := New(testConfig(), st, mock, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Stop(context.Background()) })
	return e, mock
}

func TestAllocate(t *testing.T) {
	subnet := netip.MustParsePrefix("10.8.0.0/29") // .0 net, .1 server, .2-.6 usable, .7 broadcast
	used := map[string]bool{}
	var got []string
	for {
		a, err := allocate(subnet, used)
		if err != nil {
			break
		}
		used[a.String()] = true
		got = append(got, a.String())
	}
	want := "10.8.0.2 10.8.0.3 10.8.0.4 10.8.0.5 10.8.0.6"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	a6, err := allocate(netip.MustParsePrefix("fd42::/64"), map[string]bool{"fd42::2": true})
	if err != nil || a6.String() != "fd42::3" {
		t.Fatalf("v6 allocation %v %v", a6, err)
	}
	if _, err := checkAddress(subnet, "10.8.0.1", used); err == nil {
		t.Fatal("server address accepted")
	}
	if _, err := checkAddress(subnet, "10.9.0.1", used); err == nil {
		t.Fatal("outside address accepted")
	}
}

func TestCreatePeerAndConfig(t *testing.T) {
	e, mock := newTestEngine(t)
	ctx := context.Background()
	p, err := e.CreatePeer(ctx, PeerInput{Name: "Laptop"})
	if err != nil {
		t.Fatal(err)
	}
	if p.IPv4 != "10.8.0.2" || p.IPv6 != "fd42::2" {
		t.Fatalf("addresses %s %s", p.IPv4, p.IPv6)
	}
	if p.PrivateKey == "" || p.PresharedKey == "" {
		t.Fatal("server-managed peer should have private and preshared keys")
	}
	cfg := e.ClientConfig(p)
	for _, want := range []string{
		"PrivateKey = " + p.PrivateKey,
		"Address = 10.8.0.2/29, fd42::2/64",
		"DNS = 1.1.1.1",
		"MTU = 1420",
		"PublicKey = " + e.ServerPublicKey(),
		"PresharedKey = " + p.PresharedKey,
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"Endpoint = vpn.example.com:51820",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q:\n%s", want, cfg)
		}
	}
	dev, _ := mock.Device(ctx)
	if len(dev.Peers) != 1 || dev.Peers[0].PublicKey.String() != p.PublicKey {
		t.Fatal("peer not applied to the interface")
	}
	if len(dev.Peers[0].AllowedIPs) != 2 {
		t.Fatalf("allowed ips %v", dev.Peers[0].AllowedIPs)
	}
	png, err := e.QRCode(p, 256)
	if err != nil || len(png) < 100 {
		t.Fatalf("qr: %v", err)
	}

	// A client-keyed peer: no private key, no QR code.
	priv, _ := wg.GeneratePrivateKey()
	c, err := e.CreatePeer(ctx, PeerInput{Name: "Router", PublicKey: priv.PublicKey().String(), ClientRoutes: "10.8.0.0/29"})
	if err != nil {
		t.Fatal(err)
	}
	if c.PrivateKey != "" || !strings.Contains(e.ClientConfig(c), "<your private key>") {
		t.Fatal("client-keyed peer leaked or lacked placeholder")
	}
	if _, err := e.QRCode(c, 256); err == nil {
		t.Fatal("QR for client-keyed peer should fail")
	}
	if _, err := e.CreatePeer(ctx, PeerInput{Name: "Dup", PublicKey: priv.PublicKey().String()}); err == nil {
		t.Fatal("duplicate public key accepted")
	}
	if _, err := e.CreatePeer(ctx, PeerInput{Name: ""}); err == nil {
		t.Fatal("empty name accepted")
	}
}

func TestDisableResetRotateDelete(t *testing.T) {
	e, mock := newTestEngine(t)
	ctx := context.Background()
	p, _ := e.CreatePeer(ctx, PeerInput{Name: "Phone"})
	pub, _ := wg.ParseKey(p.PublicKey)
	mock.Touch(pub, 1000, 2000, "203.0.113.5:1234")
	e.col.poll(ctx)
	l, ok := e.Live(p.ID)
	if !ok || !l.Connected || l.Endpoint != "203.0.113.5:1234" {
		t.Fatalf("live state %+v", l)
	}
	// First sight sets the memo only; the next delta counts.
	mock.Touch(pub, 500, 700, "")
	e.col.poll(ctx)
	l, _ = e.Live(p.ID)
	if l.Rx != 500 || l.Tx != 700 {
		t.Fatalf("totals %d/%d, want 500/700", l.Rx, l.Tx)
	}

	if _, err := e.SetEnabled(ctx, p.ID, false); err != nil {
		t.Fatal(err)
	}
	dev, _ := mock.Device(ctx)
	if len(dev.Peers) != 0 {
		t.Fatal("disabled peer still on interface")
	}
	e.col.poll(ctx)
	if l, _ := e.Live(p.ID); l.Connected {
		t.Fatal("disabled peer reported connected")
	}
	if _, err := e.SetEnabled(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	if dev, _ = mock.Device(ctx); len(dev.Peers) != 1 {
		t.Fatal("enabled peer not back on interface")
	}
	// Counters restart at zero after re-add; the total must not go backwards.
	mock.Touch(pub, 100, 100, "")
	e.col.poll(ctx)
	mock.Touch(pub, 100, 100, "")
	e.col.poll(ctx)
	l, _ = e.Live(p.ID)
	if l.Rx != 600 || l.Tx != 800 {
		t.Fatalf("totals after re-add %d/%d, want 600/800", l.Rx, l.Tx)
	}

	if err := e.ResetSession(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	r, err := e.RotateKeys(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.PublicKey == p.PublicKey {
		t.Fatal("key did not change")
	}
	dev, _ = mock.Device(ctx)
	if len(dev.Peers) != 1 || dev.Peers[0].PublicKey.String() != r.PublicKey {
		t.Fatal("rotated key not applied")
	}
	if err := e.col.flush(ctx); err != nil {
		t.Fatal(err)
	}
	if err := e.DeletePeer(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Peer(p.ID); err != ErrNotFound {
		t.Fatal("deleted peer still present")
	}
	if dev, _ = mock.Device(ctx); len(dev.Peers) != 0 {
		t.Fatal("deleted peer still on interface")
	}
}

func TestExpiryAndReconcile(t *testing.T) {
	e, mock := newTestEngine(t)
	ctx := context.Background()
	past := time.Now().Add(-time.Minute)
	p, err := e.CreatePeer(ctx, PeerInput{Name: "Temp", ExpiresAt: &past})
	if err != nil {
		t.Fatal(err)
	}
	if dev, _ := mock.Device(ctx); len(dev.Peers) != 0 {
		t.Fatal("expired peer applied")
	}
	future := time.Now().Add(time.Hour)
	if _, err := e.UpdatePeer(ctx, p.ID, PeerInput{Name: "Temp", ExpiresAt: &future}); err != nil {
		t.Fatal(err)
	}
	if dev, _ := mock.Device(ctx); len(dev.Peers) != 1 {
		t.Fatal("un-expired peer not applied")
	}
	// Someone removes the peer by hand; reconcile puts it back.
	pub, _ := wg.ParseKey(p.PublicKey)
	_ = mock.RemovePeer(ctx, pub)
	if err := e.reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if dev, _ := mock.Device(ctx); len(dev.Peers) != 1 {
		t.Fatal("reconcile did not restore peer")
	}
	// A stranger appears on the interface; reconcile removes it.
	stray, _ := wg.GeneratePrivateKey()
	_ = mock.SetPeer(ctx, wg.PeerConfig{PublicKey: stray.PublicKey()})
	if err := e.reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if dev, _ := mock.Device(ctx); len(dev.Peers) != 1 {
		t.Fatal("reconcile did not remove stray peer")
	}
}

func TestSettingsValidation(t *testing.T) {
	e, _ := newTestEngine(t)
	s := e.Settings()
	s.EndpointHost = ""
	if err := e.UpdateSettings(context.Background(), s); err == nil {
		t.Fatal("empty endpoint accepted")
	}
	s = e.Settings()
	s.MTU = 100
	if err := e.UpdateSettings(context.Background(), s); err == nil {
		t.Fatal("tiny MTU accepted")
	}
	s = e.Settings()
	s.DNS = "1.1.1.1, example.com"
	s.MTU = 1380
	if err := e.UpdateSettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if e.Settings().MTU != 1380 {
		t.Fatal("settings not persisted")
	}
}

func TestPersistenceAcrossRestart(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	e1 := New(testConfig(), st, wg.NewMock("wg0", false), log)
	if err := e1.Start(ctx); err != nil {
		t.Fatal(err)
	}
	p, _ := e1.CreatePeer(ctx, PeerInput{Name: "Keep"})
	key := e1.ServerPublicKey()
	_ = e1.Stop(ctx)

	e2 := New(testConfig(), st, wg.NewMock("wg0", false), log)
	if err := e2.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer e2.Stop(ctx)
	if e2.ServerPublicKey() != key {
		t.Fatal("server key changed across restart")
	}
	got, err := e2.Peer(p.ID)
	if err != nil || got.PublicKey != p.PublicKey {
		t.Fatal("peer lost across restart")
	}
	if dev, _ := e2.be.Device(ctx); len(dev.Peers) != 1 {
		t.Fatal("peer not re-applied after restart")
	}
}
