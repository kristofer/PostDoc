package handlers

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kristofer/postdoc/auth"
	"github.com/kristofer/postdoc/db"
)

// nonAlphaNum matches any character that is not alphanumeric or a hyphen.
var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a filename (without extension) into a URL-safe slug.
func slugify(name string) string {
	s := strings.ToLower(name)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "doc"
	}
	return s
}

// uniqueSlug returns a slug derived from base that does not yet exist in the DB.
// If base contains a file extension (e.g. ".pdf"), collision counters are
// inserted before the extension: "report.pdf" → "report-1.pdf", etc.
func uniqueSlug(database *db.DB, base string) (string, error) {
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	candidate := base
	for i := 1; ; i++ {
		exists, err := database.SlugExists(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d%s", nameWithoutExt, i, ext)
	}
}

// UploadHandler handles PDF uploads.
func UploadHandler(database *db.DB, uploadDir string, tmpl *template.Template, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit to 32 MB.
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("document")
		if err != nil {
			http.Error(w, "missing document field: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Validate MIME type by reading the first 512 bytes.
		buf := make([]byte, 512)
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		ct := http.DetectContentType(buf[:n])
		if ct != "application/pdf" {
			http.Error(w, "only PDF files are accepted", http.StatusBadRequest)
			return
		}

		// Build slug from the original filename, preserving the .pdf extension.
		baseName := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
		slug, err := uniqueSlug(database, slugify(baseName)+".pdf")
		if err != nil {
			log.Printf("slug generation error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Store the file on disk using the slug directly (which already includes .pdf).
		destPath := filepath.Join(uploadDir, slug)
		dest, err := os.Create(destPath)
		if err != nil {
			log.Printf("create file error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		defer dest.Close()

		// Write the already-read bytes first, then the rest.
		if _, err := dest.Write(buf[:n]); err != nil {
			log.Printf("write header error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(dest, file); err != nil {
			log.Printf("copy file error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Persist metadata in the DB.
		uploadedBy, _ := auth.UsernameFromRequest(r)
		var fileSize int64
		if info, err := os.Stat(destPath); err == nil {
			fileSize = info.Size()
		}
		if _, err := database.InsertDocument(slug, header.Filename, destPath, uploadedBy, fileSize); err != nil {
			log.Printf("db insert error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		shortURL := baseURL + "/" + slug
		if err := tmpl.ExecuteTemplate(w, "success.html", map[string]string{
			"Filename": header.Filename,
			"ShortURL": shortURL,
			"Slug":     slug,
		}); err != nil {
			log.Printf("template error: %v", err)
		}
	}
}

// ServeDocument serves the PDF file for a given slug.
func ServeDocument(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimPrefix(r.URL.Path, "/")
		if slug == "" {
			http.NotFound(w, r)
			return
		}

		doc, err := database.GetBySlug(slug)
		if err != nil {
			log.Printf("db lookup error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if doc == nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`inline; filename="%s"`, doc.Filename))
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, doc.Path)
	}
}
