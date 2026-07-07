package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/skaldfm/skald/internal/auth"
	"github.com/skaldfm/skald/internal/models"
	"github.com/skaldfm/skald/internal/views"
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
	shows, err := accessibleShows(r, h.shows)
	if err != nil {
		serverError(w, r, err)
		return
	}

	filter := scopeEpisodeFilter(r, models.EpisodeFilter{})
	allEpisodes, err := h.episodes.List(filter)
	if err != nil {
		serverError(w, r, err)
		return
	}

	// Scope the guest count to the user's shows, consistent with the rest of
	// the dashboard (admins get all).
	var guests []models.Guest
	if showIDs := auth.AccessibleShowIDs(r.Context()); showIDs == nil {
		guests, err = h.guests.List()
	} else {
		guests, err = h.guests.ListByShowIDs(showIDs)
	}
	if err != nil {
		serverError(w, r, err)
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

	if err := views.Render(w, r, "home.html", data); err != nil {
		serverError(w, r, err)
	}
}
