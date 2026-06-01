---
title: Variables de entorno
description: Referencia completa de variables de entorno de MCP-Go-MSSQL
---

Todas las credenciales y opciones de configuración se gestionan mediante variables de entorno. Nunca hardcodees credenciales en el código fuente.

## Variables requeridas

| Variable | Descripción | Ejemplo |
|----------|-------------|---------|
| `MSSQL_SERVER` | Hostname o IP del servidor SQL | `prod-server.database.windows.net` |
| `MSSQL_DATABASE` | Nombre de la base de datos | `ProductionDB` |
| `MSSQL_USER` | Usuario de SQL Server | `app_user` |
| `MSSQL_PASSWORD` | Contraseña de SQL Server | _(secreto)_ |

## Variables opcionales

| Variable | Default | Descripción |
|----------|---------|-------------|
| `MSSQL_PORT` | `1433` | Puerto de SQL Server |
| `DEVELOPER_MODE` | `false` | `true` para desarrollo (TLS relajado, errores detallados) |
| `MSSQL_READ_ONLY` | `false` | Bloquea operaciones de escritura |
| `MSSQL_WHITELIST_TABLES` | _(vacío)_ | Tablas permitidas para modificación en modo read-only |
| `MSSQL_AUTH` | `sql` | Modo de autenticación: `sql`, `integrated`, `azure` |
| `MSSQL_ENCRYPT` | _(auto)_ | Control de cifrado TLS. Solo efectivo con `DEVELOPER_MODE=true`. `false` = desactivar cifrado (**necesario para SQL Server 2008/2012**). Si no se define: `false` en dev, siempre `true` en producción |
| `MSSQL_CONNECTION_STRING` | _(vacío)_ | Connection string personalizado (anula otras variables) |
| `MSSQL_DYNAMIC_MODE` | _(auto-detect)_ | `true` = forzar modo dinámico (múltiples alias). `false` = forzar modo clásico (única conexión). Si no se define, se auto-detecta por presencia de variables `MSSQL_DYNAMIC_*`. **Importante para aislamiento entre múltiples servidores MCP.** |
| `MSSQL_IGNORE_LOCAL_ENV` | `false` | `true` = ignora completamente cualquier archivo `.env` situado junto al ejecutable. Muy útil para servidores clásicos configurados 100% vía `.mcp.json` cuando hay riesgo de archivos `.env` residuales. |

## Precedencia: Modo Clásico vs Dinámico (Importante)

El servidor decide automáticamente si funciona en **modo clásico** (una sola base de datos) o **modo dinámico** (múltiples alias) siguiendo este orden de prioridad:

| Prioridad | Condición | Resultado |
|-----------|-----------|---------|
| 1 | `MSSQL_DYNAMIC_MODE=false` (o `0`, `no`, `off`) | **Siempre clásico** (ignora todo lo demás) |
| 2 | `MSSQL_DYNAMIC_MODE=true` | **Siempre dinámico** |
| 3 | Existe `MSSQL_SERVER`, `MSSQL_CONNECTION_STRING` o `MSSQL_DATABASE` | **Clásico** (protege tus configuraciones normales en `.mcp.json`) |
| 4 | Solo existen variables `MSSQL_DYNAMIC_*` | Dinámico (auto-detección) |

**Esto soluciona el problema más común:**
Si configuras un servidor de forma clásica en Claude Desktop (con `MSSQL_SERVER` + credenciales en el bloque `"env"`), ahora **no debería activar modo dinámico** aunque tengas un `.env` cerca o variables heredadas del sistema.

### Receta recomendada: Servidor Clásico Totalmente Aislado

Si tienes varios servidores MCP (por ejemplo: uno dinámico para varias bases y varios clásicos para bases específicas), usa esta configuración en las instancias **clásicas**:

```json
{
  "mcpServers": {
    "mssql2": {
      "command": "C:\\MCPs\\MCP-EXE\\mssql2\\sinenv\\mcp-go-mssql-secure.exe",
      "args": [],
      "env": {
        "MSSQL_SERVER": "10.203.3.10",
        "MSSQL_DATABASE": "JJP_TRANSFER",
        "MSSQL_USER": "userTRANSFER",
        "MSSQL_PASSWORD": "tu_password",
        "DEVELOPER_MODE": "true",
        "MSSQL_READ_ONLY": "false",

        "MSSQL_IGNORE_LOCAL_ENV": "true",
        "MSSQL_DYNAMIC_MODE": "false"
      }
    }
  }
}
```

**Por qué estas dos líneas:**
- `MSSQL_IGNORE_LOCAL_ENV=true` → Ignora cualquier `.env` que esté en la misma carpeta del ejecutable.
- `MSSQL_DYNAMIC_MODE=false` → Fuerza modo clásico aunque el entorno del proceso padre tenga variables dinámicas.

Añade estas dos variables en **todas** tus instancias clásicas cuando uses varios servidores a la vez.

### Problemas comunes (Troubleshooting)

**"Sigo viendo las herramientas `dynamic_available` y `dynamic_connect` en un servidor que debería ser clásico"**

Causas más frecuentes y soluciones:

1. **No has reiniciado Claude Desktop** después de cambiar el `.mcp.json` → Cierra completamente Claude (todas las ventanas) y vuelve a abrirlo.
2. **El binario antiguo sigue en uso** → Asegúrate de haber reemplazado el `.exe` por la versión nueva (a partir del commit `0bf02d5`).
3. **Tienes `MSSQL_DYNAMIC_MODE` sin poner o puesto en `true`** en esa instancia → Añade explícitamente `"MSSQL_DYNAMIC_MODE": "false"`.
4. **Hay un `.env` en la misma carpeta** y no estás usando `MSSQL_IGNORE_LOCAL_ENV` → Añade esa variable.
5. **Variables dinámicas en el entorno del usuario / PowerShell profile** → La forma más robusta es usar las dos variables anteriores.

Si después de poner las dos variables de aislamiento sigue ocurriendo, activa `DEVELOPER_MODE=true` temporalmente y revisa los logs del servidor al arrancar. Debería aparecer claramente: `DYNAMIC_MODE=false (classic single-connection mode)`.

## Plantilla .env

```bash
# Copiar y editar
cp .env.example .env

# Ejemplo de contenido
MSSQL_SERVER=localhost
MSSQL_DATABASE=MyDB
MSSQL_USER=sa
MSSQL_PASSWORD=YourPassword123
MSSQL_PORT=1433
DEVELOPER_MODE=true
MSSQL_READ_ONLY=false
```

## Cargar variables

**Linux/macOS:**
```bash
source .env
```

**Windows PowerShell:**
```powershell
Get-Content .env | ForEach-Object {
  $name, $value = $_ -split '=', 2
  [Environment]::SetEnvironmentVariable($name, $value)
}
```

## Permisos de archivo

```bash
# Linux/macOS
chmod 600 .env

# Windows
icacls .env /inheritance:r /grant:r "%USERNAME%:R"
```
