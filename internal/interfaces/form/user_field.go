package form

import (
	"context"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// ParseIDList - the positive IDs of a comma-separated list (a picker's
// hidden input), in order, each once.
func ParseIDList(raw string) []int {
	var ids []int
	seen := map[int]bool{}
	for _, tok := range strings.Split(raw, ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(tok)); err == nil && id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// validateUserIDs - what's wrong with a user field's raw value ("" -
// nothing): not a list of IDs, more users than the field holds. That the
// users exist and have the field's roles - the content form's check
// (it reads the users).
func validateUserIDs(ctx context.Context, field entity.FieldInstanceDef, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	ids := ParseIDList(raw)
	if len(ids) == 0 {
		return i18n.T(ctx, "Illegal value")
	}
	limit := 1
	if field.Multiple() {
		limit = field.MaxValues()
	}
	if limit > 0 && len(ids) > limit {
		return i18n.Plural(ctx, limit, "@field: at most @count person.", "@field: at most @count people.", "@field", field.Label)
	}
	return ""
}
