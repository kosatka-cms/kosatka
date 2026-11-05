package app

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
)

// TestMain registers the engine's modules, as main does, and turns the
// catalogs off.
func TestMain(m *testing.M) {
	manifests.Register()
	// No catalogs unless a test gives one (t.Setenv, -catalog,
	// -translations): the default ones are on the network.
	for _, env := range []string{"THEMES_CATALOG_URL", "TRANSLATIONS_CATALOG_URL"} {
		if err := os.Setenv(env, ""); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
