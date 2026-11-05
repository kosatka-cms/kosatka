package dao

import (
	"context"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// deletedUserPlaceholder stands in for an author whose account no longer
// exists (e.g. removed as spam), so templates rendering .Author.Slug /
// .Author.Username on orphaned forum topics and comments don't panic.
func deletedUserPlaceholder() *entity.User {
	return &entity.User{Username: i18n.T(context.Background(), "Deleted user")}
}

// ToCommentsEntity converts generated comments to entity.Comments.
func ToCommentsEntity(cs []*generated.Comment) entity.Comments {
	comments := make([]entity.Comment, len(cs))
	for i, c := range cs {
		comments[i] = *ToCommentEntity(c)
	}

	return comments
}

// ToCommentEntity converts a generated Comment (nil - nil) to its domain
// entity, with the loaded node, author (a placeholder for a deleted
// account), parent and replies, and its depth from the path.
func ToCommentEntity(c *generated.Comment) *entity.Comment {
	if c == nil {
		return nil
	}

	comment := &entity.Comment{
		ID:         c.ID,
		Nid:        c.Nid,
		CreatedBy:  c.CreatedBy,
		Content:    c.Content,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
		Status:     c.Status,
		HoldReason: c.HoldReason,
	}

	if c.ParentID != nil {
		comment.ParentID = c.ParentID
	}

	// Depth from the materialized path: segments - 1.
	if c.Path != nil && *c.Path != "" {
		comment.Depth = strings.Count(*c.Path, "/") - 1
		if comment.Depth < 0 {
			comment.Depth = 0
		}
	}

	if c.Subject != "" {
		comment.Subject = &c.Subject
	}

	if c.Edges.Node != nil {
		node := ToNodeEntity(c.Edges.Node)
		comment.Node = &node
	}

	if c.Edges.Author != nil {
		author := ToUserEntity(c.Edges.Author)
		comment.Author = &author
	} else {
		comment.Author = deletedUserPlaceholder()
	}

	if c.Edges.Parent != nil {
		comment.Parent = ToCommentEntity(c.Edges.Parent)
	}

	if len(c.Edges.Children) > 0 {
		comment.Children = ToCommentsEntity(c.Edges.Children)
	}

	return comment
}

// DeletedUserPlaceholder - the author shown for a deleted account (a
// module's mapping).
func DeletedUserPlaceholder() *entity.User { return deletedUserPlaceholder() }
