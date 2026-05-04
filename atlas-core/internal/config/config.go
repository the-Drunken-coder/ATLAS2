package config

import (
	"fmt"
	"net/url"
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
}

func Load() (*Config, error) {
	maxConns, err := int32FromEnv("ATLAS_POSTGRES_MAX_CONNS", 8)
	if err != nil {
		return nil, err
	}
	reconcileInterval, err := durationFromEnv("ATLAS_RECONCILE_INTERVAL", time.Minute)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		PostgresHost:      envutil.OrDefault("ATLAS_POSTGRES_HOST", "localhost"),
		PostgresPort:      envutil.OrDefault("ATLAS_POSTGRES_PORT", "5432"),
		PostgresUser:      envutil.OrDefault("ATLAS_POSTGRES_USER", "atlas"),
		PostgresPassword:  envutil.OrDefault("ATLAS_POSTGRES_PASSWORD", "atlas"),
		PostgresDB:        envutil.OrDefault("ATLAS_POSTGRES_DB", "atlas_core"),
		PostgresSSLMode:   envutil.OrDefault("ATLAS_POSTGRES_SSLMODE", "disable"),
		PostgresMaxConns:  maxConns,
		ObjectStorageDir:  envutil.OrDefault("ATLAS_OBJECT_STORAGE_DIR", "/var/lib/atlas-core/objects"),
		LogLevel:          envutil.OrDefault("ATLAS_LOG_LEVEL", "info"),
		ReadyFile:         envutil.OrDefault("ATLAS_READY_FILE", "/var/lib/atlas-core/.ready"),
		ReconcileInterval: reconcileInterval,
	}

	return cfg, cfg.Validate()
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

func int32FromEnv(key string, defaultVal int32) (int32, error) {
	val := envutil.OrDefault(key, strconv.FormatInt(int64(defaultVal), 10))
	parsed, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}
	return int32(parsed), nil
}

func durationFromEnv(key string, defaultVal time.Duration) (time.Duration, error) {
	val := envutil.OrDefault(key, defaultVal.String())
	parsed, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return parsed, nil
}
