package hello

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"example.com/kosatka-hello/ent/generated"
	"example.com/kosatka-hello/ent/generated/greeting"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Greeting - one greeting.
type Greeting struct {
	ID        int
	Text      string
	CreatedAt time.Time
}

// Repository - the greetings, on the module's own ent schema (ent/).
type Repository struct {
	db *sql.DB
}

// NewRepository - the greetings on db, in the site's dialect.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// client - the module's ent client: the context's transaction (InTx - a
// hook running in the core's change, say), else the database.
func (r *Repository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// Add stores a greeting.
func (r *Repository) Add(ctx context.Context, text string) (Greeting, error) {
	row, err := r.client(ctx).Greeting.Create().SetText(text).SetCreatedAt(time.Now()).Save(ctx)
	if err != nil {
		return Greeting{}, fmt.Errorf("adding a greeting: %w", err)
	}
	return Greeting{ID: row.ID, Text: row.Text, CreatedAt: row.CreatedAt}, nil
}

// List - the greetings, newest first.
func (r *Repository) List(ctx context.Context) ([]Greeting, error) {
	return r.Latest(ctx, 0)
}

// Latest - the newest limit greetings (0 - all).
func (r *Repository) Latest(ctx context.Context, limit int) ([]Greeting, error) {
	q := r.client(ctx).Greeting.Query().Order(generated.Desc(greeting.FieldCreatedAt), generated.Desc(greeting.FieldID))
	if limit > 0 {
		q = q.Limit(limit)
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing greetings: %w", err)
	}
	out := make([]Greeting, 0, len(rows))
	for _, row := range rows {
		out = append(out, Greeting{ID: row.ID, Text: row.Text, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

// Count - how many greetings there are.
func (r *Repository) Count(ctx context.Context) (int, error) {
	n, err := r.client(ctx).Greeting.Query().Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting greetings: %w", err)
	}
	return n, nil
}
