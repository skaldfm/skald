package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type ShowHandler struct {
	store *models.ShowStore
}

func NewShowHandler(store *models.ShowStore) *ShowHandler {
	return &ShowHandler{store: store}
}

func (h *ShowHandler) Routes() chi.Router {
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

func (h *ShowHandler) List(w http.ResponseWriter, r *http.Request) {
	shows, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Shows": shows,
	}
	if err := views.Render(w, "shows/index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) New(w http.ResponseWriter, r *http.Request) {
	if err := views.Render(w, "shows/new.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) Create(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		data := map[string]any{
			"Error":       "Name is required",
			"Name":        name,
			"Description": description,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, "shows/new.html", data)
		return
	}

	show, err := h.store.Create(name, description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/shows/"+strconv.FormatInt(show.ID, 10), http.StatusSeeOther)
}

func (h *ShowHandler) Show(w http.ResponseWriter, r *http.Request) {
	show, err := h.getShow(w, r)
	if show == nil || err != nil {
		return
	}

	data := map[string]any{
		"Show": show,
	}
	if err := views.Render(w, "shows/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) Edit(w http.ResponseWriter, r *http.Request) {
	show, err := h.getShow(w, r)
	if show == nil || err != nil {
		return
	}

	data := map[string]any{
		"Show": show,
	}
	if err := views.Render(w, "shows/edit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) Update(w http.ResponseWriter, r *http.Request) {
	show, err := h.getShow(w, r)
	if show == nil || err != nil {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		data := map[string]any{
			"Show":  show,
			"Error": "Name is required",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, "shows/edit.html", data)
		return
	}

	if err := h.store.Update(show.ID, name, description); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/shows/"+strconv.FormatInt(show.ID, 10), http.StatusSeeOther)
}

func (h *ShowHandler) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	show, err := h.getShow(w, r)
	if show == nil || err != nil {
		return
	}

	if err := h.store.Delete(show.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/shows", http.StatusSeeOther)
}

func (h *ShowHandler) getShow(w http.ResponseWriter, r *http.Request) (*models.Show, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid show ID", http.StatusBadRequest)
		return nil, err
	}

	show, err := h.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	if show == nil {
		http.NotFound(w, r)
		return nil, nil
	}

	return show, nil
}
