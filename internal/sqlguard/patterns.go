package sqlguard

import "regexp"

// dangerousKeywordPatterns are pre-compiled word-boundary patterns that
// identify SQL operations forbidden under read-only mode (and used as a
// secondary defence inside other validators).
var dangerousKeywordPatterns = map[string]*regexp.Regexp{
	"INSERT":   regexp.MustCompile(`(?i)\bINSERT\b`),
	"UPDATE":   regexp.MustCompile(`(?i)\bUPDATE\b`),
	"DELETE":   regexp.MustCompile(`(?i)\bDELETE\b`),
	"DROP":     regexp.MustCompile(`(?i)\bDROP\b`),
	"CREATE":   regexp.MustCompile(`(?i)\bCREATE\b`),
	"ALTER":    regexp.MustCompile(`(?i)\bALTER\b`),
	"TRUNCATE": regexp.MustCompile(`(?i)\bTRUNCATE\b`),
	"MERGE":    regexp.MustCompile(`(?i)\bMERGE\b`),
	"EXEC":     regexp.MustCompile(`(?i)\bEXEC\b`),
	"EXECUTE":  regexp.MustCompile(`(?i)\bEXECUTE\b`),
	"CALL":     regexp.MustCompile(`(?i)\bCALL\b`),
	"BULK":     regexp.MustCompile(`(?i)\bBULK\b`),
	"BCP":      regexp.MustCompile(`(?i)\bBCP\b`),
}

// destructiveOpPatterns detect DDL operations that modify or destroy existing
// database objects. The caller decides whether to require confirmation based
// on whether the target object currently exists.
var destructiveOpPatterns = map[string]*regexp.Regexp{
	"DROP_TABLE":     regexp.MustCompile(`(?i)\bDROP\s+TABLE\b`),
	"DROP_VIEW":      regexp.MustCompile(`(?i)\bDROP\s+VIEW\b`),
	"DROP_PROC":      regexp.MustCompile(`(?i)\bDROP\s+PROCEDURE\b`),
	"DROP_FUNCTION":  regexp.MustCompile(`(?i)\bDROP\s+FUNCTION\b`),
	"ALTER_VIEW":     regexp.MustCompile(`(?i)\bALTER\s+VIEW\b`),
	"ALTER_TABLE":    regexp.MustCompile(`(?i)\bALTER\s+TABLE\b`),
	"TRUNCATE_TABLE": regexp.MustCompile(`(?i)\bTRUNCATE\s+TABLE\b`),
}

// tableExtractionPatterns match table references with optional database and
// schema qualifiers. Capture groups: 1 = full prefix (e.g. "db.schema." or
// "schema."), 2 = table name. The prefix is parsed separately to extract
// database vs schema components.
var tableExtractionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bFROM\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bJOIN\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bINTO\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bUPDATE\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bDELETE\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?\s+FROM`),
	regexp.MustCompile(`(?i)\b(?:CREATE|DROP|ALTER)\s+TABLE\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\b(?:CREATE|DROP|ALTER)\s+VIEW\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
	regexp.MustCompile(`(?i)\bTRUNCATE\s+TABLE\s+((?:\[?[\w]+\]?\.){0,2})\[?([\w]+)\]?`),
}

// systemSchemas are SQL Server schemas whose objects are excluded from
// user-table validation by default (e.g. INFORMATION_SCHEMA.COLUMNS).
var systemSchemas = map[string]bool{
	"information_schema": true,
	"sys":                true,
}

// sqlReservedWords contains SQL keywords that should never be treated as
// table names. Acts as a safety net for regex-based extraction.
var sqlReservedWords = map[string]bool{
	"as": true, "set": true, "return": true, "returns": true,
	"select": true, "where": true, "and": true, "or": true,
	"not": true, "null": true, "is": true, "in": true,
	"on": true, "by": true, "order": true, "group": true,
	"having": true, "case": true, "when": true, "then": true,
	"else": true, "end": true, "begin": true, "declare": true,
	"exec": true, "execute": true, "procedure": true, "function": true,
	"trigger": true, "index": true, "cursor": true, "open": true,
	"close": true, "fetch": true, "next": true, "values": true,
	"output": true, "inserted": true, "deleted": true,
}

// inlineCommentPattern matches /* ... */ block comments. Used by
// stripAllComments. Greedy is fine for sanitisation purposes.
var inlineCommentPattern = regexp.MustCompile(`/\*.*?\*/`)

// charConcatenationPattern matches CHAR(n)+CHAR(n)+... or NCHAR variants used
// to build keywords dynamically and bypass keyword detection.
var charConcatenationPattern = regexp.MustCompile(`(?i)(CHAR|NCHAR)\s*\(\d+\)(\s*\+\s*(CHAR|NCHAR)\s*\(\d+\))*`)

// dangerousHintsPattern matches table/index hints (NOLOCK, READUNCOMMITTED,
// etc.) that enable dirty reads or other risky behaviour.
var dangerousHintsPattern = regexp.MustCompile(`(?i)\b(WITH\s*\(\s*(NOLOCK|READUNCOMMITTED|READCOMMITTED|READCOMMITTEDLOCK|TABLOCK|UPDLOCK|HOLDLOCK|ROWLOCK)\s*\))`)

// waitforPattern matches WAITFOR which enables timing attacks.
var waitforPattern = regexp.MustCompile(`(?i)\bWAITFOR\b`)

// openrowsetPattern matches OPENROWSET (data exfiltration to external sources).
var openrowsetPattern = regexp.MustCompile(`(?i)\bOPENROWSET\b`)

// opendatasourcePattern matches OPENDATASOURCE (data exfiltration).
var opendatasourcePattern = regexp.MustCompile(`(?i)\bOPENDATASOURCE\b`)

// selectIntoPattern matches SELECT ... INTO ... FROM (creates new tables).
var selectIntoPattern = regexp.MustCompile(`(?i)\bSELECT\s+[^;]+INTO\s+[^;]+FROM\b`)

// tempTablePattern matches references to temp tables (#name).
var tempTablePattern = regexp.MustCompile(`(?i)#\w+`)

// unicodeControlChars matches Unicode bidirectional control characters and
// other potentially malicious invisible characters.
//   - U+200B..U+200F: zero-width spaces and directional formatting
//   - U+202A..U+202E: bidirectional override (LRO, RLO, etc.)
//   - U+2066..U+2069: bidirectional isolate controls
//
// Written with the regex engine's \x{NNNN} syntax so the source stays free of
// invisible characters and is reviewable on GitHub.
var unicodeControlChars = regexp.MustCompile(`[\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2066}-\x{2069}]`)

// subqueryExtractPattern matches a parenthesised SELECT ... FROM ... block.
// Used by ValidateSubqueriesForRestrictedTables. Non-greedy on the inside to
// avoid matching across nested parens — for that case the outer call still
// catches the parent block.
var subqueryExtractPattern = regexp.MustCompile(`(?i)\(\s*SELECT\s+[^)]+\)`)

// subqueryWithFromPattern is a stricter variant used by extractTablesFromSubqueries:
// requires the subquery body to contain FROM, which avoids matching boolean
// SELECT-style scalars and reduces false positives.
var subqueryWithFromPattern = regexp.MustCompile(`(?i)\(\s*SELECT\s+[^()]+FROM\s+[^()]+\)`)
