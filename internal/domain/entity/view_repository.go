package entity

import (
	"context"
	"time"
)

// ViewRepository - persistence for views, displays, filters, sorts and
// fields; running them is ViewQuery's.
type ViewRepository interface {
	// Views - the admin list rows: lone displays (ViewID nil) and View
	// Defaults, not every display (see PublicPageViews).
	Views(ctx context.Context) (ViewDisplayDefs, error)
	// PublicPageViews - every non-Default page display across all Views
	// (Views alone misses page displays inside a group), enabled or not:
	// the sitemap lists the enabled ones, a menu link may point to them.
	PublicPageViews(ctx context.Context) (ViewDisplayDefs, error)
	// AttachmentDisplays - every enabled, non-Default
	// DisplayType==attachment display, ordered by AttachWeight.
	AttachmentDisplays(ctx context.Context) (ViewDisplayDefs, error)
	// TermDisplays - every enabled, non-Default DisplayType==term
	// display, ordered by AttachWeight (then id).
	TermDisplays(ctx context.Context) (ViewDisplayDefs, error)
	// UserTabDisplays - every enabled, non-Default user_tab display,
	// ordered by AttachWeight (then id).
	UserTabDisplays(ctx context.Context) (ViewDisplayDefs, error)
	ViewByID(ctx context.Context, id int) (*ViewDisplayDef, error)
	ViewByMachineName(ctx context.Context, machineName string) (*ViewDisplayDef, error)
	// ViewByURLPath - public resolution of a page display by path.
	// Excludes IsDefault displays (defense-in-depth: they have no URL).
	ViewByURLPath(ctx context.Context, urlPath string) (*ViewDisplayDef, error)
	// PageDisplaysWithArgument - non-Default page displays whose URLPath
	// has an argument segment ("%"), e.g. "articles/%".
	PageDisplaysWithArgument(ctx context.Context) (ViewDisplayDefs, error)
	// ViewByURLPathAny - like ViewByURLPath but including Default
	// displays, for uniqueness checks: the DB UNIQUE constraint on
	// url_path covers every row regardless of is_default.
	ViewByURLPathAny(ctx context.Context, urlPath string) (*ViewDisplayDef, error)
	CreateView(ctx context.Context, v ViewDisplayDef) (*ViewDisplayDef, error)
	UpdateView(ctx context.Context, v ViewDisplayDef) (*ViewDisplayDef, error)
	DeleteView(ctx context.Context, id int) error
	// SetViewMachineName - renames a display's machine_name, bypassing
	// UpdateView's immutability. Only for createDisplay's
	// create-then-rename: the row is inserted with a placeholder (the
	// column is NOT NULL UNIQUE and the id is unknown before INSERT) and
	// renamed to "view_"+id in the same transaction.
	SetViewMachineName(ctx context.Context, id int, machineName string) (*ViewDisplayDef, error)
	// SetPublishedSnapshot - writes published_snapshot/published_at.
	// UpdateView never touches these columns, so its callers can't wipe
	// them. Called only by views.DefaultService.PublishDisplay.
	SetPublishedSnapshot(ctx context.Context, id int, snapshot string, publishedAt time.Time) (*ViewDisplayDef, error)

	// CreateViewContainer - creates a View (display group). Called only
	// by views.DefaultService.CloneDisplay.
	CreateViewContainer(ctx context.Context, v View) (*View, error)
	// SetViewContainerMachineName - the create-then-rename exception of
	// SetViewMachineName, for the container row.
	SetViewContainerMachineName(ctx context.Context, id int, machineName string) (*View, error)
	// ViewContainerByID - the View with the given id.
	ViewContainerByID(ctx context.Context, id int) (*View, error)
	// DeleteViewContainer - deletes a View by its own id. ON DELETE
	// CASCADE removes all its displays and their filters/sorts/fields,
	// so no orphaned Default is left behind.
	DeleteViewContainer(ctx context.Context, id int) error
	// DefaultDisplayOfView - the View's single IsDefault display (exists
	// once a View has been cloned). Used to resolve inherited sections.
	DefaultDisplayOfView(ctx context.Context, viewID int) (*ViewDisplayDef, error)
	// DisplaysOfView - every non-Default display of the View, for the
	// admin display tabs.
	DisplaysOfView(ctx context.Context, viewID int) (ViewDisplayDefs, error)

	FiltersByViewID(ctx context.Context, viewID int) (ViewFilterDefs, error)
	FilterByID(ctx context.Context, id int) (*ViewFilterDef, error)
	CreateFilter(ctx context.Context, f ViewFilterDef) (*ViewFilterDef, error)
	UpdateFilter(ctx context.Context, f ViewFilterDef) (*ViewFilterDef, error)
	DeleteFilter(ctx context.Context, id int) error

	SortsByViewID(ctx context.Context, viewID int) (ViewSortDefs, error)
	SortByID(ctx context.Context, id int) (*ViewSortDef, error)
	CreateSort(ctx context.Context, s ViewSortDef) (*ViewSortDef, error)
	// UpdateSort updates only Direction/SortOrder/Params; the target
	// (FieldStorageID/BuiltinField) is immutable - delete and re-add to
	// change it.
	UpdateSort(ctx context.Context, s ViewSortDef) (*ViewSortDef, error)
	DeleteSort(ctx context.Context, id int) error

	FieldsByViewID(ctx context.Context, viewID int) (ViewFieldDefs, error)
	ViewFieldByID(ctx context.Context, id int) (*ViewFieldDef, error)
	CreateViewField(ctx context.Context, f ViewFieldDef) (*ViewFieldDef, error)
	// UpdateViewField updates a field's display attributes; the target
	// (FieldStorageID/BuiltinField) is immutable - delete and re-add to
	// change it.
	UpdateViewField(ctx context.Context, f ViewFieldDef) (*ViewFieldDef, error)
	DeleteViewField(ctx context.Context, id int) error
}

// ViewQuery runs Views: the content and comment lists a display defines
// and its glossary bar. Separate from ViewRepository (the Views'
// definitions): the queries are an engine of their own.
type ViewQuery interface {
	// RunView executes a content view: each filter's value is
	// QueryParams[ExposedParam] for exposed filters (missing/empty = no
	// constraint) or Value otherwise; filters are AND-combined, sorted by
	// params.Sorts (empty = created_at desc) and paginated. Returns the
	// page's nodes and the total count. FieldStorages must contain every
	// field storage referenced by Filters/Sorts - RunView doesn't load
	// them itself.
	RunView(ctx context.Context, params ViewRunParams) (Nodes, int, error)

	// RunCommentView - RunView for a comment base entity display. Same
	// params; FieldStorages is always empty (comments have only builtin
	// filters/sorts).
	RunCommentView(ctx context.Context, params ViewRunParams) (Comments, int, error)

	// GlossaryForView computes per-letter counts for the glossary bar
	// using RunView's filter semantics (filters must exclude the
	// synthetic letter filter), grouped by the uppercased first rune of
	// the node title, or of the given text field when
	// glossaryFieldStorageID is set.
	GlossaryForView(ctx context.Context, filters ViewFilterDefs, moduleFilters []ViewRunModuleFilter, fieldStorages map[int]FieldStorageDef, glossaryFieldStorageID *int, queryParams map[string]string) (GlossaryData, error)
}

// ViewRunParams - input for ViewQuery.RunView, assembled by the
// service layer (which resolves FieldStorages), so RunView stays a pure
// query executor.
type ViewRunParams struct {
	Filters ViewFilterDefs
	// FieldStorages - by FieldStorageDef.ID, covering every filter and
	// sort that targets a generic field.
	FieldStorages map[int]FieldStorageDef
	// Sorts - in application order (Sorts[0] is the primary key). Empty
	// means "created_at desc".
	Sorts []ViewRunSort
	// QueryParams - the current request's query string, for exposed
	// filters. Never mutated.
	QueryParams map[string]string
	PageSize    int
	Page        int // 1-based
	// Skip - the first Skip items of the result are left out: page 1
	// starts after them, and the total counts only the rest
	// (ViewDisplayDef.SkipItems).
	Skip int
	// ContextCreatedByID/ContextNodeID - the node being viewed:
	// ContextCreatedByID feeds Contextual filters; ContextNodeID (when
	// >0) always excludes the current node from the results. 0 = no
	// context, in which case Contextual filters yield an empty result,
	// not an unfiltered one.
	ContextCreatedByID int
	ContextNodeID      int
	// ContextTermID - the taxonomy term whose page is being rendered,
	// for the has_current_term contextual filter (0 = no term context).
	ContextTermID int
	// ModuleFilters - the filters by modules' values (not in Filters),
	// AND-combined with them.
	ModuleFilters []ViewRunModuleFilter
}

// ViewRunModuleFilter - a filter by a module's value with the module's
// lookup, as RunView consumes it.
type ViewRunModuleFilter struct {
	Def    ViewFilterDef
	Filter ViewModuleFilter
}

// ViewRunSort - a ViewSortDef with its field storage resolved, as
// RunView consumes it.
type ViewRunSort struct {
	FieldStorage *FieldStorageDef // nil = sort by Builtin
	Builtin      ViewSortBuiltin
	Direction    string // "asc" | "desc"
	// Module - a module's criterion (Builtin is its key), with its
	// params resolved (ViewModuleParams).
	Module *ViewModuleSort
	Params map[string]string
}

// GlossaryItem - one letter of a View's glossary bar and how many
// results start with it.
type GlossaryItem struct {
	Letter string `json:"letter"`
	Count  int    `json:"count"`
}

// GlossaryData - a View's glossary bar: its letters with their counts.
type GlossaryData []GlossaryItem
