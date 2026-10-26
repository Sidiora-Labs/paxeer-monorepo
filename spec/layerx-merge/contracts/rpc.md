# RPC contract: JSON-RPC, WebSocket, gRPC and REST catalogue

| Item | Value |
|---|---|
| Version | 1.0.0 (new file) |
| Revision read | 2a1033aa2 |
| Source | the domain 7 interface blocks of the merged specification, plus the query and REST blocks of other domains whose `contract` names this file |
| Change control | Changes only by amendment; each amendment raises the version and names the covering task ids |
| Companion contracts | contracts/evm-rpc.md freezes the behaviour of each EVM JSON-RPC method; contracts/proto-api.md freezes the proto messages, gRPC queries and REST routes; contracts/sequencer.md freezes pax_sequencerReceipt, the rungs and the pending tag. This file is the catalogue that joins them. |

Status words follow CONTRACT section 3. "Tasks" lists every task whose `interfaces` list names the interface; a stable row lists none. "Params" and "Returns" are read from the Go signature (JSON-RPC, context argument and error result dropped) or from the request and response proto messages (gRPC, REST). Where a message is not inventoried as a proto_message block the cell names the message only.

## Summary of domain 7

| Kind | stable | change | new | retire |
|---|---|---|---|---|
| jsonrpc_method | 80 | 16 | 12 | 17 |
| ws_route | 1 | 3 | 0 | 0 |
| http_route | 0 | 1 | 0 | 0 |
| grpc_service_method | 22 | 0 | 0 | 0 |
| grpc_query | 59 | 0 | 0 | 0 |
| rest_route | 78 | 0 | 2 | 0 |
| cosmos_msg | 23 | 0 | 0 | 0 |
| error_code | 4 | 0 | 2 | 0 |

<a id="endpoints"></a>

## Endpoints

| Surface | Listener | Default | Defined at |
|---|---|---|---|
| EVM JSON-RPC over HTTP | evm.http_address, evm.http_port | port 8545 | rpc/config/config.go:179; server rpc/server.go:87 |
| EVM JSON-RPC over WebSocket | evm.ws_port | port 8546 | rpc/config/config.go:181; server rpc/server.go:315 |
| gRPC | grpc.address | port 9090 on all interfaces | sdk/server/config/config.go:23 |
| gRPC-web | grpc-web.address | port 9091 on all interfaces | sdk/server/config/config.go:26 |
| REST (gRPC gateway) | api.address | port 1317 on all interfaces | sdk/server/config/config.go:275 |

| Interface | Kind | Defined at | Status | Tasks | Notes |
|---|---|---|---|---|---|
| 7.http-jsonrpc-root | http_route | rpc/rpcstack.go:243 | change | 7.13.4 | HTTP JSON-RPC on evm.http_address:evm.http_port (rpc/server.go:136) at the handler prefix, other paths 404 (rpc/rpcstack.go:211-239); CORS from evm.cors_origins (rpc/server.go:304), deny list from evm.deny_list, legac... |

<a id="error-model"></a>

## Error model

| Code | Meaning | Defined at |
|---|---|---|
| -32600 | invalid request (legacy pax gate synthesises it for a malformed batch element) | rpc/pax_legacy_http.go:18 |
| -32601 | pax_ or pax2_ method not enabled behind the legacy gate | rpc/pax_legacy_http.go:19 |
| -32603 | internal error answered by the legacy gate | rpc/pax_legacy_http.go:20 |
| -32000 | method registered but not supported (`ErrEVMNotSupported`); also "pending served by sequencer" and "pax_sequencerReceipt served by sequencer" | rpc/rpc_unsupported.go:5, :19; rpc/utils.go:51; rpc/sequencer_receipt.go:49 |
| 3 | execution reverted, with revert data (`RevertError`) | rpc/simulate.go:210 |
| -32005, 4 | planned: per-IP rate limit and eth_sendRawTransactionSync timeout (see rows below) | rpc/ip_rate_limit.go (planned), rpc/send_sync.go (planned) |

Unknown methods (-32601), malformed JSON (-32700) and invalid parameters (-32602) are answered by the go-ethereum rpc package the servers import (rpc/server.go:9, rpc/rpcstack.go:34; module pinned and replaced in go.mod:27, :290) with the JSON-RPC 2.0 codes. Error codes are counted by range in rpc/metrics.go:111. gRPC handlers answer gRPC status codes carrying the module error registered in the handler's codespace; the REST gateway maps the gRPC status to the HTTP status.

| Interface | Signature | Defined at | Status | Tasks | Contract | Notes |
|---|---|---|---|---|---|---|
| 7.err-evm-not-supported-32000-j38 | // ErrCodeEVMNotSupported is the JSON-RPC error code (-32000) for EVM RPC methods that | rpc/rpc_unsupported.go:3 | stable | - | contracts/evm-rpc.md#error-codes | Answer of methods registered but refused, including debug_getRawHeader, debug_getRawBlock and debug_getRawReceipts (decision 703) |
| 7.err-pax-legacy-not-enabled-32601-j38 | paxLegacyNotEnabled = -32601 | rpc/pax_legacy_http.go:19 | stable | - | contracts/evm-rpc.md#error-codes | HTTP answer for a pax_ or pax2_ name not enabled or not gated (rpc/pax_legacy.go fail-closed branch) |
| 7.err-rate-limited-32005-j38 | -32005 rate limit exceeded (HTTP 429) | rpc/ip_rate_limit.go (planned) | new | 7.13.4 | contracts/evm-rpc.md#error-codes | Answered before dispatch when a remote address exceeds evm.ip_rate_limit_burst (decision 711); task 7.13.4 |
| 7.err-revert-3-j38 | func (e *RevertError) ErrorCode() int // 3 | rpc/simulate.go:210 | stable | - | contracts/evm-rpc.md#error-codes | eth_call and eth_estimateGas revert answer with the revert data |
| 7.err-send-sync-timeout-4-j38 | 4 transaction not yet included (data: transaction hash) | rpc/send_sync.go (planned) | new | 7.8.2 | contracts/evm-rpc.md#error-codes | eth_sendRawTransactionSync timeout answer (decision 702); task 7.8.2 |
| 7.err-sequencer-not-served-32000-j38 | sequencerNotServedCode = -32000 | rpc/sequencer_receipt.go:49 | stable | - | contracts/evm-rpc.md#error-codes | pax_sequencerReceipt answer when the node holds no sequencer receipt source |

<a id="jsonrpc"></a>

## JSON-RPC methods

<a id="jsonrpc-debug"></a>

### Namespace debug

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| debug_getRawBlock | HTTP | blockNrOrHash rpc.BlockNumberOrHash | hexutil.Bytes | ErrEVMNotSupported; -32000 not supported; -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tracers.go:723 | contracts/evm-rpc.md#debug-getrawblock |
| debug_getRawHeader | HTTP | blockNrOrHash rpc.BlockNumberOrHash | hexutil.Bytes | ErrEVMNotSupported; -32000 not supported; -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tracers.go:715 | contracts/evm-rpc.md#debug-getrawheader |
| debug_getRawReceipts | HTTP | blockNrOrHash rpc.BlockNumberOrHash | []hexutil.Bytes | ErrEVMNotSupported; -32000 not supported; -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tracers.go:731 | contracts/evm-rpc.md#debug-getrawreceipts |
| debug_getRawTransaction | HTTP | hash common.Hash | hexutil.Bytes | ErrEVMNotSupported; -32000 not supported | change | 7.18.1 | rpc/tracers.go:739 | contracts/evm-rpc.md#debug-getrawtransaction |
| debug_traceBlockByHash | HTTP | hash common.Hash; config *tracers.TraceConfig | interface{} | - | stable | - | rpc/tracers.go:667 | contracts/evm-rpc.md#debug-traceblockbyhash |
| debug_traceBlockByNumber | HTTP | number rpc.BlockNumber; config *tracers.TraceConfig | interface{} | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tracers.go:639 | contracts/evm-rpc.md#debug-traceblockbynumber |
| debug_traceCall | HTTP | args export.TransactionArgs; blockNrOrHash rpc.BlockNumberOrHash; config *tracers.TraceCallConfig | interface{} | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tracers.go:695 | contracts/evm-rpc.md#debug-tracecall |
| debug_traceStateAccess | HTTP | hash common.Hash | interface{} | - | stable | - | rpc/tracers.go:753 | contracts/evm-rpc.md#debug-tracestateaccess |
| debug_traceTransaction | HTTP | hash common.Hash; config *tracers.TraceConfig | interface{} | - | stable | - | rpc/tracers.go:282 | contracts/evm-rpc.md#debug-tracetransaction |
| debug_traceTransactionProfile | HTTP | hash common.Hash; config *tracers.TraceConfig | interface{} | - | stable | - | rpc/trace_profile.go:45 | contracts/evm-rpc.md#debug-tracetransactionprofile |

<a id="jsonrpc-echo"></a>

### Namespace echo

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| echo_echo | HTTP, WS | data string | string | - | stable | - | rpc/echo.go:9 | contracts/evm-rpc.md#echo-echo |

<a id="jsonrpc-eth"></a>

### Namespace eth

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| eth_accounts | HTTP, WS | none | []common.Address | - | stable | - | rpc/info.go:82 | contracts/evm-rpc.md#eth-accounts |
| eth_blobBaseFee | HTTP, WS | none | *hexutil.Big | -32000 not supported | change | 7.14.1 | rpc/info.go:336 | contracts/evm-rpc.md#eth-blobbasefee |
| eth_blockNumber | HTTP, WS | none | hexutil.Uint64 | - | stable | - | rpc/info.go:52 | contracts/evm-rpc.md#eth-blocknumber |
| eth_calculateGasUsedRatio | HTTP, WS | blockHeight int64 | float64 | - | retire | 7.2.2 | rpc/info.go:547 | contracts/evm-rpc.md#eth-calculategasusedratio |
| eth_call | HTTP, WS | args export.TransactionArgs; blockNrOrHash *rpc.BlockNumberOrHash; overrides *export.StateOverride; blockOverrides *e... | hexutil.Bytes | 3 execution reverted (rpc/simulate.go:210); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/simulate.go:151 | contracts/evm-rpc.md#eth-call |
| eth_chainId | HTTP, WS | none | *hexutil.Big | - | stable | - | rpc/info.go:68 | contracts/evm-rpc.md#eth-chainid |
| eth_cleanup | HTTP | none | none | - | retire | 7.2.1 | rpc/filter.go:1206 | contracts/evm-rpc.md#eth-cleanup |
| eth_coinbase | HTTP, WS | none | common.Address | - | stable | - | rpc/info.go:74 | contracts/evm-rpc.md#eth-coinbase |
| eth_createAccessList | HTTP, WS | args export.TransactionArgs; blockNrOrHash *rpc.BlockNumberOrHash | *AccessListResult | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/simulate.go:86 | contracts/evm-rpc.md#eth-createaccesslist |
| eth_estimateGas | HTTP, WS | args export.TransactionArgs; blockNrOrHash *rpc.BlockNumberOrHash; overrides *export.StateOverride | hexutil.Uint64 | 3 execution reverted (rpc/simulate.go:210); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/simulate.go:107 | contracts/evm-rpc.md#eth-estimategas |
| eth_estimateGasAfterCalls | HTTP, WS | args export.TransactionArgs; calls []export.TransactionArgs; blockNrOrHash *rpc.BlockNumberOrHash; overrides *export.... | hexutil.Uint64 | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/simulate.go:129 | contracts/evm-rpc.md#eth-estimategasaftercalls |
| eth_feeHistory | HTTP, WS | blockCount gmath.HexOrDecimal64; lastBlock rpc.BlockNumber; rewardPercentiles []float64 | *FeeHistoryResult | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/info.go:148 | contracts/evm-rpc.md#eth-feehistory |
| eth_gasPrice | HTTP, WS | none | *hexutil.Big | - | stable | - | rpc/info.go:100 | contracts/evm-rpc.md#eth-gasprice |
| eth_gasPriceHelper | HTTP, WS | baseFee *big.Int; totalGasUsedPrevBlock uint64; medianRewardPrevBlock *big.Int | *hexutil.Big | - | retire | 7.2.2 | rpc/info.go:130 | contracts/evm-rpc.md#eth-gaspricehelper |
| eth_getBalance | HTTP, WS | address common.Address; blockNrOrHash rpc.BlockNumberOrHash | *hexutil.Big | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/state.go:49 | contracts/evm-rpc.md#eth-getbalance |
| eth_getBlockByHash | HTTP, WS | blockHash common.Hash; fullTx bool | map[string]interface{} | - | change | 7.15.1 | rpc/block.go:176 | contracts/evm-rpc.md#eth-getblockbyhash |
| eth_getBlockByNumber | HTTP, WS | number rpc.BlockNumber; fullTx bool | map[string]interface{} | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.11.1, 7.15.1 | rpc/block.go:210 | contracts/evm-rpc.md#eth-getblockbynumber |
| eth_getBlockReceipts | HTTP, WS | blockNrOrHash rpc.BlockNumberOrHash | []map[string]interface{} | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/block.go:344 | contracts/evm-rpc.md#eth-getblockreceipts |
| eth_getBlockTransactionCountByHash | HTTP, WS | blockHash common.Hash | *hexutil.Uint | - | stable | - | rpc/block.go:154 | contracts/evm-rpc.md#eth-getblocktransactioncountbyhash |
| eth_getBlockTransactionCountByNumber | HTTP, WS | number rpc.BlockNumber | *hexutil.Uint | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.11.2 | rpc/block.go:128 | contracts/evm-rpc.md#eth-getblocktransactioncountbynumber |
| eth_getCode | HTTP, WS | address common.Address; blockNrOrHash rpc.BlockNumberOrHash | hexutil.Bytes | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/state.go:66 | contracts/evm-rpc.md#eth-getcode |
| eth_getFilterChanges | HTTP | filterID ethrpc.ID | interface{} | - | stable | - | rpc/filter.go:876 | contracts/evm-rpc.md#eth-getfilterchanges |
| eth_getFilterLogs | HTTP | filterID ethrpc.ID | []*FilterLog | - | stable | - | rpc/filter.go:989 | contracts/evm-rpc.md#eth-getfilterlogs |
| eth_getLogs | HTTP | crit filters.FilterCriteria | []*FilterLog | - | change | 7.17.1, 7.17.2 | rpc/filter.go:1045 | contracts/evm-rpc.md#eth-getlogs |
| eth_getNonce | HTTP, WS | address common.Address | uint64 | - | stable | - | rpc/state.go:229 | contracts/evm-rpc.md#eth-getnonce |
| eth_getProof | HTTP, WS | address common.Address; storageKeys []string; blockNrOrHash rpc.BlockNumberOrHash | *ProofResult | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/state.go:124 | contracts/evm-rpc.md#eth-getproof |
| eth_getStorageAt | HTTP, WS | address common.Address; hexKey string; blockNrOrHash rpc.BlockNumberOrHash | hexutil.Bytes | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/state.go:83 | contracts/evm-rpc.md#eth-getstorageat |
| eth_getTransactionByBlockHashAndIndex | HTTP, WS | blockHash common.Hash; txIndex hexutil.Uint | *export.RPCTransaction | - | stable | - | rpc/tx.go:242 | contracts/evm-rpc.md#eth-gettransactionbyblockhashandindex |
| eth_getTransactionByBlockNumberAndIndex | HTTP, WS | blockNr rpc.BlockNumber; txIndex hexutil.Uint | *export.RPCTransaction | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.11.3 | rpc/tx.go:208 | contracts/evm-rpc.md#eth-gettransactionbyblocknumberandindex |
| eth_getTransactionByHash | HTTP, WS | hash common.Hash | *export.RPCTransaction | - | stable | - | rpc/tx.go:269 | contracts/evm-rpc.md#eth-gettransactionbyhash |
| eth_getTransactionCount | HTTP, WS | address common.Address; blockNrOrHash rpc.BlockNumberOrHash | *hexutil.Uint64 | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tx.go:397 | contracts/evm-rpc.md#eth-gettransactioncount |
| eth_getTransactionErrorByHash | HTTP, WS | hash common.Hash | string | - | stable | - | rpc/tx.go:379 | contracts/evm-rpc.md#eth-gettransactionerrorbyhash |
| eth_getTransactionReceipt | HTTP, WS | hash common.Hash | map[string]any | - | stable | - | rpc/tx.go:106 | contracts/evm-rpc.md#eth-gettransactionreceipt |
| eth_getVMError | HTTP, WS | hash common.Hash | string | - | stable | - | rpc/tx.go:193 | contracts/evm-rpc.md#eth-getvmerror |
| eth_maxPriorityFeePerGas | HTTP, WS | none | *hexutil.Big | - | stable | - | rpc/info.go:304 | contracts/evm-rpc.md#eth-maxpriorityfeepergas |
| eth_newBlockFilter | HTTP | none | ethrpc.ID | - | stable | - | rpc/filter.go:814 | contracts/evm-rpc.md#eth-newblockfilter |
| eth_newFilter | HTTP | crit filters.FilterCriteria | ethrpc.ID | - | stable | - | rpc/filter.go:767 | contracts/evm-rpc.md#eth-newfilter |
| eth_newPendingTransactionFilter | HTTP | fullTx *bool | ethrpc.ID | ErrEVMNotSupported; -32000 not supported | stable | - | rpc/filter.go:840 | contracts/evm-rpc.md#eth-newpendingtransactionfilter |
| eth_sendRawTransaction | HTTP, WS | input hexutil.Bytes | common.Hash | - | stable | - | rpc/send.go:164 | contracts/evm-rpc.md#eth-sendrawtransaction |
| eth_sendTransaction | HTTP, WS | args export.TransactionArgs | common.Hash | - | stable | - | rpc/send.go:458 | contracts/evm-rpc.md#eth-sendtransaction |
| eth_setReceiptLogFeed | HTTP | feed *ReceiptLogFeed | none | - | retire | 7.2.3 | rpc/filter.go:583 | contracts/evm-rpc.md#eth-setreceiptlogfeed |
| eth_sign | HTTP, WS | addr common.Address; data hexutil.Bytes | hexutil.Bytes | - | stable | - | rpc/tx.go:530 | contracts/evm-rpc.md#eth-sign |
| eth_signTransaction | HTTP, WS | args apitypes.SendTxArgs; _ *string | *export.SignTransactionResult | - | stable | - | rpc/send.go:435 | contracts/evm-rpc.md#eth-signtransaction |
| eth_subscribe | see notes | subscribeMethodSuffix = "_subscribe" | - | -32000 not supported | stable | - | core/deps/geth/rpc/json.go:35 | contracts/evm-rpc.md#eth-subscribe |
| eth_subscribe | WS subscription | filter *filters.FilterCriteria | *rpc.Subscription | - | change | 7.10.4, 7.10.5 | rpc/subscribe.go:230 | contracts/evm-rpc.md#ws-logs |
| eth_subscribe | WS subscription | none | *rpc.Subscription | - | change | 7.10.1, 7.10.2, 7.10.3 | rpc/subscribe.go:185 | contracts/evm-rpc.md#ws-new-heads |
| eth_subscribe | WS subscription | none | *rpc.Subscription | ErrPendingServedBySequencer; -32000 not supported | stable | - | rpc/send.go:357 | contracts/evm-rpc.md#ws-new-pending-transactions |
| eth_syncing | HTTP, WS | none | any | - | stable | - | rpc/info.go:353 | contracts/evm-rpc.md#eth-syncing |
| eth_uninstallFilter | HTTP | filterID ethrpc.ID | bool | - | stable | - | rpc/filter.go:1176 | contracts/evm-rpc.md#eth-uninstallfilter |
| eth_unsubscribe | see notes | unsubscribeMethodSuffix = "_unsubscribe" | - | - | stable | - | core/deps/geth/rpc/json.go:36 | contracts/evm-rpc.md#eth-unsubscribe |

<a id="jsonrpc-net"></a>

### Namespace net

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| net_version | HTTP, WS | none | string | - | stable | - | rpc/net.go:24 | contracts/evm-rpc.md#net-version |

<a id="jsonrpc-pax"></a>

### Namespace pax

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| pax_associate | HTTP | req *AssociateRequest | none | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/association.go:55 | contracts/evm-rpc.md#pax-associate |
| pax_cleanup | HTTP | none | none | - | retire | 7.2.1 | rpc/filter.go:1206 | contracts/evm-rpc.md#pax-cleanup |
| pax_getBlockByHash | HTTP | blockHash common.Hash; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | change | 7.15.1 | rpc/block.go:176 | contracts/evm-rpc.md#pax-getblockbyhash |
| pax_getBlockByHashExcludeTraceFail | HTTP | blockHash common.Hash; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/block.go:123 | contracts/evm-rpc.md#pax-getblockbyhashexcludetracefail |
| pax_getBlockByNumber | HTTP | number rpc.BlockNumber; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.15.1 | rpc/block.go:210 | contracts/evm-rpc.md#pax-getblockbynumber |
| pax_getBlockByNumberExcludeTraceFail | HTTP | number rpc.BlockNumber; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/block.go:117 | contracts/evm-rpc.md#pax-getblockbynumberexcludetracefail |
| pax_getBlockReceipts | HTTP | blockNrOrHash rpc.BlockNumberOrHash | []map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/block.go:344 | contracts/evm-rpc.md#pax-getblockreceipts |
| pax_getBlockTransactionCountByHash | HTTP | blockHash common.Hash | *hexutil.Uint | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/block.go:154 | contracts/evm-rpc.md#pax-getblocktransactioncountbyhash |
| pax_getBlockTransactionCountByNumber | HTTP | number rpc.BlockNumber | *hexutil.Uint | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.11.2 | rpc/block.go:128 | contracts/evm-rpc.md#pax-getblocktransactioncountbynumber |
| pax_getCosmosTx | HTTP | ethHash common.Hash | string | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/association.go:153 | contracts/evm-rpc.md#pax-getcosmostx |
| pax_getEVMAddress | HTTP | paxAddress string | string | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/association.go:128 | contracts/evm-rpc.md#pax-getevmaddress |
| pax_getEvmTx | HTTP | cosmosHash string | string | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/association.go:179 | contracts/evm-rpc.md#pax-getevmtx |
| pax_getFilterChanges | HTTP | filterID ethrpc.ID | interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/filter.go:876 | contracts/evm-rpc.md#pax-getfilterchanges |
| pax_getFilterLogs | HTTP | filterID ethrpc.ID | []*FilterLog | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/filter.go:989 | contracts/evm-rpc.md#pax-getfilterlogs |
| pax_getLogs | HTTP | crit filters.FilterCriteria | []*FilterLog | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | change | 7.17.1, 7.17.2 | rpc/filter.go:1045 | contracts/evm-rpc.md#pax-getlogs |
| pax_getPaxAddress | HTTP | ethAddress common.Address | string | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/association.go:115 | contracts/evm-rpc.md#pax-getpaxaddress |
| pax_getRawBlock | HTTP | blockNrOrHash rpc.BlockNumberOrHash | hexutil.Bytes | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | retire | 7.3.1 | rpc/tracers.go:723 | contracts/evm-rpc.md#pax-getrawblock |
| pax_getRawHeader | HTTP | blockNrOrHash rpc.BlockNumberOrHash | hexutil.Bytes | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | retire | 7.3.1 | rpc/tracers.go:715 | contracts/evm-rpc.md#pax-getrawheader |
| pax_getRawReceipts | HTTP | blockNrOrHash rpc.BlockNumberOrHash | []hexutil.Bytes | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | retire | 7.3.1 | rpc/tracers.go:731 | contracts/evm-rpc.md#pax-getrawreceipts |
| pax_getRawTransaction | HTTP | hash common.Hash | hexutil.Bytes | - | retire | 7.3.1 | rpc/tracers.go:739 | contracts/evm-rpc.md#pax-getrawtransaction |
| pax_getTransactionByBlockHashAndIndex | HTTP | blockHash common.Hash; txIndex hexutil.Uint | *export.RPCTransaction | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tx.go:242 | contracts/evm-rpc.md#pax-gettransactionbyblockhashandindex |
| pax_getTransactionByBlockNumberAndIndex | HTTP | blockNr rpc.BlockNumber; txIndex hexutil.Uint | *export.RPCTransaction | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.11.3 | rpc/tx.go:208 | contracts/evm-rpc.md#pax-gettransactionbyblocknumberandindex |
| pax_getTransactionByHash | HTTP | hash common.Hash | *export.RPCTransaction | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tx.go:269 | contracts/evm-rpc.md#pax-gettransactionbyhash |
| pax_getTransactionCount | HTTP | address common.Address; blockNrOrHash rpc.BlockNumberOrHash | *hexutil.Uint64 | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tx.go:397 | contracts/evm-rpc.md#pax-gettransactioncount |
| pax_getTransactionErrorByHash | HTTP | hash common.Hash | string | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tx.go:379 | contracts/evm-rpc.md#pax-gettransactionerrorbyhash |
| pax_getTransactionReceipt | HTTP | hash common.Hash | map[string]any | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tx.go:106 | contracts/evm-rpc.md#pax-gettransactionreceipt |
| pax_getTransactionReceiptExcludeTraceFail | HTTP | hash common.Hash | map[string]any | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tx.go:102 | contracts/evm-rpc.md#pax-gettransactionreceiptexcludetracefail |
| pax_getVMError | HTTP | hash common.Hash | string | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tx.go:193 | contracts/evm-rpc.md#pax-getvmerror |
| pax_newBlockFilter | HTTP | none | ethrpc.ID | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/filter.go:814 | contracts/evm-rpc.md#pax-newblockfilter |
| pax_newFilter | HTTP | crit filters.FilterCriteria | ethrpc.ID | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/filter.go:767 | contracts/evm-rpc.md#pax-newfilter |
| pax_newPendingTransactionFilter | HTTP | fullTx *bool | ethrpc.ID | - | retire | 7.9.2 | rpc/filter.go:840 | contracts/evm-rpc.md#pax-newpendingtransactionfilter |
| pax_sequencerReceipt | HTTP | txHash common.Hash | *SequencerReceiptResult | -32000 served by sequencer (rpc/sequencer_receipt.go:49) | stable | - | rpc/sequencer_receipt.go:232 | contracts/evm-rpc.md#pax-sequencerreceipt |
| pax_setReceiptLogFeed | HTTP | feed *ReceiptLogFeed | none | - | retire | 7.2.3 | rpc/filter.go:583 | contracts/evm-rpc.md#pax-setreceiptlogfeed |
| pax_sign | HTTP | addr common.Address; data hexutil.Bytes | hexutil.Bytes | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tx.go:530 | contracts/evm-rpc.md#pax-sign |
| pax_traceBlockByHash | HTTP | hash common.Hash; config *tracers.TraceConfig | interface{} | - | retire | 7.3.1 | rpc/tracers.go:667 | contracts/evm-rpc.md#pax-traceblockbyhash |
| pax_traceBlockByHashExcludeTraceFail | HTTP | hash common.Hash; config *tracers.TraceConfig | interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/tracers.go:556 | contracts/evm-rpc.md#pax-traceblockbyhashexcludetracefail |
| pax_traceBlockByNumber | HTTP | number rpc.BlockNumber; config *tracers.TraceConfig | interface{} | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | retire | 7.3.1 | rpc/tracers.go:639 | contracts/evm-rpc.md#pax-traceblockbynumber |
| pax_traceBlockByNumberExcludeTraceFail | HTTP | number rpc.BlockNumber; config *tracers.TraceConfig | interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/tracers.go:520 | contracts/evm-rpc.md#pax-traceblockbynumberexcludetracefail |
| pax_traceCall | HTTP | args export.TransactionArgs; blockNrOrHash rpc.BlockNumberOrHash; config *tracers.TraceCallConfig | interface{} | -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | retire | 7.3.1 | rpc/tracers.go:695 | contracts/evm-rpc.md#pax-tracecall |
| pax_traceStateAccess | HTTP | hash common.Hash | interface{} | - | retire | 7.3.1 | rpc/tracers.go:753 | contracts/evm-rpc.md#pax-tracestateaccess |
| pax_traceTransaction | HTTP | hash common.Hash; config *tracers.TraceConfig | interface{} | - | retire | 7.3.1 | rpc/tracers.go:282 | contracts/evm-rpc.md#pax-tracetransaction |
| pax_traceTransactionProfile | HTTP | hash common.Hash; config *tracers.TraceConfig | interface{} | - | retire | 7.3.1 | rpc/trace_profile.go:45 | contracts/evm-rpc.md#pax-tracetransactionprofile |
| pax_uninstallFilter | HTTP | filterID ethrpc.ID | bool | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/filter.go:1176 | contracts/evm-rpc.md#pax-uninstallfilter |

<a id="jsonrpc-pax2"></a>

### Namespace pax2

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| pax2_getBlockByHash | HTTP | blockHash common.Hash; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | change | 7.15.1 | rpc/block.go:176 | contracts/evm-rpc.md#pax2-getblockbyhash |
| pax2_getBlockByHashExcludeTraceFail | HTTP | blockHash common.Hash; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/block.go:123 | contracts/evm-rpc.md#pax2-getblockbyhashexcludetracefail |
| pax2_getBlockByNumber | HTTP | number rpc.BlockNumber; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.15.1 | rpc/block.go:210 | contracts/evm-rpc.md#pax2-getblockbynumber |
| pax2_getBlockByNumberExcludeTraceFail | HTTP | number rpc.BlockNumber; fullTx bool | map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/block.go:117 | contracts/evm-rpc.md#pax2-getblockbynumberexcludetracefail |
| pax2_getBlockReceipts | HTTP | blockNrOrHash rpc.BlockNumberOrHash | []map[string]interface{} | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | stable | - | rpc/block.go:344 | contracts/evm-rpc.md#pax2-getblockreceipts |
| pax2_getBlockTransactionCountByHash | HTTP | blockHash common.Hash | *hexutil.Uint | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42) | stable | - | rpc/block.go:154 | contracts/evm-rpc.md#pax2-getblocktransactioncountbyhash |
| pax2_getBlockTransactionCountByNumber | HTTP | number rpc.BlockNumber | *hexutil.Uint | -32601 when not in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:42); -32000 pending served by sequencer for the pending tag off the sequencer (rpc/utils.go:138; rpc/watermark_manager.go:59) | change | 7.11.2 | rpc/block.go:128 | contracts/evm-rpc.md#pax2-getblocktransactioncountbynumber |

<a id="jsonrpc-rpc"></a>

### Namespace rpc

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| rpc_modules | HTTP, WS | none | map[string]string | - | stable | - | core/deps/geth/rpc/server.go:231 | contracts/evm-rpc.md#rpc-modules |

<a id="jsonrpc-test"></a>

### Namespace test

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| test_incrementPointerVersion | HTTP | pointerType string; offset int16 | none | - | stable | - | rpc/test.go:15 | contracts/evm-rpc.md#test-incrementpointerversion |

<a id="jsonrpc-txpool"></a>

### Namespace txpool

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| txpool_content | HTTP | none | map[string]map[string]map[string]*export.RPCTransaction | - | change | 7.7.1 | rpc/txpool.go:40 | contracts/evm-rpc.md#txpool-content |

<a id="jsonrpc-web3"></a>

### Namespace web3

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| web3_clientVersion | HTTP, WS | none | string | - | stable | - | rpc/web3.go:7 | contracts/evm-rpc.md#web3-clientversion |

<a id="jsonrpc-ws"></a>

### Namespace ws

| Method | Transport | Params | Returns | Errors | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| ws_port | WS subscription | w http.ResponseWriter; r *http.Request | none | - | change | 7.9.1, 7.9.2 | rpc/rpcstack.go:211 | contracts/evm-rpc.md#ws-endpoint |

<a id="websocket"></a>

## WebSocket

The WebSocket server registers the namespaces listed in rpc/server.go:394-430; eth_subscribe subscriptions are the ws_route rows below. Pending-tag subscriptions on the sequencer are frozen in contracts/sequencer.md.

| Subscription | Params | Status | Tasks | Defined at | Notes |
|---|---|---|---|---|---|
| ws_port | w http.ResponseWriter; r *http.Request | change | 7.9.1, 7.9.2 | rpc/rpcstack.go:211 | WebSocket JSON-RPC on evm.ws_port, upgraded in ServeHTTP when the path matches the WS prefix (rpc/rpcstack.go:213-220); the listener binds LocalAddress the all-interfaces address (rpc/server.go:361... |
| eth_subscribe | filter *filters.FilterCriteria | change | 7.10.4, 7.10.5 | rpc/subscribe.go:230 | eth_subscribe logs; WS; bounded by evm.filter_timeout, evm.max_log_no_block and evm.max_blocks_for_log (rpc/server.go:385); 3 caller site(s) |
| eth_subscribe | none | change | 7.10.1, 7.10.2, 7.10.3 | rpc/subscribe.go:185 | eth_subscribe newHeads; WS; capped by evm.max_subscriptions_new_head (rpc/server.go:385); node/app.go:556 pairs FinalizeBlock with Commit for the head event and node/newheads_test.go:19 covers the ... |
| eth_subscribe | none | stable | - | rpc/send.go:357 | eth_subscribe newPendingTransactions; registered on HTTP and WS through SendAPI; refused with ErrPendingServedBySequencer off the sequencer (rpc/send.go:362-365) and with notifications not supporte... |

<a id="grpc"></a>

## gRPC services of domain 7

### Service methods

| Full method | Request fields | Response | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|
| cosmos.base.reflection.v1beta1.ReflectionService/ListAllInterfaces | ListAllInterfacesRequest {} | ListAllInterfacesResponse {interface_names} | stable | - | sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:12 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39 |
| cosmos.base.reflection.v1beta1.ReflectionService/ListImplementations | ListImplementationsRequest {interface_name} | ListImplementationsResponse {implementation_message_names} | stable | - | sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:18 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39 |
| cosmos.base.reflection.v2alpha1.ReflectionService/GetAuthnDescriptor | GetAuthnDescriptorRequest {} | GetAuthnDescriptorResponse {authn} | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:119 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39 |
| cosmos.base.reflection.v2alpha1.ReflectionService/GetChainDescriptor | GetChainDescriptorRequest {} | GetChainDescriptorResponse {chain} | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:123 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39 |
| cosmos.base.reflection.v2alpha1.ReflectionService/GetCodecDescriptor | GetCodecDescriptorRequest {} | GetCodecDescriptorResponse {codec} | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:127 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39 |
| cosmos.base.reflection.v2alpha1.ReflectionService/GetConfigurationDescriptor | GetConfigurationDescriptorRequest {} | GetConfigurationDescriptorResponse {config} | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:131 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39 |
| cosmos.base.reflection.v2alpha1.ReflectionService/GetQueryServicesDescriptor | GetQueryServicesDescriptorRequest {} | GetQueryServicesDescriptorResponse {queries} | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:135 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39 |
| cosmos.base.reflection.v2alpha1.ReflectionService/GetTxDescriptor | GetTxDescriptorRequest {} | GetTxDescriptorResponse {tx} | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:139 | contracts/proto-api.md#grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39 |
| cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight | GetBlockByHeightRequest {height} | GetBlockByHeightResponse {block_id, block} | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:28 | contracts/proto-api.md#grpcs-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39 |
| cosmos.base.tendermint.v1beta1.Service/GetLatestBlock | GetLatestBlockRequest {} | GetLatestBlockResponse {block_id, block} | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:24 | contracts/proto-api.md#grpcs-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39 |
| cosmos.base.tendermint.v1beta1.Service/GetLatestValidatorSet | GetLatestValidatorSetRequest {pagination} | GetLatestValidatorSetResponse {block_height, validators, pagination} | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:33 | contracts/proto-api.md#grpcs-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39 |
| cosmos.base.tendermint.v1beta1.Service/GetNodeInfo | GetNodeInfoRequest {} | GetNodeInfoResponse {default_node_info, application_version} | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:16 | contracts/proto-api.md#grpcs-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39 |
| cosmos.base.tendermint.v1beta1.Service/GetSyncing | GetSyncingRequest {} | GetSyncingResponse {syncing} | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:20 | contracts/proto-api.md#grpcs-cosmos-base-tendermint-v1beta1-service-getsyncing-j39 |
| cosmos.base.tendermint.v1beta1.Service/GetValidatorSetByHeight | GetValidatorSetByHeightRequest {height, pagination} | GetValidatorSetByHeightResponse {block_height, validators, pagination} | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:37 | contracts/proto-api.md#grpcs-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39 |
| cosmos.tx.v1beta1.Service/BroadcastTx | BroadcastTxRequest {tx_bytes, mode} | BroadcastTxResponse {tx_response} | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:29 | contracts/proto-api.md#grpcs-cosmos-tx-v1beta1-service-broadcasttx-j39 |
| cosmos.tx.v1beta1.Service/GetBlockWithTxs | GetBlockWithTxsRequest {height, pagination} | GetBlockWithTxsResponse {txs, block_id, block, pagination} | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:42 | contracts/proto-api.md#grpcs-cosmos-tx-v1beta1-service-getblockwithtxs-j39 |
| cosmos.tx.v1beta1.Service/GetTx | GetTxRequest {hash} | GetTxResponse {tx, tx_response} | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:25 | contracts/proto-api.md#grpcs-cosmos-tx-v1beta1-service-gettx-j39 |
| cosmos.tx.v1beta1.Service/GetTxsEvent | GetTxsEventRequest {events, pagination, order_by} | GetTxsEventResponse {txs, tx_responses, pagination} | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:36 | contracts/proto-api.md#grpcs-cosmos-tx-v1beta1-service-gettxsevent-j39 |
| cosmos.tx.v1beta1.Service/Simulate | SimulateRequest {tx, tx_bytes} | SimulateResponse {gas_info, result} | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:18 | contracts/proto-api.md#grpcs-cosmos-tx-v1beta1-service-simulate-j39 |
| paxprotocol.paxchain.admin.v0.AdminService/GetLogLevel | GetLogLevelRequest {logger} | GetLogLevelResponse {logger, level} | stable | - | api/pax/admin/v0/admin.proto:13 | contracts/rpc.md#grpcs-pax-admin-getloglevel |
| paxprotocol.paxchain.admin.v0.AdminService/ListLoggers | ListLoggersRequest {prefix} | ListLoggersResponse {loggers} | stable | - | api/pax/admin/v0/admin.proto:15 | contracts/rpc.md#grpcs-pax-admin-listloggers |
| paxprotocol.paxchain.admin.v0.AdminService/SetLogLevel | SetLogLevelRequest {pattern, level} | SetLogLevelResponse {pattern, level, affected} | stable | - | api/pax/admin/v0/admin.proto:11 | contracts/rpc.md#grpcs-pax-admin-setloglevel |

### Queries

| Full method | Request fields | Response | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|
| cosmos.auth.v1beta1.Query/Account | QueryAccountRequest {address} | QueryAccountResponse {account} | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:23 | contracts/proto-api.md#qry-cosmos-auth-v1beta1-account-j39 |
| cosmos.auth.v1beta1.Query/Accounts | QueryAccountsRequest {pagination} | QueryAccountsResponse {accounts, pagination} | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:18 | contracts/proto-api.md#qry-cosmos-auth-v1beta1-accounts-j39 |
| cosmos.auth.v1beta1.Query/NextAccountNumber | QueryNextAccountNumberRequest {} | QueryNextAccountNumberResponse {count} | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:33 | contracts/proto-api.md#qry-cosmos-auth-v1beta1-nextaccountnumber-j39 |
| cosmos.auth.v1beta1.Query/Params | QueryParamsRequest {} | QueryParamsResponse {params} | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:28 | contracts/proto-api.md#qry-cosmos-auth-v1beta1-params-j39 |
| cosmos.authz.v1beta1.Query/GranteeGrants | QueryGranteeGrantsRequest {grantee, pagination} | QueryGranteeGrantsResponse {grants, pagination} | stable | - | sdk/proto/cosmos/authz/v1beta1/query.proto:28 | contracts/proto-api.md#qry-cosmos-authz-v1beta1-granteegrants-j39 |
| cosmos.authz.v1beta1.Query/GranterGrants | QueryGranterGrantsRequest {granter, pagination} | QueryGranterGrantsResponse {grants, pagination} | stable | - | sdk/proto/cosmos/authz/v1beta1/query.proto:21 | contracts/proto-api.md#qry-cosmos-authz-v1beta1-grantergrants-j39 |
| cosmos.authz.v1beta1.Query/Grants | QueryGrantsRequest {granter, grantee, msg_type_url, pagination} | QueryGrantsResponse {grants, pagination} | stable | - | sdk/proto/cosmos/authz/v1beta1/query.proto:14 | contracts/proto-api.md#qry-cosmos-authz-v1beta1-grants-j39 |
| cosmos.bank.v1beta1.Query/AllBalances | QueryAllBalancesRequest {address, pagination} | QueryAllBalancesResponse {balances, pagination} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:20 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-allbalances-j39 |
| cosmos.bank.v1beta1.Query/Balance | QueryBalanceRequest {address, denom} | QueryBalanceResponse {balance} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:15 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-balance-j39 |
| cosmos.bank.v1beta1.Query/DenomMetadata | QueryDenomMetadataRequest {denom} | QueryDenomMetadataResponse {metadata} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:46 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-denommetadata-j39 |
| cosmos.bank.v1beta1.Query/DenomsMetadata | QueryDenomsMetadataRequest {pagination} | QueryDenomsMetadataResponse {metadatas, pagination} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:51 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-denomsmetadata-j39 |
| cosmos.bank.v1beta1.Query/Params | QueryParamsRequest {} | QueryParamsResponse {params} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:41 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-params-j39 |
| cosmos.bank.v1beta1.Query/SpendableBalances | QuerySpendableBalancesRequest {address, pagination} | QuerySpendableBalancesResponse {balances, pagination} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:26 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-spendablebalances-j39 |
| cosmos.bank.v1beta1.Query/SupplyOf | QuerySupplyOfRequest {denom} | QuerySupplyOfResponse {amount} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:36 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-supplyof-j39 |
| cosmos.bank.v1beta1.Query/TotalSupply | QueryTotalSupplyRequest {pagination} | QueryTotalSupplyResponse {supply, pagination} | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:31 | contracts/proto-api.md#qry-cosmos-bank-v1beta1-totalsupply-j39 |
| cosmos.distribution.v1beta1.Query/CommunityPool | QueryCommunityPoolRequest {} | QueryCommunityPoolResponse {pool} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:66 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-communitypool-j39 |
| cosmos.distribution.v1beta1.Query/DelegationRewards | QueryDelegationRewardsRequest {delegator_address, validator_address} | QueryDelegationRewardsResponse {rewards} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:39 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-delegationrewards-j39 |
| cosmos.distribution.v1beta1.Query/DelegationTotalRewards | QueryDelegationTotalRewardsRequest {delegator_address} | QueryDelegationTotalRewardsResponse {rewards, total} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:47 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-delegationtotalrewards-j39 |
| cosmos.distribution.v1beta1.Query/DelegatorValidators | QueryDelegatorValidatorsRequest {delegator_address} | QueryDelegatorValidatorsResponse {validators} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:52 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-delegatorvalidators-j39 |
| cosmos.distribution.v1beta1.Query/DelegatorWithdrawAddress | QueryDelegatorWithdrawAddressRequest {delegator_address} | QueryDelegatorWithdrawAddressResponse {withdraw_address} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:59 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39 |
| cosmos.distribution.v1beta1.Query/Params | QueryParamsRequest {} | QueryParamsResponse {params} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:15 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-params-j39 |
| cosmos.distribution.v1beta1.Query/ValidatorCommission | QueryValidatorCommissionRequest {validator_address} | QueryValidatorCommissionResponse {commission} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:27 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-validatorcommission-j39 |
| cosmos.distribution.v1beta1.Query/ValidatorOutstandingRewards | QueryValidatorOutstandingRewardsRequest {validator_address} | QueryValidatorOutstandingRewardsResponse {rewards} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:20 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39 |
| cosmos.distribution.v1beta1.Query/ValidatorSlashes | QueryValidatorSlashesRequest {validator_address, starting_height, ending_height, pagination} | QueryValidatorSlashesResponse {slashes, pagination} | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:34 | contracts/proto-api.md#qry-cosmos-distribution-v1beta1-validatorslashes-j39 |
| cosmos.evidence.v1beta1.Query/AllEvidence | QueryAllEvidenceRequest {pagination} | QueryAllEvidenceResponse {evidence, pagination} | stable | - | sdk/proto/cosmos/evidence/v1beta1/query.proto:19 | contracts/proto-api.md#qry-cosmos-evidence-v1beta1-allevidence-j39 |
| cosmos.evidence.v1beta1.Query/Evidence | QueryEvidenceRequest {evidence_hash} | QueryEvidenceResponse {evidence} | stable | - | sdk/proto/cosmos/evidence/v1beta1/query.proto:14 | contracts/proto-api.md#qry-cosmos-evidence-v1beta1-evidence-j39 |
| cosmos.feegrant.v1beta1.Query/Allowance | QueryAllowanceRequest {granter, grantee} | QueryAllowanceResponse {allowance} | stable | - | sdk/proto/cosmos/feegrant/v1beta1/query.proto:14 | contracts/proto-api.md#qry-cosmos-feegrant-v1beta1-allowance-j39 |
| cosmos.feegrant.v1beta1.Query/Allowances | QueryAllowancesRequest {grantee, pagination} | QueryAllowancesResponse {allowances, pagination} | stable | - | sdk/proto/cosmos/feegrant/v1beta1/query.proto:19 | contracts/proto-api.md#qry-cosmos-feegrant-v1beta1-allowances-j39 |
| cosmos.feegrant.v1beta1.Query/AllowancesByGranter | QueryAllowancesByGranterRequest {granter, pagination} | QueryAllowancesByGranterResponse {allowances, pagination} | stable | - | sdk/proto/cosmos/feegrant/v1beta1/query.proto:25 | contracts/proto-api.md#qry-cosmos-feegrant-v1beta1-allowancesbygranter-j39 |
| cosmos.gov.v1beta1.Query/Deposit | QueryDepositRequest {proposal_id, depositor} | QueryDepositResponse {deposit} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:39 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-deposit-j39 |
| cosmos.gov.v1beta1.Query/Deposits | QueryDepositsRequest {proposal_id, pagination} | QueryDepositsResponse {deposits, pagination} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:44 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-deposits-j39 |
| cosmos.gov.v1beta1.Query/Params | QueryParamsRequest {params_type} | QueryParamsResponse {voting_params, deposit_params, tally_params} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:34 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-params-j39 |
| cosmos.gov.v1beta1.Query/Proposal | QueryProposalRequest {proposal_id} | QueryProposalResponse {proposal} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:14 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-proposal-j39 |
| cosmos.gov.v1beta1.Query/Proposals | QueryProposalsRequest {proposal_status, voter, depositor, pagination} | QueryProposalsResponse {proposals, pagination} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:19 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-proposals-j39 |
| cosmos.gov.v1beta1.Query/TallyResult | QueryTallyResultRequest {proposal_id} | QueryTallyResultResponse {tally} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:49 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-tallyresult-j39 |
| cosmos.gov.v1beta1.Query/Vote | QueryVoteRequest {proposal_id, voter} | QueryVoteResponse {vote} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:24 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-vote-j39 |
| cosmos.gov.v1beta1.Query/Votes | QueryVotesRequest {proposal_id, pagination} | QueryVotesResponse {votes, pagination} | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:29 | contracts/proto-api.md#qry-cosmos-gov-v1beta1-votes-j39 |
| cosmos.params.v1beta1.Query/Params | QueryParamsRequest {subspace, key} | QueryParamsResponse {param} | stable | - | sdk/proto/cosmos/params/v1beta1/query.proto:14 | contracts/proto-api.md#qry-cosmos-params-v1beta1-params-j39 |
| cosmos.slashing.v1beta1.Query/Params | QueryParamsRequest {} | QueryParamsResponse {params} | stable | - | sdk/proto/cosmos/slashing/v1beta1/query.proto:14 | contracts/proto-api.md#qry-cosmos-slashing-v1beta1-params-j39 |
| cosmos.slashing.v1beta1.Query/SigningInfo | QuerySigningInfoRequest {cons_address} | QuerySigningInfoResponse {val_signing_info} | stable | - | sdk/proto/cosmos/slashing/v1beta1/query.proto:19 | contracts/proto-api.md#qry-cosmos-slashing-v1beta1-signinginfo-j39 |
| cosmos.slashing.v1beta1.Query/SigningInfos | QuerySigningInfosRequest {pagination} | QuerySigningInfosResponse {info, pagination} | stable | - | sdk/proto/cosmos/slashing/v1beta1/query.proto:24 | contracts/proto-api.md#qry-cosmos-slashing-v1beta1-signinginfos-j39 |
| cosmos.staking.v1beta1.Query/Delegation | QueryDelegationRequest {delegator_addr, validator_addr} | QueryDelegationResponse {delegation_response} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:36 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-delegation-j39 |
| cosmos.staking.v1beta1.Query/DelegatorDelegations | QueryDelegatorDelegationsRequest {delegator_addr, pagination} | QueryDelegatorDelegationsResponse {delegation_responses, pagination} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:51 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-delegatordelegations-j39 |
| cosmos.staking.v1beta1.Query/DelegatorUnbondingDelegations | QueryDelegatorUnbondingDelegationsRequest {delegator_addr, pagination} | QueryDelegatorUnbondingDelegationsResponse {unbonding_responses, pagination} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:57 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39 |
| cosmos.staking.v1beta1.Query/DelegatorValidator | QueryDelegatorValidatorRequest {delegator_addr, validator_addr} | QueryDelegatorValidatorResponse {validator} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:76 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-delegatorvalidator-j39 |
| cosmos.staking.v1beta1.Query/DelegatorValidators | QueryDelegatorValidatorsRequest {delegator_addr, pagination} | QueryDelegatorValidatorsResponse {validators, pagination} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:70 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-delegatorvalidators-j39 |
| cosmos.staking.v1beta1.Query/HistoricalInfo | QueryHistoricalInfoRequest {height} | QueryHistoricalInfoResponse {hist} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:83 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-historicalinfo-j39 |
| cosmos.staking.v1beta1.Query/Params | QueryParamsRequest {} | QueryParamsResponse {params} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:93 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-params-j39 |
| cosmos.staking.v1beta1.Query/Pool | QueryPoolRequest {} | QueryPoolResponse {pool} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:88 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-pool-j39 |
| cosmos.staking.v1beta1.Query/Redelegations | QueryRedelegationsRequest {delegator_addr, src_validator_addr, dst_validator_addr, pagination} | QueryRedelegationsResponse {redelegation_responses, pagination} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:64 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-redelegations-j39 |
| cosmos.staking.v1beta1.Query/UnbondingDelegation | QueryUnbondingDelegationRequest {delegator_addr, validator_addr} | QueryUnbondingDelegationResponse {unbond} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:44 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-unbondingdelegation-j39 |
| cosmos.staking.v1beta1.Query/Validator | QueryValidatorRequest {validator_addr} | QueryValidatorResponse {validator} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:19 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-validator-j39 |
| cosmos.staking.v1beta1.Query/ValidatorDelegations | QueryValidatorDelegationsRequest {validator_addr, pagination} | QueryValidatorDelegationsResponse {delegation_responses, pagination} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:24 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-validatordelegations-j39 |
| cosmos.staking.v1beta1.Query/ValidatorUnbondingDelegations | QueryValidatorUnbondingDelegationsRequest {validator_addr, pagination} | QueryValidatorUnbondingDelegationsResponse {unbonding_responses, pagination} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:29 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-validatorunbondingdelegations-j39 |
| cosmos.staking.v1beta1.Query/Validators | QueryValidatorsRequest {status, pagination} | QueryValidatorsResponse {validators, pagination} | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:14 | contracts/proto-api.md#qry-cosmos-staking-v1beta1-validators-j39 |
| cosmos.upgrade.v1beta1.Query/AppliedPlan | QueryAppliedPlanRequest {name} | QueryAppliedPlanResponse {height} | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:17 | contracts/proto-api.md#qry-cosmos-upgrade-v1beta1-appliedplan-j39 |
| cosmos.upgrade.v1beta1.Query/CurrentPlan | QueryCurrentPlanRequest {} | QueryCurrentPlanResponse {plan} | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:12 | contracts/proto-api.md#qry-cosmos-upgrade-v1beta1-currentplan-j39 |
| cosmos.upgrade.v1beta1.Query/ModuleVersions | QueryModuleVersionsRequest {module_name} | QueryModuleVersionsResponse {module_versions} | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:35 | contracts/proto-api.md#qry-cosmos-upgrade-v1beta1-moduleversions-j39 |
| cosmos.upgrade.v1beta1.Query/UpgradedConsensusState | QueryUpgradedConsensusStateRequest {last_height} | QueryUpgradedConsensusStateResponse {upgraded_consensus_state} | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:27 | contracts/proto-api.md#qry-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39 |

### Msg services

Transaction messages are submitted through the cosmos tx service and are listed here for completeness.

| Message | Defined at | Status | Tasks | Contract |
|---|---|---|---|---|
| cosmos.authz.v1beta1.Msg/Exec: rpc Exec(MsgExec) returns (MsgExecResponse) | sdk/proto/cosmos/authz/v1beta1/tx.proto:24 | stable | - | contracts/proto-api.md#msg-cosmos-authz-v1beta1-exec-j39 |
| cosmos.authz.v1beta1.Msg/Grant: rpc Grant(MsgGrant) returns (MsgGrantResponse) | sdk/proto/cosmos/authz/v1beta1/tx.proto:19 | stable | - | contracts/proto-api.md#msg-cosmos-authz-v1beta1-grant-j39 |
| cosmos.authz.v1beta1.Msg/Revoke: rpc Revoke(MsgRevoke) returns (MsgRevokeResponse) | sdk/proto/cosmos/authz/v1beta1/tx.proto:28 | stable | - | contracts/proto-api.md#msg-cosmos-authz-v1beta1-revoke-j39 |
| cosmos.bank.v1beta1.Msg/MultiSend: rpc MultiSend(MsgMultiSend) returns (MsgMultiSendResponse) | sdk/proto/cosmos/bank/v1beta1/tx.proto:16 | stable | - | contracts/proto-api.md#msg-cosmos-bank-v1beta1-multisend-j39 |
| cosmos.bank.v1beta1.Msg/Send: rpc Send(MsgSend) returns (MsgSendResponse) | sdk/proto/cosmos/bank/v1beta1/tx.proto:13 | stable | - | contracts/proto-api.md#msg-cosmos-bank-v1beta1-send-j39 |
| cosmos.distribution.v1beta1.Msg/FundCommunityPool: rpc FundCommunityPool(MsgFundCommunityPool) returns (Msg... | sdk/proto/cosmos/distribution/v1beta1/tx.proto:26 | stable | - | contracts/proto-api.md#msg-cosmos-distribution-v1beta1-fundcommunitypool-j39 |
| cosmos.distribution.v1beta1.Msg/SetWithdrawAddress: rpc SetWithdrawAddress(MsgSetWithdrawAddress) returns (... | sdk/proto/cosmos/distribution/v1beta1/tx.proto:14 | stable | - | contracts/proto-api.md#msg-cosmos-distribution-v1beta1-setwithdrawaddress-j39 |
| cosmos.distribution.v1beta1.Msg/WithdrawDelegatorReward: rpc WithdrawDelegatorReward(MsgWithdrawDelegatorRe... | sdk/proto/cosmos/distribution/v1beta1/tx.proto:18 | stable | - | contracts/proto-api.md#msg-cosmos-distribution-v1beta1-withdrawdelegatorreward-j39 |
| cosmos.distribution.v1beta1.Msg/WithdrawValidatorCommission: rpc WithdrawValidatorCommission(MsgWithdrawVal... | sdk/proto/cosmos/distribution/v1beta1/tx.proto:22 | stable | - | contracts/proto-api.md#msg-cosmos-distribution-v1beta1-withdrawvalidatorcommission-j39 |
| cosmos.evidence.v1beta1.Msg/SubmitEvidence: rpc SubmitEvidence(MsgSubmitEvidence) returns (MsgSubmitEvidenc... | sdk/proto/cosmos/evidence/v1beta1/tx.proto:15 | stable | - | contracts/proto-api.md#msg-cosmos-evidence-v1beta1-submitevidence-j39 |
| cosmos.feegrant.v1beta1.Msg/GrantAllowance: rpc GrantAllowance(MsgGrantAllowance) returns (MsgGrantAllowanc... | sdk/proto/cosmos/feegrant/v1beta1/tx.proto:14 | stable | - | contracts/proto-api.md#msg-cosmos-feegrant-v1beta1-grantallowance-j39 |
| cosmos.feegrant.v1beta1.Msg/RevokeAllowance: rpc RevokeAllowance(MsgRevokeAllowance) returns (MsgRevokeAllo... | sdk/proto/cosmos/feegrant/v1beta1/tx.proto:18 | stable | - | contracts/proto-api.md#msg-cosmos-feegrant-v1beta1-revokeallowance-j39 |
| cosmos.gov.v1beta1.Msg/Deposit: rpc Deposit(MsgDeposit) returns (MsgDepositResponse) | sdk/proto/cosmos/gov/v1beta1/tx.proto:26 | stable | - | contracts/proto-api.md#msg-cosmos-gov-v1beta1-deposit-j39 |
| cosmos.gov.v1beta1.Msg/SubmitProposal: rpc SubmitProposal(MsgSubmitProposal) returns (MsgSubmitProposalResp... | sdk/proto/cosmos/gov/v1beta1/tx.proto:15 | stable | - | contracts/proto-api.md#msg-cosmos-gov-v1beta1-submitproposal-j39 |
| cosmos.gov.v1beta1.Msg/Vote: rpc Vote(MsgVote) returns (MsgVoteResponse) | sdk/proto/cosmos/gov/v1beta1/tx.proto:18 | stable | - | contracts/proto-api.md#msg-cosmos-gov-v1beta1-vote-j39 |
| cosmos.gov.v1beta1.Msg/VoteWeighted: rpc VoteWeighted(MsgVoteWeighted) returns (MsgVoteWeightedResponse) | sdk/proto/cosmos/gov/v1beta1/tx.proto:23 | stable | - | contracts/proto-api.md#msg-cosmos-gov-v1beta1-voteweighted-j39 |
| cosmos.slashing.v1beta1.Msg/Unjail: rpc Unjail(MsgUnjail) returns (MsgUnjailResponse) | sdk/proto/cosmos/slashing/v1beta1/tx.proto:14 | stable | - | contracts/proto-api.md#msg-cosmos-slashing-v1beta1-unjail-j39 |
| cosmos.staking.v1beta1.Msg/BeginRedelegate: rpc BeginRedelegate(MsgBeginRedelegate) returns (MsgBeginRedele... | sdk/proto/cosmos/staking/v1beta1/tx.proto:27 | stable | - | contracts/proto-api.md#msg-cosmos-staking-v1beta1-beginredelegate-j39 |
| cosmos.staking.v1beta1.Msg/CreateValidator: rpc CreateValidator(MsgCreateValidator) returns (MsgCreateValid... | sdk/proto/cosmos/staking/v1beta1/tx.proto:16 | stable | - | contracts/proto-api.md#msg-cosmos-staking-v1beta1-createvalidator-j39 |
| cosmos.staking.v1beta1.Msg/Delegate: rpc Delegate(MsgDelegate) returns (MsgDelegateResponse) | sdk/proto/cosmos/staking/v1beta1/tx.proto:23 | stable | - | contracts/proto-api.md#msg-cosmos-staking-v1beta1-delegate-j39 |
| cosmos.staking.v1beta1.Msg/EditValidator: rpc EditValidator(MsgEditValidator) returns (MsgEditValidatorResp... | sdk/proto/cosmos/staking/v1beta1/tx.proto:19 | stable | - | contracts/proto-api.md#msg-cosmos-staking-v1beta1-editvalidator-j39 |
| cosmos.staking.v1beta1.Msg/Undelegate: rpc Undelegate(MsgUndelegate) returns (MsgUndelegateResponse) | sdk/proto/cosmos/staking/v1beta1/tx.proto:31 | stable | - | contracts/proto-api.md#msg-cosmos-staking-v1beta1-undelegate-j39 |
| cosmos.vesting.v1beta1.Msg/CreateVestingAccount: rpc CreateVestingAccount(MsgCreateVestingAccount) returns ... | sdk/proto/cosmos/vesting/v1beta1/tx.proto:13 | stable | - | contracts/proto-api.md#msg-cosmos-vesting-v1beta1-createvestingaccount-j39 |

<a id="rest"></a>

## REST routes of domain 7

| Route | Path params | Query params | Returns | gRPC method | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| GET /cosmos/auth/v1beta1/accounts | none | pagination | QueryAccountsResponse {accounts, pagination} | cosmos.auth.v1beta1.Query/Accounts | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:19 | contracts/proto-api.md#rest-cosmos-auth-v1beta1-accounts-j39 |
| GET /cosmos/auth/v1beta1/accounts/{address} | address | none | QueryAccountResponse {account} | cosmos.auth.v1beta1.Query/Account | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:24 | contracts/proto-api.md#rest-cosmos-auth-v1beta1-account-j39 |
| GET /cosmos/auth/v1beta1/nextaccountnumber | none | none | QueryNextAccountNumberResponse {count} | cosmos.auth.v1beta1.Query/NextAccountNumber | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:34 | contracts/proto-api.md#rest-cosmos-auth-v1beta1-nextaccountnumber-j39 |
| GET /cosmos/auth/v1beta1/params | none | none | QueryParamsResponse {params} | cosmos.auth.v1beta1.Query/Params | stable | - | sdk/proto/cosmos/auth/v1beta1/query.proto:29 | contracts/proto-api.md#rest-cosmos-auth-v1beta1-params-j39 |
| GET /cosmos/authz/v1beta1/grants | none | granter, grantee, msg_type_url, pagination | QueryGrantsResponse {grants, pagination} | cosmos.authz.v1beta1.Query/Grants | stable | - | sdk/proto/cosmos/authz/v1beta1/query.proto:15 | contracts/proto-api.md#rest-cosmos-authz-v1beta1-grants-j39 |
| GET /cosmos/authz/v1beta1/grants/grantee/{grantee} | grantee | pagination | QueryGranteeGrantsResponse {grants, pagination} | cosmos.authz.v1beta1.Query/GranteeGrants | stable | - | sdk/proto/cosmos/authz/v1beta1/query.proto:29 | contracts/proto-api.md#rest-cosmos-authz-v1beta1-granteegrants-j39 |
| GET /cosmos/authz/v1beta1/grants/granter/{granter} | granter | pagination | QueryGranterGrantsResponse {grants, pagination} | cosmos.authz.v1beta1.Query/GranterGrants | stable | - | sdk/proto/cosmos/authz/v1beta1/query.proto:22 | contracts/proto-api.md#rest-cosmos-authz-v1beta1-grantergrants-j39 |
| GET /cosmos/bank/v1beta1/balances/{address} | address | pagination | QueryAllBalancesResponse {balances, pagination} | cosmos.bank.v1beta1.Query/AllBalances | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:21 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-allbalances-j39 |
| GET /cosmos/bank/v1beta1/balances/{address}/by_denom | address | denom | QueryBalanceResponse {balance} | cosmos.bank.v1beta1.Query/Balance | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:16 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-balance-j39 |
| GET /cosmos/bank/v1beta1/denoms_metadata | none | pagination | QueryDenomsMetadataResponse {metadatas, pagination} | cosmos.bank.v1beta1.Query/DenomsMetadata | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:52 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-denomsmetadata-j39 |
| GET /cosmos/bank/v1beta1/denoms_metadata/{denom} | denom | none | QueryDenomMetadataResponse {metadata} | cosmos.bank.v1beta1.Query/DenomMetadata | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:47 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-denommetadata-j39 |
| GET /cosmos/bank/v1beta1/denoms_metadata_by_query_string?denom=&lt;denom&gt; | none | denom | QueryDenomMetadataResponse {metadata} | cosmos.bank.v1beta1.Query/DenomMetadata | new | 7.26.1 | sdk/proto/cosmos/bank/v1beta1/query.proto (planned) | contracts/proto-api.md#rest-denom-bindings |
| GET /cosmos/bank/v1beta1/params | none | none | QueryParamsResponse {params} | cosmos.bank.v1beta1.Query/Params | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:42 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-params-j39 |
| GET /cosmos/bank/v1beta1/spendable_balances/{address} | address | pagination | QuerySpendableBalancesResponse {balances, pagination} | cosmos.bank.v1beta1.Query/SpendableBalances | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:27 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-spendablebalances-j39 |
| GET /cosmos/bank/v1beta1/supply | none | pagination | QueryTotalSupplyResponse {supply, pagination} | cosmos.bank.v1beta1.Query/TotalSupply | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:32 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-totalsupply-j39 |
| GET /cosmos/bank/v1beta1/supply/by_denom?denom=&lt;denom&gt; | none | denom | QuerySupplyOfResponse {amount} | cosmos.bank.v1beta1.Query/SupplyOf | new | 7.26.1 | sdk/proto/cosmos/bank/v1beta1/query.proto (planned) | contracts/proto-api.md#rest-denom-bindings |
| GET /cosmos/bank/v1beta1/supply/{denom} | denom | none | QuerySupplyOfResponse {amount} | cosmos.bank.v1beta1.Query/SupplyOf | stable | - | sdk/proto/cosmos/bank/v1beta1/query.proto:37 | contracts/proto-api.md#rest-cosmos-bank-v1beta1-supplyof-j39 |
| GET /cosmos/base/reflection/v1beta1/app_descriptor/authn | none | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:120 | contracts/proto-api.md#rest-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39 |
| GET /cosmos/base/reflection/v1beta1/app_descriptor/chain | none | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:124 | contracts/proto-api.md#rest-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39 |
| GET /cosmos/base/reflection/v1beta1/app_descriptor/codec | none | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:128 | contracts/proto-api.md#rest-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39 |
| GET /cosmos/base/reflection/v1beta1/app_descriptor/configuration | none | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:132 | contracts/proto-api.md#rest-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39 |
| GET /cosmos/base/reflection/v1beta1/app_descriptor/query_services | none | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:136 | contracts/proto-api.md#rest-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39 |
| GET /cosmos/base/reflection/v1beta1/app_descriptor/tx_descriptor | none | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:140 | contracts/proto-api.md#rest-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39 |
| GET /cosmos/base/reflection/v1beta1/interfaces | none | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:13 | contracts/proto-api.md#rest-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39 |
| GET /cosmos/base/reflection/v1beta1/interfaces/{interface_name}/implementations | interface_name | - | - | - | stable | - | sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:19 | contracts/proto-api.md#rest-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39 |
| GET /cosmos/base/tendermint/v1beta1/blocks/latest | none | none | GetLatestBlockResponse {block_id, block} | cosmos.base.tendermint.v1beta1.Service/GetLatestBlock | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:25 | contracts/proto-api.md#rest-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39 |
| GET /cosmos/base/tendermint/v1beta1/blocks/{height} | height | none | GetBlockByHeightResponse {block_id, block} | cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:29 | contracts/proto-api.md#rest-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39 |
| GET /cosmos/base/tendermint/v1beta1/node_info | none | none | GetNodeInfoResponse {default_node_info, application_version} | cosmos.base.tendermint.v1beta1.Service/GetNodeInfo | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:17 | contracts/proto-api.md#rest-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39 |
| GET /cosmos/base/tendermint/v1beta1/syncing | none | none | GetSyncingResponse {syncing} | cosmos.base.tendermint.v1beta1.Service/GetSyncing | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:21 | contracts/proto-api.md#rest-cosmos-base-tendermint-v1beta1-service-getsyncing-j39 |
| GET /cosmos/base/tendermint/v1beta1/validatorsets/latest | none | pagination | GetLatestValidatorSetResponse {block_height, validators, pagination} | cosmos.base.tendermint.v1beta1.Service/GetLatestValidatorSet | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:34 | contracts/proto-api.md#rest-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39 |
| GET /cosmos/base/tendermint/v1beta1/validatorsets/{height} | height | pagination | GetValidatorSetByHeightResponse {block_height, validators, pagination} | cosmos.base.tendermint.v1beta1.Service/GetValidatorSetByHeight | stable | - | sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:38 | contracts/proto-api.md#rest-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39 |
| GET /cosmos/distribution/v1beta1/community_pool | none | none | QueryCommunityPoolResponse {pool} | cosmos.distribution.v1beta1.Query/CommunityPool | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:67 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-communitypool-j39 |
| GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/rewards | delegator_address | none | QueryDelegationTotalRewardsResponse {rewards, total} | cosmos.distribution.v1beta1.Query/DelegationTotalRewards | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:48 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-delegationtotalrewards-j39 |
| GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/rewards/{validator_address} | delegator_address, validator_address | none | QueryDelegationRewardsResponse {rewards} | cosmos.distribution.v1beta1.Query/DelegationRewards | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:40 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-delegationrewards-j39 |
| GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/validators | delegator_address | none | QueryDelegatorValidatorsResponse {validators} | cosmos.distribution.v1beta1.Query/DelegatorValidators | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:53 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-delegatorvalidators-j39 |
| GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/withdraw_address | delegator_address | none | QueryDelegatorWithdrawAddressResponse {withdraw_address} | cosmos.distribution.v1beta1.Query/DelegatorWithdrawAddress | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:60 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39 |
| GET /cosmos/distribution/v1beta1/params | none | none | QueryParamsResponse {params} | cosmos.distribution.v1beta1.Query/Params | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:16 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-params-j39 |
| GET /cosmos/distribution/v1beta1/validators/{validator_address}/commission | validator_address | none | QueryValidatorCommissionResponse {commission} | cosmos.distribution.v1beta1.Query/ValidatorCommission | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:28 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-validatorcommission-j39 |
| GET /cosmos/distribution/v1beta1/validators/{validator_address}/outstanding_rewards | validator_address | none | QueryValidatorOutstandingRewardsResponse {rewards} | cosmos.distribution.v1beta1.Query/ValidatorOutstandingRewards | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:21 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39 |
| GET /cosmos/distribution/v1beta1/validators/{validator_address}/slashes | validator_address | starting_height, ending_height, pagination | QueryValidatorSlashesResponse {slashes, pagination} | cosmos.distribution.v1beta1.Query/ValidatorSlashes | stable | - | sdk/proto/cosmos/distribution/v1beta1/query.proto:35 | contracts/proto-api.md#rest-cosmos-distribution-v1beta1-validatorslashes-j39 |
| GET /cosmos/evidence/v1beta1/evidence | none | pagination | QueryAllEvidenceResponse {evidence, pagination} | cosmos.evidence.v1beta1.Query/AllEvidence | stable | - | sdk/proto/cosmos/evidence/v1beta1/query.proto:20 | contracts/proto-api.md#rest-cosmos-evidence-v1beta1-allevidence-j39 |
| GET /cosmos/evidence/v1beta1/evidence/{evidence_hash} | evidence_hash | none | QueryEvidenceResponse {evidence} | cosmos.evidence.v1beta1.Query/Evidence | stable | - | sdk/proto/cosmos/evidence/v1beta1/query.proto:15 | contracts/proto-api.md#rest-cosmos-evidence-v1beta1-evidence-j39 |
| GET /cosmos/feegrant/v1beta1/allowance/{granter}/{grantee} | granter, grantee | none | QueryAllowanceResponse {allowance} | cosmos.feegrant.v1beta1.Query/Allowance | stable | - | sdk/proto/cosmos/feegrant/v1beta1/query.proto:15 | contracts/proto-api.md#rest-cosmos-feegrant-v1beta1-allowance-j39 |
| GET /cosmos/feegrant/v1beta1/allowances/{grantee} | grantee | pagination | QueryAllowancesResponse {allowances, pagination} | cosmos.feegrant.v1beta1.Query/Allowances | stable | - | sdk/proto/cosmos/feegrant/v1beta1/query.proto:20 | contracts/proto-api.md#rest-cosmos-feegrant-v1beta1-allowances-j39 |
| GET /cosmos/feegrant/v1beta1/issued/{granter} | granter | pagination | QueryAllowancesByGranterResponse {allowances, pagination} | cosmos.feegrant.v1beta1.Query/AllowancesByGranter | stable | - | sdk/proto/cosmos/feegrant/v1beta1/query.proto:26 | contracts/proto-api.md#rest-cosmos-feegrant-v1beta1-allowancesbygranter-j39 |
| GET /cosmos/gov/v1beta1/params/{params_type} | params_type | none | QueryParamsResponse {voting_params, deposit_params, tally_params} | cosmos.gov.v1beta1.Query/Params | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:35 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-params-j39 |
| GET /cosmos/gov/v1beta1/proposals | none | proposal_status, voter, depositor, pagination | QueryProposalsResponse {proposals, pagination} | cosmos.gov.v1beta1.Query/Proposals | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:20 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-proposals-j39 |
| GET /cosmos/gov/v1beta1/proposals/{proposal_id} | proposal_id | none | QueryProposalResponse {proposal} | cosmos.gov.v1beta1.Query/Proposal | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:15 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-proposal-j39 |
| GET /cosmos/gov/v1beta1/proposals/{proposal_id}/deposits | proposal_id | pagination | QueryDepositsResponse {deposits, pagination} | cosmos.gov.v1beta1.Query/Deposits | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:45 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-deposits-j39 |
| GET /cosmos/gov/v1beta1/proposals/{proposal_id}/deposits/{depositor} | proposal_id, depositor | none | QueryDepositResponse {deposit} | cosmos.gov.v1beta1.Query/Deposit | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:40 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-deposit-j39 |
| GET /cosmos/gov/v1beta1/proposals/{proposal_id}/tally | proposal_id | none | QueryTallyResultResponse {tally} | cosmos.gov.v1beta1.Query/TallyResult | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:50 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-tallyresult-j39 |
| GET /cosmos/gov/v1beta1/proposals/{proposal_id}/votes | proposal_id | pagination | QueryVotesResponse {votes, pagination} | cosmos.gov.v1beta1.Query/Votes | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:30 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-votes-j39 |
| GET /cosmos/gov/v1beta1/proposals/{proposal_id}/votes/{voter} | proposal_id, voter | none | QueryVoteResponse {vote} | cosmos.gov.v1beta1.Query/Vote | stable | - | sdk/proto/cosmos/gov/v1beta1/query.proto:25 | contracts/proto-api.md#rest-cosmos-gov-v1beta1-vote-j39 |
| GET /cosmos/params/v1beta1/params | none | subspace, key | QueryParamsResponse {param} | cosmos.params.v1beta1.Query/Params | stable | - | sdk/proto/cosmos/params/v1beta1/query.proto:15 | contracts/proto-api.md#rest-cosmos-params-v1beta1-params-j39 |
| GET /cosmos/slashing/v1beta1/params | none | none | QueryParamsResponse {params} | cosmos.slashing.v1beta1.Query/Params | stable | - | sdk/proto/cosmos/slashing/v1beta1/query.proto:15 | contracts/proto-api.md#rest-cosmos-slashing-v1beta1-params-j39 |
| GET /cosmos/slashing/v1beta1/signing_infos | none | pagination | QuerySigningInfosResponse {info, pagination} | cosmos.slashing.v1beta1.Query/SigningInfos | stable | - | sdk/proto/cosmos/slashing/v1beta1/query.proto:25 | contracts/proto-api.md#rest-cosmos-slashing-v1beta1-signinginfos-j39 |
| GET /cosmos/slashing/v1beta1/signing_infos/{cons_address} | cons_address | none | QuerySigningInfoResponse {val_signing_info} | cosmos.slashing.v1beta1.Query/SigningInfo | stable | - | sdk/proto/cosmos/slashing/v1beta1/query.proto:20 | contracts/proto-api.md#rest-cosmos-slashing-v1beta1-signinginfo-j39 |
| GET /cosmos/staking/v1beta1/delegations/{delegator_addr} | delegator_addr | pagination | QueryDelegatorDelegationsResponse {delegation_responses, pagination} | cosmos.staking.v1beta1.Query/DelegatorDelegations | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:52 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-delegatordelegations-j39 |
| GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/redelegations | delegator_addr | src_validator_addr, dst_validator_addr, pagination | QueryRedelegationsResponse {redelegation_responses, pagination} | cosmos.staking.v1beta1.Query/Redelegations | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:65 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-redelegations-j39 |
| GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/unbonding_delegations | delegator_addr | pagination | QueryDelegatorUnbondingDelegationsResponse {unbonding_responses, pagination} | cosmos.staking.v1beta1.Query/DelegatorUnbondingDelegations | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:58 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39 |
| GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/validators | delegator_addr | pagination | QueryDelegatorValidatorsResponse {validators, pagination} | cosmos.staking.v1beta1.Query/DelegatorValidators | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:71 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-delegatorvalidators-j39 |
| GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/validators/{validator_addr} | delegator_addr, validator_addr | none | QueryDelegatorValidatorResponse {validator} | cosmos.staking.v1beta1.Query/DelegatorValidator | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:77 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-delegatorvalidator-j39 |
| GET /cosmos/staking/v1beta1/historical_info/{height} | height | none | QueryHistoricalInfoResponse {hist} | cosmos.staking.v1beta1.Query/HistoricalInfo | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:84 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-historicalinfo-j39 |
| GET /cosmos/staking/v1beta1/params | none | none | QueryParamsResponse {params} | cosmos.staking.v1beta1.Query/Params | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:94 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-params-j39 |
| GET /cosmos/staking/v1beta1/pool | none | none | QueryPoolResponse {pool} | cosmos.staking.v1beta1.Query/Pool | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:89 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-pool-j39 |
| GET /cosmos/staking/v1beta1/validators | none | status, pagination | QueryValidatorsResponse {validators, pagination} | cosmos.staking.v1beta1.Query/Validators | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:15 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-validators-j39 |
| GET /cosmos/staking/v1beta1/validators/{validator_addr} | validator_addr | none | QueryValidatorResponse {validator} | cosmos.staking.v1beta1.Query/Validator | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:20 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-validator-j39 |
| GET /cosmos/staking/v1beta1/validators/{validator_addr}/delegations | validator_addr | pagination | QueryValidatorDelegationsResponse {delegation_responses, pagination} | cosmos.staking.v1beta1.Query/ValidatorDelegations | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:25 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-validatordelegations-j39 |
| GET /cosmos/staking/v1beta1/validators/{validator_addr}/delegations/{delegator_addr} | validator_addr, delegator_addr | none | QueryDelegationResponse {delegation_response} | cosmos.staking.v1beta1.Query/Delegation | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:37 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-delegation-j39 |
| GET /cosmos/staking/v1beta1/validators/{validator_addr}/delegations/{delegator_addr}/unbonding_delegation | validator_addr, delegator_addr | none | QueryUnbondingDelegationResponse {unbond} | cosmos.staking.v1beta1.Query/UnbondingDelegation | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:45 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-unbondingdelegation-j39 |
| GET /cosmos/staking/v1beta1/validators/{validator_addr}/unbonding_delegations | validator_addr | pagination | QueryValidatorUnbondingDelegationsResponse {unbonding_responses, pagination} | cosmos.staking.v1beta1.Query/ValidatorUnbondingDelegations | stable | - | sdk/proto/cosmos/staking/v1beta1/query.proto:30 | contracts/proto-api.md#rest-cosmos-staking-v1beta1-validatorunbondingdelegations-j39 |
| GET /cosmos/tx/v1beta1/txs | none | events, pagination, order_by | GetTxsEventResponse {txs, tx_responses, pagination} | cosmos.tx.v1beta1.Service/GetTxsEvent | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:37 | contracts/proto-api.md#rest-cosmos-tx-v1beta1-service-gettxsevent-j39 |
| GET /cosmos/tx/v1beta1/txs/block/{height} | height | pagination | GetBlockWithTxsResponse {txs, block_id, block, pagination} | cosmos.tx.v1beta1.Service/GetBlockWithTxs | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:43 | contracts/proto-api.md#rest-cosmos-tx-v1beta1-service-getblockwithtxs-j39 |
| GET /cosmos/tx/v1beta1/txs/{hash} | hash | none | GetTxResponse {tx, tx_response} | cosmos.tx.v1beta1.Service/GetTx | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:26 | contracts/proto-api.md#rest-cosmos-tx-v1beta1-service-gettx-j39 |
| GET /cosmos/upgrade/v1beta1/applied_plan/{name} | name | none | QueryAppliedPlanResponse {height} | cosmos.upgrade.v1beta1.Query/AppliedPlan | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:18 | contracts/proto-api.md#rest-cosmos-upgrade-v1beta1-appliedplan-j39 |
| GET /cosmos/upgrade/v1beta1/current_plan | none | none | QueryCurrentPlanResponse {plan} | cosmos.upgrade.v1beta1.Query/CurrentPlan | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:13 | contracts/proto-api.md#rest-cosmos-upgrade-v1beta1-currentplan-j39 |
| GET /cosmos/upgrade/v1beta1/module_versions | none | module_name | QueryModuleVersionsResponse {module_versions} | cosmos.upgrade.v1beta1.Query/ModuleVersions | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:36 | contracts/proto-api.md#rest-cosmos-upgrade-v1beta1-moduleversions-j39 |
| GET /cosmos/upgrade/v1beta1/upgraded_consensus_state/{last_height} | last_height | none | QueryUpgradedConsensusStateResponse {upgraded_consensus_state} | cosmos.upgrade.v1beta1.Query/UpgradedConsensusState | stable | - | sdk/proto/cosmos/upgrade/v1beta1/query.proto:29 | contracts/proto-api.md#rest-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39 |
| POST /cosmos/tx/v1beta1/simulate body * | none | tx, tx_bytes | SimulateResponse {gas_info, result} | cosmos.tx.v1beta1.Service/Simulate | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:19 | contracts/proto-api.md#rest-cosmos-tx-v1beta1-service-simulate-j39 |
| POST /cosmos/tx/v1beta1/txs body * | none | tx_bytes, mode | BroadcastTxResponse {tx_response} | cosmos.tx.v1beta1.Service/BroadcastTx | stable | - | sdk/proto/cosmos/tx/v1beta1/service.proto:30 | contracts/proto-api.md#rest-cosmos-tx-v1beta1-service-broadcasttx-j39 |

<a id="anchored-here"></a>

## Entries frozen in this file

These blocks name `contracts/rpc.md#<anchor>` as their contract; each row carries that anchor.

### Module queries

| Anchor | Full method | Request fields | Response | Status | Tasks | Defined at |
|---|---|---|---|---|---|---|
| <a id="qry-epoch-epoch"></a>qry-epoch-epoch | paxprotocol.paxchain.epoch.Query/Epoch | QueryEpochRequest | QueryEpochResponse {epoch} | stable | - | api/epoch/query.proto:15 |
| <a id="qry-epoch-params"></a>qry-epoch-params | paxprotocol.paxchain.epoch.Query/Params | QueryParamsRequest | QueryParamsResponse {params} | stable | - | api/epoch/query.proto:19 |
| <a id="qry-evm-evmaddressbypaxaddress"></a>qry-evm-evmaddressbypaxaddress | paxprotocol.paxchain.evm.Query/EVMAddressByPaxAddress | QueryEVMAddressByPaxAddressRequest {pax_address} | QueryEVMAddressByPaxAddressResponse {evm_address, associated} | stable | - | api/evm/query.proto:15 |
| <a id="qry-evm-paxaddressbyevmaddress"></a>qry-evm-paxaddressbyevmaddress | paxprotocol.paxchain.evm.Query/PaxAddressByEVMAddress | QueryPaxAddressByEVMAddressRequest {evm_address} | QueryPaxAddressByEVMAddressResponse {pax_address, associated} | stable | - | api/evm/query.proto:11 |
| <a id="qry-evm-pointee"></a>qry-evm-pointee | paxprotocol.paxchain.evm.Query/Pointee | QueryPointeeRequest {pointer_type, pointer} | QueryPointeeResponse {pointee, version, exists} | stable | - | api/evm/query.proto:31 |
| <a id="qry-evm-pointer"></a>qry-evm-pointer | paxprotocol.paxchain.evm.Query/Pointer | QueryPointerRequest {pointer_type, pointee} | QueryPointerResponse {pointer, version, exists} | stable | - | api/evm/query.proto:23 |
| <a id="qry-evm-pointerversion"></a>qry-evm-pointerversion | paxprotocol.paxchain.evm.Query/PointerVersion | QueryPointerVersionRequest {pointer_type} | QueryPointerVersionResponse {version, cw_code_id} | stable | - | api/evm/query.proto:27 |
| <a id="qry-evm-staticcall"></a>qry-evm-staticcall | paxprotocol.paxchain.evm.Query/StaticCall | QueryStaticCallRequest {data, to} | QueryStaticCallResponse {data} | stable | - | api/evm/query.proto:19 |
| <a id="qry-layerxprograms-programbalances"></a>qry-layerxprograms-programbalances | paxprotocol.paxchain.layerxprograms.Query/ProgramBalances | QueryProgramBalancesRequest {program_id, height, prove, pagination} | QueryProgramBalancesResponse {program_id, balances, height, app_hash, pagination, state_record, state_recor... | stable | - | api/layerxprograms/query.proto:12 |
| <a id="qry-layerxprograms-programevents"></a>qry-layerxprograms-programevents | paxprotocol.paxchain.layerxprograms.Query/ProgramEvents | QueryProgramEventsRequest {program_id, from_height, to_height, pagination, after_sequence} | QueryProgramEventsResponse {events, pagination, complete_through, scanned_through} | stable | - | api/layerxprograms/query.proto:13 |
| <a id="qry-layerxprograms-programstate"></a>qry-layerxprograms-programstate | paxprotocol.paxchain.layerxprograms.Query/ProgramState | QueryProgramStateRequest {program_id, key, height, prove} | QueryProgramStateResponse {program_id, key, value, state_record, height, app_hash, proof_ops, state_record_... | stable | - | api/layerxprograms/query.proto:11 |
| <a id="qry-mint-minter"></a>qry-mint-minter | paxprotocol.paxchain.mint.Query/Minter | QueryMinterRequest | QueryMinterResponse {start_date, end_date, denom, total_mint_amount, remaining_mint_amount, last_mint_amoun... | stable | - | api/mint/v1beta1/query.proto:18 |
| <a id="qry-mint-params"></a>qry-mint-params | paxprotocol.paxchain.mint.Query/Params | QueryParamsRequest | QueryParamsResponse {params} | stable | - | api/mint/v1beta1/query.proto:13 |
| <a id="qry-oracle-actives"></a>qry-oracle-actives | paxprotocol.paxchain.oracle.Query/Actives | QueryActivesRequest | QueryActivesResponse {actives} | stable | - | api/oracle/query.proto:23 |
| <a id="qry-oracle-exchangerate"></a>qry-oracle-exchangerate | paxprotocol.paxchain.oracle.Query/ExchangeRate | QueryExchangeRateRequest {denom} | QueryExchangeRateResponse {oracle_exchange_rate} | stable | - | api/oracle/query.proto:13 |
| <a id="qry-oracle-exchangerates"></a>qry-oracle-exchangerates | paxprotocol.paxchain.oracle.Query/ExchangeRates | QueryExchangeRatesRequest | QueryExchangeRatesResponse {denom_oracle_exchange_rate_pairs} | stable | - | api/oracle/query.proto:18 |
| <a id="qry-oracle-feederdelegation"></a>qry-oracle-feederdelegation | paxprotocol.paxchain.oracle.Query/FeederDelegation | QueryFeederDelegationRequest {validator_addr} | QueryFeederDelegationResponse {feeder_addr} | stable | - | api/oracle/query.proto:42 |
| <a id="qry-oracle-params"></a>qry-oracle-params | paxprotocol.paxchain.oracle.Query/Params | QueryParamsRequest | QueryParamsResponse {params} | stable | - | api/oracle/query.proto:57 |
| <a id="qry-oracle-pricesnapshothistory"></a>qry-oracle-pricesnapshothistory | paxprotocol.paxchain.oracle.Query/PriceSnapshotHistory | QueryPriceSnapshotHistoryRequest | QueryPriceSnapshotHistoryResponse {price_snapshots} | stable | - | api/oracle/query.proto:33 |
| <a id="qry-oracle-slashwindow"></a>qry-oracle-slashwindow | paxprotocol.paxchain.oracle.Query/SlashWindow | QuerySlashWindowRequest | QuerySlashWindowResponse {window_progress} | stable | - | api/oracle/query.proto:52 |
| <a id="qry-oracle-twaps"></a>qry-oracle-twaps | paxprotocol.paxchain.oracle.Query/Twaps | QueryTwapsRequest {lookback_seconds} | QueryTwapsResponse {oracle_twaps} | stable | - | api/oracle/query.proto:37 |
| <a id="qry-oracle-votepenaltycounter"></a>qry-oracle-votepenaltycounter | paxprotocol.paxchain.oracle.Query/VotePenaltyCounter | QueryVotePenaltyCounterRequest {validator_addr} | QueryVotePenaltyCounterResponse {vote_penalty_counter} | stable | - | api/oracle/query.proto:47 |
| <a id="qry-oracle-votetargets"></a>qry-oracle-votetargets | paxprotocol.paxchain.oracle.Query/VoteTargets | QueryVoteTargetsRequest | QueryVoteTargetsResponse {vote_targets} | stable | - | api/oracle/query.proto:28 |
| <a id="qry-tokenfactory-denomallowlist"></a>qry-tokenfactory-denomallowlist | paxprotocol.paxchain.tokenfactory.Query/DenomAllowList | QueryDenomAllowListRequest {denom} | QueryDenomAllowListResponse {allow_list} | stable | - | api/tokenfactory/query.proto:40 |
| <a id="qry-tokenfactory-denomauthoritymetadata"></a>qry-tokenfactory-denomauthoritymetadata | paxprotocol.paxchain.tokenfactory.Query/DenomAuthorityMetadata | QueryDenomAuthorityMetadataRequest {denom} | QueryDenomAuthorityMetadataResponse {authority_metadata} | stable | - | api/tokenfactory/query.proto:23 |
| <a id="qry-tokenfactory-denommetadata"></a>qry-tokenfactory-denommetadata | paxprotocol.paxchain.tokenfactory.Query/DenomMetadata | QueryDenomMetadataRequest {denom} | QueryDenomMetadataResponse {metadata} | stable | - | api/tokenfactory/query.proto:29 |
| <a id="qry-tokenfactory-denomsfromcreator"></a>qry-tokenfactory-denomsfromcreator | paxprotocol.paxchain.tokenfactory.Query/DenomsFromCreator | QueryDenomsFromCreatorRequest {creator, pagination} | QueryDenomsFromCreatorResponse {denoms, pagination} | stable | - | api/tokenfactory/query.proto:35 |
| <a id="qry-tokenfactory-params"></a>qry-tokenfactory-params | paxprotocol.paxchain.tokenfactory.Query/Params | QueryParamsRequest | QueryParamsResponse {params} | stable | - | api/tokenfactory/query.proto:17 |

### Module REST routes

| Route | Path params | Query params | Returns | gRPC method | Status | Tasks | Defined at | Contract |
|---|---|---|---|---|---|---|---|---|
| <a id="rest-oracle-actives"></a>GET /pax-protocol/pax-chain/oracle/denoms/actives | none | none | QueryActivesResponse {actives} | paxprotocol.paxchain.oracle.Query/Actives | stable | - | api/oracle/query.proto:24 | contracts/rpc.md#rest-oracle-actives |
| <a id="rest-oracle-exchangerates"></a>GET /pax-protocol/pax-chain/oracle/denoms/exchange_rates | none | none | QueryExchangeRatesResponse {denom_oracle_exchange_rate_pairs} | paxprotocol.paxchain.oracle.Query/ExchangeRates | stable | - | api/oracle/query.proto:19 | contracts/rpc.md#rest-oracle-exchangerates |
| <a id="rest-oracle-pricesnapshothistory"></a>GET /pax-protocol/pax-chain/oracle/denoms/price_snapshot_history | none | none | QueryPriceSnapshotHistoryResponse {price_snapshots} | paxprotocol.paxchain.oracle.Query/PriceSnapshotHistory | stable | - | api/oracle/query.proto:34 | contracts/rpc.md#rest-oracle-pricesnapshothistory |
| <a id="rest-oracle-twaps"></a>GET /pax-protocol/pax-chain/oracle/denoms/twaps/{lookback_seconds} | lookback_seconds | none | QueryTwapsResponse {oracle_twaps} | paxprotocol.paxchain.oracle.Query/Twaps | stable | - | api/oracle/query.proto:38 | contracts/rpc.md#rest-oracle-twaps |
| <a id="rest-oracle-votetargets"></a>GET /pax-protocol/pax-chain/oracle/denoms/vote_targets | none | none | QueryVoteTargetsResponse {vote_targets} | paxprotocol.paxchain.oracle.Query/VoteTargets | stable | - | api/oracle/query.proto:29 | contracts/rpc.md#rest-oracle-votetargets |
| <a id="rest-oracle-exchangerate"></a>GET /pax-protocol/pax-chain/oracle/denoms/{denom}/exchange_rate | denom | none | QueryExchangeRateResponse {oracle_exchange_rate} | paxprotocol.paxchain.oracle.Query/ExchangeRate | stable | - | api/oracle/query.proto:14 | contracts/rpc.md#rest-oracle-exchangerate |
| <a id="rest-oracle-params"></a>GET /pax-protocol/pax-chain/oracle/params | none | none | QueryParamsResponse {params} | paxprotocol.paxchain.oracle.Query/Params | stable | - | api/oracle/query.proto:58 | contracts/rpc.md#rest-oracle-params |
| <a id="rest-oracle-slashwindow"></a>GET /pax-protocol/pax-chain/oracle/slash_window | none | none | QuerySlashWindowResponse {window_progress} | paxprotocol.paxchain.oracle.Query/SlashWindow | stable | - | api/oracle/query.proto:53 | contracts/rpc.md#rest-oracle-slashwindow |
| <a id="rest-oracle-feederdelegation"></a>GET /pax-protocol/pax-chain/oracle/validators/{validator_addr}/feeder | validator_addr | none | QueryFeederDelegationResponse {feeder_addr} | paxprotocol.paxchain.oracle.Query/FeederDelegation | stable | - | api/oracle/query.proto:43 | contracts/rpc.md#rest-oracle-feederdelegation |
| <a id="rest-oracle-votepenaltycounter"></a>GET /pax-protocol/pax-chain/oracle/validators/{validator_addr}/vote_penalty_counter | validator_addr | none | QueryVotePenaltyCounterResponse {vote_penalty_counter} | paxprotocol.paxchain.oracle.Query/VotePenaltyCounter | stable | - | api/oracle/query.proto:48 | contracts/rpc.md#rest-oracle-votepenaltycounter |
| <a id="rest-epoch-epoch"></a>GET /pax-protocol/paxchain/epoch/epoch | none | none | QueryEpochResponse {epoch} | paxprotocol.paxchain.epoch.Query/Epoch | stable | - | api/epoch/query.proto:16 | contracts/rpc.md#rest-epoch-epoch |
| <a id="rest-epoch-params"></a>GET /pax-protocol/paxchain/epoch/params | none | none | QueryParamsResponse {params} | paxprotocol.paxchain.epoch.Query/Params | stable | - | api/epoch/query.proto:20 | contracts/rpc.md#rest-epoch-params |
| <a id="rest-evm-evmaddressbypaxaddress"></a>GET /pax-protocol/paxchain/evm/evm_address | none | pax_address | QueryEVMAddressByPaxAddressResponse {evm_address, associated} | paxprotocol.paxchain.evm.Query/EVMAddressByPaxAddress | stable | - | api/evm/query.proto:16 | contracts/rpc.md#rest-evm-evmaddressbypaxaddress |
| <a id="rest-evm-paxaddressbyevmaddress"></a>GET /pax-protocol/paxchain/evm/pax_address | none | evm_address | QueryPaxAddressByEVMAddressResponse {pax_address, associated} | paxprotocol.paxchain.evm.Query/PaxAddressByEVMAddress | stable | - | api/evm/query.proto:12 | contracts/rpc.md#rest-evm-paxaddressbyevmaddress |
| <a id="rest-evm-pointee"></a>GET /pax-protocol/paxchain/evm/pointee | none | pointer_type, pointer | QueryPointeeResponse {pointee, version, exists} | paxprotocol.paxchain.evm.Query/Pointee | stable | - | api/evm/query.proto:32 | contracts/rpc.md#rest-evm-pointee |
| <a id="rest-evm-pointer"></a>GET /pax-protocol/paxchain/evm/pointer | none | pointer_type, pointee | QueryPointerResponse {pointer, version, exists} | paxprotocol.paxchain.evm.Query/Pointer | stable | - | api/evm/query.proto:24 | contracts/rpc.md#rest-evm-pointer |
| <a id="rest-evm-pointerversion"></a>GET /pax-protocol/paxchain/evm/pointer_version | none | pointer_type | QueryPointerVersionResponse {version, cw_code_id} | paxprotocol.paxchain.evm.Query/PointerVersion | stable | - | api/evm/query.proto:28 | contracts/rpc.md#rest-evm-pointerversion |
| <a id="rest-evm-staticcall"></a>GET /pax-protocol/paxchain/evm/static_call | none | data, to | QueryStaticCallResponse {data} | paxprotocol.paxchain.evm.Query/StaticCall | stable | - | api/evm/query.proto:20 | contracts/rpc.md#rest-evm-staticcall |
| <a id="rest-tokenfactory-denomallowlist"></a>GET /pax-protocol/paxchain/tokenfactory/denoms/allow_list | none | denom | QueryDenomAllowListResponse {allow_list} | paxprotocol.paxchain.tokenfactory.Query/DenomAllowList | stable | - | api/tokenfactory/query.proto:41 | contracts/rpc.md#rest-tokenfactory-denomallowlist |
| <a id="rest-tokenfactory-denommetadata"></a>GET /pax-protocol/paxchain/tokenfactory/denoms/metadata | none | denom | QueryDenomMetadataResponse {metadata} | paxprotocol.paxchain.tokenfactory.Query/DenomMetadata | stable | - | api/tokenfactory/query.proto:30 | contracts/rpc.md#rest-tokenfactory-denommetadata |
| <a id="rest-tokenfactory-denomauthoritymetadata"></a>GET /pax-protocol/paxchain/tokenfactory/denoms/{denom}/authority_metadata | denom | none | QueryDenomAuthorityMetadataResponse {authority_metadata} | paxprotocol.paxchain.tokenfactory.Query/DenomAuthorityMetadata | stable | - | api/tokenfactory/query.proto:24 | contracts/rpc.md#rest-tokenfactory-denomauthoritymetadata |
| <a id="rest-tokenfactory-denomsfromcreator"></a>GET /pax-protocol/paxchain/tokenfactory/denoms_from_creator/{creator} | creator | pagination | QueryDenomsFromCreatorResponse {denoms, pagination} | paxprotocol.paxchain.tokenfactory.Query/DenomsFromCreator | stable | - | api/tokenfactory/query.proto:36 | contracts/rpc.md#rest-tokenfactory-denomsfromcreator |
| <a id="rest-tokenfactory-params"></a>GET /pax-protocol/paxchain/tokenfactory/params | none | none | QueryParamsResponse {params} | paxprotocol.paxchain.tokenfactory.Query/Params | stable | - | api/tokenfactory/query.proto:18 | contracts/rpc.md#rest-tokenfactory-params |
| <a id="rest-mint-minter"></a>GET /paxchain/mint/v1beta1/minter | none | none | QueryMinterResponse {start_date, end_date, denom, total_mint_amount, remaining_mint_amount, last_mint_amoun... | paxprotocol.paxchain.mint.Query/Minter | stable | - | api/mint/v1beta1/query.proto:19 | contracts/rpc.md#rest-mint-minter |
| <a id="rest-mint-params"></a>GET /paxchain/mint/v1beta1/params | none | none | QueryParamsResponse {params} | paxprotocol.paxchain.mint.Query/Params | stable | - | api/mint/v1beta1/query.proto:14 | contracts/rpc.md#rest-mint-params |
| <a id="rest-layerxgov-gov-proposal"></a>POST /gov/proposals/layerx (legacy gov REST sub-route ProposalRESTSubRoute = "layerx") | none | - | - | - | stable | - | modules/layerxgov/client/cli/tx.go:187 | contracts/rpc.md#rest-layerxgov-gov-proposal |
| <a id="rest-layerxbridge-gov-proposal"></a>POST /gov/proposals/layerxbridge (legacy gov REST sub-route ProposalRESTSubRoute = "layerxbridge") | none | - | - | - | stable | - | modules/layerxbridge/client/cli/tx.go:226 | contracts/rpc.md#rest-layerxbridge-gov-proposal |

### Admin service, proto messages and build formats

| Anchor | Kind | Signature | Status | Tasks | Defined at | Notes |
|---|---|---|---|---|---|---|
| <a id="fmt-api-buf-gen-yaml"></a>fmt-api-buf-gen-yaml | file_format | buf v1 generate template: plugin gocosmos out .. opt plugins=grpc plus gogo well-known-type mappings; plugin grpc-gat... | retire | 7.22.3 | api/buf.gen.yaml:1 | no script references this template: scripts/protoc.sh:26 runs buf generate with the root v2 buf.gen.yaml, whose plugins write under build... |
| <a id="fmt-api-buf-yaml"></a>fmt-api-buf-yaml | file_format | buf v1 module config: name buf.build/pax-protocol/pax-chain; deps buf.build/pax-protocol/cosmos-sdk, buf.build/pax-pr... | retire | 7.22.3 | api/buf.yaml:1 | the root v2 buf.yaml lists api as a module path (buf.yaml:4); sdk/js generates from the module name this file declares (sdk/js/package.js... |
| <a id="grpcs-pax-admin-getloglevel"></a>grpcs-pax-admin-getloglevel | grpc_service_method | paxprotocol.paxchain.admin.v0.AdminService/GetLogLevel: rpc GetLogLevel(GetLogLevelRequest) returns (GetLogLevelRespo... | stable | - | api/pax/admin/v0/admin.proto:13 | handler admin/service.go:47 |
| <a id="grpcs-pax-admin-listloggers"></a>grpcs-pax-admin-listloggers | grpc_service_method | paxprotocol.paxchain.admin.v0.AdminService/ListLoggers: rpc ListLoggers(ListLoggersRequest) returns (ListLoggersRespo... | stable | - | api/pax/admin/v0/admin.proto:15 | handler admin/service.go:63 |
| <a id="grpcs-pax-admin-setloglevel"></a>grpcs-pax-admin-setloglevel | grpc_service_method | paxprotocol.paxchain.admin.v0.AdminService/SetLogLevel: rpc SetLogLevel(SetLogLevelRequest) returns (SetLogLevelRespo... | stable | - | api/pax/admin/v0/admin.proto:11 | handler admin/service.go:19 |
| <a id="proto-eth-accesslisttx"></a>proto-eth-accesslisttx | proto_message | message paxprotocol.paxchain.eth.AccessListTx { string chain_id = 1; uint64 nonce = 2; string gas_price = 3; uint64 g... | stable | - | api/eth/tx.proto:60 | Go type AccessListTx at modules/evm/types/ethtx/tx.pb.go:195 |
| <a id="proto-eth-accesstuple"></a>proto-eth-accesstuple | proto_message | message paxprotocol.paxchain.eth.AccessTuple { string address = 1; repeated string storage_keys = 2; } | stable | - | api/eth/tx.proto:10 | Go type AccessTuple at modules/evm/types/ethtx/tx.pb.go:27 |
| <a id="proto-eth-associatetx"></a>proto-eth-associatetx | proto_message | message paxprotocol.paxchain.eth.AssociateTx { bytes v = 1; bytes r = 2; bytes s = 3; string custom_message = 4; } | stable | - | api/eth/tx.proto:17 | Go type AssociateTx at modules/evm/types/ethtx/tx.pb.go:65 |
| <a id="proto-eth-blobtx"></a>proto-eth-blobtx | proto_message | message paxprotocol.paxchain.eth.BlobTx { string chain_id = 1; uint64 nonce = 2; string gas_tip_cap = 3; string gas_f... | stable | - | api/eth/tx.proto:151 | Go type BlobTx at modules/evm/types/ethtx/tx.pb.go:342 |
| <a id="proto-eth-blobtxsidecar"></a>proto-eth-blobtxsidecar | proto_message | message paxprotocol.paxchain.eth.BlobTxSidecar { repeated bytes blobs = 1; repeated bytes commitments = 2; repeated b... | stable | - | api/eth/tx.proto:183 | Go type BlobTxSidecar at modules/evm/types/ethtx/tx.pb.go:394 |
| <a id="proto-eth-dynamicfeetx"></a>proto-eth-dynamicfeetx | proto_message | message paxprotocol.paxchain.eth.DynamicFeeTx { string chain_id = 1; uint64 nonce = 2; string gas_tip_cap = 3; string... | stable | - | api/eth/tx.proto:122 | Go type DynamicFeeTx at modules/evm/types/ethtx/tx.pb.go:293 |
| <a id="proto-eth-extensionoptionsethereumtx"></a>proto-eth-extensionoptionsethereumtx | proto_message | message paxprotocol.paxchain.eth.ExtensionOptionsEthereumTx { } | stable | - | api/eth/tx.proto:189 | Go type ExtensionOptionsEthereumTx at modules/evm/types/ethtx/tx.pb.go:454; no in-repo reference outside the generated package |
| <a id="proto-eth-legacytx"></a>proto-eth-legacytx | proto_message | message paxprotocol.paxchain.eth.LegacyTx { uint64 nonce = 1; string gas_price = 2; uint64 gas_limit = 3; string to =... | stable | - | api/eth/tx.proto:42 | Go type LegacyTx at modules/evm/types/ethtx/tx.pb.go:149 |
| <a id="proto-eth-setcodeauthorization"></a>proto-eth-setcodeauthorization | proto_message | message paxprotocol.paxchain.eth.SetCodeAuthorization { string chain_id = 1; string address = 2; uint64 nonce = 3; by... | stable | - | api/eth/tx.proto:26 | Go type SetCodeAuthorization at modules/evm/types/ethtx/tx.pb.go:106 |
| <a id="proto-eth-setcodetx"></a>proto-eth-setcodetx | proto_message | message paxprotocol.paxchain.eth.SetCodeTx { string chain_id = 1; uint64 nonce = 2; string gas_tip_cap = 3; string ga... | stable | - | api/eth/tx.proto:88 | Go type SetCodeTx at modules/evm/types/ethtx/tx.pb.go:243 |
| <a id="proto-pax-admin-getloglevelrequest"></a>proto-pax-admin-getloglevelrequest | proto_message | message paxprotocol.paxchain.admin.v0.GetLogLevelRequest { string logger = 1; } | stable | - | api/pax/admin/v0/admin.proto:31 | Go type GetLogLevelRequest at admin/types/admin.pb.go:145 |
| <a id="proto-pax-admin-getloglevelresponse"></a>proto-pax-admin-getloglevelresponse | proto_message | message paxprotocol.paxchain.admin.v0.GetLogLevelResponse { string logger = 1; string level = 2; } | stable | - | api/pax/admin/v0/admin.proto:36 | Go type GetLogLevelResponse at admin/types/admin.pb.go:190 |
| <a id="proto-pax-admin-listloggersrequest"></a>proto-pax-admin-listloggersrequest | proto_message | message paxprotocol.paxchain.admin.v0.ListLoggersRequest { string prefix = 1; } | stable | - | api/pax/admin/v0/admin.proto:41 | Go type ListLoggersRequest at admin/types/admin.pb.go:242 |
| <a id="proto-pax-admin-listloggersresponse"></a>proto-pax-admin-listloggersresponse | proto_message | message paxprotocol.paxchain.admin.v0.ListLoggersResponse { repeated LoggerInfo loggers = 1; } | stable | - | api/pax/admin/v0/admin.proto:46 | Go type ListLoggersResponse at admin/types/admin.pb.go:287 |
| <a id="proto-pax-admin-loggerinfo"></a>proto-pax-admin-loggerinfo | proto_message | message paxprotocol.paxchain.admin.v0.LoggerInfo { string name = 1; string level = 2; } | stable | - | api/pax/admin/v0/admin.proto:50 | Go type LoggerInfo at admin/types/admin.pb.go:331 |
| <a id="proto-pax-admin-setloglevelrequest"></a>proto-pax-admin-setloglevelrequest | proto_message | message paxprotocol.paxchain.admin.v0.SetLogLevelRequest { string pattern = 1; string level = 2; } | stable | - | api/pax/admin/v0/admin.proto:18 | Go type SetLogLevelRequest at admin/types/admin.pb.go:31 |
| <a id="proto-pax-admin-setloglevelresponse"></a>proto-pax-admin-setloglevelresponse | proto_message | message paxprotocol.paxchain.admin.v0.SetLogLevelResponse { string pattern = 1; string level = 2; int32 affected = 3; } | stable | - | api/pax/admin/v0/admin.proto:25 | Go type SetLogLevelResponse at admin/types/admin.pb.go:85 |

<a id="other-domains"></a>

## RPC surfaces of other domains

Every other domain inventories its own query, REST, JSON-RPC, HTTP and WebSocket surfaces under its own contract files. Counts by domain, kind and contract file, with the non-stable count:

| Domain | Kind | Contract file | Interfaces | Non-stable |
|---|---|---|---|---|
| 2 | grpc_query | contracts/agent-functions.md | 3 | 3 |
| 2 | grpc_query | contracts/pay.md | 1 | 1 |
| 3 | grpc_query | contracts/markets.md | 6 | 6 |
| 3 | grpc_query | contracts/rpc.md | 14 | 0 |
| 3 | http_route | contracts/perps-client.md | 19 | 0 |
| 3 | http_route | contracts/perps-data.md | 121 | 19 |
| 3 | rest_route | contracts/markets.md | 8 | 8 |
| 3 | rest_route | contracts/rpc.md | 14 | 0 |
| 3 | ws_route | contracts/perps-data.md | 15 | 5 |
| 4 | grpc_query | contracts/launchpad.md | 17 | 17 |
| 4 | http_route | contracts/clients.md | 6 | 0 |
| 4 | http_route | contracts/services.md | 743 | 92 |
| 4 | rest_route | contracts/launchpad.md | 17 | 17 |
| 4 | ws_route | contracts/services.md | 10 | 0 |
| 5 | grpc_service_method | contracts/sequencer.md | 4 | 0 |
| 6 | grpc_query | contracts/cosmwasm.md | 9 | 0 |
| 6 | grpc_query | contracts/ibc.md | 35 | 0 |
| 6 | grpc_query | contracts/rpc.md | 11 | 0 |
| 6 | grpc_query | contracts/tokenfactory.md | 1 | 1 |
| 6 | grpc_query | contracts/web.md | 7 | 7 |
| 6 | grpc_service_method | contracts/web.md | 1 | 1 |
| 6 | rest_route | contracts/cosmwasm.md | 22 | 0 |
| 6 | rest_route | contracts/ibc.md | 35 | 0 |
| 6 | rest_route | contracts/proto-api.md | 2 | 2 |
| 6 | rest_route | contracts/rpc.md | 13 | 0 |
| 6 | rest_route | contracts/tokenfactory.md | 1 | 1 |
| 6 | rest_route | contracts/web.md | 7 | 7 |
| 8 | http_route | contracts/platform-cli.md | 30 | 0 |
| 8 | http_route | contracts/platform-gateway.md | 202 | 16 |
| 8 | http_route | contracts/platform-hosted.md | 139 | 24 |
| 8 | http_route | contracts/platform-middleware.md | 11 | 4 |
| 8 | http_route | contracts/platform-ramps.md | 11 | 3 |
| 8 | http_route | contracts/platform-relay-archive.md | 14 | 14 |
| 8 | http_route | contracts/platform.md | 2 | 2 |
| 8 | jsonrpc_method | contracts/platform-cli.md | 6 | 0 |
| 8 | jsonrpc_method | contracts/platform-gateway.md | 51 | 26 |
| 8 | jsonrpc_method | contracts/platform-hosted.md | 3 | 1 |
| 8 | jsonrpc_method | contracts/platform-relay-archive.md | 7 | 7 |
| 8 | ws_route | contracts/platform-gateway.md | 2 | 0 |
| 9 | http_route | contracts/agent-api.md | 64 | 56 |
| 9 | http_route | contracts/agent-mcp.md | 36 | 35 |
| 9 | jsonrpc_method | contracts/agent-mcp.md | 4 | 0 |
| 10 | http_route | contracts/services.md | 106 | 34 |
| 11 | http_route | contracts/interop-deploy.md | 9 | 1 |
| 11 | http_route | contracts/interop-gas-station.md | 5 | 3 |
| 11 | http_route | contracts/interop-gateway.md | 21 | 9 |
| 11 | http_route | contracts/interop-intent-ingester.md | 1 | 1 |
| 11 | http_route | contracts/interop-service.md | 6 | 3 |
| 11 | http_route | contracts/interop-websearch.md | 7 | 4 |
| 11 | http_route | contracts/interop.md | 2 | 2 |
| 12 | http_route | contracts/clients.md | 21 | 5 |
| 12 | http_route | contracts/services.md | 129 | 47 |
| 12 | jsonrpc_method | contracts/clients.md | 55 | 20 |
| 13 | grpc_query | contracts/rpc.md | 3 | 0 |
| 14 | grpc_service_method | contracts/sig-provider-http.md | 5 | 0 |
| 14 | grpc_service_method | contracts/verifier-http.md | 14 | 0 |
| 14 | http_route | contracts/bridge-client.md | 1 | 0 |
| 14 | http_route | contracts/explorer-frontend.md | 97 | 0 |
| 14 | http_route | contracts/explorer-http-api.md | 464 | 9 |
| 14 | http_route | contracts/explorer-metrics.md | 3 | 0 |
| 14 | http_route | contracts/sig-provider-http.md | 5 | 0 |
| 14 | http_route | contracts/verifier-http.md | 14 | 0 |
| 14 | http_route | contracts/web-client.md | 38 | 8 |
| 14 | jsonrpc_method | contracts/explorer-eth-rpc.md | 16 | 0 |
| 14 | ws_route | contracts/explorer-ws.md | 108 | 0 |
| 15 | grpc_service_method | contracts/consensus-fork.md | 3 | 0 |
| 15 | jsonrpc_method | contracts/consensus-fork.md | 35 | 13 |
| 16 | http_route | contracts/operations.md | 21 | 0 |

