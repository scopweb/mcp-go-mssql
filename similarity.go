package main

import "strings"

// findSimilarNames returns up to 3 existing table names similar to the target.
// Uses Levenshtein-like prefix/substring matching for practical suggestions.
func findSimilarNames(target string, existing map[string]bool) []string {
	var scored []struct {
		name  string
		score int
	}

	for name := range existing {
		score := similarityScore(target, name)
		if score > 0 {
			scored = append(scored, struct {
				name  string
				score int
			}{name, score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	var suggestions []string
	limit := 3
	if len(scored) < limit {
		limit = len(scored)
	}
	for i := 0; i < limit; i++ {
		suggestions = append(suggestions, "'"+scored[i].name+"'")
	}
	return suggestions
}

// similarityScore returns a score > 0 if two table names are similar enough to suggest.
// Higher score = better match. Returns 0 if not similar enough.
func similarityScore(target, candidate string) int {
	// Exact prefix match (strongest signal)
	if strings.HasPrefix(candidate, target) || strings.HasPrefix(target, candidate) {
		return 100
	}

	// Substring match
	if strings.Contains(candidate, target) || strings.Contains(target, candidate) {
		return 80
	}

	// Levenshtein distance for short names
	dist := levenshtein(target, candidate)
	maxLen := len(target)
	if len(candidate) > maxLen {
		maxLen = len(candidate)
	}
	if maxLen == 0 {
		return 0
	}

	// Allow distance up to ~30% of the longer name
	threshold := maxLen * 30 / 100
	if threshold < 2 {
		threshold = 2
	}
	if dist <= threshold {
		return 60 - dist
	}

	return 0
}

// levenshtein computes the edit distance between two strings (lowercase comparison).
func levenshtein(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Use single-row optimization
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
