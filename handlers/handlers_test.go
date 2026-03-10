package handlers_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kristofer/postdoc/db"
	"github.com/kristofer/postdoc/handlers"
	"html/template"
)

// minimalPDF is the smallest valid PDF binary we can construct in-memory.
var minimalPDF = []byte("%PDF-1.0\n1 0 obj<</Type /Catalog>>endobj\nxref\n0 2\n0000000000 65535 f \n0000000009 00000 n \ntrailer<</Size 2/Root 1 0 R>>\nstartxref\n9\n%%EOF")

func setupDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database, dir
}

func makeTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.New("success.html").Parse(`{{.ShortURL}}`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return tmpl
}

func uploadRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadHandler_SuccessfulPDF(t *testing.T) {
	database, dir := setupDB(t)
	tmpl := makeTemplate(t)

	h := handlers.UploadHandler(database, dir, tmpl, "http://localhost:8080")
	req := uploadRequest(t, "document", "my-report.pdf", minimalPDF)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "http://localhost:8080/my-report.pdf") {
		t.Errorf("expected short URL with .pdf extension in body, got: %s", body)
	}

	// Confirm the file was written on disk.
	entries, _ := os.ReadDir(dir)
	var found bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pdf") {
			found = true
		}
	}
	if !found {
		t.Error("expected a .pdf file in the upload directory")
	}
}

func TestUploadHandler_NonPDFRejected(t *testing.T) {
	database, dir := setupDB(t)
	tmpl := makeTemplate(t)

	h := handlers.UploadHandler(database, dir, tmpl, "http://localhost:8080")
	req := uploadRequest(t, "document", "image.png", []byte("\x89PNG\r\n\x1a\n"+strings.Repeat("x", 100)))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-PDF, got %d", rr.Code)
	}
}

func TestUploadHandler_MissingField(t *testing.T) {
	database, dir := setupDB(t)
	tmpl := makeTemplate(t)

	h := handlers.UploadHandler(database, dir, tmpl, "http://localhost:8080")
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=----")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing field, got %d", rr.Code)
	}
}

func TestServeDocument_NotFound(t *testing.T) {
	database, _ := setupDB(t)
	h := handlers.ServeDocument(database)

	req := httptest.NewRequest(http.MethodGet, "/no-such-doc", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServeDocument_Found(t *testing.T) {
	database, dir := setupDB(t)

	// Write a fake PDF to disk.
	pdfPath := filepath.Join(dir, "hello-world.pdf")
	if err := os.WriteFile(pdfPath, minimalPDF, 0o640); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	// Insert a record into the DB.
	if _, err := database.InsertDocument("hello-world", "Hello World.pdf", pdfPath); err != nil {
		t.Fatalf("insert: %v", err)
	}

	h := handlers.ServeDocument(database)
	req := httptest.NewRequest(http.MethodGet, "/hello-world", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/pdf") {
		t.Errorf("expected application/pdf content-type, got %s", ct)
	}

	body, _ := io.ReadAll(rr.Body)
	if !bytes.Equal(body, minimalPDF) {
		t.Error("served file content does not match uploaded content")
	}
}

func TestSlugCollisionDeduplication(t *testing.T) {
	database, dir := setupDB(t)
	tmpl := makeTemplate(t)
	h := handlers.UploadHandler(database, dir, tmpl, "http://localhost:8080")

	// Upload two PDFs with the same base name.
	for i := 0; i < 2; i++ {
		req := uploadRequest(t, "document", "report.pdf", minimalPDF)
		rr := httptest.NewRecorder()
		h(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("upload %d failed with %d: %s", i, rr.Code, rr.Body.String())
		}
	}

	// Both slugs should be present and differ.
	doc1, err := database.GetBySlug("report.pdf")
	if err != nil || doc1 == nil {
		t.Fatal("expected slug 'report.pdf' to exist")
	}
	doc2, err := database.GetBySlug("report-1.pdf")
	if err != nil || doc2 == nil {
		t.Fatal("expected slug 'report-1.pdf' to exist")
	}
}
