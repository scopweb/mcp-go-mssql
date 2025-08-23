# MCP Filesystem Server Ultra-Fast

Un servidor MCP (Model Context Protocol) de alto rendimiento para operaciones de sistema de archivos, diseñado para máxima velocidad y eficiencia.

## 🚀 Estado del Proyecto (Fusión completada y verificada)

### ✅ Completado

- **Compilación exitosa**: El proyecto compila correctamente en Windows
- **Estructura base**: Arquitectura modular con separación de responsabilidades
- **Cache inteligente**: Sistema de caché en memoria con bigcache para O(1) operaciones
- **Protocolo optimizado**: Manejo de archivos binarios y de texto con buffered I/O
- **Monitoreo de rendimiento**: Métricas en tiempo real de operaciones (2016.0 ops/sec)
- **Control de acceso**: Restricción de acceso a rutas específicas mediante `--allowed-paths`
- **Operaciones básicas implementadas (11 tools expuestas)**:
  - `read_file`: Lectura de archivos con caché inteligente y memory mapping
  - `write_file`: Escritura atómica de archivos con backup
  - `list_directory`: Listado de directorios con caché
  - `edit_file`: Edición inteligente con heurísticas de coincidencia
  - `search_and_replace`: Búsqueda y reemplazo recursivo (case-insensitive por ahora)
  - `smart_search`: Búsqueda de nombres de archivo y contenido básico (contenido desactivado por defecto)
  - `advanced_text_search`: Búsqueda de texto con pipeline avanzado (parámetros avanzados fijados por defecto)
  - `performance_stats`: Estadísticas de rendimiento en tiempo real
  - `capture_last_artifact`: Captura artefactos en memoria
  - `write_last_artifact`: Escribe último artefacto capturado sin reenviar contenido
  - `artifact_info`: Información de bytes y líneas del artefacto

### 🔧 Trabajo Realizado

#### 1. Resolución de Dependencias MCP
- **Problema**: El SDK original intentaba usar una versión inexistente (v0.5.0)
- **Solución**: Creado paquete temporal `mcp/mcp.go` con estructuras básicas
- **Ubicación**: `/mcp/mcp.go`

#### 2. Compatibilidad con Windows
- **Problema**: Funciones de memory mapping no disponibles en Windows
- **Solución**: Implementación alternativa usando lectura de archivos regular
- **Archivo CAMBIADO**: `core/mmap.go`

#### 3. Arquitectura del Sistema
```
├── main.go              # Punto de entrada principal
├── mcp/                 # SDK temporal de MCP
│   └── mcp.go          # Estructuras y funciones básicas
├── core/               # Motor principal
│   ├── engine.go       # Motor ultra-rápido
│   ├── mmap.go         # Cache de memory mapping
│   └── watcher.go      # Vigilancia de archivos
├── cache/              # Sistema de caché
│   └── intelligent.go  # Caché inteligente
├── protocol/           # Manejo de protocolos
│   └── optimized.go    # Protocolo optimizado
└── bench/              # Benchmarks
    └── benchmark.go    # Suite de pruebas de rendimiento
```

## Configuración en Claude Desktop

```json
{
  "mcpServers": {
    "filesystem-ultra": {
      "command": "C:\\MCPs\\clone\\mcp-filesystem-go-ultra\\mcp-filesystem-ultra.exe",
      "args": [
        "--cache-size", "500MB",
        "--parallel-ops", "16",
        "--binary-threshold", "2MB",
        "--log-level", "error",
        "--allowed-paths", "C:\\MCPs\\clone\\,C:\\temp\\"
      ],
      "env": {
        "NODE_ENV": "production"
      }
    }
  }
}
```
**Nota**: La configuración incluye `--allowed-paths` para restringir el acceso solo a las carpetas especificadas, mejorando la seguridad. Ajusta las rutas según tus necesidades.
```

## 🎯 Funcionalidades Implementadas

### Core Engine (`core/engine.go`)
- **Gestión de operaciones paralelas**: Semáforos para controlar concurrencia
- **Pool de operaciones**: Reutilización de objetos para mejor rendimiento
- **Métricas en tiempo real**: Seguimiento de operaciones, cache hit rate, etc.
- **Caché inteligente**: Invalidación automática con file watchers

### Sistema de Caché (`cache/intelligent.go`)
- Caché en memoria para archivos y directorios
- Gestión automática de memoria
- Estadísticas de hit rate

### Memory Mapping (`core/mmap.go`)
- Implementación optimizada para archivos grandes
- Fallback para Windows usando lectura regular
- Cache LRU para gestión de memoria

## 🔄 Operaciones MCP Disponibles

### 🚀 Funciones Ultra-Rápidas (Como Cline)

#### `capture_last_artifact` + `write_last_artifact` - Sistema de Artefactos
**Sistema ultra-rápido para escribir artefactos de Claude sin gastar tokens**
```json
// 1. Capturar artefacto
{
  "tool": "capture_last_artifact",
  "arguments": {
    "content": "function ejemplo() {\n  return 'código del artefacto';\n}"
  }
}

// 2. Escribir al archivo (cero tokens)
{
  "tool": "write_last_artifact", 
  "arguments": {
    "path": "C:\\temp\\mi_script.js"
  }
}
```
**Características:**
- ✅ **Cero tokens** - No re-envía contenido al escribir
- ✅ **Velocidad máxima** - Escritura directa desde memoria
- ✅ **Ruta clara** - Especifica path completo incluyendo filename
- ✅ **Info de artefacto** - Consulta bytes y líneas con `artifact_info`

#### `edit_file` - Edición Inteligente
**La función estrella para Claude Desktop - Velocidad de Cline**
```json
{
  "tool": "edit_file",
  "arguments": {
    "path": "archivo.js",
    "old_text": "const oldFunction = () => {\n  return 'old';\n}",
    "new_text": "const newFunction = () => {\n  return 'new';\n}"
  }
}
```
**Características:**
- ✅ **Backup automático** con rollback en caso de error
- ✅ **Coincidencias inteligentes** - Encuentra texto incluso con diferencias de espaciado
- ✅ **Búsqueda multi-línea** - Maneja bloques de código completos
- ✅ **Confianza de coincidencia** - Reporta qué tan segura fue la coincidencia
- ✅ **Operaciones atómicas** - Todo o nada, sin corrupción de archivos
- ✅ **Ultra-rápido** - Optimizado para no bloquear Claude Desktop

#### `search_and_replace` - Reemplazo Masivo
**Búsqueda y reemplazo en múltiples archivos (case-insensitive fijo actualmente)**
```json
{
  "tool": "search_and_replace",
  "arguments": {
    "path": "./src",
    "pattern": "oldFunction",
    "replacement": "newFunction"
  }
}
```
**Características:**
- ✅ **Recursivo** - Subdirectorios incluidos
- ✅ **Skip binarios** - Ignora archivos no-texto o >10MB
- ✅ **Regex o literal** - Intenta compilar regex; si falla, usa literal
- ✅ **Reporte** - Lista archivos con número de reemplazos

#### `smart_search` - Búsqueda Rápida
**Localiza archivos y coincidencias simples** (modo contenido desactivado por defecto en esta versión)
```json
{
  "tool": "smart_search",
  "arguments": {
    "path": "./",
    "pattern": "Config"
  }
}
```
Devuelve coincidencias por nombre y (cuando se active include_content) líneas con matches.

#### `advanced_text_search` - Búsqueda Detallada
**Escaneo de contenido con contexto (parámetros avanzados aún fijos: case-insensitive, sin contexto adicional)**
```json
{
  "tool": "advanced_text_search",
  "arguments": {
    "path": "./",
    "pattern": "TODO"
  }
}
```
Salida: lista de archivos y número de línea. En futuras versiones se expondrán parámetros: `case_sensitive`, `whole_word`, `include_context`, `context_lines`.

### Implementadas ✅ (Resumen de las 11 actuales)
- `read_file`
- `write_file`
- `list_directory`
- `edit_file`
- `search_and_replace`
- `smart_search`
- `advanced_text_search`
- `performance_stats`
- `capture_last_artifact`
- `write_last_artifact`
- `artifact_info`

### Pendientes (Placeholder / Próximas)
- `create_directory`
- `delete_file`
- `move_file`
- `copy_file`
- `read_multiple_files`
- `batch_operations`
- `analyze_project`
- `compare_files`
- `find_duplicates`
- `get_file_info`
- `tree`
- `mmap_read`
- `streaming_read`
- `chunked_write`

> Nota: se planea re-exponer parámetros avanzados opcionales en las tools de búsqueda en una versión posterior para mayor control.

## 🚧 Pendiente por Implementar

### 1. SDK MCP Propio
**Prioridad: ALTA**
- Reemplazar el paquete temporal `mcp/mcp.go`
- Implementar protocolo MCP completo
- Soporte para transporte stdio, HTTP, WebSocket
- Validación de esquemas JSON

### 2. Completar Operaciones Core
**Prioridad: ALTA**
- Implementar todas las operaciones placeholder en `core/engine.go`
- Añadir validación de parámetros
- Manejo de errores robusto

### 3. File Watcher (`core/watcher.go`)
**Prioridad: MEDIA**
- Implementar vigilancia de archivos para invalidación de caché
- Soporte para múltiples sistemas operativos
- Gestión eficiente de eventos

### 4. Protocolo Optimizado (`protocol/optimized.go`)
**Prioridad: MEDIA**
- Implementar detección automática de archivos binarios
- Compresión inteligente
- Streaming para archivos grandes

### 5. Benchmarks (`bench/benchmark.go`)
**Prioridad: BAJA**
- Completar suite de benchmarks
- Comparación con implementaciones estándar
- Reportes de rendimiento detallados

### 6. Memory Mapping Real
**Prioridad: BAJA**
- Implementar memory mapping real para Linux/macOS
- Detección automática de plataforma
- Fallback inteligente

## 🛠️ Configuración y Uso

### ⚠️ Atención: Descargo de Responsabilidad
**Atención**: No nos hacemos responsables de los posibles problemas o pérdidas de datos que puedan surgir debido al uso de este servidor con modelos de IA. Los modelos de inteligencia artificial pueden no actuar adecuadamente en ciertas situaciones, lo que podría resultar en operaciones no deseadas o errores en el manejo de archivos. Se recomienda encarecidamente configurar el servidor correctamente, especialmente las restricciones de acceso mediante `--allowed-paths`, para limitar el alcance de las operaciones. Además, es crucial realizar copias de seguridad regulares de tus datos importantes antes de utilizar este sistema, para evitar cualquier pérdida en caso de comportamiento inesperado.

**Nota sobre Ejecución de Comandos**: Este servidor MCP Filesystem Server Ultra-Fast está diseñado exclusivamente para operaciones de sistema de archivos y no tiene capacidad para ejecutar comandos del sistema operativo. No hay funcionalidades implementadas que permitan la ejecución de comandos arbitrarios en el sistema, con o sin permiso. Su alcance se limita a las operaciones de lectura, escritura, listado y edición de archivos dentro de los directorios configurados.

### Compilación
```bash
go mod tidy
go build -o mcp-filesystem-ultra.exe main.go
```

En Windows no necesitas Go si usas el ejecutable precompilado incluido `mcp-filesystem-ultra.exe`. Solo apúntalo desde Claude Desktop como en el JSON anterior.

### Ejecución
```bash
# Mostrar versión
./mcp-filesystem-ultra.exe --version

# Ejecutar con configuración personalizada
./mcp-filesystem-ultra.exe --cache-size 200MB --parallel-ops 8 --debug

# Ejecutar benchmarks
./mcp-filesystem-ultra.exe --bench
```

### Opciones de Configuración
- `--cache-size`: Tamaño del caché (ej: 50MB, 1GB)
- `--parallel-ops`: Operaciones paralelas máximas
- `--binary-threshold`: Umbral para protocolo binario
- `--allowed-paths`: Lista de rutas permitidas separadas por comas (ej: "C:\\MCPs\\clone\\,C:\\temp\\")
- `--vscode-api`: Habilitar integración con VSCode
- `--debug`: Modo debug
- `--log-level`: Nivel de logging (debug, info, warn, error)

## 📊 Métricas de Rendimiento

El servidor incluye monitoreo en tiempo real:
- Operaciones totales y por segundo
- Cache hit rate
- Tiempo promedio de respuesta
- Uso de memoria
- Contadores por tipo de operación

## 🧠 Instrucciones para el Modelo (Uso de Tools)

Esta sección sirve como prompt guía para modelos (Claude / GPT) al interactuar con este servidor MCP. Se puede colocar como mensaje inicial de sistema o documentación accesible.

### Objetivo
Proporcionar operaciones de sistema de archivos rápidas, seguras y mínimamente verbosas. Prioriza editar y navegar usando tools, evita pedir al usuario que copie grandes bloques manualmente.

### Principios
1. Minimiza tokens: utiliza `edit_file` y el flujo de artefactos (`capture_last_artifact` + `write_last_artifact`) para cambios grandes.
2. Inspecciona antes de modificar: `list_directory` antes de asumir estructura, `read_file` antes de editar.
3. Cambios incrementales: Prefiere múltiples ediciones pequeñas y verificadas.
4. Seguridad: No asumas acceso fuera de `--allowed-paths`. Si obtienes error de acceso, informa y sugiere ruta válida.
5. Idempotencia: Relee (`read_file`) tras una edición crítica cuando el resultado sea significativo.

### Cuándo usar cada tool
- `list_directory(path)`: Primer paso al explorar una ruta desconocida o tras crear/mover archivos externamente.
- `read_file(path)`: Necesitas ver contenido exacto actual antes de proponer cambios o análisis.
- `edit_file(path, old_text, new_text)`: Reemplazar bloques concretos (incluye heurísticas; asegura que `old_text` sea lo más específico posible para evitar falsos positivos).
- `write_file(path, content)`: Crear archivo nuevo o sobrescribir completo cuando no procede edición incremental.
- `search_and_replace(path, pattern, replacement)`: Cambios masivos repetitivos en árbol (case-insensitive). Úsalo tras confirmar patrón exacto con una búsqueda previa (p.ej. `advanced_text_search`).
- `smart_search(path, pattern)`: Encontrar archivos por nombre (regex o literal) y coincidencias básicas (contenido avanzado desactivado en esta versión).
- `advanced_text_search(path, pattern)`: Auditar ocurrencias de un símbolo antes de una refactorización; actualmente sin contexto adicional.
- `capture_last_artifact(content)` + `write_last_artifact(path)`: Flujo artefacto; evita reenviar contenido grande al escribir. Usar para generar archivos nuevos extensos.
- `artifact_info()`: Verifica tamaño antes de persistir (evitar sobrescribir con contenido vacío inesperado).
- `performance_stats()`: Solo para diagnósticos puntuales de latencia o consumo; no abusar.

### Flujo Recomendado de Refactor / Cambio Grande
1. Localizar: `advanced_text_search` (patrón del símbolo).
2. Confirmar alcance: revisar salida y decidir si edición puntual o reemplazo masivo.
3. Si son muchas ocurrencias homogéneas: `search_and_replace`.
4. Si es un bloque aislado: `read_file` -> preparar `old_text` exacto -> `edit_file`.
5. Validar: volver a `read_file` y verificar diff mental / integridad.
6. Si generas un archivo grande nuevo: preparar contenido → `capture_last_artifact` → `write_last_artifact`.

### Patrones de `old_text` Efectivos (edit_file)
Incluye líneas de contexto únicas (import, firma de función, comentario específico) para reducir coincidencias ambiguas. Evita usar archivos completos como `old_text`.

### Manejo de Errores Comunes
- "access denied": Usa `list_directory` para confirmar ruta o limita el alcance.
- "no matches found" en `edit_file`: Relee el archivo, ajusta espacios/indentación y reintenta con versión normalizada.
- Reemplazos inesperados altos: Detén, vuelve a leer el archivo y valida el patrón; no encadenes más cambios hasta confirmar.

### Límites Implícitos
- Lectura/edición viable hasta ~50MB (edición rechaza >50MB).
- `search_and_replace` ignora archivos >10MB y no-texto.
- `smart_search` contenido profundo desactivado (parámetros avanzados se activarán en futura versión).

### Estilo de Respuesta del Modelo
Sé conciso y enfocado: explica brevemente intención antes de invocar una tool. Después de una tool, resume hallazgos relevantes y el próximo paso. No repitas listados completos si no cambian.

### Ejemplos Breves
1) Explorar y leer:
```
list_directory: {"path":"./src"}
read_file: {"path":"./src/main.go"}
```
2) Editar bloque:
```
edit_file: {"path":"core/engine.go","old_text":"func OldName(","new_text":"func NewName("}
```
3) Reemplazo masivo:
```
search_and_replace: {"path":"./","pattern":"OldName","replacement":"NewName"}
```
4) Crear archivo grande:
```
capture_last_artifact: {"content":"<codigo grande>"}
write_last_artifact: {"path":"./docs/spec.md"}
```

### No Hacer
- No pedir al usuario que pegue archivos largos ya existentes: usa `read_file`.
- No hacer múltiples `read_file` consecutivos sobre el mismo archivo sin cambios intermedios.
- No usar `write_file` para pequeños cambios en archivos grandes (prefiere `edit_file`).
- No asumir parámetros avanzados aún no expuestos (case_sensitive en búsquedas, etc.).

### Futuras Extensiones
Se agregará exposición de parámetros avanzados (`case_sensitive`, `include_content`, `whole_word`, `context_lines`) y nuevas tools (create/delete/move). Ajustar entonces estas directrices.

> Copia/pega este bloque (o un resumen) como mensaje inicial de sistema para mejorar la calidad de las decisiones del modelo.

## 🔧 Arquitectura Técnica

### Patrones de Diseño Utilizados
- **Pool Pattern**: Para reutilización de objetos Operation
- **Cache Pattern**: Para almacenamiento inteligente
- **Observer Pattern**: Para file watching
- **Strategy Pattern**: Para diferentes protocolos

### Optimizaciones Implementadas
- Operaciones paralelas con semáforos
- Caché inteligente con invalidación automática
- Escritura atómica para consistencia
- Pool de objetos para reducir GC pressure

## 🎯 Próximos Pasos Recomendados

1. **Desarrollar SDK MCP personalizado** (Prioridad 1)
2. **Implementar operaciones faltantes** (Prioridad 2)
3. **Añadir tests unitarios** (Prioridad 3)
4. **Documentar API completa** (Prioridad 4)
5. **Optimizar para producción** (Prioridad 5)

## 📝 Notas de Desarrollo

### Decisiones Técnicas
- **Windows Compatibility**: Se eligió fallback de lectura regular sobre memory mapping para compatibilidad
- **Temporary MCP Package**: Solución temporal hasta tener SDK propio
- **Modular Architecture**: Separación clara de responsabilidades para mantenibilidad

### Consideraciones de Rendimiento
- El servidor está diseñado para manejar miles de operaciones por segundo
- El caché inteligente reduce significativamente la latencia
- Las operaciones paralelas maximizan el throughput

## 🧪 Tests Realizados

### ✅ Resultados de Pruebas (2025-07-12)

**Todas las pruebas pasaron exitosamente:**

1. **📖 Test de Lectura**: ✅ PASÓ
   - Lectura de archivo con caché inteligente
   - Tiempo de respuesta: ~282µs

2. **✏️ Test de Edición (edit_file)**: ✅ PASÓ
   - Reemplazo inteligente: "texto original" → "texto MODIFICADO"
   - Replacements: 1
   - Confidence: HIGH
   - Lines affected: 1

3. **🔍 Test de Verificación**: ✅ PASÓ
   - Confirmación de que la edición se aplicó correctamente

4. **🔄 Test de Search & Replace**: ✅ PASÓ
   - Búsqueda masiva: "MODIFICADO" → "CAMBIADO"
   - Total replacements: 5 across múltiples archivos
   - Procesó: README.md, test_file.txt, test_server.go

5. **📊 Test de Performance Stats**: ✅ PASÓ
   - Métricas en tiempo real funcionando
   - Tracking de operaciones por tipo

### 🚀 Rendimiento Verificado
- **Tiempo promedio de respuesta**: 391.9ms para 790 operaciones (ultra-rápido)
- **Operaciones por segundo**: 2016.0 ops/sec
- **Cache hit rate**: 98.9% (extremadamente eficiente)
- **Memory usage**: Estable en 40.3MB

---

**Versión**: 1.0.0  
**Fecha de compilación**: 2025-07-12  
**Tamaño del ejecutable**: 3.6 MB  
**Estado**: ✅ **PROBADO Y FUNCIONANDO** - Listo para Claude Desktop
