package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// nodePathStore - the nodes a rebuild walks (implemented by the nodes
// repository).
type nodePathStore interface {
	PathBatch(ctx context.Context, nodeType string, afterID, limit int, onlyAuto bool) (entity.Nodes, error)
	CountPathNodes(ctx context.Context, nodeType string, onlyAuto bool) (int, error)
	SetPath(ctx context.Context, nodeID int, slug string, auto bool) error
}

// nodePathFieldValues - the taxonomy fields' terms of many nodes
// (implemented by ContentTypeService).
type nodePathFieldValues interface {
	FieldTaxonomyValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]int, error)
	FieldTaxonomyMultiValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int][]int, error)
}

// nodePathRedirects - the old addresses redirecting (implemented by
// RedirectService).
type nodePathRedirects interface {
	MoveSlugs(ctx context.Context, moves []entity.Redirect) error
}

// NodePathRebuilder rebuilds the addresses of a content type's existing
// nodes by its pattern (NodePathService): in
// steps that stop at a deadline (a request's time), each continuing after
// the last node done; the old addresses redirect to the new ones.
type NodePathRebuilder struct {
	paths     *NodePathService
	nodes     nodePathStore
	values    nodePathFieldValues
	redirects nodePathRedirects
}

// NewNodePathRebuilder - the rebuilder on paths (the pattern's addresses),
// nodes (the type's nodes and their slugs), values (their taxonomy fields'
// terms) and redirects (from the old addresses).
func NewNodePathRebuilder(paths *NodePathService, nodes nodePathStore, values nodePathFieldValues, redirects nodePathRedirects) *NodePathRebuilder {
	return &NodePathRebuilder{paths: paths, nodes: nodes, values: values, redirects: redirects}
}

// NodePathRebuild - what a step rebuilds: the type's nodes (Fields - its
// fields, for the pattern's term tokens) after After; with IncludeManual
// also those whose address was set by hand (they follow the pattern from
// then on), else only the automatic ones; DryRun - only counts and shows
// the changes (Samples), writes nothing.
type NodePathRebuild struct {
	Type          entity.ContentTypeDef
	Fields        entity.FieldInstanceDefs
	IncludeManual bool
	DryRun        bool
	After         int
}

// NodePathChange - a node's address before and after.
type NodePathChange struct {
	NodeID   int
	Title    string
	From, To string
}

// NodePathRebuildResult - a step's outcome: the nodes it checked, those
// whose address changed (the first nodePathSamples of them), the last
// node done (the next step's After) and whether nodes remain.
type NodePathRebuildResult struct {
	Checked, Changed int
	Samples          []NodePathChange
	Last             int
	More             bool
}

// nodePathBatch - the nodes a step loads at a time; nodePathSamples - the
// changes it shows.
const (
	nodePathBatch   = 100
	nodePathSamples = 50
)

// Count - how many nodes a rebuild of req walks.
func (r *NodePathRebuilder) Count(ctx context.Context, req NodePathRebuild) (int, error) {
	return r.nodes.CountPathNodes(ctx, req.Type.MachineName, !req.IncludeManual)
}

// Step rebuilds req's nodes until until. A node by hand that keeps its
// address still becomes automatic.
func (r *NodePathRebuilder) Step(ctx context.Context, req NodePathRebuild, until time.Time) (NodePathRebuildResult, error) {
	res := NodePathRebuildResult{Last: req.After}
	for {
		nodes, err := r.nodes.PathBatch(ctx, req.Type.MachineName, res.Last, nodePathBatch, !req.IncludeManual)
		if err != nil || len(nodes) == 0 {
			return res, err
		}
		values, err := r.termValues(ctx, req.Fields, nodes)
		if err != nil {
			return res, err
		}
		var moves []entity.Redirect
		for _, n := range nodes {
			if time.Now().After(until) {
				res.More = true
				return res, r.flush(ctx, moves)
			}
			to, err := r.paths.Path(ctx, req.Type, entity.DynamicContent{Node: n, Fields: req.Fields, Values: values[n.ID]})
			if err != nil {
				return res, r.flushAfter(ctx, moves, fmt.Errorf("the address of node %d: %w", n.ID, err))
			}
			res.Checked++
			if to != n.Slug {
				res.Changed++
				if len(res.Samples) < nodePathSamples {
					res.Samples = append(res.Samples, NodePathChange{NodeID: n.ID, Title: n.Title, From: n.Slug, To: to})
				}
			}
			if !req.DryRun && (to != n.Slug || !n.PathAuto) {
				if err := r.nodes.SetPath(ctx, n.ID, to, true); err != nil {
					return res, r.flushAfter(ctx, moves, fmt.Errorf("saving the address of node %d: %w", n.ID, err))
				}
				if to != n.Slug {
					moves = append(moves, entity.Redirect{FromPath: n.Slug, ToPath: to})
				}
			}
			res.Last = n.ID
		}
		if err := r.flush(ctx, moves); err != nil {
			return res, err
		}
	}
}

// flush - the redirects of the addresses changed.
func (r *NodePathRebuilder) flush(ctx context.Context, moves []entity.Redirect) error {
	if len(moves) == 0 {
		return nil
	}
	if err := r.redirects.MoveSlugs(ctx, moves); err != nil {
		return fmt.Errorf("redirecting the old addresses: %w", err)
	}
	return nil
}

// flushAfter - flush before returning err: the addresses already changed
// keep their redirects.
func (r *NodePathRebuilder) flushAfter(ctx context.Context, moves []entity.Redirect, err error) error {
	if ferr := r.flush(ctx, moves); ferr != nil {
		return fmt.Errorf("%w (and %w)", err, ferr)
	}
	return err
}

// termValues - the nodes' terms of the taxonomy fields, by node and field
// instance (DynamicContent.Values): a single value, or the first of many.
func (r *NodePathRebuilder) termValues(ctx context.Context, fields entity.FieldInstanceDefs, nodes entity.Nodes) (map[int]map[int]any, error) {
	ids := make([]int, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	out := make(map[int]map[int]any, len(nodes))
	for _, f := range fields {
		if f.FieldType != entity.FieldTypeTaxonomy {
			continue
		}
		single, err := r.values.FieldTaxonomyValuesForNodes(ctx, f.FieldStorageID, ids)
		if err != nil {
			return nil, fmt.Errorf("the terms of %s: %w", f.MachineName, err)
		}
		multi, err := r.values.FieldTaxonomyMultiValuesForNodes(ctx, f.FieldStorageID, ids)
		if err != nil {
			return nil, fmt.Errorf("the terms of %s: %w", f.MachineName, err)
		}
		for _, id := range ids {
			var v any
			if t, ok := single[id]; ok && t > 0 {
				v = t
			} else if ts := multi[id]; len(ts) > 0 {
				v = ts
			}
			if v == nil {
				continue
			}
			if out[id] == nil {
				out[id] = map[int]any{}
			}
			out[id][f.ID] = v
		}
	}
	return out, nil
}
