package testsupport

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/anomalyco/atlas-core/services/shared/config"
	"github.com/anomalyco/atlas-core/services/shared/envutil"
)

// postgresUnavailableAction describes how a test should react when Postgres is unreachable.
type postgresUnavailableAction int

const (
	postgresUnavailableFail postgresUnavailableAction = iota
	postgresUnavailableSkip
)

func postgresUnavailableActionFromEnv() postgresUnavailableAction {
	skip, _ := strconv.ParseBool(os.Getenv("ATLAS_SKIP_POSTGRES_TESTS"))
	if skip {
		return postgresUnavailableSkip
	}
	return postgresUnavailableFail
}

// RequirePostgresOrSkip fails the test when Postgres is unreachable.
// Set ATLAS_SKIP_POSTGRES_TESTS=true to skip instead (local convenience only).
func RequirePostgresOrSkip(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		return
	}
	switch postgresUnavailableActionFromEnv() {
	case postgresUnavailableSkip:
		t.Skipf("postgres not available: %v", err)
	default:
		t.Fatalf("postgres not available: %v", err)
	}
}

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
