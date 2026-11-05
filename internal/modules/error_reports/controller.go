package errorreports

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/error_reports/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// ErrorReportController - the error reports' handlers: the report form's
// endpoint and the admin list with its actions.
type ErrorReportController struct {
	*modulekit.BaseController
	errorReportService *ErrorReportService
}

// ErrorReportDeps - what NewErrorReportController needs besides the BaseDeps.
type ErrorReportDeps struct {
	ErrorReportService *ErrorReportService
}

// NewErrorReportController - the error reports' handlers on the base deps
// and the reports' service.
func NewErrorReportController(deps modulekit.BaseDeps, d ErrorReportDeps) *ErrorReportController {
	base := modulekit.NewBaseController(deps)
	base.SetLogger(deps.Logger.With("component", "error_report.handler"))
	return &ErrorReportController{
		BaseController:     base,
		errorReportService: d.ErrorReportService,
	}
}

type errorReportSubmitBody struct {
	SelectedText       string `json:"selected_text"`
	Comment            string `json:"comment"`
	PageURL            string `json:"page_url"`
	GRecaptchaResponse string `json:"g-recaptcha-response"`
}

type errorReportSubmitResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// Submit handles POST /api/error-report (public, CSRF + optional reCAPTCHA).
func (c *ErrorReportController) Submit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.JSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body errorReportSubmitBody
	if err := httputil.DecodeJSON(r, &body); err != nil {
		httputil.JSONError(w, "invalid json", http.StatusBadRequest)
		return
	}
	if msg := c.recaptchaJSONError(r.Context(), body.GRecaptchaResponse, r); msg != "" {
		httputil.JSONError(w, msg, http.StatusBadRequest)
		return
	}

	var userID *int
	if uid, ok := modulekit.ContextUserID(r.Context()); ok && uid > 0 {
		userID = &uid
	}

	// The admins' e-mail links the site, not the request's host.
	siteBase := modulekit.MailBaseURL()
	listURL := siteBase + adminListURL

	in := ErrorReportCreateInput{
		UserID:       userID,
		PageURL:      body.PageURL,
		SelectedText: body.SelectedText,
		Comment:      body.Comment,
		ClientIP:     httputil.ClientIP(r),
		UserAgent:    r.UserAgent(),
	}
	_, err := c.errorReportService.Create(r.Context(), in, siteBase, listURL)
	if err != nil {
		switch {
		case errors.Is(err, ErrErrorReportSelectedText), errors.Is(err, ErrErrorReportPageURL), errors.Is(err, ErrErrorReportComment):
			httputil.JSONError(w, i18n.ErrorText(r.Context(), err), http.StatusBadRequest)
		default:
			c.LogFailure(err, "error report create")
			httputil.InternalServerError(w)
		}
		return
	}

	httputil.SendJSON(w, http.StatusOK, errorReportSubmitResponse{
		OK:      true,
		Message: i18n.T(r.Context(), "Thank you! Your report has been sent."),
	})
}

func (c *ErrorReportController) recaptchaJSONError(ctx context.Context, token string, r *http.Request) string {
	ok, err := modulekit.Captcha().Verify(ctx, manifest.CaptchaForm, token, httputil.ClientIP(r))
	if err != nil {
		c.LogFailure(err, "recaptcha verify")
		return i18n.T(ctx, "Could not verify the CAPTCHA. Please try again later.")
	}
	if !ok {
		return i18n.T(ctx, "Please confirm that you are not a robot.")
	}
	return ""
}

const (
	// adminListURL - the admin list of the reports.
	adminListURL = "/admin/error-reports"
	// adminPerPage - reports on a page of the admin list.
	adminPerPage = 25
)

// AdminList renders GET /admin/error-reports: the new reports (not
// marked fixed), newest first; ?status=all - all of them.
func (c *ErrorReportController) AdminList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := c.GetPageNumber(r)
	all := r.URL.Query().Get("status") == "all"
	reports, total, err := c.errorReportService.ListForAdmin(ctx, all, page, adminPerPage)
	if err != nil {
		c.LogFailure(err, "error listing error reports")
		httputil.InternalServerError(w)
		return
	}
	data := c.AdminPageData(r)
	data["title"] = i18n.T(ctx, "Error reports")
	data["pageSEO"] = modulekit.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", data["title"])}
	data["reports"] = reports
	data["showAll"] = all
	data["backURL"] = r.URL.RequestURI()
	data["error"] = r.URL.Query().Get("error")
	data["pagination"] = pagination.New(page, total, adminPerPage, "/admin/error-reports", r.URL.Query())
	c.RenderOrFail(w, r, "admin/error_reports.html", data)
}

// Resolve handles POST /admin/error-reports/{id}/resolve: marks the report
// fixed and goes back to the list it was sent from (the form's "back").
func (c *ErrorReportController) Resolve(w http.ResponseWriter, r *http.Request) {
	c.mark(w, r, c.errorReportService.Resolve, "resolve")
}

// Reopen handles POST /admin/error-reports/{id}/reopen: returns a fixed
// report to the new ones, back to the list it was sent from.
func (c *ErrorReportController) Reopen(w http.ResponseWriter, r *http.Request) {
	c.mark(w, r, c.errorReportService.Reopen, "reopen")
}

// mark runs set on the report of the path and goes back to the form's
// "back" (a page of the list); a failure - to the list with ?error=code.
func (c *ErrorReportController) mark(w http.ResponseWriter, r *http.Request, set func(context.Context, int) error, code string) {
	back := adminListURL
	if b := r.FormValue("back"); strings.HasPrefix(b, adminListURL) {
		back = b
	}
	id, ok := c.PathID(r)
	if !ok {
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}
	if err := set(r.Context(), id); err != nil {
		c.LogFailure(err, "marking the error report ("+code+")", "reportID", id)
		http.Redirect(w, r, adminListURL+"?error="+code, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// PageData - every page's data for the report form (AddPageData): its
// CAPTCHA site key, when the CAPTCHA protects it.
func PageData(_ *http.Request, data map[string]any) {
	if modulekit.Captcha() == nil {
		return
	}
	if key := modulekit.Captcha().SiteKey(manifest.CaptchaForm); key != "" {
		data["error_report_recaptcha_key"] = key
		data["recaptchaScript"] = true
	}
}
