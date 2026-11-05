package service

import (
	"context"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// teaserTaxonomyLoader - the taxonomy lookups TeaserBuilder.Build needs
// (implemented by TaxonomyService).
type teaserTaxonomyLoader interface {
	TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error)
}

// teaserFileLoader - files by ID (implemented by FileRepository).
type teaserFileLoader interface {
	ByIDs(ctx context.Context, ids []int) (map[int]entity.File, error)
}

// teaserCommentCounter - published comment counts per node
// (implemented by CommentsService).
type teaserCommentCounter interface {
	CountByNodeIDs(ctx context.Context, nids []int, status int8, rootOnly bool) (map[int]int, error)
}

// teaserImageRef - an image field value (or, isDefault, the field's
// default image for an empty value) waiting for its file.
type teaserImageRef struct {
	nodeID    int
	field     string
	fileID    int
	isDefault bool
}

// TeaserBuilder resolves nodes into entity.TeaserItem's for teaser
// cards: one query per field across all nodes of a type, not per node.
type TeaserBuilder struct {
	contentTypes entity.ContentTypeRepository
	taxonomy     teaserTaxonomyLoader
	comments     teaserCommentCounter
	files        teaserFileLoader
}

// SetFiles gives teaser items their image fields' files (wiring); nil -
// TeaserItem.Images stays empty.
func (b *TeaserBuilder) SetFiles(f teaserFileLoader) {
	b.files = f
}

// NewTeaserBuilder creates a TeaserBuilder.
func NewTeaserBuilder(contentTypes entity.ContentTypeRepository, taxonomy teaserTaxonomyLoader, comments teaserCommentCounter) *TeaserBuilder {
	return &TeaserBuilder{contentTypes: contentTypes, taxonomy: taxonomy, comments: comments}
}

// Build returns one TeaserItem per node, in the same order.
func (b *TeaserBuilder) Build(ctx context.Context, nodes entity.Nodes) ([]entity.TeaserItem, error) {
	if len(nodes) == 0 {
		return []entity.TeaserItem{}, nil
	}
	defs, err := b.contentTypes.ContentTypes(ctx)
	if err != nil {
		return nil, err
	}
	t := newTeaserBatch(b, nodes, defs)
	for typ, ids := range t.idsByType {
		if def, ok := t.defByType[typ]; ok {
			if err := t.loadType(ctx, def, ids); err != nil {
				return nil, err
			}
		}
	}
	if err := t.resolveImages(ctx); err != nil {
		return nil, err
	}
	if err := t.resolveTerms(ctx); err != nil {
		return nil, err
	}
	for i := range t.items {
		if f := t.defByType[string(t.items[i].Node.Type)].AltTitleField; f != "" {
			t.items[i].AltTitle = t.items[i].Text(f)
		}
	}
	if err := t.countComments(ctx); err != nil {
		return nil, err
	}
	return t.items, nil
}

// teaserBatch - one Build: the items and what is read for them, a type
// at a time (loadType), then the files and terms of all at once.
type teaserBatch struct {
	b         *TeaserBuilder
	items     []entity.TeaserItem
	itemByID  map[int]*entity.TeaserItem
	defByType map[string]entity.ContentTypeDef
	idsByType map[string][]int
	// nodeTermIDs - node → field machine_name → term IDs, resolved to
	// terms in one TermsByIDs call (resolveTerms).
	nodeTermIDs map[int]map[string][]int
	allTermIDs  []int
	imageRefs   []teaserImageRef
}

func newTeaserBatch(b *TeaserBuilder, nodes entity.Nodes, defs entity.ContentTypeDefs) *teaserBatch {
	t := &teaserBatch{
		b:           b,
		items:       make([]entity.TeaserItem, len(nodes)),
		itemByID:    make(map[int]*entity.TeaserItem, len(nodes)),
		defByType:   make(map[string]entity.ContentTypeDef, len(defs)),
		idsByType:   map[string][]int{},
		nodeTermIDs: map[int]map[string][]int{},
	}
	for _, d := range defs {
		t.defByType[d.MachineName] = d
	}
	for i, n := range nodes {
		t.idsByType[string(n.Type)] = append(t.idsByType[string(n.Type)], n.ID)
		t.items[i] = entity.TeaserItem{
			Node:          n,
			TypeLabel:     t.defByType[string(n.Type)].Label,
			Fields:        map[string]any{},
			Labels:        map[string]string{},
			Terms:         map[string]entity.TaxonomyTerms{},
			Images:        map[string]entity.File{},
			DefaultImages: map[string]entity.File{},
			DateFormats:   map[string]string{},
		}
	}
	for i := range t.items {
		t.itemByID[t.items[i].Node.ID] = &t.items[i]
	}
	return t
}

// each calls fn with the item of each of ids.
func (t *teaserBatch) each(ids []int, fn func(item *entity.TeaserItem)) {
	for _, id := range ids {
		fn(t.itemByID[id])
	}
}

// loadType reads what the items of type def (ids) show: the body field,
// the fields' labels, the scalar values, the images (their files later)
// and the terms (their terms later).
func (t *teaserBatch) loadType(ctx context.Context, def entity.ContentTypeDef, ids []int) error {
	fields, err := t.b.contentTypes.FieldsByContentTypeID(ctx, def.ID)
	if err != nil {
		return err
	}
	for _, f := range fields {
		if f.FieldType == entity.FieldTypeText && f.RendersInBody && f.ShowOnDetail && f.ParentGroupID == nil {
			t.each(ids, func(item *entity.TeaserItem) { item.BodyField = f.MachineName })
			break
		}
	}
	var scalar, images entity.FieldInstanceDefs
	for _, f := range fields {
		t.each(ids, func(item *entity.TeaserItem) { item.Labels[f.MachineName] = f.Label })
		switch f.FieldType {
		case entity.FieldTypeText, entity.FieldTypeNumber, entity.FieldTypeDate, entity.FieldTypeBool, entity.FieldTypeEmail, entity.FieldTypePhone:
			scalar = append(scalar, f)
			if f.FieldType == entity.FieldTypeDate && f.DateFormat != "" {
				t.each(ids, func(item *entity.TeaserItem) { item.DateFormats[f.MachineName] = f.DateFormat })
			}
		case entity.FieldTypeImage:
			if len(images) == 0 {
				t.each(ids, func(item *entity.TeaserItem) { item.ImageField, item.ImageStyle = f.MachineName, f.ListImageStyle })
			}
			images = append(images, f)
		case entity.FieldTypeTaxonomy:
			if err := t.loadTermIDs(ctx, f, ids); err != nil {
				return err
			}
		}
	}
	if err := t.collectImages(ctx, images, ids); err != nil {
		return err
	}
	return t.loadScalars(ctx, scalar, ids)
}

// loadTermIDs reads the term IDs of taxonomy field f of the nodes ids.
func (t *teaserBatch) loadTermIDs(ctx context.Context, f entity.FieldInstanceDef, ids []int) error {
	byNode := map[int][]int{}
	if f.Multiple() {
		var err error
		if byNode, err = t.b.contentTypes.FieldTaxonomyMultiValuesForNodes(ctx, f.FieldStorageID, ids); err != nil {
			return err
		}
	} else {
		single, err := t.b.contentTypes.FieldTaxonomyValuesForNodes(ctx, f.FieldStorageID, ids)
		if err != nil {
			return err
		}
		for nid, tid := range single {
			byNode[nid] = []int{tid}
		}
	}
	for nid, tids := range byNode {
		if t.nodeTermIDs[nid] == nil {
			t.nodeTermIDs[nid] = map[string][]int{}
		}
		t.nodeTermIDs[nid][f.MachineName] = tids
		t.allTermIDs = append(t.allTermIDs, tids...)
	}
	return nil
}

// collectImages notes the image of each of the image fields of the nodes
// ids (a field with several - its first; none - the field's default),
// its file read later (resolveImages).
func (t *teaserBatch) collectImages(ctx context.Context, images entity.FieldInstanceDefs, ids []int) error {
	if len(images) == 0 || t.b.files == nil {
		return nil
	}
	fileIDs, err := t.b.contentTypes.FieldValuesForNodes(ctx, ids, images)
	if err != nil {
		return err
	}
	for _, f := range images {
		for _, id := range ids {
			if fid := entity.FirstFileID(fileIDs[id][f.ID]); fid > 0 {
				t.imageRefs = append(t.imageRefs, teaserImageRef{nodeID: id, field: f.MachineName, fileID: fid})
			} else if f.DefaultImageID != nil {
				t.imageRefs = append(t.imageRefs, teaserImageRef{nodeID: id, field: f.MachineName, fileID: *f.DefaultImageID, isDefault: true})
			}
		}
	}
	return nil
}

// loadScalars reads the values of the scalar fields of the nodes ids; an
// empty date is left out.
func (t *teaserBatch) loadScalars(ctx context.Context, scalar entity.FieldInstanceDefs, ids []int) error {
	if len(scalar) == 0 {
		return nil
	}
	values, err := t.b.contentTypes.FieldValuesForNodes(ctx, ids, scalar)
	if err != nil {
		return err
	}
	for _, f := range scalar {
		for _, id := range ids {
			if v, ok := values[id][f.ID]; ok && !emptyTeaserDate(v) {
				t.itemByID[id].Fields[f.MachineName] = v
			}
		}
	}
	return nil
}

// emptyTeaserDate reports whether v is a date value with no date.
func emptyTeaserDate(v any) bool {
	switch d := v.(type) {
	case time.Time:
		return d.IsZero()
	case entity.DateValue:
		return d.Start.IsZero()
	case []entity.DateValue:
		return len(d) == 0
	}
	return false
}

// resolveImages reads the noted images' files, all at once.
func (t *teaserBatch) resolveImages(ctx context.Context) error {
	if len(t.imageRefs) == 0 {
		return nil
	}
	fileIDs := make([]int, len(t.imageRefs))
	for i, ref := range t.imageRefs {
		fileIDs[i] = ref.fileID
	}
	files, err := t.b.files.ByIDs(ctx, dedupeInts(fileIDs))
	if err != nil {
		return err
	}
	for _, ref := range t.imageRefs {
		f, ok := files[ref.fileID]
		switch {
		case !ok:
		case ref.isDefault:
			t.itemByID[ref.nodeID].DefaultImages[ref.field] = f
		default:
			t.itemByID[ref.nodeID].Images[ref.field] = f
		}
	}
	return nil
}

// resolveTerms reads the noted terms, all at once.
func (t *teaserBatch) resolveTerms(ctx context.Context) error {
	if len(t.allTermIDs) == 0 {
		return nil
	}
	terms, err := t.b.taxonomy.TermsByIDs(ctx, dedupeInts(t.allTermIDs))
	if err != nil {
		return err
	}
	for nid, byField := range t.nodeTermIDs {
		for machineName, tids := range byField {
			var out entity.TaxonomyTerms
			for _, tid := range tids {
				if term, ok := terms[tid]; ok {
					out = append(out, term)
				}
			}
			if len(out) > 0 {
				t.itemByID[nid].Terms[machineName] = out
			}
		}
	}
	return nil
}

// countComments sets the items' published comment counts.
func (t *teaserBatch) countComments(ctx context.Context) error {
	if t.b.comments == nil {
		return nil
	}
	nodeIDs := make([]int, len(t.items))
	for i := range t.items {
		nodeIDs[i] = t.items[i].Node.ID
	}
	counts, err := t.b.comments.CountByNodeIDs(ctx, nodeIDs, entity.CommentPublished, false)
	if err != nil {
		return err
	}
	for i := range t.items {
		t.items[i].CommentCount = counts[t.items[i].Node.ID]
	}
	return nil
}

func dedupeInts(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	out := ids[:0:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
