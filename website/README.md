# Documentation site

Astro Starlight site for mcp-go-mssql. Published at https://mcp-go-mssql.scopweb.com.

```bash
cd website
npm install
npm run dev
```

Source lives in `src/content/docs/` (Spanish) and `src/content/docs/en/` (English).

Use Node.js 22.12+ (tested with Node.js 24). Run `npm ci`, `npm audit`, and
`npm run build` to verify the locked dependency set.

The scoped `postcss-nested` override pins `postcss-selector-parser` to 7.1.6
to fix GHSA-rj75-hqrm-r3gf. Expressive Code still requests a vulnerable 6.x
parser through `postcss-nested`; remove the override once its dependency range
accepts a patched parser. The 7.x iterator change and the full site build were
checked when introducing this override.
