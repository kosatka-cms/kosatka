package views

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// fakeViewRepo - a minimal in-memory entity.ViewRepository for
// PublishDisplay/DiscardDisplayChanges/HasUnpublishedChanges/
// DecodePublishedSnapshot; the other methods are unused stubs.
type fakeViewRepo struct {
	displays map[int]entity.ViewDisplayDef
	filters  map[int]entity.ViewFilterDef
	sorts    map[int]entity.ViewSortDef
	fields   map[int]entity.ViewFieldDef
	nextID   int

	failCreateFilterAfter int // fail the Nth CreateFilter call (0 = never)
	createFilterCalls     int

	runs []entity.ViewRunParams // RunView's and RunCommentView's params, in order
}

func newFakeViewRepo() *fakeViewRepo {
	return &fakeViewRepo{
		displays: map[int]entity.ViewDisplayDef{},
		filters:  map[int]entity.ViewFilterDef{},
		sorts:    map[int]entity.ViewSortDef{},
		fields:   map[int]entity.ViewFieldDef{},
		nextID:   1000,
	}
}

// clone/restore - used by fakeUnitOfWork to simulate rollback on error.
func (r *fakeViewRepo) clone() *fakeViewRepo {
	return &fakeViewRepo{
		displays:              maps.Clone(r.displays),
		filters:               maps.Clone(r.filters),
		sorts:                 maps.Clone(r.sorts),
		fields:                maps.Clone(r.fields),
		nextID:                r.nextID,
		failCreateFilterAfter: r.failCreateFilterAfter,
		createFilterCalls:     r.createFilterCalls,
	}
}

func (r *fakeViewRepo) restore(snap *fakeViewRepo) {
	r.displays = snap.displays
	r.filters = snap.filters
	r.sorts = snap.sorts
	r.fields = snap.fields
	r.nextID = snap.nextID
	r.createFilterCalls = snap.createFilterCalls
}

func (r *fakeViewRepo) ViewByID(_ context.Context, id int) (*entity.ViewDisplayDef, error) {
	d, ok := r.displays[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	return &d, nil
}

func (r *fakeViewRepo) UpdateView(_ context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error) {
	existing, ok := r.displays[v.ID]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	// Like the real repository: UpdateView never touches MachineName or
	// PublishedSnapshot/PublishedAt; they have dedicated setters.
	v.MachineName = existing.MachineName
	v.PublishedSnapshot = existing.PublishedSnapshot
	v.PublishedAt = existing.PublishedAt
	r.displays[v.ID] = v
	saved := v
	return &saved, nil
}

func (r *fakeViewRepo) SetPublishedSnapshot(_ context.Context, id int, snapshot string, publishedAt time.Time) (*entity.ViewDisplayDef, error) {
	d, ok := r.displays[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	d.PublishedSnapshot = snapshot
	d.PublishedAt = &publishedAt
	r.displays[id] = d
	saved := d
	return &saved, nil
}

func (r *fakeViewRepo) DefaultDisplayOfView(_ context.Context, viewID int) (*entity.ViewDisplayDef, error) {
	for _, d := range r.displays {
		if d.IsDefault && d.ViewID != nil && *d.ViewID == viewID {
			found := d
			return &found, nil
		}
	}
	return nil, entity.ErrEntityNotFound
}

func (r *fakeViewRepo) FiltersByViewID(_ context.Context, viewID int) (entity.ViewFilterDefs, error) {
	var out entity.ViewFilterDefs
	for _, f := range r.filters {
		if f.ViewDisplayID == viewID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *fakeViewRepo) CreateFilter(_ context.Context, f entity.ViewFilterDef) (*entity.ViewFilterDef, error) {
	r.createFilterCalls++
	if r.failCreateFilterAfter != 0 && r.createFilterCalls > r.failCreateFilterAfter {
		return nil, errors.New("injected CreateFilter failure")
	}
	r.nextID++
	f.ID = r.nextID
	r.filters[f.ID] = f
	saved := f
	return &saved, nil
}

func (r *fakeViewRepo) DeleteFilter(_ context.Context, id int) error {
	delete(r.filters, id)
	return nil
}

func (r *fakeViewRepo) SortsByViewID(_ context.Context, viewID int) (entity.ViewSortDefs, error) {
	var out entity.ViewSortDefs
	for _, s := range r.sorts {
		if s.ViewDisplayID == viewID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *fakeViewRepo) CreateSort(_ context.Context, s entity.ViewSortDef) (*entity.ViewSortDef, error) {
	r.nextID++
	s.ID = r.nextID
	r.sorts[s.ID] = s
	saved := s
	return &saved, nil
}

func (r *fakeViewRepo) DeleteSort(_ context.Context, id int) error {
	delete(r.sorts, id)
	return nil
}

func (r *fakeViewRepo) FieldsByViewID(_ context.Context, viewID int) (entity.ViewFieldDefs, error) {
	var out entity.ViewFieldDefs
	for _, f := range r.fields {
		if f.ViewDisplayID == viewID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *fakeViewRepo) CreateViewField(_ context.Context, f entity.ViewFieldDef) (*entity.ViewFieldDef, error) {
	r.nextID++
	f.ID = r.nextID
	r.fields[f.ID] = f
	saved := f
	return &saved, nil
}

func (r *fakeViewRepo) DeleteViewField(_ context.Context, id int) error {
	delete(r.fields, id)
	return nil
}

// --- ViewRepository methods unused by the code under test ---

func (r *fakeViewRepo) Views(context.Context) (entity.ViewDisplayDefs, error) { return nil, nil }
func (r *fakeViewRepo) PublicPageViews(context.Context) (entity.ViewDisplayDefs, error) {
	return nil, nil
}
func (r *fakeViewRepo) AttachmentDisplays(context.Context) (entity.ViewDisplayDefs, error) {
	return nil, nil
}
func (r *fakeViewRepo) TermDisplays(context.Context) (entity.ViewDisplayDefs, error) {
	return nil, nil
}
func (r *fakeViewRepo) PageDisplaysWithArgument(context.Context) (entity.ViewDisplayDefs, error) {
	var out entity.ViewDisplayDefs
	for _, d := range r.displays {
		if d.DisplayType == entity.ViewDisplayTypePage && strings.Contains(d.URLPath, "%") {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *fakeViewRepo) UserTabDisplays(context.Context) (entity.ViewDisplayDefs, error) {
	var out entity.ViewDisplayDefs
	for _, d := range r.displays {
		if d.DisplayType == entity.ViewDisplayTypeUserTab {
			out = append(out, d)
		}
	}
	return out, nil
}
func (r *fakeViewRepo) ViewByMachineName(context.Context, string) (*entity.ViewDisplayDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) ViewByURLPath(_ context.Context, urlPath string) (*entity.ViewDisplayDef, error) {
	for _, d := range r.displays {
		if d.URLPath == urlPath {
			return &d, nil
		}
	}
	return nil, entity.ErrEntityNotFound
}
func (r *fakeViewRepo) ViewByURLPathAny(context.Context, string) (*entity.ViewDisplayDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) CreateView(_ context.Context, v entity.ViewDisplayDef) (*entity.ViewDisplayDef, error) {
	r.nextID++
	v.ID = r.nextID
	r.displays[v.ID] = v
	saved := v
	return &saved, nil
}
func (r *fakeViewRepo) DeleteView(context.Context, int) error { return errors.New("not implemented") }
func (r *fakeViewRepo) SetViewMachineName(_ context.Context, id int, machineName string) (*entity.ViewDisplayDef, error) {
	d, ok := r.displays[id]
	if !ok {
		return nil, entity.ErrEntityNotFound
	}
	d.MachineName = machineName
	r.displays[id] = d
	saved := d
	return &saved, nil
}
func (r *fakeViewRepo) CreateViewContainer(context.Context, entity.View) (*entity.View, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) SetViewContainerMachineName(context.Context, int, string) (*entity.View, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) ViewContainerByID(context.Context, int) (*entity.View, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) DeleteViewContainer(context.Context, int) error {
	return errors.New("not implemented")
}
func (r *fakeViewRepo) DisplaysOfView(context.Context, int) (entity.ViewDisplayDefs, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) FilterByID(context.Context, int) (*entity.ViewFilterDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) UpdateFilter(context.Context, entity.ViewFilterDef) (*entity.ViewFilterDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) SortByID(context.Context, int) (*entity.ViewSortDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) UpdateSort(context.Context, entity.ViewSortDef) (*entity.ViewSortDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) ViewFieldByID(context.Context, int) (*entity.ViewFieldDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) UpdateViewField(context.Context, entity.ViewFieldDef) (*entity.ViewFieldDef, error) {
	return nil, errors.New("not implemented")
}
func (r *fakeViewRepo) RunView(_ context.Context, params entity.ViewRunParams) (entity.Nodes, int, error) {
	r.runs = append(r.runs, params)
	return nil, 0, nil
}
func (r *fakeViewRepo) RunCommentView(_ context.Context, params entity.ViewRunParams) (entity.Comments, int, error) {
	r.runs = append(r.runs, params)
	return nil, 0, nil
}
func (r *fakeViewRepo) GlossaryForView(context.Context, entity.ViewFilterDefs, []entity.ViewRunModuleFilter, map[int]entity.FieldStorageDef, *int, map[string]string) (entity.GlossaryData, error) {
	return nil, errors.New("not implemented")
}

// fakeUnitOfWork simulates rollback on error, to prove all of
// DiscardDisplayChanges' writes happen inside s.uow.Do - a mid-way
// failure must leave the repo exactly as before.
type fakeUnitOfWork struct{ repo *fakeViewRepo }

func (u *fakeUnitOfWork) Do(ctx context.Context, fn func(context.Context) error) error {
	snap := u.repo.clone()
	if err := fn(ctx); err != nil {
		u.repo.restore(snap)
		return err
	}
	return nil
}

func newTestViewService(repo *fakeViewRepo) *DefaultService {
	return &DefaultService{
		repository: repo,
		query:      repo,
		uow:        &fakeUnitOfWork{repo: repo},
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestViewSnapshotSettingsApplyTo(t *testing.T) {
	fieldStorageID := 7
	snap := entity.ViewSnapshotSettings{
		DisplayStyle:           entity.ViewDisplayStyleTable,
		GridColumns:            3,
		ShowPagination:         true,
		PaginationAjax:         true,
		ShowGlossary:           true,
		GlossaryFieldStorageID: &fieldStorageID,
		PageSize:               20,
		HeaderText:             "header",
		FooterText:             "footer",
		CustomCSS:              ".x{}",
		RowTemplate:            `<a href="{{url}}">{{field:title}}</a>`,
		GroupByField:           "premiere_us",
		GroupDateFormat:        "month",
		Label:                  "published label",
		MetaTitle:              "published meta title",
		MetaDescription:        "published meta description",
	}
	// Identity fields (ID, MachineName, URLPath, Status, DisplayType) are
	// outside the publish bundle and stay untouched; Label/SEO are inside.
	display := entity.ViewDisplayDef{ID: 5, Label: "stale label", MachineName: "kept"}
	got := snap.ApplyTo(display)

	if got.ID != 5 || got.MachineName != "kept" {
		t.Fatalf("ApplyTo must not touch identity fields outside the publish bundle, got %+v", got)
	}
	if got.Label != "published label" || got.MetaTitle != "published meta title" || got.MetaDescription != "published meta description" {
		t.Fatalf("ApplyTo must overwrite Label/MetaTitle/MetaDescription too, got %+v", got)
	}
	if got.DisplayStyle != entity.ViewDisplayStyleTable || got.GridColumns != 3 || !got.ShowPagination ||
		!got.PaginationAjax || !got.ShowGlossary || got.GlossaryFieldStorageID != &fieldStorageID ||
		got.PageSize != 20 || got.HeaderText != "header" || got.FooterText != "footer" || got.CustomCSS != ".x{}" ||
		got.RowTemplate != `<a href="{{url}}">{{field:title}}</a>` ||
		got.GroupByField != "premiere_us" || got.GroupDateFormat != "month" {
		t.Fatalf("ApplyTo must copy every Settings-bundle field, got %+v", got)
	}
}

func TestPublishDisplayCapturesDefaultEffectiveValues(t *testing.T) {
	repo := newFakeViewRepo()
	viewID := 1
	def := entity.ViewDisplayDef{ID: 1, ViewID: &viewID, IsDefault: true, PageSize: 42, HeaderText: "default header"}
	sibling := entity.ViewDisplayDef{ID: 2, ViewID: &viewID, IsDefault: false, OverrideSettings: false, OverrideFilters: false, OverrideSort: false, OverrideFields: false, PageSize: 999, Label: "sibling label", MetaTitle: "sibling meta title"}
	repo.displays[def.ID] = def
	repo.displays[sibling.ID] = sibling

	s := newTestViewService(repo)
	updated, err := s.PublishDisplay(t.Context(), sibling.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.PublishedAt == nil {
		t.Fatal("expected PublishedAt to be set")
	}
	got, err := s.DecodePublishedSnapshot(*updated)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if got.Settings.PageSize != 42 {
		t.Fatalf("got snapshot PageSize %d, want 42 (Default's own, since sibling inherits Settings)", got.Settings.PageSize)
	}
	if got.Settings.HeaderText != "default header" {
		t.Fatalf("got snapshot HeaderText %q, want Default's own", got.Settings.HeaderText)
	}
	// Label/MetaTitle are published but never inherited from Default (see
	// EffectiveSettings), so they come from sibling even though
	// sibling.OverrideSettings is false.
	if got.Settings.Label != "sibling label" {
		t.Fatalf("got snapshot Label %q, want sibling's own (never inherited from Default)", got.Settings.Label)
	}
	if got.Settings.MetaTitle != "sibling meta title" {
		t.Fatalf("got snapshot MetaTitle %q, want sibling's own (never inherited from Default)", got.Settings.MetaTitle)
	}

	// ApplyTo round-trips Label/MetaTitle/MetaDescription onto a live
	// display copy, same as the other Settings-bundle fields.
	applied := got.Settings.ApplyTo(entity.ViewDisplayDef{ID: 99, Label: "stale"})
	if applied.ID != 99 {
		t.Fatalf("ApplyTo must not touch identity fields other than Label/MetaTitle/MetaDescription, got ID %d", applied.ID)
	}
	if applied.Label != "sibling label" || applied.MetaTitle != "sibling meta title" {
		t.Fatalf("got Label/MetaTitle %q/%q after ApplyTo, want sibling label/sibling meta title", applied.Label, applied.MetaTitle)
	}
}

func TestHasUnpublishedChanges(t *testing.T) {
	repo := newFakeViewRepo()
	display := entity.ViewDisplayDef{ID: 1, OverrideSettings: true, OverrideFilters: true, OverrideSort: true, OverrideFields: true, PageSize: 10}
	repo.displays[display.ID] = display
	s := newTestViewService(repo)
	ctx := t.Context()

	has, err := s.HasUnpublishedChanges(ctx, display.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatal("expected no unpublished changes before any publish")
	}

	if _, err := s.PublishDisplay(ctx, display.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	has, err = s.HasUnpublishedChanges(ctx, display.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatal("expected no unpublished changes immediately after publish")
	}

	edited := repo.displays[display.ID]
	edited.PageSize = 11
	repo.displays[display.ID] = edited
	has, err = s.HasUnpublishedChanges(ctx, display.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Fatal("expected unpublished changes after a live edit")
	}
}

func TestDiscardDisplayChangesWithoutSnapshot(t *testing.T) {
	repo := newFakeViewRepo()
	display := entity.ViewDisplayDef{ID: 1, OverrideSettings: true, OverrideFilters: true, OverrideSort: true, OverrideFields: true}
	repo.displays[display.ID] = display
	s := newTestViewService(repo)

	if _, err := s.DiscardDisplayChanges(t.Context(), display.ID); !errors.Is(err, ErrNoPublishedSnapshot) {
		t.Fatalf("got error %v, want ErrNoPublishedSnapshot", err)
	}
}

func TestDiscardDisplayChangesRestoresSnapshot(t *testing.T) {
	repo := newFakeViewRepo()
	display := entity.ViewDisplayDef{ID: 1, OverrideSettings: true, OverrideFilters: true, OverrideSort: true, OverrideFields: true, PageSize: 10}
	repo.displays[display.ID] = display
	repo.filters[1] = entity.ViewFilterDef{ID: 1, ViewDisplayID: display.ID, BuiltinField: entity.ViewFilterBuiltinTitle, Operator: entity.ViewFilterOperatorEq, Value: "published-value"}
	s := newTestViewService(repo)
	ctx := t.Context()

	if _, err := s.PublishDisplay(ctx, display.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Live-edit after publishing: change Settings, replace the filter,
	// flip the Override flags off (mimicking RevertSettings/RevertFilters).
	edited := repo.displays[display.ID]
	edited.PageSize = 999
	edited.OverrideSettings = false
	edited.OverrideFilters = false
	repo.displays[display.ID] = edited
	delete(repo.filters, 1)
	repo.filters[2] = entity.ViewFilterDef{ID: 2, ViewDisplayID: display.ID, BuiltinField: entity.ViewFilterBuiltinTitle, Operator: entity.ViewFilterOperatorEq, Value: "live-only-value"}

	restored, err := s.DiscardDisplayChanges(ctx, display.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restored.PageSize != 10 {
		t.Fatalf("got PageSize %d, want 10 (restored from snapshot)", restored.PageSize)
	}
	if !restored.OverrideSettings || !restored.OverrideFilters || !restored.OverrideSort || !restored.OverrideFields {
		t.Fatalf("expected all 4 Override flags forced true, got %+v", restored)
	}
	filters, _ := repo.FiltersByViewID(ctx, display.ID)
	if len(filters) != 1 || filters[0].Value != "published-value" {
		t.Fatalf("got filters %+v, want exactly the snapshot's own (published-value)", filters)
	}

	// DiscardDisplayChanges recreates rows with new IDs; HasUnpublishedChanges
	// must compare values, not raw structs, or the display would look
	// changed forever after a discard.
	has, err := s.HasUnpublishedChanges(ctx, display.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatal("expected no unpublished changes immediately after discard restores the published values")
	}
}

func TestDiscardDisplayChangesRollsBackOnFailure(t *testing.T) {
	repo := newFakeViewRepo()
	display := entity.ViewDisplayDef{ID: 1, OverrideSettings: true, OverrideFilters: true, OverrideSort: true, OverrideFields: true, PageSize: 10}
	repo.displays[display.ID] = display
	repo.filters[1] = entity.ViewFilterDef{ID: 1, ViewDisplayID: display.ID, BuiltinField: entity.ViewFilterBuiltinTitle, Operator: entity.ViewFilterOperatorEq, Value: "a"}
	repo.filters[2] = entity.ViewFilterDef{ID: 2, ViewDisplayID: display.ID, BuiltinField: entity.ViewFilterBuiltinTitle, Operator: entity.ViewFilterOperatorEq, Value: "b"}
	s := newTestViewService(repo)
	ctx := t.Context()

	if _, err := s.PublishDisplay(ctx, display.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Live-edit after publishing: this is the state Discard must leave
	// untouched if its own transaction fails partway through.
	edited := repo.displays[display.ID]
	edited.PageSize = 555
	repo.displays[display.ID] = edited
	beforeDisplay := repo.displays[display.ID]
	beforeFilterCount := len(repo.filters)

	// PublishDisplay never calls CreateFilter, so discard's own
	// recreate loop is the first caller - fail its very first row.
	repo.createFilterCalls = 0
	repo.failCreateFilterAfter = 1

	if _, err := s.DiscardDisplayChanges(ctx, display.ID); err == nil {
		t.Fatal("expected the injected CreateFilter failure to propagate")
	}

	if got := repo.displays[display.ID]; got.PageSize != beforeDisplay.PageSize {
		t.Fatalf("got PageSize %d after rollback, want %d (unchanged)", got.PageSize, beforeDisplay.PageSize)
	}
	if len(repo.filters) != beforeFilterCount {
		t.Fatalf("got %d filters after rollback, want %d (unchanged)", len(repo.filters), beforeFilterCount)
	}
}

func TestDecodePublishedSnapshot(t *testing.T) {
	s := &DefaultService{}

	t.Run("never published", func(t *testing.T) {
		_, err := s.DecodePublishedSnapshot(entity.ViewDisplayDef{})
		if !errors.Is(err, ErrNoPublishedSnapshot) {
			t.Fatalf("got %v, want ErrNoPublishedSnapshot", err)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		now := time.Now()
		_, err := s.DecodePublishedSnapshot(entity.ViewDisplayDef{PublishedAt: &now, PublishedSnapshot: "{not json"})
		if err == nil {
			t.Fatal("expected an error for malformed JSON")
		}
	})

	t.Run("round trip", func(t *testing.T) {
		now := time.Now()
		want := entity.ViewSnapshot{Settings: entity.ViewSnapshotSettings{PageSize: 30, HeaderText: "hi"}}
		raw, marshalErr := json.Marshal(want)
		if marshalErr != nil {
			t.Fatalf("unexpected marshal error: %v", marshalErr)
		}
		got, decodeErr := s.DecodePublishedSnapshot(entity.ViewDisplayDef{PublishedAt: &now, PublishedSnapshot: string(raw)})
		if decodeErr != nil {
			t.Fatalf("unexpected decode error: %v", decodeErr)
		}
		if got.Settings.PageSize != 30 || got.Settings.HeaderText != "hi" {
			t.Fatalf("got %+v, want round-tripped %+v", got, want)
		}
	})
}
