package hh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// The byte of the keys that the printing tests use, and every form in which a
// printer could show a run of them. One byte alone proves nothing: an address
// may contain "ab".
const loudByte = 0xAB

var loudForms = []string{
	"abababab", "ab ab ab ab", "\xab\xab", "\u00ab\u00ab", "\u00ab \u00ab", "\\xab\\xab", "u+00ab u+00ab",
	"171 171", "171, 171", "171,171", "0xab, 0xab", "0xab 0xab", "10101011 10101011", "253 253", "0253 0253",
	"0o253 0o253", "q6ur",
}

func loudKey(t *testing.T) *SecretKey {
	t.Helper()
	key, err := NewSecretKey(bytes.Repeat([]byte{loudByte}, KeySize))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { key.Close() })
	return key
}

func isLoud(text string) bool {
	// ToLower replaces bytes that are not UTF-8, so the text is searched as
	// well.
	lower := strings.ToLower(text)
	for _, form := range loudForms {
		if strings.Contains(lower, form) || strings.Contains(text, form) {
			return true
		}
	}
	return false
}

func checkQuiet(t *testing.T, what, text string) {
	t.Helper()
	if isLoud(text) {
		t.Errorf("%s shows key bytes: %q", what, text)
	}
}

// The check recognises the bytes in every form fmt and encoding/json give them.
func TestTheLoudFormsAreRecognised(t *testing.T) {
	state := loudKey(t).secret()
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "% x", "%d", "%b", "%o", "%O", "%c", "%U"} {
		if text := fmt.Sprintf(verb, state); !isLoud(text) {
			t.Errorf("%s of the state is not recognised: %q", verb, text)
		}
	}
	encoded, _ := json.Marshal(state.bytes[:])
	numbers, _ := json.Marshal(state.bytes)
	if !isLoud(string(encoded)) || !isLoud(string(numbers)) {
		t.Errorf("JSON of the bytes is not recognised: %s %s", encoded, numbers)
	}
}

var everyVerb = []string{
	"%v", "%+v", "%#v", "%T", "%s", "%q", "%x", "%X", "% x", "%#x", "%d", "%b", "%o", "%O", "%c", "%U", "%e", "%g",
	"%p", "%t", "%10v", "%-10s", "%.3s", "%z",
}

func TestAKeyPrintsAsAPlaceholder(t *testing.T) {
	key := loudKey(t)
	for _, verb := range everyVerb {
		got := fmt.Sprintf(verb, key)
		checkQuiet(t, verb, got)
		// fmt answers %T and %p itself, with the type and the address.
		if verb != "%T" && verb != "%p" && got != "hh.SecretKey(***)" {
			t.Errorf("%s of a key: %q", verb, got)
		}
	}
	if got := fmt.Sprint(key); got != "hh.SecretKey(***)" {
		t.Errorf("Sprint: %q", got)
	}
	if got := key.String(); got != "hh.SecretKey(***)" {
		t.Errorf("String: %q", got)
	}
	var none *SecretKey
	if got := fmt.Sprint(none) + none.String(); got != "hh.SecretKey(***)hh.SecretKey(***)" {
		t.Errorf("a nil key: %q", got)
	}
	var _ fmt.Formatter = key
	var _ fmt.Stringer = key
}

func TestNoFormOfAKeyPrintsItsBytes(t *testing.T) {
	key := loudKey(t)
	type embedsPointer struct {
		*SecretKey
		Name string
	}
	type embedsValue struct {
		SecretKey
		Name string
	}
	type hidesPointer struct {
		name string
		key  *SecretKey
	}
	type hidesValue struct {
		name string
		key  SecretKey
	}
	var boxed any = key
	for name, value := range map[string]any{
		"a dereferenced key":          *key,
		"a pointer to a pointer":      &key,
		"an embedded pointer":         embedsPointer{key, "embedded"},
		"a pointer to an embedding":   &embedsPointer{key, "embedded"},
		"an embedded value":           embedsValue{*key, "embedded"},
		"a pointer to a value inside": &embedsValue{*key, "embedded"},
		"an unexported pointer":       hidesPointer{"hidden", key},
		"an unexported value":         &hidesValue{"hidden", *key},
		"a slice":                     []*SecretKey{key},
		"a slice of values":           []SecretKey{*key},
		"an array of values":          [1]SecretKey{*key},
		"a map":                       map[string]*SecretKey{"key": key},
		"a map of values":             map[string]SecretKey{"key": *key},
		"an interface":                &boxed,
		"the state function":          key.state,
	} {
		for _, verb := range everyVerb {
			checkQuiet(t, name+" with "+verb, fmt.Sprintf(verb, value))
		}
		checkQuiet(t, name+" with Sprint", fmt.Sprint(value))
	}
	// A dereferenced key shows where its state function is, and nothing else.
	if got := fmt.Sprintf("%v", *key); !strings.HasPrefix(got, "{0x") || len(got) > 20 {
		t.Errorf("a dereferenced key: %q", got)
	}
}

func TestAKeyIsEmptyInJSONAndQuietInALog(t *testing.T) {
	key := loudKey(t)
	for name, value := range map[string]any{
		"a key":              key,
		"a dereferenced key": *key,
		"a struct":           struct{ Key *SecretKey }{key},
		"a struct of values": struct{ Key SecretKey }{*key},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		checkQuiet(t, name+" in JSON", string(encoded))
		if got := string(encoded); got != `{}` && got != `{"Key":{}}` {
			t.Errorf("%s in JSON: %s", name, got)
		}
	}

	var text, structured bytes.Buffer
	for _, logger := range []*slog.Logger{
		slog.New(slog.NewTextHandler(&text, nil)), slog.New(slog.NewJSONHandler(&structured, nil)),
	} {
		logger.Info("keys", "pointer", key, "value", *key, "any", slog.AnyValue(key),
			"group", slog.GroupValue(slog.Any("inner", key)), "text", fmt.Sprintf("%v %+v", key, *key))
	}
	checkQuiet(t, "the text log", text.String())
	checkQuiet(t, "the JSON log", structured.String())
	if !strings.Contains(text.String(), `pointer=hh.SecretKey(***)`) {
		t.Errorf("the text log: %s", text.String())
	}
	if !strings.Contains(structured.String(), `"pointer":{}`) {
		t.Errorf("the JSON log: %s", structured.String())
	}
}

// reachableBytes walks a value the way a dumping package does: through
// pointers, interfaces and unexported fields. It returns every byte array and
// byte slice it reaches.
func reachableBytes(v reflect.Value, seen map[uintptr]bool, found *[][]byte) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		reachableBytes(v.Elem(), seen, found)
	case reflect.Interface:
		if !v.IsNil() {
			reachableBytes(v.Elem(), seen, found)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			reachableBytes(v.Field(i), seen, found)
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			reachableBytes(it.Key(), seen, found)
			reachableBytes(it.Value(), seen, found)
		}
	case reflect.Array, reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, v.Len())
			for i := range b {
				b[i] = byte(v.Index(i).Uint())
			}
			*found = append(*found, b)
			return
		}
		for i := 0; i < v.Len(); i++ {
			reachableBytes(v.Index(i), seen, found)
		}
	}
}

func TestReflectionDoesNotReachTheKeyBytes(t *testing.T) {
	key := loudKey(t)
	// The walk finds the bytes of a state it is handed, so it would find them
	// behind a key as well if a pointer led there.
	var found [][]byte
	reachableBytes(reflect.ValueOf(key.secret()), map[uintptr]bool{}, &found)
	if len(found) != 2 || !bytes.Equal(found[0], bytes.Repeat([]byte{loudByte}, KeySize)) {
		t.Fatalf("the walk over the state found %x", found)
	}
	for name, value := range map[string]any{
		"a key":              key,
		"a dereferenced key": *key,
		"a struct":           &struct{ key *SecretKey }{key},
	} {
		found = nil
		reachableBytes(reflect.ValueOf(value), map[uintptr]bool{}, &found)
		if len(found) != 0 {
			t.Errorf("%s leads to %x", name, found)
		}
	}
}

func TestCopiesOfAKeyAreOneKey(t *testing.T) {
	key, err := NewSecretKey(testKeyBytes())
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := ImportBaseDigest(unhex(t, testDigestHex))
	duplicate := *key
	if fp, err := Keyed(digest, &duplicate); err != nil || fp != mustFingerprint(t, testKeyedHex, ModeKeyed) {
		t.Errorf("a copy: %v", err)
	}
	if duplicate.KCV() != key.KCV() {
		t.Error("the copy has another KCV")
	}
	duplicate.Close()
	if _, err := Keyed(digest, key); err != ErrInvalidKey {
		t.Errorf("the key after its copy was closed: %v", err)
	}
	if key.secret().bytes != [KeySize]byte{} {
		t.Error("closing the copy left key bytes behind")
	}
}

func TestTheZeroSecretKeyIsNotAKey(t *testing.T) {
	var zero SecretKey
	if _, err := Keyed(BaseDigest{1}, &zero); err != ErrInvalidKey {
		t.Errorf("Keyed: %v", err)
	}
	if zero.KCV() != [KCVSize]byte{} {
		t.Error("KCV of the zero SecretKey")
	}
	if err := zero.Close(); err != nil {
		t.Error(err)
	}
	if got := fmt.Sprint(&zero); got != "hh.SecretKey(***)" {
		t.Errorf("Sprint: %q", got)
	}
}

// SPEC.md section 4 makes the key check value public: it is computed when the
// key is created and outlives the key. Nothing else does.
func TestOnlyTheKCVOutlivesAClosedKey(t *testing.T) {
	key, err := NewSecretKey(testKeyBytes())
	if err != nil {
		t.Fatal(err)
	}
	before := key.KCV()
	digest, _ := ImportBaseDigest(unhex(t, testDigestHex))
	want := mustFingerprint(t, testKeyedHex, ModeKeyed)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(closes bool) {
			defer wg.Done()
			if closes {
				key.Close()
			}
			fp, err := Keyed(digest, key)
			if err != ErrInvalidKey && (err != nil || fp != want) {
				t.Errorf("a use that races with Close: %v", err)
			}
			if key.KCV() != before {
				t.Error("the KCV changed")
			}
		}(i == 4)
	}
	wg.Wait()
	if _, err := Keyed(digest, key); err != ErrInvalidKey {
		t.Errorf("Keyed with a closed key: %v", err)
	}
	if after := key.KCV(); after != before || fmt.Sprintf("%x", after) != "6a5955cf" {
		t.Errorf("KCV after Close: %x", after)
	}
}
