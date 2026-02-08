package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type ShowHandler struct {
	store        *models.ShowStore
	episodeStore *models.EpisodeStore
}

type seasonGroup struct {
	Season   *int
	Label    string
	Episodes []models.Episode
}

func NewShowHandler(store *models.ShowStore, episodeStore *models.EpisodeStore) *ShowHandler {
	return &ShowHandler{store: store, episodeStore: episodeStore}
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

	episodes, err := h.episodeStore.List(models.EpisodeFilter{ShowID: show.ID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	groups := groupBySeason(episodes)

	data := map[string]any{
		"Show":         show,
		"SeasonGroups": groups,
		"HasEpisodes":  len(episodes) > 0,
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

func groupBySeason(episodes []models.Episode) []seasonGroup {
	if len(episodes) == 0 {
		return nil
	}

	// Check if any episode has a season number
	hasSeasons := false
	for _, ep := range episodes {
		if ep.SeasonNumber != nil {
			hasSeasons = true
			break
		}
	}

	// No seasons at all — return a single group with no label
	if !hasSeasons {
		sorted := make([]models.Episode, len(episodes))
		copy(sorted, episodes)
		sort.Slice(sorted, func(i, j int) bool {
			ei, ej := sorted[i].EpisodeNumber, sorted[j].EpisodeNumber
			if ei != nil && ej != nil {
				return *ei < *ej
			}
			if ei != nil {
				return true
			}
			if ej != nil {
				return false
			}
			return sorted[i].Title < sorted[j].Title
		})
		return []seasonGroup{{Label: "", Episodes: sorted}}
	}

	// Group by season number
	grouped := map[int][]models.Episode{}  // keyed by season number
	var unassigned []models.Episode
	seasonsSeen := map[int]bool{}

	for _, ep := range episodes {
		if ep.SeasonNumber != nil {
			s := *ep.SeasonNumber
			grouped[s] = append(grouped[s], ep)
			seasonsSeen[s] = true
		} else {
			unassigned = append(unassigned, ep)
		}
	}

	// Sort season numbers
	seasons := make([]int, 0, len(seasonsSeen))
	for s := range seasonsSeen {
		seasons = append(seasons, s)
	}
	sort.Ints(seasons)

	sortEpisodes := func(eps []models.Episode) {
		sort.Slice(eps, func(i, j int) bool {
			ei, ej := eps[i].EpisodeNumber, eps[j].EpisodeNumber
			if ei != nil && ej != nil {
				return *ei < *ej
			}
			if ei != nil {
				return true
			}
			if ej != nil {
				return false
			}
			return eps[i].Title < eps[j].Title
		})
	}

	var groups []seasonGroup
	for _, s := range seasons {
		eps := grouped[s]
		sortEpisodes(eps)
		sVal := s
		groups = append(groups, seasonGroup{
			Season:   &sVal,
			Label:    "Season " + strconv.Itoa(s),
			Episodes: eps,
		})
	}

	if len(unassigned) > 0 {
		sortEpisodes(unassigned)
		groups = append(groups, seasonGroup{
			Label:    "Unassigned",
			Episodes: unassigned,
		})
	}

	return groups
}
