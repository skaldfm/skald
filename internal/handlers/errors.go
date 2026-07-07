package handlers

import (
	"log/slog"
	"net/http"
)

// serverError logs the underlying error (with request context) and returns a
// generic 500 to the client, so internal detail — SQL text, driver messages,
// file paths — is never leaked in the response body.
func serverError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("server error", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

// passwordProblem returns a user-facing message if the password is unacceptable,
// or "" if it's fine. The upper bound matches bcrypt, which silently ignores
// bytes past 72 — without this, a long password would appear set but only its
// first 72 bytes would actually be checked at login.
func passwordProblem(pw string) string {
	switch {
	case len(pw) < 8:
		return "Password must be at least 8 characters"
	case len(pw) > 72:
		return "Password must be at most 72 characters"
	default:
		return ""
	}
}
