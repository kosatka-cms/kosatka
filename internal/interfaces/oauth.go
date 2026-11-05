package interfaces

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/sociallogin"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

const (
	sessionKeyOAuthState       = "oauth_state"
	sessionKeyOAuthDestination = "oauth_destination"
	// sessionKeyOAuthTimezone - the browser's time zone the sign-in link
	// carried (?tz=, social_login.html), for a user the callback creates.
	sessionKeyOAuthTimezone = "oauth_timezone"
)

// OAuthController serves the social sign-in: going to a provider, its
// callback, completing the profile it left unfilled and disconnecting a
// provider's account.
type OAuthController struct {
	*BaseController
	oauthService    *sociallogin.Service
	userService     service.UserService
	templateService service.TemplateService
	store           *sessions.CookieStore
	logger          *slog.Logger
}

// NewOAuthController creates an OAuthController on the social sign-in
// service, the users, the templates and the session store.
func NewOAuthController(
	base *BaseController,
	oauthService *sociallogin.Service,
	userService service.UserService,
	templateService service.TemplateService,
	store *sessions.CookieStore,
	logger *slog.Logger,
) *OAuthController {
	return &OAuthController{
		BaseController:  base,
		oauthService:    oauthService,
		userService:     userService,
		templateService: templateService,
		store:           store,
		logger:          logger.With("component", "oauth.handler"),
	}
}

// OAuthStart handles GET /auth/{provider}: off to the provider - to sign
// in, or, signed in, to connect its account to theirs (the callback
// decides).
func (c *OAuthController) OAuthStart(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	state, err := c.oauthService.NewState()
	if err != nil {
		c.LogFailure(err, "oauth start: making the state")
		c.redirectOAuthError(w, r)
		return
	}

	session, err := c.store.Get(r, entity.SessionName)
	if err != nil {
		c.LogFailure(err, "oauth start: loading the session")
		c.redirectOAuthError(w, r)
		return
	}
	session.Values[sessionKeyOAuthState] = state
	session.Values[sessionKeyOAuthDestination] = service.SafeRedirectPath(r.URL.Query().Get("destination"))
	session.Values[sessionKeyOAuthTimezone] = userTimezoneFromForm(r.URL.Query().Get("tz"), "")
	if err := session.Save(r, w); err != nil {
		c.LogFailure(err, "oauth start: saving the session")
		c.redirectOAuthError(w, r)
		return
	}

	authURL, err := c.oauthService.AuthURL(providerName, state)
	if err != nil {
		if errors.Is(err, sociallogin.ErrProviderUnknown) {
			http.NotFound(w, r)
			return
		}
		c.LogFailure(err, "oauth start: the provider's address", "provider", providerName)
		c.redirectOAuthError(w, r)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OAuthCallback handles GET /auth/{provider}/callback: the provider's
// answer checked against the session's state, then the user signed in,
// signed up or their account connected (sociallogin.Service.HandleCallback);
// a refusal goes to the sign-in or profile form with its message.
func (c *OAuthController) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	ctx := r.Context()

	if errParam := strings.TrimSpace(r.URL.Query().Get("error")); errParam != "" {
		c.redirectOAuthError(w, r)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || state == "" {
		c.redirectOAuthError(w, r)
		return
	}

	session, err := c.store.Get(r, entity.SessionName)
	if err != nil {
		c.LogFailure(err, "oauth callback: loading the session", "provider", providerName)
		c.redirectOAuthError(w, r)
		return
	}
	sessionState, _ := session.Values[sessionKeyOAuthState].(string)
	destination, _ := session.Values[sessionKeyOAuthDestination].(string)
	timezone, _ := session.Values[sessionKeyOAuthTimezone].(string)
	delete(session.Values, sessionKeyOAuthState)
	delete(session.Values, sessionKeyOAuthDestination)
	delete(session.Values, sessionKeyOAuthTimezone)
	if err := session.Save(r, w); err != nil && c.logger != nil {
		c.logger.Warn("oauth callback: session save failed", "provider", providerName, "error", err)
	}

	result, err := c.oauthService.HandleCallback(
		ctx, w, r, providerName, code, state, sessionState, destination, httputil.ClientIP(r), timezone,
	)
	if err != nil {
		if errors.Is(err, sociallogin.ErrRegisterDisabled) {
			http.Redirect(w, r, "/login?msg=register_disabled", http.StatusFound)
			return
		}
		if errors.Is(err, sociallogin.ErrEmailTaken) {
			http.Redirect(w, r, "/login?msg=oauth_email_taken", http.StatusFound)
			return
		}
		if errors.Is(err, sociallogin.ErrEmailNeeded) {
			http.Redirect(w, r, "/login?msg=oauth_no_email", http.StatusFound)
			return
		}
		// Connecting from the profile: its page says why not.
		if errors.Is(err, sociallogin.ErrConnectedElsewhere) {
			http.Redirect(w, r, "/me/edit?oauth=elsewhere", http.StatusFound)
			return
		}
		if errors.Is(err, sociallogin.ErrProviderConnected) {
			http.Redirect(w, r, "/me/edit?oauth=other", http.StatusFound)
			return
		}
		if c.logger != nil {
			c.logger.Warn("oauth callback failed", "provider", providerName, "error", err)
		}
		c.redirectOAuthError(w, r)
		return
	}

	if result.ConfirmEmail {
		data := c.GetCommonData(r)
		data["email"] = result.PendingEmail
		data["pageSEO"] = PageSEOForList(i18n.T(r.Context(), "Registration"), "/user/register")
		c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "Registration"))
		data["servicePage"] = true // the sign-in pages: a card, no sidebar
		c.RenderOrFail(w, r, "user/register_success.html", data)
		return
	}
	if result.PendingApproval {
		data := c.GetCommonData(r)
		data["email"] = result.PendingEmail
		data["pendingApproval"] = true
		data["pageSEO"] = PageSEOForList(i18n.T(r.Context(), "Registration - awaiting approval"), "/user/register")
		c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "Registration"))
		data["servicePage"] = true // the sign-in pages: a card, no sidebar
		c.RenderOrFail(w, r, "user/register_success.html", data)
		return
	}

	if result.Connected {
		http.Redirect(w, r, "/me/edit?oauth=connected", http.StatusFound)
		return
	}
	http.Redirect(w, r, result.RedirectTo, http.StatusFound)
}

// CompleteProfile handles GET and POST /user/complete-profile: the
// required profile fields a social sign-up left empty, asked for and
// saved; a user with none to fill goes on to the destination.
func (c *OAuthController) CompleteProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, loggedIn := c.authService.GetUserID(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	user, err := c.userService.ByID(ctx, userID)
	if err != nil || user == nil {
		c.LogFailure(err, "loading the user", "userID", userID)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if !c.oauthService.NeedsCompleteProfile(ctx, user) {
		dest := service.SafeRedirectPath(r.URL.Query().Get("destination"))
		if dest == "" {
			dest = "/me"
		}
		http.Redirect(w, r, dest, http.StatusFound)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			c.RenderBadForm(w, r)
			return
		}
		fields := c.oauthService.UserFields()
		values, fieldError, err := ReadRequiredFields(ctx, fields, r.FormValue)
		if err != nil {
			c.LogFailure(err, "error reading user fields")
			httputil.InternalServerError(w)
			return
		}
		if fieldError != "" {
			data := c.GetCommonData(r)
			data["errors"] = map[string]string{"user_fields": fieldError}
			data["signupFields"] = RequiredFieldInputs(r, fields)
			data["destination"] = r.FormValue("destination")
			data["pageSEO"] = PageSEOForList(i18n.T(r.Context(), "Complete registration"), r.URL.Path)
			data["title"] = i18n.T(r.Context(), "Complete registration")
			c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "Complete registration"))
			c.RenderOrFail(w, r, "user/complete_profile.html", data)
			return
		}
		if err := fields.SaveValues(ctx, user.ID, values); err != nil {
			c.LogFailure(err, "error saving the user's profile fields", "user_id", user.ID)
			httputil.InternalServerError(w)
			return
		}
		dest := service.SafeRedirectPath(r.FormValue("destination"))
		if dest == "" {
			dest = "/me"
		}
		http.Redirect(w, r, dest, http.StatusFound)
		return
	}

	data := c.GetCommonData(r)
	data["destination"] = r.URL.Query().Get("destination")
	data["errors"] = map[string]string{}
	data["signupFields"] = RequiredFieldInputs(r, c.oauthService.UserFields())
	data["pageSEO"] = PageSEOForList(i18n.T(r.Context(), "Complete registration"), r.URL.Path)
	data["title"] = i18n.T(r.Context(), "Complete registration")
	c.setPathTrail(data, r.URL.Path, i18n.T(r.Context(), "Complete registration"))
	c.RenderOrFail(w, r, "user/complete_profile.html", data)
}

// Disconnect handles POST /auth/{provider}/disconnect: the provider's
// account no longer signs in to the user's (not when the account has no
// address a password reset could reach). Back to the profile form.
func (c *OAuthController) Disconnect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, err := c.GetCurrentUser(r)
	if err != nil || user == nil {
		c.RenderForbidden(w, r)
		return
	}
	switch err := c.oauthService.Disconnect(ctx, user, r.PathValue("provider")); {
	case errors.Is(err, sociallogin.ErrLastSignIn):
		http.Redirect(w, r, "/me/edit?oauth=last", http.StatusSeeOther)
	case err != nil:
		c.LogFailure(err, "disconnecting a social network", "user_id", user.ID)
		http.Redirect(w, r, "/me/edit?oauth=error", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/me/edit?oauth=disconnected", http.StatusSeeOther)
	}
}

func (c *OAuthController) redirectOAuthError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login?msg=oauth_error", http.StatusFound)
}
