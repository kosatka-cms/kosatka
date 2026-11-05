package interfaces

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
)

// NodeTypeController - a module's own pages of its content type (the
// node page and the create/edit/delete forms), registered with
// NodeDispatcher.RegisterType.
type NodeTypeController interface {
	Detail(w http.ResponseWriter, r *http.Request)
	CreateForm(w http.ResponseWriter, r *http.Request)
	Create(w http.ResponseWriter, r *http.Request)
	Edit(w http.ResponseWriter, r *http.Request)
	Save(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}

// NodeDispatcher serves a node's pages and forms: by the controller its
// module registered for its type (RegisterType), else by the
// DynamicContentController.
type NodeDispatcher struct {
	// types - the modules' content types with their own pages
	// (RegisterType).
	types map[entity.ContentType]NodeTypeController
	// dynamicCtrl handles every content type without its own pages;
	// unknown machine names yield "Unknown node type".
	dynamicCtrl *DynamicContentController
	// nodes loads the configured error pages (RenderErrorPage).
	nodes entity.NodesRepository
}

// NewNodeDispatcher creates a NodeDispatcher serving the types without
// their own pages by dynamicCtrl.
func NewNodeDispatcher(dynamicCtrl *DynamicContentController) *NodeDispatcher {
	return &NodeDispatcher{dynamicCtrl: dynamicCtrl}
}

// RegisterType gives a module's content type its own pages.
func (nd *NodeDispatcher) RegisterType(t entity.ContentType, c NodeTypeController) {
	if nd.types == nil {
		nd.types = map[entity.ContentType]NodeTypeController{}
	}
	nd.types[t] = c
}

// typeController - the registered controller of the node's type (nil -
// none).
func (nd *NodeDispatcher) typeController(node *entity.Node) NodeTypeController {
	if node == nil {
		return nil
	}
	return nd.types[node.Type]
}

// View - the node page. Types that redirect their page elsewhere do so via
// ContentTypeDef.RedirectField in dynamicCtrl.Detail; the registered
// module types have their own pages.
func (nd *NodeDispatcher) View(w http.ResponseWriter, r *http.Request) {
	node, _ := middleware.GetNode(r)
	// A node configured as the 404/403 page answers with that status at
	// its own address too.
	if !renderingErrorPage(r) {
		if status := service.ErrorPageStatus(node.ID); status != 0 {
			w = &statusWriter{ResponseWriter: w, status: status}
		}
	}
	nd.dispatch(w, r, NodeTypeController.Detail, nd.dynamicCtrl.Detail)
}

// CreateForm - GET /create/{type}: the form of a new node of the type.
func (nd *NodeDispatcher) CreateForm(w http.ResponseWriter, r *http.Request) {
	nd.dispatch(w, r, NodeTypeController.CreateForm, nd.dynamicCtrl.CreateForm)
}

// Create - POST /create/{type}: the new node's form submitted.
func (nd *NodeDispatcher) Create(w http.ResponseWriter, r *http.Request) {
	nd.dispatch(w, r, NodeTypeController.Create, nd.dynamicCtrl.CreateSubmit)
}

// Edit - GET /node/{id}/edit: the node's edit form.
func (nd *NodeDispatcher) Edit(w http.ResponseWriter, r *http.Request) {
	nd.dispatch(w, r, NodeTypeController.Edit, nd.dynamicCtrl.EditForm)
}

// Save - POST /node/{id}/edit: the node's edit form submitted.
func (nd *NodeDispatcher) Save(w http.ResponseWriter, r *http.Request) {
	nd.dispatch(w, r, NodeTypeController.Save, nd.dynamicCtrl.Save)
}

// Delete - POST /node/{id}/delete: the node deleted.
func (nd *NodeDispatcher) Delete(w http.ResponseWriter, r *http.Request) {
	nd.dispatch(w, r, NodeTypeController.Delete, nd.dynamicCtrl.Delete)
}

// Clone - a content type's node copied (DynamicContentController.Clone);
// a module's type has no copying: not found.
func (nd *NodeDispatcher) Clone(w http.ResponseWriter, r *http.Request) {
	nd.dispatch(w, r, func(_ NodeTypeController, w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}, nd.dynamicCtrl.Clone)
}

// Preview - the item a preview link shows (DynamicContentController.
// Preview).
func (nd *NodeDispatcher) Preview(w http.ResponseWriter, r *http.Request) {
	nd.dynamicCtrl.Preview(w, r)
}

// MediaPick - the media library's files a field may take
// (DynamicContentController.MediaPick).
func (nd *NodeDispatcher) MediaPick(w http.ResponseWriter, r *http.Request) {
	nd.dynamicCtrl.MediaPick(w, r)
}

// PreviewLink - a content type's node's preview link made or revoked
// (DynamicContentController.PreviewLinkAction); a module's type has none.
func (nd *NodeDispatcher) PreviewLink(w http.ResponseWriter, r *http.Request) {
	nd.dispatch(w, r, func(_ NodeTypeController, w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}, nd.dynamicCtrl.PreviewLinkAction)
}

// dispatch serves r for its node: 404 for a type of a disabled module
// (typeDisabled), the type's own page (own) when a module registered it,
// else the general one.
func (nd *NodeDispatcher) dispatch(w http.ResponseWriter, r *http.Request, own func(NodeTypeController, http.ResponseWriter, *http.Request), general http.HandlerFunc) {
	node, _ := middleware.GetNode(r)
	if nd.typeDisabled(w, r, node) {
		return
	}
	if c := nd.typeController(node); c != nil {
		own(c, w, r)
		return
	}
	general(w, r)
}

// typeDisabled answers 404 for a node whose type belongs to a disabled
// module.
func (nd *NodeDispatcher) typeDisabled(w http.ResponseWriter, r *http.Request, node *entity.Node) bool {
	if node == nil || service.NodeTypeEnabled(string(node.Type)) {
		return false
	}
	nd.dynamicCtrl.RenderNotFound(w, r)
	return true
}
