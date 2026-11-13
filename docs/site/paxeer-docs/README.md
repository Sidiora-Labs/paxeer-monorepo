# Paxeer X Docs

Launch documentation for the merged Paxeer X network: the EVM chain, deterministic LayerX kernel, autonomous agents, payments and commerce, spot and perpetual markets, custody and bridging, programs, developer tools, and network operations.

The site uses Next.js static export. Pages, full-text search, Markdown exports, SEO metadata, and the documentation assistant share one content source in `src/app`. Navigation is in `src/components/nav.ts`.

The supplied stylesheets are in `src/styles/supplied`, imported through `globals.css`. `learn-theme.css` connects their surface, typography and geometry tokens to the documentation and assistant components. The imported base stylesheet repairs one unmatched closing brace from the supplied original. Theme selection uses `body[data-theme]` and the matching class on `html`.

The supplied Hairline library is vendored in `src/lib/hairline` with its license. `DocFigure` selects relevant interactive figures for the homepage and every documentation route, supports pause/resume and reduced motion, and mounts near the viewport. `DocsLayout` provides shared illustrations, section navigation and related documents.

## Development and build

```bash
npm ci
npm run dev
npm run build
```

Development runs at `http://localhost:3000`. The generated `out/` directory contains the static pages. Use the Node runtime below when hosting the AI assistant. `next start` does not serve a static export; for local production inspection use:

```bash
python3 -m http.server 4318 --directory out
```

The production origin defaults to `https://docs.paxeer.app`. To build for a different canonical domain, set `NEXT_PUBLIC_SITE_URL` for the whole build command. Preview hosts do not become canonical URLs.

## Content and discovery

Pages live in `src/app/**/page.tsx`; navigation lives in `src/components/nav.ts`. Each documentation page needs an H1, a `page-description` paragraph, and `export const metadata = metadataFor('/its-route')` imported from `@/lib/seo`. Keep article content as static JSX so search and Markdown generation can read the same source. The content pipeline reads the page description directly, preserving the same text across rendered and machine-readable versions.

The build generates these files directly from local page content:

- Unique titles, descriptions, canonical URLs, Open Graph and Twitter metadata, and structured data.
- `robots.txt` and `sitemap.xml` using the configured canonical origin.
- `llms.txt`, complete `llms-full.txt`, and `/markdown/<route>.md` with headings, tables and code.
- The full-text search index, including section anchors and code examples.

```bash
npm run prepare:docs
npm run lint
npm run audit:seo
SEO_STRICT=1 npm run audit:seo
```

`npm run build` runs generation first and audits the actual exported HTML afterward. The audit checks sitemap pages, metadata, headings, local links, images, robots and Markdown discovery. To inspect a served build, run `node scripts/audit-seo.mjs --base-url http://localhost:4318 --site-url https://docs.paxeer.app`.

## Source and V3 adaptation

The source authority for this review is `../project-adam/tmp/LayerX-Network/paxeer-network/`. The V3 donor is `../tmp/paxeer-docs-v3-main/`. Its developer recipes informed the Foundry, Hardhat, frontend, EVM querying, Python, debugging, parallelization and node troubleshooting guides.

All 72 donor documentation pages are covered by the claim audit in `../spec/neo-capability-convergence/paxeer-v3-content-audit.md`. Imported material uses verified Paxeer interfaces and identifiers. Live RPC availability, deployed contract addresses, performance measurements and active governance settings require separate deployment evidence. Literal source denominations such as `uhpx` are preserved because changing their spelling would change commands and units.

The donor's SEO audit and documentation generation were adapted to this site's rendered HTML and local JSX; no Mintlify configuration or remote Markdown scraping is required. Its `sync-default-configs.mjs` operates on a running node's TOML files and donor MDX markers, so it is not installed as a website optimization script. See `THIRD_PARTY_NOTICES.md` for attribution.

## AI assistant and hosted runtime

The shared `../docs-assistant` package provides the Apps SDK UI chat panel and a same-origin streaming endpoint. Builds create the assistant corpus from this site's published documentation. Set `CENTRA_GATEWAY_API_KEY` in the server environment, then run `npm run serve`; the server listens on port 3000 by default and exposes `/healthz`.

`npm run preview` (Learn) and a static file server (Paxeer) are page previews. Use `npm run serve` to exercise both pages and real assistant requests. Runtime settings and release commands are documented in `../docs-assistant/README.md`.

Build the source Dockerfile from the parent workspace directory so the shared package is available:

```sh
docker build -f paxeer-docs/Dockerfile -t paxeer-docs .
```

Provide gateway credentials only at runtime. The release packager uploads the verified build output to Dokploy without dotenv files or Git history.
