package main

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestDynamicAliasPersistsAcrossRequests(t *testing.T) {
	s := newTestMCPServer()
	s.isDynamic = true
	s.dynamicAliases = map[string]DynamicAlias{"CRM": {Alias: "CRM", ReadOnly: true}}
	// An unopened pool exercises server state without contacting SQL Server.
	db, err := sql.Open("sqlserver", "server=localhost;database=master")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.closeAllConnections)
	db.SetConnMaxIdleTime(time.Nanosecond)
	db.SetConnMaxLifetime(time.Nanosecond)
	s.setDB(db)
	s.connections = map[string]*sql.DB{"CRM": db}
	s.activeAlias = "CRM"

	for range 3 {
		for _, method := range []string{"initialize", "ping", "tools/list"} {
			resp := s.handleRequest(MCPRequest{JSONRPC: "2.0", ID: 1, Method: method})
			if resp == nil || resp.Error != nil {
				t.Fatalf("%s failed: %+v", method, resp)
			}
		}
		info := s.toolGetDatabaseInfo(1, CallToolParams{}).Result.(CallToolResult).Content[0].Text
		if !strings.Contains(info, "Active Dynamic Alias: CRM") || s.getDB() != db || s.activeAlias != "CRM" {
			t.Fatalf("connection lost between requests: %s", info)
		}
	}
	// A failed switch must leave the selected alias intact.
	if err := s.connectToDynamicAlias("UNKNOWN"); err == nil {
		t.Fatal("unknown alias accepted")
	}
	if s.getDB() != db || s.activeAlias != "CRM" {
		t.Fatal("failed switch cleared active connection")
	}
	s.toolDynamicDisconnect(1, CallToolParams{})
	if s.getDB() != nil || s.activeAlias != "" {
		t.Fatal("explicit disconnect did not clear active connection")
	}
}

func TestDisconnectedDynamicInfo(t *testing.T) {
	s := newTestMCPServer()
	s.isDynamic = true
	info := s.toolGetDatabaseInfo(1, CallToolParams{}).Result.(CallToolResult).Content[0].Text
	for _, want := range []string{"DISCONNECTED", "dynamic_connect", "process restarts", "MCP Server Process ID:"} {
		if !strings.Contains(info, want) {
			t.Errorf("missing %q in %s", want, info)
		}
	}
	if strings.Contains(info, "MSSQL_SERVER: NOT SET") {
		t.Fatal("dynamic mode incorrectly requires classic connection configuration")
	}
}
