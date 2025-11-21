#!/bin/bash

# Test MCP Server with real database connection

echo "Testing MCP Server with database connection..."
echo "Server: 10.203.3.10:1433"
echo "Database: JJP_TRANSFER"
echo ""

# Load environment variables
export MSSQL_SERVER="10.203.3.10"
export MSSQL_DATABASE="JJP_TRANSFER" 
export MSSQL_USER="userTRANSFER"
export MSSQL_PASSWORD="jl3RN7o02g"
export MSSQL_PORT="1433"
export DEVELOPER_MODE="true"

# Test 1: Initialize
echo "Test 1: Initialize MCP Server"
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "test-client", "version": "1.0"}}}' | ./mcp-server-test.exe
echo ""

# Test 2: List tools  
echo "Test 2: List Available Tools"
echo '{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}' | ./mcp-server-test.exe
echo ""

# Wait a bit for database connection
sleep 3

# Test 3: Get database info
echo "Test 3: Get Database Info"  
echo '{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "get_database_info", "arguments": {}}}' | ./mcp-server-test.exe
echo ""

# Test 4: List tables
echo "Test 4: List Tables"
echo '{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "list_tables", "arguments": {}}}' | ./mcp-server-test.exe  
echo ""

# Test 5: Simple query
echo "Test 5: Simple Query (SELECT @@VERSION)"
echo '{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": {"name": "query_database", "arguments": {"query": "SELECT @@VERSION as server_version"}}}' | ./mcp-server-test.exe

echo ""
echo "Testing completed!"