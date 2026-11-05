package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// NodePermission - what a request does to a node, the node access
// middleware's check (hasPermission).
type NodePermission string

// The node permissions: viewing, editing, deleting, creating (of the
// route's type) and publishing a node.
const (
	PermissionView    NodePermission = "view"
	PermissionEdit    NodePermission = "edit"
	PermissionDelete  NodePermission = "delete"
	PermissionCreate  NodePermission = "create"
	PermissionPublish NodePermission = "publish"
)

// nodeCtxKey/userCtxKey use the contextKey type from auth.go to avoid collisions
// with string-keyed context values set elsewhere.
const (
	nodeCtxKey contextKey = "node"
	userCtxKey contextKey = "user"
)

// NodeAccessByIDMiddleware checks node access by node ID.
func NodeAccessByIDMiddleware(
	nodeByIDFn func(context.Context, int) (*entity.Node, error),
	getUserFn func(context.Context, int) (*entity.User, error),
	authService service.AuthService,
	permission NodePermission,
	notFound http.HandlerFunc,
) func(http.Handler) http.Handler {
	if notFound == nil {
		notFound = notFoundPage
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nodeIDStr := r.PathValue("id")
			if nodeIDStr == "" {
				notFound(w, r)
				return
			}

			nodeID, err := strconv.Atoi(nodeIDStr)
			if err != nil {
				notFound(w, r)
				return
			}

			node, err := nodeByIDFn(r.Context(), nodeID)
			if err != nil {
				if apperrors.IsNotFound(err) {
					notFound(w, r)
					return
				}
				slog.Error("loading the node to check access", "nodeID", nodeID, "error", err)
				httputil.InternalServerError(w)
				return
			}
			// A blocks field's item has no pages: it is its node's.
			if node.ParentID != 0 {
				notFound(w, r)
				return
			}

			checkAccessAndServe(w, r, node, authService, getUserFn, permission, next)
		})
	}
}

// NodeAccessBySlugMiddleware checks node access by slug.
//
// dynamicListFallback runs only when no node matches the slug; it serves
// Views pages (/{view.URLPath}) through the same catch-all route, since
// next is never reached on a not-found slug. nil skips straight to 404.
func NodeAccessBySlugMiddleware(
	nodeBySlugFn func(context.Context, string) (*entity.Node, error),
	getUserFn func(context.Context, int) (*entity.User, error),
	authService service.AuthService,
	permission NodePermission,
	notFound http.HandlerFunc,
	dynamicListFallback func(w http.ResponseWriter, r *http.Request) bool,
) func(http.Handler) http.Handler {
	if notFound == nil {
		notFound = notFoundPage
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			node, err := nodeBySlugFn(r.Context(), path)
			if err != nil {
				if apperrors.IsNotFound(err) {
					if dynamicListFallback != nil && dynamicListFallback(w, r) {
						return
					}
					notFound(w, r)
					return
				}
				slog.Error("loading the node to check access", "path", path, "error", err)
				httputil.InternalServerError(w)
				return
			}

			// A node in another language than the address's: to its own
			// address (one page, one address - /en/x of a Russian node is
			// /x).
			if entity.LanguagePrefix(node.Langcode) != entity.LanguagePrefix(i18n.FromContext(r.Context())) {
				target := node.URL()
				if r.URL.RawQuery != "" {
					target += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
			checkAccessAndServe(w, r, node, authService, getUserFn, permission, next)
		})
	}
}

// NodeIDRedirect - the slug route's fallback for /node/<id> (no node or
// View has that address): whoever may view node <id> goes to its own
// address, the query kept - a link that outlives the address (the action
// log's). A temporary redirect: the address may change. One who may not
// view it is answered as on its page (the login, 403). false - not such a
// path, or no such node (the caller answers 404).
func NodeIDRedirect(
	nodeByIDFn func(context.Context, int) (*entity.Node, error),
	getUserFn func(context.Context, int) (*entity.User, error),
	authService service.AuthService,
) func(w http.ResponseWriter, r *http.Request) bool {
	return func(w http.ResponseWriter, r *http.Request) bool {
		rest, ok := strings.CutPrefix(strings.Trim(r.URL.Path, "/"), "node/")
		if !ok {
			return false
		}
		id, err := strconv.Atoi(rest)
		if err != nil || id <= 0 {
			return false
		}
		node, err := nodeByIDFn(r.Context(), id)
		if err != nil {
			if apperrors.IsNotFound(err) {
				return false
			}
			slog.Error("loading the node to redirect to", "nodeID", id, "error", err)
			httputil.InternalServerError(w)
			return true
		}
		// A blocks field's item has no page of its own.
		if node.ParentID != 0 {
			return false
		}
		checkAccessAndServe(w, r, node, authService, getUserFn, PermissionView, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := node.URL()
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusFound)
		}))
		return true
	}
}

// NodeCreateAccessByTypeMiddleware checks that the request's user may
// create content of the route's {type} (a guest is sent to sign in);
// the handler finds a node of the type in the context (GetNode).
func NodeCreateAccessByTypeMiddleware(
	getUserFn func(context.Context, int) (*entity.User, error),
	authService service.AuthService,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nodeType := r.PathValue("type")
			if nodeType == "" {
				notFoundPage(w, r)
				return
			}

			node := &entity.Node{
				Type: entity.ContentType(nodeType),
			}

			checkAccessAndServe(w, r, node, authService, getUserFn, PermissionCreate, next)
		})
	}
}

// checkAccessAndServe holds the shared access check logic.
func checkAccessAndServe(
	w http.ResponseWriter,
	r *http.Request,
	node *entity.Node,
	authService service.AuthService,
	getUserFn func(context.Context, int) (*entity.User, error),
	permission NodePermission,
	next http.Handler,
) {
	userID, hasUserID := r.Context().Value(userIDKey).(int)

	var user *entity.User
	if hasUserID && userID != 0 {
		var err error
		user, err = getUserFn(r.Context(), userID)
		if err != nil {
			unauthorized(w, r)
			return
		}
	}

	if !hasPermission(r.Context(), user, node, permission, authService) {
		if user == nil {
			// Anonymous: redirect to login.
			http.Redirect(w, r, "/login", http.StatusFound)
		} else {
			// Logged in but not permitted.
			forbidden(w, r)
		}
		return
	}

	// Store node and user in the context.
	ctx := context.WithValue(r.Context(), nodeCtxKey, node)
	if user != nil {
		ctx = context.WithValue(ctx, userCtxKey, user)
	}

	next.ServeHTTP(w, r.WithContext(ctx))
}

// hasPermission - the request's user may do permission on node (the
// checks run in the request's context: its deadline, language, zone).
func hasPermission(
	ctx context.Context,
	user *entity.User,
	node *entity.Node,
	permission NodePermission,
	authService service.AuthService,
) bool {
	switch permission {
	case PermissionView:
		return authService.CanViewNode(ctx, node, user)
	case PermissionEdit:
		return authService.CanEditNode(ctx, node, user)
	case PermissionDelete:
		return authService.CanDeleteNode(ctx, node, user)
	case PermissionCreate:
		return authService.CanCreateNode(ctx, node.Type, user)
	case PermissionPublish:
		return authService.CanPublishNode(ctx, user)
	default:
		return false
	}
}

// ErrorPages - the site's pages the middleware answers errors with (set
// at startup, SetErrorPages); a missing one is plain text.
type ErrorPages struct {
	Forbidden http.HandlerFunc
	NotFound  http.HandlerFunc
	// Rejected says why what was sent is refused (status, a message in
	// the visitor's language).
	Rejected func(w http.ResponseWriter, r *http.Request, status int, message string)
	// ExpiredForm - the message of a form whose CSRF token has expired.
	ExpiredForm func(r *http.Request) string
}

var errorPages ErrorPages

// SetErrorPages sets the pages of the middleware's error responses.
func SetErrorPages(p ErrorPages) {
	errorPages = p
}

func forbidden(w http.ResponseWriter, r *http.Request) {
	if errorPages.Forbidden != nil {
		errorPages.Forbidden(w, r)
		return
	}
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// unauthorized - the request needs a signed-in user and has none (or one
// that no longer exists).
func unauthorized(w http.ResponseWriter, r *http.Request) {
	if errorPages.Rejected != nil {
		errorPages.Rejected(w, r, http.StatusUnauthorized, i18n.T(r.Context(), "Sign in to continue."))
		return
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func notFoundPage(w http.ResponseWriter, r *http.Request) {
	if errorPages.NotFound != nil {
		errorPages.NotFound(w, r)
		return
	}
	http.Error(w, "Not found", http.StatusNotFound)
}

// expiredForm - a state-changing request whose CSRF token doesn't match:
// a script (it sent the token header) gets JSON, a form the page.
func expiredForm(w http.ResponseWriter, r *http.Request) {
	msg := "Invalid CSRF token"
	if errorPages.ExpiredForm != nil {
		msg = errorPages.ExpiredForm(r)
	}
	switch {
	case r.Header.Get(csrf.HeaderName) != "":
		httputil.JSONError(w, msg, http.StatusForbidden)
	case errorPages.Rejected != nil:
		errorPages.Rejected(w, r, http.StatusForbidden, msg)
	default:
		http.Error(w, msg, http.StatusForbidden)
	}
}

// WithNode - ctx carrying node, as the node routes set it for the
// handlers (GetNode); used to render a node as an error page.
func WithNode(ctx context.Context, node *entity.Node) context.Context {
	return context.WithValue(ctx, nodeCtxKey, node)
}

// GetNode - the node the node access middleware put in r's context.
func GetNode(r *http.Request) (*entity.Node, bool) {
	node, ok := r.Context().Value(nodeCtxKey).(*entity.Node)
	return node, ok
}

// GetUser - the signed-in user the node access middleware put in r's
// context; false - a guest.
func GetUser(r *http.Request) (*entity.User, bool) {
	user, ok := r.Context().Value(userCtxKey).(*entity.User)
	return user, ok
}

// GetUserID - the ID of r's signed-in user; false - a guest.
func GetUserID(r *http.Request) (int, bool) { return GetUserIDFromContext(r.Context()) }
