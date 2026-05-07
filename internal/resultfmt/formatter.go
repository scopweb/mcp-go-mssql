package resultfmt

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"
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
	// DatabaseTypeName() returns the declared form, including any size/precision
	// parameters: e.g. "DECIMAL(10,2)", "VARCHAR(255)", "BINARY(8)", "NVARCHAR(MAX)".
	// Strip "(...)" so all DECIMAL(p,s), VARCHAR(N), BINARY(N), DATETIME2(N), etc.
	// route to their base-type formatter instead of falling through to default.
	if i := strings.IndexByte(typeName, '('); i > 0 {
		typeName = typeName[:i]
	}

	switch typeName {
	case "DATETIME", "DATETIME2", "DATETIMEOFFSET":
		return formatDateTime(raw)
	case "SMALLDATETIME":
		return formatSmallDateTime(raw)
	case "DATE":
		return formatDate(raw)
	case "TIME":
		return formatTime(raw)
	case "BIT", "BIT VARYING":
		return formatBit(raw)
	case "UNIQUEIDENTIFIER":
		return formatGUID(raw)
	case "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
		return formatDecimal(raw)
	case "FLOAT", "REAL":
		return formatFloat(raw)
	case "VARBINARY", "BINARY", "IMAGE", "TIMESTAMP":
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
			formatted, isNull := f.FormatValue(i, values[i])
			if isNull {
				// Map SQL NULL to JSON null instead of "" so the AI can
				// distinguish NULL from a genuine empty string.
				row[columns[i]] = nil
			} else {
				row[columns[i]] = formatted
			}
		}

		results = append(results, row)
		f.IncRow()
		rowCount++
	}

	// Truncation is signalled via the returned Metadata only. The previous
	// sentinel row injection has been removed — see executeSecureQueryWithDB
	// in main.go for the rationale.
	return results, f.BuildMetadata(truncated, maxRows), nil
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
	case mssql.UniqueIdentifier:
		// Canonical path: the driver stores already-reordered bytes in this
		// type (the SQL Server -> RFC 4122 reorder is applied inside Scan()).
		// String() prints them as-is. Lowercase for consistency with uuid.UUID.
		return strings.ToLower(v.String()), false
	case uuid.UUID:
		return v.String(), false
	case []byte:
		// Defensive path: if a uniqueidentifier ever arrives as a raw 16-byte
		// slice (without going through the driver's typed path), the bytes are
		// in SQL Server wire order and need reordering. Scan() does that.
		// Previous implementations used uuid.UUID(v).String() (no reorder) or
		// copy() into mssql.UniqueIdentifier (also no reorder) — both wrong.
		if len(v) == 16 {
			var u mssql.UniqueIdentifier
			if err := u.Scan(v); err == nil {
				return strings.ToLower(u.String()), false
			}
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
		// strconv.FormatFloat with precision -1 emits the minimum digits
		// required to represent v exactly, never using scientific notation.
		// This covers both integer-valued floats (1e15 -> "1000000000000000")
		// and fractional ones (0.1 -> "0.1") with a single rule.
		//
		// CAVEAT: SQL Server DECIMAL(p,s) values larger than 2^53 lose
		// precision when transported as float64. The driver normally returns
		// these as []byte to avoid that, so this branch should be rare; for
		// the rare cases it triggers, accuracy is already lost upstream and
		// we cannot recover it here.
		return strconv.FormatFloat(v, 'f', -1, 64), false
	case []byte:
		return strings.TrimSpace(string(v)), false
	case string:
		return strings.TrimSpace(v), false
	case int64:
		return strconv.FormatInt(v, 10), false
	case int:
		return strconv.Itoa(v), false
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
		// hex.EncodeToString preserves all bytes including leading zeros.
		// Previous implementation used strings.TrimLeft(_, "0") which corrupted
		// fixed-width BINARY(N) values like 0x000123 -> 0x123.
		return "0x" + hex.EncodeToString(v), false
	case string:
		// Driver returns binary as []byte; the string path is unusual
		// (e.g. CONVERT(varchar, ...) in dynamic SQL). Pass through unchanged.
		return v, false
	}
	return raw, raw == nil
}

func formatXML(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	// XML values pass through unchanged. The previous implementation tried
	// json.Unmarshal on the XML payload (which never succeeds for XML) and
	// silently fell back to the raw string anyway — pure dead code. Real XML
	// pretty-printing would need encoding/xml decoder/encoder round-tripping,
	// which is out of scope here. The AI client can format if it wants to.
	switch v := raw.(type) {
	case []byte:
		return string(v), false
	case string:
		return v, false
	}
	return raw, raw == nil
}

func formatString(raw interface{}) (interface{}, bool) {
	if raw == nil {
		return "", true
	}
	// IMPORTANT: do NOT TrimSpace here. CHAR(N)/NCHAR(N) columns are space-padded
	// by SQL Server but the padding can be indistinguishable from intentional
	// trailing spaces in user data, especially with ANSI_PADDING ON. Preserving
	// raw values matches the previous behavior of executeSecureQuery before the
	// resultfmt rewrite and avoids silently mutating identifiers/codes.
	switch v := raw.(type) {
	case []byte:
		return string(v), false
	case string:
		return v, false
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

