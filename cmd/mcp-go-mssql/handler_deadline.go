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
	return s.handleWithDeadlineOutcome(req).response
}

// handleWithDeadlineOutcome runs handleRequest under requestDeadline and
// reports whether the deadline was hit.
func (s *MCPMSSQLServer) handleWithDeadlineOutcome(req MCPRequest) deadlineOutcome {
	return runWithDeadlineOutcome(req, requestDeadline, s.handleRequest, func(format string, args ...any) {
		s.secLogger.Printf(format, args...)
	})
}

// deadlineOutcome carries the response of a deadline-bounded request and
// whether the deadline was hit. The timeout flag is needed for accounting: a
// timed-out call must not be counted as a successful one.
type deadlineOutcome struct {
	response *MCPResponse
	timedOut bool
}

// runWithDeadlineOutcome is the testable core of handleWithDeadline.
//
// On timeout the handler goroutine is abandoned (Go cannot kill it): it may keep a
// pooled DB connection until its own context or the driver releases it. That is an
// acceptable trade-off - the server keeps answering - and MaxOpenConns bounds the leak.
func runWithDeadlineOutcome(
	req MCPRequest,
	d time.Duration,
	handle func(MCPRequest) *MCPResponse,
	logf func(format string, args ...any),
) deadlineOutcome {
	done := make(chan *MCPResponse, 1) // buffered: a late handler must not block forever on send
	go func() {
		done <- handle(req)
	}()

	select {
	case resp := <-done:
		return deadlineOutcome{response: resp}
	case <-time.After(d):
		logf("Request exceeded deadline (%s), handler abandoned: method=%s", d, req.Method)
		if req.ID == nil {
			return deadlineOutcome{response: nil, timedOut: true} // notification: JSON-RPC forbids a response
		}
		return deadlineOutcome{
			response: &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32603,
					Message: fmt.Sprintf("Internal error: request exceeded %s and was abandoned; the server is still available", d),
				},
			},
			timedOut: true,
		}
	}
}

// runWithDeadline keeps the previous single-result signature for callers that
// do not need the timeout flag.
func runWithDeadline(
	req MCPRequest,
	d time.Duration,
	handle func(MCPRequest) *MCPResponse,
	logf func(format string, args ...any),
) *MCPResponse {
	return runWithDeadlineOutcome(req, d, handle, logf).response
}
