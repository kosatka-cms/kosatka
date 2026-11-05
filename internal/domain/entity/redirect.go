package entity

import "context"

// Redirect represents a 301 redirect rule
type Redirect struct {
	ID       int
	FromPath string // path without leading slash
	ToPath   string // destination path with leading slash
}

// RedirectRepository defines the interface for redirect data access
type RedirectRepository interface {
	List(ctx context.Context) ([]Redirect, error)
	// Search - a page of the redirects whose from or to path contains q
	// (letter case aside; "" - all), by from path, and how many there are.
	Search(ctx context.Context, q string, offset, limit int) ([]Redirect, int, error)
	Create(ctx context.Context, r Redirect) (*Redirect, error)
	Delete(ctx context.Context, id int) error
	DeleteByFromPath(ctx context.Context, fromPath string) error
}
