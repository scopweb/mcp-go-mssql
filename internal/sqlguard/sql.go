// Package sqlguard provides conservative lexical checks for SQL authorization.
// It is defense in depth; database grants remain the security boundary.
package sqlguard

import (
	"fmt"
	"strings"
)

type token struct {
	text   string
	word   bool
	ident  bool
	symbol bool
}

func lex(query string) ([]token, error) {
	out, err := scanQuery(query)
	if err != nil {
		return nil, err
	}
	return out.tokens, nil
}

func keyword(t token, want string) bool { return t.word && strings.EqualFold(t.text, want) }

// HasSelectInto recognizes INTO independently of the destination's quoting.
// Malformed lexical input fails closed. Quoted text never becomes a keyword.
func HasSelectInto(query string) bool {
	tokens, err := lex(query)
	if err != nil {
		return true
	}
	seenSelect := false
	for _, t := range tokens {
		if keyword(t, "SELECT") {
			seenSelect = true
		}
		if seenSelect && keyword(t, "INTO") {
			return true
		}
	}
	return false
}

// WriteTarget extracts the target of INSERT (including optional INTO and TOP)
// or SELECT_INTO. Unsupported/ambiguous references are rejected, not guessed.
// The result uses the same simple schema-qualified spelling as table allowlists.
func WriteTarget(query, operation string) (string, error) {
	tokens, err := lex(query)
	if err != nil {
		return "", err
	}
	want := "INSERT"
	if operation == "SELECT_INTO" {
		want = "INTO"
	}
	i := 0
	for i < len(tokens) && !keyword(tokens[i], want) {
		i++
	}
	i++
	if operation == "INSERT" && i < len(tokens) && keyword(tokens[i], "TOP") {
		i++
		if i >= len(tokens) || !tokens[i].symbol || tokens[i].text != "(" {
			return "", fmt.Errorf("unsupported INSERT TOP target")
		}
		depth := 0
		for ; i < len(tokens); i++ {
			if tokens[i].text == "(" && tokens[i].symbol {
				depth++
			}
			if tokens[i].text == ")" && tokens[i].symbol {
				depth--
				if depth == 0 {
					i++
					break
				}
			}
		}
		if i < len(tokens) && keyword(tokens[i], "PERCENT") {
			i++
		}
	}
	if operation == "INSERT" && i < len(tokens) && keyword(tokens[i], "INTO") {
		i++
	}
	var parts []string
	for {
		if i >= len(tokens) || !tokens[i].ident || tokens[i].text == "" {
			return "", fmt.Errorf("could not determine write target")
		}
		part := tokens[i].text
		// The legacy allowlist normalizer cannot safely represent escaped dots
		// or brackets inside a component. Deny those forms rather than aliasing them.
		if strings.ContainsAny(part, ".[]\"@#") {
			return "", fmt.Errorf("unsupported write target identifier")
		}
		parts = append(parts, part)
		i++
		if i >= len(tokens) || tokens[i].text != "." || !tokens[i].symbol {
			break
		}
		i++
	}
	if len(parts) > 3 {
		return "", fmt.Errorf("linked-server write targets are not supported")
	}
	return strings.Join(parts, "."), nil
}
