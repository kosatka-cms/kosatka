package entity

import (
	"reflect"
	"slices"
	"testing"
)

// viewDisplayOwnFields - ViewDisplayDef's fields outside the Settings
// bundle: identity, attachment, structure, publishing, bookkeeping. A
// new field goes here or into ViewSnapshotSettings (inherited, copied,
// published) - TestViewSettingsComplete fails until it is one of them.
var viewDisplayOwnFields = []string{
	"ID", "BaseEntity", "ViewID", "IsDefault", "OverrideSettings", "OverrideFilters", "OverrideSort", "OverrideFields",
	"MachineName", "URLPath", "DisplayType", "AttachContentTypes", "AttachWeight", "AttachTitle", "AttachShowTitle",
	"TermVocabularies", "ProfileTabPath", "ProfileTabAccess", "ThemeTemplate",
	"PublishedSnapshot", "PublishedAt", "Status", "CreatedBy", "CreatedAt", "UpdatedAt",
}

// filled - v with every field set to a non-zero value of its type.
func filled(t *testing.T, v reflect.Value) {
	t.Helper()
	for i := range v.NumField() {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString("x" + v.Type().Field(i).Name)
		case reflect.Int:
			f.SetInt(int64(i + 1))
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Pointer:
			p := reflect.New(f.Type().Elem())
			if p.Elem().Kind() == reflect.Int {
				p.Elem().SetInt(int64(i + 1))
			}
			f.Set(p)
		case reflect.Slice:
			f.Set(reflect.MakeSlice(f.Type(), 1, 1))
		case reflect.Struct:
			// time.Time and the like: a non-zero value isn't needed.
		default:
			t.Fatalf("field %s: kind %s not filled", v.Type().Field(i).Name, f.Kind())
		}
	}
}

// TestViewSettingsComplete - every ViewDisplayDef field is either the
// display's own or in the Settings bundle (ViewSnapshotSettings, same
// name and type); SnapshotSettingsOf and ApplyTo carry each bundle field,
// WithSettingsOf all of them but Label/meta, and none of the own ones.
func TestViewSettingsComplete(t *testing.T) {
	display := reflect.TypeFor[ViewDisplayDef]()
	snapshot := reflect.TypeFor[ViewSnapshotSettings]()
	var bundle []string
	for i := range snapshot.NumField() {
		f := snapshot.Field(i)
		df, ok := display.FieldByName(f.Name)
		if !ok || df.Type != f.Type {
			t.Errorf("ViewSnapshotSettings.%s: no such ViewDisplayDef field of type %s", f.Name, f.Type)
		}
		bundle = append(bundle, f.Name)
	}
	for i := range display.NumField() {
		if name := display.Field(i).Name; !slices.Contains(bundle, name) && !slices.Contains(viewDisplayOwnFields, name) {
			t.Errorf("ViewDisplayDef.%s: neither the display's own nor in the Settings bundle", name)
		}
	}

	var from ViewDisplayDef
	filled(t, reflect.ValueOf(&from).Elem())
	applied := reflect.ValueOf(SnapshotSettingsOf(from).ApplyTo(ViewDisplayDef{}))
	copied := reflect.ValueOf(ViewDisplayDef{}.WithSettingsOf(from))
	src := reflect.ValueOf(from)
	for i := range display.NumField() {
		name := display.Field(i).Name
		inBundle := slices.Contains(bundle, name)
		own := name == "Label" || name == "MetaTitle" || name == "MetaDescription"
		if got := !applied.Field(i).IsZero(); got != inBundle {
			t.Errorf("snapshot of %s: carried %v, want %v", name, got, inBundle)
		} else if inBundle && !reflect.DeepEqual(applied.Field(i).Interface(), src.Field(i).Interface()) {
			t.Errorf("snapshot of %s: a different value", name)
		}
		if got, want := !copied.Field(i).IsZero(), inBundle && !own; got != want {
			t.Errorf("WithSettingsOf %s: copied %v, want %v", name, got, want)
		}
	}
}
