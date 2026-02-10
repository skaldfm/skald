package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skaldfm/skald/internal/auth"
	"github.com/skaldfm/skald/internal/models"
	"github.com/skaldfm/skald/internal/views"
)

type SponsorshipHandler struct {
	store    *models.SponsorshipStore
	episodes *models.EpisodeStore
	dataDir  string
}

func NewSponsorshipHandler(store *models.SponsorshipStore, episodes *models.EpisodeStore, dataDir string) *SponsorshipHandler {
	return &SponsorshipHandler{store: store, episodes: episodes, dataDir: dataDir}
}

func (h *SponsorshipHandler) Routes() chi.Router {
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

func (h *SponsorshipHandler) List(w http.ResponseWriter, r *http.Request) {
	sponsorships, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"Sponsorships": sponsorships,
		"CanEdit":      auth.CanEdit(user),
	}
	if err := views.Render(w, r, "sponsorships/index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *SponsorshipHandler) New(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := views.Render(w, r, "sponsorships/new.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *SponsorshipHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Form too large", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		data := map[string]any{
			"Error": "Name is required",
			"Name":  name,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "sponsorships/new.html", data)
		return
	}

	sp, err := h.store.Create(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fill additional fields
	h.fillFromForm(sp, r)

	// Handle order file upload
	h.handleOrderUpload(sp, r)

	if err := h.store.Update(sp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sponsorships/"+strconv.FormatInt(sp.ID, 10), http.StatusSeeOther)
}

func (h *SponsorshipHandler) Show(w http.ResponseWriter, r *http.Request) {
	sp, err := h.getSponsorship(w, r)
	if sp == nil || err != nil {
		return
	}

	episodes, err := h.store.EpisodesForSponsorship(sp.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"Sponsorship": sp,
		"Episodes":    episodes,
		"CanEdit":     auth.CanEdit(user),
	}
	if err := views.Render(w, r, "sponsorships/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *SponsorshipHandler) Edit(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	sp, err := h.getSponsorship(w, r)
	if sp == nil || err != nil {
		return
	}

	data := map[string]any{"Sponsorship": sp}
	if err := views.Render(w, r, "sponsorships/edit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *SponsorshipHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	sp, err := h.getSponsorship(w, r)
	if sp == nil || err != nil {
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Form too large", http.StatusBadRequest)
		return
	}

	sp.Name = strings.TrimSpace(r.FormValue("name"))
	if sp.Name == "" {
		data := map[string]any{
			"Sponsorship": sp,
			"Error":       "Name is required",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "sponsorships/edit.html", data)
		return
	}

	h.fillFromForm(sp, r)

	// Handle order file upload
	h.handleOrderUpload(sp, r)

	// Handle order file removal
	if r.FormValue("remove_order_file") == "1" {
		if sp.OrderFile != "" {
			os.Remove(filepath.Join(h.dataDir, "uploads", sp.OrderFile))
		}
		sp.OrderFile = ""
	}

	if err := h.store.Update(sp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sponsorships/"+strconv.FormatInt(sp.ID, 10), http.StatusSeeOther)
}

func (h *SponsorshipHandler) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	sp, err := h.getSponsorship(w, r)
	if sp == nil || err != nil {
		return
	}

	// Clean up uploaded files
	if sp.OrderFile != "" {
		os.Remove(filepath.Join(h.dataDir, "uploads", sp.OrderFile))
	}

	if err := h.store.Delete(sp.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sponsorships", http.StatusSeeOther)
}

func (h *SponsorshipHandler) getSponsorship(w http.ResponseWriter, r *http.Request) (*models.Sponsorship, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid sponsorship ID", http.StatusBadRequest)
		return nil, err
	}

	sp, err := h.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	if sp == nil {
		http.NotFound(w, r)
		return nil, nil
	}

	return sp, nil
}

func (h *SponsorshipHandler) fillFromForm(sp *models.Sponsorship, r *http.Request) {
	sp.Description = strings.TrimSpace(r.FormValue("description"))
	sp.Script = r.FormValue("script")

	if v := strings.TrimSpace(r.FormValue("cpm")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			sp.CPM = &f
		}
	} else {
		sp.CPM = nil
	}

	if v := strings.TrimSpace(r.FormValue("average_listens")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			sp.AverageListens = &n
		}
	} else {
		sp.AverageListens = nil
	}

	if v := strings.TrimSpace(r.FormValue("total_cost")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			sp.TotalCost = &f
		}
	} else {
		sp.TotalCost = nil
	}

	if v := strings.TrimSpace(r.FormValue("drop_date")); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			sp.DropDate = &t
		}
	} else {
		sp.DropDate = nil
	}

	if v := strings.TrimSpace(r.FormValue("payment_due_date")); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			sp.PaymentDueDate = &t
		}
	} else {
		sp.PaymentDueDate = nil
	}
}

func (h *SponsorshipHandler) handleOrderUpload(sp *models.Sponsorship, r *http.Request) {
	file, header, err := r.FormFile("order_file")
	if err != nil {
		return
	}
	defer file.Close()

	idStr := strconv.FormatInt(sp.ID, 10)
	uploadDir := filepath.Join(h.dataDir, "uploads", "sponsorships", idStr)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return
	}

	ext := filepath.Ext(header.Filename)
	destPath := filepath.Join(uploadDir, "order"+ext)

	// Remove old order file if different
	if sp.OrderFile != "" {
		oldPath := filepath.Join(h.dataDir, "uploads", sp.OrderFile)
		if oldPath != destPath {
			os.Remove(oldPath)
		}
	}

	dest, err := os.Create(destPath)
	if err != nil {
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		return
	}

	sp.OrderFile = fmt.Sprintf("sponsorships/%s/order%s", idStr, ext)
}
