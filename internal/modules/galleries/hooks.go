package galleries

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// typeSettingsForm - what the content type form's galleries section
// (content_type_settings_galleries) shows.
type typeSettingsForm struct {
	TypeSettings
	// ImageFields - the type's image fields, the choices of PictureField.
	ImageFields []modulekit.FieldInstanceDef
}

// nodePage - what the node page's galleries templates (node_actions_,
// node_links_galleries, the block_node_galleries block) read as
// .moduleNode.galleries.
type nodePage struct {
	Galleries Galleries
}

// RegisterHooks adds the module's parts of the core's pages: its section
// of the content type form and of the node form, the galleries button and
// links of node pages and their data for the block (wiring, at startup).
func RegisterHooks(settings SettingsRepository, galleries GalleryService, contentTypes modulekit.ContentTypeService, logger *slog.Logger) {
	modulekit.AddContentTypeSettings(typeSettingsHook(settings, contentTypes))
	modulekit.AddNodeSettings(nodeSettingsHook(settings))
	modulekit.AddNodePage(manifest.Name, nodePageHook(settings, galleries, logger))
}

// typeFields - the content types' fields (modulekit.ContentTypeService).
type typeFields interface {
	FieldsByContentTypeID(ctx context.Context, contentTypeID int) (modulekit.FieldInstanceDefs, error)
}

// typeSettingsHook - the content type form's galleries section: the
// node switch's default and the main content image field.
func typeSettingsHook(settings SettingsRepository, contentTypes typeFields) modulekit.ContentTypeSettings {
	imageFields := func(ctx context.Context, contentTypeID int) ([]modulekit.FieldInstanceDef, error) {
		fields, err := contentTypes.FieldsByContentTypeID(ctx, contentTypeID)
		if err != nil {
			return nil, err
		}
		var out []modulekit.FieldInstanceDef
		for _, f := range fields {
			if f.FieldType == modulekit.FieldTypeImage {
				out = append(out, f)
			}
		}
		return out, nil
	}
	return modulekit.ContentTypeSettings{
		Module: manifest.Name,
		Load: func(ctx context.Context, ct modulekit.ContentTypeDef) (any, error) {
			s, err := settings.TypeSettings(ctx, ct.MachineName)
			if err != nil {
				return nil, err
			}
			fields, err := imageFields(ctx, ct.ID)
			if err != nil {
				return nil, err
			}
			return typeSettingsForm{TypeSettings: s, ImageFields: fields}, nil
		},
		Save: func(ctx context.Context, ct modulekit.ContentTypeDef, r *http.Request) error {
			s := TypeSettings{DefaultEnabled: r.FormValue("default_galleries_enabled") == "1"}
			// Only one of the type's own image fields.
			if v := strings.TrimSpace(r.FormValue("galleries_picture_field")); v != "" {
				fields, err := imageFields(ctx, ct.ID)
				if err != nil {
					return err
				}
				for _, f := range fields {
					if f.MachineName == v {
						s.PictureField = v
					}
				}
			}
			return settings.SaveTypeSettings(ctx, ct.ID, s)
		},
	}
}

// nodeSettingsHook - the node form's "Galleries" panel: whether the node
// can have galleries.
func nodeSettingsHook(settings SettingsRepository) modulekit.NodeSettings {
	return modulekit.NodeSettings{
		Module: manifest.Name,
		// Enabled: what was submitted, the node's own switch, or for a new
		// node the type's default.
		Load: func(ctx context.Context, contentType string, node *modulekit.Node, submitted url.Values) (any, error) {
			if submitted != nil {
				return submitted.Get("galleries_enabled") == "1", nil
			}
			if node != nil {
				return settings.NodeEnabled(ctx, node.ID)
			}
			s, err := settings.TypeSettings(ctx, contentType)
			return s.DefaultEnabled, err
		},
		Save: func(ctx context.Context, node modulekit.Node, r *http.Request) error {
			return settings.SetNodeEnabled(ctx, node.ID, r.FormValue("galleries_enabled") == "1")
		},
		// A new node saved by someone the panel isn't shown to: the type's
		// default.
		Default: func(ctx context.Context, node modulekit.Node) error {
			s, err := settings.TypeSettings(ctx, string(node.Type))
			if err != nil {
				return fmt.Errorf("the type's gallery settings: %w", err)
			}
			return settings.SetNodeEnabled(ctx, node.ID, s.DefaultEnabled)
		},
	}
}

// nodeSummaries - the galleries of a node with their sizes
// (GalleryService).
type nodeSummaries interface {
	SummaryByNodeID(ctx context.Context, nodeID int) (Galleries, error)
}

// nodePageHook - a node page's galleries: the button, the links and the
// block's data, for a node that can have them. A failed load leaves that
// part out of the page and is logged (the page itself renders).
func nodePageHook(settings SettingsRepository, galleries nodeSummaries, logger *slog.Logger) func(ctx context.Context, node modulekit.Node) modulekit.NodePageParts {
	warn := func(err error, msg string, nodeID int) {
		if err != nil && logger != nil {
			logger.Warn(msg, "error", err, "nodeID", nodeID)
		}
	}
	return func(ctx context.Context, node modulekit.Node) modulekit.NodePageParts {
		on, err := settings.NodeEnabled(ctx, node.ID)
		warn(err, "galleries: the node's switch", node.ID)
		if err != nil || !on {
			return modulekit.NodePageParts{}
		}
		page := nodePage{}
		g, err := galleries.SummaryByNodeID(ctx, node.ID)
		warn(err, "galleries: the node's galleries", node.ID)
		if err == nil {
			page.Galleries = g
		}
		return modulekit.NodePageParts{Data: page}
	}
}
