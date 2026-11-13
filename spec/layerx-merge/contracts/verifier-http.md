# Verifier http contract

The [smart-contract-verifier workspace](../../../services/explorer/services/smart-contract-verifier) compiles source and compares bytecode for the explorer. Paxeer is one BFT network with an integrated kernel, EVM chain ID `125` (`0x7d`), and finality at block commitment. Tenant domains are database namespaces.

The verifier routes below belong to its configured service origin, independently of node root-path JSON-RPC. Hosted `px_` methods are declared by the [gateway OpenRPC contract](../../../services/platform/hosted/gateway/openrpc.json). For signed transaction receipts, see [Platform examples](platform-examples.md#raw-transaction-receipt-contract).

## Transport and implementation provenance

[Server startup](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/run.rs) registers generated Actix HTTP handlers and Tonic gRPC services. [api_config_http.yaml](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml) maps HTTP verbs and paths to protobuf methods; [build.rs](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/build.rs) generates both transports from the v2 protos. JSON names use camelCase; protobuf identifiers below use snake_case.

The server inherits `blockscout-service-launcher` from the [workspace manifest](../../../services/explorer/services/smart-contract-verifier/Cargo.toml). The [workspace lockfile](../../../services/explorer/services/smart-contract-verifier/Cargo.lock) resolves **registry version 0.17.0**, not the similarly named local crate under `services/explorer/services/libs`. Transport and configuration claims here refer to the published [0.17.0 settings](https://docs.rs/crate/blockscout-service-launcher/0.17.0/source/src/launcher/settings.rs) and [launch implementation](https://docs.rs/crate/blockscout-service-launcher/0.17.0/source/src/launcher/launch.rs). Handler status-to-HTTP conversion belongs to [actix-prost 0.1.0](https://docs.rs/crate/actix-prost/0.1.0/source/src/error.rs), also resolved in that lockfile.

[Settings](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/settings.rs) implements `ConfigSettings` with service name `SMART_CONTRACT_VERIFIER`. The linked launcher reads an optional file named by `SMART_CONTRACT_VERIFIER__CONFIG`, then overlays environment variables with that service prefix and `__` separators. Unknown settings fields are rejected by the server settings types.

| Setting | Environment variable | Linked default |
|---|---|---|
| `server.http.enabled` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__ENABLED` | `true` |
| `server.http.addr` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__ADDR` | port `8050`, all interfaces |
| `server.http.max_body_size` | `SMART_CONTRACT_VERIFIER__SERVER__HTTP__MAX_BODY_SIZE` | `2097152` bytes for the HTTP JSON extractor |
| `server.grpc.enabled` | `SMART_CONTRACT_VERIFIER__SERVER__GRPC__ENABLED` | `false` |
| `server.grpc.addr` | `SMART_CONTRACT_VERIFIER__SERVER__GRPC__ADDR` | port `8051`, all interfaces |

`solidity.enabled`, `vyper.enabled`, `sourcify.enabled` and `zksync_solidity.enabled` control their registrations on both transports; all default to `true`. Their environment variables follow the same spelling, for example `SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ENABLED`. Health is always registered on each enabled transport. Compiler service initialization precedes listener launch and can fail startup. The base config is not an exhaustive inventory: use the settings types for supported options, including ZkSync.

## grpc-health

The service is `blockscout.smartContractVerifier.v2.Health`, with unary method `Check(HealthCheckRequest) -> HealthCheckResponse`, defined in [health.proto](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/health.proto). Its gRPC method path is `/blockscout.smartContractVerifier.v2.Health/Check`.

The current [HealthService::check](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/services/health.rs) accepts an empty `service` or exactly `blockscout.smartContractVerifier.v2.Health` and returns `SERVING`. Other nonempty names return Tonic `NOT_FOUND` with message `unknown service`. This is a liveness response, not a probe of every compiler, upstream dependency or chain head. Streaming `Watch` is not implemented.

## grpc-solidityverifier

`blockscout.smartContractVerifier.v2.SolidityVerifier` is declared in [smart-contract-verifier.proto](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto) and implemented by [SolidityVerifierService](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/services/solidity_verifier.rs).

| Method | Request | Response |
|---|---|---|
| `VerifyMultiPart` | `VerifySolidityMultiPartRequest` | `VerifyResponse` |
| `VerifyStandardJson` | `VerifySolidityStandardJsonRequest` | `VerifyResponse` |
| `BatchVerifyMultiPart` | `BatchVerifySolidityMultiPartRequest` | `BatchVerifyResponse` |
| `BatchVerifyStandardJson` | `BatchVerifySolidityStandardJsonRequest` | `BatchVerifyResponse` |
| `ListCompilerVersions` | `ListCompilerVersionsRequest` | `ListCompilerVersionsResponse` |
| `LookupMethods` | `LookupMethodsRequest` | `LookupMethodsResponse` |

Each gRPC method path is `/<fully-qualified-service>/<Method>`. Multipart means a JSON/protobuf map of files, not an HTTP multipart form upload. Its fields include `bytecode`, `bytecode_type`, `compiler_version`, optional `evm_version` and `optimization_runs`, `source_files`, optional `metadata`, and `post_actions`. The former `libraries` request field is reserved. Standard JSON carries compiler input as the **string** field `input`, with bytecode, bytecode type, compiler version, optional metadata and post actions. Compiler library settings belong inside standard JSON `settings.libraries`.

Batch multipart uses `contracts`, `compiler_version`, `sources`, optional `evm_version` and optional unsigned `optimization_runs`. Batch standard JSON uses `contracts`, `compiler_version` and string `input`. Each `Contract` may contain `creation_code`, `runtime_code` and logging metadata. These requests and responses must not be decoded as solo equivalents.

## grpc-sourcifyverifier

`blockscout.smartContractVerifier.v2.SourcifyVerifier` uses the same v2 proto and [SourcifyVerifierService](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/services/sourcify_verifier.rs).

| Method | Request | Response |
|---|---|---|
| `Verify` | `VerifySourcifyRequest` | `VerifyResponse` |
| `VerifyFromEtherscan` | `VerifyFromEtherscanSourcifyRequest` | `VerifyResponse` |

`Verify` takes `address`, string `chain`, `files` and optional `chosen_contract`; import from Etherscan takes `address` and string `chain`. The chain is an upstream Sourcify identifier. Providing `"125"` does not guarantee the configured upstream supports Paxeer. These methods use that API and do not create a network or settle a transaction.

## grpc-vyperverifier

`blockscout.smartContractVerifier.v2.VyperVerifier` uses the same v2 proto and [VyperVerifierService](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/services/vyper_verifier.rs).

| Method | Request | Response |
|---|---|---|
| `VerifyMultiPart` | `VerifyVyperMultiPartRequest` | `VerifyResponse` |
| `VerifyStandardJson` | `VerifyVyperStandardJsonRequest` | `VerifyResponse` |
| `ListCompilerVersions` | `ListCompilerVersionsRequest` | `ListCompilerVersionsResponse` |

Multipart uses `bytecode`, `bytecode_type`, `compiler_version`, optional `evm_version`, `source_files`, `interfaces` and optional metadata. The old `optimizations` field is reserved; use supported compiler settings through standard JSON `input` when needed. The generated JSON deserializer defaults omitted `interfaces` to an empty map.

## grpc-zksyncsolidityverifier

The full service name is `blockscout.smartContractVerifier.v2.zksync.solidity.ZkSyncSolidityVerifier`, defined in [zksync-solidity.proto](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/zksync-solidity.proto) and implemented by [zksync_solidity_verifier::Service](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/services/zksync_solidity_verifier.rs).

| Method | Request | Response in the ZkSync package |
|---|---|---|
| `VerifyStandardJson` | `VerifyStandardJsonRequest` | `VerifyResponse` |
| `ListCompilers` | `ListCompilersRequest` | `ListCompilersResponse` |

Verification takes `code`, optional `constructor_arguments`, `zk_compiler`, `solc_compiler` and string `input`. Its `VerifyResponse` differs from the solo v2 response. This optional compiler family does not change Paxeer's chain identity.

## verifier-health

| HTTP route | Method |
|---|---|
| `GET /health` | `Health.Check` |

An omitted `service` query parameter defaults to the empty string. `GET /health` and `GET /health?service=blockscout.smartContractVerifier.v2.Health` return `{"status":"SERVING"}`. An unknown nonempty service returns HTTP `404` with JSON `{"code":5,"message":"unknown service"}`, corresponding to gRPC `NOT_FOUND`. Arbitrary compiler service names are not recognized health-query targets.

Set `VERIFIER_ORIGIN` to the configured verifier HTTP origin before these requests:

```sh
curl --fail-with-body "$VERIFIER_ORIGIN/health"
curl --fail-with-body "$VERIFIER_ORIGIN/api/v2/verifier/solidity/versions"
```

## verifier-methods

| HTTP route | Method |
|---|---|
| `POST /api/v2/verifier/solidity/methods:lookup` | `SolidityVerifier.LookupMethods` |

The request has `bytecode`, string-encoded JSON `abi`, `sourceMap` and `fileIds` (numeric file IDs to names). The result is `LookupMethodsResponse.methods`, a map whose values have `fileName`, `fileOffset` and `length`. This is source-location lookup, not a state-changing call. Solo Solidity requests also declare `postActions: ["lookup-methods"]`; consumers should inspect `postActionResponses.lookupMethods` rather than infer the requested action occurred.

## verifier-sources

All routes below use HTTP `POST` with a JSON body mapped by `body: "*"` in the API rules. A colon in a route is literal.

| HTTP path | Method | Response family |
|---|---|---|
| `/api/v2/verifier/solidity/sources:verify-multi-part` | `SolidityVerifier.VerifyMultiPart` | Solo `VerifyResponse` |
| `/api/v2/verifier/solidity/sources:verify-standard-json` | `SolidityVerifier.VerifyStandardJson` | Solo `VerifyResponse` |
| `/api/v2/verifier/solidity/sources:batch-verify-multi-part` | `SolidityVerifier.BatchVerifyMultiPart` | `BatchVerifyResponse` |
| `/api/v2/verifier/solidity/sources:batch-verify-standard-json` | `SolidityVerifier.BatchVerifyStandardJson` | `BatchVerifyResponse` |
| `/api/v2/verifier/vyper/sources:verify-multi-part` | `VyperVerifier.VerifyMultiPart` | Solo `VerifyResponse` |
| `/api/v2/verifier/vyper/sources:verify-standard-json` | `VyperVerifier.VerifyStandardJson` | Solo `VerifyResponse` |
| `/api/v2/verifier/sourcify/sources:verify` | `SourcifyVerifier.Verify` | Solo `VerifyResponse` |
| `/api/v2/verifier/sourcify/sources:verify-from-etherscan` | `SourcifyVerifier.VerifyFromEtherscan` | Solo `VerifyResponse` |
| `/api/v2/zksync-verifier/solidity/sources:verify-standard-json` | `ZkSyncSolidityVerifier.VerifyStandardJson` | ZkSync `VerifyResponse` |

### Response families

Solo Solidity, Vyper and Sourcify responses have `message`, `status` (`SUCCESS` or `FAILURE`), optional `source`, `extraData` and `postActionResponses`. A successful source records file, contract and compiler names, source files, source type, match type and settings. `compilerSettings`, `abi`, `compilationArtifacts`, `creationInputArtifacts` and `deployedBytecodeArtifacts` are JSON-encoded **strings** when present, not nested objects. `source.libraries` includes detected manual links and compiler-linked addresses; empty `compilerSettings.libraries` does not establish that no libraries were linked.

Batch responses select either `compilationFailure` with a message or `contractVerificationResults` with `items`. Each item selects `failure` or `success`; a mismatch can occur in an otherwise successfully compiled batch. Success carries compiler/language identifiers, sources and JSON-string settings/artifacts, with optional creation/runtime match details.

ZkSync responses select `compilationFailure`, `verificationFailure` or `verificationSuccess`. Success carries ZK and EVM compiler descriptors, sources, JSON-string settings/artifacts and match information. Batch and ZkSync responses have no solo-style top-level `status`.

### Transport failures and callers

HTTP success means a response was produced: compilation errors and bytecode mismatches can appear in the failure variants above. [common.rs](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/services/common.rs) maps solo request conversion `InvalidContent` to an ordinary failure response and batch conversion to `compilationFailure`; `BadRequest` becomes Tonic `INVALID_ARGUMENT`. [verification_result.rs](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-server/src/types/verification_result.rs) also maps missing compilers to `INVALID_ARGUMENT` and internal failures to `INTERNAL`.

For handler statuses, linked actix-prost maps `INVALID_ARGUMENT` (numeric code `3`) to HTTP `400`, `NOT_FOUND` (`5`) to `404`, and `INTERNAL` (`13`) to `500`, with JSON numeric `code` and string `message`. Actix extraction failures, such as malformed JSON or size rejection, occur before the handler; do not assume the same error envelope. The HTTP JSON size limit is not a gRPC message-size setting.

The [Rust HTTP client](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/src/http_client/client.rs) calls these paths and decodes family-specific responses; non-success HTTP responses produce `http_client::Error::StatusCode`. Its [module](../../../services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/src/http_client/mod.rs) also exposes `Middleware` and `Reqwest` errors. The explorer backend's [RustVerifierInterfaceBehaviour](../../../services/explorer/backend/apps/explorer/lib/explorer/smart_contract/rust_verifier_interface_behaviour.ex) appends `/api/v2` to its configured verifier base origin and distinguishes solo and ZkSync response families. Configure that origin independently from public node RPC. 

## verifier-versions

| HTTP route | Method | JSON result |
|---|---|---|
| `GET /api/v2/verifier/solidity/versions` | `SolidityVerifier.ListCompilerVersions` | `compilerVersions` array |
| `GET /api/v2/verifier/vyper/versions` | `VyperVerifier.ListCompilerVersions` | `compilerVersions` array |
| `GET /api/v2/zksync-verifier/solidity/versions` | `ZkSyncSolidityVerifier.ListCompilers` | `zkCompilers` and `solcCompilers` arrays |

Solidity and Vyper handlers sort available versions in descending version order through `versions_to_sorted_string` in `services/common.rs`. Lists reflect configured fetchers and pools, not a guarantee that any arbitrary version can be downloaded or run. Disabled families are not registered, even if their paths appear in the generated API contract.
