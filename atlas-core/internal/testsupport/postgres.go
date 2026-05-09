package testsupport

import (
	"os"
	"strings"
	"testing"

	"github.com/anomalyco/atlas-core/internal/runtime/config"
	"github.com/anomalyco/atlas-core/internal/runtime/envutil"
)

func TestPostgresConfig() *config.Config {
	return &config.Config{
		LogLevel:         "debug",
		PostgresHost:     envutil.OrDefault("ATLAS_TEST_POSTGRES_HOST", "localhost"),
		PostgresPort:     envutil.OrDefault("ATLAS_TEST_POSTGRES_PORT", "5432"),
		PostgresUser:     envutil.OrDefault("ATLAS_TEST_POSTGRES_USER", "atlas"),
		PostgresPassword: envutil.OrDefault("ATLAS_TEST_POSTGRES_PASSWORD", "atlas"),
		PostgresDB:       envutil.OrDefault("ATLAS_TEST_POSTGRES_DB", "atlas_core_test"),
		PostgresSSLMode:  envutil.OrDefault("ATLAS_TEST_POSTGRES_SSLMODE", "disable"),
		PostgresMaxConns: 4,
	}
}

func RequireSafeDatabaseCleanup(t testing.TB, dbName string) {
	t.Helper()

	allowCleanup := os.Getenv("ATLAS_ALLOW_DB_CLEANUP") == "true"
	if strings.HasSuffix(strings.ToLower(dbName), "_test") || allowCleanup {
		return
	}

	t.Fatalf("refusing to run cleanup on database %q: database name must end with '_test' or set ATLAS_ALLOW_DB_CLEANUP=true", dbName)
}
