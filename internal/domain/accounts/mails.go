package accounts

import (
	"context"
	"fmt"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The account emails (entity.MailDef.Key).
const (
	MailInvite         = "account.invite"
	MailWelcomeAdmin   = "account.welcome_admin"
	MailEmailConfirm   = "account.email_confirm"
	MailWelcomePending = "account.welcome_pending"
	MailWelcomeOpen    = "account.welcome_open"
	MailActivation     = "account.activation"
	MailPasswordReset  = "account.password_reset"
	MailAdminNotify    = "account.admin_notify"
)

// accountMail - an account email and its text's fields in the account
// settings.
type accountMail struct {
	def   entity.MailDef
	field func(*entity.AccountSettings) (subject, body *string)
}

// accountMails - the account emails, their texts kept by the account
// settings (account_settings).
func accountMails() []accountMail {
	user := func(extra ...string) []string {
		return slices.Concat(entity.MailSitePlaceholders, entity.MailUserPlaceholders, extra)
	}
	toUser, toSite := i18n.N("to the user"), i18n.N("to the site's email")
	return []accountMail{
		{entity.MailDef{Key: MailWelcomeAdmin, Label: i18n.N("Welcome (new user created by an administrator)"),
			Description: i18n.N("An administrator created the account with a password."), Recipient: toUser,
			Optional: true, Placeholders: user("[user:one-time-login-url]"), Weight: -1},
			func(a *entity.AccountSettings) (*string, *string) { return &a.SubjWelcomeAdmin, &a.BodyWelcomeAdmin }},
		{entity.MailDef{Key: MailEmailConfirm, Label: i18n.N("Email confirmation on registration"),
			Description: i18n.N("A visitor signed up and must confirm their address."), Recipient: toUser,
			Placeholders: user("[user:confirm-email-url]"), Weight: 0},
			func(a *entity.AccountSettings) (*string, *string) { return &a.SubjEmailConfirm, &a.BodyEmailConfirm }},
		{entity.MailDef{Key: MailWelcomePending, Label: i18n.N("Welcome (awaiting approval)"),
			Description: i18n.N("A visitor signed up; an administrator must approve the account."), Recipient: toUser,
			Optional: true, Placeholders: user(), Weight: 1},
			func(a *entity.AccountSettings) (*string, *string) {
				return &a.SubjWelcomePending, &a.BodyWelcomePending
			}},
		{entity.MailDef{Key: MailWelcomeOpen, Label: i18n.N("Welcome (no approval required)"),
			Description: i18n.N("A visitor signed up and may sign in at once."), Recipient: toUser,
			Optional: true, Placeholders: user("[user:one-time-login-url]"), Weight: 2},
			func(a *entity.AccountSettings) (*string, *string) { return &a.SubjWelcomeOpen, &a.BodyWelcomeOpen }},
		{entity.MailDef{Key: MailActivation, Label: i18n.N("Account activation"),
			Description: i18n.N("An administrator approved the account."), Recipient: toUser,
			Optional: true, Placeholders: user("[user:one-time-login-url]"), Weight: 3},
			func(a *entity.AccountSettings) (*string, *string) { return &a.SubjActivation, &a.BodyActivation }},
		{entity.MailDef{Key: MailPasswordReset, Label: i18n.N("Password recovery"),
			Description: i18n.N("The user asked for a link to set a new password."), Recipient: toUser,
			Placeholders: user("[user:reset-password-url]"), Weight: 4},
			func(a *entity.AccountSettings) (*string, *string) { return &a.SubjPasswordReset, &a.BodyPasswordReset }},
		{entity.MailDef{Key: MailAdminNotify, Label: i18n.N("Admin notification (new registration request)"),
			Description: i18n.N("Someone signed up and awaits approval."), Recipient: toSite,
			Optional: true, Placeholders: user("[site:people-pending-url]"), Weight: 5},
			func(a *entity.AccountSettings) (*string, *string) { return &a.SubjAdminNotify, &a.BodyAdminNotify }},
	}
}

// invitePlaceholders - the invitation's own placeholders: its link, who
// invited and as whom.
var invitePlaceholders = []string{"[user:invite-url]", "[invite:by]", "[invite:role]"}

// inviteMail - the invitation (CreateAccount with Invite): its text the
// Mail page's (mail_templates), the default in the site's language.
func inviteMail() entity.MailDef {
	return entity.MailDef{
		Key:          MailInvite,
		Label:        i18n.N("Invitation"),
		Description:  i18n.N("An administrator invited the person: they set their own password by the link."),
		Recipient:    i18n.N("to the user"),
		Placeholders: slices.Concat(entity.MailSitePlaceholders, entity.MailUserPlaceholders, invitePlaceholders),
		Sample: map[string]string{
			"[user:invite-url]": "https://example.com/user/invite?token=example", "[invite:by]": "admin", "[invite:role]": "Editor",
		},
		Default: entity.MailTemplate{
			Subject: i18n.N("Invitation to [site:name]"),
			Body:    i18n.N("Hello!\n\n[invite:by] invites you to [site:name] as \"[invite:role]\".\n\nTo accept, set your password by the link (it works for 7 days):\n[user:invite-url]\n\nIf you didn't expect the invitation, ignore this email."),
		},
		Weight: -2,
	}
}

// RegisterMails adds the account emails to the site's
// (service.RegisterMail): the invitation, its text the Mail page's; the
// others', the account settings' (repo), blank ones the defaults in the
// site's language.
func RegisterMails(repo entity.AccountSettingsRepository) {
	service.RegisterMail(inviteMail())
	for _, m := range accountMails() {
		def, field := m.def, m.field
		def.Load = func(ctx context.Context) (entity.MailTemplate, error) {
			acc, err := repo.Get(ctx)
			if err != nil {
				return entity.MailTemplate{}, fmt.Errorf("loading the account settings: %w", err)
			}
			MergeAccountSettingsDefaults(acc)
			subject, body := field(acc)
			return entity.MailTemplate{Subject: *subject, Body: *body}, nil
		}
		def.Save = func(ctx context.Context, t entity.MailTemplate) error {
			acc, err := repo.Get(ctx)
			if err != nil {
				return fmt.Errorf("loading the account settings: %w", err)
			}
			subject, body := field(acc)
			*subject, *body = t.Subject, t.Body
			if err := repo.Update(ctx, acc); err != nil {
				return fmt.Errorf("saving the account settings: %w", err)
			}
			return nil
		}
		service.RegisterMail(def)
	}
}

// KeepMailTexts copies the account emails' texts of from into to: a save
// of the account settings (AccountPolicyService.Save), which has none of
// them (the Mail page has), keeps them.
func KeepMailTexts(to, from *entity.AccountSettings) {
	for _, m := range accountMails() {
		ts, tb := m.field(to)
		fs, fb := m.field(from)
		*ts, *tb = *fs, *fb
	}
}
