package handlers_test

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kristofer/postdoc/db"
	"github.com/kristofer/postdoc/handlers"
)

func makeDocsTemplate(t *testing.T) *template.Template {
	t.Helper()
	const tmplSrc = `
{{define "documents.html"}}DOCS COUNT:{{.TotalDocs}} PAGE:{{.CurrentPage}} PAGES:{{.TotalPages}}{{if .Error}} ERROR:{{.Error}}{{end}}{{range .Documents}} DOC:{{.Filename}}{{end}}{{end}}
`
	tmpl, err := template.New("root").Parse(tmplSrc)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return tmpl
}

func openDocsTestDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "docs_test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database, dir
}

func TestDocumentsHandler_Empty(t *testing.T) {
	database, _ := openDocsTestDB(t)
	tmpl := makeDocsTemplate(t)

	h := handlers.DocumentsHandler(database, tmpl, "http://localhost:8080")
	req := httptest.NewRequest(http.MethodGet, "/documents", nil)
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "COUNT:0") {
		t.Errorf("expected COUNT:0 in body, got: %s", body)
	}
}

func TestDocumentsHandler_WithDocs(t *testing.T) {
	database, _ := openDocsTestDB(t)
	tmpl := makeDocsTemplate(t)

	if _, err := database.InsertDocument("report.pdf", "Report.pdf", "/tmp/report.pdf", "alice", 1024, false); err != nil {
		t.Fatalf("insert: %v", err)
	}

	h := handlers.DocumentsHandler(database, tmpl, "http://localhost:8080")
	req := httptest.NewRequest(http.MethodGet, "/documents", nil)
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "COUNT:1") {
		t.Errorf("expected COUNT:1, got: %s", body)
	}
	if !strings.Contains(body, "DOC:Report.pdf") {
		t.Errorf("expected DOC:Report.pdf in body, got: %s", body)
	}
}

func TestDocumentsHandler_Pagination(t *testing.T) {
	database, _ := openDocsTestDB(t)
	tmpl := makeDocsTemplate(t)

	// Insert 35 documents (> default page size of 30).
	for i := 0; i < 35; i++ {
		slug := "slug-" + string(rune('a'+(i%26))) + string(rune('a'+(i/26)))
		if _, err := database.InsertDocument(slug, slug+".pdf", "/tmp/"+slug, "admin", 0, false); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	h := handlers.DocumentsHandler(database, tmpl, "http://localhost:8080")
	req := httptest.NewRequest(http.MethodGet, "/documents?page=2", nil)
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "COUNT:35") {
		t.Errorf("expected COUNT:35, got: %s", body)
	}
	if !strings.Contains(body, "PAGE:2") {
		t.Errorf("expected PAGE:2, got: %s", body)
	}
	if !strings.Contains(body, "PAGES:2") {
		t.Errorf("expected PAGES:2, got: %s", body)
	}
}

func TestDeleteDocumentsHandler_Success(t *testing.T) {
	database, dir := openDocsTestDB(t)
	tmpl := makeDocsTemplate(t)

	// Write a real file so we can confirm it gets removed.
	pdfPath := filepath.Join(dir, "todelete.pdf")
	if err := os.WriteFile(pdfPath, []byte("fake"), 0o640); err != nil {
		t.Fatalf("write file: %v", err)
	}
	id, err := database.InsertDocument("todelete.pdf", "ToDelete.pdf", pdfPath, "admin", 4, false)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	form := url.Values{}
	form.Set("ids", fmt.Sprintf("%d", id))
	req := httptest.NewRequest(http.MethodPost, "/documents/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()

	h := handlers.DeleteDocumentsHandler(database, tmpl, "http://localhost:8080")
	h(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/documents" {
		t.Errorf("expected redirect to /documents, got %q", loc)
	}

	n, _ := database.CountDocuments()
	if n != 0 {
		t.Errorf("expected 0 docs after delete, got %d", n)
	}
}

func TestDeleteDocumentsHandler_NoIDs(t *testing.T) {
	database, _ := openDocsTestDB(t)
	tmpl := makeDocsTemplate(t)

	// Posting with no ids should still redirect successfully.
	req := httptest.NewRequest(http.MethodPost, "/documents/delete", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()

	h := handlers.DeleteDocumentsHandler(database, tmpl, "http://localhost:8080")
	h(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d: %s", rr.Code, rr.Body.String())
	}
}
