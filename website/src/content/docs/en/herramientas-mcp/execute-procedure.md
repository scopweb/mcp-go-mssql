---
title: execute_procedure
description: Execute an authorized stored procedure
---

Executes a stored procedure that is included in the authorized procedures list (whitelist).

## Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `procedure_name` | string | Yes | Name of the procedure to execute |
| `parameters` | string | No | JSON object with parameter names and values |

## Required configuration

To use this tool, you must configure the environment variable:

```bash
MSSQL_WHITELIST_PROCEDURES="sp_GetCustomerOrders,sp_GenerateReport"
```

## Usage example

```json
{
  "name": "execute_procedure",
  "arguments": {
    "procedure_name": "sp_GetCustomerOrders",
    "parameters": "{\"customer_id\": 123}"
  }
}
```

## Security

- **Only executes whitelisted procedures** — Any unauthorized procedure is rejected
- **Name validation** — Names are validated with regex `^[\w.\[\]]+$` to prevent injection
- **Dangerous procedures blocked** — `xp_cmdshell`, `sp_configure`, `sp_executesql` and others are explicitly blocked even if added to the whitelist
- **Security logging** — Each execution is recorded in the security logs

## Safe system procedures

In read-only mode, the following system procedures are allowed without needing to be whitelisted:

- `sp_help`, `sp_helptext`, `sp_helpindex`
- `sp_columns`, `sp_tables`
- `sp_fkeys`, `sp_pkeys`
- `sp_databases`
