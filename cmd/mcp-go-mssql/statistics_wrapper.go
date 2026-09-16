package main

import (
	"errors"
	"regexp"
	"strings"
)

// statisticsWrapper is the parsed form of a query that is a SET STATISTICS
// (TIME|IO) (ON|OFF) wrapper around a single base statement.
//
// The wrapper is the only multi-statement form allowed in read-only mode. It
// is intentionally narrow: the wrapper can have at most one leading
// "SET STATISTICS TIME/IO ON/OFF;" of each kind and at most one trailing
// "SET STATISTICS TIME/IO ON/OFF;" of each kind, and the base statement must
// itself be a single statement (no internal ';' separators).
//
// A query like
//
//	SET STATISTICS TIME ON; SELECT 1; SET STATISTICS IO OFF;
//
// is accepted: the leading edge has TIME ON, the trailing edge has IO OFF.
// The base is "SELECT 1".
type statisticsWrapper struct {
	// BaseStatement is the wrapped statement with the leading and trailing
	// SET wrappers removed. It is a single SQL statement; it has already
	// been confirmed to contain no internal ';' separators.
	BaseStatement string

	// TimeOn / TimeOff / IOOn / IOOff report the net effect of the wrapper
	// tokens that were applied. A kind (TIME/IO) cannot appear as both ON
	// and OFF in the wrapper (this would be nonsensical: it would turn the
	// statistics on and immediately off before the base ran).
	TimeOn  bool
	TimeOff bool
	IOOn    bool
	IOOff   bool
}

// ErrNotAWrapper is returned by parseStatisticsWrapper when the input is not
// in the accepted wrapper shape. The validator treats this as "not a wrapper,
// fall through to normal read-only checks".
var ErrNotAWrapper = errors.New("query is not a SET STATISTICS wrapper")

// setStatsRe matches a single SET STATISTICS wrapper token (with or without
// a trailing semicolon) and captures the kind (TIME|IO) and value (ON|OFF).
//
// Anchored with ^ and $ so a multi-token input cannot match. The (?i) flag
// makes the match case-insensitive. The leading/trailing \s* tolerates any
// whitespace at the edges. (?s) makes '.' match newlines, which means a
// token may contain line/block comments inside it (handled by the split).
var setStatsRe = regexp.MustCompile(`(?is)^\s*SET\s+STATISTICS\s+(TIME|IO)\s+(ON|OFF)\s*;?\s*$`)

// setStatsStartRe matches the beginning of a SET STATISTICS token (with
// optional leading whitespace). It is used to reject malformed tokens
// like "SET STATISTICS TIME ON SELECT 1" (missing ';'): such a token
// starts with the wrapper prefix but does not match setStatsRe, so it
// cannot be a base statement either.
var setStatsStartRe = regexp.MustCompile(`(?is)^\s*SET\s+STATISTICS\b`)

// wrapperEdge tracks which wrapper tokens have been seen on a single edge
// (leading or trailing) of the base statement.
type wrapperEdge struct {
	TimeOn  bool
	TimeOff bool
	IOOn    bool
	IOOff   bool
}

// edgeKind identifies which edge of the base a wrapper token is on.
type edgeKind int

const (
	edgeLeading edgeKind = iota
	edgeTrailing
)

// parseStatisticsWrapper reports whether query is a SET STATISTICS wrapper
// around a single base statement. On success it returns a *statisticsWrapper
// describing the net effect and the base statement. On any other input shape
// it returns (nil, ErrNotAWrapper); the caller should treat that as "not a
// wrapper" and continue with normal validation.
//
// The function requires at least one SET STATISTICS token; a plain SELECT or
// any other query that has no wrapper returns ErrNotAWrapper.
//
// Fast path: a wrapper must contain the literal "SET STATISTICS" substring
// somewhere in the input. We check this up front (case-insensitive) so the
// common case of "any normal query" returns in O(1) without entering the
// segment loop. The check is on the substring, not on the first rune, so
// "base-first" forms (e.g. "WITH ... SELECT ...; SET STATISTICS TIME ON;")
// are not accidentally rejected — only queries that contain no SET
// STATISTICS token at all short-circuit out.
func parseStatisticsWrapper(query string) (*statisticsWrapper, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, ErrNotAWrapper
	}
	// Fast path: substring check for the wrapper keyword. We do a
	// literal substring search (case-insensitive) rather than checking
	// the first non-whitespace rune, because a wrapper can appear at
	// any position — leading, trailing, or interleaved with the base.
	// Plain queries like "SELECT 1", "DELETE FROM x", or
	// "WITH cte AS ..." that never mention SET STATISTICS still
	// short-circuit here in O(query length).
	if !containsSetStatistics(trimmed) {
		return nil, ErrNotAWrapper
	}

	leading := wrapperEdge{}
	trailing := wrapperEdge{}
	var base string
	// The parser walks the input and at each ';' boundary classifies the
	// The parser walks the input and at each ';' boundary classifies
	// the just-completed segment. The current "logical" edge is tracked
	// via two booleans: pendingComment (a comment-only segment was just
	// seen, so the next wrapper if any goes to the trailing edge) and
	// sawBase (the base has been seen, so subsequent wrappers go to the
	// trailing edge).
	sawBase := false
	sawAnySetStats := false
	// pendingComment is true if the just-completed segment was a
	// comment-only segment that is being deferred for the next
	// wrapper to be classified as trailing. (E.g. between two SET
	// tokens separated by "; -- off\n"; the comment is "pending".)
	pendingComment := false

	segments := splitOnStatementSeparators(trimmed)
	for _, seg := range segments {
		text := strings.TrimSpace(seg.text)
		if text == "" {
			// Empty (trimmed) segment. Distinguish:
			//   * wasComment=true: comment-only segment (e.g.
			//     "; -- off\nSET STATISTICS TIME OFF;"), allowed and
			//     sets pendingComment=true so the next wrapper (if
			//     any) is classified as trailing.
			//   * wasComment=false and BEFORE the base: pure
			//     whitespace between two ';'s (e.g. "; ;"), which is
			//     an empty statement — reject as a malformed wrapper.
			//   * wasComment=false and AFTER the base: trailing
			//     whitespace after a benign trailing ';' — accept
			//     silently.
			if seg.wasComment {
				pendingComment = true
				continue
			}
			if !sawBase {
				return nil, ErrNotAWrapper
			}
			continue
		}
		m := setStatsRe.FindStringSubmatch(text)
		if m != nil {
			// SET STATISTICS wrapper token. Decide the edge.
			kind := strings.ToUpper(m[1])
			val := strings.ToUpper(m[2])
			edge := &leading
			// Switch to trailing if: base seen, OR there was a
			// pending comment (the previous non-empty content was a
			// wrapper, with a comment between), OR this SET token's
			// segment itself contains a leading comment (which means
			// the comment separated this SET from the previous one,
			// so this SET is logically the trailing wrapper).
			if sawBase || pendingComment || seg.wasComment {
				edge = &trailing
			}
			// Disallow duplicates across edges: applying TIME ON
			// twice (once leading, once trailing) is nonsensical and
			// is rejected here. The same applies to TIME OFF, IO ON,
			// and IO OFF.
			if kind == "TIME" && val == "ON" && (leading.TimeOn || trailing.TimeOn) {
				return nil, ErrNotAWrapper
			}
			if kind == "TIME" && val == "OFF" && (leading.TimeOff || trailing.TimeOff) {
				return nil, ErrNotAWrapper
			}
			if kind == "IO" && val == "ON" && (leading.IOOn || trailing.IOOn) {
				return nil, ErrNotAWrapper
			}
			if kind == "IO" && val == "OFF" && (leading.IOOff || trailing.IOOff) {
				return nil, ErrNotAWrapper
			}
			if !applyWrapperEdge(edge, kind, val) {
				return nil, ErrNotAWrapper
			}
			sawAnySetStats = true
			pendingComment = false
			continue
		}
		// Not a wrapper token. It must be the base.
		if sawBase {
			return nil, ErrNotAWrapper
		}
		// If it starts with the SET STATISTICS prefix but doesn't
		// match the full token, reject — that's a malformed wrapper
		// (e.g. "SET STATISTICS TIME ON SELECT 1" with missing ';').
		if setStatsStartRe.MatchString(text) {
			return nil, ErrNotAWrapper
		}
		base = text
		if hasMultipleStatements(base) {
			return nil, ErrNotAWrapper
		}
		sawBase = true
		pendingComment = false
	}

	if !sawBase || !sawAnySetStats {
		return nil, ErrNotAWrapper
	}
	return &statisticsWrapper{
		BaseStatement: base,
		TimeOn:        leading.TimeOn || trailing.TimeOn,
		TimeOff:       leading.TimeOff || trailing.TimeOff,
		IOOn:          leading.IOOn || trailing.IOOn,
		IOOff:         leading.IOOff || trailing.IOOff,
	}, nil
}

// applyWrapperEdge records a SET STATISTICS wrapper token on the given edge
// and returns false if the token is invalid for that edge (duplicate, or
// conflicting ON/OFF for the same kind).
func applyWrapperEdge(edge *wrapperEdge, kind, val string) bool {
	on := val == "ON"
	switch kind {
	case "TIME":
		if on {
			if edge.TimeOn {
				return false
			}
			if edge.TimeOff {
				return false
			}
			edge.TimeOn = true
		} else {
			if edge.TimeOff {
				return false
			}
			if edge.TimeOn {
				return false
			}
			edge.TimeOff = true
		}
	case "IO":
		if on {
			if edge.IOOn {
				return false
			}
			if edge.IOOff {
				return false
			}
			edge.IOOn = true
		} else {
			if edge.IOOff {
				return false
			}
			if edge.IOOn {
				return false
			}
			edge.IOOff = true
		}
	default:
		return false
	}
	return true
}

// containsSetStatistics reports whether s contains the case-insensitive
// SET STATISTICS keyword pair with any amount of ASCII whitespace
// between the two tokens. It is used by the fast-path in
// parseStatisticsWrapper to short-circuit plain queries that never
// mention SET STATISTICS. The check is intentionally lenient: any
// whitespace-separated occurrence of the two tokens counts, because
// the segment loop applies the strict regex `setStatsRe` on each
// candidate segment afterwards. This fast-path only avoids the
// segment walk for inputs that clearly cannot be wrappers.
func containsSetStatistics(s string) bool {
	lower := strings.ToLower(s)
	// Look for "set" followed by at least one ASCII whitespace run
	// followed by "statistics". Use a small regex rather than
	// strings.Contains so we tolerate tabs and multiple spaces.
	return setStatsKeywordRe.MatchString(lower)
}

// setStatsKeywordRe is a relaxed pre-filter for the SET STATISTICS
// keyword. It accepts any amount of ASCII whitespace between "set"
// and "statistics" and is matched case-insensitively (the caller is
// expected to pass a lower-cased string). The strict per-segment
// match (with kind + value) is done by setStatsRe later.
var setStatsKeywordRe = regexp.MustCompile(`\bset\s+statistics\b`)

// sqlSegment is one piece of a SQL batch, produced by splitOnStatementSeparators.
// text is the segment content with comments stripped (comments replaced
// with spaces). wasComment is true if the original segment had comment
// content (so the parser can distinguish "empty (was whitespace)" from
// "empty (was comment)").
type sqlSegment struct {
	text       string
	wasComment bool
}

// splitOnStatementSeparators splits a SQL batch on ';' characters that act
// as statement separators, honoring the same single-quote / bracket /
// comment rules as hasMultipleStatements. The returned slice preserves
// zero-length segments (so an empty statement like ";" between two others
// is detectable) and preserves whitespace/comment content for the caller
// to classify.
func splitOnStatementSeparators(query string) []sqlSegment {
	const (
		stNormal = iota
		stString
		stBracket
		stLineComment
		stBlockComment
	)
	state := stNormal
	runes := []rune(query)
	n := len(runes)
	var segs []sqlSegment
	segStart := 0
	// We track the original text of each segment separately so we can
	// detect comment-only segments after the comment-stripping pass.
	flush := func(lo, hi int) {
		original := string(runes[lo:hi])
		stripped := stripSQLComments(original)
		segs = append(segs, sqlSegment{text: stripped, wasComment: originalHasComment(original, stripped)})
	}
	for i := 0; i < n; i++ {
		c := runes[i]
		switch state {
		case stNormal:
			switch {
			case c == '\'':
				state = stString
			case c == '[':
				state = stBracket
			case c == '-' && i+1 < n && runes[i+1] == '-':
				state = stLineComment
				i++
			case c == '/' && i+1 < n && runes[i+1] == '*':
				state = stBlockComment
				i++
			case c == ';':
				flush(segStart, i)
				segStart = i + 1
			}
		case stString:
			if c == '\'' {
				if i+1 < n && runes[i+1] == '\'' {
					i++
				} else {
					state = stNormal
				}
			}
		case stBracket:
			if c == ']' {
				if i+1 < n && runes[i+1] == ']' {
					i++
				} else {
					state = stNormal
				}
			}
		case stLineComment:
			if c == '\n' {
				state = stNormal
			}
		case stBlockComment:
			if c == '*' && i+1 < n && runes[i+1] == '/' {
				state = stNormal
				i++
			}
		}
	}
	flush(segStart, n)
	return segs
}

// originalHasComment reports whether the segment was a comment-only
// segment. The stripped version is the original with comments replaced
// by spaces; if the original had at least one non-whitespace, non-comment
// character, then the original and stripped would differ in the count
// of those characters. We detect a comment-only segment by checking
// whether the original was non-empty but contains no non-whitespace,
// non-comment content.
//
// In practice the simplest check: if the original text and the stripped
// text differ in any rune, then comments were present.
func originalHasComment(original, stripped string) bool {
	if len(original) != len(stripped) {
		return true
	}
	for i := 0; i < len(original); i++ {
		if original[i] != stripped[i] {
			return true
		}
	}
	return false
}

// stripSQLComments removes line and block SQL comments from s, preserving
// string literals (single-quoted) and bracketed identifiers ([...]) as
// opaque runs of characters. Comments are replaced with spaces so the
// output has the same length as the input, which makes position-based
// diagnostics stable. Whitespace inside comments is preserved so the
// caller can distinguish "empty (was whitespace)" from "empty (was
// comment)".
func stripSQLComments(s string) string {
	const (
		stNormal = iota
		stString
		stBracket
		stLineComment
		stBlockComment
	)
	state := stNormal
	runes := []rune(s)
	n := len(runes)
	out := make([]rune, n)
	copy(out, runes)
	for i := 0; i < n; i++ {
		c := out[i]
		switch state {
		case stNormal:
			switch {
			case c == '\'':
				state = stString
			case c == '[':
				state = stBracket
			case c == '-' && i+1 < n && out[i+1] == '-':
				state = stLineComment
				out[i] = ' '
				out[i+1] = ' '
				i++
			case c == '/' && i+1 < n && out[i+1] == '*':
				state = stBlockComment
				out[i] = ' '
				out[i+1] = ' '
				i++
			}
		case stString:
			if c == '\'' {
				if i+1 < n && out[i+1] == '\'' {
					i++
				} else {
					state = stNormal
				}
			}
		case stBracket:
			if c == ']' {
				if i+1 < n && out[i+1] == ']' {
					i++
				} else {
					state = stNormal
				}
			}
		case stLineComment:
			if c == '\n' {
				state = stNormal
			} else {
				out[i] = ' '
			}
		case stBlockComment:
			if c == '*' && i+1 < n && out[i+1] == '/' {
				state = stNormal
				out[i] = ' '
				out[i+1] = ' '
				i++
			} else if c != '\n' {
				out[i] = ' '
			}
		}
	}
	return string(out)
}
