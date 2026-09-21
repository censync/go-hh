package hh

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// These tests reproduce every record of testdata/vectors.tsv, the golden
// vectors of hh-cpp (SPEC.md section 15). The file is a byte-identical copy; a
// mismatch is a bug in this package, never in the vectors.

// records returns the records of one type, split into fields.
func records(t *testing.T, kind string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "vectors.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	var out [][]string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if fields := strings.Split(line, "\t"); fields[0] == kind {
			out = append(out, fields)
		}
	}
	return out
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// vectorInput decodes "hex:<bytes>" or "fill:<byte>:<count>".
func vectorInput(t *testing.T, field string) []byte {
	t.Helper()
	if rest, ok := strings.CutPrefix(field, "hex:"); ok {
		return unhex(t, rest)
	}
	parts := strings.Split(field, ":")
	if len(parts) != 3 || parts[0] != "fill" {
		t.Fatalf("bad input %q", field)
	}
	return bytes.Repeat(unhex(t, parts[1]), atoi(t, parts[2]))
}

// vectorDigest computes the digest of a D or F record. A text goes through both
// text entry points when it is valid UTF-8.
func vectorDigest(t *testing.T, kind string, data []byte) (BaseDigest, error) {
	t.Helper()
	if kind != "text" {
		return NewBaseDigest(data)
	}
	d, err := BaseDigestFromUTF8(data)
	if utf8.Valid(data) || len(data) == 0 || len(data) > MaxInputSize {
		d2, err2 := BaseDigestFromText(string(data))
		if d2 != d || err2 != err {
			t.Fatalf("BaseDigestFromText and BaseDigestFromUTF8 disagree: %v %v, %v %v", d2, err2, d, err)
		}
	}
	return d, err
}

func cellsText(fp Fingerprint) string {
	var b strings.Builder
	for _, c := range fp.Layout().Cells {
		b.WriteString(strconv.Itoa(int(c.Figure)))
		b.WriteString(strconv.Itoa(int(c.Color)))
	}
	return b.String()
}

type renderCase struct {
	fp   Fingerprint
	size int
	opts RenderOptions
}

// vectorRender reads the fields fp, mode, size, shape, frame, background and
// frame alpha.
func vectorRender(t *testing.T, fp, mode, size, shape, frame, background, frameAlpha string) renderCase {
	t.Helper()
	var c renderCase
	var err error
	switch mode {
	case "universal":
		c.fp = mustFingerprint(t, fp, ModeUniversal)
	case "keyed":
		c.fp = mustFingerprint(t, fp, ModeKeyed)
	default:
		t.Fatalf("bad mode %q", mode)
	}
	if c.fp.Mode().String() != mode {
		t.Fatalf("mode %v, want %s", c.fp.Mode(), mode)
	}
	c.size = atoi(t, size)
	if c.opts.Shape, err = ParseShape(shape); err != nil {
		t.Fatalf("bad shape %q", shape)
	}
	if c.opts.Frame, err = ParseFrame(frame); err != nil {
		t.Fatalf("bad frame %q", frame)
	}
	if c.opts.Background, err = ParseBackground(background); err != nil {
		t.Fatalf("bad background %q", background)
	}
	c.opts.FrameAlpha = Alpha(uint8(atoi(t, frameAlpha)))
	return c
}

// patternImage builds the test images of SPEC.md section 15: "flat:<RRGGBBAA>",
// "noise:<seed>" and "opaque:<seed>".
func patternImage(t *testing.T, pattern string, width, height int) *Image {
	t.Helper()
	kind, argument, _ := strings.Cut(pattern, ":")
	pix := make([]byte, width*height*4)
	switch kind {
	case "flat":
		value := unhex(t, argument)
		for i := range pix {
			pix[i] = value[i%4]
		}
	case "noise", "opaque":
		x := uint32(atoi(t, argument))
		for i := range pix {
			// Arithmetic modulo 2^32 and a mask give the value modulo 2^31.
			x = (x*1103515245 + 12345) & 0x7FFFFFFF
			pix[i] = byte(x >> 16)
		}
		if kind == "opaque" {
			for i := 3; i < len(pix); i += 4 {
				pix[i] = 0xFF
			}
		}
	default:
		t.Fatalf("bad pattern %q", pattern)
	}
	return &Image{Width: width, Height: height, Pix: pix}
}

// checkEncodings compares the pixels and the three encodings with the hashes of
// an R or an I record.
func checkEncodings(t *testing.T, id string, img *Image, quality int, matte RGB, want []string) {
	t.Helper()
	png, err := img.EncodePNG()
	if err != nil {
		t.Fatalf("%s: png: %v", id, err)
	}
	bmp, err := img.EncodeBMP(matte)
	if err != nil {
		t.Fatalf("%s: bmp: %v", id, err)
	}
	jpeg, err := img.EncodeJPEG(quality, matte)
	if err != nil {
		t.Fatalf("%s: jpeg: %v", id, err)
	}
	for i, got := range [][]byte{img.Pix, png, bmp, jpeg} {
		if sha256Hex(got) != want[i] {
			t.Errorf("%s: %s differs", id, [...]string{"rgba", "png", "bmp", "jpeg"}[i])
		}
	}
}

func TestVectorsFileIsComplete(t *testing.T) {
	for kind, atLeast := range map[string]int{
		"D": 20, "H": 20, "K": 8, "C": 20, "R": 80, "E": 25, "G": 20, "W": 15, "I": 14, "F": 16,
	} {
		if n := len(records(t, kind)); n < atLeast {
			t.Errorf("%d %s records, want at least %d", n, kind, atLeast)
		}
	}
}

// testdata/SOURCE names the hh-cpp release the files came from and their
// SHA-256.
func TestVectorsSource(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "SOURCE"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	// The vectors come from a release of hh-cpp, never from an untagged commit.
	if !regexp.MustCompile(`(?m)^tag: v\d+\.\d+\.\d+$`).MatchString(text) {
		t.Error("testdata/SOURCE names no release tag")
	}
	if !regexp.MustCompile(`(?m)^commit: [0-9a-f]{40}$`).MatchString(text) {
		t.Error("testdata/SOURCE names no commit")
	}
	hashes := regexp.MustCompile(`(?m)^([0-9a-f]{64})  (\S+)$`).FindAllStringSubmatch(text, -1)
	if want := 1 + len(records(t, "G")); len(hashes) != want {
		t.Errorf("%d hashes, want %d", len(hashes), want)
	}
	for _, h := range hashes {
		file, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(h[2])))
		if err != nil {
			t.Error(err)
		} else if sha256Hex(file) != h[1] {
			t.Errorf("%s is not the file testdata/SOURCE records", h[2])
		}
	}
}

func TestVectorsDerivation(t *testing.T) {
	for _, r := range records(t, "D") {
		id := r[1]
		if len(r) != 16 {
			t.Fatalf("%s: %d fields", id, len(r))
		}
		data := vectorInput(t, r[3])
		digest, err := vectorDigest(t, r[2], data)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if digest.String() != r[7] {
			t.Errorf("%s: base digest %v", id, digest)
		}
		universal := Universal(digest)
		if universal.Mode() != ModeUniversal || hex.EncodeToString(universal.bytes[:]) != r[9] {
			t.Errorf("%s: universal fingerprint", id)
		}
		if got := cellsText(universal); got != r[12] {
			t.Errorf("%s: universal cells %s", id, got)
		}
		if got := universal.Tag(); got != r[14] {
			t.Errorf("%s: universal tag %s", id, got)
		}
		// M1 is written out unless it is longer than 256 bytes; d0 = SHA-256(M1).
		kind := byte(kindBinary)
		if r[2] == "text" {
			kind = kindText
		}
		m1 := append(m1Header(kind, len(data)), data...)
		if r[5] != "-" && hex.EncodeToString(m1) != r[5] {
			t.Errorf("%s: M1", id)
		}
		if sha256Hex(m1) != r[6] {
			t.Errorf("%s: d0", id)
		}
		if hex.EncodeToString(m2(digest)) != r[8] {
			t.Errorf("%s: M2", id)
		}
		if r[4] == "-" {
			if r[10] != "-" || r[11] != "-" || r[13] != "-" || r[15] != "-" {
				t.Errorf("%s: keyed values without a key", id)
			}
			continue
		}
		key, err := NewSecretKey(unhex(t, r[4]))
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if kcv := key.KCV(); hex.EncodeToString(kcv[:]) != r[11] {
			t.Errorf("%s: KCV %x", id, kcv)
		}
		keyed, err := Keyed(digest, key)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if keyed.Mode() != ModeKeyed || hex.EncodeToString(keyed.bytes[:]) != r[10] {
			t.Errorf("%s: keyed fingerprint", id)
		}
		if got := cellsText(keyed); got != r[13] {
			t.Errorf("%s: keyed cells %s", id, got)
		}
		if got := keyed.Tag(); got != r[15] {
			t.Errorf("%s: keyed tag %s", id, got)
		}
		key.Close()
	}
}

func TestVectorsHexadecimal(t *testing.T) {
	for _, r := range records(t, "H") {
		id, text, want := r[1], string(unhex(t, r[2])), r[3]
		digest, err := BaseDigestFromHex(text)
		if strings.Contains(want, "_") { // the name of an error
			if got := errorName(t, err); got != want {
				t.Errorf("%s: %q gives %s, want %s", id, text, got, want)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %q: %v", id, text, err)
			continue
		}
		decoded, err := decodeHex(text)
		if err != nil || hex.EncodeToString(decoded) != want {
			t.Errorf("%s: %q decodes to %x, %v", id, text, decoded, err)
		}
		if binary, _ := NewBaseDigest(unhex(t, want)); binary != digest {
			t.Errorf("%s: the digest differs from that of the bytes", id)
		}
	}
}

func TestVectorsKeys(t *testing.T) {
	for _, r := range records(t, "K") {
		var raw []byte
		if r[2] != "-" {
			raw = unhex(t, r[2])
		}
		key, err := NewSecretKey(raw)
		if err != nil {
			if got := errorName(t, err); got != r[3] {
				t.Errorf("%s: %s, want %s", r[1], got, r[3])
			}
			continue
		}
		if kcv := key.KCV(); hex.EncodeToString(kcv[:]) != r[3] {
			t.Errorf("%s: KCV %x, want %s", r[1], kcv, r[3])
		}
		key.Close()
	}
}

func TestVectorsContrast(t *testing.T) {
	for _, r := range records(t, "C") {
		background, err := ParseBackground(r[2])
		if err != nil {
			t.Fatal(err)
		}
		page, err := ParseRGB(r[4])
		if err != nil {
			t.Fatal(err)
		}
		opts := RenderOptions{Background: background, FrameAlpha: Alpha(uint8(atoi(t, r[3])))}
		want := ContrastReport{FiguresX100: atoi(t, r[5]), FrameX100: atoi(t, r[6])}
		if got := MeasureContrast(opts, page); got != want {
			t.Errorf("%s: %+v, want %+v", r[1], got, want)
		}
	}
}

func TestVectorsRenders(t *testing.T) {
	for _, r := range records(t, "R") {
		id := r[1]
		if len(r) != 15 {
			t.Fatalf("%s: %d fields", id, len(r))
		}
		c := vectorRender(t, r[2], r[3], r[4], r[5], r[6], r[7], r[8])
		matte, err := ParseRGB(r[10])
		if err != nil {
			t.Fatal(err)
		}
		img, err := Render(c.fp, c.size, c.opts)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		checkEncodings(t, id, img, atoi(t, r[9]), matte, r[11:15])
	}
}

func TestVectorsRenderErrors(t *testing.T) {
	for _, r := range records(t, "E") {
		c := vectorRender(t, r[2], r[3], r[4], r[5], r[6], r[7], r[8])
		img, err := Render(c.fp, c.size, c.opts)
		if got := errorName(t, err); got != r[9] {
			t.Errorf("%s: %s, want %s", r[1], got, r[9])
		}
		if img != nil {
			t.Errorf("%s: an image came with the error", r[1])
		}
	}
}

func TestVectorsSizeSweeps(t *testing.T) {
	for _, r := range records(t, "W") {
		if len(r) != 11 {
			t.Fatalf("%s: %d fields", r[1], len(r))
		}
		c := vectorRender(t, r[2], r[3], r[8], r[4], r[5], r[6], r[7])
		h := sha256.New()
		for size := c.size; size <= atoi(t, r[9]); size++ {
			h.Write(mustRender(t, c.fp, size, c.opts).Pix)
		}
		if hex.EncodeToString(h.Sum(nil)) != r[10] {
			t.Errorf("%s: the pixels differ", r[1])
		}
	}
}

func TestVectorsImages(t *testing.T) {
	for _, r := range records(t, "I") {
		if len(r) != 11 {
			t.Fatalf("%s: %d fields", r[1], len(r))
		}
		matte, err := ParseRGB(r[6])
		if err != nil {
			t.Fatal(err)
		}
		img := patternImage(t, r[4], atoi(t, r[2]), atoi(t, r[3]))
		checkEncodings(t, r[1], img, atoi(t, r[5]), matte, r[7:11])
	}
}

func TestVectorsFailures(t *testing.T) {
	for _, r := range records(t, "F") {
		id, want := r[1], r[len(r)-1]
		var errs []error
		switch r[2] {
		case "digest":
			_, err := vectorDigest(t, r[3], vectorInput(t, r[4]))
			errs = append(errs, err)
		case "jpeg":
			_, err := patternImage(t, "flat:ffffffff", 8, 8).EncodeJPEG(atoi(t, r[3]), White)
			errs = append(errs, err)
		case "image":
			img := &Image{Width: atoi(t, r[3]), Height: atoi(t, r[4]), Pix: bytes.Repeat([]byte{0x7F}, atoi(t, r[5]))}
			_, png := img.EncodePNG()
			_, bmp := img.EncodeBMP(White)
			_, jpeg := img.EncodeJPEG(DefaultJPEGQuality, White)
			errs = append(errs, png, bmp, jpeg)
		default:
			t.Fatalf("%s: unknown operation %q", id, r[2])
		}
		for _, err := range errs {
			if got := errorName(t, err); got != want {
				t.Errorf("%s: %s, want %s", id, got, want)
			}
		}
	}
}

func TestVectorsGoldenFiles(t *testing.T) {
	for _, r := range records(t, "G") {
		c := vectorRender(t, r[3], r[4], r[5], r[6], r[7], r[8], r[9])
		want, err := os.ReadFile(filepath.Join("testdata", "golden", r[2]))
		if err != nil {
			t.Fatal(err)
		}
		got, err := mustRender(t, c.fp, c.size, c.opts).EncodePNG()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs", r[2])
		}
	}
}
