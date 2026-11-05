package interfaces

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

func TestStatusWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &statusWriter{ResponseWriter: rec, status: http.StatusNotFound}
	_, _ = w.Write([]byte("page"))
	if rec.Code != http.StatusNotFound || rec.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("implicit 200: %d %q", rec.Code, rec.Header().Get("X-Robots-Tag"))
	}
	rec = httptest.NewRecorder()
	w = &statusWriter{ResponseWriter: rec, status: http.StatusForbidden}
	w.WriteHeader(http.StatusSeeOther)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("non-200 kept: %d", rec.Code)
	}
}

type errorPageNodes struct{ entity.NodesRepository }

func (errorPageNodes) ByID(_ context.Context, id int) (*entity.Node, error) {
	switch id {
	case 5:
		return &entity.Node{ID: 5, Status: 0}, nil
	}
	return nil, errors.New("not found")
}

// TestRenderErrorPageFallsBack - no configured page, an unpublished or a
// deleted one: the caller renders the default page.
func TestRenderErrorPageFallsBack(t *testing.T) {
	t.Cleanup(func() { service.SetErrorPages(404, nil) })
	nd := &NodeDispatcher{nodes: errorPageNodes{}}
	r := httptest.NewRequest(http.MethodGet, "/missing", nil)
	for _, ids := range [][]int{nil, {5}, {99}} {
		service.SetErrorPages(404, ids)
		if nd.RenderErrorPage(httptest.NewRecorder(), r, http.StatusNotFound) {
			t.Errorf("ids %v: rendered", ids)
		}
	}
}

// TestConfiguredErrorPageNoRecursion - inside an error page render the
// hook is skipped, so a failure there uses the default page.
func TestConfiguredErrorPageNoRecursion(t *testing.T) {
	calls := 0
	SetErrorPageRenderer(func(http.ResponseWriter, *http.Request, int) bool { calls++; return true })
	t.Cleanup(func() { SetErrorPageRenderer(nil) })
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if !renderConfiguredErrorPage(httptest.NewRecorder(), r, 404) || calls != 1 {
		t.Fatal("hook not called")
	}
	inner := r.WithContext(context.WithValue(r.Context(), errorPageKey{}, true))
	if renderConfiguredErrorPage(httptest.NewRecorder(), inner, 404) || calls != 1 {
		t.Error("hook called inside an error page render")
	}
}
