package database

import (
	"path/filepath"
	"testing"

	"concord/backend/internal/model"
)

func TestOpenAndVerifySchema(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close(db) })
	if err := VerifySchema(db); err == nil {
		t.Fatal("expected missing schema error")
	}
	if err := db.AutoMigrate(&model.User{}, &model.Session{}, &model.Message{}); err != nil {
		t.Fatal(err)
	}
	if err := VerifySchema(db); err != nil {
		t.Fatalf("VerifySchema() after test migration: %v", err)
	}
}
