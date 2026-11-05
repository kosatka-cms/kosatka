package service

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestRobotsTxt - the engine's rules, an enabled module's (not a disabled
// one's), the sitemap at the site URL, then the site's own rules.
func TestRobotsTxt(t *testing.T) {
	saved := entity.Modules
	t.Cleanup(func() { entity.Modules = saved; SetModules(map[string]bool{"robots_on": true, "robots_off": true}) })
	entity.Modules = append(append([]entity.Module(nil), saved...),
		entity.Module{Name: "robots_on", RobotsDisallow: []string{"/on/*/edit"}},
		entity.Module{Name: "robots_off", RobotsDisallow: []string{"/off/"}})
	SetModules(map[string]bool{"robots_on": true, "robots_off": false})

	got := RobotsTxt("https://www.example.com/", "User-agent: SomeBot\r\nDisallow: /\r\n")
	for _, want := range []string{"User-agent: *\nDisallow: /admin/\n", "Disallow: /on/*/edit\n", "\nSitemap: https://www.example.com/sitemap.xml\n", "\nUser-agent: SomeBot\nDisallow: /\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("robots.txt lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "/off/") || strings.Contains(got, "\r") {
		t.Errorf("disabled module's rule or CR in:\n%s", got)
	}
	if strings.Index(got, "Sitemap:") > strings.Index(got, "SomeBot") {
		t.Errorf("the site's rules go last:\n%s", got)
	}
	if got := RobotsTxt("", ""); strings.Contains(got, "Sitemap:") {
		t.Errorf("no site URL - no sitemap line:\n%s", got)
	}
}
