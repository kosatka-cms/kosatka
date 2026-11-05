package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestAdminListFoot - a paged list's foot: which of how many the page
// shows (the last page's count capped by the total), the note after it,
// the pager only with more than one page; nothing for an empty list
// without a note.
func TestAdminListFoot(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	render := func(page, total int, note string) string {
		t.Helper()
		ctx := map[string]any{"pagination": pagination.New(page, total, 25, "/admin/content", nil)}
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "admin_list_foot", map[string]any{"Ctx": ctx, "Note": note}, "admin/list_foot.html", "pagination.html"); err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(buf.String()), " ")
	}
	if out := render(2, 60, ""); !strings.Contains(out, "<span>26-50 of 60</span>") || !strings.Contains(out, `class="pager"`) {
		t.Errorf("a middle page: %s", out)
	}
	if out := render(3, 60, "kept 90 days"); !strings.Contains(out, "<span>51-60 of 60 · kept 90 days</span>") {
		t.Errorf("the last page with a note: %s", out)
	}
	if out := render(1, 7, ""); !strings.Contains(out, "<span>1-7 of 7</span>") || strings.Contains(out, `class="pager"`) {
		t.Errorf("one page: %s", out)
	}
	if out := render(1, 0, ""); strings.Contains(out, "admin-table-foot") {
		t.Errorf("an empty list: %s", out)
	}
}
