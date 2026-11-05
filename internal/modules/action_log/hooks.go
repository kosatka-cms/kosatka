package actionlog

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Hooks - the log beside what it's about: the "Log" row of a content's
// form, the log icon of a user in the admin's list.
type Hooks struct {
	repo  *Repo
	users modulekit.UserRepository
	auth  modulekit.AuthService
}

// canView - whether ctx's user may see the log.
func (h Hooks) canView(ctx context.Context) bool {
	id := modulekit.ActionActorFrom(ctx).UserID
	if id == 0 || h.users == nil || h.auth == nil {
		return false
	}
	u, err := h.users.ByID(ctx, id)
	return err == nil && u != nil && h.auth.HasPermission(ctx, u, manifest.PermView)
}

// PageData - every page's data (AddPageData): actionLogCanView, whether
// the user may see the log (the users list's log icons).
func (h Hooks) PageData(r *http.Request, data map[string]any) {
	if h.canView(r.Context()) {
		data["actionLogCanView"] = true
	}
}

// nodeLog - the content's row: how many entries, the address of its log.
type nodeLog struct {
	Count int
	URL   string
}

// NodeSettings - the "Log" row of a content's form (a link, nothing
// saved): for an existing content and a user who may see the log.
func (h Hooks) NodeSettings() modulekit.NodeSettings {
	return modulekit.NodeSettings{
		Module: manifest.Name,
		Load: func(ctx context.Context, _ string, node *modulekit.Node, _ url.Values) (any, error) {
			if node == nil || node.ID == 0 || !h.canView(ctx) {
				return nil, nil
			}
			id := strconv.Itoa(node.ID)
			n, err := h.repo.Count(ctx, Filter{ObjectKind: "node", ObjectID: id})
			if err != nil {
				return nil, err
			}
			return nodeLog{Count: n, URL: ObjectLink("node", id)}, nil
		},
		Save: func(context.Context, modulekit.Node, *http.Request) error { return nil },
	}
}
