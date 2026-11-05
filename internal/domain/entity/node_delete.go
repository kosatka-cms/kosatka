package entity

import (
	"context"
	"time"
)

// NodeDeleteRepository - the single node-delete path for every content
// type (see database.NodeDeleteRepository): rows without an
// ON DELETE CASCADE foreign key are removed explicitly, everything else
// goes with the nodes row, all in one transaction.
type NodeDeleteRepository interface {
	DeleteNode(ctx context.Context, nodeID int) error
}

// NodeTrashRepository - the trash (Node.DeletedAt): a node put there is
// left out of every query until restored or deleted for good.
type NodeTrashRepository interface {
	// TrashNode puts nodeID in the trash at at, by user by (0 - unknown).
	TrashNode(ctx context.Context, nodeID, by int, at time.Time) error
	// RestoreNode takes nodeID out of the trash.
	RestoreNode(ctx context.Context, nodeID int) error
	// TrashedNodes - the nodes in the trash, the latest put there first, a
	// page of limit from offset, and how many there are.
	TrashedNodes(ctx context.Context, limit, offset int) (Nodes, int, error)
	// TrashedBefore - the nodes put in the trash before before.
	TrashedBefore(ctx context.Context, before time.Time) ([]int, error)
}
