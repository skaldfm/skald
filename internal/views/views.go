package views

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/justinas/nosurf"
	"github.com/mhermansson/skald/internal/auth"
	"github.com/yuin/goldmark"
)

var (
	templates    map[string]*template.Template
	mu           sync.RWMutex
	logoPathFunc func() string
)

// SetLogoPathFunc sets the function used to resolve the current site logo path.
func SetLogoPathFunc(fn func() string) {
	logoPathFunc = fn
}

// FuncMap returns the shared template function map.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"statusLabel":    StatusLabel,
		"statusColor":    StatusColor,
		"statusBarColor": StatusBarColor,
		"formatBytes":    formatBytes,
		"episodeCode":    EpisodeCode,
		"renderMarkdown": renderMarkdown,
		"formatCurrency": formatCurrency,
		"formatFloat":    formatFloat,
		"contains":       containsInt64,
		"toJSON":          toJSON,
	}
}

func StatusLabel(s string) string {
	labels := map[string]string{
		"idea":      "Idea",
		"research":  "Research",
		"scripted":  "Scripted",
		"recorded":  "Recorded",
		"edited":    "Edited",
		"published": "Published",
	}
	if l, ok := labels[s]; ok {
		return l
	}
	return s
}

func StatusColor(s string) string {
	colors := map[string]string{
		"idea":      "bg-gray-100 text-gray-800",
		"research":  "bg-blue-100 text-blue-800",
		"scripted":  "bg-yellow-100 text-yellow-800",
		"recorded":  "bg-purple-100 text-purple-800",
		"edited":    "bg-orange-100 text-orange-800",
		"published": "bg-green-100 text-green-800",
	}
	if c, ok := colors[s]; ok {
		return c
	}
	return "bg-gray-100 text-gray-800"
}

func StatusBarColor(s string) string {
	colors := map[string]string{
		"idea":      "bg-gray-300 dark:bg-gray-500",
		"research":  "bg-blue-400 dark:bg-blue-500",
		"scripted":  "bg-yellow-400 dark:bg-yellow-500",
		"recorded":  "bg-purple-400 dark:bg-purple-500",
		"edited":    "bg-orange-400 dark:bg-orange-500",
		"published": "bg-green-400 dark:bg-green-500",
	}
	if c, ok := colors[s]; ok {
		return c
	}
	return "bg-gray-300 dark:bg-gray-500"
}

func EpisodeCode(season, episode *int) string {
	if episode == nil {
		return ""
	}
	if season != nil {
		return fmt.Sprintf("S%02dE%02d", *season, *episode)
	}
	return fmt.Sprintf("E%02d", *episode)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func renderMarkdown(s string) template.HTML {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(s), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}
	return template.HTML(buf.String())
}

func formatCurrency(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("$%.2f", *f)
}

func formatFloat(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *f)
}

func toJSON(v any) template.HTMLAttr {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return template.HTMLAttr(b)
}

func containsInt64(slice []int64, val int64) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// Load parses all templates from the templates directory.
func Load(templatesDir string) error {
	mu.Lock()
	defer mu.Unlock()

	templates = make(map[string]*template.Template)
	layout := filepath.Join(templatesDir, "layouts", "base.html")
	authLayout := filepath.Join(templatesDir, "layouts", "auth.html")
	components, _ := filepath.Glob(filepath.Join(templatesDir, "components", "*.html"))

	// Parse each page template with the layout and components
	pageDirs := []string{"shows", "episodes", "guests", "sponsorships", "prompter", "admin"}
	for _, dir := range pageDirs {
		pages, _ := filepath.Glob(filepath.Join(templatesDir, dir, "*.html"))
		for _, page := range pages {
			name := filepath.Base(page)
			files := append([]string{layout, page}, components...)
			t, err := template.New(filepath.Base(layout)).Funcs(FuncMap()).ParseFiles(files...)
			if err != nil {
				return fmt.Errorf("parsing template %s: %w", name, err)
			}
			key := dir + "/" + name
			templates[key] = t
		}
	}

	// Parse auth templates with the auth layout
	authPages, _ := filepath.Glob(filepath.Join(templatesDir, "auth", "*.html"))
	for _, page := range authPages {
		name := filepath.Base(page)
		t, err := template.New(filepath.Base(authLayout)).Funcs(FuncMap()).ParseFiles(authLayout, page)
		if err != nil {
			return fmt.Errorf("parsing auth template %s: %w", name, err)
		}
		templates["auth/"+name] = t
	}

	// Also parse standalone pages (like home)
	standalones, _ := filepath.Glob(filepath.Join(templatesDir, "*.html"))
	for _, page := range standalones {
		name := filepath.Base(page)
		files := append([]string{layout, page}, components...)
		t, err := template.New(filepath.Base(layout)).Funcs(FuncMap()).ParseFiles(files...)
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", name, err)
		}
		templates[name] = t
	}

	return nil
}

// injectContext adds CurrentUser and CSRFToken to the template data map.
func injectContext(r *http.Request, data any) map[string]any {
	var m map[string]any
	if data == nil {
		m = make(map[string]any)
	} else if existing, ok := data.(map[string]any); ok {
		m = existing
	} else {
		// Data is a struct or something else — can't inject, return as-is wrapper
		m = map[string]any{"Data": data}
	}

	if r != nil {
		m["CSRFToken"] = nosurf.Token(r)
		m["CurrentUser"] = auth.UserFromContext(r.Context())
	}

	// Inject logo path
	logoPath := "/static/images/logo.png"
	if logoPathFunc != nil {
		if p := logoPathFunc(); p != "" {
			logoPath = "/uploads/" + p
		}
	}
	m["LogoPath"] = logoPath

	return m
}

// Render executes a named template with the given data, auto-injecting
// CurrentUser and CSRFToken from the request context.
func Render(w io.Writer, r *http.Request, name string, data any) error {
	mu.RLock()
	t, ok := templates[name]
	mu.RUnlock()

	if !ok {
		return fmt.Errorf("template %q not found", name)
	}

	return t.ExecuteTemplate(w, "base", injectContext(r, data))
}

// RenderAuth executes a named auth template with the auth layout.
func RenderAuth(w io.Writer, r *http.Request, name string, data any) error {
	mu.RLock()
	t, ok := templates[name]
	mu.RUnlock()

	if !ok {
		return fmt.Errorf("template %q not found", name)
	}

	return t.ExecuteTemplate(w, "auth", injectContext(r, data))
}
