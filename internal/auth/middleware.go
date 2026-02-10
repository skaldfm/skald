package auth

import (
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/mhermansson/skald/internal/models"
)

// LoadUser is middleware that loads the current user from the session into the
// request context. It also enforces the first-run setup redirect.
func LoadUser(sm *scs.SessionManager, users *models.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if setup is needed (no users exist)
			count, err := users.Count()
			if err == nil && count == 0 {
				path := r.URL.Path
				if !strings.HasPrefix(path, "/auth/") && !strings.HasPrefix(path, "/static/") && path != "/health" {
					http.Redirect(w, r, "/auth/setup", http.StatusFound)
					return
				}
			}

			// Load user from session
			userID := sm.GetInt64(r.Context(), "user_id")
			if userID > 0 {
				user, err := users.Get(userID)
				if err == nil && user != nil {
					r = r.WithContext(WithUser(r.Context(), user))
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth redirects to the login page if no user is in the context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin returns 403 if the current user is not an admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
