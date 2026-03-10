package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// Document represents a stored document record.
type Document struct {
	ID       int64
	Slug     string
	Filename string
	Path     string
}

// Admin represents an admin user record.
type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
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
	d := &DB{conn: conn}
	if err := d.seedDefaultAdmin(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("seed admin: %w", err)
	}
	return d, nil
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
		);
		CREATE TABLE IF NOT EXISTS admins (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password_hash TEXT    NOT NULL
		);
	`)
	return err
}

// seedDefaultAdmin creates the default "admin"/"foobar" account if no admins exist.
func (d *DB) seedDefaultAdmin() error {
	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("foobar"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(
		`INSERT INTO admins (username, password_hash) VALUES (?, ?)`,
		"admin", string(hash),
	)
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

// ---- Admin methods ----

// CreateAdmin inserts a new admin with a bcrypt-hashed password.
func (d *DB) CreateAdmin(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(
		`INSERT INTO admins (username, password_hash) VALUES (?, ?)`,
		username, string(hash),
	)
	return err
}

// GetAdminByUsername fetches the admin with the given username, or nil if not found.
func (d *DB) GetAdminByUsername(username string) (*Admin, error) {
	row := d.conn.QueryRow(
		`SELECT id, username, password_hash FROM admins WHERE username = ?`,
		username,
	)
	a := &Admin{}
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ListAdmins returns all admin records ordered by id.
func (d *DB) ListAdmins() ([]Admin, error) {
	rows, err := d.conn.Query(`SELECT id, username, password_hash FROM admins ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var admins []Admin
	for rows.Next() {
		var a Admin
		if err := rows.Scan(&a.ID, &a.Username, &a.PasswordHash); err != nil {
			return nil, err
		}
		admins = append(admins, a)
	}
	return admins, rows.Err()
}

// DeleteAdmin removes the admin with the given id.
func (d *DB) DeleteAdmin(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM admins WHERE id = ?`, id)
	return err
}

// GetAdminByID fetches the admin with the given id, or nil if not found.
func (d *DB) GetAdminByID(id int64) (*Admin, error) {
	row := d.conn.QueryRow(
		`SELECT id, username, password_hash FROM admins WHERE id = ?`, id,
	)
	a := &Admin{}
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ChangePassword replaces the stored password hash for the given username.
func (d *DB) ChangePassword(username, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(
		`UPDATE admins SET password_hash = ? WHERE username = ?`,
		string(hash), username,
	)
	return err
}

// AuthenticateAdmin checks credentials and returns the admin on success, nil on failure.
func (d *DB) AuthenticateAdmin(username, password string) (*Admin, error) {
	admin, err := d.GetAdminByUsername(username)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, nil
	}
	return admin, nil
}
