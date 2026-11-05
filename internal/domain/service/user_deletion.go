package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
)

// userDeletionNodeBatch - how many node ids are fetched per query
// inside one DeleteWithContentStep.
const userDeletionNodeBatch = 20

// UserDeletionService - "delete the account and all its content"
// (/admin/people): every node the user created (through the regular
// per-type node delete, with all its cascades), the modules' content of
// theirs (AddModuleContent), uploaded files (DB rows and disk), avatar and personal
// upload folder, then the account itself (comments, auth tokens, OAuth
// identities and poll votes go with it via ON DELETE CASCADE).
// Site configuration the user may have authored (content types, Views,
// vocabularies...) is never deleted - those FKs are SET NULL.
type UserDeletionService struct {
	contentRepo entity.UserContentRepository
	userRepo    entity.UserRepository
	deleteNode  func(ctx context.Context, nodeID int) error
	files       files.StoredFileRemover
	logger      *slog.Logger
	// comments - the comment hooks run for the nodes the deleted user had
	// comments on (SetComments; nil - none).
	comments CommentsService
	// modules - the modules' content of a user (AddModuleContent).
	modules []moduleUserContent
}

// moduleUserContent - a module's content of a user and the module's name.
type moduleUserContent struct {
	module string
	ModuleUserContent
}

// ModuleUserContent - a module's content of a user that their deletion
// with all content takes too: Label names its count on the confirmation
// (a translation source with @count, "private messages: @count"), Count -
// how much the user has, Delete - deletes it, before the account.
type ModuleUserContent struct {
	Label  string
	Count  func(ctx context.Context, userID int) (int, error)
	Delete func(ctx context.Context, userID int) error
}

// SetComments sets the comments service whose hooks a user's deletion runs.
func (s *UserDeletionService) SetComments(c CommentsService) { s.comments = c }

// AddModuleContent adds the module's content of a user to what the
// deletion counts and deletes, once the module was installed.
func (s *UserDeletionService) AddModuleContent(module string, c ModuleUserContent) {
	s.modules = append(s.modules, moduleUserContent{module, c})
}

// installedModules - the modules' content of a user, of the modules
// installed (their tables exist).
func (s *UserDeletionService) installedModules(ctx context.Context) ([]moduleUserContent, error) {
	var out []moduleUserContent
	for _, m := range s.modules {
		ok, err := s.contentRepo.ModuleInstalled(ctx, m.module)
		if err != nil {
			return nil, fmt.Errorf("whether module %q is installed: %w", m.module, err)
		}
		if ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// NewUserDeletionService creates a new UserDeletionService.
func NewUserDeletionService(
	contentRepo entity.UserContentRepository,
	userRepo entity.UserRepository,
	deleteNode func(ctx context.Context, nodeID int) error,
	storage files.StoredFileRemover,
	baseLogger *slog.Logger,
) *UserDeletionService {
	return &UserDeletionService{
		contentRepo: contentRepo,
		userRepo:    userRepo,
		deleteNode:  deleteNode,
		files:       storage,
		logger:      baseLogger.With("component", "user.deletion.service"),
	}
}

// Summary - what DeleteWithContentStep would delete, for the
// confirmation page: all of it, the trash and the types the viewer may
// not view too.
func (s *UserDeletionService) Summary(ctx context.Context, userID int) (entity.UserContentSummary, error) {
	ctx = entity.WithAllNodeTypes(ctx)
	summary, err := s.contentRepo.Summary(ctx, userID)
	if err != nil {
		return summary, err
	}
	modules, err := s.installedModules(ctx)
	if err != nil {
		return summary, err
	}
	for _, m := range modules {
		n, err := m.Count(ctx, userID)
		if err != nil {
			return summary, fmt.Errorf("counting %q: %w", m.Label, err)
		}
		summary.Modules = append(summary.Modules, entity.UserModuleContentCount{Label: m.Label, Count: n})
	}
	return summary, nil
}

// DeleteWithContentStep deletes the user's nodes until budget runs out
// and returns done=false while any remain - a user with thousands of
// nodes can't be deleted inside one HTTP request (server WriteTimeout,
// proxy timeouts), so the caller repeats the step until done. Once no
// nodes are left the same call deletes the modules' content, files and
// the account and returns done=true.
func (s *UserDeletionService) DeleteWithContentStep(ctx context.Context, userID int, budget time.Duration) (bool, error) {
	// Every node of the user's: in the trash, of a type the one deleting
	// may not view (a user deleting their own account) - none left behind.
	ctx = entity.WithAllNodeTypes(ctx)
	user, err := s.userRepo.ByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("user %d: %w", userID, err)
	}

	deadline := time.Now().Add(budget)
	deleted := map[int]bool{}
	for {
		ids, err := s.contentRepo.NodeIDs(ctx, userID, userDeletionNodeBatch)
		if err != nil {
			return false, err
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			if deleted[id] {
				return false, fmt.Errorf("node %d was not deleted", id)
			}
			// The node's own files go with it (NodeDeleteService).
			if err := s.deleteNode(ctx, id); err != nil {
				return false, fmt.Errorf("node %d: %w", id, err)
			}
			deleted[id] = true
			if time.Now().After(deadline) {
				return false, nil
			}
		}
	}

	fileIDs, err := s.contentRepo.FileIDsByCreator(ctx, userID)
	if err != nil {
		return false, err
	}
	modules, err := s.installedModules(ctx)
	if err != nil {
		return false, err
	}
	for _, m := range modules {
		if err := m.Delete(ctx, userID); err != nil {
			return false, fmt.Errorf("deleting %q: %w", m.Label, err)
		}
	}
	if err := deleteUserWithComments(ctx, s.comments, s.logger, userID, func() error { return s.userRepo.Delete(ctx, userID) }); err != nil {
		return false, fmt.Errorf("account: %w", err)
	}
	AuditUserDeleted(ctx, *user)
	files.RemoveUnreferenced(ctx, s.contentRepo, s.files, s.logger, fileIDs)
	// The avatar is a URI on the user row, not a files row; only an uploaded
	// one is removed, never the default picture.
	if strings.HasPrefix(user.AvatarStored, "public://"+files.UserAvatarBasePath+"/") {
		if err := s.files.RemoveStoredFile(ctx, user.AvatarStored); err != nil {
			s.logger.Warn("remove avatar file failed", "userID", userID, "uri", user.AvatarStored, "error", err)
		}
	}
	if err := s.files.RemoveUserDir(ctx, userID); err != nil {
		s.logger.Warn("remove user dir failed", "userID", userID, "error", err)
	}
	s.logger.Info("user deleted with all content", "userID", userID, "username", user.Username)
	return true, nil
}

// ErrCancelAdministrator - an administrator's account isn't deleted by
// its owner: another administrator deletes it.
var ErrCancelAdministrator = errors.New("an administrator's account is deleted by another administrator")

// cancelStepBudget - how long one step of a cancelled account's deletion
// with its content may take (DeleteWithContentStep) before the next.
const cancelStepBudget = 5 * time.Second

// Cancel deletes user's own account (the permission "cancel own account"):
// blocked at once, so no session of it works; then deleted - keeping
// their content without an author, or with it (deleteContent: in the
// background, step by step, as long as it takes). Never an
// administrator's: ErrCancelAdministrator.
func (s *UserDeletionService) Cancel(ctx context.Context, user entity.User, deleteContent bool) error {
	if HasGrant(user.Role, PermFullAccess) {
		return ErrCancelAdministrator
	}
	user.Status = entity.UserStatusBlocked
	if _, err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("blocking user %d: %w", user.ID, err)
	}
	if !deleteContent {
		if err := deleteUserWithComments(ctx, s.comments, s.logger, user.ID, func() error { return s.userRepo.Delete(ctx, user.ID) }); err != nil {
			return fmt.Errorf("deleting user %d: %w", user.ID, err)
		}
		AuditUserDeleted(ctx, user)
		s.logger.Info("user deleted their account", "userID", user.ID)
		return nil
	}
	bg := context.WithoutCancel(ctx)
	go func() {
		defer panicsafe.Recover(s.logger, "deleting a cancelled account")
		for {
			done, err := s.DeleteWithContentStep(bg, user.ID, cancelStepBudget)
			if err != nil {
				s.logger.Error("deleting a cancelled account with its content", "userID", user.ID, "error", err)
				return
			}
			if done {
				return
			}
		}
	}()
	return nil
}
