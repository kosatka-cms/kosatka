package admin

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// fileExtensionRe - one file extension in the upload policy.
var fileExtensionRe = regexp.MustCompile(`^[a-z0-9]{1,10}$`)

// UploadSettingsPage handles GET/POST /admin/config/upload-settings - the
// site-wide upload policy (max file size, max image dimension, allowed MIME
// types) applied by every upload path. Avatar/slider output sizes are fixed
// layout targets, not policy (see entity.UploadSettings).
func (c *Controller) UploadSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if c.uploadSettingsRepo == nil {
		httputil.InternalServerError(w)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			c.RenderBadForm(w, r)
			return
		}

		maxFileSizeMB, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("max_file_size_mb")), 64)
		if err != nil || maxFileSizeMB <= 0 {
			http.Redirect(w, r, "/admin/config/upload-settings?error=invalid", http.StatusSeeOther)
			return
		}
		maxImageDimension, err := strconv.Atoi(strings.TrimSpace(r.FormValue("max_image_dimension")))
		if err != nil || maxImageDimension <= 0 {
			http.Redirect(w, r, "/admin/config/upload-settings?error=invalid", http.StatusSeeOther)
			return
		}
		var allowedMimeTypes []string
		for _, mt := range []string{"image/jpeg", "image/png", "image/gif", "image/webp"} {
			if r.FormValue("mime_"+mt) == "1" {
				allowedMimeTypes = append(allowedMimeTypes, mt)
			}
		}
		if len(allowedMimeTypes) == 0 {
			http.Redirect(w, r, "/admin/config/upload-settings?error=empty", http.StatusSeeOther)
			return
		}

		userFileExts := entity.ParseFileExtensions(r.FormValue("user_file_extensions"))
		for _, ext := range userFileExts {
			if !fileExtensionRe.MatchString(ext) || entity.ForbiddenUserFileExtensions[ext] {
				http.Redirect(w, r, "/admin/config/upload-settings?error=extensions&ext="+url.QueryEscape(ext), http.StatusSeeOther)
				return
			}
		}

		s := &entity.UploadSettings{
			MaxFileSizeBytes:   int(maxFileSizeMB * 1024 * 1024),
			MaxImageDimension:  maxImageDimension,
			AllowedMimeTypes:   allowedMimeTypes,
			UserFileExtensions: userFileExts,
		}
		if err := c.uploadSettingsRepo.Update(ctx, s); err != nil {
			c.LogFailure(err, "upload settings update")
			http.Redirect(w, r, "/admin/config/upload-settings?error=1", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/config/upload-settings?saved=1", http.StatusSeeOther)
		return
	}

	settings, err := c.uploadSettingsRepo.Get(ctx)
	if err != nil {
		c.LogFailure(err, "upload settings get")
		httputil.InternalServerError(w)
		return
	}

	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "File upload policy")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "File upload policy"))}
	data["settings"] = settings
	data["maxFileSizeMB"] = float64(settings.MaxFileSizeBytes) / 1024 / 1024
	allowedMimeSet := make(map[string]bool, len(settings.AllowedMimeTypes))
	for _, a := range settings.AllowedMimeTypes {
		allowedMimeSet[a] = true
	}
	data["allowedMimeSet"] = allowedMimeSet
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["error"] = r.URL.Query().Get("error")
	data["badExtension"] = r.URL.Query().Get("ext")
	data["userFileExtensions"] = strings.Join(settings.UserFileExtensions, " ")
	c.RenderOrFail(w, r, "admin/config/upload_settings.html", data)
}
