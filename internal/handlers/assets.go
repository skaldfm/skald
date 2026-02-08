package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/podforge/internal/models"
)

type AssetHandler struct {
	store   *models.AssetStore
	dataDir string
}

func NewAssetHandler(store *models.AssetStore, dataDir string) *AssetHandler {
	return &AssetHandler{store: store, dataDir: dataDir}
}

func (h *AssetHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/episodes/{episodeID}/assets", h.Upload)
	r.Get("/assets/{id}/download", h.Download)
	r.Post("/assets/{id}/delete", h.Delete)
	return r
}

func (h *AssetHandler) Upload(w http.ResponseWriter, r *http.Request) {
	episodeIDStr := chi.URLParam(r, "episodeID")
	episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid episode ID", http.StatusBadRequest)
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

	// Store in database
	_, err = h.store.Create(episodeID, header.Filename, destPath, header.Header.Get("Content-Type"), written, assetType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if asset == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, asset.Filename))
	http.ServeFile(w, r, asset.Filepath)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if asset == nil {
		http.NotFound(w, r)
		return
	}

	// Remove file from disk
	os.Remove(asset.Filepath)

	// Remove from database
	if err := h.store.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/episodes/%d", asset.EpisodeID), http.StatusSeeOther)
}
