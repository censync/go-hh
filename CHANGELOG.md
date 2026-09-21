# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Each release names the hh-cpp release
its golden vectors were copied from. The algorithm itself is frozen and has no version: no
release changes a fingerprint, a pixel or an encoded byte.

## [1.0.0] - 2026-09-21

The first release. Golden vectors: hh-cpp v1.0.0.

### Added

- Package `github.com/censync/go-hh` (imported as `hh`, Go 1.21 or newer, godoc with runnable
  examples): `BaseDigest` from bytes, hexadecimal, text or UTF-8 bytes; `SecretKey` with a
  public key check value that outlives `Close`, wiped on `Close`, safe for concurrent use and
  never printed by `fmt`, `log/slog`, `encoding/json` or reflection; universal and keyed
  `Fingerprint` with `Layout` and the six-character `Tag`, and `ImportFingerprint` for a keyed
  HMAC computed elsewhere; `Render` with `RenderOptions` whose zero value is the default look:
  square and round shapes, keyed-mode frame markers, any background colour and alpha, frame
  alpha; `MeasureContrast`; `Image` with straight-alpha RGBA pixels, `NRGBA` for the standard
  image packages and the PNG, BMP and JPEG encoders.
- Text forms for configuration files and flags: `String` and `Parse...` for `Shape`, `Frame`,
  `RGB`, `Background`, `Opacity` and `Mode` in the names and hexadecimal forms of the
  specification, and the same types as `encoding.TextMarshaler` and `encoding.TextUnmarshaler`,
  so that `RenderOptions` passes through `encoding/json` unchanged and an unknown name is
  `ErrInvalidArgument`.
- Errors as values: `*hh.Error` sentinels that work with `errors.Is` and `errors.As` and carry
  the numeric codes of the specification. No function panics, not even the methods of a nil
  `*hh.Error`.
- Standard library only: `go.mod` has no requirements. SHA-256, HMAC, CRC-32 and Adler-32 come
  from the standard library; PBKDF2-HMAC-SHA-256, the fixed-Huffman deflate stream, PNG, BMP and
  the baseline JPEG encoder are written here, in integer arithmetic.
- Tests with the `testing` package: known answers of FIPS 180-4, RFC 4231 and RFC 7914 and a
  cross-check of PBKDF2 against `crypto/pbkdf2` where the toolchain has it; every record of the
  golden vectors of hh-cpp and every golden PNG; the rasteriser against a sample-by-sample
  transcription of the specification; decoding of every encoder's output with `image/png`,
  `image/jpeg` and `compress/zlib`; error order; deterministic robustness loops; concurrent use
  under the race detector; native fuzz targets for the parsers, the renderer and the encoders;
  benchmarks.
- `cmd/hh-cli` with the options of `hh_cli` of hh-cpp, `--batch` in the strict format of
  `hh_cli` and `--generate`, and `tools/crosscheck.sh`: differential test against hh-cpp with
  pseudo-random cases and the hand-made cases of `tools/edge-cases.txt`.
  `tools/update-vectors.sh` copies the vectors and writes `testdata/SOURCE`.

[1.0.0]: https://github.com/censync/go-hh/releases/tag/v1.0.0
