package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/userrecoverycode"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/usertwofactor"
)

// TwoFactorRepository - entity.TwoFactorRepository on ent.
type TwoFactorRepository struct {
	client *generated.Client
}

// NewTwoFactorRepository creates a TwoFactorRepository.
func NewTwoFactorRepository(db *sql.DB) *TwoFactorRepository {
	return &TwoFactorRepository{client: clientFor(db)}
}

// ByUser - see entity.TwoFactorRepository.ByUser.
func (r *TwoFactorRepository) ByUser(ctx context.Context, userID int) (*entity.TwoFactor, error) {
	client := GetClient(ctx, r.client)
	row, err := client.UserTwoFactor.Query().Where(usertwofactor.UserID(userID)).Only(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading the two-factor sign-in of user %d: %w", userID, err)
	}
	left, err := client.UserRecoveryCode.Query().Where(userrecoverycode.UserID(userID), userrecoverycode.UsedAtIsNil()).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting the recovery codes of user %d: %w", userID, err)
	}
	return &entity.TwoFactor{UserID: userID, Secret: openSecret(row.Secret), LastStep: row.LastStep, EnabledAt: row.EnabledAt, CodesLeft: left}, nil
}

// Enable - see entity.TwoFactorRepository.Enable (the secret sealed).
func (r *TwoFactorRepository) Enable(ctx context.Context, userID int, secret string, step int64, codeHashes []string, at time.Time) error {
	secret, err := sealSecret(secret)
	if err != nil {
		return err
	}
	err = inTx(ctx, r.client, func(c *generated.Client) error {
		if _, err := c.UserTwoFactor.Delete().Where(usertwofactor.UserID(userID)).Exec(ctx); err != nil {
			return err
		}
		if err := c.UserTwoFactor.Create().SetUserID(userID).SetSecret(secret).SetLastStep(step).SetEnabledAt(at).Exec(ctx); err != nil {
			return err
		}
		return replaceCodes(ctx, c, userID, codeHashes)
	})
	if err != nil {
		return fmt.Errorf("turning on the two-factor sign-in of user %d: %w", userID, err)
	}
	return nil
}

// replaceCodes - the user's recovery codes, the old ones deleted.
func replaceCodes(ctx context.Context, c *generated.Client, userID int, codeHashes []string) error {
	if _, err := c.UserRecoveryCode.Delete().Where(userrecoverycode.UserID(userID)).Exec(ctx); err != nil {
		return err
	}
	creates := make([]*generated.UserRecoveryCodeCreate, len(codeHashes))
	for i, h := range codeHashes {
		creates[i] = c.UserRecoveryCode.Create().SetUserID(userID).SetCodeHash(h)
	}
	return c.UserRecoveryCode.CreateBulk(creates...).Exec(ctx)
}

// AcceptStep - see entity.TwoFactorRepository.AcceptStep.
func (r *TwoFactorRepository) AcceptStep(ctx context.Context, userID int, step int64) (bool, error) {
	n, err := GetClient(ctx, r.client).UserTwoFactor.Update().
		Where(usertwofactor.UserID(userID), usertwofactor.LastStepLT(step)).
		SetLastStep(step).Save(ctx)
	if err != nil {
		return false, fmt.Errorf("recording a two-factor code of user %d: %w", userID, err)
	}
	return n > 0, nil
}

// UseRecoveryCode - see entity.TwoFactorRepository.UseRecoveryCode.
func (r *TwoFactorRepository) UseRecoveryCode(ctx context.Context, userID int, codeHash string) (bool, error) {
	n, err := GetClient(ctx, r.client).UserRecoveryCode.Update().
		Where(userrecoverycode.UserID(userID), userrecoverycode.CodeHash(codeHash), userrecoverycode.UsedAtIsNil()).
		SetUsedAt(time.Now()).Save(ctx)
	if err != nil {
		return false, fmt.Errorf("spending a recovery code of user %d: %w", userID, err)
	}
	return n > 0, nil
}

// ReplaceRecoveryCodes - see entity.TwoFactorRepository.ReplaceRecoveryCodes.
func (r *TwoFactorRepository) ReplaceRecoveryCodes(ctx context.Context, userID int, codeHashes []string) error {
	if err := inTx(ctx, r.client, func(c *generated.Client) error { return replaceCodes(ctx, c, userID, codeHashes) }); err != nil {
		return fmt.Errorf("replacing the recovery codes of user %d: %w", userID, err)
	}
	return nil
}

// Disable - see entity.TwoFactorRepository.Disable.
func (r *TwoFactorRepository) Disable(ctx context.Context, userID int) error {
	err := inTx(ctx, r.client, func(c *generated.Client) error {
		if _, err := c.UserRecoveryCode.Delete().Where(userrecoverycode.UserID(userID)).Exec(ctx); err != nil {
			return err
		}
		_, err := c.UserTwoFactor.Delete().Where(usertwofactor.UserID(userID)).Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("turning off the two-factor sign-in of user %d: %w", userID, err)
	}
	return nil
}

// EnabledAmong - see entity.TwoFactorRepository.EnabledAmong.
func (r *TwoFactorRepository) EnabledAmong(ctx context.Context, userIDs []int) (map[int]bool, error) {
	out := map[int]bool{}
	if len(userIDs) == 0 {
		return out, nil
	}
	ids, err := GetClient(ctx, r.client).UserTwoFactor.Query().Where(usertwofactor.UserIDIn(userIDs...)).
		Select(usertwofactor.FieldUserID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the users with two-factor sign-in: %w", err)
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
