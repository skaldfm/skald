package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type DashboardHandler struct {
	episodes *models.EpisodeStore
	shows    *models.ShowStore
	guests   *models.GuestStore
}

func NewDashboardHandler(episodes *models.EpisodeStore, shows *models.ShowStore, guests *models.GuestStore) *DashboardHandler {
	return &DashboardHandler{episodes: episodes, shows: shows, guests: guests}
}

type showCard struct {
	Show     models.Show
	Total    int
	Segments []pipelineSegment
}

type pipelineSegment struct {
	Status  string
	Count   int
	Percent float64
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	shows, err := h.shows.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allEpisodes, err := h.episodes.List(models.EpisodeFilter{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	guests, err := h.guests.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()

	// Global status counts
	globalCounts := make(map[string]int)
	for _, ep := range allEpisodes {
		globalCounts[ep.Status]++
	}

	var globalSegments []pipelineSegment
	total := len(allEpisodes)
	if total > 0 {
		for _, s := range models.Statuses {
			if c := globalCounts[s]; c > 0 {
				globalSegments = append(globalSegments, pipelineSegment{
					Status:  s,
					Count:   c,
					Percent: float64(c) / float64(total) * 100,
				})
			}
		}
	}

	// Recent episodes (by updated_at, already sorted from query)
	recentLimit := 8
	if len(allEpisodes) < recentLimit {
		recentLimit = len(allEpisodes)
	}
	recent := allEpisodes[:recentLimit]

	// Upcoming episodes (future publish dates, sorted nearest first)
	var upcoming []models.Episode
	for _, ep := range allEpisodes {
		if ep.PublishDate != nil && ep.PublishDate.After(now) {
			upcoming = append(upcoming, ep)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool {
		return upcoming[i].PublishDate.Before(*upcoming[j].PublishDate)
	})
	if len(upcoming) > 5 {
		upcoming = upcoming[:5]
	}

	// Per-show cards
	epsByShow := make(map[int64][]models.Episode)
	for _, ep := range allEpisodes {
		epsByShow[ep.ShowID] = append(epsByShow[ep.ShowID], ep)
	}

	var showCards []showCard
	for _, show := range shows {
		eps := epsByShow[show.ID]
		sc := showCard{Show: show, Total: len(eps)}
		if len(eps) > 0 {
			counts := make(map[string]int)
			for _, ep := range eps {
				counts[ep.Status]++
			}
			for _, s := range models.Statuses {
				if c := counts[s]; c > 0 {
					sc.Segments = append(sc.Segments, pipelineSegment{
						Status:  s,
						Count:   c,
						Percent: float64(c) / float64(len(eps)) * 100,
					})
				}
			}
		}
		showCards = append(showCards, sc)
	}

	publishedCount := globalCounts["published"]

	data := map[string]any{
		"TotalShows":     len(shows),
		"TotalEpisodes":  total,
		"TotalGuests":    len(guests),
		"PublishedCount": publishedCount,
		"GlobalSegments": globalSegments,
		"Recent":         recent,
		"Upcoming":       upcoming,
		"ShowCards":      showCards,
	}

	if err := views.Render(w, "home.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
