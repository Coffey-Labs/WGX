// Package auth holds the primitives behind the admin login: argon2id password
// hashing, opaque session tokens, RFC 6238 one-time passwords, recovery codes
// and a login rate limiter. None of it knows about HTTP.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters: 64 MiB, 3 passes, 4 lanes. Roughly 100 ms on a
// modest server, which is the point.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
)

// MinPasswordLength is the shortest password accepted.
const MinPasswordLength = 12

// ValidatePassword enforces the password policy: length only. Composition
// rules produce worse passwords, not better ones.
func ValidatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(pw) > 1024 {
		return errors.New("password is too long")
	}
	return nil
}

// HashPassword returns a PHC-format argon2id string.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks a password against a hash from HashPassword.
func VerifyPassword(hash, pw string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is verified against when the user does not exist, so a login
// for an unknown name takes as long as one for a known name.
var dummyHash, _ = HashPassword("ihasvpn-timing-equaliser-password")

// EqualiseTiming burns the cost of one hash verification.
func EqualiseTiming() { VerifyPassword(dummyHash, "not-the-password") }

// NewToken returns a random URL-safe token and its storage hash.
func NewToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken is how tokens are stored: a leaked database is not a leaked login.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewID returns a short random identifier for peers.
func NewID() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)), nil
}

// --- TOTP -----------------------------------------------------------------

// NewTOTPSecret returns a base32 secret for an authenticator app.
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// TOTPURI builds the otpauth:// URI an authenticator app scans.
func TOTPURI(issuer, account, secret string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}

func totpCode(secret string, counter uint64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", errors.New("bad secret")
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := (binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff) % 1_000_000
	return fmt.Sprintf("%06d", code), nil
}

// TOTPNow returns the current code, for the tests and the setup flow.
func TOTPNow(secret string, at time.Time) (string, error) {
	return totpCode(secret, uint64(at.Unix()/30))
}

// VerifyTOTP accepts the current code and one step either side.
func VerifyTOTP(secret, code string, at time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	if _, err := strconv.Atoi(code); err != nil {
		return false
	}
	counter := uint64(at.Unix() / 30)
	ok := false
	for _, c := range []uint64{counter - 1, counter, counter + 1} {
		want, err := totpCode(secret, c)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			ok = true
		}
	}
	return ok
}

// NewRecoveryCodes returns n codes in the form xxxx-xxxx-xxxx and their hashes.
func NewRecoveryCodes(n int) (codes, hashes []string, err error) {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	for i := 0; i < n; i++ {
		b := make([]byte, 12)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		var sb strings.Builder
		for j, x := range b {
			if j > 0 && j%4 == 0 {
				sb.WriteByte('-')
			}
			sb.WriteByte(alphabet[int(x)%len(alphabet)])
		}
		codes = append(codes, sb.String())
		hashes = append(hashes, HashToken(NormaliseRecoveryCode(sb.String())))
	}
	return codes, hashes, nil
}

// NormaliseRecoveryCode strips separators and case so typed codes match.
func NormaliseRecoveryCode(c string) string {
	return strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(c))
}

// --- Rate limiting --------------------------------------------------------

// Limiter is a fixed-window failure counter keyed by string (an IP, a
// username). After max failures in the window the key is locked out until
// the window passes.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]*bucket
}

type bucket struct {
	count int
	start time.Time
}

// NewLimiter allows max failures per window.
func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, hits: map[string]*bucket{}}
}

// Allowed reports whether the key may attempt again and how long to wait.
func (l *Limiter) Allowed(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.hits[key]
	if !ok {
		return true, 0
	}
	if time.Since(b.start) > l.window {
		delete(l.hits, key)
		return true, 0
	}
	if b.count >= l.max {
		return false, l.window - time.Since(b.start)
	}
	return true, 0
}

// Fail records a failure.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.hits[key]
	if !ok || time.Since(b.start) > l.window {
		l.hits[key] = &bucket{count: 1, start: time.Now()}
		return
	}
	b.count++
}

// Reset clears a key after success.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}

// Sweep drops stale keys; call it now and then.
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.hits {
		if time.Since(b.start) > l.window {
			delete(l.hits, k)
		}
	}
}
