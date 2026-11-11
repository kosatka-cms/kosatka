//go:build dbintegration

package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"golang.org/x/crypto/bcrypt"

	"github.com/kosatka-cms/kosatka/internal/app/demoset"
	"github.com/kosatka-cms/kosatka/internal/app/starter"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockplacementdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/comment"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/contenttypedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/imagestyle"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/menudef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/role"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/textformatdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/user"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewdisplaydef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	galleriesent "github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestCreateSiteAtomic - a failed starter (a theme that isn't there)
// saves nothing of the site: no administrator, the default name.
func TestCreateSiteAtomic(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if _, err := r.Install(ctx, migrations.CoreSet); err != nil {
			t.Fatal(err)
		}
		client := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		password, err := createSite(ctx, r, client, "Example", "Admin", "admin@example.com", "en", siteOptions{theme: "no-such-theme"})
		if err == nil || password != "" {
			t.Fatalf("a missing theme: %q %v", password, err)
		}
		if n, err := client.User.Query().Count(ctx); err != nil || n != 0 {
			t.Errorf("users: %d %v", n, err)
		}
		if si, err := client.SiteInformation.Get(ctx, 1); err != nil || si.SiteName != "Kosatka CMS" {
			t.Errorf("site information: %+v %v", si, err)
		}
	})
}

// TestInstall - on an empty database the sets install with the core's
// rows, and createSite names the site and adds an administrator who can
// sign in with the returned password.
func TestInstall(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, s := range migrations.Sets() {
			if _, err := r.Install(ctx, s.Name); err != nil {
				t.Fatal(s.Name, err)
			}
		}
		for _, table := range []string{"site_information", "account_settings", "captcha_settings", "mail_settings", "integration_settings",
			"maintenance_settings", "statistics_settings", "upload_settings", "search_settings", "rating_settings"} {
			var n int
			if err := db.DB.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE id = 1").Scan(&n); err != nil || n != 1 {
				t.Errorf("%s: %d rows %v", table, n, err)
			}
		}

		client := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		if si, err := client.SiteInformation.Get(ctx, 1); err != nil || si.SiteName != "Kosatka CMS" {
			t.Errorf("default site name: %+v %v", si, err)
		}
		// Russian downloaded first, as app install -language ru does: the
		// starter content's labels come from it.
		var ru []entity.InterfaceString
		for source, text := range map[string]string{"Article": "Статья", "Basic HTML": "Базовый HTML", "Administrator": "Администратор",
			"Main navigation": "Главное меню", "Home": "Главная"} {
			ru = append(ru, entity.InterfaceString{Lang: "ru", Source: source, Text: text})
		}
		if err := database.NewInterfacePackRepositoryOn(client).Install(ctx, entity.InterfacePack{Lang: "ru", Version: "0.1.0", Strings: len(ru), InstalledAt: time.Now()}, ru); err != nil {
			t.Fatal(err)
		}
		password, err := createSite(ctx, r, client, "Example", "Admin User", "admin@example.com", "ru", siteOptions{})
		if err != nil {
			t.Fatal(err)
		}
		si, err := client.SiteInformation.Get(ctx, 1)
		if err != nil || si.SiteName != "Example" || si.SiteEmail != "admin@example.com" {
			t.Errorf("site information: %+v %v", si, err)
		}
		// The starter set's theme and logo, both changeable in the admin.
		set, err := starter.Load()
		if err != nil {
			t.Fatal(err)
		}
		// No logo of the site's: the theme's shows (theme.yaml "logo").
		if si.Theme == nil || *si.Theme != set.Site.Theme || (si.Logo != nil && *si.Logo != "") {
			t.Errorf("default theme and no logo: %v %v", si.Theme, si.Logo)
		}
		u, err := client.User.Query().Where(user.Username("Admin User")).WithRole().Only(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if u.Slug != entity.UserSlug("Admin User") || u.Edges.Role == nil || u.Edges.Role.Name != entity.RoleAdmin || u.Edges.Role.Permissions["full_access"] != true {
			t.Errorf("administrator: %+v role %+v", u, u.Edges.Role)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil || len(password) < 20 {
			t.Errorf("password %q: %v", password, err)
		}

		// The front page is the "frontpage" View: the published content
		// promoted to it, the sticky first, then the newest, as cards -
		// image, title, date, the body's start (trimmed).
		if si, err := client.SiteInformation.Get(ctx, 1); err != nil || si.FrontFeedView != frontpageView {
			t.Errorf("front feed: %+v %v", si, err)
		}
		display, err := client.ViewDisplayDef.Query().Where(viewdisplaydef.MachineName(frontpageView)).
			WithViewFields().WithFilters().WithSorts().Only(ctx)
		if err != nil || len(display.Edges.ViewFields) != 4 || len(display.Edges.Filters) != 3 || len(display.Edges.Sorts) != 2 || display.CreatedBy != u.ID {
			t.Errorf("frontpage View: %+v %v", display, err)
		} else {
			promoted := false
			for _, f := range display.Edges.Filters {
				promoted = promoted || (f.BuiltinField == string(entity.ViewFilterBuiltinPromote) && f.Value == "1")
			}
			first := display.Edges.Sorts[0]
			for _, srt := range display.Edges.Sorts {
				if srt.SortOrder < first.SortOrder {
					first = srt
				}
			}
			if !promoted || first.BuiltinField != string(entity.ViewSortBuiltinSticky) || first.Direction != "desc" {
				t.Errorf("frontpage View: the promote filter %v, the first sort %+v (want sticky desc)", promoted, first)
			}
			var trimmed, styled int
			for _, f := range display.Edges.ViewFields {
				if f.TrimLength > 0 {
					trimmed++
				}
				if f.ImageStyle == articleCardStyle {
					styled++
				}
			}
			if trimmed != 1 || styled != 1 {
				t.Errorf("frontpage fields: %d trimmed, %d with the card style", trimmed, styled)
			}
		}

		// Article and Page share one Body field, shown as the content's body
		// (Basic HTML offered first); the Article also has an Image, above
		// the body.
		for name, want := range map[string]int{articleType: 2, pageType: 1} {
			ct, err := client.ContentTypeDef.Query().Where(contenttypedef.MachineName(name)).Only(ctx)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			fields, err := client.FieldInstanceDef.Query().Where(fieldinstancedef.ContentTypeID(ct.ID)).WithFieldStorage().All(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if ct.Status != 1 || len(fields) != want {
				t.Fatalf("%s: %+v fields %+v", name, ct, fields)
			}
			for _, f := range fields {
				switch f.Edges.FieldStorage.MachineName {
				case "field_body":
					formats, _ := f.Config["allowed_text_formats"].([]any)
					basic, err := client.TextFormatDef.Query().Where(textformatdef.MachineName("basic_html")).OnlyID(ctx)
					if !f.RendersInBody || f.Edges.FieldStorage.FieldType != "text" || err != nil || len(formats) != 2 || int(formats[0].(float64)) != basic {
						t.Errorf("%s body: %+v (Basic HTML %d %v)", name, f, basic, err)
					}
				case "field_image":
					if name != articleType || f.Edges.FieldStorage.FieldType != "image" || f.DetailImageStyle != articleWideStyle || f.DisplayOrder >= 10 {
						t.Errorf("%s image: %+v", name, f)
					}
				default:
					t.Errorf("%s: unexpected field %s", name, f.Edges.FieldStorage.MachineName)
				}
			}
		}
		if n, err := client.FieldStorageDef.Query().Count(ctx); err != nil || n != 2 {
			t.Errorf("field storages: %d %v, want the body and the image", n, err)
		}

		// The site's language: its default and the starter content's labels.
		if si, err := client.SiteInformation.Get(ctx, 1); err != nil || si.DefaultLanguage == nil || *si.DefaultLanguage != "ru" {
			t.Errorf("default language: %+v %v", si, err)
		}
		for want, got := range map[string]func() (string, error){
			"Статья": func() (string, error) {
				ct, err := client.ContentTypeDef.Query().Where(contenttypedef.MachineName(articleType)).Only(ctx)
				if err != nil {
					return "", err
				}
				return ct.Label, nil
			},
			"Базовый HTML": func() (string, error) {
				f, err := client.TextFormatDef.Query().Where(textformatdef.MachineName("basic_html")).Only(ctx)
				if err != nil {
					return "", err
				}
				return f.Label, nil
			},
			"Администратор": func() (string, error) {
				r, err := client.Role.Query().Where(role.Name(entity.RoleAdmin)).Only(ctx)
				if err != nil {
					return "", err
				}
				return r.Description, nil
			},
		} {
			if label, err := got(); err != nil || label != want {
				t.Errorf("label %q, want %q (%v)", label, want, err)
			}
		}

		// The image styles the engine and the galleries module use exist.
		for _, name := range []string{"user_avatar_list", "user_avatar_profile", articleCardStyle, articleWideStyle, "gallery_cover", "gallery_thumbnail", "gallery_medium"} {
			if n, err := client.ImageStyle.Query().Where(imagestyle.Name(name)).Count(ctx); err != nil || n != 1 {
				t.Errorf("image style %s: %d %v", name, n, err)
			}
		}

		// The main menu with a Home link, placed in the header of the
		// default theme.
		menu, err := client.MenuDef.Query().Where(menudef.MachineName(mainMenu)).WithLinks().Only(ctx)
		if err != nil || menu.Label != "Главное меню" || len(menu.Edges.Links) != 1 || menu.Edges.Links[0].LinkURL != "/" || menu.Edges.Links[0].Title != "Главная" {
			t.Errorf("main menu: %+v %v", menu, err)
		}
		if n, err := client.BlockPlacementDef.Query().Where(blockplacementdef.PlacementType("menu"), blockplacementdef.Region("header"),
			blockplacementdef.Theme(entity.ThemeKey(set.Site.Theme))).Count(ctx); err != nil || n != 1 {
			t.Errorf("menu placements in the header: %d %v", n, err)
		}

		// Every module installed here: each is on, with its state recorded.
		for _, m := range entity.Modules {
			if mod, err := client.Module.Get(ctx, m.Name); err != nil || !mod.Enabled {
				t.Errorf("module %s on a new site: %+v %v", m.Name, mod, err)
			}
		}

		// Installed: the core's install step never runs again.
		steps, err := r.Plan(ctx, migrations.CoreSet)
		if err != nil || len(steps) != 0 {
			t.Errorf("plan after install: %+v %v", steps, err)
		}
	})
}

// TestInstallStandardModules - a new site with the standard modules
// (app install without -modules): they're installed and on, the others
// off with no tables; the demo content has no part of a module that
// isn't there. Enabling one later installs it with its labels in the
// site's language (moduleInstaller).
func TestInstallStandardModules(t *testing.T) {
	paths.SetPublicFiles(t.TempDir(), "/files")
	t.Cleanup(func() { paths.SetPublicFiles("", "") })
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		chosen, err := installModules("", nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range migrations.Sets() {
			if s.Name == migrations.CoreSet || chosen[s.Name] {
				if _, err := r.Install(ctx, s.Name); err != nil {
					t.Fatal(s.Name, err)
				}
			}
		}
		client := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		if _, err := createSite(ctx, r, client, "Example", "Admin", "admin@example.com", "ru", siteOptions{demoSet: defaultDemoSet}); err != nil {
			t.Fatal(err)
		}
		for _, m := range entity.Modules {
			mod, err := client.Module.Get(ctx, m.Name)
			if err != nil || mod.Enabled != chosen[m.Name] {
				t.Errorf("module %s: %+v %v, want enabled %v", m.Name, mod, err, chosen[m.Name])
			}
			installed, err := r.Installed(ctx, m.Name)
			if err != nil || installed != chosen[m.Name] {
				t.Errorf("module %s installed %v %v, want %v", m.Name, installed, err, chosen[m.Name])
			}
		}

		const galleries = "galleries"
		if chosen[galleries] {
			t.Fatalf("%s is a standard module: pick another with a block that names a region", galleries)
		}
		if _, err := galleriesClient(db).Gallery.Query().Count(ctx); err == nil {
			t.Errorf("%s isn't installed, but its table is there", galleries)
		}
		var logs strings.Builder
		install := moduleInstaller(r, func(context.Context) (string, error) { return "ru", nil }, slog.New(slog.NewTextHandler(&logs, nil)))
		for range 2 { // the second time it's installed: nothing to do
			if err := install(ctx, galleries); err != nil {
				t.Fatal(err)
			}
		}
		// Its block, in the site's theme (PlaceModuleBlocks), once.
		if n, err := client.BlockPlacementDef.Query().Where(blockplacementdef.PluginID("node_galleries"), blockplacementdef.Region("content_inline")).Count(ctx); err != nil || n != 1 {
			t.Errorf("the galleries' block: %d %v", n, err)
		}
		if logs.Len() > 0 {
			t.Errorf("installing %s logged: %s", galleries, logs.String())
		}
	})
}

// TestInstallDemoEveryLanguage - the demo content, the core's and every
// module's, installs in each language a site can be installed in: every
// language's texts name demo users, sections and options that exist.
func TestInstallDemoEveryLanguage(t *testing.T) {
	paths.SetPublicFiles(t.TempDir(), "/files")
	t.Cleanup(func() { paths.SetPublicFiles("", "") })
	for _, lang := range builtInLanguages() {
		t.Run(lang, func(t *testing.T) {
			testdb.Each(t, func(t *testing.T, db testdb.DB) {
				ctx := context.Background()
				r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
				for _, s := range migrations.Sets() {
					if _, err := r.Install(ctx, s.Name); err != nil {
						t.Fatal(s.Name, err)
					}
				}
				client := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
				if _, err := createSite(ctx, r, client, "Demo", "Admin", "admin@example.com", lang, siteOptions{demoSet: defaultDemoSet}); err != nil {
					t.Fatal(err)
				}
				// No demo comment is dated in the future.
				after := time.Now()
				future, err := client.Comment.Query().Where(comment.CreatedAtGT(after)).Count(ctx)
				if err != nil || future > 0 {
					t.Errorf("comments dated after the install: %d %v", future, err)
				}
			})
		})
	}
}

// galleriesClient - the galleries module's own client on the test
// database (its tables are its own schema's).
func galleriesClient(db testdb.DB) *galleriesent.Client {
	return galleriesent.NewClient(galleriesent.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
}

// TestInstallDemoSet - a site installed with a demo set (the tests',
// demoset/testdata): the set's users, terms, content type, content with
// its comments and a picture of the set's own (a node without its own comment mode has its type's),
// its text format (the content's texts in it),
// Views (a theme template too), blocks in the site's theme, menus (a new
// one too), slogan, front page, permissions and the modules' content (a
// form with its block and a question filled from the address) are there.
func TestInstallDemoSet(t *testing.T) {
	paths.SetPublicFiles(t.TempDir(), "/files")
	t.Cleanup(func() { paths.SetPublicFiles("", "") })
	defer demoset.UseSets(os.DirFS(filepath.Join("demoset", "testdata")))()
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, s := range migrations.Sets() {
			if _, err := r.Install(ctx, s.Name); err != nil {
				t.Fatal(s.Name, err)
			}
		}
		client := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		if _, err := createSite(ctx, r, client, "Demo", "Admin", "admin@example.com", "en", siteOptions{demoSet: "fixture"}); err != nil {
			t.Fatal(err)
		}
		count := func(query string, args ...any) int {
			t.Helper()
			var n int
			if err := db.DB.QueryRow(db.Dialect.Rebind(query), args...).Scan(&n); err != nil {
				t.Fatalf("%s: %v", query, err)
			}
			return n
		}
		for what, c := range map[string]struct {
			n     int
			query string
			args  []any
		}{
			"the set's users":        {2, `SELECT COUNT(*) FROM users WHERE username IN (?, ?)`, []any{"reader_ann", "writer_bob"}},
			"a term's description":   {1, `SELECT COUNT(*) FROM taxonomy_term_data WHERE name = ? AND description = ?`, []any{"Rivers", "Water that moves."}},
			"the content type":       {1, `SELECT COUNT(*) FROM content_types WHERE machine_name = ? AND label = ?`, []any{"item", "Item"}},
			"the content":            {3, `SELECT COUNT(*) FROM nodes WHERE title IN (?, ?, ?)`, []any{"A walled garden", "The river in autumn", "About these notes"}},
			"the sticky item":        {1, `SELECT COUNT(*) FROM nodes WHERE title = ? AND sticky = ?`, []any{"A walled garden", true}},
			"a comment and a reply":  {2, `SELECT COUNT(*) FROM comments c JOIN nodes n ON n.id = c.nid WHERE n.title = ?`, []any{"A walled garden"}},
			"the reply":              {1, `SELECT COUNT(*) FROM comments c JOIN nodes n ON n.id = c.nid WHERE n.title = ? AND c.parent_id IS NOT NULL AND c.parent_id > 0`, []any{"A walled garden"}},
			"the Views":              {2, `SELECT COUNT(*) FROM view_displays WHERE machine_name IN (?, ?)`, []any{"latest", "items"}},
			"the blocks":             {3, `SELECT COUNT(*) FROM block_placements WHERE theme = ? AND region = ? AND title IN (?, ?, ?)`, []any{"kosatka", "sidebar", "Latest notes", "Tags", "Hello"}},
			"the new menu":           {1, `SELECT COUNT(*) FROM menus WHERE machine_name = ? AND label = ?`, []any{"footer", "Footer links"}},
			"a link to a View":       {1, `SELECT COUNT(*) FROM menu_links WHERE title = ? AND link_view_id IS NOT NULL`, []any{"All items"}},
			"a link to content":      {1, `SELECT COUNT(*) FROM menu_links WHERE title = ? AND link_node_id IS NOT NULL`, []any{"About"}},
			"the slogan":             {1, `SELECT COUNT(*) FROM site_information WHERE slogan = ?`, []any{"Notes from the field"}},
			"the type's comments":    {1, `SELECT COUNT(*) FROM nodes WHERE title = ? AND comment_mode = ?`, []any{"About these notes", "hidden"}},
			"a node's own":           {1, `SELECT COUNT(*) FROM nodes WHERE title = ? AND comment_mode = ?`, []any{"A walled garden", "open"}},
			"the theme template":     {1, `SELECT COUNT(*) FROM view_displays WHERE machine_name = ? AND theme_template = ?`, []any{"items", "cards"}},
			"the form's block":       {1, `SELECT COUNT(*) FROM block_placements WHERE region = ? AND plugin_id = ? AND title = ?`, []any{"sidebar", "webform", "Write to us"}},
			"a question's filling":   {1, `SELECT COUNT(*) FROM webform_elements WHERE machine_name = ? AND prefill_param = ?`, []any{"topic", "topic"}},
			"the set's text format":  {1, `SELECT COUNT(*) FROM text_formats WHERE machine_name = ? AND weight = ? AND allowed_tags = ? AND allow_class_attribute = ? AND has_rich_editor = ? AND tinymce_toolbar = ?`, []any{"notes_html", -1, "p a ul ol li h2 pre code", true, true, "bold italic | bullist numlist | link | codesample"}},
			"a text in it":           {1, `SELECT COUNT(*) FROM field_value_text v JOIN text_formats f ON f.id = v.text_format_id JOIN nodes n ON n.id = v.node_id WHERE n.title = ? AND f.machine_name = ? AND v.value LIKE ?`, []any{"A walled garden", "notes_html", "%<pre><code class=\"language-text\">%"}},
			"the set's own picture":  {1, `SELECT COUNT(*) FROM files WHERE uri LIKE ? AND mime_type = ? AND width = ? AND height = ? AND alt = ?`, []any{"public://%/demo/river-cover-0.png", "image/png", 8, 6, "The river at dusk"}},
			"the set's own document": {1, `SELECT COUNT(*) FROM files WHERE uri LIKE ? AND mime_type = ? AND original_filename = ? AND size_bytes > ?`, []any{"public://%/demo/river-notes-0.pdf", "application/pdf", "Field notes.pdf", 0}},
			"a gallery's pictures":   {2, `SELECT COUNT(*) FROM gallery_items gi JOIN galleries g ON g.id = gi.gallery_id WHERE g.title = ?`, []any{"Photos"}},
			"its own one":            {1, `SELECT COUNT(*) FROM files WHERE uri LIKE ? AND mime_type = ? AND alt = ?`, []any{"public://galleries/%/demo-2.png", "image/png", "The river"}},
		} {
			if got := count(c.query, c.args...); got != c.n {
				t.Errorf("%s: %d, want %d", what, got, c.n)
			}
		}
		u, err := client.Role.Query().Where(role.Name(entity.RoleUser)).Only(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if u.Permissions["create item content"] != true || u.Permissions["edit own item content"] != true {
			t.Errorf("the user role's permissions: %v", u.Permissions)
		}
	})
}
