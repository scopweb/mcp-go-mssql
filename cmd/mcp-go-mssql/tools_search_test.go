package main

// Tests for the unified search of the explore tool.
//
// The queries are built by pure functions, so the ranking, the escaping, the
// grouping of the foreign keys and the shape of the answer can be verified
// without a database.

import (
	"strings"
	"testing"
)

// TestBuildObjectSearchQueryRanksTheMatches verifies that the kind of match is
// computed in the query, so the ranking happens before the limit.
func TestBuildObjectSearchQueryRanksTheMatches(t *testing.T) {
	query, args := buildObjectSearchQuery("Ped", 30)

	if !strings.Contains(query, "match_kind") {
		t.Error("the query must report the kind of match")
	}
	if !strings.Contains(query, "WHEN o.name = @p2 THEN 'exact'") {
		t.Error("the query must detect an exact match")
	}
	if !strings.Contains(query, "WHEN o.name LIKE @p3 ESCAPE '\\' THEN 'prefix'") {
		t.Error("the query must detect a prefix match")
	}
	if !strings.Contains(query, "ORDER BY CASE") {
		t.Error("the query must order by the kind of match")
	}
	if !strings.Contains(query, "ESCAPE '\\'") {
		t.Error("the term must be matched literally")
	}

	if len(args) != 4 {
		t.Fatalf("args = %v, want the limit, the term, the prefix and the pattern", args)
	}
	if args[0] != 31 {
		t.Errorf("limit argument = %v, want 31 (one row more to detect more results)", args[0])
	}
	if args[1] != "Ped" {
		t.Errorf("term argument = %v, want Ped", args[1])
	}
	if args[2] != "Ped%" {
		t.Errorf("prefix argument = %v, want Ped%%", args[2])
	}
	if args[3] != "%Ped%" {
		t.Errorf("pattern argument = %v, want %%Ped%%", args[3])
	}
}

// TestBuildObjectSearchQueryEscapesTheTerm verifies that a wildcard of the
// term is not a wildcard of the query.
func TestBuildObjectSearchQueryEscapesTheTerm(t *testing.T) {
	_, args := buildObjectSearchQuery("a_b", 10)
	if args[2] != `a\_b%` {
		t.Errorf("prefix argument = %v, want the escaped term", args[2])
	}
	if args[3] != `%a\_b%` {
		t.Errorf("pattern argument = %v, want the escaped term", args[3])
	}
}

// TestBuildDefinitionSearchQuery verifies the query of the definition mode.
func TestBuildDefinitionSearchQuery(t *testing.T) {
	query, args := buildDefinitionSearchQuery("Pedido", 20)

	if !strings.Contains(query, "FROM sys.sql_modules m") {
		t.Error("the definition search must read sys.sql_modules")
	}
	if !strings.Contains(query, "ESCAPE '\\'") {
		t.Error("the term must be matched literally")
	}
	if len(args) != 2 {
		t.Fatalf("args = %v, want the limit and the pattern", args)
	}
	if args[0] != 21 {
		t.Errorf("limit argument = %v, want 21", args[0])
	}
}

// TestBuildForeignKeysQuery verifies the query that resolves the foreign keys
// of the matched tables.
func TestBuildForeignKeysQuery(t *testing.T) {
	query, args := buildForeignKeysQuery("dbo", []string{"Pedido", "PedidoDet"})

	if !strings.Contains(query, "FROM sys.foreign_keys fk") {
		t.Error("the query must read sys.foreign_keys")
	}
	if !strings.Contains(query, "IN (@p2, @p3)") {
		t.Errorf("the tables must be parameters: %s", query)
	}
	if len(args) != 3 {
		t.Fatalf("args = %v, want the schema and the tables", args)
	}
	if args[0] != "dbo" || args[1] != "Pedido" || args[2] != "PedidoDet" {
		t.Errorf("args = %v, want the schema and the tables", args)
	}

	if query, _ := buildForeignKeysQuery("dbo", nil); query != "" {
		t.Errorf("query = %q, want empty without tables", query)
	}
}

// TestGroupForeignKeys verifies that a composite key stays together.
func TestGroupForeignKeys(t *testing.T) {
	rows := []map[string]interface{}{
		{"from_table": "Pedido", "from_column": "ClienteId", "to_schema": "dbo", "to_table": "Cliente", "to_column": "Id"},
		{"from_table": "Pedido", "from_column": "EmpresaId", "to_schema": "dbo", "to_table": "Empresa", "to_column": "Id"},
		{"from_table": "Pedido", "from_column": "ZonaId", "to_schema": "dbo", "to_table": "Zona", "to_column": "Id"},
	}

	keys := groupForeignKeys(rows)
	if len(keys) != 3 {
		t.Fatalf("keys = %d, want 3", len(keys))
	}
	if keys[0].ToTable != "dbo.Cliente" {
		t.Errorf("to table = %q, want dbo.Cliente", keys[0].ToTable)
	}
	if len(keys[0].FromColumns) != 1 || keys[0].FromColumns[0] != "ClienteId" {
		t.Errorf("from columns = %v, want ClienteId", keys[0].FromColumns)
	}
	if len(keys[0].ToColumns) != 1 || keys[0].ToColumns[0] != "Id" {
		t.Errorf("to columns = %v, want Id", keys[0].ToColumns)
	}
}

// TestGroupColumnsByTable verifies that the columns are grouped by table.
func TestGroupColumnsByTable(t *testing.T) {
	rows := []map[string]interface{}{
		{"schema_name": "dbo", "table_name": "Pedido", "column_name": "ClienteId", "data_type": "int", "max_length": int64(4)},
		{"schema_name": "dbo", "table_name": "Pedido", "column_name": "EmpresaId", "data_type": "int", "max_length": int64(4)},
		{"schema_name": "dbo", "table_name": "PedidoDet", "column_name": "ClienteId", "data_type": "int", "max_length": int64(4)},
	}

	matches := groupColumnsByTable(rows, "Cliente")
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want 2 tables", len(matches))
	}
	if matches[0].QualifiedName != "dbo.Pedido" {
		t.Errorf("qualified name = %q, want dbo.Pedido", matches[0].QualifiedName)
	}
	if len(matches[0].Columns) != 2 {
		t.Errorf("columns = %d, want 2", len(matches[0].Columns))
	}
	if matches[0].Columns[0].Name != "ClienteId" {
		t.Errorf("column = %q, want ClienteId", matches[0].Columns[0].Name)
	}
	if matches[0].Columns[0].MaxLength == nil || *matches[0].Columns[0].MaxLength != 4 {
		t.Errorf("max length = %v, want 4", matches[0].Columns[0].MaxLength)
	}
	if matches[0].Match.Field != matchFieldColumn {
		t.Errorf("match field = %q, want %q", matches[0].Match.Field, matchFieldColumn)
	}
}

// TestSortMatches verifies the ranking: exact, then prefix, then substring.
func TestSortMatches(t *testing.T) {
	matches := []searchMatch{
		{QualifiedName: "dbo.PedidoDet", Match: searchMatchKind{Kind: matchContains}},
		{QualifiedName: "dbo.PedidoDetalle", Match: searchMatchKind{Kind: matchPrefix}},
		{QualifiedName: "dbo.Pedido", Match: searchMatchKind{Kind: matchExact}},
		{QualifiedName: "dbo.Pedidos", Match: searchMatchKind{Kind: matchPrefix}},
	}

	sortMatches(matches)

	want := []string{"dbo.Pedido", "dbo.PedidoDetalle", "dbo.Pedidos", "dbo.PedidoDet"}
	for i, name := range want {
		if matches[i].QualifiedName != name {
			t.Errorf("match %d = %q, want %q", i, matches[i].QualifiedName, name)
		}
	}
}

// TestBuildNextSteps verifies the suggested next call.
func TestBuildNextSteps(t *testing.T) {
	matches := []searchMatch{
		{ObjectType: "TABLE", QualifiedName: "dbo.Pedido"},
		{ObjectType: "VIEW", QualifiedName: "dbo.PedidoView"},
		{ObjectType: "SQL_STORED_PROCEDURE", QualifiedName: "dbo.sp_Pedido"},
	}

	steps := buildNextSteps(matches)
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	if steps[0].Tool != "inspect" {
		t.Errorf("tool = %q, want inspect", steps[0].Tool)
	}
	if steps[0].Arguments["table_name"] != "dbo.Pedido" {
		t.Errorf("table = %v, want dbo.Pedido", steps[0].Arguments["table_name"])
	}
	if steps[0].Arguments["detail"] != "all" {
		t.Errorf("detail = %v, want all", steps[0].Arguments["detail"])
	}
}

// TestBuildNextStepsAreCapped verifies that the suggestions do not grow with
// the number of matches.
func TestBuildNextStepsAreCapped(t *testing.T) {
	var matches []searchMatch
	for i := 0; i < 50; i++ {
		matches = append(matches, searchMatch{ObjectType: "TABLE", QualifiedName: "dbo.T"})
	}
	if steps := buildNextSteps(matches); len(steps) != maxNextSteps {
		t.Errorf("steps = %d, want %d", len(steps), maxNextSteps)
	}
}

// TestClampSearchLimit verifies the default and the hard cap of the limit.
func TestClampSearchLimit(t *testing.T) {
	if got := clampSearchLimit(0); got != defaultSearchLimit {
		t.Errorf("limit = %d, want the default %d", got, defaultSearchLimit)
	}
	if got := clampSearchLimit(-1); got != defaultSearchLimit {
		t.Errorf("limit = %d, want the default %d", got, defaultSearchLimit)
	}
	if got := clampSearchLimit(10); got != 10 {
		t.Errorf("limit = %d, want 10", got)
	}
	if got := clampSearchLimit(maxSearchLimit + 1); got != maxSearchLimit {
		t.Errorf("limit = %d, want the maximum %d", got, maxSearchLimit)
	}
}

// TestQualified verifies the qualified name of an object.
func TestQualified(t *testing.T) {
	if got := qualified("dbo", "Pedido"); got != "dbo.Pedido" {
		t.Errorf("qualified = %q, want dbo.Pedido", got)
	}
	if got := qualified("", "Pedido"); got != "Pedido" {
		t.Errorf("qualified = %q, want Pedido", got)
	}
}
