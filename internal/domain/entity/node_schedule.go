package entity

import (
	"context"
	"time"
)

// NodeSchedule - the scheduled publishing as a whole: how many nodes wait
// to be published and unpublished, and when the nearest one is due (zero -
// none).
type NodeSchedule struct {
	Publish   int
	Unpublish int
	Next      time.Time
}

// NodeScheduleRepository - the nodes' scheduled publishing
// (Node.PublishOn, UnpublishOn).
type NodeScheduleRepository interface {
	// DueToPublish / DueToUnpublish - the nodes whose time has come by now.
	DueToPublish(ctx context.Context, now time.Time) ([]int, error)
	DueToUnpublish(ctx context.Context, now time.Time) ([]int, error)
	// Schedule - what waits.
	Schedule(ctx context.Context) (NodeSchedule, error)
}
