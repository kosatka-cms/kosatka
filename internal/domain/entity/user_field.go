package entity

import (
	"context"
	"sort"
)

// UserFieldDef - an admin-defined profile field: a plain text value per
// user, shown on the profile and edited in the profile form, ordered by Weight.
type UserFieldDef struct {
	ID          int
	MachineName string
	Label       string
	MaxLen      int
	Weight      int
	// Required - asked at sign-up (and by the administrator's "add
	// user", and after an OAuth sign-up) and never left empty.
	Required bool
}

// UserFieldDefs - a list of profile fields.
type UserFieldDefs []UserFieldDef

// Sorted - by Weight, then ID.
func (fs UserFieldDefs) Sorted() UserFieldDefs {
	out := append(UserFieldDefs{}, fs...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Weight != out[j].Weight {
			return out[i].Weight < out[j].Weight
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// UserFieldValue - a field with one user's value, as the profile and the
// profile form show it.
type UserFieldValue struct {
	Field UserFieldDef
	Value string
}

// UserFieldRepository - storage port for the profile fields and the
// users' values.
type UserFieldRepository interface {
	Fields(ctx context.Context) (UserFieldDefs, error)
	CreateField(ctx context.Context, f UserFieldDef) (int, error)
	UpdateField(ctx context.Context, f UserFieldDef) error
	// DeleteField removes the field and every user's value of it.
	DeleteField(ctx context.Context, id int) error
	// Values - field ID → value for one user.
	Values(ctx context.Context, userID int) (map[int]string, error)
	// SetValues stores the user's values: an empty value removes it.
	SetValues(ctx context.Context, userID int, values map[int]string) error
	DeleteUserValues(ctx context.Context, userID int) error
}
