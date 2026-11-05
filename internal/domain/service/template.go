package service

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/lib/htmlimages"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TemplateService - the HTML pages and fragments rendered from the
// templates (the request's theme's copy of each, else the engine's).
type TemplateService interface {
	Render(w io.Writer, name string, data any, r *http.Request, funcMap template.FuncMap) error
	// RenderNamed renders a named block from the given template files in
	// isolation (no layout), for AJAX fragments. Shares Render's func map.
	RenderNamed(w io.Writer, blockName string, data any, templateFiles ...string) error
}

// DefaultTemplateService - the TemplateService on html/template, the
// parsed template sets cached (cacheEnabled).
type DefaultTemplateService struct {
	contentProcessor *htmlimages.ContentProcessor
	// integrations - the counters and verification code settings for the
	// layout (see SetIntegrations, template func "integrations").
	integrations *IntegrationSettingsService
	// dateFormats - the date format catalog (fieldDate).
	dateFormats *DateFormatService
	// themeSettings - the themes' settings (themeSetting,
	// themeSettingsStyle).
	themeSettings *themes.SettingsService
	// debug - template debugging (SetDebug, template_debug.go).
	debug bool

	// cacheEnabled turns on the parsed-template cache. Disable it in local
	// development so template edits apply without a restart; in production
	// a deploy's restart clears it, and ResetCache a theme change.
	cacheEnabled bool
	cacheMu      sync.RWMutex
	cache        map[string]*template.Template
}

// NewTemplateService constructs the template renderer. cacheEnabled should be
// true everywhere except local development (see the cacheEnabled field).
func NewTemplateService(cacheEnabled bool) TemplateService {
	return &DefaultTemplateService{
		cacheEnabled: cacheEnabled,
		cache:        make(map[string]*template.Template),
	}
}

// ResetCache drops the parsed template sets, e.g. after the active theme
// changes (templates resolve to another directory).
func (t *DefaultTemplateService) ResetCache() {
	t.cacheMu.Lock()
	t.cache = make(map[string]*template.Template)
	t.cacheMu.Unlock()
}

// SetIntegrations gives the layout the counters/verification/raw code
// settings (template func "integrations").
func (t *DefaultTemplateService) SetIntegrations(s *IntegrationSettingsService) {
	t.integrations = s
}

// SetThemeSettings gives the templates the themes' settings (wiring).
func (t *DefaultTemplateService) SetThemeSettings(s *themes.SettingsService) {
	t.themeSettings = s
}

// SetDateFormats gives the templates the date format catalog (wiring).
func (t *DefaultTemplateService) SetDateFormats(s *DateFormatService) {
	t.dateFormats = s
}

// SetContentProcessor gives the templates the processing of the images in
// a text (template funcs "processContent", "processText"; wiring).
func (t *DefaultTemplateService) SetContentProcessor(processor *htmlimages.ContentProcessor) {
	t.contentProcessor = processor
}

// Render writes the page of template path (the template named as its file)
// with data: in the site's or the admin area's layout and the shared
// partials, the request's theme and language, its embeds and the modules'
// assets resolved.
func (t *DefaultTemplateService) Render(
	w io.Writer,
	path string,
	data any,
	r *http.Request,
	funcMap template.FuncMap,
) error {
	pathParts := strings.Split(path, "/")
	tmplName := pathParts[len(pathParts)-1]

	theme := requestTheme(r)
	tmpl, err := t.getOrParseFor(path, funcMap, requestZone(r), requestLang(r), theme, func() ([]string, error) {
		return t.pageFilesToParseIn(path, pathParts, theme)
	})
	if err != nil {
		return err
	}
	if m, ok := data.(map[string]any); ok && tokenInURL(r) {
		m["tokenInURL"] = true
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return err
	}
	out := buf.Bytes()
	// What the texts and templates embed, as the request's visitor sees
	// it; before the assets: an embedded thing may attach its own.
	if r != nil {
		out = ResolveEmbeds(r.Context(), out)
	}
	if t.debug {
		out = debugPageComment(out, path, theme)
	}
	_, err = w.Write(resolveModuleAssets(out))
	return err
}

// tokenInURL - whether r's address carries a secret: a {token} in its
// route or a token parameter (a password reset link, a preview, an
// unsubscribe link). Such a page runs no tracking code (tokenInURL in the
// page's data): the counters would send the address on.
func tokenInURL(r *http.Request) bool {
	return r != nil && (r.PathValue("token") != "" || r.URL.Query().Has("token"))
}

// pageFilesToParse builds the full file list for a page render at path: the
// layout, every shared partial and the page's own template. Called only on
// a cache miss in getOrParse.
func (t *DefaultTemplateService) pageFilesToParse(path string, pathParts []string) ([]string, error) {
	return t.pageFilesToParseIn(path, pathParts, themeScope{})
}

// sharedTemplates - the templates every page is parsed with after its
// layout, in this order (a later define of a name wins), each the theme's
// copy when it has one (pageFilesToParseIn).
var sharedTemplates = []string{
	"footer.html",
	"header.html",
	// The engine's HTML document (html_start, html_scripts,
	// html_end, admin_top_bar) wrapped by the theme's header/footer.
	"partials/html.html",
	"pagination.html",
	"sidebar.html",
	// "block_placeholder_<region>" defines for lazy
	// block placements (empty in the engine; a theme provides its own).
	"partials/block_placeholders.html",
	"comments.html",
	"blocks/logo.html",
	"blocks/user_menu.html",
	"blocks/search_form.html",
	"blocks/last_comments.html",
	"blocks/tag_cloud.html",
	"blocks/copyright.html",
	"social_share.html",
	"article/meta.html",
	"node/edit_node_settings.html",
	"node/edit_scripts.html",
	"user/tabs/overview.html",
	"user/tabs/files.html",
	// A profile's header and tabs (profile_head), the
	// profile page's and its edit form's.
	"partials/profile_head.html",
	"user/social_login.html",
	"admin/header.html",
	"admin/footer.html",
	"admin/content-types/tabs.html",
	// The Views page's {{define}} blocks, shared between
	// the full page render and RenderNamed's AJAX fragment responses.
	"admin/views/fragments.html",
	// The Views page's modals (settings, filter, field, sort).
	"admin/views/settings_modal.html",
	"admin/views/filter_modal.html",
	"admin/views/field_modal.html",
	"admin/views/sort_modal.html",
	"partials/tinymce_user_files.html",
	// Shared link type + target field widget,
	// used by admin/menus/links.html and admin/menus/link_edit.html.
	"admin/menus/link_target_fields.html",
	// Shared content type checkboxes for
	// every block placement edit page.
	"admin/blocks/visibility_content_types.html",
	// The placement's scope: everywhere, the
	// front page, content types or pages.
	"admin/blocks/visibility_scope.html",
	// The placement roles checkboxes.
	"admin/blocks/visibility_roles.html",
	"errors/not_found.html",
	"partials/comment_list_item.html",
	"partials/breadcrumbs.html",
	"partials/seo_tokens_help.html",
	// The SEO forms' schema.org rows (seo_schema_fields).
	"admin/seo/schema_fields.html",
	"partials/node_module_links.html",
	// The node page's shared parts (moderation notice,
	// status class, actions) for the modules' node pages too.
	"partials/node_parts.html",
	// Renders a region's published block placements;
	// used by sidebar.html at each region point.
	"partials/region_blocks.html",
	// The theme_head/theme_editor_css hooks (engine
	// defaults are empty; the theme's own copy replaces them).
	"partials/theme_head.html",
	// Dynamic/page.html's attachment point for View
	// attachments.
	"partials/view_attachments.html",
	// The required profile fields' inputs of the
	// forms creating a user.
	"partials/required_user_fields.html",
	// The password rules checklist of the forms
	// setting a password.
	"partials/password_strength.html",
	// A form's reCAPTCHA widget (recaptcha).
	"partials/recaptcha.html",
	// The admin pages' result notices (admin_alerts).
	"admin/alerts.html",
	// The admin's row action icons (admin_icon_edit,
	// admin_icon_delete, admin_icon_copy).
	"admin/icons.html",
	// The admin delete confirmation pages'
	// markup (admin_confirm_delete).
	"admin/confirm_delete.html",
	// An upload's extensions and size inputs
	// (admin_upload_extensions, admin_upload_max_size).
	"admin/upload_limits.html",
	// How a link field shows: its formatter and
	// the layout of several values (field_formatter_controls), the
	// "Detail page" tab's and the View field's.
	"admin/field_formatter.html",
	// What a content revision changed
	// (revision_summary): the history's and the recent changes' rows.
	"admin/revision_summary.html",
	// A paged admin list's foot (admin_list_foot).
	"admin/list_foot.html",
	// The Mail page's tabs (admin_mail_tabs).
	"admin/config/mail_tabs.html",
	// The Interface translations page's tabs and description
	// (admin_translations_tabs, admin_translations_meta).
	"admin/config/translations_tabs.html",
	// The layouts' content row (main_column).
	"partials/main_column.html",
}

// pageFilesToParseIn - pageFilesToParse in theme's templates.
func (t *DefaultTemplateService) pageFilesToParseIn(path string, pathParts []string, theme themeScope) ([]string, error) {
	// resolve - the theme's copy of a template when it has one (see
	// paths.ResolveTemplate), else the engine's.
	var resolveErr error
	resolve := func(rel string) string {
		p, err := theme.resolve(rel)
		if err != nil && resolveErr == nil {
			resolveErr = err
		}
		return p
	}

	var tmplPath = resolve(path)

	isAdmin := len(pathParts) > 1 && pathParts[0] == "admin"

	var layoutFile string
	if isAdmin {
		layoutFile = resolve("admin/layout.html")
	} else {
		layoutFile = resolve("layout.html")
	}

	filesToParse := []string{layoutFile}
	for _, rel := range sharedTemplates {
		filesToParse = append(filesToParse, resolve(rel))
	}
	// teaser/*.html - the theme's teaser cards (one per content type plus
	// default.html), picked by renderTeaser; page/*.html - per-type node page
	// layouts and inserts (renderSuggestion/renderOptional with prefixes
	// "page"/"page_extra"); fields/*.html - field value templates
	// ("field_<type>_<machine name>" overrides), also added by RenderNamed;
	// component/*.html - the theme's blocks of a field set ("component_<set>").
	for _, dir := range []string{"teaser", "page", "fields", "component"} {
		themeFiles, err := theme.glob(filepath.Join(dir, "*.html"))
		if err != nil {
			return nil, err
		}
		filesToParse = append(filesToParse, themeFiles...)
	}
	// The modules' plugin block partials (block_<ID>, see plugin_block)
	// and partials (e.g. page_end_<module>, see moduleHook).
	for _, dir := range []string{"blocks", "partials"} {
		moduleFiles, err := theme.moduleGlob(filepath.Join(dir, "*.html"))
		if err != nil {
			return nil, err
		}
		filesToParse = append(filesToParse, moduleFiles...)
	}
	// The theme's own partials - a file of its chain's partials/ that
	// replaces none of the engine's or the modules' (those are above), for
	// its templates to call.
	themePartials, err := theme.glob(filepath.Join("partials", "*.html"))
	if err != nil {
		return nil, err
	}
	listed := make(map[string]bool, len(filesToParse))
	for _, f := range filesToParse {
		listed[f] = true
	}
	for _, f := range themePartials {
		if !listed[f] {
			filesToParse = append(filesToParse, f)
			listed[f] = true
		}
	}
	filesToParse = append(filesToParse, tmplPath)
	if resolveErr != nil {
		return nil, resolveErr
	}

	return filesToParse, nil
}

// RenderPageBlock executes one block (e.g. "content") of a page template
// parsed with everything Render parses for it (layout, partials, the
// modules' blocks) - for a module's tests of its pages.
func (t *DefaultTemplateService) RenderPageBlock(w io.Writer, page, block string, data any) error {
	tmpl, err := t.getOrParse("page-block:"+page, nil, func() ([]string, error) {
		return t.pageFilesToParse(page, strings.Split(page, "/"))
	})
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, block, data)
}

// RenderNamed parses the given template files on their own (without the page
// layout and partials Render pulls in) and executes one named block from
// them - for small AJAX fragments. It shares buildFuncMap with Render, and
// the field value templates (fields/*.html), the modules' plugin block
// partials and their partials (moduleHook) are always parsed along.
// Module assets: see resolveModuleAssets.
func (t *DefaultTemplateService) RenderNamed(w io.Writer, blockName string, data any, templateFiles ...string) error {
	cacheKey := "named:" + blockName + "\x00" + strings.Join(templateFiles, "\x00")

	theme := dataTheme(data)
	tmpl, err := t.getOrParseFor(cacheKey, nil, dataZone(data), dataLang(data), theme, func() ([]string, error) {
		tmplPaths := make([]string, len(templateFiles))
		for i, f := range templateFiles {
			p, err := theme.resolve(f)
			if err != nil {
				return nil, err
			}
			tmplPaths[i] = p
		}
		fields, err := theme.glob(filepath.Join("fields", "*.html"))
		if err != nil {
			return nil, err
		}
		// The modules' blocks and partials, as a page has them: a part
		// rendered alone (the comments' list of /api/comments) calls
		// moduleHook too.
		files := append(tmplPaths, fields...)
		for _, dir := range []string{"blocks", "partials"} {
			moduleFiles, err := theme.moduleGlob(filepath.Join(dir, "*.html"))
			if err != nil {
				return nil, err
			}
			files = append(files, moduleFiles...)
		}
		return files, nil
	})
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, blockName, data); err != nil {
		return err
	}
	out := buf.Bytes()
	// A fragment keeps its attach markers (they reach the page it is put
	// in); one with the asset placeholders (a page's <head>) gets them
	// resolved.
	if bytes.Contains(out, []byte(moduleCSSPlaceholder)) || bytes.Contains(out, []byte(moduleJSPlaceholder)) {
		out = resolveModuleAssets(out)
	}
	_, err = w.Write(out)
	return err
}

// getOrParse returns a parsed template for cacheKey, from cache when caching is
// enabled and funcMap is nil (a per-call func map bypasses the cache). On a
// miss, buildFiles supplies the file list to parse.
//
// The cache is an in-process map with no TTL or invalidation: it lives as long
// as the process, so deployments must restart the service after updating
// templates.
func (t *DefaultTemplateService) getOrParse(cacheKey string, funcMap template.FuncMap, buildFiles func() ([]string, error)) (*template.Template, error) {
	return t.getOrParseIn(cacheKey, funcMap, nil, "", buildFiles)
}

// getOrParseIn - getOrParse for a time zone and language: a user's own zone
// (pkg/sitetime) or a non-default language (pkg/i18n) gets its own cached
// template set with time and translation funcs bound to it; nil zone and
// "" lang mean the site's defaults (the shared set).
func (t *DefaultTemplateService) getOrParseIn(cacheKey string, funcMap template.FuncMap, zone *time.Location, lang string, buildFiles func() ([]string, error)) (*template.Template, error) {
	return t.getOrParseFor(cacheKey, funcMap, zone, lang, themeScope{}, buildFiles)
}

// getOrParseFor - getOrParseIn in a theme: a previewed theme (set) gets
// its own cached template sets, with its stylesheets and translations.
func (t *DefaultTemplateService) getOrParseFor(cacheKey string, funcMap template.FuncMap, zone *time.Location, lang string, theme themeScope, buildFiles func() ([]string, error)) (*template.Template, error) {
	if theme.set {
		cacheKey += "\x00theme=" + entity.ThemeKey(theme.dir)
	}
	if zone != nil {
		cacheKey += "\x00tz=" + zone.String()
	}
	if lang != "" {
		cacheKey += "\x00lang=" + lang
	}
	useCache := t.cacheEnabled && funcMap == nil

	if useCache {
		t.cacheMu.RLock()
		tmpl, ok := t.cache[cacheKey]
		t.cacheMu.RUnlock()
		if ok {
			return tmpl, nil
		}
	}

	files, err := buildFiles()
	if err != nil {
		return nil, err
	}

	// renderBlock captures a {{template "name" data}} call's output into a
	// string (used by dynamic_list_field_value's RewriteHTML wrapping). The
	// closure references `self`, which is assigned after ParseFiles succeeds;
	// it is only dereferenced during Execute, when it is always set.
	var self *template.Template
	fm := t.buildFuncMapFor(funcMap, zone, lang, theme)
	fm["renderBlock"] = func(name string, data any) (template.HTML, error) {
		var buf bytes.Buffer
		if err := self.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil
	}
	// viewGroups - a View's rows split into groups by a grouping field
	// (ViewDisplayDef.GroupByField), see groupViewRowsIn. A non-date heading is
	// rendered via dynamic_list_field_value_raw. field/dateFormat are any:
	// nil (no grouping keys in the data) means no grouping.
	fm["viewGroups"] = func(items, field, dateFormat any) ([]ViewRowGroup, error) {
		f, _ := field.(string)
		df, _ := dateFormat.(string)
		l := lang
		if l == "" {
			l = i18n.Default()
		}
		return groupViewRowsIn(items, f, df, l, func(v time.Time) time.Time {
			if zone != nil {
				return v.In(zone)
			}
			return sitetime.In(v)
		}, func(f entity.FieldInstanceDef, value, item any) (template.HTML, error) {
			var buf bytes.Buffer
			err := self.ExecuteTemplate(&buf, "dynamic_list_field_value_raw", map[string]any{"Field": f, "Value": value, "Item": item})
			return template.HTML(buf.String()), err
		})
	}
	// renderTeaser - a teaser card: the theme's teaser_<content type> define
	// when it exists, else teaser_default.
	fm["renderTeaser"] = func(item entity.TeaserItem) (template.HTML, error) {
		return renderSuggestion(self, "teaser", string(item.Node.Type), item, true, t.debug)
	}
	// renderSuggestion - "<prefix>_<type>" from the theme when it exists,
	// else the theme's "<prefix>_node", else "<prefix>_default" (e.g. the
	// node page layout page_<type>, web/templates/page/<type>.html).
	fm["renderSuggestion"] = func(prefix string, typ any, data any) (template.HTML, error) {
		return renderSuggestion(self, prefix, fmt.Sprint(typ), data, true, t.debug)
	}
	// moduleHook - the enabled modules' "<hook>_<module>" defines in
	// module order (e.g. "page_end": a module's markup at the end of every
	// page, from its templates/partials/*.html), each trimmed of
	// surrounding whitespace; none - nothing (an empty string, so a
	// template can test whether any module added markup).
	fm["moduleHook"] = func(hook string, data any) (template.HTML, error) {
		var out strings.Builder
		for _, m := range entity.Modules {
			if !ModuleEnabled(m.Name) {
				continue
			}
			h, err := renderSuggestion(self, hook, m.Name, data, false, false)
			if err != nil {
				return "", err
			}
			out.WriteString(strings.TrimSpace(string(h)))
		}
		return template.HTML(out.String()), nil
	}
	// renderOptional - like renderSuggestion without a default: renders
	// "<prefix>_<type>" only if the theme defines it.
	fm["renderOptional"] = func(prefix string, typ any, data any) (template.HTML, error) {
		return renderSuggestion(self, prefix, fmt.Sprint(typ), data, false, t.debug)
	}
	// templateDebug - a define's BEGIN/END comment (template debugging,
	// wrapDebugDefines adds the calls).
	fm["templateDebug"] = func(kind, name, file string) template.HTML {
		if kind == "END" {
			return debugComment("END " + name)
		}
		return debugComment("BEGIN " + name + " - " + file)
	}

	tmpl, err := parseTemplateFiles(template.New("").Funcs(fm), files)
	if err != nil {
		return nil, fmt.Errorf("parsing templates %v: %w", files, err)
	}
	if t.debug {
		if err := wrapDebugDefines(tmpl, fm, debugDefines(files)); err != nil {
			return nil, fmt.Errorf("template debugging: %w", err)
		}
	}
	self = tmpl

	if useCache {
		t.cacheMu.Lock()
		t.cache[cacheKey] = tmpl
		t.cacheMu.Unlock()
	}
	return tmpl, nil
}

// parseTemplateFiles parses files into t as template.ParseFiles does -
// each as a template named by its last element - reading them with
// paths.ReadTemplate, so a module's embedded template ("embed:…") parses
// as a file on disk does.
func parseTemplateFiles(t *template.Template, files []string) (*template.Template, error) {
	if len(files) == 0 {
		return nil, errors.New("no template files")
	}
	for _, f := range files {
		b, err := paths.ReadTemplate(f)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(f)
		tmpl := t
		if name != t.Name() {
			tmpl = t.New(name)
		}
		if _, err := tmpl.Parse(string(b)); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// renderSuggestion executes "<prefix>_<typ>" from set, falling back
// (withDefault) to the theme's "<prefix>_node", then the standard
// "<prefix>_default", or (without) rendering nothing; debug - the names
// tried come first, in a comment (template debugging).
func renderSuggestion(set *template.Template, prefix, typ string, data any, withDefault, debug bool) (template.HTML, error) {
	tried := []string{prefix + "_" + typ}
	if withDefault {
		// "<prefix>_node" - a theme's template for every content type. A
		// field value's ("field_<field type>_<field>") is chosen by field,
		// not by content type: it has none, so a field named "node" gets
		// its own.
		if !strings.HasPrefix(prefix, "field_") {
			tried = append(tried, prefix+"_node")
		}
		tried = append(tried, prefix+"_default")
	}
	name := ""
	for _, n := range tried {
		if set.Lookup(n) != nil {
			name = n
			break
		}
	}
	if name == "" && withDefault {
		name = prefix + "_default"
	}
	// Nothing to render stays empty in debugging too: templates test it.
	if name == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if debug {
		buf.WriteString(string(debugSuggestions(tried, name)))
	}
	if err := set.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
