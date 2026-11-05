package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluereference"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
)

// nodeDeleteStep - one delete of the node delete.
type nodeDeleteStep struct {
	table string
	run   func(ctx context.Context, c *generated.Client, nodeID int) (int, error)
}

// nodeDeleteSteps are the tables that must be cleared before the nodes row:
// the core's references to the node without ON DELETE CASCADE
// (field_value_reference.target_node_id). Everything else - the modules'
// tables too - cascades from nodes: a module's table keyed by node ID
// declares its foreign key with ON DELETE CASCADE.
var nodeDeleteSteps = []nodeDeleteStep{
	{"field_value_reference", func(ctx context.Context, c *generated.Client, id int) (int, error) {
		return c.FieldValueReference.Delete().Where(fieldvaluereference.TargetNodeID(id)).Exec(ctx)
	}},
	{"nodes", func(ctx context.Context, c *generated.Client, id int) (int, error) {
		return c.Node.Delete().Where(node.ID(id)).Exec(ctx)
	}},
}

// NodeDeleteRepository implements entity.NodeDeleteRepository.
type NodeDeleteRepository struct {
	db     *sql.DB
	client *generated.Client
}

// NewNodeDeleteRepository creates a new NodeDeleteRepository.
func NewNodeDeleteRepository(db *sql.DB) *NodeDeleteRepository {
	return &NodeDeleteRepository{db: db, client: clientFor(db)}
}

// DeleteNode deletes nodeID and everything attached to it in one
// transaction.
func (r *NodeDeleteRepository) DeleteNode(ctx context.Context, nodeID int) error {
	return inTx(ctx, r.client, func(c *generated.Client) error {
		for _, st := range nodeDeleteSteps {
			// A never-installed module's table isn't there to clear.
			if ok, err := tableInstalled(ctx, r.db, st.table); err != nil || !ok {
				if err != nil {
					return fmt.Errorf("%s: %w", st.table, err)
				}
				continue
			}
			if _, err := st.run(ctx, c, nodeID); err != nil {
				return fmt.Errorf("delete from %s: %w", st.table, err)
			}
		}
		return nil
	})
}
