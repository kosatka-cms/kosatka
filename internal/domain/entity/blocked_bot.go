package entity

import (
	"context"
	"time"
)

// BlockedBot represents a User-Agent pattern to block
type BlockedBot struct {
	ID        int
	Pattern   string // substring to match in User-Agent (case-insensitive)
	Comment   string // free-text note, e.g. why this was blocked
	CreatedAt time.Time
}

// BlockedBotRepository defines the interface for blocked bot data access
type BlockedBotRepository interface {
	List(ctx context.Context) ([]BlockedBot, error)
	Create(ctx context.Context, pattern, comment string) (*BlockedBot, error)
	Delete(ctx context.Context, id int) error
}
