package hh

const (
	// MinSize is the smallest render size in pixels.
	MinSize = 16
	// MaxSize is the largest render size in pixels.
	MaxSize = 1024
)

// Render draws the fingerprint as size x size pixels. The checks are made in
// the order of SPEC.md section 6: ErrInvalidFingerprint for the zero
// Fingerprint, ErrInvalidArgument for an unknown Shape or Frame value,
// ErrInvalidSize unless size is MinSize..MaxSize, ErrInvalidFrame if the frame
// does not fit the shape (the mode plays no part), ErrLowContrast for an opaque
// background with less than 2:1 against a palette colour, and ErrInvalidSize
// again if the size leaves no room for the cells (the round shape with
// FrameDouble or FrameThick below 18 pixels).
//
// Render at the exact device-pixel size instead of scaling a picture: the
// rasteriser is anti-aliased for the size it is asked for.
func Render(fp Fingerprint, size int, opts RenderOptions) (*Image, error) {
	if fp.IsZero() {
		return nil, ErrInvalidFingerprint
	}
	if int(opts.Shape) >= len(shapeNames) || int(opts.Frame) >= len(frameNames) {
		return nil, ErrInvalidArgument
	}
	if size < MinSize || size > MaxSize {
		return nil, ErrInvalidSize
	}
	frame := resolveFrame(opts.Frame, fp.mode, opts.Shape)
	if !frameAllowed(frame, opts.Shape) {
		return nil, ErrInvalidFrame
	}
	background := opts.Background
	if background.Alpha() == 255 && figuresContrastX100(background.Color()) < 200 {
		return nil, ErrLowContrast
	}
	g := newGeometry(size, opts.Shape, frame)
	if g.cell < 1 {
		return nil, ErrInvalidSize
	}
	r := rasteriser{
		geometry:   g,
		tester:     newFrameTester(g, opts.Shape, frame),
		background: background.Color(),
		ab:         int(background.Alpha()),
		pix:        make([]byte, size*size*4),
	}
	r.drawSurface(int(opts.FrameAlpha.Alpha()))
	r.drawFigures(cellsOf(fp.bytes))
	return &Image{Width: size, Height: size, Pix: r.pix}, nil
}

// resolveFrame replaces FrameAutomatic as SPEC.md section 6 defines.
func resolveFrame(frame Frame, mode Mode, shape Shape) Frame {
	switch {
	case frame != FrameAutomatic:
		return frame
	case mode == ModeKeyed && shape == ShapeSquare:
		return FrameRounded
	}
	return FrameNone
}

// frameAllowed is the table of SPEC.md section 6 for a resolved frame. The
// mode plays no part.
func frameAllowed(frame Frame, shape Shape) bool {
	switch frame {
	case FrameNone, FramePlain, FrameDouble, FrameThick:
		return true
	case FrameRounded, FrameChamfered, FrameBrackets:
		return shape == ShapeSquare
	case FrameTicks, FrameGaps:
		return shape == ShapeRound
	}
	return false
}

// geometry holds the values of SPEC.md section 7, in pixels.
type geometry struct {
	size   int // S
	line   int // w
	gutter int // g
	cell   int // t
	grid   int // G
	offset int // o
}

func newGeometry(size int, shape Shape, frame Frame) geometry {
	g := geometry{size: size, line: max(1, size/48), gutter: max(1, size/48)}
	if shape == ShapeRound {
		k := 1
		if frame == FrameDouble || frame == FrameThick {
			k = 3
		}
		margin := k*g.line + g.gutter
		limit := (size - 2*margin) * (size - 2*margin)
		for side := 4*(g.cell+1) + 3*g.gutter; 2*side*side <= limit; side += 4 {
			g.cell++
		}
	} else {
		margin := max(4*g.line, size/12)
		g.cell = (size - 2*margin - 3*g.gutter) / 4
	}
	g.grid = 4*g.cell + 3*g.gutter
	g.offset = (size - g.grid) / 2
	return g
}

// frameTester holds the per-sample tests of SPEC.md sections 8.2 and 8.3.
// Coordinates are in sample units: one pixel is 8 units.
type frameTester struct {
	round   bool
	frame   Frame
	s8      int // S8, the image side
	w8      int // W8, the line width
	r       int // r, the radius of the round shape and the centre of the image
	corner  int // R, the corner radius of FrameRounded
	diag    int // D, the width of a diagonal line
	chamfer int // C, the chamfer cut
	bracket int // the arm length of FrameBrackets
	gap     int // half the width of a gap of FrameGaps
	tickEnd int // Q, where the ticks of FrameTicks end
}

func newFrameTester(g geometry, shape Shape, frame Frame) frameTester {
	t := frameTester{
		round:   shape == ShapeRound,
		frame:   frame,
		s8:      8 * g.size,
		w8:      8 * g.line,
		r:       4 * g.size,
		corner:  min(16*g.offset, 4*g.size),
		diag:    (8*g.line*1414 + 500) / 1000,
		bracket: 8 * (g.size / 4),
		gap:     8 * max(1, g.size/24),
	}
	t.chamfer = max(8, (16*g.offset-t.diag-t.w8)/8*8)
	inner := t.r - t.w8
	t.tickEnd = inner - max(8, (inner-4*g.grid)*6/10)
	return t
}

// inOutline is the table of SPEC.md section 8.2.
func (t *frameTester) inOutline(u, v int) bool {
	if t.round {
		dx, dy := u-t.r, v-t.r
		return dx*dx+dy*dy <= t.r*t.r
	}
	du, dv := min(u, t.s8-u), min(v, t.s8-v)
	switch t.frame {
	case FrameRounded:
		if du < t.corner && dv < t.corner {
			ex, ey := t.corner-du, t.corner-dv
			return ex*ex+ey*ey <= t.corner*t.corner
		}
	case FrameChamfered:
		return du+dv >= t.chamfer
	}
	return true
}

// onFrame is the table of SPEC.md section 8.3. It is meaningful only for
// samples inside the outline.
func (t *frameTester) onFrame(u, v int) bool {
	if t.frame == FrameNone {
		return false
	}
	w8 := t.w8
	if t.round {
		dx, dy := u-t.r, v-t.r
		d2 := dx*dx + dy*dy
		ring := d2 > (t.r-w8)*(t.r-w8)
		switch t.frame {
		case FramePlain:
			return ring
		case FrameDouble:
			return ring || (d2 > (t.r-3*w8)*(t.r-3*w8) && d2 <= (t.r-2*w8)*(t.r-2*w8))
		case FrameThick:
			return d2 > (t.r-3*w8)*(t.r-3*w8)
		case FrameGaps:
			return ring && abs(abs(dx)-abs(dy)) >= t.gap
		case FrameTicks:
			return ring || (abs(dx) < w8 && abs(dy) >= t.tickEnd) || (abs(dy) < w8 && abs(dx) >= t.tickEnd)
		}
		return false
	}
	du, dv := min(u, t.s8-u), min(v, t.s8-v)
	e := min(du, dv)
	switch t.frame {
	case FramePlain:
		return e < w8
	case FrameDouble:
		return e < w8 || (e >= 2*w8 && e < 3*w8)
	case FrameThick:
		return e < 3*w8
	case FrameBrackets:
		return e < w8 && max(du, dv) < t.bracket
	case FrameChamfered:
		return e < w8 || du+dv-t.chamfer < t.diag
	case FrameRounded:
		if du < t.corner && dv < t.corner {
			ex, ey := t.corner-du, t.corner-dv
			return ex*ex+ey*ey > (t.corner-w8)*(t.corner-w8)
		}
		return e < w8
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// rasteriser draws one picture (SPEC.md section 8).
type rasteriser struct {
	geometry
	tester     frameTester
	background RGB
	ab         int
	pix        []byte
}

// mix is MIX of SPEC.md section 8.5: nf samples of the colour f with the alpha
// a over the background, nb samples of the background alone, the rest
// transparent.
func (r *rasteriser) mix(f RGB, a, nf, nb int) [4]byte {
	ab := r.ab
	total := nf*(255*a+ab*(255-a)) + nb*255*ab
	alpha := (total + 2040) / 4080
	if alpha == 0 {
		return [4]byte{}
	}
	channel := func(fc, bc uint8) byte {
		p := nf*(255*a*int(fc)+ab*(255-a)*int(bc)) + nb*255*ab*int(bc)
		return byte((p + total/2) / total)
	}
	b := r.background
	return [4]byte{channel(f.R, b.R), channel(f.G, b.G), channel(f.B, b.B), byte(alpha)}
}

// drawSurface is step 1 of SPEC.md section 8.5: the background, the outline and
// the frame.
//
// Every test of sections 8.2 and 8.3 depends only on the distances of a sample
// from the edges or from the centre lines, and the samples of a pixel mirror
// onto the samples of the mirrored pixel, so step 1 has the eight symmetries of
// the square. Inside the grid every sample is inside the outline and off the
// frame; a pixel within o pixels of no edge is in the grid or mirrors a pixel
// of the grid, so it is the plain surface and is not sampled. Of the margin one
// octant is sampled and copied to the other seven.
func (r *rasteriser) drawSurface(frameAlpha int) {
	surface := r.mix(frameColor, frameAlpha, 0, 16)
	copy(r.pix, surface[:])
	for n := 4; n < len(r.pix); n *= 2 {
		copy(r.pix[n:], r.pix[:n])
	}
	if !r.tester.round && r.tester.frame == FrameNone {
		return
	}
	var (
		pixels [17][17][4]byte
		known  [17][17]bool
	)
	last := r.size - 1
	for y := 0; y < r.offset; y++ {
		for x := y; x <= last/2; x++ {
			inside, onFrame := 0, 0
			for q := 0; q < 4; q++ {
				v := 2*(4*y+q) + 1
				for p := 0; p < 4; p++ {
					u := 2*(4*x+p) + 1
					if r.tester.inOutline(u, v) {
						inside++
						if r.tester.onFrame(u, v) {
							onFrame++
						}
					}
				}
			}
			if !known[inside][onFrame] {
				pixels[inside][onFrame] = r.mix(frameColor, frameAlpha, onFrame, inside-onFrame)
				known[inside][onFrame] = true
			}
			pixel := pixels[inside][onFrame][:]
			for _, at := range [8]int{
				y*r.size + x, y*r.size + last - x, (last-y)*r.size + x, (last-y)*r.size + last - x,
				x*r.size + y, x*r.size + last - y, (last-x)*r.size + y, (last-x)*r.size + last - y,
			} {
				copy(r.pix[4*at:4*at+4], pixel)
			}
		}
	}
}

// drawFigures is step 2 of SPEC.md section 8.5. How many samples of a pixel
// belong to a figure depends on the figure alone, so the counts are computed
// once per figure and shared by its cells.
func (r *rasteriser) drawFigures(cells [16]Cell) {
	var coverage [len(figureNames)][]uint8
	for i, c := range cells {
		if c.Figure == FigureNone {
			continue
		}
		if coverage[c.Figure] == nil {
			coverage[c.Figure] = figureCoverage(c.Figure, r.cell)
		}
		var shades [17][4]byte
		for n := range shades {
			shades[n] = r.mix(palette[c.Color], 255, n, 16-n)
		}
		x0 := r.offset + i%4*(r.cell+r.gutter)
		y0 := r.offset + i/4*(r.cell+r.gutter)
		counts := coverage[c.Figure]
		for py := 0; py < r.cell; py++ {
			row := r.pix[((y0+py)*r.size+x0)*4:]
			for px := 0; px < r.cell; px++ {
				copy(row[4*px:4*px+4], shades[counts[py*r.cell+px]][:])
			}
		}
	}
}

// figureCoverage returns, for every pixel of a cell of t x t pixels, row-major,
// how many of its 16 samples belong to the figure (SPEC.md section 8.4).
//
// A cell has n = 4 t samples per side; sample i of a row has u = 2 i + 1, so the
// samples on either side of the centre line u = H lie at the distances 1, 3, 5
// and so on. For the circle and the triangles that point up or down, the samples
// of row j that belong to the figure are those with (u - H)^2 <= limit:
//
//	circle          (u - H)^2 <= H^2 - (v - H)^2
//	triangle up     2 |u - H| <= v        which is  |u - H| <= j          as v = 2 j + 1
//	triangle down   2 |u - H| <= 2 H - v  which is  |u - H| <= n - 1 - j
//
// That is a run of m samples on each side of the centre line, where m is the
// largest count with (2 m - 1)^2 <= limit. The triangles that point left and
// right are the transposed triangles up and down.
func figureCoverage(f Figure, t int) []uint8 {
	counts := make([]uint8, t*t)
	if f == FigureSquare {
		for i := range counts {
			counts[i] = 16
		}
		return counts
	}
	n, h := 4*t, 4*t
	transposed := f == FigureTriangleLeft || f == FigureTriangleRight
	m := 0
	for j := 0; j < n; j++ {
		var limit int
		switch f {
		case FigureCircle:
			limit = h*h - (2*j+1-h)*(2*j+1-h)
		case FigureTriangleUp, FigureTriangleLeft:
			limit = j * j
		default:
			limit = (n - 1 - j) * (n - 1 - j)
		}
		// The limit changes little from row to row, so m is adjusted, not
		// searched.
		for (2*m+1)*(2*m+1) <= limit {
			m++
		}
		for m > 0 && (2*m-1)*(2*m-1) > limit {
			m--
		}
		if m == 0 {
			continue
		}
		first, last := n/2-m, n/2+m-1
		for px := first / 4; px <= last/4; px++ {
			run := uint8(min(last, 4*px+3) - max(first, 4*px) + 1)
			if transposed {
				counts[px*t+j/4] += run
			} else {
				counts[j/4*t+px] += run
			}
		}
	}
	return counts
}
