package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skaldfm/skald/internal/auth"
	"github.com/skaldfm/skald/internal/models"
	"github.com/skaldfm/skald/internal/views"
)

type ShowHandler struct {
	store        *models.ShowStore
	episodeStore *models.EpisodeStore
	guests       *models.GuestStore
	dataDir      string
}

type seasonGroup struct {
	Season   *int
	Label    string
	Episodes []models.Episode
}

func NewShowHandler(store *models.ShowStore, episodeStore *models.EpisodeStore, guests *models.GuestStore, dataDir string) *ShowHandler {
	return &ShowHandler{store: store, episodeStore: episodeStore, guests: guests, dataDir: dataDir}
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
	shows, err := accessibleShows(r, h.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"Shows":   shows,
		"IsAdmin": auth.IsAdmin(user),
	}
	if err := views.Render(w, r, "shows/index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) New(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAdmin(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := views.Render(w, r, "shows/new.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAdmin(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	website := strings.TrimSpace(r.FormValue("website"))
	color := r.FormValue("color")

	if name == "" {
		data := map[string]any{
			"Error":       "Name is required",
			"Name":        name,
			"Description": description,
			"Website":     website,
			"Color":       color,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "shows/new.html", data)
		return
	}

	show, err := h.store.Create(name, description, website, color)
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
	if !requireShowAccess(w, r, show.ID) {
		return
	}

	user := auth.UserFromContext(r.Context())
	episodes, err := h.episodeStore.List(models.EpisodeFilter{ShowID: show.ID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	groups := groupBySeason(episodes)

	// Compute stats for the show header
	statusCounts := make(map[string]int)
	publishedCount := 0
	var nextEpisode *models.Episode
	now := time.Now()
	for i, ep := range episodes {
		statusCounts[ep.Status]++
		if ep.Status == "published" {
			publishedCount++
		}
		if ep.PublishDate != nil && ep.PublishDate.After(now) {
			if nextEpisode == nil || ep.PublishDate.Before(*nextEpisode.PublishDate) {
				nextEpisode = &episodes[i]
			}
		}
	}

	// Build ordered status segments for the pipeline bar
	type statusSegment struct {
		Status  string
		Count   int
		Percent float64
	}
	var segments []statusSegment
	total := len(episodes)
	for _, s := range models.Statuses {
		if c := statusCounts[s]; c > 0 {
			segments = append(segments, statusSegment{
				Status:  s,
				Count:   c,
				Percent: float64(c) / float64(total) * 100,
			})
		}
	}

	hosts, _ := h.guests.HostsForShow(show.ID)

	data := map[string]any{
		"Show":           show,
		"Hosts":          hosts,
		"SeasonGroups":   groups,
		"HasEpisodes":    len(episodes) > 0,
		"TotalEpisodes":  total,
		"PublishedCount": publishedCount,
		"NextEpisode":    nextEpisode,
		"Segments":       segments,
		"CanEdit":        auth.CanEdit(user),
	}
	if err := views.Render(w, r, "shows/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) Edit(w http.ResponseWriter, r *http.Request) {
	show, err := h.getShow(w, r)
	if show == nil || err != nil {
		return
	}
	if !requireShowEdit(w, r, show.ID) {
		return
	}

	hostGuests, _ := h.guests.ListHosts()
	hostIDs, _ := h.guests.HostIDsForShow(show.ID)
	hostItems := make([]pickerItem, len(hostGuests))
	for i, g := range hostGuests {
		hostItems[i] = pickerItem{ID: g.ID, Name: g.Name}
	}

	data := map[string]any{
		"Show":          show,
		"HostItems":     hostItems,
		"LinkedHostIDs": hostIDs,
		"IsAdmin":       auth.IsAdmin(auth.UserFromContext(r.Context())),
	}
	if err := views.Render(w, r, "shows/edit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *ShowHandler) Update(w http.ResponseWriter, r *http.Request) {
	show, err := h.getShow(w, r)
	if show == nil || err != nil {
		return
	}
	if !requireShowEdit(w, r, show.ID) {
		return
	}

	// Parse multipart form (10 MB max for artwork)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Form too large", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	website := strings.TrimSpace(r.FormValue("website"))
	color := r.FormValue("color")

	if name == "" {
		hostGuests, _ := h.guests.ListHosts()
		hostIDs, _ := h.guests.HostIDsForShow(show.ID)
		hostItems := make([]pickerItem, len(hostGuests))
		for i, g := range hostGuests {
			hostItems[i] = pickerItem{ID: g.ID, Name: g.Name}
		}
		show.Website = website
		show.Color = color
		data := map[string]any{
			"Show":          show,
			"Error":         "Name is required",
			"HostItems":     hostItems,
			"LinkedHostIDs": hostIDs,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "shows/edit.html", data)
		return
	}

	// Handle artwork upload
	artwork := show.Artwork
	file, header, err := r.FormFile("artwork")
	if err == nil {
		defer file.Close()

		idStr := strconv.FormatInt(show.ID, 10)
		uploadDir := filepath.Join(h.dataDir, "uploads", "shows", idStr)
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
			return
		}

		ext := filepath.Ext(header.Filename)
		destPath := filepath.Join(uploadDir, "artwork"+ext)

		// Remove old artwork if it exists and differs
		if show.Artwork != "" {
			oldPath := filepath.Join(h.dataDir, "uploads", show.Artwork)
			if oldPath != destPath {
				os.Remove(oldPath)
			}
		}

		dest, err := os.Create(destPath)
		if err != nil {
			http.Error(w, "Failed to save file", http.StatusInternalServerError)
			return
		}
		defer dest.Close()

		if _, err := io.Copy(dest, file); err != nil {
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}

		artwork = fmt.Sprintf("shows/%s/artwork%s", idStr, ext)
	}

	// Handle artwork removal
	if r.FormValue("remove_artwork") == "1" {
		if show.Artwork != "" {
			os.Remove(filepath.Join(h.dataDir, "uploads", show.Artwork))
		}
		artwork = ""
	}

	if err := h.store.Update(show.ID, name, description, artwork, website, color); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save hosts
	var hostIDs []int64
	for _, s := range r.Form["host_ids"] {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			hostIDs = append(hostIDs, id)
		}
	}
	if err := h.guests.SetShowHosts(show.ID, hostIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/shows/"+strconv.FormatInt(show.ID, 10), http.StatusSeeOther)
}

func (h *ShowHandler) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAdmin(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
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
