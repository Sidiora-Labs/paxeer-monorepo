# Paxeer release tools

This directory holds the reproducible `paxd` release builder, artifact and checksum checks, upgrade metadata tools, rollout and rollback helpers, and local rehearsal programs. It supports the single Paxeer network: the kernel runs inside `paxd`, block commits establish receipt finality, and the EVM chain ID is 125.

## Layout

- `env/` contains blank operator-input templates for rehearsal and rollout environment files.
- `rehearsal/` contains local-network rehearsal helpers, programme checks, pool-parity checks, genesis rewriting, receipt creation, and their Python tests.
- `tests/` contains shell, Python, and integration tests for building, identity, environment templates, rehearsal, reproducibility, snapshots, staging, swapping, rollback, release verification, and workflows.
- `upgradeinfo/` contains the Go tests for release and node upgrade metadata.
- `build-paxd.sh` builds the Linux node and release assets, writes `SHA256SUMS`, and generates `upgrade-info.json`.
- `extld.sh` supplies the external linker used by the release build and makes the Wasm runtime rpath relocatable.
- `feed_key.py` creates price-feed keys and `registerFeedKey(bytes)` calldata.
- `fleet-views.json` records the precompile views and pointer checks used by fleet verification.
- `fleet_check.py` validates release inputs and runs the staged fleet checks and handoff checks.
- `rehearse-v6120.sh` runs or checks the local four-validator upgrade rehearsal.
- `reproducible-check.sh` builds twice and compares native-library and release checksums.
- `snapshot.sh` archives and verifies stopped node homes at a halt height.
- `stage.sh` provides release identity, environment, locking, process, local staging, and fleet staging functions.
- `swap.sh` verifies and installs a release binary and restarts selected node homes.
- `rollback.sh` verifies or executes the local or fleet rollback path.
- `verify-release.sh` compares a published release directory with a build directory and its checksums.
- `upgrade_info.py` writes release and node upgrade metadata.
- `version.json` records the release version and upgrade plan identity.
- `toolchain.env` records the C/C++ compiler version required by the release build.
- `release_integrity_test.go` tests compiler selection, source provenance, process ownership, swapping, and rollback.

## Build and test

Run commands from the repository root.

- `make paxd-release` invokes `tools/release/build-paxd.sh` and produces the versioned node, price sidecar, source revision, `native-libs.sha256`, `upgrade-info.json`, and `SHA256SUMS`. On Linux amd64 it packages the Wasm runtime libraries as well.
- `bash tools/release/reproducible-check.sh` performs two clean release builds and requires identical checksums.
- `bash tools/release/tests/run-all.sh` runs the release shell and Python suites, the rehearsal suites, and `go test ./tools/release/upgradeinfo/`.
- `make test-tools-python` runs the Python suites registered by `tools/release/tests/python-tests.mk` and `tools/release/rehearsal/python-tests.mk`.
- `go test ./tools/release/upgradeinfo/` runs the release metadata integration tests directly.

## Configuration

The builder reads `version.json`, `toolchain.env`, the root `go.mod` and `rust-toolchain.toml`, `wasm-runtime/libwasmvm-linux.sha256`, and inherited `CC`, `CXX`, `CARGO_HOME`, `PAXD_DIST_DIR`, `PAXD_COMMIT`, `PAXD_RELEASE_LAYERXD`, and `SOURCE_DATE_EPOCH`. `extld.sh` reads `CC`. Fleet checks also read `fleet-views.json`; rehearsal programme checks use `rehearsal/share.golden.json`, and upgrade metadata tests use `upgradeinfo/merge-plan-info.example.json`.

`env/rehearsal.env.example` names the rehearsal and sidecar inputs: `SOURCE_DATA`, `SOURCE_GENESIS`, `OLD_BIN`, `NEW_BIN`, `WORK`, `PORT_BASE`, `COSMOS_CHAIN_ID`, `EXPORT_HEIGHT`, `REHEARSAL_HALT_HEIGHT`, `CHAINFLOW_KEY_FILE`, `PRICE_SIDECAR_CONFIG`, `PRICE_SIDECAR_FEED_KEY_FILE`, `PRICE_FEED_REST_API_KEY_FILE`, `PRICE_SIDECAR_STATE_DIR`, `PRICE_SIDECAR_HEALTH_ADDR`, `PRICE_SIDECAR_CONSENSUS_RPC_URL`, and `PRICE_SIDECAR_EVM_RPC_URL`. The rehearsal env-file reader also accepts `OPERATOR_KEY_NAMES`, `KEYRING_BACKEND`, and `CREDENTIAL_DIR`. The merge rehearsal requires a plan-info document supplied with `--plan-info`.

`stage.sh`, `swap.sh`, `rollback.sh`, `snapshot.sh`, and `fleet_check.py` accept operator files based on `env/rollout.env.example`. Their names are `RELEASE_ID`, `RELEASE_TAG`, `RELEASE_SHA256`, `SIDECAR_SHA256`, `HALT_HEIGHT`, `CHAIN_ID`, `KEYRING_BACKEND`, `BIN_DIR`, `VALIDATOR_HOSTS`, `VALIDATOR_HOMES`, `VALIDATOR_UNITS`, `OPERATOR_KEY_NAMES`, `FULLNODE_HOSTS`, `FULLNODE_HOMES`, `FULLNODE_UNITS`, `ARCHIVE_HOST`, `EDGE_HOST`, `OPS_HOST`, `BUILD_HOST`, `SNAPSHOT_DIR`, `CREDENTIAL_DIR`, `AUTOMATION_UNITS`, `OLD_FEEDER_UNITS`, `SIDECAR_UNITS`, `CHAINFLOW_KEY_FILE`, `CONSENSUS_RPC`, `EVM_RPC`, `DATA_SERVICE_URLS`, `APP_URL`, `APP_BUILD_ID`, `INVENTORY_FILE`, `POOL_COUNT`, and `RECEIPT_DIR`.

Rollback forward-recovery inputs are `RECOVERY_BIN` and `RECOVERY_SHA256`; isolated-fork rollback inputs are `ROLLBACK_ISOLATED_FORK` and `ROLLBACK_SOURCE_GENESIS`. The rehearsal programme also accepts `PROGRAMME_TRADER_KEY_FILE` and `PROGRAMME_MAKER_KEY_FILE`. The node files read or updated by these paths include `config/config.toml`, `config/app.toml`, `data/upgrade-info.json`, and release `SHA256SUMS` files.

The release tests additionally read `REHEARSE_OLD_BIN`, `REHEARSAL_ENV_FILE`, `REHEARSAL_RECEIPT`, `REHEARSAL_TEST_MODE`, `PAXD_BIN`, `SIDECAR_BIN`, `OLD_BIN`, `NEW_BIN`, and `WORK` when those test inputs are supplied.

## Documentation

- [Candidate release report](../../docs/release/7.0.0-candidate-report.md)
- [Readiness checklist](../../docs/release/7.0.0-readiness-checklist.md)
- [Readiness matrix](../../docs/release/7.0.0-readiness-matrix.md)
- [Upgrade plan](../../docs/release/7.0.0-upgrade-plan.md)
- [Validator runbook](../../docs/release/7.0.0-validator-runbook.md)
- [Service operator runbook](../../docs/release/7.0.0-service-operator-runbook.md)
- [Consensus-changing commits](../../docs/release/consensus-commits.md)
- [Documentation-site operators page](../../docs/site/paxeer-docs/src/app/operators/page.tsx)
- [Documentation-site operators markdown](../../docs/site/paxeer-docs/public/markdown/operators.md)
