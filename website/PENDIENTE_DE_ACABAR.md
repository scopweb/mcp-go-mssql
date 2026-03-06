# PENDIENTE DE ACABAR - Manual Astro MCP-Go-MSSQL

## Estado actual del proyecto

**Fecha última actualización:** 2026-03-06

---

## 📊 Resumen de progreso

| Idioma   | Archivos completados | Progreso |
|----------|---------------------|----------|
| Español  | 37/37               | ✅ 100%  |
| Inglés   | 37/37               | ✅ 100%  |

---

## ✅ Archivos completados (Español)

### Inicio (4)
- ✅ `inicio/bienvenida.md`
- ✅ `inicio/instalacion.md`
- ✅ `inicio/configuracion.md`
- ✅ `inicio/inicio-rapido.md`

### Herramientas MCP (9 + 2 nuevas v2)
- ✅ `herramientas-mcp/resumen.md`
- ✅ `herramientas-mcp/query-database.md`
- ✅ `herramientas-mcp/explore.md` ← v2 (reemplaza list_tables, list_databases, list_stored_procedures, search_objects)
- ✅ `herramientas-mcp/inspect.md` ← v2 (reemplaza describe_table, get_indexes, get_foreign_keys)
- ✅ `herramientas-mcp/get-database-info.md`
- ✅ `herramientas-mcp/execute-procedure.md`
- ✅ `herramientas-mcp/list-tables.md` (legacy v1)
- ✅ `herramientas-mcp/describe-table.md` (legacy v1)
- ✅ `herramientas-mcp/list-databases.md` (legacy v1)
- ✅ `herramientas-mcp/get-indexes.md` (legacy v1)
- ✅ `herramientas-mcp/get-foreign-keys.md` (legacy v1)
- ✅ `herramientas-mcp/list-stored-procedures.md` (legacy v1)

### Seguridad (7)
- ✅ `seguridad/resumen.md`
- ✅ `seguridad/tls-cifrado.md`
- ✅ `seguridad/modo-solo-lectura.md`
- ✅ `seguridad/whitelist-tablas.md`
- ✅ `seguridad/sql-injection.md`
- ✅ `seguridad/analisis-seguridad.md`
- ✅ `seguridad/auditoria.md`

### CLI (2)
- ✅ `cli/resumen.md`
- ✅ `cli/comandos.md`

### Configuración (5)
- ✅ `configuracion/variables-entorno.md`
- ✅ `configuracion/claude-desktop.md`
- ✅ `configuracion/autenticacion.md`
- ✅ `configuracion/autenticacion-windows.md`
- ✅ `configuracion/connection-strings.md`

### Despliegue (3)
- ✅ `despliegue/produccion.md`
- ✅ `despliegue/desarrollo.md`
- ✅ `despliegue/solucion-problemas.md`

### Guías (5)
- ✅ `guias/uso-con-ia.md`
- ✅ `guias/rendimiento.md`
- ✅ `guias/testing.md`
- ✅ `guias/actualizacion-go.md`
- ✅ `guias/integracion-mcp.md`

### Problemas resueltos (2)
- ✅ `problemas-resueltos/token-overflow.md`
- ✅ `problemas-resueltos/tool-search-sesion.md`

### Otros (1)
- ✅ `changelog.md`

---

## ✅ Archivos completados (Inglés - `en/`)

Todos los archivos de la sección española tienen su equivalente en `en/` completado.

---

## 🔄 TAREAS PENDIENTES

### Contenido a revisar/actualizar
- [ ] Verificar que los changelogs del website reflejen las actualizaciones de dependencias
- [ ] Revisar si hay páginas que referencien herramientas v1 sin indicar el banner de deprecación
- [ ] Confirmar que `astro.config.mjs` incluye las rutas de `problemas-resueltos/` en el sidebar

### Mejoras opcionales
- [ ] Añadir página `problemas-resueltos/` al sidebar en ambos idiomas (si no está)
- [ ] Revisar consistencia de enlaces internos entre páginas ES ↔ EN

---

## 🚀 Comandos útiles

```bash
# Iniciar servidor de desarrollo
npm run dev

# Construir sitio para producción
npm run build

# Vista previa de la build
npm run preview
```

---

**Estado:** Documentación completa en ambos idiomas. Pendiente revisar sidebar y coherencia de enlaces.
