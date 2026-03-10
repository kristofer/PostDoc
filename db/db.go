package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Document represents a stored document record.
type Document struct {
	ID       int64
	Slug     string
	Filename string
	Path     string
}

// DB wraps an sql.DB connection.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at the given path and
// initialises the schema.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

func migrate(conn *sql.DB) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS documents (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			slug     TEXT    NOT NULL UNIQUE,
			filename TEXT    NOT NULL,
			path     TEXT    NOT NULL
		)
	`)
	return err
}

// InsertDocument inserts a new document record and returns its assigned id.
func (d *DB) InsertDocument(slug, filename, path string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO documents (slug, filename, path) VALUES (?, ?, ?)`,
		slug, filename, path,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetBySlug fetches the document with the given slug, or returns nil if not found.
func (d *DB) GetBySlug(slug string) (*Document, error) {
	row := d.conn.QueryRow(
		`SELECT id, slug, filename, path FROM documents WHERE slug = ?`,
		slug,
	)
	doc := &Document{}
	err := row.Scan(&doc.ID, &doc.Slug, &doc.Filename, &doc.Path)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// SlugExists reports whether a document with the given slug already exists.
func (d *DB) SlugExists(slug string) (bool, error) {
	var n int
	err := d.conn.QueryRow(
		`SELECT COUNT(*) FROM documents WHERE slug = ?`, slug,
	).Scan(&n)
	return n > 0, err
}
