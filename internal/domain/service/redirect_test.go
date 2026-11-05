package service

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// memRedirects - an in-memory entity.RedirectRepository.
type memRedirects struct {
	rows   []entity.Redirect
	nextID int
}

func (m *memRedirects) List(context.Context) ([]entity.Redirect, error) {
	return append([]entity.Redirect(nil), m.rows...), nil
}

func (m *memRedirects) Search(context.Context, string, int, int) ([]entity.Redirect, int, error) {
	return append([]entity.Redirect(nil), m.rows...), len(m.rows), nil
}

func (m *memRedirects) Create(_ context.Context, r entity.Redirect) (*entity.Redirect, error) {
	m.nextID++
	r.ID = m.nextID
	r.ToPath = "/" + r.ToPath
	m.rows = append(m.rows, r)
	return &r, nil
}

func (m *memRedirects) Delete(_ context.Context, id int) error {
	for i, r := range m.rows {
		if r.ID == id {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			break
		}
	}
	return nil
}

func (m *memRedirects) DeleteByFromPath(_ context.Context, from string) error {
	kept := m.rows[:0]
	for _, r := range m.rows {
		if r.FromPath != from {
			kept = append(kept, r)
		}
	}
	m.rows = kept
	return nil
}

func TestRedirectMoveSlug(t *testing.T) {
	ctx := context.Background()
	repo := &memRedirects{}
	s := NewRedirectService(repo, nil)

	if err := s.MoveSlug(ctx, "news/old", "news/new"); err != nil {
		t.Fatal(err)
	}
	if to, ok := s.GetRedirect("news/old"); !ok || to != "/news/new" {
		t.Fatalf("old → %q %v", to, ok)
	}

	// Changed back: the new address must not redirect to the old one.
	if err := s.MoveSlug(ctx, "news/new", "news/old"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GetRedirect("news/old"); ok {
		t.Error("news/old still redirects: a loop")
	}
	if to, ok := s.GetRedirect("news/new"); !ok || to != "/news/old" {
		t.Errorf("new → %q %v", to, ok)
	}
	if len(repo.rows) != 1 {
		t.Errorf("redirects: %+v", repo.rows)
	}

	// Unchanged or empty slugs leave nothing.
	for _, c := range [][2]string{{"news/old", "/news/old/"}, {"", "news/x"}, {"news/x", ""}} {
		if err := s.MoveSlug(ctx, c[0], c[1]); err != nil {
			t.Fatal(err)
		}
	}
	if len(repo.rows) != 1 {
		t.Errorf("no-op moves added redirects: %+v", repo.rows)
	}
}
