# ❓ FAQ #8: Configurar MCP MSSQL con conexiones dinámicas en Grok Build

> **📌 Recomendado**: Si tu caso es "una sola aplicación que necesita acceder a varias bases de datos relacionadas", lee primero esta guía más específica y segura:  
> **[Modo Dinámico para Una Aplicación con Varias Bases de Datos Relacionadas](./modo-dinamico-una-aplicacion-varias-bases.md)**

**Etiquetas**: `mssql`, `dynamic-connections`, `grok-build`, `configuration`, `faq`

## 🎯 **Problema**

Tengo el servidor MCP MSSQL con modo dinámico (`MSSQL_DYNAMIC_MODE=true`) y múltiples alias preconfigurados (CRM, IDENTITY, FERRATGE, etc.) en un archivo `.env`. 

Funciona en Claude Desktop y Claude Code, pero **¿cómo lo configuro correctamente en Grok Build**? La configuración difiere un poco.

## ✅ **Respuesta y Solución**

### **Diferencias clave con Claude Desktop / Claude Code**

| Aspecto                        | Claude Desktop / Code                          | Grok Build (TUI)                                      |
|--------------------------------|------------------------------------------------|-------------------------------------------------------|
| Archivo de configuración       | `claude_desktop_config.json` o `.claude.json`  | `~/.grok/config.toml` (o `.grok/config.toml` por proyecto) |
| Variables de entorno           | Se ponen en el bloque `"env": { ... }`         | Se pueden poner en `env = { ... }`, pero **no es necesario** para este servidor |
| Carga del `.env`               | El servidor busca `.env` relativo al exe       | Igual, el servidor busca `.env` relativo al ejecutable |
| Gestión en tiempo real         | Reiniciar Claude                               | Usar `/mcps` (modal) o `Ctrl+L` → habilitar/deshabilitar sin reiniciar Grok |
| Logs del servidor              | Logs de Claude                                 | `~/.grok/logs/mcp/mssql.stderr.log` (estructurado JSON) |
| Añadir nuevo alias dinámico    | Editar `.env` + reiniciar Claude               | Editar `.env` + recargar en el modal `/mcps`          |

### **📋 Configuración Recomendada en Grok Build**

#### 1. Configuración mínima (recomendada)

Edita `~/.grok/config.toml`:

```toml
[mcp_servers.mssql]
command = "C:\\MCPs\\MCP-EXE\\mssql2\\mcp-go-mssql-secure.exe"
args = []
enabled = true

# IMPORTANTE:
# No hace falta poner las variables MSSQL_DYNAMIC_* aquí.
# El servidor las carga automáticamente del archivo .env
# que está en el mismo directorio que el ejecutable.
```

#### 2. Alternativa usando `grok mcp add` (CLI)

```bash
grok mcp add mssql `
  --command "C:\MCPs\MCP-EXE\mssql2\mcp-go-mssql-secure.exe" `
  --env ""   # No necesitamos pasar variables aquí
```

Luego verifica:

```bash
grok mcp list
```

#### 3. Configuración por proyecto (opcional)

Si solo quieres el MCP MSSQL activo en proyectos concretos de JJP:

```
C:\__REPOS\jotajotape\CRM\.grok\config.toml
```

```toml
[mcp_servers.mssql]
command = "C:\\MCPs\\MCP-EXE\\mssql2\\mcp-go-mssql-secure.exe"
enabled = true
```

### **🔄 Cómo funciona el modo dinámico en Grok**

1. Grok lanza `mcp-go-mssql-secure.exe`
2. El servidor (Go) carga automáticamente el archivo `.env` situado **al lado del ejecutable**:
   ```
   C:\MCPs\MCP-EXE\mssql2\.env
   ```
3. Lee todas las variables `MSSQL_DYNAMIC_<ALIAS>_*`
4. Las herramientas `dynamic_available`, `dynamic_connect`, `dynamic_list` y `dynamic_disconnect` quedan disponibles.

Ejemplo de alias en el `.env`:

```env
MSSQL_DYNAMIC_MODE=true
DEVELOPER_MODE=true
MSSQL_AUTOPILOT=true

MSSQL_DYNAMIC_CRM_SERVER=10.203.3.10
MSSQL_DYNAMIC_CRM_DATABASE=JJP_CRM
MSSQL_DYNAMIC_CRM_USER=sa
MSSQL_DYNAMIC_CRM_PASSWORD=...
MSSQL_DYNAMIC_CRM_READ_ONLY=true
MSSQL_DYNAMIC_CRM_AUTOPILOT=true

# Puedes añadir más alias fácilmente:
MSSQL_DYNAMIC_NUEVO_PROYECTO_SERVER=...
```

### **🧪 Verificación después de configurar**

1. Reinicia Grok Build (o usa el modal en tiempo real).
2. Abre el modal de MCPs:
   - Escribe `/mcps` 
   - O pulsa `Ctrl+L` y ve a la pestaña de MCP servers
3. Verifica que `mssql` aparece como **running** (verde).
4. En la conversación, ejecuta:

```
Usa la herramienta dynamic_available para listar las conexiones disponibles.
```

Deberías ver algo como:

```
Available dynamic connections:
- IDENTITY (10.203.3.11/JJP_CRM_IDENTITY)
- CRM (10.203.3.10/JJP_CRM)
- FERRATGE (SQL01/JJP_Ferratge_DEV)
- ...
```

5. Conéctate a la que necesites:

```
Conéctate dinámicamente a CRM usando dynamic_connect con alias "CRM".
```

### **📁 Ubicación de logs (muy útil para depurar)**

```powershell
# Logs específicos del servidor mssql
tail -f $env:USERPROFILE\.grok\logs\mcp\mssql.stderr.log

# O abre con VS Code / editor
code $env:USERPROFILE\.grok\logs\mcp\mssql.stderr.log
```

Busca mensajes como:
- `Dynamic multi-connection mode enabled`
- `Dynamic connection 'CRM' activated`
- Errores de credenciales o conexión

### **⚠️ Problemas Comunes y Soluciones**

#### Problema 1: `dynamic_available` devuelve lista vacía

**Causa probable**: El servidor no encontró el `.env` o `MSSQL_DYNAMIC_MODE` no está activo.

**Solución**:
1. Verifica que el archivo existe: `C:\MCPs\MCP-EXE\mssql2\.env`
2. Confirma que tiene `MSSQL_DYNAMIC_MODE=true`
3. En el modal `/mcps`, deshabilita y vuelve a habilitar el servidor `mssql`
4. Revisa los logs (`mssql.stderr.log`) buscando "no connections configured in .env"

#### Problema 2: El servidor aparece en error / no arranca

**Solución**:
- Prueba lanzar manualmente el exe desde su carpeta para ver el error real.
- Asegúrate de que no hay otra instancia bloqueando el puerto o el proceso.
- Aumenta el timeout si es necesario (raro en este servidor):

```toml
[mcp_servers.mssql]
command = "..."
startup_timeout_sec = 15
```

#### Problema 3: Quiero pasar variables adicionales solo para Grok

Aunque no es necesario para las conexiones dinámicas, puedes hacerlo:

```toml
[mcp_servers.mssql]
command = "C:\\MCPs\\MCP-EXE\\mssql2\\mcp-go-mssql-secure.exe"
env = { MSSQL_EXTRA_LOG = "true" }   # Variables adicionales
```

Estas variables se **combinan** con las que ya carga el servidor desde su `.env`.

### **📋 Resumen de Pasos Rápidos**

1. Añade (o verifica) la sección en `~/.grok/config.toml`
2. No toques el bloque `env` (déjalo vacío o ausente)
3. Guarda el archivo
4. Abre `/mcps` → verifica que `mssql` está verde
5. Prueba con `dynamic_available`
6. ¡Listo! Ya puedes usar `dynamic_connect alias="CRM"` etc.

### **📚 Recursos Relacionados**

- **[Guía recomendada]** Modo Dinámico para Una Aplicación con Varias Bases de Datos Relacionadas → [Leer guía](./modo-dinamico-una-aplicacion-varias-bases.md)
- [FAQ #5: Configuración en Claude Desktop](./FAQ-05-claude-config.md)
- [FAQ #7: Troubleshooting general](./FAQ-07-troubleshooting.md)
- Documentación oficial de Grok: `~/.grok/docs/user-guide/07-mcp-servers.md`
- Logs: `~/.grok/logs/mcp/mssql.stderr.log`
- Archivo de conexiones: `C:\MCPs\MCP-EXE\mssql2\.env`

---

**¿Funciona correctamente en Grok Build?**  
Si ves las conexiones dinámicas con `dynamic_available` y puedes hacer `dynamic_connect`, marca este FAQ como resuelto.
