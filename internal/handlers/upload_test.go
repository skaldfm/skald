package handlers

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// fileRequest builds a multipart POST carrying a single file field.
func fileRequest(field, filename, content string) *http.Request {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile(field, filename)
	_, _ = fw.Write([]byte(content))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestSaveUpload(t *testing.T) {
	dataDir := t.TempDir()

	t.Run("no file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
		rel, uploaded, err := saveUpload(req, uploadSpec{Field: "artwork", DataDir: dataDir, Subdir: "e/1", Base: "artwork", Allowed: imageExt})
		if err != nil || uploaded || rel != "" {
			t.Fatalf("no-file = (%q,%v,%v), want (\"\",false,nil)", rel, uploaded, err)
		}
	})

	t.Run("bad extension", func(t *testing.T) {
		req := fileRequest("artwork", "evil.svg", "<svg/>")
		_, uploaded, err := saveUpload(req, uploadSpec{Field: "artwork", DataDir: dataDir, Subdir: "e/1", Base: "artwork", Allowed: imageExt})
		if !errors.Is(err, errBadUploadType) || uploaded {
			t.Fatalf("bad-ext err = %v, uploaded = %v; want errBadUploadType, false", err, uploaded)
		}
	})

	t.Run("valid save then replace with different ext removes old", func(t *testing.T) {
		req := fileRequest("artwork", "pic.png", "PNGDATA")
		rel, uploaded, err := saveUpload(req, uploadSpec{Field: "artwork", DataDir: dataDir, Subdir: "e/2", Base: "artwork", Allowed: imageExt})
		if err != nil || !uploaded {
			t.Fatalf("valid save = (%q,%v,%v)", rel, uploaded, err)
		}
		if rel != "e/2/artwork.png" {
			t.Errorf("relPath = %q, want e/2/artwork.png", rel)
		}
		oldAbs := filepath.Join(dataDir, "uploads", "e/2", "artwork.png")
		if _, err := os.Stat(oldAbs); err != nil {
			t.Fatalf("expected written file: %v", err)
		}

		// Replacing with a different extension must delete the previous file.
		req2 := fileRequest("artwork", "pic.jpg", "JPGDATA")
		rel2, _, err := saveUpload(req2, uploadSpec{Field: "artwork", DataDir: dataDir, Subdir: "e/2", Base: "artwork", OldPath: rel, Allowed: imageExt})
		if err != nil {
			t.Fatalf("replace save: %v", err)
		}
		if rel2 != "e/2/artwork.jpg" {
			t.Errorf("relPath = %q, want e/2/artwork.jpg", rel2)
		}
		if _, err := os.Stat(oldAbs); !os.IsNotExist(err) {
			t.Errorf("old file should have been removed, stat err = %v", err)
		}
	})
}
