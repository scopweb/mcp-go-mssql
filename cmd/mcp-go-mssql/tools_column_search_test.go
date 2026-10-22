package main

// Tests for the query that finds the columns of the catalog.

import (
	"strings"
	"testing"
)

// TestEscapeLikePattern verifies that the search term is matched literally.
func TestEscapeLikePattern(t *testing.T) {
	cases := map[string]string{
		"ClienteId":  `ClienteId`,
		"100%":       `100\%`,
		"a_b":        `a\_b`,
		"a[b]":       `a\[b]`,
		`back\slash`: `back\\slash`,
		"%_\\":       `\%\_\\`,
	}
	for input, want := range cases {
		if got := escapeLikePattern(input); got != want {
			t.Errorf("escapeLikePattern(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestBuildColumnSearchQueryParameterizesEverything verifies that no user
// input reaches the query text.
func TestBuildColumnSearchQueryParameterizesEverything(t *testing.T) {
	query, args := buildColumnSearchQuery("", "", "ClienteId", 50)

	if strings.Contains(query, "ClienteId") {
		t.Error("the search term must be a parameter, not query text")
	}
	if !strings.Contains(query, "@p1") || !strings.Contains(query, "@p2") {
		t.Error("the limit and the term must be parameters")
	}
	if len(args) != 2 {
		t.Fatalf("args = %v, want the limit and the term", args)
	}
	if args[0] != 51 {
		t.Errorf("limit argument = %v, want 51 (one row more to detect more results)", args[0])
	}
	if args[1] != "%ClienteId%" {
		t.Errorf("term argument = %v, want %%ClienteId%%", args[1])
	}
}

// TestBuildColumnSearchQueryWithFilters verifies the optional filters.
func TestBuildColumnSearchQueryWithFilters(t *testing.T) {
	query, args := buildColumnSearchQuery("dbo", "Pedido", "Cliente", 20)

	if len(args) != 4 {
		t.Fatalf("args = %v, want limit, term, schema and table", args)
	}
	if args[2] != "dbo" {
		t.Errorf("schema argument = %v, want dbo", args[2])
	}
	if args[3] != "%Pedido%" {
		t.Errorf("table argument = %v, want %%Pedido%%", args[3])
	}
	if !strings.Contains(query, "c.TABLE_SCHEMA = @p3") {
		t.Error("the schema filter must be an exact comparison")
	}
	if !strings.Contains(query, "c.TABLE_NAME LIKE @p4") {
		t.Error("the table filter must be a parameterized comparison")
	}
	if strings.Contains(query, "dbo") || strings.Contains(query, "Pedido") {
		t.Error("the filters must not be inlined in the query text")
	}
}

// TestBuildColumnSearchQueryIsDeterministic verifies the ordering of the
// result, so the same call always returns the same rows in the same order.
func TestBuildColumnSearchQueryIsDeterministic(t *testing.T) {
	query, _ := buildColumnSearchQuery("", "", "id", 50)
	if !strings.Contains(query, "ORDER BY c.TABLE_SCHEMA, c.TABLE_NAME, c.ORDINAL_POSITION") {
		t.Error("the column search must have a deterministic order")
	}
}

// TestBuildColumnSearchQueryIsCatalogOnly verifies the query only reads
// catalog metadata.
func TestBuildColumnSearchQueryIsCatalogOnly(t *testing.T) {
	query, _ := buildColumnSearchQuery("", "", "id", 50)
	if !strings.Contains(query, "FROM INFORMATION_SCHEMA.COLUMNS c") {
		t.Error("the column search must read INFORMATION_SCHEMA.COLUMNS")
	}
	for _, forbidden := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "MERGE"} {
		if strings.Contains(strings.ToUpper(query), forbidden) {
			t.Errorf("the column search must not contain %s", forbidden)
		}
	}
}
