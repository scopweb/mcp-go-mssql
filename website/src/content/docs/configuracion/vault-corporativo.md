---
title: Vault corporativo
description: Obtén contraseñas SQL desde un vault privado por HTTPS, con referencias por alias, proxy IIS y diagnóstico del arranque.
---

El servidor MCP puede obtener contraseñas o cadenas de conexión desde la API del vault corporativo:
`GET /api/secrets/{environment}`, con autenticación `X-Api-Key` y respuesta JSON de nombres y valores.
No es un cliente genérico de HashiCorp Vault. Esta integración no se aplica al CLI independiente.

## Preparar la aplicación

1. Publica el vault por **HTTPS** con un certificado confiable para el equipo que ejecuta el MCP.
2. Crea una aplicación para el MCP en VaultAdmin y guarda localmente su clave de API. No se utiliza la contraseña de `admin`.
3. Crea los secretos en `dev`, `staging` o `prod`. Sus nombres se comparan exactamente, incluyendo mayúsculas.
4. Configura el `.env` junto al ejecutable real del MCP y reinicia el proceso.

:::caution[Credencial inicial]
La clave de API sigue siendo un secreto local que debe proporcionar el host. No publiques el `.env`,
la clave, contraseñas, DSN reales ni capturas que los contengan. Los ejemplos usan valores ficticios.
:::

## Configuración dinámica

```dotenv title=".env junto al ejecutable — ejemplo"
MSSQL_VAULT_URL=https://vault.example.com/vault/
MSSQL_VAULT_API_KEY=<clave-api-de-la-aplicacion>
MSSQL_VAULT_ENVIRONMENT=prod
MSSQL_DYNAMIC_MODE=true
MSSQL_READ_ONLY=true

MSSQL_DYNAMIC_APP_MAIN_SERVER=sql01.example.com
MSSQL_DYNAMIC_APP_MAIN_DATABASE=AppDB
MSSQL_DYNAMIC_APP_MAIN_USER=ai_reader
MSSQL_DYNAMIC_APP_MAIN_READ_ONLY=true
MSSQL_DYNAMIC_APP_MAIN_PASSWORD_VAULT_SECRET=MainSqlPassword
```

Guarda la contraseña bajo `MainSqlPassword` en esa aplicación y entorno. Elimina el antiguo
`MSSQL_DYNAMIC_APP_MAIN_PASSWORD`. Para una cadena de conexión completa, usa en su lugar
`MSSQL_DYNAMIC_APP_MAIN_CONNECTION_STRING_VAULT_SECRET=MainSqlConnection` y elimina el DSN local.
No combines una referencia de contraseña con una cadena de conexión que la ignoraría.

En modo clásico utiliza `MSSQL_PASSWORD_VAULT_SECRET=SqlPassword` o
`MSSQL_CONNECTION_STRING_VAULT_SECRET=SqlConnection`. Conserva servidor, base y usuario si solo
recuperas la contraseña. No definas simultáneamente un valor local y una referencia para el mismo destino.

Todas las referencias usan una única aplicación y entorno. Los permisos SQL y sus listas permitidas
se configuran localmente; no se importan del vault. Las reglas de TLS de SQL Server se siguen aplicando.

## Carga y rotación

- Una petición al arrancar, antes de `initialize`, con timeout de **10 segundos** y respuesta máxima de **1 MiB**.
- Solo hay petición si existe una referencia `*_VAULT_SECRET`. No se siguen redirecciones ni proxies HTTP del entorno.
- Los secretos referenciados se conservan en una configuración privada en memoria; no se escriben a disco ni se exportan al entorno del proceso.
- Secretos ausentes o vacíos, fuentes ambiguas y fallos del vault detienen el arranque. No hay fallback local.
- Reinicia el MCP después de rotar contraseñas o la clave de API. No existe renovación periódica.
- Un ejecutable en `build/` carga `build/.env`, no el `.env` de la raíz del repositorio.
- Las variables suministradas por el cliente MCP tienen prioridad. `MSSQL_IGNORE_LOCAL_ENV=true` impide cargar el archivo.

:::note[HTTPS obligatorio]
`DEVELOPER_MODE` y `MSSQL_ALLOW_INSECURE_TLS` no relajan HTTPS del vault. No existe una opción para
aceptar HTTP o certificados no confiables. El puerto 8443, por sí solo, no proporciona cifrado.
:::

## Proxy inverso IIS bajo /vault/

Necesitas ARR, URL Rewrite y un binding HTTPS con certificado confiable. Añade esta regla dentro de
`configuration/system.webServer/rewrite/rules`, **antes de cualquier regla genérica**. Ajusta el backend
de ejemplo y conserva las reglas existentes para las otras aplicaciones:

```xml title="web.config — regla adicional"
<rule name="VaultReverseProxy" stopProcessing="true">
  <match url="^vault(?:/(.*))?$" />
  <action type="Rewrite"
          url="http://vault.internal:8443/{R:1}"
          appendQueryString="true" />
</rule>
```

Configura `MSSQL_VAULT_URL=https://gateway.example.com/vault/`. La petición
`/vault/api/secrets/prod` se reenvía como `/api/secrets/prod`, preservando `X-Api-Key`.
Usa reescritura, no redirección. El cliente admite el prefijo de ruta.

```powershell
Invoke-RestMethod "https://gateway.example.com/vault/health"
# Healthy
Invoke-RestMethod "https://gateway.example.com/vault/api/environments"
# dev, staging, prod
```

Restringe `/vault/` a la red o VPN autorizada, incluidos los endpoints administrativos.
El HTTPS del proxy protege MCP → IIS; el backend HTTP de este ejemplo deja IIS → vault sin cifrar.
Protege ese tramo o habilita también TLS en el backend. Las pruebas anteriores no validan secretos ni acceso SQL.

## Diagnóstico

| Síntoma | Qué revisar |
|---|---|
| `Vault configuration failed` y el MCP no carga | El fallo ocurre antes de inicializar MCP. Revisa stderr, la ruta del ejecutable y su `.env`. |
| `MSSQL_VAULT_URL must be an HTTPS URL...` | Usa HTTPS sin usuario/contraseña incrustados, query ni fragmento; revisa también el valor del host que puede sobrescribir el archivo. |
| `vault request failed` | DNS, conectividad, certificado, CA confiable y timeout. No desactives la validación TLS. |
| HTTP 301/302 | Configura la URL final: no se siguen redirecciones. |
| HTTP 401 | Clave de la aplicación, rotación o aplicación deshabilitada. |
| HTTP 500 | Excepción de la API del vault; consulta los logs del servidor que aloja la API. |
| Secreto ausente o vacío | Nombre exacto, aplicación y entorno. |
| `disconnected` inicial en modo dinámico | Normal: usa `dynamic_available` y después `dynamic_connect` con un alias. |
| SQL Server 18456 después de conectar al vault | Login SQL rechazado: revisa cuenta, contraseña y acceso a la base. No demuestra por sí solo contraseña incorrecta. |
| Timeout SQL | Servidor, puerto y conectividad SQL; es una fase distinta de la lectura del vault. |

Consulta también la [referencia de variables](/configuracion/variables-entorno/) y el
[modo dinámico](/guias/modo-dinamico-una-aplicacion-varias-bases/).
