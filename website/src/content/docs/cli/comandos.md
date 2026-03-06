---
title: Comandos del CLI
description: Lista completa de comandos disponibles en el CLI de Claude Code
---

## Comandos disponibles

El CLI de Claude Code proporciona varios comandos para interactuar con bases de datos MSSQL.

### Uso básico

```bash
go run claude-code/db-connector.go [comando] [argumentos]
```

## Comandos

### test

Prueba la conexión a la base de datos.

```bash
go run claude-code/db-connector.go test
```

**Salida**: Confirma si la conexión fue exitosa o muestra errores de conexión.

### info

Obtiene información general sobre la base de datos conectada.

```bash
go run claude-code/db-connector.go info
```

**Salida**: Información del servidor, versión, y configuración.

### tables

Lista todas las tablas disponibles en la base de datos.

```bash
go run claude-code/db-connector.go tables
```

**Salida**: Lista de nombres de tablas con sus esquemas.

### describe

Describe la estructura de una tabla específica.

```bash
go run claude-code/db-connector.go describe TABLE_NAME
```

**Argumentos**:
- `TABLE_NAME`: Nombre de la tabla a describir

**Salida**: Columnas, tipos de datos, restricciones y índices de la tabla.

### query

Ejecuta una consulta SQL personalizada.

```bash
go run claude-code/db-connector.go query "SELECT * FROM tabla WHERE condicion"
```

**Argumentos**:
- Consulta SQL entre comillas

**Salida**: Resultados de la consulta en formato tabular.

**Nota de seguridad**: El CLI utiliza prepared statements para prevenir inyección SQL.

## Variables de entorno requeridas

Antes de ejecutar cualquier comando, asegúrate de configurar las variables de entorno:

```bash
# Copiar plantilla
cp .env.example .env

# Editar .env con tus credenciales
# Cargar variables (Linux/Mac)
source .env

# Windows PowerShell
Get-Content .env | ForEach-Object {
  $name, $value = $_ -split '=', 2
  [Environment]::SetEnvironmentVariable($name, $value)
}
```

Variables requeridas:
- `MSSQL_SERVER`: Servidor SQL
- `MSSQL_DATABASE`: Base de datos
- `MSSQL_USER`: Usuario
- `MSSQL_PASSWORD`: Contraseña

Variables opcionales:
- `MSSQL_PORT`: Puerto (predeterminado: 1433)
- `DEVELOPER_MODE`: Modo desarrollo (true/false)
- `MSSQL_READ_ONLY`: Modo solo lectura (true/false)
- `MSSQL_WHITELIST_TABLES`: Tablas permitidas en modo solo lectura

## Ejemplos de uso

### Probar conexión

```bash
go run claude-code/db-connector.go test
```

### Listar tablas

```bash
go run claude-code/db-connector.go tables
```

### Ver estructura de tabla

```bash
go run claude-code/db-connector.go describe Users
```

### Ejecutar consulta SELECT

```bash
go run claude-code/db-connector.go query "SELECT TOP 10 * FROM Users WHERE Active = 1"
```

### Consulta con JOIN

```bash
go run claude-code/db-connector.go query "SELECT u.Name, o.OrderDate FROM Users u JOIN Orders o ON u.Id = o.UserId"
```

## Seguridad

El CLI implementa las mismas características de seguridad que el servidor MCP:

- Conexiones TLS cifradas
- Prepared statements para prevenir SQL injection
- Validación de entrada
- Logging de seguridad
- Soporte para modo solo lectura

Ver [Seguridad](/seguridad/resumen) para más detalles.

## Troubleshooting

### Error: "Database not connected"

Verifica que las variables de entorno estén configuradas:

```bash
echo "Server: $MSSQL_SERVER"
echo "Database: $MSSQL_DATABASE"
echo "User: $MSSQL_USER"
```

### Error de certificado TLS

Si ves errores de certificado, establece `DEVELOPER_MODE=true` para desarrollo:

```bash
export DEVELOPER_MODE=true
```

**Advertencia**: No uses `DEVELOPER_MODE=true` en producción.

### Error de red

Verifica firewall y que el puerto SQL Server (1433) esté abierto.

## Próximos pasos

- [Variables de entorno](/configuracion/variables-entorno)
- [Seguridad](/seguridad/resumen)
- [Solución de problemas](/despliegue/solucion-problemas)
