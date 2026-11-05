package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// CommentSettingsRepository implements entity.CommentSettingsRepository.
// Read on every comment form and post (several times a post), the row is
// kept after the first read until the repository writes it (afterWrite).
type CommentSettingsRepository struct {
	client *generated.Client
	cached atomic.Pointer[entity.CommentSettings]
}

var _ entity.CommentSettingsRepository = (*CommentSettingsRepository)(nil)

// NewCommentSettingsRepository creates a CommentSettingsRepository.
func NewCommentSettingsRepository(db *sql.DB) *CommentSettingsRepository {
	return &CommentSettingsRepository{client: clientFor(db)}
}

// Get - see entity.CommentSettingsRepository.Get; outside a transaction,
// the settings kept since the last write.
func (r *CommentSettingsRepository) Get(ctx context.Context) (entity.CommentSettings, error) {
	_, inTx := ctx.Value(txKey{}).(*generated.Client)
	if c := r.cached.Load(); c != nil && !inTx {
		return *c, nil
	}
	c, err := GetClient(ctx, r.client).CommentSettings.Get(ctx, settingsRowID)
	settings := entity.DefaultCommentSettings()
	switch {
	case generated.IsNotFound(err):
	case err != nil:
		return entity.CommentSettings{}, fmt.Errorf("loading the comment settings: %w", err)
	default:
		settings = entity.CommentSettings{NewAuthorChecks: c.NewAuthorChecks, LinksLimit: c.LinksLimit, StopWords: c.StopWords,
			StopWordsAction: c.StopWordsAction, DuplicateHours: c.DuplicateHours, SpamKeepDays: c.SpamKeepDays,
			BotTrap: c.BotTrap, BotMinSeconds: c.BotMinSeconds, LoadWithPage: c.LoadWithPage}
	}
	if !inTx {
		r.cached.Store(&settings)
	}
	return settings, nil
}

// Update saves the settings row, creating it if it isn't there.
func (r *CommentSettingsRepository) Update(ctx context.Context, s entity.CommentSettings) error {
	defer afterWrite(ctx, func() { r.cached.Store(nil) })
	client := GetClient(ctx, r.client)
	err := client.CommentSettings.UpdateOneID(settingsRowID).SetNewAuthorChecks(s.NewAuthorChecks).SetLinksLimit(s.LinksLimit).
		SetStopWords(s.StopWords).SetStopWordsAction(s.StopWordsAction).SetDuplicateHours(s.DuplicateHours).SetSpamKeepDays(s.SpamKeepDays).
		SetBotTrap(s.BotTrap).SetBotMinSeconds(s.BotMinSeconds).SetLoadWithPage(s.LoadWithPage).Exec(ctx)
	if generated.IsNotFound(err) {
		err = client.CommentSettings.Create().SetID(settingsRowID).SetNewAuthorChecks(s.NewAuthorChecks).SetLinksLimit(s.LinksLimit).
			SetStopWords(s.StopWords).SetStopWordsAction(s.StopWordsAction).SetDuplicateHours(s.DuplicateHours).SetSpamKeepDays(s.SpamKeepDays).
			SetBotTrap(s.BotTrap).SetBotMinSeconds(s.BotMinSeconds).SetLoadWithPage(s.LoadWithPage).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the comment settings: %w", err)
	}
	return nil
}
