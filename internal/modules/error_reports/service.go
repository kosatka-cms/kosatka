package errorreports

import (
	"context"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// The bounds of a report's fields, in characters: the selected text's
// least and greatest length, the comment's and the page address's
// greatest.
const (
	ErrorReportSelectedTextMin = 3
	ErrorReportSelectedTextMax = 10000
	ErrorReportCommentMax      = 2000
	ErrorReportPageURLMax      = 2048
)

// The errors of invalid report input (ErrorReportService.Create): the
// selected text's length out of its bounds, a page address that is not
// the site's, a comment too long.
var (
	ErrErrorReportSelectedText = i18n.Errorf("the selected text is too short or too long")
	ErrErrorReportPageURL      = i18n.Errorf("invalid page address")
	ErrErrorReportComment      = i18n.Errorf("the comment is too long")
)

// ErrorReportCreateInput is validated input for creating a report.
type ErrorReportCreateInput struct {
	UserID       *int
	PageURL      string
	SelectedText string
	Comment      string
	ClientIP     string
	UserAgent    string
}

// ErrorReportService handles error report submission and admin notifications.
type ErrorReportService struct {
	repo                ErrorReportRepository
	siteInformationRepo modulekit.SiteInformationRepository
	userRepo            modulekit.UserRepository
	emailSender         mailtemplate.Mailer
	logger              *slog.Logger
}

// NewErrorReportService creates ErrorReportService.
func NewErrorReportService(
	repo ErrorReportRepository,
	siteInformationRepo modulekit.SiteInformationRepository,
	userRepo modulekit.UserRepository,
	emailSender mailtemplate.Mailer,
	logger *slog.Logger,
) *ErrorReportService {
	return &ErrorReportService{
		repo:                repo,
		siteInformationRepo: siteInformationRepo,
		userRepo:            userRepo,
		emailSender:         emailSender,
		logger:              logger,
	}
}

// ValidateErrorReportPageURL ensures page_url is a path or an address of
// the site at siteHost (its host name, from the site's address).
func ValidateErrorReportPageURL(pageURL, siteHost string) error {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" || utf8.RuneCountInString(pageURL) > ErrorReportPageURLMax {
		return ErrErrorReportPageURL
	}
	if strings.HasPrefix(pageURL, "/") {
		if strings.Contains(pageURL, "//") {
			return ErrErrorReportPageURL
		}
		return nil
	}
	u, err := url.Parse(pageURL)
	if err != nil || u.Host == "" {
		return ErrErrorReportPageURL
	}
	host := strings.ToLower(strings.TrimSpace(siteHost))
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	if host == "" || strings.ToLower(u.Hostname()) != host {
		return ErrErrorReportPageURL
	}
	return nil
}

// siteHostOf - the host name of siteBase, the site's address
// (https://www.example.com); "" - none.
func siteHostOf(siteBase string) string {
	u, err := url.Parse(siteBase)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// Create validates input, saves the report, and emails the site admin. The
// lengths are in characters, as the form's maxlength and the columns count.
// siteBase - the site's address (modulekit.MailBaseURL): the page must be
// the site's, and the administrators' e-mail links it there, not at the
// request's host (a request can name any).
func (s *ErrorReportService) Create(ctx context.Context, in ErrorReportCreateInput, siteBase, adminListURL string) (*ErrorReport, error) {
	selected := strings.TrimSpace(in.SelectedText)
	if n := utf8.RuneCountInString(selected); n < ErrorReportSelectedTextMin || n > ErrorReportSelectedTextMax {
		return nil, ErrErrorReportSelectedText
	}
	comment := strings.TrimSpace(in.Comment)
	if utf8.RuneCountInString(comment) > ErrorReportCommentMax {
		return nil, ErrErrorReportComment
	}
	pageURL := strings.TrimSpace(in.PageURL)
	if err := ValidateErrorReportPageURL(pageURL, siteHostOf(siteBase)); err != nil {
		return nil, err
	}

	report := ErrorReport{
		UserID:       in.UserID,
		PageURL:      pageURL,
		SelectedText: selected,
		Comment:      comment,
		ClientIP:     strings.TrimSpace(in.ClientIP),
		UserAgent:    textcut.Runes(strings.TrimSpace(in.UserAgent), 512),
	}
	created, err := s.repo.Create(ctx, report)
	if err != nil {
		return nil, err
	}

	s.notifyAdmin(ctx, created, adminListURL, siteBase)
	return created, nil
}

func (s *ErrorReportService) notifyAdmin(ctx context.Context, report *ErrorReport, adminListURL, siteBase string) {
	if s.siteInformationRepo == nil || s.emailSender == nil {
		return
	}
	site, err := s.siteInformationRepo.Get(ctx)
	if err != nil || site == nil || strings.TrimSpace(site.SiteEmail) == "" {
		if s.logger != nil {
			s.logger.Warn("error report: site_email not configured, skip admin email", "report_id", report.ID)
		}
		return
	}

	author := i18n.T(ctx, "Guest")
	if report.UserID != nil && *report.UserID > 0 && s.userRepo != nil {
		if u, err := s.userRepo.ByID(ctx, *report.UserID); err == nil && u != nil {
			if strings.TrimSpace(u.Username) != "" {
				author = u.Username
			} else if strings.TrimSpace(u.Email) != "" {
				author = u.Email
			}
		}
	}

	pageLine := report.PageURL
	if strings.HasPrefix(pageLine, "/") && siteBase != "" {
		pageLine = strings.TrimSuffix(siteBase, "/") + pageLine
	}
	orDash := func(s string) string {
		if strings.TrimSpace(s) == "" {
			return "-"
		}
		return s
	}
	md := mailtemplate.TokenData{
		SiteName:     site.SiteName,
		SiteSlogan:   site.Slogan,
		SiteURLBrief: siteBase,
		SiteLoginURL: strings.TrimSuffix(siteBase, "/") + "/login",
		Extra: map[string]string{
			"[report:id]":        strconv.Itoa(report.ID),
			"[report:date]":      sitetime.Format(report.CreatedAt, "2006-01-02 15:04"),
			"[report:author]":    author,
			"[report:page]":      pageLine,
			"[report:text]":      report.SelectedText,
			"[report:comment]":   orDash(report.Comment),
			"[report:ip]":        orDash(report.ClientIP),
			"[report:admin-url]": orDash(adminListURL),
		},
	}
	if err := s.emailSender.Send(ctx, MailNewReport, site.SiteEmail, md); err != nil && s.logger != nil {
		s.logger.Error("error report admin email", "error", err, "report_id", report.ID)
	}
}

// ListForAdmin returns paginated reports for the admin UI: the new ones,
// or all with all.
func (s *ErrorReportService) ListForAdmin(ctx context.Context, all bool, page, perPage int) (ErrorReports, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 25
	}
	offset := (page - 1) * perPage
	return s.repo.ListForAdmin(ctx, ErrorReportListOpts{Limit: perPage, Offset: offset, All: all})
}

// CountNew returns the number of reports not marked fixed (the admin
// menu's count).
func (s *ErrorReportService) CountNew(ctx context.Context) (int, error) {
	return s.repo.CountNew(ctx)
}

// Resolve marks a report fixed.
func (s *ErrorReportService) Resolve(ctx context.Context, id int) error {
	return s.repo.Resolve(ctx, id)
}

// Reopen returns a fixed report to the new ones.
func (s *ErrorReportService) Reopen(ctx context.Context, id int) error {
	return s.repo.Reopen(ctx, id)
}
