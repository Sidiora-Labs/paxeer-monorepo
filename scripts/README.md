# scripts

The `scripts/` directory contains operational utilities, generators, diagnostics, CI checks, qualification gates, and test fixtures for the Paxeer mainnet-beta node. These commands support the single BFT network and its merged kernel, EVM chain (chain ID 125), local node workflows, and the surrounding SDK and service workspaces.

## Layout

- `build/` — versioned Make fragments for Paxeer X build steps.
- `bump_version/` — Go precompile archive and version-management tool.
- `ci/` — continuous-integration checks, scanners, release checks, and their fixtures.
- `evm_stress/` — Go helpers for local EVM transfer stress runs.
- `explorer/` — explorer lint and test gates plus Rust fixture sources.
- `fixtures/` — committed node and maintenance fixture data.
- `gates/` — wave and programme gate runners and gate tests.
- `interop/` — interop dependency policy checks.
- `maintenance/` — the Go maintenance command entry point.
- `paxeer-x/` — SDK and program qualification scripts.
- `programs/` — program fixture, ABI, and dependency-policy helpers.
- `qualification/` — qualification programs and Paxeer X evidence checks.
- `replica/` — codec replay fixture generation.
- `wallet/` — wallet lint, test, deployment, live checks, and secret scans.
- `workspace/` — workspace manifest, dependency, compiler, and core-gate checks.
- `SidioraProxyGovernance.s.sol` — Solidity governance script for the proxy timelock.
- `changelog.go` — Go changelog generation implementation.
- `check-paxd-native-digest.sh` — verifies that a `paxd` binary carries the current native archive digest.
- `checksums.go` — Go checksum publication implementation.
- `codify.sh` — installs or updates the Codify command-line tool.
- `consensus_config.go` — Go consensus configuration updater.
- `dump_app_state_for_height.sh` — exports application state at a selected height.
- `evm_stress.sh` — initializes a disposable local chain and runs EVM transfer stress traffic.
- `faucet_launcher.py` — starts the local faucet workflow.
- `generate-changelog.sh` — shell entry point for changelog generation.
- `initialize_local_chain.sh` — creates and configures a local node home.
- `maintenance.go` — Go maintenance operations for duration checks, checksums, changelogs, consensus configuration, stress setup, and state export.
- `maintenance.sh` — dispatches maintenance operations to the Go command.
- `node_maintenance.py` — validates node homes and manages snapshot and state-sync workflows.
- `package-static-wasm.py` — packages and records static WASM archives.
- `paxd-service.sh` — controls and checks the node service.
- `paxd_problems_detector.py` — reports local node, host, and storage conditions.
- `protoc.sh` — generates protobuf and Swagger sources.
- `qualify-mirror-live.sh` — qualifies a live mirror publisher.
- `qualify-mirror-verification-live.sh` — qualifies live mirror verification and failover paths.
- `repair_m802_test.go` — Go tests for maintenance operations, state exports, and local configuration.
- `run-sha256sum.sh` — publishes checksums through `maintenance.sh`.
- `run_blocktests.sh` — runs node block tests with isolated CI homes.
- `run_snapshot.sh` — coordinates a halted-height snapshot cycle.
- `snapshot.sh` — invokes the Python snapshot command.
- `snapshot_cycle.py` — validates snapshot configuration, contents, and receipts.
- `specgen.sh` — invokes the repository specification generator.
- `start_faucet.sh` — invokes `faucet_launcher.py`.
- `state_export.go` — Go state-export implementation.
- `state_sync.sh` — invokes the Python state-sync command.
- `stress_config.go` — validates and renders local stress configuration.
- `swagger_assets.py` — validates and publishes embedded Swagger assets.
- `test_paxd_native_digest.py` — tests native archive digest checking and its build integrations.
- `test_repair_22_31_320.py` — tests maintenance, recovery, and configuration contracts.
- `update-swagger-ui-statik.sh` — regenerates or embeds the node Swagger UI.

## Build and test

Run commands from the repository root. There is no standalone directory build; the maintenance command uses the root Go module, and the root Makefile supplies the workspace targets:

- `make build` builds the native kernel library used by the node.
- `make paxeer-test` runs the Go test groups defined by `chain.mk`, including packages under `scripts/`.
- `make workspace-inventory-check` runs the three checks in `scripts/workspace/`.
- `make paxeer-manifest-build` and `make paxeer-manifest-test` run the workspace build and test checks.
- `bash scripts/ci/run-tests.sh` runs the root test target at both optimization levels.
- `go build -o pax-script ./scripts/maintenance` builds the maintenance command, as documented by `maintenance.sh`.
- `bash scripts/protoc.sh` regenerates protobuf sources; `bash scripts/protoc.sh --wasm-swagger` generates the WASM Swagger output.
- `bash scripts/update-swagger-ui-statik.sh` regenerates the OpenAPI assets, and `bash scripts/update-swagger-ui-statik.sh --assets-only` embeds the checked-in assets.
- `go generate ./tools/repotest` runs the repository generation directives that invoke `scripts/protoc.sh` and `scripts/bump_version`.

The directory also holds data files and focused tests. The shell and Python utilities run with their respective interpreters; Go tests are included in `make paxeer-test`.

## Configuration

Environment variables used by these commands include:

- Node and local-home settings: `HOME`, `PAXD_BIN`, `PAX_HOME`, `PAX_SCRIPT_BIN`, `PAX_LOCAL_RESET`, `PAX_CI_TEMP_ROOT`, `PAXD_CONTROLLER`, `PAXD_SERVICE`, `SNAPSHOT_DIR`, `MOCK_BALANCES`, `GIGA_EXECUTOR`, `GIGA_OCC`, and `NO_RUN`.
- Generation and packaging settings: `STATIK`, `LLVM_AR`, `CARGO`, `CARGO_TARGET_DIR`, `PYTHONPATH`, and `DOTNET`.
- Installer settings: `CODIFY_BASE_URL`, `CODIFY_INSTALL_DIR`, and `NO_COLOR`.
- Repository and release inputs: `CHANGELOG_REPO`, `GITHUB_ENV`, `GITHUB_OUTPUT`, `LAYERX_RELEASE_VERSION`, `SCAN_SECRETS_ALLOW`, and `KUBERNETES_SERVICE_HOST`.
- Beta qualification settings: `PAXEER_X_BETA_CANDIDATE`, `PAXEER_X_BETA_LEDGER_FILE`, and `PAXEER_X_BETA_SPEC_FILE`.
- Live wallet checks: `CHECK_LIVE_ATTESTOR_BASES`, `CHECK_LIVE_CA`, `CHECK_LIVE_CUTOVER_HOST`, `CHECK_LIVE_GATEWAY_BASE`, `CHECK_LIVE_GATEWAY_TOKEN`, `CHECK_LIVE_HUMAN_ASSERTION`, `CHECK_LIVE_HUMAN_ASSET`, `CHECK_LIVE_HUMAN_BASE`, `CHECK_LIVE_HUMAN_DESTINATION`, `CHECK_LIVE_HUMAN_ORIGIN`, `CHECK_LIVE_NODE_RPC_URL`, `CHECK_LIVE_RAMP_BASE`, and `CHECK_LIVE_TIMEOUT`.
- Workspace and gate settings: `GATE_LOG_DIR`, `WALLET_GATE_BUDGET_SECONDS`, `WALLET_GATE_LOG_DIR`, `WALLET_GATE_TARGETS_FILE`, `EXPLORER_GATE_BUDGET_SECONDS`, `EXPLORER_GATE_LOG_DIR`, `EXPLORER_LINT_LOG_DIR`, and `PAXEER_X_EVIDENCE_DIR`.
- Mirror qualification inputs: `LAYERX_MIRROR_LIVE_CONFIG`, `LAYERX_MIRROR_STATUS_URL`, `LAYERX_MIRROR_PUBLISHER_BIN`, `LAYERX_MIRROR_FAULT_CONTROLLER`, `LAYERX_MIRROR_LIVE_EVIDENCE_DIR`, `LAYERX_MIRROR_VERIFY_BIN`, `LAYERX_MIRROR_VERIFY_CONFIG`, `LAYERX_MIRROR_CANONICAL_REQUEST`, `LAYERX_MIRROR_FAILOVER_REQUEST`, `LAYERX_MIRROR_DIVERGENCE_REQUEST`, `LAYERX_MIRROR_TAMPER_REQUEST`, `LAYERX_MIRROR_STATE_REQUEST`, `LAYERX_MIRROR_STATE_TAMPER_REQUEST`, `LAYERX_MIRROR_EXPLORER_VERIFY_URL`, `LAYERX_NODE_URL`, `LAYERX_GATEWAY_URL`, and `LAYERX_EXPLORER_API_ORIGIN`.
- Program qualification inputs: `PAXEER_X_PROGRAM_TERMINAL_V5_ARTIFACTS`, `PAXEER_X_PROGRAM_TERMINAL_V5_CORPUS`, `PAXEER_X_RUST_PROGRAMS_BUILD_JSON`, `PAXEER_X_RUST_PROGRAMS_EVIDENCE`, `PAXEER_X_SDKGEN_BIN`, `PAXEER_X_SDKGEN_SHA256`, `PAXEER_X_SCAN_NODE`, `PAXEER_X_TASK_DEADLINE_EPOCH`, `LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE`, `LAYERX_DOTNET_PROGRAMS_RESULTS_DIR`, `LAYERX_GO_PROGRAMS_TEST`, `LAYERX_GO_PROGRAMS_TEST_SHA256`, `LAYERX_GO_PROGRAMS_REVISION`, and `LAYERX_GO_PROGRAMS_LOG`.

Within the selected node home, the node workflows read `config/config.toml`, `config/app.toml`, `config/client.toml`, `config/genesis.json`, and node data under `data/`. Generation and checks read `buf.yaml`, `buf.gen.yaml`, `consensus/internal/buf.gen.yaml`, `consensus/internal/wireguard.buf.gen.yaml`, `docs/swagger-ui/swagger.yml`, `tools/release/version.json`, and `tools/workspace/paxeer-manifest-roots.txt`.

## Documentation

- [Paxeer Network documentation](../docs/paxeer-chain-docs.md) describes the node and Swagger generation scripts.
- [Programs workspace gates](../docs/wiki/ProgramsWorkspace.md) documents the program scripts and their workspace checks.
- [Programs](../docs/wiki/Programs.md) documents program fixtures and ABI tooling.
- [REST and gRPC APIs](../docs/site/paxeer-docs/public/markdown/rest-grpc.md) documents `scripts/protoc.sh` and the generated API assets.
- [Paxeer documentation site](../docs/site/paxeer-docs/README.md) is the documentation site's entry page.
