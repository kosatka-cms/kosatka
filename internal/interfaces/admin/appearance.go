package admin

import (
	"net/http"
	"sort"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// AppearancePage handles GET/POST /admin/config/appearance: the available
// themes and the choice of the active one.
func (c *Controller) AppearancePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		dir := strings.TrimSpace(r.FormValue("theme"))
		// Only a theme the engine release is compatible with.
		if !themes.IsUsable(dir) || c.siteInformationRepo == nil {
			http.Redirect(w, r, "/admin/config/appearance?error=1", http.StatusSeeOther)
			return
		}
		if err := c.siteInformationRepo.SetTheme(ctx, dir); err != nil {
			c.LogFailure(err, "error saving the theme", "theme", dir)
			http.Redirect(w, r, "/admin/config/appearance?error=1", http.StatusSeeOther)
			return
		}
		if c.switchTheme != nil {
			c.switchTheme(dir)
		}
		// Activated (from the preview too): no theme to preview any more.
		if err := c.setThemePreview(w, r, nil); err != nil {
			c.LogFailure(err, "error ending the theme preview")
		}
		http.Redirect(w, r, "/admin/config/appearance?saved=1", http.StatusSeeOther)
		return
	}

	options, err := themes.Available()
	if err != nil {
		c.LogFailure(err, "error listing themes")
	}
	// The active theme first, the others as listed.
	sort.SliceStable(options, func(i, j int) bool {
		return options[i].Dir == paths.Theme() && options[j].Dir != paths.Theme()
	})
	// The name, description and tags in the theme's own translations.
	for i := range options {
		th := &options[i]
		th.Name = service.ThemeText(ctx, th.Dir, th.Name)
		if th.Description != "" {
			th.Description = service.ThemeText(ctx, th.Dir, th.Description)
		}
		tags := make([]string, len(th.Tags))
		for j, tag := range th.Tags {
			tags[j] = service.ThemeText(ctx, th.Dir, tag)
		}
		th.Tags = tags
	}
	title := i18n.T(ctx, "Appearance")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["themes"] = options
	invalid, err := themes.Invalid()
	if err != nil {
		c.LogFailure(err, "error listing invalid themes")
	}
	data["invalidThemes"] = invalid
	data["activeTheme"] = paths.Theme()
	data["engineRelease"] = themes.EngineRelease()
	data["backToTopEnabled"] = service.FeatureEnabled(entity.FeatureBackToTop)
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["settingsSaved"] = r.URL.Query().Get("settings_saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"
	c.RenderOrFail(w, r, "admin/config/appearance.html", data)
}

// AppearancePreview handles POST /admin/config/appearance/preview: the
// site in another theme for this administrator only (their session,
// middleware.ThemePreviewMiddleware), from the front page. The theme's
// block layout is set up first, as on the Blocks page.
func (c *Controller) AppearancePreview(w http.ResponseWriter, r *http.Request) {
	c.startThemePreview(w, r, strings.TrimSpace(r.FormValue("theme")))
}

// startThemePreview - the preview of theme dir, from the front page (an
// unknown or the active theme - back to the Appearance page).
func (c *Controller) startThemePreview(w http.ResponseWriter, r *http.Request, dir string) {
	if dir == paths.Theme() || !themes.IsUsable(dir) {
		http.Redirect(w, r, "/admin/config/appearance?error=1", http.StatusSeeOther)
		return
	}
	key := entity.ThemeKey(dir)
	if _, err := c.blockService.InitThemePlacements(r.Context(), key, service.ActiveThemeKey(), interfaces.ThemeRegionsFor(c.logger, key)); err != nil {
		c.LogFailure(err, "error initializing theme block placements", "theme", key)
	}
	if err := c.setThemePreview(w, r, &dir); err != nil {
		c.LogFailure(err, "error starting the theme preview", "theme", key)
		http.Redirect(w, r, "/admin/config/appearance?error=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// themeSettingField - a theme setting on its form: its label and options
// translated, its current value.
type themeSettingField struct {
	entity.ThemeSetting
	Value   string
	Options []themeSettingChoice
}

type themeSettingChoice struct {
	Value, Label string
	Selected     bool
}

// AppearanceThemeSettings handles GET/POST
// /admin/config/appearance/theme/{theme}: the settings a theme offers
// (theme.yaml), by its placement key. POST "action": "reset" - back to
// the defaults, "preview" - save and see the inactive theme, else save.
func (c *Controller) AppearanceThemeSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.PathValue("theme")
	dir := entity.ThemeDir(key)
	settings := c.themeSettings.Settings(dir)
	if !themes.IsAvailable(dir) || len(settings) == 0 {
		c.RenderNotFound(w, r)
		return
	}
	pageURL := "/admin/config/appearance/theme/" + key
	if r.Method == http.MethodPost {
		var err error
		if r.FormValue("action") == "reset" {
			err = c.themeSettings.Reset(ctx, dir)
		} else {
			in := map[string]string{}
			for _, st := range settings {
				if v, ok := r.Form["setting_"+st.ID]; ok && len(v) > 0 {
					in[st.ID] = v[0]
				}
				if st.Type == entity.ThemeSettingBool {
					in[st.ID] = "0"
					if r.FormValue("setting_"+st.ID) == "1" {
						in[st.ID] = "1"
					}
				}
			}
			err = c.themeSettings.Save(ctx, dir, in)
		}
		if err != nil {
			c.LogFailure(err, "error saving theme settings", "theme", key)
			http.Redirect(w, r, pageURL+"?error=1", http.StatusSeeOther)
			return
		}
		if r.FormValue("action") == "preview" {
			c.startThemePreview(w, r, dir)
			return
		}
		http.Redirect(w, r, pageURL+"?saved=1", http.StatusSeeOther)
		return
	}

	values := c.themeSettings.Values(ctx, dir)
	fields := make([]themeSettingField, len(settings))
	for i, st := range settings {
		st.Label = service.ThemeText(ctx, dir, st.Label)
		f := themeSettingField{ThemeSetting: st, Value: values[st.ID]}
		for _, o := range st.Options {
			f.Options = append(f.Options, themeSettingChoice{Value: o.Value, Label: service.ThemeText(ctx, dir, o.Label), Selected: o.Value == f.Value})
		}
		fields[i] = f
	}
	name := key
	if info, err := themes.LoadInfoFor(dir); err == nil && info.Name != "" {
		name = service.ThemeText(ctx, dir, info.Name)
	}
	title := i18n.T(ctx, "Theme settings: @name", "@name", name)
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["themeActive"] = dir == paths.Theme()
	data["fields"] = fields
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"
	c.RenderOrFail(w, r, "admin/config/appearance_theme.html", data)
}

// AppearancePreviewStop handles POST /admin/config/appearance/preview/stop:
// the site in the active theme again.
func (c *Controller) AppearancePreviewStop(w http.ResponseWriter, r *http.Request) {
	if err := c.setThemePreview(w, r, nil); err != nil {
		c.LogFailure(err, "error ending the theme preview")
	}
	http.Redirect(w, r, "/admin/config/appearance", http.StatusSeeOther)
}

// setThemePreview - the theme the user's session previews (nil - none;
// no session store - nothing to change).
func (c *Controller) setThemePreview(w http.ResponseWriter, r *http.Request, dir *string) error {
	if c.store == nil {
		return nil
	}
	session, err := c.store.Get(r, entity.SessionName)
	if err != nil {
		return err
	}
	if dir == nil {
		if _, ok := session.Values[themes.PreviewSessionKey]; !ok {
			return nil
		}
		delete(session.Values, themes.PreviewSessionKey)
	} else {
		session.Values[themes.PreviewSessionKey] = *dir
	}
	return session.Save(r, w)
}

// AppearanceSettingsSave handles POST /admin/config/appearance/settings:
// the theme-independent display options.
func (c *Controller) AppearanceSettingsSave(w http.ResponseWriter, r *http.Request) {
	if err := c.saveFeature(r.Context(), entity.FeatureBackToTop, r.FormValue("back_to_top") == "1"); err != nil {
		c.LogFailure(err, "error saving the back to top switch")
		http.Redirect(w, r, "/admin/config/appearance?error=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/config/appearance?settings_saved=1", http.StatusSeeOther)
}
