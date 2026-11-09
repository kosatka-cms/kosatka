// Package demoset - the demo sets of "app install -demo-set <name>": a
// whole demo site as data, sets/<name>/<lang>.yaml (or a theme's own,
// <theme>/demo/<lang>.yaml) - its theme, users,
// vocabularies, fields, content types, Views, blocks, menus and content
// (nodes with their field values, comments, view counts), in one
// language. Apply creates it on top of the starter set (starter.yaml),
// whose fields and content types it may use. Everything it creates is
// ordinary content and configuration, edited and deleted in the admin;
// nothing of a set is in code.
package demoset

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kosatka-cms/kosatka/internal/app/starter"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/demo"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

//go:embed sets
var embeddedSets embed.FS

// setsFS - where the sets are read: sets/<name>/<lang>.yaml (the
// embedded ones; a test swaps in its own - UseSets).
var setsFS fs.FS = embeddedSets

// UseSets reads the sets from fsys (sets/<name>/<lang>.yaml) instead of
// the embedded ones until restore is called: for tests.
func UseSets(fsys fs.FS) (restore func()) {
	prev := setsFS
	setsFS = fsys
	return func() { setsFS = prev }
}

// Set - a demo set in one language.
type Set struct {
	Site          Site                 `yaml:"site"`
	Users         []string             `yaml:"users"`
	ImageStyles   []starter.ImageStyle `yaml:"image_styles"`
	TextFormats   []TextFormat         `yaml:"text_formats"`
	Vocabularies  []Vocabulary         `yaml:"vocabularies"`
	FieldStorages []FieldStorage       `yaml:"field_storages"`
	ContentTypes  []ContentType        `yaml:"content_types"`
	Nodes         []Node               `yaml:"nodes"`
	Views         []View               `yaml:"views"`
	Blocks        []Block              `yaml:"blocks"`
	Menus         []Menu               `yaml:"menus"`
	// Permissions - what roles may (their names) beyond the starter's:
	// "create blog content", "use_text_format_basic_html"…
	Permissions map[string][]string `yaml:"permissions"`
	// Modules - the modules' demo content by module name, each in the
	// format of the module's demo/<lang>.json; a module the site doesn't
	// have is skipped.
	Modules map[string]any `yaml:"modules"`
	// FromTheme - a theme's own set (<theme>/demo/<lang>.yaml): a site
	// built without a module it has content for (a module of its own
	// repository) installs without that content (Absent).
	FromTheme bool `yaml:"-"`
	// files - the set's directory (Load): its own pictures (Pictures);
	// nil - a set parsed alone.
	files fs.FS
}

// Absent - the modules the set has content for that this build hasn't
// (has - whether it has one), sorted: for a theme's own set they are
// skipped, for the engine's an error.
func (s *Set) Absent(has func(string) bool) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(s.Modules)) {
		if !has(name) {
			out = append(out, name)
		}
	}
	return out
}

// Site - the site's theme (its directory; "" - the starter's), slogan,
// front page (a View of the set or of the starter) and the menus the
// modules' demo content links from.
type Site struct {
	Theme         string `yaml:"theme"`
	Slogan        string `yaml:"slogan"`
	FrontPageView string `yaml:"front_page_view"`
	// MainMenu and FooterMenu - the menus (machine names) the modules'
	// demo steps add their links to.
	MainMenu   string `yaml:"main_menu"`
	FooterMenu string `yaml:"footer_menu"`
}

// TextFormat - a text format of the set's own, beside the core's
// (entity.TextFormatBasicHTML, entity.TextFormatPlainText): its machine
// name (what content types' fields name in text_formats), its name, the
// HTML tags it keeps (space-separated; never script or style) and the
// class attribute on them, the visual editor with its TinyMCE plugins and
// toolbar (empty - the editor's own). Weight - its place among the
// formats: a field's first format by weight is the one its texts are
// saved in, the set's content too. Who may use it beyond the
// administrators - the set's permissions ("use_text_format_<name>").
type TextFormat struct {
	Name        string `yaml:"name"`
	Label       string `yaml:"label"`
	Weight      int    `yaml:"weight"`
	AllowedTags string `yaml:"allowed_tags"`
	AllowClass  bool   `yaml:"allow_class"`
	RichEditor  bool   `yaml:"rich_editor"`
	Plugins     string `yaml:"plugins"`
	Toolbar     string `yaml:"toolbar"`
}

// Vocabulary - a vocabulary: its machine name (what fields and blocks
// name it by), its name, the address its term pages go under and its
// terms in order.
type Vocabulary struct {
	Name  string `yaml:"name"`
	Label string `yaml:"label"`
	Path  string `yaml:"path"`
	Terms []Term `yaml:"terms"`
}

// Term - a term: a name, or a mapping with its description too.
type Term struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// UnmarshalYAML takes a term as its name alone or as a mapping.
func (t *Term) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		t.Name = n.Value
		return nil
	}
	type plain Term
	return n.Decode((*plain)(t))
}

// FieldStorage - a field the content types share: its type and settings -
// several values, its vocabulary (taxonomy), its target content types
// (reference), its options (list) and any other setting of the type
// (Config, as the admin stores it).
type FieldStorage struct {
	Name       string                      `yaml:"name"`
	Type       entity.ContentTypeFieldType `yaml:"type"`
	Multiple   bool                        `yaml:"multiple"`
	Vocabulary string                      `yaml:"vocabulary"`
	Target     []string                    `yaml:"target"`
	Options    []Option                    `yaml:"options"`
	Config     map[string]any              `yaml:"config"`
}

// Option - a list field's option: its value and label.
type Option struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

// ContentType - a content type with its fields; Path - the address its
// items go under (<path>/<title transliterated>; "" - its name).
type ContentType struct {
	Name          string             `yaml:"name"`
	Label         string             `yaml:"label"`
	Description   string             `yaml:"description"`
	Path          string             `yaml:"path"`
	Comments      entity.CommentMode `yaml:"comments"`
	ShowSubmitted bool               `yaml:"show_submitted"`
	ShowViewCount bool               `yaml:"show_view_count"`
	Promote       bool               `yaml:"promote"`
	Revisions     bool               `yaml:"revisions"`
	// Preview - the form's Preview (entity.ContentTypeDef.NoPreview); on
	// unless false.
	Preview *bool   `yaml:"preview"`
	Fields  []Field `yaml:"fields"`
}

// NoPreview - the type's form without Preview: "preview: false".
func (t ContentType) NoPreview() bool { return t.Preview != nil && !*t.Preview }

// Field - a field storage (the set's or the starter's) on a content
// type; Widget - its form control (entity.FieldWidget), DateFormat - a
// date's format on the page.
type Field struct {
	Storage          string   `yaml:"storage"`
	Label            string   `yaml:"label"`
	Widget           string   `yaml:"widget"`
	Required         bool     `yaml:"required"`
	TextFormats      []string `yaml:"text_formats"`
	ShowOnDetail     bool     `yaml:"show_on_detail"`
	ShowLabel        bool     `yaml:"show_label"`
	RendersInBody    bool     `yaml:"renders_in_body"`
	DetailImageStyle string   `yaml:"detail_image_style"`
	ListImageStyle   string   `yaml:"list_image_style"`
	DateFormat       string   `yaml:"date_format"`
	SortOrder        int      `yaml:"sort_order"`
	DisplayOrder     int      `yaml:"display_order"`
}

// Node - a published content item: a key other items and menus name it
// by, its type, title and author (a user of the set; "" - the
// administrator), when it was created (Ago before the install, e.g.
// "3d", "5h"; "" - now), its flags, address ("" - its type's), view
// count, field values by storage name (see Apply) and comments.
type Node struct {
	Key      string             `yaml:"key"`
	Type     string             `yaml:"type"`
	Title    string             `yaml:"title"`
	Author   string             `yaml:"author"`
	Ago      string             `yaml:"ago"`
	Promote  *bool              `yaml:"promote"`
	Sticky   bool               `yaml:"sticky"`
	Comments entity.CommentMode `yaml:"comment_mode"`
	Path     string             `yaml:"path"`
	Views    int                `yaml:"views"`
	Fields   map[string]any     `yaml:"fields"`
	Replies  []demo.Comment     `yaml:"comments"`
}

// View - a View of content: a page (Path), a block (placed by Blocks), an
// attachment to the pages of content types (AttachTypes) or the pages of
// terms (TermVocabularies); its fields, filters and sorts in order.
type View struct {
	Name        string                  `yaml:"name"`
	Label       string                  `yaml:"label"`
	DisplayType entity.ViewDisplayType  `yaml:"display_type"`
	Path        string                  `yaml:"path"`
	Style       entity.ViewDisplayStyle `yaml:"style"`
	GridColumns int                     `yaml:"grid_columns"`
	Pager       bool                    `yaml:"pager"`
	PageSize    int                     `yaml:"page_size"`
	// Skip - the first Skip items left out (entity.ViewDisplayDef.SkipItems).
	Skip             int      `yaml:"skip"`
	AttachTypes      []string `yaml:"attach_types"`
	AttachTitle      string   `yaml:"attach_title"`
	TermVocabularies []string `yaml:"term_vocabularies"`
	// ThemeTemplate - a template of the site's theme rendering the list
	// (views/<name>.html, entity.ViewDisplayDef.ThemeTemplate); "" - the
	// style.
	ThemeTemplate   string `yaml:"theme_template"`
	GroupBy         string `yaml:"group_by"`
	GroupDateFormat string `yaml:"group_date_format"`
	Header          string `yaml:"header"`
	// TabPath and TabAccess - a profile tab's (display_type user_tab):
	// its address under a profile and who sees it (all, owner).
	TabPath   string       `yaml:"tab_path"`
	TabAccess string       `yaml:"tab_access"`
	Fields    []ViewField  `yaml:"fields"`
	Filters   []ViewFilter `yaml:"filters"`
	Sorts     []ViewSort   `yaml:"sorts"`
}

// ViewField - a View's column: a field storage or a built-in field.
type ViewField struct {
	Storage    string `yaml:"storage"`
	Builtin    string `yaml:"builtin"`
	Label      string `yaml:"label"`
	ShowLabel  bool   `yaml:"show_label"`
	ImageStyle string `yaml:"image_style"`
	LinkTo     string `yaml:"link_to"`
	DateFormat string `yaml:"date_format"`
	TrimLength int    `yaml:"trim_length"`
	TagFormat  string `yaml:"tag_format"`
	CSSClass   string `yaml:"css_class"`
	// Params - a module's field's settings (a key with the module's
	// prefix, "ratings.rating").
	Params map[string]string `yaml:"params"`
}

// ViewFilter - a View's filter on a field storage or a built-in field:
// fixed (Value; Relative - a relative date), exposed to visitors
// (Exposed - the query parameter, ExposedLabel) or contextual.
type ViewFilter struct {
	Storage      string `yaml:"storage"`
	Builtin      string `yaml:"builtin"`
	Operator     string `yaml:"operator"`
	Value        string `yaml:"value"`
	Relative     bool   `yaml:"relative"`
	Exposed      string `yaml:"exposed"`
	ExposedLabel string `yaml:"exposed_label"`
	Contextual   bool   `yaml:"contextual"`
}

// ViewSort - a View's sort on a field storage or a built-in field.
type ViewSort struct {
	Storage   string            `yaml:"storage"`
	Builtin   string            `yaml:"builtin"`
	Direction string            `yaml:"direction"`
	Params    map[string]string `yaml:"params"`
	// Exposed - offered to visitors as ?sort=<Exposed>
	// (entity.ViewSortDef.ExposedKey), labelled ExposedLabel.
	Exposed      string `yaml:"exposed"`
	ExposedLabel string `yaml:"exposed_label"`
}

// Block - a block placed in a region of the site's theme: a View of the
// set (View), a plugin block (Plugin, entity.PluginBlockID; Vocabulary -
// a tag cloud's, Config - its other settings) or a text block (Body,
// Basic HTML); on every page, or only the front page (Visibility
// "homepage") or the pages of content types (Types).
type Block struct {
	Region     string            `yaml:"region"`
	Title      string            `yaml:"title"`
	ShowTitle  bool              `yaml:"show_title"`
	Weight     int               `yaml:"weight"`
	Visibility string            `yaml:"visibility"`
	Types      []string          `yaml:"types"`
	View       string            `yaml:"view"`
	Plugin     string            `yaml:"plugin"`
	Vocabulary string            `yaml:"vocabulary"`
	Config     map[string]string `yaml:"config"`
	Body       string            `yaml:"body"`
}

// Menu - a menu with its links, placed in a region of the theme; a menu
// of the starter (main) gets the links after its own.
type Menu struct {
	Name   string `yaml:"name"`
	Label  string `yaml:"label"`
	Region string `yaml:"region"`
	Links  []Link `yaml:"links"`
}

// Link - a menu link: to an address (URL), a node of the set (Node, its
// key), a View page of the set (View) or a term (Term, "vocabulary/term
// name").
type Link struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
	Node  string `yaml:"node"`
	View  string `yaml:"view"`
	Term  string `yaml:"term"`
}

// themeSetDir - the directory of a theme's own demo set,
// <theme>/demo/<lang>.yaml, named by the theme's directory (the first of
// paths.ThemesDirs having it); "" - none.
const themeSetDir = "demo"

// source - where set name is read: an embedded set (sets/<name>), else
// the demo set of theme name (its demo/ directory); false - neither.
// A theme's set is the theme's own: fromTheme.
func source(name string) (fsys fs.FS, dir string, fromTheme bool, ok bool) {
	if fi, err := fs.Stat(setsFS, path.Join("sets", name)); err == nil && fi.IsDir() && name != "" {
		return setsFS, path.Join("sets", name), false, true
	}
	if dir := paths.ThemeDir(name); dir != "" {
		if hasSets(os.DirFS(dir), themeSetDir) {
			return os.DirFS(dir), themeSetDir, true, true
		}
	}
	return nil, "", false, false
}

// hasSets - whether dir of fsys has a <lang>.yaml.
func hasSets(fsys fs.FS, dir string) bool {
	matches, _ := fs.Glob(fsys, path.Join(dir, "*.yaml"))
	return len(matches) > 0
}

// Names - the sets there are, sorted: the embedded ones and the themes'
// own (a theme with demo/<lang>.yaml, by its directory's name; an
// embedded set of the same name hides it).
func Names() []string {
	entries, _ := fs.ReadDir(setsFS, "sets")
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	for _, root := range paths.ThemesDirs() {
		themes, _ := os.ReadDir(root)
		for _, th := range themes {
			if th.IsDir() && !strings.HasPrefix(th.Name(), ".") && !slices.Contains(out, th.Name()) && hasSets(os.DirFS(filepath.Join(root, th.Name())), themeSetDir) {
				out = append(out, th.Name())
			}
		}
	}
	slices.Sort(out)
	return out
}

// Languages - the languages set name is in, sorted.
func Languages(name string) []string {
	fsys, dir, _, ok := source(name)
	if !ok {
		return nil
	}
	entries, _ := fs.ReadDir(fsys, dir)
	var out []string
	for _, e := range entries {
		if lang, ok := strings.CutSuffix(e.Name(), ".yaml"); ok {
			out = append(out, lang)
		}
	}
	return out
}

// Load reads set name in lang and checks it, its own files too
// (CheckFiles). A theme's own set without a theme of its own
// (site.theme) puts the site in that theme.
func Load(name, lang string) (*Set, error) {
	fsys, dir, fromTheme, ok := source(name)
	if !ok {
		return nil, fmt.Errorf("no demo set %q (there are: %s)", name, strings.Join(Names(), ", "))
	}
	data, err := fs.ReadFile(fsys, path.Join(dir, lang+".yaml"))
	if err != nil {
		return nil, fmt.Errorf("demo set %q has no %q (it has: %s)", name, lang, strings.Join(Languages(name), ", "))
	}
	s, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("demo set %s/%s: %w", name, lang, err)
	}
	if s.files, err = fs.Sub(fsys, dir); err != nil {
		return nil, fmt.Errorf("demo set %s: %w", name, err)
	}
	if errs := s.CheckFiles(s.files); len(errs) > 0 {
		return nil, fmt.Errorf("demo set %s/%s: %w", name, lang, errors.Join(errs...))
	}
	if fromTheme {
		s.FromTheme = true
		if s.Site.Theme == "" {
			s.Site.Theme = name
		}
	}
	return s, nil
}

// Regions - the theme regions the set places its blocks and menus in,
// each once, in order of first use.
func (s *Set) Regions() []string {
	var out []string
	add := func(r string) {
		if r != "" && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	for _, b := range s.Blocks {
		add(b.Region)
	}
	for _, m := range s.Menus {
		add(m.Region)
	}
	return out
}

// Parse decodes a set (unknown keys are errors) and checks it.
func Parse(data []byte) (*Set, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var s Set
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	return &s, nil
}

// machineName - a text format's machine name (the column's 64
// characters at most).
var machineName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// names - a set's names of one kind, checked unique and not empty.
type names struct {
	kind string
	seen map[string]bool
	errs *[]error
}

func (n names) add(name string) {
	if name == "" || n.seen[name] {
		*n.errs = append(*n.errs, fmt.Errorf("%s %q: an empty or repeated name", n.kind, name))
	}
	n.seen[name] = true
}

// check reports a name used twice and a reference to something the set
// (or, an image style, the starter) doesn't have. A reference to the
// starter's fields, content types and Views is checked by Apply, which
// sees them.
func (s *Set) check() error {
	var errs []error
	kinds := map[string]names{}
	for _, k := range []string{"user", "image style", "text format", "vocabulary", "field storage", "content type", "node", "view", "menu"} {
		kinds[k] = names{kind: k, seen: map[string]bool{}, errs: &errs}
	}
	for _, u := range s.Users {
		kinds["user"].add(u)
	}
	for _, st := range s.ImageStyles {
		kinds["image style"].add(st.Name)
	}
	// The starter's image styles are the set's to use too.
	styles := maps.Clone(kinds["image style"].seen)
	if base, err := starter.Load(); err == nil {
		for _, st := range base.ImageStyles {
			styles[st.Name] = true
		}
	}
	// The core's formats and the set's are its fields' to use.
	formats := map[string]bool{entity.TextFormatBasicHTML: true, entity.TextFormatPlainText: true}
	for _, f := range s.TextFormats {
		kinds["text format"].add(f.Name)
		switch {
		case formats[f.Name]:
			errs = append(errs, fmt.Errorf("text format %q: the core's", f.Name))
		case !machineName.MatchString(f.Name):
			errs = append(errs, fmt.Errorf("text format %q: a machine name - lowercase Latin letters, digits and _, starting with a letter", f.Name))
		case f.Label == "":
			errs = append(errs, fmt.Errorf("text format %q: no label", f.Name))
		case (entity.TextFormatDef{AllowedTags: f.AllowedTags}).AllowUnsafeContent():
			errs = append(errs, fmt.Errorf("text format %q: a demo set's format keeps no script or style", f.Name))
		}
		formats[f.Name] = true
	}
	terms := map[string]map[string]bool{}
	for _, v := range s.Vocabularies {
		kinds["vocabulary"].add(v.Name)
		terms[v.Name] = map[string]bool{}
		for _, t := range v.Terms {
			terms[v.Name][t.Name] = true
		}
	}
	// vocabularyOf - a taxonomy storage's vocabulary: its values on the
	// nodes are terms of it.
	vocabularyOf := map[string]string{}
	for _, fs := range s.FieldStorages {
		kinds["field storage"].add(fs.Name)
		if fs.Type == entity.FieldTypeTaxonomy {
			vocabularyOf[fs.Name] = fs.Vocabulary
		}
		if fs.Type == entity.FieldTypeTaxonomy && terms[fs.Vocabulary] == nil {
			errs = append(errs, fmt.Errorf("field storage %q: no vocabulary %q", fs.Name, fs.Vocabulary))
		}
		if fs.Type == entity.FieldTypeList && len(fs.Options) == 0 {
			errs = append(errs, fmt.Errorf("field storage %q: a list without options", fs.Name))
		}
	}
	for _, ct := range s.ContentTypes {
		kinds["content type"].add(ct.Name)
		if !entity.IsValidCommentMode(string(ct.Comments)) {
			errs = append(errs, fmt.Errorf("content type %q: comment mode %q", ct.Name, ct.Comments))
		}
		for _, f := range ct.Fields {
			for _, style := range []string{f.DetailImageStyle, f.ListImageStyle} {
				if style != "" && !styles[style] {
					errs = append(errs, fmt.Errorf("content type %q: no image style %q", ct.Name, style))
				}
			}
			for _, format := range f.TextFormats {
				if !formats[format] {
					errs = append(errs, fmt.Errorf("content type %q: no text format %q", ct.Name, format))
				}
			}
		}
	}
	for _, n := range s.Nodes {
		kinds["node"].add(n.Key)
		if n.Author != "" && !kinds["user"].seen[n.Author] {
			errs = append(errs, fmt.Errorf("node %q: no user %q", n.Key, n.Author))
		}
		if _, err := parseAgo(n.Ago); err != nil {
			errs = append(errs, fmt.Errorf("node %q: %w", n.Key, err))
		}
		errs = append(errs, checkComments(n.Key, n.Replies, kinds["user"].seen)...)
		for _, name := range slices.Sorted(maps.Keys(n.Fields)) {
			voc, taxonomy := vocabularyOf[name]
			for _, item := range fieldItems(n.Fields[name]) {
				if term := fmt.Sprint(item); taxonomy && terms[voc] != nil && !terms[voc][term] {
					errs = append(errs, fmt.Errorf("node %q: %s: no term %q in vocabulary %q", n.Key, name, term, voc))
				}
				if f, _ := pictureFile(item); f != "" {
					check := checkPictureName
					if s.storageType(name) == entity.FieldTypeFile {
						check = checkDocumentName
					}
					if err := check(f); err != nil {
						errs = append(errs, fmt.Errorf("node %q: %s: %w", n.Key, name, err))
					}
				}
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(s.Modules)) {
		moduleFiles(s.Modules[name], func(f string) {
			if err := checkPictureName(f); err != nil {
				errs = append(errs, fmt.Errorf("modules: %s: %w", name, err))
			}
		})
	}
	for _, v := range s.Views {
		kinds["view"].add(v.Name)
		for _, f := range v.Fields {
			if (f.Storage == "") == (f.Builtin == "") {
				errs = append(errs, fmt.Errorf("view %q: a field is a storage or a built-in one", v.Name))
			}
			if f.Builtin != "" && !views.IsModuleKey(f.Builtin) && !views.FieldBuiltinAllowed(entity.ViewFieldBuiltinField(f.Builtin)) {
				errs = append(errs, fmt.Errorf("view %q: no built-in field %q", v.Name, f.Builtin))
			}
		}
		for _, f := range v.Filters {
			if (f.Storage == "") == (f.Builtin == "") {
				errs = append(errs, fmt.Errorf("view %q: a filter is on a storage or a built-in field", v.Name))
			}
			if f.Builtin != "" && !views.IsModuleKey(f.Builtin) && !views.FilterBuiltinAllowed(entity.ViewFilterBuiltinField(f.Builtin)) {
				errs = append(errs, fmt.Errorf("view %q: no built-in filter %q", v.Name, f.Builtin))
			}
		}
		exposedSorts := map[string]bool{}
		for _, st := range v.Sorts {
			if st.Builtin != "" && !views.IsModuleKey(st.Builtin) && !views.SortBuiltinAllowed(entity.ViewSortBuiltin(st.Builtin)) {
				errs = append(errs, fmt.Errorf("view %q: no built-in sort %q", v.Name, st.Builtin))
			}
			if st.Exposed != "" {
				if err := views.ValidSortExposure(st.Exposed, st.ExposedLabel); err != nil {
					errs = append(errs, fmt.Errorf("view %q: sort offered as %q: %w", v.Name, st.Exposed, err))
				} else if exposedSorts[st.Exposed] {
					errs = append(errs, fmt.Errorf("view %q: sort offered as %q twice", v.Name, st.Exposed))
				}
				exposedSorts[st.Exposed] = true
			}
		}
		for _, voc := range v.TermVocabularies {
			if terms[voc] == nil {
				errs = append(errs, fmt.Errorf("view %q: no vocabulary %q", v.Name, voc))
			}
		}
	}
	for _, b := range s.Blocks {
		sources := 0
		for _, set := range []bool{b.View != "", b.Plugin != "", b.Body != ""} {
			if set {
				sources++
			}
		}
		if sources != 1 || b.Region == "" {
			errs = append(errs, fmt.Errorf("block %q: a region and one of view, plugin or body", b.Title))
		}
		if b.Vocabulary != "" && terms[b.Vocabulary] == nil {
			errs = append(errs, fmt.Errorf("block %q: no vocabulary %q", b.Title, b.Vocabulary))
		}
	}
	for _, m := range s.Menus {
		kinds["menu"].add(m.Name)
		for _, l := range m.Links {
			switch {
			case l.Node != "" && !kinds["node"].seen[l.Node]:
				errs = append(errs, fmt.Errorf("menu %q: no node %q", m.Name, l.Node))
			case l.View != "" && !kinds["view"].seen[l.View]:
				errs = append(errs, fmt.Errorf("menu %q: no view %q", m.Name, l.View))
			case l.Term != "":
				voc, term, _ := strings.Cut(l.Term, "/")
				if !terms[voc][term] {
					errs = append(errs, fmt.Errorf("menu %q: no term %q", m.Name, l.Term))
				}
			}
		}
	}
	return errors.Join(errs...)
}

// checkComments - the comments by users the set hasn't.
func checkComments(key string, list []demo.Comment, users map[string]bool) []error {
	var errs []error
	for _, c := range list {
		if !users[c.Author] {
			errs = append(errs, fmt.Errorf("node %q: a comment by no user %q", key, c.Author))
		}
		errs = append(errs, checkComments(key, c.Replies, users)...)
	}
	return errs
}
