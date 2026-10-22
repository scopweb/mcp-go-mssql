package main

import (
	"regexp"
	"strconv"
	"strings"
)

// QueryStats is the structured statistics payload that accompanies a
// query_database response when the caller used the SET STATISTICS
// (TIME|IO) (ON|OFF) wrapper. When the wrapper is not used, this struct
// is still present in the response with Enabled=false so clients have a
// stable JSON shape.
type QueryStats struct {
	// Enabled is true if the caller wrapped the query with one or more
	// SET STATISTICS TIME/IO ON/OFF tokens (i.e. the parser detected a
	// valid wrapper around the base statement).
	Enabled bool `json:"enabled"`
	// TimeOn / IOOn report the net effect of the wrapper tokens. They
	// are false when the wrapper did not enable the corresponding
	// statistics (e.g. the wrapper used TIME OFF, or was for IO only).
	TimeOn bool `json:"time_on,omitempty"`
	IOOn   bool `json:"io_on,omitempty"`
	// Messages holds the raw informational messages emitted by SQL
	// Server during execution of the wrapped query. When Enabled is
	// false, this slice is present (and empty) so clients can iterate
	// without a nil check.
	Messages []string `json:"messages"`
	// Parsed is the numeric summary extracted from Messages. It
	// contains cpu_time_ms and elapsed_time_ms for SET STATISTICS TIME,
	// and scan_count_<table> / logical_reads_<table> /
	// physical_reads_<table> plus the aggregate logical_reads /
	// physical_reads for SET STATISTICS IO. The map is nil when no
	// fields were recognized.
	Parsed map[string]int `json:"parsed,omitempty"`
}

// ResultEnvelope carries the rows of a result and the facts of its
// truncation.
//
// Every tool that returns rows uses it, so a client reads the truncation
// flags the same way everywhere. The three truncations are separate on
// purpose: a result can be complete in rows and still have shortened cells,
// or be cut by the size budget before the row cap.
type ResultEnvelope struct {
	// Rows holds the result-set rows. Each row is a map of column name
	// to value, with []byte values converted to strings.
	Rows []map[string]interface{} `json:"rows"`
	// RowCount is the number of rows actually returned. It equals
	// len(Rows).
	RowCount int `json:"row_count"`
	// Truncated keeps the meaning of contract v1: the row cap was
	// reached, so the result is not complete. It is the same value as
	// RowsTruncated and stays for clients that only know this flag.
	Truncated bool `json:"truncated"`
	// RowsTruncated is the explicit name of Truncated.
	RowsTruncated bool `json:"rows_truncated"`
	// CellsTruncated is true when at least one cell was shortened by the
	// cell budget.
	CellsTruncated bool `json:"cells_truncated"`
	// ResponseTruncated is true when the response budget was reached and
	// the result stops there.
	ResponseTruncated bool `json:"response_truncated"`
	// TruncatedCells details the shortened cells. The list is capped, so
	// CellsTruncated is the authoritative signal that some cell was cut.
	TruncatedCells []CellTruncation `json:"truncated_cells,omitempty"`
	// Limits reports the budgets that were applied to this result.
	Limits AppliedLimits `json:"limits"`
}

// InspectAllResult is the structured result of inspect with detail=all: the
// four sections of the table plus the facts of the truncation, in the same
// envelope the other tools use.
type InspectAllResult struct {
	ResultEnvelope
	// Columns holds the columns of the table.
	Columns []map[string]interface{} `json:"columns"`
	// Indexes holds the indexes of the table, one row per index.
	Indexes []map[string]interface{} `json:"indexes"`
	// ForeignKeys holds the foreign keys of the table.
	ForeignKeys []map[string]interface{} `json:"foreign_keys"`
	// Dependencies holds the objects that reference the table.
	Dependencies []map[string]interface{} `json:"dependencies"`
	// CellsCut is the number of shortened cells of every section.
	CellsCut int `json:"cells_cut"`
}

// QueryResult is the wire-level result of executing a query via the
// MCP query_database tool. It supersedes the previous bare []map[string]interface{}
// shape so the response can carry per-statement execution statistics
// alongside the row data.
type QueryResult struct {
	ResultEnvelope
	// Stats holds the SET STATISTICS payload, always present.
	Stats QueryStats `json:"stats"`
}

// timeStatsRe extracts the CPU and elapsed time values from a SET
// STATISTICS TIME message. The SQL Server English-locale shape is:
//
//	SQL Server Execution Times:
//	   CPU time = 16 ms,  elapsed time = 22 ms.
//
// The regex is permissive: it tolerates the absence of a comma between
// the two halves and any amount of internal whitespace.
var timeStatsRe = regexp.MustCompile(`(?i)CPU time\s*=\s*(\d+)\s*ms.*?elapsed time\s*=\s*(\d+)\s*ms`)

// ioStatsRe extracts the per-table scan / read counts from a SET
// STATISTICS IO message. The SQL Server English-locale shape is:
//
//	Table 'orders'. Scan count 1, logical reads 4, physical reads 0,
//	read-ahead reads 0, lob logical reads 0, lob physical reads 0,
//	lob read-ahead reads 0.
var ioStatsRe = regexp.MustCompile(`(?i)Table\s+'([^']+)'\.\s*Scan count\s+(\d+),\s*logical reads\s+(\d+),\s*physical reads\s+(\d+)`)

// parseStatsMessages extracts integer fields out of the raw SQL Server
// informational message lines captured during execution of a wrapped
// query. Recognized shapes:
//
//   - " SQL Server Execution Times:" / "   CPU time = N ms,  elapsed time = M ms."
//     → {cpu_time_ms: N, elapsed_time_ms: M}
//
//   - "Table 'name'. Scan count S, logical reads L, physical reads P, ..."
//     → {scan_count_<name>: S, logical_reads_<name>: L,
//     physical_reads_<name>: P, logical_reads: L (aggregate),
//     physical_reads: P (aggregate)} per line.
//
// Returns an empty (non-nil) map when no fields were recognized. The
// caller may want to set Parsed to nil in the JSON response when the
// map is empty, which is done via the `omitempty` tag on QueryStats.
func parseStatsMessages(lines []string) map[string]int {
	out := map[string]int{}
	for _, line := range lines {
		if m := timeStatsRe.FindStringSubmatch(line); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil {
				out["cpu_time_ms"] = v
			}
			if v, err := strconv.Atoi(m[2]); err == nil {
				out["elapsed_time_ms"] = v
			}
			continue
		}
		if m := ioStatsRe.FindStringSubmatch(line); m != nil {
			table := sanitizeTableName(m[1])
			scan, _ := strconv.Atoi(m[2])
			logical, _ := strconv.Atoi(m[3])
			physical, _ := strconv.Atoi(m[4])
			out["scan_count_"+table] = scan
			out["logical_reads_"+table] = logical
			out["physical_reads_"+table] = physical
			// Aggregate totals across all tables in the IO report.
			// The aggregate fields accumulate (Go's zero-value for
			// missing keys is 0), so the final value is the true sum
			// across every Table '...' line, not the last-seen value.
			out["logical_reads"] += logical
			out["physical_reads"] += physical
		}
	}
	return out
}

// sanitizeTableName strips characters that are inconvenient for map
// keys (spaces, dots, brackets). Bracketed identifiers like [dbo].[x]
// are reduced to dbo_x. This keeps the Parsed map safe for clients
// that use the field names as JSON keys.
func sanitizeTableName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
