# Guía de Uso para Claude Desktop e IA

**Fecha:** 21 de noviembre de 2025  
**Versión:** 1.0.0

## 🤖 ¿Puede la IA trabajar con las restricciones de seguridad?

**Respuesta corta: ¡SÍ! Absolutamente.**

Las restricciones de seguridad están **diseñadas específicamente** para permitir que Claude Desktop y otros asistentes de IA trabajen de manera segura en bases de datos de producción. La IA **NO se verá limitada** en sus capacidades útiles.

---

## ✅ Lo que la IA PUEDE hacer (Todo lo que necesita)

### 1. **Consultas SELECT - 100% Funcional**

```sql
-- ✅ Consultas simples
SELECT * FROM users WHERE active = 1

-- ✅ JOINs complejos
SELECT u.*, o.order_total 
FROM users u 
JOIN orders o ON u.id = o.user_id

-- ✅ Subconsultas
SELECT * FROM (
    SELECT id, name FROM users WHERE country = 'ES'
) subquery

-- ✅ CTEs (Common Table Expressions)
WITH active_users AS (
    SELECT * FROM users WHERE active = 1
)
SELECT * FROM active_users

-- ✅ Agregaciones
SELECT country, COUNT(*) as total, AVG(age) as avg_age
FROM users
GROUP BY country
HAVING COUNT(*) > 10

-- ✅ Window functions
SELECT 
    name, 
    salary,
    ROW_NUMBER() OVER (PARTITION BY department ORDER BY salary DESC) as rank
FROM employees
```

**Resultado:** La IA puede analizar, consultar y extraer información de toda la base de datos sin restricciones.

### 2. **Análisis de Datos - 100% Funcional**

```sql
-- ✅ Estadísticas
SELECT 
    MIN(price) as min_price,
    MAX(price) as max_price,
    AVG(price) as avg_price,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY price) as median
FROM products

-- ✅ Tendencias temporales
SELECT 
    DATEPART(year, order_date) as year,
    DATEPART(month, order_date) as month,
    SUM(total) as monthly_revenue
FROM orders
GROUP BY DATEPART(year, order_date), DATEPART(month, order_date)
ORDER BY year, month

-- ✅ Correlaciones
SELECT 
    category,
    AVG(rating) as avg_rating,
    COUNT(*) as product_count
FROM products
GROUP BY category
```

### 3. **Exploración de Esquema - 100% Funcional**

```sql
-- ✅ Ver estructura de tablas
SELECT 
    TABLE_NAME,
    COLUMN_NAME,
    DATA_TYPE,
    IS_NULLABLE
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = 'dbo'

-- ✅ Ver relaciones
SELECT 
    fk.name as foreign_key_name,
    tp.name as parent_table,
    cp.name as parent_column,
    tr.name as referenced_table,
    cr.name as referenced_column
FROM sys.foreign_keys fk
INNER JOIN sys.tables tp ON fk.parent_object_id = tp.object_id
-- ... más JOINs para información completa
```

---

## ⚠️ Lo que la IA NO PUEDE hacer (Por seguridad)

### 1. **Modificaciones Directas a Producción**

```sql
-- ❌ BLOQUEADO en modo READ_ONLY
UPDATE users SET email = 'new@email.com' WHERE id = 1
DELETE FROM old_data WHERE date < '2020-01-01'
INSERT INTO logs VALUES ('new log entry')
DROP TABLE temp_table
```

**¿Por qué está bloqueado?**
- Protege datos de producción contra modificaciones accidentales
- Previene que la IA borre o corrompa datos críticos
- Evita que errores de la IA afecten la base de datos

**¿Cómo afecta esto a la IA?**
- **NO afecta** su capacidad de análisis
- La IA puede **sugerir** los comandos SQL correctos
- **TÚ** ejecutas manualmente las modificaciones si son necesarias

### 2. **Ejecución de Código del Sistema**

```sql
-- ❌ BLOQUEADO
EXEC xp_cmdshell 'dir'
EXEC sp_configure 'xp_cmdshell', 1
```

**¿Por qué está bloqueado?**
- Previene command injection
- Protege el servidor contra ejecución de código malicioso

---

## 🎯 Configuraciones Recomendadas por Escenario

### Escenario 1: **Análisis de Datos con IA (RECOMENDADO)**

```json
{
  "mcpServers": {
    "production-analytics": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "env": {
        "MSSQL_SERVER": "prod.database.windows.net",
        "MSSQL_DATABASE": "ProductionDB",
        "MSSQL_USER": "analytics_user",
        "MSSQL_PASSWORD": "secure_password",
        "MSSQL_READ_ONLY": "true",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

**✅ Perfecto para:**
- Análisis de datos con Claude
- Exploración de base de datos
- Generación de reportes
- Responder preguntas sobre los datos
- Optimización de queries

**❌ Limitaciones:**
- No puede modificar datos (eso es bueno para producción)

---

### Escenario 2: **IA con Tablas Temporales (AI-SAFE)**

```json
{
  "mcpServers": {
    "ai-workspace": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "env": {
        "MSSQL_SERVER": "prod.database.windows.net",
        "MSSQL_DATABASE": "ProductionDB",
        "MSSQL_USER": "ai_user",
        "MSSQL_PASSWORD": "secure_password",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,v_temp_ai,staging_ai",
        "DEVELOPER_MODE": "false"
      }
    }
  }
}
```

**✅ Perfecto para:**
- IA puede leer toda la base de datos
- IA puede escribir en tablas temporales específicas
- Ideal para experimentación y pruebas
- Procesamiento de datos intermedios

**Ejemplo de uso:**

```sql
-- ✅ La IA puede leer producción
SELECT * FROM production_users WHERE active = 1

-- ✅ La IA puede escribir en tablas temporales
INSERT INTO temp_ai (user_id, calculated_score)
SELECT id, (purchases * 0.5 + reviews * 0.3) as score
FROM production_users

-- ✅ La IA puede procesar en su workspace
UPDATE temp_ai 
SET status = 'high_value'
WHERE calculated_score > 100

-- ❌ BLOQUEADO: La IA NO puede modificar producción
UPDATE production_users SET status = 'high_value'  -- ¡Error!
```

---

### Escenario 3: **Desarrollo con IA (Acceso Completo)**

```json
{
  "mcpServers": {
    "dev-database": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "env": {
        "MSSQL_SERVER": "localhost",
        "MSSQL_DATABASE": "DevDB",
        "MSSQL_USER": "dev_user",
        "MSSQL_PASSWORD": "dev_password",
        "MSSQL_READ_ONLY": "false",
        "DEVELOPER_MODE": "true"
      }
    }
  }
}
```

**✅ Perfecto para:**
- Base de datos de desarrollo local
- Testing y experimentación
- La IA puede crear/modificar/eliminar todo
- Ideal para prototipado rápido

**⚠️ NUNCA usar en producción**

---

## 🤔 Preguntas Frecuentes

### **P: ¿La IA se sentirá "frustrada" por las limitaciones?**

**R:** No. Claude es muy capaz de trabajar dentro de restricciones. De hecho:
- Puede leer y analizar TODA la base de datos
- Puede generar las queries SQL correctas
- Puede sugerir modificaciones que TÚ ejecutas manualmente
- Es consciente de que proteger datos de producción es importante

### **P: ¿Qué pasa si la IA intenta hacer un UPDATE?**

**R:** Recibirá un error claro:

```
Query Error: read-only mode: query contains forbidden operation 'UPDATE'
```

Y Claude responderá algo como:
> "Entiendo que esta base de datos está en modo read-only. Puedo sugerirte el comando UPDATE 
> correcto para que lo ejecutes manualmente, o podemos usar una tabla temporal si está 
> configurada en el whitelist."

### **P: ¿Puede la IA analizar datos tan bien como con acceso completo?**

**R:** **¡SÍ!** El 99% del trabajo de análisis de datos usa SELECT. La IA puede:
- ✅ Hacer queries complejas con JOINs, CTEs, subqueries
- ✅ Calcular estadísticas y agregaciones
- ✅ Generar visualizaciones (extrayendo datos)
- ✅ Optimizar queries
- ✅ Explorar esquemas y relaciones
- ✅ Identificar problemas de rendimiento

### **P: ¿Necesito darle acceso completo para que sea útil?**

**R:** **NO.** La configuración AI-Safe es perfecta para la mayoría de casos:

```bash
# Configuración óptima para Claude Desktop
MSSQL_READ_ONLY=true
MSSQL_WHITELIST_TABLES=temp_ai,staging_ai
```

La IA puede:
1. Leer toda la base de datos para análisis
2. Escribir en tablas temporales para procesamiento
3. Sugerirte los cambios finales para que tú los apliques

---

## 💡 Ejemplos de Conversación con Claude

### Ejemplo 1: Análisis sin Restricciones

**Usuario:** "Muéstrame los 10 clientes con más compras este año"

**Claude:**
```sql
SELECT TOP 10
    c.customer_id,
    c.name,
    COUNT(o.order_id) as total_orders,
    SUM(o.total_amount) as total_spent
FROM customers c
JOIN orders o ON c.customer_id = o.customer_id
WHERE YEAR(o.order_date) = 2025
GROUP BY c.customer_id, c.name
ORDER BY total_spent DESC
```

✅ **Funciona perfectamente** - Es una query SELECT.

---

### Ejemplo 2: Modificación con Restricción

**Usuario:** "Actualiza el email del cliente ID 123"

**Claude (en modo READ_ONLY):**
> "Actualmente esta base de datos está en modo read-only, así que no puedo ejecutar 
> la actualización directamente. Aquí está el comando SQL que necesitas ejecutar:
> 
> ```sql
> UPDATE customers 
> SET email = 'nuevo@email.com' 
> WHERE customer_id = 123
> ```
>
> ¿Quieres que te ayude a verificar que este comando es correcto antes de ejecutarlo?"

✅ **La IA sigue siendo útil** - Te da el comando correcto.

---

### Ejemplo 3: Usando Tablas Temporales

**Usuario:** "Calcula un score para cada cliente basado en sus compras"

**Claude (con whitelist de temp_ai):**
```sql
-- Primero, limpio la tabla temporal
DELETE FROM temp_ai WHERE calculation_type = 'customer_score';

-- Luego calculo y guardo los scores
INSERT INTO temp_ai (entity_id, calculation_type, score, details)
SELECT 
    c.customer_id,
    'customer_score' as calculation_type,
    (COUNT(o.order_id) * 10 + SUM(o.total_amount) * 0.1) as score,
    JSON_OBJECT(
        'total_orders', COUNT(o.order_id),
        'total_spent', SUM(o.total_amount)
    ) as details
FROM customers c
LEFT JOIN orders o ON c.customer_id = o.customer_id
GROUP BY c.customer_id;

-- Ahora puedo mostrarte los resultados
SELECT * FROM temp_ai WHERE calculation_type = 'customer_score' ORDER BY score DESC;
```

✅ **Funciona perfectamente** - Usa tablas temporales para trabajo intermedio.

---

## 🎯 Recomendación Final

**Para Claude Desktop en producción, usa esta configuración:**

```json
{
  "mcpServers": {
    "production-safe": {
      "command": "C:\\path\\to\\mcp-go-mssql.exe",
      "env": {
        "MSSQL_SERVER": "production.database.windows.net",
        "MSSQL_DATABASE": "ProductionDB",
        "MSSQL_USER": "claude_user",
        "MSSQL_PASSWORD": "secure_password",
        "MSSQL_READ_ONLY": "true",
        "MSSQL_WHITELIST_TABLES": "temp_ai,staging_claude",
        "DEVELOPER_MODE": "false",
        "MSSQL_MAX_QUERY_SIZE": "2097152"
      }
    }
  }
}
```

**¿Por qué esta configuración?**

1. ✅ **READ_ONLY=true** - Protege toda la base de datos de modificaciones
2. ✅ **WHITELIST_TABLES** - Permite a Claude usar tablas temporales
3. ✅ **DEVELOPER_MODE=false** - Seguridad máxima en producción
4. ✅ **MAX_QUERY_SIZE=2MB** - Permite queries complejas de análisis

**Resultado:**
- 🔒 Seguridad máxima para producción
- 🤖 Claude puede analizar y procesar datos
- ⚡ Rendimiento sin limitaciones
- 📊 Capacidades de análisis al 100%

---

## 📚 Recursos Adicionales

- Ver [SECURITY_ANALYSIS.md](SECURITY_ANALYSIS.md) para detalles de seguridad
- Ver [WHITELIST_SECURITY.md](WHITELIST_SECURITY.md) para configuración de whitelist
- Ver [README.md](../README.md) para configuración completa

---

**Conclusión: Las restricciones de seguridad NO limitan la utilidad de Claude. Al contrario, permiten que Claude trabaje de manera segura en producción, que es exactamente lo que necesitas.** ✅
