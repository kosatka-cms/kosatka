// Package mailtemplate substitutes [site:...]/[user:...] placeholders, and
// a module's own (TokenData.Extra), in email subject/body templates.
package mailtemplate

import (
	"context"
	"strings"
)

// TokenData holds values for [site:...] and [user:...] placeholders in
// email bodies/subjects, and a module's own in Extra.
type TokenData struct {
	SiteName             string
	SiteSlogan           string
	SiteLoginURL         string
	SiteURLBrief         string // scheme + host, no path
	SitePeoplePendingURL string
	UserName             string
	UserEmail            string
	ConfirmEmailURL      string // /user/confirm-email?token=
	OneTimeLoginURL      string
	ResetPasswordURL     string // /user/reset-password?token=
	UserEditURL          string
	// Extra - a module's own placeholders and their values
	// ("[pm:sender]" → a name), substituted after the ones above.
	Extra map[string]string
}

// ReplacePlaceholders substitutes [token] placeholders in template text (plain).
func ReplacePlaceholders(tpl string, d TokenData) string {
	repl := []struct{ old, new string }{
		{"[site:name]", d.SiteName},
		{"[site:slogan]", d.SiteSlogan},
		{"[site:login-url]", d.SiteLoginURL},
		{"[site:url-brief]", d.SiteURLBrief},
		{"[site:people-pending-url]", d.SitePeoplePendingURL},
		{"[user:name]", d.UserName},
		{"[user:email]", d.UserEmail},
		{"[user:confirm-email-url]", d.ConfirmEmailURL},
		{"[user:one-time-login-url]", d.OneTimeLoginURL},
		{"[user:reset-password-url]", d.ResetPasswordURL},
		{"[user:edit-url]", d.UserEditURL},
	}
	s := tpl
	for _, p := range repl {
		s = strings.ReplaceAll(s, p.old, p.new)
	}
	for token, value := range d.Extra {
		s = strings.ReplaceAll(s, token, value)
	}
	return s
}

// Mailer sends one of the site's emails by its key: the site keeps its
// template and whether it is sent; data fills in the placeholders.
type Mailer interface {
	Send(ctx context.Context, key, to string, data TokenData) error
}
