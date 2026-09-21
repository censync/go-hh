package hh

// Code is the numeric value of an error condition. The values are those of the
// C ABI of hh-cpp (SPEC.md section 14) and are the same in every implementation.
type Code int

// The error codes of SPEC.md section 14. CodeBufferTooSmall and
// CodeOutOfMemory exist for completeness of the table: this package allocates
// its results itself and never reports them.
const (
	CodeOK                 Code = 0
	CodeEmptyInput         Code = 1
	CodeInputTooLarge      Code = 2
	CodeInvalidHex         Code = 3
	CodeInvalidKey         Code = 4
	CodeInvalidDigest      Code = 5
	CodeInvalidFingerprint Code = 6
	CodeInvalidSize        Code = 7
	CodeInvalidFrame       Code = 8
	CodeLowContrast        Code = 9
	CodeInvalidQuality     Code = 10
	CodeInvalidImage       Code = 11
	CodeBufferTooSmall     Code = 12
	CodeOutOfMemory        Code = 13
	CodeInvalidArgument    Code = 14
)

var codeNames = [...]string{
	CodeOK:                 "ok",
	CodeEmptyInput:         "empty_input",
	CodeInputTooLarge:      "input_too_large",
	CodeInvalidHex:         "invalid_hex",
	CodeInvalidKey:         "invalid_key",
	CodeInvalidDigest:      "invalid_digest",
	CodeInvalidFingerprint: "invalid_fingerprint",
	CodeInvalidSize:        "invalid_size",
	CodeInvalidFrame:       "invalid_frame",
	CodeLowContrast:        "low_contrast",
	CodeInvalidQuality:     "invalid_quality",
	CodeInvalidImage:       "invalid_image",
	CodeBufferTooSmall:     "buffer_too_small",
	CodeOutOfMemory:        "out_of_memory",
	CodeInvalidArgument:    "invalid_argument",
}

// String returns the name of the code as the specification and the golden
// vectors spell it, for example "invalid_hex", or "unknown" for a value outside
// the table.
func (c Code) String() string {
	if c < 0 || int(c) >= len(codeNames) {
		return "unknown"
	}
	return codeNames[c]
}

// Error is the type of every error this package returns. The package returns
// only its Err values, never a copy or a wrapped one, so errors can be compared
// with == or errors.Is, and errors.As gives access to the numeric code.
type Error struct {
	code    Code
	message string
}

// Code returns the numeric code of SPEC.md section 14. A nil *Error is no
// error: its code is CodeOK.
func (e *Error) Code() Code {
	if e == nil {
		return CodeOK
	}
	return e.code
}

// Error returns the text "hh: <name>: <description>", and "hh: ok" for a nil
// *Error.
func (e *Error) Error() string {
	if e == nil {
		return "hh: ok"
	}
	return "hh: " + e.code.String() + ": " + e.message
}

// The errors of this package.
var (
	// ErrEmptyInput reports an input without bytes.
	ErrEmptyInput = &Error{CodeEmptyInput, "the input has no bytes"}
	// ErrInputTooLarge reports an input longer than MaxInputSize bytes.
	ErrInputTooLarge = &Error{CodeInputTooLarge, "the input is longer than 1048576 bytes"}
	// ErrInvalidHex reports a string that is not an optional 0x followed by an
	// even, non-zero number of hexadecimal digits.
	ErrInvalidHex = &Error{CodeInvalidHex, "the string is not an even number of hexadecimal digits"}
	// ErrInvalidKey reports a key that is not 32 bytes, is all zero, is nil or
	// was closed.
	ErrInvalidKey = &Error{CodeInvalidKey, "the key must be 32 bytes that are not all zero"}
	// ErrInvalidDigest reports a stored base digest that is not 32 bytes.
	ErrInvalidDigest = &Error{CodeInvalidDigest, "the base digest must be 32 bytes"}
	// ErrInvalidFingerprint reports a fingerprint that is not 32 bytes, has an
	// unknown mode or is the zero value.
	ErrInvalidFingerprint = &Error{CodeInvalidFingerprint, "the fingerprint must be 32 bytes with a known mode"}
	// ErrInvalidSize reports a render size outside MinSize..MaxSize, or one
	// that leaves no room for the cells.
	ErrInvalidSize = &Error{CodeInvalidSize, "the image size must be 16..1024 and leave room for the cells"}
	// ErrInvalidFrame reports a frame that is not allowed for the shape or the
	// mode.
	ErrInvalidFrame = &Error{CodeInvalidFrame, "the frame is not allowed for this shape or mode"}
	// ErrLowContrast reports an opaque background that is too close to a
	// palette colour.
	ErrLowContrast = &Error{CodeLowContrast, "the background is too close to a palette colour"}
	// ErrInvalidQuality reports a JPEG quality outside 50..100.
	ErrInvalidQuality = &Error{CodeInvalidQuality, "the JPEG quality must be 50..100"}
	// ErrInvalidImage reports image dimensions outside 1..4096 or a pixel
	// buffer of the wrong length.
	ErrInvalidImage = &Error{CodeInvalidImage, "the image dimensions or its buffer length are invalid"}
	// ErrInvalidArgument reports an unknown Shape or Frame value, a text that
	// is not valid UTF-8, or a string that a Parse function does not accept.
	ErrInvalidArgument = &Error{CodeInvalidArgument, "an unknown enumeration value, an ill-formed text or an unknown name"}
)
