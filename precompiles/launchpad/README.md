# Launchpad precompile

This directory contains the Go EVM precompile, Solidity interface, role ABIs and tests for the native Paxeer launchpad. It connects direct calls at `0x1017` and calls forwarded by registered contract surfaces to the [launchpad keeper](../../modules/launchpad/keeper/), serving market creation, trading, quotes, fee rights, fee distribution and chain launch operations within the network. Launched chains are tenant domains that share the network's state and validators.

## Layout

- [abi/](abi/) — contract role ABIs, the nested token-call ABI and the chain launch ABI.
- [legacy/](legacy/) — version-specific precompile implementation and interface.
- [testdata/](testdata/) — ABI signature fixtures.
- [Launchpad.sol](Launchpad.sol) — the direct-call `ILaunchpad` Solidity interface, events and custom errors.
- [abi.json](abi.json) — the embedded ABI for direct precompile calls.
- [abi.go](abi.go) — embeds and loads the contract role ABIs.
- [launchpad.go](launchpad.go) — precompile construction, keeper dependencies and account conversion.
- [run.go](run.go) — direct and surface call resolution, ABI decoding, gas budgets and role dispatch.
- [gas.go](gas.go) — base, calldata and state-write gas accounting.
- [router.go](router.go) — market creation, buys, sells, token-to-token swaps, permits and multicall.
- [pool.go](pool.go) — pool swaps, reserve synchronization, guardian controls and pool views.
- [views.go](views.go) — quotes, pool registry queries and protocol parameter views.
- [fees.go](fees.go) — fee accumulation, fee strategies, distributions and airdrop claims.
- [nft.go](nft.go) — fee-right NFT ownership, approvals, transfers and enumeration.
- [treasury.go](treasury.go) — treasury balances, deposits and role checks.
- [chainlaunch.go](chainlaunch.go) — chain launch, home-chain queries, revenue epochs and chain controls.
- [nested.go](nested.go) — nested EVM calls, token permits and NFT receiver callbacks.
- [hooks.go](hooks.go) — optical callbacks for swaps and fee distribution.
- [events.go](events.go) — contract-role events and launchpad event emission.
- [errors.go](errors.go) — ABI-encoded custom revert data.
- [surface.go](surface.go) — forwarding runtime and contract surface installation.
- [retired.go](retired.go) — replacement-address revert runtime installation.
- [setup.go](setup.go) — versioned precompile registration.
- [versions](versions) — precompile version keys.
- Go test files cover ABI agreement, execution, gas, surfaces, nested calls, reentrancy, contract parity and chain operations.

## Build and test

The package belongs to the root [Go module](../../go.mod). The root [Makefile](../../Makefile) supplies the targets that cover it through [chain.mk](../../chain.mk). Run these commands from the repository root:

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds the node and its native library prerequisites, including the launchpad precompile. `paxeer-test` runs the chain's Go package tests, including this directory, using the package splits in `chain.mk`. Both targets use locally available Go dependencies with network module fetching disabled. The optical tests also compile Solidity sources with `solc`.

## Configuration

The runtime reads launchpad parameters and registered surfaces from keeper state. Its direct and role ABIs are embedded from `abi.json` and `abi/*.json` at build time.

The optical tests read these environment variables:

- `LAUNCHPAD_CONTRACTS_DIR` — selects the Solidity source directory.
- `SOLC` — selects the Solidity compiler executable.

## Documentation

- [Launch a market and chain](../../docs/launchpad/chain-launch.md)
- [Precompile interfaces](../../docs/system/precompiles.md)
- [Chain modules](../../docs/system/chain-modules.md)
- [Documentation site: Launchpad](../../docs/site/paxeer-docs/public/markdown/launchpad.md)
- [Documentation site: Precompiles](../../docs/site/paxeer-docs/public/markdown/precompiles.md)
