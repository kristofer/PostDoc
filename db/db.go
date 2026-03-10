package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// pragmaNameRe matches valid SQLite PRAGMA identifiers (letters, digits, underscores).
var pragmaNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Document represents a stored document record.
type Document struct {
	ID         int64
	Slug       string
	Filename   string
	Path       string
	UploadedBy string
	UploadedAt time.Time
	Size       int64
}

// FmtSize returns the document file size in a human-readable format.
func (d Document) FmtSize() string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case d.Size >= gb:
		return fmt.Sprintf("%.1f GB", float64(d.Size)/gb)
	case d.Size >= mb:
		return fmt.Sprintf("%.1f MB", float64(d.Size)/mb)
	case d.Size >= kb:
		return fmt.Sprintf("%.1f KB", float64(d.Size)/kb)
	default:
		return fmt.Sprintf("%d B", d.Size)
	}
}

// FmtUploadedAt returns the upload timestamp formatted for display.
func (d Document) FmtUploadedAt() string {
	if d.UploadedAt.IsZero() {
		return "—"
	}
	return d.UploadedAt.UTC().Format("2006-01-02 15:04 UTC")
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
	if err := configure(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("configure db: %w", err)
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

// QueryPragma returns the string value of the named SQLite PRAGMA.
// It is intended for use in tests. The pragma name must consist only of
// letters, digits, and underscores; an error is returned otherwise.
func (d *DB) QueryPragma(name string) (string, error) {
	if !pragmaNameRe.MatchString(name) {
		return "", fmt.Errorf("invalid pragma name: %q", name)
	}
	var value string
	err := d.conn.QueryRow("PRAGMA " + name).Scan(&value)
	return value, err
}

// configure applies SQLite PRAGMAs that enable WAL mode and improve
// concurrency and performance. It must be called immediately after opening
// the connection, before any schema migrations run.
func configure(conn *sql.DB) error {
	pragmas := []string{
		// Use Write-Ahead Logging for better concurrent read/write performance.
		`PRAGMA journal_mode=WAL`,
		// NORMAL is safe with WAL and faster than FULL.
		`PRAGMA synchronous=NORMAL`,
		// 64 MiB page cache (negative value = kibibytes).
		`PRAGMA cache_size=-65536`,
		// Enforce foreign-key constraints.
		`PRAGMA foreign_keys=ON`,
		// Wait up to 5 seconds when the database is locked before returning SQLITE_BUSY.
		`PRAGMA busy_timeout=5000`,
		// Keep temporary tables and indices in memory.
		`PRAGMA temp_store=MEMORY`,
		// Allow WAL to be shared across multiple reader connections.
		`PRAGMA wal_autocheckpoint=1000`,
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

func migrate(conn *sql.DB) error {
	_, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS documents (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			slug        TEXT    NOT NULL UNIQUE,
			filename    TEXT    NOT NULL,
			path        TEXT    NOT NULL,
			uploaded_by TEXT    NOT NULL DEFAULT '',
			uploaded_at TEXT    NOT NULL DEFAULT '',
			size        INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS admins (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password_hash TEXT    NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	// Add new columns to databases created before these fields existed.
	for _, stmt := range []string{
		`ALTER TABLE documents ADD COLUMN uploaded_by TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN uploaded_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN size INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := conn.Exec(stmt); err != nil && !isDuplicateColumn(err) {
			return err
		}
	}
	return nil
}

// isDuplicateColumn reports whether the SQLite error is "duplicate column name",
// which occurs when ALTER TABLE ADD COLUMN is run on a column that already exists.
func isDuplicateColumn(err error) bool {
	return strings.Contains(err.Error(), "duplicate column name")
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
func (d *DB) InsertDocument(slug, filename, path, uploadedBy string, size int64) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.conn.Exec(
		`INSERT INTO documents (slug, filename, path, uploaded_by, uploaded_at, size) VALUES (?, ?, ?, ?, ?, ?)`,
		slug, filename, path, uploadedBy, now, size,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetBySlug fetches the document with the given slug, or returns nil if not found.
func (d *DB) GetBySlug(slug string) (*Document, error) {
	row := d.conn.QueryRow(
		`SELECT id, slug, filename, path, uploaded_by, uploaded_at, size FROM documents WHERE slug = ?`,
		slug,
	)
	doc := &Document{}
	var uploadedAt string
	err := row.Scan(&doc.ID, &doc.Slug, &doc.Filename, &doc.Path, &doc.UploadedBy, &uploadedAt, &doc.Size)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if uploadedAt != "" {
		doc.UploadedAt, _ = time.Parse(time.RFC3339, uploadedAt)
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

// CountDocuments returns the total number of documents in the database.
func (d *DB) CountDocuments() (int, error) {
	var n int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&n)
	return n, err
}

// ListDocuments returns a page of documents ordered by most recently uploaded.
func (d *DB) ListDocuments(page, pageSize int) ([]Document, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	rows, err := d.conn.Query(
		`SELECT id, slug, filename, path, uploaded_by, uploaded_at, size FROM documents ORDER BY id DESC LIMIT ? OFFSET ?`,
		pageSize, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var doc Document
		var uploadedAt string
		if err := rows.Scan(&doc.ID, &doc.Slug, &doc.Filename, &doc.Path, &doc.UploadedBy, &uploadedAt, &doc.Size); err != nil {
			return nil, err
		}
		if uploadedAt != "" {
			doc.UploadedAt, _ = time.Parse(time.RFC3339, uploadedAt)
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// DeleteDocuments removes the documents with the given ids and returns the
// on-disk file paths that were associated with them so the caller can clean
// up the files.
func (d *DB) DeleteDocuments(ids []int64) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	// Collect file paths before deleting.
	rows, err := d.conn.Query(
		`SELECT path FROM documents WHERE id IN (`+inClause+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	_, err = d.conn.Exec(
		`DELETE FROM documents WHERE id IN (`+inClause+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return paths, nil
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
