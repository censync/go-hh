package hh

import (
	"bytes"
	"fmt"
	"testing"
)

// referenceRender is SPEC.md sections 7 and 8 written down sample by sample:
// every pixel of the canvas and every pixel of every cell evaluates its 16
// samples, with no symmetry, no skipped area and no shared table. Render must
// give the same pixels. frame is the resolved style.
func referenceRender(fp Fingerprint, S int, shape Shape, frame Frame, B RGB, ab, af int) []byte {
	// Section 7.
	w := max(1, S/48)
	g := max(1, S/48)
	var t int
	if shape == ShapeSquare {
		m := max(4*w, S/12)
		t = (S - 2*m - 3*g) / 4
	} else {
		k := 1
		if frame == FrameDouble || frame == FrameThick {
			k = 3
		}
		m := k*w + g
		for 2*(4*(t+1)+3*g)*(4*(t+1)+3*g) <= (S-2*m)*(S-2*m) {
			t++
		}
	}
	G := 4*t + 3*g
	o := (S - G) / 2

	// Section 8.1 and the constants of 8.2 and 8.3.
	W8, S8, r := 8*w, 8*S, 4*S
	R := min(16*o, 4*S)
	D := (W8*1414 + 500) / 1000
	C := max(8, 8*((16*o-D-W8)/8))
	I := r - W8
	Q := I - max(8, (I-4*G)*6/10)

	inOutline := func(U, V int) bool {
		du, dv := min(U, S8-U), min(V, S8-V)
		dx, dy := U-4*S, V-4*S
		switch {
		case shape == ShapeRound:
			return dx*dx+dy*dy <= r*r
		case frame == FrameRounded:
			return !(du < R && dv < R) || (R-du)*(R-du)+(R-dv)*(R-dv) <= R*R
		case frame == FrameChamfered:
			return du+dv >= C
		}
		return true
	}
	onFrame := func(U, V int) bool {
		du, dv := min(U, S8-U), min(V, S8-V)
		e := min(du, dv)
		dx, dy := U-4*S, V-4*S
		d2 := dx*dx + dy*dy
		if shape == ShapeSquare {
			switch frame {
			case FramePlain:
				return e < W8
			case FrameDouble:
				return e < W8 || (2*W8 <= e && e < 3*W8)
			case FrameThick:
				return e < 3*W8
			case FrameBrackets:
				return e < W8 && max(du, dv) < 8*(S/4)
			case FrameChamfered:
				return e < W8 || du+dv-C < D
			case FrameRounded:
				if du < R && dv < R {
					return (R-du)*(R-du)+(R-dv)*(R-dv) > (R-W8)*(R-W8)
				}
				return e < W8
			}
			return false
		}
		switch frame {
		case FramePlain:
			return d2 > (r-W8)*(r-W8)
		case FrameDouble:
			return d2 > (r-W8)*(r-W8) || ((r-3*W8)*(r-3*W8) < d2 && d2 <= (r-2*W8)*(r-2*W8))
		case FrameThick:
			return d2 > (r-3*W8)*(r-3*W8)
		case FrameGaps:
			return d2 > (r-W8)*(r-W8) && abs(abs(dx)-abs(dy)) >= 8*max(1, S/24)
		case FrameTicks:
			return d2 > (r-W8)*(r-W8) || (abs(dx) < W8 && abs(dy) >= Q) || (abs(dy) < W8 && abs(dx) >= Q)
		}
		return false
	}

	// Section 8.5.
	pix := make([]byte, S*S*4)
	mix := func(at int, F RGB, a, nf, nb int) {
		A := nf*(255*a+ab*(255-a)) + nb*255*ab
		alpha := (A + 2040) / 4080
		if alpha == 0 {
			copy(pix[at:at+4], []byte{0, 0, 0, 0})
			return
		}
		for c, ch := range [3][2]uint8{{F.R, B.R}, {F.G, B.G}, {F.B, B.B}} {
			P := nf*(255*a*int(ch[0])+ab*(255-a)*int(ch[1])) + nb*255*ab*int(ch[1])
			pix[at+c] = byte((P + A/2) / A)
		}
		pix[at+3] = byte(alpha)
	}
	for y := 0; y < S; y++ {
		for x := 0; x < S; x++ {
			nin, nfr := 0, 0
			for q := 0; q < 4; q++ {
				for p := 0; p < 4; p++ {
					U, V := 2*(4*x+p)+1, 2*(4*y+q)+1
					if inOutline(U, V) {
						nin++
						if onFrame(U, V) {
							nfr++
						}
					}
				}
			}
			mix((y*S+x)*4, RGB{0x80, 0x80, 0x80}, af, nfr, nin-nfr)
		}
	}
	H := 4 * t
	colours := [4]RGB{{0x7A, 0x96, 0xC5}, {0x89, 0x0A, 0xF0}, {0xC1, 0x04, 0x45}, {0xD4, 0x82, 0x00}}
	for i := 0; i < 16; i++ {
		b := int(fp.bytes[i])
		code, colour := b/32, b/8%4
		if code < 2 {
			continue
		}
		x0, y0 := o+i%4*(t+g), o+i/4*(t+g)
		for py := 0; py < t; py++ {
			for px := 0; px < t; px++ {
				nc := 0
				for q := 0; q < 4; q++ {
					for p := 0; p < 4; p++ {
						u, v := 2*(4*px+p)+1, 2*(4*py+q)+1
						var in bool
						switch code {
						case 2:
							in = true
						case 3:
							in = (u-H)*(u-H)+(v-H)*(v-H) <= H*H
						case 4:
							in = 2*abs(u-H) <= v
						case 5:
							in = 2*abs(v-H) <= 2*H-u
						case 6:
							in = 2*abs(u-H) <= 2*H-v
						case 7:
							in = 2*abs(v-H) <= u
						}
						if in {
							nc++
						}
					}
				}
				mix(((y0+py)*S+x0+px)*4, colours[colour], 255, nc, 16-nc)
			}
		}
	}
	return pix
}

// samplerFingerprint shows every figure, and every colour with several figures.
func samplerFingerprint(mode Mode) Fingerprint {
	fp := Fingerprint{mode: mode}
	for i := range fp.bytes {
		fp.bytes[i] = byte((i+2)%8<<5 | (i/3)%4<<3 | i%8)
	}
	return fp
}

// The styles each shape allows, in either mode (SPEC.md section 6).
var framesOfShape = map[Shape][]Frame{
	ShapeSquare: {FrameNone, FramePlain, FrameRounded, FrameChamfered, FrameDouble, FrameThick, FrameBrackets},
	ShapeRound:  {FrameNone, FramePlain, FrameDouble, FrameThick, FrameTicks, FrameGaps},
}

func TestRenderEqualsTheSampleBySampleReference(t *testing.T) {
	backgrounds := []Background{
		{}, Transparent(), Opaque(RGB{0x12, 0x12, 0x12}), Translucent(RGB{0x12, 0x34, 0x56}, 0x40),
		Translucent(RGB{0xFE, 0xDC, 0xBA}, 0xC0), Translucent(RGB{0xFF, 0xFF, 0xFF}, 1), Translucent(RGB{}, 254),
	}
	frameAlphas := []uint8{255, 0, 128, 1, 254, 77}
	sizes := []int{}
	for size := MinSize; size <= 100; size++ {
		sizes = append(sizes, size)
	}
	sizes = append(sizes, 127, 128, 143, 144, 191, 192, 255, 256)
	if !testing.Short() {
		sizes = append(sizes, 383, 511, 640, 1023, 1024)
	}
	random := lcg(0xC0FFEE)
	n := 0
	for _, size := range sizes {
		for _, shape := range []Shape{ShapeSquare, ShapeRound} {
			frames := framesOfShape[shape]
			// Small sizes try every style, the large ones take turns.
			for k, frame := range frames {
				if size > 256 && k != size%len(frames) {
					continue
				}
				mode := ModeKeyed
				if n%2 == 0 {
					mode = ModeUniversal
				}
				fp := samplerFingerprint(mode)
				if n%3 == 0 {
					fp, _ = ImportFingerprint(random.bytes(FingerprintSize), mode)
				}
				opts := RenderOptions{
					Shape:      shape,
					Frame:      frame,
					Background: backgrounds[n%len(backgrounds)],
					FrameAlpha: Alpha(frameAlphas[n%len(frameAlphas)]),
				}
				n++
				img, err := Render(fp, size, opts)
				if err != nil {
					// The round shape has no room for the cells below 18 pixels with these two.
					if err == ErrInvalidSize && shape == ShapeRound && size < 18 && (frame == FrameDouble || frame == FrameThick) {
						continue
					}
					t.Fatalf("size %d %+v: %v", size, opts, err)
				}
				want := referenceRender(fp, size, shape, frame, opts.Background.Color(),
					int(opts.Background.Alpha()), int(opts.FrameAlpha.Alpha()))
				if !bytes.Equal(img.Pix, want) {
					t.Fatalf("size %d %v %v %v background %v frame alpha %d: the pixels differ from the reference",
						size, mode, shape, frame, opts.Background, opts.FrameAlpha.Alpha())
				}
			}
		}
	}
	if n < 1000 {
		t.Errorf("only %d renders were compared", n)
	}
}

// The automatic frame and the universal mode go through the same comparison.
func TestAutomaticFrameEqualsTheReference(t *testing.T) {
	for _, c := range []struct {
		mode     Mode
		shape    Shape
		resolved Frame
	}{
		{ModeUniversal, ShapeSquare, FrameNone},
		{ModeUniversal, ShapeRound, FrameNone},
		{ModeKeyed, ShapeSquare, FrameRounded},
		{ModeKeyed, ShapeRound, FrameNone},
	} {
		fp := samplerFingerprint(c.mode)
		for _, size := range []int{16, 17, 31, 48, 64, 95, 96, 97, 129} {
			img := mustRender(t, fp, size, RenderOptions{Shape: c.shape})
			if !bytes.Equal(img.Pix, referenceRender(fp, size, c.shape, c.resolved, RGB{255, 255, 255}, 255, 255)) {
				t.Errorf("%v %v %d: the pixels differ from the reference", c.mode, c.shape, size)
			}
		}
	}
}

// The frame depends on the style and the shape alone: two fingerprints with the
// same bytes and different modes give identical pictures, or the same error,
// for every explicit style. Only FrameAutomatic looks at the mode.
func TestAnExplicitFrameDrawsTheSameInBothModes(t *testing.T) {
	universal, keyed := samplerFingerprint(ModeUniversal), samplerFingerprint(ModeKeyed)
	rendered := 0
	for _, shape := range []Shape{ShapeSquare, ShapeRound} {
		for frame := FrameNone; int(frame) < len(frameNames); frame++ {
			for _, size := range []int{16, 17, 18, 33, 64, 80, 97, 128} {
				for _, background := range []Background{{}, Transparent(), Translucent(RGB{0x12, 0x34, 0x56}, 0x80)} {
					opts := RenderOptions{Shape: shape, Frame: frame, Background: background, FrameAlpha: Alpha(200)}
					a, errA := Render(universal, size, opts)
					b, errB := Render(keyed, size, opts)
					if errA != errB {
						t.Fatalf("%v %v %d: %v and %v", shape, frame, size, errA, errB)
					}
					if errA != nil {
						continue
					}
					if !bytes.Equal(a.Pix, b.Pix) {
						t.Fatalf("%v %v %d background %v: the modes give different pixels", shape, frame, size, background)
					}
					rendered++
				}
			}
		}
	}
	if rendered < 250 {
		t.Errorf("only %d renders were compared", rendered)
	}
	// A universal picture with rounded corners is the keyed square picture of
	// the same bytes with the automatic frame.
	for _, size := range []int{16, 48, 80, 128, 256} {
		automatic := mustRender(t, keyed, size, RenderOptions{})
		rounded := mustRender(t, universal, size, RenderOptions{Frame: FrameRounded})
		if !bytes.Equal(automatic.Pix, rounded.Pix) {
			t.Errorf("%d: universal with FrameRounded differs from keyed with FrameAutomatic", size)
		}
	}
}

func TestFigureCoverageEqualsThePredicates(t *testing.T) {
	inFigure := func(f Figure, u, v, H int) bool {
		switch f {
		case FigureSquare:
			return true
		case FigureCircle:
			return (u-H)*(u-H)+(v-H)*(v-H) <= H*H
		case FigureTriangleUp:
			return 2*abs(u-H) <= v
		case FigureTriangleDown:
			return 2*abs(u-H) <= 2*H-v
		case FigureTriangleRight:
			return 2*abs(v-H) <= 2*H-u
		case FigureTriangleLeft:
			return 2*abs(v-H) <= u
		}
		return false
	}
	cells := []int{}
	for cell := 1; cell <= 40; cell++ {
		cells = append(cells, cell)
	}
	cells = append(cells, 63, 64, 100, 197, 251)
	for _, cell := range cells {
		for f := FigureSquare; f <= FigureTriangleLeft; f++ {
			t.Run(fmt.Sprintf("%v/%d", f, cell), func(t *testing.T) {
				counts := figureCoverage(f, cell)
				for py := 0; py < cell; py++ {
					for px := 0; px < cell; px++ {
						want := uint8(0)
						for q := 0; q < 4; q++ {
							for p := 0; p < 4; p++ {
								if inFigure(f, 2*(4*px+p)+1, 2*(4*py+q)+1, 4*cell) {
									want++
								}
							}
						}
						if got := counts[py*cell+px]; got != want {
							t.Fatalf("pixel %d,%d: %d samples, want %d", px, py, got, want)
						}
					}
				}
			})
		}
	}
}

func TestGeometry(t *testing.T) {
	// S = 128: w = g = 2, m = 10, t = 25, G = 106, o = 11 (square); m = 4, t = 19, G = 82, o = 23 (round).
	if g := newGeometry(128, ShapeSquare, FrameNone); g != (geometry{128, 2, 2, 25, 106, 11}) {
		t.Errorf("square 128: %+v", g)
	}
	if g := newGeometry(128, ShapeRound, FramePlain); g != (geometry{128, 2, 2, 19, 82, 23}) {
		t.Errorf("round 128: %+v", g)
	}
	// The cells of the round shape stay inside the circle of the margin, at every size.
	for size := MinSize; size <= MaxSize; size++ {
		for _, frame := range []Frame{FrameNone, FrameThick} {
			g := newGeometry(size, ShapeRound, frame)
			k := 1
			if frame == FrameThick {
				k = 3
			}
			limit := (size - 2*(k*g.line+g.gutter)) * (size - 2*(k*g.line+g.gutter))
			if 2*g.grid*g.grid > limit || 2*(g.grid+4)*(g.grid+4) <= limit {
				t.Fatalf("round %d %v: the grid %d is not the largest that fits", size, frame, g.grid)
			}
			if (g.cell < 1) != (size < 18 && frame == FrameThick) {
				t.Fatalf("round %d %v: cell %d", size, frame, g.cell)
			}
		}
		if g := newGeometry(size, ShapeSquare, FrameThick); g.cell < 1 || g.offset < 4*g.line {
			t.Fatalf("square %d: %+v", size, g)
		}
	}
}
