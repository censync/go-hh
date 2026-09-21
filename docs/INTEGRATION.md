# Integration

How to put go-hh into a program. What to hash, which mode to show where and how large a picture
must be are the same for every implementation and are described once, in
[INTEGRATION.md of hh-cpp](https://github.com/censync/hh-cpp/blob/v1.0.0/docs/INTEGRATION.md)
(sections 1 to 4: recommended inputs per chain, the product rules, looks). This document adds
the Go side.

## 1. The three steps and what to cache

```go
digest, err := hh.BaseDigestFromHex(address)       // slow, public: cache the 32 bytes
fp, err := hh.Keyed(digest, key)                   // one HMAC; or hh.Universal(digest)
img, err := hh.Render(fp, size, hh.RenderOptions{}) // fast
```

- The base digest costs 16 384 iterations of PBKDF2-HMAC-SHA-256: 2 to 3 ms on a desktop core
  with SHA extensions, 6 to 10 ms without them, more on small ARM cores. Compute it once per
  input and keep it. It is public and needs no protection.
- `hh.BaseDigest` is a `[32]byte`: it is comparable, works as a map key, and `digest[:]` is what
  goes into a database column or a cache; `hh.ImportBaseDigest` brings it back.
- Changing the key, the mode, the size or the look never needs the slow step again. A 128-pixel
  render takes about 0.1 ms and its PNG about 0.2 ms, so pixels rarely need a cache of their own.
- Inputs: EVM addresses as their 20 bytes (`BaseDigestFromHex` takes the usual `0x...` text in
  any case), Sui, Aptos and Solana as 32 bytes, text-only formats such as Bitcoin addresses
  through `BaseDigestFromText`. The table is in section 1 of hh-cpp's INTEGRATION.md; independent
  programs agree on a picture only if they agree on the bytes.

A cache of digests for a long-running program; its zero value is ready to use:

```go
type digestCache struct {
    mu      sync.Mutex
    digests map[string]hh.BaseDigest // keyed by the decoded input, not by its spelling
}

func (c *digestCache) ofAddress(address []byte) (hh.BaseDigest, error) {
    c.mu.Lock()
    d, ok := c.digests[string(address)]
    c.mu.Unlock()
    if ok {
        return d, nil
    }
    d, err := hh.NewBaseDigest(address) // outside the lock: this is the slow call
    if err != nil {
        return hh.BaseDigest{}, err
    }
    c.mu.Lock()
    if c.digests == nil {
        c.digests = make(map[string]hh.BaseDigest)
    }
    c.digests[string(address)] = d
    c.mu.Unlock()
    return d, nil
}
```

Bound the map (an LRU, or a size check that drops it) if the inputs come from outside: every
entry is small, but an attacker can ask for many.

## 2. Serving a picture over `net/http`

The algorithm is frozen, so the universal picture of an input at a given size and look never
changes: it can be cached for ever by browsers and proxies.

```go
// GET /picture/0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed?size=128
func (s *server) picture(w http.ResponseWriter, r *http.Request) {
    address, err := hex.DecodeString(strings.TrimPrefix(r.PathValue("address"), "0x"))
    if err != nil || len(address) != 20 {
        http.Error(w, "not an EVM address", http.StatusBadRequest)
        return
    }
    size := 128
    if v := r.URL.Query().Get("size"); v != "" {
        if size, err = strconv.Atoi(v); err != nil {
            http.Error(w, "bad size", http.StatusBadRequest)
            return
        }
    }
    digest, err := s.cache.ofAddress(address)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    img, err := hh.Render(hh.Universal(digest), size, hh.RenderOptions{})
    if errors.Is(err, hh.ErrInvalidSize) {
        http.Error(w, "the size must be 16..1024", http.StatusBadRequest)
        return
    } else if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    png, err := img.EncodePNG()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "image/png")
    w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
    w.Header().Set("Content-Length", strconv.Itoa(len(png)))
    w.Write(png)
}
```

(`r.PathValue` needs Go 1.22; with Go 1.21 cut the address out of `r.URL.Path`.)

- Validate the input before the slow step, as above, and limit request rates: each new input
  costs a few milliseconds of CPU. `Render` itself refuses sizes above 1024.
- Serve the universal picture only. A keyed picture is private to the holder of the key and
  never belongs on a public URL; if a program serves keyed pictures to its own signed-in user,
  it sends `Cache-Control: private` and keeps the key on the server.
- The encoders are deterministic, so a strong `ETag` can be the hex of the digest plus the size
  and the look; no hash of the body is needed.
- A page should ask for the exact pixel size it shows (`size = CSS pixels * devicePixelRatio`)
  instead of scaling: the rasteriser is anti-aliased for the size it is asked for.

## 3. The standard image packages

`Image.Pix` has the layout of `image.NRGBA` (straight alpha), and `Image.NRGBA()` wraps it without
a copy:

```go
img, _ := hh.Render(fp, 96, hh.RenderOptions{Background: hh.Transparent()})

canvas := image.NewRGBA(image.Rect(0, 0, 640, 120))
draw.Draw(canvas, image.Rect(12, 12, 108, 108), img.NRGBA(), image.Point{}, draw.Over)
```

- Use `draw.Over` for pictures with transparency (a transparent background, rounded or chamfered
  corners, the round shape) and `draw.Src` to copy the pixels as they are.
- `image.RGBA` is premultiplied, `image.NRGBA` is not; `draw.Draw` converts. Do not hand `Pix` to
  code that expects premultiplied alpha.
- GUI toolkits take the same value: Fyne's `canvas.NewImageFromImage(img.NRGBA())`, Gio's
  `paint.NewImageOp(img.NRGBA())`, Ebitengine's `ebiten.NewImageFromImage(img.NRGBA())`. Render at
  the device-pixel size of the widget and switch off the toolkit's smoothing if it would scale.
- `png.Encode(w, img.NRGBA())` of the standard library gives a valid file too, but its bytes
  depend on the Go release. `img.EncodePNG()` gives the same bytes in every implementation of hh
  and on every release, which is what a test, an `ETag` or a signature wants. The same holds for
  `EncodeJPEG`; prefer PNG, because JPEG rings on flat colour edges.

## 4. Keyed mode and the key

```go
key, err := hh.NewSecretKey(keyBytes) // exactly 32 bytes, not all zero
if err != nil {
    return err
}
defer key.Close()
clear(keyBytes) // the key holds a copy; wipe yours

fp, err := hh.Keyed(digest, key)
```

- The key is 32 uniformly random bytes or the output of a key derivation function; there is no
  passphrase form. A key derived from the application's master secret on a dedicated path (for
  example SLIP-0021 with the labels `"HumanizedHash"`, `"keyed"`, `"0"`) needs no storage and
  survives a restore.
- Store `key.KCV()` beside cached data. When the value differs from the stored one, the key, and
  with it every private picture, has changed: stop and explain, do not re-key silently. The
  value reveals nothing useful about the key and stays readable after `Close`.
- One `*hh.SecretKey` serves any number of goroutines. `Close` may be called while others use
  the key: each call of `hh.Keyed` either finishes with the right fingerprint or fails with
  `hh.ErrInvalidKey`. After `Close` the key check value is all that is left of the key. Pass the
  pointer; a copy of the struct is a second handle on the same key, and closing either closes
  both.
- No printer shows the key bytes. A `*hh.SecretKey` prints as `hh.SecretKey(***)` with every
  `fmt` verb (`%T` and `%p`, which `fmt` answers itself, give the type and the address) and
  through `String`. A dereferenced or copied `SecretKey`, or one in an unexported field of your
  struct, prints as the address of a function: the state sits behind a function value, where
  neither `fmt` nor a package that dumps unexported fields by reflection can follow.
  `encoding/json` writes `{}`, and so does `log/slog` with a JSON handler.

What `Close` can and cannot do. It overwrites the 32 bytes the `SecretKey` holds. Go gives a
library no more than that: `crypto/hmac` copies the padded key into buffers of its own while a
fingerprint is computed and leaves them to the garbage collector; the runtime may have copied
values when it grew a stack; nothing stops the operating system from swapping a page out or
writing it into a core dump. Your own `keyBytes`, and whatever produced them, are copies too. A
program that must keep the key out of the Go heap computes `HMAC-SHA-256(key, M2)` elsewhere (in
an HSM, a secure element, a separate process, or hh-cpp through cgo) with
`M2 = "HumanizedHash" || 00 || 02 || digest`, and imports the 32 result bytes:

```go
fp, err := hh.ImportFingerprint(resultOfTheHSM, hh.ModeKeyed)
```

The shared golden vectors guarantee that every implementation renders the same picture from it.

## 5. Looks and contrast

The zero `hh.RenderOptions` is the default look: square, the automatic frame, opaque white.

```go
opts := hh.RenderOptions{
    Shape:      hh.ShapeRound,
    Frame:      hh.FrameDouble, // a keyed-mode marker: refused for a universal fingerprint
    Background: hh.Opaque(hh.RGB{R: 0x12, G: 0x12, B: 0x12}),
    FrameAlpha: hh.Alpha(200),
}
report := hh.MeasureContrast(opts, page) // page: the colour the program paints underneath
if report.FiguresX100 < 300 {
    // warn the user: figures may be hard to see
}
```

- `hh.FrameAutomatic` gives universal pictures no frame and keyed square pictures rounded
  corners. `FrameNone` and `FramePlain` are open to both modes; every other style marks a keyed
  picture. Use one style everywhere: the marker is only useful if it is familiar.
- `Render` refuses an opaque background with less than 2:1 against any palette colour
  (`hh.ErrLowContrast`). For a translucent background it cannot know what lies underneath, so
  measure with the page colour. On a dark theme use `hh.Transparent()` over a dark surface
  (`121212` or darker) or an opaque dark background; avoid mid greys and saturated surfaces.
- `hh.ParseShape`, `hh.ParseFrame`, `hh.ParseRGB`, `hh.ParseBackground`, `hh.ParseOpacity` and
  `hh.ParseMode` read the names, the hexadecimal forms and the decimal alpha that the
  specification and the command line tools use, for flags; the `String` methods write them.
- The same types are `encoding.TextMarshaler` and `encoding.TextUnmarshaler` in those text forms,
  so `hh.RenderOptions` goes through `encoding/json`, and through any other format built on the
  two interfaces, and comes back equal. A field that is missing keeps its default, and a text
  that the `Parse` function refuses is `hh.ErrInvalidArgument` (test with `errors.Is`), never a
  silent default:

  ```go
  file := []byte(`{"Shape": "round", "Frame": "double", "Background": "121212ff"}`)
  var opts hh.RenderOptions
  err := json.Unmarshal(file, &opts)
  written, err := json.Marshal(opts)
  // {"Shape":"round","Frame":"double","Background":"121212ff","FrameAlpha":"255"}
  ```

  Every value is a JSON string, the alpha too (`"200"`); a JSON number is an error of
  `encoding/json`.
- Sizes: 16 to 1024 pixels. A picture that backs a decision is at least 64 points, better 96,
  for the square shape and about a third larger for the round shape, whose cells are smaller.
  Smaller renders (32 to 48 points in list rows) are for recognition only. Offer
  `fp.Tag()` (`K7Q-M2X`) wherever text can be compared: it is certain where a picture is not.

## 6. Errors

Every error is one of the package's `Err...` values, all of type `*hh.Error`. Compare with `==`
or `errors.Is`; `errors.As` gives the numeric code of the specification, which is the same in
every implementation, and `Code.String` its name (`"invalid_hex"`).

| Call | Errors |
|---|---|
| `NewBaseDigest`, `BaseDigestFromUTF8` | `ErrEmptyInput`, `ErrInputTooLarge` |
| `BaseDigestFromHex` | `ErrInvalidHex`, then `ErrInputTooLarge` |
| `BaseDigestFromText` | `ErrEmptyInput`, `ErrInputTooLarge`, then `ErrInvalidArgument` for a string that is not valid UTF-8 |
| `ImportBaseDigest` | `ErrInvalidDigest` |
| `NewSecretKey`; `Keyed` with a nil or closed key | `ErrInvalidKey` |
| `ImportFingerprint` | `ErrInvalidFingerprint` |
| `Render` | `ErrInvalidFingerprint` (the zero `Fingerprint`), `ErrInvalidArgument` (an unknown `Shape` or `Frame` value), then in the order of the specification `ErrInvalidSize`, `ErrInvalidFrame`, `ErrLowContrast`, `ErrInvalidSize` (no room for the cells) |
| `EncodePNG`, `EncodeBMP`, `EncodeJPEG` | `ErrInvalidImage`; `EncodeJPEG` then `ErrInvalidQuality` |
| `ParseShape`, `ParseFrame`, `ParseRGB`, `ParseBackground`, `ParseOpacity`, `ParseMode` and the `UnmarshalText` methods | `ErrInvalidArgument` |
| `MarshalText` of `Shape`, `Frame` and `Mode` | `ErrInvalidArgument` for a value outside the table |

`Universal`, `MeasureContrast`, `Tag`, `Layout` and `NRGBA` cannot fail. Nothing panics, whatever
the input; the methods of a nil `*hh.Error`, which `errors.As` leaves behind when it finds none,
answer `CodeOK` and `"hh: ok"`. The codes 12 (`buffer_too_small`) and 13 (`out_of_memory`) of the
C ABI exist as `hh.Code` constants only: this package allocates its results itself.

A Go string can hold any bytes. `BaseDigestFromText` refuses one that is not valid UTF-8 instead
of hashing it: such a string is not the encoding of any text, and another implementation given
"the same text" would hash other bytes and show another picture. A program that holds the UTF-8
bytes of a text and wants them taken verbatim, as the C++ reference takes them, uses
`BaseDigestFromUTF8`.

## 7. Concurrency and memory

The package keeps no global state, and every function may be called from any goroutine.
`BaseDigest`, `Fingerprint`, `RenderOptions` and `Layout` are plain values. A `*SecretKey`
synchronises its own use, `Close` included. An `*Image` is ordinary data: the encoders and
`NRGBA` only read it, so one image may be encoded from many goroutines, but nothing may write
`Pix` meanwhile. The test suite runs these cases under the race detector.

`Render` allocates `size * size * 4` bytes and a few small tables. The encoders allocate their
output; PNG also a working copy of the raw stream, about the size of the pixels.

## 8. Conformance

`go test ./...` reproduces every record of `testdata/vectors.tsv` and every file of
`testdata/golden/`, and compares the rasteriser with a sample-by-sample transcription of the
specification; `tools/crosscheck.sh` compares thousands of pseudo-random cases, valid and
invalid, and the hand-made cases of `tools/edge-cases.txt` with `hh_cli` of hh-cpp byte for byte.
A mismatch is a bug in this package, never a reason to change the vectors.
