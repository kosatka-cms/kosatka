package themes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const catalogFixture = `{
  "generated": "2026-10-21T00:00:00Z",
  "themes": [
    {"name": "sand", "source": "https://github.com/example/kosatka-theme-sand", "version": "v1.2.0", "title": "Sand",
     "description": "A warm light theme.", "homepage": "https://example.com/sand", "license": "MIT",
     "tags": ["light", "Bad Tag", "cards"], "min_version": "0.2.0",
     "screenshot": "https://raw.githubusercontent.com/example/kosatka-theme-sand/v1.2.0/static/screenshot.jpg", "stars": 12},
    {"name": "night", "source": "git@github.com:example/night.git", "version": "v0.3.1", "title": "",
     "homepage": "javascript:alert(1)", "screenshot": "http://example.com/x.png", "min_version": "two"},
    {"name": "Bad Name", "source": "https://github.com/example/bad", "version": "v1.0.0"},
    {"name": "local", "source": "file:///srv/theme", "version": "v1.0.0"},
    {"name": "untagged", "source": "https://github.com/example/untagged", "version": "1.0.0"},
    {"name": "option", "source": "--upload-pack=x", "version": "v1.0.0"}
  ]
}`

// TestCatalogClean - the index's themes a page can show: a valid
// name, repository and release; an https homepage and screenshot, or
// none; lowercase tags; a title, else the name.
func TestCatalogClean(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "index.json")
	if err := os.WriteFile(file, []byte(catalogFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := FetchCatalog(context.Background(), nil, file)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Themes) != 2 || c.Themes[0].Name != "sand" || c.Themes[1].Name != "night" {
		t.Fatalf("themes %+v, want sand and night only", c.Themes)
	}
	sand, night := c.Themes[0], c.Themes[1]
	if strings.Join(sand.Tags, ",") != "light,cards" || sand.Screenshot == "" || sand.Homepage == "" {
		t.Errorf("sand: %+v", sand)
	}
	if night.Title != "night" || night.Homepage != "" || night.Screenshot != "" || night.MinVersion != "" {
		t.Errorf("night: %+v (no title → the name; no javascript: homepage, http screenshot, bad min_version)", night)
	}
	if !sand.Match("WARM cards") || sand.Match("warm dark") || !sand.Match("") {
		t.Error("Match: every word, case aside")
	}
}

func TestFetchCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write([]byte(catalogFixture))
		case "/huge.json":
			_, _ = w.Write([]byte(strings.Repeat(" ", maxCatalogBytes+1)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	if c, err := FetchCatalog(ctx, srv.Client(), srv.URL+"/index.json"); err != nil || len(c.Themes) != 2 {
		t.Errorf("over http: %d themes, %v", len(c.Themes), err)
	}
	if _, err := FetchCatalog(ctx, srv.Client(), srv.URL+"/missing.json"); err == nil {
		t.Error("a 404: no error")
	}
	if _, err := FetchCatalog(ctx, srv.Client(), srv.URL+"/huge.json"); err == nil {
		t.Error("an index over the limit: no error")
	}

	if _, err := (&CatalogService{}).Catalog(ctx); !errors.Is(err, ErrNoCatalog) {
		t.Errorf("no URL: %v, want ErrNoCatalog", err)
	}
	s := &CatalogService{URL: srv.URL + "/index.json", Client: srv.Client()}
	if c, err := s.Catalog(ctx); err != nil || len(c.Themes) != 2 {
		t.Fatalf("the service: %+v %v", c, err)
	}
	srv.Close()
	if c, err := s.Catalog(ctx); err != nil || len(c.Themes) != 2 {
		t.Errorf("the cached copy within the hour: %+v %v", c, err)
	}
}

func TestReleaseNewer(t *testing.T) {
	if !ReleaseNewer("v1.10.0", "v1.9.0") || ReleaseNewer("v1.2.0", "v1.2.0") || ReleaseNewer("v0.9.0", "v1.0.0") {
		t.Error("ReleaseNewer")
	}
}
