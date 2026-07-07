package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/skaldfm/skald/internal/auth"
	"github.com/skaldfm/skald/internal/models"
	"github.com/skaldfm/skald/internal/views"
)

type AuthHandler struct {
	users            *models.UserStore
	guests           *models.GuestStore
	session          *scs.SessionManager
	openRegistration bool
	loginLimiter     *loginLimiter
}

func NewAuthHandler(users *models.UserStore, guests *models.GuestStore, session *scs.SessionManager, openRegistration bool) *AuthHandler {
	return &AuthHandler{
		users:            users,
		guests:           guests,
		session:          session,
		openRegistration: openRegistration,
		// Allow 10 failed logins per IP per 15 minutes.
		loginLimiter: newLoginLimiter(10, 15*time.Minute),
	}
}

func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/login", h.LoginForm)
	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Get("/setup", h.SetupForm)
	r.Post("/setup", h.Setup)
	r.Get("/register", h.RegisterForm)
	r.Post("/register", h.Register)
	return r
}

func (h *AuthHandler) LoginForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"OpenRegistration": h.openRegistration,
	}
	if err := views.RenderAuth(w, r, "auth/login.html", data); err != nil {
		serverError(w, r, err)
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	ip := clientIP(r)
	if !h.loginLimiter.allow(ip) {
		data := map[string]any{
			"Error":            "Too many failed attempts. Please wait a few minutes and try again.",
			"Email":            email,
			"OpenRegistration": h.openRegistration,
		}
		w.WriteHeader(http.StatusTooManyRequests)
		_ = views.RenderAuth(w, r, "auth/login.html", data)
		return
	}

	user, err := h.users.GetByEmail(email)
	valid := false
	if err == nil && user != nil {
		valid = auth.CheckPassword(user.PasswordHash, password)
	} else {
		// Equalize timing so a missing account is indistinguishable from a
		// wrong password (prevents user enumeration).
		auth.CheckDummyPassword(password)
	}
	if !valid {
		h.loginLimiter.recordFailure(ip)
		data := map[string]any{
			"Error":            "Invalid email or password",
			"Email":            email,
			"OpenRegistration": h.openRegistration,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.RenderAuth(w, r, "auth/login.html", data)
		return
	}

	h.loginLimiter.reset(ip)

	// Prevent session fixation
	if err := h.session.RenewToken(r.Context()); err != nil {
		serverError(w, r, err)
		return
	}
	h.session.Put(r.Context(), "user_id", user.ID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) RegisterForm(w http.ResponseWriter, r *http.Request) {
	if !h.openRegistration {
		http.Error(w, "Registration is disabled", http.StatusForbidden)
		return
	}
	if err := views.RenderAuth(w, r, "auth/register.html", nil); err != nil {
		serverError(w, r, err)
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if !h.openRegistration {
		http.Error(w, "Registration is disabled", http.StatusForbidden)
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
		_ = views.RenderAuth(w, r, "auth/register.html", data)
	}

	if email == "" || password == "" {
		renderErr("Email and password are required")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match")
		return
	}
	if msg := passwordProblem(password); msg != "" {
		renderErr(msg)
		return
	}

	existing, _ := h.users.GetByEmail(email)
	if existing != nil {
		renderErr("An account with that email already exists")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		serverError(w, r, err)
		return
	}

	user, err := h.users.Create(email, displayName, hash, "viewer")
	if err != nil {
		serverError(w, r, err)
		return
	}

	_, _ = h.guests.Create(&models.Guest{
		Name:  displayName,
		Email: email,
	})

	if err := h.session.RenewToken(r.Context()); err != nil {
		serverError(w, r, err)
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
		serverError(w, r, err)
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
		if msg := passwordProblem(newPassword); msg != "" {
			renderErr(msg)
			return
		}
		hash, err := auth.HashPassword(newPassword)
		if err != nil {
			serverError(w, r, err)
			return
		}
		user.PasswordHash = hash
	}

	if err := h.users.Update(user); err != nil {
		serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/profile?saved=1", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.session.Destroy(r.Context()); err != nil {
		serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func (h *AuthHandler) SetupForm(w http.ResponseWriter, r *http.Request) {
	hasUser, err := h.users.HasAnyUser()
	if err != nil {
		serverError(w, r, err)
		return
	}
	if hasUser {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	if err := views.RenderAuth(w, r, "auth/setup.html", nil); err != nil {
		serverError(w, r, err)
	}
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	// Fail closed on error: a discarded error here would let a transient DB
	// failure read as "no users" and create a second admin on a populated system.
	hasUser, err := h.users.HasAnyUser()
	if err != nil {
		serverError(w, r, err)
		return
	}
	if hasUser {
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
	if msg := passwordProblem(password); msg != "" {
		renderErr(msg)
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		serverError(w, r, err)
		return
	}

	user, err := h.users.Create(email, displayName, hash, "admin")
	if err != nil {
		serverError(w, r, err)
		return
	}

	// Create a corresponding Person entry as host
	_, _ = h.guests.Create(&models.Guest{
		Name:   displayName,
		Email:  email,
		IsHost: true,
	})

	// Log the new user in
	if err := h.session.RenewToken(r.Context()); err != nil {
		serverError(w, r, err)
		return
	}
	h.session.Put(r.Context(), "user_id", user.ID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
