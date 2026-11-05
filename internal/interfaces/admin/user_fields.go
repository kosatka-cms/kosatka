package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// User fields - profile fields shown on the profile and edited in the profile form.

const userFieldsAdminURL = "/admin/people/fields"

// userFieldsErrorCodes - the ?error= code per service error.
var userFieldsErrorCodes = map[error]string{
	accounts.ErrUserFieldName:   "name",
	accounts.ErrUserFieldLabel:  "label",
	accounts.ErrUserFieldExists: "exists",
	accounts.ErrUserFieldMaxLen: "max-len",
}

func userFieldsErrorCode(err error) string {
	for e, code := range userFieldsErrorCodes {
		if errors.Is(err, e) {
			return code
		}
	}
	return "save"
}

// UserFieldsPage renders GET /admin/people/fields.
func (c *Controller) UserFieldsPage(w http.ResponseWriter, r *http.Request) {
	fields, err := c.userFields.Fields(r.Context())
	if err != nil {
		c.LogFailure(err, "error loading user fields")
		httputil.InternalServerError(w)
		return
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "User fields")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "User fields"))}
	data["fields"] = fields
	data["success"] = r.URL.Query().Get("success")
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/people/user_fields.html", data)
}

// UserFieldCreate handles POST /admin/people/fields.
func (c *Controller) UserFieldCreate(w http.ResponseWriter, r *http.Request) {
	maxLen, _ := strconv.Atoi(r.FormValue("max_len"))
	if err := c.userFields.CreateField(r.Context(), r.FormValue("machine_name"), r.FormValue("label"), maxLen, r.FormValue("required") != ""); err != nil {
		c.logger.Warn("error creating user field", "error", err)
		http.Redirect(w, r, userFieldsAdminURL+"?error="+userFieldsErrorCode(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, userFieldsAdminURL+"?success=created", http.StatusSeeOther)
}

// UserFieldUpdate handles POST /admin/people/fields/{id}/edit.
func (c *Controller) UserFieldUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, userFieldsAdminURL, http.StatusSeeOther)
		return
	}
	maxLen, _ := strconv.Atoi(r.FormValue("max_len"))
	weight, _ := strconv.Atoi(r.FormValue("weight"))
	if err := c.userFields.UpdateField(r.Context(), id, r.FormValue("label"), maxLen, weight, r.FormValue("required") != ""); err != nil {
		c.logger.Warn("error updating user field", "error", err, "fieldID", id)
		http.Redirect(w, r, userFieldsAdminURL+"?error="+userFieldsErrorCode(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, userFieldsAdminURL+"?success=saved", http.StatusSeeOther)
}

// UserFieldDelete handles POST /admin/people/fields/{id}/delete - the
// field and every user's value of it.
func (c *Controller) UserFieldDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, userFieldsAdminURL, http.StatusSeeOther)
		return
	}
	if err := c.userFields.DeleteField(r.Context(), id); err != nil {
		c.LogFailure(err, "error deleting user field", "fieldID", id)
		http.Redirect(w, r, userFieldsAdminURL+"?error=save", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, userFieldsAdminURL+"?success=deleted", http.StatusSeeOther)
}
