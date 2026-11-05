package interfaces

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// formFieldType - how the content form takes a field type's value
// (a node's fields' and a block's): resolve - what the request gives the
// field beyond its raw value (ParseDynamicForm: uploads, rows, tags, a
// text's format…), into f; hidden - a field hidden in the form: its saved
// value kept (the request has none; nil - nothing to keep); value - its
// value from the resolved form (false - none) and, a text, its text
// format. Every field type is here (TestFieldTypesInForm).
type formFieldType struct {
	resolve func(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error
	hidden  func(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error
	value   func(f *form.DynamicForm, field entity.FieldInstanceDef) (v any, textFormat int, ok bool)
}

// formRequest - a content form's request as its fields' resolvers see it:
// the type its files are filed under (contentType, the upload tokens), the
// node's saved values (saved; nil on create) and what's read once for all
// the fields (the text formats, the user).
type formRequest struct {
	c           *DynamicContentController
	r           *http.Request
	contentType string
	saved       map[int]any

	formats      map[int]entity.TextFormatDef
	formatsRead  bool
	user         *entity.User
	userRead     bool
	nodeUser     *entity.User
	nodeUserRead bool
}

// textFormats - the text formats by ID (nil - they failed to load,
// logged).
func (fr *formRequest) textFormats() map[int]entity.TextFormatDef {
	if !fr.formatsRead {
		fr.formatsRead = true
		all, err := fr.c.textFormatService.TextFormats(fr.r.Context())
		if err != nil {
			fr.c.LogFailure(err, "error loading text formats")
			return nil
		}
		fr.formats = make(map[int]entity.TextFormatDef, len(all))
		for _, tf := range all {
			fr.formats[tf.ID] = tf
		}
	}
	return fr.formats
}

// currentUser - the session's user, for the text formats they may use
// (nil - a guest, or it failed to load: logged).
func (fr *formRequest) currentUser() *entity.User {
	if !fr.userRead {
		fr.userRead = true
		user, err := fr.c.GetCurrentUser(fr.r)
		if err != nil {
			fr.c.LogFailure(err, "error loading current user for text format resolution")
		}
		fr.user = user
	}
	return fr.user
}

// author - the form's user (currentNodeUser): uploads and new tags are
// theirs; nil - none.
func (fr *formRequest) author() *entity.User {
	if !fr.nodeUserRead {
		fr.nodeUserRead = true
		if user, ok := currentNodeUser(fr.c.BaseController, fr.r); ok {
			fr.nodeUser = user
		}
	}
	return fr.nodeUser
}

// resolveFormFields fills f with what the request r gives fields beyond
// their raw values - a hidden one keeps its saved value (saved, the
// edited node's or block's; nil on create) - through each field type's
// formFieldType. Files are filed under contentType.
func (c *DynamicContentController) resolveFormFields(r *http.Request, contentType string, fields entity.FieldInstanceDefs, saved map[int]any, f *form.DynamicForm) error {
	fr := &formRequest{c: c, r: r, contentType: contentType, saved: saved}
	for _, field := range fields {
		ft := formFieldTypes[field.FieldType]
		hook := ft.resolve
		if field.HiddenInForm {
			hook = ft.hidden
		}
		if hook == nil {
			continue
		}
		if err := hook(fr, field, f); err != nil {
			return err
		}
	}
	return nil
}

// dynamicValuesFromForm - the values of fields from the resolved form f,
// and the text formats of the texts having one; by field instance ID.
func dynamicValuesFromForm(f *form.DynamicForm, fields entity.FieldInstanceDefs) (map[int]any, map[int]int) {
	values := make(map[int]any, len(fields))
	textFormats := make(map[int]int)
	for _, field := range fields {
		value := formFieldTypes[field.FieldType].value
		if value == nil {
			continue
		}
		v, format, ok := value(f, field)
		if !ok {
			continue
		}
		values[field.ID] = v
		if format > 0 {
			textFormats[field.ID] = format
		}
	}
	return values, textFormats
}

var formFieldTypes = map[entity.ContentTypeFieldType]formFieldType{
	entity.FieldTypeText:   {resolve: resolveTextFormat, hidden: keepText, value: textValue},
	entity.FieldTypeEmail:  {hidden: keepString, value: trimmedValue},
	entity.FieldTypePhone:  {hidden: keepString, value: trimmedValue},
	entity.FieldTypeNumber: {hidden: keepNumber, value: numberValue},
	entity.FieldTypeDate:   {resolve: resolveDates, hidden: keepDates, value: dateValue},
	entity.FieldTypeBool: {hidden: keepBool, value: func(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
		return f.FieldValues[field.ID] == "1", 0, true
	}},
	entity.FieldTypeTaxonomy:  {resolve: resolveTags, hidden: keepIDs, value: idsValue},
	entity.FieldTypeList:      {resolve: resolveListValues, hidden: keepListValues, value: listValue},
	entity.FieldTypeReference: {hidden: keepIDs, value: idsValue},
	entity.FieldTypeImage:     {resolve: resolveFileUploads, hidden: keepIDs, value: idsValue},
	entity.FieldTypeFile:      {resolve: resolveFileUploads, hidden: keepIDs, value: idsValue},
	// The user IDs in order (the picker's hidden input).
	entity.FieldTypeUser: {hidden: keepIDs, value: func(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
		ids := form.ParseIDList(f.FieldValues[field.ID])
		return ids, 0, len(ids) > 0
	}},
	entity.FieldTypeLink: {resolve: resolveLinks, hidden: keepLinks, value: func(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
		// Checked already (form.DynamicForm.Validate).
		links := f.FieldLinks[field.ID]
		return links, 0, len(links) > 0
	}},
	entity.FieldTypeGroup: {},
}

func init() {
	// Blocks resolve their own fields through formFieldTypes: set after it
	// (an initialization cycle in its literal).
	formFieldTypes[entity.FieldTypeBlocks] = formFieldType{resolve: resolveBlocks, hidden: keepBlocks, value: func(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
		// The blocks in the form's order; none - the node's are deleted.
		items := []entity.BlockItem{}
		for _, in := range f.FieldBlocks[field.ID] {
			items = append(items, in.Item)
		}
		return items, 0, true
	}}
}

// --- Texts ---

// resolveTextFormat picks a text's format, when it has allowed ones: the
// submitted "text_format_<id>" if the field allows it and the user may use
// it (AuthService.CanUseTextFormat), else the lowest-weight usable one;
// none usable - no entry, and textValue falls back to the strict
// sanitizer.
func resolveTextFormat(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if len(field.AllowedTextFormatIDs()) == 0 {
		return nil
	}
	formats := fr.textFormats()
	if formats == nil {
		return nil
	}
	// Candidates: the field's allowed formats that still exist, sorted by
	// Weight. The lowest-weight usable one is the fallback when the
	// submitted choice is missing or not permitted.
	var candidates entity.TextFormatDefs
	for _, id := range field.AllowedTextFormatIDs() {
		if tf, ok := formats[id]; ok {
			candidates = append(candidates, tf)
		}
	}
	slices.SortFunc(candidates, func(a, b entity.TextFormatDef) int { return a.Weight - b.Weight })
	ctx, user := fr.r.Context(), fr.currentUser()
	submitted, _ := strconv.Atoi(fr.r.FormValue(fmt.Sprintf("text_format_%d", field.ID)))
	var chosen *entity.TextFormatDef
	for i, cand := range candidates {
		if cand.ID == submitted && fr.c.authService.CanUseTextFormat(ctx, user, cand) {
			chosen = &candidates[i]
			break
		}
	}
	if chosen == nil {
		for i, cand := range candidates {
			if fr.c.authService.CanUseTextFormat(ctx, user, cand) {
				chosen = &candidates[i]
				break
			}
		}
	}
	if chosen != nil {
		if f.TextFormatValues == nil {
			f.TextFormatValues = map[int]entity.TextFormatDef{}
		}
		f.TextFormatValues[field.ID] = *chosen
	}
	return nil
}

// keepText - a hidden text's saved value, its format resolved as a shown
// one's.
func keepText(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if err := keepString(fr, field, f); err != nil {
		return err
	}
	return resolveTextFormat(fr, field, f)
}

// textValue - a text, its HTML sanitized: by its text format
// (resolveTextFormat; none usable - the strict sanitizer, never
// unsanitized), or the allowlist of the old safe_html or wysiwyg widget.
func textValue(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
	raw := f.FieldValues[field.ID]
	if raw == "" {
		return nil, 0, false
	}
	switch {
	case len(field.AllowedTextFormatIDs()) > 0:
		if tf, ok := f.TextFormatValues[field.ID]; ok {
			return htmlsanitize.SanitizeWithFormat(raw, tf.AllowedTags, tf.AllowClassAttribute), tf.ID, true
		}
		return htmlsanitize.Sanitize(raw), 0, true
	case field.Widget() == entity.WidgetSafeHTML:
		// Sanitized on save: this widget accepts hand-typed HTML, so it
		// goes through the same allowlist as forum/comment input.
		return htmlsanitize.Sanitize(raw), 0, true
	case field.Widget() == entity.WidgetWYSIWYG:
		return htmlsanitize.SanitizeEditor(raw), 0, true
	}
	return raw, 0, true
}

// keepString - a hidden field's saved string.
func keepString(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if s, ok := fr.saved[field.ID].(string); ok {
		f.FieldValues[field.ID] = s
	}
	return nil
}

// trimmedValue - an e-mail address, a phone number: as typed, trimmed.
func trimmedValue(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
	raw := strings.TrimSpace(f.FieldValues[field.ID])
	return raw, 0, raw != ""
}

// --- Numbers, yes/no ---

func keepNumber(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if n, ok := fr.saved[field.ID].(float64); ok {
		f.FieldValues[field.ID] = strconv.FormatFloat(n, 'f', -1, 64)
	}
	return nil
}

// numberValue - a number as the field keeps them: whole, or to its
// decimals.
func numberValue(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
	raw := strings.TrimSpace(f.FieldValues[field.ID])
	if raw == "" {
		return nil, 0, false
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, 0, false
	}
	return field.RoundNumber(n), 0, true
}

func keepBool(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if b, ok := fr.saved[field.ID].(bool); ok && b {
		f.FieldValues[field.ID] = "1"
	}
	return nil
}

// --- Dates ---

// resolveDates fills f.FieldDates for a date with a time, an end or
// several dates (DateRich, DateList): the submitted rows' values
// (form.DateValueOf, times in the request's zone; what's wrong -
// f.FieldDateErrors), the inputs as sent (DateInputs, the form shown
// again). The field's raw value is its first date (the required check).
// A plain date is its raw value.
func resolveDates(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if !field.DateRich() && !field.DateList() {
		return nil
	}
	initDates(f)
	inputs := []form.DateInput{form.ParseDateInput(fr.r, field.ID)}
	if field.DateList() {
		inputs = form.ParseDateInputs(fr.r, field)
	}
	f.DateInputs[field.ID] = inputs
	f.FieldValues[field.ID] = ""
	for _, in := range inputs {
		d, ok, msg := form.DateValueOf(fr.r.Context(), field, in, sitetime.FromContext(fr.r.Context()))
		if msg != "" {
			f.FieldDateErrors[field.ID] = msg
			break
		}
		if ok {
			f.FieldDates[field.ID] = append(f.FieldDates[field.ID], d)
			if f.FieldValues[field.ID] == "" {
				f.FieldValues[field.ID] = in.Date
			}
		}
	}
	return nil
}

// keepDates - a hidden date's saved value(s).
func keepDates(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if !field.DateRich() && !field.DateList() {
		if t, ok := fr.saved[field.ID].(time.Time); ok && !t.IsZero() {
			f.FieldValues[field.ID] = t.Format("2006-01-02")
		}
		return nil
	}
	initDates(f)
	if dates := entity.DateValues(fr.saved[field.ID]); len(dates) > 0 {
		f.FieldDates[field.ID] = dates
		f.FieldValues[field.ID] = dates[0].Start.Format("2006-01-02")
	}
	return nil
}

func initDates(f *form.DynamicForm) {
	if f.FieldDates == nil {
		f.FieldDates, f.FieldDateErrors, f.DateInputs = map[int][]entity.DateValue{}, map[int]string{}, map[int][]form.DateInput{}
	}
}

// dateValue - a date (time.Time), one with a time or an end
// (entity.DateValue) or several ([]entity.DateValue): resolveDates'.
func dateValue(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
	dates := f.FieldDates[field.ID]
	switch {
	case field.DateList():
		return dates, 0, len(dates) > 0
	case field.DateRich():
		if len(dates) == 0 {
			return nil, 0, false
		}
		return dates[0], 0, true
	}
	raw := f.FieldValues[field.ID]
	if raw == "" {
		return nil, 0, false
	}
	t, err := time.Parse("2006-01-02", raw)
	return t, 0, err == nil
}

// --- IDs: terms, content, files, users ---

// keepIDs - a hidden field's saved ID(s), comma-joined.
func keepIDs(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	var ids []int
	switch v := fr.saved[field.ID].(type) {
	case int:
		if v > 0 {
			ids = []int{v}
		}
	case []int:
		ids = v
	}
	if len(ids) > 0 {
		f.FieldValues[field.ID] = joinIDs(ids)
	}
	return nil
}

// idsValue - an ID, or several (a field with several values; a user
// field's always), from the comma-joined raw value.
func idsValue(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
	raw := f.FieldValues[field.ID]
	if !field.Multiple() {
		n, err := strconv.Atoi(raw)
		return n, 0, err == nil && n > 0
	}
	var ids []int
	for _, tok := range strings.Split(raw, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(tok)); err == nil && n > 0 {
			ids = append(ids, n)
		}
	}
	return ids, 0, len(ids) > 0
}

func joinIDs(ids []int) string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = strconv.Itoa(id)
	}
	return strings.Join(strs, ",")
}

// resolveTags keeps a multi-value taxonomy field's raw tag input
// ("5,7,new:Name", form.ParseTagsInput) as sent: its "new:" tags are
// created with the content (writeFormChanges), not while the form is read -
// it may come back with errors. The select and checkbox widgets submit
// several same-named term IDs.
func resolveTags(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if !field.Multiple() {
		return nil
	}
	if w := field.Widget(); w == entity.WidgetSelect || w == entity.WidgetCheckboxes {
		// FormValue only read the first; these are always existing term
		// IDs, never "new:" tags.
		f.FieldValues[field.ID] = strings.Join(fr.r.Form[fmt.Sprintf("field_%d", field.ID)], ",")
	}
	return nil
}

// setFileAlt records the alt text sent for an image (form.DynamicForm.FileAlts).
func setFileAlt(f *form.DynamicForm, fileID int, alt string) {
	if f.FileAlts == nil {
		f.FileAlts = map[int]string{}
	}
	f.FileAlts[fileID] = alt
}

// writeFormChanges writes what reading the form f only noted, once it is
// valid and the content is being saved (ctx - the save's transaction): the
// images' alt texts and the taxonomy fields' new terms (by userID), in f
// and in its blocks; the new terms' IDs replace their "new:" tags in f
// (and the blocks' values), as the content's values are read from it.
func (c *DynamicContentController) writeFormChanges(ctx context.Context, f *form.DynamicForm, fields entity.FieldInstanceDefs, userID int) error {
	for fileID, alt := range f.FileAlts {
		if err := c.fileRepo.UpdateAlt(ctx, fileID, alt); err != nil {
			return fmt.Errorf("saving the alt text of file %d: %w", fileID, err)
		}
	}
	for _, field := range fields {
		raw := f.FieldValues[field.ID]
		switch {
		case field.FieldType == entity.FieldTypeTaxonomy && field.Multiple() && strings.Contains(raw, "new:"):
			ids, err := c.taxonomyService.ProcessTagInputs(ctx, form.TagSpecifications(form.ParseTagsInput(raw)), field.VocabularyID(), userID)
			if err != nil {
				return fmt.Errorf("creating the new terms of field %d: %w", field.ID, err)
			}
			f.FieldValues[field.ID] = joinIDs(ids)
		case field.FieldType == entity.FieldTypeBlocks:
			for i := range f.FieldBlocks[field.ID] {
				block := &f.FieldBlocks[field.ID][i]
				if block.Form == nil {
					continue
				}
				if err := c.writeFormChanges(ctx, block.Form, block.Item.Fields, userID); err != nil {
					return err
				}
				block.Item.Values, block.Item.TextFormats = dynamicValuesFromForm(block.Form, block.Item.Fields)
			}
		}
	}
	return nil
}

// --- Lists ---

// resolveListValues joins a multi-value list field's submitted values
// (several same-named inputs) with commas (a configured value has none,
// entity.FieldInstanceDef.AllowedValues).
func resolveListValues(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	if field.Multiple() {
		f.FieldValues[field.ID] = strings.Join(fr.r.Form[fmt.Sprintf("field_%d", field.ID)], ",")
	}
	return nil
}

func keepListValues(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	switch v := fr.saved[field.ID].(type) {
	case string:
		f.FieldValues[field.ID] = v
	case []string:
		f.FieldValues[field.ID] = strings.Join(v, ",")
	}
	return nil
}

// listValue - a list's value, or its several ([]string).
func listValue(f *form.DynamicForm, field entity.FieldInstanceDef) (any, int, bool) {
	raw := f.FieldValues[field.ID]
	if raw == "" {
		return nil, 0, false
	}
	if !field.Multiple() {
		return raw, 0, true
	}
	var vals []string
	for _, tok := range strings.Split(raw, ",") {
		if tok = strings.TrimSpace(tok); tok != "" {
			vals = append(vals, tok)
		}
	}
	return vals, 0, len(vals) > 0
}

// --- Links ---

// resolveLinks fills f.FieldLinks with a link field's rows as submitted
// (form.ParseLinkRows; the titles dropped where the field has none).
func resolveLinks(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	links := form.ParseLinkRows(fr.r, field.ID)
	if field.LinkTitle() == entity.LinkTitleNone {
		for i := range links {
			links[i].Title = ""
		}
	}
	initLinks(f)[field.ID] = links
	return nil
}

func keepLinks(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	initLinks(f)[field.ID] = entity.Links(fr.saved[field.ID])
	return nil
}

func initLinks(f *form.DynamicForm) map[int][]entity.Link {
	if f.FieldLinks == nil {
		f.FieldLinks = map[int][]entity.Link{}
	}
	return f.FieldLinks
}

// --- Images and files ---

// resolveFileUploads gives an image or a file field its file ID(s) from
// the multipart body, as idsValue reads them: a new upload's ID (when the
// field takes uploads), a file picked from the media library
// (<field>_library, one the field may take: pickedFiles), "0" for a
// ticked delete_field_<id>, otherwise the saved one carried forward -
// a file input can't be pre-filled and SetFieldValues replaces the
// field's rows, so an untouched field would be wiped. An image's alt
// text is applied to its file, new or kept. Several images -
// resolveImagesField.
func resolveFileUploads(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	c, r := fr.c, fr.r
	isImage := field.FieldType == entity.FieldTypeImage
	if isImage && field.Multiple() {
		c.resolveImagesField(r, fr.contentType, field, fr.saved[field.ID], f)
		return nil
	}
	fieldName := fmt.Sprintf("field_%d", field.ID)
	var alt string
	if isImage {
		alt = strings.TrimSpace(r.FormValue(fieldName + "_alt"))
	}
	uploadError := func(msg string) {
		if f.FieldUploadErrors == nil {
			f.FieldUploadErrors = make(map[int]string)
		}
		f.FieldUploadErrors[field.ID] = msg
	}

	if file, header, ferr := r.FormFile(fieldName); ferr == nil && !field.AllowsUpload() {
		_ = file.Close()
	} else if ferr == nil {
		user := fr.author()
		if user == nil {
			c.logger.Error("error saving field upload: no authenticated user", "fieldID", field.ID)
			_ = file.Close()
			return nil
		}
		var fileID int
		var saveErr error
		if isImage {
			fileID, saveErr = SaveFieldImageWithPolicy(r.Context(), &c.fileUploadService, c.fileRepo, file, header, FieldUploadTokens(fr.contentType, user.ID), field)
		} else {
			fileID, saveErr = SaveFieldFileWithPolicy(r.Context(), &c.fileUploadService, c.fileRepo, file, header, FieldUploadTokens(fr.contentType, user.ID), field)
		}
		if saveErr != nil {
			c.LogFailure(saveErr, "error saving field upload", "fieldID", field.ID)
			uploadError(i18n.ErrorText(r.Context(), saveErr))
			return nil
		}
		if alt != "" {
			setFileAlt(f, fileID, alt)
		}
		f.FieldValues[field.ID] = strconv.Itoa(fileID)
		return nil
	}

	if picked, _ := strconv.Atoi(r.FormValue(fieldName + "_library")); picked > 0 {
		saved, _ := fr.saved[field.ID].(int)
		if picked == saved || c.pickedFiles(r, field, []int{picked})[picked] {
			if isImage {
				setFileAlt(f, picked, alt)
			}
			f.FieldValues[field.ID] = strconv.Itoa(picked)
			return nil
		}
		uploadError(i18n.T(r.Context(), "This file can't be used here."))
	}

	if r.FormValue("delete_"+fieldName) == "1" {
		f.FieldValues[field.ID] = "0"
		return nil
	}
	// The file of a form sent back with errors (SentFiles): kept when the
	// field may take it from the media library.
	if sent, _ := strconv.Atoi(r.FormValue(fieldName + "_sent")); sent > 0 && c.pickedFiles(r, field, []int{sent})[sent] {
		if isImage {
			setFileAlt(f, sent, alt)
		}
		f.FieldValues[field.ID] = strconv.Itoa(sent)
		return nil
	}
	if fileID, ok := fr.saved[field.ID].(int); ok && fileID > 0 {
		if isImage {
			setFileAlt(f, fileID, alt)
		}
		f.FieldValues[field.ID] = strconv.Itoa(fileID)
	}
	return nil
}
