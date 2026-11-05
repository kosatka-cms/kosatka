package fsname

import (
	"regexp"
	"testing"
)

var testSegmentRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func TestSanitizeImageStem(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Vacation 2024", "Vacation_2024"},
		{"  Мой_снимок  ", "moy_snimok"},
		{"photo.jpg", "photo"},
		{"", "image"},
		{"___", "image"},
	}
	for _, c := range cases {
		got := SanitizeImageStem(c.in)
		if got != c.want {
			t.Errorf("SanitizeImageStem(%q) = %q, want %q", c.in, got, c.want)
		}
		if !testSegmentRe.MatchString(got) {
			t.Errorf("stem %q is not a safe segment", got)
		}
	}
}

func TestNormalizeFolderSegmentRussian(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Мои фото", "moi_foto"},
		{"test", "test"},
		{"  Album-1  ", "album-1"},
		{"Ёлка", "yolka"},
	}
	for _, c := range cases {
		got := NormalizeFolderSegment(c.in)
		if got != c.want {
			t.Errorf("NormalizeFolderSegment(%q) = %q, want %q", c.in, got, c.want)
		}
		if !testSegmentRe.MatchString(got) {
			t.Errorf("result %q does not match segment re", got)
		}
	}
}

func TestFileStem(t *testing.T) {
	for in, want := range map[string]string{
		"Foo Poster.JPG":          "foo-poster",
		"Фото: Начало (2005).png": "foto-nachalo-2005",
		"  ../../etc/passwd":      "passwd",
		"a__b--c.webp":            "a__b-c",
		"!!!.jpg":                 "fallback",
		"":                        "fallback",
	} {
		if got := FileStem(in, "fallback"); got != want {
			t.Errorf("FileStem(%q) = %q, want %q", in, got, want)
		}
	}
}
