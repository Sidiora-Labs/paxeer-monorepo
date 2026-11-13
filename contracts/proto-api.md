# Protobuf and REST API contract

The protobuf sources define typed node messages and services for Paxeer. Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`); the kernel is part of the network and proposes blocks. Receipts become final with block commitment. Module names beginning `layerx` are modules or precompile adapters in this network, not a second chain or public interface. Sub-chains are tenant namespaces in the database.

Public node JSON-RPC uses HTTPS at the configured node origin's root path. The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`. Protobuf gRPC and its HTTP bindings are separate node server transports configured by the operator; do not append an HTTP query path to an Ethereum endpoint and assume it is mounted there. The `px` namespace answers from node state; `px_register` and `px_requestFunds` are operations of the hosted identity and faucet services declared in [the gateway contract](platform-gateway.md).

## Sources and service registration

The root [api directory](../api) contains the native module definitions, including EVM, tokenfactory, launchpad, oracle, spot, perps, pricefeed, mint, xweb and the merged modules. SDK definitions live under [sdk/proto](../sdk/proto), interchain under [interchain/proto](../interchain/proto), consensus under [consensus/proto](../consensus/proto) and [consensus/internal](../consensus/internal), and Wasm under [wasm/proto](../wasm/proto).

Use each file's `package`, `go_package`, field numbers, scalar types, reserved names/numbers, custom gogoproto options and service signatures as the wire authority. Generated Go types may use custom numeric and address types; do not infer their serialized form from an application model. Protobuf JSON and Ethereum JSON-RPC hex quantities are different encodings. Keep integer precision, byte encoding, enum values and pagination semantics specified by the selected protobuf binding.

[node/app.go](../node/app.go), `RegisterAPIRoutes`, mounts legacy transaction routes, SDK transaction and Tendermint gRPC-gateway routes, then the module REST and gRPC-gateway registrations. A `.proto` declaration alone is not evidence of a running public route. Query services read state; `Msg` methods are transaction message contracts and remain subject to signing, authorization, transaction execution and commitment. Administrative services such as [pax/admin/v0/admin.proto](../api/pax/admin/v0/admin.proto) are not an invitation to expose runtime administration publicly.

The native [EVM transaction schema](../api/evm/tx.proto) defines `MsgEVMTransaction` and related operations, while the [program query schema](../api/layerxprograms/query.proto) defines state, balances and events with explicit height, proof and pagination material. The latter's `go_package` points to `precompiles/layerxprograms/types`; do not mechanically map every API directory to a `modules` package. Program event `complete_through` and `scanned_through` values describe query coverage and must survive client adaptation.

## REST denom bindings

[tokenfactory/query.proto](../api/tokenfactory/query.proto) defines `paxprotocol.paxchain.tokenfactory.Query` and the following GET bindings:

| Method | HTTP path |
| --- | --- |
| `Params` | `/pax-protocol/paxchain/tokenfactory/params` |
| `DenomAuthorityMetadata` | `/pax-protocol/paxchain/tokenfactory/denoms/{denom}/authority_metadata` |
| `DenomMetadata` | `/pax-protocol/paxchain/tokenfactory/denoms/metadata` |
| `DenomsFromCreator` | `/pax-protocol/paxchain/tokenfactory/denoms_from_creator/{creator}` |
| `DenomAllowList` | `/pax-protocol/paxchain/tokenfactory/denoms/allow_list` |
| `DenomPaused` | `/pax-protocol/paxchain/tokenfactory/denoms/{denom}/paused` |

Denoms can contain slashes. Use the additional query-parameter bindings for unambiguous denom lookup: `/pax-protocol/paxchain/tokenfactory/denom_authority_metadata?denom=<percent-encoded-denom>` and `/pax-protocol/paxchain/tokenfactory/denom_paused?denom=<percent-encoded-denom>`. Encode the query value once and preserve the full denom. `DenomMetadata` and `DenomAllowList` also take the request's `denom` query field. `QueryDenomPausedResponse` contains `paused` and `issuer_did`; authority metadata returns `authority_metadata`. Creator queries carry the declared pagination request/response.

Other HTTP routes are specified by `google.api.http` annotations in their owning files, including `additional_bindings`. Methods without an annotation do not acquire a REST route by name convention. Preserve HTTP method, body mapping, parameter encoding and generated error behavior. [api/gateway_mount_test.go](../api/gateway_mount_test.go) checks annotated routes against the registered module/SDK gateway; this page does not assert that test or a live deployment has been exercised.

## Buf workspace

The root [buf.yaml](../buf.yaml) is a **v2** workspace with local modules `api`, `sdk/proto`, `sdk/third_party/proto`, `interchain/proto`, `interchain/third_party/proto`, `consensus/proto`, `consensus/internal` and `wasm/proto`. The API module's registry name is `buf.build/pax-protocol/pax-chain`. Imports resolve from those local modules; the root [buf.lock](../buf.lock) has no external dependencies. The old API-local Buf configuration and lock are retired; use the root configuration.

Lint uses `BASIC`, `FILE_LOWER_SNAKE_CASE` and `UNARY_RPC`, with the explicit exceptions in the root file. The ignored third-party path is `sdk/third_party/proto`. Breaking checks use `WIRE` and `WIRE_JSON`; [buf.baseline.binpb](../buf.baseline.binpb) is the descriptor baseline. Keep wire field numbers and JSON compatibility intact rather than treating regeneration as permission to renumber fields.

## Codegen reproducibility

From the repository root, the maintained generation entry point is:

```sh
./scripts/protoc.sh
```

This is a regeneration instruction, not a gate claimed by this page. [scripts/protoc.sh](../scripts/protoc.sh) prepares `build/proto`, builds `protoc-gen-gocosmos` from `github.com/regen-network/cosmos-proto@v0.3.1`, and invokes Buf **v1.58.0**. The root [buf.gen.yaml](../buf.gen.yaml) selects API, SDK, interchain, consensus and Wasm inputs. Its local gocosmos output uses `plugins=interfacetype+grpc` and the declared protobuf mappings, including the SDK `Any` type. The HTTP generator is `protoc-gen-grpc-gateway@v1.16.0` with `logtostderr=true` and `allow_colon_final_segments=true`; consensus generation also uses `protoc-gen-gogofaster@v1.3.2`.

The script invokes the additional consensus internal and wireguard templates, copies generated files into their owning trees and removes the temporary output. [tools/repotest/gen.go](../tools/repotest/gen.go) exposes the script through `go:generate`. Change source declarations and their consumers together; generated output alone is not the source contract. Compatibility/parity and route tests live under [api](../api), but their execution belongs to the gate explicitly declared for the task making that interface change.

## Source declaration index

The anchors below retain registry links to SDK messages, transaction/query methods and HTTP bindings. Each entry points to the current source file; listed declarations come from that file. Follow it for exact field numbers, service request/response types and annotations. This index identifies declarations, not independently deployed endpoints. Native module sources under `api/` remain authoritative for their own packages.

<a id="proto-cosmos-auth-v1beta1-baseaccount-j39"></a>
<a id="proto-cosmos-auth-v1beta1-moduleaccount-j39"></a>
<a id="proto-cosmos-auth-v1beta1-params-j39"></a>

### cosmos.auth.v1beta1 — auth.proto

Source: [sdk/proto/cosmos/auth/v1beta1/auth.proto](../sdk/proto/cosmos/auth/v1beta1/auth.proto).

Types: [`message BaseAccount`](../sdk/proto/cosmos/auth/v1beta1/auth.proto#L13), [`message ModuleAccount`](../sdk/proto/cosmos/auth/v1beta1/auth.proto#L30), [`message Params`](../sdk/proto/cosmos/auth/v1beta1/auth.proto#L44).

<a id="proto-cosmos-auth-v1beta1-genesisstate-j39"></a>

### cosmos.auth.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/auth/v1beta1/genesis.proto](../sdk/proto/cosmos/auth/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/auth/v1beta1/genesis.proto#L11).

<a id="proto-cosmos-auth-v1beta1-queryaccountrequest-j39"></a>
<a id="proto-cosmos-auth-v1beta1-queryaccountresponse-j39"></a>
<a id="proto-cosmos-auth-v1beta1-queryaccountsrequest-j39"></a>
<a id="proto-cosmos-auth-v1beta1-queryaccountsresponse-j39"></a>
<a id="proto-cosmos-auth-v1beta1-querynextaccountnumberrequest-j39"></a>
<a id="proto-cosmos-auth-v1beta1-querynextaccountnumberresponse-j39"></a>
<a id="proto-cosmos-auth-v1beta1-queryparamsrequest-j39"></a>
<a id="proto-cosmos-auth-v1beta1-queryparamsresponse-j39"></a>
<a id="qry-cosmos-auth-v1beta1-account-j39"></a>
<a id="qry-cosmos-auth-v1beta1-accounts-j39"></a>
<a id="qry-cosmos-auth-v1beta1-nextaccountnumber-j39"></a>
<a id="qry-cosmos-auth-v1beta1-params-j39"></a>
<a id="rest-cosmos-auth-v1beta1-account-j39"></a>
<a id="rest-cosmos-auth-v1beta1-accounts-j39"></a>
<a id="rest-cosmos-auth-v1beta1-nextaccountnumber-j39"></a>
<a id="rest-cosmos-auth-v1beta1-params-j39"></a>

### cosmos.auth.v1beta1 — query.proto

Source: [sdk/proto/cosmos/auth/v1beta1/query.proto](../sdk/proto/cosmos/auth/v1beta1/query.proto).

Types: [`message QueryAccountsRequest`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L41), [`message QueryAccountsResponse`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L49), [`message QueryAccountRequest`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L58), [`message QueryAccountResponse`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L67), [`message QueryParamsRequest`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L73), [`message QueryParamsResponse`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L76), [`message QueryNextAccountNumberRequest`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L82), [`message QueryNextAccountNumberResponse`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L85).

Services: [`service Query`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L14).

Methods: [`rpc Accounts(QueryAccountsRequest) returns (QueryAccountsResponse)`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L18), [`rpc Account(QueryAccountRequest) returns (QueryAccountResponse)`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L23), [`rpc Params(QueryParamsRequest) returns (QueryParamsResponse)`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L28), [`rpc NextAccountNumber(QueryNextAccountNumberRequest) returns (QueryNextAccountNumberResponse)`](../sdk/proto/cosmos/auth/v1beta1/query.proto#L33).

<a id="proto-cosmos-authz-v1beta1-genericauthorization-j39"></a>
<a id="proto-cosmos-authz-v1beta1-grant-j39"></a>
<a id="proto-cosmos-authz-v1beta1-grantauthorization-j39"></a>

### cosmos.authz.v1beta1 — authz.proto

Source: [sdk/proto/cosmos/authz/v1beta1/authz.proto](../sdk/proto/cosmos/authz/v1beta1/authz.proto).

Types: [`message GenericAuthorization`](../sdk/proto/cosmos/authz/v1beta1/authz.proto#L15), [`message Grant`](../sdk/proto/cosmos/authz/v1beta1/authz.proto#L24), [`message GrantAuthorization`](../sdk/proto/cosmos/authz/v1beta1/authz.proto#L36).

<a id="proto-cosmos-authz-v1beta1-eventgrant-j39"></a>
<a id="proto-cosmos-authz-v1beta1-eventrevoke-j39"></a>

### cosmos.authz.v1beta1 — event.proto

Source: [sdk/proto/cosmos/authz/v1beta1/event.proto](../sdk/proto/cosmos/authz/v1beta1/event.proto).

Types: [`message EventGrant`](../sdk/proto/cosmos/authz/v1beta1/event.proto#L8), [`message EventRevoke`](../sdk/proto/cosmos/authz/v1beta1/event.proto#L18).

<a id="proto-cosmos-authz-v1beta1-genesisstate-j39"></a>

### cosmos.authz.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/authz/v1beta1/genesis.proto](../sdk/proto/cosmos/authz/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/authz/v1beta1/genesis.proto#L11).

<a id="proto-cosmos-authz-v1beta1-querygranteegrantsrequest-j39"></a>
<a id="proto-cosmos-authz-v1beta1-querygranteegrantsresponse-j39"></a>
<a id="proto-cosmos-authz-v1beta1-querygrantergrantsrequest-j39"></a>
<a id="proto-cosmos-authz-v1beta1-querygrantergrantsresponse-j39"></a>
<a id="proto-cosmos-authz-v1beta1-querygrantsrequest-j39"></a>
<a id="proto-cosmos-authz-v1beta1-querygrantsresponse-j39"></a>
<a id="qry-cosmos-authz-v1beta1-granteegrants-j39"></a>
<a id="qry-cosmos-authz-v1beta1-grantergrants-j39"></a>
<a id="qry-cosmos-authz-v1beta1-grants-j39"></a>
<a id="rest-cosmos-authz-v1beta1-granteegrants-j39"></a>
<a id="rest-cosmos-authz-v1beta1-grantergrants-j39"></a>
<a id="rest-cosmos-authz-v1beta1-grants-j39"></a>

### cosmos.authz.v1beta1 — query.proto

Source: [sdk/proto/cosmos/authz/v1beta1/query.proto](../sdk/proto/cosmos/authz/v1beta1/query.proto).

Types: [`message QueryGrantsRequest`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L34), [`message QueryGrantsResponse`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L44), [`message QueryGranterGrantsRequest`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L52), [`message QueryGranterGrantsResponse`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L60), [`message QueryGranteeGrantsRequest`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L68), [`message QueryGranteeGrantsResponse`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L76).

Services: [`service Query`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L12).

Methods: [`rpc Grants(QueryGrantsRequest) returns (QueryGrantsResponse)`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L14), [`rpc GranterGrants(QueryGranterGrantsRequest) returns (QueryGranterGrantsResponse)`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L21), [`rpc GranteeGrants(QueryGranteeGrantsRequest) returns (QueryGranteeGrantsResponse)`](../sdk/proto/cosmos/authz/v1beta1/query.proto#L28).

<a id="msg-cosmos-authz-v1beta1-exec-j39"></a>
<a id="msg-cosmos-authz-v1beta1-grant-j39"></a>
<a id="msg-cosmos-authz-v1beta1-revoke-j39"></a>
<a id="proto-cosmos-authz-v1beta1-msgexec-j39"></a>
<a id="proto-cosmos-authz-v1beta1-msgexecresponse-j39"></a>
<a id="proto-cosmos-authz-v1beta1-msggrant-j39"></a>
<a id="proto-cosmos-authz-v1beta1-msggrantresponse-j39"></a>
<a id="proto-cosmos-authz-v1beta1-msgrevoke-j39"></a>
<a id="proto-cosmos-authz-v1beta1-msgrevokeresponse-j39"></a>

### cosmos.authz.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/authz/v1beta1/tx.proto](../sdk/proto/cosmos/authz/v1beta1/tx.proto).

Types: [`message MsgGrant`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L33), [`message MsgExecResponse`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L41), [`message MsgExec`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L48), [`message MsgGrantResponse`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L57), [`message MsgRevoke`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L61), [`message MsgRevokeResponse`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L68).

Services: [`service Msg`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L14).

Methods: [`rpc Grant(MsgGrant) returns (MsgGrantResponse)`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L19), [`rpc Exec(MsgExec) returns (MsgExecResponse)`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L24), [`rpc Revoke(MsgRevoke) returns (MsgRevokeResponse)`](../sdk/proto/cosmos/authz/v1beta1/tx.proto#L28).

<a id="proto-cosmos-bank-v1beta1-sendauthorization-j39"></a>

### cosmos.bank.v1beta1 — authz.proto

Source: [sdk/proto/cosmos/bank/v1beta1/authz.proto](../sdk/proto/cosmos/bank/v1beta1/authz.proto).

Types: [`message SendAuthorization`](../sdk/proto/cosmos/bank/v1beta1/authz.proto#L14).

<a id="proto-cosmos-bank-v1beta1-allowlist-j39"></a>
<a id="proto-cosmos-bank-v1beta1-denomunit-j39"></a>
<a id="proto-cosmos-bank-v1beta1-input-j39"></a>
<a id="proto-cosmos-bank-v1beta1-metadata-j39"></a>
<a id="proto-cosmos-bank-v1beta1-output-j39"></a>
<a id="proto-cosmos-bank-v1beta1-params-j39"></a>
<a id="proto-cosmos-bank-v1beta1-sendenabled-j39"></a>
<a id="proto-cosmos-bank-v1beta1-supply-j39"></a>

### cosmos.bank.v1beta1 — bank.proto

Source: [sdk/proto/cosmos/bank/v1beta1/bank.proto](../sdk/proto/cosmos/bank/v1beta1/bank.proto).

Types: [`message Params`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L11), [`message SendEnabled`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L19), [`message Input`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L27), [`message Output`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L39), [`message Supply`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L53), [`message DenomUnit`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L69), [`message Metadata`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L84), [`message AllowList`](../sdk/proto/cosmos/bank/v1beta1/bank.proto#L104).

<a id="proto-cosmos-bank-v1beta1-balance-j39"></a>
<a id="proto-cosmos-bank-v1beta1-genesisstate-j39"></a>
<a id="proto-cosmos-bank-v1beta1-weibalance-j39"></a>

### cosmos.bank.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/bank/v1beta1/genesis.proto](../sdk/proto/cosmos/bank/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/bank/v1beta1/genesis.proto#L11), [`message DenomAllowList`](../sdk/proto/cosmos/bank/v1beta1/genesis.proto#L42), [`message Balance`](../sdk/proto/cosmos/bank/v1beta1/genesis.proto#L49), [`message WeiBalance`](../sdk/proto/cosmos/bank/v1beta1/genesis.proto#L63).

<a id="proto-cosmos-bank-v1beta1-queryallbalancesrequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-queryallbalancesresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querybalancerequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querybalanceresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querydenommetadatarequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querydenommetadataresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querydenomsmetadatarequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querydenomsmetadataresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-queryparamsrequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-queryparamsresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-queryspendablebalancesrequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-queryspendablebalancesresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querysupplyofrequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querysupplyofresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querytotalsupplyrequest-j39"></a>
<a id="proto-cosmos-bank-v1beta1-querytotalsupplyresponse-j39"></a>
<a id="qry-cosmos-bank-v1beta1-allbalances-j39"></a>
<a id="qry-cosmos-bank-v1beta1-balance-j39"></a>
<a id="qry-cosmos-bank-v1beta1-denommetadata-j39"></a>
<a id="qry-cosmos-bank-v1beta1-denomsmetadata-j39"></a>
<a id="qry-cosmos-bank-v1beta1-params-j39"></a>
<a id="qry-cosmos-bank-v1beta1-spendablebalances-j39"></a>
<a id="qry-cosmos-bank-v1beta1-supplyof-j39"></a>
<a id="qry-cosmos-bank-v1beta1-totalsupply-j39"></a>
<a id="rest-cosmos-bank-v1beta1-allbalances-j39"></a>
<a id="rest-cosmos-bank-v1beta1-balance-j39"></a>
<a id="rest-cosmos-bank-v1beta1-denommetadata-j39"></a>
<a id="rest-cosmos-bank-v1beta1-denomsmetadata-j39"></a>
<a id="rest-cosmos-bank-v1beta1-params-j39"></a>
<a id="rest-cosmos-bank-v1beta1-spendablebalances-j39"></a>
<a id="rest-cosmos-bank-v1beta1-supplyof-j39"></a>
<a id="rest-cosmos-bank-v1beta1-totalsupply-j39"></a>

### cosmos.bank.v1beta1 — query.proto

Source: [sdk/proto/cosmos/bank/v1beta1/query.proto](../sdk/proto/cosmos/bank/v1beta1/query.proto).

Types: [`message QueryBalanceRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L71), [`message QueryBalanceResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L83), [`message QueryAllBalancesRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L89), [`message QueryAllBalancesResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L102), [`message QuerySpendableBalancesRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L116), [`message QuerySpendableBalancesResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L129), [`message QueryTotalSupplyRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L143), [`message QueryTotalSupplyResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L155), [`message QuerySupplyOfRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L170), [`message QuerySupplyOfResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L176), [`message QueryParamsRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L182), [`message QueryParamsResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L185), [`message QueryDenomsMetadataRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L191), [`message QueryDenomsMetadataResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L198), [`message QueryDenomMetadataRequest`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L208), [`message QueryDenomMetadataResponse`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L215).

Services: [`service Query`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L13).

Methods: [`rpc Balance(QueryBalanceRequest) returns (QueryBalanceResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L15), [`rpc AllBalances(QueryAllBalancesRequest) returns (QueryAllBalancesResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L21), [`rpc SpendableBalances(QuerySpendableBalancesRequest) returns (QuerySpendableBalancesResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L27), [`rpc TotalSupply(QueryTotalSupplyRequest) returns (QueryTotalSupplyResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L34), [`rpc SupplyOf(QuerySupplyOfRequest) returns (QuerySupplyOfResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L39), [`rpc Params(QueryParamsRequest) returns (QueryParamsResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L47), [`rpc DenomMetadata(QueryDenomMetadataRequest) returns (QueryDenomMetadataResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L52), [`rpc DenomsMetadata(QueryDenomsMetadataRequest) returns (QueryDenomsMetadataResponse)`](../sdk/proto/cosmos/bank/v1beta1/query.proto#L64).

<a id="msg-cosmos-bank-v1beta1-multisend-j39"></a>
<a id="msg-cosmos-bank-v1beta1-send-j39"></a>
<a id="proto-cosmos-bank-v1beta1-msgmultisend-j39"></a>
<a id="proto-cosmos-bank-v1beta1-msgmultisendresponse-j39"></a>
<a id="proto-cosmos-bank-v1beta1-msgsend-j39"></a>
<a id="proto-cosmos-bank-v1beta1-msgsendresponse-j39"></a>

### cosmos.bank.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/bank/v1beta1/tx.proto](../sdk/proto/cosmos/bank/v1beta1/tx.proto).

Types: [`message MsgSend`](../sdk/proto/cosmos/bank/v1beta1/tx.proto#L20), [`message MsgSendResponse`](../sdk/proto/cosmos/bank/v1beta1/tx.proto#L33), [`message MsgMultiSend`](../sdk/proto/cosmos/bank/v1beta1/tx.proto#L36), [`message MsgMultiSendResponse`](../sdk/proto/cosmos/bank/v1beta1/tx.proto#L44).

Services: [`service Msg`](../sdk/proto/cosmos/bank/v1beta1/tx.proto#L11).

Methods: [`rpc Send(MsgSend) returns (MsgSendResponse)`](../sdk/proto/cosmos/bank/v1beta1/tx.proto#L13), [`rpc MultiSend(MsgMultiSend) returns (MsgMultiSendResponse)`](../sdk/proto/cosmos/bank/v1beta1/tx.proto#L16).

<a id="proto-cosmos-base-abci-v1beta1-abcimessagelog-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-attribute-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-gasinfo-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-msgdata-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-result-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-searchtxsresult-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-simulationresponse-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-stringevent-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-txmsgdata-j39"></a>
<a id="proto-cosmos-base-abci-v1beta1-txresponse-j39"></a>

### cosmos.base.abci.v1beta1 — abci.proto

Source: [sdk/proto/cosmos/base/abci/v1beta1/abci.proto](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto).

Types: [`message TxResponse`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L13), [`message ABCIMessageLog`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L55), [`message StringEvent`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L71), [`message Attribute`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L80), [`message GasInfo`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L86), [`message Result`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L98), [`message SimulationResponse`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L119), [`message MsgData`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L129), [`message TxMsgData`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L138), [`message SearchTxsResult`](../sdk/proto/cosmos/base/abci/v1beta1/abci.proto#L145).

<a id="proto-cosmos-base-kv-v1beta1-pair-j39"></a>
<a id="proto-cosmos-base-kv-v1beta1-pairs-j39"></a>

### cosmos.base.kv.v1beta1 — kv.proto

Source: [sdk/proto/cosmos/base/kv/v1beta1/kv.proto](../sdk/proto/cosmos/base/kv/v1beta1/kv.proto).

Types: [`message Pairs`](../sdk/proto/cosmos/base/kv/v1beta1/kv.proto#L9), [`message Pair`](../sdk/proto/cosmos/base/kv/v1beta1/kv.proto#L14).

<a id="proto-cosmos-base-query-v1beta1-pagerequest-j39"></a>
<a id="proto-cosmos-base-query-v1beta1-pageresponse-j39"></a>

### cosmos.base.query.v1beta1 — pagination.proto

Source: [sdk/proto/cosmos/base/query/v1beta1/pagination.proto](../sdk/proto/cosmos/base/query/v1beta1/pagination.proto).

Types: [`message PageRequest`](../sdk/proto/cosmos/base/query/v1beta1/pagination.proto#L13), [`message PageResponse`](../sdk/proto/cosmos/base/query/v1beta1/pagination.proto#L47).

<a id="grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39"></a>
<a id="grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39"></a>
<a id="proto-cosmos-base-reflection-v1beta1-listallinterfacesrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v1beta1-listallinterfacesresponse-j39"></a>
<a id="proto-cosmos-base-reflection-v1beta1-listimplementationsrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v1beta1-listimplementationsresponse-j39"></a>
<a id="rest-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39"></a>
<a id="rest-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39"></a>

### cosmos.base.reflection.v1beta1 — reflection.proto

Source: [sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto).

Types: [`message ListAllInterfacesRequest`](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto#L26), [`message ListAllInterfacesResponse`](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto#L29), [`message ListImplementationsRequest`](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto#L36), [`message ListImplementationsResponse`](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto#L43).

Services: [`service ReflectionService`](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto#L9).

Methods: [`rpc ListAllInterfaces(ListAllInterfacesRequest) returns (ListAllInterfacesResponse)`](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto#L12), [`rpc ListImplementations(ListImplementationsRequest) returns (ListImplementationsResponse)`](../sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto#L18).

<a id="grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39"></a>
<a id="grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39"></a>
<a id="grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39"></a>
<a id="grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39"></a>
<a id="grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39"></a>
<a id="grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-appdescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-authndescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-chaindescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-codecdescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-configurationdescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getauthndescriptorrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getauthndescriptorresponse-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getchaindescriptorrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getchaindescriptorresponse-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorresponse-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorresponse-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorresponse-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-gettxdescriptorrequest-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-gettxdescriptorresponse-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-interfaceacceptingmessagedescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-interfacedescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-interfaceimplementerdescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-msgdescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-querymethoddescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-queryservicedescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-queryservicesdescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-signingmodedescriptor-j39"></a>
<a id="proto-cosmos-base-reflection-v2alpha1-txdescriptor-j39"></a>
<a id="rest-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39"></a>
<a id="rest-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39"></a>
<a id="rest-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39"></a>
<a id="rest-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39"></a>
<a id="rest-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39"></a>
<a id="rest-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39"></a>

### cosmos.base.reflection.v2alpha1 — reflection.proto

Source: [sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto).

Types: [`message AppDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L10), [`message TxDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L27), [`message AuthnDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L38), [`message SigningModeDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L47), [`message ChainDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L58), [`message CodecDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L64), [`message InterfaceDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L70), [`message InterfaceImplementerDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L81), [`message InterfaceAcceptingMessageDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L93), [`message ConfigurationDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L103), [`message MsgDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L109), [`message GetAuthnDescriptorRequest`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L145), [`message GetAuthnDescriptorResponse`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L148), [`message GetChainDescriptorRequest`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L154), [`message GetChainDescriptorResponse`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L157), [`message GetCodecDescriptorRequest`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L163), [`message GetCodecDescriptorResponse`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L166), [`message GetConfigurationDescriptorRequest`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L172), [`message GetConfigurationDescriptorResponse`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L175), [`message GetQueryServicesDescriptorRequest`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L181), [`message GetQueryServicesDescriptorResponse`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L184), [`message GetTxDescriptorRequest`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L190), [`message GetTxDescriptorResponse`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L193), [`message QueryServicesDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L200), [`message QueryServiceDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L206), [`message QueryMethodDescriptor`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L218).

Services: [`service ReflectionService`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L115).

Methods: [`rpc GetAuthnDescriptor(GetAuthnDescriptorRequest) returns (GetAuthnDescriptorResponse)`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L119), [`rpc GetChainDescriptor(GetChainDescriptorRequest) returns (GetChainDescriptorResponse)`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L123), [`rpc GetCodecDescriptor(GetCodecDescriptorRequest) returns (GetCodecDescriptorResponse)`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L127), [`rpc GetConfigurationDescriptor(GetConfigurationDescriptorRequest) returns (GetConfigurationDescriptorResponse)`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L131), [`rpc GetQueryServicesDescriptor(GetQueryServicesDescriptorRequest) returns (GetQueryServicesDescriptorResponse)`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L135), [`rpc GetTxDescriptor(GetTxDescriptorRequest) returns (GetTxDescriptorResponse)`](../sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto#L139).

<a id="proto-cosmos-base-snapshots-v1beta1-metadata-j39"></a>
<a id="proto-cosmos-base-snapshots-v1beta1-snapshot-j39"></a>
<a id="proto-cosmos-base-snapshots-v1beta1-snapshotextensionmeta-j39"></a>
<a id="proto-cosmos-base-snapshots-v1beta1-snapshotextensionpayload-j39"></a>
<a id="proto-cosmos-base-snapshots-v1beta1-snapshotiavlitem-j39"></a>
<a id="proto-cosmos-base-snapshots-v1beta1-snapshotitem-j39"></a>
<a id="proto-cosmos-base-snapshots-v1beta1-snapshotstoreitem-j39"></a>

### cosmos.base.snapshots.v1beta1 — snapshot.proto

Source: [sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto).

Types: [`message Snapshot`](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto#L9), [`message Metadata`](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto#L18), [`message SnapshotItem`](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto#L23), [`message SnapshotStoreItem`](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto#L34), [`message SnapshotIAVLItem`](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto#L39), [`message SnapshotExtensionMeta`](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto#L49), [`message SnapshotExtensionPayload`](../sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto#L55).

<a id="proto-cosmos-base-store-v1beta1-commitid-j39"></a>
<a id="proto-cosmos-base-store-v1beta1-commitinfo-j39"></a>
<a id="proto-cosmos-base-store-v1beta1-storeinfo-j39"></a>

### cosmos.base.store.v1beta1 — commit_info.proto

Source: [sdk/proto/cosmos/base/store/v1beta1/commit_info.proto](../sdk/proto/cosmos/base/store/v1beta1/commit_info.proto).

Types: [`message CommitInfo`](../sdk/proto/cosmos/base/store/v1beta1/commit_info.proto#L10), [`message StoreInfo`](../sdk/proto/cosmos/base/store/v1beta1/commit_info.proto#L17), [`message CommitID`](../sdk/proto/cosmos/base/store/v1beta1/commit_info.proto#L24).

<a id="proto-cosmos-base-store-v1beta1-storekvpair-j39"></a>

### cosmos.base.store.v1beta1 — listening.proto

Source: [sdk/proto/cosmos/base/store/v1beta1/listening.proto](../sdk/proto/cosmos/base/store/v1beta1/listening.proto).

Types: [`message StoreKVPair`](../sdk/proto/cosmos/base/store/v1beta1/listening.proto#L11).

<a id="grpcs-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39"></a>
<a id="grpcs-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39"></a>
<a id="grpcs-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39"></a>
<a id="grpcs-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39"></a>
<a id="grpcs-cosmos-base-tendermint-v1beta1-service-getsyncing-j39"></a>
<a id="grpcs-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getblockbyheightrequest-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getblockbyheightresponse-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getlatestblockrequest-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getlatestblockresponse-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetrequest-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetresponse-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getnodeinforequest-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getnodeinforesponse-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getsyncingrequest-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getsyncingresponse-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightrequest-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightresponse-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-module-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-validator-j39"></a>
<a id="proto-cosmos-base-tendermint-v1beta1-versioninfo-j39"></a>
<a id="rest-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39"></a>
<a id="rest-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39"></a>
<a id="rest-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39"></a>
<a id="rest-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39"></a>
<a id="rest-cosmos-base-tendermint-v1beta1-service-getsyncing-j39"></a>
<a id="rest-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39"></a>

### cosmos.base.tendermint.v1beta1 — query.proto

Source: [sdk/proto/cosmos/base/tendermint/v1beta1/query.proto](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto).

Types: [`message GetValidatorSetByHeightRequest`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L43), [`message GetValidatorSetByHeightResponse`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L50), [`message GetLatestValidatorSetRequest`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L58), [`message GetLatestValidatorSetResponse`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L64), [`message Validator`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L72), [`message GetBlockByHeightRequest`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L80), [`message GetBlockByHeightResponse`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L85), [`message GetLatestBlockRequest`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L91), [`message GetLatestBlockResponse`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L94), [`message GetSyncingRequest`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L100), [`message GetSyncingResponse`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L103), [`message GetNodeInfoRequest`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L108), [`message GetNodeInfoResponse`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L111), [`message VersionInfo`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L117), [`message Module`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L130).

Services: [`service Service`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L14).

Methods: [`rpc GetNodeInfo(GetNodeInfoRequest) returns (GetNodeInfoResponse)`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L16), [`rpc GetSyncing(GetSyncingRequest) returns (GetSyncingResponse)`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L20), [`rpc GetLatestBlock(GetLatestBlockRequest) returns (GetLatestBlockResponse)`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L24), [`rpc GetBlockByHeight(GetBlockByHeightRequest) returns (GetBlockByHeightResponse)`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L28), [`rpc GetLatestValidatorSet(GetLatestValidatorSetRequest) returns (GetLatestValidatorSetResponse)`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L33), [`rpc GetValidatorSetByHeight(GetValidatorSetByHeightRequest) returns (GetValidatorSetByHeightResponse)`](../sdk/proto/cosmos/base/tendermint/v1beta1/query.proto#L37).

<a id="proto-cosmos-base-v1beta1-coin-j39"></a>
<a id="proto-cosmos-base-v1beta1-deccoin-j39"></a>
<a id="proto-cosmos-base-v1beta1-decproto-j39"></a>
<a id="proto-cosmos-base-v1beta1-intproto-j39"></a>

### cosmos.base.v1beta1 — coin.proto

Source: [sdk/proto/cosmos/base/v1beta1/coin.proto](../sdk/proto/cosmos/base/v1beta1/coin.proto).

Types: [`message Coin`](../sdk/proto/cosmos/base/v1beta1/coin.proto#L14), [`message DecCoin`](../sdk/proto/cosmos/base/v1beta1/coin.proto#L28), [`message IntProto`](../sdk/proto/cosmos/base/v1beta1/coin.proto#L39), [`message DecProto`](../sdk/proto/cosmos/base/v1beta1/coin.proto#L47).

<a id="proto-cosmos-capability-v1beta1-capability-j39"></a>
<a id="proto-cosmos-capability-v1beta1-capabilityowners-j39"></a>
<a id="proto-cosmos-capability-v1beta1-owner-j39"></a>

### cosmos.capability.v1beta1 — capability.proto

Source: [sdk/proto/cosmos/capability/v1beta1/capability.proto](../sdk/proto/cosmos/capability/v1beta1/capability.proto).

Types: [`message Capability`](../sdk/proto/cosmos/capability/v1beta1/capability.proto#L10), [`message Owner`](../sdk/proto/cosmos/capability/v1beta1/capability.proto#L18), [`message CapabilityOwners`](../sdk/proto/cosmos/capability/v1beta1/capability.proto#L28).

<a id="proto-cosmos-capability-v1beta1-genesisowners-j39"></a>
<a id="proto-cosmos-capability-v1beta1-genesisstate-j39"></a>

### cosmos.capability.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/capability/v1beta1/genesis.proto](../sdk/proto/cosmos/capability/v1beta1/genesis.proto).

Types: [`message GenesisOwners`](../sdk/proto/cosmos/capability/v1beta1/genesis.proto#L10), [`message GenesisState`](../sdk/proto/cosmos/capability/v1beta1/genesis.proto#L22).

<a id="proto-cosmos-crypto-ed25519-privkey-j39"></a>
<a id="proto-cosmos-crypto-ed25519-pubkey-j39"></a>

### cosmos.crypto.ed25519 — keys.proto

Source: [sdk/proto/cosmos/crypto/ed25519/keys.proto](../sdk/proto/cosmos/crypto/ed25519/keys.proto).

Types: [`message PubKey`](../sdk/proto/cosmos/crypto/ed25519/keys.proto#L13), [`message PrivKey`](../sdk/proto/cosmos/crypto/ed25519/keys.proto#L21).

<a id="proto-cosmos-crypto-multisig-legacyaminopubkey-j39"></a>

### cosmos.crypto.multisig — keys.proto

Source: [sdk/proto/cosmos/crypto/multisig/keys.proto](../sdk/proto/cosmos/crypto/multisig/keys.proto).

Types: [`message LegacyAminoPubKey`](../sdk/proto/cosmos/crypto/multisig/keys.proto#L12).

<a id="proto-cosmos-crypto-multisig-v1beta1-compactbitarray-j39"></a>
<a id="proto-cosmos-crypto-multisig-v1beta1-multisignature-j39"></a>

### cosmos.crypto.multisig.v1beta1 — multisig.proto

Source: [sdk/proto/cosmos/crypto/multisig/v1beta1/multisig.proto](../sdk/proto/cosmos/crypto/multisig/v1beta1/multisig.proto).

Types: [`message MultiSignature`](../sdk/proto/cosmos/crypto/multisig/v1beta1/multisig.proto#L11), [`message CompactBitArray`](../sdk/proto/cosmos/crypto/multisig/v1beta1/multisig.proto#L20).

<a id="proto-cosmos-crypto-secp256k1-privkey-j39"></a>
<a id="proto-cosmos-crypto-secp256k1-pubkey-j39"></a>

### cosmos.crypto.secp256k1 — keys.proto

Source: [sdk/proto/cosmos/crypto/secp256k1/keys.proto](../sdk/proto/cosmos/crypto/secp256k1/keys.proto).

Types: [`message PubKey`](../sdk/proto/cosmos/crypto/secp256k1/keys.proto#L13), [`message PrivKey`](../sdk/proto/cosmos/crypto/secp256k1/keys.proto#L20).

<a id="proto-cosmos-crypto-secp256r1-privkey-j39"></a>
<a id="proto-cosmos-crypto-secp256r1-pubkey-j39"></a>

### cosmos.crypto.secp256r1 — keys.proto

Source: [sdk/proto/cosmos/crypto/secp256r1/keys.proto](../sdk/proto/cosmos/crypto/secp256r1/keys.proto).

Types: [`message PubKey`](../sdk/proto/cosmos/crypto/secp256r1/keys.proto#L13), [`message PrivKey`](../sdk/proto/cosmos/crypto/secp256r1/keys.proto#L20).

<a id="proto-cosmos-crypto-sr25519-pubkey-j39"></a>

### cosmos.crypto.sr25519 — keys.proto

Source: [sdk/proto/cosmos/crypto/sr25519/keys.proto](../sdk/proto/cosmos/crypto/sr25519/keys.proto).

Types: [`message PubKey`](../sdk/proto/cosmos/crypto/sr25519/keys.proto#L12).

<a id="proto-cosmos-distribution-v1beta1-communitypoolspendproposal-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-communitypoolspendproposalwithdeposit-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-delegationdelegatorreward-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-delegatorstartinginfo-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-feepool-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-params-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatoraccumulatedcommission-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatorcurrentrewards-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatorhistoricalrewards-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatorslashevent-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatorslashevents-j39"></a>

### cosmos.distribution.v1beta1 — distribution.proto

Source: [sdk/proto/cosmos/distribution/v1beta1/distribution.proto](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto).

Types: [`message Params`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L11), [`message ValidatorHistoricalRewards`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L43), [`message ValidatorCurrentRewards`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L55), [`message ValidatorAccumulatedCommission`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L65), [`message ValidatorOutstandingRewards`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L74), [`message ValidatorSlashEvent`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L86), [`message ValidatorSlashEvents`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L95), [`message FeePool`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L104), [`message CommunityPoolSpendProposal`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L115), [`message DelegatorStartingInfo`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L135), [`message DelegationDelegatorReward`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L150), [`message CommunityPoolSpendProposalWithDeposit`](../sdk/proto/cosmos/distribution/v1beta1/distribution.proto#L164).

<a id="proto-cosmos-distribution-v1beta1-delegatorstartinginforecord-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-delegatorwithdrawinfo-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-genesisstate-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatoraccumulatedcommissionrecord-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatorcurrentrewardsrecord-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatorhistoricalrewardsrecord-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatoroutstandingrewardsrecord-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-validatorslasheventrecord-j39"></a>

### cosmos.distribution.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/distribution/v1beta1/genesis.proto](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto).

Types: [`message DelegatorWithdrawInfo`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L14), [`message ValidatorOutstandingRewardsRecord`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L26), [`message ValidatorAccumulatedCommissionRecord`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L43), [`message ValidatorHistoricalRewardsRecord`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L59), [`message ValidatorCurrentRewardsRecord`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L77), [`message DelegatorStartingInfoRecord`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L92), [`message ValidatorSlashEventRecord`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L110), [`message GenesisState`](../sdk/proto/cosmos/distribution/v1beta1/genesis.proto#L128).

<a id="proto-cosmos-distribution-v1beta1-querycommunitypoolrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querycommunitypoolresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegationrewardsrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegationrewardsresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryparamsrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryparamsresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryvalidatorcommissionrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryvalidatorcommissionresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryvalidatorslashesrequest-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-queryvalidatorslashesresponse-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-communitypool-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-delegationrewards-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-delegationtotalrewards-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-delegatorvalidators-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-params-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-validatorcommission-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39"></a>
<a id="qry-cosmos-distribution-v1beta1-validatorslashes-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-communitypool-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-delegationrewards-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-delegationtotalrewards-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-delegatorvalidators-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-params-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-validatorcommission-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39"></a>
<a id="rest-cosmos-distribution-v1beta1-validatorslashes-j39"></a>

### cosmos.distribution.v1beta1 — query.proto

Source: [sdk/proto/cosmos/distribution/v1beta1/query.proto](../sdk/proto/cosmos/distribution/v1beta1/query.proto).

Types: [`message QueryParamsRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L72), [`message QueryParamsResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L75), [`message QueryValidatorOutstandingRewardsRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L82), [`message QueryValidatorOutstandingRewardsResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L89), [`message QueryValidatorCommissionRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L95), [`message QueryValidatorCommissionResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L102), [`message QueryValidatorSlashesRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L109), [`message QueryValidatorSlashesResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L125), [`message QueryDelegationRewardsRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L135), [`message QueryDelegationRewardsResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L147), [`message QueryDelegationTotalRewardsRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L157), [`message QueryDelegationTotalRewardsResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L166), [`message QueryDelegatorValidatorsRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L178), [`message QueryDelegatorValidatorsResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L188), [`message QueryDelegatorWithdrawAddressRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L198), [`message QueryDelegatorWithdrawAddressResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L208), [`message QueryCommunityPoolRequest`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L218), [`message QueryCommunityPoolResponse`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L222).

Services: [`service Query`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L13).

Methods: [`rpc Params(QueryParamsRequest) returns (QueryParamsResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L15), [`rpc ValidatorOutstandingRewards(QueryValidatorOutstandingRewardsRequest) returns (QueryValidatorOutstandingRewardsResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L20), [`rpc ValidatorCommission(QueryValidatorCommissionRequest) returns (QueryValidatorCommissionResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L27), [`rpc ValidatorSlashes(QueryValidatorSlashesRequest) returns (QueryValidatorSlashesResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L34), [`rpc DelegationRewards(QueryDelegationRewardsRequest) returns (QueryDelegationRewardsResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L39), [`rpc DelegationTotalRewards(QueryDelegationTotalRewardsRequest) returns (QueryDelegationTotalRewardsResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L47), [`rpc DelegatorValidators(QueryDelegatorValidatorsRequest) returns (QueryDelegatorValidatorsResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L52), [`rpc DelegatorWithdrawAddress(QueryDelegatorWithdrawAddressRequest) returns (QueryDelegatorWithdrawAddressResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L59), [`rpc CommunityPool(QueryCommunityPoolRequest) returns (QueryCommunityPoolResponse)`](../sdk/proto/cosmos/distribution/v1beta1/query.proto#L66).

<a id="msg-cosmos-distribution-v1beta1-fundcommunitypool-j39"></a>
<a id="msg-cosmos-distribution-v1beta1-setwithdrawaddress-j39"></a>
<a id="msg-cosmos-distribution-v1beta1-withdrawdelegatorreward-j39"></a>
<a id="msg-cosmos-distribution-v1beta1-withdrawvalidatorcommission-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgfundcommunitypool-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgfundcommunitypoolresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgsetwithdrawaddress-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgsetwithdrawaddressresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorreward-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorrewardresponse-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommission-j39"></a>
<a id="proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommissionresponse-j39"></a>

### cosmos.distribution.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/distribution/v1beta1/tx.proto](../sdk/proto/cosmos/distribution/v1beta1/tx.proto).

Types: [`message MsgSetWithdrawAddress`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L31), [`message MsgSetWithdrawAddressResponse`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L40), [`message MsgWithdrawDelegatorReward`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L44), [`message MsgWithdrawDelegatorRewardResponse`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L53), [`message MsgWithdrawValidatorCommission`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L57), [`message MsgWithdrawValidatorCommissionResponse`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L65), [`message MsgFundCommunityPool`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L69), [`message MsgFundCommunityPoolResponse`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L81).

Services: [`service Msg`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L11).

Methods: [`rpc SetWithdrawAddress(MsgSetWithdrawAddress) returns (MsgSetWithdrawAddressResponse)`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L14), [`rpc WithdrawDelegatorReward(MsgWithdrawDelegatorReward) returns (MsgWithdrawDelegatorRewardResponse)`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L18), [`rpc WithdrawValidatorCommission(MsgWithdrawValidatorCommission) returns (MsgWithdrawValidatorCommissionResponse)`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L22), [`rpc FundCommunityPool(MsgFundCommunityPool) returns (MsgFundCommunityPoolResponse)`](../sdk/proto/cosmos/distribution/v1beta1/tx.proto#L26).

<a id="proto-cosmos-evidence-v1beta1-equivocation-j39"></a>

### cosmos.evidence.v1beta1 — evidence.proto

Source: [sdk/proto/cosmos/evidence/v1beta1/evidence.proto](../sdk/proto/cosmos/evidence/v1beta1/evidence.proto).

Types: [`message Equivocation`](../sdk/proto/cosmos/evidence/v1beta1/evidence.proto#L12).

<a id="proto-cosmos-evidence-v1beta1-genesisstate-j39"></a>

### cosmos.evidence.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/evidence/v1beta1/genesis.proto](../sdk/proto/cosmos/evidence/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/evidence/v1beta1/genesis.proto#L9).

<a id="proto-cosmos-evidence-v1beta1-queryallevidencerequest-j39"></a>
<a id="proto-cosmos-evidence-v1beta1-queryallevidenceresponse-j39"></a>
<a id="proto-cosmos-evidence-v1beta1-queryevidencerequest-j39"></a>
<a id="proto-cosmos-evidence-v1beta1-queryevidenceresponse-j39"></a>
<a id="qry-cosmos-evidence-v1beta1-allevidence-j39"></a>
<a id="qry-cosmos-evidence-v1beta1-evidence-j39"></a>
<a id="rest-cosmos-evidence-v1beta1-allevidence-j39"></a>
<a id="rest-cosmos-evidence-v1beta1-evidence-j39"></a>

### cosmos.evidence.v1beta1 — query.proto

Source: [sdk/proto/cosmos/evidence/v1beta1/query.proto](../sdk/proto/cosmos/evidence/v1beta1/query.proto).

Types: [`message QueryEvidenceRequest`](../sdk/proto/cosmos/evidence/v1beta1/query.proto#L25), [`message QueryEvidenceResponse`](../sdk/proto/cosmos/evidence/v1beta1/query.proto#L31), [`message QueryAllEvidenceRequest`](../sdk/proto/cosmos/evidence/v1beta1/query.proto#L38), [`message QueryAllEvidenceResponse`](../sdk/proto/cosmos/evidence/v1beta1/query.proto#L45).

Services: [`service Query`](../sdk/proto/cosmos/evidence/v1beta1/query.proto#L12).

Methods: [`rpc Evidence(QueryEvidenceRequest) returns (QueryEvidenceResponse)`](../sdk/proto/cosmos/evidence/v1beta1/query.proto#L14), [`rpc AllEvidence(QueryAllEvidenceRequest) returns (QueryAllEvidenceResponse)`](../sdk/proto/cosmos/evidence/v1beta1/query.proto#L19).

<a id="msg-cosmos-evidence-v1beta1-submitevidence-j39"></a>
<a id="proto-cosmos-evidence-v1beta1-msgsubmitevidence-j39"></a>
<a id="proto-cosmos-evidence-v1beta1-msgsubmitevidenceresponse-j39"></a>

### cosmos.evidence.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/evidence/v1beta1/tx.proto](../sdk/proto/cosmos/evidence/v1beta1/tx.proto).

Types: [`message MsgSubmitEvidence`](../sdk/proto/cosmos/evidence/v1beta1/tx.proto#L20), [`message MsgSubmitEvidenceResponse`](../sdk/proto/cosmos/evidence/v1beta1/tx.proto#L29).

Services: [`service Msg`](../sdk/proto/cosmos/evidence/v1beta1/tx.proto#L12).

Methods: [`rpc SubmitEvidence(MsgSubmitEvidence) returns (MsgSubmitEvidenceResponse)`](../sdk/proto/cosmos/evidence/v1beta1/tx.proto#L15).

<a id="proto-cosmos-feegrant-v1beta1-allowedmsgallowance-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-basicallowance-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-grant-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-periodicallowance-j39"></a>

### cosmos.feegrant.v1beta1 — feegrant.proto

Source: [sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto](../sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto).

Types: [`message BasicAllowance`](../sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto#L16), [`message PeriodicAllowance`](../sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto#L33), [`message AllowedMsgAllowance`](../sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto#L69), [`message Grant`](../sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto#L81).

<a id="proto-cosmos-feegrant-v1beta1-genesisstate-j39"></a>

### cosmos.feegrant.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/feegrant/v1beta1/genesis.proto](../sdk/proto/cosmos/feegrant/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/feegrant/v1beta1/genesis.proto#L11).

<a id="proto-cosmos-feegrant-v1beta1-queryallowancerequest-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-queryallowanceresponse-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-queryallowancesbygranterrequest-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-queryallowancesbygranterresponse-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-queryallowancesrequest-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-queryallowancesresponse-j39"></a>
<a id="qry-cosmos-feegrant-v1beta1-allowance-j39"></a>
<a id="qry-cosmos-feegrant-v1beta1-allowances-j39"></a>
<a id="qry-cosmos-feegrant-v1beta1-allowancesbygranter-j39"></a>
<a id="rest-cosmos-feegrant-v1beta1-allowance-j39"></a>
<a id="rest-cosmos-feegrant-v1beta1-allowances-j39"></a>
<a id="rest-cosmos-feegrant-v1beta1-allowancesbygranter-j39"></a>

### cosmos.feegrant.v1beta1 — query.proto

Source: [sdk/proto/cosmos/feegrant/v1beta1/query.proto](../sdk/proto/cosmos/feegrant/v1beta1/query.proto).

Types: [`message QueryAllowanceRequest`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L31), [`message QueryAllowanceResponse`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L40), [`message QueryAllowancesRequest`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L46), [`message QueryAllowancesResponse`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L54), [`message QueryAllowancesByGranterRequest`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L63), [`message QueryAllowancesByGranterResponse`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L71).

Services: [`service Query`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L12).

Methods: [`rpc Allowance(QueryAllowanceRequest) returns (QueryAllowanceResponse)`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L14), [`rpc Allowances(QueryAllowancesRequest) returns (QueryAllowancesResponse)`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L19), [`rpc AllowancesByGranter(QueryAllowancesByGranterRequest) returns (QueryAllowancesByGranterResponse)`](../sdk/proto/cosmos/feegrant/v1beta1/query.proto#L25).

<a id="msg-cosmos-feegrant-v1beta1-grantallowance-j39"></a>
<a id="msg-cosmos-feegrant-v1beta1-revokeallowance-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-msggrantallowance-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-msggrantallowanceresponse-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-msgrevokeallowance-j39"></a>
<a id="proto-cosmos-feegrant-v1beta1-msgrevokeallowanceresponse-j39"></a>

### cosmos.feegrant.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/feegrant/v1beta1/tx.proto](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto).

Types: [`message MsgGrantAllowance`](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto#L23), [`message MsgGrantAllowanceResponse`](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto#L35), [`message MsgRevokeAllowance`](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto#L38), [`message MsgRevokeAllowanceResponse`](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto#L47).

Services: [`service Msg`](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto#L11).

Methods: [`rpc GrantAllowance(MsgGrantAllowance) returns (MsgGrantAllowanceResponse)`](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto#L14), [`rpc RevokeAllowance(MsgRevokeAllowance) returns (MsgRevokeAllowanceResponse)`](../sdk/proto/cosmos/feegrant/v1beta1/tx.proto#L18).

<a id="proto-cosmos-genutil-v1beta1-genesisstate-j39"></a>

### cosmos.genutil.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/genutil/v1beta1/genesis.proto](../sdk/proto/cosmos/genutil/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/genutil/v1beta1/genesis.proto#L9).

<a id="proto-cosmos-gov-v1beta1-genesisstate-j39"></a>

### cosmos.gov.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/gov/v1beta1/genesis.proto](../sdk/proto/cosmos/gov/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/gov/v1beta1/genesis.proto#L11).

<a id="proto-cosmos-gov-v1beta1-deposit-j39"></a>
<a id="proto-cosmos-gov-v1beta1-depositparams-j39"></a>
<a id="proto-cosmos-gov-v1beta1-proposal-j39"></a>
<a id="proto-cosmos-gov-v1beta1-proposalstatus-j39"></a>
<a id="proto-cosmos-gov-v1beta1-tallyparams-j39"></a>
<a id="proto-cosmos-gov-v1beta1-tallyresult-j39"></a>
<a id="proto-cosmos-gov-v1beta1-textproposal-j39"></a>
<a id="proto-cosmos-gov-v1beta1-vote-j39"></a>
<a id="proto-cosmos-gov-v1beta1-voteoption-j39"></a>
<a id="proto-cosmos-gov-v1beta1-votingparams-j39"></a>
<a id="proto-cosmos-gov-v1beta1-weightedvoteoption-j39"></a>

### cosmos.gov.v1beta1 — gov.proto

Source: [sdk/proto/cosmos/gov/v1beta1/gov.proto](../sdk/proto/cosmos/gov/v1beta1/gov.proto).

Types: [`enum VoteOption`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L17), [`message WeightedVoteOption`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L35), [`message TextProposal`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L46), [`message Deposit`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L58), [`message Proposal`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L71), [`enum ProposalStatus`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L113), [`message TallyResult`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L136), [`message Vote`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L162), [`message DepositParams`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L177), [`message VotingParams`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L204), [`message TallyParams`](../sdk/proto/cosmos/gov/v1beta1/gov.proto#L223).

<a id="proto-cosmos-gov-v1beta1-querydepositrequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-querydepositresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-querydepositsrequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-querydepositsresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryparamsrequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryparamsresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryproposalrequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryproposalresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryproposalsrequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryproposalsresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-querytallyresultrequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-querytallyresultresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryvoterequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryvoteresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryvotesrequest-j39"></a>
<a id="proto-cosmos-gov-v1beta1-queryvotesresponse-j39"></a>
<a id="qry-cosmos-gov-v1beta1-deposit-j39"></a>
<a id="qry-cosmos-gov-v1beta1-deposits-j39"></a>
<a id="qry-cosmos-gov-v1beta1-params-j39"></a>
<a id="qry-cosmos-gov-v1beta1-proposal-j39"></a>
<a id="qry-cosmos-gov-v1beta1-proposals-j39"></a>
<a id="qry-cosmos-gov-v1beta1-tallyresult-j39"></a>
<a id="qry-cosmos-gov-v1beta1-vote-j39"></a>
<a id="qry-cosmos-gov-v1beta1-votes-j39"></a>
<a id="rest-cosmos-gov-v1beta1-deposit-j39"></a>
<a id="rest-cosmos-gov-v1beta1-deposits-j39"></a>
<a id="rest-cosmos-gov-v1beta1-params-j39"></a>
<a id="rest-cosmos-gov-v1beta1-proposal-j39"></a>
<a id="rest-cosmos-gov-v1beta1-proposals-j39"></a>
<a id="rest-cosmos-gov-v1beta1-tallyresult-j39"></a>
<a id="rest-cosmos-gov-v1beta1-vote-j39"></a>
<a id="rest-cosmos-gov-v1beta1-votes-j39"></a>

### cosmos.gov.v1beta1 — query.proto

Source: [sdk/proto/cosmos/gov/v1beta1/query.proto](../sdk/proto/cosmos/gov/v1beta1/query.proto).

Types: [`message QueryProposalRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L55), [`message QueryProposalResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L61), [`message QueryProposalsRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L66), [`message QueryProposalsResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L85), [`message QueryVoteRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L93), [`message QueryVoteResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L105), [`message QueryVotesRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L111), [`message QueryVotesResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L120), [`message QueryParamsRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L129), [`message QueryParamsResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L136), [`message QueryDepositRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L146), [`message QueryDepositResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L158), [`message QueryDepositsRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L164), [`message QueryDepositsResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L173), [`message QueryTallyResultRequest`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L181), [`message QueryTallyResultResponse`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L187).

Services: [`service Query`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L12).

Methods: [`rpc Proposal(QueryProposalRequest) returns (QueryProposalResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L14), [`rpc Proposals(QueryProposalsRequest) returns (QueryProposalsResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L19), [`rpc Vote(QueryVoteRequest) returns (QueryVoteResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L24), [`rpc Votes(QueryVotesRequest) returns (QueryVotesResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L29), [`rpc Params(QueryParamsRequest) returns (QueryParamsResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L34), [`rpc Deposit(QueryDepositRequest) returns (QueryDepositResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L39), [`rpc Deposits(QueryDepositsRequest) returns (QueryDepositsResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L44), [`rpc TallyResult(QueryTallyResultRequest) returns (QueryTallyResultResponse)`](../sdk/proto/cosmos/gov/v1beta1/query.proto#L49).

<a id="msg-cosmos-gov-v1beta1-deposit-j39"></a>
<a id="msg-cosmos-gov-v1beta1-submitproposal-j39"></a>
<a id="msg-cosmos-gov-v1beta1-vote-j39"></a>
<a id="msg-cosmos-gov-v1beta1-voteweighted-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgdeposit-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgdepositresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgsubmitproposal-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgsubmitproposalresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgvote-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgvoteresponse-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgvoteweighted-j39"></a>
<a id="proto-cosmos-gov-v1beta1-msgvoteweightedresponse-j39"></a>

### cosmos.gov.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/gov/v1beta1/tx.proto](../sdk/proto/cosmos/gov/v1beta1/tx.proto).

Types: [`message MsgSubmitProposal`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L31), [`message MsgSubmitProposalResponse`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L48), [`message MsgVote`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L56), [`message MsgVoteResponse`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L71), [`message MsgVoteWeighted`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L76), [`message MsgVoteWeightedResponse`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L90), [`message MsgDeposit`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L93), [`message MsgDepositResponse`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L111).

Services: [`service Msg`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L13).

Methods: [`rpc SubmitProposal(MsgSubmitProposal) returns (MsgSubmitProposalResponse)`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L15), [`rpc Vote(MsgVote) returns (MsgVoteResponse)`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L18), [`rpc VoteWeighted(MsgVoteWeighted) returns (MsgVoteWeightedResponse)`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L23), [`rpc Deposit(MsgDeposit) returns (MsgDepositResponse)`](../sdk/proto/cosmos/gov/v1beta1/tx.proto#L26).

<a id="proto-cosmos-params-v1beta1-cosmosgasparams-j39"></a>
<a id="proto-cosmos-params-v1beta1-feesparams-j39"></a>
<a id="proto-cosmos-params-v1beta1-genesisstate-j39"></a>

### cosmos.params.v1beta1 — types.proto

Source: [sdk/proto/cosmos/params/types/types.proto](../sdk/proto/cosmos/params/types/types.proto).

Types: [`message FeesParams`](../sdk/proto/cosmos/params/types/types.proto#L11), [`message CosmosGasParams`](../sdk/proto/cosmos/params/types/types.proto#L19), [`message GenesisState`](../sdk/proto/cosmos/params/types/types.proto#L24).

<a id="proto-cosmos-params-v1beta1-paramchange-j39"></a>
<a id="proto-cosmos-params-v1beta1-parameterchangeproposal-j39"></a>

### cosmos.params.v1beta1 — params.proto

Source: [sdk/proto/cosmos/params/v1beta1/params.proto](../sdk/proto/cosmos/params/v1beta1/params.proto).

Types: [`message ParameterChangeProposal`](../sdk/proto/cosmos/params/v1beta1/params.proto#L10), [`message ParamChange`](../sdk/proto/cosmos/params/v1beta1/params.proto#L22).

<a id="proto-cosmos-params-v1beta1-queryparamsrequest-j39"></a>
<a id="proto-cosmos-params-v1beta1-queryparamsresponse-j39"></a>
<a id="qry-cosmos-params-v1beta1-params-j39"></a>
<a id="rest-cosmos-params-v1beta1-params-j39"></a>

### cosmos.params.v1beta1 — query.proto

Source: [sdk/proto/cosmos/params/v1beta1/query.proto](../sdk/proto/cosmos/params/v1beta1/query.proto).

Types: [`message QueryParamsRequest`](../sdk/proto/cosmos/params/v1beta1/query.proto#L20), [`message QueryParamsResponse`](../sdk/proto/cosmos/params/v1beta1/query.proto#L29).

Services: [`service Query`](../sdk/proto/cosmos/params/v1beta1/query.proto#L11).

Methods: [`rpc Params(QueryParamsRequest) returns (QueryParamsResponse)`](../sdk/proto/cosmos/params/v1beta1/query.proto#L14).

<a id="proto-cosmos-slashing-v1beta1-genesisstate-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-genesisstatelegacymissingheights-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-genesisstatelegacyv43-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-missedblock-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-signinginfo-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-signinginfolegacymissedheights-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-validatormissedblocks-j39"></a>

### cosmos.slashing.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/slashing/v1beta1/genesis.proto](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto#L10), [`message GenesisStateLegacyMissingHeights`](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto#L30), [`message GenesisStateLegacyV43`](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto#L50), [`message SigningInfo`](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto#L70), [`message SigningInfoLegacyMissedHeights`](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto#L81), [`message ValidatorMissedBlocks`](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto#L93), [`message MissedBlock`](../sdk/proto/cosmos/slashing/v1beta1/genesis.proto#L104).

<a id="proto-cosmos-slashing-v1beta1-queryparamsrequest-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-queryparamsresponse-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-querysigninginforequest-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-querysigninginforesponse-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-querysigninginfosrequest-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-querysigninginfosresponse-j39"></a>
<a id="qry-cosmos-slashing-v1beta1-params-j39"></a>
<a id="qry-cosmos-slashing-v1beta1-signinginfo-j39"></a>
<a id="qry-cosmos-slashing-v1beta1-signinginfos-j39"></a>
<a id="rest-cosmos-slashing-v1beta1-params-j39"></a>
<a id="rest-cosmos-slashing-v1beta1-signinginfo-j39"></a>
<a id="rest-cosmos-slashing-v1beta1-signinginfos-j39"></a>

### cosmos.slashing.v1beta1 — query.proto

Source: [sdk/proto/cosmos/slashing/v1beta1/query.proto](../sdk/proto/cosmos/slashing/v1beta1/query.proto).

Types: [`message QueryParamsRequest`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L30), [`message QueryParamsResponse`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L33), [`message QuerySigningInfoRequest`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L39), [`message QuerySigningInfoResponse`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L46), [`message QuerySigningInfosRequest`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L53), [`message QuerySigningInfosResponse`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L59).

Services: [`service Query`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L12).

Methods: [`rpc Params(QueryParamsRequest) returns (QueryParamsResponse)`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L14), [`rpc SigningInfo(QuerySigningInfoRequest) returns (QuerySigningInfoResponse)`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L19), [`rpc SigningInfos(QuerySigningInfosRequest) returns (QuerySigningInfosResponse)`](../sdk/proto/cosmos/slashing/v1beta1/query.proto#L24).

<a id="proto-cosmos-slashing-v1beta1-params-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-validatormissedblockarray-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-validatormissedblockarraylegacymissedheights-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-validatorsigninginfo-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-validatorsigninginfolegacymissedheights-j39"></a>

### cosmos.slashing.v1beta1 — slashing.proto

Source: [sdk/proto/cosmos/slashing/v1beta1/slashing.proto](../sdk/proto/cosmos/slashing/v1beta1/slashing.proto).

Types: [`message ValidatorSigningInfoLegacyMissedHeights`](../sdk/proto/cosmos/slashing/v1beta1/slashing.proto#L11), [`message ValidatorSigningInfo`](../sdk/proto/cosmos/slashing/v1beta1/slashing.proto#L33), [`message ValidatorMissedBlockArrayLegacyMissedHeights`](../sdk/proto/cosmos/slashing/v1beta1/slashing.proto#L59), [`message ValidatorMissedBlockArray`](../sdk/proto/cosmos/slashing/v1beta1/slashing.proto#L66), [`message Params`](../sdk/proto/cosmos/slashing/v1beta1/slashing.proto#L75).

<a id="msg-cosmos-slashing-v1beta1-unjail-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-msgunjail-j39"></a>
<a id="proto-cosmos-slashing-v1beta1-msgunjailresponse-j39"></a>

### cosmos.slashing.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/slashing/v1beta1/tx.proto](../sdk/proto/cosmos/slashing/v1beta1/tx.proto).

Types: [`message MsgUnjail`](../sdk/proto/cosmos/slashing/v1beta1/tx.proto#L18), [`message MsgUnjailResponse`](../sdk/proto/cosmos/slashing/v1beta1/tx.proto#L29).

Services: [`service Msg`](../sdk/proto/cosmos/slashing/v1beta1/tx.proto#L10).

Methods: [`rpc Unjail(MsgUnjail) returns (MsgUnjailResponse)`](../sdk/proto/cosmos/slashing/v1beta1/tx.proto#L14).

<a id="proto-cosmos-staking-v1beta1-authorizationtype-j39"></a>
<a id="proto-cosmos-staking-v1beta1-stakeauthorization-j39"></a>
<a id="proto-cosmos-staking-v1beta1-stakeauthorization-validators-j39"></a>

### cosmos.staking.v1beta1 — authz.proto

Source: [sdk/proto/cosmos/staking/v1beta1/authz.proto](../sdk/proto/cosmos/staking/v1beta1/authz.proto).

Types: [`message StakeAuthorization`](../sdk/proto/cosmos/staking/v1beta1/authz.proto#L13), [`message Validators`](../sdk/proto/cosmos/staking/v1beta1/authz.proto#L28), [`enum AuthorizationType`](../sdk/proto/cosmos/staking/v1beta1/authz.proto#L38).

<a id="proto-cosmos-staking-v1beta1-genesisstate-j39"></a>
<a id="proto-cosmos-staking-v1beta1-lastvalidatorpower-j39"></a>

### cosmos.staking.v1beta1 — genesis.proto

Source: [sdk/proto/cosmos/staking/v1beta1/genesis.proto](../sdk/proto/cosmos/staking/v1beta1/genesis.proto).

Types: [`message GenesisState`](../sdk/proto/cosmos/staking/v1beta1/genesis.proto#L10), [`message LastValidatorPower`](../sdk/proto/cosmos/staking/v1beta1/genesis.proto#L48).

<a id="proto-cosmos-staking-v1beta1-querydelegationrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegationresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatordelegationsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatordelegationsresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatorvalidatorrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatorvalidatorresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatorvalidatorsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querydelegatorvalidatorsresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryhistoricalinforequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryhistoricalinforesponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryparamsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryparamsresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querypoolrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-querypoolresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryredelegationsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryredelegationsresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryunbondingdelegationrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryunbondingdelegationresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatordelegationsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatordelegationsresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatorrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatorresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatorsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatorsresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsrequest-j39"></a>
<a id="proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsresponse-j39"></a>
<a id="qry-cosmos-staking-v1beta1-delegation-j39"></a>
<a id="qry-cosmos-staking-v1beta1-delegatordelegations-j39"></a>
<a id="qry-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39"></a>
<a id="qry-cosmos-staking-v1beta1-delegatorvalidator-j39"></a>
<a id="qry-cosmos-staking-v1beta1-delegatorvalidators-j39"></a>
<a id="qry-cosmos-staking-v1beta1-historicalinfo-j39"></a>
<a id="qry-cosmos-staking-v1beta1-params-j39"></a>
<a id="qry-cosmos-staking-v1beta1-pool-j39"></a>
<a id="qry-cosmos-staking-v1beta1-redelegations-j39"></a>
<a id="qry-cosmos-staking-v1beta1-unbondingdelegation-j39"></a>
<a id="qry-cosmos-staking-v1beta1-validator-j39"></a>
<a id="qry-cosmos-staking-v1beta1-validatordelegations-j39"></a>
<a id="qry-cosmos-staking-v1beta1-validators-j39"></a>
<a id="qry-cosmos-staking-v1beta1-validatorunbondingdelegations-j39"></a>
<a id="rest-cosmos-staking-v1beta1-delegation-j39"></a>
<a id="rest-cosmos-staking-v1beta1-delegatordelegations-j39"></a>
<a id="rest-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39"></a>
<a id="rest-cosmos-staking-v1beta1-delegatorvalidator-j39"></a>
<a id="rest-cosmos-staking-v1beta1-delegatorvalidators-j39"></a>
<a id="rest-cosmos-staking-v1beta1-historicalinfo-j39"></a>
<a id="rest-cosmos-staking-v1beta1-params-j39"></a>
<a id="rest-cosmos-staking-v1beta1-pool-j39"></a>
<a id="rest-cosmos-staking-v1beta1-redelegations-j39"></a>
<a id="rest-cosmos-staking-v1beta1-unbondingdelegation-j39"></a>
<a id="rest-cosmos-staking-v1beta1-validator-j39"></a>
<a id="rest-cosmos-staking-v1beta1-validatordelegations-j39"></a>
<a id="rest-cosmos-staking-v1beta1-validators-j39"></a>
<a id="rest-cosmos-staking-v1beta1-validatorunbondingdelegations-j39"></a>

### cosmos.staking.v1beta1 — query.proto

Source: [sdk/proto/cosmos/staking/v1beta1/query.proto](../sdk/proto/cosmos/staking/v1beta1/query.proto).

Types: [`message QueryValidatorsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L99), [`message QueryValidatorsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L108), [`message QueryValidatorRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L117), [`message QueryValidatorResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L123), [`message QueryValidatorDelegationsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L130), [`message QueryValidatorDelegationsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L140), [`message QueryValidatorUnbondingDelegationsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L152), [`message QueryValidatorUnbondingDelegationsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L162), [`message QueryDelegationRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L170), [`message QueryDelegationResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L182), [`message QueryUnbondingDelegationRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L189), [`message QueryUnbondingDelegationResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L202), [`message QueryDelegatorDelegationsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L209), [`message QueryDelegatorDelegationsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L222), [`message QueryDelegatorUnbondingDelegationsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L232), [`message QueryDelegatorUnbondingDelegationsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L245), [`message QueryRedelegationsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L254), [`message QueryRedelegationsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L273), [`message QueryDelegatorValidatorsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L282), [`message QueryDelegatorValidatorsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L295), [`message QueryDelegatorValidatorRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L305), [`message QueryDelegatorValidatorResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L318), [`message QueryHistoricalInfoRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L325), [`message QueryHistoricalInfoResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L332), [`message QueryPoolRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L338), [`message QueryPoolResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L341), [`message QueryParamsRequest`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L347), [`message QueryParamsResponse`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L350).

Services: [`service Query`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L12).

Methods: [`rpc Validators(QueryValidatorsRequest) returns (QueryValidatorsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L14), [`rpc Validator(QueryValidatorRequest) returns (QueryValidatorResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L19), [`rpc ValidatorDelegations(QueryValidatorDelegationsRequest) returns (QueryValidatorDelegationsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L24), [`rpc ValidatorUnbondingDelegations(QueryValidatorUnbondingDelegationsRequest) returns (QueryValidatorUnbondingDelegationsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L29), [`rpc Delegation(QueryDelegationRequest) returns (QueryDelegationResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L36), [`rpc UnbondingDelegation(QueryUnbondingDelegationRequest) returns (QueryUnbondingDelegationResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L44), [`rpc DelegatorDelegations(QueryDelegatorDelegationsRequest) returns (QueryDelegatorDelegationsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L51), [`rpc DelegatorUnbondingDelegations(QueryDelegatorUnbondingDelegationsRequest) returns (QueryDelegatorUnbondingDelegationsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L57), [`rpc Redelegations(QueryRedelegationsRequest) returns (QueryRedelegationsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L64), [`rpc DelegatorValidators(QueryDelegatorValidatorsRequest) returns (QueryDelegatorValidatorsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L70), [`rpc DelegatorValidator(QueryDelegatorValidatorRequest) returns (QueryDelegatorValidatorResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L76), [`rpc HistoricalInfo(QueryHistoricalInfoRequest) returns (QueryHistoricalInfoResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L83), [`rpc Pool(QueryPoolRequest) returns (QueryPoolResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L88), [`rpc Params(QueryParamsRequest) returns (QueryParamsResponse)`](../sdk/proto/cosmos/staking/v1beta1/query.proto#L93).

<a id="proto-cosmos-staking-v1beta1-bondstatus-j39"></a>
<a id="proto-cosmos-staking-v1beta1-commission-j39"></a>
<a id="proto-cosmos-staking-v1beta1-commissionrates-j39"></a>
<a id="proto-cosmos-staking-v1beta1-delegation-j39"></a>
<a id="proto-cosmos-staking-v1beta1-delegationresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-description-j39"></a>
<a id="proto-cosmos-staking-v1beta1-dvpair-j39"></a>
<a id="proto-cosmos-staking-v1beta1-dvpairs-j39"></a>
<a id="proto-cosmos-staking-v1beta1-dvvtriplet-j39"></a>
<a id="proto-cosmos-staking-v1beta1-dvvtriplets-j39"></a>
<a id="proto-cosmos-staking-v1beta1-historicalinfo-j39"></a>
<a id="proto-cosmos-staking-v1beta1-params-j39"></a>
<a id="proto-cosmos-staking-v1beta1-pool-j39"></a>
<a id="proto-cosmos-staking-v1beta1-redelegation-j39"></a>
<a id="proto-cosmos-staking-v1beta1-redelegationentry-j39"></a>
<a id="proto-cosmos-staking-v1beta1-redelegationentryresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-redelegationresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-unbondingdelegation-j39"></a>
<a id="proto-cosmos-staking-v1beta1-unbondingdelegationentry-j39"></a>
<a id="proto-cosmos-staking-v1beta1-valaddresses-j39"></a>
<a id="proto-cosmos-staking-v1beta1-validator-j39"></a>

### cosmos.staking.v1beta1 — staking.proto

Source: [sdk/proto/cosmos/staking/v1beta1/staking.proto](../sdk/proto/cosmos/staking/v1beta1/staking.proto).

Types: [`message HistoricalInfo`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L18), [`message CommissionRates`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L25), [`message Commission`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L49), [`message Description`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L67), [`message Validator`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L91), [`enum BondStatus`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L139), [`message ValAddresses`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L153), [`message DVPair`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L163), [`message DVPairs`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L173), [`message DVVTriplet`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L181), [`message DVVTriplets`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L192), [`message Delegation`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L199), [`message UnbondingDelegation`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L217), [`message UnbondingDelegationEntry`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L231), [`message RedelegationEntry`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L257), [`message Redelegation`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L284), [`message Params`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L300), [`message DelegationResponse`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L342), [`message RedelegationEntryResponse`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L354), [`message RedelegationResponse`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L367), [`message Pool`](../sdk/proto/cosmos/staking/v1beta1/staking.proto#L376).

<a id="msg-cosmos-staking-v1beta1-beginredelegate-j39"></a>
<a id="msg-cosmos-staking-v1beta1-createvalidator-j39"></a>
<a id="msg-cosmos-staking-v1beta1-delegate-j39"></a>
<a id="msg-cosmos-staking-v1beta1-editvalidator-j39"></a>
<a id="msg-cosmos-staking-v1beta1-undelegate-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgbeginredelegate-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgbeginredelegateresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgcreatevalidator-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgcreatevalidatorresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgdelegate-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgdelegateresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgeditvalidator-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgeditvalidatorresponse-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgundelegate-j39"></a>
<a id="proto-cosmos-staking-v1beta1-msgundelegateresponse-j39"></a>

### cosmos.staking.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/staking/v1beta1/tx.proto](../sdk/proto/cosmos/staking/v1beta1/tx.proto).

Types: [`message MsgCreateValidator`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L35), [`message MsgCreateValidatorResponse`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L53), [`message MsgEditValidator`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L56), [`message MsgEditValidatorResponse`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L78), [`message MsgDelegate`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L82), [`message MsgDelegateResponse`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L91), [`message MsgBeginRedelegate`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L95), [`message MsgBeginRedelegateResponse`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L105), [`message MsgUndelegate`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L114), [`message MsgUndelegateResponse`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L123).

Services: [`service Msg`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L14).

Methods: [`rpc CreateValidator(MsgCreateValidator) returns (MsgCreateValidatorResponse)`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L16), [`rpc EditValidator(MsgEditValidator) returns (MsgEditValidatorResponse)`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L19), [`rpc Delegate(MsgDelegate) returns (MsgDelegateResponse)`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L23), [`rpc BeginRedelegate(MsgBeginRedelegate) returns (MsgBeginRedelegateResponse)`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L27), [`rpc Undelegate(MsgUndelegate) returns (MsgUndelegateResponse)`](../sdk/proto/cosmos/staking/v1beta1/tx.proto#L31).

<a id="proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-j39"></a>
<a id="proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-multi-j39"></a>
<a id="proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-single-j39"></a>
<a id="proto-cosmos-tx-signing-v1beta1-signaturedescriptor-j39"></a>
<a id="proto-cosmos-tx-signing-v1beta1-signaturedescriptors-j39"></a>
<a id="proto-cosmos-tx-signing-v1beta1-signmode-j39"></a>

### cosmos.tx.signing.v1beta1 — signing.proto

Source: [sdk/proto/cosmos/tx/signing/v1beta1/signing.proto](../sdk/proto/cosmos/tx/signing/v1beta1/signing.proto).

Types: [`enum SignMode`](../sdk/proto/cosmos/tx/signing/v1beta1/signing.proto#L10), [`message SignatureDescriptors`](../sdk/proto/cosmos/tx/signing/v1beta1/signing.proto#L42), [`message SignatureDescriptor`](../sdk/proto/cosmos/tx/signing/v1beta1/signing.proto#L51), [`message Data`](../sdk/proto/cosmos/tx/signing/v1beta1/signing.proto#L63), [`message Single`](../sdk/proto/cosmos/tx/signing/v1beta1/signing.proto#L74), [`message Multi`](../sdk/proto/cosmos/tx/signing/v1beta1/signing.proto#L83).

<a id="grpcs-cosmos-tx-v1beta1-service-broadcasttx-j39"></a>
<a id="grpcs-cosmos-tx-v1beta1-service-getblockwithtxs-j39"></a>
<a id="grpcs-cosmos-tx-v1beta1-service-gettx-j39"></a>
<a id="grpcs-cosmos-tx-v1beta1-service-gettxsevent-j39"></a>
<a id="grpcs-cosmos-tx-v1beta1-service-simulate-j39"></a>
<a id="proto-cosmos-tx-v1beta1-broadcastmode-j39"></a>
<a id="proto-cosmos-tx-v1beta1-broadcasttxrequest-j39"></a>
<a id="proto-cosmos-tx-v1beta1-broadcasttxresponse-j39"></a>
<a id="proto-cosmos-tx-v1beta1-getblockwithtxsrequest-j39"></a>
<a id="proto-cosmos-tx-v1beta1-getblockwithtxsresponse-j39"></a>
<a id="proto-cosmos-tx-v1beta1-gettxrequest-j39"></a>
<a id="proto-cosmos-tx-v1beta1-gettxresponse-j39"></a>
<a id="proto-cosmos-tx-v1beta1-gettxseventrequest-j39"></a>
<a id="proto-cosmos-tx-v1beta1-gettxseventresponse-j39"></a>
<a id="proto-cosmos-tx-v1beta1-orderby-j39"></a>
<a id="proto-cosmos-tx-v1beta1-simulaterequest-j39"></a>
<a id="proto-cosmos-tx-v1beta1-simulateresponse-j39"></a>
<a id="rest-cosmos-tx-v1beta1-service-broadcasttx-j39"></a>
<a id="rest-cosmos-tx-v1beta1-service-getblockwithtxs-j39"></a>
<a id="rest-cosmos-tx-v1beta1-service-gettx-j39"></a>
<a id="rest-cosmos-tx-v1beta1-service-gettxsevent-j39"></a>
<a id="rest-cosmos-tx-v1beta1-service-simulate-j39"></a>

### cosmos.tx.v1beta1 — service.proto

Source: [sdk/proto/cosmos/tx/v1beta1/service.proto](../sdk/proto/cosmos/tx/v1beta1/service.proto).

Types: [`message GetTxsEventRequest`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L49), [`enum OrderBy`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L58), [`message GetTxsEventResponse`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L69), [`message BroadcastTxRequest`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L80), [`enum BroadcastMode`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L87), [`message BroadcastTxResponse`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L103), [`message SimulateRequest`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L110), [`message SimulateResponse`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L122), [`message GetTxRequest`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L131), [`message GetTxResponse`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L137), [`message GetBlockWithTxsRequest`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L148), [`message GetBlockWithTxsResponse`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L158).

Services: [`service Service`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L16).

Methods: [`rpc Simulate(SimulateRequest) returns (SimulateResponse)`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L18), [`rpc GetTx(GetTxRequest) returns (GetTxResponse)`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L25), [`rpc BroadcastTx(BroadcastTxRequest) returns (BroadcastTxResponse)`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L29), [`rpc GetTxsEvent(GetTxsEventRequest) returns (GetTxsEventResponse)`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L36), [`rpc GetBlockWithTxs(GetBlockWithTxsRequest) returns (GetBlockWithTxsResponse)`](../sdk/proto/cosmos/tx/v1beta1/service.proto#L42).

<a id="proto-cosmos-tx-v1beta1-authinfo-j39"></a>
<a id="proto-cosmos-tx-v1beta1-fee-j39"></a>
<a id="proto-cosmos-tx-v1beta1-modeinfo-j39"></a>
<a id="proto-cosmos-tx-v1beta1-modeinfo-multi-j39"></a>
<a id="proto-cosmos-tx-v1beta1-modeinfo-single-j39"></a>
<a id="proto-cosmos-tx-v1beta1-signdoc-j39"></a>
<a id="proto-cosmos-tx-v1beta1-signerinfo-j39"></a>
<a id="proto-cosmos-tx-v1beta1-tx-j39"></a>
<a id="proto-cosmos-tx-v1beta1-txbody-j39"></a>
<a id="proto-cosmos-tx-v1beta1-txraw-j39"></a>

### cosmos.tx.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/tx/v1beta1/tx.proto](../sdk/proto/cosmos/tx/v1beta1/tx.proto).

Types: [`message Tx`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L13), [`message TxRaw`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L32), [`message SignDoc`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L48), [`message TxBody`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L67), [`message AuthInfo`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L99), [`message SignerInfo`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L115), [`message ModeInfo`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L132), [`message Single`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L146), [`message Multi`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L152), [`message Fee`](../sdk/proto/cosmos/tx/v1beta1/tx.proto#L165).

<a id="proto-cosmos-upgrade-v1beta1-queryappliedplanrequest-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-queryappliedplanresponse-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-querycurrentplanrequest-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-querycurrentplanresponse-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-querymoduleversionsrequest-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-querymoduleversionsresponse-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstaterequest-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstateresponse-j39"></a>
<a id="qry-cosmos-upgrade-v1beta1-appliedplan-j39"></a>
<a id="qry-cosmos-upgrade-v1beta1-currentplan-j39"></a>
<a id="qry-cosmos-upgrade-v1beta1-moduleversions-j39"></a>
<a id="qry-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39"></a>
<a id="rest-cosmos-upgrade-v1beta1-appliedplan-j39"></a>
<a id="rest-cosmos-upgrade-v1beta1-currentplan-j39"></a>
<a id="rest-cosmos-upgrade-v1beta1-moduleversions-j39"></a>
<a id="rest-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39"></a>

### cosmos.upgrade.v1beta1 — query.proto

Source: [sdk/proto/cosmos/upgrade/v1beta1/query.proto](../sdk/proto/cosmos/upgrade/v1beta1/query.proto).

Types: [`message QueryCurrentPlanRequest`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L42), [`message QueryCurrentPlanResponse`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L46), [`message QueryAppliedPlanRequest`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L53), [`message QueryAppliedPlanResponse`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L60), [`message QueryUpgradedConsensusStateRequest`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L67), [`message QueryUpgradedConsensusStateResponse`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L77), [`message QueryModuleVersionsRequest`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L89), [`message QueryModuleVersionsResponse`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L100).

Services: [`service Query`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L10).

Methods: [`rpc CurrentPlan(QueryCurrentPlanRequest) returns (QueryCurrentPlanResponse)`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L12), [`rpc AppliedPlan(QueryAppliedPlanRequest) returns (QueryAppliedPlanResponse)`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L17), [`rpc UpgradedConsensusState(QueryUpgradedConsensusStateRequest) returns (QueryUpgradedConsensusStateResponse)`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L27), [`rpc ModuleVersions(QueryModuleVersionsRequest) returns (QueryModuleVersionsResponse)`](../sdk/proto/cosmos/upgrade/v1beta1/query.proto#L35).

<a id="proto-cosmos-upgrade-v1beta1-cancelsoftwareupgradeproposal-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-moduleversion-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-plan-j39"></a>
<a id="proto-cosmos-upgrade-v1beta1-softwareupgradeproposal-j39"></a>

### cosmos.upgrade.v1beta1 — upgrade.proto

Source: [sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto](../sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto).

Types: [`message Plan`](../sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto#L12), [`message SoftwareUpgradeProposal`](../sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto#L53), [`message CancelSoftwareUpgradeProposal`](../sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto#L64), [`message ModuleVersion`](../sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto#L75).

<a id="msg-cosmos-vesting-v1beta1-createvestingaccount-j39"></a>
<a id="proto-cosmos-vesting-v1beta1-msgcreatevestingaccount-j39"></a>
<a id="proto-cosmos-vesting-v1beta1-msgcreatevestingaccountresponse-j39"></a>

### cosmos.vesting.v1beta1 — tx.proto

Source: [sdk/proto/cosmos/vesting/v1beta1/tx.proto](../sdk/proto/cosmos/vesting/v1beta1/tx.proto).

Types: [`message MsgCreateVestingAccount`](../sdk/proto/cosmos/vesting/v1beta1/tx.proto#L18), [`message MsgCreateVestingAccountResponse`](../sdk/proto/cosmos/vesting/v1beta1/tx.proto#L34).

Services: [`service Msg`](../sdk/proto/cosmos/vesting/v1beta1/tx.proto#L10).

Methods: [`rpc CreateVestingAccount(MsgCreateVestingAccount) returns (MsgCreateVestingAccountResponse)`](../sdk/proto/cosmos/vesting/v1beta1/tx.proto#L13).

<a id="proto-cosmos-vesting-v1beta1-basevestingaccount-j39"></a>
<a id="proto-cosmos-vesting-v1beta1-continuousvestingaccount-j39"></a>
<a id="proto-cosmos-vesting-v1beta1-delayedvestingaccount-j39"></a>
<a id="proto-cosmos-vesting-v1beta1-period-j39"></a>
<a id="proto-cosmos-vesting-v1beta1-periodicvestingaccount-j39"></a>
<a id="proto-cosmos-vesting-v1beta1-permanentlockedaccount-j39"></a>

### cosmos.vesting.v1beta1 — vesting.proto

Source: [sdk/proto/cosmos/vesting/v1beta1/vesting.proto](../sdk/proto/cosmos/vesting/v1beta1/vesting.proto).

Types: [`message BaseVestingAccount`](../sdk/proto/cosmos/vesting/v1beta1/vesting.proto#L12), [`message ContinuousVestingAccount`](../sdk/proto/cosmos/vesting/v1beta1/vesting.proto#L41), [`message DelayedVestingAccount`](../sdk/proto/cosmos/vesting/v1beta1/vesting.proto#L52), [`message Period`](../sdk/proto/cosmos/vesting/v1beta1/vesting.proto#L60), [`message PeriodicVestingAccount`](../sdk/proto/cosmos/vesting/v1beta1/vesting.proto#L72), [`message PermanentLockedAccount`](../sdk/proto/cosmos/vesting/v1beta1/vesting.proto#L89).
