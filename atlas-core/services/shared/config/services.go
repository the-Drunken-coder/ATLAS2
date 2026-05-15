package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type DataStorageConfig struct {
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
	ListenAddress     string
	ReconcileInterval time.Duration
	ReconcileTimeout  time.Duration
}

type FunctionsConfig struct {
	DataStorageAddress string
	LogLevel           string
	ReadyFile          string
	ListenAddress      string
}

func LoadDataStorage() (*DataStorageConfig, error) {
	cfg := &DataStorageConfig{
		PostgresHost:      envOrDefault("ATLAS_POSTGRES_HOST", "localhost"),
		PostgresPort:      envOrDefault("ATLAS_POSTGRES_PORT", "5432"),
		PostgresUser:      envOrDefault("ATLAS_POSTGRES_USER", "atlas"),
		PostgresPassword:  envOrDefault("ATLAS_POSTGRES_PASSWORD", "atlas"),
		PostgresDB:        envOrDefault("ATLAS_POSTGRES_DB", "atlas_core"),
		PostgresSSLMode:   envOrDefault("ATLAS_POSTGRES_SSLMODE", "disable"),
		ObjectStorageDir:  envOrDefault("ATLAS_OBJECT_STORAGE_DIR", "/var/lib/atlas-datastorage/objects"),
		LogLevel:          envOrDefault("ATLAS_LOG_LEVEL", "info"),
		ReadyFile:         envOrDefault("ATLAS_READY_FILE", "/var/lib/atlas-datastorage/.ready"),
		ListenAddress:     envOrDefault("ATLAS_DATASTORAGE_LISTEN_ADDR", "0.0.0.0:8081"),
		ReconcileInterval: durationOrDefault("ATLAS_RECONCILE_INTERVAL", time.Minute),
		ReconcileTimeout:  durationOrDefault("ATLAS_RECONCILE_TIMEOUT", 30*time.Second),
	}
	maxConns, err := int32EnvOrDefault("ATLAS_POSTGRES_MAX_CONNS", 8)
	if err != nil {
		return nil, err
	}
	cfg.PostgresMaxConns = maxConns
	return cfg, cfg.Validate()
}

func LoadFunctions() (*FunctionsConfig, error) {
	cfg := &FunctionsConfig{
		DataStorageAddress: envOrDefault("ATLAS_DATASTORAGE_ADDR", "atlas-datastorage:8081"),
		LogLevel:           envOrDefault("ATLAS_LOG_LEVEL", "info"),
		ReadyFile:          envOrDefault("ATLAS_READY_FILE", "/var/lib/atlas-functions/.ready"),
		ListenAddress:      envOrDefault("ATLAS_FUNCTIONS_LISTEN_ADDR", "0.0.0.0:8080"),
	}
	return cfg, cfg.Validate()
}

func (c *DataStorageConfig) Validate() error {
	if c.PostgresHost == "" || c.PostgresPort == "" || c.PostgresUser == "" || c.PostgresDB == "" || c.PostgresSSLMode == "" {
		return fmt.Errorf("postgres configuration is incomplete")
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
	if c.ListenAddress == "" {
		return fmt.Errorf("ATLAS_DATASTORAGE_LISTEN_ADDR is required")
	}
	if c.ReconcileInterval < 0 {
		return fmt.Errorf("ATLAS_RECONCILE_INTERVAL must be zero or greater")
	}
	if c.ReconcileTimeout <= 0 {
		return fmt.Errorf("ATLAS_RECONCILE_TIMEOUT must be greater than zero")
	}
	return nil
}

func (c *FunctionsConfig) Validate() error {
	if c.DataStorageAddress == "" {
		return fmt.Errorf("ATLAS_DATASTORAGE_ADDR is required")
	}
	if c.ReadyFile == "" {
		return fmt.Errorf("ATLAS_READY_FILE is required")
	}
	if c.ListenAddress == "" {
		return fmt.Errorf("ATLAS_FUNCTIONS_LISTEN_ADDR is required")
	}
	return nil
}

func (c *DataStorageConfig) PostgresDSN() string {
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

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func int32EnvOrDefault(key string, defaultValue int32) (int32, error) {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
		}
		return int32(parsed), nil
	}
	return defaultValue, nil
}

func durationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		parsed, err := time.ParseDuration(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}
