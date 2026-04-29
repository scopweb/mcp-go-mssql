package sqlguard

import (
	"strings"
	"unicode"
)

// ContainsHomoglyphs reports whether the (already-stripped) query contains
// non-ASCII letters that could be homoglyphs of ASCII letters used in SQL
// keywords — for example Cyrillic 'е' (U+0435) for Latin 'e'. The caller is
// expected to have removed string literals first so we don't false-positive
// on data values.
func ContainsHomoglyphs(query string) bool {
	queryWithoutStrings := StripStringLiterals(query)
	queryClean := StripUnicodeControlChars(queryWithoutStrings)
	for _, r := range queryClean {
		if unicode.IsLetter(r) && !isLatin(r) {
			return true
		}
	}
	return false
}

// NormalizeToASCII transliterates common Cyrillic, Greek, and full-width
// Latin homoglyphs to their plain ASCII equivalents. The returned string can
// then be re-checked against the dangerous keyword patterns to detect
// keyword obfuscation attacks like SELесT.
//
// The mapping is intentionally narrow — only characters that visually
// resemble ASCII letters and have appeared in real obfuscation attempts.
// Adding more later is safe; removing existing entries is not.
func NormalizeToASCII(query string) string {
	homoglyphMap := map[rune]rune{
		// Cyrillic lowercase that mimic Latin
		'а': 'a',
		'е': 'e',
		'о': 'o',
		'р': 'p',
		'с': 'c',
		'х': 'x',
		'ё': 'e',
		// Greek lowercase that mimic Latin
		'α': 'a',
		'ε': 'e',
		'ο': 'o',
		'ρ': 'p',
		'τ': 't',
		'υ': 'u',
		// Full-width Latin lookalikes
		'ａ': 'a',
		'ｂ': 'b',
		'ｃ': 'c',
		'ｄ': 'd',
		'ｅ': 'e',
	}

	result := make([]rune, 0, len(query))
	for _, r := range query {
		if replacement, ok := homoglyphMap[r]; ok {
			result = append(result, replacement)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// IsSystemSchemaTable reports whether a table reference (lowercased, possibly
// schema-qualified by ExtractTableRefs) belongs to a SQL Server system schema
// (sys.*, information_schema.*). ExtractTableRefs preserves the schema prefix
// in TableRef.Table for whitelist rejection; this helper lets callers branch
// on the same set without re-encoding the schema list.
func IsSystemSchemaTable(table string) bool {
	for sysSchema := range systemSchemas {
		if strings.HasPrefix(table, sysSchema+".") {
			return true
		}
	}
	return false
}

// ParseTablePrefix parses a dot-separated prefix like "DB.schema." or
// "schema." into (database, schema) components. Empty prefix returns two
// empty strings. Forms with more than two components return empty for both
// (caller will treat the reference as malformed and ignore it).
func ParseTablePrefix(prefix string) (database, schema string) {
	prefix = strings.TrimRight(prefix, ".")
	if prefix == "" {
		return "", ""
	}
	parts := strings.Split(prefix, ".")
	for i, p := range parts {
		parts[i] = strings.Trim(p, "[] \t")
	}
	switch len(parts) {
	case 1:
		return "", strings.ToLower(parts[0])
	case 2:
		return strings.ToLower(parts[0]), strings.ToLower(parts[1])
	default:
		return "", ""
	}
}

// ParseTableRef parses a (prefix, name) pair returned by the table extraction
// regex into (schema, tableName). When prefix is empty the schema defaults to
// "dbo" — SQL Server's default schema. Brackets and quotes are stripped.
//
// This is the helper used by ExtractDDLTargetObjects, where we always want a
// concrete schema. For general table-reference extraction (which preserves
// the cross-database qualifier) see ExtractTableRefs.
func ParseTableRef(prefix, name string) (schema, tableName string) {
	tableName = strings.ToLower(strings.Trim(strings.Trim(name, "[]"), `"`))

	if prefix == "" {
		return "dbo", tableName
	}

	// Strip all bracket/quote characters then split on the last dot.
	// "[" and "]" may appear in the middle of the prefix (e.g. [dbo].[users])
	// so we remove all of them before splitting rather than using strings.Trim.
	clean := strings.ReplaceAll(strings.ReplaceAll(prefix, "[", ""), "]", "")
	clean = strings.ReplaceAll(clean, `"`, "")
	parts := strings.Split(clean, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2], tableName
	}
	return "dbo", tableName
}
