package galleries

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// fakeSettings - SettingsRepository in memory.
type fakeSettings struct {
	types map[string]TypeSettings
	saved map[int]TypeSettings
	nodes map[int]bool
}

func newFakeSettings() *fakeSettings {
	return &fakeSettings{types: map[string]TypeSettings{}, saved: map[int]TypeSettings{}, nodes: map[int]bool{}}
}

func (f *fakeSettings) TypeSettings(_ context.Context, contentType string) (TypeSettings, error) {
	if s, ok := f.types[contentType]; ok {
		return s, nil
	}
	return DefaultTypeSettings(), nil
}

func (f *fakeSettings) SaveTypeSettings(_ context.Context, id int, s TypeSettings) error {
	f.saved[id] = s
	return nil
}

func (f *fakeSettings) NodeEnabled(_ context.Context, nodeID int) (bool, error) {
	return f.nodes[nodeID], nil
}

func (f *fakeSettings) EnabledNodeIDs(context.Context) ([]int, error) {
	var ids []int
	for id, on := range f.nodes {
		if on {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (f *fakeSettings) SetNodeEnabled(_ context.Context, nodeID int, on bool) error {
	f.nodes[nodeID] = on
	return nil
}

type fakeTypeFields modulekit.FieldInstanceDefs

func (f fakeTypeFields) FieldsByContentTypeID(context.Context, int) (modulekit.FieldInstanceDefs, error) {
	return modulekit.FieldInstanceDefs(f), nil
}

type fakeSummaries Galleries

func (f fakeSummaries) SummaryByNodeID(context.Context, int) (Galleries, error) {
	return Galleries(f), nil
}

// TestTypeSettingsHook - the type's section shows its settings and image
// fields; saving keeps only one of its own image fields as the main content
// image's.
func TestTypeSettingsHook(t *testing.T) {
	ctx := context.Background()
	settings := newFakeSettings()
	settings.types["hero"] = TypeSettings{DefaultEnabled: true, PictureField: "photo"}
	fields := fakeTypeFields{
		{MachineName: "photo", FieldType: modulekit.FieldTypeImage},
		{MachineName: "body", FieldType: modulekit.FieldTypeText},
	}
	hook := typeSettingsHook(settings, fields)
	ct := modulekit.ContentTypeDef{ID: 4, MachineName: "hero"}

	v, err := hook.Load(ctx, ct)
	if err != nil {
		t.Fatal(err)
	}
	form := v.(typeSettingsForm)
	if !form.DefaultEnabled || len(form.ImageFields) != 1 || form.ImageFields[0].MachineName != "photo" {
		t.Errorf("load: %+v", form)
	}

	for _, tc := range []struct {
		name string
		form url.Values
		want TypeSettings
	}{
		{"all set", url.Values{"default_galleries_enabled": {"1"}, "galleries_picture_field": {"photo"}}, TypeSettings{DefaultEnabled: true, PictureField: "photo"}},
		{"not an image field", url.Values{"galleries_picture_field": {"body"}}, TypeSettings{}},
	} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := hook.Save(ctx, ct, r); err != nil {
			t.Fatal(err)
		}
		if got := settings.saved[4]; got != tc.want {
			t.Errorf("%s: saved %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// TestNodeSettingsHook - the node's panel: a new node starts from the
// type's default, an existing one shows its own switch, a form shown again
// what was sent; saving sets the switch, a new node saved without the
// panel gets the type's default.
func TestNodeSettingsHook(t *testing.T) {
	ctx := context.Background()
	settings := newFakeSettings()
	settings.types["hero"] = TypeSettings{DefaultEnabled: true}
	hook := nodeSettingsHook(settings)

	for _, tc := range []struct {
		name      string
		node      *modulekit.Node
		submitted url.Values
		want      bool
	}{
		{"new node: the type's default", nil, nil, true},
		{"existing node: its own switch", &modulekit.Node{ID: 9}, nil, false},
		{"shown again: what was sent", nil, url.Values{"title": {"x"}}, false},
	} {
		v, err := hook.Load(ctx, "hero", tc.node, tc.submitted)
		if err != nil || v != tc.want {
			t.Errorf("%s: %v %v, want %v", tc.name, v, err, tc.want)
		}
	}

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("galleries_enabled=1"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := hook.Save(ctx, modulekit.Node{ID: 9}, r); err != nil || !settings.nodes[9] {
		t.Fatalf("save on: %v %v", err, settings.nodes)
	}
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("title=x"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := hook.Save(ctx, modulekit.Node{ID: 9}, r); err != nil || settings.nodes[9] {
		t.Fatalf("save off: %v %v", err, settings.nodes)
	}
	// A new node saved by someone the panel isn't shown to: the type's
	// default.
	if err := hook.Default(ctx, modulekit.Node{ID: 10, Type: "hero"}); err != nil || !settings.nodes[10] {
		t.Fatalf("default: %v %v", err, settings.nodes)
	}
}

// TestNodePageHook - a node that can have galleries gets them (the
// button's, the links' and the block's data); one that can't gets nothing.
func TestNodePageHook(t *testing.T) {
	ctx := context.Background()
	settings := newFakeSettings()
	settings.nodes[3] = true
	hook := nodePageHook(settings, fakeSummaries{{ID: 1, Items: GalleryItems{{}, {}}}}, nil)

	parts := hook(ctx, modulekit.Node{ID: 3, Type: "hero"})
	page, ok := parts.Data.(nodePage)
	if !ok || len(page.Galleries) != 1 {
		t.Errorf("enabled: %+v", parts)
	}
	if parts := hook(ctx, modulekit.Node{ID: 4, Type: "hero"}); parts.Data != nil {
		t.Errorf("disabled: %+v", parts)
	}
}
