package galleries

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

func withModule(t *testing.T, on bool) {
	t.Helper()
	testkit.WithModules(t, map[string]bool{manifest.Name: on})
}

// TestFormSections - the content type form and the node form show the
// module's sections with their values while it is on (the type form's as
// rows of its "Modules" group), nothing while off.
func TestFormSections(t *testing.T) {
	testkit.UseEngineRoot(t)
	typeData := map[string]any{
		"contentType": &modulekit.ContentTypeDef{ID: 4, MachineName: "hero", Label: "Hero"},
		"moduleSettings": map[string]any{manifest.Name: typeSettingsForm{
			TypeSettings: TypeSettings{DefaultEnabled: true, PictureField: "photo"},
			ImageFields:  []modulekit.FieldInstanceDef{{MachineName: "photo", Label: "Photo"}},
		}},
	}
	nodeData := map[string]any{
		"canEditNodeSettings": true,
		"dc":                  &modulekit.DynamicContent{},
		"form":                map[string]any{},
		"moduleSettings":      map[string]any{manifest.Name: true},
	}

	withModule(t, true)
	out := testkit.RenderPageBlock(t, "admin/content-types/edit.html", "content", typeData)
	for _, want := range []string{
		`id="default_galleries_enabled" name="default_galleries_enabled" value="1" class="form-check-input" checked`,
		`<option value="photo" selected>`,
		// Rows of the form's "Modules" group, each naming the module.
		`<h2 class="admin-settings-fold-title">Modules</h2>`,
		`<span class="admin-setting-help">Galleries · `,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("content type form lacks %s", want)
		}
	}
	if out := testkit.RenderPageBlock(t, "node/edit_dynamic.html", "edit_node_settings.html", nodeData); !strings.Contains(out, `id="galleries_enabled" value="1" checked`) {
		t.Errorf("node form: no checked galleries switch:\n%s", out)
	}

	withModule(t, false)
	if out := testkit.RenderPageBlock(t, "admin/content-types/edit.html", "content", typeData); strings.Contains(out, "galleries") {
		t.Error("content type form: the section while the module is off")
	}
	if out := testkit.RenderPageBlock(t, "node/edit_dynamic.html", "edit_node_settings.html", nodeData); strings.Contains(out, "galleries") {
		t.Error("node form: the panel while the module is off")
	}
}

// TestNodePageParts - a node page shows the "Galleries" button to an
// editor, links to galleries of more than one image under the node and,
// in the "The content's galleries" block, the lazy mount (with the block's
// previews); the block shows nothing off a node page.
func TestNodePageParts(t *testing.T) {
	testkit.UseEngineRoot(t)
	withModule(t, true)
	data := map[string]any{
		"canEdit": true,
		"item":    &modulekit.DynamicContent{Node: modulekit.Node{ID: 7}},
		"moduleNode": map[string]any{manifest.Name: nodePage{Galleries: Galleries{
			{ID: 1, Title: new("Covers"), Items: GalleryItems{{}, {}}},
			{ID: 2, Title: new("Single"), Items: GalleryItems{{}}},
		}}},
	}
	if out := testkit.RenderPageBlock(t, "dynamic/page.html", "node_actions_galleries", data); !strings.Contains(out, `href="/node/7/galleries"`) {
		t.Errorf("no galleries button: %s", out)
	}
	out := testkit.RenderPageBlock(t, "dynamic/page.html", "node_module_links", data)
	if !strings.Contains(out, `<a href="/galleries/1">Covers (2)</a>`) || strings.Contains(out, "/galleries/2") {
		t.Errorf("links: %s", out)
	}
	block := func(ctx map[string]any, previews string) string {
		p := modulekit.BlockPlacement{PluginConfig: map[string]string{manifest.NodeGalleriesPreviews: previews}}
		return testkit.RenderPageBlock(t, "dynamic/page.html", "block_node_galleries", map[string]any{"Ctx": ctx, "Placement": p})
	}
	if out := block(data, "0"); !strings.Contains(out, `data-endpoint="/api/node/7/galleries-embed"`) {
		t.Errorf("no lazy block: %s", out)
	}
	if out := block(data, "3"); !strings.Contains(out, `data-endpoint="/api/node/7/galleries-embed?previews=3"`) {
		t.Errorf("the block's previews: %s", out)
	}
	if out := block(map[string]any{}, "3"); strings.TrimSpace(out) != "" {
		t.Errorf("a block off a node page: %q", out)
	}

	data["canEdit"] = false
	if out := testkit.RenderPageBlock(t, "dynamic/page.html", "node_actions_galleries", data); strings.Contains(out, "galleries") {
		t.Errorf("a button for a reader: %s", out)
	}
	delete(data, "moduleNode")
	if out := testkit.RenderPageBlock(t, "dynamic/page.html", "node_module_links", data); strings.TrimSpace(out) != "" {
		t.Errorf("links without the module's data: %q", out)
	}
}
