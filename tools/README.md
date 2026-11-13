# Tools

Developer and operator tools for building, inspecting, testing, releasing, and
operating Paxeer X Network. They support the node, applications, and hosted
services around one BFT network. The kernel runs inside `paxd`, and a receipt is
final when its block is committed.

## Layout

| Path | Contents |
| --- | --- |
| [benchmark/](benchmark/) | Chain benchmarks, transaction-load generation, scenarios, and statistics. |
| [bringup/](bringup/) | Node and service setup, public-name routing through `edge.sh`, and live checks through `check-live.sh`. |
| [build/](build/) | Shared build helpers, including `sanitizers.mk` for the root Makefile and `npm-workspace-build.mjs` for JavaScript workspaces. |
| [chain/](chain/) | The `paxd tools` command group, registered by `daemon/paxd/cmd/root.go`, and a standalone upgrade replay over a copy of chain state. |
| [ci/](ci/) | CI reporting, consensus checks, wallet test execution, and workflow tests. |
| [codebase-map/](codebase-map/) | An offline, browsable map of the checkout. |
| [containers/](containers/) | Container build checks and image tests. |
| [deploy/](deploy/) | Image construction and Railway deployment scripts. |
| [deps/](deps/) | Dependency-floor definitions and manifest and lockfile checks. |
| [docs/](docs/) | System-documentation generation scripts and tests. |
| [flyci/](flyci/) | The Fly.io CI controller and runner image; each has its own `fly.toml`. |
| [hpx/](hpx/) | Node installation, artifact distribution, and registry hosting. |
| [hyperfide/](hyperfide/) | Vendored Hyperfide code and developer tooling. |
| [jev/](jev/) | Decision-service client and checks for dependencies, documentation, failures, ledgers, and pull requests. |
| [launchpad/](launchpad/) | Launchpad cache, storage, delivery, coverage, and production checks. |
| [loadtest/](loadtest/) | The `paxload` transaction generator, sender, profiles, and statistics. |
| [names/](names/) | The `paxnames` CLI for publishing and verifying content-addressed sites. |
| [ops/](ops/) | Backup, restore, monitoring, and keyring utilities. |
| [paxctl/](paxctl/) | The `seictl` node-management CLI, sidecar, and workflow packages. |
| [paxeer-x/](paxeer-x/) | Network build fragments, release scripts, governance helpers, and checks. |
| [platform/](platform/) | Rust workspace for the platform CLI, emulator library, and release tooling. |
| [pnsdns/](pnsdns/) | DNS service backed by the on-chain names registry and resolver. |
| [price-sidecar/](price-sidecar/) | Price-sidecar configuration template, service unit, and packaging tests. |
| [proofs/](proofs/) | SMT arithmetic proof inputs. |
| [qualification/](qualification/) | Network and application checks, local-node drivers, and release runners. |
| [railway/](railway/) | Deployment-file conversion and environment-file tests. |
| [release/](release/) | Node release builds, snapshots, staging, rollback, and upgrade rehearsals. |
| [repotest/](repotest/) | Repository structure, module, test reachability, and generation checks. |
| [rollout/](rollout/) | Rollout execution and ledger utilities. |
| [runtime/](runtime/) | `run-with-clock.sh`, which runs a command with the runtime clock. |
| [spec/](spec/) | Hyperfide projection helpers and numeric fixtures. |
| [specgen/](specgen/) | Compiles KVX specifications under `spec/` into generated rule files with `go run ./tools/specgen -root .`. |
| [tx-scanner/](tx-scanner/) | The `paxd tools scan-tx` command, gRPC queries, and checkpoint storage. |
| [uci/](uci/) | Vendored continuous-integration configuration and documentation. |
| [utils/](utils/) | Shared Go module-store helpers. |
| [v6120-halt-export/](v6120-halt-export/) | Launchpad halt-state fixture export and upgrade replay. |
| [v6120-holders-scan/](v6120-holders-scan/) | Launchpad token-holder fixture collection at a selected block height. |
| [wallet/](wallet/) | Wallet PWA argument tests. |
| [wasm-tools/](wasm-tools/) | Vendored WebAssembly tools and supporting crates. |
| [workspace/](workspace/) | Workspace manifests, check inventories, and tool checksums. |
| [log_inspect.c](log_inspect.c) | Reports log capacity, valid tail, durable head, and resume sequence. |
| [lxp_arith_proof.sh](lxp_arith_proof.sh) | Runs arithmetic proofs and sanitizer checks. |
| [lxp_corpus_gen.c](lxp_corpus_gen.c) | Generates replay activity corpora and root ledgers. |
| [lxp_legacy_reader.c](lxp_legacy_reader.c) | Reads canonical activity streams from local files. |
| [lxp_qual_replay_matrix.sh](lxp_qual_replay_matrix.sh) | Builds replay runners and compares their output across compilers and architectures. |
| [lxp_reserve_report.c](lxp_reserve_report.c) | Decodes reserve reports and lists reserve classes. |
| [lxp_shadow_compare.c](lxp_shadow_compare.c) | Compares execution outcomes and records divergences. |

## Build and test

There is no Makefile or package manifest for `tools/` as a whole. The root build
files cover integrated tools; independent projects have their own commands.
Run the following from the repository root unless the directory column says
otherwise. These commands require the dependencies of the selected component.

| Directory | Command | Purpose and source |
| --- | --- | --- |
| Repository root | `make -f chain.mk build` | Builds `paxd`, including the transaction scanner, through [chain.mk](../chain.mk). |
| Repository root | `make -f chain.mk build-loadtest` | Builds the benchmark load generator through [chain.mk](../chain.mk). |
| Repository root | `make test-rebuild test-reserve test-legacy-readonly test-shadow` | Builds and exercises the C log, reserve, legacy-reader, and comparison helpers through [Makefile](../Makefile). |
| Repository root | `make qualify-replay` | Builds the corpus generator and runs the replay matrix through [Makefile](../Makefile). |
| Repository root | `make qualify-arith` | Runs arithmetic tests and the proof script through [Makefile](../Makefile). |
| Repository root | `make test-tools-python` | Runs the selected Python tools suites declared in [Makefile](../Makefile) and included test fragments. |
| Repository root | `make test-qualification-python` | Runs the Python qualification suite selected by [Makefile](../Makefile). |
| Repository root | `make platform-build` | Builds the platform tools and related workspaces through [services/platform/Makefile.inc](../services/platform/Makefile.inc). |
| Repository root | `make platform-test-tooling` | Tests platform tooling with its runtime-clock and TLS prerequisites through [services/platform/Makefile.inc](../services/platform/Makefile.inc). |
| Repository root | `npm run build` | Runs the workspace build helper declared in [package.json](../package.json). |
| Repository root | `npm --prefix tools/codebase-map run generate` | Generates the offline map using [codebase-map/package.json](codebase-map/package.json). |
| Repository root | `npm --prefix tools/codebase-map run check` | Tests the map generator and viewer using [codebase-map/package.json](codebase-map/package.json). |
| `tools/loadtest` | `make build` and `make test` | Builds `paxload` and runs its tests with race detection and coverage; see [loadtest/Makefile](loadtest/Makefile). |
| `tools/paxctl` | `make build` and `make test` | Builds and checks the CLI release and runs Go tests; see [paxctl/Makefile](paxctl/Makefile). |
| `tools/names` | `npm run build` | Compiles the CLI using [names/package.json](names/package.json). |
| `tools/hyperfide` | `make all` and `make test` | Builds the native CLI and runs unit and integration tests; see [hyperfide/Makefile](hyperfide/Makefile). |
| `tools/wasm-tools` | `ci/build-release-artifacts.sh` | Builds release artifacts using the checked-in [script](wasm-tools/ci/build-release-artifacts.sh). |
| `tools/wasm-tools` | `cargo test --workspace --locked --features prefer-btree-collections` | Runs the workspace tests selected by the [CI workflow](wasm-tools/.github/workflows/main.yml). |

The [proofs/](proofs/), [price-sidecar/](price-sidecar/),
[uci/](uci/), and [workspace/](workspace/) directories hold proof inputs,
packaging, configuration, or inventory files rather than standalone binaries.

### Transaction scanner

The scanner compares block transactions with indexed transaction hashes and
checks that each indexed transaction can be queried by hash. Use an archive
node with the required block history and gRPC transaction queries enabled.
The scanner records blocks with missing or mismatched indexed transactions.

`paxd tools scan-tx` reads blocks through the node's gRPC server. `--endpoint`
defaults to the loopback interface, and `--port` defaults to 9090.
`--start-height` sets the first height; zero or
negative values start at height 1. `--state-dir` enables checkpoint storage.
`--batch-size` defaults to 100 and `--bps-limit` to 400 blocks per second. Both
must be positive, and batches are capped at the rate limit.

A valid checkpoint takes precedence over `--start-height`. The scanner resumes
from its saved height, processes blocks below the latest height, and polls every
ten seconds when caught up. A batch with block-query errors is retried at the
same height; the checkpoint advances only after all block queries in that batch
succeed. It continues until stopped.

The checkpoint `tx-scanner-state.json` contains `last_processed_height`, the next
height to scan after a successful batch, and `blocks_missing_txs`, the heights
with missing or mismatched indexed transactions. Checkpoint writes preserve the
previous valid checkpoint as `tx-scanner-state.json.previous` before replacing
the current file.

Reindexing is a separate offline operation. Stop the node before using
`paxd tendermint reindex-event` against its `--home`. Its `--start-height` and
`--end-height` bounds are inclusive. The command rebuilds block and transaction
events for the selected range using the configured event sinks.

## Configuration

Configuration belongs to each tool. Environment-variable names and checked-in
configuration sources include:

- Codebase map: `LAYERX_CODE_MAP_HOST` and `LAYERX_CODE_MAP_PORT` configure the
  server. Generation reads `.codegraph/graph.db`,
  [spec/workflow.kvx](../spec/workflow.kvx), and the selected specification;
  see [codebase-map/README.md](codebase-map/README.md).
- Transaction scanner: the command-line flags described above select the gRPC
  server, scan range, throughput, and checkpoint directory.
- Upgrade replay: `UPGRADE_REPLAY_WORKDIR`, `UPGRADE_REPLAY_BINARY`,
  `UPGRADE_REPLAY_LOGDIR`, `UPGRADE_REPLAY_IN_PLACE`, and
  `UPGRADE_REPLAY_KEEP_STORES` are read by [run.sh](chain/upgrade-replay/run.sh).
- Load generators: [benchmark/loadtest/config.json](benchmark/loadtest/config.json)
  configures the benchmark generator; `paxload` selects JSON configuration with
  `--config`, with profiles under [loadtest/profiles/](loadtest/profiles/).
- Runtime clock wrapper: `LAYERX_RUNTIME_CLOCK_SOCKET`,
  `LAYERX_RUNTIME_CLOCK_BIN`, `LAYERX_RUNTIME_CLOCK_DIRECTORY`, and
  `CARGO_TARGET_DIR` configure [run-with-clock.sh](runtime/run-with-clock.sh).
- Arithmetic proofs: `LXP_QUAL_TOOL_ROOT` selects the proof toolchain location
  for [lxp_arith_proof.sh](lxp_arith_proof.sh).
- Node releases: `CC`, `CXX`, `PAXD_COMMIT`, `PAXD_DIST_DIR`, and
  `SOURCE_DATE_EPOCH` are read by [build-paxd.sh](release/build-paxd.sh), alongside
  [rust-toolchain.toml](../rust-toolchain.toml),
  [toolchain.env](release/toolchain.env), and [version.json](release/version.json).
- Fly.io controller: `GITHUB_REPOSITORY`, `GITHUB_TOKEN`, `FLY_API_TOKEN`,
  `FLY_RUNNER_APP`, `FLY_REGION`, `RUNNER_IMAGE`, `RUNNER_LABELS`,
  `MACHINE_CPU_KIND`, `MACHINE_CPUS`, `MACHINE_MEMORY_MB`, `MAX_MACHINES`,
  `POLL_INTERVAL`, `RUNNER_IDLE_TIMEOUT`, `ORPHAN_TTL`, `GITHUB_API_URL`,
  `FLY_API_URL`, and `CI_READINESS_CONFIG_FILE` are read by
  [config.go](flyci/controller/config.go). Deployment files are
  [controller/fly.toml](flyci/controller/fly.toml) and
  [runner/fly.toml](flyci/runner/fly.toml).
- Names CLI: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and
  `AWS_SESSION_TOKEN` configure storage authentication in
  [cli.ts](names/src/cli.ts).
- Node-management sidecar: `SEI_CHAIN_ID`, `SEI_GENESIS_BUCKET`,
  `SEI_GENESIS_REGION`, `SEI_SNAPSHOT_BUCKET`, `SEI_SNAPSHOT_REGION`,
  `SEI_SNAPSHOT_UPLOAD_INTERVAL`, `SEI_SNAPSHOT_UPLOAD_TIMEOUT`,
  `SEI_KEYRING_BACKEND`, `SEI_KEYRING_DIR`, and `SEI_KEYRING_PASSPHRASE` are read
  by [serve.go](paxctl/serve.go); see [paxctl/README.md](paxctl/README.md).
- HPX hosting reads `HPX_REGISTER_TOKEN` in [deploy.sh](hpx/hosting/deploy.sh);
  see [hpx/README.md](hpx/README.md) for node and mirror configuration.
- Decision client: `JEV_ENDPOINT`, `JEV_MODEL`, and `OPENROUTER_API_KEY` are read
  by [client.py](jev/client.py).
- Backup: `BACKUP_ENV` selects the environment file;
  [backup.env.example](ops/backup/backup.env.example) lists its configuration
  names. Monitoring reads the environment file supplied to
  [render.sh](ops/monitoring/render.sh) and uses
  [prometheus.yml](ops/monitoring/prometheus.yml),
  [blackbox.yml](ops/monitoring/blackbox.yml), and
  [rules.yml](ops/monitoring/rules.yml).
- Platform release tooling reads [registries.kvx](platform/release/registries.kvx).
  Price-sidecar packaging provides [config.template.json](price-sidecar/config.template.json).

## Documentation

- [Codebase map](codebase-map/README.md), [dependency floors](deps/README.md),
  [HPX](hpx/README.md), [load testing](loadtest/README.md),
  [node management](paxctl/README.md), and [WebAssembly tools](wasm-tools/README.md).
- [Hyperfide](hyperfide/README.md), its [documentation guide](../docs/hyperfide/DOCUMENTATION.md),
  and [scripts and tooling reference](../docs/hyperfide/reference/scripts-and-tooling.md).
- [Validator runbook](../docs/release/7.0.0-validator-runbook.md) and
  [service operator runbook](../docs/release/7.0.0-service-operator-runbook.md).
- Documentation site: [network operations](../docs/site/paxeer-docs/public/markdown/network-operations.md)
  and [admin and HPX](../docs/site/paxeer-docs/public/markdown/admin-hpx.md).
