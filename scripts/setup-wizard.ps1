<#
.SYNOPSIS
    MCP MSSQL Server - Asistente de Configuración
.DESCRIPTION
    Este script te guía paso a paso para configurar el servidor MCP MSSQL.
    Explica cada opción y te ayuda a generar la configuración correcta.
.NOTES
    Ejecutar en PowerShell como Administrador si es necesario.
#>

# ─────────────────────────────────────────────────────────────────────────────
# COLORES PARA LA TERMINAL
# ─────────────────────────────────────────────────────────────────────────────
function Write-Title { param($text) Write-Host "`n=== $text ===" -ForegroundColor Cyan }
function Write-Subtitle { param($text) Write-Host "`n$text" -ForegroundColor Yellow }
function Write-Info { param($text) Write-Host "  ℹ  $text" -ForegroundColor Gray }
function Write-Warning { param($text) Write-Host "  ⚠  $text" -ForegroundColor Yellow }
function Write-Success { param($text) Write-Host "  ✅ $text" -ForegroundColor Green }
function Write-Error { param($text) Write-Host "  ❌ $text" -ForegroundColor Red }
function Write-Explanation { param($text) Write-Host "     $text" -ForegroundColor DarkGray }

# ─────────────────────────────────────────────────────────────────────────────
# FUNCIONES DE UTILIDAD
# ─────────────────────────────────────────────────────────────────────────────
function Show-Explanation {
    param($title, $text)
    Write-Host "`n  ┌─ $title ─" -ForegroundColor DarkCyan
    $lines = $text -split "`n"
    foreach ($line in $lines) {
        Write-Host "  │ $line" -ForegroundColor DarkGray
    }
    Write-Host "  └─────────────────────────────────────────" -ForegroundColor DarkCyan
}

function Ask-YesNo {
    param($question, $default = "N")
    $prompt = "  [$default] "
    Write-Host "$question (s/n): " -NoNewline -ForegroundColor White
    $answer = Read-Host
    if ([string]::IsNullOrWhiteSpace($answer)) { $answer = $default }
    return $answer.ToLower() -eq "s"
}

function Ask-Choice {
    param($question, $options, $default = 0)
    Write-Host "`n$question" -ForegroundColor White
    for ($i = 0; $i -lt $options.Count; $i++) {
        $marker = if ($i -eq $default) { "→" } else { " " }
        Write-Host "  $marker [$($i+1)] $($options[$i].text)" -ForegroundColor White
    }
    Write-Host "  ─────────────────────────────────" -ForegroundColor DarkGray

    do {
        Write-Host "  Opción (1-$($options.Count)) [$($default+1)]: " -NoNewline -ForegroundColor White
        $choice = Read-Host
        if ([string]::IsNullOrWhiteSpace($choice)) { $choice = $default + 1 }
    } while ($choice -notmatch "^[1-$($options.Count)]$")

    return [int]$choice - 1
}

# ─────────────────────────────────────────────────────────────────────────────
# PREGUNTAS DEL ASISTENTE
# ─────────────────────────────────────────────────────────────────────────────

Write-Host @"

╔══════════════════════════════════════════════════════════════════════════════╗
║          MCP MSSQL SERVER - ASISTENTE DE CONFIGURACIÓN                     ║
║                                                                              ║
║  Este asistente te ayudará a configurar el servidor MCP para conectarte     ║
║  a tu base de datos SQL Server de forma segura.                            ║
║                                                                              ║
║  Te explicará cada opción para que entiendas qué estás configurando.        ║
╚══════════════════════════════════════════════════════════════════════════════╝
"@ -ForegroundColor Cyan

Write-Title "PASO 1: CONEXIÓN A LA BASE DE DATOS"

Write-Subtitle "1.1 ¿Qué tipo de autenticación usas?"
Show-Explanation "Autenticación SQL Server" @"
La forma más común. Necesitas un usuario y contraseña de SQL Server.
Ejemplo: usuario 'sa' con contraseña 'miPassword123'
"@

Write-Subtitle "1.2 Datos de conexión"
Write-Host "  Servidor (hostname o IP): " -NoNewline -ForegroundColor White
$server = Read-Host

Write-Host "  Puerto (default 1433): " -NoNewline -ForegroundColor White
$port = Read-Host
if ([string]::IsNullOrWhiteSpace($port)) { $port = "1433" }

Write-Host "  Nombre de la base de datos: " -NoNewline -ForegroundColor White
$database = Read-Host

Write-Host "  Usuario SQL Server: " -NoNewline -ForegroundColor White
$username = Read-Host

Write-Host "  Contraseña SQL Server: " -NoNewline -ForegroundColor White
$password = Read-Host

Write-Host "  ¿Usar conexión encriptada? (encrypt=disable) [N]: " -NoNewline -ForegroundColor White
$encrypt = Read-Host
if ([string]::IsNullOrWhiteSpace($encrypt)) { $encrypt = "disable" }

Write-Title "PASO 2: ¿PARA QUÉ VAS A USAR ESTO?"

$usageOptions = @(
    @{ text = "Desarrollo / Prueba"; desc = "Experimentar, probar queries, desarrollo sin riesgo" },
    @{ text = "Producción - Solo lectura"; desc = "Consultar datos sin modificar nada" },
    @{ text = "Producción - Lectura + Escritura limitada"; desc = "AI puede modificar solo ciertas tablas" },
    @{ text = "Producción - Acceso total"; desc = "AI puede hacer TODO, máximo cuidado" }
)
$usageChoice = Ask-Choice "¿Cómo usarás la base de datos?" $usageOptions

Write-Title "PASO 3: EXPLICACIÓN DE MODOS DE SEGURIDAD"

Write-Subtitle "3.1 DEVELOPER_MODE (Modo Desarrollador)"
Show-Explanation "Qué hace" @"
- Muestra errores detallados (en producción solo errores genéricos)
- Permite certificados TLS de servidor self-signed o no confiable
- Recomendado para desarrollo local o cuando tienes problemas de conexión
"@

$devMode = Ask-YesNo "¿Activar DEVELOPER_MODE?" "S"

Write-Subtitle "3.2 MSSQL_READ_ONLY (Solo Lectura)"
Show-Explanation "Qué hace y POR QUÉ importan" @"
Cuando está ACTIVADO (true):
  - SELECT: ✅ Funciona en TODAS las tablas
  - INSERT/UPDATE/DELETE: ❌ BLOQUEADO en todas las tablas

Cuando está DESACTIVADO (false):
  - SELECT: ✅ Funciona en TODAS las tablas
  - INSERT/UPDATE/DELETE: ✅ Funcionan en TODAS las tablas

¿POR QUÉ ACTIVARLO?
  → Protección contra errores del AI que borren o modifiquen datos
  → Recomendado en bases de datos de producción reales

¿POR QUÉ DESACTIVARLO?
  → El AI necesita insertar/actualizar datos intencionalmente
  → Ejemplo: AI está entrenando, limpiando datos, generando reportes
"@

$readOnly = !$Ask-YesNo "El AI necesitará INSERT/UPDATE/DELETE?" "N"

Write-Subtitle "3.3 MSSQL_WHITELIST_TABLES (Tablas Permitidas)"

if ($readOnly -eq $false) {
    Show-Explanation "Qué hace y POR QUÉ importan" @"
Imagina que tu AI tiene acceso TOTAL a la base de datos.
Un error podría borrar una tabla importante.

La WHITELIST es como una "lista blanca" de tablas seguras.
Solo在这些 tablas, el AI puede hacer INSERT/UPDATE/DELETE.

EJEMPLO:
  MSSQL_WHITELIST_TABLES=temp_ai,resultados_ia,importaciones

  → Tablas en la lista: INSERT/UPDATE/DELETE ✅
  → Otras tablas: INSERT/UPDATE/DELETE ❌ (pero SELECT sigue funcionando)

¿POR QUÉ USARLO?
  → 即使AI犯错误，也只会影响临时表或测试表
  → Protege tus tablas importantes (clientes, pedidos, etc.)
  → Recomendado: crear tablas específicas para que el AI trabaje
"@

    $useWhitelist = Ask-YesNo "¿Limitar las tablas modificables?" "S"

    if ($useWhitelist) {
        Write-Host "`n  Escribe los nombres de tabla separados por coma:" -ForegroundColor White
        Write-Host "  Ejemplo: temp_ai,v_temp_ia,resultados" -ForegroundColor DarkGray
        Write-Host "  Tablas: " -NoNewline -ForegroundColor White
        $whitelistTables = Read-Host
    } else {
        $whitelistTables = ""
    }
} else {
    $useWhitelist = $false
    $whitelistTables = ""
    Write-Info "READ_ONLY=true activo: whitelist no necesaria (todo está bloqueado de todas formas)"
}

Write-Subtitle "3.4 MSSQL_AUTOPILOT (Modo Autopiloto)"
Show-Explanation "Qué hace y POR QUÉ es útil" @"
El AUTOPILOT es un modo especial que simplifica el uso:

  ✅ Skipa la CONFIRMACIÓN DESTRUCTIVA
     → No necesitas confirmar operaciones DROP/ALTER/CREATE
     → El AI puede hacer cambios sin pausar para preguntar

  ✅ Skipa la VALIDACIÓN DE ESQUEMA
     → Puede consultar tablas que aún no existen
     → Útil cuando el AI está creando objetos nuevos

  ⚠️ NO skipea READ_ONLY ni WHITELIST
     → Si READ_ONLY=true, las modificaciones siguen bloqueadas
     → Si una tabla no está en whitelist, no se puede modificar

¿CUÁNDO USARLO?
  → Cuando quieres que el AI trabaje sin interrupciones
  → Desarrollo rápido, prototyping, experimentos
  → Cuando ya sabes lo que haces y no necesitas confirmación

¿CUÁNDO NO USARLO?
  → Producción con datos críticos
  → Cuando quieres control total sobre cambios peligrosos
"@

$autopilot = Ask-YesNo "¿Activar AUTOPILOT?" "S"

Write-Subtitle "3.5 MSSQL_CONFIRM_DESTRUCTIVE (Confirmar Operaciones Peligrosas)"
Show-Explanation "Qué son operaciones destructivas" @"
Operaciones destructivas son comandos que pueden PERDER datos:

  ❌ DROP TABLE   → Borra una tabla completa con todos sus datos
  ❌ ALTER TABLE   → Cambia la estructura de una tabla
  ❌ TRUNCATE      → Borra todos los datos de una tabla
  ❌ DROP VIEW     → Borra una vista

Cuando CONFIRM_DESTRUCTIVE=true:
  → DROP TABLE mi_tabla → El servidor PREGUNTA: "¿Estás seguro?"
  → Necesitas llamar a confirm_operation con un token
  → El token expira en 5 minutos

Cuando CONFIRM_DESTRUCTIVE=false:
  → DROP TABLE mi_tabla → Se ejecuta inmediatamente
  → Para CI/CD automatización
  → ¡PELIGRO! Sin confirmación, un error puede ser catastrófico
"@

if (-not $autopilot) {
    $confirmDestructive = Ask-YesNo "¿Confirmar operaciones destructivas?" "S"
} else {
    $confirmDestructive = $false
    Write-Info "AUTOPILOT=true: confirmación destructiva deshabilitada automáticamente"
}

Write-Title "PASO 4: RESUMEN DE TU CONFIGURACIÓN"

Write-Host @"

┌──────────────────────────────────────────────────────────────────────────────┐
│                           TU CONFIGURACIÓN                                   │
├──────────────────────────────────────────────────────────────────────────────┤
"@ -ForegroundColor White

Write-Host "│  Servidor:        $server" -ForegroundColor White
Write-Host "│  Puerto:          $port" -ForegroundColor White
Write-Host "│  Base de datos:  $database" -ForegroundColor White
Write-Host "│  Usuario:         $username" -ForegroundColor White
Write-Host "│  Encrypt:         $encrypt" -ForegroundColor White
Write-Host "│" -ForegroundColor White
Write-Host "│  DEVELOPER_MODE:         $($devMode ? 'true' : 'false')" -ForegroundColor $(if ($devMode) { 'Green' } else { 'White' })
Write-Host "│  MSSQL_READ_ONLY:        $($readOnly ? 'true' : 'false')" -ForegroundColor $(if ($readOnly) { 'Green' } else { 'Yellow' })
Write-Host "│  MSSQL_WHITELIST_TABLES: $($useWhitelist ? $whitelistTables : '(todas)')" -ForegroundColor White
Write-Host "│  MSSQL_AUTOPILOT:        $($autopilot ? 'true' : 'false')" -ForegroundColor $(if ($autopilot) { 'Green' } else { 'White' })
Write-Host "│  MSSQL_CONFIRM_DESTRUCTIVE: $($confirmDestructive ? 'true' : 'false')" -ForegroundColor $(if ($confirmDestructive) { 'Green' } else { 'Yellow' })

Write-Host @"
└──────────────────────────────────────────────────────────────────────────────┘
"@ -ForegroundColor White

Write-Title "PASO 5: GENERAR CONFIGURACIÓN"

$encryptParam = if ($encrypt -eq "disable") { "encrypt=disable&trustservercertificate=true" } else { "encrypt=true" }
$connectionString = "sqlserver://${username}:${password}@${server}:${port}?database=${database}&${encryptParam}"

Write-Subtitle "Para Claude Desktop (claude_desktop_config.json)"
$configJson = @"
    "mssql-${database}": {
      "command": "C:\\MCPs\\clone\\mcp-go-mssql\\build\\mcp-go-mssql.exe",
      "args": [],
      "env": {
        "MSSQL_CONNECTION_STRING": "$connectionString",
        "DEVELOPER_MODE": "$($devMode.ToString().ToLower())"$(if ($readOnly) { ',' } else { '' })
$(
if ($readOnly) {
@"

        "MSSQL_READ_ONLY": "true"
"@
} elseif ($useWhitelist -and $whitelistTables) {
@",

        "MSSQL_WHITELIST_TABLES": "$whitelistTables"
"@
})%(if ($autopilot) { @",

        "MSSQL_AUTOPILOT": "true"
"@ })%(if (-not $confirmDestructive -and -not $autopilot) { @",

        "MSSQL_CONFIRM_DESTRUCTIVE": "false"
"@ })
      }
    }
"@

Write-Host $configJson -ForegroundColor Green

Write-Subtitle "Para archivo .env"
$envContent = @"
MSSQL_CONNECTION_STRING=$connectionString
DEVELOPER_MODE=$($devMode.ToString().ToLower())
MSSQL_READ_ONLY=$($readOnly.ToString().ToLower())
MSSQL_WHITELIST_TABLES=$whitelistTables
MSSQL_AUTOPILOT=$($autopilot.ToString().ToLower())
MSSQL_CONFIRM_DESTRUCTIVE=$($confirmDestructive.ToString().ToLower())
"@

Write-Host $envContent -ForegroundColor Cyan

Write-Subtitle "¿Qué significa todo esto?"
Show-Explanation "Resumen de seguridad" @"
LECTURA:
  → SELECT funciona siempre en todas las tablas

ESCRITURA (INSERT/UPDATE/DELETE):
$(
if ($readOnly) {
@"
  → ❌ BLOQUEADO en todas las tablas (READ_ONLY=true)
"@ } elseif ($useWhitelist -and $whitelistTables) {
@"
  → ✅ Solo en tablas whitelist: $whitelistTables
  → ❌ Bloqueado en otras tablas
"@ } else {
@"
  → ✅ PERMITIDO en todas las tablas
"@ }

OPERACIONES DESTRUCTIVAS (DROP/ALTER):
$(
if ($autopilot) {
@"
  → ✅ PERMITIDO sin confirmación (AUTOPILOT=true)
"@ } elseif ($confirmDestructive) {
@"
  → ⚠️ Requiere confirmación manual
"@ } else {
@"
  → ⚠️ PERMITIDO sin confirmación (para CI/CD)
"@ }
)
"@

Write-Title "PRÓXIMOS PASOS"
Write-Host @"
1.  Copia la configuración de arriba
2.  Pégala en tu Claude Desktop config
3.  Reinicia Claude Desktop
4.  El servidor MCP se conectará automáticamente
"@ -ForegroundColor White

$save = Ask-YesNo "¿Guardar configuración en archivo?" "N"
if ($save) {
    $configPath = Join-Path $PSScriptRoot "..\generated-config.json"
    $envPath = Join-Path $PSScriptRoot "..\generated-config.env"
    $configJson | Out-File -FilePath $configPath -Encoding UTF8
    $envContent | Out-File -FilePath $envPath -Encoding UTF8
    Write-Success "Configuración guardada en:"
    Write-Host "  JSON: $configPath" -ForegroundColor Green
    Write-Host "  ENV:  $envPath" -ForegroundColor Green
}

Write-Host "`n¡Listo! Si tienes dudas, consulta docs/config-visual.md`n" -ForegroundColor Cyan