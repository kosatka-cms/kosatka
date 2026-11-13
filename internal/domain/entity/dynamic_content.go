package entity

import "context"

// DynamicContentRepository - port for a content-type-scoped instance of the
// generic repository (see database.NewDynamicContentRepository). Built per
// request via a factory, since the content type is known only after the
// node/path is resolved.
type DynamicContentRepository interface {
	Create(ctx context.Context, e *DynamicContent) (*DynamicContent, error)
	Update(ctx context.Context, e *DynamicContent) (*DynamicContent, error)
	ByNodeID(ctx context.Context, nodeID int) (*DynamicContent, error)
}

// DynamicContent is the entity for a node of an admin-created content type.
// It has no static fields: Values holds one entry per field, keyed by
// FieldInstanceDef.ID and typed per FieldType (string for text, float64 for
// number, time.Time for date, bool for bool, int for taxonomy/reference/image -
// a term, node or file ID).
type DynamicContent struct {
	Node        Node
	ContentType ContentTypeDef
	Fields      FieldInstanceDefs
	Values      map[int]any
	// TextFormats - keyed like Values: the TextFormatDef.ID chosen for each
	// FieldTypeText value that uses text formats. Absent for other fields
	// (text_format_id stays NULL).
	TextFormats map[int]int
	Tags        TaxonomyTerms
}

// GetNode (with GetContent) implements
// interfaces.SEOProvider, so DynamicContent works with the generic
// PageSEO() helper.
func (d DynamicContent) GetNode() Node { return d.Node }

// GetContent returns the value of the first text field that renders in the
// body, is shown on the detail page and is not in a group - the closest thing
// to a body field, used as a meta description fallback. The group check
// matters because grouped fields may also be ShowOnDetail and Fields is
// sorted by SortOrder, so a grouped field could otherwise win.
func (d DynamicContent) GetContent() string {
	for _, f := range d.Fields {
		if f.FieldType == FieldTypeText && f.RendersInBody && f.ShowOnDetail && f.ParentGroupID == nil {
			if s, ok := d.Values[f.ID].(string); ok {
				return s
			}
		}
	}
	return ""
}

// FieldValueByMachineName returns the stored value of the field with the
// given machine name (typed per FieldType), or nil if the field is not
// attached or not set.
func (d DynamicContent) FieldValueByMachineName(machineName string) any {
	for _, f := range d.Fields {
		if f.MachineName == machineName {
			return d.Values[f.ID]
		}
	}
	return nil
}
