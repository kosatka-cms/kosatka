package database

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockplacementdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/breadcrumbruledef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldstoragedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomy"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomymulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/searchtype"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/seorule"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/taxonomytermdata"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewfielddef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewfilterdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewsortdef"
)

// ModuleUninstallRepository - the core tables' side of a module's
// uninstall (service.ModuleUninstall): its content, the settings of its
// content types, the files only its tables use, a vocabulary it owns.
type ModuleUninstallRepository struct {
	db     *sql.DB
	client *generated.Client
}

// NewModuleUninstallRepository creates a ModuleUninstallRepository.
func NewModuleUninstallRepository(db *sql.DB) *ModuleUninstallRepository {
	return &ModuleUninstallRepository{db: db, client: clientFor(db)}
}

// NodeIDsOfTypes - up to limit IDs of nodes of the types, the trash's
// too.
func (r *ModuleUninstallRepository) NodeIDsOfTypes(ctx context.Context, types []string, limit int) ([]int, error) {
	if len(types) == 0 {
		return nil, nil
	}
	return GetClient(ctx, r.client).Node.Query().Where(node.TypeIn(types...)).Order(generated.Asc(node.FieldID)).Limit(limit).IDs(everyNode(ctx))
}

// FileIDs - the files the columns reference (each a table and a column of
// a module's tables, from its schema); none while the module isn't
// installed.
func (r *ModuleUninstallRepository) FileIDs(ctx context.Context, columns [][2]string) ([]int, error) {
	var out []int
	for _, tc := range columns {
		table, column := tc[0], tc[1]
		if !moduleTableName.MatchString(table) || !moduleTableName.MatchString(column) {
			return nil, fmt.Errorf("invalid column %s.%s", table, column)
		}
		if ok, err := tableInstalled(ctx, r.db, table); err != nil || !ok {
			if err != nil {
				return nil, err
			}
			continue
		}
		ids, err := r.columnIDs(ctx, table, column)
		if err != nil {
			return nil, err
		}
		out = append(out, ids...)
	}
	return distinct(out), nil
}

// columnIDs - the distinct non-null ids in table.column.
func (r *ModuleUninstallRepository) columnIDs(ctx context.Context, table, column string) ([]int, error) {
	query, args := entsql.Dialect(entDialect).Select(column).Distinct().From(entsql.Table(table)).Where(entsql.NotNull(column)).Query()
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ModuleUninstallRepository.columnIDs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("ModuleUninstallRepository.columnIDs: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// DeleteNodeTypeSettings removes what core settings keep for the content
// types: their search, SEO and breadcrumb rules; a block shown only on
// them loses them from its visibility, and is deleted when none is left
// (it showed on their pages only).
func (r *ModuleUninstallRepository) DeleteNodeTypeSettings(ctx context.Context, types []string) error {
	if len(types) == 0 {
		return nil
	}
	return inTx(ctx, r.client, func(c *generated.Client) error {
		if _, err := c.SearchType.Delete().Where(searchtype.IDIn(types...)).Exec(ctx); err != nil {
			return fmt.Errorf("ModuleUninstallRepository.DeleteNodeTypeSettings: %w", err)
		}
		if _, err := c.SeoRule.Delete().Where(seorule.ContentTypeIn(types...)).Exec(ctx); err != nil {
			return fmt.Errorf("ModuleUninstallRepository.DeleteNodeTypeSettings: %w", err)
		}
		if _, err := c.BreadcrumbRuleDef.Delete().Where(breadcrumbruledef.ContentTypeIn(types...)).Exec(ctx); err != nil {
			return fmt.Errorf("ModuleUninstallRepository.DeleteNodeTypeSettings: %w", err)
		}
		placements, err := c.BlockPlacementDef.Query().
			Where(blockplacementdef.VisibilityScope(string(entity.BlockVisibilityContentType)), blockplacementdef.VisibilityContentTypeNotNil()).
			All(ctx)
		if err != nil {
			return fmt.Errorf("ModuleUninstallRepository.DeleteNodeTypeSettings: %w", err)
		}
		for _, p := range placements {
			var keep []string
			for _, t := range strings.Split(*p.VisibilityContentType, ",") {
				if t = strings.TrimSpace(t); t != "" && !slices.Contains(types, t) {
					keep = append(keep, t)
				}
			}
			switch {
			case len(keep) == 0:
				err = c.BlockPlacementDef.DeleteOneID(p.ID).Exec(ctx)
			case strings.Join(keep, ",") != *p.VisibilityContentType:
				err = c.BlockPlacementDef.UpdateOneID(p.ID).SetVisibilityContentType(strings.Join(keep, ",")).Exec(ctx)
			}
			if err != nil {
				return fmt.Errorf("ModuleUninstallRepository.DeleteNodeTypeSettings: %w", err)
			}
		}
		return nil
	})
}

// DeleteViewCriteria removes the Views' sort criteria, filters and fields
// whose key (builtin_field) starts with prefix: a module's.
func (r *ModuleUninstallRepository) DeleteViewCriteria(ctx context.Context, prefix string) error {
	if prefix == "" {
		return nil
	}
	return inTx(ctx, r.client, func(c *generated.Client) error {
		if _, err := c.ViewSortDef.Delete().Where(viewsortdef.BuiltinFieldHasPrefix(prefix)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the module's View sorts: %w", err)
		}
		if _, err := c.ViewFilterDef.Delete().Where(viewfilterdef.BuiltinFieldHasPrefix(prefix)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the module's View filters: %w", err)
		}
		if _, err := c.ViewFieldDef.Delete().Where(viewfielddef.BuiltinFieldHasPrefix(prefix)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the module's View fields: %w", err)
		}
		return nil
	})
}

// RemoveTextFilters takes the filters out of the text formats that have
// them on; returns those formats' filters left, by ID.
func (r *ModuleUninstallRepository) RemoveTextFilters(ctx context.Context, names []string) (map[int][]string, error) {
	c := GetClient(ctx, r.client)
	formats, err := c.TextFormatDef.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing text formats: %w", err)
	}
	left := map[int][]string{}
	for _, f := range formats {
		kept := slices.DeleteFunc(strings.Fields(f.Filters), func(n string) bool { return slices.Contains(names, n) })
		if len(kept) == len(strings.Fields(f.Filters)) {
			continue
		}
		if err := c.TextFormatDef.UpdateOneID(f.ID).SetFilters(strings.Join(kept, " ")).Exec(ctx); err != nil {
			return nil, fmt.Errorf("removing text filters (format %v): %w", f.ID, err)
		}
		left[f.ID] = kept
	}
	return left, nil
}

// ClearFieldFormatters makes the fields shown by these formatters - on
// the detail page (field_instances.detail_formatter) and in Views
// (view_fields.formatter) - show as links again.
func (r *ModuleUninstallRepository) ClearFieldFormatters(ctx context.Context, names []string) error {
	c := GetClient(ctx, r.client)
	if err := c.FieldInstanceDef.Update().Where(fieldinstancedef.DetailFormatterIn(names...)).SetDetailFormatter("").Exec(ctx); err != nil {
		return fmt.Errorf("clearing the field formatters of the content types: %w", err)
	}
	if err := c.ViewFieldDef.Update().Where(viewfielddef.FormatterIn(names...)).SetFormatter("").Exec(ctx); err != nil {
		return fmt.Errorf("clearing the field formatters of the Views: %w", err)
	}
	return nil
}

// DeleteVocabularyIfUnused deletes the vocabulary with its terms unless a
// taxonomy field, a search badge or a field value still uses it; reports
// whether it was deleted.
func (r *ModuleUninstallRepository) DeleteVocabularyIfUnused(ctx context.Context, vid int) (bool, error) {
	c := GetClient(ctx, r.client)
	storages, err := c.FieldStorageDef.Query().Where(fieldstoragedef.FieldTypeEQ(string(entity.FieldTypeTaxonomy))).All(ctx)
	if err != nil {
		return false, fmt.Errorf("ModuleUninstallRepository.DeleteVocabularyIfUnused: %w", err)
	}
	for _, s := range storages {
		if v, ok := s.Config["vocabulary_id"].(float64); ok && int(v) == vid {
			return false, nil
		}
	}
	if n, err := c.SearchType.Query().Where(searchtype.BadgeVocabularyID(vid)).Count(ctx); err != nil || n > 0 {
		return false, err
	}
	terms, err := c.TaxonomyTermData.Query().Where(taxonomytermdata.Vid(vid)).IDs(ctx)
	if err != nil {
		return false, fmt.Errorf("ModuleUninstallRepository.DeleteVocabularyIfUnused: %w", err)
	}
	if len(terms) > 0 {
		single, err := c.FieldValueTaxonomy.Query().Where(fieldvaluetaxonomy.TermIDIn(terms...)).Count(ctx)
		if err != nil || single > 0 {
			return false, err
		}
		multi, err := c.FieldValueTaxonomyMulti.Query().Where(fieldvaluetaxonomymulti.TermIDIn(terms...)).Count(ctx)
		if err != nil || multi > 0 {
			return false, err
		}
	}
	// The terms go with it (ON DELETE CASCADE), their node index too.
	if err := c.TaxonomyVocabulary.DeleteOneID(vid).Exec(ctx); err != nil && !generated.IsNotFound(err) {
		return false, err
	}
	return true, nil
}
