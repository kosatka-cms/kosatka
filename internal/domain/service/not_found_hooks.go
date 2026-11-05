package service

import (
	"context"
	"slices"
	"sync"
)

// NotFoundHook - told of a page request answered "not found" (404): its
// path (no query) and where the visitor came from ("" - nowhere told).
type NotFoundHook func(ctx context.Context, path, referrer string)

var (
	notFoundHooksMu sync.RWMutex
	notFoundHooks   []NotFoundHook
)

// OnNotFound adds a hook told of the requests answered 404 (a module's
// log of them). It runs after the answer, in the request: quick, its
// errors its own.
func OnNotFound(fn NotFoundHook) {
	notFoundHooksMu.Lock()
	defer notFoundHooksMu.Unlock()
	notFoundHooks = append(notFoundHooks, fn)
}

// ResetNotFoundHooks forgets the hooks (an application built again - the
// tests build it more than once).
func ResetNotFoundHooks() {
	notFoundHooksMu.Lock()
	defer notFoundHooksMu.Unlock()
	notFoundHooks = nil
}

// NotFoundHooks - the hooks added (nil - none: nothing to watch).
func NotFoundHooks() []NotFoundHook {
	notFoundHooksMu.RLock()
	defer notFoundHooksMu.RUnlock()
	return slices.Clone(notFoundHooks)
}
