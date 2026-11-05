package service

import (
	"context"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

type fakeDateFormatRepo struct {
	formats   []entity.DateFormat
	usage     map[string][]string
	instances map[string]string
	loads     int
}

func (r *fakeDateFormatRepo) DateFormats(context.Context) ([]entity.DateFormat, error) {
	r.loads++
	return r.formats, nil
}
func (r *fakeDateFormatRepo) DateFormatByID(_ context.Context, id int) (*entity.DateFormat, error) {
	for _, f := range r.formats {
		if f.ID == id {
			return &f, nil
		}
	}
	return nil, errors.New("not found")
}
func (r *fakeDateFormatRepo) CreateDateFormat(_ context.Context, f entity.DateFormat) (int, error) {
	f.ID = len(r.formats) + 1
	r.formats = append(r.formats, f)
	return f.ID, nil
}
func (r *fakeDateFormatRepo) UpdateDateFormat(_ context.Context, f entity.DateFormat) error {
	for i := range r.formats {
		if r.formats[i].ID == f.ID {
			r.formats[i].Label, r.formats[i].Pattern = f.Label, f.Pattern
		}
	}
	return nil
}
func (r *fakeDateFormatRepo) DeleteDateFormat(_ context.Context, id int) error {
	for i := range r.formats {
		if r.formats[i].ID == id {
			r.formats = append(r.formats[:i], r.formats[i+1:]...)
			return nil
		}
	}
	return nil
}
func (r *fakeDateFormatRepo) DateFormatUsage(_ context.Context, name string) ([]string, error) {
	return r.usage[name], nil
}
func (r *fakeDateFormatRepo) InstanceDateFormats(context.Context) (map[string]string, error) {
	return r.instances, nil
}

func newTestDateFormats() (*DateFormatService, *fakeDateFormatRepo) {
	repo := &fakeDateFormatRepo{
		formats: []entity.DateFormat{
			{ID: 1, MachineName: "short", Label: "Short", Pattern: "d.m.Y"},
			{ID: 2, MachineName: "medium", Label: "Medium", Pattern: "j F Y"},
		},
		usage:     map[string][]string{"medium": {"Premiere field of the Movies type"}},
		instances: map[string]string{entity.InstanceDateFormatKey("movies", 7): "medium"},
	}
	return NewDateFormatService(repo, slog.New(slog.NewTextHandler(io.Discard, nil))), repo
}

// TestDateFormatService - patterns by name with the default for an
// empty or unknown name, cached until a save; validation of new
// formats; the default and used formats can't be deleted; a field's
// own format by content type.
func TestDateFormatService(t *testing.T) {
	ctx := t.Context()
	s, repo := newTestDateFormats()
	if s.Pattern(ctx, "medium") != "j F Y" || s.Pattern(ctx, "") != "d.m.Y" || s.Pattern(ctx, "nope") != "d.m.Y" {
		t.Fatal("Pattern")
	}
	if repo.loads != 1 {
		t.Fatalf("patterns must be cached, loads = %d", repo.loads)
	}
	for f, want := range map[entity.DateFormat]error{
		{Label: "", Pattern: "Y", MachineName: "y"}:          ErrDateFormatLabel,
		{Label: "Year", Pattern: "", MachineName: "y"}:       ErrDateFormatPattern,
		{Label: "Year", Pattern: "Y", MachineName: "Year"}:   ErrDateFormatMachineName,
		{Label: "Year", Pattern: "Y", MachineName: "medium"}: ErrDateFormatTaken,
		{Label: "Year", Pattern: " Y ", MachineName: "year"}: nil,
	} {
		if _, err := s.Save(ctx, f); !errors.Is(err, want) {
			t.Errorf("Save(%+v) = %v, want %v", f, err, want)
		}
	}
	if s.Pattern(ctx, "year") != "Y" {
		t.Fatal("a saved format must be visible (cache reset)")
	}
	if _, err := s.Delete(ctx, 1); !errors.Is(err, ErrDateFormatDefault) {
		t.Fatalf("delete default: %v", err)
	}
	if usage, err := s.Delete(ctx, 2); !errors.Is(err, ErrDateFormatInUse) || len(usage) != 1 {
		t.Fatalf("delete used: %v %v", usage, err)
	}
	if _, err := s.Delete(ctx, 3); err != nil || s.Pattern(ctx, "year") != "d.m.Y" {
		t.Fatalf("delete unused: %v", err)
	}
	if s.InstanceFormat(ctx, "movies", 7) != "medium" || s.InstanceFormat(ctx, "tv", 7) != "" {
		t.Fatal("InstanceFormat")
	}
}

// TestFieldDateRendering - fieldDate gives the default template the
// field's format, and a theme's field_date_<field> the same; a View row takes the
// Views field's format, else the format of the field on the row's type.
func TestFieldDateRendering(t *testing.T) {
	zzChdirToProjectRoot(t)
	formats, _ := newTestDateFormats()
	render := func(t *testing.T, tpl string, data any) string {
		t.Helper()
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}, dateFormats: formats}
		files, err := paths.GlobTemplates(filepath.Join("fields", "*.html"))
		if err != nil || len(files) == 0 {
			t.Fatalf("field templates: %v %v", files, err)
		}
		set, err := svc.getOrParse("fielddate", nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		page, err := set.New("p").Parse(tpl)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		if err := page.Execute(&b, data); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	d := time.Date(2027, 3, 5, 0, 0, 0, 0, time.UTC)
	if got := render(t, `{{renderSuggestion "field_date" "x" (fieldDate .V "medium")}}`, map[string]any{"V": d}); got != "5 March 2027" {
		t.Fatalf("engine, medium: %q", got)
	}
	if got := render(t, `{{renderSuggestion "field_date" "x" (fieldDate .V "")}}`, map[string]any{"V": d}); got != "05.03.2027" {
		t.Fatalf("engine, default: %q", got)
	}

	zzUseFixtureTheme(t)
	if got := render(t, `{{renderSuggestion "field_date" "release" (fieldDate .V "medium")}}`, map[string]any{"V": d}); got != `<time class="release">5 March 2027</time>` {
		t.Fatalf("theme's field template: %q", got)
	}
	if got := render(t, `{{renderSuggestion "field_date" "x" (fieldDate .V "medium")}}`, map[string]any{"V": d}); got != "5 March 2027" {
		t.Fatalf("theme, another field: %q", got)
	}

	field := entity.FieldInstanceDef{FieldStorageID: 7, Config: map[string]any{"date_format": ""}}
	row := views.ContentListItem{Node: entity.Node{Type: "movies"}}
	if got := render(t, `{{viewDateFormat .F .Item}}`, map[string]any{"F": field, "Item": row}); got != "medium" {
		t.Fatalf("instance format: %q", got)
	}
	field.Config["date_format"] = "short"
	if got := render(t, `{{viewDateFormat .F .Item}}`, map[string]any{"F": field, "Item": row}); got != "short" {
		t.Fatalf("views field format: %q", got)
	}
}

// TestDateFormatAdminTemplates - the date formats list and form and a date
// field's output format setting render with their data.
func TestDateFormatAdminTemplates(t *testing.T) {
	zzChdirToProjectRoot(t)
	content := func(t *testing.T, path string, data map[string]any) string {
		t.Helper()
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files, err := svc.pageFilesToParse(path, strings.Split(path, "/"))
		if err != nil {
			t.Fatal(err)
		}
		set, err := svc.getOrParse(path, nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		if err := set.ExecuteTemplate(&b, "content", data); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return b.String()
	}
	formats := []entity.DateFormat{{ID: 1, MachineName: "short", Label: "Short", Pattern: "d.m.Y"}, {ID: 2, MachineName: "medium", Label: "Medium", Pattern: "j F Y"}}

	out := content(t, "admin/date-formats/index.html", map[string]any{
		"formats": []struct {
			entity.DateFormat
			Example string
		}{{formats[0], "05.03.2027"}, {formats[1], "5 March 2027"}},
		"defaultFormat": "short",
		"error":         "in-use",
		"usage":         "Premiere field of the Movies type",
	})
	for _, want := range []string{"5 March 2027", `admin-pill--ok">default</span>`, "/admin/config/date-formats/2/delete", "Premiere field of the Movies type"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/admin/config/date-formats/1/delete") {
		t.Fatal("the default format has no delete button")
	}

	out = content(t, "admin/date-formats/edit.html", map[string]any{"format": formats[1], "example": "5 March 2027", "letters": []struct{ Letter, Meaning, Example string }{{"F", "month", "December"}}})
	if !strings.Contains(out, `action="/admin/config/date-formats/2"`) || !strings.Contains(out, `value="j F Y"`) || !strings.Contains(out, "disabled") {
		t.Fatalf("form:\n%s", out)
	}

	out = content(t, "admin/content-types/field_edit.html", map[string]any{
		"field":       entity.FieldInstanceDef{ID: 53, FieldType: entity.FieldTypeDate, DateFormat: "medium", Label: "Premiere"},
		"contentType": entity.ContentTypeDef{ID: 13},
		"dateFormats": formats,
	})
	if !strings.Contains(out, `name="date_format"`) || !strings.Contains(out, `<option value="medium" selected>`) {
		t.Fatalf("field edit:\n%s", out)
	}
}

// TestSEODateTokenFormat - [node:field:<date>:<format>] renders the date
// in a catalog date format; :year and no modifier keep their output; an
// unknown format falls back to 02.01.2006.
func TestSEODateTokenFormat(t *testing.T) {
	formats, _ := newTestDateFormats()
	s := &SEOService{dateFormats: formats}
	dc := entity.DynamicContent{
		Fields: entity.FieldInstanceDefs{{ID: 1, MachineName: "premiere_us", FieldType: entity.FieldTypeDate}},
		Values: map[int]any{1: time.Date(2027, 5, 15, 0, 0, 0, 0, time.UTC)},
	}
	for modifier, want := range map[string]string{"medium": "15 May 2027", "year": "2027", "": "15.05.2027", "nope": "15.05.2027"} {
		if got := s.fieldToken(t.Context(), dc, "premiere_us", modifier); got != want {
			t.Errorf("modifier %q: %q, want %q", modifier, got, want)
		}
	}
}

// TestCommentDateFormat - a comment date field keeps its default text
// and, with a date format of its own, renders in it.
func TestCommentDateFormat(t *testing.T) {
	zzChdirToProjectRoot(t)
	formats, _ := newTestDateFormats()
	created := time.Date(2027, 5, 15, 9, 30, 0, 0, time.UTC)
	render := func(t *testing.T, format string) string {
		t.Helper()
		fields := views.BuildCommentFieldInstances(context.Background(), entity.ViewFieldDefs{{ID: 9, BuiltinField: entity.ViewFieldBuiltinCommentCreatedAt, DateFormat: format}})
		items := views.BuildCommentListItems(context.Background(), entity.Comments{{ID: 1, CreatedAt: created}}, fields)
		if !items[0].Dates[9].Equal(created) || items[0].Values[9] == "" {
			t.Fatalf("item = %+v", items[0])
		}
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}, dateFormats: formats}
		set, err := svc.getOrParse("comment-date", nil, func() ([]string, error) { return []string{"web/templates/dynamic/comment_list.html"}, nil })
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		if err := set.ExecuteTemplate(&b, "dynamic_comment_field_value", map[string]any{"Field": fields[0], "Value": items[0].Values[9], "Date": items[0].Dates[9]}); err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(b.String())
	}
	if got := render(t, ""); !strings.Contains(got, ">15.05.2027 09:30<") {
		t.Fatalf("default: %q", got)
	}
	if got := render(t, "medium"); !strings.Contains(got, ">15 May 2027<") {
		t.Fatalf("medium: %q", got)
	}
}
