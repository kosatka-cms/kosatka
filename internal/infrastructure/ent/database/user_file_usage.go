package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// UserFileUsageRepository implements entity.UserFileUsageRepository. The
// text scan reads many tables generically, so it stays hand-written SQL
// (portable; placeholders through rebind).
type UserFileUsageRepository struct {
	db *sql.DB
}

var _ entity.UserFileUsageRepository = (*UserFileUsageRepository)(nil)

// NewUserFileUsageRepository - the user file usage repository on db.
func NewUserFileUsageRepository(db *sql.DB) *UserFileUsageRepository {
	return &UserFileUsageRepository{db: db}
}

// userFileTextSource - a table column (or columns) holding HTML that can
// link to users' files; id is the entity the usage is recorded against.
type userFileTextSource struct {
	source  string
	table   string
	id      string
	columns []string
}

// coreFileTextSources - the core's texts; the modules' come from the
// registry (userFileTextSources).
var coreFileTextSources = []userFileTextSource{
	{entity.UserFileSourceNodeField, "field_value_text", "node_id", []string{"value"}},
	{entity.UserFileSourceComment, "comments", "id", []string{"content"}},
	{entity.UserFileSourceSignature, "users", "id", []string{"signature"}},
	{entity.UserFileSourceUserField, "user_field_values", "user_id", []string{"value"}},
	{entity.UserFileSourceBlock, "blocks", "id", []string{"body"}},
	{entity.UserFileSourceView, "view_displays", "id", []string{"header_text", "footer_text"}},
}

// userFileTextSources - the core's texts, then the registered modules'
// (entity.Module.FileSources).
func userFileTextSources() []userFileTextSource {
	out := append([]userFileTextSource(nil), coreFileTextSources...)
	for _, m := range entity.Modules {
		for _, s := range m.FileSources {
			out = append(out, userFileTextSource{s.Source, s.Table, s.IDColumn, s.Columns})
		}
	}
	return out
}

// ScanTextsFor - see entity.UserFileUsageRepository.ScanTextsFor; path is
// relative to the public files' URL.
func (r *UserFileUsageRepository) ScanTextsFor(ctx context.Context, path string, fn func(string, int, string)) error {
	return r.scan(ctx, "%"+escapeLike(paths.PublicFilesURL()+path)+"%", fn)
}

func (r *UserFileUsageRepository) scan(ctx context.Context, like string, fn func(string, int, string)) error {
	for _, src := range userFileTextSources() {
		// A never-installed module's table holds no texts.
		if ok, err := tableInstalled(ctx, r.db, src.table); err != nil || !ok {
			if err != nil {
				return fmt.Errorf("%s: %w", src.table, err)
			}
			continue
		}
		conds := make([]string, len(src.columns))
		args := make([]any, len(src.columns))
		for i, c := range src.columns {
			conds[i] = c + " LIKE ?"
			args[i] = like
		}
		q := fmt.Sprintf("SELECT %s, CONCAT_WS(' ', %s) FROM %s WHERE %s",
			src.id, strings.Join(src.columns, ", "), src.table, strings.Join(conds, " OR "))
		if err := r.scanQuery(ctx, rebind(q), args, func(id int, text string) { fn(src.source, id, text) }); err != nil {
			return fmt.Errorf("%s: %w", src.table, err)
		}
	}
	return nil
}

// Describe gives each usage the title of its text's owner and a link.
func (r *UserFileUsageRepository) Describe(ctx context.Context, usages []entity.UserFileUsage) ([]entity.UserFileRef, error) {
	bySource := map[string][]int{}
	for _, u := range usages {
		bySource[u.Source] = append(bySource[u.Source], u.EntityID)
	}
	type info struct{ title, url string }
	infos := map[string]map[int]info{}
	query := func(source, q string, ids []int, build func(rows *sql.Rows) (int, info, error)) error {
		if len(ids) == 0 {
			return nil
		}
		ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		rows, err := r.db.QueryContext(ctx, rebind(strings.Replace(q, "(?)", "("+ph+")", 1)), args...)
		if err != nil {
			return fmt.Errorf("UserFileUsageRepository.Describe: %w", err)
		}
		defer func() { _ = rows.Close() }()
		infos[source] = map[int]info{}
		for rows.Next() {
			id, in, err := build(rows)
			if err != nil {
				return err
			}
			infos[source][id] = in
		}
		return rows.Err()
	}
	nodeInfo := func(rows *sql.Rows) (int, info, error) {
		var id int
		var title, typ, slug string
		err := rows.Scan(&id, &title, &typ, &slug)
		return id, info{title, "/" + entity.NodeRelPath(entity.ContentType(typ), slug)}, err
	}
	userInfo := func(rows *sql.Rows) (int, info, error) {
		var id int
		var name, slug string
		err := rows.Scan(&id, &name, &slug)
		return id, info{name, "/" + strings.Trim(slug, "/")}, err
	}
	nodeSources := []string{entity.UserFileSourceNodeField}
	for _, m := range entity.Modules {
		for _, s := range m.FileSources {
			if s.NodeBacked {
				nodeSources = append(nodeSources, s.Source)
			}
		}
	}
	for _, src := range nodeSources {
		if err := query(src, `SELECT id, COALESCE(title, ''), type, COALESCE(slug, '') FROM nodes WHERE id IN (?)`, bySource[src], nodeInfo); err != nil {
			return nil, err
		}
	}
	for _, src := range []string{entity.UserFileSourceSignature, entity.UserFileSourceUserField} {
		if err := query(src, `SELECT id, username, COALESCE(slug, '') FROM users WHERE id IN (?)`, bySource[src], userInfo); err != nil {
			return nil, err
		}
	}
	if err := query(entity.UserFileSourceComment, `SELECT c.id, COALESCE(n.title, '') FROM comments c JOIN nodes n ON n.id = c.nid WHERE c.id IN (?)`, bySource[entity.UserFileSourceComment], func(rows *sql.Rows) (int, info, error) {
		var id int
		var title string
		err := rows.Scan(&id, &title)
		return id, info{title, "/comment/" + strconv.Itoa(id)}, err
	}); err != nil {
		return nil, err
	}
	if err := query(entity.UserFileSourceBlock, `SELECT id, title FROM blocks WHERE id IN (?)`, bySource[entity.UserFileSourceBlock], func(rows *sql.Rows) (int, info, error) {
		var id int
		var title string
		err := rows.Scan(&id, &title)
		return id, info{title, "/admin/config/blocks/" + strconv.Itoa(id) + "/edit"}, err
	}); err != nil {
		return nil, err
	}
	if err := query(entity.UserFileSourceView, `SELECT d.id, v.label, v.id FROM view_displays d JOIN views v ON v.id = d.view_id WHERE d.id IN (?)`, bySource[entity.UserFileSourceView], func(rows *sql.Rows) (int, info, error) {
		var id, viewID int
		var label string
		err := rows.Scan(&id, &label, &viewID)
		return id, info{label, "/admin/config/views/" + strconv.Itoa(viewID)}, err
	}); err != nil {
		return nil, err
	}
	// The modules' own texts (not a node's): the start of their title
	// column and their edit page - when the module's table is installed
	// and the text has a title (a private one has neither).
	for _, m := range entity.Modules {
		for _, s := range m.FileSources {
			if s.NodeBacked || s.TitleColumn == "" || len(bySource[s.Source]) == 0 {
				continue
			}
			ok, err := tableInstalled(ctx, r.db, s.Table)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			editPath := s.EditPath
			q := fmt.Sprintf(`SELECT %s, LEFT(%s, 80) FROM %s WHERE %s IN (?)`, s.IDColumn, s.TitleColumn, s.Table, s.IDColumn)
			if err := query(s.Source, q, bySource[s.Source], func(rows *sql.Rows) (int, info, error) {
				var id int
				var text string
				err := rows.Scan(&id, &text)
				return id, info{text, editPath + strconv.Itoa(id)}, err
			}); err != nil {
				return nil, err
			}
		}
	}

	out := make([]entity.UserFileRef, 0, len(usages))
	for _, u := range usages {
		in := infos[u.Source][u.EntityID]
		out = append(out, entity.UserFileRef{Source: u.Source, EntityID: u.EntityID, Title: in.title, URL: in.url})
	}
	return out, nil
}

// escapeLike escapes LIKE wildcards in s (the default escape "\" of MySQL
// and PostgreSQL).
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// scanQuery runs q and hands each row's id and text to fn.
func (r *UserFileUsageRepository) scanQuery(ctx context.Context, q string, args []any, fn func(int, string)) error {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("UserFileUsageRepository.scanQuery: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int
		var text sql.NullString
		if err := rows.Scan(&id, &text); err != nil {
			return fmt.Errorf("UserFileUsageRepository.scanQuery: %w", err)
		}
		fn(id, text.String)
	}
	return rows.Err()
}
