package service

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/mailsender"
)

type fakeUninstallModules struct {
	fakeModuleRepo
	deleted   []entity.PluginBlockID
	menuPaths []string
}

func (f *fakeUninstallModules) DeleteMenuLinks(_ context.Context, paths []string) error {
	f.menuPaths = append(f.menuPaths, paths...)
	return nil
}

func (f *fakeUninstallModules) DeletePluginPlacements(_ context.Context, ids []entity.PluginBlockID) error {
	f.deleted = append(f.deleted, ids...)
	return nil
}

type fakeRevoker struct{ revoked []string }

func (f *fakeRevoker) Revoke(_ context.Context, keys []string) error {
	f.revoked = append(f.revoked, keys...)
	return nil
}

// TestModuleUninstall - only a disabled, installed module without content
// types or core data (or with its own uninstall step) is uninstalled; it
// loses its placements, the menu links to its pages, permissions and
// schema and stays switched off.
func TestModuleUninstall(t *testing.T) {
	// A module with pages, a block and a permission (none built in has all three).
	withExtraModules(t, entity.Module{Name: "widgets", Paths: []string{"/widgets"},
		PluginBlocks: []entity.ModulePluginBlock{{ID: "random_widget"}},
		Permissions:  []entity.ModulePermission{{Key: "manage widgets"}}})
	withModules(t, map[string]bool{"widgets": false, "polls": false, "galleries": false, "feeds": false})
	installed := map[string]bool{"widgets": true, "polls": true, "galleries": true, "error_reports": true}
	repo := &fakeUninstallModules{fakeModuleRepo: fakeModuleRepo{states: map[string]bool{}}}
	perms := &fakeRevoker{}
	var dropped []string
	u := ModuleUninstall{
		Installed: func(_ context.Context, m string) (bool, error) { return installed[m], nil },
		Steps:     map[string]func(context.Context) error{"galleries": func(context.Context) error { return nil }},
		DropSchema: func(_ context.Context, m string) error {
			dropped = append(dropped, m)
			installed[m] = false
			return nil
		},
		Modules:     repo,
		Permissions: perms,
	}
	ctx := context.Background()
	for name, want := range map[string]error{
		"nope":          ErrModuleUnknown,
		"error_reports": ErrModuleEnabled,
		"feeds":         ErrModuleNotInstalled,
		"polls":         ErrModuleNeedsOwnUninstall,
		"galleries":     nil,
		"widgets":       nil,
	} {
		if err := u.Check(ctx, name); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", name, err, want)
		}
	}

	if err := u.Run(ctx, "polls"); !errors.Is(err, ErrModuleNeedsOwnUninstall) || len(dropped) != 0 {
		t.Fatalf("polls run: %v, dropped %v", err, dropped)
	}
	if err := u.Run(ctx, "widgets"); err != nil {
		t.Fatal(err)
	}
	widgets, _ := entity.ModuleByName("widgets")
	blocks := make([]entity.PluginBlockID, len(widgets.PluginBlocks))
	for i, b := range widgets.PluginBlocks {
		blocks[i] = b.ID
	}
	if !slices.Equal(repo.menuPaths, widgets.Paths) || len(widgets.Paths) == 0 {
		t.Errorf("menu links to %v, want the module's paths %v", repo.menuPaths, widgets.Paths)
	}
	if !slices.Equal(dropped, []string{"widgets"}) || !slices.Equal(repo.deleted, blocks) ||
		!slices.Equal(perms.revoked, widgets.PermissionKeys()) || repo.states["widgets"] {
		t.Errorf("dropped %v, placements %v, permissions %v, states %v", dropped, repo.deleted, perms.revoked, repo.states)
	}
	if ModuleEnabled("widgets") {
		t.Error("widgets must stay switched off")
	}
	if err := u.Check(ctx, "widgets"); !errors.Is(err, ErrModuleNotInstalled) {
		t.Errorf("after uninstall: %v", err)
	}
}

type fakeUninstallStore struct {
	nodes      []int
	files      []int
	settings   []string
	viewPrefix string
	// mailTransports - the ways of sending whose settings were deleted.
	mailTransports []string
	textFilters    []string
	formatters     []string
	columns        [][2]string
	log            *[]string
}

func (f *fakeUninstallStore) NodeIDsOfTypes(_ context.Context, _ []string, limit int) ([]int, error) {
	if len(f.nodes) < limit {
		return slices.Clone(f.nodes), nil
	}
	return slices.Clone(f.nodes[:limit]), nil
}

func (f *fakeUninstallStore) FileIDs(_ context.Context, columns [][2]string) ([]int, error) {
	f.columns = columns
	*f.log = append(*f.log, "files")
	return f.files, nil
}

func (f *fakeUninstallStore) DeleteNodeTypeSettings(_ context.Context, types []string) error {
	f.settings = types
	return nil
}

func (f *fakeUninstallStore) DeleteMailTransportSettings(_ context.Context, transports []string) error {
	f.mailTransports = transports
	return nil
}

func (f *fakeUninstallStore) DeleteViewCriteria(_ context.Context, prefix string) error {
	f.viewPrefix = prefix
	return nil
}

func (f *fakeUninstallStore) RemoveTextFilters(_ context.Context, names []string) (map[int][]string, error) {
	f.textFilters = names
	return map[int][]string{3: {"kept"}}, nil
}

func (f *fakeUninstallStore) ClearFieldFormatters(_ context.Context, names []string) error {
	f.formatters = names
	return nil
}

type fakeFileCleaner struct{ got []int }

func (f *fakeFileCleaner) NodeFileIDs(context.Context, []int) ([]int, error) { return nil, nil }
func (f *fakeFileCleaner) DeleteUnreferencedFiles(_ context.Context, ids []int) ([]string, error) {
	f.got = ids
	return []string{"public://a.jpg"}, nil
}

type fakeStorage struct{ removed []string }

func (f *fakeStorage) RemoveStoredFile(_ context.Context, uri string) error {
	f.removed = append(f.removed, uri)
	return nil
}
func (f *fakeStorage) RemoveUserDir(context.Context, int) error { return nil }

// TestModuleUninstallContent - a module with content is uninstalled only
// after DeleteContent (in batches, stopping at the deadline); then Run
// revokes its content types' permissions too, removes their settings, runs
// its own step before the tables go and the files nothing else uses after.
func TestModuleUninstallContent(t *testing.T) {
	withModules(t, map[string]bool{"polls": false})
	withTextFilters(t, map[int][]string{3: {"poll_results", "kept"}}, TextFilter{Name: "poll_results", Module: "polls"}, TextFilter{Name: "kept", Module: "other"})
	withFieldFormatters(t, FieldFormatter{Name: "poll_link", Module: "polls", FieldType: entity.FieldTypeLink}, FieldFormatter{Name: "other_link", Module: "other", FieldType: entity.FieldTypeLink})
	RegisterMailTransport(MailTransport{Name: "poll_mail", Module: "polls", Title: "Poll mail", New: func(map[string]string, string, time.Duration) (mailsender.EmailSender, error) { return nil, nil }})
	t.Cleanup(func() { UnregisterMailTransport("poll_mail") })
	var log []string
	installed := map[string]bool{"polls": true}
	store := &fakeUninstallStore{nodes: []int{1, 2, 3}, files: []int{7, 8}, log: &log}
	repo := &fakeUninstallModules{fakeModuleRepo: fakeModuleRepo{states: map[string]bool{}}}
	perms := &fakeRevoker{}
	cleaner, storage := &fakeFileCleaner{}, &fakeStorage{}
	u := ModuleUninstall{
		Installed: func(_ context.Context, m string) (bool, error) { return installed[m], nil },
		DropSchema: func(context.Context, string) error {
			log = append(log, "drop")
			installed["polls"] = false
			return nil
		},
		Modules:     repo,
		Permissions: perms,
		Store:       store,
		DeleteNode: func(_ context.Context, id int) error {
			store.nodes = slices.DeleteFunc(store.nodes, func(n int) bool { return n == id })
			return nil
		},
		FileColumns: func(string) [][2]string { return [][2]string{{"t", "fid"}} },
		Files:       cleaner,
		Storage:     storage,
		Steps: map[string]func(context.Context) error{"polls": func(context.Context) error {
			log = append(log, "step")
			return nil
		}},
	}
	ctx := context.Background()
	if err := u.Check(ctx, "polls"); !errors.Is(err, ErrModuleHasContent) {
		t.Fatalf("with content: %v", err)
	}
	if n, more, err := u.DeleteContent(ctx, "polls", time.Now().Add(-time.Second)); err != nil || n != 0 || !more {
		t.Fatalf("past deadline: %d %v %v", n, more, err)
	}
	if n, more, err := u.DeleteContent(ctx, "polls", time.Now().Add(time.Minute)); err != nil || n != 3 || more || len(store.nodes) != 0 {
		t.Fatalf("delete content: %d %v %v, left %v", n, more, err, store.nodes)
	}
	if err := u.Run(ctx, "polls"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(perms.revoked, PermCreate("poll")) || !slices.Contains(perms.revoked, PermDeleteAny("poll")) {
		t.Errorf("revoked %v", perms.revoked)
	}
	if !slices.Equal(store.settings, []string{"poll"}) || !slices.Equal(log, []string{"files", "step", "drop"}) {
		t.Errorf("settings %v, order %v", store.settings, log)
	}
	// Its ways of sending email's settings.
	if !slices.Equal(store.mailTransports, []string{"poll_mail"}) {
		t.Errorf("mail transports' settings deleted %v", store.mailTransports)
	}
	if store.viewPrefix != "polls." {
		t.Errorf("View criteria removed by prefix %q, want %q", store.viewPrefix, "polls.")
	}
	// Its text filters out of the formats, in the store and in memory.
	if !slices.Equal(store.textFilters, []string{"poll_results"}) || !slices.Equal(formatFilters[3], []string{"kept"}) {
		t.Errorf("text filters removed %v, format 3 left %v", store.textFilters, formatFilters[3])
	}
	// Its field formatters out of the fields' settings.
	if !slices.Equal(store.formatters, []string{"poll_link"}) {
		t.Errorf("field formatters cleared %v", store.formatters)
	}
	if !slices.Equal(cleaner.got, []int{7, 8}) || !slices.Equal(storage.removed, []string{"public://a.jpg"}) {
		t.Errorf("files %v, removed %v", cleaner.got, storage.removed)
	}
}
