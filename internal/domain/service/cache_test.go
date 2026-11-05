package service

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestZoneProbe - no probe (no double render) while user time zones are
// off; a zone other than the site's when on.
func TestZoneProbe(t *testing.T) {
	t.Cleanup(func() { sitetime.SetUserZones(false) })
	if zoneProbe() != nil {
		t.Fatal("off - no probe")
	}
	sitetime.SetUserZones(true)
	if p := zoneProbe(); p == nil || sitetime.IsSite(p) {
		t.Fatalf("probe = %v", p)
	}
}

// TestForceUpdateCoalesces - calls made while a refresh runs add up to one
// more refresh after it, not one each; a call after that starts a new one.
func TestForceUpdateCoalesces(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	s := &CacheService{logger: slog.New(slog.DiscardHandler)}
	s.rebuild = func(context.Context) error {
		runs.Add(1)
		started <- struct{}{}
		<-release
		return nil
	}
	idle := func() bool {
		s.forceMu.Lock()
		defer s.forceMu.Unlock()
		return !s.forceRunning
	}

	s.ForceUpdate()
	<-started
	for range 30 {
		s.ForceUpdate()
	}
	release <- struct{}{} // the first refresh ends; the one pending starts
	<-started
	release <- struct{}{}
	deadline := time.Now().Add(2 * time.Second)
	for !idle() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := runs.Load(); got != 2 {
		t.Fatalf("refreshes: %d, want 2", got)
	}

	s.ForceUpdate()
	<-started
	release <- struct{}{}
	for !idle() && time.Now().Before(deadline.Add(2*time.Second)) {
		time.Sleep(time.Millisecond)
	}
	if got := runs.Load(); got != 3 {
		t.Fatalf("after idle: %d refreshes, want 3", got)
	}
}

// TestCacheStopWaitsForRefresh - stopping waits for the refresh running;
// no refresh starts after it.
func TestCacheStopWaitsForRefresh(t *testing.T) {
	var runs atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	s := &CacheService{logger: slog.New(slog.DiscardHandler)}
	s.rebuild = func(context.Context) error {
		runs.Add(1)
		started <- struct{}{}
		<-release
		return nil
	}
	s.ForceUpdate()
	<-started
	stopped := make(chan struct{})
	go func() {
		s.stop()
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("stopped while a refresh ran")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("not stopped after the refresh")
	}
	s.ForceUpdate()
	time.Sleep(20 * time.Millisecond)
	if got := runs.Load(); got != 1 {
		t.Fatalf("refreshes: %d, want 1 (none after stopping)", got)
	}
}

// flakyMenus - a MenuService of the menus "main" and "footer" (a tree of
// one item, its title - its version) whose list or a tree may fail.
type flakyMenus struct {
	MenuService
	version string
	listErr error
	treeErr map[string]error
}

func (m *flakyMenus) Menus(context.Context) (entity.MenuDefs, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return entity.MenuDefs{{MachineName: "main"}, {MachineName: "footer"}}, nil
}

func (m *flakyMenus) ResolvedTree(_ context.Context, name string) ([]MenuTreeItem, error) {
	if err := m.treeErr[name]; err != nil {
		return nil, err
	}
	return []MenuTreeItem{{Title: name + " " + m.version}}, nil
}

// TestCacheKeepsMenusOnFailure - a refresh whose menus fail to load keeps
// the previous ones (a menu whose tree fails - its previous tree), rather
// than pages without them; a menu gone is dropped.
func TestCacheKeepsMenusOnFailure(t *testing.T) {
	menus := &flakyMenus{version: "v1"}
	s := &CacheService{commentsService: &accessComments{}, menuService: menus, logger: slog.New(slog.DiscardHandler)}
	refresh := func() map[string][]MenuTreeItem {
		t.Helper()
		if err := s.update(context.Background()); err != nil {
			t.Fatal(err)
		}
		return s.GetSidebarData().MenuTrees
	}
	title := func(trees map[string][]MenuTreeItem, name string) string {
		if len(trees[name]) == 0 {
			return ""
		}
		return trees[name][0].Title
	}
	refresh()

	menus.version, menus.listErr = "v2", errors.New("connection reset")
	if got := refresh(); title(got, "main") != "main v1" || title(got, "footer") != "footer v1" {
		t.Errorf("the list failed: %v", got)
	}
	menus.listErr, menus.treeErr = nil, map[string]error{"main": errors.New("timeout"), "footer": entity.ErrEntityNotFound}
	if got := refresh(); title(got, "main") != "main v1" || len(got["footer"]) != 0 {
		t.Errorf("a tree failed, another gone: %v", got)
	}
	menus.treeErr = nil
	if got := refresh(); title(got, "main") != "main v2" || title(got, "footer") != "footer v2" {
		t.Errorf("loaded again: %v", got)
	}
}

// TestVisitorCacheGenerations - what is built from an older refresh's
// data isn't kept once a newer one is out, nor read for it.
func TestVisitorCacheGenerations(t *testing.T) {
	var c visitorCache[string]
	c.put(1, "k", "first")
	if v, ok := c.get(1, "k"); !ok || v != "first" {
		t.Fatalf("kept: %q %v", v, ok)
	}
	c.reset(2)
	c.put(1, "k", "stale") // built before refresh 2, put after it
	if v, ok := c.get(2, "k"); ok {
		t.Errorf("an older refresh's kept: %q", v)
	}
	c.put(2, "k", "second")
	if _, ok := c.get(1, "k"); ok {
		t.Error("read for an older refresh")
	}
	c.put(3, "other", "third") // a newer refresh's drops the rest
	if _, ok := c.get(3, "k"); ok {
		t.Error("a newer refresh kept the older entries")
	}
	if v, ok := c.get(3, "other"); !ok || v != "third" {
		t.Errorf("newer: %q %v", v, ok)
	}
}
