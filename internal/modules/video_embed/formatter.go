package videoembed

import (
	"bytes"
	"context"
	"html/template"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The module's ways of showing a link field (modulekit.FieldFormatter):
// the videos' players, or their previews starting the player on a click.
const (
	formatterPlayer  = manifest.Name + "_player"
	formatterPreview = manifest.Name + "_preview"
)

// fieldTemplate - the module's template of a link field it shows (a
// theme may have its own), and its block; the videos in it are
// embedTemplate's.
const (
	fieldTemplate = "video_embed/field.html"
	fieldBlock    = "video_embed_field"
)

// fieldItem - one link of a link field as the template shows it: its
// video (Video; nil - not a video link of a service on: the link itself).
type fieldItem struct {
	Video *embedView
	Link  modulekit.Link
}

// FieldFormatters - the module's ways of showing a link field
// (modulekit.AddFieldFormatter): with the services' sites to offer as
// the field's allowed ones and the content form's preview of a link.
func (f *Filter) FieldFormatters() []modulekit.FieldFormatter {
	formatter := func(name, label string, clickToPlay bool) modulekit.FieldFormatter {
		return modulekit.FieldFormatter{Name: name, Module: manifest.Name, Label: label, FieldType: modulekit.FieldTypeLink,
			Render: f.fieldRender(clickToPlay), Hosts: f.sites, EditorScript: "video_embed_field"}
	}
	return []modulekit.FieldFormatter{
		formatter(formatterPlayer, i18n.N("Video: player"), false),
		formatter(formatterPreview, i18n.N("Video: preview, player on a click"), true),
	}
}

// sites - the hosts of the links of the services on.
func (f *Filter) sites(ctx context.Context) []string {
	on := f.service.Settings(ctx).On()
	var out []string
	for _, p := range providers {
		if on[p.Name] {
			out = append(out, p.Sites...)
		}
	}
	return out
}

// fieldRender - a link field's links as videos: the players (or, with
// clickToPlay, the previews), a link of no service on as a link; in the
// "featured" layout the first as chosen, the others as previews after it.
// A link's own title stands for its video's. "" when it can't be shown:
// the core shows the links.
func (f *Filter) fieldRender(clickToPlay bool) func(context.Context, any, modulekit.FieldLayout) template.HTML {
	return func(ctx context.Context, value any, layout modulekit.FieldLayout) template.HTML {
		links := modulekit.Links(value)
		if len(links) == 0 {
			return ""
		}
		settings := f.service.Settings(ctx)
		on := settings.On()
		items := make([]fieldItem, len(links))
		matched := make(map[int]Match, len(links))
		matches := make([]Match, 0, len(links))
		for i, l := range links {
			items[i].Link = l
			if m, ok := MatchURL(l.URL, on); ok {
				matched[i] = m
				matches = append(matches, m)
			}
		}
		embeds := f.service.Embeds(ctx, matches)
		for i, m := range matched {
			small := layout == modulekit.FieldLayoutFeatured && i > 0
			view := videoView(m, embeds[m.URL], settings, clickToPlay || small)
			if t := links[i].Title; t != "" {
				view.Title = t
			}
			items[i].Video = &view
		}
		var buf bytes.Buffer
		data := map[string]any{"Items": items, "Layout": string(layout), modulekit.RequestLanguageKey: i18n.FromContext(ctx)}
		if err := f.templates.RenderNamed(&buf, fieldBlock, data, fieldTemplate, embedTemplate); err != nil {
			modulekit.LogFallback(f.logger, err, "rendering a link field's videos (the links shown)")
			return ""
		}
		return template.HTML(buf.String())
	}
}
