package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/file"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/user"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
)

// MediaRepository implements entity.MediaRepository: the files table
// with the places that use its files - the file and image fields, the
// types' default images and the modules' tables of files
// (migrations.FileColumns; entity.Module.NodeFiles name their content).
type MediaRepository struct {
	db     *sql.DB
	client *generated.Client
}

// NewMediaRepository - the media repository on db.
func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db, client: clientFor(db)}
}

// fileColumn - a table's column of file IDs.
type fileColumn struct{ table, column string }

// usingColumns - every column that uses a file: the fields', the
// default images', the installed modules' (the module's name beside).
func (r *MediaRepository) usingColumns(ctx context.Context) ([]fileColumn, map[fileColumn]string, error) {
	cols := []fileColumn{{"field_value_file", "file_id"}, {"field_value_file_multi", "file_id"}, {"field_instances", "default_image_id"}}
	modules := map[fileColumn]string{}
	for _, m := range entity.Modules {
		for _, c := range migrations.FileColumns(m.Name) {
			ok, err := tableInstalled(ctx, r.db, c.Table)
			if err != nil {
				return nil, nil, err
			}
			if ok {
				fc := fileColumn{c.Table, c.Column}
				cols = append(cols, fc)
				modules[fc] = m.Name
			}
		}
	}
	return cols, modules, nil
}

// MediaFiles - see entity.MediaRepository.MediaFiles.
func (r *MediaRepository) MediaFiles(ctx context.Context, f entity.MediaFilter) ([]entity.File, int, error) {
	var preds []predicate.File
	if q := strings.TrimSpace(f.Search); q != "" {
		preds = append(preds, file.Or(file.URIContainsFold(q), file.OriginalFilenameContainsFold(q), file.AltContainsFold(q)))
	}
	switch f.Kind {
	case entity.MediaImages:
		preds = append(preds, file.MimeTypeHasPrefix("image/"))
	case entity.MediaDocuments:
		preds = append(preds, file.Or(file.MimeTypeIsNil(), file.Not(file.MimeTypeHasPrefix("image/"))))
	}
	if f.CreatedBy > 0 {
		preds = append(preds, file.CreatedBy(f.CreatedBy))
	}
	if f.Scheme != "" {
		preds = append(preds, file.URIHasPrefix(f.Scheme))
	}
	if len(f.Extensions) > 0 {
		var ext []predicate.File
		for _, e := range f.Extensions {
			ext = append(ext, file.URIHasSuffix("."+strings.ToLower(e)), file.URIHasSuffix("."+strings.ToUpper(e)))
		}
		preds = append(preds, file.Or(ext...))
	}
	if f.MinWidth > 0 {
		preds = append(preds, file.WidthGTE(f.MinWidth))
	}
	if f.MinHeight > 0 {
		preds = append(preds, file.HeightGTE(f.MinHeight))
	}
	if f.MaxBytes > 0 {
		preds = append(preds, file.SizeBytesLTE(f.MaxBytes))
	}
	if len(f.IDs) > 0 {
		preds = append(preds, file.IDIn(f.IDs...))
	}
	if f.Unused {
		cols, _, err := r.usingColumns(ctx)
		if err != nil {
			return nil, 0, err
		}
		preds = append(preds, func(s *entsql.Selector) {
			for _, c := range cols {
				s.Where(entsql.ExprP(fmt.Sprintf("%s NOT IN (SELECT %s FROM %s WHERE %s IS NOT NULL)", s.C(file.FieldID), c.column, c.table, c.column)))
			}
		})
	}
	q := GetClient(ctx, r.client).File.Query().Where(preds...)
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting the media files: %w", err)
	}
	rows, err := q.WithUsers().Order(generated.Desc(file.FieldCreatedAt), generated.Desc(file.FieldID)).
		Offset(f.Offset).Limit(f.Limit).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing the media files: %w", err)
	}
	out := make([]entity.File, len(rows))
	for i, row := range rows {
		out[i] = dao.ToFileEntity(row)
	}
	return out, total, nil
}

// MediaUseCounts - see entity.MediaRepository.MediaUseCounts.
func (r *MediaRepository) MediaUseCounts(ctx context.Context, fileIDs []int) (map[int]int, error) {
	counts := map[int]int{}
	if len(fileIDs) == 0 {
		return counts, nil
	}
	cols, _, err := r.usingColumns(ctx)
	if err != nil {
		return nil, err
	}
	in, args := inList(fileIDs)
	for _, c := range cols {
		if err := r.countUses(ctx, c, in, args, counts); err != nil {
			return nil, err
		}
	}
	return counts, nil
}

// countUses adds c's rows using each of the files (in, args) to counts.
func (r *MediaRepository) countUses(ctx context.Context, c fileColumn, in string, args []any, counts map[int]int) error {
	rows, err := r.db.QueryContext(ctx, rebind("SELECT "+c.column+", COUNT(*) FROM "+c.table+" WHERE "+c.column+" IN "+in+" GROUP BY "+c.column), args...)
	if err != nil {
		return fmt.Errorf("counting the uses of files in %s: %w", c.table, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, n int
		if err := rows.Scan(&id, &n); err != nil {
			return fmt.Errorf("counting the uses of files in %s: %w", c.table, err)
		}
		counts[id] += n
	}
	return rows.Err()
}

// fieldUsesQuery - a field table's uses of a file: the content item, its
// type's field's label (none if gone).
const fieldUsesQuery = `SELECT n.id, n.title, n.type, n.slug, n.deleted_at IS NOT NULL, COALESCE(fi.label, '')
FROM %s v JOIN nodes n ON n.id = v.node_id
LEFT JOIN content_types ct ON ct.machine_name = n.type
LEFT JOIN field_instances fi ON fi.content_type_id = ct.id AND fi.field_storage_id = v.field_id
WHERE v.file_id = ? ORDER BY n.id`

// MediaUses - see entity.MediaRepository.MediaUses.
func (r *MediaRepository) MediaUses(ctx context.Context, fileID int) ([]entity.MediaUse, error) {
	cols, modules, err := r.usingColumns(ctx)
	if err != nil {
		return nil, err
	}
	var out []entity.MediaUse
	for _, table := range []string{"field_value_file", "field_value_file_multi"} {
		uses, err := r.nodeUses(ctx, fmt.Sprintf(fieldUsesQuery, table), fileID, "")
		if err != nil {
			return nil, err
		}
		out = append(out, uses...)
	}
	defaults, err := r.defaultImageUses(ctx, fileID)
	if err != nil {
		return nil, err
	}
	out = append(out, defaults...)
	for _, c := range cols {
		module, ok := modules[c]
		if !ok {
			continue
		}
		uses, err := r.moduleUses(ctx, c, module, fileID)
		if err != nil {
			return nil, err
		}
		out = append(out, uses...)
	}
	return out, nil
}

// nodeUses - the content items query (fieldUsesQuery's columns) finds
// for the file, each with module set.
func (r *MediaRepository) nodeUses(ctx context.Context, query string, fileID int, module string) ([]entity.MediaUse, error) {
	rows, err := r.db.QueryContext(ctx, rebind(query), fileID)
	if err != nil {
		return nil, fmt.Errorf("finding the uses of file %d: %w", fileID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []entity.MediaUse
	for rows.Next() {
		u := entity.MediaUse{Module: module}
		if err := rows.Scan(&u.NodeID, &u.Title, &u.Type, &u.Slug, &u.InTrash, &u.Field); err != nil {
			return nil, fmt.Errorf("finding the uses of file %d: %w", fileID, err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// defaultImageUses - the content types' image fields with the file as
// their default image.
func (r *MediaRepository) defaultImageUses(ctx context.Context, fileID int) ([]entity.MediaUse, error) {
	rows, err := r.db.QueryContext(ctx, rebind(`SELECT fi.label, ct.label FROM field_instances fi
JOIN content_types ct ON ct.id = fi.content_type_id WHERE fi.default_image_id = ? ORDER BY ct.label, fi.label`), fileID)
	if err != nil {
		return nil, fmt.Errorf("finding the default images of file %d: %w", fileID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []entity.MediaUse
	for rows.Next() {
		var u entity.MediaUse
		if err := rows.Scan(&u.Field, &u.TypeLabel); err != nil {
			return nil, fmt.Errorf("finding the default images of file %d: %w", fileID, err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// moduleUses - module's rows of c using the file: with the content item
// its NodeFiles attach them to, else one use per row without it.
func (r *MediaRepository) moduleUses(ctx context.Context, c fileColumn, module string, fileID int) ([]entity.MediaUse, error) {
	if m, ok := entity.ModuleByName(module); ok {
		for _, nf := range m.NodeFiles {
			if nf.Table != c.table || nf.FileColumn != c.column {
				continue
			}
			join := "JOIN nodes n ON n.id = t." + nf.NodeColumn
			if nf.Via != "" {
				join = "JOIN " + nf.Via + " v ON v.id = t." + nf.ViaColumn + " JOIN nodes n ON n.id = v." + nf.NodeColumn
			}
			return r.nodeUses(ctx, "SELECT n.id, n.title, n.type, n.slug, n.deleted_at IS NOT NULL, '' FROM "+c.table+" t "+join+
				" WHERE t."+c.column+" = ? ORDER BY n.id", fileID, module)
		}
	}
	var n int
	if err := r.db.QueryRowContext(ctx, rebind("SELECT COUNT(*) FROM "+c.table+" WHERE "+c.column+" = ?"), fileID).Scan(&n); err != nil {
		return nil, fmt.Errorf("finding the uses of file %d in %s: %w", fileID, c.table, err)
	}
	out := make([]entity.MediaUse, n)
	for i := range out {
		out[i].Module = module
	}
	return out, nil
}

// MediaUploaders - see entity.MediaRepository.MediaUploaders.
func (r *MediaRepository) MediaUploaders(ctx context.Context) ([]entity.User, error) {
	client := GetClient(ctx, r.client)
	ids, err := client.File.Query().Where(file.CreatedByNotNil()).Unique(true).Select(file.FieldCreatedBy).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the uploaders: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	users, err := client.User.Query().Where(user.IDIn(ids...)).Order(generated.Asc(user.FieldUsername)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the uploaders: %w", err)
	}
	out := make([]entity.User, len(users))
	for i, u := range users {
		out[i] = dao.ToUserEntity(u)
	}
	return out, nil
}

// MediaTotals - how many files there are and their size in all.
func (r *MediaRepository) MediaTotals(ctx context.Context) (entity.MediaTotals, error) {
	var t entity.MediaTotals
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM files").Scan(&t.Count, &t.Bytes)
	if err != nil {
		return t, fmt.Errorf("counting the media files: %w", err)
	}
	return t, nil
}
