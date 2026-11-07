package database

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewdisplaydef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewfielddef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewfilterdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewsortdef"
)

// NewViewRepository - the View repository on client.
func NewViewRepository(client *generated.Client) *ViewRepository {
	return &ViewRepository{client: client}
}

// ViewRepository is the ent adapter for entity.ViewRepository: View
// metadata CRUD here, query execution in view_query.go (filters in
// view_filters.go, sorting in view_sort.go).
type ViewRepository struct {
	client *generated.Client
}

// --- View (display) CRUD ---

// Views returns the admin views list: one row per View, not per display.
// An ungrouped display shows as itself; a grouped View is represented by
// its Default display, which always exists and so keeps a stable identity
// as other displays are added or deleted. Sibling displays are reachable
// via the tab strip in the edit page.
func (r ViewRepository) Views(ctx context.Context) (entity.ViewDisplayDefs, error) {
	client := GetClient(ctx, r.client)
	lone, err := client.ViewDisplayDef.Query().
		Where(viewdisplaydef.ViewIDIsNil(), viewdisplaydef.IsDefault(false)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ViewRepository.Views: %w", err)
	}
	defaults, err := client.ViewDisplayDef.Query().
		Where(viewdisplaydef.IsDefault(true)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ViewRepository.Views: %w", err)
	}

	views := make(entity.ViewDisplayDefs, 0, len(lone)+len(defaults))
	for _, v := range lone {
		views = append(views, dao.ToViewDisplayDefEntity(v))
	}
	for _, v := range defaults {
		views = append(views, dao.ToViewDisplayDefEntity(v))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Label < views[j].Label })
	return views, nil
}

// AttachmentDisplays returns every enabled, non-Default attachment display,
// ordered by AttachWeight (then id); node pages filter them by content type.
func (r ViewRepository) AttachmentDisplays(ctx context.Context) (entity.ViewDisplayDefs, error) {
	return r.enabledDisplaysOf(ctx, entity.ViewDisplayTypeAttachment)
}

// profileTabAccessOrDefault: an unset access means every visitor.
func profileTabAccessOrDefault(a entity.ViewProfileTabAccess) entity.ViewProfileTabAccess {
	if a == "" {
		return entity.ViewProfileTabAccessAll
	}
	return a
}

// UserTabDisplays returns every enabled, non-Default user_tab display,
// ordered by AttachWeight (then id): the user profile's View tabs.
func (r ViewRepository) UserTabDisplays(ctx context.Context) (entity.ViewDisplayDefs, error) {
	return r.enabledDisplaysOf(ctx, entity.ViewDisplayTypeUserTab)
}

// TermDisplays returns every enabled, non-Default term display, ordered by
// AttachWeight (then id); term pages filter them by vocabulary.
func (r ViewRepository) TermDisplays(ctx context.Context) (entity.ViewDisplayDefs, error) {
	return r.enabledDisplaysOf(ctx, entity.ViewDisplayTypeTerm)
}

// enabledDisplaysOf - the enabled, non-Default displays of type t,
// ordered by AttachWeight (then id).
func (r ViewRepository) enabledDisplaysOf(ctx context.Context, t entity.ViewDisplayType) (entity.ViewDisplayDefs, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().
		Where(
			viewdisplaydef.DisplayTypeEQ(string(t)),
			viewdisplaydef.IsDefault(false),
			viewdisplaydef.Status(1),
		).
		Order(generated.Asc(viewdisplaydef.FieldAttachWeight), generated.Asc(viewdisplaydef.FieldID)).
		All(ctx)
	return all(res, err, dao.ToViewDisplayDefEntity, "the "+string(t)+" displays")
}

// PublicPageViews - every non-Default page display across all Views,
// enabled or not (the callers check Status).
func (r ViewRepository) PublicPageViews(ctx context.Context) (entity.ViewDisplayDefs, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().
		Where(
			viewdisplaydef.DisplayTypeEQ(string(entity.ViewDisplayTypePage)),
			viewdisplaydef.IsDefault(false),
		).
		All(ctx)
	return all(res, err, dao.ToViewDisplayDefEntity, "ViewRepository.PublicPageViews")
}

// ViewByID - the display with id; ErrEntityNotFound if there is none.
func (r ViewRepository) ViewByID(ctx context.Context, id int) (*entity.ViewDisplayDef, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Get(ctx, id)
	return one(res, err, dao.ToViewDisplayDefEntity, "view with id %d", id)
}

// ViewByMachineName - the display named machineName; ErrEntityNotFound if
// there is none.
func (r ViewRepository) ViewByMachineName(ctx context.Context, machineName string) (*entity.ViewDisplayDef, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().Where(viewdisplaydef.MachineName(machineName)).Only(ctx)
	return one(res, err, dao.ToViewDisplayDefEntity, "view with machine_name %q", machineName)
}

// ViewByURLPath excludes Default displays as defense in depth: their
// url_path is a placeholder that must never resolve as a page (see
// entity.ViewDisplayDef.IsDefault).
func (r ViewRepository) ViewByURLPath(ctx context.Context, urlPath string) (*entity.ViewDisplayDef, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().
		Where(viewdisplaydef.URLPath(urlPath), viewdisplaydef.IsDefault(false)).
		Only(ctx)
	return one(res, err, dao.ToViewDisplayDefEntity, "view with url_path %q", urlPath)
}

// PageDisplaysWithArgument - see entity.ViewRepository.
func (r ViewRepository) PageDisplaysWithArgument(ctx context.Context) (entity.ViewDisplayDefs, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().
		Where(
			viewdisplaydef.DisplayTypeEQ(string(entity.ViewDisplayTypePage)),
			viewdisplaydef.IsDefault(false),
			viewdisplaydef.URLPathContains(entity.ViewURLArgument),
		).
		Order(generated.Asc(viewdisplaydef.FieldID)).
		All(ctx)
	return all(res, err, dao.ToViewDisplayDefEntity, "ViewRepository.PageDisplaysWithArgument")
}

// ViewByURLPathAny - see entity.ViewRepository.ViewByURLPathAny.
func (r ViewRepository) ViewByURLPathAny(ctx context.Context, urlPath string) (*entity.ViewDisplayDef, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().
		Where(viewdisplaydef.URLPath(urlPath)).
		Only(ctx)
	return one(res, err, dao.ToViewDisplayDefEntity, "view with url_path %q", urlPath)
}

// CreateView adds the display v (an empty URLPath stored as none) and
// returns it with its ID.
func (r ViewRepository) CreateView(ctx context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.ViewDisplayDef.Create().
		SetBaseEntity(string(v.BaseEntity)).
		SetNillableViewID(v.ViewID).
		SetIsDefault(v.IsDefault).
		SetOverrideSettings(v.OverrideSettings).
		SetOverrideFilters(v.OverrideFilters).
		SetOverrideSort(v.OverrideSort).
		SetOverrideFields(v.OverrideFields).
		SetMachineName(v.MachineName).
		SetLabel(v.Label).
		SetMetaTitle(v.MetaTitle).
		SetMetaDescription(v.MetaDescription).
		SetDisplayType(string(v.DisplayType)).
		SetDisplayStyle(string(v.DisplayStyle)).
		SetGridColumns(v.GridColumns).
		SetOverflowAfter(v.OverflowAfter).
		SetCarouselSlidesPerView(v.CarouselSlidesPerView).
		SetCarouselAutoplay(v.CarouselAutoplay).
		SetCarouselLoop(v.CarouselLoop).
		SetCarouselShowNavigation(v.CarouselShowNavigation).
		SetCarouselShowPagination(v.CarouselShowPagination).
		SetCarouselEffect(v.CarouselEffect).
		SetShowPagination(v.ShowPagination).
		SetPaginationAjax(v.PaginationAjax).
		SetShowGlossary(v.ShowGlossary).
		SetPageSize(v.PageSize).
		SetSkipItems(v.SkipItems).
		SetHeaderText(v.HeaderText).
		SetFooterText(v.FooterText).
		SetOverflowHeaderText(v.OverflowHeaderText).
		SetAttachContentTypes(v.AttachContentTypes).
		SetAttachWeight(v.AttachWeight).
		SetTermVocabularies(v.TermVocabularies).
		SetProfileTabPath(v.ProfileTabPath).
		SetProfileTabAccess(string(profileTabAccessOrDefault(v.ProfileTabAccess))).
		SetThemeTemplate(v.ThemeTemplate).
		SetAttachTitle(v.AttachTitle).
		SetAttachShowTitle(v.AttachShowTitle).
		SetCustomCSS(v.CustomCSS).
		SetRowTemplate(v.RowTemplate).
		SetGroupByField(v.GroupByField).
		SetGroupDateFormat(v.GroupDateFormat).
		SetStatus(v.Status).
		SetCreatedBy(v.CreatedBy).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	// Leave url_path NULL instead of "": the column is unique and an empty
	// string is a real value that would collide with another block display.
	// Only page displays have a non-empty URLPath.
	if v.URLPath != "" {
		create = create.SetURLPath(v.URLPath)
	}
	if v.GlossaryFieldStorageID != nil {
		create = create.SetGlossaryFieldStorageID(*v.GlossaryFieldStorageID)
	}
	if v.CarouselLinkFieldStorageID != nil {
		create = create.SetCarouselLinkFieldStorageID(*v.CarouselLinkFieldStorageID)
	}
	res, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("ViewRepository.CreateView: %w", err)
	}

	created := dao.ToViewDisplayDefEntity(res)
	return &created, nil
}

// UpdateView never touches machine_name (see SetViewMachineName),
// base_entity (immutable after creation) or published_snapshot/
// published_at (see SetPublishedSnapshot), so a caller passing a plain
// entity.ViewDisplayDef can't accidentally wipe them.
func (r ViewRepository) UpdateView(ctx context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error) {
	client := GetClient(ctx, r.client)
	update := client.ViewDisplayDef.UpdateOneID(v.ID).
		SetNillableViewID(v.ViewID).
		SetOverrideSettings(v.OverrideSettings).
		SetOverrideFilters(v.OverrideFilters).
		SetOverrideSort(v.OverrideSort).
		SetOverrideFields(v.OverrideFields).
		SetLabel(v.Label).
		SetMetaTitle(v.MetaTitle).
		SetMetaDescription(v.MetaDescription).
		SetDisplayType(string(v.DisplayType)).
		SetDisplayStyle(string(v.DisplayStyle)).
		SetGridColumns(v.GridColumns).
		SetOverflowAfter(v.OverflowAfter).
		SetCarouselSlidesPerView(v.CarouselSlidesPerView).
		SetCarouselAutoplay(v.CarouselAutoplay).
		SetCarouselLoop(v.CarouselLoop).
		SetCarouselShowNavigation(v.CarouselShowNavigation).
		SetCarouselShowPagination(v.CarouselShowPagination).
		SetCarouselEffect(v.CarouselEffect).
		SetShowPagination(v.ShowPagination).
		SetPaginationAjax(v.PaginationAjax).
		SetShowGlossary(v.ShowGlossary).
		SetPageSize(v.PageSize).
		SetSkipItems(v.SkipItems).
		SetHeaderText(v.HeaderText).
		SetFooterText(v.FooterText).
		SetOverflowHeaderText(v.OverflowHeaderText).
		SetAttachContentTypes(v.AttachContentTypes).
		SetAttachWeight(v.AttachWeight).
		SetTermVocabularies(v.TermVocabularies).
		SetProfileTabPath(v.ProfileTabPath).
		SetProfileTabAccess(string(profileTabAccessOrDefault(v.ProfileTabAccess))).
		SetThemeTemplate(v.ThemeTemplate).
		SetAttachTitle(v.AttachTitle).
		SetAttachShowTitle(v.AttachShowTitle).
		SetCustomCSS(v.CustomCSS).
		SetRowTemplate(v.RowTemplate).
		SetGroupByField(v.GroupByField).
		SetGroupDateFormat(v.GroupDateFormat).
		SetStatus(v.Status).
		SetUpdatedAt(time.Now())
	// An empty URLPath (block display) must clear the column, not store ""
	// (see CreateView).
	if v.URLPath != "" {
		update = update.SetURLPath(v.URLPath)
	} else {
		update = update.ClearURLPath()
	}
	if v.GlossaryFieldStorageID != nil {
		update = update.SetGlossaryFieldStorageID(*v.GlossaryFieldStorageID)
	} else {
		update = update.ClearGlossaryFieldStorageID()
	}
	if v.CarouselLinkFieldStorageID != nil {
		update = update.SetCarouselLinkFieldStorageID(*v.CarouselLinkFieldStorageID)
	} else {
		update = update.ClearCarouselLinkFieldStorageID()
	}
	res, err := update.Save(ctx)
	return one(res, err, dao.ToViewDisplayDefEntity, "view with id %d", v.ID)
}

// SetViewMachineName is called only from views.DefaultService's
// create-then-rename sequence right after CreateView.
func (r ViewRepository) SetViewMachineName(ctx context.Context, id int, machineName string) (*entity.ViewDisplayDef, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.UpdateOneID(id).SetMachineName(machineName).Save(ctx)
	return one(res, err, dao.ToViewDisplayDefEntity, "view with id %d", id)
}

// SetPublishedSnapshot is called only from views.DefaultService.PublishDisplay.
func (r ViewRepository) SetPublishedSnapshot(ctx context.Context, id int, snapshot string, publishedAt time.Time) (*entity.ViewDisplayDef, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.UpdateOneID(id).
		SetPublishedSnapshot(snapshot).
		SetPublishedAt(publishedAt).
		Save(ctx)
	return one(res, err, dao.ToViewDisplayDefEntity, "view with id %d", id)
}

// DeleteView deletes the display with id, its filters, sorts and fields
// with it; ErrEntityNotFound if there is none.
func (r ViewRepository) DeleteView(ctx context.Context, id int) error {
	return notFoundAs(GetClient(ctx, r.client).ViewDisplayDef.DeleteOneID(id).Exec(ctx), "view with id %d", id)
}

// --- View (container) CRUD: displays grouped into a View ---

// CreateViewContainer is called only from views.DefaultService.CloneDisplay
// (see entity.ViewRepository.CreateViewContainer).
func (r ViewRepository) CreateViewContainer(ctx context.Context, v entity.View) (*entity.View, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	res, err := client.View.Create().
		SetMachineName(v.MachineName).
		SetLabel(v.Label).
		SetCreatedBy(v.CreatedBy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	created := dao.ToViewEntity(res)
	return &created, nil
}

// ViewContainerByID - see entity.ViewRepository.ViewContainerByID.
func (r ViewRepository) ViewContainerByID(ctx context.Context, id int) (*entity.View, error) {
	res, err := GetClient(ctx, r.client).View.Get(ctx, id)
	return one(res, err, dao.ToViewEntity, "view container with id %d", id)
}

// SetViewContainerMachineName is called only from views.DefaultService's
// create-then-rename sequence right after CreateViewContainer.
func (r ViewRepository) SetViewContainerMachineName(ctx context.Context, id int, machineName string) (*entity.View, error) {
	res, err := GetClient(ctx, r.client).View.UpdateOneID(id).SetMachineName(machineName).Save(ctx)
	return one(res, err, dao.ToViewEntity, "view container with id %d", id)
}

// DeleteViewContainer deletes the View row by id; ON DELETE CASCADE
// removes its view_displays rows and, in turn, their filters, sorts and
// fields.
func (r ViewRepository) DeleteViewContainer(ctx context.Context, id int) error {
	return notFoundAs(GetClient(ctx, r.client).View.DeleteOneID(id).Exec(ctx), "view container with id %d", id)
}

// DefaultDisplayOfView - see entity.ViewRepository.DefaultDisplayOfView.
func (r ViewRepository) DefaultDisplayOfView(ctx context.Context, viewID int) (*entity.ViewDisplayDef, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().
		Where(viewdisplaydef.ViewID(viewID), viewdisplaydef.IsDefault(true)).
		Only(ctx)
	return one(res, err, dao.ToViewDisplayDefEntity, "default display of view %d", viewID)
}

// DisplaysOfView - see entity.ViewRepository.DisplaysOfView; by creation
// time.
func (r ViewRepository) DisplaysOfView(ctx context.Context, viewID int) (entity.ViewDisplayDefs, error) {
	res, err := GetClient(ctx, r.client).ViewDisplayDef.Query().
		Where(viewdisplaydef.ViewID(viewID), viewdisplaydef.IsDefault(false)).
		Order(generated.Asc(viewdisplaydef.FieldCreatedAt)).
		All(ctx)
	return all(res, err, dao.ToViewDisplayDefEntity, "ViewRepository.DisplaysOfView")
}

// --- Filter CRUD ---

// FiltersByViewID - the filters of the display viewID, by sort order,
// then ID.
func (r ViewRepository) FiltersByViewID(ctx context.Context, viewID int) (entity.ViewFilterDefs, error) {
	res, err := GetClient(ctx, r.client).ViewFilterDef.Query().
		Where(viewfilterdef.ViewDisplayID(viewID)).
		Order(generated.Asc(viewfilterdef.FieldSortOrder), generated.Asc(viewfilterdef.FieldID)).
		All(ctx)
	return all(res, err, dao.ToViewFilterDefEntity, "ViewRepository.FiltersByViewID")
}

// FilterByID - the filter with id; ErrEntityNotFound if there is none.
func (r ViewRepository) FilterByID(ctx context.Context, id int) (*entity.ViewFilterDef, error) {
	res, err := GetClient(ctx, r.client).ViewFilterDef.Get(ctx, id)
	return one(res, err, dao.ToViewFilterDefEntity, "view filter with id %d", id)
}

// CreateFilter adds the filter f and returns it with its ID.
func (r ViewRepository) CreateFilter(ctx context.Context, f entity.ViewFilterDef) (*entity.ViewFilterDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.ViewFilterDef.Create().
		SetViewDisplayID(f.ViewDisplayID).
		SetBuiltinField(string(f.BuiltinField)).
		SetOperator(string(f.Operator)).
		SetValue(f.Value).
		SetValueIsRelative(f.ValueIsRelative).
		SetExposed(f.Exposed).
		SetExposedParam(f.ExposedParam).
		SetExposedLabel(f.ExposedLabel).
		SetContextual(f.Contextual).
		SetSortOrder(f.SortOrder).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	if f.FieldStorageID != nil {
		create = create.SetFieldStorageID(*f.FieldStorageID)
	}
	create = create.SetNillableViaFieldStorageID(f.ViaFieldStorageID)
	res, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("ViewRepository.CreateFilter: %w", err)
	}

	created := dao.ToViewFilterDefEntity(res)
	return &created, nil
}

// UpdateFilter saves f over the filter f.ID (its display stays) and
// returns it; ErrEntityNotFound if there is none.
func (r ViewRepository) UpdateFilter(ctx context.Context, f entity.ViewFilterDef) (*entity.ViewFilterDef, error) {
	client := GetClient(ctx, r.client)
	update := client.ViewFilterDef.UpdateOneID(f.ID).
		SetBuiltinField(string(f.BuiltinField)).
		SetOperator(string(f.Operator)).
		SetValue(f.Value).
		SetValueIsRelative(f.ValueIsRelative).
		SetExposed(f.Exposed).
		SetExposedParam(f.ExposedParam).
		SetExposedLabel(f.ExposedLabel).
		SetContextual(f.Contextual).
		SetSortOrder(f.SortOrder).
		SetUpdatedAt(time.Now())
	if f.FieldStorageID != nil {
		update = update.SetFieldStorageID(*f.FieldStorageID)
	} else {
		update = update.ClearFieldStorageID()
	}
	if f.ViaFieldStorageID != nil {
		update = update.SetViaFieldStorageID(*f.ViaFieldStorageID)
	} else {
		update = update.ClearViaFieldStorageID()
	}
	res, err := update.Save(ctx)
	return one(res, err, dao.ToViewFilterDefEntity, "view filter with id %d", f.ID)
}

// DeleteFilter deletes the filter with id; ErrEntityNotFound if there is
// none.
func (r ViewRepository) DeleteFilter(ctx context.Context, id int) error {
	return notFoundAs(GetClient(ctx, r.client).ViewFilterDef.DeleteOneID(id).Exec(ctx), "view filter with id %d", id)
}

// --- Sort criterion CRUD ("Sort criteria") ---

// SortsByViewID - the sort criteria of the display viewID, by sort order,
// then ID.
func (r ViewRepository) SortsByViewID(ctx context.Context, viewID int) (entity.ViewSortDefs, error) {
	res, err := GetClient(ctx, r.client).ViewSortDef.Query().
		Where(viewsortdef.ViewDisplayID(viewID)).
		Order(generated.Asc(viewsortdef.FieldSortOrder), generated.Asc(viewsortdef.FieldID)).
		All(ctx)
	return all(res, err, dao.ToViewSortDefEntity, "ViewRepository.SortsByViewID")
}

// SortByID - the sort criterion with id; ErrEntityNotFound if there is
// none.
func (r ViewRepository) SortByID(ctx context.Context, id int) (*entity.ViewSortDef, error) {
	res, err := GetClient(ctx, r.client).ViewSortDef.Get(ctx, id)
	return one(res, err, dao.ToViewSortDefEntity, "view sort with id %d", id)
}

// CreateSort adds the sort criterion s and returns it with its ID.
func (r ViewRepository) CreateSort(ctx context.Context, s entity.ViewSortDef) (*entity.ViewSortDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.ViewSortDef.Create().
		SetViewDisplayID(s.ViewDisplayID).
		SetBuiltinField(string(s.BuiltinField)).
		SetDirection(s.Direction).
		SetSortOrder(s.SortOrder).
		SetExposedKey(s.ExposedKey).
		SetExposedLabel(s.ExposedLabel).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	if s.FieldStorageID != nil {
		create = create.SetFieldStorageID(*s.FieldStorageID)
	}
	if len(s.Params) > 0 {
		create = create.SetParams(s.Params)
	}
	res, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("ViewRepository.CreateSort: %w", err)
	}

	created := dao.ToViewSortDefEntity(res)
	return &created, nil
}

// UpdateSort never touches FieldStorageID/BuiltinField: a criterion's
// target is immutable once created; Params are replaced.
func (r ViewRepository) UpdateSort(ctx context.Context, s entity.ViewSortDef) (*entity.ViewSortDef, error) {
	client := GetClient(ctx, r.client)
	update := client.ViewSortDef.UpdateOneID(s.ID).
		SetDirection(s.Direction).
		SetSortOrder(s.SortOrder).
		SetExposedKey(s.ExposedKey).
		SetExposedLabel(s.ExposedLabel).
		SetUpdatedAt(time.Now())
	if len(s.Params) > 0 {
		update = update.SetParams(s.Params)
	} else {
		update = update.ClearParams()
	}
	res, err := update.Save(ctx)
	return one(res, err, dao.ToViewSortDefEntity, "view sort with id %d", s.ID)
}

// DeleteSort deletes the sort criterion with id; ErrEntityNotFound if
// there is none.
func (r ViewRepository) DeleteSort(ctx context.Context, id int) error {
	return notFoundAs(GetClient(ctx, r.client).ViewSortDef.DeleteOneID(id).Exec(ctx), "view sort with id %d", id)
}

// --- View field CRUD ("Fields") ---

// FieldsByViewID - the fields of the display viewID, by sort order, then
// ID.
func (r ViewRepository) FieldsByViewID(ctx context.Context, viewID int) (entity.ViewFieldDefs, error) {
	res, err := GetClient(ctx, r.client).ViewFieldDef.Query().
		Where(viewfielddef.ViewDisplayID(viewID)).
		Order(generated.Asc(viewfielddef.FieldSortOrder), generated.Asc(viewfielddef.FieldID)).
		All(ctx)
	return all(res, err, dao.ToViewFieldDefEntity, "ViewRepository.FieldsByViewID")
}

// ViewFieldByID - the View field with id; ErrEntityNotFound if there is
// none.
func (r ViewRepository) ViewFieldByID(ctx context.Context, id int) (*entity.ViewFieldDef, error) {
	res, err := GetClient(ctx, r.client).ViewFieldDef.Get(ctx, id)
	return one(res, err, dao.ToViewFieldDefEntity, "view field with id %d", id)
}

// CreateViewField adds the View field f and returns it with its ID.
func (r ViewRepository) CreateViewField(ctx context.Context, f entity.ViewFieldDef) (*entity.ViewFieldDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.ViewFieldDef.Create().
		SetViewDisplayID(f.ViewDisplayID).
		SetBuiltinField(string(f.BuiltinField)).
		SetLabel(f.Label).
		SetShowLabel(f.ShowLabel).
		SetImageStyle(f.ImageStyle).
		SetDateFormat(f.DateFormat).
		SetTagFormat(string(f.TagFormat)).
		SetImageDisplay(string(f.ImageDisplay)).
		SetFormatter(f.Formatter).
		SetLayout(string(f.Layout)).
		SetLinkTo(string(f.LinkTo)).
		SetCSSClass(f.CSSClass).
		SetRewriteHTML(f.RewriteHTML).
		SetTrimLength(f.TrimLength).
		SetSortOrder(f.SortOrder).
		SetExcludeFromDisplay(f.ExcludeFromDisplay).
		SetHideInOverflow(f.HideInOverflow).
		SetFallbackFieldKey(f.FallbackFieldKey).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	if f.FieldStorageID != nil {
		create = create.SetFieldStorageID(*f.FieldStorageID)
	}
	if len(f.Params) > 0 {
		create = create.SetParams(f.Params)
	}
	res, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("ViewRepository.CreateViewField: %w", err)
	}

	created := dao.ToViewFieldDefEntity(res)
	return &created, nil
}

// UpdateViewField never touches FieldStorageID/BuiltinField: a field's
// target is immutable once created.
func (r ViewRepository) UpdateViewField(ctx context.Context, f entity.ViewFieldDef) (*entity.ViewFieldDef, error) {
	client := GetClient(ctx, r.client)
	update := client.ViewFieldDef.UpdateOneID(f.ID).
		SetLabel(f.Label).
		SetShowLabel(f.ShowLabel).
		SetImageStyle(f.ImageStyle).
		SetDateFormat(f.DateFormat).
		SetTagFormat(string(f.TagFormat)).
		SetImageDisplay(string(f.ImageDisplay)).
		SetFormatter(f.Formatter).
		SetLayout(string(f.Layout)).
		SetLinkTo(string(f.LinkTo)).
		SetCSSClass(f.CSSClass).
		SetRewriteHTML(f.RewriteHTML).
		SetTrimLength(f.TrimLength).
		SetSortOrder(f.SortOrder).
		SetExcludeFromDisplay(f.ExcludeFromDisplay).
		SetHideInOverflow(f.HideInOverflow).
		SetFallbackFieldKey(f.FallbackFieldKey).
		SetUpdatedAt(time.Now())
	if len(f.Params) > 0 {
		update = update.SetParams(f.Params)
	} else {
		update = update.ClearParams()
	}
	res, err := update.Save(ctx)
	return one(res, err, dao.ToViewFieldDefEntity, "view field with id %d", f.ID)
}

// DeleteViewField deletes the View field with id; ErrEntityNotFound if
// there is none.
func (r ViewRepository) DeleteViewField(ctx context.Context, id int) error {
	return notFoundAs(GetClient(ctx, r.client).ViewFieldDef.DeleteOneID(id).Exec(ctx), "view field with id %d", id)
}
