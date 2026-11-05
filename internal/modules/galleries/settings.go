package galleries

import (
	"context"
	"database/sql"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated/gallerycontenttype"
	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated/gallerynode"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// TypeSettings - the galleries settings of a content type
// (gallery_content_types).
type TypeSettings struct {
	// DefaultEnabled - whether a new node of the type can have galleries
	// (the node's own switch starts from it).
	DefaultEnabled bool
	// PictureField - the type's image field the "Main content image"
	// button of gallery management writes into ("" - no button).
	PictureField string
}

// DefaultTypeSettings - the settings of a type without a row.
func DefaultTypeSettings() TypeSettings { return TypeSettings{} }

// SettingsRepository - the module's settings of content types and nodes.
type SettingsRepository interface {
	// TypeSettings - the type's settings (DefaultTypeSettings without a
	// row).
	TypeSettings(ctx context.Context, contentType string) (TypeSettings, error)
	SaveTypeSettings(ctx context.Context, contentTypeID int, s TypeSettings) error
	// NodeEnabled - whether the node can have galleries.
	NodeEnabled(ctx context.Context, nodeID int) (bool, error)
	// EnabledNodeIDs - the nodes that can have galleries.
	EnabledNodeIDs(ctx context.Context) ([]int, error)
	SetNodeEnabled(ctx context.Context, nodeID int, enabled bool) error
}

// ContentTypeIDs - a content type's ID by its machine name (the core's
// content types).
type ContentTypeIDs interface {
	ContentTypeByMachineName(ctx context.Context, machineName string) (*modulekit.ContentTypeDef, error)
}

// EntSettingsRepository - SettingsRepository on the module's own ent
// schema (ent/).
type EntSettingsRepository struct {
	db    *sql.DB
	types ContentTypeIDs
}

// NewSettingsRepository creates the settings repository on db; the
// content types come from the core.
func NewSettingsRepository(db *sql.DB, types ContentTypeIDs) *EntSettingsRepository {
	return &EntSettingsRepository{db: db, types: types}
}

// client - the module's client for ctx: in its transaction, if any.
func (r *EntSettingsRepository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// TypeSettings - see SettingsRepository.TypeSettings.
func (r *EntSettingsRepository) TypeSettings(ctx context.Context, contentType string) (TypeSettings, error) {
	ct, err := r.types.ContentTypeByMachineName(ctx, contentType)
	if modulekit.IsNotFound(err) {
		return DefaultTypeSettings(), nil
	}
	if err != nil {
		return TypeSettings{}, err
	}
	row, err := r.client(ctx).GalleryContentType.Query().
		Where(gallerycontenttype.ContentTypeID(ct.ID)).
		Only(ctx)
	if generated.IsNotFound(err) {
		return DefaultTypeSettings(), nil
	}
	if err != nil {
		return TypeSettings{}, err
	}
	return TypeSettings{DefaultEnabled: row.DefaultEnabled, PictureField: row.PictureField}, nil
}

// SaveTypeSettings writes the content type's galleries settings (its
// row, created if missing).
func (r *EntSettingsRepository) SaveTypeSettings(ctx context.Context, contentTypeID int, s TypeSettings) error {
	client := r.client(ctx)
	// Whether the row exists is asked, not taken from the UPDATE's count:
	// MySQL without clientFoundRows counts only the rows it changed.
	exists, err := client.GalleryContentType.Query().Where(gallerycontenttype.ContentTypeID(contentTypeID)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return client.GalleryContentType.Update().
			Where(gallerycontenttype.ContentTypeID(contentTypeID)).
			SetDefaultEnabled(s.DefaultEnabled).
			SetPictureField(s.PictureField).
			Exec(ctx)
	}
	return client.GalleryContentType.Create().
		SetContentTypeID(contentTypeID).
		SetDefaultEnabled(s.DefaultEnabled).
		SetPictureField(s.PictureField).
		Exec(ctx)
}

// NodeEnabled - see SettingsRepository.NodeEnabled.
func (r *EntSettingsRepository) NodeEnabled(ctx context.Context, nodeID int) (bool, error) {
	return r.client(ctx).GalleryNode.Query().Where(gallerynode.Nid(nodeID)).Exist(ctx)
}

// EnabledNodeIDs - see SettingsRepository.EnabledNodeIDs.
func (r *EntSettingsRepository) EnabledNodeIDs(ctx context.Context) ([]int, error) {
	return r.client(ctx).GalleryNode.Query().Select(gallerynode.FieldNid).Ints(ctx)
}

// SetNodeEnabled turns the node's galleries on or off (whether it can
// have galleries).
func (r *EntSettingsRepository) SetNodeEnabled(ctx context.Context, nodeID int, enabled bool) error {
	client := r.client(ctx)
	if !enabled {
		_, err := client.GalleryNode.Delete().Where(gallerynode.Nid(nodeID)).Exec(ctx)
		return err
	}
	on, err := r.NodeEnabled(ctx, nodeID)
	if err != nil || on {
		return err
	}
	return client.GalleryNode.Create().SetNid(nodeID).Exec(ctx)
}
