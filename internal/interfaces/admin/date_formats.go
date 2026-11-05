package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/lib/dateformat"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// Date formats (/admin/config/date-formats) - the date format catalog:
// named patterns that date fields and Views fields pick.

const dateFormatsAdminURL = "/admin/config/date-formats"

// dateFormatErrorCodes - the ?error= code per service error.
var dateFormatErrorCodes = map[error]string{
	service.ErrDateFormatLabel:       "label",
	service.ErrDateFormatMachineName: "machine-name",
	service.ErrDateFormatTaken:       "taken",
	service.ErrDateFormatPattern:     "pattern",
	service.ErrDateFormatDefault:     "default",
	service.ErrDateFormatInUse:       "in-use",
}

func dateFormatErrorCode(err error) string {
	for e, code := range dateFormatErrorCodes {
		if errors.Is(err, e) {
			return code
		}
	}
	return "save"
}

// dateFormatRow - a catalog row with its example on today's date.
type dateFormatRow struct {
	entity.DateFormat
	Example string
}

// DateFormatList handles GET /admin/config/date-formats.
func (c *Controller) DateFormatList(w http.ResponseWriter, r *http.Request) {
	formats, err := c.dateFormats.DateFormats(r.Context())
	if err != nil {
		c.LogFailure(err, "error listing date formats")
	}
	now := time.Now().In(sitetime.FromContext(r.Context()))
	rows := make([]dateFormatRow, len(formats))
	for i, f := range formats {
		rows[i] = dateFormatRow{DateFormat: f, Example: dateformat.Format(now, f.Pattern, i18n.FromContext(r.Context()))}
	}
	data := c.getAdminData(r)
	data["title"] = i18n.T(r.Context(), "Date formats")
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", i18n.T(r.Context(), "Date formats"))}
	data["formats"] = rows
	data["defaultFormat"] = entity.DefaultDateFormat
	data["success"] = r.URL.Query().Get("success")
	data["error"] = r.URL.Query().Get("error")
	data["usage"] = r.URL.Query().Get("usage")
	c.RenderOrFail(w, r, "admin/date-formats/index.html", data)
}

// DateFormatNew handles GET /admin/config/date-formats/new.
func (c *Controller) DateFormatNew(w http.ResponseWriter, r *http.Request) {
	c.renderDateFormatForm(w, r, entity.DateFormat{})
}

// DateFormatEdit handles GET /admin/config/date-formats/{id}.
func (c *Controller) DateFormatEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, dateFormatsAdminURL, http.StatusSeeOther)
		return
	}
	f, err := c.dateFormats.DateFormatByID(r.Context(), id)
	if err != nil {
		http.Redirect(w, r, dateFormatsAdminURL, http.StatusSeeOther)
		return
	}
	c.renderDateFormatForm(w, r, *f)
}

func (c *Controller) renderDateFormatForm(w http.ResponseWriter, r *http.Request, f entity.DateFormat) {
	data := c.getAdminData(r)
	title := i18n.T(r.Context(), "New date format")
	if f.ID != 0 {
		title = i18n.T(r.Context(), "Date format: @label", "@label", f.Label)
	}
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(r.Context(), "@title - Admin panel", "@title", title)}
	data["format"] = f
	lang := i18n.FromContext(r.Context())
	data["example"] = dateformat.Format(time.Now().In(sitetime.FromContext(r.Context())), f.Pattern, lang)
	data["letters"] = dateformat.LettersIn(lang)
	// Month names for the live example script (the same forms as Format).
	data["monthsInDate"], data["months"], data["monthsShort"] = dateformat.MonthNames(lang)
	data["weekdays"], data["weekdaysShort"] = dateformat.WeekdayNames(lang)
	data["success"] = r.URL.Query().Get("success") != ""
	data["error"] = r.URL.Query().Get("error")
	c.RenderOrFail(w, r, "admin/date-formats/edit.html", data)
}

// DateFormatSave handles POST /admin/config/date-formats (create) and
// POST /admin/config/date-formats/{id} (update).
func (c *Controller) DateFormatSave(w http.ResponseWriter, r *http.Request) {
	id := 0
	if r.PathValue("id") != "" {
		var ok bool
		if id, ok = c.PathID(r); !ok {
			http.Redirect(w, r, dateFormatsAdminURL, http.StatusSeeOther)
			return
		}
	}
	back := dateFormatsAdminURL + "/new"
	if id != 0 {
		back = dateFormatsAdminURL + "/" + strconv.Itoa(id)
	}
	saved, err := c.dateFormats.Save(r.Context(), entity.DateFormat{
		ID:          id,
		MachineName: strings.TrimSpace(r.FormValue("machine_name")),
		Label:       r.FormValue("label"),
		Pattern:     r.FormValue("pattern"),
	})
	if err != nil {
		c.logger.Warn("error saving date format", "error", err, "id", id)
		http.Redirect(w, r, back+"?error="+dateFormatErrorCode(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, dateFormatsAdminURL+"/"+strconv.Itoa(saved)+"?success=1", http.StatusSeeOther)
}

// DateFormatDelete handles POST /admin/config/date-formats/{id}/delete:
// refused for the default format and for one a field chose (listed).
func (c *Controller) DateFormatDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, dateFormatsAdminURL, http.StatusSeeOther)
		return
	}
	usage, err := c.dateFormats.Delete(r.Context(), id)
	if err != nil {
		code := dateFormatErrorCode(err)
		if code == "save" {
			c.LogFailure(err, "error deleting date format", "id", id)
		}
		q := url.Values{"error": {code}}
		if len(usage) > 0 {
			q.Set("usage", strings.Join(usage, "; "))
		}
		http.Redirect(w, r, dateFormatsAdminURL+"?"+q.Encode(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, dateFormatsAdminURL+"?success=deleted", http.StatusSeeOther)
}
