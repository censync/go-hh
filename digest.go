package hh

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"
)

const (
	// DigestSize is the size of a base digest in bytes.
	DigestSize = 32
	// MaxInputSize is the largest input in bytes (SPEC.md section 3).
	MaxInputSize = 1 << 20
)

// The constants of SPEC.md section 4.
const (
	domainTag         = "HumanizedHash"
	stretchSalt       = "HumanizedHash/stretch"
	stretchIterations = 16384

	stageDigest = 0x01
	stageKeyed  = 0x02
	stageKCV    = 0x03

	kindBinary = 0x00
	kindText   = 0x01
)

// BaseDigest is the stretched, public 32-byte value of an input (SPEC.md
// section 4). It is the only slow step, about 16 000 HMAC calls, so hosts cache
// it per input; both modes and any key derive their fingerprint from it
// cheaply. It needs no protection.
//
// BaseDigest is an array: it is comparable, can be a map key and is stored as
// its 32 bytes (d[:]); ImportBaseDigest restores it.
type BaseDigest [DigestSize]byte

// String returns the digest as 64 lowercase hexadecimal digits.
func (d BaseDigest) String() string { return hex.EncodeToString(d[:]) }

// NewBaseDigest computes the base digest of a binary input: the bytes of an
// address, a public key or a hash, 1 to MaxInputSize of them. It fails with
// ErrEmptyInput or ErrInputTooLarge.
func NewBaseDigest(data []byte) (BaseDigest, error) {
	return baseDigest(kindBinary, data)
}

// BaseDigestFromHex computes the base digest of a binary input given as
// hexadecimal text: an optional "0x" or "0X", then an even, non-zero number of
// hexadecimal digits of either case. Every spelling of one address gives the
// same digest. It fails with ErrInvalidHex, or with ErrInputTooLarge for
// well-formed text that denotes more than MaxInputSize bytes.
func BaseDigestFromHex(s string) (BaseDigest, error) {
	data, err := decodeHex(s)
	if err != nil {
		return BaseDigest{}, err
	}
	return baseDigest(kindBinary, data)
}

// BaseDigestFromText computes the base digest of a text input: the UTF-8 bytes
// of s, without normalisation. A text input and a binary input with equal bytes
// give different digests. It fails with ErrEmptyInput or ErrInputTooLarge, and
// after the length check with ErrInvalidArgument if s is not valid UTF-8: such
// a string is not the encoding of any text, and hashing it would give a picture
// that no other implementation shows for the text it was meant to be.
func BaseDigestFromText(s string) (BaseDigest, error) {
	if err := checkText(s); err != nil {
		return BaseDigest{}, err
	}
	return baseDigest(kindText, []byte(s))
}

// BaseDigestFromUTF8 computes the base digest of a text input from its UTF-8
// bytes, taken verbatim and not validated (SPEC.md section 3). It fails with
// ErrEmptyInput or ErrInputTooLarge.
func BaseDigestFromUTF8(text []byte) (BaseDigest, error) {
	return baseDigest(kindText, text)
}

// ImportBaseDigest restores a cached digest from its 32 bytes. It fails with
// ErrInvalidDigest for any other length.
func ImportBaseDigest(b []byte) (BaseDigest, error) {
	if len(b) != DigestSize {
		return BaseDigest{}, ErrInvalidDigest
	}
	var d BaseDigest
	copy(d[:], b)
	return d, nil
}

func checkLength(n int) error {
	if n == 0 {
		return ErrEmptyInput
	}
	if n > MaxInputSize {
		return ErrInputTooLarge
	}
	return nil
}

// checkText applies the checks of a text input in the order of SPEC.md
// section 3: the length first, then the form.
func checkText(s string) error {
	if err := checkLength(len(s)); err != nil {
		return err
	}
	if !utf8.ValidString(s) {
		return ErrInvalidArgument
	}
	return nil
}

// decodeHex decodes the hexadecimal form of SPEC.md section 3. The syntax is
// checked before the length, so an overlong string with a bad character is
// ErrInvalidHex.
func decodeHex(s string) ([]byte, error) {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	if len(s) == 0 || len(s)%2 != 0 {
		return nil, ErrInvalidHex
	}
	for i := 0; i < len(s); i++ {
		if hexValue(s[i]) < 0 {
			return nil, ErrInvalidHex
		}
	}
	if len(s)/2 > MaxInputSize {
		return nil, ErrInputTooLarge
	}
	out := make([]byte, len(s)/2)
	for i := range out {
		out[i] = byte(hexValue(s[2*i])<<4 | hexValue(s[2*i+1]))
	}
	return out, nil
}

func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// prefix returns DST || 00 || stage, the start of every hashed message.
func prefix(stage byte) []byte {
	return append([]byte(domainTag), 0x00, stage)
}

// m1Header returns M1 without the data: DST || 00 || 01 || kind || u32be(n).
func m1Header(kind byte, n int) []byte {
	return append(prefix(stageDigest), kind, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// m2 returns DST || 00 || 02 || s, the message of the keyed fingerprint.
func m2(d BaseDigest) []byte {
	return append(prefix(stageKeyed), d[:]...)
}

func baseDigest(kind byte, data []byte) (BaseDigest, error) {
	if err := checkLength(len(data)); err != nil {
		return BaseDigest{}, err
	}
	h := sha256.New()
	h.Write(m1Header(kind, len(data)))
	h.Write(data)
	d0 := h.Sum(nil)
	var d BaseDigest
	copy(d[:], pbkdf2SHA256(d0, []byte(stretchSalt), stretchIterations, DigestSize))
	return d, nil
}
