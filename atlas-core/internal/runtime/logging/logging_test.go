package logging

import (
	"bytes"
	"strings"
	"testing"

	"github.com/anomalyco/atlas-core/internal/runtime/config"
)

func TestNew_DefaultsUnknownLevelToInfo(t *testing.T) {
	logger := New(&config.Config{LogLevel: "verbose"}, "test")
	var out bytes.Buffer
	logger.out = &out

	logger.Debug("component", "debug suppressed")
	if out.Len() != 0 {
		t.Fatal("expected debug log to be suppressed when log level falls back to info")
	}

	logger.Info("component", "info emitted")
	if !strings.Contains(out.String(), `"level":"info"`) {
		t.Fatalf("expected info log output, got %q", out.String())
	}
}
