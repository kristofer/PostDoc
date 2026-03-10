package main

import (
	"crypto/rand"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kristofer/postdoc/auth"
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

	// Initialise JWT auth with a random secret per process.
	// Enable Secure cookie flag when the base URL uses HTTPS.
	jwtSecret := make([]byte, 32)
	if _, err := rand.Read(jwtSecret); err != nil {
		log.Fatalf("cannot generate JWT secret: %v", err)
	}
	auth.Init(jwtSecret, strings.HasPrefix(base, "https://"))

	// Load HTML templates.
	tmplGlob := filepath.Join("templates", "*.html")
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}).ParseGlob(tmplGlob)
	if err != nil {
		log.Fatalf("cannot parse templates: %v", err)
	}

	// Routes.
	mux := http.NewServeMux()

	// Public: login / logout.
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		handlers.LoginHandler(database, tmpl)(w, r)
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		handlers.LoginHandler(database, tmpl)(w, r)
	})
	mux.Handle("GET /logout", handlers.LogoutHandler())

	// Protected: upload form (root).
	mux.Handle("GET /", auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
			log.Printf("template error: %v", err)
		}
	})))

	// Protected: upload endpoint.
	mux.Handle("POST /upload", auth.Middleware(handlers.UploadHandler(database, *uploadDir, tmpl, base)))

	// Protected: admin management.
	mux.Handle("GET /admin", auth.Middleware(handlers.AdminHandler(database, tmpl)))
	mux.Handle("POST /admin/add", auth.Middleware(handlers.AddAdminHandler(database, tmpl)))
	mux.Handle("POST /admin/delete", auth.Middleware(handlers.DeleteAdminHandler(database, tmpl)))
	mux.Handle("POST /admin/change-password", auth.Middleware(handlers.ChangePasswordHandler(database, tmpl)))

	// Protected: document tracking.
	mux.Handle("GET /documents", auth.Middleware(handlers.DocumentsHandler(database, tmpl, base)))
	mux.Handle("POST /documents/delete", auth.Middleware(handlers.DeleteDocumentsHandler(database, tmpl, base)))
	mux.Handle("GET /documents/{id}/downloads", auth.Middleware(handlers.DownloadEventsHandler(database, tmpl)))

	// Public: short-link document serving.
	mux.Handle("GET /{slug}", handlers.ServeDocument(database, tmpl))
	mux.Handle("POST /{slug}", handlers.ServeDocumentPost(database, tmpl))

	log.Printf("PostDoc listening on %s (base URL: %s)", *addr, base)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
