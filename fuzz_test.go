package hh

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
	"unicode/utf8"
)

// Native fuzz targets. Without -fuzz they run their seed corpus as plain tests;
// "go test -fuzz FuzzRender -fuzztime 1m" looks for more.

func FuzzDecodeHex(f *testing.F) {
	for _, seed := range []string{
		"", "0x", "0X", "0", "00", "0x00ff", "0X5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED", testAddress,
		"abc", "0xabc", "0xzz", "0x0g", " ab", "ab ", "a b0", "0x 12", "x012", "00x1", "+1ab", "ab:cd", "0x0x12",
		"ab\x00cd", "\u00e9\u00e9", "\u0661\u0662", "\uff11\uff12", "0x\n", strings.Repeat("f", 300),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		checkHexAgainstOracle(t, text)
	})
}

func FuzzCheckText(f *testing.F) {
	for _, seed := range []string{
		"", "T", testBitcoinText, "  spaced  ", "caf\u00e9 \u20ac \U00010348", "\U0010FFFF", "a\x00b",
		"\xff", "a\xc3", "\xed\xa0\x80", "\xed\xb0\x80", "\xc0\xaf", "\xf4\x90\x80\x80", "\xf8\x88\x80\x80\x80", "ok\x80",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		var want error
		switch {
		case len(text) == 0:
			want = ErrEmptyInput
		case len(text) > MaxInputSize:
			want = ErrInputTooLarge
		case !utf8.ValidString(text) || strings.ToValidUTF8(text, "\ufffd") != text:
			want = ErrInvalidArgument
		}
		if err := checkText(text); err != want {
			t.Fatalf("%q: %v, want %v", text, err, want)
		}
	})
}

func FuzzParseOptions(f *testing.F) {
	for _, seed := range []string{
		"", "square", "round", "Round", "square ", "automatic", "none", "plain", "rounded", "chamfered", "double",
		"thick", "brackets", "ticks", "gaps", "double_line", "invalid", "ffffff", "FFFFFF", "7a96C5", "fffff", "fffffff",
		"ffffffff", "00000000", "12345680", "1234568", "#ffffff", "0xffffff", "ff ff ff", "gggggg", "\u0661\u0662\u0663",
		"0", "255", "256", "007", "+5", "-0", "2_5", "99999999999999999999", "universal", "keyed", "Keyed",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if shape, err := ParseShape(text); err == nil {
			if shape.String() != text {
				t.Errorf("shape %q parsed as %v", text, shape)
			}
		} else if err != ErrInvalidArgument || text == "square" || text == "round" {
			t.Errorf("shape %q: %v", text, err)
		}
		if frame, err := ParseFrame(text); err == nil {
			if frame.String() != text || frame > FrameGaps {
				t.Errorf("frame %q parsed as %v", text, frame)
			}
		} else if err != ErrInvalidArgument {
			t.Errorf("frame %q: %v", text, err)
		}
		hexDigits := strings.Trim(text, "0123456789abcdefABCDEF") == ""
		if c, err := ParseRGB(text); err == nil {
			if !hexDigits || len(text) != 6 || c.String() != strings.ToLower(text) {
				t.Errorf("colour %q parsed as %v", text, c)
			}
		} else if err != ErrInvalidArgument || (hexDigits && len(text) == 6) {
			t.Errorf("colour %q: %v", text, err)
		}
		if b, err := ParseBackground(text); err == nil {
			if !hexDigits || len(text) != 8 || b.String() != strings.ToLower(text) {
				t.Errorf("background %q parsed as %v", text, b)
			}
		} else if err != ErrInvalidArgument || (hexDigits && len(text) == 8) {
			t.Errorf("background %q: %v", text, err)
		}
		digits := text != "" && strings.Trim(text, "0123456789") == ""
		alpha := 0
		for i := 0; digits && i < len(text) && alpha <= 255; i++ {
			alpha = 10*alpha + int(text[i]-'0')
		}
		if o, err := ParseOpacity(text); err == nil {
			if !digits || alpha > 255 || int(o.Alpha()) != alpha || strings.TrimLeft(text, "0") != strings.TrimLeft(o.String(), "0") {
				t.Errorf("opacity %q parsed as %v", text, o)
			}
		} else if err != ErrInvalidArgument || o != (Opacity{}) || (digits && alpha <= 255) {
			t.Errorf("opacity %q: %v", text, err)
		}
		if m, err := ParseMode(text); err == nil {
			if m.String() != text {
				t.Errorf("mode %q parsed as %v", text, m)
			}
		} else if err != ErrInvalidArgument || text == "universal" || text == "keyed" {
			t.Errorf("mode %q: %v", text, err)
		}
		// UnmarshalText is the Parse function of the type; a refusal leaves the
		// value alone.
		shape, frame, colour, background, opacity, mode := ShapeRound, FrameTicks, RGB{1, 2, 3}, Transparent(),
			Alpha(9), ModeKeyed
		for name, c := range map[string]struct {
			got, want error
		}{
			"shape":      {shape.UnmarshalText([]byte(text)), errorOf(ParseShape(text))},
			"frame":      {frame.UnmarshalText([]byte(text)), errorOf(ParseFrame(text))},
			"colour":     {colour.UnmarshalText([]byte(text)), errorOf(ParseRGB(text))},
			"background": {background.UnmarshalText([]byte(text)), errorOf(ParseBackground(text))},
			"opacity":    {opacity.UnmarshalText([]byte(text)), errorOf(ParseOpacity(text))},
			"mode":       {mode.UnmarshalText([]byte(text)), errorOf(ParseMode(text))},
		} {
			if c.got != c.want {
				t.Errorf("%s %q: UnmarshalText %v, Parse %v", name, text, c.got, c.want)
			}
		}
		if (shape == ShapeRound) != (errorOf(ParseShape(text)) != nil || text == "round") ||
			(frame == FrameTicks) != (errorOf(ParseFrame(text)) != nil || text == "ticks") ||
			(colour == RGB{1, 2, 3}) != (errorOf(ParseRGB(text)) != nil || text == "010203") ||
			(background == Transparent()) != (errorOf(ParseBackground(text)) != nil || text == "00000000") ||
			(opacity == Alpha(9)) != (errorOf(ParseOpacity(text)) != nil || strings.TrimLeft(text, "0") == "9") ||
			(mode == ModeKeyed) != (errorOf(ParseMode(text)) != nil || text == "keyed") {
			t.Errorf("%q: UnmarshalText changed a value it should not have, or kept one it should not have", text)
		}
	})
}

func errorOf[T any](_ T, err error) error { return err }

func FuzzRender(f *testing.F) {
	digest, keyed := unhex(f, testDigestHex), unhex(f, testKeyedHex)
	f.Add(digest, uint8(1), 64, uint8(0), uint8(0), uint32(0xFFFFFFFF), uint8(255))
	f.Add(keyed, uint8(2), 64, uint8(0), uint8(0), uint32(0xFFFFFFFF), uint8(255))
	f.Add(keyed, uint8(2), 16, uint8(1), uint8(6), uint32(0x121212FF), uint8(128))
	f.Add(keyed, uint8(2), 17, uint8(1), uint8(5), uint32(0x00000000), uint8(0))
	f.Add(keyed, uint8(2), 18, uint8(1), uint8(5), uint32(0x12345640), uint8(77))
	f.Add(keyed, uint8(2), 48, uint8(0), uint8(4), uint32(0xFEDCBAC0), uint8(200))
	f.Add(keyed, uint8(2), 97, uint8(1), uint8(8), uint32(0x000000FF), uint8(255))
	f.Add(keyed, uint8(2), 33, uint8(1), uint8(9), uint32(0xF2F2F2FF), uint8(254))
	f.Add(keyed, uint8(2), 50, uint8(0), uint8(7), uint32(0xFFFFFF01), uint8(1))
	f.Add(digest, uint8(1), 64, uint8(0), uint8(3), uint32(0xFFFFFFFF), uint8(255))
	f.Add(digest, uint8(1), 64, uint8(0), uint8(0), uint32(0x9E9E9EFF), uint8(255))
	f.Add(digest, uint8(1), 15, uint8(0), uint8(0), uint32(0xFFFFFFFF), uint8(255))
	f.Add(digest, uint8(1), 1025, uint8(0), uint8(0), uint32(0xFFFFFFFF), uint8(255))
	f.Add(digest, uint8(1), -1, uint8(2), uint8(10), uint32(0), uint8(0))
	f.Add(digest[:31], uint8(1), 64, uint8(0), uint8(0), uint32(0xFFFFFFFF), uint8(255))
	f.Add(digest, uint8(0), 64, uint8(0), uint8(0), uint32(0xFFFFFFFF), uint8(255))
	f.Add([]byte{}, uint8(3), 0, uint8(255), uint8(255), uint32(0x80808080), uint8(9))
	f.Fuzz(func(t *testing.T, raw []byte, mode uint8, size int, shape, frame uint8, background uint32, frameAlpha uint8) {
		fp, err := ImportFingerprint(raw, Mode(mode))
		if err != nil {
			if err != ErrInvalidFingerprint || !fp.IsZero() || (len(raw) == FingerprintSize && (mode == 1 || mode == 2)) {
				t.Fatalf("import of %d bytes, mode %d: %v", len(raw), mode, err)
			}
		}
		opts := RenderOptions{
			Shape:      Shape(shape),
			Frame:      Frame(frame),
			Background: Translucent(RGB{byte(background >> 24), byte(background >> 16), byte(background >> 8)}, byte(background)),
			FrameAlpha: Alpha(frameAlpha),
		}
		img, err := Render(fp, size, opts)
		if err != nil {
			switch err {
			case ErrInvalidFingerprint, ErrInvalidArgument, ErrInvalidSize, ErrInvalidFrame, ErrLowContrast:
			default:
				t.Fatalf("an undocumented error: %v", err)
			}
			if img != nil {
				t.Fatal("an image came with the error")
			}
			return
		}
		if img.Width != size || img.Height != size || len(img.Pix) != 4*size*size {
			t.Fatalf("size %d: %d x %d, %d bytes", size, img.Width, img.Height, len(img.Pix))
		}
		// The sample-by-sample reference is slow; small pictures show every case.
		if size <= 72 {
			resolved := resolveFrame(opts.Frame, fp.Mode(), opts.Shape)
			want := referenceRender(fp, size, opts.Shape, resolved, opts.Background.Color(),
				int(opts.Background.Alpha()), int(frameAlpha))
			if !bytes.Equal(img.Pix, want) {
				t.Fatalf("size %d %+v: the pixels differ from the reference", size, opts)
			}
		}
	})
}

func FuzzEncode(f *testing.F) {
	f.Add(1, 1, []byte{1, 2, 3, 4}, 92, uint32(0xFFFFFF))
	f.Add(3, 5, bytes.Repeat([]byte{0x7A, 0x96, 0xC5, 0xFF}, 15), 50, uint32(0x0048FF))
	f.Add(9, 8, bytes.Repeat([]byte{0, 0, 0, 0, 255, 255, 255, 255, 1, 2, 3, 128}, 24), 100, uint32(0x121212))
	f.Add(2, 2, make([]byte, 15), 92, uint32(0))
	f.Add(0, 1, []byte{}, 92, uint32(0))
	f.Add(-1, -4, []byte{1, 2, 3, 4}, 92, uint32(0))
	f.Add(4097, 1, []byte{}, 49, uint32(0))
	f.Add(2, 2, make([]byte, 16), 101, uint32(0))
	f.Fuzz(func(t *testing.T, width, height int, pix []byte, quality int, matte uint32) {
		img := &Image{Width: width, Height: height, Pix: pix}
		m := RGB{byte(matte >> 16), byte(matte >> 8), byte(matte)}
		valid := width >= 1 && width <= MaxEncodeDimension && height >= 1 && height <= MaxEncodeDimension &&
			len(pix) == 4*width*height
		pngData, errPNG := img.EncodePNG()
		bmpData, errBMP := img.EncodeBMP(m)
		jpegData, errJPEG := img.EncodeJPEG(quality, m)
		if !valid {
			if errPNG != ErrInvalidImage || errBMP != ErrInvalidImage || errJPEG != ErrInvalidImage {
				t.Fatalf("%d x %d with %d bytes: %v, %v, %v", width, height, len(pix), errPNG, errBMP, errJPEG)
			}
			return
		}
		if errPNG != nil || errBMP != nil || len(bmpData) != 54+(3*width+3)/4*4*height {
			t.Fatalf("%d x %d: %v, %v, %d bytes of BMP", width, height, errPNG, errBMP, len(bmpData))
		}
		decoded, err := png.Decode(bytes.NewReader(pngData))
		if err != nil {
			t.Fatalf("%d x %d: %v", width, height, err)
		}
		switch d := decoded.(type) {
		case *image.RGBA:
			if !bytes.Equal(d.Pix, pix) {
				t.Fatal("the PNG decodes to other pixels")
			}
		case *image.NRGBA:
			if !bytes.Equal(d.Pix, pix) {
				t.Fatal("the PNG decodes to other pixels")
			}
		default:
			t.Fatalf("the PNG decodes as %T", decoded)
		}
		if quality < MinJPEGQuality || quality > MaxJPEGQuality {
			if errJPEG != ErrInvalidQuality {
				t.Fatalf("quality %d: %v", quality, errJPEG)
			}
			return
		}
		if errJPEG != nil {
			t.Fatal(errJPEG)
		}
		if config, err := jpeg.DecodeConfig(bytes.NewReader(jpegData)); err != nil || config.Width != width || config.Height != height {
			t.Fatalf("%d x %d: the JPEG header reads %+v, %v", width, height, config, err)
		}
		if _, err := jpeg.Decode(bytes.NewReader(jpegData)); err != nil {
			t.Fatalf("%d x %d: %v", width, height, err)
		}
	})
}
