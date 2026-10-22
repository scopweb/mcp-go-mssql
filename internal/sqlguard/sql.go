// Package sqlguard provides conservative lexical checks for SQL authorization.
// It is defense in depth; database grants remain the security boundary.
package sqlguard

import (
	"fmt"
	"strings"
	"unicode"
)

type token struct {
	text   string
	word   bool
	ident  bool
	symbol bool
}

func lex(query string) ([]token, error) {
	r := []rune(query)
	var out []token
	for i := 0; i < len(r); {
		switch {
		case unicode.IsSpace(r[i]):
			i++
		case i+1 < len(r) && r[i] == '-' && r[i+1] == '-':
			for i < len(r) && r[i] != '\n' {
				i++
			}
		case i+1 < len(r) && r[i] == '/' && r[i+1] == '*':
			i += 2
			depth := 1
			for i < len(r) && depth > 0 {
				if i+1 < len(r) && r[i] == '/' && r[i+1] == '*' {
					depth++
					i += 2
				} else if i+1 < len(r) && r[i] == '*' && r[i+1] == '/' {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("unterminated SQL comment")
			}
		case r[i] == '\'' || r[i] == '"' || r[i] == '[':
			open := r[i]
			close := open
			if open == '[' {
				close = ']'
			}
			i++
			var b strings.Builder
			closed := false
			for i < len(r) {
				if r[i] == close {
					if i+1 < len(r) && r[i+1] == close {
						b.WriteRune(close)
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				b.WriteRune(r[i])
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated SQL quote")
			}
			out = append(out, token{text: b.String(), ident: open != '\''})
		case unicode.IsLetter(r[i]) || r[i] == '_' || r[i] == '#' || r[i] == '@':
			start := i
			i++
			for i < len(r) && (unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) || strings.ContainsRune("_$#@", r[i])) {
				i++
			}
			out = append(out, token{text: string(r[start:i]), word: true, ident: true})
		default:
			out = append(out, token{text: string(r[i]), symbol: true})
			i++
		}
	}
	return out, nil
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
