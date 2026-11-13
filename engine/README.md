# Engine

This directory holds the EVM executors, state and keeper support, and execution tests used by `paxd` in the Paxeer network. The executor runs transactions through go-ethereum with either its Geth interpreter or an evmone backend connected through EVMC. The node supplies the chain configuration, state database and custom precompiles; the engine executes against that state as part of block processing.

## Layout

- [deps/](deps/) — bank and EVM keepers, EVM state storage, a KV cache, the optimistic concurrency scheduler and test helpers.
- [executor/](executor/) — transaction execution, executor configuration, the EVMC host and interpreter, the evmone library loader and precompile abort handling.
- [tests/](tests/) — in-process comparisons of V2 and Giga execution, a state-fixture loader and builders, and fixture data.
- [executor/executor.go](executor/executor.go) — Geth and evmone constructors, standard transaction execution and execution with fees already charged.
- [executor/config/config.go](executor/config/config.go) — Giga executor options and the application configuration template.
- [executor/lib/evmlib.go](executor/lib/evmlib.go) — loads the platform-specific evmone shared library.
- [tests/AGENTS.md](tests/AGENTS.md) — fixture selection and state-comparison test instructions.

## Build and test

The packages belong to the root [Go module](../go.mod). Run these targets from the repository root; their recipes are in [chain.mk](../chain.mk):

| Command | Purpose |
| --- | --- |
| `make -f chain.mk build` | Build `build/paxd`, including the engine through the node, after building its native libraries. |
| `make -f chain.mk evmone-lib` | Build the evmone shared library for Linux on amd64 for the EVMC backend. |
| `make -f chain.mk test` | Run the root Go test groups, including the engine packages, with race detection and coverage. |

Before running the state-comparison tests, configure fixture selection as described in [the state-test instructions](tests/AGENTS.md). Those tests exercise transactions through the node's `ProcessBlock` path and compare execution results, gas usage and explicit account and log expectations.

## Configuration

[Executor configuration](executor/config/config.go) reads `giga_executor.enabled` and `giga_executor.occ_enabled` from the node's application options. The `[giga_executor]` section is included in `app.toml` by [the node configuration](../daemon/paxd/cmd/app_config.go).

The state-test sources read these environment variables:

- `STATE_TEST_ROOT` — fixture root, relative to the test package working directory unless absolute.
- `STATE_TEST_DIR` — fixture category to select.
- `STATE_TEST_NAME` — fixture name selector.
- `STATE_TEST_FORK` — fixture post-state fork to select.
- `STATE_TEST_COUNT` — required number of executed fixtures.
- `IGNORE_SKIP_LIST` — selects execution without category or case exclusions.
- `VERIFY_ETHEREUM_SPEC` — enables explicit fixture account and log checks.
- `USE_REGULAR_STORE` — selects the regular KV store for the Giga comparison.

The harness loads JSON fixtures and can extract [tests/data/fixtures_general_state_tests.tgz](tests/data/fixtures_general_state_tests.tgz). Its skip-list loader reads [tests/data/skip_list.json](tests/data/skip_list.json); the state-comparison entry point executes its selected fixtures without exclusions. Fixture expectations used by the tests also live in [tests/data/state_repairs.json](tests/data/state_repairs.json).

## Documentation

- [Engine guide](../docs/site/paxeer-docs/public/markdown/engine.md)
- [Engine documentation site page](../docs/site/paxeer-docs/src/app/engine/page.tsx)
- [EVM module guide](../docs/site/paxeer-docs/public/markdown/evm.md)
- [Parallel execution guide](../docs/site/paxeer-docs/public/markdown/parallelization.md)
- [Precompiles](../docs/system/precompiles.md)
