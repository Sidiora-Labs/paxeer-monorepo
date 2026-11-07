# Node RPC and module query contracts

Paxeer mainnet-beta is one BFT network, served by `paxd`, with EVM chain ID
`125` (`0x7d`). The kernel is part of the network and proposes blocks. A
transaction receipt is final when its block is committed. Sub-chains are
tenant domains in the database and do not introduce another public chain ID.

This page documents node access and the module queries referenced by the
interface registry. See [JavaScript SDK](sdk-js.md) for the native-module
client and [verifier HTTP](verifier-http.md) for explorer source verification.
Source verification compares compiled code; it does not establish consensus
finality.

## Public JSON-RPC

Use the root of `https://api-mainnet-beta.paxeer.network`, or a numbered node
from `https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. WebSocket URLs use `wss://` with
the same host and root path. The [public endpoint reference](../docs/site/docs/reference/public-rpc.md)
lists each URL and the node proxy's deployment settings.

```bash
curl --max-time 15 https://api1.mainnet-beta.paxeer.network \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Expect `result: "0x7d"`. Check the chain before signing. Keep multiple nodes
configured, compare `eth_blockNumber`, and set client deadlines. After a
broadcast timeout, look up the original transaction hash before resubmitting.

The public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test`
and `txpool`; this is not a promise of every upstream Geth method. The
implementation registers APIs in `rpc/server.go`; deployment configuration
controls exposure. The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in [the gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).

The pre-merge router and its prefixed endpoints are retired.
`node.hyperpaxeer.com` is the hpx library mirror, never an RPC endpoint.

## Native module transport

The module services below are protobuf gRPC services, with HTTP GET bindings
on the node's configured Cosmos REST/gRPC-gateway listener. Do not append
these paths to the public EVM JSON-RPC URL and assume that listener is exposed.
Obtain the REST or gRPC address from the node operator. `node/app.go` mounts
module routes through `ModuleBasics.RegisterRESTRoutes` and
`ModuleBasics.RegisterGRPCGatewayRoutes`; each module registers its query
implementation in `RegisterServices`.

A gRPC method has the form `/<package>.Query/<Method>`. Tables give the
package, exact HTTP binding, request fields and response fields. `{field}`
denotes a path parameter; other request fields are query parameters. Use the
protobuf definitions linked below for nested message fields. The gateway
uses its protobuf JSON marshaler: byte fields use base64 and 64-bit integers
use decimal strings. Native amounts and decimals are strings, distinct from
the hexadecimal quantities used by EVM JSON-RPC. Do not treat a REST response
as an already decoded SDK protobuf object.

Transport errors and module errors are not successful empty responses. For
example, tokenfactory metadata queries return gRPC `InvalidArgument` (3) for
an empty denom and `NotFound` (5) when metadata is absent. Other keepers also
return SDK errors; retain the actual status and message instead of assigning
every failure one code. Address and pointer lookups explicitly represent
absence with `associated: false` or `exists: false`.

## Epoch queries

Package `paxprotocol.paxchain.epoch`, from
[epoch/query.proto](../api/epoch/query.proto). Requests are empty.
`Epoch` includes genesis/start timestamps, duration, epoch number and height;
the current `Params` message is empty.

| Method | HTTP GET | Response |
| --- | --- | --- |
| <a id="qry-epoch-epoch"></a><a id="rest-epoch-epoch"></a>`Epoch` | `/pax-protocol/paxchain/epoch/epoch` | `epoch` |
| <a id="qry-epoch-params"></a><a id="rest-epoch-params"></a>`Params` | `/pax-protocol/paxchain/epoch/params` | `params` |

## Mint queries

Package `paxprotocol.paxchain.mint`, from
[mint/v1beta1/query.proto](../api/mint/v1beta1/query.proto). This is the chain's
custom mint module, distinct from the retired Cosmos mint SDK codecs.
Requests are empty.

| Method | HTTP GET | Response |
| --- | --- | --- |
| <a id="qry-mint-params"></a><a id="rest-mint-params"></a>`Params` | `/paxchain/mint/v1beta1/params` | `params` |
| <a id="qry-mint-minter"></a><a id="rest-mint-minter"></a>`Minter` | `/paxchain/mint/v1beta1/minter` | `start_date`, `end_date`, `denom`, `total_mint_amount`, `remaining_mint_amount`, `last_mint_amount`, `last_mint_date`, `last_mint_height` |

## Oracle queries

Package `paxprotocol.paxchain.oracle`, from
[oracle/query.proto](../api/oracle/query.proto). Its HTTP prefix contains
`pax-chain`, unlike epoch, EVM and tokenfactory. Omitted request fields below
mean an empty request. Nested oracle values are defined in
[oracle.proto](../api/oracle/oracle.proto).

| Method | HTTP GET | Request | Response |
| --- | --- | --- | --- |
| <a id="qry-oracle-exchangerate"></a><a id="rest-oracle-exchangerate"></a>`ExchangeRate` | `/pax-protocol/pax-chain/oracle/denoms/{denom}/exchange_rate` | `denom` | `oracle_exchange_rate` |
| <a id="qry-oracle-exchangerates"></a><a id="rest-oracle-exchangerates"></a>`ExchangeRates` | `/pax-protocol/pax-chain/oracle/denoms/exchange_rates` | — | `denom_oracle_exchange_rate_pairs[]` |
| <a id="qry-oracle-actives"></a><a id="rest-oracle-actives"></a>`Actives` | `/pax-protocol/pax-chain/oracle/denoms/actives` | — | `actives[]` |
| <a id="qry-oracle-votetargets"></a><a id="rest-oracle-votetargets"></a>`VoteTargets` | `/pax-protocol/pax-chain/oracle/denoms/vote_targets` | — | `vote_targets[]` |
| <a id="qry-oracle-pricesnapshothistory"></a><a id="rest-oracle-pricesnapshothistory"></a>`PriceSnapshotHistory` | `/pax-protocol/pax-chain/oracle/denoms/price_snapshot_history` | — | `price_snapshots[]` |
| <a id="qry-oracle-twaps"></a><a id="rest-oracle-twaps"></a>`Twaps` | `/pax-protocol/pax-chain/oracle/denoms/twaps/{lookback_seconds}` | `lookback_seconds` (uint64) | `oracle_twaps[]` |
| <a id="qry-oracle-feederdelegation"></a><a id="rest-oracle-feederdelegation"></a>`FeederDelegation` | `/pax-protocol/pax-chain/oracle/validators/{validator_addr}/feeder` | `validator_addr` | `feeder_addr` |
| <a id="qry-oracle-votepenaltycounter"></a><a id="rest-oracle-votepenaltycounter"></a>`VotePenaltyCounter` | `/pax-protocol/pax-chain/oracle/validators/{validator_addr}/vote_penalty_counter` | `validator_addr` | `vote_penalty_counter` |
| <a id="qry-oracle-slashwindow"></a><a id="rest-oracle-slashwindow"></a>`SlashWindow` | `/pax-protocol/pax-chain/oracle/slash_window` | — | `window_progress` |
| <a id="qry-oracle-params"></a><a id="rest-oracle-params"></a>`Params` | `/pax-protocol/pax-chain/oracle/params` | — | `params` |

## EVM module queries

Package `paxprotocol.paxchain.evm`, from
[evm/query.proto](../api/evm/query.proto); implementation:
[grpc_query.go](../modules/evm/keeper/grpc_query.go). These are native query
methods, not `eth_` JSON-RPC method names.

| Method | HTTP GET | Request | Response |
| --- | --- | --- | --- |
| <a id="qry-evm-paxaddressbyevmaddress"></a><a id="rest-evm-paxaddressbyevmaddress"></a>`PaxAddressByEVMAddress` | `/pax-protocol/paxchain/evm/pax_address` | `evm_address` | `pax_address`, `associated` |
| <a id="qry-evm-evmaddressbypaxaddress"></a><a id="rest-evm-evmaddressbypaxaddress"></a>`EVMAddressByPaxAddress` | `/pax-protocol/paxchain/evm/evm_address` | `pax_address` | `evm_address`, `associated` |
| <a id="qry-evm-staticcall"></a><a id="rest-evm-staticcall"></a>`StaticCall` | `/pax-protocol/paxchain/evm/static_call` | `to`, `data` (bytes) | `data` (bytes) |
| <a id="qry-evm-pointer"></a><a id="rest-evm-pointer"></a>`Pointer` | `/pax-protocol/paxchain/evm/pointer` | `pointer_type`, `pointee` | `pointer`, `version`, `exists` |
| <a id="qry-evm-pointerversion"></a><a id="rest-evm-pointerversion"></a>`PointerVersion` | `/pax-protocol/paxchain/evm/pointer_version` | `pointer_type` | `version`, `cw_code_id` |
| <a id="qry-evm-pointee"></a><a id="rest-evm-pointee"></a>`Pointee` | `/pax-protocol/paxchain/evm/pointee` | `pointer_type`, `pointer` | `pointee`, `version`, `exists` |

`PointerType` values are `ERC20=0`, `ERC721=1`, `NATIVE=2`, `CW20=3`,
`CW721=4`, `ERC1155=5`, `CW1155=6` in
[enums.proto](../api/evm/enums.proto). Address lookups require a nonempty
input; the Pax address parser validates Bech32. A static call requires `to`
and cannot create a contract. Pointer and pointee requests require their
respective input identifiers; unsupported pointer types return an error.

## Tokenfactory queries

Package `paxprotocol.paxchain.tokenfactory`, from
[tokenfactory/query.proto](../api/tokenfactory/query.proto); implementation:
[grpc_query.go](../modules/tokenfactory/keeper/grpc_query.go).

| Method | HTTP GET | Request | Response |
| --- | --- | --- | --- |
| <a id="qry-tokenfactory-params"></a><a id="rest-tokenfactory-params"></a>`Params` | `/pax-protocol/paxchain/tokenfactory/params` | empty | `params` |
| <a id="qry-tokenfactory-denomauthoritymetadata"></a><a id="rest-tokenfactory-denomauthoritymetadata"></a>`DenomAuthorityMetadata` | `/pax-protocol/paxchain/tokenfactory/denoms/{denom}/authority_metadata` | `denom` | `authority_metadata` |
| <a id="qry-tokenfactory-denommetadata"></a><a id="rest-tokenfactory-denommetadata"></a>`DenomMetadata` | `/pax-protocol/paxchain/tokenfactory/denoms/metadata` | `denom` | `metadata` |
| <a id="qry-tokenfactory-denomsfromcreator"></a><a id="rest-tokenfactory-denomsfromcreator"></a>`DenomsFromCreator` | `/pax-protocol/paxchain/tokenfactory/denoms_from_creator/{creator}` | `creator`, optional `pagination` | `denoms[]`, `pagination` |
| <a id="qry-tokenfactory-denomallowlist"></a><a id="rest-tokenfactory-denomallowlist"></a>`DenomAllowList` | `/pax-protocol/paxchain/tokenfactory/denoms/allow_list` | `denom` | `allow_list` |
| `DenomPaused` | `/pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused` | `denom` | `paused`, `issuer_did` |

Slash-containing denoms can use the query-only additional bindings
`/pax-protocol/paxchain/tokenfactory/denom_authority_metadata?denom=...`
and `/pax-protocol/paxchain/tokenfactory/denom_paused?denom=...`.
URL-encode parameter values. Pagination uses the SDK `PageRequest` and
`PageResponse`; keep opaque page keys and request subsequent pages until
`next_key` is empty. `DenomPaused` reports `NotFound` for missing authority
metadata; do not interpret that as an unpaused registered denom.

## Governance proposal REST bindings

These legacy REST handlers generate unsigned governance transactions. They
do not sign, broadcast or enact a proposal. Both are mounted through the
governance proposal handlers in `node/app.go`.

| Anchor | POST route | Implementation |
| --- | --- | --- |
| <a id="rest-layerxbridge-gov-proposal"></a>Bridge proposal | `/gov/proposals/layerxbridge` | [layerxbridge/client/cli/tx.go](../modules/layerxbridge/client/cli/tx.go) |
| <a id="rest-layerxgov-gov-proposal"></a>Network governance proposal | `/gov/proposals/layerx` | [layerxgov/client/cli/tx.go](../modules/layerxgov/client/cli/tx.go) |

The JSON body has `base_req`, `deposit` (coin string), and `proposal` (the
typed proposal object, not a JSON string). The handlers validate the sender,
deposit and proposal content and return HTTP 400 on invalid input. Use the
proposal generator's complete output; unknown or missing content fields are
rejected. Sign and submit the resulting transaction through the normal node
transaction flow.

## Program query schema availability

[layerxprograms/query.proto](../api/layerxprograms/query.proto) declares
`paxprotocol.paxchain.layerxprograms.Query` and generated Go types under
`precompiles/layerxprograms/types`. It has no HTTP annotations. The current
tree does not mount an implementation of this generated query service;
these schema anchors do not advertise live HTTP or gRPC endpoints. The
active program implementation lives in the EVM precompile under
`precompiles/layerxprograms`.

### qry-layerxprograms-programstate

Declared request: `program_id`, `key` (bytes), `height` (int64), `prove`.
Response: `program_id`, `key`, `value`, `state_record`, `height`, `app_hash`,
`proof_ops`, `state_record_proof_ops`.

### qry-layerxprograms-programbalances

Declared request: `program_id`, `height`, `prove`, `pagination`. Response:
`program_id`, `balances[]`, `height`, `app_hash`, `pagination`, `state_record`,
`state_record_proof_ops`. Each balance includes account/asset/seed identifiers,
registration sequence/digest, `balance_be`, frozen state, account and binding
records and their proofs; preserve the byte encodings defined by the proto.

### qry-layerxprograms-programevents

Declared request: `program_id`, `from_height`, `to_height`, `pagination`,
`after_sequence`. Response: `events[]`, `pagination`, `complete_through`,
`scanned_through`. Event fields include height, log index, transaction hash,
topics, data, global sequence, ordinal, activity type and receipt digest.

## Local administration

The dedicated loopback-only gRPC server in [admin/server.go](../admin/server.go)
implements `paxprotocol.paxchain.admin.v0.AdminService`. It is not a public
node JSON-RPC namespace and has no HTTP bindings. The
[admin proto](../api/pax/admin/v0/admin.proto) and
[service implementation](../admin/service.go) define these messages:

| Method and message anchors | Request | Response |
| --- | --- | --- |
| <a id="grpcs-pax-admin-getloglevel"></a><a id="proto-pax-admin-getloglevelrequest"></a><a id="proto-pax-admin-getloglevelresponse"></a>`GetLogLevel` | `logger` | `logger`, `level` |
| <a id="grpcs-pax-admin-setloglevel"></a><a id="proto-pax-admin-setloglevelrequest"></a><a id="proto-pax-admin-setloglevelresponse"></a>`SetLogLevel` | `pattern`, `level` | `pattern`, `level`, `affected` (int32) |
| <a id="grpcs-pax-admin-listloggers"></a><a id="proto-pax-admin-listloggersrequest"></a><a id="proto-pax-admin-listloggersresponse"></a>`ListLoggers` | optional `prefix` | `loggers[]` |
| <a id="proto-pax-admin-loggerinfo"></a>`LoggerInfo` | — | `name`, `level` |

Empty logger/pattern or an invalid level returns `InvalidArgument` (3).
An unknown logger or unmatched pattern returns `NotFound` (5).
`ListLoggers` sorts names and filters by prefix. `SetLogLevel` accepts a name,
glob or `*`; the wildcard also sets the default level. Changes affect the
running process, not an on-chain governance parameter.

## Ethereum protobuf envelopes

These are native transaction message schemas in
[eth/tx.proto](../api/eth/tx.proto), not JSON-RPC payloads. Integer amount,
chain-ID and fee fields declared as strings must stay decimal strings;
signature/data fields are protobuf bytes. Preserve field numbers and use the
generated codec rather than constructing wire bytes manually.

| Message | Contents |
| --- | --- |
| <a id="proto-eth-accesstuple"></a>`AccessTuple` | `address`, repeated `storage_keys` |
| <a id="proto-eth-associatetx"></a>`AssociateTx` | `v`, `r`, `s`, `custom_message` |
| <a id="proto-eth-legacytx"></a>`LegacyTx` | nonce, gas price/limit, recipient, value, data, signature |
| <a id="proto-eth-accesslisttx"></a>`AccessListTx` | chain ID, nonce, gas price/limit, recipient, value, data, access list, signature |
| <a id="proto-eth-dynamicfeetx"></a>`DynamicFeeTx` | chain ID, nonce, tip/fee caps, gas limit, recipient, value, data, access list, signature |
| <a id="proto-eth-blobtx"></a>`BlobTx` | dynamic fee fields plus blob fee cap, blob hashes and sidecar |
| <a id="proto-eth-blobtxsidecar"></a>`BlobTxSidecar` | repeated byte arrays `blobs`, `commitments`, `proofs` |
| <a id="proto-eth-setcodeauthorization"></a>`SetCodeAuthorization` | `chain_id`, `address`, `nonce`, `v`, `r`, `s` |
| <a id="proto-eth-setcodetx"></a>`SetCodeTx` | dynamic fee fields plus `auth_list` of authorizations |
| <a id="proto-eth-extensionoptionsethereumtx"></a>`ExtensionOptionsEthereumTx` | empty extension marker |

The JavaScript package's [eth coverage](sdk-js.md#encoding-eth) is narrower
than this proto; in particular it does not export the set-code messages.

## Protobuf generation configuration

### fmt-api-buf-yaml

The former `api/buf.yaml` target is replaced by the root
[buf.yaml](../buf.yaml). It is a Buf v2 workspace containing `api`, SDK,
interchain, consensus and Wasm proto modules with `WIRE` and `WIRE_JSON`
breaking checks. See [API maintenance](../api/README.md).

### fmt-api-buf-gen-yaml

The former `api/buf.gen.yaml` target is replaced by the root
[buf.gen.yaml](../buf.gen.yaml). It configures gocosmos, gRPC-gateway and
gogofaster generation into `build/proto`. The maintained regeneration entry
point is `scripts/protoc.sh`, which copies generated files to their owners.
