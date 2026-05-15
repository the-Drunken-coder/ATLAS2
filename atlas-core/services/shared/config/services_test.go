package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDataStorageRejectsInvalidReconcileInterval(t *testing.T) {
	t.Setenv("ATLAS_RECONCILE_INTERVAL", "not-a-duration")
	_, err := LoadDataStorage()
	if err == nil || !strings.Contains(err.Error(), "ATLAS_RECONCILE_INTERVAL") {
		t.Fatalf("expected reconcile interval parse error, got %v", err)
	}
}

func TestLoadDataStorageRejectsInvalidReconcileTimeout(t *testing.T) {
	t.Setenv("ATLAS_RECONCILE_TIMEOUT", "still-not-a-duration")
	_, err := LoadDataStorage()
	if err == nil || !strings.Contains(err.Error(), "ATLAS_RECONCILE_TIMEOUT") {
		t.Fatalf("expected reconcile timeout parse error, got %v", err)
	}
}

func TestLoadDataStorageAppliesConfigFileDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	content := `{
		"postgres_host": "db.internal",
		"postgres_port": "5433",
		"postgres_user": "atlas_cfg",
		"postgres_password": "secret",
		"postgres_db": "atlas_cfg_db",
		"postgres_sslmode": "require",
		"postgres_max_conns": 16,
		"object_storage_dir": "/srv/atlas/objects",
		"log_level": "debug",
		"ready_file": "/srv/atlas/.ready",
		"reconcile_interval": "2m",
		"reconcile_timeout": "45s"
	}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("ATLAS_CONFIG_FILE", cfgPath)
	cfg, err := LoadDataStorage()
	if err != nil {
		t.Fatalf("load datastorage config: %v", err)
	}

	if cfg.PostgresHost != "db.internal" || cfg.PostgresPort != "5433" || cfg.PostgresUser != "atlas_cfg" || cfg.PostgresDB != "atlas_cfg_db" || cfg.PostgresSSLMode != "require" {
		t.Fatalf("expected config file postgres settings to apply, got %+v", cfg)
	}
	if cfg.PostgresMaxConns != 16 {
		t.Fatalf("expected config file max conns 16, got %d", cfg.PostgresMaxConns)
	}
	if cfg.ObjectStorageDir != "/srv/atlas/objects" || cfg.LogLevel != "debug" || cfg.ReadyFile != "/srv/atlas/.ready" {
		t.Fatalf("expected config file storage/log/ready defaults to apply, got %+v", cfg)
	}
	if cfg.ReconcileInterval.String() != "2m0s" || cfg.ReconcileTimeout.String() != "45s" {
		t.Fatalf("expected config file reconcile durations to apply, got interval=%s timeout=%s", cfg.ReconcileInterval, cfg.ReconcileTimeout)
	}
}
