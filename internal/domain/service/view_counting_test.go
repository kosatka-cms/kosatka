package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

type countViewNodes struct {
	entity.NodesRepository
	nodes map[int]*entity.Node
}

func (n countViewNodes) ByID(_ context.Context, id int) (*entity.Node, error) {
	if node, ok := n.nodes[id]; ok {
		return node, nil
	}
	return nil, fmt.Errorf("%w: node with ID %d", entity.ErrEntityNotFound, id)
}

type countViewCounters struct {
	counted []int
	before  []int
}

func (c *countViewCounters) IncrementView(_ context.Context, nid int) error {
	c.counted = append(c.counted, nid)
	return nil
}

func (c *countViewCounters) DeleteViewDaysBefore(_ context.Context, day int) error {
	c.before = append(c.before, day)
	return nil
}

type viewSettingsRepo struct{ s entity.StatisticsSettings }

func (r *viewSettingsRepo) Get(context.Context) (*entity.StatisticsSettings, error) {
	s := r.s
	return &s, nil
}

func (r *viewSettingsRepo) Update(_ context.Context, s *entity.StatisticsSettings) error {
	r.s = *s
	return nil
}

// TestCountView - a view of a public node is counted; nothing is while
// counting is off, for a missing node or for a node not everyone may see.
func TestCountView(t *testing.T) {
	ctx := context.Background()
	counters := &countViewCounters{}
	nodes := countViewNodes{nodes: map[int]*entity.Node{
		1: {ID: 1, Status: 1},
		2: {ID: 2, Status: 0},
	}}
	s := NewViewCountingService(nil, nodes, counters)

	if err := s.CountView(ctx, 1); !errors.Is(err, ErrViewCountingOff) {
		t.Fatalf("counting off: %v", err)
	}
	s.enabled = true
	if err := s.CountView(ctx, 1); err != nil {
		t.Fatalf("public node: %v", err)
	}
	if err := s.CountView(ctx, 2); !errors.Is(err, ErrNodeNotPublic) {
		t.Errorf("unpublished node: %v", err)
	}
	if err := s.CountView(ctx, 3); !apperrors.IsNotFound(err) {
		t.Errorf("missing node: %v", err)
	}
	if len(counters.counted) != 1 || counters.counted[0] != 1 {
		t.Errorf("counted %v, want [1]", counters.counted)
	}
}

// TestViewDaysPurgeJob - the job deletes the views by day before the
// kept days (today the first of them); with 0 days kept, nothing.
func TestViewDaysPurgeJob(t *testing.T) {
	ctx := context.Background()
	counters := &countViewCounters{}
	settings := &viewSettingsRepo{s: entity.StatisticsSettings{DailyViewsKeepDays: 30}}
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, sitetime.Location())
	job := NewViewCountingService(settings, nil, counters).ViewDaysPurgeJob(func() time.Time { return now })

	if err := job.Run(ctx); err != nil {
		t.Fatal(err)
	}
	settings.s.DailyViewsKeepDays = 0
	if err := job.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(counters.before) != 1 || counters.before[0] != 20260302 {
		t.Errorf("deleted before %v, want [20260302]", counters.before)
	}
}
