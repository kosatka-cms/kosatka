package database

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/comment"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefile"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefilemulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/file"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/schema"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
)

// UserContentRepository implements entity.UserContentRepository -
// cross-table "everything this user owns" queries that don't belong to any
// single entity's repository.
type UserContentRepository struct {
	db     *sql.DB
	client *generated.Client
}

// NewUserContentRepository creates a new UserContentRepository.
func NewUserContentRepository(db *sql.DB) *UserContentRepository {
	return &UserContentRepository{db: db, client: clientFor(db)}
}

// ModuleInstalled - whether the module was installed (its tables exist).
func (r *UserContentRepository) ModuleInstalled(ctx context.Context, module string) (bool, error) {
	return moduleInstalled(ctx, r.db, module)
}

// queryInts runs a hand-written query (? placeholders) of one int column.
func (r *UserContentRepository) queryInts(ctx context.Context, q string, args []any) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, rebind(q), args...)
	if err != nil {
		return nil, fmt.Errorf("UserContentRepository.queryInts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("UserContentRepository.queryInts: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// inList - "(?,?,…)" for ids and the ids as arguments.
func inList(ids []int) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return "(" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")", args
}

// moduleNodeFileIDs - the files the installed modules attach to the nodes
// (entity.Module.NodeFiles).
func (r *UserContentRepository) moduleNodeFileIDs(ctx context.Context, nodeIDs []int) ([]int, error) {
	in, args := inList(nodeIDs)
	var out []int
	for _, m := range entity.Modules {
		for _, nf := range m.NodeFiles {
			if ok, err := tableInstalled(ctx, r.db, nf.Table); err != nil || !ok {
				if err != nil {
					return nil, err
				}
				continue
			}
			q := "SELECT t." + nf.FileColumn + " FROM " + nf.Table + " t"
			node := "t." + nf.NodeColumn
			if nf.Via != "" {
				q += " JOIN " + nf.Via + " v ON v.id = t." + nf.ViaColumn
				node = "v." + nf.NodeColumn
			}
			ids, err := r.queryInts(ctx, q+" WHERE "+node+" IN "+in, args)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", nf.Table, err)
			}
			out = append(out, ids...)
		}
	}
	return out, nil
}

// moduleFileColumns - the columns of the installed modules' tables that
// reference files (their foreign keys to files, from the ent schema -
// the core's or a module's own: migrations.FileColumns).
func (r *UserContentRepository) moduleFileColumns(ctx context.Context) ([][2]string, error) {
	var out [][2]string
	for _, m := range entity.Modules {
		for _, c := range migrations.FileColumns(m.Name) {
			ok, err := tableInstalled(ctx, r.db, c.Table)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, [2]string{c.Table, c.Column})
			}
		}
	}
	return out, nil
}

// Summary - the user's nodes by type (the most first), comments and
// uploaded files, counted; the modules' content is the caller's.
func (r *UserContentRepository) Summary(ctx context.Context, userID int) (entity.UserContentSummary, error) {
	client := GetClient(ctx, r.client)
	var s entity.UserContentSummary
	var byType []struct {
		Type  string `json:"type"`
		Count int    `json:"count"`
	}
	err := client.Node.Query().Where(node.CreatedBy(userID)).
		GroupBy(node.FieldType).Aggregate(generated.Count()).
		Scan(ctx, &byType)
	if err != nil {
		return s, fmt.Errorf("UserContentRepository.Summary: %w", err)
	}
	sort.Slice(byType, func(i, j int) bool {
		if byType[i].Count != byType[j].Count {
			return byType[i].Count > byType[j].Count
		}
		return byType[i].Type < byType[j].Type
	})
	for _, c := range byType {
		s.NodesByType = append(s.NodesByType, entity.UserContentTypeCount{Type: c.Type, Count: c.Count})
		s.Nodes += c.Count
	}
	if s.Comments, err = client.Comment.Query().Where(comment.CreatedBy(userID)).Count(ctx); err != nil {
		return s, err
	}
	if s.Files, err = client.File.Query().Where(file.CreatedBy(userID)).Count(ctx); err != nil {
		return s, err
	}
	return s, nil
}

// NodeIDs - see entity.UserContentRepository.NodeIDs; by ID.
func (r *UserContentRepository) NodeIDs(ctx context.Context, userID int, limit int) ([]int, error) {
	return GetClient(ctx, r.client).Node.Query().
		Where(node.CreatedBy(userID)).
		Order(generated.Asc(node.FieldID)).
		Limit(limit).
		IDs(ctx)
}

// NodeFileIDs - the files of the nodes' file fields (their blocks'
// too: deleted with them) and the ones the modules attach to them.
func (r *UserContentRepository) NodeFileIDs(ctx context.Context, nodeIDs []int) ([]int, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	client := GetClient(ctx, r.client)
	items, err := client.Node.Query().Where(node.ParentIDIn(nodeIDs...)).IDs(schema.WithBlocks(ctx))
	if err != nil {
		return nil, fmt.Errorf("loading the blocks of nodes: %w", err)
	}
	nodeIDs = append(slices.Clone(nodeIDs), items...)
	moduleFiles, err := r.moduleNodeFileIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	fields, err := client.FieldValueFile.Query().
		Where(fieldvaluefile.NodeIDIn(nodeIDs...)).
		Select(fieldvaluefile.FieldFileID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserContentRepository.NodeFileIDs: %w", err)
	}
	multi, err := client.FieldValueFileMulti.Query().
		Where(fieldvaluefilemulti.NodeIDIn(nodeIDs...)).
		Select(fieldvaluefilemulti.FieldFileID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserContentRepository.NodeFileIDs: %w", err)
	}
	return distinct(append(append(moduleFiles, fields...), multi...)), nil
}

// NodesWithFile - the content items whose file fields hold the file: a
// blocks field's item counts as its node.
func (r *UserContentRepository) NodesWithFile(ctx context.Context, fileID int) ([]int, error) {
	client := GetClient(ctx, r.client)
	single, err := client.FieldValueFile.Query().
		Where(fieldvaluefile.FileID(fileID)).
		Select(fieldvaluefile.FieldNodeID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("the nodes with file %d: %w", fileID, err)
	}
	multi, err := client.FieldValueFileMulti.Query().
		Where(fieldvaluefilemulti.FileID(fileID)).
		Select(fieldvaluefilemulti.FieldNodeID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("the nodes with file %d: %w", fileID, err)
	}
	ids := distinct(append(single, multi...))
	if len(ids) == 0 {
		return nil, nil
	}
	nodes, err := client.Node.Query().Where(node.IDIn(ids...)).
		Select(node.FieldID, node.FieldParentID).All(schema.WithBlocks(ctx))
	if err != nil {
		return nil, fmt.Errorf("the nodes with file %d: %w", fileID, err)
	}
	out := make([]int, 0, len(nodes))
	for _, n := range nodes {
		if n.ParentID != nil && *n.ParentID != 0 {
			out = append(out, *n.ParentID)
		} else {
			out = append(out, n.ID)
		}
	}
	return distinct(out), nil
}

// FileIDsByCreator - see entity.UserContentRepository.FileIDsByCreator.
func (r *UserContentRepository) FileIDsByCreator(ctx context.Context, userID int) ([]int, error) {
	return GetClient(ctx, r.client).File.Query().Where(file.CreatedBy(userID)).IDs(ctx)
}

// DeleteUnreferencedFiles deletes those of the files no file field, default
// image or module table uses, and returns the URIs no remaining files row shares
// (safe to remove from disk).
func (r *UserContentRepository) DeleteUnreferencedFiles(ctx context.Context, fileIDs []int) ([]string, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	client := GetClient(ctx, r.client)
	files, err := client.File.Query().Where(file.IDIn(fileIDs...)).Select(file.FieldID, file.FieldURI).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("UserContentRepository.DeleteUnreferencedFiles: %w", err)
	}
	moduleColumns, err := r.moduleFileColumns(ctx)
	if err != nil {
		return nil, err
	}
	in, args := inList(fileIDs)
	used := map[int]bool{}
	queries := []func() ([]int, error){
		func() ([]int, error) {
			return client.FieldValueFile.Query().Where(fieldvaluefile.FileIDIn(fileIDs...)).Select(fieldvaluefile.FieldFileID).Ints(ctx)
		},
		func() ([]int, error) {
			return client.FieldValueFileMulti.Query().Where(fieldvaluefilemulti.FileIDIn(fileIDs...)).Select(fieldvaluefilemulti.FieldFileID).Ints(ctx)
		},
		func() ([]int, error) {
			return client.FieldInstanceDef.Query().Where(fieldinstancedef.DefaultImageIDIn(fileIDs...)).Select(fieldinstancedef.FieldDefaultImageID).Ints(ctx)
		},
	}
	for _, c := range moduleColumns {
		queries = append(queries, func() ([]int, error) {
			return r.queryInts(ctx, "SELECT "+c[1]+" FROM "+c[0]+" WHERE "+c[1]+" IN "+in, args)
		})
	}
	for _, q := range queries {
		ids, err := q()
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			used[id] = true
		}
	}
	var ids []int
	var uris []string
	for _, f := range files {
		if !used[f.ID] {
			ids = append(ids, f.ID)
			uris = append(uris, f.URI)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := client.File.Delete().Where(file.IDIn(ids...)).Exec(ctx); err != nil {
		return nil, fmt.Errorf("UserContentRepository.DeleteUnreferencedFiles: %w", err)
	}
	// Another remaining files row may share the same URI - keep such files
	// on disk.
	var removable []string
	for _, uri := range uris {
		n, err := client.File.Query().Where(file.URI(uri)).Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("UserContentRepository.DeleteUnreferencedFiles: %w", err)
		}
		if n == 0 {
			removable = append(removable, uri)
		}
	}
	return removable, nil
}

// distinct - ids without repeats, ascending.
func distinct(ids []int) []int {
	slices.Sort(ids)
	return slices.Compact(ids)
}
