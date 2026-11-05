package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/menudef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/menulinkdef"
)

// NewMenuRepository - the menu repository on client.
func NewMenuRepository(client *generated.Client) *MenuRepository {
	return &MenuRepository{client: client}
}

// MenuRepository is the ent adapter for entity.MenuRepository.
type MenuRepository struct {
	client *generated.Client
}

// Menus - every menu, by label.
func (r MenuRepository) Menus(ctx context.Context) (entity.MenuDefs, error) {
	client := GetClient(ctx, r.client)
	res, err := client.MenuDef.Query().
		Order(generated.Asc(menudef.FieldLabel)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("MenuRepository.Menus: %w", err)
	}

	menus := make(entity.MenuDefs, len(res))
	for i, v := range res {
		menus[i] = dao.ToMenuDefEntity(v)
	}
	return menus, nil
}

// MenuByID - the menu with id; ErrEntityNotFound if there is none.
func (r MenuRepository) MenuByID(ctx context.Context, id int) (*entity.MenuDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.MenuDef.Get(ctx, id)
	return one(res, err, dao.ToMenuDefEntity, "menu with id %d", id)
}

// MenuByMachineName - the menu named machineName; ErrEntityNotFound if
// there is none.
func (r MenuRepository) MenuByMachineName(ctx context.Context, machineName string) (*entity.MenuDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.MenuDef.Query().Where(menudef.MachineName(machineName)).Only(ctx)
	return one(res, err, dao.ToMenuDefEntity, "menu with machine_name %q", machineName)
}

// CreateMenu adds the menu m and returns it with its ID.
func (r MenuRepository) CreateMenu(ctx context.Context, m entity.MenuDef) (*entity.MenuDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	v, err := client.MenuDef.Create().
		SetMachineName(m.MachineName).
		SetLabel(m.Label).
		SetDescription(m.Description).
		SetInSitemap(m.InSitemap).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	created := dao.ToMenuDefEntity(v)
	return &created, nil
}

// UpdateMenu never touches MachineName - identity strings are not
// renamed (same as TextFormatDef/ImageStyleDef).
func (r MenuRepository) UpdateMenu(ctx context.Context, m entity.MenuDef) (*entity.MenuDef, error) {
	client := GetClient(ctx, r.client)
	v, err := client.MenuDef.UpdateOneID(m.ID).
		SetLabel(m.Label).
		SetDescription(m.Description).
		SetInSitemap(m.InSitemap).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, notFoundAs(err, "menu with id %d", m.ID)
	}

	updated := dao.ToMenuDefEntity(v)
	return &updated, nil
}

// DeleteMenu removes the menu and all its links. This is an
// application-level cascade (fk_menu_links_menu has no ON DELETE CASCADE),
// so it runs two statements on the same tx-carrying ctx.
func (r MenuRepository) DeleteMenu(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	if _, err := client.MenuLinkDef.Delete().Where(menulinkdef.MenuID(id)).Exec(ctx); err != nil {
		return fmt.Errorf("error deleting menu links: %w", err)
	}
	return notFoundAs(client.MenuDef.DeleteOneID(id).Exec(ctx), "menu with id %d", id)
}

// MenuLinksByMenuID - the menu's links, all levels in one flat list, by
// weight, then ID.
func (r MenuRepository) MenuLinksByMenuID(ctx context.Context, menuID int) (entity.MenuLinkDefs, error) {
	client := GetClient(ctx, r.client)
	res, err := client.MenuLinkDef.Query().
		Where(menulinkdef.MenuID(menuID)).
		Order(generated.Asc(menulinkdef.FieldWeight), generated.Asc(menulinkdef.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("MenuRepository.MenuLinksByMenuID: %w", err)
	}

	links := make(entity.MenuLinkDefs, len(res))
	for i, v := range res {
		links[i] = dao.ToMenuLinkDefEntity(v)
	}
	return links, nil
}

// MenuLinkByID - the menu link with id; ErrEntityNotFound if there is
// none.
func (r MenuRepository) MenuLinkByID(ctx context.Context, id int) (*entity.MenuLinkDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.MenuLinkDef.Get(ctx, id)
	return one(res, err, dao.ToMenuLinkDefEntity, "menu link with id %d", id)
}

// CreateMenuLink adds the menu link l and returns it with its ID.
func (r MenuRepository) CreateMenuLink(ctx context.Context, l entity.MenuLinkDef) (*entity.MenuLinkDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.MenuLinkDef.Create().
		SetMenuID(l.MenuID).
		SetTitle(l.Title).
		SetLinkType(string(l.LinkType)).
		SetLinkURL(l.LinkURL).
		SetNillableLinkNodeID(l.LinkNodeID).
		SetNillableLinkTermID(l.LinkTermID).
		SetNillableLinkViewID(l.LinkViewID).
		SetWeight(l.Weight).
		SetEnabled(l.Enabled).
		SetNillableParentID(l.ParentID).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	v, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("MenuRepository.CreateMenuLink: %w", err)
	}

	created := dao.ToMenuLinkDefEntity(v)
	return &created, nil
}

// UpdateMenuLink saves l over the link l.ID (its menu stays) and returns
// it; ErrEntityNotFound if there is none.
func (r MenuRepository) UpdateMenuLink(ctx context.Context, l entity.MenuLinkDef) (*entity.MenuLinkDef, error) {
	client := GetClient(ctx, r.client)
	update := client.MenuLinkDef.UpdateOneID(l.ID).
		SetTitle(l.Title).
		SetLinkType(string(l.LinkType)).
		SetLinkURL(l.LinkURL).
		SetWeight(l.Weight).
		SetEnabled(l.Enabled).
		SetUpdatedAt(time.Now())

	if l.LinkNodeID != nil {
		update.SetLinkNodeID(*l.LinkNodeID)
	} else {
		update.ClearLinkNodeID()
	}
	if l.LinkTermID != nil {
		update.SetLinkTermID(*l.LinkTermID)
	} else {
		update.ClearLinkTermID()
	}
	if l.LinkViewID != nil {
		update.SetLinkViewID(*l.LinkViewID)
	} else {
		update.ClearLinkViewID()
	}
	if l.ParentID != nil {
		update.SetParentID(*l.ParentID)
	} else {
		update.ClearParentID()
	}

	v, err := update.Save(ctx)
	return one(v, err, dao.ToMenuLinkDefEntity, "menu link with id %d", l.ID)
}

// DeleteMenuLink moves the deleted link's children up one level (to its
// ParentID) instead of relying on the FK's ON DELETE SET NULL, which would
// promote them all the way to the menu's top level.
func (r MenuRepository) DeleteMenuLink(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)

	link, err := client.MenuLinkDef.Get(ctx, id)
	if err != nil {
		return notFoundAs(err, "menu link with id %d", id)
	}

	children, err := client.MenuLinkDef.Query().Where(menulinkdef.ParentID(id)).All(ctx)
	if err != nil {
		return fmt.Errorf("error listing menu link children: %w", err)
	}
	for _, child := range children {
		update := child.Update()
		if link.ParentID != nil {
			update = update.SetParentID(*link.ParentID)
		} else {
			update = update.ClearParentID()
		}
		if _, err := update.Save(ctx); err != nil {
			return fmt.Errorf("error reparenting menu link %d: %w", child.ID, err)
		}
	}

	if err := client.MenuLinkDef.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("error deleting menu link: %w", err)
	}
	return nil
}

// ReorderMenuLinks bulk-applies Weight/ParentID from the admin
// drag-and-drop tree (one UpdateOneID call per row).
func (r MenuRepository) ReorderMenuLinks(ctx context.Context, updates []entity.MenuLinkOrderUpdate) error {
	client := GetClient(ctx, r.client)
	for _, u := range updates {
		update := client.MenuLinkDef.UpdateOneID(u.ID).SetWeight(u.Weight)
		if u.ParentID != nil {
			update = update.SetParentID(*u.ParentID)
		} else {
			update = update.ClearParentID()
		}
		if err := update.Exec(ctx); err != nil {
			if generated.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("error reordering menu link %d: %w", u.ID, err)
		}
	}
	return nil
}
