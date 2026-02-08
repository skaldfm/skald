package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type GuestHandler struct {
	store *models.GuestStore
}

func NewGuestHandler(store *models.GuestStore) *GuestHandler {
	return &GuestHandler{store: store}
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

	data := map[string]any{"Guests": guests}
	if err := views.Render(w, "guests/index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) New(w http.ResponseWriter, r *http.Request) {
	if err := views.Render(w, "guests/new.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) Create(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	bio := strings.TrimSpace(r.FormValue("bio"))
	website := strings.TrimSpace(r.FormValue("website"))

	if name == "" {
		data := map[string]any{
			"Error":   "Name is required",
			"Name":    name,
			"Email":   email,
			"Bio":     bio,
			"Website": website,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		views.Render(w, "guests/new.html", data)
		return
	}

	guest, err := h.store.Create(name, email, bio, website)
	if err != nil {
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

	data := map[string]any{
		"Guest":    guest,
		"Episodes": episodes,
	}
	if err := views.Render(w, "guests/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) Edit(w http.ResponseWriter, r *http.Request) {
	guest, err := h.getGuest(w, r)
	if guest == nil || err != nil {
		return
	}

	data := map[string]any{"Guest": guest}
	if err := views.Render(w, "guests/edit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GuestHandler) Update(w http.ResponseWriter, r *http.Request) {
	guest, err := h.getGuest(w, r)
	if guest == nil || err != nil {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	bio := strings.TrimSpace(r.FormValue("bio"))
	website := strings.TrimSpace(r.FormValue("website"))

	if name == "" {
		data := map[string]any{
			"Guest": guest,
			"Error": "Name is required",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		views.Render(w, "guests/edit.html", data)
		return
	}

	if err := h.store.Update(guest.ID, name, email, bio, website); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/guests/"+strconv.FormatInt(guest.ID, 10), http.StatusSeeOther)
}

func (h *GuestHandler) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	guest, err := h.getGuest(w, r)
	if guest == nil || err != nil {
		return
	}

	if err := h.store.Delete(guest.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/guests", http.StatusSeeOther)
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
