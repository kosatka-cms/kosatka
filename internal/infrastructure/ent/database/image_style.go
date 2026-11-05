package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/imagestyle"
)

// NewImageStyleRepository - the image style repository on client.
func NewImageStyleRepository(client *generated.Client) *ImageStyleRepository {
	return &ImageStyleRepository{client: client}
}

// ImageStyleRepository is the ent adapter for entity.ImageStyleRepository.
type ImageStyleRepository struct {
	client *generated.Client
}

// ImageStyles - every image style, by name.
func (r ImageStyleRepository) ImageStyles(ctx context.Context) (entity.ImageStyleDefs, error) {
	client := GetClient(ctx, r.client)
	res, err := client.ImageStyle.Query().
		Order(generated.Asc(imagestyle.FieldName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ImageStyleRepository.ImageStyles: %w", err)
	}

	styles := make(entity.ImageStyleDefs, len(res))
	for i, v := range res {
		styles[i] = dao.ToImageStyleDefEntity(v)
	}
	return styles, nil
}

// ImageStyleByID - the image style with id; ErrEntityNotFound if there is
// none.
func (r ImageStyleRepository) ImageStyleByID(ctx context.Context, id int) (*entity.ImageStyleDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.ImageStyle.Get(ctx, id)
	return one(res, err, dao.ToImageStyleDefEntity, "image style with id %d", id)
}

// ImageStyleByName - the image style named name; ErrEntityNotFound if
// there is none.
func (r ImageStyleRepository) ImageStyleByName(ctx context.Context, name string) (*entity.ImageStyleDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.ImageStyle.Query().Where(imagestyle.Name(name)).Only(ctx)
	return one(res, err, dao.ToImageStyleDefEntity, "image style with name %q", name)
}

// CreateImageStyle adds the image style s and returns it with its ID.
func (r ImageStyleRepository) CreateImageStyle(ctx context.Context, s entity.ImageStyleDef) (*entity.ImageStyleDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	v, err := client.ImageStyle.Create().
		SetName(s.Name).
		SetLabel(s.Label).
		SetDescription(s.Description).
		SetWidth(s.Width).
		SetHeight(s.Height).
		SetMode(s.Mode).
		SetAnchor(s.Anchor).
		SetQuality(s.Quality).
		SetFormat(entity.NormalizeImageStyleFormat(s.Format)).
		SetHidpi(s.HiDPI).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	created := dao.ToImageStyleDefEntity(v)
	return &created, nil
}

// UpdateImageStyle never touches Name: URLs and Go call sites reference a
// style by name, so renaming would silently break them.
func (r ImageStyleRepository) UpdateImageStyle(ctx context.Context, s entity.ImageStyleDef) (*entity.ImageStyleDef, error) {
	client := GetClient(ctx, r.client)
	v, err := client.ImageStyle.UpdateOneID(s.ID).
		SetLabel(s.Label).
		SetDescription(s.Description).
		SetWidth(s.Width).
		SetHeight(s.Height).
		SetMode(s.Mode).
		SetAnchor(s.Anchor).
		SetQuality(s.Quality).
		SetFormat(entity.NormalizeImageStyleFormat(s.Format)).
		SetHidpi(s.HiDPI).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, notFoundAs(err, "image style with id %d", s.ID)
	}

	updated := dao.ToImageStyleDefEntity(v)
	return &updated, nil
}

// DeleteImageStyle deletes the image style with id; ErrEntityNotFound if
// there is none.
func (r ImageStyleRepository) DeleteImageStyle(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	return notFoundAs(client.ImageStyle.DeleteOneID(id).Exec(ctx), "image style with id %d", id)
}
