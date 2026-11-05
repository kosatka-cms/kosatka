package galleries

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Wire - the module's services, controller, node page hooks, routes and
// [[gallery:<id>]] in texts (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	galleries := NewGalleryService(NewGalleryRepository(d.DB, d.NodeRepo, d.Files, d.ImageStyles), d.ImageStyles, d.Logger)
	settings := NewSettingsRepository(d.DB, d.ContentTypes)
	c := NewGalleryController(d.Base, GalleryDeps{
		FileUploadService: d.FileUpload,
		FileRepo:          d.Files,
		NodeService:       d.Nodes,
		GalleryService:    galleries,
		ContentTypes:      d.ContentTypes,
		Settings:          settings,
		ImageStyles:       d.ImageStyles,
	})
	RegisterHooks(settings, galleries, d.ContentTypes, d.Logger.With("component", "galleries.hooks"))
	modulekit.AddEmbedKind(c.embedKind())
	modulekit.SetPluginBlockSettings(manifest.PluginBlockNodeGalleries, func(context.Context) ([]modulekit.PluginBlockSetting, error) {
		return []modulekit.PluginBlockSetting{{Key: manifest.NodeGalleriesPreviews, Label: i18n.N("Previews of each gallery (0 - a row of its grid: 5, or 6 for the portrait grid)"),
			Type: modulekit.PluginBlockSettingNumber, Default: "0", Min: 0, Max: maxNodePreviews}}, nil
	})
	return modulekit.Parts{Routes: func(r modulekit.Routes) {
		RegisterAdminRoutes(r.Mux, c, r.AdminOnly)
		RegisterGalleryRoutes(r.Mux, c)
	}}
}
