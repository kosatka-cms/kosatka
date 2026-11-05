// Package assets - the versioned URLs of the minified static files: the
// engine's (/static/css, /static/js), a theme's (/static/themes/<theme>,
// the first theme of its chain with the file) and a module's
// (/static/modules/<module>). The ?v= of each is the release's asset
// version (Version), so browsers fetch the files again after a deploy.
// The template service (its staticCSS, themeCSS… template funcs) and the
// themes (their stylesheets, a screenshot) use it; it imports neither.
package assets

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/buildinfo"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

var (
	version     string
	versionOnce sync.Once
)

// Version - the static files' version, the ?v= of their URLs: the
// STATIC_ASSET_VERSION environment variable, else the commit the binary
// was built from (a new engine changes it on every site it serves), else
// the working copy's git commit, else "dev". Resolved once.
func Version() string {
	versionOnce.Do(func() {
		if v := strings.TrimSpace(os.Getenv("STATIC_ASSET_VERSION")); v != "" {
			version = v
			return
		}
		if c := buildinfo.Get().ShortCommit(); c != "" {
			version = c
			return
		}
		out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
		if err == nil {
			version = strings.TrimSpace(string(out))
		}
		if version == "" {
			version = "dev"
		}
	})
	return version
}

// StaticCSS returns a versioned URL for a minified stylesheet in /static/css/.
// name is the file basename without extension, e.g. "styles".
func StaticCSS(name string) string {
	name = strings.TrimSuffix(strings.Trim(name, "/"), ".css")
	return fmt.Sprintf("/static/css/%s.min.css?v=%s", name, Version())
}

// ThemeCSS returns a versioned URL for a minified stylesheet of theme,
// /static/themes/<theme>/css/<name>.min.css ("" without a theme): the
// first theme of its chain with the file (a subtheme uses its base's
// stylesheets it doesn't have).
func ThemeCSS(theme, name string) string {
	if theme == "" {
		return ""
	}
	name = strings.TrimSuffix(strings.Trim(name, "/"), ".css")
	if t := paths.ThemeWithStatic(theme, "css/"+name+".css"); t != theme {
		theme = t
	} else {
		theme = paths.ThemeWithStatic(theme, "css/"+name+".min.css")
	}
	return fmt.Sprintf("/static/themes/%s/css/%s.min.css?v=%s", theme, name, Version())
}

// ThemeJS returns a versioned URL for a minified script of theme,
// /static/themes/<theme>/js/<name>.min.js ("" without a theme): the
// first theme of its chain with the file, as ThemeCSS. The ?v= makes
// browsers fetch it again after a deploy.
func ThemeJS(theme, name string) string {
	if theme == "" {
		return ""
	}
	name = strings.TrimSuffix(strings.TrimSuffix(strings.Trim(name, "/"), ".js"), ".min")
	if t := paths.ThemeWithStatic(theme, "js/"+name+".js"); t != theme {
		theme = t
	} else {
		theme = paths.ThemeWithStatic(theme, "js/"+name+".min.js")
	}
	return fmt.Sprintf("/static/themes/%s/js/%s.min.js?v=%s", theme, name, Version())
}

// ThemeCSSAsync - StaticCSSAsync for a theme stylesheet.
func ThemeCSSAsync(theme, name string) template.HTML {
	href := ThemeCSS(theme, name)
	if href == "" {
		return ""
	}
	return template.HTML(
		`<link rel="preload" href="` + href + `" as="style" onload="this.onload=null;this.rel='stylesheet'">` +
			`<noscript><link rel="stylesheet" href="` + href + `"></noscript>`)
}

// StaticJS returns a versioned URL for a minified script in /static/js/.
// name is the file basename without extension, e.g. "app".
func StaticJS(name string) string {
	name = strings.TrimSuffix(strings.Trim(name, "/"), ".js")
	return fmt.Sprintf("/static/js/%s.min.js?v=%s", name, Version())
}

// StaticCSSAsync returns non-render-blocking link tags for a minified stylesheet.
func StaticCSSAsync(name string) template.HTML {
	href := StaticCSS(name)
	return template.HTML(
		`<link rel="preload" href="` + href + `" as="style" onload="this.onload=null;this.rel='stylesheet'">` +
			`<noscript><link rel="stylesheet" href="` + href + `"></noscript>`)
}

// ModuleURL - the versioned URL of a module's minified asset,
// /static/modules/<module>/<file>.min.<ext> (entity.Module.CSS/JS).
func ModuleURL(module, file, ext string) string {
	return fmt.Sprintf("/static/modules/%s/%s.min.%s?v=%s", module, file, ext, Version())
}
