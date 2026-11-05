package middleware

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestShouldSkipMaintenanceCheck - the core's open paths and those of the
// enabled modules pass the maintenance page; a disabled module's don't.
func TestShouldSkipMaintenanceCheck(t *testing.T) {
	entity.RegisterModule(entity.Module{Name: "albums", MaintenancePaths: []string{"/api/album/random"}})
	t.Cleanup(func() {
		entity.Modules = entity.Modules[:len(entity.Modules)-1]
		service.SetModules(map[string]bool{"albums": true})
	})
	service.SetModules(map[string]bool{"albums": true})
	for path, want := range map[string]bool{
		"/login":               true,
		"/static/js/app.js":    true,
		"/api/album/random?x":  true,
		"/news/some-article":   false,
		"/api/album/something": false,
	} {
		if got := shouldSkipMaintenanceCheck(path); got != want {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}
	service.SetModules(map[string]bool{"albums": false})
	if shouldSkipMaintenanceCheck("/api/album/random") {
		t.Error("a disabled module's path passes")
	}
}
