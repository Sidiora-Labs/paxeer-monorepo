# Paxeer X chain modules

Chain-specific modules and integration helpers for `paxd` live here. Paxeer
mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`). The kernel
is part of this network and proposes blocks; a receipt is final when its block
is committed. Kernel execution and native balances share chain state.

## Merged kernel and economic modules

- [`lxphost`](lxphost/) — shared cgo host for kernel validation and execution,
  with module state persisted in the SDK store and exported through genesis;
  also backs the `layerxprograms` precompile at `0x101D`.
- [`layerxledger`](layerxledger/) — asset registration, derived accounts,
  transfers, and exact payments through `0x101A`. The old payer-grant issue,
  revoke, and draw operations are retired.
- [`layerxauth`](layerxauth/) — capability and session grants, revocation, and
  SDK authorization integration through `0x101C`.
- [`layerxpay`](layerxpay/) — escrows, budgets, streams, and service agreements
  through `0x101B`, subject to the configured family enablement and pause gates.
- [`layerxagent`](layerxagent/) — scheduled EVM calls, job deposits, and bounded
  end-block execution. Agent-function precompiles occupy `0x0901`–`0x0908`;
  the scheduler is at `0x0905`.
- [`layerxchains`](layerxchains/) — tenant-domain registry, funding, allowances,
  operator controls, and lifecycle through `0x101E`. Sub-chains are database
  namespaces on the same network and need no new infrastructure.
- [`sequencer`](sequencer/) — on-chain leader list, receipt keys, and sequence
  state used by the network's block proposal and receipt machinery.
- [`layerxgov`](layerxgov/) — governance proposal content (`LayerXProposal`)
  that dispatches packaged parameter and admin messages through the SDK
  `MsgServiceRouter`.

## Execution, markets, and data

- [`evm`](evm/) — native EVM execution, address association, receipts, pointers,
  and precompile integration.
- [`perps`](perps/) — native perpetuals markets and synchronous trading through
  the perps precompile at `0x1015`.
- [`spot`](spot/) — native spot markets, orders, stop orders, quoters, and vaults.
- [`pricefeed`](pricefeed/) — validator-signed index and mark price observations
  with mid-block aggregation.
- [`layerxbridge`](layerxbridge/) — attested transfers between Paxeer and external
  chains: chain registry, attestor set, per-asset caps, and `bridgeIn`/`bridgeOut`
  through the `layerxBridge` precompile.
- [`launchpad`](launchpad/) — native bonding-curve markets over `tokenfactory`
  denoms through the `launchpad` precompile.
- [`xweb`](xweb/) — attested web data for EVM contracts and kernel programs;
  governance controls the attestor set, threshold, and pause state.
- [`oracle`](oracle/) — validator exchange-rate voting and price aggregation.
- [`epoch`](epoch/) — time-based hooks and epoch lifecycle management.
- [`mint`](mint/) — inflation and native-token minting policy.
- [`tokenfactory`](tokenfactory/) — permissioned creation and management of native
  token denominations.
- [`store`](store/) — module-level store integration helpers.

## Retired settlement compatibility

These directories retain legacy types, historical state handling, and retirement
checks. They do not provide a separate execution or settlement domain:

- [`layerxanchor`](layerxanchor/) — retired checkpoint and bond machinery behind
  `0x1014`. Use committed blocks for finality and committed application state
  with per-key proofs for state verification. Compatibility calls are governed
  by the merge and subsequent store-deletion gates.
- [`layerxcustody`](layerxcustody/) — legacy custody registry and retirement
  handling behind `0x1013`. Deposits, proof-carrying withdrawals, forced exits,
  and deposit-root publication are retired. Fund derived sub-accounts through
  native bank sends; use native bank or ledger moves for withdrawals and
  `layerxbridge` for outward transfers.
- [`layerxexchange`](layerxexchange/) — retired exchange-intent submission and
  proof-based settlement machinery. Use the native `perps` module at `0x1015`
  for orders, positions, and margin.

## Wiring and related code

[`node/app.go`](../node/app.go) wires the module keepers and lifecycle hooks;
[`precompiles/setup.go`](../precompiles/setup.go) registers the EVM entry points
and their upgrade gates. Precompile ABIs live under [`precompiles/`](../precompiles/).

Framework-provided modules remain under [`sdk/x/`](../sdk/x/); interchain
applications live under [`interchain/modules/`](../interchain/modules/).
See [public JSON-RPC](../docs/site/docs/reference/public-rpc.md) for node endpoints
at the root path and chain connection details.
