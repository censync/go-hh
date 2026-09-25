package hh

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestEverySpellingOfAnAddressGivesOneDigest(t *testing.T) {
	want, err := NewBaseDigest(unhex(t, testAddress))
	if err != nil {
		t.Fatal(err)
	}
	if want.String() != testDigestHex {
		t.Fatalf("digest %v", want)
	}
	for _, form := range []string{
		testAddress, "0x" + testAddress, "0X" + strings.ToUpper(testAddress), strings.ToLower(testAddress),
	} {
		if got, err := BaseDigestFromHex(form); err != nil || got != want {
			t.Errorf("%s: %v, %v", form, got, err)
		}
	}
}

func TestTextAndBinaryInputsAreSeparated(t *testing.T) {
	text, err := BaseDigestFromText(testBitcoinText)
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "dc705192e4a205d8c403ae7693290df45f09cec04116ad38f6140f349392548f" {
		t.Errorf("text digest %v", text)
	}
	if binary, _ := NewBaseDigest([]byte(testBitcoinText)); binary == text {
		t.Error("a text and a binary input with equal bytes share a digest")
	}
	if verbatim, _ := BaseDigestFromUTF8([]byte(testBitcoinText)); verbatim != text {
		t.Error("BaseDigestFromUTF8 differs from BaseDigestFromText")
	}
}

func TestInvalidInputs(t *testing.T) {
	tooLong := strings.Repeat("a", MaxInputSize+1)
	for _, c := range []struct {
		name string
		call func() (BaseDigest, error)
		want error
	}{
		{"no bytes", func() (BaseDigest, error) { return NewBaseDigest(nil) }, ErrEmptyInput},
		{"empty slice", func() (BaseDigest, error) { return NewBaseDigest([]byte{}) }, ErrEmptyInput},
		{"empty text", func() (BaseDigest, error) { return BaseDigestFromText("") }, ErrEmptyInput},
		{"empty UTF-8", func() (BaseDigest, error) { return BaseDigestFromUTF8(nil) }, ErrEmptyInput},
		{"long bytes", func() (BaseDigest, error) { return NewBaseDigest([]byte(tooLong)) }, ErrInputTooLarge},
		{"long text", func() (BaseDigest, error) { return BaseDigestFromText(tooLong) }, ErrInputTooLarge},
		{"long UTF-8", func() (BaseDigest, error) { return BaseDigestFromUTF8([]byte(tooLong)) }, ErrInputTooLarge},
		{"empty hex", func() (BaseDigest, error) { return BaseDigestFromHex("") }, ErrInvalidHex},
		{"bad hex", func() (BaseDigest, error) { return BaseDigestFromHex("0xzz") }, ErrInvalidHex},
		{"Arabic-Indic digits", func() (BaseDigest, error) { return BaseDigestFromHex("\u0661\u0662") }, ErrInvalidHex},
		{"fullwidth digits", func() (BaseDigest, error) { return BaseDigestFromHex("\uff11\uff12") }, ErrInvalidHex},
		{"short digest", func() (BaseDigest, error) { return ImportBaseDigest(make([]byte, 31)) }, ErrInvalidDigest},
		{"long digest", func() (BaseDigest, error) { return ImportBaseDigest(make([]byte, 33)) }, ErrInvalidDigest},
		{"no digest", func() (BaseDigest, error) { return ImportBaseDigest(nil) }, ErrInvalidDigest},
	} {
		digest, err := c.call()
		if err != c.want || !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if digest != (BaseDigest{}) {
			t.Errorf("%s: a digest came with the error", c.name)
		}
	}
	if _, err := NewBaseDigest(make([]byte, MaxInputSize)); err != nil {
		t.Errorf("the largest input: %v", err)
	}
}

// SPEC.md section 3: the length of a text is checked before its form.
func TestTextIsCheckedForLengthThenForForm(t *testing.T) {
	for _, bad := range []string{"\xff", "a\xc3", "\xed\xa0\x80", "\xc0\xaf", "\xf4\x90\x80\x80", "ok\x80"} {
		if _, err := BaseDigestFromText(bad); err != ErrInvalidArgument {
			t.Errorf("%q: %v", bad, err)
		}
		// The bytes themselves are a legitimate input of the verbatim entry point.
		if _, err := BaseDigestFromUTF8([]byte(bad)); err != nil {
			t.Errorf("%q verbatim: %v", bad, err)
		}
	}
	if err := checkText(strings.Repeat("a", MaxInputSize) + "\xff"); err != ErrInputTooLarge {
		t.Errorf("an overlong ill-formed text: %v", err)
	}
	if err := checkText(strings.Repeat("\u00e9", MaxInputSize/2) + "a"); err != ErrInputTooLarge {
		t.Errorf("a text of %d bytes: %v", MaxInputSize+1, err)
	}
	if err := checkText(strings.Repeat("\u00e9", MaxInputSize/2)); err != nil {
		t.Errorf("a text of %d bytes: %v", MaxInputSize, err)
	}
	// U+10348 is one code point and four UTF-8 bytes; U+10FFFF is the last one.
	supplementary, err := BaseDigestFromText("\U00010348")
	if verbatim, _ := BaseDigestFromUTF8(unhex(t, "f0908d88")); err != nil || supplementary != verbatim {
		t.Errorf("U+10348: %v", err)
	}
	if err := checkText("\U0010FFFF"); err != nil {
		t.Errorf("U+10FFFF: %v", err)
	}
}

// SPEC.md section 3: the syntax of a hexadecimal string is checked before its
// length.
func TestHexadecimalSyntaxIsCheckedBeforeItsLength(t *testing.T) {
	tooLong := strings.Repeat("a", 2*(MaxInputSize+1))
	if _, err := decodeHex(tooLong); err != ErrInputTooLarge {
		t.Errorf("too long: %v", err)
	}
	if _, err := decodeHex(tooLong[1:] + "g"); err != ErrInvalidHex {
		t.Errorf("too long with a bad digit: %v", err)
	}
	if _, err := decodeHex(tooLong[1:]); err != ErrInvalidHex {
		t.Errorf("too long and odd: %v", err)
	}
	if b, err := decodeHex("0x" + tooLong[2:]); err != nil || len(b) != MaxInputSize {
		t.Errorf("the longest: %d bytes, %v", len(b), err)
	}
}

func TestKeysAreChecked(t *testing.T) {
	for _, bad := range [][]byte{
		nil, {}, bytes.Repeat([]byte{1}, 31), bytes.Repeat([]byte{1}, 33), make([]byte, 32), bytes.Repeat([]byte{1}, 64),
	} {
		key, err := NewSecretKey(bad)
		if err != ErrInvalidKey || key != nil {
			t.Errorf("%d bytes: %v, %v", len(bad), key, err)
		}
	}
	key, err := NewSecretKey(testKeyBytes())
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	if kcv := key.KCV(); hex.EncodeToString(kcv[:]) != "6a5955cf" {
		t.Errorf("KCV %x", kcv)
	}
}

func TestAKeyCopiesItsBytesAndIsUnusableOnceClosed(t *testing.T) {
	raw := testKeyBytes()
	key, err := NewSecretKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	clear(raw) // the caller wipes its own slice; the key is not affected
	digest, _ := ImportBaseDigest(unhex(t, testDigestHex))
	keyed, err := Keyed(digest, key)
	if err != nil {
		t.Fatal(err)
	}
	if b := keyed.Bytes(); hex.EncodeToString(b[:]) != testKeyedHex || keyed.Mode() != ModeKeyed {
		t.Errorf("keyed fingerprint %x", b)
	}
	if err := key.Close(); err != nil {
		t.Error(err)
	}
	if err := key.Close(); err != nil {
		t.Error(err)
	}
	if state := key.secret(); state.bytes != [KeySize]byte{} || state.open {
		t.Error("Close left key bytes behind")
	}
	if fp, err := Keyed(digest, key); err != ErrInvalidKey || !fp.IsZero() {
		t.Errorf("a closed key: %v, %v", fp, err)
	}
	if kcv := key.KCV(); hex.EncodeToString(kcv[:]) != "6a5955cf" {
		t.Errorf("KCV after Close %x", kcv)
	}
}

func TestANilKeyIsInvalid(t *testing.T) {
	var key *SecretKey
	if _, err := Keyed(BaseDigest{1}, key); err != ErrInvalidKey {
		t.Errorf("Keyed: %v", err)
	}
	if key.KCV() != [KCVSize]byte{} {
		t.Error("KCV of a nil key")
	}
	if err := key.Close(); err != nil {
		t.Error(err)
	}
}

func TestFingerprintsCompareByBytesAndMode(t *testing.T) {
	digest, _ := ImportBaseDigest(unhex(t, testDigestHex))
	universal := Universal(digest)
	if universal.Mode() != ModeUniversal || universal.Tag() != "TKSPVH" || universal.IsZero() {
		t.Errorf("universal: %v %s", universal.Mode(), universal.Tag())
	}
	if universal.Bytes() != [FingerprintSize]byte(digest) {
		t.Error("the universal fingerprint is not the base digest")
	}
	if universal != mustFingerprint(t, testDigestHex, ModeUniversal) {
		t.Error("equal fingerprints differ")
	}
	if universal == mustFingerprint(t, testDigestHex, ModeKeyed) {
		t.Error("the mode is not part of the comparison")
	}
	seen := map[Fingerprint]bool{universal: true}
	if !seen[mustFingerprint(t, testDigestHex, ModeUniversal)] {
		t.Error("a fingerprint is not usable as a map key")
	}
	for _, c := range []struct {
		n    int
		mode Mode
	}{{16, ModeKeyed}, {64, ModeKeyed}, {0, ModeUniversal}, {32, 0}, {32, 3}, {32, 255}} {
		if fp, err := ImportFingerprint(make([]byte, c.n), c.mode); err != ErrInvalidFingerprint || !fp.IsZero() {
			t.Errorf("%d bytes, mode %d: %v", c.n, c.mode, err)
		}
	}
	// An import copies.
	raw := unhex(t, testDigestHex)
	imported, _ := ImportFingerprint(raw, ModeKeyed)
	clear(raw)
	if b := imported.Bytes(); hex.EncodeToString(b[:]) != testDigestHex {
		t.Error("the fingerprint shares the caller's slice")
	}
}

func TestTheZeroFingerprint(t *testing.T) {
	var zero Fingerprint
	if !zero.IsZero() || zero.Mode() != 0 || zero.Mode().String() != "invalid" || zero.Tag() != "000000" {
		t.Errorf("zero fingerprint: %v %s", zero.Mode(), zero.Tag())
	}
	if l := zero.Layout(); l.Cells != [16]Cell{} || l.Palette != palette || l.FrameColor != frameColor {
		t.Errorf("layout of the zero fingerprint: %+v", l)
	}
	if img, err := Render(zero, 64, RenderOptions{}); err != ErrInvalidFingerprint || img != nil {
		t.Errorf("render of the zero fingerprint: %v", err)
	}
}

func TestLayoutDescribesTheCells(t *testing.T) {
	raw := make([]byte, FingerprintSize)
	for i := 0; i < 16; i++ {
		raw[i] = byte(i/2<<5 | i%4<<3 | 7)
	}
	fp, _ := ImportFingerprint(raw, ModeKeyed)
	layout := fp.Layout()
	figures := []Figure{
		FigureNone, FigureNone, FigureSquare, FigureCircle,
		FigureTriangleUp, FigureTriangleRight, FigureTriangleDown, FigureTriangleLeft,
	}
	if layout.Mode != ModeKeyed {
		t.Errorf("mode %v", layout.Mode)
	}
	for i, cell := range layout.Cells {
		want := Cell{Figure: figures[i/2], Color: uint8(i % 4)}
		if want.Figure == FigureNone {
			want.Color = 0
		}
		if cell != want {
			t.Errorf("cell %d: %+v, want %+v", i, cell, want)
		}
	}
	wantPalette := [4]RGB{{0x7A, 0x96, 0xC5}, {0x89, 0x0A, 0xF0}, {0xC1, 0x04, 0x45}, {0xD4, 0x82, 0x00}}
	if layout.Palette != wantPalette || layout.FrameColor != (RGB{0x80, 0x80, 0x80}) {
		t.Errorf("palette %v, frame %v", layout.Palette, layout.FrameColor)
	}
}

// SPEC.md section 5.3, by hand: the top 30 bits of fp[16..19], five at a time.
func TestTag(t *testing.T) {
	for _, c := range []struct{ word, want string }{
		{"00000000", "000000"},
		{"00000003", "000000"}, // the two lowest bits are not used
		{"fffffffc", "ZZZZZZ"},
		{"00000004", "000001"},
		{"08000000", "100000"},
		{"08864298", "123456"}, // 00001 00010 00011 00100 00101 00110, then two unused bits
	} {
		raw := make([]byte, FingerprintSize)
		copy(raw[16:], unhex(t, c.word))
		fp, _ := ImportFingerprint(raw, ModeUniversal)
		if got := fp.Tag(); got != c.want {
			t.Errorf("%s: %s, want %s", c.word, got, c.want)
		}
	}
	if strings.ContainsAny(tagAlphabet, "ILOU") || len(tagAlphabet) != 32 {
		t.Error("the alphabet is not Crockford's")
	}
}

func TestTheZeroRenderOptionsAreTheDefault(t *testing.T) {
	var opts RenderOptions
	if opts.Shape != ShapeSquare || opts.Frame != FrameAutomatic || opts.Background.Color() != White ||
		opts.Background.Alpha() != 255 || opts.FrameAlpha.Alpha() != 255 {
		t.Errorf("zero options: %+v", opts)
	}
	if opts.Background != Opaque(White) || opts.FrameAlpha != Alpha(255) {
		t.Error("the default is not equal to its explicit form")
	}
	if Transparent().Alpha() != 0 || Translucent(RGB{1, 2, 3}, 4).Color() != (RGB{1, 2, 3}) || Alpha(0).Alpha() != 0 {
		t.Error("the accessors do not return what the constructors took")
	}
	if got := MeasureContrast(opts, White); got != (ContrastReport{300, 394}) {
		t.Errorf("contrast of the default: %+v", got)
	}
	transparent := RenderOptions{Background: Transparent()}
	if got := MeasureContrast(transparent, RGB{0x12, 0x12, 0x12}).FiguresX100; got != 300 {
		t.Errorf("over 121212: %d", got)
	}
	if got := MeasureContrast(transparent, RGB{0x9E, 0x9E, 0x9E}).FiguresX100; got != 112 {
		t.Errorf("over 9e9e9e: %d", got)
	}
}

func TestNamesRoundTrip(t *testing.T) {
	for s := ShapeSquare; s <= ShapeRound; s++ {
		if got, err := ParseShape(s.String()); err != nil || got != s {
			t.Errorf("shape %v: %v, %v", s, got, err)
		}
	}
	for f := FrameAutomatic; f <= FrameGaps; f++ {
		if got, err := ParseFrame(f.String()); err != nil || got != f {
			t.Errorf("frame %v: %v, %v", f, got, err)
		}
	}
	if Shape(2).String() != "invalid" || Frame(10).String() != "invalid" || Figure(7).String() != "invalid" ||
		Mode(0).String() != "invalid" || ModeKeyed.String() != "keyed" || FigureTriangleLeft.String() != "triangle_left" {
		t.Error("names of values outside the tables")
	}
	for _, bad := range []string{"", "Square", "square ", "double_line", "invalid", "0"} {
		if _, err := ParseShape(bad); err != ErrInvalidArgument {
			t.Errorf("ParseShape(%q): %v", bad, err)
		}
		if _, err := ParseFrame(bad); err != ErrInvalidArgument {
			t.Errorf("ParseFrame(%q): %v", bad, err)
		}
	}
	if c, err := ParseRGB("7A96c5"); err != nil || c != (RGB{0x7A, 0x96, 0xC5}) || c.String() != "7a96c5" {
		t.Errorf("ParseRGB: %v, %v", c, err)
	}
	if b, err := ParseBackground("12345680"); err != nil || b != Translucent(RGB{0x12, 0x34, 0x56}, 0x80) ||
		b.String() != "12345680" {
		t.Errorf("ParseBackground: %v, %v", b, err)
	}
	if (Background{}).String() != "ffffffff" || Transparent().String() != "00000000" {
		t.Error("names of the default and the transparent background")
	}
	for _, bad := range []string{"", "fff", "ffffff0", "#ffffff", "0xffff", "gggggg", "ffffffff", "ff ff ff", "\u0661\u0662\u0663"} {
		if _, err := ParseRGB(bad); err != ErrInvalidArgument {
			t.Errorf("ParseRGB(%q): %v", bad, err)
		}
	}
	for _, bad := range []string{"", "ffffff", "fffffffff", "#fffffff", "gggggggg", "ffffff f"} {
		if _, err := ParseBackground(bad); err != ErrInvalidArgument {
			t.Errorf("ParseBackground(%q): %v", bad, err)
		}
	}
}

func TestPixelsAreStraightAlphaRGBA(t *testing.T) {
	fp := mustFingerprint(t, testKeyedHex, ModeKeyed)
	img := mustRender(t, fp, 64, RenderOptions{})
	if img.Width != 64 || img.Height != 64 || len(img.Pix) != 64*64*4 {
		t.Fatalf("%d x %d, %d bytes", img.Width, img.Height, len(img.Pix))
	}
	if !bytes.Equal(img.Pix[:4], []byte{0, 0, 0, 0}) {
		t.Errorf("outside the rounded corner of a keyed picture: % x", img.Pix[:4])
	}
	centre := img.Pix[(31*64+31)*4:][:4] // S = 64: o = 6, t = 12, g = 1
	if !bytes.Equal(centre, []byte{255, 255, 255, 255}) {
		t.Errorf("the gutter at the centre: % x", centre)
	}

	nrgba := img.NRGBA()
	if nrgba.Bounds() != image.Rect(0, 0, 64, 64) || nrgba.Stride != 256 || &nrgba.Pix[0] != &img.Pix[0] {
		t.Errorf("NRGBA: %v, stride %d", nrgba.Bounds(), nrgba.Stride)
	}
	for _, at := range []image.Point{{0, 0}, {31, 31}, {20, 20}, {63, 40}} {
		p := img.Pix[(at.Y*64+at.X)*4:]
		if got, want := nrgba.NRGBAAt(at.X, at.Y), (color.NRGBA{p[0], p[1], p[2], p[3]}); got != want {
			t.Errorf("pixel %v: %v, want %v", at, got, want)
		}
	}
	// Two renders do not share memory.
	again := mustRender(t, fp, 64, RenderOptions{})
	clear(img.Pix)
	if bytes.Equal(again.Pix, img.Pix) {
		t.Error("two renders share their pixels")
	}
}

func TestNRGBAOfAnInvalidImageIsEmpty(t *testing.T) {
	for _, img := range []*Image{nil, {}, {Width: 2, Height: 2, Pix: make([]byte, 15)}, {Width: -1, Height: -1}} {
		if got := img.NRGBA(); got == nil || !got.Bounds().Empty() {
			t.Errorf("%+v: %v", img, got)
		}
	}
}

func TestErrorsCarryTheCodesOfTheSpecification(t *testing.T) {
	all := []*Error{
		ErrEmptyInput, ErrInputTooLarge, ErrInvalidHex, ErrInvalidKey, ErrInvalidDigest, ErrInvalidFingerprint,
		ErrInvalidSize, ErrInvalidFrame, ErrLowContrast, ErrInvalidQuality, ErrInvalidImage, ErrInvalidArgument,
	}
	codes := []Code{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 14}
	names := []string{
		"empty_input", "input_too_large", "invalid_hex", "invalid_key", "invalid_digest", "invalid_fingerprint",
		"invalid_size", "invalid_frame", "low_contrast", "invalid_quality", "invalid_image", "invalid_argument",
	}
	for i, e := range all {
		if e.Code() != codes[i] || e.Code().String() != names[i] {
			t.Errorf("%v: code %d %s", e, e.Code(), e.Code())
		}
		if !strings.HasPrefix(e.Error(), "hh: "+names[i]+": ") {
			t.Errorf("text %q", e.Error())
		}
		var target *Error
		wrapped := fmt.Errorf("picture: %w", error(e))
		if !errors.Is(wrapped, e) || !errors.As(wrapped, &target) || target.Code() != codes[i] {
			t.Errorf("%v does not survive wrapping", e)
		}
		for k, other := range all {
			if (k == i) != errors.Is(e, other) {
				t.Errorf("errors.Is(%v, %v)", e, other)
			}
		}
	}
	// errors.As leaves a nil *Error behind when it finds none; asking it is safe.
	var none *Error
	if errors.As(errors.New("another error"), &none) || none.Code() != CodeOK || none.Error() != "hh: ok" {
		t.Errorf("a nil *Error: %d %q", none.Code(), none.Error())
	}
	if CodeOK.String() != "ok" || CodeBufferTooSmall.String() != "buffer_too_small" || CodeOutOfMemory != 13 ||
		CodeOutOfMemory.String() != "out_of_memory" || Code(15).String() != "unknown" || Code(-1).String() != "unknown" {
		t.Error("the code table")
	}
}

func TestVersion(t *testing.T) {
	if Version != "1.1.0" {
		t.Errorf("version %s", Version)
	}
}
