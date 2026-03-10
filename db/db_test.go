package db_test

import (
	"path/filepath"
	"testing"

	"github.com/kristofer/postdoc/db"
)
func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestInsertAndGetBySlug(t *testing.T) {
	database := openTestDB(t)

	id, err := database.InsertDocument("my-doc", "My Doc.pdf", "/uploads/my-doc.pdf", "admin", 0, false)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	doc, err := database.GetBySlug("my-doc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if doc == nil {
		t.Fatal("expected doc, got nil")
	}
	if doc.Slug != "my-doc" || doc.Filename != "My Doc.pdf" {
		t.Errorf("unexpected doc: %+v", doc)
	}
}

func TestGetBySlug_NotFound(t *testing.T) {
	database := openTestDB(t)
	doc, err := database.GetBySlug("nonexistent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if doc != nil {
		t.Errorf("expected nil, got %+v", doc)
	}
}

func TestSlugExists(t *testing.T) {
	database := openTestDB(t)
	exists, err := database.SlugExists("x")
	if err != nil {
		t.Fatalf("slug exists: %v", err)
	}
	if exists {
		t.Error("expected false before insert")
	}

	if _, err := database.InsertDocument("x", "x.pdf", "/tmp/x.pdf", "", 0, false); err != nil {
		t.Fatalf("insert: %v", err)
	}

	exists, err = database.SlugExists("x")
	if err != nil {
		t.Fatalf("slug exists after insert: %v", err)
	}
	if !exists {
		t.Error("expected true after insert")
	}
}

func TestUniqueSlugConstraint(t *testing.T) {
	database := openTestDB(t)

	if _, err := database.InsertDocument("dup", "a.pdf", "/a.pdf", "", 0, false); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Inserting the same slug should fail.
	if _, err := database.InsertDocument("dup", "b.pdf", "/b.pdf", "", 0, false); err == nil {
		t.Error("expected error on duplicate slug, got nil")
	}
}

func TestDocumentFields(t *testing.T) {
	database := openTestDB(t)

	id, err := database.InsertDocument("doc-with-meta", "Meta.pdf", "/uploads/meta.pdf", "alice", 12345, false)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	doc, err := database.GetBySlug("doc-with-meta")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if doc == nil {
		t.Fatal("expected doc, got nil")
	}
	if doc.UploadedBy != "alice" {
		t.Errorf("UploadedBy: got %q, want %q", doc.UploadedBy, "alice")
	}
	if doc.Size != 12345 {
		t.Errorf("Size: got %d, want 12345", doc.Size)
	}
	if doc.UploadedAt.IsZero() {
		t.Error("expected non-zero UploadedAt")
	}
}

func TestCountDocuments(t *testing.T) {
	database := openTestDB(t)

	n, err := database.CountDocuments()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}

	for i := 0; i < 3; i++ {
		slug := "doc-count-" + string(rune('a'+i))
		if _, err := database.InsertDocument(slug, slug+".pdf", "/"+slug, "", 0, false); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	n, err = database.CountDocuments()
	if err != nil {
		t.Fatalf("count after inserts: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestListDocuments_Pagination(t *testing.T) {
	database := openTestDB(t)

	for i := 0; i < 5; i++ {
		slug := "page-doc-" + string(rune('a'+i))
		if _, err := database.InsertDocument(slug, slug+".pdf", "/"+slug, "admin", int64(i*100), false); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// Page 1 with size 3 should return 3 docs.
	docs, err := database.ListDocuments(1, 3)
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(docs) != 3 {
		t.Errorf("page 1: expected 3 docs, got %d", len(docs))
	}

	// Page 2 with size 3 should return 2 docs.
	docs, err = database.ListDocuments(2, 3)
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("page 2: expected 2 docs, got %d", len(docs))
	}
}

func TestDeleteDocuments(t *testing.T) {
	database := openTestDB(t)

	id1, err := database.InsertDocument("del-a", "a.pdf", "/tmp/del-a.pdf", "", 0, false)
	if err != nil {
		t.Fatalf("insert a: %v", err)
	}
	id2, err := database.InsertDocument("del-b", "b.pdf", "/tmp/del-b.pdf", "", 0, false)
	if err != nil {
		t.Fatalf("insert b: %v", err)
	}

	paths, err := database.DeleteDocuments([]int64{id1, id2})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}

	n, err := database.CountDocuments()
	if err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 docs after deletion, got %d", n)
	}
}

func TestDeleteDocuments_Empty(t *testing.T) {
	database := openTestDB(t)
	paths, err := database.DeleteDocuments(nil)
	if err != nil {
		t.Fatalf("delete empty: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths, got %v", paths)
	}
}

func TestDocument_FmtSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536, "1.5 KB"},
	}
	for _, tc := range tests {
		doc := db.Document{Size: tc.size}
		got := doc.FmtSize()
		if got != tc.want {
			t.Errorf("FmtSize(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}

func TestInsertDocument_TrackDownloads(t *testing.T) {
	database := openTestDB(t)

	// Insert a document with tracking enabled.
	id, err := database.InsertDocument("track-doc", "Track.pdf", "/tmp/track.pdf", "admin", 100, true)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	doc, err := database.GetBySlug("track-doc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if doc == nil {
		t.Fatal("expected doc, got nil")
	}
	if !doc.TrackDownloads {
		t.Error("expected TrackDownloads to be true")
	}
}

func TestInsertDownloadEvent_AndList(t *testing.T) {
	database := openTestDB(t)

	id, err := database.InsertDocument("dl-doc", "Download.pdf", "/tmp/dl.pdf", "admin", 50, true)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	if err := database.InsertDownloadEvent(id, "alice@example.com"); err != nil {
		t.Fatalf("insert event 1: %v", err)
	}
	if err := database.InsertDownloadEvent(id, "bob@example.com"); err != nil {
		t.Fatalf("insert event 2: %v", err)
	}

	events, err := database.ListDownloadEvents(id)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	// Events are ordered by most recent first.
	if events[0].Email != "bob@example.com" {
		t.Errorf("expected bob first (most recent), got %q", events[0].Email)
	}
	if events[0].DocumentID != id {
		t.Errorf("expected document_id %d, got %d", id, events[0].DocumentID)
	}
	if events[0].DownloadedAt.IsZero() {
		t.Error("expected non-zero DownloadedAt")
	}
}

func TestListDocuments_DownloadCount(t *testing.T) {
	database := openTestDB(t)

	id, err := database.InsertDocument("count-doc", "Count.pdf", "/tmp/count.pdf", "admin", 0, true)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	// No events yet.
	docs, err := database.ListDocuments(1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].DownloadCount != 0 {
		t.Errorf("expected DownloadCount 0, got %d", docs[0].DownloadCount)
	}

	// Add two events.
	if err := database.InsertDownloadEvent(id, "x@x.com"); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if err := database.InsertDownloadEvent(id, "y@y.com"); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	docs, err = database.ListDocuments(1, 10)
	if err != nil {
		t.Fatalf("list after events: %v", err)
	}
	if docs[0].DownloadCount != 2 {
		t.Errorf("expected DownloadCount 2, got %d", docs[0].DownloadCount)
	}
}

func TestGetDocumentByID(t *testing.T) {
	database := openTestDB(t)

	id, err := database.InsertDocument("byid-doc", "ByID.pdf", "/tmp/byid.pdf", "admin", 0, false)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	doc, err := database.GetDocumentByID(id)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if doc == nil {
		t.Fatal("expected doc, got nil")
	}
	if doc.Slug != "byid-doc" {
		t.Errorf("expected slug 'byid-doc', got %q", doc.Slug)
	}
}

func TestGetDocumentByID_NotFound(t *testing.T) {
	database := openTestDB(t)
	doc, err := database.GetDocumentByID(99999)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if doc != nil {
		t.Error("expected nil for non-existent id")
	}
}

func TestDownloadEvents_CascadeDelete(t *testing.T) {
	database := openTestDB(t)

	id, err := database.InsertDocument("cascade-doc", "Cascade.pdf", "/tmp/c.pdf", "admin", 0, true)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}
	if err := database.InsertDownloadEvent(id, "a@b.com"); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	// Deleting the document should cascade-delete its events.
	if _, err := database.DeleteDocuments([]int64{id}); err != nil {
		t.Fatalf("delete doc: %v", err)
	}

	events, err := database.ListDownloadEvents(id)
	if err != nil {
		t.Fatalf("list events after delete: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events after cascade delete, got %d", len(events))
	}
}


func TestDefaultAdminSeeded(t *testing.T) {
	database := openTestDB(t)
	admins, err := database.ListAdmins()
	if err != nil {
		t.Fatalf("list admins: %v", err)
	}
	if len(admins) != 1 {
		t.Fatalf("expected 1 default admin, got %d", len(admins))
	}
	if admins[0].Username != "admin" {
		t.Errorf("expected default username 'admin', got %q", admins[0].Username)
	}
}

func TestAuthenticateAdmin_Success(t *testing.T) {
	database := openTestDB(t)
	admin, err := database.AuthenticateAdmin("admin", "foobar")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if admin == nil {
		t.Fatal("expected admin, got nil")
	}
	if admin.Username != "admin" {
		t.Errorf("expected 'admin', got %q", admin.Username)
	}
}

func TestAuthenticateAdmin_WrongPassword(t *testing.T) {
	database := openTestDB(t)
	admin, err := database.AuthenticateAdmin("admin", "wrongpassword")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if admin != nil {
		t.Error("expected nil for wrong password")
	}
}

func TestAuthenticateAdmin_UnknownUser(t *testing.T) {
	database := openTestDB(t)
	admin, err := database.AuthenticateAdmin("nobody", "foobar")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if admin != nil {
		t.Error("expected nil for unknown user")
	}
}

func TestCreateAndDeleteAdmin(t *testing.T) {
	database := openTestDB(t)

	if err := database.CreateAdmin("alice", "secret"); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	alice, err := database.GetAdminByUsername("alice")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if alice == nil {
		t.Fatal("expected alice, got nil")
	}

	if err := database.DeleteAdmin(alice.ID); err != nil {
		t.Fatalf("delete admin: %v", err)
	}

	aliceAfter, err := database.GetAdminByUsername("alice")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if aliceAfter != nil {
		t.Error("expected nil after deletion")
	}
}

func TestCreateAdmin_DuplicateUsername(t *testing.T) {
	database := openTestDB(t)
	if err := database.CreateAdmin("bob", "pass1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := database.CreateAdmin("bob", "pass2"); err == nil {
		t.Error("expected error on duplicate username")
	}
}

func TestGetAdminByID(t *testing.T) {
	database := openTestDB(t)
	admins, _ := database.ListAdmins()
	if len(admins) == 0 {
		t.Fatal("expected at least one admin")
	}
	a, err := database.GetAdminByID(admins[0].ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if a == nil || a.Username != admins[0].Username {
		t.Errorf("unexpected admin: %+v", a)
	}
}

func TestGetAdminByID_NotFound(t *testing.T) {
	database := openTestDB(t)
	a, err := database.GetAdminByID(99999)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if a != nil {
		t.Error("expected nil for non-existent id")
	}
}

func TestChangePassword(t *testing.T) {
	database := openTestDB(t)

	// Change the default admin's password.
	if err := database.ChangePassword("admin", "newpass"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	// Old password should no longer work.
	admin, err := database.AuthenticateAdmin("admin", "foobar")
	if err != nil {
		t.Fatalf("authenticate with old password: %v", err)
	}
	if admin != nil {
		t.Error("expected nil for old password after change")
	}

	// New password should work.
	admin, err = database.AuthenticateAdmin("admin", "newpass")
	if err != nil {
		t.Fatalf("authenticate with new password: %v", err)
	}
	if admin == nil {
		t.Fatal("expected admin with new password")
	}
}

func TestSQLitePragmas(t *testing.T) {
	database := openTestDB(t)

	tests := []struct {
		pragma string
		want   string
	}{
		{"journal_mode", "wal"},
		{"foreign_keys", "1"},
	}
	for _, tc := range tests {
		val, err := database.QueryPragma(tc.pragma)
		if err != nil {
			t.Fatalf("QueryPragma(%q): %v", tc.pragma, err)
		}
		if val != tc.want {
			t.Errorf("PRAGMA %s = %q, want %q", tc.pragma, val, tc.want)
		}
	}
}

func TestQueryPragma_InvalidName(t *testing.T) {
	database := openTestDB(t)
	_, err := database.QueryPragma("bad; DROP TABLE admins--")
	if err == nil {
		t.Error("expected error for invalid pragma name, got nil")
	}
}
