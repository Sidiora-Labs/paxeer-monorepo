# Codebase map

This directory holds the generator, static browser application, HTTP server and tests for an offline, navigable snapshot of the Paxeer X Network checkout. The map combines the Git file inventory and working-tree state with Codify symbols, resolved calls, imports and routes from `.codegraph/graph.db`, and requirements and task dependencies from the active specification. It supports exploring the network's source code and its static relationships.

## Layout

- [app.js](app.js) — browser views, search, graph rendering and selection inspector.
- [delivery.mjs](delivery.mjs) — snapshot loading, task search, wave grouping and paginated delivery graphs.
- [generate.mjs](generate.mjs) — snapshot generation from Git, the Codify database and the active specification.
- [index.html](index.html) — browser application entry point and navigation controls.
- [package.json](package.json) — Node.js requirement and generation, serving and test scripts.
- [README.md](README.md) — directory overview and usage.
- [serve.mjs](serve.mjs) — static HTTP server for the application and generated snapshot.
- [styles.css](styles.css) — application styling.
- [data/](data/) — generated snapshot location; its `.gitignore` excludes generated contents.
- [test/](test/) — snapshot, server and browser-consumer checks with local fixtures.

### Snapshot contents

- Every Git-tracked path is present in the file inventory, including paths excluded from semantic indexing.
- Indexed files carry the database's language, line count, hash, symbol counts and reference counts. These describe the last index; refresh it before generating a snapshot of changed source.
- Internal call edges exist only where Codify records a concrete `target_id`.
- Internal import edges exist only where Codify records a concrete `target_file_id`.
- Package views are manifest scopes: a file is grouped under its nearest ancestor manifest, and scope-to-scope edges are aggregated from resolved file edges.
- External, receiver-only, ambiguous and unresolved references remain visible as coverage counts; the viewer does not invent targets for them.
- Delivery data contains tasks, statuses, waves, requirements, declared symbols, touch paths and dependencies from the selected specification. Decision, policy and other KVX sections cannot overwrite adjacent tasks or requirements.
- The header displays the Git commit, branch, dirty-file indicator and generation time. Snapshot metadata also records the graph database timestamp and Codify schema.

The graph renders static source relationships. It does not establish runtime reachability, deployment state or task qualification. The coverage strip and each file inspector distinguish indexed files from files known only through Git.

### Navigation

- Start in **Systems**, then double-click an area, package or file to drill toward symbols.
- Use **Imports** to see unresolved/external modules globally or the exact imports for a selected file.
- Use **Routes** for detected framework entry points.
- **Delivery** starts with a wave overview, including aggregate dependencies between waves. Click a wave in the inventory or graph to expand it. Wave pages show at most 80 waves; task pages show at most 160 tasks in a fixed grid. Use **Previous page** and **Next page** to browse, and **All** to return to the overview. Dependency edges within a task page are drawn; the inspector retains prerequisites and dependents across pages and waves.
- Search reaches all tasks, files and symbols, including items outside the visible page. Choosing a task opens its wave and the page containing it. Enter at least two characters; results are capped at 120 matches, so narrow a broad query with a task ID or title.
- Press `/` to search, `F` to fit, `Escape` to clear and `Left Arrow` to return to the previous scope.

## Build and test

The scripts in [package.json](package.json) run directly with Node.js. The `generate` script builds the snapshot; the browser assets are served directly.

Generation requires:

- Node.js 22.5 or later, with the built-in `node:sqlite` module, and Git available on `PATH`.
- A compatible Codify index at `.codegraph/graph.db`. For a new checkout, initialize it with `cg init`; refresh an existing compatible index with `cg sync` before generation.
- Graph schema version **14**, enforced by `GRAPH_SCHEMA_VERSION`. A different database schema produces an `Unsupported Codify schema …; expected 14.` error. Both `meta.codifySchemaVersion` and `meta.sources` identify the accepted schema. Use a compatible index and tool version.

From the repository root, with those prerequisites satisfied:

```bash
cg sync
npm --prefix tools/codebase-map run generate
npm --prefix tools/codebase-map run serve
```

Generation writes `tools/codebase-map/data/codebase-map.json`, which is ignored by Git. The server serves the existing snapshot without generating or refreshing it. Generate the snapshot before opening the viewer at the location printed by the server, and regenerate after source or specification changes.

The portable checks run the snapshot producer, HTTP server and Delivery consumer using local fixtures and an in-memory schema-14 store. Run them from the repository root with the required Node.js version:

```bash
npm --prefix tools/codebase-map run test:portable
```

Run all checks, including the [live-source checks](test/map.test.mjs), with:

```bash
npm --prefix tools/codebase-map run check
```

`npm --prefix tools/codebase-map test` invokes the same check script. The live-source checks read the current Git inventory, compatible Codify index and active specification, generate a fresh snapshot in a temporary directory, and serve it to the consumer. They do not require a previously generated `data/codebase-map.json`.

## Configuration

The server reads these environment variables:

- `LAYERX_CODE_MAP_PORT` — listening port.
- `LAYERX_CODE_MAP_HOST` — listening interface; the server binds to loopback by default.

The generator reads `[meta] active_feature` in [`spec/workflow.kvx`](../../spec/workflow.kvx) to select the requirements and tasks from [`spec/layerx-merge/spec.kvx`](../../spec/layerx-merge/spec.kvx). It resolves that setting on each run and records the selected path in `meta.sources`. Missing configuration, an invalid feature name or a missing authoritative specification produces an explicit error. An existing snapshot does not substitute for these inputs.

The generator also reads `.codegraph/graph.db` and the package manifests in the file inventory to determine package names and scopes. The browser loads `data/codebase-map.json`.

## Documentation

- [Release readiness matrix](../../docs/release/7.0.0-readiness-matrix.md) — lists this directory's generation and test entry points.
- [Repository tools](../README.md) — index of tools in the checkout.
