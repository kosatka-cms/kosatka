package form

import (
	"context"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	tooLong := make([]byte, 256)
	for i := range tooLong {
		tooLong[i] = 'a'
	}

	cases := []struct {
		name     string
		slug     string
		wantFail bool
		wantMsg  string
	}{
		{"empty is valid", "", false, ""},
		{"valid slug", "articles/foo-2099", false, ""},
		{"valid slug no path", "foo", false, ""},
		{"too long", string(tooLong), true, "The URL alias must not exceed 255 characters"},
		{"uppercase char", "Foo", true, "The URL alias may contain only lowercase Latin letters, digits, hyphens and slashes"},
		{"invalid char", "foo!", true, "The URL alias may contain only lowercase Latin letters, digits, hyphens and slashes"},
		{"leading hyphen", "-foo", true, "The URL alias cannot start or end with a hyphen"},
		{"trailing hyphen", "foo-", true, "The URL alias cannot start or end with a hyphen"},
		{"double hyphen", "bat--man", true, "The URL alias cannot contain consecutive hyphens"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg, failed := validateSlug(context.Background(), c.slug)
			if failed != c.wantFail {
				t.Fatalf("validateSlug(%q) failed=%v, want %v", c.slug, failed, c.wantFail)
			}
			if failed && msg != c.wantMsg {
				t.Fatalf("validateSlug(%q) message=%q, want %q", c.slug, msg, c.wantMsg)
			}
		})
	}
}

func TestValidateSlugMultipleFailuresLastWins(t *testing.T) {
	// Too long and double hyphen: the double-hyphen check runs last, so its
	// message is the one left in the errors map.
	b := make([]byte, 300)
	for i := range b {
		b[i] = 'a'
	}
	b[10] = '-'
	b[11] = '-'
	slug := string(b)

	msg, failed := validateSlug(context.Background(), slug)
	if !failed {
		t.Fatal("expected failure")
	}
	want := "The URL alias cannot contain consecutive hyphens"
	if msg != want {
		t.Fatalf("message=%q, want %q (last-rule-wins ordering)", msg, want)
	}
}

func TestValidateRequiredTrimmed(t *testing.T) {
	const msg = "required field"

	cases := []struct {
		name     string
		value    string
		wantFail bool
	}{
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"non-empty", "x", false},
		{"non-empty with padding", "  x  ", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, failed := validateRequiredTrimmed(c.value, msg)
			if failed != c.wantFail {
				t.Fatalf("failed=%v, want %v", failed, c.wantFail)
			}
			if failed && got != msg {
				t.Fatalf("message=%q, want %q", got, msg)
			}
			if !failed && got != "" {
				t.Fatalf("message=%q, want empty", got)
			}
		})
	}
}

func TestValidateMaxLen(t *testing.T) {
	const msg = "too long"

	cases := []struct {
		name     string
		value    string
		max      int
		wantFail bool
	}{
		{"at boundary", "12345", 5, false},
		{"one over boundary", "123456", 5, true},
		{"well under", "1", 5, false},
		{"empty", "", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, failed := validateMaxLen(c.value, c.max, msg)
			if failed != c.wantFail {
				t.Fatalf("failed=%v, want %v", failed, c.wantFail)
			}
			if failed && got != msg {
				t.Fatalf("message=%q, want %q", got, msg)
			}
		})
	}
}

func TestValidateDateYMD(t *testing.T) {
	const msg = "invalid date format"

	cases := []struct {
		name     string
		value    string
		wantFail bool
	}{
		{"empty is valid (optional field)", "", false},
		{"valid date", "2026-09-01", false},
		{"invalid format", "01-09-2026", true},
		{"garbage", "not-a-date", true},
		{"valid date with time suffix rejected", "2026-09-01T00:00:00", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, failed := validateDateYMD(c.value, msg)
			if failed != c.wantFail {
				t.Fatalf("failed=%v, want %v", failed, c.wantFail)
			}
			if failed && got != msg {
				t.Fatalf("message=%q, want %q", got, msg)
			}
		})
	}
}
