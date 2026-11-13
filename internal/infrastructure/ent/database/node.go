package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/contenttypedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/nodecounter"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/pageseo"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/taxonomyindex"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/user"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/schema"
)

// NewNodeRepository - the node repository on client.
func NewNodeRepository(client *generated.Client) *NodeRepository {
	return &NodeRepository{
		client: client,
	}
}

// NodeRepository provides database access to nodes.
type NodeRepository struct {
	client *generated.Client
}

// URLsByIDs - see entity.NodesRepository.URLsByIDs.
func (r NodeRepository) URLsByIDs(ctx context.Context, ids []int) (map[int]string, error) {
	out := map[int]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := GetClient(ctx, r.client).Node.Query().Where(node.IDIn(ids...)).
		Select(node.FieldID, node.FieldType, node.FieldSlug, node.FieldLangcode).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the addresses of nodes %v: %w", ids, err)
	}
	for _, n := range rows {
		if n.Slug != "" {
			out[n.ID] = entity.Node{Type: entity.ContentType(n.Type), Slug: n.Slug, Langcode: n.Langcode}.URL()
		}
	}
	return out, nil
}

// ByID - the node with nodeID, with its author and view counter;
// ErrEntityNotFound if there is none.
func (r NodeRepository) ByID(ctx context.Context, nodeID int) (*entity.Node, error) {
	client := GetClient(ctx, r.client)
	res, err := client.Node.Query().
		Where(node.ID(nodeID)).
		Select(
			node.FieldID,
			node.FieldTitle,
			node.FieldSlug,
			node.FieldLangcode,
			node.FieldType,
			node.FieldStatus,
			node.FieldOnModeration,
			node.FieldCommentMode,
			node.FieldParentID,
			node.FieldViewRoles,
			node.FieldDeletedAt,
			node.FieldDeletedBy,
			node.FieldCreatedAt,
			node.FieldUpdatedAt,
		).
		WithUsers(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldEmail,
				user.FieldSlug,
				user.FieldCreatedAt,
			)
		}).
		WithNodeCounter(func(ncq *generated.NodeCounterQuery) {
			ncq.Select(
				nodecounter.FieldNid,
				nodecounter.FieldTotalcount,
				nodecounter.FieldDaycount,
				nodecounter.FieldLastViewed,
			)
		}).
		Only(ctx)

	if err != nil {
		return nil, notFoundAs(err, "node with ID %d", nodeID)
	}

	n := dao.ToNodeEntity(res)

	return &n, nil
}

// SearchByTitle is a lightweight node search for pickers: no eager-loaded
// edges, since callers need only id/title/type/slug. Matches nodes.title or
// any short text field value (where alternative titles live), of public
// nodes only (published, not on moderation).
func (r NodeRepository) SearchByTitle(ctx context.Context, query string, types []string, limit int) ([]entity.NodeSearchHit, error) {
	client := GetClient(ctx, r.client)
	shortTextMatch := func(s *sql.Selector) *sql.Predicate {
		t := sql.Table("field_value_text")
		return sql.Exists(sql.Select(t.C("id")).From(t).Where(sql.And(
			sql.ColumnsEQ(t.C("node_id"), s.C(node.FieldID)),
			sql.P(func(b *sql.Builder) { b.WriteString("CHAR_LENGTH(").WriteString(t.C("value")).WriteString(") <= 255") }),
			sql.ContainsFold(t.C("value"), query),
		)))
	}
	q := client.Node.Query().
		Where(
			node.StatusEQ(1),
			node.OnModeration(0),
			func(s *sql.Selector) {
				s.Where(sql.Or(sql.ContainsFold(s.C(node.FieldTitle), query), shortTextMatch(s)))
			},
		)
	if len(types) > 0 {
		q = q.Where(node.TypeIn(types...))
	}
	res, err := q.Select(node.FieldID, node.FieldTitle, node.FieldSlug, node.FieldType, node.FieldLangcode).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	hits := make([]entity.NodeSearchHit, len(res))
	var needMatch []int
	lowerQuery := strings.ToLower(query)
	for i, v := range res {
		hits[i] = entity.NodeSearchHit{Node: entity.Node{ID: v.ID, Type: entity.ContentType(v.Type), Title: v.Title, Slug: v.Slug, Langcode: v.Langcode}}
		if !strings.Contains(strings.ToLower(v.Title), lowerQuery) {
			needMatch = append(needMatch, v.ID)
		}
	}
	if len(needMatch) > 0 {
		matches, err := client.FieldValueText.Query().
			Where(
				fieldvaluetext.NodeIDIn(needMatch...),
				fieldvaluetext.ValueContainsFold(query),
			).
			Select(fieldvaluetext.FieldNodeID, fieldvaluetext.FieldValue).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("NodeRepository.SearchByTitle: %w", err)
		}
		byNode := make(map[int]string, len(matches))
		for _, m := range matches {
			if _, ok := byNode[m.NodeID]; !ok && len([]rune(m.Value)) <= 255 {
				byNode[m.NodeID] = strings.TrimSpace(m.Value)
			}
		}
		for i := range hits {
			if m, ok := byNode[hits[i].Node.ID]; ok {
				hits[i].Match = m
			}
		}
	}
	return hits, nil
}

// BySlug - the node with slug, with its author (and the author's role) and
// view counter; ErrEntityNotFound if there is none.
func (r NodeRepository) BySlug(ctx context.Context, slug string) (*entity.Node, error) {
	client := GetClient(ctx, r.client)
	res, err := client.Node.Query().
		Where(node.Slug(slug)).
		Select(
			node.FieldID,
			node.FieldTitle,
			node.FieldSlug,
			node.FieldLangcode,
			node.FieldType,
			node.FieldStatus,
			node.FieldOnModeration,
			node.FieldCommentMode,
			node.FieldParentID,
			node.FieldViewRoles,
			node.FieldDeletedAt,
			node.FieldDeletedBy,
			node.FieldCreatedAt,
			node.FieldUpdatedAt,
		).
		WithUsers(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldEmail,
				user.FieldSlug,
				user.FieldCreatedAt,
			).WithRole()
		}).
		WithNodeCounter(func(ncq *generated.NodeCounterQuery) {
			ncq.Select(
				nodecounter.FieldNid,
				nodecounter.FieldTotalcount,
				nodecounter.FieldDaycount,
				nodecounter.FieldLastViewed,
			)
		}).
		Only(ctx)

	if err != nil {
		return nil, notFoundAs(err, "node with slug %s", slug)
	}

	n := dao.ToNodeEntity(res)

	return &n, nil
}

// AnyWithViewRoles reports whether some node has view roles of its own
// (entity.Node.ViewRoles), whoever asks.
func (r NodeRepository) AnyWithViewRoles(ctx context.Context) (bool, error) {
	ok, err := GetClient(ctx, r.client).Node.Query().Where(node.ViewRolesNEQ("")).Exist(entity.WithAllNodeTypes(ctx))
	if err != nil {
		return false, fmt.Errorf("checking for nodes with view roles: %w", err)
	}
	return ok, nil
}

// SlugExists reports whether a node has the slug (of any type, whoever
// asks - entity.WithAllNodeTypes: slugs are unique across all nodes).
func (r NodeRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	ctx = entity.WithAllNodeTypes(ctx)
	return GetClient(ctx, r.client).Node.Query().Where(node.SlugEQ(slug)).Exist(ctx)
}

// SlugExistsExcludingNode reports whether another node than excludeNodeID
// has the slug (of any type, whoever asks).
func (r NodeRepository) SlugExistsExcludingNode(ctx context.Context, slug string, excludeNodeID int) (bool, error) {
	ctx = entity.WithAllNodeTypes(ctx)
	return GetClient(ctx, r.client).Node.Query().Where(node.SlugEQ(slug), node.IDNEQ(excludeNodeID)).Exist(ctx)
}

// UpsertPageSEO - see entity.NodesRepository.UpsertPageSEO.
func (r NodeRepository) UpsertPageSEO(ctx context.Context, nodeID int, metaTitle, metaDescription string) error {
	client := GetClient(ctx, r.client)
	metaTitle = strings.TrimSpace(metaTitle)
	metaDescription = strings.TrimSpace(metaDescription)
	if metaTitle == "" && metaDescription == "" {
		_, err := client.PageSeo.Delete().Where(pageseo.Nid(nodeID)).Exec(ctx)
		return err
	}

	existing, err := client.PageSeo.Query().Where(pageseo.Nid(nodeID)).Only(ctx)
	if generated.IsNotFound(err) {
		create := client.PageSeo.Create().SetNid(nodeID)
		if metaTitle != "" {
			create = create.SetTitle(metaTitle)
		}
		if metaDescription != "" {
			create = create.SetDescription(metaDescription)
		}
		_, err = create.Save(ctx)
		return err
	}
	if err != nil {
		return fmt.Errorf("NodeRepository.UpsertPageSEO: %w", err)
	}

	upd := client.PageSeo.UpdateOneID(existing.ID)
	if metaTitle != "" {
		upd = upd.SetTitle(metaTitle)
	} else {
		upd = upd.ClearTitle()
	}
	if metaDescription != "" {
		upd = upd.SetDescription(metaDescription)
	} else {
		upd = upd.ClearDescription()
	}
	_, err = upd.Save(ctx)
	return err
}

// CountByType counts all nodes of the given type (any status, the trash
// too), used to guard deleting or renaming a content type that still has
// nodes.
func (r NodeRepository) CountByType(ctx context.Context, nodeType string) (int, error) {
	client := GetClient(ctx, r.client)
	return client.Node.Query().Where(node.TypeEQ(nodeType)).Count(everyNode(ctx))
}

// everyNode - ctx whose node queries see every node: the blocks fields'
// items (a field set's nodes), the trash, the types the visitor may not
// view. For what must not leave a node behind (the guards and cleanups
// of a deletion).
func everyNode(ctx context.Context) context.Context {
	return schema.WithBlocks(entity.WithAllNodeTypes(ctx))
}

// PathBatch - see entity.NodesRepository.PathBatch.
func (r NodeRepository) PathBatch(ctx context.Context, nodeType string, afterID, limit int, onlyAuto bool) (entity.Nodes, error) {
	q := GetClient(ctx, r.client).Node.Query().Where(node.TypeEQ(nodeType), node.IDGT(afterID))
	if onlyAuto {
		q = q.Where(node.PathAuto(true))
	}
	res, err := q.Order(generated.Asc(node.FieldID)).Limit(limit).
		Select(node.FieldID, node.FieldType, node.FieldTitle, node.FieldSlug, node.FieldCreatedAt, node.FieldPathAuto, node.FieldLangcode).
		All(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make(entity.Nodes, 0, len(res))
	for _, n := range res {
		nodes = append(nodes, dao.ToNodeEntity(n))
	}
	return nodes, nil
}

// CountPathNodes - how many nodes of nodeType PathBatch goes through
// (onlyAuto - only those following their type's pattern).
func (r NodeRepository) CountPathNodes(ctx context.Context, nodeType string, onlyAuto bool) (int, error) {
	q := GetClient(ctx, r.client).Node.Query().Where(node.TypeEQ(nodeType))
	if onlyAuto {
		q = q.Where(node.PathAuto(true))
	}
	return q.Count(ctx)
}

// SetPath - see entity.NodesRepository.SetPath.
func (r NodeRepository) SetPath(ctx context.Context, nodeID int, slug string, auto bool) error {
	n, err := GetClient(ctx, r.client).Node.Query().Where(node.ID(nodeID)).Select(node.FieldUpdatedAt).Only(ctx)
	if err != nil {
		return fmt.Errorf("NodeRepository.SetPath: %w", err)
	}
	// updated_at has an update default (now): set to itself, it stays.
	return GetClient(ctx, r.client).Node.UpdateOneID(nodeID).SetSlug(slug).SetPathAuto(auto).SetUpdatedAt(n.UpdatedAt).Exec(ctx)
}

// UpdateStatus publishes (1) or unpublishes (0) the node; publishing
// clears its scheduled publishing, unpublishing its scheduled unpublishing
// (done, either way).
func (r NodeRepository) UpdateStatus(ctx context.Context, nodeID int, status int) error {
	client := GetClient(ctx, r.client)
	u := client.Node.UpdateOneID(nodeID).
		SetStatus(status).
		SetUpdatedAt(time.Now())
	if status == 1 {
		u = u.ClearPublishOn()
	} else {
		u = u.ClearUnpublishOn()
	}
	_, err := u.Save(ctx)

	return err
}

// ClearSchedule clears the node's scheduled publishing and unpublishing.
func (r NodeRepository) ClearSchedule(ctx context.Context, nodeID int) error {
	if err := GetClient(ctx, r.client).Node.UpdateOneID(nodeID).ClearPublishOn().ClearUnpublishOn().Exec(ctx); err != nil {
		return fmt.Errorf("clearing the schedule of node %d: %w", nodeID, err)
	}
	return nil
}

// TrashNode - see entity.NodeTrashRepository.TrashNode.
func (r NodeRepository) TrashNode(ctx context.Context, nodeID, by int, at time.Time) error {
	u := GetClient(ctx, r.client).Node.UpdateOneID(nodeID).SetDeletedAt(at)
	if by > 0 {
		u = u.SetDeletedBy(by)
	}
	if err := u.Exec(ctx); err != nil {
		return fmt.Errorf("putting node %d in the trash: %w", nodeID, err)
	}
	return nil
}

// RestoreNode - see entity.NodeTrashRepository.RestoreNode.
func (r NodeRepository) RestoreNode(ctx context.Context, nodeID int) error {
	if err := GetClient(ctx, r.client).Node.UpdateOneID(nodeID).ClearDeletedAt().ClearDeletedBy().Exec(ctx); err != nil {
		return fmt.Errorf("restoring node %d from the trash: %w", nodeID, err)
	}
	return nil
}

// TrashedNodes - see entity.NodeTrashRepository.TrashedNodes.
func (r NodeRepository) TrashedNodes(ctx context.Context, limit, offset int) (entity.Nodes, int, error) {
	ctx = entity.WithTrashed(ctx)
	q := GetClient(ctx, r.client).Node.Query().Where(node.DeletedAtNotNil())
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting the trash: %w", err)
	}
	rows, err := q.Order(generated.Desc(node.FieldDeletedAt), generated.Desc(node.FieldID)).
		WithUsers().Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing the trash: %w", err)
	}
	out := make(entity.Nodes, len(rows))
	for i, row := range rows {
		out[i] = dao.ToNodeEntity(row)
	}
	return out, total, nil
}

// TrashedBefore - see entity.NodeTrashRepository.TrashedBefore.
func (r NodeRepository) TrashedBefore(ctx context.Context, before time.Time) ([]int, error) {
	ctx = entity.WithTrashed(ctx)
	ids, err := GetClient(ctx, r.client).Node.Query().Where(node.DeletedAtLT(before)).IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("the trash before %s: %w", before.Format(time.RFC3339), err)
	}
	return ids, nil
}

// SetPublishing - see entity.NodesRepository.SetPublishing: updated_at
// set to what it was (the schema's update default would make it now).
func (r NodeRepository) SetPublishing(ctx context.Context, nodeID, status, onModeration int) error {
	client := GetClient(ctx, r.client)
	n, err := client.Node.Query().Where(node.ID(nodeID)).Select(node.FieldUpdatedAt).Only(ctx)
	if err != nil {
		return notFoundAs(err, "node with id %d", nodeID)
	}
	if err := client.Node.UpdateOneID(nodeID).SetStatus(status).SetOnModeration(onModeration).SetUpdatedAt(n.UpdatedAt).Exec(ctx); err != nil {
		return fmt.Errorf("setting the publishing of node %d: %w", nodeID, err)
	}
	return nil
}

// UpdateOnModeration sets whether the node waits on moderation (1) or not
// (0).
func (r NodeRepository) UpdateOnModeration(ctx context.Context, nodeID int, onModeration int) error {
	client := GetClient(ctx, r.client)
	_, err := client.Node.UpdateOneID(nodeID).
		SetOnModeration(onModeration).
		SetUpdatedAt(time.Now()).
		Save(ctx)

	return err
}

// ListForAdmin returns nodes for the admin list with filters and pagination.
func (r NodeRepository) ListForAdmin(ctx context.Context, filter entity.AdminNodesFilter) (*entity.Nodes, int, error) {
	client := GetClient(ctx, r.client)

	query := client.Node.Query().
		WithUsers(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldSlug,
			)
		}).
		WithNodeCounter(func(ncq *generated.NodeCounterQuery) {
			ncq.Select(
				nodecounter.FieldNid,
				nodecounter.FieldTotalcount,
				nodecounter.FieldDaycount,
				nodecounter.FieldLastViewed,
			)
		})

	switch filter.SortBy {
	case entity.AdminNodesSortTitle:
		if filter.Desc {
			query = query.Order(generated.Desc(node.FieldTitle), generated.Desc(node.FieldID))
		} else {
			query = query.Order(generated.Asc(node.FieldTitle), generated.Asc(node.FieldID))
		}
	case entity.AdminNodesSortStatus:
		if filter.Desc {
			query = query.Order(generated.Desc(node.FieldStatus), generated.Desc(node.FieldID))
		} else {
			query = query.Order(generated.Asc(node.FieldStatus), generated.Asc(node.FieldID))
		}
	case entity.AdminNodesSortModeration:
		if filter.Desc {
			query = query.Order(generated.Desc(node.FieldOnModeration), generated.Desc(node.FieldID))
		} else {
			query = query.Order(generated.Asc(node.FieldOnModeration), generated.Asc(node.FieldID))
		}
	case entity.AdminNodesSortCreated:
		if filter.Desc {
			query = query.Order(generated.Desc(node.FieldCreatedAt), generated.Desc(node.FieldID))
		} else {
			query = query.Order(generated.Asc(node.FieldCreatedAt), generated.Asc(node.FieldID))
		}
	case entity.AdminNodesSortUpdated:
		if filter.Desc {
			query = query.Order(generated.Desc(node.FieldUpdatedAt), generated.Desc(node.FieldID))
		} else {
			query = query.Order(generated.Asc(node.FieldUpdatedAt), generated.Asc(node.FieldID))
		}
	case entity.AdminNodesSortPath:
		if filter.Desc {
			query = query.Order(generated.Desc(node.FieldSlug), generated.Desc(node.FieldID))
		} else {
			query = query.Order(generated.Asc(node.FieldSlug), generated.Asc(node.FieldID))
		}
	case entity.AdminNodesSortViews:
		if filter.Desc {
			query = query.Order(
				OrderByCounter(nodecounter.FieldTotalcount, true),
				generated.Desc(node.FieldID),
			)
		} else {
			query = query.Order(
				OrderByCounter(nodecounter.FieldTotalcount, false),
				generated.Asc(node.FieldID),
			)
		}
	default:
		query = query.Order(generated.Desc(node.FieldUpdatedAt), generated.Desc(node.FieldID))
	}

	if filter.Scheduled {
		query = query.Where(node.Or(node.PublishOnNotNil(), node.UnpublishOnNotNil()))
	}
	if filter.Status != nil {
		query = query.Where(node.Status(*filter.Status))
	}
	if filter.OnModeration != nil {
		query = query.Where(node.OnModeration(*filter.OnModeration))
	}
	if filter.Type != nil && *filter.Type != "" {
		query = query.Where(node.TypeEQ(*filter.Type))
	}
	if filter.Search != "" {
		query = query.Where(node.Or(node.TitleContainsFold(filter.Search), node.SlugContains(filter.Search)))
	}
	if filter.Path != "" {
		query = query.Where(node.SlugContains(filter.Path))
	}

	// Count with the same filters.
	totalCount, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("NodeRepository.ListForAdmin: %w", err)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	res, err := query.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("NodeRepository.ListForAdmin: %w", err)
	}

	nodes := make(entity.Nodes, 0, len(res))
	for _, n := range res {
		nodes = append(nodes, dao.ToNodeEntity(n))
	}

	return &nodes, totalCount, nil
}

// knownNodeTypes - nodes of a type the binary serves: a site content
// type's or a registered module's. A module installed in the database but
// missing from the binary leaves nodes of a type nothing serves (their
// pages answer 404): the lists of public content leave them out.
func knownNodeTypes(ctx context.Context, client *generated.Client) (predicate.Node, error) {
	types, err := client.ContentTypeDef.Query().Select(contenttypedef.FieldMachineName).Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the content types: %w", err)
	}
	return node.TypeIn(append(types, entity.ModuleNodeTypes()...)...), nil
}

// sitemapNodePredicates selects published, public nodes with a page of
// their own (knownNodeTypes): a type whose page redirects elsewhere
// (ContentTypeDef.RedirectField, e.g. carousel slides) is excluded.
func sitemapNodePredicates(ctx context.Context, client *generated.Client, excludeTypes []string) ([]predicate.Node, error) {
	known, err := knownNodeTypes(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("sitemapNodePredicates: %w", err)
	}
	preds := []predicate.Node{node.StatusEQ(1), node.OnModerationEQ(0), node.SlugNEQ(""), known}
	if len(excludeTypes) > 0 {
		preds = append(preds, node.TypeNotIn(excludeTypes...))
	}
	redirecting, err := client.ContentTypeDef.Query().
		Where(contenttypedef.RedirectFieldNEQ("")).
		Select(contenttypedef.FieldMachineName).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("sitemapNodePredicates: %w", err)
	}
	if len(redirecting) > 0 {
		preds = append(preds, node.TypeNotIn(redirecting...))
	}
	return preds, nil
}

// CountPublicSitemapNodes - see
// entity.NodesRepository.CountPublicSitemapNodes.
func (r NodeRepository) CountPublicSitemapNodes(ctx context.Context, excludeTypes []string) (int, error) {
	client := GetClient(ctx, r.client)
	preds, err := sitemapNodePredicates(ctx, client, excludeTypes)
	if err != nil {
		return 0, err
	}
	return client.Node.Query().Where(preds...).Count(ctx)
}

// ListForFeed - see entity.NodesRepository.ListForFeed.
func (r NodeRepository) ListForFeed(ctx context.Context, filter entity.FeedNodeFilter) (entity.Nodes, error) {
	if len(filter.Types) == 0 || filter.Limit <= 0 {
		return nil, nil
	}
	preds := []predicate.Node{
		node.TypeIn(filter.Types...),
		node.Status(1),
		node.OnModeration(0),
	}
	if !filter.Since.IsZero() {
		preds = append(preds, node.CreatedAtGTE(filter.Since))
	}
	if len(filter.TermIDs) > 0 {
		preds = append(preds, node.HasTaxonomyIndicesWith(taxonomyindex.TidIn(filter.TermIDs...)))
	}
	rows, err := GetClient(ctx, r.client).Node.Query().
		Where(preds...).
		Order(generated.Desc(node.FieldCreatedAt), generated.Desc(node.FieldID)).
		Limit(filter.Limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("NodeRepository.ListForFeed: %w", err)
	}
	out := make(entity.Nodes, 0, len(rows))
	for _, n := range rows {
		out = append(out, dao.ToNodeEntity(n))
	}
	return out, nil
}

// ListPublicSitemapNodes - see
// entity.NodesRepository.ListPublicSitemapNodes.
func (r NodeRepository) ListPublicSitemapNodes(ctx context.Context, excludeTypes []string, limit, offset int) ([]entity.SitemapNodeRow, error) {
	client := GetClient(ctx, r.client)
	preds, err := sitemapNodePredicates(ctx, client, excludeTypes)
	if err != nil {
		return nil, err
	}
	rows, err := client.Node.Query().
		Where(preds...).
		Select(node.FieldID, node.FieldSlug, node.FieldType, node.FieldLangcode, node.FieldUpdatedAt).
		Order(generated.Desc(node.FieldUpdatedAt), generated.Desc(node.FieldID)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("NodeRepository.ListPublicSitemapNodes: %w", err)
	}
	out := make([]entity.SitemapNodeRow, 0, len(rows))
	for _, n := range rows {
		out = append(out, entity.SitemapNodeRow{
			ID:        n.ID,
			Slug:      n.Slug,
			Type:      n.Type,
			Langcode:  n.Langcode,
			UpdatedAt: n.UpdatedAt,
		})
	}
	return out, nil
}
