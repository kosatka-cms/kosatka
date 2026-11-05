package admin

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestAdminHeaderUserMenu - the top bar's user menu is rendered by the
// server: a signed-in user's dropdown with their name, a guest's links -
// never both (no script swaps them).
func TestAdminHeaderUserMenu(t *testing.T) {
	render := func(data map[string]any) string {
		t.Helper()
		var buf strings.Builder
		ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
		if err := ts.RenderPageBlock(&buf, "admin/reports/popular.html", "admin_header", data); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	out := render(map[string]any{"user": &entity.User{ID: 1, Username: "ann"}, "isAdmin": true})
	if !strings.Contains(out, `<span class="user-name">ann</span>`) || !strings.Contains(out, `href="/logout"`) ||
		strings.Contains(out, `href="/login"`) || strings.Contains(out, "display: none") {
		t.Errorf("a signed-in user's top bar: %s", out)
	}
	if out := render(map[string]any{}); !strings.Contains(out, `href="/login"`) || strings.Contains(out, `id="user-menu-toggle"`) {
		t.Errorf("a guest's top bar: %s", out)
	}
}
