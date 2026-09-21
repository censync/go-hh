package hh

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"hash/adler32"
	"hash/crc32"
	"strings"
	"testing"
)

// The primitives come from the standard library except PBKDF2, which is written
// here. The known answers pin down how this package uses them: the hash, the
// CRC polynomial, the argument order of HMAC.

// FIPS 180-4 examples and the usual long message.
func TestSHA256KnownAnswers(t *testing.T) {
	for _, c := range []struct{ message, want string }{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
			"248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"},
		{"abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmnoijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrstu",
			"cf5b16a778af8380036ce59e7b0492370b249b11e8f07a51afac45037afee9d1"},
		{strings.Repeat("a", 1000000), "cdc76e5c9914fb9281a1c7e284d73e67f1809a48a497200e046d39ccc7112cd0"},
	} {
		if got := sha256Hex([]byte(c.message)); got != c.want {
			t.Errorf("SHA-256 of %d bytes: %s", len(c.message), got)
		}
	}
}

// RFC 4231, all seven test cases. Case 5 truncates the output to 128 bits.
func TestHMACSHA256KnownAnswers(t *testing.T) {
	for i, c := range []struct {
		key, message []byte
		want         string
	}{
		{bytes.Repeat([]byte{0x0b}, 20), []byte("Hi There"),
			"b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"},
		{[]byte("Jefe"), []byte("what do ya want for nothing?"),
			"5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"},
		{bytes.Repeat([]byte{0xaa}, 20), bytes.Repeat([]byte{0xdd}, 50),
			"773ea91e36800e46854db8ebd09181a72959098b3ef8c122d9635514ced565fe"},
		{unhex(t, "0102030405060708090a0b0c0d0e0f10111213141516171819"), bytes.Repeat([]byte{0xcd}, 50),
			"82558a389a443c0ea4cc819899f2083a85f0faa3e578f8077a2e3ff46729665b"},
		{bytes.Repeat([]byte{0x0c}, 20), []byte("Test With Truncation"),
			"a3b6167473100ee06e0c796c2955552b"},
		{bytes.Repeat([]byte{0xaa}, 131), []byte("Test Using Larger Than Block-Size Key - Hash Key First"),
			"60e431591ee0b67f0d8a26aacbf5b77f8e0bc6213728c5140546040f0ee37f54"},
		{bytes.Repeat([]byte{0xaa}, 131), []byte("This is a test using a larger than block-size key and a larger than " +
			"block-size data. The key needs to be hashed before being used by the HMAC algorithm."),
			"9b09ffa71b942fcb27635fbcd5b0e944bfdc63644f0713938a7f51535c3a35e2"},
	} {
		got := hex.EncodeToString(hmacSHA256(c.key, c.message))
		if got[:len(c.want)] != c.want {
			t.Errorf("RFC 4231 case %d: %s", i+1, got)
		}
	}
}

func TestPBKDF2KnownAnswers(t *testing.T) {
	for _, c := range []struct {
		password, salt string
		iterations     int
		want           string
	}{
		// RFC 7914 section 11.
		{"passwd", "salt", 1,
			"55ac046e56e3089fec1691c22544b605f94185216dde0465e68b9d57c20dacbc" +
				"49ca9cccf179b645991664b39d77ef317c71b845b1e30bd509112041d3a19783"},
		{"Password", "NaCl", 80000,
			"4ddcd8f60b98be21830cee5ef22701f9641a4418d04c0414aeff08876b34ab56" +
				"a1d425a1225833549adb841b51c9b3176a272bdebba1d078478f62b397f33c8d"},
		// The vectors of RFC 6070 recomputed for SHA-256, as widely published.
		{"password", "salt", 1, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{"password", "salt", 2, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{"password", "salt", 4096, "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
		{"passwordPASSWORDpassword", "saltSALTsaltSALTsaltSALTsaltSALTsalt", 4096,
			"348c89dbcbd32b2f32d814b8116e84cf2b17347ebc1800181c4e2a1fb8dd53e1c635518c7dac47e9"},
		{"pass\x00word", "sa\x00lt", 4096, "89b69d0516f829893c696226650a8687"},
	} {
		want := unhex(t, c.want)
		got := pbkdf2SHA256([]byte(c.password), []byte(c.salt), c.iterations, len(want))
		if !bytes.Equal(got, want) {
			t.Errorf("PBKDF2(%q, %q, %d): %x", c.password, c.salt, c.iterations, got)
		}
	}
}

// Every length around the block boundaries, against the definition of RFC 8018
// section 5.2 written without reusing the MAC.
func TestPBKDF2OutputLengths(t *testing.T) {
	password, salt := []byte("p"), []byte("s")
	var long []byte
	for block := byte(1); block <= 3; block++ {
		u := hmacSHA256(password, append(append([]byte{}, salt...), 0, 0, 0, block))
		f := append([]byte{}, u...)
		for i := 1; i < 3; i++ {
			u = hmacSHA256(password, u)
			for k := range f {
				f[k] ^= u[k]
			}
		}
		long = append(long, f...)
	}
	for n := 1; n <= len(long); n++ {
		if got := pbkdf2SHA256(password, salt, 3, n); !bytes.Equal(got, long[:n]) {
			t.Fatalf("%d bytes: %x, want %x", n, got, long[:n])
		}
	}
}

// The stretching of SPEC.md section 4 for the EIP-55 test address.
func TestStretch(t *testing.T) {
	data := unhex(t, testAddress)
	d0 := sha256.Sum256(append(m1Header(kindBinary, len(data)), data...))
	s := pbkdf2SHA256(d0[:], []byte("HumanizedHash/stretch"), 16384, 32)
	if hex.EncodeToString(s) != testDigestHex {
		t.Errorf("s = %x", s)
	}
}

// The check values of CRC-32 (ISO 3309, as PNG uses it) and Adler-32 (RFC 1950).
func TestChecksums(t *testing.T) {
	if got := crc32.ChecksumIEEE([]byte("123456789")); got != 0xCBF43926 {
		t.Errorf("CRC-32 check value %08x", got)
	}
	if got := crc32.ChecksumIEEE([]byte("IEND")); got != 0xAE426082 {
		t.Errorf("CRC-32 of IEND %08x", got)
	}
	if got := adler32.Checksum([]byte("Wikipedia")); got != 0x11E60398 {
		t.Errorf("Adler-32 of Wikipedia %08x", got)
	}
	if got := adler32.Checksum(nil); got != 1 {
		t.Errorf("Adler-32 of nothing %08x", got)
	}
}
