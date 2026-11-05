package admin

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/sociallogin"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// oauthRedirectURLs - the callback URLs to register in the providers'
// consoles (oauth.Registry).
type oauthRedirectURLs interface {
	RedirectURL(provider string) string
}

// oauthProviderRow - a provider's block on the settings page.
type oauthProviderRow struct {
	Name, Label string
	Enabled     bool
	ClientID    string
	SecretSaved bool
	KeysSaved   bool
	Source      string
	Active      bool
	RedirectURL string
	ConsoleURL  string
	LinkedCount int
}

// OAuthPage handles GET/POST /admin/config/oauth: each social login
// provider's switch and keys. Full-access roles only: the page holds the
// client secrets, which are never shown back.
func (c *Controller) OAuthPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := c.GetCurrentUser(r)
	if user == nil || !service.HasGrant(user.Role, service.PermFullAccess) {
		c.RenderForbidden(w, r)
		return
	}
	if c.oauthSettings == nil {
		httputil.InternalServerError(w)
		return
	}
	saved, err := c.oauthSettings.Saved(ctx)
	if err != nil {
		c.LogFailure(err, "error loading the social login settings")
		httputil.InternalServerError(w)
		return
	}

	if r.Method == http.MethodPost {
		fromConfig := map[string]bool{}
		for _, st := range c.oauthSettings.Status() {
			fromConfig[st.Name] = st.Source == sociallogin.SourceConfig
		}
		rows, bad := oauthSettingsFromForm(r, saved, fromConfig)
		if bad != "" {
			http.Redirect(w, r, "/admin/config/oauth?error=pair&provider="+bad, http.StatusSeeOther)
			return
		}
		if err := c.oauthSettings.Save(ctx, rows); err != nil {
			c.LogFailure(err, "error saving the social login settings")
			http.Redirect(w, r, "/admin/config/oauth?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/config/oauth?saved=1", http.StatusSeeOther)
		return
	}

	linked, err := c.oauthSettings.LinkedAccounts(ctx)
	if err != nil {
		c.LogFailure(err, "error counting linked accounts")
	}
	status := map[string]sociallogin.ProviderStatus{}
	if c.oauthSettings != nil {
		for _, st := range c.oauthSettings.Status() {
			status[st.Name] = st
		}
	}
	providers := make([]oauthProviderRow, 0, len(entity.OAuthProviders))
	for _, name := range entity.OAuthProviders {
		row, ok := saved[name]
		st := status[name]
		p := oauthProviderRow{
			Name:        name,
			Label:       i18n.T(ctx, interfaces.OAuthProviderLabels[name]),
			Enabled:     !ok || row.Enabled,
			ClientID:    row.ClientID,
			SecretSaved: row.ClientSecret != "",
			KeysSaved:   row.ClientID != "" || row.ClientSecret != "",
			Source:      st.Source,
			Active:      st.Active,
			ConsoleURL:  entity.OAuthConsoleURLs[name],
			LinkedCount: linked[name],
		}
		if c.oauthRedirects != nil {
			p.RedirectURL = c.oauthRedirects.RedirectURL(name)
		}
		providers = append(providers, p)
	}
	title := i18n.T(ctx, "Social login")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["providers"] = providers
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error")
	data["errorProvider"] = i18n.T(ctx, interfaces.OAuthProviderLabels[r.URL.Query().Get("provider")])
	c.RenderOrFail(w, r, "admin/people/oauth.html", data)
}

// oauthSettingsFromForm - each provider's row as the form submits it on
// top of the saved one, or the provider whose keys are not a pair; a
// provider with keys in the server config (fromConfig) keeps its saved
// keys - only its switch changes.
func oauthSettingsFromForm(r *http.Request, saved map[string]entity.OAuthProviderSettings, fromConfig map[string]bool) ([]entity.OAuthProviderSettings, string) {
	rows := make([]entity.OAuthProviderSettings, 0, len(entity.OAuthProviders))
	for _, name := range entity.OAuthProviders {
		row := saved[name]
		row.Provider = name
		row.Enabled = r.FormValue(name+"_enabled") == "1"
		if fromConfig[name] {
			rows = append(rows, row)
			continue
		}
		if r.FormValue(name+"_clear") == "1" {
			row.ClientID, row.ClientSecret = "", ""
		} else {
			row.ClientID = strings.TrimSpace(r.FormValue(name + "_client_id"))
			// An empty secret field keeps the saved secret (it is never
			// shown on the page).
			if secret := strings.TrimSpace(r.FormValue(name + "_client_secret")); secret != "" {
				row.ClientSecret = secret
			}
			if (row.ClientID == "") != (row.ClientSecret == "") {
				return nil, name
			}
		}
		rows = append(rows, row)
	}
	return rows, ""
}
