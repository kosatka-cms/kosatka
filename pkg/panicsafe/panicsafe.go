// Package panicsafe recovers panics in goroutines that run outside Go's
// per-request HTTP panic recovery.
//
// net/http only recovers a panic for the goroutine it spawns to handle a
// given request. A bare `go func() { ... }()` launched from inside a
// handler, or from a background service (cache warming, search fan-out,
// fire-and-forget email sending), runs outside that safety net: an
// unrecovered panic there crashes the entire process, not just the request
// or task that triggered it.
package panicsafe

import (
	"log/slog"
	"runtime/debug"
)

// Recover recovers a panic in the current goroutine and logs it instead of
// letting it crash the process. Call it as the first line of any goroutine
// launched with `go func() { ... }()`:
//
//	go func() {
//		defer panicsafe.Recover(logger, "cache.warm")
//		...
//	}()
//
// logger may be nil, in which case the panic is still recovered but not
// logged.
func Recover(logger *slog.Logger, label string) {
	if r := recover(); r != nil {
		if logger != nil {
			logger.Error("recovered from panic",
				"component", label,
				"panic", r,
				"stack", string(debug.Stack()),
			)
		}
	}
}
