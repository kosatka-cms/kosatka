package schema

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestIndexNamesUniqueAcrossTables - an explicit index name
// (StorageKey) is used once: PostgreSQL scopes index names to the whole
// schema, not to the table as MySQL does.
func TestIndexNamesUniqueAcrossTables(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`index\.Fields\([^)]*\)[^,\n]*\.StorageKey\("([^"]+)"\)`)
	seen := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if prev, ok := seen[m[1]]; ok {
				t.Errorf("index name %q in both %s and %s", m[1], prev, f)
			}
			seen[m[1]] = f
		}
	}
}
