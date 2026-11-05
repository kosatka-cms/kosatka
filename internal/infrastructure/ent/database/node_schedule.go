package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
)

// DueToPublish - see entity.NodeScheduleRepository.DueToPublish; the
// earliest due first.
func (r NodeRepository) DueToPublish(ctx context.Context, now time.Time) ([]int, error) {
	ids, err := GetClient(ctx, r.client).Node.Query().Where(node.PublishOnLTE(now)).Order(generated.Asc(node.FieldPublishOn)).IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("the nodes due to be published: %w", err)
	}
	return ids, nil
}

// DueToUnpublish - the nodes whose scheduled unpublishing has come by
// now, the earliest due first.
func (r NodeRepository) DueToUnpublish(ctx context.Context, now time.Time) ([]int, error) {
	ids, err := GetClient(ctx, r.client).Node.Query().Where(node.UnpublishOnLTE(now)).Order(generated.Asc(node.FieldUnpublishOn)).IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("the nodes due to be unpublished: %w", err)
	}
	return ids, nil
}

// Schedule - see entity.NodeScheduleRepository.Schedule.
func (r NodeRepository) Schedule(ctx context.Context) (entity.NodeSchedule, error) {
	client := GetClient(ctx, r.client)
	var out entity.NodeSchedule
	var err error
	if out.Publish, err = client.Node.Query().Where(node.PublishOnNotNil()).Count(ctx); err != nil {
		return out, fmt.Errorf("counting the scheduled publishing: %w", err)
	}
	if out.Unpublish, err = client.Node.Query().Where(node.UnpublishOnNotNil()).Count(ctx); err != nil {
		return out, fmt.Errorf("counting the scheduled unpublishing: %w", err)
	}
	for _, q := range []struct {
		field string
		pred  func() *generated.NodeQuery
	}{
		{node.FieldPublishOn, func() *generated.NodeQuery { return client.Node.Query().Where(node.PublishOnNotNil()) }},
		{node.FieldUnpublishOn, func() *generated.NodeQuery { return client.Node.Query().Where(node.UnpublishOnNotNil()) }},
	} {
		n, err := q.pred().Order(generated.Asc(q.field)).First(ctx)
		if generated.IsNotFound(err) {
			continue
		}
		if err != nil {
			return out, fmt.Errorf("the nearest scheduled %s: %w", q.field, err)
		}
		at := n.PublishOn
		if q.field == node.FieldUnpublishOn {
			at = n.UnpublishOn
		}
		if at != nil && (out.Next.IsZero() || at.Before(out.Next)) {
			out.Next = *at
		}
	}
	return out, nil
}
