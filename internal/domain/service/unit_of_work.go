package service

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// inUnitOfWork runs fn in uow's transaction; without one (nil - a test, a
// service built without it), as is.
func inUnitOfWork(ctx context.Context, uow entity.UnitOfWork, fn func(ctx context.Context) error) error {
	if uow == nil {
		return fn(ctx)
	}
	return uow.Do(ctx, fn)
}
