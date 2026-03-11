package handlers_test

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kristofer/postdoc/db"
	"github.com/kristofer/postdoc/handlers"
)

func openCSVTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "csv_test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestDocumentsCSVHandler_Empty(t *testing.T) {
	database := openCSVTestDB(t)

	h := handlers.DocumentsCSVHandler(database, "http://localhost:8080")
	req := httptest.NewRequest(http.MethodGet, "/documents/csv", nil)
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("expected text/csv content type, got %q", ct)
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "documents.csv") {
		t.Errorf("expected documents.csv in Content-Disposition, got %q", cd)
	}

	records, err := csv.NewReader(rr.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	// Only the header row should be present.
	if len(records) != 1 {
		t.Errorf("expected 1 row (header only), got %d", len(records))
	}
	if records[0][0] != "id" {
		t.Errorf("expected header row starting with 'id', got %q", records[0][0])
	}
}

func TestDocumentsCSVHandler_WithDocs(t *testing.T) {
	database := openCSVTestDB(t)

	if _, err := database.InsertDocument("report.pdf", "Report.pdf", "/tmp/report.pdf", "alice", 2048, false); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := database.InsertDocument("summary.pdf", "Summary.pdf", "/tmp/summary.pdf", "bob", 1024, false); err != nil {
		t.Fatalf("insert: %v", err)
	}

	h := handlers.DocumentsCSVHandler(database, "http://localhost:8080")
	req := httptest.NewRequest(http.MethodGet, "/documents/csv", nil)
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	records, err := csv.NewReader(rr.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	// Header + 2 data rows.
	if len(records) != 3 {
		t.Errorf("expected 3 rows, got %d", len(records))
	}
	// Check header columns.
	header := records[0]
	expectedCols := []string{"id", "filename", "uploaded_by", "uploaded_at", "size_bytes", "url", "download_count"}
	for i, col := range expectedCols {
		if i >= len(header) || header[i] != col {
			t.Errorf("header[%d]: expected %q, got %q", i, col, header[i])
		}
	}
	// Data rows should contain the document filenames.
	filenames := map[string]bool{}
	for _, row := range records[1:] {
		if len(row) > 1 {
			filenames[row[1]] = true
		}
	}
	if !filenames["Report.pdf"] {
		t.Errorf("expected Report.pdf in CSV data")
	}
	if !filenames["Summary.pdf"] {
		t.Errorf("expected Summary.pdf in CSV data")
	}
}

func TestDownloadEventsCSVHandler_NotFound(t *testing.T) {
	database := openCSVTestDB(t)

	h := handlers.DownloadEventsCSVHandler(database)
	req := httptest.NewRequest(http.MethodGet, "/documents/999/downloads/csv", nil)
	req.SetPathValue("id", "999")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDownloadEventsCSVHandler_InvalidID(t *testing.T) {
	database := openCSVTestDB(t)

	h := handlers.DownloadEventsCSVHandler(database)
	req := httptest.NewRequest(http.MethodGet, "/documents/abc/downloads/csv", nil)
	req.SetPathValue("id", "abc")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDownloadEventsCSVHandler_NoEvents(t *testing.T) {
	database := openCSVTestDB(t)

	if _, err := database.InsertDocument("doc.pdf", "Doc.pdf", "/tmp/doc.pdf", "admin", 512, true); err != nil {
		t.Fatalf("insert: %v", err)
	}

	h := handlers.DownloadEventsCSVHandler(database)
	req := httptest.NewRequest(http.MethodGet, "/documents/1/downloads/csv", nil)
	req.SetPathValue("id", "1")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("expected text/csv content type, got %q", ct)
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "downloads.csv") {
		t.Errorf("expected downloads.csv in Content-Disposition, got %q", cd)
	}

	records, err := csv.NewReader(rr.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	// Only header row.
	if len(records) != 1 {
		t.Errorf("expected 1 row (header only), got %d", len(records))
	}
}

func TestDownloadEventsCSVHandler_WithEvents(t *testing.T) {
	database := openCSVTestDB(t)

	docID, err := database.InsertDocument("tracked.pdf", "Tracked.pdf", "/tmp/tracked.pdf", "admin", 1024, true)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}
	if err := database.InsertDownloadEvent(docID, "user1@example.com"); err != nil {
		t.Fatalf("insert event 1: %v", err)
	}
	if err := database.InsertDownloadEvent(docID, "user2@example.com"); err != nil {
		t.Fatalf("insert event 2: %v", err)
	}

	h := handlers.DownloadEventsCSVHandler(database)
	req := httptest.NewRequest(http.MethodGet, "/documents/1/downloads/csv", nil)
	req.SetPathValue("id", "1")
	req.AddCookie(sessionCookie(t, "admin"))
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	records, err := csv.NewReader(rr.Body).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	// Header + 2 data rows.
	if len(records) != 3 {
		t.Errorf("expected 3 rows, got %d", len(records))
	}
	// Check header columns.
	header := records[0]
	expectedCols := []string{"#", "email", "downloaded_at"}
	for i, col := range expectedCols {
		if i >= len(header) || header[i] != col {
			t.Errorf("header[%d]: expected %q, got %q", i, col, header[i])
		}
	}
	// Collect emails from data rows.
	emails := map[string]bool{}
	for _, row := range records[1:] {
		if len(row) > 1 {
			emails[row[1]] = true
		}
	}
	if !emails["user1@example.com"] {
		t.Errorf("expected user1@example.com in CSV data")
	}
	if !emails["user2@example.com"] {
		t.Errorf("expected user2@example.com in CSV data")
	}
}
