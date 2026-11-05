// Package manifest declares the video embed module (its registry entry):
// a link to a video on its own line in a text shows as the video's player.
package manifest

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/ent"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Name - the module's machine name; also its text filter's
// (modulekit.AddTextFilter).
const Name = "video_embed"

// AdminPath - the module's settings page.
const AdminPath = "/admin/config/video-embed"

// PreviewStyle - the image style of the previews' copies on the site.
const PreviewStyle = "video_preview"

// Module - the module's registry entry: its tables are its own schema
// (RegisterSteps). New to the engine after sites were installed: off on
// them until enabled, installed on a new site.
func Module() manifestkit.Module {
	return manifestkit.Module{
		Name: Name, Label: i18n.N("Video embeds"),
		Description: i18n.N("A link to a video (YouTube, VK Video, Rutube, Vimeo) on its own line in a text shows as the video's player: a preview first, the player on a click. On for the text formats that tick it."),
		Category:    manifestkit.CategoryMedia,
		CSS:         []string{"video_embed"},
		JS:          []string{"video_embed"},
		Tables:      []string{"video_embeds", "video_embed_settings"},
		AdminMenu: []manifestkit.ModuleMenuItem{{Section: manifestkit.AdminSectionConfiguration,
			Item: manifestkit.AdminMenuItem{Label: i18n.N("Video embeds"), Path: AdminPath, Group: manifestkit.AdminGroupTexts}}},
		InstallByDefault: true,
		OffByDefault:     true,
	}
}

// RegisterSteps registers the module's own ent schema (its tables and
// migration files) and its install step (manifests.Register).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	manifestkit.RegisterInstall(Name, install)
}

// install - the previews' image style (a style already there is left as
// it is) and the module's text filter on for the text formats with the
// visual editor.
func install(ctx context.Context, env manifestkit.InstallEnv) error {
	if err := manifestkit.EnsureImageStyle(ctx, env, manifestkit.ImageStyle{Name: PreviewStyle, Label: i18n.N("Video preview"), Mode: "fit", Anchor: "center", Width: 960}); err != nil {
		return err
	}
	return manifestkit.EnableTextFilter(ctx, env, Name)
}
