<!-- Source: https://docs.paxeer.app/json-rpc/ -->

# JSON-RPC API

Paxeer's EVM JSON-RPC interface with standard Ethereum compatibility and Paxeer-specific enhancements.

For complete read-only examples, follow [Query the EVM](https://docs.paxeer.app/querying-evm) or the [Python Quickstart](https://docs.paxeer.app/python). Use [Debug Transactions](https://docs.paxeer.app/debugging-transactions) to work from a failed receipt to a reproducible diagnosis.

**Source:** `rpc/server.go` (registration), `rpc/tx.go` (pending nonces), `rpc/state.go` (proofs), and `rpc/subscribe.go` and `rpc/send.go` (subscriptions)

## Architecture Overview

Paxeer serves [Ethereum JSON-RPC](https://ethereum.org/en/developers/docs/apis/json-rpc/) at the node's root path. Mainnet-beta uses EVM chain ID 125 (`0x7d`) and BFT block commitment. The source registers the namespaces below; availability on a particular endpoint depends on its configuration and enabled capabilities.

### Namespace Summary

| Namespace | Transaction Visibility | Use Case |
| --- | --- | --- |
| `eth_` | EVM view and native venue receipts | Pure EVM applications, Ethereum tooling compatibility |
| `pax_` | Extended EVM/Cosmos views and signed receipts | Allowlisted Cosmos helpers and `pax_sequencerReceipt` |
| `pax2_` | EVM + Cosmos + bank transfers | Complete transaction view including native transfers |
| `debug_` | EVM tracing and debugging | Transaction replay, gas profiling, state inspection |
| `px_` | Native account, kernel and receipt data | Node methods and `px_getReceiptKeys`; hosted gateway methods have their own contract |
| `echo_`, `net_`, `web3_` | Liveness, network identity, client version | Standard utility namespaces |
| `txpool_` | Mempool inspection | Pending/queued transaction views on HTTP and WebSocket |

HTTP registers `echo`, `eth`, `pax`, `pax2`, `px`, `net`, `txpool`, `web3` and `debug`. WebSocket registers `echo`, `eth`, `net`, `web3`, `txpool`, `px` and the signed-receipt method in `pax`. The `test` namespace is HTTP-only and requires explicit enablement on a non-live chain. The hosted gateway's `px_` contract is declared in `services/platform/hosted/gateway/openrpc.json`.

## eth_ Endpoints

The `eth_` prefixed endpoints provide the EVM-compatible view, including chain-generated venue receipts where present:

- **Transaction selection:** EVM transactions and native venue receipts; Cosmos contract activity uses the extended view

- **Ethereum tooling:** Use ethers.js, viem, web3.js, Hardhat, and Foundry with the method-specific differences below

- **Transaction view:** EVM transactions, their emitted logs, and supported native venue receipt logs

- **Method contracts:** Standard request shapes with documented Paxeer differences

### Key eth_ Methods

- `eth_blockNumber`

- `eth_getBlockByNumber`, `eth_getBlockByHash`

- `eth_getTransactionByHash`

- `eth_getTransactionReceipt`

- `eth_getLogs`

- `eth_call`, `eth_estimateGas`

- `eth_sendRawTransaction`

- `eth_getBalance`, `eth_getCode`

## pax_ Endpoints

The `pax_` prefixed endpoints provide an enhanced view that includes both EVM and relevant Cosmos transactions:

- **Synthetic transactions:** Cosmos events (CW20, CW721) exposed as EVM logs

- **Extended visibility:** EVM transactions and eligible Cosmos contract activity with synthetic receipts

- **Cross-chain events:** Index pointer contracts and token transfers

- **Trace filtering:** Variants to exclude pre-state check failures

### Synthetic Transaction Endpoints

These endpoints bridge Cosmos and EVM by exposing Cosmos-native events as EVM-compatible logs:

- `pax_getLogs` — Enhanced `eth_getLogs` with synthetic logs

- `pax_getFilterLogs` — Enhanced `eth_getFilterLogs` with synthetic logs

- `pax_getBlockByNumber`, `pax_getBlockByHash` — Include synthetic transactions

- `pax_getBlockReceipts` — Include receipts for synthetic transactions

**Receipt lookup:** Use the allowlisted `pax_getTransactionReceipt` for Cosmos synthetic receipts. `eth_getTransactionReceipt` also serves native venue receipts by hash.

### Trace Failure Filtering

A block can contain transactions that fail before EVM execution, including nonce or balance checks. Use the following endpoints when you need a view that excludes those failures:

- `pax_traceBlockByNumberExcludeTraceFail`

- `pax_traceBlockByHashExcludeTraceFail`

- `pax_getTransactionReceiptExcludeTraceFail`

- `pax_getBlockByNumberExcludeTraceFail`

- `pax_getBlockByHashExcludeTraceFail`

## pax2_ Endpoints (Bank Transfers)

The `pax2_` namespace exposes the same block shape as `pax_` but includes **bank transfers** in block payloads:

- Seven methods: block, block receipts, transaction counts, `ExcludeTraceFail` variants

- No `pax2_` transaction or filter API

- HTTP only (not WebSocket)

### Extended API Gating

Extended `pax_*` and `pax2_*` methods on HTTP are gated by `[evm].enabled_legacy_pax_apis` in `app.toml`. The signed-receipt lookup `pax_sequencerReceipt` is exempt from this allowlist:

- **New integrations:** Prefer standard EVM methods and the native receipt APIs

- **Allow list:** Only methods in the config array are enabled

- **Default config:** Three `pax_*` address/Cosmos helpers pre-filled (`pax_getPaxAddress`, `pax_getEVMAddress`, `pax_getCosmosTx`)

- **Disabled response:** JSON-RPC error `-32601` with the method's enablement requirement

**Coverage:** `rpc/pax_legacy_test.go`

## Transaction Index Mismatches

**Important:** Transaction indices differ between `eth_` and `pax_` endpoints.

### Example

Consider a block with two EVM transactions and an eligible Cosmos contract transaction, without additional venue receipts:

1. EVM Transaction 1
2. Cosmos Transaction 1
3. EVM Transaction 2

#### eth_getBlockReceipts

Returns only EVM transactions with sequential indices:

- EVM Transaction 1 (tx index: 0)

- EVM Transaction 2 (tx index: 1)

#### pax_getBlockReceipts

Includes the eligible Cosmos transaction and returns these indices:

- EVM Transaction 1 (tx index: 0)

- Cosmos Transaction 1 (tx index: 1)

- EVM Transaction 2 (tx index: 2)

### Receipts and Logs

- **EVM-originating:** Synthetic events included in both `eth_getLogs` and `eth_getTransactionReceipt`

- **Cosmos contract transactions:** Their synthetic events are *not* in the ordinary `eth_` transaction view; use `pax_getLogs` and `pax_getBlockReceipts`

- **Native venue receipts:** Chain-generated venue receipts can appear in both views; their transaction indices use fixed slots after the block's transaction count

### Best Practice

- Use the same endpoint consistently within your application

- Account for index differences when switching endpoints

- Prefer transaction hashes over indices (hashes are consistent across endpoints)

## debug_ Endpoints

Use `debug_trace*` methods to inspect historical execution:

- Inspect failed call frames and returned error data

- Compare gas consumption across calls and opcodes

- Review the state accesses and changes exposed by the selected tracer

### Key debug_ Methods

- `debug_traceBlockByNumber`, `debug_traceBlockByHash`

- `debug_traceTransaction`

- `debug_traceCall`

## Paxeer RPC Distinctions

Paxeer's RPC deviates from Ethereum in several areas:

### Block Tags and Pending Nonces

- Committed-state reads resolve supported latest, safe, and finalized tags through the RPC publication watermarks; supported pending reads use a configured open execution layer

- Pending account nonces have a separate path: `eth_getTransactionCount(address, "pending")` uses the mempool’s next nonce or a configured proxy

- Track submitted hashes and use the pending nonce when coordinating multiple transactions from one account

### No Uncle Blocks

- Paxeer finalizes blocks through BFT consensus and does not produce Ethereum uncle blocks

- The uncle endpoints are registered on HTTP and WebSocket and answer with the empty result: `eth_getUncleByBlockNumberAndIndex` and `eth_getUncleByBlockHashAndIndex` return `null`, and `eth_getUncleCountByBlockNumber` / `eth_getUncleCountByBlockHash` return `0x0` for known blocks

### Storage Proof Format

- `eth_getProof` is implemented for proof-capable stores

- The result contains `address`, `hexValues`, and ICS23 `storageProof` operations through the application root; it is not Ethereum’s Merkle Patricia account-proof shape

- Use a verifier for the returned application-store proof operations and an endpoint retaining the requested committed state; pending proofs are refused

### No Proof-of-Work

- `eth_mining` and `eth_hashrate` are not registered on any API, so they return method-not-found (`-32601`) rather than the explicit `-32000` unsupported error

- Paxeer uses BFT consensus, not PoW

### No Blobs

- EIP-4844 blob transactions not supported

- `eth_blobBaseFee` returns error `-32000`: "blobs not supported on this chain"

See [Unsupported Methods](https://docs.paxeer.app/json-rpc-unsupported) for the complete list.

## Consistent Historical Reads

Pin related reads to one block number and retain its block hash with indexed data. Use a provider with the historical state your application needs, and keep the same namespace when reading blocks, receipts, and logs.

## WebSocket Support

Paxeer RPC supports WebSocket for:

- `eth_subscribe` for `newHeads` and `logs`

- `newPendingTransactions` is served by `SendAPI` when the node has the receipt-backed pending feed; it emits hashes in receipt order and a `lagged` notification if its buffer drops hashes

- `eth_unsubscribe`

Extended `pax_*` block/filter helpers, `pax2_*` and `debug_` are HTTP-only. WebSocket also serves `pax_sequencerReceipt`, `px_` node APIs and `txpool_`. Use receipt lookup to reconcile notifications with execution results, and verify block commitment for finality.
