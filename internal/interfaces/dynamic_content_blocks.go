package interfaces

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// A blocks field (entity.FieldTypeBlocks) in the content form: its blocks
// in order - field_<id>_blocks, one input per block holding its key; a
// block's inputs are named as a node's (field_<set field id>…, text_format_…,
// delete_field_…) after its prefix field_<id>_<key>_ (blockInputPrefix),
// with its set (…type) and, a saved one, its ID (…id). A new block's key
// is made by the browser (fieldBlocks in app.js).

// blockKeyRe - a block's key in the form.
var blockKeyRe = regexp.MustCompile(`^[a-z0-9]{1,24}$`)

// blockInputPrefix - the prefix of the inputs of the block key of the
// blocks field fieldID.
func blockInputPrefix(fieldID int, key string) string {
	return fmt.Sprintf("field_%d_%s_", fieldID, key)
}

// newBlockKey - the key of a new block's template: the browser puts the
// block's own in its place.
const newBlockKey = "__KEY__"

// blockSetForm - a field set as the content form uses it.
type blockSetForm struct {
	Def    entity.ContentTypeDef
	Fields entity.FieldInstanceDefs
}

// blockSetsOf - the field sets of the blocks field f, in its order; a
// name that isn't a field set (any more) is left out.
func (c *DynamicContentController) blockSetsOf(ctx context.Context, f entity.FieldInstanceDef) ([]blockSetForm, error) {
	return c.blockSetsNamed(ctx, f.BlockTypes())
}

// blockSetsNamed - the field sets of names, in their order, each once; a
// name that isn't a field set (any more) is left out.
func (c *DynamicContentController) blockSetsNamed(ctx context.Context, names []string) ([]blockSetForm, error) {
	var out []blockSetForm
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		def, fields, err := c.resolveByMachineName(ctx, name)
		if err != nil {
			if service.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("loading the field set %q: %w", name, err)
		}
		if def.Embedded {
			out = append(out, blockSetForm{Def: *def, Fields: fields})
		}
	}
	return out, nil
}

// prefixedRequest - r with only its form values and files named prefix…,
// the prefix cut off: a block's inputs as a node's, for the field
// resolvers of the content form.
func prefixedRequest(r *http.Request, prefix string) *http.Request {
	pick := func(in url.Values) url.Values {
		out := url.Values{}
		for k, v := range in {
			if name, ok := strings.CutPrefix(k, prefix); ok {
				out[name] = v
			}
		}
		return out
	}
	sub := r.WithContext(r.Context())
	sub.Form, sub.PostForm = pick(r.Form), pick(r.PostForm)
	sub.MultipartForm = &multipart.Form{Value: map[string][]string{}, File: map[string][]*multipart.FileHeader{}}
	if r.MultipartForm != nil {
		sub.MultipartForm.Value = pick(r.MultipartForm.Value)
		for k, v := range r.MultipartForm.File {
			if name, ok := strings.CutPrefix(k, prefix); ok {
				sub.MultipartForm.File[name] = v
			}
		}
	}
	return sub
}

// resolveBlocks fills f.FieldBlocks with a blocks field's blocks as
// submitted, each resolved as a node's fields are (resolveBlockForm: its
// uploads saved, its terms, links, dates…; its saved values - fr.saved's
// blocks - carried forward where the form doesn't resend them), and
// f.BlockErrors with what's wrong with them. A new block left empty is
// dropped; one of a set the field doesn't take, or with a malformed key,
// is ignored - a saved one of a set taken out of the field stays.
func resolveBlocks(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	c, r := fr.c, fr.r
	ctx := r.Context()
	initBlocks(f)
	saved := map[int]entity.BlockItem{}
	// The field's sets, and those of its saved blocks (a set taken out of
	// the field: its blocks stay till removed).
	names := field.BlockTypes()
	for _, item := range entity.BlockItems(fr.saved[field.ID]) {
		saved[item.ID] = item
		names = append(names, item.Type)
	}
	sets, err := c.blockSetsNamed(ctx, names)
	if err != nil {
		return err
	}
	for _, key := range r.Form[fmt.Sprintf("field_%d_blocks", field.ID)] {
		if !blockKeyRe.MatchString(key) {
			continue
		}
		prefix := blockInputPrefix(field.ID, key)
		typ := r.FormValue(prefix + "type")
		// Its own saved values: a block of this field, of this set.
		id, _ := strconv.Atoi(r.FormValue(prefix + "id"))
		own, ok := saved[id]
		if !ok || own.Type != typ {
			id, own = 0, entity.BlockItem{}
		}
		var set *blockSetForm
		for i := range sets {
			if sets[i].Def.MachineName == typ {
				set = &sets[i]
			}
		}
		if set == nil || (id == 0 && !slices.Contains(field.BlockTypes(), typ)) {
			continue
		}
		sub := prefixedRequest(r, prefix)
		bf, err := c.resolveBlockForm(sub, *set, own.Values, f.SettingsAllowed)
		if err != nil {
			return err
		}
		values, formats := dynamicValuesFromForm(bf, set.Fields)
		// Its new tags aren't values yet (writeFormChanges makes them).
		if id == 0 && blockEmpty(values) && len(newTags(set.Fields, *bf)) == 0 {
			continue
		}
		n := len(f.FieldBlocks[field.ID]) + 1
		errs := bf.ValidateFields(ctx, set.Fields)
		c.validateUserFields(ctx, set.Fields, bf, errs)
		for name, msg := range errs {
			f.BlockErrors[prefix+name] = i18n.T(ctx, "@field, block @n: @error", "@field", field.Label, "@n", strconv.Itoa(n), "@error", msg)
		}
		f.FieldBlocks[field.ID] = append(f.FieldBlocks[field.ID], form.BlockInput{
			Key:  key,
			Item: entity.BlockItem{ID: id, Type: typ, Fields: set.Fields, Values: values, TextFormats: formats},
			Form: bf,
		})
	}
	return nil
}

// keepBlocks - a hidden blocks field's saved blocks.
func keepBlocks(fr *formRequest, field entity.FieldInstanceDef, f *form.DynamicForm) error {
	initBlocks(f)
	for _, item := range entity.BlockItems(fr.saved[field.ID]) {
		f.FieldBlocks[field.ID] = append(f.FieldBlocks[field.ID], form.BlockInput{Key: "e" + strconv.Itoa(item.ID), Item: item})
	}
	return nil
}

func initBlocks(f *form.DynamicForm) {
	if f.FieldBlocks == nil {
		f.FieldBlocks, f.BlockErrors = map[int][]form.BlockInput{}, map[string]string{}
	}
}

// resolveBlockForm - a block's form: its fields' inputs in r (a
// prefixedRequest) resolved as a node's are (resolveFormFields), saved -
// its saved values (nil for a new block).
func (c *DynamicContentController) resolveBlockForm(r *http.Request, set blockSetForm, saved map[int]any, settingsAllowed bool) (*form.DynamicForm, error) {
	ids := make([]int, len(set.Fields))
	for i, f := range set.Fields {
		ids[i] = f.ID
	}
	bf, _ := (&form.DynamicForm{}).ParseDynamicForm(r, ids)
	bf.SettingsAllowed = settingsAllowed
	if err := c.resolveFormFields(r, set.Def.MachineName, set.Fields, saved, bf); err != nil {
		return nil, err
	}
	return bf, nil
}

// blockEmpty - a block's values hold nothing (an unticked box aside).
func blockEmpty(values map[int]any) bool {
	for _, v := range values {
		if b, ok := v.(bool); ok && !b {
			continue
		}
		return false
	}
	return true
}

// blockItemForm - a block of a blocks field in the content form: its key
// (its inputs' prefix, blockInputPrefix), its saved ID (0 - new), its set
// and fields, its summary (the heading of its folded card) and what its
// inputs show (fieldInputData).
type blockItemForm struct {
	Key     string
	ID      int
	Set     entity.ContentTypeDef
	Fields  entity.FieldInstanceDefs
	Summary string
	Inputs  map[string]any
}

// Prefix - the prefix of the block's inputs' names.
func (b blockItemForm) Prefix(fieldID int) string { return blockInputPrefix(fieldID, b.Key) }

// blocksFieldForm - a blocks field in the content form: its blocks, and a
// new block of each of its sets (the templates of "Add").
type blocksFieldForm struct {
	Items []blockItemForm
	New   []blockItemForm
}

// blockFieldForms - the blocks fields of the content form by field ID:
// the blocks submitted (f's, a form shown again) or saved (values, the
// node's); what the blocks' inputs show is read for all of a field's
// blocks at once (fieldInputDataOf).
func (c *DynamicContentController) blockFieldForms(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any, f form.DynamicForm) map[int]blocksFieldForm {
	out := map[int]blocksFieldForm{}
	for _, field := range fields {
		if field.FieldType != entity.FieldTypeBlocks {
			continue
		}
		sets, err := c.blockSetsOf(ctx, field)
		if err != nil {
			c.LogFailure(err, "error loading the field sets of a blocks field", "fieldID", field.ID)
			continue
		}
		var bf blocksFieldForm
		var inputs []fieldInput
		for _, set := range sets {
			bf.New = append(bf.New, blockItemForm{Key: newBlockKey, Set: set.Def, Fields: set.Fields})
			inputs = append(inputs, fieldInput{fields: set.Fields})
		}
		// A saved block of a set taken out of the field is shown still.
		var names []string
		for _, item := range entity.BlockItems(values[field.ID]) {
			names = append(names, item.Type)
		}
		for _, in := range f.FieldBlocks[field.ID] {
			names = append(names, in.Item.Type)
		}
		others, err := c.blockSetsNamed(ctx, names)
		if err != nil {
			c.LogFailure(err, "error loading the field sets of a blocks field", "fieldID", field.ID)
		}
		setOf := func(name string) (blockSetForm, bool) {
			for _, s := range append(sets, others...) {
				if s.Def.MachineName == name {
					return s, true
				}
			}
			return blockSetForm{}, false
		}
		add := func(key string, item entity.BlockItem, sent *form.DynamicForm) {
			set, ok := setOf(item.Type)
			if !ok {
				return
			}
			shown := form.DynamicForm{}
			if sent != nil {
				shown = *sent
			}
			summary := entity.BlockItem{Fields: set.Fields, Values: item.Values}.Summary()
			bf.Items = append(bf.Items, blockItemForm{Key: key, ID: item.ID, Set: set.Def, Fields: set.Fields, Summary: summary})
			inputs = append(inputs, fieldInput{fields: set.Fields, saved: item.Values, textFormats: item.TextFormats, sent: shown})
		}
		if sent, ok := f.FieldBlocks[field.ID]; ok {
			for _, in := range sent {
				add(in.Key, in.Item, in.Form)
			}
		} else {
			for _, item := range entity.BlockItems(values[field.ID]) {
				add("e"+strconv.Itoa(item.ID), item, nil)
			}
		}
		data := c.fieldInputDataOf(ctx, inputs)
		for i := range bf.New {
			bf.New[i].Inputs = data[i]
		}
		for i := range bf.Items {
			bf.Items[i].Inputs = data[len(bf.New)+i]
		}
		out[field.ID] = bf
	}
	return out
}

// blockSetFields - the fields of the sets of blocks (their text formats
// need the formats' list).
func blockSetFields(blocks map[int]blocksFieldForm) entity.FieldInstanceDefs {
	var out entity.FieldInstanceDefs
	for _, bf := range blocks {
		for _, b := range bf.New {
			out = append(out, b.Fields...)
		}
	}
	return out
}

// fieldInput - the fields of a form (a node's or a block's) and what
// their inputs show: the saved values, their text formats, the form as
// sent.
type fieldInput struct {
	fields      entity.FieldInstanceDefs
	saved       map[int]any
	textFormats map[int]int
	sent        form.DynamicForm
}

// fieldInputData - what the inputs of fields show, by the keys the
// form's templates read (dynamicFieldInputIn): fieldInputDataOf of one
// form.
func (c *DynamicContentController) fieldInputData(ctx context.Context, fields entity.FieldInstanceDefs, saved map[int]any, textFormats map[int]int, f form.DynamicForm) map[string]any {
	return c.fieldInputDataOf(ctx, []fieldInput{{fields: fields, saved: saved, textFormats: textFormats, sent: f}})[0]
}

// fieldInputDataOf - what the inputs of each of forms show, by the keys
// the form's templates read (dynamicFieldInputIn): values (the saved
// ones, textFormats theirs; a submitted form's links and dates with a
// time or an end as sent), the image and file fields' files, the terms,
// nodes and users chosen (tagPickers - the pickers' tags as JSON), the
// vocabularies' terms, the text formats chosen; blockFields empty (a
// block's - pageData puts the page's). The files, terms, nodes and users
// of all the forms are read at once (keyFields), a vocabulary once.
func (c *DynamicContentController) fieldInputDataOf(ctx context.Context, forms []fieldInput) []map[string]any {
	sets := make([]entity.FieldInstanceDefs, len(forms))
	saved := make([]map[int]any, len(forms))
	for i, in := range forms {
		sets[i], saved[i] = in.fields, in.saved
	}
	k := keyFields(sets, saved)
	// imageFieldURLs/imageFieldAlts: file inputs can't be pre-filled, so show
	// the saved image and its alt text.
	imageURLs, imageAlts, _ := c.imageFieldURLs(ctx, k.fields, k.values, func(f entity.FieldInstanceDef) string {
		return f.DetailImageStyle
	})
	// imageFieldItems: a field with several images lists each (its own
	// only, not the default image) to keep, reorder, describe or delete.
	multiImages := entity.FieldInstanceDefs{}
	for _, f := range k.fields {
		if f.FieldType == entity.FieldTypeImage && f.Multiple() {
			f.DefaultImageID = nil
			multiImages = append(multiImages, f)
		}
	}
	imageItems := c.imageFieldItems(ctx, multiImages, k.values, func(f entity.FieldInstanceDef) string {
		if f.ListImageStyle != "" {
			return f.ListImageStyle
		}
		return imageStyleOriginal
	})
	files := splitKeyed(k, c.fileFieldLinks(ctx, k.fields, k.values))
	fileItems := splitKeyed(k, c.fileFieldItems(ctx, k.fields, k.values))
	// The terms, nodes and users chosen, by field ID: the pickers' tags.
	terms := splitKeyed(k, c.taxonomyFieldTerms(ctx, k.fields, k.values))
	nodes := splitKeyed(k, c.referenceFieldNodes(ctx, k.fields, k.values))
	users := splitKeyed(k, c.userFieldUsers(ctx, k.fields, k.values))
	urls, alts, items := splitKeyed(k, imageURLs), splitKeyed(k, imageAlts), splitKeyed(k, imageItems)
	vocabularies := map[int]entity.TaxonomyTerms{}

	out := make([]map[string]any, len(forms))
	for i, in := range forms {
		data := map[string]any{"blockFields": map[int]blocksFieldForm{}}
		values := displayValues(in.fields, in.saved, sitetime.FromContext(ctx))
		for id, links := range in.sent.FieldLinks {
			values[id] = links
		}
		for _, field := range in.fields {
			inputs, ok := in.sent.DateInputs[field.ID]
			switch {
			case !ok:
			case field.DateList():
				values[field.ID] = inputs
			case len(inputs) > 0:
				values[field.ID] = inputs[0]
			}
		}
		data["values"] = values
		// textFormatIDs - each field's selected text format; empty on create.
		textFormats := in.textFormats
		if textFormats == nil {
			textFormats = map[int]int{}
		}
		data["textFormatIDs"] = textFormats
		sentAlts(in, alts[i], items[i])
		data["imageFieldURLs"], data["imageFieldAlts"], data["imageFieldItems"] = urls[i], alts[i], items[i]
		data["fileFieldLinks"], data["fileFieldItems"] = files[i], fileItems[i]
		// A form sent back with errors: the file kept for the next submit,
		// the saved one ticked for deleting (sentBack).
		data["fileFieldSent"], data["fileFieldDeletes"] = in.sent.SentFiles, in.sent.FileDeletes
		data["multiTaxonomyFieldTerms"] = terms[i]
		data["referenceFieldNodes"] = nodes[i]
		data["userFieldUsers"] = users[i]
		data["tagPickers"] = tagPickers(in.fields, terms[i], nodes[i], users[i], newTags(in.fields, in.sent))

		taxonomyOptions := make(map[int]entity.TaxonomyTerms)
		// taxonomyHierarchyLabels: per field, term ID -> indented label
		// (TaxonomyTerms.BuildHierarchy) for select/radio/checkbox widgets.
		taxonomyHierarchyLabels := make(map[int]map[int]string)
		for _, field := range in.fields {
			if field.FieldType != entity.FieldTypeTaxonomy {
				continue
			}
			vid := field.VocabularyID()
			vocabulary, ok := vocabularies[vid]
			if !ok {
				var err error
				if vocabulary, err = c.taxonomyService.TermsByVocabularyID(ctx, vid); err != nil {
					c.LogFailure(err, "error loading vocabulary terms for dynamic field", "fieldID", field.ID)
					vocabulary = entity.TaxonomyTerms{}
				}
				vocabularies[vid] = vocabulary
			}
			taxonomyOptions[field.ID] = vocabulary.SortByHierarchy()
			taxonomyHierarchyLabels[field.ID] = vocabulary.BuildHierarchy()
		}
		data["taxonomyHierarchyLabels"] = taxonomyHierarchyLabels
		data["taxonomyOptions"] = taxonomyOptions
		out[i] = data
	}
	return out
}

// keyedFields - the fields of several forms or blocks (sets of fields
// with their values) as one: each set's fields under keys of their own
// (keys: set → field ID → key), so a field helper (imageFieldItems,
// taxonomyFieldTerms, referenceFieldNodes…) resolves all of them in one
// query; splitKeyed hands each set back its own.
type keyedFields struct {
	fields entity.FieldInstanceDefs
	values map[int]any
	keys   []map[int]int
}

// keyFields - sets' fields with values (values[i] - sets[i]'s) as one.
func keyFields(sets []entity.FieldInstanceDefs, values []map[int]any) keyedFields {
	k := keyedFields{values: map[int]any{}}
	next := 1
	for i, fields := range sets {
		keys := map[int]int{}
		for _, f := range fields {
			keyed := f
			keyed.ID = next
			keys[f.ID] = next
			k.fields = append(k.fields, keyed)
			if v, ok := values[i][f.ID]; ok {
				k.values[next] = v
			}
			next++
		}
		k.keys = append(k.keys, keys)
	}
	return k
}

// splitKeyed - a field helper's result over keyed fields (by key) as each
// set's own (by its field IDs).
func splitKeyed[T any](k keyedFields, m map[int]T) []map[int]T {
	out := make([]map[int]T, len(k.keys))
	for i, keys := range k.keys {
		out[i] = map[int]T{}
		for id, key := range keys {
			if v, ok := m[key]; ok {
				out[i][id] = v
			}
		}
	}
	return out
}

// newTags - the "new:" tags of sent's taxonomy fields (of fields), by
// field ID: created only when the content is saved (writeFormChanges).
func newTags(fields entity.FieldInstanceDefs, sent form.DynamicForm) map[int][]string {
	out := map[int][]string{}
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeTaxonomy || !f.Multiple() {
			continue
		}
		for _, spec := range form.TagSpecifications(form.ParseTagsInput(sent.FieldValues[f.ID])) {
			if spec.IsNew && strings.TrimSpace(spec.Value) != "" {
				out[f.ID] = append(out[f.ID], strings.TrimSpace(spec.Value))
			}
		}
	}
	return out
}

// sentAlts puts a sent form's alt texts (form.DynamicForm.FileAlts, not
// saved yet) over the files' own: the single images' (alts, by field ID)
// and the several images' items.
func sentAlts(in fieldInput, alts map[int]string, items map[int][]imageItem) {
	if len(in.sent.FileAlts) == 0 {
		return
	}
	for _, f := range in.fields {
		if f.FieldType != entity.FieldTypeImage {
			continue
		}
		if alt, ok := in.sent.FileAlts[resolveImageFieldFileID(in.saved[f.ID], nil)]; ok {
			alts[f.ID] = alt
		}
		for j := range items[f.ID] {
			if alt, ok := in.sent.FileAlts[items[f.ID][j].FileID]; ok {
				items[f.ID][j].Alt = alt
			}
		}
	}
}

// tagPicker - a tag of a picker (the terms, nodes or users chosen in a
// field): the pickers of edit_scripts.html read them from data-tags.
type tagPicker struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

// tagPickers - the tags of the pickers of fields (a taxonomy, a reference,
// a user field) as JSON, by field ID.
func tagPickers(fields entity.FieldInstanceDefs, terms map[int]entity.TaxonomyTerms, nodes map[int]entity.Nodes, users map[int][]entity.User, newTerms map[int][]string) map[int]string {
	out := map[int]string{}
	for _, f := range fields {
		tags := []tagPicker{}
		switch f.FieldType {
		case entity.FieldTypeTaxonomy:
			for _, t := range terms[f.ID] {
				tags = append(tags, tagPicker{ID: t.ID, Value: t.Name})
			}
			// The ones typed and not created yet (a form sent back): "new:"
			// again when sent.
			for _, name := range newTerms[f.ID] {
				tags = append(tags, tagPicker{Value: name})
			}
		case entity.FieldTypeReference:
			for _, n := range nodes[f.ID] {
				tags = append(tags, tagPicker{ID: n.ID, Value: n.Title})
			}
		case entity.FieldTypeUser:
			for _, u := range users[f.ID] {
				tags = append(tags, tagPicker{ID: u.ID, Value: u.Username})
			}
		default:
			continue
		}
		b, _ := json.Marshal(tags)
		out[f.ID] = string(b)
	}
	return out
}

// blockView - a block on its node's page: its section of the page shows
// it by its set's display (Sections, node_sections), or a theme's
// component_<set> (fields/blocks.html). Ctx - what node_sections reads
// (the block's values as "item", its terms, files, users), and the
// block's values and terms by machine name (fieldValues, fieldTerms) for
// a theme's own layout.
type blockView struct {
	ID       int
	Type     string
	Sections entity.DisplaySections
	Ctx      map[string]any
}

// blockViews - the blocks of the blocks fields among fields (values -
// the node's), by field ID; their files, terms, content and users are
// read for all of a field's blocks at once (keyFields).
func (c *DynamicContentController) blockViews(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any) map[int][]blockView {
	out := map[int][]blockView{}
	for _, field := range fields {
		if field.FieldType != entity.FieldTypeBlocks {
			continue
		}
		var items []entity.BlockItem
		var sets []entity.FieldInstanceDefs
		var setValues []map[int]any
		for _, item := range entity.BlockItems(values[field.ID]) {
			if item.ID == 0 || item.Fields == nil {
				continue
			}
			items = append(items, item)
			sets = append(sets, item.Fields.SortByDisplayOrder())
			setValues = append(setValues, item.Values)
		}
		if len(items) == 0 {
			continue
		}
		k := keyFields(sets, setValues)
		imageItems := splitKeyed(k, c.imageFieldItems(ctx, k.fields, k.values, func(f entity.FieldInstanceDef) string {
			return f.DetailImageStyle
		}))
		terms := splitKeyed(k, c.taxonomyFieldTerms(ctx, k.fields, k.values))
		// The content the block names (an actor, say): the public only.
		references := splitKeyed(k, c.publicReferenceNodes(ctx, k.fields, k.values))
		files := splitKeyed(k, c.fileFieldLinks(ctx, k.fields, k.values))
		fileItems := splitKeyed(k, c.fileFieldItems(ctx, k.fields, k.values))
		users := splitKeyed(k, c.userFieldUsers(ctx, k.fields, k.values))
		for i, item := range items {
			imageFieldURLs, imageFieldAlts := map[int]string{}, map[int]string{}
			for id, images := range imageItems[i] {
				imageFieldURLs[id], imageFieldAlts[id] = images[0].URL, images[0].Alt
			}
			dc := &entity.DynamicContent{Node: entity.Node{ID: item.ID, Type: entity.ContentType(item.Type)}, Fields: item.Fields, Values: item.Values, TextFormats: item.TextFormats}
			fieldValues, fieldTerms := map[string]any{}, map[string]entity.TaxonomyTerms{}
			for _, f := range item.Fields {
				if v, ok := item.Values[f.ID]; ok {
					fieldValues[f.MachineName] = v
				}
				if ts := terms[i][f.ID]; len(ts) > 0 {
					fieldTerms[f.MachineName] = ts
				}
			}
			out[field.ID] = append(out[field.ID], blockView{
				ID:       item.ID,
				Type:     item.Type,
				Sections: displaySectionsOf(sets[i], dc, terms[i], imageFieldURLs, references[i]),
				Ctx: map[string]any{
					"item":                dc,
					"imageFieldURLs":      imageFieldURLs,
					"imageFieldAlts":      imageFieldAlts,
					"imageFieldItems":     imageItems[i],
					"fileFieldLinks":      files[i],
					"fileFieldItems":      fileItems[i],
					"taxonomyFieldTerms":  terms[i],
					"userFieldUsers":      users[i],
					"referenceFieldNodes": references[i],
					"fieldValues":         fieldValues,
					"fieldTerms":          fieldTerms,
				},
			})
		}
	}
	return out
}
