package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/skaldfm/skald/internal/models"
	"github.com/skaldfm/skald/internal/views"
)

type PrompterHandler struct {
	episodes *models.EpisodeStore
}

func NewPrompterHandler(episodes *models.EpisodeStore) *PrompterHandler {
	return &PrompterHandler{episodes: episodes}
}

func (h *PrompterHandler) Prompter(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid episode ID", http.StatusBadRequest)
		return
	}

	ep, err := h.episodes.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ep == nil {
		http.NotFound(w, r)
		return
	}
	if !requireShowAccess(w, r, ep.ShowID) {
		return
	}

	data := map[string]any{
		"Episode": ep,
	}
	if err := views.Render(w, r, "prompter/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
