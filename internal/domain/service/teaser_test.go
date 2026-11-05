package service

import (
	"context"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

type fakeTeaserContentTypes struct {
	entity.ContentTypeRepository
	defs   entity.ContentTypeDefs
	fields map[int]entity.FieldInstanceDefs
	values map[int]map[int]any
	single map[int]map[int]int
	multi  map[int]map[int][]int
}

func (f *fakeTeaserContentTypes) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return f.defs, nil
}

func (f *fakeTeaserContentTypes) FieldsByContentTypeID(_ context.Context, id int) (entity.FieldInstanceDefs, error) {
	return f.fields[id], nil
}

func (f *fakeTeaserContentTypes) FieldValuesForNodes(_ context.Context, _ []int, _ entity.FieldInstanceDefs) (map[int]map[int]any, error) {
	return f.values, nil
}

func (f *fakeTeaserContentTypes) FieldTaxonomyValuesForNodes(_ context.Context, storageID int, _ []int) (map[int]int, error) {
	return f.single[storageID], nil
}

func (f *fakeTeaserContentTypes) FieldTaxonomyMultiValuesForNodes(_ context.Context, storageID int, _ []int) (map[int][]int, error) {
	return f.multi[storageID], nil
}

type fakeTeaserTaxonomy struct {
	terms map[int]entity.TaxonomyTerm
}

func (f *fakeTeaserTaxonomy) TermsByIDs(context.Context, []int) (map[int]entity.TaxonomyTerm, error) {
	return f.terms, nil
}

type fakeTeaserFiles map[int]entity.File

func (f fakeTeaserFiles) ByIDs(_ context.Context, ids []int) (map[int]entity.File, error) {
	out := map[int]entity.File{}
	for _, id := range ids {
		if file, ok := f[id]; ok {
			out[id] = file
		}
	}
	return out, nil
}

type fakeTeaserComments map[int]int

func (f fakeTeaserComments) CountByNodeIDs(context.Context, []int, int8, bool) (map[int]int, error) {
	return f, nil
}

func TestTeaserBuilderBuild(t *testing.T) {
	premiere := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	// Video 1 and news 2 (free tags on a multi-value "tags" field).
	ct := &fakeTeaserContentTypes{
		defs: entity.ContentTypeDefs{
			{ID: 13, MachineName: "video", Label: "Video", AltTitleField: "title_alt"},
			{ID: 21, MachineName: "news", Label: "News"},
		},
		fields: map[int]entity.FieldInstanceDefs{
			13: {
				{ID: 1, FieldStorageID: 101, MachineName: "title_alt", Label: "Alternative title", FieldType: entity.FieldTypeText},
				{ID: 2, FieldStorageID: 102, MachineName: "date_global", Label: "World date", FieldType: entity.FieldTypeDate, DateFormat: "medium"},
				{ID: 3, FieldStorageID: 103, MachineName: "date_local", Label: "Local date", FieldType: entity.FieldTypeDate},
				{ID: 4, FieldStorageID: 104, MachineName: "field_kind", Label: "Kind", FieldType: entity.FieldTypeTaxonomy},
				{ID: 8, FieldStorageID: 110, MachineName: "cover_image", Label: "Cover", FieldType: entity.FieldTypeImage, ListImageStyle: "thumb"},
			},
			21: {
				{ID: 6, FieldStorageID: 106, MachineName: "content_short", Label: "Teaser", FieldType: entity.FieldTypeText, RendersInBody: true},
				{ID: 5, FieldStorageID: 105, MachineName: "content", Label: "Text", FieldType: entity.FieldTypeText, RendersInBody: true, ShowOnDetail: true},
				{ID: 7, FieldStorageID: 107, MachineName: "tags", Label: "Tags", FieldType: entity.FieldTypeTaxonomy, Config: map[string]any{"multiple": true, "vocabulary_id": 3}},
			},
		},
		values: map[int]map[int]any{
			1: {1: "Title", 2: premiere, 3: time.Time{}, 8: 55},
			2: {5: "<p>Text</p>"},
		},
		single: map[int]map[int]int{104: {1: 2847}},
		multi:  map[int]map[int][]int{107: {2: {7}}},
	}
	tax := &fakeTeaserTaxonomy{
		terms: map[int]entity.TaxonomyTerm{2847: {ID: 2847, Name: "Kind A"}, 7: {ID: 7, Name: "Tag"}},
	}
	b := NewTeaserBuilder(ct, tax, fakeTeaserComments{2: 4})
	b.SetFiles(fakeTeaserFiles{55: {ID: 55, URI: "public://cover.jpg"}})
	items, err := b.Build(t.Context(), entity.Nodes{{ID: 1, Type: "video"}, {ID: 2, Type: "news"}})
	if err != nil {
		t.Fatal(err)
	}
	video, news := items[0], items[1]
	if video.BodyField != "" || news.BodyField != "content" {
		t.Fatalf("body fields = %q, %q", video.BodyField, news.BodyField)
	}
	if video.Image("cover_image").URI != "public://cover.jpg" || news.Image("cover_image").URI != "" {
		t.Fatalf("images = %v, %v", video.Images, news.Images)
	}
	if video.ImageField != "cover_image" || video.ImageStyle != "thumb" || video.MainImage().URI != "public://cover.jpg" || news.ImageField != "" {
		t.Fatalf("main image = %q/%q, news %q", video.ImageField, video.ImageStyle, news.ImageField)
	}
	if _, isImage := video.Fields["cover_image"]; isImage {
		t.Fatal("an image field is not a text value")
	}
	if video.AltTitle != "Title" || news.AltTitle != "" {
		t.Fatalf("alt titles = %q, %q", video.AltTitle, news.AltTitle)
	}
	if video.TypeLabel != "Video" || video.Text("title_alt") != "Title" || video.Labels["date_global"] != "World date" {
		t.Fatalf("video item = %+v", video)
	}
	if video.DateFormats["date_global"] != "medium" || len(video.DateFormats) != 1 || len(news.DateFormats) != 0 {
		t.Fatalf("date formats = %v, %v", video.DateFormats, news.DateFormats)
	}
	if video.Fields["date_global"] != premiere {
		t.Fatalf("date_global = %v", video.Fields["date_global"])
	}
	if _, has := video.Fields["date_local"]; has {
		t.Fatal("a zero date must be left out")
	}
	if got := video.Terms["field_kind"]; len(got) != 1 || got[0].Name != "Kind A" {
		t.Fatalf("field_kind terms = %v", got)
	}
	if _, has := video.Terms["tags"]; has {
		t.Fatal("a type without a tags field gets no tags")
	}
	if got := news.Terms["tags"]; len(got) != 1 || got[0].Name != "Tag" {
		t.Fatalf("tags = %v", got)
	}
	if news.CommentCount != 4 || news.Text("content") != "<p>Text</p>" || news.Body() != "<p>Text</p>" {
		t.Fatalf("news item = %+v", news)
	}
}

// TestPlainExcerpt - the text of HTML, blocks apart, cut at a word with
// "…" (a View field's TrimLength).
func TestPlainExcerpt(t *testing.T) {
	for _, c := range []struct {
		in    string
		limit int
		want  string
	}{
		{"<p>Short.</p>", 50, "Short."},
		{"<p>One two three.</p><p>Four five.</p>", 0, "One two three. Four five."},
		{"<p>One two three.</p><p>Four five.</p>", 16, "One two three…"},
		{"<p>Один два три четыре</p>", 12, "Один два…"},
		{"<p>Hello &amp; world</p>", 100, "Hello & world"},
		// Characters, not bytes: a Cyrillic letter takes two.
		{"один два три", 12, "один два три"},
	} {
		if got := PlainExcerpt(c.in, c.limit); got != c.want {
			t.Errorf("PlainExcerpt(%q, %d) = %q, want %q", c.in, c.limit, got, c.want)
		}
	}
}

// zzTeaserPage - {{renderTeaser .}} on the active theme's teaser and
// field templates.
func zzTeaserPage(t *testing.T) *template.Template {
	t.Helper()
	files, err := paths.GlobTemplates(filepath.Join("teaser", "*.html"))
	if err != nil || len(files) == 0 {
		t.Fatalf("teaser templates: %v %v", files, err)
	}
	fields, err := paths.GlobTemplates(filepath.Join("fields", "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, fields...)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	set, err := svc.getOrParse("teaser-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	page, err := set.New("teaser_test_page").Parse(`{{renderTeaser .}}`)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

// TestRenderTeaser - a type with its own teaser template (the fixture
// theme's teaser_articles) uses it, any other type falls back to the
// engine's teaser_default.
func TestRenderTeaser(t *testing.T) {
	zzChdirToProjectRoot(t)
	zzUseFixtureTheme(t)
	page := zzTeaserPage(t)
	for typ, want := range map[string]string{
		"articles": `<div class="fixture-teaser"><a href="/x/title">Title</a> <span>Label</span></div>`,
		"anything": `<h3 class="node-teaser-title"><a href="/x/title">Title</a></h3>`,
	} {
		item := entity.TeaserItem{
			Node:      entity.Node{ID: 1, Type: entity.ContentType(typ), Title: "Title", Slug: "x/title"},
			Images:    map[string]entity.File{"cover_image": {URI: "public://a.jpg"}},
			TypeLabel: "Label",
		}
		var buf strings.Builder
		if err := page.Execute(&buf, item); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if got := buf.String(); !strings.Contains(got, want) || (typ != "articles") == strings.Contains(got, "fixture-teaser") {
			t.Fatalf("%s teaser lacks %q:\n%s", typ, want, got)
		}
	}
}

// TestFieldDateTemplates - a date field renders through the engine's
// field_date_default (plain date); a zero date renders nothing (a theme's
// field_date_<machine name>: TestFieldDateRendering's fixture "release").
func TestFieldDateTemplates(t *testing.T) {
	zzChdirToProjectRoot(t)
	future := time.Date(time.Now().Year()+3, 12, 31, 0, 0, 0, 0, time.UTC)
	render := func(t *testing.T, field string, v time.Time) string {
		t.Helper()
		files, err := paths.GlobTemplates(filepath.Join("fields", "*.html"))
		if err != nil || len(files) == 0 {
			t.Fatalf("field templates: %v %v", files, err)
		}
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		set, err := svc.getOrParse("fields-test", nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		page, err := set.New("fields_test_page").Parse(`{{renderSuggestion "field_date" .F (fieldDate .V "")}}`)
		if err != nil {
			t.Fatal(err)
		}
		var buf strings.Builder
		if err := page.Execute(&buf, map[string]any{"F": field, "V": v}); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	want := future.Format("02.01.2006")
	if got := render(t, "premiere_us", future); got != want {
		t.Fatalf("engine default: got %q, want %q", got, want)
	}
	if got := render(t, "premiere_us", time.Time{}); got != "" {
		t.Fatalf("zero date: got %q", got)
	}
}

// TestEngineDefaultTeaser - without a theme the default teaser shows the
// type's first image field in its list style, the title and the start
// of the body text, linked by the node's path.
func TestEngineDefaultTeaser(t *testing.T) {
	zzChdirToProjectRoot(t)
	files, err := paths.GlobTemplates(filepath.Join("teaser", "*.html"))
	if err != nil || len(files) == 0 {
		t.Fatalf("teaser templates: %v %v", files, err)
	}
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	set, err := svc.getOrParse("engine-teaser-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	page, err := set.New("engine_teaser_page").Parse(`{{renderTeaser .}}`)
	if err != nil {
		t.Fatal(err)
	}
	render := func(item entity.TeaserItem) string {
		var buf strings.Builder
		if err := page.Execute(&buf, item); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	item := entity.TeaserItem{
		Node:       entity.Node{ID: 1, Type: "place", Title: "Gotham", Slug: "places/gotham"},
		Fields:     map[string]any{"body": "<p>City.</p>"},
		Images:     map[string]entity.File{"photo": {URI: "public://g.jpg"}},
		BodyField:  "body",
		ImageField: "photo",
		ImageStyle: "thumb",
	}
	out := render(item)
	for _, want := range []string{`href="/places/gotham"`, `src="/uploads/styles/thumb/g.webp"`, ">Gotham</a>", "City."} {
		if !strings.Contains(out, want) {
			t.Fatalf("default teaser lacks %q:\n%s", want, out)
		}
	}
	item.ImageStyle = ""
	if out := render(item); !strings.Contains(out, `src="/uploads/g.jpg"`) {
		t.Fatalf("no list style - the original:\n%s", out)
	}
	item.ImageField = ""
	if out := render(item); strings.Contains(out, "<img") {
		t.Fatalf("no image field - no picture:\n%s", out)
	}
}

// TestFieldBoolTemplates - a boolean field renders through the engine's
// field_bool_default (yes/no) unless the theme defines
// field_bool_<machine name>.
func TestFieldBoolTemplates(t *testing.T) {
	zzChdirToProjectRoot(t)
	files, err := paths.GlobTemplates(filepath.Join("fields", "*.html"))
	if err != nil || len(files) == 0 {
		t.Fatalf("field templates: %v %v", files, err)
	}
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	set, err := svc.getOrParse("fields-bool-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := set.New("field_bool_custom").Parse(`{{if .}}present{{else}}absent{{end}}`); err != nil {
		t.Fatal(err)
	}
	page, err := set.New("bool_test_page").Parse(`{{renderSuggestion "field_bool" .F .V}}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		field string
		v     bool
		want  string
	}{{"in_stock", true, "Yes"}, {"in_stock", false, "No"}, {"custom", true, "present"}} {
		var buf strings.Builder
		if err := page.Execute(&buf, map[string]any{"F": c.field, "V": c.v}); err != nil {
			t.Fatal(err)
		}
		if buf.String() != c.want {
			t.Fatalf("%s=%v: got %q, want %q", c.field, c.v, buf.String(), c.want)
		}
	}
}

// TestViewGroupLabel - date headings by year/month/day, the rendered
// field otherwise (also for a date with no format), "" for no value.
// Russian is requested explicitly: the month heading checks a Russian
// month name in the nominative case.
func TestViewGroupLabel(t *testing.T) {
	d := time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		v      any
		format string
		want   template.HTML
	}{
		{d, "year", "2027"}, {d, "month", "Декабрь 2027"}, {d, "day", "31.12.2027"},
		{d, "", "<span>??.??.2027</span>"}, {"Harbour", "month", "<span>??.??.2027</span>"},
		{time.Time{}, "year", ""},
	} {
		if got := viewGroupLabel(c.v, c.format, " <span>??.??.2027</span> ", "ru"); got != c.want {
			t.Errorf("ViewGroupLabel(%v, %q) = %q, want %q", c.v, c.format, got, c.want)
		}
	}
}

// TestGroupViewRows - rows grouped by a field's value in order of first
// appearance (equal values together whatever the order), rows without a
// value under an empty heading, one heading-less group without a field,
// none without rows.
func TestGroupViewRows(t *testing.T) {
	type row struct {
		Name   string
		Fields entity.FieldInstanceDefs
		Values map[int]any
	}
	team := entity.FieldInstanceDef{ID: 1, MachineName: "team"}
	mk := func(name string, v any) row {
		values := map[int]any{}
		if v != nil {
			values[1] = v
		}
		return row{Name: name, Fields: entity.FieldInstanceDefs{team}, Values: values}
	}
	rows := []row{mk("Alice", "Sailing"), mk("Bob", "Rowing"), mk("Carol", "Sailing"), mk("Dave", nil)}
	render := func(_ entity.FieldInstanceDef, v, _ any) (template.HTML, error) {
		return template.HTML("<b>" + v.(string) + "</b>"), nil
	}
	groups, err := groupViewRowsIn(rows, "team", "", i18n.Default(), sitetime.In, render)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, g := range groups {
		names := string(g.Heading) + ":"
		for _, it := range g.Items {
			names += " " + it.(row).Name
		}
		got = append(got, names)
	}
	want := []string{"<b>Sailing</b>: Alice Carol", "<b>Rowing</b>: Bob", ": Dave"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("groups = %q, want %q", got, want)
	}
	if g, _ := groupViewRowsIn(rows, "", "", i18n.Default(), sitetime.In, render); len(g) != 1 || g[0].Heading != "" || len(g[0].Items) != 4 {
		t.Fatalf("no field: %+v", g)
	}
	if g, _ := groupViewRowsIn([]row{}, "team", "", i18n.Default(), sitetime.In, render); g != nil {
		t.Fatalf("no rows: %+v", g)
	}
}

// TestTeaserDefaultImage - a node without a value of an image field shows
// the field's default image; OwnImage (feeds) stays empty for it.
func TestTeaserDefaultImage(t *testing.T) {
	defaultID := 90
	ct := &fakeTeaserContentTypes{
		defs: entity.ContentTypeDefs{{ID: 1, MachineName: "article", Label: "Article"}},
		fields: map[int]entity.FieldInstanceDefs{
			1: {{ID: 8, FieldStorageID: 110, MachineName: "cover_image", FieldType: entity.FieldTypeImage, DefaultImageID: &defaultID}},
		},
		values: map[int]map[int]any{1: {8: 55}},
	}
	b := NewTeaserBuilder(ct, &fakeTeaserTaxonomy{}, fakeTeaserComments{})
	b.SetFiles(fakeTeaserFiles{55: {ID: 55, URI: "public://own.jpg"}, 90: {ID: 90, URI: "public://default.png"}})
	items, err := b.Build(t.Context(), entity.Nodes{{ID: 1, Type: "article"}, {ID: 2, Type: "article"}})
	if err != nil {
		t.Fatal(err)
	}
	own, empty := items[0], items[1]
	if own.Image("cover_image").URI != "public://own.jpg" || own.OwnImage("cover_image").URI != "public://own.jpg" {
		t.Errorf("own image: %+v", own.Images)
	}
	if empty.Image("cover_image").URI != "public://default.png" || empty.MainImage().URI != "public://default.png" || empty.OwnImage("cover_image").URI != "" {
		t.Errorf("default image: %+v / %+v", empty.Images, empty.DefaultImages)
	}
}
