package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/anomalyco/atlas-core/internal/config"
)

type Logger struct {
	out     io.Writer
	level   string
	runID   string
	service string
	levels  map[string]int
}

func New(cfg *config.Config, runID string) *Logger {
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	return &Logger{
		out:     os.Stdout,
		level:   cfg.LogLevel,
		runID:   runID,
		service: "atlas-core",
		levels:  levels,
	}
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	RunID     string `json:"run_id"`
	Service   string `json:"service"`
	Component string `json:"component"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

func (l *Logger) log(level, component, message string) {
	if !l.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		RunID:     l.runID,
		Service:   l.service,
		Component: component,
		Level:     level,
		Message:   message,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging error: %v\n", err)
		return
	}

	fmt.Fprintln(l.out, string(data))
}

func (l *Logger) shouldLog(level string) bool {
	threshold, ok := l.levels[l.level]
	if !ok {
		return true
	}
	levelVal, ok := l.levels[level]
	if !ok {
		return true
	}
	return levelVal >= threshold
}

func (l *Logger) Debug(component, message string) {
	l.log("debug", component, message)
}

func (l *Logger) Info(component, message string) {
	l.log("info", component, message)
}

func (l *Logger) Warn(component, message string) {
	l.log("warn", component, message)
}

func (l *Logger) Error(component, message string) {
	l.log("error", component, message)
}
