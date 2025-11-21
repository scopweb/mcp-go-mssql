# Security Check Script for MCP Go MSSQL
# Verifica que todas las protecciones de seguridad esten activas

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  MCP Go MSSQL - Security Validation" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

$errors = 0
$warnings = 0
$passed = 0

function Test-Security {
    param(
        [string]$TestName,
        [scriptblock]$TestBlock,
        [string]$Severity = "ERROR"
    )
    
    Write-Host "Testing: $TestName..." -NoNewline
    
    try {
        $result = & $TestBlock
        if ($result) {
            Write-Host " PASS" -ForegroundColor Green
            $script:passed++
            return $true
        } else {
            if ($Severity -eq "WARNING") {
                Write-Host " WARNING" -ForegroundColor Yellow
                $script:warnings++
            } else {
                Write-Host " FAIL" -ForegroundColor Red
                $script:errors++
            }
            return $false
        }
    } catch {
        Write-Host " ERROR: $_" -ForegroundColor Red
        $script:errors++
        return $false
    }
}

Write-Host "Running Security Checks..." -ForegroundColor Yellow
Write-Host ""

# 1. Verificar que main.go usa Prepared Statements
Test-Security "SQL Injection Protection (Prepared Statements)" {
    $content = Get-Content "main.go" -Raw
    $hasPrepare = $content -match "PrepareContext"
    $hasQueryContext = $content -match "QueryContext"
    return ($hasPrepare -and $hasQueryContext)
}

# 2. Verificar TLS encryption en producción
Test-Security "TLS Encryption Configuration" {
    $content = Get-Content "main.go" -Raw
    $hasEncrypt = $content -match 'encrypt\s*:=\s*"true"'
    return $hasEncrypt
}

# 3. Verificar sanitización de logs
Test-Security "Log Sanitization (Password Masking)" {
    $content = Get-Content "main.go" -Raw
    $hasSanitize = $content -match "sanitizeForLogging"
    $hasPattern = $content -match "sensitivePatterns"
    return ($hasSanitize -and $hasPattern)
}

# 4. Verificar validación de comandos peligrosos
Test-Security "Command Injection Protection" {
    $content = Get-Content "main.go" -Raw
    $hasValidation = $content -match "validateReadOnlyQuery"
    $hasDangerousCheck = $content -match "EXEC |EXECUTE |XP_"
    return ($hasValidation -and $hasDangerousCheck)
}

# 5. Verificar whitelist de tablas
Test-Security "Granular Table Permissions (Whitelist)" {
    $content = Get-Content "main.go" -Raw
    $hasWhitelist = $content -match "validateTablePermissions"
    $hasWhitelistCheck = $content -match "MSSQL_WHITELIST_TABLES"
    return ($hasWhitelist -and $hasWhitelistCheck)
}

# 6. Verificar context timeouts
Test-Security "Context Timeout Protection" {
    $content = Get-Content "main.go" -Raw
    $hasTimeout = $content -match "WithTimeout"
    return $hasTimeout
}

# 7. Verificar input validation
Test-Security "Input Size Validation" {
    $content = Get-Content "main.go" -Raw
    $hasValidation = $content -match "validateBasicInput"
    $hasMaxSize = $content -match "maxSize"
    return ($hasValidation -and $hasMaxSize)
}

# 8. Verificar que no hay credenciales hardcoded
Test-Security "No Hardcoded Credentials" {
    $content = Get-Content "main.go" -Raw
    $usesEnv = $content -match 'os\.Getenv\("MSSQL_PASSWORD"'
    return $usesEnv
}

# 9. Verificar .gitignore para archivos sensibles
Test-Security ".gitignore Configuration" {
    if (Test-Path ".gitignore") {
        $content = Get-Content ".gitignore" -Raw
        $hasEnv = $content -match "\.env"
        $hasConfig = $content -match "config\.json"
        return ($hasEnv -and $hasConfig)
    }
    return $false
}

# 10. Verificar que existen tests de seguridad
Test-Security "Security Tests Existence" {
    $hasTests = (Test-Path "test\security\cves_test.go") -and (Test-Path "test\security\security_tests.go")
    return $hasTests
}

# 11. Verificar dependencias actualizadas
Write-Host "Testing: Dependency Versions..." -NoNewline
$goModContent = Get-Content "go.mod" -Raw
if ($goModContent -match "golang.org/x/crypto v0\.(\d+)\.0") {
    $cryptoVersion = [int]$matches[1]
    if ($cryptoVersion -ge 45) {
        Write-Host " PASS (v0.$cryptoVersion.0)" -ForegroundColor Green
        $passed++
    } else {
        Write-Host " WARNING (v0.$cryptoVersion.0 - consider updating)" -ForegroundColor Yellow
        $warnings++
    }
} else {
    Write-Host " FAIL (version not found)" -ForegroundColor Red
    $errors++
}

# 12. Verificar ejemplo de configuración segura
Test-Security "Secure Configuration Examples" {
    $readme = Get-Content "README.md" -Raw
    $hasReadOnly = $readme -match "MSSQL_READ_ONLY"
    $hasWhitelist = $readme -match "MSSQL_WHITELIST_TABLES"
    return ($hasReadOnly -and $hasWhitelist)
}

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Security Check Results" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Passed:   $passed" -ForegroundColor Green
Write-Host "Warnings: $warnings" -ForegroundColor Yellow
Write-Host "Errors:   $errors" -ForegroundColor Red
Write-Host ""

if ($errors -eq 0 -and $warnings -eq 0) {
    Write-Host "All security checks passed! System is secure." -ForegroundColor Green
    exit 0
} elseif ($errors -eq 0) {
    Write-Host "No critical issues found. Review warnings above." -ForegroundColor Yellow
    exit 0
} else {
    Write-Host "SECURITY ISSUES DETECTED! Please review and fix errors above." -ForegroundColor Red
    exit 1
}
