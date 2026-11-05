package videoembed

import (
	"net/http"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Controller - the module's settings page and the editor's check of a
// link.
type Controller struct {
	*modulekit.BaseController
	service *Service
}

// NewController creates a Controller.
func NewController(base *modulekit.BaseController, service *Service) *Controller {
	base.SetLogger(base.Logger().With("component", "video_embed.handler"))
	return &Controller{BaseController: base, service: service}
}

// providerRow - a service on the settings page.
type providerRow struct {
	Provider
	Checked bool
}

// Settings renders GET /admin/config/video-embed: the services, the
// preview and the player's loading.
func (c *Controller) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := c.service.Settings(ctx)
	on := st.On()
	rows := make([]providerRow, len(providers))
	for i, p := range providers {
		rows[i] = providerRow{Provider: p, Checked: on[p.Name]}
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Video embeds")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["providers"] = rows
	data["settings"] = st
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/video_embed.html", data)
}

// SaveSettings handles POST /admin/config/video-embed.
func (c *Controller) SaveSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderBadForm(w, r)
		return
	}
	var names []string
	for _, p := range providers {
		if slices.Contains(r.Form["providers"], p.Name) {
			names = append(names, p.Name)
		}
	}
	st := Settings{Providers: names, ClickToPlay: r.FormValue("click_to_play") == "1", LocalPreviews: r.FormValue("local_previews") == "1"}
	if err := c.service.SaveSettings(r.Context(), st); err != nil {
		c.LogFailure(err, "saving the video embed settings")
		http.Redirect(w, r, manifest.AdminPath+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, manifest.AdminPath+"?saved=1", http.StatusSeeOther)
}

// resolveResponse - the editor's check of a link: the service's name,
// the video's title and preview (when the service told them), whether
// it's vertical; OK false - not a video link of a service on.
type resolveResponse struct {
	OK       bool   `json:"ok"`
	Provider string `json:"provider,omitempty"`
	Title    string `json:"title,omitempty"`
	Preview  string `json:"preview,omitempty"`
	Portrait bool   `json:"portrait,omitempty"`
}

// Resolve handles GET /api/video-embed?url=: the editor's "Video" button
// and cards (signed in). The service is asked once; afterwards the
// answer is the table's.
func (c *Controller) Resolve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// The editors' check: who writes content. Another member would make
	// the site ask the video services, and keep their answers, at will.
	user, _ := c.GetCurrentUser(r)
	if len(c.AuthService().CreatableNodeTypes(ctx, user)) == 0 && !c.AuthService().HasPermission(ctx, user, modulekit.PermAccessAdmin) {
		httputil.SendJSON(w, http.StatusForbidden, resolveResponse{})
		return
	}
	st := c.service.Settings(ctx)
	m, ok := MatchURL(r.URL.Query().Get("url"), st.On())
	if !ok {
		httputil.SendJSON(w, http.StatusOK, resolveResponse{})
		return
	}
	e, known := c.service.Embeds(ctx, []Match{m})[m.URL]
	if !known || e.Failed {
		var err error
		if e, err = c.service.Resolve(ctx, m); err != nil {
			c.LogFailure(err, "resolving a video link", "url", m.URL)
		}
	}
	resp := resolveResponse{OK: true, Provider: m.Provider.Label, Title: e.Title, Portrait: m.Portrait() || e.Height > e.Width}
	switch {
	case e.ThumbnailFile != "":
		resp.Preview = modulekit.StyledImage(modulekit.File{URI: "public://" + e.ThumbnailFile}, manifest.PreviewStyle).URI
	case e.ThumbnailURL != "" && !st.LocalPreviews:
		resp.Preview = e.ThumbnailURL
	}
	httputil.SendJSON(w, http.StatusOK, resp)
}
