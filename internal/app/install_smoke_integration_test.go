//go:build dbintegration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

var (
	appBinOnce sync.Once
	appBin     string
	appBinErr  error
)

// buildApp builds the app binary once per test run.
func buildApp(t *testing.T) string {
	t.Helper()
	appBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "kosatka-smoke")
		if err != nil {
			appBinErr = err
			return
		}
		appBin = filepath.Join(dir, "app")
		build := exec.Command("go", "build", "-o", appBin, "./cmd/app")
		build.Dir = repoRoot(t)
		out, err := build.CombinedOutput()
		if err != nil {
			appBinErr = fmt.Errorf("go build: %w\n%s", err, out)
		}
	})
	if appBinErr != nil {
		t.Fatal(appBinErr)
	}
	return appBin
}

// freePort - a TCP port nothing listens on.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

var (
	passwordLine = regexp.MustCompile(`password[^:]*: (\S+)\s*$`)
	csrfMeta     = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)
)

// site - a new site installed with "app install" and served by the app,
// with an HTTP client keeping cookies (no redirects followed).
type site struct {
	t        *testing.T
	base     string
	client   *http.Client
	password string
	// installed - what app install printed.
	installed string
	log       *syncBuffer
	stop      func()
	// The site's binary and its environment (its config), for app's
	// other commands (s.app).
	bin string
	env []string
}

// installSite runs "app install" on the empty database, then beforeServe
// (it may change the database), then starts the server.
func installSite(t *testing.T, db testdb.DB, beforeServe func(), installArgs ...string) *site {
	t.Helper()
	return installSiteBin(t, db, buildApp(t), beforeServe, installArgs...)
}

// installSiteBin is installSite with the binary bin (a site's own, with
// modules from outside the engine), run in the repository's root.
func installSiteBin(t *testing.T, db testdb.DB, bin string, beforeServe func(), installArgs ...string) *site {
	t.Helper()
	root := repoRoot(t)
	port := freePort(t)
	env := siteEnv(t, db, port)

	install := exec.Command(bin, append([]string{"install", "-site-name", "Smoke Site", "-admin-name", "admin", "-admin-email", "admin@example.com"}, installArgs...)...)
	install.Dir, install.Env = root, env
	out, err := install.CombinedOutput()
	if err != nil {
		t.Fatalf("app install: %v\n%s", err, out)
	}
	s := &site{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port), bin: bin, env: env, installed: string(out)}
	for _, line := range strings.Split(string(out), "\n") {
		if m := passwordLine.FindStringSubmatch(line); m != nil {
			s.password = m[1]
		}
	}
	if s.password == "" {
		t.Fatalf("no password in the install output:\n%s", out)
	}
	// The bot trap off: the tests post forms at once, without the time
	// they were shown (TestBotTrap turns it on).
	if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO comment_settings (id, bot_trap) VALUES (1, ?)`), false); err != nil {
		t.Fatal(err)
	}
	if beforeServe != nil {
		beforeServe()
	}
	jar, _ := cookiejar.New(nil)
	s.client = &http.Client{Jar: jar, Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	s.serve()
	return s
}

// siteEnv - the environment of a site's commands and server on db: a
// config of its own (port, the database) in CONFIG_PATH, its private
// files in a directory of their own.
func siteEnv(t *testing.T, db testdb.DB, port int) []string {
	t.Helper()
	driver, tls := "mysql", ""
	if db.Driver == "pgx" {
		driver, tls = "postgres", "disable"
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := fmt.Sprintf(`env: local
port: %d
public_base_url: http://127.0.0.1:%d
db:
  driver: %s
  host: %s
  port: %d
  scheme: %s
  username: %s
  password: %s
  tls: %q
session:
  secret_key: smoke-test-session-key-0123456789abcdef
`, port, port, driver, db.Conn.Host, db.Conn.Port, db.Conn.Name, db.Conn.User, db.Conn.Password, tls)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return append(os.Environ(), "CONFIG_PATH="+cfgPath, "FILES_PRIVATE_DIR="+t.TempDir())
}

// serve starts the server (s.bin, a log of its own) and waits until it
// answers; checkLog stops it.
func (s *site) serve() {
	s.t.Helper()
	s.log = &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	server := exec.CommandContext(ctx, s.bin)
	server.Dir, server.Env = repoRoot(s.t), s.env
	server.Stdout, server.Stderr = s.log, s.log
	if err := server.Start(); err != nil {
		s.t.Fatal(err)
	}
	s.stop = func() { cancel(); _ = server.Wait() }
	s.t.Cleanup(s.stop)

	deadline := time.Now().Add(60 * time.Second)
	for {
		if resp, err := s.client.Get(s.base + "/"); err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("the server didn't start:\n%s", s.log.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// app runs "app <args>" on the site (its config), in the repository's
// root; it fails the test on an error and returns the output.
func (s *site) app(args ...string) string {
	s.t.Helper()
	cmd := exec.Command(s.bin, args...)
	cmd.Dir, cmd.Env = repoRoot(s.t), s.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("app %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func (s *site) get(path string) (int, string) {
	s.t.Helper()
	resp, err := s.client.Get(s.base + path)
	if err != nil {
		s.t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// csrf - the CSRF token of the page at path; a guest's page has none (no
// session yet), so, as the page's script does (csrf_head), it is asked
// for at /api/csrf-token.
func (s *site) csrf(path string) string {
	s.t.Helper()
	_, page := s.get(path)
	if m := csrfMeta.FindStringSubmatch(page); m != nil {
		return m[1]
	}
	code, body := s.get("/api/csrf-token")
	var got struct {
		Token string `json:"token"`
	}
	if code != http.StatusOK || json.Unmarshal([]byte(body), &got) != nil || got.Token == "" {
		s.t.Fatalf("no CSRF token for %s: /api/csrf-token %d %s", path, code, body)
	}
	return got.Token
}

func (s *site) post(path string, form url.Values) *http.Response {
	s.t.Helper()
	resp, err := s.client.PostForm(s.base+path, form)
	if err != nil {
		s.t.Fatalf("POST %s: %v", path, err)
	}
	_ = resp.Body.Close()
	return resp
}

// postMultipart posts form as multipart/form-data (the content forms).
func (s *site) postMultipart(path string, form url.Values) *http.Response {
	s.t.Helper()
	return s.postFiles(path, form, nil)
}

// postFiles posts form as multipart/form-data with files (field → name
// and content).
func (s *site) postFiles(path string, form url.Values, files map[string][2]string) *http.Response {
	s.t.Helper()
	var list []upload
	for field, f := range files {
		list = append(list, upload{field, f[0], f[1]})
	}
	return s.postUploads(path, form, list)
}

// upload - a file a form posts: its field, name and content.
type upload struct{ field, name, content string }

// postUploads posts form as multipart/form-data with files, several under
// one field too, in order.
func (s *site) postUploads(path string, form url.Values, files []upload) *http.Response {
	s.t.Helper()
	resp, _ := s.postUploadsPage(path, form, files)
	return resp
}

// postUploadsPage - postUploads with the page answered.
func (s *site) postUploadsPage(path string, form url.Values, files []upload) (*http.Response, string) {
	s.t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, vs := range form {
		for _, v := range vs {
			if err := w.WriteField(k, v); err != nil {
				s.t.Fatal(err)
			}
		}
	}
	for _, f := range files {
		fw, err := w.CreateFormFile(f.field, f.name)
		if err != nil {
			s.t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.content)); err != nil {
			s.t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		s.t.Fatal(err)
	}
	resp, err := s.client.Post(s.base+path, w.FormDataContentType(), &body)
	if err != nil {
		s.t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	page, _ := io.ReadAll(resp.Body)
	return resp, string(page)
}

// signIn signs the administrator in with the printed password.
func (s *site) signIn() {
	s.t.Helper()
	resp := s.post("/login", url.Values{"login": {"admin"}, "pass": {s.password}, "csrf_token": {s.csrf("/login")}})
	if resp.StatusCode != http.StatusFound || !strings.HasSuffix(resp.Header.Get("Location"), "/me") {
		s.t.Fatalf("sign-in: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

// checkLog stops the server and fails on errors it logged.
func (s *site) checkLog() {
	s.t.Helper()
	s.stop()
	for _, line := range strings.Split(s.log.String(), "\n") {
		if strings.Contains(line, "level=ERROR") || strings.Contains(line, "panic") {
			s.t.Errorf("server log: %s", line)
		}
	}
}

// TestInstallAndServe - a new site end to end on MySQL and PostgreSQL:
// "app install" on an empty database, then the server answers the public
// pages, the administrator signs in with the printed password and opens
// the admin, and the server logs no error.
func TestInstallAndServe(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, nil)
		// No SECRETS_KEY in the config: app install offers one.
		if !regexp.MustCompile(`(?m)SECRETS_KEY=[0-9a-f]{64}$`).MatchString(s.installed) {
			t.Errorf("no SECRETS_KEY offered:\n%s", s.installed)
		}
		for path, want := range map[string]int{"/": 200, "/login": 200, "/user/register": 200, "/rss.xml": 200,
			"/api/search?q=smoke": 200, "/no-such-page": 404} {
			if code, _ := s.get(path); code != want {
				t.Errorf("GET %s: %d, want %d", path, code, want)
			}
		}
		// Installing again is refused with its own exit code: the Docker
		// start script then runs app migrate (any other failure stops it) -
		// with flags a new site would refuse too (a demo set in a language
		// it has no content in): they don't matter to an installed site.
		for _, args := range [][]string{nil, {"-demo-set", "no-such-set", "-language", "en"}} {
			again := exec.Command(s.bin, append([]string{"install", "-site-name", "Again", "-admin-name", "admin", "-admin-email", "admin@example.com"}, args...)...)
			again.Dir, again.Env = repoRoot(t), s.env
			if out, _ := again.CombinedOutput(); again.ProcessState == nil || again.ProcessState.ExitCode() != exitInstalledAlready {
				t.Errorf("app install %v on an installed site: %v, want exit %d\n%s", args, again.ProcessState, exitInstalledAlready, out)
			}
		}
		// A guest reading pages gets no cookie (no session), so a CDN can
		// keep the page; the CSRF token comes when asked for.
		for _, path := range []string{"/", "/login", "/user/register"} {
			resp, err := http.Get(s.base + path)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if len(resp.Cookies()) > 0 {
				t.Errorf("GET %s as a guest set cookies: %v", path, resp.Cookies())
			}
		}
		_, home := s.get("/")
		for _, want := range []string{"<title>Smoke Site", `class="site-name">Smoke Site`, `href="/login"`,
			`class="site-footer-credit block-copyright">© ` + strconv.Itoa(time.Now().Year()) + ` Smoke Site</p>`,
			`class="block block-menu"`, `<a href="/"`} {
			if !strings.Contains(home, want) {
				t.Errorf("the home page has no %q", want)
			}
		}

		s.signIn()
		// The standard modules only (entity.Module.InstallByDefault): the
		// feeds aren't installed; enabled on the Modules page, they are.
		if code, _ := s.get("/admin/config/feeds"); code != http.StatusNotFound {
			t.Errorf("GET /admin/config/feeds before the feeds are enabled: %d, want 404", code)
		}
		modules := url.Values{"csrf_token": {s.csrf("/admin/config/modules")}}
		for _, m := range entity.Modules {
			if m.InstallByDefault || m.Name == "feeds" {
				modules.Set("module_"+m.Name, "1")
			}
		}
		if resp := s.post("/admin/config/modules", modules); resp.StatusCode != http.StatusSeeOther || !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("enabling the feeds: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
		for path, want := range map[string]int{"/admin/config/feeds": 200, "/admin/config/feeds/new": 200} {
			if code, _ := s.get(path); code != want {
				t.Errorf("GET %s with the feeds enabled: %d, want %d", path, code, want)
			}
		}
		// A module's assets only on the pages that show it: the 404 log's
		// CSS on its page, not on the front page; no marker is left.
		if _, home := s.get("/"); strings.Contains(home, "/static/modules/not_found_log/") || strings.Contains(home, "<!--kosatka:") {
			t.Error("the front page has the 404 log's CSS or an attach marker")
		}
		if _, page := s.get("/admin/config/redirects/not-found"); !strings.Contains(page, "/static/modules/not_found_log/not_found_log.min.css") || strings.Contains(page, "<!--kosatka:") {
			t.Error("the 404 log's page has no CSS of its own, or an attach marker")
		}

		if _, home := s.get("/"); !strings.Contains(home, `href="/logout"`) {
			t.Error("signed in, the header has no log out link")
		}
		// A user without an avatar shows the engine's placeholder, which
		// is there.
		if _, me := s.get("/me"); !strings.Contains(me, entity.AvatarPlaceholderURL) {
			t.Error("the profile has no avatar placeholder")
		}
		if code, _ := s.get(entity.AvatarPlaceholderURL); code != 200 {
			t.Errorf("GET %s: %d", entity.AvatarPlaceholderURL, code)
		}
		for _, path := range []string{"/me", "/admin/content", "/admin/users", "/admin/config/modules",
			"/admin/config/system/site-information", "/admin/config/date-formats", "/admin/config/text-formats",
			"/create/article", "/create/page"} {
			if code, _ := s.get(path); code != 200 {
				t.Errorf("GET %s as the administrator: %d", path, code)
			}
		}
		// /admin is the Overview: the figures and what waits.
		if code, page := s.get("/admin"); code != http.StatusOK || !strings.Contains(page, "admin-dash-tiles") || !strings.Contains(page, `id="dash-waiting"`) {
			t.Errorf("GET /admin: %d, want the Overview\n%s", code, page)
		}
		// The style guide: in the site's layout (the theme's styles), with
		// the sample list of cards and the editor of a new site's format
		// with one (Basic HTML) over the sample text.
		if code, guide := s.get("/admin/config/appearance/styleguide"); code != 200 ||
			!strings.Contains(guide, "/static/themes/kosatka/") || !strings.Contains(guide, `class="view-list-item"`) ||
			!strings.Contains(guide, `data-target-field="sg-editor-text"`) || !strings.Contains(guide, "&lt;h2&gt;Heading 2&lt;/h2&gt;") {
			t.Errorf("GET the style guide: %d, the theme's styles: %v, the list: %v, the editor: %v, its text: %v", code,
				strings.Contains(guide, "/static/themes/kosatka/"), strings.Contains(guide, `class="view-list-item"`),
				strings.Contains(guide, `data-target-field="sg-editor-text"`), strings.Contains(guide, "&lt;h2&gt;Heading 2&lt;/h2&gt;"))
		}
		// A theme's preview: the base theme (the engine's templates)
		// active, the administrator sees the site in Kosatka, with the
		// preview's notice, a guest in the active theme; until the preview
		// ends. Then Kosatka is active again.
		themeCSS := func(page string) bool { return strings.Contains(page, "/static/themes/kosatka/") }
		token := s.csrf("/admin/config/appearance")
		if resp := s.post("/admin/config/appearance", url.Values{"theme": {""}, "csrf_token": {token}}); !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("choosing the base theme: %s", resp.Header.Get("Location"))
		}
		if resp := s.post("/admin/config/appearance/preview", url.Values{"theme": {"kosatka"}, "csrf_token": {token}}); resp.StatusCode != http.StatusSeeOther {
			t.Errorf("starting the preview: %d", resp.StatusCode)
		}
		if _, previewed := s.get("/"); !themeCSS(previewed) || !strings.Contains(previewed, "admin-top-bar-preview") {
			t.Errorf("the preview: kosatka's styles %v, the notice %v", themeCSS(previewed), strings.Contains(previewed, "admin-top-bar-preview"))
		}
		if resp, err := http.Get(s.base + "/"); err != nil {
			t.Fatal(err)
		} else {
			guest, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if themeCSS(string(guest)) {
				t.Error("a guest sees the previewed theme, not the active one")
			}
		}
		s.post("/admin/config/appearance/preview/stop", url.Values{"csrf_token": {token}})
		if _, after := s.get("/"); themeCSS(after) {
			t.Error("the preview goes on after it ended")
		}
		if resp := s.post("/admin/config/appearance", url.Values{"theme": {"kosatka"}, "csrf_token": {token}}); !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("choosing kosatka again: %s", resp.Header.Get("Location"))
		}
		// The active theme's settings: a colour set on its page reaches the
		// site's pages as its CSS variable; reset, it's the default.
		if code, page := s.get("/admin/config/appearance/theme/kosatka"); code != 200 || !strings.Contains(page, `name="setting_accent"`) {
			t.Errorf("GET kosatka's settings: %d, the accent field: %v", code, strings.Contains(page, `name="setting_accent"`))
		}
		token = s.csrf("/admin/config/appearance/theme/kosatka")
		if resp := s.post("/admin/config/appearance/theme/kosatka", url.Values{"setting_accent": {"#aa0000"}, "setting_accent_dark": {"#22d3ee"},
			"setting_scheme": {"light"}, "csrf_token": {token}}); resp.StatusCode != http.StatusSeeOther {
			t.Errorf("saving kosatka's settings: %d", resp.StatusCode)
		}
		if _, home := s.get("/"); !strings.Contains(home, "--kosatka-accent: #aa0000;") || !strings.Contains(home, `f="light"`) {
			t.Errorf("the settings on the site: the accent %v, the scheme %v", strings.Contains(home, "--kosatka-accent: #aa0000;"), strings.Contains(home, `f="light"`))
		}
		s.post("/admin/config/appearance/theme/kosatka", url.Values{"action": {"reset"}, "csrf_token": {token}})
		if _, home := s.get("/"); !strings.Contains(home, "--kosatka-accent: #0e7490;") || !strings.Contains(home, `f="dark"`) {
			t.Error("the reset accent or scheme isn't the default (dark)")
		}
		// The status report on a fresh site: the database is up to date,
		// the modules are listed with their last migration, the database's
		// version is read.
		code, status := s.get("/admin/reports/status")
		if code != 200 {
			t.Fatalf("status report: %d", code)
		}
		for _, want := range []string{"Database updates", "Up to date", "Public files", "Feeds", string(db.Dialect)} {
			if !strings.Contains(status, want) {
				t.Errorf("the status report has no %q", want)
			}
		}
		s.checkLog()
	})
}

// routerGETPaths - the GET routes without path parameters of the router
// and the modules.
func routerGETPaths(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "internal", "modules", "*", "routes.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filepath.Join(repoRoot(t), "internal", "http", "router.go"))
	var paths []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`"GET (/[^"{ ]*)"`).FindAllStringSubmatch(string(src), -1) {
			paths = append(paths, m[1])
		}
	}
	if len(paths) < 20 {
		t.Fatalf("only %d GET routes found in router.go", len(paths))
	}
	return paths
}

// TestInstallDemoAndServe - "install -demo": the front page lists the
// demo articles with their covers (the image style's copy is made), an
// article opens, the About page is in the main menu.
func TestInstallDemoAndServe(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSiteWith(t, db, true)
		demo, err := loadDemoContent("en")
		if err != nil {
			t.Fatal(err)
		}
		_, home := s.get("/")
		_, page2 := s.get("/?page=2")
		// The front page lists ten articles a page, the newest (the first) on top.
		const perPage = 10
		for i, a := range demo.Articles {
			if on := map[bool]string{true: home, false: page2}[i < perPage]; !strings.Contains(on, a.Title) {
				t.Errorf("front page %d has no %q", 1+i/perPage, a.Title)
			}
		}
		covers := regexp.MustCompile(`<img[^>]+src="([^"]*/styles/`+articleCardStyle+`/[^"]+)"`).FindAllStringSubmatch(home, -1)
		if want := min(perPage, len(demo.Articles)); len(covers) != want {
			t.Fatalf("the front page has %d covers, want %d", len(covers), want)
		}
		// The sidebar: search, the most viewed article in Popular, the
		// latest comment (its text: comments have no subject).
		top := demo.Articles[0]
		for _, a := range demo.Articles {
			if a.Views > top.Views {
				top = a
			}
		}
		latest := demo.Articles[0].Comments[len(demo.Articles[0].Comments)-1].Body
		// The tag cloud: its title, the tags sized by use, linking their pages.
		cloud := `<li class="tag-cloud-item tag-cloud-level-6"><a href="/` + demo.Vocabularies.Tags.Slug + `/`
		for _, want := range []string{`id="siteSearch"`, demo.Blocks.Popular, top.Title, latest, ">" + demo.Blocks.Tags + "</h2>", cloud} {
			if !strings.Contains(home, want) {
				t.Errorf("the front page's sidebar has no %q", want)
			}
		}
		resp, err := s.client.Get(s.base + covers[0][1])
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
			t.Errorf("GET %s: %d %s", covers[0][1], resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		// The card style's 2x derivatives (srcset), made on request.
		wide := regexp.MustCompile(`srcset="[^"]*?(\S+/styles/` + articleCardStyle + `/@2x/\S+) 2x"`).FindStringSubmatch(home)
		if wide == nil {
			t.Fatal("the covers have no srcset with the card style's 2x")
		}
		if resp, err := s.client.Get(s.base + wide[1]); err != nil {
			t.Fatal(err)
		} else if _ = resp.Body.Close(); resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
			t.Errorf("GET %s: %d %s", wide[1], resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		article := regexp.MustCompile(`href="(/article/[^"]+)"`).FindStringSubmatch(home)
		if article == nil {
			t.Fatal("the front page links no article")
		}
		// Its picture preloaded with the srcset its <img> has.
		if code, page := s.get(article[1]); code != 200 || !strings.Contains(page, "<h2>") || !strings.Contains(page, `rel="preload" as="image"`) || !strings.Contains(page, "imagesrcset=") {
			t.Errorf("GET %s: %d", article[1], code)
		}
		// The newest article: its section and tags link their term pages,
		// its comments and replies load, by the demo users.
		first := demo.Articles[0]
		_, firstPage := s.get("/article/" + slug.Generate(first.Title))
		for _, want := range append([]string{first.Section}, first.Tags...) {
			if !strings.Contains(firstPage, want) {
				t.Errorf("the first article has no term %q", want)
			}
		}
		// The live search finds it by the second half of its title's words:
		// a new site searches its content types without setting them up.
		words := strings.Fields(first.Title)
		_, found := s.get("/api/search?q=" + url.QueryEscape(strings.Join(words[len(words)/2:], " ")))
		var answer struct {
			Results []struct{ Title, Type string } `json:"results"`
		}
		if err := json.Unmarshal([]byte(found), &answer); err != nil || len(answer.Results) == 0 || answer.Results[0].Title != first.Title || answer.Results[0].Type != "article" {
			t.Errorf("the live search for the first article: %v %s", err, found)
		}
		tagPage := regexp.MustCompile(`href="(/tags/[^"]+)"`).FindStringSubmatch(firstPage)
		if tagPage == nil {
			t.Fatal("the first article links no tag page")
		}
		// The term pages' View lists it (the sidebar names it too: look
		// in the list only).
		code, page := s.get(tagPage[1])
		list, _, _ := strings.Cut(page, `class="sidebar`)
		_, list, _ = strings.Cut(list, `class="view-content"`)
		if code != 200 || !strings.Contains(list, first.Title) {
			t.Errorf("GET %s: %d, lists the article: %v", tagPage[1], code, strings.Contains(list, first.Title))
		}
		// A section's page lists its articles only, not another section's.
		sectionPage := regexp.MustCompile(`href="(/` + demo.Vocabularies.Sections.Slug + `/[^"]+)"`).FindStringSubmatch(firstPage)
		if sectionPage == nil {
			t.Fatal("the first article links no section page")
		}
		_, page = s.get(sectionPage[1])
		list, _, _ = strings.Cut(page, `class="sidebar`)
		_, list, _ = strings.Cut(list, `class="view-content"`)
		for _, a := range demo.Articles {
			if in := strings.Contains(list, a.Title); in != (a.Section == first.Section) {
				t.Errorf("the %s section's page lists %q (of %s): %v", first.Section, a.Title, a.Section, in)
			}
		}
		nodeID := regexp.MustCompile(`data-node-id="(\d+)"`).FindStringSubmatch(firstPage)
		if nodeID == nil {
			t.Fatal("the first article has no comments section")
		}
		// A new site has the comments in the page's HTML; the API gives
		// the same (the pager's, an ajax site's).
		if !strings.Contains(firstPage, `data-loaded="1"`) || !strings.Contains(firstPage, first.Comments[0].Body) {
			t.Error("the first article's page hasn't its comments in its HTML")
		}
		_, comments := s.get("/api/comments/" + nodeID[1] + "?page=1")
		for _, c := range first.Comments {
			if !strings.Contains(comments, c.Body) || !strings.Contains(comments, c.Author) {
				t.Errorf("the first article's comments have no %q by %s", c.Body, c.Author)
			}
			for _, r := range c.Replies {
				if !strings.Contains(comments, r.Body) {
					t.Errorf("the first article's comments have no reply %q", r.Body)
				}
			}
		}
		about := regexp.MustCompile(`<a href="(/page/[^"]+)"[^>]*>` + regexp.QuoteMeta(demo.Page.Title) + `<`).FindStringSubmatch(home)
		if about == nil {
			t.Fatal("the main menu has no link to the About page")
		}
		if code, page := s.get(about[1]); code != 200 || !strings.Contains(page, demo.Page.Title) {
			t.Errorf("GET %s: %d", about[1], code)
		}
		checkDemoModules(t, s, home, nodeID[1], firstPage)
		s.checkLog()
	})
}

// moduleDemo reads a module's English demo content (its
// manifest/demo/en.json) into v.
func moduleDemo(t *testing.T, module string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("internal", "modules", module, "manifest", "demo", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// checkDemoModules - the modules' demo steps ran: the poll and the form,
// linked from the footer, with the votes' options and the form's
// questions; the poll's block on the front page only; the gallery of the
// newest article
// (articleID, its page articlePage) in its block, its pages open to a
// guest; the galleries overview, where the administrator switches the
// article's galleries off - then they don't open.
func checkDemoModules(t *testing.T, s *site, home, articleID, articlePage string) {
	t.Helper()
	has := func(page, want string) bool {
		return strings.Contains(page, want) || strings.Contains(page, html.EscapeString(want))
	}
	var poll struct {
		Title, Menu string
		Options     []string
	}
	moduleDemo(t, "polls", &poll)
	var form struct {
		Title, Menu string
		Elements    []struct{ Label string }
	}
	moduleDemo(t, "forms", &form)
	var gallery struct{ Title string }
	moduleDemo(t, "galleries", &gallery)
	var ratings struct{ Block string }
	moduleDemo(t, "ratings", &ratings)

	for _, link := range []string{poll.Menu, form.Menu} {
		if !has(home, link) {
			t.Errorf("the front page has no %q", link)
		}
	}
	code, page := s.get("/poll/" + slug.Generate(poll.Title))
	if code != 200 {
		t.Errorf("GET the poll: %d", code)
	}
	for _, o := range poll.Options {
		if !has(page, o) {
			t.Errorf("the poll has no option %q", o)
		}
	}
	// The poll's block: on the front page only, loading the poll alone.
	embed := regexp.MustCompile(`data-poll-embed="([^"]+)"`).FindStringSubmatch(home)
	if embed == nil {
		t.Fatal("the front page has no poll block")
	}
	if strings.Contains(articlePage, "data-poll-embed") || !strings.Contains(articlePage, `id="siteSearch"`) {
		t.Error("the poll block shows beyond the front page")
	}
	code, page = s.get(embed[1])
	if code != 200 || !has(page, poll.Title) {
		t.Errorf("GET %s: %d", embed[1], code)
	}
	for _, o := range poll.Options {
		if !has(page, o) {
			t.Errorf("the poll block has no option %q", o)
		}
	}
	code, page = s.get("/form/" + slug.Generate(form.Title))
	if code != 200 {
		t.Errorf("GET the form: %d", code)
	}
	for _, e := range form.Elements {
		if !has(page, e.Label) {
			t.Errorf("the form has no question %q", e.Label)
		}
	}
	code, galleries := s.get("/api/node/" + articleID + "/galleries-embed")
	if code != 200 || !has(galleries, gallery.Title) {
		t.Errorf("the newest article's galleries: %d, has %q: %v", code, gallery.Title, has(galleries, gallery.Title))
	}
	galleryPage := regexp.MustCompile(`href="(/galleries/\d+)"`).FindStringSubmatch(galleries)
	if galleryPage == nil {
		t.Fatal("the galleries block links no gallery's page")
	}
	guest := s.visitor()
	for _, path := range []string{galleryPage[1], "/node/" + articleID + "/galleries"} {
		if code, _ := guest.get(path); code != http.StatusOK {
			t.Errorf("a guest, %s: %d", path, code)
		}
	}
	// The "The content's galleries" block, placed on install, loads them.
	if !has(articlePage, "/api/node/"+articleID+"/galleries-embed") {
		t.Error("the newest article's page has no galleries block")
	}

	// The galleries overview: the article can have galleries, has one;
	// switched off there, it no longer can.
	s.signIn()
	code, overview := s.get("/admin/content/galleries?show=on")
	if code != 200 || !strings.Contains(overview, `action="/admin/content/galleries/`+articleID+`"`) || !strings.Contains(overview, "1 gallery") {
		t.Fatalf("the galleries overview: %d, the article on it: %v", code, strings.Contains(overview, `/admin/content/galleries/`+articleID))
	}
	resp := s.post("/admin/content/galleries/"+articleID, url.Values{"query": {"show=on"}, "csrf_token": {s.csrf("/admin/content/galleries")}})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/admin/content/galleries?show=on" {
		t.Errorf("switching off: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if _, overview := s.get("/admin/content/galleries?show=on"); strings.Contains(overview, `action="/admin/content/galleries/`+articleID+`"`) {
		t.Error("switched off, the article is still among the content that can have galleries")
	}
	// Switched off, its galleries don't open by their addresses either.
	for path, want := range map[string]int{galleryPage[1]: http.StatusNotFound, "/node/" + articleID + "/galleries": http.StatusForbidden} {
		if code, _ := guest.get(path); code != want {
			t.Errorf("switched off, a guest, %s: %d, want %d", path, code, want)
		}
	}
	// The top rated list in the sidebar: the rated articles, their
	// averages - ratings (off by default, like a site's own choice) on.
	s.signIn()
	_, modules := s.get("/admin/config/modules")
	switched := settingsFormValues(modules)
	switched.Set("module_ratings", "1")
	switched.Set("csrf_token", s.csrf("/admin/config/modules"))
	if resp := s.post("/admin/config/modules", switched); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("modules: %d", resp.StatusCode)
	}
	if _, rated := s.get("/"); !has(rated, ratings.Block) || !strings.Contains(rated, `class="rating-average"`) {
		t.Errorf("the sidebar has no top rated list %q with averages", ratings.Block)
	}
}

// TestServeModulesNeverInstalled - a site installed without modules
// ("app install -modules none") has none of their tables: every page,
// as a guest and as the administrator, answers without a server error,
// content and a user with content can be deleted, and nothing logs an
// error (core code skips a never-installed module's tables).
func TestServeModulesNeverInstalled(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := installSite(t, db, func() {
			for _, m := range entity.Modules {
				for _, table := range m.Tables {
					if _, err := db.DB.Exec("SELECT 1 FROM " + table + " WHERE 1 = 0"); err == nil {
						t.Errorf("%s's table %s is there, want none", m.Name, table)
					}
				}
			}
		}, "-modules", "none")

		paths := routerGETPaths(t)
		crawl := func(who string) {
			for _, p := range paths {
				if code, _ := s.get(p); code >= 500 {
					t.Errorf("GET %s as %s: %d", p, who, code)
				}
			}
		}
		crawl("a guest")
		s.signIn()
		crawl("the administrator")
		s.signIn() // the crawl went through /logout

		// An article is created and deleted; a user is deleted with content.
		form := func(path string) url.Values {
			_, page := s.get(path)
			field := regexp.MustCompile(`<textarea[^>]*name="(field_\d+)"`).FindStringSubmatch(page)
			if field == nil {
				t.Fatalf("no body field on %s", path)
			}
			return url.Values{"csrf_token": {s.csrf(path)}, "title": {"Module test"}, field[1]: {"<p>Body</p>"}, "status": {"1"}}
		}
		if resp := s.postMultipart("/create/article", form("/create/article")); resp.StatusCode != http.StatusFound {
			t.Fatalf("create: %d", resp.StatusCode)
		}
		var nodeID int
		if err := db.DB.QueryRow("SELECT id FROM nodes WHERE title = 'Module test'").Scan(&nodeID); err != nil {
			t.Fatal(err)
		}
		del := fmt.Sprintf("/node/%d/delete", nodeID)
		if resp := s.post(del, url.Values{"csrf_token": {s.csrf(del)}}); resp.StatusCode >= 400 {
			t.Errorf("delete: %d", resp.StatusCode)
		}
		// To the trash first, then deleted for good from there.
		trash := "/admin/content/trash"
		if resp := s.post(fmt.Sprintf("%s/%d/delete", trash, nodeID), url.Values{"csrf_token": {s.csrf(trash)}}); !strings.Contains(resp.Header.Get("Location"), "success=deleted") {
			t.Errorf("delete for good: %d %s", resp.StatusCode, resp.Header.Get("Location"))
		}
		var left int
		if err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE id = " + fmt.Sprint(nodeID)).Scan(&left); err != nil || left != 0 {
			t.Errorf("the article is still there: %d %v", left, err)
		}

		for _, q := range []string{
			"INSERT INTO users (id, username, slug, password_hash) VALUES (50, 'bob', 'users/bob', 'x')",
			"INSERT INTO nodes (type, title, slug, status, created_by) VALUES ('article', 'Bob post', 'article/bob-post', 1, 50)",
		} {
			if _, err := db.DB.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(http.MethodPost, s.base+"/api/admin/bulk/users", strings.NewReader(`{"action":"delete_with_content","ids":[50]}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", s.csrf("/admin/users"))
		resp, err := s.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if !strings.Contains(string(body), `"processed":1`) {
			t.Errorf("delete a user with content: %d %s", resp.StatusCode, body)
		}
		s.checkLog()
	})
}
