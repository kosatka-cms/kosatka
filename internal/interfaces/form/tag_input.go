package form

import (
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TagInput is one parsed tag value from a Tagify hidden input's
// "5,7,new:Name" format: Type is "existing" (Value is the term ID as a
// string) or "new" (Value is the name of a term to create).
type TagInput struct {
	Type  string // "existing" or "new"
	Value string
}

// ParseTagsInput exports parseTagsInput for callers outside package form
// that resolve a Tagify input's "new:<name>"/plain-ID encoding (e.g.
// multi-value taxonomy fields in DynamicContentController).
func ParseTagsInput(tagsStr string) []TagInput {
	return parseTagsInput(tagsStr)
}

// parseTagsInput parses a raw tags string into TagInput values.
func parseTagsInput(tagsStr string) []TagInput {
	if tagsStr == "" {
		return []TagInput{}
	}

	tagValues := strings.Split(tagsStr, ",")
	inputs := make([]TagInput, 0, len(tagValues))

	for _, value := range tagValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if strings.HasPrefix(value, "new:") {
			inputs = append(inputs, TagInput{
				Type:  "new",
				Value: strings.TrimPrefix(value, "new:"),
			})
		} else {
			inputs = append(inputs, TagInput{
				Type:  "existing",
				Value: value, // kept as a string
			})
		}
	}

	return inputs
}

// TagSpecifications converts parsed Tagify TagInput values into
// entity.TagSpecification, ready for TaxonomyService.ProcessTagInputs.
// Shared by every content type's own tag-saving path (saveContentTaxonomy)
// and by DynamicContentController's multi-value taxonomy fields.
func TagSpecifications(tags []TagInput) []entity.TagSpecification {
	specs := make([]entity.TagSpecification, 0, len(tags))

	for _, tag := range tags {
		if tag.Type == "new" {
			specs = append(specs, entity.TagSpecification{
				Value: tag.Value,
				IsNew: true,
			})
		} else {
			specs = append(specs, entity.TagSpecification{
				Value: tag.Value,
				IsNew: false,
			})
		}
	}

	return specs
}
