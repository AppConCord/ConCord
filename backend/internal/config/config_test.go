package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("CONCORD_HTTP_ADDRESS", "127.0.0.1:9000")
	t.Setenv("CONCORD_DATABASE_PATH", "test.db")
	t.Setenv("CONCORD_SNOWFLAKE_NODE_ID", "12")
	t.Setenv("CONCORD_BCRYPT_COST", "10")
	t.Setenv("CONCORD_SESSION_TTL", "24h")
	t.Setenv("CONCORD_SHUTDOWN_TIMEOUT", "5s")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != "127.0.0.1:9000" || cfg.DatabasePath != "test.db" || cfg.SnowflakeNodeID != 12 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.BcryptCost != 10 || cfg.SessionTTL != 24*time.Hour || cfg.ShutdownTimeout != 5*time.Second {
		t.Fatalf("unexpected numeric config: %#v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := map[string]struct {
		name  string
		value string
	}{
		"node":     {"CONCORD_SNOWFLAKE_NODE_ID", "1024"},
		"bcrypt":   {"CONCORD_BCRYPT_COST", "3"},
		"ttl":      {"CONCORD_SESSION_TTL", "0s"},
		"shutdown": {"CONCORD_SHUTDOWN_TIMEOUT", "invalid"},
		"database": {"CONCORD_DATABASE_PATH", ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CONCORD_HTTP_ADDRESS", ":8080")
			t.Setenv("CONCORD_DATABASE_PATH", "test.db")
			t.Setenv("CONCORD_SNOWFLAKE_NODE_ID", "0")
			t.Setenv("CONCORD_BCRYPT_COST", "12")
			t.Setenv("CONCORD_SESSION_TTL", "1h")
			t.Setenv("CONCORD_SHUTDOWN_TIMEOUT", "1s")
			t.Setenv(test.name, test.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
