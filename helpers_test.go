package hh

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

// The EIP-55 test address, its base digest and the keyed fingerprint under
// testKey: the values the examples of every implementation use.
const (
	testAddress     = "5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	testDigestHex   = "e212927148fcf76f6669c244a0db08bdd4f36dc50a378f6a1a3fe472807e7852"
	testKeyedHex    = "26ea8171aab23c8e1bf7c23417d33d6dba81d881af70edd2b0675348e080b478"
	testBitcoinText = "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"
)

// testKeyBytes returns the key 00 01 02 ... 1f.
func testKeyBytes() []byte {
	b := make([]byte, KeySize)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func unhex(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// errorName returns "ok" or the name the specification gives the error.
func errorName(t testing.TB, err error) string {
	t.Helper()
	if err == nil {
		return "ok"
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("%v is not an *hh.Error", err)
	}
	return e.Code().String()
}

func mustFingerprint(t testing.TB, fpHex string, mode Mode) Fingerprint {
	t.Helper()
	fp, err := ImportFingerprint(unhex(t, fpHex), mode)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func mustRender(t testing.TB, fp Fingerprint, size int, opts RenderOptions) *Image {
	t.Helper()
	img, err := Render(fp, size, opts)
	if err != nil {
		t.Fatalf("render %d %v: %v", size, opts, err)
	}
	return img
}

// lcg is a small deterministic generator for the robustness loops.
type lcg uint64

func (g *lcg) next() uint32 {
	*g = *g*6364136223846793005 + 1442695040888963407
	return uint32(*g >> 33)
}

func (g *lcg) intn(n int) int { return int(g.next() % uint32(n)) }

func (g *lcg) bytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(g.next())
	}
	return b
}
