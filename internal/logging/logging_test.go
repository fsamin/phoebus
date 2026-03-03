package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestGELFHandler_BasicOutput(t *testing.T) {
	var buf bytes.Buffer
	h := NewGELFHandler(&buf, slog.LevelDebug)
	logger := slog.New(h)

	logger.Info("test message", "key", "value")

	var msg map[string]any
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		t.Fatalf("failed to parse GELF output: %v", err)
	}

	if msg["version"] != "1.1" {
		t.Errorf("expected version 1.1, got %v", msg["version"])
	}
	if msg["short_message"] != "test message" {
		t.Errorf("expected short_message 'test message', got %v", msg["short_message"])
	}
	if msg["_key"] != "value" {
		t.Errorf("expected _key 'value', got %v", msg["_key"])
	}
	// Level 6 = syslog Informational
	if msg["level"] != float64(6) {
		t.Errorf("expected level 6, got %v", msg["level"])
	}
	if _, ok := msg["timestamp"]; !ok {
		t.Error("expected timestamp field")
	}
	if _, ok := msg["host"]; !ok {
		t.Error("expected host field")
	}
}

func TestGELFHandler_Levels(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected float64
	}{
		{slog.LevelDebug, 7},
		{slog.LevelInfo, 6},
		{slog.LevelWarn, 4},
		{slog.LevelError, 3},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		h := NewGELFHandler(&buf, slog.LevelDebug)
		logger := slog.New(h)

		logger.Log(nil, tt.level, "msg")

		var msg map[string]any
		if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
			t.Fatalf("level %v: failed to parse: %v", tt.level, err)
		}
		if msg["level"] != tt.expected {
			t.Errorf("level %v: expected syslog %v, got %v", tt.level, tt.expected, msg["level"])
		}
	}
}

func TestGELFHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewGELFHandler(&buf, slog.LevelDebug)
	logger := slog.New(h).With("service", "phoebus")

	logger.Info("test")

	var msg map[string]any
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg["_service"] != "phoebus" {
		t.Errorf("expected _service 'phoebus', got %v", msg["_service"])
	}
}

func TestGELFHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := NewGELFHandler(&buf, slog.LevelDebug)
	logger := slog.New(h).WithGroup("sync").With("repo_id", "abc")

	logger.Info("test")

	var msg map[string]any
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	if msg["_sync.repo_id"] != "abc" {
		t.Errorf("expected _sync.repo_id 'abc', got %v", msg["_sync.repo_id"])
	}
}

func TestGELFHandler_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	h := NewGELFHandler(&buf, slog.LevelWarn)
	logger := slog.New(h)

	logger.Info("should be filtered")
	if buf.Len() > 0 {
		t.Error("expected no output for info when level is warn")
	}

	logger.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("expected output for warn when level is warn")
	}
}

func TestGELFHandler_TimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	h := NewGELFHandler(&buf, slog.LevelDebug)
	logger := slog.New(h)

	logger.Info("ts test")

	var msg map[string]any
	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	ts, ok := msg["timestamp"].(float64)
	if !ok {
		t.Fatal("timestamp should be a float64")
	}
	// Should be a reasonable epoch timestamp (after 2020)
	if ts < float64(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()) {
		t.Errorf("timestamp seems too old: %v", ts)
	}
}

func TestSyncCollector_DualWrite(t *testing.T) {
	var buf bytes.Buffer
	parent := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	collector := NewSyncCollector(parent)
	logger := slog.New(collector)

	logger.Info("step 1", "module", "intro")
	logger.Warn("missing field", "file", "step.md")
	logger.Error("parse failed", "error", "bad yaml")

	// Check parent received output
	if buf.Len() == 0 {
		t.Error("parent handler should have received output")
	}

	// Check collected entries
	data, err := collector.Entries()
	if err != nil {
		t.Fatal(err)
	}
	var entries []SyncLogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Level != "info" || entries[0].Message != "step 1" {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Level != "warn" {
		t.Errorf("expected warn, got %s", entries[1].Level)
	}
	if entries[2].Level != "error" {
		t.Errorf("expected error, got %s", entries[2].Level)
	}
	if entries[0].Fields["module"] != "intro" {
		t.Errorf("expected module=intro, got %v", entries[0].Fields["module"])
	}
}

func TestNew_Formats(t *testing.T) {
	for _, fmt := range []string{"json", "text", "gelf", "unknown"} {
		l := New(fmt, "info")
		if l == nil {
			t.Errorf("New(%q) returned nil", fmt)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for input, expected := range tests {
		got := parseLevel(input)
		if got != expected {
			t.Errorf("parseLevel(%q) = %v, want %v", input, got, expected)
		}
	}
}

func TestContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default().With("test", true)
	ctx = WithLogger(ctx, logger)
	got := FromContext(ctx)
	if got != logger {
		t.Error("FromContext should return the stored logger")
	}

	// FromContext with no logger should return default
	got2 := FromContext(context.Background())
	if got2 == nil {
		t.Error("FromContext should return default logger when none set")
	}
}
