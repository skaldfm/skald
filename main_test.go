package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestUploadsForceDownload proves the /uploads server neutralizes stored-XSS:
// an uploaded file that keeps its extension (here .html) is served with
// Content-Disposition: attachment so a browser downloads it instead of running
// it as script on the app's origin.
func TestUploadsForceDownload(t *testing.T) {
	dir := t.TempDir()
	uploads := filepath.Join(dir, "uploads", "12")
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploads, "evil.html"),
		[]byte("<script>alert(1)</script>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := forceDownload(http.StripPrefix("/uploads/",
		http.FileServer(noListFS{http.Dir(filepath.Join(dir, "uploads"))})))

	req := httptest.NewRequest(http.MethodGet, "/uploads/12/evil.html", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "attachment" {
		t.Fatalf("Content-Disposition = %q, want %q", got, "attachment")
	}
}

// TestUploadsNoDirectoryListing confirms the tree can't be enumerated.
func TestUploadsNoDirectoryListing(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "uploads", "12"), 0o755); err != nil {
		t.Fatal(err)
	}

	srv := forceDownload(http.StripPrefix("/uploads/",
		http.FileServer(noListFS{http.Dir(filepath.Join(dir, "uploads"))})))

	req := httptest.NewRequest(http.MethodGet, "/uploads/12/", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("directory listing status = %d, want 404", rec.Code)
	}
}
