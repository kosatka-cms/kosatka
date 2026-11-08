package app

import (
	"context"
	"embed"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"time"

	"github.com/kosatka-cms/kosatka/internal/app/demoset"
	"github.com/kosatka-cms/kosatka/internal/app/starter"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/demo"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/contenttypedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/menudef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/textformatdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewdisplaydef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// defaultDemoSet - the demo set of "install -demo": the blog about the
// engine (createDemoContent, demo/<lang>.json); the other sets are data
// (demoset).
const defaultDemoSet = "blog"

// demoSetNames - the demo sets: the blog, then demoset's.
func demoSetNames() []string {
	return append([]string{defaultDemoSet}, demoset.Names()...)
}

// demoSetModules - the modules the demo set name in lang has content
// for (none for the blog: its modules' content comes with them) and,
// for a theme's own set, the ones this build hasn't (skipped: a module
// of its own repository the site isn't built with); an error when there
// is no such set.
func demoSetModules(name, lang string) (modules, skipped []string, err error) {
	if name == defaultDemoSet {
		return nil, nil, nil
	}
	set, err := demoset.Load(name, lang)
	if err != nil {
		return nil, nil, err
	}
	modules = slices.Sorted(maps.Keys(set.Modules))
	if !set.FromTheme {
		return modules, nil, nil
	}
	skipped = set.Absent(func(name string) bool {
		_, ok := entity.ModuleByName(name)
		return ok
	})
	return slices.DeleteFunc(modules, func(m string) bool { return slices.Contains(skipped, m) }), skipped, nil
}

// demoFS - the demo content ("install -demo") by language: demo/<lang>.json.
//
//go:embed demo/*.json
var demoFS embed.FS

// demoContent - what demo/<lang>.json holds: two vocabularies (sections
// and tags, with Article fields for them), users who comment, the sidebar
// blocks' titles, articles (with a generated cover each, the first one
// newest; a section, tags, a view count and threaded comments), a page
// linked from the main menu and the name of the footer menu (the modules'
// demo links go there).
type demoContent struct {
	Vocabularies struct {
		Sections demoVocabulary `json:"sections"`
		Tags     demoVocabulary `json:"tags"`
	} `json:"vocabularies"`
	Users      []string      `json:"users"`
	Blocks     demoBlocks    `json:"blocks"`
	Frontpage  demoFrontpage `json:"frontpage"`
	Articles   []demoArticle `json:"articles"`
	Page       demoNode      `json:"page"`
	FooterMenu string        `json:"footer_menu"`
}

// demoVocabulary - a vocabulary, the label of the Article field using it
// and its terms (the tags' come from the articles).
type demoVocabulary struct {
	Name  string   `json:"name"`
	Slug  string   `json:"slug"`
	Field string   `json:"field"`
	Terms []string `json:"terms"`
}

// vocabulary - v with the terms names, to create.
func (v demoVocabulary) vocabulary(names []string) demo.Vocabulary {
	return demo.Vocabulary{Name: v.Name, Slug: v.Slug, Terms: names}
}

// demoFrontpage - the label of the front page cards' author.
type demoFrontpage struct {
	Author string `json:"author"`
}

// demoBlocks - the titles of the sidebar blocks.
type demoBlocks struct {
	Popular  string `json:"popular"`
	Comments string `json:"comments"`
	Search   string `json:"search"`
	Tags     string `json:"tags"`
}

type demoNode struct {
	Title string `json:"title"`
	Body  string `json:"body"` // Basic HTML
}

type demoArticle struct {
	demoNode
	Section  string         `json:"section"`
	Tags     []string       `json:"tags"`
	Views    int            `json:"views"`
	Comments []demo.Comment `json:"comments"`
	// Sticky - at the top of lists (the front page leads with it).
	Sticky bool `json:"sticky"`
}

// demoCoverDir - where the covers go, under the public files directory
// (the directory of uploaded content images).
var demoCoverDir = path.Join(files.DynamicContentImageBasePath, "demo")

// The names in the starter set (starter/starter.yaml) the demo content
// fills: its content types, their shared fields, the front page View and
// the main menu.
const (
	articleType   = "article"
	pageType      = "page"
	bodyStorage   = "field_body"
	imageStorage  = "field_image"
	frontpageView = "frontpage"
	mainMenu      = "main"
)

// starterFields - the starter set's rows the demo content builds on: the
// site's theme and the shared fields.
type starterFields struct {
	theme       string
	body, image int
}

// demoStarter - starterFields of the set Apply created.
func demoStarter(set *starter.Set, created starter.Result) starterFields {
	return starterFields{theme: set.Site.Theme, body: created.Storages[bodyStorage], image: created.Storages[imageStorage]}
}

// demoFields - the Article fields the demo content fills.
type demoFields struct {
	starterFields
	section, tags int
}

// demoArticleStep - the time between two demo articles: the first is the
// newest, the rest go back from it.
const demoArticleStep = 19 * time.Hour

// createDemoContent adds the demo content: users, the section and tag
// vocabularies with their Article fields, the articles with their covers
// (written to the public files directory), terms, view counts and
// comments, the front page showing them, the sidebar blocks (search,
// popular articles, latest comments), the sections and the page in the
// main menu, a footer menu; then the demo steps of the installed modules
// (demo.Register). Everything is ordinary content and configuration,
// edited and deleted in the admin.
func createDemoContent(ctx context.Context, c *generated.Client, createdBy int, base starterFields, lang string, installed map[string]bool, dialect string) error {
	var content demoContent
	if err := demo.Load(demoFS, lang, &content); err != nil {
		return err
	}
	basicHTML, err := c.TextFormatDef.Query().Where(textformatdef.MachineName(entity.TextFormatBasicHTML)).OnlyID(ctx)
	if err != nil {
		return fmt.Errorf("the basic_html text format: %w", err)
	}
	now := time.Now()
	users, err := demo.CreateUsers(ctx, c, content.Users, now)
	if err != nil {
		return fmt.Errorf("users: %w", err)
	}
	env := demo.Env{Client: c, Lang: lang, CreatedBy: createdBy, Users: users, Theme: entity.ThemeKey(base.theme), Now: now, Dialect: dialect}
	fields := demoFields{starterFields: base}
	sections, err := demo.CreateVocabulary(ctx, c, content.Vocabularies.Sections.vocabulary(content.Vocabularies.Sections.Terms), createdBy, now)
	if err != nil {
		return fmt.Errorf("sections: %w", err)
	}
	var tagNames []string
	for _, a := range content.Articles {
		tagNames = append(tagNames, a.Tags...)
	}
	tags, err := demo.CreateVocabulary(ctx, c, content.Vocabularies.Tags.vocabulary(tagNames), createdBy, now)
	if err != nil {
		return fmt.Errorf("tags: %w", err)
	}
	if fields.section, fields.tags, err = createDemoTermFields(ctx, c, content, sections.ID, tags.ID, now); err != nil {
		return fmt.Errorf("term fields: %w", err)
	}
	for i, a := range content.Articles {
		at := now.Add(-time.Duration(i) * demoArticleStep)
		nodeID, err := createDemoNode(ctx, c, articleType, a.demoNode, entity.CommentModeOpen, true, createdBy, at, fields.body, basicHTML)
		if err != nil {
			return fmt.Errorf("article %q: %w", a.Title, err)
		}
		if a.Sticky {
			if err := c.Node.UpdateOneID(nodeID).SetSticky(true).Exec(ctx); err != nil {
				return fmt.Errorf("article %q: %w", a.Title, err)
			}
		}
		env.Articles = append(env.Articles, nodeID)
		fileID, err := createDemoCover(ctx, env, i, a.Title, at)
		if err != nil {
			return fmt.Errorf("cover of %q: %w", a.Title, err)
		}
		if err := c.FieldValueFile.Create().SetNodeID(nodeID).SetFieldID(fields.image).SetFileID(fileID).Exec(ctx); err != nil {
			return err
		}
		if err := setDemoTerms(ctx, c, nodeID, fields, sections.Terms[a.Section], tags.TermIDs(a.Tags)); err != nil {
			return fmt.Errorf("terms of %q: %w", a.Title, err)
		}
		if err := c.NodeCounter.Create().SetNid(nodeID).SetTotalcount(a.Views).SetDaycount(a.Views / 25).SetLastViewed(now).Exec(ctx); err != nil {
			return err
		}
		if _, _, err := demo.CreateComments(ctx, env, nodeID, a.Comments, at); err != nil {
			return fmt.Errorf("comments of %q: %w", a.Title, err)
		}
	}
	if err := c.ContentTypeDef.Update().Where(contenttypedef.MachineName(articleType)).SetShowViewCount(true).Exec(ctx); err != nil {
		return err
	}
	if err := extendDemoFrontpage(ctx, c, fields, content.Frontpage, now); err != nil {
		return fmt.Errorf("front page: %w", err)
	}
	if err := createDemoSidebar(ctx, c, base.theme, content.Blocks, tags.ID, createdBy, now); err != nil {
		return fmt.Errorf("sidebar: %w", err)
	}
	pageID, err := createDemoNode(ctx, c, pageType, content.Page, entity.CommentModeHidden, false, createdBy, now, fields.body, basicHTML)
	if err != nil {
		return fmt.Errorf("page %q: %w", content.Page.Title, err)
	}
	if env.MainMenu, err = createDemoMenuLinks(ctx, c, content, sections, pageID, now); err != nil {
		return fmt.Errorf("main menu: %w", err)
	}
	if env.FooterMenu, err = createDemoFooterMenu(ctx, c, base.theme, content.FooterMenu, now); err != nil {
		return fmt.Errorf("footer menu: %w", err)
	}
	for _, set := range migrations.Sets() {
		if set.Name != migrations.CoreSet && !installed[set.Name] {
			continue
		}
		if err := demo.Run(ctx, set.Name, env); err != nil {
			return fmt.Errorf("%s: %w", set.Name, err)
		}
	}
	return nil
}

// createDemoTermFields adds the Article's Section (one term, a select
// list) and Tags (several, autocomplete) fields, shown on the article's
// page after the body; returns their storage IDs.
func createDemoTermFields(ctx context.Context, c *generated.Client, demo demoContent, sectionsVID, tagsVID int, now time.Time) (int, int, error) {
	article, err := c.ContentTypeDef.Query().Where(contenttypedef.MachineName(articleType)).OnlyID(ctx)
	if err != nil {
		return 0, 0, err
	}
	var ids [2]int
	for i, f := range []struct {
		machine, label, widget string
		vid                    int
		multiple               bool
	}{
		{"field_section", demo.Vocabularies.Sections.Field, string(entity.WidgetSelect), sectionsVID, false},
		{"field_tags", demo.Vocabularies.Tags.Field, string(entity.WidgetAutocomplete), tagsVID, true},
	} {
		storage, err := c.FieldStorageDef.Create().
			SetMachineName(f.machine).
			SetFieldType(string(entity.FieldTypeTaxonomy)).
			SetConfig(map[string]any{"vocabulary_id": f.vid, "multiple": f.multiple}).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			return 0, 0, err
		}
		err = c.FieldInstanceDef.Create().
			SetContentTypeID(article).
			SetFieldStorageID(storage.ID).
			SetConfig(map[string]any{"widget": f.widget}).
			SetLabel(f.label).
			SetShowOnDetail(true).
			SetShowLabel(true).
			// In the form first: before the starter's image and body.
			SetSortOrder(10 + i*10).
			SetDisplayOrder(20 + i*10).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Exec(ctx)
		if err != nil {
			return 0, 0, err
		}
		ids[i] = storage.ID
	}
	return ids[0], ids[1], nil
}

// setDemoTerms stores an article's section and tags and its taxonomy index
// (term pages and Views read it).
func setDemoTerms(ctx context.Context, c *generated.Client, nodeID int, fields demoFields, section int, tags []int) error {
	var all []int
	if section != 0 {
		if err := c.FieldValueTaxonomy.Create().SetNodeID(nodeID).SetFieldID(fields.section).SetTermID(section).Exec(ctx); err != nil {
			return err
		}
		all = append(all, section)
	}
	for _, t := range tags {
		if err := c.FieldValueTaxonomyMulti.Create().SetNodeID(nodeID).SetFieldID(fields.tags).SetTermID(t).Exec(ctx); err != nil {
			return err
		}
		all = append(all, t)
	}
	return database.NewTaxonomyRepository(c).UpdateNodeTerms(ctx, nodeID, all)
}

// extendDemoFrontpage adds to the front page cards the section (a badge),
// the author (after the title), the tags (links), the view and the comment
// count (with their labels). The title and the picture link to the
// article: no "Read more" of its own.
func extendDemoFrontpage(ctx context.Context, c *generated.Client, fields demoFields, labels demoFrontpage, now time.Time) error {
	display, err := c.ViewDisplayDef.Query().Where(viewdisplaydef.MachineName(frontpageView)).OnlyID(ctx)
	if err != nil {
		return err
	}
	if err := c.ViewFieldDef.Create().SetViewDisplayID(display).SetLabel("").
		SetFieldStorageID(fields.section).SetTagFormat(string(entity.ViewTagFormatBadges)).SetShowLabel(false).SetSortOrder(5).
		SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
		return err
	}
	if err := c.ViewFieldDef.Create().SetViewDisplayID(display).SetLabel("").
		SetFieldStorageID(fields.tags).SetTagFormat(string(entity.ViewTagFormatLinked)).SetShowLabel(true).SetSortOrder(35).
		SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
		return err
	}
	if err := c.ViewFieldDef.Create().SetViewDisplayID(display).SetLabel(labels.Author).
		SetBuiltinField(string(entity.ViewFieldBuiltinAuthor)).SetShowLabel(true).SetSortOrder(15).
		SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
		return err
	}
	for i, b := range []entity.ViewFieldBuiltinField{entity.ViewFieldBuiltinViewCount, entity.ViewFieldBuiltinCommentCount} {
		if err := c.ViewFieldDef.Create().SetViewDisplayID(display).SetLabel("").
			SetBuiltinField(string(b)).SetShowLabel(true).SetSortOrder(40 + i*10).
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// demoPopularView - the machine name of the sidebar's popular articles
// View.
const demoPopularView = "popular"

// createDemoSidebar fills the sidebar: search, the most viewed articles
// (a View block), the latest comments and the tag cloud of the tags
// vocabulary (tagsVID).
func createDemoSidebar(ctx context.Context, c *generated.Client, theme string, titles demoBlocks, tagsVID, createdBy int, now time.Time) error {
	popular, err := c.ViewDisplayDef.Create().
		SetMachineName(demoPopularView).
		SetLabel(titles.Popular).
		SetDisplayType(string(entity.ViewDisplayTypeBlock)).
		SetBaseEntity("content").
		SetDisplayStyle(string(entity.ViewDisplayStyleList)).
		SetPageSize(5).
		SetShowPagination(false).
		SetStatus(1).
		SetCreatedBy(createdBy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	for i, b := range []entity.ViewFieldBuiltinField{entity.ViewFieldBuiltinTitle, entity.ViewFieldBuiltinViewCount} {
		q := c.ViewFieldDef.Create().SetViewDisplayID(popular.ID).SetLabel("").
			SetBuiltinField(string(b)).SetShowLabel(b == entity.ViewFieldBuiltinViewCount).SetSortOrder(i * 10).
			SetCreatedAt(now).SetUpdatedAt(now)
		if b == entity.ViewFieldBuiltinTitle {
			q = q.SetLinkTo("content")
		}
		if err := q.Exec(ctx); err != nil {
			return err
		}
	}
	for i, f := range [][3]string{
		{string(entity.ViewFilterBuiltinContentType), "in", articleType},
		{string(entity.ViewFilterBuiltinStatus), "eq", "1"},
		{string(entity.ViewFilterBuiltinOnModeration), "eq", "0"},
	} {
		if err := c.ViewFilterDef.Create().SetViewDisplayID(popular.ID).
			SetBuiltinField(f[0]).SetOperator(f[1]).SetValue(f[2]).SetSortOrder(i * 10).
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
			return err
		}
	}
	if err := c.ViewSortDef.Create().SetViewDisplayID(popular.ID).
		SetBuiltinField(string(entity.ViewSortBuiltinViews)).SetDirection("desc").SetSortOrder(0).
		SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx); err != nil {
		return err
	}
	// The title shows on Popular, the latest comments and the tag cloud:
	// the search needs none.
	place := func(q *generated.BlockPlacementDefCreate, title string, showTitle bool, weight int) error {
		return q.SetTheme(entity.ThemeKey(theme)).SetRegion("sidebar").
			SetTitle(title).SetShowTitle(showTitle).SetWeight(weight).
			SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx)
	}
	if err := place(c.BlockPlacementDef.Create().SetPlacementType(string(entity.BlockPlacementPlugin)).SetPluginID(string(entity.PluginBlockSearchForm)), titles.Search, false, 0); err != nil {
		return err
	}
	if err := place(c.BlockPlacementDef.Create().SetPlacementType(string(entity.BlockPlacementView)).SetViewDisplayID(popular.ID), titles.Popular, true, 10); err != nil {
		return err
	}
	if err := place(c.BlockPlacementDef.Create().SetPlacementType(string(entity.BlockPlacementPlugin)).SetPluginID(string(entity.PluginBlockLastComments)), titles.Comments, true, 20); err != nil {
		return err
	}
	return place(c.BlockPlacementDef.Create().SetPlacementType(string(entity.BlockPlacementPlugin)).SetPluginID(string(entity.PluginBlockTagCloud)).
		SetPluginConfig(map[string]string{entity.TagCloudVocabularyKey: strconv.Itoa(tagsVID), entity.TagCloudCountKey: "10"}), titles.Tags, true, 30)
}

// createDemoMenuLinks puts the sections (their term pages) and the page in
// the main menu, after Home; returns the menu's ID.
func createDemoMenuLinks(ctx context.Context, c *generated.Client, content demoContent, sections demo.VocabularyRows, pageID int, now time.Time) (int, error) {
	menu, err := c.MenuDef.Query().Where(menudef.MachineName(mainMenu)).Only(ctx)
	if err != nil {
		return 0, fmt.Errorf("the %s menu: %w", mainMenu, err)
	}
	for i, name := range content.Vocabularies.Sections.Terms {
		if err := c.MenuLinkDef.Create().
			SetMenuID(menu.ID).
			SetTitle(name).
			SetLinkType(string(entity.MenuLinkTerm)).
			SetLinkTermID(sections.Terms[name]).
			SetWeight(1 + i).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			Exec(ctx); err != nil {
			return 0, err
		}
	}
	return menu.ID, c.MenuLinkDef.Create().
		SetMenuID(menu.ID).
		SetTitle(content.Page.Title).
		SetLinkType(string(entity.MenuLinkNode)).
		SetLinkNodeID(pageID).
		SetWeight(10).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Exec(ctx)
}

// demoFooterMenu - the machine name of the demo's footer menu.
const demoFooterMenu = "footer"

// createDemoFooterMenu adds the footer menu (the modules' demo steps link
// their pages there), placed in the footer without a title; returns its ID.
func createDemoFooterMenu(ctx context.Context, c *generated.Client, theme, label string, now time.Time) (int, error) {
	menu, err := c.MenuDef.Create().
		SetMachineName(demoFooterMenu).
		SetLabel(label).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return menu.ID, c.BlockPlacementDef.Create().
		SetPlacementType(string(entity.BlockPlacementMenu)).
		SetMenuID(menu.ID).
		SetTheme(entity.ThemeKey(theme)).
		SetRegion("footer").
		SetTitle(label).
		SetShowTitle(false).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Exec(ctx)
}

// createDemoNode adds a published node of type with its body, returning
// its ID; promote - on the front page (as its type's default).
func createDemoNode(ctx context.Context, c *generated.Client, contentType string, d demoNode, comments entity.CommentMode,
	promote bool, createdBy int, at time.Time, bodyField, format int) (int, error) {
	n, err := c.Node.Create().
		SetType(contentType).
		SetTitle(d.Title).
		SetSlug(contentType + "/" + slug.Generate(d.Title)).
		SetStatus(1).
		SetPromote(promote).
		SetCommentMode(string(comments)).
		SetCreatedBy(createdBy).
		SetCreatedAt(at).
		SetUpdatedAt(at).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return n.ID, c.FieldValueText.Create().
		SetNodeID(n.ID).SetFieldID(bodyField).SetValue(d.Body).SetTextFormatID(format).
		Exec(ctx)
}

// The demo covers: 1600×900 pictures (demo.Picture).
const demoCoverWidth, demoCoverHeight = 1600, 900

// createDemoCover writes the i-th cover to the public files directory and
// adds its file row, returning its ID.
func createDemoCover(ctx context.Context, env demo.Env, i int, alt string, at time.Time) (int, error) {
	return demo.ImageFile(ctx, env, path.Join(demoCoverDir, fmt.Sprintf("demo-%d.jpg", i+1)), demoCoverWidth, demoCoverHeight, i, alt, at)
}
