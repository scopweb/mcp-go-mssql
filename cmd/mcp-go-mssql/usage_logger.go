package main

// Usage accounting: one counted record per tool call.
//
// Every record is a fact about a request that really happened: bytes on the
// wire, elapsed time, rows delivered, truncation state and outcome. Nothing
// here is an estimate, and nothing is written that could carry SQL literals,
// query results, object definitions or credentials.
//
// The records go to stderr as JSON with component="usage", on a logger that
// the MCP logging/setLevel request does NOT control: a harness that lowers
// the security log level must still be able to count calls. Set
// MSSQL_USAGE_LOG=false to silence the stream entirely.
//
// This is a separate stream from the security log and does not change any
// tool result, so it does not affect the agent contract v1 result shape.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Tool call outcomes.
const (
	UsageStatusSuccess   = "success"
	UsageStatusToolError = "tool_error"
	UsageStatusRPCError  = "rpc_error"
	UsageStatusTimeout   = "timeout"
)

// UsageRecord is one counted tool call.
//
// Rows and RowTruncated are pointers so "not observable" stays null instead
// of being reported as a zero. Only query_database exposes the row count and
// the truncation flag today; the catalog tools return text.
type UsageRecord struct {
	CallID          uint64
	Tool            string
	Method          string
	Status          string
	ElapsedMS       int64
	ResponseBytes   int
	TextBytes       int
	StructuredBytes int
	Rows            *int
	RowTruncated    *bool
	Alias           string
	ReadOnly        *bool
}

// UsageLogger writes usage records to a writer.
type UsageLogger struct {
	logger    *slog.Logger
	enabled   bool
	sessionID string
	calls     atomic.Uint64
}

// NewUsageLogger returns a usage logger writing to stderr.
func NewUsageLogger(enabled bool) *UsageLogger {
	return newUsageLogger(enabled, os.Stderr)
}

// newUsageLogger returns a usage logger writing to the given writer.
func newUsageLogger(enabled bool, w io.Writer) *UsageLogger {
	level := &slog.LevelVar{}
	level.Set(slog.LevelInfo)
	return &UsageLogger{
		logger: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})).
			With(slog.String("component", "usage")),
		enabled:   enabled,
		sessionID: newSessionID(),
	}
}

// newSessionID returns an opaque identifier for this process.
//
// It separates concurrent server processes; it authenticates nothing, so a
// fallback to a timestamp is acceptable when the entropy source fails.
func newSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// SessionID returns the identifier of this session.
func (ul *UsageLogger) SessionID() string {
	if ul == nil {
		return ""
	}
	return ul.sessionID
}

// NextCallID returns the next per-session call identifier.
func (ul *UsageLogger) NextCallID() uint64 {
	if ul == nil {
		return 0
	}
	return ul.calls.Add(1)
}

// LogCall writes one counted record.
func (ul *UsageLogger) LogCall(rec UsageRecord) {
	if ul == nil || !ul.enabled {
		return
	}

	attrs := []any{
		slog.String("session_id", ul.sessionID),
		slog.Uint64("call_id", rec.CallID),
		slog.String("tool", rec.Tool),
		slog.String("method", rec.Method),
		slog.String("status", rec.Status),
		slog.Int64("elapsed_ms", rec.ElapsedMS),
		slog.Int("response_bytes", rec.ResponseBytes),
		slog.Int("text_bytes", rec.TextBytes),
	}
	if rec.StructuredBytes > 0 {
		attrs = append(attrs, slog.Int("structured_bytes", rec.StructuredBytes))
	}
	if rec.Rows != nil {
		attrs = append(attrs, slog.Int("rows", *rec.Rows))
	}
	if rec.RowTruncated != nil {
		attrs = append(attrs, slog.Bool("row_truncated", *rec.RowTruncated))
	}
	if rec.Alias != "" {
		attrs = append(attrs, slog.String("alias", rec.Alias))
	}
	if rec.ReadOnly != nil {
		attrs = append(attrs, slog.Bool("read_only", *rec.ReadOnly))
	}

	ul.logger.Info("tool call", attrs...)
}

// usageLogEnabled reports whether the usage stream is enabled.
func usageLogEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MSSQL_USAGE_LOG"))) {
	case "false", "0", "no", "off":
		return false
	default:
		return true
	}
}

// toolNameFromParams extracts the canonical tool name of a tools/call request.
// It returns an empty string for any other method.
func toolNameFromParams(method string, params interface{}) string {
	if method != "tools/call" {
		return ""
	}
	asMap, ok := params.(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := asMap["name"].(string)
	return name
}

// usageFromResponse derives the countable facts of a tool response.
//
// It returns the row count and the truncation flag when the tool exposes
// them (query_database, through its structured result), and nil when the
// tool does not report them.
func usageFromResponse(response *MCPResponse) (rows *int, truncated *bool, textBytes, structuredBytes int, isToolError bool) {
	if response == nil {
		return nil, nil, 0, 0, false
	}

	result, ok := response.Result.(CallToolResult)
	if !ok {
		return nil, nil, 0, 0, false
	}

	for _, item := range result.Content {
		textBytes += len(item.Text)
	}

	if result.StructuredContent != nil {
		if encoded, err := json.Marshal(result.StructuredContent); err == nil {
			structuredBytes = len(encoded)
		}
	}

	if queryResult, ok := result.StructuredContent.(QueryResult); ok {
		count := queryResult.RowCount
		flag := queryResult.Truncated
		rows = &count
		truncated = &flag
	}
	if envelope, ok := result.StructuredContent.(ResultEnvelope); ok {
		count := envelope.RowCount
		flag := envelope.Truncated
		rows = &count
		truncated = &flag
	}

	return rows, truncated, textBytes, structuredBytes, result.IsError
}

// usageStatus classifies a tool call.
func usageStatus(response *MCPResponse, timedOut bool, isToolError bool) string {
	switch {
	case timedOut:
		return UsageStatusTimeout
	case response != nil && response.Error != nil:
		return UsageStatusRPCError
	case isToolError:
		return UsageStatusToolError
	default:
		return UsageStatusSuccess
	}
}

// usageContext returns the active dynamic alias and the effective read-only
// posture for the usage record. The alias is empty in classic mode.
//
// The two locks are taken one after the other, never nested: getEffectiveConfig
// takes dynamicMu itself.
func (s *MCPMSSQLServer) usageContext() (string, *bool) {
	s.dynamicMu.RLock()
	alias := s.activeAlias
	s.dynamicMu.RUnlock()

	effective := s.getEffectiveConfig()
	readOnly := effective.readOnly
	return alias, &readOnly
}
