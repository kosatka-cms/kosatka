package entity

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
)

// ErrEntityNotFound - a missing record: the engine's one sentinel for it,
// apperrors.ErrNotFound itself (the handlers answer it 404); ent's
// not-found errors match it (generated.NotFoundError.Is), the modules'
// too (schemakit.ErrEntityNotFound).
var ErrEntityNotFound = apperrors.ErrNotFound

// IsNotFound - err says the record is missing (ErrEntityNotFound, which
// ent's not-found errors match, wrapped or not).
func IsNotFound(err error) bool {
	return errors.Is(err, ErrEntityNotFound)
}

// Found - v a repository looked up (err its error): a missing record
// as it is (IsNotFound), another error wrapped with what was looked for
// (a format of args).
func Found[T any](v T, err error, what string, args ...any) (T, error) {
	if err == nil {
		return v, nil
	}
	var zero T
	if IsNotFound(err) {
		return zero, err
	}
	return zero, fmt.Errorf("%s: %w", fmt.Sprintf(what, args...), err)
}

// Wrapped - v a repository returned, its error wrapped with what was
// done (a format of args).
func Wrapped[T any](v T, err error, what string, args ...any) (T, error) {
	if err != nil {
		var zero T
		return zero, fmt.Errorf("%s: %w", fmt.Sprintf(what, args...), err)
	}
	return v, nil
}

// IsDuplicateKey detects a unique-constraint violation wrapped by ent:
// MySQL's "Duplicate entry", PostgreSQL's SQLSTATE 23505. Ent has no typed
// error for it (its ConstraintError is any constraint), so it matches the
// driver's text, keeping the drivers out of the domain.
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "Duplicate entry") || strings.Contains(s, "SQLSTATE 23505")
}
