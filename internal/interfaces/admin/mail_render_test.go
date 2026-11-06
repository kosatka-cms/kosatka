package admin

import (
	"context"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestMailSettingsPageRender - the Mail page's ways of sending: SMTP and
// a module's, each with its section; a secret field never shows the saved
// value, a plain one does; the warnings of a saved transport the binary
// lacks and of a transport whose module is off.
func TestMailSettingsPageRender(t *testing.T) {
	useTestMailTransport(t)
	saved := entity.MailSettings{Enabled: true, Transport: "testapi", FromAddress: "site@example.com", TransportSettings: testapiSettings("k_secret", "eu")}
	out := renderMailPage(t, "admin/config/mail.html", map[string]any{
		"settings":   &saved,
		"transport":  "testapi",
		"transports": mailTransportSections(context.Background(), saved),
		"status":     service.MailStatus{Enabled: true, Source: service.MailSourceAdmin, Transport: "testapi", TransportTitle: "Test API", Missing: "gone", ModuleOff: true},
	})
	for _, want := range []string{
		`id="mail-transport-smtp" name="transport" value="smtp" >`,
		`id="mail-transport-testapi" name="transport" value="testapi" checked>`, `<label class="form-check-label" for="mail-transport-testapi">Test API</label>`,
		`<h2 class="admin-settings-title">Test API</h2>`,
		`<input type="password" id="mail-ts.testapi.key" name="ts.testapi.key"`, `placeholder="saved - leave empty to keep it"`,
		`<input type="text" id="mail-ts.testapi.region" name="ts.testapi.region" class="form-control" value="eu"`,
		`<span class="admin-setting-help">Where the service is.</span>`,
		"The saved way of sending, gone, isn't in this site's binary",
		"The module of the way of sending in effect, Test API, is disabled",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if strings.Contains(out, "k_secret") {
		t.Error("the saved secret is shown")
	}
}
