package admin

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// canEditBaseURL - the site's address goes into the links of the mails
// (a password reset's among them): full-access roles only change it.
func (c *Controller) canEditBaseURL(r *http.Request) bool {
	user, _ := c.GetCurrentUser(r)
	return user != nil && service.HasGrant(user.Role, service.PermFullAccess)
}

// SiteInformationSettings handles GET and POST
// /admin/config/system/site-information: the site's name, slogan, email,
// address, front page feed, time zone, default language, error pages,
// logo, icon and who runs it (schema.org), shown and saved.
func (c *Controller) SiteInformationSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.siteInformationRepo == nil {
		httputil.InternalServerError(w)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			c.RenderBadForm(w, r)
			return
		}
		si := &entity.SiteInformation{
			SiteName:        strings.TrimSpace(r.FormValue("site_name")),
			Slogan:          strings.TrimSpace(r.FormValue("slogan")),
			SiteEmail:       strings.TrimSpace(r.FormValue("site_email")),
			FrontFeedView:   strings.TrimSpace(r.FormValue("front_feed_view")),
			Timezone:        strings.TrimSpace(r.FormValue("timezone")),
			DefaultLanguage: strings.TrimSpace(r.FormValue("default_language")),
			BaseURL:         service.NormalizeBaseURL(r.FormValue("base_url")),
		}
		// base_url - an origin (https://www.example.com), else not saved.
		if !service.ValidBaseURL(si.BaseURL) {
			http.Redirect(w, r, "/admin/config/system/site-information?error=base-url", http.StatusSeeOther)
			return
		}
		// Error pages - each path must lead to a node.
		var missing []string
		for _, f := range []struct {
			field string
			dst   *string
		}{{"page_404", &si.Page404}, {"page_403", &si.Page403}} {
			paths := service.ParseErrorPagePaths(r.FormValue(f.field))
			_, bad := service.ResolveErrorPages(ctx, c.nodeRepo.BySlug, paths)
			missing = append(missing, bad...)
			*f.dst = strings.Join(paths, "\n")
		}
		if len(missing) > 0 {
			http.Redirect(w, r, "/admin/config/system/site-information?error=error-pages&missing="+url.QueryEscape(strings.Join(missing, ", ")), http.StatusSeeOther)
			return
		}
		// default_language - one the interface can be shown in
		// (pkg/i18n), else the source language.
		if !i18n.Current().Available(si.DefaultLanguage) {
			si.DefaultLanguage = i18n.SourceLanguage
		}
		// timezone - a valid IANA zone (sitetime.Load), else not saved.
		if _, err := sitetime.Load(si.Timezone); err != nil {
			http.Redirect(w, r, "/admin/config/system/site-information?error=timezone", http.StatusSeeOther)
			return
		}

		// The logo and the icon: a new file, a removal, else the current
		// one as is.
		existing, err := c.siteInformationRepo.Get(ctx)
		if err != nil {
			c.LogFailure(err, "site information get (preserving the logo and the icon)")
			http.Redirect(w, r, "/admin/config/system/site-information?error=1", http.StatusSeeOther)
			return
		}
		// The site's address is the one in the links mailed for a
		// password reset: full-access roles only change it.
		if !c.canEditBaseURL(r) {
			si.BaseURL = existing.BaseURL
		}
		for _, img := range []struct {
			field, basePath string
			current         entity.File
			dst             *entity.File
		}{
			{"logo", files.SiteLogoBasePath, existing.Logo, &si.Logo},
			{"icon", service.SiteIconBasePath, existing.Icon, &si.Icon},
		} {
			*img.dst = img.current
			if r.FormValue("delete_"+img.field) == "1" {
				*img.dst = entity.File{}
				continue
			}
			file, header, err := r.FormFile(img.field)
			if err != nil || header == nil || header.Size == 0 {
				continue
			}
			uri, _, _, _, _, err := c.fileUploadService.Upload(ctx, file, header, img.basePath)
			_ = file.Close()
			if err != nil {
				c.LogFailure(err, "site image upload", "field", img.field)
				http.Redirect(w, r, "/admin/config/system/site-information?error=1", http.StatusSeeOther)
				return
			}
			*img.dst = entity.File{URI: uri}
		}

		if err := c.siteInformationRepo.Update(ctx, si); err != nil {
			c.LogFailure(err, "site information update")
			http.Redirect(w, r, "/admin/config/system/site-information?error=1", http.StatusSeeOther)
			return
		}
		// A new default language on a site of several: the old one keeps
		// its addresses under its code (entity.MoveDefaultLanguage).
		languages := existing.Languages
		if len(languages) > 0 && si.DefaultLanguage != existing.DefaultLanguage {
			languages = entity.MoveDefaultLanguage(languages, existing.DefaultLanguage, si.DefaultLanguage)
			if err := c.siteInformationRepo.SetLanguages(ctx, languages); err != nil {
				c.LogFailure(err, "site languages update (a new default language)")
				http.Redirect(w, r, "/admin/config/system/site-information?error=1", http.StatusSeeOther)
				return
			}
		}
		if err := c.siteInformationRepo.SetSchemaOrg(ctx, schemaOrgFromForm(r)); err != nil {
			c.LogFailure(err, "who runs the site update")
			http.Redirect(w, r, "/admin/config/system/site-information?error=1", http.StatusSeeOther)
			return
		}
		_ = sitetime.Set(si.Timezone)
		i18n.SetDefault(si.DefaultLanguage)
		entity.SetSiteLanguages(i18n.Default(), languages)
		service.SetSiteIcon(si.Icon.URI)
		if c.siteURL != nil {
			c.siteURL.Apply(si.BaseURL)
		}
		service.LoadErrorPages(ctx, c.nodeRepo.BySlug, si)
		if err := interfaces.SaveFormSections(ctx, interfaces.FormSiteInformation, entity.SiteTextRef, r); err != nil {
			c.LogFailure(err, "saving the modules' sections of site information")
			http.Redirect(w, r, "/admin/config/system/site-information?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/config/system/site-information?saved=1", http.StatusSeeOther)
		return
	}

	si, err := c.siteInformationRepo.Get(ctx)
	if err != nil {
		c.LogFailure(err, "site information get")
		httputil.InternalServerError(w)
		return
	}

	// Feed View options: content Views (any display type).
	var feedViews entity.ViewDisplayDefs
	if views, err := c.viewService.Views(ctx); err == nil {
		for _, v := range views {
			if !v.IsDefault && !v.IsComment() {
				feedViews = append(feedViews, v)
			}
		}
	} else {
		c.LogFailure(err, "views for the front feed setting")
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Site information")
	data["feedViews"] = feedViews
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Site information"))}
	data["site"] = si
	// themeLogo - the active theme's logo, shown while none is uploaded.
	data["themeLogo"] = service.SiteLogoFor(paths.Theme(), "")
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"
	data["timezoneError"] = r.URL.Query().Get("error") == "timezone"
	data["baseURLError"] = r.URL.Query().Get("error") == "base-url"
	data["canEditBaseURL"] = c.canEditBaseURL(r)
	if c.siteURL != nil {
		data["baseURLOverride"] = c.siteURL.Override()
	}
	if r.URL.Query().Get("error") == "error-pages" {
		data["errorPagesMissing"] = r.URL.Query().Get("missing")
	}
	// Configured error pages that won't be shown: missing or unpublished.
	var unpublished, gone []string
	for _, text := range []string{si.Page404, si.Page403} {
		paths := service.ParseErrorPagePaths(text)
		nodes, bad := service.ResolveErrorPages(ctx, c.nodeRepo.BySlug, paths)
		gone = append(gone, bad...)
		for _, n := range nodes {
			if n.Status != 1 {
				unpublished = append(unpublished, "/"+n.Slug)
			}
		}
	}
	data["errorPagesUnpublished"] = strings.Join(unpublished, ", ")
	data["errorPagesGone"] = strings.Join(gone, ", ")
	data["timezones"] = sitetime.ZoneOptions(si.Timezone)
	data["languages"] = i18n.Current().Options()
	c.AddFormSectionsData(ctx, data, interfaces.FormSiteInformation, entity.SiteTextRef)
	c.RenderOrFail(w, r, "admin/config/site_information.html", data)
}

// AccountSettingsPage GET/POST /admin/config/account
func (c *Controller) AccountSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.accountSettings == nil {
		httputil.InternalServerError(w)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			c.RenderBadForm(w, r)
			return
		}
		mode := strings.TrimSpace(r.FormValue("registration_mode"))
		switch mode {
		case entity.RegistrationModeAdminOnly, entity.RegistrationModeVisitors, entity.RegistrationModeVisitorsApproval:
		default:
			mode = entity.RegistrationModeVisitors
		}
		s := &entity.AccountSettings{
			RegistrationMode:           mode,
			RequireEmailVerification:   r.FormValue("require_email_verification") == "1",
			UserTimezones:              r.FormValue("user_timezones") == "1",
			RegistrationBlockedDomains: r.FormValue("registration_blocked_domains"),
			AvatarAllowedExtensions:    interfaces.ParseExtensionsForm(r.FormValue("avatar_allowed_extensions")),
			AvatarMaxFileSizeBytes:     interfaces.MaxFileSizeBytesFromMBForm(r.FormValue("avatar_max_filesize_mb")),
			TwoFactorIssuer:            strings.TrimSpace(r.FormValue("two_factor_issuer")),
			CancelContent:              entity.CancelKeepContent,
		}
		if r.FormValue("cancel_content") == entity.CancelDeleteContent {
			s.CancelContent = entity.CancelDeleteContent
		}
		for _, v := range r.Form["two_factor_roles"] {
			if id, err := strconv.Atoi(v); err == nil && id > 0 {
				s.TwoFactorRoles = append(s.TwoFactorRoles, id)
			}
		}
		if err := c.accountSettings.Save(ctx, s); err != nil {
			c.LogFailure(err, "account settings update")
			http.Redirect(w, r, "/admin/config/account?error=1", http.StatusSeeOther)
			return
		}
		// The modules' sections (AddAccountSettingsSection).
		if err := interfaces.SaveAccountSettingsSections(ctx, r); err != nil {
			c.LogFailure(err, "account settings of the modules")
			http.Redirect(w, r, "/admin/config/account?error=1", http.StatusSeeOther)
			return
		}
		saved := "/admin/config/account?saved=1"
		if c.twoFactor != nil {
			c.twoFactor.Invalidate()
			// The administrator's own role now needs it and it's off: say
			// so (the next page asks to turn it on).
			if me, _ := c.GetCurrentUser(r); me != nil {
				if needed, err := c.twoFactor.SetupNeeded(ctx, *me); err == nil && needed {
					saved += "&self_two_factor=1"
				}
			}
		}
		http.Redirect(w, r, saved, http.StatusSeeOther)
		return
	}

	acc, err := c.accountSettings.Settings(ctx)
	if err != nil {
		c.LogFailure(err, "account settings get")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Account settings")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Account settings"))}
	data["account"] = acc
	data["moduleSettings"] = c.LoadAccountSettingsSections(ctx)
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error") == "1"
	data["selfTwoFactor"] = r.URL.Query().Get("self_two_factor") == "1"
	twoFactorRoles := map[int]bool{}
	for _, id := range acc.TwoFactorRoles {
		twoFactorRoles[id] = true
	}
	data["twoFactorRoles"] = twoFactorRoles
	if c.roleRepo != nil {
		if roles, err := c.roleRepo.List(ctx); err != nil {
			c.LogFailure(err, "listing the roles for the two-factor setting")
		} else if roles != nil {
			data["roles"] = *roles
		}
	}
	c.RenderOrFail(w, r, "admin/people/account_settings.html", data)
}

// TwoFactorReset handles POST /admin/users/{id}/two-factor/reset: the
// user's two-factor sign-in turned off (they lost the phone and the
// codes); they turn it on again after signing in. Back to their form.
func (c *Controller) TwoFactorReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || c.twoFactor == nil {
		c.RenderNotFound(w, r)
		return
	}
	user, err := c.userRepo.ByID(ctx, id)
	if err != nil || user == nil {
		c.RenderLoadFailure(w, r, err, "loading the user to reset the two-factor sign-in", "id", id)
		return
	}
	// As for the rest of the account: not one with rights the editor may
	// not give.
	if current, _ := c.GetCurrentUser(r); !service.MayManageAccount(ctx, c.authService, current, user) {
		c.RenderForbidden(w, r)
		return
	}
	if err := c.twoFactor.Reset(ctx, *user); err != nil {
		c.LogFailure(err, "resetting the two-factor sign-in", "id", id)
		// The form shows ?error as the message.
		http.Redirect(w, r, "/user/"+strconv.Itoa(id)+"/edit?error="+url.QueryEscape(i18n.T(ctx, "Error while saving.")), http.StatusSeeOther)
		return
	}
	// Sessions opened without the second factor end.
	if err := c.authService.EndSessions(ctx, w, r, id); err != nil {
		c.LogFailure(err, "ending the sessions after resetting the two-factor sign-in", "id", id)
	}
	http.Redirect(w, r, "/user/"+strconv.Itoa(id)+"/edit?two_factor_reset=1", http.StatusSeeOther)
}

// schemaOrgFromForm - who runs the site as the site information form
// sent it: the profiles one per line, web addresses only.
func schemaOrgFromForm(r *http.Request) entity.SiteSchemaOrg {
	o := entity.SiteSchemaOrg{
		Kind:      r.FormValue("schema_org_kind"),
		Name:      strings.TrimSpace(r.FormValue("schema_org_name")),
		Phone:     strings.TrimSpace(r.FormValue("schema_org_phone")),
		Email:     strings.TrimSpace(r.FormValue("schema_org_email")),
		Address:   strings.TrimSpace(r.FormValue("schema_org_address")),
		SearchBox: r.FormValue("schema_org_search") == "1",
	}
	if o.Kind != entity.SchemaOrgLocalBusiness && o.Kind != entity.SchemaOrgPerson {
		o.Kind = entity.SchemaOrgOrganization
	}
	for _, line := range strings.Split(r.FormValue("schema_org_same_as"), "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "https://") || strings.HasPrefix(line, "http://") {
			o.SameAs = append(o.SameAs, line)
		}
	}
	return o
}
