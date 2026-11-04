# EVM Module Concepts

## Overview

The `modules/evm` module integrates Ethereum transaction execution into the Pax SDK chain, with bidirectional Pax/EVM address association and interoperability between CosmWasm (CW) and EVM contracts through pointers.

Paxeer mainnet-beta is one BFT network running `paxd`. The kernel is part of that network and proposes blocks; a receipt is final once its block commits. The root EVM chain ID is **125** (`0x7d`). Public JSON-RPC uses the root URL of the public nodes, for example `https://api1.mainnet-beta.paxeer.network`; the numbered hosts `api1` through `api14.mainnet-beta.paxeer.network` and `api-mainnet-beta.paxeer.network` serve the node API. Sub-chains are tenant domains in the database, with their own registered EVM chain IDs and state namespaces.

---

## Dual Address Model

Every account can have both a Pax (bech32) address and an EVM (hex) address. The module maintains a bidirectional mapping between them.

- **Explicit association** — an Associate transaction or an accepted EVM transaction recovers the addresses from its signature and creates the mapping when needed. The Cosmos ante chain also associates signers whose public keys can be parsed as secp256k1 keys.
- **Default (cast) addresses** — [address lookups](keeper/address.go) fall back to a deterministic byte-cast when no mapping exists. After an EVM address is associated with a different Pax address, its old cast Pax address cannot receive bank transfers. A mapping to the cast address itself remains receivable, as with EVM contract addresses.

Association migrates balances from the cast account to the derived Pax account, avoiding separate balance views for the same key.

---

## EVM Execution

The [message server](keeper/msg_server.go) executes Ethereum transactions through the repository's go-ethereum integration:

1. An incoming `MsgEVMTransaction` is unpacked into a native Ethereum transaction.
2. A go-ethereum `vm.EVM` instance is created with the block and transaction context.
3. Execution runs through go-ethereum's `core.StateTransition`.
4. Gas is converted between Pax and EVM units using the configured `PriorityNormalizer`.
5. State changes are finalized into SDK stores, and logs and gas usage feed transient receipts.

Each Ethereum transaction carries a gas limit. [Basic admission checks](ante/basic.go) also reject a transaction whose gas limit exceeds a positive consensus `Block.MaxGas`; gas accounting must therefore respect the configured block limit as well as the transaction limit.

Other modules and CosmWasm contracts can invoke the EVM through [internal call messages](keeper/evm.go). Writable re-entry is constrained by the execution context: `CallEVM` rejects an EVM-to-CW-to-EVM call unless the context marks entry through the Wasmd precompile, then clears that entry flag for the internal execution. CosmWasm cannot call the Solo precompile through this path.

---

## Supported Transaction Types

| Type | EIP | Notes |
| --- | --- | --- |
| Legacy | EIP-155 | Type 0, replay-protected on normal execution paths |
| AccessList | EIP-2930 | Type 1 |
| DynamicFee | EIP-1559 | Type 2 |
| SetCode | EIP-7702 | Type 4, code delegation via authorization list, enabled with the Prague signer |
| Associate | Pax-specific | Binds Pax and EVM addresses in the root domain |

The [signer and type table](ante/preprocess.go) depends on the active EVM fork. Blob transactions (type 3) are rejected by admission with `sdkerrors.ErrUnsupportedTxType`, even though the transaction data types can represent them. Unprotected legacy transactions are rejected during signature recovery outside block-test mode.

---

## Ante Handler Pipeline

The mounted chains are defined in [node/ante.go](../../node/ante.go). The in-process `EVMRouterDecorator` selects the EVM ante chain for a `MsgEVMTransaction` and rejects an EVM transaction containing more than one message. Other transactions use the Cosmos ante chain.

The ordinary EVM ante chain runs these decorators in order:

1. **`EVMNoCosmosFieldsDecorator`** — rejects Cosmos wrapper memo, timeout height, extension options, signer information, fee amounts, payer/granter and signatures. This runs before any association can write state.
2. **`EVMChainIDDecorator`** — performs stateless checks and resolves the signed transaction's chain ID to the root or a registered, active tenant domain. It sets that domain on the context before preprocessing and fee purchase.
3. **`EVMPreprocessDecorator`** — unpacks the Ethereum data, recovers the sender using the fork-appropriate signer, populates derived metadata and associates addresses when needed. An Associate transaction must have a positive balance, creates its root-domain association and returns here without running later decorators.
4. **`BasicDecorator`** — checks init-code size, non-negative value, intrinsic gas, unsupported blob transactions and the consensus gas bound. For an eligible delivered transaction with the expected nonce, it installs the callback that bumps the nonce if execution or later ante handling does not do so.
5. **`EVMFeeCheckDecorator`** — checks the fee cap against the selected base fee and the module minimum fee, checks funds and buys gas. Tenant transactions use `ChainsFeeReader.NextBaseFee(ctx, domainID)`; root transactions use the root base fee. Delivery finalizes ante fee state and records surplus. An enabled alternative fee denomination is charged through the fee-token path; transaction value still requires network coin. Simulation skips this decorator's fee purchase.
6. **`EVMDomainSigVerifyDecorator`** — delegates root transactions to `EVMSigVerifyDecorator` and checks tenant nonces in the selected domain. CheckTx accepts a nonce at or above the stored nonce for pending transactions; delivery requires an exact match. Failures return `sdkerrors.ErrWrongSequence`. Signature recovery has already occurred during preprocessing.
7. **`GasDecorator`** — installs the Pax gas meter using the EVM gas limit multiplied by `PriorityNormalizer`, and sets the gas estimate.

Domain admission is implemented by `ResolveTransactionChain` in [node/ante/evm_checktx.go](../../node/ante/evm_checktx.go). Root transactions select domain 0. Tenant chain IDs are `125000000 + domainID`, with a nonzero domain ID, as defined in [layerxchains/types/domain.go](../layerxchains/types/domain.go). A numeric ID in that range alone is insufficient: it must resolve to a registered domain with `StatusActive`. An unknown chain ID returns `sdkerrors.ErrInvalidChainID`; a registered inactive domain returns `chainstypes.ErrDomainNotActive`. Admission recognizes an unprotected legacy envelope as root, but normal signature recovery subsequently rejects it.

`EVMAddressDecorator` belongs to the **Cosmos** ante chain, after signature verification and sequence increment. It is not a separate address-derivation stage in the EVM chain.

The optimized `EvmCheckTxAnte` path in [node/ante/evm_checktx.go](../../node/ante/evm_checktx.go) performs wrapper/stateless checks, domain resolution, signature recovery and association, fee checks and gas purchase, nonce checks, then context decoration. Its Associate path returns early after association checks. The tracing ante chain mounts wrapper rejection, chain-ID admission, preprocessing, basic checks and domain signature/nonce verification; it omits fee purchase and the final gas decorator.

---

## StateDB Bridge

The [state package](state/statedb.go) implements go-ethereum's `vm.StateDB` interface on Cosmos SDK stores. It does not own an Ethereum state trie root.

Key design choices:

- **Balance representation** — Pax uses 6-decimal `uhpx` while EVM expects 18-decimal wei. The StateDB converts between them, tracking the sub-uhpx remainder (`wei`) separately: [1 `uhpx` = 10^12 wei](state/utils.go).
- **Snapshots and reverts** — uses Cosmos `CacheMultiStore` for state snapshots. A journal records in-memory mutations so they can be rolled back on revert.
- **Transient state** — logs, transient storage (EIP-1153), access lists and gas refunds are held in memory per transaction. Logs feed receipts; transient storage and access lists do not become persistent contract storage. `Finalize` writes surviving cached state and events back to the surrounding transaction context.
- **Coinbase collection** — each transaction gets a deterministic coinbase account for fee collection, which is swept to the fee collector at end-of-block.

---

## Deferred Processing

Execution finalizes its own state, while [deferred processing](keeper/deferred.go) supplies the information needed by [EndBlock](keeper/abci.go):

- **Deferred info** — execution records the transaction bloom and fee surplus in transient storage. If a reverted transaction has no deferred record, `GetAllEVMTxDeferredInfo` reconstructs its error from the transaction result.
- **End-of-block aggregation** — transaction coinbase balances are swept, surplus is combined, failed receipts are written for nonce-bumped transactions, and block-level bloom filters are composed. Domain fee accounting also adjusts domain base fees and pays operator shares.

Per-transaction coinbase accounts avoid a shared fee-collection account during execution. Deferred records let the end-block hook collect fees and compose blooms across the completed block. Receipt persistence is a later pre-commit step.

---

## Fee Management (EIP-1559)

The module implements [EIP-1559-style dynamic base fee adjustment](keeper/fee.go):

- At the end of each block, the root base fee is adjusted based on actual gas usage compared to a target.
- Usage above the target increases the base fee; usage below it decreases the base fee.
- Configured adjustment rates and minimum/maximum fee bounds constrain the result.
- A zero target leaves the previous fee unchanged; absent block consensus parameters skip the root adjustment.

Per-transaction coinbase balances are swept to the fee collector at end-of-block in normal execution. StateDB and ante surplus are tracked separately and reconciled into the EVM module account. Active tenant domains have separate base-fee adjustment and operator fee-share accounting in [EndBlock](keeper/abci.go).

---

## Pointer Contracts (CW ↔ ERC Interoperability)

Pointer contracts enable tokens on one VM to be accessed from the other VM.

**EVM pointers for CW/native tokens:**

- Native Cosmos denoms get an ERC20 representation.
- CW20 tokens get an ERC20 pointer.
- CW721 NFTs get an ERC721 pointer.
- CW1155 multi-tokens get an ERC1155 pointer.

**CW wrappers for ERC tokens:**

- ERC20 tokens get a CW20 wrapper.
- ERC721 NFTs get a CW721 wrapper.
- ERC1155 multi-tokens get a CW1155 wrapper.

Pointers are versioned and can be upgraded. The [pointer registry](keeper/pointer.go) records forward and reverse mappings, allowing lookup of the original token from its pointer address. It rejects a pointer to another pointer.

EVM bytecode/ABIs and CW wrapper Wasm are embedded from [artifacts/](artifacts/).

---

## Precompiled Contracts

The module supports custom precompiled contracts such as Bank, Staking, Gov and Wasmd that expose SDK functionality to EVM callers. The [keeper's precompile selection](keeper/keeper.go) gates activation by the block context. Ordinary execution uses the latest enabled implementation; tracing selects the implementation for the historical height.

The [payable-precompile set](keeper/precompile.go) identifies addresses whose transfer events are suppressed to avoid double-counting forwarded value.

---

## ABCI Lifecycle

### BeginBlock

- Resets per-block tracking state (transaction results, messages) outside tracing.

### EndBlock

The [keeper hook](keeper/abci.go) performs:

1. Captures an available trace snapshot and queues the preceding height for trace baking outside tracing, after the first height.
2. Cleans up old transaction hashes.
3. Migrates legacy receipts in periodic batches, if any remain.
4. Prunes zero-value storage slots in resumable batches.
5. Adjusts the root dynamic base fee for the next block.
6. Processes deferred records: writes eligible failed receipts, sweeps transaction coinbase balances to the fee collector and reconciles surplus.
7. Adjusts active domain base fees and pays domain operator shares.
8. Composes block blooms, including chain-generated synthetic receipts.

Before commit, [node/app.go](../../node/app.go) calls `FlushTransientReceipts` to persist the block's receipts.

---

## Transaction Receipts and Logs

- Receipts are built from StateDB logs, bloom filters, gas usage and contract addresses.
- Within a block, receipts live in transient storage and are flushed to the dedicated receipt store before commit, with cumulative gas usage calculated during the flush. See [receipt.go](keeper/receipt.go).
- Block blooms support log filtering. The EVM-only bloom excludes transaction shell receipts but includes all logs on the selected EVM receipts, including synthetic logs. Chain-generated end-block receipts also contribute their logs. `GetLogsForTx` converts receipt logs without filtering the synthetic flag. See [log.go](keeper/log.go) and [abci.go](keeper/abci.go).

### Synthetic Receipts and Logs

There are several types of synthetic receipts/logs on Pax:

- **CW->EVM** — a Cosmos transaction calling an EVM contract creates a receipt carrying EVM logs.
- **CW pointee** — a Cosmos transaction calling a CosmWasm contract with an EVM pointer can emit synthetic logs representing its token activity to EVM clients.
- **EVM->CW** — an EVM transaction calling a CosmWasm contract with an EVM pointer can include synthetic logs on its EVM receipt.

There are also [chain-generated receipts](keeper/synthetic_receipts.go) for perps and spot end-block processing and the v6.12.0 activation. Each kind merges its block's internal EVM calls under a deterministic hash; receipt flushing assigns its final transaction index after the real transactions.

### Receipts for Failure Scenarios

An EVM transaction can fail during ante handling before EVM execution. Receipt creation depends on how far admission progressed:

- **Nonce mismatch** — no failed receipt is created when the sender nonce was not bumped.
- **Later ante failure with a nonce bump** — `EndBlock` writes a status-0 receipt from deferred error information when the transaction's nonce-bump marker is present.
- **Early admission failure** — wrapper rejection, unknown/inactive domains and signature recovery failures occur before the basic decorator can install its nonce-bump callback. These do not automatically produce failed receipts.

The controlling condition is `GetNonceBumped` in [abci.go](keeper/abci.go), not merely the presence of an ante error.

---

## WASM Integration

CosmWasm contracts can interact with the EVM through two mechanisms:

- **Queries** — [static EVM calls, ERC20/721/1155 token queries and address lookups](client/wasm/query.go).
- **Messages** — `MsgInternalEVMCall` for regular calls and `MsgInternalEVMDelegateCall` for the pointer delegate-call path. [Internal call handling](keeper/evm.go) requires the delegate-call source to match a registered pointer and the sender to have an address association; delegate-creation is rejected.

---
