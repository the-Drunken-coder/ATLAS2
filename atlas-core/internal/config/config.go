package config

import (
	"fmt"
	"os"
)

type Config struct {
	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	ObjectStorageDir string
	LogLevel         string
}

func Load() (*Config, error) {
	cfg := &Config{
		PostgresHost:     envOrDefault("ATLAS_POSTGRES_HOST", "localhost"),
		PostgresPort:     envOrDefault("ATLAS_POSTGRES_PORT", "5432"),
		PostgresUser:     envOrDefault("ATLAS_POSTGRES_USER", "atlas"),
		PostgresPassword: envOrDefault("ATLAS_POSTGRES_PASSWORD", "atlas"),
		PostgresDB:       envOrDefault("ATLAS_POSTGRES_DB", "atlas_core"),
		ObjectStorageDir: envOrDefault("ATLAS_OBJECT_STORAGE_DIR", "/var/lib/atlas-core/objects"),
		LogLevel:         envOrDefault("ATLAS_LOG_LEVEL", "info"),
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
	if c.ObjectStorageDir == "" {
		return fmt.Errorf("ATLAS_OBJECT_STORAGE_DIR is required")
	}
	return nil
}

func (c *Config) PostgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.PostgresUser, c.PostgresPassword,
		c.PostgresHost, c.PostgresPort,
		c.PostgresDB,
	)
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
