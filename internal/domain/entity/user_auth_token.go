package entity

import (
	"context"
	"time"
)

// The one-time tokens' kinds: a login link, a password reset, an e-mail
// confirmation, an invitation.
const (
	UserAuthTokenKindOneTimeLogin  = "one_time_login"
	UserAuthTokenKindPasswordReset = "password_reset"
	// UserAuthTokenKindEmailConfirm - the link confirming an account's
	// e-mail address.
	UserAuthTokenKindEmailConfirm = "email_confirm"
	// UserAuthTokenKindInvite - an invitation: the link by which the
	// person an administrator created sets their own password.
	UserAuthTokenKindInvite = "invite"
)

// UserAuthTokenRepository stores hashed one-time tokens: login links,
// password resets, e-mail confirmations.
type UserAuthTokenRepository interface {
	// InvalidateUserKind removes unused tokens of a kind for a user (e.g. before issuing a new reset).
	InvalidateUserKind(ctx context.Context, userID int, kind string) error
	Insert(ctx context.Context, userID int, tokenHash, kind string, expiresAt time.Time) error
	// Consume marks the newest matching unused token as used and returns user_id (single-use).
	Consume(ctx context.Context, tokenHash, kind string) (userID int, ok bool, err error)
	// Valid - the user of an unused, unexpired token, which stays unused
	// (a form shown before the token is taken).
	Valid(ctx context.Context, tokenHash, kind string) (userID int, ok bool, err error)
	// Pending - when the newest unused token of kind of each of userIDs
	// expires (an expired one too); a user without one - absent.
	Pending(ctx context.Context, kind string, userIDs []int) (map[int]time.Time, error)
}
