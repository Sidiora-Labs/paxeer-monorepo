# Test plan

This page lists focused test examples for the kernel, node, modules, services
and clients. Commands run from the repository root unless a command selects a
package directory explicitly. Test files, package manifests and the root Makefile
define the available targets.

## Toolchains and native dependencies

The root [Go module](../../../../go.mod) declares Go 1.25.8. Nested modules retain
their own manifests; `go -C` selects the nested module in the examples below.
The [Rust toolchain](../../../../rust-toolchain.toml) defines the compiler and
components for Cargo packages. JavaScript packages use their declared manager
and lockfile; the explorer frontend declares Yarn in its `package.json`.

The cgo host in [modules/lxphost/host.go](../../../../modules/lxphost/host.go)
links `liblayerx`, `liblayerx_programs_sandbox`, OpenSSL and platform libraries.
Its headers include `build/generated`. The root `paxd-native-libs` Make target
builds native dependencies, and `test-lxphost` depends on that target. Go tests
that exercise the native host require cgo and those libraries.

The agent TypeScript SDK compiles its `src`, `test` and `examples` directories
into `dist`; its example below builds that package before selecting compiled
tests. The seller middleware tests import `dist` as well; its example builds
the middleware and its declared workspace dependencies first. The perps
TypeScript example uses its declared `tsx` loader.

The root `foundry.toml` and `foundry.paxeer.toml` select different Solidity test
paths. Chain contract examples select `foundry.paxeer.toml` explicitly. The
Ethereum mirror has its own Foundry project.

## Selecting tests

| Runner | Selection |
| --- | --- |
| Go | `-run` matches top-level and slash-separated subtest names; `-count=1` disables cached results |
| Cargo | `--manifest-path` and `-p` select the package; `--test`, `--lib` or `--bin` selects the target; `--exact` matches the full test name |
| Node | `--test-name-pattern` matches registered test names; TypeScript needs a supported runtime or loader |
| Vitest | `-t` matches the complete title, including enclosing `describe` titles |
| Playwright | File, project and title filters select browser cases |
| Hardhat | `--grep` matches Mocha titles, including suite names |
| pytest | `file::Class::test` or `file::test` selects a Python case |
| unittest | A dotted module, class and method selects a Python case |
| Foundry | `--match-path` and `--match-test` select Solidity tests |
| ExUnit | A test file and case line select a case; `--only` matches declared tags |
| Maven | Surefire's `-Dtest=Class#method` selects JVM cases |
| .NET | `--filter` selects test names or properties |
| SwiftPM | `--filter` selects test class and method names |

## Targets by domain

### 1. kernel

| Toolchain | Manifest boundaries |
| --- | --- |
| C | `Makefile` |
| Go | `go.mod` |
| Python | Python 3 |

C example:

```sh
make -C . build/tests/kernel/codec/lxp_test_case_dispatch && ./build/tests/kernel/codec/lxp_test_case_dispatch case_dispatch_runs_named_case case_dispatch_rejects_unknown_case
```

Go example:

```sh
CGO_ENABLED=1 go test -count=1 ./modules/lxphost/ -run '^(TestKernelArchiveSymbolsReadsLibrary)$'
```

Python example:

```sh
python3 -m pytest layerxproof/retired/test_anchor_vectors_retired.py::test_anchor_vectors_generator_absent -q
```

### 2. ledger-auth-pay

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `go.mod` |

Go example:

```sh
go test -count=1 ./modules/layerxledger/keeper/ ./modules/layerxauth/keeper/ -run '^(TestLedgerFromKernelCodes|TestAuthFromKernelCodes)$'
```

### 3. markets

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `go.mod` |
| JS/TS | `apps/perps/package.json` |

Go example:

```sh
go test -count=1 ./modules/perps/types/ -run '^(TestParamsCustomTypeRoundTrip|TestMarketCustomTypeRoundTrip)$'
```

JS/TS example:

```sh
pnpm --dir apps/perps exec node --test --import tsx --test-name-pattern '^(retiredSpotOperationsAbsent)$' tests/graphql-operations.test.ts
```

### 4. launchpad

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `go.mod`; `sdk/launchpad/protocol/go.mod`; `sdk/launchpad/shared/go.mod`; `services/launchpad/api/go.mod`; `services/launchpad/discovery/go.mod`; `services/launchpad/gateway/go.mod`; `services/launchpad/indexer/go.mod`; `services/launchpad/livestream/go.mod`; `services/launchpad/metadata/go.mod`; `services/launchpad/notifications/go.mod`; `services/launchpad/pnl-tracker/go.mod`; `services/launchpad/ranking-algo/go.mod`; `services/launchpad/social/go.mod`; `services/launchpad/stats-workers/go.mod`; `services/launchpad/trading-charts/go.mod`; `services/launchpad/user/go.mod` |
| JS/TS | `apps/launchpad/mobile/package.json`; `apps/launchpad/web/package.json`; `contracts/launchpad/package.json`; `package.json`; `sdk/launchpad/packages/sdk/package.json`; `sdk/launchpad/packages/sidiora-launchpad-v3/package.json` |
| Python | Python 3 |

Go example:

```sh
go test -count=1 ./precompiles/launchpad/ -run '^(TestVersionedLaunchpad)$'
```

JS/TS example:

```sh
pnpm --dir contracts/launchpad exec hardhat test test/production/Health.test.js --grep 'NativeSurfaceIdentity'
```

Python example:

```sh
python3 -m pytest services/launchpad/security/supplychain/test_workspace_gates.py::test_makefile_modules_match_go_work -q
```

### 5. node-runtime

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `go.mod` |
| Python | Python 3 |
| Rust | `cmd/layerxctl/Cargo.toml` |

Go example:

```sh
go test -count=1 ./node/ -run '^(TestSetSequencerSourceReturnsInstalledSource|TestSetSequencerRPCKeepsOptions|TestSetCommitListenerKeepsListener)$'
```

Python example:

```sh
python3 -m pytest cmd/layerx-module-registry/test_registry_cli.py::test_read_node_is_refused cmd/layerx-module-registry/test_registry_cli.py::test_socket_and_actor_flags_are_refused -q
```

Rust example:

```sh
cargo test --locked --manifest-path cmd/layerxctl/Cargo.toml --test files -- --exact files::rejects_symlinks_directories_empty_and_oversized_files
```

### 6. chain-modules

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `go.mod` |
| JS/TS | `contracts/package.json` |
| Python | Python 3 |
| Solidity | `foundry.paxeer.toml` for chain contracts; `foundry.toml` for kernel contracts |

Go example:

```sh
go test -count=1 ./precompiles/ ./precompiles/common/ -run '^(TestMergeAddressTable|TestRetiredAtMergePlan)$'
```

JS/TS example:

```sh
node --test --test-name-pattern '^hardhatSolcMatchesFoundryPaxeer$' contracts/test/hardhat_solc_parity.test.js
```

Python example:

```sh
python3 -m pytest tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_reports_a_tracked_retired_file tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_reports_a_live_import_outside_the_retired_set tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_ignores_documentation_references -q
```

Solidity example:

```sh
forge test --root . --config-path foundry.paxeer.toml --match-path contracts/test/IBankServedSelectors.t.sol --match-test '^test_IBankDeclaresOnlyServedSelectors$'
```

### 7. rpc-api

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `go.mod` |

Go example:

```sh
go test -count=1 ./rpc/ -run '^(TestServedMethodsIncludesPromotedMethods|TestServedMethodsMatchServerDispatch)$'
```

### 8. platform

| Toolchain | Manifest boundaries |
| --- | --- |
| Dotnet | `sdk/platform/dotnet/tests/LayerX.Sdk.Tests/LayerX.Sdk.Tests.csproj` |
| Go | `sdk/platform/go/go.mod` |
| JS/TS | `package.json`; `services/platform/examples/marketplace/package.json`; `services/platform/middleware/agent/package.json`; `services/platform/middleware/buyer/package.json`; `services/platform/middleware/conformance/package.json`; `services/platform/middleware/merchant/package.json`; `services/platform/middleware/seller/package.json` |
| JVM | `sdk/platform/jvm/pom.xml` |
| Python | Python 3 |
| Rust | `sdk/platform/generators/Cargo.toml`; `services/platform/hosted/agent-boundary/Cargo.toml`; `services/platform/hosted/authority/Cargo.toml`; `services/platform/hosted/core/Cargo.toml`; `services/platform/hosted/gateway/Cargo.toml`; `services/platform/hosted/indexer/Cargo.toml`; `services/platform/hosted/paxeer/Cargo.toml`; `services/platform/hosted/registry/Cargo.toml`; `services/platform/hosted/testnet/Cargo.toml`; `services/platform/hosted/webhooks/Cargo.toml`; `services/platform/ramps/reference/Cargo.toml`; `services/platform/ramps/toolkit/Cargo.toml`; `tools/platform/cli/Cargo.toml`; `tools/platform/emulator/Cargo.toml`; `tools/platform/release/Cargo.toml` |
| Swift | `sdk/platform/swift/Package.swift` |

Dotnet example:

```sh
dotnet test sdk/platform/dotnet/tests/LayerX.Sdk.Tests/LayerX.Sdk.Tests.csproj --filter 'FullyQualifiedName=LayerX.Sdk.Tests.SequencerReceiptTests.SequencerReceiptV1CasesVerifyReceipted|FullyQualifiedName=LayerX.Sdk.Tests.SequencerReceiptTests.SequencerReceiptV2RevertDecodesReason|FullyQualifiedName=LayerX.Sdk.Tests.SequencerReceiptTests.SequencerReceiptV3RefusalsNameTheirCheck'
```

Go example:

```sh
go -C sdk/platform/go test -count=1 . -run '^(TestSequencerReceiptV1CasesVerifyReceipted|TestSequencerReceiptV2RevertDecodesReason|TestSequencerReceiptV3RefusalsNameTheirCheck)$'
```

JS/TS example:

```sh
npm --prefix services/platform/middleware/seller run build
node --test --test-name-pattern='^(encodesLedgerDrawCalldataWithDrawSelector|decodesPaymentExactLogFromLedgerPrecompile|refusesLedgerLogFromAnotherAddress)$' services/platform/middleware/seller/test/ledger.test.mjs
```

JVM example:

```sh
mvn -q -f sdk/platform/jvm/pom.xml test -Dtest='SequencerReceiptTest#sequencerReceiptV1CasesVerifyReceipted+sequencerReceiptV2RevertDecodesReason+sequencerReceiptV3RefusalsNameTheirCheck' -Dsurefire.failIfNoSpecifiedTests=true
```

Python example:

```sh
python3 -m pytest services/platform/hosted/tests/test_retired_kernel_scripts.py::test_paxeer_deployment_ships_no_anchor_genesis -q
```

Rust example:

```sh
cargo test --locked --manifest-path services/platform/Cargo.toml -p layerx-platform-gateway --lib -- --exact tests::router_methods_are_sorted_unique_and_namespaced tests::router_method_refuses_every_lx_name
```

Swift example:

```sh
swift test --package-path sdk/platform/swift --filter '^LayerXSDKTests\.SequencerReceiptTests/(testSequencerReceiptV1CasesVerifyReceipted|testSequencerReceiptV2RevertDecodesReason|testSequencerReceiptV3RefusalsNameTheirCheck)$'
```

### 9. agent

| Toolchain | Manifest boundaries |
| --- | --- |
| JS/TS | `services/agent/sdk/typescript/package.json` |
| Python | `services/agent/sdk/python/pyproject.toml` |
| Rust | `services/agent/Cargo.toml`; `services/agent/crates/layerx-agent-api/Cargo.toml`; `services/agent/crates/layerx-agentd-host/Cargo.toml`; `services/agent/crates/layerx-agentd/Cargo.toml`; `services/agent/crates/layerx-client/Cargo.toml`; `services/agent/crates/layerx-crypto/Cargo.toml`; `services/agent/crates/layerx-mcp/Cargo.toml`; `services/agent/crates/layerx-proof/Cargo.toml`; `services/agent/crates/layerx-sdk/Cargo.toml`; `services/agent/crates/layerx-types/Cargo.toml`; `services/agent/crates/layerx-wire/Cargo.toml`; `services/agent/fuzz/Cargo.toml`; `services/agent/qualification/pay2/Cargo.toml`; `services/agent/tests/boundary/Cargo.toml`; `services/agent/tests/parity/Cargo.toml`; `services/agent/tests/qualify/Cargo.toml`; `services/agent/tools/boundary-check/Cargo.toml`; `services/agent/tools/sdk-gen/Cargo.toml` |

JS/TS example:

```sh
npm --prefix services/agent/sdk/typescript run build
node --test --test-name-pattern='^(placeOrder_call_carries_the_account_id|exchange_wallet_requests_are_type_2_on_chain_125)$' services/agent/sdk/typescript/dist/test/exchange-v2.test.js
```

Python example:

```sh
python3 -m pytest services/agent/sdk/python/tests/test_px_commit.py::test_network_head_has_no_anchor services/agent/sdk/python/tests/test_px_commit.py::test_history_cursor_is_block_position -q
```

Rust example:

```sh
cargo test --locked --manifest-path services/agent/Cargo.toml -p layerx-agent-api --test rung -- --exact rung_and_rung_status_are_exported_from_the_verify_contract
```

### 10. human

| Toolchain | Manifest boundaries |
| --- | --- |
| Rust | `services/human/crates/layerx-human-kms/Cargo.toml`; `services/human/crates/layerx-human-service/Cargo.toml`; `services/human/crates/layerx-human-test-support/Cargo.toml`; `services/human/crates/layerx-intents/Cargo.toml`; `services/human/crates/layerx-network-gateway/Cargo.toml`; `services/human/tools/api-gen/Cargo.toml`; `services/human/tools/boundary-check/Cargo.toml`; `services/human/tools/copy-lint/Cargo.toml`; `services/human/tools/schema-check/Cargo.toml` |

Rust example:

```sh
cargo test --locked --manifest-path services/human/Cargo.toml -p layerx-human-service --test frame_transport -- --exact frame_transport_round_trips_a_frame_over_uds_within_limits frame_transport_refuses_a_frame_above_max_frame_bytes
```

### 11. interop

| Toolchain | Manifest boundaries |
| --- | --- |
| Python | Python 3 |
| Rust | `services/interop/Cargo.toml`; `services/interop/contracts/solana-mirror/Cargo.toml`; `services/interop/crates/layerx-ap2/Cargo.toml`; `services/interop/crates/layerx-bridge-relayer/Cargo.toml`; `services/interop/crates/layerx-fiat/Cargo.toml`; `services/interop/crates/layerx-gas-station/Cargo.toml`; `services/interop/crates/layerx-interop-gateway/Cargo.toml`; `services/interop/crates/layerx-interop-service/Cargo.toml`; `services/interop/crates/layerx-mirror/Cargo.toml`; `services/interop/crates/layerx-portable/Cargo.toml`; `services/interop/crates/layerx-ucp/Cargo.toml`; `services/interop/crates/layerx-visa-tap/Cargo.toml`; `services/interop/crates/layerx-x402/Cargo.toml`; `services/interop/crates/x-websearch/Cargo.toml` |
| Solidity | `services/interop/contracts/ethereum-mirror/foundry.toml` |

Python example:

```sh
python3 -m pytest services/interop/deploy/gateway/tests/test_env_template.py::test_template_names_every_variable_the_service_reads -q
```

Rust example:

```sh
cargo test --locked --manifest-path services/interop/Cargo.toml -p layerx-interop-gateway --test evidence -- --exact settlement_rung_labels_are_submitted_receipted_and_committed canonical_receipt_digest_matches_the_recorded_pay_exact_vector
```

Solidity example:

```sh
forge test --root services/interop/contracts/ethereum-mirror --match-path '**/test/LayerXMirrorArchive.t.sol' --match-test '^(testBeginRecordsCommittedHeightAndBlockHash|testBeginRejectsZeroBlockHash|testBeginRejectsDifferentBlockHashOnReopen)$'
```

### 12. wallet

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `services/wallet/attestor/go.mod` |
| JS/TS | `apps/wallet/package.json`; `sdk/wallet/package.json`; `services/wallet/demo/package.json`; `services/wallet/gateway/package.json` |
| Rust | `services/wallet/attestor/internal/policy/lx/testdata/kernelvectors/Cargo.toml` |

Go example:

```sh
go -C services/wallet/attestor test -count=1 ./internal/server/ -run '^(TestEVMReviewDisclosesDecodedCall|TestEVMReviewRecordsNoSpend|TestEVMReviewRefusesWrongChain)$'
```

JS/TS example:

```sh
pnpm --dir services/wallet/gateway exec vitest run test/env.test.ts -t '^gateway environment (sequencerUrlDefaultsToFirstReadUrl|configuredSequencerUrlIsKept)$'
```

Rust example:

```sh
cargo test --locked --manifest-path services/wallet/attestor/internal/policy/lx/testdata/kernelvectors/Cargo.toml --bin attestor-kernel-vectors -- --exact tests::generated_vectors_match_pinned_file
```

### 13. programs

| Toolchain | Manifest boundaries |
| --- | --- |
| C | `Makefile` |
| Go | `go.mod` |
| Python | Python 3 |
| Rust | `programs/crates/layerx-programs-ai-market/Cargo.toml`; `programs/crates/layerx-programs-arbiter/Cargo.toml`; `programs/crates/layerx-programs-registry/Cargo.toml`; `programs/crates/layerx-programs-runtime/Cargo.toml`; `programs/crates/layerx-programs-sandbox/Cargo.toml`; `programs/fuzz/Cargo.toml` |

C example:

```sh
make build/tests/programs_parallel_differential && ./build/tests/programs_parallel_differential
```

Go example:

```sh
go test -count=1 ./precompiles/layerxprograms/ -run '^(TestProgramsABIEventTopics|TestProgramsABIKeepsThirtySixMethods)$'
```

Python example:

```sh
python3 -m pytest programs/fixtures/pay5/test_native_roundtrip.py::test_native_roundtrip_replays_without_history_feed -q
```

Rust example:

```sh
cargo test --locked --manifest-path programs/Cargo.toml -p layerx-programs-registry --test chain_proto -- --exact chain_proto_reader_decodes_canonical_fields chain_proto_reader_refuses_noncanonical_varint_and_order
```

### 14. web-explorer-bridge

| Toolchain | Manifest boundaries |
| --- | --- |
| Elixir | `services/explorer/backend/apps/block_scout_web/mix.exs`; `services/explorer/backend/apps/ethereum_jsonrpc/mix.exs`; `services/explorer/backend/apps/explorer/mix.exs`; `services/explorer/backend/apps/indexer/mix.exs` |
| Go | `go.mod` |
| JS/TS | `apps/bridge/package.json`; `apps/explorer/package.json`; `apps/web/package.json`; `sdk/js/package.json` |
| Python | Python 3 |
| Rust | `services/explorer/services/sig-provider/sig-provider-server/Cargo.toml`; `services/explorer/services/sig-provider/sig-provider/Cargo.toml` |
| Shell | `Makefile` |

Elixir example:

```sh
services/explorer/deploy/tools/mix-in-builder.sh test apps/explorer/test/explorer/chain/paxeer_x/capabilities_retired_probe_test.exs:6 apps/explorer/test/explorer/chain/paxeer_x/capabilities_retired_probe_test.exs:10
```

Go example:

```sh
go test -count=1 ./services/bridge/deploy/appenv/ -run '^(TestEnvLinesFromEVMRecords|TestRefusesARecordWithAnotherChainID|TestRefusesAZeroVault)$'
```

JS/TS example:

```sh
node --test --test-name-pattern '^(sdk_js_has_no_vendor_pin|package_entries_resolve_under_require|package_types_subpath_resolves_to_runtime_module)$' sdk/js/test/package.test.cjs
```

Python example:

```sh
python3 -m pytest services/explorer/tests/test_paxeer_x_readme_retirements.py::test_readmes_name_no_retired_fetcher -q
```

Rust example:

```sh
cargo test --locked --manifest-path services/explorer/services/sig-provider/Cargo.toml -p sig-provider --lib -- --exact sources::precompiles::tests::selector_resolves_to_exchange_signature sources::precompiles::tests::unknown_selector_returns_empty
```

Shell example:

```sh
bash services/bridge/deploy/tests/workflow-check.sh
```

### 15. cosmos-fork

| Toolchain | Manifest boundaries |
| --- | --- |
| Go | `go.mod` |
| JS/TS | `sdk/integrations/agents/package.json`; `sdk/integrations/express/package.json`; `sdk/integrations/next/package.json` |
| JVM | `sdk/integrations/android/pom.xml`; `sdk/integrations/spring/pom.xml` |
| Python | `sdk/integrations/fastapi/pyproject.toml` |
| Swift | `sdk/integrations/ios/Package.swift` |

Go example:

```sh
go test -count=1 ./sdk/testutil/oteltest/ -run '^(TestReaderSeesInstrumentCreatedBeforeInstall|TestLegacySinkCapturesTelemetryCounter)$'
```

JS/TS example:

```sh
node --test --test-name-pattern '^(loadConfigReadsReceiptKeysJson|integrationSettlesSignedSequencerReceipt)$' sdk/integrations/express/test/receipt-keys.test.mjs
```

JVM example:

```sh
mvn -q -f sdk/integrations/spring/pom.xml -Dtest='ReceiptKeysConfigTest#declaredConfigBindsReceiptKeys+autoConfigurationBuildsAuthorityFromReceiptKeys' test
```

Python example:

```sh
PYTHONPATH=sdk/integrations/fastapi:services/agent/sdk/python python3 -m pytest sdk/integrations/fastapi/tests/test_receipt_keys_config.py::test_load_config_reads_receipt_keys sdk/integrations/fastapi/tests/test_receipt_keys_config.py::test_load_config_refuses_authorized_batch_only -q
```

Swift example:

```sh
swift test --package-path sdk/integrations/ios --filter 'LayerXMobileTests.SampleEnvironmentTests/testDecodesSignedReceiptPayload'
```

### 16. Tools and fixtures

The C case dispatcher is built by the root Makefile. Process-block Go fixtures
live in `tests/testutil/processblock`.

```sh
make -C . build/tests/lxp_test_harness && ./build/tests/lxp_test_harness --run harness.run-one-runs-named-case && ./build/tests/lxp_test_harness --run harness.run-one-reports-unknown-case
```

```sh
go test -count=1 ./tests/testutil/processblock/ -run '^(TestNewTestAppWithGenesisOverridesModuleSection|TestNewTestAppWithGenesisRejectsUnknownModule)$'
```

### 17. KVX documents

The programme parser tests cover section loading and duplicate-section refusal.

```sh
python3 -m pytest spec/layerx-merge/programme/tests/test_kvx.py::test_loads_workflow_sections spec/layerx-merge/programme/tests/test_kvx.py::test_refuses_duplicate_section -q
```

## Integration fixtures

The SDK network fixture uses a single validator by default in
[sdk/testutil/network](../../../../sdk/testutil/network/network.go).
Human-service transport tests exercise Unix sockets, and the KMS integration
fixtures live under `services/human/crates/layerx-human-kms/tests/support`.

The explorer backend's
[Mix wrapper](../../../../services/explorer/deploy/tools/mix-in-builder.sh)
selects its builder image and disposable PostgreSQL and Redis sidecars. Its
arguments after the wrapper options are Mix arguments, relative to
`services/explorer/backend`. Browser tests use the frontend package's Playwright
configuration and browser dependencies.

## Behavioral coverage

[Activation and retirement](activation-and-migration.md) describes the focused
merge, export/import, precompile and store-deletion cases. Receipt-order coverage
includes accepted batches, swapped transactions, count mismatches, sequence gaps,
previous-hash mismatches and checks that proposal validation leaves committed
state untouched. Sequencer restart and standby cases live in `node/sequencer`.

The root Makefile also provides `workspace-build`, `workspace-test`,
`workspace-lint`, `workspace-ci` and `core-test-all` for broader package coverage.
The focused commands above identify individual behaviors within those packages.
