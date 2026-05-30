# Guía: Modo Dinámico para Una Aplicación con Varias Bases de Datos Relacionadas

**Objetivo de esta guía**: Configurar el servidor MCP MSSQL en modo dinámico de forma **segura y práctica** cuando tienes **una sola aplicación lógica** que necesita acceder a varias bases de datos relacionadas.

---

## El Caso de Uso Real

Muchas aplicaciones empresariales no viven en una sola base de datos. Es muy común tener escenarios como:

- **CRM** + **Identity** + **Pedidos** + **Facturación**
- Base principal + Base de auditoría/logs + Base de reporting
- Módulos separados por razones técnicas o históricas

En estos casos, quieres:
- Cargar **solo un servidor MCP** en Claude Desktop / Grok / Claude Code.
- Poder cambiar fácilmente entre las bases de datos relacionadas de esa aplicación.
- Mantener un nivel de seguridad alto (no exponer todas las bases de la empresa).

El **Modo Dinámico** es la herramienta correcta para esto.

---

## Principio Fundamental de Seguridad

> **Define SOLO los alias que pertenecen a esa aplicación.**  
> Nunca expongas todas las bases de datos de la empresa solo porque "alguna vez puede que las necesite".

Este es el error más común y peligroso.

---

## Configuración Recomendada (Segura)

### 1. Estructura de tu `.env`

```env
# ============================================
# MODO DINÁMICO - Una sola aplicación
# ============================================
MSSQL_DYNAMIC_MODE=true
DEVELOPER_MODE=false

# --- Base de datos principal de la aplicación ---
MSSQL_DYNAMIC_APP_MAIN_SERVER=sql01.empresa.local
MSSQL_DYNAMIC_APP_MAIN_DATABASE=MiAplicacion_Produccion
MSSQL_DYNAMIC_APP_MAIN_USER=app_ai_read
MSSQL_DYNAMIC_APP_MAIN_PASSWORD=...
MSSQL_DYNAMIC_APP_MAIN_READ_ONLY=true

# --- Base de Identity / Usuarios (relacionada) ---
MSSQL_DYNAMIC_APP_IDENTITY_SERVER=sql01.empresa.local
MSSQL_DYNAMIC_APP_IDENTITY_DATABASE=IdentityDB
MSSQL_DYNAMIC_APP_IDENTITY_USER=app_ai_read
MSSQL_DYNAMIC_APP_IDENTITY_PASSWORD=...
MSSQL_DYNAMIC_APP_IDENTITY_READ_ONLY=true

# --- Base de Auditoría / Logs ---
MSSQL_DYNAMIC_APP_AUDIT_SERVER=sql01.empresa.local
MSSQL_DYNAMIC_APP_AUDIT_DATABASE=AuditLogs
MSSQL_DYNAMIC_APP_AUDIT_USER=app_ai_read
MSSQL_DYNAMIC_APP_AUDIT_PASSWORD=...
MSSQL_DYNAMIC_APP_AUDIT_READ_ONLY=true

# ============================================
# SOLO si realmente necesitas escribir
# ============================================

# Ejemplo: Base de trabajo / staging para la IA
MSSQL_DYNAMIC_APP_WORK_SERVER=devsql01.empresa.local
MSSQL_DYNAMIC_APP_WORK_DATABASE=AI_WorkArea
MSSQL_DYNAMIC_APP_WORK_USER=app_ai_writer
MSSQL_DYNAMIC_APP_WORK_PASSWORD=...

# IMPORTANTE: Aunque permitas escritura, limita estrictamente las tablas
MSSQL_DYNAMIC_APP_WORK_READ_ONLY=false
MSSQL_DYNAMIC_APP_WORK_WHITELIST_TABLES=ai_temp_tasks,ai_temp_results,ai_work_orders
```

### 2. Recomendaciones de Usuarios en SQL Server

Crea usuarios específicos con permisos mínimos:

| Alias              | Usuario SQL Recomendado     | Permisos recomendados                  |
|--------------------|-----------------------------|----------------------------------------|
| `APP_MAIN`         | `app_ai_read`               | Solo `SELECT` + vistas específicas     |
| `APP_IDENTITY`     | `app_ai_read`               | Solo `SELECT`                          |
| `APP_AUDIT`        | `app_ai_read`               | Solo `SELECT`                          |
| `APP_WORK`         | `app_ai_writer`             | Solo en las tablas de la whitelist     |

**Nunca uses `sa` ni un usuario con permisos amplios** para los aliases dinámicos.

---

## Cómo Usarlo en la Práctica (con la IA)

Una vez configurado, en tu conversación con Claude o Grok puedes hacer:

```
Carga el modo dinámico de la aplicación.

Primero conéctate a la base principal usando dynamic_connect con el alias "APP_MAIN".

Luego dime cuántos clientes activos hay en la tabla Clientes.

Ahora cambia a la base de Identity (alias APP_IDENTITY) y busca el usuario con email juan.perez@empresa.com.

Vuelve a la base principal y tráeme los últimos 10 pedidos de ese cliente.
```

La IA puede cambiar de base de datos durante la misma conversación usando `dynamic_connect`.

---

## Consultas entre Bases de Datos (Cross-Database Queries)

SQL Server permite hacer consultas entre bases de datos usando nombres de 3 o 4 partes:

```sql
-- Desde APP_MAIN, consultar datos de Identity
SELECT 
    c.Nombre,
    c.Email,
    u.LastLogin
FROM Clientes c
INNER JOIN IdentityDB.dbo.Usuarios u 
    ON c.UsuarioId = u.Id
WHERE c.Estado = 'Activo';
```

**Ventajas**:
- Puedes quedarte mayoritariamente en un alias (el principal).
- Reduces la cantidad de cambios de conexión.

**Requisitos**:
- El usuario de la conexión debe tener permisos en las otras bases de datos.
- Las bases de datos deben estar en el mismo servidor (o usar Linked Servers, que es más complejo).

---

## Estrategias de Seguridad Avanzadas (Recomendadas)

### Comportamiento por Defecto Seguro

El servidor aplica **"secure by default"** de forma estricta:

- Si no especificas `MSSQL_DYNAMIC_<ALIAS>_READ_ONLY`, el alias se carga automáticamente como **solo lectura**.
- Si un alias está en modo escritura pero no defines `WHITELIST_TABLES`, **no se permite ninguna modificación**.
- Cualquier operación de modificación en un alias con `READ_ONLY=false` requiere confirmación explícita mediante la herramienta `confirm_operation` antes de ejecutarse.

### Opción 1: Todo en solo lectura + una base de trabajo (Más segura)

Esta es la recomendación más equilibrada:

- Todas las bases "reales" → `READ_ONLY=true`
- Una base dedicada para trabajo de la IA → `READ_ONLY=false` + whitelist muy estricta + confirmación obligatoria vía `confirm_operation`

Ejemplo de tablas permitidas en la base de trabajo:
- `ai_prompts`
- `ai_results_temp`
- `ai_suggested_actions`
- `ai_audit_log`

### Opción 2: Usar un usuario con permisos muy limitados incluso en escritura

Aunque pongas `READ_ONLY=false`, el usuario de base de datos debe tener permisos solo sobre las tablas de la whitelist (no `db_owner`).

---

## Resumen de Buenas Prácticas

| Práctica                              | Recomendado          | Comentario |
|---------------------------------------|----------------------|----------|
| Número de aliases por aplicación      | 3 a 6 máximo         | Menos es más seguro |
| `READ_ONLY=true` por defecto          | Sí                   | Regla de oro |
| Usar `WHITELIST_TABLES` en escritura  | Siempre              | Limita el radio de explosión |
| Usuario de BD por tipo de alias       | Diferente            | `app_ai_read` vs `app_ai_writer` |
| Exponer todas las bases de la empresa | Nunca                | Define solo lo de esta aplicación |

---

## Ejemplo Completo Recomendado para una Aplicación

```env
MSSQL_DYNAMIC_MODE=true
DEVELOPER_MODE=false

# === Solo lectura (la mayoría) ===
MSSQL_DYNAMIC_APP_MAIN_READ_ONLY=true
MSSQL_DYNAMIC_APP_IDENTITY_READ_ONLY=true
MSSQL_DYNAMIC_APP_AUDIT_READ_ONLY=true
MSSQL_DYNAMIC_APP_REPORTING_READ_ONLY=true

# === Escritura controlada (solo trabajo de IA) ===
MSSQL_DYNAMIC_APP_AIWORK_READ_ONLY=false
MSSQL_DYNAMIC_APP_AIWORK_WHITELIST_TABLES=ai_temp,ai_results,ai_tasks,ai_logs
```

Con esta configuración cargas **un solo servidor MCP** y puedes trabajar de forma segura con todas las bases relacionadas de tu aplicación.

---

## ¿Necesitas Ayuda para tu Caso Concreto?

Si me dices:
- Cuántas bases de datos tiene tu aplicación
- Cuáles de ellas realmente necesitan escritura
- Qué tipo de operaciones quieres que haga la IA (insertar en tablas temporales, actualizar estados, generar reportes, etc.)

Puedo darte la configuración `.env` exacta y los permisos recomendados en SQL Server.

---

**Herramienta de Confirmación**

Cuando uses aliases con `READ_ONLY=false`, la IA deberá llamar primero a la herramienta `confirm_operation` con una descripción clara de la operación que quiere realizar. Solo después de recibir confirmación se permitirá la consulta de modificación.

**Relacionado**:
- [FAQ: Conexiones Dinámicas](./faq-conexiones-dinamicas.md)
- [Variables de Entorno](../configuracion/variables-entorno.md)
- [Modo Solo Lectura y Whitelist](../seguridad/modo-solo-lectura.md)
- [Whitelist de Tablas](../seguridad/whitelist-tablas.md)
```

Now I have created the guide. Next, I should also create or link to an English version, and update the existing dynamic FAQ to reference this new guide.

Let's also create the English version for completeness, since the site is bilingual.