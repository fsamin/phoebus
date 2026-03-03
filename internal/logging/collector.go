package logging

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// SyncLogEntry represents a single log entry captured during a sync job.
type SyncLogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// SyncCollector is a slog.Handler that captures log records in memory
// while also forwarding them to a parent handler (dual-write).
type SyncCollector struct {
	parent  slog.Handler
	entries []SyncLogEntry
	attrs   []slog.Attr
	groups  []string
	mu      sync.Mutex
}

// NewSyncCollector creates a collector that dual-writes to the given parent handler.
func NewSyncCollector(parent slog.Handler) *SyncCollector {
	return &SyncCollector{parent: parent}
}

func (c *SyncCollector) Enabled(ctx context.Context, level slog.Level) bool {
	return c.parent.Enabled(ctx, level)
}

func (c *SyncCollector) Handle(ctx context.Context, r slog.Record) error {
	// Collect entry
	entry := SyncLogEntry{
		Timestamp: r.Time,
		Level:     FormatLevel(r.Level),
		Message:   r.Message,
	}

	fields := make(map[string]any)
	prefix := ""
	for _, g := range c.groups {
		prefix += g + "."
	}
	for _, a := range c.attrs {
		fields[prefix+a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		fields[prefix+a.Key] = a.Value.Any()
		return true
	})
	if len(fields) > 0 {
		entry.Fields = fields
	}

	c.mu.Lock()
	c.entries = append(c.entries, entry)
	c.mu.Unlock()

	// Forward to parent
	return c.parent.Handle(ctx, r)
}

func (c *SyncCollector) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SyncCollector{
		parent:  c.parent.WithAttrs(attrs),
		entries: c.entries, // shared slice pointer
		attrs:   append(cloneAttrs(c.attrs), attrs...),
		groups:  cloneStrings(c.groups),
		mu:      c.mu,
	}
}

func (c *SyncCollector) WithGroup(name string) slog.Handler {
	if name == "" {
		return c
	}
	return &SyncCollector{
		parent:  c.parent.WithGroup(name),
		entries: c.entries,
		attrs:   cloneAttrs(c.attrs),
		groups:  append(cloneStrings(c.groups), name),
		mu:      c.mu,
	}
}

// Entries returns all collected log entries as JSON bytes.
func (c *SyncCollector) Entries() (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) == 0 {
		return json.RawMessage("[]"), nil
	}
	return json.Marshal(c.entries)
}
