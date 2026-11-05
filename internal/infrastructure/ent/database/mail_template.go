package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/mailtemplate"
)

// MailTemplateRepository implements entity.MailTemplateRepository.
type MailTemplateRepository struct {
	client *generated.Client
}

// NewMailTemplateRepository creates a new MailTemplateRepository.
func NewMailTemplateRepository(db *sql.DB) *MailTemplateRepository {
	return &MailTemplateRepository{client: clientFor(db)}
}

// All - the emails changed, by key.
func (r *MailTemplateRepository) All(ctx context.Context) (map[string]entity.StoredMail, error) {
	rows, err := GetClient(ctx, r.client).MailTemplate.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the emails' templates: %w", err)
	}
	out := make(map[string]entity.StoredMail, len(rows))
	for _, m := range rows {
		out[m.MailKey] = entity.StoredMail{Enabled: m.Enabled, MailTemplate: entity.MailTemplate{Subject: m.Subject, Body: m.Body}}
	}
	return out, nil
}

// SaveText stores an email's text (blank - the default's).
func (r *MailTemplateRepository) SaveText(ctx context.Context, key string, t entity.MailTemplate) error {
	return r.upsert(ctx, key, func(u *generated.MailTemplateUpdate) *generated.MailTemplateUpdate {
		return u.SetSubject(t.Subject).SetBody(t.Body)
	}, func(c *generated.MailTemplateCreate) *generated.MailTemplateCreate {
		return c.SetSubject(t.Subject).SetBody(t.Body)
	})
}

// SetEnabled switches an email on or off.
func (r *MailTemplateRepository) SetEnabled(ctx context.Context, key string, on bool) error {
	return r.upsert(ctx, key, func(u *generated.MailTemplateUpdate) *generated.MailTemplateUpdate {
		return u.SetEnabled(on)
	}, func(c *generated.MailTemplateCreate) *generated.MailTemplateCreate {
		return c.SetEnabled(on).SetBody("")
	})
}

// upsert changes key's row, made on its first change (a concurrent first
// change - the update again).
func (r *MailTemplateRepository) upsert(ctx context.Context, key string,
	update func(*generated.MailTemplateUpdate) *generated.MailTemplateUpdate,
	create func(*generated.MailTemplateCreate) *generated.MailTemplateCreate,
) error {
	client := GetClient(ctx, r.client)
	for attempt := 0; ; attempt++ {
		n, err := update(client.MailTemplate.Update().Where(mailtemplate.MailKey(key))).Save(ctx)
		if err != nil {
			return fmt.Errorf("saving the email %q: %w", key, err)
		}
		if n > 0 {
			return nil
		}
		err = create(client.MailTemplate.Create().SetMailKey(key)).Exec(ctx)
		if err == nil {
			return nil
		}
		if !generated.IsConstraintError(err) || attempt > 0 {
			return fmt.Errorf("saving the email %q: %w", key, err)
		}
	}
}
