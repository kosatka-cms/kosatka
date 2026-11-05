package videoembed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// png - the smallest picture a preview answer gives.
var png = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")

// testService - a service answering oEmbed (/oembed), a video page's
// Open Graph tags (/page), a preview (/thumb.png), 404 otherwise; and a
// Fetcher of its client keeping the previews in dir.
func testService(t *testing.T) (*httptest.Server, *Fetcher, string) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oembed":
			if !strings.HasPrefix(r.URL.Query().Get("url"), "https://video.test/") {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"title":"Trailer","thumbnail_url":"` + srv.URL + `/thumb.png","width":"640","height":360}`))
		case "/page":
			_, _ = w.Write([]byte(`<html><head><meta property="og:title" content="Clip"><meta property="og:image" content="` + srv.URL + `/thumb.png"><meta property="og:video:width" content="720"><meta property="og:video:height" content="1280"></head><body><meta property="og:title" content="not this"></body></html>`))
		case "/thumb.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	return srv, &Fetcher{client: srv.Client(), files: func() filestore.Store { return filestore.NewLocal(dir) }}, dir
}

// TestFetchOEmbed - the oEmbed answer's title, sizes (a number or a
// string) and preview, copied to the site; not copied - its address only.
func TestFetchOEmbed(t *testing.T) {
	srv, f, dir := testService(t)
	p := &Provider{Name: "test", OEmbed: srv.URL + "/oembed?url={url}", ThumbnailHosts: []string{"127.0.0.1"}}
	m := Match{Provider: p, URL: "https://video.test/1"}
	e := f.Fetch(context.Background(), m, true)
	if e.Failed || e.Title != "Trailer" || e.Width != 640 || e.Height != 360 || e.ThumbnailURL != srv.URL+"/thumb.png" {
		t.Fatalf("oEmbed: %+v", e)
	}
	if !strings.HasPrefix(e.ThumbnailFile, "video_embed/") || !strings.HasSuffix(e.ThumbnailFile, ".png") {
		t.Fatalf("preview copy: %q", e.ThumbnailFile)
	}
	if b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(e.ThumbnailFile))); err != nil || len(b) != len(png) {
		t.Errorf("preview copy on disk: %d %v", len(b), err)
	}
	if e := f.Fetch(context.Background(), m, false); e.ThumbnailFile != "" || e.ThumbnailURL == "" {
		t.Errorf("no copy: %+v", e)
	}
	// The link kind's proportions over the service's.
	m.Width, m.Height = 9, 16
	if e := f.Fetch(context.Background(), m, false); e.Width != 9 || e.Height != 16 {
		t.Errorf("the kind's proportions: %+v", e)
	}
}

// TestFetchOpenGraph - the video page's first og: tags of its head.
func TestFetchOpenGraph(t *testing.T) {
	srv, f, _ := testService(t)
	p := &Provider{Name: "test", OpenGraph: true, ThumbnailHosts: []string{"127.0.0.1"}}
	e := f.Fetch(context.Background(), Match{Provider: p, URL: srv.URL + "/page"}, false)
	if e.Failed || e.Title != "Clip" || e.Width != 720 || e.Height != 1280 || e.ThumbnailURL != srv.URL+"/thumb.png" {
		t.Errorf("Open Graph: %+v", e)
	}
}

// TestFetchFailed - no answer: failed, nothing kept; a preview from a
// host not the service's is dropped.
func TestFetchFailed(t *testing.T) {
	srv, f, dir := testService(t)
	p := &Provider{Name: "test", OEmbed: srv.URL + "/oembed?url={url}", ThumbnailHosts: []string{"127.0.0.1"}}
	if e := f.Fetch(context.Background(), Match{Provider: p, URL: "https://other.test/1"}, true); !e.Failed || e.Title != "" {
		t.Errorf("404: %+v", e)
	}
	p.ThumbnailHosts = []string{"cdn.example"}
	if e := f.Fetch(context.Background(), Match{Provider: p, URL: "https://video.test/1"}, true); e.Failed || e.ThumbnailURL != "" || e.ThumbnailFile != "" {
		t.Errorf("a foreign preview: %+v", e)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("nothing copied, got %v", entries)
	}
}

// TestCutTitle - a long title is cut to the column, a character whole.
func TestCutTitle(t *testing.T) {
	long := strings.Repeat("я", titleMaxLength+5)
	if got := cutTitle("  " + long); len([]rune(got)) != titleMaxLength {
		t.Errorf("cut to %d", len([]rune(got)))
	}
}
