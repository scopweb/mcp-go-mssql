---
title: SET STATISTICS en modo solo lectura
description: Cómo capturar tiempos de ejecución por sentencia dentro de una consulta en modo solo lectura
---

Cuando el servidor está corriendo con `MSSQL_READ_ONLY=true`, puedes
anteponer `SET STATISTICS TIME ON;` (y/o `SET STATISTICS IO ON;`) a un
único SELECT para pedirle a SQL Server que reporte tiempos de
ejecución por sentencia o lecturas lógicas/físicas. Es la **única**
forma multi-statement que el modo solo lectura acepta — cualquier otro
patrón multi-statement (`SELECT 1; DELETE FROM prod`,
`PRINT 'x'; SELECT 1`, …) es rechazado.

## Uso

```json
{
  "name": "query_database",
  "arguments": {
    "query": "SET STATISTICS TIME ON; SET STATISTICS IO ON; SELECT TOP 10 id, name FROM orders ORDER BY id; SET STATISTICS TIME OFF;"
  }
}
```

El envoltorio es parseado por un parser estricto de lenguaje regular.
Permitido:

- `SET STATISTICS TIME/IO (ON|OFF)` inicial (cero o uno de cada tipo)
- Una sentencia base (sentencia única, sin `;` internos)
- `SET STATISTICS TIME/IO (ON|OFF)` final (cero o uno de cada tipo)

Rechazado:

- Un envoltorio alrededor de una sentencia que no es de solo lectura (`SET STATISTICS TIME ON; DELETE FROM prod` → bloqueado por el chequeo existente de read-only sobre la sentencia base).
- Más de una sentencia base (`SET STATISTICS TIME ON; SELECT 1; SELECT 2` → bloqueado como multi-statement).
- Un token distinto de `SET STATISTICS` dentro del envoltorio (`SET STATISTICS TIME ON; PRINT 'x'; SELECT 1` → bloqueado como multi-statement).
- Sentencias vacías entre los tokens del envoltorio (`SET STATISTICS TIME ON; ; SELECT 1` → bloqueado).
- Un envoltorio alrededor de una escritura se rechaza con `READ_ONLY=true`, aunque la tabla esté en la whitelist.

## Respuesta

La forma del payload de respuesta cambia de un array JSON plano de
filas a un objeto estructurado con un bloque `stats`:

```json
{
  "rows": [{ "id": 1, "name": "x" }, { "id": 2, "name": "y" }],
  "row_count": 2,
  "truncated": false,
  "stats": {
    "enabled": true,
    "time_on": true,
    "io_on": true,
    "messages": [
      " SQL Server Execution Times:",
      "   CPU time = 16 ms,  elapsed time = 22 ms.",
      "Table 'orders'. Scan count 1, logical reads 4, physical reads 0, read-ahead reads 0, lob logical reads 0, lob physical reads 0, lob read-ahead reads 0."
    ],
    "parsed": {
      "cpu_time_ms": 16,
      "elapsed_time_ms": 22,
      "logical_reads": 4,
      "physical_reads": 0,
      "scan_count_orders": 1,
      "logical_reads_orders": 4,
      "physical_reads_orders": 0
    }
  }
}
```

El bloque `stats` siempre está presente (con `enabled:false` cuando
no se usó un envoltorio) para que los clientes tengan una forma
estable. La fila centinela `_truncated` previa se reemplaza por el
booleano `truncated`.

## Notas

- El envoltorio se captura a través de la cola `sqlexp.ReturnMessage`
  expuesta por `go-mssqldb` v1.10.0; la capa de conexión se refactorizó
  de `sql.Open("sqlserver", dsn)` a
  `sql.OpenDB(mssql.NewConnector(dsn))` para que esto funcione.
- La herramienta CLI (`examples/claude-code/db-connector.go`) no se ve afectada
  — los envoltorios son una preocupación exclusiva del tool MCP.
- Para lotes multi-statement el bloque `stats` reporta los tiempos de
  cada sentencia individualmente (los tiempos de la sentencia base;
  los SET inicial/final no producen tiempos por sí mismos).