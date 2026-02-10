package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/auth"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type GuestHandler struct {
	store   *models.GuestStore
	dataDir string
}

func NewGuestHandler(store *models.GuestStore, dataDir string) *GuestHandler {
	return &GuestHandler{store: store, dataDir: dataDir}
}

func (h *GuestHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/new", h.New)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Show)
	r.Get("/{id}/edit", h.Edit)
	r.Post("/{id}", h.Update)
	r.Post("/{id}/delete", h.DeleteConfirm)
	return r
}

func (h *GuestHandler) List(w http.ResponseWriter, r *http.Request) {
	guests, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	showMap, err := h.store.ShowsForAllGuests()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"Guests":     guests,
		"GuestShows": showMap,
		"CanEdit":    auth.CanEdit(user),
	}
	if err := views.Render(w, r, "guests/index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) New(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := views.Render(w, r, "guests/new.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Form too large", http.StatusBadRequest)
		return
	}

	g := h.fillFromForm(r)

	if g.Name == "" {
		data := map[string]any{
			"Error": "Name is required",
			"Guest": g,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "guests/new.html", data)
		return
	}

	guest, err := h.store.Create(g)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle image upload
	if err := h.handleImageUpload(r, guest); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/guests/"+strconv.FormatInt(guest.ID, 10), http.StatusSeeOther)
}

func (h *GuestHandler) Show(w http.ResponseWriter, r *http.Request) {
	guest, err := h.getGuest(w, r)
	if guest == nil || err != nil {
		return
	}

	episodes, err := h.store.EpisodesForGuest(guest.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"Guest":    guest,
		"Episodes": episodes,
		"CanEdit":  auth.CanEdit(user),
	}
	if err := views.Render(w, r, "guests/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) Edit(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	guest, err := h.getGuest(w, r)
	if guest == nil || err != nil {
		return
	}

	data := map[string]any{"Guest": guest}
	if err := views.Render(w, r, "guests/edit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	guest, err := h.getGuest(w, r)
	if guest == nil || err != nil {
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Form too large", http.StatusBadRequest)
		return
	}

	g := h.fillFromForm(r)
	g.ID = guest.ID
	g.Image = guest.Image

	if g.Name == "" {
		data := map[string]any{
			"Guest": g,
			"Error": "Name is required",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "guests/edit.html", data)
		return
	}

	// Handle image removal
	if r.FormValue("remove_image") == "1" {
		if guest.Image != "" {
			os.Remove(filepath.Join(h.dataDir, "uploads", guest.Image))
		}
		g.Image = ""
	}

	// Handle image upload
	if err := h.handleImageUpload(r, &models.Guest{ID: guest.ID, Image: g.Image}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Re-read to get the updated image path
	updated, _ := h.store.Get(guest.ID)
	if updated != nil && updated.Image != g.Image {
		g.Image = updated.Image
	}

	if err := h.store.Update(g); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/guests/"+strconv.FormatInt(guest.ID, 10), http.StatusSeeOther)
}

func (h *GuestHandler) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	guest, err := h.getGuest(w, r)
	if guest == nil || err != nil {
		return
	}

	// Remove image files
	if guest.Image != "" {
		os.Remove(filepath.Join(h.dataDir, "uploads", guest.Image))
	}

	if err := h.store.Delete(guest.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
}

func (h *GuestHandler) fillFromForm(r *http.Request) *models.Guest {
	return &models.Guest{
		Name:      strings.TrimSpace(r.FormValue("name")),
		Email:     strings.TrimSpace(r.FormValue("email")),
		Bio:       strings.TrimSpace(r.FormValue("bio")),
		Website:   strings.TrimSpace(r.FormValue("website")),
		Company:   strings.TrimSpace(r.FormValue("company")),
		Podcast:   strings.TrimSpace(r.FormValue("podcast")),
		Twitter:   strings.TrimSpace(r.FormValue("twitter")),
		Instagram: strings.TrimSpace(r.FormValue("instagram")),
		LinkedIn:  strings.TrimSpace(r.FormValue("linkedin")),
		Mastodon:  strings.TrimSpace(r.FormValue("mastodon")),
		IsHost:    r.FormValue("is_host") == "1",
	}
}

func (h *GuestHandler) handleImageUpload(r *http.Request, guest *models.Guest) error {
	file, header, err := r.FormFile("image")
	if err != nil {
		return nil // no file uploaded
	}
	defer file.Close()

	idStr := strconv.FormatInt(guest.ID, 10)
	uploadDir := filepath.Join(h.dataDir, "uploads", "guests", idStr)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return err
	}

	ext := filepath.Ext(header.Filename)
	destPath := filepath.Join(uploadDir, "image"+ext)

	// Remove old image if it exists and differs
	if guest.Image != "" {
		oldPath := filepath.Join(h.dataDir, "uploads", guest.Image)
		if oldPath != destPath {
			os.Remove(oldPath)
		}
	}

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		return err
	}

	// Update image path in DB
	relPath := filepath.Join("guests", idStr, "image"+ext)
	guest.Image = relPath
	return h.store.UpdateImage(guest.ID, relPath)
}

func (h *GuestHandler) getGuest(w http.ResponseWriter, r *http.Request) (*models.Guest, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid guest ID", http.StatusBadRequest)
		return nil, err
	}

	guest, err := h.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	if guest == nil {
		http.NotFound(w, r)
		return nil, nil
	}

	return guest, nil
}
