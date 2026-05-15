package config

import (
	"strings"
	"testing"
)

func TestLoadDataStorageRejectsInvalidReconcileInterval(t *testing.T) {
	t.Setenv("ATLAS_RECONCILE_INTERVAL", "not-a-duration")
	_, err := LoadDataStorage()
	if err == nil || !strings.Contains(err.Error(), "ATLAS_RECONCILE_INTERVAL") {
		t.Fatalf("expected reconcile interval parse error, got %v", err)
	}
}

func TestLoadDataStorageRejectsInvalidReconcileTimeout(t *testing.T) {
	t.Setenv("ATLAS_RECONCILE_TIMEOUT", "still-not-a-duration")
	_, err := LoadDataStorage()
	if err == nil || !strings.Contains(err.Error(), "ATLAS_RECONCILE_TIMEOUT") {
		t.Fatalf("expected reconcile timeout parse error, got %v", err)
	}
}
