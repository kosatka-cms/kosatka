package entity

import "context"

// UnitOfWork - a transaction: Do runs fn in one (the repositories called
// with fn's ctx write in it; in a transaction already - in that one),
// rolled back when fn fails.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
