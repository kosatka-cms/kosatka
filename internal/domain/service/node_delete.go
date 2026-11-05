package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
)

// NodeDeleteService - the single way to delete a node of any type (bulk,
// with a user's content, per-node buttons). The repository removes what is
// attached by schema; afterwards the service removes the node's files no
// one else references (rows and disk) and refreshes the content cache so
// the node doesn't linger in cached listings.
type NodeDeleteService struct {
	nodeRepo   entity.NodesRepository
	deleteRepo entity.NodeDeleteRepository
	files      files.NodeFileCleaner
	storage    files.StoredFileRemover
	// onDeleted - runs after every successful delete (cache refresh;
	// debounced by the caller, see internal/lib/debounce). May be nil.
	onDeleted func()
	// afterDelete - modules' own cleanup of a deleted node (AfterDelete).
	afterDelete []func(ctx context.Context, n entity.Node)
	// trash - where Remove puts a content type's node (SetTrash); nil -
	// no trash, every removal for good. trashDays - how long a node stays
	// there before it is deleted for good (<= 0 - until emptied by hand).
	trash     entity.NodeTrashRepository
	trashDays int
	now       func() time.Time
	logger    *slog.Logger
}

// SetTrash gives the service the trash and how many days a node stays
// there (wiring; the config's content.trash_days).
func (s *NodeDeleteService) SetTrash(t entity.NodeTrashRepository, days int) {
	s.trash, s.trashDays = t, days
}

// Trashes - whether Remove puts n in the trash: a content type's node,
// with a trash (a module's type's is deleted for good).
func (s *NodeDeleteService) Trashes(n entity.Node) bool {
	return s != nil && s.trash != nil && !slices.Contains(entity.ModuleNodeTypes(), n.Type.ToString())
}

// TrashDays - how many days a node stays in the trash (<= 0 - until
// emptied by hand).
func (s *NodeDeleteService) TrashDays() int { return s.trashDays }

// Remove takes nodeID out of sight as user's action: a content type's
// node into the trash (restored or deleted for good later), a module's
// type's node - they have no trash - deleted for good (Delete). trashed -
// which of the two.
func (s *NodeDeleteService) Remove(ctx context.Context, nodeID int, user *entity.User) (trashed bool, err error) {
	n, err := s.nodeRepo.ByID(ctx, nodeID)
	if err != nil {
		return false, fmt.Errorf("node %d: %w", nodeID, err)
	}
	if !s.Trashes(*n) {
		return false, s.Delete(ctx, nodeID)
	}
	by := 0
	if user != nil {
		by = user.ID
	}
	if err := s.trash.TrashNode(ctx, nodeID, by, s.clock()); err != nil {
		return false, err
	}
	audit.Record(ctx, audit.Action{Type: audit.NodeTrash, Object: nodeObject(*n, false)})
	if s.onDeleted != nil {
		s.onDeleted()
	}
	s.logger.Info("node moved to the trash", "nodeID", nodeID, "type", n.Type, "title", n.Title)
	return true, nil
}

// Restore takes nodeID out of the trash.
func (s *NodeDeleteService) Restore(ctx context.Context, nodeID int) error {
	n, err := s.trashed(ctx, nodeID)
	if err != nil {
		return err
	}
	if err := s.trash.RestoreNode(ctx, nodeID); err != nil {
		return err
	}
	audit.Record(ctx, audit.Action{Type: audit.NodeRestore, Object: nodeObject(*n, true)})
	if s.onDeleted != nil {
		s.onDeleted()
	}
	return nil
}

// TrashedNodes - a page of the trash (NodeTrashRepository.TrashedNodes);
// none without a trash.
func (s *NodeDeleteService) TrashedNodes(ctx context.Context, limit, offset int) (entity.Nodes, int, error) {
	if s.trash == nil {
		return nil, 0, nil
	}
	return s.trash.TrashedNodes(ctx, limit, offset)
}

// EmptyTrash deletes every node in the trash for good; done - how many
// before a failure (or all).
func (s *NodeDeleteService) EmptyTrash(ctx context.Context) (done int, err error) {
	if s.trash == nil {
		return 0, nil
	}
	ids, err := s.trash.TrashedBefore(ctx, s.clock().Add(time.Second))
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.Delete(ctx, id); err != nil {
			return done, fmt.Errorf("emptying the trash: %w", err)
		}
		done++
	}
	return done, nil
}

// Purge deletes nodeID, in the trash, for good (Delete).
func (s *NodeDeleteService) Purge(ctx context.Context, nodeID int) error {
	if _, err := s.trashed(ctx, nodeID); err != nil {
		return err
	}
	return s.Delete(ctx, nodeID)
}

// trashed - nodeID, in the trash; ErrEntityNotFound for one that isn't.
func (s *NodeDeleteService) trashed(ctx context.Context, nodeID int) (*entity.Node, error) {
	if s.trash == nil {
		return nil, fmt.Errorf("%w: no trash", entity.ErrEntityNotFound)
	}
	n, err := s.nodeRepo.ByID(entity.WithTrashed(ctx), nodeID)
	if err != nil {
		return nil, fmt.Errorf("node %d: %w", nodeID, err)
	}
	if !n.InTrash() {
		return nil, fmt.Errorf("%w: node %d isn't in the trash", entity.ErrEntityNotFound, nodeID)
	}
	return n, nil
}

// trashSweepEvery - how often the trash is emptied of the nodes kept
// long enough.
const trashSweepEvery = time.Hour

// TrashJob - the background job deleting for good the nodes in the trash
// longer than TrashDays (none while it is <= 0: the trash is emptied by
// hand only).
func (s *NodeDeleteService) TrashJob() Job {
	return Job{Name: "trash_empty", Every: trashSweepEvery, Run: func(ctx context.Context) error {
		if s.trashDays <= 0 || s.trash == nil {
			return nil
		}
		ids, err := s.trash.TrashedBefore(ctx, s.clock().AddDate(0, 0, -s.trashDays))
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.Delete(ctx, id); err != nil {
				return fmt.Errorf("emptying the trash: %w", err)
			}
		}
		return nil
	}}
}

// clock - now (tests set it).
func (s *NodeDeleteService) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// AfterDelete adds a module's own cleanup of a deleted node (files it
// keeps outside the database), run after every successful delete, however
// the node was deleted (its page, a bulk action, a module's uninstall).
func (s *NodeDeleteService) AfterDelete(fn func(ctx context.Context, n entity.Node)) {
	s.afterDelete = append(s.afterDelete, fn)
}

// NewNodeDeleteService creates a new NodeDeleteService.
func NewNodeDeleteService(nodeRepo entity.NodesRepository, deleteRepo entity.NodeDeleteRepository, nodeFiles files.NodeFileCleaner, storage files.StoredFileRemover, onDeleted func(), baseLogger *slog.Logger) *NodeDeleteService {
	return &NodeDeleteService{
		nodeRepo:   nodeRepo,
		deleteRepo: deleteRepo,
		files:      nodeFiles,
		storage:    storage,
		onDeleted:  onDeleted,
		logger:     baseLogger.With("component", "node.delete.service"),
	}
}

// Delete deletes nodeID with everything attached to it, for good -
// whether it is in the trash or not.
func (s *NodeDeleteService) Delete(ctx context.Context, nodeID int) error {
	ctx = entity.WithTrashed(ctx)
	n, err := s.nodeRepo.ByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node %d: %w", nodeID, err)
	}
	// Collected first - the links (file fields, the modules' files) are deleted
	// with the node.
	fileIDs, err := s.files.NodeFileIDs(ctx, []int{nodeID})
	if err != nil {
		s.logger.Warn("collect node files failed, files will stay", "error", err, "nodeID", nodeID)
	}
	if err := s.deleteRepo.DeleteNode(ctx, nodeID); err != nil {
		return fmt.Errorf("deleting node (nodeID %v, type %v): %w", nodeID, n.Type, err)
	}
	files.RemoveUnreferenced(ctx, s.files, s.storage, s.logger, fileIDs)
	for _, fn := range s.afterDelete {
		fn(ctx, *n)
	}
	audit.Record(ctx, audit.Action{Type: audit.NodeDelete, Object: nodeObject(*n, false)})
	if s.onDeleted != nil {
		s.onDeleted()
	}
	s.logger.Info("node deleted", "nodeID", nodeID, "type", n.Type, "title", n.Title, "files", len(fileIDs))
	return nil
}
