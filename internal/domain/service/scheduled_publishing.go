package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// scheduledNodes - what the scheduled publishing changes: a node's
// publishing (UpdateStatus clears the schedule it fulfils), its
// moderation, the schedule of a node a module publishes (ClearSchedule).
type scheduledNodes interface {
	ByID(ctx context.Context, id int) (*entity.Node, error)
	UpdateStatus(ctx context.Context, nodeID int, status int) error
	UpdateOnModeration(ctx context.Context, nodeID int, onModeration int) error
	ClearSchedule(ctx context.Context, nodeID int) error
}

// ScheduledPublishing - the job publishing and unpublishing the nodes
// whose time has come (Node.PublishOn, UnpublishOn): a published one
// leaves moderation too. Each change is recorded in the node's history
// as any publishing is, with no user: the site did it.
type ScheduledPublishing struct {
	schedule  entity.NodeScheduleRepository
	nodes     scheduledNodes
	revisions *NodeRevisionService
	now       func() time.Time
}

// NewScheduledPublishing - the scheduled publishing on schedule (the
// nodes' due changes), nodes (their publishing and moderation) and
// revisions (the history each change is recorded in).
func NewScheduledPublishing(schedule entity.NodeScheduleRepository, nodes scheduledNodes, revisions *NodeRevisionService) *ScheduledPublishing {
	return &ScheduledPublishing{schedule: schedule, nodes: nodes, revisions: revisions, now: time.Now}
}

// Job - the scheduler's job, every minute.
func (p *ScheduledPublishing) Job() Job {
	return Job{Name: "scheduled_publishing", Every: time.Minute, Run: p.Run}
}

// Run publishes the nodes due to be published, then unpublishes the ones
// due to be unpublished; a node that fails doesn't stop the others. A
// node a module publishes (PublishingManaged: scheduled before its type
// joined) isn't: its schedule is cleared.
func (p *ScheduledPublishing) Run(ctx context.Context) error {
	now := p.now()
	var errs []error
	publish, err := p.schedule.DueToPublish(ctx, now)
	if err != nil {
		return err
	}
	for _, id := range publish {
		if managed, err := p.managed(ctx, id); managed || err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, p.track(ctx, id, func(ctx context.Context) error {
			if err := p.nodes.UpdateStatus(ctx, id, 1); err != nil {
				return fmt.Errorf("publishing node %d on schedule: %w", id, err)
			}
			return p.nodes.UpdateOnModeration(ctx, id, 0)
		}))
	}
	unpublish, err := p.schedule.DueToUnpublish(ctx, now)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, id := range unpublish {
		if managed, err := p.managed(ctx, id); managed || err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, p.track(ctx, id, func(ctx context.Context) error {
			if err := p.nodes.UpdateStatus(ctx, id, 0); err != nil {
				return fmt.Errorf("unpublishing node %d on schedule: %w", id, err)
			}
			return nil
		}))
	}
	return errors.Join(errs...)
}

// managed - node id is published by a module (PublishingManaged): its
// schedule cleared.
func (p *ScheduledPublishing) managed(ctx context.Context, id int) (bool, error) {
	node, err := p.nodes.ByID(ctx, id)
	if err != nil {
		return false, fmt.Errorf("loading scheduled node %d: %w", id, err)
	}
	if !PublishingManaged(ctx, string(node.Type)) {
		return false, nil
	}
	if err := p.nodes.ClearSchedule(ctx, id); err != nil {
		return true, fmt.Errorf("clearing the schedule of node %d: %w", id, err)
	}
	return true, nil
}

// track runs change on node id, recorded in its history when it keeps one.
func (p *ScheduledPublishing) track(ctx context.Context, id int, change func(ctx context.Context) error) error {
	if p.revisions == nil {
		return change(ctx)
	}
	return p.revisions.Track(ctx, id, nil, change)
}

// CheckScheduler - the background jobs keep running (their last tick at
// most five minutes old) and what the scheduled publishing waits for.
func CheckScheduler(lastTick func() time.Time, schedule func(ctx context.Context) (entity.NodeSchedule, error)) StatusCheck {
	return func(ctx context.Context) []entity.Requirement {
		t := lastTick()
		r := entity.Requirement{Title: i18n.T(ctx, "Background jobs")}
		switch {
		case t.IsZero():
			r.Value = i18n.T(ctx, "Not run yet")
			r.Severity = entity.RequirementWarning
		case time.Since(t) > 5*SchedulerTick:
			r.Value = i18n.T(ctx, "Last run @ago ago", "@ago", time.Since(t).Round(time.Minute).String())
			r.Severity = entity.RequirementWarning
			r.Description = i18n.T(ctx, "The content scheduled for publishing is not published. Check that the site's service is running and its log.")
		default:
			r.Value = i18n.T(ctx, "Last run @ago ago", "@ago", time.Since(t).Round(time.Second).String())
		}
		if r.Severity == entity.RequirementOK {
			if s, err := schedule(ctx); err == nil && (s.Publish > 0 || s.Unpublish > 0) {
				r.Description = i18n.T(ctx, "Scheduled: @publish to publish, @unpublish to unpublish; the nearest in @in.",
					"@publish", s.Publish, "@unpublish", s.Unpublish, "@in", hoursMinutes(ctx, time.Until(s.Next)))
			}
		}
		return []entity.Requirement{r}
	}
}

// hoursMinutes - d as "3 h 5 min" (or "5 min"), not below a minute.
func hoursMinutes(ctx context.Context, d time.Duration) string {
	minutes := max(int(d.Round(time.Minute)/time.Minute), 1)
	if minutes < 60 {
		return i18n.T(ctx, "@m min", "@m", minutes)
	}
	return i18n.T(ctx, "@h h @m min", "@h", minutes/60, "@m", minutes%60)
}
