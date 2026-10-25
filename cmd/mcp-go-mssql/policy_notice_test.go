package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// The write policy must reach the agent without being paid for on every
// turn: once per session through initialize.instructions, and on the
// rejection itself when a write is actually attempted.

func TestBuildInstructionsClassicReadOnly(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: true}

	instructions := s.buildInstructions()

	if !strings.Contains(instructions, "READ-ONLY") {
		t.Errorf("instructions must state the read-only posture, got: %s", instructions)
	}
	if !strings.Contains(instructions, "final SQL") {
		t.Errorf("instructions must offer the alternative to the blocked write, got: %s", instructions)
	}
}

func TestBuildInstructionsClassicWhitelist(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: false, whitelistTables: []string{"temp_ai", "v_temp_ia"}}

	instructions := s.buildInstructions()

	if !strings.Contains(instructions, "temp_ai") || !strings.Contains(instructions, "v_temp_ia") {
		t.Errorf("instructions must list the writable tables, got: %s", instructions)
	}
	if strings.Contains(instructions, "this instance is READ-ONLY") {
		t.Errorf("instructions must not claim read-only on a writable instance, got: %s", instructions)
	}
}

func TestBuildInstructionsClassicFullAccess(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: false}

	instructions := s.buildInstructions()

	if strings.Contains(instructions, "READ-ONLY") {
		t.Errorf("instructions must not claim read-only on a writable instance, got: %s", instructions)
	}
	if !strings.Contains(instructions, "WRITE POLICY") {
		t.Errorf("instructions must always state the write policy, got: %s", instructions)
	}
}

func TestBuildInstructionsDynamicMode(t *testing.T) {
	s := newTestMCPServer()
	s.isDynamic = true

	instructions := s.buildInstructions()

	if !strings.Contains(instructions, "dynamic_available") {
		t.Errorf("instructions must point to the tool that reports the per-alias posture, got: %s", instructions)
	}
	if !strings.Contains(instructions, "per alias") {
		t.Errorf("instructions must state that the posture is per alias, got: %s", instructions)
	}
}

func TestInitializeReturnsPostureInstructions(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: true}

	response := s.handleRequest(MCPRequest{
		JSONRPC: "2.0",
		ID:      "init-policy",
		Method:  "initialize",
		Params:  InitializeParams{ProtocolVersion: "2025-11-25"},
	})

	resultBytes, _ := json.Marshal(response.Result)
	var initResult InitializeResult
	if err := json.Unmarshal(resultBytes, &initResult); err != nil {
		t.Fatalf("failed to decode initialize result: %v", err)
	}

	if !strings.Contains(initResult.Instructions, "READ-ONLY") {
		t.Errorf("initialize must announce the posture, got: %s", initResult.Instructions)
	}
}

func TestPolicyHintStatesTheRuleAndTheAlternative(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: true}

	hint := s.policyHint()

	if !strings.Contains(hint, "READ-ONLY") {
		t.Errorf("hint must state the posture, got: %s", hint)
	}
	if !strings.Contains(hint, "Do not retry") {
		t.Errorf("hint must stop the agent from retrying, got: %s", hint)
	}
	if !strings.Contains(hint, "final SQL") {
		t.Errorf("hint must offer the alternative, got: %s", hint)
	}
}

func TestPolicyHintListsWritableTables(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: false, whitelistTables: []string{"temp_ai"}}

	hint := s.policyHint()

	if !strings.Contains(hint, "temp_ai") {
		t.Errorf("hint must list the writable tables, got: %s", hint)
	}
}

func TestWithPolicyHintKeepsOriginalMessage(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: true}

	err := s.withPolicyHint(errors.New("read-only mode: only SELECT and read operations are allowed"))

	if !strings.Contains(err.Error(), "read-only mode: only SELECT and read operations are allowed") {
		t.Errorf("original rejection must be preserved, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "WRITE POLICY") {
		t.Errorf("rejection must carry the policy hint, got: %s", err.Error())
	}
}

func TestWithPolicyHintLeavesConfirmationRequestsAlone(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: false}

	err := s.withPolicyHint(&confirmationError{msg: "CONFIRMATION REQUIRED: call confirm_operation first"})

	if strings.Contains(err.Error(), "WRITE POLICY") {
		t.Errorf("a confirmation request already carries its instruction, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "CONFIRMATION REQUIRED") {
		t.Errorf("confirmation request must be preserved, got: %s", err.Error())
	}
}

func TestReadOnlyRejectionCarriesTheFullGuidance(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: true}

	err := s.validateReadOnlyQuery("DELETE FROM dbo.temp_ai")
	if err == nil {
		t.Fatal("a write must be rejected in read-only mode")
	}

	text := s.withPolicyHint(err).Error()
	for _, want := range []string{"read-only mode", "WRITE POLICY", "Do not retry", "final SQL"} {
		if !strings.Contains(text, want) {
			t.Errorf("rejection must contain %q, got: %s", want, text)
		}
	}
}

func TestWhitelistRejectionCarriesTheFullGuidance(t *testing.T) {
	s := newTestMCPServer()
	s.config = serverConfig{readOnly: false, whitelistTables: []string{"temp_ai"}}

	err := s.validateTablePermissions("DELETE FROM prod_users")
	if err == nil {
		t.Fatal("a write to a non-whitelisted table must be rejected")
	}

	text := s.withPolicyHint(err).Error()
	for _, want := range []string{"not whitelisted", "WRITE POLICY", "temp_ai", "final SQL"} {
		if !strings.Contains(text, want) {
			t.Errorf("rejection must contain %q, got: %s", want, text)
		}
	}
}

func TestToolAnnotationsDescribeTheToolAndNotTheSession(t *testing.T) {
	// The MCP annotations describe the tool surface, not the posture of the
	// running instance: query_database can modify data in a writable
	// configuration, so it stays published as destructive. The posture of the
	// session is announced in the initialize instructions instead.
	for _, readOnly := range []bool{true, false} {
		s := newTestMCPServer()
		s.config = serverConfig{readOnly: readOnly}

		response := s.handleRequest(MCPRequest{JSONRPC: "2.0", ID: "tools-ann", Method: "tools/list"})
		resultBytes, _ := json.Marshal(response.Result)
		var toolsResult ToolsListResult
		if err := json.Unmarshal(resultBytes, &toolsResult); err != nil {
			t.Fatalf("failed to decode tools/list result: %v", err)
		}

		for _, tool := range toolsResult.Tools {
			switch tool.Name {
			case "query_database", "execute_procedure":
				if tool.Annotations.ReadOnlyHint == nil || *tool.Annotations.ReadOnlyHint {
					t.Errorf("tool %q must not be advertised as read-only (readOnly=%v)", tool.Name, readOnly)
				}
				if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
					t.Errorf("tool %q must stay marked destructive (readOnly=%v)", tool.Name, readOnly)
				}
			}
		}
	}
}

func TestToolAnnotationsStayConservativeInDynamicMode(t *testing.T) {
	s := newTestMCPServer()
	s.isDynamic = true
	s.config = serverConfig{readOnly: true}

	response := s.handleRequest(MCPRequest{JSONRPC: "2.0", ID: "tools-dyn", Method: "tools/list"})
	resultBytes, _ := json.Marshal(response.Result)
	var toolsResult ToolsListResult
	if err := json.Unmarshal(resultBytes, &toolsResult); err != nil {
		t.Fatalf("failed to decode tools/list result: %v", err)
	}

	for _, tool := range toolsResult.Tools {
		if tool.Name != "query_database" {
			continue
		}
		if tool.Annotations.ReadOnlyHint == nil || *tool.Annotations.ReadOnlyHint {
			t.Error("query_database must not be advertised as read-only in dynamic mode: the posture depends on the active alias")
		}
		if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
			t.Error("query_database must stay marked destructive in dynamic mode")
		}
	}
}
