package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockplacementdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/menulinkdef"
)

var _ entity.ModuleRepository = (*ModuleRepository)(nil)

// ModuleRepository implements entity.ModuleRepository.
type ModuleRepository struct {
	db     *sql.DB
	client *generated.Client
}

// NewModuleRepository - the module repository on db.
func NewModuleRepository(db *sql.DB) *ModuleRepository {
	return &ModuleRepository{db: db, client: clientFor(db)}
}

// States - module name → enabled, for every module with a row.
func (r *ModuleRepository) States(ctx context.Context) (map[string]bool, error) {
	rows, err := GetClient(ctx, r.client).Module.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ModuleRepository.States: %w", err)
	}
	states := make(map[string]bool, len(rows))
	for _, m := range rows {
		states[m.ID] = m.Enabled
	}
	return states, nil
}

// SetEnabled stores the module's state.
func (r *ModuleRepository) SetEnabled(ctx context.Context, name string, enabled bool) error {
	client := GetClient(ctx, r.client)
	now := time.Now()
	err := client.Module.UpdateOneID(name).SetEnabled(enabled).SetUpdatedAt(now).Exec(ctx)
	if !generated.IsNotFound(err) {
		return err
	}
	return client.Module.Create().SetID(name).SetEnabled(enabled).SetUpdatedAt(now).Exec(ctx)
}

// moduleTableName - the table names CountRows accepts (they come from
// entity.Module.Data, never from a request).
var moduleTableName = regexp.MustCompile(`^[a-z_]+$`)

// CountRows - rows in a module's data table (any table, so the query is
// built for it, quoted for the database).
func (r *ModuleRepository) CountRows(ctx context.Context, table string) (int, error) {
	if !moduleTableName.MatchString(table) {
		return 0, fmt.Errorf("invalid table name %q", table)
	}
	// A module never installed has no tables, so no rows.
	if ok, err := tableInstalled(ctx, r.db, table); err != nil || !ok {
		return 0, err
	}
	query, args := entsql.Dialect(entDialect).Select(entsql.Count("*")).From(entsql.Table(table)).Query()
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

// DeletePluginPlacements removes the plugin blocks' placements, every
// theme.
func (r *ModuleRepository) DeletePluginPlacements(ctx context.Context, ids []entity.PluginBlockID) error {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = string(id)
	}
	_, err := GetClient(ctx, r.client).BlockPlacementDef.Delete().
		Where(blockplacementdef.PlacementType(string(entity.BlockPlacementPlugin)), blockplacementdef.PluginIDIn(names...)).
		Exec(ctx)
	return err
}

// DeleteMenuLinks - see entity.ModuleRepository.DeleteMenuLinks.
func (r *ModuleRepository) DeleteMenuLinks(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	return inTx(ctx, r.client, func(c *generated.Client) error {
		links, err := c.MenuLinkDef.Query().Where(menulinkdef.LinkType(string(entity.MenuLinkURL))).All(ctx)
		if err != nil {
			return fmt.Errorf("the menu links: %w", err)
		}
		for _, l := range links {
			if !underPaths(l.LinkURL, paths) {
				continue
			}
			// Its children move up to its place, then it goes.
			children := c.MenuLinkDef.Update().Where(menulinkdef.ParentID(l.ID))
			if l.ParentID != nil {
				children.SetParentID(*l.ParentID)
			} else {
				children.ClearParentID()
			}
			if err := children.Exec(ctx); err != nil {
				return fmt.Errorf("the children of menu link %d: %w", l.ID, err)
			}
			if err := c.MenuLinkDef.DeleteOneID(l.ID).Exec(ctx); err != nil {
				return fmt.Errorf("deleting menu link %d: %w", l.ID, err)
			}
		}
		return nil
	})
}

// underPaths - whether url is one of paths or under one ("/quotes",
// "/quotes/byauthor", "/quotes?page=2").
func underPaths(url string, paths []string) bool {
	for _, p := range paths {
		p = strings.TrimSuffix(p, "/")
		if url == p || strings.HasPrefix(url, p+"/") || strings.HasPrefix(url, p+"?") || strings.HasPrefix(url, p+"#") {
			return true
		}
	}
	return false
}

// PluginPlacements - where the plugin blocks are placed, every theme.
func (r *ModuleRepository) PluginPlacements(ctx context.Context, ids []entity.PluginBlockID) ([]entity.PluginPlacementRef, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = string(id)
	}
	rows, err := GetClient(ctx, r.client).BlockPlacementDef.Query().
		Where(blockplacementdef.PlacementType(string(entity.BlockPlacementPlugin)), blockplacementdef.PluginIDIn(names...)).
		Order(generated.Asc(blockplacementdef.FieldTheme), generated.Asc(blockplacementdef.FieldRegion), generated.Asc(blockplacementdef.FieldWeight)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ModuleRepository.PluginPlacements: %w", err)
	}
	out := make([]entity.PluginPlacementRef, 0, len(rows))
	for _, p := range rows {
		ref := entity.PluginPlacementRef{Theme: p.Theme, Region: entity.Region(p.Region)}
		if p.PluginID != nil {
			ref.PluginID = entity.PluginBlockID(*p.PluginID)
		}
		out = append(out, ref)
	}
	return out, nil
}
