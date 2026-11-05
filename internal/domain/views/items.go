package views

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// FiltersByViewID - the display viewID's own filters, in their order
// (EffectiveFiltersByView - the ones it applies).
func (s *DefaultService) FiltersByViewID(ctx context.Context, viewID int) (entity.ViewFilterDefs, error) {
	filters, err := s.repository.FiltersByViewID(ctx, viewID)
	return entity.Wrapped(filters, err, "listing view filters (viewID %v)", viewID)
}

// FilterByID - the filter with the given id.
func (s *DefaultService) FilterByID(ctx context.Context, id int) (*entity.ViewFilterDef, error) {
	f, err := s.repository.FilterByID(ctx, id)
	return entity.Found(f, err, "finding view filter by id (id %v)", id)
}

// exposedParamRe - an exposed filter's query parameter: lowercase latin
// letters, digits and _, from a letter.
var exposedParamRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// checkExposedParam - f's parameter given, of exposedParamRe's shape, not
// one a View page reads itself, and no other exposed filter's of the
// display (both would read one value).
func (s *DefaultService) checkExposedParam(ctx context.Context, f entity.ViewFilterDef) error {
	switch p := f.ExposedParam; {
	case p == "":
		return ErrFilterExposedParamRequired
	case !exposedParamRe.MatchString(p), p == entity.ViewPageParam, p == entity.ViewLetterParam, p == entity.ViewSortParam:
		return fmt.Errorf("%w: %q", ErrFilterExposedParamInvalid, p)
	}
	others, err := s.repository.FiltersByViewID(ctx, f.ViewDisplayID)
	if err != nil {
		return fmt.Errorf("listing the display's filters (viewDisplayID %v): %w", f.ViewDisplayID, err)
	}
	for _, o := range others {
		if o.ID != f.ID && o.Exposed && o.ExposedParam == f.ExposedParam {
			return fmt.Errorf("%w: %q", ErrFilterExposedParamTaken, f.ExposedParam)
		}
	}
	return nil
}

// validateFilter - CreateFilter/UpdateFilter checks: exactly one of
// FieldStorageID/BuiltinField, Operator valid for the target,
// ExposedParam valid when Exposed (checkExposedParam), valid Contextual
// usage.
func (s *DefaultService) validateFilter(ctx context.Context, f entity.ViewFilterDef) error {
	if f.Exposed {
		if err := s.checkExposedParam(ctx, f); err != nil {
			return err
		}
	}
	if f.FieldStorageID == nil && IsModuleKey(string(f.BuiltinField)) {
		return validateModuleFilter(f)
	}
	if f.Contextual {
		if f.Exposed {
			return ErrFilterContextualExposedConflict
		}
		if f.FieldStorageID == nil && f.BuiltinField != entity.ViewFilterBuiltinCreatedBy && f.BuiltinField != entity.ViewFilterBuiltinCommentCreatedBy && f.BuiltinField != entity.ViewFilterBuiltinSimilarToCurrentNode && f.BuiltinField != entity.ViewFilterBuiltinSharesTermsWithCurrentNode && f.BuiltinField != entity.ViewFilterBuiltinHasCurrentTerm {
			return ErrFilterContextualUnsupported
		}
	}
	// has_current_term reads the term from the page being rendered -
	// Contextual only.
	if f.BuiltinField == entity.ViewFilterBuiltinHasCurrentTerm && !f.Contextual {
		return ErrFilterContextualUnsupported
	}
	// shares_terms_with_current and similar_to_current_node only make sense
	// as Contextual - their Value is a vocabulary scope, not a comparison
	// value.
	if f.BuiltinField == entity.ViewFilterBuiltinSharesTermsWithCurrentNode || f.BuiltinField == entity.ViewFilterBuiltinSimilarToCurrentNode {
		if !f.Contextual {
			return ErrFilterContextualUnsupported
		}
		if v := strings.TrimSpace(f.Value); v != "" {
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("%w: vocabulary ID must be a number, got %q", ErrFilterOperatorUnsupported, v)
			}
		}
	}

	if f.ViaFieldStorageID != nil {
		if err := s.validateFilterVia(ctx, f); err != nil {
			return err
		}
	}
	var allowed []entity.ViewFilterOperator
	if f.FieldStorageID != nil {
		fs, err := s.contentTypeRepo.FieldStorageByID(ctx, *f.FieldStorageID)
		if err != nil {
			return err
		}
		if !FilterableFieldType(fs.FieldType) {
			return fmt.Errorf("%w: %q", ErrFilterFieldTypeUnsupported, fs.FieldType)
		}
		// A field filter is Contextual if and only if it targets a reference
		// field (its operators read the current node, not Value) or names
		// the profile's user in a user field.
		if f.Contextual != (fs.FieldType == entity.FieldTypeReference || f.Operator == entity.ViewFilterOperatorIsProfileUser) {
			return ErrFilterContextualUnsupported
		}
		allowed = validOperatorsFor(fs.FieldType)
		if f.ValueIsRelative && fs.FieldType != entity.FieldTypeDate {
			return fmt.Errorf("%w: relative dates only apply to date fields, got %q", ErrFilterFieldTypeUnsupported, fs.FieldType)
		}
	} else {
		allowed = validOperatorsForBuiltin(f.BuiltinField)
		if f.ValueIsRelative {
			return fmt.Errorf("%w: relative dates only apply to date fields", ErrFilterFieldTypeUnsupported)
		}
	}
	if !slices.Contains(allowed, f.Operator) {
		return fmt.Errorf("%w: %q", ErrFilterOperatorUnsupported, f.Operator)
	}
	// Reject a malformed relative date expression at save time (see
	// entity.ParseRelativeDate).
	if f.ValueIsRelative {
		if _, err := entity.ParseRelativeDate(f.Value, time.Now()); err != nil {
			return fmt.Errorf("%w: %w", ErrFilterInvalidRelativeDate, err)
		}
	}
	return nil
}

// validateFilterVia - a filter in a blocks field's blocks
// (ViaFieldStorageID): a blocks field, the filtered field one of a
// field set of it.
func (s *DefaultService) validateFilterVia(ctx context.Context, f entity.ViewFilterDef) error {
	if f.FieldStorageID == nil {
		return fmt.Errorf("%w: a blocks field without a field of its blocks", ErrFilterFieldTypeUnsupported)
	}
	via, err := s.contentTypeRepo.FieldStorageByID(ctx, *f.ViaFieldStorageID)
	if err != nil {
		return err
	}
	if via.FieldType != entity.FieldTypeBlocks {
		return fmt.Errorf("%w: %q holds no blocks", ErrFilterFieldTypeUnsupported, via.FieldType)
	}
	for _, name := range via.BlockTypes() {
		set, err := s.contentTypeRepo.ContentTypeByMachineName(ctx, name)
		if err != nil {
			if entity.IsNotFound(err) {
				continue
			}
			return err
		}
		fields, err := s.contentTypeRepo.FieldsByContentTypeID(ctx, set.ID)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(fields, func(fi entity.FieldInstanceDef) bool { return fi.FieldStorageID == *f.FieldStorageID }) {
			return nil
		}
	}
	return fmt.Errorf("%w: the field isn't one of the blocks' sets", ErrFilterFieldTypeUnsupported)
}

// validateModuleFilter - validateFilter of a module's filter: its module
// on, a number compared (empty - no constraint), never contextual or a
// relative date.
func validateModuleFilter(f entity.ViewFilterDef) error {
	if _, ok := ModuleFilter(string(f.BuiltinField)); !ok {
		return fmt.Errorf("%w: %q", ErrFilterFieldTypeUnsupported, f.BuiltinField)
	}
	if f.Contextual {
		return ErrFilterContextualUnsupported
	}
	if f.ValueIsRelative {
		return fmt.Errorf("%w: relative dates only apply to date fields", ErrFilterFieldTypeUnsupported)
	}
	if mf, _ := ModuleFilter(string(f.BuiltinField)); mf.Choice() {
		if !slices.Contains(entity.ViewModuleChoiceOperators, f.Operator) {
			return fmt.Errorf("%w: %q", ErrFilterOperatorUnsupported, f.Operator)
		}
		return nil
	}
	if !slices.Contains(entity.ViewModuleFilterOperators, f.Operator) {
		return fmt.Errorf("%w: %q", ErrFilterOperatorUnsupported, f.Operator)
	}
	if v := strings.TrimSpace(f.Value); v != "" && !f.Exposed {
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return fmt.Errorf("%w: a number expected, got %q", ErrFilterOperatorUnsupported, v)
		}
	}
	return nil
}

// CreateFilter creates a display's filter once its target, operator,
// exposed parameter and contextual mode are valid (validateFilter).
func (s *DefaultService) CreateFilter(ctx context.Context, f entity.ViewFilterDef) (*entity.ViewFilterDef, error) {
	if err := s.validateFilter(ctx, f); err != nil {
		return nil, err
	}

	created, err := s.repository.CreateFilter(ctx, f)
	return entity.Wrapped(created, err, "creating view filter (viewDisplayID %v)", f.ViewDisplayID)
}

// UpdateFilter - a filter of a module that is off is saved as it is (its
// place in the list), not validated.
func (s *DefaultService) UpdateFilter(ctx context.Context, f entity.ViewFilterDef) (*entity.ViewFilterDef, error) {
	if _, on := ModuleFilter(string(f.BuiltinField)); on || !IsModuleKey(string(f.BuiltinField)) {
		if err := s.validateFilter(ctx, f); err != nil {
			return nil, err
		}
	}

	updated, err := s.repository.UpdateFilter(ctx, f)
	return entity.Wrapped(updated, err, "updating view filter (id %v)", f.ID)
}

// DeleteFilter deletes the filter with the given id.
func (s *DefaultService) DeleteFilter(ctx context.Context, id int) error {
	if err := s.repository.DeleteFilter(ctx, id); err != nil {
		return fmt.Errorf("deleting view filter (id %v): %w", id, err)
	}
	return nil
}

// SortsByViewID - the display viewID's own sort criteria, in their order
// (EffectiveSortsByView - the ones it applies).
func (s *DefaultService) SortsByViewID(ctx context.Context, viewID int) (entity.ViewSortDefs, error) {
	sorts, err := s.repository.SortsByViewID(ctx, viewID)
	return entity.Wrapped(sorts, err, "listing view sorts (viewID %v)", viewID)
}

// SortByID - the sort criterion with the given id.
func (s *DefaultService) SortByID(ctx context.Context, id int) (*entity.ViewSortDef, error) {
	sd, err := s.repository.SortByID(ctx, id)
	return entity.Found(sd, err, "finding view sort by id (id %v)", id)
}

// resolveSortTarget validates a ViewSortDef's target - a generic
// field (FieldStorageID set), a builtin or a module's criterion
// (BuiltinField set) - same either/or validation shape as
// resolveViewFieldTarget, and sets its Params (moduleSortParams).
func (s *DefaultService) resolveSortTarget(ctx context.Context, sd *entity.ViewSortDef) error {
	if sd.FieldStorageID == nil && IsModuleKey(string(sd.BuiltinField)) {
		if _, ok := ModuleSort(string(sd.BuiltinField)); !ok {
			return fmt.Errorf("%w: %q", ErrSortFieldTypeUnsupported, sd.BuiltinField)
		}
		moduleSortParams(sd)
		return nil
	}
	sd.Params = nil
	if sd.FieldStorageID != nil {
		fs, err := s.contentTypeRepo.FieldStorageByID(ctx, *sd.FieldStorageID)
		if err != nil {
			return err
		}
		if !SortableFieldType(fs.FieldType) {
			return fmt.Errorf("%w: %q", ErrSortFieldTypeUnsupported, fs.FieldType)
		}
		return nil
	}
	if !SortBuiltinAllowed(sd.BuiltinField) {
		return fmt.Errorf("%w: %q", ErrSortFieldTypeUnsupported, sd.BuiltinField)
	}
	return nil
}

// moduleSortParams sets the Params of a module's criterion to the values
// its params accept (entity.ViewModuleParams); its module off - kept as
// they are, to apply when it is on again. Others' - none.
func moduleSortParams(sd *entity.ViewSortDef) {
	if sd.FieldStorageID != nil || !IsModuleKey(string(sd.BuiltinField)) {
		sd.Params = nil
		return
	}
	if ms, ok := ModuleSort(string(sd.BuiltinField)); ok {
		sd.Params = entity.ViewModuleParams(ms.Params, sd.Params)
	}
}

// sortExposedKey - the shape of ViewSortDef.ExposedKey.
var sortExposedKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidSortExposure checks an offer of a sort criterion to visitors (a
// non-empty key): the key's shape and a label.
func ValidSortExposure(key, label string) error {
	if !sortExposedKey.MatchString(key) {
		return fmt.Errorf("%w: %q", ErrSortExposedKeyInvalid, key)
	}
	if label == "" {
		return ErrSortExposedLabelRequired
	}
	return nil
}

// validateSortExposure checks sd's offer to visitors: a key of the
// right shape, unused by the display's other criteria, and a label; not
// offered - no label.
func (s *DefaultService) validateSortExposure(ctx context.Context, sd *entity.ViewSortDef) error {
	sd.ExposedKey, sd.ExposedLabel = strings.TrimSpace(sd.ExposedKey), strings.TrimSpace(sd.ExposedLabel)
	if sd.ExposedKey == "" {
		sd.ExposedLabel = ""
		return nil
	}
	if err := ValidSortExposure(sd.ExposedKey, sd.ExposedLabel); err != nil {
		return err
	}
	others, err := s.repository.SortsByViewID(ctx, sd.ViewDisplayID)
	if err != nil {
		return fmt.Errorf("listing view sorts (viewID %v): %w", sd.ViewDisplayID, err)
	}
	for _, o := range others {
		if o.ID != sd.ID && o.ExposedKey == sd.ExposedKey {
			return fmt.Errorf("%w: %q", ErrSortExposedKeyTaken, sd.ExposedKey)
		}
	}
	return nil
}

// CreateSort creates a display's sort criterion once its target and its
// offer to visitors are valid (resolveSortTarget, validateSortExposure).
func (s *DefaultService) CreateSort(ctx context.Context, sd entity.ViewSortDef) (*entity.ViewSortDef, error) {
	if err := s.resolveSortTarget(ctx, &sd); err != nil {
		return nil, err
	}
	if err := s.validateSortExposure(ctx, &sd); err != nil {
		return nil, err
	}

	created, err := s.repository.CreateSort(ctx, sd)
	return entity.Wrapped(created, err, "creating view sort (viewDisplayID %v)", sd.ViewDisplayID)
}

// UpdateSort never re-validates FieldStorageID/BuiltinField: a sort's
// target is immutable once created (see entity.ViewRepository.UpdateSort).
func (s *DefaultService) UpdateSort(ctx context.Context, sd entity.ViewSortDef) (*entity.ViewSortDef, error) {
	moduleSortParams(&sd)
	if err := s.validateSortExposure(ctx, &sd); err != nil {
		return nil, err
	}
	updated, err := s.repository.UpdateSort(ctx, sd)
	return entity.Wrapped(updated, err, "updating view sort (id %v)", sd.ID)
}

// DeleteSort deletes the sort criterion with the given id.
func (s *DefaultService) DeleteSort(ctx context.Context, id int) error {
	if err := s.repository.DeleteSort(ctx, id); err != nil {
		return fmt.Errorf("deleting view sort (id %v): %w", id, err)
	}
	return nil
}

// FieldsByViewID - the display viewID's own fields, in their order
// (EffectiveFieldsByView - the ones it shows).
func (s *DefaultService) FieldsByViewID(ctx context.Context, viewID int) (entity.ViewFieldDefs, error) {
	fields, err := s.repository.FieldsByViewID(ctx, viewID)
	return entity.Wrapped(fields, err, "listing view fields (viewID %v)", viewID)
}

// ViewFieldByID - the View field with the given id.
func (s *DefaultService) ViewFieldByID(ctx context.Context, id int) (*entity.ViewFieldDef, error) {
	f, err := s.repository.ViewFieldByID(ctx, id)
	return entity.Found(f, err, "finding view field by id (id %v)", id)
}

// resolveViewFieldTarget validates f's target (a generic field or a
// builtin) and normalizes ImageStyle/TagFormat, for CreateViewField/
// UpdateViewField. An empty Label is kept as is; the fallback to the
// field's machine name happens at render time
// (BuildContentFieldInstances).
func (s *DefaultService) resolveViewFieldTarget(ctx context.Context, f *entity.ViewFieldDef) error {
	if f.LinkTo != entity.ViewFieldLinkNone && f.LinkTo != entity.ViewFieldLinkContent && f.LinkTo != entity.ViewFieldLinkFile {
		return fmt.Errorf("%w: %q", ErrFieldLinkTargetUnsupported, f.LinkTo)
	}

	if f.BuiltinField != "" && IsModuleKey(string(f.BuiltinField)) {
		mf, ok := ModuleField(string(f.BuiltinField))
		if !ok {
			return fmt.Errorf("%w: %q", ErrFieldTypeNotDisplayable, f.BuiltinField)
		}
		// The module's markup is the value: no image, date or tag
		// options, no link to a file.
		f.TagFormat, f.ImageStyle, f.DateFormat, f.TrimLength = "", "", "", 0
		f.ImageDisplay = entity.ViewImageFirst
		f.Formatter, f.Layout = "", entity.FieldLayoutAll
		if f.LinkTo == entity.ViewFieldLinkFile {
			f.LinkTo = ""
		}
		f.Params = entity.ViewModuleParams(mf.Params, f.Params)
		return nil
	}
	f.Params = nil
	if f.BuiltinField != "" {
		if !FieldBuiltinAllowed(f.BuiltinField) {
			return fmt.Errorf("%w: %q", ErrFieldTypeNotDisplayable, f.BuiltinField)
		}
		// No builtin is an image, so ImageStyle and LinkTo "file" are
		// meaningless - clear them rather than trust the form.
		f.TagFormat = ""
		f.ImageStyle = ""
		f.ImageDisplay = entity.ViewImageFirst
		f.Formatter, f.Layout = "", entity.FieldLayoutAll
		if f.LinkTo == entity.ViewFieldLinkFile {
			f.LinkTo = ""
		}
		if !f.BuiltinField.IsDate() {
			f.DateFormat = ""
		}
		return nil
	}

	fs, err := s.contentTypeRepo.FieldStorageByID(ctx, f.FieldStorageIDValue())
	if err != nil {
		return err
	}
	if !DisplayableFieldType(fs.FieldType) {
		return fmt.Errorf("%w: %q", ErrFieldTypeNotDisplayable, fs.FieldType)
	}
	// ImageStyle/TagFormat only make sense for their own FieldType - clear
	// them for other types rather than trust the form.
	if fs.FieldType != entity.FieldTypeImage || f.ImageDisplay != entity.ViewImageAll {
		f.ImageDisplay = entity.ViewImageFirst
	}
	if fs.FieldType != entity.FieldTypeImage {
		f.ImageStyle = ""
		if f.LinkTo == entity.ViewFieldLinkFile {
			f.LinkTo = ""
		}
	}
	if fs.FieldType != entity.FieldTypeDate {
		f.DateFormat = ""
	}
	if fs.FieldType != entity.FieldTypeTaxonomy {
		f.TagFormat = ""
	} else if f.TagFormat == "" {
		f.TagFormat = entity.ViewTagFormatPlain
	}
	// A formatter - link fields only (the admin checks it is one of the
	// type's); a layout - link and date fields.
	if fs.FieldType != entity.FieldTypeLink {
		f.Formatter = ""
	}
	if fs.FieldType != entity.FieldTypeLink && fs.FieldType != entity.FieldTypeDate {
		f.Layout = entity.FieldLayoutAll
	}
	return nil
}

// CreateViewField creates a display's field once its target is valid,
// its options normalized for it (resolveViewFieldTarget).
func (s *DefaultService) CreateViewField(ctx context.Context, f entity.ViewFieldDef) (*entity.ViewFieldDef, error) {
	if err := s.resolveViewFieldTarget(ctx, &f); err != nil {
		return nil, err
	}

	created, err := s.repository.CreateViewField(ctx, f)
	return entity.Wrapped(created, err, "creating view field (viewDisplayID %v)", f.ViewDisplayID)
}

// UpdateViewField - a field of a module that is off is saved as it is
// (its place in the list), not validated.
func (s *DefaultService) UpdateViewField(ctx context.Context, f entity.ViewFieldDef) (*entity.ViewFieldDef, error) {
	if _, on := ModuleField(string(f.BuiltinField)); on || !IsModuleKey(string(f.BuiltinField)) {
		if err := s.resolveViewFieldTarget(ctx, &f); err != nil {
			return nil, err
		}
	}

	updated, err := s.repository.UpdateViewField(ctx, f)
	return entity.Wrapped(updated, err, "updating view field (id %v)", f.ID)
}

// DeleteViewField deletes the View field with the given id.
func (s *DefaultService) DeleteViewField(ctx context.Context, id int) error {
	if err := s.repository.DeleteViewField(ctx, id); err != nil {
		return fmt.Errorf("deleting view field (id %v): %w", id, err)
	}
	return nil
}
