package main

// Catalog documentation check.
//
// The tools/list catalog is the contract the agent actually sees. The
// agent-facing documents (the skill, its reference and the README) must
// describe it exactly, so a renamed tool or a changed parameter cannot
// silently drift away from the code.
//
// Each document carries a generated block that this test owns. After
// changing the catalog, regenerate it with:
//
//	go test ./cmd/mcp-go-mssql -run TestToolCatalogDocumentation -update
//
// and commit the result together with the code change.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const (
	docBlockBegin = "<!-- BEGIN GENERATED TOOL CATALOG -->"
	docBlockEnd   = "<!-- END GENERATED TOOL CATALOG -->"
)

// updateDocBlocks regenerates the generated blocks instead of verifying them.
var updateDocBlocks = flag.Bool("update", false, "regenerate the generated tool catalog blocks in the agent-facing docs")

// docFile is one agent-facing document that must describe the served catalog.
type docFile struct {
	relPath string
	// anchor is the line after which the block is inserted when it is missing.
	anchor string
}

// docFiles are the documents the agent reads to learn the tool surface.
var docFiles = []docFile{
	{
		relPath: filepath.Join(".claude", "skills", "mcp-mssql-tools", "SKILL.md"),
		anchor:  "# MCP MSSQL Tools — Quick Reference",
	},
	{
		relPath: filepath.Join(".claude", "skills", "mcp-mssql-tools", "reference", "tools-reference.md"),
		anchor:  "# MCP MSSQL Tools — Complete Reference",
	},
	{
		relPath: "README.md",
		anchor:  "Agent and harness contract: [docs/AGENT_CONTRACT.md](docs/AGENT_CONTRACT.md).",
	},
}

// docRow is one parsed row of a generated catalog block.
type docRow struct {
	name        string
	params      []string
	required    []string
	mode        string
	readOnly    string
	destructive string
	idempotent  string
}

// repoRoot returns the repository root, derived from this file's location so
// the test does not depend on the working directory.
func repoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

// servedTools returns the catalog the server would publish for the given mode.
// It needs no database: listTools only reads the server mode.
func servedTools(t *testing.T, dynamic bool) []Tool {
	t.Helper()

	server := &MCPMSSQLServer{isDynamic: dynamic}
	resp := server.listTools(nil)
	if resp == nil {
		t.Fatalf("listTools(dynamic=%v) returned nil", dynamic)
	}
	result, ok := resp.Result.(ToolsListResult)
	if !ok {
		t.Fatalf("listTools(dynamic=%v) returned %T, want ToolsListResult", dynamic, resp.Result)
	}
	return result.Tools
}

// yesNo renders an annotation hint as a stable token.
func yesNo(hint *bool) string {
	switch {
	case hint == nil:
		return "unset"
	case *hint:
		return "yes"
	default:
		return "no"
	}
}

// cellList renders a list of tokens as a markdown cell.
func cellList(items []string) string {
	if len(items) == 0 {
		return "—"
	}
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, "`"+item+"`")
	}
	return strings.Join(quoted, ", ")
}

// toolParams returns the "name:type" tokens of a tool input schema, sorted.
func toolParams(tool Tool) []string {
	params := make([]string, 0, len(tool.InputSchema.Properties))
	for name, prop := range tool.InputSchema.Properties {
		params = append(params, name+":"+prop.Type)
	}
	sort.Strings(params)
	return params
}

// toolRequired returns the required argument names, sorted.
func toolRequired(tool Tool) []string {
	required := make([]string, 0, len(tool.InputSchema.Required))
	required = append(required, tool.InputSchema.Required...)
	sort.Strings(required)
	return required
}

// rowFor builds the documented row of a tool for the given mode.
func rowFor(tool Tool, mode string) docRow {
	row := docRow{
		name:        tool.Name,
		params:      toolParams(tool),
		required:    toolRequired(tool),
		mode:        mode,
		readOnly:    "unset",
		destructive: "unset",
		idempotent:  "unset",
	}
	if tool.Annotations != nil {
		row.readOnly = yesNo(tool.Annotations.ReadOnlyHint)
		row.destructive = yesNo(tool.Annotations.DestructiveHint)
		row.idempotent = yesNo(tool.Annotations.IdempotentHint)
	}
	return row
}

// expectedRows builds the catalog the documents must describe: every tool of
// the dynamic catalog, marked as classic when it is also served in classic mode.
func expectedRows(classic, dynamic []Tool) map[string]docRow {
	classicNames := make(map[string]bool, len(classic))
	for _, tool := range classic {
		classicNames[tool.Name] = true
	}

	expected := make(map[string]docRow, len(dynamic))
	for _, tool := range dynamic {
		mode := "dynamic-only"
		if classicNames[tool.Name] {
			mode = "classic"
		}
		expected[tool.Name] = rowFor(tool, mode)
	}
	return expected
}

// renderDocBlock renders the generated block for a catalog.
func renderDocBlock(expected map[string]docRow, order []string) string {
	var b strings.Builder
	b.WriteString(docBlockBegin)
	b.WriteString("\n")
	b.WriteString("<!-- Generated from tools/list by cmd/mcp-go-mssql/tools_docs_test.go. Do not edit by hand. -->\n")
	b.WriteString("<!-- Regenerate: go test ./cmd/mcp-go-mssql -run TestToolCatalogDocumentation -update -->\n")
	b.WriteString("\n")
	b.WriteString("| Tool | Parameters | Required | Mode | readOnly | destructive | idempotent |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, name := range order {
		row := expected[name]
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s | %s |\n",
			row.name, cellList(row.params), cellList(row.required), row.mode,
			row.readOnly, row.destructive, row.idempotent))
	}
	b.WriteString("\n")
	b.WriteString(docBlockEnd)
	return b.String()
}

// parseDocRows extracts the rows of a generated block.
func parseDocRows(content string) ([]docRow, error) {
	start := strings.Index(content, docBlockBegin)
	end := strings.Index(content, docBlockEnd)
	if start < 0 || end < 0 {
		return nil, fmt.Errorf("generated block markers not found (expected %q and %q)", docBlockBegin, docBlockEnd)
	}
	if end < start {
		return nil, fmt.Errorf("generated block markers are in the wrong order")
	}

	var rows []docRow
	for _, line := range strings.Split(content[start+len(docBlockBegin):end], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 7 {
			continue
		}
		name := strings.Trim(strings.TrimSpace(cells[0]), "`")
		if name == "" || name == "Tool" || strings.HasPrefix(name, "-") {
			continue
		}
		rows = append(rows, docRow{
			name:        name,
			params:      parseCellList(cells[1]),
			required:    parseCellList(cells[2]),
			mode:        strings.TrimSpace(cells[3]),
			readOnly:    strings.TrimSpace(cells[4]),
			destructive: strings.TrimSpace(cells[5]),
			idempotent:  strings.TrimSpace(cells[6]),
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("generated block contains no rows")
	}
	return rows, nil
}

// parseCellList parses one markdown cell into its sorted tokens.
func parseCellList(cell string) []string {
	var out []string
	for _, part := range strings.Split(cell, ",") {
		token := strings.Trim(strings.TrimSpace(part), "`")
		if token == "" || token == "—" || token == "-" {
			continue
		}
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

// diffRows reports every difference between the expected catalog and the
// documented one.
func diffRows(expected map[string]docRow, actual []docRow) []string {
	var diffs []string

	seen := make(map[string]bool, len(actual))
	for _, row := range actual {
		seen[row.name] = true
		want, ok := expected[row.name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("documented tool %q is not served by tools/list", row.name))
			continue
		}
		if !equalTokens(row.params, want.params) {
			diffs = append(diffs, fmt.Sprintf("%s: documented parameters %v, served %v", row.name, row.params, want.params))
		}
		if !equalTokens(row.required, want.required) {
			diffs = append(diffs, fmt.Sprintf("%s: documented required %v, served %v", row.name, row.required, want.required))
		}
		if row.mode != want.mode {
			diffs = append(diffs, fmt.Sprintf("%s: documented mode %q, served %q", row.name, row.mode, want.mode))
		}
		if row.readOnly != want.readOnly || row.destructive != want.destructive || row.idempotent != want.idempotent {
			diffs = append(diffs, fmt.Sprintf("%s: documented annotations (readOnly=%s destructive=%s idempotent=%s), served (readOnly=%s destructive=%s idempotent=%s)",
				row.name, row.readOnly, row.destructive, row.idempotent,
				want.readOnly, want.destructive, want.idempotent))
		}
	}

	names := make([]string, 0, len(expected))
	for name := range expected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !seen[name] {
			diffs = append(diffs, fmt.Sprintf("tool %q is served by tools/list but missing from the documentation", name))
		}
	}
	return diffs
}

// equalTokens compares two sorted token lists.
func equalTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// upsertDocBlock replaces the generated block, or inserts it after the anchor
// when it is not there yet.
func upsertDocBlock(content, block, anchor string) (string, error) {
	start := strings.Index(content, docBlockBegin)
	end := strings.Index(content, docBlockEnd)
	if start >= 0 && end > start {
		return content[:start] + block + content[end+len(docBlockEnd):], nil
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == anchor {
			rest := strings.Join(lines[i+1:], "\n")
			return strings.Join(lines[:i+1], "\n") + "\n\n" + block + "\n" + rest, nil
		}
	}
	return "", fmt.Errorf("anchor line %q not found", anchor)
}

// publishedSchemaPath is the machine readable schema of the served catalog.
//
// It is published so an agent (or a client) can read what the server serves
// without starting it, and so the CI can prove that the published schema
// and the binary agree.
var publishedSchemaPath = filepath.Join("docs", "mcp-tools.json")

// publishedSchema is the published shape of the catalog.
type publishedSchema struct {
	// Contract is the version of the agent contract the schema belongs to.
	Contract string `json:"contract"`
	// Classic is the catalog served in classic mode.
	Classic []Tool `json:"classic"`
	// Dynamic is the catalog served in dynamic mode.
	Dynamic []Tool `json:"dynamic"`
}

// buildPublishedSchema returns the schema of the served catalog.
func buildPublishedSchema(classic, dynamic []Tool) publishedSchema {
	return publishedSchema{Contract: "1", Classic: classic, Dynamic: dynamic}
}

// renderPublishedSchema renders the published schema in a stable form.
func renderPublishedSchema(schema publishedSchema) (string, error) {
	encoded, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded) + "\n", nil
}

// TestPublishedToolSchemaMatchesTheServedCatalog verifies that the published
// schema of tools/list is the one the server serves.
func TestPublishedToolSchemaMatchesTheServedCatalog(t *testing.T) {
	classic := servedTools(t, false)
	dynamic := servedTools(t, true)
	schema := buildPublishedSchema(classic, dynamic)

	path := filepath.Join(repoRoot(), publishedSchemaPath)

	if *updateDocBlocks {
		rendered, err := renderPublishedSchema(schema)
		if err != nil {
			t.Fatalf("cannot render the published schema: %v", err)
		}
		if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
			t.Fatalf("cannot write the published schema: %v", err)
		}
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the published schema: %v", err)
	}

	var published publishedSchema
	if err := json.Unmarshal(content, &published); err != nil {
		t.Fatalf("the published schema is not valid JSON: %v", err)
	}

	if published.Contract != schema.Contract {
		t.Errorf("published contract = %q, want %q", published.Contract, schema.Contract)
	}
	if !equalToolLists(published.Classic, classic) {
		t.Errorf("the published classic catalog does not match the served one:\n  published %s\n  served    %s",
			toolNames(published.Classic), toolNames(classic))
	}
	if !equalToolLists(published.Dynamic, dynamic) {
		t.Errorf("the published dynamic catalog does not match the served one:\n  published %s\n  served    %s",
			toolNames(published.Dynamic), toolNames(dynamic))
	}
}

// equalToolLists compares two catalogs by their serialized form.
func equalToolLists(left, right []Tool) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

// toolNames lists the names of a catalog.
func toolNames(tools []Tool) string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return strings.Join(names, ", ")
}

// TestToolCatalogDocumentation verifies that every agent-facing document
// describes the served catalog exactly. With -update it regenerates the
// generated blocks instead.
func TestToolCatalogDocumentation(t *testing.T) {
	classic := servedTools(t, false)
	dynamic := servedTools(t, true)
	expected := expectedRows(classic, dynamic)

	order := make([]string, 0, len(dynamic))
	for _, tool := range dynamic {
		order = append(order, tool.Name)
	}
	block := renderDocBlock(expected, order)

	for _, doc := range docFiles {
		path := filepath.Join(repoRoot(), doc.relPath)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: cannot read: %v", doc.relPath, err)
			continue
		}

		if *updateDocBlocks {
			updated, err := upsertDocBlock(string(content), block, doc.anchor)
			if err != nil {
				t.Errorf("%s: %v", doc.relPath, err)
				continue
			}
			if updated != string(content) {
				if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
					t.Errorf("%s: cannot write: %v", doc.relPath, err)
				}
			}
			content = []byte(updated)
		}

		rows, err := parseDocRows(string(content))
		if err != nil {
			t.Errorf("%s: %v", doc.relPath, err)
			continue
		}
		if diffs := diffRows(expected, rows); len(diffs) > 0 {
			t.Errorf("%s: tool catalog drift:\n  - %s", doc.relPath, strings.Join(diffs, "\n  - "))
			t.Errorf("  regenerate with: go test ./cmd/mcp-go-mssql -run TestToolCatalogDocumentation -update")
		}
	}
}

// TestToolCatalogDocumentationDetectsDrift proves the comparison fails on a
// wrong document, so a green test really means the documents are in sync.
func TestToolCatalogDocumentationDetectsDrift(t *testing.T) {
	classic := servedTools(t, false)
	dynamic := servedTools(t, true)
	expected := expectedRows(classic, dynamic)

	rows := make([]docRow, 0, len(expected))
	for name := range expected {
		rows = append(rows, expected[name])
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	if diffs := diffRows(expected, rows); len(diffs) != 0 {
		t.Fatalf("in-sync catalog reported as drifted: %v", diffs)
	}

	// A parameter that the server does not accept must be reported.
	for i := range rows {
		if rows[i].name == "dynamic_disconnect" {
			rows[i].params = []string{"alias:string"}
		}
	}
	if diffs := diffRows(expected, rows); len(diffs) == 0 {
		t.Error("a documented parameter that the server does not accept was not detected")
	}
}

// TestToolsReferenceCoversEveryTool verifies that every served tool has its
// own section in the skill reference, so a renamed tool cannot keep a stale
// section unnoticed.
func TestToolsReferenceCoversEveryTool(t *testing.T) {
	path := filepath.Join(repoRoot(), ".claude", "skills", "mcp-mssql-tools", "reference", "tools-reference.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read tools reference: %v", err)
	}

	for _, tool := range servedTools(t, true) {
		pattern := regexp.MustCompile(`(?m)^##\s+\d+\.\s+` + "`" + regexp.QuoteMeta(tool.Name) + "`")
		if !pattern.MatchString(string(content)) {
			t.Errorf("tools reference has no section for served tool %q", tool.Name)
		}
	}
}

// TestSkillToolCountMatchesCatalog rejects a stale tool count in the skill.
// Only the counts actually served by the server are accepted.
func TestSkillToolCountMatchesCatalog(t *testing.T) {
	classic := servedTools(t, false)
	dynamic := servedTools(t, true)

	path := filepath.Join(repoRoot(), ".claude", "skills", "mcp-mssql-tools", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read skill: %v", err)
	}

	allowed := map[int]bool{len(classic): true, len(dynamic): true}
	countRe := regexp.MustCompile(`(?i)\b(\d+)\s+tools\b`)
	for _, match := range countRe.FindAllStringSubmatch(string(content), -1) {
		count := 0
		for _, r := range match[1] {
			count = count*10 + int(r-'0')
		}
		if !allowed[count] {
			t.Errorf("skill states %q but the server serves %d tools in classic mode and %d in dynamic mode",
				match[0], len(classic), len(dynamic))
		}
	}
}
