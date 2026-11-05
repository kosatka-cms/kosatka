package forms

import (
	"context"
	"fmt"
	"html/template"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// embedPreviewQuestions - how many questions the editor's preview shows.
const embedPreviewQuestions = 4

// embedKind - the kind "form": [[form:<node ID>]] in a text shows the form
// as the "Form" block does (blocks/webform.html, webform_embed: forms.js
// loads it for each visitor, its address checks they may see it). The
// editor's window finds the forms by their title, its preview shows the
// questions.
func embedKind(webforms *WebformService, nodes modulekit.NodesRepository, templates modulekit.TemplateService) modulekit.EmbedKind {
	byName := func(ctx context.Context, name string) (*Webform, error) {
		id, err := strconv.Atoi(name)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%w: form %q", modulekit.ErrEntityNotFound, name)
		}
		wf, err := webforms.ByNodeID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading the embedded form %d: %w", id, err)
		}
		return wf, nil
	}
	return modulekit.EmbedKind{Name: "form", Module: manifest.Name, Label: i18n.N("Form"),
		Search: func(ctx context.Context, query string, limit int) ([]modulekit.EmbedOption, error) {
			hits, err := nodes.SearchByTitle(ctx, query, []string{string(manifest.ContentType)}, limit)
			if err != nil {
				return nil, fmt.Errorf("searching the forms: %w", err)
			}
			out := make([]modulekit.EmbedOption, len(hits))
			for i, h := range hits {
				out[i] = modulekit.EmbedOption{Name: strconv.Itoa(h.Node.ID), Title: h.Node.Title}
			}
			return out, nil
		},
		Describe: func(ctx context.Context, name string) (modulekit.EmbedOption, error) {
			wf, err := byName(ctx, name)
			if err != nil {
				return modulekit.EmbedOption{}, err
			}
			opt := modulekit.EmbedOption{Title: wf.Node.Title, Meta: i18n.Plural(ctx, len(wf.Elements), "@count question", "@count questions")}
			for _, e := range wf.Elements {
				if len(opt.Lines) < embedPreviewQuestions {
					opt.Lines = append(opt.Lines, e.Label)
				}
			}
			opt.More = len(wf.Elements) - len(opt.Lines)
			if wf.Closed {
				opt.Note = i18n.T(ctx, "Closed: it takes no submissions.")
			}
			return opt, nil
		},
		Render: func(ctx context.Context, name string) (template.HTML, error) {
			wf, err := byName(ctx, name)
			if err != nil {
				return "", err
			}
			return modulekit.EmbedTemplate(ctx, templates, "webform_embed", map[string]any{"NodeID": wf.Node.ID})
		}}
}
