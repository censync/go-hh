package hh

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"sync"
)

const (
	// KeySize is the size of a secret key in bytes.
	KeySize = 32
	// KCVSize is the size of a key check value in bytes.
	KCVSize = 4
)

// SecretKey is the 32-byte secret of keyed mode. The key must be uniformly
// random or the output of a key derivation function; there is no passphrase
// form.
//
// Close overwrites the key with zeros. That is as far as a garbage-collected
// runtime lets a library go: the copies that crypto/hmac makes while a
// fingerprint is computed, and any copy the runtime made when it moved a stack,
// stay in memory until they are reused. A host that must keep the key out of
// the Go heap computes the keyed HMAC elsewhere and uses ImportFingerprint.
//
// A SecretKey is safe for concurrent use, including Close. Pass the pointer. A
// copy of the struct is a second handle on the same key, not a second key:
// closing either closes both.
//
// No printer shows the bytes. A *SecretKey prints as a placeholder through
// Format and String. A SecretKey value, which has neither method, prints as the
// address of a function, and so does a key in an unexported field of another
// struct. Packages that dump values by reflection, unexported fields included,
// find the same function value and nothing behind it, and encoding/json writes
// {}.
type SecretKey struct {
	// The state is held by a function value, not by a pointer: reflection
	// follows pointers into unexported fields and cannot look into a closure.
	state func() *secretState
}

// secretState is what a SecretKey and its copies share.
type secretState struct {
	mu    sync.RWMutex
	bytes [KeySize]byte
	kcv   [KCVSize]byte
	open  bool
}

// NewSecretKey accepts exactly 32 bytes that are not all zero; anything else is
// ErrInvalidKey. A zero-filled buffer is what a failed key load looks like and
// must never produce pictures. The bytes are copied: wipe your own slice.
func NewSecretKey(b []byte) (*SecretKey, error) {
	if len(b) != KeySize {
		return nil, ErrInvalidKey
	}
	var acc byte
	for _, v := range b {
		acc |= v
	}
	if acc == 0 {
		return nil, ErrInvalidKey
	}
	s := &secretState{open: true}
	copy(s.bytes[:], b)
	copy(s.kcv[:], hmacSHA256(s.bytes[:], prefix(stageKCV)))
	return &SecretKey{state: func() *secretState { return s }}, nil
}

// secret returns the shared state, or nil for a nil key and for the zero
// SecretKey, which is not a key.
func (k *SecretKey) secret() *secretState {
	if k == nil || k.state == nil {
		return nil
	}
	return k.state()
}

// KCV returns the key check value: 4 bytes a host stores beside its cached data
// to notice that the key, and with it every keyed picture, changed. It is
// public: it reveals nothing useful about the key, it is computed when the key
// is created and it stays available after Close. The KCV of a nil key is all
// zero.
func (k *SecretKey) KCV() [KCVSize]byte {
	s := k.secret()
	if s == nil {
		return [KCVSize]byte{}
	}
	return s.kcv
}

// Close overwrites the key with zeros; every later use of the key fails with
// ErrInvalidKey, except KCV. Closing twice, or closing a nil key, is harmless.
// The error is always nil; the signature is that of io.Closer.
func (k *SecretKey) Close() error {
	s := k.secret()
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open = false
	clear(s.bytes[:])
	return nil
}

// secretKeyText is what a key prints as.
const secretKeyText = "hh.SecretKey(***)"

// Format prints a placeholder for every verb that fmt hands to a Formatter, so
// that a key never reaches a log through the fmt package. The verbs %T and %p,
// which fmt answers itself, give the type and the address.
func (k *SecretKey) Format(f fmt.State, verb rune) {
	io.WriteString(f, secretKeyText)
}

// String returns the placeholder that Format prints, for the printers that
// look for a fmt.Stringer and not for a fmt.Formatter.
func (k *SecretKey) String() string { return secretKeyText }

// keyedFingerprint computes HMAC-SHA-256(K, M2) of SPEC.md section 4.
func (k *SecretKey) keyedFingerprint(d BaseDigest) ([FingerprintSize]byte, error) {
	var fp [FingerprintSize]byte
	s := k.secret()
	if s == nil {
		return fp, ErrInvalidKey
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.open {
		return fp, ErrInvalidKey
	}
	copy(fp[:], hmacSHA256(s.bytes[:], m2(d)))
	return fp, nil
}

// hmacSHA256 is HMAC of RFC 2104 over SHA-256.
func hmacSHA256(key, message []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return mac.Sum(nil)
}
