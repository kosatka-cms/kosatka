package entity

// DisplaySection is one rendered block of a node's detail page
// (dynamic/page.html), in DisplayOrder; every field renders at its own
// position (see DynamicContentController.buildDisplaySections). At most one
// of Group/Body/Image/Formatted/Blocks is set:
//   - Group: a FieldTypeGroup's boxed ".item-fieldgroup" card, Fields its
//     members.
//   - Body: a single RendersInBody FieldTypeText field as a full-width body
//     section.
//   - Image: a single FieldTypeImage field as a full-width picture block.
//   - Formatted: a single field a module's formatter shows
//     (DetailFormatter, its module on) as a full-width block - a link
//     field's videos, say.
//   - Blocks: a blocks field (FieldTypeBlocks) with blocks, each shown
//     by its field set's display - whatever its page group.
//   - none: a contiguous run of compact fields (text/number/date/bool/
//     taxonomy/list/file/link) in Fields, rendered together in one
//     ".item-characteristic" card.
type DisplaySection struct {
	Group     *FieldInstanceDef
	Body      *FieldInstanceDef
	Image     *FieldInstanceDef
	Formatted *FieldInstanceDef
	Blocks    *FieldInstanceDef
	Fields    FieldInstanceDefs
}

// DisplaySections - a node page's field blocks in display order.
type DisplaySections []DisplaySection

// Without - the sections without the given fields (by machine_name),
// for a theme template that places those fields by hand; a section left
// empty is dropped.
func (s DisplaySections) Without(machineNames ...string) DisplaySections {
	skip := make(map[string]bool, len(machineNames))
	for _, name := range machineNames {
		skip[name] = true
	}
	out := make(DisplaySections, 0, len(s))
	for _, sec := range s {
		switch {
		case sec.Image != nil:
			if skip[sec.Image.MachineName] {
				continue
			}
		case sec.Body != nil:
			if skip[sec.Body.MachineName] {
				continue
			}
		case sec.Formatted != nil:
			if skip[sec.Formatted.MachineName] {
				continue
			}
		case sec.Blocks != nil:
			if skip[sec.Blocks.MachineName] {
				continue
			}
		default:
			var fields FieldInstanceDefs
			for _, f := range sec.Fields {
				if !skip[f.MachineName] {
					fields = append(fields, f)
				}
			}
			if len(fields) == 0 {
				continue
			}
			sec.Fields = fields
		}
		out = append(out, sec)
	}
	return out
}
