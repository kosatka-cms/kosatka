package entity

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Modules are declared by their own packages (internal/modules/<name>/
// manifest: a Module value) and
// registered at startup (RegisterModule, internal/modules/manifests): the
// core knows them only through the registry below. A disabled module
// keeps its data; its routes answer 404 and its links, blocks,
// permissions and node types are hidden.

// Feature switches: smaller engine features turned on and off in their
// own admin sections (not on the Modules page). Their states are stored
// with the modules' (ModuleRepository); a missing state means enabled.
const (
	FeatureBackToTop = "back_to_top"
	FeatureSearch    = "search"
)

// Module describes a module - everything it owns, in one place,
// declared by the module's manifest package:
// what a disabled module hides is derived from here (ModuleOfNodeType,
// ModuleOfPluginBlock, ModuleOfPermission). Its code wires its routes
// (router.ModuleMux in internal/app/routes.go).
type Module struct {
	Name        string
	Label       string
	Description string
	// Category - its group on the Modules page, one of ModuleCategories
	// ("" or another - ModuleCategoryOther).
	Category string
	// External - the module comes from its own repository (registered
	// with modulekit.Register), not with the engine; set at registration.
	External bool
	// Requires - modules this one needs: it can't be enabled while one of
	// them is off, and they can't be switched off while it is on.
	Requires []string
	// NodeTypes - content types the module provides (no content_types
	// row of their own); they don't open, get created or edited, or show
	// in the sitemap and search while it is off.
	NodeTypes []ModuleNodeType
	// PluginBlocks - plugin blocks it provides (PluginBlocks()).
	PluginBlocks []ModulePluginBlock
	// Permissions - permissions it adds to the matrix.
	Permissions []ModulePermission
	// AdminMenu - its pages in the admin toolbar (AdminMenu()).
	AdminMenu []ModuleMenuItem
	// CaptchaForms - its forms that can show the CAPTCHA
	// (CaptchaFormNames()).
	CaptchaForms []ModuleCaptchaForm
	// FileSources - its texts that can link to users' files (the files
	// manager's "used in").
	FileSources []ModuleFileSource
	// NodeFiles - its tables of files attached to nodes (e.g. gallery
	// images): deleting a node deletes those files nothing else uses.
	NodeFiles []ModuleNodeFiles
	// MaintenancePaths - path prefixes of its pages that stay open while
	// the site is in maintenance mode (e.g. a fragment the public page
	// shell loads).
	MaintenancePaths []string
	// Paths - the addresses of its own pages ("/quotes": that path and the
	// ones under it): uninstalling removes the menu links to them.
	Paths []string
	// RobotsDisallow - paths of its pages crawlers should skip (forms,
	// actions), added to robots.txt while it is enabled (service.RobotsTxt);
	// "*" matches any part of a path.
	RobotsDisallow []string
	// LabelColumns - its tables' columns of labels its install step writes
	// in English (marked with i18n.N): app install puts them in the site's
	// language.
	LabelColumns []ModuleLabelColumn
	// Tables - every table the module owns: its schema migrations are its
	// own set (created when the module is installed); every other table is
	// the core's. A module's table may reference core tables, never the
	// other way round.
	Tables []string
	// Data - those of Tables the Modules page counts (a disabled module
	// keeps them).
	Data []ModuleData
	// OffByDefault - the module is off until enabled on the Modules page
	// (no state row - off): a module added to the engine after sites were
	// set up doesn't switch itself on there; enabling it installs it.
	OffByDefault bool
	// InstallByDefault - a new site has the module: app install installs
	// and enables it unless its -modules says otherwise. The others a new
	// site gets
	// not installed and off; enabling one on the Modules page installs it.
	InstallByDefault bool
	// CoreData - what the module keeps in core tables besides what the
	// fields above describe (a core column, a vocabulary), as the uninstall
	// page names it. A module with CoreData is uninstalled only with its
	// own uninstall step that removes them; everything else - its
	// NodeTypes' content and settings, files only its tables use -
	// an uninstall derives from this struct and the schema.
	CoreData []string
	// CSS/JS - the module's own assets, internal/modules/<Name>/static/
	// <file>.css and .js (served minified at /static/modules/<Name>/). A
	// page includes them while the module is enabled and only if it shows
	// something of the module: the module's templates call
	// {{attach "<Name>"}}, its plugin blocks attach it themselves (see
	// service.resolveModuleAssets). A theme styles the module's blocks in
	// its own CSS, loaded after these.
	CSS []string
	JS  []string
}

// Modules - the registered modules, in the order of the Modules page
// (RegisterModule; filled at startup).
var Modules []Module

// RegisterModule adds a module to the registry (at startup, before
// anything reads it); a second module with the same name panics.
func RegisterModule(m Module) {
	for _, have := range Modules {
		if have.Name == m.Name {
			panic("entity: module " + m.Name + " registered twice")
		}
	}
	Modules = append(Modules, m)
}

// ModulePermission - a permission a module adds: its key and its label
// (a translation source).
type ModulePermission struct {
	Key   string
	Label string
}

// ModulePluginBlock - a plugin block a module provides and its label in
// the block picker (a translation source). Region - where installing the
// module places it: that region of the site's active theme, if the theme
// has it (migrations.PlaceModuleBlocks); "" - the site places it on the
// Blocks page.
type ModulePluginBlock struct {
	ID     PluginBlockID
	Label  string
	Region Region
}

// ModuleMenuItem - a module's page in the admin toolbar: Section - the
// label (translation source) of the section it joins (AdminSection*).
// Item.Module is set to the module by AdminMenu().
type ModuleMenuItem struct {
	Section string
	Item    AdminMenuItem
}

// ModuleCaptchaForm - a module's form that can show the CAPTCHA: its key
// (CaptchaSettings.Forms), its label on the settings page (translation
// source) and whether it shows it until the settings say otherwise.
type ModuleCaptchaForm struct {
	Key     string
	Label   string
	Default bool
}

// ModuleFileSource - a module's texts that can link to users' files: the
// HTML Columns of Table, recorded against IDColumn. Label names a place of
// use ("Quote "@title"", a translation source). NodeBacked - IDColumn is a
// node's ID: the title and link are the node's; otherwise the title is the
// start of TitleColumn and the link EditPath + the ID - without
// TitleColumn neither (a private text, such as a private message: Label
// alone).
type ModuleFileSource struct {
	Source      string
	Table       string
	IDColumn    string
	Columns     []string
	Label       string
	NodeBacked  bool
	TitleColumn string
	EditPath    string
}

// ModuleNodeFiles - a module's table of files attached to nodes:
// Table.FileColumn, whose node is Table.NodeColumn, or with Via set
// Via.NodeColumn of the Via row Table.ViaColumn points to (Via.id).
type ModuleNodeFiles struct {
	Table      string
	FileColumn string
	NodeColumn string
	Via        string
	ViaColumn  string
}

// ModuleLabelColumn - a column of labels in a module's table (rows keyed
// by an id column).
type ModuleLabelColumn struct {
	Table  string
	Column string
}

// PermissionKeys - the keys of the module's permissions.
func (m Module) PermissionKeys() []string {
	out := make([]string, len(m.Permissions))
	for i, p := range m.Permissions {
		out[i] = p.Key
	}
	return out
}

// The groups of the Modules page (Module.Category), in its order
// (ModuleCategories): the engine's set, a module picks one of them.
const (
	ModuleCategoryContent        = "content"
	ModuleCategoryMedia          = "media"
	ModuleCategoryCommunity      = "community"
	ModuleCategoryAdministration = "administration"
	ModuleCategoryIntegrations   = "integrations"
	ModuleCategoryOther          = "other"
)

// ModuleCategory - a group of the Modules page: its name and label.
type ModuleCategory struct {
	Name  string
	Label string
}

// moduleCategories - the groups, in the page's order, "Other" last.
var moduleCategories = []ModuleCategory{
	{ModuleCategoryContent, i18n.NC("module category", "Content")},
	{ModuleCategoryMedia, i18n.NC("module category", "Media")},
	{ModuleCategoryCommunity, i18n.NC("module category", "Community")},
	{ModuleCategoryAdministration, i18n.NC("module category", "Administration")},
	{ModuleCategoryIntegrations, i18n.NC("module category", "Integrations")},
	{ModuleCategoryOther, i18n.NC("module category", "Other")},
}

// ModuleCategoryContext - the translation context of the categories'
// labels.
const ModuleCategoryContext = "module category"

// ModuleCategories - the groups of the Modules page, in its order.
func ModuleCategories() []ModuleCategory { return slices.Clone(moduleCategories) }

// CategoryName - the module's group: its Category when the engine knows
// it, else ModuleCategoryOther.
func (m Module) CategoryName() string {
	for _, c := range moduleCategories {
		if c.Name == m.Category {
			return c.Name
		}
	}
	return ModuleCategoryOther
}

// ModuleByName - the built-in module with the name, false if unknown.
func ModuleByName(name string) (Module, bool) {
	for _, m := range Modules {
		if m.Name == name {
			return m, true
		}
	}
	return Module{}, false
}

// ModuleNodeType - a node type a module provides.
type ModuleNodeType struct {
	Name  string
	Label string
	// CreatePath - its standalone create page; empty when it is created
	// elsewhere (a forum topic inside a forum section).
	CreatePath string
	// PathPrefix - prepended to a node's slug that doesn't carry it yet
	// (NodeRelPath): a forum topic's bare slug lives under "forum/".
	PathPrefix string
	// OwnDiscussion - its comments are a discussion of their own (a forum
	// topic's replies), kept out of the site's latest comments.
	OwnDiscussion bool
	// EditPath / DeletePath - its own edit and delete pages, "{id}" the
	// node's ID ("" - the core's /node/{id}/edit and /delete).
	EditPath   string
	DeletePath string
}

// ModuleData - a table of a module's data, as the Modules page names it.
type ModuleData struct {
	Label string
	Table string
}

// PluginPlacementRef - where a plugin block is placed.
type PluginPlacementRef struct {
	PluginID PluginBlockID
	Theme    string
	Region   Region
}

// FeaturePluginBlocks - plugin blocks provided by feature switches (not
// modules).
var FeaturePluginBlocks = map[PluginBlockID]string{
	PluginBlockSearchForm: FeatureSearch,
}

// ModuleOfNodeType - the module that provides the node type.
func ModuleOfNodeType(nodeType string) (string, bool) {
	for _, m := range Modules {
		for _, t := range m.NodeTypes {
			if t.Name == nodeType {
				return m.Name, true
			}
		}
	}
	return "", false
}

// ModuleOfPluginBlock - the module or feature switch that provides the
// plugin block.
func ModuleOfPluginBlock(id PluginBlockID) (string, bool) {
	for _, m := range Modules {
		for _, b := range m.PluginBlocks {
			if b.ID == id {
				return m.Name, true
			}
		}
	}
	name, ok := FeaturePluginBlocks[id]
	return name, ok
}

// ModuleOfTable - the module that owns the table ("" for a core table).
func ModuleOfTable(table string) string {
	for _, m := range Modules {
		for _, t := range m.Tables {
			if t == table {
				return m.Name
			}
		}
	}
	return ""
}

// ModuleOfPermission - the module that adds the permission.
func ModuleOfPermission(key string) (string, bool) {
	for _, m := range Modules {
		for _, p := range m.Permissions {
			if p.Key == key {
				return m.Name, true
			}
		}
	}
	return "", false
}

// ModuleNodeTypes - every module-provided node type, in module order.
func ModuleNodeTypes() []string {
	var out []string
	for _, m := range Modules {
		for _, t := range m.NodeTypes {
			out = append(out, t.Name)
		}
	}
	return out
}

// ModuleNodeTypeLabels - labels of the modules' node types (they have no
// content_types row; every other type's label is its
// ContentTypeDef.Label).
func ModuleNodeTypeLabels() map[string]string {
	return moduleNodeTypeMap(func(t ModuleNodeType) string { return t.Label })
}

// NodeTypesByLabel - the node types of labels (machine name → label,
// ContentTypeService.NodeTypeLabels) sorted by label: a "Type" filter's
// options.
func NodeTypesByLabel(labels map[string]string) []ContentType {
	types := make([]ContentType, 0, len(labels))
	for t := range labels {
		types = append(types, ContentType(t))
	}
	slices.SortFunc(types, func(a, b ContentType) int {
		return cmp.Or(cmp.Compare(labels[string(a)], labels[string(b)]), cmp.Compare(a, b))
	})
	return types
}

// ModuleNodeTypeCreatePaths - standalone create pages of the modules'
// node types (types without one are left out).
func ModuleNodeTypeCreatePaths() map[string]string {
	return moduleNodeTypeMap(func(t ModuleNodeType) string { return t.CreatePath })
}

// OwnDiscussionNodeTypes - the modules' node types whose comments are a
// discussion of their own (ModuleNodeType.OwnDiscussion).
func OwnDiscussionNodeTypes() []string {
	var out []string
	for _, m := range Modules {
		for _, t := range m.NodeTypes {
			if t.OwnDiscussion {
				out = append(out, t.Name)
			}
		}
	}
	return out
}

// NodeEditPath / NodeDeletePath - a node's edit and delete pages: its
// module type's own (ModuleNodeType.EditPath/DeletePath) or the core's.
func NodeEditPath(nodeType string, id int) string {
	return nodePagePath(nodeType, id, func(t ModuleNodeType) string { return t.EditPath }, "/node/{id}/edit")
}

// NodeDeletePath - a node's delete page: its module type's own
// (ModuleNodeType.DeletePath) or the core's.
func NodeDeletePath(nodeType string, id int) string {
	return nodePagePath(nodeType, id, func(t ModuleNodeType) string { return t.DeletePath }, "/node/{id}/delete")
}

func nodePagePath(nodeType string, id int, own func(ModuleNodeType) string, core string) string {
	pattern := core
	for _, m := range Modules {
		for _, t := range m.NodeTypes {
			if t.Name == nodeType && own(t) != "" {
				pattern = own(t)
			}
		}
	}
	return strings.Replace(pattern, "{id}", strconv.Itoa(id), 1)
}

// ModuleNodeTypePathPrefix - the path prefix of a module's node type ("" -
// none, or not a module's type).
func ModuleNodeTypePathPrefix(nodeType string) string {
	for _, m := range Modules {
		for _, t := range m.NodeTypes {
			if t.Name == nodeType {
				return t.PathPrefix
			}
		}
	}
	return ""
}

func moduleNodeTypeMap(value func(ModuleNodeType) string) map[string]string {
	out := map[string]string{}
	for _, m := range Modules {
		for _, t := range m.NodeTypes {
			if v := value(t); v != "" {
				out[t.Name] = v
			}
		}
	}
	return out
}

// ModuleRequirementsMissing - for each module enabled in states, the
// modules it requires that are off (a name missing from states counts as
// enabled).
func ModuleRequirementsMissing(states map[string]bool) map[string][]string {
	on := func(name string) bool {
		enabled, ok := states[name]
		return !ok || enabled
	}
	out := map[string][]string{}
	for _, m := range Modules {
		if !on(m.Name) {
			continue
		}
		for _, req := range m.Requires {
			if !on(req) {
				out[m.Name] = append(out[m.Name], req)
			}
		}
	}
	return out
}

// ModulesRequiring - the modules that require name.
func ModulesRequiring(name string) []string {
	var out []string
	for _, m := range Modules {
		for _, req := range m.Requires {
			if req == name {
				out = append(out, m.Name)
			}
		}
	}
	return out
}

// ModuleRepository stores which modules and features are switched off; a
// name with no row is enabled.
type ModuleRepository interface {
	States(ctx context.Context) (map[string]bool, error)
	SetEnabled(ctx context.Context, name string, enabled bool) error
	// CountRows - rows in a module's data table (Module.Data).
	CountRows(ctx context.Context, table string) (int, error)
	// PluginPlacements - where the plugin blocks are placed, every theme.
	PluginPlacements(ctx context.Context, ids []PluginBlockID) ([]PluginPlacementRef, error)
	// DeletePluginPlacements removes the plugin blocks' placements, every
	// theme (a module's uninstall).
	DeletePluginPlacements(ctx context.Context, ids []PluginBlockID) error
	// DeleteMenuLinks removes the menu links to paths (Module.Paths: each
	// path and the ones under it), every menu; a removed link's children
	// take its place (a module's uninstall).
	DeleteMenuLinks(ctx context.Context, paths []string) error
}
