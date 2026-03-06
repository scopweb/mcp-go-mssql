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
cd claude-code
go run db-connector.go test
go run db-connector.go tables
go run db-connector.go query "SELECT TOP 10 * FROM mi_tabla"
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
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ia",
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
| `list_tables` | Listar tablas y vistas |
| `describe_table` | Estructura de columnas (soporta `schema.tabla`) |
| `list_databases` | Listar bases de datos del servidor |
| `get_indexes` | Índices de una tabla |
| `get_foreign_keys` | Relaciones de claves foráneas |
| `list_stored_procedures` | Listar procedimientos almacenados |
| `execute_procedure` | Ejecutar procedimientos almacenados autorizados |

## Siguiente paso

Consulta la sección [Herramientas MCP](/herramientas-mcp/resumen/) para conocer cada herramienta en detalle.
