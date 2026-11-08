// Package demokit - the part of the module SDK a module's demo step uses
// (app install -demo, docs/modules/write-a-module.md): the
// step's registration, its texts by language, a block and a menu link of
// the core's, and the install transaction for the module's own ent
// client. Light like manifestkit: a manifest registers the step.
package demokit

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"entgo.io/ent/dialect"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/demo"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// Env - what a demo step works with: the site's language, the
// administrator, the demo users and articles, the theme, the menus; the
// core's rows it adds through this package's helpers (in the install's
// transaction).
type Env struct {
	Lang      string
	CreatedBy int // the administrator
	// Users - the demo users by name, the authors of comments and votes.
	Users map[string]int
	// Articles - the demo articles' node IDs, the newest first.
	Articles []int
	// Theme - the block placement key of the site's theme.
	Theme string
	// MainMenu and FooterMenu - the IDs of the menus in the header and
	// the footer.
	MainMenu, FooterMenu int
	Now                  time.Time

	// core - the core's step env (the install transaction's client, a
	// demo set's content).
	core demo.Env
}

// envOf - the core's demo env as a module's.
func envOf(e demo.Env) Env {
	return Env{Lang: e.Lang, CreatedBy: e.CreatedBy, Users: e.Users, Articles: e.Articles, Theme: e.Theme,
		MainMenu: e.MainMenu, FooterMenu: e.FooterMenu, Now: e.Now, core: e}
}

// Node - the ID of a demo set's node by its key (the keys its content
// names).
func (env Env) Node(key string) (int, error) { return env.core.Node(key) }

// User - the ID of the demo user name.
func (env Env) User(name string) (int, error) { return env.core.User(name) }

// Step - a module's demo step.
type Step func(ctx context.Context, env Env) error

type (
	// Placement - where PlacePlugin and PlaceView put a block.
	Placement = demo.Placement
	// ViewBlock - a block View of content (PlaceView): its fields,
	// filters and sorts, built-in or a module's ("<module>.<key>").
	ViewBlock = demo.ViewBlock
	// ViewBlockField - a ViewBlock's field.
	ViewBlockField = demo.ViewBlockField
	// ViewBlockFilter - a ViewBlock's filter.
	ViewBlockFilter = demo.ViewBlockFilter
	// ViewBlockSort - a ViewBlock's sort.
	ViewBlockSort = demo.ViewBlockSort
	// Node - a demo material (CreateNode).
	Node = demo.Node
	// Comment - a demo comment and its replies (CreateComments).
	Comment = demo.Comment
)

// CreateNode creates a demo material of the module's content type.
func CreateNode(ctx context.Context, env Env, n Node) (int, error) {
	return demo.CreateNode(ctx, env.core, n)
}

// ImageFile draws a demo picture (palette i, w×h) at rel under the public
// files directory and stores its file row: its ID.
func ImageFile(ctx context.Context, env Env, rel string, w, h, i int, alt string, at time.Time) (int, error) {
	return demo.ImageFile(ctx, env.core, rel, w, h, i, alt, at)
}

// SetPicture stores a picture of the demo set's own - file, a path in
// its directory (a theme's demo/), as the set's content names it
// ({file: …}) - at rel under the public files directory with its file
// row: its ID. A JPEG, a PNG or a WebP.
func SetPicture(ctx context.Context, env Env, rel, file, alt string, at time.Time) (int, error) {
	if env.core.Files == nil {
		return 0, fmt.Errorf("picture %q: the demo set has no directory of its own", file)
	}
	data, err := fs.ReadFile(env.core.Files, file)
	if err != nil {
		return 0, fmt.Errorf("picture %q: %w", file, err)
	}
	return demo.StoredImage(ctx, env.core, rel, data, alt, at)
}

// Vocabulary creates a demo vocabulary (its terms' addresses under
// prefix): its ID.
func Vocabulary(ctx context.Context, env Env, name, slug, prefix string) (int, error) {
	v, err := env.core.Client.TaxonomyVocabulary.Create().
		SetName(name).SetSlug(slug).SetTermPathPrefix(prefix).
		SetCreatedBy(env.CreatedBy).SetCreatedAt(env.Now).SetUpdatedAt(env.Now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("vocabulary %q: %w", name, err)
	}
	return v.ID, nil
}

// Term creates a demo term of vocabulary vid (its address path; parent -
// nil for a top one): its ID.
func Term(ctx context.Context, env Env, vid int, name, description, path string, parent *int, weight int) (int, error) {
	t, err := env.core.Client.TaxonomyTermData.Create().
		SetVid(vid).SetName(name).SetDescription(description).SetSlug(path).
		SetNillableParentID(parent).SetWeight(weight).
		SetCreatedBy(env.CreatedBy).SetCreatedAt(env.Now).SetUpdatedAt(env.Now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("term %q: %w", name, err)
	}
	return t.ID, nil
}

// IndexTerm files node nodeID under term tid (the terms index), at.
func IndexTerm(ctx context.Context, env Env, nodeID, tid int, at time.Time) error {
	return env.core.Client.TaxonomyIndex.Create().SetNid(nodeID).SetTid(tid).SetCreatedAt(at).Exec(ctx)
}

// CreateComments adds the comments (and their replies) to node nodeID,
// from at on; it returns how many and the last one's time.
func CreateComments(ctx context.Context, env Env, nodeID int, list []Comment, at time.Time) (int, time.Time, error) {
	return demo.CreateComments(ctx, env.core, nodeID, list, at)
}

// Register sets the module's demo step (its manifest's RegisterSteps).
func Register(set string, run Step) {
	demo.Register(set, func(ctx context.Context, e demo.Env) error { return run(ctx, envOf(e)) })
}

// Load reads demo/<lang>.json of fsys into v (demo/en.json for a language
// without its own).
func Load(fsys fs.FS, lang string, v any) error { return demo.Load(fsys, lang, v) }

// Content reads the step's content into v: a demo set's for the module
// (app install -demo-set), else demo/<lang>.json of fsys (Load).
// A set's content has the format of the module's demo/<lang>.json.
func Content(env Env, fsys fs.FS, v any) error { return demo.Content(env.core, fsys, v) }

// PlacePlugin places the plugin block (the module's) with its settings.
func PlacePlugin(ctx context.Context, env Env, plugin manifestkit.PluginBlockID, config map[string]string, p Placement) error {
	return demo.PlacePlugin(ctx, env.core, plugin, config, p)
}

// PlaceView creates the block View v (a list of content: say, the
// module's Views field and sort for a top list) and places it.
func PlaceView(ctx context.Context, env Env, v ViewBlock, p Placement) error {
	return demo.PlaceView(ctx, env.core, v, p)
}

// MenuLink adds a link to the menu (Env.MainMenu, Env.FooterMenu): to url,
// or to the node nodeID when url is "". Menu 0 (a demo set names none) -
// no link.
func MenuLink(ctx context.Context, env Env, menuID int, title, url string, nodeID, weight int) error {
	return demo.MenuLink(ctx, env.core, menuID, title, url, nodeID, weight)
}

// Driver - the install transaction as an ent driver, for the module's own
// client (its generated NewClient(Driver(…))): its rows go in with the
// core's, seeing them, and nothing stays if the install fails.
func Driver(env Env) dialect.Driver {
	return database.InTxDriver(env.core.Dialect, env.core.Client)
}
