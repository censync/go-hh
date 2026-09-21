package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	address = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	keyHex  = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	digest  = "e212927148fcf76f6669c244a0db08bdd4f36dc50a378f6a1a3fe472807e7852"
	keyed   = "26ea8171aab23c8e1bf7c23417d33d6dba81d881af70edd2b0675348e080b478"
	// The base digest of the text input whose only byte is ff, as hh_cli of hh-cpp prints it.
	notUTF8 = "4a6bd25a8411e1401a222181b32bf82a4bcd2cc2847d3c0f038cea1c4280192e"
)

func golden(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The lines are those hh_cli of hh-cpp prints for the same arguments.
func TestSingle(t *testing.T) {
	out := filepath.Join(t.TempDir(), "address.png")
	var stdout, stderr bytes.Buffer
	if code := single([]string{address, "--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d: %s", code, stderr.String())
	}
	want := "mode         universal\n" +
		"base digest  " + digest + "\n" +
		"fingerprint  " + digest + "\n" +
		"tag          TKS-PVH\n" +
		"cells        <0 .  ^2 O2 \n" +
		"             S1 <3 <2 O1 \n" +
		"             O0 O1 v0 S0 \n" +
		"             >0 v3 .  >3 \n" +
		"wrote        " + out + " (4360 bytes)\n"
	if stdout.String() != want {
		t.Errorf("printed\n%s\nwant\n%s", stdout.String(), want)
	}
	if written, _ := os.ReadFile(out); !bytes.Equal(written, golden(t, "evm-1-universal-128.png")) {
		t.Error("the file is not the golden picture")
	}
}

func TestSingleKeyed(t *testing.T) {
	out := filepath.Join(t.TempDir(), "private.bmp")
	var stdout, stderr bytes.Buffer
	args := []string{"--key", keyHex, "--size", "48", "--out", out, address}
	if code := single(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d: %s", code, stderr.String())
	}
	for _, line := range []string{"mode         keyed\n", "fingerprint  " + keyed + "\n", "key check    6a5955cf\n", "(6966 bytes)\n"} {
		if !strings.Contains(stdout.String(), line) {
			t.Errorf("%q is missing in\n%s", line, stdout.String())
		}
	}
	if written, _ := os.ReadFile(out); !bytes.HasPrefix(written, []byte("BM")) {
		t.Error("the extension did not choose the format")
	}
}

func TestSingleFailures(t *testing.T) {
	for _, c := range []struct {
		args   []string
		code   int
		stderr string
	}{
		{[]string{"0xzz"}, 1, "error: invalid_hex: "},
		{[]string{address, "--size", "15"}, 1, "error: invalid_size: "},
		{[]string{address, "--frame", "thick"}, 1, "error: invalid_frame: "},
		{[]string{address, "--key", "00"}, 1, "error: invalid_key: "},
		{[]string{address, "--format", "gif"}, 1, "error: invalid_argument: "},
		{[]string{address, "--format", "jpeg", "--quality", "49"}, 1, "error: invalid_quality: "},
		{[]string{}, 2, "usage: "},
		{[]string{address, address}, 2, "usage: "},
		{[]string{address, "--size"}, 2, "usage: "},
		{[]string{address, "--shape", "oval"}, 2, "usage: "},
		{[]string{address, "--frame-alpha", "256"}, 2, "usage: "},
		{[]string{address, "--frame-alpha", "+5"}, 2, "usage: "},
		{[]string{address, "--frame-alpha", "-0"}, 2, "usage: "},
		{[]string{address, "--frame-alpha", ""}, 2, "usage: "},
		{[]string{address, "--size", "+64"}, 2, "usage: "},
		{[]string{address, "--size", "-64"}, 2, "usage: "},
		{[]string{address, "--size", "6_4"}, 2, "usage: "},
		{[]string{address, "--size", "64px"}, 2, "usage: "},
		{[]string{address, "--size", " 64"}, 2, "usage: "},
		{[]string{address, "--size", "\u0666\u0664"}, 2, "usage: "},
		{[]string{address, "--size", "10000000000"}, 2, "usage: "},
		{[]string{address, "--size", "9999999999"}, 1, "error: invalid_size: "},
		{[]string{address, "--size", "0000000064", "--quality", "-92"}, 2, "usage: "},
		{[]string{address, "--quality", "+92"}, 2, "usage: "},
		{[]string{address, "--quality", "9e1"}, 2, "usage: "},
		{[]string{address, "--format", "jpeg", "--quality", "4294967388"}, 1, "error: invalid_quality: "},
		{[]string{"--batch", "cases.txt"}, 2, "usage: "},
		{[]string{"--batch", filepath.Join(t.TempDir(), "missing.txt"), t.TempDir()}, 1, "cannot read "},
		{[]string{address, "--matte", "fff"}, 2, "usage: "},
		{[]string{address, "--verbose"}, 2, "usage: "},
		{[]string{"--help"}, 0, "usage: "},
	} {
		var stdout, stderr bytes.Buffer
		if code := single(c.args, &stdout, &stderr); code != c.code || !strings.HasPrefix(stderr.String(), c.stderr) {
			t.Errorf("%v: exit code %d, %q", c.args, code, stderr.String())
		}
	}
}

func TestNumbersAreDigitsWithoutASign(t *testing.T) {
	for text, want := range map[string]int{
		"0": 0, "7": 7, "064": 64, "0000000064": 64, "255": 255, "2147483647": 2147483647, "2147483648": 2147483647,
		"4294967295": 2147483647, "4294967296": 2147483647, "9999999999": 2147483647,
	} {
		if got, ok := parseNumber(text); !ok || got != want {
			t.Errorf("parseNumber(%q): %d, %v", text, got, ok)
		}
	}
	for _, bad := range []string{
		"", "+1", "-1", "-0", "1_0", "1 ", " 1", "1.0", "1e3", "0x10", "ten", "00000000064", "10000000000",
		"99999999999999999999", "\u0661", "\uff11", "1\x00",
	} {
		if got, ok := parseNumber(bad); ok {
			t.Errorf("parseNumber(%q): %d", bad, got)
		}
	}
}

func TestFieldsAreSeparatedBySpacesAndTabsAlone(t *testing.T) {
	for line, want := range map[string][]string{
		"a b":                     {"a", "b"},
		" \ta \t\t b\t ":          {"a", "b"},
		"a\vb a\fb a\u00a0b":      {"a\vb", "a\fb", "a\u00a0b"},
		"a\u2003b a\u0085b a\rb":  {"a\u2003b", "a\u0085b", "a\rb"},
		"\xff\xfe \xc3":           {"\xff\xfe", "\xc3"},
		"   ":                     nil,
		"a\x00b \x1f":             {"a\x00b", "\x1f"},
		"hex 0x00 - 64 # comment": {"hex", "0x00", "-", "64", "#", "comment"},
	} {
		got := splitFields(line)
		if len(got) != len(want) {
			t.Errorf("%q: %q", line, got)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q: %q", line, got)
			}
		}
	}
}

// Only a lower-case extension of a path of more than four bytes chooses the
// format, as in hh_cli.
func TestTheExtensionChoosesTheFormat(t *testing.T) {
	dir := t.TempDir()
	for name, magic := range map[string]string{
		"a.bmp": "BM", "a.jpg": "\xff\xd8", "a.jpeg": "\xff\xd8", "a.png": "\x89PNG", "a.BMP": "\x89PNG",
		"a.Jpeg": "\x89PNG", "abmp": "\x89PNG", "a.bmp.txt": "\x89PNG", "a.gif": "\x89PNG",
		"picture": "\x89PNG", "a.txt.jpg": "\xff\xd8",
	} {
		var stdout, stderr bytes.Buffer
		out := filepath.Join(dir, name)
		if code := single([]string{address, "--size", "16", "--out", out}, &stdout, &stderr); code != 0 {
			t.Fatalf("%s: exit code %d: %s", name, code, stderr.String())
		}
		if written, _ := os.ReadFile(out); !bytes.HasPrefix(written, []byte(magic)) {
			t.Errorf("%s begins with % x", name, written[:4])
		}
	}
	// Raw pixels have no signature; the size tells.
	var stdout, stderr bytes.Buffer
	out := filepath.Join(dir, "a.rgba")
	single([]string{address, "--size", "16", "--out", out}, &stdout, &stderr)
	if written, _ := os.ReadFile(out); len(written) != 16*16*4 {
		t.Errorf("a.rgba has %d bytes", len(written))
	}
}

func TestBatch(t *testing.T) {
	dir := t.TempDir()
	cases := strings.Join([]string{
		"# comments and empty lines are not cases",
		"",
		"hex " + address + " - 128 square automatic ffffffff 255 png 92 ffffff",
		"hex " + address + " " + keyHex + " 128 square automatic 121212ff 255 png 92 ffffff",
		"text 54 - 64 round plain 00000000 128 rgba 92 ffffff",
		"hex 0xzz " + keyHex[2:] + " 15 square ticks 9e9e9eff 255 gif 49 ffffff",
		"hex " + address + " " + keyHex[2:] + " 15 square ticks 9e9e9eff 255 gif 49 ffffff",
		"hex " + address + " - 15 square ticks 9e9e9eff 255 gif 49 ffffff",
		"hex " + address + " - 64 square ticks 9e9e9eff 255 gif 49 ffffff",
		"hex " + address + " - 64 square none 9e9e9eff 255 gif 49 ffffff",
		"hex " + address + " - 64 square none ffffffff 255 gif 49 ffffff",
		"hex " + address + " - 64 square none ffffffff 255 jpeg 49 ffffff",
		"binary " + address + " - 64 square none ffffffff 255 png 92 ffffff",
		"text zz - 64 square none ffffffff 255 png 92 ffffff",
		"hex " + address + " - 64 square none ffffffff 256 png 92 ffffff",
		"hex " + address + " - 64 square none ffffffff 255 png 92",
		" ",
		"hex " + address + " - 4294967296 square none ffffffff 255 png 92 ffffff",
		"hex " + address + " - 4294967295 square none ffffffff 255 png 92 ffffff",
		"hex " + address + " - 10000000000 square none ffffffff 255 png 92 ffffff",
		"hex " + address + " - +64 square none ffffffff 255 png 92 ffffff",
		"hex " + address + " - 64 square none ffffffff 255 png 92 ffffff extra",
		"hex " + address + " - 64\fsquare none ffffffff 255 png 92 ffffff",
		"hex " + address + " - 64\u00a0square none ffffffff 255 png 92 ffffff",
		"hex\t" + address + " \t- 64 square none ffffffff 255 jpeg 9999999999 ffffff \r",
		"\r",
		"text ff - 16 square none ffffffff 0 bmp 0 000000",
	}, "\n")
	file := filepath.Join(dir, "cases.txt")
	if err := os.WriteFile(file, []byte(cases), 0o666); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := batch(file, filepath.Join(dir, "out"), &stdout); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	want := []string{
		"1\tok\t" + digest + "\t" + digest + "\tTKSPVH\t-",
		"2\tok\t" + digest + "\t" + keyed + "\tQA0XH0\t6a5955cf",
		"", // a text input: checked below
		"4\tinvalid_hex", "5\tinvalid_key", "6\tinvalid_size", "7\tinvalid_frame", "8\tlow_contrast",
		"9\tinvalid_argument", "10\tinvalid_quality", "11\tbad_case", "12\tbad_case", "13\tbad_case", "14\tbad_case",
		"15\tbad_case", "16\tinvalid_size", "17\tinvalid_size", "18\tbad_case", "19\tbad_case", "20\tbad_case",
		"21\tbad_case", "22\tbad_case", "23\tinvalid_quality",
		// The byte ff is no UTF-8 and goes to the library as it is.
		"24\tok\t" + notUTF8 + "\t" + notUTF8 + "\t9F6JSG\t-",
	}
	if len(lines) != len(want) {
		t.Fatalf("%d lines:\n%s", len(lines), stdout.String())
	}
	for i, line := range lines {
		if i == 2 {
			if !strings.HasPrefix(line, "3\tok\t") || !strings.HasSuffix(line, "\t-") || line[5:69] == digest {
				t.Errorf("printed %q", line)
			}
		} else if line != want[i] {
			t.Errorf("printed %q, want %q", line, want[i])
		}
	}
	for name, goldenName := range map[string]string{
		"case-1.png": "evm-1-universal-128.png",
		"case-2.png": "evm-1-keyed-128-dark.png",
	} {
		if written, _ := os.ReadFile(filepath.Join(dir, "out", name)); !bytes.Equal(written, golden(t, goldenName)) {
			t.Errorf("%s is not %s", name, goldenName)
		}
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "out", "case-3.rgba")); len(raw) != 64*64*4 {
		t.Errorf("case-3.rgba has %d bytes", len(raw))
	}
	if bmp, _ := os.ReadFile(filepath.Join(dir, "out", "case-24.bmp")); len(bmp) != 54+16*16*3 {
		t.Errorf("case-24.bmp has %d bytes", len(bmp))
	}
	if files, _ := os.ReadDir(filepath.Join(dir, "out")); len(files) != 4 {
		t.Errorf("%d files were written", len(files))
	}
}

// The hand-made cases of tools/crosscheck.sh: bytes that are not UTF-8, a CR LF
// and a last line without an end. What hh_cli of hh-cpp answers to each line is
// the matter of the crosscheck; here every case gets one well-formed answer.
func TestTheEdgeCases(t *testing.T) {
	file := filepath.Join("..", "..", "tools", "edge-cases.txt")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	cases := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		if line = bytes.TrimSuffix(line, []byte("\r")); len(line) > 0 && line[0] != '#' {
			cases++
		}
	}
	dir := t.TempDir()
	var stdout bytes.Buffer
	if err := batch(file, dir, &stdout); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != cases || cases < 100 {
		t.Fatalf("%d answers to %d cases", len(lines), cases)
	}
	counts := map[string]int{}
	for i, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] != strconv.Itoa(i+1) || (fields[1] == "ok") != (len(fields) == 6) {
			t.Fatalf("answer %d is %q", i+1, line)
		}
		counts[fields[1]]++
	}
	for _, name := range []string{
		"ok", "bad_case", "invalid_hex", "invalid_key", "invalid_size", "invalid_frame", "low_contrast",
		"invalid_quality", "invalid_argument",
	} {
		if counts[name] == 0 {
			t.Errorf("no case answers %s", name)
		}
		delete(counts, name)
	}
	if len(counts) != 0 {
		t.Errorf("unexpected answers: %v", counts)
	}
	if files, _ := os.ReadDir(dir); len(files) == 0 {
		t.Error("no file was written")
	}
}

// The generator is deterministic and its cases are well-formed; most of them
// render.
func TestGenerate(t *testing.T) {
	var first, second, other bytes.Buffer
	generate(150, 1, &first)
	generate(150, 1, &second)
	generate(150, 2, &other)
	if !bytes.Equal(first.Bytes(), second.Bytes()) || bytes.Equal(first.Bytes(), other.Bytes()) {
		t.Fatal("the cases do not depend on the seed alone")
	}
	lines := strings.Split(strings.TrimSuffix(first.String(), "\n"), "\n")
	if len(lines) != 151 || lines[0] != "# 150 cases, seed 1" {
		t.Fatalf("%d lines, the first is %q", len(lines), lines[0])
	}
	rendered := 0
	for _, line := range lines[1:] {
		rq, ok := parseCase(splitFields(line))
		if !ok {
			t.Fatalf("a bad case: %s", line)
		}
		if rq.size > 300 {
			continue // large pictures take long and add nothing here
		}
		if _, err := run(rq); err == nil {
			rendered++
		}
	}
	if rendered < 75 {
		t.Errorf("only %d of 150 cases render", rendered)
	}
}
