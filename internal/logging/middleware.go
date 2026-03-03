package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type requestIDKey struct{}

// RequestID retrieves the request ID from context.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// Middleware returns an HTTP middleware that:
//   - Extracts or generates a request ID (from header if configured, else UUID)
//   - Injects a contextual logger with request_id into the context
//   - Logs the completed request with all relevant fields
//   - Sets the request ID in the response header
func Middleware(requestIDHeader string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Resolve request ID
			reqID := ""
			if requestIDHeader != "" {
				reqID = r.Header.Get(requestIDHeader)
			}
			if reqID == "" {
				reqID = uuid.New().String()
			}

			// Set response header
			w.Header().Set("X-Request-Id", reqID)

			// Build contextual logger and inject into context
			logger := slog.Default().With("request_id", reqID)
			ctx := WithLogger(r.Context(), logger)
			ctx = context.WithValue(ctx, requestIDKey{}, reqID)

			// Wrap response writer to capture status code and bytes
			sr := &statusRecorder{ResponseWriter: w, statusCode: 200}

			next.ServeHTTP(sr, r.WithContext(ctx))

			duration := time.Since(start)

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", sr.statusCode,
				"duration_ms", duration.Milliseconds(),
				"request_id", reqID,
				"user_agent", r.UserAgent(),
				"remote_addr", r.RemoteAddr,
				"bytes_written", sr.bytesWritten,
			}

			if r.URL.RawQuery != "" {
				attrs = append(attrs, "query", r.URL.RawQuery)
			}

			level := slog.LevelInfo
			msg := fmt.Sprintf("%s %s %d", r.Method, r.URL.Path, sr.statusCode)
			switch {
			case sr.statusCode >= 500:
				level = slog.LevelError
			case sr.statusCode >= 400:
				level = slog.LevelWarn
			}

			logger.Log(r.Context(), level, msg, attrs...)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	n, err := sr.ResponseWriter.Write(b)
	sr.bytesWritten += n
	return n, err
}
