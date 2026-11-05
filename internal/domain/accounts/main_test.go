package accounts

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/i18ntest"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestMain loads the translation catalogs; output is in the source
// language (English) unless a test asks for another one.
func TestMain(m *testing.M) {
	tr, err := i18n.Load(i18ntest.FS)
	if err != nil {
		panic(err)
	}
	i18n.SetTranslator(tr)
	i18n.SetDefault(i18n.SourceLanguage)
	os.Exit(m.Run())
}
