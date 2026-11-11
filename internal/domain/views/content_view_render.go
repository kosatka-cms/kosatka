package views

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/fallback"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Sentinel FieldInstanceDef IDs (and FieldStorageIDs) for content builtin
// fields (entity.ViewFieldBuiltinField) in View lists. Negative so they
// never collide with generic fields, which use their (positive)
// field_storage_id.
const (
	contentViewBuiltinTitleID        = -1
	contentViewBuiltinContentTypeID  = -3
	contentViewBuiltinCreatedAtID    = -4
	contentViewBuiltinViewCountID    = -5
	contentViewBuiltinCommentCountID = -6
	contentViewBuiltinAuthorID       = -7
	contentViewBuiltinLinkID         = -8
	contentViewBuiltinReadingTimeID  = -9
	contentViewBuiltinStatusID       = -10
	contentViewBuiltinUpdatedAtID    = -11
	contentViewBuiltinEditLinkID     = -12
	// A module's field: moduleFieldIDBase minus its position among the
	// View's fields.
	moduleFieldIDBase = -1000
)

// ContentListItem - one row of a content View's list (its page, its
// preview, its block, a contextual fragment - BuildContentList); the
// counterpart of CommentListItem. dynamic/list.html reads ImageFieldURLs
// for every field, not only image fields.
type ContentListItem struct {
	Node   entity.Node
	Fields entity.FieldInstanceDefs
	Values map[int]any
	// ImageFieldURLs/ImageFieldAlts/ImageFieldOriginalURLs - an image
	// field's (first) styled URL, its file's alt text and the original
	// file's URL (a field linking to the file), by field ID.
	ImageFieldURLs         map[int]string
	ImageFieldAlts         map[int]string
	ImageFieldOriginalURLs map[int]string
	// ImageFieldPictures - the (first) image's picture by field ID: its
	// styled URL with the style's other widths and its size (the template
	// function img).
	ImageFieldPictures map[int]imageurl.Picture
	// ImageFieldImages - every image of a field with several, in order, by
	// field ID (a View field showing them all, "image_display" "all").
	ImageFieldImages map[int][]ListImage
	// TaxonomyFieldTerms - a taxonomy field's resolved terms by field
	// ID (dynamic/list.html renders them; Values holds the term IDs).
	TaxonomyFieldTerms map[int]entity.TaxonomyTerms
	// UserFieldUsers - a user field's users in order by field ID
	// (dynamic/list.html renders them; Values holds their IDs).
	UserFieldUsers map[int][]entity.User
}

// ListImage - one image of a View row's image field: its styled URL
// (Picture - with the style's other widths and its size), alt text and
// the original file's URL.
type ListImage struct {
	URL      string
	Picture  imageurl.Picture
	Alt      string
	Original string
}

// UsersLoader - the users of user fields of View rows (implemented by
// the user repository).
type UsersLoader interface {
	ByIDs(ctx context.Context, ids []int) (map[int]entity.User, error)
}

// TermsLoader - term names for taxonomy fields of View rows
// (implemented by service.TaxonomyService).
type TermsLoader interface {
	TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error)
}

// FileLoader - files by ID (implemented by FileRepository).
type FileLoader interface {
	ByIDs(ctx context.Context, ids []int) (map[int]entity.File, error)
}

// CommentCounter - published comments per node (the comment count
// builtin; implemented by service.CommentsService).
type CommentCounter interface {
	CountByNodeIDs(ctx context.Context, nids []int, status int8, rootOnly bool) (map[int]int, error)
}

// ContentListSources - what BuildContentList reads. Any may be nil: what
// it would supply is left out (the styleguide builds a list with none).
// Logger records the parts that fail and are left out.
type ContentListSources struct {
	Nodes    entity.NodesRepository
	Types    entity.ContentTypeRepository
	Terms    TermsLoader
	Files    FileLoader
	Comments CommentCounter
	Users    UsersLoader
	Logger   *slog.Logger
}

// ContentListOptions - how BuildContentList builds a list.
type ContentListOptions struct {
	// AltTitles - the builtin title is the node's alternative title (its
	// type's AltTitleField) when it has one, as View blocks show it; a View
	// page shows the title itself.
	AltTitles bool
}

// BuildContentList builds a content View's list of nodes - the one way
// its page, its admin preview, its block and a contextual fragment get
// it: the View's fields as field instances (BuildContentFieldInstances,
// their storages, labels and which text storages hold HTML loaded here)
// and each node's values (BuildContentListItems, batched across the
// nodes). A part that fails to load is logged and left out (a field, the
// labels, the counts); a failure to load the values is returned.
func BuildContentList(ctx context.Context, src ContentListSources, viewID int, viewFields entity.ViewFieldDefs, nodes entity.Nodes, opts ContentListOptions) (entity.FieldInstanceDefs, []ContentListItem, error) {
	sorted := viewFields.SortBySortOrder()
	storages := make(map[int]entity.FieldStorageDef, len(sorted))
	var storageIDs []int
	if src.Types != nil {
		for _, vf := range sorted {
			if vf.BuiltinField != "" {
				continue
			}
			fs, err := src.Types.FieldStorageByID(ctx, vf.FieldStorageIDValue())
			if err != nil {
				fallback.Log(src.Logger, err, "loading a View field's storage (the field left out)", "viewID", viewID, "fieldStorageID", vf.FieldStorageIDValue())
				continue
			}
			storages[vf.FieldStorageIDValue()] = *fs
			storageIDs = append(storageIDs, vf.FieldStorageIDValue())
		}
	}
	var labels map[int]string
	var rich map[int]bool
	var typeLabels map[string]string
	if src.Types != nil {
		var err error
		if len(storageIDs) > 0 {
			labels, err = src.Types.FieldInstanceLabelsByFieldStorageIDs(ctx, storageIDs)
			fallback.Log(src.Logger, err, "loading the View fields' labels (their machine names shown)", "viewID", viewID)
			// Without it every text field is escaped: safe, if plainer.
			rich, err = src.Types.RichTextFieldStorageIDs(ctx, storageIDs)
			fallback.Log(src.Logger, err, "loading which text fields hold HTML (all escaped)", "viewID", viewID)
		}
		cts, err := src.Types.ContentTypes(ctx)
		fallback.Log(src.Logger, err, "loading the content types' labels (the content type field blank)", "viewID", viewID)
		typeLabels = make(map[string]string, len(cts))
		for _, ct := range cts {
			typeLabels[ct.MachineName] = ct.Label
		}
	}
	var commentCounts, altTitles = map[int]int{}, map[int]string(nil)
	if len(nodes) > 0 {
		nodeIDs := make([]int, len(nodes))
		for i, n := range nodes {
			nodeIDs[i] = n.ID
		}
		if src.Comments != nil {
			counts, err := src.Comments.CountByNodeIDs(ctx, nodeIDs, entity.CommentPublished, false)
			fallback.Log(src.Logger, err, "counting the nodes' comments (0 shown)", "viewID", viewID)
			if err == nil {
				commentCounts = counts
			}
		}
		if opts.AltTitles && src.Nodes != nil {
			titles, err := src.Nodes.AltTitles(ctx, nodes)
			fallback.Log(src.Logger, err, "loading the nodes' alternative titles (their titles shown)", "viewID", viewID)
			altTitles = titles
		}
	}
	listFields := BuildContentFieldInstances(ctx, sorted, storages, labels, rich)
	items, err := BuildContentListItems(ctx, src.Types, nodes, listFields, typeLabels, src.Terms, commentCounts, src.Files, altTitles)
	if err != nil {
		return listFields, items, err
	}
	addModuleFieldValues(ctx, src.Logger, viewID, listFields, items)
	addUserFieldUsers(ctx, src, viewID, listFields, items)
	return listFields, items, nil
}

// addUserFieldUsers fills the items' UserFieldUsers: the users of their
// user fields, one query for the list; failing - none shown (logged).
func addUserFieldUsers(ctx context.Context, src ContentListSources, viewID int, listFields entity.FieldInstanceDefs, items []ContentListItem) {
	if src.Users == nil {
		return
	}
	var ids []int
	for _, it := range items {
		for _, lf := range listFields {
			if lf.FieldType == entity.FieldTypeUser {
				ids = append(ids, entity.UserIDs(it.Values[lf.ID])...)
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	users, err := src.Users.ByIDs(ctx, uniqueIDs(ids))
	if err != nil {
		fallback.Log(src.Logger, err, "loading the users of a View's user fields (left out)", "viewID", viewID)
		return
	}
	for i := range items {
		for _, lf := range listFields {
			if lf.FieldType != entity.FieldTypeUser {
				continue
			}
			for _, id := range entity.UserIDs(items[i].Values[lf.ID]) {
				if u, ok := users[id]; ok {
					if items[i].UserFieldUsers == nil {
						items[i].UserFieldUsers = map[int][]entity.User{}
					}
					items[i].UserFieldUsers[lf.ID] = append(items[i].UserFieldUsers[lf.ID], u)
				}
			}
		}
	}
}

// addModuleFieldValues fills the modules' fields of items: each module's
// markup for all the rows at once. One that fails is logged and left
// empty.
func addModuleFieldValues(ctx context.Context, logger *slog.Logger, viewID int, listFields entity.FieldInstanceDefs, items []ContentListItem) {
	if len(items) == 0 {
		return
	}
	ids := make([]int, len(items))
	for i, item := range items {
		ids[i] = item.Node.ID
	}
	filled := false
	for _, lf := range listFields {
		key, _ := lf.Config[moduleFieldConfigKey].(string)
		if key == "" {
			continue
		}
		mf, ok := ModuleField(key)
		if !ok {
			continue
		}
		params, _ := lf.Config[moduleFieldConfigParams].(map[string]string)
		html, err := mf.Render(ctx, ids, params)
		if err != nil {
			fallback.Log(logger, fmt.Errorf("rendering %s: %w", key, err), "rendering a module's View field (left empty)", "viewID", viewID)
			continue
		}
		for i := range items {
			if v, ok := html[items[i].Node.ID]; ok && v != "" {
				items[i].Values[lf.ID] = v
				filled = true
			}
		}
	}
	// Another field may fall back to a module's.
	if filled {
		for i := range items {
			entity.ApplyFieldFallbacks(listFields, items[i].Values)
		}
	}
}

// ContentListData - what dynamic/list.html reads of a content View's
// list and its look (display style, carousel, header and footer, custom
// CSS, row template, grouping), the same for its page, its block and a
// contextual fragment; the caller adds what only it has (pagination,
// glossary, exposed filters, isViewBlock). carouselLinkURLs - node ID →
// the slide's link (the display's CarouselLinkFieldStorageID).
func ContentListData(view entity.ViewDisplayDef, listFields entity.FieldInstanceDefs, items []ContentListItem, carouselLinkURLs map[int]string) map[string]any {
	return map[string]any{
		"items":                      items,
		"listFieldsHead":             listFields,
		"displayStyle":               string(view.DisplayStyle),
		"gridColumns":                view.GridColumns,
		"overflowAfter":              view.OverflowAfter,
		"overflowHeaderText":         view.OverflowHeaderText,
		"carouselSlidesPerView":      view.CarouselSlidesPerView,
		"carouselAutoplay":           view.CarouselAutoplay,
		"carouselLoop":               view.CarouselLoop,
		"carouselShowNavigation":     view.CarouselShowNavigation,
		"carouselShowPagination":     view.CarouselShowPagination,
		"carouselEffect":             view.CarouselEffect,
		"carouselLinkFieldStorageID": view.CarouselLinkFieldStorageIDValue(),
		"carouselLinkURLs":           carouselLinkURLs,
		// headerText/footerText - rich text before/after the list;
		// sanitized on save, rendered unescaped.
		"headerText": view.HeaderText,
		"footerText": view.FooterText,
		// customCSS - rendered outside the AJAX-replaced list body so
		// pagination doesn't duplicate or drop it.
		"customCSS":       view.CustomCSS,
		"rowTemplate":     view.RowTemplate,
		"groupByField":    view.GroupByField,
		"groupDateFormat": view.GroupDateFormat,
		// isViewList - Views don't render the node's title and main image
		// on their own; they are fields (entity.ViewFieldBuiltinField).
		"isViewList": true,
	}
}

// CarouselLinkURLs - node ID → the value of the text field that links a
// carousel slide (ViewDisplayDef.CarouselLinkFieldStorageID; 0 - none).
// Never nil.
func CarouselLinkURLs(ctx context.Context, types entity.ContentTypeRepository, fieldStorageID int, nodes entity.Nodes) (map[int]string, error) {
	if fieldStorageID <= 0 || len(nodes) == 0 || types == nil {
		return map[int]string{}, nil
	}
	nodeIDs := make([]int, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}
	urls, err := types.FieldTextValuesForNodes(ctx, fieldStorageID, nodeIDs)
	if err != nil {
		return map[int]string{}, err
	}
	return urls, nil
}

// builtinFieldInstance - the field instance of a builtin field (vf) with
// the sentinel id, its type, its label (defaultLabel when the View gives
// none) and its View display hints; extra goes into its Config.
func builtinFieldInstance(vf entity.ViewFieldDef, id int, fieldType entity.ContentTypeFieldType, defaultLabel string, extra map[string]any) entity.FieldInstanceDef {
	label := vf.Label
	if label == "" {
		label = defaultLabel
	}
	config := map[string]any{"css_class": vf.CSSClass, "link_to": string(vf.LinkTo), "rewrite_html": vf.RewriteHTML, "hide_in_overflow": vf.HideInOverflow}
	maps.Copy(config, extra)
	f := entity.FieldInstanceDef{
		ID:                 id,
		FieldStorageID:     id,
		MachineName:        string(vf.BuiltinField),
		Label:              label,
		FieldType:          fieldType,
		ShowLabelInList:    vf.ShowLabel,
		ExcludeFromDisplay: vf.ExcludeFromDisplay,
		FallbackFieldKey:   vf.FallbackFieldKey,
		Config:             config,
	}
	if fieldType != entity.FieldTypeText {
		f.WidgetConfig = map[string]any{"widget": string(entity.WidgetSafeHTML)}
	}
	return f
}

// BuildContentFieldInstances builds one field instance per View field: the
// builtins (sentinel IDs, values from the node), and every generic field
// whose storage is in fieldStorages - its ID and FieldStorageID both the
// storage's (FieldValuesForNodes queries by FieldStorageID and keys by
// ID), its Config the storage's (a copy) plus the View's display hints
// read by dynamic/list.html, its label the View's, else the storage's
// (fieldStorageLabels), else its machine name. richStorages - the text
// storages holding HTML (RichTextFieldStorageIDs): only those render as
// HTML, other text is escaped.
func BuildContentFieldInstances(ctx context.Context, viewFields entity.ViewFieldDefs, fieldStorages map[int]entity.FieldStorageDef, fieldStorageLabels map[int]string, richStorages map[int]bool) entity.FieldInstanceDefs {
	var listFields entity.FieldInstanceDefs
	for i, vf := range viewFields {
		if bi, ok := viewBuiltins[vf.BuiltinField]; ok {
			listFields = append(listFields, bi.instance(ctx, vf))
			continue
		}
		if vf.BuiltinField != "" && IsModuleKey(string(vf.BuiltinField)) {
			if mf, ok := ModuleField(string(vf.BuiltinField)); ok {
				listFields = append(listFields, moduleFieldInstance(ctx, vf, mf, moduleFieldIDBase-i))
			}
			continue
		}

		fs, ok := fieldStorages[vf.FieldStorageIDValue()]
		if !ok {
			continue
		}
		label := vf.Label
		if label == "" {
			label = fieldStorageLabels[fs.ID]
		}
		if label == "" {
			label = fs.MachineName
		}
		// config: a copy of fs.Config (never mutate the cached map) plus
		// View-only display hints read by dynamic/list.html.
		config := make(map[string]any, len(fs.Config)+9)
		maps.Copy(config, fs.Config)
		config["tag_format"] = string(vf.TagFormat)
		config["image_display"] = string(vf.ImageDisplay)
		// formatter/layout - a link field's formatter and the layout of
		// its several values (fields/link.html).
		config["formatter"] = vf.Formatter
		config["layout"] = string(vf.Layout)
		config["link_to"] = string(vf.LinkTo)
		config["css_class"] = vf.CSSClass
		config["rewrite_html"] = vf.RewriteHTML
		config["hide_in_overflow"] = vf.HideInOverflow
		// date_format - the View's date format; empty falls back to the field
		// instance's format at render time (viewDateFormat).
		config["date_format"] = vf.DateFormat
		// trim_length - a text field as plain text cut to it (plainExcerpt).
		config["trim_length"] = vf.TrimLength
		// A text field is HTML (the "safe_html" widget, RichText) only when
		// every instance of its storage stores sanitized HTML; plain text
		// is escaped (dynamic/list.html).
		widget := map[string]any{"widget": string(entity.WidgetSafeHTML)}
		if fs.FieldType == entity.FieldTypeText && !richStorages[fs.ID] {
			widget = nil
		}
		listFields = append(listFields, entity.FieldInstanceDef{
			ID:                 vf.FieldStorageIDValue(),
			FieldStorageID:     vf.FieldStorageIDValue(),
			MachineName:        fs.MachineName,
			Label:              label,
			FieldType:          fs.FieldType,
			ShowLabelInList:    vf.ShowLabel,
			ExcludeFromDisplay: vf.ExcludeFromDisplay,
			FallbackFieldKey:   vf.FallbackFieldKey,
			ListImageStyle:     vf.ImageStyle,
			Config:             config,
			WidgetConfig:       widget,
		})
	}
	return listFields
}

// BuildContentListItems pairs each node with listFields
// (BuildContentFieldInstances) and their resolved values, batching
// queries across all nodes:
//   - generic fields via one FieldValuesForNodes call; image files via
//     one files.ByIDs call (an image field without a style shows nothing:
//     there is no derivative to point at);
//   - taxonomy term IDs per field, names via one terms.TermsByIDs call
//     (IDs go to Values so FallbackFieldKey sees empty fields as empty);
//   - Title - altTitles[node] when set, else n.Title;
//   - ContentType from contentTypeLabels (machine name → label).
//
// commentCounts is nodeID → count; a nil map means 0 for every node.
func BuildContentListItems(ctx context.Context, contentTypeRepo entity.ContentTypeRepository, nodes entity.Nodes, listFields entity.FieldInstanceDefs, contentTypeLabels map[string]string, terms TermsLoader, commentCounts map[int]int, files FileLoader, altTitles map[int]string) ([]ContentListItem, error) {
	b := &listBatch{fields: listFields, contentTypeLabels: contentTypeLabels, commentCounts: commentCounts, altTitles: altTitles}
	if err := b.load(ctx, contentTypeRepo, nodes, terms, files); err != nil {
		return nil, err
	}
	items := make([]ContentListItem, len(nodes))
	for i, n := range nodes {
		items[i] = b.item(n)
	}
	return items, nil
}

// listBatch - BuildContentListItems' rows: what is read for all the
// nodes at once (load), then each node's item (item).
type listBatch struct {
	fields            entity.FieldInstanceDefs
	contentTypeLabels map[string]string
	commentCounts     map[int]int
	altTitles         map[int]string
	// readingTimes - node → its reading time as the list shows it ("5
	// min read"), when the list has the reading time field.
	readingTimes map[int]string
	// statusLabels - the status field's words (statusKey), when the list
	// has it.
	statusLabels map[string]string

	// batched - node → field → its value (FieldValuesForNodes).
	batched map[int]map[int]any
	// termIDs - taxonomy field → node → its term IDs; termsByID - the
	// terms.
	termIDs   map[int]map[int][]int
	termsByID map[int]entity.TaxonomyTerm
	// imageFiles - the image fields' files and their default images
	// (imageDefaults: type → field storage → file).
	imageFiles    map[int]entity.File
	imageDefaults map[string]map[int]int
}

// load reads the fields' values of the nodes: generic ones in one
// FieldValuesForNodes call, taxonomy terms (viewRowTerms), the images'
// files with the default images in one files.ByIDs call (without files
// an image field shows nothing).
func (b *listBatch) load(ctx context.Context, contentTypeRepo entity.ContentTypeRepository, nodes entity.Nodes, terms TermsLoader, files FileLoader) error {
	var genericFields, taxonomyFields entity.FieldInstanceDefs
	for _, lf := range b.fields {
		if lf.ID < 0 || (lf.FieldType == entity.FieldTypeImage && files == nil) {
			continue
		}
		if lf.FieldType == entity.FieldTypeTaxonomy {
			taxonomyFields = append(taxonomyFields, lf)
			continue
		}
		genericFields = append(genericFields, lf)
	}
	var err error
	if b.termIDs, b.termsByID, err = viewRowTerms(ctx, contentTypeRepo, terms, nodes, taxonomyFields); err != nil {
		return err
	}
	if slices.ContainsFunc(b.fields, func(f entity.FieldInstanceDef) bool { return f.ID == contentViewBuiltinStatusID }) {
		b.statusLabels = map[string]string{
			"published":   i18n.T(ctx, "Published"),
			"unpublished": i18n.T(ctx, "Unpublished"),
			"moderation":  i18n.T(ctx, "On moderation"),
		}
	}
	if slices.ContainsFunc(b.fields, func(f entity.FieldInstanceDef) bool { return f.ID == contentViewBuiltinReadingTimeID }) {
		if b.readingTimes, err = readingTimes(ctx, contentTypeRepo, nodes); err != nil {
			return err
		}
	}
	if len(genericFields) > 0 && contentTypeRepo != nil && len(nodes) > 0 {
		nodeIDs := make([]int, len(nodes))
		for i, n := range nodes {
			nodeIDs[i] = n.ID
		}
		if b.batched, err = contentTypeRepo.FieldValuesForNodes(ctx, nodeIDs, genericFields); err != nil {
			return err
		}
	}
	if files == nil {
		return nil
	}
	if b.imageDefaults, err = contentImageDefaults(ctx, contentTypeRepo, nodes, b.fields); err != nil {
		return err
	}
	var ids []int
	for _, byStorage := range b.imageDefaults {
		for _, fid := range byStorage {
			ids = append(ids, fid)
		}
	}
	for _, byField := range b.batched {
		for _, lf := range b.fields {
			if lf.FieldType == entity.FieldTypeImage {
				ids = append(ids, entity.FileIDs(byField[lf.ID])...)
			}
		}
	}
	if len(ids) > 0 {
		if b.imageFiles, err = files.ByIDs(ctx, uniqueIDs(ids)); err != nil {
			return err
		}
	}
	return nil
}

// item - node n's row: each field's value (a builtin's - viewBuiltins), a
// taxonomy field's terms, an image field's images; then the fallbacks
// (image values live in the image maps, so fallbacks don't apply to
// image fields).
func (b *listBatch) item(n entity.Node) ContentListItem {
	it := ContentListItem{Node: n, Fields: b.fields, Values: make(map[int]any, len(b.fields)),
		ImageFieldURLs: map[int]string{}, ImageFieldAlts: map[int]string{}, ImageFieldOriginalURLs: map[int]string{}, ImageFieldPictures: map[int]imageurl.Picture{}}
	for _, lf := range b.fields {
		switch lf.FieldType {
		case entity.FieldTypeTaxonomy:
			b.addTerms(&it, lf)
		case entity.FieldTypeImage:
			b.addImages(&it, lf)
		case entity.FieldTypeList:
			// Its chosen options (printed - their labels, not values).
			if v, ok := b.batched[n.ID][lf.ID]; ok {
				if items := lf.ListItems(v); len(items) > 0 {
					it.Values[lf.ID] = items
				}
			}
		default:
			if bi, ok := viewBuiltinByID[lf.ID]; ok {
				it.Values[lf.ID] = bi.value(b, n, lf)
			} else if v, ok := b.batched[n.ID][lf.ID]; ok {
				it.Values[lf.ID] = v
			}
		}
	}
	entity.ApplyFieldFallbacks(b.fields, it.Values)
	return it
}

// addTerms adds taxonomy field lf's terms to it (its value - their IDs,
// so FallbackFieldKey sees an empty field as empty).
func (b *listBatch) addTerms(it *ContentListItem, lf entity.FieldInstanceDef) {
	ids := b.termIDs[lf.ID][it.Node.ID]
	var ts entity.TaxonomyTerms
	for _, id := range ids {
		if t, ok := b.termsByID[id]; ok {
			ts = append(ts, t)
		}
	}
	if len(ts) == 0 {
		return
	}
	if it.TaxonomyFieldTerms == nil {
		it.TaxonomyFieldTerms = map[int]entity.TaxonomyTerms{}
	}
	it.TaxonomyFieldTerms[lf.ID] = ts
	it.Values[lf.ID] = ids
}

// addImages adds image field lf's images to it: the field's own files
// (several in order); none - the default image of the field on the
// node's type. No style - nothing to show (no derivative to point at).
// The first stands for the field.
func (b *listBatch) addImages(it *ContentListItem, lf entity.FieldInstanceDef) {
	if lf.ListImageStyle == "" {
		return
	}
	fileIDs := entity.FileIDs(b.batched[it.Node.ID][lf.ID])
	if len(fileIDs) == 0 {
		if fid, ok := b.imageDefaults[string(it.Node.Type)][lf.FieldStorageID]; ok {
			fileIDs = []int{fid}
		}
	}
	var list []ListImage
	for _, fid := range fileIDs {
		if file := b.imageFiles[fid]; file.URI != "" {
			pic := imageurl.StyledPicture(file, lf.ListImageStyle)
			list = append(list, ListImage{URL: pic.Src, Picture: pic, Alt: file.Alt, Original: imageurl.Original(file).URI})
		}
	}
	if len(list) == 0 {
		return
	}
	it.ImageFieldURLs[lf.ID], it.ImageFieldAlts[lf.ID], it.ImageFieldOriginalURLs[lf.ID] = list[0].URL, list[0].Alt, list[0].Original
	it.ImageFieldPictures[lf.ID] = list[0].Picture
	if lf.Multiple() {
		if it.ImageFieldImages == nil {
			it.ImageFieldImages = map[int][]ListImage{}
		}
		it.ImageFieldImages[lf.ID] = list
	}
}

// viewBuiltin - a builtin field of a content View: its sentinel ID, its
// instance (type, default label, extra config; nodeLink - the author or
// link field, NodeLinkFieldInstance) and its value for a node.
type viewBuiltin struct {
	id        int
	fieldType entity.ContentTypeFieldType
	config    func(vf entity.ViewFieldDef) map[string]any
	nodeLink  bool
	value     func(b *listBatch, n entity.Node, lf entity.FieldInstanceDef) any
}

// instance - the builtin's field instance for View field vf.
func (bi viewBuiltin) instance(ctx context.Context, vf entity.ViewFieldDef) entity.FieldInstanceDef {
	if bi.nodeLink {
		return NodeLinkFieldInstance(ctx, vf, contentViewBuiltinAuthorID, contentViewBuiltinLinkID, contentViewBuiltinEditLinkID)
	}
	var extra map[string]any
	if bi.config != nil {
		extra = bi.config(vf)
	}
	return builtinFieldInstance(vf, bi.id, bi.fieldType, FieldBuiltinLabel(ctx, vf.BuiltinField), extra)
}

// viewBuiltins - the builtin fields of a content View
// (TestContentViewBuiltinsHandled: every content builtin is here).
var viewBuiltins = map[entity.ViewFieldBuiltinField]viewBuiltin{
	entity.ViewFieldBuiltinTitle: {id: contentViewBuiltinTitleID, fieldType: entity.FieldTypeText,
		// The alternative title when set (a page shows the nodes' own).
		value: func(b *listBatch, n entity.Node, _ entity.FieldInstanceDef) any {
			if alt := b.altTitles[n.ID]; alt != "" {
				return alt
			}
			return n.Title
		}},
	entity.ViewFieldBuiltinContentType: {id: contentViewBuiltinContentTypeID, fieldType: entity.FieldTypeText,
		value: func(b *listBatch, n entity.Node, _ entity.FieldInstanceDef) any {
			return b.contentTypeLabels[string(n.Type)]
		}},
	entity.ViewFieldBuiltinCreatedAt: {id: contentViewBuiltinCreatedAtID, fieldType: entity.FieldTypeDate,
		config: func(vf entity.ViewFieldDef) map[string]any {
			return map[string]any{"date_format": vf.DateFormat, "timestamp": true}
		},
		value: func(_ *listBatch, n entity.Node, _ entity.FieldInstanceDef) any { return n.CreatedAt }},
	entity.ViewFieldBuiltinViewCount: {id: contentViewBuiltinViewCountID, fieldType: entity.FieldTypeNumber,
		value: func(_ *listBatch, n entity.Node, _ entity.FieldInstanceDef) any { return n.Counter.TotalCount }},
	// commentCounts nil - 0 for every node.
	entity.ViewFieldBuiltinCommentCount: {id: contentViewBuiltinCommentCountID, fieldType: entity.FieldTypeNumber,
		value: func(b *listBatch, n entity.Node, _ entity.FieldInstanceDef) any { return b.commentCounts[n.ID] }},
	// readingTimes nil (the list doesn't show it) - none.
	entity.ViewFieldBuiltinReadingTime: {id: contentViewBuiltinReadingTimeID, fieldType: entity.FieldTypeText,
		value: func(b *listBatch, n entity.Node, _ entity.FieldInstanceDef) any { return b.readingTimes[n.ID] }},
	entity.ViewFieldBuiltinAuthor: {id: contentViewBuiltinAuthorID, nodeLink: true,
		value: func(_ *listBatch, n entity.Node, _ entity.FieldInstanceDef) any { return n.CreatedBy.Username }},
	entity.ViewFieldBuiltinLink: {id: contentViewBuiltinLinkID, nodeLink: true,
		value: func(_ *listBatch, _ entity.Node, lf entity.FieldInstanceDef) any { return lf.Label }},
	entity.ViewFieldBuiltinStatus: {id: contentViewBuiltinStatusID, fieldType: entity.FieldTypeText,
		value: func(b *listBatch, n entity.Node, _ entity.FieldInstanceDef) any { return b.statusLabels[statusKey(n)] }},
	entity.ViewFieldBuiltinUpdatedAt: {id: contentViewBuiltinUpdatedAtID, fieldType: entity.FieldTypeDate,
		config: func(vf entity.ViewFieldDef) map[string]any {
			return map[string]any{"date_format": vf.DateFormat, "timestamp": true}
		},
		value: func(_ *listBatch, n entity.Node, _ entity.FieldInstanceDef) any { return n.UpdatedAt }},
	entity.ViewFieldBuiltinEditLink: {id: contentViewBuiltinEditLinkID, nodeLink: true,
		value: func(_ *listBatch, _ entity.Node, lf entity.FieldInstanceDef) any { return lf.Label }},
}

// statusKey - the status field's word for n: published, unpublished, or
// published but on moderation.
func statusKey(n entity.Node) string {
	switch {
	case n.Status != 1:
		return "unpublished"
	case n.OnModeration != 0:
		return "moderation"
	}
	return "published"
}

// readingTimes - each node's reading time (entity.ReadingMinutesOf its
// type's fields) in ctx's language, "" when it has nothing to read: the
// fields' values a type at a time.
func readingTimes(ctx context.Context, contentTypeRepo entity.ContentTypeRepository, nodes entity.Nodes) (map[int]string, error) {
	if contentTypeRepo == nil || len(nodes) == 0 {
		return nil, nil
	}
	types, err := contentTypeRepo.ContentTypes(ctx)
	if err != nil {
		return nil, err
	}
	idsByType := map[string][]int{}
	for _, n := range nodes {
		idsByType[string(n.Type)] = append(idsByType[string(n.Type)], n.ID)
	}
	out := make(map[int]string, len(nodes))
	for _, ct := range types {
		ids := idsByType[ct.MachineName]
		if len(ids) == 0 {
			continue
		}
		fields, err := contentTypeRepo.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			return nil, err
		}
		var read entity.FieldInstanceDefs
		for _, f := range fields {
			if f.ReadsAsText() {
				read = append(read, f)
			}
		}
		if len(read) == 0 {
			continue
		}
		values, err := contentTypeRepo.FieldValuesForNodes(ctx, ids, read)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if minutes := entity.ReadingMinutesOf(read, func(f entity.FieldInstanceDef) any { return values[id][f.ID] }); minutes > 0 {
				out[id] = ReadingTimeText(ctx, minutes)
			}
		}
	}
	return out, nil
}

// ReadingTimeText - minutes of reading as the site shows them: "5 min
// read" in ctx's language.
func ReadingTimeText(ctx context.Context, minutes int) string {
	return i18n.Plural(ctx, minutes, "@count min read", "@count min read")
}

// viewBuiltinByID - viewBuiltins by their sentinel IDs.
var viewBuiltinByID = func() map[int]viewBuiltin {
	out := make(map[int]viewBuiltin, len(viewBuiltins))
	for _, bi := range viewBuiltins {
		out[bi.id] = bi
	}
	return out
}()

// viewRowTerms - for each taxonomy field: node ID → its term IDs, plus
// every referenced term by ID (one TermsByIDs call). Empty without
// taxonomy fields or the repositories to resolve them.
func viewRowTerms(ctx context.Context, contentTypeRepo entity.ContentTypeRepository, terms TermsLoader, nodes entity.Nodes, fields entity.FieldInstanceDefs) (map[int]map[int][]int, map[int]entity.TaxonomyTerm, error) {
	if len(fields) == 0 || contentTypeRepo == nil || terms == nil || len(nodes) == 0 {
		return nil, nil, nil
	}
	nodeIDs := make([]int, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}
	byField := make(map[int]map[int][]int, len(fields))
	var all []int
	for _, f := range fields {
		byNode := map[int][]int{}
		if f.Multiple() {
			multi, err := contentTypeRepo.FieldTaxonomyMultiValuesForNodes(ctx, f.FieldStorageID, nodeIDs)
			if err != nil {
				return nil, nil, err
			}
			byNode = multi
		} else {
			single, err := contentTypeRepo.FieldTaxonomyValuesForNodes(ctx, f.FieldStorageID, nodeIDs)
			if err != nil {
				return nil, nil, err
			}
			for nid, tid := range single {
				byNode[nid] = []int{tid}
			}
		}
		byField[f.ID] = byNode
		for _, ids := range byNode {
			all = append(all, ids...)
		}
	}
	if len(all) == 0 {
		return byField, nil, nil
	}
	termsByID, err := terms.TermsByIDs(ctx, uniqueIDs(all))
	if err != nil {
		return nil, nil, err
	}
	return byField, termsByID, nil
}

// contentImageDefaults - node type → field storage → the default image
// (FieldInstanceDef.DefaultImageID) of that type's image field, for the
// image fields among listFields; types without one are left out.
func contentImageDefaults(ctx context.Context, repo entity.ContentTypeRepository, nodes entity.Nodes, listFields entity.FieldInstanceDefs) (map[string]map[int]int, error) {
	storages := map[int]bool{}
	for _, lf := range listFields {
		if lf.FieldType == entity.FieldTypeImage {
			storages[lf.FieldStorageID] = true
		}
	}
	out := map[string]map[int]int{}
	if len(storages) == 0 || repo == nil || len(nodes) == 0 {
		return out, nil
	}
	types := map[string]bool{}
	for _, n := range nodes {
		types[string(n.Type)] = true
	}
	defs, err := repo.ContentTypes(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range defs {
		if !types[d.MachineName] {
			continue
		}
		fields, err := repo.FieldsByContentTypeID(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		for _, f := range fields {
			if f.FieldType == entity.FieldTypeImage && f.DefaultImageID != nil && storages[f.FieldStorageID] {
				if out[d.MachineName] == nil {
					out[d.MachineName] = map[int]int{}
				}
				out[d.MachineName][f.FieldStorageID] = *f.DefaultImageID
			}
		}
	}
	return out, nil
}

// uniqueIDs - ids sorted without repeats (a copy), for an IN query.
func uniqueIDs(ids []int) []int {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

// The Config keys of a module's field instance: its key and its params
// in the View (addModuleFieldValues reads them).
const (
	moduleFieldConfigKey    = "module_field"
	moduleFieldConfigParams = "module_params"
)

// moduleFieldInstance - the field instance of a module's field (vf, mf)
// with the sentinel id: a text field whose values are the module's
// markup (template.HTML, shown as it is), its label the View's, else the
// module's.
func moduleFieldInstance(ctx context.Context, vf entity.ViewFieldDef, mf entity.ViewModuleField, id int) entity.FieldInstanceDef {
	return builtinFieldInstance(vf, id, entity.FieldTypeText, i18n.T(ctx, mf.Label), map[string]any{
		moduleFieldConfigKey:    mf.Key,
		moduleFieldConfigParams: entity.ViewModuleParams(mf.Params, vf.Params),
	})
}

// NodeLinkFieldInstance - the field instance of the builtin author, link
// or edit link field (vf), for View blocks and View pages alike: the
// author's name linking their profile (link_to "author",
// dynamic/list.html), or a link to the node (to its edit form: link_to
// "edit") whose text is the field's label ("Read more", "Edit" when
// empty; no label shown before it).
// authorID/linkID/editLinkID - the caller's sentinel IDs for them.
func NodeLinkFieldInstance(ctx context.Context, vf entity.ViewFieldDef, authorID, linkID, editLinkID int) entity.FieldInstanceDef {
	config := map[string]any{"css_class": vf.CSSClass, "rewrite_html": vf.RewriteHTML, "hide_in_overflow": vf.HideInOverflow}
	if vf.BuiltinField == entity.ViewFieldBuiltinEditLink {
		label := vf.Label
		if label == "" {
			label = i18n.T(ctx, "Edit")
		}
		config["link_to"] = "edit"
		return entity.FieldInstanceDef{ID: editLinkID, FieldStorageID: editLinkID, MachineName: string(entity.ViewFieldBuiltinEditLink), Label: label,
			FieldType: entity.FieldTypeText, ExcludeFromDisplay: vf.ExcludeFromDisplay, Config: config}
	}
	if vf.BuiltinField == entity.ViewFieldBuiltinLink {
		label := vf.Label
		if label == "" {
			label = i18n.T(ctx, "Read more")
		}
		config["link_to"] = string(entity.ViewFieldLinkContent)
		return entity.FieldInstanceDef{ID: linkID, FieldStorageID: linkID, MachineName: string(entity.ViewFieldBuiltinLink), Label: label,
			FieldType: entity.FieldTypeText, ExcludeFromDisplay: vf.ExcludeFromDisplay, Config: config}
	}
	label := vf.Label
	if label == "" {
		label = FieldBuiltinLabel(ctx, entity.ViewFieldBuiltinAuthor)
	}
	config["link_to"] = "author"
	return entity.FieldInstanceDef{ID: authorID, FieldStorageID: authorID, MachineName: string(entity.ViewFieldBuiltinAuthor), Label: label,
		FieldType: entity.FieldTypeText, ShowLabelInList: vf.ShowLabel, ExcludeFromDisplay: vf.ExcludeFromDisplay,
		FallbackFieldKey: vf.FallbackFieldKey, Config: config}
}
