package main

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
)

// Security Logger — structured logging via log/slog (stdlib Go 1.21+)
type SecurityLogger struct {
	logger   *slog.Logger
	levelVar *slog.LevelVar // dynamic level controlled by MCP logging/setLevel
}

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

// Printf provides backward-compatible formatted logging.
func (sl *SecurityLogger) Printf(format string, args ...interface{}) {
	sl.logger.Info(fmt.Sprintf(format, args...))
}

func (sl *SecurityLogger) LogConnectionAttempt(success bool) {
	sl.logger.Info("database connection attempt",
		slog.Bool("success", success),
	)
}

// Compiled regex patterns for better performance
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|pwd|secret|key|token)=[^;\s]*`),
	regexp.MustCompile(`(?i)(password|pwd)\s*=\s*[^;\s]*`),
}

// Pre-compiled pattern for procedure name validation. Used by
// validateProcedureName below; the SQL-validation patterns now live in the
// sqlguard package.
var validProcedureNamePattern = regexp.MustCompile(`^[\w.\[\]]+$`)

func (sl *SecurityLogger) sanitizeForLogging(input string) string {
	result := input
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "${1}=***")
	}

	return result
}
