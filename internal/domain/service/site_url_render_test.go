package service

import (
	"html/template"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestSiteURLAdminPagesRender executes the Site information page (its
// site address field, editable or set by the config) and the Access
// blocking page (its robots.txt rules).
func TestSiteURLAdminPagesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	site := &entity.SiteInformation{SiteName: "Site", Timezone: "Europe/Moscow", BaseURL: "https://saved.example", RobotsExtra: "User-agent: SomeBot\nDisallow: /"}
	for _, tc := range []struct {
		path     string
		data     map[string]any
		want     []string
		dontWant []string
	}{
		{"admin/config/site_information.html", map[string]any{"site": site, "baseURLError": true, "canEditBaseURL": true, "timezones": []sitetime.ZoneOption{{Name: "UTC", Label: "UTC (UTC)"}, {Name: "Europe/Berlin", Label: "Berlin (UTC+1)", Group: "Europe"}, {Name: "Europe/Moscow", Label: "Moscow (UTC+3)", Group: "Europe"}, {Name: "Pacific/Auckland", Label: "Auckland (UTC+13)", Group: "Pacific"}}},
			[]string{`name="base_url" value="https://saved.example"`, "https://www.example.com - ",
				`<option value="UTC">UTC (UTC)</option><optgroup label="Europe"><option value="Europe/Berlin">`, `<option value="Europe/Moscow" selected>Moscow (UTC&#43;3)</option></optgroup><optgroup label="Pacific">`, `Auckland (UTC&#43;13)</option></optgroup>`},
			[]string{"disabled"}},
		{"admin/config/site_information.html", map[string]any{"site": site, "baseURLOverride": "https://config.example"},
			[]string{`value="https://config.example" disabled`, `type="hidden" name="base_url" value="https://saved.example"`, "PUBLIC_BASE_URL"}, nil},
		// Without full access: shown, not sent (the handler keeps the saved one).
		{"admin/config/site_information.html", map[string]any{"site": site},
			[]string{`id="base_url" value="https://saved.example" disabled`}, []string{`name="base_url"`}},
		{"admin/config/blocked_bots.html", map[string]any{"robotsExtra": site.RobotsExtra, "robots_saved": true, "error": "", "error_ip": ""},
			[]string{`action="/admin/config/robots/rules"`, "User-agent: SomeBot\nDisallow: /</textarea>", "robots.txt rules saved."}, nil},
	} {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		files, err := svc.pageFilesToParse(tc.path, strings.Split(tc.path, "/"))
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		set, err := svc.getOrParse(tc.path, nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, "content", tc.data); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		for _, w := range tc.want {
			if !strings.Contains(buf.String(), w) {
				t.Errorf("%s: missing %q", tc.path, w)
			}
		}
		for _, w := range tc.dontWant {
			if strings.Contains(buf.String(), w) {
				t.Errorf("%s: unexpected %q", tc.path, w)
			}
		}
	}
}
