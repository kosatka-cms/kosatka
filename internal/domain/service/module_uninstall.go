package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Why a module can't be uninstalled now.
var (
	ErrModuleUnknown           = i18n.Errorf("Unknown module.")
	ErrModuleEnabled           = i18n.Errorf("Switch the module off first.")
	ErrModuleNotInstalled      = i18n.Errorf("The module is not installed.")
	ErrModuleRequired          = i18n.Errorf("An installed module requires it: uninstall that one first.")
	ErrModuleNeedsOwnUninstall = i18n.Errorf("The module keeps data in core tables, and it has no uninstall step for them yet.")
	ErrModuleHasContent        = i18n.Errorf("Delete the module's content first.")
)

// moduleContentBatch - node IDs fetched per query while deleting a
// module's content.
const moduleContentBatch = 50

// ModuleUninstallStore - the core tables' side of an uninstall.
type ModuleUninstallStore interface {
	// NodeIDsOfTypes - up to limit IDs of nodes of the types.
	NodeIDsOfTypes(ctx context.Context, types []string, limit int) ([]int, error)
	// FileIDs - the files the module tables' columns reference.
	FileIDs(ctx context.Context, columns [][2]string) ([]int, error)
	// DeleteNodeTypeSettings - the search, SEO, breadcrumb and block
	// visibility settings of the content types.
	DeleteNodeTypeSettings(ctx context.Context, types []string) error
	// DeleteViewCriteria - the Views' sort criteria, filters and fields
	// whose key starts with prefix (a module's, views.ModuleKeyPrefix).
	DeleteViewCriteria(ctx context.Context, prefix string) error
	// RemoveTextFilters takes the text filters out of the text formats
	// that have them on; returns those formats' filters left, by ID.
	RemoveTextFilters(ctx context.Context, names []string) (map[int][]string, error)
	// ClearFieldFormatters makes the fields shown by these formatters (on
	// the detail page, in Views) show as the core shows them.
	ClearFieldFormatters(ctx context.Context, names []string) error
}

// ModuleUninstall removes a disabled module completely. Its content (entity.Module.NodeTypes) is deleted first, as a
// separate action (DeleteContent). Then Run removes, from entity.Module and
// the schema: its plugin blocks' placements, the menu links to its pages
// (Module.Paths), its permissions (and those of
// its content types) in roles, its content types' settings, its criteria
// in Views (views.AddModuleSort…), its text filters in the text formats
// (AddTextFilter), its field formatters in the fields' settings
// (AddFieldFormatter), its own steps
// (Steps), its tables and
// recorded migrations, and the files only its tables used. The module
// stays switched off and not installed; enabling it again installs it from
// scratch.
type ModuleUninstall struct {
	// Installed - whether the module's migration set is recorded.
	Installed func(ctx context.Context, module string) (bool, error)
	// DropSchema drops the module's tables and forgets its migrations.
	DropSchema  func(ctx context.Context, module string) error
	Modules     entity.ModuleRepository
	Permissions interface {
		Revoke(ctx context.Context, keys []string) error
	}

	// Store, DeleteNode (NodeDeleteService.Delete), FileColumns
	// (migrations.FileColumns), Files and Storage: the module's content,
	// settings and files; nil Store - none of that is touched.
	Store       ModuleUninstallStore
	DeleteNode  func(ctx context.Context, nodeID int) error
	FileColumns func(module string) [][2]string
	Files       files.NodeFileCleaner
	Storage     files.StoredFileRemover
	// Steps - a module's own uninstall step that needs the application
	// (a vocabulary through the taxonomy), run before its tables go.
	Steps  map[string]func(ctx context.Context) error
	Logger *slog.Logger
}

func (u ModuleUninstall) logger() *slog.Logger {
	if u.Logger != nil {
		return u.Logger
	}
	return slog.Default()
}

func moduleNodeTypes(m entity.Module) []string {
	types := make([]string, len(m.NodeTypes))
	for i, nt := range m.NodeTypes {
		types[i] = nt.Name
	}
	return types
}

// checkState - the module is known, disabled, installed, and no installed
// module requires it.
func (u ModuleUninstall) checkState(ctx context.Context, name string) (entity.Module, error) {
	m, ok := entity.ModuleByName(name)
	if !ok {
		return m, ErrModuleUnknown
	}
	if ModuleEnabled(name) {
		return m, ErrModuleEnabled
	}
	installed, err := u.Installed(ctx, name)
	if err != nil {
		return m, err
	}
	if !installed {
		return m, ErrModuleNotInstalled
	}
	for _, other := range entity.ModulesRequiring(name) {
		installed, err := u.Installed(ctx, other)
		if err != nil {
			return m, err
		}
		if installed {
			return m, ErrModuleRequired
		}
	}
	return m, nil
}

// Check - why the module can't be uninstalled now (nil - it can).
func (u ModuleUninstall) Check(ctx context.Context, name string) error {
	m, err := u.checkState(ctx, name)
	if err != nil {
		return err
	}
	if len(m.CoreData) > 0 && u.Steps[name] == nil {
		return ErrModuleNeedsOwnUninstall
	}
	if len(m.NodeTypes) > 0 {
		if u.Store == nil {
			return ErrModuleNeedsOwnUninstall
		}
		ids, err := u.Store.NodeIDsOfTypes(ctx, moduleNodeTypes(m), 1)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			return ErrModuleHasContent
		}
	}
	return nil
}

// DeleteContent deletes the module's content (its NodeTypes' nodes, each
// through the regular node delete) until it is gone or until passes;
// returns how many were deleted and whether some are left.
func (u ModuleUninstall) DeleteContent(ctx context.Context, name string, until time.Time) (int, bool, error) {
	m, err := u.checkState(ctx, name)
	if err != nil {
		return 0, false, err
	}
	types := moduleNodeTypes(m)
	if len(types) == 0 || u.Store == nil || u.DeleteNode == nil {
		return 0, false, nil
	}
	deleted := 0
	for {
		ids, err := u.Store.NodeIDsOfTypes(ctx, types, moduleContentBatch)
		if err != nil || len(ids) == 0 {
			return deleted, false, err
		}
		for _, id := range ids {
			if time.Now().After(until) {
				return deleted, true, nil
			}
			if err := u.DeleteNode(ctx, id); err != nil {
				return deleted, true, err
			}
			deleted++
		}
	}
}

// Run uninstalls the module (after Check). A failure leaves the rest for
// a second run: every step can run again.
func (u ModuleUninstall) Run(ctx context.Context, name string) error {
	if err := u.Check(ctx, name); err != nil {
		return err
	}
	m, _ := entity.ModuleByName(name)
	types := moduleNodeTypes(m)
	blocks := make([]entity.PluginBlockID, len(m.PluginBlocks))
	for i, b := range m.PluginBlocks {
		blocks[i] = b.ID
	}
	if err := u.Modules.DeletePluginPlacements(ctx, blocks); err != nil {
		return err
	}
	if err := u.Modules.DeleteMenuLinks(ctx, m.Paths); err != nil {
		return err
	}
	keys := m.PermissionKeys()
	for _, t := range types {
		keys = append(keys, PermCreate(t), PermEditOwn(t), PermEditAny(t), PermDeleteAny(t))
	}
	if err := u.Permissions.Revoke(ctx, keys); err != nil {
		return err
	}
	var fileIDs []int
	if u.Store != nil {
		if err := u.Store.DeleteNodeTypeSettings(ctx, types); err != nil {
			return err
		}
		if err := u.Store.DeleteViewCriteria(ctx, views.ModuleKeyPrefix(name)); err != nil {
			return err
		}
		if filters := moduleTextFilters(name); len(filters) > 0 {
			left, err := u.Store.RemoveTextFilters(ctx, filters)
			if err != nil {
				return err
			}
			for id, names := range left {
				SetTextFormatFilters(id, names)
			}
		}
		if formatters := moduleFieldFormatters(name); len(formatters) > 0 {
			if err := u.Store.ClearFieldFormatters(ctx, formatters); err != nil {
				return err
			}
		}
		if u.FileColumns != nil {
			ids, err := u.Store.FileIDs(ctx, u.FileColumns(name))
			if err != nil {
				return err
			}
			fileIDs = ids
		}
	}
	if step := u.Steps[name]; step != nil {
		if err := step(ctx); err != nil {
			return err
		}
	}
	if err := u.DropSchema(ctx, name); err != nil {
		return err
	}
	// The module's tables are gone: their files nothing else uses go too.
	if u.Files != nil && u.Storage != nil {
		files.RemoveUnreferenced(ctx, u.Files, u.Storage, u.logger(), fileIDs)
	}
	// The row stays, switched off: a module without one counts as enabled.
	if err := u.Modules.SetEnabled(ctx, name, false); err != nil {
		return err
	}
	SetModules(map[string]bool{name: false})
	audit.Record(ctx, audit.Action{Type: audit.ModuleUninstall, Object: audit.Object{Kind: audit.KindModule, ID: name, Label: i18n.T(ctx, m.Label)}})
	return nil
}
