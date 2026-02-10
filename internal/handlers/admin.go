package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
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
	backups  *backup.Manager
	users    *models.UserStore
	guests   *models.GuestStore
	shows    *models.ShowStore
	settings *models.SiteSettingsStore
	dataDir  string
}

func NewAdminHandler(backups *backup.Manager, users *models.UserStore, guests *models.GuestStore, shows *models.ShowStore, settings *models.SiteSettingsStore, dataDir string) *AdminHandler {
	return &AdminHandler{backups: backups, users: users, guests: guests, shows: shows, settings: settings, dataDir: dataDir}
}

func (h *AdminHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", http.RedirectHandler("/admin/users", http.StatusSeeOther).ServeHTTP)
	r.Get("/backups", h.Backups)
	r.Post("/backups", h.CreateBackup)
	r.Get("/backups/{name}", h.DownloadBackup)
	r.Get("/users", h.Users)
	r.Post("/users", h.CreateUser)
	r.Post("/users/{id}/role", h.SetRole)
	r.Get("/users/{id}/shows", h.UserShowsForm)
	r.Post("/users/{id}/shows", h.UserShowsSave)
	r.Post("/users/{id}/delete", h.DeleteUser)
	r.Get("/settings", h.Settings)
	r.Post("/settings", h.UpdateSettings)
	r.Post("/settings/remove", h.RemoveLogo)
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

	// Load show assignments per user
	userShows := make(map[int64][]int64)
	allShows, _ := h.shows.List()
	showNames := make(map[int64]string)
	for _, s := range allShows {
		showNames[s.ID] = s.Name
	}
	for _, u := range users {
		ids, _ := h.users.ShowIDsForUser(u.ID)
		userShows[u.ID] = ids
	}

	data := map[string]any{
		"Users":     users,
		"UserShows": userShows,
		"ShowNames": showNames,
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
	if role != "admin" && role != "editor" {
		role = "viewer"
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

	_, _ = h.guests.Create(&models.Guest{
		Name:  displayName,
		Email: email,
	})

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *AdminHandler) SetRole(w http.ResponseWriter, r *http.Request) {
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

	role := r.FormValue("role")
	if role != "admin" && role != "editor" && role != "viewer" {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	user.Role = role
	if err := h.users.Update(user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (h *AdminHandler) UserShowsForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.users.Get(id)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	allShows, err := h.shows.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	assignedIDs, _ := h.users.ShowIDsForUser(id)
	showItems := make([]pickerItem, len(allShows))
	for i, s := range allShows {
		showItems[i] = pickerItem{ID: s.ID, Name: s.Name}
	}

	data := map[string]any{
		"User":        user,
		"ShowItems":   showItems,
		"AssignedIDs": assignedIDs,
		"ActiveTab":   "users",
	}
	if err := views.Render(w, r, "admin/user_shows.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AdminHandler) UserShowsSave(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.users.Get(id)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	var showIDs []int64
	for _, s := range r.Form["show_ids"] {
		if sid, err := strconv.ParseInt(s, 10, 64); err == nil {
			showIDs = append(showIDs, sid)
		}
	}

	if err := h.users.SetUserShows(id, showIDs); err != nil {
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

func (h *AdminHandler) Settings(w http.ResponseWriter, r *http.Request) {
	ss, err := h.settings.Get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"ActiveTab":     "settings",
		"HasCustomLogo": ss.LogoPath != "",
	}
	if err := views.Render(w, r, "admin/settings.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("logo")
	if err != nil {
		http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".svg" && ext != ".webp" {
		http.Error(w, "Unsupported image format", http.StatusBadRequest)
		return
	}

	// Remove old logo file if present
	ss, _ := h.settings.Get()
	if ss != nil && ss.LogoPath != "" {
		oldFile := filepath.Join(h.dataDir, "uploads", ss.LogoPath)
		_ = os.Remove(oldFile)
	}

	uploadDir := filepath.Join(h.dataDir, "uploads", "site")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := "logo" + ext
	destPath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	relativePath := fmt.Sprintf("site/%s", filename)
	if err := h.settings.Update(relativePath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}

func (h *AdminHandler) RemoveLogo(w http.ResponseWriter, r *http.Request) {
	ss, err := h.settings.Get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ss.LogoPath != "" {
		oldFile := filepath.Join(h.dataDir, "uploads", ss.LogoPath)
		_ = os.Remove(oldFile)
	}

	if err := h.settings.Update(""); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}
