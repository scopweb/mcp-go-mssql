# Contributing to MCP-Go-MSSQL

Thank you for your interest in contributing to MCP-Go-MSSQL! This document provides guidelines and information for contributors.

## 🎯 Project Goals

This project aims to provide a secure, production-ready Microsoft SQL Server connectivity solution for Claude AI integrations, with emphasis on:
- **Security first**: Protection against SQL injection, secure credential handling
- **Legacy support**: Compatibility with older SQL Server versions (2008+)
- **Flexibility**: Support for various deployment scenarios and security requirements

## 🔧 Development Setup

### Prerequisites
- **Go 1.24+** installed
- **Access to SQL Server** (any version 2008+) for testing
- **Git** for version control

### Setup Steps
1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/mcp-go-mssql.git`
3. Install dependencies: `go mod tidy`
4. Set up test environment variables (see `.env.example`)

### Building
```bash
# Development build
go build -o mcp-go-mssql.exe

# Production build
go build -ldflags "-w -s" -o mcp-go-mssql-secure.exe

# Quick build (Windows)
build.bat
```

## 🧪 Testing

### Manual Testing
1. **Connection testing**: `go run test/test-connection.go`
2. **Debug testing**: `go run debug/debug-connection.go`
3. **Claude Desktop integration**: Test with actual Claude Desktop setup

### Test Coverage
Please test your changes with:
- **Modern SQL Server** (2019+)
- **Legacy SQL Server** (2008/2012)
- **Azure SQL Database**
- **Read-only mode** (`MSSQL_READ_ONLY=true`)
- **Various encryption settings**

## 📝 Code Guidelines

### Security Requirements
- **Never hardcode credentials** or sensitive data
- **Always use prepared statements** for SQL queries
- **Sanitize all logging output** to prevent credential leaks
- **Validate all user inputs** before processing
- **Follow the principle of least privilege**

### Code Style
- Follow standard Go conventions (`gofmt`, `go vet`)
- Add meaningful comments for complex logic
- Use descriptive variable and function names
- Handle errors appropriately (return vs log)

### Commit Messages
Use conventional commit format:
```
type(scope): description

- feat: add new feature
- fix: bug fix
- docs: documentation changes
- security: security improvements
- refactor: code refactoring
```

## 🛡️ Security Considerations

### Reporting Security Issues
If you discover a security vulnerability, please:
1. **Do NOT** create a public issue
2. Email the maintainers directly
3. Provide detailed information about the vulnerability
4. Allow time for the issue to be addressed before public disclosure

### Security Best Practices
- Always test with `DEVELOPER_MODE=false` for production scenarios
- Verify TLS encryption is working properly
- Test input validation with malicious inputs
- Ensure no sensitive data appears in logs

## 📋 Issue Guidelines

### Before Creating an Issue
1. Check [existing issues](.github/ISSUES/README.md) for similar problems
2. Review the [troubleshooting section](README.md#troubleshooting)
3. Test with the debug tools in `debug/`

### Issue Types
- **Bug reports**: Use the bug report template
- **Feature requests**: Use the feature request template
- **Security issues**: Contact maintainers privately
- **Documentation**: General issues for doc improvements

## 🔄 Pull Request Process

### Before Submitting
1. **Fork** the repository and create a feature branch
2. **Test** your changes thoroughly (see testing section)
3. **Update documentation** if needed (README.md, CLAUDE.md)
4. **Run** `go fmt` and `go vet`
5. **Check** that no sensitive data is included

### PR Requirements
- [ ] Clear description of changes
- [ ] Related issue referenced
- [ ] Tests performed and documented
- [ ] Documentation updated
- [ ] Security checklist completed
- [ ] No breaking changes (or clearly documented)

### Review Process
1. Automated checks must pass
2. Code review by maintainers
3. Security review for sensitive changes
4. Final testing with various SQL Server versions

## 📚 Documentation

### Required Documentation Updates
When making changes, update relevant documentation:
- **README.md**: User-facing configuration and usage
- **CLAUDE.md**: Claude Code specific instructions
- **.github/ISSUES/**: Document resolved issues
- **Code comments**: For complex or security-critical code

### Documentation Style
- Use clear, concise language
- Provide examples for configuration options
- Include security warnings where appropriate
- Keep examples up to date with current features

## 🏗️ Architecture

### Key Components
- **Connection Management**: `buildSecureConnectionString()`
- **Security Validation**: `validateReadOnlyQuery()`, `validateBasicInput()`
- **MCP Protocol**: Request/response handling
- **Logging**: Security-aware logging with sanitization

### Adding New Features
1. **Consider security implications** first
2. **Maintain backward compatibility** when possible
3. **Add appropriate configuration options**
4. **Include comprehensive error handling**
5. **Update all relevant documentation**

## 📞 Getting Help

- **General questions**: Create a discussion or issue
- **Bug reports**: Use the bug report template
- **Security concerns**: Contact maintainers privately
- **Feature ideas**: Use the feature request template

## 📄 License

By contributing to this project, you agree that your contributions will be licensed under the same license as the project.

---

Thank you for contributing to MCP-Go-MSSQL! Your efforts help make secure database connectivity accessible to the Claude AI community.