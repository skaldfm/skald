package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/auth"
	"github.com/mhermansson/skald/internal/backup"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type AdminHandler struct {
	backups *backup.Manager
	users   *models.UserStore
}

func NewAdminHandler(backups *backup.Manager, users *models.UserStore) *AdminHandler {
	return &AdminHandler{backups: backups, users: users}
}

func (h *AdminHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/backups", h.Backups)
	r.Post("/backups", h.CreateBackup)
	r.Get("/backups/{name}", h.DownloadBackup)
	r.Get("/users", h.Users)
	r.Post("/users", h.CreateUser)
	r.Post("/users/{id}/role", h.ToggleRole)
	r.Post("/users/{id}/delete", h.DeleteUser)
	return r
}

func (h *AdminHandler) Backups(w http.ResponseWriter, r *http.Request) {
	backups, err := h.backups.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Backups":   backups,
		"ActiveTab": "backups",
	}
	if err := views.Render(w, r, "admin/backups.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AdminHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	if _, err := h.backups.Create("manual"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.backups.Prune()
	http.Redirect(w, r, "/admin/backups", http.StatusSeeOther)
}

func (h *AdminHandler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	// Sanitize to prevent path traversal
	name = filepath.Base(name)
	path := h.backups.FilePath(name)

	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	http.ServeFile(w, r, path)
}

func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Users":     users,
		"ActiveTab": "users",
	}
	if err := views.Render(w, r, "admin/users.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	password := r.FormValue("password")
	role := r.FormValue("role")

	if email == "" || password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}
	if len(password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if role != "admin" {
		role = "user"
	}

	existing, _ := h.users.GetByEmail(email)
	if existing != nil {
		http.Error(w, "A user with that email already exists", http.StatusConflict)
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := h.users.Create(email, displayName, hash, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *AdminHandler) ToggleRole(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	current := auth.UserFromContext(r.Context())
	if current.ID == id {
		http.Error(w, "Cannot change your own role", http.StatusBadRequest)
		return
	}

	user, err := h.users.Get(id)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.Role == "admin" {
		user.Role = "user"
	} else {
		user.Role = "admin"
	}

	if err := h.users.Update(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	current := auth.UserFromContext(r.Context())
	if current.ID == id {
		http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
		return
	}

	if err := h.users.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
