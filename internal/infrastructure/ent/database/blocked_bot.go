package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockedbot"
)

// BlockedBotRepository implements entity.BlockedBotRepository.
type BlockedBotRepository struct {
	client *generated.Client
}

// NewBlockedBotRepository creates a new BlockedBotRepository.
func NewBlockedBotRepository(db *sql.DB) *BlockedBotRepository {
	return &BlockedBotRepository{client: clientFor(db)}
}

// List returns all blocked bot patterns, newest first.
func (r *BlockedBotRepository) List(ctx context.Context) ([]entity.BlockedBot, error) {
	rows, err := GetClient(ctx, r.client).BlockedBot.Query().Order(generated.Desc(blockedbot.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("BlockedBotRepository.List: %w", err)
	}
	var result []entity.BlockedBot
	for _, b := range rows {
		result = append(result, entity.BlockedBot{ID: b.ID, Pattern: b.Pattern, Comment: deref(b.Comment), CreatedAt: b.CreatedAt})
	}
	return result, nil
}

// Create inserts a new blocked bot pattern.
func (r *BlockedBotRepository) Create(ctx context.Context, pattern, comment string) (*entity.BlockedBot, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	comment = strings.TrimSpace(comment)
	createdAt := time.Now()
	row, err := GetClient(ctx, r.client).BlockedBot.Create().
		SetPattern(pattern).SetNillableComment(nilIfEmpty(comment)).SetCreatedAt(createdAt).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &entity.BlockedBot{ID: row.ID, Pattern: pattern, Comment: comment, CreatedAt: createdAt}, nil
}

// Delete removes a blocked bot by ID.
func (r *BlockedBotRepository) Delete(ctx context.Context, id int) error {
	_, err := GetClient(ctx, r.client).BlockedBot.Delete().Where(blockedbot.ID(id)).Exec(ctx)
	return err
}
