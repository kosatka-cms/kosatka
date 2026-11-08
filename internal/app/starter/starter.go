// Package starter - what a new site starts with (app install), as data:
// starter.yaml describes the site's look, image styles, fields, content
// types, Views and menus; Apply creates them. Its labels, descriptions
// and titles are English, translated into the site's language on install
// (i18n-extract puts them in the catalogs).
package starter

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

//go:embed starter.yaml
var file []byte

// Set - the starter set (starter.yaml).
type Set struct {
	Site          Site           `yaml:"site"`
	ImageStyles   []ImageStyle   `yaml:"image_styles"`
	FieldStorages []FieldStorage `yaml:"field_storages"`
	ContentTypes  []ContentType  `yaml:"content_types"`
	Views         []View         `yaml:"views"`
	Menus         []Menu         `yaml:"menus"`
}

// Site - the site's theme (its directory), logo (a path on the site) and
// front page (a View of the set).
type Site struct {
	Theme         string `yaml:"theme"`
	Logo          string `yaml:"logo"`
	FrontPageView string `yaml:"front_page_view"`
}

// ImageStyle - an image style (entity.ImageStyleDef).
type ImageStyle struct {
	Name   string `yaml:"name"`
	Label  string `yaml:"label"`
	Width  int    `yaml:"width"`
	Height int    `yaml:"height"`
	Mode   string `yaml:"mode"`
	Anchor string `yaml:"anchor"`
	// HiDPI - its derivatives at double size too (srcset 2x).
	HiDPI bool `yaml:"hidpi"`
}

// FieldStorage - a field the content types share (entity.FieldStorageDef).
type FieldStorage struct {
	Name string                      `yaml:"name"`
	Type entity.ContentTypeFieldType `yaml:"type"`
}

// ContentType - a content type with its fields (entity.ContentTypeDef):
// the default comment mode, whether the author and date are shown,
// whether a new item is promoted to the front page, whether the type
// keeps the history of changes and whether its form previews.
type ContentType struct {
	Name          string             `yaml:"name"`
	Label         string             `yaml:"label"`
	Description   string             `yaml:"description"`
	Comments      entity.CommentMode `yaml:"comments"`
	ShowSubmitted bool               `yaml:"show_submitted"`
	Promote       bool               `yaml:"promote"`
	Revisions     bool               `yaml:"revisions"`
	// Preview - the form's Preview (entity.ContentTypeDef.NoPreview); on
	// unless false.
	Preview *bool   `yaml:"preview"`
	Fields  []Field `yaml:"fields"`
}

// NoPreview - the type's form without Preview: "preview: false".
func (t ContentType) NoPreview() bool { return t.Preview != nil && !*t.Preview }

// Field - a field storage on a content type (entity.FieldInstanceDef):
// the widget, the text formats it allows (by machine name), where it is
// shown and in what order on the form (SortOrder) and the page
// (DisplayOrder).
type Field struct {
	Storage          string   `yaml:"storage"`
	Label            string   `yaml:"label"`
	Widget           string   `yaml:"widget"`
	TextFormats      []string `yaml:"text_formats"`
	ShowOnDetail     bool     `yaml:"show_on_detail"`
	ShowLabel        bool     `yaml:"show_label"`
	RendersInBody    bool     `yaml:"renders_in_body"`
	DetailImageStyle string   `yaml:"detail_image_style"`
	ListImageStyle   string   `yaml:"list_image_style"`
	SortOrder        int      `yaml:"sort_order"`
	DisplayOrder     int      `yaml:"display_order"`
}

// View - a View of content (entity.ViewDisplayDef) with its fields,
// filters and sorts, in their order.
type View struct {
	Name        string                  `yaml:"name"`
	Label       string                  `yaml:"label"`
	DisplayType entity.ViewDisplayType  `yaml:"display_type"`
	Style       entity.ViewDisplayStyle `yaml:"style"`
	Pager       bool                    `yaml:"pager"`
	PageSize    int                     `yaml:"page_size"`
	Fields      []ViewField             `yaml:"fields"`
	Filters     []ViewFilter            `yaml:"filters"`
	Sorts       []ViewSort              `yaml:"sorts"`
}

// ViewField - a View's column: a field storage or a built-in field.
type ViewField struct {
	Storage    string `yaml:"storage"`
	Builtin    string `yaml:"builtin"`
	ImageStyle string `yaml:"image_style"`
	LinkTo     string `yaml:"link_to"`
	DateFormat string `yaml:"date_format"`
	TrimLength int    `yaml:"trim_length"`
}

// ViewFilter - a View's filter on a built-in field; Contextual - it reads
// the page being rendered (has_current_term: the term page's term).
type ViewFilter struct {
	Builtin    string `yaml:"builtin"`
	Operator   string `yaml:"operator"`
	Value      string `yaml:"value"`
	Contextual bool   `yaml:"contextual"`
}

// ViewSort - a View's sort on a built-in field.
type ViewSort struct {
	Builtin   string `yaml:"builtin"`
	Direction string `yaml:"direction"`
}

// Menu - a menu with its links, placed in a region of the site's theme.
type Menu struct {
	Name   string `yaml:"name"`
	Label  string `yaml:"label"`
	Region string `yaml:"region"`
	Links  []Link `yaml:"links"`
}

// Link - a menu link to an address on the site.
type Link struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

// Load reads the starter set and checks its references.
func Load() (*Set, error) {
	return parse(file)
}

func parse(data []byte) (*Set, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var s Set
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("the starter set: %w", err)
	}
	if err := s.check(); err != nil {
		return nil, fmt.Errorf("the starter set: %w", err)
	}
	return &s, nil
}

// check reports a name used twice or a reference to something the set
// doesn't have (a field's storage or image style, the front page View).
func (s *Set) check() error {
	styles, storages, views := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var errs []error
	add := func(kind, name string, seen map[string]bool) {
		if name == "" || seen[name] {
			errs = append(errs, fmt.Errorf("%s %q: an empty or repeated name", kind, name))
		}
		seen[name] = true
	}
	for _, st := range s.ImageStyles {
		add("image style", st.Name, styles)
	}
	for _, fs := range s.FieldStorages {
		add("field storage", fs.Name, storages)
	}
	types := map[string]bool{}
	for _, ct := range s.ContentTypes {
		add("content type", ct.Name, types)
		if !entity.IsValidCommentMode(string(ct.Comments)) {
			errs = append(errs, fmt.Errorf("content type %q: comment mode %q", ct.Name, ct.Comments))
		}
		for _, f := range ct.Fields {
			if !storages[f.Storage] {
				errs = append(errs, fmt.Errorf("content type %q: no field storage %q", ct.Name, f.Storage))
			}
			for _, style := range []string{f.DetailImageStyle, f.ListImageStyle} {
				if style != "" && !styles[style] {
					errs = append(errs, fmt.Errorf("content type %q: no image style %q", ct.Name, style))
				}
			}
		}
	}
	for _, v := range s.Views {
		add("view", v.Name, views)
		for _, f := range v.Fields {
			if (f.Storage == "") == (f.Builtin == "") {
				errs = append(errs, fmt.Errorf("view %q: a field is a storage or a built-in one", v.Name))
			}
			if f.Storage != "" && !storages[f.Storage] {
				errs = append(errs, fmt.Errorf("view %q: no field storage %q", v.Name, f.Storage))
			}
			if f.ImageStyle != "" && !styles[f.ImageStyle] {
				errs = append(errs, fmt.Errorf("view %q: no image style %q", v.Name, f.ImageStyle))
			}
		}
	}
	menus := map[string]bool{}
	for _, m := range s.Menus {
		add("menu", m.Name, menus)
	}
	if s.Site.FrontPageView != "" && !views[s.Site.FrontPageView] {
		errs = append(errs, fmt.Errorf("site: no front page view %q", s.Site.FrontPageView))
	}
	if s.Site.Theme == "" {
		errs = append(errs, errors.New("site: no theme"))
	}
	return errors.Join(errs...)
}
