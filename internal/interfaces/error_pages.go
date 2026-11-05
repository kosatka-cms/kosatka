package interfaces

import (
	"context"
	"math/rand"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
)

// statusWriter answers with an error status instead of 200 and marks the
// page noindex: a node rendered as the site's 404/403 page.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusWriter) WriteHeader(code int) {
	if s.wrote {
		return
	}
	s.wrote = true
	if code == http.StatusOK {
		code = s.status
	}
	s.Header().Set("X-Robots-Tag", "noindex")
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if !s.wrote {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}

// errorPageKey marks a request rendering an error page, so a failure
// inside it falls back to the default page instead of recursing.
type errorPageKey struct{}

func renderingErrorPage(r *http.Request) bool {
	return r.Context().Value(errorPageKey{}) != nil
}

// errorPageRenderer renders the site's configured page for an error
// status; set at startup (SetErrorPageRenderer).
var errorPageRenderer func(w http.ResponseWriter, r *http.Request, status int) bool

// SetErrorPageRenderer sets the renderer of the configured 404/403 pages.
func SetErrorPageRenderer(f func(w http.ResponseWriter, r *http.Request, status int) bool) {
	errorPageRenderer = f
}

// renderConfiguredErrorPage renders the configured page for status;
// false - none configured or it can't be shown (use the default page).
func renderConfiguredErrorPage(w http.ResponseWriter, r *http.Request, status int) bool {
	if errorPageRenderer == nil || renderingErrorPage(r) {
		return false
	}
	return errorPageRenderer(w, r, status)
}

// SetNodes gives the dispatcher the nodes repository (error pages).
func (nd *NodeDispatcher) SetNodes(nodes entity.NodesRepository) {
	nd.nodes = nodes
}

// RenderErrorPage renders one of the nodes configured for status (Site
// information → 404/403 page) as the response, with that status. False
// when none is configured or the chosen node is missing or unpublished.
func (nd *NodeDispatcher) RenderErrorPage(w http.ResponseWriter, r *http.Request, status int) bool {
	ids := service.ErrorPageIDs(status)
	if len(ids) == 0 || nd.nodes == nil {
		return false
	}
	node, err := nd.nodes.ByID(r.Context(), ids[rand.Intn(len(ids))])
	if err != nil && nd.dynamicCtrl != nil {
		service.LogFallback(nd.dynamicCtrl.logger, err, "loading the configured error page (the default one shown)", "status", status)
	}
	if err != nil || node == nil || node.Status != 1 {
		return false
	}
	ctx := context.WithValue(middleware.WithNode(r.Context(), node), errorPageKey{}, true)
	nd.View(&statusWriter{ResponseWriter: w, status: status}, r.WithContext(ctx))
	return true
}
