// Package logging provides the slog-backed implementation of domain.Logger
// (issue #14). Output is sent to os.Stderr — never stdout — because a TUI app
// owns stdout and any stray writes there corrupt the rendered interface.
package logging

import (
	"log/slog"
	"os"
)

// slogLogger adapts *slog.Logger to the domain.Logger port.
type slogLogger struct {
	l *slog.Logger
}

func (s slogLogger) Debug(msg string, args ...any) { s.l.Debug(msg, args...) }
func (s slogLogger) Info(msg string, args ...any)  { s.l.Info(msg, args...) }
func (s slogLogger) Warn(msg string, args ...any)  { s.l.Warn(msg, args...) }
func (s slogLogger) Error(msg string, args ...any) { s.l.Error(msg, args...) }

// NewLogger builds a domain.Logger backed by slog's text handler writing to
// stderr. level selects the minimum severity emitted. Records carry structured
// key/value fields rather than free-form text, so downstream tooling can parse
// and filter them.
func NewLogger(level slog.Level) slogLogger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slogLogger{l: slog.New(handler)}
}
