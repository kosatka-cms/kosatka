package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

type txKey struct{}

// EntUnitOfWork implements entity.UnitOfWork with ent transactions.
type EntUnitOfWork struct {
	client *generated.Client
}

// NewUnitOfWork - the unit of work running its transactions on client.
func NewUnitOfWork(client *generated.Client) *EntUnitOfWork {
	return &EntUnitOfWork{
		client: client,
	}
}

// Do runs fn in a transaction: the repositories fn calls (GetClient)
// write in it, and it commits when fn succeeds. Inside another unit of
// work, or with a transaction's client, fn joins that transaction: a
// second one on another connection would not see its rows and could wait
// on its locks.
func (u *EntUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*generated.Client); ok {
		return fn(ctx)
	}
	tx, err := u.client.Tx(ctx)
	if errors.Is(err, generated.ErrTxStarted) {
		// The client is a transaction's (an install step's): fn writes in it.
		return fn(context.WithValue(ctx, txKey{}, u.client))
	}
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Roll back on panic.
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()

	// Store tx.Client() (a *generated.Client) in the context, and the
	// transaction itself (generated.TxFromContext: afterWrite's commit
	// hooks).
	txCtx := generated.NewTxContext(context.WithValue(ctx, txKey{}, tx.Client()), tx)

	if err := fn(txCtx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("rolling back transaction: %w (original error: %w)", rerr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// GetClient returns the transactional client from ctx, or the given client
// if there is no transaction.
func GetClient(ctx context.Context, defaultClient *generated.Client) *generated.Client {
	if tx, ok := ctx.Value(txKey{}).(*generated.Client); ok {
		return tx
	}
	return defaultClient
}

// afterWrite runs forget (a kept copy's reset) once a write made with ctx
// is what every reader sees: at once, and again when ctx's transaction
// commits - a read outside it in between still saw, and may have kept,
// the row before the write.
func afterWrite(ctx context.Context, forget func()) {
	forget()
	tx := generated.TxFromContext(ctx)
	if tx == nil {
		return
	}
	tx.OnCommit(func(next generated.Committer) generated.Committer {
		return generated.CommitFunc(func(ctx context.Context, tx *generated.Tx) error {
			err := next.Commit(ctx, tx)
			forget()
			return err
		})
	})
}
