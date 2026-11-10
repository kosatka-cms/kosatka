package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// addFixture - a host over an empty sites root whose engine runs answer
// with install (an output starting with "ERR " fails), recording the
// commands it runs; the new site answers /healthz.
func addFixture(t *testing.T, install string) (*sitesHost, *[][]string, *[]string) {
	t.Helper()
	root := t.TempDir()
	var runs [][]string
	var commands []string
	h := &sitesHost{engine: "/opt/kosatka", root: root, out: &strings.Builder{}, pause: func(time.Duration) {}}
	h.run = func(_ context.Context, s hostSite, _ string, args ...string) (string, error) {
		runs = append(runs, append([]string{s.Name}, args...))
		if args[0] == "config" {
			return `{"port": 8101, "metrics_listen": "127.0.0.1:9101"}`, nil
		}
		if rest, failed := strings.CutPrefix(install, "ERR "); failed {
			return rest, errors.New("exit status 1")
		}
		return install, nil
	}
	h.command = func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return "", nil
	}
	h.status = func(context.Context, string) (int, error) { return 200, nil }
	return h, &runs, &commands
}

// testNewSite - a valid new site on port.
func testNewSite(name string, port int) newSite {
	return newSite{Name: name, Port: port, URL: "https://" + name + ".example.com", DBDriver: "postgres", DBHost: "127.0.0.1",
		DBName: name, DBUser: name, DBPassword: `pa"ss\word`, Install: []string{"install", "-site-name", "Shop", "-admin-name", "admin", "-admin-email", "admin@example.com", "-language", "en"}}
}

// TestSitesAdd - the site's directory, config and secrets, its install
// through the engine, its start and the nginx site.
func TestSitesAdd(t *testing.T) {
	h, runs, commands := addFixture(t, "install: done\n")
	if err := h.add(context.Background(), testNewSite("shop", 18732), defaultStart, time.Second, ""); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(h.root, "shop")
	cfg, _ := os.ReadFile(filepath.Join(dir, siteConfigFile))
	for _, want := range []string{"port: 18732", "public_base_url: https://shop.example.com", "driver: postgres", "scheme: shop"} {
		if !strings.Contains(string(cfg), want) {
			t.Errorf("site.yaml lacks %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(string(cfg), "metrics:") {
		t.Errorf("metrics without -metrics-port:\n%s", cfg)
	}
	envFile := filepath.Join(dir, siteEnvFile)
	if fi, err := os.Stat(envFile); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("site.env mode %v, %v; want 0600", fi.Mode().Perm(), err)
	}
	f, _ := os.Open(envFile)
	vars, err := parseEnvFile(f)
	_ = f.Close()
	got := strings.Join(vars, "\n")
	if err != nil || !strings.Contains(got, `DB_PASSWORD=pa"ss\word`) || !strings.Contains(got, "DB_USER=shop") || !strings.Contains(got, "SESSION_SECRET_KEY=") || !strings.Contains(got, "SECRETS_KEY=") {
		t.Errorf("site.env reads %q, %v", vars, err)
	}
	for _, sub := range []string{"themes", "private", "web/uploads"} {
		if fi, err := os.Stat(filepath.Join(dir, sub)); err != nil || !fi.IsDir() {
			t.Errorf("no %s: %v", sub, err)
		}
	}
	if want := []string{"shop", "install", "-site-name", "Shop", "-admin-name", "admin", "-admin-email", "admin@example.com", "-language", "en"}; !reflect.DeepEqual((*runs)[0], want) {
		t.Errorf("ran %q, want %q", (*runs)[0], want)
	}
	if want := []string{"systemctl enable --now kosatka-site@shop"}; !reflect.DeepEqual(*commands, want) {
		t.Errorf("commands %q, want %q", *commands, want)
	}
	out := h.out.(*strings.Builder).String()
	for _, want := range []string{"server_name shop.example.com;", "proxy_pass http://127.0.0.1:18732;", "root " + filepath.Join(dir, "web") + ";", "healthy: http://127.0.0.1:18732/healthz"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

// TestSitesAddChecks - what add refuses before writing anything.
func TestSitesAddChecks(t *testing.T) {
	h, _, _ := addFixture(t, "")
	if err := os.MkdirAll(filepath.Join(h.root, "blog"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, "blog", siteConfigFile), []byte("port: 8101\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, mod := range map[string]func(*newSite){
		"a bad name":                     func(s *newSite) { s.Name = "Shop!" },
		"a taken name":                   func(s *newSite) { s.Name = "blog" },
		"no address":                     func(s *newSite) { s.URL = "shop.example.com" },
		"another database":               func(s *newSite) { s.DBDriver = "sqlite" },
		"no database":                    func(s *newSite) { s.DBName = "" },
		"no password":                    func(s *newSite) { s.DBPassword = "" },
		"no port":                        func(s *newSite) { s.Port = 0 },
		"another site's port":            func(s *newSite) { s.Port = 8101 },
		"metrics on its port":            func(s *newSite) { s.MetricsPort = s.Port },
		"another site's metrics port":    func(s *newSite) { s.MetricsPort = 9101 },
		"metrics on another site's port": func(s *newSite) { s.MetricsPort = 8101 },
		"a metrics port out of range":    func(s *newSite) { s.MetricsPort = 70000 },
	} {
		s := testNewSite("shop", 18733)
		mod(&s)
		if err := h.add(context.Background(), s, "", time.Second, ""); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := os.Stat(filepath.Join(h.root, "shop")); err == nil {
		t.Error("a refused site left its directory")
	}
}

// TestSitesAddInstallFails - a failed install removes the directory and
// starts nothing.
func TestSitesAddInstallFails(t *testing.T) {
	h, _, commands := addFixture(t, "ERR install: the database has the core already\n")
	err := h.add(context.Background(), testNewSite("shop", 18734), defaultStart, time.Second, "")
	if err == nil || !strings.Contains(err.Error(), "installing it") {
		t.Fatalf("error %v", err)
	}
	if _, err := os.Stat(filepath.Join(h.root, "shop")); err == nil {
		t.Error("the directory stays after a failed install")
	}
	if len(*commands) != 0 {
		t.Errorf("started %q after a failed install", *commands)
	}
}

// TestNginxSiteUploads - the nginx site serves the uploads as the
// application does: images sandboxed, PDF shown without a sandbox (the
// browser's viewer), anything else downloaded and sandboxed.
func TestNginxSiteUploads(t *testing.T) {
	out := nginxSite("/opt/kosatka/engine", "/opt/kosatka/sites/shop", testNewSite("shop", 18733))
	block := func(start string) string {
		t.Helper()
		i := strings.Index(out, start)
		if i < 0 {
			t.Fatalf("no %q in:\n%s", start, out)
		}
		return out[i : i+strings.Index(out[i:], "}")]
	}
	if images := block(`location ~* \.(jpe?g|png|gif|webp)$ {`); !strings.Contains(images, "sandbox") || !strings.Contains(images, "nosniff") {
		t.Errorf("images without the sandbox:\n%s", images)
	}
	if pdf := block(`location ~* \.pdf$ {`); strings.Contains(pdf, "sandbox") || !strings.Contains(pdf, "nosniff") {
		t.Errorf("PDF sandboxed, or without nosniff:\n%s", pdf)
	}
	if !strings.Contains(out, `add_header Content-Disposition "attachment" always;`) {
		t.Error("other files not downloaded")
	}
}

// TestSitesAddMetricsPort - -metrics-port writes metrics.listen on the
// loopback address.
func TestSitesAddMetricsPort(t *testing.T) {
	h, _, _ := addFixture(t, "install: done\n")
	s := testNewSite("shop", 18735)
	s.MetricsPort = 18736
	if err := h.add(context.Background(), s, "", time.Second, ""); err != nil {
		t.Fatal(err)
	}
	cfg, _ := os.ReadFile(filepath.Join(h.root, "shop", siteConfigFile))
	if !strings.Contains(string(cfg), "metrics:\n  listen: 127.0.0.1:18736\n") {
		t.Errorf("site.yaml lacks metrics.listen:\n%s", cfg)
	}
}

func TestListenPort(t *testing.T) {
	for addr, want := range map[string]int{"127.0.0.1:9101": 9101, ":9102": 9102, "": 0, "localhost": 0, "[::1]:9103": 9103} {
		if got := listenPort(addr); got != want {
			t.Errorf("listenPort(%q) = %d, want %d", addr, got, want)
		}
	}
}
