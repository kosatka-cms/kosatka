package slug

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"Привет Мир", "privet-mir"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"Already-Slugged", "already-slugged"},
		{"Ёлка", "yolka"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Generate(c.in); got != c.want {
			t.Errorf("Generate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

type fakeChecker struct {
	existing map[string]bool
	err      error
}

func (f *fakeChecker) SlugExists(ctx context.Context, slug, contentType string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.existing[slug], nil
}

func TestEnsureUniqueReturnsBaseWhenFree(t *testing.T) {
	checker := &fakeChecker{existing: map[string]bool{}}
	got, err := EnsureUnique(context.Background(), "my-slug", "article", checker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "my-slug" {
		t.Fatalf("EnsureUnique() = %q, want %q", got, "my-slug")
	}
}

func TestEnsureUniqueAppendsSuffix(t *testing.T) {
	checker := &fakeChecker{existing: map[string]bool{
		"my-slug":   true,
		"my-slug-2": true,
	}}
	got, err := EnsureUnique(context.Background(), "my-slug", "article", checker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "my-slug-3" {
		t.Fatalf("EnsureUnique() = %q, want %q", got, "my-slug-3")
	}
}

func TestEnsureUniqueEmptyBase(t *testing.T) {
	checker := &fakeChecker{}
	if _, err := EnsureUnique(context.Background(), "", "article", checker); err == nil {
		t.Fatal("expected error for empty base slug")
	}
}

func TestEnsureUniquePropagatesCheckerError(t *testing.T) {
	wantErr := errors.New("db down")
	checker := &fakeChecker{err: wantErr}
	if _, err := EnsureUnique(context.Background(), "my-slug", "article", checker); err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		slug    string
		wantErr bool
	}{
		{"valid-slug-123", false},
		{"", true},
		{"Has-Upper", true},
		{"-leading-hyphen", true},
		{"trailing-hyphen-", true},
		{"double--hyphen", true},
		{"has_underscore", true},
	}
	for _, c := range cases {
		err := Validate(c.slug)
		if (err != nil) != c.wantErr {
			t.Errorf("Validate(%q) error = %v, wantErr %v", c.slug, err, c.wantErr)
		}
	}
}

func TestMachineName(t *testing.T) {
	for in, want := range map[string]string{
		"Контакты в подвале":   "kontakty_v_podvale",
		"Footer \u2014 links!": "footer_links",
		"2026 banner":          "block_2026_banner",
		"***":                  "block",
		"":                     "block",
	} {
		if got := MachineName(in, "block"); got != want {
			t.Errorf("MachineName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := MachineName(strings.Repeat("a", 100), "block"); len(got) != 56 {
		t.Errorf("long: %d", len(got))
	}
}
