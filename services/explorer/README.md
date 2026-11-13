# Paxeer X Network Explorer

The Paxeer X Network block explorer is a fork of Blockscout. Its backend and
Rust services live here; its frontend lives in [`apps/explorer`](../../apps/explorer).
The components run as independent processes and communicate over HTTP.

## Network and interfaces

The explorer indexes Paxeer mainnet-beta, one BFT network running `paxd` with
EVM chain id **125** (`0x7d`). The kernel is part of this network and proposes
blocks. A transaction receipt is final once its block is committed.

Public node JSON-RPC uses the root URL, for example
`https://api1.mainnet-beta.paxeer.network/`. The public node names are
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`
and `api-mainnet-beta.paxeer.network`. The node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `px`, `test` and `txpool`. The node registers `px` account,
balance, receipt and consensus reads at the root; asset and history reads use
its configured indexer. `px_register` and `px_requestFunds` require hosted
services. The gateway's methods have their own
[OpenRPC contract](../platform/hosted/gateway/openrpc.json).
`node.hyperpaxeer.com` is only the hpx library mirror, not an RPC endpoint.

The explorer's indexed REST API is served by its backend under `/api/v2`;
account features use `/api/account/v2`. These are explorer HTTP APIs, separate
from node JSON-RPC. See the [backend API overview](backend/README.md#paxeer-x-additions)
and the [account API blueprint](backend/apps/block_scout_web/API%20blueprint.md).
Sub-chains are tenant domains in the database, launchable without new
infrastructure; they do not imply another network or a new node process.

## Components

Paths in this table are relative to the monorepo root. Revisions identify the
upstream imports; the fork includes local changes.

| Part | Upstream | Revision |
| --- | --- | --- |
| [`services/explorer/backend/`](backend/) | blockscout/blockscout | tag `v10.2.6`, `90f7dd8e9348123b74dfd23f69ab7da76191a820` |
| [`apps/explorer/`](../../apps/explorer/) | blockscout/frontend | tag `v2.7.2`, `446c409eeb54274aab90ff371a705f9699cd84ef` |
| [`services/explorer/services/`](services/) | blockscout/blockscout-rs | `934a80f42976bac8e13dc45770104a0ede8bd7d7` |

The Rust tree contains `smart-contract-verifier`, `sig-provider` and the shared
`libs` workspace.

## Licence boundary

The explorer backend is GPL-3.0 ([root licence](LICENSE),
[backend licence](backend/LICENSE)); the frontend retains its GPL-3.0
[licence in `apps/explorer`](../../apps/explorer/LICENSE). The imported Rust
services retain the [MIT licence](services/LICENSE-MIT). The explorer runs as
its own set of processes and reaches the node over JSON-RPC. Its backend,
frontend and service binaries are built separately from the Apache-2.0 node
and agent packages.

## Building

Run these from the monorepo root, using a subshell for each component:

```sh
(cd services/explorer/backend && mix deps.get && mix compile)
(cd apps/explorer && corepack enable && HUSKY=0 yarn install --frozen-lockfile && yarn build)
(cd services/explorer/services/smart-contract-verifier && cargo check --locked)
(cd services/explorer/services/sig-provider && cargo check --locked)
```

Use the Elixir and Erlang versions in
[`backend/.tool-versions`](backend/.tool-versions), and the Node version in
[`apps/explorer/.nvmrc`](../../apps/explorer/.nvmrc). The frontend pins Yarn in
[`package.json`](../../apps/explorer/package.json); `HUSKY=0` prevents the
imported prepare step from changing repository hooks. Both Rust workspaces
resolve shared crates from `services/explorer/services/libs`.

[`docs/explorer/UPSTREAM.md`](../../docs/explorer/UPSTREAM.md) records the exact upstream revisions. Keep local
changes to the imported trees small so the fork can still be rebased onto
later upstream releases.

## Running backend commands

`deploy/tools/mix-in-builder.sh` runs one mix command against `backend/` inside
the pinned Elixir builder image, so a compile, a format check and a test suite
behave the same wherever they run. Call it from the repository root:

```sh
services/explorer/deploy/tools/mix-in-builder.sh compile
services/explorer/deploy/tools/mix-in-builder.sh format --check-formatted
services/explorer/deploy/tools/mix-in-builder.sh test apps/explorer/test/explorer
```

It mounts the backend
`apps`, `config`, `rel`, `mix.exs`, `mix.lock`, `README.md`, `.formatter.exs` and
`.credo.exs`, plus the repository precompiles, into the container. It keeps the
compiled artefacts in a named volume, starts disposable PostgreSQL
16 and Redis 7.4 sidecars, and joins them and the mix container to the same
network namespace. It waits for both services before running Mix and installs the
headless browser driver when the test selection needs `block_scout_web`. It
prints the command with the database password redacted and exits with the Mix
exit code. Sidecars and their anonymous volumes are removed on exit unless
`--reuse-db` is selected.

The builder image is built on demand from
[`docker/explorer-elixir-builder/Dockerfile`](../../docker/explorer-elixir-builder/Dockerfile)
at the repository root, which pins the Elixir and Erlang versions the explorer
build workflow uses. The script labels the image it
builds with the digest of that Dockerfile, `mix.lock` and the application
manifests, and builds again when the image is absent or when one of those
inputs has moved since. An image without that digest, including an externally
built `--image` override, is reported as having unknown freshness rather than
being overwritten.

Options come before the mix arguments; `--` ends them.

| Option | What it does |
| --- | --- |
| `--reuse-db` | Reuse the PostgreSQL and Redis sidecars from a previous `--reuse-db` run and keep them running at exit |
| `--image <ref>` | run a different builder image reference |
| `--check` | resolve the container runtime, the image and the mounts, print the command that would run, and exit without starting anything |
| `-h`, `--help` | print the usage, including every variable below |

Without `--reuse-db`, each invocation starts fresh database and Redis sidecars.

| Variable | Default |
| --- | --- |
| `MIX_ENV` | `test` |
| `MIX_BUILD_PATH` | `/build`, where the named volume is mounted |
| `MIX_BUILD_VOLUME` | a name derived from the backend path, so two working trees never share one build |
| `CHAIN_TYPE` | `paxeer_x` |
| `ETHEREUM_JSONRPC_VARIANT` | `paxeer_x` |
| `PGUSER`, `PGPASSWORD` | Local test database settings from `backend/apps/explorer/config/test.exs` |
| `MIX_IN_BUILDER_IMAGE` | the same override as `--image` |
| `MIX_IN_BUILDER_BROWSER_DRIVER` | unset, which lets the script decide from the mix arguments; `1` always installs the driver, `0` never does |

## Explorer development gates

The explorer provides two scripts, callable from the repository root:

```sh
scripts/explorer/gate-test.sh
scripts/explorer/gate-lint.sh
```

`scripts/explorer/gate-test.sh` runs the backend Paxeer X suites - every
`*_test.exs` under `backend/apps` whose path carries `paxeer_x` - through
`deploy/tools/mix-in-builder.sh`, one invocation per umbrella application, so
each application gets the pinned builder image, a database sidecar and a
virtual machine of its own, which is what its test helper configures; it then
runs the frontend dependency install, `yarn lint:tsc` and `yarn test:vitest:ci`
from `apps/explorer/`, followed by `cargo test --locked --workspace` in each of
`libs`, `smart-contract-verifier` and `sig-provider`. Rust database tests use
`DATABASE_URL`. The frontend install uses `HUSKY=0`.

`scripts/explorer/gate-lint.sh` runs `scripts/explorer/lint-backend.sh`,
`scripts/explorer/lint-frontend.sh` and `scripts/explorer/lint-services.sh` in that
order, which is what the `explorer-lint` job runs.

Each gate stops on the first leg that fails and reports the leg, the command it
ran, its exit code and its log path rather than starting the next leg; the exit
code of the gate is the exit code of that leg.

| Variable | What it does |
| --- | --- |
| `EXPLORER_GATE_BUDGET_SECONDS` | seconds allowed per leg, `1500` by default; a leg that exhausts it is terminated, reported as a timeout and stops the gate with exit code `124` |
| `EXPLORER_GATE_LOG_DIR` | directory the logs are written to; the default is `build/explorer-gates/` under the repository root, which the gate prints on every run |

Both gates take `--check`, which validates the gate script itself, the
container runtime, the builder image recipe, the Node toolchain and the three
lint scripts, prints the log directory and exits without running a suite or a
check. This validates toolchain resolution; it does not establish that any
suite passes. The test script also resolves Cargo and all three Rust workspace
manifests and lockfiles:

```sh
scripts/explorer/gate-lint.sh --check
scripts/explorer/gate-test.sh --check
```

## Running the whole stack locally

`deploy/docker-compose.local.yml` brings up Postgres, the backend, the
frontend, the smart contract verifier and the signature provider. The four
explorer services run the images `.github/workflows/explorer-images.yml`
publishes to `ghcr.io/sidiora-labs/`, so nothing is compiled locally;
`EXPLORER_IMAGE_TAG` is required and selects a published image tag, such as
`main` or `sha-<short commit>`.

| Variable | What it is |
| --- | --- |
| `EXPLORER_IMAGE_TAG` | Published explorer image tag; required |
| `RPC_HTTP_URL` | JSON-RPC HTTP endpoint of the node to index; defaults to port 8545 on the Docker host gateway |
| `RPC_WS_URL` | JSON-RPC websocket endpoint of the same node; same default |
| `CHAIN_ID` | EIP-155 chain id; set to `125` for mainnet-beta; no Compose default |
| `SECRET_KEY_BASE` | Phoenix signing secret, at least 64 bytes; no default, `openssl rand -base64 48` produces one |

```sh
: "${EXPLORER_IMAGE_TAG:?Set a published explorer image tag}"
: "${RPC_HTTP_URL:?Set the node HTTP root URL}"
: "${RPC_WS_URL:?Set the node WebSocket root URL}"
: "${SECRET_KEY_BASE:?Set the Phoenix signing secret}"
export EXPLORER_IMAGE_TAG RPC_HTTP_URL RPC_WS_URL SECRET_KEY_BASE
CHAIN_ID=125 docker compose -f services/explorer/deploy/docker-compose.local.yml up
```

The default published backend port is 4000 and frontend port is 3000. Both
services have health checks, so `docker compose ps` reports their readiness. Leaving `CHAIN_ID` or `SECRET_KEY_BASE` empty
stops the affected container rather than starting it with a stand-in value.
The Compose file loads
[`deploy/env/backend.example.env`](deploy/env/backend.example.env) and
[`deploy/env/frontend.example.env`](deploy/env/frontend.example.env) and sets
connection URLs and published host/port values explicitly. Use
`EXPLORER_PUBLIC_HOST`, `BACKEND_PUBLISHED_PORT` and `FRONTEND_PUBLISHED_PORT`
to change the browser-facing host and ports. It sets the backend's
`ETHEREUM_JSONRPC_HTTP_URL`, `ETHEREUM_JSONRPC_TRACE_URL` and
`ETHEREUM_JSONRPC_ETH_CALL_URL` from `RPC_HTTP_URL`, and
`ETHEREUM_JSONRPC_WS_URL` from `RPC_WS_URL`.

The `fixture` profile adds `rpc-fixture`, a recorded JSON-RPC server
(`deploy/tools/rpc-fixture-server.py` serving `deploy/tools/fixtures/`), in
place of a node; `deploy/tools/tests/compose-smoke-test.sh` brings the
database, that server and the backend up under it. To check the definition
without starting anything:

```sh
docker compose -f services/explorer/deploy/docker-compose.local.yml config
```

## Deploying

`deploy/railway/` holds the per-service build and deploy configuration for a
Railway project together with the variable names the backend and the frontend
need. `deploy/railway/README.md` also records the image and the config file path
each service has to be given, because neither is expressible in
`railway.json`.

`deploy/tools/copy-blockscout-11-to-10.sh` copies the core chain tables out of
a Blockscout 11.x database into a freshly migrated 10.2.6 one, intersecting the
two schemas by column name instead of assuming they match. It takes both
connection strings from the environment; run it with `DRY_RUN=1` first to see
which columns each table would gain and lose.

## Test ratio and lint scripts

The [explorer build workflow](../../.github/workflows/explorer-build.yml)
runs for pull requests touching `apps/explorer/`, `services/explorer/`,
`scripts/explorer/`, `tests/explorer/` or the workflow itself. Besides builds,
it runs the backend lint job, the frontend/services lint matrix and the test
ratio job. Lint and ratio failures fail their jobs.

### The ratio rule

A change under `apps/explorer/` or `services/explorer/` carries at least as
many test files as source files, and every changed source file has a changed
test that references it. A test
references a source file when it names its Elixir module, names something the
file exports, contains a fragment of its path, or sits where that source file's
test belongs - beside it under the same name, or under the umbrella `test/`
mirror of its `lib/` path.

Files are classified by name, and by content for Rust:

| Class | Files |
| --- | --- |
| source | an Elixir `.ex`; a TypeScript `.ts` or `.tsx` that is not a test; a Rust `.rs` that is not a test module |
| test | an Elixir `*_test.exs`; a TypeScript `*.test.ts`, `*.test.tsx`, `*.spec.ts`, `*.spec.tsx` or `*.pw.tsx`; a Rust file under a `tests/` directory or carrying a `#[cfg(test)]` module |
| neither | everything else - shell scripts, YAML workflows and compose files, JSON deployment definitions, environment presets, SVG marks and Markdown |

A file classified as neither carries no test requirement, and a range that
changes no explorer source file passes whatever else it carries.

### Running the ratio gate locally

`scripts/explorer/test-ratio.sh` takes the range to measure. The pull request job
passes the merge base of the base branch and the head commit, which is what the
three-dot form computes:

```sh
scripts/explorer/test-ratio.sh origin/main...HEAD
scripts/explorer/test-ratio.sh HEAD~1..HEAD
scripts/explorer/test-ratio.sh origin/main HEAD
```

It exits 0 when the ratio holds or the range changes no explorer source file, 1
when it does not and names every source file left without a referencing test,
and 2 when the arguments or the repository do not resolve.
`tests/explorer/test-ratio-test.sh` is the gate's own test: it builds a
throwaway git history and asserts the exit code and the named files for a
satisfied ratio, an unsatisfied one, a test that references nothing changed, a
shell and documentation change and an empty range.

### Running the lint gates locally

One script per language, each running exactly what its leg of `explorer-lint`
runs, each from anywhere in the repository:

```sh
scripts/explorer/lint-backend.sh
scripts/explorer/lint-frontend.sh
scripts/explorer/lint-services.sh
```

`lint-backend.sh` runs `mix format --check-formatted` and `mix credo --strict`
through `deploy/tools/mix-in-builder.sh`, so the container, the toolchain and
the mix invocations are the ones the job uses; it runs both checks even when the
first fails and exits with the first non-zero code, naming the failing command
and its log. `lint-frontend.sh` installs the frozen lockfile and runs
`yarn lint:eslint` and `yarn lint:tsc` from `apps/explorer/`, with `HUSKY=0`.
`lint-services.sh` runs
`cargo fmt --all --check` and `cargo clippy --all-targets --locked -- -D warnings`
in each Rust service, installing the protocol-buffer compiler and the OpenAPI
plugin the builds generate from if they are absent.
