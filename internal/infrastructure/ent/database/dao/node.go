package dao

import (
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToNodeEntity converts a generated Node to its domain entity, with the
// author, counter and page SEO when loaded.
func ToNodeEntity(n *generated.Node) entity.Node {
	node := entity.Node{
		ID:           n.ID,
		Type:         entity.ContentType(n.Type),
		Title:        n.Title,
		Slug:         n.Slug,
		Status:       n.Status,
		OnModeration: n.OnModeration,
		CommentMode:  entity.CommentMode(n.CommentMode),
		PathAuto:     n.PathAuto,
		Promote:      n.Promote,
		Sticky:       n.Sticky,
		ViewRoles:    splitViewRoles(n.ViewRoles),
		CreatedAt:    n.CreatedAt,
		UpdatedAt:    n.UpdatedAt,
	}

	if n.ParentID != nil {
		node.ParentID = *n.ParentID
	}
	if n.PublishOn != nil {
		node.PublishOn = *n.PublishOn
	}
	if n.UnpublishOn != nil {
		node.UnpublishOn = *n.UnpublishOn
	}
	if n.DeletedAt != nil {
		node.DeletedAt = *n.DeletedAt
	}
	node.Langcode = n.Langcode
	if n.DeletedBy != nil {
		node.DeletedBy = *n.DeletedBy
	}

	if n.Edges.Users != nil {
		node.CreatedBy = ToUserEntity(n.Edges.Users)
	}

	if n.Edges.NodeCounter != nil {
		node.Counter = ToCounterEntity(n.Edges.NodeCounter)
	}

	if n.Edges.PageSeo != nil {
		node.PageSEO = ToPageSeoEntity(n.Edges.PageSeo)
	}

	return node
}

// ToCounterEntity converts a generated NodeCounter to entity.Counter.
func ToCounterEntity(c *generated.NodeCounter) entity.Counter {
	return entity.Counter{
		NID:        c.Nid,
		TotalCount: c.Totalcount,
		DayCount:   c.Daycount,
		LastViewed: c.LastViewed,
	}
}
