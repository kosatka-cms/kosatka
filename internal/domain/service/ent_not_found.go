package service

import (
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/fallback"
)

// IsNotFound - err says the record is missing (entity.ErrEntityNotFound,
// which ent's not-found errors match, or apperrors.ErrNotFound, wrapped
// or not).
func IsNotFound(err error) bool {
	return entity.IsNotFound(err)
}

// LogFallback logs an error a service (the core's or a module's) deals
// with itself by going on with a fallback (a default value, an empty
// list): a missing record at debug level, anything else as a warning. Nil - nothing. An error the service
// returns isn't logged here: it is wrapped with what the service was
// doing and logged once, where it is handled (the handler). The
// implementation is fallback.Log, which the packages below service
// (views, themes) call directly.
func LogFallback(logger *slog.Logger, err error, msg string, args ...any) {
	fallback.Log(logger, err, msg, args...)
}
