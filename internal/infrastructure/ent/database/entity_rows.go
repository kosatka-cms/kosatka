package database

import (
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// one - the row a query returned (err its error) as an entity (conv); a
// row not found - entity.ErrEntityNotFound with what (a format of args)
// it was.
func one[R, E any](row R, err error, conv func(R) E, what string, args ...any) (*E, error) {
	if err != nil {
		return nil, notFoundAs(err, what, args...)
	}
	e := conv(row)
	return &e, nil
}

// all - the rows a query returned (err its error, wrapped with op) as
// entities (conv).
func all[R, E any](rows []R, err error, conv func(R) E, op string) ([]E, error) {
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	out := make([]E, len(rows))
	for i, row := range rows {
		out[i] = conv(row)
	}
	return out, nil
}

// notFoundAs - err of a query for one row: not found -
// entity.ErrEntityNotFound with what (a format of args) it was; nil - nil.
func notFoundAs(err error, what string, args ...any) error {
	if generated.IsNotFound(err) {
		return fmt.Errorf("%w: %s", entity.ErrEntityNotFound, fmt.Sprintf(what, args...))
	}
	return err
}
