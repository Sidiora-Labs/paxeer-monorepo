# Paxeer X Network Explorer

This directory holds the Blockscout-based explorer backend, supporting Rust services, and deployment configuration for Paxeer mainnet-beta. The frontend lives in [`apps/explorer`](../../apps/explorer/). These components run as independent processes and communicate over HTTP; the backend indexes the single BFT network running `paxd`, with EVM chain id **125** (`0x7d`). The kernel is part of the network and proposes blocks, and a transaction receipt is final once its block is committed.

## Layout

| Path | Contents |
| --- | --- |
| [`backend/`](backend/) | Elixir umbrella containing the explorer database, indexer, JSON-RPC client, web APIs, utilities, and NFT media handler. |
| [`deploy/`](deploy/) | Local Compose definitions, environment templates, Railway service definitions, database tools, and deployment checks. |
| [`services/`](services/) | Rust `smart-contract-verifier` and `sig-provider` workspaces, plus the shared `libs` workspace. |
| [`tests/`](tests/) | Python checks for the environment template, OpenAPI block schema, and backend documentation. |
| [`swagger.yaml`](swagger.yaml) | Symlink to the frontend's [`public/openapi.yaml`](../../apps/explorer/public/openapi.yaml) API schema. |
| [`LICENSE`](LICENSE) | GPL-3.0 licence text. |
| [`README.md`](README.md) | Component layout, commands, configuration, and documentation. |

The backend exposes indexed REST APIs under `/api/v2` and account features under `/api/account/v2`. Public node JSON-RPC uses HTTPS at the configured node's root path, with namespaces `debug`, `echo`, `eth`, `net`, `pax`, `test`, and `txpool`. The hosted gateway's `px_` methods have their own [OpenRPC contract](../platform/hosted/gateway/openrpc.json). Sub-chains are tenant domains in the database, launchable without new infrastructure.

The backend and frontend retain their GPL-3.0 licences ([backend](backend/LICENSE), [frontend](../../apps/explorer/LICENSE)); the Rust services retain their [MIT licence](services/LICENSE-MIT). Their binaries are built separately from the Apache-2.0 node and agent packages, and the explorer reaches the node over JSON-RPC.

## Build and test

Run the commands below from the monorepo root. The backend toolchain is pinned in [`backend/.tool-versions`](backend/.tool-versions); the frontend declares its Node version in [`.nvmrc`](../../apps/explorer/.nvmrc) and Yarn version in [`package.json`](../../apps/explorer/package.json). Rust builds use the manifests and lockfiles of the individual workspaces.

### Backend

The [backend builder](deploy/tools/mix-in-builder.sh) accepts Mix arguments and runs them inside the pinned Elixir builder image:

```sh
services/explorer/deploy/tools/mix-in-builder.sh compile
services/explorer/deploy/tools/mix-in-builder.sh format --check-formatted
services/explorer/deploy/tools/mix-in-builder.sh test apps/explorer/test/explorer
```

It mounts the backend applications, configuration, release files, manifests, and repository precompiles. Compiled artifacts stay in a named volume. Each invocation starts disposable PostgreSQL and Redis sidecars, shares their network namespace with the Mix container, and waits for readiness. The script installs the headless browser driver when the selected tests need `block_scout_web`, redacts the database password in its printed command, and returns the Mix exit code. Sidecars and their anonymous volumes are removed on exit unless reuse is selected.

The image is built on demand from the [builder Dockerfile](../../docker/explorer-elixir-builder/Dockerfile). Its input digest covers that Dockerfile, `mix.lock`, and the application manifests; a missing image or changed inputs trigger a rebuild. An image without the digest, including an externally built override, is reported as having unknown freshness and is retained.

Options precede the Mix arguments; `--` ends option parsing.

| Option | Behaviour |
| --- | --- |
| `--reuse-db` | Reuse PostgreSQL and Redis sidecars from an earlier invocation with this option and leave them running. |
| `--image <ref>` | Select another builder image. |
| `--check` | Resolve the runtime, image, and mounts and print the command without starting containers. |
| `-h`, `--help` | Print usage and environment variable names. |

The direct dependency and compile command used by the [build workflow](../../.github/workflows/explorer-build.yml) is:

```sh
(cd services/explorer/backend && mix deps.get && mix compile)
```

### Frontend and Rust services

The frontend builds independently. Its dependency installation disables the imported hook preparation step:

```sh
(cd apps/explorer && corepack enable && HUSKY=0 yarn install --frozen-lockfile && yarn build)
```

The Rust image Dockerfiles build the service workspaces with `cargo build --release --locked` ([verifier](../../docker/explorer-smart-contract-verifier/Dockerfile), [signature provider](../../docker/explorer-sig-provider/Dockerfile)). To build and test each workspace using the command in the build workflow:

```sh
(cd services/explorer/services/libs && cargo test --locked --workspace)
(cd services/explorer/services/smart-contract-verifier && cargo test --locked --workspace)
(cd services/explorer/services/sig-provider && cargo test --locked --workspace)
```

Database-backed Rust tests read `DATABASE_URL`. The workflow installs native build dependencies, the protocol-buffer compiler, and `protoc-gen-openapiv2` before testing.

### Combined test and lint scripts

```sh
scripts/explorer/gate-test.sh
scripts/explorer/gate-lint.sh
```

The test script discovers backend `*_test.exs` files whose paths contain `paxeer_x`, runs them through the builder once per umbrella application, then installs the frontend dependencies and runs `yarn lint:tsc` and `yarn test:vitest:ci`. It finishes with `cargo test --locked --workspace` in `libs`, `smart-contract-verifier`, and `sig-provider`. Frontend installation uses `HUSKY=0`.

The lint script runs the following scripts in order; each is also callable independently:

```sh
scripts/explorer/lint-backend.sh
scripts/explorer/lint-frontend.sh
scripts/explorer/lint-services.sh
```

Backend lint runs `mix format --check-formatted` and `mix credo --strict` through the builder. Both checks execute, and the script returns the first nonzero result. Frontend lint installs the frozen lockfile with `HUSKY=0`, then runs `yarn lint:eslint` and `yarn lint:tsc`. Service lint runs `cargo fmt --all --check` and `cargo clippy --all-targets --locked -- -D warnings` in each Rust service, installing the protocol-buffer compiler and OpenAPI plugin when absent.

Each combined script stops on the first unsuccessful leg and reports its command and exit code. A leg that exhausts its configured time budget terminates with exit code `124`. Both scripts support toolchain resolution without running suites:

```sh
scripts/explorer/gate-test.sh --check
scripts/explorer/gate-lint.sh --check
```

These checks resolve the container runtime, builder recipe, Node toolchain, and lint scripts. The test script also resolves Cargo and all three Rust workspace manifests and lockfiles.

The [test-ratio script](../../scripts/explorer/test-ratio.sh) measures a Git range:

```sh
scripts/explorer/test-ratio.sh origin/main...HEAD
scripts/explorer/test-ratio.sh HEAD~1..HEAD
scripts/explorer/test-ratio.sh origin/main HEAD
```

It requires at least as many changed referencing test files as changed source files under `apps/explorer/` and `services/explorer/`, and a referencing changed test for every changed source. References may use an Elixir module, an exported name, a path fragment, or the corresponding test location. Elixir `.ex`, non-test TypeScript `.ts` and `.tsx`, and Rust files with changed production items count as source. Elixir `*_test.exs`, TypeScript test and spec files (including `*.pw.tsx`), Rust files under `tests/`, and changed Rust test items count as tests. Mixed Rust files can count as both; unchanged inline tests do not count for a production change. Rust classification compares base and head blobs through [`rust-test-ranges.py`](../../scripts/explorer/rust-test-ranges.py) using Python 3. Shell, YAML, JSON, environment templates, SVG, and Markdown count as neither.

The ratio script returns `0` when the ratio holds or no explorer source changes, `1` when coverage is insufficient, and `2` when arguments or the repository do not resolve. Its [shell test](../../tests/explorer/test-ratio-test.sh) covers satisfied and unsatisfied ratios, unrelated tests, shell and documentation changes, and an empty range.

### Local stack

The [local Compose definition](deploy/docker-compose.local.yml) runs PostgreSQL, the backend, frontend, smart contract verifier, and signature provider using published images. With the image tag, node endpoints, chain identity, and signing secret supplied through the environment:

```sh
docker compose -f services/explorer/deploy/docker-compose.local.yml config
docker compose -f services/explorer/deploy/docker-compose.local.yml up
```

The backend and frontend have health checks. Empty chain identity or signing secret settings prevent startup. The `fixture` profile adds a recorded JSON-RPC server from [`deploy/tools/rpc-fixture-server.py`](deploy/tools/rpc-fixture-server.py), serving [`deploy/tools/fixtures/`](deploy/tools/fixtures/). The [Compose smoke test](deploy/tools/tests/compose-smoke-test.sh) runs the database, recorded server, and backend together.

[Railway configuration](deploy/railway/README.md) describes each service's image and configuration file selection. The [database copy tool](deploy/tools/copy-blockscout-11-to-10.sh) copies core chain tables between the schemas named in that script by intersecting their column names. It reads source and destination connection strings from the environment and supports a dry run that reports the column plan.

## Configuration

### Backend and local deployment

The backend reads [`backend/config/config.exs`](backend/config/config.exs), the application configuration under [`backend/apps/`](backend/apps/), and environment-specific files under [`backend/config/`](backend/config/). Runtime settings are read in [`backend/config/runtime.exs`](backend/config/runtime.exs) and [`backend/config/runtime/`](backend/config/runtime/). The [RPC configuration helper](backend/config/config_helper.exs) reads transport and endpoint settings.

| Purpose | Environment variables |
| --- | --- |
| Chain identity and adapter | `CHAIN_TYPE`, `CHAIN_ID`, `ETHEREUM_JSONRPC_VARIANT`, `PAXEER_X_COSMOS_TRANSACTION_TYPE`, `PAXEER_X_NATIVE_COIN_DECIMALS` |
| Node transport | `ETHEREUM_JSONRPC_TRANSPORT`, `ETHEREUM_JSONRPC_HTTP_URL`, `ETHEREUM_JSONRPC_TRACE_URL`, `ETHEREUM_JSONRPC_ETH_CALL_URL`, `ETHEREUM_JSONRPC_WS_URL` |
| Database and web endpoint | `DATABASE_URL`, `TEST_DATABASE_URL`, `POOL_SIZE`, `POOL_SIZE_API`, `ECTO_USE_SSL`, `SECRET_KEY_BASE`, `PORT` |
| Receipt authority and capability discovery | `PAXEER_X_RECEIPT_AUTHORITY_FILE`, `PAXEER_X_NODE_RPC_URL`, `PAXEER_X_GATEWAY_URL`, `PAXEER_X_CAPABILITIES_ENABLED`, `PAXEER_X_CAPABILITIES_REFRESH_SECONDS` |
| Price-feed indexing | `PAXEER_X_PRICE_FEED_ENABLED`, `PAXEER_X_COSMOS_REST_URLS`, `PAXEER_X_COMETBFT_RPC_URLS`, `PAXEER_X_PRICE_FEED_START_BLOCK`, `PAXEER_X_PRICE_FEED_CONCURRENCY`, `PAXEER_X_PRICE_FEED_MARKETS_REFRESH_SECONDS`, `PAXEER_X_PRICE_FEED_PRECOMPILE`, `PAXEER_X_PRICE_FEED_RETENTION_DAYS` |
| Service clients | `MICROSERVICE_SC_VERIFIER_ENABLED`, `MICROSERVICE_SC_VERIFIER_URL`, `MICROSERVICE_SIG_PROVIDER_ENABLED`, `MICROSERVICE_SIG_PROVIDER_URL` |
| Compose image and node selection | `EXPLORER_IMAGE_TAG`, `RPC_HTTP_URL`, `RPC_WS_URL` |
| Compose publication and database | `EXPLORER_PUBLIC_HOST`, `BACKEND_PUBLISHED_PORT`, `FRONTEND_PUBLISHED_PORT`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_PUBLISHED_PORT` |

The receipt authority file is JSON and is loaded from the path named by `PAXEER_X_RECEIPT_AUTHORITY_FILE`. The local Compose file reads [`deploy/env/backend.example.env`](deploy/env/backend.example.env) and [`deploy/env/frontend.example.env`](deploy/env/frontend.example.env). It maps `RPC_HTTP_URL` to the backend HTTP, trace, and contract-call endpoints, and `RPC_WS_URL` to the subscription endpoint. It sets browser-facing hosts and ports explicitly and passes `CHAIN_ID` to `NEXT_PUBLIC_NETWORK_ID`.

Frontend configuration names include `NEXT_PUBLIC_API_HOST`, `NEXT_PUBLIC_API_PORT`, `NEXT_PUBLIC_APP_HOST`, `NEXT_PUBLIC_APP_PORT`, `NEXT_PUBLIC_NETWORK_ID`, and `NEXT_PUBLIC_PAXEER_X_ENABLED`. Additional frontend names are documented in the [environment catalogue](../../docs/explorer/ENVS.md); backend templates are in [`backend/docker-compose/envs/`](backend/docker-compose/envs/).

### Rust services

The verifier loads a file selected by `SMART_CONTRACT_VERIFIER__CONFIG`; the signature provider loads one selected by `SIG_PROVIDER__CONFIG`. Each service applies environment overrides with its service prefix and double underscores between settings. Sample files are [`smart-contract-verifier-server/config/base.toml`](services/smart-contract-verifier/smart-contract-verifier-server/config/base.toml), [`smart-contract-verifier-server/config/base.env`](services/smart-contract-verifier/smart-contract-verifier-server/config/base.env), and [`sig-provider-server/config/base.toml`](services/sig-provider/sig-provider-server/config/base.toml). Samples are not loaded automatically.

The local deployment uses `SMART_CONTRACT_VERIFIER__SERVER__HTTP__ENABLED`, `SMART_CONTRACT_VERIFIER__SERVER__HTTP__ADDR`, `SMART_CONTRACT_VERIFIER__SERVER__GRPC__ENABLED`, `SMART_CONTRACT_VERIFIER__SOLIDITY__ENABLED`, `SMART_CONTRACT_VERIFIER__SOLIDITY__COMPILERS_DIR`, `SMART_CONTRACT_VERIFIER__VYPER__ENABLED`, `SMART_CONTRACT_VERIFIER__VYPER__COMPILERS_DIR`, and `SMART_CONTRACT_VERIFIER__SOURCIFY__ENABLED`. Signature-provider listener settings use `SIG_PROVIDER__SERVER__HTTP__ENABLED`, `SIG_PROVIDER__SERVER__HTTP__ADDR`, and `SIG_PROVIDER__SERVER__GRPC__ENABLED`.

### Command helpers

| Helper | Environment variables |
| --- | --- |
| Backend builder | `MIX_ENV`, `MIX_BUILD_PATH`, `MIX_BUILD_VOLUME`, `CHAIN_TYPE`, `ETHEREUM_JSONRPC_VARIANT`, `PGUSER`, `PGPASSWORD`, `MIX_IN_BUILDER_IMAGE`, `MIX_IN_BUILDER_BROWSER_DRIVER` |
| Combined test and lint scripts | `EXPLORER_GATE_BUDGET_SECONDS`, `EXPLORER_GATE_LOG_DIR` |
| Database copy tool | `SRC_DATABASE_URL`, `DST_DATABASE_URL`, `TABLES`, `BLOCK_BATCH`, `DRY_RUN`, `ALLOW_NONEMPTY_DESTINATION` |

## Documentation

- [Explorer components and licences](../../docs/explorer/UPSTREAM.md).
- [Frontend environment variables](../../docs/explorer/ENVS.md), [build-time variables](../../docs/explorer/BUILD-TIME_ENVS.md), and [deprecated variables](../../docs/explorer/DEPRECATED_ENVS.md).
- [Custom frontend builds](../../docs/explorer/CUSTOM_BUILD.md) and [contribution guide](../../docs/explorer/CONTRIBUTING.md).
- [Documentation-site guide to querying the EVM](../../docs/site/paxeer-docs/public/markdown/querying-evm.md).
- [Backend overview](backend/README.md#paxeer-x-additions) and [account API blueprint](backend/apps/block_scout_web/API%20blueprint.md).
- [Smart contract verifier](services/smart-contract-verifier/smart-contract-verifier-server/README.md) and [signature provider](services/sig-provider/README.md).
- [Deployment checks](deploy/README.md) and [Railway service definitions](deploy/railway/README.md).
