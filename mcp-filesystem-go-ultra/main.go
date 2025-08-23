package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mcp/filesystem-ultra/cache"
	"github.com/mcp/filesystem-ultra/core"
	localmcp "github.com/mcp/filesystem-ultra/mcp"
)

// Configuration holds all server configuration
type Configuration struct {
	CacheSize        int64    // Cache size in bytes
	ParallelOps      int      // Max concurrent operations
	BinaryThreshold  int64    // File size threshold for binary protocol
	VSCodeAPIEnabled bool     // Enable VSCode API integration when available
	DebugMode        bool     // Enable debug logging
	LogLevel         string   // Log level (info, debug, error)
	AllowedPaths     []string // List of allowed base paths for access control
}

// DefaultConfiguration returns optimized defaults based on system
func DefaultConfiguration() *Configuration {
	// Auto-detect optimal settings based on system resources
	cpuCount := runtime.NumCPU()
	parallelOps := cpuCount * 2 // 2x CPU cores for I/O bound operations
	if parallelOps > 16 {
		parallelOps = 16 // Cap at 16 to avoid overhead
	}

	return &Configuration{
		CacheSize:        100 * 1024 * 1024, // 100MB default
		ParallelOps:      parallelOps,
		BinaryThreshold:  1024 * 1024, // 1MB threshold
		VSCodeAPIEnabled: true,
		DebugMode:        false,
		LogLevel:         "info",
		AllowedPaths:     []string{}, // No restrictions by default
	}
}

func main() {
	config := DefaultConfiguration()

	// Parse command line arguments
	var (
		cacheSize       = flag.String("cache-size", "100MB", "Memory cache limit (e.g., 50MB, 1GB)")
		parallelOps     = flag.Int("parallel-ops", config.ParallelOps, "Max concurrent operations")
		binaryThreshold = flag.String("binary-threshold", "1MB", "File size threshold for binary protocol")
		vsCodeAPI       = flag.Bool("vscode-api", true, "Enable VSCode API integration when available")
		debugMode       = flag.Bool("debug", false, "Enable debug mode")
		logLevel        = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		allowedPaths    = flag.String("allowed-paths", "", "Comma-separated list of allowed base paths for access control")
		version         = flag.Bool("version", false, "Show version information")
		benchmark       = flag.Bool("bench", false, "Run performance benchmark")
	)
	flag.Parse()

	if *version {
		fmt.Printf("MCP Filesystem Server Ultra-Fast v1.0.0\n")
		fmt.Printf("Build: %s\n", time.Now().Format("2006-01-02"))
		fmt.Printf("Go: %s\n", runtime.Version())
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	// Parse cache size
	if size, err := parseSize(*cacheSize); err != nil {
		log.Fatalf("Invalid cache size: %v", err)
	} else {
		config.CacheSize = size
	}

	// Parse binary threshold
	if threshold, err := parseSize(*binaryThreshold); err != nil {
		log.Fatalf("Invalid binary threshold: %v", err)
	} else {
		config.BinaryThreshold = threshold
	}

	config.ParallelOps = *parallelOps
	config.VSCodeAPIEnabled = *vsCodeAPI
	config.DebugMode = *debugMode
	config.LogLevel = *logLevel
	if *allowedPaths != "" {
		config.AllowedPaths = strings.Split(*allowedPaths, ",")
		for i, path := range config.AllowedPaths {
			config.AllowedPaths[i] = strings.TrimSpace(path)
		}
	}

	// Setup logging
	setupLogging(config)

	log.Printf("🚀 Starting MCP Filesystem Server Ultra-Fast")
	log.Printf("📊 Config: Cache=%s, Parallel=%d, Binary=%s, VSCode=%v, AllowedPaths=%v",
		formatSize(config.CacheSize), config.ParallelOps,
		formatSize(config.BinaryThreshold), config.VSCodeAPIEnabled, config.AllowedPaths)

	if *benchmark {
		runBenchmark(config)
		return
	}

	// Initialize components
	ctx := context.Background()

	// Initialize cache system
	cacheSystem, err := cache.NewIntelligentCache(config.CacheSize)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cacheSystem.Close()

	// Initialize core engine
	engine, err := core.NewUltraFastEngine(&core.Config{
		Cache:            cacheSystem,
		ParallelOps:      config.ParallelOps,
		VSCodeAPIEnabled: config.VSCodeAPIEnabled,
		DebugMode:        config.DebugMode,
		AllowedPaths:     config.AllowedPaths,
		BinaryThreshold:  config.BinaryThreshold,
	})
	if err != nil {
		log.Fatalf("Failed to initialize engine: %v", err)
	}
	defer engine.Close()

	// Create MCP server using mark3labs SDK
	s := server.NewMCPServer(
		"filesystem-ultra",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	if err := registerTools(s, engine); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start performance monitoring
	go engine.StartMonitoring(ctx)

	log.Printf("✅ Server ready - Waiting for connections...")

	// Start the stdio server using new API
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// registerTools registers all optimized filesystem tools
func registerTools(s *server.MCPServer, engine *core.UltraFastEngine) error {
	// Read file tool
	readTool := mcp.NewTool("read_file",
		mcp.WithDescription("Read file with ultra-fast caching and memory mapping"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to the file to read")),
	)
	s.AddTool(readTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
		}

		content, err := engine.ReadFileContent(ctx, path)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}
		return mcp.NewToolResultText(content), nil
	})

	// Write file tool
	writeTool := mcp.NewTool("write_file",
		mcp.WithDescription("Write file with atomic operations and backup"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path where to write the file")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Content to write to the file")),
	)
	s.AddTool(writeTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
		}

		content, err := request.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid content: %v", err)), nil
		}

		err = engine.WriteFileContent(ctx, path, content)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)), nil
	})

	// List directory tool
	listTool := mcp.NewTool("list_directory",
		mcp.WithDescription("List directory with intelligent caching"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to the directory to list")),
	)
	s.AddTool(listTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
		}

		listing, err := engine.ListDirectoryContent(ctx, path)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}
		return mcp.NewToolResultText(listing), nil
	})

	// Edit file tool
	editTool := mcp.NewTool("edit_file",
		mcp.WithDescription("Intelligent file editing with backup and rollback"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to the file to edit")),
		mcp.WithString("old_text", mcp.Required(), mcp.Description("Text to be replaced")),
		mcp.WithString("new_text", mcp.Required(), mcp.Description("New text to replace with")),
	)
	s.AddTool(editTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
		}

		oldText, err := request.RequireString("old_text")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid old_text: %v", err)), nil
		}

		newText, err := request.RequireString("new_text")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid new_text: %v", err)), nil
		}

		result, err := engine.EditFile(path, oldText, newText)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ Successfully edited %s\n📊 Changes: %d replacement(s)\n🎯 Match confidence: %s\n📝 Lines affected: %d",
			path, result.ReplacementCount, result.MatchConfidence, result.LinesAffected)), nil
	})

	// Performance stats tool
	statsTool := mcp.NewTool("performance_stats",
		mcp.WithDescription("Get real-time performance statistics"),
	)
	s.AddTool(statsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stats := engine.GetPerformanceStats()
		return mcp.NewToolResultText(stats), nil
	})

	// Capture last artifact tool
	captureLastTool := mcp.NewTool("capture_last_artifact",
		mcp.WithDescription("Store the most recent artifact code in memory"),
		mcp.WithString("content", mcp.Required(), mcp.Description("Artifact code content")),
	)
	s.AddTool(captureLastTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content, err := request.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid content: %v", err)), nil
		}

		err = engine.CaptureLastArtifact(ctx, content)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		lines := strings.Count(content, "\n") + 1
		return mcp.NewToolResultText(fmt.Sprintf("Captured artifact: %d bytes, %d lines", len(content), lines)), nil
	})

	// Write last artifact tool
	writeLastTool := mcp.NewTool("write_last_artifact",
		mcp.WithDescription("Write last captured artifact to file - SPECIFY FULL PATH"),
		mcp.WithString("path", mcp.Required(), mcp.Description("FULL file path including directory and filename (e.g., C:\\temp\\script.py)")),
	)
	s.AddTool(writeLastTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
		}

		err = engine.WriteLastArtifact(ctx, path)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("✅ Wrote last artifact to: %s", path)), nil
	})

	// Artifact info tool
	artifactInfoTool := mcp.NewTool("artifact_info",
		mcp.WithDescription("Get info about last captured artifact"),
	)
	s.AddTool(artifactInfoTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		info := engine.GetLastArtifactInfo()
		return mcp.NewToolResultText(info), nil
	})

	// Search & replace tool
	searchReplaceTool := mcp.NewTool("search_and_replace",
		mcp.WithDescription("Recursive search & replace (text files <=10MB each). Args: path, pattern, replacement"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Base file or directory path")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Regex or literal to search")),
		mcp.WithString("replacement", mcp.Required(), mcp.Description("Replacement text")),
	)
	s.AddTool(searchReplaceTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		pattern, err := request.RequireString("pattern")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		replacement, err := request.RequireString("replacement")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		resp, err := engine.SearchAndReplace(path, pattern, replacement, false)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(resp.Content) > 0 {
			return mcp.NewToolResultText(resp.Content[0].Text), nil
		}
		return mcp.NewToolResultText("No output"), nil
	})

	// Smart search tool
	smartSearchTool := mcp.NewTool("smart_search",
		mcp.WithDescription("Search filenames (and content <=5MB) using regex. Args: path, pattern"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Base directory or file")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Regex or literal pattern")),
	)
	s.AddTool(smartSearchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		pattern, err := request.RequireString("pattern")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		engineReq := localmcp.CallToolRequest{Arguments: map[string]interface{}{"path": path, "pattern": pattern, "include_content": false, "file_types": []interface{}{}}}
		resp, err := engine.SmartSearch(ctx, engineReq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(resp.Content) > 0 {
			return mcp.NewToolResultText(resp.Content[0].Text), nil
		}
		return mcp.NewToolResultText("No matches"), nil
	})

	// Advanced text search tool
	advancedTextSearchTool := mcp.NewTool("advanced_text_search",
		mcp.WithDescription("Advanced content search (default: case-insensitive, no context). Args: path, pattern"),
		mcp.WithString("path", mcp.Required(), mcp.Description("Directory or file")),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("Regex or literal pattern")),
	)
	s.AddTool(advancedTextSearchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		pattern, err := request.RequireString("pattern")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		engineReq := localmcp.CallToolRequest{Arguments: map[string]interface{}{"path": path, "pattern": pattern, "case_sensitive": false, "whole_word": false, "include_context": false, "context_lines": 3}}
		resp, err := engine.AdvancedTextSearch(ctx, engineReq)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(resp.Content) > 0 {
			return mcp.NewToolResultText(resp.Content[0].Text), nil
		}
		return mcp.NewToolResultText("No matches"), nil
	})
	log.Printf("📚 Registered 11 ultra-fast tools (consolidated)")

	return nil
}

// Helper to convert []string -> []interface{} (for building arguments)
func toIfaceSlice(in []string) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	return out
}

// parseSize parses size strings like "50MB", "1GB", etc.
func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	return size * multiplier, nil
}

// formatSize formats bytes to human readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// setupLogging configures logging based on configuration
func setupLogging(config *Configuration) {
	if config.DebugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(log.LstdFlags)
	}
}

// runBenchmark runs performance benchmarks
func runBenchmark(config *Configuration) {
	log.Printf("🧪 Running performance benchmark...")

	// This would run comprehensive benchmarks comparing:
	// 1. This ultra-fast server vs standard MCP
	// 2. Various cache sizes and parallel operation counts
	// 3. Different file sizes and operation types

	fmt.Printf("Benchmark results will be implemented in bench/ package\n")
	fmt.Printf("Run: cd bench && go run benchmark.go\n")
}
