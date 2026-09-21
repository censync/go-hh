package hh

import (
	"os"
	"strings"
	"testing"
)

// The digest cache of docs/INTEGRATION.md is the code of example_cache_test.go,
// which the compiler and the example runner check.
func TestTheCacheOfTheIntegrationGuideIsCompiled(t *testing.T) {
	guide, err := os.ReadFile("docs/INTEGRATION.md")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("example_cache_test.go")
	if err != nil {
		t.Fatal(err)
	}
	_, block, found := strings.Cut(string(guide), "```go\ntype digestCache struct {")
	block, _, closed := strings.Cut(block, "```")
	if !found || !closed {
		t.Fatal("docs/INTEGRATION.md has no digestCache")
	}
	block = "type digestCache struct {" + block
	if !strings.Contains(strings.ReplaceAll(string(source), "\t", "    "), block) {
		t.Errorf("example_cache_test.go does not contain the code of the guide:\n%s", block)
	}
}
