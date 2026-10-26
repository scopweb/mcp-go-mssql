---
title: Corporate Vault
description: Load SQL credentials from a private HTTPS vault with per-alias references, an IIS reverse proxy and startup diagnostics.
---

The MCP server can retrieve passwords or connection strings from the corporate vault API:
`GET /api/secrets/{environment}`, authenticated with `X-Api-Key`, returning a JSON object of secret names and values.
This is not a generic HashiCorp Vault client. The standalone CLI does not use this integration.

## Prepare the application

1. Publish the vault over **HTTPS** with a certificate trusted by the machine running the MCP server.
2. Create an application for the MCP server in VaultAdmin and store its API key locally. The `admin` password is not used.
3. Create secrets under `dev`, `staging` or `prod`. Names are matched exactly, including case.
4. Configure the `.env` next to the actual MCP executable and restart the process.

:::caution[Bootstrap credential]
The API key remains a local secret supplied by the host. Never publish the `.env`, keys, passwords,
real connection strings or screenshots containing them. All examples use fictitious values.
:::

## Dynamic configuration

```dotenv title=".env next to the executable — example"
MSSQL_VAULT_URL=https://vault.example.com/vault/
MSSQL_VAULT_API_KEY=<application-api-key>
MSSQL_VAULT_ENVIRONMENT=prod
MSSQL_DYNAMIC_MODE=true
MSSQL_READ_ONLY=true

MSSQL_DYNAMIC_APP_MAIN_SERVER=sql01.example.com
MSSQL_DYNAMIC_APP_MAIN_DATABASE=AppDB
MSSQL_DYNAMIC_APP_MAIN_USER=ai_reader
MSSQL_DYNAMIC_APP_MAIN_READ_ONLY=true
MSSQL_DYNAMIC_APP_MAIN_PASSWORD_VAULT_SECRET=MainSqlPassword
```

Store the password as `MainSqlPassword` in that application and environment. Remove the previous
`MSSQL_DYNAMIC_APP_MAIN_PASSWORD`. For a complete connection string, use
`MSSQL_DYNAMIC_APP_MAIN_CONNECTION_STRING_VAULT_SECRET=MainSqlConnection` instead and remove the local DSN.
Do not combine a password reference with a connection string that would override it.

In classic mode, use `MSSQL_PASSWORD_VAULT_SECRET=SqlPassword` or
`MSSQL_CONNECTION_STRING_VAULT_SECRET=SqlConnection`. Keep server, database and user when retrieving
only the password. Do not configure a local value and a reference for the same destination.

All references use one application and environment. SQL permissions and allowlists remain local
configuration, not vault settings. SQL Server TLS rules still apply to the retrieved credentials.

## Loading and rotation

- One request at startup, before `initialize`, with a **10-second** timeout and a **1 MiB** response limit.
- The vault is contacted only when a `*_VAULT_SECRET` reference exists. Redirects and ambient HTTP proxies are not used.
- Referenced secrets stay in a private in-memory configuration; they are not written to disk or exported to the process environment.
- Missing/empty secrets, ambiguous sources and vault failures stop startup. There is no local fallback.
- Restart the MCP after rotating a password or API key. There is no periodic refresh.
- An executable under `build/` loads `build/.env`, not the repository-root `.env`.
- Values supplied by the MCP client take precedence. `MSSQL_IGNORE_LOCAL_ENV=true` skips the file entirely.

:::note[HTTPS is mandatory]
Neither `DEVELOPER_MODE` nor `MSSQL_ALLOW_INSECURE_TLS` relaxes vault HTTPS. There is no option to
allow HTTP or untrusted certificates. Port 8443 alone does not provide encryption.
:::

## IIS reverse proxy under /vault/

Enable ARR and URL Rewrite and configure an HTTPS binding with a trusted certificate. Add this rule
inside `configuration/system.webServer/rewrite/rules`, **before any catch-all rule**. Replace the
example backend and keep existing rules for other applications:

```xml title="web.config — additional rule"
<rule name="VaultReverseProxy" stopProcessing="true">
  <match url="^vault(?:/(.*))?$" />
  <action type="Rewrite"
          url="http://vault.internal:8443/{R:1}"
          appendQueryString="true" />
</rule>
```

Set `MSSQL_VAULT_URL=https://gateway.example.com/vault/`. A request for
`/vault/api/secrets/prod` is forwarded as `/api/secrets/prod`, preserving `X-Api-Key`.
Use rewriting, not redirects. The client supports the base-path prefix.

```powershell
Invoke-RestMethod "https://gateway.example.com/vault/health"
# Healthy
Invoke-RestMethod "https://gateway.example.com/vault/api/environments"
# dev, staging, prod
```

Restrict `/vault/` to the authorized network or VPN, including its administrative endpoints.
Proxy HTTPS secures MCP → IIS; the example HTTP backend leaves IIS → vault unencrypted.
Protect that network segment or configure backend TLS too. These probes do not validate secrets or SQL access.

## Troubleshooting

| Symptom | What to check |
|---|---|
| `Vault configuration failed` and MCP will not load | Failure happens before MCP initialization. Check stderr, executable path and its `.env`. |
| `MSSQL_VAULT_URL must be an HTTPS URL...` | Use HTTPS without embedded credentials, query or fragment; check host settings that may override the file. |
| `vault request failed` | DNS, connectivity, certificate, trusted CA and timeout. Do not disable TLS validation. |
| HTTP 301/302 | Configure the final URL: redirects are not followed. |
| HTTP 401 | Application API key, rotation or a disabled application. |
| HTTP 500 | Vault API exception; inspect logs on the machine hosting the API. |
| Missing or empty secret | Exact name, application and environment. |
| Initial `disconnected` in dynamic mode | Normal: use `dynamic_available`, then `dynamic_connect` with an alias. |
| SQL Server 18456 after vault retrieval | SQL login rejected: check account, password and database access. The code alone does not prove an incorrect password. |
| SQL timeout | SQL host, port and connectivity; this is separate from vault retrieval. |

See the [environment reference](/en/configuracion/variables-entorno/) and
[dynamic mode guide](/en/guias/modo-dinamico-una-aplicacion-varias-bases/).
