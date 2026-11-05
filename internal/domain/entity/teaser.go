package entity

// TeaserItem - one material rendered as a teaser card. The theme template
// teaser/<type>.html (fallback teaser/default.html) gets the node plus its
// field values keyed by machine_name, so card markup lives in the theme.
type TeaserItem struct {
	Node Node
	// TypeLabel - the content type's label (content_types.label).
	TypeLabel string
	// Fields - text/number/date/bool field values by machine_name.
	Fields map[string]any
	// Labels - field instance labels by machine_name.
	Labels map[string]string
	// Terms - taxonomy field terms by machine_name.
	Terms        map[string]TaxonomyTerms
	CommentCount int
	// Images - image field files by machine_name.
	Images map[string]File
	// DefaultImages - the default image (FieldInstanceDef.DefaultImageID)
	// of each image field the node leaves empty, by machine_name.
	DefaultImages map[string]File
	// AltTitle - the alternative title: the value of the type's
	// AltTitleField ("" - none).
	AltTitle string
	// BodyField - machine_name of the type's body text field (the first one
	// shown in the detail page body), "" when none.
	BodyField string
	// ImageField/ImageStyle - the type's first image field (its
	// machine_name, "" when none) and that field's list image style
	// ("" - the original): the engine's default teaser picture.
	ImageField string
	ImageStyle string
	// DateFormats - the chosen date format machine name per date field, by
	// machine_name; a theme teaser passes it to fieldDate.
	DateFormats map[string]string
}

// FeedImage - which image field (and style) a feed's items show.
type FeedImage struct {
	Field string
	Style string
}

// Image - an image field's file, else the field's default image (zero
// value - neither).
func (t TeaserItem) Image(machineName string) File {
	if f, ok := t.Images[machineName]; ok {
		return f
	}
	return t.DefaultImages[machineName]
}

// OwnImage - the node's own file of an image field, never the default
// image (feeds: no placeholder pictures in enclosures).
func (t TeaserItem) OwnImage(machineName string) File {
	return t.Images[machineName]
}

// MainImage - Image of the ImageField.
func (t TeaserItem) MainImage() File {
	return t.Image(t.ImageField)
}

// Body - the body text field's value (the RSS description, the
// [node:summary] source).
func (t TeaserItem) Body() string {
	return t.Text(t.BodyField)
}

// Text - a text field's value ("" when absent).
func (t TeaserItem) Text(machineName string) string {
	s, _ := t.Fields[machineName].(string)
	return s
}
