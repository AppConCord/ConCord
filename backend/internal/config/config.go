// Package config loads and validates backend configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultAddress         = ":8080"
	defaultBcryptCost      = 12
	defaultDatabasePath    = "./data/concord.db"
	defaultSessionTTL      = 30 * 24 * time.Hour
	defaultShutdownTimeout = 10 * time.Second
)

// Config contains all runtime configuration required by the HTTP API.
type Config struct {
	Address         string
	BcryptCost      int
	DatabasePath    string
	SessionTTL      time.Duration
	ShutdownTimeout time.Duration
	SnowflakeNodeID int64
}

// Load reads configuration from CONCORD_* environment variables and applies development defaults.
func Load() (Config, error) {
	cfg := Config{
		Address:         envOrDefault("CONCORD_HTTP_ADDRESS", defaultAddress),
		DatabasePath:    envOrDefault("CONCORD_DATABASE_PATH", defaultDatabasePath),
		SessionTTL:      defaultSessionTTL,
		ShutdownTimeout: defaultShutdownTimeout,
		BcryptCost:      defaultBcryptCost,
	}

	var err error
	if cfg.SnowflakeNodeID, err = parseInt64("CONCORD_SNOWFLAKE_NODE_ID", 0); err != nil {
		return Config{}, err
	}
	if cfg.BcryptCost, err = parseInt("CONCORD_BCRYPT_COST", defaultBcryptCost); err != nil {
		return Config{}, err
	}
	if cfg.SessionTTL, err = parseDuration("CONCORD_SESSION_TTL", defaultSessionTTL); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = parseDuration("CONCORD_SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return Config{}, err
	}

	if cfg.Address == "" {
		return Config{}, fmt.Errorf("CONCORD_HTTP_ADDRESS cannot be empty")
	}
	if cfg.DatabasePath == "" {
		return Config{}, fmt.Errorf("CONCORD_DATABASE_PATH cannot be empty")
	}
	if cfg.SnowflakeNodeID < 0 || cfg.SnowflakeNodeID > 1023 {
		return Config{}, fmt.Errorf("CONCORD_SNOWFLAKE_NODE_ID must be between 0 and 1023")
	}
	if cfg.BcryptCost < 4 || cfg.BcryptCost > 31 {
		return Config{}, fmt.Errorf("CONCORD_BCRYPT_COST must be between 4 and 31")
	}
	if cfg.SessionTTL <= 0 {
		return Config{}, fmt.Errorf("CONCORD_SESSION_TTL must be positive")
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("CONCORD_SHUTDOWN_TIMEOUT must be positive")
	}

	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func parseDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return value, nil
}

func parseInt(name string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return value, nil
}

func parseInt64(name string, fallback int64) (int64, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return value, nil
}
