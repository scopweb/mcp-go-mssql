package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestServer() *MCPMSSQLServer {
	s := &MCPMSSQLServer{
		secLogger: NewSecurityLogger(),
		devMode:   true,
	}
	s.rateLimiter.maxTokens = 5
	s.rateLimiter.tokens = 5
	s.rateLimiter.lastReset = time.Now()
	s.rateLimiter.interval = time.Minute
	return s
}

func TestRateLimit_Basic(t *testing.T) {
	s := newTestServer()

	// First 5 calls should succeed (matching maxTokens=5)
	for i := range 5 {
		if !s.checkRateLimit() {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}

	// 6th call should be rate limited
	if s.checkRateLimit() {
		t.Fatal("6th call should be rate limited")
	}
}

func TestRateLimit_Reset(t *testing.T) {
	s := newTestServer()
	s.rateLimiter.interval = 10 * time.Millisecond // fast reset for testing

	// Exhaust all tokens
	for range 5 {
		s.checkRateLimit()
	}
	if s.checkRateLimit() {
		t.Fatal("should be rate limited after exhausting tokens")
	}

	// Wait for reset
	time.Sleep(15 * time.Millisecond)

	// Should be allowed again
	if !s.checkRateLimit() {
		t.Fatal("should be allowed after interval reset")
	}
}

func TestRateLimit_Concurrent(t *testing.T) {
	s := newTestServer()
	s.rateLimiter.maxTokens = 100
	s.rateLimiter.tokens = 100

	var allowed atomic.Int64
	var denied atomic.Int64
	var wg sync.WaitGroup

	// Launch 200 goroutines competing for 100 tokens
	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.checkRateLimit() {
				allowed.Add(1)
			} else {
				denied.Add(1)
			}
		}()
	}
	wg.Wait()

	if allowed.Load() != 100 {
		t.Errorf("expected exactly 100 allowed, got %d", allowed.Load())
	}
	if denied.Load() != 100 {
		t.Errorf("expected exactly 100 denied, got %d", denied.Load())
	}
}

func TestRateLimit_ToolCallIntegration(t *testing.T) {
	s := newTestServer()
	s.rateLimiter.maxTokens = 2
	s.rateLimiter.tokens = 2

	params := CallToolParams{
		Name:      "get_database_info",
		Arguments: map[string]interface{}{},
	}

	// First 2 calls should return normal results (not rate limited)
	for i := range 2 {
		resp := s.handleToolCall("test", params)
		if resp.Error != nil {
			t.Fatalf("call %d: unexpected protocol error", i+1)
		}
	}

	// 3rd call should return rate limit error as tool result with IsError
	resp := s.handleToolCall("test", params)
	if resp.Error != nil {
		t.Fatal("rate limit should return tool result error, not protocol error")
	}
	// The result should contain IsError: true
	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("expected CallToolResult in response")
	}
	if !result.IsError {
		t.Error("expected IsError=true for rate-limited response")
	}
}
