package handlers

import (
	"net/http"

	"github.com/mhermansson/skald/internal/auth"
	"github.com/mhermansson/skald/internal/models"
)

// accessibleShows returns shows filtered by user access (admin gets all).
func accessibleShows(r *http.Request, showStore *models.ShowStore) ([]models.Show, error) {
	ids := auth.AccessibleShowIDs(r.Context())
	if ids == nil {
		return showStore.List()
	}
	return showStore.ListByIDs(ids)
}

// scopeEpisodeFilter adds ShowIDs restriction for non-admins.
func scopeEpisodeFilter(r *http.Request, filter models.EpisodeFilter) models.EpisodeFilter {
	ids := auth.AccessibleShowIDs(r.Context())
	if ids != nil {
		filter.ShowIDs = ids
	}
	return filter
}

// requireShowAccess writes 403 and returns false if user can't view the show.
func requireShowAccess(w http.ResponseWriter, r *http.Request, showID int64) bool {
	if !auth.CanAccessShow(r.Context(), showID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// requireShowEdit writes 403 and returns false if user can't edit the show.
func requireShowEdit(w http.ResponseWriter, r *http.Request, showID int64) bool {
	user := auth.UserFromContext(r.Context())
	if !auth.CanEdit(user) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	if !auth.CanAccessShow(r.Context(), showID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}
