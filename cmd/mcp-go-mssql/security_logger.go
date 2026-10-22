package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
)

// SecurityLogger writes structured security events to stderr.
// Audit events cannot be disabled by MCP logging/setLevel. That setting only
// controls diagnostics. Credential-shaped messages are redacted in full.
type SecurityLogger struct {
	logger      *slog.Logger
	diagnostics *slog.Logger
	levelVar    *slog.LevelVar // dynamic level controlled by MCP logging/setLevel
}

// NewSecurityLogger returns a JSON logger on stderr at Info level.
func NewSecurityLogger() *SecurityLogger {
	return newSecurityLogger(os.Stderr)
}

func newSecurityLogger(w io.Writer) *SecurityLogger {
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: lvl,
	})
	return &SecurityLogger{
		logger:      slog.New(slog.NewJSONHandler(w, nil)).With(slog.String("component", "security")),
		diagnostics: slog.New(handler).With(slog.String("component", "diagnostics")),
		levelVar:    lvl,
	}
}

// Printf logs a formatted message at Info.
// It keeps a printf-style API over slog for existing call sites.
func (sl *SecurityLogger) Printf(format string, args ...interface{}) {
	sl.logger.Info(sl.sanitizeForLogging(fmt.Sprintf(format, args...)))
}

// Debugf logs diagnostic metadata at the client's requested level. Callers
// must never pass raw SQL, request bodies or result data.
func (sl *SecurityLogger) Debugf(format string, args ...interface{}) {
	sl.diagnostics.Debug(sl.sanitizeForLogging(fmt.Sprintf(format, args...)))
}

// LogConnectionAttempt records whether a database connection succeeded.
// Host, user, and password are intentionally omitted.
func (sl *SecurityLogger) LogConnectionAttempt(success bool) {
	sl.logger.Info("database connection attempt",
		slog.Bool("success", success),
	)
}

// sensitivePatterns identifies messages that must be redacted in full.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(password|pwd|secret|key|token)\s*=`),
	regexp.MustCompile(`(?i)://[^\s]*@`),
}

// sanitizeForLogging redacts credential-bearing messages without trying to
// guess the end of a quoted/braced/URL-encoded credential.
func (sl *SecurityLogger) sanitizeForLogging(input string) string {
	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(input) {
			return "*** [credential-bearing message redacted]"
		}
	}
	return input
}
