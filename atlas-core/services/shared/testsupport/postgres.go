package testsupport

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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
	if os.Getenv("ATLAS_SKIP_POSTGRES_TESTS") == "true" {
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

var postgresSchemaUnsafeChars = regexp.MustCompile(`[^a-z0-9_]+`)

// ConfigureIsolatedPostgresSchema creates a temporary schema and points poolCfg at
// it so package-level Go test parallelism cannot make cleanup in one package
// delete another package's fixtures.
func ConfigureIsolatedPostgresSchema(t testing.TB, ctx context.Context, poolCfg *pgxpool.Config) {
	t.Helper()

	schema := postgresTestSchemaName(t)
	adminCfg := poolCfg.Copy()
	adminPool, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatalf("create postgres admin pool: %v", err)
	}
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA `+quotePostgresIdentifier(schema)); err != nil {
		adminPool.Close()
		t.Fatalf("create postgres test schema %q: %v", schema, err)
	}
	t.Cleanup(func() {
		defer adminPool.Close()
		if _, err := adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+quotePostgresIdentifier(schema)+` CASCADE`); err != nil {
			t.Errorf("drop postgres test schema %q: %v", schema, err)
		}
	})

	if poolCfg.ConnConfig.RuntimeParams == nil {
		poolCfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	poolCfg.ConnConfig.RuntimeParams["search_path"] = schema
}

func postgresTestSchemaName(t testing.TB) string {
	name := strings.ToLower(t.Name())
	name = postgresSchemaUnsafeChars.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		name = "test"
	}
	if len(name) > 32 {
		name = name[:32]
	}
	return "atlas_test_" + strconv.Itoa(os.Getpid()) + "_" + name + "_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func quotePostgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
