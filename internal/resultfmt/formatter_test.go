package resultfmt

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	mssql "github.com/microsoft/go-mssqldb"
)

// ── formatBinary ────────────────────────────────────────────────────────────
// Regression tests for the leading-zero corruption bug:
// previous implementation used strings.TrimLeft(_, "0") which silently
// changed the value of fixed-width BINARY(N) columns.

func TestFormatBinary_LeadingZerosPreserved(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"single leading zero", []byte{0x00, 0x01, 0x23}, "0x000123"},
		{"all zeros", []byte{0x00, 0x00, 0x00, 0x00}, "0x00000000"},
		{"single zero byte", []byte{0x00}, "0x00"},
		{"empty bytes", []byte{}, "0x"},
		{"binary(16) with leading zeros", []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, "0x00000000000000000000000000000001"},
		{"no leading zeros", []byte{0xab, 0xcd, 0xef}, "0xabcdef"},
		{"mixed", []byte{0x00, 0xff, 0x00, 0xff}, "0x00ff00ff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, isNull := formatBinary(tc.in)
			if isNull {
				t.Errorf("isNull=true, want false")
			}
			if got != tc.want {
				t.Errorf("formatBinary(%v)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatBinary_Nil(t *testing.T) {
	got, isNull := formatBinary(nil)
	if !isNull {
		t.Error("isNull=false, want true")
	}
	if got != "" {
		t.Errorf("got=%q, want empty string", got)
	}
}

func TestFormatBinary_StringPassthrough(t *testing.T) {
	// Driver normally returns []byte; the string path is unusual but must not
	// alter the value. Previous implementation stripped the "0x" prefix.
	got, _ := formatBinary("0xff")
	if got != "0xff" {
		t.Errorf("got=%v, want '0xff' (passthrough)", got)
	}
}

// ── formatGUID ──────────────────────────────────────────────────────────────
// Regression tests for the SQL Server -> RFC 4122 byte-order bug:
// previous implementation called uuid.UUID(b).String() on raw bytes, producing
// a GUID with the first 8 bytes in the wrong order. Driver returns
// mssql.UniqueIdentifier, whose String() applies the canonical reorder.

// expectedReorderedGUID corresponds to raw SQL Server bytes:
//
//	{0x12,0x34,0x56,0x78, 0x9a,0xbc, 0xde,0xf0, 0x12,0x34, 0x56,0x78,0x9a,0xbc,0xde,0xf0}
//
// First 4 bytes reversed: 78563412
// Next 2 reversed:        bc9a
// Next 2 reversed:        f0de
// Last 8 bytes unchanged: 1234-56789abcdef0
const expectedReorderedGUID = "78563412-bc9a-f0de-1234-56789abcdef0"

var rawSQLServerGUIDBytes = [16]byte{
	0x12, 0x34, 0x56, 0x78,
	0x9a, 0xbc,
	0xde, 0xf0,
	0x12, 0x34,
	0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0,
}

func TestFormatGUID_MssqlUniqueIdentifier(t *testing.T) {
	// The driver populates mssql.UniqueIdentifier via its Scan() method, which
	// applies the SQL Server -> RFC 4122 byte reorder. Direct construction
	// (mssql.UniqueIdentifier(wireBytes)) skips Scan and therefore the reorder
	// — that's not how the driver creates the value, so we mimic the real
	// path here.
	var raw mssql.UniqueIdentifier
	if err := raw.Scan(rawSQLServerGUIDBytes[:]); err != nil {
		t.Fatalf("UniqueIdentifier.Scan failed: %v", err)
	}
	got, isNull := formatGUID(raw)
	if isNull {
		t.Fatal("isNull=true, want false")
	}
	if got != expectedReorderedGUID {
		t.Errorf("formatGUID(mssql.UniqueIdentifier)=%q, want %q", got, expectedReorderedGUID)
	}
}

func TestFormatGUID_BytesAppliesSQLServerReorder(t *testing.T) {
	raw := rawSQLServerGUIDBytes[:]
	got, isNull := formatGUID(raw)
	if isNull {
		t.Fatal("isNull=true, want false")
	}
	if got != expectedReorderedGUID {
		t.Errorf("formatGUID([]byte)=%q, want %q (SQL Server byte order must be applied)", got, expectedReorderedGUID)
	}
}

func TestFormatGUID_BytesNot16(t *testing.T) {
	raw := []byte("not-a-guid")
	got, _ := formatGUID(raw)
	if got != "not-a-guid" {
		t.Errorf("got=%v, want 'not-a-guid' (passthrough for non-16-byte input)", got)
	}
}

func TestFormatGUID_StringIsNormalized(t *testing.T) {
	raw := "550e8400-e29b-41d4-a716-446655440000"
	want, _ := uuid.Parse(raw)
	got, _ := formatGUID(raw)
	if got != want.String() {
		t.Errorf("got=%v, want %v", got, want.String())
	}
}

func TestFormatGUID_Nil(t *testing.T) {
	got, isNull := formatGUID(nil)
	if !isNull {
		t.Error("isNull=false, want true")
	}
	if got != "" {
		t.Errorf("got=%v, want empty string", got)
	}
}

// ── formatString ────────────────────────────────────────────────────────────
// Regression tests for the silent-trim bug:
// previous implementation called strings.TrimSpace which silently mutated
// CHAR(N)/NCHAR(N) padding and any data with intentional leading/trailing
// whitespace.

func TestFormatString_PreservesPadding(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		{"trailing spaces []byte", []byte("hello   "), "hello   "},
		{"leading spaces []byte", []byte("   hello"), "   hello"},
		{"both sides []byte", []byte(" hello "), " hello "},
		{"only spaces []byte", []byte("   "), "   "},
		{"trailing spaces string", "data  ", "data  "},
		{"leading spaces string", "  data", "  data"},
		{"plain string", "foo", "foo"},
		{"empty []byte", []byte(""), ""},
		{"empty string", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, isNull := formatString(tc.in)
			if isNull {
				t.Errorf("isNull=true, want false")
			}
			if got != tc.want {
				t.Errorf("formatString(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatString_Nil(t *testing.T) {
	got, isNull := formatString(nil)
	if !isNull {
		t.Error("isNull=false, want true")
	}
	if got != "" {
		t.Errorf("got=%q, want empty string", got)
	}
}

// ── FormatValue: parameterized type names ───────────────────────────────────
// Regression tests for the silent type-fallthrough bug:
// DatabaseTypeName() returns the declared form including size/precision
// parameters ("DECIMAL(10,2)", "VARCHAR(255)", "BINARY(8)", ...). Without
// stripping the "(...)" suffix, the literal switch matched only specific
// hardcoded variants; everything else fell through to the default branch
// and was returned unformatted.

// newFormatter is a test helper that bypasses NewFormatter (which needs real
// *sql.ColumnType slices) by setting columnTypes directly.
func newFormatter(types ...string) *Formatter {
	return &Formatter{
		columnTypes: types,
		nullCounts:  make([]int, len(types)),
	}
}

func TestFormatValue_ParameterizedTypeMatchesBase(t *testing.T) {
	// For each pair, FormatValue(parameterized) must produce the same value as
	// FormatValue(base). If the parameterized form falls through to default,
	// the outputs diverge and the test fails.
	cases := []struct {
		name          string
		baseType      string
		parameterized string
		raw           interface{}
	}{
		{"DECIMAL(p,s)", "DECIMAL", "DECIMAL(10,2)", float64(123)},
		{"DECIMAL(18,0)", "DECIMAL", "DECIMAL(18,0)", float64(456)},
		{"NUMERIC(p,s)", "NUMERIC", "NUMERIC(38,10)", float64(789)},
		{"VARCHAR(N)", "VARCHAR", "VARCHAR(255)", []byte("hello")},
		{"NVARCHAR(MAX)", "NVARCHAR", "NVARCHAR(MAX)", []byte("foo")},
		{"CHAR(N) preserves padding", "CHAR", "CHAR(10)", []byte("abc       ")},
		{"BINARY(N)", "BINARY", "BINARY(8)", []byte{0x00, 0x12, 0x34}},
		{"VARBINARY(MAX)", "VARBINARY", "VARBINARY(MAX)", []byte{0xab, 0xcd}},
		{"DATETIME2(N)", "DATETIME2", "DATETIME2(7)", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)},
		{"DATETIMEOFFSET(N)", "DATETIMEOFFSET", "DATETIMEOFFSET(7)", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)},
		{"FLOAT(N)", "FLOAT", "FLOAT(53)", float64(1.5)},
		{"TIME(N)", "TIME", "TIME(7)", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBase, _ := newFormatter(tc.baseType).FormatValue(0, tc.raw)
			gotParam, _ := newFormatter(tc.parameterized).FormatValue(0, tc.raw)
			if !reflect.DeepEqual(gotBase, gotParam) {
				t.Errorf("type %q routed differently from base %q:\n  base  = %#v\n  param = %#v",
					tc.parameterized, tc.baseType, gotBase, gotParam)
			}
		})
	}
}

// TestFormatValue_ParameterizedDoesNotFallThrough is a stricter check: it
// asserts that the formatted output is NOT the raw input, which is what the
// default branch returns for non-[]byte values. This catches the case where
// both base and parameterized happen to fall through (so DeepEqual passes
// but neither got formatted).
func TestFormatValue_ParameterizedDoesNotFallThrough(t *testing.T) {
	cases := []struct {
		name     string
		typeName string
		raw      interface{}
	}{
		// float64 -> formatDecimal yields a string; default would keep float64
		{"DECIMAL(10,2) formats float", "DECIMAL(10,2)", float64(123)},
		{"NUMERIC(38,10) formats float", "NUMERIC(38,10)", float64(456)},
		// time.Time -> formatDateTime yields ISO string; default would keep time.Time
		{"DATETIME2(7) formats time", "DATETIME2(7)", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)},
		// []byte -> formatBinary yields "0x..." string; default would yield raw string(b)
		{"BINARY(8) formats with 0x prefix", "BINARY(8)", []byte{0x00, 0x12, 0x34}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := newFormatter(tc.typeName).FormatValue(0, tc.raw)
			if reflect.DeepEqual(got, tc.raw) {
				t.Errorf("type %q fell through to default: got=%#v identical to raw input", tc.typeName, got)
			}
		})
	}
}

// ── FormatValue: NULL contract ──────────────────────────────────────────────
// FormatValue documents "never returns nil — NULL is returned as ('', true)".
// Callers are expected to translate isNull -> JSON null when serializing.

func TestFormatValue_NullReturnsEmptyStringWithIsNull(t *testing.T) {
	f := newFormatter("VARCHAR")
	got, isNull := f.FormatValue(0, nil)
	if !isNull {
		t.Error("isNull=false, want true for nil raw value")
	}
	if got != "" {
		t.Errorf("got=%q, want empty string (per FormatValue doc contract)", got)
	}
	// Null counts must be incremented per column.
	if f.nullCounts[0] != 1 {
		t.Errorf("nullCounts[0]=%d, want 1", f.nullCounts[0])
	}
}

func TestFormatValue_NullCountsAreColumnLocal(t *testing.T) {
	f := newFormatter("INT", "VARCHAR", "DECIMAL")
	_, _ = f.FormatValue(1, nil)
	_, _ = f.FormatValue(1, nil)
	_, _ = f.FormatValue(2, nil)
	want := []int{0, 2, 1}
	if !reflect.DeepEqual(f.nullCounts, want) {
		t.Errorf("nullCounts=%v, want %v", f.nullCounts, want)
	}
}

// ── formatDecimal ───────────────────────────────────────────────────────────
// Regression tests for scientific-notation suppression. The float64 branch
// must never emit "1.23e+15"-style output, regardless of magnitude or
// fractional form.

func TestFormatDecimal_NoScientificNotation(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		{"large integer-valued float", float64(1e15), "1000000000000000"},
		{"small integer-valued float", float64(123), "123"},
		{"zero", float64(0), "0"},
		{"negative integer", float64(-456), "-456"},
		{"fractional", float64(1.5), "1.5"},
		{"small fractional", float64(0.1), "0.1"},
		{"int64 large", int64(9223372036854775807), "9223372036854775807"},
		{"int64 negative", int64(-100), "-100"},
		{"int", int(42), "42"},
		{"[]byte passthrough", []byte("123.45"), "123.45"},
		{"[]byte trimmed", []byte("  78.9  "), "78.9"},
		{"string passthrough", "456.78", "456.78"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, isNull := formatDecimal(tc.in)
			if isNull {
				t.Errorf("isNull=true, want false")
			}
			if got != tc.want {
				t.Errorf("formatDecimal(%v)=%q, want %q", tc.in, got, tc.want)
			}
			// Belt-and-suspenders: never any 'e' or 'E' in the output for
			// numeric-typed inputs (covers exponent format from %v, %g, etc).
			if s, ok := got.(string); ok {
				if strings.ContainsAny(s, "eE") {
					t.Errorf("output %q contains exponent character", s)
				}
			}
		})
	}
}

func TestFormatDecimal_Nil(t *testing.T) {
	got, isNull := formatDecimal(nil)
	if !isNull {
		t.Error("isNull=false, want true")
	}
	if got != "" {
		t.Errorf("got=%q, want empty string", got)
	}
}

// ── formatXML: passthrough ──────────────────────────────────────────────────
// Regression: previous implementation tried json.Unmarshal on XML, which always
// failed and silently fell back to the raw string. After simplification, XML
// passes through unchanged.

func TestFormatXML_PassesThrough(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		{"simple xml []byte", []byte("<root><a>1</a></root>"), "<root><a>1</a></root>"},
		{"simple xml string", "<root><a>1</a></root>", "<root><a>1</a></root>"},
		{"xml with whitespace preserved", []byte("<root>\n  <a>1</a>\n</root>"), "<root>\n  <a>1</a>\n</root>"},
		{"empty []byte", []byte(""), ""},
		{"empty string", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, isNull := formatXML(tc.in)
			if isNull {
				t.Error("isNull=true, want false")
			}
			if got != tc.want {
				t.Errorf("formatXML(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatXML_Nil(t *testing.T) {
	got, isNull := formatXML(nil)
	if !isNull {
		t.Error("isNull=false, want true")
	}
	if got != "" {
		t.Errorf("got=%q, want empty string", got)
	}
}
