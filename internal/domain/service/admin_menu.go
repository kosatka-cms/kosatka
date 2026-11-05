package service

import (
	"context"
	"log/slog"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// adminMenuCount - an admin menu item's counter and what it counts.
type adminMenuCount struct {
	count func(ctx context.Context) (int, error)
	label string
}

// adminMenuCounts - the counters of admin menu items, by the item's path
// (RegisterAdminMenuCount).
var (
	adminMenuCountsMu sync.RWMutex
	adminMenuCounts   = map[string]adminMenuCount{}
)

// RegisterAdminMenuCount sets the counter of the admin menu item at path:
// the number shown by it (what waits there - new reports, say), counted
// for each admin page while the item is shown; label (i18n.N, optional) -
// what it counts, where the Overview lists it ("" - the item's label).
func RegisterAdminMenuCount(path string, count func(ctx context.Context) (int, error), label ...string) {
	adminMenuCountsMu.Lock()
	defer adminMenuCountsMu.Unlock()
	c := adminMenuCount{count: count}
	if len(label) > 0 {
		c.label = label[0]
	}
	adminMenuCounts[path] = c
}

// CountAdminMenu fills the Count of menu's items that have a counter; one
// that can't count shows none (logged).
func CountAdminMenu(ctx context.Context, menu []entity.AdminMenuSection, logger *slog.Logger) {
	adminMenuCountsMu.RLock()
	defer adminMenuCountsMu.RUnlock()
	for i := range menu {
		for j := range menu[i].Items {
			c, ok := adminMenuCounts[menu[i].Items[j].Path]
			if !ok {
				continue
			}
			n, err := c.count(ctx)
			if err != nil {
				LogFallback(logger, err, "counting an admin menu item (none shown)", "path", menu[i].Items[j].Path)
				continue
			}
			menu[i].Items[j].Count, menu[i].Items[j].CountLabel = n, c.label
		}
	}
}

// VisibleAdminMenu - the admin toolbar for one user (entity.AdminMenu):
// items of disabled modules and features and items whose permission the
// user lacks (has) are dropped, and so are dropdown sections left empty.
func VisibleAdminMenu(has func(permission string) bool) []entity.AdminMenuSection {
	out := make([]entity.AdminMenuSection, 0, len(entity.AdminMenu()))
	for _, section := range entity.AdminMenu() {
		if section.Path != "" {
			out = append(out, section)
			continue
		}
		items := make([]entity.AdminMenuItem, 0, len(section.Items))
		for _, item := range section.Items {
			if item.Module != "" && !ModuleEnabled(item.Module) {
				continue
			}
			if item.Feature != "" && !FeatureEnabled(item.Feature) {
				continue
			}
			if item.Permission != "" && (has == nil || !has(item.Permission)) {
				continue
			}
			items = append(items, item)
		}
		if len(items) == 0 {
			continue
		}
		section.Items = items
		out = append(out, section)
	}
	return out
}
