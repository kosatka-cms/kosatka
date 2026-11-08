package demoset

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/demo"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/menudef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/menulinkdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/role"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/textformatdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewdisplaydef"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// Target - where Apply creates a set: the install transaction's client,
// the administrator (the author of what has none), the site's theme (its
// directory: the blocks and menus go in its regions), the time of the
// install, the site's language, the database's dialect and the modules
// the site has (by name; their demo steps get the set's content).
type Target struct {
	Client    *generated.Client
	CreatedBy int
	Theme     string
	Now       time.Time
	Lang      string
	Dialect   string
	Installed map[string]bool
}

// Result - what Apply created that more steps may build on: the users'
// and nodes' IDs by name and key.
type Result struct {
	Users map[string]int
	Nodes map[string]int
}

// The pictures a set's image fields get: generated (demo.Picture),
// 1600×900, under the content images' directory.
const pictureWidth, pictureHeight = 1600, 900

// pictureDir - where a set's pictures go, under the public files.
var pictureDir = path.Join(files.DynamicContentImageBasePath, "demo")

// Apply creates the set on a new site that has the starter set: image
// styles, users, vocabularies, field storages, content types, nodes with
// their values, view counts and comments, Views, blocks, menus and the
// site's slogan and front page, the roles' permissions, then the
// modules' demo steps with the set's content for them.
//
// A node's field values, by its storage's type: text - a string (HTML,
// in the field's first text format); number - a number; bool; date -
// "2006-01-02", "2006-01-02 15:04" or relative to the install, "+10d",
// "-2y", "+5h"; taxonomy - a term name of its vocabulary (several: a
// list); list - an option's value (or a list); reference - a node's key
// (or a list); image and file - a picture number (or a list; a mapping
// {picture, alt}); link - an address or {url, title} (or a list).
func (s *Set) Apply(ctx context.Context, t Target) (Result, error) {
	a := &applier{Target: t, set: s, users: map[string]int{}, vocabularies: map[string]demo.VocabularyRows{},
		storages: map[string]entity.FieldStorageDef{}, types: map[string]typeRows{}, nodes: map[string]int{}, views: map[string]int{},
		repo: database.NewContentTypeRepository(t.Client)}
	steps := []struct {
		name string
		run  func(context.Context) error
	}{
		{"image styles", a.imageStyles},
		{"users", a.createUsers},
		{"vocabularies", a.createVocabularies},
		{"field storages", a.fieldStorages},
		{"content types", a.contentTypes},
		{"nodes", a.createNodes},
		{"views", a.createViews},
		{"blocks", a.blocks},
		{"menus", a.menus},
		{"site", a.site},
		{"permissions", a.permissions},
		{"modules", a.modules},
	}
	for _, st := range steps {
		if err := st.run(ctx); err != nil {
			return Result{}, fmt.Errorf("%s: %w", st.name, err)
		}
	}
	return Result{Users: a.users, Nodes: a.nodes}, nil
}

// typeRows - a content type's row: its ID, address and fields.
type typeRows struct {
	id      int
	path    string
	promote bool
	// comments - the type's default comment mode, a node's without its
	// own (Node.Comments).
	comments entity.CommentMode
	fields   entity.FieldInstanceDefs
}

type applier struct {
	Target
	set          *Set
	users        map[string]int
	vocabularies map[string]demo.VocabularyRows
	storages     map[string]entity.FieldStorageDef
	types        map[string]typeRows
	nodes        map[string]int
	views        map[string]int
	repo         *database.ContentTypeRepository
}

func (a *applier) imageStyles(ctx context.Context) error {
	for _, st := range a.set.ImageStyles {
		if err := a.Client.ImageStyle.Create().SetName(st.Name).SetLabel(st.Label).
			SetWidth(st.Width).SetHeight(st.Height).SetMode(st.Mode).SetAnchor(st.Anchor).SetHidpi(st.HiDPI).
			SetCreatedAt(a.Now).SetUpdatedAt(a.Now).Exec(ctx); err != nil {
			return fmt.Errorf("%s: %w", st.Name, err)
		}
	}
	return nil
}

func (a *applier) createUsers(ctx context.Context) (err error) {
	a.users, err = demo.CreateUsers(ctx, a.Client, a.set.Users, a.Now)
	return err
}

func (a *applier) createVocabularies(ctx context.Context) error {
	for _, v := range a.set.Vocabularies {
		dv := demo.Vocabulary{Name: v.Label, Slug: v.Path, Descriptions: map[string]string{}}
		if dv.Slug == "" {
			dv.Slug = v.Name
		}
		for _, t := range v.Terms {
			dv.Terms = append(dv.Terms, t.Name)
			dv.Descriptions[t.Name] = t.Description
		}
		rows, err := demo.CreateVocabulary(ctx, a.Client, dv, a.CreatedBy, a.Now)
		if err != nil {
			return fmt.Errorf("%s: %w", v.Name, err)
		}
		a.vocabularies[v.Name] = rows
	}
	return nil
}

// fieldStorages creates the set's field storages after reading the
// starter's (a set's content types may use them).
func (a *applier) fieldStorages(ctx context.Context) error {
	existing, err := a.repo.FieldStorages(ctx)
	if err != nil {
		return err
	}
	for _, fs := range existing {
		a.storages[fs.MachineName] = fs
	}
	for _, fs := range a.set.FieldStorages {
		config := maps.Clone(fs.Config)
		if config == nil {
			config = map[string]any{}
		}
		if fs.Multiple {
			config["multiple"] = true
		}
		if fs.Vocabulary != "" {
			config["vocabulary_id"] = a.vocabularies[fs.Vocabulary].ID
		}
		if len(fs.Target) > 0 {
			config["target_content_type"] = strings.Join(fs.Target, ",")
		}
		if len(fs.Options) > 0 {
			var options []any
			for _, o := range fs.Options {
				options = append(options, map[string]any{"value": o.Value, "label": o.Label})
			}
			config["allowed_values"] = options
		}
		row, err := a.Client.FieldStorageDef.Create().SetMachineName(fs.Name).SetFieldType(string(fs.Type)).
			SetConfig(config).SetCreatedAt(a.Now).SetUpdatedAt(a.Now).Save(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", fs.Name, err)
		}
		a.storages[fs.Name] = entity.FieldStorageDef{ID: row.ID, MachineName: fs.Name, FieldType: fs.Type, Config: config}
	}
	return nil
}

// storage - the field storage name (the set's or the starter's).
func (a *applier) storage(name string) (entity.FieldStorageDef, error) {
	fs, ok := a.storages[name]
	if !ok {
		return fs, fmt.Errorf("no field storage %q", name)
	}
	return fs, nil
}

// contentTypes creates the set's content types with their fields, then
// reads every content type's fields (the starter's too: a set's nodes may
// be of its types).
func (a *applier) contentTypes(ctx context.Context) error {
	for _, t := range a.set.ContentTypes {
		prefix := t.Path
		if prefix == "" {
			prefix = t.Name
		}
		ct, err := a.Client.ContentTypeDef.Create().
			SetMachineName(t.Name).
			SetLabel(t.Label).
			SetDescription(t.Description).
			SetPathPattern(prefix + "/[node:title]").
			SetStatus(1).
			SetDefaultCommentMode(string(t.Comments.OrDefault())).
			SetDefaultPromote(t.Promote).
			SetShowSubmitted(t.ShowSubmitted).
			SetShowViewCount(t.ShowViewCount).
			SetRevisions(t.Revisions).
			SetCreatedBy(a.CreatedBy).
			SetCreatedAt(a.Now).
			SetUpdatedAt(a.Now).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
		for _, f := range t.Fields {
			if err := a.field(ctx, ct.ID, f); err != nil {
				return fmt.Errorf("%s.%s: %w", t.Name, f.Storage, err)
			}
		}
	}
	all, err := a.Client.ContentTypeDef.Query().All(ctx)
	if err != nil {
		return err
	}
	for _, ct := range all {
		fields, err := a.repo.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", ct.MachineName, err)
		}
		prefix := ct.MachineName
		if p, ok := strings.CutSuffix(ct.PathPattern, "/[node:title]"); ok && p != "" {
			prefix = p
		}
		a.types[ct.MachineName] = typeRows{id: ct.ID, path: prefix, promote: ct.DefaultPromote, comments: entity.CommentMode(ct.DefaultCommentMode), fields: fields}
	}
	return nil
}

// field attaches a field storage to content type typeID.
func (a *applier) field(ctx context.Context, typeID int, f Field) error {
	fs, err := a.storage(f.Storage)
	if err != nil {
		return err
	}
	config := map[string]any{}
	if f.Widget != "" {
		config["widget"] = f.Widget
	}
	if len(f.TextFormats) > 0 {
		ids, err := a.Client.TextFormatDef.Query().Where(textformatdef.MachineNameIn(f.TextFormats...)).
			Order(generated.Asc(textformatdef.FieldWeight)).IDs(ctx)
		if err != nil {
			return err
		}
		if len(ids) != len(f.TextFormats) {
			return fmt.Errorf("text formats %v: %d found", f.TextFormats, len(ids))
		}
		config["allowed_text_formats"] = ids
	}
	return a.Client.FieldInstanceDef.Create().
		SetContentTypeID(typeID).
		SetFieldStorageID(fs.ID).
		SetConfig(config).
		SetLabel(f.Label).
		SetRequired(f.Required).
		SetShowOnDetail(f.ShowOnDetail).
		SetShowLabel(f.ShowLabel).
		SetRendersInBody(f.RendersInBody).
		SetDetailImageStyle(f.DetailImageStyle).
		SetListImageStyle(f.ListImageStyle).
		SetDateFormat(f.DateFormat).
		SetSortOrder(f.SortOrder).
		SetDisplayOrder(f.DisplayOrder).
		SetCreatedAt(a.Now).
		SetUpdatedAt(a.Now).
		Exec(ctx)
}

// agoPattern - a time before the install: days, hours, minutes ("2d5h").
var agoPattern = regexp.MustCompile(`^(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?$`)

// parseAgo - the duration of Node.Ago ("" - none).
func parseAgo(s string) (time.Duration, error) {
	m := agoPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("ago %q: days, hours, minutes, like 2d5h", s)
	}
	var d time.Duration
	for i, unit := range []time.Duration{24 * time.Hour, time.Hour, time.Minute} {
		if m[i+1] != "" {
			n, _ := strconv.Atoi(m[i+1])
			d += time.Duration(n) * unit
		}
	}
	return d, nil
}

// createNodes creates the nodes, then their values (a reference may name
// a node after it), view counts and comments.
func (a *applier) createNodes(ctx context.Context) error {
	env := demo.Env{Client: a.Client, CreatedBy: a.CreatedBy, Users: a.users, Now: a.Now}
	created := map[string]time.Time{}
	for _, n := range a.set.Nodes {
		t, ok := a.types[n.Type]
		if !ok {
			return fmt.Errorf("node %q: no content type %q", n.Key, n.Type)
		}
		ago, _ := parseAgo(n.Ago)
		at := a.Now.Add(-ago)
		author := a.CreatedBy
		if n.Author != "" {
			author = a.users[n.Author]
		}
		promote := t.promote
		if n.Promote != nil {
			promote = *n.Promote
		}
		comments := n.Comments
		if comments == "" {
			comments = t.comments
		}
		address := n.Path
		if address == "" {
			address = t.path + "/" + slug.Generate(n.Title)
		}
		row, err := a.Client.Node.Create().
			SetType(n.Type).
			SetTitle(n.Title).
			SetSlug(address).
			SetStatus(1).
			SetOnModeration(0).
			SetPromote(promote).
			SetSticky(n.Sticky).
			SetCommentMode(string(comments.OrDefault())).
			SetCreatedBy(author).
			SetCreatedAt(at).
			SetUpdatedAt(at).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("node %q: %w", n.Key, err)
		}
		a.nodes[n.Key] = row.ID
		created[n.Key] = at
	}
	taxonomy := database.NewTaxonomyRepository(a.Client)
	for _, n := range a.set.Nodes {
		id := a.nodes[n.Key]
		fields := a.types[n.Type].fields
		values, formats, terms, err := a.values(ctx, n, fields, created[n.Key])
		if err != nil {
			return fmt.Errorf("node %q: %w", n.Key, err)
		}
		if err := a.repo.SetFieldValues(ctx, id, fields, values, formats); err != nil {
			return fmt.Errorf("node %q: %w", n.Key, err)
		}
		if err := taxonomy.UpdateNodeTerms(ctx, id, terms); err != nil {
			return fmt.Errorf("node %q: %w", n.Key, err)
		}
		if n.Views > 0 {
			if err := a.Client.NodeCounter.Create().SetNid(id).SetTotalcount(n.Views).SetDaycount(n.Views / 25).SetLastViewed(a.Now).Exec(ctx); err != nil {
				return err
			}
		}
		if _, _, err := demo.CreateComments(ctx, env, id, n.Replies, created[n.Key]); err != nil {
			return fmt.Errorf("node %q: comments: %w", n.Key, err)
		}
	}
	return nil
}

// values - node n's field values (by field instance) as the field stores
// take them, their text formats and the terms among them.
func (a *applier) values(ctx context.Context, n Node, fields entity.FieldInstanceDefs, at time.Time) (map[int]any, map[int]int, []int, error) {
	values, formats := map[int]any{}, map[int]int{}
	var terms []int
	for name, raw := range n.Fields {
		i := slices.IndexFunc(fields, func(f entity.FieldInstanceDef) bool { return f.MachineName == name })
		if i < 0 {
			return nil, nil, nil, fmt.Errorf("its type %s has no field %q", n.Type, name)
		}
		f := fields[i]
		items := list(raw)
		var v any
		var err error
		switch f.FieldType {
		case entity.FieldTypeText:
			s, ok := raw.(string)
			if !ok {
				return nil, nil, nil, fmt.Errorf("%s: a text", name)
			}
			v = strings.TrimSpace(s)
			if ids := f.AllowedTextFormatIDs(); len(ids) > 0 {
				formats[f.ID] = ids[0]
			}
		case entity.FieldTypeNumber:
			v, err = number(raw)
		case entity.FieldTypeBool:
			b, ok := raw.(bool)
			if !ok {
				err = fmt.Errorf("a yes or no")
			}
			v = b
		case entity.FieldTypeDate:
			var dates []entity.DateValue
			for _, item := range items {
				d, err := a.date(item)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
				}
				dates = append(dates, d)
			}
			v = dates
		case entity.FieldTypeTaxonomy:
			var ids []int
			if ids, err = a.termIDs(f, items); err == nil {
				terms = append(terms, ids...)
				v = one(f, ids)
			}
		case entity.FieldTypeList:
			var vals []string
			for _, item := range items {
				vals = append(vals, fmt.Sprint(item))
			}
			if f.Multiple() {
				v = vals
			} else if len(vals) > 0 {
				v = vals[0]
			}
		case entity.FieldTypeReference:
			var ids []int
			for _, item := range items {
				id, ok := a.nodes[fmt.Sprint(item)]
				if !ok {
					return nil, nil, nil, fmt.Errorf("%s: no node %q", name, item)
				}
				ids = append(ids, id)
			}
			v = one(f, ids)
		case entity.FieldTypeImage, entity.FieldTypeFile:
			var ids []int
			for j, item := range items {
				id, err := a.picture(ctx, n, name, j, item, at)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
				}
				ids = append(ids, id)
			}
			v = one(f, ids)
		case entity.FieldTypeLink:
			var links []entity.Link
			for _, item := range items {
				l, err := link(item)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
				}
				links = append(links, l)
			}
			v = links
		default:
			err = fmt.Errorf("a %s field takes no demo values", f.FieldType)
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		values[f.ID] = v
	}
	return values, formats, terms, nil
}

// list - raw as a list of values (a value alone - one).
func list(raw any) []any {
	if l, ok := raw.([]any); ok {
		return l
	}
	return []any{raw}
}

// one - ids as field f stores them: all of them, or the first.
func one(f entity.FieldInstanceDef, ids []int) any {
	if f.Multiple() {
		return ids
	}
	if len(ids) == 0 {
		return nil
	}
	return ids[0]
}

func number(raw any) (float64, error) {
	switch n := raw.(type) {
	case int:
		return float64(n), nil
	case float64:
		return n, nil
	}
	return 0, fmt.Errorf("a number, not %v", raw)
}

// relativeDate - a date relative to the install: "+10d", "-2y", "+5h".
var relativeDate = regexp.MustCompile(`^([+-]\d+)([hdy])$`)

// date - a date value: absolute ("2006-01-02", "2006-01-02 15:04") or
// relative to the install (relativeDate; hours - with the time).
func (a *applier) date(raw any) (entity.DateValue, error) {
	// YAML reads an unquoted 2006-01-02 as a time.
	if t, ok := raw.(time.Time); ok {
		return entity.DateValue{Start: t, WithTime: t.Hour() != 0 || t.Minute() != 0}, nil
	}
	s := strings.TrimSpace(fmt.Sprint(raw))
	if m := relativeDate.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		day := time.Date(a.Now.Year(), a.Now.Month(), a.Now.Day(), 0, 0, 0, 0, time.UTC)
		switch m[2] {
		case "h":
			return entity.DateValue{Start: a.Now.Add(time.Duration(n) * time.Hour).Truncate(time.Hour), WithTime: true}, nil
		case "d":
			return entity.DateValue{Start: day.AddDate(0, 0, n)}, nil
		default:
			return entity.DateValue{Start: day.AddDate(n, 0, 0)}, nil
		}
	}
	if t, err := time.Parse("2006-01-02 15:04", s); err == nil {
		return entity.DateValue{Start: t, WithTime: true}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return entity.DateValue{}, fmt.Errorf("a date, not %q", s)
	}
	return entity.DateValue{Start: t}, nil
}

// termIDs - the terms named of taxonomy field f's vocabulary.
func (a *applier) termIDs(f entity.FieldInstanceDef, names []any) ([]int, error) {
	var voc *demo.VocabularyRows
	for _, v := range a.vocabularies {
		if v.ID == f.VocabularyID() {
			voc = &v
			break
		}
	}
	if voc == nil {
		return nil, fmt.Errorf("its vocabulary isn't the set's")
	}
	var ids []int
	for _, n := range names {
		id, ok := voc.Terms[fmt.Sprint(n)]
		if !ok {
			return nil, fmt.Errorf("no term %q", n)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// picture - the file of a generated picture: its number (an int), or
// {picture, alt}; the node's title is its alternative text by default.
func (a *applier) picture(ctx context.Context, n Node, field string, j int, raw any, at time.Time) (int, error) {
	number, alt := 0, n.Title
	switch v := raw.(type) {
	case int:
		number = v
	case map[string]any:
		p, ok := v["picture"].(int)
		if !ok {
			return 0, fmt.Errorf("a picture number")
		}
		number = p
		if s, ok := v["alt"].(string); ok {
			alt = s
		}
	default:
		return 0, fmt.Errorf("a picture number, not %v", raw)
	}
	rel := path.Join(pictureDir, fmt.Sprintf("%s-%s-%d.jpg", slug.Generate(n.Key), strings.TrimPrefix(field, "field_"), j))
	env := demo.Env{Client: a.Client, CreatedBy: a.CreatedBy}
	return demo.ImageFile(ctx, env, rel, pictureWidth, pictureHeight, number, alt, at)
}

// link - a link value: an address, or {url, title}.
func link(raw any) (entity.Link, error) {
	switch v := raw.(type) {
	case string:
		return entity.Link{URL: v}, nil
	case map[string]any:
		u, _ := v["url"].(string)
		t, _ := v["title"].(string)
		if u == "" {
			return entity.Link{}, fmt.Errorf("a link without url")
		}
		return entity.Link{URL: u, Title: t}, nil
	}
	return entity.Link{}, fmt.Errorf("a link, not %v", raw)
}

func (a *applier) createViews(ctx context.Context) error {
	for _, v := range a.set.Views {
		id, err := a.view(ctx, v)
		if err != nil {
			return fmt.Errorf("%s: %w", v.Name, err)
		}
		a.views[v.Name] = id
	}
	return nil
}

// view creates a View of content with its fields, filters and sorts in
// their order (10 apart).
func (a *applier) view(ctx context.Context, v View) (int, error) {
	style := v.Style
	if style == "" {
		style = entity.ViewDisplayStyleList
	}
	q := a.Client.ViewDisplayDef.Create().
		SetMachineName(v.Name).
		SetLabel(v.Label).
		SetDisplayType(string(v.DisplayType)).
		SetBaseEntity("content").
		SetDisplayStyle(string(style)).
		SetGridColumns(v.GridColumns).
		SetShowPagination(v.Pager).
		SetPageSize(v.PageSize).
		SetSkipItems(v.Skip).
		SetThemeTemplate(v.ThemeTemplate).
		SetAttachContentTypes(strings.Join(v.AttachTypes, ",")).
		SetAttachTitle(v.AttachTitle).
		SetAttachShowTitle(v.AttachTitle != "").
		SetGroupDateFormat(v.GroupDateFormat).
		SetHeaderText(v.Header).
		SetStatus(1).
		SetCreatedBy(a.CreatedBy).
		SetCreatedAt(a.Now).
		SetUpdatedAt(a.Now)
	// The address is unique: only a page's.
	if v.Path != "" {
		q = q.SetURLPath(v.Path)
	}
	if len(v.TermVocabularies) > 0 {
		var ids []string
		for _, name := range v.TermVocabularies {
			ids = append(ids, strconv.Itoa(a.vocabularies[name].ID))
		}
		q = q.SetTermVocabularies(strings.Join(ids, ","))
	}
	if v.GroupBy != "" {
		q = q.SetGroupByField(v.GroupBy)
	}
	if v.TabPath != "" {
		access := v.TabAccess
		if access == "" {
			access = string(entity.ViewProfileTabAccessAll)
		}
		q = q.SetProfileTabPath(v.TabPath).SetProfileTabAccess(access)
	}
	display, err := q.Save(ctx)
	if err != nil {
		return 0, err
	}
	for i, f := range v.Fields {
		c := a.Client.ViewFieldDef.Create().SetViewDisplayID(display.ID).SetLabel(f.Label).SetShowLabel(f.ShowLabel).
			SetSortOrder(i * 10).SetImageStyle(f.ImageStyle).SetLinkTo(f.LinkTo).SetDateFormat(f.DateFormat).
			SetTrimLength(f.TrimLength).SetTagFormat(f.TagFormat).SetCSSClass(f.CSSClass).
			SetCreatedAt(a.Now).SetUpdatedAt(a.Now)
		if f.Storage != "" {
			fs, err := a.storage(f.Storage)
			if err != nil {
				return 0, err
			}
			c = c.SetFieldStorageID(fs.ID)
		} else {
			c = c.SetBuiltinField(f.Builtin)
		}
		if len(f.Params) > 0 {
			c = c.SetParams(f.Params)
		}
		if err := c.Exec(ctx); err != nil {
			return 0, err
		}
	}
	for i, f := range v.Filters {
		value := f.Value
		if f.Storage != "" {
			var err error
			if value, err = a.filterValue(f); err != nil {
				return 0, err
			}
		}
		c := a.Client.ViewFilterDef.Create().SetViewDisplayID(display.ID).SetOperator(f.Operator).SetValue(value).
			SetValueIsRelative(f.Relative).SetExposed(f.Exposed != "").SetExposedParam(f.Exposed).SetExposedLabel(f.ExposedLabel).
			SetContextual(f.Contextual).SetSortOrder(i * 10).SetCreatedAt(a.Now).SetUpdatedAt(a.Now)
		if f.Storage != "" {
			c = c.SetFieldStorageID(a.storages[f.Storage].ID)
		} else {
			c = c.SetBuiltinField(f.Builtin)
		}
		if err := c.Exec(ctx); err != nil {
			return 0, err
		}
	}
	for i, st := range v.Sorts {
		c := a.Client.ViewSortDef.Create().SetViewDisplayID(display.ID).SetDirection(st.Direction).SetSortOrder(i * 10).
			SetExposedKey(st.Exposed).SetExposedLabel(st.ExposedLabel).SetCreatedAt(a.Now).SetUpdatedAt(a.Now)
		if st.Storage != "" {
			fs, err := a.storage(st.Storage)
			if err != nil {
				return 0, err
			}
			c = c.SetFieldStorageID(fs.ID)
		} else {
			c = c.SetBuiltinField(st.Builtin)
		}
		if len(st.Params) > 0 {
			c = c.SetParams(st.Params)
		}
		if err := c.Exec(ctx); err != nil {
			return 0, err
		}
	}
	return display.ID, nil
}

// filterValue - a fixed filter's value on a field storage as the View
// stores it: a taxonomy field's terms by their IDs (the set names them),
// a reference's nodes by theirs; the others as they are.
func (a *applier) filterValue(f ViewFilter) (string, error) {
	fs, err := a.storage(f.Storage)
	if err != nil || f.Value == "" {
		return f.Value, err
	}
	var ids []string
	for _, item := range strings.Split(f.Value, ",") {
		item = strings.TrimSpace(item)
		switch fs.FieldType {
		case entity.FieldTypeTaxonomy:
			found := false
			for _, v := range a.vocabularies {
				if v.ID == fs.VocabularyID() {
					if id, ok := v.Terms[item]; ok {
						ids, found = append(ids, strconv.Itoa(id)), true
					}
				}
			}
			if !found {
				return "", fmt.Errorf("filter on %s: no term %q", f.Storage, item)
			}
		case entity.FieldTypeReference:
			id, ok := a.nodes[item]
			if !ok {
				return "", fmt.Errorf("filter on %s: no node %q", f.Storage, item)
			}
			ids = append(ids, strconv.Itoa(id))
		default:
			return f.Value, nil
		}
	}
	return strings.Join(ids, ","), nil
}

// blocks places the set's blocks in the regions of the site's theme.
func (a *applier) blocks(ctx context.Context) error {
	for i, b := range a.set.Blocks {
		q := a.Client.BlockPlacementDef.Create()
		switch {
		case b.View != "":
			id, ok := a.views[b.View]
			if !ok {
				return fmt.Errorf("block %q: no view %q", b.Title, b.View)
			}
			q = q.SetPlacementType(string(entity.BlockPlacementView)).SetViewDisplayID(id)
		case b.Plugin != "":
			config := maps.Clone(b.Config)
			if config == nil {
				config = map[string]string{}
			}
			if b.Vocabulary != "" {
				config[entity.TagCloudVocabularyKey] = strconv.Itoa(a.vocabularies[b.Vocabulary].ID)
			}
			q = q.SetPlacementType(string(entity.BlockPlacementPlugin)).SetPluginID(b.Plugin).SetPluginConfig(config)
		default:
			format, err := a.Client.TextFormatDef.Query().Where(textformatdef.MachineName("basic_html")).OnlyID(ctx)
			if err != nil {
				return fmt.Errorf("the basic_html text format: %w", err)
			}
			block, err := a.Client.BlockDef.Create().SetTitle(b.Title).SetShowTitle(b.ShowTitle).SetBody(strings.TrimSpace(b.Body)).
				SetTextFormatID(format).SetStatus(true).SetCreatedAt(a.Now).SetUpdatedAt(a.Now).Save(ctx)
			if err != nil {
				return fmt.Errorf("block %q: %w", b.Title, err)
			}
			q = q.SetPlacementType(string(entity.BlockPlacementCustom)).SetBlockID(block.ID)
		}
		switch {
		case len(b.Types) > 0:
			q = q.SetVisibilityScope(string(entity.BlockVisibilityContentType)).SetVisibilityContentType(strings.Join(b.Types, ","))
		case b.Visibility != "":
			q = q.SetVisibilityScope(b.Visibility)
		}
		weight := b.Weight
		if weight == 0 {
			weight = i * 10
		}
		if err := q.SetTheme(entity.ThemeKey(a.Theme)).SetRegion(b.Region).SetTitle(b.Title).SetShowTitle(b.ShowTitle).
			SetWeight(weight).SetCreatedAt(a.Now).SetUpdatedAt(a.Now).Exec(ctx); err != nil {
			return fmt.Errorf("block %q: %w", b.Title, err)
		}
	}
	return nil
}

// menus creates the set's menus, placed in their regions, or adds links
// to a menu the site has (the starter's main menu) after its own.
func (a *applier) menus(ctx context.Context) error {
	for _, m := range a.set.Menus {
		menu, err := a.Client.MenuDef.Query().Where(menudef.MachineName(m.Name)).Only(ctx)
		first := 0
		switch {
		case err == nil:
			first, err = a.Client.MenuLinkDef.Query().Where(menulinkdef.MenuID(menu.ID)).Count(ctx)
			if err != nil {
				return err
			}
		case generated.IsNotFound(err):
			if menu, err = a.Client.MenuDef.Create().SetMachineName(m.Name).SetLabel(m.Label).
				SetCreatedAt(a.Now).SetUpdatedAt(a.Now).Save(ctx); err != nil {
				return fmt.Errorf("%s: %w", m.Name, err)
			}
			if m.Region != "" {
				if err := a.Client.BlockPlacementDef.Create().SetPlacementType(string(entity.BlockPlacementMenu)).SetMenuID(menu.ID).
					SetTheme(entity.ThemeKey(a.Theme)).SetRegion(m.Region).SetCreatedAt(a.Now).SetUpdatedAt(a.Now).Exec(ctx); err != nil {
					return fmt.Errorf("%s: %w", m.Name, err)
				}
			}
		default:
			return err
		}
		for i, l := range m.Links {
			q := a.Client.MenuLinkDef.Create().SetMenuID(menu.ID).SetTitle(l.Title).SetWeight(first + i).
				SetCreatedAt(a.Now).SetUpdatedAt(a.Now)
			switch {
			case l.Node != "":
				q = q.SetLinkType(string(entity.MenuLinkNode)).SetLinkNodeID(a.nodes[l.Node])
			case l.View != "":
				q = q.SetLinkType(string(entity.MenuLinkView)).SetLinkViewID(a.views[l.View])
			case l.Term != "":
				voc, term, _ := strings.Cut(l.Term, "/")
				q = q.SetLinkType(string(entity.MenuLinkTerm)).SetLinkTermID(a.vocabularies[voc].Terms[term])
			default:
				q = q.SetLinkType(string(entity.MenuLinkURL)).SetLinkURL(l.URL)
			}
			if err := q.Exec(ctx); err != nil {
				return fmt.Errorf("%s: %q: %w", m.Name, l.Title, err)
			}
		}
	}
	return nil
}

// permissions adds the set's permissions to its roles (by name), keeping
// the ones they have.
func (a *applier) permissions(ctx context.Context) error {
	for _, name := range slices.Sorted(maps.Keys(a.set.Permissions)) {
		r, err := a.Client.Role.Query().Where(role.Name(name)).Only(ctx)
		if err != nil {
			return fmt.Errorf("role %q: %w", name, err)
		}
		perms := r.Permissions
		if perms == nil {
			perms = map[string]any{}
		}
		for _, p := range a.set.Permissions[name] {
			perms[p] = true
		}
		if err := a.Client.Role.UpdateOne(r).SetPermissions(perms).Exec(ctx); err != nil {
			return fmt.Errorf("role %q: %w", name, err)
		}
	}
	return nil
}

// modules runs the demo step of every module of the set's Modules the
// site has (in name order), with its content as JSON (demo.Env.Data).
func (a *applier) modules(ctx context.Context) error {
	if len(a.set.Modules) == 0 {
		return nil
	}
	env := demo.Env{Client: a.Client, Lang: a.Lang, CreatedBy: a.CreatedBy, Users: a.users, Nodes: a.nodes,
		Theme: entity.ThemeKey(a.Theme), Now: a.Now, Dialect: a.Dialect}
	var err error
	if env.MainMenu, err = a.menuID(ctx, a.set.Site.MainMenu); err != nil {
		return err
	}
	if env.FooterMenu, err = a.menuID(ctx, a.set.Site.FooterMenu); err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(a.set.Modules)) {
		if !demo.Has(name) {
			// A theme's set may name a module of its own repository the
			// site isn't built with (install said so).
			if a.set.FromTheme {
				continue
			}
			return fmt.Errorf("%s: no module with a demo step", name)
		}
		if !a.Installed[name] {
			continue
		}
		if env.Data, err = json.Marshal(a.set.Modules[name]); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := demo.Run(ctx, name, env); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// menuID - the ID of the menu name (0 for "").
func (a *applier) menuID(ctx context.Context, name string) (int, error) {
	if name == "" {
		return 0, nil
	}
	id, err := a.Client.MenuDef.Query().Where(menudef.MachineName(name)).OnlyID(ctx)
	if err != nil {
		return 0, fmt.Errorf("menu %q: %w", name, err)
	}
	return id, nil
}

// site sets the slogan and the front page (a View of the set, or one the
// site has).
func (a *applier) site(ctx context.Context) error {
	q := a.Client.SiteInformation.UpdateOneID(1)
	if a.set.Site.Slogan != "" {
		q = q.SetSlogan(a.set.Site.Slogan)
	}
	if v := a.set.Site.FrontPageView; v != "" {
		if _, ok := a.views[v]; !ok {
			if n, err := a.Client.ViewDisplayDef.Query().Where(viewdisplaydef.MachineName(v)).Count(ctx); err != nil || n == 0 {
				return fmt.Errorf("no front page view %q", v)
			}
		}
		q = q.SetFrontFeedView(v)
	}
	return q.Exec(ctx)
}
