# Explorer source verifier transport contract

The explorer's smart-contract verifier compares supplied or retrieved source
compilation output with contract bytecode. It is an explorer service, not a
consensus or transaction-finality service. Paxeer is one BFT network with EVM
chain ID `125` (`0x7d`); the kernel participates in block proposal, and
receipts are final when their block commits. A source match does not change
that lifecycle.

See [node RPC](rpc.md) to read chain bytecode and
[JavaScript SDK](sdk-js.md) for native-module clients. The SDK REST `Querier`
does not implement this verifier API. A verifier deployment address must be
configured explicitly; the public node JSON-RPC origin does not imply a
verifier deployment on that host.

## Sources of truth

Paths below are relative to
[`services/explorer/services/smart-contract-verifier`](../services/explorer/services/smart-contract-verifier):

| Contract | Source |
| --- | --- |
| Solidity, Vyper, Sourcify services and messages | [proto/v2/smart-contract-verifier.proto](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto) |
| ZkSync Solidity service and messages | [proto/v2/zksync-solidity.proto](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/zksync-solidity.proto) |
| Health service | [proto/v2/health.proto](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/health.proto) |
| HTTP verbs and paths | [api_config_http.yaml](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml) |
| Route/service registration | [server/src/run.rs](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/run.rs) |
| Generated JSON representation | [proto/build.rs](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/build.rs), using `actix-prost-macros` 0.1.0 |

The build generates the HTTP handlers and gRPC services from v2. Retained
v1 proto files do not establish live v1 routes.

## Deployment and transport

The shared launcher defaults to HTTP enabled at `0.0.0.0:8050` and gRPC
disabled at `0.0.0.0:8051`. HTTP request bodies default to a maximum of
`2097152` bytes (`server.http.max_body_size`). These are local configuration
defaults, not public endpoint guarantees. See
[launcher settings](../services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs)
and [verifier settings](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/settings.rs).

Configuration is loaded from `SMART_CONTRACT_VERIFIER__CONFIG` when set,
then environment overrides using `__` between nested keys, for example
`SMART_CONTRACT_VERIFIER__SERVER__HTTP__BASE_PATH`. Setting
`server.http.base_path` mounts all HTTP routes under that prefix: with
`/explorer-verifier`, health is `/explorer-verifier/health`. The prefix does
not change gRPC service names. With no base path, the routes below are
relative to the origin root.

`solidity.enabled`, `vyper.enabled`, `sourcify.enabled` and
`zksync_solidity.enabled` control registration on both transports. They
default to true. A disabled family is not registered. Health is registered
independently. HTTP and gRPC listeners must themselves be enabled.

The Rust HTTP client in
[http_client/client.rs](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/src/http_client/client.rs)
sets absolute route paths with `Url::set_path`; it does not preserve a prefix
in its configured base URL. A prefixed deployment therefore needs a caller
that constructs the full path or proxy mapping that exposes the root API.
Do not assume a base URL path is carried forward by that client.

All mapped POST methods accept `Content-Type: application/json` with the
whole request as the body (`body: "*"`). They are not MIME multipart uploads,
despite the `MultiPart` method name. GET inputs use query parameters. gRPC
uses the same messages in protobuf wire form and unary methods.

## JSON encoding

The generated Actix serde layer uses lower camel case: `bytecodeType`,
`compilerVersion`, `sourceFiles`, `optimizationRuns`, `postActions`, and so
on. Maps are JSON objects, with string keys; `fileIds` uses decimal uint32
keys as JSON object keys. Enum values are symbolic uppercase strings, such
as `CREATION_INPUT`, `DEPLOYED_BYTECODE`, `SUCCESS`, `FAILURE`, `PARTIAL` and
`FULL`. Optional message/scalar values may be absent or null; preserve the
distinction between an absent option and an explicit numeric zero.

Bytecode, constructor arguments and returned code parts are hex strings
(use `0x`-prefixed even-length hex), not base64 byte arrays. These proto
fields are strings. Source maps such as `sourceFiles`, `sources`, `files`
and `interfaces` map names/paths to string contents. `input` is a string
containing standard input JSON, not an embedded object. Likewise `abi`,
`compilerSettings`, artifact fields, match `values` and `transformations`
contain JSON text inside strings; parse them separately if needed.

Send all required nonoptional fields, including empty maps where appropriate.
Do not assume the HTTP decoder implements every protobuf JSON default.
The build explicitly defaults omitted health `service`, Vyper `interfaces`
and Solidity `postActions`. Integer counts here are 32-bit JSON numbers.

## grpc-solidityverifier

gRPC service: `blockscout.smartContractVerifier.v2.SolidityVerifier`.
For example, the full method name is
`/blockscout.smartContractVerifier.v2.SolidityVerifier/VerifyMultiPart`.

| Method | HTTP | Response |
| --- | --- | --- |
| `VerifyMultiPart` | POST `/api/v2/verifier/solidity/sources:verify-multi-part` | `VerifyResponse` |
| `VerifyStandardJson` | POST `/api/v2/verifier/solidity/sources:verify-standard-json` | `VerifyResponse` |
| `BatchVerifyMultiPart` | POST `/api/v2/verifier/solidity/sources:batch-verify-multi-part` | `BatchVerifyResponse` |
| `BatchVerifyStandardJson` | POST `/api/v2/verifier/solidity/sources:batch-verify-standard-json` | `BatchVerifyResponse` |
| `ListCompilerVersions` | GET `/api/v2/verifier/solidity/versions` | `ListCompilerVersionsResponse` |
| `LookupMethods` | POST `/api/v2/verifier/solidity/methods:lookup` | `LookupMethodsResponse` |

Single multipart requests contain `bytecode`, `bytecodeType`,
`compilerVersion`, `sourceFiles`, optional `evmVersion`, optional signed
int32 `optimizationRuns`, optional `metadata`, and `postActions` (default
empty). An absent optimization count disables optimization; a supplied count
enables it. Standard JSON requests replace the multipart source/settings
fields with `input`, retaining bytecode/type, compiler, metadata and actions.

`metadata` has optional string `chainId` and `contractAddress`; it provides
explorer context, not a trusted chain proof. For Paxeer, the chain ID string
is `"125"`. The proto reserves the old top-level `libraries` fields: place
compiler library settings in standard JSON input instead.

The proto declares `postActions: ["lookup-methods"]`, but the current solo
conversion/service path does not execute those actions: success construction
sets `postActionResponses.lookupMethods` to null. Use `LookupMethods`
explicitly when method locations are required.

Batch requests share `compilerVersion` and a `contracts` array. Each contract
contains optional hex `creationCode`, optional hex `runtimeCode` and optional
metadata. Supply code to compare. Multipart batches use `sources` (not
`sourceFiles`), optional `evmVersion`, and optional uint32 `optimizationRuns`.
Standard JSON batches use `input` instead of those source/settings fields.

## grpc-vyperverifier

gRPC service: `blockscout.smartContractVerifier.v2.VyperVerifier`.

| Method | HTTP | Response |
| --- | --- | --- |
| `VerifyMultiPart` | POST `/api/v2/verifier/vyper/sources:verify-multi-part` | `VerifyResponse` |
| `VerifyStandardJson` | POST `/api/v2/verifier/vyper/sources:verify-standard-json` | `VerifyResponse` |
| `ListCompilerVersions` | GET `/api/v2/verifier/vyper/versions` | `ListCompilerVersionsResponse` |

Multipart input contains `bytecode`, `bytecodeType`, `compilerVersion`,
`sourceFiles`, optional `evmVersion`, optional `metadata`, and `interfaces`
(default empty). The old `optimizations` field is reserved. Standard JSON
uses `bytecode`, `bytecodeType`, `compilerVersion`, string `input` and
optional metadata. There are no Vyper batch methods in this service.

## grpc-sourcifyverifier

gRPC service: `blockscout.smartContractVerifier.v2.SourcifyVerifier`.

| Method | HTTP | Response |
| --- | --- | --- |
| `Verify` | POST `/api/v2/verifier/sourcify/sources:verify` | `VerifyResponse` |
| `VerifyFromEtherscan` | POST `/api/v2/verifier/sourcify/sources:verify-from-etherscan` | `VerifyResponse` |

`Verify` accepts string `address`, string `chain`, a `files` map and optional
int32 `chosenContract`. `VerifyFromEtherscan` accepts only `address` and
`chain`. These methods call the configured Sourcify service; supported chains,
imports and upstream availability depend on that service. The existence of
the route does not promise Sourcify support for chain 125.

## grpc-zksyncsolidityverifier

gRPC service:
`blockscout.smartContractVerifier.v2.zksync.solidity.ZkSyncSolidityVerifier`.
This is an optional verifier capability, not a second Paxeer network.

| Method | HTTP | Response |
| --- | --- | --- |
| `VerifyStandardJson` | POST `/api/v2/zksync-verifier/solidity/sources:verify-standard-json` | ZkSync `VerifyResponse` |
| `ListCompilers` | GET `/api/v2/zksync-verifier/solidity/versions` | `ListCompilersResponse` |

The verification request contains hex `code`, optional hex
`constructorArguments`, `zkCompiler`, `solcCompiler` and string `input`.
Its response is a flattened oneof with one of `compilationFailure`,
`verificationFailure` or `verificationSuccess`. Failure objects contain
`message`. Success contains file/contract names, `zkCompiler` and
`evmCompiler` descriptors, language, settings, source map, artifact strings,
optional `creationMatch` and `runtimeMatch`. Match objects contain `type`,
JSON-text `values` and JSON-text `transformations`. This response is distinct
from the ordinary `VerifyResponse` status envelope.

## verifier-versions

Solidity and Vyper versions endpoints accept empty requests and return
`{"compilerVersions":[...]}`. Implementations sort versions descending.
ZkSync returns `zkCompilers` and `solcCompilers` arrays. Select an exact
advertised compiler identifier rather than assuming an installed system
compiler or a version inferred from a pragma. Availability depends on the
configured fetchers and refresh cycle; unsupported compilers return an error.

For a root-mounted local service, for example:

```bash
curl --fail-with-body --max-time 15 \
  http://127.0.0.1:8050/api/v2/verifier/solidity/versions
```

## verifier-methods

`LookupMethods` accepts hex `bytecode`, string `abi` containing an ABI JSON
array, string `sourceMap`, and `fileIds` mapping compiler source IDs to file
names. It returns `methods`, a map from selector to
`{fileName, fileOffset, length}` with uint32 byte offsets/lengths. Invalid
bytecode, ABI or source map returns `InvalidArgument`. Source locations are
lookup results, not proof of a contract source match.

## verifier-sources

Inspect the response body even when HTTP is 200. Ordinary `VerifyResponse`
has `message`, `status`, `source`, `extraData` and `postActionResponses`.
`SUCCESS` supplies a source; `FAILURE` reports compilation or match failure,
normally with null source/extra/action fields. `STATUS_UNSPECIFIED` is not
verification success.

A source contains `fileName`, `contractName`, `compilerVersion`,
`compilerSettings`, `sourceType` (`SOLIDITY`, `VYPER` or `YUL`), `sourceFiles`,
optional `abi`, optional `constructorArguments`, `matchType`, optional
artifact strings, `isBlueprint` and `libraries`. `PARTIAL` and `FULL` express
the code/metadata match, not chain finality. `libraries` maps fully qualified
library names to addresses.

`extraData` contains `localCreationInputParts` and
`localDeployedBytecodeParts`, arrays of `{type, data}` with `main`/`meta`
parts and hex data. Sourcify returns empty arrays here.

Solidity batch responses instead flatten their oneof into either
`{"compilationFailure":{"message":"..."}}` or
`{"contractVerificationResults":{"items":[...]}}`. Each item is itself
either `{"failure":{}}` or `{"success":{...}}`, in input order. Success
contains creation/runtime code, compiler/language/version, file/contract
names, `sources`, JSON-text settings/artifacts and optional creation/runtime
match details. Check each item; a successful HTTP batch call does not mean
every contract matched.

## grpc-health

gRPC service `blockscout.smartContractVerifier.v2.Health`, method `Check`,
accepts `HealthCheckRequest { service }` and returns
`HealthCheckResponse { status }`. There is no registered streaming `Watch`
method. Only an empty service string or the exact name
`blockscout.smartContractVerifier.v2.Health` is accepted.

## verifier-health

GET `/health` maps to `Health.Check`. With an omitted/empty `service`, or the
exact health service name, it returns HTTP 200 with `{"status":"SERVING"}`.
Any other nonempty name, including a verifier service name, returns gRPC
`NotFound` (5), mapped to HTTP 404 with
`{"code":5,"message":"unknown service"}`. A successful health check does
not prove a language family is enabled, a compiler is downloadable, Sourcify
is reachable or a contract will match.

## Errors and caller behavior

Service errors use tonic status codes. `actix-prost` 0.1.0 maps those to HTTP
JSON `{code: <numeric gRPC code>, message: <string>}`. Relevant mappings are:

| gRPC status | Code | HTTP |
| --- | --- | --- |
| `InvalidArgument` | 3 | 400 |
| `NotFound` | 5 | 404 |
| `Internal` | 13 | 500 |
| `Unavailable` | 14 | 503 |
| `DeadlineExceeded` | 4 | 504 |

Malformed bytecode, unsupported compilers and bad request conversion can
produce `InvalidArgument`. Invalid standard-input content and compiler
diagnostics can instead be returned as HTTP 200 verification failure or a
batch/ZkSync compilation-failure variant. Internal errors become transport
errors. HTTP routing, JSON extraction and payload-size rejection occur
before service execution and need not have the verification response shape.
See [service conversion rules](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/services/common.rs)
and [verification result handling](../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/types/verification_result.rs).

Use client deadlines and report transport errors separately from compilation
and mismatch results. The Rust HTTP client's success path deserializes the
body; it does not turn a `FAILURE` status into a transport error. Non-success
HTTP statuses return its `StatusCode` error. Verification calls return their
result in the response; this API exposes no durable job ID or polling route
for resuming an interrupted verification after restart.
