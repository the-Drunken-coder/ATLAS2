package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	InternalToken     string
	LogLevel          string
	ReadyFile         string
	ListenAddress     string
	ReconcileInterval time.Duration
	ReconcileTimeout  time.Duration
}

type FunctionsConfig struct {
	DataStorageAddress string
	DataStorageToken   string
	LogLevel           string
	ReadyFile          string
	ListenAddress      string
}

func LoadDataStorage() (*DataStorageConfig, error) {
	sharedDefaults := &Config{
		PostgresHost:      "localhost",
		PostgresPort:      "5432",
		PostgresUser:      "atlas",
		PostgresPassword:  "atlas",
		PostgresDB:        "atlas_core",
		PostgresSSLMode:   "disable",
		PostgresMaxConns:  8,
		ObjectStorageDir:  "/var/lib/atlas-datastorage/objects",
		LogLevel:          "info",
		ReadyFile:         "/var/lib/atlas-datastorage/.ready",
		ReconcileInterval: time.Minute,
		ReconcileTimeout:  30 * time.Second,
	}
	if err := applySharedConfigFile(sharedDefaults); err != nil {
		return nil, err
	}

	reconcileInterval, err := durationEnvOrDefault("ATLAS_RECONCILE_INTERVAL", sharedDefaults.ReconcileInterval)
	if err != nil {
		return nil, err
	}
	reconcileTimeout, err := durationEnvOrDefault("ATLAS_RECONCILE_TIMEOUT", sharedDefaults.ReconcileTimeout)
	if err != nil {
		return nil, err
	}

	cfg := &DataStorageConfig{
		PostgresHost:      envOrDefault("ATLAS_POSTGRES_HOST", sharedDefaults.PostgresHost),
		PostgresPort:      envOrDefault("ATLAS_POSTGRES_PORT", sharedDefaults.PostgresPort),
		PostgresUser:      envOrDefault("ATLAS_POSTGRES_USER", sharedDefaults.PostgresUser),
		PostgresPassword:  envOrDefault("ATLAS_POSTGRES_PASSWORD", sharedDefaults.PostgresPassword),
		PostgresDB:        envOrDefault("ATLAS_POSTGRES_DB", sharedDefaults.PostgresDB),
		PostgresSSLMode:   envOrDefault("ATLAS_POSTGRES_SSLMODE", sharedDefaults.PostgresSSLMode),
		ObjectStorageDir:  envOrDefault("ATLAS_OBJECT_STORAGE_DIR", sharedDefaults.ObjectStorageDir),
		InternalToken:     envOrDefault("ATLAS_DATASTORAGE_INTERNAL_TOKEN", ""),
		LogLevel:          envOrDefault("ATLAS_LOG_LEVEL", sharedDefaults.LogLevel),
		ReadyFile:         envOrDefault("ATLAS_READY_FILE", sharedDefaults.ReadyFile),
		ListenAddress:     envOrDefault("ATLAS_DATASTORAGE_LISTEN_ADDR", "0.0.0.0:8081"),
		ReconcileInterval: reconcileInterval,
		ReconcileTimeout:  reconcileTimeout,
	}
	maxConns, err := int32EnvOrDefault("ATLAS_POSTGRES_MAX_CONNS", sharedDefaults.PostgresMaxConns)
	if err != nil {
		return nil, err
	}
	cfg.PostgresMaxConns = maxConns
	return cfg, cfg.Validate()
}

func LoadFunctions() (*FunctionsConfig, error) {
	sharedDefaults := &Config{
		LogLevel:  "info",
		ReadyFile: "/var/lib/atlas-functions/.ready",
	}
	if err := applySharedConfigFile(sharedDefaults); err != nil {
		return nil, err
	}

	cfg := &FunctionsConfig{
		DataStorageAddress: envOrDefault("ATLAS_DATASTORAGE_ADDR", "atlas-datastorage:8081"),
		DataStorageToken:   envOrDefault("ATLAS_DATASTORAGE_INTERNAL_TOKEN", ""),
		LogLevel:           envOrDefault("ATLAS_LOG_LEVEL", sharedDefaults.LogLevel),
		ReadyFile:          envOrDefault("ATLAS_READY_FILE", sharedDefaults.ReadyFile),
		ListenAddress:      envOrDefault("ATLAS_FUNCTIONS_LISTEN_ADDR", "0.0.0.0:8080"),
	}
	return cfg, cfg.Validate()
}

func applySharedConfigFile(cfg *Config) error {
	path, err := resolveConfigFilePath()
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	return applyConfigFile(cfg, path)
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
	if strings.TrimSpace(c.InternalToken) == "" {
		return fmt.Errorf("ATLAS_DATASTORAGE_INTERNAL_TOKEN is required")
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
	if strings.TrimSpace(c.DataStorageToken) == "" {
		return fmt.Errorf("ATLAS_DATASTORAGE_INTERNAL_TOKEN is required")
	}
	if c.ReadyFile == "" {
		return fmt.Errorf("ATLAS_READY_FILE is required")
	}
	if c.ListenAddress == "" {
		return fmt.Errorf("ATLAS_FUNCTIONS_LISTEN_ADDR is required")
	}
	return nil
}

func postgresDSN(user, password, host, port, db, sslmode string) string {
	return (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   db,
		RawQuery: url.Values{
			"sslmode": []string{sslmode},
		}.Encode(),
	}).String()
}

func (c *DataStorageConfig) PostgresDSN() string {
	return postgresDSN(c.PostgresUser, c.PostgresPassword, c.PostgresHost, c.PostgresPort, c.PostgresDB, c.PostgresSSLMode)
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

func durationEnvOrDefault(key string, defaultValue time.Duration) (time.Duration, error) {
	if value := os.Getenv(key); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
		}
		return parsed, nil
	}
	return defaultValue, nil
}
