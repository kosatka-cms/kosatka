//go:build dbintegration

package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestSiteIcon - the pages' icon is the theme's until one is uploaded on
// Site information; then the site's (a PNG also the home screen's), on the
// public and the admin pages, and /favicon.ico leads to it; deleted, the
// theme's again.
func TestSiteIcon(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		// html/template writes the "+" of the type as &#43;.
		const themeIcon = `<link rel="icon" href="/static/images/kosatka-icon.svg" type="image/svg&#43;xml">`
		if _, home := s.get("/"); !strings.Contains(home, themeIcon) {
			t.Error("the front page has no theme's icon")
		}
		favicon := func() string {
			t.Helper()
			resp, err := s.client.Get(s.base + "/favicon.ico")
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusFound {
				t.Fatalf("GET /favicon.ico: %d", resp.StatusCode)
			}
			return resp.Header.Get("Location")
		}
		if where := favicon(); where != "/static/images/kosatka-icon.svg" {
			t.Errorf("/favicon.ico before the upload: %q", where)
		}

		var pic bytes.Buffer
		img := image.NewRGBA(image.Rect(0, 0, 180, 180))
		img.Set(1, 1, color.RGBA{R: 255, A: 255})
		if err := png.Encode(&pic, img); err != nil {
			t.Fatal(err)
		}
		page := "/admin/config/system/site-information"
		save := func(extra url.Values, files map[string][2]string) {
			t.Helper()
			form := url.Values{"csrf_token": {s.csrf(page)}, "site_name": {"Smoke Site"}, "timezone": {"UTC"}, "default_language": {"en"}}
			for k, v := range extra {
				form[k] = v
			}
			if resp := s.postFiles(page, form, files); resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "saved=1") {
				t.Fatalf("saving site information: %d %s\n%s", resp.StatusCode, resp.Header.Get("Location"), s.log.String())
			}
		}
		save(nil, map[string][2]string{"icon": {"mark.png", pic.String()}})

		icon := regexp.MustCompile(`<link rel="icon" href="([^"]+/site/icon/[^"]+\.png)" type="image/png">`)
		_, home := s.get("/")
		m := icon.FindStringSubmatch(home)
		if m == nil {
			t.Fatalf("the front page has no uploaded icon:\n%s", home[:min(len(home), 2000)])
		}
		if !strings.Contains(home, `<link rel="apple-touch-icon" href="`+m[1]+`">`) {
			t.Error("a PNG icon isn't the home screen's")
		}
		if _, admin := s.get("/admin/content"); !strings.Contains(admin, `<link rel="icon" href="`+m[1]+`"`) {
			t.Error("the admin has no site's icon")
		}
		if where := favicon(); where != m[1] {
			t.Errorf("/favicon.ico: %q, want %q", where, m[1])
		}
		if resp, err := s.client.Get(s.base + m[1]); err != nil || resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" {
			t.Errorf("GET the icon: %v %v", resp, err)
		}

		// Saved again without a file: kept; deleted: the theme's.
		save(nil, nil)
		if _, home := s.get("/"); !strings.Contains(home, m[1]) {
			t.Error("a save without a file dropped the icon")
		}
		save(url.Values{"delete_icon": {"1"}}, nil)
		if _, home := s.get("/"); !strings.Contains(home, themeIcon) || strings.Contains(home, "apple-touch-icon") {
			t.Error("after the deletion: not the theme's icon")
		}
		s.checkLog()
	})
}
