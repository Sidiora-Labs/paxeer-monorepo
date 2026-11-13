# LayerX Chains precompile

This directory contains the Go implementation, Solidity interface, ABI and tests for the `layerxchains` EVM precompile. It exposes the tenant-domain registry in [`modules/layerxchains`](../../modules/layerxchains): callers can create and fund domains, manage their lifecycle and operators, and query registry records and parameters. Tenant domains share Paxeer's consensus and EVM state and require no additional infrastructure. The precompile is served after merge activation at the address declared in [`address.go`](address.go).

## Layout

- [`LayerXChains.sol`](LayerXChains.sol) — Solidity interface, precompile address, methods and events.
- [`abi.json`](abi.json) — ABI embedded by the Go implementation.
- [`address.go`](address.go) — precompile address and name constants.
- [`layerxchains.go`](layerxchains.go) — constructor registration, dynamic gas accounting, payment handling, method dispatch and registry queries.
- [`keepers.go`](keepers.go) — registry and EVM keeper dependencies.
- [`gas.go`](gas.go) — method gas table.
- [`events.go`](events.go) — EVM event topics and data encoding.
- [`errors.go`](errors.go) — registry errors translated into LXP revert reasons.
- [`abi_test.go`](abi_test.go) — ABI, Solidity interface, gas table and encoding-vector checks.
- [`layerxchains_test.go`](layerxchains_test.go) — EVM calls through the node's keepers, covering registration, payments, lifecycle, authorization, events and queries.

## Build and test

This package belongs to the root Go module in [`go.mod`](../../go.mod) and has no separate build target. From the repository root, use the targets in the root [`Makefile`](../../Makefile):

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` delegates to [`chain.mk`](../../chain.mk) to build `paxd`, including its native library prerequisites. `paxeer-test` delegates to the chain test groups, which include this package's Go tests. Both targets use the local Go dependency cache with `GOPROXY` disabled for network lookup.

## Configuration

The runtime implementation reads no environment variables or external configuration files. It embeds `abi.json` and receives the registry, bank and EVM keepers from the node. Registry settings come from the keeper's on-chain parameters; `params()` exposes the authority, minimum bond, creation fee, gas allowance bounds, remaining total gas allowance and maximum chain count.

The ABI vector test reads `UPDATE_VECTORS` to control regeneration of [`tests/vectors/chains/abi-vectors.json`](../../tests/vectors/chains/abi-vectors.json). This variable affects tests only.

## Documentation

- [Virtual chains: registry, lifecycle, allowances and client calls](../../docs/wiki/VirtualChains.md)
- [Precompile ABI catalogue](../../docs/system/precompiles.md#layerxchains-abijson)
- [Tenant-domain module parameters](../../docs/system/chain-modules.md#layerxchains)
- [Network architecture and virtual chains](../../docs/paxeer-network.md#virtual-chains)
- [Documentation site: EVM precompiles](../../docs/site/paxeer-docs/public/markdown/precompiles.md)
