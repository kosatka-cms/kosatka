package galleries

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// User 1 may edit node 1 (gallery 1, items 10-11); node 2 (gallery 2,
// item 20) belongs to someone else.
type galleryAccessAuth struct{ modulekit.AuthService }

func (galleryAccessAuth) GetUserID(*http.Request) (int, bool) { return 1, true }
func (galleryAccessAuth) CanEditNode(_ context.Context, node *modulekit.Node, _ *modulekit.User) bool {
	return node.ID == 1
}

type galleryAccessUsers struct{ modulekit.UserService }

func (galleryAccessUsers) ByID(_ context.Context, id int) (*modulekit.User, error) {
	return &modulekit.User{ID: id}, nil
}

type galleryAccessNodes struct{ modulekit.NodesService }

func (galleryAccessNodes) ByID(_ context.Context, id int) (*modulekit.Node, error) {
	return &modulekit.Node{ID: id}, nil
}

type galleryAccessGalleries struct {
	GalleryService
	changed []string
}

var galleryAccessItems = map[int]*GalleryItem{
	10: {ID: 10, GalleryID: 1, FileID: 100},
	11: {ID: 11, GalleryID: 1, FileID: 101},
	20: {ID: 20, GalleryID: 2, FileID: 200},
}

func (g *galleryAccessGalleries) GetItemByID(_ context.Context, id int) (*GalleryItem, error) {
	if it, ok := galleryAccessItems[id]; ok {
		return it, nil
	}
	return nil, errors.New("not found")
}

func (g *galleryAccessGalleries) ByID(_ context.Context, id int) (*Gallery, error) {
	return &Gallery{ID: id, NodeID: id}, nil
}

func (g *galleryAccessGalleries) GetItems(_ context.Context, galleryID int) (GalleryItems, error) {
	var out GalleryItems
	for _, it := range galleryAccessItems {
		if it.GalleryID == galleryID {
			out = append(out, it)
		}
	}
	return out, nil
}

func (g *galleryAccessGalleries) RemoveItem(context.Context, int) error {
	g.changed = append(g.changed, "remove")
	return nil
}

func (g *galleryAccessGalleries) UpdateItem(_ context.Context, id int, _ GalleryItemUpdateInput) (*GalleryItem, error) {
	g.changed = append(g.changed, "update")
	return galleryAccessItems[id], nil
}

func (g *galleryAccessGalleries) SetCover(context.Context, int, int) error {
	g.changed = append(g.changed, "cover")
	return nil
}

func (g *galleryAccessGalleries) ReorderItems(context.Context, []GalleryItemReorder) error {
	g.changed = append(g.changed, "reorder")
	return nil
}

// TestGalleryItemAccess - item endpoints check the item's own gallery:
// another node's item can't be removed, edited, made a cover or reordered.
func TestGalleryItemAccess(t *testing.T) {
	for _, c := range []struct {
		name, method, pattern, path, body string
		handler                           func(*GalleryController) http.HandlerFunc
		code                              int
	}{
		{"remove own", "DELETE", "/api/gallery/items/{id}", "/api/gallery/items/10", "", func(g *GalleryController) http.HandlerFunc { return g.GalleryRemoveItem }, http.StatusOK},
		{"remove foreign", "DELETE", "/api/gallery/items/{id}", "/api/gallery/items/20", "", func(g *GalleryController) http.HandlerFunc { return g.GalleryRemoveItem }, http.StatusForbidden},
		{"remove missing", "DELETE", "/api/gallery/items/{id}", "/api/gallery/items/99", "", func(g *GalleryController) http.HandlerFunc { return g.GalleryRemoveItem }, http.StatusNotFound},
		{"update own", "PATCH", "/api/gallery/items/{id}", "/api/gallery/items/10", "title=x", func(g *GalleryController) http.HandlerFunc { return g.GalleryUpdateItem }, http.StatusOK},
		{"update foreign", "PATCH", "/api/gallery/items/{id}", "/api/gallery/items/20", "title=x", func(g *GalleryController) http.HandlerFunc { return g.GalleryUpdateItem }, http.StatusForbidden},
		{"cover own", "POST", "/api/gallery/{id}/set-cover", "/api/gallery/1/set-cover", "item_id=11", func(g *GalleryController) http.HandlerFunc { return g.GallerySetCover }, http.StatusOK},
		{"cover foreign item", "POST", "/api/gallery/{id}/set-cover", "/api/gallery/1/set-cover", "item_id=20", func(g *GalleryController) http.HandlerFunc { return g.GallerySetCover }, http.StatusBadRequest},
		{"main image foreign item", "POST", "/api/gallery/{id}/set-node-main-image", "/api/gallery/1/set-node-main-image", "item_id=20", func(g *GalleryController) http.HandlerFunc { return g.GallerySetNodeMainImage }, http.StatusBadRequest},
		{"reorder own", "POST", "/api/gallery/{id}/reorder", "/api/gallery/1/reorder", `{"items":[{"ID":10,"Weight":1},{"ID":11,"Weight":0}]}`, func(g *GalleryController) http.HandlerFunc { return g.GalleryReorder }, http.StatusOK},
		{"reorder foreign item", "POST", "/api/gallery/{id}/reorder", "/api/gallery/1/reorder", `{"items":[{"ID":10,"Weight":1},{"ID":20,"Weight":0}]}`, func(g *GalleryController) http.HandlerFunc { return g.GalleryReorder }, http.StatusBadRequest},
	} {
		t.Run(c.name, func(t *testing.T) {
			galleries := &galleryAccessGalleries{}
			gc := &GalleryController{
				BaseController: modulekit.NewBaseController(modulekit.BaseDeps{Auth: galleryAccessAuth{}, Users: galleryAccessUsers{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}),
				nodeService:    galleryAccessNodes{},
				galleryService: galleries,
			}
			mux := http.NewServeMux()
			mux.HandleFunc(c.method+" "+c.pattern, c.handler(gc))
			req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
			if strings.HasPrefix(c.body, "{") {
				req.Header.Set("Content-Type", "application/json")
			} else {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != c.code {
				t.Fatalf("code %d, want %d: %s", rec.Code, c.code, rec.Body.String())
			}
			if c.code != http.StatusOK && len(galleries.changed) > 0 {
				t.Errorf("rejected request changed data: %v", galleries.changed)
			}
		})
	}
}

func TestCheckGalleryLen(t *testing.T) {
	// Cyrillic on purpose: two bytes per rune checks the limit counts runes.
	ok := strings.Repeat("я", galleryTitleMaxLen) // runes, not bytes
	if err := checkGalleryLen("title", &ok, galleryTitleMaxLen); err != nil {
		t.Fatalf("255 Cyrillic runes must fit: %v", err)
	}
	long := ok + "я"
	if err := checkGalleryLen("title", &long, galleryTitleMaxLen); !errors.Is(err, ErrGalleryFieldTooLong) {
		t.Fatalf("256 runes must be rejected with ErrGalleryFieldTooLong, got %v", err)
	}
	if err := checkGalleryLen("title", nil, galleryTitleMaxLen); err != nil {
		t.Fatalf("nil (field not submitted) must pass: %v", err)
	}
}
