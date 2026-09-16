package main

import (
	"strings"
	"unicode"
)

var statementStartKeywords = map[string]bool{
	"ALTER":       true,
	"BACKUP":      true,
	"BEGIN":       true,
	"BULK":        true,
	"CHECKPOINT":  true,
	"COMMIT":      true,
	"CREATE":      true,
	"DBCC":        true,
	"DECLARE":     true,
	"DELETE":      true,
	"DENY":        true,
	"DROP":        true,
	"EXEC":        true,
	"EXECUTE":     true,
	"GRANT":       true,
	"IF":          true,
	"INSERT":      true,
	"KILL":        true,
	"MERGE":       true,
	"PRINT":       true,
	"RAISERROR":   true,
	"RECONFIGURE": true,
	"RESTORE":     true,
	"RETURN":      true,
	"REVOKE":      true,
	"ROLLBACK":    true,
	"SAVE":        true,
	"SELECT":      true,
	"SET":         true,
	"SHUTDOWN":    true,
	"THROW":       true,
	"TRUNCATE":    true,
	"UPDATE":      true,
	"USE":         true,
	"WAITFOR":     true,
	"WHILE":       true,
	"WITH":        true,
}

func topLevelKeywords(query string) []string {
	runes := []rune(query)
	n := len(runes)
	const (
		stNormal = iota
		stString
		stBracket
		stQuotedIdent
		stLineComment
		stBlockComment
	)
	state := stNormal
	depth := 0
	var keywords []string
	for i := 0; i < n; {
		c := runes[i]
		switch state {
		case stNormal:
			switch {
			case c == '\'':
				state = stString
				i++
			case (c == 'N' || c == 'n') && i+1 < n && runes[i+1] == '\'':
				state = stString
				i += 2
			case c == '[':
				state = stBracket
				i++
			case c == '"':
				state = stQuotedIdent
				i++
			case c == '-' && i+1 < n && runes[i+1] == '-':
				state = stLineComment
				i += 2
			case c == '/' && i+1 < n && runes[i+1] == '*':
				state = stBlockComment
				i += 2
			case c == '(':
				depth++
				i++
			case c == ')':
				if depth > 0 {
					depth--
				}
				i++
			case depth == 0 && (unicode.IsLetter(c) || c == '_'):
				start := i
				i++
				for i < n {
					r := runes[i]
					if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' {
						i++
						continue
					}
					break
				}
				keywords = append(keywords, strings.ToUpper(string(runes[start:i])))
			default:
				i++
			}
		case stString:
			if c == '\'' {
				if i+1 < n && runes[i+1] == '\'' {
					i += 2
				} else {
					state = stNormal
					i++
				}
			} else {
				i++
			}
		case stBracket:
			if c == ']' {
				if i+1 < n && runes[i+1] == ']' {
					i += 2
				} else {
					state = stNormal
					i++
				}
			} else {
				i++
			}
		case stQuotedIdent:
			if c == '"' {
				if i+1 < n && runes[i+1] == '"' {
					i += 2
				} else {
					state = stNormal
					i++
				}
			} else {
				i++
			}
		case stLineComment:
			if c == '\n' {
				state = stNormal
			}
			i++
		case stBlockComment:
			if c == '*' && i+1 < n && runes[i+1] == '/' {
				state = stNormal
				i += 2
			} else {
				i++
			}
		}
	}
	return keywords
}

func isKeywordStackedStatement(current, keyword, previous string) bool {
	if !statementStartKeywords[keyword] {
		return false
	}
	switch current {
	case "WITH":
		switch keyword {
		case "SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "WITH":
			return false
		}
	case "INSERT":
		switch keyword {
		case "SELECT", "WITH", "UPDATE", "DELETE":
			return false
		}
	case "MERGE":
		switch keyword {
		case "INSERT", "UPDATE", "DELETE", "WITH", "SELECT":
			return false
		}
	case "CREATE", "ALTER":
		switch keyword {
		case "SELECT", "WITH", "BEGIN", "RETURN", "DECLARE", "SET", "IF", "WHILE", "EXEC", "EXECUTE":
			return false
		}
	case "UPDATE":
		if keyword == "SET" {
			return false
		}
	case "SELECT":
		switch keyword {
		case "SELECT":
			return previous != "UNION" && previous != "EXCEPT" && previous != "INTERSECT"
		case "WITH":
			return false
		}
	case "EXEC", "EXECUTE":
		if keyword == "WITH" {
			return false
		}
	}
	return keyword != current || (keyword == "SELECT" && previous != "UNION" && previous != "EXCEPT" && previous != "INTERSECT")
}

func hasKeywordStackedStatements(query string) bool {
	keywords := topLevelKeywords(query)
	current := ""
	previous := ""
	for _, kw := range keywords {
		if current == "" {
			if statementStartKeywords[kw] {
				current = kw
			}
			previous = kw
			continue
		}
		if isKeywordStackedStatement(current, kw, previous) {
			return true
		}
		previous = kw
	}
	return false
}

func hasGOBatchSeparator(query string) bool {
	lines := strings.Split(query, "\n")
	for i, line := range lines {
		if !strings.EqualFold(strings.TrimSpace(stripSQLComments(line)), "GO") {
			continue
		}
		if i+1 < len(lines) && strings.TrimSpace(stripSQLComments(strings.Join(lines[i+1:], "\n"))) != "" {
			return true
		}
	}
	return false
}
