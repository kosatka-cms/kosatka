package middleware

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/paths"
)

// ThemePreviewMiddleware renders the request in the theme the signed-in
// user previews (paths.WithTheme): the session's
// themes.PreviewSessionKey, when the user may change the theme
// (service.PermAccessAdmin, as the Appearance page) and the theme is
// available and not the active one; any other request is left as it is.
// Must run after OptionalAuthMiddleware.
func ThemePreviewMiddleware(store *sessions.CookieStore, userRepo entity.UserRepository, authService service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value(userIDKey).(int)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			session, err := store.Get(r, entity.SessionName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			theme, ok := session.Values[themes.PreviewSessionKey].(string)
			if !ok || theme == paths.Theme() || !themes.IsAvailable(theme) {
				next.ServeHTTP(w, r)
				return
			}
			user, err := userRepo.ByID(r.Context(), userID)
			if err != nil || user == nil || !authService.HasPermission(r.Context(), user, service.PermAccessAdmin) {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(paths.WithTheme(r.Context(), theme)))
		})
	}
}
