package main

import (
	"context"
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

	results, statsMsgs, err := s.executeSecureQuery(ctx, query)
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

	// Strip the legacy _truncated sentinel row from the results;
	// its information is now carried by Truncated bool.
	rowCount := len(results)
	truncated := false
	if rowCount > 0 {
		if _, ok := results[rowCount-1]["_truncated"]; ok {
			truncated = true
			results = results[:rowCount-1]
			rowCount--
		}
	}

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

	result := QueryResult{
		Rows:      results,
		RowCount:  rowCount,
		Truncated: truncated,
		Stats:     stats,
	}

	// Format result as JSON
	resultBytes, err := json.MarshalIndent(result, "", "  ")
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
			StructuredContent: result,
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
		_, _ = conn.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF") // #nosec G104 - best-effort cleanup
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
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err == nil {
			planLines = append(planLines, line)
		}
	}
	_, _ = conn.ExecContext(ctx, "SET SHOWPLAN_TEXT OFF") // #nosec G104 - best-effort cleanup

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
