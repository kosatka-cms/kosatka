package service

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestSiteURLApply - the admin's URL applies unless the config sets one;
// listeners get the effective URL.
func TestSiteURLApply(t *testing.T) {
	prev := entity.NewSystemSettings().Domain
	t.Cleanup(func() { entity.SetSiteDomain(prev) })

	s := NewSiteURL("")
	var heard string
	s.OnChange(func(u string) { heard = u })
	s.Apply(" https://saved.example/ ")
	if got := entity.NewSystemSettings().Domain; got != "https://saved.example" || heard != got {
		t.Errorf("admin URL: domain %q, listener %q", got, heard)
	}

	s = NewSiteURL("https://config.example/")
	s.Apply("https://saved.example")
	if got := entity.NewSystemSettings().Domain; got != "https://config.example" || s.Override() != got {
		t.Errorf("config URL must win: %q", got)
	}
}

func TestValidBaseURL(t *testing.T) {
	for u, want := range map[string]bool{
		"":                         true,
		"https://www.example.com":  true,
		"http://localhost:3333/":   true,
		"www.example.com":          false,
		"ftp://example.com":        false,
		"https://example.com/blog": false,
		"https://example.com?x=1":  false,
		"https://user@example.com": false,
	} {
		if got := ValidBaseURL(u); got != want {
			t.Errorf("ValidBaseURL(%q) = %v", u, got)
		}
	}
}
