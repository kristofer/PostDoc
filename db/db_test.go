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

	id, err := database.InsertDocument("my-doc", "My Doc.pdf", "/uploads/my-doc.pdf")
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

	if _, err := database.InsertDocument("x", "x.pdf", "/tmp/x.pdf"); err != nil {
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

	if _, err := database.InsertDocument("dup", "a.pdf", "/a.pdf"); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// Inserting the same slug should fail.
	if _, err := database.InsertDocument("dup", "b.pdf", "/b.pdf"); err == nil {
		t.Error("expected error on duplicate slug, got nil")
	}
}
