package admin

import (
	"context"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestModulesPageGroups - the modules by their categories in the page's
// order (no empty ones), how many are on in each; a module unknown to the
// registry's categories is under "Other".
func TestModulesPageGroups(t *testing.T) {
	ctx := context.Background()
	rows := []moduleRow{{Name: "subscriptions", Enabled: true}, {Name: "feeds"}, {Name: "forms", Enabled: true}, {Name: "polls"}, {Name: "nowhere"}}
	groups := moduleGroups(ctx, rows)
	var got []string
	for _, g := range groups {
		var names []string
		for _, m := range g.Modules {
			names = append(names, m.Name)
		}
		got = append(got, g.Name+":"+strings.Join(names, ",")+":"+string(rune('0'+g.On)))
	}
	want := "content:forms,polls:1 community:subscriptions:1 integrations:feeds:0 other:nowhere:0"
	if strings.Join(got, " ") != want {
		t.Errorf("groups %v, want %s", got, want)
	}
	if groups[0].Label != "Content" || groups[len(groups)-1].Label != "Other" {
		t.Errorf("labels: %+v", groups)
	}
}

// TestModulesPageRender - the links to the groups with their counts; a
// group folded (opened by its link), its title with how many are on; a module's switch, its pills (not
// installed, from its own repository), Uninstall for a disabled installed
// one; Save at the bottom of the one form.
func TestModulesPageRender(t *testing.T) {
	rows := []moduleRow{
		{Name: "subscriptions", Label: "Subscriptions", Description: "Notifications.", Enabled: true, Installed: true},
		{Name: "ratings", Label: "Ratings", Description: "Stars.", Installed: true},
		{Name: "polls", Label: "Polls", Description: "Votes."},
		{Name: "hello", Label: "Hello", Description: "Greets.", External: true, Installed: true, Enabled: true},
	}
	data := map[string]any{"groups": moduleGroups(context.Background(), rows), "uninstallAvailable": true}
	var buf strings.Builder
	ts := service.NewTemplateService(false).(*service.DefaultTemplateService)
	if err := ts.RenderPageBlock(&buf, "admin/modules/index.html", "content", data); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{
		`<a href="#modules-community" class="admin-module-cat">Community <span>1/2</span></a>`,
		`<a href="#modules-other" class="admin-module-cat">Other <span>1/1</span></a>`,
		`<details class="admin-settings-group admin-settings-fold" id="modules-community" data-settings-fold>`,
		`<h2 class="admin-settings-fold-title admin-module-group-title">Community <span class="admin-module-count">1 of 2 on</span></h2>`,
		`data-settings-fold-all`,
		`id="module_subscriptions" name="module_subscriptions" value="1" class="form-check-input" checked>`,
		`id="module_ratings" name="module_ratings" value="1" class="form-check-input">`,
		`<a href="/admin/config/modules/ratings/uninstall" class="link-danger">Uninstall</a>`,
		`<span class="admin-pill">Not installed</span>`, `admin-pill--external">from its own repository</span>`,
		`<div class="admin-settings-actions"> <button type="submit" class="btn btn-primary">Save</button>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if strings.Contains(out, "/admin/config/modules/subscriptions/uninstall") || strings.Contains(out, "/admin/config/modules/polls/uninstall") {
		t.Error("Uninstall only for a disabled installed module")
	}
}
