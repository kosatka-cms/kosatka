package polls

import (
	"context"
	"fmt"
	"html/template"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// embedPreviewOptions - how many options the editor's preview shows.
const embedPreviewOptions = 4

// embedKind - the kind "poll": [[poll:<node ID>]] in a text shows the poll
// as the "Poll" block does (blocks/poll.html, poll_embed: polls.js loads
// it for each visitor, its address checks they may see it). The editor's
// window finds the polls by their question, its preview shows the
// options.
func embedKind(polls PollService, nodes modulekit.NodesRepository, templates modulekit.TemplateService) modulekit.EmbedKind {
	byName := func(ctx context.Context, name string) (*Poll, error) {
		id, err := strconv.Atoi(name)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%w: poll %q", modulekit.ErrEntityNotFound, name)
		}
		p, err := polls.ByNodeID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("loading the embedded poll %d: %w", id, err)
		}
		return p, nil
	}
	return modulekit.EmbedKind{Name: "poll", Module: manifest.Name, Label: i18n.N("Poll"),
		Search: func(ctx context.Context, query string, limit int) ([]modulekit.EmbedOption, error) {
			hits, err := nodes.SearchByTitle(ctx, query, []string{string(manifest.ContentType)}, limit)
			if err != nil {
				return nil, fmt.Errorf("searching the polls: %w", err)
			}
			out := make([]modulekit.EmbedOption, len(hits))
			for i, h := range hits {
				out[i] = modulekit.EmbedOption{Name: strconv.Itoa(h.Node.ID), Title: h.Node.Title}
			}
			return out, nil
		},
		Describe: func(ctx context.Context, name string) (modulekit.EmbedOption, error) {
			p, err := byName(ctx, name)
			if err != nil {
				return modulekit.EmbedOption{}, err
			}
			opt := modulekit.EmbedOption{Title: p.Node.Title, Meta: i18n.Plural(ctx, len(p.Options), "@count option", "@count options"),
				Note: i18n.T(ctx, "Each visitor sees their own vote.")}
			for _, o := range p.Options {
				if len(opt.Lines) < embedPreviewOptions {
					opt.Lines = append(opt.Lines, o.Label)
				}
			}
			opt.More = len(p.Options) - len(opt.Lines)
			return opt, nil
		},
		Render: func(ctx context.Context, name string) (template.HTML, error) {
			p, err := byName(ctx, name)
			if err != nil {
				return "", err
			}
			return modulekit.EmbedTemplate(ctx, templates, "poll_embed", map[string]any{"NodeID": p.Node.ID})
		}}
}
