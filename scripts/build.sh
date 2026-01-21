#!/bin/bash

# MCP Go MSSQL Server Build Script

cd "$(dirname "$0")/.." || exit 1

echo "Building MCP Go MSSQL Server..."
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    echo "Please download Go from https://go.dev/dl/"
    exit 1
fi

echo "Checking Go version..."
GO_VERSION=$(go version | awk '{print $3}')
echo "Found Go version: $GO_VERSION"
echo ""

# Create build directory if it doesn't exist
mkdir -p build
if [ $? -eq 0 ]; then
    echo "Build directory ready"
fi
echo ""

# Clean previous builds
if [ -f "build/mcp-go-mssql" ]; then
    rm build/mcp-go-mssql
    echo "Cleaned previous build"
fi

# Download dependencies
echo "Downloading dependencies..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "Error: Failed to download dependencies"
    exit 1
fi

# Build production version with optimizations
echo "Building production executable..."
go build -ldflags "-w -s" -o build/mcp-go-mssql -v
if [ $? -ne 0 ]; then
    echo "Error: Build failed"
    exit 1
fi

# Get file size
if [ -f "build/mcp-go-mssql" ]; then
    FILE_SIZE=$(stat -f%z "build/mcp-go-mssql" 2>/dev/null || stat -c%s "build/mcp-go-mssql" 2>/dev/null)
    echo ""
    echo "File size: $FILE_SIZE bytes"
fi

echo ""
echo "========================================"
echo "✓ Build successful!"
echo "========================================"
echo ""
echo "Executable location: build/mcp-go-mssql"
echo ""
echo "To use with Claude Desktop:"
echo "1. Ensure claude_desktop_config.json has the correct path:"
echo "   \"command\": \"/path/to/mcp-go-mssql/build/mcp-go-mssql\""
echo "2. Configure environment variables in the config"
echo "3. Restart Claude Desktop to reload the MCP server"
echo ""
echo "On macOS/Linux, you may need to make it executable:"
echo "chmod +x build/mcp-go-mssql"
echo ""
