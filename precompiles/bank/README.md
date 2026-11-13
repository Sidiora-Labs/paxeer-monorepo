# Bank precompile

This directory implements the bank precompile in the Paxeer node, exposing the native bank ledger to EVM contracts through `IBank`. It provides denomination balances, metadata and supply reads, transfers authorized by registered native ERC-20 pointers, and payable `sendNative` transfers to Bech32 recipients. The context-hash `send` overload emits `LayerXReceipt` for nonzero transfers when the merge plan is active. Execution is part of the network's transaction processing; finality is block commit.

## Layout

- [bank.go](bank.go): executor, keeper integration, transfer authorization, queries, gas accounting and upgrade-dependent behavior.
- [Bank.sol](Bank.sol): Solidity `IBank` interface and bank precompile constants.
- [abi.json](abi.json): ABI embedded in the Go implementation, including the `LayerXReceipt` event.
- [setup.go](setup.go): generated constructors for versioned bank precompiles.
- [versions](versions): bank precompile version list.
- [legacy/](legacy/): version-specific Go implementations and embedded ABIs.
- [bank_test.go](bank_test.go): balance, metadata, transfer, context-hash receipt and insufficient-funds tests.
- [abi_freeze_test.go](abi_freeze_test.go): ABI signatures, selectors, event topics and constructor checks.
- [bank_call_kind_test.go](bank_call_kind_test.go): static-call and delegate-call refusal tests.
- [bank_gate_test.go](bank_gate_test.go): plan-gating behavior with no upgrade keeper.
- [bank_paused_test.go](bank_paused_test.go): disabled-denomination transfer checks.
- [bank_send_native_test.go](bank_send_native_test.go): native transfer value and recipient validation.
- [bank_v6120_test.go](bank_v6120_test.go): agreement between bank and node upgrade plan names.
- [gas_golden_test.go](gas_golden_test.go): gas charges and invalid-selector checks.
- [proof_0x1001_bank_test.go](proof_0x1001_bank_test.go): node transaction, receipt, balance and storage-reopening checks.
- [repair_m1280_test.go](repair_m1280_test.go): committed bank balances after reopening storage.
- [resolution_reason_test.go](resolution_reason_test.go): address-resolution and ABI refusal checks.

## Build and test

The package belongs to the root [Go module](../../go.mod) and has no local Makefile. Run these commands from the repository root:

```sh
make -f chain.mk build
make -f chain.mk test
```

The [chain Makefile](../../chain.mk) builds `build/paxd`, including this precompile and its native library prerequisites. Its test target splits the root Go packages into groups and runs their tests, including the bank package, with race detection and coverage.

## Configuration

The bank implementation reads no environment variables or runtime configuration files. Node keepers provide bank state, address associations, pointer registrations and upgrade activation state. `abi.json` is embedded at build time. Bank amounts use integer denomination units; `decimals` returns zero. The `v6.12.0` plan controls insufficient-funds behavior for the four-argument `send`, and the merge plan controls the context-hash overload and coded refusal reasons.

## Documentation

- [Assets and tokens](../../docs/wiki/Assets.md): bank units, native pointers and transfers.
- [Bank ABI reference](../../docs/system/precompiles.md#bank-abijson): functions and events.
- [Precompiles on the documentation site](../../docs/site/paxeer-docs/public/markdown/precompiles.md#bank-balances-and-transfers): bank calls and native value transfers.
