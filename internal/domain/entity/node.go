package entity

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Nodes - a list of nodes.
type Nodes []Node

// Counter - a node's views (node_counter): all time, on the site's day of
// the last view, and the last view's time.
type Counter struct {
	NID        int
	TotalCount int
	DayCount   int
	LastViewed time.Time
}

// Node - a content item (a nodes row): its type, title, address (Slug),
// status (1 - published), author and dates; its content type's fields
// are DynamicContent's.
type Node struct {
	ID           int
	Type         ContentType
	Title        string
	Slug         string
	PageSEO      PageSEO
	Status       int
	OnModeration int
	Counter      Counter
	CreatedBy    User
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// CommentMode - this node's comment visibility. Copied from the type's
	// ContentTypeDef.DefaultCommentMode at creation, then edited per node.
	// The DB default is CommentModeOpen.
	CommentMode CommentMode
	// PathAuto - the slug follows the type's path pattern
	// (ContentTypeDef.PathPattern) on every save; off - set by hand.
	PathAuto bool
	// Promote - "on the front page", Sticky - "at the top of lists":
	// Views filter (ViewFilterBuiltinPromote/Sticky) and sort
	// (ViewSortBuiltinSticky) by them. A new node takes its type's
	// DefaultPromote/DefaultSticky; who may administer content sets them.
	Promote bool
	Sticky  bool
	// PublishOn / UnpublishOn - when the scheduled publishing publishes
	// or unpublishes the node (ScheduledPublishing); zero - not scheduled.
	PublishOn   time.Time
	UnpublishOn time.Time
	// ParentID - an item of a blocks field (FieldTypeBlocks): the node
	// holding it; 0 - content of its own.
	ParentID int
	// ViewRoles - the roles who may view the item, of those its type is
	// for (ContentTypeDef.ViewRoles); none - whoever may view its type.
	// Its author, who may edit it and who may view every content see it
	// anyway (AuthService.CanViewNode).
	ViewRoles []string
	// DeletedAt / DeletedBy - in the trash since then, put there by that
	// user (0 - unknown); zero - not in the trash. A node in the trash is
	// left out of every query (WithTrashed - kept) and shown to nobody.
	DeletedAt time.Time
	DeletedBy int
	// Langcode - the content's language (i18n code); its addresses carry
	// the language's prefix (LanguagePrefix), the default one's none.
	Langcode string
}

// InTrash - whether the node is in the trash (DeletedAt).
func (n Node) InTrash() bool { return !n.DeletedAt.IsZero() }

// ViewableBy - whether a visitor with roles (UserRoleNames) is one of
// those the item is for (ViewRoles), whatever its type.
func (n Node) ViewableBy(roles []string) bool {
	return ContentTypeDef{ViewRoles: n.ViewRoles}.ViewableBy(roles)
}

// IsPublic - published and not on moderation: everyone sees it (an
// unpublished node only who may view unpublished content, one on
// moderation its author too).
func (n Node) IsPublic() bool {
	return n.Status == 1 && n.OnModeration == 0
}

// ContentType - a content type's machine name (Node.Type).
type ContentType string

// URL - the node's public path: "/" + NodeRelPath, after its language's
// prefix (LocalPath: /en/… - none for the site's default language).
// Templates and code link to a node only through it, never "/" + Slug.
func (n Node) URL() string {
	return LocalPath(n.Langcode, "/"+NodeRelPath(n.Type, n.Slug))
}

// NodeRelPath returns a node's public URL path relative to the site root
// (no leading "/"), path-escaped. Slug is usually the full relative path
// ("articles/foo"); a module's node type with a PathPrefix (a forum
// topic's bare slug under "forum/") gets the prefix unless the slug
// already carries it (imported topics). Build node links through this
// function - using Slug alone breaks such links.
func NodeRelPath(nodeType ContentType, slug string) string {
	slug = strings.Trim(strings.TrimSpace(slug), "/")
	if slug == "" {
		return ""
	}
	rel := slug
	if prefix := ModuleNodeTypePathPrefix(string(nodeType)); prefix != "" && !strings.HasPrefix(slug, prefix) {
		rel = prefix + slug
	}
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, url.PathEscape(p))
	}
	return strings.Join(out, "/")
}

// CreatableMaterialLink is a menu entry for "create material" UI (JSON in USER_DATA).
type CreatableMaterialLink struct {
	URL   string `json:"url"`
	Label string `json:"label"`
}

// ToString - the machine name as a string.
func (c ContentType) ToString() string {
	return string(c)
}

// SitemapNodeRow holds minimal node fields for sitemap URL generation.
type SitemapNodeRow struct {
	ID        int
	Slug      string
	Type      string
	Langcode  string
	UpdatedAt time.Time
}

// Admin content list sort keys (query param sort=)
const (
	AdminNodesSortTitle      = "title"
	AdminNodesSortStatus     = "status"
	AdminNodesSortModeration = "moderation"
	AdminNodesSortCreated    = "created"
	AdminNodesSortUpdated    = "updated"
	AdminNodesSortViews      = "views"
	// AdminNodesSortPath - by the address (the URL aliases page).
	AdminNodesSortPath = "path"
)

// AdminNodesFilter defines filters for admin content list
type AdminNodesFilter struct {
	Status       *int // 0=unpublished, 1=published
	OnModeration *int // 0=not on moderation, 1=on moderation
	// Scheduled - only the nodes with a scheduled publishing or
	// unpublishing (PublishOn, UnpublishOn).
	Scheduled bool
	Type      *string
	// Path - a part of the address (nodes.slug); empty - any.
	Path string
	// Search - a part of the title (any case) or of the address; empty -
	// any.
	Search string
	SortBy string // AdminNodesSort*; empty = updated_at desc (default)
	Desc   bool   // ignored when SortBy is empty
	Limit  int
	Offset int
}

// NodeSearchHit - one NodesRepository.SearchByTitle result.
type NodeSearchHit struct {
	Node  Node
	Match string
}

// NodesRepository - storage port for the nodes.
type NodesRepository interface {
	ByID(ctx context.Context, nodeID int) (*Node, error)
	// URLsByIDs - the public addresses (Node.URL) of the nodes among ids
	// that exist, in one query (a menu's links).
	URLsByIDs(ctx context.Context, ids []int) (map[int]string, error)
	BySlug(ctx context.Context, slug string) (*Node, error)
	// SearchByTitle - generic node search for pickers (menu link
	// target, reference fields' edit widget): published nodes whose
	// nodes.title - or any short (≤ 255 chars) text field value, e.g. a
	// localized title field - contains query; types (machine names)
	// narrows the result, empty = every type. Match is the short field
	// value that matched when the title itself didn't.
	SearchByTitle(ctx context.Context, query string, types []string, limit int) ([]NodeSearchHit, error)
	ListForAdmin(ctx context.Context, filter AdminNodesFilter) (*Nodes, int, error)
	// SlugExists/SlugExistsExcludingNode - whether a node has the slug.
	// Slugs share one URL space (nodes.slug is unique across all types).
	SlugExists(ctx context.Context, slug string) (bool, error)
	SlugExistsExcludingNode(ctx context.Context, slug string, excludeNodeID int) (bool, error)
	// ProfileMaterialDisplayTitle - the title for lists: the type's alternative
	// title field (ContentTypeDef.AltTitleField) if set, otherwise n.Title.
	ProfileMaterialDisplayTitle(ctx context.Context, n Node) string
	// AltTitles - ProfileMaterialDisplayTitle's alternative titles for a
	// list of nodes, by node ID (those without one left out).
	AltTitles(ctx context.Context, nodes Nodes) (map[int]string, error)
	UpdateStatus(ctx context.Context, nodeID int, status int) error
	UpdateOnModeration(ctx context.Context, nodeID int, onModeration int) error
	// SetPublishing sets the node's publishing (status) and moderation as
	// a change of the site's, not an edit: its last save (UpdatedAt) and
	// its schedule stay - a type joining an editorial workflow, say.
	SetPublishing(ctx context.Context, nodeID, status, onModeration int) error
	// SetLangcode sets the node's language (Node.Langcode); its words in
	// the search index are stemmed in it again.
	SetLangcode(ctx context.Context, nodeID int, code string) error
	// UpsertPageSEO creates or updates page_seo for the node. Empty metaTitle and metaDescription removes the row (fallback to PageSEO() defaults).
	UpsertPageSEO(ctx context.Context, nodeID int, metaTitle, metaDescription string) error
	// PageSEO - the node's own title and description (page_seo), empty
	// without them.
	PageSEO(ctx context.Context, nodeID int) (PageSEO, error)
	// CreateRow and UpdateRow - a node's own row: its type, title, slug,
	// status, moderation, comment mode, author and dates (UpdateRow keeps
	// the author and the creation time it isn't given) - for a module's
	// content type with pages of its own, whose rows go with it (in the
	// context's transaction). CreateRow returns the new node's ID.
	CreateRow(ctx context.Context, n Node) (int, error)
	UpdateRow(ctx context.Context, n Node) error
	// ByType - the nodes of a type (ID and title), the newest first.
	ByType(ctx context.Context, nodeType string) (Nodes, error)
	// ByIDs - the nodes among ids that exist, by ID: their own columns
	// (ID, type, title, slug, status, the author's ID in CreatedBy.ID), no
	// edges - in one query (a module's rows that reference nodes).
	ByIDs(ctx context.Context, ids []int) (map[int]Node, error)

	// CountPublicSitemapNodes counts published nodes visible to anonymous users (status=1, on_moderation=0, non-empty slug), except types whose page redirects (ContentTypeDef.RedirectField) and excludeTypes.
	CountPublicSitemapNodes(ctx context.Context, excludeTypes []string) (int, error)
	// ListPublicSitemapNodes returns rows ordered by updated_at desc, id desc.
	ListPublicSitemapNodes(ctx context.Context, excludeTypes []string, limit, offset int) ([]SitemapNodeRow, error)
	// ListForFeed - the materials a feed takes (see FeedNodeFilter).
	ListForFeed(ctx context.Context, filter FeedNodeFilter) (Nodes, error)

	// CountByType counts all nodes of the given type (any status); guards
	// deleting/renaming a content type that still has nodes.
	CountByType(ctx context.Context, nodeType string) (int, error)
	// PathBatch - up to limit nodes of nodeType after afterID (by ID),
	// for rebuilding their addresses: ID, type, title, slug, created,
	// PathAuto; onlyAuto - those following their type's pattern.
	// CountPathNodes counts them.
	PathBatch(ctx context.Context, nodeType string, afterID, limit int, onlyAuto bool) (Nodes, error)
	CountPathNodes(ctx context.Context, nodeType string, onlyAuto bool) (int, error)
	// SetPath - a node's slug and PathAuto; its updated time stays (the
	// content didn't change).
	SetPath(ctx context.Context, nodeID int, slug string, auto bool) error
}

// ReservedContentTypeMachineNames - machine names a content type can't
// take: the built-in modules' node types (ModuleNodeTypeLabels), and
// "node" and "default", the suffixes of the templates for every type
// (page_node, teaser_default…).
func ReservedContentTypeMachineNames() []string {
	labels := ModuleNodeTypeLabels()
	names := make([]string, 0, len(labels)+2)
	names = append(names, "node", "default")
	for name := range labels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
