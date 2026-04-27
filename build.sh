#!/bin/bash

# MCP-Go-MSSQL Build Script

BUILD_DIR="build"
LOG_FILE="crash.log"

show_menu() {
    cat << EOF
========================================
  MCP-Go-MSSQL Build Script
========================================

Selecciona el tipo de build:

1) Produccion (SEGURO)
   Genera: $BUILD_DIR/mcp-go-mssql.exe (~7MB)
   Flags: -ldflags "-w -s" (stripped)
   Uso: Despliegue en servidores de produccion
   Stack traces: Solo direcciones hex

2) Desarrollo (DEBUG)
   Genera: $BUILD_DIR/mcp-go-mssql-dev.exe (~10MB)
   Flags: Ninguno (sin strip)
   Uso: Debugging local, errores visibles en terminal
   Stack traces: Nombres completos de funciones y lineas

3) Con log a archivo (DEBUG + CRASH LOG)
   Genera: $BUILD_DIR/mcp-go-mssql-dev.exe (~10MB)
   Flags: Ninguno (sin strip)
   Uso: Guardar output en crash.log para analisis posterior
   Execution: $BUILD_DIR/mcp-go-mssql-dev.exe 2>&1 | tee $LOG_FILE

4) Salir

EOF
}

build_prod() {
    echo ""
    echo "========================================"
    echo "  Build Produccion (SEGURO)"
    echo "========================================"
    echo ""
    echo "Generando executable seguro para produccion..."
    echo ""

    go build -ldflags "-w -s" -o "$BUILD_DIR/mcp-go-mssql.exe" -v

    if [ $? -ne 0 ]; then
        echo ""
        echo "[ERROR] Build fallido"
        read -p "Press ENTER para continuar..."
        return 1
    fi

    size=$(du -h "$BUILD_DIR/mcp-go-mssql.exe" | cut -f1)
    echo ""
    echo "========================================"
    echo "  Build Produccion exitoso"
    echo "========================================"
    echo ""
    echo "Ubicacion: $BUILD_DIR/mcp-go-mssql.exe"
    echo "Tamanio: $size (~7MB)"
    echo ""
    echo "Caracteristicas:"
    echo "  - Binary pequeno (~7MB)"
    echo "  - Sin symbol table ni debug info (stripped)"
    echo "  - OPTIMIZADO PARA PRODUCCION"
    echo ""
    echo "Limitaciones en produccion:"
    echo "  - Stack traces sin nombres de funciones"
    echo "  - Necesitas este binary + logs del servidor para debuggear"
    echo ""

    read -p "Press ENTER para continuar..."
}

build_dev() {
    echo ""
    echo "========================================"
    echo "  Build Desarrollo (DEBUG)"
    echo "========================================"
    echo ""
    echo "Generando executable con debug symbols..."
    echo ""

    go build -o "$BUILD_DIR/mcp-go-mssql-dev.exe" -v

    if [ $? -ne 0 ]; then
        echo ""
        echo "[ERROR] Build fallido"
        read -p "Press ENTER para continuar..."
        return 1
    fi

    size=$(du -h "$BUILD_DIR/mcp-go-mssql-dev.exe" | cut -f1)
    echo ""
    echo "========================================"
    echo "  Build Desarrollo exitoso"
    echo "========================================"
    echo ""
    echo "Ubicacion: $BUILD_DIR/mcp-go-mssql-dev.exe"
    echo "Tamanio: $size (~10MB)"
    echo ""
    echo "Caracteristicas:"
    echo "  - Binary completo (~10MB)"
    echo "  - Symbol table y debug info incluidos"
    echo "  - Stack traces completos con nombres de funciones"
    echo ""
    echo "Uso:"
    echo "  1. Ejecuta: $BUILD_DIR/mcp-go-mssql-dev.exe"
    echo "  2. Los errores apareceran en esta terminal"
    echo "  3. Copia los stack traces para analizar"
    echo ""

    read -p "Press ENTER para continuar..."
}

build_log() {
    echo ""
    echo "========================================"
    echo "  Build Desarrollo + Log (DEBUG + CRASH LOG)"
    echo "========================================"
    echo ""
    echo "Generando executable con debug symbols..."

    # Limpiar log anterior si existe
    if [ -f "$LOG_FILE" ]; then
        echo "Limpiando $LOG_FILE anterior..."
        rm -f "$LOG_FILE"
    fi
    echo ""

    go build -o "$BUILD_DIR/mcp-go-mssql-dev.exe" -v

    if [ $? -ne 0 ]; then
        echo ""
        echo "[ERROR] Build fallido"
        read -p "Press ENTER para continuar..."
        return 1
    fi

    size=$(du -h "$BUILD_DIR/mcp-go-mssql-dev.exe" | cut -f1)
    echo ""
    echo "========================================"
    echo "  Build exitoso"
    echo "========================================"
    echo ""
    echo "Ubicacion: $BUILD_DIR/mcp-go-mssql-dev.exe"
    echo "Log file: $LOG_FILE (se crea al ejecutar)"
    echo "Tamanio: $size (~10MB)"
    echo ""
    echo "Caracteristicas:"
    echo "  - Binary completo (~10MB)"
    echo "  - Symbol table y debug info incluidos"
    echo "  - Output completo guardado en $LOG_FILE"
    echo ""
    echo "Ejecucion:"
    echo "  $BUILD_DIR/mcp-go-mssql-dev.exe 2>&1 | tee $LOG_FILE"
    echo ""
    echo "Esto guardara todo el output en $LOG_FILE"
    echo "INCLUDING: stderr (errores) + stdout (logs JSON)"
    echo ""

    read -p "Press ENTER para continuar..."
}

# Create build directory if it doesn't exist
mkdir -p "$BUILD_DIR"

while true; do
    clear
    show_menu
    read -p "Opcion (1-4): " choice

    case $choice in
        1) build_prod ;;
        2) build_dev ;;
        3) build_log ;;
        4) echo "Saliendo..."; exit 0 ;;
        *) echo "Opcion invalida"; sleep 1 ;;
    esac
done
 