package hh

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// textValue is what every type with a text form has.
type textValue interface {
	fmt.Stringer
	encoding.TextMarshaler
}

func TestTheTextFormIsTheStringForm(t *testing.T) {
	values := []textValue{
		ShapeSquare, ShapeRound, FrameAutomatic, FrameGaps, RGB{}, White, RGB{0x7A, 0x96, 0xC5}, Background{},
		Transparent(), Translucent(RGB{1, 2, 3}, 4), Opacity{}, Alpha(0), Alpha(7), Alpha(200), ModeUniversal, ModeKeyed,
	}
	for f := FrameAutomatic; f <= FrameGaps; f++ {
		values = append(values, f)
	}
	for _, v := range values {
		text, err := v.MarshalText()
		if err != nil || string(text) != v.String() {
			t.Errorf("%T %v: %q, %v", v, v, text, err)
		}
		// Round trip through the matching UnmarshalText.
		var back textValue
		switch v.(type) {
		case Shape:
			var x Shape
			err = x.UnmarshalText(text)
			back = x
		case Frame:
			var x Frame
			err = x.UnmarshalText(text)
			back = x
		case RGB:
			var x RGB
			err = x.UnmarshalText(text)
			back = x
		case Background:
			var x Background
			err = x.UnmarshalText(text)
			back = x
		case Opacity:
			var x Opacity
			err = x.UnmarshalText(text)
			back = x
		case Mode:
			var x Mode
			err = x.UnmarshalText(text)
			back = x
		}
		if err != nil || back != v {
			t.Errorf("%T %v came back as %v, %v", v, v, back, err)
		}
	}
}

func TestOpacityPrintsItsAlpha(t *testing.T) {
	for _, c := range []struct {
		alpha uint8
		want  string
	}{{0, "0"}, {7, "7"}, {55, "55"}, {200, "200"}, {255, "255"}} {
		if got := Alpha(c.alpha).String(); got != c.want {
			t.Errorf("Alpha(%d).String() = %q", c.alpha, got)
		}
		if got := fmt.Sprintf("%v %s", Alpha(c.alpha), Alpha(c.alpha)); got != c.want+" "+c.want {
			t.Errorf("Alpha(%d) prints %q", c.alpha, got)
		}
	}
	if got := (Opacity{}).String(); got != "255" {
		t.Errorf("the zero Opacity: %q", got)
	}
	opts := RenderOptions{Shape: ShapeRound, Frame: FrameDouble, Background: Transparent(), FrameAlpha: Alpha(200)}
	if got := fmt.Sprintf("%v", opts); got != "{round double 00000000 200}" {
		t.Errorf("options print as %q", got)
	}
}

func TestParseOpacity(t *testing.T) {
	for alpha := 0; alpha <= 255; alpha++ {
		text := fmt.Sprint(alpha)
		if got, err := ParseOpacity(text); err != nil || got != Alpha(uint8(alpha)) || got.String() != text {
			t.Errorf("ParseOpacity(%q): %v, %v", text, got, err)
		}
	}
	for text, want := range map[string]uint8{"007": 7, "00": 0, "0000000000000000000000255": 255} {
		if got, err := ParseOpacity(text); err != nil || got.Alpha() != want {
			t.Errorf("ParseOpacity(%q): %v, %v", text, got, err)
		}
	}
	for _, bad := range []string{
		"", " ", "256", "1000", "99999999999999999999999999", "+5", "-0", "-1", "2_5", " 5", "5 ", "5\n", "0x10", "ff",
		"1e2", "2.0", "٢٥", "２５", "opaque",
	} {
		if got, err := ParseOpacity(bad); err != ErrInvalidArgument || got != (Opacity{}) {
			t.Errorf("ParseOpacity(%q): %v, %v", bad, got, err)
		}
	}
}

func TestParseMode(t *testing.T) {
	for _, m := range []Mode{ModeUniversal, ModeKeyed} {
		if got, err := ParseMode(m.String()); err != nil || got != m {
			t.Errorf("ParseMode(%q): %v, %v", m.String(), got, err)
		}
	}
	for _, bad := range []string{"", "invalid", "Keyed", "keyed ", "1", "private"} {
		if got, err := ParseMode(bad); err != ErrInvalidArgument || got != 0 {
			t.Errorf("ParseMode(%q): %v, %v", bad, got, err)
		}
	}
}

func TestValuesOutsideTheTablesHaveNoTextForm(t *testing.T) {
	for _, v := range []encoding.TextMarshaler{Shape(2), Shape(255), Frame(10), Frame(255), Mode(0), Mode(3)} {
		if text, err := v.MarshalText(); err != ErrInvalidArgument || text != nil {
			t.Errorf("%T(%d): %q, %v", v, v, text, err)
		}
	}
	if _, err := json.Marshal(RenderOptions{Frame: Frame(10)}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("JSON of an unknown frame: %v", err)
	}
}

func TestATextThatIsRefusedLeavesTheValueAlone(t *testing.T) {
	shape, frame, colour, background, opacity, mode :=
		ShapeRound, FrameTicks, RGB{1, 2, 3}, Transparent(), Alpha(9), ModeKeyed
	for name, err := range map[string]error{
		"shape":      shape.UnmarshalText([]byte("circle")),
		"frame":      frame.UnmarshalText([]byte("Double")),
		"colour":     colour.UnmarshalText([]byte("#ffffff")),
		"background": background.UnmarshalText([]byte("ffffff")),
		"opacity":    opacity.UnmarshalText([]byte("256")),
		"mode":       mode.UnmarshalText([]byte("")),
		"no text":    shape.UnmarshalText(nil),
	} {
		if err != ErrInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
	if shape != ShapeRound || frame != FrameTicks || colour != (RGB{1, 2, 3}) || background != Transparent() ||
		opacity != Alpha(9) || mode != ModeKeyed {
		t.Error("a refused text changed the value")
	}
	for name, err := range map[string]error{
		"shape":      (*Shape)(nil).UnmarshalText([]byte("round")),
		"frame":      (*Frame)(nil).UnmarshalText([]byte("double")),
		"colour":     (*RGB)(nil).UnmarshalText([]byte("ffffff")),
		"background": (*Background)(nil).UnmarshalText([]byte("ffffffff")),
		"opacity":    (*Opacity)(nil).UnmarshalText([]byte("255")),
		"mode":       (*Mode)(nil).UnmarshalText([]byte("keyed")),
	} {
		if err != ErrInvalidArgument {
			t.Errorf("a nil %s: %v", name, err)
		}
	}
}

func TestRenderOptionsPassThroughJSONUnchanged(t *testing.T) {
	backgrounds := []Background{
		{}, Transparent(), Opaque(RGB{0x12, 0x12, 0x12}), Translucent(RGB{0xFE, 0x01, 0x80}, 1), Translucent(White, 0),
	}
	n := 0
	for shape := ShapeSquare; shape <= ShapeRound; shape++ {
		for frame := FrameAutomatic; frame <= FrameGaps; frame++ {
			for i, background := range backgrounds {
				for _, alpha := range []uint8{0, 1, 127, 200, 254, 255} {
					opts := RenderOptions{Shape: shape, Frame: frame, Background: background, FrameAlpha: Alpha(alpha)}
					encoded, err := json.Marshal(opts)
					if err != nil {
						t.Fatalf("%+v: %v", opts, err)
					}
					want := fmt.Sprintf(`{"Shape":"%v","Frame":"%v","Background":"%v","FrameAlpha":"%d"}`,
						shape, frame, background, alpha)
					if string(encoded) != want {
						t.Fatalf("%s, want %s", encoded, want)
					}
					// The target is not the default, so every field must be read.
					back := RenderOptions{
						Shape: 1 - shape, Frame: 9 - frame, Background: backgrounds[(i+1)%5], FrameAlpha: Alpha(^alpha),
					}
					if err := json.Unmarshal(encoded, &back); err != nil || back != opts {
						t.Fatalf("%s came back as %+v, %v", encoded, back, err)
					}
					n++
				}
			}
		}
	}
	if n != 2*10*5*6 {
		t.Errorf("%d combinations", n)
	}
}

func TestRenderOptionsInAConfigurationFile(t *testing.T) {
	zero, err := json.Marshal(RenderOptions{})
	if err != nil || string(zero) != `{"Shape":"square","Frame":"automatic","Background":"ffffffff","FrameAlpha":"255"}` {
		t.Errorf("the default look: %s, %v", zero, err)
	}
	var opts RenderOptions
	if err := json.Unmarshal([]byte(`{}`), &opts); err != nil || opts != (RenderOptions{}) {
		t.Errorf("an empty object: %+v, %v", opts, err)
	}
	// A field that is missing keeps its default; hexadecimal digits of either case are read.
	if err := json.Unmarshal([]byte(`{"Frame": "plain", "Background": "0A0B0C80"}`), &opts); err != nil ||
		opts != (RenderOptions{Frame: FramePlain, Background: Translucent(RGB{0x0A, 0x0B, 0x0C}, 0x80)}) {
		t.Errorf("two fields: %+v, %v", opts, err)
	}
	for _, bad := range []string{
		`{"Shape": "circle"}`, `{"Shape": ""}`, `{"Frame": "Double"}`, `{"Frame": "invalid"}`,
		`{"Background": "ffffff"}`, `{"Background": "#ffffffff"}`, `{"Background": "transparent"}`,
		`{"FrameAlpha": "256"}`, `{"FrameAlpha": "-1"}`, `{"FrameAlpha": "+1"}`, `{"FrameAlpha": "opaque"}`,
	} {
		var target RenderOptions
		if err := json.Unmarshal([]byte(bad), &target); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: %v", bad, err)
		}
	}
	// Nothing but a string is a text: a number or an object is an error of
	// encoding/json, not the default look.
	for _, bad := range []string{`{"FrameAlpha": 200}`, `{"Shape": 1}`, `{"Background": {}}`, `{"Frame": ["double"]}`} {
		var target RenderOptions
		if err := json.Unmarshal([]byte(bad), &target); err == nil {
			t.Errorf("%s was read as %+v", bad, target)
		}
	}
}

func TestALayoutInJSON(t *testing.T) {
	layout := mustFingerprint(t, testKeyedHex, ModeKeyed).Layout()
	encoded, err := json.Marshal(layout)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"Mode":"keyed"`, `"Palette":["7a96c5","890af0","c10445","d48200"]`, `"FrameColor":"808080"`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("%s has no %s", encoded, want)
		}
	}
	var back Layout
	if err := json.Unmarshal(encoded, &back); err != nil || back != layout {
		t.Errorf("the layout came back as %+v, %v", back, err)
	}
}

func TestTextFormsAsMapKeys(t *testing.T) {
	encoded, err := json.Marshal(map[Frame]RGB{FrameDouble: White})
	if err != nil || string(encoded) != `{"double":"ffffff"}` {
		t.Errorf("%s, %v", encoded, err)
	}
	var back map[Frame]RGB
	if err := json.Unmarshal(encoded, &back); err != nil || len(back) != 1 || back[FrameDouble] != White {
		t.Errorf("%v, %v", back, err)
	}
}
