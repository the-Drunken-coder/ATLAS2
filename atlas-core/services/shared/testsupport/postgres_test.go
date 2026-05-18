package testsupport

import "testing"

func TestPostgresUnavailableActionFromEnv(t *testing.T) {
	t.Run("default fails", func(t *testing.T) {
		t.Setenv("ATLAS_SKIP_POSTGRES_TESTS", "")
		if got := postgresUnavailableActionFromEnv(); got != postgresUnavailableFail {
			t.Fatalf("expected fail action, got %v", got)
		}
	})

	t.Run("explicit skip", func(t *testing.T) {
		t.Setenv("ATLAS_SKIP_POSTGRES_TESTS", "true")
		if got := postgresUnavailableActionFromEnv(); got != postgresUnavailableSkip {
			t.Fatalf("expected skip action, got %v", got)
		}
	})

	t.Run("other values do not skip", func(t *testing.T) {
		t.Setenv("ATLAS_SKIP_POSTGRES_TESTS", "1")
		if got := postgresUnavailableActionFromEnv(); got != postgresUnavailableFail {
			t.Fatalf("expected fail action, got %v", got)
		}
	})
}

func TestRequirePostgresOrSkipNoError(t *testing.T) {
	RequirePostgresOrSkip(t, nil)
}
