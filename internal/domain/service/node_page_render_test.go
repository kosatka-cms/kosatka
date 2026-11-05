package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/assets"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
)

// TestNodeBodyValueEscaping - a body section outputs HTML only for a
// RichText field; a plain text field rendered in the body is escaped.
func TestNodeBodyValueEscaping(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("node-body-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	page, err := set.New("node_body_test").Parse(`{{template "node_body_value" .}}`)
	if err != nil {
		t.Fatal(err)
	}
	value := `<p>text</p><img src=x onerror=alert(1)>`
	for _, tc := range []struct {
		field entity.FieldInstanceDef
		want  string
	}{
		{entity.FieldInstanceDef{FieldType: entity.FieldTypeText}, "&lt;p&gt;text&lt;/p&gt;&lt;img src=x onerror=alert(1)&gt;"},
		{entity.FieldInstanceDef{FieldType: entity.FieldTypeText, WidgetConfig: map[string]any{"allowed_text_formats": []any{float64(1)}}}, "<p>text</p>"},
	} {
		var buf strings.Builder
		if err := page.Execute(&buf, map[string]any{"Field": &tc.field, "Value": value}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), tc.want) {
			t.Errorf("rich=%v: no %q in %s", tc.field.RichText(), tc.want, buf.String())
		}
	}
}

// TestStandardNodePage - the standard page_default (no theme override):
// fields one after another, an image field as its own block, no floated
// picture (a tall one marked for a theme); a theme's page_node
// (page/node.html) overrides it for every type.
func TestStandardNodePage(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("node-page-std-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	image := entity.FieldInstanceDef{ID: 5, MachineName: "poster", Label: "Poster", FieldType: entity.FieldTypeImage}
	data := map[string]any{
		"item":            entity.DynamicContent{Node: entity.Node{ID: 1, Type: "x", Title: "Title", Slug: "x/title", Status: 1, CommentMode: entity.CommentModeHidden}},
		"contentType":     entity.ContentTypeDef{},
		"displaySections": []map[string]any{{"Image": &image}},
		"imageFieldURLs":  map[int]string{5: "/poster.webp"},
		"imageFieldItems": map[int][]struct {
			Picture imageurl.Picture
			Alt     string
		}{5: {{Picture: imageurl.Picture{Src: "/poster.webp", Width: 600, Height: 400}}}},
		"imageFieldAlts":  map[int]string{},
		"fieldImages":     map[string]map[string]any{},
		"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{},
	}
	var buf strings.Builder
	if err := set.ExecuteTemplate(&buf, "page_default", data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `class="dynamic-field dynamic-field-image"`) || !strings.Contains(out, `src="/poster.webp"`) || strings.Contains(out, "node-main-picture") {
		t.Fatalf("standard page:\n%s", out)
	}
	if set.Lookup("page_node") != nil {
		t.Fatal("the engine has no page_node: a theme's layout for every type lives in the theme")
	}
	// A picture taller than wide (a cover) is marked for a theme.
	data["imageFieldItems"] = map[int][]struct {
		Picture imageurl.Picture
		Alt     string
	}{5: {{Picture: imageurl.Picture{Src: "/poster.webp", Width: 600, Height: 900}}}}
	buf.Reset()
	if err := set.ExecuteTemplate(&buf, "page_default", data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `class="dynamic-field dynamic-field-image dynamic-field-image-portrait"`) {
		t.Fatalf("a portrait picture:\n%s", buf.String())
	}
}

// TestNodeImagesCaptionEscaped - the alt of an image in a field with
// several images reaches the lightbox's caption (set as HTML) as text.
func TestNodeImagesCaptionEscaped(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("node-images-caption-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	images := entity.FieldInstanceDef{ID: 5, MachineName: "photos", Label: "Photos", FieldType: entity.FieldTypeImage, Config: map[string]any{"multiple": true}}
	data := map[string]any{
		"item":            entity.DynamicContent{Node: entity.Node{ID: 1, Type: "x", Title: "Title", Slug: "x/title", Status: 1, CommentMode: entity.CommentModeHidden}},
		"contentType":     entity.ContentTypeDef{},
		"displaySections": []map[string]any{{"Image": &images}},
		"imageFieldItems": map[int][]struct {
			Picture  imageurl.Picture
			Alt      string
			Original string
			Lightbox imageurl.Picture
		}{5: {{Picture: imageurl.Picture{Src: "/a.webp", Width: 600, Height: 400}, Alt: `<img src=x onerror=alert(1)>`, Original: "/a.jpg", Lightbox: imageurl.Picture{Src: "/a.jpg", Width: 600, Height: 400}}}},
		"fieldImages":     map[string]map[string]any{},
		"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{},
	}
	var buf strings.Builder
	if err := set.ExecuteTemplate(&buf, "page_default", data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// The attribute holds the caption's HTML; the alt in it is escaped
	// once more, so the browser's decoded value is &lt;img…, not a tag.
	if !strings.Contains(out, `data-pswp-caption-html="&lt;div class=&#34;pswp-caption-block&#34;&gt;&lt;div class=&#34;pswp-caption-title&#34;&gt;&amp;lt;img src=x onerror=alert(1)&amp;gt;`) {
		t.Fatalf("the caption isn't escaped:\n%s", out)
	}
}

// TestRenderSuggestionChain - page_<type>, else the theme's page_node,
// else the standard page_default; renderOptional has no fallback.
func TestRenderSuggestionChain(t *testing.T) {
	set := template.Must(template.New("x").Parse(`{{define "page_default"}}default{{end}}{{define "page_news"}}news{{end}}`))
	for typ, want := range map[string]string{"news": "news", "article": "default"} {
		if got, _ := renderSuggestion(set, "page", typ, nil, true, false); string(got) != want {
			t.Errorf("without page_node, %s: %q, want %q", typ, got, want)
		}
	}
	set = template.Must(template.New("y").Parse(`{{define "page_default"}}default{{end}}{{define "page_news"}}news{{end}}{{define "page_node"}}node{{end}}`))
	for typ, want := range map[string]string{"news": "news", "article": "node"} {
		if got, _ := renderSuggestion(set, "page", typ, nil, true, false); string(got) != want {
			t.Errorf("with page_node, %s: %q, want %q", typ, got, want)
		}
	}
	if got, _ := renderSuggestion(set, "page", "article", nil, false, false); got != "" {
		t.Errorf("renderOptional fell back to %q", got)
	}
	// A field value's templates are by field: no _node step, and a field
	// named "node" gets its own template, not a type-wide default.
	set = template.Must(template.New("z").Parse(`{{define "field_date_default"}}default{{end}}{{define "field_date_node"}}node{{end}}`))
	for field, want := range map[string]string{"release": "default", "node": "node"} {
		if got, _ := renderSuggestion(set, "field_date", field, nil, true, false); string(got) != want {
			t.Errorf("field %s: %q, want %q", field, got, want)
		}
	}
	set = template.Must(template.New("w").Parse(`{{define "field_bool_default"}}default{{end}}{{define "field_bool_node"}}node{{end}}`))
	if got, _ := renderSuggestion(set, "field_bool", "featured", nil, true, false); string(got) != "default" {
		t.Errorf("a yes/no field took field_bool_node: %q", got)
	}
}

// TestThemeHooks - without a theme the page template set still parses and
// the hooks give the engine's neutral look (base.css, no editor CSS); with
// a theme they emit its stylesheets instead.
func TestThemeHooks(t *testing.T) {
	zzChdirToProjectRoot(t)
	render := func() string {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
		if err != nil {
			t.Fatal(err)
		}
		set, err := svc.getOrParse("theme-hooks", nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		page, err := set.New("theme_hooks_test").Parse(`{{template "theme_head" .}}|{{template "theme_editor_css" .}}`)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		if err := page.Execute(&b, map[string]any{}); err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(b.String())
	}

	if got := render(); !strings.Contains(got, "/static/css/base.min.css") || !strings.HasSuffix(got, "|") {
		t.Errorf("no theme: want the engine's base.css and no editor CSS, got %q", got)
	}
	zzUseFixtureTheme(t)
	got := render()
	if strings.Contains(got, "/static/css/base.min.css") {
		t.Errorf("a theme replaces the engine's base.css: %q", got)
	}
	if !strings.Contains(got, "/static/themes/"+zzFixtureTheme+"/css/styles.min.css") || !strings.HasSuffix(got, "|/static/themes/"+zzFixtureTheme+"/css/styles.min.css?v="+assets.Version()) {
		t.Errorf("theme hooks not rendered: %q", got)
	}
}

// TestThemePageOverride - a theme's page_<type> (page/<type>.html) renders
// that type's node page, with a partial of the theme's own; another type
// gets the engine's page_default.
func TestThemePageOverride(t *testing.T) {
	zzChdirToProjectRoot(t)
	zzUseFixtureTheme(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("theme-page-test", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	page, err := set.New("theme_page_test").Parse(`{{renderSuggestion "page" .item.Node.Type .}}`)
	if err != nil {
		t.Fatal(err)
	}
	for typ, themed := range map[string]bool{"articles": true, "story": false} {
		data := map[string]any{
			"item":            entity.DynamicContent{Node: entity.Node{ID: 1, Type: entity.ContentType(typ), Title: "Title", Slug: typ + "/x", Status: 1}},
			"contentType":     entity.ContentTypeDef{},
			"fieldValues":     map[string]any{"content": "<p>Body</p>"},
			"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{},
		}
		var buf strings.Builder
		if err := page.Execute(&buf, data); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if got := strings.Contains(buf.String(), `<article class="fixture-page"><h1>Title</h1><p class="fixture-byline">#1</p>`); got != themed {
			t.Errorf("%s: the theme's page %v, want %v:\n%s", typ, got, themed, buf.String())
		}
	}
}
