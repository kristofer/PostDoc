package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/kristofer/postdoc/auth"
	"github.com/kristofer/postdoc/db"
)

const docsPageSize = 30

// DocumentsHandler serves the document tracking page (GET /documents).
func DocumentsHandler(database *db.DB, tmpl *template.Template, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)

		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				page = n
			}
		}

		total, err := database.CountDocuments()
		if err != nil {
			log.Printf("count documents error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		totalPages := (total + docsPageSize - 1) / docsPageSize
		if totalPages < 1 {
			totalPages = 1
		}
		if page > totalPages {
			page = totalPages
		}

		docs, err := database.ListDocuments(page, docsPageSize)
		if err != nil {
			log.Printf("list documents error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Documents":   docs,
			"Username":    username,
			"BaseURL":     baseURL,
			"CurrentPage": page,
			"TotalPages":  totalPages,
			"TotalDocs":   total,
			"HasPrev":     page > 1,
			"HasNext":     page < totalPages,
			"PrevPage":    page - 1,
			"NextPage":    page + 1,
		}
		if err := tmpl.ExecuteTemplate(w, "documents.html", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

// DeleteDocumentsHandler handles POST /documents/delete — deletes the selected
// documents from the database and removes their files from disk.
func DeleteDocumentsHandler(database *db.DB, tmpl *template.Template, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var ids []int64
		for _, s := range r.Form["ids"] {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil || id <= 0 {
				continue
			}
			ids = append(ids, id)
		}

		username, _ := auth.UsernameFromRequest(r)

		var flashErr string
		if len(ids) > 0 {
			paths, err := database.DeleteDocuments(ids)
			if err != nil {
				log.Printf("delete documents error: %v", err)
				flashErr = "Could not delete the selected documents."
			} else {
				for _, p := range paths {
					if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
						log.Printf("remove file %s: %v", p, err)
					}
				}
			}
		}

		if flashErr == "" {
			http.Redirect(w, r, "/documents", http.StatusSeeOther)
			return
		}

		// Re-render the page with the error message.
		total, _ := database.CountDocuments()
		totalPages := (total + docsPageSize - 1) / docsPageSize
		if totalPages < 1 {
			totalPages = 1
		}
		docs, _ := database.ListDocuments(1, docsPageSize)

		data := map[string]interface{}{
			"Documents":   docs,
			"Username":    username,
			"BaseURL":     baseURL,
			"CurrentPage": 1,
			"TotalPages":  totalPages,
			"TotalDocs":   total,
			"HasPrev":     false,
			"HasNext":     totalPages > 1,
			"PrevPage":    0,
			"NextPage":    2,
			"Error":       flashErr,
		}
		if err := tmpl.ExecuteTemplate(w, "documents.html", data); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}
