package entity

import (
	"context"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The core's forms that can show the CAPTCHA (CaptchaSettings.Forms);
// modules add theirs (Module.CaptchaForms).
const (
	CaptchaFormLogin    = "login"
	CaptchaFormRegister = "register"
	CaptchaFormPassword = "password"
	// CaptchaFormComment - the comment form, for authors without a
	// published comment yet.
	CaptchaFormComment = "comment"
)

// CoreCaptchaForms - the core's CAPTCHA forms with their labels
// (translation sources), shown by default.
var CoreCaptchaForms = []ModuleCaptchaForm{
	{CaptchaFormLogin, i18n.N("Login form"), true},
	{CaptchaFormRegister, i18n.N("Registration"), true},
	{CaptchaFormPassword, i18n.N("Password reset"), true},
	{CaptchaFormComment, i18n.N("Comment form (authors without a published comment)"), false},
}

// CaptchaForms - every CAPTCHA form, in the order of the settings page:
// the core's, then the registered modules'.
func CaptchaForms() []ModuleCaptchaForm {
	out := append([]ModuleCaptchaForm(nil), CoreCaptchaForms...)
	for _, m := range Modules {
		out = append(out, m.CaptchaForms...)
	}
	return out
}

// CaptchaFormNames - the keys of CaptchaForms().
func CaptchaFormNames() []string {
	forms := CaptchaForms()
	out := make([]string, len(forms))
	for i, f := range forms {
		out[i] = f.Key
	}
	return out
}

// CaptchaSettings - reCAPTCHA v2 keys and the forms that show it (one
// row). Empty keys mean the server config's keys apply.
type CaptchaSettings struct {
	// Enabled - the master switch: off means no CAPTCHA on any form; the
	// keys and Forms are kept.
	Enabled   bool
	SiteKey   string
	SecretKey string
	// Forms - form name (CaptchaForm*) → shown.
	Forms map[string]bool
}

// CaptchaSettingsRepository stores the CAPTCHA settings row.
type CaptchaSettingsRepository interface {
	Get(ctx context.Context) (*CaptchaSettings, error)
	Update(ctx context.Context, s *CaptchaSettings) error
}
