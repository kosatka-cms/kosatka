package domain_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLayerImports - the domain's dependency direction
// (docs/contributing/architecture.md): entity uses only pkg/* and
// internal/lib/* (the helpers knowing nothing of the domain); service uses
// entity, the domain packages below it (views, themes, assets, fallback)
// and those helpers, never the infrastructure (ent, the OAuth adapter…) or the
// config, which the adapters and internal/app translate into entity types.
func TestLayerImports(t *testing.T) {
	const module = "github.com/kosatka-cms/kosatka/"
	for dir, allowed := range map[string][]string{
		"entity":  {module + "pkg/", module + "internal/lib/"},
		"service": {module + "pkg/", module + "internal/lib/", module + "internal/domain/", module + "internal/paths", module + "internal/buildinfo"},
		// Views come under the template service and the block cache
		// (service imports views), never the other way round.
		"views": {module + "pkg/", module + "internal/lib/", module + "internal/domain/entity", module + "internal/domain/imageurl", module + "internal/domain/fallback", module + "internal/paths"},
		// The themes (their manifests, settings, catalog, lock) come under
		// the template service, the blocks and the admin (service imports
		// themes), never the other way round.
		"themes": {module + "pkg/", module + "internal/lib/", module + "internal/domain/entity", module + "internal/domain/assets", module + "internal/domain/fallback", module + "internal/paths", module + "internal/buildinfo"},
		// The static files' versioned URLs and the fallbacks' logging sit
		// below service, views and themes.
		"assets": {module + "pkg/", module + "internal/lib/", module + "internal/paths", module + "internal/buildinfo"},
		// The actions an action log keeps (who did what) sit below
		// service: the services and the HTTP layer record them.
		"audit":    {module + "pkg/", module + "internal/lib/"},
		"fallback": {module + "pkg/", module + "internal/lib/", module + "internal/domain/entity"},
		// The web paths of public files and their image-style derivatives
		// sit below service and views.
		"imageurl": {module + "pkg/", module + "internal/lib/", module + "internal/domain/entity", module + "internal/paths"},
		// The uploaded files (validation, storage, the users' own files)
		// sit below service: nothing here uses the services.
		"files": {module + "pkg/", module + "internal/lib/", module + "internal/domain/entity", module + "internal/domain/imageurl", module + "internal/paths"},
		// The sign-in through OAuth providers comes above service (it uses
		// the users' and the auth services; nothing in service uses it).
		"sociallogin": {module + "pkg/", module + "internal/lib/", module + "internal/domain/entity", module + "internal/domain/service", module + "internal/domain/accounts"},
		// The accounts (registration, password reset, account settings,
		// profile fields) come above service like sociallogin: they use
		// the users' service; nothing in service uses them.
		"accounts": {module + "pkg/", module + "internal/lib/", module + "internal/domain/entity", module + "internal/domain/service"},
	} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if !strings.HasPrefix(path, module) {
					continue
				}
				ok := false
				for _, a := range allowed {
					ok = ok || strings.HasPrefix(path, a)
				}
				if !ok {
					t.Errorf("%s imports %s", file, path)
				}
			}
		}
	}
}
