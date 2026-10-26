# Evm rpc contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## client-parity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getrawtransactionbyblockhashandindex-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/raw_tx_index.go</code> (planned) | <code>eth_getRawTransactionByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (hexutil.Bytes, error)</code> |
| <code>interface.7.rpc-eth-getrawtransactionbyblocknumberandindex-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/raw_tx_index.go</code> (planned) | <code>eth_getRawTransactionByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (hexutil.Bytes, error)</code> |
| <code>interface.7.rpc-eth-getrawtransactionbyhash-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/raw_tx.go</code> (planned) | <code>eth_getRawTransactionByHash(hash common.Hash) (hexutil.Bytes, error)</code> |
| <code>interface.7.rpc-eth-getunclebyblockhashandindex-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/uncle.go</code> (planned) | <code>eth_getUncleByBlockHashAndIndex(blockHash common.Hash, index hexutil.Uint) (map[string]interface{}, error)</code> |
| <code>interface.7.rpc-eth-getunclebyblocknumberandindex-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/uncle.go</code> (planned) | <code>eth_getUncleByBlockNumberAndIndex(blockNr rpc.BlockNumber, index hexutil.Uint) (map[string]interface{}, error)</code> |
| <code>interface.7.rpc-eth-getunclecountbyblockhash-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/uncle_count.go</code> (planned) | <code>eth_getUncleCountByBlockHash(blockHash common.Hash) (*hexutil.Uint, error)</code> |
| <code>interface.7.rpc-eth-getunclecountbyblocknumber-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/uncle_count.go</code> (planned) | <code>eth_getUncleCountByBlockNumber(blockNr rpc.BlockNumber) (*hexutil.Uint, error)</code> |
| <code>interface.7.rpc-eth-sendrawtransactionsync-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/send_sync.go</code> (planned) | <code>eth_sendRawTransactionSync(input hexutil.Bytes, timeoutMs *hexutil.Uint64) (map[string]interface{}, error)</code> |
| <code>interface.7.rpc-net-listening-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/net.go</code> (planned) | <code>net_listening() (bool, error)</code> |
| <code>interface.7.rpc-net-peercount-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/net_peers.go</code> (planned) | <code>net_peerCount() (hexutil.Uint, error)</code> |
| <code>interface.7.rpc-txpool-status-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/txpool.go</code> (planned) | <code>txpool_status() (map[string]hexutil.Uint, error)</code> |
| <code>interface.7.rpc-web3-sha3-j38</code> | <code>jsonrpc_method</code> | <code>new</code> | <code>rpc/web3.go</code> (planned) | <code>web3_sha3(input hexutil.Bytes) hexutil.Bytes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getrawtransactionbyblockhashandindex-j38</code> | notes | HTTP and WS; null for an unknown block; task 7.5.2 |
| <code>interface.7.rpc-eth-getrawtransactionbyblocknumberandindex-j38</code> | notes | HTTP and WS; null past the end; task 7.5.2 |
| <code>interface.7.rpc-eth-getrawtransactionbyhash-j38</code> | notes | HTTP and WS; binary envelope, null for an unknown hash, pending sequencer transactions included; tasks 7.5.1 and 7.9.2 |
| <code>interface.7.rpc-eth-getunclebyblockhashandindex-j38</code> | notes | HTTP and WS; always null; task 7.6.1 |
| <code>interface.7.rpc-eth-getunclebyblocknumberandindex-j38</code> | notes | HTTP and WS; always null; task 7.6.1 |
| <code>interface.7.rpc-eth-getunclecountbyblockhash-j38</code> | notes | HTTP and WS; 0x0 for a known block, null for an unknown block; task 7.6.2 |
| <code>interface.7.rpc-eth-getunclecountbyblocknumber-j38</code> | notes | HTTP and WS; 0x0 for a known block, null for an unknown block; task 7.6.2 |
| <code>interface.7.rpc-eth-sendrawtransactionsync-j38</code> | notes | HTTP and WS; returns the receipt or error code 4 with the hash as data on timeout (decision 702); tasks 7.8.1, 7.8.2 and 7.9.2 |
| <code>interface.7.rpc-net-listening-j38</code> | notes | HTTP and WS; NetInfo.Listening of the consensus client; task 7.4.2 |
| <code>interface.7.rpc-net-peercount-j38</code> | notes | HTTP and WS; NetInfo.NPeers; task 7.4.3 |
| <code>interface.7.rpc-txpool-status-j38</code> | notes | HTTP only; pending is the EVM transactions of the consensus mempool, queued 0x0 (decision 714); task 7.7.1 |
| <code>interface.7.rpc-web3-sha3-j38</code> | notes | HTTP and WS; Keccak-256 of the input; task 7.4.1 |

Contract context <code>contract_context.7.client-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Methods called by in-repo clients and the go-ethereum client are served on the transport the client uses. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Every method in the client matrix is served; uncle methods answer null or 0x0 because blocks carry no uncles. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.1.7</code> | req | 7001.7 |
| <code>acceptance.7.1.7</code> | scenario | Given the HTTP server of the package test setup and a sequencer receipt server, when each client matrix name is looked up in HTTPMethods, then every name is present |
| <code>acceptance.7.1.7</code> | test | rpc/client_matrix_test.go::TestClientMatrixMethodsServedHTTP (planned) |
| <code>acceptance.7.1.7</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_7 | Every eth_, net_, web3_, debug_, txpool_ and pax_ name that a client row of spec/layerx-merge/contracts/clients.md:282 calls is served over HTTP. |
| <code>acceptance.7.1.8</code> | req | 7001.8 |
| <code>acceptance.7.1.8</code> | scenario | Given the WebSocket server of the package test setup, when each client matrix name is looked up in WSMethods, then the eth_ names and the three subscriptions are present and no debug_, txpool_ or pax_ name is present |
| <code>acceptance.7.1.8</code> | test | rpc/client_matrix_test.go::TestClientMatrixMethodsServedWS (planned) |
| <code>acceptance.7.1.8</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_8 | Every eth_ read name and subscription a client row calls is served over WebSocket, and debug_, txpool_ and pax_ names are not, as registered at rpc/server.go:392. |
| <code>acceptance.7.4.1</code> | req | 7004.1 |
| <code>acceptance.7.4.1</code> | scenario | Given the HTTP server of the package test setup, when web3_sha3 is called with 0x68656c6c6f20776f726c64, then the result is 0x47173285a8d7341e5e972fc677286384f802f8ef42a5ec5f03bbfa254cb01fad |
| <code>acceptance.7.4.1</code> | test | rpc/web3_sha3_test.go::TestWeb3Sha3HashesInput (planned) |
| <code>acceptance.7.4.1</code> | qualification | UNRUN |
| <code>req.7004</code> | ac_1 | web3_sha3, absent at HEAD where Web3API has only ClientVersion (rpc/web3.go:7), returns the Keccak-256 of its input. |
| <code>acceptance.7.4.2</code> | req | 7004.2 |
| <code>acceptance.7.4.2</code> | scenario | Given the HTTP server of the package test setup whose consensus client reports Listening true, when net_listening is called, then the result is true |
| <code>acceptance.7.4.2</code> | test | rpc/net_listening_test.go::TestNetListeningReportsNetInfo (planned) |
| <code>acceptance.7.4.2</code> | qualification | UNRUN |
| <code>req.7004</code> | ac_2 | net_listening, absent at HEAD where NetAPI has only Version (rpc/net.go:24), returns the Listening field of the consensus NetInfo (consensus/rpc/coretypes/responses.go:221). |
| <code>acceptance.7.4.3</code> | req | 7004.3 |
| <code>acceptance.7.4.3</code> | scenario | Given the HTTP server of the package test setup whose consensus client reports three peers, when net_peerCount is called, then the result is 0x3 |
| <code>acceptance.7.4.3</code> | test | rpc/net_peers_test.go::TestNetPeerCountReportsPeers (planned) |
| <code>acceptance.7.4.3</code> | qualification | UNRUN |
| <code>req.7004</code> | ac_3 | net_peerCount, absent at HEAD, returns the NPeers field of the consensus NetInfo (consensus/rpc/coretypes/responses.go:223) as a hex quantity. |
| <code>acceptance.7.5.1</code> | req | 7005.1 |
| <code>acceptance.7.5.1</code> | scenario | Given a RawTransactionAPI built from the package test fixtures, when GetRawTransactionByHash is called with a committed transaction hash and with an unknown hash, then the first decodes to a transaction of that hash and the second is nil |
| <code>acceptance.7.5.1</code> | test | rpc/raw_tx_test.go::TestGetRawTransactionByHashReturnsEnvelope (planned) |
| <code>acceptance.7.5.1</code> | qualification | UNRUN |
| <code>req.7005</code> | ac_1 | GetRawTransactionByHash on RawTransactionAPI (planned) returns the binary envelope of a committed transaction found the way GetTransactionByHash finds it (rpc/tx.go:269) and nil for an unknown hash. |
| <code>acceptance.7.5.2</code> | req | 7005.2 |
| <code>acceptance.7.5.2</code> | scenario | Given a RawTransactionAPI over the multi transaction test block, when index 0 and an index past the end are requested by number, then the first decodes to the block's first transaction and the second is nil |
| <code>acceptance.7.5.2</code> | test | rpc/raw_tx_index_test.go::TestGetRawTransactionByBlockNumberAndIndex (planned) |
| <code>acceptance.7.5.2</code> | qualification | UNRUN |
| <code>req.7005</code> | ac_2 | GetRawTransactionByBlockNumberAndIndex (planned) returns the envelope of the transaction at that index of the block, resolved as rpc/tx.go:226 resolves it, and nil past the end. |
| <code>acceptance.7.5.3</code> | req | 7005.3 |
| <code>acceptance.7.5.3</code> | scenario | Given a RawTransactionAPI over the multi transaction test block, when index 0 is requested by the block hash and by an unknown hash, then the first decodes to the block's first transaction and the second is nil |
| <code>acceptance.7.5.3</code> | test | rpc/raw_tx_index_test.go::TestGetRawTransactionByBlockHashAndIndex (planned) |
| <code>acceptance.7.5.3</code> | qualification | UNRUN |
| <code>req.7005</code> | ac_3 | GetRawTransactionByBlockHashAndIndex (planned) returns the envelope at that index of the block found by hash, as rpc/tx.go:242 finds it, and nil for an unknown hash. |
| <code>acceptance.7.6.1</code> | req | 7006.1 |
| <code>acceptance.7.6.1</code> | scenario | Given an UncleAPI built from the package test fixtures, when both uncle lookups are called for block 1 index 0, then both return nil without an error |
| <code>acceptance.7.6.1</code> | test | rpc/uncle_test.go::TestUncleByBlockIndexIsNull (planned) |
| <code>acceptance.7.6.1</code> | qualification | UNRUN |
| <code>req.7006</code> | ac_1 | GetUncleByBlockNumberAndIndex and GetUncleByBlockHashAndIndex (planned) return null for every block, which carries sha3Uncles EmptyUncleHash (rpc/block.go:636). |
| <code>acceptance.7.6.2</code> | req | 7006.2 |
| <code>acceptance.7.6.2</code> | scenario | Given an UncleAPI built from the package test fixtures, when both uncle counts are requested for block 1 by number and by hash, then both return 0 |
| <code>acceptance.7.6.2</code> | test | rpc/uncle_count_test.go::TestUncleCountKnownBlockIsZero (planned) |
| <code>acceptance.7.6.2</code> | qualification | UNRUN |
| <code>req.7006</code> | ac_2 | GetUncleCountByBlockNumber and GetUncleCountByBlockHash (planned) return 0x0 for a committed block, resolved through getBlockNumber (rpc/utils.go:93). |
| <code>acceptance.7.6.3</code> | req | 7006.3 |
| <code>acceptance.7.6.3</code> | scenario | Given an UncleAPI built from the package test fixtures, when both uncle counts are requested for a future block number and an unknown hash, then both return nil without an error |
| <code>acceptance.7.6.3</code> | test | rpc/uncle_count_test.go::TestUncleCountUnknownBlockIsNull (planned) |
| <code>acceptance.7.6.3</code> | qualification | UNRUN |
| <code>req.7006</code> | ac_3 | GetUncleCountByBlockNumber and GetUncleCountByBlockHash (planned) return null for a block that does not exist, as rpc/block.go:137 does for transaction counts. |
| <code>acceptance.7.7.1</code> | req | 7007.1 |
| <code>acceptance.7.7.1</code> | scenario | Given the HTTP server of the package test setup whose consensus client holds unconfirmed EVM and Cosmos transactions, when txpool_status is called, then pending equals the EVM count and queued is 0x0 |
| <code>acceptance.7.7.1</code> | test | rpc/txpool_status_test.go::TestTxPoolStatusCountsPendingEVMTxs (planned) |
| <code>acceptance.7.7.1</code> | qualification | UNRUN |
| <code>req.7007</code> | ac_1 | txpool_status (planned) reports pending as the EVM transactions of UnconfirmedTxs, decoded as Content decodes them (rpc/txpool.go:40), and queued 0x0. |
| <code>acceptance.7.8.1</code> | req | 7008.1 |
| <code>acceptance.7.8.1</code> | scenario | Given the HTTP server of the package test setup and a signed transfer whose receipt the test keeper stores, when eth_sendRawTransactionSync is called with its bytes, then the result is a receipt whose transactionHash is the transfer hash |
| <code>acceptance.7.8.1</code> | test | rpc/send_sync_test.go::TestSendRawTransactionSyncReturnsReceipt (planned) |
| <code>acceptance.7.8.1</code> | qualification | UNRUN |
| <code>req.7008</code> | ac_1 | eth_sendRawTransactionSync (planned) sends through SendRawTransaction (rpc/send.go:164) and returns the receipt encoded as eth_getTransactionReceipt encodes it (rpc/tx.go:106). |
| <code>resolution.17.764</code> | title | decision.702 binds: eth_sendRawTransactionSync semantics |
| <code>resolution.17.764</code> | applies_to | decision.702<br>task.7.8.1<br>task.7.8.2<br>task.7.13.5 |
| <code>resolution.17.764</code> | source | decision.702 in fragments/J38/decision.kvx; tasks from affects |
| <code>resolution.17.766</code> | title | decision.704 binds: Methods not served |
| <code>resolution.17.766</code> | applies_to | decision.704<br>task.7.0.4 |
| <code>resolution.17.766</code> | source | decision.704 in fragments/J38/decision.kvx; tasks from affects |
| <code>resolution.17.772</code> | title | decision.710 binds: web3_clientVersion |
| <code>resolution.17.772</code> | applies_to | decision.710<br>task.7.4.1 |
| <code>resolution.17.772</code> | source | decision.710 in fragments/J38/decision.kvx; tasks from affects |
| <code>resolution.17.776</code> | title | decision.714 binds: txpool scope |
| <code>resolution.17.776</code> | applies_to | decision.714<br>task.7.7.1 |
| <code>resolution.17.776</code> | source | decision.714 in fragments/J38/decision.kvx; tasks from affects |
| <code>resolution.17.777</code> | title | decision.715 binds: Pending receipts |
| <code>resolution.17.777</code> | applies_to | decision.715<br>task.7.0.3<br>task.7.8.1 |
| <code>resolution.17.777</code> | source | decision.715 in fragments/J38/decision.kvx; tasks from affects |

## config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-batch-request-limit-j38</code> | <code>config_key</code> | <code>new</code> | <code>rpc/config/config.go</code> (planned) | <code>evm.batch_request_limit int (default 1000)</code> |
| <code>interface.7.cfg-evm-batch-response-max-size-j38</code> | <code>config_key</code> | <code>new</code> | <code>rpc/config/config.go</code> (planned) | <code>evm.batch_response_max_size int (default 25000000)</code> |
| <code>interface.7.cfg-evm-max-subscriptions-logs-j38</code> | <code>config_key</code> | <code>new</code> | <code>rpc/config/config.go</code> (planned) | <code>evm.max_subscriptions_logs uint64 (default 10000)</code> |
| <code>interface.7.cfg-evm-send-raw-tx-sync-timeout-j38</code> | <code>config_key</code> | <code>new</code> | <code>rpc/config/config.go</code> (planned) | <code>evm.send_raw_tx_sync_timeout duration (default 2s)</code> |
| <code>interface.7.go-evmrpcconfig-config-j38</code> | <code>go_exported</code> | <code>change</code> | <code>rpc/config/config.go:38</code> | <code>type Config struct</code> |
| <code>interface.7.go-evmrpcconfig-configtemplate-j38</code> | <code>go_exported</code> | <code>change</code> | <code>rpc/config/config.go:484</code> | <code>const ConfigTemplate = `</code> |
| <code>interface.7.go-evmrpcconfig-defaultconfig-j38</code> | <code>go_exported</code> | <code>change</code> | <code>rpc/config/config.go:176</code> | <code>var DefaultConfig = Config</code> |
| <code>interface.7.go-evmrpcconfig-readconfig-j38</code> | <code>go_exported</code> | <code>change</code> | <code>rpc/config/config.go:268</code> | <code>func ReadConfig(opts servertypes.AppOptions) (Config, error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-batch-request-limit-j38</code> | notes | Maximum calls in one JSON-RPC batch on HTTP and WS; 0 disables; tasks 7.13.5 and 7.13.4 (decision 720) |
| <code>interface.7.cfg-evm-batch-response-max-size-j38</code> | notes | Maximum bytes of one batch answer on HTTP and WS; 0 disables; tasks 7.13.5 and 7.13.4 (decision 720) |
| <code>interface.7.cfg-evm-max-subscriptions-logs-j38</code> | notes | Live logs subscription cap; 0 disables; tasks 7.13.5, 7.10.5 and 7.13.4 |
| <code>interface.7.cfg-evm-send-raw-tx-sync-timeout-j38</code> | notes | Default and cap of eth_sendRawTransactionSync waiting; tasks 7.13.5 and 7.9.2 |
| <code>interface.7.go-evmrpcconfig-config-j38</code> | notes | Gains the four keys of task 7.13.5 and loses CheckTxTimeout in task 7.13.2; 5 use(s) outside rpc/ |
| <code>interface.7.go-evmrpcconfig-configtemplate-j38</code> | notes | Renders every read key and deny_list as a TOML array; tasks 7.13.1 and 7.13.6; 1 use(s) outside rpc/ |
| <code>interface.7.go-evmrpcconfig-defaultconfig-j38</code> | notes | Defaults of the four new keys; task 7.13.5; 5 use(s) outside rpc/ |
| <code>interface.7.go-evmrpcconfig-readconfig-j38</code> | notes | Reads the four new keys and stops reading evm.checktx_timeout; tasks 7.13.5 and 7.13.2; 1 use(s) outside rpc/ |

Contract context <code>contract_context.7.config</code>:

| Field | Frozen rule |
|---|---|
| interface | Every key ReadConfig reads is rendered by ConfigTemplate; evm.checktx_timeout is retired and an app.toml carrying it still reads. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The rendered template decodes as TOML and round-trips DefaultConfig. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.transport-limits</code>:

| Field | Frozen rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size (decisions 711 and 720). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.13.1</code> | req | 7013.1 |
| <code>acceptance.7.13.1</code> | scenario | Given ConfigTemplate rendered over DefaultConfig, when the output is decoded as TOML, then the evm table holds the four keys with the DefaultConfig values |
| <code>acceptance.7.13.1</code> | test | rpc/config/template_test.go::TestConfigTemplateRendersReadKeys (planned) |
| <code>acceptance.7.13.1</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_1 | ConfigTemplate (rpc/config/config.go:484) renders max_tx_pool_txs, enable_test_api, max_concurrent_simulation_calls and rpc_stats_interval, keys ReadConfig reads at rpc/config/config.go:350 that the template omits at HEAD. |
| <code>acceptance.7.13.10</code> | req | 7013.10 |
| <code>acceptance.7.13.10</code> | scenario | Given a config with DenyList debug_traceBlockByNumber and eth_sign, when ConfigTemplate is rendered and decoded as TOML, then evm.deny_list is the two-element string array in that order |
| <code>acceptance.7.13.10</code> | test | rpc/config/template_denylist_test.go::TestConfigTemplateRendersDenyListArray (planned) |
| <code>acceptance.7.13.10</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_10 | ConfigTemplate renders evm.deny_list as a TOML string array instead of the Go slice text produced at rpc/config/config.go:552. |
| <code>acceptance.7.13.11</code> | req | 7013.11 |
| <code>acceptance.7.13.11</code> | scenario | Given ConfigTemplate rendered over DefaultConfig, when its evm keys are compared with the evm flag constants, then every flag has a rendered key |
| <code>acceptance.7.13.11</code> | test | rpc/config/template_flags_test.go::TestConfigTemplateCoversEveryFlag (planned) |
| <code>acceptance.7.13.11</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_11 | Every evm. flag ReadConfig reads (rpc/config/config.go:225-265) has a key in the rendered ConfigTemplate. |
| <code>acceptance.7.13.2</code> | req | 7013.2 |
| <code>acceptance.7.13.2</code> | scenario | Given app options with evm.checktx_timeout 5s, when ReadConfig is called and ConfigTemplate is rendered over DefaultConfig, then ReadConfig returns no error and the rendered text has no checktx_timeout line |
| <code>acceptance.7.13.2</code> | test | rpc/config/config_test.go::TestReadConfigIgnoresChecktxTimeout (planned) |
| <code>acceptance.7.13.2</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_2 | evm.checktx_timeout, read at rpc/config/config.go:345 and used nowhere, is retired: an app.toml that still carries it reads without an error and the template no longer renders it. |
| <code>acceptance.7.13.7</code> | req | 7013.7 |
| <code>acceptance.7.13.7</code> | scenario | Given app options with evm.batch_request_limit 50 and evm.batch_response_max_size 1000 and empty app options, when ReadConfig is called on each, then the first yields 50 and 1000 and the second 1000 and 25000000 |
| <code>acceptance.7.13.7</code> | test | rpc/config/config_test.go::TestReadConfigBatchLimits (planned) |
| <code>acceptance.7.13.7</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_7 | ReadConfig (rpc/config/config.go:268) reads evm.batch_request_limit and evm.batch_response_max_size and defaults them to 1000 and 25000000. |
| <code>acceptance.7.13.8</code> | req | 7013.8 |
| <code>acceptance.7.13.8</code> | scenario | Given app options with evm.send_raw_tx_sync_timeout 5s and with abc, when ReadConfig is called on each, then the first yields 5s and the second returns an error |
| <code>acceptance.7.13.8</code> | test | rpc/config/config_test.go::TestReadConfigSendRawTxSyncTimeout (planned) |
| <code>acceptance.7.13.8</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_8 | ReadConfig (rpc/config/config.go:268) reads evm.send_raw_tx_sync_timeout as a duration defaulting to 2s and refuses a value that does not parse. |
| <code>acceptance.7.13.9</code> | req | 7013.9 |
| <code>acceptance.7.13.9</code> | scenario | Given app options with evm.max_subscriptions_logs 7 and empty app options, when ReadConfig is called on each, then the first yields 7 and the second 10000 |
| <code>acceptance.7.13.9</code> | test | rpc/config/config_test.go::TestReadConfigMaxSubscriptionsLogs (planned) |
| <code>acceptance.7.13.9</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_9 | ReadConfig (rpc/config/config.go:268) reads evm.max_subscriptions_logs defaulting to 10000, the default of max_subscriptions_new_head at rpc/config/config.go:197. |

## config-checktx-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-checktx-timeout</code> | <code>config_key</code> | <code>retire</code> | <code>rpc/config/config.go:93</code> | <code>CheckTxTimeout time.Duration `mapstructure:"checktx_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-checktx-timeout</code> | notes | evm.checktx_timeout; Config.CheckTxTimeout time.Duration; default 5 * time.Second (rpc/config/config.go:191); parsed by ReadConfig (rpc/config/config.go:345-346) and in the template (rpc/config/config.go:543), but no production code reads CheckTxTimeout; only rpc/config/config_test.go:91 names it |
| <code>interface.7.cfg-evm-checktx-timeout</code> | overridden_by | override.J38.17 |

Contract context <code>contract_context.7.config</code>:

| Field | Frozen rule |
|---|---|
| interface | Every key ReadConfig reads is rendered by ConfigTemplate; evm.checktx_timeout is retired and an app.toml carrying it still reads. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The rendered template decodes as TOML and round-trips DefaultConfig. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.774</code> | title | decision.712 binds: checktx_timeout |
| <code>resolution.17.774</code> | applies_to | decision.712<br>task.7.13.2 |
| <code>resolution.17.774</code> | source | decision.712 in fragments/J38/decision.kvx; tasks from affects |

## config-cors-origins

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-cors-origins</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:84</code> | <code>CORSOrigins string `mapstructure:"cors_origins"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-cors-origins</code> | notes | evm.cors_origins; Config.CORSOrigins string; default "*" (rpc/config/config.go:188) |

## config-deny-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-deny-list</code> | <code>config_key</code> | <code>change</code> | <code>rpc/config/config.go:102</code> | <code>DenyList []string `mapstructure:"deny_list"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-deny-list</code> | notes | evm.deny_list; Config.DenyList []string; default make([]string, 0) (rpc/config/config.go:194); applied through RegisterDenyList on HTTP only (rpc/rpcstack.go:317-319); the WS server (rpc/server.go:315-441) ignores it, so denied methods stay callable over WS |

Contract context <code>contract_context.7.config</code>:

| Field | Frozen rule |
|---|---|
| interface | Every key ReadConfig reads is rendered by ConfigTemplate; evm.checktx_timeout is retired and an app.toml carrying it still reads. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The rendered template decodes as TOML and round-trips DefaultConfig. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.lx-switch-off</code>:

| Field | Frozen rule |
|---|---|
| interface | No lx_* method is registered by the node; every lx_* name answers -32601 on HTTP and WS (decision 701). The gateway switch-off belongs to domain 8. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No served name starts with lx_. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.ws-transport</code>:

| Field | Frozen rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug_, txpool_ and pax_ stay HTTP-only (decision 708). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config-enable-parallelized-block-trace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-enable-parallelized-block-trace</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:136</code> | <code>EnableParallelizedBlockTrace bool `mapstructure:"enable_parallelized_block_trace"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-enable-parallelized-block-trace</code> | notes | evm.enable_parallelized_block_trace; Config.EnableParallelizedBlockTrace bool; default false (rpc/config/config.go:204) |

## config-enable-test-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-enable-test-api</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:114</code> | <code>EnableTestAPI bool `mapstructure:"enable_test_api"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-enable-test-api</code> | notes | evm.enable_test_api; Config.EnableTestAPI bool; default false (rpc/config/config.go:198); read by ReadConfig but absent from ConfigTemplate (rpc/config/config.go:484-679); the test namespace is registered only on a non-live chain id (rpc/server.go:293) |

## config-enable-unsafe-keyring-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-enable-unsafe-keyring-rpc</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:119</code> | <code>EnableUnsafeKeyringRPC bool `mapstructure:"enable_unsafe_keyring_rpc"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-enable-unsafe-keyring-rpc</code> | notes | evm.enable_unsafe_keyring_rpc; Config.EnableUnsafeKeyringRPC bool; default false (rpc/config/config.go:199); startup fails when set on a live chain id (rpc/server.go:104, :344) |

## config-enabled-legacy-pax-apis

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-enabled-legacy-pax-apis</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:151</code> | <code>EnabledLegacyPaxApis []string `mapstructure:"enabled_legacy_pax_apis"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-enabled-legacy-pax-apis</code> | notes | evm.enabled_legacy_pax_apis; Config.EnabledLegacyPaxApis []string; default []string{ (rpc/config/config.go:208); names outside paxLegacyGatedMethods are ignored (rpc/pax_legacy.go:110-126) |

## config-filter-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-filter-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:90</code> | <code>FilterTimeout time.Duration `mapstructure:"filter_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-filter-timeout</code> | notes | evm.filter_timeout; Config.FilterTimeout time.Duration; default 120 * time.Second (rpc/config/config.go:190) |

## config-http-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-http-address</code> | <code>config_key</code> | <code>change</code> | <code>rpc/config/config.go:40</code> | <code>HTTPAddress string `mapstructure:"http_address"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-http-address</code> | notes | evm.http_address; Config.HTTPAddress string; default the all-interfaces address (rpc/config/config.go:177); HTTP binds it (rpc/server.go:136) and it must be a literal IP (rpc/config/config.go:277), but the WS listener binds LocalAddress the all-interfaces address (rpc/server.go:361) regardless |

Contract context <code>contract_context.7.ws-transport</code>:

| Field | Frozen rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug_, txpool_ and pax_ stay HTTP-only (decision 708). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config-http-enabled

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-http-enabled</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:41</code> | <code>HTTPEnabled bool   `mapstructure:"http_enabled"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-http-enabled</code> | notes | evm.http_enabled; Config.HTTPEnabled bool; default true (rpc/config/config.go:178) |

## config-http-port

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-http-port</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:42</code> | <code>HTTPPort    int    `mapstructure:"http_port"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-http-port</code> | notes | evm.http_port; Config.HTTPPort int; default 8545 (rpc/config/config.go:179) |

## config-idle-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-idle-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:75</code> | <code>IdleTimeout time.Duration `mapstructure:"idle_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-idle-timeout</code> | notes | evm.idle_timeout; Config.IdleTimeout time.Duration; default rpc.DefaultHTTPTimeouts.IdleTimeout (rpc/config/config.go:185) |

## config-ip-rate-limit-burst

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-ip-rate-limit-burst</code> | <code>config_key</code> | <code>change</code> | <code>rpc/config/config.go:173</code> | <code>IPRateLimitBurst int `mapstructure:"ip_rate_limit_burst"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-ip-rate-limit-burst</code> | notes | evm.ip_rate_limit_burst; Config.IPRateLimitBurst int; default 400 (rpc/config/config.go:221); the template promises a per-IP burst (rpc/config/config.go:676-677), but no production code reads IPRateLimitBurst; only rpc/config/config_test.go:160 names it |

Contract context <code>contract_context.7.transport-limits</code>:

| Field | Frozen rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size (decisions 711 and 720). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## config-ip-rate-limit-rps

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-ip-rate-limit-rps</code> | <code>config_key</code> | <code>change</code> | <code>rpc/config/config.go:170</code> | <code>IPRateLimitRPS float64 `mapstructure:"ip_rate_limit_rps"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-ip-rate-limit-rps</code> | notes | evm.ip_rate_limit_rps; Config.IPRateLimitRPS float64; default 200 (rpc/config/config.go:220); the template promises per-IP rate limiting (rpc/config/config.go:672-674), but no production code reads IPRateLimitRPS; only rpc/config/config_test.go:157 names it |

Contract context <code>contract_context.7.transport-limits</code>:

| Field | Frozen rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size (decisions 711 and 720). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.773</code> | title | decision.711 binds: Rate limit key and answer |
| <code>resolution.17.773</code> | applies_to | decision.711<br>task.7.13.3<br>task.7.13.4 |
| <code>resolution.17.773</code> | source | decision.711 in fragments/J38/decision.kvx; tasks from affects |
| <code>resolution.17.782</code> | title | decision.720 binds: Batch limit defaults |
| <code>resolution.17.782</code> | applies_to | decision.720<br>task.7.13.4<br>task.7.13.5 |
| <code>resolution.17.782</code> | source | decision.720 in fragments/J38/decision.kvx; tasks from affects |

## config-max-blocks-for-log

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-max-blocks-for-log</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:108</code> | <code>MaxBlocksForLog int64 `mapstructure:"max_blocks_for_log"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-max-blocks-for-log</code> | notes | evm.max_blocks_for_log; Config.MaxBlocksForLog int64; default 2000 (rpc/config/config.go:196) |

## config-max-concurrent-simulation-calls

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-max-concurrent-simulation-calls</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:127</code> | <code>MaxConcurrentSimulationCalls int `mapstructure:"max_concurrent_simulation_calls"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-max-concurrent-simulation-calls</code> | notes | evm.max_concurrent_simulation_calls; Config.MaxConcurrentSimulationCalls int; default runtime.NumCPU() (rpc/config/config.go:201); read by ReadConfig but absent from ConfigTemplate (rpc/config/config.go:484-679) |

## config-max-concurrent-trace-calls

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-max-concurrent-trace-calls</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:123</code> | <code>MaxConcurrentTraceCalls uint64 `mapstructure:"max_concurrent_trace_calls"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-max-concurrent-trace-calls</code> | notes | evm.max_concurrent_trace_calls; Config.MaxConcurrentTraceCalls uint64; default 10 (rpc/config/config.go:200) |

## config-max-log-no-block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-max-log-no-block</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:105</code> | <code>MaxLogNoBlock int64 `mapstructure:"max_log_no_block"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-max-log-no-block</code> | notes | evm.max_log_no_block; Config.MaxLogNoBlock int64; default 10000 (rpc/config/config.go:195) |

## config-max-subscriptions-new-head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-max-subscriptions-new-head</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:111</code> | <code>MaxSubscriptionsNewHead uint64 `mapstructure:"max_subscriptions_new_head"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-max-subscriptions-new-head</code> | notes | evm.max_subscriptions_new_head; Config.MaxSubscriptionsNewHead uint64; default 10000 (rpc/config/config.go:197) |

## config-max-trace-lookback-blocks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-max-trace-lookback-blocks</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:130</code> | <code>MaxTraceLookbackBlocks int64 `mapstructure:"max_trace_lookback_blocks"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-max-trace-lookback-blocks</code> | notes | evm.max_trace_lookback_blocks; Config.MaxTraceLookbackBlocks int64; default 10000 (rpc/config/config.go:202) |

## config-max-tx-pool-txs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-max-tx-pool-txs</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:96</code> | <code>MaxTxPoolTxs uint64 `mapstructure:"max_tx_pool_txs"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-max-tx-pool-txs</code> | notes | evm.max_tx_pool_txs; Config.MaxTxPoolTxs uint64; default 1000 (rpc/config/config.go:192); read by ReadConfig but absent from ConfigTemplate (rpc/config/config.go:484-679) |

## config-read-header-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-read-header-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:63</code> | <code>ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-read-header-timeout</code> | notes | evm.read_header_timeout; Config.ReadHeaderTimeout time.Duration; default rpc.DefaultHTTPTimeouts.ReadHeaderTimeout (rpc/config/config.go:183) |

## config-read-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-read-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:55</code> | <code>ReadTimeout time.Duration `mapstructure:"read_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-read-timeout</code> | notes | evm.read_timeout; Config.ReadTimeout time.Duration; default rpc.DefaultHTTPTimeouts.ReadTimeout (rpc/config/config.go:182) |

## config-rpc-stats-interval

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-rpc-stats-interval</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:139</code> | <code>RPCStatsInterval time.Duration `mapstructure:"rpc_stats_interval"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-rpc-stats-interval</code> | notes | evm.rpc_stats_interval; Config.RPCStatsInterval time.Duration; default 10 * time.Second (rpc/config/config.go:205); read by ReadConfig but absent from ConfigTemplate (rpc/config/config.go:484-679) |

## config-simulation-evm-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-simulation-evm-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:81</code> | <code>SimulationEVMTimeout time.Duration `mapstructure:"simulation_evm_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-simulation-evm-timeout</code> | notes | evm.simulation_evm_timeout; Config.SimulationEVMTimeout time.Duration; default 60 * time.Second (rpc/config/config.go:187) |

## config-simulation-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-simulation-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:78</code> | <code>SimulationGasLimit uint64 `mapstructure:"simulation_gas_limit"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-simulation-gas-limit</code> | notes | evm.simulation_gas_limit; Config.SimulationGasLimit uint64; default 10_000_000 (rpc/config/config.go:186) |

## config-slow

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-slow</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:99</code> | <code>Slow bool `mapstructure:"slow"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-slow</code> | notes | evm.slow; Config.Slow bool; default false (rpc/config/config.go:193) |

## config-trace-bake-enabled

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-enabled</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:156</code> | <code>TraceBakeEnabled      bool     `mapstructure:"trace_bake_enabled"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-enabled</code> | notes | evm.trace_bake_enabled; Config.TraceBakeEnabled bool; default false (rpc/config/config.go:213) |

## config-trace-bake-queue-size

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-queue-size</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:158</code> | <code>TraceBakeQueueSize    int      `mapstructure:"trace_bake_queue_size"`    // in-flight height queue (default 4096)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-queue-size</code> | notes | evm.trace_bake_queue_size; Config.TraceBakeQueueSize int; default 4096 (rpc/config/config.go:215) |

## config-trace-bake-snapshot-window

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-snapshot-window</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:166</code> | <code>TraceBakeSnapshotWindow int64 `mapstructure:"trace_bake_snapshot_window"` // recent snapshots to keep (default 64)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-snapshot-window</code> | notes | evm.trace_bake_snapshot_window; Config.TraceBakeSnapshotWindow int64; default 64 (rpc/config/config.go:219) |

## config-trace-bake-tracers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-tracers</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:159</code> | <code>TraceBakeTracers      []string `mapstructure:"trace_bake_tracers"`       // tracers to bake (default ["callTracer"])</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-tracers</code> | notes | evm.trace_bake_tracers; Config.TraceBakeTracers []string; default []string{"callTracer"} (rpc/config/config.go:216); the template line about trace baker dropped and baked counters (rpc/config/config.go:665) has no exported metric behind it; the counts are Go accessors and logs only (rpc/trace_baker.go:44, :153-154) |

## config-trace-bake-use-snapshot

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-use-snapshot</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:165</code> | <code>TraceBakeUseSnapshot    bool  `mapstructure:"trace_bake_use_snapshot"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-use-snapshot</code> | notes | evm.trace_bake_use_snapshot; Config.TraceBakeUseSnapshot bool; default false (rpc/config/config.go:218) |

## config-trace-bake-window-blocks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-window-blocks</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:160</code> | <code>TraceBakeWindowBlocks int64    `mapstructure:"trace_bake_window_blocks"` // rolling prune window; 0 disables</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-window-blocks</code> | notes | evm.trace_bake_window_blocks; Config.TraceBakeWindowBlocks int64; default 0 (rpc/config/config.go:217); 0 disables the rolling prune window |

## config-trace-bake-workers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-workers</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:157</code> | <code>TraceBakeWorkers      int      `mapstructure:"trace_bake_workers"`       // re-execution goroutines (default 1)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-bake-workers</code> | notes | evm.trace_bake_workers; Config.TraceBakeWorkers int; default 1 (rpc/config/config.go:214) |

## config-trace-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-trace-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:133</code> | <code>TraceTimeout time.Duration `mapstructure:"trace_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-trace-timeout</code> | notes | evm.trace_timeout; Config.TraceTimeout time.Duration; default 30 * time.Second (rpc/config/config.go:203) |

## config-worker-pool-size

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-worker-pool-size</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:143</code> | <code>WorkerPoolSize int `mapstructure:"worker_pool_size"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-worker-pool-size</code> | notes | evm.worker_pool_size; Config.WorkerPoolSize int; default min(MaxWorkerPoolSize, runtime.NumCPU()*2) (rpc/config/config.go:206) |

## config-worker-queue-size

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-worker-queue-size</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:147</code> | <code>WorkerQueueSize int `mapstructure:"worker_queue_size"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-worker-queue-size</code> | notes | evm.worker_queue_size; Config.WorkerQueueSize int; default DefaultWorkerQueueSize (rpc/config/config.go:207) |

## config-write-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-write-timeout</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:69</code> | <code>WriteTimeout time.Duration `mapstructure:"write_timeout"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-write-timeout</code> | notes | evm.write_timeout; Config.WriteTimeout time.Duration; default rpc.DefaultHTTPTimeouts.WriteTimeout (rpc/config/config.go:184) |

## config-ws-enabled

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-ws-enabled</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:45</code> | <code>WSEnabled bool `mapstructure:"ws_enabled"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-ws-enabled</code> | notes | evm.ws_enabled; Config.WSEnabled bool; default true (rpc/config/config.go:180) |

## config-ws-origins

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-ws-origins</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:87</code> | <code>WSOrigins string `mapstructure:"ws_origins"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-ws-origins</code> | notes | evm.ws_origins; Config.WSOrigins string; default "*" (rpc/config/config.go:189) |

## config-ws-port

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.cfg-evm-ws-port</code> | <code>config_key</code> | <code>stable</code> | <code>rpc/config/config.go:46</code> | <code>WSPort    int  `mapstructure:"ws_port"`</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.cfg-evm-ws-port</code> | notes | evm.ws_port; Config.WSPort int; default 8546 (rpc/config/config.go:181) |

## conformance

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.19.1</code> | req | 7019.1 |
| <code>acceptance.7.19.1</code> | scenario | Given an ethclient dialled to the package test HTTP server, when BlockByNumber is called for block 2, then it returns a block with number 2 and the transactions of the multi transaction block |
| <code>acceptance.7.19.1</code> | test | rpc/ethclient_blocks_test.go::TestEthclientBlockByNumberDecodes (planned) |
| <code>acceptance.7.19.1</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_1 | ethclient BlockByNumber (core/deps/geth/ethclient/ethclient.go:105) decodes eth_getBlockByNumber of the test server, passing the uncle and transaction checks of getBlock (core/deps/geth/ethclient/ethclient.go:140). |
| <code>acceptance.7.19.10</code> | req | 7019.10 |
| <code>acceptance.7.19.10</code> | scenario | Given an ethclient dialled to the package test HTTP server, when EstimateGas is called for a value transfer between two accounts, then the result is 21000 |
| <code>acceptance.7.19.10</code> | test | rpc/ethclient_call_test.go::TestEthclientEstimateGas (planned) |
| <code>acceptance.7.19.10</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_10 | ethclient EstimateGas (core/deps/geth/ethclient/ethclient.go:650) returns 21000 for a plain value transfer. |
| <code>acceptance.7.19.11</code> | req | 7019.11 |
| <code>acceptance.7.19.11</code> | scenario | Given an ethclient dialled to the package test HTTP server, when SuggestGasPrice and SuggestGasTipCap are called, then both are positive and the price is not below the tip |
| <code>acceptance.7.19.11</code> | test | rpc/ethclient_fees_test.go::TestEthclientSuggestGasPriceAndTip (planned) |
| <code>acceptance.7.19.11</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_11 | ethclient SuggestGasPrice (core/deps/geth/ethclient/ethclient.go:583) and SuggestGasTipCap (core/deps/geth/ethclient/ethclient.go:593) return positive values with the price at least the tip. |
| <code>acceptance.7.19.12</code> | req | 7019.12 |
| <code>acceptance.7.19.12</code> | scenario | Given an ethclient dialled to the package test HTTP server, when FeeHistory is called with blockCount 2 and percentiles 50, then BaseFee has 3 entries and Reward has 2 rows |
| <code>acceptance.7.19.12</code> | test | rpc/ethclient_fees_test.go::TestEthclientFeeHistory (planned) |
| <code>acceptance.7.19.12</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_12 | ethclient FeeHistory (core/deps/geth/ethclient/ethclient.go:618) decodes eth_feeHistory with blockCount base fees plus one and one reward row per block. |
| <code>acceptance.7.19.13</code> | req | 7019.13 |
| <code>acceptance.7.19.13</code> | scenario | Given an ethclient dialled to the package test HTTP server, when ChainID and NetworkID are called, then both equal the keeper chain id |
| <code>acceptance.7.19.13</code> | test | rpc/ethclient_chain_test.go::TestEthclientChainIDAndNetworkID (planned) |
| <code>acceptance.7.19.13</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_13 | ethclient ChainID (core/deps/geth/ethclient/ethclient.go:71) and NetworkID (core/deps/geth/ethclient/ethclient.go:382) both return the keeper chain id that net_version formats at rpc/net.go:24. |
| <code>acceptance.7.19.14</code> | req | 7019.14 |
| <code>acceptance.7.19.14</code> | scenario | Given an ethclient dialled to the package test HTTP server whose consensus client reports no catch-up, when SyncProgress is called, then it returns nil without an error |
| <code>acceptance.7.19.14</code> | test | rpc/ethclient_chain_test.go::TestEthclientSyncProgress (planned) |
| <code>acceptance.7.19.14</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_14 | ethclient SyncProgress (core/deps/geth/ethclient/ethclient.go:349) decodes eth_syncing as nil for a node that is not catching up. |
| <code>acceptance.7.19.15</code> | req | 7019.15 |
| <code>acceptance.7.19.15</code> | scenario | Given an ethclient dialled to the package test HTTP server, when FilterLogs is called for block 2 by range and by its hash, then both return the same non-empty log list with blockHash set |
| <code>acceptance.7.19.15</code> | test | rpc/ethclient_logs_test.go::TestEthclientFilterLogs (planned) |
| <code>acceptance.7.19.15</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_15 | ethclient FilterLogs (core/deps/geth/ethclient/ethclient.go:457) decodes eth_getLogs over a block range and by block hash with the same logs. |
| <code>acceptance.7.19.16</code> | req | 7019.16 |
| <code>acceptance.7.19.16</code> | scenario | Given an ethclient dialled to the package test WebSocket server with a newHeads subscription, when one head is received, then header.Hash() equals the hash of BlockByNumber at that height |
| <code>acceptance.7.19.16</code> | test | rpc/ethclient_newhead_test.go::TestEthclientSubscribeNewHeadHashMatchesBlock (planned) |
| <code>acceptance.7.19.16</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_16 | A header received through ethclient SubscribeNewHead (core/deps/geth/ethclient/ethclient.go:368) hashes to the hash eth_getBlockByNumber returns for the same height. |
| <code>acceptance.7.19.17</code> | req | 7019.17 |
| <code>acceptance.7.19.17</code> | scenario | Given an ethclient dialled to the package test HTTP server and a signed transfer, when SendTransaction is called and the test block containing it is committed, then TransactionByHash and TransactionReceipt return the transfer and a status 1 receipt |
| <code>acceptance.7.19.17</code> | test | rpc/ethclient_send_test.go::TestEthclientSendTransactionRoundTrip (planned) |
| <code>acceptance.7.19.17</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_17 | A transaction sent with ethclient SendTransaction (core/deps/geth/ethclient/ethclient.go:685) is returned by TransactionByHash and its receipt by TransactionReceipt once committed. |
| <code>acceptance.7.19.18</code> | req | 7019.18 |
| <code>acceptance.7.19.18</code> | scenario | Given an ethclient dialled to an HTTP server built with SequencerOption and PendingOption holding one pending transfer from the test account, when PendingNonceAt and eth_getBlockByNumber pending are read, then the nonce is the committed nonce plus one and the pending block lists the transfer hash |
| <code>acceptance.7.19.18</code> | test | rpc/ethclient_pending_test.go::TestEthclientPendingNonceAndPendingBlock (planned) |
| <code>acceptance.7.19.18</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_18 | ethclient PendingNonceAt (core/deps/geth/ethclient/ethclient.go:529) counts a pending transfer and the pending block lists it on a server built with the sequencer and pending options. |
| <code>acceptance.7.19.2</code> | req | 7019.2 |
| <code>acceptance.7.19.2</code> | scenario | Given an ethclient dialled to the package test HTTP server, when HeaderByHash is called with the hash of block 2, then number, parentHash, stateRoot and receiptsRoot equal those of BlockByNumber 2 |
| <code>acceptance.7.19.2</code> | test | rpc/ethclient_blocks_test.go::TestEthclientHeaderByHashMatchesBlock (planned) |
| <code>acceptance.7.19.2</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_2 | ethclient HeaderByHash (core/deps/geth/ethclient/ethclient.go:216) returns the header whose fields equal those of the block returned by number. |
| <code>acceptance.7.19.3</code> | req | 7019.3 |
| <code>acceptance.7.19.3</code> | scenario | Given an ethclient dialled to the package test HTTP server, when TransactionByHash is called with a committed hash, then it returns that transaction and isPending false |
| <code>acceptance.7.19.3</code> | test | rpc/ethclient_transactions_test.go::TestEthclientTransactionByHash (planned) |
| <code>acceptance.7.19.3</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_3 | ethclient TransactionByHash (core/deps/geth/ethclient/ethclient.go:268) decodes a committed transaction with isPending false. |
| <code>acceptance.7.19.4</code> | req | 7019.4 |
| <code>acceptance.7.19.4</code> | scenario | Given an ethclient dialled to the package test HTTP server, when TransactionInBlock is called with the hash of block 2 and index 0, then it returns the first transaction of the block |
| <code>acceptance.7.19.4</code> | test | rpc/ethclient_transactions_test.go::TestEthclientTransactionInBlock (planned) |
| <code>acceptance.7.19.4</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_4 | ethclient TransactionInBlock (core/deps/geth/ethclient/ethclient.go:319) returns the transaction at the index of a block found by hash. |
| <code>acceptance.7.19.5</code> | req | 7019.5 |
| <code>acceptance.7.19.5</code> | scenario | Given an ethclient dialled to the package test HTTP server, when TransactionReceipt is called with a committed hash, then status is 1 and blockHash equals the hash of the containing block |
| <code>acceptance.7.19.5</code> | test | rpc/ethclient_receipts_test.go::TestEthclientTransactionReceiptDecodes (planned) |
| <code>acceptance.7.19.5</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_5 | ethclient TransactionReceipt (core/deps/geth/ethclient/ethclient.go:338) decodes eth_getTransactionReceipt with status, gasUsed, logs and blockHash. |
| <code>acceptance.7.19.6</code> | req | 7019.6 |
| <code>acceptance.7.19.6</code> | scenario | Given an ethclient dialled to the package test HTTP server, when BlockReceipts is called for block 2, then the number of receipts equals the number of block transactions |
| <code>acceptance.7.19.6</code> | test | rpc/ethclient_receipts_test.go::TestEthclientBlockReceiptsDecode (planned) |
| <code>acceptance.7.19.6</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_6 | ethclient BlockReceipts (core/deps/geth/ethclient/ethclient.go:124) decodes eth_getBlockReceipts with one receipt per EVM transaction. |
| <code>acceptance.7.19.7</code> | req | 7019.7 |
| <code>acceptance.7.19.7</code> | scenario | Given an ethclient dialled to the package test HTTP server, when BalanceAt and NonceAt are called for the funded test account at latest, then they equal the keeper balance and nonce |
| <code>acceptance.7.19.7</code> | test | rpc/ethclient_state_test.go::TestEthclientBalanceAndNonceAt (planned) |
| <code>acceptance.7.19.7</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_7 | ethclient BalanceAt (core/deps/geth/ethclient/ethclient.go:396) and NonceAt (core/deps/geth/ethclient/ethclient.go:441) return the funded test account balance and nonce. |
| <code>acceptance.7.19.8</code> | req | 7019.8 |
| <code>acceptance.7.19.8</code> | scenario | Given an ethclient dialled to the package test HTTP server, when CodeAt and StorageAt slot 0 are called for the test contract, then they equal the keeper code and state |
| <code>acceptance.7.19.8</code> | test | rpc/ethclient_state_test.go::TestEthclientCodeAndStorageAt (planned) |
| <code>acceptance.7.19.8</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_8 | ethclient CodeAt (core/deps/geth/ethclient/ethclient.go:426) and StorageAt (core/deps/geth/ethclient/ethclient.go:411) return the deployed test contract code and slot 0. |
| <code>acceptance.7.19.9</code> | req | 7019.9 |
| <code>acceptance.7.19.9</code> | scenario | Given an ethclient dialled to the package test HTTP server, when CallContract runs the test contract view at latest, then the output equals the stored value |
| <code>acceptance.7.19.9</code> | test | rpc/ethclient_call_test.go::TestEthclientCallContract (planned) |
| <code>acceptance.7.19.9</code> | qualification | UNRUN |
| <code>req.7019</code> | ac_9 | ethclient CallContract (core/deps/geth/ethclient/ethclient.go:550) returns the output of a view call on the test contract. |

## debug-getrawblock

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-getrawblock</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:723</code> | <code>func (api *DebugAPI) GetRawBlock(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-getrawblock</code> | notes | debug_getRawBlock; HTTP only; always returns ErrEVMNotSupported, as rpc/AGENTS.md:12 states; 3 call site(s) in 1 file(s) |

## debug-getrawheader

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-getrawheader</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:715</code> | <code>func (api *DebugAPI) GetRawHeader(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-getrawheader</code> | notes | debug_getRawHeader; HTTP only; always returns ErrEVMNotSupported, as rpc/AGENTS.md:12 states; 3 call site(s) in 1 file(s) |

## debug-getrawreceipts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-getrawreceipts</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:731</code> | <code>func (api *DebugAPI) GetRawReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ []hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-getrawreceipts</code> | notes | debug_getRawReceipts; HTTP only; always returns ErrEVMNotSupported, as rpc/AGENTS.md:12 states; 3 call site(s) in 1 file(s) |

## debug-getrawtransaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-getrawtransaction</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/tracers.go:739</code> | <code>func (api *DebugAPI) GetRawTransaction(ctx context.Context, hash common.Hash) (_ hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-getrawtransaction</code> | notes | debug_getRawTransaction; HTTP only; always returns ErrEVMNotSupported, as rpc/AGENTS.md:12 states; 3 call site(s) in 1 file(s) |
| <code>interface.7.rpc-debug-getrawtransaction</code> | overridden_by | override.J38.14 |

Contract context <code>contract_context.7.tracing</code>:

| Field | Frozen rule |
|---|---|
| interface | debug_getRawTransaction returns the envelope; debug_getRawHeader, debug_getRawBlock and debug_getRawReceipts stay refused (decision 703); tracer outputs follow geth shapes. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | debug_traceBlockByNumber returns one entry per EVM transaction in block order. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.765</code> | title | decision.703 binds: debug raw header, block and receipts |
| <code>resolution.17.765</code> | applies_to | decision.703<br>task.7.18.1 |
| <code>resolution.17.765</code> | source | decision.703 in fragments/J38/decision.kvx; tasks from affects |
| <code>resolution.17.771</code> | title | decision.709 binds: rpc/AGENTS.md |
| <code>resolution.17.771</code> | applies_to | decision.709<br>task.7.18.1 |
| <code>resolution.17.771</code> | source | decision.709 in fragments/J38/decision.kvx; tasks from task ids cited in the decision value |

## debug-traceblockbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-traceblockbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:667</code> | <code>func (api *DebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-traceblockbyhash</code> | notes | debug_traceBlockByHash; HTTP only; 1 call site(s) in 1 file(s) |

## debug-traceblockbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-traceblockbynumber</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:639</code> | <code>func (api *DebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-traceblockbynumber</code> | notes | debug_traceBlockByNumber; HTTP only; 16 call site(s) in 14 file(s) |

## debug-tracecall

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-tracecall</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:695</code> | <code>func (api *DebugAPI) TraceCall(ctx context.Context, args export.TransactionArgs, blockNrOrHash rpc.BlockNumberOrHash, config *tracers.TraceCallConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-tracecall</code> | notes | debug_traceCall; HTTP only; 1 call site(s) in 1 file(s) |

## debug-tracestateaccess

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-tracestateaccess</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:753</code> | <code>func (api *DebugAPI) TraceStateAccess(ctx context.Context, hash common.Hash) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-tracestateaccess</code> | notes | debug_traceStateAccess; HTTP only; 3 call site(s) in 2 file(s) |

## debug-tracetransaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-tracetransaction</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:282</code> | <code>func (api *DebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-tracetransaction</code> | notes | debug_traceTransaction; HTTP only; 40 call site(s) in 24 file(s) |

## debug-tracetransactionprofile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-debug-tracetransactionprofile</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/trace_profile.go:45</code> | <code>func (api *DebugAPI) TraceTransactionProfile(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-debug-tracetransactionprofile</code> | notes | debug_traceTransactionProfile; HTTP only; 2 call site(s) in 1 file(s) |

## echo-echo

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-echo-echo</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/echo.go:9</code> | <code>func (a *EchoAPI) Echo(data string) string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-echo-echo</code> | notes | echo_echo; HTTP and WS; 1 call site(s) in 1 file(s) |

## error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.err-evm-not-supported-32000-j38</code> | <code>error_code</code> | <code>stable</code> | <code>rpc/rpc_unsupported.go:3</code> | <code>// ErrCodeEVMNotSupported is the JSON-RPC error code (-32000) for EVM RPC methods that</code> |
| <code>interface.7.err-pax-legacy-not-enabled-32601-j38</code> | <code>error_code</code> | <code>stable</code> | <code>rpc/pax_legacy_http.go:19</code> | <code>paxLegacyNotEnabled = -32601</code> |
| <code>interface.7.err-rate-limited-32005-j38</code> | <code>error_code</code> | <code>new</code> | <code>rpc/ip_rate_limit.go</code> (planned) | <code>-32005 rate limit exceeded (HTTP 429)</code> |
| <code>interface.7.err-revert-3-j38</code> | <code>error_code</code> | <code>stable</code> | <code>rpc/simulate.go:210</code> | <code>func (e *RevertError) ErrorCode() int  // 3</code> |
| <code>interface.7.err-send-sync-timeout-4-j38</code> | <code>error_code</code> | <code>new</code> | <code>rpc/send_sync.go</code> (planned) | <code>4 transaction not yet included (data: transaction hash)</code> |
| <code>interface.7.err-sequencer-not-served-32000-j38</code> | <code>error_code</code> | <code>stable</code> | <code>rpc/sequencer_receipt.go:49</code> | <code>sequencerNotServedCode = -32000</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.err-evm-not-supported-32000-j38</code> | notes | Answer of methods registered but refused, including debug_getRawHeader, debug_getRawBlock and debug_getRawReceipts (decision 703) |
| <code>interface.7.err-pax-legacy-not-enabled-32601-j38</code> | notes | HTTP answer for a pax_ or pax2_ name not enabled or not gated (rpc/pax_legacy.go fail-closed branch) |
| <code>interface.7.err-rate-limited-32005-j38</code> | notes | Answered before dispatch when a remote address exceeds evm.ip_rate_limit_burst (decision 711); task 7.13.4 |
| <code>interface.7.err-revert-3-j38</code> | notes | eth_call and eth_estimateGas revert answer with the revert data |
| <code>interface.7.err-send-sync-timeout-4-j38</code> | notes | eth_sendRawTransactionSync timeout answer (decision 702); task 7.8.2 |
| <code>interface.7.err-sequencer-not-served-32000-j38</code> | notes | pax_sequencerReceipt answer when the node holds no sequencer receipt source |

Contract context <code>contract_context.7.error-codes</code>:

| Field | Frozen rule |
|---|---|
| interface | Error codes: -32000 unsupported or not served, -32601 method not found or pax not enabled, 3 revert, -32005 rate limited, 4 sync send timeout. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Each code is produced by exactly one documented path. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.transport-limits</code>:

| Field | Frozen rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size (decisions 711 and 720). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.20.1</code> | req | 7020.1 |
| <code>acceptance.7.20.1</code> | scenario | Given the error producers of the package, when each refusal is produced through its real path, then the codes are -32000, -32601, -32000, 3, -32005 and 4 |
| <code>acceptance.7.20.1</code> | test | rpc/error_codes_test.go::TestErrorCodeRegistry (planned) |
| <code>acceptance.7.20.1</code> | qualification | UNRUN |
| <code>req.7020</code> | ac_1 | The node answers -32000 for unsupported methods (rpc/rpc_unsupported.go:5), -32601 for disabled pax methods (rpc/pax_legacy_http.go:19), 3 for reverts, -32005 for rate limits and 4 for a sync send timeout. |
| <code>acceptance.7.8.2</code> | req | 7008.2 |
| <code>acceptance.7.8.2</code> | scenario | Given the HTTP server of the package test setup and a signed transfer whose receipt is never stored, when eth_sendRawTransactionSync is called with timeoutMs 200, then within one second the answer carries code 4 and data equal to the transfer hash |
| <code>acceptance.7.8.2</code> | test | rpc/send_sync_timeout_test.go::TestSendRawTransactionSyncTimeoutReturnsHash (planned) |
| <code>acceptance.7.8.2</code> | qualification | UNRUN |
| <code>req.7008</code> | ac_2 | When no receipt appears within the timeout, eth_sendRawTransactionSync (planned) answers error code 4 with the transaction hash as data, after the send of rpc/send.go:164 succeeded. |

## eth-accounts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-accounts</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:82</code> | <code>func (i *InfoAPI) Accounts(ctx context.Context) (result []common.Address, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-accounts</code> | notes | eth_accounts; HTTP and WS; keyring path enabled by evm.enable_unsafe_keyring_rpc, refused at startup on a live chain (rpc/server.go:104); 89 call site(s) in 50 file(s) |

## eth-blobbasefee

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-blobbasefee</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/info.go:336</code> | <code>func (i *InfoAPI) BlobBaseFee(ctx context.Context) (result *hexutil.Big, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-blobbasefee</code> | notes | eth_blobBaseFee; HTTP and WS; returns blobs not supported; the metric endpoint label is eth_BlobBaseFee (rpc/info.go:339), not the method name eth_blobBaseFee used by every other handler; 2 call site(s) in 1 file(s) |

Contract context <code>contract_context.7.metrics</code>:

| Field | Frozen rule |
|---|---|
| interface | Request latency is recorded under the served method name on the evmrpc meter; the legacy request counter, latency and websocket connect emission from rpc/ stops (decision 718). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Every served method records one latency point per call under its own name. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-blocknumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-blocknumber</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:52</code> | <code>func (i *InfoAPI) BlockNumber(ctx context.Context) (result hexutil.Uint64, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-blocknumber</code> | notes | eth_blockNumber; HTTP and WS; 202 call site(s) in 116 file(s), first site per file listed, 69 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-calculategasusedratio

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-calculategasusedratio</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/info.go:547</code> | <code>func (i *InfoAPI) CalculateGasUsedRatio(ctx context.Context, blockHeight int64) (float64, error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-calculategasusedratio</code> | notes | eth_calculateGasUsedRatio; HTTP and WS; InfoAPI.CalculateGasUsedRatio is the feeHistory helper (rpc/info.go:546-547) and is registered on HTTP and WS only because it is exported; no in-repo client calls it; no in-repo caller |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-call</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/simulate.go:151</code> | <code>func (s *SimulationAPI) Call(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride, blockOverrides *export.BlockOverrides) (result hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-call</code> | notes | eth_call; HTTP and WS; 936 call site(s) in 276 file(s), first site per file listed, 218 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-chainid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-chainid</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:68</code> | <code>func (i *InfoAPI) ChainId(ctx context.Context) *hexutil.Big</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-chainid</code> | notes | eth_chainId; HTTP and WS; 345 call site(s) in 193 file(s), first site per file listed, 131 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-cleanup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-cleanup</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/filter.go:1206</code> | <code>func (a *FilterAPI) Cleanup()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-cleanup</code> | notes | eth_cleanup; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); FilterAPI.Cleanup is exported so the server registers it on HTTP; one client call cancels every filter and closes toDelete (rpc/filter.go:1206-1219), a second call panics on the closed channel; it is a shutdown hook, not a client method; no in-repo caller |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.778</code> | title | decision.716 binds: Accidental exports |
| <code>resolution.17.778</code> | applies_to | decision.716<br>task.7.2.1<br>task.7.2.2<br>task.7.2.3 |
| <code>resolution.17.778</code> | source | decision.716 in fragments/J38/decision.kvx; tasks from affects |

## eth-coinbase

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-coinbase</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:74</code> | <code>func (i *InfoAPI) Coinbase(ctx context.Context) (addr common.Address, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-coinbase</code> | notes | eth_coinbase; HTTP and WS; 21 call site(s) in 7 file(s) |

## eth-createaccesslist

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-createaccesslist</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/simulate.go:86</code> | <code>func (s *SimulationAPI) CreateAccessList(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash) (result *AccessListResult, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-createaccesslist</code> | notes | eth_createAccessList; HTTP and WS; 5 call site(s) in 5 file(s) |

## eth-estimategas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-estimategas</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/simulate.go:107</code> | <code>func (s *SimulationAPI) EstimateGas(ctx context.Context, args export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride) (result hexutil.Uint64, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-estimategas</code> | notes | eth_estimateGas; HTTP and WS; 88 call site(s) in 54 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-estimategasaftercalls

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-estimategasaftercalls</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/simulate.go:129</code> | <code>func (s *SimulationAPI) EstimateGasAfterCalls(ctx context.Context, args export.TransactionArgs, calls []export.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *export.StateOverride) (result hexutil.Uint64, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-estimategasaftercalls</code> | notes | eth_estimateGasAfterCalls; HTTP and WS; 1 call site(s) in 1 file(s) |

## eth-feehistory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-feehistory</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:148</code> | <code>func (i *InfoAPI) FeeHistory(ctx context.Context, blockCount gmath.HexOrDecimal64, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (result *FeeHistoryResult, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-feehistory</code> | notes | eth_feeHistory; HTTP and WS; 31 call site(s) in 7 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gasprice

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gasprice</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:100</code> | <code>func (i *InfoAPI) GasPrice(ctx context.Context) (result *hexutil.Big, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gasprice</code> | notes | eth_gasPrice; HTTP and WS; 56 call site(s) in 42 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gaspricehelper

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gaspricehelper</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/info.go:130</code> | <code>func (i *InfoAPI) GasPriceHelper(ctx context.Context, baseFee *big.Int, totalGasUsedPrevBlock uint64, medianRewardPrevBlock *big.Int) (*hexutil.Big, error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gaspricehelper</code> | notes | eth_gasPriceHelper; HTTP and WS; InfoAPI.GasPriceHelper is commented as a test helper (rpc/info.go:129) and is registered on HTTP and WS only because it is exported; no in-repo client calls it; no in-repo caller |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getbalance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getbalance</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/state.go:49</code> | <code>func (a *StateAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Big, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getbalance</code> | notes | eth_getBalance; HTTP and WS; 153 call site(s) in 84 file(s), first site per file listed, 35 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getblockbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getblockbyhash</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:176</code> | <code>func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getblockbyhash</code> | notes | eth_getBlockByHash; HTTP and WS; 82 call site(s) in 39 file(s) |
| <code>interface.7.rpc-eth-getblockbyhash</code> | overridden_by | override.J38.28 |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.height-availability</code>:

| Field | Frozen rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getblockbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getblockbynumber</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:210</code> | <code>func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getblockbynumber</code> | notes | eth_getBlockByNumber; HTTP and WS; 497 call site(s) in 240 file(s), first site per file listed, 190 files beyond the line cap are in inventory/J03.md |
| <code>interface.7.rpc-eth-getblockbynumber</code> | overridden_by | override.J38.29 |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.height-availability</code>:

| Field | Frozen rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.pending-block</code>:

| Field | Frozen rule |
|---|---|
| interface | On a sequencer the pending tag answers from the pending snapshot: header fields, transaction count and transaction by index; without a pending source the HEAD answers stay. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.769</code> | title | decision.707 binds: Pending block field set |
| <code>resolution.17.769</code> | applies_to | decision.707<br>task.7.11.1<br>task.7.11.2<br>task.7.11.3 |
| <code>resolution.17.769</code> | source | decision.707 in fragments/J38/decision.kvx; tasks from affects |

## eth-getblockreceipts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getblockreceipts</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:344</code> | <code>func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getblockreceipts</code> | notes | eth_getBlockReceipts; HTTP and WS; 51 call site(s) in 22 file(s) |

## eth-getblocktransactioncountbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getblocktransactioncountbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:154</code> | <code>func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getblocktransactioncountbyhash</code> | notes | eth_getBlockTransactionCountByHash; HTTP and WS; 11 call site(s) in 4 file(s) |

## eth-getblocktransactioncountbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getblocktransactioncountbynumber</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:128</code> | <code>func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getblocktransactioncountbynumber</code> | notes | eth_getBlockTransactionCountByNumber; HTTP and WS; 17 call site(s) in 6 file(s) |
| <code>interface.7.rpc-eth-getblocktransactioncountbynumber</code> | overridden_by | override.J38.23 |

Contract context <code>contract_context.7.pending-block</code>:

| Field | Frozen rule |
|---|---|
| interface | On a sequencer the pending tag answers from the pending snapshot: header fields, transaction count and transaction by index; without a pending source the HEAD answers stay. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getcode

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getcode</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/state.go:66</code> | <code>func (a *StateAPI) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getcode</code> | notes | eth_getCode; HTTP and WS; 191 call site(s) in 79 file(s), first site per file listed, 29 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getfilterchanges

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getfilterchanges</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:876</code> | <code>func (a *FilterAPI) GetFilterChanges(ctx context.Context, filterID ethrpc.ID) (res interface{}, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getfilterchanges</code> | notes | eth_getFilterChanges; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); 22 call site(s) in 6 file(s) |

## eth-getfilterlogs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getfilterlogs</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:989</code> | <code>func (a *FilterAPI) GetFilterLogs(ctx context.Context, filterID ethrpc.ID) (res []*FilterLog, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getfilterlogs</code> | notes | eth_getFilterLogs; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); 21 call site(s) in 7 file(s) |

## eth-getlogs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getlogs</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/filter.go:1045</code> | <code>func (a *FilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getlogs</code> | notes | eth_getLogs; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); 202 call site(s) in 84 file(s), first site per file listed, 34 files beyond the line cap are in inventory/J03.md |
| <code>interface.7.rpc-eth-getlogs</code> | overridden_by | override.J38.21 |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.filters</code>:

| Field | Frozen rule |
|---|---|
| interface | eth_getLogs fails when the earliest height cannot be read and answers unknown block for an unknown or zero blockHash (decision 719). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A log query never scans from height 0 because of a store read failure. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.781</code> | title | decision.719 binds: Unknown blockHash in eth_getLogs |
| <code>resolution.17.781</code> | applies_to | decision.719<br>task.7.17.2 |
| <code>resolution.17.781</code> | source | decision.719 in fragments/J38/decision.kvx; tasks from affects |

## eth-getnonce

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getnonce</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/state.go:229</code> | <code>func (a *StateAPI) GetNonce(ctx context.Context, address common.Address) uint64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getnonce</code> | notes | eth_getNonce; HTTP and WS; no in-repo caller |

## eth-getproof

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getproof</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/state.go:124</code> | <code>func (a *StateAPI) GetProof(ctx context.Context, address common.Address, storageKeys []string, blockNrOrHash rpc.BlockNumberOrHash) (result *ProofResult, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getproof</code> | notes | eth_getProof; HTTP and WS; 11 call site(s) in 10 file(s) |

## eth-getstorageat

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getstorageat</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/state.go:83</code> | <code>func (a *StateAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (result hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getstorageat</code> | notes | eth_getStorageAt; HTTP and WS; 73 call site(s) in 29 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactionbyblockhashandindex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyblockhashandindex</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:242</code> | <code>func (t *TransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyblockhashandindex</code> | notes | eth_getTransactionByBlockHashAndIndex; HTTP and WS; 14 call site(s) in 6 file(s) |

## eth-gettransactionbyblocknumberandindex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyblocknumberandindex</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/tx.go:208</code> | <code>func (t *TransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyblocknumberandindex</code> | notes | eth_getTransactionByBlockNumberAndIndex; HTTP and WS; 10 call site(s) in 5 file(s) |
| <code>interface.7.rpc-eth-gettransactionbyblocknumberandindex</code> | overridden_by | override.J38.26 |

Contract context <code>contract_context.7.pending-block</code>:

| Field | Frozen rule |
|---|---|
| interface | On a sequencer the pending tag answers from the pending snapshot: header fields, transaction count and transaction by index; without a pending source the HEAD answers stay. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactionbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:269</code> | <code>func (t *TransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (result *export.RPCTransaction, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gettransactionbyhash</code> | notes | eth_getTransactionByHash; HTTP and WS; 168 call site(s) in 90 file(s), first site per file listed, 52 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactioncount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactioncount</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:397</code> | <code>func (t *TransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Uint64, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gettransactioncount</code> | notes | eth_getTransactionCount; HTTP and WS; 178 call site(s) in 81 file(s), first site per file listed, 27 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-gettransactionerrorbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionerrorbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:379</code> | <code>func (t *TransactionAPI) GetTransactionErrorByHash(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gettransactionerrorbyhash</code> | notes | eth_getTransactionErrorByHash; HTTP and WS; 2 call site(s) in 2 file(s) |

## eth-gettransactionreceipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-gettransactionreceipt</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:106</code> | <code>func (t *TransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-gettransactionreceipt</code> | notes | eth_getTransactionReceipt; HTTP and WS; 262 call site(s) in 162 file(s), first site per file listed, 116 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-getvmerror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-getvmerror</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:193</code> | <code>func (t *TransactionAPI) GetVMError(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-getvmerror</code> | notes | eth_getVMError; HTTP and WS; 7 call site(s) in 4 file(s) |

## eth-maxpriorityfeepergas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-maxpriorityfeepergas</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:304</code> | <code>func (i *InfoAPI) MaxPriorityFeePerGas(ctx context.Context) (fee *hexutil.Big, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-maxpriorityfeepergas</code> | notes | eth_maxPriorityFeePerGas; HTTP and WS; 38 call site(s) in 27 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-newblockfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-newblockfilter</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:814</code> | <code>func (a *FilterAPI) NewBlockFilter(ctx context.Context) (id ethrpc.ID, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-newblockfilter</code> | notes | eth_newBlockFilter; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); 15 call site(s) in 6 file(s) |

## eth-newfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-newfilter</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:767</code> | <code>func (a *FilterAPI) NewFilter(ctx context.Context, crit filters.FilterCriteria) (id ethrpc.ID, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-newfilter</code> | notes | eth_newFilter; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); 23 call site(s) in 9 file(s) |

## eth-newpendingtransactionfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-newpendingtransactionfilter</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:840</code> | <code>func (a *FilterAPI) NewPendingTransactionFilter(ctx context.Context, fullTx *bool) (id ethrpc.ID, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-newpendingtransactionfilter</code> | notes | eth_newPendingTransactionFilter; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); served on the sequencer and standby from the receipt-log feed and refused with ErrEVMNotSupported elsewhere (rpc/filter.go:838-850), while rpc/AGENTS.md:13 still lists it as unsupported everywhere; 3 call site(s) in 1 file(s) |
| <code>interface.7.rpc-eth-newpendingtransactionfilter</code> | overridden_by | override.J38.15 |

## eth-sendrawtransaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-sendrawtransaction</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/send.go:164</code> | <code>func (s *SendAPI) SendRawTransaction(ctx context.Context, input hexutil.Bytes) (hash common.Hash, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-sendrawtransaction</code> | notes | eth_sendRawTransaction; HTTP and WS; 173 call site(s) in 87 file(s), first site per file listed, 33 files beyond the line cap are in inventory/J03.md |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-sendtransaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-sendtransaction</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/send.go:458</code> | <code>func (s *SendAPI) SendTransaction(ctx context.Context, args export.TransactionArgs) (result common.Hash, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-sendtransaction</code> | notes | eth_sendTransaction; HTTP and WS; keyring path enabled by evm.enable_unsafe_keyring_rpc, refused at startup on a live chain (rpc/server.go:104); 109 call site(s) in 63 file(s), first site per file listed, 4 files beyond the line cap are in inventory/J03.md |

## eth-setreceiptlogfeed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-setreceiptlogfeed</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/filter.go:583</code> | <code>func (a *FilterAPI) SetReceiptLogFeed(feed *ReceiptLogFeed)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-setreceiptlogfeed</code> | notes | eth_setReceiptLogFeed; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); FilterAPI.SetReceiptLogFeed is an internal setter (rpc/filter.go:580-585) exported and therefore registered on HTTP; a client call with null clears the sequencer receipt-log feed so pending filters and pending logs stop being served; no in-repo caller |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-sign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-sign</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:530</code> | <code>func (t *TransactionAPI) Sign(ctx context.Context, addr common.Address, data hexutil.Bytes) (result hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-sign</code> | notes | eth_sign; HTTP and WS; keyring path enabled by evm.enable_unsafe_keyring_rpc, refused at startup on a live chain (rpc/server.go:104); 57 call site(s) in 22 file(s) |

## eth-signtransaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-signtransaction</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/send.go:435</code> | <code>func (s *SendAPI) SignTransaction(ctx context.Context, args apitypes.SendTxArgs, _ *string) (result *export.SignTransactionResult, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-signtransaction</code> | notes | eth_signTransaction; HTTP and WS; keyring path enabled by evm.enable_unsafe_keyring_rpc, refused at startup on a live chain (rpc/server.go:104); 14 call site(s) in 12 file(s) |

## eth-subscribe

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-subscribe</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>core/deps/geth/rpc/json.go:35</code> | <code>subscribeMethodSuffix    = "_subscribe"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-subscribe</code> | notes | eth_subscribe; server built-in for every namespace that registers a subscription; eth on WS serves newHeads, logs and newPendingTransactions; on HTTP the call fails with notifications not supported; core/deps is not in the domain table, rpc-api owns the served surface; 36 call site(s) in 17 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-syncing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-syncing</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/info.go:353</code> | <code>func (i *InfoAPI) Syncing(ctx context.Context) (result any, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-syncing</code> | notes | eth_syncing; HTTP and WS; 20 call site(s) in 7 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## eth-uninstallfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-uninstallfilter</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:1176</code> | <code>func (a *FilterAPI) UninstallFilter(ctx context.Context, filterID ethrpc.ID) (res bool)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-uninstallfilter</code> | notes | eth_uninstallFilter; HTTP only; FilterAPI is not registered on WS (rpc/server.go:392-433); 38 call site(s) in 9 file(s) |

## eth-unsubscribe

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-eth-unsubscribe</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>core/deps/geth/rpc/json.go:36</code> | <code>unsubscribeMethodSuffix  = "_unsubscribe"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-eth-unsubscribe</code> | notes | eth_unsubscribe; server built-in paired with eth_subscribe; WS only in practice; core/deps is not in the domain table, rpc-api owns the served surface; 17 call site(s) in 9 file(s) |

## filters

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.17.1</code> | req | 7017.1 |
| <code>acceptance.7.17.1</code> | scenario | Given a FilterAPI whose watermarks cannot report the earliest height, when GetLogs is called over fromBlock 0x1 to latest, then it returns an error naming the earliest height |
| <code>acceptance.7.17.1</code> | test | rpc/filter_earliest_test.go::TestGetLogsFailsWhenEarliestHeightUnavailable (planned) |
| <code>acceptance.7.17.1</code> | qualification | UNRUN |
| <code>req.7017</code> | ac_1 | eth_getLogs returns an error when the earliest available height cannot be read, instead of scanning from height 0 as the fallback at rpc/filter.go:1076 does. |
| <code>acceptance.7.17.2</code> | req | 7017.2 |
| <code>acceptance.7.17.2</code> | scenario | Given the HTTP server of the package test setup, when eth_getLogs is called with an unknown blockHash and with the zero hash, then both answers are errors whose message is unknown block |
| <code>acceptance.7.17.2</code> | test | rpc/filter_blockhash_test.go::TestGetLogsUnknownBlockHashErrors (planned) |
| <code>acceptance.7.17.2</code> | qualification | UNRUN |
| <code>req.7017</code> | ac_2 | eth_getLogs with a blockHash that is zero or not found answers the error unknown block instead of the empty list returned at rpc/filter.go:1677. |

## height-availability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.go-pacific1evmlaunchheight-j38</code> | <code>go_exported</code> | <code>retire</code> | <code>rpc/utils.go:74</code> | <code>const Pacific1EVMLaunchHeight int64 = 79123881</code> |
| <code>interface.7.go-validateevmblockheight-j38</code> | <code>go_exported</code> | <code>retire</code> | <code>rpc/utils.go:254</code> | <code>func ValidateEVMBlockHeight(chainID string, blockHeight int64) error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.go-pacific1evmlaunchheight-j38</code> | notes | Constant used only by ValidateEVMBlockHeight; removed in task 7.15.1; 0 use(s) outside rpc/ |
| <code>interface.7.go-validateevmblockheight-j38</code> | notes | Launch height floor for chain id pacific-1; removed in task 7.15.1; 0 use(s) outside rpc/ |

Contract context <code>contract_context.7.height-availability</code>:

| Field | Frozen rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.15.1</code> | req | 7015.1 |
| <code>acceptance.7.15.1</code> | scenario | Given the package test server whose context reports chain id pacific-1, when eth_getBlockByNumber is called for 0x2 and eth_getBlockByHash for its hash, then both return the block |
| <code>acceptance.7.15.1</code> | test | rpc/block_launch_height_test.go::TestBlockLookupHasNoLaunchHeightFloor (planned) |
| <code>acceptance.7.15.1</code> | qualification | UNRUN |
| <code>req.7015</code> | ac_1 | Block lookups no longer refuse heights below 79123881 on chain id pacific-1, the floor applied at rpc/utils.go:254 for a chain this network never runs. |
| <code>acceptance.7.15.2</code> | req | 7015.2 |
| <code>acceptance.7.15.2</code> | scenario | Given the package test server whose context reports chain id pacific-1 and a contract returning BASEFEE, when eth_call runs it at latest, then the result equals GetNextBaseFeePerGas of the previous height |
| <code>acceptance.7.15.2</code> | test | rpc/simulate_basefee_test.go::TestSimulateHeaderKeepsBaseFee (planned) |
| <code>acceptance.7.15.2</code> | qualification | UNRUN |
| <code>req.7015</code> | ac_2 | Simulated headers keep baseFee on every chain id; the pacific-1 branch at rpc/simulate.go:881 is removed. |

## http-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.http-jsonrpc-root</code> | <code>http_route</code> | <code>change</code> | <code>rpc/rpcstack.go:243</code> | <code>func CheckPath(r *http.Request, path string) bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.http-jsonrpc-root</code> | notes | HTTP JSON-RPC on evm.http_address:evm.http_port (rpc/server.go:136) at the handler prefix, other paths 404 (rpc/rpcstack.go:211-239); CORS from evm.cors_origins (rpc/server.go:304), deny list from evm.deny_list, legacy pax gate and the Pax-Legacy-RPC-Deprecation header wrapper (rpc/pax_legacy_http.go:27) |
| <code>interface.7.http-jsonrpc-root</code> | overridden_by | override.J38.16 |

Contract context <code>contract_context.7.transport-limits</code>:

| Field | Frozen rule |
|---|---|
| interface | Per-IP token bucket keyed by RemoteAddr answering HTTP 429 with -32005, and batch item and response limits from evm.batch_request_limit and evm.batch_response_max_size (decisions 711 and 720). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | rps 0 disables the limiter; limits 0 disable batch bounds. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## lx-switch-off

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.16.1</code> | req | 7016.1 |
| <code>acceptance.7.16.1</code> | scenario | Given the HTTP server of the package test setup, when each of the 26 lx_* names is called, then each answer carries -32601 and none is in HTTPMethods |
| <code>acceptance.7.16.1</code> | test | rpc/lx_switch_off_test.go::TestLxMethodsNotServedHTTP (planned) |
| <code>acceptance.7.16.1</code> | qualification | UNRUN |
| <code>req.7016</code> | ac_1 | None of the 26 lx_* names of spec/layerx-merge/contracts/clients.md:197-222 is served by the node over HTTP; each answers -32601. |
| <code>acceptance.7.16.2</code> | req | 7016.2 |
| <code>acceptance.7.16.2</code> | scenario | Given the WebSocket server of the package test setup, when each of the 26 lx_* names is sent, then each answer carries -32601 and none is in WSMethods |
| <code>acceptance.7.16.2</code> | test | rpc/lx_switch_off_test.go::TestLxMethodsNotServedWS (planned) |
| <code>acceptance.7.16.2</code> | qualification | UNRUN |
| <code>req.7016</code> | ac_2 | None of the 26 lx_* names of spec/layerx-merge/contracts/clients.md:197-222 is served by the node over WebSocket; each answers -32601. |

## metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.env-evm-debug-metrics-j38</code> | <code>env_var</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:19</code> | <code>EVM_DEBUG_METRICS=true</code> |
| <code>interface.7.met-evmrpc-trace-bake-baked-total-j38</code> | <code>metric</code> | <code>new</code> | <code>rpc/metrics.go</code> (planned) | <code>evmrpc_trace_bake_baked_total Int64ObservableCounter</code> |
| <code>interface.7.met-evmrpc-trace-bake-dropped-total-j38</code> | <code>metric</code> | <code>new</code> | <code>rpc/metrics.go</code> (planned) | <code>evmrpc_trace_bake_dropped_total Int64ObservableCounter</code> |
| <code>interface.7.met-evmrpc-trace-bake-failed-total-j38</code> | <code>metric</code> | <code>new</code> | <code>rpc/metrics.go</code> (planned) | <code>evmrpc_trace_bake_failed_total Int64ObservableCounter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.env-evm-debug-metrics-j38</code> | notes | Enables periodic stdout printing of the worker pool metrics; Prometheus export is unconditional |
| <code>interface.7.met-evmrpc-trace-bake-baked-total-j38</code> | notes | Exports TraceBaker.BakedCount on the evmrpc meter; task 7.14.2 |
| <code>interface.7.met-evmrpc-trace-bake-dropped-total-j38</code> | notes | Exports TraceBaker.DroppedCount on the evmrpc meter; task 7.14.2 |
| <code>interface.7.met-evmrpc-trace-bake-failed-total-j38</code> | notes | Exports TraceBaker.FailedCount on the evmrpc meter; task 7.14.2 |

Contract context <code>contract_context.7.metrics</code>:

| Field | Frozen rule |
|---|---|
| interface | Request latency is recorded under the served method name on the evmrpc meter; the legacy request counter, latency and websocket connect emission from rpc/ stops (decision 718). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Every served method records one latency point per call under its own name. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.14.1</code> | req | 7014.1 |
| <code>acceptance.7.14.1</code> | scenario | Given a manual OTel reader and the HTTP server of the package test setup, when the seven InfoAPI methods are called once, then the latency histogram endpoint labels equal the seven served names |
| <code>acceptance.7.14.1</code> | test | rpc/info_metric_labels_test.go::TestInfoAPIMetricLabelsUseServedNames (planned) |
| <code>acceptance.7.14.1</code> | qualification | UNRUN |
| <code>req.7014</code> | ac_1 | The InfoAPI records eth_blockNumber, eth_chainId, eth_coinbase, eth_accounts, eth_gasPrice, eth_blobBaseFee and eth_syncing instead of the capitalised labels at rpc/info.go:55 and rpc/info.go:339. |
| <code>acceptance.7.14.2</code> | req | 7014.2 |
| <code>acceptance.7.14.2</code> | scenario | Given a meter provider with a manual reader and a baker that baked two blocks and dropped one, when the reader collects, then the three counters read 1 dropped, 2 baked and 0 failed |
| <code>acceptance.7.14.2</code> | test | rpc/trace_baker_metrics_test.go::TestTraceBakerCountersExported (planned) |
| <code>acceptance.7.14.2</code> | qualification | UNRUN |
| <code>req.7014</code> | ac_2 | The trace baker counters, readable only through DroppedCount, BakedCount and FailedCount at rpc/trace_baker.go:153, are exported as evmrpc_trace_bake_dropped_total, evmrpc_trace_bake_baked_total and evmrpc_trace_bake_failed_total. |
| <code>acceptance.7.14.3</code> | req | 7014.3 |
| <code>acceptance.7.14.3</code> | scenario | Given a manual OTel reader and the legacy counter value before the dial, when one WebSocket connection is opened to the test server, then evmrpc_websocket_connects_total rises by one and the legacy counter is unchanged |
| <code>acceptance.7.14.3</code> | test | rpc/websockets_metrics_test.go::TestWSConnectDoesNotEmitLegacyCounter (planned) |
| <code>acceptance.7.14.3</code> | qualification | UNRUN |
| <code>req.7014</code> | ac_3 | A WebSocket connection is counted only by evmrpc_websocket_connects_total; the legacy IncWebsocketConnects call at rpc/websockets.go:16 is gone. |
| <code>acceptance.7.14.4</code> | req | 7014.4 |
| <code>acceptance.7.14.4</code> | scenario | Given a manual OTel reader and the legacy request counter value before the call, when recordMetricsWithError runs for eth_chainId, then the histogram gains one point and the legacy counter is unchanged |
| <code>acceptance.7.14.4</code> | test | rpc/utils_metrics_test.go::TestRecordMetricsDoesNotEmitLegacyRequestMetrics (planned) |
| <code>acceptance.7.14.4</code> | qualification | UNRUN |
| <code>req.7014</code> | ac_4 | recordMetricsWithError (rpc/utils.go:483) records only the OTel latency histogram; the legacy request counter and latency calls at rpc/utils.go:493 are gone. |
| <code>acceptance.7.14.5</code> | req | 7014.5 |
| <code>acceptance.7.14.5</code> | scenario | Given a manual OTel reader and the HTTP server of the package test setup, when pax_getTransactionByHash is called once, then the latency histogram has a point with endpoint pax_getTransactionByHash and none added for eth_getTransactionByHash |
| <code>acceptance.7.14.5</code> | test | rpc/tx_metric_labels_test.go::TestPaxTransactionMethodsRecordPaxLabels (planned) |
| <code>acceptance.7.14.5</code> | qualification | UNRUN |
| <code>req.7014</code> | ac_5 | pax_getTransactionByHash records its latency under endpoint pax_getTransactionByHash, not the eth_getTransactionByHash literal at rpc/tx.go:272. |
| <code>acceptance.7.14.6</code> | req | 7014.6 |
| <code>acceptance.7.14.6</code> | scenario | Given a manual OTel reader and the HTTP server of the package test setup, when every served eth_, net_, web3_, txpool_ and debug_ name is called once, then each name appears as an endpoint label |
| <code>acceptance.7.14.6</code> | test | rpc/served_labels_test.go::TestServedMethodsRecordServedEndpointLabel (planned) |
| <code>acceptance.7.14.6</code> | qualification | UNRUN |
| <code>req.7014</code> | ac_6 | Every served eth_, net_, web3_, txpool_ and debug_ method records evmrpc_request_latency_seconds (rpc/metrics.go:49) under its served name. |

## metrics-legacy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.met-pax-evm-subscriptions-errors</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:553</code> | <code>telemetry.IncrCounter(1, "pax", "evm", "subscriptions", "errors")</code> |
| <code>interface.7.met-pax-rpc-request-counter</code> | <code>metric</code> | <code>retire</code> | <code>utils/metrics/metrics_util.go:274</code> | <code>func IncrementRpcRequestCounter(endpoint string, connectionType string, success bool)</code> |
| <code>interface.7.met-pax-rpc-request-latency-ms</code> | <code>metric</code> | <code>retire</code> | <code>utils/metrics/metrics_util.go:291</code> | <code>func MeasureRpcRequestLatency(endpoint string, connectionType string, startTime time.Time)</code> |
| <code>interface.7.met-pax-websocket-connect</code> | <code>metric</code> | <code>retire</code> | <code>utils/metrics/metrics_util.go:265</code> | <code>func IncWebsocketConnects()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.met-pax-evm-subscriptions-errors</code> | notes | pax_evm_subscriptions_errors legacy telemetry counter incremented by RecordSubscriptionError (rpc/worker_pool_metrics.go:551) |
| <code>interface.7.met-pax-rpc-request-counter</code> | notes | pax_rpc_request_counter; legacy telemetry dual-emitted beside the evmrpc OTel metrics; a TODO above the helper marks it for removal once dashboards use evmrpc_* and no dashboard in the repository reads it; utils/ is not in the domain table, the only emitter is rpc/ |
| <code>interface.7.met-pax-rpc-request-latency-ms</code> | notes | pax_rpc_request_latency_ms; legacy telemetry dual-emitted beside the evmrpc OTel metrics; a TODO above the helper marks it for removal once dashboards use evmrpc_* and no dashboard in the repository reads it; utils/ is not in the domain table, the only emitter is rpc/ |
| <code>interface.7.met-pax-websocket-connect</code> | notes | pax_websocket_connect; legacy telemetry dual-emitted beside the evmrpc OTel metrics; a TODO above the helper marks it for removal once dashboards use evmrpc_* and no dashboard in the repository reads it; utils/ is not in the domain table, the only emitter is rpc/ |

Contract context <code>contract_context.7.metrics</code>:

| Field | Frozen rule |
|---|---|
| interface | Request latency is recorded under the served method name on the evmrpc meter; the legacy request counter, latency and websocket connect emission from rpc/ stops (decision 718). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Every served method records one latency point per call under its own name. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.780</code> | title | decision.718 binds: Legacy metrics |
| <code>resolution.17.780</code> | applies_to | decision.718<br>task.7.14.3<br>task.7.14.4 |
| <code>resolution.17.780</code> | source | decision.718 in fragments/J38/decision.kvx; tasks from affects |

## metrics-workerpool

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.met-evmrpc-db-semaphore-available</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:211</code> | <code>Int64Gauge("evmrpc_db_semaphore_available")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-avg-wait-ms</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:291</code> | <code>Float64Gauge("evmrpc_db_semaphore_avg_wait_ms")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-capacity</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:201</code> | <code>Int64Gauge("evmrpc_db_semaphore_capacity")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-inuse</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:206</code> | <code>Int64Gauge("evmrpc_db_semaphore_inuse")</code> |
| <code>interface.7.met-evmrpc-db-semaphore-wait-count</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:216</code> | <code>Int64Gauge("evmrpc_db_semaphore_wait_count")</code> |
| <code>interface.7.met-evmrpc-getlogs-avg-blockrange</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:246</code> | <code>Float64Gauge("evmrpc_getlogs_avg_blockrange")</code> |
| <code>interface.7.met-evmrpc-getlogs-avg-latency-ms</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:256</code> | <code>Float64Gauge("evmrpc_getlogs_avg_latency_ms")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-backpressure</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:276</code> | <code>Int64Gauge("evmrpc_getlogs_errors_backpressure")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-range-too-large</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:266</code> | <code>Int64Gauge("evmrpc_getlogs_errors_range_too_large")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-rate-limited</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:271</code> | <code>Int64Gauge("evmrpc_getlogs_errors_rate_limited")</code> |
| <code>interface.7.met-evmrpc-getlogs-errors-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:236</code> | <code>Int64Gauge("evmrpc_getlogs_errors_total")</code> |
| <code>interface.7.met-evmrpc-getlogs-max-latency-ms</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:261</code> | <code>Float64Gauge("evmrpc_getlogs_max_latency_ms")</code> |
| <code>interface.7.met-evmrpc-getlogs-peak-blockrange</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:251</code> | <code>Int64Gauge("evmrpc_getlogs_peak_blockrange")</code> |
| <code>interface.7.met-evmrpc-getlogs-requests-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:226</code> | <code>Int64Gauge("evmrpc_getlogs_requests_total")</code> |
| <code>interface.7.met-evmrpc-getlogs-success-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:231</code> | <code>Int64Gauge("evmrpc_getlogs_success_total")</code> |
| <code>interface.7.met-evmrpc-getlogs-tps</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:241</code> | <code>Float64Gauge("evmrpc_getlogs_tps")</code> |
| <code>interface.7.met-evmrpc-subscriptions-active</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:221</code> | <code>Int64Gauge("evmrpc_subscriptions_active")</code> |
| <code>interface.7.met-evmrpc-workerpool-avg-exec-time-ms</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:286</code> | <code>Float64Gauge("evmrpc_workerpool_avg_exec_time_ms")</code> |
| <code>interface.7.met-evmrpc-workerpool-avg-queue-wait-ms</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:281</code> | <code>Float64Gauge("evmrpc_workerpool_avg_queue_wait_ms")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-capacity</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:161</code> | <code>Int64Gauge("evmrpc_workerpool_queue_capacity")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-depth</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:166</code> | <code>Int64Gauge("evmrpc_workerpool_queue_depth")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-peak</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:171</code> | <code>Int64Gauge("evmrpc_workerpool_queue_peak")</code> |
| <code>interface.7.met-evmrpc-workerpool-queue-utilization-pct</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:176</code> | <code>Float64Gauge("evmrpc_workerpool_queue_utilization_pct")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-completed-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:186</code> | <code>Int64Gauge("evmrpc_workerpool_tasks_completed_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-panicked-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:196</code> | <code>Int64Gauge("evmrpc_workerpool_tasks_panicked_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-rejected-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:191</code> | <code>Int64Gauge("evmrpc_workerpool_tasks_rejected_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-tasks-submitted-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:181</code> | <code>Int64Gauge("evmrpc_workerpool_tasks_submitted_total")</code> |
| <code>interface.7.met-evmrpc-workerpool-workers-active</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:151</code> | <code>Int64Gauge("evmrpc_workerpool_workers_active")</code> |
| <code>interface.7.met-evmrpc-workerpool-workers-idle</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:156</code> | <code>Int64Gauge("evmrpc_workerpool_workers_idle")</code> |
| <code>interface.7.met-evmrpc-workerpool-workers-total</code> | <code>metric</code> | <code>stable</code> | <code>rpc/worker_pool_metrics.go:146</code> | <code>Int64Gauge("evmrpc_workerpool_workers_total")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.met-evmrpc-db-semaphore-available</code> | notes | evmrpc_db_semaphore_available; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); DB semaphore available slots; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-db-semaphore-avg-wait-ms</code> | notes | evmrpc_db_semaphore_avg_wait_ms; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Average DB semaphore wait (ms); unit ms; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-db-semaphore-capacity</code> | notes | evmrpc_db_semaphore_capacity; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); DB semaphore capacity; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-db-semaphore-inuse</code> | notes | evmrpc_db_semaphore_inuse; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); DB semaphore currently acquired; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-db-semaphore-wait-count</code> | notes | evmrpc_db_semaphore_wait_count; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); DB semaphore wait count; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-getlogs-avg-blockrange</code> | notes | evmrpc_getlogs_avg_blockrange; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Average block range for eth_getLogs; unit {blocks}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-getlogs-avg-latency-ms</code> | notes | evmrpc_getlogs_avg_latency_ms; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Average eth_getLogs latency (ms); unit ms; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-getlogs-errors-backpressure</code> | notes | evmrpc_getlogs_errors_backpressure; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Errors due to backpressure; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; ErrBackpressure is incremented by categorizeError matching the error text (rpc/worker_pool_metrics.go:485-499) of the eth_getLogs refusals at rpc/filter.go:1095, :1105, :1115 |
| <code>interface.7.met-evmrpc-getlogs-errors-range-too-large</code> | notes | evmrpc_getlogs_errors_range_too_large; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Errors due to block range too large; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; ErrRangeTooLarge is incremented by categorizeError matching the error text (rpc/worker_pool_metrics.go:485-499) of the eth_getLogs refusals at rpc/filter.go:1095, :1105, :1115 |
| <code>interface.7.met-evmrpc-getlogs-errors-rate-limited</code> | notes | evmrpc_getlogs_errors_rate_limited; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Errors due to rate limiting; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; ErrRateLimited is incremented by categorizeError matching the error text (rpc/worker_pool_metrics.go:485-499) of the eth_getLogs refusals at rpc/filter.go:1095, :1105, :1115 |
| <code>interface.7.met-evmrpc-getlogs-errors-total</code> | notes | evmrpc_getlogs_errors_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Errored eth_getLogs requests; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |
| <code>interface.7.met-evmrpc-getlogs-max-latency-ms</code> | notes | evmrpc_getlogs_max_latency_ms; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Max eth_getLogs latency (ms); unit ms; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-getlogs-peak-blockrange</code> | notes | evmrpc_getlogs_peak_blockrange; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Peak block range for eth_getLogs; unit {blocks}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-getlogs-requests-total</code> | notes | evmrpc_getlogs_requests_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Total eth_getLogs requests; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |
| <code>interface.7.met-evmrpc-getlogs-success-total</code> | notes | evmrpc_getlogs_success_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Successful eth_getLogs requests; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |
| <code>interface.7.met-evmrpc-getlogs-tps</code> | notes | evmrpc_getlogs_tps; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); eth_getLogs throughput (req/s); unit 1/s; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-subscriptions-active</code> | notes | evmrpc_subscriptions_active; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Active subscriptions; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-avg-exec-time-ms</code> | notes | evmrpc_workerpool_avg_exec_time_ms; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Average execution time (ms); unit ms; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-avg-queue-wait-ms</code> | notes | evmrpc_workerpool_avg_queue_wait_ms; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Average queue wait time (ms); unit ms; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-queue-capacity</code> | notes | evmrpc_workerpool_queue_capacity; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Task queue capacity; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-queue-depth</code> | notes | evmrpc_workerpool_queue_depth; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Current task queue depth; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-queue-peak</code> | notes | evmrpc_workerpool_queue_peak; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Peak queue depth observed; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-queue-utilization-pct</code> | notes | evmrpc_workerpool_queue_utilization_pct; OTel Float64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Queue utilization percentage; unit 1; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-tasks-completed-total</code> | notes | evmrpc_workerpool_tasks_completed_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Tasks completed; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |
| <code>interface.7.met-evmrpc-workerpool-tasks-panicked-total</code> | notes | evmrpc_workerpool_tasks_panicked_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Tasks that panicked; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |
| <code>interface.7.met-evmrpc-workerpool-tasks-rejected-total</code> | notes | evmrpc_workerpool_tasks_rejected_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Tasks rejected due to full queue; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |
| <code>interface.7.met-evmrpc-workerpool-tasks-submitted-total</code> | notes | evmrpc_workerpool_tasks_submitted_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Tasks submitted; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |
| <code>interface.7.met-evmrpc-workerpool-workers-active</code> | notes | evmrpc_workerpool_workers_active; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Active worker count; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-workers-idle</code> | notes | evmrpc_workerpool_workers_idle; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Idle worker count; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter |
| <code>interface.7.met-evmrpc-workerpool-workers-total</code> | notes | evmrpc_workerpool_workers_total; OTel Int64Gauge on meter evmrpc_workerpool (rpc/worker_pool_metrics.go:111); Total worker count; unit {count}; sampled from WorkerPoolMetrics by the periodic exporter; named _total but registered as a gauge |

## net-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-net-version</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/net.go:24</code> | <code>func (i *NetAPI) Version(ctx context.Context) string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-net-version</code> | notes | net_version; HTTP and WS; 14 call site(s) in 9 file(s) |

Contract context <code>contract_context.7.conformance</code>:

| Field | Frozen rule |
|---|---|
| interface | The go-ethereum ethclient decodes every answer it requests from the node over HTTP and WS. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No ethclient decode fails on a node answer. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## newheads

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.10.1</code> | req | 7010.1 |
| <code>acceptance.7.10.1</code> | scenario | Given a committed Autobahn head with every root set, when encodeCommittedBlock encodes it, then sha3Uncles is EmptyUncleHash and excessBlobGas, parentBeaconBlockRoot, withdrawalsRoot and blobGasUsed are absent |
| <code>acceptance.7.10.1</code> | test | rpc/notifier_internal_test.go::TestEncodeCommittedBlockWithRoots (existing) |
| <code>acceptance.7.10.1</code> | qualification | UNRUN |
| <code>req.7010</code> | ac_1 | encodeCommittedBlock carries sha3Uncles EmptyUncleHash and no blob or withdrawal keys, replacing the zero hash and the four keys set at rpc/subscribe.go:560-570. |
| <code>acceptance.7.10.2</code> | req | 7010.2 |
| <code>acceptance.7.10.2</code> | scenario | Given a committed Autobahn head and the encoded block of the same height, when their key sets are compared without transactions, uncles, size and totalDifficulty, then both sets are equal |
| <code>acceptance.7.10.2</code> | test | rpc/notifier_internal_test.go::TestNewHeadsKeySetMatchesBlockHeader (planned) |
| <code>acceptance.7.10.2</code> | qualification | UNRUN |
| <code>req.7010</code> | ac_2 | The newHeads payload has exactly the header keys of the eth_getBlockByNumber map built at rpc/block.go:630. |
| <code>acceptance.7.10.3</code> | req | 7010.3 |
| <code>acceptance.7.10.3</code> | scenario | Given the multi transaction test block with EVM receipts and one Cosmos transaction, when its head and its EncodeTmBlock map are encoded, then logsBloom, gasUsed and transactionsRoot are equal |
| <code>acceptance.7.10.3</code> | test | rpc/notifier_internal_test.go::TestNewHeadsBloomAndGasUsedMatchBlock (planned) |
| <code>acceptance.7.10.3</code> | qualification | UNRUN |
| <code>req.7010</code> | ac_3 | newHeads logsBloom, gasUsed and transactionsRoot equal those eth_getBlockByNumber returns, replacing the empty bloom at rpc/subscribe.go:554 and the Comet gas sum at rpc/subscribe.go:538. |
| <code>acceptance.7.10.4</code> | req | 7010.4 |
| <code>acceptance.7.10.4</code> | scenario | Given a committed Autobahn head whose header AppHash and finalize response AppHash differ, when encodeCommittedBlock encodes it, then stateRoot equals the header AppHash |
| <code>acceptance.7.10.4</code> | test | rpc/notifier_internal_test.go::TestEncodeCommittedBlockStateRootIsHeaderAppHash (planned) |
| <code>acceptance.7.10.4</code> | qualification | UNRUN |
| <code>req.7010</code> | ac_4 | The Autobahn newHeads stateRoot is the header AppHash that eth_getBlockByNumber returns (rpc/block.go:478), not the finalize response AppHash used at rpc/subscribe.go:472. |
| <code>acceptance.7.10.5</code> | req | 7010.5 |
| <code>acceptance.7.10.5</code> | scenario | Given a logs subscription whose log fetch fails, when the subscription goroutine handles the failure, then no notification carrying the error is sent and the subscription ends |
| <code>acceptance.7.10.5</code> | test | rpc/subscribe_logs_test.go::TestLogsSubscriptionNeverNotifiesErrorPayload (planned) |
| <code>acceptance.7.10.5</code> | qualification | UNRUN |
| <code>req.7010</code> | ac_5 | A logs subscription whose fetch fails ends without notifying the error value as a payload, which HEAD does at rpc/subscribe.go:271. |
| <code>acceptance.7.10.6</code> | req | 7010.6 |
| <code>acceptance.7.10.6</code> | scenario | Given a SubscriptionAPI with logLimit 1 and one live logs subscription, when a second Logs call is made and then the first is unsubscribed and Logs is called again, then the second call fails and the third succeeds |
| <code>acceptance.7.10.6</code> | test | rpc/subscribe_logs_limit_test.go::TestLogsSubscriptionLimit (planned) |
| <code>acceptance.7.10.6</code> | qualification | UNRUN |
| <code>req.7010</code> | ac_6 | Logs (rpc/subscribe.go:230) refuses a subscription once logLimit live logs subscriptions exist, as newHeads does at rpc/subscribe.go:199. |
| <code>acceptance.7.10.7</code> | req | 7010.7 |
| <code>acceptance.7.10.7</code> | scenario | Given a polling logs subscription (no receipt log feed) opened over WebSocket whose eth_subscribe call has returned, when two later blocks carrying matching logs are committed and the client then calls eth_unsubscribe, then the subscriber receives the logs of both blocks and the polling goroutine exits after the unsubscribe |
| <code>acceptance.7.10.7</code> | test | rpc/subscribe_logs_test.go::TestLogsSubscriptionOutlivesSubscribeCall (planned) |
| <code>acceptance.7.10.7</code> | qualification | UNRUN |
| <code>req.7010</code> | ac_7 | A polling logs subscription keeps delivering logs of blocks committed after the eth_subscribe call returned and ends when rpcSub.Err() fires, instead of waiting on the call context at rpc/subscribe.go:316, which the handler cancels when the call returns (core/deps/geth/rpc/handler.go:387-389). |

## pax-associate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-associate</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/association.go:55</code> | <code>func (t *AssociationAPI) Associate(ctx context.Context, req *AssociateRequest) (returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-associate</code> | notes | pax_associate; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 1 call site(s) in 1 file(s) |

## pax-cleanup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-cleanup</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/filter.go:1206</code> | <code>func (a *FilterAPI) Cleanup()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-cleanup</code> | notes | pax_cleanup; HTTP only; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-cleanup</code> | overridden_by | override.J38.1 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getblockbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbyhash</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:176</code> | <code>func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getblockbyhash</code> | notes | pax_getBlockByHash; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |
| <code>interface.7.rpc-pax-getblockbyhash</code> | overridden_by | override.J38.30 |

Contract context <code>contract_context.7.height-availability</code>:

| Field | Frozen rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getblockbyhashexcludetracefail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbyhashexcludetracefail</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:123</code> | <code>func (a *PaxBlockAPI) GetBlockByHashExcludeTraceFail(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getblockbyhashexcludetracefail</code> | notes | pax_getBlockByHashExcludeTraceFail; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 2 call site(s) in 1 file(s) |

## pax-getblockbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbynumber</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:210</code> | <code>func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getblockbynumber</code> | notes | pax_getBlockByNumber; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 2 call site(s) in 2 file(s) |
| <code>interface.7.rpc-pax-getblockbynumber</code> | overridden_by | override.J38.31 |

Contract context <code>contract_context.7.height-availability</code>:

| Field | Frozen rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getblockbynumberexcludetracefail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getblockbynumberexcludetracefail</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:117</code> | <code>func (a *PaxBlockAPI) GetBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getblockbynumberexcludetracefail</code> | notes | pax_getBlockByNumberExcludeTraceFail; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 1 call site(s) in 1 file(s) |

## pax-getblockreceipts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getblockreceipts</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:344</code> | <code>func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getblockreceipts</code> | notes | pax_getBlockReceipts; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-getblocktransactioncountbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getblocktransactioncountbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:154</code> | <code>func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getblocktransactioncountbyhash</code> | notes | pax_getBlockTransactionCountByHash; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-getblocktransactioncountbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getblocktransactioncountbynumber</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:128</code> | <code>func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getblocktransactioncountbynumber</code> | notes | pax_getBlockTransactionCountByNumber; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |
| <code>interface.7.rpc-pax-getblocktransactioncountbynumber</code> | overridden_by | override.J38.24 |

Contract context <code>contract_context.7.pending-block</code>:

| Field | Frozen rule |
|---|---|
| interface | On a sequencer the pending tag answers from the pending snapshot: header fields, transaction count and transaction by index; without a pending source the HEAD answers stay. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getcosmostx

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getcosmostx</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/association.go:153</code> | <code>func (t *AssociationAPI) GetCosmosTx(ctx context.Context, ethHash common.Hash) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getcosmostx</code> | notes | pax_getCosmosTx; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); enabled by default (rpc/config/config.go:151); 2 call site(s) in 2 file(s) |

## pax-getevmaddress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getevmaddress</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/association.go:128</code> | <code>func (t *AssociationAPI) GetEVMAddress(ctx context.Context, paxAddress string) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getevmaddress</code> | notes | pax_getEVMAddress; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); enabled by default (rpc/config/config.go:151); 2 call site(s) in 2 file(s) |

## pax-getevmtx

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getevmtx</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/association.go:179</code> | <code>func (t *AssociationAPI) GetEvmTx(ctx context.Context, cosmosHash string) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getevmtx</code> | notes | pax_getEvmTx; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 1 call site(s) in 1 file(s) |

## pax-getfilterchanges

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getfilterchanges</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:876</code> | <code>func (a *FilterAPI) GetFilterChanges(ctx context.Context, filterID ethrpc.ID) (res interface{}, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getfilterchanges</code> | notes | pax_getFilterChanges; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 2 call site(s) in 1 file(s) |

## pax-getfilterlogs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getfilterlogs</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:989</code> | <code>func (a *FilterAPI) GetFilterLogs(ctx context.Context, filterID ethrpc.ID) (res []*FilterLog, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getfilterlogs</code> | notes | pax_getFilterLogs; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 2 call site(s) in 1 file(s) |

## pax-getlogs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getlogs</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/filter.go:1045</code> | <code>func (a *FilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) (res []*FilterLog, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getlogs</code> | notes | pax_getLogs; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 1 call site(s) in 1 file(s) |
| <code>interface.7.rpc-pax-getlogs</code> | overridden_by | override.J38.22 |

Contract context <code>contract_context.7.filters</code>:

| Field | Frozen rule |
|---|---|
| interface | eth_getLogs fails when the earliest height cannot be read and answers unknown block for an unknown or zero blockHash (decision 719). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A log query never scans from height 0 because of a store read failure. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getpaxaddress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getpaxaddress</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/association.go:115</code> | <code>func (t *AssociationAPI) GetPaxAddress(ctx context.Context, ethAddress common.Address) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getpaxaddress</code> | notes | pax_getPaxAddress; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); enabled by default (rpc/config/config.go:151); 3 call site(s) in 2 file(s) |

## pax-getrawblock

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getrawblock</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:723</code> | <code>func (api *DebugAPI) GetRawBlock(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getrawblock</code> | notes | pax_getRawBlock; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-getrawblock</code> | overridden_by | override.J38.4 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.779</code> | title | decision.717 binds: Pax registered set |
| <code>resolution.17.779</code> | applies_to | decision.717<br>task.7.1.1<br>task.7.3.1<br>task.7.3.2 |
| <code>resolution.17.779</code> | source | decision.717 in fragments/J38/decision.kvx; tasks from affects |

## pax-getrawheader

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getrawheader</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:715</code> | <code>func (api *DebugAPI) GetRawHeader(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getrawheader</code> | notes | pax_getRawHeader; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-getrawheader</code> | overridden_by | override.J38.5 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getrawreceipts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getrawreceipts</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:731</code> | <code>func (api *DebugAPI) GetRawReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (_ []hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getrawreceipts</code> | notes | pax_getRawReceipts; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-getrawreceipts</code> | overridden_by | override.J38.6 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-getrawtransaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getrawtransaction</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:739</code> | <code>func (api *DebugAPI) GetRawTransaction(ctx context.Context, hash common.Hash) (_ hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getrawtransaction</code> | notes | pax_getRawTransaction; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-getrawtransaction</code> | overridden_by | override.J38.7 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-gettransactionbyblockhashandindex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyblockhashandindex</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:242</code> | <code>func (t *TransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyblockhashandindex</code> | notes | pax_getTransactionByBlockHashAndIndex; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-gettransactionbyblocknumberandindex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyblocknumberandindex</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/tx.go:208</code> | <code>func (t *TransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, txIndex hexutil.Uint) (result *export.RPCTransaction, _err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyblocknumberandindex</code> | notes | pax_getTransactionByBlockNumberAndIndex; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |
| <code>interface.7.rpc-pax-gettransactionbyblocknumberandindex</code> | overridden_by | override.J38.27 |

Contract context <code>contract_context.7.pending-block</code>:

| Field | Frozen rule |
|---|---|
| interface | On a sequencer the pending tag answers from the pending snapshot: header fields, transaction count and transaction by index; without a pending source the HEAD answers stay. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-gettransactionbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:269</code> | <code>func (t *TransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (result *export.RPCTransaction, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-gettransactionbyhash</code> | notes | pax_getTransactionByHash; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-gettransactioncount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactioncount</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:397</code> | <code>func (t *TransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (result *hexutil.Uint64, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-gettransactioncount</code> | notes | pax_getTransactionCount; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-gettransactionerrorbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionerrorbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:379</code> | <code>func (t *TransactionAPI) GetTransactionErrorByHash(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-gettransactionerrorbyhash</code> | notes | pax_getTransactionErrorByHash; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-gettransactionreceipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionreceipt</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:106</code> | <code>func (t *TransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-gettransactionreceipt</code> | notes | pax_getTransactionReceipt; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-gettransactionreceiptexcludetracefail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-gettransactionreceiptexcludetracefail</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:102</code> | <code>func (t *PaxTransactionAPI) GetTransactionReceiptExcludeTraceFail(ctx context.Context, hash common.Hash) (result map[string]any, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-gettransactionreceiptexcludetracefail</code> | notes | pax_getTransactionReceiptExcludeTraceFail; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 2 call site(s) in 1 file(s) |

## pax-getvmerror

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-getvmerror</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:193</code> | <code>func (t *TransactionAPI) GetVMError(ctx context.Context, hash common.Hash) (result string, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-getvmerror</code> | notes | pax_getVMError; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax-newblockfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-newblockfilter</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:814</code> | <code>func (a *FilterAPI) NewBlockFilter(ctx context.Context) (id ethrpc.ID, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-newblockfilter</code> | notes | pax_newBlockFilter; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 2 call site(s) in 2 file(s) |

## pax-newfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-newfilter</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:767</code> | <code>func (a *FilterAPI) NewFilter(ctx context.Context, crit filters.FilterCriteria) (id ethrpc.ID, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-newfilter</code> | notes | pax_newFilter; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 5 call site(s) in 3 file(s) |

## pax-newpendingtransactionfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-newpendingtransactionfilter</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/filter.go:840</code> | <code>func (a *FilterAPI) NewPendingTransactionFilter(ctx context.Context, fullTx *bool) (id ethrpc.ID, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-newpendingtransactionfilter</code> | notes | pax_newPendingTransactionFilter; HTTP only; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-newpendingtransactionfilter</code> | overridden_by | override.J38.3 |

Contract context <code>contract_context.7.served-methods</code>:

| Field | Frozen rule |
|---|---|
| interface | The geth server registers every exported method of a service whose results are at most two with a trailing error (core/deps/geth/rpc/service.go); ServedMethods reproduces that rule so tests read the exact HTTP and WS sets. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Served sets are a pure function of the registered rpc.API lists; the goldens change only with a registration change. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.770</code> | title | decision.708 binds: WebSocket service set |
| <code>resolution.17.770</code> | applies_to | decision.708<br>task.7.0.4<br>task.7.9.2 |
| <code>resolution.17.770</code> | source | decision.708 in fragments/J38/decision.kvx; tasks from affects |

## pax-sequencerreceipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-sequencerreceipt</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/sequencer_receipt.go:232</code> | <code>func (a *SequencerReceiptAPI) SequencerReceipt(ctx context.Context, txHash common.Hash) (result *SequencerReceiptResult, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-sequencerreceipt</code> | notes | pax_sequencerReceipt; HTTP only; exempt from the legacy gate (rpc/pax_legacy.go:145); local on sequencer and standby, forwarded elsewhere; rungs submitted, receipted, committed; 57 call site(s) in 19 file(s); a node with neither a source nor a forward endpoint answers -32000 "served by sequencer" (rpc/sequencer_receipt.go:49, :99) |

| Decision | Title | Value | Basis | Affects |
|---|---|---|---|---|
| <code>decision.1722</code> | Interface notes follow the code at HEAD | The launchpad add-protocol-fees notes cite ProtocolFeesKey at modules/launchpad/types/keys.go:76 under ProtocolFeesPrefix at modules/launchpad/types/keys.go:38; pax_sequencerReceipt notes state the -32000 not-served answer; override.J55.17 carries both transfers-panel spans and override.J55.16 agrees with it. | ProtocolFeesPrefix is modules/launchpad/types/keys.go:38; sequencerNotServedCode is rpc/sequencer_receipt.go:49. | <code>interface.4.go-launchpad-keeper-keeper-add-protocol-fees</code><br><code>interface.7.rpc-pax-sequencerreceipt</code><br><code>override.J55.17</code> |

## pax-setreceiptlogfeed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-setreceiptlogfeed</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/filter.go:583</code> | <code>func (a *FilterAPI) SetReceiptLogFeed(feed *ReceiptLogFeed)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-setreceiptlogfeed</code> | notes | pax_setReceiptLogFeed; HTTP only; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-setreceiptlogfeed</code> | overridden_by | override.J38.2 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-sign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-sign</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tx.go:530</code> | <code>func (t *TransactionAPI) Sign(ctx context.Context, addr common.Address, data hexutil.Bytes) (result hexutil.Bytes, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-sign</code> | notes | pax_sign; HTTP only; promoted from embedded TransactionAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 6 call site(s) in 3 file(s) |

## pax-traceblockbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-traceblockbyhash</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:667</code> | <code>func (api *DebugAPI) TraceBlockByHash(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-traceblockbyhash</code> | notes | pax_traceBlockByHash; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-traceblockbyhash</code> | overridden_by | override.J38.8 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-traceblockbyhashexcludetracefail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-traceblockbyhashexcludetracefail</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:556</code> | <code>func (api *PaxDebugAPI) TraceBlockByHashExcludeTraceFail(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-traceblockbyhashexcludetracefail</code> | notes | pax_traceBlockByHashExcludeTraceFail; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 1 call site(s) in 1 file(s) |

## pax-traceblockbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-traceblockbynumber</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:639</code> | <code>func (api *DebugAPI) TraceBlockByNumber(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-traceblockbynumber</code> | notes | pax_traceBlockByNumber; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-traceblockbynumber</code> | overridden_by | override.J38.9 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-traceblockbynumberexcludetracefail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-traceblockbynumberexcludetracefail</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/tracers.go:520</code> | <code>func (api *PaxDebugAPI) TraceBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-traceblockbynumberexcludetracefail</code> | notes | pax_traceBlockByNumberExcludeTraceFail; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 1 call site(s) in 1 file(s) |

## pax-tracecall

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-tracecall</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:695</code> | <code>func (api *DebugAPI) TraceCall(ctx context.Context, args export.TransactionArgs, blockNrOrHash rpc.BlockNumberOrHash, config *tracers.TraceCallConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-tracecall</code> | notes | pax_traceCall; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-tracecall</code> | overridden_by | override.J38.10 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-tracestateaccess

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-tracestateaccess</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:753</code> | <code>func (api *DebugAPI) TraceStateAccess(ctx context.Context, hash common.Hash) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-tracestateaccess</code> | notes | pax_traceStateAccess; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-tracestateaccess</code> | overridden_by | override.J38.11 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-tracetransaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-tracetransaction</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/tracers.go:282</code> | <code>func (api *DebugAPI) TraceTransaction(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-tracetransaction</code> | notes | pax_traceTransaction; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-tracetransaction</code> | overridden_by | override.J38.12 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-tracetransactionprofile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-tracetransactionprofile</code> | <code>jsonrpc_method</code> | <code>retire</code> | <code>rpc/trace_profile.go:45</code> | <code>func (api *DebugAPI) TraceTransactionProfile(ctx context.Context, hash common.Hash, config *tracers.TraceConfig) (result interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-tracetransactionprofile</code> | notes | pax_traceTransactionProfile; HTTP only; promoted from embedded DebugAPI; not in paxLegacyGatedMethods (rpc/pax_legacy.go:42-80), so the HTTP gate refuses it fail-closed (rpc/pax_legacy.go:158-161); no in-repo caller |
| <code>interface.7.rpc-pax-tracetransactionprofile</code> | overridden_by | override.J38.13 |

Contract context <code>contract_context.7.retirements</code>:

| Field | Frozen rule |
|---|---|
| interface | Exported helpers on served services become served methods; Cleanup, SetReceiptLogFeed, GasPriceHelper, CalculateGasUsedRatio and the promoted pax debug methods are retired by unexporting or by removing the embedding. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The registered pax and pax2 set equals PaxLegacyAllGatedMethodNames minus nothing and plus only pax_sequencerReceipt on a sequencer receipt server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax-uninstallfilter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax-uninstallfilter</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/filter.go:1176</code> | <code>func (a *FilterAPI) UninstallFilter(ctx context.Context, filterID ethrpc.ID) (res bool)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax-uninstallfilter</code> | notes | pax_uninstallFilter; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); 1 call site(s) in 1 file(s) |

## pax2-getblockbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbyhash</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:176</code> | <code>func (a *BlockAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax2-getblockbyhash</code> | notes | pax2_getBlockByHash; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |
| <code>interface.7.rpc-pax2-getblockbyhash</code> | overridden_by | override.J38.32 |

Contract context <code>contract_context.7.height-availability</code>:

| Field | Frozen rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax2-getblockbyhashexcludetracefail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbyhashexcludetracefail</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:123</code> | <code>func (a *PaxBlockAPI) GetBlockByHashExcludeTraceFail(ctx context.Context, blockHash common.Hash, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax2-getblockbyhashexcludetracefail</code> | notes | pax2_getBlockByHashExcludeTraceFail; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax2-getblockbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbynumber</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:210</code> | <code>func (a *BlockAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax2-getblockbynumber</code> | notes | pax2_getBlockByNumber; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |
| <code>interface.7.rpc-pax2-getblockbynumber</code> | overridden_by | override.J38.33 |

Contract context <code>contract_context.7.height-availability</code>:

| Field | Frozen rule |
|---|---|
| interface | Block lookups and simulations carry no chain-id specific floor or baseFee branch for pacific-1. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | A height inside the store is answered on every chain id. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pax2-getblockbynumberexcludetracefail

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockbynumberexcludetracefail</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:117</code> | <code>func (a *PaxBlockAPI) GetBlockByNumberExcludeTraceFail(ctx context.Context, number rpc.BlockNumber, fullTx bool) (result map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax2-getblockbynumberexcludetracefail</code> | notes | pax2_getBlockByNumberExcludeTraceFail; HTTP only; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax2-getblockreceipts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax2-getblockreceipts</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:344</code> | <code>func (a *BlockAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (result []map[string]interface{}, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax2-getblockreceipts</code> | notes | pax2_getBlockReceipts; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax2-getblocktransactioncountbyhash

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax2-getblocktransactioncountbyhash</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/block.go:154</code> | <code>func (a *BlockAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (result *hexutil.Uint, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax2-getblocktransactioncountbyhash</code> | notes | pax2_getBlockTransactionCountByHash; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |

## pax2-getblocktransactioncountbynumber

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-pax2-getblocktransactioncountbynumber</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/block.go:128</code> | <code>func (a *BlockAPI) GetBlockTransactionCountByNumber(ctx context.Context, number rpc.BlockNumber) (result *hexutil.Uint, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-pax2-getblocktransactioncountbynumber</code> | notes | pax2_getBlockTransactionCountByNumber; HTTP only; promoted from embedded BlockAPI; deprecated and gated: refused with paxLegacyNotEnabled and data legacy_pax_deprecated unless listed in evm.enabled_legacy_pax_apis (rpc/pax_legacy.go:150-166); no in-repo caller |
| <code>interface.7.rpc-pax2-getblocktransactioncountbynumber</code> | overridden_by | override.J38.25 |

Contract context <code>contract_context.7.pending-block</code>:

| Field | Frozen rule |
|---|---|
| interface | On a sequencer the pending tag answers from the pending snapshot: header fields, transaction count and transaction by index; without a pending source the HEAD answers stay. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | The pending block never claims a hash or a stateRoot. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## pending-block

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.11.1</code> | req | 7011.1 |
| <code>acceptance.7.11.1</code> | scenario | Given a sequencer server holding one pending transfer, when eth_getBlockByNumber is called with pending and false, then parentHash equals the latest block hash, gasUsed equals the transfer receipt gas and hash is null |
| <code>acceptance.7.11.1</code> | test | rpc/pending_block_fields_test.go::TestPendingBlockCarriesHeaderFields (planned) |
| <code>acceptance.7.11.1</code> | qualification | UNRUN |
| <code>req.7011</code> | ac_1 | eth_getBlockByNumber("pending") returns parentHash, miner, gasLimit, gasUsed, baseFeePerGas, logsBloom and sha3Uncles in addition to the five keys built at rpc/block.go:296. |
| <code>acceptance.7.11.2</code> | req | 7011.2 |
| <code>acceptance.7.11.2</code> | scenario | Given a sequencer server holding two pending transfers, when eth_getBlockTransactionCountByNumber is called with pending, then the result is 0x2 |
| <code>acceptance.7.11.2</code> | test | rpc/pending_block_txcount_test.go::TestPendingBlockTransactionCount (planned) |
| <code>acceptance.7.11.2</code> | qualification | UNRUN |
| <code>req.7011</code> | ac_2 | eth_getBlockTransactionCountByNumber("pending") on a sequencer returns the number of receipted pending transactions instead of the null of rpc/block.go:137. |
| <code>acceptance.7.11.3</code> | req | 7011.3 |
| <code>acceptance.7.11.3</code> | scenario | Given a sequencer server holding one pending transfer, when eth_getTransactionByBlockNumberAndIndex is called with pending and 0 and with pending and 1, then the first returns the transfer hash and the second is null |
| <code>acceptance.7.11.3</code> | test | rpc/pending_tx_index_test.go::TestPendingTransactionByBlockNumberAndIndex (planned) |
| <code>acceptance.7.11.3</code> | qualification | UNRUN |
| <code>req.7011</code> | ac_3 | eth_getTransactionByBlockNumberAndIndex("pending", 0) on a sequencer returns the first pending transaction encoded by encodePendingTransaction (rpc/tx.go:348) and null past the end. |

## retirements

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.1.9</code> | req | 7001.9 |
| <code>acceptance.7.1.9</code> | scenario | Given the HTTP server of the package test setup, when its pax_ and pax2_ names are compared with PaxLegacyAllGatedMethodNames, then both sets are equal and pax_sequencerReceipt is absent |
| <code>acceptance.7.1.9</code> | test | rpc/pax_served_set_test.go::TestPaxServedSetEqualsGatedSet (planned) |
| <code>acceptance.7.1.9</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_9 | The pax_ and pax2_ names the HTTP server registers equal PaxLegacyAllGatedMethodNames (rpc/pax_legacy.go:99), so the fail-closed branch at rpc/pax_legacy.go:159 refuses no registered name. |
| <code>acceptance.7.2.1</code> | req | 7002.1 |
| <code>acceptance.7.2.1</code> | scenario | Given the HTTP server of the package test setup, when eth_cleanup is called and HTTPMethods is read, then the call answers -32601 and neither eth_cleanup nor pax_cleanup is listed |
| <code>acceptance.7.2.1</code> | test | rpc/filter_shutdown_test.go::TestFilterCleanupNotServed (planned) |
| <code>acceptance.7.2.1</code> | qualification | UNRUN |
| <code>req.7002</code> | ac_1 | eth_cleanup and pax_cleanup, served at HEAD from the exported FilterAPI.Cleanup (rpc/filter.go:1206), are no longer registered and eth_cleanup answers -32601. |
| <code>acceptance.7.2.2</code> | req | 7002.2 |
| <code>acceptance.7.2.2</code> | scenario | Given a fresh HTTP server with one installed log filter, when Stop is called, then the filter context is cancelled and a second shutdown call returns without panic |
| <code>acceptance.7.2.2</code> | test | rpc/filter_shutdown_test.go::TestHTTPServerStopCancelsFilters (planned) |
| <code>acceptance.7.2.2</code> | qualification | UNRUN |
| <code>req.7002</code> | ac_2 | Stopping the HTTP server through Stop (rpc/rpcstack.go:253) runs FilterAPI shutdown once, cancelling every installed filter. |
| <code>acceptance.7.2.3</code> | req | 7002.3 |
| <code>acceptance.7.2.3</code> | scenario | Given the HTTP server of the package test setup, when eth_gasPriceHelper and eth_calculateGasUsedRatio are called, then both answer -32601 |
| <code>acceptance.7.2.3</code> | test | rpc/info_helpers_test.go::TestInfoHelpersNotServed (planned) |
| <code>acceptance.7.2.3</code> | qualification | UNRUN |
| <code>req.7002</code> | ac_3 | eth_gasPriceHelper and eth_calculateGasUsedRatio, served at HEAD because GasPriceHelper (rpc/info.go:130) and CalculateGasUsedRatio (rpc/info.go:547) are exported, answer -32601. |
| <code>acceptance.7.2.4</code> | req | 7002.4 |
| <code>acceptance.7.2.4</code> | scenario | Given a block with two EVM receipts of known gas used, when CalculateGasUsedRatioForTest is called, then the ratio equals the summed gas over the block gas limit |
| <code>acceptance.7.2.4</code> | test | rpc/info_test.go::TestCalculateGasUsedRatioGasAccumulation (existing) |
| <code>acceptance.7.2.4</code> | qualification | UNRUN |
| <code>req.7002</code> | ac_4 | calculateGasUsedRatio keeps summing receipt gas across the block when reached through CalculateGasUsedRatioForTest (planned), the behaviour rpc/info_test.go:334 asserts at HEAD. |
| <code>acceptance.7.2.5</code> | req | 7002.5 |
| <code>acceptance.7.2.5</code> | scenario | Given the HTTP server of the package test setup, when eth_setReceiptLogFeed is called with an empty object, then the answer carries error code -32601 |
| <code>acceptance.7.2.5</code> | test | rpc/filter_feed_setter_test.go::TestSetReceiptLogFeedNotServed (planned) |
| <code>acceptance.7.2.5</code> | qualification | UNRUN |
| <code>req.7002</code> | ac_5 | eth_setReceiptLogFeed, served at HEAD because SetReceiptLogFeed is exported on FilterAPI (rpc/filter.go:583), answers -32601. |
| <code>acceptance.7.3.1</code> | req | 7003.1 |
| <code>acceptance.7.3.1</code> | scenario | Given NewPaxDebugAPI built from the package test fixtures, when ServedMethods is called on it under the pax namespace, then the result is exactly pax_traceBlockByHashExcludeTraceFail and pax_traceBlockByNumberExcludeTraceFail |
| <code>acceptance.7.3.1</code> | test | rpc/pax_debug_surface_test.go::TestPaxDebugServesOnlyExcludeTraceFail (planned) |
| <code>acceptance.7.3.1</code> | qualification | UNRUN |
| <code>req.7003</code> | ac_1 | PaxDebugAPI, which at HEAD embeds *DebugAPI (rpc/tracers.go:193) and so registers pax_traceTransaction, pax_traceCall, pax_getRaw* and the other debug methods that the gate at rpc/pax_legacy.go:159 refuses, registers only its two ExcludeTraceFail methods. |
| <code>acceptance.7.3.2</code> | req | 7003.2 |
| <code>acceptance.7.3.2</code> | scenario | Given NewPaxFilterAPI over a FilterAPI built from the package test fixtures, when ServedMethods is called on it under the pax namespace, then the result is exactly the six gated pax filter names |
| <code>acceptance.7.3.2</code> | test | rpc/pax_filter_test.go::TestPaxFilterAPIMethodSetMatchesGate (planned) |
| <code>acceptance.7.3.2</code> | qualification | UNRUN |
| <code>req.7003</code> | ac_2 | PaxFilterAPI in rpc/pax_filter.go (planned) serves exactly pax_newFilter, pax_newBlockFilter, pax_getFilterChanges, pax_getFilterLogs, pax_getLogs and pax_uninstallFilter, the filter names of rpc/pax_legacy.go:42. |

## rpc-modules

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-rpc-modules</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>core/deps/geth/rpc/server.go:231</code> | <code>func (s *RPCService) Modules() map[string]string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-rpc-modules</code> | notes | rpc_modules; server built-in registered as MetadataApi rpc (core/deps/geth/rpc/server.go:74) on HTTP and WS; lists the registered namespaces; 1 call site(s) in 1 file(s) |

## sequencer-options

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.go-newreceiptlogfeed-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/filter.go:266</code> | <code>func NewReceiptLogFeed(committedHeight int64) *ReceiptLogFeed</code> |
| <code>interface.7.go-pendingoption-j38</code> | <code>go_exported</code> | <code>new</code> | <code>rpc/server_options.go</code> (planned) | <code>type PendingOption; func NewPendingOption(source PendingSnapshot) (PendingOption, error)</code> |
| <code>interface.7.go-receiptlogfeed-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/filter.go:214</code> | <code>type ReceiptLogFeed struct</code> |
| <code>interface.7.go-sequencerforwardoption-j38</code> | <code>go_exported</code> | <code>new</code> | <code>rpc/server_options.go</code> (planned) | <code>type SequencerForwardOption; func NewSequencerForwardOption(raw string) (SequencerForwardOption, error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.go-newreceiptlogfeed-j38</code> | notes | Receipt log feed shared by the HTTP filter service and the WS logs subscription; 0 use(s) outside rpc/ |
| <code>interface.7.go-pendingoption-j38</code> | notes | ServerOption installing the pending snapshot through SetPendingSnapshot; tasks 7.12.1 and 7.12.2; node pass-through is wiring.J38.1 |
| <code>interface.7.go-receiptlogfeed-j38</code> | notes | Receipt log feed shared by the HTTP filter service and the WS logs subscription; 0 use(s) outside rpc/ |
| <code>interface.7.go-sequencerforwardoption-j38</code> | notes | ServerOption setting SendConfig.SequencerURL so follower nodes forward raw transactions; tasks 7.12.1 and 7.12.2 |

Contract context <code>contract_context.7.sequencer-options</code>:

| Field | Frozen rule |
|---|---|
| interface | NewEVMHTTPServer and NewEVMWebSocketServer accept SequencerOption, SequencerReceiptOption, PendingOption and SequencerForwardOption; the node passes them through wiring.J38.1 and wiring.J38.2 (decision 713). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | SequencerOption and SequencerForwardOption are mutually exclusive on one server. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.1.5</code> | req | 7001.5 |
| <code>acceptance.7.1.5</code> | scenario | Given an HTTP and a WebSocket server built with the same SequencerOption, when a signed transfer is sent with eth_sendRawTransaction over HTTP, then the WebSocket newPendingTransactions subscriber receives its hash and the logs subscriber receives its receipt logs |
| <code>acceptance.7.1.5</code> | test | rpc/sequencer_options_integration_test.go::TestSequencerOptionsShareFeedAcrossHTTPAndWS (planned) |
| <code>acceptance.7.1.5</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_5 | One SequencerOption built by NewSequencerOption (rpc/server.go:55) and handed to both servers makes a hash accepted over HTTP reach newPendingTransactions and logs subscribers on the WebSocket server. |
| <code>acceptance.7.1.6</code> | req | 7001.6 |
| <code>acceptance.7.1.6</code> | scenario | Given an HTTP server built with the sequencer, sequencer receipt and pending options, when a transfer is accepted and then eth_getBlockByNumber pending and pax_sequencerReceipt are called, then the pending block lists the hash and the receipt carries the same hash |
| <code>acceptance.7.1.6</code> | test | rpc/sequencer_options_integration_test.go::TestSequencerOptionsServePendingAndReceipt (planned) |
| <code>acceptance.7.1.6</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_6 | With SequencerOption, SequencerReceiptOption (rpc/sequencer_receipt.go:100) and PendingOption (planned) set, eth_getBlockByNumber("pending") lists an accepted hash and pax_sequencerReceipt returns its receipt. |
| <code>acceptance.7.12.1</code> | req | 7012.1 |
| <code>acceptance.7.12.1</code> | scenario | Given a nil source and the pending sequencer fixture, when NewPendingOption is called with each, then the first returns an error and the second returns an option |
| <code>acceptance.7.12.1</code> | test | rpc/server_options_test.go::TestNewPendingOptionRejectsNil (planned) |
| <code>acceptance.7.12.1</code> | qualification | UNRUN |
| <code>req.7012</code> | ac_1 | NewPendingOption (planned) refuses a nil PendingSnapshot (rpc/utils.go:55) and accepts the pending sequencer fixture. |
| <code>acceptance.7.12.2</code> | req | 7012.2 |
| <code>acceptance.7.12.2</code> | scenario | Given a non-HTTP scheme URL, an HTTP URL without a host and the URL of a local test endpoint, when NewSequencerForwardOption is called with each, then the first two return errors and the third returns an option |
| <code>acceptance.7.12.2</code> | test | rpc/server_options_test.go::TestNewSequencerForwardOptionRejectsNonHTTPURL (planned) |
| <code>acceptance.7.12.2</code> | qualification | UNRUN |
| <code>req.7012</code> | ac_2 | NewSequencerForwardOption (planned) accepts only an http or https URL with a host for SendConfig.SequencerURL (rpc/send.go:52). |
| <code>acceptance.7.12.3</code> | req | 7012.3 |
| <code>acceptance.7.12.3</code> | scenario | Given an HTTP server built with a PendingOption over the pending sequencer fixture holding one transfer, when eth_getBlockByNumber is called with pending, then the transactions list holds the transfer hash |
| <code>acceptance.7.12.3</code> | test | rpc/server_options_wiring_test.go::TestHTTPServerPendingOptionServesPendingBlock (planned) |
| <code>acceptance.7.12.3</code> | qualification | UNRUN |
| <code>req.7012</code> | ac_3 | A PendingOption passed to NewEVMHTTPServer installs the pending source through SetPendingSnapshot (rpc/watermark_manager.go:51), which no production path calls at HEAD. |
| <code>acceptance.7.12.4</code> | req | 7012.4 |
| <code>acceptance.7.12.4</code> | scenario | Given an HTTP server built with a SequencerForwardOption pointing at a recording test endpoint, when eth_sendRawTransaction is called, then the endpoint receives one eth_sendRawTransaction request with the same bytes and the server returns the transaction hash |
| <code>acceptance.7.12.4</code> | test | rpc/server_options_wiring_test.go::TestHTTPServerForwardOptionForwardsRawTransaction (planned) |
| <code>acceptance.7.12.4</code> | qualification | UNRUN |
| <code>req.7012</code> | ac_4 | A SequencerForwardOption sets SendConfig.SequencerURL (rpc/send.go:52), never set at HEAD, so eth_sendRawTransaction forwards the bytes to that endpoint. |
| <code>resolution.17.775</code> | title | decision.713 binds: Sequencer options in the node |
| <code>resolution.17.775</code> | applies_to | decision.713<br>task.7.0.3<br>task.7.12.1<br>task.7.12.2 |
| <code>resolution.17.775</code> | source | decision.713 in fragments/J38/decision.kvx; tasks from affects |

## served-methods

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.go-associaterequest-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/association.go:48</code> | <code>type AssociateRequest struct</code> |
| <code>interface.7.go-blockheadernotifier-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/notifier.go:43</code> | <code>type BlockHeaderNotifier struct</code> |
| <code>interface.7.go-connectiontypehttp-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/server.go:26</code> | <code>var ConnectionTypeHTTP ConnectionType = "http"</code> |
| <code>interface.7.go-evmserver-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/server.go:31</code> | <code>type EVMServer interface</code> |
| <code>interface.7.go-httpconfig-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/rpcstack.go:40</code> | <code>type HTTPConfig struct</code> |
| <code>interface.7.go-httpserver-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/rpcstack.go:72</code> | <code>type HTTPServer struct</code> |
| <code>interface.7.go-httpserver-servedmethods-j38</code> | <code>go_exported</code> | <code>new</code> | <code>rpc/served_methods.go</code> (planned) | <code>func (h *HTTPServer) HTTPMethods() []string; func (h *HTTPServer) WSMethods() []string</code> |
| <code>interface.7.go-latestctxheight-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/utils.go:42</code> | <code>const LatestCtxHeight int64 = -1</code> |
| <code>interface.7.go-localaddress-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/server.go:28</code> | <code>const LocalAddress</code> |
| <code>interface.7.go-newblockapi-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/block.go:57</code> | <code>func NewBlockAPI(tmClient client.LocalClient, k *keeper.Keeper, ctxProvider func(int64) sdk.Context, txConfigProvider func(int64) client.TxConfig, connectionType ConnectionType, watermarks *WatermarkManager, globalBlockCache BlockCache, cacheCreationMutex *sync.Mutex) *BlockAPI</code> |
| <code>interface.7.go-newblockcache-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/filter.go:61</code> | <code>func NewBlockCache(maxSize int) BlockCache</code> |
| <code>interface.7.go-newblockheadernotifier-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/notifier.go:50</code> | <code>func NewBlockHeaderNotifier(capacity int) *BlockHeaderNotifier</code> |
| <code>interface.7.go-newechoapi-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/echo.go:5</code> | <code>func NewEchoAPI() *EchoAPI</code> |
| <code>interface.7.go-newevmhttpserver-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/server.go:87</code> | <code>func NewEVMHTTPServer(</code> |
| <code>interface.7.go-newevmwebsocketserver-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/server.go:315</code> | <code>func NewEVMWebSocketServer(</code> |
| <code>interface.7.go-newhttpserver-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/rpcstack.go:102</code> | <code>func NewHTTPServer(timeouts rpc.HTTPTimeouts) *HTTPServer</code> |
| <code>interface.7.go-newpaxblockapi-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/block.go:73</code> | <code>func NewPaxBlockAPI(</code> |
| <code>interface.7.go-newsequenceroption-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/server.go:55</code> | <code>func NewSequencerOption(accepter SequencerAccepter, feed *ReceiptLogFeed) SequencerOption</code> |
| <code>interface.7.go-newwatermarkmanager-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/watermark_manager.go:34</code> | <code>func NewWatermarkManager(</code> |
| <code>interface.7.go-pendingsnapshot-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/utils.go:55</code> | <code>type PendingSnapshot interface</code> |
| <code>interface.7.go-pendingtx-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/utils.go:65</code> | <code>type PendingTx struct</code> |
| <code>interface.7.go-sequenceraccepter-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/send.go:124</code> | <code>type SequencerAccepter interface</code> |
| <code>interface.7.go-sequenceroption-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/server.go:48</code> | <code>type SequencerOption struct</code> |
| <code>interface.7.go-sequencerreceiptoption-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/sequencer_receipt.go:100</code> | <code>type SequencerReceiptOption struct</code> |
| <code>interface.7.go-servedmethods-j38</code> | <code>go_exported</code> | <code>new</code> | <code>rpc/served_methods.go</code> (planned) | <code>func ServedMethods(apis []rpc.API) []string</code> |
| <code>interface.7.go-tracecontextprovider-j38</code> | <code>go_exported</code> | <code>stable</code> | <code>rpc/simulate.go:245</code> | <code>type TraceContextProvider func(int64) (sdk.Context, func())</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.go-associaterequest-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-blockheadernotifier-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-connectiontypehttp-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 2 use(s) outside rpc/ |
| <code>interface.7.go-evmserver-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 5 use(s) outside rpc/ |
| <code>interface.7.go-httpconfig-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-httpserver-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 2 use(s) outside rpc/ |
| <code>interface.7.go-httpserver-servedmethods-j38</code> | notes | Served sets of the lists registered by EnableRPC and EnableWS; task 7.0.1 |
| <code>interface.7.go-latestctxheight-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 4 use(s) outside rpc/ |
| <code>interface.7.go-localaddress-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 9 use(s) outside rpc/, first 8 listed |
| <code>interface.7.go-newblockapi-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-newblockcache-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-newblockheadernotifier-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 2 use(s) outside rpc/ |
| <code>interface.7.go-newechoapi-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 2 use(s) outside rpc/ |
| <code>interface.7.go-newevmhttpserver-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-newevmwebsocketserver-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-newhttpserver-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 2 use(s) outside rpc/ |
| <code>interface.7.go-newpaxblockapi-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-newsequenceroption-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 0 use(s) outside rpc/ |
| <code>interface.7.go-newwatermarkmanager-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-pendingsnapshot-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 0 use(s) outside rpc/ |
| <code>interface.7.go-pendingtx-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 1 use(s) outside rpc/ |
| <code>interface.7.go-sequenceraccepter-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 0 use(s) outside rpc/ |
| <code>interface.7.go-sequenceroption-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 0 use(s) outside rpc/ |
| <code>interface.7.go-sequencerreceiptoption-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 0 use(s) outside rpc/ |
| <code>interface.7.go-servedmethods-j38</code> | notes | Lists ns_method names the geth server dispatches, promoted methods included; task 7.0.1 |
| <code>interface.7.go-tracecontextprovider-j38</code> | notes | Exported rpc API used outside rpc/; J38 keeps its signature; 4 use(s) outside rpc/ |

Contract context <code>contract_context.7.lx-switch-off</code>:

| Field | Frozen rule |
|---|---|
| interface | No lx_* method is registered by the node; every lx_* name answers -32601 on HTTP and WS (decision 701). The gateway switch-off belongs to domain 8. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | No served name starts with lx_. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.served-methods</code>:

| Field | Frozen rule |
|---|---|
| interface | The geth server registers every exported method of a service whose results are at most two with a trailing error (core/deps/geth/rpc/service.go); ServedMethods reproduces that rule so tests read the exact HTTP and WS sets. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Served sets are a pure function of the registered rpc.API lists; the goldens change only with a registration change. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.1.1</code> | req | 7001.1 |
| <code>acceptance.7.1.1</code> | scenario | Given an rpc.API list holding only the pax PaxBlockAPI service, when ServedMethods is called, then the result contains pax_getBlockByNumber and pax_getBlockByNumberExcludeTraceFail and every name starts with pax_ |
| <code>acceptance.7.1.1</code> | test | rpc/served_methods_test.go::TestServedMethodsIncludesPromotedMethods (planned) |
| <code>acceptance.7.1.1</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_1 | ServedMethods in rpc/served_methods.go (planned) lists pax_getBlockByNumber, which PaxBlockAPI inherits from BlockAPI (rpc/block.go:210), next to its own pax_getBlockByNumberExcludeTraceFail. |
| <code>acceptance.7.1.2</code> | req | 7001.2 |
| <code>acceptance.7.1.2</code> | scenario | Given the HTTP server started on TestPort by the package test setup, when each name returned by HTTPMethods is called with an empty parameter list, then no answer carries error code -32601 |
| <code>acceptance.7.1.2</code> | test | rpc/served_methods_test.go::TestServedMethodsMatchServerDispatch (planned) |
| <code>acceptance.7.1.2</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_2 | Every name HTTPServer.HTTPMethods (planned) returns for the test server is dispatched by the geth server, so a call with no parameters never answers -32601 (method not found), matching the callback rule at core/deps/geth/rpc/service.go:135. |
| <code>acceptance.7.1.3</code> | req | 7001.3 |
| <code>acceptance.7.1.3</code> | scenario | Given the HTTP server started by the package test setup, when HTTPMethods is read and compared with rpc/testdata/served_methods_http.golden, then both lists are equal |
| <code>acceptance.7.1.3</code> | test | rpc/served_methods_golden_test.go::TestServedMethodsHTTPGolden (planned) |
| <code>acceptance.7.1.3</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_3 | The HTTP server answers exactly the names in rpc/testdata/served_methods_http.golden (planned), the list registered at rpc/server.go:214 after the retirements of tasks 7.2.1, 7.2.2, 7.2.3 and 7.3.1. |
| <code>acceptance.7.1.4</code> | req | 7001.4 |
| <code>acceptance.7.1.4</code> | scenario | Given the WebSocket server started by the package test setup, when WSMethods is read and compared with rpc/testdata/served_methods_ws.golden, then both lists are equal |
| <code>acceptance.7.1.4</code> | test | rpc/served_methods_golden_test.go::TestServedMethodsWSGolden (planned) |
| <code>acceptance.7.1.4</code> | qualification | UNRUN |
| <code>req.7001</code> | ac_4 | The WebSocket server answers exactly the names in rpc/testdata/served_methods_ws.golden (planned), the list registered at rpc/server.go:392 plus the raw transaction, uncle and eth filter services of task 7.9.2. |
| <code>acceptance.7.9.3</code> | req | 7009.3 |
| <code>acceptance.7.9.3</code> | scenario | Given the HTTP and WebSocket servers of the package test setup, when eth_getRawTransactionByHash and eth_getUncleCountByBlockNumber are called on both, then neither answer carries -32601 |
| <code>acceptance.7.9.3</code> | test | rpc/server_registration_test.go::TestRawAndUncleMethodsServedHTTPAndWS (planned) |
| <code>acceptance.7.9.3</code> | qualification | UNRUN |
| <code>req.7009</code> | ac_3 | eth_getRawTransactionByHash, eth_getRawTransactionByBlockNumberAndIndex, eth_getRawTransactionByBlockHashAndIndex, the four eth_getUncle names and eth_sendRawTransactionSync are served on HTTP and WebSocket after registration in the lists at rpc/server.go:214 and rpc/server.go:392. |
| <code>acceptance.7.9.4</code> | req | 7009.4 |
| <code>acceptance.7.9.4</code> | scenario | Given the HTTP and WebSocket servers of the package test setup, when eth_getLogs over block 2 is called on both, then both return the same log list |
| <code>acceptance.7.9.4</code> | test | rpc/server_registration_test.go::TestWSServesEthGetLogs (planned) |
| <code>acceptance.7.9.4</code> | qualification | UNRUN |
| <code>req.7009</code> | ac_4 | The WebSocket server, which registers no FilterAPI at HEAD (rpc/server.go:392), answers eth_getLogs with the same logs the HTTP server returns. |
| <code>acceptance.7.9.5</code> | req | 7009.5 |
| <code>acceptance.7.9.5</code> | scenario | Given the HTTP server of the package test setup, when HTTPMethods is read, then pax_newPendingTransactionFilter is absent and pax_getLogs is present |
| <code>acceptance.7.9.5</code> | test | rpc/server_registration_test.go::TestPaxNewPendingTransactionFilterNotServed (planned) |
| <code>acceptance.7.9.5</code> | qualification | UNRUN |
| <code>req.7009</code> | ac_5 | pax_newPendingTransactionFilter, registered at HEAD through the pax FilterAPI at rpc/server.go:269, is no longer in HTTPMethods, while the six gated pax filter names stay. |
| <code>acceptance.7.999.1</code> | req | 7999.1 |
| <code>acceptance.7.999.1</code> | scenario | Given the candidate after every domain-7 wave-6 task, when TestRpcCompletionRetiredSurfacesAbsent runs, then no retired surface remains and every planned path exists |
| <code>acceptance.7.999.1</code> | test | rpc/completion_gate_test.go::TestRpcCompletionRetiredSurfacesAbsent (planned) |
| <code>acceptance.7.999.1</code> | qualification | UNRUN |
| <code>req.7999</code> | ac_1 | TestRpcCompletionRetiredSurfacesAbsent passes on the merged candidate: no retired domain-7 surface remains and every planned domain-7 wave-6 path exists. |
| <code>capability.7.0.9</code> | title | Domain 7 completion gate |
| <code>capability.7.0.9</code> | reqs | 7999.1 |
| <code>resolution.17.763</code> | title | decision.701 binds: lx_* at HEAD |
| <code>resolution.17.763</code> | applies_to | decision.701<br>task.7.0.2<br>task.7.16.1 |
| <code>resolution.17.763</code> | source | decision.701 in fragments/J38/decision.kvx; tasks from affects |

## test-incrementpointerversion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-test-incrementpointerversion</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/test.go:15</code> | <code>func (a *TestAPI) IncrementPointerVersion(pointerType string, offset int16) error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-test-incrementpointerversion</code> | notes | test_incrementPointerVersion; HTTP only; registered only when evm.enable_test_api is set and the chain id is not live (rpc/server.go:293); no in-repo caller |

## tracing

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.18.1</code> | req | 7018.1 |
| <code>acceptance.7.18.1</code> | scenario | Given the HTTP server of the package test setup, when debug_getRawTransaction is called with a committed transaction hash, then the result decodes to a transaction with that hash |
| <code>acceptance.7.18.1</code> | test | rpc/debug_raw_tx_test.go::TestDebugGetRawTransactionReturnsEnvelope (planned) |
| <code>acceptance.7.18.1</code> | qualification | UNRUN |
| <code>req.7018</code> | ac_1 | debug_getRawTransaction returns the binary envelope of a committed transaction instead of the ErrEVMNotSupported answer at rpc/tracers.go:739. |
| <code>acceptance.7.18.2</code> | req | 7018.2 |
| <code>acceptance.7.18.2</code> | scenario | Given the HTTP server of the package test setup and a committed contract call, when debug_traceTransaction is called with tracer callTracer, then the result has type CALL, from, to, gas, gasUsed, input and output |
| <code>acceptance.7.18.2</code> | test | rpc/debug_tracer_shape_test.go::TestDebugTraceTransactionCallTracerShape (planned) |
| <code>acceptance.7.18.2</code> | qualification | UNRUN |
| <code>req.7018</code> | ac_2 | debug_traceTransaction (rpc/tracers.go:282) with callTracer returns the geth call frame shape. |
| <code>acceptance.7.18.3</code> | req | 7018.3 |
| <code>acceptance.7.18.3</code> | scenario | Given the HTTP server of the package test setup and a committed contract call, when debug_traceTransaction is called with tracer prestateTracer, then the result maps the sender and the contract to objects carrying balance and nonce |
| <code>acceptance.7.18.3</code> | test | rpc/debug_tracer_shape_test.go::TestDebugTraceTransactionPrestateTracerShape (planned) |
| <code>acceptance.7.18.3</code> | qualification | UNRUN |
| <code>req.7018</code> | ac_3 | debug_traceTransaction (rpc/tracers.go:282) with prestateTracer returns the geth prestate map. |
| <code>acceptance.7.18.4</code> | req | 7018.4 |
| <code>acceptance.7.18.4</code> | scenario | Given the HTTP server of the package test setup, when debug_traceBlockByNumber and eth_getBlockByNumber are called for the multi transaction block, then the entry txHash list equals the block transaction list |
| <code>acceptance.7.18.4</code> | test | rpc/debug_trace_block_test.go::TestDebugTraceBlockByNumberEntriesMatchBlockTxs (planned) |
| <code>acceptance.7.18.4</code> | qualification | UNRUN |
| <code>req.7018</code> | ac_4 | debug_traceBlockByNumber (rpc/tracers.go:639) returns one entry per EVM transaction of the block, in block order, each with the transaction hash. |

## transport-limits

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.13.3</code> | req | 7013.3 |
| <code>acceptance.7.13.3</code> | scenario | Given a limiter with rps 1 and burst 2, when three requests from one remote address arrive at once, then the first two are admitted and the third is refused |
| <code>acceptance.7.13.3</code> | test | rpc/ip_rate_limit_test.go::TestIPRateLimiterRefusesAboveBurst (planned) |
| <code>acceptance.7.13.3</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_3 | ipRateLimiter (planned) admits burst requests from one address and refuses the next until the rate refills, enforcing evm.ip_rate_limit_rps and evm.ip_rate_limit_burst (rpc/config/config.go:170), which nothing reads at HEAD. |
| <code>acceptance.7.13.4</code> | req | 7013.4 |
| <code>acceptance.7.13.4</code> | scenario | Given a limiter with rps 1 and burst 1 that refused a first remote address, when a second remote address sends one request and a limiter built with rps 0 is asked, then the second address is admitted and the rps 0 constructor returns nil |
| <code>acceptance.7.13.4</code> | test | rpc/ip_rate_limit_test.go::TestIPRateLimiterKeysByRemoteIP (planned) |
| <code>acceptance.7.13.4</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_4 | ipRateLimiter (planned) keys by the RemoteAddr host so a different address keeps its own budget, and rps 0 disables limiting as documented at rpc/config/config.go:170. |
| <code>acceptance.7.13.5</code> | req | 7013.5 |
| <code>acceptance.7.13.5</code> | scenario | Given an HTTP server built with IPRateLimitRPS 1 and IPRateLimitBurst 1, when two eth_chainId requests are sent at once from the loopback address, then the first returns a result and the second returns status 429 with error code -32005 |
| <code>acceptance.7.13.5</code> | test | rpc/transport_limits_test.go::TestHTTPRateLimitAnswers429 (planned) |
| <code>acceptance.7.13.5</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_5 | An address over evm.ip_rate_limit_burst is refused with HTTP 429 and JSON-RPC error -32005 before the request reaches the handler built at rpc/rpcstack.go:322. |
| <code>acceptance.7.13.6</code> | req | 7013.6 |
| <code>acceptance.7.13.6</code> | scenario | Given an HTTP server built with BatchRequestLimit 2, when a batch of three eth_chainId calls is sent, then the answer is a batch-too-large error and a batch of two returns two results |
| <code>acceptance.7.13.6</code> | test | rpc/transport_limits_test.go::TestBatchRequestLimitEnforced (planned) |
| <code>acceptance.7.13.6</code> | qualification | UNRUN |
| <code>req.7013</code> | ac_6 | evm.batch_request_limit reaches SetBatchLimits (rpc/rpcstack.go:312), whose limits are never set at HEAD, so a batch above the limit is refused. |

## txpool-content

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-txpool-content</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>rpc/txpool.go:40</code> | <code>func (t *TxPoolAPI) Content(ctx context.Context) (result map[string]map[string]map[string]*export.RPCTransaction, returnErr error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-txpool-content</code> | notes | txpool_content; HTTP only; bounded by evm.max_tx_pool_txs (rpc/server.go:277); 6 call site(s) in 3 file(s) |
| <code>interface.7.rpc-txpool-content</code> | overridden_by | override.J38.18 |

Contract context <code>contract_context.7.client-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Methods called by in-repo clients and the go-ethereum client are served on the transport the client uses. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Every method in the client matrix is served; uncle methods answer null or 0x0 because blocks carry no uncles. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## web3-clientversion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rpc-web3-clientversion</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>rpc/web3.go:7</code> | <code>func (w *Web3API) ClientVersion() string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rpc-web3-clientversion</code> | notes | web3_clientVersion; HTTP and WS; 12 call site(s) in 12 file(s) |

## wiring-integration

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.961.1</code> | req | 7961.1 |
| <code>acceptance.7.961.1</code> | scenario | Given the HTTP JSON-RPC server with a sequencer receipt source holding no entry, when pax_sequencerReceipt is called with a random hash, then the JSON result is null and there is no error. |
| <code>acceptance.7.961.1</code> | test | rpc/sequencer_receipt_hosted_test.go::TestSequencerReceiptUnknownTxIsNull (planned) |
| <code>acceptance.7.961.1</code> | qualification | UNRUN |
| <code>req.7961</code> | ac_1 | pax_sequencerReceipt answers JSON null for an unknown transaction on the HTTP listener. |
| <code>acceptance.7.961.2</code> | req | 7961.2 |
| <code>acceptance.7.961.2</code> | scenario | Given a source holding a signed receipt for transaction t, when pax_sequencerReceipt is called with t, then the result carries the receipt bytes and signingKey, Verify returns nil, the signature checks under signingKey, and changing one byte of signingKey makes Verify fail. |
| <code>acceptance.7.961.2</code> | test | rpc/sequencer_receipt_hosted_test.go::TestSequencerReceiptCarriesSigningKey (planned) |
| <code>acceptance.7.961.2</code> | qualification | UNRUN |
| <code>req.7961</code> | ac_2 | A receipted answer carries the signed receipt bytes, the signing key and KeyID equal to the SequencerID pinned for the receipt height (node/sequencer/receiptkey.go:91-100), so a verifier with a pinned key list accepts it. |

## ws-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.ws-endpoint-root</code> | <code>ws_route</code> | <code>change</code> | <code>rpc/rpcstack.go:211</code> | <code>func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.ws-endpoint-root</code> | notes | WebSocket JSON-RPC on evm.ws_port, upgraded in ServeHTTP when the path matches the WS prefix (rpc/rpcstack.go:213-220); the listener binds LocalAddress the all-interfaces address (rpc/server.go:361) instead of evm.http_address, origins from evm.ws_origins (rpc/server.go:435); the deny list is applied only on HTTP (rpc/rpcstack.go:317-319) so evm.deny_list does not reach WS; services echo, eth Block/Transaction/State/Info/Send/Simulation/Subscription, net, web3 (rpc/server.go:392-433) |

Contract context <code>contract_context.7.served-methods</code>:

| Field | Frozen rule |
|---|---|
| interface | The geth server registers every exported method of a service whose results are at most two with a trailing error (core/deps/geth/rpc/service.go); ServedMethods reproduces that rule so tests read the exact HTTP and WS sets. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | Served sets are a pure function of the registered rpc.API lists; the goldens change only with a registration change. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

Contract context <code>contract_context.7.ws-transport</code>:

| Field | Frozen rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug_, txpool_ and pax_ stay HTTP-only (decision 708). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

## ws-logs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.ws-eth-subscribe-logs</code> | <code>ws_route</code> | <code>change</code> | <code>rpc/subscribe.go:230</code> | <code>func (a *SubscriptionAPI) Logs(ctx context.Context, filter *filters.FilterCriteria) (s *rpc.Subscription, _err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.ws-eth-subscribe-logs</code> | notes | eth_subscribe logs; WS; bounded by evm.filter_timeout, evm.max_log_no_block and evm.max_blocks_for_log (rpc/server.go:385); 3 caller site(s) |
| <code>interface.7.ws-eth-subscribe-logs</code> | overridden_by | override.J38.20 |

Contract context <code>contract_context.7.newheads</code>:

| Field | Frozen rule |
|---|---|
| interface | newHeads carries the header keys of eth_getBlockByNumber with stateRoot from the header AppHash and bloom and gasUsed from EVM receipts; logs subscriptions end instead of sending an error as a log. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | header.Hash() of a received head equals the eth_getBlockByNumber hash of the same height. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.795</code> | title | decision.733 binds: Logs subscription lifetime |
| <code>resolution.17.795</code> | applies_to | decision.733<br>task.7.10.4<br>task.7.10.5 |
| <code>resolution.17.795</code> | source | decision.733 in fragments/J68/decision.kvx; tasks from affects |

## ws-new-heads

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.ws-eth-subscribe-newheads</code> | <code>ws_route</code> | <code>change</code> | <code>rpc/subscribe.go:185</code> | <code>func (a *SubscriptionAPI) NewHeads(ctx context.Context) (s *rpc.Subscription, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.ws-eth-subscribe-newheads</code> | notes | eth_subscribe newHeads; WS; capped by evm.max_subscriptions_new_head (rpc/server.go:385); node/app.go:556 pairs FinalizeBlock with Commit for the head event and node/newheads_test.go:19 covers the stale-head clear; 10 caller site(s) |
| <code>interface.7.ws-eth-subscribe-newheads</code> | overridden_by | override.J38.19 |

Contract context <code>contract_context.7.newheads</code>:

| Field | Frozen rule |
|---|---|
| interface | newHeads carries the header keys of eth_getBlockByNumber with stateRoot from the header AppHash and bloom and gasUsed from EVM receipts; logs subscriptions end instead of sending an error as a log. |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | header.Hash() of a received head equals the eth_getBlockByNumber hash of the same height. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.767</code> | title | decision.705 binds: newHeads stateRoot |
| <code>resolution.17.767</code> | applies_to | decision.705<br>task.7.10.3 |
| <code>resolution.17.767</code> | source | decision.705 in fragments/J38/decision.kvx; tasks from affects |
| <code>resolution.17.768</code> | title | decision.706 binds: newHeads field set |
| <code>resolution.17.768</code> | applies_to | decision.706<br>task.7.10.1<br>task.7.10.2 |
| <code>resolution.17.768</code> | source | decision.706 in fragments/J38/decision.kvx; tasks from affects |

## ws-new-pending-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.ws-eth-subscribe-newpendingtransactions</code> | <code>ws_route</code> | <code>stable</code> | <code>rpc/send.go:357</code> | <code>func (s *SendAPI) NewPendingTransactions(ctx context.Context) (sub *rpc.Subscription, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.ws-eth-subscribe-newpendingtransactions</code> | notes | eth_subscribe newPendingTransactions; registered on HTTP and WS through SendAPI; refused with ErrPendingServedBySequencer off the sequencer (rpc/send.go:362-365) and with notifications not supported on HTTP (rpc/send.go:366-368); 1 caller site(s) |

## ws-transport

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.go-wsconfig-j38</code> | <code>go_exported</code> | <code>change</code> | <code>rpc/rpcstack.go:53</code> | <code>type WsConfig struct</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.go-wsconfig-j38</code> | notes | Gains DenyList applied in EnableWS; task 7.9.1; 1 use(s) outside rpc/ |

Contract context <code>contract_context.7.ws-transport</code>:

| Field | Frozen rule |
|---|---|
| interface | The WebSocket listener binds evm.http_address and applies evm.deny_list as HTTP does; debug_, txpool_ and pax_ stay HTTP-only (decision 708). |
| storage | None; the rpc layer reads keeper, receipt store and consensus state and writes no chain state. |
| errors | JSON-RPC errors per contracts/evm-rpc.md#error-codes; -32601 for any name not served. |
| events | None emitted on chain; WebSocket subscriptions deliver notifications only. |
| gas | No gas is charged by the rpc layer; eth_call and eth_estimateGas simulate with the block gas limit. |
| activation | Takes effect when the node binary carrying the change starts; no upgrade height. |
| invariants | For a method served on both transports the answers are equal for equal parameters. |
| pause | evm.http_enabled and evm.ws_enabled switch the transports off; evm.deny_list refuses single methods. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.9.1</code> | req | 7009.1 |
| <code>acceptance.7.9.1</code> | scenario | Given a WebSocket server built with the loopback address as HTTPAddress on a free port, when its listen address is read after Start, then the host is the loopback address |
| <code>acceptance.7.9.1</code> | test | rpc/ws_transport_test.go::TestWSBindsHTTPAddress (planned) |
| <code>acceptance.7.9.1</code> | qualification | UNRUN |
| <code>req.7009</code> | ac_1 | The WebSocket listener binds evm.http_address instead of the all-interfaces LocalAddress used at rpc/server.go:361. |
| <code>acceptance.7.9.2</code> | req | 7009.2 |
| <code>acceptance.7.9.2</code> | scenario | Given a WebSocket server built with DenyList eth_chainId, when eth_chainId is called over a WebSocket connection, then the answer is an error and eth_blockNumber still returns a result |
| <code>acceptance.7.9.2</code> | test | rpc/ws_transport_test.go::TestWSAppliesDenyList (planned) |
| <code>acceptance.7.9.2</code> | qualification | UNRUN |
| <code>req.7009</code> | ac_2 | A method in evm.deny_list is refused over WebSocket as it is over HTTP at rpc/rpcstack.go:317. |
| <code>acceptance.7.9.6</code> | req | 7009.6 |
| <code>acceptance.7.9.6</code> | scenario | Given the HTTP and WebSocket servers of the package test setup, when ten shared read methods are called with the same parameters on both, then every pair of results is equal |
| <code>acceptance.7.9.6</code> | test | rpc/http_ws_parity_test.go::TestHTTPAndWSAnswersMatch (planned) |
| <code>acceptance.7.9.6</code> | qualification | UNRUN |
| <code>req.7009</code> | ac_6 | For every read method served on both transports, the WebSocket answer equals the HTTP answer for the same parameters, using the helpers at rpc/setup_test.go:1224 and rpc/setup_test.go:1280. |
