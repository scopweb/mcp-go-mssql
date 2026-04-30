package resultfmt

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Formatter converts SQL Server column values to AI-friendly representations.
// It reduces token waste by formatting types semantically and collecting
// result-set metadata for downstream consumers.
type Formatter struct {
	// columnTypes holds the SQL Server type name for each column (1-indexed).
	columnTypes []string
	// nullCount tracks how many NULL values appear per column.
	nullCounts []int
	// totalRows is the number of data rows scanned (excludes the truncation row).
	totalRows int
}

// NewFormatter returns a Formatter ready to process rows from the given column
// metadata slice, as returned by (*sql.Rows).ColumnTypes().
func NewFormatter(cols []*sql.ColumnType) *Formatter {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = strings.ToUpper(c.DatabaseTypeName())
	}
	return &Formatter{
		columnTypes: names,
		nullCounts:  make([]int, len(cols)),
	}
}

// FormatValue converts a raw database value to its semantic representation.
// It never returns nil — NULL values are returned as the empty string with
// isNull=true so callers can distinguish them from empty strings.
func (f *Formatter) FormatValue(colIdx int, raw interface{}) (value interface{}, isNull bool) {
	if raw == nil {
		f.nullCounts[colIdx]++
		return "", true
	}

	// Defensive: out-of-range columns are returned as-is.
	if colIdx < 0 || colIdx >= len(f.columnTypes) {
		return raw, false
	}

	typeName := f.columnTypes[colIdx]

	switch typeName {
	case "DATETIME", "DATETIME2", "DATETIME2(7)", "DATETIMEOFFSET":
		return formatDateTime(raw)
	case "SMALLDATETIME":
		return formatSmallDateTime(raw)
	case "DATE":
		return formatDate(raw)
	case "TIME", "TIME(7)", "TIME(0)":
		return formatTime(raw)
	case "BIT", "BIT VARYING":
		return formatBit(raw)
	case "UNIQUEIDENTIFIER":
		return formatGUID(raw)
	case "DECIMAL", "NUMERIC", "DECIMAL(18,0)", "NUMERIC(18,0)", "MONEY", "SMALLMONEY", "DECIMAL(19,4)":
		return formatDecimal(raw)
	case "FLOAT", "REAL", "FLOAT(53)":
		return formatFloat(raw)
	case "VARBINARY", "BINARY", "IMAGE", "VARBINARY(MAX)", "BINARY(16)", "TIMESTAMP":
		return formatBinary(raw)
	case "XML":
		return formatXML(raw)
	case "NVARCHAR", "VARCHAR", "CHAR", "NCHAR", "NTEXT", "TEXT":
		return formatString(raw)
	case "INT", "BIGINT", "SMALLINT", "TINYINT":
		return formatInt(raw)
	default:
		// Fallback: handle []byte (e.g. from sql.Scanner conversions).
		if b, ok := raw.([]byte); ok {
			return string(b), false
		}
		return raw, false
	}
}

// Metadata holds result-set information produced after formatting all rows.
type Metadata struct {
	TotalRows   int
	ColumnCount int
	ColumnTypes []string
	NullCounts  []int
	Truncated   bool
	TruncatedAt int // maxQueryRows limit when Truncated is true
}

// BuildMetadata returns the immutable Metadata for the result set.
func (f *Formatter) BuildMetadata(truncated bool, limit int) Metadata {
	return Metadata{
		TotalRows:   f.totalRows,
		ColumnCount: len(f.columnTypes),
		ColumnTypes: f.columnTypes,
		NullCounts:  f.nullCounts,
		Truncated:   truncated,
		TruncatedAt: limit,
	}
}

// IncRow increments the row counter. Call once per data row scanned.
func (f *Formatter) IncRow() {
	f.totalRows++
}

// FormatResult converts rows to AI-optimized output.
// It takes raw column types and rows from sql.Rows, formats each value
// semantically, and returns both the rows and per-column metadata.
//
// After formatting all rows, call BuildMetadata() to retrieve column stats.
func FormatResult(colTypes []*sql.ColumnType, rows *sql.Rows, maxRows int) ([]map[string]interface{}, Metadata, error) {
	cols, err := rows.ColumnTypes()
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("ColumnTypes: %w", err)
	}

	f := NewFormatter(cols)

	columns, err := rows.Columns()
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("Columns: %w", err)
	}

	var results []map[string]interface{}
	rowCount := 0
	truncated := false

	for rows.Next() {
		if rowCount >= maxRows {
			truncated = true
			break
		}

		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range cols {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, Metadata{}, fmt.Errorf("Scan: %w", err)
		}

		row := make(map[string]interface{})
		for i := range columns {
			formatted, _ := f.FormatValue(i, values[i])
			row[columns[i]] = formatted
		}

		results = append(results, row)
		f.IncRow()
		rowCount++
	}

	if truncated {
		results = append(results, map[string]interface{}{
			"_truncated": fmt.Sprintf("Results limited to %d rows. Use WHERE or TOP to narrow the query.", maxRows),
		})
	}

	return results, f.BuildMetadata(truncated, maxRows), nil
}

// ToJSON serialises a result set with its metadata in a single JSON document.
// The _meta key is appended so downstream MCP handlers can include it in
// CallToolResult.Meta without re-serialising.
func ToJSON(rows []map[string]interface{}, meta Metadata) ([]byte, error) {
	doc := map[string]interface{}{
		"_data":  rows,
		"_meta":  meta,
		"_types": meta.ColumnTypes,
	}
	return json.MarshalIndent(doc, "", "  ")
}

// ── Type-specific formatters ─────────────────────────────────────────────────

func formatDateTime(raw interface{}) (interface{}, bool) {
	switch v := raw.(type) {
	case time.Time:
		return v.UTC().Format("2006-01-02T15:04:05.000Z"), false
	case []byte:
		if t, err := time.Parse("2006-01-02 15:04:05.000", strings.TrimSpace(string(v))); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05.000Z"), false
		}
		return string(v), false
	case string:
		if t, err := time.Parse("2006-01-02 15:04:05.000", strings.TrimSpace(v)); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05.000Z"), false
		}
		return v, false
	}
	return raw, raw == nil
}

func formatSmallDateTime(raw interface{}) (interface{}, bool) {
	switch v := raw.(type) {
	case time.Time:
		return v.UTC().Format("2006-01-02T15:04:00Z"), false
	case []byte:
		if t, err := time.Parse("2006-01-02 15:04:00", strings.TrimSpace(string(v))); err == nil {
			return t.UTC().Format("2006-01-02T15:04:00Z"), false
		}
		return string(v), false
	case string:
		if t, err := time.Parse("2006-01-02 15:04:00", strings.TrimSpace(v)); err == nil {
			return t.UTC().Format("2006-01-02T15:04:00Z"), false
		}
		return v, false
	}
	return raw, raw == nil
}

func formatDate(raw interface{}) (interface{}, bool) {
	switch v := raw.(type) {
	case time.Time:
		return v.UTC().Format("2006-01-02"), false
	case []byte:
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(string(v))); err == nil {
			return t.UTC().Format("2006-01-02"), false
		}
		return string(v), false
	case string:
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(v)); err == nil {
			return t.UTC().Format("2006-01-02"), false
		}
		return v, false
	}
	return raw, raw == nil
}

func formatTime(raw interface{}) (interface{}, bool) {
	switch v := raw.(type) {
	case time.Time:
		return v.UTC().Format("15:04:05.000Z"), false
	case []byte:
		if t, err := time.Parse("15:04:05.000", strings.TrimSpace(string(v))); err == nil {
			return t.UTC().Format("15:04:05.000Z"), false
		}
		return string(v), false
	case string:
		if t, err := time.Parse("15:04:05.000", strings.TrimSpace(v)); err == nil {
			return t.UTC().Format("15:04:05.000Z"), false
		}
		return v, false
	}
	return raw, raw == nil
}

func formatBit(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case bool:
		return v, false
	case []byte:
		s := strings.TrimSpace(string(v))
		return s == "1" || strings.EqualFold(s, "true"), false
	case string:
		s := strings.TrimSpace(v)
		return s == "1" || strings.EqualFold(s, "true"), false
	case int64:
		return v != 0, false
	case int:
		return v != 0, false
	}
	return raw, raw == nil
}

func formatGUID(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case uuid.UUID:
		return v.String(), false
	case []byte:
		if len(v) == 16 {
			return uuid.UUID(v).String(), false
		}
		return string(v), false
	case string:
		if u, err := uuid.Parse(v); err == nil {
			return u.String(), false
		}
		return v, false
	}
	return raw, raw == nil
}

func formatDecimal(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case float64:
		// Suppress scientific notation for large numbers — SQL Server decimal
		// values that fit in an int64 should not appear as 1.23E+15.
		if v == float64(int64(v)) {
			return fmt.Sprintf("%.0f", v), false
		}
		return fmt.Sprintf("%v", v), false
	case []byte:
		return strings.TrimSpace(string(v)), false
	case string:
		return strings.TrimSpace(v), false
	case int64:
		return fmt.Sprintf("%d", v), false
	case int:
		return fmt.Sprintf("%d", v), false
	}
	return raw, raw == nil
}

func formatFloat(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case float64:
		// Keep full precision; avoid dropping significant digits.
		return fmt.Sprintf("%v", v), false
	case []byte:
		return strings.TrimSpace(string(v)), false
	case string:
		return strings.TrimSpace(v), false
	}
	return raw, raw == nil
}

func formatBinary(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case []byte:
		return "0x" + strings.TrimLeft(fmt.Sprintf("%x", v), "0"), false
	case string:
		return strings.TrimPrefix(strings.TrimSpace(v), "0x"), false
	}
	return raw, raw == nil
}

func formatXML(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case []byte:
		// Basic indent: collapse spaces/newlines then re-indent via json.
		// Proper XML indent would require an XML library; this reduces bloat.
		collapsed := collapseSpaces(string(v))
		var any interface{}
		if err := json.Unmarshal([]byte(collapsed), &any); err == nil {
			if indented, err := json.MarshalIndent(any, "", "  "); err == nil {
				return string(indented), false
			}
		}
		return string(v), false
	case string:
		collapsed := collapseSpaces(v)
		var any interface{}
		if err := json.Unmarshal([]byte(collapsed), &any); err == nil {
			if indented, err := json.MarshalIndent(any, "", "  "); err == nil {
				return string(indented), false
			}
		}
		return v, false
	}
	return raw, raw == nil
}

func formatString(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case []byte:
		return strings.TrimSpace(string(v)), false
	case string:
		return strings.TrimSpace(v), false
	}
	return raw, raw == nil
}

func formatInt(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	switch v := raw.(type) {
	case int64:
		return v, false
	case int:
		return v, false
	case []byte:
		return strings.TrimSpace(string(v)), false
	case string:
		return strings.TrimSpace(v), false
	}
	return raw, raw == nil
}

// collapseSpaces collapses runs of whitespace in s to a single space.
// Used to compact JSON-like XML before re-formatting.
func collapseSpaces(s string) string {
	var out []rune
	var prevSpace bool
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				out = append(out, ' ')
				prevSpace = true
			}
		} else {
			out = append(out, r)
			prevSpace = false
		}
	}
	return string(out)
}
