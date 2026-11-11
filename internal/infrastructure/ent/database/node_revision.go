package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/noderevision"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/user"
)

// NodeRevisionRepository - entity.NodeRevisionRepository on ent.
type NodeRevisionRepository struct {
	client *generated.Client
}

// NewNodeRevisionRepository - the node revision repository on client.
func NewNodeRevisionRepository(client *generated.Client) *NodeRevisionRepository {
	return &NodeRevisionRepository{client: client}
}

// Create stores the revision rev (its snapshot encoded) and returns its
// ID.
func (r NodeRevisionRepository) Create(ctx context.Context, rev entity.NodeRevision) (int, error) {
	snapshot, err := rev.Snapshot.Encode()
	if err != nil {
		return 0, err
	}
	create := GetClient(ctx, r.client).NodeRevision.Create().
		SetNodeID(rev.NodeID).
		SetCreatedAt(rev.CreatedAt).
		SetKind(string(rev.Kind)).
		SetChanged(rev.Changed).
		SetSnapshot(snapshot)
	if rev.User != nil && rev.User.ID > 0 {
		create = create.SetUserID(rev.User.ID)
	}
	if !rev.RevertedFromAt.IsZero() {
		create = create.SetRevertedFromAt(rev.RevertedFromAt)
	}
	row, err := create.Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("creating a revision of node %d: %w", rev.NodeID, err)
	}
	return row.ID, nil
}

// Latest - see entity.NodeRevisionRepository.Latest.
func (r NodeRevisionRepository) Latest(ctx context.Context, nodeID int) (*entity.NodeRevision, error) {
	row, err := GetClient(ctx, r.client).NodeRevision.Query().
		Where(noderevision.NodeID(nodeID)).
		Order(generated.Desc(noderevision.FieldID)).
		First(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the latest revision of node %d: %w", nodeID, err)
	}
	return toNodeRevision(row)
}

// ByID - see entity.NodeRevisionRepository.ByID.
func (r NodeRevisionRepository) ByID(ctx context.Context, nodeID, id int) (*entity.NodeRevision, error) {
	row, err := GetClient(ctx, r.client).NodeRevision.Query().
		Where(noderevision.ID(id), noderevision.NodeID(nodeID)).
		WithUser().
		Only(ctx)
	if generated.IsNotFound(err) {
		return nil, fmt.Errorf("%w: revision %d of node %d", entity.ErrEntityNotFound, id, nodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("revision %d of node %d: %w", id, nodeID, err)
	}
	return toNodeRevision(row)
}

// List - see entity.NodeRevisionRepository.List.
func (r NodeRevisionRepository) List(ctx context.Context, nodeID, offset, limit int) ([]entity.NodeRevision, int, error) {
	client := GetClient(ctx, r.client)
	total, err := client.NodeRevision.Query().Where(noderevision.NodeID(nodeID)).Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting the revisions of node %d: %w", nodeID, err)
	}
	rows, err := client.NodeRevision.Query().
		Where(noderevision.NodeID(nodeID)).
		WithUser().
		Order(generated.Desc(noderevision.FieldID)).
		Offset(offset).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("the revisions of node %d: %w", nodeID, err)
	}
	out := make([]entity.NodeRevision, 0, len(rows))
	for _, row := range rows {
		rev, err := toNodeRevision(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rev)
	}
	return out, total, nil
}

// Prune - see entity.NodeRevisionRepository.Prune.
func (r NodeRevisionRepository) Prune(ctx context.Context, nodeID, keep int) error {
	if keep <= 0 {
		return nil
	}
	client := GetClient(ctx, r.client)
	// The oldest one kept: everything before it goes.
	ids, err := client.NodeRevision.Query().
		Where(noderevision.NodeID(nodeID)).
		Order(generated.Desc(noderevision.FieldID)).
		Offset(keep - 1).
		Limit(1).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("finding the revisions of node %d to prune: %w", nodeID, err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := client.NodeRevision.Delete().
		Where(noderevision.NodeID(nodeID), noderevision.IDLT(ids[0])).
		Exec(ctx); err != nil {
		return fmt.Errorf("pruning the revisions of node %d: %w", nodeID, err)
	}
	return nil
}

// toNodeRevision - row as the entity, its user when loaded.
func toNodeRevision(row *generated.NodeRevision) (*entity.NodeRevision, error) {
	snapshot, err := entity.DecodeNodeSnapshot(row.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("revision %d: %w", row.ID, err)
	}
	rev := &entity.NodeRevision{
		ID:        row.ID,
		NodeID:    row.NodeID,
		CreatedAt: row.CreatedAt,
		Kind:      entity.RevisionKind(row.Kind),
		Changed:   row.Changed,
		Snapshot:  snapshot,
	}
	if row.RevertedFromAt != nil {
		rev.RevertedFromAt = *row.RevertedFromAt
	}
	if u := row.Edges.User; u != nil {
		user := dao.ToUserEntity(u)
		rev.User = &user
	} else if row.UserID != nil {
		rev.User = &entity.User{ID: *row.UserID}
	}
	return rev, nil
}

// Recent - see entity.NodeRevisionRepository.Recent.
func (r NodeRevisionRepository) Recent(ctx context.Context, f entity.RecentRevisionsFilter) ([]entity.RecentRevision, int, error) {
	var scope []predicate.NodeRevision
	if len(f.AnyTypes) > 0 {
		scope = append(scope, noderevision.HasNodeWith(node.TypeIn(f.AnyTypes...)))
	}
	if len(f.OwnTypes) > 0 && f.OwnerID > 0 {
		scope = append(scope, noderevision.HasNodeWith(node.TypeIn(f.OwnTypes...), node.CreatedBy(f.OwnerID)))
	}
	if len(scope) == 0 {
		return nil, 0, nil
	}
	where := []predicate.NodeRevision{noderevision.Or(scope...)}
	var nodeWhere []predicate.Node
	if t := strings.TrimSpace(f.Title); t != "" {
		nodeWhere = append(nodeWhere, node.TitleContainsFold(t))
	}
	if f.Type != "" {
		nodeWhere = append(nodeWhere, node.Type(f.Type))
	}
	if len(nodeWhere) > 0 {
		where = append(where, noderevision.HasNodeWith(nodeWhere...))
	}
	if a := strings.TrimSpace(f.Actor); a != "" {
		where = append(where, noderevision.HasUserWith(user.UsernameContainsFold(a)))
	}
	if !f.From.IsZero() {
		where = append(where, noderevision.CreatedAtGTE(f.From))
	}
	if !f.To.IsZero() {
		where = append(where, noderevision.CreatedAtLT(f.To))
	}
	client := GetClient(ctx, r.client)
	q := client.NodeRevision.Query().Where(where...)
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting the recent revisions: %w", err)
	}
	// Without the snapshots: a page of them may weigh megabytes.
	rows, err := q.Select(noderevision.FieldID, noderevision.FieldNodeID, noderevision.FieldUserID, noderevision.FieldCreatedAt,
		noderevision.FieldKind, noderevision.FieldRevertedFromAt, noderevision.FieldChanged).
		WithNode(func(nq *generated.NodeQuery) {
			nq.Select(node.FieldID, node.FieldTitle, node.FieldType, node.FieldSlug, node.FieldLangcode)
		}).
		WithUser().
		Order(generated.Desc(noderevision.FieldID)).
		Offset(f.Offset).
		Limit(f.Limit).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("listing the recent revisions: %w", err)
	}
	out := make([]entity.RecentRevision, len(rows))
	var nodeIDs, statusIDs []int
	for i, row := range rows {
		rev := entity.NodeRevision{ID: row.ID, NodeID: row.NodeID, CreatedAt: row.CreatedAt, Kind: entity.RevisionKind(row.Kind), Changed: row.Changed}
		if row.RevertedFromAt != nil {
			rev.RevertedFromAt = *row.RevertedFromAt
		}
		if u := row.Edges.User; u != nil {
			usr := dao.ToUserEntity(u)
			rev.User = &usr
		}
		out[i] = entity.RecentRevision{NodeRevision: rev}
		if n := row.Edges.Node; n != nil {
			out[i].NodeTitle, out[i].NodeType = n.Title, n.Type
			out[i].NodeURL = fmt.Sprintf("/node/%d/edit", n.ID)
			if n.Slug != "" {
				out[i].NodeURL = entity.Node{ID: n.ID, Type: entity.ContentType(n.Type), Slug: n.Slug, Langcode: n.Langcode}.URL()
			}
		}
		nodeIDs = append(nodeIDs, row.NodeID)
		if len(row.Changed) == 1 && (row.Changed[0] == entity.SnapshotStatus || row.Changed[0] == entity.SnapshotOnModeration) {
			statusIDs = append(statusIDs, row.ID)
		}
	}
	if err := r.previous(ctx, client, out, nodeIDs); err != nil {
		return nil, 0, err
	}
	if err := r.statuses(ctx, client, out, statusIDs); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// previous fills the revisions' PreviousID: the node's revision just
// before each.
func (r NodeRevisionRepository) previous(ctx context.Context, client *generated.Client, revs []entity.RecentRevision, nodeIDs []int) error {
	if len(revs) == 0 {
		return nil
	}
	others, err := client.NodeRevision.Query().
		Where(noderevision.NodeIDIn(nodeIDs...), noderevision.IDLT(revs[0].ID)).
		Select(noderevision.FieldID, noderevision.FieldNodeID).
		All(ctx)
	if err != nil {
		return fmt.Errorf("the revisions before the recent ones: %w", err)
	}
	for i := range revs {
		for _, o := range others {
			if o.NodeID == revs[i].NodeID && o.ID < revs[i].ID && o.ID > revs[i].PreviousID {
				revs[i].PreviousID = o.ID
			}
		}
	}
	return nil
}

// statuses fills the Status and OnModeration of the revisions ids (those
// that changed only them) from their snapshots.
func (r NodeRevisionRepository) statuses(ctx context.Context, client *generated.Client, revs []entity.RecentRevision, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := client.NodeRevision.Query().Where(noderevision.IDIn(ids...)).Select(noderevision.FieldID, noderevision.FieldSnapshot).All(ctx)
	if err != nil {
		return fmt.Errorf("the recent revisions' snapshots: %w", err)
	}
	for _, row := range rows {
		snap, err := entity.DecodeNodeSnapshot(row.Snapshot)
		if err != nil {
			return fmt.Errorf("revision %d: %w", row.ID, err)
		}
		for i := range revs {
			if revs[i].ID == row.ID {
				revs[i].Snapshot.Status, revs[i].Snapshot.OnModeration = snap.Status, snap.OnModeration
			}
		}
	}
	return nil
}
