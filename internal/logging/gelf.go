package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// GELFHandler is a slog.Handler that outputs GELF 1.1 spec-compliant JSON.
// See https://go2docs.graylog.org/current/getting_in_log_data/gelf.html
type GELFHandler struct {
	w      io.Writer
	level  slog.Level
	host   string
	attrs  []slog.Attr
	groups []string
	mu     sync.Mutex
}

// NewGELFHandler creates a new GELF handler writing to w.
func NewGELFHandler(w io.Writer, level slog.Level) *GELFHandler {
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown"
	}
	return &GELFHandler{w: w, level: level, host: host}
}

func (h *GELFHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *GELFHandler) Handle(_ context.Context, r slog.Record) error {
	msg := make(map[string]any)
	msg["version"] = "1.1"
	msg["host"] = h.host
	msg["short_message"] = r.Message
	msg["timestamp"] = float64(r.Time.UnixNano()) / 1e9
	msg["level"] = slogToSyslog(r.Level)

	// Pre-set attrs from WithAttrs/WithGroup
	prefix := ""
	if len(h.groups) > 0 {
		for _, g := range h.groups {
			prefix += g + "."
		}
	}
	for _, a := range h.attrs {
		msg["_"+prefix+a.Key] = a.Value.Any()
	}

	// Record attrs
	r.Attrs(func(a slog.Attr) bool {
		msg["_"+prefix+a.Key] = a.Value.Any()
		return true
	})

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("gelf marshal: %w", err)
	}
	data = append(data, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err = h.w.Write(data)
	return err
}

func (h *GELFHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &GELFHandler{
		w:      h.w,
		level:  h.level,
		host:   h.host,
		attrs:  append(cloneAttrs(h.attrs), attrs...),
		groups: cloneStrings(h.groups),
	}
}

func (h *GELFHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &GELFHandler{
		w:      h.w,
		level:  h.level,
		host:   h.host,
		attrs:  cloneAttrs(h.attrs),
		groups: append(cloneStrings(h.groups), name),
	}
}

// slogToSyslog maps slog levels to syslog severity levels used by GELF.
func slogToSyslog(l slog.Level) int {
	switch {
	case l >= slog.LevelError:
		return 3 // Error
	case l >= slog.LevelWarn:
		return 4 // Warning
	case l >= slog.LevelInfo:
		return 6 // Informational
	default:
		return 7 // Debug
	}
}

func cloneAttrs(a []slog.Attr) []slog.Attr {
	if a == nil {
		return nil
	}
	c := make([]slog.Attr, len(a))
	copy(c, a)
	return c
}

func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}
