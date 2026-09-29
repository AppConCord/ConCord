// Command migrate applies and inspects Concord's independently versioned database migrations.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

const (
	defaultDatabasePath = "./data/concord.db"
	defaultMigrations   = "./migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger, os.Args[1:]); err != nil {
		logger.Error("migration command failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("usage: migrate <up|down|status|version>")
	}
	command := arguments[0]
	if command != "up" && command != "down" && command != "status" && command != "version" {
		return fmt.Errorf("unsupported command %q; expected up, down, status, or version", command)
	}
	databasePath := envOrDefault("CONCORD_DATABASE_PATH", defaultDatabasePath)
	migrationsPath := envOrDefault("CONCORD_MIGRATIONS_DIR", defaultMigrations)
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite3", databasePath+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.Error("close migration database", "error", closeErr)
		}
	}()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set Goose dialect: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch command {
	case "up":
		err = goose.UpContext(ctx, db, migrationsPath)
	case "down":
		err = goose.DownContext(ctx, db, migrationsPath)
	case "status":
		err = goose.StatusContext(ctx, db, migrationsPath)
	case "version":
		var version int64
		version, err = goose.GetDBVersionContext(ctx, db)
		if err == nil {
			logger.Info("database migration version", "version", version)
		}
	}
	if err != nil {
		return fmt.Errorf("goose %s: %w", command, err)
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
