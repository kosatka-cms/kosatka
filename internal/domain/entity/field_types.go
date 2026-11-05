package entity

import "encoding/json"

// FieldTypes - every field type, in the order the admin offers them. A
// new type goes here and into fieldTypeDefs; the completeness tests of
// each layer (storage, revisions, the form, the page, Views) then fail
// until it's handled there.
var FieldTypes = []ContentTypeFieldType{
	FieldTypeText, FieldTypeNumber, FieldTypeDate, FieldTypeBool, FieldTypeTaxonomy, FieldTypeReference,
	FieldTypeImage, FieldTypeFile, FieldTypeList, FieldTypeLink, FieldTypeUser, FieldTypeEmail, FieldTypePhone,
	FieldTypeBlocks, FieldTypeGroup,
}

// fieldTypeDef - what the domain knows of a field type: whether a field
// of it can hold several values (multiple; FieldInstanceDef.Multiple) and
// how a revision's value of it is read back (decode, NodeSnapshot.Values:
// raw as the field holds values now; nil - no value, a group).
type fieldTypeDef struct {
	multiple bool
	decode   func(f FieldInstanceDef, raw json.RawMessage) (any, bool)
}

var fieldTypeDefs = map[ContentTypeFieldType]fieldTypeDef{
	FieldTypeText:   {decode: decodeAs[string]},
	FieldTypeEmail:  {decode: decodeAs[string]},
	FieldTypePhone:  {decode: decodeAs[string]},
	FieldTypeNumber: {decode: decodeAs[float64]},
	FieldTypeBool:   {decode: decodeAs[bool]},
	FieldTypeDate:   {multiple: true, decode: decodeDate},
	FieldTypeList:   {multiple: true, decode: decodeOneOrMany[string]},
	// The IDs of terms, content, files.
	FieldTypeTaxonomy:  {multiple: true, decode: decodeOneOrMany[int]},
	FieldTypeReference: {multiple: true, decode: decodeOneOrMany[int]},
	FieldTypeImage:     {multiple: true, decode: decodeOneOrMany[int]},
	FieldTypeFile:      {decode: decodeOneOrMany[int]},
	// Links and users: a slice whatever the cardinality.
	FieldTypeLink: {multiple: true, decode: decodeAs[[]Link]},
	FieldTypeUser: {multiple: true, decode: decodeAs[[]int]},
	// Items with their Snapshot: resolved with their sets' fields.
	FieldTypeBlocks: {decode: decodeAs[[]BlockItem]},
	FieldTypeGroup:  {},
}

// AllowsMultiple - the field type can hold several values per node
// (FieldInstanceDef.Multiple): taxonomy, list, reference, image, link,
// date and user fields.
func (t ContentTypeFieldType) AllowsMultiple() bool {
	return fieldTypeDefs[t].multiple
}

// decodeAs - raw as a T.
func decodeAs[T any](_ FieldInstanceDef, raw json.RawMessage) (any, bool) {
	var v T
	return v, json.Unmarshal(raw, &v) == nil
}

// decodeOneOrMany - raw as a T, or a []T for a field with several values.
func decodeOneOrMany[T any](f FieldInstanceDef, raw json.RawMessage) (any, bool) {
	if f.Multiple() {
		return decodeAs[[]T](f, raw)
	}
	return decodeAs[T](f, raw)
}

// decodeDate - several dates (an array), a date (a string) or one with a
// time or an end (DateValue's object), read as the field holds them now.
func decodeDate(f FieldInstanceDef, raw json.RawMessage) (any, bool) {
	var list []DateValue
	if json.Unmarshal(raw, &list) != nil {
		var v DateValue
		if json.Unmarshal(raw, &v) != nil {
			return nil, false
		}
		list = []DateValue{v}
	}
	switch {
	case len(list) == 0:
		return nil, false
	case f.Multiple():
		return list, true
	case f.DateRich():
		return list[0], true
	}
	return list[0].Start, true
}
