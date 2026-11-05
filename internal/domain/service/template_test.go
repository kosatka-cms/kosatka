package service

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestTemplateSetPerZone - a user's own zone gets its own template set:
// siteTime/formatDate in that zone, the site's set untouched; RenderNamed
// takes the zone from RequestLocationKey; a builtin timestamp of a View
// row (Config "timestamp") is shown in the zone, a date field as stored,
// in cells and in group headings.
func TestTemplateSetPerZone(t *testing.T) {
	zzChdirToProjectRoot(t)
	if err := sitetime.Set("Europe/Moscow"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sitetime.Set("UTC") })
	vlad, _ := sitetime.Load("Asia/Vladivostok")
	utc := time.Date(2027, 1, 1, 20, 30, 0, 0, time.UTC) // 23:30 Moscow, 06:30 (2 Jan) Vladivostok

	svc := &DefaultTemplateService{cacheEnabled: true, cache: map[string]*template.Template{}}
	files := []string{"web/templates/fields/date.html"}
	run := func(zone *time.Location) string {
		set, err := svc.getOrParseIn("zone-test", nil, zone, "", func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		page, err := set.Clone()
		if err != nil {
			t.Fatal(err)
		}
		page = template.Must(page.New("p").Parse(`{{formatDate .T}}|{{(siteTime .T).Format "15:04"}}`))
		var b strings.Builder
		if err := page.Execute(&b, map[string]any{"T": utc}); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if got := run(nil); got != "01.01.2027 23:30|23:30" {
		t.Fatalf("site zone: %q", got)
	}
	if got := run(vlad); got != "02.01.2027 06:30|06:30" {
		t.Fatalf("user zone: %q", got)
	}
	if len(svc.cache) != 2 {
		t.Fatalf("one set per zone, got %d", len(svc.cache))
	}
	if dataZone(map[string]any{RequestLocationKey: vlad}) != vlad || dataZone(map[string]any{RequestLocationKey: sitetime.Location()}) != nil || dataZone(nil) != nil {
		t.Fatal("dataZone")
	}

	// View rows: builtin timestamp converted, date field as stored.
	created := entity.FieldInstanceDef{ID: -4, MachineName: "created_at", FieldType: entity.FieldTypeDate, Config: map[string]any{"timestamp": true}}
	premiere := entity.FieldInstanceDef{ID: 7, MachineName: "premiere", FieldType: entity.FieldTypeDate, Config: map[string]any{}}
	fm := svc.buildFuncMapIn(nil, vlad, "")
	conv := fm["viewDateValue"].(func(entity.FieldInstanceDef, any) any)
	if got := conv(created, utc).(time.Time); got.Day() != 2 || got.Hour() != 6 {
		t.Fatalf("timestamp: %v", got)
	}
	if got := conv(premiere, utc).(time.Time); !got.Equal(utc) || got.Location() != time.UTC {
		t.Fatalf("date field converted: %v", got)
	}
	rows := []views.ContentListItem{{Node: entity.Node{ID: 1}, Fields: entity.FieldInstanceDefs{created}, Values: map[int]any{-4: utc}}}
	groups, err := groupViewRowsIn(rows, "created_at", "day", "ru", func(v time.Time) time.Time { return v.In(vlad) }, nil)
	if err != nil || len(groups) != 1 || groups[0].Heading != "02.01.2027" {
		t.Fatalf("group heading in the zone: %+v %v", groups, err)
	}
}

func writeFragmentFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "frag.html")
	if err := os.WriteFile(file, []byte(`{{define "frag"}}hi{{end}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestGetOrParse_CachesWhenEnabled(t *testing.T) {
	file := writeFragmentFixture(t)
	svc := &DefaultTemplateService{cacheEnabled: true, cache: make(map[string]*template.Template)}

	calls := 0
	build := func() ([]string, error) {
		calls++
		return []string{file}, nil
	}

	tmpl1, err := svc.getOrParse("key", nil, build)
	if err != nil {
		t.Fatal(err)
	}
	tmpl2, err := svc.getOrParse("key", nil, build)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("expected buildFiles called once with a warm cache, got %d calls", calls)
	}
	if tmpl1 != tmpl2 {
		t.Error("expected the second call to return the same cached *template.Template instance")
	}
}

func TestGetOrParse_ReparsesWhenCacheDisabled(t *testing.T) {
	file := writeFragmentFixture(t)
	svc := &DefaultTemplateService{cacheEnabled: false, cache: make(map[string]*template.Template)}

	calls := 0
	build := func() ([]string, error) {
		calls++
		return []string{file}, nil
	}

	if _, err := svc.getOrParse("key", nil, build); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.getOrParse("key", nil, build); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected buildFiles called on every request with caching disabled (local dev hot-reload), got %d calls", calls)
	}
}

func TestGetOrParse_BypassesCacheWithFuncMapOverride(t *testing.T) {
	file := writeFragmentFixture(t)
	svc := &DefaultTemplateService{cacheEnabled: true, cache: make(map[string]*template.Template)}

	calls := 0
	build := func() ([]string, error) {
		calls++
		return []string{file}, nil
	}

	override := template.FuncMap{"noop": func() string { return "" }}
	if _, err := svc.getOrParse("key", override, build); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.getOrParse("key", override, build); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("expected a per-call FuncMap override to bypass the cache, got %d calls (want 2)", calls)
	}
}

// TestAdminTemplatePathsExist - every admin template path the code names
// ("admin/…html", the engine's or a module's) is a file: a moved or
// misspelt one would fail only when its page is opened.
func TestAdminTemplatePathsExist(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	literal := regexp.MustCompile(`"(admin/[a-z0-9_/-]+\.html)"`)
	moduleDirs, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || strings.Contains(p, "generated") {
				return err
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, m := range literal.FindAllStringSubmatch(string(src), -1) {
				found++
				candidates := []string{filepath.Join(root, "web", "templates", m[1])}
				for _, md := range moduleDirs {
					candidates = append(candidates, filepath.Join(md, m[1]))
				}
				exists := false
				for _, c := range candidates {
					if _, err := os.Stat(c); err == nil {
						exists = true
						break
					}
				}
				if !exists {
					t.Errorf("%s: no template %s", p, m[1])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if found < 50 {
		t.Errorf("only %d admin template paths found: the scan is broken", found)
	}
}

func TestTemplateResetCache(t *testing.T) {
	svc := &DefaultTemplateService{cache: map[string]*template.Template{"x": template.New("x")}}
	svc.ResetCache()
	if len(svc.cache) != 0 {
		t.Error("cache not reset")
	}
}

// TestTokenInURL - a {token} in the route or a token parameter.
func TestTokenInURL(t *testing.T) {
	mux := http.NewServeMux()
	var got []bool
	mux.HandleFunc("GET /preview/{token}", func(_ http.ResponseWriter, r *http.Request) { got = append(got, tokenInURL(r)) })
	mux.HandleFunc("GET /page", func(_ http.ResponseWriter, r *http.Request) { got = append(got, tokenInURL(r)) })
	for _, target := range []string{"/preview/abc", "/page?token=abc", "/page?q=token"} {
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	}
	if fmt.Sprint(got) != "[true true false]" || tokenInURL(nil) {
		t.Errorf("got %v", got)
	}
}
