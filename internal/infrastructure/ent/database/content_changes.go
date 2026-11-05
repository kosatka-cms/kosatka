package database

import (
	"context"

	"entgo.io/ent"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// OnContentChange calls changed after each change of the content written
// through client - a node's or a comment's, created, updated or deleted:
// once its transaction commits (never on a rollback), at once outside
// one. It runs for each change, a bulk update's once: changed should be
// cheap (a debounced refresh). Writes through clients of their own
// (clientFor: the deletions' raw paths) aren't seen: they signal their own.
func OnContentChange(client *generated.Client, changed func()) {
	hook := func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			v, err := next.Mutate(ctx, m)
			if err != nil {
				return v, err
			}
			if tm, ok := m.(interface{ Tx() (*generated.Tx, error) }); ok {
				if tx, err := tm.Tx(); err == nil {
					tx.OnCommit(func(next generated.Committer) generated.Committer {
						return generated.CommitFunc(func(ctx context.Context, tx *generated.Tx) error {
							if err := next.Commit(ctx, tx); err != nil {
								return err
							}
							changed()
							return nil
						})
					})
					return v, nil
				}
			}
			changed()
			return v, nil
		})
	}
	client.Node.Use(hook)
	client.Comment.Use(hook)
}
