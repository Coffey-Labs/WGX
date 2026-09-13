package auth

import (
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "correct horse battery staple") {
		t.Fatal("right password rejected")
	}
	if VerifyPassword(h, "correct horse battery stapl") {
		t.Fatal("wrong password accepted")
	}
	if VerifyPassword("garbage", "x") {
		t.Fatal("garbage hash accepted")
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	if err := ValidatePassword("twelve chars"); err != nil {
		t.Fatalf("12-character password rejected: %v", err)
	}
}

func TestTOTP(t *testing.T) {
	// RFC 6238 test vector: secret "12345678901234567890" (base32
	// GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ), time 59 -> 287082 with SHA1.
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := TOTPNow(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatal(err)
	}
	if code != "287082" {
		t.Fatalf("got %s, want 287082", code)
	}
	if !VerifyTOTP(secret, "287082", time.Unix(59, 0)) {
		t.Fatal("valid code rejected")
	}
	// One step later still accepted (window of one either side).
	if !VerifyTOTP(secret, "287082", time.Unix(59+30, 0)) {
		t.Fatal("previous-step code rejected")
	}
	if VerifyTOTP(secret, "287082", time.Unix(59+120, 0)) {
		t.Fatal("stale code accepted")
	}
	if VerifyTOTP(secret, "28708", time.Unix(59, 0)) {
		t.Fatal("short code accepted")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes, hashes, err := NewRecoveryCodes(8)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 8 || len(hashes) != 8 {
		t.Fatal("wrong count")
	}
	if HashToken(NormaliseRecoveryCode("  "+codes[0]+" ")) != hashes[0] {
		t.Fatal("normalised code does not hash to stored value")
	}
	if len(codes[0]) != 14 {
		t.Fatalf("unexpected format %q", codes[0])
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(2, time.Minute)
	if ok, _ := l.Allowed("a"); !ok {
		t.Fatal("fresh key blocked")
	}
	l.Fail("a")
	l.Fail("a")
	if ok, wait := l.Allowed("a"); ok || wait <= 0 {
		t.Fatal("key not blocked after max failures")
	}
	if ok, _ := l.Allowed("b"); !ok {
		t.Fatal("unrelated key blocked")
	}
	l.Reset("a")
	if ok, _ := l.Allowed("a"); !ok {
		t.Fatal("reset key still blocked")
	}
}

func TestTokens(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if HashToken(tok) != hash {
		t.Fatal("hash mismatch")
	}
	id, err := NewID()
	if err != nil || len(id) != 16 {
		t.Fatalf("bad id %q %v", id, err)
	}
}
