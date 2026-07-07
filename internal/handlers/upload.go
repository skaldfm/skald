package handlers

import (
	"path/filepath"
	"strings"
)

// Allowed upload extensions. SVG is deliberately excluded from images because
// it can carry inline scripts; user uploads are served from the app's own
// origin, so an SVG/HTML upload would be a stored-XSS vector.
var (
	imageExts = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	}
	docExts = map[string]bool{
		".pdf": true, ".doc": true, ".docx": true, ".txt": true,
		".rtf": true, ".odt": true, ".csv": true, ".xlsx": true,
	}
)

// imageExt returns the lower-cased extension of filename if it is an allowed
// image type, and whether it is allowed.
func imageExt(filename string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext, imageExts[ext]
}

// docExt returns the lower-cased extension of filename if it is an allowed
// document type (for sponsor order files), and whether it is allowed.
func docExt(filename string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext, docExts[ext]
}
