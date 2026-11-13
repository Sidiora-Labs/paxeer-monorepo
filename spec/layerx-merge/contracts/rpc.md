# RPC contract: node transports and method handlers

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). Node JSON-RPC uses the configured root path. A transaction receipt is final when its block commits. Hosted gateway `px_` methods are described by [openrpc.json](../../../services/platform/hosted/gateway/openrpc.json); method availability follows the selected endpoint and registration.

## Endpoints

| Transport | Configuration | Default port | Implementation |
|---|---|---|---|
| EVM HTTP | `evm.http_address`, `evm.http_port`, `evm.http_enabled` | 8545 | [RPC config](../../../rpc/config/config.go) |
| EVM WebSocket | `evm.ws_port`, `evm.ws_enabled` | 8546 | [RPC config](../../../rpc/config/config.go) |
| gRPC | `grpc.address`, `grpc.enable` | 9090 | [SDK server config](../../../sdk/server/config/config.go) |
| gRPC-web | `grpc-web.address`, `grpc-web.enable` | 9091 | [SDK server config](../../../sdk/server/config/config.go) |
| REST gateway | `api.address`, `api.enable` | 1317 | [SDK server config](../../../sdk/server/config/config.go) |

The public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`. [server.go](../../../rpc/server.go) assembles services for each transport; local options, legacy gates and configured upstreams determine additional handlers. A source handler is not a promise that every deployment exposes it.

Tenant HTTP and WebSocket views use `/chain/<id>`; see [virtual chains](virtual-chains.md#rpc). Account state remains shared while block transaction and log views are scoped.

## Error model

JSON-RPC uses `-32700` for parse errors, `-32600` for invalid requests, `-32601` for unknown methods and `-32602` for invalid parameters. [RevertError](../../../rpc/simulate.go) reports EVM reverts with code `3` and revert data. [Unsupported methods](../../../rpc/rpc_unsupported.go) and unavailable sequencer-local reads use `-32000`. The [rate limiter](../../../rpc/ip_rate_limit.go) returns `-32005`; the [synchronous send handler](../../../rpc/send_sync.go) uses code `4` when the transaction has not been included within the configured timeout.

Module gRPC handlers use gRPC statuses; REST gateways map them to HTTP responses. See [protobuf APIs](proto-api.md) for schemas and annotations.

## JSON-RPC

The tables below identify method names and the current Go handler declarations. Activation and transport exposure follow server registration.

### Namespace debug

| Method | Handler | Source |
|---|---|---|
| `debug_getRawBlock` | `func (api *DebugAPI) GetRawBlock(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_getRawHeader` | `func (api *DebugAPI) GetRawHeader(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_getRawReceipts` | `func (api *DebugAPI) GetRawReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ []hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_getRawTransaction` | `func (api *DebugAPI) GetRawTransaction(ctx context.Context, hash common.Hash) (_ hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_traceBlockByHash` | `func (api *DebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_traceBlockByNumber` | `func (api *DebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_traceCall` | `func (api *DebugAPI) TraceCall(ctx context.Context, args export.TransactionArgs, blockNrOrHash rpc.BlockNumberOrHash, config *tracers.TraceCallConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_traceStateAccess` | `func (api *DebugAPI) TraceStateAccess(ctx context.Context, hash common.Hash) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_traceTransaction` | `func (api *DebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `debug_traceTransactionProfile` | `func (api *DebugAPI) TraceTransactionProfile(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/trace_profile.go](../../../rpc/trace_profile.go) |

### Namespace echo

| Method | Handler | Source |
|---|---|---|
| `echo_echo` | `func (a *EchoAPI) Echo(data string) string` | [rpc/echo.go](../../../rpc/echo.go) |

### Namespace eth

| Method | Handler | Source |
|---|---|---|
| `eth_accounts` | `func (i *InfoAPI) Accounts(ctx context.Context) (result []common.Address, returnErr error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_blobBaseFee` | `func (i *InfoAPI) BlobBaseFee(ctx context.Context) (result *hexutil.Big, returnErr error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_blockNumber` | `func (i *InfoAPI) BlockNumber(ctx context.Context) (result hexutil.Uint64, returnErr error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_call` | `func (s *SimulationAPI) Call(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride, blockOverrides *export.BlockOverrides) (result hexutil.Bytes, returnErr error)` | [rpc/simulate.go](../../../rpc/simulate.go) |
| `eth_chainId` | `func (i *InfoAPI) ChainId(ctx context.Context) *hexutil.Big` | [rpc/info.go](../../../rpc/info.go) |
| `eth_coinbase` | `func (i *InfoAPI) Coinbase(ctx context.Context) (addr common.Address, err error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_createAccessList` | `func (s *SimulationAPI) CreateAccessList(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash) (result *AccessListResult, returnErr error)` | [rpc/simulate.go](../../../rpc/simulate.go) |
| `eth_estimateGas` | `func (s *SimulationAPI) EstimateGas(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride) (result hexutil.Uint64, returnErr error)` | [rpc/simulate.go](../../../rpc/simulate.go) |
| `eth_estimateGasAfterCalls` | `func (s *SimulationAPI) EstimateGasAfterCalls(ctx context.Context, args export.TransactionArgs, calls []export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride) (result hexutil.Uint64, returnErr error)` | [rpc/simulate.go](../../../rpc/simulate.go) |
| `eth_feeHistory` | `func (i *InfoAPI) FeeHistory(ctx context.Context, blockCount gmath.HexOrDecimal64, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (result *FeeHistoryResult, returnErr error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_gasPrice` | `func (i *InfoAPI) GasPrice(ctx context.Context) (result *hexutil.Big, returnErr error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_getBalance` | `func (a *StateAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Big, returnErr error)` | [rpc/state.go](../../../rpc/state.go) |
| `eth_getBlockByHash` | `func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `eth_getBlockByNumber` | `func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `eth_getBlockReceipts` | `func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `eth_getBlockTransactionCountByHash` | `func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `eth_getBlockTransactionCountByNumber` | `func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `eth_getCode` | `func (a *StateAPI) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error)` | [rpc/state.go](../../../rpc/state.go) |
| `eth_getFilterChanges` | `func (a *FilterAPI) GetFilterChanges( ctx context.Context, filterID ethrpc.ID, ) (res interface{}, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `eth_getFilterLogs` | `func (a *FilterAPI) GetFilterLogs( ctx context.Context, filterID ethrpc.ID, ) (res []*FilterLog, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `eth_getLogs` | `func (a *FilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `eth_getNonce` | `func (a *StateAPI) GetNonce(ctx context.Context, address common.Address) uint64` | [rpc/state.go](../../../rpc/state.go) |
| `eth_getProof` | `func (a *StateAPI) GetProof(ctx context.Context, address common.Address, storageKeys []string, blockNrOrHash rpc.BlockNumberOrHash) (result *ProofResult, returnErr error)` | [rpc/state.go](../../../rpc/state.go) |
| `eth_getStorageAt` | `func (a *StateAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error)` | [rpc/state.go](../../../rpc/state.go) |
| `eth_getTransactionByBlockHashAndIndex` | `func (t *TransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_getTransactionByBlockNumberAndIndex` | `func (t *TransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_getTransactionByHash` | `func (t *TransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (result *export.RPCTransaction, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_getTransactionCount` | `func (t *TransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Uint64, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_getTransactionErrorByHash` | `func (t *TransactionAPI) GetTransactionErrorByHash(ctx context.Context, hash common.Hash) (result string, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_getTransactionReceipt` | `func (t *TransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_getVMError` | `func (t *TransactionAPI) GetVMError(ctx context.Context, hash common.Hash) (result string, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_maxPriorityFeePerGas` | `func (i *InfoAPI) MaxPriorityFeePerGas(ctx context.Context) (fee *hexutil.Big, returnErr error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_newBlockFilter` | `func (a *FilterAPI) NewBlockFilter( ctx context.Context, ) (id ethrpc.ID, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `eth_newFilter` | `func (a *FilterAPI) NewFilter( ctx context.Context, crit filters.FilterCriteria, ) (id ethrpc.ID, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `eth_newPendingTransactionFilter` | `func (a *FilterAPI) NewPendingTransactionFilter( ctx context.Context, fullTx *bool, ) (id ethrpc.ID, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `eth_sendRawTransaction` | `func (s *SendAPI) SendRawTransaction(ctx context.Context, input hexutil.Bytes) (hash common.Hash, err error)` | [rpc/send.go](../../../rpc/send.go) |
| `eth_sendTransaction` | `func (s *SendAPI) SendTransaction(ctx context.Context, args export.TransactionArgs) (result common.Hash, returnErr error)` | [rpc/send.go](../../../rpc/send.go) |
| `eth_sign` | `func (t *TransactionAPI) Sign(ctx context.Context, addr common.Address, data hexutil.Bytes) (result hexutil.Bytes, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `eth_signTransaction` | `func (s *SendAPI) SignTransaction(ctx context.Context, args apitypes.SendTxArgs, _ *string) (result *export.SignTransactionResult, returnErr error)` | [rpc/send.go](../../../rpc/send.go) |
| `eth_syncing` | `func (i *InfoAPI) Syncing(ctx context.Context) (result any, returnErr error)` | [rpc/info.go](../../../rpc/info.go) |
| `eth_uninstallFilter` | `func (a *FilterAPI) UninstallFilter( ctx context.Context, filterID ethrpc.ID, ) (res bool)` | [rpc/filter.go](../../../rpc/filter.go) |

### Namespace net

| Method | Handler | Source |
|---|---|---|
| `net_version` | `func (i *NetAPI) Version(ctx context.Context) string` | [rpc/net.go](../../../rpc/net.go) |

### Namespace pax

| Method | Handler | Source |
|---|---|---|
| `pax_associate` | `func (t *AssociationAPI) Associate(ctx context.Context, req *AssociateRequest) (returnErr error)` | [rpc/association.go](../../../rpc/association.go) |
| `pax_getBlockByHash` | `func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax_getBlockByHashExcludeTraceFail` | `func (a *PaxBlockAPI) GetBlockByHashExcludeTraceFail(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax_getBlockByNumber` | `func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax_getBlockByNumberExcludeTraceFail` | `func (a *PaxBlockAPI) GetBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax_getBlockReceipts` | `func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax_getBlockTransactionCountByHash` | `func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax_getBlockTransactionCountByNumber` | `func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax_getCosmosTx` | `func (t *AssociationAPI) GetCosmosTx(ctx context.Context, ethHash common.Hash) (result string, returnErr error)` | [rpc/association.go](../../../rpc/association.go) |
| `pax_getEVMAddress` | `func (t *AssociationAPI) GetEVMAddress(ctx context.Context, paxAddress string) (result string, returnErr error)` | [rpc/association.go](../../../rpc/association.go) |
| `pax_getEvmTx` | `func (t *AssociationAPI) GetEvmTx(ctx context.Context, cosmosHash string) (result string, returnErr error)` | [rpc/association.go](../../../rpc/association.go) |
| `pax_getFilterChanges` | `func (a *FilterAPI) GetFilterChanges( ctx context.Context, filterID ethrpc.ID, ) (res interface{}, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `pax_getFilterLogs` | `func (a *FilterAPI) GetFilterLogs( ctx context.Context, filterID ethrpc.ID, ) (res []*FilterLog, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `pax_getLogs` | `func (a *FilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `pax_getPaxAddress` | `func (t *AssociationAPI) GetPaxAddress(ctx context.Context, ethAddress common.Address) (result string, returnErr error)` | [rpc/association.go](../../../rpc/association.go) |
| `pax_getRawBlock` | `func (api *DebugAPI) GetRawBlock(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_getRawHeader` | `func (api *DebugAPI) GetRawHeader(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_getRawReceipts` | `func (api *DebugAPI) GetRawReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ []hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_getRawTransaction` | `func (api *DebugAPI) GetRawTransaction(ctx context.Context, hash common.Hash) (_ hexutil.Bytes, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_getTransactionByBlockHashAndIndex` | `func (t *TransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_getTransactionByBlockNumberAndIndex` | `func (t *TransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_getTransactionByHash` | `func (t *TransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (result *export.RPCTransaction, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_getTransactionCount` | `func (t *TransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Uint64, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_getTransactionErrorByHash` | `func (t *TransactionAPI) GetTransactionErrorByHash(ctx context.Context, hash common.Hash) (result string, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_getTransactionReceipt` | `func (t *TransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_getTransactionReceiptExcludeTraceFail` | `func (t *PaxTransactionAPI) GetTransactionReceiptExcludeTraceFail(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_getVMError` | `func (t *TransactionAPI) GetVMError(ctx context.Context, hash common.Hash) (result string, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_newBlockFilter` | `func (a *FilterAPI) NewBlockFilter( ctx context.Context, ) (id ethrpc.ID, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `pax_newFilter` | `func (a *FilterAPI) NewFilter( ctx context.Context, crit filters.FilterCriteria, ) (id ethrpc.ID, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `pax_newPendingTransactionFilter` | `func (a *FilterAPI) NewPendingTransactionFilter( ctx context.Context, fullTx *bool, ) (id ethrpc.ID, err error)` | [rpc/filter.go](../../../rpc/filter.go) |
| `pax_sequencerReceipt` | `func (a *SequencerReceiptAPI) SequencerReceipt(ctx context.Context, txHash common.Hash) (result *SequencerReceiptResult, returnErr error)` | [rpc/sequencer_receipt.go](../../../rpc/sequencer_receipt.go) |
| `pax_sign` | `func (t *TransactionAPI) Sign(ctx context.Context, addr common.Address, data hexutil.Bytes) (result hexutil.Bytes, returnErr error)` | [rpc/tx.go](../../../rpc/tx.go) |
| `pax_traceBlockByHash` | `func (api *DebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_traceBlockByHashExcludeTraceFail` | `func (api *PaxDebugAPI) TraceBlockByHashExcludeTraceFail(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_traceBlockByNumber` | `func (api *DebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_traceBlockByNumberExcludeTraceFail` | `func (api *PaxDebugAPI) TraceBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_traceCall` | `func (api *DebugAPI) TraceCall(ctx context.Context, args export.TransactionArgs, blockNrOrHash rpc.BlockNumberOrHash, config *tracers.TraceCallConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_traceStateAccess` | `func (api *DebugAPI) TraceStateAccess(ctx context.Context, hash common.Hash) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_traceTransaction` | `func (api *DebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/tracers.go](../../../rpc/tracers.go) |
| `pax_traceTransactionProfile` | `func (api *DebugAPI) TraceTransactionProfile(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)` | [rpc/trace_profile.go](../../../rpc/trace_profile.go) |
| `pax_uninstallFilter` | `func (a *FilterAPI) UninstallFilter( ctx context.Context, filterID ethrpc.ID, ) (res bool)` | [rpc/filter.go](../../../rpc/filter.go) |

### Namespace pax2

| Method | Handler | Source |
|---|---|---|
| `pax2_getBlockByHash` | `func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax2_getBlockByHashExcludeTraceFail` | `func (a *PaxBlockAPI) GetBlockByHashExcludeTraceFail(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax2_getBlockByNumber` | `func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax2_getBlockByNumberExcludeTraceFail` | `func (a *PaxBlockAPI) GetBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax2_getBlockReceipts` | `func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax2_getBlockTransactionCountByHash` | `func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |
| `pax2_getBlockTransactionCountByNumber` | `func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)` | [rpc/block.go](../../../rpc/block.go) |

### Namespace rpc

| Method | Handler | Source |
|---|---|---|
| `rpc_modules` | `func (s *RPCService) Modules() map[string]string` | [core/deps/geth/rpc/server.go](../../../core/deps/geth/rpc/server.go) |

### Namespace test

| Method | Handler | Source |
|---|---|---|
| `test_incrementPointerVersion` | `func (a *TestAPI) IncrementPointerVersion(pointerType string, offset int16) error` | [rpc/test.go](../../../rpc/test.go) |

### Namespace txpool

| Method | Handler | Source |
|---|---|---|
| `txpool_content` | `func (t *TxPoolAPI) Content(ctx context.Context) (result map[string]map[string]map[string]*export.RPCTransaction, returnErr error)` | [rpc/txpool.go](../../../rpc/txpool.go) |

### Namespace web3

| Method | Handler | Source |
|---|---|---|
| `web3_clientVersion` | `func (w *Web3API) ClientVersion() string` | [rpc/web3.go](../../../rpc/web3.go) |

## WebSocket

Subscriptions are registered by [server.go](../../../rpc/server.go) and [filters](../../../rpc/filter.go). `newHeads` follows shared block headers; tenant log subscriptions restrict logs to that domain. Receipt-stage events are distinct from committed EVM logs.

## gRPC

[Module schemas](proto-api.md) define request, response, Query and Msg services. The node registers services through module `RegisterServices` implementations and the SDK application router. A Msg schema defines a transaction message rather than an HTTP JSON-RPC method.

## REST

REST gateway routes follow protobuf HTTP annotations and module gateway registration. They use the API listener, independently of the EVM HTTP listener and its root-path JSON-RPC dispatcher.

## Other domains

See [sequencer receipts](sequencer.md), [hosted gateway](platform-gateway.md), [native ledger and auth](ledger-auth.md), [pay](pay.md), [programs](programs.md), [markets](markets.md), [web](web.md) and [virtual chains](virtual-chains.md) for their served contracts.

<a id="rpc-contract-json-rpc-websocket-grpc-and-rest-catalogue"></a>

<a id="summary-of-domain-7"></a>

<a id="json-rpc-methods"></a>

<a id="namespace-ws"></a>

<a id="grpc-services-of-domain-7"></a>

<a id="service-methods"></a>

<a id="queries"></a>

<a id="msg-services"></a>

<a id="rest-routes-of-domain-7"></a>

<a id="entries-frozen-in-this-file"></a>

<a id="module-queries"></a>

<a id="module-rest-routes"></a>

<a id="admin-service-proto-messages-and-build-formats"></a>

<a id="rpc-surfaces-of-other-domains"></a>

<a id="jsonrpc"></a>

<a id="jsonrpc-debug"></a>

<a id="jsonrpc-echo"></a>

<a id="jsonrpc-eth"></a>

<a id="jsonrpc-net"></a>

<a id="jsonrpc-pax"></a>

<a id="jsonrpc-pax2"></a>

<a id="jsonrpc-rpc"></a>

<a id="jsonrpc-test"></a>

<a id="jsonrpc-txpool"></a>

<a id="jsonrpc-web3"></a>

<a id="jsonrpc-ws"></a>

<a id="anchored-here"></a>

<a id="qry-epoch-epoch"></a>

<a id="qry-epoch-params"></a>

<a id="qry-evm-evmaddressbypaxaddress"></a>

<a id="qry-evm-paxaddressbyevmaddress"></a>

<a id="qry-evm-pointee"></a>

<a id="qry-evm-pointer"></a>

<a id="qry-evm-pointerversion"></a>

<a id="qry-evm-staticcall"></a>

<a id="qry-layerxprograms-programbalances"></a>

<a id="qry-layerxprograms-programevents"></a>

<a id="qry-layerxprograms-programstate"></a>

<a id="qry-mint-minter"></a>

<a id="qry-mint-params"></a>

<a id="qry-oracle-actives"></a>

<a id="qry-oracle-exchangerate"></a>

<a id="qry-oracle-exchangerates"></a>

<a id="qry-oracle-feederdelegation"></a>

<a id="qry-oracle-params"></a>

<a id="qry-oracle-pricesnapshothistory"></a>

<a id="qry-oracle-slashwindow"></a>

<a id="qry-oracle-twaps"></a>

<a id="qry-oracle-votepenaltycounter"></a>

<a id="qry-oracle-votetargets"></a>

<a id="qry-tokenfactory-denomallowlist"></a>

<a id="qry-tokenfactory-denomauthoritymetadata"></a>

<a id="qry-tokenfactory-denommetadata"></a>

<a id="qry-tokenfactory-denomsfromcreator"></a>

<a id="qry-tokenfactory-params"></a>

<a id="rest-oracle-actives"></a>

<a id="rest-oracle-exchangerates"></a>

<a id="rest-oracle-pricesnapshothistory"></a>

<a id="rest-oracle-twaps"></a>

<a id="rest-oracle-votetargets"></a>

<a id="rest-oracle-exchangerate"></a>

<a id="rest-oracle-params"></a>

<a id="rest-oracle-slashwindow"></a>

<a id="rest-oracle-feederdelegation"></a>

<a id="rest-oracle-votepenaltycounter"></a>

<a id="rest-epoch-epoch"></a>

<a id="rest-epoch-params"></a>

<a id="rest-evm-evmaddressbypaxaddress"></a>

<a id="rest-evm-paxaddressbyevmaddress"></a>

<a id="rest-evm-pointee"></a>

<a id="rest-evm-pointer"></a>

<a id="rest-evm-pointerversion"></a>

<a id="rest-evm-staticcall"></a>

<a id="rest-tokenfactory-denomallowlist"></a>

<a id="rest-tokenfactory-denommetadata"></a>

<a id="rest-tokenfactory-denomauthoritymetadata"></a>

<a id="rest-tokenfactory-denomsfromcreator"></a>

<a id="rest-tokenfactory-params"></a>

<a id="rest-mint-minter"></a>

<a id="rest-mint-params"></a>

<a id="rest-layerxgov-gov-proposal"></a>

<a id="rest-layerxbridge-gov-proposal"></a>

<a id="fmt-api-buf-gen-yaml"></a>

<a id="fmt-api-buf-yaml"></a>

<a id="grpcs-pax-admin-getloglevel"></a>

<a id="grpcs-pax-admin-listloggers"></a>

<a id="grpcs-pax-admin-setloglevel"></a>

<a id="proto-eth-accesslisttx"></a>

<a id="proto-eth-accesstuple"></a>

<a id="proto-eth-associatetx"></a>

<a id="proto-eth-blobtx"></a>

<a id="proto-eth-blobtxsidecar"></a>

<a id="proto-eth-dynamicfeetx"></a>

<a id="proto-eth-extensionoptionsethereumtx"></a>

<a id="proto-eth-legacytx"></a>

<a id="proto-eth-setcodeauthorization"></a>

<a id="proto-eth-setcodetx"></a>

<a id="proto-pax-admin-getloglevelrequest"></a>

<a id="proto-pax-admin-getloglevelresponse"></a>

<a id="proto-pax-admin-listloggersrequest"></a>

<a id="proto-pax-admin-listloggersresponse"></a>

<a id="proto-pax-admin-loggerinfo"></a>

<a id="proto-pax-admin-setloglevelrequest"></a>

<a id="proto-pax-admin-setloglevelresponse"></a>
