# Paxeer X Docs

This directory is the documentation site of this repository. It covers the Paxeer X network, the embedded LayerX kernel, agents, payments, markets, programs, developer tools and network operations.

The site uses Next.js static export. Pages, full-text search, Markdown exports, SEO metadata and the documentation assistant share the content in `src/app`. Navigation is in `src/components/nav.ts`. The shared assistant package is in `../docs-assistant`.

## Development and build

From the repository root, enter the site directory and install its locked dependencies with Node.js 22 and npm:

```bash
cd docs/site/paxeer-docs
npm ci
npm run dev
```

Development runs at `http://localhost:3000`. From this directory, build the production static export:

```bash
npm run build
```

The generated `out/` directory contains the static pages. The documentation workflow uploads that directory and deploys pushes to `main` to GitHub Pages. The production canonical origin defaults to `https://docs.paxeer.app`. Set `NEXT_PUBLIC_SITE_URL` for the whole build command to use a different canonical origin; preview hosts do not change canonical URLs.

For local inspection of the static export:

```bash
python3 -m http.server 4318 --directory out
```

## Content and discovery

Pages live in `src/app/**/page.tsx`. Each documentation page needs an H1, a `page-description` paragraph and `export const metadata = metadataFor('/its-route')`, imported from `@/lib/seo`. Keep article content as static JSX so search and Markdown generation can read the same source. The content pipeline reads each page description directly, preserving it across rendered and machine-readable versions.

The build generates:

- Titles, descriptions, canonical URLs, social metadata and structured data.
- `robots.txt` and `sitemap.xml` using the configured canonical origin.
- `llms.txt`, `llms-full.txt` and `/markdown/<route>.md`.
- A full-text search index with section anchors and code examples.
- `.generated/assistant-docs.json` for the assistant server.

From this directory, the individual content and audit commands are:

```bash
npm run prepare:docs
npm run lint
npm run audit:seo
SEO_STRICT=1 npm run audit:seo
```

`npm run build` runs content generation before the Next.js build and audits the exported HTML afterward. The audit checks sitemap pages, metadata, headings, local links, images, robots and Markdown discovery. To inspect a served export:

```bash
node scripts/audit-seo.mjs --base-url http://localhost:4318 --site-url https://docs.paxeer.app
```

## Styles and illustrations

`src/styles/supplied` contains the imported stylesheets, loaded through `globals.css`. `learn-theme.css` connects their tokens to the documentation and assistant components. Theme selection uses `body[data-theme]` and the matching class on `html`.

The Hairline library is vendored in `src/lib/hairline` with its license. `DocFigure` selects interactive figures, supports pause/resume and reduced motion, and mounts near the viewport. `DocsLayout` provides section navigation and related documents. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for attribution.

## Documentation assistant

The `../docs-assistant` package provides the chat panel and a same-origin streaming endpoint. A static host serves the pages; the Node server also handles assistant requests. From this directory, supply `CENTRA_GATEWAY_API_KEY` through the server environment and run:

```bash
npm run serve
```

The server serves `out/`, listens on port 3000 by default and exposes `/healthz`. Runtime settings are documented in [the assistant README](../docs-assistant/README.md).

Build the Docker image from the repository root with `docs/site` as its context:

```sh
docker build -f docs/site/paxeer-docs/Dockerfile -t paxeer-docs docs/site
```

Provide gateway credentials only at runtime.
