package starter

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/textformatdef"
)

// Result - what Apply created that the demo content builds on: the field
// storages' ids by name.
type Result struct {
	Storages map[string]int
}

// Apply creates the set on a new site through c (the install's
// transaction): its image styles, field storages, content types with
// their fields, Views, menus (placed in the site theme's regions) and the
// front page; createdBy is their author, label translates their labels
// into the site's language.
func (s *Set) Apply(ctx context.Context, c *generated.Client, createdBy int, label func(string) string) (Result, error) {
	a := applier{c: c, createdBy: createdBy, label: label, now: time.Now(), storages: map[string]int{}}
	steps := []struct {
		name string
		run  func(context.Context, *Set) error
	}{
		{"image styles", a.imageStyles},
		{"field storages", a.fieldStorages},
		{"content types", a.contentTypes},
		{"views", a.views},
		{"menus", a.menus},
		{"site", a.site},
	}
	for _, st := range steps {
		if err := st.run(ctx, s); err != nil {
			return Result{}, fmt.Errorf("%s: %w", st.name, err)
		}
	}
	return Result{Storages: a.storages}, nil
}

type applier struct {
	c         *generated.Client
	createdBy int
	label     func(string) string
	now       time.Time
	storages  map[string]int
}

func (a *applier) imageStyles(ctx context.Context, s *Set) error {
	for _, st := range s.ImageStyles {
		if err := a.c.ImageStyle.Create().SetName(st.Name).SetLabel(a.label(st.Label)).
			SetWidth(st.Width).SetHeight(st.Height).SetMode(st.Mode).SetAnchor(st.Anchor).
			SetHidpi(st.HiDPI).
			SetCreatedAt(a.now).SetUpdatedAt(a.now).Exec(ctx); err != nil {
			return fmt.Errorf("%s: %w", st.Name, err)
		}
	}
	return nil
}

func (a *applier) fieldStorages(ctx context.Context, s *Set) error {
	for _, fs := range s.FieldStorages {
		row, err := a.c.FieldStorageDef.Create().SetMachineName(fs.Name).SetFieldType(string(fs.Type)).
			SetConfig(map[string]any{}).SetCreatedAt(a.now).SetUpdatedAt(a.now).Save(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", fs.Name, err)
		}
		a.storages[fs.Name] = row.ID
	}
	return nil
}

func (a *applier) contentTypes(ctx context.Context, s *Set) error {
	for _, t := range s.ContentTypes {
		ct, err := a.c.ContentTypeDef.Create().
			SetMachineName(t.Name).
			SetLabel(a.label(t.Label)).
			SetDescription(a.label(t.Description)).
			SetStatus(1).
			SetDefaultCommentMode(string(t.Comments)).
			SetDefaultPromote(t.Promote).
			SetShowSubmitted(t.ShowSubmitted).
			SetRevisions(t.Revisions).
			SetCreatedBy(a.createdBy).
			SetCreatedAt(a.now).
			SetUpdatedAt(a.now).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
		for _, f := range t.Fields {
			config, err := a.fieldConfig(ctx, f)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", t.Name, f.Storage, err)
			}
			if err := a.c.FieldInstanceDef.Create().
				SetContentTypeID(ct.ID).
				SetFieldStorageID(a.storages[f.Storage]).
				SetConfig(config).
				SetLabel(a.label(f.Label)).
				SetShowOnDetail(f.ShowOnDetail).
				SetShowLabel(f.ShowLabel).
				SetRendersInBody(f.RendersInBody).
				SetDetailImageStyle(f.DetailImageStyle).
				SetListImageStyle(f.ListImageStyle).
				SetSortOrder(f.SortOrder).
				SetDisplayOrder(f.DisplayOrder).
				SetCreatedAt(a.now).
				SetUpdatedAt(a.now).
				Exec(ctx); err != nil {
				return fmt.Errorf("%s.%s: %w", t.Name, f.Storage, err)
			}
		}
	}
	return nil
}

// fieldConfig - a field's config: its widget and the ids of the text
// formats it allows (by their weight, the first the default).
func (a *applier) fieldConfig(ctx context.Context, f Field) (map[string]any, error) {
	config := map[string]any{}
	if f.Widget != "" {
		config["widget"] = f.Widget
	}
	if len(f.TextFormats) > 0 {
		ids, err := a.c.TextFormatDef.Query().
			Where(textformatdef.MachineNameIn(f.TextFormats...)).
			Order(generated.Asc(textformatdef.FieldWeight)).
			IDs(ctx)
		if err != nil {
			return nil, err
		}
		if len(ids) != len(f.TextFormats) {
			return nil, fmt.Errorf("text formats %v: %d found", f.TextFormats, len(ids))
		}
		config["allowed_text_formats"] = ids
	}
	return config, nil
}

func (a *applier) views(ctx context.Context, s *Set) error {
	for _, v := range s.Views {
		if err := a.view(ctx, v); err != nil {
			return fmt.Errorf("%s: %w", v.Name, err)
		}
	}
	return nil
}

// view creates a View of content, its fields, filters and sorts in their
// order (10 apart).
func (a *applier) view(ctx context.Context, v View) error {
	display, err := a.c.ViewDisplayDef.Create().
		SetMachineName(v.Name).
		SetLabel(a.label(v.Label)).
		SetDisplayType(string(v.DisplayType)).
		SetBaseEntity("content").
		SetDisplayStyle(string(v.Style)).
		SetShowPagination(v.Pager).
		SetPageSize(v.PageSize).
		SetStatus(1).
		SetCreatedBy(a.createdBy).
		SetCreatedAt(a.now).
		SetUpdatedAt(a.now).
		Save(ctx)
	if err != nil {
		return err
	}
	for i, f := range v.Fields {
		q := a.c.ViewFieldDef.Create().SetViewDisplayID(display.ID).SetLabel("").SetShowLabel(false).SetSortOrder(i * 10).
			SetCreatedAt(a.now).SetUpdatedAt(a.now)
		if f.Storage != "" {
			q = q.SetFieldStorageID(a.storages[f.Storage])
		} else {
			q = q.SetBuiltinField(f.Builtin)
		}
		if f.ImageStyle != "" {
			q = q.SetImageStyle(f.ImageStyle)
		}
		if f.LinkTo != "" {
			q = q.SetLinkTo(f.LinkTo)
		}
		if f.DateFormat != "" {
			q = q.SetDateFormat(f.DateFormat)
		}
		if f.TrimLength > 0 {
			q = q.SetTrimLength(f.TrimLength)
		}
		if err := q.Exec(ctx); err != nil {
			return err
		}
	}
	for i, f := range v.Filters {
		if err := a.c.ViewFilterDef.Create().SetViewDisplayID(display.ID).
			SetBuiltinField(f.Builtin).SetOperator(f.Operator).SetValue(f.Value).SetContextual(f.Contextual).SetSortOrder(i * 10).
			SetCreatedAt(a.now).SetUpdatedAt(a.now).Exec(ctx); err != nil {
			return err
		}
	}
	for i, st := range v.Sorts {
		if err := a.c.ViewSortDef.Create().SetViewDisplayID(display.ID).
			SetBuiltinField(st.Builtin).SetDirection(st.Direction).SetSortOrder(i * 10).
			SetCreatedAt(a.now).SetUpdatedAt(a.now).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// menus creates the menus with their links and places each in its
// region of the site's theme.
func (a *applier) menus(ctx context.Context, s *Set) error {
	for _, m := range s.Menus {
		menu, err := a.c.MenuDef.Create().SetMachineName(m.Name).SetLabel(a.label(m.Label)).
			SetCreatedAt(a.now).SetUpdatedAt(a.now).Save(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", m.Name, err)
		}
		for i, l := range m.Links {
			if err := a.c.MenuLinkDef.Create().SetMenuID(menu.ID).SetTitle(a.label(l.Title)).SetLinkURL(l.URL).
				SetWeight(i).SetCreatedAt(a.now).SetUpdatedAt(a.now).Exec(ctx); err != nil {
				return fmt.Errorf("%s: %w", m.Name, err)
			}
		}
		if m.Region == "" {
			continue
		}
		if err := a.c.BlockPlacementDef.Create().SetPlacementType("menu").SetMenuID(menu.ID).
			SetTheme(entity.ThemeKey(s.Site.Theme)).SetRegion(m.Region).
			SetCreatedAt(a.now).SetUpdatedAt(a.now).Exec(ctx); err != nil {
			return fmt.Errorf("%s: %w", m.Name, err)
		}
	}
	return nil
}

// site makes the set's View the site's front page.
func (a *applier) site(ctx context.Context, s *Set) error {
	if s.Site.FrontPageView == "" {
		return nil
	}
	return a.c.SiteInformation.UpdateOneID(1).SetFrontFeedView(s.Site.FrontPageView).Exec(ctx)
}
