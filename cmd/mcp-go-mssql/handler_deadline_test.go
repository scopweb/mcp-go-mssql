package main

import (
	"testing"
	"time"
)

func noopLogf(string, ...any) {}

// A handler that finishes in time returns its own response untouched.
func TestRunWithDeadlineReturnsHandlerResponse(t *testing.T) {
	want := &MCPResponse{JSONRPC: "2.0", ID: "ok-1"}
	got := runWithDeadline(MCPRequest{JSONRPC: "2.0", ID: "ok-1", Method: "ping"}, time.Second,
		func(MCPRequest) *MCPResponse { return want }, noopLogf)
	if got != want {
		t.Fatalf("expected the handler's response, got %+v", got)
	}
}

// A handler that never returns must not block the caller: a -32603 error comes back
// after the deadline. This is the bug that froze the whole stdio loop.
func TestRunWithDeadlineAbandonsBlockedHandler(t *testing.T) {
	block := make(chan struct{})
	defer close(block) // release the abandoned goroutine at the end of the test

	start := time.Now()
	resp := runWithDeadline(MCPRequest{JSONRPC: "2.0", ID: "hang-1", Method: "tools/call"}, 50*time.Millisecond,
		func(MCPRequest) *MCPResponse { <-block; return nil }, noopLogf)

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("runWithDeadline blocked for %s", elapsed)
	}
	if resp == nil || resp.Error == nil || resp.Error.Code != -32603 {
		t.Fatalf("expected -32603 error response, got %+v", resp)
	}
	if resp.ID != "hang-1" {
		t.Fatalf("response must keep the request ID, got %v", resp.ID)
	}
}

// Notifications (no ID) must never get a response, not even on timeout.
func TestRunWithDeadlineNotificationTimeoutHasNoResponse(t *testing.T) {
	block := make(chan struct{})
	defer close(block)

	resp := runWithDeadline(MCPRequest{JSONRPC: "2.0", Method: "notifications/cancelled"}, 50*time.Millisecond,
		func(MCPRequest) *MCPResponse { <-block; return nil }, noopLogf)
	if resp != nil {
		t.Fatalf("notification must not receive a response, got %+v", resp)
	}
}

// The request deadline must stay above the 30s per-query contexts, otherwise a slow but
// healthy query would be reported as a server timeout instead of its own error.
func TestRequestDeadlineAboveQueryTimeout(t *testing.T) {
	if requestDeadline <= 30*time.Second {
		t.Fatalf("requestDeadline (%s) must be greater than the 30s query timeout", requestDeadline)
	}
}
