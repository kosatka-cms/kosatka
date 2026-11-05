package sociallogin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/internal/lib/oauth"
	"golang.org/x/oauth2"
)

type stubOAuthRepo struct {
	byProvider map[string]*entity.UserOAuthIdentity
	created    []entity.UserOAuthIdentity
}

func (s *stubOAuthRepo) FindByProviderUser(_ context.Context, provider, providerUserID string) (*entity.UserOAuthIdentity, error) {
	key := provider + ":" + providerUserID
	if id, ok := s.byProvider[key]; ok {
		return id, nil
	}
	return nil, apperrors.ErrNotFound
}

func (s *stubOAuthRepo) FindByUserAndProvider(_ context.Context, userID int, provider string) (*entity.UserOAuthIdentity, error) {
	for _, id := range s.byProvider {
		if id.UserID == userID && id.Provider == provider {
			return id, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (s *stubOAuthRepo) ByUser(_ context.Context, userID int) ([]entity.UserOAuthIdentity, error) {
	var out []entity.UserOAuthIdentity
	for _, id := range s.byProvider {
		if id.UserID == userID {
			out = append(out, *id)
		}
	}
	return out, nil
}

func (s *stubOAuthRepo) Delete(_ context.Context, userID int, provider string) error {
	for key, id := range s.byProvider {
		if id.UserID == userID && id.Provider == provider {
			delete(s.byProvider, key)
		}
	}
	return nil
}

func (s *stubOAuthRepo) Create(_ context.Context, identity entity.UserOAuthIdentity) (*entity.UserOAuthIdentity, error) {
	identity.ID = int64(len(s.created) + 1)
	s.created = append(s.created, identity)
	key := identity.Provider + ":" + identity.ProviderUserID
	s.byProvider[key] = &identity
	return &identity, nil
}

type stubUserServiceOAuth struct {
	users   map[int]*entity.User
	byEmail map[string]*entity.User
	byName  map[string]*entity.User
	nextID  int
}

func (s *stubUserServiceOAuth) Find(_ context.Context, f entity.UserFilter) (*entity.User, error) {
	if u, ok := s.byEmail[f.Email]; ok {
		return u, nil
	}
	return nil, apperrors.ErrNotFound
}

func (s *stubUserServiceOAuth) FindByEmailOrUsername(context.Context, string) (*entity.User, error) {
	return nil, apperrors.ErrNotFound
}

func (s *stubUserServiceOAuth) ByID(_ context.Context, id int) (*entity.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, apperrors.ErrNotFound
}

func (s *stubUserServiceOAuth) FindBySlug(context.Context, string) (*entity.User, error) {
	return nil, apperrors.ErrNotFound
}

func (s *stubUserServiceOAuth) FindByUsername(_ context.Context, username string) (*entity.User, error) {
	if u, ok := s.byName[username]; ok {
		return u, nil
	}
	return nil, apperrors.ErrNotFound
}

func (s *stubUserServiceOAuth) Edit(context.Context, entity.User) (*entity.User, error) {
	return nil, nil
}

func (s *stubUserServiceOAuth) Create(_ context.Context, u entity.User) (*entity.User, error) {
	s.nextID++
	u.ID = s.nextID
	cp := u
	s.users[u.ID] = &cp
	s.byEmail[u.Email] = &cp
	s.byName[u.Username] = &cp
	return &cp, nil
}

func (s *stubUserServiceOAuth) List(context.Context) (*entity.Users, error) { return nil, nil }
func (s *stubUserServiceOAuth) Delete(context.Context, int) error           { return nil }
func (s *stubUserServiceOAuth) ByIDs(context.Context, []int) (map[int]entity.User, error) {
	return nil, nil
}
func (s *stubUserServiceOAuth) Search(context.Context, string, []int, int) (*entity.Users, error) {
	return nil, nil
}
func (s *stubUserServiceOAuth) ConfirmEmail(context.Context, int) (*entity.User, error) {
	return nil, nil
}
func (s *stubUserServiceOAuth) CountWithStatus(context.Context, int) (int, error)    { return 0, nil }
func (s *stubUserServiceOAuth) BumpSessionVersion(context.Context, int) (int, error) { return 1, nil }

type stubRoleRepoOAuth struct {
	role entity.Role
}

func (s *stubRoleRepoOAuth) ByName(context.Context, string) (*entity.Role, error) {
	return &s.role, nil
}
func (s *stubRoleRepoOAuth) ByID(context.Context, int) (*entity.Role, error) { return &s.role, nil }
func (s *stubRoleRepoOAuth) Find(context.Context, entity.RoleFilter) (*entity.Role, error) {
	return &s.role, nil
}
func (s *stubRoleRepoOAuth) List(context.Context) (*entity.Roles, error) { return nil, nil }
func (s *stubRoleRepoOAuth) Create(context.Context, entity.Role) (*entity.Role, error) {
	return &s.role, nil
}
func (s *stubRoleRepoOAuth) Update(context.Context, entity.Role) (*entity.Role, error) {
	return &s.role, nil
}
func (s *stubRoleRepoOAuth) Delete(context.Context, int) error            { return nil }
func (s *stubRoleRepoOAuth) CountUsers(context.Context, int) (int, error) { return 0, nil }

type stubAuthOAuth struct {
	loggedUserID int
}

func (s *stubAuthOAuth) Login(context.Context, http.ResponseWriter, *http.Request, string, string) (*entity.User, error) {
	return nil, nil
}
func (s *stubAuthOAuth) LoginUser(_ context.Context, _ http.ResponseWriter, _ *http.Request, user *entity.User) error {
	s.loggedUserID = user.ID
	return nil
}
func (s *stubAuthOAuth) Logout(context.Context, http.ResponseWriter, *http.Request) error { return nil }
func (s *stubAuthOAuth) EndSessions(context.Context, http.ResponseWriter, *http.Request, int) error {
	return nil
}
func (s *stubAuthOAuth) GetUserID(*http.Request) (int, bool) {
	return s.loggedUserID, s.loggedUserID > 0
}
func (s *stubAuthOAuth) GetCurrentUser(context.Context, *http.Request) (*entity.User, error) {
	return nil, nil
}
func (s *stubAuthOAuth) CanViewNode(context.Context, *entity.Node, *entity.User) bool { return false }
func (s *stubAuthOAuth) ContentVisitor(context.Context, *entity.User) (entity.ContentVisitor, error) {
	return entity.ContentVisitor{}, nil
}
func (s *stubAuthOAuth) CanEditNode(context.Context, *entity.Node, *entity.User) bool { return false }
func (s *stubAuthOAuth) CanCreateNode(context.Context, entity.ContentType, *entity.User) bool {
	return false
}
func (s *stubAuthOAuth) CanDeleteNode(context.Context, *entity.Node, *entity.User) bool { return false }
func (s *stubAuthOAuth) CanPublishNode(context.Context, *entity.User) bool              { return false }
func (s *stubAuthOAuth) HasPermission(context.Context, *entity.User, string) bool       { return false }
func (s *stubAuthOAuth) CreatableNodeTypes(context.Context, *entity.User) []entity.CreatableMaterialLink {
	return nil
}
func (s *stubAuthOAuth) CanUseSiteInMaintenanceMode(context.Context, *entity.User) bool { return false }
func (s *stubAuthOAuth) CanUseTextFormat(context.Context, *entity.User, entity.TextFormatDef) bool {
	return false
}
func (s *stubAuthOAuth) VerifyPassword(context.Context, int, string) (bool, error) { return false, nil }

func (s *stubAuthOAuth) PendingSecondFactor(context.Context, *http.Request) (*entity.User, bool) {
	return nil, false
}

func (s *stubAuthOAuth) CompleteSecondFactor(context.Context, http.ResponseWriter, *http.Request, string) (*entity.User, error) {
	return nil, nil
}

func (s *stubAuthOAuth) CancelSecondFactor(http.ResponseWriter, *http.Request) {}

type stubAccountSettingsOAuth struct {
	settings *entity.AccountSettings
}

func (s *stubAccountSettingsOAuth) Get(context.Context) (*entity.AccountSettings, error) {
	return s.settings, nil
}
func (s *stubAccountSettingsOAuth) Update(context.Context, *entity.AccountSettings) error { return nil }

type mockOAuthProvider struct {
	name    string
	profile *oauth.Profile
}

func (m *mockOAuthProvider) Name() string { return m.name }
func (m *mockOAuthProvider) AuthCodeURL(string) string {
	return "https://example.com/auth"
}
func (m *mockOAuthProvider) Exchange(context.Context, string) (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "tok"}, nil
}
func (m *mockOAuthProvider) UserInfo(context.Context, *oauth2.Token) (*oauth.Profile, error) {
	return m.profile, nil
}

type testRegistry struct {
	p oauth.Provider
}

func (t *testRegistry) Get(name string) (oauth.Provider, bool) {
	if t.p != nil && t.p.Name() == name {
		return t.p, true
	}
	return nil, false
}

func (t *testRegistry) EnabledNames() []string {
	if t.p == nil {
		return nil
	}
	return []string{t.p.Name()}
}

func TestStateMismatch(t *testing.T) {
	svc := NewService(nil, &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{}}, nil, nil, &stubAuthOAuth{}, nil, nil, nil, slog.New(slog.DiscardHandler))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := svc.HandleCallback(context.Background(), w, r, "google", "code", "state", "other", "", "", "")
	if !errors.Is(err, ErrStateMismatch) {
		t.Fatalf("expected state mismatch, got %v", err)
	}
}

func TestUniqueUsername(t *testing.T) {
	us := &stubUserServiceOAuth{
		users:   map[int]*entity.User{},
		byEmail: map[string]*entity.User{},
		byName:  map[string]*entity.User{"ivan": {ID: 1, Username: "ivan"}},
	}
	svc := NewService(nil, nil, us, nil, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	name, err := svc.uniqueUsername(context.Background(), "ivan", "")
	if err != nil {
		t.Fatal(err)
	}
	if name == "ivan" {
		t.Fatal("expected suffixed username")
	}
}

func TestCreateUserPendingApproval(t *testing.T) {
	repo := &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{}}
	us := &stubUserServiceOAuth{users: map[int]*entity.User{}, byEmail: map[string]*entity.User{}, byName: map[string]*entity.User{}}
	svc := NewService(nil, repo, us, &stubRoleRepoOAuth{role: entity.Role{ID: 1}}, &stubAuthOAuth{}, &stubAccountSettingsOAuth{
		settings: &entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitorsApproval},
	}, nil, nil, slog.New(slog.DiscardHandler))

	user, outcome, err := svc.createOAuthUser(context.Background(), &entity.AccountSettings{
		RegistrationMode: entity.RegistrationModeVisitorsApproval,
	}, entity.OAuthProviderGoogle, &oauth.Profile{ProviderUserID: "1", Email: "new@example.com", DisplayName: "New"}, "new@example.com", "127.0.0.1", "Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if outcome != accounts.SignupPendingApproval || user.Status != entity.UserStatusPending {
		t.Fatalf("expected pending user, got outcome=%v status=%d", outcome, user.Status)
	}
	if user.Timezone != "Asia/Tokyo" {
		t.Errorf("a new user gets the browser's time zone: %q", user.Timezone)
	}
}

func TestHandleCallback_ExistingIdentity(t *testing.T) {
	user := &entity.User{ID: 5, Username: "u", Email: "a@b.com", Status: entity.UserStatusActive, EmailVerified: true}
	repo := &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{
		"google:sub-1": {UserID: 5, Provider: "google", ProviderUserID: "sub-1"},
	}}
	us := &stubUserServiceOAuth{users: map[int]*entity.User{5: user}}
	auth := &stubAuthOAuth{}
	svc := NewService(&testRegistry{p: &mockOAuthProvider{
		name:    "google",
		profile: &oauth.Profile{ProviderUserID: "sub-1", Email: "a@b.com"},
	}}, repo, us, nil, auth, nil, nil, nil, slog.New(slog.DiscardHandler))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=x&state=s", nil)
	result, err := svc.HandleCallback(context.Background(), w, r, "google", "x", "s", "s", "/me", "", "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	if result.RedirectTo != "/me" {
		t.Fatalf("redirect %q", result.RedirectTo)
	}
	if auth.loggedUserID != 5 {
		t.Fatalf("expected login user 5, got %d", auth.loggedUserID)
	}
	if us.users[5].Timezone != "" {
		t.Errorf("signing in changed an existing user's time zone: %q", us.users[5].Timezone)
	}
}

func TestHandleCallback_RegisterDisabled(t *testing.T) {
	repo := &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{}}
	us := &stubUserServiceOAuth{users: map[int]*entity.User{}, byEmail: map[string]*entity.User{}, byName: map[string]*entity.User{}}
	svc := NewService(&testRegistry{p: &mockOAuthProvider{
		name:    "google",
		profile: &oauth.Profile{ProviderUserID: "new-1", Email: "x@y.com", DisplayName: "X"},
	}}, repo, us, &stubRoleRepoOAuth{role: entity.Role{ID: 1}}, &stubAuthOAuth{}, &stubAccountSettingsOAuth{
		settings: &entity.AccountSettings{RegistrationMode: entity.RegistrationModeAdminOnly},
	}, nil, nil, slog.New(slog.DiscardHandler))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := svc.HandleCallback(context.Background(), w, r, "google", "c", "s", "s", "", "", "")
	if !errors.Is(err, ErrRegisterDisabled) {
		t.Fatalf("expected register disabled, got %v", err)
	}
}

// TestHandleCallback_EmailTaken - a provider's account with an existing
// account's e-mail is never linked to it by the address (whoever
// registered the address here first would get the provider's sign-ins),
// whatever the provider says of it and the registration settings: its
// owner signs in with the password and connects the provider.
func TestHandleCallback_EmailTaken(t *testing.T) {
	for _, tc := range []struct {
		verified bool
		mode     string
	}{{false, entity.RegistrationModeVisitors}, {true, entity.RegistrationModeVisitors}, {true, entity.RegistrationModeAdminOnly}} {
		existing := &entity.User{ID: 10, Username: "member", Email: "user@example.com", Status: entity.UserStatusActive, EmailVerified: true}
		repo := &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{}}
		us := &stubUserServiceOAuth{
			users:   map[int]*entity.User{10: existing},
			byEmail: map[string]*entity.User{"user@example.com": existing},
			byName:  map[string]*entity.User{},
		}
		auth := &stubAuthOAuth{}
		svc := NewService(&testRegistry{p: &mockOAuthProvider{
			name:    "google",
			profile: &oauth.Profile{ProviderUserID: "g-1", Email: "user@example.com", EmailVerified: tc.verified},
		}}, repo, us, &stubRoleRepoOAuth{role: entity.Role{ID: 1}}, auth, &stubAccountSettingsOAuth{
			settings: &entity.AccountSettings{RegistrationMode: tc.mode},
		}, nil, nil, slog.New(slog.DiscardHandler))
		_, err := svc.HandleCallback(context.Background(), httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), "google", "c", "s", "s", "", "", "")
		if !errors.Is(err, ErrEmailTaken) || len(repo.created) != 0 || auth.loggedUserID != 0 || len(us.users) != 1 {
			t.Errorf("%+v: err %v, links %+v, signed in %d, users %d", tc, err, repo.created, auth.loggedUserID, len(us.users))
		}
	}
}

// TestHandleCallback_Connect - signed in, the provider's account is
// connected to theirs; not one connected to another account, nor a
// second account of the provider.
func TestHandleCallback_Connect(t *testing.T) {
	callback := func(repo *stubOAuthRepo, signedIn int) (*Result, error) {
		svc := NewService(&testRegistry{p: &mockOAuthProvider{
			name:    "google",
			profile: &oauth.Profile{ProviderUserID: "g-1", Email: "Other@Example.com"},
		}}, repo, &stubUserServiceOAuth{users: map[int]*entity.User{}}, nil, &stubAuthOAuth{loggedUserID: signedIn}, nil, nil, nil, slog.New(slog.DiscardHandler))
		return svc.HandleCallback(context.Background(), httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), "google", "c", "s", "s", "/me/edit", "", "")
	}
	repo := &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{}}
	result, err := callback(repo, 10)
	if err != nil || !result.Connected || len(repo.created) != 1 || repo.created[0].UserID != 10 || repo.created[0].Email != "other@example.com" {
		t.Fatalf("connect: %+v, %v, links %+v", result, err, repo.created)
	}
	if result, err := callback(repo, 10); err != nil || result.Connected || len(repo.created) != 1 {
		t.Errorf("connected again: %+v, %v", result, err)
	}
	if _, err := callback(repo, 11); !errors.Is(err, ErrConnectedElsewhere) {
		t.Errorf("another user's: %v", err)
	}
	other := &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{"google:g-2": {UserID: 10, Provider: "google", ProviderUserID: "g-2"}}}
	if _, err := callback(other, 10); !errors.Is(err, ErrProviderConnected) {
		t.Errorf("a second account of the provider: %v", err)
	}
}

// TestHandleCallback_ConfirmEmail - where the site signs in confirmed
// addresses only, a new account whose address the provider doesn't
// vouch for waits for its confirmation link; a provider giving no address
// is sent to the sign-up form; elsewhere such an address is kept
// unconfirmed.
func TestHandleCallback_ConfirmEmail(t *testing.T) {
	callback := func(acc *entity.AccountSettings, profile *oauth.Profile) (*Result, *stubUserServiceOAuth, *stubAuthOAuth, error) {
		us := &stubUserServiceOAuth{users: map[int]*entity.User{}, byEmail: map[string]*entity.User{}, byName: map[string]*entity.User{}}
		auth := &stubAuthOAuth{}
		svc := NewService(&testRegistry{p: &mockOAuthProvider{name: "vk", profile: profile}},
			&stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{}}, us, &stubRoleRepoOAuth{role: entity.Role{ID: 1}}, auth,
			&stubAccountSettingsOAuth{settings: acc}, nil, nil, slog.New(slog.DiscardHandler))
		result, err := svc.HandleCallback(context.Background(), httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), "vk", "c", "s", "s", "", "", "")
		return result, us, auth, err
	}
	confirm := &entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors, RequireEmailVerification: true}
	result, us, auth, err := callback(confirm, &oauth.Profile{ProviderUserID: "vk-1", Email: "new@example.com", DisplayName: "New"})
	user := us.byEmail["new@example.com"]
	if err != nil || !result.ConfirmEmail || result.PendingEmail != "new@example.com" || user == nil || user.EmailVerified || auth.loggedUserID != 0 {
		t.Fatalf("an unconfirmed address: %+v, %v, user %+v, signed in %d", result, err, user, auth.loggedUserID)
	}
	if _, us, _, err := callback(confirm, &oauth.Profile{ProviderUserID: "vk-2", DisplayName: "Nameless"}); !errors.Is(err, ErrEmailNeeded) || len(us.users) != 0 {
		t.Errorf("no address: %v, users %d", err, len(us.users))
	}
	_, us, auth, err = callback(&entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitors}, &oauth.Profile{ProviderUserID: "vk-3", Email: "open@example.com", DisplayName: "Open"})
	if user := us.byEmail["open@example.com"]; err != nil || user == nil || user.EmailVerified || auth.loggedUserID != user.ID {
		t.Errorf("an address the provider doesn't vouch for, no confirmation needed: %v, %+v", err, user)
	}
}

// TestHandleCallback_PendingSignUpLinksProvider - a
// sign-up awaiting approval gets the provider link right away: before the
// approval the provider brings back the "awaiting approval" page, after it
// signs in to the account (whatever the provider says of the e-mail).
func TestHandleCallback_PendingSignUpLinksProvider(t *testing.T) {
	repo := &stubOAuthRepo{byProvider: map[string]*entity.UserOAuthIdentity{}}
	us := &stubUserServiceOAuth{users: map[int]*entity.User{}, byEmail: map[string]*entity.User{}, byName: map[string]*entity.User{}}
	auth := &stubAuthOAuth{}
	svc := NewService(&testRegistry{p: &mockOAuthProvider{
		name:    "vk",
		profile: &oauth.Profile{ProviderUserID: "vk-7", Email: "newcomer@example.com", DisplayName: "Newcomer"},
	}}, repo, us, &stubRoleRepoOAuth{role: entity.Role{ID: 1}}, auth, &stubAccountSettingsOAuth{
		settings: &entity.AccountSettings{RegistrationMode: entity.RegistrationModeVisitorsApproval},
	}, nil, nil, slog.New(slog.DiscardHandler))
	callback := func() (*Result, error) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		return svc.HandleCallback(context.Background(), w, r, "vk", "c", "s", "s", "/me", "", "")
	}

	result, err := callback()
	if err != nil || !result.PendingApproval || result.PendingEmail != "newcomer@example.com" {
		t.Fatalf("sign-up: result %+v, err %v", result, err)
	}
	user := us.byEmail["newcomer@example.com"]
	if user == nil || user.Status != entity.UserStatusPending {
		t.Fatalf("no pending account: %+v", user)
	}
	if len(repo.created) != 1 || repo.created[0].UserID != user.ID || repo.created[0].ProviderUserID != "vk-7" {
		t.Fatalf("the pending account isn't linked to the provider: %+v", repo.created)
	}
	if auth.loggedUserID != 0 {
		t.Fatalf("a pending account signed in: %d", auth.loggedUserID)
	}

	result, err = callback()
	if err != nil || !result.PendingApproval || auth.loggedUserID != 0 || len(repo.created) != 1 {
		t.Fatalf("before approval: result %+v, err %v, signed in %d, links %d", result, err, auth.loggedUserID, len(repo.created))
	}

	user.Status = entity.UserStatusActive
	result, err = callback()
	if err != nil || result.PendingApproval || result.RedirectTo != "/me" || auth.loggedUserID != user.ID {
		t.Fatalf("after approval: result %+v, err %v, signed in %d", result, err, auth.loggedUserID)
	}
}

func TestCompleteProfileURL(t *testing.T) {
	got := oauthCompleteProfileURL("/forum")
	if got != "/user/complete-profile?destination=%2Fforum" {
		t.Fatalf("got %q", got)
	}
}

// profileFieldsRepo - one required profile field, and the users who have
// a value of it.
type profileFieldsRepo struct {
	entity.UserFieldRepository
	filled map[int]bool
}

func (r profileFieldsRepo) Fields(context.Context) (entity.UserFieldDefs, error) {
	return entity.UserFieldDefs{{ID: 1, MachineName: "city", Label: "City", Required: true}}, nil
}

func (r profileFieldsRepo) Values(_ context.Context, userID int) (map[int]string, error) {
	if r.filled[userID] {
		return map[int]string{1: "Paris"}, nil
	}
	return map[int]string{}, nil
}

// TestNeedsCompleteProfile - an OAuth sign-up's user completes the
// profile when a required profile field has no value of theirs; without
// the profile fields set, nothing to complete.
func TestNeedsCompleteProfile(t *testing.T) {
	ctx := context.Background()
	s := &Service{}
	if s.NeedsCompleteProfile(ctx, &entity.User{ID: 8}) {
		t.Error("without the profile fields set, nothing to complete")
	}
	s.SetUserFields(accounts.NewUserFieldService(profileFieldsRepo{filled: map[int]bool{7: true}}))
	if !s.NeedsCompleteProfile(ctx, &entity.User{ID: 8}) || s.NeedsCompleteProfile(ctx, &entity.User{ID: 7}) {
		t.Error("NeedsCompleteProfile follows the required fields")
	}
}
