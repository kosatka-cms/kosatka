package subscriptions

import (
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
)

// TestMailDef - the notifications e-mail on the Mail page: the module's,
// always sent (the users choose theirs), its text kept by the module, a
// sample of each of its own placeholders.
func TestMailDef(t *testing.T) {
	def := NewRepo(nil).MailDef()
	if def.Key != MailNotifications || def.Module != manifest.Name || def.Optional || def.Load == nil || def.Save == nil {
		t.Errorf("the e-mail: %+v", def)
	}
	for _, p := range MailPlaceholders {
		if !slices.Contains(def.Placeholders, p) || def.Sample[p] == "" {
			t.Errorf("placeholder %s: listed %v, sample %q", p, slices.Contains(def.Placeholders, p), def.Sample[p])
		}
	}
}
