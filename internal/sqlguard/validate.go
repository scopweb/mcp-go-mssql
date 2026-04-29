package sqlguard

import (
	"fmt"
	"regexp"
	"strings"
)

// ContainsCharConcatenation reports whether the query uses CHAR() / NCHAR()
// concatenation to construct keywords dynamically (a classic technique for
// dodging keyword-based filters). Three or more concatenations are required
// to flag — a single CHAR(83) by itself is not realistic abuse.
//
// String literals are stripped before analysis so 'CHAR(83)' as data won't
// match.
func ContainsCharConcatenation(query string) bool {
	queryWithoutStrings := StripStringLiterals(query)

	if !charConcatenationPattern.MatchString(queryWithoutStrings) {
		return false
	}

	// Cheap quick-fail done; now require ≥ 3 concatenations.
	concatPattern := regexp.MustCompile(`(?i)(CHAR|NCHAR)\s*\(\s*\d+\s*\)(\s*\+\s*(CHAR|NCHAR)\s*\(\s*\d+\s*\)){2,}`)
	return concatPattern.MatchString(queryWithoutStrings)
}

// ContainsDangerousHints reports whether the query uses table or index hints
// like NOLOCK / READUNCOMMITTED that enable dirty reads or other risky
// behaviour.
func ContainsDangerousHints(query string) bool {
	queryWithoutStrings := StripStringLiterals(query)
	return dangerousHintsPattern.MatchString(queryWithoutStrings)
}

// ContainsWaitfor reports whether the query uses WAITFOR, which enables
// timing attacks to infer data existence (blind SQLi).
func ContainsWaitfor(query string) bool {
	queryWithoutStrings := StripStringLiterals(query)
	return waitforPattern.MatchString(queryWithoutStrings)
}

// ContainsDangerousSelectPatterns reports whether the query uses
// SELECT-based patterns associated with data exfiltration: OPENROWSET,
// OPENDATASOURCE, SELECT INTO, or temp-table writes via SELECT INTO.
//
// Comments are stripped before matching so that benign mentions of these
// patterns inside `--` or `/* */` comments do not produce false positives.
// Real attempts to smuggle e.g. `SELECT * /*x*/ INTO #t FROM t` past the
// regex still match because the regex tolerates whitespace between tokens
// and StripAllComments collapses comments to whitespace.
func ContainsDangerousSelectPatterns(query string) bool {
	cleaned := StripAllComments(query)
	cleaned = StripStringLiterals(cleaned)
	upperQuery := strings.ToUpper(cleaned)

	if openrowsetPattern.MatchString(upperQuery) {
		return true
	}
	if opendatasourcePattern.MatchString(upperQuery) {
		return true
	}
	if selectIntoPattern.MatchString(upperQuery) {
		return true
	}
	if tempTablePattern.MatchString(upperQuery) {
		// Only flag if SELECT INTO writes to a temp table. Just
		// referencing one (#tmp) is fine.
		if strings.Contains(upperQuery, "SELECT") && strings.Contains(upperQuery, "INTO") {
			return true
		}
	}
	return false
}

// ValidateStructuralSafety performs deep structural analysis of the query to
// detect obfuscation techniques that simple keyword matching would miss.
// Returns a non-nil error describing the specific threat when one is found,
// or nil when the query is clean by these criteria.
//
// Checks performed:
//   - Comment-hidden keywords (DROP inside /* */ or after --)
//   - CHAR / NCHAR concatenation bypass
//   - Dirty-read table/index hints (NOLOCK, etc.)
//   - WAITFOR (timing-based blind SQLi)
//   - OPENROWSET / OPENDATASOURCE / SELECT INTO exfiltration patterns
func ValidateStructuralSafety(query string) error {
	// Comment-hidden keyword detection: if the query contains comment
	// markers AND any dangerous keyword (with string literals stripped to
	// avoid false positives on data), reject. This catches both
	//   "SELECT /*DROP*/ ..."
	// and
	//   "/*DROP*/ SELECT ..."
	queryForCommentCheck := StripStringLiterals(query)
	if strings.Contains(query, "/*") || strings.Contains(query, "--") {
		upperOriginal := strings.ToUpper(queryForCommentCheck)
		for keyword := range dangerousKeywordPatterns {
			if strings.Contains(upperOriginal, keyword) {
				return fmt.Errorf("query contains forbidden keyword '%s' — comments cannot hide SQL keywords from security validation", keyword)
			}
		}
	}

	if ContainsCharConcatenation(query) {
		return fmt.Errorf("query contains character concatenation pattern that may be used to bypass keyword detection")
	}

	if ContainsDangerousHints(query) {
		return fmt.Errorf("query contains forbidden table hint (e.g. NOLOCK) which enables dirty reads")
	}

	if ContainsWaitfor(query) {
		return fmt.Errorf("query contains WAITFOR which can be used for timing-based attacks to infer data existence")
	}

	if ContainsDangerousSelectPatterns(query) {
		return fmt.Errorf("query contains dangerous pattern (OPENROWSET/OPENDATASOURCE/SELECT INTO) that could be used for data exfiltration")
	}

	return nil
}

// ValidateUnicodeSafety checks for Unicode-based obfuscation: bidirectional
// control characters and homoglyphs of ASCII letters used in SQL keywords.
// When a homoglyph is detected the query is normalised to ASCII and re-tested
// against the dangerous-keyword set; a positive match returns a specific
// error naming the keyword.
func ValidateUnicodeSafety(query string) error {
	cleanQuery := StripAllComments(query)
	cleanQuery = StripStringLiterals(cleanQuery)

	if unicodeControlChars.MatchString(cleanQuery) {
		return fmt.Errorf("query contains Unicode control characters (e.g. bidirectional override) which can be used for obfuscation")
	}

	if ContainsHomoglyphs(cleanQuery) {
		normalized := NormalizeToASCII(cleanQuery)
		upperNormalized := strings.ToUpper(normalized)

		for keyword, pattern := range dangerousKeywordPatterns {
			if pattern.MatchString(upperNormalized) {
				return fmt.Errorf("query contains non-ASCII characters that appear to be homoglyphs of keyword '%s' — possible Unicode obfuscation attack", keyword)
			}
		}

		return fmt.Errorf("query contains non-Latin Unicode characters that may be used for homoglyph obfuscation")
	}

	return nil
}
