package views

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"sync"
)

var (
	templates map[string]*template.Template
	mu        sync.RWMutex
)

// FuncMap returns the shared template function map.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"statusLabel":    StatusLabel,
		"statusColor":    StatusColor,
		"statusBarColor": StatusBarColor,
		"formatBytes":    formatBytes,
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

// Load parses all templates from the templates directory.
func Load(templatesDir string) error {
	mu.Lock()
	defer mu.Unlock()

	templates = make(map[string]*template.Template)
	layout := filepath.Join(templatesDir, "layouts", "base.html")
	components, _ := filepath.Glob(filepath.Join(templatesDir, "components", "*.html"))

	// Parse each page template with the layout and components
	pageDirs := []string{"shows", "episodes", "guests", "prompter"}
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

// Render executes a named template with the given data.
func Render(w io.Writer, name string, data any) error {
	mu.RLock()
	t, ok := templates[name]
	mu.RUnlock()

	if !ok {
		return fmt.Errorf("template %q not found", name)
	}

	return t.ExecuteTemplate(w, "base", data)
}
