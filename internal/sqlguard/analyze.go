package sqlguard

import (
	"errors"
	"strings"
	"unicode"
)

const (
	maxCommentDepth = 64
	maxParenDepth   = 512
)

// ErrUnterminated is returned when a literal, delimited identifier, or block
// comment is not closed, or when nesting exceeds the analyzer limit. The
// error text never includes the input.
var ErrUnterminated = errors.New("unterminated SQL literal, identifier, or comment")

// Analysis is the shared lexical view of one SQL batch. Words and TopLevel
// contain uppercase executable identifiers only. Variables, literals, comments,
// and delimited identifiers are omitted. A non-nil error from Analyze means
// the input must not be authorized.
type Analysis struct {
	// Statements is the number of non-empty statements. Empty separators at
	// the start or end are ignored. An empty separator does not hide a later
	// non-empty statement.
	Statements int
	// GO is true when a sqlcmd-style GO line, with or without a count, appears
	// outside literals, delimited identifiers, and comments.
	GO bool
	// TopLevel lists executable words at parenthesis depth zero, in order.
	TopLevel []string
	// Words lists every executable word, including those nested in parentheses.
	Words []string
}

// Segment is one piece of a batch split on statement-separator semicolons.
// Text has comments replaced with spaces so adjacent tokens are not joined.
// Empty segments are preserved.
type Segment struct {
	Text       string
	WasComment bool
}

type scanState int

const (
	scanNormal scanState = iota
	scanLineComment
	scanBlockComment
)

type scanOut struct {
	tokens     []token
	semis      []int
	masked     []rune
	goSep      bool
	statements int
	topLevel   []string
	words      []string
}

// Analyze classifies query without executing it. A non-nil error is a lexical
// failure and must not be treated as a single authorized statement.
func Analyze(query string) (Analysis, error) {
	out, err := scanQuery(query)
	if err != nil {
		return Analysis{}, err
	}
	return Analysis{
		Statements: out.statements,
		GO:         out.goSep,
		TopLevel:   out.topLevel,
		Words:      out.words,
	}, nil
}

// MaskComments replaces comments with spaces and leaves every other rune in
// place. SELECT/**/1 therefore cannot become SELECT1. An unclosed construct
// returns ErrUnterminated and an empty string.
func MaskComments(query string) (string, error) {
	out, err := scanQuery(query)
	if err != nil {
		return "", err
	}
	return string(out.masked), nil
}

// SplitStatements splits query on semicolons that sit outside literals,
// delimited identifiers, and comments. Lexical errors are returned rather
// than a partial split.
func SplitStatements(query string) ([]Segment, error) {
	out, err := scanQuery(query)
	if err != nil {
		return nil, err
	}
	runes := []rune(query)
	bounds := make([]int, 0, len(out.semis)+2)
	bounds = append(bounds, 0)
	bounds = append(bounds, out.semis...)
	bounds = append(bounds, len(runes))
	segs := make([]Segment, 0, len(bounds)-1)
	for i := 0; i < len(bounds)-1; i++ {
		lo := bounds[i]
		hi := bounds[i+1]
		if i > 0 {
			lo++
		}
		if lo > hi {
			lo = hi
		}
		orig := runes[lo:hi]
		mask := out.masked[lo:hi]
		segs = append(segs, Segment{
			Text:       string(mask),
			WasComment: runesDiffer(orig, mask),
		})
	}
	return segs, nil
}

func runesDiffer(a, b []rune) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

func scanQuery(query string) (scanOut, error) {
	r := []rune(query)
	n := len(r)
	var out scanOut
	out.masked = make([]rune, n)
	copy(out.masked, r)

	state := scanNormal
	commentDepth := 0
	parenDepth := 0
	lineStart := true
	pending := false

	flushStatement := func() {
		if pending {
			out.statements++
			pending = false
		}
	}
	noteContent := func() { pending = true }
	emitWord := func(text string, variable bool) {
		noteContent()
		out.tokens = append(out.tokens, token{text: text, word: !variable, ident: true})
		if variable {
			return
		}
		upper := strings.ToUpper(text)
		out.words = append(out.words, upper)
		if parenDepth == 0 {
			out.topLevel = append(out.topLevel, upper)
		}
	}
	emitSymbol := func(c rune) {
		noteContent()
		out.tokens = append(out.tokens, token{text: string(c), symbol: true})
	}

	for i := 0; i < n; {
		c := r[i]
		switch state {
		case scanNormal:
			switch {
			case c == '\n' || c == '\r':
				lineStart = true
				i++
			case unicode.IsSpace(c):
				i++
			case c == '-' && i+1 < n && r[i+1] == '-':
				lineStart = false
				state = scanLineComment
				out.masked[i] = ' '
				out.masked[i+1] = ' '
				i += 2
			case c == '/' && i+1 < n && r[i+1] == '*':
				lineStart = false
				state = scanBlockComment
				commentDepth = 1
				out.masked[i] = ' '
				out.masked[i+1] = ' '
				i += 2
			case (c == 'N' || c == 'n') && i+1 < n && r[i+1] == '\'':
				lineStart = false
				noteContent()
				i += 2
				if err := consumeQuoted(r, &i, '\'', false, &out); err != nil {
					return scanOut{}, err
				}
			case c == '\'':
				lineStart = false
				noteContent()
				i++
				if err := consumeQuoted(r, &i, '\'', false, &out); err != nil {
					return scanOut{}, err
				}
			case c == '[':
				lineStart = false
				noteContent()
				i++
				if err := consumeQuoted(r, &i, ']', true, &out); err != nil {
					return scanOut{}, err
				}
			case c == '"':
				lineStart = false
				noteContent()
				i++
				if err := consumeQuoted(r, &i, '"', true, &out); err != nil {
					return scanOut{}, err
				}
			case unicode.IsLetter(c) || c == '_' || c == '#' || c == '@':
				start := i
				variable := c == '@'
				i++
				for i < n && isIdentContinue(r[i]) {
					i++
				}
				text := string(r[start:i])
				if lineStart && !variable && strings.EqualFold(text, "GO") && goLineSuffix(r, i) {
					out.goSep = true
				}
				lineStart = false
				emitWord(text, variable)
			case c == ';':
				out.semis = append(out.semis, i)
				flushStatement()
				out.tokens = append(out.tokens, token{text: ";", symbol: true})
				lineStart = false
				i++
			case c == '(':
				if parenDepth == maxParenDepth {
					return scanOut{}, ErrUnterminated
				}
				parenDepth++
				lineStart = false
				emitSymbol(c)
				i++
			case c == ')':
				if parenDepth > 0 {
					parenDepth--
				}
				lineStart = false
				emitSymbol(c)
				i++
			default:
				lineStart = false
				emitSymbol(c)
				i++
			}
		case scanLineComment:
			if c == '\n' {
				state = scanNormal
				lineStart = true
				i++
				continue
			}
			out.masked[i] = ' '
			i++
		case scanBlockComment:
			if c == '/' && i+1 < n && r[i+1] == '*' {
				if commentDepth == maxCommentDepth {
					return scanOut{}, ErrUnterminated
				}
				commentDepth++
				out.masked[i] = ' '
				out.masked[i+1] = ' '
				i += 2
				continue
			}
			if c == '*' && i+1 < n && r[i+1] == '/' {
				out.masked[i] = ' '
				out.masked[i+1] = ' '
				i += 2
				commentDepth--
				if commentDepth == 0 {
					state = scanNormal
				}
				continue
			}
			if c != '\n' && c != '\r' {
				out.masked[i] = ' '
			}
			i++
		default:
			return scanOut{}, ErrUnterminated
		}
	}
	if state == scanLineComment {
		state = scanNormal
	}
	if state != scanNormal {
		return scanOut{}, ErrUnterminated
	}
	flushStatement()
	return out, nil
}

func consumeQuoted(r []rune, i *int, close rune, ident bool, out *scanOut) error {
	n := len(r)
	var b strings.Builder
	for *i < n {
		c := r[*i]
		if c == close {
			if *i+1 < n && r[*i+1] == close {
				b.WriteRune(close)
				*i += 2
				continue
			}
			*i++
			out.tokens = append(out.tokens, token{text: b.String(), ident: ident})
			return nil
		}
		b.WriteRune(c)
		*i++
	}
	return ErrUnterminated
}

func goLineSuffix(r []rune, i int) bool {
	n := len(r)
	for i < n && isInlineSpace(r[i]) {
		i++
	}
	if i < n && r[i] >= '0' && r[i] <= '9' {
		for i < n && r[i] >= '0' && r[i] <= '9' {
			i++
		}
		if i < n && isIdentContinue(r[i]) {
			return false
		}
	}
	for i < n && r[i] != '\n' && r[i] != '\r' {
		if isInlineSpace(r[i]) {
			i++
			continue
		}
		if r[i] == '-' && i+1 < n && r[i+1] == '-' {
			return true
		}
		if r[i] == '/' && i+1 < n && r[i+1] == '*' {
			end, ok := skipBlock(r, i)
			if !ok {
				return false
			}
			i = end
			continue
		}
		return false
	}
	return true
}

func skipBlock(r []rune, i int) (int, bool) {
	n := len(r)
	i += 2
	depth := 1
	for i < n && depth > 0 {
		if i+1 < n && r[i] == '/' && r[i+1] == '*' {
			if depth == maxCommentDepth {
				return i, false
			}
			depth++
			i += 2
			continue
		}
		if i+1 < n && r[i] == '*' && r[i+1] == '/' {
			depth--
			i += 2
			continue
		}
		i++
	}
	if depth != 0 {
		return i, false
	}
	return i, true
}

func isInlineSpace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\v' || c == '\f'
}

func isIdentContinue(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '$' || c == '#' || c == '@'
}

// ExecutableHas reports whether words contains want as a whole token.
func ExecutableHas(words []string, want string) bool {
	want = strings.ToUpper(want)
	for _, w := range words {
		if w == want {
			return true
		}
	}
	return false
}

// ExecutableContains reports whether any executable word contains needle.
// The original SQL text is not inspected.
func ExecutableContains(words []string, needle string) bool {
	needle = strings.ToUpper(needle)
	if needle == "" {
		return false
	}
	for _, w := range words {
		if strings.Contains(w, needle) {
			return true
		}
	}
	return false
}
