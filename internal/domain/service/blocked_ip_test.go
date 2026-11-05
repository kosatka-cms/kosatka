package service

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeBlockedIPRepo struct{ list []entity.BlockedIP }

func (r *fakeBlockedIPRepo) List(context.Context) ([]entity.BlockedIP, error) { return r.list, nil }

func (r *fakeBlockedIPRepo) Create(_ context.Context, ip, _ string) (*entity.BlockedIP, error) {
	b := entity.BlockedIP{ID: len(r.list) + 1, IP: ip}
	r.list = append(r.list, b)
	return &b, nil
}

func (r *fakeBlockedIPRepo) Delete(_ context.Context, id int) error {
	for i, b := range r.list {
		if b.ID == id {
			r.list = append(r.list[:i], r.list[i+1:]...)
		}
	}
	return nil
}

// TestBlockedIPs - an address, an IPv4 and an IPv6 network block what
// they cover; a pattern that is neither blocks nothing; a deleted one
// stops blocking.
func TestBlockedIPs(t *testing.T) {
	ctx := context.Background()
	s := NewBlockedIPService(&fakeBlockedIPRepo{})
	for _, p := range []string{"192.0.2.7", "198.51.100.0/24", "2001:db8::/32", "not an address"} {
		if err := s.Create(ctx, p, ""); err != nil {
			t.Fatal(err)
		}
	}
	for ip, want := range map[string]bool{
		"192.0.2.7": true, "192.0.2.8": false, "198.51.100.200": true, "198.51.101.1": false,
		"2001:db8::1": true, "2001:db9::1": false, "": false, "garbage": false,
	} {
		if got := s.IsBlocked(ip); got != want {
			t.Errorf("%q: blocked %v, want %v", ip, got, want)
		}
	}
	if err := s.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if s.IsBlocked("192.0.2.7") {
		t.Error("blocked after its deletion")
	}
}
