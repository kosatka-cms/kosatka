package views

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// Guard errors returned by Service. Callers should detect them with
// errors.Is.
var (
	ErrDuplicate = errors.New("view machine_name or url_path already taken")
	// ErrProfileTabPathInvalid - a user_tab display's path segment
	// isn't lowercase latin/digits/dashes, or is a built-in profile tab.
	ErrProfileTabPathInvalid = errors.New("invalid profile tab path")
	// ErrProfileTabPathTaken - another user_tab display has it.
	ErrProfileTabPathTaken = errors.New("profile tab path already taken")
	// ErrURLPathTaken - something else on the site answers the page's
	// URLPath (the engine, content, a term page, a module's page).
	ErrURLPathTaken = errors.New("url_path is answered by something else on the site")
	// ErrURLArgumentInvalid - a page URLPath with more than one "%"
	// argument, or "%" inside a segment.
	ErrURLArgumentInvalid = errors.New("url_path may have one whole \"%\" segment")
	// ErrThemeTemplateInvalid - ThemeTemplate isn't lowercase
	// latin/digits/underscores.
	ErrThemeTemplateInvalid = errors.New("invalid theme template name")
	// ErrFilterOperatorUnsupported - CreateFilter/UpdateFilter's
	// Operator isn't valid for the target field's FieldType (or
	// BuiltinField) - see validOperatorsFor's doc comment.
	ErrFilterOperatorUnsupported = errors.New("operator not supported for this field type")
	// ErrFilterFieldTypeUnsupported - CreateFilter/UpdateFilter
	// targets a field whose FieldType can't be filtered on (see
	// viewFieldTypes).
	ErrFilterFieldTypeUnsupported = errors.New("field type doesn't support view filters")
	// ErrSortFieldTypeUnsupported - CreateSort/UpdateSort targets a
	// field whose FieldType can't be sorted on (see viewFieldTypes), or
	// a BuiltinField not in sortBuiltinFields.
	ErrSortFieldTypeUnsupported = errors.New("field type can't be used to sort a view")
	// ErrSortExposedKeyInvalid - a criterion offered to visitors has an
	// ExposedKey that isn't 1-64 Latin letters, digits, "_" and "-".
	ErrSortExposedKeyInvalid = errors.New("invalid exposed sort key")
	// ErrSortExposedLabelRequired - a criterion offered to visitors has
	// no ExposedLabel: the choice would show an empty option.
	ErrSortExposedLabelRequired = errors.New("exposed sort requires a label")
	// ErrSortExposedKeyTaken - another criterion of the display offers
	// the same ExposedKey.
	ErrSortExposedKeyTaken = errors.New("exposed sort key already used in this display")
	// ErrFilterExposedParamRequired - Exposed=true without
	// ExposedParam: RunView would have no query parameter to read.
	ErrFilterExposedParamRequired = errors.New("exposed filter requires exposed_param")
	// ErrFilterExposedParamInvalid - an exposed filter's parameter isn't
	// lowercase latin letters, digits and _ from a letter, or is one a
	// View page reads itself (page, letter, sort).
	ErrFilterExposedParamInvalid = errors.New("invalid exposed_param")
	// ErrFilterExposedParamTaken - another exposed filter of the display
	// reads the parameter.
	ErrFilterExposedParamTaken = errors.New("exposed_param taken")
	// ErrGlossaryFieldTypeUnsupported - GlossaryFieldStorageID points
	// at a non-text field (see viewFieldTypes).
	ErrGlossaryFieldTypeUnsupported = errors.New("field type can't be used as a glossary target")
	// ErrCannotEditDefault - a clone, a section's override or revert, a
	// publish or a discard (editableDisplay) aimed at a Default display.
	// Its own filters, sorts, fields and settings are edited on its tab
	// as they are: it inherits from nothing and is never cloned.
	ErrCannotEditDefault = errors.New("the Default display can't be cloned, overridden, or reverted directly")
	// ErrNoPublishedSnapshot - DiscardDisplayChanges or
	// DecodePublishedSnapshot on a display that was never published.
	ErrNoPublishedSnapshot = errors.New("this display has never been published - nothing to discard")
	// ErrFilterInvalidRelativeDate - ValueIsRelative is set but Value
	// isn't a valid relative date expression; rejected at save time rather
	// than at RunView.
	ErrFilterInvalidRelativeDate = errors.New("invalid relative date expression")
	// ErrFilterContextualExposedConflict - a filter sets both
	// Contextual and Exposed; they are mutually exclusive value sources.
	ErrFilterContextualExposedConflict = errors.New("a filter can't be both contextual and exposed")
	// ErrFilterContextualUnsupported - Contextual is set on a target
	// that can't take its value from context, or is missing on one that
	// requires it (see validateFilter).
	ErrFilterContextualUnsupported = errors.New("contextual mode isn't valid for this filter: only reference fields, a user field's is_profile_user and the created_by, comment_created_by, similar_to_current_node, shares_terms_with_current and has_current_term builtins can be contextual, and the last three must be")
)

// Service - the Views: their displays, filters, sort criteria and fields,
// the displays' sections inherited from a View's Default, the
// publish/discard workflow, and running a display for its lists.
type Service interface {
	Views(ctx context.Context) (entity.ViewDisplayDefs, error)
	PublicPageViews(ctx context.Context) (entity.ViewDisplayDefs, error)
	// AttachmentDisplays - see entity.ViewRepository.AttachmentDisplays.
	AttachmentDisplays(ctx context.Context) (entity.ViewDisplayDefs, error)
	// TermDisplays - see entity.ViewRepository.TermDisplays.
	TermDisplays(ctx context.Context) (entity.ViewDisplayDefs, error)
	// UserTabDisplays - see entity.ViewRepository.UserTabDisplays.
	UserTabDisplays(ctx context.Context) (entity.ViewDisplayDefs, error)
	// RunViewForUser - a user_tab display's results for the profile of
	// userID: the created_by contextual filter resolves to userID.
	RunViewForUser(ctx context.Context, view entity.ViewDisplayDef, userID, page int) (entity.Nodes, int, error)
	ViewByID(ctx context.Context, id int) (*entity.ViewDisplayDef, error)
	ViewByMachineName(ctx context.Context, machineName string) (*entity.ViewDisplayDef, error)
	ViewByURLPath(ctx context.Context, urlPath string) (*entity.ViewDisplayDef, error)
	// ViewByURLArgument - the page display whose URLPath with a "%"
	// segment matches path, plus the argument value (e.g. "articles/%" for
	// /articles/<alias>).
	ViewByURLArgument(ctx context.Context, path string) (*entity.ViewDisplayDef, string, error)
	// RunViewForPage - RunView on a page whose URL argument is a user:
	// the created_by contextual filter resolves to contextUserID;
	// RunViewFromSnapshotForPage - the same on a published snapshot.
	RunViewForPage(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string, page, contextUserID int) (entity.Nodes, int, error)
	RunViewFromSnapshotForPage(ctx context.Context, view entity.ViewDisplayDef, snap entity.ViewSnapshot, queryParams map[string]string, page, contextUserID int) (entity.Nodes, int, error)
	CreateView(ctx context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error)
	UpdateView(ctx context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error)
	DeleteView(ctx context.Context, id int) error

	FiltersByViewID(ctx context.Context, viewID int) (entity.ViewFilterDefs, error)
	FilterByID(ctx context.Context, id int) (*entity.ViewFilterDef, error)
	CreateFilter(ctx context.Context, f entity.ViewFilterDef) (*entity.ViewFilterDef, error)
	UpdateFilter(ctx context.Context, f entity.ViewFilterDef) (*entity.ViewFilterDef, error)
	DeleteFilter(ctx context.Context, id int) error

	SortsByViewID(ctx context.Context, viewID int) (entity.ViewSortDefs, error)
	SortByID(ctx context.Context, id int) (*entity.ViewSortDef, error)
	CreateSort(ctx context.Context, sd entity.ViewSortDef) (*entity.ViewSortDef, error)
	UpdateSort(ctx context.Context, sd entity.ViewSortDef) (*entity.ViewSortDef, error)
	DeleteSort(ctx context.Context, id int) error

	FieldsByViewID(ctx context.Context, viewID int) (entity.ViewFieldDefs, error)
	ViewFieldByID(ctx context.Context, id int) (*entity.ViewFieldDef, error)
	CreateViewField(ctx context.Context, f entity.ViewFieldDef) (*entity.ViewFieldDef, error)
	UpdateViewField(ctx context.Context, f entity.ViewFieldDef) (*entity.ViewFieldDef, error)
	DeleteViewField(ctx context.Context, id int) error

	// RunView resolves its filters' field storages, then delegates the
	// query to entity.ViewQuery.RunView.
	RunView(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string, page int) (entity.Nodes, int, error)
	// RunViewWithContext - RunView for a contextual View block:
	// contextCreatedByID/contextNodeID come from the node being rendered;
	// they resolve Contextual filters and exclude that node itself. Without
	// contextual filters it behaves like RunView with no QueryParams.
	RunViewWithContext(ctx context.Context, view entity.ViewDisplayDef, contextCreatedByID, contextNodeID int) (entity.Nodes, int, error)
	// RunViewForTerm - RunView for a term display: termID feeds the
	// has_current_term contextual filter, page is 1-based. Returns the
	// page's nodes and the total for pagination.
	RunViewForTerm(ctx context.Context, view entity.ViewDisplayDef, termID, page int) (entity.Nodes, int, error)

	// RunCommentView - RunView for a base_entity=="comment" display
	// (entity.ViewBaseEntity). Comment views are always rendered live; there
	// is no snapshot variant.
	RunCommentView(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string, page int) (entity.Comments, int, error)
	// RunCommentViewForUser - a comment user_tab display's results for
	// the profile of userID (comment_created_by contextual = userID).
	RunCommentViewForUser(ctx context.Context, view entity.ViewDisplayDef, userID, page int) (entity.Comments, int, error)

	// Glossary computes a view's letter-index bar data
	// (entity.ViewDisplayDef.ShowGlossary).
	Glossary(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string) (entity.GlossaryData, error)
	// GlossaryFromSnapshot - Glossary on a published snapshot, as
	// RunViewFromSnapshotForPage runs the view on it.
	GlossaryFromSnapshot(ctx context.Context, view entity.ViewDisplayDef, snap entity.ViewSnapshot, queryParams map[string]string) (entity.GlossaryData, error)

	// PreviewView is RunView for the admin live preview. Filters/fields
	// are read from the DB (the admin UI saves them immediately), but page
	// settings (display style, grid columns, pagination, page size, sort)
	// come from settings, since they may still be unsaved in the settings
	// modal. settings.Page drives the preview's own AJAX pager.
	PreviewView(ctx context.Context, view entity.ViewDisplayDef, settings PreviewSettings) (entity.Nodes, int, error)

	// --- Displays grouped into Views ---

	// DefaultDisplayOfView/DisplaysOfView - repository pass-throughs used
	// for the admin tab strip and for rendering inherited sections.
	DefaultDisplayOfView(ctx context.Context, viewID int) (*entity.ViewDisplayDef, error)
	DisplaysOfView(ctx context.Context, viewID int) (entity.ViewDisplayDefs, error)
	// ViewContainerByID/DeleteViewContainer - delete the whole View with
	// all its displays, as opposed to DeleteView, which removes one display
	// (see entity.ViewRepository.DeleteViewContainer).
	ViewContainerByID(ctx context.Context, id int) (*entity.View, error)
	DeleteViewContainer(ctx context.Context, id int) error

	// EffectiveFiltersByView/EffectiveSortsByView/EffectiveFieldsByView/
	// EffectiveSettings - resolve a section from the display's own rows or
	// from its View's Default, depending on display.OverrideX (see
	// DefaultService.effectiveOwnerID). Rendering and admin code should
	// use these instead of reading the display's own rows directly.
	EffectiveFiltersByView(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewFilterDefs, error)
	EffectiveSortsByView(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewSortDefs, error)
	EffectiveFieldsByView(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewFieldDefs, error)
	EffectiveSettings(ctx context.Context, display entity.ViewDisplayDef) (entity.ViewDisplayDef, error)

	// CloneDisplay - the only way a View container is created: if source
	// has no group yet, creates one plus a Default seeded with source's
	// config. Returns a sibling copying source's current per-section state;
	// AddDisplay is the blank-slate counterpart.
	CloneDisplay(ctx context.Context, sourceDisplayID int, createdBy int) (*entity.ViewDisplayDef, error)

	// AddDisplay - adds a sibling to view's group that inherits every
	// section from Default (nothing copied). Requires an existing group;
	// CloneDisplay is what creates one.
	AddDisplay(ctx context.Context, viewID int, createdBy int) (*entity.ViewDisplayDef, error)

	// OverrideSettings/Filters/Sort/Fields - fork a section: copy its
	// current effective (possibly inherited) state into the display's own
	// rows/columns and set the OverrideX flag.
	// RevertSettings/Filters/Sort/Fields - drop the display's own copy and
	// clear the flag, inheriting from Default again.
	OverrideSettings(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error)
	RevertSettings(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error)
	OverrideFilters(ctx context.Context, displayID int) (entity.ViewFilterDefs, error)
	RevertFilters(ctx context.Context, displayID int) error
	OverrideSort(ctx context.Context, displayID int) (entity.ViewSortDefs, error)
	RevertSort(ctx context.Context, displayID int) error
	OverrideFields(ctx context.Context, displayID int) (entity.ViewFieldDefs, error)
	RevertFields(ctx context.Context, displayID int) error

	// PublishDisplay/DiscardDisplayChanges/HasUnpublishedChanges/
	// DecodePublishedSnapshot - publish/discard workflow: public pages render
	// from a published snapshot while admins edit live rows. See each
	// method's doc comment on DefaultService.
	PublishDisplay(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error)
	DiscardDisplayChanges(ctx context.Context, displayID int) (*entity.ViewDisplayDef, error)
	HasUnpublishedChanges(ctx context.Context, displayID int) (bool, error)
	DecodePublishedSnapshot(display entity.ViewDisplayDef) (entity.ViewSnapshot, error)
}

// PreviewSettings - see Service.PreviewView. Sorts are absent
// because, like filters/fields, they are saved immediately and read from
// the DB.
type PreviewSettings struct {
	DisplayStyle   entity.ViewDisplayStyle
	GridColumns    int
	ShowPagination bool
	PageSize       int
	Page           int
	// SkipItems - the draft ViewDisplayDef.SkipItems.
	SkipItems int
	// Letter - the preview's glossary letter. A preview has no real URL,
	// so a letter click re-POSTs it instead of navigating. Used only when
	// the view's ShowGlossary (read from the DB) is on.
	Letter string
	// ExposedFilterValues - the preview's values for exposed filters,
	// re-POSTed by the preview form instead of a "?param=value" query
	// string. Passed to RunView as QueryParams.
	ExposedFilterValues map[string]string
}

// DefaultService - the Service on the views repository and query.
type DefaultService struct {
	repository      entity.ViewRepository
	query           entity.ViewQuery
	contentTypeRepo entity.ContentTypeRepository
	uow             entity.UnitOfWork
	// pathOwner - what else on the site answers a path ("" - nothing):
	// a page display doesn't take it (nil - not checked).
	pathOwner func(ctx context.Context, path string) (string, error)
	// renamed - called in the transaction renaming a display (SetRenamed):
	// what refers to it by its machine name follows (the front page feed).
	renamed func(ctx context.Context, oldName, newName string) error
	logger  *slog.Logger
}

// SetRenamed sets what follows a display's new machine name (wiring).
func (s *DefaultService) SetRenamed(f func(ctx context.Context, oldName, newName string) error) {
	s.renamed = f
}

// ErrMachineNameInvalid - a display's machine name isn't
// [a-z][a-z0-9_]* (up to 64).
var ErrMachineNameInvalid = errors.New("a display's machine name must be lowercase letters, digits and _, starting with a letter")

// displayMachineNameRe - a display's machine name.
var displayMachineNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// NewService - the Views service on the views repository r and query q,
// the content types' repository (fields' storages), the unit of work and
// pathOwner (what else on the site answers a page's path; nil - not
// checked).
func NewService(
	r entity.ViewRepository,
	q entity.ViewQuery,
	contentTypeRepo entity.ContentTypeRepository,
	uow entity.UnitOfWork,
	pathOwner func(ctx context.Context, path string) (string, error),
	baseLogger *slog.Logger,
) Service {
	return &DefaultService{
		repository:      r,
		query:           q,
		contentTypeRepo: contentTypeRepo,
		uow:             uow,
		pathOwner:       pathOwner,
		logger:          baseLogger.With("component", "view.service"),
	}
}

// Views - see entity.ViewRepository.Views.
func (s *DefaultService) Views(ctx context.Context) (entity.ViewDisplayDefs, error) {
	views, err := s.repository.Views(ctx)
	return entity.Wrapped(views, err, "listing views")
}

// AttachmentDisplays - see entity.ViewRepository.AttachmentDisplays.
func (s *DefaultService) AttachmentDisplays(ctx context.Context) (entity.ViewDisplayDefs, error) {
	return s.repository.AttachmentDisplays(ctx)
}

// TermDisplays - see entity.ViewRepository.TermDisplays.
func (s *DefaultService) TermDisplays(ctx context.Context) (entity.ViewDisplayDefs, error) {
	return s.repository.TermDisplays(ctx)
}

// UserTabDisplays - see entity.ViewRepository.UserTabDisplays.
func (s *DefaultService) UserTabDisplays(ctx context.Context) (entity.ViewDisplayDefs, error) {
	return s.repository.UserTabDisplays(ctx)
}

// PublicPageViews - see entity.ViewRepository.PublicPageViews.
func (s *DefaultService) PublicPageViews(ctx context.Context) (entity.ViewDisplayDefs, error) {
	views, err := s.repository.PublicPageViews(ctx)
	return entity.Wrapped(views, err, "listing public page views")
}

// ViewByID - the display with the given id.
func (s *DefaultService) ViewByID(ctx context.Context, id int) (*entity.ViewDisplayDef, error) {
	v, err := s.repository.ViewByID(ctx, id)
	return entity.Found(v, err, "finding view by id (id %v)", id)
}

// ViewByMachineName - the display with the given machine name.
func (s *DefaultService) ViewByMachineName(ctx context.Context, machineName string) (*entity.ViewDisplayDef, error) {
	v, err := s.repository.ViewByMachineName(ctx, machineName)
	return entity.Found(v, err, "finding view by machine name (machineName %v)", machineName)
}

// ViewByURLPath - see entity.ViewRepository.ViewByURLPath.
func (s *DefaultService) ViewByURLPath(ctx context.Context, urlPath string) (*entity.ViewDisplayDef, error) {
	v, err := s.repository.ViewByURLPath(ctx, urlPath)
	return entity.Found(v, err, "finding view by url path (urlPath %v)", urlPath)
}

// resolveGlossaryTarget validates v.GlossaryFieldStorageID for
// CreateView/UpdateView. nil is always valid and means Node.Title.
func (s *DefaultService) resolveGlossaryTarget(ctx context.Context, v entity.ViewDisplayDef) error {
	if v.GlossaryFieldStorageID == nil {
		return nil
	}
	fs, err := s.contentTypeRepo.FieldStorageByID(ctx, *v.GlossaryFieldStorageID)
	if err != nil {
		return err
	}
	if !GlossaryFieldType(fs.FieldType) {
		return fmt.Errorf("%w: %q", ErrGlossaryFieldTypeUnsupported, fs.FieldType)
	}
	return nil
}

// CreateView creates a display: its URLPath trimmed (none for a display
// without a URL) and checked, its glossary target validated, named
// "view_"+id (createNamed); ErrDuplicate when the path is taken.
func (s *DefaultService) CreateView(ctx context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error) {
	v.URLPath = strings.Trim(strings.TrimSpace(v.URLPath), "/")
	// Displays without a URL (e.g. blocks) never keep a url_path,
	// whatever the caller passed.
	if !v.DisplayType.HasURL() {
		v.URLPath = ""
	} else if err := checkURLArgument(v.URLPath); err != nil {
		return nil, err
	} else if err := s.checkPathOwner(ctx, v); err != nil {
		return nil, err
	}
	if err := s.resolveGlossaryTarget(ctx, v); err != nil {
		return nil, err
	}

	created, err := s.createDisplay(ctx, v)
	if err != nil {
		if entity.IsDuplicateKey(err) {
			return nil, fmt.Errorf("%w", ErrDuplicate)
		}
		return nil, fmt.Errorf("creating view: %w", err)
	}
	return created, nil
}

// UpdateView saves a display: its URLPath, profile tab, theme template,
// glossary target and machine name checked; a renamed display's
// references follow its new name in the same transaction (SetRenamed).
func (s *DefaultService) UpdateView(ctx context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error) {
	v.URLPath = strings.Trim(strings.TrimSpace(v.URLPath), "/")
	// Displays without a URL never keep a url_path (the form may still
	// submit a hidden value after switching page→block), and skip the
	// availability/duplicate checks below.
	if !v.DisplayType.HasURL() {
		v.URLPath = ""
	} else {
		if err := checkURLArgument(v.URLPath); err != nil {
			return nil, err
		}
		existing, err := s.repository.ViewByURLPath(ctx, v.URLPath)
		if err == nil && existing.ID != v.ID {
			return nil, fmt.Errorf("%w", ErrDuplicate)
		}
		if err := s.checkPathOwner(ctx, v); err != nil {
			return nil, err
		}
	}
	if err := s.checkProfileTab(ctx, &v); err != nil {
		return nil, err
	}
	v.ThemeTemplate = strings.ToLower(strings.TrimSpace(v.ThemeTemplate))
	if v.ThemeTemplate != "" && !themeTemplateRe.MatchString(v.ThemeTemplate) {
		return nil, ErrThemeTemplateInvalid
	}
	if err := s.resolveGlossaryTarget(ctx, v); err != nil {
		return nil, err
	}
	existing, err := s.repository.ViewByID(ctx, v.ID)
	if err != nil {
		return nil, fmt.Errorf("loading view (id %v): %w", v.ID, err)
	}
	oldName := existing.MachineName
	if v.MachineName == "" {
		v.MachineName = oldName
	}
	if v.MachineName != oldName {
		if !displayMachineNameRe.MatchString(v.MachineName) {
			return nil, fmt.Errorf("%w: %q", ErrMachineNameInvalid, v.MachineName)
		}
		if other, err := s.repository.ViewByMachineName(ctx, v.MachineName); err == nil && other.ID != v.ID {
			return nil, fmt.Errorf("%w: machine name %q", ErrDuplicate, v.MachineName)
		} else if err != nil && !entity.IsNotFound(err) {
			return nil, fmt.Errorf("checking the machine name %q: %w", v.MachineName, err)
		}
	}

	var updated *entity.ViewDisplayDef
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		if updated, err = s.repository.UpdateView(ctx, v); err != nil {
			return err
		}
		if v.MachineName == oldName {
			return nil
		}
		if updated, err = s.repository.SetViewMachineName(ctx, v.ID, v.MachineName); err != nil {
			return err
		}
		if s.renamed != nil {
			return s.renamed(ctx, oldName, v.MachineName)
		}
		return nil
	})
	if err != nil {
		if entity.IsDuplicateKey(err) {
			return nil, fmt.Errorf("%w", ErrDuplicate)
		}
		return nil, fmt.Errorf("updating view (id %v): %w", v.ID, err)
	}
	return updated, nil
}

// checkURLArgument - at most one "%" in a URLPath, as a whole segment.
func checkURLArgument(urlPath string) error {
	if !strings.Contains(urlPath, entity.ViewURLArgument) {
		return nil
	}
	args := 0
	for _, seg := range strings.Split(urlPath, "/") {
		if seg == entity.ViewURLArgument {
			args++
		} else if strings.Contains(seg, entity.ViewURLArgument) {
			return ErrURLArgumentInvalid
		}
	}
	if args > 1 {
		return ErrURLArgumentInvalid
	}
	return nil
}

var profileTabPathRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// themeTemplateRe - a ViewDisplayDef.ThemeTemplate name (a file name and
// a template name suffix).
var themeTemplateRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)

// checkProfileTab validates a user_tab display's path segment (other
// display types keep no tab path) and defaults its access.
func (s *DefaultService) checkProfileTab(ctx context.Context, v *entity.ViewDisplayDef) error {
	if v.DisplayType != entity.ViewDisplayTypeUserTab {
		v.ProfileTabPath = ""
		return nil
	}
	v.ProfileTabPath = strings.ToLower(strings.Trim(strings.TrimSpace(v.ProfileTabPath), "/"))
	if !profileTabPathRe.MatchString(v.ProfileTabPath) || entity.ProfileTabPathReserved(v.ProfileTabPath) {
		return ErrProfileTabPathInvalid
	}
	if v.ProfileTabAccess != entity.ViewProfileTabAccessOwner {
		v.ProfileTabAccess = entity.ViewProfileTabAccessAll
	}
	tabs, err := s.repository.UserTabDisplays(ctx)
	if err != nil {
		return err
	}
	for _, t := range tabs {
		if t.ID != v.ID && t.ProfileTabPath == v.ProfileTabPath {
			return ErrProfileTabPathTaken
		}
	}
	return nil
}

// DeleteView deletes one display (DeleteViewContainer - the whole View).
func (s *DefaultService) DeleteView(ctx context.Context, id int) error {
	if err := s.repository.DeleteView(ctx, id); err != nil {
		return fmt.Errorf("deleting view (id %v): %w", id, err)
	}
	return nil
}

// ViewContainerByID - see entity.ViewRepository.ViewContainerByID.
func (s *DefaultService) ViewContainerByID(ctx context.Context, id int) (*entity.View, error) {
	v, err := s.repository.ViewContainerByID(ctx, id)
	return entity.Found(v, err, "finding view container by id (id %v)", id)
}

// DeleteViewContainer - see entity.ViewRepository.DeleteViewContainer.
func (s *DefaultService) DeleteViewContainer(ctx context.Context, id int) error {
	if err := s.repository.DeleteViewContainer(ctx, id); err != nil {
		return fmt.Errorf("deleting view container (id %v): %w", id, err)
	}
	return nil
}

// --- Displays grouped into Views ---

// DefaultDisplayOfView - see entity.ViewRepository.DefaultDisplayOfView.
func (s *DefaultService) DefaultDisplayOfView(ctx context.Context, viewID int) (*entity.ViewDisplayDef, error) {
	d, err := s.repository.DefaultDisplayOfView(ctx, viewID)
	return entity.Found(d, err, "finding default display of view (viewID %v)", viewID)
}

// DisplaysOfView - see entity.ViewRepository.DisplaysOfView.
func (s *DefaultService) DisplaysOfView(ctx context.Context, viewID int) (entity.ViewDisplayDefs, error) {
	displays, err := s.repository.DisplaysOfView(ctx, viewID)
	return entity.Wrapped(displays, err, "listing displays of view (viewID %v)", viewID)
}

// uniqueViewSlug appends "-2", "-3", ... to base until exists reports
// false.
func (s *DefaultService) uniqueViewSlug(ctx context.Context, base string, exists func(context.Context, string) (bool, error)) (string, error) {
	candidate := base
	for i := 2; i < 100; i++ {
		taken, err := exists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("could not generate a unique slug for %q", base)
}

// createDisplay inserts a view_displays row (createNamed).
func (s *DefaultService) createDisplay(ctx context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error) {
	return createNamed(ctx, s, func(ctx context.Context, machineName string) (int, error) {
		v.MachineName = machineName
		created, err := s.repository.CreateView(ctx, v)
		if err != nil {
			return 0, err
		}
		return created.ID, nil
	}, s.repository.SetViewMachineName)
}

// createViewContainer inserts the View container row (createNamed).
func (s *DefaultService) createViewContainer(ctx context.Context, v entity.View) (*entity.View, error) {
	return createNamed(ctx, s, func(ctx context.Context, machineName string) (int, error) {
		v.MachineName = machineName
		created, err := s.repository.CreateViewContainer(ctx, v)
		if err != nil {
			return 0, err
		}
		return created.ID, nil
	}, s.repository.SetViewContainerMachineName)
}

// createNamed creates a row with a placeholder machine_name (create:
// NOT NULL UNIQUE, but the id isn't known before INSERT), then renames
// it to "view_"+id (rename), in one transaction. An id-based name is
// unique by construction and doesn't go stale when the label changes;
// url_path stays label-based and editable.
func createNamed[T any](ctx context.Context, s *DefaultService, create func(ctx context.Context, machineName string) (int, error), rename func(ctx context.Context, id int, machineName string) (*T, error)) (*T, error) {
	var result *T
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		id, err := create(ctx, uuid.NewString())
		if err != nil {
			return err
		}
		result, err = rename(ctx, id, "view_"+strconv.Itoa(id))
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// checkPathOwner - ErrURLPathTaken when page display v takes a new
// URLPath something else on the site answers (pathOwner). An unchanged
// address isn't checked again, and one with a "%" argument isn't checked
// (it stands for many).
func (s *DefaultService) checkPathOwner(ctx context.Context, v entity.ViewDisplayDef) error {
	if s.pathOwner == nil || v.URLPath == "" || strings.Contains(v.URLPath, entity.ViewURLArgument) {
		return nil
	}
	if current, err := s.repository.ViewByID(ctx, v.ID); err == nil && current.URLPath == v.URLPath {
		return nil
	}
	owner, err := s.pathOwner(ctx, v.URLPath)
	if err != nil {
		return fmt.Errorf("checking the address %q: %w", v.URLPath, err)
	}
	if owner != "" {
		return fmt.Errorf("%w (%s)", ErrURLPathTaken, owner)
	}
	return nil
}

func (s *DefaultService) urlPathTaken(ctx context.Context, urlPath string) (bool, error) {
	_, err := s.repository.ViewByURLPath(ctx, urlPath)
	if err == nil {
		return true, nil
	}
	if entity.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// urlPathTakenAny - url_path uniqueness check for a new Default,
// which, unlike urlPathTaken, also sees existing Defaults (see
// entity.ViewRepository.ViewByURLPathAny).
// pageURLTaken - whether urlPath is a View page's or something else's on
// the site (pathOwner: content, a term, a route): a new page display's
// address is picked free of both.
func (s *DefaultService) pageURLTaken(ctx context.Context, urlPath string) (bool, error) {
	if taken, err := s.urlPathTaken(ctx, urlPath); err != nil || taken {
		return taken, err
	}
	if s.pathOwner == nil {
		return false, nil
	}
	owner, err := s.pathOwner(ctx, urlPath)
	return owner != "", err
}

func (s *DefaultService) urlPathTakenAny(ctx context.Context, urlPath string) (bool, error) {
	_, err := s.repository.ViewByURLPathAny(ctx, urlPath)
	if err == nil {
		return true, nil
	}
	if entity.IsNotFound(err) {
		return false, nil
	}
	return false, err
}
