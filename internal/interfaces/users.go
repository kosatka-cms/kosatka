package interfaces

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/domain/accounts"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/sociallogin"
	"github.com/kosatka-cms/kosatka/internal/domain/twofactor"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// UserController serves the account pages: sign-in (and its second
// factor), sign-out, sign-up, email confirmation, password reset,
// invitations, the profiles, their editing and cancelling an account.
type UserController struct {
	*BaseController
	// The services and repositories a page needs (users, auth, templates,
	// the store, the logger, site information) are the BaseController's.

	// userTimezones - the request time zone cache (middleware); a
	// profile save drops the user's entry (UserDeps).
	userTimezones     interface{ Forget(userID int) }
	fileUploadService *files.FileUploadService
	roleRepo          entity.RoleRepository
	userTokenService  *accounts.UserAuthTokenService
	// profileViews/profileTabs - the profile's View tabs (UserDeps).
	profileViews views.Service
	profileTabs  profileTabRenderer
	// userFields - admin-defined profile fields (UserDeps).
	userFields *accounts.UserFieldService
	// registration - the sign-up form's accounts (UserDeps).
	registration *accounts.RegistrationService
	// passwordReset - the forgotten password's link and new password
	// (UserDeps).
	passwordReset *accounts.PasswordResetService
	// twoFactor - two-factor sign-in (UserDeps; nil - none).
	twoFactor *twofactor.Service
	// socialLogin - the social networks connected to an account
	// (UserDeps; nil - none).
	socialLogin *sociallogin.Service
	// cancel, accountPolicy - deleting one's own account and what it
	// leaves (UserDeps; nil - not offered).
	cancel        *service.UserDeletionService
	accountPolicy *accounts.AccountPolicyService
}

// UserDeps - what NewUserController needs besides the BaseDeps.
type UserDeps struct {
	FileUploadService *files.FileUploadService
	RoleRepo          entity.RoleRepository
	UserTokenService  *accounts.UserAuthTokenService
	UserFields        *accounts.UserFieldService
	ProfileViews      views.Service
	ProfileTabs       profileTabRenderer
	UserTimezones     interface{ Forget(userID int) }
	Registration      *accounts.RegistrationService
	PasswordReset     *accounts.PasswordResetService
	TwoFactor         *twofactor.Service
	SocialLogin       *sociallogin.Service
	// AccountCancel, AccountPolicy - a user deleting their own account.
	AccountCancel *service.UserDeletionService
	AccountPolicy *accounts.AccountPolicyService
}

// NewUserController creates a UserController on deps and d.
func NewUserController(deps BaseDeps, d UserDeps) *UserController {
	base := NewBaseController(deps)
	base.logger = deps.Logger.With("component", "users.handler")

	return &UserController{
		BaseController:    base,
		fileUploadService: d.FileUploadService,
		roleRepo:          d.RoleRepo,
		userTokenService:  d.UserTokenService,
		userFields:        d.UserFields,
		profileViews:      d.ProfileViews,
		profileTabs:       d.ProfileTabs,
		userTimezones:     d.UserTimezones,
		registration:      d.Registration,
		twoFactor:         d.TwoFactor,
		socialLogin:       d.SocialLogin,
		passwordReset:     d.PasswordReset,
		cancel:            d.AccountCancel,
		accountPolicy:     d.AccountPolicy,
	}
}

// captchaFormError - a user-visible message when the CAPTCHA is on for
// form (entity.CaptchaForm*) and its response doesn't verify; "" when it
// passes or is off.
func (c *UserController) captchaFormError(ctx context.Context, r *http.Request, form string) string {
	ok, err := captcha.Verify(ctx, form, r.FormValue("g-recaptcha-response"), httputil.ClientIP(r))
	if err != nil {
		c.LogFailure(err, "recaptcha verify")
		return i18n.T(ctx, "Could not verify the CAPTCHA. Please try again later.")
	}
	if !ok {
		return i18n.T(ctx, "Please confirm that you are not a robot.")
	}
	return ""
}

// Profile - GET /me, /me/{tab}, /users/{alias} and /users/{alias}/{tab}:
// a user's profile (/me - the signed-in user's) with its tabs: the files
// (/me/files), a module's (AddProfileTab) or a View's (user_tab display).
func (c *UserController) Profile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	ctx := r.Context()

	// Slug from /users/{alias} (entity.UserPathPrefix).
	slug := r.PathValue("alias")

	var user *entity.User
	var err error
	var isCurrentUser bool

	if slug == "" {
		// /me shows the current user's profile without a redirect.
		userID, loggedIn := c.authService.GetUserID(r)
		if !loggedIn {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		user, err = c.userService.ByID(ctx, userID)
		if err != nil {
			c.LogFailure(err, "error getting user by id", "user_id", userID)
			httputil.InternalServerError(w)
			return
		}
		isCurrentUser = true
	} else {
		slug = entity.UserSlugFromAlias(slug)
		user, err = c.userService.FindBySlug(ctx, slug)
		if err != nil {
			if apperrors.IsNotFound(err) {
				c.RenderNotFound(w, r)
				return
			}
			c.LogFailure(err, "error finding user by slug", "slug", slug)
			httputil.InternalServerError(w)
			return
		}

		if userID, loggedIn := c.authService.GetUserID(r); loggedIn && userID == user.ID {
			isCurrentUser = true
		}
	}

	// Can current user edit this profile? (self or admin)
	canEdit := isCurrentUser
	isAdmin := false
	currentUser, err := c.authService.GetCurrentUser(ctx, r)
	if err == nil {
		isAdmin = c.authService.HasPermission(ctx, currentUser, service.PermAdministerUsers)
		if isAdmin {
			canEdit = true
		}
	} else {
		currentUser = nil
	}

	// Blocked users are hidden from their normal profile URL - only the
	// account owner (edge case: blocked mid-session) and admins can still see it.
	if user.Status == entity.UserStatusBlocked && !isCurrentUser && !isAdmin {
		c.RenderNotFound(w, r)
		return
	}

	activeTab := "overview"
	path := r.URL.Path

	// /me/files - the current user's files (file manager).
	if path == "/me/files" {
		activeTab = "files"
	}

	// A /{tab} URL: a module's tab (AddProfileTab) - 403 when the visitor
	// may not open it - else one of the View tabs below.
	seg := r.PathValue("tab")
	var moduleTab ProfileTab
	isModuleTab := false
	if seg != "" {
		moduleTab, isModuleTab = enabledProfileTab(seg)
	}
	if isModuleTab && !profileTabAllowed(ctx, moduleTab, currentUser, user, isCurrentUser, isAdmin) {
		c.renderProfileTabForbidden(w, r, user, moduleTab, currentUser == nil)
		return
	}

	// data["user"] is the logged-in user (header).
	data := c.GetCommonData(r)
	profileForPage := *user
	profileForPage.Avatar = imageurl.Avatar(user.Avatar, imageurl.AvatarProfileStyle)
	data["profileUser"] = &profileForPage
	data["profileFields"] = c.userFieldValues(r.Context(), user.ID)
	if isCurrentUser {
		data["user"] = user // small header preview (user_avatar_list)
	}
	// The tabs this visitor may see - the modules', then the Views'
	// (user_tab displays); a /{tab} URL opens one of them. Tab links share
	// the profile's base URL (profile_page.html).
	profileBase := "/" + user.Slug
	if isCurrentUser {
		profileBase = "/me"
	}
	moduleTabs := c.profileModuleTabs(ctx, currentUser, user, isCurrentUser, isAdmin, profileBase, seg)
	viewTabs := c.profileViewTabs(ctx, user.ID, isCurrentUser || isAdmin, profileBase)
	var activeViewTab *profileViewTab
	if seg != "" && !isModuleTab {
		for i := range viewTabs {
			if viewTabs[i].display.ProfileTabPath == seg {
				activeViewTab = &viewTabs[i]
				viewTabs[i].Active = true
			}
		}
		if activeViewTab == nil {
			c.RenderNotFound(w, r)
			return
		}
		activeTab = "view"
	}
	if isModuleTab {
		activeTab = "module"
		data["activeModuleTab"] = moduleTab.Module
	}

	data["activeTab"] = activeTab
	data["profileModuleTabs"] = moduleTabs
	data["profileViewTabs"] = viewTabs
	data["isCurrentUser"] = isCurrentUser
	data["canEdit"] = canEdit
	data["title"] = ""
	c.setPathTrail(data, "/"+profileForPage.Slug, profileForPage.Username)

	data["pageSEO"] = PageSEOForUserProfile(&profileForPage, r.URL.Path)
	if activeTab == "files" {
		filesSEO := PageSEOForUserProfileTab(&profileForPage, i18n.T(ctx, "Files"), r.URL.Path)
		filesSEO.NoIndex = true
		data["pageSEO"] = filesSEO
		data["CSRFToken"] = csrf.EnsureToken(c.store, w, r)
	}
	if activeViewTab != nil {
		page := c.GetPageNumber(r)
		html, _, err := c.profileTabs.RenderProfileTab(ctx, activeViewTab.display, user.ID, page, r.URL.Path, r.URL.Query())
		if err != nil {
			c.LogFailure(err, "error rendering profile view tab", "user_id", user.ID, "view_id", activeViewTab.display.ID)
		}
		data["profileTabHTML"] = html
		data["pageSEO"] = PageSEOForUserProfileTab(&profileForPage, activeViewTab.Label, r.URL.Path)
	}
	if isModuleTab {
		// The module's Load may change the SEO (e.g. not indexed).
		data["pageSEO"] = PageSEOForUserProfileTab(&profileForPage, i18n.T(ctx, moduleTab.Label), r.URL.Path)
		if moduleTab.Load != nil {
			if err := moduleTab.Load(ctx, w, r, user, data); err != nil {
				c.LogFailure(err, "error loading a module's profile tab", "module", moduleTab.Module, "user_id", user.ID)
			}
		}
	}

	if activeTab == "overview" {
		c.renderOverviewTab(ctx, data, user)
	}

	c.RenderOrFail(w, r, "user/profile_page.html", data)
}

// profileTabAllowed - whether viewer (nil - a guest) may open the
// module's tab on profile's page.
func profileTabAllowed(ctx context.Context, t ProfileTab, viewer, profile *entity.User, isOwner, canAdminister bool) bool {
	return t.Allowed == nil || t.Allowed(ctx, viewer, profile, isOwner, canAdminister)
}

// profileModuleTab - a module's tab (AddProfileTab) as the profile page
// lists it.
type profileModuleTab struct {
	Module string
	Label  string
	URL    string
	Badge  int
	Active bool
}

// profileModuleTabs - the enabled modules' tabs viewer may open on
// profile's page (base - its URL), the one at active marked.
func (c *UserController) profileModuleTabs(ctx context.Context, viewer, profile *entity.User, isOwner, canAdminister bool, base, active string) []profileModuleTab {
	var out []profileModuleTab
	for _, t := range profileTabs {
		if !service.ModuleEnabled(t.Module) || !profileTabAllowed(ctx, t, viewer, profile, isOwner, canAdminister) {
			continue
		}
		tab := profileModuleTab{Module: t.Module, Label: i18n.T(ctx, t.Label), URL: strings.TrimSuffix(base, "/") + "/" + t.Path, Active: t.Path == active}
		// The badge counts the viewer's own (unread messages…): on their
		// own profile only.
		if isOwner && viewer != nil && t.Badge != nil {
			tab.Badge = t.Badge(ctx, viewer)
		}
		out = append(out, tab)
	}
	return out
}

// renderProfileTabForbidden renders a 403 for a module's profile tab the
// visitor may not open: a guest is offered to log in or register.
func (c *UserController) renderProfileTabForbidden(w http.ResponseWriter, r *http.Request, profileUser *entity.User, tab ProfileTab, anonymous bool) {
	ctx := r.Context()
	data := c.GetCommonData(r)
	profileForPage := *profileUser
	profileForPage.Avatar = imageurl.Avatar(profileUser.Avatar, imageurl.AvatarProfileStyle)
	label := i18n.T(ctx, tab.Label)
	data["profileUser"] = &profileForPage
	data["profileTabLabel"] = label
	data["title"] = label
	c.setChildTrail(data, []entity.BreadcrumbItem{{URL: "/" + profileForPage.Slug, Label: profileForPage.Username}}, label)
	data["pageSEO"] = PageSEOForList(i18n.T(ctx, "Access to \"@tab\" is restricted", "@tab", label), r.URL.Path)
	data["profileTabForbiddenAnonymous"] = anonymous
	data["profileTabForbiddenDestination"] = url.QueryEscape(r.URL.Path)

	var buf bytes.Buffer
	if err := c.templateService.Render(&buf, "user/profile_tab_forbidden.html", data, r, nil); err != nil {
		c.LogFailure(err, "error rendering the profile tab forbidden template")
		httputil.TemplateExecutionError(w, err)
		return
	}
	w.WriteHeader(http.StatusForbidden)
	_, _ = buf.WriteTo(w)
}

// SearchUsers handles AJAX requests to search for users (for autocomplete):
// ?q= (2-50 characters), &roles=1,2 - of those roles only (a user field's
// picker). Requires authentication; never exposes email addresses.
func (c *UserController) SearchUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
		return
	}

	ctx := r.Context()
	viewer, _ := c.GetCurrentUser(r)

	query := r.URL.Query().Get("q")
	if n := utf8.RuneCountInString(query); n < 2 || n > 50 {
		_ = json.NewEncoder(w).Encode(map[string]any{"users": []any{}})
		return
	}
	// Of some roles - a user field's choice in a content form: only for
	// who writes content or works in the admin; another member would
	// learn who has which role.
	var roles []int
	if c.authService.HasPermission(ctx, viewer, service.PermAccessAdmin) || len(c.authService.CreatableNodeTypes(ctx, viewer)) > 0 {
		for _, s := range strings.Split(r.URL.Query().Get("roles"), ",") {
			if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && id > 0 {
				roles = append(roles, id)
			}
		}
	}

	users, err := c.userService.Search(ctx, query, roles, 10)
	if err != nil {
		c.LogFailure(err, "error searching users", "query", query)
		httputil.InternalServerError(w)
		return
	}

	type UserResponse struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}

	var response []UserResponse
	if users != nil {
		for _, user := range *users {
			response = append(response, UserResponse{
				ID:       user.ID,
				Username: user.Username,
			})
		}
	} else {
		response = []UserResponse{}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"users": response})
}

func (c *UserController) renderOverviewTab(ctx context.Context, data map[string]any, user *entity.User) {
	duration := time.Since(user.CreatedAt)
	years := int(duration.Hours() / 24 / 365)
	months := int(duration.Hours()/24/30) % 12

	var membershipDuration string
	if years > 0 {
		membershipDuration = i18n.Plural(ctx, years, "@count year", "@count years") + " " + i18n.Plural(ctx, months, "@count month", "@count months")
	} else if months > 0 {
		membershipDuration = i18n.Plural(ctx, months, "@count month", "@count months")
	} else {
		membershipDuration = i18n.T(ctx, "Recently joined")
	}
	data["membershipDuration"] = membershipDuration
}

// profileTabRenderer - renders and counts user_tab Views for a profile
// (implemented by DynamicContentController).
type profileTabRenderer interface {
	RenderProfileTab(ctx context.Context, view entity.ViewDisplayDef, userID, page int, basePath string, query url.Values) (template.HTML, int, error)
	ProfileTabCount(ctx context.Context, view entity.ViewDisplayDef, userID int) (int, error)
}

// profileViewTab - a user_tab View shown as a profile tab.
type profileViewTab struct {
	display entity.ViewDisplayDef
	Label   string
	URL     string
	Count   int
	// Public - visible to every visitor (also shown in the profile's
	// stats bar).
	Public bool
	Active bool
}

// profileViewTabs - the user_tab displays this visitor may see on the
// profile of userID (owner-only ones only for the owner and admins),
// with their result counts; base is the profile's own URL.
func (c *UserController) profileViewTabs(ctx context.Context, userID int, ownerOrAdmin bool, base string) []profileViewTab {
	if c.profileViews == nil || c.profileTabs == nil {
		return nil
	}
	displays, err := c.profileViews.UserTabDisplays(ctx)
	if err != nil {
		c.LogFailure(err, "error loading profile view tabs")
		return nil
	}
	var tabs []profileViewTab
	for _, d := range displays {
		// A module's tab has the path (AddProfileTab): it is served.
		if _, ok := enabledProfileTab(d.ProfileTabPath); ok {
			continue
		}
		public := d.ProfileTabAccess != entity.ViewProfileTabAccessOwner
		if !public && !ownerOrAdmin {
			continue
		}
		count, err := c.profileTabs.ProfileTabCount(ctx, d, userID)
		if err != nil {
			c.LogFailure(err, "error counting profile view tab", "view_id", d.ID)
		}
		tabs = append(tabs, profileViewTab{display: d, Label: d.Label, URL: strings.TrimSuffix(base, "/") + "/" + d.ProfileTabPath, Count: count, Public: public})
	}
	return tabs
}
