// Package database configures the GORM SQLite connection used by the backend.
package database

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open creates a SQLite-backed GORM connection with foreign keys, WAL, and a busy timeout enabled.
// Schema migration is deliberately excluded and belongs to the independent database service.
func Open(path string) (*gorm.DB, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	dsn := "file:" + filepath.ToSlash(absolutePath) + "?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		PrepareStmt:    true,
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying database connection: %w", err)
	}
	// A single connection avoids SQLite lock contention and ensures connection-level pragmas apply.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping SQLite database: %w", err)
	}
	return db, nil
}

// Close closes the database/sql pool owned by a GORM connection.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying database connection: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}

// VerifySchema fails fast when the independently managed database migrations have not been run.
func VerifySchema(db *gorm.DB) error {
	for _, table := range []string{"users", "sessions", "messages"} {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("required table %q is missing; run the migration service", table)
		}
	}
	return nil
}
