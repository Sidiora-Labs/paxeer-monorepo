# Sig provider http contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## grpc-abiservice

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-sig-provider-abiservice-batchgeteventabis</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/sig-provider.proto:16</code> | <code>rpc BatchGetEventAbis(BatchGetEventAbisRequest) returns (BatchGetEventAbisResponse) in blockscout.sig_provider.v1.AbiService</code> |
| <code>interface.14.grpcs-sig-provider-abiservice-geteventabi</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/sig-provider.proto:14</code> | <code>rpc GetEventAbi(GetEventAbiRequest) returns (GetEventAbiResponse) in blockscout.sig_provider.v1.AbiService</code> |
| <code>interface.14.grpcs-sig-provider-abiservice-getfunctionabi</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/sig-provider.proto:13</code> | <code>rpc GetFunctionAbi(GetFunctionAbiRequest) returns (GetFunctionAbiResponse) in blockscout.sig_provider.v1.AbiService</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-sig-provider-abiservice-batchgeteventabis</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-sig-provider-abiservice-geteventabi</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-sig-provider-abiservice-getfunctionabi</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## grpc-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-sig-provider-health-check</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/health.proto:46</code> | <code>rpc Check(HealthCheckRequest) returns (HealthCheckResponse) in blockscout.sig_provider.v1.Health</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-sig-provider-health-check</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## grpc-signatureservice

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-sig-provider-signatureservice-createsignatures</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/sig-provider.proto:8</code> | <code>rpc CreateSignatures(CreateSignaturesRequest) returns (CreateSignaturesResponse) in blockscout.sig_provider.v1.SignatureService</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-sig-provider-signatureservice-createsignatures</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## sig-provider-event

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-sig-provider-get-api-v1-abi-event</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/api_config_http.yaml:13</code> | <code>GET /api/v1/abi/event -&gt; blockscout.sig_provider.v1.AbiService.GetEventAbi</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-sig-provider-get-api-v1-abi-event</code> | notes | served by sig-provider-server through the launcher HTTP router generated from this rule |

## sig-provider-events

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-sig-provider-post-api-v1-abi-events-batch-get</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/api_config_http.yaml:17</code> | <code>POST /api/v1/abi/events:batch-get -&gt; blockscout.sig_provider.v1.AbiService.BatchGetEventAbis</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-sig-provider-post-api-v1-abi-events-batch-get</code> | notes | served by sig-provider-server through the launcher HTTP router generated from this rule |

## sig-provider-function

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-sig-provider-get-api-v1-abi-function</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/api_config_http.yaml:10</code> | <code>GET /api/v1/abi/function -&gt; blockscout.sig_provider.v1.AbiService.GetFunctionAbi</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-sig-provider-get-api-v1-abi-function</code> | notes | served by sig-provider-server through the launcher HTTP router generated from this rule |

## sig-provider-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-sig-provider-get-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/api_config_http.yaml:22</code> | <code>GET /health -&gt; blockscout.sig_provider.v1.Health.Check</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-sig-provider-get-health</code> | notes | served by sig-provider-server through the launcher HTTP router generated from this rule; health probe; also exposed as gRPC Health.Check; no in-repo caller found |

## sig-provider-signatures

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-sig-provider-post-api-v1-signatures</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/sig-provider/sig-provider-proto/proto/api_config_http.yaml:7</code> | <code>POST /api/v1/signatures -&gt; blockscout.sig_provider.v1.SignatureService.CreateSignatures</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-sig-provider-post-api-v1-signatures</code> | notes | served by sig-provider-server through the launcher HTTP router generated from this rule |
