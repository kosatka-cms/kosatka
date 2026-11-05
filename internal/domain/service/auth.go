package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/audit"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"golang.org/x/crypto/bcrypt"
)

// Login errors shown on the login form (i18n.ErrorText).
var (
	errLoginWrongLoginOrPassword = i18n.Errorf("Unrecognized username, email address or password.")
	// ErrLoginFailed - the login couldn't be checked (the database, the
	// session), not the visitor's mistake; its cause is joined to it.
	ErrLoginFailed       = i18n.Errorf("Login failed. Please try again later.")
	errLoginSessionError = i18n.Errorf("Could not save the session. Please try again.")
	// ErrSecondFactorNeeded - the password (or a link) was right and the
	// user has two-factor sign-in on: no session yet, the code page asks
	// for the code (CompleteSecondFactor). Not shown to anyone.
	ErrSecondFactorNeeded = errors.New("the second factor is needed")
	errSecondFactorWrong  = i18n.Errorf("The code is wrong or already used. Try the next one.")
	errSecondFactorGone   = i18n.Errorf("Too many wrong codes, or too much time has passed. Sign in again.")
)

// SecondFactorPath - the page asking for the code of a pending sign-in.
const SecondFactorPath = "/login/code"

// SecondFactorURL - the code page, then dest (a path on the site; "" -
// the code page's default).
func SecondFactorURL(dest string) string {
	if dest = SafeRedirectPath(dest); dest == "" {
		return SecondFactorPath
	}
	return SecondFactorPath + "?destination=" + url.QueryEscape(dest)
}

// The second step of a sign-in: how long it waits for the code, how many
// wrong codes an account takes in that time.
const (
	secondFactorTTL   = 10 * time.Minute
	secondFactorTries = 5
)

// secondFactorFailures - each account's wrong two-factor codes of the last
// secondFactorTTL. Kept on the server: a count in the browser's session
// would start again from a copy of the cookie sent as it was.
type secondFactorFailures struct {
	mu sync.Mutex
	at map[int][]time.Time
}

// recent - userID's failures since now-secondFactorTTL (mu held).
func (f *secondFactorFailures) recent(userID int, now time.Time) []time.Time {
	var out []time.Time
	for _, t := range f.at[userID] {
		if now.Sub(t) < secondFactorTTL {
			out = append(out, t)
		}
	}
	return out
}

// count - userID's recent failures.
func (f *secondFactorFailures) count(userID int, now time.Time) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.recent(userID, now))
}

// add records a failure of userID: the recent ones with it.
func (f *secondFactorFailures) add(userID int, now time.Time) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.at == nil {
		f.at = map[int][]time.Time{}
	}
	f.at[userID] = append(f.recent(userID, now), now)
	return len(f.at[userID])
}

// clear forgets userID's failures (a right code).
func (f *secondFactorFailures) clear(userID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.at, userID)
}

// secondFactor - two-factor sign-in (twofactor.Service): whether a user
// has it on, and a code checked (each taken once).
type secondFactor interface {
	Enabled(ctx context.Context, userID int) (bool, error)
	Verify(ctx context.Context, userID int, code string) (bool, error)
}

// AuthService - signing in and out (the password, a link, the second
// factor), the request's user, and what a user may do with the content
// (the role permissions, the content access).
type AuthService interface {
	Login(ctx context.Context, w http.ResponseWriter, r *http.Request, loginIdentity, password string) (*entity.User, error)
	// LoginUser establishes a session for an already authenticated user record (OAuth, email confirm, etc.).
	LoginUser(ctx context.Context, w http.ResponseWriter, r *http.Request, user *entity.User) error
	Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error
	// EndSessions ends every session of user userID (a new password,
	// two-factor sign-in turned on or off): on the next request each is
	// signed out. The request's own session, when it is that user's, is
	// kept.
	EndSessions(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int) error
	GetUserID(r *http.Request) (int, bool)
	GetCurrentUser(ctx context.Context, r *http.Request) (*entity.User, error)
	CanViewNode(ctx context.Context, node *entity.Node, user *entity.User) bool
	// ContentVisitor - what of the content user may not view (the closed
	// types, the items with view roles they lack; ContentAccess): their
	// requests' lists leave it out (entity.WithContentVisitor).
	ContentVisitor(ctx context.Context, user *entity.User) (entity.ContentVisitor, error)
	CanEditNode(ctx context.Context, node *entity.Node, user *entity.User) bool
	CanCreateNode(ctx context.Context, contentType entity.ContentType, user *entity.User) bool
	CanDeleteNode(ctx context.Context, node *entity.Node, user *entity.User) bool
	CanPublishNode(ctx context.Context, user *entity.User) bool
	// HasPermission reports whether user's role grants the permission.
	HasPermission(ctx context.Context, user *entity.User, permission string) bool
	// CreatableNodeTypes - node types user may create (the "Create content" links).
	CreatableNodeTypes(ctx context.Context, user *entity.User) []entity.CreatableMaterialLink
	CanUseSiteInMaintenanceMode(ctx context.Context, user *entity.User) bool
	CanUseTextFormat(ctx context.Context, user *entity.User, format entity.TextFormatDef) bool
	VerifyPassword(ctx context.Context, userID int, password string) (bool, error)
	// PendingSecondFactor - the user whose sign-in waits for the code in
	// this browser (Login and LoginUser answered ErrSecondFactorNeeded).
	PendingSecondFactor(ctx context.Context, r *http.Request) (*entity.User, bool)
	// CompleteSecondFactor checks the code of the pending sign-in: right -
	// the session, the user signed in; wrong - an error to show (after
	// too many, or too late, the sign-in starts over).
	CompleteSecondFactor(ctx context.Context, w http.ResponseWriter, r *http.Request, code string) (*entity.User, error)
	// CancelSecondFactor forgets the pending sign-in.
	CancelSecondFactor(w http.ResponseWriter, r *http.Request)
}

// DefaultAuthService - the AuthService on the users and the session
// cookie store; the permissions, the content access and the second factor
// it is given by wiring (the Set methods).
type DefaultAuthService struct {
	userService UserService
	store       sessions.CookieStore
	policy      emailVerificationPolicy
	// perms - role permissions (see SetPermissions); nil fails every
	// permission check.
	perms *PermissionService
	// second - two-factor sign-in (SetSecondFactor); nil - none.
	second secondFactor
	// access - who may view each content type (SetContentAccess); nil -
	// everyone.
	access *ContentAccess
	// failures - the accounts' recent wrong two-factor codes.
	failures secondFactorFailures
}

// SetContentAccess gives the service the content types' view access
// (wiring).
func (s *DefaultAuthService) SetContentAccess(a *ContentAccess) {
	s.access = a
}

// ContentVisitor - see AuthService.ContentVisitor.
func (s *DefaultAuthService) ContentVisitor(ctx context.Context, user *entity.User) (entity.ContentVisitor, error) {
	if s.access == nil {
		return entity.ContentVisitor{}, nil
	}
	return s.access.Visitor(ctx, user)
}

// SetSecondFactor gives the service two-factor sign-in (wiring).
func (s *DefaultAuthService) SetSecondFactor(sf secondFactor) {
	s.second = sf
}

// emailVerificationPolicy - whether a sign-in needs a confirmed email
// (accounts.AccountPolicyService, the account settings).
type emailVerificationPolicy interface {
	RequireEmailVerification(ctx context.Context) (bool, error)
}

// NewAuthService - the auth service on us (the users), the session store
// s and policy (whether a sign-in needs a confirmed email; nil - it
// doesn't).
func NewAuthService(us UserService, s sessions.CookieStore, policy emailVerificationPolicy) AuthService {
	return &DefaultAuthService{
		userService: us,
		store:       s,
		policy:      policy,
	}
}

// Login signs in by a username or an email and the password: the user,
// signed in, or ErrSecondFactorNeeded with two-factor sign-in on; a wrong
// login or password, a blocked or unconfirmed account - an error to show.
func (s *DefaultAuthService) Login(ctx context.Context, w http.ResponseWriter, r *http.Request, loginIdentity, password string) (*entity.User, error) {
	user, err := s.userService.FindByEmailOrUsername(ctx, loginIdentity)
	if err != nil {
		// The repository maps ent's not-found to apperrors.ErrNotFound or
		// entity.ErrEntityNotFound, so no raw ent error reaches here.
		if errors.Is(err, apperrors.ErrNotFound) || errors.Is(err, entity.ErrEntityNotFound) {
			// The time a password check takes, as for an account: the
			// answer's delay doesn't tell whether the login exists.
			_ = bcrypt.CompareHashAndPassword(absentAccountHash(), []byte(password))
			auditLoginFailed(ctx, loginIdentity, nil, i18n.T(ctx, "no such user"))
			return nil, errLoginWrongLoginOrPassword
		}
		return nil, fmt.Errorf("finding the user to log in: %w", errors.Join(ErrLoginFailed, err))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		auditLoginFailed(ctx, loginIdentity, user, i18n.T(ctx, "wrong password"))
		return nil, errLoginWrongLoginOrPassword
	}

	if err := s.validateUserForLogin(ctx, user); err != nil {
		auditLoginFailed(ctx, loginIdentity, user, i18n.ErrorText(ctx, err))
		return nil, err
	}
	if err := s.startSession(ctx, r, w, user); err != nil {
		return user, err
	}
	return user, nil
}

// startSession signs user in - or, with two-factor sign-in on, keeps the
// sign-in pending until the code (ErrSecondFactorNeeded). Every way in
// (the password, a link, a social network) goes through here.
func (s *DefaultAuthService) startSession(ctx context.Context, r *http.Request, w http.ResponseWriter, user *entity.User) error {
	if s.second != nil {
		on, err := s.second.Enabled(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("checking the two-factor sign-in (user %d): %w", user.ID, errors.Join(ErrLoginFailed, err))
		}
		if on {
			if err := s.savePending(r, w, user.ID); err != nil {
				return fmt.Errorf("saving the pending sign-in (user %d): %w", user.ID, errors.Join(ErrLoginFailed, err))
			}
			return ErrSecondFactorNeeded
		}
	}
	if err := s.saveUserSession(r, w, user); err != nil {
		return fmt.Errorf("saving the session (user %d): %w", user.ID, errors.Join(ErrLoginFailed, err))
	}
	s.auditLogin(ctx, user)
	return nil
}

// absentAccountHash - a password hash no password matches, checked for a
// login with no account (made once, at the cost real hashes have).
var absentAccountHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("no account has this password"), bcrypt.DefaultCost)
	if err != nil {
		// Only an over-long password or a bad cost fail: neither here.
		panic(err)
	}
	return hash
})

// The pending sign-in's values in the session: whose, since when.
const (
	pendingUserKey = "second_factor_user"
	pendingAtKey   = "second_factor_at"
)

// savePending keeps userID's sign-in waiting for the code; 0 - forgets
// it.
func (s *DefaultAuthService) savePending(r *http.Request, w http.ResponseWriter, userID int) error {
	session, err := s.store.Get(r, entity.SessionName)
	if err != nil {
		return errLoginSessionError
	}
	if userID == 0 {
		delete(session.Values, pendingUserKey)
		delete(session.Values, pendingAtKey)
	} else {
		session.Values[pendingAtKey] = time.Now().Unix()
		session.Values[pendingUserKey] = userID
	}
	if err := session.Save(r, w); err != nil {
		return errLoginSessionError
	}
	return nil
}

// pending - the pending sign-in's user, if one waits and hasn't expired.
func (s *DefaultAuthService) pending(r *http.Request) (userID int, ok bool) {
	session, err := s.store.Get(r, entity.SessionName)
	if err != nil {
		return 0, false
	}
	userID, _ = session.Values[pendingUserKey].(int)
	at, _ := session.Values[pendingAtKey].(int64)
	if userID == 0 || time.Since(time.Unix(at, 0)) > secondFactorTTL {
		return 0, false
	}
	return userID, true
}

// PendingSecondFactor - see AuthService.PendingSecondFactor.
func (s *DefaultAuthService) PendingSecondFactor(ctx context.Context, r *http.Request) (*entity.User, bool) {
	userID, ok := s.pending(r)
	if !ok {
		return nil, false
	}
	user, err := s.userService.ByID(ctx, userID)
	if err != nil || user == nil {
		return nil, false
	}
	return user, true
}

// CompleteSecondFactor - see AuthService.CompleteSecondFactor.
func (s *DefaultAuthService) CompleteSecondFactor(ctx context.Context, w http.ResponseWriter, r *http.Request, code string) (*entity.User, error) {
	userID, ok := s.pending(r)
	if !ok || s.second == nil {
		return nil, errSecondFactorGone
	}
	// Too many wrong codes of late: no more tries until they age out.
	if s.failures.count(userID, time.Now()) >= secondFactorTries {
		_ = s.savePending(r, w, 0)
		return nil, errSecondFactorGone
	}
	user, err := s.userService.ByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("loading the pending sign-in's user %d: %w", userID, errors.Join(ErrLoginFailed, err))
	}
	right, err := s.second.Verify(ctx, userID, code)
	if err != nil {
		return nil, fmt.Errorf("checking the two-factor code (user %d): %w", userID, errors.Join(ErrLoginFailed, err))
	}
	if !right {
		auditLoginFailed(ctx, user.Username, user, i18n.T(ctx, "wrong two-factor code"))
		if s.failures.add(userID, time.Now()) >= secondFactorTries {
			_ = s.savePending(r, w, 0)
			return nil, errSecondFactorGone
		}
		return nil, errSecondFactorWrong
	}
	s.failures.clear(userID)
	// The account may have been blocked meanwhile.
	if err := s.validateUserForLogin(ctx, user); err != nil {
		_ = s.savePending(r, w, 0)
		return nil, err
	}
	if err := s.savePending(r, w, 0); err != nil {
		return nil, err
	}
	if err := s.saveUserSession(r, w, user); err != nil {
		return nil, fmt.Errorf("saving the session (user %d): %w", user.ID, errors.Join(ErrLoginFailed, err))
	}
	s.auditLogin(ctx, user)
	return user, nil
}

// CancelSecondFactor - see AuthService.CancelSecondFactor.
func (s *DefaultAuthService) CancelSecondFactor(w http.ResponseWriter, r *http.Request) {
	_ = s.savePending(r, w, 0)
}

// auditLoginFailed records a failed sign-in for the action log: the name
// typed, the account when there is one, why.
func auditLoginFailed(ctx context.Context, typed string, user *entity.User, reason string) {
	obj := audit.Object{Kind: audit.KindUser, Label: PlainExcerpt(typed, 100)}
	if user != nil {
		obj = userObject(*user, true)
	}
	audit.Record(ctx, audit.Action{Type: audit.LoginFailed, Object: obj, Changes: []audit.Change{
		{Field: i18n.T(ctx, "Reason"), To: reason},
	}})
}

// auditLogin records the sign-in of a user with access to the admin area
// for the action log (others' sign-ins aren't kept).
func (s *DefaultAuthService) auditLogin(ctx context.Context, user *entity.User) {
	if !s.HasPermission(ctx, user, PermAccessAdmin) {
		return
	}
	audit.Record(audit.WithUser(ctx, user.ID), audit.Action{Type: audit.Login, Object: userObject(*user, true)})
}

// LoginUser - see AuthService.LoginUser.
func (s *DefaultAuthService) LoginUser(ctx context.Context, w http.ResponseWriter, r *http.Request, user *entity.User) error {
	if user == nil {
		return ErrLoginFailed
	}
	if err := s.validateUserForLogin(ctx, user); err != nil {
		return err
	}
	return s.startSession(ctx, r, w, user)
}

func (s *DefaultAuthService) validateUserForLogin(ctx context.Context, user *entity.User) error {
	if user.Status == 0 {
		return i18n.Errorf("The account has been blocked")
	}
	if user.Status == entity.UserStatusPending {
		return i18n.Errorf("The account is awaiting administrator approval")
	}
	requireVerify := false
	if s.policy != nil {
		v, err := s.policy.RequireEmailVerification(ctx)
		if err != nil {
			return fmt.Errorf("checking whether a sign-in needs a confirmed e-mail (user %d): %w", user.ID, errors.Join(ErrLoginFailed, err))
		}
		requireVerify = v
	}
	if requireVerify && !user.EmailVerified {
		return i18n.Errorf("Please confirm your email address")
	}
	return nil
}

// SessionVersionKey - the session value holding the account's session
// version (entity.User.SessionVersion) it was started with.
const SessionVersionKey = "session_version"

func (s *DefaultAuthService) saveUserSession(r *http.Request, w http.ResponseWriter, user *entity.User) error {
	session, err := s.store.Get(r, entity.SessionName)
	if err != nil {
		return errLoginSessionError
	}
	session.Values["user_id"] = user.ID
	session.Values[SessionVersionKey] = user.SessionVersion
	if err := session.Save(r, w); err != nil {
		return errLoginSessionError
	}
	return nil
}

// EndSessions - see AuthService.EndSessions.
func (s *DefaultAuthService) EndSessions(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int) error {
	version, err := s.userService.BumpSessionVersion(ctx, userID)
	if err != nil {
		return err
	}
	if current, ok := s.GetUserID(r); !ok || current != userID {
		return nil
	}
	session, err := s.store.Get(r, entity.SessionName)
	if err != nil {
		return errLoginSessionError
	}
	session.Values[SessionVersionKey] = version
	if err := session.Save(r, w); err != nil {
		return fmt.Errorf("keeping the session of user %d: %w", userID, err)
	}
	return nil
}

// Logout ends the request's session (its cookie expires); an unreadable
// session counts as already signed out.
func (s *DefaultAuthService) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	session, err := s.store.Get(r, entity.SessionName)
	if err != nil {
		// An unreadable session (e.g. after a secret key change) means the
		// user is already effectively logged out.
		return nil
	}

	session.Options.MaxAge = -1

	if err := session.Save(r, w); err != nil {
		return err
	}

	return nil
}

// GetUserID - the ID of the user signed in by the request's session;
// false - none (or the session can't be read).
func (s *DefaultAuthService) GetUserID(r *http.Request) (int, bool) {
	session, err := s.store.Get(r, entity.SessionName)
	if err != nil {
		// An unreadable session (e.g. after a secret key change) is treated as
		// not authenticated rather than an error.
		return 0, false
	}

	userID, ok := session.Values["user_id"].(int)
	if !ok || userID == 0 {
		return 0, false
	}

	return userID, true
}

// GetCurrentUser returns the currently logged-in user.
func (s *DefaultAuthService) GetCurrentUser(ctx context.Context, r *http.Request) (*entity.User, error) {
	userID, ok := s.GetUserID(r)
	if !ok {
		return nil, fmt.Errorf("user not authenticated")
	}

	user, err := s.userService.ByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// SetPermissions gives the service the role permissions (wiring).
func (s *DefaultAuthService) SetPermissions(p *PermissionService) {
	s.perms = p
}

// HasPermission reports whether user has the permission.
func (s *DefaultAuthService) HasPermission(ctx context.Context, user *entity.User, permission string) bool {
	return s.perms != nil && s.perms.HasPermission(ctx, user, permission)
}

// CreatableNodeTypes delegates to PermissionService.CreatableNodeTypes.
func (s *DefaultAuthService) CreatableNodeTypes(ctx context.Context, user *entity.User) []entity.CreatableMaterialLink {
	if s.perms == nil {
		return nil
	}
	return s.perms.CreatableNodeTypes(ctx, user)
}

// CanViewNode - the node's type lets user view its content (or they may
// edit the node), so do its own view roles (or it is theirs, or they may
// edit it or view every content), and it is published, or they may view
// unpublished content, or it is their own on moderation.
func (s *DefaultAuthService) CanViewNode(ctx context.Context, node *entity.Node, user *entity.User) bool {
	// In the trash: nobody's (the trash's page lists it, nothing shows it).
	if node.InTrash() {
		return false
	}
	if s.access != nil {
		ok, err := s.access.CanViewType(ctx, user, node.Type.ToString())
		if err != nil {
			// Closed, not open: the type may be restricted.
			LogFallback(slog.Default(), err, "checking the view access of a content type", "type", node.Type)
		}
		if !ok && !s.CanEditNode(ctx, node, user) {
			return false
		}
	}
	if !node.ViewableBy(entity.UserRoleNames(user)) && (user == nil || user.ID != node.CreatedBy.ID) &&
		!s.HasPermission(ctx, user, PermBypassContentAccess) && !s.CanEditNode(ctx, node, user) {
		return false
	}
	if node.IsPublic() {
		return true
	}

	if s.HasPermission(ctx, user, PermViewUnpublished) {
		return true
	}

	if user != nil && user.ID == node.CreatedBy.ID && node.Status == 1 && node.OnModeration == 1 {
		return true
	}

	return false
}

// CanEditNode - "edit any <type> content", or "edit own <type>
// content" for the author of a published node.
func (s *DefaultAuthService) CanEditNode(ctx context.Context, node *entity.Node, user *entity.User) bool {
	if user == nil {
		return false
	}
	nodeType := node.Type.ToString()
	if s.HasPermission(ctx, user, PermEditAny(nodeType)) {
		return true
	}
	return node.CreatedBy.ID == user.ID && node.Status == 1 && s.HasPermission(ctx, user, PermEditOwn(nodeType))
}

// CanCreateNode - "create <type> content".
func (s *DefaultAuthService) CanCreateNode(ctx context.Context, contentType entity.ContentType, user *entity.User) bool {
	return s.HasPermission(ctx, user, PermCreate(contentType.ToString()))
}

// CanDeleteNode - "delete any <type> content".
func (s *DefaultAuthService) CanDeleteNode(ctx context.Context, node *entity.Node, user *entity.User) bool {
	return s.HasPermission(ctx, user, PermDeleteAny(node.Type.ToString()))
}

// CanPublishNode - "publish content".
func (s *DefaultAuthService) CanPublishNode(ctx context.Context, user *entity.User) bool {
	return s.HasPermission(ctx, user, PermPublishContent)
}

// CanUseSiteInMaintenanceMode - "use_maintenance_mode".
func (s *DefaultAuthService) CanUseSiteInMaintenanceMode(ctx context.Context, user *entity.User) bool {
	return s.HasPermission(ctx, user, PermUseMaintenance)
}

// CanUseTextFormat reports whether user may save a field value in format
// ("use_text_format_<machine_name>"). A format that allows script/style
// (format.AllowUnsafeContent) is full-access only regardless of role grants;
// see entity.TextFormatDef.AllowUnsafeContent for why.
func (s *DefaultAuthService) CanUseTextFormat(ctx context.Context, user *entity.User, format entity.TextFormatDef) bool {
	if format.AllowUnsafeContent() {
		return s.HasPermission(ctx, user, PermFullAccess)
	}
	return s.HasPermission(ctx, user, PermUseTextFormat(format.MachineName))
}

// VerifyPassword checks if the given password matches the user's stored hash.
func (s *DefaultAuthService) VerifyPassword(ctx context.Context, userID int, password string) (bool, error) {
	user, err := s.userService.ByID(ctx, userID)
	if err != nil {
		return false, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil, nil
}
