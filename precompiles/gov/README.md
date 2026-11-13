# Governance precompile

This directory contains the Go implementation, EVM ABI, Solidity interface, and tests for the governance precompile. It connects EVM calls to governance in `paxd` on the merged Paxeer network through `vote`, `voteWeighted`, `deposit`, and `submitProposal`. Callers need an EVM-to-Paxeer address association; deposits and proposal submission use transaction value, while voting is nonpayable.

## Layout

- [Gov.sol](Gov.sol) — Solidity interface and weighted vote option structure.
- [abi.json](abi.json) — ABI embedded by the Go implementation.
- [gov.go](gov.go) — Construction, method dispatch, gas requirements, voting, deposits, and proposal submission.
- [handler.go](handler.go) — Proposal JSON types and handlers for text, parameter changes, software upgrades, upgrade cancellation, and community pool spending.
- [setup.go](setup.go) — Versioned precompile constructors keyed by upgrade name.
- [versions](versions) — Governance precompile version list.
- [legacy/](legacy/) — Versioned governance executors and their ABIs.
- [gov_test.go](gov_test.go) — Governance calls and proposal validation through EVM transactions.
- [abi_freeze_test.go](abi_freeze_test.go) — ABI, address, and method selector assertions.
- [gas_golden_test.go](gas_golden_test.go) — Gas requirements across merge activation.
- [repair_m1267_test.go](repair_m1267_test.go) — Parameter-change execution and empty or omitted proposal type defaults.

## Build and test

The package belongs to the root [Go module](../../go.mod) and uses the shared targets in [chain.mk](../../chain.mk). Run these commands from the repository root:

```sh
make -f chain.mk build
make -f chain.mk test
```

The build target prepares native libraries and builds `paxd`, which includes the governance precompile. The test target runs the root Go module's package test groups, including this package.

## Configuration

The package reads no environment variables or runtime configuration files. [abi.json](abi.json) is embedded at build time. The node supplies the governance message server, EVM keeper, and bank keeper to the constructor; upgrade selection uses [setup.go](setup.go).

## Documentation

- [Governance proposal types](../../docs/system/governance.md).
- [Precompile addresses and ABI reference](../../docs/system/precompiles.md).
- [Documentation site: governance](../../docs/site/paxeer-docs/src/app/governance/page.tsx).
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
