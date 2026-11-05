package accounts

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// User field management errors.
var (
	ErrUserFieldName   = errors.New("user field machine name: lowercase latin letters, digits and _, starting with a letter")
	ErrUserFieldLabel  = errors.New("user field label is required")
	ErrUserFieldExists = errors.New("a user field with this machine name exists")
	ErrUserFieldMaxLen = errors.New("user field max length must be 1…65535")
)

// UserFieldTooLongError - a submitted value longer than the field allows.
type UserFieldTooLongError struct {
	Field entity.UserFieldDef
}

func (e UserFieldTooLongError) Error() string {
	return fmt.Sprintf("%q: at most %d characters", e.Field.Label, e.Field.MaxLen)
}

// Message - the error for the user in ctx's language.
func (e UserFieldTooLongError) Message(ctx context.Context) string {
	return i18n.Plural(ctx, e.Field.MaxLen, "\"@field\": at most @count character", "\"@field\": at most @count characters", "@field", e.Field.Label)
}

// UserFieldRequiredError - a required field submitted empty.
type UserFieldRequiredError struct {
	Field entity.UserFieldDef
}

func (e UserFieldRequiredError) Error() string {
	return fmt.Sprintf("%q is required", e.Field.Label)
}

// Message - the error for the user in ctx's language.
func (e UserFieldRequiredError) Message(ctx context.Context) string {
	return i18n.T(ctx, "\"@field\" is required", "@field", e.Field.Label)
}

// UserFieldFormError - a submitted value the user has to correct
// (UserFieldTooLongError, UserFieldRequiredError); Message says what, in
// the user's language.
type UserFieldFormError interface {
	error
	Message(ctx context.Context) string
}

var userFieldNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

const userFieldDefaultMaxLen = 255

// UserFieldService - the admin-defined profile fields and users' values.
type UserFieldService struct {
	repo entity.UserFieldRepository
}

// NewUserFieldService - the profile fields' service on repo.
func NewUserFieldService(repo entity.UserFieldRepository) *UserFieldService {
	return &UserFieldService{repo: repo}
}

// Fields - every field, by weight.
func (s *UserFieldService) Fields(ctx context.Context) (entity.UserFieldDefs, error) {
	fields, err := s.repo.Fields(ctx)
	if err != nil {
		return nil, err
	}
	return fields.Sorted(), nil
}

// CreateField adds a field after the existing ones (maxLen ≤ 0 - 255).
func (s *UserFieldService) CreateField(ctx context.Context, machineName, label string, maxLen int, required bool) error {
	machineName, label = strings.TrimSpace(machineName), strings.TrimSpace(label)
	if !userFieldNameRe.MatchString(machineName) {
		return ErrUserFieldName
	}
	if label == "" {
		return ErrUserFieldLabel
	}
	if maxLen <= 0 {
		maxLen = userFieldDefaultMaxLen
	}
	if maxLen > 65535 {
		return ErrUserFieldMaxLen
	}
	fields, err := s.repo.Fields(ctx)
	if err != nil {
		return err
	}
	weight := 0
	for _, f := range fields {
		if f.MachineName == machineName {
			return ErrUserFieldExists
		}
		if f.Weight >= weight {
			weight = f.Weight + 10
		}
	}
	_, err = s.repo.CreateField(ctx, entity.UserFieldDef{MachineName: machineName, Label: label, MaxLen: maxLen, Weight: weight, Required: required})
	return err
}

// UpdateField changes a field's label, max length, weight and whether it
// is required (the machine name stays).
func (s *UserFieldService) UpdateField(ctx context.Context, id int, label string, maxLen, weight int, required bool) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return ErrUserFieldLabel
	}
	if maxLen <= 0 || maxLen > 65535 {
		return ErrUserFieldMaxLen
	}
	return s.repo.UpdateField(ctx, entity.UserFieldDef{ID: id, Label: label, MaxLen: maxLen, Weight: weight, Required: required})
}

// DeleteField removes the field and every user's value of it.
func (s *UserFieldService) DeleteField(ctx context.Context, id int) error {
	return s.repo.DeleteField(ctx, id)
}

// UserValues - every field with the user's value (empty when unset), by
// weight.
func (s *UserFieldService) UserValues(ctx context.Context, userID int) ([]entity.UserFieldValue, error) {
	fields, err := s.Fields(ctx)
	if err != nil || len(fields) == 0 {
		return nil, err
	}
	values, err := s.repo.Values(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]entity.UserFieldValue, len(fields))
	for i, f := range fields {
		out[i] = entity.UserFieldValue{Field: f, Value: values[f.ID]}
	}
	return out, nil
}

// RequiredFields - the fields asked at sign-up, by weight.
func (s *UserFieldService) RequiredFields(ctx context.Context) (entity.UserFieldDefs, error) {
	fields, err := s.Fields(ctx)
	if err != nil {
		return nil, err
	}
	var out entity.UserFieldDefs
	for _, f := range fields {
		if f.Required {
			out = append(out, f)
		}
	}
	return out, nil
}

// FormValues reads every field's submitted value via get (the profile
// form's "user_field_<id>"), trimmed and checked (a UserFieldFormError).
func (s *UserFieldService) FormValues(ctx context.Context, get func(name string) string) (map[int]string, error) {
	fields, err := s.Fields(ctx)
	if err != nil {
		return nil, err
	}
	return formValues(fields, get)
}

// RequiredFormValues - FormValues of the required fields only (the forms
// that create a user ask those).
func (s *UserFieldService) RequiredFormValues(ctx context.Context, get func(name string) string) (map[int]string, error) {
	fields, err := s.RequiredFields(ctx)
	if err != nil {
		return nil, err
	}
	return formValues(fields, get)
}

func formValues(fields entity.UserFieldDefs, get func(name string) string) (map[int]string, error) {
	out := make(map[int]string, len(fields))
	for _, f := range fields {
		v := strings.TrimSpace(get(UserFieldFormName(f.ID)))
		if f.Required && v == "" {
			return nil, UserFieldRequiredError{Field: f}
		}
		if f.MaxLen > 0 && utf8.RuneCountInString(v) > f.MaxLen {
			return nil, UserFieldTooLongError{Field: f}
		}
		out[f.ID] = v
	}
	return out, nil
}

// MissingRequired - the user has left a required field empty (a user
// created before the field was, or by an OAuth sign-up, which asks
// nothing): they complete their profile.
func (s *UserFieldService) MissingRequired(ctx context.Context, userID int) (bool, error) {
	fields, err := s.RequiredFields(ctx)
	if err != nil || len(fields) == 0 {
		return false, err
	}
	values, err := s.repo.Values(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, f := range fields {
		if strings.TrimSpace(values[f.ID]) == "" {
			return true, nil
		}
	}
	return false, nil
}

// SaveValues stores values from FormValues for the user.
func (s *UserFieldService) SaveValues(ctx context.Context, userID int, values map[int]string) error {
	if len(values) == 0 {
		return nil
	}
	return s.repo.SetValues(ctx, userID, values)
}

// UserFieldFormName - the profile form input name of a user field.
func UserFieldFormName(fieldID int) string {
	return fmt.Sprintf("user_field_%d", fieldID)
}
