package config

import (
	"net/url"
	"testing"
)

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
