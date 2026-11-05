package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/redirect"
)

// RedirectRepository implements entity.RedirectRepository.
type RedirectRepository struct {
	client *generated.Client
}

// NewRedirectRepository creates a new RedirectRepository.
func NewRedirectRepository(db *sql.DB) *RedirectRepository {
	return &RedirectRepository{client: clientFor(db)}
}

// List returns all redirects ordered by from_path.
func (r *RedirectRepository) List(ctx context.Context) ([]entity.Redirect, error) {
	rows, err := GetClient(ctx, r.client).Redirect.Query().Order(generated.Asc(redirect.FieldFromPath)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("RedirectRepository.List: %w", err)
	}
	var result []entity.Redirect
	for _, red := range rows {
		result = append(result, entity.Redirect{ID: red.ID, FromPath: red.FromPath, ToPath: red.ToPath})
	}
	return result, nil
}

// Search returns a page of the redirects whose from or to path contains
// q (letter case aside; "" - all), ordered by from_path, and their count.
func (r *RedirectRepository) Search(ctx context.Context, q string, offset, limit int) ([]entity.Redirect, int, error) {
	query := GetClient(ctx, r.client).Redirect.Query()
	if q = strings.TrimSpace(q); q != "" {
		query = query.Where(redirect.Or(redirect.FromPathContainsFold(q), redirect.ToPathContainsFold(q)))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting redirects: %w", err)
	}
	rows, err := query.Order(generated.Asc(redirect.FieldFromPath)).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("searching redirects: %w", err)
	}
	result := make([]entity.Redirect, len(rows))
	for i, red := range rows {
		result[i] = entity.Redirect{ID: red.ID, FromPath: red.FromPath, ToPath: red.ToPath}
	}
	return result, total, nil
}

// Create inserts a new redirect.
func (r *RedirectRepository) Create(ctx context.Context, red entity.Redirect) (*entity.Redirect, error) {
	fromPath := normalizeFromPath(red.FromPath)
	toPath := normalizeToPath(red.ToPath)
	row, err := GetClient(ctx, r.client).Redirect.Create().SetFromPath(fromPath).SetToPath(toPath).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("RedirectRepository.Create: %w", err)
	}
	return &entity.Redirect{ID: row.ID, FromPath: fromPath, ToPath: toPath}, nil
}

// Delete removes a redirect by ID.
func (r *RedirectRepository) Delete(ctx context.Context, id int) error {
	_, err := GetClient(ctx, r.client).Redirect.Delete().Where(redirect.ID(id)).Exec(ctx)
	return err
}

// DeleteByFromPath removes a redirect by from_path.
func (r *RedirectRepository) DeleteByFromPath(ctx context.Context, fromPath string) error {
	_, err := GetClient(ctx, r.client).Redirect.Delete().Where(redirect.FromPath(normalizeFromPath(fromPath))).Exec(ctx)
	return err
}

func normalizeFromPath(p string) string {
	return strings.Trim(strings.TrimPrefix(p, "/"), " ")
}

func normalizeToPath(p string) string {
	p = strings.Trim(p, " ")
	if p != "" && !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
