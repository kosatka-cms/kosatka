package entity

import (
	"slices"
	"testing"
)

func TestFieldStorageDefConfigAccessors(t *testing.T) {
	f := FieldStorageDef{
		Config: map[string]any{
			"vocabulary_id":       float64(12), // JSON-decoded numbers are float64
			"target_content_type": "events",
			"max_len":             float64(500),
		},
	}
	if got := f.VocabularyID(); got != 12 {
		t.Errorf("VocabularyID() = %d, want 12", got)
	}
	if got := f.TargetContentType(); got != "events" {
		t.Errorf("TargetContentType() = %q, want %q", got, "events")
	}
	if got := f.MaxLen(); got != 500 {
		t.Errorf("MaxLen() = %d, want 500", got)
	}
	if !f.HasExplicitMaxLen() {
		t.Errorf("HasExplicitMaxLen() = false, want true")
	}

	empty := FieldStorageDef{}
	if got := empty.VocabularyID(); got != 0 {
		t.Errorf("VocabularyID() on empty config = %d, want 0", got)
	}
	if got := empty.TargetContentType(); got != "" {
		t.Errorf("TargetContentType() on empty config = %q, want empty", got)
	}
	if got := empty.MaxLen(); got != 255 {
		t.Errorf("MaxLen() on empty config = %d, want default 255", got)
	}
	if empty.HasExplicitMaxLen() {
		t.Errorf("HasExplicitMaxLen() on empty config = true, want false")
	}
}

// TestFieldStorageUsageRemovable - a storage leaves the catalog only when
// no instance, View, breadcrumb or SEO rule holds it, and SEO rules were
// checked.
func TestFieldStorageUsageRemovable(t *testing.T) {
	cases := []struct {
		name string
		u    FieldStorageUsage
		want bool
	}{
		{"nothing holds it", FieldStorageUsage{}, true},
		{"an instance", FieldStorageUsage{Instances: 1}, false},
		{"a view", FieldStorageUsage{Views: 1}, false},
		{"a breadcrumb rule", FieldStorageUsage{BreadcrumbRules: 1}, false},
		{"an SEO rule", FieldStorageUsage{SEORules: 2}, false},
	}
	for _, c := range cases {
		if got := c.u.Removable(); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if n := (FieldStorageUsage{Views: 2, BreadcrumbRules: 1, SEORules: 3}).References(); n != 6 {
		t.Errorf("references: %d", n)
	}
}

// TestFileIDs - an image or file field's value as its files: one, several
// in order, or none; the first of them.
func TestFileIDs(t *testing.T) {
	for _, tc := range []struct {
		v     any
		want  []int
		first int
	}{{7, []int{7}, 7}, {[]int{3, 1}, []int{3, 1}, 3}, {0, nil, 0}, {nil, nil, 0}, {"7", nil, 0}, {[]int{}, []int{}, 0}} {
		if got := FileIDs(tc.v); !slices.Equal(got, tc.want) || FirstFileID(tc.v) != tc.first {
			t.Errorf("%#v: %v %d, want %v %d", tc.v, got, FirstFileID(tc.v), tc.want, tc.first)
		}
	}
	if !FieldTypeImage.AllowsMultiple() || FieldTypeFile.AllowsMultiple() {
		t.Error("images hold several values, files one")
	}
}
