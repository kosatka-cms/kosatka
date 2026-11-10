package entity

import (
	"slices"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// AdminMenuSection - a section of the admin toolbar: a link (Path set, no
// Items) or a dropdown of Items.
type AdminMenuSection struct {
	Label string
	Path  string
	// Exact - the section is current on its Path only, not the pages
	// under it (the Overview, /admin).
	Exact bool
	Items []AdminMenuItem
	// CreateContent - the section ends with the "Create content" submenu
	// (the user's creatable types).
	CreateContent bool
}

// AdminMenuItem - a link in a section. Group starts a titled group within
// the dropdown; Module/Feature hide the item while that module or feature
// is off; Permission - needed besides access to the admin. Count - the
// number shown by it (what waits there, e.g. new reports): set for each
// page from its counter (service.RegisterAdminMenuCount), 0 - none;
// CountLabel - what the number counts where the Overview lists it ("" -
// the item's Label).
type AdminMenuItem struct {
	Label      string
	Path       string
	Group      string
	Module     string
	Feature    string
	Permission string
	Count      int
	CountLabel string
}

// Admin menu groups (AdminMenuItem.Group) of the Configuration section;
// AdminGroupModules - the modules' settings pages, AdminGroupTexts - the
// texts' and files', AdminGroupSite - the site's own (a module's page may
// join them too).
var (
	AdminGroupSite     = i18n.N("Site")
	AdminGroupTexts    = i18n.N("Texts and files")
	adminGroupSearch   = i18n.N("Search and SEO")
	AdminGroupModules  = i18n.N("Modules")
	adminGroupSecurity = i18n.N("Security")
)

// Admin menu sections a module's page joins (ModuleMenuItem.Section):
// their labels.
var (
	AdminSectionContent       = i18n.N("Content")
	AdminSectionStructure     = i18n.N("Structure")
	AdminSectionConfiguration = i18n.N("Configuration")
	AdminSectionReports       = i18n.N("Reports")
)

// AdminMenu - the admin toolbar, in order: the core's sections with the
// registered modules' pages added to theirs (each item tagged with its
// module): after the last item of the item's group, else at the end.
// Labels are i18n.N sources, translated when rendered.
func AdminMenu() []AdminMenuSection {
	out := make([]AdminMenuSection, len(adminMenu))
	for i, sec := range adminMenu {
		sec.Items = append([]AdminMenuItem(nil), sec.Items...)
		for _, m := range Modules {
			for _, it := range m.AdminMenu {
				if it.Section == sec.Label {
					item := it.Item
					item.Module = m.Name
					sec.Items = insertInGroup(sec.Items, item)
				}
			}
		}
		out[i] = sec
	}
	return out
}

// insertInGroup - items with item after the last one of its group (a
// group's items stay together under its title), else at the end.
func insertInGroup(items []AdminMenuItem, item AdminMenuItem) []AdminMenuItem {
	at := len(items)
	if item.Group != "" {
		for i := len(items) - 1; i >= 0; i-- {
			if items[i].Group == item.Group {
				at = i + 1
				break
			}
		}
	}
	return slices.Insert(items, at, item)
}

// adminMenu - the core's admin toolbar.
var adminMenu = []AdminMenuSection{
	{Label: i18n.N("Overview"), Path: "/admin", Exact: true},
	{Label: AdminSectionContent, CreateContent: true, Items: []AdminMenuItem{
		{Label: i18n.N("Content"), Path: "/admin/content"},
		{Label: i18n.N("Changes"), Path: "/admin/content/changes", Permission: "view content revisions"},
		{Label: i18n.N("Pending moderation"), Path: "/admin/moderation"},
		{Label: i18n.N("Comments"), Path: "/admin/content/comment"},
		{Label: i18n.N("Media library"), Path: "/admin/content/media", Permission: "administer content"},
		{Label: i18n.N("Users' files"), Path: "/admin/content/files", Permission: "administer user files"},
		{Label: i18n.N("Trash"), Path: "/admin/content/trash", Permission: "administer content"},
	}},
	{Label: AdminSectionStructure, Items: []AdminMenuItem{
		{Label: i18n.N("Content types"), Path: "/admin/config/content-types"},
		{Label: i18n.N("Field sets"), Path: "/admin/config/field-sets"},
		{Label: i18n.N("Taxonomy"), Path: "/admin/config/taxonomy"},
		{Label: i18n.N("Views (lists)"), Path: "/admin/config/views"},
		{Label: i18n.N("Menus"), Path: "/admin/config/menus"},
		{Label: i18n.N("Blocks"), Path: "/admin/config/blocks"},
		{Label: i18n.N("Breadcrumbs"), Path: "/admin/config/breadcrumbs"},
	}},
	{Label: i18n.N("Appearance"), Path: "/admin/config/appearance"},
	{Label: i18n.N("Modules"), Path: "/admin/config/modules"},
	{Label: AdminSectionConfiguration, Items: []AdminMenuItem{
		{Label: i18n.N("Site information"), Path: "/admin/config/system/site-information", Group: AdminGroupSite},
		{Label: i18n.N("Date formats"), Path: "/admin/config/date-formats", Group: AdminGroupSite},
		{Label: i18n.N("Interface translations"), Path: "/admin/config/translations", Group: AdminGroupSite},
		{Label: i18n.N("Maintenance mode"), Path: "/admin/config/maintenance", Group: AdminGroupSite},
		{Label: i18n.N("Performance"), Path: "/admin/config/performance", Group: AdminGroupSite},
		{Label: i18n.N("Statistics"), Path: "/admin/config/statistics", Group: AdminGroupSite},
		{Label: i18n.N("Mail"), Path: "/admin/config/mail", Group: AdminGroupSite, Permission: "full_access"},
		{Label: i18n.N("Text formats"), Path: "/admin/config/text-formats", Group: AdminGroupTexts},
		{Label: i18n.N("File upload policy"), Path: "/admin/config/upload-settings", Group: AdminGroupTexts},
		{Label: i18n.N("Image styles"), Path: "/admin/config/image-styles", Group: AdminGroupTexts},
		{Label: i18n.N("Search"), Path: "/admin/config/search", Group: adminGroupSearch},
		{Label: "SEO", Path: "/admin/config/seo", Group: adminGroupSearch},
		{Label: i18n.N("URL aliases"), Path: "/admin/config/url-aliases", Group: adminGroupSearch},
		{Label: i18n.N("Redirects"), Path: "/admin/config/redirects", Group: adminGroupSearch},
		{Label: i18n.N("Counters and codes"), Path: "/admin/config/integrations", Group: adminGroupSearch},
		{Label: i18n.N("Comments"), Path: "/admin/config/comments", Group: adminGroupSecurity, Permission: "administer comments"},
		{Label: i18n.N("Access blocking"), Path: "/admin/config/robots", Group: adminGroupSecurity},
		{Label: i18n.N("Spam protection (reCAPTCHA)"), Path: "/admin/config/captcha", Group: adminGroupSecurity, Permission: "full_access"},
	}},
	{Label: i18n.N("Users"), Items: []AdminMenuItem{
		{Label: i18n.N("User list"), Path: "/admin/users", Permission: "administer users"},
		{Label: i18n.N("Roles"), Path: "/admin/users/roles", Permission: "administer permissions"},
		{Label: i18n.N("Permissions"), Path: "/admin/users/permissions", Permission: "administer permissions"},
		{Label: i18n.N("User fields"), Path: "/admin/users/fields", Permission: "administer users"},
		{Label: i18n.N("Account settings"), Path: "/admin/config/account", Permission: "administer users"},
		{Label: i18n.N("Social login"), Path: "/admin/config/oauth", Permission: "full_access"},
	}},
	{Label: AdminSectionReports, Items: []AdminMenuItem{
		{Label: i18n.N("Status report"), Path: "/admin/reports/status"},
		{Label: i18n.N("Popular content"), Path: "/admin/reports/popular"},
	}},
}

// AdminMenuCurrent - the section and item of menu path belongs to: the
// item whose Path is path or a prefix of it at a "/" (the longest such);
// a section that is a link counts as its own item. False - none.
func AdminMenuCurrent(menu []AdminMenuSection, path string) (AdminMenuSection, AdminMenuItem, bool) {
	var section AdminMenuSection
	var item AdminMenuItem
	under := func(p string) bool {
		return p != "" && (path == p || strings.HasPrefix(path, strings.TrimSuffix(p, "/")+"/")) && len(p) > len(item.Path)
	}
	for _, s := range menu {
		if s.Path != "" && (s.Exact && path == s.Path || !s.Exact && under(s.Path)) {
			section, item = s, AdminMenuItem{Label: s.Label, Path: s.Path}
		}
		for _, it := range s.Items {
			if under(it.Path) {
				section, item = s, it
			}
		}
	}
	return section, item, item.Path != ""
}
