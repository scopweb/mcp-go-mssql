---
title: Audit & Logging
description: Security logging and audit system in MCP-Go-MSSQL
---

MCP-Go-MSSQL includes a dedicated security logging system that records relevant events without exposing sensitive data.

## SecurityLogger

The `SecurityLogger` component handles recording all security events with automatic sanitization.

### Recorded events

- Database connection attempts (success and failure)
- Queries blocked by read-only mode
- Access denied to tables outside the whitelist
- Detected SQL injection attempts
- Input validation errors

### Automatic sanitization

The logger automatically removes sensitive data before writing to disk:

- Passwords and tokens
- Full connection strings
- User data in queries

## Log format

Security logs are written in a structured format with the following fields:

| Field | Description |
|-------|-------------|
| `timestamp` | UTC date and time of the event |
| `level` | Level: INFO, WARN, ERROR, SECURITY |
| `event` | Security event type |
| `source` | Component that generated the event |
| `message` | Sanitized event description |

## Configuration

Security logging is enabled by default and cannot be disabled. Error messages to the client are always generic in production mode (`DEVELOPER_MODE=false`), while technical details are recorded internally.

## Best practices

1. Review security logs periodically
2. Set up alerts for SECURITY-level events
3. Rotate and archive logs according to retention policies
4. Do not expose log files to unauthorized users
