package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/anomalyco/atlas-core/internal/envutil"
)

type Config struct {
	PostgresHost      string
	PostgresPort      string
	PostgresUser      string
	PostgresPassword  string
	PostgresDB        string
	PostgresSSLMode   string
	PostgresMaxConns  int32
	ObjectStorageDir  string
	LogLevel          string
	ReadyFile         string
	ReconcileInterval time.Duration
	ReconcileTimeout  time.Duration
}

// configFileJSON is the optional on-disk JSON shape (atlas-core/config.example.json).
// Empty or omitted fields do not override earlier defaults.
type configFileJSON struct {
	PostgresHost      string `json:"postgres_host,omitempty"`
	PostgresPort      string `json:"postgres_port,omitempty"`
	PostgresUser      string `json:"postgres_user,omitempty"`
	PostgresPassword  string `json:"postgres_password,omitempty"`
	PostgresDB        string `json:"postgres_db,omitempty"`
	PostgresSSLMode   string `json:"postgres_sslmode,omitempty"`
	PostgresMaxConns  *int32 `json:"postgres_max_conns,omitempty"`
	ObjectStorageDir  string `json:"object_storage_dir,omitempty"`
	LogLevel          string `json:"log_level,omitempty"`
	ReadyFile         string `json:"ready_file,omitempty"`
	ReconcileInterval string `json:"reconcile_interval,omitempty"`
	ReconcileTimeout  string `json:"reconcile_timeout,omitempty"`
}

func Load() (*Config, error) {
	cfg := defaultConfig()

	path, err := resolveConfigFilePath()
	if err != nil {
		return nil, err
	}
	if path != "" {
		if err := applyConfigFile(cfg, path); err != nil {
			return nil, err
		}
	}

	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}

	return cfg, cfg.Validate()
}

func defaultConfig() *Config {
	return &Config{
		PostgresHost:      "localhost",
		PostgresPort:      "5432",
		PostgresUser:      "atlas",
		PostgresPassword:  "atlas",
		PostgresDB:        "atlas_core",
		PostgresSSLMode:   "disable",
		PostgresMaxConns:  8,
		ObjectStorageDir:  "/var/lib/atlas-core/objects",
		LogLevel:          "info",
		ReadyFile:         "/var/lib/atlas-core/.ready",
		ReconcileInterval: time.Minute,
		ReconcileTimeout:  30 * time.Second,
	}
}

func resolveConfigFilePath() (path string, err error) {
	if p := envutil.OrDefault("ATLAS_CONFIG_FILE", ""); p != "" {
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	candidate := filepath.Join(wd, "config.json")
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate, nil
	}
	return "", nil
}

func applyConfigFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	var raw configFileJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}

	if raw.PostgresHost != "" {
		cfg.PostgresHost = raw.PostgresHost
	}
	if raw.PostgresPort != "" {
		cfg.PostgresPort = raw.PostgresPort
	}
	if raw.PostgresUser != "" {
		cfg.PostgresUser = raw.PostgresUser
	}
	if raw.PostgresPassword != "" {
		cfg.PostgresPassword = raw.PostgresPassword
	}
	if raw.PostgresDB != "" {
		cfg.PostgresDB = raw.PostgresDB
	}
	if raw.PostgresSSLMode != "" {
		cfg.PostgresSSLMode = raw.PostgresSSLMode
	}
	if raw.PostgresMaxConns != nil {
		cfg.PostgresMaxConns = *raw.PostgresMaxConns
	}
	if raw.ObjectStorageDir != "" {
		cfg.ObjectStorageDir = raw.ObjectStorageDir
	}
	if raw.LogLevel != "" {
		cfg.LogLevel = raw.LogLevel
	}
	if raw.ReadyFile != "" {
		cfg.ReadyFile = raw.ReadyFile
	}
	if raw.ReconcileInterval != "" {
		d, err := time.ParseDuration(raw.ReconcileInterval)
		if err != nil {
			return fmt.Errorf("config file %q: reconcile_interval: %w", path, err)
		}
		cfg.ReconcileInterval = d
	}
	if raw.ReconcileTimeout != "" {
		d, err := time.ParseDuration(raw.ReconcileTimeout)
		if err != nil {
			return fmt.Errorf("config file %q: reconcile_timeout: %w", path, err)
		}
		cfg.ReconcileTimeout = d
	}
	return nil
}

func applyEnvOverrides(c *Config) error {
	if v, ok := os.LookupEnv("ATLAS_POSTGRES_HOST"); ok && v != "" {
		c.PostgresHost = v
	}
	if v, ok := os.LookupEnv("ATLAS_POSTGRES_PORT"); ok && v != "" {
		c.PostgresPort = v
	}
	if v, ok := os.LookupEnv("ATLAS_POSTGRES_USER"); ok && v != "" {
		c.PostgresUser = v
	}
	if v, ok := os.LookupEnv("ATLAS_POSTGRES_PASSWORD"); ok && v != "" {
		c.PostgresPassword = v
	}
	if v, ok := os.LookupEnv("ATLAS_POSTGRES_DB"); ok && v != "" {
		c.PostgresDB = v
	}
	if v, ok := os.LookupEnv("ATLAS_POSTGRES_SSLMODE"); ok && v != "" {
		c.PostgresSSLMode = v
	}
	if v, ok := os.LookupEnv("ATLAS_POSTGRES_MAX_CONNS"); ok && v != "" {
		parsed, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return fmt.Errorf("ATLAS_POSTGRES_MAX_CONNS must be a valid integer: %w", err)
		}
		c.PostgresMaxConns = int32(parsed)
	}
	if v, ok := os.LookupEnv("ATLAS_OBJECT_STORAGE_DIR"); ok && v != "" {
		c.ObjectStorageDir = v
	}
	if v, ok := os.LookupEnv("ATLAS_LOG_LEVEL"); ok && v != "" {
		c.LogLevel = v
	}
	if v, ok := os.LookupEnv("ATLAS_READY_FILE"); ok && v != "" {
		c.ReadyFile = v
	}
	if v, ok := os.LookupEnv("ATLAS_RECONCILE_INTERVAL"); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("ATLAS_RECONCILE_INTERVAL must be a valid duration: %w", err)
		}
		c.ReconcileInterval = d
	}
	if v, ok := os.LookupEnv("ATLAS_RECONCILE_TIMEOUT"); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("ATLAS_RECONCILE_TIMEOUT must be a valid duration: %w", err)
		}
		c.ReconcileTimeout = d
	}
	return nil
}

func (c *Config) Validate() error {
	if c.PostgresHost == "" {
		return fmt.Errorf("ATLAS_POSTGRES_HOST is required")
	}
	if c.PostgresPort == "" {
		return fmt.Errorf("ATLAS_POSTGRES_PORT is required")
	}
	if c.PostgresUser == "" {
		return fmt.Errorf("ATLAS_POSTGRES_USER is required")
	}
	if c.PostgresDB == "" {
		return fmt.Errorf("ATLAS_POSTGRES_DB is required")
	}
	if c.PostgresSSLMode == "" {
		return fmt.Errorf("ATLAS_POSTGRES_SSLMODE is required")
	}
	if c.PostgresMaxConns < 1 {
		return fmt.Errorf("ATLAS_POSTGRES_MAX_CONNS must be greater than zero")
	}
	if c.ObjectStorageDir == "" {
		return fmt.Errorf("ATLAS_OBJECT_STORAGE_DIR is required")
	}
	if c.ReadyFile == "" {
		return fmt.Errorf("ATLAS_READY_FILE is required")
	}
	if c.ReconcileInterval < 0 {
		return fmt.Errorf("ATLAS_RECONCILE_INTERVAL must be zero or greater")
	}
	if c.ReconcileTimeout <= 0 {
		return fmt.Errorf("ATLAS_RECONCILE_TIMEOUT must be greater than zero")
	}
	return nil
}

func (c *Config) PostgresDSN() string {
	return (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:   fmt.Sprintf("%s:%s", c.PostgresHost, c.PostgresPort),
		Path:   c.PostgresDB,
		RawQuery: url.Values{
			"sslmode": []string{c.PostgresSSLMode},
		}.Encode(),
	}).String()
}
