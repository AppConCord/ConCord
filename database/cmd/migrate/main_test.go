package main

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
)

func TestMigrationLifecycleAgainstSQLite(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "concord.db")
	migrationsPath, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCORD_DATABASE_PATH", databasePath)
	t.Setenv("CONCORD_MIGRATIONS_DIR", migrationsPath)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := run(logger, []string{"up"}); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	db, err := sql.Open("sqlite3", databasePath+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, table := range []string{"users", "sessions", "messages", "goose_db_version"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %q was not created", table)
		}
	}
	if _, err := db.Exec(`
		INSERT INTO users (id, username, password_hash, created_at, updated_at)
		VALUES (1, 'Uppercase', 'hash', 1, 1)
	`); err == nil {
		t.Fatal("database accepted an invalid uppercase username")
	}
	if _, err := db.Exec(`
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, updated_at)
		VALUES (2, 999, zeroblob(32), 2, 1, 1)
	`); err == nil {
		t.Fatal("database accepted a session with a missing user")
	}
	if err := run(logger, []string{"status"}); err != nil {
		t.Fatalf("migration status: %v", err)
	}
	if err := run(logger, []string{"version"}); err != nil {
		t.Fatalf("migration version: %v", err)
	}
	if err := run(logger, []string{"down"}); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("down migration did not remove application tables")
	}
}

func TestMigrationCommandValidation(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := run(logger, nil); err == nil {
		t.Fatal("expected usage error")
	}
	if err := run(logger, []string{"explode"}); err == nil {
		t.Fatal("expected unsupported command error")
	}
}
