package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/maillog"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// MailLogRepository implements entity.MailLogRepository.
type MailLogRepository struct {
	client *generated.Client
}

// NewMailLogRepository creates a new MailLogRepository.
func NewMailLogRepository(db *sql.DB) *MailLogRepository {
	return &MailLogRepository{client: clientFor(db)}
}

// Add records an email (its address, subject and error cut to the
// columns).
func (r *MailLogRepository) Add(ctx context.Context, e entity.MailLogEntry) error {
	err := GetClient(ctx, r.client).MailLog.Create().
		SetSentAt(e.SentAt).SetMailKey(textcut.Runes(e.Key, 128)).SetRecipient(textcut.Runes(e.To, 255)).
		SetSubject(textcut.Runes(e.Subject, 512)).SetStatus(e.Status).SetError(textcut.Runes(e.Error, 1024)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("recording an email: %w", err)
	}
	return nil
}

// List - the entries f asks for, newest first, and how many there are.
func (r *MailLogRepository) List(ctx context.Context, f entity.MailLogFilter) ([]entity.MailLogEntry, int, error) {
	var where []predicate.MailLog
	if f.To != "" {
		where = append(where, maillog.RecipientContainsFold(f.To))
	}
	if f.Key != "" {
		where = append(where, maillog.MailKey(f.Key))
	}
	if f.Status != "" {
		where = append(where, maillog.Status(f.Status))
	}
	q := GetClient(ctx, r.client).MailLog.Query().Where(where...)
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting the mail log: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}
	rows, err := q.Order(generated.Desc(maillog.FieldSentAt), generated.Desc(maillog.FieldID)).
		Limit(f.Limit).Offset(f.Offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing the mail log: %w", err)
	}
	out := make([]entity.MailLogEntry, len(rows))
	for i, m := range rows {
		out[i] = entity.MailLogEntry{ID: m.ID, SentAt: m.SentAt, Key: m.MailKey, To: m.Recipient, Subject: m.Subject, Status: m.Status, Error: m.Error}
	}
	return out, total, nil
}

// LastSent - when each email (by key) was last sent.
func (r *MailLogRepository) LastSent(ctx context.Context) (map[string]time.Time, error) {
	var rows []struct {
		MailKey string    `json:"mail_key"`
		Max     time.Time `json:"max"`
	}
	err := GetClient(ctx, r.client).MailLog.Query().
		Where(maillog.Status(entity.MailStatusSent), maillog.MailKeyNEQ("")).
		GroupBy(maillog.FieldMailKey).
		Aggregate(generated.Max(maillog.FieldSentAt)).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("the emails last sent: %w", err)
	}
	out := make(map[string]time.Time, len(rows))
	for _, row := range rows {
		out[row.MailKey] = row.Max
	}
	return out, nil
}

// DeleteBefore removes the entries older than t.
func (r *MailLogRepository) DeleteBefore(ctx context.Context, t time.Time) error {
	if _, err := GetClient(ctx, r.client).MailLog.Delete().Where(maillog.SentAtLT(t)).Exec(ctx); err != nil {
		return fmt.Errorf("deleting the old mail log: %w", err)
	}
	return nil
}
