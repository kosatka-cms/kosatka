package http

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

func TestFileServerWithCache_setsHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := FileServerWithCache(dir, CacheStatic)
	req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != CacheStatic {
		t.Fatalf("Cache-Control = %q, want %q", got, CacheStatic)
	}
}

// TestFileServerWithCacheFilesOnly - a folder (with or without its
// slash) and a dotfile are not found: no listing of what is there.
func TestFileServerWithCacheFilesOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "js", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"js/app.js", ".env", "js/.git/config"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := FileServerWithCache(dir, CacheStatic)
	for target, want := range map[string]int{"/js/app.js": http.StatusOK, "/js/": http.StatusNotFound, "/js": http.StatusNotFound, "/": http.StatusNotFound, "/.env": http.StatusNotFound, "/js/.git/config": http.StatusNotFound} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != want {
			t.Errorf("%s: %d, want %d", target, rec.Code, want)
		}
	}
}

func TestFileServerWithCache_sitesStripPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pic.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := http.StripPrefix("/uploads/", FileServerWithCache(dir, CacheUploads))
	req := httptest.NewRequest(http.MethodGet, "/uploads/pic.jpg", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != CacheUploads {
		t.Fatalf("Cache-Control = %q, want %q", got, CacheUploads)
	}
}

// TestUploadsFileServer - uploaded files: regular files only, no
// listings or dotfiles, nosniff and a sandboxing CSP on every file, and
// anything but images and PDF as a download.
func TestUploadsFileServer(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"uploads/u1/pic.jpg":       "jpg",
		"uploads/u1/doc.pdf":       "pdf",
		"uploads/u1/page.html":     "<script>alert(1)</script>",
		"uploads/u1/notes.txt":     "text",
		"uploads/tmp/.htaccess":    "deny",
		"uploads/u1/.hidden/a.jpg": "x",
		"secret.txt":               "outside the files",
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := http.StripPrefix("/uploads/", UploadsFileServer(filestore.NewLocal(filepath.Join(dir, "uploads")), func() string { return "" }, CacheUploads))
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	for _, path := range []string{
		"/uploads/", "/uploads/u1/", "/uploads/u1", "/uploads/../secret.txt",
		"/uploads/tmp/.htaccess", "/uploads/u1/.hidden/a.jpg", "/uploads/u1/missing.jpg",
	} {
		if rec := get(path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}

	rec := get("/uploads/u1/pic.jpg")
	if rec.Code != http.StatusOK || rec.Body.String() != "jpg" {
		t.Fatalf("image: %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("image headers: %v", rec.Header())
	}
	if rec.Header().Get("Content-Disposition") != "" || rec.Header().Get("Cache-Control") != CacheUploads {
		t.Errorf("image: disposition %q, cache %q", rec.Header().Get("Content-Disposition"), rec.Header().Get("Cache-Control"))
	}
	if rec := get("/uploads/u1/doc.pdf"); rec.Header().Get("Content-Disposition") != "" || rec.Header().Get("Content-Security-Policy") != "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("PDF should open in place, unsandboxed, with nosniff: %v", rec.Header())
	}
	for _, path := range []string{"/uploads/u1/page.html", "/uploads/u1/notes.txt"} {
		rec := get(path)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Disposition") != "attachment" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s: %d, disposition %q", path, rec.Code, rec.Header().Get("Content-Disposition"))
		}
	}
	// Traversal stays inside the root.
	if rec := get("/uploads/../../../etc/passwd"); rec.Code != http.StatusNotFound {
		t.Errorf("traversal: %d", rec.Code)
	}
}

// TestUploadsFileServerRedirectsToStore - a store serving its files
// itself gets the visitors, with the query; dotfiles and folders stay 404.
func TestUploadsFileServerRedirectsToStore(t *testing.T) {
	h := http.StripPrefix("/files/", UploadsFileServer(filestore.NewLocal(t.TempDir()), func() string { return "https://cdn.example.com/site/" }, CacheUploads))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/u1/my%20pic.jpg?v=2", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://cdn.example.com/site/u1/my%20pic.jpg?v=2" {
		t.Errorf("redirect: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, path := range []string{"/files/u1/.htaccess", "/files/u1/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
}

// TestModuleStatic - /static/modules/<name>/ serves the module's own
// static files (internal/modules/<name>/static, or the binary's embedded
// ones) and falls back to web/static/modules/<name>/; a path out of the
// directory isn't served.
func TestModuleStatic(t *testing.T) {
	root := t.TempDir()
	for p, body := range map[string]string{
		"internal/modules/demo/static/demo.js": "own",
		"web/static/modules/legacy/legacy.js":  "legacy",
		"internal/modules/demo/secret.go":      "code",
		"web/static/modules/demo/fallback.css": "web",
	} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	mux := http.NewServeMux()
	mux.Handle("GET /static/modules/{module}/{filepath...}", moduleStatic(FileServerWithCache("./web/static", CacheStatic)))
	for path, want := range map[string]string{
		"/static/modules/demo/demo.js":          "own",
		"/static/modules/demo/fallback.css":     "web",
		"/static/modules/legacy/legacy.js":      "legacy",
		"/static/modules/demo/../secret.go":     "",
		"/static/modules/demo/%2e%2e/secret.go": "",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		got := ""
		if rec.Code == http.StatusOK {
			got = rec.Body.String()
		}
		if got != want {
			t.Errorf("%s: %d %q, want %q", path, rec.Code, got, want)
		}
	}

	// The binary's own (paths.SetModuleAssets, embedded only): the source
	// directory is ignored, the fallback stays.
	paths.SetModuleAssets(map[string]fs.FS{"demo": fstest.MapFS{"static/demo.js": {Data: []byte("embedded")}}}, true)
	t.Cleanup(func() { paths.SetModuleAssets(nil, false) })
	for path, want := range map[string]string{
		"/static/modules/demo/demo.js":      "embedded",
		"/static/modules/demo/fallback.css": "web",
		"/static/modules/demo/../demo.js":   "",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		got := ""
		if rec.Code == http.StatusOK {
			got = rec.Body.String()
			if cc := rec.Header().Get("Cache-Control"); want == "embedded" && cc != CacheStatic {
				t.Errorf("%s: Cache-Control %q", path, cc)
			}
		}
		if got != want {
			t.Errorf("embedded %s: %d %q, want %q", path, rec.Code, got, want)
		}
	}
}

// TestThemeStaticMinFallback - a theme's minified file, else its
// unminified one; a file the theme lacks goes on to web/static.
func TestThemeStaticMinFallback(t *testing.T) {
	themes := t.TempDir()
	files := map[string]string{"t/static/css/main.css": "src", "t/static/js/app.min.js": "min", "t/static/js/app.js": "src"}
	for rel, body := range files {
		p := filepath.Join(themes, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths.SetThemesDir(themes)
	t.Cleanup(func() { paths.SetThemesDir("") })
	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "web/static", http.StatusNotFound) })
	mux := http.NewServeMux()
	mux.Handle("GET /static/themes/{theme}/{filepath...}", themeStatic(fallback))
	for path, want := range map[string]string{
		"/static/themes/t/css/main.min.css":  "src",
		"/static/themes/t/js/app.min.js":     "min",
		"/static/themes/t/css/other.min.css": "web/static\n",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Body.String() != want {
			t.Errorf("%s: %d %q, want %q", path, rec.Code, rec.Body.String(), want)
		}
	}
}
