package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skaldfm/skald/internal/auth"
	"github.com/skaldfm/skald/internal/models"
	"github.com/skaldfm/skald/internal/views"
)

type pickerItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type EpisodeHandler struct {
	episodes     *models.EpisodeStore
	shows        *models.ShowStore
	assets       *models.AssetStore
	guests       *models.GuestStore
	tags         *models.TagStore
	sponsorships *models.SponsorshipStore
	dataDir      string
}

func NewEpisodeHandler(episodes *models.EpisodeStore, shows *models.ShowStore, assets *models.AssetStore, guests *models.GuestStore, tags *models.TagStore, sponsorships *models.SponsorshipStore, dataDir string) *EpisodeHandler {
	return &EpisodeHandler{episodes: episodes, shows: shows, assets: assets, guests: guests, tags: tags, sponsorships: sponsorships, dataDir: dataDir}
}

func (h *EpisodeHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/new", h.New)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Show)
	r.Get("/{id}/edit", h.Edit)
	r.Post("/{id}", h.Update)
	r.Get("/next-number", h.NextNumber)
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

	user := auth.UserFromContext(r.Context())
	data := map[string]any{
		"Episodes": episodes,
		"Shows":    shows,
		"Filter":   filter,
		"Statuses": models.Statuses,
		"CanEdit":  auth.CanEdit(user),
	}
	if err := views.Render(w, r, "episodes/index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) New(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if !auth.CanEdit(user) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	shows, err := accessibleShows(r, h.shows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Shows":    shows,
		"Statuses": models.Statuses,
		"ShowID":   r.URL.Query().Get("show"),
	}
	if err := views.Render(w, r, "episodes/new.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !auth.CanEdit(auth.UserFromContext(r.Context())) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Parse multipart form (10 MB max for artwork)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Form too large", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	status := r.FormValue("status")
	showIDStr := r.FormValue("show_id")

	showID, _ := strconv.ParseInt(showIDStr, 10, 64)

	if showID > 0 && !auth.CanAccessShow(r.Context(), showID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if title == "" || showID == 0 {
		shows, _ := accessibleShows(r, h.shows)
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
		_ = views.Render(w, r, "episodes/new.html", data)
		return
	}

	// Parse episode/season numbers early for validation
	var epNumber, seNumber *int
	if epNum := r.FormValue("episode_number"); epNum != "" {
		n, _ := strconv.Atoi(epNum)
		epNumber = &n
	}
	if seNum := r.FormValue("season_number"); seNum != "" {
		n, _ := strconv.Atoi(seNum)
		seNumber = &n
	}

	// Check for duplicate episode number
	if epNumber != nil {
		exists, err := h.episodes.EpisodeNumberExists(showID, seNumber, epNumber, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if exists {
			shows, _ := accessibleShows(r, h.shows)
			code := views.EpisodeCode(seNumber, epNumber)
			data := map[string]any{
				"Error":       fmt.Sprintf("%s already exists in this show", code),
				"Title":       title,
				"Description": description,
				"Status":      status,
				"ShowID":      showIDStr,
				"Shows":       shows,
				"Statuses":    models.Statuses,
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = views.Render(w, r, "episodes/new.html", data)
			return
		}
	}

	ep, err := h.episodes.Create(showID, title, description, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set additional fields that Create() doesn't handle
	ep.EpisodeNumber = epNumber
	ep.SeasonNumber = seNumber
	if pd := r.FormValue("publish_date"); pd != "" {
		if t, err := time.Parse("2006-01-02", pd); err == nil {
			ep.PublishDate = &t
		}
	}
	ep.Script = r.FormValue("script")
	ep.ShowNotes = r.FormValue("show_notes")

	// Handle artwork upload
	file, header, artErr := r.FormFile("artwork")
	if artErr == nil {
		defer file.Close()

		idStr := strconv.FormatInt(ep.ID, 10)
		uploadDir := filepath.Join(h.dataDir, "uploads", "episodes", idStr)
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
			return
		}

		ext := filepath.Ext(header.Filename)
		destPath := filepath.Join(uploadDir, "artwork"+ext)

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

		ep.Artwork = fmt.Sprintf("episodes/%s/artwork%s", idStr, ext)
	}

	if err := h.episodes.Update(ep); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save tags
	tagsInput := strings.TrimSpace(r.FormValue("tags"))
	var tagNames []string
	if tagsInput != "" {
		for _, t := range strings.Split(tagsInput, ",") {
			if name := strings.TrimSpace(t); name != "" {
				tagNames = append(tagNames, name)
			}
		}
	}
	_ = h.tags.SetEpisodeTags(ep.ID, tagNames)

	// Auto-inherit show hosts
	showHostIDs, _ := h.guests.HostIDsForShow(showID)
	for _, hid := range showHostIDs {
		_ = h.guests.LinkGuest(ep.ID, hid, "host")
	}

	http.Redirect(w, r, "/episodes/"+strconv.FormatInt(ep.ID, 10), http.StatusSeeOther)
}

func (h *EpisodeHandler) Show(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}
	if !requireShowAccess(w, r, ep.ShowID) {
		return
	}

	user := auth.UserFromContext(r.Context())
	assets, _ := h.assets.ListForEpisode(ep.ID)
	hosts, _ := h.guests.HostsForEpisode(ep.ID)
	guests, _ := h.guests.GuestsForEpisode(ep.ID)
	tags, _ := h.tags.TagsForEpisode(ep.ID)
	sponsorships, _ := h.sponsorships.SponsorshipsForEpisode(ep.ID)

	data := map[string]any{
		"Episode":      ep,
		"Statuses":     models.Statuses,
		"Assets":       assets,
		"Hosts":        hosts,
		"Guests":       guests,
		"Tags":         tags,
		"Sponsorships": sponsorships,
		"CanEdit":      auth.CanEdit(user),
	}
	if err := views.Render(w, r, "episodes/show.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) Edit(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}
	if !requireShowEdit(w, r, ep.ShowID) {
		return
	}

	data := h.editData(r, ep, "")
	if err := views.Render(w, r, "episodes/edit.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *EpisodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}
	if !requireShowEdit(w, r, ep.ShowID) {
		return
	}

	// Parse multipart form (10 MB max for artwork)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Form too large", http.StatusBadRequest)
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
	} else {
		ep.EpisodeNumber = nil
	}
	if seNum := r.FormValue("season_number"); seNum != "" {
		n, _ := strconv.Atoi(seNum)
		ep.SeasonNumber = &n
	} else {
		ep.SeasonNumber = nil
	}
	if pd := r.FormValue("publish_date"); pd != "" {
		if t, err := time.Parse("2006-01-02", pd); err == nil {
			ep.PublishDate = &t
		}
	} else {
		ep.PublishDate = nil
	}

	// Check for duplicate episode number
	if ep.EpisodeNumber != nil {
		exists, err := h.episodes.EpisodeNumberExists(ep.ShowID, ep.SeasonNumber, ep.EpisodeNumber, ep.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if exists {
			code := views.EpisodeCode(ep.SeasonNumber, ep.EpisodeNumber)
			data := h.editData(r, ep, fmt.Sprintf("%s already exists in this show", code))
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = views.Render(w, r, "episodes/edit.html", data)
			return
		}
	}

	// Handle artwork upload
	file, header, err := r.FormFile("artwork")
	if err == nil {
		defer file.Close()

		idStr := strconv.FormatInt(ep.ID, 10)
		uploadDir := filepath.Join(h.dataDir, "uploads", "episodes", idStr)
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
			return
		}

		ext := filepath.Ext(header.Filename)
		destPath := filepath.Join(uploadDir, "artwork"+ext)

		// Remove old artwork if it exists and differs
		if ep.Artwork != "" {
			oldPath := filepath.Join(h.dataDir, "uploads", ep.Artwork)
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

		ep.Artwork = fmt.Sprintf("episodes/%s/artwork%s", idStr, ext)
	}

	// Handle artwork removal
	if r.FormValue("remove_artwork") == "1" {
		if ep.Artwork != "" {
			os.Remove(filepath.Join(h.dataDir, "uploads", ep.Artwork))
		}
		ep.Artwork = ""
	}

	if ep.Title == "" {
		data := h.editData(r, ep, "Title is required")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = views.Render(w, r, "episodes/edit.html", data)
		return
	}

	if err := h.episodes.Update(ep); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update tags
	tagsInput := strings.TrimSpace(r.FormValue("tags"))
	var tagNames []string
	if tagsInput != "" {
		for _, t := range strings.Split(tagsInput, ",") {
			if name := strings.TrimSpace(t); name != "" {
				tagNames = append(tagNames, name)
			}
		}
	}
	_ = h.tags.SetEpisodeTags(ep.ID, tagNames)

	// Update sponsorship links
	selectedSponsorships := r.Form["sponsorship_ids"]
	selectedMap := make(map[int64]bool)
	for _, s := range selectedSponsorships {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			selectedMap[id] = true
		}
	}
	// Get current links and diff
	currentIDs, _ := h.sponsorships.SponsorshipIDsForEpisode(ep.ID)
	currentMap := make(map[int64]bool)
	for _, id := range currentIDs {
		currentMap[id] = true
	}
	// Add new links
	for id := range selectedMap {
		if !currentMap[id] {
			_ = h.sponsorships.LinkEpisode(id, ep.ID)
		}
	}
	// Remove old links
	for _, id := range currentIDs {
		if !selectedMap[id] {
			_ = h.sponsorships.UnlinkEpisode(id, ep.ID)
		}
	}

	// Update guest links
	selectedGuests := r.Form["guest_ids"]
	selectedGuestMap := make(map[int64]bool)
	for _, s := range selectedGuests {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			selectedGuestMap[id] = true
		}
	}
	currentGuestIDs, _ := h.guests.GuestIDsForEpisode(ep.ID)
	currentGuestMap := make(map[int64]bool)
	for _, id := range currentGuestIDs {
		currentGuestMap[id] = true
	}
	for id := range selectedGuestMap {
		if !currentGuestMap[id] {
			_ = h.guests.LinkGuest(ep.ID, id, "guest")
		}
	}
	for _, id := range currentGuestIDs {
		if !selectedGuestMap[id] {
			_ = h.guests.UnlinkGuest(ep.ID, id)
		}
	}

	// Update host links
	selectedHosts := r.Form["host_ids"]
	selectedHostMap := make(map[int64]bool)
	for _, s := range selectedHosts {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			selectedHostMap[id] = true
		}
	}
	currentHostIDs, _ := h.guests.HostIDsForEpisode(ep.ID)
	currentHostMap := make(map[int64]bool)
	for _, id := range currentHostIDs {
		currentHostMap[id] = true
	}
	for id := range selectedHostMap {
		if !currentHostMap[id] {
			_ = h.guests.LinkGuest(ep.ID, id, "host")
		}
	}
	for _, id := range currentHostIDs {
		if !selectedHostMap[id] {
			_ = h.guests.UnlinkGuest(ep.ID, id)
		}
	}

	http.Redirect(w, r, "/episodes/"+strconv.FormatInt(ep.ID, 10), http.StatusSeeOther)
}

func (h *EpisodeHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	ep, err := h.getEpisode(w, r)
	if ep == nil || err != nil {
		return
	}
	if !requireShowEdit(w, r, ep.ShowID) {
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
		_, _ = w.Write([]byte(`<span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ` +
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
	if !requireShowEdit(w, r, ep.ShowID) {
		return
	}

	if err := h.episodes.Delete(ep.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/episodes", http.StatusSeeOther)
}

func (h *EpisodeHandler) NextNumber(w http.ResponseWriter, r *http.Request) {
	showIDStr := r.URL.Query().Get("show_id")
	showID, _ := strconv.ParseInt(showIDStr, 10, 64)
	if showID == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	var season *int
	if s := r.URL.Query().Get("season_number"); s != "" {
		n, _ := strconv.Atoi(s)
		season = &n
	}

	taken, err := h.episodes.TakenEpisodeNumbers(showID, season)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find next available number
	next := 1
	takenSet := make(map[int]bool, len(taken))
	for _, n := range taken {
		takenSet[n] = true
	}
	for takenSet[next] {
		next++
	}

	w.Header().Set("Content-Type", "text/html")
	if len(taken) > 0 {
		fmt.Fprintf(w, `<span class="text-xs text-gray-500 dark:text-gray-400">Next available: %d (taken: %s)</span>`,
			next, formatIntSlice(taken))
	} else {
		fmt.Fprintf(w, `<span class="text-xs text-gray-500 dark:text-gray-400">Next available: %d</span>`, next)
	}
}

func formatIntSlice(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

func (h *EpisodeHandler) editData(r *http.Request, ep *models.Episode, errMsg string) map[string]any {
	shows, _ := accessibleShows(r, h.shows)
	tags, _ := h.tags.TagsForEpisode(ep.ID)
	var tagNames []string
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}
	allSp, _ := h.sponsorships.List()
	linkedSpIDs, _ := h.sponsorships.SponsorshipIDsForEpisode(ep.ID)
	allG, _ := h.guests.List()
	hostG, _ := h.guests.ListHosts()
	linkedGIDs, _ := h.guests.GuestIDsForEpisode(ep.ID)
	linkedHostIDs, _ := h.guests.HostIDsForEpisode(ep.ID)

	guestItems := make([]pickerItem, len(allG))
	for i, g := range allG {
		guestItems[i] = pickerItem{ID: g.ID, Name: g.Name}
	}
	hostItems := make([]pickerItem, len(hostG))
	for i, g := range hostG {
		hostItems[i] = pickerItem{ID: g.ID, Name: g.Name}
	}
	sponsorItems := make([]pickerItem, len(allSp))
	for i, s := range allSp {
		sponsorItems[i] = pickerItem{ID: s.ID, Name: s.Name}
	}

	data := map[string]any{
		"Episode":              ep,
		"Shows":                shows,
		"Statuses":             models.Statuses,
		"Tags":                 strings.Join(tagNames, ", "),
		"HostItems":            hostItems,
		"LinkedHostIDs":        linkedHostIDs,
		"GuestItems":           guestItems,
		"LinkedGuestIDs":       linkedGIDs,
		"SponsorItems":         sponsorItems,
		"LinkedSponsorshipIDs": linkedSpIDs,
	}
	if errMsg != "" {
		data["Error"] = errMsg
	}
	return data
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
