# Common precompile support

This directory holds the shared Go implementation used by Paxeer X's native EVM precompiles inside `paxd`. It connects EVM calls to the SDK context, dispatches ABI methods, handles gas and payments, encodes errors and events, and checks upgrade activation and retirement. It also embeds CosmWasm pointer runtimes. This is a support package for the network's registered precompiles.

## Layout

- [legacy/](legacy/): versioned common implementations and keeper interfaces used by versioned precompiles.
- [pointer_artifacts/](pointer_artifacts/): embedded CW20, CW721 and CW1155 runtime binaries and their checksum and compiler manifest.
- [precompiles.go](precompiles.go): fixed and dynamic gas wrappers, ABI dispatch, revert encoding, argument and payment validation, address resolution, and payment helpers.
- [evm_events.go](evm_events.go): EVM log emission and staking and distribution event encoders.
- [lxp_errors.go](lxp_errors.go): shared LXP error registry, kernel result names, and `Error(string)` revert construction.
- [merge_plan.go](merge_plan.go): upgrade keeper queries for plan activation at the call's block height.
- [retire.go](retire.go): upgrade activation reader and retirement responses carrying `LXP_MODULE_DISABLED`.
- [pointer_artifacts.go](pointer_artifacts.go): embedded runtime loading, checksum validation, and installation over a recognized deployed runtime.
- [erc20_abi.json](erc20_abi.json): ERC20 ABI template used by the package tests.
- [precompiles_test.go](precompiles_test.go): validation, dispatch, and dynamic gas revert tests.
- [evm_events_test.go](evm_events_test.go): event signatures, log emission, and ABI encoding tests.
- [resolution_reason_test.go](resolution_reason_test.go): registry reasons, payment remainder validation, fixed gas tables, and unknown selector tests.
- [precompile_stdout_test.go](precompile_stdout_test.go): checks that dynamic gas dispatch writes no standard output.
- [merge_plan_test.go](merge_plan_test.go): plan activation height and absent keeper tests.
- [retire_test.go](retire_test.go): retirement activation, refusal data, registration, and gas behavior tests.
- [retire_reader_test.go](retire_reader_test.go): upgrade reader lookup tests.
- [retirement_option_test.go](retirement_option_test.go): retirement option behavior at and below the plan height.
- [lxp_errors_test.go](lxp_errors_test.go): registry agreement with kernel results and revert encoding tests.
- [lxp_agent_pay_rows_test.go](lxp_agent_pay_rows_test.go): agent, payment, and arbiter registry coverage tests.
- [lxp_da_missing_row_test.go](lxp_da_missing_row_test.go): checks that the DA missing registry entry is distinct from kernel results.
- [lxp_market_rows_test.go](lxp_market_rows_test.go): registered perpetuals, pricefeed, and launchpad error coverage tests.
- [lxp_site_rows_test.go](lxp_site_rows_test.go): framework and precompile error coverage tests.
- [lxp_python_parity_test.go](lxp_python_parity_test.go): Python SDK retryable error map agreement tests.
- [lxp_typescript_parity_test.go](lxp_typescript_parity_test.go): TypeScript SDK retryable error set agreement tests.
- [proof_error_registry_test.go](proof_error_registry_test.go): Go revert production and Python SDK decoding tests, including precompile call paths.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no local Makefile. Run these commands from the repository root using the chain targets in [chain.mk](../../chain.mk):

```sh
make -f chain.mk build
make -f chain.mk test
```

The build produces `build/paxd` and includes common precompile support through the node. Both targets prepare native dependencies through `paxd-native-libs`. The test target runs the root Go packages with tests in groups, including this package, with race detection and coverage. Its registry tests read the kernel declarations, the registry documentation, and the SDK sources from the repository; the decoding tests also invoke `python3`.

## Configuration

The package reads no environment variables or runtime configuration files directly. Callers supply the ABI, executors, gas tables, retirement options, and keeper access. Upgrade checks read on-chain plan state through the upgrade keeper. Pointer runtime loading reads the embedded [pointer_artifacts/manifest.json](pointer_artifacts/manifest.json) and binaries; these are compiled assets.

## Documentation

- [Precompile ABIs and availability](../../docs/system/precompiles.md).
- [Upgrade plan and precompile activation](../../docs/release/7.0.0-upgrade-plan.md).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Documentation site: assets and pointers](../../docs/site/paxeer-docs/src/app/assets/page.tsx).
