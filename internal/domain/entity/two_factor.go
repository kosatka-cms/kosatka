package entity

import (
	"context"
	"time"
)

// TwoFactor - a user's two-factor sign-in, while it is on: the
// authenticator's secret (base32), the last time step a code was accepted
// for, since when, how many recovery codes are left unused.
type TwoFactor struct {
	UserID    int
	Secret    string
	LastStep  int64
	EnabledAt time.Time
	CodesLeft int
}

// TwoFactorRepository - the users' two-factor sign-in and recovery codes
// (SHA-256 hex of each).
type TwoFactorRepository interface {
	// ByUser - the user's, nil while it is off.
	ByUser(ctx context.Context, userID int) (*TwoFactor, error)
	// Enable turns it on with secret, the step its confirming code was
	// for and the recovery codes, in place of any earlier ones.
	Enable(ctx context.Context, userID int, secret string, step int64, codeHashes []string, at time.Time) error
	// AcceptStep records a code accepted for step: false - one for this
	// step or a later one was accepted already (a code isn't taken twice).
	AcceptStep(ctx context.Context, userID int, step int64) (bool, error)
	// UseRecoveryCode spends the unused recovery code: false - no such one.
	UseRecoveryCode(ctx context.Context, userID int, codeHash string) (bool, error)
	// ReplaceRecoveryCodes - new recovery codes; the old ones stop working.
	ReplaceRecoveryCodes(ctx context.Context, userID int, codeHashes []string) error
	// Disable turns it off: the secret and the codes are deleted.
	Disable(ctx context.Context, userID int) error
	// EnabledAmong - which of the users have it on.
	EnabledAmong(ctx context.Context, userIDs []int) (map[int]bool, error)
}
