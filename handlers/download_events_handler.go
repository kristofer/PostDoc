package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

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
