package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mhermansson/skald/internal/models"
	"github.com/mhermansson/skald/internal/views"
)

type CalendarHandler struct {
	episodes *models.EpisodeStore
	shows    *models.ShowStore
}

func NewCalendarHandler(episodes *models.EpisodeStore, shows *models.ShowStore) *CalendarHandler {
	return &CalendarHandler{episodes: episodes, shows: shows}
}

func (h *CalendarHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Calendar)
	return r
}

type CalendarDay struct {
	Day      int
	InMonth  bool
	IsToday  bool
	Episodes []models.Episode
}

func (h *CalendarHandler) Calendar(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if y := r.URL.Query().Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		}
	}
	if m := r.URL.Query().Get("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v >= 1 && v <= 12 {
			month = v
		}
	}

	filter := models.EpisodeFilter{}
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

	// Group episodes by day-of-month for the target month
	byDay := make(map[int][]models.Episode)
	var unscheduled []models.Episode
	for _, ep := range episodes {
		if ep.PublishDate == nil {
			unscheduled = append(unscheduled, ep)
			continue
		}
		if ep.PublishDate.Year() == year && int(ep.PublishDate.Month()) == month {
			day := ep.PublishDate.Day()
			byDay[day] = append(byDay[day], ep)
		}
	}

	// Build calendar grid
	firstOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	daysInMonth := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.Local).Day()

	// Monday = 0, Sunday = 6
	weekday := int(firstOfMonth.Weekday())
	if weekday == 0 {
		weekday = 6 // Sunday
	} else {
		weekday-- // Shift so Monday = 0
	}

	today := now.Day()
	isCurrentMonth := now.Year() == year && int(now.Month()) == month

	var weeks [][]CalendarDay
	var week []CalendarDay

	// Leading empty cells
	for i := 0; i < weekday; i++ {
		week = append(week, CalendarDay{InMonth: false})
	}

	for day := 1; day <= daysInMonth; day++ {
		cd := CalendarDay{
			Day:      day,
			InMonth:  true,
			IsToday:  isCurrentMonth && day == today,
			Episodes: byDay[day],
		}
		week = append(week, cd)
		if len(week) == 7 {
			weeks = append(weeks, week)
			week = nil
		}
	}

	// Trailing empty cells
	if len(week) > 0 {
		for len(week) < 7 {
			week = append(week, CalendarDay{InMonth: false})
		}
		weeks = append(weeks, week)
	}

	// Prev/next month
	prevMonth := time.Date(year, time.Month(month)-1, 1, 0, 0, 0, 0, time.Local)
	nextMonth := time.Date(year, time.Month(month)+1, 1, 0, 0, 0, 0, time.Local)

	data := map[string]any{
		"Weeks":       weeks,
		"Year":        year,
		"Month":       month,
		"MonthName":   time.Month(month).String(),
		"PrevYear":    prevMonth.Year(),
		"PrevMonth":   int(prevMonth.Month()),
		"NextYear":    nextMonth.Year(),
		"NextMonth":   int(nextMonth.Month()),
		"Shows":       shows,
		"Filter":      filter,
		"Unscheduled": unscheduled,
	}

	if err := views.Render(w, "episodes/calendar.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type TimelineHandler struct {
	episodes *models.EpisodeStore
	shows    *models.ShowStore
}

func NewTimelineHandler(episodes *models.EpisodeStore, shows *models.ShowStore) *TimelineHandler {
	return &TimelineHandler{episodes: episodes, shows: shows}
}

func (h *TimelineHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Timeline)
	return r
}

type TimelineColumn struct {
	Name     string
	IsCurrent bool
	Episodes []models.Episode
}

func (h *TimelineHandler) Timeline(w http.ResponseWriter, r *http.Request) {
	filter := models.EpisodeFilter{}
	if showID := r.URL.Query().Get("show"); showID != "" {
		id, _ := strconv.ParseInt(showID, 10, 64)
		filter.ShowID = id
	}

	zoom := r.URL.Query().Get("zoom")
	if zoom != "week" {
		zoom = "month"
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

	now := time.Now()
	var columns []TimelineColumn
	var unscheduled []models.Episode
	todayIdx := -1

	if zoom == "week" {
		// 8 weeks back, 8 weeks ahead (17 weeks total)
		// Find Monday of the current week
		todayWeekday := int(now.Weekday())
		if todayWeekday == 0 {
			todayWeekday = 7
		}
		thisMonday := now.AddDate(0, 0, -(todayWeekday - 1))
		thisMonday = time.Date(thisMonday.Year(), thisMonday.Month(), thisMonday.Day(), 0, 0, 0, 0, time.Local)

		start := thisMonday.AddDate(0, 0, -8*7)
		end := thisMonday.AddDate(0, 0, 9*7) // 8 weeks ahead + end of that week

		// Group episodes by week (Monday)
		type weekKey struct {
			year int
			month int
			day  int
		}
		grouped := make(map[weekKey][]models.Episode)
		for _, ep := range episodes {
			if ep.PublishDate == nil {
				unscheduled = append(unscheduled, ep)
				continue
			}
			if ep.PublishDate.Before(start) || !ep.PublishDate.Before(end) {
				continue
			}
			// Find the Monday of this episode's week
			epWeekday := int(ep.PublishDate.Weekday())
			if epWeekday == 0 {
				epWeekday = 7
			}
			monday := ep.PublishDate.AddDate(0, 0, -(epWeekday - 1))
			k := weekKey{monday.Year(), int(monday.Month()), monday.Day()}
			grouped[k] = append(grouped[k], ep)
		}

		// Build week columns
		cur := start
		for cur.Before(end) {
			sunday := cur.AddDate(0, 0, 6)
			var name string
			if cur.Month() == sunday.Month() {
				name = cur.Format("Jan 2") + "–" + strconv.Itoa(sunday.Day())
			} else {
				name = cur.Format("Jan 2") + "–" + sunday.Format("Jan 2")
			}

			k := weekKey{cur.Year(), int(cur.Month()), cur.Day()}
			isCurrent := cur.Equal(thisMonday)
			columns = append(columns, TimelineColumn{
				Name:      name,
				IsCurrent: isCurrent,
				Episodes:  grouped[k],
			})
			if isCurrent {
				todayIdx = len(columns) - 1
			}
			cur = cur.AddDate(0, 0, 7)
		}
	} else {
		// Month view: 3 months back to 3 months ahead
		start := time.Date(now.Year(), now.Month()-3, 1, 0, 0, 0, 0, time.Local)
		end := time.Date(now.Year(), now.Month()+4, 0, 0, 0, 0, 0, time.Local)

		type monthKey struct {
			year  int
			month int
		}
		grouped := make(map[monthKey][]models.Episode)
		for _, ep := range episodes {
			if ep.PublishDate == nil {
				unscheduled = append(unscheduled, ep)
				continue
			}
			if ep.PublishDate.Before(start) || ep.PublishDate.After(end) {
				continue
			}
			k := monthKey{ep.PublishDate.Year(), int(ep.PublishDate.Month())}
			grouped[k] = append(grouped[k], ep)
		}

		cur := start
		for !cur.After(end) {
			k := monthKey{cur.Year(), int(cur.Month())}
			isCurrent := cur.Year() == now.Year() && cur.Month() == now.Month()
			columns = append(columns, TimelineColumn{
				Name:      cur.Month().String()[:3] + " " + strconv.Itoa(cur.Year()),
				IsCurrent: isCurrent,
				Episodes:  grouped[k],
			})
			if isCurrent {
				todayIdx = len(columns) - 1
			}
			cur = time.Date(cur.Year(), cur.Month()+1, 1, 0, 0, 0, 0, time.Local)
		}
	}

	data := map[string]any{
		"Columns":     columns,
		"Shows":       shows,
		"Filter":      filter,
		"Unscheduled": unscheduled,
		"TodayIdx":    todayIdx,
		"Zoom":        zoom,
	}

	if err := views.Render(w, "episodes/timeline.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
