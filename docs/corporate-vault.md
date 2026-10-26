# Vault corporativo: credenciales de SQL Server

El servidor MCP soporta la API del vault corporativo: `GET /api/secrets/{environment}`
con la cabecera `X-Api-Key`. La respuesta es un objeto JSON de nombres y valores de secretos.
No utiliza el login de administrador ni depende de HashiCorp Vault. El CLI independiente no carga secretos del vault.

## Preparar el vault

1. Publica la API con **HTTPS** y un certificado confiable para el equipo donde se ejecuta el MCP.
   `http://vault.internal:8443/` es HTTP aunque use el puerto 8443: no se admite para transportar secretos.
   Usa el nombre DNS o IP que figure en el certificado e instala la CA corporativa en el almacén del sistema.
2. En VaultAdmin crea una aplicación para este MCP y guarda su clave de API en el mecanismo de
   secretos del host que ejecuta el MCP. La clave se muestra una sola vez.
3. Crea los secretos en el entorno elegido (`dev`, `staging` o `prod`).
4. Configura las referencias siguientes y reinicia el MCP.

## Conexión principal

Ejemplo de variables de entorno (los valores entre `<...>` se sustituyen localmente):

```dotenv
MSSQL_VAULT_URL=https://<nombre-del-vault>
MSSQL_VAULT_API_KEY=<clave-de-la-aplicacion>
MSSQL_VAULT_ENVIRONMENT=prod

MSSQL_SERVER=sql01
MSSQL_DATABASE=MiBase
MSSQL_USER=mcp_reader
MSSQL_READ_ONLY=true
MSSQL_PASSWORD_VAULT_SECRET=SqlPassword
```

En VaultAdmin, guarda la contraseña bajo la clave `SqlPassword` de esa aplicación y entorno.
Elimina `MSSQL_PASSWORD` de la configuración local para evitar fuentes ambiguas.

También se puede almacenar la **cadena de conexión completa**: utiliza
`MSSQL_CONNECTION_STRING_VAULT_SECRET=SqlConnection` y guarda el DSN en `SqlConnection`.
Es útil para los DSN URL usados con servidores antiguos. Las reglas de TLS de SQL Server
se aplican igualmente al DSN recuperado del vault.

## Alias dinámicos

```dotenv
MSSQL_DYNAMIC_MODE=true
MSSQL_READ_ONLY=true
MSSQL_VAULT_URL=https://<nombre-del-vault>
MSSQL_VAULT_API_KEY=<clave-de-la-aplicacion>
MSSQL_VAULT_ENVIRONMENT=prod

MSSQL_DYNAMIC_APP_MAIN_SERVER=sql01
MSSQL_DYNAMIC_APP_MAIN_DATABASE=MiBase
MSSQL_DYNAMIC_APP_MAIN_USER=mcp_reader
MSSQL_DYNAMIC_APP_MAIN_READ_ONLY=true
MSSQL_DYNAMIC_APP_MAIN_PASSWORD_VAULT_SECRET=MainSqlPassword

MSSQL_DYNAMIC_LEGACY_READ_ONLY=true
MSSQL_DYNAMIC_LEGACY_CONNECTION_STRING_VAULT_SECRET=LegacySqlConnection
```

Los secretos referenciados deben pertenecer a la misma aplicación y entorno del vault.
Las claves de los secretos se comparan exactamente, incluyendo mayúsculas y minúsculas.
Se admiten alias con guiones bajos, por ejemplo `APP_MAIN`.

## Comportamiento

- Solo se consultará el vault cuando exista una referencia `*_VAULT_SECRET`.
- Solo se admiten referencias para `PASSWORD` y `CONNECTION_STRING`, clásicas o por alias.
  Los permisos (`READ_ONLY`, listas permitidas, etc.) se configuran localmente.
- Se realiza **una petición al arrancar**, con timeout de 10 segundos y límite de respuesta de 1 MiB.
  El arranque MCP puede demorarse hasta ese timeout si el vault no responde.
- No se siguen redirecciones ni se utilizan los proxies HTTP del entorno.
- Los valores se conservan en memoria para construir las conexiones; no se escriben a disco,
  no se exportan al entorno del proceso ni se ofrecen como herramientas MCP.
- Un HTTP 401/500, fallo TLS, secreto ausente/vacío o respuesta inválida detiene el arranque.
  No hay recuperación automática usando una contraseña local.
- Una referencia y un valor local para el mismo destino son incompatibles. También se rechaza
  combinar una referencia de contraseña con una cadena de conexión que la ignoraría.
- Para recoger una rotación de contraseña o clave de API, actualiza lo necesario y **reinicia el MCP**.
  No hay renovación periódica en esta primera integración.
- La clave de API es la credencial inicial que debe proporcionar el host. No la incluyas en Git,
  en capturas o en conversaciones. La cuenta `admin` del vault no se utiliza en el MCP.

## Diagnóstico

El `.env` se busca junto al ejecutable, no en el directorio de trabajo. Por ejemplo, un binario en
`build/` carga `build/.env`. Las variables del host tienen prioridad; `MSSQL_IGNORE_LOCAL_ENV=true`
impide cargar ese archivo. Confirma la ruta `command` del cliente MCP y reinicia tras cambiar la configuración.

`Vault configuration failed` ocurre antes de `initialize`: un error de URL, autenticación o secretos
puede aparecer en el cliente como un fallo genérico de carga del MCP. La URL debe ser HTTPS, sin
credenciales incrustadas, query ni fragmento. No existe una excepción para HTTP; ni `DEVELOPER_MODE`
ni `MSSQL_ALLOW_INSECURE_TLS` relajan la conexión al vault.

`Vault configuration failed: vault returned HTTP 500` indica un error del servidor vault;
consulta sus logs para corregirlo. Los cuerpos de error se omiten para evitar filtrar secretos.
Un fallo de certificado exige corregir HTTPS/la confianza en la CA: no se desactiva su validación.

La integración se prueba con servidores HTTPS locales de test; la conexión real requiere un vault
operativo, la aplicación creada y su clave configurada en el equipo de ejecución.

En modo dinámico, `disconnected` al inicializar es normal. Usa `dynamic_available` y `dynamic_connect`
para seleccionar un alias. Un error SQL 18456 significa que SQL Server rechaza el inicio de sesión;
revisa login y acceso a la base, no solo la contraseña. Un timeout SQL requiere comprobar red,
servidor y puerto. Ambos pueden ocurrir después de recuperar correctamente los secretos.

## HTTPS mediante proxy IIS

Con ARR y URL Rewrite habilitados y un binding HTTPS con certificado confiable, añade esta regla
**antes** de la regla genérica del sitio. Sustituye el backend de ejemplo por el de tu despliegue:

```xml
<rule name="VaultReverseProxy" stopProcessing="true">
  <match url="^vault(?:/(.*))?$" />
  <action type="Rewrite"
          url="http://vault.internal:8443/{R:1}"
          appendQueryString="true" />
</rule>
```

La regla va dentro de `configuration/system.webServer/rewrite/rules`; no reemplaza las reglas de
otras aplicaciones. Con `MSSQL_VAULT_URL=https://gateway.example.com/vault/`, el MCP solicita
`/vault/api/secrets/prod`, que se reenvía al backend como `/api/secrets/prod`. La cabecera `X-Api-Key`
debe llegar intacta. No uses una redirección al backend: el cliente no sigue redirecciones.

Comprueba la ruta sin enviar credenciales:

```powershell
Invoke-RestMethod "https://gateway.example.com/vault/health"
# Healthy
Invoke-RestMethod "https://gateway.example.com/vault/api/environments"
# dev, staging, prod
```

Restringe `/vault/` a la red o VPN autorizada, incluidos los endpoints administrativos. El HTTPS del
proxy cifra MCP → IIS; con este ejemplo, IIS → vault sigue siendo HTTP. Protege ese tramo de red o
configura también TLS en el backend. Un health correcto no valida la clave de API ni las credenciales SQL.
