package service

import (
	"bytes"
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/assets"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// A page includes a module's assets (entity.Module.CSS/JS) only when it
// shows something of the module: the
// module's templates call {{attach "<module>"}} (a plugin block of the
// module attaches it itself, region_blocks.html), which leaves a marker in
// the output; moduleStyles/moduleScripts leave placeholders in <head>.
// After a page renders (resolveModuleAssets) the markers are removed and
// the placeholders become the tags of the attached, enabled modules. The
// markers are HTML comments, so they survive in HTML rendered apart and
// put into a page later (a cached block, a module hook).
const (
	attachMarkerPrefix   = "<!--kosatka:attach "
	attachMarkerSuffix   = "-->"
	moduleCSSPlaceholder = "<!--kosatka:module-css-->"
	moduleJSPlaceholder  = "<!--kosatka:module-js-->"
)

// attachModule - {{attach "<module>"}}: the page needs the module's assets.
func attachModule(module string) template.HTML {
	return template.HTML(attachMarkerPrefix + module + attachMarkerSuffix)
}

// attachPluginBlock - {{attachBlock .PluginID}}: the page shows the
// plugin block (or its lazy placeholder), so it needs its module's assets.
func attachPluginBlock(id any) template.HTML {
	if module, ok := entity.ModuleOfPluginBlock(entity.PluginBlockID(fmt.Sprint(id))); ok {
		return attachModule(module)
	}
	return ""
}

// AttachedModules - the modules html attaches ({{attach}} markers), in
// the order first attached: for a page whose <head> renders apart from
// its content (the Views preview frame).
func AttachedModules(html []byte) []string {
	var out []string
	for rest := html; ; {
		i := bytes.Index(rest, []byte(attachMarkerPrefix))
		if i < 0 {
			return out
		}
		rest = rest[i+len(attachMarkerPrefix):]
		end := bytes.Index(rest, []byte(attachMarkerSuffix))
		if end < 0 {
			return out
		}
		if m := string(rest[:end]); !slices.Contains(out, m) {
			out = append(out, m)
		}
		rest = rest[end+len(attachMarkerSuffix):]
	}
}

// moduleStyles - where the attached modules' stylesheets go.
func moduleStyles() template.HTML { return moduleCSSPlaceholder }

// moduleScripts - where the attached modules' scripts go (deferred, after
// app.js, so they can use its shared functions).
func moduleScripts() template.HTML { return moduleJSPlaceholder }

// resolveModuleAssets removes the attach markers from a rendered page and
// puts the attached, enabled modules' tags (in the registry's order) in
// place of the placeholders: stylesheets non-blocking like
// staticCSSAsync (assets.StaticCSSAsync), scripts deferred.
func resolveModuleAssets(page []byte) []byte {
	if !bytes.Contains(page, []byte("<!--kosatka:")) {
		return page
	}
	attached := map[string]bool{}
	var out bytes.Buffer
	out.Grow(len(page))
	rest := page
	for {
		i := bytes.Index(rest, []byte(attachMarkerPrefix))
		if i < 0 {
			out.Write(rest)
			break
		}
		end := bytes.Index(rest[i+len(attachMarkerPrefix):], []byte(attachMarkerSuffix))
		if end < 0 {
			out.Write(rest)
			break
		}
		out.Write(rest[:i])
		attached[string(rest[i+len(attachMarkerPrefix):i+len(attachMarkerPrefix)+end])] = true
		rest = rest[i+len(attachMarkerPrefix)+end+len(attachMarkerSuffix):]
	}
	var css, js strings.Builder
	for _, m := range entity.Modules {
		if !attached[m.Name] || !ModuleEnabled(m.Name) {
			continue
		}
		for _, f := range m.CSS {
			href := assets.ModuleURL(m.Name, f, "css")
			css.WriteString(`<link rel="preload" href="` + href + `" as="style" onload="this.onload=null;this.rel='stylesheet'">` +
				`<noscript><link rel="stylesheet" href="` + href + `"></noscript>`)
		}
		for _, f := range m.JS {
			js.WriteString(`<script src="` + assets.ModuleURL(m.Name, f, "js") + `" defer></script>`)
		}
	}
	resolved := out.Bytes()
	resolved = bytes.ReplaceAll(resolved, []byte(moduleCSSPlaceholder), []byte(css.String()))
	return bytes.ReplaceAll(resolved, []byte(moduleJSPlaceholder), []byte(js.String()))
}
