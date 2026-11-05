package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// AdminBulkService handles admin bulk operations (delete nodes, update user status).
type AdminBulkService interface {
	DeleteNode(ctx context.Context, nodeID int) error
	BulkDeleteNodes(ctx context.Context, user *entity.User, ids []int) (processed int, errors []string)
	// BulkUpdateNodeStatus / BulkUpdateNodeModeration - by user: the
	// revisions of the nodes that keep a history say who.
	BulkUpdateNodeStatus(ctx context.Context, user *entity.User, ids []int, status int) (processed int, errors []string)
	BulkUpdateNodeModeration(ctx context.Context, user *entity.User, ids []int, onModeration int) (processed int, errors []string)
	BulkUpdateUserStatus(ctx context.Context, ids []int, status int) (processed int, errors []string)
	BulkDeleteUsers(ctx context.Context, ids []int) (processed int, errors []string)
}

// DefaultAdminBulkService implements AdminBulkService.
type DefaultAdminBulkService struct {
	nodeRepo    entity.NodesRepository
	nodeDeleter *NodeDeleteService
	userRepo    entity.UserRepository
	logger      *slog.Logger
	// comments - see UserDeletionService.comments.
	comments CommentsService
	// revisions - the history of the nodes whose publishing changes.
	revisions *NodeRevisionService
}

// SetComments sets the comments service whose hooks deleting users runs.
func (s *DefaultAdminBulkService) SetComments(c CommentsService) { s.comments = c }

// SetRevisions sets the history a node's publishing change is recorded in.
func (s *DefaultAdminBulkService) SetRevisions(r *NodeRevisionService) { s.revisions = r }

// track runs change on node id as user, recorded in the node's history
// when there is one.
func (s *DefaultAdminBulkService) track(ctx context.Context, user *entity.User, id int, change func(ctx context.Context) error) error {
	if s.revisions == nil {
		return change(ctx)
	}
	return s.revisions.Track(ctx, id, user, change)
}

// NewAdminBulkService creates a new AdminBulkService.
func NewAdminBulkService(
	nodeRepo entity.NodesRepository,
	nodeDeleter *NodeDeleteService,
	userRepo entity.UserRepository,
	baseLogger *slog.Logger,
) *DefaultAdminBulkService {
	return &DefaultAdminBulkService{
		nodeRepo:    nodeRepo,
		nodeDeleter: nodeDeleter,
		userRepo:    userRepo,
		logger:      baseLogger.With("component", "admin.bulk.service"),
	}
}

// DeleteNode deletes a node of any content type for good via
// NodeDeleteService (a user's content deleted with them).
func (s *DefaultAdminBulkService) DeleteNode(ctx context.Context, nodeID int) error {
	return s.nodeDeleter.Delete(ctx, nodeID)
}

// BulkDeleteNodes removes multiple nodes as user (NodeDeleteService.
// Remove: a content type's to the trash, a module's type's for good),
// collecting errors.
func (s *DefaultAdminBulkService) BulkDeleteNodes(ctx context.Context, user *entity.User, ids []int) (processed int, errors []string) {
	for _, id := range ids {
		if _, err := s.nodeDeleter.Remove(ctx, id, user); err != nil {
			s.logger.Warn("bulk delete node failed", "nodeID", id, "error", err)
			errors = append(errors, fmt.Sprintf("ID %d: %v", id, err))
		} else {
			processed++
		}
	}
	return processed, errors
}

// managedPublishing - why node id's publishing can't be changed here
// ("" - it can): a module publishes its type's items (PublishingManaged).
func (s *DefaultAdminBulkService) managedPublishing(ctx context.Context, id int) string {
	node, err := s.nodeRepo.ByID(ctx, id)
	if err != nil || node == nil || !PublishingManaged(ctx, string(node.Type)) {
		return ""
	}
	return i18n.T(ctx, "ID @id: published by its workflow's transitions, not here.", "@id", id)
}

// BulkUpdateNodeStatus updates status for multiple nodes (0=unpublished,
// 1=published); a node a module publishes (PublishingManaged) is left
// out with an error.
func (s *DefaultAdminBulkService) BulkUpdateNodeStatus(ctx context.Context, user *entity.User, ids []int, status int) (processed int, errors []string) {
	for _, id := range ids {
		if msg := s.managedPublishing(ctx, id); msg != "" {
			errors = append(errors, msg)
			continue
		}
		if err := s.track(ctx, user, id, func(ctx context.Context) error { return s.nodeRepo.UpdateStatus(ctx, id, status) }); err != nil {
			s.logger.Warn("bulk update node status failed", "nodeID", id, "error", err)
			errors = append(errors, fmt.Sprintf("ID %d: %v", id, err))
		} else {
			processed++
		}
	}
	return processed, errors
}

// BulkUpdateNodeModeration updates on_moderation for multiple nodes
// (0=off, 1=on); a node a module publishes is left out, as by
// BulkUpdateNodeStatus.
func (s *DefaultAdminBulkService) BulkUpdateNodeModeration(ctx context.Context, user *entity.User, ids []int, onModeration int) (processed int, errors []string) {
	for _, id := range ids {
		if msg := s.managedPublishing(ctx, id); msg != "" {
			errors = append(errors, msg)
			continue
		}
		if err := s.track(ctx, user, id, func(ctx context.Context) error { return s.nodeRepo.UpdateOnModeration(ctx, id, onModeration) }); err != nil {
			s.logger.Warn("bulk update node moderation failed", "nodeID", id, "error", err)
			errors = append(errors, fmt.Sprintf("ID %d: %v", id, err))
		} else {
			processed++
		}
	}
	return processed, errors
}

// BulkUpdateUserStatus updates status for multiple users (0=blocked, 1=active).
func (s *DefaultAdminBulkService) BulkUpdateUserStatus(ctx context.Context, ids []int, status int) (processed int, errors []string) {
	for _, id := range ids {
		user, err := s.userRepo.ByID(ctx, id)
		if err != nil {
			s.logger.Warn("bulk update user status: user not found", "userID", id, "error", err)
			errors = append(errors, fmt.Sprintf("ID %d: %v", id, err))
			continue
		}
		from := user.Status
		user.Status = status
		if _, err := s.userRepo.Update(ctx, *user); err != nil {
			s.logger.Warn("bulk update user status failed", "userID", id, "error", err)
			errors = append(errors, fmt.Sprintf("ID %d: %v", id, err))
		} else {
			processed++
			AuditUserStatus(ctx, *user, from, status)
		}
	}
	return processed, errors
}

// BulkDeleteUsers deletes multiple users.
func (s *DefaultAdminBulkService) BulkDeleteUsers(ctx context.Context, ids []int) (processed int, errors []string) {
	for _, id := range ids {
		user, _ := s.userRepo.ByID(ctx, id) // for the action log only
		if err := deleteUserWithComments(ctx, s.comments, s.logger, id, func() error { return s.userRepo.Delete(ctx, id) }); err != nil {
			s.logger.Warn("bulk delete user failed", "userID", id, "error", err)
			errors = append(errors, fmt.Sprintf("ID %d: %v", id, err))
		} else {
			processed++
			if user != nil {
				AuditUserDeleted(ctx, *user)
			}
		}
	}
	return processed, errors
}
