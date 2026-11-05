package service

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestSiteTimeZone - times are shown in the site's time zone
// (pkg/sitetime): sitetime.Format, the siteTime template func (also
// for a *time.Time) and "today" of relative date filters.
func TestSiteTimeZone(t *testing.T) {
	if err := sitetime.Set("Asia/Vladivostok"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sitetime.Set("UTC") })
	utc := time.Date(2027, 1, 1, 20, 30, 0, 0, time.UTC) // 2 Jan 06:30 in Vladivostok

	if got := sitetime.Format(utc, "02.01.2006 15:04"); got != "02.01.2027 06:30" {
		t.Fatalf("sitetime.Format = %q", got)
	}
	if got := sitetime.Format(time.Time{}, "02.01.2006"); got != "-" {
		t.Fatalf("zero: %q", got)
	}

	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	tmpl := template.Must(template.New("x").Funcs(svc.buildFuncMap(nil)).Parse(`{{(siteTime .T).Format "02.01 15:04"}}|{{(siteTime .P).Format "15:04"}}|{{(siteTime .N).IsZero}}`))
	var nilTime *time.Time
	var b strings.Builder
	if err := tmpl.Execute(&b, map[string]any{"T": utc, "P": &utc, "N": nilTime}); err != nil {
		t.Fatal(err)
	}
	if b.String() != "02.01 06:30|06:30|true" {
		t.Fatalf("siteTime: %q", b.String())
	}

	today, err := entity.ParseRelativeDate("now", utc)
	if err != nil || !today.Equal(time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("relative today = %v, %v", today, err)
	}
}
