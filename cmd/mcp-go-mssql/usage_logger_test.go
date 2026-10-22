package main

// Tests for the usage accounting records.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestUsageFromResponse checks which facts are observable per tool.
func TestUsageFromResponse(t *testing.T) {
	rows := 3
	truncated := true
	queryResult := QueryResult{
		Rows:      []map[string]interface{}{{"id": 1}},
		RowCount:  rows,
		Truncated: truncated,
	}

	response := &MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: CallToolResult{
			Content:           []ContentItem{{Type: "text", Text: "Query executed successfully"}},
			StructuredContent: queryResult,
		},
	}

	gotRows, gotTruncated, gotCells, gotResponse, textBytes, structuredBytes, isToolError := usageFromResponse(response)
	if gotRows == nil || *gotRows != rows {
		t.Errorf("rows = %v, want %d", gotRows, rows)
	}
	if gotTruncated == nil || *gotTruncated != truncated {
		t.Errorf("row_truncated = %v, want %v", gotTruncated, truncated)
	}
	if gotCells == nil || *gotCells {
		t.Errorf("cells_truncated = %v, want false", gotCells)
	}
	if gotResponse == nil || *gotResponse {
		t.Errorf("response_truncated = %v, want false", gotResponse)
	}
	if textBytes != len("Query executed successfully") {
		t.Errorf("text_bytes = %d, want %d", textBytes, len("Query executed successfully"))
	}
	if structuredBytes <= 0 {
		t.Error("structured_bytes should be measured for a structured result")
	}
	if isToolError {
		t.Error("a successful call must not be reported as a tool error")
	}
}

// TestUsageFromResponseWithoutRowFacts checks that a tool that does not expose
// rows reports them as unknown instead of zero.
func TestUsageFromResponseWithoutRowFacts(t *testing.T) {
	response := &MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  CallToolResult{Content: []ContentItem{{Type: "text", Text: "Tables found"}}},
	}

	rows, truncated, _, _, _, _, _ := usageFromResponse(response)
	if rows != nil {
		t.Errorf("rows = %v, want nil (not observable)", *rows)
	}
	if truncated != nil {
		t.Errorf("row_truncated = %v, want nil (not observable)", *truncated)
	}
}

// TestUsageStatus classifies every outcome of a tool call.
func TestUsageStatus(t *testing.T) {
	toolError := &MCPResponse{Result: CallToolResult{IsError: true}}
	rpcError := &MCPResponse{Error: &MCPError{Code: -32602, Message: "Unknown tool"}}

	if got := usageStatus(&MCPResponse{}, false, false); got != UsageStatusSuccess {
		t.Errorf("status = %q, want %q", got, UsageStatusSuccess)
	}
	if got := usageStatus(toolError, false, true); got != UsageStatusToolError {
		t.Errorf("status = %q, want %q", got, UsageStatusToolError)
	}
	if got := usageStatus(rpcError, false, false); got != UsageStatusRPCError {
		t.Errorf("status = %q, want %q", got, UsageStatusRPCError)
	}
	if got := usageStatus(rpcError, true, false); got != UsageStatusTimeout {
		t.Errorf("status = %q, want %q", got, UsageStatusTimeout)
	}
}

// TestToolNameFromParams extracts the canonical tool name of a tools/call.
func TestToolNameFromParams(t *testing.T) {
	params := map[string]interface{}{"name": "query_database", "arguments": map[string]interface{}{}}
	if got := toolNameFromParams("tools/call", params); got != "query_database" {
		t.Errorf("tool = %q, want %q", got, "query_database")
	}
	if got := toolNameFromParams("initialize", params); got != "" {
		t.Errorf("tool = %q, want empty for a non tool/call method", got)
	}
	if got := toolNameFromParams("tools/call", "not-an-object"); got != "" {
		t.Errorf("tool = %q, want empty for malformed params", got)
	}
}

// TestUsageLoggerWritesCountedRecord checks the shape of the emitted record
// and that it carries no request payload.
func TestUsageLoggerWritesCountedRecord(t *testing.T) {
	var out bytes.Buffer
	logger := newUsageLogger(true, &out)

	rows := 500
	truncated := true
	logger.LogCall(UsageRecord{
		CallID:          7,
		Tool:            "query_database",
		Method:          "tools/call",
		Status:          UsageStatusSuccess,
		ElapsedMS:       42,
		ResponseBytes:   1234,
		TextBytes:       1000,
		StructuredBytes: 234,
		Rows:            &rows,
		RowTruncated:    &truncated,
	})

	var record map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("usage record is not valid JSON: %v\n%s", err, out.String())
	}

	expected := map[string]interface{}{
		"component":      "usage",
		"call_id":        float64(7),
		"tool":           "query_database",
		"method":         "tools/call",
		"status":         UsageStatusSuccess,
		"elapsed_ms":     float64(42),
		"response_bytes": float64(1234),
		"text_bytes":     float64(1000),
		"rows":           float64(500),
		"row_truncated":  true,
	}
	for key, want := range expected {
		got, ok := record[key]
		if !ok {
			t.Errorf("usage record is missing %q", key)
			continue
		}
		if got != want {
			t.Errorf("usage record %q = %v, want %v", key, got, want)
		}
	}

	if _, ok := record["session_id"]; !ok {
		t.Error("usage record is missing session_id")
	}
	for forbidden := range map[string]bool{"query": true, "arguments": true, "sql": true, "password": true} {
		if _, ok := record[forbidden]; ok {
			t.Errorf("usage record must not carry %q", forbidden)
		}
	}
	if strings.Contains(out.String(), "SELECT") {
		t.Error("usage record must not carry SQL literals")
	}
}

// TestUsageLoggerCarriesTheTruncationFlags verifies that the usage record
// carries the three truncations, so a harness can tell a complete answer from
// a cut one.
func TestUsageLoggerCarriesTheTruncationFlags(t *testing.T) {
	response := &MCPResponse{
		Result: CallToolResult{
			Content: []ContentItem{{Type: "text", Text: "x"}},
			StructuredContent: QueryResult{
				ResultEnvelope: ResultEnvelope{
					RowCount:          5,
					Truncated:         false,
					RowsTruncated:     false,
					CellsTruncated:    true,
					ResponseTruncated: true,
				},
			},
		},
	}

	rows, rowTruncated, cellsTruncated, responseTruncated, _, _, _ := usageFromResponse(response)
	if rows == nil || *rows != 5 {
		t.Errorf("rows = %v, want 5", rows)
	}
	if rowTruncated == nil || *rowTruncated {
		t.Errorf("row_truncated = %v, want false", rowTruncated)
	}
	if cellsTruncated == nil || !*cellsTruncated {
		t.Errorf("cells_truncated = %v, want true", cellsTruncated)
	}
	if responseTruncated == nil || !*responseTruncated {
		t.Errorf("response_truncated = %v, want true", responseTruncated)
	}

	var out bytes.Buffer
	logger := newUsageLogger(true, &out)
	logger.LogCall(UsageRecord{
		Tool:              "query_database",
		Status:            UsageStatusSuccess,
		Rows:              rows,
		RowTruncated:      rowTruncated,
		CellsTruncated:    cellsTruncated,
		ResponseTruncated: responseTruncated,
	})

	var record map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("the usage record is not valid JSON: %v", err)
	}
	if record["cells_truncated"] != true {
		t.Errorf("record = %v, want cells_truncated true", record)
	}
	if record["response_truncated"] != true {
		t.Errorf("record = %v, want response_truncated true", record)
	}
}
func TestUsageLoggerDisabledWritesNothing(t *testing.T) {
	var out bytes.Buffer
	logger := newUsageLogger(false, &out)
	logger.LogCall(UsageRecord{Tool: "explore", Status: UsageStatusSuccess})
	if out.Len() != 0 {
		t.Errorf("disabled usage logger wrote %q", out.String())
	}
}

// TestUsageLoggerOmitsUnknownRowFacts verifies that unobservable facts are
// absent from the record instead of reported as zero.
func TestUsageLoggerOmitsUnknownRowFacts(t *testing.T) {
	var out bytes.Buffer
	logger := newUsageLogger(true, &out)
	logger.LogCall(UsageRecord{Tool: "explore", Status: UsageStatusSuccess})

	var record map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("usage record is not valid JSON: %v", err)
	}
	if _, ok := record["rows"]; ok {
		t.Error("rows must be absent when the tool does not expose them")
	}
	if _, ok := record["row_truncated"]; ok {
		t.Error("row_truncated must be absent when the tool does not expose it")
	}
}

// TestUsageSessionID verifies the session identifier is opaque and unique.
func TestUsageSessionID(t *testing.T) {
	first := newSessionID()
	second := newSessionID()

	if len(first) != 16 {
		t.Errorf("session id %q has %d chars, want 16 hex chars", first, len(first))
	}
	for _, r := range first {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Errorf("session id %q is not hexadecimal", first)
			break
		}
	}
	if first == second {
		t.Error("two sessions must not share the same identifier")
	}
}

// TestUsageCallIDsAreSequential verifies the per-session call counter.
func TestUsageCallIDsAreSequential(t *testing.T) {
	logger := newUsageLogger(false, &bytes.Buffer{})
	if got := logger.NextCallID(); got != 1 {
		t.Errorf("first call id = %d, want 1", got)
	}
	if got := logger.NextCallID(); got != 2 {
		t.Errorf("second call id = %d, want 2", got)
	}
}

// TestUsageContextReportsPosture verifies the alias and read-only posture.
func TestUsageContextReportsPosture(t *testing.T) {
	server := &MCPMSSQLServer{config: serverConfig{readOnly: true}}
	alias, readOnly := server.usageContext()
	if alias != "" {
		t.Errorf("alias = %q, want empty in classic mode", alias)
	}
	if readOnly == nil || !*readOnly {
		t.Error("read_only should report the configured posture")
	}

	dynamic := &MCPMSSQLServer{
		isDynamic:      true,
		config:         serverConfig{readOnly: true},
		dynamicAliases: map[string]DynamicAlias{"CRM": {Alias: "CRM", ReadOnly: false}},
		activeAlias:    "CRM",
	}
	alias, readOnly = dynamic.usageContext()
	if alias != "CRM" {
		t.Errorf("alias = %q, want CRM", alias)
	}
	if readOnly == nil || *readOnly {
		t.Error("read_only should follow the active alias posture")
	}
}

// TestUsageLogEnabled verifies the opt-out switch.
func TestUsageLogEnabled(t *testing.T) {
	t.Setenv("MSSQL_USAGE_LOG", "")
	if !usageLogEnabled() {
		t.Error("usage logging should be enabled by default")
	}
	t.Setenv("MSSQL_USAGE_LOG", "false")
	if usageLogEnabled() {
		t.Error("MSSQL_USAGE_LOG=false must disable the usage stream")
	}
	t.Setenv("MSSQL_USAGE_LOG", "OFF")
	if usageLogEnabled() {
		t.Error("MSSQL_USAGE_LOG=OFF must disable the usage stream")
	}
}
