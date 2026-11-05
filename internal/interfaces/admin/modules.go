package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// saveFeature stores a feature switch and applies it at once.
func (c *Controller) saveFeature(ctx context.Context, name string, enabled bool) error {
	if c.modules == nil {
		return errors.New("module states store not set")
	}
	if err := c.modules.SetEnabled(ctx, name, enabled); err != nil {
		return err
	}
	service.SetModules(map[string]bool{name: enabled})
	if c.modulesChanged != nil {
		c.modulesChanged()
	}
	return nil
}

// moduleRow - a module on the Modules page.
type moduleRow struct {
	Name        string
	Label       string
	Description string
	Enabled     bool
	// External - from its own repository, not the engine's.
	External bool
	// Installed - its tables exist (it was enabled once, not uninstalled).
	Installed bool
	// UninstallBlocked - why an installed, disabled module can't be
	// uninstalled now ("" - it can).
	UninstallBlocked string
	// Details - what the module includes (hidden while it is off).
	Details []moduleDetail
}

// moduleGroup - a group of the Modules page (entity.ModuleCategories):
// its modules in the registry's order, how many of them are on.
type moduleGroup struct {
	Name    string
	Label   string
	Modules []moduleRow
	On      int
}

// moduleGroups - rows by their modules' categories, in the page's order;
// the empty groups left out.
func moduleGroups(ctx context.Context, rows []moduleRow) []moduleGroup {
	byCat := map[string][]moduleRow{}
	for _, row := range rows {
		m, _ := entity.ModuleByName(row.Name)
		cat := m.CategoryName()
		byCat[cat] = append(byCat[cat], row)
	}
	var out []moduleGroup
	for _, c := range entity.ModuleCategories() {
		if len(byCat[c.Name]) == 0 {
			continue
		}
		g := moduleGroup{Name: c.Name, Label: i18n.TC(ctx, entity.ModuleCategoryContext, c.Label), Modules: byCat[c.Name]}
		for _, row := range g.Modules {
			if row.Enabled {
				g.On++
			}
		}
		out = append(out, g)
	}
	return out
}

// moduleDetail - one kind of thing a module includes, with its items.
type moduleDetail struct {
	Heading string
	Items   []string
}

// moduleDetailSources - what moduleDetails counts (the page's
// dependencies; tests stub them).
type moduleDetailSources struct {
	countRows        func(ctx context.Context, table string) (int, error)
	pluginPlacements func(ctx context.Context, ids []entity.PluginBlockID) ([]entity.PluginPlacementRef, error)
	countNodes       func(ctx context.Context, nodeType string) (int, error)
	typeLabels       map[string]string
	roles            entity.Roles
	regionLabel      func(theme string, region entity.Region) string
}

// moduleDetails lists what the module includes: content types (with how
// many items), placed plugin blocks, permissions (with how many roles
// have them), admin sections, data tables (with row counts) and its own
// files. Counts are on demand; a failed count leaves the number out.
func moduleDetails(ctx context.Context, m entity.Module, src moduleDetailSources, logger interface {
	Warn(msg string, args ...any)
}) []moduleDetail {
	var out []moduleDetail
	add := func(heading string, items []string) {
		if len(items) > 0 {
			out = append(out, moduleDetail{Heading: heading, Items: items})
		}
	}
	withCount := func(label string, n int, err error) string {
		if err != nil {
			logger.Warn("error counting for the Modules page", "module", m.Name, "item", label, "error", err)
			return label
		}
		return label + " - " + strconv.Itoa(n)
	}

	add(i18n.T(ctx, "Requires"), moduleLabelList(ctx, m.Requires))
	add(i18n.T(ctx, "Required by"), moduleLabelList(ctx, entity.ModulesRequiring(m.Name)))

	var types []string
	for _, nt := range m.NodeTypes {
		label := src.typeLabels[nt.Name]
		if label == "" {
			// A module's built-in type has no content_types row.
			label = i18n.T(ctx, nt.Label)
		}
		n, err := src.countNodes(ctx, nt.Name)
		types = append(types, withCount(label, n, err))
	}
	add(i18n.T(ctx, "Content types"), types)

	var blocks []string
	if len(m.PluginBlocks) > 0 {
		ids := make([]entity.PluginBlockID, len(m.PluginBlocks))
		for i, b := range m.PluginBlocks {
			ids[i] = b.ID
		}
		placements, err := src.pluginPlacements(ctx, ids)
		if err != nil {
			logger.Warn("error listing plugin placements for the Modules page", "module", m.Name, "error", err)
		}
		for _, b := range m.PluginBlocks {
			id, label := b.ID, i18n.T(ctx, b.Label)
			var where []string
			for _, p := range placements {
				if p.PluginID == id {
					where = append(where, p.Theme+": "+src.regionLabel(p.Theme, p.Region))
				}
			}
			if len(where) == 0 {
				blocks = append(blocks, label+" - "+i18n.T(ctx, "not placed"))
			} else {
				blocks = append(blocks, label+" - "+strings.Join(where, ", "))
			}
		}
	}
	add(i18n.T(ctx, "Blocks"), blocks)

	var perms []string
	for _, p := range m.Permissions {
		roles := 0
		for i := range src.roles {
			if service.HasGrant(&src.roles[i], p.Key) {
				roles++
			}
		}
		perms = append(perms, i18n.T(ctx, p.Label)+" - "+i18n.T(ctx, "roles: @count", "@count", roles))
	}
	add(i18n.T(ctx, "Permissions"), perms)

	var sections []string
	for _, sec := range entity.AdminMenu() {
		for _, it := range sec.Items {
			if it.Module == m.Name {
				sections = append(sections, i18n.T(ctx, sec.Label)+" → "+i18n.T(ctx, it.Label))
			}
		}
	}
	add(i18n.T(ctx, "Admin sections"), sections)

	var data []string
	for _, d := range m.Data {
		n, err := src.countRows(ctx, d.Table)
		data = append(data, withCount(i18n.T(ctx, d.Label), n, err))
	}
	add(i18n.T(ctx, "Data (kept when off)"), data)

	var files []string
	for _, f := range m.CSS {
		files = append(files, f+".css")
	}
	for _, f := range m.JS {
		files = append(files, f+".js")
	}
	add(i18n.T(ctx, "Files"), files)
	return out
}

// moduleLabel - a module's translated label (the name if unknown).
func moduleLabel(ctx context.Context, name string) string {
	if m, ok := entity.ModuleByName(name); ok {
		return i18n.T(ctx, m.Label)
	}
	return name
}

// moduleLabelList - the labels of the named modules (unknown names left
// out).
func moduleLabelList(ctx context.Context, names []string) []string {
	var out []string
	for _, n := range names {
		if _, ok := entity.ModuleByName(n); ok {
			out = append(out, moduleLabel(ctx, n))
		}
	}
	return out
}

// moduleLabels - "label", … of the named modules.
func moduleLabels(ctx context.Context, names []string) string {
	labels := moduleLabelList(ctx, names)
	for i, l := range labels {
		labels[i] = "\"" + l + "\""
	}
	return strings.Join(labels, ", ")
}

// moduleRequiresMessage - why the save was refused; only known module
// names from the query make it into the text.
func moduleRequiresMessage(ctx context.Context, module, needs string) string {
	if _, ok := entity.ModuleByName(module); !ok {
		return ""
	}
	return i18n.T(ctx, "\"@module\" requires @modules: enable them too, or switch \"@module\" off.",
		"@module", moduleLabel(ctx, module), "@modules", moduleLabels(ctx, strings.Split(needs, ",")))
}

// moduleDetailSourcesFor - the page's real sources.
func (c *Controller) moduleDetailSourcesFor(ctx context.Context) moduleDetailSources {
	src := moduleDetailSources{
		countRows:        c.modules.CountRows,
		pluginPlacements: c.modules.PluginPlacements,
		countNodes:       c.nodeRepo.CountByType,
		typeLabels:       map[string]string{},
	}
	if types, err := c.contentTypeService.ContentTypes(ctx); err == nil {
		for _, t := range types {
			src.typeLabels[t.MachineName] = t.Label
		}
	} else {
		c.LogFailure(err, "listing the content types")
	}
	if c.roleRepo != nil {
		if roles, err := c.roleRepo.List(ctx); err == nil && roles != nil {
			src.roles = *roles
		}
	}
	infos := map[string]entity.ThemeInfo{}
	src.regionLabel = func(theme string, region entity.Region) string {
		info, ok := infos[theme]
		if !ok {
			dir := theme
			if dir == "default" {
				dir = ""
			}
			info, _ = themes.LoadInfoFor(dir)
			infos[theme] = info
		}
		for _, reg := range info.Regions {
			if reg.ID == region && reg.Label != "" {
				return i18n.T(ctx, reg.Label)
			}
		}
		return string(region)
	}
	return src
}

// ModulesPage handles GET/POST /admin/config/modules: switching the
// built-in modules on and off.
func (c *Controller) ModulesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodPost {
		if c.modules == nil {
			http.Redirect(w, r, "/admin/config/modules?error=1", http.StatusSeeOther)
			return
		}
		states := make(map[string]bool, len(entity.Modules))
		for _, m := range entity.Modules {
			states[m.Name] = r.FormValue("module_"+m.Name) == "1"
		}
		// A module needs the modules it requires (entity.Module.Requires):
		// nothing is saved while one of them would be off.
		if missing := entity.ModuleRequirementsMissing(states); len(missing) > 0 {
			for _, m := range entity.Modules {
				if reqs, ok := missing[m.Name]; ok {
					http.Redirect(w, r, "/admin/config/modules?error=requires&module="+m.Name+"&needs="+strings.Join(reqs, ","), http.StatusSeeOther)
					return
				}
			}
		}
		// A module enabled for the first time is installed: its tables are
		// created (its schema migrations) and its install step runs.
		// Nothing is saved if that fails.
		if c.installModule != nil {
			for _, m := range entity.Modules {
				if !states[m.Name] {
					continue
				}
				if err := c.installModule(ctx, m.Name); err != nil {
					c.LogFailure(err, "error installing module", "module", m.Name)
					http.Redirect(w, r, "/admin/config/modules?error=install&module="+m.Name, http.StatusSeeOther)
					return
				}
			}
		}
		for _, m := range entity.Modules {
			enabled := states[m.Name]
			if err := c.modules.SetEnabled(ctx, m.Name, enabled); err != nil {
				c.LogFailure(err, "error saving module state", "module", m.Name)
				http.Redirect(w, r, "/admin/config/modules?error=1", http.StatusSeeOther)
				return
			}
		}
		// The action log before the new states: disabling the log itself
		// is still recorded.
		for _, m := range entity.Modules {
			if was := service.ModuleEnabled(m.Name); was != states[m.Name] {
				t := audit.ModuleDisable
				if states[m.Name] {
					t = audit.ModuleEnable
				}
				audit.Record(ctx, audit.Action{Type: t, Object: audit.Object{Kind: audit.KindModule, ID: m.Name, Label: moduleLabel(ctx, m.Name), URL: "/admin/config/modules"}})
			}
		}
		service.SetModules(states)
		if c.modulesChanged != nil {
			c.modulesChanged()
		}
		http.Redirect(w, r, "/admin/config/modules?saved=1", http.StatusSeeOther)
		return
	}

	rows := make([]moduleRow, 0, len(entity.Modules))
	var src moduleDetailSources
	if c.modules != nil {
		src = c.moduleDetailSourcesFor(ctx)
	}
	for _, m := range entity.Modules {
		row := moduleRow{
			Name:        m.Name,
			Label:       i18n.T(ctx, m.Label),
			Description: i18n.T(ctx, m.Description),
			Enabled:     service.ModuleEnabled(m.Name),
			External:    m.External,
		}
		if c.modules != nil {
			row.Details = moduleDetails(ctx, m, src, c.logger)
		}
		if c.uninstall != nil {
			if installed, err := c.uninstall.Installed(ctx, m.Name); err != nil {
				c.logger.Warn("error checking a module's install state", "module", m.Name, "error", err)
				row.Installed = true
			} else {
				row.Installed = installed
			}
			// Content doesn't block the link: the uninstall page deletes it
			// first.
			if row.Installed && !row.Enabled {
				if err := c.uninstall.Check(ctx, m.Name); err != nil && !errors.Is(err, service.ErrModuleHasContent) {
					row.UninstallBlocked = i18n.ErrorText(ctx, err)
				}
			}
		}
		rows = append(rows, row)
	}
	title := i18n.T(ctx, "Modules")
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title)}
	data["groups"] = moduleGroups(ctx, rows)
	data["saved"] = r.URL.Query().Get("saved") == "1"
	data["uninstallAvailable"] = c.uninstall != nil
	q := r.URL.Query()
	if name := q.Get("uninstalled"); name != "" {
		if _, ok := entity.ModuleByName(name); ok {
			data["uninstalled"] = moduleLabel(ctx, name)
		}
	}
	data["error"] = q.Get("error") == "1"
	if q.Get("error") == "install" {
		if _, ok := entity.ModuleByName(q.Get("module")); ok {
			data["requiresError"] = i18n.T(ctx, "Could not install \"@module\": its tables were not created. The details are in the log.", "@module", moduleLabel(ctx, q.Get("module")))
		}
	}
	if q.Get("error") == "requires" {
		data["requiresError"] = moduleRequiresMessage(ctx, q.Get("module"), q.Get("needs"))
	}
	c.RenderOrFail(w, r, "admin/modules/index.html", data)
}

// ModuleUninstallPage handles GET/POST /admin/config/modules/{name}/uninstall:
// what uninstalling the module removes, confirmed by typing its machine
// name; POST uninstalls it (service.ModuleUninstall).
func (c *Controller) ModuleUninstallPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m, ok := entity.ModuleByName(r.PathValue("name"))
	if !ok || c.uninstall == nil || c.modules == nil {
		c.RenderNotFound(w, r)
		return
	}
	self := "/admin/config/modules/" + m.Name + "/uninstall"
	if r.Method == http.MethodPost {
		if strings.TrimSpace(r.FormValue("confirm")) != m.Name {
			http.Redirect(w, r, self+"?error=confirm", http.StatusSeeOther)
			return
		}
		if err := c.uninstall.Run(ctx, m.Name); err != nil {
			// A refusal (content left, module on) is not a failure.
			var refusal *i18n.Error
			if errors.As(err, &refusal) {
				c.logger.Warn("module uninstall refused", "module", m.Name, "reason", err)
			} else {
				c.LogFailure(err, "error uninstalling module", "module", m.Name)
			}
			http.Redirect(w, r, self+"?error=run", http.StatusSeeOther)
			return
		}
		c.logger.Info("module uninstalled", "module", m.Name)
		if c.modulesChanged != nil {
			c.modulesChanged()
		}
		http.Redirect(w, r, "/admin/config/modules?uninstalled="+m.Name, http.StatusSeeOther)
		return
	}

	label := i18n.T(ctx, m.Label)
	title := i18n.T(ctx, "Uninstall \"@module\"", "@module", label)
	data := c.getAdminData(r)
	data["title"] = title
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "@title - Admin panel", "@title", title), NoIndex: true}
	data["module"] = m
	data["moduleLabel"] = label
	if err := c.uninstall.Check(ctx, m.Name); errors.Is(err, service.ErrModuleHasContent) {
		data["hasContent"] = true
	} else if err != nil {
		data["blocked"] = i18n.ErrorText(ctx, err)
	}
	q := r.URL.Query()
	if n, err := strconv.Atoi(q.Get("deleted")); err == nil {
		data["deletedCount"] = n
		data["deletedMore"] = q.Get("more") == "1"
	}
	details := moduleDetails(ctx, m, c.moduleDetailSourcesFor(ctx), c.logger)
	var core []string
	for _, d := range m.CoreData {
		core = append(core, i18n.T(ctx, d))
	}
	if len(core) > 0 {
		details = append(details, moduleDetail{Heading: i18n.T(ctx, "Data in core tables"), Items: core})
	}
	if c.uninstall.Store != nil && c.uninstall.FileColumns != nil {
		if cols := c.uninstall.FileColumns(m.Name); len(cols) > 0 {
			if ids, err := c.uninstall.Store.FileIDs(ctx, cols); err != nil {
				c.logger.Warn("error counting a module's files", "module", m.Name, "error", err)
			} else {
				details = append(details, moduleDetail{Heading: i18n.T(ctx, "Files"),
					Items: []string{i18n.T(ctx, "Files of its tables - @count (those nothing else uses are deleted)", "@count", len(ids))}})
			}
		}
	}
	data["details"] = details
	switch q.Get("error") {
	case "confirm":
		data["errorText"] = i18n.T(ctx, "Type the module's machine name to confirm.")
	case "run":
		data["errorText"] = i18n.T(ctx, "Could not uninstall the module. The details are in the log; running it again finishes what is left.")
	}
	c.RenderOrFail(w, r, "admin/modules/uninstall.html", data)
}

// ModuleContentDelete handles POST /admin/config/modules/{name}/delete-content:
// the module's content (its content types' items, each with everything
// attached) is deleted before the uninstall, confirmed by the machine name;
// a large site takes several runs.
func (c *Controller) ModuleContentDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m, ok := entity.ModuleByName(r.PathValue("name"))
	if !ok || c.uninstall == nil {
		c.RenderNotFound(w, r)
		return
	}
	self := "/admin/config/modules/" + m.Name + "/uninstall"
	if strings.TrimSpace(r.FormValue("confirm")) != m.Name {
		http.Redirect(w, r, self+"?error=confirm", http.StatusSeeOther)
		return
	}
	deleted, more, err := c.uninstall.DeleteContent(ctx, m.Name, time.Now().Add(interfaces.StepBudget()))
	c.logger.Info("module content deleted", "module", m.Name, "deleted", deleted, "more", more, "error", err)
	if err != nil {
		http.Redirect(w, r, self+"?error=run&deleted="+strconv.Itoa(deleted), http.StatusSeeOther)
		return
	}
	u := self + "?deleted=" + strconv.Itoa(deleted)
	if more {
		u += "&more=1"
	}
	http.Redirect(w, r, u, http.StatusSeeOther)
}
