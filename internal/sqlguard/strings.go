package sqlguard

import "strings"

// StripStringLiterals removes the *contents* of SQL string literals (both
// single and double quoted) so that patterns appearing inside strings are not
// falsely flagged as code. The opening and closing quote characters are
// preserved as empty literals (”) to keep the query structure intact.
//
// Handles SQL's escape convention of doubled quotes: ” inside a single-
// quoted string and "" inside a double-quoted identifier.
func StripStringLiterals(query string) string {
	var result strings.Builder
	result.Grow(len(query))

	i := 0
	for i < len(query) {
		ch := query[i]

		if ch == '\'' {
			result.WriteByte(ch) // opening quote
			i++
			for i < len(query) {
				if query[i] == '\'' {
					if i+1 < len(query) && query[i+1] == '\'' {
						i += 2 // escaped quote — drop both
					} else {
						result.WriteByte('\'')
						i++
						break
					}
				} else {
					i++
				}
			}
			continue
		}

		if ch == '"' {
			result.WriteByte(ch)
			i++
			for i < len(query) {
				if query[i] == '"' {
					if i+1 < len(query) && query[i+1] == '"' {
						i += 2
					} else {
						result.WriteByte('"')
						i++
						break
					}
				} else {
					i++
				}
			}
			continue
		}

		result.WriteByte(ch)
		i++
	}

	return result.String()
}

// StripAllComments removes ALL SQL comments (block /* */ and line --) from
// the query. Inline comments anywhere can be used to hide keywords from
// pattern matching, so this is the strongest variant available. Strings are
// preserved.
func StripAllComments(query string) string {
	result := inlineCommentPattern.ReplaceAllString(query, " ")
	result = stripLineComments(result)
	return result
}

// stripLineComments removes "-- to end of line" comments while preserving
// content inside SQL string literals. Bracket-quoted identifiers ([name])
// are treated as opaque (a -- inside [] is left alone) — this matches the
// historical behaviour expected by the validator and is conservative.
func stripLineComments(query string) string {
	var result strings.Builder
	inString := false
	var stringChar byte
	escapeNext := false

	for i := 0; i < len(query); i++ {
		ch := query[i]

		if escapeNext {
			escapeNext = false
			result.WriteByte(ch)
			continue
		}

		if ch == '\\' && inString {
			result.WriteByte(ch)
			escapeNext = true
			continue
		}

		if ch == '\'' && !inString {
			inString = true
			stringChar = '\''
			result.WriteByte(ch)
			continue
		}

		if ch == '"' && !inString {
			inString = true
			stringChar = '"'
			result.WriteByte(ch)
			continue
		}

		if ch == '[' && !inString {
			// SQL bracket-quoted identifier — pass through.
			result.WriteByte(ch)
			continue
		}

		if inString && ch == stringChar {
			if i+1 < len(query) && query[i+1] == stringChar {
				// Doubled quote escape — emit both, advance past pair.
				result.WriteByte(ch)
				result.WriteByte(ch)
				i++
				continue
			}
			inString = false
			result.WriteByte(ch)
			continue
		}

		if !inString && i+1 < len(query) && query[i] == '-' && query[i+1] == '-' {
			// Line comment — skip to end of line.
			i += 2
			for i < len(query) && query[i] != '\n' && query[i] != '\r' {
				i++
			}
			i-- // outer loop will re-increment.
			continue
		}

		result.WriteByte(ch)
	}

	return result.String()
}

// StripLeadingComments returns the query with leading whitespace, line
// comments, and block comments stripped, then upper-cased. Used by callers
// who need to determine the leading SQL verb (SELECT, WITH, INSERT, ...).
func StripLeadingComments(query string) string {
	q := strings.TrimSpace(strings.ToUpper(query))
	for strings.HasPrefix(q, "--") || strings.HasPrefix(q, "/*") || strings.HasPrefix(q, " ") || strings.HasPrefix(q, "\t") || strings.HasPrefix(q, "\n") || strings.HasPrefix(q, "\r") {
		switch {
		case strings.HasPrefix(q, "--"):
			if idx := strings.Index(q, "\n"); idx != -1 {
				q = strings.TrimSpace(q[idx+1:])
			} else {
				return q
			}
		case strings.HasPrefix(q, "/*"):
			if idx := strings.Index(q, "*/"); idx != -1 {
				q = strings.TrimSpace(q[idx+2:])
			} else {
				return q
			}
		default:
			q = strings.TrimSpace(q[1:])
		}
	}
	return q
}

// StripUnicodeControlChars removes invisible bidirectional control characters
// and zero-width formatting marks that could be used to obfuscate the query.
func StripUnicodeControlChars(query string) string {
	return unicodeControlChars.ReplaceAllString(query, "")
}

// ContainsUnicodeControlChars reports whether the query contains any of the
// invisible bidirectional/control characters covered by unicodeControlChars.
// Use ValidateUnicodeSafety for the full check that also catches homoglyph
// obfuscation; this function is the narrow detector for callers that want
// the boolean signal without the keyword-rewrite step.
func ContainsUnicodeControlChars(query string) bool {
	return unicodeControlChars.MatchString(query)
}

// isLatin reports whether r is a basic Latin letter (ASCII A-Z / a-z). SQL
// keywords are pure ASCII Latin so any non-Latin letter is suspect.
func isLatin(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
