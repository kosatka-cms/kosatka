package modulekit_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestHooksRunWhileModuleEnabled - the module's hooks run while it is
// enabled and not once it is off, whatever the function does: its action
// listener, its 404 hook, its admin menu count, its language visibility.
func TestHooksRunWhileModuleEnabled(t *testing.T) {
	const module = "hooks_test_module"
	t.Cleanup(func() {
		audit.ResetListeners()
		service.ResetNotFoundHooks()
		entity.SetLanguageVisibility(nil)
	})
	var heard int
	modulekit.OnAction(module, func(context.Context, modulekit.ActionEvent) { heard++ })
	modulekit.OnNotFound(module, func(context.Context, string, string) { heard++ })
	notFound := func() {
		for _, h := range service.NotFoundHooks() {
			h(context.Background(), "/missing", "")
		}
	}
	modulekit.AddAdminMenuCount(module, "/admin/hooks-test", func(context.Context) (int, error) { return 7, nil })
	modulekit.SetLanguageVisibility(module, func(string, string, string) bool { return false })

	count := func() int {
		menu := []entity.AdminMenuSection{{Items: []entity.AdminMenuItem{{Path: "/admin/hooks-test"}}}}
		service.CountAdminMenu(context.Background(), menu, slog.New(slog.DiscardHandler))
		return menu[0].Items[0].Count
	}
	modulekit.RecordAction(context.Background(), modulekit.Action{Type: "hooks_test"})
	notFound()
	if heard != 2 || count() != 7 || entity.VisibleInLanguage("menu_link", "1", "ru") {
		t.Fatalf("on: heard %d, count %d, visible %v", heard, count(), entity.VisibleInLanguage("menu_link", "1", "ru"))
	}

	testkit.WithModules(t, map[string]bool{module: false})
	modulekit.RecordAction(context.Background(), modulekit.Action{Type: "hooks_test"})
	notFound()
	if heard != 2 || count() != 0 || !entity.VisibleInLanguage("menu_link", "1", "ru") {
		t.Errorf("off: heard %d, count %d, visible %v", heard, count(), entity.VisibleInLanguage("menu_link", "1", "ru"))
	}
}
