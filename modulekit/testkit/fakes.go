package testkit

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	router "github.com/kosatka-cms/kosatka/internal/http"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
)

// Auth - the core's AuthService of a handler test: UserID is the
// signed-in user (0 - none), Allow answers the content permission checks
// (create, edit, delete), Permissions the site-wide ones; any other
// method panics (the embedded nil interface).
type Auth struct {
	modulekit.AuthService
	UserID      int
	Allow       bool
	Permissions map[string]bool
}

// GetUserID - UserID, signed in when it isn't 0.
func (a Auth) GetUserID(*http.Request) (int, bool) { return a.UserID, a.UserID > 0 }

// HasPermission - the site-wide permissions in Permissions (none by
// default: the pages render as for a regular user).
func (a Auth) HasPermission(_ context.Context, _ *modulekit.User, perm string) bool {
	return a.Permissions[perm]
}

// CreatableNodeTypes - none.
func (a Auth) CreatableNodeTypes(context.Context, *modulekit.User) []modulekit.CreatableMaterialLink {
	return nil
}

// CanUseSiteInMaintenanceMode - no one.
func (a Auth) CanUseSiteInMaintenanceMode(context.Context, *modulekit.User) bool { return false }

// CanViewNode - a published node, or any while Allow.
func (a Auth) CanViewNode(_ context.Context, n *modulekit.Node, _ *modulekit.User) bool {
	return a.Allow || n.IsPublic()
}

// ContentVisitor - nothing hidden: all the content is open.
func (a Auth) ContentVisitor(context.Context, *modulekit.User) (modulekit.ContentVisitor, error) {
	return modulekit.ContentVisitor{}, nil
}

// CanEditNode - Allow.
func (a Auth) CanEditNode(context.Context, *modulekit.Node, *modulekit.User) bool { return a.Allow }

// CanDeleteNode - Allow.
func (a Auth) CanDeleteNode(context.Context, *modulekit.Node, *modulekit.User) bool { return a.Allow }

// CanCreateNode - Allow.
func (a Auth) CanCreateNode(context.Context, modulekit.ContentType, *modulekit.User) bool {
	return a.Allow
}

// Users - the core's UserService knowing the users by ID (Known); any
// other method panics.
type Users struct {
	modulekit.UserService
	Known map[int]*modulekit.User
}

// ByID - the user in Known, else ErrEntityNotFound.
func (u Users) ByID(_ context.Context, id int) (*modulekit.User, error) {
	if user, ok := u.Known[id]; ok {
		return user, nil
	}
	return nil, modulekit.ErrEntityNotFound
}

// Nodes - the core's NodesRepository knowing the content by ID (Known);
// any other method panics.
type Nodes struct {
	modulekit.NodesRepository
	Known map[int]*modulekit.Node
}

// ByID - the content in Known, else ErrEntityNotFound.
func (n Nodes) ByID(_ context.Context, id int) (*modulekit.Node, error) {
	if node, ok := n.Known[id]; ok {
		return node, nil
	}
	return nil, modulekit.ErrEntityNotFound
}

// Logger - a logger writing nowhere.
func Logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// NodeDeleter - the core's deleting of content of a handler test: nodes
// finds an item, deleteNode deletes it (its files untouched).
func NodeDeleter(nodes modulekit.NodesRepository, deleteNode func(ctx context.Context, id int) error) modulekit.NodeDeleter {
	return service.NewNodeDeleteService(nodes, nodeDelete(deleteNode), noFiles{}, noFiles{}, nil, Logger())
}

type nodeDelete func(ctx context.Context, id int) error

func (d nodeDelete) DeleteNode(ctx context.Context, id int) error { return d(ctx, id) }

type noFiles struct{}

func (noFiles) NodeFileIDs(context.Context, []int) ([]int, error)                { return nil, nil }
func (noFiles) DeleteUnreferencedFiles(context.Context, []int) ([]string, error) { return nil, nil }
func (noFiles) RemoveStoredFile(context.Context, string) error                   { return nil }
func (noFiles) RemoveUserDir(context.Context, int) error                         { return nil }

// ModuleMux - the module's routes' mux on mux, as the site gives it: its
// routes answer notFound while the module is off.
func ModuleMux(mux *http.ServeMux, module string, notFound http.HandlerFunc) modulekit.Mux {
	return router.ModuleMux(mux, module, notFound)
}

// Session - a cookie store with a session holding a CSRF token: its
// cookies and the token, for a form a test posts.
func Session(t testing.TB) (*sessions.CookieStore, []*http.Cookie, string) {
	t.Helper()
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	rec := httptest.NewRecorder()
	csrf.EnsureToken(store, rec, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := rec.Result().Cookies()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	token := csrf.Token(store, req)
	if token == "" {
		t.Fatal("no CSRF token issued")
	}
	return store, cookies, token
}

// PostForm - a form POST to path (with the session's cookies; the form
// carries what the test gives, the CSRF token too if it wants it).
func PostForm(path string, form url.Values, cookies []*http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}

// CaptureActions - the actions recorded for the action log during the
// test (modulekit.RecordAction, RecordNodeCreated…), in order; the other
// listeners are dropped until it ends.
func CaptureActions(t *testing.T) *[]modulekit.ActionEvent {
	t.Helper()
	audit.ResetListeners()
	t.Cleanup(audit.ResetListeners)
	var got []modulekit.ActionEvent
	modulekit.OnAction("", func(_ context.Context, e modulekit.ActionEvent) { got = append(got, e) })
	return &got
}
