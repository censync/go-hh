package hh

// FingerprintSize is the size of a fingerprint in bytes.
const FingerprintSize = 32

// Mode tells how a fingerprint was derived. Universal pictures are the same for
// everyone; keyed pictures can be computed only with the secret key. The two
// pictures of one input are unrelated.
type Mode uint8

const (
	// ModeUniversal is the mode of the public picture, the one two parties
	// compare.
	ModeUniversal Mode = 1
	// ModeKeyed is the mode of the private picture, the default inside an
	// application.
	ModeKeyed Mode = 2
)

// String returns "universal" or "keyed", or "invalid" for any other value.
func (m Mode) String() string {
	switch m {
	case ModeUniversal:
		return "universal"
	case ModeKeyed:
		return "keyed"
	}
	return "invalid"
}

// ParseMode is the inverse of Mode.String: it takes "universal" or "keyed" and
// fails with ErrInvalidArgument.
func ParseMode(name string) (Mode, error) {
	switch name {
	case "universal":
		return ModeUniversal, nil
	case "keyed":
		return ModeKeyed, nil
	}
	return 0, ErrInvalidArgument
}

// MarshalText returns the name that String returns, for a host that stores the
// mode beside the bytes of a fingerprint. It fails with ErrInvalidArgument for
// any value other than ModeUniversal and ModeKeyed.
func (m Mode) MarshalText() ([]byte, error) {
	if m != ModeUniversal && m != ModeKeyed {
		return nil, ErrInvalidArgument
	}
	return []byte(m.String()), nil
}

// UnmarshalText is ParseMode. It fails with ErrInvalidArgument and then leaves
// the value as it was.
func (m *Mode) UnmarshalText(text []byte) error {
	return unmarshalText(m, text, ParseMode)
}

// Fingerprint is 32 bytes and the mode they were derived in: everything a
// picture depends on. It is comparable and can be a map key. The zero value is
// not a fingerprint: Render refuses it with ErrInvalidFingerprint.
type Fingerprint struct {
	bytes [FingerprintSize]byte
	mode  Mode
}

// Universal returns the universal fingerprint, which is the base digest itself.
func Universal(d BaseDigest) Fingerprint {
	return Fingerprint{bytes: d, mode: ModeUniversal}
}

// Keyed returns the keyed fingerprint: one HMAC of the base digest under the
// key. It fails with ErrInvalidKey if the key is nil or was closed.
func Keyed(d BaseDigest, key *SecretKey) (Fingerprint, error) {
	b, err := key.keyedFingerprint(d)
	if err != nil {
		return Fingerprint{}, err
	}
	return Fingerprint{bytes: b, mode: ModeKeyed}, nil
}

// ImportFingerprint is for hosts that compute the keyed HMAC elsewhere, for
// example inside a secure element: it takes the 32 fingerprint bytes and the
// mode they belong to. It fails with ErrInvalidFingerprint for any other length
// or an unknown mode.
func ImportFingerprint(b []byte, mode Mode) (Fingerprint, error) {
	if len(b) != FingerprintSize || (mode != ModeUniversal && mode != ModeKeyed) {
		return Fingerprint{}, ErrInvalidFingerprint
	}
	fp := Fingerprint{mode: mode}
	copy(fp.bytes[:], b)
	return fp, nil
}

// Mode returns the mode of the fingerprint, or 0 for the zero value.
func (fp Fingerprint) Mode() Mode { return fp.mode }

// Bytes returns the 32 bytes.
func (fp Fingerprint) Bytes() [FingerprintSize]byte { return fp.bytes }

// IsZero reports whether fp is the zero value, which no function of this
// package returns without an error.
func (fp Fingerprint) IsZero() bool { return fp.mode == 0 }

// Crockford Base32 (SPEC.md section 5.3).
const tagAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Tag returns the six-character Crockford Base32 tag, for example "K7QM2X";
// hosts display it grouped as "K7Q-M2X". Text allows a certain check where a
// picture does not. The tag of a keyed fingerprint depends on the key.
func (fp Fingerprint) Tag() string {
	b := fp.bytes
	word := uint32(b[16])<<24 | uint32(b[17])<<16 | uint32(b[18])<<8 | uint32(b[19])
	v := word >> 2
	var tag [6]byte
	for i := range tag {
		tag[i] = tagAlphabet[v>>(25-5*uint(i))&31]
	}
	return string(tag[:])
}

// Figure is what a cell shows. The values are the layout values of SPEC.md
// section 5.1.
type Figure uint8

// The figures of SPEC.md section 5.1. A triangle is isosceles: its base is one
// full side of the cell and its apex the midpoint of the opposite side.
const (
	FigureNone          Figure = 0 // an empty cell
	FigureSquare        Figure = 1 // the full cell
	FigureCircle        Figure = 2 // the disc inscribed in the cell
	FigureTriangleUp    Figure = 3 // base on the bottom side
	FigureTriangleRight Figure = 4 // base on the left side
	FigureTriangleDown  Figure = 5 // base on the top side
	FigureTriangleLeft  Figure = 6 // base on the right side
)

var figureNames = [...]string{
	"none", "square", "circle", "triangle_up", "triangle_right", "triangle_down", "triangle_left",
}

// String returns the name of the figure, for example "triangle_up", or
// "invalid" for a value outside the table.
func (f Figure) String() string {
	if int(f) >= len(figureNames) {
		return "invalid"
	}
	return figureNames[f]
}

// figureOfCode maps the figure code, the top three bits of a cell byte, to the
// figure (SPEC.md section 5.1).
var figureOfCode = [8]Figure{
	FigureNone, FigureNone, FigureSquare, FigureCircle,
	FigureTriangleUp, FigureTriangleRight, FigureTriangleDown, FigureTriangleLeft,
}

// Cell is one cell of the 4 x 4 matrix.
type Cell struct {
	// Figure is what the cell shows.
	Figure Figure
	// Color is the index of the figure's colour in Layout.Palette, 0..3. It
	// is 0 for an empty cell.
	Color uint8
}

// Layout is what a fingerprint shows, for hosts that draw vectors themselves.
// Cells are row-major from the top left. The raster of Render is the canonical
// form and the only one covered by byte-exact vectors.
type Layout struct {
	// Mode is the mode of the fingerprint.
	Mode Mode
	// Cells are the 16 cells, row-major from the top left.
	Cells [16]Cell
	// Palette holds the four colours of the figures.
	Palette [4]RGB
	// FrameColor is the colour of the frame.
	FrameColor RGB
}

// The colours of SPEC.md section 5.2.
var (
	palette    = [4]RGB{{0x7A, 0x96, 0xC5}, {0x89, 0x0A, 0xF0}, {0xC1, 0x04, 0x45}, {0xD4, 0x82, 0x00}}
	frameColor = RGB{0x80, 0x80, 0x80}
)

// Layout returns the cells, the palette and the mode. The zero Fingerprint
// gives 16 empty cells.
func (fp Fingerprint) Layout() Layout {
	l := Layout{Mode: fp.mode, Palette: palette, FrameColor: frameColor}
	if !fp.IsZero() {
		l.Cells = cellsOf(fp.bytes)
	}
	return l
}

func cellsOf(b [FingerprintSize]byte) [16]Cell {
	var cells [16]Cell
	for i := range cells {
		figure := figureOfCode[b[i]>>5]
		if figure != FigureNone {
			cells[i] = Cell{Figure: figure, Color: b[i] >> 3 & 3}
		}
	}
	return cells
}
