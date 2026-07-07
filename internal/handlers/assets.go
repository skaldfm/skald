package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/skaldfm/skald/internal/models"
)

type AssetHandler struct {
	store    *models.AssetStore
	episodes *models.EpisodeStore
	dataDir  string
}

func NewAssetHandler(store *models.AssetStore, episodes *models.EpisodeStore, dataDir string) *AssetHandler {
	return &AssetHandler{store: store, episodes: episodes, dataDir: dataDir}
}

// resolvePath turns a stored asset path into an on-disk path. New assets are
// stored relative to the data dir; older rows may hold an absolute path, which
// is used as-is for backward compatibility.
func (h *AssetHandler) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(h.dataDir, p)
}

func (h *AssetHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{id}/download", h.Download)
	r.Post("/{id}/delete", h.Delete)
	return r
}

func (h *AssetHandler) Upload(w http.ResponseWriter, r *http.Request) {
	episodeIDStr := chi.URLParam(r, "episodeID")
	episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid episode ID", http.StatusBadRequest)
		return
	}

	ep, err := h.episodes.Get(episodeID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if ep == nil {
		http.NotFound(w, r)
		return
	}
	if !requireShowEdit(w, r, ep.ShowID) {
		return
	}

	// Parse multipart form (32 MB max)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	assetType := r.FormValue("asset_type")
	if assetType == "" {
		assetType = "other"
	}

	// Create upload directory
	uploadDir := filepath.Join(h.dataDir, "uploads", episodeIDStr)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
		return
	}

	// Save file
	destPath := filepath.Join(uploadDir, header.Filename)
	dest, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	written, err := io.Copy(dest, file)
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	// Store the path relative to the data dir so the DB stays portable if the
	// data directory moves or is restored on another host.
	relPath := filepath.Join("uploads", episodeIDStr, header.Filename)
	_, err = h.store.Create(episodeID, header.Filename, relPath, header.Header.Get("Content-Type"), written, assetType)
	if err != nil {
		serverError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/episodes/%d", episodeID), http.StatusSeeOther)
}

func (h *AssetHandler) Download(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid asset ID", http.StatusBadRequest)
		return
	}

	asset, err := h.store.Get(id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if asset == nil {
		http.NotFound(w, r)
		return
	}

	ep, err := h.episodes.Get(asset.EpisodeID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if ep == nil {
		http.NotFound(w, r)
		return
	}
	if !requireShowAccess(w, r, ep.ShowID) {
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, asset.Filename))
	http.ServeFile(w, r, h.resolvePath(asset.Filepath))
}

func (h *AssetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid asset ID", http.StatusBadRequest)
		return
	}

	asset, err := h.store.Get(id)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if asset == nil {
		http.NotFound(w, r)
		return
	}

	ep, err := h.episodes.Get(asset.EpisodeID)
	if err != nil {
		serverError(w, r, err)
		return
	}
	if ep == nil {
		http.NotFound(w, r)
		return
	}
	if !requireShowEdit(w, r, ep.ShowID) {
		return
	}

	// Remove file from disk
	os.Remove(h.resolvePath(asset.Filepath))

	// Remove from database
	if err := h.store.Delete(id); err != nil {
		serverError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/episodes/%d", asset.EpisodeID), http.StatusSeeOther)
}
