// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	integrations: [
		starlight({
			title: {
				es: 'MCP-Go-MSSQL',
				en: 'MCP-Go-MSSQL',
			},
			description: 'Servidor MCP seguro en Go para conectar Claude Desktop y Claude Code con Microsoft SQL Server',
			logo: {
				src: './src/assets/scopweb.png',
				alt: 'scopweb',
			},
			expressiveCode: {
				themes: ['starlight-dark', 'starlight-light'],
			},
			components: {
				Head: './src/components/Head.astro',
			},
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/DavidSerrano-Rodriguez/mcp-go-mssql' },
			],
			defaultLocale: 'root',
			locales: {
				root: {
					label: 'Español',
					lang: 'es',
				},
				en: {
					label: 'English',
					lang: 'en',
				},
			},
			sidebar: [
				{
					label: 'Inicio',
					translations: { en: 'Getting Started' },
					items: [
						{ label: 'Bienvenida', translations: { en: 'Welcome' }, slug: 'inicio/bienvenida' },
						{ label: 'Instalación', translations: { en: 'Installation' }, slug: 'inicio/instalacion' },
						{ label: 'Configuración básica', translations: { en: 'Basic Configuration' }, slug: 'inicio/configuracion' },
						{ label: 'Inicio rápido', translations: { en: 'Quick Start' }, slug: 'inicio/inicio-rapido' },
					],
				},
				{
					label: 'Herramientas MCP',
					translations: { en: 'MCP Tools' },
					items: [
						{ label: 'Resumen', translations: { en: 'Overview' }, slug: 'herramientas-mcp/resumen' },
						{ label: 'query_database', slug: 'herramientas-mcp/query-database' },
						{ label: 'get_database_info', slug: 'herramientas-mcp/get-database-info' },
						{ label: 'explore', slug: 'herramientas-mcp/explore' },
						{ label: 'inspect', slug: 'herramientas-mcp/inspect' },
						{ label: 'execute_procedure', slug: 'herramientas-mcp/execute-procedure' },
					],
				},
				{
					label: 'CLI de Claude Code',
					translations: { en: 'Claude Code CLI' },
					items: [
						{ label: 'Resumen', translations: { en: 'Overview' }, slug: 'cli/resumen' },
						{ label: 'Comandos', translations: { en: 'Commands' }, slug: 'cli/comandos' },
					],
				},
				{
					label: 'Seguridad',
					translations: { en: 'Security' },
					items: [
						{ label: 'Resumen de seguridad', translations: { en: 'Security Overview' }, slug: 'seguridad/resumen' },
						{ label: 'TLS y cifrado', translations: { en: 'TLS & Encryption' }, slug: 'seguridad/tls-cifrado' },
						{ label: 'Modo solo lectura', translations: { en: 'Read-Only Mode' }, slug: 'seguridad/modo-solo-lectura' },
						{ label: 'Whitelist de tablas', translations: { en: 'Table Whitelist' }, slug: 'seguridad/whitelist-tablas' },
						{ label: 'Protección SQL Injection', translations: { en: 'SQL Injection Protection' }, slug: 'seguridad/sql-injection' },
						{ label: 'Protección ataques IA', translations: { en: 'AI Attack Protection' }, slug: 'seguridad/ataques-ia' },
						{ label: 'Análisis de seguridad', translations: { en: 'Security Analysis' }, slug: 'seguridad/analisis-seguridad' },
						{ label: 'Auditoría y logging', translations: { en: 'Audit & Logging' }, slug: 'seguridad/auditoria' },
					],
				},
				{
					label: 'Configuración',
					translations: { en: 'Configuration' },
					items: [
						{ label: 'Variables de entorno', translations: { en: 'Environment Variables' }, slug: 'configuracion/variables-entorno' },
						{ label: 'Claude Desktop', slug: 'configuracion/claude-desktop' },
						{ label: 'Modos de autenticación', translations: { en: 'Authentication Modes' }, slug: 'configuracion/autenticacion' },
						{ label: 'Autenticación Windows (SSPI)', translations: { en: 'Windows Auth (SSPI)' }, slug: 'configuracion/autenticacion-windows' },
						{ label: 'Connection strings', slug: 'configuracion/connection-strings' },
					],
				},
				{
					label: 'Despliegue',
					translations: { en: 'Deployment' },
					items: [
						{ label: 'Producción', translations: { en: 'Production' }, slug: 'despliegue/produccion' },
						{ label: 'Desarrollo', translations: { en: 'Development' }, slug: 'despliegue/desarrollo' },
						{ label: 'Solución de problemas', translations: { en: 'Troubleshooting' }, slug: 'despliegue/solucion-problemas' },
					],
				},
				{
					label: 'Guías',
					translations: { en: 'Guides' },
					items: [
						{ label: 'Uso con IA', translations: { en: 'AI Usage' }, slug: 'guias/uso-con-ia' },
						{ label: 'Rendimiento', translations: { en: 'Performance' }, slug: 'guias/rendimiento' },
						{ label: 'Testing', slug: 'guias/testing' },
						{ label: 'Actualización de Go', translations: { en: 'Go Upgrade' }, slug: 'guias/actualizacion-go' },
						{ label: 'Integración MCP', translations: { en: 'MCP Integration' }, slug: 'guias/integracion-mcp' },
					],
				},
				{
					label: 'Problemas Resueltos',
					translations: { en: 'Solved Issues' },
					items: [
						{ label: 'tool_search en cada sesión', translations: { en: 'tool_search on every session' }, slug: 'problemas-resueltos/tool-search-sesion' },
					{ label: 'Overflow de tokens en BDs grandes', translations: { en: 'Token overflow on large databases' }, slug: 'problemas-resueltos/token-overflow' },
					],
				},
				{
					label: 'Changelog',
					items: [
						{ label: 'Changelog', slug: 'changelog' },
					],
				},
			],
			customCss: ['./src/styles/custom.css'],
		}),
	],
});
