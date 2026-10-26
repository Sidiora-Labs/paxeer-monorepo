# Verifier http contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## grpc-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-verifier-health-check</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/health.proto:45</code> | <code>rpc Check(HealthCheckRequest) returns (HealthCheckResponse) in blockscout.smartContractVerifier.v2.Health</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-verifier-health-check</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## grpc-solidityverifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-verifier-solidityverifier-batchverifymultipart</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:12</code> | <code>rpc BatchVerifyMultiPart(BatchVerifySolidityMultiPartRequest) returns (BatchVerifyResponse) in blockscout.smartContractVerifier.v2.SolidityVerifier</code> |
| <code>interface.14.grpcs-verifier-solidityverifier-batchverifystandardjson</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:14</code> | <code>rpc BatchVerifyStandardJson(BatchVerifySolidityStandardJsonRequest) returns (BatchVerifyResponse) in blockscout.smartContractVerifier.v2.SolidityVerifier</code> |
| <code>interface.14.grpcs-verifier-solidityverifier-listcompilerversions</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:16</code> | <code>rpc ListCompilerVersions(ListCompilerVersionsRequest) returns (ListCompilerVersionsResponse) in blockscout.smartContractVerifier.v2.SolidityVerifier</code> |
| <code>interface.14.grpcs-verifier-solidityverifier-lookupmethods</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:18</code> | <code>rpc LookupMethods(LookupMethodsRequest) returns (LookupMethodsResponse) in blockscout.smartContractVerifier.v2.SolidityVerifier</code> |
| <code>interface.14.grpcs-verifier-solidityverifier-verifymultipart</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:8</code> | <code>rpc VerifyMultiPart(VerifySolidityMultiPartRequest) returns (VerifyResponse) in blockscout.smartContractVerifier.v2.SolidityVerifier</code> |
| <code>interface.14.grpcs-verifier-solidityverifier-verifystandardjson</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:10</code> | <code>rpc VerifyStandardJson(VerifySolidityStandardJsonRequest) returns (VerifyResponse) in blockscout.smartContractVerifier.v2.SolidityVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-verifier-solidityverifier-batchverifymultipart</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-solidityverifier-batchverifystandardjson</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-solidityverifier-listcompilerversions</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-solidityverifier-lookupmethods</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-solidityverifier-verifymultipart</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-solidityverifier-verifystandardjson</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## grpc-sourcifyverifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-verifier-sourcifyverifier-verify</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:30</code> | <code>rpc Verify(VerifySourcifyRequest) returns (VerifyResponse) in blockscout.smartContractVerifier.v2.SourcifyVerifier</code> |
| <code>interface.14.grpcs-verifier-sourcifyverifier-verifyfrometherscan</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:32</code> | <code>rpc VerifyFromEtherscan(VerifyFromEtherscanSourcifyRequest) returns (VerifyResponse) in blockscout.smartContractVerifier.v2.SourcifyVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-verifier-sourcifyverifier-verify</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-sourcifyverifier-verifyfrometherscan</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## grpc-vyperverifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-verifier-vyperverifier-listcompilerversions</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:26</code> | <code>rpc ListCompilerVersions(ListCompilerVersionsRequest) returns (ListCompilerVersionsResponse) in blockscout.smartContractVerifier.v2.VyperVerifier</code> |
| <code>interface.14.grpcs-verifier-vyperverifier-verifymultipart</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:22</code> | <code>rpc VerifyMultiPart(VerifyVyperMultiPartRequest) returns (VerifyResponse) in blockscout.smartContractVerifier.v2.VyperVerifier</code> |
| <code>interface.14.grpcs-verifier-vyperverifier-verifystandardjson</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto:24</code> | <code>rpc VerifyStandardJson(VerifyVyperStandardJsonRequest) returns (VerifyResponse) in blockscout.smartContractVerifier.v2.VyperVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-verifier-vyperverifier-listcompilerversions</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-vyperverifier-verifymultipart</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-vyperverifier-verifystandardjson</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## grpc-zksyncsolidityverifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.grpcs-verifier-zksyncsolidityverifier-listcompilers</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/zksync-solidity.proto:10</code> | <code>rpc ListCompilers(ListCompilersRequest) returns (ListCompilersResponse) in blockscout.smartContractVerifier.v2.zksync.solidity.ZkSyncSolidityVerifier</code> |
| <code>interface.14.grpcs-verifier-zksyncsolidityverifier-verifystandardjson</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/zksync-solidity.proto:8</code> | <code>rpc VerifyStandardJson(VerifyStandardJsonRequest) returns (VerifyResponse) in blockscout.smartContractVerifier.v2.zksync.solidity.ZkSyncSolidityVerifier</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.grpcs-verifier-zksyncsolidityverifier-listcompilers</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |
| <code>interface.14.grpcs-verifier-zksyncsolidityverifier-verifystandardjson</code> | notes | served over gRPC when &lt;SERVICE&gt;__SERVER__GRPC__ENABLED is true (services/explorer/services/libs/blockscout-service-launcher/src/launcher/settings.rs:150); no in-repo gRPC client found |

## verifier-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-verifier-get-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:67</code> | <code>GET /health -&gt; blockscout.smartContractVerifier.v2.Health.Check</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-verifier-get-health</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule; health probe; also exposed as gRPC Health.Check; no in-repo caller found |

## verifier-methods

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-methods-lookup</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:28</code> | <code>POST /api/v2/verifier/solidity/methods:lookup -&gt; blockscout.smartContractVerifier.v2.SolidityVerifier.LookupMethods</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-methods-lookup</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |

## verifier-sources

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-batch-verify-multi-part</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:17</code> | <code>POST /api/v2/verifier/solidity/sources:batch-verify-multi-part -&gt; blockscout.smartContractVerifier.v2.SolidityVerifier.BatchVerifyMultiPart</code> |
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-batch-verify-standard-json</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:21</code> | <code>POST /api/v2/verifier/solidity/sources:batch-verify-standard-json -&gt; blockscout.smartContractVerifier.v2.SolidityVerifier.BatchVerifyStandardJson</code> |
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-verify-multi-part</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:9</code> | <code>POST /api/v2/verifier/solidity/sources:verify-multi-part -&gt; blockscout.smartContractVerifier.v2.SolidityVerifier.VerifyMultiPart</code> |
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-verify-standard-json</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:13</code> | <code>POST /api/v2/verifier/solidity/sources:verify-standard-json -&gt; blockscout.smartContractVerifier.v2.SolidityVerifier.VerifyStandardJson</code> |
| <code>interface.14.http-verifier-post-api-v2-verifier-sourcify-sources-verify</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:48</code> | <code>POST /api/v2/verifier/sourcify/sources:verify -&gt; blockscout.smartContractVerifier.v2.SourcifyVerifier.Verify</code> |
| <code>interface.14.http-verifier-post-api-v2-verifier-sourcify-sources-verify-from-etherscan</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:52</code> | <code>POST /api/v2/verifier/sourcify/sources:verify-from-etherscan -&gt; blockscout.smartContractVerifier.v2.SourcifyVerifier.VerifyFromEtherscan</code> |
| <code>interface.14.http-verifier-post-api-v2-verifier-vyper-sources-verify-multi-part</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:35</code> | <code>POST /api/v2/verifier/vyper/sources:verify-multi-part -&gt; blockscout.smartContractVerifier.v2.VyperVerifier.VerifyMultiPart</code> |
| <code>interface.14.http-verifier-post-api-v2-verifier-vyper-sources-verify-standard-json</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:39</code> | <code>POST /api/v2/verifier/vyper/sources:verify-standard-json -&gt; blockscout.smartContractVerifier.v2.VyperVerifier.VerifyStandardJson</code> |
| <code>interface.14.http-verifier-post-api-v2-zksync-verifier-solidity-sources-verify-standard-json</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:58</code> | <code>POST /api/v2/zksync-verifier/solidity/sources:verify-standard-json -&gt; blockscout.smartContractVerifier.v2.zksync.solidity.ZkSyncSolidityVerifier.VerifyStandardJson</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-batch-verify-multi-part</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-batch-verify-standard-json</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-verify-multi-part</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-verifier-solidity-sources-verify-standard-json</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-verifier-sourcify-sources-verify</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-verifier-sourcify-sources-verify-from-etherscan</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-verifier-vyper-sources-verify-multi-part</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-verifier-vyper-sources-verify-standard-json</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-post-api-v2-zksync-verifier-solidity-sources-verify-standard-json</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |

## verifier-versions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-verifier-get-api-v2-verifier-solidity-versions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:25</code> | <code>GET /api/v2/verifier/solidity/versions -&gt; blockscout.smartContractVerifier.v2.SolidityVerifier.ListCompilerVersions</code> |
| <code>interface.14.http-verifier-get-api-v2-verifier-vyper-versions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:43</code> | <code>GET /api/v2/verifier/vyper/versions -&gt; blockscout.smartContractVerifier.v2.VyperVerifier.ListCompilerVersions</code> |
| <code>interface.14.http-verifier-get-api-v2-zksync-verifier-solidity-versions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/services/smart-contract-verifier/smart-contract-verifier-proto/proto/v2/api_config_http.yaml:62</code> | <code>GET /api/v2/zksync-verifier/solidity/versions -&gt; blockscout.smartContractVerifier.v2.zksync.solidity.ZkSyncSolidityVerifier.ListCompilers</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-verifier-get-api-v2-verifier-solidity-versions</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-get-api-v2-verifier-vyper-versions</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
| <code>interface.14.http-verifier-get-api-v2-zksync-verifier-solidity-versions</code> | notes | served by smart-contract-verifier-server through the launcher HTTP router generated from this rule |
