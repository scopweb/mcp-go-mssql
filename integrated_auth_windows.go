//go:build windows

package main

// Integrated Windows Authentication support (Windows only).
//
// This file is intentionally empty except for the blank import.
// The real implementation lives in the go-mssqldb driver.
//
// On non-Windows platforms this file is excluded by the build tag above,
// allowing `go build`, `go test`, and govulncheck to succeed on Linux CI runners.
import _ "github.com/microsoft/go-mssqldb/integratedauth/winsspi"
