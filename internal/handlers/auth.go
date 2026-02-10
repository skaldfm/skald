package handlers

import (
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/auth"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type AuthHandler struct {
	users   *models.UserStore
	session *scs.SessionManager
}

func NewAuthHandler(users *models.UserStore, session *scs.SessionManager) *AuthHandler {
	return &AuthHandler{users: users, session: session}
}

func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/login", h.LoginForm)
	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Get("/setup", h.SetupForm)
	r.Post("/setup", h.Setup)
	return r
}

func (h *AuthHandler) LoginForm(w http.ResponseWriter, r *http.Request) {
	if err := views.RenderAuth(w, r, "auth/login.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	user, err := h.users.GetByEmail(email)
	if err != nil || user == nil || !auth.CheckPassword(user.PasswordHash, password) {
		data := map[string]any{
			"Error": "Invalid email or password",
			"Email": email,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.RenderAuth(w, r, "auth/login.html", data)
		return
	}

	// Prevent session fixation
	if err := h.session.RenewToken(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.session.Put(r.Context(), "user_id", user.ID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) ProfileForm(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"User":  user,
		"Saved": r.URL.Query().Get("saved") == "1",
	}
	if err := views.Render(w, r, "profile.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AuthHandler) ProfileUpdate(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	name := strings.TrimSpace(r.FormValue("display_name"))
	email := strings.TrimSpace(r.FormValue("email"))
	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	renderErr := func(msg string) {
		data := map[string]any{
			"User":  user,
			"Error": msg,
		}
		user.DisplayName = name
		user.Email = email
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "profile.html", data)
	}

	if email == "" {
		renderErr("Email is required")
		return
	}

	user.DisplayName = name
	user.Email = email

	// Password change (only if any password field is filled)
	if currentPassword != "" || newPassword != "" || confirmPassword != "" {
		if !auth.CheckPassword(user.PasswordHash, currentPassword) {
			renderErr("Current password is incorrect")
			return
		}
		if newPassword != confirmPassword {
			renderErr("New passwords do not match")
			return
		}
		if len(newPassword) < 8 {
			renderErr("New password must be at least 8 characters")
			return
		}
		hash, err := auth.HashPassword(newPassword)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		user.PasswordHash = hash
	}

	if err := h.users.Update(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile?saved=1", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.session.Destroy(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func (h *AuthHandler) SetupForm(w http.ResponseWriter, r *http.Request) {
	count, _ := h.users.Count()
	if count > 0 {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	if err := views.RenderAuth(w, r, "auth/setup.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	count, _ := h.users.Count()
	if count > 0 {
		http.Error(w, "Setup already completed", http.StatusConflict)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	renderErr := func(msg string) {
		data := map[string]any{
			"Error":       msg,
			"Email":       email,
			"DisplayName": displayName,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.RenderAuth(w, r, "auth/setup.html", data)
	}

	if email == "" || password == "" {
		renderErr("Email and password are required")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user, err := h.users.Create(email, displayName, hash, "admin")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the new user in
	if err := h.session.RenewToken(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.session.Put(r.Context(), "user_id", user.ID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
