package main

// Tests for the response budgets: cell limit, response budget and the flags
// that report them. They need no database: every rule is a pure function.

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestQueryLimitsCannotBeDisabledOrRaised(t *testing.T) {
	for _, value := range []interface{}{float64(0), float64(-1), 16.5, math.NaN(), math.Inf(1), float64(1 << 40), -1, json.Number("0"), json.Number("999999999999999999999")} {
		if _, err := parseQueryLimits(map[string]interface{}{"max_cell_bytes": value}); err == nil {
			t.Errorf("accepted %v", value)
		}
	}
	if _, err := parseQueryLimits(map[string]interface{}{"max_cell_bytes": maxCellLimitBytes + 1}); err == nil {
		t.Fatal("cell ceiling bypassed")
	}
	if _, err := parseQueryLimits(map[string]interface{}{"max_response_bytes": maxResponseLimitBytes + 1}); err == nil {
		t.Fatal("response ceiling bypassed")
	}
}

func TestDefaultBudgetTruncatesLargeBytes(t *testing.T) {
	b := rowBudget{}
	row := map[string]interface{}{"blob": []byte(strings.Repeat("x", maxCellLimitBytes+100))}
	if !b.addRow(row, 0) || !b.cellsTruncated {
		t.Fatal("default cell ceiling not applied")
	}
	value := row["blob"].(string)
	if len(value) > maxCellLimitBytes+100 || !strings.Contains(value, "truncated:") {
		t.Fatal("missing truncation marker")
	}
	if b.truncatedCells[0].OriginalBytes != maxCellLimitBytes+100 {
		t.Fatal("incorrect original size")
	}
	for i := 1; i < 10; i++ {
		if !b.addRow(map[string]interface{}{"v": strings.Repeat("x", maxCellLimitBytes)}, i) {
			break
		}
	}
	if !b.responseTruncated {
		t.Fatal("default response ceiling not applied")
	}
}

// TestParseQueryLimits verifies the optional budgets of a tool call.
func TestParseQueryLimits(t *testing.T) {
	limits, err := parseQueryLimits(map[string]interface{}{})
	if err != nil {
		t.Fatalf("no arguments must not fail: %v", err)
	}
	if limits.MaxCellBytes != maxCellLimitBytes || limits.MaxResponseBytes != maxResponseLimitBytes {
		t.Errorf("limits = %+v, want server ceilings", limits)
	}

	limits, err = parseQueryLimits(map[string]interface{}{
		"max_cell_bytes":     float64(4096),
		"max_response_bytes": float64(262144),
	})
	if err != nil {
		t.Fatalf("valid budgets must be accepted: %v", err)
	}
	if limits.MaxCellBytes != 4096 || limits.MaxResponseBytes != 262144 {
		t.Errorf("limits = %+v, want the requested budgets", limits)
	}
}

// TestParseQueryLimitsRejectsBadValues verifies that an impossible budget is
// reported instead of silently producing an empty result.
func TestParseQueryLimitsRejectsBadValues(t *testing.T) {
	if _, err := parseQueryLimits(map[string]interface{}{"max_cell_bytes": float64(1)}); err == nil {
		t.Error("a cell budget below the floor must be rejected")
	}
	if _, err := parseQueryLimits(map[string]interface{}{"max_response_bytes": float64(10)}); err == nil {
		t.Error("a response budget below the floor must be rejected")
	}
	if _, err := parseQueryLimits(map[string]interface{}{"max_cell_bytes": "4096"}); err == nil {
		t.Error("a budget that is not a number must be rejected")
	}
}

// Internal callers cannot disable the server ceilings with zero-valued limits.
func TestAppliedLimitsReportServerCeilings(t *testing.T) {
	applied := queryLimits{}.appliedLimits()
	if applied.MaxRows != maxQueryRows {
		t.Errorf("max_rows = %d, want %d", applied.MaxRows, maxQueryRows)
	}
	if applied.MaxCellBytes == nil || applied.MaxResponseBytes == nil {
		t.Fatalf("limits = %+v, want mandatory budgets", applied)
	}

	encoded, err := json.Marshal(applied)
	if err != nil {
		t.Fatalf("cannot marshal the limits: %v", err)
	}
	if !strings.Contains(string(encoded), `"max_cell_bytes":1048576`) {
		t.Errorf("limits = %s, want the server max_cell_bytes", encoded)
	}
}

// TestShortenCell verifies the cell limit.
func TestShortenCell(t *testing.T) {
	if shortened, cut := shortenCell("short", 100); cut || shortened != "short" {
		t.Errorf("a value below the limit must not change: %q %v", shortened, cut)
	}

	shortened, cut := shortenCell("abcdefghij", 4)
	if !cut {
		t.Fatal("a value above the limit must be cut")
	}
	if !strings.HasPrefix(shortened, "abcd") {
		t.Errorf("shortened = %q, want the prefix", shortened)
	}
	if !strings.Contains(shortened, "10 bytes total") {
		t.Errorf("shortened = %q, want the original size", shortened)
	}
}

// TestShortenCellKeepsRuneBoundaries verifies that a multi byte character is
// never split.
func TestShortenCellKeepsRuneBoundaries(t *testing.T) {
	value := "áéí" // two bytes per character
	shortened, cut := shortenCell(value, 3)
	if !cut {
		t.Fatal("a value above the limit must be cut")
	}
	if !isValidUTF8(shortened) {
		t.Errorf("shortened = %q, want valid UTF-8", shortened)
	}
	if strings.Contains(shortened, "�") {
		t.Errorf("shortened = %q, want no replacement character", shortened)
	}
}

// isValidUTF8 reports whether the value is valid UTF-8.
func isValidUTF8(value string) bool {
	for _, r := range value {
		if r == '�' {
			return false
		}
	}
	return true
}

// TestRowBudgetShortensCells verifies the cell budget over a row.
func TestRowBudgetShortensCells(t *testing.T) {
	budget := rowBudget{limits: queryLimits{MaxCellBytes: 8}}
	row := map[string]interface{}{
		"id":    1,
		"notes": "a value that is longer than the cap",
	}

	if !budget.addRow(row, 0) {
		t.Fatal("a row must fit when there is no response budget")
	}
	if !budget.cellsTruncated {
		t.Error("cells_truncated must be set")
	}
	if budget.cellsCut != 1 {
		t.Errorf("cells cut = %d, want 1", budget.cellsCut)
	}
	if len(budget.truncatedCells) != 1 {
		t.Fatalf("truncated cells = %d, want 1", len(budget.truncatedCells))
	}
	detail := budget.truncatedCells[0]
	if detail.Row != 0 || detail.Column != "notes" {
		t.Errorf("detail = %+v, want row 0 and column notes", detail)
	}
	if detail.OriginalBytes != len("a value that is longer than the cap") {
		t.Errorf("original bytes = %d, want the original size", detail.OriginalBytes)
	}
	if !strings.HasPrefix(row["notes"].(string), "a value ") {
		t.Errorf("notes = %q, want the prefix of the value", row["notes"])
	}
	if row["id"] != 1 {
		t.Errorf("a short cell must not change: %v", row["id"])
	}
}

// TestRowBudgetStopsAtTheResponseBudget verifies that the result stops when
// the budget is reached.
func TestRowBudgetStopsAtTheResponseBudget(t *testing.T) {
	first := map[string]interface{}{"id": 1, "name": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	second := map[string]interface{}{"id": 2, "name": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}

	// The budget fits the first row and leaves no room for the second.
	budget := rowBudget{limits: queryLimits{MaxResponseBytes: estimateRowBytes(first) + 4}}

	if !budget.addRow(first, 0) {
		t.Fatal("the first row must fit")
	}
	if budget.addRow(second, 1) {
		t.Error("the second row must not fit in the budget")
	}
	if !budget.responseTruncated {
		t.Error("response_truncated must be set")
	}
}

// TestRowBudgetCountsEveryRow verifies that a budget large enough keeps all
// the rows.
func TestRowBudgetCountsEveryRow(t *testing.T) {
	budget := rowBudget{limits: queryLimits{MaxResponseBytes: 1024}}
	for i := 0; i < 10; i++ {
		if !budget.addRow(map[string]interface{}{"id": i}, i) {
			t.Fatalf("row %d must fit in the budget", i)
		}
	}
	if budget.responseTruncated {
		t.Error("response_truncated must not be set")
	}
}

// TestWithTruncationSentinel verifies the legacy sentinel of the catalog
// tools.
func TestWithTruncationSentinel(t *testing.T) {
	rows := []map[string]interface{}{{"id": 1}}
	if got := withTruncationSentinel(rows, false); len(got) != 1 {
		t.Errorf("a complete result must not carry the sentinel: %v", got)
	}

	withSentinel := withTruncationSentinel(rows, true)
	if len(withSentinel) != 2 {
		t.Fatalf("rows = %d, want the row and the sentinel", len(withSentinel))
	}
	if _, ok := withSentinel[1]["_truncated"]; !ok {
		t.Errorf("the sentinel is missing: %v", withSentinel[1])
	}
}

// TestTruncationNotice verifies the notice that reports a partial answer.
func TestTruncationNotice(t *testing.T) {
	if notice := truncationNotice(false, false, false, 0); notice != "" {
		t.Errorf("notice = %q, want empty for a complete result", notice)
	}
	notice := truncationNotice(true, true, true, 3)
	for _, want := range []string{"rows truncated", "cell(s) shortened", "response size limit"} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice = %q, want it to mention %q", notice, want)
		}
	}
}

// TestEstimateCellBytes verifies the size estimation of the response budget.
func TestEstimateCellBytes(t *testing.T) {
	if got := estimateCellBytes(nil); got != 4 {
		t.Errorf("null = %d, want 4", got)
	}
	if got := estimateCellBytes("abc"); got != 5 {
		t.Errorf("a plain string = %d, want 5", got)
	}
	if got := estimateCellBytes("a\"b"); got != 6 {
		t.Errorf("a string with a quote = %d, want 6", got)
	}
	if got := estimateCellBytes(int64(12345)); got != 5 {
		t.Errorf("a number = %d, want 5", got)
	}
	if got := estimateCellBytes(true); got != 4 {
		t.Errorf("true = %d, want 4", got)
	}
}

// TestEstimateRowBytesGrowsWithTheRow verifies that the estimate follows the
// content of the row.
func TestEstimateRowBytesGrowsWithTheRow(t *testing.T) {
	small := map[string]interface{}{"id": 1}
	big := map[string]interface{}{"id": 1, "notes": strings.Repeat("x", 1000)}
	if estimateRowBytes(big) <= estimateRowBytes(small) {
		t.Error("a bigger row must estimate more bytes")
	}
}

// TestResultEnvelopeFlagsAreSeparate verifies that the three truncations are
// reported independently.
func TestResultEnvelopeFlagsAreSeparate(t *testing.T) {
	envelope := ResultEnvelope{
		RowCount:          1,
		Truncated:         true,
		RowsTruncated:     true,
		CellsTruncated:    true,
		ResponseTruncated: false,
		Limits:            AppliedLimits{MaxRows: maxQueryRows},
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("cannot marshal the envelope: %v", err)
	}
	for _, key := range []string{"rows_truncated", "cells_truncated", "response_truncated", "limits"} {
		if !strings.Contains(string(encoded), `"`+key+`"`) {
			t.Errorf("envelope = %s, want the key %q", encoded, key)
		}
	}
	if strings.Contains(string(encoded), `"response_truncated":true`) {
		t.Errorf("envelope = %s, want response_truncated false", encoded)
	}
}
