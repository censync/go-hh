# Contributing to go-hh

go-hh is the Go implementation of Humanized Hash (`hh`). The C++17 repository hh-cpp is the
reference: it owns `docs/SPEC.md`, `docs/SECURITY.md` and the canonical golden vectors. This
repository carries a byte-identical copy of the vectors in `testdata/` and records the hh-cpp
release and the file hashes in `testdata/SOURCE`. The port follows `SPEC.md`, not the C++ source.

Bug reports and patches are welcome: open an issue or a pull request. Security problems are
reported privately, as `docs/SECURITY.md` of hh-cpp describes.

## The algorithm is frozen

Output is byte-identical with hh-cpp. A mismatch against the vectors is a bug here, never a reason
to change the vectors. The algorithm has no version and never changes; what the specification
leaves open (API shape, error texts, performance) may evolve under SemVer.

hh is a standalone library. Nothing here names a particular host application.

## Dependencies and license

- The standard library only, everywhere: the package, `cmd/hh-cli`, the tests. `go.mod` has no
  `require` line and there is no `go.sum`.
- The package uses the standard library where it is the vetted implementation of a standard
  primitive: `crypto/sha256`, `crypto/hmac`, `hash/crc32`, `hash/adler32`. Everything the
  specification pins byte for byte is written here: PBKDF2, the deflate stream, PNG, BMP, the
  baseline JPEG encoder, the rasteriser, the contrast arithmetic. The package must not import
  `image/png`, `image/jpeg`, `compress/...` or `math`. Tests may decode with `image/png`,
  `image/jpeg` and `compress/zlib`.
- The package imports these standard library packages and no others: `crypto/hmac`,
  `crypto/sha256`, `encoding`, `encoding/binary`, `encoding/hex`, `fmt`, `hash/adler32`,
  `hash/crc32`, `image`, `io`, `math/bits`, `strconv`, `sync`, `unicode/utf8`. Of these, `image`
  serves `Image.NRGBA` alone and `encoding` names the text interfaces. The hygiene test holds the
  same list and fails when the two differ or when an entry is no longer imported.
- No linters or formatters beyond the Go distribution: `gofmt`, `go vet`.
- License: MIT (`LICENSE`); contributions are accepted under it.

## Style

- Everything is English: code, comments, documentation, commit messages. No emoji.
- `gofmt`; lines up to about 120 columns. Go naming: MixedCaps, initialisms in one case (`PNG`,
  `KCV`), American `Color` in identifiers as in `image/color`.
- A doc comment on every exported identifier; the package comment is in `doc.go`; examples in
  `example_test.go` are part of the documentation and run as tests.
- Comments explain the code and cite the section of `SPEC.md` or the standard (FIPS 180-4,
  RFC 1951, ITU-T T.81).
- Unexported helpers in the package rather than an `internal` tree.

## Library rules

- Integer arithmetic only: no `float32`, `float64` or `math` in the package (`math/bits` is
  fine). Tests may use what they like.
- Every function is total: a result or one of the `Err...` values, never a panic. New error
  conditions use the codes of `SPEC.md` section 14.
- The zero value of `RenderOptions` is the default look; keep it so.
- Everything is safe for concurrent use and the package keeps no global mutable state.
- Buffers that held key material are wiped before release, as far as Go allows; what it does not
  allow is stated in the doc comment of `SecretKey` and in `docs/INTEGRATION.md`.
- A fast path in the rasteriser needs a test that it equals the sample-by-sample reference in
  `raster_test.go`.

## Build and test

- `gofmt -l .` prints nothing, `go vet ./...` is clean, `go test -race ./...` passes, on every Go
  release from 1.21 to the current one (`GOTOOLCHAIN=go1.21.13 go test ./...` runs an older one).
- `go test -run '^$' -bench . .` prints the timings; `go test -run '^$' -fuzz FuzzRender .` runs
  a fuzz target. A failing fuzz input lands in `testdata/fuzz/` and is committed with its fix.
- `tools/crosscheck.sh <hh_cli>` runs the differential test against hh-cpp: generated cases and
  the hand-made ones of `tools/edge-cases.txt`. That file is bytes, the same in every
  implementation of hh; do not edit it here.
- `tools/update-vectors.sh <hh-cpp checkout>` refreshes `testdata/` and `testdata/SOURCE`; never
  edit those files by hand.

## Commits

Atomic, imperative, lower case, for example "add pbkdf2 with rfc 7914 vectors". Every commit
builds and passes the tests.
