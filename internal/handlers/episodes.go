package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/podforge/internal/models"
	"github.com/mhermansson/podforge/internal/views"
)

type EpisodeHandler struct {
	episodes *models.EpisodeStore
	shows    *models.ShowStore
	assets   *models.AssetStore
	guests   *models.GuestStore
}

func NewEpisodeHandler(episodes *models.EpisodeStore, shows *models.ShowStore, assets *models.AssetStore, guests *models.GuestStore) *EpisodeHandler {
	return &EpisodeHandler{episodes: episodes, shows: shows, assets: assets, guests: guests}
}

func (h *EpisodeHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/new", h.New)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Show)
	r.Get("/{id}/edit", h.Edit)
	r.Post("/{id}", h.Update)
	r.Post("/{id}/status", h.UpdateStatus)
	r.Post("/{id}/delete", h.DeleteConfirm)
	return r
}

func (h *EpisodeHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := models.EpisodeFilter{
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("q"),
	}
	if showID := r.URL.Query().Get("show"); showID != "" {
		id, _ := strconv.ParseInt(showID, 10, 64)
		filter.ShowID = id
	}

	episodes, err := h.episodes.List(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shows, err := h.shows.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Episodes": episodes,
		"Shows":    shows,
		"Filter":   filter,
		"Statuses": models.Statuses,
	}
	if err := views.Render(w, "episodes/index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) New(w http.ResponseWriter, r *http.Request) {
	shows, err := h.shows.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Shows":    shows,
		"Statuses": models.Statuses,
		"ShowID":   r.URL.Query().Get("show"),
	}
	if err := views.Render(w, "episodes/new.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	status := r.FormValue("status")
	showIDStr := r.FormValue("show_id")

	showID, _ := strconv.ParseInt(showIDStr, 10, 64)

	if title == "" || showID == 0 {
		shows, _ := h.shows.List()
		data := map[string]any{
			"Error":       "Title and show are required",
			"Title":       title,
			"Description": description,
			"Status":      status,
			"ShowID":      showIDStr,
			"Shows":       shows,
			"Statuses":    models.Statuses,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		views.Render(w, "episodes/new.html", data)
		return
	}

	ep, err := h.episodes.Create(showID, title, description, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/episodes/"+strconv.FormatInt(ep.ID, 10), http.StatusSeeOther)
}

func (h *EpisodeHandler) Show(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}

	assets, _ := h.assets.ListForEpisode(ep.ID)
	guests, _ := h.guests.GuestsForEpisode(ep.ID)

	data := map[string]any{
		"Episode":  ep,
		"Statuses": models.Statuses,
		"Assets":   assets,
		"Guests":   guests,
	}
	if err := views.Render(w, "episodes/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) Edit(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}

	shows, err := h.shows.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Episode":  ep,
		"Shows":    shows,
		"Statuses": models.Statuses,
	}
	if err := views.Render(w, "episodes/edit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}

	ep.Title = strings.TrimSpace(r.FormValue("title"))
	ep.Description = strings.TrimSpace(r.FormValue("description"))
	ep.Status = r.FormValue("status")
	ep.Script = r.FormValue("script")
	ep.ShowNotes = r.FormValue("show_notes")

	if showID, err := strconv.ParseInt(r.FormValue("show_id"), 10, 64); err == nil {
		ep.ShowID = showID
	}
	if epNum := r.FormValue("episode_number"); epNum != "" {
		n, _ := strconv.Atoi(epNum)
		ep.EpisodeNumber = &n
	}
	if seNum := r.FormValue("season_number"); seNum != "" {
		n, _ := strconv.Atoi(seNum)
		ep.SeasonNumber = &n
	}

	if ep.Title == "" {
		shows, _ := h.shows.List()
		data := map[string]any{
			"Episode":  ep,
			"Shows":    shows,
			"Statuses": models.Statuses,
			"Error":    "Title is required",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		views.Render(w, "episodes/edit.html", data)
		return
	}

	if err := h.episodes.Update(ep); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/episodes/"+strconv.FormatInt(ep.ID, 10), http.StatusSeeOther)
}

func (h *EpisodeHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}

	status := r.FormValue("status")
	if err := h.episodes.UpdateStatus(ep.ID, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If HTMX request, return just the updated status badge
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ` +
			views.StatusColor(status) + `">` + views.StatusLabel(status) + `</span>`))
		return
	}

	http.Redirect(w, r, "/episodes/"+strconv.FormatInt(ep.ID, 10), http.StatusSeeOther)
}

func (h *EpisodeHandler) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}

	if err := h.episodes.Delete(ep.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/episodes", http.StatusSeeOther)
}

func (h *EpisodeHandler) getEpisode(w http.ResponseWriter, r *http.Request) (*models.Episode, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid episode ID", http.StatusBadRequest)
		return nil, err
	}

	ep, err := h.episodes.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	if ep == nil {
		http.NotFound(w, r)
		return nil, nil
	}

	return ep, nil
}
