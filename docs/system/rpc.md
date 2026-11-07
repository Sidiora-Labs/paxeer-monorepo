# EVM JSON-RPC methods

Paxeer mainnet-beta is one BFT network running `paxd`, with the kernel inside
the network proposing blocks. The EVM chain ID is **125** (`0x7d`); a transaction
is final once its block is committed.

Public JSON-RPC uses the root path of `https://api1.mainnet-beta.paxeer.network`
through `https://api14.mainnet-beta.paxeer.network`, and
`https://api-mainnet-beta.paxeer.network`. Use a node's root WebSocket URL, for
example `wss://api1.mainnet-beta.paxeer.network`, for subscriptions.
`node.hyperpaxeer.com` is the hpx library mirror, not an RPC endpoint.

```bash
curl https://api1.mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The result is `"0x7d"`. Sub-chains are tenant domains stored in the database;
they do not require another node deployment. The node handler resolves registered
domains at `/chain/<id>`; the root endpoint selects the base chain.

## Catalogue and availability

Sources: the HTTP and WebSocket service registrations in
[`rpc/server.go`](../../rpc/server.go), the registered types' exported and
promoted methods, and the callback eligibility rules in
[`core/deps/geth/rpc/service.go`](../../core/deps/geth/rpc/service.go).
JSON-RPC names lowercase only the first character of the Go method name.
`DebugAPI.AsRawJSON` is a Go helper returning `([]byte, bool)`; the callback
rules reject that signature, so it has no JSON-RPC method.

[`rpc/served_methods.go`](../../rpc/served_methods.go) distinguishes ordinary
calls from subscription selectors. Its frozen registration lists are
[`served_methods_http.golden`](../../rpc/testdata/served_methods_http.golden)
and [`served_methods_ws.golden`](../../rpc/testdata/served_methods_ws.golden).

| Registration configuration | Service calls | Subscription topics |
|---|---|---|
| HTTP, all legacy methods enabled, test API disabled | 120 | 0 |
| HTTP, default legacy allowlist, test API disabled | 87 | 0 |
| WebSocket | 72 | 3 |

These counts cover the service calls below. They exclude Geth's built-in
`rpc_modules` metadata method and the WebSocket subscription control methods
`eth_subscribe` and `eth_unsubscribe`. Enabling the eligible test API adds one
HTTP call. Registration does not guarantee successful execution: configuration
and backend availability still apply.

Configuration is under `[evm]` in `app.toml`, as defined in
[`rpc/config/config.go`](../../rpc/config/config.go):

- `deny_list` can deny registered methods on either transport.
- `enabled_legacy_pax_apis` gates the 36 legacy `pax_*` and `pax2_*` calls on
  HTTP. Its defaults allow only `pax_getPaxAddress`, `pax_getEVMAddress`, and
  `pax_getCosmosTx`. A disabled legacy call returns code `-32601` with error
  data `"legacy_pax_deprecated"`. Enabled legacy calls carry the
  `Pax-Legacy-RPC-Deprecation` response header. Prefer standard `eth_*` and
  `debug_*` calls for new integrations. `pax_sequencerReceipt` is exempt from
  this legacy gate; WebSocket does not register either legacy namespace.
- `enable_unsafe_keyring_rpc` defaults to `false`. Local signing calls
  (`eth_sign`, `eth_signTransaction`, `eth_sendTransaction`, and legacy
  `pax_sign`) remain registered but refuse execution while disabled.
  Enabling this option on a live chain prevents either server from starting.
  Sign locally and broadcast with `eth_sendRawTransaction` instead.
- `enable_test_api` defaults to `false`. The `test` service is registered only
  on HTTP when this option is `true` **and** the chain ID is non-live.

In the tables, **HTTP, WS** means registered on both transports. HTTP-only
legacy rows remain subject to the allowlist described above. Subscription
selectors are listed separately under [WebSocket subscriptions](#websocket-subscriptions).

## debug

| Method | Transport | Service type | Source |
|---|---|---|---|
| `debug_getRawBlock` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_getRawHeader` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_getRawReceipts` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_getRawTransaction` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_traceBlockByHash` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_traceBlockByNumber` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_traceCall` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_traceStateAccess` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_traceTransaction` | HTTP | DebugAPI | `rpc/tracers.go` |
| `debug_traceTransactionProfile` | HTTP | DebugAPI | `rpc/trace_profile.go` |

## echo

| Method | Transport | Service type | Source |
|---|---|---|---|
| `echo_echo` | HTTP, WS | EchoAPI | `rpc/echo.go` |

## eth

| Method | Transport | Service type | Source |
|---|---|---|---|
| `eth_accounts` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_blobBaseFee` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_blockNumber` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_call` | HTTP, WS | SimulationAPI | `rpc/simulate.go` |
| `eth_chainId` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_coinbase` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_createAccessList` | HTTP, WS | SimulationAPI | `rpc/simulate.go` |
| `eth_estimateGas` | HTTP, WS | SimulationAPI | `rpc/simulate.go` |
| `eth_estimateGasAfterCalls` | HTTP, WS | SimulationAPI | `rpc/simulate.go` |
| `eth_feeHistory` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_gasPrice` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_getBalance` | HTTP, WS | StateAPI | `rpc/state.go` |
| `eth_getBlockByHash` | HTTP, WS | BlockAPI | `rpc/block.go` |
| `eth_getBlockByNumber` | HTTP, WS | BlockAPI | `rpc/block.go` |
| `eth_getBlockReceipts` | HTTP, WS | BlockAPI | `rpc/block.go` |
| `eth_getBlockTransactionCountByHash` | HTTP, WS | BlockAPI | `rpc/block.go` |
| `eth_getBlockTransactionCountByNumber` | HTTP, WS | BlockAPI | `rpc/block.go` |
| `eth_getCode` | HTTP, WS | StateAPI | `rpc/state.go` |
| `eth_getFilterChanges` | HTTP, WS | FilterAPI | `rpc/filter.go` |
| `eth_getFilterLogs` | HTTP, WS | FilterAPI | `rpc/filter.go` |
| `eth_getLogs` | HTTP, WS | FilterAPI | `rpc/filter.go` |
| `eth_getNonce` | HTTP, WS | StateAPI | `rpc/state.go` |
| `eth_getProof` | HTTP, WS | StateAPI | `rpc/state.go` |
| `eth_getRawTransactionByBlockHashAndIndex` | HTTP, WS | RawTransactionAPI | `rpc/raw_tx_index.go` |
| `eth_getRawTransactionByBlockNumberAndIndex` | HTTP, WS | RawTransactionAPI | `rpc/raw_tx_index.go` |
| `eth_getRawTransactionByHash` | HTTP, WS | RawTransactionAPI | `rpc/raw_tx.go` |
| `eth_getStorageAt` | HTTP, WS | StateAPI | `rpc/state.go` |
| `eth_getTransactionByBlockHashAndIndex` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_getTransactionByBlockNumberAndIndex` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_getTransactionByHash` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_getTransactionCount` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_getTransactionErrorByHash` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_getTransactionReceipt` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_getUncleByBlockHashAndIndex` | HTTP, WS | UncleAPI | `rpc/uncle.go` |
| `eth_getUncleByBlockNumberAndIndex` | HTTP, WS | UncleAPI | `rpc/uncle.go` |
| `eth_getUncleCountByBlockHash` | HTTP, WS | UncleAPI | `rpc/uncle_count.go` |
| `eth_getUncleCountByBlockNumber` | HTTP, WS | UncleAPI | `rpc/uncle_count.go` |
| `eth_getVMError` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_maxPriorityFeePerGas` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_newBlockFilter` | HTTP, WS | FilterAPI | `rpc/filter.go` |
| `eth_newFilter` | HTTP, WS | FilterAPI | `rpc/filter.go` |
| `eth_newPendingTransactionFilter` | HTTP, WS | FilterAPI | `rpc/filter.go` |
| `eth_sendRawTransaction` | HTTP, WS | SendAPI | `rpc/send.go` |
| `eth_sendRawTransactionSync` | HTTP, WS | SendAPI | `rpc/send_sync.go` |
| `eth_sendTransaction` | HTTP, WS | SendAPI | `rpc/send.go` |
| `eth_sign` | HTTP, WS | TransactionAPI | `rpc/tx.go` |
| `eth_signTransaction` | HTTP, WS | SendAPI | `rpc/send.go` |
| `eth_syncing` | HTTP, WS | InfoAPI | `rpc/info.go` |
| `eth_uninstallFilter` | HTTP, WS | FilterAPI | `rpc/filter.go` |

## net

| Method | Transport | Service type | Source |
|---|---|---|---|
| `net_listening` | HTTP, WS | NetAPI | `rpc/net.go` |
| `net_peerCount` | HTTP, WS | NetAPI | `rpc/net_peers.go` |
| `net_version` | HTTP, WS | domainNetAPI (embeds NetAPI) | `rpc/info.go` |

## pax

| Method | Transport | Service type | Source |
|---|---|---|---|
| `pax_associate` | HTTP | AssociationAPI | `rpc/association.go` |
| `pax_getBlockByHash` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax_getBlockByHashExcludeTraceFail` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax_getBlockByNumber` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax_getBlockByNumberExcludeTraceFail` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax_getBlockReceipts` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax_getBlockTransactionCountByHash` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax_getBlockTransactionCountByNumber` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax_getCosmosTx` | HTTP | AssociationAPI | `rpc/association.go` |
| `pax_getEVMAddress` | HTTP | AssociationAPI | `rpc/association.go` |
| `pax_getEvmTx` | HTTP | AssociationAPI | `rpc/association.go` |
| `pax_getFilterChanges` | HTTP | PaxFilterAPI | `rpc/pax_filter.go` |
| `pax_getFilterLogs` | HTTP | PaxFilterAPI | `rpc/pax_filter.go` |
| `pax_getLogs` | HTTP | PaxFilterAPI | `rpc/pax_filter.go` |
| `pax_getPaxAddress` | HTTP | AssociationAPI | `rpc/association.go` |
| `pax_getTransactionByBlockHashAndIndex` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_getTransactionByBlockNumberAndIndex` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_getTransactionByHash` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_getTransactionCount` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_getTransactionErrorByHash` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_getTransactionReceipt` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_getTransactionReceiptExcludeTraceFail` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_getVMError` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_newBlockFilter` | HTTP | PaxFilterAPI | `rpc/pax_filter.go` |
| `pax_newFilter` | HTTP | PaxFilterAPI | `rpc/pax_filter.go` |
| `pax_sequencerReceipt` | HTTP | SequencerReceiptAPI | `rpc/sequencer_receipt.go` |
| `pax_sign` | HTTP | PaxTransactionAPI | `rpc/tx.go` |
| `pax_traceBlockByHashExcludeTraceFail` | HTTP | PaxDebugAPI | `rpc/tracers.go` |
| `pax_traceBlockByNumberExcludeTraceFail` | HTTP | PaxDebugAPI | `rpc/tracers.go` |
| `pax_uninstallFilter` | HTTP | PaxFilterAPI | `rpc/pax_filter.go` |

## pax2

| Method | Transport | Service type | Source |
|---|---|---|---|
| `pax2_getBlockByHash` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax2_getBlockByHashExcludeTraceFail` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax2_getBlockByNumber` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax2_getBlockByNumberExcludeTraceFail` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax2_getBlockReceipts` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax2_getBlockTransactionCountByHash` | HTTP | PaxBlockAPI | `rpc/block.go` |
| `pax2_getBlockTransactionCountByNumber` | HTTP | PaxBlockAPI | `rpc/block.go` |

## px

| Method | Transport | Service type | Source |
|---|---|---|---|
| `px_getAccount` | HTTP, WS | pxAccountAPI | `rpc/px_account.go` |
| `px_getBalances` | HTTP, WS | pxAccountAPI | `rpc/px_account.go` |
| `px_getCapabilities` | HTTP, WS | pxAccountAPI | `rpc/px_discovery.go` |
| `px_getCommit` | HTTP, WS | pxKernelAPI | `rpc/px_kernel.go` |
| `px_getHistory` | HTTP, WS | pxHistoryAPI | `rpc/px_history.go` |
| `px_getNetwork` | HTTP, WS | pxAccountAPI | `rpc/px_account.go` |
| `px_getReceiptKeys` | HTTP, WS | receiptKeysAPI | `rpc/sequencer_receipt.go` |
| `px_getRouteCatalogue` | HTTP, WS | pxAccountAPI | `rpc/px_discovery.go` |
| `px_getSequenceBatch` | HTTP, WS | pxKernelAPI | `rpc/px_kernel.go` |
| `px_getUnifiedHistory` | HTTP, WS | pxHistoryAPI | `rpc/px_history.go` |
| `px_getValidators` | HTTP, WS | pxKernelAPI | `rpc/px_kernel.go` |
| `px_listAssets` | HTTP, WS | pxAccountAPI | `rpc/px_assets.go` |
| `px_listHolderRecords` | HTTP, WS | pxHistoryAPI | `rpc/px_history.go` |
| `px_register` | HTTP, WS | pxAccountAPI | `rpc/px_account.go` |
| `px_requestFunds` | HTTP, WS | pxAccountAPI | `rpc/px_account.go` |
| `px_resolveAccount` | HTTP, WS | pxAccountAPI | `rpc/px_account.go` |

The `px` namespace is served at the node root beside the other namespaces and
answers from node state. `px_getReceiptKeys` returns the governed receipt
signing keys used to authenticate `pax_sequencerReceipt` results. Its result is
`{"keys":[...]}`; entries contain `keyId`, `publicKey`, `firstHeight`, and
`lastHeight`. An empty list is valid. `px_register` and `px_requestFunds`
validate their parameters and then answer `-32004` with error data code
`hosted_gateway_service_required`: registration and funding are operations of
the hosted identity and faucet services, whose method contract is
[`services/platform/hosted/gateway/openrpc.json`](../../services/platform/hosted/gateway/openrpc.json).

Both `px_getReceiptKeys` and `pax_sequencerReceipt` require a configured local
receipt source or receipt forwarding URL. Without either, they return code
`-32000` and the message `<method> served by sequencer`. These are roles within
the merged network. Receipt progress is `submitted`, `receipted`, or `committed`;
a signed execution receipt before commitment does not establish block finality.

## test

| Method | Transport | Service type | Source |
|---|---|---|---|
| `test_incrementPointerVersion` | HTTP (opt-in, non-live only) | TestAPI | `rpc/test.go` |

## txpool

| Method | Transport | Service type | Source |
|---|---|---|---|
| `txpool_content` | HTTP | TxPoolAPI | `rpc/txpool.go` |
| `txpool_status` | HTTP | TxPoolAPI | `rpc/txpool.go` |

## web3

| Method | Transport | Service type | Source |
|---|---|---|---|
| `web3_clientVersion` | HTTP, WS | Web3API | `rpc/web3.go` |
| `web3_sha3` | HTTP, WS | Web3API | `rpc/web3.go` |

## WebSocket subscriptions

Use `eth_subscribe` with a selector as the first parameter. The selectors below
are not standalone JSON-RPC method names.

| Selector | Parameters | Service type | Source |
|---|---|---|---|
| `newHeads` | `["newHeads"]` | SubscriptionAPI | `rpc/subscribe.go` |
| `logs` | `["logs", filter]` (filter is optional) | SubscriptionAPI | `rpc/subscribe.go` |
| `newPendingTransactions` | `["newPendingTransactions"]` | SendAPI | `rpc/send.go` |

For example, send this JSON on an open WebSocket connection:

```json
{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}
```

The result is a subscription ID. Notifications use `eth_subscription`; stop a
subscription with `eth_unsubscribe` and `[subscriptionId]`. HTTP clients can
poll with `eth_newFilter`, `eth_newBlockFilter`, or
`eth_newPendingTransactionFilter`, read updates with `eth_getFilterChanges`,
read a log filter with `eth_getFilterLogs`, and release it with
`eth_uninstallFilter`. These polling calls are also registered on WebSocket.

Pending transaction notifications require the node's receipt feed:
`newPendingTransactions` emits hashes in receipt order and refuses subscriptions
on nodes without that feed. A slow subscriber can receive a
`{"lagged":"0x<count>"}` notification reporting dropped hashes.
`eth_newPendingTransactionFilter` likewise requires a receipt log feed and
supports hashes only; requesting full transactions is refused. Its changes
are the hashes receipted since the previous poll.
