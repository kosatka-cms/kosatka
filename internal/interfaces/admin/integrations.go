package admin

import (
	"errors"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Integrations (/admin/config/integrations): site verification, Yandex
// Metrika, Google Analytics, and raw code for everything else.

const integrationsAdminURL = "/admin/config/integrations"

// canEditIntegrationCode - the raw code fields are HTML/JS put on every
// page as is: full-access roles only.
func (c *Controller) canEditIntegrationCode(r *http.Request) bool {
	user, _ := c.GetCurrentUser(r)
	return user != nil && service.HasGrant(user.Role, service.PermFullAccess)
}

// IntegrationsPage handles GET and POST /admin/config/integrations.
func (c *Controller) IntegrationsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	canEditCode := c.canEditIntegrationCode(r)
	if r.Method == http.MethodPost {
		in := entity.IntegrationSettings{
			VerificationTags:        r.FormValue("verification_tags"),
			MetrikaID:               r.FormValue("metrika_id"),
			MetrikaClickmap:         r.FormValue("metrika_clickmap") == "1",
			MetrikaTrackLinks:       r.FormValue("metrika_track_links") == "1",
			MetrikaAccurateBounce:   r.FormValue("metrika_accurate_bounce") == "1",
			MetrikaWebvisor:         r.FormValue("metrika_webvisor") == "1",
			GAID:                    r.FormValue("ga_id"),
			HeadCode:                r.FormValue("head_code"),
			BodyEndCode:             r.FormValue("body_end_code"),
			BodyEndSkipServicePages: r.FormValue("body_end_skip_service_pages") == "1",
			Consent: entity.Consent{
				Enabled:      r.FormValue("consent_enabled") == "1",
				Mode:         entity.ConsentMode(r.FormValue("consent_mode")),
				Text:         r.FormValue("consent_text"),
				PolicyURL:    r.FormValue("consent_policy_url"),
				MetrikaGroup: entity.ParseConsentGroup(r.FormValue("metrika_group")),
				GAGroup:      entity.ParseConsentGroup(r.FormValue("ga_group")),
				HeadGroup:    entity.ParseConsentGroup(r.FormValue("head_code_group")),
				BodyEndGroup: entity.ParseConsentGroup(r.FormValue("body_end_group")),
			},
		}
		if err := c.integrations.Save(ctx, in, canEditCode); err != nil {
			code := "save"
			switch {
			case errors.Is(err, service.ErrIntegrationMetrikaID):
				code = "metrika"
			case errors.Is(err, service.ErrIntegrationGAID):
				code = "ga"
			case errors.Is(err, service.ErrConsentPolicyURL):
				code = "consent_page"
			default:
				c.LogFailure(err, "error saving integration settings")
			}
			http.Redirect(w, r, integrationsAdminURL+"?error="+code, http.StatusSeeOther)
			return
		}
		// "Ask everyone again" - its own button of the same form.
		if r.FormValue("consent_reset") == "1" {
			if err := c.integrations.ResetConsent(ctx); err != nil {
				c.LogFailure(err, "error resetting the visitors' consent")
				http.Redirect(w, r, integrationsAdminURL+"?error=save", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, integrationsAdminURL+"?saved=reset", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, integrationsAdminURL+"?saved=1", http.StatusSeeOther)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Counters and codes")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Counters and codes"))}
	data["settings"] = c.integrations.Current(ctx)
	data["canEditCode"] = canEditCode
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["consentReset"] = r.URL.Query().Get("saved") == "reset"
	data["consentGroups"] = entity.ConsentGroups
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/config/integrations.html", data)
}
