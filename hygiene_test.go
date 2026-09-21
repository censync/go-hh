package hh

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// The rules of the package that a compiler does not enforce, checked on its
// source: the standard library packages it may import, integer arithmetic only,
// and a doc comment on every exported identifier.

var allowedImports = map[string]bool{
	"crypto/hmac":     true,
	"crypto/sha256":   true,
	"encoding":        true, // the interfaces TextMarshaler and TextUnmarshaler
	"encoding/binary": true,
	"encoding/hex":    true,
	"fmt":             true,
	"hash/adler32":    true,
	"hash/crc32":      true,
	"image":           true, // for Image.NRGBA alone
	"io":              true,
	"math/bits":       true,
	"strconv":         true,
	"sync":            true,
	"unicode/utf8":    true,
}

func packageFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = file
	}
	if len(files) < 10 {
		t.Fatalf("only %d source files were found", len(files))
	}
	return files
}

func TestThePackageImportsOnlyWhatItMay(t *testing.T) {
	used := map[string]bool{}
	for name, file := range packageFiles(t) {
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			used[path] = true
			if !allowedImports[path] {
				t.Errorf("%s imports %s", name, path)
			}
		}
	}
	// The list is what the package imports, not what it once might.
	for path := range allowedImports {
		if !used[path] {
			t.Errorf("%s is allowed and nothing imports it", path)
		}
	}
}

// CONTRIBUTING.md lists the imports in one sentence; it and the list above say
// the same.
func TestTheContributionRulesNameTheImports(t *testing.T) {
	rules, err := os.ReadFile("CONTRIBUTING.md")
	if err != nil {
		t.Fatal(err)
	}
	_, sentence, found := strings.Cut(string(rules), "standard library packages and no others:")
	sentence, _, _ = strings.Cut(sentence, ". ")
	if !found {
		t.Fatal("CONTRIBUTING.md has no list of imports")
	}
	named := map[string]bool{}
	for i, part := range strings.Split(sentence, "`") {
		if i%2 == 1 {
			named[part] = true
			if !allowedImports[part] {
				t.Errorf("CONTRIBUTING.md names %s, which the package may not import", part)
			}
		}
	}
	for path := range allowedImports {
		if !named[path] {
			t.Errorf("CONTRIBUTING.md does not name %s", path)
		}
	}
}

func TestThePackageUsesNoFloatingPoint(t *testing.T) {
	for name, file := range packageFiles(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BasicLit:
				if n.Kind == token.FLOAT || n.Kind == token.IMAG {
					t.Errorf("%s has the floating-point literal %s", name, n.Value)
				}
			case *ast.Ident:
				switch n.Name {
				case "float32", "float64", "complex64", "complex128":
					t.Errorf("%s uses %s", name, n.Name)
				}
			}
			return true
		})
	}
}

func TestEveryExportedIdentifierIsDocumented(t *testing.T) {
	for name, file := range packageFiles(t) {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() && d.Doc == nil {
					t.Errorf("%s: func %s has no doc comment", name, d.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() && d.Doc == nil && s.Doc == nil {
							t.Errorf("%s: type %s has no doc comment", name, s.Name)
						}
					case *ast.ValueSpec:
						for _, id := range s.Names {
							if id.IsExported() && d.Doc == nil && s.Doc == nil && s.Comment == nil {
								t.Errorf("%s: %s has no doc comment", name, id)
							}
						}
					}
				}
			}
		}
	}
}
