// Package demo - the demo content of "app install -demo" beyond the core's
// (internal/app): the steps the modules register (Register, like
// migrations.RegisterInstall) and what every demo step shares - its texts
// by language, the sea pictures, image files, nodes, comments, block
// placements and menu links. Everything a step writes is ordinary content
// and configuration, edited and deleted in the admin.
package demo

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // the decoders StoredImage reads a picture's size with
	_ "image/png"
	"io/fs"
	"path"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "golang.org/x/image/webp" // a WebP picture too

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/role"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// Env - what a demo step works with. Client is the install transaction's:
// a step writes its rows with it (a module's repositories open their own
// transaction, which a transaction's client can't). The core's demo comes
// first, so its users, articles and menus are there.
type Env struct {
	Client    *generated.Client
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
	// Dialect - the database's (dialect.MySQL, dialect.Postgres), for a
	// module's own ent client on Client (demokit.Driver).
	Dialect string
	// Data - a demo set's content for the step (JSON in the step's own
	// format), nil for the blog's: the step reads it instead of its
	// demo/<lang>.json (demokit.Content).
	Data []byte
	// Nodes - a demo set's nodes by key, the ones its Data may name.
	Nodes map[string]int
	// Files - a demo set's directory (a theme's demo/): the pictures of
	// its own its Data names ({file: …}); nil - none.
	Files fs.FS
}

// Step - a set's demo step.
type Step func(ctx context.Context, env Env) error

var steps = map[string]Step{}

// Register sets a set's demo step (a module's, from its RegisterSteps).
func Register(set string, run Step) {
	steps[set] = run
}

// Has - set has a demo step.
func Has(set string) bool {
	_, ok := steps[set]
	return ok
}

// Run runs the demo step of set, if it has one.
func Run(ctx context.Context, set string, env Env) error {
	if run, ok := steps[set]; ok {
		return run(ctx, env)
	}
	return nil
}

// Load reads demo/<lang>.json of fsys into v, demo/en.json for a language
// without its own.
func Load(fsys fs.FS, lang string, v any) error {
	name := "demo/" + lang + ".json"
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		name = "demo/en.json"
		data, err = fs.ReadFile(fsys, name)
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// Content reads a step's content into v: the demo set's (env.Data), else
// demo/<lang>.json of fsys (Load). A key v hasn't in the set's content is
// an error: a typo would be lost silently.
func Content(env Env, fsys fs.FS, v any) error {
	if env.Data == nil {
		return Load(fsys, env.Lang, v)
	}
	d := json.NewDecoder(bytes.NewReader(env.Data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("the demo set's content: %w", err)
	}
	return nil
}

// Node - the ID of a demo set's node by its key.
func (e Env) Node(key string) (int, error) {
	id, ok := e.Nodes[key]
	if !ok {
		return 0, fmt.Errorf("unknown demo node %q", key)
	}
	return id, nil
}

// User - the ID of the demo user name.
func (e Env) User(name string) (int, error) {
	id, ok := e.Users[name]
	if !ok {
		return 0, fmt.Errorf("unknown demo user %q", name)
	}
	return id, nil
}

// WriteImage writes the i-th demo picture (Picture) of w×h to rel in the
// public files' store, returning its public:// URI.
func WriteImage(ctx context.Context, rel string, w, h, i int) (string, error) {
	data, err := Picture(w, h, i)
	if err != nil {
		return "", err
	}
	if _, err := paths.PublicStore().Put(ctx, rel, bytes.NewReader(data), filestore.PutOptions{ContentType: "image/jpeg"}); err != nil {
		return "", err
	}
	return entity.PublicScheme + rel, nil
}

// ImageFile writes the i-th demo picture of w×h to rel (WriteImage) and
// adds its file row, returning its ID.
func ImageFile(ctx context.Context, env Env, rel string, w, h, i int, alt string, at time.Time) (int, error) {
	uri, err := WriteImage(ctx, rel, w, h, i)
	if err != nil {
		return 0, err
	}
	info, err := paths.PublicStore().Stat(ctx, rel)
	if err != nil {
		return 0, err
	}
	return imageRow(ctx, env, uri, "image/jpeg", w, h, int(info.Size), alt, at)
}

// StoredImage puts the picture data (a JPEG, a PNG or a WebP: a demo
// set's own) at rel of the public files and adds its file row, returning
// its ID.
func StoredImage(ctx context.Context, env Env, rel string, data []byte, alt string, at time.Time) (int, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("reading the picture %s: %w", rel, err)
	}
	mime := "image/" + format
	if _, err := paths.PublicStore().Put(ctx, rel, bytes.NewReader(data), filestore.PutOptions{ContentType: mime}); err != nil {
		return 0, err
	}
	return imageRow(ctx, env, entity.PublicScheme+rel, mime, cfg.Width, cfg.Height, len(data), alt, at)
}

// imageRow adds the file row of an image written to uri.
func imageRow(ctx context.Context, env Env, uri, mime string, w, h, size int, alt string, at time.Time) (int, error) {
	f, err := env.Client.File.Create().
		SetURI(uri).
		SetMimeType(mime).
		SetWidth(w).
		SetHeight(h).
		SetSizeBytes(size).
		SetAlt(alt).
		SetCreatedBy(env.CreatedBy).
		SetCreatedAt(at).
		SetUpdatedAt(at).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return f.ID, nil
}

// Node - a published node to create: its type, title (the slug is
// <type>/<title transliterated>), author, comment mode (open when empty)
// and time.
type Node struct {
	Type     string
	Title    string
	Author   int
	Comments entity.CommentMode
	At       time.Time
	// Views - the node's view count (a node_counter row when above 0).
	Views int
}

// CreateNode adds the node, returning its ID. The type's own rows (a
// body, a poll, a form) are the step's.
func CreateNode(ctx context.Context, env Env, n Node) (int, error) {
	created, err := env.Client.Node.Create().
		SetType(n.Type).
		SetTitle(n.Title).
		SetSlug(n.Type + "/" + slug.Generate(n.Title)).
		SetStatus(1).
		SetOnModeration(0).
		SetCommentMode(string(n.Comments.OrDefault())).
		SetCreatedBy(n.Author).
		SetCreatedAt(n.At).
		SetUpdatedAt(n.At).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("node %q: %w", n.Title, err)
	}
	if n.Views > 0 {
		if err := env.Client.NodeCounter.Create().SetNid(created.ID).SetTotalcount(n.Views).SetDaycount(n.Views / 25).SetLastViewed(env.Now).Exec(ctx); err != nil {
			return 0, err
		}
	}
	return created.ID, nil
}

// Comment - a comment by one of the demo users, with its replies.
type Comment struct {
	Author  string    `json:"author"`
	Body    string    `json:"body"`
	Replies []Comment `json:"replies"`
}

// commentStep - the time between a node's comments.
const commentStep = 47 * time.Minute

// CreateComments adds a node's comments and their replies (through the
// comment repository, which sets the thread path), the first one
// commentStep after at and each a commentStep after the previous one.
// Returns how many it added and when the last one was posted.
func CreateComments(ctx context.Context, env Env, nodeID int, list []Comment, at time.Time) (int, time.Time, error) {
	repo := database.NewCommentRepository(env.Client)
	count := 0
	var add func(parent *int, list []Comment) error
	add = func(parent *int, list []Comment) error {
		for _, dc := range list {
			author, err := env.User(dc.Author)
			if err != nil {
				return err
			}
			created, err := repo.Create(ctx, &entity.Comment{Nid: nodeID, ParentID: parent, CreatedBy: author, Content: "<p>" + dc.Body + "</p>", Status: 1})
			if err != nil {
				return err
			}
			at = at.Add(commentStep)
			count++
			if err := env.Client.Comment.UpdateOneID(created.ID).SetCreatedAt(at).SetUpdatedAt(at).Exec(ctx); err != nil {
				return err
			}
			id := created.ID
			if err := add(&id, dc.Replies); err != nil {
				return err
			}
		}
		return nil
	}
	err := add(nil, list)
	return count, at, err
}

// Placement - a block placement in the site's theme: its region, title
// (shown or not), weight and on which pages it shows (every page when
// empty; entity.BlockVisibilityHomepage - the front page only).
type Placement struct {
	Region     string
	Title      string
	ShowTitle  bool
	Weight     int
	Visibility entity.BlockVisibilityScope
}

// PlacePlugin places the plugin block with its settings (PluginConfig,
// e.g. entity.PluginBlockTargetKey) in the site's theme.
func PlacePlugin(ctx context.Context, env Env, plugin entity.PluginBlockID, config map[string]string, p Placement) error {
	q := env.Client.BlockPlacementDef.Create().
		SetPlacementType(string(entity.BlockPlacementPlugin)).
		SetPluginID(string(plugin))
	if len(config) > 0 {
		q = q.SetPluginConfig(config)
	}
	return Place(ctx, env, q, p)
}

// Place saves a block placement (the caller sets what is placed) in the
// site's theme.
func Place(ctx context.Context, env Env, q *generated.BlockPlacementDefCreate, p Placement) error {
	if p.Visibility != "" {
		q = q.SetVisibilityScope(string(p.Visibility))
	}
	return q.SetTheme(env.Theme).SetRegion(p.Region).
		SetTitle(p.Title).SetShowTitle(p.ShowTitle).SetWeight(p.Weight).
		SetCreatedAt(env.Now).SetUpdatedAt(env.Now).Exec(ctx)
}

// ViewBlock - a block View of content a demo step adds: its machine name,
// label, the items it shows (PageSize) and its fields, filters and sorts
// in their order, each a built-in or a module's ("<module>.<key>", its
// parameters in Params).
type ViewBlock struct {
	Name     string
	Label    string
	PageSize int
	Fields   []ViewBlockField
	Filters  []ViewBlockFilter
	Sorts    []ViewBlockSort
}

// ViewBlockField - a ViewBlock's field: LinkTo "content" links it to the
// item.
type ViewBlockField struct {
	Builtin   string
	Label     string
	ShowLabel bool
	LinkTo    string
	Params    map[string]string
}

// ViewBlockFilter - a ViewBlock's filter (entity.ViewFilterDef).
type ViewBlockFilter struct {
	Builtin  string
	Operator string
	Value    string
}

// ViewBlockSort - a ViewBlock's sort (entity.ViewSortDef).
type ViewBlockSort struct {
	Builtin   string
	Direction string
	Params    map[string]string
}

// PlaceView creates the block View v (a list, no pager, enabled) and
// places it in the site's theme (Placement).
func PlaceView(ctx context.Context, env Env, v ViewBlock, p Placement) error {
	display, err := env.Client.ViewDisplayDef.Create().
		SetMachineName(v.Name).
		SetLabel(v.Label).
		SetDisplayType(string(entity.ViewDisplayTypeBlock)).
		SetBaseEntity("content").
		SetDisplayStyle(string(entity.ViewDisplayStyleList)).
		SetPageSize(v.PageSize).
		SetShowPagination(false).
		SetStatus(1).
		SetCreatedBy(env.CreatedBy).
		SetCreatedAt(env.Now).
		SetUpdatedAt(env.Now).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("the %s View: %w", v.Name, err)
	}
	for i, f := range v.Fields {
		q := env.Client.ViewFieldDef.Create().SetViewDisplayID(display.ID).SetBuiltinField(f.Builtin).
			SetLabel(f.Label).SetShowLabel(f.ShowLabel).SetLinkTo(f.LinkTo).SetSortOrder(i * 10).
			SetCreatedAt(env.Now).SetUpdatedAt(env.Now)
		if len(f.Params) > 0 {
			q = q.SetParams(f.Params)
		}
		if err := q.Exec(ctx); err != nil {
			return fmt.Errorf("the %s View's field %s: %w", v.Name, f.Builtin, err)
		}
	}
	for i, f := range v.Filters {
		if err := env.Client.ViewFilterDef.Create().SetViewDisplayID(display.ID).SetBuiltinField(f.Builtin).
			SetOperator(f.Operator).SetValue(f.Value).SetSortOrder(i * 10).
			SetCreatedAt(env.Now).SetUpdatedAt(env.Now).Exec(ctx); err != nil {
			return fmt.Errorf("the %s View's filter %s: %w", v.Name, f.Builtin, err)
		}
	}
	for i, st := range v.Sorts {
		q := env.Client.ViewSortDef.Create().SetViewDisplayID(display.ID).SetBuiltinField(st.Builtin).
			SetDirection(st.Direction).SetSortOrder(i * 10).SetCreatedAt(env.Now).SetUpdatedAt(env.Now)
		if len(st.Params) > 0 {
			q = q.SetParams(st.Params)
		}
		if err := q.Exec(ctx); err != nil {
			return fmt.Errorf("the %s View's sort %s: %w", v.Name, st.Builtin, err)
		}
	}
	return Place(ctx, env, env.Client.BlockPlacementDef.Create().
		SetPlacementType(string(entity.BlockPlacementView)).SetViewDisplayID(display.ID), p)
}

// AvatarSize - the side of the demo users' avatars (Picture).
const AvatarSize = 256

// CreateUsers adds the users who comment: ordinary members with a
// generated avatar and a random password (they're for show, nobody logs
// in as them). Returns their IDs by name.
func CreateUsers(ctx context.Context, c *generated.Client, names []string, now time.Time) (map[string]int, error) {
	userRole, err := c.Role.Query().Where(role.Name(entity.RoleUser)).OnlyID(ctx)
	if err != nil {
		return nil, fmt.Errorf("the %s role: %w", entity.RoleUser, err)
	}
	ids := make(map[string]int, len(names))
	for i, name := range names {
		secret := make([]byte, 18)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(secret)), bcrypt.MinCost)
		if err != nil {
			return nil, err
		}
		avatar, err := WriteImage(ctx, path.Join(files.UserAvatarBasePath, "demo-"+slug.Generate(name)+".jpg"), AvatarSize, AvatarSize, i+3)
		if err != nil {
			return nil, err
		}
		u, err := c.User.Create().
			SetUsername(name).
			SetSlug(entity.UserSlug(name)).
			SetEmail(entity.NormalizeEmail(name + "@example.com")).
			SetPasswordHash(string(hash)).
			SetStatus(1).
			SetEmailVerified(true).
			SetRoleID(userRole).
			SetAvatar(avatar).
			SetCreatedAt(now.Add(-time.Duration(len(names)-i) * 24 * time.Hour)).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		ids[name] = u.ID
	}
	return ids, nil
}

// Vocabulary - a vocabulary to create: its name, the slug its term pages
// go under and its terms (names; a name twice - one term).
type Vocabulary struct {
	Name  string
	Slug  string
	Terms []string
	// Descriptions - the terms' descriptions by name (none - empty).
	Descriptions map[string]string
}

// VocabularyRows - a created vocabulary and its terms' IDs by name.
type VocabularyRows struct {
	ID    int
	Terms map[string]int
}

// TermIDs - the IDs of the terms named, those it has (in order).
func (v VocabularyRows) TermIDs(names []string) []int {
	ids := make([]int, 0, len(names))
	for _, n := range names {
		if id, ok := v.Terms[n]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// CreateVocabulary adds a vocabulary with public term pages under its
// slug and its terms, each with the URL <slug>/<term name transliterated>.
func CreateVocabulary(ctx context.Context, c *generated.Client, v Vocabulary, createdBy int, now time.Time) (VocabularyRows, error) {
	voc, err := c.TaxonomyVocabulary.Create().
		SetName(v.Name).
		SetSlug(v.Slug).
		SetTermPathPrefix(v.Slug).
		SetPublicTermPages(true).
		SetCreatedBy(createdBy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return VocabularyRows{}, err
	}
	rows := VocabularyRows{ID: voc.ID, Terms: map[string]int{}}
	for i, name := range v.Terms {
		if _, ok := rows.Terms[name]; ok {
			continue
		}
		q := c.TaxonomyTermData.Create().
			SetVid(voc.ID).
			SetName(name).
			SetSlug(v.Slug + "/" + slug.Generate(name)).
			SetWeight(i).
			SetCreatedBy(createdBy).
			SetCreatedAt(now).
			SetUpdatedAt(now)
		if d := v.Descriptions[name]; d != "" {
			q = q.SetDescription(d)
		}
		t, err := q.Save(ctx)
		if err != nil {
			return VocabularyRows{}, fmt.Errorf("term %q: %w", name, err)
		}
		rows.Terms[name] = t.ID
	}
	return rows, nil
}

// MenuLink adds a link to a menu (Env.MainMenu, Env.FooterMenu): to url,
// or to the node when nodeID isn't 0. No menu (a demo set without one,
// menuID 0) - no link.
func MenuLink(ctx context.Context, env Env, menuID int, title, url string, nodeID, weight int) error {
	if menuID == 0 {
		return nil
	}
	q := env.Client.MenuLinkDef.Create().
		SetMenuID(menuID).
		SetTitle(title).
		SetWeight(weight).
		SetCreatedAt(env.Now).
		SetUpdatedAt(env.Now)
	if nodeID != 0 {
		q = q.SetLinkType(string(entity.MenuLinkNode)).SetLinkNodeID(nodeID)
	} else {
		q = q.SetLinkType(string(entity.MenuLinkURL)).SetLinkURL(url)
	}
	return q.Exec(ctx)
}
