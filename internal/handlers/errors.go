package handlers

import (
	"log"
	"net/http"
)

// serverError logs the underlying error (with request context) and returns a
// generic 500 to the client, so internal detail — SQL text, driver messages,
// file paths — is never leaked in the response body.
func serverError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("server error: %s %s: %v", r.Method, r.URL.Path, err)
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}
