package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/auth"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type KanbanHandler struct {
	episodes *models.EpisodeStore
	shows    *models.ShowStore
}

func NewKanbanHandler(episodes *models.EpisodeStore, shows *models.ShowStore) *KanbanHandler {
	return &KanbanHandler{episodes: episodes, shows: shows}
}

func (h *KanbanHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Board)
	return r
}

func (h *KanbanHandler) Board(w http.ResponseWriter, r *http.Request) {
	filter := models.EpisodeFilter{}
	if showID := r.URL.Query().Get("show"); showID != "" {
		id, _ := strconv.ParseInt(showID, 10, 64)
		filter.ShowID = id
	}
	filter = scopeEpisodeFilter(r, filter)

	episodes, err := h.episodes.List(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shows, err := accessibleShows(r, h.shows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Group episodes by status
	columns := make(map[string][]models.Episode)
	for _, ep := range episodes {
		columns[ep.Status] = append(columns[ep.Status], ep)
	}

	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"Columns":  columns,
		"Statuses": models.Statuses,
		"Shows":    shows,
		"Filter":   filter,
		"CanEdit":  auth.CanEdit(user),
	}
	if err := views.Render(w, r, "episodes/kanban.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
