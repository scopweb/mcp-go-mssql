package main

import (
	"errors"
	"fmt"
	"strings"
)

// Write-policy announcements.
//
// The server enforces the write policy, but an agent only discovers it by
// failing. Three channels carry the policy to the model, priced so that the
// read-only path stays free:
//
//   - initialize.instructions: once per session (the host injects it as the
//     server instructions). No per-turn cost.
//   - the rejection message: only paid when a write is actually attempted,
//     which is exactly when the agent needs the rule.
//   - dynamic_connect / get_database_info: paid on connection changes, the
//     only moment the posture can change.
//
// Nothing is added to the tool descriptions on purpose: tool schemas are
// resent on every request, so prose there would be billed on every turn.

// currentAliasName returns the active dynamic alias ("" in classic mode).
func (s *MCPMSSQLServer) currentAliasName() string {
	s.dynamicMu.RLock()
	defer s.dynamicMu.RUnlock()
	return s.activeAlias
}

// buildInstructions returns the `instructions` payload of the initialize
// result: a one-time briefing that states the write policy of this instance
// instead of leaving the agent to find out by failing a write.
//
// The text is deliberately short: the host sends the instructions with every
// request, so every word here is paid on every turn. The long version of the
// guidance lives in the rejection message, which is only paid when a write is
// actually attempted.
func (s *MCPMSSQLServer) buildInstructions() string {
	var b strings.Builder
	b.WriteString("Secure access to a Microsoft SQL Server database. get_database_info shows the connection state, explore and inspect show the schema, query_database runs SQL.")

	if s.isDynamic {
		b.WriteString(" WRITE POLICY: it is per alias, and aliases are read-only by default. dynamic_available lists each alias's posture; dynamic_connect reports the posture of the alias it activates. A rejected write is final: don't retry it, rephrase it or route it through another tool. If the user needs that change, give them the final SQL to run in their own client.")
		return b.String()
	}

	effective := s.getEffectiveConfig()
	switch {
	case effective.readOnly:
		b.WriteString(" WRITE POLICY: this instance is READ-ONLY. INSERT, UPDATE, DELETE, DDL and stored procedures are rejected before they reach SQL Server. A rejected write is final: don't retry it, rephrase it or route it through another tool. If the user asks for a change, give them the final SQL to run in their own client.")
	case len(effective.whitelistTables) == 0:
		b.WriteString(" WRITE POLICY: writes are allowed on this instance, with no table restriction.")
	default:
		b.WriteString(fmt.Sprintf(" WRITE POLICY: writes are allowed only on these tables: %s. ", strings.Join(effective.whitelistTables, ", ")))
		b.WriteString("Any other table is read-only, and a rejected write is final: don't retry it or rephrase it. If the user needs a change there, give them the final SQL to run in their own client.")
	}
	return b.String()
}

// policyHint tells the agent what to do after a write was rejected by the
// policy. It is only attached to rejections, so it costs nothing while the
// agent only reads.
func (s *MCPMSSQLServer) policyHint() string {
	effective := s.getEffectiveConfig()
	switch {
	case effective.readOnly:
		return "WRITE POLICY: this connection is READ-ONLY, so the statement was rejected by the server policy and not by SQL Server. Do not retry it, do not rephrase it and do not route it through another tool: execute_procedure, SET tricks and reformulations are rejected too. If the user needs this change, give them the final SQL to run in their own client."
	case len(effective.whitelistTables) == 0:
		return "WRITE POLICY: this connection has no write whitelist, so no modification is allowed. Do not retry it. If the user needs this change, give them the final SQL to run in their own client."
	default:
		return fmt.Sprintf("WRITE POLICY: writes are allowed only on these tables: %s. Do not retry the same statement. If the user needs the change on another table, give them the final SQL to run in their own client.", strings.Join(effective.whitelistTables, ", "))
	}
}

// policyError is a rejection caused by the write policy, with the guidance
// that tells the agent to stop trying variants of the same statement.
type policyError struct {
	err  error
	hint string
}

func (e *policyError) Error() string { return e.err.Error() + "\n\n" + e.hint } //nolint:staticcheck // multi-line user-facing message; capitalization + punctuation are intentional

func (e *policyError) Unwrap() error { return e.err }

// withPolicyHint attaches the write-policy guidance to a policy rejection.
// A confirmation request already carries its own instruction and is left
// untouched.
func (s *MCPMSSQLServer) withPolicyHint(err error) error {
	if err == nil {
		return nil
	}
	var cerr *confirmationError
	if errors.As(err, &cerr) {
		return err
	}
	return &policyError{err: err, hint: s.policyHint()}
}
