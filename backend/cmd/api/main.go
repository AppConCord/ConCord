// Command api starts the Concord HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"concord/backend/internal/auth"
	"concord/backend/internal/config"
	"concord/backend/internal/database"
	"concord/backend/internal/httpapi"
	"concord/backend/internal/messages"
	"concord/backend/internal/snowflake"
	"concord/backend/internal/store"
	"github.com/labstack/echo/v5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("backend stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			logger.Error("close database", "error", closeErr)
		}
	}()
	if err := database.VerifySchema(db); err != nil {
		return err
	}

	ids, err := snowflake.NewGenerator(cfg.SnowflakeNodeID)
	if err != nil {
		return fmt.Errorf("create Snowflake generator: %w", err)
	}
	persistence := store.New(db)
	authService := auth.NewService(persistence, ids, cfg.BcryptCost, cfg.SessionTTL)
	messageService := messages.NewService(persistence, ids)
	server := httpapi.New(authService, messageService, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startConfig := echo.StartConfig{
		Address:         cfg.Address,
		GracefulTimeout: cfg.ShutdownTimeout,
		HideBanner:      true,
		HidePort:        true,
		BeforeServeFunc: func(httpServer *http.Server) error {
			httpServer.ReadHeaderTimeout = 5 * time.Second
			httpServer.IdleTimeout = time.Minute
			httpServer.MaxHeaderBytes = 1 << 20
			return nil
		},
		OnShutdownError: func(shutdownErr error) {
			logger.Error("graceful shutdown failed", "error", shutdownErr)
		},
	}
	logger.Info("starting backend", "address", cfg.Address, "database", cfg.DatabasePath)
	if err := startConfig.Start(ctx, server.Handler()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
