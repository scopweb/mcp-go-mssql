package main

import (
	"testing"
)

func TestParseStatsMessages_Time(t *testing.T) {
	lines := []string{
		" SQL Server Execution Times:",
		"   CPU time = 16 ms,  elapsed time = 22 ms.",
	}
	got := parseStatsMessages(lines)
	if got["cpu_time_ms"] != 16 {
		t.Errorf("cpu_time_ms = %d, want 16", got["cpu_time_ms"])
	}
	if got["elapsed_time_ms"] != 22 {
		t.Errorf("elapsed_time_ms = %d, want 22", got["elapsed_time_ms"])
	}
}

func TestParseStatsMessages_Time_NoComma(t *testing.T) {
	lines := []string{
		"   CPU time = 5 ms. elapsed time = 12 ms.",
	}
	got := parseStatsMessages(lines)
	if got["cpu_time_ms"] != 5 {
		t.Errorf("cpu_time_ms = %d, want 5", got["cpu_time_ms"])
	}
	if got["elapsed_time_ms"] != 12 {
		t.Errorf("elapsed_time_ms = %d, want 12", got["elapsed_time_ms"])
	}
}

func TestParseStatsMessages_IO(t *testing.T) {
	lines := []string{
		"Table 'orders'. Scan count 1, logical reads 4, physical reads 0, read-ahead reads 0, lob logical reads 0, lob physical reads 0, lob read-ahead reads 0.",
	}
	got := parseStatsMessages(lines)
	if got["scan_count_orders"] != 1 {
		t.Errorf("scan_count_orders = %d, want 1", got["scan_count_orders"])
	}
	if got["logical_reads_orders"] != 4 {
		t.Errorf("logical_reads_orders = %d, want 4", got["logical_reads_orders"])
	}
	if got["physical_reads_orders"] != 0 {
		t.Errorf("physical_reads_orders = %d, want 0", got["physical_reads_orders"])
	}
	if got["logical_reads"] != 4 {
		t.Errorf("logical_reads aggregate = %d, want 4", got["logical_reads"])
	}
	if got["physical_reads"] != 0 {
		t.Errorf("physical_reads aggregate = %d, want 0", got["physical_reads"])
	}
}

func TestParseStatsMessages_IO_MultipleTables(t *testing.T) {
	lines := []string{
		"Table 'orders'. Scan count 1, logical reads 4, physical reads 0, read-ahead reads 0, lob logical reads 0, lob physical reads 0, lob read-ahead reads 0.",
		"Table 'customers'. Scan count 2, logical reads 8, physical reads 1, read-ahead reads 0, lob logical reads 0, lob physical reads 0, lob read-ahead reads 0.",
	}
	got := parseStatsMessages(lines)
	if got["scan_count_orders"] != 1 {
		t.Errorf("scan_count_orders = %d, want 1", got["scan_count_orders"])
	}
	if got["scan_count_customers"] != 2 {
		t.Errorf("scan_count_customers = %d, want 2", got["scan_count_customers"])
	}
	if got["logical_reads_customers"] != 8 {
		t.Errorf("logical_reads_customers = %d, want 8", got["logical_reads_customers"])
	}
	// Aggregate fields must be SUMS across every Table '...' line,
	// not the last-seen value. With orders=4 and customers=8, the
	// aggregate logical_reads must be 12 (and physical_reads 0+1=1).
	if got["logical_reads"] != 12 {
		t.Errorf("logical_reads aggregate = %d, want 12 (4+8)", got["logical_reads"])
	}
	if got["physical_reads"] != 1 {
		t.Errorf("physical_reads aggregate = %d, want 1 (0+1)", got["physical_reads"])
	}
}

func TestParseStatsMessages_Worktable(t *testing.T) {
	lines := []string{
		"Table 'Worktable'. Scan count 0, logical reads 0, physical reads 0, read-ahead reads 0, lob logical reads 0, lob physical reads 0, lob read-ahead reads 0.",
	}
	got := parseStatsMessages(lines)
	if got["scan_count_Worktable"] != 0 {
		t.Errorf("scan_count_Worktable = %d, want 0", got["scan_count_Worktable"])
	}
	if got["logical_reads"] != 0 {
		t.Errorf("logical_reads = %d, want 0", got["logical_reads"])
	}
}

func TestParseStatsMessages_NilInput(t *testing.T) {
	got := parseStatsMessages(nil)
	if got == nil {
		t.Fatal("parseStatsMessages(nil) returned nil; expected empty map")
	}
	if len(got) != 0 {
		t.Errorf("parseStatsMessages(nil) returned %d entries; want 0", len(got))
	}
}

func TestParseStatsMessages_EmptyInput(t *testing.T) {
	got := parseStatsMessages([]string{})
	if got == nil {
		t.Fatal("parseStatsMessages([]) returned nil; expected empty map")
	}
	if len(got) != 0 {
		t.Errorf("parseStatsMessages([]) returned %d entries; want 0", len(got))
	}
}

func TestParseStatsMessages_UnrecognizedLine(t *testing.T) {
	lines := []string{
		"Beginning execution loop",
		"(1 row affected)",
	}
	got := parseStatsMessages(lines)
	if len(got) != 0 {
		t.Errorf("parseStatsMessages(unrecognized) returned %d entries; want 0", len(got))
	}
}

func TestParseStatsMessages_BracketedTableName(t *testing.T) {
	lines := []string{
		"Table '[dbo].[orders]'. Scan count 1, logical reads 4, physical reads 0, read-ahead reads 0, lob logical reads 0, lob physical reads 0, lob read-ahead reads 0.",
	}
	got := parseStatsMessages(lines)
	// The bracketed/dotted name should be sanitized into safe JSON key
	// chars. '[', ']', '.' all become '_'. We just check the key was
	// recorded under SOME sanitized form.
	found := false
	for k, v := range got {
		if len(k) > len("scan_count_") && k[:len("scan_count_")] == "scan_count_" && v == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a scan_count_* key with value 1; got %v", got)
	}
}
