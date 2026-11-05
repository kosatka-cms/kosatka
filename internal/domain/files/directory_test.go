package files

import (
	"errors"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

func TestResolveFileDirectory(t *testing.T) {
	// Dates are taken in the site's time zone.
	if err := sitetime.Set("Europe/Moscow"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sitetime.Set("UTC") })
	tokens := FileDirectoryTokens{
		Now:         time.Date(2026, 10, 31, 22, 0, 0, 0, time.UTC), // 1 Nov in UTC+3
		ContentType: "article",
		UserID:      42,
	}
	for pattern, want := range map[string]string{
		"":   "content/images",
		"  ": "content/images",
		"content/[date:custom:Y]-[date:custom:m]": "content/2026-11",
		"[node:type]/[date:custom:d]":             "article/01",
		"uploads/[current-user:uid]":              "uploads/42",
		// Cyrillic on purpose: checks transliteration into a directory name.
		"/Обложки Героев/": "oblozhki_geroev",
	} {
		got, err := ResolveFileDirectory(pattern, "content/images", tokens)
		if err != nil || got != want {
			t.Errorf("ResolveFileDirectory(%q) = %q, %v; want %q", pattern, got, err, want)
		}
	}
	for _, bad := range []string{
		"content/[node:id]", // unknown token
		"content/../etc",
		"C:/files",
		"styles/thumb",
		"STYLES",
		"resize",
		"user_pictures/x",
		"u42/images",
		"a//!!!",
	} {
		if _, err := ResolveFileDirectory(bad, "content/images", tokens); !errors.Is(err, ErrInvalidFileDirectory) {
			t.Errorf("ResolveFileDirectory(%q) err = %v, want ErrInvalidFileDirectory", bad, err)
		}
	}
	if err := ValidateFileDirectory("uploads/users"); err != nil {
		t.Errorf("a non-reserved u-prefixed name is fine: %v", err)
	}
}
