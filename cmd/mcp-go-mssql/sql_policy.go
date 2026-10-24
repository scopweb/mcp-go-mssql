package main

import "mcp-go-mssql/internal/sqlguard"

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
	a, err := sqlguard.Analyze(query)
	if err != nil {
		return true
	}
	return stackedWords(a.TopLevel)
}

func stackedWords(keywords []string) bool {
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
		// UNION ALL is one set operator; an arbitrary ALL must not
		// exempt a subsequent SELECT from the stacked-statement guard.
		if kw == "ALL" && previous == "UNION" {
			previous = "UNION ALL"
			continue
		}
		if kw == "SELECT" && previous == "UNION ALL" {
			previous = "UNION"
		}
		if isKeywordStackedStatement(current, kw, previous) {
			return true
		}
		previous = kw
	}
	return false
}
