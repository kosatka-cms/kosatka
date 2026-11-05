package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/userfield"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/userfieldvalue"
)

// UserFieldRepository - user_fields / user_field_values.
type UserFieldRepository struct {
	client *generated.Client
}

// NewUserFieldRepository - the profile field repository on db.
func NewUserFieldRepository(db *sql.DB) *UserFieldRepository {
	return &UserFieldRepository{client: clientFor(db)}
}

// Fields - every profile field, by weight, then ID.
func (r *UserFieldRepository) Fields(ctx context.Context) (entity.UserFieldDefs, error) {
	rows, err := GetClient(ctx, r.client).UserField.Query().
		Order(generated.Asc(userfield.FieldWeight), generated.Asc(userfield.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserFieldRepository.Fields: %w", err)
	}
	var out entity.UserFieldDefs
	for _, f := range rows {
		out = append(out, entity.UserFieldDef{ID: f.ID, MachineName: f.MachineName, Label: f.Label, MaxLen: f.MaxLen, Weight: f.Weight, Required: f.Required})
	}
	return out, nil
}

// CreateField adds the profile field f and returns its ID.
func (r *UserFieldRepository) CreateField(ctx context.Context, f entity.UserFieldDef) (int, error) {
	now := time.Now()
	row, err := GetClient(ctx, r.client).UserField.Create().
		SetMachineName(f.MachineName).SetLabel(f.Label).SetMaxLen(f.MaxLen).SetWeight(f.Weight).SetRequired(f.Required).SetCreatedAt(now).SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}

// UpdateField saves f's label, length limit, weight and whether it is
// required over the field f.ID (its machine name stays).
func (r *UserFieldRepository) UpdateField(ctx context.Context, f entity.UserFieldDef) error {
	_, err := GetClient(ctx, r.client).UserField.Update().Where(userfield.ID(f.ID)).
		SetLabel(f.Label).SetMaxLen(f.MaxLen).SetWeight(f.Weight).SetRequired(f.Required).SetUpdatedAt(time.Now()).
		Save(ctx)
	return err
}

// DeleteField - see entity.UserFieldRepository.DeleteField.
func (r *UserFieldRepository) DeleteField(ctx context.Context, id int) error {
	return inTx(ctx, r.client, func(c *generated.Client) error {
		if _, err := c.UserFieldValue.Delete().Where(userfieldvalue.FieldIDEQ(id)).Exec(ctx); err != nil {
			return fmt.Errorf("UserFieldRepository.DeleteField: %w", err)
		}
		_, err := c.UserField.Delete().Where(userfield.ID(id)).Exec(ctx)
		return err
	})
}

// Values - see entity.UserFieldRepository.Values.
func (r *UserFieldRepository) Values(ctx context.Context, userID int) (map[int]string, error) {
	rows, err := GetClient(ctx, r.client).UserFieldValue.Query().Where(userfieldvalue.UserID(userID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserFieldRepository.Values: %w", err)
	}
	out := map[int]string{}
	for _, v := range rows {
		out[v.FieldID] = v.Value
	}
	return out, nil
}

// SetValues saves the user's values: an empty one deletes the row, the
// rest are updated or created.
func (r *UserFieldRepository) SetValues(ctx context.Context, userID int, values map[int]string) error {
	return inTx(ctx, r.client, func(c *generated.Client) error {
		for fieldID, v := range values {
			own := userfieldvalue.And(userfieldvalue.UserID(userID), userfieldvalue.FieldIDEQ(fieldID))
			if strings.TrimSpace(v) == "" {
				if _, err := c.UserFieldValue.Delete().Where(own).Exec(ctx); err != nil {
					return fmt.Errorf("UserFieldRepository.SetValues: %w", err)
				}
				continue
			}
			// Asked, not taken from the UPDATE's count: MySQL without
			// clientFoundRows counts only the rows it changed.
			exists, err := c.UserFieldValue.Query().Where(own).Exist(ctx)
			if err != nil {
				return fmt.Errorf("UserFieldRepository.SetValues: %w", err)
			}
			if exists {
				err = c.UserFieldValue.Update().Where(own).SetValue(v).Exec(ctx)
			} else {
				err = c.UserFieldValue.Create().SetUserID(userID).SetFieldID(fieldID).SetValue(v).Exec(ctx)
			}
			if err != nil {
				return fmt.Errorf("UserFieldRepository.SetValues: %w", err)
			}
		}
		return nil
	})
}

// DeleteUserValues deletes every profile field value of the user.
func (r *UserFieldRepository) DeleteUserValues(ctx context.Context, userID int) error {
	_, err := GetClient(ctx, r.client).UserFieldValue.Delete().Where(userfieldvalue.UserID(userID)).Exec(ctx)
	return err
}
