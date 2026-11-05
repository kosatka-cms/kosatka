package interfaces

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/i18ntest"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestMain registers the engine's modules (the real registry) and loads
// the translation catalogs; output is in the source language (English)
// unless a test asks for another one.
func TestMain(m *testing.M) {
	manifests.Register()
	tr, err := i18n.Load(i18ntest.FS)
	if err != nil {
		panic(err)
	}
	i18n.SetTranslator(tr)
	i18n.SetDefault(i18n.SourceLanguage)
	os.Exit(m.Run())
}
