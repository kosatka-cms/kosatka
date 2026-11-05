package interfaces

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// The site's addresses are answered by several things - the engine's
// routes, content, term pages, View pages, a module's pages (a feed) -
// in that order: a path one of them answers is hidden from the later
// ones. Each registers itself here (AddPathOwner), and a form setting an
// address of its own (a View page, a feed) asks the others first
// (PathOwner).

// Owners of the core's addresses (PathOwner's names).
const (
	PathOwnerRoutes  = "routes"
	PathOwnerContent = "content"
	PathOwnerTerms   = "terms"
	PathOwnerViews   = "views"
)

type pathOwner struct {
	name    string
	answers func(ctx context.Context, path string) (bool, error)
}

var (
	pathOwnersMu sync.RWMutex
	pathOwners   []pathOwner
)

// AddPathOwner registers answers, telling whether name (the core's
// PathOwner* or a module's name) answers path (no leading "/"); one
// registered under the same name before is replaced.
func AddPathOwner(name string, answers func(ctx context.Context, path string) (bool, error)) {
	pathOwnersMu.Lock()
	defer pathOwnersMu.Unlock()
	for i, o := range pathOwners {
		if o.name == name {
			pathOwners[i].answers = answers
			return
		}
	}
	pathOwners = append(pathOwners, pathOwner{name: name, answers: answers})
}

// PathOwner - the name of the first owner but except that answers path
// ("" - none): the address is free for except. A disabled module's
// addresses are free.
func PathOwner(ctx context.Context, path, except string) (string, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return "", nil
	}
	pathOwnersMu.RLock()
	owners := append([]pathOwner(nil), pathOwners...)
	pathOwnersMu.RUnlock()
	for _, o := range owners {
		if o.name == except || !service.ModuleEnabled(o.name) {
			continue
		}
		ok, err := o.answers(ctx, path)
		if err != nil {
			return "", fmt.Errorf("checking whether %s answers %q: %w", o.name, path, err)
		}
		if ok {
			return o.name, nil
		}
	}
	return "", nil
}

func resetPathOwners() {
	pathOwnersMu.Lock()
	defer pathOwnersMu.Unlock()
	pathOwners = nil
}
