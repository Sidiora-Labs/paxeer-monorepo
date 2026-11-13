# Client contract

First-party clients submit EVM transactions and read the merged Paxeer BFT network. The mainnet-beta EVM chain ID is `125` (`0x7d`); x402 uses `eip155:125`. The kernel participates in block proposal and execution, and block commit establishes finality. Public node JSON-RPC uses the root path. The [RPC contract](rpc.md), [receipt contract](sequencer.md), and each precompile's ABI define the wire formats.

## (a) Operations

| Operation | Current path | Implementation |
| --- | --- | --- |
| User write | Sign an EIP-1559 transaction with the user's secp256k1 key and send it with `eth_sendRawTransaction`. | [EVM signing](../../../services/agent/crates/layerx-crypto/src/evm_transaction.rs) |
| Sponsored write | Request a gas-station quote and submit the user's signed authorization through `POST /quote` and `POST /submit`. | [Gas station](../../../services/interop/crates/layerx-gas-station/src/service.rs) |
| Receipt read | Call `pax_sequencerReceipt` with the transaction hash. | [Node receipt API](../../../rpc/sequencer_receipt.go) |
| State read | Use `eth_call`, `eth_getBalance`, `eth_getTransactionCount`, or the available `px_*` read methods. | [Node RPC](../../../rpc), [hosted dispatch](../../../services/platform/hosted/gateway/src/rpc.rs) |
| Identity binding | The DID key signs the binding message; the EVM sender calls `bindLayerX` at the addr precompile. | [Binding implementation](../../../precompiles/addr/addr.go) |
| Subscription | Use the supported `eth_subscribe` subscriptions, including `logs`, `newHeads` and `newPendingTransactions`. | [Node subscriptions](../../../rpc/filter.go) |

The receipt API reports these transaction rungs:

| Rung | Meaning |
| --- | --- |
| `submitted` | The transaction is retained without a signed receipt. |
| `receipted` | A signed receipt exists and the batch is not yet committed. |
| `committed` | The receipt is covered by a sequence batch in a committed block. |

Submission alone does not establish payment or execution success. Clients match the transaction hash and verify the receipt and required rung before acting on it. An unknown submission outcome is resolved using the retained transaction and authenticated receipt evidence.

## (b) Identity and keys

The [addr precompile](../../../precompiles/addr/addr.go) at `0x0000000000000000000000000000000000001004` serves `bindLayerX`, `unbindLayerX` and `getLayerXDid`. The [binding message](../../../modules/evm/types/layerx_binding.go) contains the `LX:PAXEER-BIND:v1` domain, uint256 big-endian chain ID, EVM address and nonce, signed by the ed25519 DID key. The secp256k1 key signs the EVM transaction.

Hosted `px_register` admits a tenant and key through the [registration route](../../../services/platform/hosted/gateway/src/rpc_register.rs); its binding digest domain is `layerx-register-binding-v1`. The embedded wallet's [attestor client](../../../services/wallet/gateway/src/attestor/client.ts) requires five attestor endpoints and a signing threshold of three. Secrets are supplied through the service's protected configuration.

## (c) Receipt verification

Receipt consumers decode the canonical signed bytes, select an independently trusted key whose height interval covers the receipt, verify the signature, and match the transaction hash and execution fields. Payment consumers additionally match the expected log address, topics, asset, amount and payment reference. A committed result requires verified block and commit evidence.

| Consumer | Verification boundary |
| --- | --- |
| Node receipt API | [sequencer_receipt.go](../../../rpc/sequencer_receipt.go) validates the receipt view and committed coverage. Governed pins are exposed by `px_getReceiptKeys`. |
| Go receipt keys | [receipt_keys.go](../../../modules/sequencer/keeper/receipt_keys.go) selects height-bounded keys and verifies receipts. |
| Wallet SDK | [receipt.ts](../../../sdk/wallet/src/receipt.ts) reads governed pins and checks the signed receipt against the raw transaction and RPC view. |
| Rust agent proof library | [receipt.rs](../../../services/agent/crates/layerx-proof/src/receipt.rs) provides `verify_sequencer_signature` with `ReceiptKeys::HeightRanges`. Its `verify` entry point takes an `AuthorizedBatch` for the full receipt proof. |
| Rust agent SDK | [rpc_verification.rs](../../../services/agent/crates/layerx-sdk/src/rpc_verification.rs) verifies RPC reads; [rpc_commit.rs](../../../services/agent/crates/layerx-sdk/src/rpc_commit.rs) requires finality trust for committed evidence. |
| Seller middleware | [verifyPaymentReceipt](../../../services/platform/middleware/seller/src/index.ts) verifies signed transaction-bound receipt evidence against configured receipt keys and payment terms. |

<a id="lx-switch-off"></a>

## (d) Switch-off

The node and hosted gateway reject retired `lx_*` JSON-RPC names with method-not-found (`-32601`). The hosted [dispatcher](../../../services/platform/hosted/gateway/src/rpc.rs) accepts the `px_*` and EVM namespaces and `pax_sequencerReceipt`, then routes supported methods. Its WebSocket surface uses the same dispatch boundary.

| Operation | Current interface |
| --- | --- |
| Submit | `eth_sendRawTransaction` or the gas station's sponsored submission route |
| Pending sender nonce | `eth_getTransactionCount(address, "pending")` |
| Account and balances | `px_getAccount`, `px_getBalances`, `px_listAssets` |
| Network and history | `px_getNetwork`, `px_getHistory` |
| Resolve identity | `px_resolveAccount` |
| Register | `px_register` |
| Testnet funding | Hosted `px_requestFunds`; the [funding route](../../../services/platform/hosted/gateway/src/rpc_faucet.rs) forwards an authenticated claim to the configured faucet. |
| Receipt keys | `px_getReceiptKeys` |

The [gateway OpenRPC document](../../../services/platform/hosted/gateway/openrpc.json) describes its hosted surface. Node and hosted deployments expose different method sets; clients configure the appropriate endpoint for each operation.

## (e) Conformance

### k3r-receipt-checks

The [seller verifier](../../../services/platform/middleware/seller/src/index.ts) checks receipt keys, height bounds, transaction identity and payment logs before releasing a resource. Committed payment terms require a configured commitment resolver. The [ramp toolkit](../../../services/platform/ramps/toolkit/src) binds orders to EVM calls and verifies their transaction receipts.

### c10-x402-conformance

Seller and buyer middleware use `eip155:125` and the transaction rungs `receipted` and `committed` for payment terms. The [seller](../../../services/platform/middleware/seller/src/index.ts) and [buyer](../../../services/platform/middleware/buyer/src/index.ts) validate supported payment requirements. A missing committed-evidence resolver does not turn a committed requirement into a receipted payment.

### o52-x402-builders

[Payment builders](../../../services/platform/middleware/seller/src/grant.ts) construct ledger grant and payment calls. The ledger ABI at `0x000000000000000000000000000000000000101A` defines `draw`, `payExact` and their emitted logs; see the [ledger contract](ledger-auth.md).

### o53-x402-verification

The [seller receipt verifier](../../../services/platform/middleware/seller/src/index.ts) binds a payment to the signed receipt's transaction and logs. The receipt digest, signing key and key interval are checked independently of the caller's claimed rung.

### o54-seller-rpc

The [seller RPC client](../../../services/platform/middleware/seller/src/rpc.ts) submits raw transactions and resolves signed receipt evidence. A submitted transaction remains pending until the required evidence is available.

### wiring-integration

The [Express](../../../sdk/integrations/express/src/index.ts) and [Next](../../../sdk/integrations/next/src/index.ts) integrations load `receiptKeys` from `LAYERX_RECEIPT_KEYS_JSON` and pass them to the payment middleware and receipt authority.

## (f) Client packages

| Package | Purpose | Code root |
| --- | --- | --- |
| [launchpad-sdk-shared-go](#launchpad-sdk-shared-go) | Shared Go packages of the launchpad services | [sdk/launchpad/shared](../../../sdk/launchpad/shared) |
| [wallet-app](#wallet-app) | Wallet app | [apps/wallet](../../../apps/wallet) |
| [launchpad-sdk-sdk](#launchpad-sdk-sdk) | Launchpad TypeScript SDK | [sdk/launchpad/packages/sdk](../../../sdk/launchpad/packages/sdk) |
| [launchpad-sdk-sdk-v2](#launchpad-sdk-sdk-v2) | Launchpad TypeScript SDK, v2 routes | [sdk/launchpad/packages/sdk-v2](../../../sdk/launchpad/packages/sdk-v2) |
| [launchpad-sdk-production-contracts-v2](#launchpad-sdk-production-contracts-v2) | Generated launchpad contract types, v2 | [sdk/launchpad/packages/production-contracts-v2](../../../sdk/launchpad/packages/production-contracts-v2) |
| [wallet-sdk](#wallet-sdk) | Wallet SDK used by the wallet app and third-party pages | [sdk/wallet](../../../sdk/wallet) |
| [launchpad-sdk-production-contracts](#launchpad-sdk-production-contracts) | Generated launchpad contract types | [sdk/launchpad/packages/production-contracts](../../../sdk/launchpad/packages/production-contracts) |
| [launchpad-sdk-protocol-go](#launchpad-sdk-protocol-go) | Go bindings and address catalogue of the launchpad protocol | [sdk/launchpad/protocol](../../../sdk/launchpad/protocol) |
| [launchpad-web-app](#launchpad-web-app) | Launchpad web app | [apps/launchpad/web](../../../apps/launchpad/web) |
| [wallet-demo](#wallet-demo) | Embedded wallet demo page | [services/wallet/demo](../../../services/wallet/demo) |
| [launchpad-sdk-candles-sdk](#launchpad-sdk-candles-sdk) | Candles SDK | [sdk/launchpad/packages/candles-sdk](../../../sdk/launchpad/packages/candles-sdk) |
| [launchpad-sdk-users-sdk](#launchpad-sdk-users-sdk) | Users SDK | [sdk/launchpad/packages/users-sdk](../../../sdk/launchpad/packages/users-sdk) |
| [launchpad-mobile-app](#launchpad-mobile-app) | Launchpad mobile app | [apps/launchpad/mobile](../../../apps/launchpad/mobile) |
| [launchpad-sdk-metadata-sdk](#launchpad-sdk-metadata-sdk) | Metadata SDK | [sdk/launchpad/packages/metadata-sdk](../../../sdk/launchpad/packages/metadata-sdk) |
| [launchpad-sdk-ranking-algo-sdk](#launchpad-sdk-ranking-algo-sdk) | Ranking SDK | [sdk/launchpad/packages/ranking-algo-sdk](../../../sdk/launchpad/packages/ranking-algo-sdk) |
| [launchpad-sdk-sidiora-launchpad-v3](#launchpad-sdk-sidiora-launchpad-v3) | Launchpad v3 contract package | [sdk/launchpad/packages/sidiora-launchpad-v3](../../../sdk/launchpad/packages/sidiora-launchpad-v3) |
| [launchpad-sdk-stats-sdk](#launchpad-sdk-stats-sdk) | Stats SDK | [sdk/launchpad/packages/stats-sdk](../../../sdk/launchpad/packages/stats-sdk) |
| [launchpad-sdk-indexer-sdk](#launchpad-sdk-indexer-sdk) | Indexer SDK | [sdk/launchpad/packages/indexer-sdk](../../../sdk/launchpad/packages/indexer-sdk) |
| [explorer-smart-contract-verifier-metrics](#explorer-smart-contract-verifier-metrics) | Metrics of the contract verifier used by explorer clients | [services/explorer/services/smart-contract-verifier](../../../services/explorer/services/smart-contract-verifier) |
| [explorer-recaptcha-verifier-errors](#explorer-recaptcha-verifier-errors) | Error codes of the explorer recaptcha verifier | [services/explorer/services/libs](../../../services/explorer/services/libs) |
| [explorer-service-launcher-metrics](#explorer-service-launcher-metrics) | Metrics of the explorer service launcher library | [services/explorer/services/libs](../../../services/explorer/services/libs) |
| [explorer-smart-contract-verifier-server-errors](#explorer-smart-contract-verifier-server-errors) | Server error codes of the contract verifier | [services/explorer/services/smart-contract-verifier](../../../services/explorer/services/smart-contract-verifier) |
| [launchpad-sdk-sidiora-v3](#launchpad-sdk-sidiora-v3) | Launchpad v3 contract package | [sdk/launchpad/packages/sidiora-launchpad-v3](../../../sdk/launchpad/packages/sidiora-launchpad-v3) |
| [explorer-smart-contract-verifier-server-metrics](#explorer-smart-contract-verifier-server-metrics) | Server metrics of the contract verifier | [services/explorer/services/smart-contract-verifier](../../../services/explorer/services/smart-contract-verifier) |

### launchpad-sdk-shared-go

Shared Go packages of the launchpad services. Source: [sdk/launchpad/shared](../../../sdk/launchpad/shared).

### wallet-app

Wallet app. Source: [apps/wallet](../../../apps/wallet).

### launchpad-sdk-sdk

Launchpad TypeScript SDK. Source: [sdk/launchpad/packages/sdk](../../../sdk/launchpad/packages/sdk).

### launchpad-sdk-sdk-v2

Launchpad TypeScript SDK, v2 routes. Source: [sdk/launchpad/packages/sdk-v2](../../../sdk/launchpad/packages/sdk-v2).

### launchpad-sdk-production-contracts-v2

Generated launchpad contract types, v2. Source: [sdk/launchpad/packages/production-contracts-v2](../../../sdk/launchpad/packages/production-contracts-v2).

### wallet-sdk

Wallet SDK used by the wallet app and third-party pages. Source: [sdk/wallet](../../../sdk/wallet).

### launchpad-sdk-production-contracts

Generated launchpad contract types. Source: [sdk/launchpad/packages/production-contracts](../../../sdk/launchpad/packages/production-contracts).

### launchpad-sdk-protocol-go

Go bindings and address catalogue of the launchpad protocol. Source: [sdk/launchpad/protocol](../../../sdk/launchpad/protocol).

### launchpad-web-app

Launchpad web app. Source: [apps/launchpad/web](../../../apps/launchpad/web).

### wallet-demo

Embedded wallet demo page. Source: [services/wallet/demo](../../../services/wallet/demo).

### launchpad-sdk-candles-sdk

Candles SDK. Source: [sdk/launchpad/packages/candles-sdk](../../../sdk/launchpad/packages/candles-sdk).

### launchpad-sdk-users-sdk

Users SDK. Source: [sdk/launchpad/packages/users-sdk](../../../sdk/launchpad/packages/users-sdk).

### launchpad-mobile-app

Launchpad mobile app. Source: [apps/launchpad/mobile](../../../apps/launchpad/mobile).

### launchpad-sdk-metadata-sdk

Metadata SDK. Source: [sdk/launchpad/packages/metadata-sdk](../../../sdk/launchpad/packages/metadata-sdk).

### launchpad-sdk-ranking-algo-sdk

Ranking SDK. Source: [sdk/launchpad/packages/ranking-algo-sdk](../../../sdk/launchpad/packages/ranking-algo-sdk).

### launchpad-sdk-sidiora-launchpad-v3

Launchpad v3 contract package. Source: [sdk/launchpad/packages/sidiora-launchpad-v3](../../../sdk/launchpad/packages/sidiora-launchpad-v3).

### launchpad-sdk-stats-sdk

Stats SDK. Source: [sdk/launchpad/packages/stats-sdk](../../../sdk/launchpad/packages/stats-sdk).

### launchpad-sdk-indexer-sdk

Indexer SDK. Source: [sdk/launchpad/packages/indexer-sdk](../../../sdk/launchpad/packages/indexer-sdk).

### explorer-smart-contract-verifier-metrics

Metrics of the contract verifier used by explorer clients. Source: [services/explorer/services/smart-contract-verifier](../../../services/explorer/services/smart-contract-verifier).

### explorer-recaptcha-verifier-errors

Error codes of the explorer recaptcha verifier. Source: [services/explorer/services/libs](../../../services/explorer/services/libs).

### explorer-service-launcher-metrics

Metrics of the explorer service launcher library. Source: [services/explorer/services/libs](../../../services/explorer/services/libs).

### explorer-smart-contract-verifier-server-errors

Server error codes of the contract verifier. Source: [services/explorer/services/smart-contract-verifier](../../../services/explorer/services/smart-contract-verifier).

### launchpad-sdk-sidiora-v3

Launchpad v3 contract package. Source: [sdk/launchpad/packages/sidiora-launchpad-v3](../../../sdk/launchpad/packages/sidiora-launchpad-v3).

### explorer-smart-contract-verifier-server-metrics

Server metrics of the contract verifier. Source: [services/explorer/services/smart-contract-verifier](../../../services/explorer/services/smart-contract-verifier).

## Client path proof

The [client proof source](../../../services/agent/crates/layerx-client/tests/proof_client.rs) covers finalized snapshot binding, durable signing records and public RPC decoders. `CommitFinality` binds block and application hashes and a store proof digest; `decode_snapshot_binding` refuses a commit before the snapshot's execution height. `OperationJournal` preserves exact signed transaction bytes across reopening and refuses mismatched identifiers or truncated records.

The public client decoders read chain ID, native balance, account, assets, network head and history through the HTTP JSON-RPC transport. History cursors are validated before transmission; malformed response fields are refused.
