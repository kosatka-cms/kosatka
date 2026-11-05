package privatemessages

import (
	"os"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestMain registers the modules, as main does (moduleHook renders the
// registered modules' hooks).
func TestMain(m *testing.M) {
	testkit.RegisterModules()
	os.Exit(m.Run())
}
