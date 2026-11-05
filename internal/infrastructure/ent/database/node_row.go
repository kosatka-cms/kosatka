package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/pageseo"
)

// A node's own row, for a module's content type with pages of its own
// (polls, forms): the module stores its rows around it, in the context's
// transaction (GetClient), without the core's ent client.

// CreateRow stores a new node's row: its type, title, slug, status,
// moderation, comment mode, author and dates (created now unless set);
// its title's words go to the search index.
func (r NodeRepository) CreateRow(ctx context.Context, n entity.Node) (int, error) {
	now := time.Now()
	created := n.CreatedAt
	if created.IsZero() {
		created = now
	}
	client := GetClient(ctx, r.client)
	row, err := client.Node.Create().
		SetType(string(n.Type)).
		SetTitle(n.Title).
		SetSlug(n.Slug).
		SetStatus(n.Status).
		SetOnModeration(n.OnModeration).
		SetCommentMode(string(n.CommentMode.OrDefault())).
		SetCreatedBy(n.CreatedBy.ID).
		SetCreatedAt(created).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("creating the %s node: %w", n.Type, err)
	}
	return row.ID, indexTitle(ctx, client, row.ID)
}

// UpdateRow stores a node's row (CreateRow's columns), keeping its author
// and creation time when n doesn't set them; its title's words go to the
// search index.
func (r NodeRepository) UpdateRow(ctx context.Context, n entity.Node) error {
	client := GetClient(ctx, r.client)
	u := client.Node.UpdateOneID(n.ID).
		SetTitle(n.Title).
		SetSlug(n.Slug).
		SetStatus(n.Status).
		SetOnModeration(n.OnModeration).
		SetCommentMode(string(n.CommentMode.OrDefault())).
		SetUpdatedAt(time.Now())
	if n.CreatedBy.ID > 0 {
		u = u.SetCreatedBy(n.CreatedBy.ID)
	}
	if !n.CreatedAt.IsZero() {
		u = u.SetCreatedAt(n.CreatedAt)
	}
	if err := u.Exec(ctx); err != nil {
		return fmt.Errorf("updating node %d: %w", n.ID, err)
	}
	return indexTitle(ctx, client, n.ID)
}

// SetLangcode - see entity.NodesRepository.SetLangcode.
func (r NodeRepository) SetLangcode(ctx context.Context, nodeID int, code string) error {
	client := GetClient(ctx, r.client)
	if err := client.Node.UpdateOneID(nodeID).SetLangcode(code).Exec(indexAll(ctx)); err != nil {
		return fmt.Errorf("setting the language of node %d: %w", nodeID, err)
	}
	return indexNode(ctx, client, nodeID)
}

// PageSEO - the node's own title and description (page_seo), empty
// without them.
func (r NodeRepository) PageSEO(ctx context.Context, nodeID int) (entity.PageSEO, error) {
	row, err := GetClient(ctx, r.client).PageSeo.Query().Where(pageseo.Nid(nodeID)).Only(ctx)
	if generated.IsNotFound(err) {
		return entity.PageSEO{}, nil
	}
	if err != nil {
		return entity.PageSEO{}, fmt.Errorf("the page SEO of node %d: %w", nodeID, err)
	}
	return dao.ToPageSeoEntity(row), nil
}

// ByIDs - the nodes among ids that exist, by ID: their own columns, no
// edges.
func (r NodeRepository) ByIDs(ctx context.Context, ids []int) (map[int]entity.Node, error) {
	out := make(map[int]entity.Node, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := GetClient(ctx, r.client).Node.Query().
		Where(node.IDIn(ids...)).
		Select(node.FieldID, node.FieldType, node.FieldTitle, node.FieldSlug, node.FieldStatus, node.FieldOnModeration, node.FieldParentID, node.FieldLangcode, node.FieldCreatedBy).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodes by id: %w", err)
	}
	for _, row := range rows {
		n := entity.Node{ID: row.ID, Type: entity.ContentType(row.Type), Title: row.Title, Slug: row.Slug, Status: row.Status, OnModeration: row.OnModeration, Langcode: row.Langcode,
			CreatedBy: entity.User{ID: row.CreatedBy}}
		if row.ParentID != nil {
			n.ParentID = *row.ParentID
		}
		out[row.ID] = n
	}
	return out, nil
}

// ByType - the nodes of a type (ID and title), the newest first.
func (r NodeRepository) ByType(ctx context.Context, nodeType string) (entity.Nodes, error) {
	rows, err := GetClient(ctx, r.client).Node.Query().
		Where(node.Type(nodeType)).
		Order(generated.Desc(node.FieldCreatedAt), generated.Desc(node.FieldID)).
		Select(node.FieldID, node.FieldTitle).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the %s nodes: %w", nodeType, err)
	}
	out := make(entity.Nodes, 0, len(rows))
	for _, row := range rows {
		out = append(out, entity.Node{ID: row.ID, Title: row.Title, Type: entity.ContentType(nodeType)})
	}
	return out, nil
}
