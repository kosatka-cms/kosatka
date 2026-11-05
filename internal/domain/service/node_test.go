package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// takenSlugs - the nodes' slugs, for SlugExists.
type takenSlugs struct {
	entity.NodesRepository
	taken map[string]bool
}

func (t takenSlugs) SlugExists(_ context.Context, slug string) (bool, error) {
	return t.taken[slug], nil
}

// TestGenerateUniqueSlug - the type's prefix is a path segment (a title
// starting with the type's name gets it too), a taken slug a number.
func TestGenerateUniqueSlug(t *testing.T) {
	svc := NewNodesService(takenSlugs{taken: map[string]bool{"poll/lunch": true}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for title, want := range map[string]string{"Pollution": "poll/pollution", "Lunch": "poll/lunch-1"} {
		if got, err := svc.GenerateUniqueSlug(context.Background(), title, "poll"); err != nil || got != want {
			t.Errorf("%s: %q %v, want %q", title, got, err, want)
		}
	}
}
