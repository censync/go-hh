package hh

import (
	"encoding"
	"encoding/hex"
	"strconv"
)

// Shape is the outline of the picture.
type Shape uint8

const (
	// ShapeSquare is the default.
	ShapeSquare Shape = 0
	// ShapeRound inscribes the 4 x 4 grid in a circle; no cell is clipped, the
	// cells are smaller.
	ShapeRound Shape = 1
)

var shapeNames = [...]string{"square", "round"}

// String returns the name of SPEC.md section 6, "square" or "round", or
// "invalid" for any other value.
func (s Shape) String() string {
	if int(s) >= len(shapeNames) {
		return "invalid"
	}
	return shapeNames[s]
}

// ParseShape is the inverse of Shape.String. It fails with ErrInvalidArgument.
func ParseShape(name string) (Shape, error) {
	for i, n := range shapeNames {
		if n == name {
			return Shape(i), nil
		}
	}
	return 0, ErrInvalidArgument
}

// MarshalText returns the name that String returns, so that a Shape is a name
// in JSON and in every other format that knows encoding.TextMarshaler. It fails
// with ErrInvalidArgument for a value outside the table.
func (s Shape) MarshalText() ([]byte, error) {
	if int(s) >= len(shapeNames) {
		return nil, ErrInvalidArgument
	}
	return []byte(shapeNames[s]), nil
}

// UnmarshalText is ParseShape. It fails with ErrInvalidArgument and then leaves
// the value as it was.
func (s *Shape) UnmarshalText(text []byte) error {
	return unmarshalText(s, text, ParseShape)
}

// Frame is the frame style of a picture. Every style is open to universal and
// keyed fingerprints alike; what limits it is the shape. FrameNone, FramePlain,
// FrameDouble and FrameThick fit both shapes, FrameRounded, FrameChamfered and
// FrameBrackets need ShapeSquare, FrameTicks and FrameGaps need ShapeRound, and
// Render refuses a style that does not fit the shape with ErrInvalidFrame. Only
// FrameAutomatic looks at the mode: it gives keyed square pictures rounded
// corners. A host that marks its keyed pictures with a frame picks the style.
type Frame uint8

// The frame styles of SPEC.md section 6, each open to both modes.
const (
	FrameAutomatic Frame = 0 // keyed and square: FrameRounded; otherwise FrameNone
	FrameNone      Frame = 1 // either shape: no frame
	FramePlain     Frame = 2 // either shape: a thin square frame, or a thin ring
	FrameRounded   Frame = 3 // square only: rounded corners
	FrameChamfered Frame = 4 // square only: four cut corners
	FrameDouble    Frame = 5 // either shape: two thin lines
	FrameThick     Frame = 6 // either shape: one line three times as thick
	FrameBrackets  Frame = 7 // square only: corner brackets
	FrameTicks     Frame = 8 // round only: a ring with four ticks
	FrameGaps      Frame = 9 // round only: a ring with four gaps
)

var frameNames = [...]string{
	"automatic", "none", "plain", "rounded", "chamfered", "double", "thick", "brackets", "ticks", "gaps",
}

// String returns the name of SPEC.md section 6, for example "double", or
// "invalid" for a value outside the table.
func (f Frame) String() string {
	if int(f) >= len(frameNames) {
		return "invalid"
	}
	return frameNames[f]
}

// ParseFrame is the inverse of Frame.String. It fails with ErrInvalidArgument.
func ParseFrame(name string) (Frame, error) {
	for i, n := range frameNames {
		if n == name {
			return Frame(i), nil
		}
	}
	return 0, ErrInvalidArgument
}

// MarshalText returns the name that String returns. It fails with
// ErrInvalidArgument for a value outside the table.
func (f Frame) MarshalText() ([]byte, error) {
	if int(f) >= len(frameNames) {
		return nil, ErrInvalidArgument
	}
	return []byte(frameNames[f]), nil
}

// UnmarshalText is ParseFrame. It fails with ErrInvalidArgument and then leaves
// the value as it was.
func (f *Frame) UnmarshalText(text []byte) error {
	return unmarshalText(f, text, ParseFrame)
}

// RGB is an sRGB colour.
type RGB struct {
	// R, G and B are the red, green and blue channels.
	R, G, B uint8
}

// White is the default background and the usual matte.
var White = RGB{0xFF, 0xFF, 0xFF}

// String returns the colour as six lowercase hexadecimal digits, "rrggbb".
func (c RGB) String() string {
	return hex.EncodeToString([]byte{c.R, c.G, c.B})
}

// ParseRGB parses six hexadecimal digits of either case, "RRGGBB". It fails
// with ErrInvalidArgument.
func ParseRGB(s string) (RGB, error) {
	var b [3]byte
	if len(s) != 6 {
		return RGB{}, ErrInvalidArgument
	}
	if _, err := hex.Decode(b[:], []byte(s)); err != nil {
		return RGB{}, ErrInvalidArgument
	}
	return RGB{b[0], b[1], b[2]}, nil
}

// MarshalText returns the six digits that String returns. It never fails.
func (c RGB) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

// UnmarshalText is ParseRGB. It fails with ErrInvalidArgument and then leaves
// the value as it was.
func (c *RGB) UnmarshalText(text []byte) error {
	return unmarshalText(c, text, ParseRGB)
}

// Background is what lies behind the figures: an sRGB colour and an alpha. The
// zero value is the default, opaque white. Outside rounded or chamfered corners
// and outside the disc of the round shape a picture is always transparent.
// Backgrounds are comparable: equal values give equal pictures.
type Background struct {
	// The complements of the channels and of the alpha, so that the zero
	// value is ff ff ff ff.
	r, g, b, a uint8
}

// Opaque returns an opaque background. Render refuses one that is too close to
// a palette colour with ErrLowContrast.
func Opaque(c RGB) Background { return Translucent(c, 255) }

// Translucent returns a background with an alpha from 0 (transparent) to 255
// (opaque).
func Translucent(c RGB, alpha uint8) Background {
	return Background{^c.R, ^c.G, ^c.B, ^alpha}
}

// Transparent returns the fully transparent background, for pictures laid over
// the host's own surface.
func Transparent() Background { return Translucent(RGB{}, 0) }

// Color returns the colour of the background.
func (b Background) Color() RGB { return RGB{^b.r, ^b.g, ^b.b} }

// Alpha returns the alpha of the background, 0 (transparent) to 255 (opaque).
func (b Background) Alpha() uint8 { return ^b.a }

// String returns the background as eight lowercase hexadecimal digits,
// "rrggbbaa", the form the golden vectors use.
func (b Background) String() string {
	c := b.Color()
	return hex.EncodeToString([]byte{c.R, c.G, c.B, b.Alpha()})
}

// ParseBackground parses eight hexadecimal digits of either case, "RRGGBBAA".
// It fails with ErrInvalidArgument.
func ParseBackground(s string) (Background, error) {
	var b [4]byte
	if len(s) != 8 {
		return Background{}, ErrInvalidArgument
	}
	if _, err := hex.Decode(b[:], []byte(s)); err != nil {
		return Background{}, ErrInvalidArgument
	}
	return Translucent(RGB{b[0], b[1], b[2]}, b[3]), nil
}

// MarshalText returns the eight digits that String returns. It never fails.
func (b Background) MarshalText() ([]byte, error) { return []byte(b.String()), nil }

// UnmarshalText is ParseBackground. It fails with ErrInvalidArgument and then
// leaves the value as it was.
func (b *Background) UnmarshalText(text []byte) error {
	return unmarshalText(b, text, ParseBackground)
}

// Opacity is an alpha value whose zero value is fully opaque; Alpha makes any
// other.
type Opacity struct {
	transparency uint8
}

// Alpha returns the opacity with the given alpha, 0 (transparent) to 255
// (opaque).
func Alpha(alpha uint8) Opacity { return Opacity{^alpha} }

// Alpha returns the alpha value, 255 for the zero Opacity.
func (o Opacity) Alpha() uint8 { return ^o.transparency }

// String returns the alpha as a decimal number, "0" to "255".
func (o Opacity) String() string { return strconv.Itoa(int(o.Alpha())) }

// ParseOpacity parses an alpha, 0 to 255, written as ASCII decimal digits
// without a sign. It fails with ErrInvalidArgument.
func ParseOpacity(s string) (Opacity, error) {
	if s == "" {
		return Opacity{}, ErrInvalidArgument
	}
	alpha := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return Opacity{}, ErrInvalidArgument
		}
		if alpha = 10*alpha + int(s[i]-'0'); alpha > 255 {
			return Opacity{}, ErrInvalidArgument
		}
	}
	return Alpha(uint8(alpha)), nil
}

// MarshalText returns the number that String returns. It never fails.
func (o Opacity) MarshalText() ([]byte, error) { return []byte(o.String()), nil }

// UnmarshalText is ParseOpacity. It fails with ErrInvalidArgument and then
// leaves the value as it was.
func (o *Opacity) UnmarshalText(text []byte) error {
	return unmarshalText(o, text, ParseOpacity)
}

// unmarshalText is the UnmarshalText of every type here: the Parse function of
// the type, and no panic for a nil receiver.
func unmarshalText[T any](to *T, text []byte, parse func(string) (T, error)) error {
	if to == nil {
		return ErrInvalidArgument
	}
	v, err := parse(string(text))
	if err != nil {
		return err
	}
	*to = v
	return nil
}

// Every field of RenderOptions, and Mode, has a text form.
var (
	_ encoding.TextMarshaler   = Shape(0)
	_ encoding.TextUnmarshaler = (*Shape)(nil)
	_ encoding.TextMarshaler   = Frame(0)
	_ encoding.TextUnmarshaler = (*Frame)(nil)
	_ encoding.TextMarshaler   = RGB{}
	_ encoding.TextUnmarshaler = (*RGB)(nil)
	_ encoding.TextMarshaler   = Background{}
	_ encoding.TextUnmarshaler = (*Background)(nil)
	_ encoding.TextMarshaler   = Opacity{}
	_ encoding.TextUnmarshaler = (*Opacity)(nil)
	_ encoding.TextMarshaler   = Mode(0)
	_ encoding.TextUnmarshaler = (*Mode)(nil)
)

// RenderOptions chooses the look of a render; the cells, the palette and the
// geometry are fixed by the specification. The zero value is the default: a
// square picture on opaque white with the automatic frame.
//
// Every field is an encoding.TextMarshaler and an encoding.TextUnmarshaler in
// the text form of its String method and its Parse function, so the options
// pass unchanged through encoding/json and every other format that is built on
// those interfaces:
//
//	{"Shape":"round","Frame":"double","Background":"121212ff","FrameAlpha":"200"}
//
// A field that is missing keeps its default. A text that the Parse function of
// the field refuses is ErrInvalidArgument.
type RenderOptions struct {
	// Shape is square or round.
	Shape Shape
	// Frame is the frame style: any style that fits the shape, in either mode.
	// A host that marks its keyed pictures with a frame uses one style
	// everywhere, since a marker is only useful if it is familiar, and names
	// the mode in the caption, since a frame alone proves nothing.
	Frame Frame
	// Background is the colour and alpha behind the figures.
	Background Background
	// FrameAlpha is the alpha of the frame; the frame colour itself is fixed.
	FrameAlpha Opacity
}
