package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

type requestMark struct{}

// markAuth - an AuthService whose node edit check records the context's
// mark; any other method panics.
type markAuth struct {
	service.AuthService
	seen *string
}

func (a markAuth) CanEditNode(ctx context.Context, _ *entity.Node, _ *entity.User) bool {
	*a.seen, _ = ctx.Value(requestMark{}).(string)
	return true
}

// TestNodeAccessByIDMiddleware - the permission check runs in the
// request's context; without a not-found handler a bad ID gets the 404
// page, not a panic.
func TestNodeAccessByIDMiddleware(t *testing.T) {
	var seen string
	nodes := func(_ context.Context, id int) (*entity.Node, error) {
		if id == 7 {
			return &entity.Node{ID: 7, Type: "article"}, nil
		}
		return nil, entity.ErrEntityNotFound
	}
	users := func(context.Context, int) (*entity.User, error) { return &entity.User{ID: 3}, nil }
	mw := NodeAccessByIDMiddleware(nodes, users, markAuth{seen: &seen}, PermissionEdit, nil)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	serve := func(id string) int {
		req := httptest.NewRequest(http.MethodGet, "/node/"+id+"/edit", nil)
		ctx := context.WithValue(req.Context(), requestMark{}, "this request")
		ctx = context.WithValue(ctx, userIDKey, 3)
		req = req.WithContext(ctx)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := serve("7"); code != http.StatusNoContent || seen != "this request" {
		t.Errorf("allowed: %d, the check saw %q", code, seen)
	}
	for _, id := range []string{"x", "99"} {
		if code := serve(id); code != http.StatusNotFound {
			t.Errorf("node %s: %d, want 404", id, code)
		}
	}
}

// viewAuth - an AuthService that lets view only the public nodes; any
// other method panics.
type viewAuth struct{ service.AuthService }

func (viewAuth) CanViewNode(_ context.Context, n *entity.Node, _ *entity.User) bool {
	return n.IsPublic()
}

// TestNodeIDRedirect - /node/<id> goes to the node's own address (the
// query kept) for whoever may view it, temporarily; a hidden node's
// address isn't given away (a guest is sent to sign in); another path,
// an unknown node or a blocks field's item is the caller's 404.
func TestNodeIDRedirect(t *testing.T) {
	nodes := func(_ context.Context, id int) (*entity.Node, error) {
		switch id {
		case 5:
			return &entity.Node{ID: 5, Type: "article", Slug: "tv/joker-laugh-riot", Status: 1}, nil
		case 6:
			return &entity.Node{ID: 6, Type: "article", Slug: "drafts/secret", Status: 0}, nil
		case 8:
			return &entity.Node{ID: 8, Type: "article", Slug: "x", Status: 1, ParentID: 5}, nil
		}
		return nil, entity.ErrEntityNotFound
	}
	users := func(context.Context, int) (*entity.User, error) { return &entity.User{ID: 3}, nil }
	redirect := NodeIDRedirect(nodes, users, viewAuth{})

	serve := func(target string) (bool, *httptest.ResponseRecorder) {
		rec := httptest.NewRecorder()
		handled := redirect(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return handled, rec
	}
	if handled, rec := serve("/node/5?page=2"); !handled || rec.Code != http.StatusFound || rec.Header().Get("Location") != "/tv/joker-laugh-riot?page=2" {
		t.Errorf("visible: %v %d %q", handled, rec.Code, rec.Header().Get("Location"))
	}
	if handled, rec := serve("/node/6"); !handled || rec.Header().Get("Location") != "/login" {
		t.Errorf("hidden, a guest: %v %d %q", handled, rec.Code, rec.Header().Get("Location"))
	}
	for _, target := range []string{"/node/99", "/node/8", "/node/x", "/node/5/extra", "/articles/5"} {
		if handled, rec := serve(target); handled {
			t.Errorf("%s: handled (%d), want the caller's 404", target, rec.Code)
		}
	}
}
