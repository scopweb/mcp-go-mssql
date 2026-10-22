package main

// Tests for the index grouping that replaces STRING_AGG.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// indexRow builds a catalog row of the per-column index query.
func indexRow(name, indexType string, unique, primary bool, column string, ordinal, columnID int) map[string]interface{} {
	return map[string]interface{}{
		"index_name":      name,
		"index_type":      indexType,
		"is_unique":       unique,
		"is_primary_key":  primary,
		"column_name":     column,
		"key_ordinal":     int64(ordinal),
		"index_column_id": int64(columnID),
	}
}

// TestGroupIndexRowsJoinsColumns verifies the output shape is the one the
// aggregated query produced: one row per index, columns joined by ", ".
func TestGroupIndexRowsJoinsColumns(t *testing.T) {
	rows := []map[string]interface{}{
		indexRow("PK_Pedido", "CLUSTERED", true, true, "Id", 1, 1),
		indexRow("PK_Pedido", "CLUSTERED", true, true, "Codigo", 2, 2),
	}

	grouped := groupIndexRows(rows)
	if len(grouped) != 1 {
		t.Fatalf("grouped = %d rows, want 1", len(grouped))
	}
	if grouped[0]["columns"] != "Id, Codigo" {
		t.Errorf("columns = %v, want %q", grouped[0]["columns"], "Id, Codigo")
	}
	if grouped[0]["index_name"] != "PK_Pedido" {
		t.Errorf("index_name = %v, want PK_Pedido", grouped[0]["index_name"])
	}
	if grouped[0]["index_type"] != "CLUSTERED" {
		t.Errorf("index_type = %v, want CLUSTERED", grouped[0]["index_type"])
	}
	if grouped[0]["is_unique"] != true || grouped[0]["is_primary_key"] != true {
		t.Errorf("annotations lost: %v", grouped[0])
	}
}

// TestGroupIndexRowsKeepsIndexOrder verifies the order of the indexes is the
// order the server returned (primary keys first, then by name).
func TestGroupIndexRowsKeepsIndexOrder(t *testing.T) {
	rows := []map[string]interface{}{
		indexRow("PK_Pedido", "CLUSTERED", true, true, "Id", 1, 1),
		indexRow("IX_Pedido_Fecha", "NONCLUSTERED", false, false, "Fecha", 1, 1),
		indexRow("IX_Pedido_Codigo", "NONCLUSTERED", true, false, "Codigo", 1, 1),
	}

	grouped := groupIndexRows(rows)
	if len(grouped) != 3 {
		t.Fatalf("grouped = %d rows, want 3", len(grouped))
	}
	want := []string{"PK_Pedido", "IX_Pedido_Fecha", "IX_Pedido_Codigo"}
	for i, name := range want {
		if grouped[i]["index_name"] != name {
			t.Errorf("row %d = %v, want %s", i, grouped[i]["index_name"], name)
		}
	}
}

// TestGroupIndexRowsOrdersColumns verifies the column order reproduces the
// aggregated query: by key ordinal, with included columns (key ordinal 0)
// first, as ORDER BY key_ordinal did.
func TestGroupIndexRowsOrdersColumns(t *testing.T) {
	rows := []map[string]interface{}{
		indexRow("IX_Cliente", "NONCLUSTERED", false, false, "Ciudad", 0, 3),
		indexRow("IX_Cliente", "NONCLUSTERED", false, false, "Nombre", 1, 1),
		indexRow("IX_Cliente", "NONCLUSTERED", false, false, "Apellido", 2, 2),
	}

	grouped := groupIndexRows(rows)
	if len(grouped) != 1 {
		t.Fatalf("grouped = %d rows, want 1", len(grouped))
	}
	if grouped[0]["columns"] != "Ciudad, Nombre, Apellido" {
		t.Errorf("columns = %v, want %q", grouped[0]["columns"], "Ciudad, Nombre, Apellido")
	}
}

// TestGroupIndexRowsBreaksTiesDeterministically verifies that columns with
// the same key ordinal keep a stable order.
func TestGroupIndexRowsBreaksTiesDeterministically(t *testing.T) {
	rows := []map[string]interface{}{
		indexRow("IX_Cliente", "NONCLUSTERED", false, false, "B", 0, 2),
		indexRow("IX_Cliente", "NONCLUSTERED", false, false, "A", 0, 1),
	}

	grouped := groupIndexRows(rows)
	if grouped[0]["columns"] != "A, B" {
		t.Errorf("columns = %v, want %q", grouped[0]["columns"], "A, B")
	}
}

// TestGroupIndexRowsKeepsTruncationSentinel verifies that a partial result
// stays visible instead of looking complete.
func TestGroupIndexRowsKeepsTruncationSentinel(t *testing.T) {
	rows := []map[string]interface{}{
		indexRow("PK_Pedido", "CLUSTERED", true, true, "Id", 1, 1),
		{"_truncated": "Results limited to 500 rows. Use WHERE or TOP to narrow the query."},
	}

	grouped := groupIndexRows(rows)
	if len(grouped) != 2 {
		t.Fatalf("grouped = %d rows, want the index and the sentinel", len(grouped))
	}
	if _, ok := grouped[1]["_truncated"]; !ok {
		t.Errorf("the truncation sentinel was lost: %v", grouped[1])
	}
	if grouped[0]["index_name"] != "PK_Pedido" {
		t.Errorf("the sentinel must come last, got %v", grouped[0])
	}
}

// TestGroupIndexRowsOnEmptyInput verifies the empty case.
func TestGroupIndexRowsOnEmptyInput(t *testing.T) {
	grouped := groupIndexRows(nil)
	if len(grouped) != 0 {
		t.Errorf("grouped = %d rows, want 0", len(grouped))
	}
}

// TestValueToInt verifies the conversions of the catalog values.
func TestValueToInt(t *testing.T) {
	cases := []struct {
		value interface{}
		want  int
	}{
		{int64(7), 7},
		{int(7), 7},
		{int32(7), 7},
		{float64(7), 7},
		{nil, 0},
		{"7", 0},
	}
	for _, c := range cases {
		if got := valueToInt(c.value); got != c.want {
			t.Errorf("valueToInt(%v) = %d, want %d", c.value, got, c.want)
		}
	}
}

// TestIndexesQueryHasNoStringAgg guards the SQL Server 2008/2012
// compatibility: STRING_AGG only exists from SQL Server 2017.
func TestIndexesQueryHasNoStringAgg(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "tools_schema.go"))
	if err != nil {
		t.Fatalf("cannot read tools_schema.go: %v", err)
	}
	if strings.Contains(stripGoComments(string(source)), "STRING_AGG") {
		t.Error("tools_schema.go uses STRING_AGG, which only exists from SQL Server 2017")
	}
}

// stripGoComments removes the comments of a Go source file so a guard can
// look for a real usage instead of a mention in a comment.
func stripGoComments(source string) string {
	withoutBlocks := strings.ReplaceAll(source, "/*", "\x00")
	withoutBlocks = strings.ReplaceAll(withoutBlocks, "*/", "")

	var kept []string
	for _, line := range strings.Split(withoutBlocks, "\n") {
		if index := strings.Index(line, "//"); index >= 0 {
			line = line[:index]
		}
		kept = append(kept, strings.ReplaceAll(line, "\x00", ""))
	}
	return strings.Join(kept, "\n")
}
