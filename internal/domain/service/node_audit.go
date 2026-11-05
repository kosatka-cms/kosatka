package service

import (
	"context"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// nodeObject - the node n as an action log's object; withURL - it still
// exists.
func nodeObject(n entity.Node, withURL bool) audit.Object {
	o := audit.Object{Kind: audit.KindNode, ID: strconv.Itoa(n.ID), Label: n.Title, Subtype: string(n.Type)}
	if withURL {
		o.URL = "/node/" + strconv.Itoa(n.ID)
	}
	return o
}

// AuditNodeObject - the node n as an action log's object (a module's own
// action on it shows in its log too).
func AuditNodeObject(n entity.Node) audit.Object { return nodeObject(n, true) }

// AuditNodeCreated records for the action log that the node n was created.
func AuditNodeCreated(ctx context.Context, n entity.Node) {
	audit.Record(ctx, audit.Action{Type: audit.NodeCreate, Object: nodeObject(n, true)})
}

// AuditNodeChange records what a save changed of the node for the action
// log: it became public or stopped being, a new author, a new address.
func AuditNodeChange(ctx context.Context, before, after entity.Node) {
	obj := nodeObject(after, true)
	switch {
	case !before.IsPublic() && after.IsPublic():
		audit.Record(ctx, audit.Action{Type: audit.NodePublish, Object: obj})
	case before.IsPublic() && !after.IsPublic():
		audit.Record(ctx, audit.Action{Type: audit.NodeUnpublish, Object: obj})
	}
	if before.CreatedBy.ID != after.CreatedBy.ID {
		audit.Record(ctx, audit.Action{Type: audit.NodeAuthor, Object: obj, Changes: []audit.Change{
			{Field: i18n.T(ctx, "Author"), From: before.CreatedBy.Username, To: after.CreatedBy.Username},
		}})
	}
	if before.Slug != after.Slug {
		audit.Record(ctx, audit.Action{Type: audit.NodeAddress, Object: obj, Changes: []audit.Change{
			{Field: i18n.T(ctx, "Address"), From: "/" + before.Slug, To: "/" + after.Slug},
		}})
	}
}
