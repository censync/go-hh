//go:build go1.24

package hh

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/sha256"
	"testing"
)

// crypto/pbkdf2 exists since Go 1.24; where it does, it must agree.
func TestPBKDF2AgainstTheStandardLibrary(t *testing.T) {
	random := lcg(0x5EED)
	for i := 0; i < 60; i++ {
		// Passwords shorter than 14 bytes are refused in FIPS 140-only mode.
		password := random.bytes(14 + random.intn(120))
		salt := random.bytes(16 + random.intn(64))
		iterations := 1 + random.intn(300)
		keyLen := 14 + random.intn(90)
		want, err := pbkdf2.Key(sha256.New, string(password), salt, iterations, keyLen)
		if err != nil {
			t.Fatal(err)
		}
		if got := pbkdf2SHA256(password, salt, iterations, keyLen); !bytes.Equal(got, want) {
			t.Fatalf("password %x salt %x c = %d dkLen = %d: %x, want %x", password, salt, iterations, keyLen, got, want)
		}
	}
}
