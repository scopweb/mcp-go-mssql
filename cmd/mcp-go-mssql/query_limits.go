package main

// Response budgets for tool results.
//
// A result set can be bounded on three axes: rows (the long standing cap of
// maxQueryRows), the size of each cell and the approximate size of the whole
// response. The cell and response budgets are opt-in: without them a result
// keeps exactly the behaviour of agent contract v1.
//
// The budgets are applied in Go, after the rows are read. The SQL of the
// caller is never rewritten: no TOP is injected and no predicate is added.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Floors of the optional budgets. A budget below them could not even carry
// its own flags, so it is rejected instead of silently producing an empty
// result.
const (
	minCellLimitBytes     = 16
	minResponseLimitBytes = 512
)

// maxTruncatedCellsListed caps the detail list of shortened cells, so the
// detail of a truncation cannot grow as much as the data it describes.
const maxTruncatedCellsListed = 50

// AppliedLimits reports the budgets that were applied to a result.
type AppliedLimits struct {
	// MaxRows is the row cap of the server. It is always applied.
	MaxRows int `json:"max_rows"`
	// MaxCellBytes is the cap of a single cell, or null when there is none.
	MaxCellBytes *int `json:"max_cell_bytes"`
	// MaxResponseBytes is the approximate cap of the whole result, or null
	// when there is none.
	MaxResponseBytes *int `json:"max_response_bytes"`
}

// CellTruncation records a cell that was shortened.
type CellTruncation struct {
	Row           int    `json:"row"`
	Column        string `json:"column"`
	OriginalBytes int    `json:"original_bytes"`
}

// queryLimits are the optional budgets of one execution.
type queryLimits struct {
	MaxCellBytes     int
	MaxResponseBytes int
}

// appliedLimits returns the budgets to report to the caller.
func (l queryLimits) appliedLimits() AppliedLimits {
	applied := AppliedLimits{MaxRows: maxQueryRows}
	if l.MaxCellBytes > 0 {
		limit := l.MaxCellBytes
		applied.MaxCellBytes = &limit
	}
	if l.MaxResponseBytes > 0 {
		limit := l.MaxResponseBytes
		applied.MaxResponseBytes = &limit
	}
	return applied
}

// parseQueryLimits reads the optional budgets of a tool call.
//
// A budget that is not a number, or that is below its floor, is rejected with
// an explicit error: a caller that asks for an impossible budget has to learn
// the limit instead of receiving an empty result.
func parseQueryLimits(arguments map[string]interface{}) (queryLimits, error) {
	limits := queryLimits{}

	cellBytes, err := limitArgument(arguments, "max_cell_bytes")
	if err != nil {
		return limits, err
	}
	if cellBytes > 0 && cellBytes < minCellLimitBytes {
		return limits, fmt.Errorf("max_cell_bytes must be at least %d", minCellLimitBytes)
	}
	limits.MaxCellBytes = cellBytes

	responseBytes, err := limitArgument(arguments, "max_response_bytes")
	if err != nil {
		return limits, err
	}
	if responseBytes > 0 && responseBytes < minResponseLimitBytes {
		return limits, fmt.Errorf("max_response_bytes must be at least %d", minResponseLimitBytes)
	}
	limits.MaxResponseBytes = responseBytes

	return limits, nil
}

// limitArgument reads one optional numeric argument.
func limitArgument(arguments map[string]interface{}, name string) (int, error) {
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return 0, nil
	}
	switch typed := raw.(type) {
	case float64:
		return int(typed), nil
	case int:
		return typed, nil
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer", name)
		}
		return int(parsed), nil
	default:
		return 0, fmt.Errorf("%s must be an integer", name)
	}
}

// cellMarker is appended to a shortened cell so the agent can see that the
// value is not complete.
func cellMarker(totalBytes int) string {
	return fmt.Sprintf(" [truncated: %d bytes total]", totalBytes)
}

// shortenCell keeps a prefix of the value and marks it as shortened.
//
// The prefix is cut on a rune boundary, so a multi byte character is never
// split. The result is empty when the value has no valid text at all, which
// is the normal case for binary columns: the marker still reports the size.
func shortenCell(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	prefix := trimToRuneBoundary(value, maxBytes)
	return prefix + cellMarker(len(value)), true
}

// trimToRuneBoundary cuts the string to at most maxBytes without splitting a
// rune.
func trimToRuneBoundary(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	cut := value[:maxBytes]
	for len(cut) > 0 {
		rune, size := utf8.DecodeLastRuneInString(cut)
		if rune != utf8.RuneError || size > 1 {
			return cut
		}
		cut = cut[:len(cut)-1]
	}
	return ""
}

// escapedJSONLen returns the length of the JSON encoding of a string.
//
// It is an estimate, not an exact count: it accounts for the characters that
// JSON escapes. A byte that is not valid UTF-8 is encoded by encoding/json
// as the six character escape, so it is counted as such.
func escapedJSONLen(value string) int {
	size := len(value) + 2 // the surrounding quotes
	for _, rune := range value {
		switch rune {
		case '"', '\\':
			// The escape adds the backslash.
			size++
		case '\n', '\r', '\t':
			// The escape is two characters.
			size++
		case utf8.RuneError:
			// An invalid byte is encoded by encoding/json as the six
			// character escape sequence, so it costs five more bytes.
			size += 5
		default:
			if rune < 0x20 {
				// \u00XX is six characters.
				size += 5
			}
		}
	}
	return size
}

// estimateCellBytes estimates the JSON size of a cell value.
func estimateCellBytes(value interface{}) int {
	switch typed := value.(type) {
	case nil:
		return 4 // null
	case string:
		return escapedJSONLen(typed)
	case bool:
		if typed {
			return 4 // true
		}
		return 5 // false
	case int:
		return len(strconv.Itoa(typed))
	case int64:
		return len(strconv.FormatInt(typed, 10))
	case float64:
		return len(strconv.FormatFloat(typed, 'f', -1, 64))
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return 8
		}
		return len(encoded)
	}
}

// estimateRowBytes estimates the JSON size of a row, including its keys.
func estimateRowBytes(row map[string]interface{}) int {
	size := 2 // the braces
	for name, value := range row {
		size += len(name) + 3 // "name":
		size += estimateCellBytes(value)
		size++ // the comma
	}
	return size
}

// rowBudget applies the optional budgets while the rows are read.
type rowBudget struct {
	limits            queryLimits
	totalBytes        int
	cellsTruncated    bool
	responseTruncated bool
	truncatedCells    []CellTruncation
	cellsCut          int
}

// addRow applies the cell limit to a row and reports whether the row fits in
// the response budget.
//
// A row that does not fit is not returned: the result stops there and the
// response is reported as truncated. The rows are never rewritten by the
// server, only the values that are too long are shortened.
func (b *rowBudget) addRow(row map[string]interface{}, rowIndex int) bool {
	if b.limits.MaxCellBytes > 0 {
		// The columns are sorted so the detail of a truncation does not
		// depend on the order of the map.
		columns := make([]string, 0, len(row))
		for column := range row {
			columns = append(columns, column)
		}
		sort.Strings(columns)

		for _, column := range columns {
			text, ok := row[column].(string)
			if !ok {
				continue
			}
			shortened, cut := shortenCell(text, b.limits.MaxCellBytes)
			if !cut {
				continue
			}
			row[column] = shortened
			b.cellsTruncated = true
			b.cellsCut++
			if len(b.truncatedCells) < maxTruncatedCellsListed {
				b.truncatedCells = append(b.truncatedCells, CellTruncation{
					Row:           rowIndex,
					Column:        column,
					OriginalBytes: len(text),
				})
			}
		}
	}

	if b.limits.MaxResponseBytes > 0 {
		cost := estimateRowBytes(row)
		if b.totalBytes+cost > b.limits.MaxResponseBytes {
			b.responseTruncated = true
			return false
		}
		b.totalBytes += cost
	}
	return true
}

// withTruncationSentinel appends the legacy truncation row to a result set.
//
// The catalog tools have always reported a truncated result with this row, so
// it is kept for them. query_database reports the truncation with the
// explicit flags instead.
func withTruncationSentinel(rows []map[string]interface{}, rowsTruncated bool) []map[string]interface{} {
	if !rowsTruncated {
		return rows
	}
	return append(rows, map[string]interface{}{
		"_truncated": fmt.Sprintf("Results limited to %d rows. Use WHERE or TOP to narrow the query.", maxQueryRows),
	})
}

// truncationNotice describes a truncation for the text of the catalog tools.
func truncationNotice(rowsTruncated, cellsTruncated, responseTruncated bool, cellsCut int) string {
	if !rowsTruncated && !cellsTruncated && !responseTruncated {
		return ""
	}
	var parts []string
	if rowsTruncated {
		parts = append(parts, fmt.Sprintf("rows truncated at %d", maxQueryRows))
	}
	if cellsTruncated {
		parts = append(parts, fmt.Sprintf("%d cell(s) shortened", cellsCut))
	}
	if responseTruncated {
		parts = append(parts, "response size limit reached")
	}
	return "[truncated: " + strings.Join(parts, ", ") + "]"
}
