# Codebase map

An offline, navigable snapshot of the Paxeer X Network checkout. The map combines the Git file inventory and working-tree state with Codify symbols, resolved calls, imports and routes from `.codegraph/graph.db`, and requirements and task dependencies from the active specification.

The active requirements and tasks live in [`spec/layerx-merge/spec.kvx`](../../spec/layerx-merge/spec.kvx), selected by `[meta] active_feature` in [`spec/workflow.kvx`](../../spec/workflow.kvx). The generator resolves that setting on each run and records the selected spec path in `meta.sources`. A missing configuration, invalid feature name or missing authoritative specification is an explicit error; an older snapshot does not substitute for the source.

## Generation prerequisites

- Node.js 22.5 or later, with the built-in `node:sqlite` module, and Git available on `PATH`.
- A compatible Codify index at `.codegraph/graph.db`. For a new checkout, initialize it with `cg init`; refresh an existing compatible index with `cg sync` before generation.
- Graph schema version **14**, enforced by `GRAPH_SCHEMA_VERSION`. A different database schema produces an `Unsupported Codify schema …; expected 14.` error. Both `meta.codifySchemaVersion` and `meta.sources` identify the accepted schema. If your Codify version creates another schema, a compatible index/tool version is required; do not relabel the database's version to bypass the check.

From the repository root, with those prerequisites satisfied:

```bash
cg sync
npm --prefix tools/codebase-map run generate
npm --prefix tools/codebase-map run serve
```

Open `http://localhost:4177`. Generation writes `tools/codebase-map/data/codebase-map.json`, which is intentionally ignored by Git. The [server](serve.mjs) serves the existing snapshot; it does not generate or refresh it. Without that file, the viewer cannot load. Regenerate after source or spec changes to refresh the view.

Set `LAYERX_CODE_MAP_PORT` to use another port (default `4177`). The server binds to `127.0.0.1` unless `LAYERX_CODE_MAP_HOST` is explicitly set.

## What the map proves

- Every Git-tracked path is present in the file inventory, including paths excluded from semantic indexing.
- Indexed files carry the database's language, line count, hash, symbol counts and reference counts. These describe the last index; refresh it before generating a snapshot of changed source.
- Internal call edges exist only where Codify records a concrete `target_id`.
- Internal import edges exist only where Codify records a concrete `target_file_id`.
- Package views are manifest scopes: a file is grouped under its nearest ancestor manifest, and scope-to-scope edges are aggregated from resolved file edges.
- External, receiver-only, ambiguous and unresolved references remain visible as coverage counts; the viewer does not invent targets for them.
- Delivery data contains tasks, statuses, waves, requirements, declared symbols, touch paths and dependencies from the selected specification. Decision, policy and other KVX sections cannot overwrite adjacent tasks or requirements.
- The header displays the Git commit, branch, dirty-file indicator and generation time. Snapshot metadata also records the graph database timestamp and Codify schema.

The graph renders static source relationships. It does not establish runtime reachability, deployment state or task qualification. The coverage strip and each file inspector distinguish indexed files from files known only through Git.

## Navigation

- Start in **Systems**, then double-click an area, package or file to drill toward symbols.
- Use **Imports** to see unresolved/external modules globally or the exact imports for a selected file.
- Use **Routes** for detected framework entry points.
- **Delivery** starts with a wave overview, including aggregate dependencies between waves. Click a wave in the inventory or graph to expand it. Wave pages show at most 80 waves; task pages show at most 160 tasks in a fixed grid. Use **Previous page** and **Next page** to browse, and **All** to return to the overview. Dependency edges within a task page are drawn; the inspector retains prerequisites and dependents across pages and waves.
- Search reaches all tasks, files and symbols, including items outside the visible page. Choosing a task opens its wave and the page containing it. Enter at least two characters; results are capped at 120 matches, so narrow a broad query with a task ID or title.
- Press `/` to search, `F` to fit, `Escape` to clear and `Left Arrow` to return to the previous scope.

The focused [repair checks](test/repair-22-33-799.test.mjs) run the production snapshot producer against an in-memory schema-14 store and recorded Git inventory, serve its output over loopback HTTP with the production server, and load it through the Delivery consumer. They need no generated snapshot, live index, external toolchain or credentials:

```bash
npm --prefix tools/codebase-map install --no-audit --no-fund
node --test tools/codebase-map/test/repair-22-33-799.test.mjs
```

For the existing snapshot-dependent [checks](test/map.test.mjs), generate a fresh map first, then run `npm --prefix tools/codebase-map run check`. Those checks skip snapshot-dependent cases if `data/codebase-map.json` is absent. Their live-source comparisons require the same Git commit, compatible index and `cg spec status --json` inputs used to generate the snapshot.
