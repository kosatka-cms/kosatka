package accounts

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestDefaultAccountSettingsLanguage - the built-in e-mail texts are in
// the site's language.
func TestDefaultAccountSettingsLanguage(t *testing.T) {
	if got := DefaultAccountSettings().SubjWelcomeOpen; got != "Welcome to [site:name]" {
		t.Errorf("source language: %q", got)
	}
	i18n.SetDefault("ru")
	t.Cleanup(func() { i18n.SetDefault(i18n.SourceLanguage) })
	d := DefaultAccountSettings()
	if d.SubjWelcomeOpen != "Добро пожаловать на [site:name]" || !strings.Contains(d.BodyEmailConfirm, "[user:confirm-email-url]") || !strings.HasPrefix(d.BodyEmailConfirm, "Здравствуйте") {
		t.Errorf("ru: %q / %q", d.SubjWelcomeOpen, d.BodyEmailConfirm)
	}
}
