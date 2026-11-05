package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockedip"
)

// BlockedIPRepository implements entity.BlockedIPRepository.
type BlockedIPRepository struct {
	client *generated.Client
}

// NewBlockedIPRepository creates a new BlockedIPRepository.
func NewBlockedIPRepository(db *sql.DB) *BlockedIPRepository {
	return &BlockedIPRepository{client: clientFor(db)}
}

// List returns all blocked IP patterns, newest first.
func (r *BlockedIPRepository) List(ctx context.Context) ([]entity.BlockedIP, error) {
	rows, err := GetClient(ctx, r.client).BlockedIP.Query().Order(generated.Desc(blockedip.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("BlockedIPRepository.List: %w", err)
	}
	var result []entity.BlockedIP
	for _, b := range rows {
		result = append(result, entity.BlockedIP{ID: b.ID, IP: b.IP, Comment: deref(b.Comment), CreatedAt: b.CreatedAt})
	}
	return result, nil
}

// Create inserts a new, permanent, manually-added blocked IP.
func (r *BlockedIPRepository) Create(ctx context.Context, ip, comment string) (*entity.BlockedIP, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return nil, nil
	}
	comment = strings.TrimSpace(comment)
	createdAt := time.Now()
	row, err := GetClient(ctx, r.client).BlockedIP.Create().
		SetIP(ip).SetNillableComment(nilIfEmpty(comment)).SetCreatedAt(createdAt).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &entity.BlockedIP{ID: row.ID, IP: ip, Comment: comment, CreatedAt: createdAt}, nil
}

// Delete removes a blocked IP by ID.
func (r *BlockedIPRepository) Delete(ctx context.Context, id int) error {
	_, err := GetClient(ctx, r.client).BlockedIP.Delete().Where(blockedip.ID(id)).Exec(ctx)
	return err
}
