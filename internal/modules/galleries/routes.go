package galleries

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// RegisterGalleryRoutes registers the galleries' routes on r: a node's
// galleries list, editing and deleting a gallery, its public page, the
// lazy-loaded galleries block of a node page and the JSON API of galleries
// and their images.
func RegisterGalleryRoutes(r modulekit.Mux, gc *GalleryController) {
	// Gallery management routes
	r.HandleFunc("GET /node/{id}/galleries", gc.GalleriesList)
	r.HandleFunc("GET /node/{id}/galleries/{gallery_id}/edit", gc.EditGallery)
	r.HandleFunc("GET /node/{id}/galleries/{gallery_id}/delete", gc.GalleryDeleteConfirm)
	r.HandleFunc("POST /node/{id}/galleries/{gallery_id}/delete", gc.GalleryDeletePage)

	// Public gallery view
	r.HandleFunc("GET /galleries/{id}", gc.ViewGallery)
	r.HandleFunc("GET /galleries/{gallery_id}/delete", gc.GalleryDeleteConfirm)

	// Lazy-loaded galleries block on a node page (HTML fragment).
	r.HandleFunc("GET /api/node/{id}/galleries-embed", gc.GalleriesEmbedHTML)

	// Gallery API endpoints
	r.HandleFunc("POST /api/node/{id}/galleries", gc.GalleryCreate)
	r.HandleFunc("PATCH /api/gallery/{id}", gc.GalleryUpdate)
	r.HandleFunc("POST /api/gallery/{id}/items", gc.GalleryAddItem)
	r.HandleFunc("DELETE /api/gallery/items/{id}", gc.GalleryRemoveItem)
	r.HandleFunc("POST /api/gallery/{id}/reorder", gc.GalleryReorder)
	r.HandleFunc("PATCH /api/gallery/items/{id}", gc.GalleryUpdateItem)
	r.HandleFunc("POST /api/gallery/{id}/set-cover", gc.GallerySetCover)
	r.HandleFunc("POST /api/gallery/{id}/set-node-main-image", gc.GallerySetNodeMainImage)
}

// RegisterAdminRoutes registers the galleries overview behind admin
// (sign-in and the admin permission).
func RegisterAdminRoutes(r modulekit.Mux, gc *GalleryController, admin func(http.Handler) http.Handler) {
	r.Handle("GET "+overviewPath, admin(http.HandlerFunc(gc.GalleriesOverview)))
	r.Handle("POST "+overviewPath+"/{nid}", admin(http.HandlerFunc(gc.GalleriesOverviewToggle)))
}
