package hh

import "encoding/binary"

// EncodeBMP encodes the image as a 24-bit BMP (SPEC.md section 12). BMP carries
// no alpha here, so every pixel is laid over the matte; White is the usual one.
// It fails with ErrInvalidImage.
func (m *Image) EncodeBMP(matte RGB) ([]byte, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	row := 4 * ((3*m.Width + 3) / 4)
	dataSize := row * m.Height
	le := binary.LittleEndian
	out := make([]byte, 54+dataSize)
	out[0], out[1] = 'B', 'M'
	le.PutUint32(out[2:], uint32(54+dataSize))
	le.PutUint32(out[10:], 54) // offset of the pixels
	le.PutUint32(out[14:], 40) // BITMAPINFOHEADER
	le.PutUint32(out[18:], uint32(m.Width))
	le.PutUint32(out[22:], uint32(m.Height)) // positive: bottom-up
	le.PutUint16(out[26:], 1)                // planes
	le.PutUint16(out[28:], 24)               // bits per pixel
	le.PutUint32(out[34:], uint32(dataSize))
	le.PutUint32(out[38:], 2835) // 72 dpi in pixels per metre
	le.PutUint32(out[42:], 2835)
	at := 54
	for y := m.Height - 1; y >= 0; y-- {
		src := m.Pix[y*m.Width*4 : (y+1)*m.Width*4]
		dst := out[at : at+row]
		for x := 0; x < m.Width; x++ {
			r, g, b := flattenPixel(src[4*x:4*x+4], matte)
			dst[3*x], dst[3*x+1], dst[3*x+2] = b, g, r
		}
		at += row
	}
	return out, nil
}
