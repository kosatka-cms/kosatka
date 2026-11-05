package accounts

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// MergeAccountSettingsDefaults fills empty template fields with the built-in defaults.
func MergeAccountSettingsDefaults(a *entity.AccountSettings) {
	if a == nil {
		return
	}
	d := DefaultAccountSettings()
	if a.SubjWelcomeAdmin == "" {
		a.SubjWelcomeAdmin = d.SubjWelcomeAdmin
	}
	if stringsOrBlank(a.BodyWelcomeAdmin) {
		a.BodyWelcomeAdmin = d.BodyWelcomeAdmin
	}
	if a.SubjWelcomePending == "" {
		a.SubjWelcomePending = d.SubjWelcomePending
	}
	if stringsOrBlank(a.BodyWelcomePending) {
		a.BodyWelcomePending = d.BodyWelcomePending
	}
	if a.SubjWelcomeOpen == "" {
		a.SubjWelcomeOpen = d.SubjWelcomeOpen
	}
	if stringsOrBlank(a.BodyWelcomeOpen) {
		a.BodyWelcomeOpen = d.BodyWelcomeOpen
	}
	if a.SubjActivation == "" {
		a.SubjActivation = d.SubjActivation
	}
	if stringsOrBlank(a.BodyActivation) {
		a.BodyActivation = d.BodyActivation
	}
	if a.SubjPasswordReset == "" {
		a.SubjPasswordReset = d.SubjPasswordReset
	}
	if stringsOrBlank(a.BodyPasswordReset) {
		a.BodyPasswordReset = d.BodyPasswordReset
	}
	if a.SubjEmailConfirm == "" {
		a.SubjEmailConfirm = d.SubjEmailConfirm
	}
	if stringsOrBlank(a.BodyEmailConfirm) {
		a.BodyEmailConfirm = d.BodyEmailConfirm
	}
	if a.SubjAdminNotify == "" {
		a.SubjAdminNotify = d.SubjAdminNotify
	}
	if stringsOrBlank(a.BodyAdminNotify) {
		a.BodyAdminNotify = d.BodyAdminNotify
	}
	if a.RegistrationMode == "" {
		a.RegistrationMode = entity.RegistrationModeVisitors
	}
}

func stringsOrBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\n' && r != '\t' && r != '\r' {
			return false
		}
	}
	return true
}

// DefaultAccountSettings - the factory defaults; the e-mail texts in the
// site's language.
func DefaultAccountSettings() *entity.AccountSettings {
	lang := i18n.Default()
	tr := func(s string) string { return i18n.Current().Text(lang, "", s) }
	return &entity.AccountSettings{
		RegistrationMode:         entity.RegistrationModeVisitors,
		RequireEmailVerification: false,
		SubjWelcomeAdmin:         tr(i18n.N("An administrator created an account for you at [site:name]")),
		BodyWelcomeAdmin:         tr(i18n.N("[user:name],\n\nA site administrator at [site:name] has created an account for you.\n\nYou may now sign in by clicking this link or copying and pasting it into your browser:\n\n[user:one-time-login-url]\n\nThis link can only be used once. After signing in you will be redirected to [user:edit-url], where you can set or change your password.\n\nYou can also sign in at [site:login-url] with the username [user:name] and the password the administrator set for your account.\n\n-- [site:name] team")),
		SubjWelcomePending:       tr(i18n.N("Account details for [user:name] at [site:name] (pending admin approval)")),
		BodyWelcomePending:       tr(i18n.N("[user:name],\n\nThank you for registering at [site:name]. Your application for an account is currently pending approval.\n\nOnce it has been approved, you will receive another email with information about how to sign in.\n\n-- [site:name] team")),
		SubjWelcomeOpen:          tr(i18n.N("Welcome to [site:name]")),
		BodyWelcomeOpen:          tr(i18n.N("[user:name],\n\nThank you for registering at [site:name].\n\nYou may now sign in by clicking this link or copying and pasting it into your browser:\n\n[user:one-time-login-url]\n\nThis link can only be used once. After signing in you will be redirected to [user:edit-url], where you can change your password.\n\nYou can also sign in at [site:login-url] with the username [user:name] and the password you chose at registration.\n\n-- [site:name] team")),
		SubjActivation:           tr(i18n.N("Account details for [user:name] at [site:name] (approved)")),
		BodyActivation:           tr(i18n.N("[user:name],\n\nYour account at [site:name] has been activated.\n\nYou may now sign in by clicking this link or copying and pasting it into your browser:\n\n[user:one-time-login-url]\n\nThis link can only be used once.\n\nAfter signing in you will be redirected to [user:edit-url], where you can change your password.\n\nOnce you have set your password, you can sign in at [site:login-url] with:\n\nUsername: [user:name]")),
		SubjPasswordReset:        tr(i18n.N("Password reset for [user:name] at [site:name]")),
		BodyPasswordReset:        tr(i18n.N("[user:name],\n\nA request to reset the password for your account at [site:name] has been made.\n\nFollow the link to set a new password (valid for one day):\n\n[user:reset-password-url]\n\nOnce you have set your password, you can sign in at [site:login-url].")),
		SubjEmailConfirm:         tr(i18n.N("Confirm your registration at [site:name]")),
		BodyEmailConfirm:         tr(i18n.N("Hello, [user:name]!\n\nTo complete your registration, follow the link:\n[user:confirm-email-url]\n\nThe link is valid for 24 hours. If you did not register at [site:name], ignore this email.")),
		SubjAdminNotify:          tr(i18n.N("New registration awaiting approval: [user:name] - [site:name]")),
		BodyAdminNotify:          tr(i18n.N("A new user has registered at [site:name] and is awaiting approval.\n\nName: [user:name]\nEmail: [user:email]\n\nGo to the list of users:\n[site:people-pending-url]")),
	}
}
