package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/backup"
	"github.com/mhermansson/skald/internal/views"
)

type AdminHandler struct {
	backups *backup.Manager
}

func NewAdminHandler(backups *backup.Manager) *AdminHandler {
	return &AdminHandler{backups: backups}
}

func (h *AdminHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/backups", h.Backups)
	r.Post("/backups", h.CreateBackup)
	r.Get("/backups/{name}", h.DownloadBackup)
	return r
}

func (h *AdminHandler) Backups(w http.ResponseWriter, r *http.Request) {
	backups, err := h.backups.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Backups": backups,
	}
	if err := views.Render(w, "admin/backups.html", data); err != nil {
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
