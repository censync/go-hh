package hh_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"

	hh "github.com/censync/go-hh"
)

// The key of the examples. A real key is uniformly random or the output of a
// key derivation function, and never appears in source code.
var exampleKey = []byte("an example key, 32 bytes long...")

// The picture of an EVM address that everyone can compute.
func Example() {
	digest, err := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed") // slow: cache it
	if err != nil {
		fmt.Println(err)
		return
	}
	fp := hh.Universal(digest)
	img, err := hh.Render(fp, 128, hh.RenderOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	png, err := img.EncodePNG()
	if err != nil {
		fmt.Println(err)
		return
	}
	tag := fp.Tag()
	fmt.Printf("%d x %d pixels, %d bytes of PNG, tag %s-%s\n", img.Width, img.Height, len(png), tag[:3], tag[3:])
	// Output:
	// 128 x 128 pixels, 4360 bytes of PNG, tag TKS-PVH
}

// The private picture: only holders of the key can compute it.
func ExampleKeyed() {
	key, err := hh.NewSecretKey(exampleKey)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer key.Close()

	digest, _ := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	fp, err := hh.Keyed(digest, key)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%v %s, key check value %x\n", fp.Mode(), fp.Tag(), key.KCV())
	// Output:
	// keyed 1KKSCS, key check value 6ae70605
}

// Every spelling of an address gives the same digest, and its 32 bytes are what
// a host caches.
func ExampleBaseDigestFromHex() {
	a, _ := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	b, _ := hh.BaseDigestFromHex("5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED")
	fmt.Println(a == b)

	cached := a[:]
	restored, _ := hh.ImportBaseDigest(cached)
	fmt.Println(restored)
	// Output:
	// true
	// e212927148fcf76f6669c244a0db08bdd4f36dc50a378f6a1a3fe472807e7852
}

// Formats that exist only as text, such as Bitcoin addresses, are hashed as
// text.
func ExampleBaseDigestFromText() {
	digest, err := hh.BaseDigestFromText("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4")
	fmt.Println(digest, err)
	// Output:
	// dc705192e4a205d8c403ae7693290df45f09cec04116ad38f6140f349392548f <nil>
}

// A host that computes the keyed HMAC elsewhere, for example in a secure
// element, imports the result and never creates a SecretKey.
func ExampleImportFingerprint() {
	fromElsewhere := make([]byte, hh.FingerprintSize)
	fp, err := hh.ImportFingerprint(fromElsewhere, hh.ModeKeyed)
	fmt.Println(fp.Mode(), fp.Tag(), err)

	_, err = hh.ImportFingerprint(fromElsewhere[:16], hh.ModeKeyed)
	fmt.Println(err)
	// Output:
	// keyed 000000 <nil>
	// hh: invalid_fingerprint: the fingerprint must be 32 bytes with a known mode
}

// A round picture with a double frame on a dark surface. Every style that fits
// the shape is open to both modes; one that does not is refused.
func ExampleRender() {
	key, _ := hh.NewSecretKey(exampleKey)
	defer key.Close()
	digest, _ := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	fp, _ := hh.Keyed(digest, key)

	opts := hh.RenderOptions{
		Shape:      hh.ShapeRound,
		Frame:      hh.FrameDouble,
		Background: hh.Opaque(hh.RGB{R: 0x12, G: 0x12, B: 0x12}),
	}
	img, err := hh.Render(fp, 96, opts)
	fmt.Println(img.Width, img.Height, len(img.Pix), err)

	// The same look for the public picture.
	img, err = hh.Render(hh.Universal(digest), 96, opts)
	fmt.Println(img.Width, img.Height, len(img.Pix), err)

	// Rounded corners need the square shape.
	opts.Frame = hh.FrameRounded
	_, err = hh.Render(fp, 96, opts)
	fmt.Println(err)
	// Output:
	// 96 96 36864 <nil>
	// 96 96 36864 <nil>
	// hh: invalid_frame: the frame is not allowed for this shape
}

// Errors are values: compare them, or read the numeric code that every
// implementation of hh shares.
func ExampleError() {
	_, err := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAe") // one digit short
	fmt.Println(errors.Is(err, hh.ErrInvalidHex))

	var e *hh.Error
	if errors.As(err, &e) {
		fmt.Println(int(e.Code()), e.Code())
	}
	// Output:
	// true
	// 3 invalid_hex
}

// The look of a program in its configuration file. Every field of RenderOptions
// reads and writes the names and the hexadecimal forms of the specification.
func ExampleRenderOptions() {
	type config struct {
		PictureSize int
		Picture     hh.RenderOptions
	}
	file := []byte(`{"PictureSize": 96, "Picture": {"Shape": "round", "Frame": "double", "Background": "121212ff"}}`)
	var c config
	if err := json.Unmarshal(file, &c); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(c.Picture.Shape, c.Picture.Frame, c.Picture.Background, c.Picture.FrameAlpha)

	written, _ := json.Marshal(c.Picture)
	fmt.Println(string(written))

	err := json.Unmarshal([]byte(`{"Picture": {"Frame": "dotted"}}`), &c)
	fmt.Println(errors.Is(err, hh.ErrInvalidArgument))
	// Output:
	// round double 121212ff 255
	// {"Shape":"round","Frame":"double","Background":"121212ff","FrameAlpha":"255"}
	// true
}

// Figures that vanish into the background make different pictures look alike,
// so a host that lets users choose a background measures it first.
func ExampleMeasureContrast() {
	page := hh.RGB{R: 0x12, G: 0x12, B: 0x12} // what the host paints underneath
	for _, background := range []hh.Background{
		hh.Transparent(),
		hh.Opaque(hh.RGB{R: 0xF2, G: 0xF2, B: 0xF2}),
		hh.Opaque(hh.RGB{R: 0x9E, G: 0x9E, B: 0x9E}),
	} {
		report := hh.MeasureContrast(hh.RenderOptions{Background: background}, page)
		switch {
		case report.FiguresX100 < 200:
			fmt.Printf("%v: %d, Render refuses it if it is opaque\n", background, report.FiguresX100)
		case report.FiguresX100 < 300:
			fmt.Printf("%v: %d, warn the user\n", background, report.FiguresX100)
		default:
			fmt.Printf("%v: %d\n", background, report.FiguresX100)
		}
	}
	// Output:
	// 00000000: 300
	// f2f2f2ff: 268, warn the user
	// 9e9e9eff: 112, Render refuses it if it is opaque
}

// The pixels have the layout of image.NRGBA, so the standard image packages
// take them as they are.
func ExampleImage_NRGBA() {
	digest, _ := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	img, _ := hh.Render(hh.Universal(digest), 64, hh.RenderOptions{Background: hh.Transparent()})

	canvas := image.NewRGBA(image.Rect(0, 0, 200, 80))
	draw.Draw(canvas, image.Rect(8, 8, 72, 72), img.NRGBA(), image.Point{}, draw.Over)
	fmt.Println(canvas.RGBAAt(8+10, 8+10), canvas.RGBAAt(8+31, 8+31))
	// Output:
	// {122 150 197 255} {0 0 0 0}
}

// A host that draws vectors itself reads the cells instead of the pixels.
func ExampleFingerprint_Layout() {
	digest, _ := hh.BaseDigestFromHex("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	layout := hh.Universal(digest).Layout()
	for i, cell := range layout.Cells {
		if cell.Figure == hh.FigureNone {
			fmt.Printf("[%21s]", "")
		} else {
			fmt.Printf("[%-14v %v]", cell.Figure, layout.Palette[cell.Color])
		}
		if i%4 == 3 {
			fmt.Println()
		}
	}
	// Output:
	// [triangle_left  7a96c5][                     ][triangle_up    c10445][circle         c10445]
	// [square         890af0][triangle_left  d48200][triangle_left  c10445][circle         890af0]
	// [circle         7a96c5][circle         890af0][triangle_down  7a96c5][square         7a96c5]
	// [triangle_right 7a96c5][triangle_down  d48200][                     ][triangle_right d48200]
}
