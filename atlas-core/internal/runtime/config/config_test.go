package config

import (
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLoadUsesDefaultsForEmptyEnvironmentValues(t *testing.T) {
	for _, key := range []string{
		"ATLAS_POSTGRES_HOST",
		"ATLAS_POSTGRES_PORT",
		"ATLAS_POSTGRES_USER",
		"ATLAS_POSTGRES_PASSWORD",
		"ATLAS_POSTGRES_DB",
		"ATLAS_POSTGRES_SSLMODE",
		"ATLAS_POSTGRES_MAX_CONNS",
		"ATLAS_OBJECT_STORAGE_DIR",
		"ATLAS_LOG_LEVEL",
		"ATLAS_READY_FILE",
		"ATLAS_RECONCILE_INTERVAL",
		"ATLAS_RECONCILE_TIMEOUT",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed with empty env vars: %v", err)
	}

	if cfg.ReadyFile != "/var/lib/atlas-core/.ready" {
		t.Fatalf("expected default ready file, got %q", cfg.ReadyFile)
	}
	if cfg.PostgresMaxConns != 8 {
		t.Fatalf("expected default max conns 8, got %d", cfg.PostgresMaxConns)
	}
	if cfg.ReconcileInterval != time.Minute {
		t.Fatalf("expected default reconcile interval %s, got %s", time.Minute, cfg.ReconcileInterval)
	}
}

func TestPostgresDSN_EncodesCredentialsAndSSLMode(t *testing.T) {
	cfg := &Config{
		PostgresHost:     "db.example.com",
		PostgresPort:     "5432",
		PostgresUser:     "atlas user",
		PostgresPassword: "pa:ss@word",
		PostgresDB:       "atlas_core",
		PostgresSSLMode:  "require",
	}

	dsn := cfg.PostgresDSN()
	if _, err := pgxpool.ParseConfig(dsn); err != nil {
		t.Fatalf("pgxpool.ParseConfig rejected DSN: %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("Parse DSN failed: %v", err)
	}

	if parsed.User.Username() != cfg.PostgresUser {
		t.Fatalf("expected username %q, got %q", cfg.PostgresUser, parsed.User.Username())
	}
	password, _ := parsed.User.Password()
	if password != cfg.PostgresPassword {
		t.Fatalf("expected password %q, got %q", cfg.PostgresPassword, password)
	}
	if parsed.Query().Get("sslmode") != cfg.PostgresSSLMode {
		t.Fatalf("expected sslmode %q, got %q", cfg.PostgresSSLMode, parsed.Query().Get("sslmode"))
	}
}
