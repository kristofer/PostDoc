package handlers

import (
	"encoding/csv"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/kristofer/postdoc/auth"
	"github.com/kristofer/postdoc/db"
)

// DownloadEventsHandler serves the download history page for a single document
// (GET /documents/{id}/downloads). Only accessible to authenticated admins.
func DownloadEventsHandler(database *db.DB, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}

		doc, err := database.GetDocumentByID(id)
		if err != nil {
			log.Printf("get document error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if doc == nil {
			http.NotFound(w, r)
			return
		}

		events, err := database.ListDownloadEvents(id)
		if err != nil {
			log.Printf("list download events error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Document": doc,
			"Events":   events,
			"Username": username,
		}
		if err := tmpl.ExecuteTemplate(w, "download_events.html", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

// DownloadEventsCSVHandler serves a CSV download of all download events for a document
// (GET /documents/{id}/downloads/csv). Only accessible to authenticated admins.
func DownloadEventsCSVHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}

		doc, err := database.GetDocumentByID(id)
		if err != nil {
			log.Printf("get document error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if doc == nil {
			http.NotFound(w, r)
			return
		}

		events, err := database.ListDownloadEvents(id)
		if err != nil {
			log.Printf("list download events error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		safeName := strings.NewReplacer(`"`, `_`, `\`, `_`).Replace(doc.Filename)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+safeName+`-downloads.csv"`)

		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"#", "email", "downloaded_at"})
		for i, e := range events {
			_ = cw.Write([]string{
				strconv.Itoa(i + 1),
				e.Email,
				e.FmtDownloadedAt(),
			})
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("csv write error: %v", err)
		}
	}
}
