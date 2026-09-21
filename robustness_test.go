package hh

import (
	"bytes"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
)

// Deterministic pseudo-random loops: any input gives a result or one of the
// documented errors, never anything else.

func TestRandomFingerprintsRenderWithRandomOptions(t *testing.T) {
	random := lcg(0xF00D)
	rendered := 0
	for i := 0; i < 1500; i++ {
		fp, err := ImportFingerprint(random.bytes(FingerprintSize), Mode(1+random.intn(2)))
		if err != nil {
			t.Fatal(err)
		}
		opts := RenderOptions{
			Shape:      Shape(random.intn(2)),
			Frame:      Frame(random.intn(10)),
			Background: Translucent(RGB{byte(random.next()), byte(random.next()), byte(random.next())}, byte(random.next())),
			FrameAlpha: Alpha(byte(random.next())),
		}
		if random.intn(3) == 0 {
			opts.Background = Opaque(opts.Background.Color())
		}
		size := 8 + random.intn(90)
		if random.intn(100) == 0 {
			size = random.intn(2000) - 100
		}
		img, err := Render(fp, size, opts)
		if err != nil {
			if img != nil || (err != ErrInvalidSize && err != ErrInvalidFrame && err != ErrLowContrast) {
				t.Fatalf("size %d %+v: %v", size, opts, err)
			}
			continue
		}
		rendered++
		if img.Width != size || img.Height != size || len(img.Pix) != size*size*4 {
			t.Fatalf("size %d: %d x %d, %d bytes", size, img.Width, img.Height, len(img.Pix))
		}
		for p := 0; p < len(img.Pix); p += 4 {
			// A transparent pixel is 00 00 00 00.
			if img.Pix[p+3] == 0 && (img.Pix[p] != 0 || img.Pix[p+1] != 0 || img.Pix[p+2] != 0) {
				t.Fatalf("size %d %+v: a transparent pixel has a colour", size, opts)
			}
		}
	}
	if rendered < 200 {
		t.Errorf("only %d renders succeeded", rendered)
	}
}

func TestHexadecimalParsingAgreesWithASimpleOracle(t *testing.T) {
	random := lcg(0xA11CE)
	const alphabet = "0123456789abcdefABCDEFxXgG -:\n"
	for i := 0; i < 4000; i++ {
		var b strings.Builder
		if random.intn(4) == 0 {
			b.WriteString([]string{"0x", "0X"}[random.intn(2)])
		}
		for n := random.intn(12); n > 0; n-- {
			if random.intn(10) < 9 {
				b.WriteByte(alphabet[random.intn(22)])
			} else {
				b.WriteByte(alphabet[random.intn(len(alphabet))])
			}
		}
		checkHexAgainstOracle(t, b.String())
	}
}

// checkHexAgainstOracle compares decodeHex with SPEC.md section 3 written as
// plainly as possible.
func checkHexAgainstOracle(t *testing.T, text string) {
	t.Helper()
	digits := text
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		digits = text[2:]
	}
	valid := digits != "" && len(digits)%2 == 0 && strings.Trim(digits, "0123456789abcdefABCDEF") == ""
	decoded, err := decodeHex(text)
	switch {
	case valid && len(digits)/2 > MaxInputSize:
		if err != ErrInputTooLarge {
			t.Fatalf("%q: %v", text, err)
		}
	case valid:
		if err != nil || !strings.EqualFold(digits, hex.EncodeToString(decoded)) {
			t.Fatalf("%q: %x, %v", text, decoded, err)
		}
	case err != ErrInvalidHex || decoded != nil:
		t.Fatalf("%q: %x, %v", text, decoded, err)
	}
}

func TestEncodersTakeAnyPixels(t *testing.T) {
	random := lcg(0xBEEF)
	for i := 0; i < 300; i++ {
		width, height := 1+random.intn(40), 1+random.intn(40)
		img := &Image{Width: width, Height: height, Pix: random.bytes(width * height * 4)}
		switch random.intn(3) {
		case 0:
			for p := 3; p < len(img.Pix); p += 4 {
				img.Pix[p] = 255
			}
		case 1:
			// Long runs and repeats, which is what the deflate matcher looks for.
			for p := 4; p < len(img.Pix); p++ {
				if random.intn(8) != 0 {
					img.Pix[p] = img.Pix[p-4]
				}
			}
		}
		before := append([]byte{}, img.Pix...)
		matte := RGB{byte(random.next()), byte(random.next()), byte(random.next())}
		png, errPNG := img.EncodePNG()
		bmp, errBMP := img.EncodeBMP(matte)
		jpeg, errJPEG := img.EncodeJPEG(50+random.intn(51), matte)
		if errPNG != nil || errBMP != nil || errJPEG != nil {
			t.Fatalf("%d x %d: %v, %v, %v", width, height, errPNG, errBMP, errJPEG)
		}
		if len(png) < 67 || len(bmp) != 54+(3*width+3)/4*4*height || len(jpeg) < 600 {
			t.Fatalf("%d x %d: %d, %d, %d bytes", width, height, len(png), len(bmp), len(jpeg))
		}
		if !bytes.Equal(before, img.Pix) {
			t.Fatal("an encoder changed the pixels")
		}
	}
}

// Everything is safe for concurrent use; run with -race. One key serves many
// goroutines and is closed while they work: each of them gets the right
// fingerprint or ErrInvalidKey, never one computed under a wiped key.
func TestConcurrentUse(t *testing.T) {
	digest, _ := ImportBaseDigest(unhex(t, testDigestHex))
	key, err := NewSecretKey(testKeyBytes())
	if err != nil {
		t.Fatal(err)
	}
	want := mustFingerprint(t, testKeyedHex, ModeKeyed)
	wantPNG, _ := mustRender(t, want, 64, RenderOptions{}).EncodePNG()
	shared := mustRender(t, want, 48, RenderOptions{Shape: ShapeRound})

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if i == 20 && worker == 0 {
					key.Close()
				}
				fp, err := Keyed(digest, key)
				if err == ErrInvalidKey {
					fp = want
				} else if err != nil || fp != want {
					t.Errorf("worker %d: %v, %v", worker, fp, err)
					return
				}
				_ = key.KCV()
				img, err := Render(fp, 64, RenderOptions{})
				if err != nil {
					t.Errorf("worker %d: %v", worker, err)
					return
				}
				if png, _ := img.EncodePNG(); !bytes.Equal(png, wantPNG) {
					t.Errorf("worker %d: the PNG differs", worker)
				}
				// Encoders only read, so one image may be encoded from many goroutines.
				if _, err := shared.EncodeJPEG(DefaultJPEGQuality, White); err != nil {
					t.Error(err)
				}
				if _, err := shared.EncodeBMP(White); err != nil {
					t.Error(err)
				}
				_ = shared.NRGBA()
				_ = MeasureContrast(RenderOptions{}, White)
				_ = fp.Layout()
				_ = fp.Tag()
			}
		}(worker)
	}
	wg.Wait()
	if _, err := Keyed(digest, key); err != ErrInvalidKey {
		t.Errorf("after Close: %v", err)
	}
}

func TestConcurrentBaseDigests(t *testing.T) {
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if d, err := BaseDigestFromHex(testAddress); err != nil || d.String() != testDigestHex {
				t.Errorf("%v, %v", d, err)
			}
		}()
	}
	wg.Wait()
}
