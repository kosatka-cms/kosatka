package privatemessages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
)

// mailTimeout - how long the background e-mails of one message may take.
const mailTimeout = 2 * time.Minute

// Mailer - the e-mails of private messages: the members of a thread
// told of a new message in it (the first one or a reply), but its author
// and who deleted the thread.
type Mailer struct {
	threads Service
	users   modulekit.UserRepository
	site    modulekit.SiteInformationRepository
	mailOff func(ctx context.Context, userIDs []int) (map[int]bool, error)
	mail    mailtemplate.Mailer
	logger  *slog.Logger
}

// MailerDeps - what NewMailer needs.
type MailerDeps struct {
	Threads Service
	Users   modulekit.UserRepository
	Site    modulekit.SiteInformationRepository
	// MailOff - those of the users who turned the e-mail off
	// (SettingsRepo.MailOff).
	MailOff func(ctx context.Context, userIDs []int) (map[int]bool, error)
	Mail    mailtemplate.Mailer
	Logger  *slog.Logger
}

// NewMailer - the e-mails of new messages, with the threads, users, site
// and mail sender of d.
func NewMailer(d MailerDeps) *Mailer {
	return &Mailer{threads: d.Threads, users: d.Users, site: d.Site, mailOff: d.MailOff, mail: d.Mail, logger: d.Logger}
}

// NotifyLater - Notify in the background, so the request doesn't wait
// for the mail; a failure is logged.
func (m *Mailer) NotifyLater(threadID, authorID int, baseURL string) {
	go func() {
		defer panicsafe.Recover(m.logger, "private_messages.mail")
		ctx, cancel := context.WithTimeout(context.Background(), mailTimeout)
		defer cancel()
		if err := m.Notify(ctx, threadID, authorID, baseURL); err != nil && m.logger != nil {
			m.logger.Error("private message e-mails not sent", "thread_id", threadID, "error", err)
		}
	}()
}

// Notify mails the thread's members of authorID's new message: each one
// but the author, who deleted the thread, who turned the e-mail off
// (their profile) and an account not active (blocked, pending), once. baseURL - the site's address for the thread's
// link ("" - the site URL isn't set: none sent). A recipient that can't be
// loaded or mailed is logged and skipped; a failure to load the thread,
// the site or the members' settings is returned.
func (m *Mailer) Notify(ctx context.Context, threadID, authorID int, baseURL string) error {
	if baseURL == "" {
		modulekit.LogFallback(m.logger, errors.New("the site URL isn't set"), "private message e-mails not sent", "thread_id", threadID)
		return nil
	}
	site, err := m.site.Get(ctx)
	if err != nil {
		return fmt.Errorf("loading the site: %w", err)
	}
	thread, err := m.threads.ThreadByID(ctx, threadID)
	if err != nil {
		return fmt.Errorf("loading the thread: %w", err)
	}
	senderName := ""
	if sender, err := m.users.ByID(ctx, authorID); err == nil && sender != nil {
		senderName = sender.Username
	} else {
		modulekit.LogFallback(m.logger, err, "loading the sender of a private message (mailed without the name)", "user_id", authorID)
	}
	threadURL := strings.TrimSuffix(baseURL, "/") + "/me/" + manifest.TabPath + "/thread/" + strconv.Itoa(threadID)
	memberIDs := make([]int, 0, len(thread.Members))
	for _, member := range thread.Members {
		memberIDs = append(memberIDs, member.UserID)
	}
	off, err := m.mailOff(ctx, memberIDs)
	if err != nil {
		return fmt.Errorf("loading the members' e-mail settings: %w", err)
	}
	mailed := map[int]bool{authorID: true}
	for _, member := range thread.Members {
		if member.Deleted || mailed[member.UserID] || off[member.UserID] {
			continue
		}
		mailed[member.UserID] = true
		recipient, err := m.users.ByID(ctx, member.UserID)
		if err != nil || recipient == nil {
			modulekit.LogFallback(m.logger, err, "loading the recipient of a private message e-mail (skipped)", "user_id", member.UserID)
			continue
		}
		// A blocked or pending account gets no mail.
		if recipient.Status != modulekit.UserStatusActive {
			continue
		}
		md := modulekit.UserMailTokens(baseURL, site, recipient)
		md.Extra = map[string]string{"[pm:sender]": senderName, "[pm:subject]": thread.Subject, "[pm:thread-url]": threadURL}
		modulekit.LogFallback(m.logger, m.mail.Send(ctx, MailNewMessage, recipient.Email, md),
			"sending a private message e-mail", "user_id", recipient.ID, "thread_id", threadID)
	}
	return nil
}
