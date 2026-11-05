package accounts

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/randtoken"
)

const (
	oneTimeLoginTTL  = 24 * time.Hour
	passwordResetTTL = 24 * time.Hour
	// InviteTTL - how long an invitation's link works.
	InviteTTL = 7 * 24 * time.Hour
)

// UserAuthTokenService issues and consumes the one-time tokens of the
// links mailed to an account: a login, a password reset, an e-mail
// confirmation.
type UserAuthTokenService struct {
	repo entity.UserAuthTokenRepository
}

// NewUserAuthTokenService - the one-time tokens' service on repo.
func NewUserAuthTokenService(repo entity.UserAuthTokenRepository) *UserAuthTokenService {
	return &UserAuthTokenService{repo: repo}
}

// issue creates a token of kind for userID, valid for ttl, its earlier
// unused ones dropped: the raw secret for the link (only its hash is
// kept).
func (s *UserAuthTokenService) issue(ctx context.Context, userID int, kind string, ttl time.Duration) (string, error) {
	if s == nil || s.repo == nil {
		return "", fmt.Errorf("token service not configured")
	}
	raw, err := randtoken.Generate(32)
	if err != nil {
		return "", err
	}
	if err := s.repo.InvalidateUserKind(ctx, userID, kind); err != nil {
		return "", err
	}
	if err := s.repo.Insert(ctx, userID, randtoken.Hash(raw), kind, time.Now().Add(ttl)); err != nil {
		return "", err
	}
	return raw, nil
}

// consume takes a raw token of kind (single use): its user's ID.
func (s *UserAuthTokenService) consume(ctx context.Context, rawToken, kind string) (userID int, ok bool, err error) {
	if s == nil || s.repo == nil {
		return 0, false, fmt.Errorf("token service not configured")
	}
	return s.repo.Consume(ctx, randtoken.Hash(rawToken), kind)
}

// IssueOneTimeLogin creates a one-time login token.
func (s *UserAuthTokenService) IssueOneTimeLogin(ctx context.Context, userID int) (rawToken string, err error) {
	return s.issue(ctx, userID, entity.UserAuthTokenKindOneTimeLogin, oneTimeLoginTTL)
}

// IssuePasswordReset creates a password-reset token.
func (s *UserAuthTokenService) IssuePasswordReset(ctx context.Context, userID int) (rawToken string, err error) {
	return s.issue(ctx, userID, entity.UserAuthTokenKindPasswordReset, passwordResetTTL)
}

// IssueEmailConfirm creates a token for the link confirming userID's
// e-mail address.
func (s *UserAuthTokenService) IssueEmailConfirm(ctx context.Context, userID int) (rawToken string, err error) {
	return s.issue(ctx, userID, entity.UserAuthTokenKindEmailConfirm, emailConfirmationTTL)
}

// ConsumeOneTimeLogin consumes a one-time login token: its user's ID.
func (s *UserAuthTokenService) ConsumeOneTimeLogin(ctx context.Context, rawToken string) (userID int, ok bool, err error) {
	return s.consume(ctx, rawToken, entity.UserAuthTokenKindOneTimeLogin)
}

// ConsumePasswordReset consumes a reset token: its user's ID.
func (s *UserAuthTokenService) ConsumePasswordReset(ctx context.Context, rawToken string) (userID int, ok bool, err error) {
	return s.consume(ctx, rawToken, entity.UserAuthTokenKindPasswordReset)
}

// ConsumeEmailConfirm consumes an e-mail confirmation token: its user's
// ID.
func (s *UserAuthTokenService) ConsumeEmailConfirm(ctx context.Context, rawToken string) (userID int, ok bool, err error) {
	return s.consume(ctx, rawToken, entity.UserAuthTokenKindEmailConfirm)
}

// IssueInvite creates an invitation's token (InviteTTL), the user's
// earlier one dropped.
func (s *UserAuthTokenService) IssueInvite(ctx context.Context, userID int) (rawToken string, err error) {
	return s.issue(ctx, userID, entity.UserAuthTokenKindInvite, InviteTTL)
}

// ValidInvite - the user of an invitation's token that still works, the
// token left unused (the form setting the password is shown).
func (s *UserAuthTokenService) ValidInvite(ctx context.Context, rawToken string) (userID int, ok bool, err error) {
	if s == nil || s.repo == nil {
		return 0, false, fmt.Errorf("token service not configured")
	}
	return s.repo.Valid(ctx, randtoken.Hash(rawToken), entity.UserAuthTokenKindInvite)
}

// ConsumeInvite takes an invitation's token (single use): its user's ID.
func (s *UserAuthTokenService) ConsumeInvite(ctx context.Context, rawToken string) (userID int, ok bool, err error) {
	return s.consume(ctx, rawToken, entity.UserAuthTokenKindInvite)
}

// PendingInvites - when each of userIDs' invitation not accepted yet
// expires (an expired one too); a user not invited, or who accepted -
// absent.
func (s *UserAuthTokenService) PendingInvites(ctx context.Context, userIDs []int) (map[int]time.Time, error) {
	if s == nil || s.repo == nil {
		return map[int]time.Time{}, nil
	}
	return s.repo.Pending(ctx, entity.UserAuthTokenKindInvite, userIDs)
}
