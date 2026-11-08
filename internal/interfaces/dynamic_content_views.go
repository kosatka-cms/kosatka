package interfaces

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"maps"
	"net/url"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// attachContextualViewBlocks renders View blocks that have a contextual filter
// (entity.ViewFilterDef.Contextual) for the current node. CacheService skips
// them at refresh time since there is no current node then. Which blocks apply
// is decided by block placements.
//
// Only the "all" and "content_type" visibility scopes are supported here;
// other scopes combined with a contextual filter never render.
//
// data["viewBlockContent"] is a shared cache map read concurrently, so it is
// copied before modification (only when a contextual block is found).
func (c *DynamicContentController) attachContextualViewBlocks(ctx context.Context, data map[string]any, n entity.Node) {
	raw, _ := data["viewBlockContent"].(map[int]template.HTML)
	var out map[int]template.HTML

	for _, placements := range c.sidebarDataFor(ctx).BlockPlacements {
		for _, bp := range placements {
			if !bp.Placement.IsView() || bp.ViewDisplay == nil {
				continue
			}
			switch bp.Placement.VisibilityScope {
			case entity.BlockVisibilityAll:
			case entity.BlockVisibilityContentType:
				if !bp.Placement.MatchesContentType(string(n.Type)) {
					continue
				}
			default:
				continue
			}
			filters, err := c.viewService.EffectiveFiltersByView(ctx, *bp.ViewDisplay)
			if err != nil {
				c.LogFailure(err, "error loading view filters for contextual block", "placementID", bp.Placement.ID, "nodeID", n.ID)
				continue
			}
			if !filters.HasContextual() {
				continue
			}
			html, err := c.renderContextualViewBlock(ctx, *bp.ViewDisplay, n)
			if err != nil {
				c.LogFailure(err, "error rendering contextual view block", "placementID", bp.Placement.ID, "nodeID", n.ID)
				continue
			}
			if out == nil {
				out = make(map[int]template.HTML, len(raw)+1)
				maps.Copy(out, raw)
			}
			out[bp.Placement.ID] = html
			c.markCarousel(ctx, data, *bp.ViewDisplay)
		}
	}
	if out != nil {
		data["viewBlockContent"] = out
	}
}

// viewAttachment - one rendered attachment display for
// dynamic/page.html's "view_attachments" point.
type viewAttachment struct {
	MachineName string
	Title       string
	ShowTitle   bool
	HTML        template.HTML
}

// attachViewAttachments renders every attachment display
// (entity.ViewDisplayTypeAttachment) for n's content type into
// data["viewAttachments"], in AttachWeight order (AttachmentDisplays'
// order). They are rendered per request with n as the contextual node.
func (c *DynamicContentController) attachViewAttachments(ctx context.Context, data map[string]any, n entity.Node) {
	displays, err := c.viewService.AttachmentDisplays(ctx)
	if err != nil {
		c.LogFailure(err, "error loading view attachments", "nodeID", n.ID)
		return
	}
	var out []viewAttachment
	for _, d := range displays {
		if !d.MatchesContentType(string(n.Type)) {
			continue
		}
		html, count, err := c.renderContextualView(ctx, d, n)
		if err != nil {
			c.LogFailure(err, "error rendering view attachment", "viewID", d.ID, "nodeID", n.ID)
			continue
		}
		if count == 0 {
			continue
		}
		c.markCarousel(ctx, data, d)
		title := d.AttachTitle
		if title == "" {
			title = d.Label
		}
		out = append(out, viewAttachment{MachineName: d.MachineName, Title: title, ShowTitle: d.AttachShowTitle, HTML: html})
	}
	if out != nil {
		data["viewAttachments"] = out
	}
}

// markCarousel sets data["needsSwiperCSS"] when view renders as a
// carousel, so header.html loads the Swiper stylesheet for this page.
func (c *DynamicContentController) markCarousel(ctx context.Context, data map[string]any, view entity.ViewDisplayDef) {
	effective, err := c.viewService.EffectiveSettings(ctx, view)
	if err == nil && effective.DisplayStyle == entity.ViewDisplayStyleCarousel {
		data["needsSwiperCSS"] = true
	}
}

// renderContextualViewBlock renders a View block like
// CacheService.renderContentViewBlock, but runs the View live with n as the
// current node.
func (c *DynamicContentController) renderContextualViewBlock(ctx context.Context, view entity.ViewDisplayDef, n entity.Node) (template.HTML, error) {
	html, _, err := c.renderContextualView(ctx, view, n)
	return html, err
}

// renderContextualView is renderContextualViewBlock that also reports
// how many nodes the View returned - an attachment with no results is
// skipped entirely (attachViewAttachments), not shown as a bare heading.
func (c *DynamicContentController) renderContextualView(ctx context.Context, view entity.ViewDisplayDef, n entity.Node) (template.HTML, int, error) {
	nodes, _, err := c.viewService.RunViewWithContext(ctx, view, n.CreatedBy.ID, n.ID)
	if err != nil {
		return "", 0, fmt.Errorf("error running contextual content view for block: %w", err)
	}
	html, err := c.renderContentViewFragment(ctx, view, nodes, nil)
	return html, len(nodes), err
}

// RenderProfileTab renders a user_tab display (entity.ViewDisplayTypeUserTab)
// for the profile of userID - the created_by contextual filter is that
// user - as the View's own list markup with pagination (basePath and
// query build the page links). Returns the total number of results.
func (c *DynamicContentController) RenderProfileTab(ctx context.Context, view entity.ViewDisplayDef, userID, page int, basePath string, query url.Values) (template.HTML, int, error) {
	return c.renderEmbeddedView(ctx, view, userID, page, basePath, query)
}

// RenderFrontFeed renders the front page feed - the View named by
// SiteInformation.FrontFeedView - with its pager; ok is false when there
// is no such enabled View.
func (c *DynamicContentController) RenderFrontFeed(ctx context.Context, machineName string, page int, basePath string, query url.Values) (template.HTML, bool, error) {
	view, ok := c.enabledViewByMachineName(ctx, machineName)
	if !ok {
		return "", false, nil
	}
	html, _, err := c.renderEmbeddedView(ctx, *view, 0, page, basePath, query)
	return html, true, err
}

// FrontFeedItems - the first limit items of the front page feed View
// (RSS), as teaser items, and the View's first image field: its
// machine_name and image style ("" - the original).
func (c *DynamicContentController) FrontFeedItems(ctx context.Context, machineName string, limit int) ([]entity.TeaserItem, entity.FeedImage, error) {
	view, ok := c.enabledViewByMachineName(ctx, machineName)
	if !ok || c.teasers == nil {
		return nil, entity.FeedImage{}, nil
	}
	return c.viewFeedItems(ctx, *view, limit)
}

// viewFeedItems - the first limit items of view as teaser items and the
// View's first image field (machine_name and image style, "" - the
// original): the front page RSS's cover.
func (c *DynamicContentController) viewFeedItems(ctx context.Context, view entity.ViewDisplayDef, limit int) ([]entity.TeaserItem, entity.FeedImage, error) {
	var image entity.FeedImage
	if c.teasers == nil {
		return nil, image, nil
	}
	var nodes entity.Nodes
	for page := 1; len(nodes) < limit; page++ {
		batch, total, err := c.viewService.RunView(ctx, view, nil, page)
		if err != nil {
			return nil, image, err
		}
		nodes = append(nodes, batch...)
		if len(batch) == 0 || len(nodes) >= total {
			break
		}
	}
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}
	items, err := c.teasers.Build(ctx, nodes)
	if err != nil {
		return nil, image, err
	}
	fields, err := c.viewService.EffectiveFieldsByView(ctx, view)
	if err != nil {
		return nil, image, err
	}
	for _, f := range fields.SortBySortOrder() {
		if f.FieldStorageID == nil {
			continue
		}
		fs, err := c.contentTypeRepo.FieldStorageByID(ctx, *f.FieldStorageID)
		if err != nil || fs == nil || fs.FieldType != entity.FieldTypeImage {
			continue
		}
		image = entity.FeedImage{Field: fs.MachineName, Style: f.ImageStyle}
		break
	}
	return items, image, nil
}

func (c *DynamicContentController) enabledViewByMachineName(ctx context.Context, machineName string) (*entity.ViewDisplayDef, bool) {
	if machineName == "" {
		return nil, false
	}
	view, err := c.viewService.ViewByMachineName(ctx, machineName)
	if err != nil || view == nil || view.Status != 1 || view.IsComment() {
		c.LogFailure(err, "loading the View display", "machineName", machineName)
		return nil, false
	}
	return view, true
}

// renderEmbeddedView renders a View inside another page (profile tab, front
// page feed) with page links built from basePath/query; userID feeds the
// created_by contextual filter (0 - none). Returns the total number of
// results.
func (c *DynamicContentController) renderEmbeddedView(ctx context.Context, view entity.ViewDisplayDef, userID, page int, basePath string, query url.Values) (template.HTML, int, error) {
	effective, err := c.viewService.EffectiveSettings(ctx, view)
	if err != nil {
		return "", 0, fmt.Errorf("error resolving profile tab view settings: %w", err)
	}
	if effective.IsComment() {
		return c.renderProfileCommentTab(ctx, effective, userID, page, basePath, query)
	}
	nodes, total, err := c.viewService.RunViewForUser(ctx, view, userID, page)
	if err != nil {
		return "", 0, fmt.Errorf("error running profile tab view: %w", err)
	}
	var pager *pagination.Pagination
	if effective.ShowPagination {
		pager = pagination.New(page, total, effective.PageSize, basePath, query)
	}
	html, err := c.renderContentViewFragment(ctx, effective, nodes, pager)
	return html, total, err
}

// ProfileTabCount - how many results a user_tab display has for the
// profile of userID (for the tab label).
func (c *DynamicContentController) ProfileTabCount(ctx context.Context, view entity.ViewDisplayDef, userID int) (int, error) {
	if view.IsComment() {
		_, total, err := c.viewService.RunCommentViewForUser(ctx, view, userID, 1)
		return total, err
	}
	_, total, err := c.viewService.RunViewForUser(ctx, view, userID, 1)
	return total, err
}

// renderProfileCommentTab - RenderProfileTab for a comment View
// (dynamic/comment_list.html's "content", like the comment View page).
func (c *DynamicContentController) renderProfileCommentTab(ctx context.Context, view entity.ViewDisplayDef, userID, page int, basePath string, query url.Values) (template.HTML, int, error) {
	comments, total, err := c.viewService.RunCommentViewForUser(ctx, view, userID, page)
	if err != nil {
		return "", 0, fmt.Errorf("error running profile comment tab view: %w", err)
	}
	viewFields, err := c.viewService.EffectiveFieldsByView(ctx, view)
	if err != nil {
		return "", 0, fmt.Errorf("error loading profile comment tab view fields: %w", err)
	}
	pager := pagination.New(page, total, view.PageSize, basePath, query)
	if view.ThemeTemplate != "" {
		var themePager *pagination.Pagination
		if view.ShowPagination {
			themePager = pager
		}
		if html, ok, err := c.renderThemeCommentViewList(ctx, view.ThemeTemplate, comments, themePager); ok {
			return views.WithViewTexts(view, html), total, err
		}
	}
	data := views.CommentListData(ctx, view, viewFields, comments)
	data["showPagination"] = view.ShowPagination
	data["pagination"] = pager
	var buf bytes.Buffer
	withRequestContext(ctx, data)
	if err := c.templateService.RenderNamed(&buf, "content", data, "dynamic/comment_list.html", "pagination.html"); err != nil {
		return "", 0, fmt.Errorf("error rendering profile comment tab: %w", err)
	}
	return template.HTML(buf.String()), total, nil
}

// renderThemeViewList renders nodes as teaser items through the theme's
// views/<name>.html; ok is false when the theme has no such file (the caller
// falls back to the field grid).
func (c *DynamicContentController) renderThemeViewList(ctx context.Context, name string, nodes entity.Nodes, pager *pagination.Pagination) (template.HTML, bool, error) {
	if c.teasers == nil || !views.ThemeTemplateExists(ctx, name) {
		return "", false, nil
	}
	items, err := c.teasers.Build(ctx, nodes)
	if err != nil {
		return "", true, fmt.Errorf("error building teaser items for theme view template %q: %w", name, err)
	}
	return c.renderThemeView(ctx, name, withRequestContext(ctx, map[string]any{"items": items}), pager)
}

// renderThemeCommentViewList - renderThemeViewList for a comment View:
// the template gets the comments themselves (with their node and
// author) as "comments".
func (c *DynamicContentController) renderThemeCommentViewList(ctx context.Context, name string, comments entity.Comments, pager *pagination.Pagination) (template.HTML, bool, error) {
	if !views.ThemeTemplateExists(ctx, name) {
		return "", false, nil
	}
	return c.renderThemeView(ctx, name, withRequestContext(ctx, map[string]any{"comments": comments}), pager)
}

// renderThemeView - views.RenderThemeTemplate (ok is always true: the
// template exists).
func (c *DynamicContentController) renderThemeView(ctx context.Context, name string, data map[string]any, pager *pagination.Pagination) (template.HTML, bool, error) {
	html, err := views.RenderThemeTemplate(ctx, c.templateService, name, data, pager)
	return html, true, err
}

// renderContentViewFragment - a content View's list markup for nodes
// (dynamic/list.html's "content"), with page links when pager is set.
func (c *DynamicContentController) renderContentViewFragment(ctx context.Context, view entity.ViewDisplayDef, nodes entity.Nodes, pager *pagination.Pagination) (template.HTML, error) {
	if view.ThemeTemplate != "" {
		if html, ok, err := c.renderThemeViewList(ctx, view.ThemeTemplate, nodes, pager); ok {
			return views.WithViewTexts(view, html), err
		}
	}
	viewFields, err := c.viewService.EffectiveFieldsByView(ctx, view)
	if err != nil {
		return "", fmt.Errorf("error loading contextual content view fields for block: %w", err)
	}
	listFields, items, err := views.BuildContentList(ctx, c.contentListSources(), view.ID, viewFields, nodes, views.ContentListOptions{AltTitles: true})
	if err != nil {
		return "", fmt.Errorf("error building contextual content view block items: %w", err)
	}

	data := views.ContentListData(view, listFields, items, c.resolveCarouselLinkURLs(ctx, view.CarouselLinkFieldStorageIDValue(), nodes))
	data["showPagination"] = pager != nil
	data["isViewBlock"] = true
	data["viewID"] = view.ID
	if pager != nil {
		data["pagination"] = pager
	}

	var buf bytes.Buffer
	withRequestContext(ctx, data)
	if err := c.templateService.RenderNamed(&buf, "content", data, "dynamic/list.html", "pagination.html"); err != nil {
		return "", fmt.Errorf("error rendering contextual content view block: %w", err)
	}
	return template.HTML(buf.String()), nil
}
