package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// errBadUploadType signals that an uploaded file's extension isn't in the
// allowlist, so callers can map it to 400 rather than 500.
var errBadUploadType = errors.New("unsupported file type")

// uploadSpec describes a single fixed-name file upload (episode/show/guest
// artwork, sponsor order doc, site logo — all "one current file per record").
type uploadSpec struct {
	Field   string                      // multipart form field name
	DataDir string                      // app data dir; files land under <DataDir>/uploads
	Subdir  string                      // path under uploads/, e.g. "episodes/12" or "site"
	Base    string                      // filename without extension, e.g. "artwork"
	OldPath string                      // previous stored path to delete if it differs (optional)
	Allowed func(string) (string, bool) // extension allowlist (imageExt or docExt)
}

// saveUpload reads an optional multipart file, validates its extension, writes it
// to uploads/<Subdir>/<Base><ext>, removes any previous file at OldPath that
// differs, and returns the new path relative to uploads/ (forward slashes, for
// both the DB and URLs). Returns ("", false, nil) when no file was submitted and
// ("", false, errBadUploadType) for a disallowed extension.
func saveUpload(r *http.Request, spec uploadSpec) (relPath string, uploaded bool, err error) {
	file, header, err := r.FormFile(spec.Field)
	if err != nil {
		return "", false, nil // no file submitted
	}
	defer file.Close()

	ext, ok := spec.Allowed(header.Filename)
	if !ok {
		return "", false, errBadUploadType
	}

	uploadDir := filepath.Join(spec.DataDir, "uploads", filepath.FromSlash(spec.Subdir))
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", false, fmt.Errorf("creating upload dir: %w", err)
	}

	name := spec.Base + ext
	destPath := filepath.Join(uploadDir, name)

	// Remove a previous file only if it lived at a different path (a same-name
	// overwrite is handled by os.Create).
	if spec.OldPath != "" {
		if old := filepath.Join(spec.DataDir, "uploads", filepath.FromSlash(spec.OldPath)); old != destPath {
			_ = os.Remove(old)
		}
	}

	dest, err := os.Create(destPath)
	if err != nil {
		return "", false, fmt.Errorf("creating upload file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		return "", false, fmt.Errorf("writing upload file: %w", err)
	}

	return path.Join(spec.Subdir, name), true, nil
}

// removeEpisodeUploads deletes an episode's on-disk upload directories: fixed-
// name artwork under uploads/episodes/<id> and generic attachments under
// uploads/<id>. Best-effort — called after the DB row (and its cascaded asset
// rows) are already gone, so a filesystem error can't be surfaced to the user.
func removeEpisodeUploads(dataDir string, episodeID int64) {
	id := strconv.FormatInt(episodeID, 10)
	_ = os.RemoveAll(filepath.Join(dataDir, "uploads", "episodes", id))
	_ = os.RemoveAll(filepath.Join(dataDir, "uploads", id))
}

// removeShowUploads deletes a show's on-disk artwork directory
// (uploads/shows/<id>). Best-effort, like removeEpisodeUploads.
func removeShowUploads(dataDir string, showID int64) {
	_ = os.RemoveAll(filepath.Join(dataDir, "uploads", "shows", strconv.FormatInt(showID, 10)))
}

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
