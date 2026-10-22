package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// query_database and explain_query.

// toolQueryDatabase handles the query_database tool.
func (s *MCPMSSQLServer) toolQueryDatabase(id interface{}, params CallToolParams) *MCPResponse {
	if s.getDB() == nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps.",
					},
				},
				IsError: true,
			},
		}
	}

	query, ok := params.Arguments["query"].(string)
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: "Error: Missing or invalid 'query' parameter",
					},
				},
				IsError: true,
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Optional response budgets. They are opt-in and never rewrite the SQL
	// of the caller: the caps are applied in Go while the rows are read.
	limits, err := parseQueryLimits(params.Arguments)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Query Error: %v", err),
					},
				},
				IsError: true,
			},
		}
	}

	outcome, err := s.executeSecureQueryWithLimits(ctx, query, limits)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Query Error: %v", err),
					},
				},
				IsError: true,
			},
		}
	}
	results := outcome.Rows
	statsMsgs := outcome.StatsMessages

	// Build the Stats payload. When statsMsgs is non-nil the query
	// was a SET STATISTICS wrapper; the parser detection ran inside
	// executeSecureQuery and installed the per-call message sink.
	// Otherwise Enabled=false so clients see a stable JSON shape.
	stats := QueryStats{Enabled: false, Messages: []string{}}
	if wrapper, _ := parseStatisticsWrapper(query); wrapper != nil {
		parsed := parseStatsMessages(statsMsgs)
		stats = QueryStats{
			Enabled:  true,
			TimeOn:   wrapper.TimeOn,
			IOOn:     wrapper.IOOn,
			Messages: statsMsgs,
			Parsed:   parsed,
		}
		if len(parsed) == 0 {
			stats.Parsed = nil
		}
	}

	envelope := ResultEnvelope{
		RowCount:          len(results),
		Truncated:         outcome.RowsTruncated,
		RowsTruncated:     outcome.RowsTruncated,
		CellsTruncated:    outcome.CellsTruncated,
		ResponseTruncated: outcome.ResponseTruncated,
		TruncatedCells:    outcome.TruncatedCells,
		Limits:            outcome.Limits,
	}

	// The format is opt-in: the default keeps the shape of contract v1 and
	// the columnar format is only used when the caller asks for it.
	format, _ := params.Arguments["format"].(string)
	if format == "" {
		format = FormatMap
	}

	var structured interface{}
	switch format {
	case FormatColumnar:
		if duplicates := duplicateColumns(outcome.Columns); len(duplicates) > 0 {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: CallToolResult{
					Content: []ContentItem{
						{
							Type: "text",
							Text: fmt.Sprintf("Query Error: the columnar format needs unique column names, but the result repeats %s. Use the default format.", strings.Join(duplicates, ", ")),
						},
					},
					IsError: true,
				},
			}
		}
		structured = ColumnarResult{
			ResultEnvelope: envelope,
			Format:         FormatColumnar,
			Columns:        outcome.Columns,
			Rows:           columnarRows(results, outcome.Columns),
			Stats:          stats,
		}
	default:
		structured = QueryResult{
			ResultEnvelope: envelope,
			Rows:           results,
			Stats:          stats,
		}
	}

	// Format result as JSON
	resultBytes, err := json.MarshalIndent(structured, "", "  ")
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{
					{
						Type: "text",
						Text: fmt.Sprintf("Error formatting results: %v", err),
					},
				},
				IsError: true,
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("Query executed successfully. Results:\n%s", string(resultBytes)),
				},
			},
			StructuredContent: structured,
		},
	}
}

// toolExplainQuery handles the explain_query tool.
func (s *MCPMSSQLServer) toolExplainQuery(id interface{}, params CallToolParams) *MCPResponse {
	if s.getDB() == nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: Database not connected. Call the get_database_info tool to see current configuration, diagnose the problem, and get specific troubleshooting steps."}},
				IsError: true,
			},
		}
	}

	query, ok := params.Arguments["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: Missing or invalid 'query' parameter"}},
				IsError: true,
			},
		}
	}

	// Apply the same baseline input guards as every other query path, so
	// explain_query is not an exception to the uniform defense (M5): size
	// limits and the stacked-statement check. Although SHOWPLAN compiles
	// without executing the batch, "SELECT 1; DROP TABLE x" starts with
	// SELECT and would otherwise slip past the prefix-based operation
	// detector below.
	if err := s.validateBasicInput(query); err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: " + err.Error()}},
				IsError: true,
			},
		}
	}
	if hasMultipleStatements(query) {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: explain_query does not allow multiple SQL statements in a single request"}},
				IsError: true,
			},
		}
	}

	// Only allow SELECT queries for safety (always enforced, regardless of MSSQL_READ_ONLY)
	if op := s.extractOperation(query); op != "SELECT" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: explain_query only accepts SELECT queries, got: " + op}},
				IsError: true,
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a dedicated connection so SET SHOWPLAN_TEXT applies only to this query
	conn, err := s.getDB().Conn(ctx)
	if err != nil {
		connErrMsg := "Error acquiring connection"
		if s.devMode {
			connErrMsg += ": " + err.Error()
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: connErrMsg}},
				IsError: true,
			},
		}
	}
	defer conn.Close()

	// Enable showplan (does not execute the query, only returns the plan)
	if _, err := conn.ExecContext(ctx, "SET SHOWPLAN_TEXT ON"); err != nil {
		showplanErrMsg := "Error enabling SHOWPLAN"
		if s.devMode {
			showplanErrMsg += ": " + err.Error()
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: showplanErrMsg}},
				IsError: true,
			},
		}
	}

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		resetShowplanOrDiscard(ctx, conn)
		planErrMsg := "Error getting execution plan"
		if s.devMode {
			planErrMsg += ": " + err.Error()
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: planErrMsg}},
				IsError: true,
			},
		}
	}

	// SHOWPLAN_TEXT returns TWO result sets per statement: first the statement text, then
	// the plan. Read ALL of them: reading only the first returned the caller's own SQL,
	// and leaving the rest unread made the next command on this connection (SHOWPLAN OFF)
	// run against a half-consumed TDS stream - the likely cause of the server hang.
	var planLines []string
	for {
		for rows.Next() {
			var line sql.NullString
			if err := rows.Scan(&line); err == nil && line.Valid {
				planLines = append(planLines, line.String)
			}
		}
		if !rows.NextResultSet() {
			break
		}
	}
	// Close explicitly BEFORE issuing another command on the same connection
	// (a deferred Close ran only after SHOWPLAN OFF).
	_ = rows.Close()

	resetShowplanOrDiscard(ctx, conn)

	if len(planLines) == 0 {
		planLines = []string{"(no plan returned — query may be too simple or unsupported)"}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: "Execution plan:\n\n" + strings.Join(planLines, "\n"),
				},
			},
		},
	}

	// === Dynamic multi-connection tools (only reachable when s.isDynamic) ===
	// When !s.isDynamic these cases are unreachable because the tools are not
	// advertised in tools/list, but we keep cheap runtime guards for safety.
}

// resetShowplanOrDiscard turns SHOWPLAN_TEXT off on a dedicated connection before it
// goes back to the pool. If that fails (expired ctx, broken stream...) the connection is
// DISCARDED instead: a pooled connection left in SHOWPLAN mode would make a later
// query_database return an execution plan instead of data, silently.
func resetShowplanOrDiscard(ctx context.Context, conn *sql.Conn) {
	if _, err := conn.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF"); err != nil {
		// Returning driver.ErrBadConn from Raw makes database/sql close the connection
		// on conn.Close() instead of returning it to the pool.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
}
