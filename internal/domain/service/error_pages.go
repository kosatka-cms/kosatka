package service

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// The site's error pages (Site information → 404/403 page): node IDs
// resolved from the configured paths, set at startup and on save, read on
// every error response.
var (
	errorPagesMu sync.RWMutex
	errorPages   = map[int][]int{}
)

// SetErrorPages sets the nodes shown for an error status (404 or 403).
func SetErrorPages(status int, ids []int) {
	errorPagesMu.Lock()
	defer errorPagesMu.Unlock()
	errorPages[status] = append([]int(nil), ids...)
}

// ErrorPageIDs - the nodes configured for an error status.
func ErrorPageIDs(status int) []int {
	errorPagesMu.RLock()
	defer errorPagesMu.RUnlock()
	return append([]int(nil), errorPages[status]...)
}

// ErrorPageStatus - the error status a node is configured for (404 or
// 403), 0 if it is not an error page.
func ErrorPageStatus(nodeID int) int {
	errorPagesMu.RLock()
	defer errorPagesMu.RUnlock()
	for status, ids := range errorPages {
		for _, id := range ids {
			if id == nodeID {
				return status
			}
		}
	}
	return 0
}

// ParseErrorPagePaths splits the setting into paths: one per line, blank
// lines skipped; a full URL is reduced to its path.
func ParseErrorPagePaths(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		if u, err := url.Parse(p); err == nil && u.Host != "" {
			p = u.Path
		}
		out = append(out, "/"+strings.Trim(p, "/"))
	}
	return out
}

// ResolveErrorPages finds the node of each path (by slug); paths with no
// node are returned in missing.
func ResolveErrorPages(ctx context.Context, bySlug func(context.Context, string) (*entity.Node, error), paths []string) (nodes []*entity.Node, missing []string) {
	for _, p := range paths {
		n, err := bySlug(ctx, strings.TrimPrefix(p, "/"))
		if err != nil || n == nil {
			missing = append(missing, p)
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes, missing
}

// LoadErrorPages resolves the site's 404/403 settings and makes them
// current; unknown paths are skipped (the form rejects them on save).
func LoadErrorPages(ctx context.Context, bySlug func(context.Context, string) (*entity.Node, error), site *entity.SiteInformation) {
	for status, text := range map[int]string{404: site.Page404, 403: site.Page403} {
		nodes, _ := ResolveErrorPages(ctx, bySlug, ParseErrorPagePaths(text))
		ids := make([]int, len(nodes))
		for i, n := range nodes {
			ids[i] = n.ID
		}
		SetErrorPages(status, ids)
	}
}
