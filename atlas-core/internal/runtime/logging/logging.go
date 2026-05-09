package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/anomalyco/atlas-core/internal/runtime/config"
)

type contextKey string

const (
	requestIDKey   contextKey = "request_id"
	operationIDKey contextKey = "operation_id"
)

type Field struct {
	Key   string
	Value any
}

type Logger struct {
	out       io.Writer
	threshold int
	runID     string
	service   string
}

func New(cfg *config.Config, runID string) *Logger {
	return &Logger{
		out:       os.Stdout,
		threshold: levelValue(cfg.LogLevel),
		runID:     runID,
		service:   "atlas-core",
	}
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func ContextWithOperationID(ctx context.Context, operationID string) context.Context {
	return context.WithValue(ctx, operationIDKey, operationID)
}

func String(key, value string) Field  { return Field{Key: key, Value: value} }
func Any(key string, value any) Field { return Field{Key: key, Value: value} }
func ErrorField(err error) Field {
	if err == nil {
		return Field{}
	}
	return Field{Key: "error", Value: err.Error()}
}

func (l *Logger) SetOutput(out io.Writer) { l.out = out }

func (l *Logger) log(ctx context.Context, level int, levelName, component, message string, fields ...Field) {
	if level < l.threshold {
		return
	}
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"run_id":    l.runID,
		"service":   l.service,
		"component": component,
		"level":     levelName,
		"message":   message,
	}
	if requestID, ok := ctx.Value(requestIDKey).(string); ok && requestID != "" {
		entry["request_id"] = requestID
	}
	if operationID, ok := ctx.Value(operationIDKey).(string); ok && operationID != "" {
		entry["operation_id"] = operationID
	}
	for _, field := range fields {
		if field.Key == "" {
			continue
		}
		entry[field.Key] = field.Value
	}
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging error: %v\n", err)
		return
	}
	fmt.Fprintln(l.out, string(data))
}

func (l *Logger) Debug(component, message string, fields ...Field) {
	l.DebugContext(context.Background(), component, message, fields...)
}

func (l *Logger) DebugContext(ctx context.Context, component, message string, fields ...Field) {
	l.log(ctx, 0, "debug", component, message, fields...)
}

func (l *Logger) Info(component, message string, fields ...Field) {
	l.InfoContext(context.Background(), component, message, fields...)
}

func (l *Logger) InfoContext(ctx context.Context, component, message string, fields ...Field) {
	l.log(ctx, 1, "info", component, message, fields...)
}

func (l *Logger) Warn(component, message string, fields ...Field) {
	l.WarnContext(context.Background(), component, message, fields...)
}

func (l *Logger) WarnContext(ctx context.Context, component, message string, fields ...Field) {
	l.log(ctx, 2, "warn", component, message, fields...)
}

func (l *Logger) Error(component, message string, fields ...Field) {
	l.ErrorContext(context.Background(), component, message, fields...)
}

func (l *Logger) ErrorContext(ctx context.Context, component, message string, fields ...Field) {
	l.log(ctx, 3, "error", component, message, fields...)
}

func levelValue(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return 0
	case "warn":
		return 2
	case "error":
		return 3
	default:
		return 1
	}
}
