// Package fallback - logging an error that the code handles itself by
// falling back to something (a default value, an empty list) instead of
// returning it. It sits below the domain packages (service, views,
// themes) so that they all log fallbacks the same way. service.LogFallback
// and modulekit.LogFallback call it.
package fallback

import (
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// Log logs an error that a service (the core's or a module's) handles
// itself by falling back to a default value or an empty list: a missing
// record (entity.ErrEntityNotFound or apperrors.ErrNotFound, wrapped or
// not) at debug level, anything else as a warning. A nil error logs
// nothing. Don't log here an error the service returns: wrap it with
// what the service was doing, and the handler logs it once.
func Log(logger *slog.Logger, err error, msg string, args ...any) {
	if err == nil || logger == nil {
		return
	}
	if entity.IsNotFound(err) {
		logger.Debug(msg, append(args, "error", err)...)
		return
	}
	logger.Warn(msg, append(args, "error", err)...)
}
