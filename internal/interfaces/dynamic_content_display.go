package interfaces

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// displayValues formats stored values for the edit form inputs. Dates need
// "2006-01-02" for <input type="date">, a date with a time or an end -
// its inputs (form.DateInput, times in zone); everything else uses default
// formatting.
func displayValues(fields entity.FieldInstanceDefs, values map[int]any, zone *time.Location) map[int]any {
	display := make(map[int]any, len(values))
	for _, field := range fields {
		v, ok := values[field.ID]
		if !ok {
			continue
		}
		if field.FieldType == entity.FieldTypeDate {
			if field.DateList() {
				var rows []form.DateInput
				for _, d := range entity.DateValues(v) {
					rows = append(rows, form.DateInputOf(d, zone))
				}
				display[field.ID] = rows
				continue
			}
			if field.DateRich() {
				d, _ := entity.AsDateValue(v)
				display[field.ID] = form.DateInputOf(d, zone)
				continue
			}
			if t, ok := entity.DateStart(v); ok {
				display[field.ID] = t.Format("2006-01-02")
				continue
			}
		}
		display[field.ID] = v
	}
	return display
}

// resolveImageFieldFileID returns the file ID an image/file field should
// render: the node's own value (the first of several) if positive, else the
// field's DefaultImageID, else 0.
func resolveImageFieldFileID(raw any, defaultImageID *int) int {
	if ids := resolveImageFieldFileIDs(raw, defaultImageID); len(ids) > 0 {
		return ids[0]
	}
	return 0
}

// resolveImageFieldFileIDs - the files an image field renders: the node's
// own (one, or several in order), else the field's DefaultImageID.
func resolveImageFieldFileIDs(raw any, defaultImageID *int) []int {
	if ids := entity.FileIDs(raw); len(ids) > 0 {
		return ids
	}
	if defaultImageID != nil && *defaultImageID > 0 {
		return []int{*defaultImageID}
	}
	return nil
}

// imageItem - one picture of an image field: its file, its URL in the
// field's image style (Picture - with its other widths and size, for the
// template function img), its alt text, the original file's URL and what
// the lightbox opens (Lightbox: imageurl.LightboxPicture).
type imageItem struct {
	FileID   int
	URL      string
	Picture  imageurl.Picture
	Alt      string
	Original string
	Lightbox imageurl.Picture
}

// imageStyleOriginal - imageFieldItems' style for the original file (no
// image style is named so).
const imageStyleOriginal = "*"

// imageFieldItems returns, per FieldTypeImage field ID, its pictures in
// order (resolveImageFieldFileIDs: several for a field with several
// values). styleOf picks the image style (imageStyleOriginal - none);
// fields with no style ("") are skipped.
func (c *DynamicContentController) imageFieldItems(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any, styleOf func(entity.FieldInstanceDef) string) map[int][]imageItem {
	items := make(map[int][]imageItem)
	// Every field's files in one query.
	fieldIDs := map[int][]int{}
	var all []int
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeImage || styleOf(f) == "" {
			continue
		}
		ids := resolveImageFieldFileIDs(values[f.ID], f.DefaultImageID)
		fieldIDs[f.ID] = ids
		all = append(all, ids...)
	}
	if len(all) == 0 {
		return items
	}
	files, err := c.fileRepo.ByIDs(ctx, all)
	if err != nil {
		c.LogFailure(err, "error resolving image field files", "fileIDs", all)
		return items
	}
	for _, f := range fields {
		ids, ok := fieldIDs[f.ID]
		if !ok {
			continue
		}
		style := styleOf(f)
		for _, id := range ids {
			file, ok := files[id]
			if !ok {
				continue
			}
			item := imageItem{FileID: id, Alt: file.Alt, Original: imageurl.Original(file).URI, Lightbox: imageurl.LightboxPicture(file)}
			item.Picture = imageurl.OriginalPicture(file)
			if style != imageStyleOriginal {
				item.Picture = imageurl.StyledPicture(file, style)
			}
			item.URL = item.Picture.Src
			items[f.ID] = append(items[f.ID], item)
		}
	}
	return items
}

// imageFieldURLs returns, per FieldTypeImage field ID, the first picture's
// styled URL, alt text and original file URL (used by View fields linking
// to the file): imageFieldItems, where one picture stands for the field.
func (c *DynamicContentController) imageFieldURLs(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any, styleOf func(entity.FieldInstanceDef) string) (map[int]string, map[int]string, map[int]string) {
	urls := make(map[int]string)
	alts := make(map[int]string)
	origURLs := make(map[int]string)
	for id, items := range c.imageFieldItems(ctx, fields, values, styleOf) {
		urls[id], alts[id], origURLs[id] = items[0].URL, items[0].Alt, items[0].Original
	}
	return urls, alts, origURLs
}

// fileLinkInfo is one FieldTypeFile value's resolved download-link info
// for the file field's line (field_line_file, fields/line.html).
type fileLinkInfo struct {
	URI       string
	Name      string
	SizeBytes int
}

// fileFieldLinks is the FieldTypeFile counterpart of imageFieldURLs (same
// default-file fallback, no image style). Name falls back to the URI's
// basename when the file has no original filename.
func (c *DynamicContentController) fileFieldLinks(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any) map[int]fileLinkInfo {
	links := make(map[int]fileLinkInfo)
	// Every field's file in one query.
	fieldFile := map[int]int{}
	var all []int
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeFile {
			continue
		}
		if fileID := resolveImageFieldFileID(values[f.ID], f.DefaultImageID); fileID > 0 {
			fieldFile[f.ID] = fileID
			all = append(all, fileID)
		}
	}
	if len(all) == 0 {
		return links
	}
	files, err := c.fileRepo.ByIDs(ctx, all)
	if err != nil {
		c.LogFailure(err, "error resolving file field files", "fileIDs", all)
		return links
	}
	for _, f := range fields {
		fileID, ok := fieldFile[f.ID]
		if !ok {
			continue
		}
		file, ok := files[fileID]
		if !ok {
			continue
		}
		name := file.OriginalFilename
		if name == "" {
			name = path.Base(file.URI)
		}
		links[f.ID] = fileLinkInfo{
			URI:       imageurl.Original(file).URI,
			Name:      name,
			SizeBytes: file.SizeBytes,
		}
	}
	return links
}

// taxonomyFieldTerms resolves each taxonomy field's stored term ID(s) into
// terms for rendering. A single-value field gets a slice of 0 or 1 term, so
// templates handle both cases the same way.
func (c *DynamicContentController) taxonomyFieldTerms(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any) map[int]entity.TaxonomyTerms {
	// Collect all term IDs first and resolve them in one query: multi-value
	// fields can hold many terms and this runs on public pages.
	fieldTermIDs := make(map[int][]int, len(fields))
	var allTermIDs []int
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeTaxonomy {
			continue
		}
		raw, ok := values[f.ID]
		if !ok {
			continue
		}
		var termIDs []int
		if f.Multiple() {
			ids, ok := raw.([]int)
			if !ok {
				continue
			}
			termIDs = ids
		} else {
			termID, ok := raw.(int)
			if !ok || termID <= 0 {
				continue
			}
			termIDs = []int{termID}
		}
		fieldTermIDs[f.ID] = termIDs
		allTermIDs = append(allTermIDs, termIDs...)
	}

	if len(allTermIDs) == 0 {
		return map[int]entity.TaxonomyTerms{}
	}
	termsByID, err := c.taxonomyService.TermsByIDs(ctx, allTermIDs)
	if err != nil {
		c.LogFailure(err, "error batch-resolving taxonomy field terms")
		return map[int]entity.TaxonomyTerms{}
	}

	terms := make(map[int]entity.TaxonomyTerms, len(fieldTermIDs))
	for fieldID, termIDs := range fieldTermIDs {
		resolved := make(entity.TaxonomyTerms, 0, len(termIDs))
		for _, termID := range termIDs {
			if term, ok := termsByID[termID]; ok {
				resolved = append(resolved, term)
			}
		}
		if len(resolved) > 0 {
			terms[fieldID] = resolved
		}
	}
	return terms
}

// sectionData - what the page's sections are built from: the node (its
// values), its taxonomy fields' terms, its image fields' URLs in their
// styles and, a block's, the content its references name (nil - a node
// page's: Views show those).
type sectionData struct {
	dc         *entity.DynamicContent
	terms      map[int]entity.TaxonomyTerms
	imageURLs  map[int]string
	references map[int]entity.Nodes
}

// pageFieldType - how the content page shows a field type's value
// (buildDisplaySections): line - it has a value for a "Label: value"
// line (dynamic/page.html's field_line_<type>: the compact flow, a
// group's member; nil - never); own - the section of its own it stands
// in (a text in the body, an image, a module's formatter's, blocks; nil -
// none, its line); anyGroup - that section whatever its page group (it is
// no group's member). Every field type is here (TestFieldTypesOnPage).
type pageFieldType struct {
	line     func(d sectionData, f entity.FieldInstanceDef) bool
	own      func(d sectionData, f entity.FieldInstanceDef) (entity.DisplaySection, bool)
	anyGroup bool
}

var pageFieldTypes = map[entity.ContentTypeFieldType]pageFieldType{
	entity.FieldTypeText: {line: hasString, own: func(d sectionData, f entity.FieldInstanceDef) (entity.DisplaySection, bool) {
		// A text "in the article's body": a full-width section.
		if !f.RendersInBody || !hasString(d, f) {
			return entity.DisplaySection{}, false
		}
		return entity.DisplaySection{Body: &f}, true
	}},
	entity.FieldTypeEmail: {line: hasString},
	entity.FieldTypePhone: {line: hasString},
	entity.FieldTypeNumber: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		n, ok := d.dc.Values[f.ID].(float64)
		return ok && n != 0
	}},
	entity.FieldTypeDate: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		switch v := d.dc.Values[f.ID].(type) {
		case time.Time:
			return true
		case entity.DateValue:
			return !v.Start.IsZero()
		case []entity.DateValue:
			return len(v) > 0
		}
		return false
	}},
	entity.FieldTypeBool: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		b, ok := d.dc.Values[f.ID].(bool)
		return ok && b
	}},
	entity.FieldTypeTaxonomy: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		return len(d.terms[f.ID]) > 0
	}},
	entity.FieldTypeList: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		switch v := d.dc.Values[f.ID].(type) {
		case string:
			return v != ""
		case []string:
			return len(v) > 0
		}
		return false
	}},
	entity.FieldTypeFile: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		id, ok := d.dc.Values[f.ID].(int)
		return ok && id > 0
	}},
	// A reference - a block's: links to the content (a node page's are
	// shown by Views).
	entity.FieldTypeReference: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		return len(d.references[f.ID]) > 0
	}},
	entity.FieldTypeUser: {line: func(d sectionData, f entity.FieldInstanceDef) bool {
		return len(entity.UserIDs(d.dc.Values[f.ID])) > 0
	}},
	// Links: their line, or a module's formatter's section (a video
	// player, say; its module off - the line).
	entity.FieldTypeLink: {line: hasLinks, own: func(d sectionData, f entity.FieldInstanceDef) (entity.DisplaySection, bool) {
		if _, ok := service.FieldFormatterFor(f.FieldType, f.DetailFormatter); !ok || !hasLinks(d, f) {
			return entity.DisplaySection{}, false
		}
		return entity.DisplaySection{Formatted: &f}, true
	}},
	// An image: a picture of its own, when it has a URL in its style.
	entity.FieldTypeImage: {own: func(d sectionData, f entity.FieldInstanceDef) (entity.DisplaySection, bool) {
		if _, ok := d.imageURLs[f.ID]; !ok {
			return entity.DisplaySection{}, false
		}
		return entity.DisplaySection{Image: &f}, true
	}},
	// Blocks: a section of their own, each block by its set's display.
	entity.FieldTypeBlocks: {anyGroup: true, own: func(d sectionData, f entity.FieldInstanceDef) (entity.DisplaySection, bool) {
		if len(entity.BlockItems(d.dc.Values[f.ID])) == 0 {
			return entity.DisplaySection{}, false
		}
		return entity.DisplaySection{Blocks: &f}, true
	}},
	// A group: a container of its members (buildDisplaySections).
	entity.FieldTypeGroup: {},
}

func hasString(d sectionData, f entity.FieldInstanceDef) bool {
	s, ok := d.dc.Values[f.ID].(string)
	return ok && s != ""
}

func hasLinks(d sectionData, f entity.FieldInstanceDef) bool {
	return len(entity.Links(d.dc.Values[f.ID])) > 0
}

// buildDisplaySections splits displayFields (sorted by DisplayOrder) into
// sections in that same order, so a field's position is decided only by its
// display order. A group with visible members and a field standing in a
// section of its own (pageFieldType.own: a body text, an image, a module's
// formatter, blocks) each get theirs; the fields with a line in between
// are batched into ordinary sections.
func buildDisplaySections(displayFields entity.FieldInstanceDefs, dc *entity.DynamicContent, taxonomyFieldTerms map[int]entity.TaxonomyTerms, imageFieldURLs map[int]string) entity.DisplaySections {
	return displaySectionsOf(displayFields, dc, taxonomyFieldTerms, imageFieldURLs, nil)
}

// displaySectionsOf - buildDisplaySections, a block's: its reference
// fields shown too, by their nodes (references - the ones to show, by
// field ID; nil - none, the node page's: Views attach those).
func displaySectionsOf(displayFields entity.FieldInstanceDefs, dc *entity.DynamicContent, taxonomyFieldTerms map[int]entity.TaxonomyTerms, imageFieldURLs map[int]string, references map[int]entity.Nodes) entity.DisplaySections {
	d := sectionData{dc: dc, terms: taxonomyFieldTerms, imageURLs: imageFieldURLs, references: references}
	hasLine := func(f entity.FieldInstanceDef) bool {
		line := pageFieldTypes[f.FieldType].line
		return line != nil && line(d, f)
	}
	membersByGroup := make(map[int]entity.FieldInstanceDefs)
	for _, f := range displayFields {
		if f.ParentGroupID == nil || !f.ShowOnDetail || pageFieldTypes[f.FieldType].anyGroup || !hasLine(f) {
			continue
		}
		membersByGroup[*f.ParentGroupID] = append(membersByGroup[*f.ParentGroupID], f)
	}

	var sections entity.DisplaySections
	var ordinary entity.FieldInstanceDefs
	flushOrdinary := func() {
		if len(ordinary) > 0 {
			sections = append(sections, entity.DisplaySection{Fields: ordinary})
			ordinary = nil
		}
	}
	for _, f := range displayFields {
		pt := pageFieldTypes[f.FieldType]
		if f.ParentGroupID != nil && !pt.anyGroup {
			continue // rendered as part of its group's section instead
		}
		if !f.ShowOnDetail {
			continue
		}
		if f.FieldType == entity.FieldTypeGroup {
			members := membersByGroup[f.ID]
			if len(members) == 0 {
				continue
			}
			flushOrdinary()
			group := f
			sections = append(sections, entity.DisplaySection{Group: &group, Fields: members})
			continue
		}
		if pt.own != nil {
			if section, ok := pt.own(d, f); ok {
				flushOrdinary()
				sections = append(sections, section)
				continue
			}
		}
		if hasLine(f) {
			ordinary = append(ordinary, f)
		}
	}
	flushOrdinary()
	return sections
}

// redirectTarget - a redirect field's value as a safe target: a
// site-relative path ("/x", not "//host") or an absolute http(s) URL;
// anything else ("" - no redirect).
func redirectTarget(v any) string {
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//"):
		return s
	case strings.HasPrefix(s, "https://"), strings.HasPrefix(s, "http://"):
		return s
	}
	return ""
}

// fieldImage - an image field's file and its URL in the field's
// DetailImageStyle (none - the original), for theme templates; a field
// with several images - its first, and All of them in order.
type fieldImage struct {
	File entity.File
	URI  string
	All  []fieldImage
}

// fieldImages returns each image field shown on the detail page, by machine
// name, with its own files (not the default image). Theme templates
// (page/<type>.html, page/node.html) use it to place an image by hand and
// leave it out of displaySections.
func (c *DynamicContentController) fieldImages(ctx context.Context, dc *entity.DynamicContent) map[string]fieldImage {
	out := map[string]fieldImage{}
	for _, f := range dc.Fields {
		if f.FieldType != entity.FieldTypeImage || !f.ShowOnDetail {
			continue
		}
		ids := entity.FileIDs(dc.Values[f.ID])
		if len(ids) == 0 {
			continue
		}
		files, err := c.fileRepo.ByIDs(ctx, ids)
		if err != nil {
			c.LogFailure(err, "error resolving image field files", "fieldID", f.ID, "fileIDs", ids)
			continue
		}
		var all []fieldImage
		for _, id := range ids {
			file, ok := files[id]
			if !ok {
				continue
			}
			uri := imageurl.Original(file).URI
			if f.DetailImageStyle != "" {
				uri = imageurl.Styled(file, f.DetailImageStyle).URI
			}
			all = append(all, fieldImage{File: file, URI: uri})
		}
		if len(all) > 0 {
			first := all[0]
			first.All = all
			out[f.MachineName] = first
		}
	}
	return out
}

// userFieldUsers resolves each user field's users (entity.UserIDs) in
// the field's order, one query for all of them; a user gone is left out
// (failing - none, logged).
func (c *DynamicContentController) userFieldUsers(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any) map[int][]entity.User {
	// Never nil: templates index it by field ID.
	out := map[int][]entity.User{}
	var ids []int
	for _, f := range fields {
		if f.FieldType == entity.FieldTypeUser {
			ids = append(ids, entity.UserIDs(values[f.ID])...)
		}
	}
	if len(ids) == 0 {
		return out
	}
	users, err := c.userService.ByIDs(ctx, ids)
	if err != nil {
		c.LogFailure(err, "error resolving the users of user fields", "userIDs", ids)
		return out
	}
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeUser {
			continue
		}
		for _, id := range entity.UserIDs(values[f.ID]) {
			if u, ok := users[id]; ok {
				out[f.ID] = append(out[f.ID], u)
			}
		}
	}
	return out
}

// referenceFieldNodes resolves each reference field's stored target IDs
// ([]int in editor's order for a Multiple() field, an int otherwise) into
// nodes (their own columns: NodesRepository.ByIDs, one query) for the
// edit form's search widget and a block's links. A target that no longer
// resolves is skipped.
func (c *DynamicContentController) referenceFieldNodes(ctx context.Context, fields entity.FieldInstanceDefs, values map[int]any) map[int]entity.Nodes {
	out := map[int]entity.Nodes{}
	fieldIDs := map[int][]int{}
	var all []int
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeReference {
			continue
		}
		var ids []int
		switch v := values[f.ID].(type) {
		case []int:
			ids = v
		case int:
			ids = []int{v}
		}
		fieldIDs[f.ID] = ids
		all = append(all, ids...)
	}
	if len(all) == 0 {
		return out
	}
	nodes, err := c.nodeRepo.ByIDs(ctx, all)
	if err != nil {
		c.LogFailure(err, "error resolving the nodes of reference fields", "nodeIDs", all)
		return out
	}
	for _, f := range fields {
		for _, id := range fieldIDs[f.ID] {
			if n, ok := nodes[id]; ok {
				out[f.ID] = append(out[f.ID], n)
			}
		}
	}
	return out
}
