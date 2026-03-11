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

func makeServeTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.New("email_prompt.html").Parse(`EMAIL_PROMPT:{{.Slug}}`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return tmpl
}

func TestServeDocument_NotFound(t *testing.T) {
	database, _ := setupDB(t)
	h := handlers.ServeDocument(database, makeServeTemplate(t))

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

	// Insert a record into the DB (tracking disabled, so PDF served directly).
	if _, err := database.InsertDocument("hello-world", "Hello World.pdf", pdfPath, "", 0, false); err != nil {
		t.Fatalf("insert: %v", err)
	}

	h := handlers.ServeDocument(database, makeServeTemplate(t))
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

func uploadRequestWithTracking(t *testing.T, filename string, content []byte, trackDownloads bool) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("document", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if trackDownloads {
		if err := w.WriteField("track_downloads", "on"); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadHandler_TrackDownloads(t *testing.T) {
	database, dir := setupDB(t)
	tmpl := makeTemplate(t)

	h := handlers.UploadHandler(database, dir, tmpl, "http://localhost:8080")
	req := uploadRequestWithTracking(t, "tracked.pdf", minimalPDF, true)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	doc, err := database.GetBySlug("tracked.pdf")
	if err != nil || doc == nil {
		t.Fatal("expected slug 'tracked.pdf' to exist")
	}
	if !doc.TrackDownloads {
		t.Error("expected TrackDownloads to be true")
	}
}

func TestServeDocument_TrackingShowsEmailForm(t *testing.T) {
	database, dir := setupDB(t)

	pdfPath := filepath.Join(dir, "secret.pdf")
	if err := os.WriteFile(pdfPath, minimalPDF, 0o640); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	// Insert a tracking-enabled document.
	if _, err := database.InsertDocument("secret.pdf", "Secret.pdf", pdfPath, "", 0, true); err != nil {
		t.Fatalf("insert: %v", err)
	}

	emailTmpl, err := template.New("email_prompt.html").Parse(`EMAIL_PROMPT:{{.Slug}}`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	h := handlers.ServeDocument(database, emailTmpl)
	req := httptest.NewRequest(http.MethodGet, "/secret.pdf", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "EMAIL_PROMPT:secret.pdf") {
		t.Errorf("expected email prompt in body, got: %s", body)
	}
}

func TestServeDocumentPost_ValidEmail(t *testing.T) {
	database, dir := setupDB(t)

	pdfPath := filepath.Join(dir, "tracked.pdf")
	if err := os.WriteFile(pdfPath, minimalPDF, 0o640); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	docID, err := database.InsertDocument("tracked.pdf", "Tracked.pdf", pdfPath, "", 0, true)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	emailTmpl, err := template.New("email_prompt.html").Parse(`EMAIL_PROMPT`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	h := handlers.ServeDocumentPost(database, emailTmpl)
	form := strings.NewReader("email=user%40example.com")
	req := httptest.NewRequest(http.MethodPost, "/tracked.pdf", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/pdf") {
		t.Errorf("expected application/pdf, got %s", ct)
	}

	// Confirm download event was recorded.
	events, err := database.ListDownloadEvents(docID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Email != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got %q", events[0].Email)
	}
}

func TestServeDocumentPost_InvalidEmail(t *testing.T) {
	database, dir := setupDB(t)

	pdfPath := filepath.Join(dir, "tracked2.pdf")
	if err := os.WriteFile(pdfPath, minimalPDF, 0o640); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	if _, err := database.InsertDocument("tracked2.pdf", "Tracked2.pdf", pdfPath, "", 0, true); err != nil {
		t.Fatalf("insert: %v", err)
	}

	emailTmpl, err := template.New("email_prompt.html").Parse(`ERROR:{{.Error}}`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	h := handlers.ServeDocumentPost(database, emailTmpl)
	form := strings.NewReader("email=not-an-email")
	req := httptest.NewRequest(http.MethodPost, "/tracked2.pdf", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render), got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "ERROR:") {
		t.Errorf("expected error message in body, got: %s", body)
	}
	ct := rr.Header().Get("Content-Type")
	if strings.HasPrefix(ct, "application/pdf") {
		t.Error("should not serve PDF for invalid email")
	}
}

func TestServeDocumentPost_NoTracking_Redirects(t *testing.T) {
	database, dir := setupDB(t)

	pdfPath := filepath.Join(dir, "notrack.pdf")
	if err := os.WriteFile(pdfPath, minimalPDF, 0o640); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	if _, err := database.InsertDocument("notrack.pdf", "NoTrack.pdf", pdfPath, "", 0, false); err != nil {
		t.Fatalf("insert: %v", err)
	}

	emailTmpl, err := template.New("email_prompt.html").Parse(`EMAIL_PROMPT`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	h := handlers.ServeDocumentPost(database, emailTmpl)
	form := strings.NewReader("email=user%40example.com")
	req := httptest.NewRequest(http.MethodPost, "/notrack.pdf", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/notrack.pdf" {
		t.Errorf("expected redirect to /notrack.pdf, got %q", loc)
	}
}

func TestServeDocumentPost_MultipleDownloads_CountCorrect(t *testing.T) {
database, dir := setupDB(t)

pdfPath := filepath.Join(dir, "multidown.pdf")
if err := os.WriteFile(pdfPath, minimalPDF, 0o640); err != nil {
t.Fatalf("write pdf: %v", err)
}

docID, err := database.InsertDocument("multidown.pdf", "MultiDown.pdf", pdfPath, "", 0, true)
if err != nil {
t.Fatalf("insert: %v", err)
}

emailTmpl, err := template.New("email_prompt.html").Parse(`EMAIL_PROMPT`)
if err != nil {
t.Fatalf("parse template: %v", err)
}

h := handlers.ServeDocumentPost(database, emailTmpl)

for i := 1; i <= 4; i++ {
form := strings.NewReader("email=user%40example.com")
req := httptest.NewRequest(http.MethodPost, "/multidown.pdf", form)
req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
rr := httptest.NewRecorder()
h(rr, req)

events, err := database.ListDownloadEvents(docID)
if err != nil {
t.Fatalf("download %d: list events: %v", i, err)
}
if len(events) != i {
t.Errorf("after download %d: expected %d events, got %d", i, i, len(events))
}
}
}

func TestServeDocumentPost_MultipleDownloads_WithRealServer(t *testing.T) {
database, dir := setupDB(t)

pdfPath := filepath.Join(dir, "rsvr.pdf")
if err := os.WriteFile(pdfPath, minimalPDF, 0o640); err != nil {
t.Fatalf("write pdf: %v", err)
}

docID, err := database.InsertDocument("rsvr.pdf", "RealServer.pdf", pdfPath, "", 0, true)
if err != nil {
t.Fatalf("insert: %v", err)
}

emailTmpl, err := template.New("email_prompt.html").Parse(`EMAIL_PROMPT`)
if err != nil {
t.Fatalf("parse template: %v", err)
}

mux := http.NewServeMux()
mux.Handle("GET /{slug}", handlers.ServeDocument(database, emailTmpl))
mux.Handle("POST /{slug}", handlers.ServeDocumentPost(database, emailTmpl))

server := httptest.NewServer(mux)
defer server.Close()

var redirectLog []string
client := &http.Client{
CheckRedirect: func(req *http.Request, via []*http.Request) error {
redirectLog = append(redirectLog,
via[len(via)-1].Method+" -> "+req.Method+" "+req.URL.String())
return nil
},
}

for i := 1; i <= 4; i++ {
redirectLog = nil
resp, err := client.PostForm(server.URL+"/rsvr.pdf",
map[string][]string{"email": {"user@example.com"}})
if err != nil {
t.Fatalf("download %d: %v", i, err)
}
resp.Body.Close()

if len(redirectLog) > 0 {
t.Logf("download %d: redirects: %v", i, redirectLog)
}

events, err := database.ListDownloadEvents(docID)
if err != nil {
t.Fatalf("download %d: list events: %v", i, err)
}
if len(events) != i {
t.Errorf("after download %d: expected %d events, got %d (redirects: %v)", i, i, len(events), redirectLog)
}
}
}
