package envutil

import "testing"

func TestOrDefaultUsesDefaultForUnsetAndEmptyValues(t *testing.T) {
	const (
		defaultVal = "fallback"
	)

	t.Run("unset", func(t *testing.T) {
		const key = "ATLAS_ENVUTIL_TEST_UNSET"
		if got := OrDefault(key, defaultVal); got != defaultVal {
			t.Fatalf("expected default %q for unset env var, got %q", defaultVal, got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		const key = "ATLAS_ENVUTIL_TEST_EMPTY"
		t.Setenv(key, "")
		if got := OrDefault(key, defaultVal); got != defaultVal {
			t.Fatalf("expected default %q for empty env var, got %q", defaultVal, got)
		}
	})

	t.Run("present", func(t *testing.T) {
		const key = "ATLAS_ENVUTIL_TEST_PRESENT"
		t.Setenv(key, "configured")
		if got := OrDefault(key, defaultVal); got != "configured" {
			t.Fatalf("expected configured value, got %q", got)
		}
	})
}
