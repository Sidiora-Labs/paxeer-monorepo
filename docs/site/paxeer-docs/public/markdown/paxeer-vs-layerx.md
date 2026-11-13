<!-- Source: https://docs.paxeer.app/paxeer-vs-layerx/ -->

# Paxeer X and LayerX

One BFT network combines EVM smart contracts, autonomous agent execution, and programmable finance, with finality at block commit.

## One Network, Kernel In-Process

Paxeer X is one BFT network with EVM chain ID `125`. `paxd` supplies the Cosmos SDK chain and native EVM. The LayerX kernel is embedded in the node through `modules/lxphost` and participates in block proposal and deterministic execution. `node/sequencer` supplies execution ordering within the node. Applications compose both through native EVM precompiles and protocol receipts.

The kernel runs inside `paxd`. Native wallet payments submit signed EVM transactions to the ledger precompile; protocol activity authorization binds the payment terms. Kernel state changes commit with the rest of the node's transaction state. A receipt is final once its block is committed. There is no later checkpoint or settlement step.

| Capability | Paxeer surface | LayerX surface |
| --- | --- | --- |
| Application execution | Solidity contracts, native precompiles, Cosmos modules | Signed activities, economic modules, deterministic WASM programs |
| Identity | EVM and Paxeer addresses | Agent DIDs, sessions, scoped grants, recovery and revocation |
| Money | Native assets and bank balances | Payments, holds, escrow, budgets, streams and protocol balances |
| Markets | `perps` at `0x1015`, `pricefeed` at `0x1020`, spot at `0x1021`–`0x1025`, launchpad at `0x1017`, bridge at `0x1016` | Kernel economic operations; the EVM perpetuals venue is implemented in `modules/perps` |
| Evidence | Committed blocks; `pax_sequencerReceipt` | Signed receipts, ordered batches, state witnesses and replay |
| Security authority | Validators and governance under BFT consensus | Actor authority, deterministic execution and independently replayed activity |

## Applications on the Shared Network

### Agent Financial Controls

An agent operates through its own identity and bounded authority. It pays for services, commits funds to escrow, spends against a budget, and receives delivery evidence. Businesses issue assets and agents trade the resulting instruments. Programs coordinate these operations while the protocol controls every balance change.

### Solidity Applications That Understand Agent Activity

A contract consumes a verified service receipt, proves a LayerX account balance, or checks a program result before applying its own rules. The merge precompiles at `0x101A`–`0x101E` and `0x0901`–`0x0908` expose ledger, payment, authorization, program, and tenant-domain surfaces. Select the interface for the upgrade active at the queried height.

### Markets and Assets

`perps` is served at `0x1015`. Native applications also include `launchpad` at `0x1017`, `pricefeed` at `0x1020`, and `spot_pecor`, `spot_orders`, `spot_stoporders`, `spot_quoter` and `spot_vault` at `0x1021`–`0x1025`. Registered external chains connect through `layerxBridge` at `0x1016`. Each path retains its own economic and verification rules.

## A Shared Account, Explicit Authority

The address precompile binds an EVM address to a `did:layerx` identity with consent from both keys. The DID signs a chain-bound, address-bound message with a nonce; the EVM owner submits it. Unified account resolution connects the EVM address, Paxeer address, DID public key, and LayerX main account.

Binding makes identity discoverable across surfaces. Spending remains subject to the authority of the operation: an EVM transaction, a LayerX session or scoped grant, or a program spending grant. Revocation and expiry constrain delegated actions.

## Evidence and Finality

1. Execute payments, trades, deliveries, or program calls as signed activities or EVM transactions.
2. Receive a signed execution receipt. `pax_sequencerReceipt` reports `submitted`, `receipted`, or `committed`.
3. Treat the result as final only at `committed`, when the block covering its sequence batch has committed.

`modules/lxphost` stages kernel writes in the transaction store, and the node commits them with the block. Receipt verification uses the authorized receipt key and evidence for the containing committed block. EVM callers use the ABIs in `precompiles/` for ledger, payment, authorization and program operations.

The native asset pays EVM transaction gas. Its human-unit identifier in node parameters is `hpx`, with base unit `uhpx`. LayerX activities use their committed fee schedule and declared fee limits. Applications show the activity fee and EVM gas at the corresponding step.

## Choose Your Starting Point

- Read [Unified Network Architecture](https://docs.paxeer.app/unified-network) for state, proof and authority boundaries.

- Build contracts with [Foundry](https://docs.paxeer.app/foundry) or [Hardhat](https://docs.paxeer.app/hardhat), then explore [native precompiles](https://docs.paxeer.app/precompiles).

- Explore [protocol modules](https://docs.paxeer.app/modules) and the [WASM runtime](https://docs.paxeer.app/wasm-runtime) for programmable execution.

- Use [Network Parameters](https://docs.paxeer.app/network-parameters) for chain identity and [Building a Frontend](https://docs.paxeer.app/building-a-frontend) for wallet integration.
