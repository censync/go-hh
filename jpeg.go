package hh

// The range of the JPEG quality and the value hosts should use when they have
// no reason for another.
const (
	MinJPEGQuality     = 50
	MaxJPEGQuality     = 100
	DefaultJPEGQuality = 92
)

// The tables of SPEC.md appendix B, which are the example tables of ITU-T T.81
// annex K.

// zigzag[i] is the natural index of the i-th coefficient in zigzag order.
var zigzag = [64]uint8{
	0, 1, 8, 16, 9, 2, 3, 10, 17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34, 27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36, 29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46, 53, 60, 61, 54, 47, 55, 62, 63,
}

// The base quantiser tables in natural order: luminance, then chrominance.
var baseQuantisers = [2][64]uint8{
	{
		16, 11, 10, 16, 24, 40, 51, 61,
		12, 12, 14, 19, 26, 58, 60, 55,
		14, 13, 16, 24, 40, 57, 69, 56,
		14, 17, 22, 29, 51, 87, 80, 62,
		18, 22, 37, 56, 68, 109, 103, 77,
		24, 35, 55, 64, 81, 104, 113, 92,
		49, 64, 78, 87, 103, 121, 120, 101,
		72, 92, 95, 98, 112, 100, 103, 99,
	},
	{
		17, 18, 24, 47, 99, 99, 99, 99,
		18, 21, 26, 66, 99, 99, 99, 99,
		24, 26, 56, 99, 99, 99, 99, 99,
		47, 66, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
	},
}

// huffmanSpec is a Huffman table as a DHT segment holds it: the number of codes
// of each length 1..16, then the symbols in code order.
type huffmanSpec struct {
	id      byte // table class * 16 + destination
	counts  [16]uint8
	symbols []uint8
}

// The four tables in the order of the file: DC and AC luminance, DC and AC
// chrominance.
var huffmanSpecs = [4]huffmanSpec{
	{
		id:      0x00,
		counts:  [16]uint8{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0},
		symbols: []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	{
		id:     0x10,
		counts: [16]uint8{0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 0x7D},
		symbols: []uint8{
			0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
			0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xA1, 0x08, 0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0,
			0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28,
			0x29, 0x2A, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
			0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
			0x6A, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
			0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7,
			0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5,
			0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2,
			0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8,
			0xF9, 0xFA,
		},
	},
	{
		id:      0x01,
		counts:  [16]uint8{0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0},
		symbols: []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	{
		id:     0x11,
		counts: [16]uint8{0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 0x77},
		symbols: []uint8{
			0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21, 0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
			0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91, 0xA1, 0xB1, 0xC1, 0x09, 0x23, 0x33, 0x52, 0xF0,
			0x15, 0x62, 0x72, 0xD1, 0x0A, 0x16, 0x24, 0x34, 0xE1, 0x25, 0xF1, 0x17, 0x18, 0x19, 0x1A, 0x26,
			0x27, 0x28, 0x29, 0x2A, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
			0x49, 0x4A, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
			0x69, 0x6A, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
			0x88, 0x89, 0x8A, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3, 0xA4, 0xA5,
			0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3,
			0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA,
			0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8,
			0xF9, 0xFA,
		},
	},
}

// huffmanCode is the code of one symbol; a length of 0 means the symbol has
// none.
type huffmanCode struct {
	code   uint16
	length uint8
}

// huffmanCodes holds the codes of the four tables, assigned as ITU-T T.81 annex
// C defines, in the order of huffmanSpecs.
var huffmanCodes = func() (tables [4][256]huffmanCode) {
	for i, spec := range huffmanSpecs {
		code, k := uint16(0), 0
		for length := 1; length <= 16; length++ {
			for n := 0; n < int(spec.counts[length-1]); n++ {
				tables[i][spec.symbols[k]] = huffmanCode{code, uint8(length)}
				code++
				k++
			}
			code <<= 1
		}
	}
	return
}()

// EncodeJPEG encodes the image as baseline JFIF, 8 bits per sample, 4:4:4, with
// the tables of ITU-T T.81 annex K (SPEC.md section 13). JPEG has no alpha, so
// every pixel is laid over the matte; White is the usual one. It fails with
// ErrInvalidImage, then with ErrInvalidQuality unless quality is 50..100.
//
// JPEG is offered for compatibility; it rings on flat colour edges. Prefer PNG
// or the pixels themselves.
func (m *Image) EncodeJPEG(quality int, matte RGB) ([]byte, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	if quality < MinJPEGQuality || quality > MaxJPEGQuality {
		return nil, ErrInvalidQuality
	}

	// Section 13.2: the quantisers of this quality.
	var quantisers [2][64]int32
	scale := int32(200 - 2*quality)
	for t := range quantisers {
		for k, base := range baseQuantisers[t] {
			quantisers[t][k] = min(max((int32(base)*scale+50)/100, 1), 255)
		}
	}

	out := make([]byte, 0, m.Width*m.Height/2+1024)
	out = append(out, 0xFF, 0xD8) // SOI
	out = append(out, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x02, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00)
	for t := range quantisers {
		out = append(out, 0xFF, 0xDB, 0x00, 0x43, byte(t))
		for _, k := range zigzag {
			out = append(out, byte(quantisers[t][k]))
		}
	}
	out = append(out, 0xFF, 0xC0, 0x00, 0x11, 0x08,
		byte(m.Height>>8), byte(m.Height), byte(m.Width>>8), byte(m.Width),
		0x03, 0x01, 0x11, 0x00, 0x02, 0x11, 0x01, 0x03, 0x11, 0x01)
	for _, spec := range huffmanSpecs {
		length := 2 + 1 + 16 + len(spec.symbols)
		out = append(out, 0xFF, 0xC4, byte(length>>8), byte(length), spec.id)
		out = append(out, spec.counts[:]...)
		out = append(out, spec.symbols...)
	}
	out = append(out, 0xFF, 0xDA, 0x00, 0x0C, 0x03, 0x01, 0x00, 0x02, 0x11, 0x03, 0x11, 0x00, 0x3F, 0x00)

	// Sections 13.3 to 13.7: the blocks, row-major, each as Y, Cb, Cr.
	w := scanWriter{out: out}
	var (
		blocks     [3][64]int32
		previousDC [3]int32
	)
	for by := 0; by < m.Height; by += 8 {
		for bx := 0; bx < m.Width; bx += 8 {
			for j := 0; j < 8; j++ {
				// A block beyond the image repeats the last column and row.
				row := m.Pix[min(by+j, m.Height-1)*m.Width*4:]
				for i := 0; i < 8; i++ {
					x := min(bx+i, m.Width-1)
					fr, fg, fb := flattenPixel(row[4*x:4*x+4], matte)
					r, g, b := int32(fr), int32(fg), int32(fb)
					k := 8*j + i
					blocks[0][k] = (19595*r+38470*g+7471*b+32768)>>16 - 128
					blocks[1][k] = (-11059*r-21709*g+32768*b+8421375)>>16 - 128
					blocks[2][k] = (32768*r-27439*g-5329*b+8421375)>>16 - 128
				}
			}
			for c := range blocks {
				t := min(c, 1) // Y uses the luminance tables, Cb and Cr the chrominance tables
				forwardDCT(&blocks[c])
				quantise(&blocks[c], &quantisers[t])
				w.block(&blocks[c], previousDC[c], &huffmanCodes[2*t], &huffmanCodes[2*t+1])
				previousDC[c] = blocks[c][0]
			}
		}
	}
	out = w.finish()
	return append(out, 0xFF, 0xD9), nil // EOI
}

// The constants of SPEC.md section 13.5: cosines scaled by 2^13.
const (
	f0298 = 2446
	f0390 = 3196
	f0541 = 4433
	f0765 = 6270
	f0899 = 7373
	f1175 = 9633
	f1501 = 12299
	f1847 = 15137
	f1961 = 16069
	f2053 = 16819
	f2562 = 20995
	f3072 = 25172
)

// descale is DESCALE of SPEC.md section 13.5. The shift of a signed integer is
// arithmetic, which is the floor division the specification asks for.
func descale(x int32, n uint) int32 {
	return (x + 1<<(n-1)) >> n
}

// forwardDCT is the transform of SPEC.md section 13.5, in place: first every
// row, then every column. The result is the DCT scaled by 8.
func forwardDCT(d *[64]int32) {
	for row := 0; row < 8; row++ {
		dctPass(d, 8*row, 1, true)
	}
	for column := 0; column < 8; column++ {
		dctPass(d, column, 8, false)
	}
}

// dctPass transforms the eight values at base, base + step, ...
func dctPass(d *[64]int32, base, step int, first bool) {
	d0, d1, d2, d3 := &d[base], &d[base+step], &d[base+2*step], &d[base+3*step]
	d4, d5, d6, d7 := &d[base+4*step], &d[base+5*step], &d[base+6*step], &d[base+7*step]

	t0, t7 := *d0+*d7, *d0-*d7
	t1, t6 := *d1+*d6, *d1-*d6
	t2, t5 := *d2+*d5, *d2-*d5
	t3, t4 := *d3+*d4, *d3-*d4
	t10, t13 := t0+t3, t0-t3
	t11, t12 := t1+t2, t1-t2

	n := uint(15)
	if first {
		n = 11
		*d0 = (t10 + t11) * 4
		*d4 = (t10 - t11) * 4
	} else {
		*d0 = descale(t10+t11, 2)
		*d4 = descale(t10-t11, 2)
	}

	z1 := (t12 + t13) * f0541
	*d2 = descale(z1+t13*f0765, n)
	*d6 = descale(z1-t12*f1847, n)

	z1, z2, z3, z4 := t4+t7, t5+t6, t4+t6, t5+t7
	z5 := (z3 + z4) * f1175
	t4, t5, t6, t7 = t4*f0298, t5*f2053, t6*f3072, t7*f1501
	z1, z2, z3, z4 = -z1*f0899, -z2*f2562, -z3*f1961+z5, -z4*f0390+z5
	*d7 = descale(t4+z1+z3, n)
	*d5 = descale(t5+z2+z4, n)
	*d3 = descale(t6+z2+z3, n)
	*d1 = descale(t7+z1+z4, n)
}

// quantise is SPEC.md section 13.6, in place and in natural order. AC values
// are clamped to the range the baseline tables can code; DC values lie in
// -1024..1016 by construction.
func quantise(d *[64]int32, quantiser *[64]int32) {
	for k, c := range d {
		divisor := 8 * quantiser[k]
		var value int32
		if c >= 0 {
			value = (c + divisor/2) / divisor
		} else {
			value = -((-c + divisor/2) / divisor)
		}
		if k != 0 {
			value = min(max(value, -1023), 1023)
		}
		d[k] = value
	}
}

// scanWriter packs the entropy-coded data: most significant bit first, every
// byte FF followed by 00 (SPEC.md section 13.7).
type scanWriter struct {
	out  []byte
	acc  uint32
	used uint
}

func (w *scanWriter) bits(value uint32, count uint) {
	w.acc = w.acc<<count | value&(1<<count-1)
	w.used += count
	for w.used >= 8 {
		b := byte(w.acc >> (w.used - 8))
		w.out = append(w.out, b)
		if b == 0xFF {
			w.out = append(w.out, 0x00)
		}
		w.used -= 8
	}
}

func (w *scanWriter) symbol(table *[256]huffmanCode, s int) {
	w.bits(uint32(table[s].code), uint(table[s].length))
}

// value writes a category symbol and the additional bits of v: its low bits if
// v >= 0, otherwise those of v - 1. run is the number of zeros before an AC
// coefficient.
func (w *scanWriter) value(table *[256]huffmanCode, run int, v int32) {
	magnitude := v
	if v < 0 {
		magnitude = -v
		v--
	}
	category := 0
	for ; magnitude != 0; magnitude >>= 1 {
		category++
	}
	w.symbol(table, run*16+category)
	w.bits(uint32(v), uint(category))
}

// block codes one quantised block given in natural order.
func (w *scanWriter) block(d *[64]int32, previousDC int32, dc, ac *[256]huffmanCode) {
	w.value(dc, 0, d[0]-previousDC)
	run := 0
	for _, k := range zigzag[1:] {
		v := d[k]
		if v == 0 {
			run++
			continue
		}
		for ; run >= 16; run -= 16 {
			w.symbol(ac, 0xF0)
		}
		w.value(ac, run, v)
		run = 0
	}
	if run != 0 {
		w.symbol(ac, 0x00) // end of block
	}
}

// finish fills the last byte with 1 bits and returns the data.
func (w *scanWriter) finish() []byte {
	if w.used != 0 {
		pad := 8 - w.used
		w.bits(1<<pad-1, pad)
	}
	return w.out
}
