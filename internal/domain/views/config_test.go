package views

import (
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestPublishedConfig - once a display is published, visitors' runs (a
// block's, a term page's, a profile tab's - every Run* and Glossary) use
// its published version, the admin's live edits made since aside; the
// live configuration (the preview) has them. A display never published,
// or one whose published version is unreadable, runs live.
func TestPublishedConfig(t *testing.T) {
	repo := newFakeViewRepo()
	block := entity.ViewDisplayDef{ID: 1, DisplayType: entity.ViewDisplayTypeBlock, OverrideSettings: true, OverrideFilters: true, OverrideSort: true, OverrideFields: true, PageSize: 5, Label: "Latest"}
	repo.displays[block.ID] = block
	repo.filters[1] = entity.ViewFilterDef{ID: 1, ViewDisplayID: block.ID, BuiltinField: entity.ViewFilterBuiltinStatus, Operator: entity.ViewFilterOperatorEq, Value: "1"}
	s := newTestViewService(repo)
	ctx := t.Context()

	published, err := s.PublishDisplay(ctx, block.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Edits after publishing: a filter, a page size.
	repo.filters[2] = entity.ViewFilterDef{ID: 2, ViewDisplayID: block.ID, BuiltinField: entity.ViewFilterBuiltinPromote, Operator: entity.ViewFilterOperatorEq, Value: "1"}
	edited := *published
	edited.PageSize = 50
	repo.displays[block.ID] = edited

	cfg, err := s.PublishedConfig(ctx, edited)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Published || len(cfg.Filters) != 1 || cfg.View.PageSize != 5 || cfg.View.ID != block.ID {
		t.Errorf("published: %+v", cfg)
	}
	live, err := s.LiveConfig(ctx, edited)
	if err != nil {
		t.Fatal(err)
	}
	if live.Published || len(live.Filters) != 2 || live.View.PageSize != 50 {
		t.Errorf("live: %+v", live)
	}

	// The runs visitors' pages make: the published filter and page size.
	runs := map[string]func() error{
		"RunView":            func() error { _, _, err := s.RunView(ctx, edited, nil, 1); return err },
		"RunViewWithContext": func() error { _, _, err := s.RunViewWithContext(ctx, edited, 3, 4); return err },
		"RunViewForTerm":     func() error { _, _, err := s.RunViewForTerm(ctx, edited, 6, 1); return err },
		"RunViewForUser":     func() error { _, _, err := s.RunViewForUser(ctx, edited, 3, 1); return err },
	}
	for name, run := range runs {
		repo.runs = nil
		if err := run(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(repo.runs) != 1 || len(repo.runs[0].Filters) != 1 || repo.runs[0].PageSize != 5 {
			t.Errorf("%s ran %+v, want the published version", name, repo.runs)
		}
	}

	// Never published - live.
	fresh := entity.ViewDisplayDef{ID: 7, DisplayType: entity.ViewDisplayTypeBlock, OverrideSettings: true, OverrideFilters: true, OverrideSort: true, OverrideFields: true, PageSize: 9}
	repo.displays[fresh.ID] = fresh
	if cfg, err := s.PublishedConfig(ctx, fresh); err != nil || cfg.Published || cfg.View.PageSize != 9 {
		t.Errorf("never published: %+v %v", cfg, err)
	}
	// An unreadable published version - live.
	now := time.Now()
	broken := fresh
	broken.PublishedAt, broken.PublishedSnapshot = &now, "{not json"
	if cfg, err := s.PublishedConfig(ctx, broken); err != nil || cfg.Published || cfg.View.PageSize != 9 {
		t.Errorf("unreadable: %+v %v", cfg, err)
	}
}

// TestRunLeavesOutGoneFields - a criterion whose field is gone (a
// published version can name a field deleted since) is left out of the
// run instead of failing it; the rest apply.
func TestRunLeavesOutGoneFields(t *testing.T) {
	repo := newFakeViewRepo()
	s := newTestViewService(repo)
	s.contentTypeRepo = &fakeListTypes{storages: map[int]entity.FieldStorageDef{2: {ID: 2, MachineName: "field_kept", FieldType: entity.FieldTypeText}}}
	gone, kept := 1, 2
	cfg := Config{
		View: entity.ViewDisplayDef{ID: 1, PageSize: 10},
		Filters: entity.ViewFilterDefs{
			{FieldStorageID: &gone, Operator: entity.ViewFilterOperatorEq, Value: "x"},
			{FieldStorageID: &kept, Operator: entity.ViewFilterOperatorEq, Value: "y"},
			{BuiltinField: entity.ViewFilterBuiltinStatus, Operator: entity.ViewFilterOperatorEq, Value: "1"},
		},
		Sorts: entity.ViewSortDefs{{FieldStorageID: &gone, Direction: "desc"}, {BuiltinField: entity.ViewSortBuiltinCreatedAt, Direction: "desc"}},
	}
	if _, _, err := s.RunConfig(t.Context(), cfg, RunContext{}); err != nil {
		t.Fatal(err)
	}
	run := repo.runs[0]
	if len(run.Filters) != 2 || *run.Filters[0].FieldStorageID != kept || len(run.Sorts) != 1 || run.Sorts[0].Builtin != entity.ViewSortBuiltinCreatedAt {
		t.Errorf("the run: filters %+v, sorts %+v", run.Filters, run.Sorts)
	}
	if run.Page != 1 {
		t.Errorf("page %d, want 1 (none given)", run.Page)
	}
}
