package main

import (
	"context"
	"fmt"
	"time"
)

// Prepared query execution and the row cap that keeps tool responses bounded.

// maxQueryRows limits the number of rows returned by any query to prevent token overflow.
const maxQueryRows = 500

// executeSecureQuery runs a validated, prepared query and returns up
// to maxQueryRows rows plus, when the query is a SET STATISTICS
// (TIME|IO) (ON|OFF) wrapper, the captured informational messages
// from SQL Server. The wrapper detection is a no-op for non-wrapper
// queries; in that case the messages slice is nil.
//
// If the result is truncated, the last element contains a
// "_truncated" warning key.
//
// Backwards compatibility: the function returns three values
// ([]map[string]interface{}, []string, error). Callers that ignore the
// stats messages can assign them to `_`.
func (s *MCPMSSQLServer) executeSecureQuery(ctx context.Context, query string, args ...interface{}) ([]map[string]interface{}, []string, error) {
	db := s.getDB()
	if db == nil {
		return nil, nil, fmt.Errorf("database not connected")
	}

	if err := s.validateBasicInput(query); err != nil {
		return nil, nil, err
	}

	// Detect a SET STATISTICS wrapper. The wrapper is the only
	// multi-statement form allowed in read-only mode and is a pure
	// observability statement — it neither reads nor writes user
	// data. We unwrap it once here and use the base statement for
	// every security check (validateReadOnlyQuery already recurses
	// into the base itself, but validateTablePermissions does NOT
	// unwrap — if we passed it the raw wrapped query, extractOperation
	// would not see the DELETE/UPDATE/INSERT prefix and would
	// misclassify the operation as SELECT, letting the modification
	// through the whitelist gate).
	wrapper, _ := parseStatisticsWrapper(query)
	permCheckQuery := query
	if wrapper != nil {
		permCheckQuery = wrapper.BaseStatement
	}

	// Validate read-only restrictions (the recursion inside
	// validateReadOnlyQuery re-unwraps the wrapper for its own check;
	// passing the base here is also fine because the wrapper branch
	// would return ErrNotAWrapper for a plain base statement and the
	// existing SELECT/prefix checks would run as normal).
	if err := s.validateReadOnlyQuery(query); err != nil {
		s.secLogger.Printf("Read-only violation blocked: %s", err)
		return nil, nil, err
	}

	// Validate granular table permissions on the BASE statement
	// (unwrapped) so the whitelist gate correctly classifies the
	// operation and the referenced tables.
	if err := s.validateTablePermissions(permCheckQuery); err != nil {
		s.secLogger.Printf("Permission violation blocked: %s", err)
		return nil, nil, err
	}

	// If a wrapper was detected, install a per-call message sink so we
	// can capture the informational messages SQL Server emits. The
	// wrapper is sent as a single batch via db.PrepareContext +
	// stmt.QueryContext; the driver delivers the messages to the sink
	// via the sqlexp.ReturnMessage queue.
	var sink *messageSink
	if wrapper != nil {
		sink = newMessageSink()
		sink.start(ctx)
		args = append(args, sink.rm)
		defer sink.stop(250 * time.Millisecond)
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		if s.devMode {
			s.secLogger.Printf("Failed to prepare statement: %v", err)
			return nil, nil, fmt.Errorf("query preparation failed: %v", err)
		}
		s.secLogger.Printf("Failed to prepare statement: query preparation error")
		return nil, nil, fmt.Errorf("query preparation failed: check SQL syntax, table/column names, and permissions. Use explore tool to verify table exists")
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		if s.devMode {
			s.secLogger.Printf("Failed to execute query: %v", err)
			return nil, nil, fmt.Errorf("query execution failed: %v", err)
		}
		s.secLogger.Printf("Failed to execute query: execution error")
		return nil, nil, fmt.Errorf("query execution failed: the query syntax is valid but execution was rejected by the server. Check permissions and data constraints")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var results []map[string]interface{}
	rowCount := 0
	truncated := false
	for rows.Next() {
		if rowCount >= maxQueryRows {
			truncated = true
			break
		}
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading query results: %w", err)
	}

	if truncated {
		results = append(results, map[string]interface{}{
			"_truncated": fmt.Sprintf("Results limited to %d rows. Use WHERE or TOP to narrow the query.", maxQueryRows),
		})
	}

	var statsMsgs []string
	if wrapper != nil && sink != nil {
		// Give the drain goroutine a moment to flush any messages
		// that arrived during rows.Next().
		sink.stop(100 * time.Millisecond)
		statsMsgs = sink.linesSnapshot()
	}

	return results, statsMsgs, nil
}
