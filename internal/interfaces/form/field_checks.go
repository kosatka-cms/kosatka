package form

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// fieldCheck - how the content form checks a field type's value (raw -
// its submitted value; the resolved ones are f's): before - what's wrong
// before the field being required is checked (an upload, a date's parts,
// a limit, a user); own - the type checks being required itself (its
// value is rows, not raw); after - the form of a value given. Every
// field type is here (TestFieldTypesChecked).
type fieldCheck struct {
	before func(ctx context.Context, f *DynamicForm, field entity.FieldInstanceDef, raw string) string
	own    bool
	after  func(ctx context.Context, f *DynamicForm, field entity.FieldInstanceDef, raw string) string
}

var fieldChecks = map[entity.ContentTypeFieldType]fieldCheck{
	entity.FieldTypeText: {after: checkText},
	entity.FieldTypeNumber: {after: func(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
		return validateNumber(ctx, field, raw)
	}},
	entity.FieldTypeEmail: {after: func(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
		return validateEmail(ctx, field, raw)
	}},
	entity.FieldTypePhone: {after: func(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
		return validatePhone(ctx, field, raw)
	}},
	entity.FieldTypeDate: {before: checkDates, after: func(ctx context.Context, _ *DynamicForm, _ entity.FieldInstanceDef, raw string) string {
		msg, _ := validateDateYMD(raw, i18n.T(ctx, "Invalid date format (use YYYY-MM-DD)"))
		return msg
	}},
	entity.FieldTypeBool:     {},
	entity.FieldTypeList:     {after: checkList},
	entity.FieldTypeTaxonomy: {after: checkTerms},
	// Several references and images - comma-joined IDs (the search
	// widget, resolveImagesField).
	entity.FieldTypeReference: {after: checkIDs},
	entity.FieldTypeImage:     {before: checkUpload, after: checkIDs},
	entity.FieldTypeFile:      {before: checkUpload, after: checkIDs},
	entity.FieldTypeUser: {before: func(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
		return validateUserIDs(ctx, field, raw)
	}},
	entity.FieldTypeLink: {own: true, before: func(ctx context.Context, f *DynamicForm, field entity.FieldInstanceDef, _ string) string {
		return validateLinks(ctx, field, f.FieldLinks[field.ID])
	}},
	entity.FieldTypeBlocks: {own: true, before: checkBlocks},
	entity.FieldTypeGroup:  {},
}

// checkUpload - what was wrong with an image's or a file's upload
// (FieldUploadErrors: extensions, size, resolution).
func checkUpload(_ context.Context, f *DynamicForm, field entity.FieldInstanceDef, _ string) string {
	return f.FieldUploadErrors[field.ID]
}

// checkDates - what's wrong with a date's parts (FieldDateErrors) and,
// several dates, how many (each checked by resolveDates).
func checkDates(ctx context.Context, f *DynamicForm, field entity.FieldInstanceDef, _ string) string {
	if msg, failed := f.FieldDateErrors[field.ID]; failed {
		return msg
	}
	if field.DateList() {
		if limit := field.MaxValues(); limit > 0 && len(f.FieldDates[field.ID]) > limit {
			return i18n.Plural(ctx, limit, "@field: at most @count date.", "@field: at most @count dates.", "@field", field.Label)
		}
	}
	return ""
}

// checkBlocks - a blocks field's blocks: one at least, if required, and
// no more than its limit.
func checkBlocks(ctx context.Context, f *DynamicForm, field entity.FieldInstanceDef, _ string) string {
	n := len(f.FieldBlocks[field.ID])
	switch {
	case n == 0 && field.Required && !field.HiddenInForm:
		return i18n.T(ctx, "@field is required", "@field", field.Label)
	case field.MaxValues() > 0 && n > field.MaxValues():
		return i18n.Plural(ctx, field.MaxValues(), "@field: at most @count block.", "@field: at most @count blocks.", "@field", field.Label)
	}
	return ""
}

// checkText - a text's length (only an explicit max length: none -
// unlimited) and its pattern (FieldInstanceDef.Pattern; an invalid
// stored pattern is no constraint).
func checkText(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
	if field.HasExplicitMaxLen() {
		maxLen := field.MaxLen()
		if msg, failed := validateMaxLen(raw, maxLen, i18n.Plural(ctx, maxLen, "@field must not exceed @count character", "@field must not exceed @count characters", "@field", field.Label)); failed {
			return msg
		}
	}
	if raw == "" {
		return ""
	}
	if pattern := field.Pattern(); pattern != "" {
		if re, err := regexp.Compile(pattern); err == nil && !re.MatchString(raw) {
			if msg := field.PatternMessage(); msg != "" {
				return msg
			}
			return i18n.T(ctx, "@field: invalid format", "@field", field.Label)
		}
	}
	return ""
}

// checkList - a list's values among its AllowedValues; several arrive
// comma-joined (resolveListValues), so a configured value has no
// comma.
func checkList(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
	values := []string{raw}
	if field.Multiple() {
		values = strings.Split(raw, ",")
	}
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" && !fieldAllowedValuesContain(field, v) {
			return i18n.T(ctx, "Illegal value")
		}
	}
	return ""
}

// checkTerms - a term's ID or, several, comma-joined term IDs and
// "new:<name>" tags (as parseTagsInput reads them).
func checkTerms(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
	if !field.Multiple() {
		return checkIDs(ctx, nil, field, raw)
	}
	for _, tok := range strings.Split(raw, ",") {
		if tok = strings.TrimSpace(tok); tok == "" || strings.HasPrefix(tok, "new:") {
			continue
		}
		if _, err := strconv.Atoi(tok); err != nil {
			return i18n.T(ctx, "Invalid value")
		}
	}
	return ""
}

// checkIDs - an ID or, a field with several values, comma-joined IDs.
func checkIDs(ctx context.Context, _ *DynamicForm, field entity.FieldInstanceDef, raw string) string {
	if !field.Multiple() {
		if raw == "" {
			return ""
		}
		if _, err := strconv.Atoi(raw); err != nil {
			return i18n.T(ctx, "Invalid value")
		}
		return ""
	}
	for _, tok := range strings.Split(raw, ",") {
		if tok = strings.TrimSpace(tok); tok == "" {
			continue
		}
		if _, err := strconv.Atoi(tok); err != nil {
			return i18n.T(ctx, "Invalid value")
		}
	}
	return ""
}
