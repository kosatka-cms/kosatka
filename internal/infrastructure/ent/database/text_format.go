package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/textformatdef"
)

// NewTextFormatRepository - the text format repository on client.
func NewTextFormatRepository(client *generated.Client) *TextFormatRepository {
	return &TextFormatRepository{client: client}
}

// TextFormatRepository is the ent adapter for entity.TextFormatRepository.
type TextFormatRepository struct {
	client *generated.Client
}

// TextFormats - every text format, by weight.
func (r TextFormatRepository) TextFormats(ctx context.Context) (entity.TextFormatDefs, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TextFormatDef.Query().
		Order(generated.Asc(textformatdef.FieldWeight)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("TextFormatRepository.TextFormats: %w", err)
	}

	formats := make(entity.TextFormatDefs, len(res))
	for i, v := range res {
		formats[i] = dao.ToTextFormatDefEntity(v)
	}
	return formats, nil
}

// TextFormatByID - the text format with id; ErrEntityNotFound if there is
// none.
func (r TextFormatRepository) TextFormatByID(ctx context.Context, id int) (*entity.TextFormatDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TextFormatDef.Get(ctx, id)
	return one(res, err, dao.ToTextFormatDefEntity, "text format with id %d", id)
}

// TextFormatByMachineName - the text format named machineName;
// ErrEntityNotFound if there is none.
func (r TextFormatRepository) TextFormatByMachineName(ctx context.Context, machineName string) (*entity.TextFormatDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TextFormatDef.Query().Where(textformatdef.MachineName(machineName)).Only(ctx)
	return one(res, err, dao.ToTextFormatDefEntity, "text format with machine_name %q", machineName)
}

// CreateTextFormat adds the text format f and returns it with its ID.
func (r TextFormatRepository) CreateTextFormat(ctx context.Context, f entity.TextFormatDef) (*entity.TextFormatDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	v, err := client.TextFormatDef.Create().
		SetMachineName(f.MachineName).
		SetLabel(f.Label).
		SetWeight(f.Weight).
		SetAllowedTags(f.AllowedTags).
		SetAllowClassAttribute(f.AllowClassAttribute).
		SetHasRichEditor(f.HasRichEditor).
		SetTinymcePlugins(f.TinyMCEPlugins).
		SetTinymceToolbar(f.TinyMCEToolbar).
		SetAllowedExtensions(f.AllowedExtensions).
		SetMaxFileSizeBytes(f.MaxFileSizeBytes).
		SetStatus(f.Status).
		SetFilters(f.Filters).
		SetCreatedBy(f.CreatedBy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	created := dao.ToTextFormatDefEntity(v)
	return &created, nil
}

// UpdateTextFormat never touches MachineName: like other machine names, it
// is an immutable identity string.
func (r TextFormatRepository) UpdateTextFormat(ctx context.Context, f entity.TextFormatDef) (*entity.TextFormatDef, error) {
	client := GetClient(ctx, r.client)
	v, err := client.TextFormatDef.UpdateOneID(f.ID).
		SetLabel(f.Label).
		SetWeight(f.Weight).
		SetAllowedTags(f.AllowedTags).
		SetAllowClassAttribute(f.AllowClassAttribute).
		SetHasRichEditor(f.HasRichEditor).
		SetTinymcePlugins(f.TinyMCEPlugins).
		SetTinymceToolbar(f.TinyMCEToolbar).
		SetAllowedExtensions(f.AllowedExtensions).
		SetMaxFileSizeBytes(f.MaxFileSizeBytes).
		SetStatus(f.Status).
		SetFilters(f.Filters).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, notFoundAs(err, "text format with id %d", f.ID)
	}

	updated := dao.ToTextFormatDefEntity(v)
	return &updated, nil
}

// DeleteTextFormat doesn't guard against formats still referenced by
// field_value_text.text_format_id: that content was sanitized at save time
// and still renders; the edit form just falls back to the field's
// lowest-weight allowed format.
func (r TextFormatRepository) DeleteTextFormat(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	return notFoundAs(client.TextFormatDef.DeleteOneID(id).Exec(ctx), "text format with id %d", id)
}
