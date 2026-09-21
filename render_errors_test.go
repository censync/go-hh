package hh

import "testing"

// SPEC.md section 6 fixes the order of the checks of a render: the size, the
// frame, the contrast, then the room for the cells. This package puts the zero
// Fingerprint and unknown enumeration values in front, as hh-cpp does.
func TestRenderErrorsComeInTheSpecifiedOrder(t *testing.T) {
	universal := mustFingerprint(t, testDigestHex, ModeUniversal)
	keyed := mustFingerprint(t, testKeyedHex, ModeKeyed)
	grey := Opaque(RGB{0x9E, 0x9E, 0x9E}) // 1.12:1 against the first palette colour
	for _, c := range []struct {
		name string
		fp   Fingerprint
		size int
		opts RenderOptions
		want error
	}{
		{"everything wrong", Fingerprint{}, 0, RenderOptions{Shape: 9, Frame: 99, Background: grey}, ErrInvalidFingerprint},
		{"unknown shape, bad size", universal, 0, RenderOptions{Shape: 2, Frame: FrameThick, Background: grey}, ErrInvalidArgument},
		{"unknown frame, bad size", universal, 0, RenderOptions{Frame: 10, Background: grey}, ErrInvalidArgument},
		{"bad size, frame and contrast", universal, 15, RenderOptions{Frame: FrameThick, Background: grey}, ErrInvalidSize},
		{"bad frame and contrast", universal, 64, RenderOptions{Frame: FrameThick, Background: grey}, ErrInvalidFrame},
		{"bad contrast, no room", keyed, 16, RenderOptions{Shape: ShapeRound, Frame: FrameThick, Background: grey}, ErrLowContrast},
		{"no room", keyed, 16, RenderOptions{Shape: ShapeRound, Frame: FrameThick}, ErrInvalidSize},
		{"no room at 17", keyed, 17, RenderOptions{Shape: ShapeRound, Frame: FrameDouble}, ErrInvalidSize},
		{"room at 18", keyed, 18, RenderOptions{Shape: ShapeRound, Frame: FrameDouble}, nil},
		{"negative size", universal, -128, RenderOptions{}, ErrInvalidSize},
		{"huge size", universal, 1 << 30, RenderOptions{}, ErrInvalidSize},
		{"smallest", universal, MinSize, RenderOptions{}, nil},
		{"largest", universal, MaxSize, RenderOptions{}, nil},
		{"one more", universal, MaxSize + 1, RenderOptions{}, ErrInvalidSize},
	} {
		img, err := Render(c.fp, c.size, c.opts)
		if err != c.want {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
		if (img == nil) != (err != nil) {
			t.Errorf("%s: image %v with error %v", c.name, img != nil, err)
		}
	}
}

// The table of SPEC.md section 6, in full.
func TestFrameTable(t *testing.T) {
	type row struct{ square, round, universal, keyed bool }
	table := map[Frame]row{
		FrameNone:      {true, true, true, true},
		FramePlain:     {true, true, true, true},
		FrameRounded:   {true, false, false, true},
		FrameChamfered: {true, false, false, true},
		FrameBrackets:  {true, false, false, true},
		FrameDouble:    {true, true, false, true},
		FrameThick:     {true, true, false, true},
		FrameTicks:     {false, true, false, true},
		FrameGaps:      {false, true, false, true},
	}
	for frame, r := range table {
		for _, mode := range []Mode{ModeUniversal, ModeKeyed} {
			for _, shape := range []Shape{ShapeSquare, ShapeRound} {
				allowed := (shape == ShapeSquare && r.square || shape == ShapeRound && r.round) &&
					(mode == ModeUniversal && r.universal || mode == ModeKeyed && r.keyed)
				_, err := Render(mustFingerprint(t, testDigestHex, mode), 64, RenderOptions{Shape: shape, Frame: frame})
				if allowed && err != nil || !allowed && err != ErrInvalidFrame {
					t.Errorf("%v %v %v: %v", frame, shape, mode, err)
				}
			}
		}
	}
	// Automatic is rounded for a keyed square picture and none otherwise.
	for _, c := range []struct {
		mode  Mode
		shape Shape
		same  Frame
	}{
		{ModeKeyed, ShapeSquare, FrameRounded}, {ModeKeyed, ShapeRound, FrameNone},
		{ModeUniversal, ShapeSquare, FrameNone}, {ModeUniversal, ShapeRound, FrameNone},
	} {
		fp := mustFingerprint(t, testDigestHex, c.mode)
		automatic := mustRender(t, fp, 64, RenderOptions{Shape: c.shape})
		explicit := mustRender(t, fp, 64, RenderOptions{Shape: c.shape, Frame: c.same})
		if string(automatic.Pix) != string(explicit.Pix) {
			t.Errorf("%v %v: automatic is not %v", c.mode, c.shape, c.same)
		}
	}
}

// Section 9: an opaque background is refused below 2:1, a translucent one never.
func TestContrastRule(t *testing.T) {
	fp := mustFingerprint(t, testDigestHex, ModeUniversal)
	for _, c := range []struct {
		background Background
		want       error
	}{
		{Opaque(White), nil},
		{Opaque(RGB{0x12, 0x12, 0x12}), nil},
		{Opaque(RGB{}), nil},
		{Opaque(RGB{0xF2, 0xF2, 0xF2}), nil},
		{Opaque(RGB{0x9E, 0x9E, 0x9E}), ErrLowContrast},
		{Opaque(RGB{0x7A, 0x96, 0xC5}), ErrLowContrast},
		{Opaque(RGB{0x80, 0x80, 0x80}), ErrLowContrast},
		{Translucent(RGB{0x9E, 0x9E, 0x9E}, 254), nil},
		{Translucent(RGB{0x7A, 0x96, 0xC5}, 0), nil},
		{Transparent(), nil},
	} {
		_, err := Render(fp, 32, RenderOptions{Background: c.background})
		if err != c.want {
			t.Errorf("background %v: %v, want %v", c.background, err, c.want)
		}
		report := MeasureContrast(RenderOptions{Background: c.background}, White)
		if c.background.Alpha() == 255 && (report.FiguresX100 < 200) != (err == ErrLowContrast) {
			t.Errorf("background %v: %d with %v", c.background, report.FiguresX100, err)
		}
	}
}

func TestEncodersCheckTheImageThenTheQuality(t *testing.T) {
	good := &Image{Width: 2, Height: 2, Pix: make([]byte, 16)}
	for _, quality := range []int{-1, 0, 49, 101, 1000, -1 << 31} {
		if _, err := good.EncodeJPEG(quality, White); err != ErrInvalidQuality {
			t.Errorf("quality %d: %v", quality, err)
		}
	}
	for _, quality := range []int{50, 92, 100} {
		if _, err := good.EncodeJPEG(quality, White); err != nil {
			t.Errorf("quality %d: %v", quality, err)
		}
	}
	for _, bad := range []*Image{
		nil,
		{},
		{Width: 0, Height: 1},
		{Width: 1, Height: 0},
		{Width: -1, Height: -1, Pix: make([]byte, 4)},
		{Width: 2, Height: 2, Pix: make([]byte, 15)},
		{Width: 2, Height: 2, Pix: make([]byte, 17)},
		{Width: 2, Height: 2},
		{Width: 4097, Height: 1, Pix: make([]byte, 4097*4)},
		{Width: 1, Height: 4097, Pix: make([]byte, 4097*4)},
		{Width: 1<<31 - 1, Height: 1<<31 - 1},
	} {
		_, png := bad.EncodePNG()
		_, bmp := bad.EncodeBMP(White)
		_, jpeg := bad.EncodeJPEG(0, White) // the image is checked before the quality
		if png != ErrInvalidImage || bmp != ErrInvalidImage || jpeg != ErrInvalidImage {
			t.Errorf("%+v: %v, %v, %v", bad, png, bmp, jpeg)
		}
	}
	for _, size := range [][2]int{{4096, 1}, {1, 4096}} {
		img := &Image{Width: size[0], Height: size[1], Pix: make([]byte, 4096*4)}
		if _, err := img.EncodePNG(); err != nil {
			t.Errorf("%d x %d: %v", size[0], size[1], err)
		}
	}
}
