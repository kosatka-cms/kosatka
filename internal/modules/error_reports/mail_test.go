package errorreports

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

type fakeReportSite struct {
	modulekit.SiteInformationRepository
}

func (fakeReportSite) Get(context.Context) (*modulekit.SiteInformation, error) {
	return &modulekit.SiteInformation{SiteName: "Example", SiteEmail: "site@example.com"}, nil
}

type fakeReportMail struct {
	key, to string
	data    mailtemplate.TokenData
}

func (f *fakeReportMail) Send(_ context.Context, key, to string, data mailtemplate.TokenData) error {
	f.key, f.to, f.data = key, to, data
	return nil
}

// TestMailDef - the new report e-mail on the Mail page: optional, its
// text kept there (a default), a sample of each of its placeholders.
func TestMailDef(t *testing.T) {
	def := mailDef()
	if def.Key != MailNewReport || !def.Optional || def.Load != nil || def.Default.Subject == "" || def.Default.Body == "" {
		t.Errorf("the e-mail: %+v", def)
	}
	for _, p := range mailPlaceholders {
		if !slices.Contains(def.Placeholders, p) || def.Sample[p] == "" {
			t.Errorf("placeholder %s: listed %v, sample %q", p, slices.Contains(def.Placeholders, p), def.Sample[p])
		}
	}
}

// TestNotifyAdmin - a new report mails the site's address the module's
// e-mail with the report's values: the page as an address of the site, a
// guest author, a dash for no comment.
func TestNotifyAdmin(t *testing.T) {
	mail := &fakeReportMail{}
	s := NewErrorReportService(&fakeReportRepo{}, fakeReportSite{}, nil, mail, nil)
	if _, err := s.Create(context.Background(), ErrorReportCreateInput{PageURL: "/article", SelectedText: "words selected"}, "https://example.com", "https://example.com/admin/error-reports"); err != nil {
		t.Fatal(err)
	}
	extra := mail.data.Extra
	if mail.key != MailNewReport || mail.to != "site@example.com" || mail.data.SiteName != "Example" ||
		extra["[report:page]"] != "https://example.com/article" || extra["[report:text]"] != "words selected" ||
		extra["[report:author]"] != "Guest" || extra["[report:comment]"] != "-" || extra["[report:admin-url]"] != "https://example.com/admin/error-reports" {
		t.Errorf("e-mail %s to %s: %+v", mail.key, mail.to, mail.data)
	}
}
