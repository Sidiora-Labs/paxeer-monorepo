# Chainflow Data Gateway SDKs

This directory holds Go, Python, Rust and TypeScript clients for the Chainflow Data Gateway. They provide REST access to price feeds, market summaries, TradingView data, explorer observations and CoinGecko/CoinMarketCap-compatible adapters, with WebSocket support where the language client provides it. Applications use these clients for data alongside Paxeer node access. Paxeer mainnet-beta runs `paxd` with EVM chain ID 125 (`0x7d`); the LayerX kernel inside `paxd` executes and orders each transaction and builds each block; validators check and commit it. Receipts become final when their block is committed. Configure node JSON-RPC separately from the data gateway.

## Layout

- [`go/`](go/) — Go client, request options, API reference, tests and a consumer module linked by its Go workspace.
- [`python/`](python/) — synchronous and asynchronous Python clients, response models, WebSocket helpers and tests.
- [`rust/`](rust/) — asynchronous Rust client, transport interfaces, resource builders, examples and tests.
- [`typescript/`](typescript/) — TypeScript and JavaScript client, Node.js and browser WebSocket transports, build scripts and smoke tests.
- [`README.md`](README.md) — directory overview and links to the language packages.

## Build and test

Each language package has its own manifest. Run the following commands in the directory shown, after installing its declared dependencies.

| Directory | Build or install | Tests or static checks |
| --- | --- | --- |
| [`go/`](go/go.mod) | `go build ./...` | `go test ./...` |
| [`python/`](python/pyproject.toml) | `python -m pip install '.[websockets]'` | `python -m pytest tests` |
| [`rust/`](rust/Cargo.toml) | `cargo build --all-features` | `cargo test --all-features` |
| [`typescript/`](typescript/package.json) | `npm run build` | `npm run typecheck` |

The Python manifest selects Hatchling as the package build backend. Install `pytest` in the Python environment to run its tests; the `websockets` extra supplies the WebSocket transport used by the tests. The Go package uses [`go.work`](go/go.work) to include its local consumer module. Rust's [SDK workflow](rust/.github/workflows/sdk-ci.yml) declares the build and test commands above; all features include the feature-gated transport tests. The TypeScript manifest builds ES modules and CommonJS output using [`tsconfig.json`](typescript/tsconfig.json), [`tsconfig.cjs.json`](typescript/tsconfig.cjs.json) and [`scripts/finalize-build.mjs`](typescript/scripts/finalize-build.mjs).

The generated smoke scripts also declare `go run tests/smoke-test.go` from `go/`, `python tests/smoke-test.py` from `python/`, and `bun tests/smoke-test.ts` from `typescript/`. These scripts call gateway operations and need a configured gateway and operation credentials. The TypeScript smoke script uses the built package exports. Rust's [`tests/smoke.rs`](rust/tests/smoke.rs) sends requests when `SIDIORALABS_BASE_URL` is configured.

## Configuration

Client constructors, request options and builders accept explicit configuration. Environment-based configuration is read by the [Go defaults](go/client.go), [Python clients](python/src/chainflow_data_gateway/_client.py), [Rust `from_env`](rust/src/client.rs) and [TypeScript client](typescript/src/client.ts):

| Environment variable | Purpose | Clients |
| --- | --- | --- |
| `REST_API_KEY` | Authentication for REST data reads | Go, Python, Rust, TypeScript |
| `WS_MINT_KEY` | Authentication for WebSocket token minting | Go, Python, Rust, TypeScript |
| `WS_TOKEN` | Token for WebSocket upgrades | Go, Python, Rust, TypeScript |
| `SIDIORALABS_BASE_URL` | Data gateway base URL | Go, Python, Rust, TypeScript |
| `SIDIORALABS_CUSTOM_HEADERS` | Additional HTTP headers | Go, Python, TypeScript |
| `SIDIORALABS_LOG` | Client logging | Python, TypeScript |
| `DEFER_PYDANTIC_BUILD` | Deferred response-model construction | Python |

Smoke scripts read `SCALAR_SMOKE_FILTER` and `SCALAR_SMOKE_REPORT`; the Go, Python and TypeScript scripts also read `SCALAR_SMOKE_CONCURRENCY`. Runtime configuration is supplied through the client APIs and environment variables. Package manifests and the compiler and workspace files linked above configure builds.

## Documentation

- Language guides: [Go](go/README.md), [Python](python/README.md), [Rust](rust/README.md) and [TypeScript](typescript/README.md).
- API references: [Go](go/api.md), [Python](python/api.md), [Rust](rust/api.md) and [TypeScript](typescript/api.md).
- [Release project matrix](../../docs/release/7.0.0-readiness-matrix.md) lists the Chainflow language packages.
- [Perps data API](../../docs/perps/data-api.md) describes how chart data uses the external Chainflow feed.
- [Public RPC documentation site page](../../docs/site/paxeer-docs/src/app/public-rpc/page.tsx) describes node access alongside gateway data access.
