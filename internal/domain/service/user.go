package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
)

// UserService - the user accounts: finding them, saving them, confirming
// their e-mail and ending their sessions.
type UserService interface {
	Find(ctx context.Context, filter entity.UserFilter) (*entity.User, error)
	FindByEmailOrUsername(ctx context.Context, identity string) (*entity.User, error)
	ByID(ctx context.Context, id int) (*entity.User, error)
	// ByIDs - the users with these ids, by id (a missing one is absent).
	ByIDs(ctx context.Context, ids []int) (map[int]entity.User, error)
	FindBySlug(ctx context.Context, slug string) (*entity.User, error)
	FindByUsername(ctx context.Context, username string) (*entity.User, error)
	Edit(ctx context.Context, ct entity.User) (*entity.User, error)
	Create(ctx context.Context, ct entity.User) (*entity.User, error)
	List(ctx context.Context) (*entity.Users, error)
	// Search - active users by username (SearchActiveByName).
	Search(ctx context.Context, query string, roles []int, limit int) (*entity.Users, error)
	// ConfirmEmail marks user id's e-mail address confirmed (its
	// confirmation link followed): the user.
	ConfirmEmail(ctx context.Context, id int) (*entity.User, error)
	CountWithStatus(ctx context.Context, status int) (int, error)
	// BumpSessionVersion ends every session of the account (see
	// entity.UserRepository.BumpSessionVersion); the new version.
	BumpSessionVersion(ctx context.Context, id int) (int, error)
}

// DefaultUserService - the UserService on the users' repository.
type DefaultUserService struct {
	repository entity.UserRepository
	logger     *slog.Logger
}

// NewUserService - the user service on r (the users) and baseLogger.
func NewUserService(r entity.UserRepository, baseLogger *slog.Logger) UserService {
	return &DefaultUserService{
		repository: r,
		logger:     baseLogger.With("component", "user.service"),
	}
}

// Find - the user matching filter (by e-mail).
func (s *DefaultUserService) Find(ctx context.Context, filter entity.UserFilter) (*entity.User, error) {
	ct, err := s.repository.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	return ct, nil
}

// FindByEmailOrUsername - the user whose e-mail or username is identity
// (trimmed, any case); an empty one - apperrors.ErrNotFound.
func (s *DefaultUserService) FindByEmailOrUsername(ctx context.Context, identity string) (*entity.User, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return nil, fmt.Errorf("%w", apperrors.ErrNotFound)
	}
	return s.repository.FindByEmailOrUsername(ctx, identity)
}

// ByIDs - see UserService.ByIDs.
func (s *DefaultUserService) ByIDs(ctx context.Context, ids []int) (map[int]entity.User, error) {
	users, err := s.repository.ByIDs(ctx, ids)
	return entity.Wrapped(users, err, "getting users by ids (%v)", ids)
}

// ByID - the user id.
func (s *DefaultUserService) ByID(ctx context.Context, id int) (*entity.User, error) {
	ct, err := s.repository.ByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting user by id (id %v): %w", id, err)
	}

	return ct, nil
}

// Create creates the user; one created by a signed-in user (an
// administrator - not a sign-up) goes to the action log.
func (s *DefaultUserService) Create(ctx context.Context, ct entity.User) (*entity.User, error) {
	res, err := s.repository.Create(ctx, ct)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	if audit.ActorFrom(ctx).UserID != 0 {
		audit.Record(ctx, audit.Action{Type: audit.UserCreate, Object: userObject(*res, true)})
	}

	return res, nil
}

// List - every user.
func (s *DefaultUserService) List(ctx context.Context) (*entity.Users, error) {
	res, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	return res, nil
}

// Edit saves the user ct.
func (s *DefaultUserService) Edit(ctx context.Context, ct entity.User) (*entity.User, error) {
	res, err := s.repository.Update(ctx, ct)
	if err != nil {
		return nil, fmt.Errorf("updating user (id %v): %w", ct.ID, err)
	}

	return res, nil
}

// Search - active users whose username contains query (a member looking
// for another: not by e-mail, so the search can't tell whether an
// address has an account).
func (s *DefaultUserService) Search(ctx context.Context, query string, roles []int, limit int) (*entity.Users, error) {
	users, err := s.repository.SearchActiveByName(ctx, query, roles, limit)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// FindBySlug - the user whose profile slug is slug.
func (s *DefaultUserService) FindBySlug(ctx context.Context, slug string) (*entity.User, error) {
	user, err := s.repository.BySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	return user, err
}

// FindByUsername - the user named username.
func (s *DefaultUserService) FindByUsername(ctx context.Context, username string) (*entity.User, error) {
	user, err := s.repository.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ConfirmEmail - see UserService.ConfirmEmail.
func (s *DefaultUserService) ConfirmEmail(ctx context.Context, id int) (*entity.User, error) {
	if err := s.repository.ConfirmEmail(ctx, id); err != nil {
		return nil, fmt.Errorf("confirming the e-mail of user %d: %w", id, err)
	}
	user, err := s.repository.ByID(ctx, id)
	return entity.Wrapped(user, err, "loading user %d", id)
}

// CountWithStatus - how many users have status (entity.UserStatus*).
func (s *DefaultUserService) CountWithStatus(ctx context.Context, status int) (int, error) {
	return s.repository.CountWithStatus(ctx, status)
}

// BumpSessionVersion - see UserService.BumpSessionVersion.
func (s *DefaultUserService) BumpSessionVersion(ctx context.Context, id int) (int, error) {
	v, err := s.repository.BumpSessionVersion(ctx, id)
	return entity.Wrapped(v, err, "ending the sessions of user %d", id)
}
