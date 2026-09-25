package main

import (
	"fmt"
	"strings"
	"time"
)

// Dynamic alias tools and confirm_operation.

// toolDynamicAvailable handles the dynamic_available tool.
func (s *MCPMSSQLServer) toolDynamicAvailable(id interface{}, params CallToolParams) *MCPResponse {
	if !s.isDynamic {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: dynamic tools are not available in this server instance (classic single-connection mode). This usually means the server was started with classic MSSQL_SERVER / MSSQL_DATABASE configuration."}},
				IsError: true,
			},
		}
	}
	s.dynamicMu.RLock()
	defer s.dynamicMu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Available dynamic connections (loaded from MSSQL_DYNAMIC_* variables):\n\n")

	if len(s.dynamicAliases) == 0 {
		sb.WriteString("No dynamic aliases configured.\n")
		sb.WriteString("Define variables like MSSQL_DYNAMIC_APP_MAIN_SERVER, MSSQL_DYNAMIC_APP_MAIN_DATABASE, etc.\n")
	} else {
		for alias, a := range s.dynamicAliases {
			ro := "READ-ONLY"
			if !a.ReadOnly {
				if len(a.WhitelistTables) > 0 {
					ro = "WRITABLE (whitelist-restricted)"
				} else {
					ro = "WRITABLE (FULL ACCESS - no whitelist)"
				}
			}
			wl := ""
			if len(a.WhitelistTables) > 0 {
				wl = fmt.Sprintf(" | Whitelist: %s", strings.Join(a.WhitelistTables, ","))
			}
			// Marker only — never print the connection string (it may contain credentials).
			override := ""
			if a.ConnectionString != "" {
				override = " | ConnectionString: (custom override set)"
			}
			fmt.Fprintf(&sb, "- %s → %s/%s (%s%s%s)\n", alias, a.Server, a.Database, ro, wl, override)
		}
	}

	sb.WriteString("\nSecurity posture is applied **per alias** (much safer than global settings).\n")
	sb.WriteString("Use dynamic_connect with the alias name to switch the active connection.\n")
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{{Type: "text", Text: sb.String()}},
		},
	}
}

// toolDynamicConnect handles the dynamic_connect tool.
func (s *MCPMSSQLServer) toolDynamicConnect(id interface{}, params CallToolParams) *MCPResponse {
	if !s.isDynamic {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: dynamic_connect is not available in this server instance (classic single-connection mode)."}},
				IsError: true,
			},
		}
	}
	aliasIface := params.Arguments["alias"]
	alias := ""
	if s, ok := aliasIface.(string); ok {
		alias = strings.ToUpper(strings.TrimSpace(s))
	}
	if alias == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: 'alias' parameter is required (e.g. 'APP_MAIN', 'APP_IDENTITY')"}},
				IsError: true,
			},
		}
	}

	if err := s.connectToDynamicAlias(alias); err != nil {
		// In production the raw driver error goes only to the security log;
		// the client gets a generic message (consistent with the classic path).
		var errMsg string
		if s.devMode {
			errMsg = fmt.Sprintf("Error connecting to alias '%s': %v", alias, err)
		} else {
			s.secLogger.Printf("dynamic_connect failed for alias '%s': %v", alias, err)
			errMsg = fmt.Sprintf("Error connecting to alias '%s': connection failed. Verify the alias configuration, credentials and server reachability.", alias)
		}
		// Surface an actionable hint when the error is the well-known TLS 1.0
		// handshake rejection that legacy SQL Server 2000/2008/2012 instances
		// trigger with modern Go runtimes. The wording covers both
		// "protocol version 301" (Go's exact phrasing) and a broader
		// "unsupported protocol" fallback for forward compatibility.
		if isLegacyTLSPivotError(err) {
			errMsg += "\n\nHint: this server likely speaks only TLS 1.0 (SQL Server 2000/2008/2012). " +
				"Set MSSQL_DYNAMIC_<ALIAS>_CONNECTION_STRING to a URL-form DSN: " +
				"sqlserver://USER:PASS@HOST:1433?database=DB&encrypt=disable&trustservercertificate=true"
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: errMsg}},
				IsError: true,
			},
		}
	}

	// Success - report the effective security posture of the newly active alias
	effective := s.getEffectiveConfig()
	roStatus := "READ-ONLY (safe)"
	if !effective.readOnly {
		if len(effective.whitelistTables) > 0 {
			roStatus = "WRITABLE (whitelist-restricted, confirmation required)"
		} else {
			roStatus = "WRITABLE (FULL ACCESS - no whitelist configured, confirmation required)"
		}
	}

	msg := fmt.Sprintf("Successfully connected to dynamic alias '%s'.\n\nEffective security posture for this connection:\n- Read-only mode: %s\n- Whitelisted tables for modification: %v\n\nAll subsequent queries will be validated against this alias's security rules.", alias, roStatus, effective.whitelistTables)

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{{Type: "text", Text: msg}},
		},
	}
}

// toolDynamicDisconnect handles the dynamic_disconnect tool.
func (s *MCPMSSQLServer) toolDynamicDisconnect(id interface{}, params CallToolParams) *MCPResponse {
	if !s.isDynamic {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: dynamic_disconnect is not available in this server instance (classic single-connection mode)."}},
				IsError: true,
			},
		}
	}
	s.dynamicMu.Lock()
	defer s.dynamicMu.Unlock()

	if s.activeAlias == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "No active dynamic connection to disconnect."}},
			},
		}
	}

	closedAlias := s.activeAlias

	// Close the connection
	if conn, ok := s.connections[s.activeAlias]; ok && conn != nil {
		_ = conn.Close()
		delete(s.connections, s.activeAlias)
	}

	s.dbMu.Lock()
	s.db = nil
	s.dbMu.Unlock()

	s.activeAlias = ""

	// Drop any pending confirmation together with the alias it belonged to.
	s.confirmMu.Lock()
	s.pendingConfirmation = nil
	s.confirmMu.Unlock()

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Disconnected from alias '%s'. No active database connection.", closedAlias)}},
		},
	}
}

// toolDynamicList handles the dynamic_list tool.
func (s *MCPMSSQLServer) toolDynamicList(id interface{}, params CallToolParams) *MCPResponse {
	if !s.isDynamic {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: dynamic_list is not available in this server instance (classic single-connection mode)."}},
				IsError: true,
			},
		}
	}
	s.dynamicMu.RLock()
	defer s.dynamicMu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Dynamic aliases currently loaded:\n\n")

	if len(s.dynamicAliases) == 0 {
		sb.WriteString("(none)\n")
	} else {
		for alias, a := range s.dynamicAliases {
			active := ""
			if alias == s.activeAlias {
				active = "  ← ACTIVE"
			}
			fmt.Fprintf(&sb, "- %s (%s/%s)%s\n", alias, a.Server, a.Database, active)
		}
	}

	fmt.Fprintf(&sb, "\nTotal: %d aliases\n", len(s.dynamicAliases))
	if s.activeAlias != "" {
		fmt.Fprintf(&sb, "Active connection: %s\n", s.activeAlias)
	} else {
		sb.WriteString("No active dynamic connection (use dynamic_connect <alias>)\n")
	}
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{{Type: "text", Text: sb.String()}},
		},
	}
}

// toolConfirmOperation handles the confirm_operation tool.
func (s *MCPMSSQLServer) toolConfirmOperation(id interface{}, params CallToolParams) *MCPResponse {
	if !s.isDynamic {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: confirm_operation is only relevant for writable dynamic aliases and is not available in classic single-connection mode."}},
				IsError: true,
			},
		}
	}
	descIface := params.Arguments["description"]
	description := ""
	if d, ok := descIface.(string); ok {
		description = strings.TrimSpace(d)
	}
	if description == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Error: 'description' parameter is required. Describe clearly what operation you want to perform."}},
				IsError: true,
			},
		}
	}

	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()

	if s.pendingConfirmation == nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "No pending operation requires confirmation at this moment."}},
				IsError: true,
			},
		}
	}

	if time.Now().After(s.pendingConfirmation.ExpiresAt) {
		s.pendingConfirmation = nil
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "The previous confirmation request has expired. Please try the operation again to generate a new confirmation request."}},
				IsError: true,
			},
		}
	}

	// Accept the confirmation only when the description actually references
	// the pending operation AND at least one of its target tables. The old
	// blanket "any description longer than 10 chars" rule made the
	// confirmation meaningless (the agent could confirm anything). Requiring
	// both the operation keyword and a concrete table name forces the caller
	// to demonstrate intent about *this specific* operation.
	pendingDesc := strings.ToLower(s.pendingConfirmation.Description)
	userDesc := strings.ToLower(description)

	mentionsOperation := strings.Contains(userDesc, strings.ToLower(s.pendingConfirmation.Operation))
	mentionsTable := false
	for _, t := range s.pendingConfirmation.Tables {
		if t != "" && strings.Contains(userDesc, strings.ToLower(t)) {
			mentionsTable = true
			break
		}
	}
	// A near-verbatim echo of the generated description is also acceptable.
	echoesPending := pendingDesc != "" && strings.Contains(userDesc, pendingDesc)

	if (mentionsOperation && mentionsTable) || echoesPending {

		s.pendingConfirmation.Approved = true
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result: CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Confirmation accepted. You may now execute the exact modification query that generated this approval. This confirmation is valid once."}},
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("The description you provided does not sufficiently match the pending operation.\n\nPending operation was: %s\n\nPlease call confirm_operation again with a description that clearly references the intended action.", s.pendingConfirmation.Description)}},
			IsError: true,
		},
	}
}
