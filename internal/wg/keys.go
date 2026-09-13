package wg

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// GeneratePrivateKey returns a fresh Curve25519 private key, clamped the way
// WireGuard expects.
func GeneratePrivateKey() (Key, error) {
	var k Key
	if _, err := rand.Read(k[:]); err != nil {
		return Key{}, fmt.Errorf("generate private key: %w", err)
	}
	k[0] &= 248
	k[31] &= 127
	k[31] |= 64
	return k, nil
}

// GeneratePresharedKey returns 32 random bytes for use as a preshared key.
func GeneratePresharedKey() (Key, error) {
	var k Key
	if _, err := rand.Read(k[:]); err != nil {
		return Key{}, fmt.Errorf("generate preshared key: %w", err)
	}
	return k, nil
}

// PublicKey derives the public key of a private key.
func (k Key) PublicKey() Key {
	var pub Key
	priv := k
	curve25519.ScalarBaseMult((*[32]byte)(&pub), (*[32]byte)(&priv))
	return pub
}

// String renders the key the way wg(8) does: standard base64.
func (k Key) String() string { return base64.StdEncoding.EncodeToString(k[:]) }

// IsZero reports whether the key is all zeros.
func (k Key) IsZero() bool { return k == Key{} }

// ParseKey parses a base64 key as produced by wg genkey / wg pubkey.
func ParseKey(s string) (Key, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Key{}, errors.New("key is not valid base64")
	}
	if len(b) != 32 {
		return Key{}, errors.New("key must decode to 32 bytes")
	}
	var k Key
	copy(k[:], b)
	return k, nil
}
