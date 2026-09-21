// Package hh implements Humanized Hash: it turns a blockchain address, a public
// key or any hash into a small deterministic picture that a person can compare
// at a glance, a 4 x 4 matrix of solid squares, circles and triangles in four
// colours. It exists to catch address poisoning and clipboard substitution,
// which work because people check only the ends of a long string.
//
// A picture is made in three steps:
//
//	// Slow and public: cache the 32 bytes per input.
//	digest, err := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
//	// One HMAC at most: hh.Universal(digest) or hh.Keyed(digest, key).
//	fp := hh.Universal(digest)
//	// Fast.
//	img, err := hh.Render(fp, 128, hh.RenderOptions{})
//
// The base digest is the only slow value (16 384 iterations of
// PBKDF2-HMAC-SHA-256, a few milliseconds); it is public, and hosts cache its 32
// bytes per input. A universal fingerprint is the same for everyone and is what
// two parties compare. A keyed fingerprint is one HMAC of the base digest under
// a 32-byte SecretKey: without the key nobody can compute, and therefore nobody
// can search for, a lookalike. Inside an application keyed pictures are the
// default.
//
// An Image holds straight-alpha RGBA pixels; Image.NRGBA hands them to the
// standard image packages, and Image.EncodePNG, Image.EncodeBMP and
// Image.EncodeJPEG write files. Fingerprint.Tag gives six characters for a check
// that is certain where a picture is not, and Fingerprint.Layout describes the
// cells for hosts that draw vectors themselves.
//
// The algorithm is frozen and has no version. This package follows the
// specification of hh-cpp, the reference implementation
// (https://github.com/censync/hh-cpp, docs/SPEC.md), and produces the same
// fingerprints, pixels and PNG, BMP and JPEG bytes as every other
// implementation. It uses integer arithmetic only and nothing beyond the
// standard library.
//
// Every function is total: any input gives a result or one of the Err values of
// this package, which carry the numeric codes of the specification. Nothing
// panics. The package keeps no global state and is safe for concurrent use.
package hh

// Version is the version of this library. The algorithm itself has no version:
// no release changes a fingerprint, a pixel or an encoded byte.
const Version = "1.0.0"
