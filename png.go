package hh

import (
	"encoding/binary"
	"hash/adler32"
	"hash/crc32"
	"math/bits"
)

// EncodePNG encodes the image as an 8-bit truecolour PNG, with an alpha channel
// only if some pixel is not opaque (SPEC.md section 11). It fails with
// ErrInvalidImage.
func (m *Image) EncodePNG() ([]byte, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	opaque := true
	for p := 3; p < len(m.Pix); p += 4 {
		if m.Pix[p] != 255 {
			opaque = false
			break
		}
	}
	bpp, colorType := 4, byte(6)
	if opaque {
		bpp, colorType = 3, 2
	}

	// The raw stream: every row is the filter byte 00 and its pixels.
	stride := 1 + m.Width*bpp
	raw := make([]byte, stride*m.Height)
	for y := 0; y < m.Height; y++ {
		src := m.Pix[y*m.Width*4 : (y+1)*m.Width*4]
		dst := raw[y*stride+1 : (y+1)*stride]
		if opaque {
			for x := 0; x < m.Width; x++ {
				copy(dst[3*x:3*x+3], src[4*x:4*x+3])
			}
		} else {
			copy(dst, src)
		}
	}

	zlib := make([]byte, 0, len(raw)/8+64)
	zlib = append(zlib, 0x78, 0x01)
	zlib = deflateFixed(zlib, raw, bpp, stride)
	zlib = binary.BigEndian.AppendUint32(zlib, adler32.Checksum(raw))

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(m.Width))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(m.Height))
	ihdr[8], ihdr[9] = 8, colorType

	out := make([]byte, 0, len(zlib)+80)
	out = append(out, 0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A)
	out = appendChunk(out, "IHDR", ihdr)
	out = appendChunk(out, "sRGB", []byte{0})
	out = appendChunk(out, "IDAT", zlib)
	out = appendChunk(out, "IEND", nil)
	return out, nil
}

// appendChunk appends u32be(length) || type || data || u32be(CRC-32 of type || data).
func appendChunk(out []byte, kind string, data []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(data)))
	start := len(out)
	out = append(out, kind...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[start:]))
}

// The length and distance codes of RFC 1951 section 3.2.5: the base value and
// the number of extra bits of each code.
var (
	lengthBase = [29]uint16{
		3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258,
	}
	lengthExtra = [29]uint8{
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0,
	}
	distBase = [30]uint16{
		1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097,
		6145, 8193, 12289, 16385, 24577,
	}
	distExtra = [30]uint8{
		0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13,
	}
)

// deflateFixed appends raw as one final deflate block with the fixed Huffman
// codes (RFC 1951 section 3.2.6). Matching is greedy over two candidate
// distances, the previous pixel and the pixel above, in this order (SPEC.md
// section 11).
func deflateFixed(out, raw []byte, bpp, stride int) []byte {
	w := bitWriter{out: out}
	w.bits(1, 1) // BFINAL
	w.bits(1, 2) // BTYPE = 01
	for i := 0; i < len(raw); {
		limit := min(258, len(raw)-i)
		best, dist := 0, 0
		for _, d := range [2]int{bpp, stride} {
			if i < d {
				continue
			}
			n := 0
			for n < limit && raw[i+n] == raw[i-d+n] {
				n++
			}
			if n > best {
				best, dist = n, d
			}
		}
		if best >= 3 {
			w.match(best, dist)
			i += best
		} else {
			w.symbol(int(raw[i]))
			i++
		}
	}
	w.symbol(256) // end of block
	return w.flush()
}

// bitWriter packs bits from the least significant bit of each byte, as deflate
// does.
type bitWriter struct {
	out  []byte
	acc  uint64
	used uint
}

func (w *bitWriter) bits(value uint32, count uint) {
	w.acc |= uint64(value) << w.used
	w.used += count
	for w.used >= 8 {
		w.out = append(w.out, byte(w.acc))
		w.acc >>= 8
		w.used -= 8
	}
}

// code writes a Huffman code, which goes most significant bit first.
func (w *bitWriter) code(code uint32, count uint) {
	w.bits(bits.Reverse32(code)>>(32-count), count)
}

// symbol writes a literal/length symbol with the fixed code.
func (w *bitWriter) symbol(s int) {
	switch {
	case s < 144:
		w.code(uint32(0x30+s), 8)
	case s < 256:
		w.code(uint32(0x190+s-144), 9)
	case s < 280:
		w.code(uint32(s-256), 7)
	default:
		w.code(uint32(0xC0+s-280), 8)
	}
}

func (w *bitWriter) match(length, dist int) {
	li := len(lengthBase) - 1
	for int(lengthBase[li]) > length {
		li--
	}
	w.symbol(257 + li)
	w.bits(uint32(length-int(lengthBase[li])), uint(lengthExtra[li]))
	di := len(distBase) - 1
	for int(distBase[di]) > dist {
		di--
	}
	w.code(uint32(di), 5)
	w.bits(uint32(dist-int(distBase[di])), uint(distExtra[di]))
}

// flush pads the last byte with zero bits and returns the stream.
func (w *bitWriter) flush() []byte {
	if w.used > 0 {
		w.out = append(w.out, byte(w.acc))
		w.acc, w.used = 0, 0
	}
	return w.out
}
