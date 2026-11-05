package service

import (
	"context"
	"slices"
	"sync"
)

// DashboardBlock - a module's block on the admin's Overview (/admin):
// Name - its template, dashboard_<Name> (a define in the module's
// templates/partials), shown in a card titled Title (i18n.N) with an
// optional "all" link (Link, LinkLabel); Side - the narrow column, else
// the wide one; Weight - its place among the column's module blocks
// (lightest first, after the core's); Permission - needed to see it
// ("" - anyone in the admin); Data - the template's .Data, for the
// request (an error leaves the block out, logged).
type DashboardBlock struct {
	Name       string
	Module     string
	Title      string
	Link       string
	LinkLabel  string
	Side       bool
	Weight     int
	Permission string
	Data       func(ctx context.Context) (any, error)
}

var (
	dashboardMu     sync.RWMutex
	dashboardBlocks []DashboardBlock
)

// RegisterDashboardBlock adds a module's block to the Overview.
func RegisterDashboardBlock(b DashboardBlock) {
	dashboardMu.Lock()
	defer dashboardMu.Unlock()
	dashboardBlocks = append(dashboardBlocks, b)
}

// DashboardBlocks - the modules' blocks, lightest first (then as
// registered).
func DashboardBlocks() []DashboardBlock {
	dashboardMu.RLock()
	defer dashboardMu.RUnlock()
	out := slices.Clone(dashboardBlocks)
	slices.SortStableFunc(out, func(a, b DashboardBlock) int { return a.Weight - b.Weight })
	return out
}

// ResetDashboardBlocks forgets the blocks (the modules wire again).
func ResetDashboardBlocks() {
	dashboardMu.Lock()
	defer dashboardMu.Unlock()
	dashboardBlocks = nil
}
