package service

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// fakePreviewLinks - the links by token hash.
type fakePreviewLinks struct {
	byHash map[string]entity.PreviewLink
}

func (f *fakePreviewLinks) ReplacePreviewLink(_ context.Context, link entity.PreviewLink, hash string) error {
	for h, l := range f.byHash {
		if l.NodeID == link.NodeID {
			delete(f.byHash, h)
		}
	}
	f.byHash[hash] = link
	return nil
}

func (f *fakePreviewLinks) PreviewLinkOf(_ context.Context, nodeID int) (*entity.PreviewLink, error) {
	for _, l := range f.byHash {
		if l.NodeID == nodeID {
			return &l, nil
		}
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakePreviewLinks) PreviewLinkByHash(_ context.Context, hash string) (*entity.PreviewLink, error) {
	if l, ok := f.byHash[hash]; ok {
		return &l, nil
	}
	return nil, entity.ErrEntityNotFound
}

func (f *fakePreviewLinks) DeletePreviewLinks(_ context.Context, nodeID int) error {
	for h, l := range f.byHash {
		if l.NodeID == nodeID {
			delete(f.byHash, h)
		}
	}
	return nil
}

// TestPreviewLinks - a link resolves to its node while it works; a new one
// replaces it; revoked or expired, it resolves to nothing.
func TestPreviewLinks(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	p := &PreviewLinks{Repo: &fakePreviewLinks{byHash: map[string]entity.PreviewLink{}}, now: func() time.Time { return now }}
	ctx := context.Background()
	first, expires, err := p.Create(ctx, 7, 1)
	if err != nil || !expires.Equal(now.Add(PreviewLinkTTL)) || len(first) < 32 {
		t.Fatalf("create: %q %v %v", first, expires, err)
	}
	if id, ok, err := p.Resolve(ctx, first); err != nil || !ok || id != 7 {
		t.Errorf("resolve: %d %v %v", id, ok, err)
	}
	second, _, _ := p.Create(ctx, 7, 1)
	if _, ok, _ := p.Resolve(ctx, first); ok {
		t.Error("a replaced link still works")
	}
	if _, ok, _ := p.Active(ctx, 7); !ok {
		t.Error("no active link")
	}
	now = now.Add(PreviewLinkTTL + time.Minute)
	if _, ok, _ := p.Resolve(ctx, second); ok {
		t.Error("an expired link works")
	}
	if _, ok, _ := p.Active(ctx, 7); ok {
		t.Error("an expired link is active")
	}
	third, _, _ := p.Create(ctx, 7, 1)
	if err := p.Revoke(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := p.Resolve(ctx, third); ok {
		t.Error("a revoked link works")
	}
	if _, ok, _ := p.Resolve(ctx, ""); ok {
		t.Error("an empty token works")
	}
}
