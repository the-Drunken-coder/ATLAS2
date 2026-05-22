package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestLogEntryMarshalJSON_IgnoresReservedExtraKeys(t *testing.T) {
	entry := logEntry{
		Timestamp: "2026-01-01T00:00:00Z",
		RunID:     "run",
		Service:   "svc",
		Component: "cmp",
		Level:     "info",
		Message:   "hello",
		Extra: []Field{
			{Key: "level", Value: "error"},
			{Key: "safe", Value: "ok"},
		},
	}
	data, err := entry.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["level"] != "info" {
		t.Fatalf("expected canonical level to remain info, got %#v", out["level"])
	}
	if out["safe"] != "ok" {
		t.Fatalf("expected non-reserved extra field, got %#v", out["safe"])
	}
}

func TestLogger_Log_NilContextSafe(t *testing.T) {
	logger := New("info", "atlas-test", "test")
	var out bytes.Buffer
	logger.out = &out

	logger.InfoContext(nil, "component", "nil ctx ok")
	if !strings.Contains(out.String(), `"message":"nil ctx ok"`) {
		t.Fatalf("expected log output, got %q", out.String())
	}
}

func TestLogger_Log_UsesRequestIDFromContext(t *testing.T) {
	logger := New("info", "atlas-test", "test")
	var out bytes.Buffer
	logger.out = &out

	ctx := ContextWithRequestID(context.Background(), "req-123")
	logger.InfoContext(ctx, "component", "with request id")
	if !strings.Contains(out.String(), `"request_id":"req-123"`) {
		t.Fatalf("expected request_id in output, got %q", out.String())
	}
}

func TestNew_DefaultsUnknownLevelToInfo(t *testing.T) {
	logger := New("verbose", "atlas-test", "test")
	var out bytes.Buffer
	logger.out = &out

	logger.Info("component", "info emitted")
	if !strings.Contains(out.String(), `"level":"info"`) {
		t.Fatalf("expected info log output, got %q", out.String())
	}
}
