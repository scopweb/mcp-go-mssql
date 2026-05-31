//go:build windows

package main

// Windows Integrated Authentication support for the example CLI tool.
// This file is excluded on non-Windows platforms.
import _ "github.com/microsoft/go-mssqldb/integratedauth/winsspi"
