# Paxeer network tests

This directory holds native kernel tests, chain and API integration suites, SDK checks, contract tests, shared harnesses, and fixtures for the Paxeer network. The suites exercise execution, encoding, balances, authority, programs, storage recovery, and consumer behavior. The kernel participates in block proposal inside `paxd`; receipts become final when their blocks are committed.

## Layout

| Directory | Contents |
| --- | --- |
| [agent/](agent/) | Python agent SDK tests and receipt evidence. |
| [arith/](arith/) | Checked integer arithmetic, rounding, conservation, and compiler rejection cases. |
| [bridge/](bridge/) | Contract boundary fixtures and source retirement assertions. |
| [chain/](chain/) | Go chain tests for distribution, epochs, minting, fees, and sequencing. |
| [ci/](ci/) | Repository consistency tests, suite runners, and their input data. |
| [codec/](codec/) | Canonical encoding, decoding, binding, and state-root tests. |
| [conformance/](conformance/) | Receipt-vector consistency checks. |
| [crypto/](crypto/) | Hashing, signatures, account keys, Merkle proofs, and cryptographic vectors. |
| [daemon/](daemon/) | Node protocol ownership, handover, and recorded execution fixtures. |
| [explorer/](explorer/) | Explorer data-boundary and shell-script tests. |
| [fixtures/](fixtures/) | Genesis, asset, authority, custody, governance, program, contract, and web fixtures. |
| [fuzz/](fuzz/) | Fuzz targets, property tests, and seed inputs. |
| [integration/](integration/) | YAML-driven chain scenarios, Go and Python runners, dApp tests, and RPC suites. |
| [ledger/](ledger/) | Accounts, transfers, send and receive behavior, receipts, and ledger write ownership. |
| [modules/](modules/) | Asset, budget, escrow, governance, oracle, trading, service, and stream tests. |
| [network/](network/) | Verification-pool and gateway key-value tests. |
| [occ/](occ/) | Go optimistic concurrency control tests and message helpers. |
| [oracle/](oracle/) | Native object checks for oracle adapter network isolation. |
| [platform/](platform/) | Emulator and gateway maintenance-head checks. |
| [programs/](programs/) | Program registration, execution, lifecycle, metering, ABI, and custody tests. |
| [proof/](proof/) | Public C header and producer-to-consumer proof cases. |
| [protocol/](protocol/) | Protocol results, activity envelopes, kernel dispatch, host migration, and policy checks. |
| [qualification/](qualification/) | Arithmetic, replay, fault, wallet capability, governance, and entitlement checks. |
| [replay/](replay/) | Golden replay test and its activity history. |
| [sdk/](sdk/) | TypeScript program terminal-result tests. |
| [sequencer/](sequencer/) | Activity admission tests. |
| [solidity/](solidity/) | Foundry tests for names and governance contracts. |
| [state/](state/) | Journaling, identities, grants, authority, allowances, revocation, and rotation tests. |
| [storage/](storage/) | Log durability, recovery, projections, and rebuild tests. |
| [support/](support/) | Genesis generation and shared metadata readers. |
| [testutil/](testutil/) | Go keeper, process-block, state, and fixture helpers. |
| [vectors/](vectors/) | Shared codec, signature, chain, budget, checkpoint, program, and replay vectors. |
| [wallet/](wallet/) | Wallet deployment, live-check, gate, and secret-scanning script tests. |
| [workspace/](workspace/) | Dashboard shell interaction tests. |

| File | Contents |
| --- | --- |
| [lxp_test_harness.h](lxp_test_harness.h) | Shared C case registration and assertion declarations. |
| [lxp_test_harness.c](lxp_test_harness.c) | C case runner and assertion implementation. |
| [explorer_fixture.c](explorer_fixture.c) | Native explorer fixture producer. |
| [explorer_fixture_golden.json](explorer_fixture_golden.json) | Expected explorer fixture hash and signature. |
| [explorer_fixture_test.py](explorer_fixture_test.py) | Fixture encoding and signature checks. |
| [test_authority_multisig.c](test_authority_multisig.c) | Multisignature authority tests. |
| [test_batch_header.c](test_batch_header.c) | Batch header encoding and hashing tests. |
| [test_beta_withdrawal_genesis.py](test_beta_withdrawal_genesis.py) | Withdrawal genesis generation tests. |
| [test_bridge_deposit.c](test_bridge_deposit.c) | Deposit behavior tests. |
| [test_bridge_withdraw.c](test_bridge_withdraw.c) | Withdrawal behavior tests. |
| [test_fee_replay.c](test_fee_replay.c) | Fee replay tests. |
| [test_fees.c](test_fees.c) | Fee calculation tests. |
| [test_fees_v3.c](test_fees_v3.c) | Fee parameter encoding and behavior tests. |
| [test_fees_v4.c](test_fees_v4.c) | Fee parameter encoding and behavior tests. |
| [test_fuzz_smoke.c](test_fuzz_smoke.c) | Fuzz smoke-test entry point. |
| [test_gateway_requirement.c](test_gateway_requirement.c) | Gateway requirement tests. |
| [test_gateway_send.c](test_gateway_send.c) | Gateway send tests. |
| [test_genesis_builder_cli.c](test_genesis_builder_cli.c) | Genesis builder command-line tests. |
| [test_genesis_import.c](test_genesis_import.c) | Genesis import tests. |
| [test_genesis_manifest.c](test_genesis_manifest.c) | Genesis manifest tests. |
| [test_genesis_module_table.c](test_genesis_module_table.c) | Genesis module table tests. |
| [test_genesis_reconcile.c](test_genesis_reconcile.c) | Genesis reconciliation tests. |
| [test_governance_activation.c](test_governance_activation.c) | Governance activation tests. |
| [test_governance_emergency.c](test_governance_emergency.c) | Emergency governance tests. |
| [test_governance_params.c](test_governance_params.c) | Governance parameter tests. |
| [test_legacy_readonly.c](test_legacy_readonly.c) | Read-only stream and write-refusal tests. |
| [test_lxgb_producers.py](test_lxgb_producers.py) | Genesis bundle producer tests. |
| [test_metering.c](test_metering.c) | Execution metering tests. |
| [test_programs_oracle_read.c](test_programs_oracle_read.c) | Program oracle-read tests. |
| [test_programs_web_read.c](test_programs_web_read.c) | Program web-read tests. |
| [test_qual_arith.c](test_qual_arith.c) | Arithmetic qualification entry point. |
| [test_qual_recovery.c](test_qual_recovery.c) | Storage recovery qualification entry point. |
| [test_qual_replay.c](test_qual_replay.c) | Replay qualification entry point. |
| [test_qual_replay_tail.c](test_qual_replay_tail.c) | Replay tail tests. |
| [test_reserve_reconcile.c](test_reserve_reconcile.c) | Reserve reconciliation tests. |
| [test_shadow_replay.c](test_shadow_replay.c) | Replay outcome comparison tests. |
| [test_snapshot.c](test_snapshot.c) | Snapshot tests. |
| [test_state_commitment_transition.c](test_state_commitment_transition.c) | State commitment transition tests. |
| [test_web_program_path.c](test_web_program_path.c) | Web program execution-path tests. |

## Build and test

The native tests use the root [Makefile](../Makefile). Run these commands from the repository root with the relevant C, Go, Rust, Python, and Foundry toolchains and native dependencies installed; see [Qualification](../docs/QUALIFICATION.md) for prerequisites and broader coverage.

| Command | Purpose |
| --- | --- |
| `make build` | Build the native kernel library used by C tests. |
| `make list-tests` | Build the C harness and list its registered cases. |
| `make test-harness` | Build and run the C harness smoke cases. |
| `make test` | Build and run the native suites selected by the root target. |
| `make test-genesis` | Build and run the genesis bootstrap tests. |
| `make test-programs-metered-call` | Build and run the metered program-call test. |
| `make test-tests-python` | Run the selected Python repository and genesis tests and supporting shell tests. |
| `make test-python-suites` | Build native fixture prerequisites and invoke the Python suite runner. |
| `make test-contracts` | Check the contract state surface and run the names and governance Foundry profiles. |
| `make paxeer-test` | Invoke the Go test groups defined in [chain.mk](../chain.mk). |

The [integration guide](integration/README.md) describes local cluster setup and YAML runner usage. Integration scenarios transact against a disposable local chain. Their prerequisites include Docker and Docker Compose; the Python runner also uses PyYAML.

The [RPC suite manifest](integration/rpc_tests/package.json) provides these commands, run from `tests/integration/rpc_tests`:

```sh
npm ci
npm run compile
npm run rpc:ci
```

The RPC runner expects a running local Paxeer cluster, prepares a local geth reference, and runs bootstrap and method tests. The [RPC suite guide](integration/rpc_tests/README.md) describes endpoint configuration and manual commands. Contract examples and fixture data under [fixtures/](fixtures/) have their own manifests where compilation is needed.

## Configuration

The root Makefile accepts `CC`, `AR`, `BUILD_DIR`, `OPT_LEVEL`, `EXTRA_CFLAGS`, `EXTRA_LDFLAGS`, and `RUN_PREFIX`. The Python targets set `PYTHONPATH`, `LAYERX_TEST_LIBRARY`, and `LAYERX_TEST_BUILD_DIR` for SDK imports and native fixture discovery. The suite runner reads [ci/data/python_suite_exclusions.txt](ci/data/python_suite_exclusions.txt) and [ci/data/python_suite_baseline.txt](ci/data/python_suite_baseline.txt).

The root [foundry.toml](../foundry.toml) selects Solidity source and test directories, compiler settings, and filesystem permissions. `FOUNDRY_PROFILE` selects the contract profile.

The RPC suite reads [integration/rpc_tests/config/endpoints.ts](integration/rpc_tests/config/endpoints.ts), [integration/rpc_tests/hardhat.config.ts](integration/rpc_tests/hardhat.config.ts), [integration/rpc_tests/.mocharc.bootstrap.json](integration/rpc_tests/.mocharc.bootstrap.json), and [integration/rpc_tests/.mocharc.run.json](integration/rpc_tests/.mocharc.run.json). Its environment settings include:

- Node endpoints: `PAX_EVM_RPC`, `PAX_EVM_WS`, `PAX_COSMOS_RPC`, and `PAX_REST`.
- Reference endpoints and account setup: `RPC_ETH_GETH`, `RPC_ETH_FORK`, `RPC_ACCOUNTLESS`, and `PAX_ADMIN_MNEMONIC`.
- Runner settings: `RPC_TESTS_RUNTIME_STATE`, `PAX_TIMEOUT`, `GETH_TIMEOUT`, `GETH_VERSION`, and `SKIP_NPM_CI`.
- Chain expectations and polling: `RPC_EXPECTED_EVM_CHAIN_ID`, `RPC_NETWORK_MODE`, `RPC_POLLING_INTERVAL_MS`, and `RPC_TESTS_WASM_MODE`.
- Optional fork settings in [integration/rpc_tests/hardhat/hardhat.config.ts](integration/rpc_tests/hardhat/hardhat.config.ts): `ETH_MAINNET_UPSTREAM` and `ETH_MAINNET_FORK_BLOCK`.

The dApp suite's [integration/dapp_tests/hardhat.config.js](integration/dapp_tests/hardhat.config.js) reads `DAPP_TESTS_MNEMONIC`, `PAXEER_LOCAL_EVM_RPC_URL`, `PAXEER_TESTNET_EVM_RPC_URL`, and `PAXEER_DEVNET_EVM_RPC_URL`; its scenarios read `DAPP_TEST_ENV`. See the [dApp suite guide](integration/dapp_tests/README.md) for scenario setup.

## Documentation

- [Network qualification and suite prerequisites](../docs/QUALIFICATION.md).
- [Monorepo layout and build boundaries](../docs/MONOREPO.md).
- [Network overview](../docs/paxeer-network.md).
- [Integration framework](integration/README.md).
- [RPC parity suite](integration/rpc_tests/README.md).
- [EVM RPC input/output fixtures](integration/evm_module/rpc_io_test/RPC_IO_README.md).
- [Documentation site: Foundry contract builds and tests](../docs/site/paxeer-docs/public/markdown/foundry.md).
- [Documentation site: Hardhat development](../docs/site/paxeer-docs/public/markdown/hardhat.md).
