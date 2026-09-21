package hh

import (
	"crypto/hmac"
	"crypto/sha256"
)

// pbkdf2SHA256 is PBKDF2 of RFC 8018 section 5.2 with HMAC-SHA-256 as the
// pseudo-random function. iterations and keyLen must be positive.
func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	mac := hmac.New(sha256.New, password)
	out := make([]byte, 0, (keyLen+sha256.Size-1)/sha256.Size*sha256.Size)
	u := make([]byte, 0, sha256.Size)
	for block := uint32(1); len(out) < keyLen; block++ {
		// U1 = PRF(P, S || INT(i)); Uj = PRF(P, Uj-1); T = U1 xor ... xor Uc.
		mac.Reset()
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u = mac.Sum(u[:0])
		out = append(out, u...)
		t := out[len(out)-sha256.Size:]
		for i := 1; i < iterations; i++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(u[:0])
			for k := range t {
				t[k] ^= u[k]
			}
		}
	}
	return out[:keyLen]
}
