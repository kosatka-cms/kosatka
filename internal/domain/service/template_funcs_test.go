package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	texttemplate "text/template"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestDateTemplateFunc - {{date}} formats by the catalog: "" is the
// default format, "datetime" falls back to the date and time while the
// catalog has none; no date renders empty.
func TestDateTemplateFunc(t *testing.T) {
	formats, repo := newTestDateFormats()
	svc := &DefaultTemplateService{dateFormats: formats}
	d := time.Date(2027, 5, 15, 9, 30, 0, 0, time.UTC)
	render := func(tpl string, data any) string {
		t.Helper()
		var b strings.Builder
		if err := texttemplate.Must(texttemplate.New("t").Funcs(svc.buildFuncMapIn(nil, time.UTC, "en")).Parse(tpl)).Execute(&b, data); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if got := render(`{{date .D ""}}|{{date .D "medium"}}|{{date .D "datetime"}}|{{date .P ""}}|{{date .Z ""}}`,
		map[string]any{"D": d, "P": &d, "Z": time.Time{}}); got != "15.05.2027|15 May 2027|15.05.2027 09:30|15.05.2027|" {
		t.Errorf("got %q", got)
	}
	repo.formats = append(repo.formats, entity.DateFormat{ID: 3, MachineName: entity.DateTimeFormat, Pattern: "Y-m-d H:i"})
	formats.Reset()
	if got := render(`{{date .D "datetime"}}|{{formatDate .D}}`, map[string]any{"D": d}); got != "2027-05-15 09:30|2027-05-15 09:30" {
		t.Errorf("with a datetime format: %q", got)
	}
}

// TestNoDeprecatedHTMLFunc - the engine's, the modules' and the bundled
// themes' templates output raw markup with rawHTML: html is Go's escaper
// (html/template refuses it in a pipeline) and safe doesn't exist.
func TestNoDeprecatedHTMLFunc(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	use := regexp.MustCompile(`\|\s*(html|safe)\s*\}\}|\{\{-?\s*(html|safe)\s`)
	for _, dir := range []string{"web/templates", "internal/modules", "themes"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(src), "\n") {
				if use.MatchString(line) {
					t.Errorf("%s:%d uses html or safe - rawHTML", path, i+1)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
