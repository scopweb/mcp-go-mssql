#!/bin/bash

# MCP Go MSSQL Server Build Script

echo "Building MCP Go MSSQL Server..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    exit 1
fi

# Create build directory if it doesn't exist
mkdir -p build

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

# Build production version
echo "Building production executable..."
go build -ldflags "-w -s" -o build/mcp-go-mssql
if [ $? -ne 0 ]; then
    echo "Error: Build failed"
    exit 1
fi

echo ""
echo "✓ Build successful!"
echo "✓ Executable: build/mcp-go-mssql"
echo ""
echo "To use with Claude Desktop:"
echo "1. Copy build/mcp-go-mssql to your desired location"
echo "2. Update your Claude Desktop config with the full path"
echo "3. Configure environment variables in the config"
echo ""
