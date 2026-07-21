# MCP Go MSSQL

MCP server y CLI en Go para dar acceso seguro a Microsoft SQL Server desde Claude Desktop, Claude Code y otros clientes MCP.

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Documentación completa: [mcp-go-mssql.scopweb.com](https://mcp-go-mssql.scopweb.com)

## Qué hace

- Servidor MCP (stdio) con 6 herramientas base: `query_database`, `get_database_info`, `explore`, `inspect`, `execute_procedure`, `explain_query`.
- Modo dinámico opcional para varias bases de datos con un solo servidor (añade `dynamic_available`, `dynamic_connect`, `dynamic_disconnect`, `dynamic_list`, `confirm_operation`).
- CLI independiente para uso directo desde terminal (`examples/claude-code/db-connector.go`).

Pensado para entornos donde la seguridad importa: modo solo lectura con whitelist por tabla, prepared statements, TLS fail-closed en producción y soporte explícito para SQL Server 2008/2012 (TLS 1.0).

## Instalación

Descarga el binario de la [última release](https://github.com/scopweb/mcp-go-mssql/releases/latest) o compila:

```bash
go build -ldflags "-w -s" -o mcp-go-mssql.exe
```

Requiere Go 1.26+.

## Configuración (Claude Desktop)

```json
{
  "mcpServers": {
    "mssql": {
      "command": "C:\\MCPs\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "your-server.database.windows.net",
        "MSSQL_DATABASE": "YourDatabase",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "your_password",
        "DEVELOPER_MODE": "false",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai"
      }
    }
  }
}
```

Con `MSSQL_READ_ONLY=true` el agente solo puede hacer SELECT, salvo en las tablas de la whitelist. `MSSQL_WHITELIST_TABLES` es opcional; sin ella no se permite ninguna modificación.

Variables principales:

| Variable | Descripción |
|----------|-------------|
| `MSSQL_SERVER` / `MSSQL_PORT` | Host y puerto (1433 por defecto) |
| `MSSQL_DATABASE` | Base de datos (opcional con auth integrada) |
| `MSSQL_USER` / `MSSQL_PASSWORD` | Credenciales SQL |
| `MSSQL_AUTH` | `sql` (defecto), `integrated`/`windows` (SSPI, sin contraseña) |
| `MSSQL_CONNECTION_STRING` | DSN completo; anula el resto de variables `MSSQL_*` |
| `DEVELOPER_MODE` | `false` = producción (TLS estricto, errores genéricos) |
| `MSSQL_READ_ONLY` / `MSSQL_WHITELIST_TABLES` | Control de escritura |
| `MSSQL_WHITELIST_PROCEDURES` | Procedimientos que puede ejecutar `execute_procedure` |
| `MSSQL_ALLOW_INSECURE_TLS` | Permite TLS inseguro en producción (solo servidores legacy) |

La lista completa está en [.env.example](.env.example) y en la [documentación](https://mcp-go-mssql.scopweb.com/configuracion/variables-entorno).

## SQL Server legacy (2008/2012, TLS 1.0)

El runtime de Go actual no negocia TLS 1.0. Para estos servidores usa un connection string en formato URL:

```ini
MSSQL_CONNECTION_STRING=sqlserver://user:password@legacy-server:1433?database=LegacyDB&encrypt=disable&trustservercertificate=true
```

En producción (`DEVELOPER_MODE=false`) esto falla al arrancar por diseño (fail-closed); hay que confirmarlo explícitamente con `MSSQL_ALLOW_INSECURE_TLS=true`, que deja un `WARN` auditable en el log de cada conexión.

## Modo dinámico (varias bases de datos)

Si necesitas acceder a varias bases de datos relacionadas con una sola instancia del servidor:

```ini
MSSQL_DYNAMIC_MODE=true
MSSQL_DYNAMIC_APP_MAIN_SERVER=sql01.local
MSSQL_DYNAMIC_APP_MAIN_DATABASE=AppDB
MSSQL_DYNAMIC_APP_MAIN_USER=ai_reader
MSSQL_DYNAMIC_APP_MAIN_PASSWORD=...
# Cada alias es read-only por defecto
```

Reglas importantes:

- Cada alias nace en `READ_ONLY=true`.
- Un alias escribible (`_READ_ONLY=false`) sin `_WHITELIST_TABLES` no permite modificar nada.
- Con whitelist, las modificaciones piden `confirm_operation` primero.
- No uses `MSSQL_READ_ONLY=false` global en modo dinámico.

Guía completa: [modo dinámico](https://mcp-go-mssql.scopweb.com/guias/modo-dinamico-una-aplicacion-varias-bases).

## CLI (Claude Code / terminal)

```bash
go run examples/claude-code/db-connector.go test                    # probar conexión
go run examples/claude-code/db-connector.go tables                  # listar tablas
go run examples/claude-code/db-connector.go describe MiTabla        # estructura
go run examples/claude-code/db-connector.go query "SELECT TOP 10 * FROM MiTabla"
```

## Troubleshooting

| Error | Causa | Solución |
|-------|-------|----------|
| `certificate signed by unknown authority` | Certificado autofirmado | `DEVELOPER_MODE=true` (solo dev) |
| `protocol version 301` / TLS handshake | Servidor con TLS 1.0 (2008/2012) | Connection string URL con `encrypt=disable` (ver arriba) |
| `Refusing to start... MSSQL_ALLOW_INSECURE_TLS` | TLS inseguro en producción | Corrige el cifrado o confirma con `MSSQL_ALLOW_INSECURE_TLS=true` |
| `Database not connected` | Variables sin definir | Revisa `.env` (ver `.env.example`) |
| `read-only mode: only SELECT...` | Modo solo lectura activo | Añade la tabla a `MSSQL_WHITELIST_TABLES` o ejecuta el SQL manualmente |

Probar la conexión sin levantar el servidor: `go run test/test-connection.go`.

## Estructura del repo

```
main.go                      servidor MCP
statistics_wrapper.go        wrappers SET STATISTICS en modo read-only
messagesink.go               captura de mensajes info del driver
examples/claude-code/        CLI (db-connector.go)
test/                        utilidad de test de conexión
scripts/                     scripts auxiliares
website/                     documentación (Astro/Starlight)
.env.example                 plantilla de variables
config.example.json          plantilla para Claude Desktop
```

## Notas de seguridad

- La whitelist y el modo read-only son defensa en profundidad; el control real de escritura debe estar en la base de datos (login con mínimos privilegios).
- No subas `.env` ni credenciales al repo. Solo `.env.example` y `config.example.json`.
- Política de seguridad y reporte de vulnerabilidades: [SECURITY.md](SECURITY.md).

## Licencia

MIT — ver [LICENSE](LICENSE).
