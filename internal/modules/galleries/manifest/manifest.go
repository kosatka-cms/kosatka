// Package manifest declares the galleries module (its registry entry).
package manifest

import (
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name.
const Name = "galleries"

// PluginBlockNodeGalleries - the galleries of the content page the block
// is on.
const PluginBlockNodeGalleries manifestkit.PluginBlockID = "node_galleries"

// NodeGalleriesPreviews - the block's setting: how many previews each
// gallery shows.
const NodeGalleriesPreviews = "previews"

// Module - the module's registry entry.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Galleries"), Description: i18n.N("Image galleries attached to content: the galleries button and the content's galleries block on content pages, gallery pages."),
		Category: manifestkit.CategoryMedia,
		JS:       []string{"galleries"},
		CSS:      []string{"galleries"},
		// Installing the module places the block inside the content page.
		PluginBlocks:   []manifestkit.ModulePluginBlock{{ID: PluginBlockNodeGalleries, Label: i18n.N("The content's galleries"), Region: "content_inline"}},
		RobotsDisallow: []string{"/galleries/*/delete", "/node/*/galleries/*/edit", "/node/*/galleries/*/delete"},
		Paths:          []string{"/galleries"},
		Tables:         []string{"galleries", "gallery_items", "gallery_content_types", "gallery_nodes"},
		Data:           []manifestkit.ModuleData{{Label: i18n.N("Galleries"), Table: "galleries"}, {Label: i18n.N("Gallery images"), Table: "gallery_items"}},
		NodeFiles:      []manifestkit.ModuleNodeFiles{{Table: "gallery_items", FileColumn: "fid", Via: "galleries", ViaColumn: "gallery_id", NodeColumn: "nid"}},
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionContent,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Galleries"), Path: "/admin/content/galleries"}}},
	}
}
