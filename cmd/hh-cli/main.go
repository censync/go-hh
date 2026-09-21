// Command hh-cli renders the picture of an address or a hash. It is the
// counterpart of hh_cli in hh-cpp: both take the same options, print the same
// lines and write the same files, which is what tools/crosscheck.sh compares.
//
//	hh-cli 0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed --out address.png
//	hh-cli --text bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4 --size 256 --out address.png
//	hh-cli <hex> --key <64 hex digits> --shape round --frame double --out private.png
//	hh-cli --batch FILE DIR           runs the cases of FILE, writes case-<n>.<format> into DIR
//	hh-cli --generate COUNT SEED      prints COUNT pseudo-random cases, valid and invalid ones
//
// The batch format is the one hh_cli defines, and every tool follows it to the
// letter:
//
//   - The file is bytes. Lines end with LF; one CR before it is dropped. A line
//     that is then empty or begins with '#' is skipped and not counted. Cases
//     are numbered from 1.
//   - A case is exactly 11 fields separated by runs of ASCII spaces or tabs:
//     hex|text input key|- size shape frame background frame-alpha format
//     quality matte
//   - size, frame-alpha and quality are 1 to 10 ASCII digits without a sign. A
//     frame alpha above 255 is a bad case; size and quality go to the library
//     as they are, however large.
//   - For "text" the input is the hexadecimal form of the UTF-8 bytes, which go
//     to the library verbatim; for "hex" it is passed on as written. background
//     is 8 and matte 6 hexadecimal digits.
//   - A line that breaks these rules, or names an unknown kind, shape or frame,
//     prints "<n>\tbad_case". Everything else is the library's answer:
//     "<n>\t<error name>" and, for ok, the base digest, the fingerprint, the tag
//     and the key check value (or "-"), and the file DIR/case-<n>.<format>.
//
// The numeric options of a single render follow the same rule; a malformed one
// is a usage error.
//
// This is a demonstration and a test tool. A real host never takes a key from
// the command line, where other processes can read it.
package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	hh "github.com/censync/go-hh"
)

const usage = `usage: hh-cli [options] <input>
  <input>               hexadecimal bytes (optional 0x), or text with --text
  --text                hash the input as UTF-8 text
  --key HEX             64 hex digits: render the keyed (private) picture
  --size N              16..1024 pixels (default 128)
  --shape NAME          square (default) or round
  --frame NAME          automatic (default), none, plain, rounded, chamfered, double,
                        thick, brackets, ticks, gaps
  --background RRGGBBAA background colour and alpha (default ffffffff)
  --frame-alpha N       0..255 (default 255)
  --format NAME         png (default), bmp, jpeg or rgba
  --quality N           JPEG quality 50..100 (default 92)
  --matte RRGGBB        what BMP and JPEG flatten transparency over (default ffffff)
  --out FILE            write the picture; without it only the values are printed
  --batch FILE DIR      run the cases of FILE, write case-<n>.<format> into DIR;
                        a case is 11 fields separated by spaces or tabs:
                        hex|text <input> <key|-> <size> <shape> <frame> <background>
                        <frame alpha> <format> <quality> <matte>
                        (the input of a text case is the hex form of its bytes;
                        numbers are 1 to 10 digits without a sign)
  --generate COUNT SEED print COUNT pseudo-random cases for --batch
`

// request is one picture to make.
type request struct {
	text    bool
	input   []byte // hexadecimal digits, or the text itself
	key     string // hexadecimal digits, or empty for the universal picture
	size    int
	options hh.RenderOptions
	format  string
	quality int
	matte   hh.RGB
}

type result struct {
	digest      hh.BaseDigest
	fingerprint hh.Fingerprint
	kcv         string
	bytes       []byte
}

// run follows hh_cli step by step, so that the first error is the same one.
func run(rq request) (result, error) {
	var (
		r   result
		err error
	)
	if rq.text {
		r.digest, err = hh.BaseDigestFromUTF8(rq.input)
	} else {
		r.digest, err = hh.BaseDigestFromHex(string(rq.input))
	}
	if err != nil {
		return r, err
	}
	if rq.key == "" {
		r.fingerprint = hh.Universal(r.digest)
	} else {
		keyBytes, err := hex.DecodeString(rq.key)
		if err != nil {
			return r, hh.ErrInvalidKey
		}
		key, err := hh.NewSecretKey(keyBytes)
		if err != nil {
			return r, err
		}
		defer key.Close()
		kcv := key.KCV()
		r.kcv = hex.EncodeToString(kcv[:])
		if r.fingerprint, err = hh.Keyed(r.digest, key); err != nil {
			return r, err
		}
	}
	img, err := hh.Render(r.fingerprint, rq.size, rq.options)
	if err != nil {
		return r, err
	}
	switch rq.format {
	case "png":
		r.bytes, err = img.EncodePNG()
	case "bmp":
		r.bytes, err = img.EncodeBMP(rq.matte)
	case "jpeg":
		r.bytes, err = img.EncodeJPEG(rq.quality, rq.matte)
	case "rgba":
		r.bytes = img.Pix
	default:
		err = hh.ErrInvalidArgument
	}
	return r, err
}

// errorName is the name the specification gives the error.
func errorName(err error) string {
	var e *hh.Error
	if errors.As(err, &e) {
		return e.Code().String()
	}
	return "unknown"
}

// maxNumber is what a larger number of a case or an option becomes: a value
// that is just as invalid and fits an int everywhere.
const maxNumber = 1<<31 - 1

// parseNumber reads 1 to 10 ASCII digits without a sign.
func parseNumber(text string) (int, bool) {
	if len(text) < 1 || len(text) > 10 {
		return 0, false
	}
	var v uint64
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, false
		}
		v = v*10 + uint64(text[i]-'0')
	}
	return int(min(v, maxNumber)), true
}

// splitFields cuts a line at runs of ASCII spaces and tabs. Nothing else
// separates fields: strings.Fields would also cut at a form feed or a no-break
// space.
func splitFields(line string) []string {
	return strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '\t' })
}

// parseCase reads the fields of one line of a batch file.
func parseCase(fields []string) (request, bool) {
	var rq request
	if len(fields) != 11 || (fields[0] != "hex" && fields[0] != "text") {
		return rq, false
	}
	size, okSize := parseNumber(fields[3])
	shape, errShape := hh.ParseShape(fields[4])
	frame, errFrame := hh.ParseFrame(fields[5])
	background, errBackground := hh.ParseBackground(fields[6])
	frameAlpha, okFrameAlpha := parseNumber(fields[7])
	quality, okQuality := parseNumber(fields[9])
	matte, errMatte := hh.ParseRGB(fields[10])
	if !okSize || !okFrameAlpha || frameAlpha > 255 || !okQuality ||
		errors.Join(errShape, errFrame, errBackground, errMatte) != nil {
		return rq, false
	}
	rq.text = fields[0] == "text"
	rq.input = []byte(fields[1])
	if rq.text {
		text, err := hex.DecodeString(fields[1])
		if err != nil {
			return rq, false
		}
		rq.input = text
	}
	if fields[2] != "-" {
		rq.key = fields[2]
	}
	rq.size = size
	rq.options = hh.RenderOptions{
		Shape:      shape,
		Frame:      frame,
		Background: background,
		FrameAlpha: hh.Alpha(uint8(frameAlpha)),
	}
	rq.format = fields[8]
	rq.quality = quality
	rq.matte = matte
	return rq, true
}

func batch(file, dir string, stdout io.Writer) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read %s", file)
	}
	// A directory that cannot be made shows when the first file is written.
	os.MkdirAll(dir, 0o777)
	out := bufio.NewWriter(stdout)
	defer out.Flush()
	n := 0
	// The file is bytes: a Go string holds them as they are.
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || line[0] == '#' {
			continue
		}
		n++
		rq, ok := parseCase(splitFields(line))
		if !ok {
			fmt.Fprintf(out, "%d\tbad_case\n", n)
			continue
		}
		r, err := run(rq)
		if err != nil {
			fmt.Fprintf(out, "%d\t%s\n", n, errorName(err))
			continue
		}
		kcv := r.kcv
		if kcv == "" {
			kcv = "-"
		}
		fp := r.fingerprint.Bytes()
		fmt.Fprintf(out, "%d\tok\t%v\t%x\t%s\t%s\n", n, r.digest, fp[:], r.fingerprint.Tag(), kcv)
		name := filepath.Join(dir, fmt.Sprintf("case-%d.%s", n, rq.format))
		if err := os.WriteFile(name, r.bytes, 0o666); err != nil {
			return fmt.Errorf("cannot write into %s", dir)
		}
	}
	return nil
}

// random is SplitMix64: small, and the same on every platform and Go release.
type random uint64

func (r *random) next() uint64 {
	*r += 0x9E3779B97F4A7C15
	z := uint64(*r)
	z = (z ^ z>>30) * 0xBF58476D1CE4E5B9
	z = (z ^ z>>27) * 0x94D049BB133111EB
	return z ^ z>>31
}

func (r *random) intn(n int) int { return int(r.next() >> 11 % uint64(n)) }

func (r *random) hex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.next() >> 56)
	}
	return hex.EncodeToString(b)
}

func pick(r *random, values ...string) string { return values[r.intn(len(values))] }

// generate prints cases for --batch: mostly valid ones, so that most of them
// render, and every kind of invalid one.
func generate(count int, seed uint64, stdout io.Writer) {
	out := bufio.NewWriter(stdout)
	defer out.Flush()
	r := random(seed)
	fmt.Fprintf(out, "# %d cases, seed %d\n", count, seed)
	for i := 0; i < count; i++ {
		kind, input := "hex", ""
		switch {
		case r.intn(6) == 0:
			kind = "text"
			text := pick(&r, "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4", "caf\u00e9 \u20ac \U00010348", "T", "  spaced  ")
			input = hex.EncodeToString([]byte(text + strconv.Itoa(r.intn(1000))))
		case r.intn(25) == 0:
			input = pick(&r, "zz", "abc", "0x", "12_34")
		default:
			input = pick(&r, "0x", "") + r.hex(1+r.intn(40))
		}

		key := "-"
		switch n := r.intn(8); {
		case n == 3 && r.intn(4) == 0:
			key = strings.Repeat("00", 32)
		case n == 3:
			key = r.hex(31)
		case n > 3:
			key = r.hex(32)
		}

		size := 17 + r.intn(240)
		switch r.intn(30) {
		case 0:
			size = 15
		case 1:
			size = 1025
		case 2:
			size = 1024
		case 3:
			size = 16
		}

		background := pick(&r, "ffffffff", "ffffffff", "00000000", "00000000", "121212ff", "000000ff", "f2f2f2ff", "9e9e9eff")
		if r.intn(3) == 0 {
			background = r.hex(4)
		}

		shape := pick(&r, "square", "round")
		// Mostly a frame that fits the mode and the shape.
		var frame string
		switch {
		case r.intn(8) == 0:
			frame = hh.Frame(r.intn(10)).String()
		case key == "-":
			frame = pick(&r, "automatic", "none", "plain")
		case shape == "round":
			frame = pick(&r, "automatic", "none", "plain", "double", "thick", "ticks", "gaps")
		default:
			frame = pick(&r, "automatic", "none", "plain", "rounded", "chamfered", "double", "thick", "brackets")
		}

		format := pick(&r, "png", "png", "bmp", "jpeg", "rgba")
		quality := 50 + r.intn(51)
		if r.intn(12) == 0 {
			quality = 40 + r.intn(70)
		}
		fmt.Fprintln(out, kind, input, key, size, shape, frame, background, r.intn(256), format, quality, r.hex(3))
	}
}

// single renders one picture as the options of the command line ask, or runs
// a batch.
func single(args []string, stdout, stderr io.Writer) int {
	rq := request{size: 128, format: "png", quality: hh.DefaultJPEGQuality, matte: hh.White}
	var outPath string
	haveInput, formatGiven := false, false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func() (string, bool) {
			if i+1 >= len(args) {
				return "", false
			}
			i++
			return args[i], true
		}
		var (
			v   string
			err error
			ok  = true
		)
		switch arg {
		case "--help", "-h":
			io.WriteString(stderr, usage)
			return 0
		case "--batch":
			file, haveFile := value()
			dir, haveDir := value()
			if !haveFile || !haveDir {
				io.WriteString(stderr, usage)
				return 2
			}
			if err := batch(file, dir, stdout); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		case "--text":
			rq.text = true
		case "--key":
			rq.key, ok = value()
		case "--size":
			if v, ok = value(); ok {
				rq.size, ok = parseNumber(v)
			}
		case "--shape":
			if v, ok = value(); ok {
				rq.options.Shape, err = hh.ParseShape(v)
			}
		case "--frame":
			if v, ok = value(); ok {
				rq.options.Frame, err = hh.ParseFrame(v)
			}
		case "--background":
			if v, ok = value(); ok {
				rq.options.Background, err = hh.ParseBackground(v)
			}
		case "--frame-alpha":
			if v, ok = value(); ok {
				var alpha int
				alpha, ok = parseNumber(v)
				ok = ok && alpha <= 255
				rq.options.FrameAlpha = hh.Alpha(uint8(alpha))
			}
		case "--format":
			rq.format, ok = value()
			formatGiven = true
		case "--quality":
			if v, ok = value(); ok {
				rq.quality, ok = parseNumber(v)
			}
		case "--matte":
			if v, ok = value(); ok {
				rq.matte, err = hh.ParseRGB(v)
			}
		case "--out":
			outPath, ok = value()
		default:
			if haveInput || strings.HasPrefix(arg, "--") {
				ok = false
			}
			rq.input = []byte(arg)
			haveInput = true
		}
		if !ok || err != nil {
			io.WriteString(stderr, usage)
			return 2
		}
	}
	if !haveInput {
		io.WriteString(stderr, usage)
		return 2
	}
	// The extension chooses the format as hh_cli does it: what follows the last
	// dot of a path of more than four bytes, in lower case only.
	if !formatGiven && len(outPath) > 4 {
		switch outPath[strings.LastIndexByte(outPath, '.')+1:] {
		case "bmp":
			rq.format = "bmp"
		case "rgba":
			rq.format = "rgba"
		case "jpg", "jpeg":
			rq.format = "jpeg"
		}
	}

	r, err := run(rq)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", strings.TrimPrefix(err.Error(), "hh: "))
		return 1
	}
	layout := r.fingerprint.Layout()
	fp := r.fingerprint.Bytes()
	tag := r.fingerprint.Tag()
	fmt.Fprintf(stdout, "mode         %v\n", layout.Mode)
	fmt.Fprintf(stdout, "base digest  %v\n", r.digest)
	fmt.Fprintf(stdout, "fingerprint  %x\n", fp[:])
	fmt.Fprintf(stdout, "tag          %s-%s\n", tag[:3], tag[3:])
	if r.kcv != "" {
		fmt.Fprintf(stdout, "key check    %s\n", r.kcv)
	}
	for row := 0; row < 4; row++ {
		label := "cells        "
		if row > 0 {
			label = "             "
		}
		io.WriteString(stdout, label)
		for _, c := range layout.Cells[4*row : 4*row+4] {
			colour := byte(' ')
			if c.Figure != hh.FigureNone {
				colour = '0' + c.Color
			}
			fmt.Fprintf(stdout, "%c%c ", ".SO^>v<"[c.Figure], colour)
		}
		io.WriteString(stdout, "\n")
	}
	if outPath != "" {
		if err := os.WriteFile(outPath, r.bytes, 0o666); err != nil {
			fmt.Fprintf(stderr, "cannot write %s\n", outPath)
			return 1
		}
		fmt.Fprintf(stdout, "wrote        %s (%d bytes)\n", outPath, len(r.bytes))
	}
	return 0
}

func main() {
	args := os.Args[1:]
	switch {
	case len(args) == 3 && args[0] == "--generate":
		count, errCount := strconv.Atoi(args[1])
		seed, errSeed := strconv.ParseUint(args[2], 10, 64)
		if errCount != nil || errSeed != nil || count < 0 {
			io.WriteString(os.Stderr, usage)
			os.Exit(2)
		}
		generate(count, seed, os.Stdout)
	default:
		os.Exit(single(args, os.Stdout, os.Stderr))
	}
}
