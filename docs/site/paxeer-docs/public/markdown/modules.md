<!-- Source: https://docs.paxeer.app/modules/ -->

# Paxeer X Modules

Native execution, markets, issuance, and data modules in `modules/`, including retired settlement-compatibility directories.

**Source:** `modules/README.md`

## Unified Paxeer X modules

Paxeer X combines EVM and Cosmos execution with the LayerX kernel. The chain modules below enforce the chain-side rules; the [LayerX kernel](https://docs.paxeer.app/layerx-kernel) is part of the same network, and a receipt is final when its block is committed. Addresses below are shortened; see [precompiles](https://docs.paxeer.app/precompiles) for full addresses and upgrade gates.

| Module | Role in Paxeer X | EVM entry point |
| --- | --- | --- |
| `lxphost` | Shared cgo host for kernel validation and execution; also backs the `layerxprograms` precompile | `0x101D` |
| `layerxledger` | Asset registration, derived accounts, transfers, and exact payments. The old payer-grant issue, revoke, and draw operations are retired | `0x101A` |
| `layerxauth` | Capability and session grants, revocation, and SDK authorization integration | `0x101C` |
| `layerxpay` | Escrows, budgets, streams, and service agreements, subject to configured family enablement and pause gates | `0x101B` |
| `layerxagent` | Scheduled EVM calls, job deposits, and bounded end-block execution. Agent-function precompiles occupy `0x0901`–`0x0908`; the scheduler is at `0x0905` | `0x0901`–`0x0908` |
| `layerxchains` | Tenant-domain registry, funding, allowances, operator controls, and lifecycle. Sub-chains are database namespaces on the same network | `0x101E` |
| `sequencer` | On-chain leader list, receipt keys, and sequence state used by block proposal and receipt machinery | — |
| `layerxgov` | Governance proposal content (`LayerXProposal`) that dispatches packaged parameter and admin messages through the SDK `MsgServiceRouter` | Governance proposals |
| `perps` | Native perpetuals markets and synchronous trading: orders, positions, and margin | `0x1015` |
| `spot` | Native spot markets, orders, stop orders, quoters, and vaults | — |
| `pricefeed` | Validator-signed index and mark price observations with mid-block aggregation | — |
| `layerxbridge` | Attested transfers between Paxeer and external chains: chain registry, attestor set, per-asset caps, and `bridgeIn`/`bridgeOut` | `0x1016` |
| `launchpad` | Native bonding-curve markets over `tokenfactory` denoms | `0x1017` |
| `xweb` | Attested web data for EVM contracts and kernel programs; governance controls the attestor set, threshold, and pause state | `0x1019` |

Explore the [unified network](https://docs.paxeer.app/unified-network), [asset model](https://docs.paxeer.app/assets), [settlement lifecycle](https://docs.paxeer.app/settlement), and [bridge](https://docs.paxeer.app/bridge). `modules/README.md` describes module ownership.

## Retired settlement compatibility

These directories retain legacy types, historical state handling, and retirement checks. They do not provide a separate execution or settlement domain:

| Module | Status | EVM entry point |
| --- | --- | --- |
| `layerxcustody` | Legacy custody registry and retirement handling. Deposits, proof-carrying withdrawals, forced exits, and deposit-root publication are retired. Fund derived sub-accounts through native bank sends; use `layerxbridge` for outward transfers | `0x1013` |
| `layerxanchor` | Retired checkpoint and bond machinery. Use committed blocks for finality and committed application state with per-key proofs for state verification | `0x1014` |
| `layerxexchange` | Retired exchange-intent submission and proof-based settlement machinery. Use the native `perps` module at `0x1015` for orders, positions, and margin | `0x1015` (superseded by `perps`) |

## Module Overview

Paxeer X-specific chain modules live under `modules/`. These extend the Cosmos SDK with functionality unique to Paxeer X’s EVM L1 design. The pages in this section document `epoch`, `mint`, `oracle`, `store`, and `tokenfactory` in detail; the modules above are documented in their own sections or in `modules/README.md`.

## Module List

| Module | Purpose |
| --- | --- |
| [`evm`](https://docs.paxeer.app/evm) | Native EVM execution, address association, receipts, pointers, and precompile integration |
| [`epoch`](https://docs.paxeer.app/modules/epoch) | Time-based hooks and epoch lifecycle management |
| [`mint`](https://docs.paxeer.app/modules/mint) | Scheduled native-token minting, released daily through the epoch hook |
| [`oracle`](https://docs.paxeer.app/modules/oracle) | Validator exchange-rate voting and weighted-median price aggregation (native interfaces; the EVM precompile is retired) |
| [`store`](https://docs.paxeer.app/modules/store) | Single cached-context helper; not a registered chain module |
| [`tokenfactory`](https://docs.paxeer.app/modules/tokenfactory) | Creation and admin management of native token denominations by any sender |

## Framework Modules

Framework-provided modules (bank, staking, distribution, governance, etc.) remain under `sdk/x/`. Interchain application modules live under `interchain/modules/`.

## EVM Module

The `evm` module is Paxeer X’s core EVM execution layer:

- **EVM execution:** Runs Ethereum transactions through the configured EVM and upgrade rules

- **Address association:** Maps Cosmos addresses to EVM addresses

- **Receipts:** Generates EVM transaction receipts

- **Pointers:** Bridging between Cosmos and EVM state

- **Precompiles:** Integration with Paxeer X-specific precompiled contracts

See [EVM documentation](https://docs.paxeer.app/evm) for details.

## Epoch Module

The `epoch` module provides time-based lifecycle hooks:

- **Epoch boundaries:** Fixed time periods (60s by default) checked in `BeginBlock`

- **Hooks:** Registered modules run `AfterEpochEnd` / `BeforeEpochStart` callbacks at each boundary. Only the mint keeper is wired in `node/app.go`

See [Epoch documentation](https://docs.paxeer.app/modules/epoch) for details.

## Mint Module

The `mint` module manages native token issuance:

- **Release schedule:** A `total_mint_amount` released between a start and end date

- **Minting trigger:** Only the epoch `AfterEpochEnd` hook, which mints at most once per day (UTC date)

- **Destination:** Newly minted coins are sent to the `fee_collector` account

See [Mint documentation](https://docs.paxeer.app/modules/mint) for details.

## Oracle Module

The `oracle` module enables validator-based price feeds on native interfaces:

- **Exchange rate voting:** Validators submit price observations for configured vote targets

- **Aggregation:** Weighted-median computation in the mid-blocker at the last block of each vote period

- **Penalties:** The end-blocker processes miss/abstain counters at each slash window; counters reflect participation and in-range votes

- **EVM access:** The oracle precompile at `0x…1008` is retired and reverts; use native or CosmWasm queries

See [Oracle documentation](https://docs.paxeer.app/modules/oracle) for details.

## Store Module

`modules/store/` is a single helper file, not a registered module:

- **`GetCachedContext`:** Returns a context backed by a `CacheMultiStore` plus the cache itself. It has no keeper, types, queries, or messages

See [Store documentation](https://docs.paxeer.app/modules/store) for details.

## Token Factory Module

The `tokenfactory` module enables native token creation:

- **Native denominations:** Create new token types as `factory/{creator}/{subdenom}`

- **Creation:** `MsgCreateDenom` has no permission gate; any sender can create a denom and becomes its admin

- **Management:** The admin can mint (to itself), burn (from itself), change admin, update the allow list, set metadata, and pause/unpause the denom

See [Token Factory documentation](https://docs.paxeer.app/modules/tokenfactory) for details.

## Module Integration

Modules are registered in the application through `node/app.go`:

- **Module manager:** Coordinates module initialization and upgrades

- **Begin/end block:** Modules register hooks for block lifecycle

- **Message routing:** Module-specific transactions routed to handlers

- **Query routing:** Module-specific queries routed to keepers
