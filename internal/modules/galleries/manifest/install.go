package manifest

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent"
	"github.com/kosatka-cms/kosatka/modulekit/demokit"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The galleries module's image styles. Labels in English, marked for the
// catalog (app install translates them).
var galleriesStyles = []manifestkit.ImageStyle{
	{Name: "gallery_cover", Label: i18n.N("Gallery cover"), Mode: "fill", Anchor: "top", Width: 600, Height: 200},
	{Name: "gallery_thumbnail", Label: i18n.N("Gallery thumbnail"), Mode: "fill", Anchor: "center", Width: 215, Height: 144},
	{Name: "gallery_medium", Label: i18n.N("Gallery medium"), Mode: "fill", Anchor: "top", Width: 250, Height: 250},
}

// RegisterSteps registers the module's own ent schema (its tables and
// migration files), its install step and its demo step
// (manifests.Register).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	demokit.Register(Name, createDemo)
	manifestkit.RegisterInstall(Name, install)
}

// install - the image styles (the core's: a style already there - kept by
// an uninstall, or made by the site - is left as it is); a gallery's
// previews use gallery_thumbnail and gallery_medium unless it chooses
// others.
func install(ctx context.Context, env manifestkit.InstallEnv) error {
	for _, st := range galleriesStyles {
		if err := manifestkit.EnsureImageStyle(ctx, env, st); err != nil {
			return err
		}
	}
	return nil
}
