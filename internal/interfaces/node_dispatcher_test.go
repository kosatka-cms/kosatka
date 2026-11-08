package interfaces

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
)

// stubTypeController - a module's type's pages, answering 200 everywhere;
// with preview, a NodeFormPreviewer answering 202.
type stubTypeController struct{}

func (stubTypeController) Detail(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
func (stubTypeController) CreateForm(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
func (stubTypeController) Create(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
func (stubTypeController) Edit(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
func (stubTypeController) Save(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
func (stubTypeController) Delete(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type stubPreviewingController struct{ stubTypeController }

func (stubPreviewingController) FormPreview(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusAccepted)
}

// TestNodeDispatcherModulePreview - a module's type previews its form
// through its NodeFormPreviewer; a type without one has no preview.
func TestNodeDispatcherModulePreview(t *testing.T) {
	// The not-found page: its status only.
	SetErrorPageRenderer(func(w http.ResponseWriter, _ *http.Request, status int) bool { w.WriteHeader(status); return true })
	t.Cleanup(func() { SetErrorPageRenderer(nil) })
	ctrl := &DynamicContentController{BaseController: &BaseController{}}
	nd := NewNodeDispatcher(ctrl)
	nd.RegisterType("stubbed", stubTypeController{})
	nd.RegisterType("previewed", stubPreviewingController{})
	served := func(nodeType entity.ContentType) int {
		r := httptest.NewRequest(http.MethodPost, "/node/1/edit/preview", nil)
		r = r.WithContext(middleware.WithNode(context.Background(), &entity.Node{ID: 1, Type: nodeType}))
		w := httptest.NewRecorder()
		nd.modulePreview(nd.typeController(&entity.Node{Type: nodeType}), w, r)
		return w.Code
	}
	if got := served("previewed"); got != http.StatusAccepted {
		t.Errorf("a type with a previewer: %d", got)
	}
	if got := served("stubbed"); got != http.StatusNotFound {
		t.Errorf("a type without one: %d, want 404", got)
	}
}
