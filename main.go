package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kristofer/postdoc/db"
	"github.com/kristofer/postdoc/handlers"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "postdoc.db", "SQLite database path")
	uploadDir := flag.String("uploads", "uploads", "Directory for uploaded files")
	baseURL := flag.String("base-url", "", "Base URL for short links (e.g. https://example.com). Defaults to http://<addr>")
	flag.Parse()

	// Ensure upload directory exists.
	if err := os.MkdirAll(*uploadDir, 0o750); err != nil {
		log.Fatalf("cannot create upload dir %s: %v", *uploadDir, err)
	}

	// Open database.
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("cannot open database: %v", err)
	}
	defer database.Close()

	// Resolve base URL.
	base := *baseURL
	if base == "" {
		host := *addr
		if strings.HasPrefix(host, ":") {
			host = "localhost" + host
		}
		base = "http://" + host
	}
	base = strings.TrimRight(base, "/")

	// Load HTML templates.
	tmplGlob := filepath.Join("templates", "*.html")
	tmpl, err := template.ParseGlob(tmplGlob)
	if err != nil {
		log.Fatalf("cannot parse templates: %v", err)
	}

	// Routes.
	mux := http.NewServeMux()

	// Upload form.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// Let the slug handler deal with it.
			handlers.ServeDocument(database)(w, r)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
			log.Printf("template error: %v", err)
		}
	})

	// Upload endpoint.
	mux.Handle("POST /upload", handlers.UploadHandler(database, *uploadDir, tmpl, base))

	// Short-link document serving.
	mux.Handle("GET /{slug}", handlers.ServeDocument(database))

	log.Printf("PostDoc listening on %s (base URL: %s)", *addr, base)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
