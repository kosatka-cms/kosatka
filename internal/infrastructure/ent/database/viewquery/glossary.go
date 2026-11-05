package viewquery

import (
	"context"
	"sort"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
)

// GlossaryForView implements entity.ViewQuery.GlossaryForView.
// A nil glossaryFieldStorageID groups by Node.Title, otherwise by that
// generic text field's value.
func (e *Engine) GlossaryForView(ctx context.Context, filters entity.ViewFilterDefs, moduleFilters []entity.ViewRunModuleFilter, fieldStorages map[int]entity.FieldStorageDef, glossaryFieldStorageID *int, queryParams map[string]string) (entity.GlossaryData, error) {
	client := database.GetClient(ctx, e.client)
	ids, err := matchingNodeIDs(ctx, client, filters, moduleFilters, fieldStorages, queryParams)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return entity.GlossaryData{}, nil
	}
	var values map[int]string
	if glossaryFieldStorageID != nil {
		values, err = database.NewContentTypeRepository(e.client).FieldTextValuesForNodes(ctx, *glossaryFieldStorageID, ids)
	} else {
		values, err = nodeTitlesForIDs(ctx, client, ids)
	}
	if err != nil {
		return nil, err
	}
	return buildGlossaryFromValues(values), nil
}

// buildGlossaryFromValues is the pure core of GlossaryForView: groups
// values (titles or a text field's values) by their uppercased first rune.
// Takes a plain map so it can be tested without a DB.
func buildGlossaryFromValues(values map[int]string) entity.GlossaryData {
	counts := make(map[string]int)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		letter := strings.ToUpper(string([]rune(value)[0]))
		counts[letter]++
	}
	letters := make([]string, 0, len(counts))
	for l := range counts {
		letters = append(letters, l)
	}
	sort.Strings(letters)
	data := make(entity.GlossaryData, 0, len(letters))
	for _, l := range letters {
		data = append(data, entity.GlossaryItem{Letter: l, Count: counts[l]})
	}
	return data
}
