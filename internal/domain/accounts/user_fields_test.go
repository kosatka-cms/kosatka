package accounts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeUserFieldRepo struct {
	fields entity.UserFieldDefs
	values map[int]map[int]string // user → field → value
	nextID int
}

func (f *fakeUserFieldRepo) Fields(context.Context) (entity.UserFieldDefs, error) {
	return f.fields, nil
}

func (f *fakeUserFieldRepo) CreateField(_ context.Context, d entity.UserFieldDef) (int, error) {
	f.nextID++
	d.ID = f.nextID
	f.fields = append(f.fields, d)
	return d.ID, nil
}

func (f *fakeUserFieldRepo) UpdateField(_ context.Context, d entity.UserFieldDef) error {
	for i := range f.fields {
		if f.fields[i].ID == d.ID {
			f.fields[i].Label, f.fields[i].MaxLen, f.fields[i].Weight, f.fields[i].Required = d.Label, d.MaxLen, d.Weight, d.Required
		}
	}
	return nil
}

func (f *fakeUserFieldRepo) DeleteField(_ context.Context, id int) error {
	var kept entity.UserFieldDefs
	for _, d := range f.fields {
		if d.ID != id {
			kept = append(kept, d)
		}
	}
	f.fields = kept
	for _, vals := range f.values {
		delete(vals, id)
	}
	return nil
}

func (f *fakeUserFieldRepo) Values(_ context.Context, userID int) (map[int]string, error) {
	return f.values[userID], nil
}

func (f *fakeUserFieldRepo) SetValues(_ context.Context, userID int, values map[int]string) error {
	if f.values == nil {
		f.values = map[int]map[int]string{}
	}
	if f.values[userID] == nil {
		f.values[userID] = map[int]string{}
	}
	for id, v := range values {
		if v == "" {
			delete(f.values[userID], id)
			continue
		}
		f.values[userID][id] = v
	}
	return nil
}

func (f *fakeUserFieldRepo) DeleteUserValues(_ context.Context, userID int) error {
	delete(f.values, userID)
	return nil
}

func TestUserFieldCreateValidation(t *testing.T) {
	ctx := context.Background()
	s := NewUserFieldService(&fakeUserFieldRepo{})
	cases := []struct {
		name, machine, label string
		maxLen               int
		want                 error
	}{
		{"bad machine name", "Book", "Book", 0, ErrUserFieldName},
		{"empty label", "book", " ", 0, ErrUserFieldLabel},
		{"max len too big", "book", "Book", 70000, ErrUserFieldMaxLen},
		{"ok", "book", "Book", 0, nil},
		{"duplicate", "book", "Book 2", 0, ErrUserFieldExists},
	}
	for _, tc := range cases {
		if err := s.CreateField(ctx, tc.machine, tc.label, tc.maxLen, false); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
	fields, _ := s.Fields(ctx)
	if len(fields) != 1 || fields[0].MaxLen != 255 {
		t.Fatalf("default max len 255 expected: %+v", fields)
	}
}

func TestUserFieldOrderAndValues(t *testing.T) {
	ctx := context.Background()
	repo := &fakeUserFieldRepo{}
	s := NewUserFieldService(repo)
	_ = s.CreateField(ctx, "first", "First", 0, false)
	_ = s.CreateField(ctx, "second", "Second", 5, false)
	fields, _ := s.Fields(ctx)
	if fields[0].MachineName != "first" || fields[1].Weight <= fields[0].Weight {
		t.Fatalf("new field must go after existing ones: %+v", fields)
	}
	first, second := fields[0], fields[1]

	// Reorder: second goes first.
	if err := s.UpdateField(ctx, second.ID, "Second", 5, -1, false); err != nil {
		t.Fatal(err)
	}

	form := map[string]string{UserFieldFormName(first.ID): "  value  ", UserFieldFormName(second.ID): ""}
	values, err := s.FormValues(ctx, func(n string) string { return form[n] })
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveValues(ctx, 7, values); err != nil {
		t.Fatal(err)
	}
	got, _ := s.UserValues(ctx, 7)
	if len(got) != 2 || got[0].Field.ID != second.ID || got[0].Value != "" || got[1].Value != "value" {
		t.Fatalf("UserValues = %+v", got)
	}

	// Too long for "second" (max 5).
	form[UserFieldFormName(second.ID)] = strings.Repeat("x", 6)
	_, err = s.FormValues(ctx, func(n string) string { return form[n] })
	var tooLong UserFieldTooLongError
	if !errors.As(err, &tooLong) || tooLong.Field.ID != second.ID {
		t.Fatalf("expected too-long error for second, got %v", err)
	}

	// Clearing a value removes it.
	_ = s.SaveValues(ctx, 7, map[int]string{first.ID: ""})
	if v := repo.values[7][first.ID]; v != "" {
		t.Fatalf("cleared value must be removed, got %q", v)
	}
}

// TestUserFieldRequired - a required field: an empty value is refused
// (UserFieldRequiredError, a UserFieldFormError); the forms creating a
// user read only the required fields; a user without a value of one is
// missing it (an OAuth sign-up completes the profile: sociallogin).
func TestUserFieldRequired(t *testing.T) {
	ctx := context.Background()
	s := NewUserFieldService(&fakeUserFieldRepo{})
	_ = s.CreateField(ctx, "city", "City", 0, true)
	_ = s.CreateField(ctx, "site", "Site", 0, false)
	fields, _ := s.Fields(ctx)
	city, site := fields[0], fields[1]

	if req, err := s.RequiredFields(ctx); err != nil || len(req) != 1 || req[0].ID != city.ID {
		t.Fatalf("required fields: %+v %v", req, err)
	}
	empty := func(string) string { return "  " }
	var invalid UserFieldFormError
	if _, err := s.FormValues(ctx, empty); !errors.As(err, &invalid) || !errors.As(err, new(UserFieldRequiredError)) {
		t.Errorf("an empty required field: %v", err)
	}
	form := map[string]string{UserFieldFormName(city.ID): " Paris ", UserFieldFormName(site.ID): "x"}
	values, err := s.RequiredFormValues(ctx, func(n string) string { return form[n] })
	if err != nil || len(values) != 1 || values[city.ID] != "Paris" {
		t.Errorf("required form values: %v %v", values, err)
	}

	if missing, err := s.MissingRequired(ctx, 7); err != nil || !missing {
		t.Errorf("a user without the required value: %v %v", missing, err)
	}
	_ = s.SaveValues(ctx, 7, values)
	if missing, err := s.MissingRequired(ctx, 7); err != nil || missing {
		t.Errorf("a user with it: %v %v", missing, err)
	}
}
