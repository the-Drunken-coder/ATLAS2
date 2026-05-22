package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

type Field struct {
	Key   string
	Value any
}

type logEntry struct {
	Timestamp string  `json:"timestamp"`
	RunID     string  `json:"run_id"`
	Service   string  `json:"service"`
	Component string  `json:"component"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	RequestID string  `json:"request_id,omitempty"`
	Extra     []Field `json:"-"`
}

var reservedLogFieldKeys = map[string]struct{}{
	"timestamp":  {},
	"run_id":     {},
	"service":    {},
	"component":  {},
	"level":      {},
	"message":    {},
	"request_id": {},
}

func (e logEntry) MarshalJSON() ([]byte, error) {
	if len(e.Extra) == 0 {
		type alias logEntry
		return json.Marshal(alias(e))
	}
	out := map[string]any{
		"timestamp": e.Timestamp,
		"run_id":    e.RunID,
		"service":   e.Service,
		"component": e.Component,
		"level":     e.Level,
		"message":   e.Message,
	}
	if e.RequestID != "" {
		out["request_id"] = e.RequestID
	}
	for _, field := range e.Extra {
		if field.Key == "" {
			continue
		}
		if _, reserved := reservedLogFieldKeys[field.Key]; reserved {
			continue
		}
		out[field.Key] = field.Value
	}
	return json.Marshal(out)
}

type Logger struct {
	out       io.Writer
	threshold int
	runID     string
	service   string
}

func New(logLevel, service, runID string) *Logger {
	return &Logger{
		out:       os.Stdout,
		threshold: levelValue(logLevel),
		runID:     runID,
		service:   service,
	}
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok && id != ""
}

func String(key, value string) Field  { return Field{Key: key, Value: value} }
func Any(key string, value any) Field { return Field{Key: key, Value: value} }
func ErrorField(err error) Field {
	if err == nil {
		return Field{}
	}
	return Field{Key: "error", Value: err.Error()}
}

func (l *Logger) log(ctx context.Context, level int, levelName, component, message string, fields ...Field) {
	if level < l.threshold {
		return
	}
	entry := logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		RunID:     l.runID,
		Service:   l.service,
		Component: component,
		Level:     levelName,
		Message:   message,
	}
	if requestID, ok := RequestIDFromContext(ctx); ok {
		entry.RequestID = requestID
	}
	for _, field := range fields {
		if field.Key == "" {
			continue
		}
		entry.Extra = append(entry.Extra, field)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging error: %v\n", err)
		return
	}
	fmt.Fprintln(l.out, string(data))
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
