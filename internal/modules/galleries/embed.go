package galleries

import (
	"context"
	"fmt"
	"html/template"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// embedKind - the kind "gallery": [[gallery:<ID>]] in a text shows the
// gallery as its content page does (blocks/node_galleries.html,
// galleries_list), to a visitor its gallery page opens for (nodeOpen:
// they may see its content, which can have galleries); a hidden or
// empty gallery - nothing. The editor's window finds the galleries by
// their title or their content's, its preview shows their pictures.
func (c *GalleryController) embedKind() modulekit.EmbedKind {
	return modulekit.EmbedKind{Name: "gallery", Module: manifest.Name, Label: i18n.N("Gallery"), Search: c.searchEmbeds, Describe: c.describeEmbed, Render: func(ctx context.Context, name string) (template.HTML, error) {
		id, err := strconv.Atoi(name)
		if err != nil || id <= 0 {
			return "", fmt.Errorf("%w: gallery %q", modulekit.ErrEntityNotFound, name)
		}
		gallery, err := c.galleryService.ByID(ctx, id)
		if err != nil {
			return "", fmt.Errorf("loading the embedded gallery %d: %w", id, err)
		}
		if !gallery.Status || gallery.ItemsCount() == 0 {
			return "", nil
		}
		node, err := c.nodeService.ByID(ctx, gallery.NodeID)
		if err != nil {
			return "", fmt.Errorf("loading the content of the embedded gallery %d: %w", id, err)
		}
		var user *modulekit.User
		if uid, ok := modulekit.ContextUserID(ctx); ok && uid > 0 {
			if user, err = c.UserService().ByID(ctx, uid); err != nil {
				return "", fmt.Errorf("loading the visitor of the embedded gallery %d: %w", id, err)
			}
		}
		if !c.nodeOpen(ctx, node, user) {
			return "", nil
		}
		return modulekit.EmbedTemplate(ctx, c.TemplateService(), "galleries_list", map[string]any{"Galleries": []*Gallery{gallery}})
	}}
}

// embedPreviewThumbs - how many pictures the editor's preview shows.
const embedPreviewThumbs = 6

// searchEmbeds - the published galleries whose title (or their content's)
// has query, of the content ctx's visitor may see (the "gallery" kind's
// Search).
func (c *GalleryController) searchEmbeds(ctx context.Context, query string, limit int) ([]modulekit.EmbedOption, error) {
	published := true
	filter := GalleriesFilter{Status: &published, Limit: limit, Query: query}
	if query != "" {
		hits, err := c.NodeRepo().SearchByTitle(ctx, query, nil, limit)
		if err != nil {
			return nil, fmt.Errorf("searching the galleries' content: %w", err)
		}
		for _, h := range hits {
			filter.NodeIDs = append(filter.NodeIDs, h.Node.ID)
		}
	}
	found, err := c.galleryService.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("searching the galleries: %w", err)
	}
	var out []modulekit.EmbedOption
	for _, g := range found {
		// The content's node isn't found for a visitor it is closed to.
		if g.Node == nil || g.ItemsCount() == 0 {
			continue
		}
		opt := modulekit.EmbedOption{Name: strconv.Itoa(g.ID), Title: g.GetTitle(), Meta: c.galleryMeta(ctx, g)}
		if first := g.GetFirstItem(); first != nil && first.PreviewSmall != nil {
			opt.Thumbs = []string{first.PreviewSmall.URI}
		}
		out = append(out, opt)
	}
	return out, nil
}

// galleryMeta - "12 photos · From "The trip"".
func (c *GalleryController) galleryMeta(ctx context.Context, g *Gallery) string {
	meta := i18n.Plural(ctx, g.ItemsCount(), "@count photo", "@count photos")
	if g.Node != nil {
		meta += " · " + i18n.T(ctx, "From \"@title\"", "@title", g.Node.Title)
	}
	return meta
}

// describeEmbed - a gallery for the editor: its first pictures and how
// many more (the "gallery" kind's Describe).
func (c *GalleryController) describeEmbed(ctx context.Context, name string) (modulekit.EmbedOption, error) {
	id, err := strconv.Atoi(name)
	if err != nil || id <= 0 {
		return modulekit.EmbedOption{}, fmt.Errorf("%w: gallery %q", modulekit.ErrEntityNotFound, name)
	}
	g, err := c.galleryService.ByID(ctx, id)
	if err != nil {
		return modulekit.EmbedOption{}, fmt.Errorf("loading the gallery %d: %w", id, err)
	}
	if g.Node == nil {
		if g.Node, err = c.nodeService.ByID(ctx, g.NodeID); err != nil {
			return modulekit.EmbedOption{}, fmt.Errorf("loading the content of the gallery %d: %w", id, err)
		}
	}
	opt := modulekit.EmbedOption{Title: g.GetTitle(), Meta: c.galleryMeta(ctx, g),
		Note: i18n.T(ctx, "Seen by those who may see \"@title\".", "@title", g.Node.Title)}
	for _, it := range g.GetPreviewItems(embedPreviewThumbs) {
		if it.PreviewSmall != nil {
			opt.Thumbs = append(opt.Thumbs, it.PreviewSmall.URI)
		}
	}
	opt.More = max(g.ItemsCount()-len(opt.Thumbs), 0)
	switch {
	case !g.Status:
		opt.Note = i18n.T(ctx, "Hidden: nothing shows in its place.")
	case g.ItemsCount() == 0:
		opt.Note = i18n.T(ctx, "No pictures yet: nothing shows in its place.")
	}
	return opt, nil
}
