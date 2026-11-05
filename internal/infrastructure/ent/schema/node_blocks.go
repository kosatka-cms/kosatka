package schema

import (
	"context"
	"strings"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	gen "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/comment"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/intercept"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
)

// A blocks field's items are nodes (parent_id set) but not content of
// their own: no page, list, search, count or feed has them. Every node
// query leaves them out - with no condition of its own to forget - save
// one run with WithBlocks (the code reading and saving the items, or
// handling all the nodes of a type).

// withBlocksKey - the context value of WithBlocks.
type withBlocksKey struct{}

// WithBlocks - ctx whose node queries keep the blocks fields' items.
func WithBlocks(ctx context.Context) context.Context {
	return context.WithValue(ctx, withBlocksKey{}, true)
}

// Interceptors of the Node: the items left out (WithBlocks - kept), and
// the content in the trash or ctx's visitor may not view (LiveNodes).
func (Node) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseNode(func(ctx context.Context, q *gen.NodeQuery) error {
			if keep, _ := ctx.Value(withBlocksKey{}).(bool); !keep {
				q.Where(node.ParentIDIsNil())
			}
			q.Where(LiveNodes(ctx)...)
			return nil
		}),
	}
}

// Interceptors of the Comment: the comments on the content in the trash
// or ctx's visitor may not view (LiveNodes) left out.
func (Comment) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseComment(func(ctx context.Context, q *gen.CommentQuery) error {
			if live := LiveNodes(ctx); len(live) > 0 {
				q.Where(comment.HasNodeWith(live...))
			}
			return nil
		}),
	}
}

// LiveNodes - the conditions of the content ctx's queries see: out of
// the trash (entity.TrashVisible - kept), and what its visitor may view
// (ViewableNodes). A node query gets them from its interceptor, a
// subquery of another entity's (HasNodeWith) must add them itself.
func LiveNodes(ctx context.Context) []predicate.Node {
	preds := ViewableNodes(ctx)
	if !entity.TrashVisible(ctx) {
		preds = append(preds, node.DeletedAtIsNil())
	}
	return preds
}

// ViewableNodes - the conditions of the content ctx's visitor may view
// (entity.VisitorOf): not of a type closed to them, and an item with no
// view roles of its own, or one of theirs, or their own, or of a type
// whose every item they view; none for a ctx that hides nothing. A node
// query gets them from its interceptor, a subquery of another entity's
// (HasNodeWith) must add them itself.
func ViewableNodes(ctx context.Context) []predicate.Node {
	v, _ := entity.VisitorOf(ctx)
	var preds []predicate.Node
	if len(v.HiddenTypes) > 0 {
		preds = append(preds, node.TypeNotIn(v.HiddenTypes...))
	}
	if v.RestrictsItems() {
		items := []predicate.Node{node.ViewRoles("")}
		for _, r := range v.Roles {
			items = append(items, hasViewRole(r))
		}
		if v.UserID > 0 {
			items = append(items, node.CreatedBy(v.UserID))
		}
		if len(v.ItemTypes) > 0 {
			items = append(items, node.TypeIn(v.ItemTypes...))
		}
		preds = append(preds, node.Or(items...))
	}
	return preds
}

// hasViewRole - the item's view roles (comma-separated, Node.ViewRoles)
// have role.
func hasViewRole(role string) predicate.Node {
	return func(s *sql.Selector) {
		pattern := "%," + likeEscaper.Replace(role) + ",%"
		s.Where(sql.P(func(b *sql.Builder) {
			b.WriteString("CONCAT(',', ").WriteString(s.C(node.FieldViewRoles)).WriteString(", ',') LIKE ").Arg(pattern)
		}))
	}
}

// likeEscaper - a LIKE pattern's literal text: its wildcards escaped.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
