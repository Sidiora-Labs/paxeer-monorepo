# EVM module

This directory implements Ethereum transaction execution within the Paxeer BFT network running `paxd`. It connects EVM execution to SDK state, associates Pax and EVM addresses, manages gas, fees, receipts and logs, and provides pointer contracts for interoperability with native tokens and CosmWasm. The kernel is part of the network and proposes blocks; receipts become final when their blocks commit. Mainnet-beta uses EVM chain ID 125 (`0x7d`), and tenant sub-chains use separate database domains within the same network.

## Layout

- [ante/](ante/) — transaction admission, signature recovery, address association, fee checks and gas accounting.
- [artifacts/](artifacts/) — embedded pointer-contract ABIs, bytecode, CosmWasm wrappers and Go bindings.
- [blocktest/](blocktest/) — configuration for Ethereum block-test execution.
- [client/](client/) — CLI transaction and query commands, plus CosmWasm query bindings and message encoding.
- [config/](config/) — Cosmos-to-EVM chain-ID mapping and live-chain checks.
- [derived/](derived/) — signer metadata derived during transaction preprocessing.
- [keeper/](keeper/) — execution, persistent state access, fees, address mappings, pointers, precompile selection, receipts and block hooks.
- [migrations/](migrations/) — module parameter, pointer and stored-state migrations.
- [querier/](querier/) — EVM query gas-limit configuration.
- [replay/](replay/) — Ethereum replay configuration.
- [state/](state/) — go-ethereum StateDB implementation over SDK stores, including snapshots, journals, balances and transient state.
- [types/](types/) — messages, transaction encodings, parameters, genesis types, codecs and generated service bindings.
- [module.go](module.go) — SDK module registration, services, migrations and lifecycle hooks.
- [genesis.go](genesis.go) — genesis import, streaming import and export.
- [handler.go](handler.go) — SDK message and governance proposal dispatch.
- [gov.go](gov.go) — governance handlers for pointer registration and binding.
- [integration_test.go](integration_test.go) — contract and pointer integration tests.
- [pointer_abi.json](pointer_abi.json) — pointer ABI embedded by the integration tests.
- [AGENTS.md](AGENTS.md) — detailed module concepts and execution lifecycle reference.

## Build and test

The directory is part of the root [Go module](../../go.mod). Run these commands from the repository root:

```sh
make paxd-build
make test-paxd-domain
```

The root [Makefile](../../Makefile) defines `paxd-build` to build the node and its native libraries. `test-paxd-domain` builds the native libraries and runs host, node, EVM keeper and RPC tests.

The chain-wide Go test target also includes the other EVM packages and their tests:

```sh
make -f chain.mk test
```

[chain.mk](../../chain.mk) splits packages with Go tests into groups and runs them with race detection and coverage. [artifacts/README](artifacts/README) describes the Solidity compilation and Go binding generation commands for pointer artifacts; their Solidity sources are in [contracts/src/](../../contracts/src/).

## Configuration

Runtime options come from the node's `app.toml`, whose sections are defined in [the node configuration template](../../daemon/paxd/cmd/root.go) and [application configuration types](../../daemon/paxd/cmd/app_config.go). The module reads these option names:

- [querier/config.go](querier/config.go): `evm_query.evm_query_gas_limit`.
- [replay/config.go](replay/config.go): `eth_replay.eth_replay_enabled`, `eth_replay.eth_rpc`, `eth_replay.eth_data_dir` and `eth_replay.contract_state_checks`.
- [blocktest/config.go](blocktest/config.go): `eth_blocktest.eth_blocktest_enabled` and `eth_blocktest.eth_blocktest_test_data_path`.

On-chain module parameters and state are loaded from the EVM portion of `genesis.json` through [genesis.go](genesis.go) and [types/genesis.go](types/genesis.go). [types/params.go](types/params.go) defines fee, gas, pointer and fee-token parameters; [types/config.go](types/config.go) defines Ethereum fork configuration. [config/config.go](config/config.go) maps the network's Cosmos chain ID to its EVM chain ID.

## Documentation

- [Chain modules and parameters](../../docs/system/chain-modules.md) — EVM parameters, messages and governance proposals.
- [Precompile ABIs](../../docs/system/precompiles.md) — precompile interfaces and activation rules.
- [EVM JSON-RPC method behavior](../../docs/evm_jsonrpc_unsupported.md) — explicit refusals and method-specific behavior.
- [EVM module site page](../../docs/site/paxeer-docs/src/app/evm/page.tsx) and [Markdown page](../../docs/site/paxeer-docs/public/markdown/evm.md).
- [Query the EVM](../../docs/site/paxeer-docs/public/markdown/querying-evm.md) — balance, contract, receipt and log queries.
