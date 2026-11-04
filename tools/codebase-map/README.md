# Codebase map

An offline, navigable snapshot of the Paxeer X Network checkout. The map combines the Git file inventory and working-tree state with Codify symbols, resolved calls, imports and routes from `.codegraph/graph.db`, and requirements and task dependencies from a specification.

The merged network's active requirements and tasks live in [`spec/layerx-merge/spec.kvx`](../../spec/layerx-merge/spec.kvx), selected by `active_feature` in [`spec/workflow.kvx`](../../spec/workflow.kvx). Use those files as the source of current work.

## Generation prerequisites

The checked-in [generator](generate.mjs) still hardcodes the absent `spec/layerx-platform/spec.kvx`; it does not read `active_feature` or offer a spec-path environment override. With a graph database present, generation stops with `Missing authoritative spec/layerx-platform/spec.kvx.` before writing a snapshot. Generation requires a generator migration to the active specification above. Changing the workflow setting alone cannot resolve this blocker, and an existing map is not evidence that the active programme was loaded.

The generator also requires:

- Node.js 22.5 or later, with the built-in `node:sqlite` module, and Git available on `PATH`.
- A Codify index at `.codegraph/graph.db`, created with `cg init` and refreshed with `cg sync`.
- Graph schema version **14**, enforced by `GRAPH_SCHEMA_VERSION`. A different database schema produces an `Unsupported Codify schema …; expected 14.` error. The output's `meta.codifySchemaVersion` records the accepted schema; the hardcoded `meta.sources` label currently says schema 10 and is stale.

Once the generator reads the active spec and these prerequisites are satisfied, the package commands from the repository root are:

```bash
cg sync
npm --prefix tools/codebase-map run generate
npm --prefix tools/codebase-map run serve
```

Open `http://localhost:4177`. Successful generation writes `tools/codebase-map/data/codebase-map.json`, which is intentionally ignored by Git. The [server](serve.mjs) serves the existing snapshot; it does not generate or refresh it. Without that file, the viewer cannot load.

Set `LAYERX_CODE_MAP_PORT` to use another port (default `4177`). The server binds to `127.0.0.1` unless `LAYERX_CODE_MAP_HOST` is explicitly set.

## What the map proves

- Every Git-tracked path is present in the file inventory, including paths excluded from semantic indexing.
- Indexed files carry the database's language, line count, hash, symbol counts and reference counts. These describe the last index; refresh it before generating a snapshot of changed source.
- Internal call edges exist only where Codify records a concrete `target_id`.
- Internal import edges exist only where Codify records a concrete `target_file_id`.
- Package views are manifest scopes: a file is grouped under its nearest ancestor manifest, and scope-to-scope edges are aggregated from resolved file edges.
- External, receiver-only, ambiguous and unresolved references remain visible as coverage counts; the viewer does not invent targets for them.
- Delivery data contains parsed tasks, statuses, waves, requirements, declared symbols, touch paths and dependency edges from the generator's spec source. Until the source migration above is implemented, it cannot represent the active `layerx-merge` programme on a clean checkout.
- The header displays the Git commit, branch, dirty-file indicator and generation time. Snapshot metadata also records the graph database timestamp and Codify schema.

The graph renders static source relationships. It does not establish runtime reachability, deployment state or task qualification. The coverage strip and each file inspector distinguish indexed files from files known only through Git.

## Navigation

- Start in **Systems**, then double-click an area, package or file to drill toward symbols.
- Use **Imports** to see unresolved/external modules globally or the exact imports for a selected file.
- Use **Routes** for detected framework entry points and **Delivery** for task dependencies.
- Search scans every file and symbol even when a dense graph view limits the nodes drawn. Enter at least two characters; results are capped at 120 matches.
- Press `/` to search, `F` to fit, `Escape` to clear and `Left Arrow` to return to the previous scope.

After successful generation, the package's [checks](test/map.test.mjs) cover parsing, snapshot relationships, live source counts and browser rendering:

```bash
npm --prefix tools/codebase-map run check
```

The `check` script runs `node --test test/*.test.mjs`. Snapshot-dependent tests currently skip when `data/codebase-map.json` is absent, so a passing check without generated data does not prove generation or snapshot consistency. The live-source checks compare against the current Git commit, index and `cg spec status --json`; use a freshly generated snapshot from those same sources.
