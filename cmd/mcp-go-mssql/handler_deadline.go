package main

import (
	"fmt"
	"time"
)

// requestDeadline bounds how long a single JSON-RPC request may block the stdio loop.
// It must stay above the per-query contexts (30s in query/explain/procedure tools) so
// that a slow-but-healthy query still returns its own error instead of this one.
const requestDeadline = 90 * time.Second

// handleWithDeadline runs handleRequest under requestDeadline.
//
// The stdio loop in main.go processes requests one at a time. Without a bound, a
// handler that never returns (e.g. a driver call blocked outside ctx) stalls the loop
// forever: every later request - even notifications/cancelled - queues up and the
// host (Claude Desktop) gives up after its own ~4 min tool timeout. Restarting the
// process was the only way out.
func (s *MCPMSSQLServer) handleWithDeadline(req MCPRequest) *MCPResponse {
	return runWithDeadline(req, requestDeadline, s.handleRequest, func(format string, args ...any) {
		s.secLogger.Printf(format, args...)
	})
}

// runWithDeadline is the testable core of handleWithDeadline.
//
// On timeout the handler goroutine is abandoned (Go cannot kill it): it may keep a
// pooled DB connection until its own context or the driver releases it. That is an
// acceptable trade-off - the server keeps answering - and MaxOpenConns bounds the leak.
func runWithDeadline(
	req MCPRequest,
	d time.Duration,
	handle func(MCPRequest) *MCPResponse,
	logf func(format string, args ...any),
) *MCPResponse {
	done := make(chan *MCPResponse, 1) // buffered: a late handler must not block forever on send
	go func() {
		done <- handle(req)
	}()

	select {
	case resp := <-done:
		return resp
	case <-time.After(d):
		logf("Request exceeded deadline (%s), handler abandoned: method=%s", d, req.Method)
		if req.ID == nil {
			return nil // notification: JSON-RPC forbids a response
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32603,
				Message: fmt.Sprintf("Internal error: request exceeded %s and was abandoned; the server is still available", d),
			},
		}
	}
}
