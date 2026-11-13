# LayerX tenant-domain registry

`layerxchains` is the SDK module that stores tenant-domain registrations in the
Paxeer network. It manages names, operators, bonds, gas and byte allowances,
pause and retirement controls, transaction history, gas usage and per-domain
base fees. Sub-chains are database namespaces within the shared network and
use its validators and committed blocks; creating one requires no new node
infrastructure.

## Layout

- [`module.go`](module.go) — SDK module registration, genesis validation, import and export.
- [`keeper/`](keeper/) — persistent registry, bank transfers, lifecycle controls, accounting, genesis history and keeper tests.
- [`types/`](types/) — domain records, chain-ID mapping, context helpers, parameters, store keys, errors, genesis validation and parameter tests.

## Build and test

This directory belongs to the root [Go module](../../go.mod) and uses the root
[Makefile](../../Makefile) and [chain targets](../../chain.mk). Run these commands
from the repository root:

```sh
make paxd-build
make paxeer-test
```

`paxd-build` builds the node and its native library dependencies, including this
module through the node application. `paxeer-test` runs the root Go test groups,
which include the keeper and types packages here. Their tests cover domain
lifecycle, allowance and authority checks, bank transfers, chain-ID mapping,
base-fee accounting, genesis round trips and transaction-history recovery.

## Configuration

The module receives configuration through genesis and persisted keeper
parameters. Its sources read no environment variables or standalone
configuration files. The parameter names in [`types/params.go`](types/params.go)
are `authority`, `min_bond`, `creation_fee`, `min_gas_allowance`,
`max_gas_allowance`, `max_total_gas_allowance`, `min_byte_allowance`,
`max_byte_allowance`, `max_chains` and `max_fee_share_bps`.

[`types/genesis.go`](types/genesis.go) defines the genesis fields `params`,
`domains`, `next_id` and `next_base_fees`. The keeper's
[`genesis_history.go`](keeper/genesis_history.go) adds `domain_history` for
`ExportGenesisWithHistory` and `InitGenesisWithHistory`.

## Documentation

- [Virtual chains](../../docs/wiki/VirtualChains.md) — registry operations, chain IDs, allowances and accounting.
- [Chain module reference](../../docs/system/chain-modules.md#layerxchains) — module parameters and exposed controls.
- [Launch a market and chain](../../docs/launchpad/chain-launch.md) — launchpad integration and chain management.
- [Documentation site: modules](../../docs/site/paxeer-docs/src/app/modules/page.tsx) — tenant registry in the module catalogue.
- [Documentation site: unified network](../../docs/site/paxeer-docs/src/app/unified-network/page.tsx) — tenant domains within the shared network.
