package hh

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"testing"
)

// The encoders are written here; the decoders of the standard library read what
// they write and give back the pixels.

// decoderImages returns renders and patterns that cover both PNG colour types,
// odd dimensions and partial JPEG blocks.
func decoderImages(t *testing.T) map[string]*Image {
	t.Helper()
	keyed := mustFingerprint(t, testKeyedHex, ModeKeyed)
	universal := mustFingerprint(t, testDigestHex, ModeUniversal)
	return map[string]*Image{
		"universal 128":   mustRender(t, universal, 128, RenderOptions{}),
		"keyed 128":       mustRender(t, keyed, 128, RenderOptions{}),
		"keyed round 97":  mustRender(t, keyed, 97, RenderOptions{Shape: ShapeRound, Frame: FrameTicks}),
		"transparent 64":  mustRender(t, universal, 64, RenderOptions{Background: Transparent()}),
		"translucent 33":  mustRender(t, keyed, 33, RenderOptions{Background: Translucent(RGB{0x12, 0x34, 0x56}, 0x80)}),
		"noise 17 x 9":    patternImage(t, "noise:5", 17, 9),
		"opaque 9 x 64":   patternImage(t, "opaque:6", 9, 64),
		"noise 1 x 1":     patternImage(t, "noise:1", 1, 1),
		"flat 300 x 2":    patternImage(t, "flat:7a96c5ff", 300, 2),
		"opaque 257 x 31": patternImage(t, "opaque:8", 257, 31),
	}
}

func TestPNGDecodesToTheSamePixels(t *testing.T) {
	for name, img := range decoderImages(t) {
		data, err := img.EncodePNG()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if decoded.Bounds() != image.Rect(0, 0, img.Width, img.Height) {
			t.Errorf("%s: bounds %v", name, decoded.Bounds())
			continue
		}
		opaque := true
		for p := 3; p < len(img.Pix); p += 4 {
			opaque = opaque && img.Pix[p] == 255
		}
		// Truecolour without alpha decodes as *image.RGBA, with alpha as *image.NRGBA.
		switch d := decoded.(type) {
		case *image.RGBA:
			if !opaque || !bytes.Equal(d.Pix, img.Pix) {
				t.Errorf("%s: the pixels differ (opaque %v)", name, opaque)
			}
		case *image.NRGBA:
			if opaque || !bytes.Equal(d.Pix, img.Pix) {
				t.Errorf("%s: the pixels differ (opaque %v)", name, opaque)
			}
		default:
			t.Errorf("%s: decoded as %T", name, decoded)
		}
	}
}

// The structure of SPEC.md section 11: four chunks, one zlib stream with one
// deflate block.
func TestPNGStructure(t *testing.T) {
	for name, img := range decoderImages(t) {
		data, _ := img.EncodePNG()
		if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			t.Fatalf("%s: signature", name)
		}
		var kinds []string
		var idat []byte
		for at := 8; at < len(data); {
			length := int(binary.BigEndian.Uint32(data[at:]))
			kind := string(data[at+4 : at+8])
			body := data[at+8 : at+8+length]
			if crc := binary.BigEndian.Uint32(data[at+8+length:]); crc != crc32.ChecksumIEEE(data[at+4:at+8+length]) {
				t.Errorf("%s: CRC of %s", name, kind)
			}
			kinds = append(kinds, kind)
			if kind == "IDAT" {
				idat = body
			}
			at += 12 + length
		}
		if len(kinds) != 4 || kinds[0] != "IHDR" || kinds[1] != "sRGB" || kinds[2] != "IDAT" || kinds[3] != "IEND" {
			t.Errorf("%s: chunks %v", name, kinds)
		}
		if idat[0] != 0x78 || idat[1] != 0x01 || idat[2]&7 != 3 {
			t.Errorf("%s: zlib header % x, first block bits %03b", name, idat[:2], idat[2]&7)
		}
		r, err := zlib.NewReader(bytes.NewReader(idat))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		raw, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if bpp := (len(raw)/img.Height - 1) / img.Width; (bpp != 3 && bpp != 4) || len(raw) != img.Height*(1+img.Width*bpp) {
			t.Errorf("%s: %d raw bytes", name, len(raw))
		}
		for y := 0; y < img.Height; y++ {
			if raw[y*len(raw)/img.Height] != 0 {
				t.Errorf("%s: row %d has the filter %d", name, y, raw[y*len(raw)/img.Height])
			}
		}
	}
}

// A BMP of SPEC.md section 12 read back: bottom-up rows of B, G, R, padded to
// four bytes, the pixels flattened over the matte.
func TestBMPReadsBackToTheFlattenedPixels(t *testing.T) {
	matte := RGB{0x00, 0x48, 0xFF}
	for name, img := range decoderImages(t) {
		data, err := img.EncodeBMP(matte)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		le := binary.LittleEndian
		row := (3*img.Width + 3) &^ 3
		if string(data[:2]) != "BM" || int(le.Uint32(data[2:])) != len(data) || le.Uint32(data[10:]) != 54 ||
			le.Uint32(data[14:]) != 40 || int(le.Uint32(data[18:])) != img.Width || int(le.Uint32(data[22:])) != img.Height ||
			le.Uint16(data[26:]) != 1 || le.Uint16(data[28:]) != 24 || le.Uint32(data[30:]) != 0 ||
			int(le.Uint32(data[34:])) != row*img.Height || len(data) != 54+row*img.Height {
			t.Errorf("%s: header % x", name, data[:54])
			continue
		}
		for y := 0; y < img.Height; y++ {
			line := data[54+(img.Height-1-y)*row:][:row]
			for x := 0; x < img.Width; x++ {
				p := img.Pix[(y*img.Width+x)*4:]
				a := int(p[3])
				want := [3]byte{
					byte((a*int(p[2]) + (255-a)*int(matte.B) + 127) / 255),
					byte((a*int(p[1]) + (255-a)*int(matte.G) + 127) / 255),
					byte((a*int(p[0]) + (255-a)*int(matte.R) + 127) / 255),
				}
				if [3]byte(line[3*x:3*x+3]) != want {
					t.Fatalf("%s: pixel %d,%d is % x, want % x", name, x, y, line[3*x:3*x+3], want)
				}
			}
			for _, pad := range line[3*img.Width:] {
				if pad != 0 {
					t.Fatalf("%s: row %d is padded with %d", name, y, pad)
				}
			}
		}
	}
}

// A baseline decoder reads the JPEG. The flat areas of a render come back
// almost exactly; noise does not, so only renders are compared.
func TestJPEGDecodes(t *testing.T) {
	for name, img := range decoderImages(t) {
		for _, quality := range []int{50, 92, 100} {
			data, err := img.EncodeJPEG(quality, White)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			config, err := jpeg.DecodeConfig(bytes.NewReader(data))
			if err != nil || config.Width != img.Width || config.Height != img.Height {
				t.Errorf("%s quality %d: config %+v, %v", name, quality, config, err)
			}
			decoded, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Errorf("%s quality %d: %v", name, quality, err)
				continue
			}
			ycbcr, ok := decoded.(*image.YCbCr)
			if !ok || ycbcr.SubsampleRatio != image.YCbCrSubsampleRatio444 {
				t.Errorf("%s quality %d: decoded as %T", name, quality, decoded)
			}
		}
	}

	// The centre of a filled cell, far from any edge, keeps its colour.
	fp := mustFingerprint(t, "5f"+testDigestHex[2:], ModeUniversal) // cell 0: a square of colour 3
	img := mustRender(t, fp, 128, RenderOptions{})
	data, _ := img.EncodeJPEG(DefaultJPEGQuality, White)
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decoded.At(23, 23).RGBA() // S = 128: the first cell spans 11..35
	for i, got := range []uint32{r >> 8, g >> 8, b >> 8} {
		want := uint32([]uint8{0xD4, 0x82, 0x00}[i])
		if got+4 < want || want+4 < got {
			t.Errorf("channel %d of the first cell: %d, want about %d", i, got, want)
		}
	}
}

// Every ff byte of the entropy-coded data is followed by 00, and the file ends
// with EOI (SPEC.md section 13.7).
func TestJPEGStuffing(t *testing.T) {
	data, err := patternImage(t, "noise:9", 64, 64).EncodeJPEG(100, White)
	if err != nil {
		t.Fatal(err)
	}
	sos := bytes.Index(data, []byte{0xFF, 0xDA})
	scan := data[sos+2+12 : len(data)-2]
	stuffed := 0
	for i, b := range scan {
		if b == 0xFF {
			if i+1 == len(scan) || scan[i+1] != 0x00 {
				t.Fatalf("ff at %d is not followed by 00", i)
			}
			stuffed++
		}
	}
	if stuffed == 0 {
		t.Error("the test image has no ff byte to stuff")
	}
	if !bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF, 0xE0}) || !bytes.HasSuffix(data, []byte{0xFF, 0xD9}) {
		t.Error("SOI, APP0 or EOI is missing")
	}
}

// The Huffman codes of ITU-T T.81 annex C are a prefix code: complete tables
// for DC, and for AC every symbol the encoder can emit.
func TestHuffmanTables(t *testing.T) {
	for i, table := range huffmanCodes {
		spec := huffmanSpecs[i]
		total := 0
		for _, n := range spec.counts {
			total += int(n)
		}
		if total != len(spec.symbols) {
			t.Errorf("table %02x: %d codes for %d symbols", spec.id, total, len(spec.symbols))
		}
		for _, s := range spec.symbols {
			if table[s].length == 0 {
				t.Errorf("table %02x: symbol %02x has no code", spec.id, s)
			}
			for _, o := range spec.symbols {
				a, b := table[s], table[o]
				if s != o && a.length <= b.length && b.code>>(b.length-a.length) == a.code {
					t.Errorf("table %02x: the code of %02x is a prefix of that of %02x", spec.id, s, o)
				}
			}
		}
		if spec.id&0x10 == 0 {
			continue
		}
		for run := 0; run < 16; run++ {
			for category := 1; category <= 10; category++ {
				if table[run*16+category].length == 0 {
					t.Errorf("table %02x: no code for run %d category %d", spec.id, run, category)
				}
			}
		}
		if table[0x00].length == 0 || table[0xF0].length == 0 {
			t.Errorf("table %02x: no code for EOB or ZRL", spec.id)
		}
	}
	seen := map[uint8]bool{}
	for _, k := range zigzag {
		seen[k] = true
	}
	if len(seen) != 64 {
		t.Error("zigzag is not a permutation")
	}
}
