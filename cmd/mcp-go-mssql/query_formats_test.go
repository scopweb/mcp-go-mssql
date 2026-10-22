package main

// Tests for the formats a query result can be rendered in.
//
// The columnar format is opt-in and must not change the default: the same
// data has to be there, in the same order, and the answer has to be smaller.

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestColumnarRowsKeepsTheOrderOfTheColumns verifies that the values keep the
// order of the result set.
func TestColumnarRowsKeepsTheOrderOfTheColumns(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": 1, "name": "Acme", "active": true},
		{"id": 2, "name": "Beta", "active": false},
	}
	columns := []string{"id", "name", "active"}

	rendered := columnarRows(rows, columns)
	if len(rendered) != 2 {
		t.Fatalf("rows = %d, want 2", len(rendered))
	}
	if len(rendered[0]) != 3 {
		t.Fatalf("values = %d, want 3", len(rendered[0]))
	}
	if rendered[0][0] != 1 || rendered[0][1] != "Acme" || rendered[0][2] != true {
		t.Errorf("first row = %v, want the values in the order of the columns", rendered[0])
	}
	if rendered[1][1] != "Beta" {
		t.Errorf("second row = %v, want Beta in the second position", rendered[1])
	}
}

// TestColumnarRowsKeepsNulls verifies that a null is a null, not a missing
// value.
func TestColumnarRowsKeepsNulls(t *testing.T) {
	rows := []map[string]interface{}{{"id": 1, "note": nil}}
	rendered := columnarRows(rows, []string{"id", "note"})

	if len(rendered[0]) != 2 {
		t.Fatalf("values = %d, want 2", len(rendered[0]))
	}
	if rendered[0][1] != nil {
		t.Errorf("note = %v, want null", rendered[0][1])
	}
}

// TestColumnarRowsOnEmptyResult verifies the empty case.
func TestColumnarRowsOnEmptyResult(t *testing.T) {
	rendered := columnarRows(nil, []string{"id"})
	if len(rendered) != 0 {
		t.Errorf("rows = %d, want 0", len(rendered))
	}
}

// TestDuplicateColumns verifies the detection of repeated column names.
func TestDuplicateColumns(t *testing.T) {
	if got := duplicateColumns([]string{"id", "name"}); len(got) != 0 {
		t.Errorf("duplicates = %v, want none", got)
	}
	got := duplicateColumns([]string{"id", "id", "name"})
	if len(got) != 1 || got[0] != "id" {
		t.Errorf("duplicates = %v, want id", got)
	}
}

// TestColumnarFormatIsSmaller verifies the benefit of the format: the same
// data, with the column names paid once.
func TestColumnarFormatIsSmaller(t *testing.T) {
	columns := []string{"id", "name", "city", "email", "active", "created", "amount", "currency", "country", "notes"}
	rows := make([]map[string]interface{}, 0, 200)
	for i := 0; i < 200; i++ {
		row := map[string]interface{}{}
		for _, column := range columns {
			row[column] = "value"
		}
		rows = append(rows, row)
	}

	mapResult := QueryResult{
		ResultEnvelope: ResultEnvelope{RowCount: len(rows)},
		Rows:           rows,
	}
	columnarResult := ColumnarResult{
		ResultEnvelope: ResultEnvelope{RowCount: len(rows)},
		Format:         FormatColumnar,
		Columns:        columns,
		Rows:           columnarRows(rows, columns),
	}

	mapJSON, err := json.Marshal(mapResult)
	if err != nil {
		t.Fatalf("cannot marshal the map result: %v", err)
	}
	columnarJSON, err := json.Marshal(columnarResult)
	if err != nil {
		t.Fatalf("cannot marshal the columnar result: %v", err)
	}

	if len(columnarJSON) >= len(mapJSON) {
		t.Errorf("the columnar result is %d bytes and the map one %d bytes, want the columnar one to be smaller", len(columnarJSON), len(mapJSON))
	}
	// The columns are paid once, so the saving grows with the number of
	// rows: with ten columns, the map repeats ten names per row.
	if savings := float64(len(mapJSON)-len(columnarJSON)) / float64(len(mapJSON)); savings < 0.3 {
		t.Errorf("the columnar format only saves %.0f%%, want a clear saving", savings*100)
	}
}

// TestColumnarFormatKeepsTheValues verifies that the columnar result carries
// the same values as the map one.
func TestColumnarFormatKeepsTheValues(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": 1, "name": "Acme"},
		{"id": 2, "name": nil},
	}
	columns := []string{"id", "name"}
	rendered := columnarRows(rows, columns)

	encoded, err := json.Marshal(rendered)
	if err != nil {
		t.Fatalf("cannot marshal the values: %v", err)
	}
	if !strings.Contains(string(encoded), `[1,"Acme"]`) {
		t.Errorf("values = %s, want the value of the second column", encoded)
	}
	if !strings.Contains(string(encoded), `[2,null]`) {
		t.Errorf("values = %s, want a null in the second row", encoded)
	}
}
