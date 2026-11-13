# Paxeer X chain modules

Chain-specific modules and integration helpers for `paxd` live here. Paxeer
mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`). The kernel
is part of this network and proposes blocks; a receipt is final when its block
is committed. Kernel execution and native balances share chain state.

## Layout

- [`README.md`](README.md) — directory overview, build commands, configuration, and documentation links.
- [`lxphost`](lxphost/) — shared cgo host for kernel validation and execution, with module state persisted in the SDK store and exported through genesis; also backs the `layerxprograms` precompile at `0x101D`.
- [`layerxledger`](layerxledger/) — asset registration, derived accounts, transfers, and exact payments through `0x101A`. Payer-grant issue, revoke, and draw operations are retired.
- [`layerxauth`](layerxauth/) — capability and session grants, revocation, and SDK authorization integration through `0x101C`.
- [`layerxpay`](layerxpay/) — escrows, budgets, streams, and service agreements through `0x101B`, subject to the configured family enablement and pause gates.
- [`layerxagent`](layerxagent/) — scheduled EVM calls, job deposits, and bounded end-block execution. Agent-function precompiles occupy `0x0901`–`0x0908`; the scheduler is at `0x0905`.
- [`layerxchains`](layerxchains/) — tenant-domain registry, funding, allowances, operator controls, and lifecycle through `0x101E`. Sub-chains are database namespaces on the same network and need no new infrastructure.
- [`sequencer`](sequencer/) — on-chain leader list, receipt keys, and sequence state used by the network's block proposal and receipt machinery.
- [`layerxgov`](layerxgov/) — governance proposal content (`LayerXProposal`) that dispatches packaged parameter and admin messages through the SDK `MsgServiceRouter`.
- [`evm`](evm/) — native EVM execution, address association, receipts, pointers, and precompile integration.
- [`perps`](perps/) — native perpetuals markets and synchronous trading through the perps precompile at `0x1015`.
- [`spot`](spot/) — native spot markets, orders, stop orders, quoters, and vaults.
- [`pricefeed`](pricefeed/) — validator-signed index and mark price observations with mid-block aggregation.
- [`layerxbridge`](layerxbridge/) — attested transfers between Paxeer and external chains: chain registry, attestor set, per-asset caps, and `bridgeIn`/`bridgeOut` through the `layerxBridge` precompile.
- [`launchpad`](launchpad/) — native bonding-curve markets over `tokenfactory` denoms through the `launchpad` precompile.
- [`xweb`](xweb/) — attested web data for EVM contracts and kernel programs; governance controls the attestor set, threshold, and pause state.
- [`oracle`](oracle/) — validator exchange-rate voting and price aggregation.
- [`epoch`](epoch/) — time-based hooks and epoch lifecycle management.
- [`mint`](mint/) — governed native-token release schedules and minting.
- [`tokenfactory`](tokenfactory/) — account-created native token denominations with admin and issuer controls.
- [`store`](store/) — module-level store integration helper `GetCachedContext`, which returns a cached context and its cache.
- [`layerxanchor`](layerxanchor/) — retired checkpoint and bond machinery behind `0x1014`, with retained types and state for compatibility. Use committed blocks for finality and committed application state with per-key proofs for state verification. Compatibility calls are governed by the merge and subsequent store-deletion gates.
- [`layerxcustody`](layerxcustody/) — custody compatibility types, state, and retirement handling behind `0x1013`. Deposits, proof-carrying withdrawals, forced exits, and deposit-root publication are retired. Fund derived sub-accounts through native bank sends; use native bank or ledger moves for withdrawals and `layerxbridge` for outward transfers.
- [`layerxexchange`](layerxexchange/) — retired exchange-intent submission and proof-based settlement machinery, with retained types and state for compatibility. Use the native `perps` module at `0x1015` for orders, positions, and margin.

The settlement compatibility directories retain historical state and retirement
checks within the same execution domain.

[`node/app.go`](../node/app.go) wires the module keepers and lifecycle hooks;
[`precompiles/setup.go`](../precompiles/setup.go) registers the EVM entry points
and their upgrade gates. Precompile ABIs live under [`precompiles/`](../precompiles/).

Framework-provided modules remain under [`sdk/x/`](../sdk/x/); interchain
applications live under [`interchain/modules/`](../interchain/modules/).

## Build and test

The modules use the repository's root [Go manifest](../go.mod) and build targets
in the root [Makefile](../Makefile) and [chain.mk](../chain.mk). Run these commands
from the repository root:

- `make paxd-build` — builds the native libraries and the `paxd` node that integrates the modules.
- `make test-lxphost` — builds the native libraries and tests the shared kernel host.
- `make test-paxd-domain` — tests the host, node, EVM keeper, and RPC integration.
- `make -f chain.mk test` — runs the chain's Go package tests, including the module packages, in the declared test groups.

## Configuration

Module genesis state comes from `genesis.json`; module parameters are stored in
chain state and updated through the registered governance handlers. The node
passes startup options from `app.toml` to the EVM configuration readers:

- [`evm/querier/config.go`](evm/querier/config.go): `evm_query.evm_query_gas_limit`.
- [`evm/replay/config.go`](evm/replay/config.go): `eth_replay.eth_replay_enabled`, `eth_replay.eth_rpc`, `eth_replay.eth_data_dir`, and `eth_replay.contract_state_checks`.
- [`evm/blocktest/config.go`](evm/blocktest/config.go): `eth_blocktest.eth_blocktest_enabled` and `eth_blocktest.eth_blocktest_test_data_path`.

The configuration sections are declared in the node's
[application configuration template](../daemon/paxd/cmd/root.go). Module sources
do not read environment variables directly.

## Documentation

- [Chain modules and parameters](../docs/system/chain-modules.md).
- [Kernel module declarations](../docs/system/kernel-modules.md).
- [Governance proposal types](../docs/system/governance.md).
- [Precompile ABIs and availability](../docs/system/precompiles.md).
- [Documentation site modules page](../docs/site/paxeer-docs/src/app/modules/page.tsx) and its [Markdown page](../docs/site/paxeer-docs/public/markdown/modules.md).
- [Public JSON-RPC](../docs/site/paxeer-docs/public/markdown/public-rpc.md), including node endpoints at the root path and chain connection details.
