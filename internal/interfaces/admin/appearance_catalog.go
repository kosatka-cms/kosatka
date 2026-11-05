package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// catalogEntry - a theme of the catalog on its page: Compatible - the
// engine release meets its min_version; Installed - the release installed
// from the same repository ("" - not installed), Update - the catalog's is
// newer; Taken - its directory is a theme's already, not this one's (a
// shipped theme, or one from another repository); Command - what installs
// or updates it.
type catalogEntry struct {
	themes.CatalogTheme
	Compatible bool
	Installed  string
	Update     bool
	Taken      bool
	Command    string
}

// AppearanceCatalog handles GET /admin/config/appearance/catalog: the
// themes' catalog (themes_catalog_url, themes.CatalogService) - each
// theme with what installs it (app theme add; the admin doesn't install),
// ?q= words filtering them.
func (c *Controller) AppearanceCatalog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := i18n.T(ctx, "Themes catalog")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	data["query"] = query

	catalog, err := c.themeCatalog.Catalog(ctx)
	switch {
	case errors.Is(err, themes.ErrNoCatalog):
		data["notConfigured"] = true
	case err != nil:
		c.LogFailure(err, "error reading the themes catalog")
		data["fetchError"] = true
	default:
		lock, lerr := themes.ReadLock()
		if lerr != nil {
			c.LogFailure(lerr, "error reading the themes lock")
		}
		var entries []catalogEntry
		for _, th := range catalog.Themes {
			if !th.Match(query) {
				continue
			}
			e := catalogEntry{CatalogTheme: th, Compatible: themes.Compatible(th.MinVersion), Command: "app theme add " + th.Source}
			if entry, ok := lock.Themes[th.Name]; ok && entry.Source == th.Source {
				e.Installed = entry.Version
				e.Update = themes.ReleaseNewer(th.Version, entry.Version)
				e.Command = ""
				if e.Update {
					e.Command = "app theme update " + th.Name
				}
			} else if paths.ThemeDir(th.Name) != "" {
				e.Taken = true
				e.Command = "app theme add " + th.Source + " -name <name>"
			}
			entries = append(entries, e)
		}
		data["entries"] = entries
		data["generated"] = catalog.Generated
		data["total"] = len(catalog.Themes)
	}
	c.RenderOrFail(w, r, "admin/config/appearance_catalog.html", data)
}
