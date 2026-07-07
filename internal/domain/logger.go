package domain

// Logger is the structured logging port used across the application and
// infrastructure layers. It mirrors the subset of log/slog's API that the
// codebase needs, so adapters can depend on this abstraction rather than a
// concrete logger (issue #14).
//
// Args are key/value pairs, identical to slog's ...any convention, e.g.
//
//	logger.Warn("download failed", "job_id", job.ID, "err", err)
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// NoopLogger discards every record. It is the zero-value default used when no
// logger has been wired in (e.g. in tests that don't assert on output).
type NoopLogger struct{}

func (NoopLogger) Debug(string, ...any) {}
func (NoopLogger) Info(string, ...any)  {}
func (NoopLogger) Warn(string, ...any)  {}
func (NoopLogger) Error(string, ...any) {}
