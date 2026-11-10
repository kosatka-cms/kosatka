package app

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/paths"
)

// siteNameRe - a site's name: its directory's, its service instance's.
var siteNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// defaultStart - how add starts a new site ({site} - its name): enabled,
// so it starts with the server too.
const defaultStart = "systemctl enable --now kosatka-site@{site}"

// newSite - what app sites add makes: the site's name and directory, its
// config and secrets, and app install's flags.
type newSite struct {
	Name string
	Port int
	// MetricsPort - the port of its metrics (metrics.listen on
	// 127.0.0.1); 0 - none.
	MetricsPort    int
	URL            string
	DBDriver       string
	DBHost         string
	DBPort         int
	DBName, DBUser string
	DBPassword     string
	Install        []string
}

// runSitesAdd is "app sites add <name> …".
func (h *sitesHost) runSitesAdd(args []string) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(os.Stderr, "usage: app sites add <name> -port <n> -url <address> -db-name <name> -site-name … -admin-name … -admin-email … (app sites add -h)")
		return 2
	}
	s := newSite{Name: args[0]}
	fs := flag.NewFlagSet("sites add", flag.ContinueOnError)
	fs.StringVar(&h.root, "root", h.root, "the sites root: a directory per site")
	fs.IntVar(&s.Port, "port", 0, "the port the site listens on (required; each site its own)")
	fs.IntVar(&s.MetricsPort, "metrics-port", 0, "the port of the site's metrics, /metrics on 127.0.0.1 (metrics.listen; empty - none; each site its own)")
	fs.StringVar(&s.URL, "url", "", "the site's address, e.g. https://blog.example.com (required)")
	fs.StringVar(&s.DBDriver, "db-driver", "mysql", "the database: mysql or postgres")
	fs.StringVar(&s.DBHost, "db-host", "127.0.0.1", "the database host")
	fs.IntVar(&s.DBPort, "db-port", 0, "the database port (empty - the driver's usual)")
	fs.StringVar(&s.DBName, "db-name", "", "the site's database, created beforehand (required)")
	fs.StringVar(&s.DBUser, "db-user", "", "the database user (empty - the site's name)")
	siteName := fs.String("site-name", "", "the site name (required)")
	adminName := fs.String("admin-name", "", "the administrator's username (required)")
	adminEmail := fs.String("admin-email", "", "the administrator's e-mail (required)")
	language := fs.String("language", "en", "the site's language")
	demo := fs.Bool("demo", false, "add demo content")
	modules := fs.String("modules", "", `the site's modules: comma-separated, "all" or "none" (empty - the standard ones)`)
	start := fs.String("start", defaultStart, `how to start the site, {site} - its name ("" - don't)`)
	wait := fs.Duration("wait", time.Minute, "how long the started site may take to answer /healthz")
	owner := fs.String("user", "www-data", "the service's user, owning the site's file directories (when run as root)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	s.DBUser = cmp.Or(s.DBUser, s.Name)
	// The password from the environment, not the command line, which
	// other users of the server see.
	s.DBPassword = os.Getenv("DB_PASSWORD")
	s.Install = []string{"install", "-site-name", *siteName, "-admin-name", *adminName, "-admin-email", *adminEmail, "-language", *language}
	if *demo {
		s.Install = append(s.Install, "-demo")
	}
	if *modules != "" {
		s.Install = append(s.Install, "-modules", *modules)
	}
	if *siteName == "" || *adminName == "" || *adminEmail == "" {
		fmt.Fprintln(os.Stderr, "sites add: -site-name, -admin-name and -admin-email are required")
		return 2
	}
	if err := h.add(context.Background(), s, *start, *wait, *owner); err != nil {
		fmt.Fprintln(os.Stderr, "sites add:", err)
		return 1
	}
	return 0
}

// add makes site s: checks it, writes its directory, installs it, starts
// it (start, {site} - its name; "" - not) and waits for /healthz, then
// prints its nginx site. A failure before the install is done removes the
// directory again.
func (h *sitesHost) add(ctx context.Context, s newSite, start string, wait time.Duration, owner string) error {
	if err := h.checkNewSite(ctx, s); err != nil {
		return err
	}
	dir := filepath.Join(h.root, s.Name)
	if err := writeSiteDir(dir, s); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	_, _ = fmt.Fprintf(h.out, "==> %s: %s\n", s.Name, dir)
	// A release: the site runs this one, through its engine link.
	if h.release != "" {
		if err := setLink(dir, siteEngineLink, h.release); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("its engine link: %w", err)
		}
		_, _ = fmt.Fprintf(h.out, "  release: %s\n", releaseName(h.release))
	}
	site := hostSite{Name: s.Name, Dir: dir}
	out, err := h.run(ctx, site, "", s.Install...)
	h.indent(out)
	if err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("installing it: %w (its directory is removed; the database may hold a part of it - empty it before trying again)", err)
	}
	if err := chownSiteFiles(dir, owner); err != nil {
		return err
	}
	if cmd := siteCommand(start, s.Name); len(cmd) > 0 {
		_, _ = fmt.Fprintf(h.out, "  start: %s\n", strings.Join(cmd, " "))
		out, err := h.command(ctx, cmd[0], cmd[1:]...)
		h.indent(out)
		if err != nil {
			return fmt.Errorf("starting it: %w", err)
		}
		if err := h.waitHealthy(ctx, s.Port, wait); err != nil {
			return err
		}
	}
	// The engine's static files through the site's engine link (a release):
	// they follow it to each release it moves to.
	engine := h.engine
	if h.release != "" {
		engine = filepath.Join(dir, siteEngineLink)
	}
	_, _ = fmt.Fprintf(h.out, "\nThe nginx site (see the guide: docs/guides/multisite.md); its certificate: certbot certonly --nginx -d %s\n\n%s", hostOf(s.URL), nginxSite(engine, dir, s))
	return nil
}

// checkNewSite - whether s can be made: a valid name not taken, its
// address, a database, a port no other site has and nothing listens on,
// the database password.
func (h *sitesHost) checkNewSite(ctx context.Context, s newSite) error {
	if !siteNameRe.MatchString(s.Name) {
		return fmt.Errorf("site name %q: lower-case letters, digits and hyphens", s.Name)
	}
	if _, err := os.Stat(filepath.Join(h.root, s.Name)); err == nil {
		return fmt.Errorf("%s exists already", filepath.Join(h.root, s.Name))
	}
	if u, err := url.Parse(s.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("-url %q: an http(s) address, e.g. https://blog.example.com", s.URL)
	}
	if s.DBDriver != "mysql" && s.DBDriver != "postgres" {
		return fmt.Errorf("-db-driver %q: mysql or postgres", s.DBDriver)
	}
	if s.DBName == "" {
		return errors.New("-db-name is required: create the site's database and user first")
	}
	if s.DBPassword == "" {
		return errors.New("DB_PASSWORD is empty: give the database password in the environment (DB_PASSWORD=… app sites add …)")
	}
	if s.Port <= 0 || s.Port > 65535 {
		return errors.New("-port is required: the port the site listens on, each site its own")
	}
	if err := os.MkdirAll(h.root, 0o755); err != nil {
		return fmt.Errorf("the sites root: %w", err)
	}
	sites, err := h.sites(nil)
	if err != nil {
		return err
	}
	if s.MetricsPort < 0 || s.MetricsPort > 65535 || (s.MetricsPort != 0 && s.MetricsPort == s.Port) {
		return fmt.Errorf("-metrics-port %d: a port of its own, not the site's", s.MetricsPort)
	}
	for _, other := range sites {
		info, err := h.siteInfo(ctx, other)
		if err != nil {
			continue
		}
		taken := []int{info.Port}
		if p := listenPort(info.MetricsListen); p > 0 {
			taken = append(taken, p)
		}
		for _, p := range taken {
			if p == s.Port || (s.MetricsPort != 0 && p == s.MetricsPort) {
				return fmt.Errorf("port %d is site %s's", p, other.Name)
			}
		}
	}
	for _, p := range []int{s.Port, s.MetricsPort} {
		if p == 0 {
			continue
		}
		l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
		if err != nil {
			return fmt.Errorf("port %d is in use", p)
		}
		_ = l.Close()
	}
	return nil
}

// listenPort - the port of an address like 127.0.0.1:9101, 0 when there
// is none.
func listenPort(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return p
}

// writeSiteDir writes site s's directory: its config, its secrets (a
// generated session key; readable by root only - systemd reads them
// before dropping to the service's user), its themes and file
// directories.
func writeSiteDir(dir string, s newSite) error {
	for _, d := range []struct {
		path string
		mode os.FileMode
	}{{dir, 0o755}, {filepath.Join(dir, "themes"), 0o755}, {filepath.Join(dir, paths.DefaultPublicFilesDir), 0o755}, {filepath.Join(dir, "private"), 0o750}} {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return err
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	// The key the secrets in its database are sealed with (SECRETS_KEY).
	secretsKey, err := newSecretsKey()
	if err != nil {
		return err
	}
	cfg := fmt.Sprintf("# The site %s of this engine (app sites add); its secrets are in site.env.\nenv: prod\nport: %d\npublic_base_url: %s\ndb:\n  driver: %s\n  host: %s\n", s.Name, s.Port, s.URL, s.DBDriver, s.DBHost)
	if s.DBPort > 0 {
		cfg += fmt.Sprintf("  port: %d\n", s.DBPort)
	}
	cfg += fmt.Sprintf("  scheme: %s\n", s.DBName)
	if s.MetricsPort > 0 {
		cfg += fmt.Sprintf("metrics:\n  listen: 127.0.0.1:%d\n", s.MetricsPort)
	}
	if err := os.WriteFile(filepath.Join(dir, siteConfigFile), []byte(cfg), 0o644); err != nil {
		return err
	}
	env := fmt.Sprintf("# The site %s's secrets (app sites add): its service reads them (EnvironmentFile).\nDB_USER=%s\nDB_PASSWORD=%s\nSESSION_SECRET_KEY=%s\nSECRETS_KEY=%s\n",
		s.Name, envQuote(s.DBUser), envQuote(s.DBPassword), hex.EncodeToString(key), secretsKey)
	return os.WriteFile(filepath.Join(dir, siteEnvFile), []byte(env), 0o600)
}

// envQuote - v as an env file value: double-quoted, " and \ escaped.
func envQuote(v string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
}

// chownSiteFiles gives the site's file directories (and what the install
// wrote in them) to owner, the service's user - when run as root; as
// anyone else they are the caller's already.
func chownSiteFiles(dir, owner string) error {
	if os.Geteuid() != 0 || owner == "" {
		return nil
	}
	u, err := user.Lookup(owner)
	if err != nil {
		return fmt.Errorf("the service's user: %w", err)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	for _, root := range []string{filepath.Join(dir, paths.DefaultPublicFilesDir), filepath.Join(dir, "private")} {
		if err := filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Lchown(p, uid, gid)
		}); err != nil {
			return fmt.Errorf("giving %s to %s: %w", root, owner, err)
		}
	}
	return nil
}

// waitHealthy waits up to wait for the site on port to answer /healthz.
func (h *sitesHost) waitHealthy(ctx context.Context, port int, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		err := h.healthy(ctx, port)
		if err == nil {
			_, _ = fmt.Fprintf(h.out, "  healthy: %s\n", healthURL(port))
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("not healthy after %s: %w", wait, err)
		}
		h.pause(time.Second)
	}
}

// hostOf - address's host.
func hostOf(address string) string {
	u, _ := url.Parse(address)
	if u == nil {
		return address
	}
	return u.Hostname()
}

// nginxSite - site s's nginx server block over HTTPS (dir - its
// directory, engine - the engine's, or its engine link): its static and
// public files from disk, its themes and everything else from its port.
func nginxSite(engine, dir string, s newSite) string {
	host := hostOf(s.URL)
	app := "http://127.0.0.1:" + strconv.Itoa(s.Port)
	proxy := "        proxy_pass " + app + ";\n        proxy_http_version 1.1;\n        proxy_set_header Host $host;\n        proxy_set_header X-Real-IP $remote_addr;\n        proxy_set_header X-Forwarded-For $remote_addr;\n        proxy_set_header X-Forwarded-Proto $scheme;\n"
	return `server {
    listen 80;
    listen [::]:80;
    server_name ` + host + `;
    location / {
        return 301 https://` + host + `$request_uri;
    }
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name ` + host + `;

    ssl_certificate     /etc/letsencrypt/live/` + host + `/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/` + host + `/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    client_max_body_size 16m;

    # The site's themes: its own, else the engine's.
    location ^~ /static/themes/ {
` + proxy + `    }

    # The engine's and the modules' static files, every site's.
    location ^~ /static/ {
        root ` + filepath.Join(engine, "web") + `;
        try_files $uri @app;
        add_header Cache-Control "public, max-age=31536000, immutable";
    }

    # The site's public files, as the application serves them: no
    # dotfiles; images sandboxed, PDF shown (a sandbox would stop the
    # browser's viewer), anything else downloaded.
    location ^~ ` + paths.DefaultPublicFilesURL + ` {
        root ` + filepath.Join(dir, "web") + `;
        location ~ /\. {
            return 404;
        }
        location ~* \.(jpe?g|png|gif|webp)$ {
            try_files $uri @app;
            add_header Cache-Control "public, max-age=31536000";
            add_header X-Content-Type-Options "nosniff" always;
            add_header Content-Security-Policy "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox" always;
        }
        location ~* \.pdf$ {
            try_files $uri @app;
            add_header Cache-Control "public, max-age=31536000";
            add_header X-Content-Type-Options "nosniff" always;
        }
        try_files $uri @app;
        add_header Cache-Control "public, max-age=31536000";
        add_header X-Content-Type-Options "nosniff" always;
        add_header Content-Security-Policy "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox" always;
        add_header Content-Disposition "attachment" always;
    }

    location / {
` + proxy + `    }

    location @app {
` + proxy + `    }
}
`
}
