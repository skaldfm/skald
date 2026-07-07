// Package logging configures the application's structured logger (log/slog) and
// provides an HTTP request-logging middleware that shares the same stream.
package logging

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// httpRequests counts HTTP requests served, exposed via HTTPRequestCount for the
// metrics endpoint.
var httpRequests atomic.Int64

// HTTPRequestCount returns the number of HTTP requests handled since start.
func HTTPRequestCount() int64 { return httpRequests.Load() }

// Setup builds a slog logger at the given level ("debug"/"info"/"warn"/"error")
// and format ("text" or "json"), installs it as the default, and returns it.
func Setup(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler
	if strings.EqualFold(format, "json") {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// statusRecorder captures the response status code for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// RequestLogger logs one slog record per HTTP request. 5xx responses log at
// Error, 4xx at Warn, everything else at Info — so SKALD_LOG_LEVEL meaningfully
// filters request noise.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpRequests.Add(1)
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}
		slog.LogAttrs(r.Context(), level, "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
			slog.String("remote", r.RemoteAddr),
		)
	})
}
