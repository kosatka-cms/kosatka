package views

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/fallback"
)

// runPlan - a View's filters and sorts as the query runs them: the field
// storages they reference, the core's filters, the modules' filters
// (their modules on) and the sorts in order, a module's whose module is
// off left out, and so is a criterion whose field is gone (a published
// snapshot can name a field deleted since).
type runPlan struct {
	fieldStorages map[int]entity.FieldStorageDef
	filters       entity.ViewFilterDefs
	moduleFilters []entity.ViewRunModuleFilter
	sorts         []entity.ViewRunSort
}

// planRun resolves filters and sorts (sorted by SortOrder already) for
// the query (runPlan).
func (s *DefaultService) planRun(ctx context.Context, filters entity.ViewFilterDefs, sorts entity.ViewSortDefs) (runPlan, error) {
	plan := runPlan{fieldStorages: map[int]entity.FieldStorageDef{}}
	// load - the field storage id into the plan; false - it is gone (the
	// criterion is left out, logged).
	load := func(id int) (bool, error) {
		if _, ok := plan.fieldStorages[id]; ok {
			return true, nil
		}
		fs, err := s.contentTypeRepo.FieldStorageByID(ctx, id)
		if errors.Is(err, entity.ErrEntityNotFound) {
			fallback.Log(s.logger, err, "a View criterion's field is gone (the criterion left out)", "fieldStorageID", id)
			return false, nil
		}
		if err != nil {
			return false, err
		}
		plan.fieldStorages[id] = *fs
		return true, nil
	}
	for _, f := range filters {
		switch {
		case IsModuleKey(string(f.BuiltinField)):
			if mf, ok := ModuleFilter(string(f.BuiltinField)); ok {
				plan.moduleFilters = append(plan.moduleFilters, entity.ViewRunModuleFilter{Def: f, Filter: mf})
			}
		case f.FieldStorageID != nil:
			ok, err := load(*f.FieldStorageID)
			if err != nil {
				return runPlan{}, err
			}
			if ok {
				plan.filters = append(plan.filters, f)
			}
		default:
			plan.filters = append(plan.filters, f)
		}
	}
	for _, sd := range sorts {
		rs := entity.ViewRunSort{Builtin: sd.BuiltinField, Direction: sd.Direction}
		switch {
		case sd.FieldStorageID != nil:
			ok, err := load(*sd.FieldStorageID)
			if err != nil {
				return runPlan{}, err
			}
			if !ok {
				continue
			}
			fs := plan.fieldStorages[*sd.FieldStorageID]
			rs.FieldStorage = &fs
		case IsModuleKey(string(sd.BuiltinField)):
			ms, ok := ModuleSort(string(sd.BuiltinField))
			if !ok {
				continue
			}
			rs.Module = &ms
			rs.Params = entity.ViewModuleParams(ms.Params, sd.Params)
		}
		plan.sorts = append(plan.sorts, rs)
	}
	return plan, nil
}

// glossaryLetterFilter builds the synthetic (never persisted)
// ViewFilterDef that RunView appends when ShowGlossary is on and the
// request has a "letter" query param. It bypasses validateFilter.
// Targets view.GlossaryFieldStorageID when set, else the title builtin.
func glossaryLetterFilter(view entity.ViewDisplayDef, letter string) entity.ViewFilterDef {
	f := entity.ViewFilterDef{
		Operator: entity.ViewFilterOperatorStartsWith,
		Value:    strings.ToUpper(strings.TrimSpace(letter)),
	}
	if view.GlossaryFieldStorageID != nil {
		f.FieldStorageID = view.GlossaryFieldStorageID
	} else {
		f.BuiltinField = entity.ViewFilterBuiltinTitle
	}
	return f
}

// RunView - see Service.RunView.
func (s *DefaultService) RunView(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string, page int) (entity.Nodes, int, error) {
	return s.runPublished(ctx, view, RunContext{QueryParams: queryParams, Page: page})
}

// RunViewForPage - see Service.RunViewForPage.
func (s *DefaultService) RunViewForPage(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string, page, contextUserID int) (entity.Nodes, int, error) {
	return s.runPublished(ctx, view, RunContext{QueryParams: queryParams, Page: page, CreatedByID: contextUserID})
}

// ViewByURLArgument - see the Service interface's doc comment.
func (s *DefaultService) ViewByURLArgument(ctx context.Context, path string) (*entity.ViewDisplayDef, string, error) {
	views, err := s.repository.PageDisplaysWithArgument(ctx)
	if err != nil {
		return nil, "", err
	}
	for i := range views {
		if arg, ok := entity.MatchURLArgument(views[i].URLPath, path); ok {
			return &views[i], arg, nil
		}
	}
	return nil, "", fmt.Errorf("%w: no view matches %q", entity.ErrEntityNotFound, path)
}

// RunViewWithContext - see Service. No QueryParams (a block has no
// request query string for exposed filters), always page 1 (blocks don't
// paginate).
func (s *DefaultService) RunViewWithContext(ctx context.Context, view entity.ViewDisplayDef, contextCreatedByID, contextNodeID int) (entity.Nodes, int, error) {
	return s.runPublished(ctx, view, RunContext{Page: 1, CreatedByID: contextCreatedByID, NodeID: contextNodeID})
}

// RunViewForTerm - see Service. No QueryParams, same as
// RunViewWithContext.
func (s *DefaultService) RunViewForTerm(ctx context.Context, view entity.ViewDisplayDef, termID, page int) (entity.Nodes, int, error) {
	return s.runPublished(ctx, view, RunContext{Page: page, TermID: termID})
}

// RunViewForUser - see Service.
func (s *DefaultService) RunViewForUser(ctx context.Context, view entity.ViewDisplayDef, userID, page int) (entity.Nodes, int, error) {
	return s.runPublished(ctx, view, RunContext{Page: page, CreatedByID: userID})
}

func (s *DefaultService) runView(ctx context.Context, effective entity.ViewDisplayDef, filters entity.ViewFilterDefs, sorts entity.ViewSortDefs, queryParams map[string]string, page, contextCreatedByID, contextNodeID, contextTermID int) (entity.Nodes, int, error) {
	if effective.ShowGlossary {
		if letter := queryParams["letter"]; letter != "" {
			filters = append(filters, glossaryLetterFilter(effective, letter))
		}
	}

	plan, err := s.planRun(ctx, filters, sorts.SortBySortOrder().WithChoice(queryParams[entity.ViewSortParam]))
	if err != nil {
		return nil, 0, fmt.Errorf("loading view field storage (viewID %v): %w", effective.ID, err)
	}

	nodes, total, err := s.query.RunView(ctx, entity.ViewRunParams{
		Filters:            plan.filters,
		ModuleFilters:      plan.moduleFilters,
		FieldStorages:      plan.fieldStorages,
		Sorts:              plan.sorts,
		QueryParams:        queryParams,
		PageSize:           effective.PageSize,
		Page:               page,
		Skip:               effective.SkipItems,
		ContextCreatedByID: contextCreatedByID,
		ContextNodeID:      contextNodeID,
		ContextTermID:      contextTermID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("running view (viewID %v): %w", effective.ID, err)
	}
	return nodes, total, nil
}

// RunCommentView - RunView for a base_entity=="comment" display: no
// glossary and no field storages (comments have none); resolves
// Filters/Sorts and delegates to the repository.
func (s *DefaultService) RunCommentView(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string, page int) (entity.Comments, int, error) {
	return s.runPublishedComments(ctx, view, RunContext{QueryParams: queryParams, Page: page})
}

// RunCommentViewForUser - RunCommentView on the profile of userID: the
// comment_created_by contextual filter resolves to userID.
func (s *DefaultService) RunCommentViewForUser(ctx context.Context, view entity.ViewDisplayDef, userID, page int) (entity.Comments, int, error) {
	return s.runPublishedComments(ctx, view, RunContext{Page: page, CreatedByID: userID})
}

func (s *DefaultService) runCommentView(ctx context.Context, effective entity.ViewDisplayDef, filters entity.ViewFilterDefs, sorts entity.ViewSortDefs, queryParams map[string]string, page, contextCreatedByID int) (entity.Comments, int, error) {
	sorts = sorts.SortBySortOrder().WithChoice(queryParams[entity.ViewSortParam])
	runSorts := make([]entity.ViewRunSort, len(sorts))
	for i, sd := range sorts {
		runSorts[i] = entity.ViewRunSort{Builtin: sd.BuiltinField, Direction: sd.Direction}
	}

	comments, total, err := s.query.RunCommentView(ctx, entity.ViewRunParams{
		Filters:            filters,
		Sorts:              runSorts,
		QueryParams:        queryParams,
		PageSize:           effective.PageSize,
		Page:               page,
		Skip:               effective.SkipItems,
		ContextCreatedByID: contextCreatedByID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("running comment view (viewID %v): %w", effective.ID, err)
	}
	return comments, total, nil
}

// Glossary computes a view's letter-index bar data
// (entity.ViewDisplayDef.ShowGlossary) as visitors see the display: the
// same filters RunView would apply except the letter constraint itself,
// so every letter stays clickable while other active filters still narrow
// the counts.
func (s *DefaultService) Glossary(ctx context.Context, view entity.ViewDisplayDef, queryParams map[string]string) (entity.GlossaryData, error) {
	cfg, err := s.publishedConfig(ctx, view, false)
	if err != nil {
		return nil, err
	}
	return s.GlossaryOf(ctx, cfg, queryParams)
}

func (s *DefaultService) glossary(ctx context.Context, effective entity.ViewDisplayDef, filters entity.ViewFilterDefs, queryParams map[string]string) (entity.GlossaryData, error) {
	plan, err := s.planRun(ctx, filters, nil)
	if err != nil {
		return nil, fmt.Errorf("loading view field storage for glossary (viewID %v): %w", effective.ID, err)
	}
	fieldStorages := plan.fieldStorages
	// The letter filter is never added above, so make sure the glossary
	// target's FieldStorageDef is resolved anyway.
	if effective.GlossaryFieldStorageID != nil {
		if _, ok := fieldStorages[*effective.GlossaryFieldStorageID]; !ok {
			fs, err := s.contentTypeRepo.FieldStorageByID(ctx, *effective.GlossaryFieldStorageID)
			if errors.Is(err, entity.ErrEntityNotFound) {
				// Its field is gone (a published snapshot's): no bar.
				fallback.Log(s.logger, err, "a View's glossary field is gone (no glossary shown)", "viewID", effective.ID)
				return entity.GlossaryData{}, nil
			}
			if err != nil {
				return nil, fmt.Errorf("loading glossary target field storage (viewID %v): %w", effective.ID, err)
			}
			fieldStorages[*effective.GlossaryFieldStorageID] = *fs
		}
	}
	data, err := s.query.GlossaryForView(ctx, plan.filters, plan.moduleFilters, fieldStorages, effective.GlossaryFieldStorageID, queryParams)
	return entity.Wrapped(data, err, "computing view glossary (viewID %v)", effective.ID)
}

// PreviewView - see Service.PreviewView.
func (s *DefaultService) PreviewView(ctx context.Context, view entity.ViewDisplayDef, settings PreviewSettings) (entity.Nodes, int, error) {
	effective, err := s.EffectiveSettings(ctx, view)
	if err != nil {
		return nil, 0, fmt.Errorf("resolving effective settings (viewID %v): %w", view.ID, err)
	}
	filters, err := s.EffectiveFiltersByView(ctx, view)
	if err != nil {
		return nil, 0, fmt.Errorf("loading view filters (viewID %v): %w", view.ID, err)
	}
	if effective.ShowGlossary && settings.Letter != "" {
		filters = append(filters, glossaryLetterFilter(effective, settings.Letter))
	}
	sorts, err := s.EffectiveSortsByView(ctx, view)
	if err != nil {
		return nil, 0, fmt.Errorf("loading view sorts (viewID %v): %w", view.ID, err)
	}

	plan, err := s.planRun(ctx, filters, sorts.SortBySortOrder().WithChoice(settings.ExposedFilterValues[entity.ViewSortParam]))
	if err != nil {
		return nil, 0, fmt.Errorf("loading view field storage (viewID %v): %w", view.ID, err)
	}

	pageSize := settings.PageSize
	if pageSize <= 0 {
		pageSize = effective.PageSize
	}
	page := settings.Page
	if page < 1 {
		page = 1
	}

	nodes, total, err := s.query.RunView(ctx, entity.ViewRunParams{
		Filters:       plan.filters,
		ModuleFilters: plan.moduleFilters,
		FieldStorages: plan.fieldStorages,
		Sorts:         plan.sorts,
		QueryParams:   settings.ExposedFilterValues,
		PageSize:      pageSize,
		Page:          page,
		Skip:          settings.SkipItems,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("previewing view (viewID %v): %w", view.ID, err)
	}
	return nodes, total, nil
}
