package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// DigestHour - the hour of the site's day the daily digests go out at.
const DigestHour = 8

// keepNotifications - how long a notification is kept.
const keepNotifications = 90 * 24 * time.Hour

// MailTemplate - the notifications e-mail's subject and body: the core's
// placeholders ([site:…], [user:…]) and the module's (MailPlaceholders).
type MailTemplate = modulekit.MailTemplate

// MailNotifications - the notifications e-mail (modulekit.AddMail).
const MailNotifications = manifest.Name + ".notifications"

// MailPlaceholders - the module's placeholders of the e-mail: its subject
// (what it tells of, or how many things), the list of what happened and
// the Notifications tab's address.
var MailPlaceholders = []string{"[subscriptions:subject]", "[subscriptions:list]", "[subscriptions:url]"}

// tr - source in the site's language, as plain text (the e-mails).
func tr(source string, args ...any) string {
	return i18n.Current().Text(i18n.Default(), "", source, args...)
}

// defaultMailTemplate - the e-mail's built-in text in the site's language.
func defaultMailTemplate() MailTemplate {
	return MailTemplate{
		Subject: tr(i18n.N("[subscriptions:subject] - [site:name]")),
		Body:    tr(i18n.N("Hello, [user:name]!\n\n[subscriptions:list]\n\nAll your notifications: [subscriptions:url]\nTurn the e-mails off or change how often they come in your profile: [user:edit-url]\n\n-- [site:name] team")),
	}
}

// MailTemplate - the e-mail's template: a blank subject or body (never
// saved, or cleared) is the default's.
func (r *Repo) MailTemplate(ctx context.Context) (MailTemplate, error) {
	var t MailTemplate
	row, err := r.settingsRow(ctx)
	if err != nil {
		return t, err
	}
	if row != nil {
		t = MailTemplate{Subject: row.MailSubject, Body: row.MailBody}
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

// MailDef - the notifications e-mail on the Mail page: its text kept in
// the module's settings (MailTemplate, SaveMailTemplate).
func (r *Repo) MailDef() modulekit.MailDef {
	return modulekit.MailDef{
		Key:          MailNotifications,
		Module:       manifest.Name,
		Label:        i18n.N("Notifications"),
		Description:  i18n.N("What happened in a user's subscriptions, at once or in a daily digest, as they chose in their profile."),
		Recipient:    i18n.N("to the subscriber"),
		Placeholders: slices.Concat(modulekit.MailSitePlaceholders, modulekit.MailUserPlaceholders, MailPlaceholders),
		Sample: map[string]string{
			"[subscriptions:subject]": "New comment: How themes work",
			"[subscriptions:list]":    "anna commented on How themes work\nhttps://example.com/articles/how-themes-work",
			"[subscriptions:url]":     "https://example.com/me/" + manifest.TabPath,
		},
		Load: r.MailTemplate,
		Save: r.SaveMailTemplate,
	}
}

// SaveMailTemplate stores the e-mail's template.
func (r *Repo) SaveMailTemplate(ctx context.Context, t MailTemplate) error {
	return r.updateSettings(ctx, func(m *generated.SubscriptionSettingsMutation) {
		m.SetMailSubject(t.Subject)
		m.SetMailBody(t.Body)
	})
}

// Mailer - the notifications' e-mails: at once (a minute's events in one
// e-mail), the daily digest, and the cleanup of old notifications.
type Mailer struct {
	repo     *Repo
	describe describeDeps
	users    interface {
		ByID(ctx context.Context, id int) (*modulekit.User, error)
	}
	site    modulekit.SiteInformationRepository
	mail    mailtemplate.Mailer
	baseURL func() string
	logger  *slog.Logger
	now     func() time.Time
	visitor func(ctx context.Context, user *modulekit.User) (modulekit.ContentVisitor, error)
}

// MailerDeps - what NewMailer needs.
type MailerDeps struct {
	Repo     *Repo
	Describe describeDeps
	Users    interface {
		ByID(ctx context.Context, id int) (*modulekit.User, error)
	}
	Site    modulekit.SiteInformationRepository
	Mail    mailtemplate.Mailer
	BaseURL func() string
	Logger  *slog.Logger
	// Visitor - what of the content a user may not view
	// (AuthService.ContentVisitor): no e-mail tells them of such content.
	// Nil - nothing.
	Visitor func(ctx context.Context, user *modulekit.User) (modulekit.ContentVisitor, error)
}

// NewMailer - the notifications' e-mails with the repository, users,
// site, mail sender and visitor of d.
func NewMailer(d MailerDeps) *Mailer {
	return &Mailer{repo: d.Repo, describe: d.Describe, users: d.Users, site: d.Site, mail: d.Mail, baseURL: d.BaseURL, logger: d.Logger, now: time.Now, visitor: d.Visitor}
}

// Jobs - the module's background jobs.
func (m *Mailer) Jobs() []modulekit.Job {
	return []modulekit.Job{
		{Name: manifest.Name + "_mail", Every: time.Minute, Run: m.SendNow},
		{Name: manifest.Name + "_digest", Every: 15 * time.Minute, Run: m.SendDigests},
		{Name: manifest.Name + "_cleanup", Every: 24 * time.Hour, Run: m.Cleanup},
	}
}

// SendNow mails the waiting notifications of the users mailed at once,
// one e-mail each; a user who turned the e-mails off has theirs dropped,
// one with the daily digest waits for it.
func (m *Mailer) SendNow(ctx context.Context) error {
	return m.send(ctx, MailNow)
}

// SendDigests mails the daily digests: once a day, from DigestHour of the
// site's day.
func (m *Mailer) SendDigests(ctx context.Context) error {
	now := sitetime.In(m.now())
	if now.Hour() < DigestHour {
		return nil
	}
	today := now.Format("2006-01-02")
	row, err := m.repo.settingsRow(ctx)
	if err != nil {
		return err
	}
	if row != nil && row.DigestDay == today {
		return nil
	}
	if err := m.send(ctx, MailDaily); err != nil {
		return err
	}
	return m.repo.updateSettings(ctx, func(s *generated.SubscriptionSettingsMutation) { s.SetDigestDay(today) })
}

// Cleanup deletes the notifications older than keepNotifications.
func (m *Mailer) Cleanup(ctx context.Context) error {
	_, err := m.repo.DeleteOlder(ctx, m.now().Add(-keepNotifications))
	return err
}

// send mails the waiting notifications of the users of mode; a recipient
// who can't be mailed is logged and skipped (theirs stay waiting only
// when the site's address isn't set).
func (m *Mailer) send(ctx context.Context, mode string) error {
	waiting, err := m.repo.WaitingMail(ctx)
	if err != nil || len(waiting) == 0 {
		return err
	}
	base := strings.TrimSuffix(m.baseURL(), "/")
	if base == "" {
		modulekit.LogFallback(m.logger, errors.New("the site URL isn't set"), "notification e-mails not sent")
		return nil
	}
	ids := make([]int, 0, len(waiting))
	for id := range waiting {
		ids = append(ids, id)
	}
	settings, err := m.repo.UserSettings(ctx, ids)
	if err != nil {
		return err
	}
	site, err := m.site.Get(ctx)
	if err != nil {
		return fmt.Errorf("loading the site: %w", err)
	}
	for userID, ns := range waiting {
		nIDs := make([]int, len(ns))
		for i, n := range ns {
			nIDs[i] = n.ID
		}
		switch settings[userID].Mail {
		case MailNever:
			if err := m.repo.setMailState(ctx, nIDs, mailSkipped); err != nil {
				return err
			}
			continue
		case mode:
		default:
			continue
		}
		state := mailSent
		if err := m.mailUser(ctx, base, site, userID, ns); err != nil {
			modulekit.LogFallback(m.logger, err, "sending a notifications e-mail (dropped)", "user_id", userID)
			state = mailSkipped
		}
		if err := m.repo.setMailState(ctx, nIDs, state); err != nil {
			return err
		}
	}
	return nil
}

// mailUser mails userID their notifications ns: an active account only.
func (m *Mailer) mailUser(ctx context.Context, base string, site *modulekit.SiteInformation, userID int, ns []Notification) error {
	user, err := m.users.ByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("loading user %d: %w", userID, err)
	}
	if user.Status != modulekit.UserStatusActive || user.Email == "" {
		return nil
	}
	// As the user sees the site: the content they may not view left out.
	if m.visitor != nil {
		v, err := m.visitor(ctx, user)
		if err != nil {
			return fmt.Errorf("what of the content user %d may not view: %w", userID, err)
		}
		ctx = modulekit.WithContentVisitor(ctx, v)
	}
	items, err := describe(ctx, m.describe, ns, false)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	tokens, err := m.unsubscribeTokens(ctx, userID)
	if err != nil {
		return err
	}
	var list []string
	for _, it := range items {
		list = append(list, mailItem(base, it, tokens))
	}
	// In the site's language; a count only: nothing in it for HTML to
	// escape.
	siteCtx, count := i18n.WithLang(ctx, i18n.Default()), len(items)
	subject := i18n.Plural(siteCtx, count, "@count new notification", "@count new notifications")
	if len(items) == 1 {
		subject = mailSubject(items[0])
	}
	md := modulekit.UserMailTokens(base, site, user)
	md.Extra = map[string]string{
		"[subscriptions:subject]": subject,
		"[subscriptions:list]":    strings.Join(list, "\n\n"),
		"[subscriptions:url]":     base + "/me/" + manifest.TabPath,
	}
	return m.mail.Send(ctx, MailNotifications, user.Email, md)
}

// unsubscribeTokens - the unsubscribe links' tokens of userID's
// subscriptions, by content and by term.
func (m *Mailer) unsubscribeTokens(ctx context.Context, userID int) (map[Target]string, error) {
	subs, err := m.repo.UserSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[Target]string, len(subs))
	for _, s := range subs {
		out[s.Target] = s.Token
	}
	return out, nil
}

// mailSubject - the subject of an e-mail of one item.
func mailSubject(it Item) string {
	switch it.Kind {
	case KindReply:
		return tr(i18n.N("A reply to your comment - @title"), "@title", it.NodeTitle)
	case KindComment:
		return tr(i18n.N("A new comment - @title"), "@title", it.NodeTitle)
	}
	return tr(i18n.N("New: @title"), "@title", it.NodeTitle)
}

// mailItem - an item in the e-mail's list: what happened, the quote, the
// link (through the notification: it gets read), the unsubscribe link of
// the subscription it came from.
func mailItem(base string, it Item, tokens map[Target]string) string {
	var b strings.Builder
	switch it.Kind {
	case KindReply:
		b.WriteString(tr(i18n.N("@user replied to your comment on \"@title\":"), "@user", it.Actor, "@title", it.NodeTitle))
	case KindComment:
		b.WriteString(tr(i18n.N("@user commented on \"@title\":"), "@user", it.Actor, "@title", it.NodeTitle))
	default:
		b.WriteString(tr(i18n.N("New content with \"@term\": \"@title\""), "@term", it.TermName, "@title", it.NodeTitle))
	}
	if it.Excerpt != "" {
		b.WriteString("\n\"" + it.Excerpt + "\"")
	}
	b.WriteString("\n" + base + "/notifications/" + strconv.Itoa(it.ID))
	switch it.Kind {
	case KindComment:
		if t := tokens[Target{NodeID: it.NodeID}]; t != "" {
			b.WriteString("\n" + tr(i18n.N("Unsubscribe from \"@title\": @url"), "@title", it.NodeTitle, "@url", base+"/subscriptions/unsubscribe/"+t))
		}
	case KindContent:
		if t := tokens[Target{TermID: it.TermID}]; t != "" {
			b.WriteString("\n" + tr(i18n.N("Stop following \"@term\": @url"), "@term", it.TermName, "@url", base+"/subscriptions/unsubscribe/"+t))
		}
	}
	return b.String()
}
