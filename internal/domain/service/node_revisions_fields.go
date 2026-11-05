package service

import (
	"context"
	"fmt"
	"path"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// revisionFieldType - how a node's history shows and brings back the
// values of a field type: items - a value as a comparison's items (nil -
// its text); names - the names of what its values name by ID (terms,
// content, files, users) that still exist (nil - it names nothing:
// reverting keeps its values as they were). Every field type is here
// (TestFieldTypesInRevisions).
type revisionFieldType struct {
	items func(ctx context.Context, s *NodeRevisionService, f entity.FieldInstanceDef, v any) ([]RevisionItem, error)
	names nameFunc
}

var revisionFieldTypes = map[entity.ContentTypeFieldType]revisionFieldType{
	// Texts compare by their paragraphs and words (textLines).
	entity.FieldTypeText:  {},
	entity.FieldTypeEmail: {},
	entity.FieldTypePhone: {},
	entity.FieldTypeNumber: {items: func(_ context.Context, _ *NodeRevisionService, _ entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
		return []RevisionItem{{Text: strconv.FormatFloat(v.(float64), 'f', -1, 64)}}, nil
	}},
	entity.FieldTypeBool: {items: func(ctx context.Context, _ *NodeRevisionService, _ entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
		return []RevisionItem{{Text: map[bool]string{true: i18n.T(ctx, "Yes"), false: i18n.T(ctx, "No")}[v.(bool)]}}, nil
	}},
	// Each date; a range - its start and its end.
	entity.FieldTypeDate: {items: func(_ context.Context, _ *NodeRevisionService, _ entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
		var out []RevisionItem
		for _, d := range entity.DateValues(v) {
			out = append(out, RevisionItem{Time: d.Start})
			if d.HasEnd() {
				out = append(out, RevisionItem{Time: d.End})
			}
		}
		return out, nil
	}},
	entity.FieldTypeLink: {items: func(_ context.Context, _ *NodeRevisionService, _ entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
		var out []RevisionItem
		for _, l := range entity.Links(v) {
			text := l.URL
			if l.Title != "" {
				text = l.Title + " - " + l.URL
			}
			out = append(out, RevisionItem{Text: text})
		}
		return out, nil
	}},
	entity.FieldTypeList: {items: func(_ context.Context, _ *NodeRevisionService, f entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
		var out []RevisionItem
		for _, value := range listValues(v) {
			out = append(out, RevisionItem{Text: f.AllowedValueLabel(value)})
		}
		return out, nil
	}},
	entity.FieldTypeTaxonomy:  {items: idItems(termNames), names: termNames},
	entity.FieldTypeReference: {items: idItems(nodeTitles), names: nodeTitles},
	entity.FieldTypeUser:      {items: idItems(userNames), names: userNames},
	entity.FieldTypeImage:     {items: idItems(fileNames), names: fileNames},
	entity.FieldTypeFile:      {items: idItems(fileNames), names: fileNames},
	// Each block: its set and its summary (its first texts); reverting
	// makes them anew (revertBlocks).
	entity.FieldTypeBlocks: {items: func(ctx context.Context, s *NodeRevisionService, _ entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
		var out []RevisionItem
		for _, item := range entity.BlockItems(v) {
			def, setFields, ok, err := s.typeOf(ctx, item.Type)
			if err != nil {
				return nil, err
			}
			if !ok || item.Snapshot == nil {
				out = append(out, RevisionItem{Text: item.Type})
				continue
			}
			iv, _ := item.Snapshot.Values(setFields)
			text := def.Label
			if summary := (entity.BlockItem{Fields: setFields.SortBySortOrder(), Values: iv}).Summary(); summary != "" {
				text += ": " + summary
			}
			out = append(out, RevisionItem{Text: text})
		}
		return out, nil
	}},
	entity.FieldTypeGroup: {},
}

// nameFunc - the names of the things ids name that still exist.
type nameFunc = func(ctx context.Context, s *NodeRevisionService, ids []int) (map[int]string, error)

// idItems - the items of a value naming things by ID: their names
// (names), "#<id>" for one that's gone.
func idItems(names nameFunc) func(ctx context.Context, s *NodeRevisionService, f entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
	return func(ctx context.Context, s *NodeRevisionService, _ entity.FieldInstanceDef, v any) ([]RevisionItem, error) {
		ids := idValues(v)
		found, err := names(ctx, s, ids)
		if err != nil {
			return nil, err
		}
		out := make([]RevisionItem, 0, len(ids))
		for _, id := range ids {
			out = append(out, RevisionItem{Text: cmp(found[id], "#"+strconv.Itoa(id))})
		}
		return out, nil
	}
}

// termNames - the names of the terms ids that still exist.
func termNames(ctx context.Context, s *NodeRevisionService, ids []int) (map[int]string, error) {
	terms, err := s.d.Terms.TermsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("loading terms %v: %w", ids, err)
	}
	out := map[int]string{}
	for id, term := range terms {
		out[id] = term.Name
	}
	return out, nil
}

// nodeTitles - the titles of the content ids that still exists.
func nodeTitles(ctx context.Context, s *NodeRevisionService, ids []int) (map[int]string, error) {
	nodes, err := s.d.Nodes.ByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("loading nodes %v: %w", ids, err)
	}
	out := map[int]string{}
	for id, n := range nodes {
		out[id] = n.Title
	}
	return out, nil
}

// userNames - the names of the users ids that still exist.
func userNames(ctx context.Context, s *NodeRevisionService, ids []int) (map[int]string, error) {
	users, err := s.d.Users.ByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("loading users %v: %w", ids, err)
	}
	out := map[int]string{}
	for id, u := range users {
		out[id] = u.Username
	}
	return out, nil
}

// fileNames - the names of the files ids that still exist.
func fileNames(ctx context.Context, s *NodeRevisionService, ids []int) (map[int]string, error) {
	files, err := s.d.Files.ByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("loading files %v: %w", ids, err)
	}
	out := map[int]string{}
	for id, f := range files {
		out[id] = cmp(f.OriginalFilename, path.Base(f.URI))
	}
	return out, nil
}

// isIDList - a value of several IDs (one - an int).
func isIDList(v any) bool {
	_, ok := v.([]int)
	return ok
}
