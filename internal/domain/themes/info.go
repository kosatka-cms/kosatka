package themes

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/kosatka-cms/kosatka/internal/buildinfo"
	"github.com/kosatka-cms/kosatka/internal/domain/assets"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// InfoFile - the theme description (regions, default region, its
// settings and description): a theme's at the root of its directory
// (themes/<theme>/theme.yaml), the engine's at the templates root.
const InfoFile = "theme.yaml"

// LoadInfo reads the active theme's theme.yaml (the engine's when
// the theme has none). Read on every call (the admin block pages and the
// cache refresh, not page renders), so a deployed change applies without
// a restart.
func LoadInfo() (entity.ThemeInfo, error) {
	dir := paths.Theme()
	if p, err := manifestPath(dir); err == nil && dir != "" {
		if _, err := os.Stat(p); err != nil {
			dir = ""
		}
	}
	return LoadInfoFor(dir)
}

// manifestPath - theme dir's theme.yaml ("" - the engine's).
func manifestPath(dir string) (string, error) {
	if dir == "" {
		base, err := paths.TemplatesDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, InfoFile), nil
	}
	if dir != filepath.Base(dir) || strings.HasPrefix(dir, ".") {
		return "", fmt.Errorf("theme %q: not a directory's name", dir)
	}
	if found := paths.ThemeDir(dir); found != "" {
		return filepath.Join(found, InfoFile), nil
	}
	root, err := paths.ThemesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, dir, InfoFile), nil
}

// themeDirNames - the directories of every themes root (paths.ThemesDirs:
// the site's, the engine's), each name once: a site's own comes first.
// Hidden directories aren't themes.
func themeDirNames() ([]string, error) {
	var names []string
	seen := map[string]bool{}
	for _, root := range paths.ThemesDirs() {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !seen[e.Name()] {
				seen[e.Name()] = true
				names = append(names, e.Name())
			}
		}
	}
	return names, nil
}

// readManifest - theme dir's theme.yaml as written (no inheritance).
func readManifest(dir string) (entity.ThemeInfo, string, error) {
	var info entity.ThemeInfo
	p, err := manifestPath(dir)
	if err != nil {
		return info, "", err
	}
	f, err := os.Open(p)
	if err != nil {
		return info, p, err
	}
	defer func() { _ = f.Close() }()
	if err := cleanenv.ParseYAML(f, &info); err != nil {
		return info, p, fmt.Errorf("parsing %s: %w", p, err)
	}
	return info, p, nil
}

// faviconRe - a manifest's favicon or logo: a path on the site (not
// another host's, "//…").
var faviconRe = regexp.MustCompile(`^/[A-Za-z0-9._~-][A-Za-z0-9._~/-]*$`)

// stylesheetRe - a stylesheet's name in a manifest (css/<name>.css).
var stylesheetRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Option - a theme the site can switch to (Appearance page): Dir -
// the directory under themes/ ("" - the engine's own
// templates), Name - from its theme.yaml (English, translated by the
// admin pages like the region labels), Base - a subtheme's base theme,
// Regions - how many it declares, Settings - how many settings it offers
// (entity.ThemeSetting).
type Option struct {
	Dir      string
	Name     string
	Base     string
	Regions  int
	Settings int
	// Its description (entity.ThemeInfo): Screenshot - the picture's URL
	// ("" - none); Compatible - the engine release meets its MinVersion
	// (Compatible).
	Description string
	Version     string
	Author      string
	Homepage    string
	License     string
	Tags        []string
	Screenshot  string
	MinVersion  string
	Compatible  bool
	// Source - the repository it was installed from (themes/themes.lock,
	// Lock; "" - shipped with the project); Version is then the tag
	// installed.
	Source string
}

// newOption - the Appearance page's entry of theme dir.
func newOption(dir string, info entity.ThemeInfo) Option {
	name := info.Name
	if name == "" {
		name = dir
	}
	o := Option{Dir: dir, Name: name, Base: info.Base, Regions: len(info.Regions), Settings: len(info.Settings),
		Description: info.Description, Version: info.Version, Author: info.Author, Homepage: info.Homepage,
		License: info.License, Tags: info.Tags, MinVersion: info.MinVersion, Compatible: Compatible(info.MinVersion)}
	if info.Screenshot != "" {
		if dir == "" {
			o.Screenshot = "/static/" + info.Screenshot
		} else {
			o.Screenshot = "/static/themes/" + dir + "/" + info.Screenshot
		}
		o.Screenshot += "?v=" + assets.Version()
	}
	return o
}

// Key - the theme's placement key (entity.ThemeKey), as in its settings'
// address.
func (o Option) Key() string {
	return entity.ThemeKey(o.Dir)
}

// Available lists the engine's own templates and every directory
// under themes/ (the site's and the engine's, themeDirNames) that has a
// valid theme.yaml (a subtheme's with its base theme, LoadInfoFor).
func Available() ([]Option, error) {
	names, err := themeDirNames()
	if err != nil {
		return nil, err
	}
	var out []Option
	if info, err := LoadInfoFor(""); err == nil {
		out = append(out, newOption("", info))
	}
	// The installed themes' source and release (a broken lock - none).
	lock, lockErr := ReadLock()
	if lockErr != nil {
		lock = Lock{}
	}
	for _, name := range names {
		info, err := LoadInfoFor(name)
		if err != nil {
			continue
		}
		o := newOption(name, info)
		if entry, ok := lock.Themes[name]; ok {
			o.Source = entry.Source
			o.Version = strings.TrimPrefix(entry.Version, "v")
		}
		out = append(out, o)
	}
	return out, nil
}

// InvalidTheme - a directory under themes/ that Available leaves out:
// Dir, and Err - why (no theme.yaml, a key it can't take…), its paths
// relative to the project's.
type InvalidTheme struct {
	Dir string
	Err string
}

// Invalid - the directories under themes/ that aren't themes (no
// theme.yaml, or one that fails its checks), each with its error, so a
// theme's author sees why a theme isn't listed (the Appearance page, app
// theme list). Hidden directories aren't themes.
func Invalid() ([]InvalidTheme, error) {
	names, err := themeDirNames()
	if err != nil {
		return nil, err
	}
	var out []InvalidTheme
	for _, name := range names {
		if _, err := LoadInfoFor(name); err != nil {
			msg := err.Error()
			for _, root := range paths.ThemesDirs() {
				msg = strings.ReplaceAll(msg, root+string(os.PathSeparator), "themes/")
			}
			out = append(out, InvalidTheme{Dir: name, Err: msg})
		}
	}
	return out, nil
}

// IsUsable reports whether theme dir can be previewed or set active:
// one of Available that the engine release is compatible with.
func IsUsable(dir string) bool {
	if !IsAvailable(dir) {
		return false
	}
	info, err := LoadInfoFor(dir)
	return err == nil && Compatible(info.MinVersion)
}

// IsAvailable reports whether dir is one of Available ("" - the
// engine's templates - always is).
func IsAvailable(dir string) bool {
	if dir == "" {
		return true
	}
	themes, err := Available()
	if err != nil {
		return false
	}
	for _, t := range themes {
		if t.Dir == dir {
			return true
		}
	}
	return false
}

// LoadInfoFor reads the theme.yaml of the theme in dir ("" - the
// engine's own), whatever theme is active: the blocks admin edits any
// theme's layout. A subtheme (base: <theme>) without regions of its own
// takes its base's; its settings are its base's and its own (its own
// replace the base's with the same ID).
func LoadInfoFor(dir string) (entity.ThemeInfo, error) {
	return loadInfo(dir, 0)
}

func loadInfo(dir string, depth int) (entity.ThemeInfo, error) {
	info, p, err := readManifest(dir)
	if err != nil {
		return info, err
	}
	for _, name := range info.Stylesheets {
		if !stylesheetRe.MatchString(name) {
			return info, fmt.Errorf("%s: stylesheet %q: latin letters, digits, - and _ (css/<name>.css)", p, name)
		}
	}
	if err := checkMeta(info); err != nil {
		return info, fmt.Errorf("%s: %w", p, err)
	}
	if info.Favicon != "" && !faviconRe.MatchString(info.Favicon) {
		return info, fmt.Errorf("%s: favicon %q: a path on the site, /…", p, info.Favicon)
	}
	if info.Logo != "" && !faviconRe.MatchString(info.Logo) {
		return info, fmt.Errorf("%s: logo %q: a path on the site, /…", p, info.Logo)
	}
	if info.Base != "" {
		switch {
		case dir == "":
			return info, fmt.Errorf("%s: the engine's templates have no base theme", p)
		case info.Base == dir:
			return info, fmt.Errorf("%s: base theme %q loops", p, info.Base)
		case depth+2 > paths.MaxThemeChain: // this theme, its base and the themes above it
			return info, fmt.Errorf("%s: base theme %q: a chain of base themes loops or is deeper than %d", p, info.Base, paths.MaxThemeChain)
		}
		base, err := loadInfo(info.Base, depth+1)
		if err != nil {
			return info, fmt.Errorf("%s: base theme %q: %w", p, info.Base, err)
		}
		if len(info.Regions) == 0 {
			info.Regions = base.Regions
			if info.DefaultRegion == "" {
				info.DefaultRegion = base.DefaultRegion
			}
		}
		settings := append([]entity.ThemeSetting(nil), base.Settings...)
		for _, own := range info.Settings {
			if i := slices.IndexFunc(settings, func(s entity.ThemeSetting) bool { return s.ID == own.ID }); i >= 0 {
				settings[i] = own
			} else {
				settings = append(settings, own)
			}
		}
		info.Settings = settings
		if info.Favicon == "" {
			info.Favicon = base.Favicon
		}
		if info.Logo == "" {
			info.Logo = base.Logo
		}
		if len(info.Blocks) == 0 {
			info.Blocks = base.Blocks
		}
	}
	if len(info.Regions) == 0 {
		return info, fmt.Errorf("%s: no regions", p)
	}
	if err := info.CheckSettings(); err != nil {
		return info, fmt.Errorf("%s: %w", p, err)
	}
	if err := info.CheckBlocks(); err != nil {
		return info, fmt.Errorf("%s: %w", p, err)
	}
	if info.DefaultRegion == "" || !info.Has(info.DefaultRegion) {
		info.DefaultRegion = info.Regions[0].ID
	}
	return info, nil
}

// manifests - the manifests' base themes and stylesheets by file,
// read once (page renders ask for them: paths.ThemeChain, themeStylesheets;
// theme.yaml changes apply after a restart, as on a deploy).
var manifests sync.Map

func cachedManifest(dir string) entity.ThemeInfo {
	key, err := manifestPath(dir)
	if err != nil {
		return entity.ThemeInfo{}
	}
	if v, ok := manifests.Load(key); ok {
		return v.(entity.ThemeInfo)
	}
	info, _, err := readManifest(dir)
	if err != nil {
		info = entity.ThemeInfo{}
	}
	manifests.Store(key, info)
	return info
}

// Favicon - the favicon theme dir's own manifest gives (theme.yaml
// "favicon", read once like the base theme; not its base's: the caller
// walks the chain, paths.ThemeChain); "" - none, or not a path on the
// site.
func Favicon(dir string) string {
	if favicon := cachedManifest(dir).Favicon; favicon != "" && faviconRe.MatchString(favicon) {
		return favicon
	}
	return ""
}

// Logo - the logo theme dir's own manifest gives (theme.yaml "logo", as
// Favicon: not its base's, the caller walks the chain); "" - none, or not
// a path on the site.
func Logo(dir string) string {
	if logo := cachedManifest(dir).Logo; logo != "" && faviconRe.MatchString(logo) {
		return logo
	}
	return ""
}

func init() {
	paths.SetBaseThemes(func(dir string) string { return cachedManifest(dir).Base })
}

// Stylesheets - the stylesheets the chain of theme dir declares
// (theme.yaml "stylesheets"), its deepest base's first: each a theme's
// css/<name>.css (assets.ThemeCSS in that theme's chain), after the base theme's
// theme_head (site_styles).
func Stylesheets(dir string) []string {
	chain := paths.ThemeChain(dir)
	var out []string
	for i := len(chain) - 1; i >= 0; i-- {
		for _, name := range cachedManifest(chain[i]).Stylesheets {
			if stylesheetRe.MatchString(name) {
				out = append(out, assets.ThemeCSS(chain[i], name))
			}
		}
	}
	return out
}

// PreviewSessionKey - the user session value naming the theme its
// user previews (set and cleared on the Appearance page, read by
// middleware.ThemePreviewMiddleware).
const PreviewSessionKey = "theme_preview"

var (
	// versionRe - a release in a manifest: "1.2.0" (semver, no "v").
	versionRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	// tagRe - a tag: a lowercase word, digits and "-".
	tagRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	// licenseRe - an SPDX license expression ("MIT",
	// "GPL-2.0-or-later", "MIT OR Apache-2.0").
	licenseRe = regexp.MustCompile(`^[A-Za-z0-9.+-]+( (OR|AND|WITH) [A-Za-z0-9.+-]+)*$`)
	// screenshotRe - a picture in the theme's static directory.
	screenshotRe = regexp.MustCompile(`^([A-Za-z0-9_-]+/)*[A-Za-z0-9_-][A-Za-z0-9._-]*\.(png|jpe?g|webp)$`)
	// engineReleaseRe - the release a build is of (buildinfo.Version, git
	// describe: "v1.2.0", "v1.2.0-3-g1a2b3c4d" - three commits after it).
	engineReleaseRe = regexp.MustCompile(`^v((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))(-|$)`)
)

// maxTags - how many tags a manifest may give.
const maxTags = 20

// checkMeta - whether the manifest's description is well formed:
// releases as "1.2.0", a homepage on http(s), an SPDX license, lowercase
// tags, a screenshot in the theme's static directory.
func checkMeta(info entity.ThemeInfo) error {
	for _, v := range []struct{ key, value string }{{"version", info.Version}, {"min_version", info.MinVersion}} {
		if v.value != "" && !versionRe.MatchString(v.value) {
			return fmt.Errorf("%s %q: a release, 1.2.0", v.key, v.value)
		}
	}
	if info.Homepage != "" {
		u, err := url.Parse(info.Homepage)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("homepage %q: an http(s) address", info.Homepage)
		}
	}
	if info.License != "" && !licenseRe.MatchString(info.License) {
		return fmt.Errorf("license %q: an SPDX identifier (MIT, GPL-2.0-or-later…)", info.License)
	}
	if len(info.Tags) > maxTags {
		return fmt.Errorf("%d tags: at most %d", len(info.Tags), maxTags)
	}
	for _, tag := range info.Tags {
		if !tagRe.MatchString(tag) {
			return fmt.Errorf("tag %q: lowercase latin letters, digits and -", tag)
		}
	}
	if info.Screenshot != "" && !screenshotRe.MatchString(info.Screenshot) {
		return fmt.Errorf("screenshot %q: a .png, .jpg or .webp in the theme's static directory", info.Screenshot)
	}
	return nil
}

// EngineRelease - the release the running engine is of ("1.2.0"; also
// for a build a few commits after it); "" - not a release build (a
// developer's).
func EngineRelease() string {
	return releaseOf(buildinfo.Get().Version)
}

// releaseOf - the release a build version (git describe) is of.
func releaseOf(version string) string {
	if m := engineReleaseRe.FindStringSubmatch(version); m != nil {
		return m[1]
	}
	return ""
}

// Compatible - whether the engine release meets the theme's
// min_version; a build that isn't a release meets every one.
func Compatible(minVersion string) bool {
	return releaseMeets(EngineRelease(), minVersion)
}

// releaseMeets - whether release is minVersion or later ("" - either
// unknown: it does).
func releaseMeets(release, minVersion string) bool {
	if minVersion == "" || release == "" {
		return true
	}
	return compareReleases(release, minVersion) >= 0
}

// compareReleases - -1, 0, 1 as release a is before, the same as or after
// b (both "X.Y.Z").
func compareReleases(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3 && i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// ReadManifestFile - the theme manifest at path (a theme's not yet
// installed: the catalog's builder), checked as LoadInfoFor checks
// an installed one's, but for its base theme (not looked up: it may be
// installed later), so a subtheme needs no regions of its own.
func ReadManifestFile(path string) (entity.ThemeInfo, error) {
	var info entity.ThemeInfo
	f, err := os.Open(path)
	if err != nil {
		return info, err
	}
	defer func() { _ = f.Close() }()
	if err := cleanenv.ParseYAML(f, &info); err != nil {
		return info, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := checkMeta(info); err != nil {
		return info, err
	}
	if info.Favicon != "" && !faviconRe.MatchString(info.Favicon) {
		return info, fmt.Errorf("favicon %q: a path on the site, /…", info.Favicon)
	}
	for _, name := range info.Stylesheets {
		if !stylesheetRe.MatchString(name) {
			return info, fmt.Errorf("stylesheet %q: latin letters, digits, - and _", name)
		}
	}
	if info.Base == "" && len(info.Regions) == 0 {
		return info, fmt.Errorf("no regions")
	}
	if err := info.CheckSettings(); err != nil {
		return info, err
	}
	if len(info.Regions) > 0 {
		if err := info.CheckBlocks(); err != nil {
			return info, err
		}
	}
	return info, nil
}

// ReleaseNewer - whether release tag a ("v1.3.0") is after b.
func ReleaseNewer(a, b string) bool {
	return compareReleases(strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v")) > 0
}
