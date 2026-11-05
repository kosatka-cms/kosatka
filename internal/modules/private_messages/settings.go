package privatemessages

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated/pmusersettings"
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// MailTemplate - the new message e-mail's subject and body: the core's
// placeholders ([site:…], [user:…]) and the module's (MailPlaceholders).
type MailTemplate = modulekit.MailTemplate

// MailNewMessage - the new message e-mail (modulekit.AddMail).
const MailNewMessage = manifest.Name + ".new_message"

// MailPlaceholders - the module's own placeholders of the new message
// e-mail: the sender's name, the thread's subject and address.
var MailPlaceholders = []string{"[pm:sender]", "[pm:subject]", "[pm:thread-url]"}

// defaultMailTemplate - the e-mail's built-in text in the site's language.
func defaultMailTemplate() MailTemplate {
	lang := i18n.Default()
	tr := func(s string) string { return i18n.Current().Text(lang, "", s) }
	return MailTemplate{
		Subject: tr(i18n.N("New private message from [pm:sender] - [site:name]")),
		Body:    tr(i18n.N("Hello, [user:name]!\n\n[pm:sender] has sent you a private message at [site:name].\n\nSubject: [pm:subject]\n\nTo read the message and reply, follow the link:\n[pm:thread-url]\n\nIf you don't want to receive these notifications, turn them off in your profile:\n[user:edit-url]\n\n-- [site:name] team")),
	}
}

// SettingsRepo - the module's settings (pm_settings, one row: id 1).
type SettingsRepo struct{ db *sql.DB }

// NewSettingsRepo - the module's settings on db.
func NewSettingsRepo(db *sql.DB) *SettingsRepo { return &SettingsRepo{db: db} }

func (r *SettingsRepo) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// MailTemplate - the new message e-mail's template: a blank subject or
// body (never saved, or cleared) is the default's.
func (r *SettingsRepo) MailTemplate(ctx context.Context) (MailTemplate, error) {
	var t MailTemplate
	row, err := r.client(ctx).PmSettings.Get(ctx, 1)
	switch {
	case err == nil:
		t = MailTemplate{Subject: row.NewMessageSubject, Body: row.NewMessageBody}
	case !generated.IsNotFound(err):
		return MailTemplate{}, fmt.Errorf("loading the private messages settings: %w", err)
	}
	d := defaultMailTemplate()
	if strings.TrimSpace(t.Subject) == "" {
		t.Subject = d.Subject
	}
	if strings.TrimSpace(t.Body) == "" {
		t.Body = d.Body
	}
	return t, nil
}

// SaveMailTemplate stores the e-mail's template (the row made on the
// first save).
func (r *SettingsRepo) SaveMailTemplate(ctx context.Context, t MailTemplate) error {
	c := r.client(ctx)
	n, err := c.PmSettings.Update().SetNewMessageSubject(t.Subject).SetNewMessageBody(t.Body).Save(ctx)
	if err != nil {
		return fmt.Errorf("saving the private messages settings: %w", err)
	}
	if n > 0 {
		return nil
	}
	if err := c.PmSettings.Create().SetID(1).SetNewMessageSubject(t.Subject).SetNewMessageBody(t.Body).Exec(ctx); err != nil {
		return fmt.Errorf("saving the private messages settings: %w", err)
	}
	return nil
}

// MailDef - the new message e-mail on the Mail page: its text kept in
// the module's settings (MailTemplate, SaveMailTemplate).
func (r *SettingsRepo) MailDef() modulekit.MailDef {
	return modulekit.MailDef{
		Key:          MailNewMessage,
		Module:       manifest.Name,
		Label:        i18n.N("New private message notification"),
		Description:  i18n.N("A member of a conversation wrote a new message; a user may turn it off in their profile."),
		Recipient:    i18n.N("to the conversation's members"),
		Optional:     true,
		Placeholders: slices.Concat(modulekit.MailSitePlaceholders, modulekit.MailUserPlaceholders, MailPlaceholders),
		Sample:       map[string]string{"[pm:sender]": "anna", "[pm:subject]": "Hello", "[pm:thread-url]": "https://example.com/me/" + manifest.TabPath + "/thread/1"},
		Load:         r.MailTemplate,
		Save:         r.SaveMailTemplate,
	}
}

// MailOff - those of userIDs who turned the new message e-mail off.
func (r *SettingsRepo) MailOff(ctx context.Context, userIDs []int) (map[int]bool, error) {
	off := map[int]bool{}
	if len(userIDs) == 0 {
		return off, nil
	}
	ids, err := r.client(ctx).PmUserSettings.Query().
		Where(pmusersettings.UserIDIn(userIDs...), pmusersettings.MailNewMessage(false)).
		Select(pmusersettings.FieldUserID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("the users' e-mail settings: %w", err)
	}
	for _, id := range ids {
		off[id] = true
	}
	return off, nil
}

// SetMailNewMessage stores whether the user gets the new message e-mail
// (the row made on the first save).
func (r *SettingsRepo) SetMailNewMessage(ctx context.Context, userID int, on bool) error {
	c := r.client(ctx)
	n, err := c.PmUserSettings.Update().Where(pmusersettings.UserID(userID)).SetMailNewMessage(on).Save(ctx)
	if err == nil && n == 0 {
		err = c.PmUserSettings.Create().SetUserID(userID).SetMailNewMessage(on).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the e-mail setting of user %d: %w", userID, err)
	}
	return nil
}

// userSettingsForm - the module's section of the profile form.
type userSettingsForm struct {
	MailNewMessage bool
}

// UserSettingsSection - the user's new message e-mail switch on their
// profile form (template "user_settings_private_messages"). The form
// marks the section shown (pm_user_settings): a form without it (a
// theme's own) leaves the setting as it is.
func (r *SettingsRepo) UserSettingsSection() modulekit.UserSettingsSection {
	return modulekit.UserSettingsSection{
		Module: manifest.Name,
		Load: func(ctx context.Context, u modulekit.User) (any, error) {
			off, err := r.MailOff(ctx, []int{u.ID})
			return userSettingsForm{MailNewMessage: !off[u.ID]}, err
		},
		Save: func(ctx context.Context, u modulekit.User, req *http.Request) error {
			if req.FormValue("pm_user_settings") != "1" {
				return nil
			}
			return r.SetMailNewMessage(ctx, u.ID, req.FormValue("pm_mail_new_message") == "1")
		},
	}
}
