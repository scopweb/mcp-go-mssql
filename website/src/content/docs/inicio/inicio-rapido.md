---
title: Inicio rápido
description: Puesta en marcha rápida de MCP-Go-MSSQL
---

## En 3 pasos

### 1. Configurar credenciales

```bash
cp .env.example .env
# Editar .env con tus datos de conexión
```

### 2. Compilar

```bash
go build -o mcp-go-mssql.exe
```

### 3. Ejecutar

**Como servidor MCP (para Claude Desktop):**
```bash
./mcp-go-mssql.exe
```

**Como herramienta CLI (para Claude Code):**
```bash
go run examples/claude-code/db-connector.go test
go run examples/claude-code/db-connector.go tables
go run examples/claude-code/db-connector.go query "SELECT TOP 10 * FROM mi_tabla"
```

## Ejemplo de configuración Claude Desktop

Añade esto a tu `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "mi-base-de-datos": {
      "command": "C:\\ruta\\a\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "mi-servidor.database.windows.net",
        "MSSQL_DATABASE": "MiBaseDeDatos",
        "MSSQL_USER": "mi_usuario",
        "MSSQL_PASSWORD": "mi_contraseña",
        "MSSQL_PORT": "1433",
        "MSSQL_READ_ONLY": "true",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

## Herramientas disponibles

Una vez conectado, Claude Desktop tendrá acceso a estas herramientas:

| Herramienta | Descripción |
|-------------|-------------|
| `get_database_info` | Estado de conexión, cifrado y modo de acceso |
| `query_database` | Ejecutar consultas SQL de forma segura |
| `explore` | Listar tablas, vistas, bases de datos y procedimientos; buscar objetos |
| `inspect` | Estructura de una tabla: columnas, índices, claves foráneas, dependencias |
| `execute_procedure` | Ejecutar procedimientos almacenados autorizados |
| `explain_query` | Plan de ejecución estimado sin ejecutar la consulta |

En modo dinámico se exponen además: `dynamic_available`, `dynamic_connect`, `dynamic_disconnect`, `dynamic_list` y `confirm_operation`.

## Siguiente paso

Consulta la sección [Herramientas MCP](/herramientas-mcp/resumen/) para conocer cada herramienta en detalle.
