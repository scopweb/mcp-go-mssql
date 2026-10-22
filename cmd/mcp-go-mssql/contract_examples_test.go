package main

// Tests for the examples of the agent contract.
//
// The examples are part of the contract, so they must not become fiction:
// the CI unmarshals them into the real types of the server. When a result
// field changes, the example has to change with it or the build fails.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contractExample is one example of the contract and the type it must match.
type contractExample struct {
	marker string
	// kind is "query" for a query_database result and "search" for the
	// answer of a search.
	kind string
	// required are the keys the example must carry.
	required []string
}

// contractExamples are the examples the contract must carry.
var contractExamples = []contractExample{
	{
		marker:   "example:query-result",
		kind:     "query",
		required: []string{"rows", "row_count", "truncated", "rows_truncated", "limits", "stats"},
	},
	{
		marker:   "example:truncated-result",
		kind:     "query",
		required: []string{"rows", "truncated", "rows_truncated", "cells_truncated", "response_truncated", "truncated_cells", "limits"},
	},
	{
		marker:   "example:search-result",
		kind:     "search",
		required: []string{"matches", "returned", "truncated", "limits", "next_steps"},
	},
}

// contractPath is the document that carries the examples.
var contractPath = filepath.Join("docs", "AGENT_CONTRACT.md")

// TestContractExamplesMatchTheServerTypes verifies that every example of the
// contract is a valid instance of the type it documents.
func TestContractExamplesMatchTheServerTypes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repoRoot(), contractPath))
	if err != nil {
		t.Fatalf("cannot read the agent contract: %v", err)
	}
	contract := string(content)

	for _, example := range contractExamples {
		block, ok := extractJSONBlockAfter(contract, example.marker)
		if !ok {
			t.Errorf("the contract has no JSON example after %q", example.marker)
			continue
		}

		switch example.kind {
		case "query":
			var result QueryResult
			if err := json.Unmarshal([]byte(block), &result); err != nil {
				t.Errorf("the example %q is not a valid query result: %v", example.marker, err)
				continue
			}
			if result.Limits.MaxRows != maxQueryRows {
				t.Errorf("the example %q reports max_rows %d, want %d", example.marker, result.Limits.MaxRows, maxQueryRows)
			}
		case "search":
			var result searchResult
			if err := json.Unmarshal([]byte(block), &result); err != nil {
				t.Errorf("the example %q is not a valid search result: %v", example.marker, err)
				continue
			}
			if result.Limits.MaxObjects == 0 {
				t.Errorf("the example %q does not report the limit of the search", example.marker)
			}
		}

		var fields map[string]interface{}
		if err := json.Unmarshal([]byte(block), &fields); err != nil {
			t.Errorf("the example %q is not valid JSON: %v", example.marker, err)
			continue
		}
		for _, key := range example.required {
			if _, ok := fields[key]; !ok {
				t.Errorf("the example %q is missing the key %q", example.marker, key)
			}
		}
	}
}

// extractJSONBlockAfter returns the first JSON block that follows the marker.
func extractJSONBlockAfter(content, marker string) (string, bool) {
	index := strings.Index(content, marker)
	if index < 0 {
		return "", false
	}
	rest := content[index:]

	start := strings.Index(rest, "```json")
	if start < 0 {
		return "", false
	}
	rest = rest[start+len("```json"):]

	end := strings.Index(rest, "```")
	if end < 0 {
		return "", false
	}
	return strings.TrimSpace(rest[:end]), true
}
