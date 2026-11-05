package interfaces

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/lib/htmlimages"
)

// TestLoadFailure - a missing record, however wrapped, is a not-found; any
// other failure is the server's: a 500, not a 404.
func TestLoadFailure(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want bool
	}{
		"ent not found":    {fmt.Errorf("error finding gallery by ID: %w", &generated.NotFoundError{}), true},
		"entity not found": {fmt.Errorf("%w: node with ID 5", entity.ErrEntityNotFound), true},
		"database down":    {errors.New("dial tcp: connection refused"), false},
	} {
		if got := isNotFound(c.err); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	bc := &BaseController{logger: testLogger()}
	rec := httptest.NewRecorder()
	bc.RenderLoadFailure(rec, httptest.NewRequest(http.MethodGet, "/gallery/5", nil), errors.New("connection refused"), "error loading gallery", "galleryID", 5)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("a failure: %d, want 500", rec.Code)
	}
}

// fakeTemplateService is a minimal service.TemplateService for testing RenderOrFail
// without a real template set.
type fakeTemplateService struct {
	err     error
	written string
}

func (f *fakeTemplateService) Render(w io.Writer, name string, data any, r *http.Request, funcMap template.FuncMap) error {
	if f.err != nil {
		return f.err
	}
	_, err := io.WriteString(w, f.written)
	return err
}

func (f *fakeTemplateService) RenderNamed(w io.Writer, blockName string, data any, templateFiles ...string) error {
	if f.err != nil {
		return f.err
	}
	_, err := io.WriteString(w, f.written)
	return err
}

func (f *fakeTemplateService) SetContentProcessor(_ *htmlimages.ContentProcessor) {}

func newTestBaseController(ts *fakeTemplateService) *BaseController {
	return &BaseController{
		templateService: ts,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestRenderOrFail_Success(t *testing.T) {
	c := newTestBaseController(&fakeTemplateService{written: "<html>ok</html>"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	ok := c.RenderOrFail(w, r, "some/template.html", map[string]any{"title": "x"})

	if !ok {
		t.Fatal("expected RenderOrFail to return true on success")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected default 200 status (no WriteHeader called), got %d", w.Code)
	}
	if w.Body.String() != "<html>ok</html>" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
}

func TestRenderOrFail_Error(t *testing.T) {
	c := newTestBaseController(&fakeTemplateService{err: errors.New("template: boom")})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	ok := c.RenderOrFail(w, r, "broken/template.html", map[string]any{})

	if ok {
		t.Fatal("expected RenderOrFail to return false on render error")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatal("expected an error body to be written, got empty response")
	}
}
