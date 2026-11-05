package http

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
)

// TestMain registers the engine's modules (the real registry).
func TestMain(m *testing.M) {
	manifests.Register()
	os.Exit(m.Run())
}
