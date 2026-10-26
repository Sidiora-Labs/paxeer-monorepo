# Proto api contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## buf-workspace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.fmt-api-buf-lock-j39</code> | <code>file_format</code> | <code>retire</code> | <code>api/buf.lock:1</code> | <code>buf v1 lock pinning two remote deps (cosmos-sdk and third-party) by commit and digest for api/buf.yaml</code> |
| <code>interface.7.fmt-buf-baseline-binpb-j39</code> | <code>file_format</code> | <code>new</code> | <code>buf.baseline.binpb</code> (planned) | <code>buf image (binary FileDescriptorSet, no source info) of the root workspace used as the against input of buf breaking</code> |
| <code>interface.7.fmt-buf-lock-j39</code> | <code>file_format</code> | <code>stable</code> | <code>buf.lock:2</code> | <code>buf v2 lock with no deps</code> |
| <code>interface.7.fmt-buf-yaml-j39</code> | <code>file_format</code> | <code>change</code> | <code>buf.yaml:2</code> | <code>buf v2 workspace: modules api, sdk/proto, sdk/third_party/proto, interchain/proto, interchain/third_party/proto, consensus/proto, consensus/internal, wasm/proto; lint use BASIC, FILE_LOWER_SNAKE_CASE, UNARY_RPC except COMMENT_FIELD, PACKAGE_VERSION_SUFFIX, PACKAGE_SAME_DIRECTORY, PACKAGE_DIRECTORY_MATCH, PACKAGE_SAME_GO_PACKAGE; lint ignore wasm/third_party/proto, sdk/third_party/proto; breaking use WIRE, WIRE_JSON</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.fmt-api-buf-lock-j39</code> | notes | no in-repo script runs buf inside api/; the root workspace at buf.yaml:3-12 resolves the same imports locally |
| <code>interface.7.fmt-buf-baseline-binpb-j39</code> | notes | planned by task 7.22.2; read by api/buf_breaking_test.go |
| <code>interface.7.fmt-buf-lock-j39</code> | notes | the root workspace resolves every import locally from its modules |
| <code>interface.7.fmt-buf-yaml-j39</code> | notes | lint.ignore entry wasm/third_party/proto at buf.yaml:26 names a directory that does not exist; the api module also declares the registry module name at buf.yaml:5 |

Contract context <code>contract_context.7.proto-workspace</code>:

| Field | Frozen rule |
|---|---|
| interface | Root buf v2 workspace: modules, lint, breaking baseline and generation template for api/ and sdk/proto/. |
| storage | buf.yaml, buf.gen.yaml, buf.lock and buf.baseline.binpb at the repository root; no chain state. |
| errors | buf lint, build and breaking exit non-zero with the failing file and rule; tests fail on any finding. |
| events | None. |
| gas | None; tooling only. |
| activation | Effective from the revision that carries the configuration; no upgrade height. |
| invariants | Every proto under api/ and sdk/proto/ is built exactly once by the root workspace; WIRE and WIRE_JSON hold against the baseline. |
| pause | Not pausable. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.241.1</code> | req | 7241.1 |
| <code>acceptance.7.241.1</code> | scenario | Given the root buf workspace declared at ./buf.yaml:3-12 and the source trees api/ and sdk/proto/, when TestBufBuildWorkspaceImage builds the image with buf v1.58.0 and walks both trees, then the build exits 0 and every .proto name occurs exactly once in the image |
| <code>acceptance.7.241.1</code> | test | api/buf_build_test.go::TestBufBuildWorkspaceImage (planned) |
| <code>acceptance.7.241.1</code> | qualification | UNRUN |
| <code>req.7241</code> | ac_1 | Building the root buf workspace (./buf.yaml:3-12) with the buf CLI pinned at scripts/protoc.sh:26 yields one image in which every .proto under api/ and sdk/proto/ appears exactly once under its module-relative name (planned). |
| <code>acceptance.7.244.1</code> | req | 7244.1 |
| <code>acceptance.7.244.1</code> | scenario | Given ./buf.yaml without the dead wasm/third_party/proto ignore entry, when TestBufLintWorkspace runs buf v1.58.0 lint at the repository root, then it exits 0 with empty output and each remaining ignore entry is an existing directory |
| <code>acceptance.7.244.1</code> | test | api/buf_lint_test.go::TestBufLintWorkspace (planned) |
| <code>acceptance.7.244.1</code> | qualification | UNRUN |
| <code>req.7244</code> | ac_1 | Running buf lint at the repository root exits 0 with no findings, and every lint.ignore entry in ./buf.yaml:25-27 names an existing directory (planned). |
| <code>acceptance.7.245.1</code> | req | 7245.1 |
| <code>acceptance.7.245.1</code> | scenario | Given the committed baseline buf.baseline.binpb and the workspace sources, when TestBufBreakingAgainstBaseline runs buf v1.58.0 breaking against it, then it exits 0 with empty output and the baseline contains the three named files |
| <code>acceptance.7.245.1</code> | test | api/buf_breaking_test.go::TestBufBreakingAgainstBaseline (planned) |
| <code>acceptance.7.245.1</code> | qualification | UNRUN |
| <code>req.7245</code> | ac_1 | buf breaking with the WIRE and WIRE_JSON rules of ./buf.yaml:29-32 against the committed baseline image buf.baseline.binpb exits 0, and the baseline holds evm/params.proto, oracle/oracle.proto and cosmos/bank/v1beta1/query.proto (planned). |
| <code>acceptance.7.246.1</code> | req | 7246.1 |
| <code>acceptance.7.246.1</code> | scenario | Given api/ without its own buf.yaml, buf.gen.yaml and buf.lock, when TestBufBuildFromApiDirMatchesWorkspaceImage runs buf v1.58.0 build with working directory api/, then the image equals the root workspace image file by file, imports from sdk/proto included |
| <code>acceptance.7.246.1</code> | test | api/buf_apidir_test.go::TestBufBuildFromApiDirMatchesWorkspaceImage (planned) |
| <code>acceptance.7.246.1</code> | qualification | UNRUN |
| <code>req.7246</code> | ac_1 | With api/buf.yaml, api/buf.gen.yaml and api/buf.lock removed, buf build run inside api/ resolves the root workspace of ./buf.yaml:1-12 and yields the same file descriptors as the build at the repository root (planned). |
| <code>resolution.17.783</code> | title | decision.721 binds: The api/ test package belongs to rpc-api |
| <code>resolution.17.783</code> | applies_to | decision.721<br>task.7.21.1<br>task.7.29.1<br>task.7.29.2 |
| <code>resolution.17.783</code> | source | decision.721 in fragments/J39/decision.kvx; tasks from affects |
| <code>resolution.17.785</code> | title | decision.723 binds: Frozen camelCase proto fields keep their names |
| <code>resolution.17.785</code> | applies_to | decision.723<br>task.7.22.1 |
| <code>resolution.17.785</code> | source | decision.723 in fragments/J39/decision.kvx; tasks from affects |
| <code>resolution.17.786</code> | title | decision.724 binds: Breaking changes are checked with WIRE and WIRE_JSON against a committed image |
| <code>resolution.17.786</code> | applies_to | decision.724<br>task.7.22.2 |
| <code>resolution.17.786</code> | source | decision.724 in fragments/J39/decision.kvx; tasks from affects |

## codegen-reproducibility

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.fmt-buf-gen-yaml-j39</code> | <code>file_format</code> | <code>stable</code> | <code>buf.gen.yaml:1</code> | <code>buf v2 generate template: inputs api, sdk/proto, interchain/proto, consensus/proto, wasm/proto; plugin protoc-gen-gocosmos (local build) opt plugins=interfacetype+grpc with five M mappings out ./build/proto/gocosmos; protoc-gen-grpc-gateway v1.16.0 opt logtostderr=true, allow_colon_final_segments=true; protoc-gen-gogofaster v1.3.2 out ./build/proto/gogofaster</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.fmt-buf-gen-yaml-j39</code> | notes | grpc-gateway pin at buf.gen.yaml:23 equals go.mod:48; protoc-gen-gocosmos is built from the version at go.mod:144 by scripts/protoc.sh:19-24 |

Contract context <code>contract_context.7.proto-workspace</code>:

| Field | Frozen rule |
|---|---|
| interface | Root buf v2 workspace: modules, lint, breaking baseline and generation template for api/ and sdk/proto/. |
| storage | buf.yaml, buf.gen.yaml, buf.lock and buf.baseline.binpb at the repository root; no chain state. |
| errors | buf lint, build and breaking exit non-zero with the failing file and rule; tests fail on any finding. |
| events | None. |
| gas | None; tooling only. |
| activation | Effective from the revision that carries the configuration; no upgrade height. |
| invariants | Every proto under api/ and sdk/proto/ is built exactly once by the root workspace; WIRE and WIRE_JSON hold against the baseline. |
| pause | Not pausable. |

| Decision | Title | Value | Basis | Affects |
|---|---|---|---|---|
| <code>decision.732</code> | Codegen toolchain pins | buf v1.58.0, protoc-gen-gocosmos from the cosmos-proto module at go.mod:144, grpc-gateway v1.16.0 and gogofaster v1.3.2 are the generation toolchain; tests run the same pins. | scripts/protoc.sh:19-26, buf.gen.yaml:23, buf.gen.yaml:39 and go.mod:48 | <code>7.29.2</code><br><code>interface.7.fmt-buf-gen-yaml-j39</code> |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.288.1</code> | req | 7288.1 |
| <code>acceptance.7.288.1</code> | scenario | Given the pinned protoc-gen-gocosmos, grpc-gateway and buf versions, when TestBufGenerateMatchesCheckedInGo generates api and sdk/proto into a temporary directory and maps outputs as scripts/protoc.sh copies them, then every output equals its checked-in file and every checked-in generated file of those inputs is produced |
| <code>acceptance.7.288.1</code> | test | api/buf_generate_test.go::TestBufGenerateMatchesCheckedInGo (planned) |
| <code>acceptance.7.288.1</code> | qualification | UNRUN |
| <code>req.7288</code> | ac_1 | buf generate with the plugins and options of ./buf.gen.yaml:8-27 over api and sdk/proto, using protoc-gen-gocosmos from go.mod:144 and grpc-gateway v1.16.0 (./buf.gen.yaml:23), reproduces every checked-in .pb.go and .pb.gw.go of those inputs byte for byte and leaves none unproduced. |
| <code>resolution.17.794</code> | title | decision.732 binds: Codegen toolchain pins |
| <code>resolution.17.794</code> | applies_to | decision.732<br>task.7.29.2 |
| <code>resolution.17.794</code> | source | decision.732 in fragments/J39/decision.kvx; tasks from affects |

## descriptor-parity

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.242.1</code> | req | 7242.1 |
| <code>acceptance.7.242.1</code> | scenario | Given the buf image of api/eth and the generated descriptors read through one representative type per file, when TestEthTxDescriptorParity compares them file by file, then no difference is reported and every .proto in the directory has a table entry |
| <code>acceptance.7.242.1</code> | test | api/eth_tx_parity_test.go::TestEthTxDescriptorParity (planned) |
| <code>acceptance.7.242.1</code> | qualification | UNRUN |
| <code>req.7242</code> | ac_1 | Every .proto under api/eth (1 at HEAD, for example api/eth/tx.proto:1) matches the descriptor of its generated package read through a representative type, in package, imports, messages, fields, enums, rpcs and HTTP rules (planned). |
| <code>acceptance.7.243.1</code> | req | 7243.1 |
| <code>acceptance.7.243.1</code> | scenario | Given the buf image of api/pax/admin/v0 and the descriptors its generated Go registers, when TestPaxAdminDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.243.1</code> | test | api/pax_admin_parity_test.go::TestPaxAdminDescriptorParity (planned) |
| <code>acceptance.7.243.1</code> | qualification | UNRUN |
| <code>req.7243</code> | ac_1 | Every .proto under api/pax/admin/v0 (1 at HEAD, for example api/pax/admin/v0/admin.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.247.1</code> | req | 7247.1 |
| <code>acceptance.7.247.1</code> | scenario | Given the buf image of api/epoch and the descriptors its generated Go registers, when TestEpochDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.247.1</code> | test | api/epoch_parity_test.go::TestEpochDescriptorParity (planned) |
| <code>acceptance.7.247.1</code> | qualification | UNRUN |
| <code>req.7247</code> | ac_1 | Every .proto under api/epoch (5 at HEAD, for example api/epoch/epoch.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.248.1</code> | req | 7248.1 |
| <code>acceptance.7.248.1</code> | scenario | Given the buf image of api/mint/v1beta1 and the descriptors its generated Go registers, when TestMintDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.248.1</code> | test | api/mint_parity_test.go::TestMintDescriptorParity (planned) |
| <code>acceptance.7.248.1</code> | qualification | UNRUN |
| <code>req.7248</code> | ac_1 | Every .proto under api/mint/v1beta1 (4 at HEAD, for example api/mint/v1beta1/genesis.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.249.1</code> | req | 7249.1 |
| <code>acceptance.7.249.1</code> | scenario | Given the buf image of api/oracle and the descriptors its generated Go registers, when TestOracleDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.249.1</code> | test | api/oracle_parity_test.go::TestOracleDescriptorParity (planned) |
| <code>acceptance.7.249.1</code> | qualification | UNRUN |
| <code>req.7249</code> | ac_1 | Every .proto under api/oracle (4 at HEAD, for example api/oracle/genesis.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.250.1</code> | req | 7250.1 |
| <code>acceptance.7.250.1</code> | scenario | Given the buf image of api/pricefeed and the descriptors its generated Go registers, when TestPricefeedDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.250.1</code> | test | api/pricefeed_parity_test.go::TestPricefeedDescriptorParity (planned) |
| <code>acceptance.7.250.1</code> | qualification | UNRUN |
| <code>req.7250</code> | ac_1 | Every .proto under api/pricefeed (1 at HEAD, for example api/pricefeed/tx.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.251.1</code> | req | 7251.1 |
| <code>acceptance.7.251.1</code> | scenario | Given the buf image of api/spot and the descriptors its generated Go registers, when TestSpotDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.251.1</code> | test | api/spot_parity_test.go::TestSpotDescriptorParity (planned) |
| <code>acceptance.7.251.1</code> | qualification | UNRUN |
| <code>req.7251</code> | ac_1 | Every .proto under api/spot (1 at HEAD, for example api/spot/tx.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.252.1</code> | req | 7252.1 |
| <code>acceptance.7.252.1</code> | scenario | Given the buf image of api/layerxexchange and the descriptors its generated Go registers, when TestLayerxExchangeDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.252.1</code> | test | api/layerx_exchange_parity_test.go::TestLayerxExchangeDescriptorParity (planned) |
| <code>acceptance.7.252.1</code> | qualification | UNRUN |
| <code>req.7252</code> | ac_1 | Every .proto under api/layerxexchange (5 at HEAD, for example api/layerxexchange/events.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.253.1</code> | req | 7253.1 |
| <code>acceptance.7.253.1</code> | scenario | Given the buf image of api/perps and the descriptors its generated Go registers, when TestPerpsDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.253.1</code> | test | api/perps_parity_test.go::TestPerpsDescriptorParity (planned) |
| <code>acceptance.7.253.1</code> | qualification | UNRUN |
| <code>req.7253</code> | ac_1 | Every .proto under api/perps (planned) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source. |
| <code>acceptance.7.254.1</code> | req | 7254.1 |
| <code>acceptance.7.254.1</code> | scenario | Given the buf image of api/launchpad and the descriptors its generated Go registers, when TestLaunchpadDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.254.1</code> | test | api/launchpad_parity_test.go::TestLaunchpadDescriptorParity (planned) |
| <code>acceptance.7.254.1</code> | qualification | UNRUN |
| <code>req.7254</code> | ac_1 | Every .proto under api/launchpad (1 at HEAD, for example api/launchpad/tx.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.255.1</code> | req | 7255.1 |
| <code>acceptance.7.255.1</code> | scenario | Given the buf image of api/layerxagent and the descriptors its generated Go registers, when TestLayerxAgentDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.255.1</code> | test | api/layerx_agent_parity_test.go::TestLayerxAgentDescriptorParity (planned) |
| <code>acceptance.7.255.1</code> | qualification | UNRUN |
| <code>req.7255</code> | ac_1 | Every .proto under api/layerxagent (1 at HEAD, for example api/layerxagent/events.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.256.1</code> | req | 7256.1 |
| <code>acceptance.7.256.1</code> | scenario | Given the buf image of api/layerxauth and the descriptors its generated Go registers, when TestLayerxAuthDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.256.1</code> | test | api/layerx_auth_parity_test.go::TestLayerxAuthDescriptorParity (planned) |
| <code>acceptance.7.256.1</code> | qualification | UNRUN |
| <code>req.7256</code> | ac_1 | Every .proto under api/layerxauth (1 at HEAD, for example api/layerxauth/tx.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.257.1</code> | req | 7257.1 |
| <code>acceptance.7.257.1</code> | scenario | Given the buf image of api/layerxledger and the descriptors its generated Go registers, when TestLayerxLedgerDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.257.1</code> | test | api/layerx_ledger_parity_test.go::TestLayerxLedgerDescriptorParity (planned) |
| <code>acceptance.7.257.1</code> | qualification | UNRUN |
| <code>req.7257</code> | ac_1 | Every .proto under api/layerxledger (1 at HEAD, for example api/layerxledger/tx.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.258.1</code> | req | 7258.1 |
| <code>acceptance.7.258.1</code> | scenario | Given the buf image of api/evm and the generated descriptors read through one representative type per file, when TestEvmDescriptorParity compares them file by file, then no difference is reported and every .proto in the directory has a table entry |
| <code>acceptance.7.258.1</code> | test | api/evm_parity_test.go::TestEvmDescriptorParity (planned) |
| <code>acceptance.7.258.1</code> | qualification | UNRUN |
| <code>req.7258</code> | ac_1 | Every .proto under api/evm (9 at HEAD, for example api/evm/config.proto:1) matches the descriptor of its generated package read through a representative type, in package, imports, messages, fields, enums, rpcs and HTTP rules (planned). |
| <code>acceptance.7.259.1</code> | req | 7259.1 |
| <code>acceptance.7.259.1</code> | scenario | Given the buf image of api/tokenfactory and the descriptors its generated Go registers, when TestTokenfactoryDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.259.1</code> | test | api/tokenfactory_parity_test.go::TestTokenfactoryDescriptorParity (planned) |
| <code>acceptance.7.259.1</code> | qualification | UNRUN |
| <code>req.7259</code> | ac_1 | Every .proto under api/tokenfactory (5 at HEAD, for example api/tokenfactory/authority_metadata.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.260.1</code> | req | 7260.1 |
| <code>acceptance.7.260.1</code> | scenario | Given the buf image of api/layerxgov and the descriptors its generated Go registers, when TestLayerxGovDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.260.1</code> | test | api/layerx_gov_parity_test.go::TestLayerxGovDescriptorParity (planned) |
| <code>acceptance.7.260.1</code> | qualification | UNRUN |
| <code>req.7260</code> | ac_1 | Every .proto under api/layerxgov (1 at HEAD, for example api/layerxgov/proposal.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.261.1</code> | req | 7261.1 |
| <code>acceptance.7.261.1</code> | scenario | Given the buf image of api/layerxanchor and the descriptors its generated Go registers, when TestLayerxAnchorDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.261.1</code> | test | api/layerx_anchor_parity_test.go::TestLayerxAnchorDescriptorParity (planned) |
| <code>acceptance.7.261.1</code> | qualification | UNRUN |
| <code>req.7261</code> | ac_1 | Every .proto under api/layerxanchor (1 at HEAD, for example api/layerxanchor/tx.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.262.1</code> | req | 7262.1 |
| <code>acceptance.7.262.1</code> | scenario | Given the buf image of api/layerxbridge and the descriptors its generated Go registers, when TestLayerxBridgeDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.262.1</code> | test | api/layerx_bridge_parity_test.go::TestLayerxBridgeDescriptorParity (planned) |
| <code>acceptance.7.262.1</code> | qualification | UNRUN |
| <code>req.7262</code> | ac_1 | Every .proto under api/layerxbridge (2 at HEAD, for example api/layerxbridge/proposal.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.263.1</code> | req | 7263.1 |
| <code>acceptance.7.263.1</code> | scenario | Given the buf image of api/layerxcustody and the descriptors its generated Go registers, when TestLayerxCustodyDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.263.1</code> | test | api/layerx_custody_parity_test.go::TestLayerxCustodyDescriptorParity (planned) |
| <code>acceptance.7.263.1</code> | qualification | UNRUN |
| <code>req.7263</code> | ac_1 | Every .proto under api/layerxcustody (6 at HEAD, for example api/layerxcustody/events.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.264.1</code> | req | 7264.1 |
| <code>acceptance.7.264.1</code> | scenario | Given the buf image of api/xweb and the descriptors its generated Go registers, when TestXwebDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.264.1</code> | test | api/xweb_parity_test.go::TestXwebDescriptorParity (planned) |
| <code>acceptance.7.264.1</code> | qualification | UNRUN |
| <code>req.7264</code> | ac_1 | Every .proto under api/xweb (1 at HEAD, for example api/xweb/tx.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.265.1</code> | req | 7265.1 |
| <code>acceptance.7.265.1</code> | scenario | Given the buf image of sdk/proto/cosmos/auth/v1beta1, sdk/proto/cosmos/vesting/v1beta1 and the descriptors its generated Go registers, when TestCosmosAuthVestingDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.265.1</code> | test | api/cosmos_auth_vesting_parity_test.go::TestCosmosAuthVestingDescriptorParity (planned) |
| <code>acceptance.7.265.1</code> | qualification | UNRUN |
| <code>req.7265</code> | ac_1 | Every .proto under sdk/proto/cosmos/auth/v1beta1, sdk/proto/cosmos/vesting/v1beta1 (5 at HEAD, for example sdk/proto/cosmos/auth/v1beta1/auth.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.266.1</code> | req | 7266.1 |
| <code>acceptance.7.266.1</code> | scenario | Given the buf image of sdk/proto/cosmos/authz/v1beta1 and the descriptors its generated Go registers, when TestCosmosAuthzDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.266.1</code> | test | api/cosmos_authz_parity_test.go::TestCosmosAuthzDescriptorParity (planned) |
| <code>acceptance.7.266.1</code> | qualification | UNRUN |
| <code>req.7266</code> | ac_1 | Every .proto under sdk/proto/cosmos/authz/v1beta1 (5 at HEAD, for example sdk/proto/cosmos/authz/v1beta1/authz.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.267.1</code> | req | 7267.1 |
| <code>acceptance.7.267.1</code> | scenario | Given the buf image of sdk/proto/cosmos/feegrant/v1beta1 and the descriptors its generated Go registers, when TestCosmosFeegrantDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.267.1</code> | test | api/cosmos_feegrant_parity_test.go::TestCosmosFeegrantDescriptorParity (planned) |
| <code>acceptance.7.267.1</code> | qualification | UNRUN |
| <code>req.7267</code> | ac_1 | Every .proto under sdk/proto/cosmos/feegrant/v1beta1 (4 at HEAD, for example sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.268.1</code> | req | 7268.1 |
| <code>acceptance.7.268.1</code> | scenario | Given the buf image of sdk/proto/cosmos/base/abci/v1beta1, sdk/proto/cosmos/base/kv/v1beta1, sdk/proto/cosmos/base/query/v1beta1, sdk/proto/cosmos/base/reflection/v1beta1, sdk/proto/cosmos/base/reflection/v2alpha1, sdk/proto/cosmos/base/snapshots/v1beta1, sdk/proto/cosmos/base/store/v1beta1, sdk/proto/cosmos/base/tendermint/v1beta1, sdk/proto/cosmos/base/v1beta1 and the descriptors its generated Go registers, when TestCosmosBaseDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.268.1</code> | test | api/cosmos_base_parity_test.go::TestCosmosBaseDescriptorParity (planned) |
| <code>acceptance.7.268.1</code> | qualification | UNRUN |
| <code>req.7268</code> | ac_1 | Every .proto under sdk/proto/cosmos/base/abci/v1beta1, sdk/proto/cosmos/base/kv/v1beta1, sdk/proto/cosmos/base/query/v1beta1, sdk/proto/cosmos/base/reflection/v1beta1, sdk/proto/cosmos/base/reflection/v2alpha1, sdk/proto/cosmos/base/snapshots/v1beta1, sdk/proto/cosmos/base/store/v1beta1, sdk/proto/cosmos/base/tendermint/v1beta1, sdk/proto/cosmos/base/v1beta1 (10 at HEAD, for example sdk/proto/cosmos/base/abci/v1beta1/abci.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.269.1</code> | req | 7269.1 |
| <code>acceptance.7.269.1</code> | scenario | Given the buf image of sdk/proto/cosmos/crypto/ed25519, sdk/proto/cosmos/crypto/multisig, sdk/proto/cosmos/crypto/multisig/v1beta1, sdk/proto/cosmos/crypto/secp256k1, sdk/proto/cosmos/crypto/secp256r1, sdk/proto/cosmos/crypto/sr25519, sdk/proto/cosmos/capability/v1beta1, sdk/proto/cosmos/genutil/v1beta1 and the descriptors its generated Go registers, when TestCosmosCryptoCapabilityGenutilDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.269.1</code> | test | api/cosmos_crypto_capability_genutil_parity_test.go::TestCosmosCryptoCapabilityGenutilDescriptorParity (planned) |
| <code>acceptance.7.269.1</code> | qualification | UNRUN |
| <code>req.7269</code> | ac_1 | Every .proto under sdk/proto/cosmos/crypto/ed25519, sdk/proto/cosmos/crypto/multisig, sdk/proto/cosmos/crypto/multisig/v1beta1, sdk/proto/cosmos/crypto/secp256k1, sdk/proto/cosmos/crypto/secp256r1, sdk/proto/cosmos/crypto/sr25519, sdk/proto/cosmos/capability/v1beta1, sdk/proto/cosmos/genutil/v1beta1 (9 at HEAD, for example sdk/proto/cosmos/crypto/ed25519/keys.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.270.1</code> | req | 7270.1 |
| <code>acceptance.7.270.1</code> | scenario | Given the buf image of sdk/proto/cosmos/distribution/v1beta1 and the descriptors its generated Go registers, when TestCosmosDistributionDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.270.1</code> | test | api/cosmos_distribution_parity_test.go::TestCosmosDistributionDescriptorParity (planned) |
| <code>acceptance.7.270.1</code> | qualification | UNRUN |
| <code>req.7270</code> | ac_1 | Every .proto under sdk/proto/cosmos/distribution/v1beta1 (4 at HEAD, for example sdk/proto/cosmos/distribution/v1beta1/distribution.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.271.1</code> | req | 7271.1 |
| <code>acceptance.7.271.1</code> | scenario | Given the buf image of sdk/proto/cosmos/evidence/v1beta1 and the descriptors its generated Go registers, when TestCosmosEvidenceDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.271.1</code> | test | api/cosmos_evidence_parity_test.go::TestCosmosEvidenceDescriptorParity (planned) |
| <code>acceptance.7.271.1</code> | qualification | UNRUN |
| <code>req.7271</code> | ac_1 | Every .proto under sdk/proto/cosmos/evidence/v1beta1 (4 at HEAD, for example sdk/proto/cosmos/evidence/v1beta1/evidence.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.272.1</code> | req | 7272.1 |
| <code>acceptance.7.272.1</code> | scenario | Given the buf image of sdk/proto/cosmos/gov/v1beta1 and the descriptors its generated Go registers, when TestCosmosGovDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.272.1</code> | test | api/cosmos_gov_parity_test.go::TestCosmosGovDescriptorParity (planned) |
| <code>acceptance.7.272.1</code> | qualification | UNRUN |
| <code>req.7272</code> | ac_1 | Every .proto under sdk/proto/cosmos/gov/v1beta1 (4 at HEAD, for example sdk/proto/cosmos/gov/v1beta1/genesis.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.273.1</code> | req | 7273.1 |
| <code>acceptance.7.273.1</code> | scenario | Given the buf image of sdk/proto/cosmos/params/types, sdk/proto/cosmos/params/v1beta1 and the descriptors its generated Go registers, when TestCosmosParamsDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.273.1</code> | test | api/cosmos_params_parity_test.go::TestCosmosParamsDescriptorParity (planned) |
| <code>acceptance.7.273.1</code> | qualification | UNRUN |
| <code>req.7273</code> | ac_1 | Every .proto under sdk/proto/cosmos/params/types, sdk/proto/cosmos/params/v1beta1 (3 at HEAD, for example sdk/proto/cosmos/params/types/types.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.274.1</code> | req | 7274.1 |
| <code>acceptance.7.274.1</code> | scenario | Given the buf image of sdk/proto/cosmos/slashing/v1beta1 and the descriptors its generated Go registers, when TestCosmosSlashingDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.274.1</code> | test | api/cosmos_slashing_parity_test.go::TestCosmosSlashingDescriptorParity (planned) |
| <code>acceptance.7.274.1</code> | qualification | UNRUN |
| <code>req.7274</code> | ac_1 | Every .proto under sdk/proto/cosmos/slashing/v1beta1 (4 at HEAD, for example sdk/proto/cosmos/slashing/v1beta1/genesis.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.275.1</code> | req | 7275.1 |
| <code>acceptance.7.275.1</code> | scenario | Given the buf image of sdk/proto/cosmos/staking/v1beta1 and the descriptors its generated Go registers, when TestCosmosStakingDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.275.1</code> | test | api/cosmos_staking_parity_test.go::TestCosmosStakingDescriptorParity (planned) |
| <code>acceptance.7.275.1</code> | qualification | UNRUN |
| <code>req.7275</code> | ac_1 | Every .proto under sdk/proto/cosmos/staking/v1beta1 (5 at HEAD, for example sdk/proto/cosmos/staking/v1beta1/authz.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.276.1</code> | req | 7276.1 |
| <code>acceptance.7.276.1</code> | scenario | Given the buf image of sdk/proto/cosmos/upgrade/v1beta1 and the descriptors its generated Go registers, when TestCosmosUpgradeDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.276.1</code> | test | api/cosmos_upgrade_parity_test.go::TestCosmosUpgradeDescriptorParity (planned) |
| <code>acceptance.7.276.1</code> | qualification | UNRUN |
| <code>req.7276</code> | ac_1 | Every .proto under sdk/proto/cosmos/upgrade/v1beta1 (2 at HEAD, for example sdk/proto/cosmos/upgrade/v1beta1/query.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.277.1</code> | req | 7277.1 |
| <code>acceptance.7.277.1</code> | scenario | Given the buf image of sdk/proto/cosmos/tx/signing/v1beta1, sdk/proto/cosmos/tx/v1beta1 and the descriptors its generated Go registers, when TestCosmosTxDescriptorParity compares package, imports, messages, fields, enums, rpcs and HTTP rules file by file, then no difference is reported and every .proto in the directory resolves to a registered descriptor |
| <code>acceptance.7.277.1</code> | test | api/cosmos_tx_parity_test.go::TestCosmosTxDescriptorParity (planned) |
| <code>acceptance.7.277.1</code> | qualification | UNRUN |
| <code>req.7277</code> | ac_1 | Every .proto under sdk/proto/cosmos/tx/signing/v1beta1, sdk/proto/cosmos/tx/v1beta1 (3 at HEAD, for example sdk/proto/cosmos/tx/signing/v1beta1/signing.proto:1) has a descriptor registered by its generated Go whose package, imports, messages, fields, enums, rpcs and HTTP rules equal the buf image of the source (planned). |
| <code>acceptance.7.286.1</code> | req | 7286.1 |
| <code>acceptance.7.286.1</code> | scenario | Given the layerxprograms protos and the generated Go of wiring.J39.8 linked into the test binary, when TestLayerxProgramsDescriptorParity resolves each file in the gogo registry and compares shapes, then every file resolves and no difference is reported |
| <code>acceptance.7.286.1</code> | test | api/layerx_programs_parity_test.go::TestLayerxProgramsDescriptorParity (planned) |
| <code>acceptance.7.286.1</code> | qualification | UNRUN |
| <code>req.7286</code> | ac_1 | Every .proto under api/layerxprograms (api/layerxprograms/genesis.proto:1, api/layerxprograms/query.proto:1, api/layerxprograms/tx.proto:1) has a descriptor registered by checked-in generated Go under its go_package directory, equal in shape to the buf image of the source. |
| <code>resolution.17.787</code> | title | decision.725 binds: Proto parity is checked at descriptor shape level |
| <code>resolution.17.787</code> | applies_to | decision.725<br>task.7.21.2<br>task.7.24.4<br>task.7.26.2<br>task.7.29.2 |
| <code>resolution.17.787</code> | source | decision.725 in fragments/J39/decision.kvx; tasks from affects |
| <code>resolution.17.793</code> | title | decision.731 binds: layerxprograms protos are owned by the programs domain |
| <code>resolution.17.793</code> | applies_to | decision.731<br>task.7.28.3 |
| <code>resolution.17.793</code> | source | decision.731 in fragments/J39/decision.kvx; tasks from affects |
| <code>resolution.17.937</code> | title | decision.1341 binds: layerxprograms Msg and Query services stay descriptor-only |
| <code>resolution.17.937</code> | applies_to | decision.1341<br>task.7.28.3<br>task.13.95.1 |
| <code>resolution.17.937</code> | source | decision.1341 in fragments/J74/decision.kvx; tasks from affects |

## descriptor-registry

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.284.1</code> | req | 7284.1 |
| <code>acceptance.7.284.1</code> | scenario | Given a test binary linking node with the regenerated engine/deps/xevm/types of wiring.J39.6, when TestXevmDescriptorsDoNotShadowEvmFiles looks up the ten evm and eth file names in the gogo registry, then each returns the modules/evm/types bytes, and every xevm descriptor uses a distinct file name and a package equal to its RegisterType prefix |
| <code>acceptance.7.284.1</code> | test | api/xevm_shadow_test.go::TestXevmDescriptorsDoNotShadowEvmFiles (planned) |
| <code>acceptance.7.284.1</code> | qualification | UNRUN |
| <code>req.7284</code> | ac_1 | In a binary that links node, gogoproto.FileDescriptor of each evm/*.proto name and of eth/tx.proto returns the descriptor of modules/evm/types (modules/evm/types/params.pb.go:533), and the engine/deps/xevm/types descriptors (engine/deps/xevm/types/params.pb.go:446) register under other file names with a package matching their RegisterType names (engine/deps/xevm/types/tx.pb.go:917). |
| <code>acceptance.7.285.1</code> | req | 7285.1 |
| <code>acceptance.7.285.1</code> | scenario | Given a test binary linking node with the regenerated engine/deps/xbank/types of wiring.J39.7, when TestXbankDescriptorsDoNotShadowBankFiles looks up the three bank file names in the gogo registry, then each returns the sdk/x/bank/types bytes, and every xbank descriptor uses a distinct file name and a hyphen-free package equal to its RegisterType prefix |
| <code>acceptance.7.285.1</code> | test | api/xbank_shadow_test.go::TestXbankDescriptorsDoNotShadowBankFiles (planned) |
| <code>acceptance.7.285.1</code> | qualification | UNRUN |
| <code>req.7285</code> | ac_1 | In a binary that links node, gogoproto.FileDescriptor of cosmos/bank/v1beta1/authz.proto, bank.proto and genesis.proto returns the sdk/x/bank/types descriptors (sdk/x/bank/types/bank.pb.go:476), and the engine/deps/xbank/types descriptors (engine/deps/xbank/types/bank.pb.go:477) register under other file names with a valid package matching their RegisterType names. |
| <code>resolution.17.792</code> | title | decision.730 binds: One registration per proto file name |
| <code>resolution.17.792</code> | applies_to | decision.730<br>task.7.28.1<br>task.7.28.2 |
| <code>resolution.17.792</code> | source | decision.730 in fragments/J39/decision.kvx; tasks from affects |

## gateway-mount

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.287.1</code> | req | 7287.1 |
| <code>acceptance.7.287.1</code> | scenario | Given the gateway mux registered by authtx, tmservice and node.ModuleBasics with a node-less client context, when TestEveryHTTPRuleIsMounted sends one request per declared HTTP rule with every path variable filled, then no request answers 404 or 405, the two reflection services excepted |
| <code>acceptance.7.287.1</code> | test | api/gateway_mount_test.go::TestEveryHTTPRuleIsMounted (planned) |
| <code>acceptance.7.287.1</code> | qualification | UNRUN |
| <code>req.7287</code> | ac_1 | Every google.api.http rule and additional binding declared under api/ and sdk/proto/, except the two reflection services, is matched by the gateway mux built the way RegisterAPIRoutes builds it (node/app.go:3014-3033) and answers with a status other than 404 and 405. |
| <code>resolution.17.789</code> | title | decision.727 binds: REST path prefixes stay as declared |
| <code>resolution.17.789</code> | applies_to | decision.727<br>task.7.29.1 |
| <code>resolution.17.789</code> | source | decision.727 in fragments/J39/decision.kvx; tasks from affects |
| <code>resolution.17.790</code> | title | decision.728 binds: Reflection services stay gRPC-only |
| <code>resolution.17.790</code> | applies_to | decision.728<br>task.7.29.1 |
| <code>resolution.17.790</code> | source | decision.728 in fragments/J39/decision.kvx; tasks from affects |

## grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:12</code> | <code>cosmos.base.reflection.v1beta1.ReflectionService/ListAllInterfaces: rpc ListAllInterfaces(ListAllInterfacesRequest) returns (ListAllInterfacesResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39</code> | notes | handler sdk/client/grpc/reflection/reflection.go:25; callers are references to request type ListAllInterfacesRequest |

## grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:18</code> | <code>cosmos.base.reflection.v1beta1.ReflectionService/ListImplementations: rpc ListImplementations(ListImplementationsRequest) returns (ListImplementationsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39</code> | notes | handler sdk/client/grpc/reflection/reflection.go:33; callers are references to request type ListImplementationsRequest |

## grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:119</code> | <code>cosmos.base.reflection.v2alpha1.ReflectionService/GetAuthnDescriptor: rpc GetAuthnDescriptor(GetAuthnDescriptorRequest) returns (GetAuthnDescriptorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39</code> | notes | handler sdk/server/grpc/reflection/v2alpha1/reflection.go:37; callers are references to request type GetAuthnDescriptorRequest |

## grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:123</code> | <code>cosmos.base.reflection.v2alpha1.ReflectionService/GetChainDescriptor: rpc GetChainDescriptor(GetChainDescriptorRequest) returns (GetChainDescriptorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39</code> | notes | handler sdk/server/grpc/reflection/v2alpha1/reflection.go:41; callers are references to request type GetChainDescriptorRequest |

## grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:127</code> | <code>cosmos.base.reflection.v2alpha1.ReflectionService/GetCodecDescriptor: rpc GetCodecDescriptor(GetCodecDescriptorRequest) returns (GetCodecDescriptorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39</code> | notes | handler sdk/server/grpc/reflection/v2alpha1/reflection.go:45; callers are references to request type GetCodecDescriptorRequest |

## grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:131</code> | <code>cosmos.base.reflection.v2alpha1.ReflectionService/GetConfigurationDescriptor: rpc GetConfigurationDescriptor(GetConfigurationDescriptorRequest) returns (GetConfigurationDescriptorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39</code> | notes | handler sdk/server/grpc/reflection/v2alpha1/reflection.go:49; callers are references to request type GetConfigurationDescriptorRequest |

## grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:135</code> | <code>cosmos.base.reflection.v2alpha1.ReflectionService/GetQueryServicesDescriptor: rpc GetQueryServicesDescriptor(GetQueryServicesDescriptorRequest) returns (GetQueryServicesDescriptorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39</code> | notes | handler sdk/server/grpc/reflection/v2alpha1/reflection.go:53; callers are references to request type GetQueryServicesDescriptorRequest |

## grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:139</code> | <code>cosmos.base.reflection.v2alpha1.ReflectionService/GetTxDescriptor: rpc GetTxDescriptor(GetTxDescriptorRequest) returns (GetTxDescriptorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39</code> | notes | handler sdk/server/grpc/reflection/v2alpha1/reflection.go:57; callers are references to request type GetTxDescriptorRequest |

## grpcs-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:28</code> | <code>cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight: rpc GetBlockByHeight(GetBlockByHeightRequest) returns (GetBlockByHeightResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39</code> | notes | handler sdk/client/grpc/tmservice/service.go:82; callers are references to request type GetBlockByHeightRequest |

## grpcs-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:24</code> | <code>cosmos.base.tendermint.v1beta1.Service/GetLatestBlock: rpc GetLatestBlock(GetLatestBlockRequest) returns (GetLatestBlockResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39</code> | notes | handler sdk/client/grpc/tmservice/service.go:63; callers are references to request type GetLatestBlockRequest |

## grpcs-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:33</code> | <code>cosmos.base.tendermint.v1beta1.Service/GetLatestValidatorSet: rpc GetLatestValidatorSet(GetLatestValidatorSetRequest) returns (GetLatestValidatorSetResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39</code> | notes | handler sdk/client/grpc/tmservice/service.go:103; callers are references to request type GetLatestValidatorSetRequest |

## grpcs-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:16</code> | <code>cosmos.base.tendermint.v1beta1.Service/GetNodeInfo: rpc GetNodeInfo(GetNodeInfoRequest) returns (GetNodeInfoResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39</code> | notes | handler sdk/client/grpc/tmservice/service.go:174; callers are references to request type GetNodeInfoRequest |

## grpcs-cosmos-base-tendermint-v1beta1-service-getsyncing-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getsyncing-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:20</code> | <code>cosmos.base.tendermint.v1beta1.Service/GetSyncing: rpc GetSyncing(GetSyncingRequest) returns (GetSyncingResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getsyncing-j39</code> | notes | handler sdk/client/grpc/tmservice/service.go:52; callers are references to request type GetSyncingRequest |

## grpcs-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:37</code> | <code>cosmos.base.tendermint.v1beta1.Service/GetValidatorSetByHeight: rpc GetValidatorSetByHeight(GetValidatorSetByHeightRequest) returns (GetValidatorSetByHeightResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39</code> | notes | handler sdk/client/grpc/tmservice/service.go:123; callers are references to request type GetValidatorSetByHeightRequest |

## grpcs-cosmos-tx-v1beta1-service-broadcasttx-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-broadcasttx-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:29</code> | <code>cosmos.tx.v1beta1.Service/BroadcastTx: rpc BroadcastTx(BroadcastTxRequest) returns (BroadcastTxResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-broadcasttx-j39</code> | notes | handler sdk/x/auth/tx/service.go:247; callers are references to request type BroadcastTxRequest |

## grpcs-cosmos-tx-v1beta1-service-getblockwithtxs-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-getblockwithtxs-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:42</code> | <code>cosmos.tx.v1beta1.Service/GetBlockWithTxs: rpc GetBlockWithTxs(GetBlockWithTxsRequest) returns (GetBlockWithTxsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-getblockwithtxs-j39</code> | notes | handler sdk/x/auth/tx/service.go:173; callers are references to request type GetBlockWithTxsRequest |

## grpcs-cosmos-tx-v1beta1-service-gettx-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-gettx-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:25</code> | <code>cosmos.tx.v1beta1.Service/GetTx: rpc GetTx(GetTxRequest) returns (GetTxResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-gettx-j39</code> | notes | handler sdk/x/auth/tx/service.go:134; callers are references to request type GetTxRequest |

## grpcs-cosmos-tx-v1beta1-service-gettxsevent-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-gettxsevent-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:36</code> | <code>cosmos.tx.v1beta1.Service/GetTxsEvent: rpc GetTxsEvent(GetTxsEventRequest) returns (GetTxsEventResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-gettxsevent-j39</code> | notes | handler sdk/x/auth/tx/service.go:52; callers are references to request type GetTxsEventRequest |

## grpcs-cosmos-tx-v1beta1-service-simulate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-simulate-j39</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:18</code> | <code>cosmos.tx.v1beta1.Service/Simulate: rpc Simulate(SimulateRequest) returns (SimulateResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.grpcs-cosmos-tx-v1beta1-service-simulate-j39</code> | notes | handler sdk/x/auth/tx/service.go:100; callers are references to request type SimulateRequest |

## msg-cosmos-authz-v1beta1-exec-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-authz-v1beta1-exec-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:24</code> | <code>cosmos.authz.v1beta1.Msg/Exec: rpc Exec(MsgExec) returns (MsgExecResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-authz-v1beta1-exec-j39</code> | notes | handler sdk/x/authz/keeper/msg_server.go:65; callers are references to request type MsgExec |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-authz-v1beta1-grant-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-authz-v1beta1-grant-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:19</code> | <code>cosmos.authz.v1beta1.Msg/Grant: rpc Grant(MsgGrant) returns (MsgGrantResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-authz-v1beta1-grant-j39</code> | notes | handler sdk/x/authz/keeper/msg_server.go:14; callers are references to request type MsgGrant |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-authz-v1beta1-revoke-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-authz-v1beta1-revoke-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:28</code> | <code>cosmos.authz.v1beta1.Msg/Revoke: rpc Revoke(MsgRevoke) returns (MsgRevokeResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-authz-v1beta1-revoke-j39</code> | notes | handler sdk/x/authz/keeper/msg_server.go:45; callers are references to request type MsgRevoke |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-bank-v1beta1-multisend-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-bank-v1beta1-multisend-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/tx.proto:16</code> | <code>cosmos.bank.v1beta1.Msg/MultiSend: rpc MultiSend(MsgMultiSend) returns (MsgMultiSendResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-bank-v1beta1-multisend-j39</code> | notes | handler sdk/x/bank/keeper/msg_server.go:85; callers are references to request type MsgMultiSend |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-bank-v1beta1-send-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-bank-v1beta1-send-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/tx.proto:13</code> | <code>cosmos.bank.v1beta1.Msg/Send: rpc Send(MsgSend) returns (MsgSendResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-bank-v1beta1-send-j39</code> | notes | handler sdk/x/bank/keeper/msg_server.go:28; callers are references to request type MsgSend (68 files, first 25 listed) |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-distribution-v1beta1-fundcommunitypool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-fundcommunitypool-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:26</code> | <code>cosmos.distribution.v1beta1.Msg/FundCommunityPool: rpc FundCommunityPool(MsgFundCommunityPool) returns (MsgFundCommunityPoolResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-fundcommunitypool-j39</code> | notes | handler sdk/x/distribution/keeper/msg_server.go:131; callers are references to request type MsgFundCommunityPool |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-distribution-v1beta1-setwithdrawaddress-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-setwithdrawaddress-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:14</code> | <code>cosmos.distribution.v1beta1.Msg/SetWithdrawAddress: rpc SetWithdrawAddress(MsgSetWithdrawAddress) returns (MsgSetWithdrawAddressResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-setwithdrawaddress-j39</code> | notes | handler sdk/x/distribution/keeper/msg_server.go:27; callers are references to request type MsgSetWithdrawAddress |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-distribution-v1beta1-withdrawdelegatorreward-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-withdrawdelegatorreward-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:18</code> | <code>cosmos.distribution.v1beta1.Msg/WithdrawDelegatorReward: rpc WithdrawDelegatorReward(MsgWithdrawDelegatorReward) returns (MsgWithdrawDelegatorRewardResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-withdrawdelegatorreward-j39</code> | notes | handler sdk/x/distribution/keeper/msg_server.go:54; callers are references to request type MsgWithdrawDelegatorReward |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-distribution-v1beta1-withdrawvalidatorcommission-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-withdrawvalidatorcommission-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:22</code> | <code>cosmos.distribution.v1beta1.Msg/WithdrawValidatorCommission: rpc WithdrawValidatorCommission(MsgWithdrawValidatorCommission) returns (MsgWithdrawValidatorCommissionResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-distribution-v1beta1-withdrawvalidatorcommission-j39</code> | notes | handler sdk/x/distribution/keeper/msg_server.go:94; callers are references to request type MsgWithdrawValidatorCommission |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-evidence-v1beta1-submitevidence-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-evidence-v1beta1-submitevidence-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/tx.proto:15</code> | <code>cosmos.evidence.v1beta1.Msg/SubmitEvidence: rpc SubmitEvidence(MsgSubmitEvidence) returns (MsgSubmitEvidenceResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-evidence-v1beta1-submitevidence-j39</code> | notes | handler sdk/x/evidence/keeper/msg_server.go:23; callers are references to request type MsgSubmitEvidence |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-feegrant-v1beta1-grantallowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-feegrant-v1beta1-grantallowance-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/tx.proto:14</code> | <code>cosmos.feegrant.v1beta1.Msg/GrantAllowance: rpc GrantAllowance(MsgGrantAllowance) returns (MsgGrantAllowanceResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-feegrant-v1beta1-grantallowance-j39</code> | notes | handler sdk/x/feegrant/keeper/msg_server.go:27; callers are references to request type MsgGrantAllowance |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-feegrant-v1beta1-revokeallowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-feegrant-v1beta1-revokeallowance-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/tx.proto:18</code> | <code>cosmos.feegrant.v1beta1.Msg/RevokeAllowance: rpc RevokeAllowance(MsgRevokeAllowance) returns (MsgRevokeAllowanceResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-feegrant-v1beta1-revokeallowance-j39</code> | notes | handler sdk/x/feegrant/keeper/msg_server.go:59; callers are references to request type MsgRevokeAllowance |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-gov-v1beta1-deposit-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-deposit-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:26</code> | <code>cosmos.gov.v1beta1.Msg/Deposit: rpc Deposit(MsgDeposit) returns (MsgDepositResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-deposit-j39</code> | notes | handler sdk/x/gov/keeper/msg_server.go:135; callers are references to request type MsgDeposit |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-gov-v1beta1-submitproposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-submitproposal-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:15</code> | <code>cosmos.gov.v1beta1.Msg/SubmitProposal: rpc SubmitProposal(MsgSubmitProposal) returns (MsgSubmitProposalResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-submitproposal-j39</code> | notes | handler sdk/x/gov/keeper/msg_server.go:27; callers are references to request type MsgSubmitProposal |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-gov-v1beta1-vote-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-vote-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:18</code> | <code>cosmos.gov.v1beta1.Msg/Vote: rpc Vote(MsgVote) returns (MsgVoteResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-vote-j39</code> | notes | handler sdk/x/gov/keeper/msg_server.go:66; callers are references to request type MsgVote |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-gov-v1beta1-voteweighted-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-voteweighted-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:23</code> | <code>cosmos.gov.v1beta1.Msg/VoteWeighted: rpc VoteWeighted(MsgVoteWeighted) returns (MsgVoteWeightedResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-gov-v1beta1-voteweighted-j39</code> | notes | handler sdk/x/gov/keeper/msg_server.go:101; callers are references to request type MsgVoteWeighted |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-slashing-v1beta1-unjail-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-slashing-v1beta1-unjail-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/tx.proto:14</code> | <code>cosmos.slashing.v1beta1.Msg/Unjail: rpc Unjail(MsgUnjail) returns (MsgUnjailResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-slashing-v1beta1-unjail-j39</code> | notes | handler sdk/x/slashing/keeper/msg_server.go:25; callers are references to request type MsgUnjail |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-staking-v1beta1-beginredelegate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-beginredelegate-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:27</code> | <code>cosmos.staking.v1beta1.Msg/BeginRedelegate: rpc BeginRedelegate(MsgBeginRedelegate) returns (MsgBeginRedelegateResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-beginredelegate-j39</code> | notes | handler sdk/x/staking/keeper/msg_server.go:259; callers are references to request type MsgBeginRedelegate |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-staking-v1beta1-createvalidator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-createvalidator-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:16</code> | <code>cosmos.staking.v1beta1.Msg/CreateValidator: rpc CreateValidator(MsgCreateValidator) returns (MsgCreateValidatorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-createvalidator-j39</code> | notes | handler sdk/x/staking/keeper/msg_server.go:30; callers are references to request type MsgCreateValidator |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-staking-v1beta1-delegate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-delegate-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:23</code> | <code>cosmos.staking.v1beta1.Msg/Delegate: rpc Delegate(MsgDelegate) returns (MsgDelegateResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-delegate-j39</code> | notes | handler sdk/x/staking/keeper/msg_server.go:196; callers are references to request type MsgDelegate |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-staking-v1beta1-editvalidator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-editvalidator-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:19</code> | <code>cosmos.staking.v1beta1.Msg/EditValidator: rpc EditValidator(MsgEditValidator) returns (MsgEditValidatorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-editvalidator-j39</code> | notes | handler sdk/x/staking/keeper/msg_server.go:133; callers are references to request type MsgEditValidator |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-staking-v1beta1-undelegate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-undelegate-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:31</code> | <code>cosmos.staking.v1beta1.Msg/Undelegate: rpc Undelegate(MsgUndelegate) returns (MsgUndelegateResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-staking-v1beta1-undelegate-j39</code> | notes | handler sdk/x/staking/keeper/msg_server.go:331; callers are references to request type MsgUndelegate |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## msg-cosmos-vesting-v1beta1-createvestingaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.msg-cosmos-vesting-v1beta1-createvestingaccount-j39</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/tx.proto:13</code> | <code>cosmos.vesting.v1beta1.Msg/CreateVestingAccount: rpc CreateVestingAccount(MsgCreateVestingAccount) returns (MsgCreateVestingAccountResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.msg-cosmos-vesting-v1beta1-createvestingaccount-j39</code> | notes | handler sdk/x/auth/vesting/msg_server.go:32; callers are references to request type MsgCreateVestingAccount |

Contract context <code>contract_context.7.descriptor-parity</code>:

| Field | Frozen rule |
|---|---|
| interface | Proto sources under api/ and sdk/proto/ and the descriptors registered by their generated Go. |
| storage | Gzipped FileDescriptorProto bytes in generated *.pb.go files and the gogo file registry; no chain state. |
| errors | Parity tests report &lt;file&gt;: &lt;element&gt; &lt;name&gt;: source &lt;value&gt; generated &lt;value&gt;. |
| events | None. |
| gas | None. |
| activation | Effective at build time of the node binary. |
| invariants | Each registered descriptor equals its source in package, imports, messages, fields, enums, rpcs and HTTP rules; removed field numbers stay reserved; one registration per file name. |
| pause | Not pausable. |

## proto-cosmos-auth-v1beta1-baseaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-baseaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/auth.proto:13</code> | <code>message cosmos.auth.v1beta1.BaseAccount { string address = 1; google.protobuf.Any pub_key = 2; uint64 account_number = 3; uint64 sequence = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-baseaccount-j39</code> | notes | Go type BaseAccount at sdk/x/auth/types/auth.pb.go:31 |

## proto-cosmos-auth-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/genesis.proto:11</code> | <code>message cosmos.auth.v1beta1.GenesisState { Params params = 1; repeated google.protobuf.Any accounts = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/auth/types/genesis.pb.go:28 |

## proto-cosmos-auth-v1beta1-moduleaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-moduleaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/auth.proto:30</code> | <code>message cosmos.auth.v1beta1.ModuleAccount { BaseAccount base_account = 1; string name = 2; repeated string permissions = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-moduleaccount-j39</code> | notes | Go type ModuleAccount at sdk/x/auth/types/auth.pb.go:71 |

## proto-cosmos-auth-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-params-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/auth.proto:44</code> | <code>message cosmos.auth.v1beta1.Params { uint64 max_memo_characters = 1; uint64 tx_sig_limit = 2; uint64 tx_size_cost_per_byte = 3; uint64 sig_verify_cost_ed25519 = 4; uint64 sig_verify_cost_secp256k1 = 5; bool disable_seqno_check = 6; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-params-j39</code> | notes | Go type Params at sdk/x/auth/types/auth.pb.go:110 |

## proto-cosmos-auth-v1beta1-queryaccountrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:58</code> | <code>message cosmos.auth.v1beta1.QueryAccountRequest { string address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountrequest-j39</code> | notes | Go type QueryAccountRequest at sdk/x/auth/types/query.pb.go:141 |

## proto-cosmos-auth-v1beta1-queryaccountresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:67</code> | <code>message cosmos.auth.v1beta1.QueryAccountResponse { google.protobuf.Any account = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountresponse-j39</code> | notes | Go type QueryAccountResponse at sdk/x/auth/types/query.pb.go:180 |

## proto-cosmos-auth-v1beta1-queryaccountsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:41</code> | <code>message cosmos.auth.v1beta1.QueryAccountsRequest { cosmos.base.query.v1beta1.PageRequest pagination = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountsrequest-j39</code> | notes | Go type QueryAccountsRequest at sdk/x/auth/types/query.pb.go:38 |

## proto-cosmos-auth-v1beta1-queryaccountsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:49</code> | <code>message cosmos.auth.v1beta1.QueryAccountsResponse { repeated google.protobuf.Any accounts = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryaccountsresponse-j39</code> | notes | Go type QueryAccountsResponse at sdk/x/auth/types/query.pb.go:86 |

## proto-cosmos-auth-v1beta1-querynextaccountnumberrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-querynextaccountnumberrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:82</code> | <code>message cosmos.auth.v1beta1.QueryNextAccountNumberRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-querynextaccountnumberrequest-j39</code> | notes | Go type QueryNextAccountNumberRequest at sdk/x/auth/types/query.pb.go:309 |

## proto-cosmos-auth-v1beta1-querynextaccountnumberresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-querynextaccountnumberresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:85</code> | <code>message cosmos.auth.v1beta1.QueryNextAccountNumberResponse { uint64 count = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-querynextaccountnumberresponse-j39</code> | notes | Go type QueryNextAccountNumberResponse at sdk/x/auth/types/query.pb.go:346 |

## proto-cosmos-auth-v1beta1-queryparamsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryparamsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:73</code> | <code>message cosmos.auth.v1beta1.QueryParamsRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryparamsrequest-j39</code> | notes | Go type QueryParamsRequest at sdk/x/auth/types/query.pb.go:226 |

## proto-cosmos-auth-v1beta1-queryparamsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryparamsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:76</code> | <code>message cosmos.auth.v1beta1.QueryParamsResponse { Params params = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-auth-v1beta1-queryparamsresponse-j39</code> | notes | Go type QueryParamsResponse at sdk/x/auth/types/query.pb.go:263 |

## proto-cosmos-authz-v1beta1-eventgrant-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-eventgrant-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/event.proto:8</code> | <code>message cosmos.authz.v1beta1.EventGrant { string msg_type_url = 2; string granter = 3; string grantee = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-eventgrant-j39</code> | notes | Go type EventGrant at sdk/x/authz/event.pb.go:26 |

## proto-cosmos-authz-v1beta1-eventrevoke-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-eventrevoke-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/event.proto:18</code> | <code>message cosmos.authz.v1beta1.EventRevoke { string msg_type_url = 2; string granter = 3; string grantee = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-eventrevoke-j39</code> | notes | Go type EventRevoke at sdk/x/authz/event.pb.go:90 |

## proto-cosmos-authz-v1beta1-genericauthorization-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-genericauthorization-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/authz.proto:15</code> | <code>message cosmos.authz.v1beta1.GenericAuthorization { string msg = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-genericauthorization-j39</code> | notes | Go type GenericAuthorization at sdk/x/authz/authz.pb.go:34 |

## proto-cosmos-authz-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/genesis.proto:11</code> | <code>message cosmos.authz.v1beta1.GenesisState { repeated GrantAuthorization authorization = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/authz/genesis.pb.go:27 |

## proto-cosmos-authz-v1beta1-grant-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-grant-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/authz.proto:24</code> | <code>message cosmos.authz.v1beta1.Grant { google.protobuf.Any authorization = 1; google.protobuf.Timestamp expiration = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-grant-j39</code> | notes | Go type Grant at sdk/x/authz/authz.pb.go:74 |

## proto-cosmos-authz-v1beta1-grantauthorization-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-grantauthorization-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/authz.proto:36</code> | <code>message cosmos.authz.v1beta1.GrantAuthorization { string granter = 1; string grantee = 2; google.protobuf.Any authorization = 3; google.protobuf.Timestamp expiration = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-grantauthorization-j39</code> | notes | Go type GrantAuthorization at sdk/x/authz/authz.pb.go:116 |

## proto-cosmos-authz-v1beta1-msgexec-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgexec-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:48</code> | <code>message cosmos.authz.v1beta1.MsgExec { string grantee = 1; repeated google.protobuf.Any msgs = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgexec-j39</code> | notes | Go type MsgExec at sdk/x/authz/tx.pb.go:115 |

## proto-cosmos-authz-v1beta1-msgexecresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgexecresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:41</code> | <code>message cosmos.authz.v1beta1.MsgExecResponse { repeated bytes results = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgexecresponse-j39</code> | notes | Go type MsgExecResponse at sdk/x/authz/tx.pb.go:75 |

## proto-cosmos-authz-v1beta1-msggrant-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msggrant-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:33</code> | <code>message cosmos.authz.v1beta1.MsgGrant { string granter = 1; string grantee = 2; cosmos.authz.v1beta1.Grant grant = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msggrant-j39</code> | notes | Go type MsgGrant at sdk/x/authz/tx.pb.go:35 |

## proto-cosmos-authz-v1beta1-msggrantresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msggrantresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:57</code> | <code>message cosmos.authz.v1beta1.MsgGrantResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msggrantresponse-j39</code> | notes | Go type MsgGrantResponse at sdk/x/authz/tx.pb.go:157 |

## proto-cosmos-authz-v1beta1-msgrevoke-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgrevoke-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:61</code> | <code>message cosmos.authz.v1beta1.MsgRevoke { string granter = 1; string grantee = 2; string msg_type_url = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgrevoke-j39</code> | notes | Go type MsgRevoke at sdk/x/authz/tx.pb.go:195 |

## proto-cosmos-authz-v1beta1-msgrevokeresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgrevokeresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/tx.proto:68</code> | <code>message cosmos.authz.v1beta1.MsgRevokeResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-msgrevokeresponse-j39</code> | notes | Go type MsgRevokeResponse at sdk/x/authz/tx.pb.go:235 |

## proto-cosmos-authz-v1beta1-querygranteegrantsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygranteegrantsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:68</code> | <code>message cosmos.authz.v1beta1.QueryGranteeGrantsRequest { string grantee = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygranteegrantsrequest-j39</code> | notes | Go type QueryGranteeGrantsRequest at sdk/x/authz/query.pb.go:268 |

## proto-cosmos-authz-v1beta1-querygranteegrantsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygranteegrantsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:76</code> | <code>message cosmos.authz.v1beta1.QueryGranteeGrantsResponse { repeated GrantAuthorization grants = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygranteegrantsresponse-j39</code> | notes | Go type QueryGranteeGrantsResponse at sdk/x/authz/query.pb.go:322 |

## proto-cosmos-authz-v1beta1-querygrantergrantsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantergrantsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:52</code> | <code>message cosmos.authz.v1beta1.QueryGranterGrantsRequest { string granter = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantergrantsrequest-j39</code> | notes | Go type QueryGranterGrantsRequest at sdk/x/authz/query.pb.go:159 |

## proto-cosmos-authz-v1beta1-querygrantergrantsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantergrantsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:60</code> | <code>message cosmos.authz.v1beta1.QueryGranterGrantsResponse { repeated GrantAuthorization grants = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantergrantsresponse-j39</code> | notes | Go type QueryGranterGrantsResponse at sdk/x/authz/query.pb.go:213 |

## proto-cosmos-authz-v1beta1-querygrantsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:34</code> | <code>message cosmos.authz.v1beta1.QueryGrantsRequest { string granter = 1; string grantee = 2; string msg_type_url = 3; cosmos.base.query.v1beta1.PageRequest pagination = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantsrequest-j39</code> | notes | Go type QueryGrantsRequest at sdk/x/authz/query.pb.go:33 |

## proto-cosmos-authz-v1beta1-querygrantsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:44</code> | <code>message cosmos.authz.v1beta1.QueryGrantsResponse { repeated Grant grants = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-authz-v1beta1-querygrantsresponse-j39</code> | notes | Go type QueryGrantsResponse at sdk/x/authz/query.pb.go:104 |

## proto-cosmos-bank-v1beta1-allowlist-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-allowlist-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:104</code> | <code>message cosmos.bank.v1beta1.AllowList { repeated string addresses = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-allowlist-j39</code> | notes | Go type AllowList at sdk/x/bank/types/bank.pb.go:420 |

## proto-cosmos-bank-v1beta1-balance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-balance-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/genesis.proto:37</code> | <code>message cosmos.bank.v1beta1.Balance { string address = 1; repeated cosmos.base.v1beta1.Coin coins = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-balance-j39</code> | notes | Go type Balance at sdk/x/bank/types/genesis.pb.go:113; 26 referencing files, first 25 listed |

## proto-cosmos-bank-v1beta1-denomunit-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-denomunit-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:69</code> | <code>message cosmos.bank.v1beta1.DenomUnit { string denom = 1; uint32 exponent = 2; repeated string aliases = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-denomunit-j39</code> | notes | Go type DenomUnit at sdk/x/bank/types/bank.pb.go:256; 30 referencing files, first 25 listed |

## proto-cosmos-bank-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/genesis.proto:11</code> | <code>message cosmos.bank.v1beta1.GenesisState { Params params = 1; repeated Balance balances = 2; repeated cosmos.base.v1beta1.Coin supply = 3; repeated Metadata denom_metadata = 4; repeated WeiBalance wei_balances = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/bank/types/genesis.pb.go:29 |

## proto-cosmos-bank-v1beta1-input-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-input-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:27</code> | <code>message cosmos.bank.v1beta1.Input { string address = 1; repeated cosmos.base.v1beta1.Coin coins = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-input-j39</code> | notes | Go type Input at sdk/x/bank/types/bank.pb.go:135 |

## proto-cosmos-bank-v1beta1-metadata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-metadata-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:84</code> | <code>message cosmos.bank.v1beta1.Metadata { string description = 1; repeated DenomUnit denom_units = 2; string base = 3; string display = 4; string name = 5; string symbol = 6; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-metadata-j39</code> | notes | Go type Metadata at sdk/x/bank/types/bank.pb.go:325; 52 referencing files, first 25 listed |

## proto-cosmos-bank-v1beta1-msgmultisend-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgmultisend-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/tx.proto:36</code> | <code>message cosmos.bank.v1beta1.MsgMultiSend { repeated Input inputs = 1; repeated Output outputs = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgmultisend-j39</code> | notes | Go type MsgMultiSend at sdk/x/bank/types/tx.pb.go:111 |

## proto-cosmos-bank-v1beta1-msgmultisendresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgmultisendresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/tx.proto:44</code> | <code>message cosmos.bank.v1beta1.MsgMultiSendResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgmultisendresponse-j39</code> | notes | Go type MsgMultiSendResponse at sdk/x/bank/types/tx.pb.go:164 |

## proto-cosmos-bank-v1beta1-msgsend-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgsend-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/tx.proto:20</code> | <code>message cosmos.bank.v1beta1.MsgSend { string from_address = 1; string to_address = 2; repeated cosmos.base.v1beta1.Coin amount = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgsend-j39</code> | notes | Go type MsgSend at sdk/x/bank/types/tx.pb.go:34; 69 referencing files, first 25 listed |

## proto-cosmos-bank-v1beta1-msgsendresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgsendresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/tx.proto:33</code> | <code>message cosmos.bank.v1beta1.MsgSendResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-msgsendresponse-j39</code> | notes | Go type MsgSendResponse at sdk/x/bank/types/tx.pb.go:74 |

## proto-cosmos-bank-v1beta1-output-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-output-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:39</code> | <code>message cosmos.bank.v1beta1.Output { string address = 1; repeated cosmos.base.v1beta1.Coin coins = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-output-j39</code> | notes | Go type Output at sdk/x/bank/types/bank.pb.go:174 |

## proto-cosmos-bank-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-params-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:11</code> | <code>message cosmos.bank.v1beta1.Params { repeated SendEnabled send_enabled = 1; bool default_send_enabled = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-params-j39</code> | notes | Go type Params at sdk/x/bank/types/bank.pb.go:30 |

## proto-cosmos-bank-v1beta1-queryallbalancesrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryallbalancesrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:75</code> | <code>message cosmos.bank.v1beta1.QueryAllBalancesRequest { string address = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryallbalancesrequest-j39</code> | notes | Go type QueryAllBalancesRequest at sdk/x/bank/types/query.pb.go:123 |

## proto-cosmos-bank-v1beta1-queryallbalancesresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryallbalancesresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:88</code> | <code>message cosmos.bank.v1beta1.QueryAllBalancesResponse { repeated cosmos.base.v1beta1.Coin balances = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryallbalancesresponse-j39</code> | notes | Go type QueryAllBalancesResponse at sdk/x/bank/types/query.pb.go:165 |

## proto-cosmos-bank-v1beta1-querybalancerequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querybalancerequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:57</code> | <code>message cosmos.bank.v1beta1.QueryBalanceRequest { string address = 1; string denom = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querybalancerequest-j39</code> | notes | Go type QueryBalanceRequest at sdk/x/bank/types/query.pb.go:36 |

## proto-cosmos-bank-v1beta1-querybalanceresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querybalanceresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:69</code> | <code>message cosmos.bank.v1beta1.QueryBalanceResponse { cosmos.base.v1beta1.Coin balance = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querybalanceresponse-j39</code> | notes | Go type QueryBalanceResponse at sdk/x/bank/types/query.pb.go:77 |

## proto-cosmos-bank-v1beta1-querydenommetadatarequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenommetadatarequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:189</code> | <code>message cosmos.bank.v1beta1.QueryDenomMetadataRequest { string denom = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenommetadatarequest-j39</code> | notes | Go type QueryDenomMetadataRequest at sdk/x/bank/types/query.pb.go:694 |

## proto-cosmos-bank-v1beta1-querydenommetadataresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenommetadataresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:196</code> | <code>message cosmos.bank.v1beta1.QueryDenomMetadataResponse { Metadata metadata = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenommetadataresponse-j39</code> | notes | Go type QueryDenomMetadataResponse at sdk/x/bank/types/query.pb.go:741 |

## proto-cosmos-bank-v1beta1-querydenomsmetadatarequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenomsmetadatarequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:173</code> | <code>message cosmos.bank.v1beta1.QueryDenomsMetadataRequest { cosmos.base.query.v1beta1.PageRequest pagination = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenomsmetadatarequest-j39</code> | notes | Go type QueryDenomsMetadataRequest at sdk/x/bank/types/query.pb.go:592 |

## proto-cosmos-bank-v1beta1-querydenomsmetadataresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenomsmetadataresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:180</code> | <code>message cosmos.bank.v1beta1.QueryDenomsMetadataResponse { repeated Metadata metadatas = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querydenomsmetadataresponse-j39</code> | notes | Go type QueryDenomsMetadataResponse at sdk/x/bank/types/query.pb.go:639 |

## proto-cosmos-bank-v1beta1-queryparamsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryparamsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:165</code> | <code>message cosmos.bank.v1beta1.QueryParamsRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryparamsrequest-j39</code> | notes | Go type QueryParamsRequest at sdk/x/bank/types/query.pb.go:510 |

## proto-cosmos-bank-v1beta1-queryparamsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryparamsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:168</code> | <code>message cosmos.bank.v1beta1.QueryParamsResponse { Params params = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryparamsresponse-j39</code> | notes | Go type QueryParamsResponse at sdk/x/bank/types/query.pb.go:547 |

## proto-cosmos-bank-v1beta1-queryspendablebalancesrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryspendablebalancesrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:101</code> | <code>message cosmos.bank.v1beta1.QuerySpendableBalancesRequest { string address = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryspendablebalancesrequest-j39</code> | notes | Go type QuerySpendableBalancesRequest at sdk/x/bank/types/query.pb.go:221 |

## proto-cosmos-bank-v1beta1-queryspendablebalancesresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryspendablebalancesresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:114</code> | <code>message cosmos.bank.v1beta1.QuerySpendableBalancesResponse { repeated cosmos.base.v1beta1.Coin balances = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-queryspendablebalancesresponse-j39</code> | notes | Go type QuerySpendableBalancesResponse at sdk/x/bank/types/query.pb.go:263 |

## proto-cosmos-bank-v1beta1-querysupplyofrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querysupplyofrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:153</code> | <code>message cosmos.bank.v1beta1.QuerySupplyOfRequest { string denom = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querysupplyofrequest-j39</code> | notes | Go type QuerySupplyOfRequest at sdk/x/bank/types/query.pb.go:418 |

## proto-cosmos-bank-v1beta1-querysupplyofresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querysupplyofresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:159</code> | <code>message cosmos.bank.v1beta1.QuerySupplyOfResponse { cosmos.base.v1beta1.Coin amount = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querysupplyofresponse-j39</code> | notes | Go type QuerySupplyOfResponse at sdk/x/bank/types/query.pb.go:464 |

## proto-cosmos-bank-v1beta1-querytotalsupplyrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querytotalsupplyrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:127</code> | <code>message cosmos.bank.v1beta1.QueryTotalSupplyRequest { cosmos.base.query.v1beta1.PageRequest pagination = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querytotalsupplyrequest-j39</code> | notes | Go type QueryTotalSupplyRequest at sdk/x/bank/types/query.pb.go:319 |

## proto-cosmos-bank-v1beta1-querytotalsupplyresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querytotalsupplyresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:139</code> | <code>message cosmos.bank.v1beta1.QueryTotalSupplyResponse { repeated cosmos.base.v1beta1.Coin supply = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-querytotalsupplyresponse-j39</code> | notes | Go type QueryTotalSupplyResponse at sdk/x/bank/types/query.pb.go:361 |

## proto-cosmos-bank-v1beta1-sendauthorization-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-sendauthorization-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/authz.proto:14</code> | <code>message cosmos.bank.v1beta1.SendAuthorization { repeated cosmos.base.v1beta1.Coin spend_limit = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-sendauthorization-j39</code> | notes | Go type SendAuthorization at sdk/x/bank/types/authz.pb.go:33 |

## proto-cosmos-bank-v1beta1-sendenabled-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-sendenabled-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:19</code> | <code>message cosmos.bank.v1beta1.SendEnabled { string denom = 1; bool enabled = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-sendenabled-j39</code> | notes | Go type SendEnabled at sdk/x/bank/types/bank.pb.go:83 |

## proto-cosmos-bank-v1beta1-supply-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-supply-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/bank.proto:53</code> | <code>message cosmos.bank.v1beta1.Supply { repeated cosmos.base.v1beta1.Coin total = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-supply-j39</code> | notes | Go type Supply at sdk/x/bank/types/bank.pb.go:217 |

## proto-cosmos-bank-v1beta1-weibalance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-weibalance-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/genesis.proto:51</code> | <code>message cosmos.bank.v1beta1.WeiBalance { string address = 1; string amount = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-bank-v1beta1-weibalance-j39</code> | notes | Go type WeiBalance at sdk/x/bank/types/genesis.pb.go:153 |

## proto-cosmos-base-abci-v1beta1-abcimessagelog-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-abcimessagelog-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:55</code> | <code>message cosmos.base.abci.v1beta1.ABCIMessageLog { uint32 msg_index = 1; string log = 2; repeated StringEvent events = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-abcimessagelog-j39</code> | notes | Go type ABCIMessageLog at sdk/types/abci.pb.go:102 |

## proto-cosmos-base-abci-v1beta1-attribute-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-attribute-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:80</code> | <code>message cosmos.base.abci.v1beta1.Attribute { string key = 1; string value = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-attribute-j39</code> | notes | Go type Attribute at sdk/types/abci.pb.go:218 |

## proto-cosmos-base-abci-v1beta1-gasinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-gasinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:86</code> | <code>message cosmos.base.abci.v1beta1.GasInfo { uint64 gas_wanted = 1; uint64 gas_used = 2; uint64 gas_estimate = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-gasinfo-j39</code> | notes | Go type GasInfo at sdk/types/abci.pb.go:270 |

## proto-cosmos-base-abci-v1beta1-msgdata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-msgdata-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:129</code> | <code>message cosmos.base.abci.v1beta1.MsgData { string msg_type = 1; bytes data = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-msgdata-j39</code> | notes | Go type MsgData at sdk/types/abci.pb.go:427 |

## proto-cosmos-base-abci-v1beta1-result-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-result-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:98</code> | <code>message cosmos.base.abci.v1beta1.Result { bytes data = 1; string log = 2; repeated tendermint.abci.Event events = 3; string evmError = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-result-j39</code> | notes | Go type Result at sdk/types/abci.pb.go:333; 43 referencing files, first 25 listed |

## proto-cosmos-base-abci-v1beta1-searchtxsresult-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-searchtxsresult-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:145</code> | <code>message cosmos.base.abci.v1beta1.SearchTxsResult { uint64 total_count = 1; uint64 count = 2; uint64 page_number = 3; uint64 page_total = 4; uint64 limit = 5; repeated TxResponse txs = 6; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-searchtxsresult-j39</code> | notes | Go type SearchTxsResult at sdk/types/abci.pb.go:524 |

## proto-cosmos-base-abci-v1beta1-simulationresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-simulationresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:119</code> | <code>message cosmos.base.abci.v1beta1.SimulationResponse { GasInfo gas_info = 1; Result result = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-simulationresponse-j39</code> | notes | Go type SimulationResponse at sdk/types/abci.pb.go:381 |

## proto-cosmos-base-abci-v1beta1-stringevent-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-stringevent-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:71</code> | <code>message cosmos.base.abci.v1beta1.StringEvent { string type = 1; repeated Attribute attributes = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-stringevent-j39</code> | notes | Go type StringEvent at sdk/types/abci.pb.go:165 |

## proto-cosmos-base-abci-v1beta1-txmsgdata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-txmsgdata-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:138</code> | <code>message cosmos.base.abci.v1beta1.TxMsgData { repeated MsgData data = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-txmsgdata-j39</code> | notes | Go type TxMsgData at sdk/types/abci.pb.go:480 |

## proto-cosmos-base-abci-v1beta1-txresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-txresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/abci/v1beta1/abci.proto:13</code> | <code>message cosmos.base.abci.v1beta1.TxResponse { int64 height = 1; string txhash = 2; string codespace = 3; uint32 code = 4; string data = 5; string raw_log = 6; repeated ABCIMessageLog logs = 7; string info = 8; int64 gas_wanted = 9; int64 gas_used = 10; google.protobuf.Any tx = 11; string timestamp = 12; repeated tendermint.abci.Event events = 13; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-abci-v1beta1-txresponse-j39</code> | notes | Go type TxResponse at sdk/types/abci.pb.go:32 |

## proto-cosmos-base-kv-v1beta1-pair-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-kv-v1beta1-pair-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/kv/v1beta1/kv.proto:14</code> | <code>message cosmos.base.kv.v1beta1.Pair { bytes key = 1; bytes value = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-kv-v1beta1-pair-j39</code> | notes | Go type Pair at sdk/types/kv/kv.pb.go:72; 32 referencing files, first 25 listed |

## proto-cosmos-base-kv-v1beta1-pairs-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-kv-v1beta1-pairs-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/kv/v1beta1/kv.proto:9</code> | <code>message cosmos.base.kv.v1beta1.Pairs { repeated Pair pairs = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-kv-v1beta1-pairs-j39</code> | notes | Go type Pairs at sdk/types/kv/kv.pb.go:27 |

## proto-cosmos-base-query-v1beta1-pagerequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-query-v1beta1-pagerequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/query/v1beta1/pagination.proto:13</code> | <code>message cosmos.base.query.v1beta1.PageRequest { bytes key = 1; uint64 offset = 2; uint64 limit = 3; bool count_total = 4; bool reverse = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-query-v1beta1-pagerequest-j39</code> | notes | Go type PageRequest at sdk/types/query/pagination.pb.go:32; 29 referencing files, first 25 listed |

## proto-cosmos-base-query-v1beta1-pageresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-query-v1beta1-pageresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/query/v1beta1/pagination.proto:47</code> | <code>message cosmos.base.query.v1beta1.PageResponse { bytes next_key = 1; uint64 total = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-query-v1beta1-pageresponse-j39</code> | notes | Go type PageResponse at sdk/types/query/pagination.pb.go:130 |

## proto-cosmos-base-reflection-v1beta1-listallinterfacesrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listallinterfacesrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:26</code> | <code>message cosmos.base.reflection.v1beta1.ListAllInterfacesRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listallinterfacesrequest-j39</code> | notes | Go type ListAllInterfacesRequest at sdk/client/grpc/reflection/reflection.pb.go:32 |

## proto-cosmos-base-reflection-v1beta1-listallinterfacesresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listallinterfacesresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:29</code> | <code>message cosmos.base.reflection.v1beta1.ListAllInterfacesResponse { repeated string interface_names = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listallinterfacesresponse-j39</code> | notes | Go type ListAllInterfacesResponse at sdk/client/grpc/reflection/reflection.pb.go:69 |

## proto-cosmos-base-reflection-v1beta1-listimplementationsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listimplementationsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:36</code> | <code>message cosmos.base.reflection.v1beta1.ListImplementationsRequest { string interface_name = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listimplementationsrequest-j39</code> | notes | Go type ListImplementationsRequest at sdk/client/grpc/reflection/reflection.pb.go:116 |

## proto-cosmos-base-reflection-v1beta1-listimplementationsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listimplementationsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:43</code> | <code>message cosmos.base.reflection.v1beta1.ListImplementationsResponse { repeated string implementation_message_names = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v1beta1-listimplementationsresponse-j39</code> | notes | Go type ListImplementationsResponse at sdk/client/grpc/reflection/reflection.pb.go:163 |

## proto-cosmos-base-reflection-v2alpha1-appdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-appdescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:10</code> | <code>message cosmos.base.reflection.v2alpha1.AppDescriptor { AuthnDescriptor authn = 1; ChainDescriptor chain = 2; CodecDescriptor codec = 3; ConfigurationDescriptor configuration = 4; QueryServicesDescriptor query_services = 5; TxDescriptor tx = 6; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-appdescriptor-j39</code> | notes | Go type AppDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:32 |

## proto-cosmos-base-reflection-v2alpha1-authndescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-authndescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:38</code> | <code>message cosmos.base.reflection.v2alpha1.AuthnDescriptor { repeated SigningModeDescriptor sign_modes = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-authndescriptor-j39</code> | notes | Go type AuthnDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:182 |

## proto-cosmos-base-reflection-v2alpha1-chaindescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-chaindescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:58</code> | <code>message cosmos.base.reflection.v2alpha1.ChainDescriptor { string id = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-chaindescriptor-j39</code> | notes | Go type ChainDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:296 |

## proto-cosmos-base-reflection-v2alpha1-codecdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-codecdescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:64</code> | <code>message cosmos.base.reflection.v2alpha1.CodecDescriptor { repeated InterfaceDescriptor interfaces = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-codecdescriptor-j39</code> | notes | Go type CodecDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:342 |

## proto-cosmos-base-reflection-v2alpha1-configurationdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-configurationdescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:103</code> | <code>message cosmos.base.reflection.v2alpha1.ConfigurationDescriptor { string bech32_account_address_prefix = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-configurationdescriptor-j39</code> | notes | Go type ConfigurationDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:569 |

## proto-cosmos-base-reflection-v2alpha1-getauthndescriptorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getauthndescriptorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:145</code> | <code>message cosmos.base.reflection.v2alpha1.GetAuthnDescriptorRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getauthndescriptorrequest-j39</code> | notes | Go type GetAuthnDescriptorRequest at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:661 |

## proto-cosmos-base-reflection-v2alpha1-getauthndescriptorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getauthndescriptorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:148</code> | <code>message cosmos.base.reflection.v2alpha1.GetAuthnDescriptorResponse { AuthnDescriptor authn = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getauthndescriptorresponse-j39</code> | notes | Go type GetAuthnDescriptorResponse at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:698 |

## proto-cosmos-base-reflection-v2alpha1-getchaindescriptorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getchaindescriptorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:154</code> | <code>message cosmos.base.reflection.v2alpha1.GetChainDescriptorRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getchaindescriptorrequest-j39</code> | notes | Go type GetChainDescriptorRequest at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:744 |

## proto-cosmos-base-reflection-v2alpha1-getchaindescriptorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getchaindescriptorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:157</code> | <code>message cosmos.base.reflection.v2alpha1.GetChainDescriptorResponse { ChainDescriptor chain = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getchaindescriptorresponse-j39</code> | notes | Go type GetChainDescriptorResponse at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:781 |

## proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:163</code> | <code>message cosmos.base.reflection.v2alpha1.GetCodecDescriptorRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorrequest-j39</code> | notes | Go type GetCodecDescriptorRequest at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:827 |

## proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:166</code> | <code>message cosmos.base.reflection.v2alpha1.GetCodecDescriptorResponse { CodecDescriptor codec = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getcodecdescriptorresponse-j39</code> | notes | Go type GetCodecDescriptorResponse at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:864 |

## proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:172</code> | <code>message cosmos.base.reflection.v2alpha1.GetConfigurationDescriptorRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorrequest-j39</code> | notes | Go type GetConfigurationDescriptorRequest at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:910 |

## proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:175</code> | <code>message cosmos.base.reflection.v2alpha1.GetConfigurationDescriptorResponse { ConfigurationDescriptor config = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getconfigurationdescriptorresponse-j39</code> | notes | Go type GetConfigurationDescriptorResponse at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:947 |

## proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:181</code> | <code>message cosmos.base.reflection.v2alpha1.GetQueryServicesDescriptorRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorrequest-j39</code> | notes | Go type GetQueryServicesDescriptorRequest at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:993 |

## proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:184</code> | <code>message cosmos.base.reflection.v2alpha1.GetQueryServicesDescriptorResponse { QueryServicesDescriptor queries = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-getqueryservicesdescriptorresponse-j39</code> | notes | Go type GetQueryServicesDescriptorResponse at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:1030 |

## proto-cosmos-base-reflection-v2alpha1-gettxdescriptorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-gettxdescriptorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:190</code> | <code>message cosmos.base.reflection.v2alpha1.GetTxDescriptorRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-gettxdescriptorrequest-j39</code> | notes | Go type GetTxDescriptorRequest at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:1076 |

## proto-cosmos-base-reflection-v2alpha1-gettxdescriptorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-gettxdescriptorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:193</code> | <code>message cosmos.base.reflection.v2alpha1.GetTxDescriptorResponse { TxDescriptor tx = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-gettxdescriptorresponse-j39</code> | notes | Go type GetTxDescriptorResponse at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:1113 |

## proto-cosmos-base-reflection-v2alpha1-interfaceacceptingmessagedescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-interfaceacceptingmessagedescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:93</code> | <code>message cosmos.base.reflection.v2alpha1.InterfaceAcceptingMessageDescriptor { string fullname = 1; repeated string field_descriptor_names = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-interfaceacceptingmessagedescriptor-j39</code> | notes | Go type InterfaceAcceptingMessageDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:512; no in-repo reference outside the generated package |

## proto-cosmos-base-reflection-v2alpha1-interfacedescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-interfacedescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:70</code> | <code>message cosmos.base.reflection.v2alpha1.InterfaceDescriptor { string fullname = 1; repeated InterfaceAcceptingMessageDescriptor interface_accepting_messages = 2; repeated InterfaceImplementerDescriptor interface_implementers = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-interfacedescriptor-j39</code> | notes | Go type InterfaceDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:388 |

## proto-cosmos-base-reflection-v2alpha1-interfaceimplementerdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-interfaceimplementerdescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:81</code> | <code>message cosmos.base.reflection.v2alpha1.InterfaceImplementerDescriptor { string fullname = 1; string type_url = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-interfaceimplementerdescriptor-j39</code> | notes | Go type InterfaceImplementerDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:453 |

## proto-cosmos-base-reflection-v2alpha1-msgdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-msgdescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:109</code> | <code>message cosmos.base.reflection.v2alpha1.MsgDescriptor { string msg_type_url = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-msgdescriptor-j39</code> | notes | Go type MsgDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:615 |

## proto-cosmos-base-reflection-v2alpha1-querymethoddescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-querymethoddescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:218</code> | <code>message cosmos.base.reflection.v2alpha1.QueryMethodDescriptor { string name = 1; string full_query_path = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-querymethoddescriptor-j39</code> | notes | Go type QueryMethodDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:1272 |

## proto-cosmos-base-reflection-v2alpha1-queryservicedescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-queryservicedescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:206</code> | <code>message cosmos.base.reflection.v2alpha1.QueryServiceDescriptor { string fullname = 1; bool is_module = 2; repeated QueryMethodDescriptor methods = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-queryservicedescriptor-j39</code> | notes | Go type QueryServiceDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:1206 |

## proto-cosmos-base-reflection-v2alpha1-queryservicesdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-queryservicesdescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:200</code> | <code>message cosmos.base.reflection.v2alpha1.QueryServicesDescriptor { repeated QueryServiceDescriptor query_services = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-queryservicesdescriptor-j39</code> | notes | Go type QueryServicesDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:1160 |

## proto-cosmos-base-reflection-v2alpha1-signingmodedescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-signingmodedescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:47</code> | <code>message cosmos.base.reflection.v2alpha1.SigningModeDescriptor { string name = 1; int32 number = 2; string authn_info_provider_method_fullname = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-signingmodedescriptor-j39</code> | notes | Go type SigningModeDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:231 |

## proto-cosmos-base-reflection-v2alpha1-txdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-txdescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:27</code> | <code>message cosmos.base.reflection.v2alpha1.TxDescriptor { string fullname = 1; repeated MsgDescriptor msgs = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-reflection-v2alpha1-txdescriptor-j39</code> | notes | Go type TxDescriptor at sdk/server/grpc/reflection/v2alpha1/reflection.pb.go:124 |

## proto-cosmos-base-snapshots-v1beta1-metadata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-metadata-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto:18</code> | <code>message cosmos.base.snapshots.v1beta1.Metadata { repeated bytes chunk_hashes = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-metadata-j39</code> | notes | Go type Metadata at sdk/snapshots/types/snapshot.pb.go:104 |

## proto-cosmos-base-snapshots-v1beta1-snapshot-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshot-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto:9</code> | <code>message cosmos.base.snapshots.v1beta1.Snapshot { uint64 height = 1; uint32 format = 2; uint32 chunks = 3; bytes hash = 4; Metadata metadata = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshot-j39</code> | notes | Go type Snapshot at sdk/snapshots/types/snapshot.pb.go:27 |

## proto-cosmos-base-snapshots-v1beta1-snapshotextensionmeta-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotextensionmeta-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto:49</code> | <code>message cosmos.base.snapshots.v1beta1.SnapshotExtensionMeta { string name = 1; uint32 format = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotextensionmeta-j39</code> | notes | Go type SnapshotExtensionMeta at sdk/snapshots/types/snapshot.pb.go:380 |

## proto-cosmos-base-snapshots-v1beta1-snapshotextensionpayload-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotextensionpayload-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto:55</code> | <code>message cosmos.base.snapshots.v1beta1.SnapshotExtensionPayload { bytes payload = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotextensionpayload-j39</code> | notes | Go type SnapshotExtensionPayload at sdk/snapshots/types/snapshot.pb.go:433 |

## proto-cosmos-base-snapshots-v1beta1-snapshotiavlitem-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotiavlitem-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto:39</code> | <code>message cosmos.base.snapshots.v1beta1.SnapshotIAVLItem { bytes key = 1; bytes value = 2; int64 version = 3; int32 height = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotiavlitem-j39</code> | notes | Go type SnapshotIAVLItem at sdk/snapshots/types/snapshot.pb.go:309 |

## proto-cosmos-base-snapshots-v1beta1-snapshotitem-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotitem-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto:23</code> | <code>message cosmos.base.snapshots.v1beta1.SnapshotItem { SnapshotStoreItem store = 1; SnapshotIAVLItem iavl = 2; SnapshotExtensionMeta extension = 3; SnapshotExtensionPayload extension_payload = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotitem-j39</code> | notes | Go type SnapshotItem at sdk/snapshots/types/snapshot.pb.go:149 |

## proto-cosmos-base-snapshots-v1beta1-snapshotstoreitem-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotstoreitem-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/snapshots/v1beta1/snapshot.proto:34</code> | <code>message cosmos.base.snapshots.v1beta1.SnapshotStoreItem { string name = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-snapshots-v1beta1-snapshotstoreitem-j39</code> | notes | Go type SnapshotStoreItem at sdk/snapshots/types/snapshot.pb.go:264 |

## proto-cosmos-base-store-v1beta1-commitid-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-commitid-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/store/v1beta1/commit_info.proto:24</code> | <code>message cosmos.base.store.v1beta1.CommitID { int64 version = 1; bytes hash = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-commitid-j39</code> | notes | Go type CommitID at sdk/store/types/commit_info.pb.go:136 |

## proto-cosmos-base-store-v1beta1-commitinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-commitinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/store/v1beta1/commit_info.proto:10</code> | <code>message cosmos.base.store.v1beta1.CommitInfo { int64 version = 1; repeated StoreInfo store_infos = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-commitinfo-j39</code> | notes | Go type CommitInfo at sdk/store/types/commit_info.pb.go:28 |

## proto-cosmos-base-store-v1beta1-storeinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-storeinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/store/v1beta1/commit_info.proto:17</code> | <code>message cosmos.base.store.v1beta1.StoreInfo { string name = 1; CommitID commit_id = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-storeinfo-j39</code> | notes | Go type StoreInfo at sdk/store/types/commit_info.pb.go:82 |

## proto-cosmos-base-store-v1beta1-storekvpair-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-storekvpair-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/store/v1beta1/listening.proto:11</code> | <code>message cosmos.base.store.v1beta1.StoreKVPair { string store_key = 1; bool delete = 2; bytes key = 3; bytes value = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-store-v1beta1-storekvpair-j39</code> | notes | Go type StoreKVPair at sdk/store/types/listening.pb.go:30; no in-repo reference outside the generated package |

## proto-cosmos-base-tendermint-v1beta1-getblockbyheightrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getblockbyheightrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:80</code> | <code>message cosmos.base.tendermint.v1beta1.GetBlockByHeightRequest { int64 height = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getblockbyheightrequest-j39</code> | notes | Go type GetBlockByHeightRequest at sdk/client/grpc/tmservice/query.pb.go:329 |

## proto-cosmos-base-tendermint-v1beta1-getblockbyheightresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getblockbyheightresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:85</code> | <code>message cosmos.base.tendermint.v1beta1.GetBlockByHeightResponse { tendermint.types.BlockID block_id = 1; tendermint.types.Block block = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getblockbyheightresponse-j39</code> | notes | Go type GetBlockByHeightResponse at sdk/client/grpc/tmservice/query.pb.go:374 |

## proto-cosmos-base-tendermint-v1beta1-getlatestblockrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestblockrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:91</code> | <code>message cosmos.base.tendermint.v1beta1.GetLatestBlockRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestblockrequest-j39</code> | notes | Go type GetLatestBlockRequest at sdk/client/grpc/tmservice/query.pb.go:427 |

## proto-cosmos-base-tendermint-v1beta1-getlatestblockresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestblockresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:94</code> | <code>message cosmos.base.tendermint.v1beta1.GetLatestBlockResponse { tendermint.types.BlockID block_id = 1; tendermint.types.Block block = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestblockresponse-j39</code> | notes | Go type GetLatestBlockResponse at sdk/client/grpc/tmservice/query.pb.go:464 |

## proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:58</code> | <code>message cosmos.base.tendermint.v1beta1.GetLatestValidatorSetRequest { cosmos.base.query.v1beta1.PageRequest pagination = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetrequest-j39</code> | notes | Go type GetLatestValidatorSetRequest at sdk/client/grpc/tmservice/query.pb.go:152 |

## proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:64</code> | <code>message cosmos.base.tendermint.v1beta1.GetLatestValidatorSetResponse { int64 block_height = 1; repeated Validator validators = 2; cosmos.base.query.v1beta1.PageResponse pagination = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getlatestvalidatorsetresponse-j39</code> | notes | Go type GetLatestValidatorSetResponse at sdk/client/grpc/tmservice/query.pb.go:198 |

## proto-cosmos-base-tendermint-v1beta1-getnodeinforequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getnodeinforequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:108</code> | <code>message cosmos.base.tendermint.v1beta1.GetNodeInfoRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getnodeinforequest-j39</code> | notes | Go type GetNodeInfoRequest at sdk/client/grpc/tmservice/query.pb.go:599 |

## proto-cosmos-base-tendermint-v1beta1-getnodeinforesponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getnodeinforesponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:111</code> | <code>message cosmos.base.tendermint.v1beta1.GetNodeInfoResponse { tendermint.p2p.NodeInfo default_node_info = 1; VersionInfo application_version = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getnodeinforesponse-j39</code> | notes | Go type GetNodeInfoResponse at sdk/client/grpc/tmservice/query.pb.go:636 |

## proto-cosmos-base-tendermint-v1beta1-getsyncingrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getsyncingrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:100</code> | <code>message cosmos.base.tendermint.v1beta1.GetSyncingRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getsyncingrequest-j39</code> | notes | Go type GetSyncingRequest at sdk/client/grpc/tmservice/query.pb.go:517 |

## proto-cosmos-base-tendermint-v1beta1-getsyncingresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getsyncingresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:103</code> | <code>message cosmos.base.tendermint.v1beta1.GetSyncingResponse { bool syncing = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getsyncingresponse-j39</code> | notes | Go type GetSyncingResponse at sdk/client/grpc/tmservice/query.pb.go:554 |

## proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:43</code> | <code>message cosmos.base.tendermint.v1beta1.GetValidatorSetByHeightRequest { int64 height = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightrequest-j39</code> | notes | Go type GetValidatorSetByHeightRequest at sdk/client/grpc/tmservice/query.pb.go:36 |

## proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:50</code> | <code>message cosmos.base.tendermint.v1beta1.GetValidatorSetByHeightResponse { int64 block_height = 1; repeated Validator validators = 2; cosmos.base.query.v1beta1.PageResponse pagination = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-getvalidatorsetbyheightresponse-j39</code> | notes | Go type GetValidatorSetByHeightResponse at sdk/client/grpc/tmservice/query.pb.go:90 |

## proto-cosmos-base-tendermint-v1beta1-module-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-module-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:130</code> | <code>message cosmos.base.tendermint.v1beta1.Module { string path = 1; string version = 2; string sum = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-module-j39</code> | notes | Go type Module at sdk/client/grpc/tmservice/query.pb.go:791 |

## proto-cosmos-base-tendermint-v1beta1-validator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-validator-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:72</code> | <code>message cosmos.base.tendermint.v1beta1.Validator { string address = 1; google.protobuf.Any pub_key = 2; int64 voting_power = 3; int64 proposer_priority = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-validator-j39</code> | notes | Go type Validator at sdk/client/grpc/tmservice/query.pb.go:260 |

## proto-cosmos-base-tendermint-v1beta1-versioninfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-versioninfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:117</code> | <code>message cosmos.base.tendermint.v1beta1.VersionInfo { string name = 1; string app_name = 2; string version = 3; string git_commit = 4; string build_tags = 5; string go_version = 6; repeated Module build_deps = 7; string cosmos_sdk_version = 8; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-tendermint-v1beta1-versioninfo-j39</code> | notes | Go type VersionInfo at sdk/client/grpc/tmservice/query.pb.go:689 |

## proto-cosmos-base-v1beta1-coin-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-coin-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/v1beta1/coin.proto:14</code> | <code>message cosmos.base.v1beta1.Coin { string denom = 1; string amount = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-coin-j39</code> | notes | Go type Coin at sdk/types/coin.pb.go:30; 147 referencing files, first 25 listed |

## proto-cosmos-base-v1beta1-deccoin-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-deccoin-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/v1beta1/coin.proto:28</code> | <code>message cosmos.base.v1beta1.DecCoin { string denom = 1; string amount = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-deccoin-j39</code> | notes | Go type DecCoin at sdk/types/coin.pb.go:78 |

## proto-cosmos-base-v1beta1-decproto-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-decproto-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/v1beta1/coin.proto:47</code> | <code>message cosmos.base.v1beta1.DecProto { string dec = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-decproto-j39</code> | notes | Go type DecProto at sdk/types/coin.pb.go:160 |

## proto-cosmos-base-v1beta1-intproto-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-intproto-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/v1beta1/coin.proto:39</code> | <code>message cosmos.base.v1beta1.IntProto { string int = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-base-v1beta1-intproto-j39</code> | notes | Go type IntProto at sdk/types/coin.pb.go:123 |

## proto-cosmos-capability-v1beta1-capability-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-capability-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/capability/v1beta1/capability.proto:10</code> | <code>message cosmos.capability.v1beta1.Capability { uint64 index = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-capability-j39</code> | notes | Go type Capability at sdk/x/capability/types/capability.pb.go:28; 43 referencing files, first 25 listed |

## proto-cosmos-capability-v1beta1-capabilityowners-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-capabilityowners-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/capability/v1beta1/capability.proto:28</code> | <code>message cosmos.capability.v1beta1.CapabilityOwners { repeated Owner owners = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-capabilityowners-j39</code> | notes | Go type CapabilityOwners at sdk/x/capability/types/capability.pb.go:112 |

## proto-cosmos-capability-v1beta1-genesisowners-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-genesisowners-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/capability/v1beta1/genesis.proto:10</code> | <code>message cosmos.capability.v1beta1.GenesisOwners { uint64 index = 1; CapabilityOwners index_owners = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-genesisowners-j39</code> | notes | Go type GenesisOwners at sdk/x/capability/types/genesis.pb.go:27 |

## proto-cosmos-capability-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/capability/v1beta1/genesis.proto:22</code> | <code>message cosmos.capability.v1beta1.GenesisState { uint64 index = 1; repeated GenesisOwners owners = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/capability/types/genesis.pb.go:82 |

## proto-cosmos-capability-v1beta1-owner-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-owner-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/capability/v1beta1/capability.proto:18</code> | <code>message cosmos.capability.v1beta1.Owner { string module = 1; string name = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-capability-v1beta1-owner-j39</code> | notes | Go type Owner at sdk/x/capability/types/capability.pb.go:73 |

## proto-cosmos-crypto-ed25519-privkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-ed25519-privkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/ed25519/keys.proto:21</code> | <code>message cosmos.crypto.ed25519.PrivKey { bytes key = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-ed25519-privkey-j39</code> | notes | Go type PrivKey at sdk/crypto/keys/ed25519/keys.pb.go:77 |

## proto-cosmos-crypto-ed25519-pubkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-ed25519-pubkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/ed25519/keys.proto:13</code> | <code>message cosmos.crypto.ed25519.PubKey { bytes key = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-ed25519-pubkey-j39</code> | notes | Go type PubKey at sdk/crypto/keys/ed25519/keys.pb.go:32; 27 referencing files, first 25 listed |

## proto-cosmos-crypto-multisig-legacyaminopubkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-multisig-legacyaminopubkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/multisig/keys.proto:12</code> | <code>message cosmos.crypto.multisig.LegacyAminoPubKey { uint32 threshold = 1; repeated google.protobuf.Any public_keys = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-multisig-legacyaminopubkey-j39</code> | notes | Go type LegacyAminoPubKey at sdk/crypto/keys/multisig/keys.pb.go:30 |

## proto-cosmos-crypto-multisig-v1beta1-compactbitarray-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-multisig-v1beta1-compactbitarray-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/multisig/v1beta1/multisig.proto:20</code> | <code>message cosmos.crypto.multisig.v1beta1.CompactBitArray { uint32 extra_bits_stored = 1; bytes elems = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-multisig-v1beta1-compactbitarray-j39</code> | notes | Go type CompactBitArray at sdk/crypto/types/multisig.pb.go:78 |

## proto-cosmos-crypto-multisig-v1beta1-multisignature-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-multisig-v1beta1-multisignature-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/multisig/v1beta1/multisig.proto:11</code> | <code>message cosmos.crypto.multisig.v1beta1.MultiSignature { repeated bytes signatures = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-multisig-v1beta1-multisignature-j39</code> | notes | Go type MultiSignature at sdk/crypto/types/multisig.pb.go:29 |

## proto-cosmos-crypto-secp256k1-privkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256k1-privkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/secp256k1/keys.proto:20</code> | <code>message cosmos.crypto.secp256k1.PrivKey { bytes key = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256k1-privkey-j39</code> | notes | Go type PrivKey at sdk/crypto/keys/secp256k1/keys.pb.go:75 |

## proto-cosmos-crypto-secp256k1-pubkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256k1-pubkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/secp256k1/keys.proto:13</code> | <code>message cosmos.crypto.secp256k1.PubKey { bytes key = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256k1-pubkey-j39</code> | notes | Go type PubKey at sdk/crypto/keys/secp256k1/keys.pb.go:31; 34 referencing files, first 25 listed |

## proto-cosmos-crypto-secp256r1-privkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256r1-privkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/secp256r1/keys.proto:20</code> | <code>message cosmos.crypto.secp256r1.PrivKey { bytes secret = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256r1-privkey-j39</code> | notes | Go type PrivKey at sdk/crypto/keys/secp256r1/keys.pb.go:70 |

## proto-cosmos-crypto-secp256r1-pubkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256r1-pubkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/secp256r1/keys.proto:13</code> | <code>message cosmos.crypto.secp256r1.PubKey { bytes key = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-secp256r1-pubkey-j39</code> | notes | Go type PubKey at sdk/crypto/keys/secp256r1/keys.pb.go:27 |

## proto-cosmos-crypto-sr25519-pubkey-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-crypto-sr25519-pubkey-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/crypto/sr25519/keys.proto:12</code> | <code>message cosmos.crypto.sr25519.PubKey { bytes key = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-crypto-sr25519-pubkey-j39</code> | notes | Go type PubKey at sdk/crypto/keys/sr25519/keys.pb.go:28 |

## proto-cosmos-distribution-v1beta1-communitypoolspendproposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-communitypoolspendproposal-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:115</code> | <code>message cosmos.distribution.v1beta1.CommunityPoolSpendProposal { string title = 1; string description = 2; string recipient = 3; repeated cosmos.base.v1beta1.Coin amount = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-communitypoolspendproposal-j39</code> | notes | Go type CommunityPoolSpendProposal at sdk/x/distribution/types/distribution.pb.go:428 |

## proto-cosmos-distribution-v1beta1-communitypoolspendproposalwithdeposit-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-communitypoolspendproposalwithdeposit-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:164</code> | <code>message cosmos.distribution.v1beta1.CommunityPoolSpendProposalWithDeposit { string title = 1; string description = 2; string recipient = 3; string amount = 4; string deposit = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-communitypoolspendproposalwithdeposit-j39</code> | notes | Go type CommunityPoolSpendProposalWithDeposit at sdk/x/distribution/types/distribution.pb.go:568 |

## proto-cosmos-distribution-v1beta1-delegationdelegatorreward-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegationdelegatorreward-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:150</code> | <code>message cosmos.distribution.v1beta1.DelegationDelegatorReward { string validator_address = 1; repeated cosmos.base.v1beta1.DecCoin reward = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegationdelegatorreward-j39</code> | notes | Go type DelegationDelegatorReward at sdk/x/distribution/types/distribution.pb.go:528 |

## proto-cosmos-distribution-v1beta1-delegatorstartinginfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegatorstartinginfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:135</code> | <code>message cosmos.distribution.v1beta1.DelegatorStartingInfo { uint64 previous_period = 1; string stake = 2; uint64 height = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegatorstartinginfo-j39</code> | notes | Go type DelegatorStartingInfo at sdk/x/distribution/types/distribution.pb.go:473 |

## proto-cosmos-distribution-v1beta1-delegatorstartinginforecord-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegatorstartinginforecord-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:92</code> | <code>message cosmos.distribution.v1beta1.DelegatorStartingInfoRecord { string delegator_address = 1; string validator_address = 2; DelegatorStartingInfo starting_info = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegatorstartinginforecord-j39</code> | notes | Go type DelegatorStartingInfoRecord at sdk/x/distribution/types/genesis.pb.go:240 |

## proto-cosmos-distribution-v1beta1-delegatorwithdrawinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegatorwithdrawinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:14</code> | <code>message cosmos.distribution.v1beta1.DelegatorWithdrawInfo { string delegator_address = 1; string withdraw_address = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-delegatorwithdrawinfo-j39</code> | notes | Go type DelegatorWithdrawInfo at sdk/x/distribution/types/genesis.pb.go:31 |

## proto-cosmos-distribution-v1beta1-feepool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-feepool-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:104</code> | <code>message cosmos.distribution.v1beta1.FeePool { repeated cosmos.base.v1beta1.DecCoin community_pool = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-feepool-j39</code> | notes | Go type FeePool at sdk/x/distribution/types/distribution.pb.go:381 |

## proto-cosmos-distribution-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:128</code> | <code>message cosmos.distribution.v1beta1.GenesisState { Params params = 1; FeePool fee_pool = 2; repeated DelegatorWithdrawInfo delegator_withdraw_infos = 3; string previous_proposer = 4; repeated ValidatorOutstandingRewardsRecord outstanding_rewards = 5; repeated ValidatorAccumulatedCommissionRecord validator_accumulated_commissions = 6; repeated ValidatorHistoricalRewardsRecord validator_historical_rewards = 7; repeated ValidatorCurrentRewardsRecord validator_current_rewards = 8; repeated DelegatorStartingInfoRecord delegator_starting_infos = 9; repeated ValidatorSlashEventRecord validator_slash_events = 10; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/distribution/types/genesis.pb.go:328 |

## proto-cosmos-distribution-v1beta1-msgfundcommunitypool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgfundcommunitypool-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:69</code> | <code>message cosmos.distribution.v1beta1.MsgFundCommunityPool { repeated cosmos.base.v1beta1.Coin amount = 1; string depositor = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgfundcommunitypool-j39</code> | notes | Go type MsgFundCommunityPool at sdk/x/distribution/types/tx.pb.go:267 |

## proto-cosmos-distribution-v1beta1-msgfundcommunitypoolresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgfundcommunitypoolresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:81</code> | <code>message cosmos.distribution.v1beta1.MsgFundCommunityPoolResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgfundcommunitypoolresponse-j39</code> | notes | Go type MsgFundCommunityPoolResponse at sdk/x/distribution/types/tx.pb.go:306 |

## proto-cosmos-distribution-v1beta1-msgsetwithdrawaddress-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgsetwithdrawaddress-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:31</code> | <code>message cosmos.distribution.v1beta1.MsgSetWithdrawAddress { string delegator_address = 1; string withdraw_address = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgsetwithdrawaddress-j39</code> | notes | Go type MsgSetWithdrawAddress at sdk/x/distribution/types/tx.pb.go:35 |

## proto-cosmos-distribution-v1beta1-msgsetwithdrawaddressresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgsetwithdrawaddressresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:40</code> | <code>message cosmos.distribution.v1beta1.MsgSetWithdrawAddressResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgsetwithdrawaddressresponse-j39</code> | notes | Go type MsgSetWithdrawAddressResponse at sdk/x/distribution/types/tx.pb.go:74 |

## proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorreward-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorreward-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:44</code> | <code>message cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward { string delegator_address = 1; string validator_address = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorreward-j39</code> | notes | Go type MsgWithdrawDelegatorReward at sdk/x/distribution/types/tx.pb.go:112 |

## proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorrewardresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorrewardresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:53</code> | <code>message cosmos.distribution.v1beta1.MsgWithdrawDelegatorRewardResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawdelegatorrewardresponse-j39</code> | notes | Go type MsgWithdrawDelegatorRewardResponse at sdk/x/distribution/types/tx.pb.go:151 |

## proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommission-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommission-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:57</code> | <code>message cosmos.distribution.v1beta1.MsgWithdrawValidatorCommission { string validator_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommission-j39</code> | notes | Go type MsgWithdrawValidatorCommission at sdk/x/distribution/types/tx.pb.go:189 |

## proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommissionresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommissionresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/tx.proto:65</code> | <code>message cosmos.distribution.v1beta1.MsgWithdrawValidatorCommissionResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-msgwithdrawvalidatorcommissionresponse-j39</code> | notes | Go type MsgWithdrawValidatorCommissionResponse at sdk/x/distribution/types/tx.pb.go:227 |

## proto-cosmos-distribution-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-params-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:11</code> | <code>message cosmos.distribution.v1beta1.Params { string community_tax = 1; string base_proposer_reward = 2; string bonus_proposer_reward = 3; bool withdraw_addr_enabled = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-params-j39</code> | notes | Go type Params at sdk/x/distribution/types/distribution.pb.go:29 |

## proto-cosmos-distribution-v1beta1-querycommunitypoolrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querycommunitypoolrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:218</code> | <code>message cosmos.distribution.v1beta1.QueryCommunityPoolRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querycommunitypoolrequest-j39</code> | notes | Go type QueryCommunityPoolRequest at sdk/x/distribution/types/query.pb.go:758 |

## proto-cosmos-distribution-v1beta1-querycommunitypoolresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querycommunitypoolresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:222</code> | <code>message cosmos.distribution.v1beta1.QueryCommunityPoolResponse { repeated cosmos.base.v1beta1.DecCoin pool = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querycommunitypoolresponse-j39</code> | notes | Go type QueryCommunityPoolResponse at sdk/x/distribution/types/query.pb.go:796 |

## proto-cosmos-distribution-v1beta1-querydelegationrewardsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationrewardsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:135</code> | <code>message cosmos.distribution.v1beta1.QueryDelegationRewardsRequest { string delegator_address = 1; string validator_address = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationrewardsrequest-j39</code> | notes | Go type QueryDelegationRewardsRequest at sdk/x/distribution/types/query.pb.go:413 |

## proto-cosmos-distribution-v1beta1-querydelegationrewardsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationrewardsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:147</code> | <code>message cosmos.distribution.v1beta1.QueryDelegationRewardsResponse { repeated cosmos.base.v1beta1.DecCoin rewards = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationrewardsresponse-j39</code> | notes | Go type QueryDelegationRewardsResponse at sdk/x/distribution/types/query.pb.go:455 |

## proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:157</code> | <code>message cosmos.distribution.v1beta1.QueryDelegationTotalRewardsRequest { string delegator_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsrequest-j39</code> | notes | Go type QueryDelegationTotalRewardsRequest at sdk/x/distribution/types/query.pb.go:502 |

## proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:166</code> | <code>message cosmos.distribution.v1beta1.QueryDelegationTotalRewardsResponse { repeated DelegationDelegatorReward rewards = 1; repeated cosmos.base.v1beta1.DecCoin total = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegationtotalrewardsresponse-j39</code> | notes | Go type QueryDelegationTotalRewardsResponse at sdk/x/distribution/types/query.pb.go:542 |

## proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:178</code> | <code>message cosmos.distribution.v1beta1.QueryDelegatorValidatorsRequest { string delegator_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsrequest-j39</code> | notes | Go type QueryDelegatorValidatorsRequest at sdk/x/distribution/types/query.pb.go:598 |

## proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:188</code> | <code>message cosmos.distribution.v1beta1.QueryDelegatorValidatorsResponse { repeated string validators = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorvalidatorsresponse-j39</code> | notes | Go type QueryDelegatorValidatorsResponse at sdk/x/distribution/types/query.pb.go:638 |

## proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:198</code> | <code>message cosmos.distribution.v1beta1.QueryDelegatorWithdrawAddressRequest { string delegator_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressrequest-j39</code> | notes | Go type QueryDelegatorWithdrawAddressRequest at sdk/x/distribution/types/query.pb.go:678 |

## proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:208</code> | <code>message cosmos.distribution.v1beta1.QueryDelegatorWithdrawAddressResponse { string withdraw_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-querydelegatorwithdrawaddressresponse-j39</code> | notes | Go type QueryDelegatorWithdrawAddressResponse at sdk/x/distribution/types/query.pb.go:718 |

## proto-cosmos-distribution-v1beta1-queryparamsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryparamsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:72</code> | <code>message cosmos.distribution.v1beta1.QueryParamsRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryparamsrequest-j39</code> | notes | Go type QueryParamsRequest at sdk/x/distribution/types/query.pb.go:36 |

## proto-cosmos-distribution-v1beta1-queryparamsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryparamsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:75</code> | <code>message cosmos.distribution.v1beta1.QueryParamsResponse { Params params = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryparamsresponse-j39</code> | notes | Go type QueryParamsResponse at sdk/x/distribution/types/query.pb.go:73 |

## proto-cosmos-distribution-v1beta1-queryvalidatorcommissionrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorcommissionrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:95</code> | <code>message cosmos.distribution.v1beta1.QueryValidatorCommissionRequest { string validator_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorcommissionrequest-j39</code> | notes | Go type QueryValidatorCommissionRequest at sdk/x/distribution/types/query.pb.go:217 |

## proto-cosmos-distribution-v1beta1-queryvalidatorcommissionresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorcommissionresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:102</code> | <code>message cosmos.distribution.v1beta1.QueryValidatorCommissionResponse { ValidatorAccumulatedCommission commission = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorcommissionresponse-j39</code> | notes | Go type QueryValidatorCommissionResponse at sdk/x/distribution/types/query.pb.go:264 |

## proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:82</code> | <code>message cosmos.distribution.v1beta1.QueryValidatorOutstandingRewardsRequest { string validator_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsrequest-j39</code> | notes | Go type QueryValidatorOutstandingRewardsRequest at sdk/x/distribution/types/query.pb.go:120 |

## proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:89</code> | <code>message cosmos.distribution.v1beta1.QueryValidatorOutstandingRewardsResponse { ValidatorOutstandingRewards rewards = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatoroutstandingrewardsresponse-j39</code> | notes | Go type QueryValidatorOutstandingRewardsResponse at sdk/x/distribution/types/query.pb.go:169 |

## proto-cosmos-distribution-v1beta1-queryvalidatorslashesrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorslashesrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:109</code> | <code>message cosmos.distribution.v1beta1.QueryValidatorSlashesRequest { string validator_address = 1; uint64 starting_height = 2; uint64 ending_height = 3; cosmos.base.query.v1beta1.PageRequest pagination = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorslashesrequest-j39</code> | notes | Go type QueryValidatorSlashesRequest at sdk/x/distribution/types/query.pb.go:311 |

## proto-cosmos-distribution-v1beta1-queryvalidatorslashesresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorslashesresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:125</code> | <code>message cosmos.distribution.v1beta1.QueryValidatorSlashesResponse { repeated ValidatorSlashEvent slashes = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-queryvalidatorslashesresponse-j39</code> | notes | Go type QueryValidatorSlashesResponse at sdk/x/distribution/types/query.pb.go:357 |

## proto-cosmos-distribution-v1beta1-validatoraccumulatedcommission-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoraccumulatedcommission-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:65</code> | <code>message cosmos.distribution.v1beta1.ValidatorAccumulatedCommission { repeated cosmos.base.v1beta1.DecCoin commission = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoraccumulatedcommission-j39</code> | notes | Go type ValidatorAccumulatedCommission at sdk/x/distribution/types/distribution.pb.go:197 |

## proto-cosmos-distribution-v1beta1-validatoraccumulatedcommissionrecord-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoraccumulatedcommissionrecord-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:43</code> | <code>message cosmos.distribution.v1beta1.ValidatorAccumulatedCommissionRecord { string validator_address = 1; ValidatorAccumulatedCommission accumulated = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoraccumulatedcommissionrecord-j39</code> | notes | Go type ValidatorAccumulatedCommissionRecord at sdk/x/distribution/types/genesis.pb.go:114 |

## proto-cosmos-distribution-v1beta1-validatorcurrentrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorcurrentrewards-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:55</code> | <code>message cosmos.distribution.v1beta1.ValidatorCurrentRewards { repeated cosmos.base.v1beta1.DecCoin rewards = 1; uint64 period = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorcurrentrewards-j39</code> | notes | Go type ValidatorCurrentRewards at sdk/x/distribution/types/distribution.pb.go:143 |

## proto-cosmos-distribution-v1beta1-validatorcurrentrewardsrecord-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorcurrentrewardsrecord-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:77</code> | <code>message cosmos.distribution.v1beta1.ValidatorCurrentRewardsRecord { string validator_address = 1; ValidatorCurrentRewards rewards = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorcurrentrewardsrecord-j39</code> | notes | Go type ValidatorCurrentRewardsRecord at sdk/x/distribution/types/genesis.pb.go:199 |

## proto-cosmos-distribution-v1beta1-validatorhistoricalrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorhistoricalrewards-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:43</code> | <code>message cosmos.distribution.v1beta1.ValidatorHistoricalRewards { repeated cosmos.base.v1beta1.DecCoin cumulative_reward_ratio = 1; uint32 reference_count = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorhistoricalrewards-j39</code> | notes | Go type ValidatorHistoricalRewards at sdk/x/distribution/types/distribution.pb.go:88 |

## proto-cosmos-distribution-v1beta1-validatorhistoricalrewardsrecord-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorhistoricalrewardsrecord-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:59</code> | <code>message cosmos.distribution.v1beta1.ValidatorHistoricalRewardsRecord { string validator_address = 1; uint64 period = 2; ValidatorHistoricalRewards rewards = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorhistoricalrewardsrecord-j39</code> | notes | Go type ValidatorHistoricalRewardsRecord at sdk/x/distribution/types/genesis.pb.go:156 |

## proto-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:74</code> | <code>message cosmos.distribution.v1beta1.ValidatorOutstandingRewards { repeated cosmos.base.v1beta1.DecCoin rewards = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39</code> | notes | Go type ValidatorOutstandingRewards at sdk/x/distribution/types/distribution.pb.go:243 |

## proto-cosmos-distribution-v1beta1-validatoroutstandingrewardsrecord-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoroutstandingrewardsrecord-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:26</code> | <code>message cosmos.distribution.v1beta1.ValidatorOutstandingRewardsRecord { string validator_address = 1; repeated cosmos.base.v1beta1.DecCoin outstanding_rewards = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatoroutstandingrewardsrecord-j39</code> | notes | Go type ValidatorOutstandingRewardsRecord at sdk/x/distribution/types/genesis.pb.go:72 |

## proto-cosmos-distribution-v1beta1-validatorslashevent-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorslashevent-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:86</code> | <code>message cosmos.distribution.v1beta1.ValidatorSlashEvent { uint64 validator_period = 1; string fraction = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorslashevent-j39</code> | notes | Go type ValidatorSlashEvent at sdk/x/distribution/types/distribution.pb.go:291 |

## proto-cosmos-distribution-v1beta1-validatorslasheventrecord-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorslasheventrecord-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/genesis.proto:110</code> | <code>message cosmos.distribution.v1beta1.ValidatorSlashEventRecord { string validator_address = 1; uint64 height = 2; uint64 period = 3; ValidatorSlashEvent validator_slash_event = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorslasheventrecord-j39</code> | notes | Go type ValidatorSlashEventRecord at sdk/x/distribution/types/genesis.pb.go:283 |

## proto-cosmos-distribution-v1beta1-validatorslashevents-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorslashevents-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/distribution.proto:95</code> | <code>message cosmos.distribution.v1beta1.ValidatorSlashEvents { repeated ValidatorSlashEvent validator_slash_events = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-distribution-v1beta1-validatorslashevents-j39</code> | notes | Go type ValidatorSlashEvents at sdk/x/distribution/types/distribution.pb.go:337 |

## proto-cosmos-evidence-v1beta1-equivocation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-equivocation-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/evidence.proto:12</code> | <code>message cosmos.evidence.v1beta1.Equivocation { int64 height = 1; google.protobuf.Timestamp time = 2; int64 power = 3; string consensus_address = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-equivocation-j39</code> | notes | Go type Equivocation at sdk/x/evidence/types/evidence.pb.go:32 |

## proto-cosmos-evidence-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/genesis.proto:9</code> | <code>message cosmos.evidence.v1beta1.GenesisState { repeated google.protobuf.Any evidence = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/evidence/types/genesis.pb.go:27 |

## proto-cosmos-evidence-v1beta1-msgsubmitevidence-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-msgsubmitevidence-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/tx.proto:20</code> | <code>message cosmos.evidence.v1beta1.MsgSubmitEvidence { string submitter = 1; google.protobuf.Any evidence = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-msgsubmitevidence-j39</code> | notes | Go type MsgSubmitEvidence at sdk/x/evidence/types/tx.pb.go:36 |

## proto-cosmos-evidence-v1beta1-msgsubmitevidenceresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-msgsubmitevidenceresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/tx.proto:29</code> | <code>message cosmos.evidence.v1beta1.MsgSubmitEvidenceResponse { bytes hash = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-msgsubmitevidenceresponse-j39</code> | notes | Go type MsgSubmitEvidenceResponse at sdk/x/evidence/types/tx.pb.go:75 |

## proto-cosmos-evidence-v1beta1-queryallevidencerequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryallevidencerequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:38</code> | <code>message cosmos.evidence.v1beta1.QueryAllEvidenceRequest { cosmos.base.query.v1beta1.PageRequest pagination = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryallevidencerequest-j39</code> | notes | Go type QueryAllEvidenceRequest at sdk/x/evidence/types/query.pb.go:129 |

## proto-cosmos-evidence-v1beta1-queryallevidenceresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryallevidenceresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:45</code> | <code>message cosmos.evidence.v1beta1.QueryAllEvidenceResponse { repeated google.protobuf.Any evidence = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryallevidenceresponse-j39</code> | notes | Go type QueryAllEvidenceResponse at sdk/x/evidence/types/query.pb.go:176 |

## proto-cosmos-evidence-v1beta1-queryevidencerequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryevidencerequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:25</code> | <code>message cosmos.evidence.v1beta1.QueryEvidenceRequest { bytes evidence_hash = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryevidencerequest-j39</code> | notes | Go type QueryEvidenceRequest at sdk/x/evidence/types/query.pb.go:36 |

## proto-cosmos-evidence-v1beta1-queryevidenceresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryevidenceresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:31</code> | <code>message cosmos.evidence.v1beta1.QueryEvidenceResponse { google.protobuf.Any evidence = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-evidence-v1beta1-queryevidenceresponse-j39</code> | notes | Go type QueryEvidenceResponse at sdk/x/evidence/types/query.pb.go:82 |

## proto-cosmos-feegrant-v1beta1-allowedmsgallowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-allowedmsgallowance-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto:69</code> | <code>message cosmos.feegrant.v1beta1.AllowedMsgAllowance { google.protobuf.Any allowance = 1; repeated string allowed_messages = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-allowedmsgallowance-j39</code> | notes | Go type AllowedMsgAllowance at sdk/x/feegrant/feegrant.pb.go:180 |

## proto-cosmos-feegrant-v1beta1-basicallowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-basicallowance-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto:16</code> | <code>message cosmos.feegrant.v1beta1.BasicAllowance { repeated cosmos.base.v1beta1.Coin spend_limit = 1; google.protobuf.Timestamp expiration = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-basicallowance-j39</code> | notes | Go type BasicAllowance at sdk/x/feegrant/feegrant.pb.go:36 |

## proto-cosmos-feegrant-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/genesis.proto:11</code> | <code>message cosmos.feegrant.v1beta1.GenesisState { repeated Grant allowances = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/feegrant/genesis.pb.go:27 |

## proto-cosmos-feegrant-v1beta1-grant-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-grant-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto:81</code> | <code>message cosmos.feegrant.v1beta1.Grant { string granter = 1; string grantee = 2; google.protobuf.Any allowance = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-grant-j39</code> | notes | Go type Grant at sdk/x/feegrant/feegrant.pb.go:221 |

## proto-cosmos-feegrant-v1beta1-msggrantallowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msggrantallowance-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/tx.proto:23</code> | <code>message cosmos.feegrant.v1beta1.MsgGrantAllowance { string granter = 1; string grantee = 2; google.protobuf.Any allowance = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msggrantallowance-j39</code> | notes | Go type MsgGrantAllowance at sdk/x/feegrant/tx.pb.go:34 |

## proto-cosmos-feegrant-v1beta1-msggrantallowanceresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msggrantallowanceresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/tx.proto:35</code> | <code>message cosmos.feegrant.v1beta1.MsgGrantAllowanceResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msggrantallowanceresponse-j39</code> | notes | Go type MsgGrantAllowanceResponse at sdk/x/feegrant/tx.pb.go:98 |

## proto-cosmos-feegrant-v1beta1-msgrevokeallowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msgrevokeallowance-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/tx.proto:38</code> | <code>message cosmos.feegrant.v1beta1.MsgRevokeAllowance { string granter = 1; string grantee = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msgrevokeallowance-j39</code> | notes | Go type MsgRevokeAllowance at sdk/x/feegrant/tx.pb.go:135 |

## proto-cosmos-feegrant-v1beta1-msgrevokeallowanceresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msgrevokeallowanceresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/tx.proto:47</code> | <code>message cosmos.feegrant.v1beta1.MsgRevokeAllowanceResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-msgrevokeallowanceresponse-j39</code> | notes | Go type MsgRevokeAllowanceResponse at sdk/x/feegrant/tx.pb.go:190 |

## proto-cosmos-feegrant-v1beta1-periodicallowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-periodicallowance-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/feegrant.proto:33</code> | <code>message cosmos.feegrant.v1beta1.PeriodicAllowance { BasicAllowance basic = 1; google.protobuf.Duration period = 2; repeated cosmos.base.v1beta1.Coin period_spend_limit = 3; repeated cosmos.base.v1beta1.Coin period_can_spend = 4; google.protobuf.Timestamp period_reset = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-periodicallowance-j39</code> | notes | Go type PeriodicAllowance at sdk/x/feegrant/feegrant.pb.go:94 |

## proto-cosmos-feegrant-v1beta1-queryallowancerequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancerequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:31</code> | <code>message cosmos.feegrant.v1beta1.QueryAllowanceRequest { string granter = 1; string grantee = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancerequest-j39</code> | notes | Go type QueryAllowanceRequest at sdk/x/feegrant/query.pb.go:33 |

## proto-cosmos-feegrant-v1beta1-queryallowanceresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowanceresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:40</code> | <code>message cosmos.feegrant.v1beta1.QueryAllowanceResponse { cosmos.feegrant.v1beta1.Grant allowance = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowanceresponse-j39</code> | notes | Go type QueryAllowanceResponse at sdk/x/feegrant/query.pb.go:88 |

## proto-cosmos-feegrant-v1beta1-queryallowancesbygranterrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesbygranterrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:63</code> | <code>message cosmos.feegrant.v1beta1.QueryAllowancesByGranterRequest { string granter = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesbygranterrequest-j39</code> | notes | Go type QueryAllowancesByGranterRequest at sdk/x/feegrant/query.pb.go:243 |

## proto-cosmos-feegrant-v1beta1-queryallowancesbygranterresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesbygranterresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:71</code> | <code>message cosmos.feegrant.v1beta1.QueryAllowancesByGranterResponse { repeated cosmos.feegrant.v1beta1.Grant allowances = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesbygranterresponse-j39</code> | notes | Go type QueryAllowancesByGranterResponse at sdk/x/feegrant/query.pb.go:297 |

## proto-cosmos-feegrant-v1beta1-queryallowancesrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:46</code> | <code>message cosmos.feegrant.v1beta1.QueryAllowancesRequest { string grantee = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesrequest-j39</code> | notes | Go type QueryAllowancesRequest at sdk/x/feegrant/query.pb.go:134 |

## proto-cosmos-feegrant-v1beta1-queryallowancesresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:54</code> | <code>message cosmos.feegrant.v1beta1.QueryAllowancesResponse { repeated cosmos.feegrant.v1beta1.Grant allowances = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-feegrant-v1beta1-queryallowancesresponse-j39</code> | notes | Go type QueryAllowancesResponse at sdk/x/feegrant/query.pb.go:188 |

## proto-cosmos-genutil-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-genutil-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/genutil/v1beta1/genesis.proto:9</code> | <code>message cosmos.genutil.v1beta1.GenesisState { repeated bytes gen_txs = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-genutil-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/genutil/types/genesis.pb.go:28 |

## proto-cosmos-gov-v1beta1-deposit-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-deposit-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:58</code> | <code>message cosmos.gov.v1beta1.Deposit { uint64 proposal_id = 1; string depositor = 2; repeated cosmos.base.v1beta1.Coin amount = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-deposit-j39</code> | notes | Go type Deposit at sdk/x/gov/types/gov.pb.go:205 |

## proto-cosmos-gov-v1beta1-depositparams-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-depositparams-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:175</code> | <code>message cosmos.gov.v1beta1.DepositParams { repeated cosmos.base.v1beta1.Coin min_deposit = 1; google.protobuf.Duration max_deposit_period = 2; repeated cosmos.base.v1beta1.Coin min_expedited_deposit = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-depositparams-j39</code> | notes | Go type DepositParams at sdk/x/gov/types/gov.pb.go:375 |

## proto-cosmos-gov-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/genesis.proto:11</code> | <code>message cosmos.gov.v1beta1.GenesisState { uint64 starting_proposal_id = 1; repeated Deposit deposits = 2; repeated Vote votes = 3; repeated Proposal proposals = 4; DepositParams deposit_params = 5; VotingParams voting_params = 6; TallyParams tally_params = 7; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/gov/types/genesis.pb.go:27 |

## proto-cosmos-gov-v1beta1-msgdeposit-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgdeposit-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:93</code> | <code>message cosmos.gov.v1beta1.MsgDeposit { uint64 proposal_id = 1; string depositor = 2; repeated cosmos.base.v1beta1.Coin amount = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgdeposit-j39</code> | notes | Go type MsgDeposit at sdk/x/gov/types/tx.pb.go:278 |

## proto-cosmos-gov-v1beta1-msgdepositresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgdepositresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:111</code> | <code>message cosmos.gov.v1beta1.MsgDepositResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgdepositresponse-j39</code> | notes | Go type MsgDepositResponse at sdk/x/gov/types/tx.pb.go:317 |

## proto-cosmos-gov-v1beta1-msgsubmitproposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgsubmitproposal-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:31</code> | <code>message cosmos.gov.v1beta1.MsgSubmitProposal { google.protobuf.Any content = 1; repeated cosmos.base.v1beta1.Coin initial_deposit = 2; string proposer = 3; bool is_expedited = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgsubmitproposal-j39</code> | notes | Go type MsgSubmitProposal at sdk/x/gov/types/tx.pb.go:37 |

## proto-cosmos-gov-v1beta1-msgsubmitproposalresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgsubmitproposalresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:48</code> | <code>message cosmos.gov.v1beta1.MsgSubmitProposalResponse { uint64 proposal_id = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgsubmitproposalresponse-j39</code> | notes | Go type MsgSubmitProposalResponse at sdk/x/gov/types/tx.pb.go:77 |

## proto-cosmos-gov-v1beta1-msgvote-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvote-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:56</code> | <code>message cosmos.gov.v1beta1.MsgVote { uint64 proposal_id = 1; string voter = 2; VoteOption option = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvote-j39</code> | notes | Go type MsgVote at sdk/x/gov/types/tx.pb.go:122 |

## proto-cosmos-gov-v1beta1-msgvoteresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvoteresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:71</code> | <code>message cosmos.gov.v1beta1.MsgVoteResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvoteresponse-j39</code> | notes | Go type MsgVoteResponse at sdk/x/gov/types/tx.pb.go:161 |

## proto-cosmos-gov-v1beta1-msgvoteweighted-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvoteweighted-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:76</code> | <code>message cosmos.gov.v1beta1.MsgVoteWeighted { uint64 proposal_id = 1; string voter = 2; repeated WeightedVoteOption options = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvoteweighted-j39</code> | notes | Go type MsgVoteWeighted at sdk/x/gov/types/tx.pb.go:200 |

## proto-cosmos-gov-v1beta1-msgvoteweightedresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvoteweightedresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/tx.proto:90</code> | <code>message cosmos.gov.v1beta1.MsgVoteWeightedResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-msgvoteweightedresponse-j39</code> | notes | Go type MsgVoteWeightedResponse at sdk/x/gov/types/tx.pb.go:241 |

## proto-cosmos-gov-v1beta1-proposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-proposal-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:71</code> | <code>message cosmos.gov.v1beta1.Proposal { uint64 proposal_id = 1; google.protobuf.Any content = 2; ProposalStatus status = 3; TallyResult final_tally_result = 4; google.protobuf.Timestamp submit_time = 5; google.protobuf.Timestamp deposit_end_time = 6; repeated cosmos.base.v1beta1.Coin total_deposit = 7; google.protobuf.Timestamp voting_start_time = 8; google.protobuf.Timestamp voting_end_time = 9; bool is_expedited = 10; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-proposal-j39</code> | notes | Go type Proposal at sdk/x/gov/types/gov.pb.go:244 |

## proto-cosmos-gov-v1beta1-proposalstatus-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-proposalstatus-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:113</code> | <code>enum cosmos.gov.v1beta1.ProposalStatus { PROPOSAL_STATUS_UNSPECIFIED = 0; PROPOSAL_STATUS_DEPOSIT_PERIOD = 1; PROPOSAL_STATUS_VOTING_PERIOD = 2; PROPOSAL_STATUS_PASSED = 3; PROPOSAL_STATUS_REJECTED = 4; PROPOSAL_STATUS_FAILED = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-proposalstatus-j39</code> | notes | Go enum ProposalStatus at sdk/x/gov/types/gov.pb.go:75 |

## proto-cosmos-gov-v1beta1-querydepositrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:146</code> | <code>message cosmos.gov.v1beta1.QueryDepositRequest { uint64 proposal_id = 1; string depositor = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositrequest-j39</code> | notes | Go type QueryDepositRequest at sdk/x/gov/types/query.pb.go:533 |

## proto-cosmos-gov-v1beta1-querydepositresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:158</code> | <code>message cosmos.gov.v1beta1.QueryDepositResponse { Deposit deposit = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositresponse-j39</code> | notes | Go type QueryDepositResponse at sdk/x/gov/types/query.pb.go:574 |

## proto-cosmos-gov-v1beta1-querydepositsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:164</code> | <code>message cosmos.gov.v1beta1.QueryDepositsRequest { uint64 proposal_id = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositsrequest-j39</code> | notes | Go type QueryDepositsRequest at sdk/x/gov/types/query.pb.go:620 |

## proto-cosmos-gov-v1beta1-querydepositsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:173</code> | <code>message cosmos.gov.v1beta1.QueryDepositsResponse { repeated Deposit deposits = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querydepositsresponse-j39</code> | notes | Go type QueryDepositsResponse at sdk/x/gov/types/query.pb.go:675 |

## proto-cosmos-gov-v1beta1-queryparamsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryparamsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:129</code> | <code>message cosmos.gov.v1beta1.QueryParamsRequest { string params_type = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryparamsrequest-j39</code> | notes | Go type QueryParamsRequest at sdk/x/gov/types/query.pb.go:422 |

## proto-cosmos-gov-v1beta1-queryparamsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryparamsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:136</code> | <code>message cosmos.gov.v1beta1.QueryParamsResponse { VotingParams voting_params = 1; DepositParams deposit_params = 2; TallyParams tally_params = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryparamsresponse-j39</code> | notes | Go type QueryParamsResponse at sdk/x/gov/types/query.pb.go:469 |

## proto-cosmos-gov-v1beta1-queryproposalrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:55</code> | <code>message cosmos.gov.v1beta1.QueryProposalRequest { uint64 proposal_id = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalrequest-j39</code> | notes | Go type QueryProposalRequest at sdk/x/gov/types/query.pb.go:34 |

## proto-cosmos-gov-v1beta1-queryproposalresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:61</code> | <code>message cosmos.gov.v1beta1.QueryProposalResponse { Proposal proposal = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalresponse-j39</code> | notes | Go type QueryProposalResponse at sdk/x/gov/types/query.pb.go:80 |

## proto-cosmos-gov-v1beta1-queryproposalsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:66</code> | <code>message cosmos.gov.v1beta1.QueryProposalsRequest { ProposalStatus proposal_status = 1; string voter = 2; string depositor = 3; cosmos.base.query.v1beta1.PageRequest pagination = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalsrequest-j39</code> | notes | Go type QueryProposalsRequest at sdk/x/gov/types/query.pb.go:125 |

## proto-cosmos-gov-v1beta1-queryproposalsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:85</code> | <code>message cosmos.gov.v1beta1.QueryProposalsResponse { repeated Proposal proposals = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryproposalsresponse-j39</code> | notes | Go type QueryProposalsResponse at sdk/x/gov/types/query.pb.go:171 |

## proto-cosmos-gov-v1beta1-querytallyresultrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querytallyresultrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:181</code> | <code>message cosmos.gov.v1beta1.QueryTallyResultRequest { uint64 proposal_id = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querytallyresultrequest-j39</code> | notes | Go type QueryTallyResultRequest at sdk/x/gov/types/query.pb.go:729 |

## proto-cosmos-gov-v1beta1-querytallyresultresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querytallyresultresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:187</code> | <code>message cosmos.gov.v1beta1.QueryTallyResultResponse { TallyResult tally = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-querytallyresultresponse-j39</code> | notes | Go type QueryTallyResultResponse at sdk/x/gov/types/query.pb.go:775 |

## proto-cosmos-gov-v1beta1-queryvoterequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvoterequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:93</code> | <code>message cosmos.gov.v1beta1.QueryVoteRequest { uint64 proposal_id = 1; string voter = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvoterequest-j39</code> | notes | Go type QueryVoteRequest at sdk/x/gov/types/query.pb.go:225 |

## proto-cosmos-gov-v1beta1-queryvoteresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvoteresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:105</code> | <code>message cosmos.gov.v1beta1.QueryVoteResponse { Vote vote = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvoteresponse-j39</code> | notes | Go type QueryVoteResponse at sdk/x/gov/types/query.pb.go:266 |

## proto-cosmos-gov-v1beta1-queryvotesrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvotesrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:111</code> | <code>message cosmos.gov.v1beta1.QueryVotesRequest { uint64 proposal_id = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvotesrequest-j39</code> | notes | Go type QueryVotesRequest at sdk/x/gov/types/query.pb.go:312 |

## proto-cosmos-gov-v1beta1-queryvotesresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvotesresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:120</code> | <code>message cosmos.gov.v1beta1.QueryVotesResponse { repeated Vote votes = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-queryvotesresponse-j39</code> | notes | Go type QueryVotesResponse at sdk/x/gov/types/query.pb.go:367 |

## proto-cosmos-gov-v1beta1-tallyparams-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-tallyparams-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:222</code> | <code>message cosmos.gov.v1beta1.TallyParams { bytes quorum = 1; bytes threshold = 2; bytes veto_threshold = 3; bytes expedited_quorum = 4; bytes expedited_threshold = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-tallyparams-j39</code> | notes | Go type TallyParams at sdk/x/gov/types/gov.pb.go:458 |

## proto-cosmos-gov-v1beta1-tallyresult-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-tallyresult-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:136</code> | <code>message cosmos.gov.v1beta1.TallyResult { string yes = 1; string abstain = 2; string no = 3; string no_with_veto = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-tallyresult-j39</code> | notes | Go type TallyResult at sdk/x/gov/types/gov.pb.go:290 |

## proto-cosmos-gov-v1beta1-textproposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-textproposal-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:46</code> | <code>message cosmos.gov.v1beta1.TextProposal { string title = 1; string description = 2; bool is_expedited = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-textproposal-j39</code> | notes | Go type TextProposal at sdk/x/gov/types/gov.pb.go:165 |

## proto-cosmos-gov-v1beta1-vote-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-vote-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:160</code> | <code>message cosmos.gov.v1beta1.Vote { uint64 proposal_id = 1; string voter = 2; VoteOption option = 3; repeated WeightedVoteOption options = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-vote-j39</code> | notes | Go type Vote at sdk/x/gov/types/gov.pb.go:331 |

## proto-cosmos-gov-v1beta1-voteoption-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-voteoption-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:17</code> | <code>enum cosmos.gov.v1beta1.VoteOption { VOTE_OPTION_UNSPECIFIED = 0; VOTE_OPTION_YES = 1; VOTE_OPTION_ABSTAIN = 2; VOTE_OPTION_NO = 3; VOTE_OPTION_NO_WITH_VETO = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-voteoption-j39</code> | notes | Go enum VoteOption at sdk/x/gov/types/gov.pb.go:35 |

## proto-cosmos-gov-v1beta1-votingparams-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-votingparams-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:203</code> | <code>message cosmos.gov.v1beta1.VotingParams { google.protobuf.Duration voting_period = 1; google.protobuf.Duration expedited_voting_period = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-votingparams-j39</code> | notes | Go type VotingParams at sdk/x/gov/types/gov.pb.go:418 |

## proto-cosmos-gov-v1beta1-weightedvoteoption-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-weightedvoteoption-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/gov.proto:35</code> | <code>message cosmos.gov.v1beta1.WeightedVoteOption { VoteOption option = 1; string weight = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-gov-v1beta1-weightedvoteoption-j39</code> | notes | Go type WeightedVoteOption at sdk/x/gov/types/gov.pb.go:126 |

## proto-cosmos-params-v1beta1-cosmosgasparams-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-cosmosgasparams-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/types/types.proto:19</code> | <code>message cosmos.params.v1beta1.CosmosGasParams { uint64 cosmos_gas_multiplier_numerator = 1; uint64 cosmos_gas_multiplier_denominator = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-cosmosgasparams-j39</code> | notes | Go type CosmosGasParams at sdk/x/params/types/types.pb.go:81 |

## proto-cosmos-params-v1beta1-feesparams-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-feesparams-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/types/types.proto:11</code> | <code>message cosmos.params.v1beta1.FeesParams { repeated cosmos.base.v1beta1.DecCoin global_minimum_gas_prices = 1; repeated string allowed_fee_denoms = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-feesparams-j39</code> | notes | Go type FeesParams at sdk/x/params/types/types.pb.go:29 |

## proto-cosmos-params-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/types/types.proto:24</code> | <code>message cosmos.params.v1beta1.GenesisState { FeesParams fees_params = 1; CosmosGasParams cosmos_gas_params = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/params/types/types.pb.go:133 |

## proto-cosmos-params-v1beta1-paramchange-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-paramchange-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/v1beta1/params.proto:22</code> | <code>message cosmos.params.v1beta1.ParamChange { string subspace = 1; string key = 2; string value = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-paramchange-j39</code> | notes | Go type ParamChange at sdk/x/params/types/proposal/params.pb.go:68 |

## proto-cosmos-params-v1beta1-parameterchangeproposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-parameterchangeproposal-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/v1beta1/params.proto:10</code> | <code>message cosmos.params.v1beta1.ParameterChangeProposal { string title = 1; string description = 2; repeated ParamChange changes = 3; bool is_expedited = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-parameterchangeproposal-j39</code> | notes | Go type ParameterChangeProposal at sdk/x/params/types/proposal/params.pb.go:27 |

## proto-cosmos-params-v1beta1-queryparamsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-queryparamsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/v1beta1/query.proto:20</code> | <code>message cosmos.params.v1beta1.QueryParamsRequest { string subspace = 1; string key = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-queryparamsrequest-j39</code> | notes | Go type QueryParamsRequest at sdk/x/params/types/proposal/query.pb.go:33 |

## proto-cosmos-params-v1beta1-queryparamsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-queryparamsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/v1beta1/query.proto:29</code> | <code>message cosmos.params.v1beta1.QueryParamsResponse { ParamChange param = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-params-v1beta1-queryparamsresponse-j39</code> | notes | Go type QueryParamsResponse at sdk/x/params/types/proposal/query.pb.go:88 |

## proto-cosmos-slashing-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/genesis.proto:10</code> | <code>message cosmos.slashing.v1beta1.GenesisState { Params params = 1; repeated SigningInfo signing_infos = 2; repeated ValidatorMissedBlockArray missed_blocks = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/slashing/types/genesis.pb.go:27 |

## proto-cosmos-slashing-v1beta1-genesisstatelegacymissingheights-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-genesisstatelegacymissingheights-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/genesis.proto:30</code> | <code>message cosmos.slashing.v1beta1.GenesisStateLegacyMissingHeights { Params params = 1; repeated SigningInfo signing_infos = 2; repeated ValidatorMissedBlockArrayLegacyMissedHeights missed_blocks = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-genesisstatelegacymissingheights-j39</code> | notes | Go type GenesisStateLegacyMissingHeights at sdk/x/slashing/types/genesis.pb.go:93; no in-repo reference outside the generated package |

## proto-cosmos-slashing-v1beta1-genesisstatelegacyv43-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-genesisstatelegacyv43-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/genesis.proto:50</code> | <code>message cosmos.slashing.v1beta1.GenesisStateLegacyV43 { Params params = 1; repeated SigningInfo signing_infos = 2; repeated ValidatorMissedBlocks missed_blocks = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-genesisstatelegacyv43-j39</code> | notes | Go type GenesisStateLegacyV43 at sdk/x/slashing/types/genesis.pb.go:159; no in-repo reference outside the generated package |

## proto-cosmos-slashing-v1beta1-missedblock-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-missedblock-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/genesis.proto:104</code> | <code>message cosmos.slashing.v1beta1.MissedBlock { int64 index = 1; bool missed = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-missedblock-j39</code> | notes | Go type MissedBlock at sdk/x/slashing/types/genesis.pb.go:391; no in-repo reference outside the generated package |

## proto-cosmos-slashing-v1beta1-msgunjail-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-msgunjail-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/tx.proto:18</code> | <code>message cosmos.slashing.v1beta1.MsgUnjail { string validator_addr = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-msgunjail-j39</code> | notes | Go type MsgUnjail at sdk/x/slashing/types/tx.pb.go:32 |

## proto-cosmos-slashing-v1beta1-msgunjailresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-msgunjailresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/tx.proto:29</code> | <code>message cosmos.slashing.v1beta1.MsgUnjailResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-msgunjailresponse-j39</code> | notes | Go type MsgUnjailResponse at sdk/x/slashing/types/tx.pb.go:70 |

## proto-cosmos-slashing-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-params-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/slashing.proto:75</code> | <code>message cosmos.slashing.v1beta1.Params { int64 signed_blocks_window = 1; bytes min_signed_per_window = 2; google.protobuf.Duration downtime_jail_duration = 3; bytes slash_fraction_double_sign = 4; bytes slash_fraction_downtime = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-params-j39</code> | notes | Go type Params at sdk/x/slashing/types/slashing.pb.go:331 |

## proto-cosmos-slashing-v1beta1-queryparamsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-queryparamsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:30</code> | <code>message cosmos.slashing.v1beta1.QueryParamsRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-queryparamsrequest-j39</code> | notes | Go type QueryParamsRequest at sdk/x/slashing/types/query.pb.go:34 |

## proto-cosmos-slashing-v1beta1-queryparamsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-queryparamsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:33</code> | <code>message cosmos.slashing.v1beta1.QueryParamsResponse { Params params = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-queryparamsresponse-j39</code> | notes | Go type QueryParamsResponse at sdk/x/slashing/types/query.pb.go:71 |

## proto-cosmos-slashing-v1beta1-querysigninginforequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginforequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:39</code> | <code>message cosmos.slashing.v1beta1.QuerySigningInfoRequest { string cons_address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginforequest-j39</code> | notes | Go type QuerySigningInfoRequest at sdk/x/slashing/types/query.pb.go:117 |

## proto-cosmos-slashing-v1beta1-querysigninginforesponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginforesponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:46</code> | <code>message cosmos.slashing.v1beta1.QuerySigningInfoResponse { ValidatorSigningInfo val_signing_info = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginforesponse-j39</code> | notes | Go type QuerySigningInfoResponse at sdk/x/slashing/types/query.pb.go:164 |

## proto-cosmos-slashing-v1beta1-querysigninginfosrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginfosrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:53</code> | <code>message cosmos.slashing.v1beta1.QuerySigningInfosRequest { cosmos.base.query.v1beta1.PageRequest pagination = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginfosrequest-j39</code> | notes | Go type QuerySigningInfosRequest at sdk/x/slashing/types/query.pb.go:211 |

## proto-cosmos-slashing-v1beta1-querysigninginfosresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginfosresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:59</code> | <code>message cosmos.slashing.v1beta1.QuerySigningInfosResponse { repeated cosmos.slashing.v1beta1.ValidatorSigningInfo info = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-querysigninginfosresponse-j39</code> | notes | Go type QuerySigningInfosResponse at sdk/x/slashing/types/query.pb.go:257 |

## proto-cosmos-slashing-v1beta1-signinginfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-signinginfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/genesis.proto:70</code> | <code>message cosmos.slashing.v1beta1.SigningInfo { string address = 1; ValidatorSigningInfo validator_signing_info = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-signinginfo-j39</code> | notes | Go type SigningInfo at sdk/x/slashing/types/genesis.pb.go:225 |

## proto-cosmos-slashing-v1beta1-signinginfolegacymissedheights-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-signinginfolegacymissedheights-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/genesis.proto:81</code> | <code>message cosmos.slashing.v1beta1.SigningInfoLegacyMissedHeights { string address = 1; ValidatorSigningInfoLegacyMissedHeights validator_signing_info = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-signinginfolegacymissedheights-j39</code> | notes | Go type SigningInfoLegacyMissedHeights at sdk/x/slashing/types/genesis.pb.go:280; no in-repo reference outside the generated package |

## proto-cosmos-slashing-v1beta1-validatormissedblockarray-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatormissedblockarray-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/slashing.proto:66</code> | <code>message cosmos.slashing.v1beta1.ValidatorMissedBlockArray { string address = 1; int64 window_size = 2; repeated uint64 missed_blocks = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatormissedblockarray-j39</code> | notes | Go type ValidatorMissedBlockArray at sdk/x/slashing/types/slashing.pb.go:268 |

## proto-cosmos-slashing-v1beta1-validatormissedblockarraylegacymissedheights-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatormissedblockarraylegacymissedheights-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/slashing.proto:59</code> | <code>message cosmos.slashing.v1beta1.ValidatorMissedBlockArrayLegacyMissedHeights { string address = 1; repeated int64 missed_heights = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatormissedblockarraylegacymissedheights-j39</code> | notes | Go type ValidatorMissedBlockArrayLegacyMissedHeights at sdk/x/slashing/types/slashing.pb.go:210 |

## proto-cosmos-slashing-v1beta1-validatormissedblocks-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatormissedblocks-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/genesis.proto:93</code> | <code>message cosmos.slashing.v1beta1.ValidatorMissedBlocks { string address = 1; repeated MissedBlock missed_blocks = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatormissedblocks-j39</code> | notes | Go type ValidatorMissedBlocks at sdk/x/slashing/types/genesis.pb.go:336; no in-repo reference outside the generated package |

## proto-cosmos-slashing-v1beta1-validatorsigninginfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatorsigninginfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/slashing.proto:33</code> | <code>message cosmos.slashing.v1beta1.ValidatorSigningInfo { string address = 1; int64 start_height = 2; int64 index_offset = 3; google.protobuf.Timestamp jailed_until = 4; bool tombstoned = 5; int64 missed_blocks_counter = 6; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatorsigninginfo-j39</code> | notes | Go type ValidatorSigningInfo at sdk/x/slashing/types/slashing.pb.go:117 |

## proto-cosmos-slashing-v1beta1-validatorsigninginfolegacymissedheights-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatorsigninginfolegacymissedheights-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/slashing.proto:11</code> | <code>message cosmos.slashing.v1beta1.ValidatorSigningInfoLegacyMissedHeights { string address = 1; int64 start_height = 2; google.protobuf.Timestamp jailed_until = 3; bool tombstoned = 4; int64 missed_blocks_counter = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-slashing-v1beta1-validatorsigninginfolegacymissedheights-j39</code> | notes | Go type ValidatorSigningInfoLegacyMissedHeights at sdk/x/slashing/types/slashing.pb.go:31 |

## proto-cosmos-staking-v1beta1-authorizationtype-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-authorizationtype-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/authz.proto:38</code> | <code>enum cosmos.staking.v1beta1.AuthorizationType { AUTHORIZATION_TYPE_UNSPECIFIED = 0; AUTHORIZATION_TYPE_DELEGATE = 1; AUTHORIZATION_TYPE_UNDELEGATE = 2; AUTHORIZATION_TYPE_REDELEGATE = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-authorizationtype-j39</code> | notes | Go enum AuthorizationType at sdk/x/staking/types/authz.pb.go:31 |

## proto-cosmos-staking-v1beta1-bondstatus-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-bondstatus-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:139</code> | <code>enum cosmos.staking.v1beta1.BondStatus { BOND_STATUS_UNSPECIFIED = 0; BOND_STATUS_UNBONDED = 1; BOND_STATUS_UNBONDING = 2; BOND_STATUS_BONDED = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-bondstatus-j39</code> | notes | Go enum BondStatus at sdk/x/staking/types/staking.pb.go:43 |

## proto-cosmos-staking-v1beta1-commission-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-commission-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:49</code> | <code>message cosmos.staking.v1beta1.Commission { CommissionRates commission_rates = 1; google.protobuf.Timestamp update_time = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-commission-j39</code> | notes | Go type Commission at sdk/x/staking/types/staking.pb.go:178 |

## proto-cosmos-staking-v1beta1-commissionrates-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-commissionrates-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:25</code> | <code>message cosmos.staking.v1beta1.CommissionRates { string rate = 1; string max_rate = 2; string max_change_rate = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-commissionrates-j39</code> | notes | Go type CommissionRates at sdk/x/staking/types/staking.pb.go:136 |

## proto-cosmos-staking-v1beta1-delegation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-delegation-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:199</code> | <code>message cosmos.staking.v1beta1.Delegation { string delegator_address = 1; string validator_address = 2; string shares = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-delegation-j39</code> | notes | Go type Delegation at sdk/x/staking/types/staking.pb.go:589 |

## proto-cosmos-staking-v1beta1-delegationresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-delegationresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:342</code> | <code>message cosmos.staking.v1beta1.DelegationResponse { Delegation delegation = 1; cosmos.base.v1beta1.Coin balance = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-delegationresponse-j39</code> | notes | Go type DelegationResponse at sdk/x/staking/types/staking.pb.go:923 |

## proto-cosmos-staking-v1beta1-description-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-description-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:67</code> | <code>message cosmos.staking.v1beta1.Description { string moniker = 1; string identity = 2; string website = 3; string security_contact = 4; string details = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-description-j39</code> | notes | Go type Description at sdk/x/staking/types/staking.pb.go:225 |

## proto-cosmos-staking-v1beta1-dvpair-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvpair-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:163</code> | <code>message cosmos.staking.v1beta1.DVPair { string delegator_address = 1; string validator_address = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvpair-j39</code> | notes | Go type DVPair at sdk/x/staking/types/staking.pb.go:417 |

## proto-cosmos-staking-v1beta1-dvpairs-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvpairs-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:173</code> | <code>message cosmos.staking.v1beta1.DVPairs { repeated DVPair pairs = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvpairs-j39</code> | notes | Go type DVPairs at sdk/x/staking/types/staking.pb.go:455 |

## proto-cosmos-staking-v1beta1-dvvtriplet-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvvtriplet-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:181</code> | <code>message cosmos.staking.v1beta1.DVVTriplet { string delegator_address = 1; string validator_src_address = 2; string validator_dst_address = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvvtriplet-j39</code> | notes | Go type DVVTriplet at sdk/x/staking/types/staking.pb.go:503 |

## proto-cosmos-staking-v1beta1-dvvtriplets-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvvtriplets-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:192</code> | <code>message cosmos.staking.v1beta1.DVVTriplets { repeated DVVTriplet triplets = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-dvvtriplets-j39</code> | notes | Go type DVVTriplets at sdk/x/staking/types/staking.pb.go:542 |

## proto-cosmos-staking-v1beta1-genesisstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-genesisstate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/genesis.proto:10</code> | <code>message cosmos.staking.v1beta1.GenesisState { Params params = 1; bytes last_total_power = 2; repeated LastValidatorPower last_validator_powers = 3; repeated Validator validators = 4; repeated Delegation delegations = 5; repeated UnbondingDelegation unbonding_delegations = 6; repeated Redelegation redelegations = 7; bool exported = 8; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-genesisstate-j39</code> | notes | Go type GenesisState at sdk/x/staking/types/genesis.pb.go:28 |

## proto-cosmos-staking-v1beta1-historicalinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-historicalinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:18</code> | <code>message cosmos.staking.v1beta1.HistoricalInfo { tendermint.types.Header header = 1; repeated Validator valset = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-historicalinfo-j39</code> | notes | Go type HistoricalInfo at sdk/x/staking/types/staking.pb.go:82 |

## proto-cosmos-staking-v1beta1-lastvalidatorpower-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-lastvalidatorpower-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/genesis.proto:48</code> | <code>message cosmos.staking.v1beta1.LastValidatorPower { string address = 1; int64 power = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-lastvalidatorpower-j39</code> | notes | Go type LastValidatorPower at sdk/x/staking/types/genesis.pb.go:131 |

## proto-cosmos-staking-v1beta1-msgbeginredelegate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgbeginredelegate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:95</code> | <code>message cosmos.staking.v1beta1.MsgBeginRedelegate { string delegator_address = 1; string validator_src_address = 2; string validator_dst_address = 3; cosmos.base.v1beta1.Coin amount = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgbeginredelegate-j39</code> | notes | Go type MsgBeginRedelegate at sdk/x/staking/types/tx.pb.go:303 |

## proto-cosmos-staking-v1beta1-msgbeginredelegateresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgbeginredelegateresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:105</code> | <code>message cosmos.staking.v1beta1.MsgBeginRedelegateResponse { google.protobuf.Timestamp completion_time = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgbeginredelegateresponse-j39</code> | notes | Go type MsgBeginRedelegateResponse at sdk/x/staking/types/tx.pb.go:372 |

## proto-cosmos-staking-v1beta1-msgcreatevalidator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgcreatevalidator-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:35</code> | <code>message cosmos.staking.v1beta1.MsgCreateValidator { Description description = 1; CommissionRates commission = 2; string min_self_delegation = 3; string delegator_address = 4; string validator_address = 5; google.protobuf.Any pubkey = 6; cosmos.base.v1beta1.Coin value = 7; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgcreatevalidator-j39</code> | notes | Go type MsgCreateValidator at sdk/x/staking/types/tx.pb.go:40 |

## proto-cosmos-staking-v1beta1-msgcreatevalidatorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgcreatevalidatorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:53</code> | <code>message cosmos.staking.v1beta1.MsgCreateValidatorResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgcreatevalidatorresponse-j39</code> | notes | Go type MsgCreateValidatorResponse at sdk/x/staking/types/tx.pb.go:84 |

## proto-cosmos-staking-v1beta1-msgdelegate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgdelegate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:82</code> | <code>message cosmos.staking.v1beta1.MsgDelegate { string delegator_address = 1; string validator_address = 2; cosmos.base.v1beta1.Coin amount = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgdelegate-j39</code> | notes | Go type MsgDelegate at sdk/x/staking/types/tx.pb.go:204; 26 referencing files, first 25 listed |

## proto-cosmos-staking-v1beta1-msgdelegateresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgdelegateresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:91</code> | <code>message cosmos.staking.v1beta1.MsgDelegateResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgdelegateresponse-j39</code> | notes | Go type MsgDelegateResponse at sdk/x/staking/types/tx.pb.go:265 |

## proto-cosmos-staking-v1beta1-msgeditvalidator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgeditvalidator-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:56</code> | <code>message cosmos.staking.v1beta1.MsgEditValidator { Description description = 1; string validator_address = 2; string commission_rate = 3; string min_self_delegation = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgeditvalidator-j39</code> | notes | Go type MsgEditValidator at sdk/x/staking/types/tx.pb.go:121 |

## proto-cosmos-staking-v1beta1-msgeditvalidatorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgeditvalidatorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:78</code> | <code>message cosmos.staking.v1beta1.MsgEditValidatorResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgeditvalidatorresponse-j39</code> | notes | Go type MsgEditValidatorResponse at sdk/x/staking/types/tx.pb.go:166 |

## proto-cosmos-staking-v1beta1-msgundelegate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgundelegate-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:114</code> | <code>message cosmos.staking.v1beta1.MsgUndelegate { string delegator_address = 1; string validator_address = 2; cosmos.base.v1beta1.Coin amount = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgundelegate-j39</code> | notes | Go type MsgUndelegate at sdk/x/staking/types/tx.pb.go:418; 26 referencing files, first 25 listed |

## proto-cosmos-staking-v1beta1-msgundelegateresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgundelegateresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/tx.proto:123</code> | <code>message cosmos.staking.v1beta1.MsgUndelegateResponse { google.protobuf.Timestamp completion_time = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-msgundelegateresponse-j39</code> | notes | Go type MsgUndelegateResponse at sdk/x/staking/types/tx.pb.go:479 |

## proto-cosmos-staking-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-params-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:300</code> | <code>message cosmos.staking.v1beta1.Params { google.protobuf.Duration unbonding_time = 1; uint32 max_validators = 2; uint32 max_entries = 3; uint32 historical_entries = 4; string bond_denom = 5; string min_commission_rate = 6; string max_voting_power_ratio = 7; string max_voting_power_enforcement_threshold = 8; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-params-j39</code> | notes | Go type Params at sdk/x/staking/types/staking.pb.go:835 |

## proto-cosmos-staking-v1beta1-pool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-pool-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:376</code> | <code>message cosmos.staking.v1beta1.Pool { string not_bonded_tokens = 1; string bonded_tokens = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-pool-j39</code> | notes | Go type Pool at sdk/x/staking/types/staking.pb.go:1079 |

## proto-cosmos-staking-v1beta1-querydelegationrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegationrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:170</code> | <code>message cosmos.staking.v1beta1.QueryDelegationRequest { string delegator_addr = 1; string validator_addr = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegationrequest-j39</code> | notes | Go type QueryDelegationRequest at sdk/x/staking/types/query.pb.go:466 |

## proto-cosmos-staking-v1beta1-querydelegationresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegationresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:182</code> | <code>message cosmos.staking.v1beta1.QueryDelegationResponse { DelegationResponse delegation_response = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegationresponse-j39</code> | notes | Go type QueryDelegationResponse at sdk/x/staking/types/query.pb.go:507 |

## proto-cosmos-staking-v1beta1-querydelegatordelegationsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatordelegationsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:209</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorDelegationsRequest { string delegator_addr = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatordelegationsrequest-j39</code> | notes | Go type QueryDelegatorDelegationsRequest at sdk/x/staking/types/query.pb.go:643 |

## proto-cosmos-staking-v1beta1-querydelegatordelegationsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatordelegationsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:222</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorDelegationsResponse { repeated DelegationResponse delegation_responses = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatordelegationsresponse-j39</code> | notes | Go type QueryDelegatorDelegationsResponse at sdk/x/staking/types/query.pb.go:685 |

## proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:232</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorUnbondingDelegationsRequest { string delegator_addr = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsrequest-j39</code> | notes | Go type QueryDelegatorUnbondingDelegationsRequest at sdk/x/staking/types/query.pb.go:741 |

## proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:245</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorUnbondingDelegationsResponse { repeated UnbondingDelegation unbonding_responses = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorunbondingdelegationsresponse-j39</code> | notes | Go type QueryDelegatorUnbondingDelegationsResponse at sdk/x/staking/types/query.pb.go:787 |

## proto-cosmos-staking-v1beta1-querydelegatorvalidatorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:305</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorValidatorRequest { string delegator_addr = 1; string validator_addr = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorrequest-j39</code> | notes | Go type QueryDelegatorValidatorRequest at sdk/x/staking/types/query.pb.go:1045 |

## proto-cosmos-staking-v1beta1-querydelegatorvalidatorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:318</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorValidatorResponse { Validator validator = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorresponse-j39</code> | notes | Go type QueryDelegatorValidatorResponse at sdk/x/staking/types/query.pb.go:1087 |

## proto-cosmos-staking-v1beta1-querydelegatorvalidatorsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:282</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorValidatorsRequest { string delegator_addr = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorsrequest-j39</code> | notes | Go type QueryDelegatorValidatorsRequest at sdk/x/staking/types/query.pb.go:947 |

## proto-cosmos-staking-v1beta1-querydelegatorvalidatorsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:295</code> | <code>message cosmos.staking.v1beta1.QueryDelegatorValidatorsResponse { repeated Validator validators = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querydelegatorvalidatorsresponse-j39</code> | notes | Go type QueryDelegatorValidatorsResponse at sdk/x/staking/types/query.pb.go:989 |

## proto-cosmos-staking-v1beta1-queryhistoricalinforequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryhistoricalinforequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:325</code> | <code>message cosmos.staking.v1beta1.QueryHistoricalInfoRequest { int64 height = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryhistoricalinforequest-j39</code> | notes | Go type QueryHistoricalInfoRequest at sdk/x/staking/types/query.pb.go:1134 |

## proto-cosmos-staking-v1beta1-queryhistoricalinforesponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryhistoricalinforesponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:332</code> | <code>message cosmos.staking.v1beta1.QueryHistoricalInfoResponse { HistoricalInfo hist = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryhistoricalinforesponse-j39</code> | notes | Go type QueryHistoricalInfoResponse at sdk/x/staking/types/query.pb.go:1181 |

## proto-cosmos-staking-v1beta1-queryparamsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryparamsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:347</code> | <code>message cosmos.staking.v1beta1.QueryParamsRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryparamsrequest-j39</code> | notes | Go type QueryParamsRequest at sdk/x/staking/types/query.pb.go:1310 |

## proto-cosmos-staking-v1beta1-queryparamsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryparamsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:350</code> | <code>message cosmos.staking.v1beta1.QueryParamsResponse { Params params = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryparamsresponse-j39</code> | notes | Go type QueryParamsResponse at sdk/x/staking/types/query.pb.go:1347 |

## proto-cosmos-staking-v1beta1-querypoolrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querypoolrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:338</code> | <code>message cosmos.staking.v1beta1.QueryPoolRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querypoolrequest-j39</code> | notes | Go type QueryPoolRequest at sdk/x/staking/types/query.pb.go:1227 |

## proto-cosmos-staking-v1beta1-querypoolresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querypoolresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:341</code> | <code>message cosmos.staking.v1beta1.QueryPoolResponse { Pool pool = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-querypoolresponse-j39</code> | notes | Go type QueryPoolResponse at sdk/x/staking/types/query.pb.go:1264 |

## proto-cosmos-staking-v1beta1-queryredelegationsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryredelegationsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:254</code> | <code>message cosmos.staking.v1beta1.QueryRedelegationsRequest { string delegator_addr = 1; string src_validator_addr = 2; string dst_validator_addr = 3; cosmos.base.query.v1beta1.PageRequest pagination = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryredelegationsrequest-j39</code> | notes | Go type QueryRedelegationsRequest at sdk/x/staking/types/query.pb.go:846 |

## proto-cosmos-staking-v1beta1-queryredelegationsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryredelegationsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:273</code> | <code>message cosmos.staking.v1beta1.QueryRedelegationsResponse { repeated RedelegationResponse redelegation_responses = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryredelegationsresponse-j39</code> | notes | Go type QueryRedelegationsResponse at sdk/x/staking/types/query.pb.go:892 |

## proto-cosmos-staking-v1beta1-queryunbondingdelegationrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryunbondingdelegationrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:189</code> | <code>message cosmos.staking.v1beta1.QueryUnbondingDelegationRequest { string delegator_addr = 1; string validator_addr = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryunbondingdelegationrequest-j39</code> | notes | Go type QueryUnbondingDelegationRequest at sdk/x/staking/types/query.pb.go:554 |

## proto-cosmos-staking-v1beta1-queryunbondingdelegationresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryunbondingdelegationresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:202</code> | <code>message cosmos.staking.v1beta1.QueryUnbondingDelegationResponse { UnbondingDelegation unbond = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryunbondingdelegationresponse-j39</code> | notes | Go type QueryUnbondingDelegationResponse at sdk/x/staking/types/query.pb.go:596 |

## proto-cosmos-staking-v1beta1-queryvalidatordelegationsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatordelegationsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:130</code> | <code>message cosmos.staking.v1beta1.QueryValidatorDelegationsRequest { string validator_addr = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatordelegationsrequest-j39</code> | notes | Go type QueryValidatorDelegationsRequest at sdk/x/staking/types/query.pb.go:237 |

## proto-cosmos-staking-v1beta1-queryvalidatordelegationsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatordelegationsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:140</code> | <code>message cosmos.staking.v1beta1.QueryValidatorDelegationsResponse { repeated DelegationResponse delegation_responses = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatordelegationsresponse-j39</code> | notes | Go type QueryValidatorDelegationsResponse at sdk/x/staking/types/query.pb.go:293 |

## proto-cosmos-staking-v1beta1-queryvalidatorrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:117</code> | <code>message cosmos.staking.v1beta1.QueryValidatorRequest { string validator_addr = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorrequest-j39</code> | notes | Go type QueryValidatorRequest at sdk/x/staking/types/query.pb.go:144 |

## proto-cosmos-staking-v1beta1-queryvalidatorresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:123</code> | <code>message cosmos.staking.v1beta1.QueryValidatorResponse { Validator validator = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorresponse-j39</code> | notes | Go type QueryValidatorResponse at sdk/x/staking/types/query.pb.go:190 |

## proto-cosmos-staking-v1beta1-queryvalidatorsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:99</code> | <code>message cosmos.staking.v1beta1.QueryValidatorsRequest { string status = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorsrequest-j39</code> | notes | Go type QueryValidatorsRequest at sdk/x/staking/types/query.pb.go:34 |

## proto-cosmos-staking-v1beta1-queryvalidatorsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:108</code> | <code>message cosmos.staking.v1beta1.QueryValidatorsResponse { repeated Validator validators = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorsresponse-j39</code> | notes | Go type QueryValidatorsResponse at sdk/x/staking/types/query.pb.go:89 |

## proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:152</code> | <code>message cosmos.staking.v1beta1.QueryValidatorUnbondingDelegationsRequest { string validator_addr = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsrequest-j39</code> | notes | Go type QueryValidatorUnbondingDelegationsRequest at sdk/x/staking/types/query.pb.go:348 |

## proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:162</code> | <code>message cosmos.staking.v1beta1.QueryValidatorUnbondingDelegationsResponse { repeated UnbondingDelegation unbonding_responses = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-queryvalidatorunbondingdelegationsresponse-j39</code> | notes | Go type QueryValidatorUnbondingDelegationsResponse at sdk/x/staking/types/query.pb.go:408 |

## proto-cosmos-staking-v1beta1-redelegation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegation-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:284</code> | <code>message cosmos.staking.v1beta1.Redelegation { string delegator_address = 1; string validator_src_address = 2; string validator_dst_address = 3; repeated RedelegationEntry entries = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegation-j39</code> | notes | Go type Redelegation at sdk/x/staking/types/staking.pb.go:791 |

## proto-cosmos-staking-v1beta1-redelegationentry-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegationentry-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:257</code> | <code>message cosmos.staking.v1beta1.RedelegationEntry { int64 creation_height = 1; google.protobuf.Timestamp completion_time = 2; string initial_balance = 3; string shares_dst = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegationentry-j39</code> | notes | Go type RedelegationEntry at sdk/x/staking/types/staking.pb.go:732 |

## proto-cosmos-staking-v1beta1-redelegationentryresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegationentryresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:354</code> | <code>message cosmos.staking.v1beta1.RedelegationEntryResponse { RedelegationEntry redelegation_entry = 1; string balance = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegationentryresponse-j39</code> | notes | Go type RedelegationEntryResponse at sdk/x/staking/types/staking.pb.go:977 |

## proto-cosmos-staking-v1beta1-redelegationresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegationresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:367</code> | <code>message cosmos.staking.v1beta1.RedelegationResponse { Redelegation redelegation = 1; repeated RedelegationEntryResponse entries = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-redelegationresponse-j39</code> | notes | Go type RedelegationResponse at sdk/x/staking/types/staking.pb.go:1025 |

## proto-cosmos-staking-v1beta1-stakeauthorization-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-stakeauthorization-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/authz.proto:13</code> | <code>message cosmos.staking.v1beta1.StakeAuthorization { cosmos.base.v1beta1.Coin max_tokens = 1; Validators allow_list = 2; Validators deny_list = 3; AuthorizationType authorization_type = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-stakeauthorization-j39</code> | notes | Go type StakeAuthorization at sdk/x/staking/types/authz.pb.go:69 |

## proto-cosmos-staking-v1beta1-stakeauthorization-validators-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-stakeauthorization-validators-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/authz.proto:28</code> | <code>message cosmos.staking.v1beta1.StakeAuthorization.Validators { repeated string address = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-stakeauthorization-validators-j39</code> | notes | Go type StakeAuthorization_Validators at sdk/x/staking/types/authz.pb.go:176 |

## proto-cosmos-staking-v1beta1-unbondingdelegation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-unbondingdelegation-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:217</code> | <code>message cosmos.staking.v1beta1.UnbondingDelegation { string delegator_address = 1; string validator_address = 2; repeated UnbondingDelegationEntry entries = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-unbondingdelegation-j39</code> | notes | Go type UnbondingDelegation at sdk/x/staking/types/staking.pb.go:632 |

## proto-cosmos-staking-v1beta1-unbondingdelegationentry-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-unbondingdelegationentry-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:231</code> | <code>message cosmos.staking.v1beta1.UnbondingDelegationEntry { int64 creation_height = 1; google.protobuf.Timestamp completion_time = 2; string initial_balance = 3; string balance = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-unbondingdelegationentry-j39</code> | notes | Go type UnbondingDelegationEntry at sdk/x/staking/types/staking.pb.go:674 |

## proto-cosmos-staking-v1beta1-valaddresses-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-valaddresses-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:153</code> | <code>message cosmos.staking.v1beta1.ValAddresses { repeated string addresses = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-valaddresses-j39</code> | notes | Go type ValAddresses at sdk/x/staking/types/staking.pb.go:371 |

## proto-cosmos-staking-v1beta1-validator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-validator-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/staking.proto:91</code> | <code>message cosmos.staking.v1beta1.Validator { string operator_address = 1; google.protobuf.Any consensus_pubkey = 2; bool jailed = 3; BondStatus status = 4; string tokens = 5; string delegator_shares = 6; Description description = 7; int64 unbonding_height = 8; google.protobuf.Timestamp unbonding_time = 9; Commission commission = 10; string min_self_delegation = 11; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-staking-v1beta1-validator-j39</code> | notes | Go type Validator at sdk/x/staking/types/staking.pb.go:313; 48 referencing files, first 25 listed |

## proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/signing/v1beta1/signing.proto:63</code> | <code>message cosmos.tx.signing.v1beta1.SignatureDescriptor.Data { Single single = 1; Multi multi = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-j39</code> | notes | Go type SignatureDescriptor_Data at sdk/types/tx/signing/signing.pb.go:196 |

## proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-multi-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-multi-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/signing/v1beta1/signing.proto:83</code> | <code>message cosmos.tx.signing.v1beta1.SignatureDescriptor.Data.Multi { cosmos.crypto.multisig.v1beta1.CompactBitArray bitarray = 1; repeated Data signatures = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-multi-j39</code> | notes | Go type SignatureDescriptor_Data_Multi at sdk/types/tx/signing/signing.pb.go:340 |

## proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-single-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-single-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/signing/v1beta1/signing.proto:74</code> | <code>message cosmos.tx.signing.v1beta1.SignatureDescriptor.Data.Single { SignMode mode = 1; bytes signature = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-data-single-j39</code> | notes | Go type SignatureDescriptor_Data_Single at sdk/types/tx/signing/signing.pb.go:285 |

## proto-cosmos-tx-signing-v1beta1-signaturedescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/signing/v1beta1/signing.proto:51</code> | <code>message cosmos.tx.signing.v1beta1.SignatureDescriptor { google.protobuf.Any public_key = 1; Data data = 2; uint64 sequence = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptor-j39</code> | notes | Go type SignatureDescriptor at sdk/types/tx/signing/signing.pb.go:131 |

## proto-cosmos-tx-signing-v1beta1-signaturedescriptors-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptors-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/signing/v1beta1/signing.proto:42</code> | <code>message cosmos.tx.signing.v1beta1.SignatureDescriptors { repeated SignatureDescriptor signatures = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signaturedescriptors-j39</code> | notes | Go type SignatureDescriptors at sdk/types/tx/signing/signing.pb.go:82 |

## proto-cosmos-tx-signing-v1beta1-signmode-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signmode-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/signing/v1beta1/signing.proto:10</code> | <code>enum cosmos.tx.signing.v1beta1.SignMode { SIGN_MODE_UNSPECIFIED = 0; SIGN_MODE_DIRECT = 1; SIGN_MODE_TEXTUAL = 2; SIGN_MODE_LEGACY_AMINO_JSON = 127; SIGN_MODE_EIP_191 = 191; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-signing-v1beta1-signmode-j39</code> | notes | Go enum SignMode at sdk/types/tx/signing/signing.pb.go:28 |

## proto-cosmos-tx-v1beta1-authinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-authinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:99</code> | <code>message cosmos.tx.v1beta1.AuthInfo { repeated SignerInfo signer_infos = 1; Fee fee = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-authinfo-j39</code> | notes | Go type AuthInfo at sdk/types/tx/tx.pb.go:344 |

## proto-cosmos-tx-v1beta1-broadcastmode-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-broadcastmode-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:87</code> | <code>enum cosmos.tx.v1beta1.BroadcastMode { BROADCAST_MODE_UNSPECIFIED = 0; BROADCAST_MODE_BLOCK = 1; BROADCAST_MODE_SYNC = 2; BROADCAST_MODE_ASYNC = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-broadcastmode-j39</code> | notes | Go enum BroadcastMode at sdk/types/tx/service.pb.go:70 |

## proto-cosmos-tx-v1beta1-broadcasttxrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-broadcasttxrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:80</code> | <code>message cosmos.tx.v1beta1.BroadcastTxRequest { bytes tx_bytes = 1; BroadcastMode mode = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-broadcasttxrequest-j39</code> | notes | Go type BroadcastTxRequest at sdk/types/tx/service.pb.go:239 |

## proto-cosmos-tx-v1beta1-broadcasttxresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-broadcasttxresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:103</code> | <code>message cosmos.tx.v1beta1.BroadcastTxResponse { cosmos.base.abci.v1beta1.TxResponse tx_response = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-broadcasttxresponse-j39</code> | notes | Go type BroadcastTxResponse at sdk/types/tx/service.pb.go:294 |

## proto-cosmos-tx-v1beta1-fee-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-fee-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:165</code> | <code>message cosmos.tx.v1beta1.Fee { repeated cosmos.base.v1beta1.Coin amount = 1; uint64 gas_limit = 2; string payer = 3; string granter = 4; uint64 gas_estimate = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-fee-j39</code> | notes | Go type Fee at sdk/types/tx/tx.pb.go:670 |

## proto-cosmos-tx-v1beta1-getblockwithtxsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-getblockwithtxsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:148</code> | <code>message cosmos.tx.v1beta1.GetBlockWithTxsRequest { int64 height = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-getblockwithtxsrequest-j39</code> | notes | Go type GetBlockWithTxsRequest at sdk/types/tx/service.pb.go:561 |

## proto-cosmos-tx-v1beta1-getblockwithtxsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-getblockwithtxsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:158</code> | <code>message cosmos.tx.v1beta1.GetBlockWithTxsResponse { repeated cosmos.tx.v1beta1.Tx txs = 1; tendermint.types.BlockID block_id = 2; tendermint.types.Block block = 3; cosmos.base.query.v1beta1.PageResponse pagination = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-getblockwithtxsresponse-j39</code> | notes | Go type GetBlockWithTxsResponse at sdk/types/tx/service.pb.go:618 |

## proto-cosmos-tx-v1beta1-gettxrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:131</code> | <code>message cosmos.tx.v1beta1.GetTxRequest { string hash = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxrequest-j39</code> | notes | Go type GetTxRequest at sdk/types/tx/service.pb.go:457 |

## proto-cosmos-tx-v1beta1-gettxresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:137</code> | <code>message cosmos.tx.v1beta1.GetTxResponse { cosmos.tx.v1beta1.Tx tx = 1; cosmos.base.abci.v1beta1.TxResponse tx_response = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxresponse-j39</code> | notes | Go type GetTxResponse at sdk/types/tx/service.pb.go:503 |

## proto-cosmos-tx-v1beta1-gettxseventrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxseventrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:49</code> | <code>message cosmos.tx.v1beta1.GetTxsEventRequest { repeated string events = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; OrderBy order_by = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxseventrequest-j39</code> | notes | Go type GetTxsEventRequest at sdk/types/tx/service.pb.go:110 |

## proto-cosmos-tx-v1beta1-gettxseventresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxseventresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:69</code> | <code>message cosmos.tx.v1beta1.GetTxsEventResponse { repeated cosmos.tx.v1beta1.Tx txs = 1; repeated cosmos.base.abci.v1beta1.TxResponse tx_responses = 2; cosmos.base.query.v1beta1.PageResponse pagination = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-gettxseventresponse-j39</code> | notes | Go type GetTxsEventResponse at sdk/types/tx/service.pb.go:174 |

## proto-cosmos-tx-v1beta1-modeinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-modeinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:132</code> | <code>message cosmos.tx.v1beta1.ModeInfo { Single single = 1; Multi multi = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-modeinfo-j39</code> | notes | Go type ModeInfo at sdk/types/tx/tx.pb.go:475 |

## proto-cosmos-tx-v1beta1-modeinfo-multi-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-modeinfo-multi-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:152</code> | <code>message cosmos.tx.v1beta1.ModeInfo.Multi { cosmos.crypto.multisig.v1beta1.CompactBitArray bitarray = 1; repeated ModeInfo mode_infos = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-modeinfo-multi-j39</code> | notes | Go type ModeInfo_Multi at sdk/types/tx/tx.pb.go:612 |

## proto-cosmos-tx-v1beta1-modeinfo-single-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-modeinfo-single-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:146</code> | <code>message cosmos.tx.v1beta1.ModeInfo.Single { cosmos.tx.signing.v1beta1.SignMode mode = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-modeinfo-single-j39</code> | notes | Go type ModeInfo_Single at sdk/types/tx/tx.pb.go:566 |

## proto-cosmos-tx-v1beta1-orderby-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-orderby-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:58</code> | <code>enum cosmos.tx.v1beta1.OrderBy { ORDER_BY_UNSPECIFIED = 0; ORDER_BY_ASC = 1; ORDER_BY_DESC = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-orderby-j39</code> | notes | Go enum OrderBy at sdk/types/tx/service.pb.go:38 |

## proto-cosmos-tx-v1beta1-signdoc-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-signdoc-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:48</code> | <code>message cosmos.tx.v1beta1.SignDoc { bytes body_bytes = 1; bytes auth_info_bytes = 2; string chain_id = 3; uint64 account_number = 4; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-signdoc-j39</code> | notes | Go type SignDoc at sdk/types/tx/tx.pb.go:171 |

## proto-cosmos-tx-v1beta1-signerinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-signerinfo-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:115</code> | <code>message cosmos.tx.v1beta1.SignerInfo { google.protobuf.Any public_key = 1; ModeInfo mode_info = 2; uint64 sequence = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-signerinfo-j39</code> | notes | Go type SignerInfo at sdk/types/tx/tx.pb.go:406 |

## proto-cosmos-tx-v1beta1-simulaterequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-simulaterequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:110</code> | <code>message cosmos.tx.v1beta1.SimulateRequest { cosmos.tx.v1beta1.Tx tx = 1; bytes tx_bytes = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-simulaterequest-j39</code> | notes | Go type SimulateRequest at sdk/types/tx/service.pb.go:341 |

## proto-cosmos-tx-v1beta1-simulateresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-simulateresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:122</code> | <code>message cosmos.tx.v1beta1.SimulateResponse { cosmos.base.abci.v1beta1.GasInfo gas_info = 1; cosmos.base.abci.v1beta1.Result result = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-simulateresponse-j39</code> | notes | Go type SimulateResponse at sdk/types/tx/service.pb.go:401 |

## proto-cosmos-tx-v1beta1-tx-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-tx-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:13</code> | <code>message cosmos.tx.v1beta1.Tx { TxBody body = 1; AuthInfo auth_info = 2; repeated bytes signatures = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-tx-j39</code> | notes | Go type Tx at sdk/types/tx/tx.pb.go:32 |

## proto-cosmos-tx-v1beta1-txbody-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-txbody-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:67</code> | <code>message cosmos.tx.v1beta1.TxBody { repeated google.protobuf.Any messages = 1; string memo = 2; uint64 timeout_height = 3; repeated google.protobuf.Any extension_options = 1023; repeated google.protobuf.Any non_critical_extension_options = 2047; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-txbody-j39</code> | notes | Go type TxBody at sdk/types/tx/tx.pb.go:248 |

## proto-cosmos-tx-v1beta1-txraw-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-txraw-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/tx.proto:32</code> | <code>message cosmos.tx.v1beta1.TxRaw { bytes body_bytes = 1; bytes auth_info_bytes = 2; repeated bytes signatures = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-tx-v1beta1-txraw-j39</code> | notes | Go type TxRaw at sdk/types/tx/tx.pb.go:103 |

## proto-cosmos-upgrade-v1beta1-cancelsoftwareupgradeproposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-cancelsoftwareupgradeproposal-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto:64</code> | <code>message cosmos.upgrade.v1beta1.CancelSoftwareUpgradeProposal { string title = 1; string description = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-cancelsoftwareupgradeproposal-j39</code> | notes | Go type CancelSoftwareUpgradeProposal at sdk/x/upgrade/types/upgrade.pb.go:131 |

## proto-cosmos-upgrade-v1beta1-moduleversion-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-moduleversion-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto:75</code> | <code>message cosmos.upgrade.v1beta1.ModuleVersion { string name = 1; uint64 version = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-moduleversion-j39</code> | notes | Go type ModuleVersion at sdk/x/upgrade/types/upgrade.pb.go:171 |

## proto-cosmos-upgrade-v1beta1-plan-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-plan-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto:12</code> | <code>message cosmos.upgrade.v1beta1.Plan { string name = 1; google.protobuf.Timestamp time = 2; int64 height = 3; string info = 4; google.protobuf.Any upgraded_client_state = 5; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-plan-j39</code> | notes | Go type Plan at sdk/x/upgrade/types/upgrade.pb.go:32; 47 referencing files, first 25 listed |

## proto-cosmos-upgrade-v1beta1-queryappliedplanrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryappliedplanrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:53</code> | <code>message cosmos.upgrade.v1beta1.QueryAppliedPlanRequest { string name = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryappliedplanrequest-j39</code> | notes | Go type QueryAppliedPlanRequest at sdk/x/upgrade/types/query.pb.go:118 |

## proto-cosmos-upgrade-v1beta1-queryappliedplanresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryappliedplanresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:60</code> | <code>message cosmos.upgrade.v1beta1.QueryAppliedPlanResponse { int64 height = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryappliedplanresponse-j39</code> | notes | Go type QueryAppliedPlanResponse at sdk/x/upgrade/types/query.pb.go:165 |

## proto-cosmos-upgrade-v1beta1-querycurrentplanrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querycurrentplanrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:42</code> | <code>message cosmos.upgrade.v1beta1.QueryCurrentPlanRequest {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querycurrentplanrequest-j39</code> | notes | Go type QueryCurrentPlanRequest at sdk/x/upgrade/types/query.pb.go:33 |

## proto-cosmos-upgrade-v1beta1-querycurrentplanresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querycurrentplanresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:46</code> | <code>message cosmos.upgrade.v1beta1.QueryCurrentPlanResponse { Plan plan = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querycurrentplanresponse-j39</code> | notes | Go type QueryCurrentPlanResponse at sdk/x/upgrade/types/query.pb.go:71 |

## proto-cosmos-upgrade-v1beta1-querymoduleversionsrequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querymoduleversionsrequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:89</code> | <code>message cosmos.upgrade.v1beta1.QueryModuleVersionsRequest { string module_name = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querymoduleversionsrequest-j39</code> | notes | Go type QueryModuleVersionsRequest at sdk/x/upgrade/types/query.pb.go:313 |

## proto-cosmos-upgrade-v1beta1-querymoduleversionsresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querymoduleversionsresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:100</code> | <code>message cosmos.upgrade.v1beta1.QueryModuleVersionsResponse { repeated ModuleVersion module_versions = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-querymoduleversionsresponse-j39</code> | notes | Go type QueryModuleVersionsResponse at sdk/x/upgrade/types/query.pb.go:364 |

## proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstaterequest-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstaterequest-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:67</code> | <code>message cosmos.upgrade.v1beta1.QueryUpgradedConsensusStateRequest { int64 last_height = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstaterequest-j39</code> | notes | Go type QueryUpgradedConsensusStateRequest at sdk/x/upgrade/types/query.pb.go:214 |

## proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstateresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstateresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:77</code> | <code>message cosmos.upgrade.v1beta1.QueryUpgradedConsensusStateResponse { bytes upgraded_consensus_state = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-queryupgradedconsensusstateresponse-j39</code> | notes | Go type QueryUpgradedConsensusStateResponse at sdk/x/upgrade/types/query.pb.go:264 |

## proto-cosmos-upgrade-v1beta1-softwareupgradeproposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-softwareupgradeproposal-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/upgrade.proto:53</code> | <code>message cosmos.upgrade.v1beta1.SoftwareUpgradeProposal { string title = 1; string description = 2; Plan plan = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-upgrade-v1beta1-softwareupgradeproposal-j39</code> | notes | Go type SoftwareUpgradeProposal at sdk/x/upgrade/types/upgrade.pb.go:91 |

## proto-cosmos-vesting-v1beta1-basevestingaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-basevestingaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/vesting.proto:12</code> | <code>message cosmos.vesting.v1beta1.BaseVestingAccount { cosmos.auth.v1beta1.BaseAccount base_account = 1; repeated cosmos.base.v1beta1.Coin original_vesting = 2; repeated cosmos.base.v1beta1.Coin delegated_free = 3; repeated cosmos.base.v1beta1.Coin delegated_vesting = 4; int64 end_time = 5; string admin = 6; int64 cancelled_time = 7; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-basevestingaccount-j39</code> | notes | Go type BaseVestingAccount at sdk/x/auth/vesting/types/vesting.pb.go:31 |

## proto-cosmos-vesting-v1beta1-continuousvestingaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-continuousvestingaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/vesting.proto:41</code> | <code>message cosmos.vesting.v1beta1.ContinuousVestingAccount { BaseVestingAccount base_vesting_account = 1; int64 start_time = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-continuousvestingaccount-j39</code> | notes | Go type ContinuousVestingAccount at sdk/x/auth/vesting/types/vesting.pb.go:77 |

## proto-cosmos-vesting-v1beta1-delayedvestingaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-delayedvestingaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/vesting.proto:52</code> | <code>message cosmos.vesting.v1beta1.DelayedVestingAccount { BaseVestingAccount base_vesting_account = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-delayedvestingaccount-j39</code> | notes | Go type DelayedVestingAccount at sdk/x/auth/vesting/types/vesting.pb.go:117 |

## proto-cosmos-vesting-v1beta1-msgcreatevestingaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-msgcreatevestingaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/tx.proto:18</code> | <code>message cosmos.vesting.v1beta1.MsgCreateVestingAccount { string from_address = 1; string to_address = 2; repeated cosmos.base.v1beta1.Coin amount = 3; int64 end_time = 4; bool delayed = 5; string admin = 6; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-msgcreatevestingaccount-j39</code> | notes | Go type MsgCreateVestingAccount at sdk/x/auth/vesting/types/tx.pb.go:35 |

## proto-cosmos-vesting-v1beta1-msgcreatevestingaccountresponse-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-msgcreatevestingaccountresponse-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/tx.proto:34</code> | <code>message cosmos.vesting.v1beta1.MsgCreateVestingAccountResponse {  }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-msgcreatevestingaccountresponse-j39</code> | notes | Go type MsgCreateVestingAccountResponse at sdk/x/auth/vesting/types/tx.pb.go:120 |

## proto-cosmos-vesting-v1beta1-period-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-period-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/vesting.proto:60</code> | <code>message cosmos.vesting.v1beta1.Period { int64 length = 1; repeated cosmos.base.v1beta1.Coin amount = 2; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-period-j39</code> | notes | Go type Period at sdk/x/auth/vesting/types/vesting.pb.go:154 |

## proto-cosmos-vesting-v1beta1-periodicvestingaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-periodicvestingaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/vesting.proto:72</code> | <code>message cosmos.vesting.v1beta1.PeriodicVestingAccount { BaseVestingAccount base_vesting_account = 1; int64 start_time = 2; repeated Period vesting_periods = 3; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-periodicvestingaccount-j39</code> | notes | Go type PeriodicVestingAccount at sdk/x/auth/vesting/types/vesting.pb.go:207 |

## proto-cosmos-vesting-v1beta1-permanentlockedaccount-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-permanentlockedaccount-j39</code> | <code>proto_message</code> | <code>stable</code> | <code>sdk/proto/cosmos/vesting/v1beta1/vesting.proto:89</code> | <code>message cosmos.vesting.v1beta1.PermanentLockedAccount { BaseVestingAccount base_vesting_account = 1; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.proto-cosmos-vesting-v1beta1-permanentlockedaccount-j39</code> | notes | Go type PermanentLockedAccount at sdk/x/auth/vesting/types/vesting.pb.go:250 |

## qry-cosmos-auth-v1beta1-account-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-account-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:23</code> | <code>cosmos.auth.v1beta1.Query/Account: rpc Account(QueryAccountRequest) returns (QueryAccountResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-account-j39</code> | notes | handler sdk/x/auth/keeper/grpc_query.go:48; callers are references to request type QueryAccountRequest |

## qry-cosmos-auth-v1beta1-accounts-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-accounts-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:18</code> | <code>cosmos.auth.v1beta1.Query/Accounts: rpc Accounts(QueryAccountsRequest) returns (QueryAccountsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-accounts-j39</code> | notes | handler sdk/x/auth/keeper/grpc_query.go:19; callers are references to request type QueryAccountsRequest |

## qry-cosmos-auth-v1beta1-nextaccountnumber-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-nextaccountnumber-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:33</code> | <code>cosmos.auth.v1beta1.Query/NextAccountNumber: rpc NextAccountNumber(QueryNextAccountNumberRequest) returns (QueryNextAccountNumberResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-nextaccountnumber-j39</code> | notes | handler sdk/x/auth/keeper/grpc_query.go:87; callers are references to request type QueryNextAccountNumberRequest |

## qry-cosmos-auth-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-params-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:28</code> | <code>cosmos.auth.v1beta1.Query/Params: rpc Params(QueryParamsRequest) returns (QueryParamsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-auth-v1beta1-params-j39</code> | notes | handler sdk/x/auth/keeper/grpc_query.go:77; callers are references to request type QueryParamsRequest |

## qry-cosmos-authz-v1beta1-granteegrants-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-authz-v1beta1-granteegrants-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:28</code> | <code>cosmos.authz.v1beta1.Query/GranteeGrants: rpc GranteeGrants(QueryGranteeGrantsRequest) returns (QueryGranteeGrantsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-authz-v1beta1-granteegrants-j39</code> | notes | handler sdk/x/authz/keeper/grpc_query.go:132; callers are references to request type QueryGranteeGrantsRequest |

## qry-cosmos-authz-v1beta1-grantergrants-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-authz-v1beta1-grantergrants-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:21</code> | <code>cosmos.authz.v1beta1.Query/GranterGrants: rpc GranterGrants(QueryGranterGrantsRequest) returns (QueryGranterGrantsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-authz-v1beta1-grantergrants-j39</code> | notes | handler sdk/x/authz/keeper/grpc_query.go:84; callers are references to request type QueryGranterGrantsRequest |

## qry-cosmos-authz-v1beta1-grants-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-authz-v1beta1-grants-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:14</code> | <code>cosmos.authz.v1beta1.Query/Grants: rpc Grants(QueryGrantsRequest) returns (QueryGrantsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-authz-v1beta1-grants-j39</code> | notes | handler sdk/x/authz/keeper/grpc_query.go:19; callers are references to request type QueryGrantsRequest |

## qry-cosmos-bank-v1beta1-allbalances-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-allbalances-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:20</code> | <code>cosmos.bank.v1beta1.Query/AllBalances: rpc AllBalances(QueryAllBalancesRequest) returns (QueryAllBalancesResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-allbalances-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:43; callers are references to request type QueryAllBalancesRequest |

## qry-cosmos-bank-v1beta1-balance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-balance-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:15</code> | <code>cosmos.bank.v1beta1.Query/Balance: rpc Balance(QueryBalanceRequest) returns (QueryBalanceResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-balance-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:18; callers are references to request type QueryBalanceRequest |

## qry-cosmos-bank-v1beta1-denommetadata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-denommetadata-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:46</code> | <code>cosmos.bank.v1beta1.Query/DenomMetadata: rpc DenomMetadata(QueryDenomMetadataRequest) returns (QueryDenomMetadataResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-denommetadata-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:183; callers are references to request type QueryDenomMetadataRequest |

## qry-cosmos-bank-v1beta1-denomsmetadata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-denomsmetadata-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:51</code> | <code>cosmos.bank.v1beta1.Query/DenomsMetadata: rpc DenomsMetadata(QueryDenomsMetadataRequest) returns (QueryDenomsMetadataResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-denomsmetadata-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:155; callers are references to request type QueryDenomsMetadataRequest |

## qry-cosmos-bank-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-params-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:41</code> | <code>cosmos.bank.v1beta1.Query/Params: rpc Params(QueryParamsRequest) returns (QueryParamsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-params-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:143; callers are references to request type QueryParamsRequest |

## qry-cosmos-bank-v1beta1-spendablebalances-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-spendablebalances-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:26</code> | <code>cosmos.bank.v1beta1.Query/SpendableBalances: rpc SpendableBalances(QuerySpendableBalancesRequest) returns (QuerySpendableBalancesResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-spendablebalances-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:81; callers are references to request type QuerySpendableBalancesRequest |

## qry-cosmos-bank-v1beta1-supplyof-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-supplyof-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:36</code> | <code>cosmos.bank.v1beta1.Query/SupplyOf: rpc SupplyOf(QuerySupplyOfRequest) returns (QuerySupplyOfResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-supplyof-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:127; callers are references to request type QuerySupplyOfRequest |

## qry-cosmos-bank-v1beta1-totalsupply-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-totalsupply-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:31</code> | <code>cosmos.bank.v1beta1.Query/TotalSupply: rpc TotalSupply(QueryTotalSupplyRequest) returns (QueryTotalSupplyResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-bank-v1beta1-totalsupply-j39</code> | notes | handler sdk/x/bank/keeper/grpc_query.go:116; callers are references to request type QueryTotalSupplyRequest |

## qry-cosmos-distribution-v1beta1-communitypool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-communitypool-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:66</code> | <code>cosmos.distribution.v1beta1.Query/CommunityPool: rpc CommunityPool(QueryCommunityPoolRequest) returns (QueryCommunityPoolResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-communitypool-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:245; callers are references to request type QueryCommunityPoolRequest |

## qry-cosmos-distribution-v1beta1-delegationrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegationrewards-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:39</code> | <code>cosmos.distribution.v1beta1.Query/DelegationRewards: rpc DelegationRewards(QueryDelegationRewardsRequest) returns (QueryDelegationRewardsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegationrewards-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:119; callers are references to request type QueryDelegationRewardsRequest |

## qry-cosmos-distribution-v1beta1-delegationtotalrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegationtotalrewards-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:47</code> | <code>cosmos.distribution.v1beta1.Query/DelegationTotalRewards: rpc DelegationTotalRewards(QueryDelegationTotalRewardsRequest) returns (QueryDelegationTotalRewardsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegationtotalrewards-j39</code> | notes | handler precompiles/distribution/distribution_test.go:889; callers are references to request type QueryDelegationTotalRewardsRequest |

## qry-cosmos-distribution-v1beta1-delegatorvalidators-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegatorvalidators-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:52</code> | <code>cosmos.distribution.v1beta1.Query/DelegatorValidators: rpc DelegatorValidators(QueryDelegatorValidatorsRequest) returns (QueryDelegatorValidatorsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegatorvalidators-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:197; callers are references to request type QueryDelegatorValidatorsRequest |

## qry-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:59</code> | <code>cosmos.distribution.v1beta1.Query/DelegatorWithdrawAddress: rpc DelegatorWithdrawAddress(QueryDelegatorWithdrawAddressRequest) returns (QueryDelegatorWithdrawAddressResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:225; callers are references to request type QueryDelegatorWithdrawAddressRequest |

## qry-cosmos-distribution-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-params-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:15</code> | <code>cosmos.distribution.v1beta1.Query/Params: rpc Params(QueryParamsRequest) returns (QueryParamsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-params-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:20; callers are references to request type QueryParamsRequest |

## qry-cosmos-distribution-v1beta1-validatorcommission-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-validatorcommission-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:27</code> | <code>cosmos.distribution.v1beta1.Query/ValidatorCommission: rpc ValidatorCommission(QueryValidatorCommissionRequest) returns (QueryValidatorCommissionResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-validatorcommission-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:50; callers are references to request type QueryValidatorCommissionRequest |

## qry-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:20</code> | <code>cosmos.distribution.v1beta1.Query/ValidatorOutstandingRewards: rpc ValidatorOutstandingRewards(QueryValidatorOutstandingRewardsRequest) returns (QueryValidatorOutstandingRewardsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:29; callers are references to request type QueryValidatorOutstandingRewardsRequest |

## qry-cosmos-distribution-v1beta1-validatorslashes-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-validatorslashes-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:34</code> | <code>cosmos.distribution.v1beta1.Query/ValidatorSlashes: rpc ValidatorSlashes(QueryValidatorSlashesRequest) returns (QueryValidatorSlashesResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-distribution-v1beta1-validatorslashes-j39</code> | notes | handler sdk/x/distribution/keeper/grpc_query.go:71; callers are references to request type QueryValidatorSlashesRequest |

## qry-cosmos-evidence-v1beta1-allevidence-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-evidence-v1beta1-allevidence-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:19</code> | <code>cosmos.evidence.v1beta1.Query/AllEvidence: rpc AllEvidence(QueryAllEvidenceRequest) returns (QueryAllEvidenceResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-evidence-v1beta1-allevidence-j39</code> | notes | handler sdk/x/evidence/keeper/grpc_query.go:52; callers are references to request type QueryAllEvidenceRequest |

## qry-cosmos-evidence-v1beta1-evidence-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-evidence-v1beta1-evidence-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:14</code> | <code>cosmos.evidence.v1beta1.Query/Evidence: rpc Evidence(QueryEvidenceRequest) returns (QueryEvidenceResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-evidence-v1beta1-evidence-j39</code> | notes | handler sdk/x/evidence/keeper/grpc_query.go:22; callers are references to request type QueryEvidenceRequest |

## qry-cosmos-feegrant-v1beta1-allowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-feegrant-v1beta1-allowance-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:14</code> | <code>cosmos.feegrant.v1beta1.Query/Allowance: rpc Allowance(QueryAllowanceRequest) returns (QueryAllowanceResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-feegrant-v1beta1-allowance-j39</code> | notes | handler sdk/x/feegrant/keeper/grpc_query.go:20; callers are references to request type QueryAllowanceRequest |

## qry-cosmos-feegrant-v1beta1-allowances-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-feegrant-v1beta1-allowances-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:19</code> | <code>cosmos.feegrant.v1beta1.Query/Allowances: rpc Allowances(QueryAllowancesRequest) returns (QueryAllowancesResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-feegrant-v1beta1-allowances-j39</code> | notes | handler sdk/x/feegrant/keeper/grpc_query.go:62; callers are references to request type QueryAllowancesRequest |

## qry-cosmos-feegrant-v1beta1-allowancesbygranter-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-feegrant-v1beta1-allowancesbygranter-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:25</code> | <code>cosmos.feegrant.v1beta1.Query/AllowancesByGranter: rpc AllowancesByGranter(QueryAllowancesByGranterRequest) returns (QueryAllowancesByGranterResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-feegrant-v1beta1-allowancesbygranter-j39</code> | notes | handler sdk/x/feegrant/keeper/grpc_query.go:98; callers are references to request type QueryAllowancesByGranterRequest |

## qry-cosmos-gov-v1beta1-deposit-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-deposit-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:39</code> | <code>cosmos.gov.v1beta1.Query/Deposit: rpc Deposit(QueryDepositRequest) returns (QueryDepositResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-deposit-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:186; callers are references to request type QueryDepositRequest |

## qry-cosmos-gov-v1beta1-deposits-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-deposits-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:44</code> | <code>cosmos.gov.v1beta1.Query/Deposits: rpc Deposits(QueryDepositsRequest) returns (QueryDepositsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-deposits-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:215; callers are references to request type QueryDepositsRequest |

## qry-cosmos-gov-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-params-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:34</code> | <code>cosmos.gov.v1beta1.Query/Params: rpc Params(QueryParamsRequest) returns (QueryParamsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-params-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:159; callers are references to request type QueryParamsRequest |

## qry-cosmos-gov-v1beta1-proposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-proposal-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:14</code> | <code>cosmos.gov.v1beta1.Query/Proposal: rpc Proposal(QueryProposalRequest) returns (QueryProposalResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-proposal-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:18; callers are references to request type QueryProposalRequest |

## qry-cosmos-gov-v1beta1-proposals-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-proposals-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:19</code> | <code>cosmos.gov.v1beta1.Query/Proposals: rpc Proposals(QueryProposalsRequest) returns (QueryProposalsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-proposals-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:38; callers are references to request type QueryProposalsRequest |

## qry-cosmos-gov-v1beta1-tallyresult-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-tallyresult-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:49</code> | <code>cosmos.gov.v1beta1.Query/TallyResult: rpc TallyResult(QueryTallyResultRequest) returns (QueryTallyResultResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-tallyresult-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:248; callers are references to request type QueryTallyResultRequest |

## qry-cosmos-gov-v1beta1-vote-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-vote-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:24</code> | <code>cosmos.gov.v1beta1.Query/Vote: rpc Vote(QueryVoteRequest) returns (QueryVoteResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-vote-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:96; callers are references to request type QueryVoteRequest |

## qry-cosmos-gov-v1beta1-votes-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-votes-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:29</code> | <code>cosmos.gov.v1beta1.Query/Votes: rpc Votes(QueryVotesRequest) returns (QueryVotesResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-gov-v1beta1-votes-j39</code> | notes | handler sdk/x/gov/keeper/grpc_query.go:125; callers are references to request type QueryVotesRequest |

## qry-cosmos-params-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-params-v1beta1-params-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/v1beta1/query.proto:14</code> | <code>cosmos.params.v1beta1.Query/Params: rpc Params(QueryParamsRequest) returns (QueryParamsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-params-v1beta1-params-j39</code> | notes | handler sdk/x/params/keeper/grpc_query.go:17; callers are references to request type QueryParamsRequest |

## qry-cosmos-slashing-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-slashing-v1beta1-params-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:14</code> | <code>cosmos.slashing.v1beta1.Query/Params: rpc Params(QueryParamsRequest) returns (QueryParamsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-slashing-v1beta1-params-j39</code> | notes | handler sdk/x/slashing/keeper/grpc_query.go:17; callers are references to request type QueryParamsRequest |

## qry-cosmos-slashing-v1beta1-signinginfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-slashing-v1beta1-signinginfo-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:19</code> | <code>cosmos.slashing.v1beta1.Query/SigningInfo: rpc SigningInfo(QuerySigningInfoRequest) returns (QuerySigningInfoResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-slashing-v1beta1-signinginfo-j39</code> | notes | handler sdk/x/slashing/keeper/grpc_query.go:28; callers are references to request type QuerySigningInfoRequest |

## qry-cosmos-slashing-v1beta1-signinginfos-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-slashing-v1beta1-signinginfos-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:24</code> | <code>cosmos.slashing.v1beta1.Query/SigningInfos: rpc SigningInfos(QuerySigningInfosRequest) returns (QuerySigningInfosResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-slashing-v1beta1-signinginfos-j39</code> | notes | handler sdk/x/slashing/keeper/grpc_query.go:51; callers are references to request type QuerySigningInfosRequest |

## qry-cosmos-staking-v1beta1-delegation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegation-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:36</code> | <code>cosmos.staking.v1beta1.Query/Delegation: rpc Delegation(QueryDelegationRequest) returns (QueryDelegationResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegation-j39</code> | notes | handler precompiles/staking/staking_test.go:284; callers are references to request type QueryDelegationRequest |

## qry-cosmos-staking-v1beta1-delegatordelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatordelegations-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:51</code> | <code>cosmos.staking.v1beta1.Query/DelegatorDelegations: rpc DelegatorDelegations(QueryDelegatorDelegationsRequest) returns (QueryDelegatorDelegationsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatordelegations-j39</code> | notes | handler precompiles/staking/staking_test.go:308; callers are references to request type QueryDelegatorDelegationsRequest |

## qry-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:57</code> | <code>cosmos.staking.v1beta1.Query/DelegatorUnbondingDelegations: rpc DelegatorUnbondingDelegations(QueryDelegatorUnbondingDelegationsRequest) returns (QueryDelegatorUnbondingDelegationsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39</code> | notes | handler precompiles/staking/staking_test.go:316; callers are references to request type QueryDelegatorUnbondingDelegationsRequest |

## qry-cosmos-staking-v1beta1-delegatorvalidator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatorvalidator-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:76</code> | <code>cosmos.staking.v1beta1.Query/DelegatorValidator: rpc DelegatorValidator(QueryDelegatorValidatorRequest) returns (QueryDelegatorValidatorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatorvalidator-j39</code> | notes | handler precompiles/staking/staking_test.go:312; callers are references to request type QueryDelegatorValidatorRequest |

## qry-cosmos-staking-v1beta1-delegatorvalidators-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatorvalidators-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:70</code> | <code>cosmos.staking.v1beta1.Query/DelegatorValidators: rpc DelegatorValidators(QueryDelegatorValidatorsRequest) returns (QueryDelegatorValidatorsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-delegatorvalidators-j39</code> | notes | handler precompiles/staking/staking_test.go:324; callers are references to request type QueryDelegatorValidatorsRequest |

## qry-cosmos-staking-v1beta1-historicalinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-historicalinfo-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:83</code> | <code>cosmos.staking.v1beta1.Query/HistoricalInfo: rpc HistoricalInfo(QueryHistoricalInfoRequest) returns (QueryHistoricalInfoResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-historicalinfo-j39</code> | notes | handler precompiles/staking/staking_test.go:328; callers are references to request type QueryHistoricalInfoRequest |

## qry-cosmos-staking-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-params-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:93</code> | <code>cosmos.staking.v1beta1.Query/Params: rpc Params(QueryParamsRequest) returns (QueryParamsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-params-j39</code> | notes | handler precompiles/staking/staking_test.go:336; callers are references to request type QueryParamsRequest |

## qry-cosmos-staking-v1beta1-pool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-pool-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:88</code> | <code>cosmos.staking.v1beta1.Query/Pool: rpc Pool(QueryPoolRequest) returns (QueryPoolResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-pool-j39</code> | notes | handler precompiles/staking/staking_test.go:332; callers are references to request type QueryPoolRequest |

## qry-cosmos-staking-v1beta1-redelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-redelegations-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:64</code> | <code>cosmos.staking.v1beta1.Query/Redelegations: rpc Redelegations(QueryRedelegationsRequest) returns (QueryRedelegationsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-redelegations-j39</code> | notes | handler precompiles/staking/staking_test.go:320; callers are references to request type QueryRedelegationsRequest |

## qry-cosmos-staking-v1beta1-unbondingdelegation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-unbondingdelegation-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:44</code> | <code>cosmos.staking.v1beta1.Query/UnbondingDelegation: rpc UnbondingDelegation(QueryUnbondingDelegationRequest) returns (QueryUnbondingDelegationResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-unbondingdelegation-j39</code> | notes | handler precompiles/staking/staking_test.go:304; callers are references to request type QueryUnbondingDelegationRequest |

## qry-cosmos-staking-v1beta1-validator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validator-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:19</code> | <code>cosmos.staking.v1beta1.Query/Validator: rpc Validator(QueryValidatorRequest) returns (QueryValidatorResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validator-j39</code> | notes | handler precompiles/staking/staking_test.go:292; callers are references to request type QueryValidatorRequest |

## qry-cosmos-staking-v1beta1-validatordelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validatordelegations-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:24</code> | <code>cosmos.staking.v1beta1.Query/ValidatorDelegations: rpc ValidatorDelegations(QueryValidatorDelegationsRequest) returns (QueryValidatorDelegationsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validatordelegations-j39</code> | notes | handler precompiles/staking/staking_test.go:296; callers are references to request type QueryValidatorDelegationsRequest |

## qry-cosmos-staking-v1beta1-validators-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validators-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:14</code> | <code>cosmos.staking.v1beta1.Query/Validators: rpc Validators(QueryValidatorsRequest) returns (QueryValidatorsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validators-j39</code> | notes | handler precompiles/staking/staking_test.go:288; callers are references to request type QueryValidatorsRequest |

## qry-cosmos-staking-v1beta1-validatorunbondingdelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validatorunbondingdelegations-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:29</code> | <code>cosmos.staking.v1beta1.Query/ValidatorUnbondingDelegations: rpc ValidatorUnbondingDelegations(QueryValidatorUnbondingDelegationsRequest) returns (QueryValidatorUnbondingDelegationsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-staking-v1beta1-validatorunbondingdelegations-j39</code> | notes | handler precompiles/staking/staking_test.go:300; callers are references to request type QueryValidatorUnbondingDelegationsRequest |

## qry-cosmos-upgrade-v1beta1-appliedplan-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-appliedplan-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:17</code> | <code>cosmos.upgrade.v1beta1.Query/AppliedPlan: rpc AppliedPlan(QueryAppliedPlanRequest) returns (QueryAppliedPlanResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-appliedplan-j39</code> | notes | handler sdk/x/upgrade/keeper/grpc_query.go:26; callers are references to request type QueryAppliedPlanRequest |

## qry-cosmos-upgrade-v1beta1-currentplan-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-currentplan-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:12</code> | <code>cosmos.upgrade.v1beta1.Query/CurrentPlan: rpc CurrentPlan(QueryCurrentPlanRequest) returns (QueryCurrentPlanResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-currentplan-j39</code> | notes | handler sdk/x/upgrade/keeper/grpc_query.go:14; callers are references to request type QueryCurrentPlanRequest |

## qry-cosmos-upgrade-v1beta1-moduleversions-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-moduleversions-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:35</code> | <code>cosmos.upgrade.v1beta1.Query/ModuleVersions: rpc ModuleVersions(QueryModuleVersionsRequest) returns (QueryModuleVersionsResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-moduleversions-j39</code> | notes | handler sdk/x/upgrade/keeper/grpc_query.go:53; callers are references to request type QueryModuleVersionsRequest |

## qry-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39</code> | <code>grpc_query</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:27</code> | <code>cosmos.upgrade.v1beta1.Query/UpgradedConsensusState: rpc UpgradedConsensusState(QueryUpgradedConsensusStateRequest) returns (QueryUpgradedConsensusStateResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.qry-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39</code> | notes | handler sdk/x/upgrade/keeper/grpc_query.go:39; callers are references to request type QueryUpgradedConsensusStateRequest |

## reserved-fields

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.282.1</code> | req | 7282.1 |
| <code>acceptance.7.282.1</code> | scenario | Given api/evm/params.proto with the reserved statements of wiring.J39.4 and the regenerated modules/evm/types/params.pb.go, when TestEvmParamsRemovedFieldsReserved reads Params from the buf image and from Params.Descriptor, then both reserve 1, 5, 6 and 7 with the four names and no live field uses a reserved number |
| <code>acceptance.7.282.1</code> | test | api/evm_reserved_test.go::TestEvmParamsRemovedFieldsReserved (planned) |
| <code>acceptance.7.282.1</code> | qualification | UNRUN |
| <code>req.7282</code> | ac_1 | Message Params of api/evm/params.proto:9 reserves the numbers 1, 5, 6 and 7 and the names base_denom, chain_config, chain_id and whitelisted_codehashes_bank_send in both the source and the descriptor registered by modules/evm/types/params.pb.go:533. |
| <code>acceptance.7.283.1</code> | req | 7283.1 |
| <code>acceptance.7.283.1</code> | scenario | Given api/oracle/oracle.proto and api/oracle/tx.proto with the reserved statements of wiring.J39.5 and their regenerated Go, when TestOracleRemovedFieldsReserved reads both messages from the buf image and from the generated descriptors, then Params reserves 8 and MsgAggregateExchangeRateVote reserves 1 and salt, and the genesis reservation is unchanged |
| <code>acceptance.7.283.1</code> | test | api/oracle_reserved_test.go::TestOracleRemovedFieldsReserved (planned) |
| <code>acceptance.7.283.1</code> | qualification | UNRUN |
| <code>req.7283</code> | ac_1 | Oracle Params (api/oracle/oracle.proto:8) reserves number 8 and MsgAggregateExchangeRateVote (api/oracle/tx.proto:20) reserves number 1 and the name salt, in both the source and the registered descriptors. |
| <code>resolution.17.791</code> | title | decision.729 binds: Removed field numbers are reserved |
| <code>resolution.17.791</code> | applies_to | decision.729<br>task.7.27.1<br>task.7.27.2 |
| <code>resolution.17.791</code> | source | decision.729 in fragments/J39/decision.kvx; tasks from affects |

## rest-cosmos-auth-v1beta1-account-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-account-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:24</code> | <code>GET /cosmos/auth/v1beta1/accounts/{address}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-account-j39</code> | notes | gateway handler generated in sdk/x/auth/types/query.pb.gw.go for cosmos.auth.v1beta1.Query/Account |

## rest-cosmos-auth-v1beta1-accounts-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-accounts-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:19</code> | <code>GET /cosmos/auth/v1beta1/accounts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-accounts-j39</code> | notes | gateway handler generated in sdk/x/auth/types/query.pb.gw.go for cosmos.auth.v1beta1.Query/Accounts |

## rest-cosmos-auth-v1beta1-nextaccountnumber-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-nextaccountnumber-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:34</code> | <code>GET /cosmos/auth/v1beta1/nextaccountnumber</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-nextaccountnumber-j39</code> | notes | gateway handler generated in sdk/x/auth/types/query.pb.gw.go for cosmos.auth.v1beta1.Query/NextAccountNumber |

## rest-cosmos-auth-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-params-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/auth/v1beta1/query.proto:29</code> | <code>GET /cosmos/auth/v1beta1/params</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-auth-v1beta1-params-j39</code> | notes | gateway handler generated in sdk/x/auth/types/query.pb.gw.go for cosmos.auth.v1beta1.Query/Params |

## rest-cosmos-authz-v1beta1-granteegrants-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-authz-v1beta1-granteegrants-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:29</code> | <code>GET /cosmos/authz/v1beta1/grants/grantee/{grantee}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-authz-v1beta1-granteegrants-j39</code> | notes | gateway handler generated in sdk/x/authz/query.pb.gw.go for cosmos.authz.v1beta1.Query/GranteeGrants |

## rest-cosmos-authz-v1beta1-grantergrants-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-authz-v1beta1-grantergrants-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:22</code> | <code>GET /cosmos/authz/v1beta1/grants/granter/{granter}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-authz-v1beta1-grantergrants-j39</code> | notes | gateway handler generated in sdk/x/authz/query.pb.gw.go for cosmos.authz.v1beta1.Query/GranterGrants |

## rest-cosmos-authz-v1beta1-grants-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-authz-v1beta1-grants-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/authz/v1beta1/query.proto:15</code> | <code>GET /cosmos/authz/v1beta1/grants</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-authz-v1beta1-grants-j39</code> | notes | gateway handler generated in sdk/x/authz/query.pb.gw.go for cosmos.authz.v1beta1.Query/Grants |

## rest-cosmos-bank-v1beta1-allbalances-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-allbalances-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:21</code> | <code>GET /cosmos/bank/v1beta1/balances/{address}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-allbalances-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/AllBalances |

## rest-cosmos-bank-v1beta1-balance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-balance-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:16</code> | <code>GET /cosmos/bank/v1beta1/balances/{address}/by_denom</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-balance-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/Balance |

## rest-cosmos-bank-v1beta1-denommetadata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-denommetadata-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:47</code> | <code>GET /cosmos/bank/v1beta1/denoms_metadata/{denom}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-denommetadata-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/DenomMetadata; the {denom} variable binds one path segment, so a denom that contains a slash such as factory/&lt;creator&gt;/&lt;subdenom&gt; (modules/tokenfactory/types/denoms.go:26) cannot be addressed on this route; task 7.26.1 adds a query-string binding |

Contract context <code>contract_context.7.rest-gateway</code>:

| Field | Frozen rule |
|---|---|
| interface | grpc-gateway REST routes generated from google.api.http rules and mounted by RegisterAPIRoutes. |
| storage | None; handlers forward to gRPC query servers. |
| errors | Unmatched routes answer 404 from the mux; gRPC status codes map to HTTP status through grpc-gateway v1.16.0. |
| events | None. |
| gas | Queries charge no gas to the caller. |
| activation | Effective when the node binary carrying the regenerated gateways starts; no upgrade height. |
| invariants | Every declared HTTP rule except the two reflection services is mounted; denom-valued routes have a query-string binding that accepts denoms containing slashes. |
| pause | The API server switch disables all routes together. |

## rest-cosmos-bank-v1beta1-denomsmetadata-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-denomsmetadata-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:52</code> | <code>GET /cosmos/bank/v1beta1/denoms_metadata</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-denomsmetadata-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/DenomsMetadata |

## rest-cosmos-bank-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-params-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:42</code> | <code>GET /cosmos/bank/v1beta1/params</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-params-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/Params |

## rest-cosmos-bank-v1beta1-spendablebalances-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-spendablebalances-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:27</code> | <code>GET /cosmos/bank/v1beta1/spendable_balances/{address}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-spendablebalances-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/SpendableBalances |

## rest-cosmos-bank-v1beta1-supplyof-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-supplyof-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:37</code> | <code>GET /cosmos/bank/v1beta1/supply/{denom}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-supplyof-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/SupplyOf; the {denom} variable binds one path segment, so a denom that contains a slash such as factory/&lt;creator&gt;/&lt;subdenom&gt; (modules/tokenfactory/types/denoms.go:26) cannot be addressed on this route; task 7.26.1 adds a query-string binding |

Contract context <code>contract_context.7.rest-gateway</code>:

| Field | Frozen rule |
|---|---|
| interface | grpc-gateway REST routes generated from google.api.http rules and mounted by RegisterAPIRoutes. |
| storage | None; handlers forward to gRPC query servers. |
| errors | Unmatched routes answer 404 from the mux; gRPC status codes map to HTTP status through grpc-gateway v1.16.0. |
| events | None. |
| gas | Queries charge no gas to the caller. |
| activation | Effective when the node binary carrying the regenerated gateways starts; no upgrade height. |
| invariants | Every declared HTTP rule except the two reflection services is mounted; denom-valued routes have a query-string binding that accepts denoms containing slashes. |
| pause | The API server switch disables all routes together. |

## rest-cosmos-bank-v1beta1-totalsupply-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-totalsupply-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto:32</code> | <code>GET /cosmos/bank/v1beta1/supply</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-bank-v1beta1-totalsupply-j39</code> | notes | gateway handler generated in sdk/x/bank/types/query.pb.gw.go for cosmos.bank.v1beta1.Query/TotalSupply |

## rest-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:13</code> | <code>GET /cosmos/base/reflection/v1beta1/interfaces</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v1beta1-reflectionservice-listallinterfaces-j39</code> | notes | gateway handler generated in sdk/client/grpc/reflection/reflection.pb.gw.go for cosmos.base.reflection.v1beta1.ReflectionService/ListAllInterfaces; no in-repo client calls this route |

## rest-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v1beta1/reflection.proto:19</code> | <code>GET /cosmos/base/reflection/v1beta1/interfaces/{interface_name}/implementations</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v1beta1-reflectionservice-listimplementations-j39</code> | notes | gateway handler generated in sdk/client/grpc/reflection/reflection.pb.gw.go for cosmos.base.reflection.v1beta1.ReflectionService/ListImplementations; no in-repo client calls this route |

## rest-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:120</code> | <code>GET /cosmos/base/reflection/v1beta1/app_descriptor/authn</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getauthndescriptor-j39</code> | notes | gateway handler generated in sdk/server/grpc/reflection/v2alpha1/reflection.pb.gw.go for cosmos.base.reflection.v2alpha1.ReflectionService/GetAuthnDescriptor; no in-repo client calls this route |

## rest-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:124</code> | <code>GET /cosmos/base/reflection/v1beta1/app_descriptor/chain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getchaindescriptor-j39</code> | notes | gateway handler generated in sdk/server/grpc/reflection/v2alpha1/reflection.pb.gw.go for cosmos.base.reflection.v2alpha1.ReflectionService/GetChainDescriptor; no in-repo client calls this route |

## rest-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:128</code> | <code>GET /cosmos/base/reflection/v1beta1/app_descriptor/codec</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getcodecdescriptor-j39</code> | notes | gateway handler generated in sdk/server/grpc/reflection/v2alpha1/reflection.pb.gw.go for cosmos.base.reflection.v2alpha1.ReflectionService/GetCodecDescriptor; no in-repo client calls this route |

## rest-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:132</code> | <code>GET /cosmos/base/reflection/v1beta1/app_descriptor/configuration</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getconfigurationdescriptor-j39</code> | notes | gateway handler generated in sdk/server/grpc/reflection/v2alpha1/reflection.pb.gw.go for cosmos.base.reflection.v2alpha1.ReflectionService/GetConfigurationDescriptor; no in-repo client calls this route |

## rest-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:136</code> | <code>GET /cosmos/base/reflection/v1beta1/app_descriptor/query_services</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-getqueryservicesdescriptor-j39</code> | notes | gateway handler generated in sdk/server/grpc/reflection/v2alpha1/reflection.pb.gw.go for cosmos.base.reflection.v2alpha1.ReflectionService/GetQueryServicesDescriptor; no in-repo client calls this route |

## rest-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/reflection/v2alpha1/reflection.proto:140</code> | <code>GET /cosmos/base/reflection/v1beta1/app_descriptor/tx_descriptor</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-reflection-v2alpha1-reflectionservice-gettxdescriptor-j39</code> | notes | gateway handler generated in sdk/server/grpc/reflection/v2alpha1/reflection.pb.gw.go for cosmos.base.reflection.v2alpha1.ReflectionService/GetTxDescriptor; no in-repo client calls this route |

## rest-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:29</code> | <code>GET /cosmos/base/tendermint/v1beta1/blocks/{height}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getblockbyheight-j39</code> | notes | gateway handler generated in sdk/client/grpc/tmservice/query.pb.gw.go for cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight |

## rest-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:25</code> | <code>GET /cosmos/base/tendermint/v1beta1/blocks/latest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getlatestblock-j39</code> | notes | gateway handler generated in sdk/client/grpc/tmservice/query.pb.gw.go for cosmos.base.tendermint.v1beta1.Service/GetLatestBlock |

## rest-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:34</code> | <code>GET /cosmos/base/tendermint/v1beta1/validatorsets/latest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getlatestvalidatorset-j39</code> | notes | gateway handler generated in sdk/client/grpc/tmservice/query.pb.gw.go for cosmos.base.tendermint.v1beta1.Service/GetLatestValidatorSet |

## rest-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:17</code> | <code>GET /cosmos/base/tendermint/v1beta1/node_info</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getnodeinfo-j39</code> | notes | gateway handler generated in sdk/client/grpc/tmservice/query.pb.gw.go for cosmos.base.tendermint.v1beta1.Service/GetNodeInfo |

## rest-cosmos-base-tendermint-v1beta1-service-getsyncing-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getsyncing-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:21</code> | <code>GET /cosmos/base/tendermint/v1beta1/syncing</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getsyncing-j39</code> | notes | gateway handler generated in sdk/client/grpc/tmservice/query.pb.gw.go for cosmos.base.tendermint.v1beta1.Service/GetSyncing |

## rest-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/base/tendermint/v1beta1/query.proto:38</code> | <code>GET /cosmos/base/tendermint/v1beta1/validatorsets/{height}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-base-tendermint-v1beta1-service-getvalidatorsetbyheight-j39</code> | notes | gateway handler generated in sdk/client/grpc/tmservice/query.pb.gw.go for cosmos.base.tendermint.v1beta1.Service/GetValidatorSetByHeight |

## rest-cosmos-distribution-v1beta1-communitypool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-communitypool-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:67</code> | <code>GET /cosmos/distribution/v1beta1/community_pool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-communitypool-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/CommunityPool |

## rest-cosmos-distribution-v1beta1-delegationrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegationrewards-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:40</code> | <code>GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/rewards/{validator_address}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegationrewards-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/DelegationRewards |

## rest-cosmos-distribution-v1beta1-delegationtotalrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegationtotalrewards-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:48</code> | <code>GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/rewards</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegationtotalrewards-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/DelegationTotalRewards |

## rest-cosmos-distribution-v1beta1-delegatorvalidators-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegatorvalidators-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:53</code> | <code>GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/validators</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegatorvalidators-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/DelegatorValidators |

## rest-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:60</code> | <code>GET /cosmos/distribution/v1beta1/delegators/{delegator_address}/withdraw_address</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-delegatorwithdrawaddress-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/DelegatorWithdrawAddress |

## rest-cosmos-distribution-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-params-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:16</code> | <code>GET /cosmos/distribution/v1beta1/params</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-params-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/Params |

## rest-cosmos-distribution-v1beta1-validatorcommission-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-validatorcommission-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:28</code> | <code>GET /cosmos/distribution/v1beta1/validators/{validator_address}/commission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-validatorcommission-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/ValidatorCommission |

## rest-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:21</code> | <code>GET /cosmos/distribution/v1beta1/validators/{validator_address}/outstanding_rewards</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-validatoroutstandingrewards-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/ValidatorOutstandingRewards |

## rest-cosmos-distribution-v1beta1-validatorslashes-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-validatorslashes-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/distribution/v1beta1/query.proto:35</code> | <code>GET /cosmos/distribution/v1beta1/validators/{validator_address}/slashes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-distribution-v1beta1-validatorslashes-j39</code> | notes | gateway handler generated in sdk/x/distribution/types/query.pb.gw.go for cosmos.distribution.v1beta1.Query/ValidatorSlashes |

## rest-cosmos-evidence-v1beta1-allevidence-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-evidence-v1beta1-allevidence-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:20</code> | <code>GET /cosmos/evidence/v1beta1/evidence</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-evidence-v1beta1-allevidence-j39</code> | notes | gateway handler generated in sdk/x/evidence/types/query.pb.gw.go for cosmos.evidence.v1beta1.Query/AllEvidence |

## rest-cosmos-evidence-v1beta1-evidence-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-evidence-v1beta1-evidence-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/evidence/v1beta1/query.proto:15</code> | <code>GET /cosmos/evidence/v1beta1/evidence/{evidence_hash}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-evidence-v1beta1-evidence-j39</code> | notes | gateway handler generated in sdk/x/evidence/types/query.pb.gw.go for cosmos.evidence.v1beta1.Query/Evidence |

## rest-cosmos-feegrant-v1beta1-allowance-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-feegrant-v1beta1-allowance-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:15</code> | <code>GET /cosmos/feegrant/v1beta1/allowance/{granter}/{grantee}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-feegrant-v1beta1-allowance-j39</code> | notes | gateway handler generated in sdk/x/feegrant/query.pb.gw.go for cosmos.feegrant.v1beta1.Query/Allowance |

## rest-cosmos-feegrant-v1beta1-allowances-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-feegrant-v1beta1-allowances-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:20</code> | <code>GET /cosmos/feegrant/v1beta1/allowances/{grantee}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-feegrant-v1beta1-allowances-j39</code> | notes | gateway handler generated in sdk/x/feegrant/query.pb.gw.go for cosmos.feegrant.v1beta1.Query/Allowances |

## rest-cosmos-feegrant-v1beta1-allowancesbygranter-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-feegrant-v1beta1-allowancesbygranter-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/feegrant/v1beta1/query.proto:26</code> | <code>GET /cosmos/feegrant/v1beta1/issued/{granter}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-feegrant-v1beta1-allowancesbygranter-j39</code> | notes | gateway handler generated in sdk/x/feegrant/query.pb.gw.go for cosmos.feegrant.v1beta1.Query/AllowancesByGranter |

## rest-cosmos-gov-v1beta1-deposit-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-deposit-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:40</code> | <code>GET /cosmos/gov/v1beta1/proposals/{proposal_id}/deposits/{depositor}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-deposit-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/Deposit |

## rest-cosmos-gov-v1beta1-deposits-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-deposits-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:45</code> | <code>GET /cosmos/gov/v1beta1/proposals/{proposal_id}/deposits</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-deposits-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/Deposits |

## rest-cosmos-gov-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-params-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:35</code> | <code>GET /cosmos/gov/v1beta1/params/{params_type}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-params-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/Params |

## rest-cosmos-gov-v1beta1-proposal-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-proposal-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:15</code> | <code>GET /cosmos/gov/v1beta1/proposals/{proposal_id}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-proposal-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/Proposal |

## rest-cosmos-gov-v1beta1-proposals-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-proposals-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:20</code> | <code>GET /cosmos/gov/v1beta1/proposals</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-proposals-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/Proposals |

## rest-cosmos-gov-v1beta1-tallyresult-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-tallyresult-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:50</code> | <code>GET /cosmos/gov/v1beta1/proposals/{proposal_id}/tally</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-tallyresult-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/TallyResult |

## rest-cosmos-gov-v1beta1-vote-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-vote-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:25</code> | <code>GET /cosmos/gov/v1beta1/proposals/{proposal_id}/votes/{voter}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-vote-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/Vote |

## rest-cosmos-gov-v1beta1-votes-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-votes-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/gov/v1beta1/query.proto:30</code> | <code>GET /cosmos/gov/v1beta1/proposals/{proposal_id}/votes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-gov-v1beta1-votes-j39</code> | notes | gateway handler generated in sdk/x/gov/types/query.pb.gw.go for cosmos.gov.v1beta1.Query/Votes |

## rest-cosmos-params-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-params-v1beta1-params-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/params/v1beta1/query.proto:15</code> | <code>GET /cosmos/params/v1beta1/params</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-params-v1beta1-params-j39</code> | notes | gateway handler generated in sdk/x/params/types/proposal/query.pb.gw.go for cosmos.params.v1beta1.Query/Params |

## rest-cosmos-slashing-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-slashing-v1beta1-params-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:15</code> | <code>GET /cosmos/slashing/v1beta1/params</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-slashing-v1beta1-params-j39</code> | notes | gateway handler generated in sdk/x/slashing/types/query.pb.gw.go for cosmos.slashing.v1beta1.Query/Params |

## rest-cosmos-slashing-v1beta1-signinginfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-slashing-v1beta1-signinginfo-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:20</code> | <code>GET /cosmos/slashing/v1beta1/signing_infos/{cons_address}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-slashing-v1beta1-signinginfo-j39</code> | notes | gateway handler generated in sdk/x/slashing/types/query.pb.gw.go for cosmos.slashing.v1beta1.Query/SigningInfo |

## rest-cosmos-slashing-v1beta1-signinginfos-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-slashing-v1beta1-signinginfos-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/slashing/v1beta1/query.proto:25</code> | <code>GET /cosmos/slashing/v1beta1/signing_infos</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-slashing-v1beta1-signinginfos-j39</code> | notes | gateway handler generated in sdk/x/slashing/types/query.pb.gw.go for cosmos.slashing.v1beta1.Query/SigningInfos |

## rest-cosmos-staking-v1beta1-delegation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegation-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:37</code> | <code>GET /cosmos/staking/v1beta1/validators/{validator_addr}/delegations/{delegator_addr}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegation-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/Delegation |

## rest-cosmos-staking-v1beta1-delegatordelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatordelegations-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:52</code> | <code>GET /cosmos/staking/v1beta1/delegations/{delegator_addr}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatordelegations-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/DelegatorDelegations |

## rest-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:58</code> | <code>GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/unbonding_delegations</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatorunbondingdelegations-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/DelegatorUnbondingDelegations |

## rest-cosmos-staking-v1beta1-delegatorvalidator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatorvalidator-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:77</code> | <code>GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/validators/{validator_addr}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatorvalidator-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/DelegatorValidator |

## rest-cosmos-staking-v1beta1-delegatorvalidators-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatorvalidators-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:71</code> | <code>GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/validators</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-delegatorvalidators-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/DelegatorValidators |

## rest-cosmos-staking-v1beta1-historicalinfo-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-historicalinfo-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:84</code> | <code>GET /cosmos/staking/v1beta1/historical_info/{height}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-historicalinfo-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/HistoricalInfo |

## rest-cosmos-staking-v1beta1-params-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-params-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:94</code> | <code>GET /cosmos/staking/v1beta1/params</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-params-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/Params |

## rest-cosmos-staking-v1beta1-pool-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-pool-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:89</code> | <code>GET /cosmos/staking/v1beta1/pool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-pool-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/Pool |

## rest-cosmos-staking-v1beta1-redelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-redelegations-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:65</code> | <code>GET /cosmos/staking/v1beta1/delegators/{delegator_addr}/redelegations</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-redelegations-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/Redelegations |

## rest-cosmos-staking-v1beta1-unbondingdelegation-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-unbondingdelegation-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:45</code> | <code>GET /cosmos/staking/v1beta1/validators/{validator_addr}/delegations/{delegator_addr}/unbonding_delegation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-unbondingdelegation-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/UnbondingDelegation |

## rest-cosmos-staking-v1beta1-validator-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validator-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:20</code> | <code>GET /cosmos/staking/v1beta1/validators/{validator_addr}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validator-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/Validator |

## rest-cosmos-staking-v1beta1-validatordelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validatordelegations-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:25</code> | <code>GET /cosmos/staking/v1beta1/validators/{validator_addr}/delegations</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validatordelegations-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/ValidatorDelegations |

## rest-cosmos-staking-v1beta1-validators-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validators-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:15</code> | <code>GET /cosmos/staking/v1beta1/validators</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validators-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/Validators |

## rest-cosmos-staking-v1beta1-validatorunbondingdelegations-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validatorunbondingdelegations-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/staking/v1beta1/query.proto:30</code> | <code>GET /cosmos/staking/v1beta1/validators/{validator_addr}/unbonding_delegations</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-staking-v1beta1-validatorunbondingdelegations-j39</code> | notes | gateway handler generated in sdk/x/staking/types/query.pb.gw.go for cosmos.staking.v1beta1.Query/ValidatorUnbondingDelegations |

## rest-cosmos-tx-v1beta1-service-broadcasttx-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-broadcasttx-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:30</code> | <code>POST /cosmos/tx/v1beta1/txs body *</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-broadcasttx-j39</code> | notes | gateway handler generated in sdk/types/tx/service.pb.gw.go for cosmos.tx.v1beta1.Service/BroadcastTx |

## rest-cosmos-tx-v1beta1-service-getblockwithtxs-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-getblockwithtxs-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:43</code> | <code>GET /cosmos/tx/v1beta1/txs/block/{height}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-getblockwithtxs-j39</code> | notes | gateway handler generated in sdk/types/tx/service.pb.gw.go for cosmos.tx.v1beta1.Service/GetBlockWithTxs; no in-repo client calls this route |

## rest-cosmos-tx-v1beta1-service-gettx-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-gettx-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:26</code> | <code>GET /cosmos/tx/v1beta1/txs/{hash}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-gettx-j39</code> | notes | gateway handler generated in sdk/types/tx/service.pb.gw.go for cosmos.tx.v1beta1.Service/GetTx |

## rest-cosmos-tx-v1beta1-service-gettxsevent-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-gettxsevent-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:37</code> | <code>GET /cosmos/tx/v1beta1/txs</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-gettxsevent-j39</code> | notes | gateway handler generated in sdk/types/tx/service.pb.gw.go for cosmos.tx.v1beta1.Service/GetTxsEvent |

## rest-cosmos-tx-v1beta1-service-simulate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-simulate-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/tx/v1beta1/service.proto:19</code> | <code>POST /cosmos/tx/v1beta1/simulate body *</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-tx-v1beta1-service-simulate-j39</code> | notes | gateway handler generated in sdk/types/tx/service.pb.gw.go for cosmos.tx.v1beta1.Service/Simulate; no in-repo client calls this route |

## rest-cosmos-upgrade-v1beta1-appliedplan-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-appliedplan-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:18</code> | <code>GET /cosmos/upgrade/v1beta1/applied_plan/{name}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-appliedplan-j39</code> | notes | gateway handler generated in sdk/x/upgrade/types/query.pb.gw.go for cosmos.upgrade.v1beta1.Query/AppliedPlan |

## rest-cosmos-upgrade-v1beta1-currentplan-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-currentplan-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:13</code> | <code>GET /cosmos/upgrade/v1beta1/current_plan</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-currentplan-j39</code> | notes | gateway handler generated in sdk/x/upgrade/types/query.pb.gw.go for cosmos.upgrade.v1beta1.Query/CurrentPlan |

## rest-cosmos-upgrade-v1beta1-moduleversions-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-moduleversions-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:36</code> | <code>GET /cosmos/upgrade/v1beta1/module_versions</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-moduleversions-j39</code> | notes | gateway handler generated in sdk/x/upgrade/types/query.pb.gw.go for cosmos.upgrade.v1beta1.Query/ModuleVersions |

## rest-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39</code> | <code>rest_route</code> | <code>stable</code> | <code>sdk/proto/cosmos/upgrade/v1beta1/query.proto:29</code> | <code>GET /cosmos/upgrade/v1beta1/upgraded_consensus_state/{last_height}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.7.rest-cosmos-upgrade-v1beta1-upgradedconsensusstate-j39</code> | notes | gateway handler generated in sdk/x/upgrade/types/query.pb.gw.go for cosmos.upgrade.v1beta1.Query/UpgradedConsensusState |

## rest-denom-bindings

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.rest-tokenfactory-denomauthoritymetadata-by-query-j39</code> | <code>rest_route</code> | <code>new</code> | <code>api/tokenfactory/query.proto</code> (planned) | <code>GET /pax-protocol/paxchain/tokenfactory/denom_authority_metadata?denom=&lt;denom&gt;</code> |
| <code>interface.6.rest-tokenfactory-denompaused-by-query-j39</code> | <code>rest_route</code> | <code>new</code> | <code>api/tokenfactory/query.proto</code> (planned) | <code>GET /pax-protocol/paxchain/tokenfactory/denom_paused?denom=&lt;denom&gt;</code> |
| <code>interface.7.rest-cosmos-bank-v1beta1-denommetadata-by-query-string-j39</code> | <code>rest_route</code> | <code>new</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto</code> (planned) | <code>GET /cosmos/bank/v1beta1/denoms_metadata_by_query_string?denom=&lt;denom&gt;</code> |
| <code>interface.7.rest-cosmos-bank-v1beta1-supplyof-by-denom-j39</code> | <code>rest_route</code> | <code>new</code> | <code>sdk/proto/cosmos/bank/v1beta1/query.proto</code> (planned) | <code>GET /cosmos/bank/v1beta1/supply/by_denom?denom=&lt;denom&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.rest-tokenfactory-denomauthoritymetadata-by-query-j39</code> | notes | planned additional binding of paxprotocol.paxchain.tokenfactory.Query/DenomAuthorityMetadata (api/tokenfactory/query.proto:24) requested by wiring.J39.2 |
| <code>interface.6.rest-tokenfactory-denompaused-by-query-j39</code> | notes | planned additional binding of the planned paxprotocol.paxchain.tokenfactory.Query/DenomPaused requested by wiring.J39.3 |
| <code>interface.7.rest-cosmos-bank-v1beta1-denommetadata-by-query-string-j39</code> | notes | planned additional binding of cosmos.bank.v1beta1.Query/DenomMetadata (sdk/proto/cosmos/bank/v1beta1/query.proto:47); generated into sdk/x/bank/types/query.pb.gw.go by wiring.J39.1 |
| <code>interface.7.rest-cosmos-bank-v1beta1-supplyof-by-denom-j39</code> | notes | planned additional binding of cosmos.bank.v1beta1.Query/SupplyOf (sdk/proto/cosmos/bank/v1beta1/query.proto:37); generated into sdk/x/bank/types/query.pb.gw.go by wiring.J39.1 |

Contract context <code>contract_context.7.rest-gateway</code>:

| Field | Frozen rule |
|---|---|
| interface | grpc-gateway REST routes generated from google.api.http rules and mounted by RegisterAPIRoutes. |
| storage | None; handlers forward to gRPC query servers. |
| errors | Unmatched routes answer 404 from the mux; gRPC status codes map to HTTP status through grpc-gateway v1.16.0. |
| events | None. |
| gas | Queries charge no gas to the caller. |
| activation | Effective when the node binary carrying the regenerated gateways starts; no upgrade height. |
| invariants | Every declared HTTP rule except the two reflection services is mounted; denom-valued routes have a query-string binding that accepts denoms containing slashes. |
| pause | The API server switch disables all routes together. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.7.278.1</code> | req | 7278.1 |
| <code>acceptance.7.278.1</code> | scenario | Given the bank query.proto source with the block-form http options, when TestBankQueryAdditionalBindings reads SupplyOf and DenomMetadata from the buf image, then each keeps its {denom} path route and has exactly one additional GET binding, /cosmos/bank/v1beta1/supply/by_denom and /cosmos/bank/v1beta1/denoms_metadata_by_query_string, with no body |
| <code>acceptance.7.278.1</code> | test | api/bank_http_rules_test.go::TestBankQueryAdditionalBindings (planned) |
| <code>acceptance.7.278.1</code> | qualification | UNRUN |
| <code>req.7278</code> | ac_1 | The SupplyOf rpc (sdk/proto/cosmos/bank/v1beta1/query.proto:36-38) and the DenomMetadata rpc (sdk/proto/cosmos/bank/v1beta1/query.proto:46-48) each keep their path route and carry exactly one bodiless additional_bindings GET route without a path variable (planned). |
| <code>acceptance.7.279.1</code> | req | 7279.1 |
| <code>acceptance.7.279.1</code> | scenario | Given the buf image of sdk/proto/cosmos/bank/v1beta1 and the generated descriptors read through one representative type per file, when TestCosmosBankDescriptorParity compares them file by file, then no difference is reported and every .proto in the directory has a table entry |
| <code>acceptance.7.279.1</code> | test | api/cosmos_bank_parity_test.go::TestCosmosBankDescriptorParity (planned) |
| <code>acceptance.7.279.1</code> | qualification | UNRUN |
| <code>req.7279</code> | ac_1 | Every .proto under sdk/proto/cosmos/bank/v1beta1 (5 at HEAD, for example sdk/proto/cosmos/bank/v1beta1/authz.proto:1) matches the descriptor of its generated package read through a representative type, in package, imports, messages, fields, enums, rpcs and HTTP rules (planned). |
| <code>acceptance.7.279.2</code> | req | 7279.2 |
| <code>acceptance.7.279.2</code> | scenario | Given an app from node.SetupWithDefaultHome with 1000 of factory/&lt;creator&gt;/rest minted and the regenerated bank gateway mounted, when TestBankSupplyOfFactoryDenomOverREST issues GET /cosmos/bank/v1beta1/supply/by_denom with the url-escaped denom, then the answer is 200 with amount 1000 of that denom, and the path form answers 404 |
| <code>acceptance.7.279.2</code> | test | api/bank_rest_test.go::TestBankSupplyOfFactoryDenomOverREST (planned) |
| <code>acceptance.7.279.2</code> | qualification | UNRUN |
| <code>req.7279</code> | ac_2 | GET /cosmos/bank/v1beta1/supply/by_denom?denom=factory%2F&lt;creator&gt;%2Frest returns the supply of that factory denom through SupplyOf (sdk/x/bank/keeper/grpc_query.go:127), while the path form on pattern_Query_SupplyOf_0 (sdk/x/bank/types/query.pb.gw.go:853) returns 404. |
| <code>acceptance.7.279.3</code> | req | 7279.3 |
| <code>acceptance.7.279.3</code> | scenario | Given an app with factory/&lt;creator&gt;/rest created by the tokenfactory keeper and the regenerated bank gateway mounted, when TestBankDenomMetadataFactoryDenomOverREST issues GET /cosmos/bank/v1beta1/denoms_metadata_by_query_string with the url-escaped denom, then the answer is 200 with metadata.base equal to that denom, and the path form answers 404 |
| <code>acceptance.7.279.3</code> | test | api/bank_rest_test.go::TestBankDenomMetadataFactoryDenomOverREST (planned) |
| <code>acceptance.7.279.3</code> | qualification | UNRUN |
| <code>req.7279</code> | ac_3 | GET /cosmos/bank/v1beta1/denoms_metadata_by_query_string?denom=factory%2F&lt;creator&gt;%2Frest returns the metadata CreateDenom stored (modules/tokenfactory/keeper/createdenom.go:13) through DenomMetadata (sdk/x/bank/keeper/grpc_query.go:183), while the path form on pattern_Query_DenomMetadata_0 (sdk/x/bank/types/query.pb.gw.go:857) returns 404. |
| <code>acceptance.7.280.1</code> | req | 7280.1 |
| <code>acceptance.7.280.1</code> | scenario | Given an app with factory/&lt;creator&gt;/meta created by the tokenfactory keeper and the regenerated tokenfactory gateway mounted, when TestTokenfactoryAuthorityMetadataFactoryDenomOverREST issues GET /pax-protocol/paxchain/tokenfactory/denom_authority_metadata with the url-escaped denom, then the answer is 200 with authority_metadata.admin equal to the creator, and the path form answers 404 |
| <code>acceptance.7.280.1</code> | test | api/tokenfactory_rest_test.go::TestTokenfactoryAuthorityMetadataFactoryDenomOverREST (planned) |
| <code>acceptance.7.280.1</code> | qualification | UNRUN |
| <code>req.7280</code> | ac_1 | GET /pax-protocol/paxchain/tokenfactory/denom_authority_metadata?denom=factory%2F&lt;creator&gt;%2Fmeta returns Admin equal to the creator through DenomAuthorityMetadata (modules/tokenfactory/keeper/grpc_query.go:22), while the path route of api/tokenfactory/query.proto:24 returns 404 for that denom. |
| <code>acceptance.7.281.1</code> | req | 7281.1 |
| <code>acceptance.7.281.1</code> | scenario | Given an app with one paused and one unpaused factory denom and the regenerated tokenfactory gateway mounted, when TestTokenfactoryDenomPausedFactoryDenomOverREST issues GET /pax-protocol/paxchain/tokenfactory/denom_paused for each url-escaped denom, then the answers are 200 with paused true and paused false, and the path form answers 404 |
| <code>acceptance.7.281.1</code> | test | api/tokenfactory_paused_rest_test.go::TestTokenfactoryDenomPausedFactoryDenomOverREST (planned) |
| <code>acceptance.7.281.1</code> | qualification | UNRUN |
| <code>req.7281</code> | ac_1 | GET /pax-protocol/paxchain/tokenfactory/denom_paused?denom=factory%2F&lt;creator&gt;%2Fhalt reports paused true for a paused factory denom and false for an unpaused one, while the planned path route under /pax-protocol/paxchain/tokenfactory/denoms/ returns 404 for that denom (planned). |
| <code>resolution.17.788</code> | title | decision.726 binds: Denom-valued REST routes gain query-string bindings |
| <code>resolution.17.788</code> | applies_to | decision.726<br>task.7.26.1<br>task.7.26.2<br>task.7.26.3<br>task.7.26.4 |
| <code>resolution.17.788</code> | source | decision.726 in fragments/J39/decision.kvx; tasks from affects |
