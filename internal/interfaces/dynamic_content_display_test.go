package interfaces

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestResolveImageFieldFileID(t *testing.T) {
	tests := []struct {
		name           string
		raw            any
		defaultImageID *int
		want           int
	}{
		{"nil value, no default -> 0", nil, nil, 0},
		{"nil value, default set -> default", nil, intPtr(7), 7},
		{"positive value -> value, default ignored", 5, intPtr(7), 5},
		{"zero value -> default", 0, intPtr(7), 7},
		{"negative value -> default", -1, intPtr(7), 7},
		{"non-int value -> default", "not-an-int", intPtr(7), 7},
		{"zero value, no default -> 0", 0, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveImageFieldFileID(tt.raw, tt.defaultImageID)
			if got != tt.want {
				t.Fatalf("resolveImageFieldFileID(%v, %v) = %d, want %d", tt.raw, tt.defaultImageID, got, tt.want)
			}
		})
	}
}

// Every image field shown on the page is its own section in the
// standard output (a theme places a picture by hand from fieldImages).
func TestBuildDisplaySectionsImageSections(t *testing.T) {
	fields := entity.FieldInstanceDefs{
		{ID: 1, MachineName: "cover_image", FieldType: entity.FieldTypeImage, ShowOnDetail: true},
		{ID: 2, MachineName: "content", FieldType: entity.FieldTypeText, ShowOnDetail: true, RendersInBody: true},
		{ID: 3, MachineName: "hidden_image", FieldType: entity.FieldTypeImage},
	}
	dc := &entity.DynamicContent{Fields: fields, Values: map[int]any{1: 10, 2: "<p>Text</p>", 3: 11}}
	sections := buildDisplaySections(fields, dc, nil, map[int]string{1: "/a.webp", 3: "/b.webp"})
	if len(sections) != 2 || sections[0].Image == nil || sections[0].Image.ID != 1 || sections[1].Body == nil {
		t.Fatalf("sections = %+v", sections)
	}
}

// A redirect field's value is followed only as a site-relative path or
// an http(s) URL - never a scheme-relative //host or javascript:.
func TestRedirectTarget(t *testing.T) {
	for in, want := range map[any]string{
		"/articles/foo":         "/articles/foo",
		" /events/born-again ":  "/events/born-again",
		"https://example.com/x": "https://example.com/x",
		"http://example.com":    "http://example.com",
		"//evil.example":        "",
		"javascript:alert(1)":   "",
		"articles/foo":          "",
		"":                      "",
		nil:                     "",
		42:                      "",
	} {
		if got := redirectTarget(in); got != want {
			t.Errorf("redirectTarget(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestReferenceFieldNodes - both single- and multi-value reference fields
// seed the search widget with their current targets; a target that no
// longer resolves is skipped.
func TestReferenceFieldNodes(t *testing.T) {
	c := &DynamicContentController{nodeRepo: referenceNodes{}}
	fields := entity.FieldInstanceDefs{
		{ID: 1, FieldType: entity.FieldTypeReference},
		{ID: 2, FieldType: entity.FieldTypeReference, Config: map[string]any{"multiple": true}},
		{ID: 3, FieldType: entity.FieldTypeReference},
		{ID: 4, FieldType: entity.FieldTypeText},
	}
	got := c.referenceFieldNodes(context.Background(), fields, map[int]any{1: 7, 2: []int{8, 404, 9}, 3: 404, 4: 5})
	if n := got[1]; len(n) != 1 || n[0].ID != 7 {
		t.Errorf("single-value: %+v", n)
	}
	if n := got[2]; len(n) != 2 || n[0].ID != 8 || n[1].ID != 9 {
		t.Errorf("multi-value: %+v", n)
	}
	if n, ok := got[3]; ok {
		t.Errorf("missing target: %+v", n)
	}
	if _, ok := got[4]; ok {
		t.Error("a text field resolved as a reference")
	}
}
