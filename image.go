package hh

import "image"

// MaxEncodeDimension is the largest width and height the encoders accept.
const MaxEncodeDimension = 4096

// Image is pixels, not a picture: Width x Height pixels, row-major from the top
// left, 4 bytes per pixel in the order R, G, B, A with straight
// (non-premultiplied) alpha, the layout of image.NRGBA.
//
// Render returns square images. The encoders take any Image of 1..4096 by
// 1..4096 pixels whose Pix has Width * Height * 4 bytes and fail with
// ErrInvalidImage otherwise. They are deterministic: the same image gives the
// same bytes in every implementation of hh.
type Image struct {
	// Width and Height are the dimensions in pixels.
	Width, Height int
	// Pix holds the pixels: R, G, B, A of the top left pixel, then the rest
	// of its row, then the rows below.
	Pix []byte
}

// NRGBA returns the image as an *image.NRGBA for the standard image packages.
// The result shares Pix with m. An Image that the encoders would refuse gives
// an empty image.
func (m *Image) NRGBA() *image.NRGBA {
	if m.check() != nil {
		return &image.NRGBA{}
	}
	return &image.NRGBA{Pix: m.Pix, Stride: 4 * m.Width, Rect: image.Rect(0, 0, m.Width, m.Height)}
}

// check is the test of SPEC.md section 10 that every encoder makes first.
func (m *Image) check() error {
	if m == nil || m.Width < 1 || m.Width > MaxEncodeDimension || m.Height < 1 || m.Height > MaxEncodeDimension ||
		len(m.Pix) != m.Width*m.Height*4 {
		return ErrInvalidImage
	}
	return nil
}

// flatten lays one channel of a pixel with the alpha a over a matte channel
// (SPEC.md section 10).
func flatten(value, a, matte uint8) uint8 {
	return uint8((int(a)*int(value) + (255-int(a))*int(matte) + 127) / 255)
}

// flattenPixel lays the pixel at p over the matte.
func flattenPixel(p []byte, matte RGB) (r, g, b uint8) {
	a := p[3]
	if a == 255 {
		return p[0], p[1], p[2]
	}
	return flatten(p[0], a, matte.R), flatten(p[1], a, matte.G), flatten(p[2], a, matte.B)
}
