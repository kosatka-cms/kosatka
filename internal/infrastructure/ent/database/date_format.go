package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/dateformat"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewfielddef"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// DateFormatRepository manages the date_formats catalog.
type DateFormatRepository struct {
	client *generated.Client
}

// NewDateFormatRepository - the date format repository on db.
func NewDateFormatRepository(db *sql.DB) *DateFormatRepository {
	return &DateFormatRepository{client: clientFor(db)}
}

func toDateFormat(f *generated.DateFormat) entity.DateFormat {
	return entity.DateFormat{ID: f.ID, MachineName: f.MachineName, Label: f.Label, Pattern: f.Pattern, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt}
}

// DateFormats - every date format, by label, then ID.
func (r *DateFormatRepository) DateFormats(ctx context.Context) ([]entity.DateFormat, error) {
	rows, err := GetClient(ctx, r.client).DateFormat.Query().
		Order(generated.Asc(dateformat.FieldLabel), generated.Asc(dateformat.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("DateFormatRepository.DateFormats: %w", err)
	}
	var out []entity.DateFormat
	for _, f := range rows {
		out = append(out, toDateFormat(f))
	}
	return out, nil
}

// DateFormatByID - the date format with id.
func (r *DateFormatRepository) DateFormatByID(ctx context.Context, id int) (*entity.DateFormat, error) {
	row, err := GetClient(ctx, r.client).DateFormat.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("DateFormatRepository.DateFormatByID: %w", err)
	}
	f := toDateFormat(row)
	return &f, nil
}

// CreateDateFormat adds the date format f and returns its ID.
func (r *DateFormatRepository) CreateDateFormat(ctx context.Context, f entity.DateFormat) (int, error) {
	now := time.Now()
	row, err := GetClient(ctx, r.client).DateFormat.Create().
		SetMachineName(f.MachineName).SetLabel(f.Label).SetPattern(f.Pattern).SetCreatedAt(now).SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}

// UpdateDateFormat updates label and pattern; the machine name never changes.
func (r *DateFormatRepository) UpdateDateFormat(ctx context.Context, f entity.DateFormat) error {
	_, err := GetClient(ctx, r.client).DateFormat.Update().Where(dateformat.ID(f.ID)).
		SetLabel(f.Label).SetPattern(f.Pattern).SetUpdatedAt(time.Now()).
		Save(ctx)
	return err
}

// DeleteDateFormat deletes the date format with id.
func (r *DateFormatRepository) DeleteDateFormat(ctx context.Context, id int) error {
	_, err := GetClient(ctx, r.client).DateFormat.Delete().Where(dateformat.ID(id)).Exec(ctx)
	return err
}

// DateFormatUsage - see entity.DateFormatRepository.DateFormatUsage.
func (r *DateFormatRepository) DateFormatUsage(ctx context.Context, machineName string) ([]string, error) {
	client := GetClient(ctx, r.client)
	instances, err := client.FieldInstanceDef.Query().
		Where(fieldinstancedef.DateFormat(machineName)).
		WithContentType().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("DateFormatRepository.DateFormatUsage: %w", err)
	}
	var out []string
	for _, i := range instances {
		out = append(out, i18n.T(ctx, "Field \"@field\" of the type \"@type\"", "@field", i.Label, "@type", i.Edges.ContentType.Label))
	}
	viewFields, err := client.ViewFieldDef.Query().
		Where(viewfielddef.DateFormat(machineName)).
		WithViewDisplay().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("DateFormatRepository.DateFormatUsage: %w", err)
	}
	for _, f := range viewFields {
		out = append(out, i18n.T(ctx, "View field \"@display\"", "@display", f.Edges.ViewDisplay.Label))
	}
	return out, nil
}

// InstanceDateFormats - see
// entity.DateFormatRepository.InstanceDateFormats.
func (r *DateFormatRepository) InstanceDateFormats(ctx context.Context) (map[string]string, error) {
	instances, err := GetClient(ctx, r.client).FieldInstanceDef.Query().
		Where(fieldinstancedef.DateFormatNEQ("")).
		WithContentType().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("DateFormatRepository.InstanceDateFormats: %w", err)
	}
	out := map[string]string{}
	for _, i := range instances {
		out[entity.InstanceDateFormatKey(i.Edges.ContentType.MachineName, i.FieldStorageID)] = i.DateFormat
	}
	return out, nil
}
