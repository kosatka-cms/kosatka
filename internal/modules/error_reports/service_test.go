package errorreports

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidateErrorReportPageURL(t *testing.T) {
	host := "example.com"
	tests := []struct {
		url  string
		want error
	}{
		{"/articles/foo", nil},
		{"/path?x=1", nil},
		{"", ErrErrorReportPageURL},
		{"https://evil.com/page", ErrErrorReportPageURL},
		{"https://example.com/page", nil},
		{"//evil.com/x", ErrErrorReportPageURL},
	}
	for _, tc := range tests {
		got := ValidateErrorReportPageURL(tc.url, host)
		if !errors.Is(got, tc.want) {
			t.Errorf("ValidateErrorReportPageURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
	// No site address to compare with: a path only.
	if err := ValidateErrorReportPageURL("https://example.com/page", ""); !errors.Is(err, ErrErrorReportPageURL) {
		t.Errorf("an address without the site's: %v", err)
	}
	if got := siteHostOf("https://www.example.com"); got != "www.example.com" {
		t.Errorf("the site's host: %q", got)
	}
}

type fakeReportRepo struct {
	ErrorReportRepository
	created []ErrorReport
}

func (f *fakeReportRepo) Create(_ context.Context, r ErrorReport) (*ErrorReport, error) {
	f.created = append(f.created, r)
	return &r, nil
}

// TestCreateCountsCharacters - the lengths are in characters, as the
// form's maxlength: a comment of 1500 Cyrillic letters (3000 bytes) is
// taken, one of 2001 refused; the user agent is cut to 512 characters, a
// letter whole.
func TestCreateCountsCharacters(t *testing.T) {
	repo := &fakeReportRepo{}
	s := NewErrorReportService(repo, nil, nil, nil, nil)
	in := ErrorReportCreateInput{PageURL: "/article", SelectedText: "опечтка", UserAgent: strings.Repeat("я", 600)}
	in.Comment = strings.Repeat("ж", 1500)
	if _, err := s.Create(context.Background(), in, "example.com", ""); err != nil {
		t.Fatalf("1500 letters: %v", err)
	}
	if ua := repo.created[0].UserAgent; utf8.RuneCountInString(ua) != 512 || !utf8.ValidString(ua) {
		t.Errorf("the user agent: %d characters, valid %v", utf8.RuneCountInString(ua), utf8.ValidString(ua))
	}
	in.Comment = strings.Repeat("ж", ErrorReportCommentMax+1)
	if _, err := s.Create(context.Background(), in, "example.com", ""); !errors.Is(err, ErrErrorReportComment) {
		t.Errorf("2001 letters: %v", err)
	}
}
