package main

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
)

// SecurityLogger writes structured security events to stderr.
// Secrets are redacted before they are logged. The level starts at Info and
// can be changed at runtime by the MCP logging/setLevel request.
type SecurityLogger struct {
	logger   *slog.Logger
	levelVar *slog.LevelVar // dynamic level controlled by MCP logging/setLevel
}

// NewSecurityLogger returns a JSON logger on stderr at Info level.
func NewSecurityLogger() *SecurityLogger {
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	})
	return &SecurityLogger{
		logger:   slog.New(handler).With(slog.String("component", "security")),
		levelVar: lvl,
	}
}

// Printf logs a formatted message at Info.
// It keeps a printf-style API over slog for existing call sites.
func (sl *SecurityLogger) Printf(format string, args ...interface{}) {
	sl.logger.Info(fmt.Sprintf(format, args...))
}

// Debugf logs at Debug level. Used for verbose payloads (e.g. the full request
// body / query text) that may contain PII embedded in SQL literals and should
// not be emitted at the default Info level. Gated by the dynamic level set via
// the MCP logging/setLevel request.
func (sl *SecurityLogger) Debugf(format string, args ...interface{}) {
	sl.logger.Debug(fmt.Sprintf(format, args...))
}

// LogConnectionAttempt records whether a database connection succeeded.
// Host, user, and password are intentionally omitted.
func (sl *SecurityLogger) LogConnectionAttempt(success bool) {
	sl.logger.Info("database connection attempt",
		slog.Bool("success", success),
	)
}

// sensitivePatterns redacts credential-shaped substrings before logging.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|pwd|secret|key|token)=[^;\s]*`),
	regexp.MustCompile(`(?i)(password|pwd)\s*=\s*[^;\s]*`),
	regexp.MustCompile(`(?i)(://[^:/\s]+:)[^@/\s]+@`),
}

// sanitizeForLogging replaces credential-shaped substrings with ***.
func (sl *SecurityLogger) sanitizeForLogging(input string) string {
	result := input
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "${1}=***")
	}

	return result
}
