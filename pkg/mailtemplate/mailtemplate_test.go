package mailtemplate

import "testing"

func TestReplacePlaceholders(t *testing.T) {
	// A module's own placeholders come in Extra.
	tpl := "Hi [user:name], visit [site:login-url] or reply to [pm:sender] about [pm:subject]."
	data := TokenData{
		UserName:     "Alice",
		SiteLoginURL: "https://example.com/login",
		Extra:        map[string]string{"[pm:sender]": "Bob", "[pm:subject]": "Hello"},
	}

	got := ReplacePlaceholders(tpl, data)
	want := "Hi Alice, visit https://example.com/login or reply to Bob about Hello."
	if got != want {
		t.Fatalf("ReplacePlaceholders() = %q, want %q", got, want)
	}
}

func TestReplacePlaceholdersLeavesUnknownUnmatched(t *testing.T) {
	got := ReplacePlaceholders("no placeholders here", TokenData{})
	if got != "no placeholders here" {
		t.Fatalf("expected text without placeholders to be unchanged, got %q", got)
	}
}
