<!-- Source: https://docs.paxeer.app/unified-network/ -->

# Unified Network Architecture

How Paxeer X combines EVM and Cosmos execution with LayerX activities, identity, programs, and receipts in one BFT network.

## The Network Model

Paxeer X is one BFT network with EVM chain ID `125` (`0x7d`). The node binary `paxd` runs the Cosmos SDK application and native EVM. The LayerX kernel runs in-process inside `paxd` through `modules/lxphost`, and `node/sequencer` supplies execution ordering within the node. There is no separate kernel daemon to connect to.

Kernel state changes commit with the rest of the node's transaction state. A receipt is final once its block is committed. There is no later checkpoint registration or external settlement step required for finality. The standalone kernel daemon and the external checkpoint settlement model are retired.

Sub-chains are tenant domains stored in the database and managed by `modules/layerxchains`. Creating a domain namespaces state within the shared network. It does not require new node infrastructure.

## State and Authority

| State | Authoritative owner | How applications use it |
| --- | --- | --- |
| EVM contracts and Cosmos balances | Paxeer application state committed by chain consensus | Contract calls, transactions, native modules and registered token pointers |
| Agent identities and delegated authority | LayerX identity kernel | DIDs, primary keys, sessions, scoped grants, recovery and revocation |
| Economic activity and balances | Kernel state committed in `paxd` blocks | Receipts, replay, module queries and state witnesses |
| Receipt finality | Committed block | `pax_sequencerReceipt` rungs: `submitted`, `receipted`, `committed` |
| Tenant domains | `modules/layerxchains` | Namespaced state within the shared network |
| Indexes and user interfaces | Projections of committed evidence | Search, account history and operational views, with receipts retained for verification |

An address binding identifies the same participant across the EVM and LayerX identity systems. Grants still determine the actions that participant or its delegates can perform. A verified signature establishes who signed evidence. Authorization and application policy establish whether that evidence is acceptable for a specific operation.

## From Intent to Committed Activity

### Canonical Signed Activities

A LayerX state change uses an LXC/1 binary envelope containing protocol and network identifiers, activity type, actor DID, authority, account sequence, timestamp bounds, idempotency key, fee limit, payload binding, and signature. Fixed-width integers and canonical decoding keep independent executors in agreement. Time checks use the batch timestamp.

The kernel resolves authority, checks sequence and payload integrity, computes the fee, and executes the module inside a state journal. Successful execution commits its effects. An admitted operation that fails rolls back module effects while still recording its fee, sequence, and receipt. An admission refusal consumes neither fee nor sequence. An idempotent retry returns the original receipt without repeating the economic effect.

### Receipts, Replay and State Roots

Every admitted activity receives a global sequence and a signed receipt linked to the previous and resulting state roots. The node executes transactions before proposing them in receipt order. The signed receipt records execution. The committed block establishes finality. `pax_sequencerReceipt` distinguishes a known transaction without a receipt (`submitted`), a signed receipt awaiting commitment (`receipted`), and a receipt covered by the committed block's sequence batch (`committed`). `px_getReceiptKeys` lists the node's pinned receipt keys.

For independent verification, authenticate the block header and CometBFT commit, then verify transaction or state inclusion against the corresponding authenticated root. A successful submission or signed receipt alone must not be treated as a committed result.

## Native Precompiles

Contracts reach native modules through `precompiles/`. The merge plan is `v7.0`. Its address table contains thirteen entries, listed below with the base and versioned interfaces most relevant to this architecture. Select the interface for the upgrade active at the queried height.

| Precompile | Address suffix | Role |
| --- | --- | --- |
| Addr | `0x1004` | Resolve Paxeer and EVM identities; bind and resolve LayerX DIDs and unified accounts. |
| Perps | `0x1015` | Perpetuals interface served at this address in the v6.12.0 set, in place of the earlier `layerxExchange`. |
| LayerXBridge | `0x1016` | Mint and burn bridged assets under registered-chain, attestor, threshold and cap rules. |
| Launchpad | `0x1017` | Second-generation token launch interface in the v6.12.0 set. |
| FeeToken / XWeb | `0x1018` / `0x1019` | Versioned interfaces introduced at v6.7 and v6.8. |
| layerxledger, layerxpay, layerxauth, layerxprograms, layerxchains | `0x101A`–`0x101E` | Merge precompiles for ledger, payment, authorization, programs, and tenant domains. |
| orob, clearing, oracle aggregator, pofq, scheduler, streams, tee attestor, eip712 | `0x0901`–`0x0908` | Remaining merge precompiles. |
| Pricefeed and spot | `0x1020`–`0x1025` | `pricefeed` at `0x1020`; `spot_pecor`, `spot_orders`, `spot_stoporders`, `spot_quoter` and `spot_vault` at `0x1021`–`0x1025` in the v6.12.0 set. |

Each suffix identifies a 20-byte address padded with leading zeros. See [Precompiles](https://docs.paxeer.app/precompiles) for contract interfaces.

The pre-merge `layerxVerify` (`0x1012`) and `layerxCustody` (`0x1013`) interfaces are retired at the merge. `layerxAnchor` (`0x1014`) rejects writes after the merge. Its remaining historical views retire at `v7.0-anchor-store-delete`. These historical interfaces do not provide current funding or finality paths. The historical anchor status codes were `0` unknown, `1` submitted, and `2` final. They describe the retired ladder only.

## Unified Identity

A LayerX identity is rendered as `did:layerx:` followed by its 32-byte Ed25519 public key in lowercase hexadecimal. The canonical main account name is `agent:did:layerx:<public-key>:main`. The address precompile resolves the EVM address, Paxeer address, DID key, and derived LayerX main account together.

Binding requires both keys: the DID signs the binding domain, EVM chain ID, EVM address, and nonce; the EVM account submits the transaction. Bindings are one-to-one. Unbinding consumes a nonce, and previously signed binding messages cannot be replayed as a fresh association.

## Finality

1. **Submit:** A signed activity or transaction is known to the sequencer (`submitted`).
2. **Execute:** The kernel executes it and issues a signed receipt (`receipted`).
3. **Commit:** The block containing its sequence batch commits (`committed`). The receipt is final at this point.

The external checkpoint registry, bonded guarantor attestations, and L0–L4 settlement ladder are retired. Archived certificates describe that former protocol, not the current network's finality. The retained anchor module refuses new checkpoint submissions. See [Checkpoints](https://docs.paxeer.app/checkpoints) and [Finality](https://docs.paxeer.app/finality).

## Developer and Service Boundaries

EVM applications use Ethereum-compatible tooling and JSON-RPC. Public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`. Method availability depends on node configuration. The hosted gateway exposes a separate `px_` API. Its deployment URL and credentials come from the operator. Gateway method availability does not follow from public node URLs.

Workflows carry the actor identity, intended action, and resulting evidence through routing, execution, and indexing. A unified experience can present one account and one history while retaining the receipt and EVM transaction identifiers needed to audit each step.

## Build Across the Whole System

- Compose agent commerce from identity, budgets, escrow, delivery attestations and receipt-backed payment.

- Build programmable businesses with WASM programs, program-owned accounts and bounded spending grants.

- Connect Solidity applications to verified agent execution and committed LayerX state.

- Combine token launches, spot and perps activity, and external-chain assets under their own verification rules.

Continue with [Paxeer X and LayerX](https://docs.paxeer.app/paxeer-vs-layerx), [Modules](https://docs.paxeer.app/modules), [Contracts](https://docs.paxeer.app/contracts), and [Consensus](https://docs.paxeer.app/consensus).
