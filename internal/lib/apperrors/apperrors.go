package apperrors

import "errors"

// ErrNotFound is returned when a requested entity does not exist (HTTP 404).
var ErrNotFound = errors.New("not found")

// IsNotFound reports whether err is or wraps ErrNotFound.
func IsNotFound(err error) bool {
	return err != nil && errors.Is(err, ErrNotFound)
}
