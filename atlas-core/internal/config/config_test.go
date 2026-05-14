package config

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLoadUsesDefaultsForEmptyEnvironmentValues(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, key := range []string{
		"ATLAS_CONFIG_FILE",
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

func TestLoadConfigJSONInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	clearAtlasEnv(t)
	content := `{
  "postgres_host": "from-json.example",
  "postgres_port": "5433",
  "ready_file": "/tmp/ready-from-json"
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PostgresHost != "from-json.example" {
		t.Fatalf("postgres host: got %q", cfg.PostgresHost)
	}
	if cfg.PostgresPort != "5433" {
		t.Fatalf("postgres port: got %q", cfg.PostgresPort)
	}
	if cfg.ReadyFile != "/tmp/ready-from-json" {
		t.Fatalf("ready file: got %q", cfg.ReadyFile)
	}
	// Unset in JSON → still default from defaultConfig()
	if cfg.PostgresUser != "atlas" {
		t.Fatalf("expected default user from defaults, got %q", cfg.PostgresUser)
	}
}

func TestLoadEnvOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	clearAtlasEnv(t)
	content := `{"postgres_host": "json-host", "log_level": "json-level"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ATLAS_POSTGRES_HOST", "env-host")
	t.Setenv("ATLAS_LOG_LEVEL", "env-level")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PostgresHost != "env-host" {
		t.Fatalf("expected env override for host, got %q", cfg.PostgresHost)
	}
	if cfg.LogLevel != "env-level" {
		t.Fatalf("expected env override for log level, got %q", cfg.LogLevel)
	}
}

func TestLoadATLAS_CONFIGFilePath(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	clearAtlasEnv(t)

	cfgPath := filepath.Join(dir, "custom.json")
	if err := os.WriteFile(cfgPath, []byte(`{"postgres_host": "custom-file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ATLAS_CONFIG_FILE", cfgPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PostgresHost != "custom-file" {
		t.Fatalf("host: got %q", cfg.PostgresHost)
	}
}

func TestLoadConfigFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	clearAtlasEnv(t)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func clearAtlasEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ATLAS_CONFIG_FILE",
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
