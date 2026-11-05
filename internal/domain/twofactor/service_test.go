package twofactor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// memRepo - entity.TwoFactorRepository in memory.
type memRepo struct {
	rows  map[int]*entity.TwoFactor
	codes map[int]map[string]bool // hash → used
}

func newMemRepo() *memRepo {
	return &memRepo{rows: map[int]*entity.TwoFactor{}, codes: map[int]map[string]bool{}}
}

func (m *memRepo) ByUser(_ context.Context, id int) (*entity.TwoFactor, error) {
	r, ok := m.rows[id]
	if !ok {
		return nil, nil
	}
	cp := *r
	cp.CodesLeft = 0
	for _, used := range m.codes[id] {
		if !used {
			cp.CodesLeft++
		}
	}
	return &cp, nil
}

func (m *memRepo) Enable(_ context.Context, id int, secret string, step int64, hashes []string, at time.Time) error {
	m.rows[id] = &entity.TwoFactor{UserID: id, Secret: secret, LastStep: step, EnabledAt: at}
	return m.ReplaceRecoveryCodes(context.Background(), id, hashes)
}

func (m *memRepo) AcceptStep(_ context.Context, id int, step int64) (bool, error) {
	if r := m.rows[id]; r != nil && r.LastStep < step {
		r.LastStep = step
		return true, nil
	}
	return false, nil
}

func (m *memRepo) UseRecoveryCode(_ context.Context, id int, hash string) (bool, error) {
	if used, ok := m.codes[id][hash]; ok && !used {
		m.codes[id][hash] = true
		return true, nil
	}
	return false, nil
}

func (m *memRepo) ReplaceRecoveryCodes(_ context.Context, id int, hashes []string) error {
	m.codes[id] = map[string]bool{}
	for _, h := range hashes {
		m.codes[id][h] = false
	}
	return nil
}

func (m *memRepo) Disable(_ context.Context, id int) error {
	delete(m.rows, id)
	delete(m.codes, id)
	return nil
}

func (m *memRepo) EnabledAmong(_ context.Context, ids []int) (map[int]bool, error) {
	out := map[int]bool{}
	for _, id := range ids {
		if m.rows[id] != nil {
			out[id] = true
		}
	}
	return out, nil
}

type stubSettings struct{ s entity.AccountSettings }

func (s *stubSettings) Settings(context.Context) (*entity.AccountSettings, error) {
	cp := s.s
	return &cp, nil
}

// TestService - turning on takes the app's code (the secret stays off
// otherwise); a code at sign-in is taken once, the next step's too; a
// recovery code is spent; new codes replace the old; turning off takes a
// code and isn't allowed while the role needs it; an administrator's
// reset is; the issuer is the settings', else the site's.
func TestService(t *testing.T) {
	ctx := context.Background()
	repo := newMemRepo()
	settings := &stubSettings{s: entity.AccountSettings{TwoFactorRoles: []int{3}}}
	svc := NewService(repo, settings, func(context.Context) string { return "Example Site" })
	now := time.Unix(1700000000, 0)
	svc.now = func() time.Time { return now }
	ann := entity.User{ID: 7, Username: "ann", RoleID: 2}

	secret, _ := NewSecret()
	setup, err := svc.SetupFor(ctx, ann, secret)
	if err != nil || setup.Issuer != "Example Site" || setup.Account != "ann" || setup.QR == "" {
		t.Fatalf("setup %+v %v", setup, err)
	}
	if _, err := svc.Enable(ctx, ann, secret, "000000"); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("a wrong code turned it on: %v", err)
	}
	at := Step(now)
	codes, err := svc.Enable(ctx, ann, secret, Code(secret, at))
	if err != nil || len(codes) != RecoveryCodes {
		t.Fatalf("enable: %v %v", codes, err)
	}
	if ok, _ := svc.Verify(ctx, ann.ID, Code(secret, at)); ok {
		t.Error("the code that turned it on taken again")
	}
	if ok, _ := svc.Verify(ctx, ann.ID, Code(secret, at+1)); !ok {
		t.Error("the next step's code refused")
	}
	if ok, _ := svc.Verify(ctx, ann.ID, codes[0]); !ok {
		t.Error("a recovery code refused")
	}
	if ok, _ := svc.Verify(ctx, ann.ID, codes[0]); ok {
		t.Error("a recovery code taken twice")
	}
	if st, _ := svc.Status(ctx, ann.ID); st == nil || st.CodesLeft != RecoveryCodes-1 {
		t.Errorf("status %+v", st)
	}
	fresh, err := svc.NewRecoveryCodes(ctx, ann, codes[1])
	if err != nil || len(fresh) != RecoveryCodes {
		t.Fatalf("new codes: %v", err)
	}
	if ok, _ := svc.Verify(ctx, ann.ID, codes[2]); ok {
		t.Error("an old code still works")
	}
	if err := svc.Disable(ctx, ann, "000000"); !errors.Is(err, ErrWrongCode) {
		t.Errorf("turned off with a wrong code: %v", err)
	}
	if err := svc.Disable(ctx, ann, fresh[0]); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if on, _ := svc.Enabled(ctx, ann.ID); on {
		t.Error("still on")
	}

	// A role that needs it: can't turn it off, but an administrator's
	// reset does; set up is needed while it's off.
	bob := entity.User{ID: 8, Username: "bob", Role: &entity.Role{ID: 3}}
	if needed, _ := svc.SetupNeeded(ctx, bob); !needed {
		t.Error("bob's role needs it")
	}
	if needed, _ := svc.SetupNeeded(ctx, ann); needed {
		t.Error("ann's role doesn't")
	}
	secret2, _ := NewSecret()
	if _, err := svc.Enable(ctx, bob, secret2, Code(secret2, at)); err != nil {
		t.Fatal(err)
	}
	if err := svc.Disable(ctx, bob, Code(secret2, at+1)); !errors.Is(err, ErrRequired) {
		t.Errorf("a required one turned off: %v", err)
	}
	if err := svc.Reset(ctx, bob); err != nil {
		t.Fatal(err)
	}
	if on, _ := svc.Enabled(ctx, bob.ID); on {
		t.Error("reset left it on")
	}

	// The roles are kept until the settings are saved again.
	settings.s.TwoFactorRoles = nil
	if !svc.Required(ctx, bob) {
		t.Error("the roles reloaded without Invalidate")
	}
	svc.Invalidate()
	if svc.Required(ctx, bob) {
		t.Error("the roles kept after Invalidate")
	}
	settings.s.TwoFactorIssuer = "SA"
	if s, _ := svc.SetupFor(ctx, ann, secret); s.Issuer != "SA" {
		t.Errorf("issuer %q", s.Issuer)
	}
}

// TestRequiredForTheBaseRole - required for the role every signed-in user
// has, two-factor sign-in is everyone's, whatever their own role; the base
// role's id is read once.
func TestRequiredForTheBaseRole(t *testing.T) {
	ctx := context.Background()
	settings := &stubSettings{s: entity.AccountSettings{TwoFactorRoles: []int{2}}}
	svc := NewService(newMemRepo(), settings, func(context.Context) string { return "Example Site" })
	editor := entity.User{ID: 9, Username: "eve", Role: &entity.Role{ID: 5}}
	if svc.Required(ctx, editor) {
		t.Error("required without the base role known")
	}
	reads := 0
	svc.SetBaseRole(func(context.Context) (int, error) { reads++; return 2, nil })
	// Twice: the base role's id is read once.
	for range 2 {
		if !svc.Required(ctx, editor) {
			t.Error("the base role required, another role's account not")
		}
	}
	if reads != 1 {
		t.Errorf("the base role read %d times, want once", reads)
	}
	settings.s.TwoFactorRoles = []int{7}
	svc.Invalidate()
	if svc.Required(ctx, editor) {
		t.Error("required though neither its role nor the base role is")
	}
}
