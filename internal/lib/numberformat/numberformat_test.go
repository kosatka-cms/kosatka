package numberformat

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/i18ntest"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestMain loads the separators from the engine's catalogs.
func TestMain(m *testing.M) {
	tr, err := i18n.Load(i18ntest.FS)
	if err != nil {
		panic(err)
	}
	i18n.SetTranslator(tr)
	os.Exit(m.Run())
}

// TestFormat - the language's separators (Russian - a no-break space), thousands from five digits of
// the whole part, the decimals asked for (or as many as there are), a
// minus sign.
func TestFormat(t *testing.T) {
	for _, tc := range []struct {
		v        float64
		decimals int
		lang     string
		want     string
	}{
		{12500, 0, "ru", "12\u00a0500"},
		{2015, 0, "ru", "2015"},
		{4.7, -1, "ru", "4,7"},
		{1234567.891, 2, "en", "1,234,567.89"},
		{1234567.891, 2, "ru", "1\u00a0234\u00a0567,89"},
		{-3.5, 1, "en", "−3.5"},
		{95, 0, "en", "95"},
	} {
		if got := Format(tc.v, tc.decimals, tc.lang); got != tc.want {
			t.Errorf("Format(%v, %d, %s) = %q, want %q", tc.v, tc.decimals, tc.lang, got, tc.want)
		}
	}
}
