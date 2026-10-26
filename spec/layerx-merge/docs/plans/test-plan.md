# Test plan

The test contract is one acceptance criterion, one unique test selector, and one owning task. The merged candidate contains 4,025 criteria, 4,025 acceptance blocks, 4,025 distinct task test selectors, and 2,545 tasks. There are no duplicate test owners, uncovered acceptance selectors, or task selectors without an acceptance. All acceptance qualification values remain `UNRUN`; these counts describe planned verification, not passing executions.

Baseline revision: `2a1033aa26a3f669f2fc23098926cc8dd5940000`. Candidate SHA-256 (spec.kvx): `0c3b740c6cde1e8b2f0aef2914d0b1d65989268d928022cb4181417bf06ee32e`. Counts come from `out/spec.kvx`; the build input is the final fragments, including the domain checks, wiring and graph repairs. `inventory/J20.md` sections 1–3 provide the baseline test and manifest inventory. The command and manifest review below reads source only; no repository build, test, dependency install or generated-file command was executed.

## Counts and ownership

“Criteria = tests” counts requirement criteria and their acceptance tests by criterion domain. P/E means planned/existing according to `acceptance.test`, independently of whether its containing file already exists. “Owned tests” counts selectors in that domain’s task lists. A planned test in an existing file remains planned. Each selected top-level test may cover several input cases; those cases do not create extra acceptance criteria.

| Domain | Tasks | Declared planned / existing gates | Criteria = tests | P / E | Owned tests | Owned P / E |
|---|---:|---:|---:|---:|---:|---:|
| 1 kernel | 141 | 141 / 0 | 244 | 244 / 0 | 244 | 244 / 0 |
| 2 ledger-auth-pay | 153 | 151 / 2 | 283 | 281 / 2 | 283 | 281 / 2 |
| 3 markets | 170 | 166 / 4 | 225 | 218 / 7 | 225 | 218 / 7 |
| 4 launchpad | 186 | 173 / 13 | 263 | 244 / 19 | 263 | 244 / 19 |
| 5 node-runtime | 232 | 231 / 1 | 337 | 332 / 5 | 382 | 377 / 5 |
| 6 chain-modules | 253 | 248 / 5 | 344 | 331 / 13 | 345 | 332 / 13 |
| 7 rpc-api | 116 | 116 / 0 | 147 | 145 / 2 | 147 | 145 / 2 |
| 8 platform | 213 | 212 / 1 | 378 | 370 / 8 | 378 | 370 / 8 |
| 9 agent | 148 | 136 / 12 | 280 | 262 / 18 | 280 | 262 / 18 |
| 10 human | 125 | 122 / 3 | 199 | 189 / 10 | 199 | 189 / 10 |
| 11 interop | 125 | 125 / 0 | 236 | 234 / 2 | 236 | 234 / 2 |
| 12 wallet | 117 | 116 / 1 | 168 | 167 / 1 | 168 | 167 / 1 |
| 13 programs | 128 | 121 / 7 | 206 | 199 / 7 | 206 | 199 / 7 |
| 14 web-explorer-bridge | 163 | 161 / 2 | 274 | 272 / 2 | 274 | 272 / 2 |
| 15 cosmos-fork | 57 | 57 / 0 | 136 | 136 / 0 | 90 | 90 / 0 |
| 16 tools-ops | 153 | 152 / 1 | 216 | 214 / 2 | 216 | 214 / 2 |
| 17 programme | 65 | 65 / 0 | 89 | 89 / 0 | 89 | 89 / 0 |
| Total | 2545 | 2493 / 52 | 4025 | 3927 / 98 | 4025 | 3927 / 98 |

Domain 15 keeps 46 acceptance criteria whose implementation paths belong to other domains: 45 `consensus/` tests are owned by tasks `5.61.1`–`5.69.1`; `acceptance.15.272.1` is owned by `6.13.1` for the `interchain/` proto registry. This explains the 337/382, 344/345 and 136/90 criterion/owner totals in domains 5, 6 and 15. Count the same selector once across the join; do not duplicate it to force each owner’s local criterion count to match.

The baseline inventory includes many unchanged tests and whole-script gates. It is not the acceptance denominator. Do not add acceptance tests solely to repeat unchanged behavior or to meet a task-count floor. An existing test can satisfy a changed criterion only when its assertions exercise the required behavior through the real implementation. A new test must fail for the relevant pre-change behavior; a file-presence check cannot establish transaction, storage, signature, or refusal behavior.

## Command shapes and manifest constraints

Task `build_cmd` and `verify_cmd` are separate bounded invocations. Run the exact declared selectors in an isolated implementation checkout, with one build and one verification invocation per task and at most two repair cycles within the task budget. A shell command containing several exact selectors still has to preserve every child exit status. Listing targets, type checking, compilation and dry runs are preparation; they do not count as executing acceptance tests.

| Toolchain | Bounded verification shape | Selector obligation |
|---|---|---|
| Go | `go test -count=1 ./<package>/ -run '^(TestA\|TestB)$'` | Use the containing go.mod; nested modules use go -C <module> test. Suite methods need the actual top-level suite runner and anchored subtest path. |
| Rust | `cargo test --locked --manifest-path <workspace>/Cargo.toml -p <crate> --test <target> -- --exact <full_name>` | Use --lib or --bin only for that target type; verify every full module-qualified name and the selected workspace lock. |
| Vitest | `<manager> exec vitest run <file> -t '<full-title-pattern>'` | Match the full describe/test title. A leaf-only ^name$ pattern can select zero tests under describe. |
| Node | `node --test --test-name-pattern '^(name)$' <file>` | Require named node:test cases and a nonzero executed-case count; use --import tsx or the supported type-stripping runtime for TypeScript. |
| Playwright | `pnpm --dir <app> exec playwright test --config=<config> <file> -g '<title>$'` | Select the declared project and browser and compare discovered and executed titles with the task selectors. |
| Hardhat | `pnpm --dir contracts/launchpad exec hardhat test <file> --grep '^<title>$'` | Anchor the complete Mocha title when nesting adds suite names; count executed cases. |
| Python pytest | `python3 -m pytest <file>::<test> -q` | For classes use file::Class::test. Supply the package import roots rather than substituting a source scan. |
| Python unittest | `python3 -m unittest <module>.<Class>.<test>` | List exact methods; a declared script gate must dispatch its named cases and reject unknown names. |
| Solidity | `forge test --root <root> --match-path <file> --match-test '^<name>$'` | Choose the matching Foundry configuration and require an executed test rather than successful compilation. |
| C | `make -C . <exact-binary> && ./<exact-binary> <case>` | Named dispatch must reject unknown cases and execute only the declared case; a legacy main-only binary remains a whole-binary gate until refactored. |
| Elixir | `services/explorer/deploy/tools/mix-in-builder.sh test <file>:<line>` | Use a real case location or tested ExUnit tags; a display title is not automatically an ExUnit tag. |
| JVM | `mvn -q -f <module>/pom.xml -Dtest='<Class>#<method>' test` | Limit the Surefire class and method and require executed tests. |
| Dotnet | `dotnet test <test-project>.csproj --filter 'FullyQualifiedName=<class.method>'` | Use exact names or an explicit union; reject an empty or broader-than-declared selection. |
| Swift | `swift test --package-path <package> --filter <Class>/<method>` | Use the declared test target and anchor the method filter where necessary. |

Go is pinned by `go.mod:3`. Native linkage in `modules/lxphost/host.go:7` requires the kernel static library, the programs sandbox static library, OpenSSL development libraries, and `CGO_ENABLED=1`. Provision those dependencies through the foundation/build tasks before the bounded verification command; do not consume the verification budget finding missing libraries. Root-wide Go coverage belongs to release aggregation, not an individual task.

Rust is pinned by `rust-toolchain.toml:2`. `8.94.2` and `8.96.4` select `-p layerx-platform-registry`, the package at `services/platform/hosted/registry/Cargo.toml:2`. The six commands selecting `layerx-committed-proof` are an expected planned boundary: task `11.15.1` creates that crate, adds it to `services/interop/Cargo.toml:2`, and updates the lock before its dependants run.

The root JavaScript packages use the workspace declarations at `package.json:8`. J20 section 2.3 records root npm locking, app-local pnpm locking and the explorer yarn lock. Use the matching lockfile manager when provisioning dependencies. The declared gate commands below are reproduced from task blocks; using `pnpm exec` does not authorize installing a second dependency tree in a package governed by npm or yarn. The perps test script explicitly supplies `--import tsx` at `apps/perps/package.json:10`. TypeScript files run directly with Node require an explicit compatible loader or type-stripping runtime; a filename suffix alone proves nothing.

The chain Solidity project uses `foundry.paxeer.toml:4`, while `foundry.toml:3` selects `tests/solidity`. The root skip list at `foundry.toml:16` excludes the chain contract tests, so `6.82.1` must keep its `foundry.paxeer.toml` selection. Bootstrap the declared Solidity libraries before the bounded build. The mirror contracts have their own Foundry root.

The explorer builder wrapper documents a disposable database and forwards a single Mix command at `services/explorer/deploy/tools/mix-in-builder.sh:75`. Python import roots come from the local `pyproject.toml` or task environment; there is no repository-wide pytest configuration in the baseline inventory. The SDK test project declares its runtime at `sdk/platform/dotnet/tests/LayerX.Sdk.Tests/LayerX.Sdk.Tests.csproj:3`, and the Swift SDK declares the test target at `sdk/platform/swift/Package.swift:17`.

The iOS integration package at `sdk/integrations/ios/Package.swift:4` has no test target at baseline. Task `15.4.9` explicitly adds `LayerXMobileTests` and its test directory before that task and `15.4.10` can execute their planned Swift selectors. This is a planned manifest change, not an existing runnable target.

## Targets by domain and toolchain

The per-domain toolchain tables below were classified on the 2,517-task candidate and are not recomputed here; the 28 tasks added since (domains 8, 10 and 13) and their 28 criteria are counted only in the table above.

Each table counts owned, unique `path::TestName` selectors and distinct test files. Manifest citations were read at the pinned revision. Multiple manifest entries are separate boundaries, not an instruction to run each entire module. `task-defined runner environment` means that there is no local manifest for that target; it does not certify an absent dependency or a new package. The following representative commands are exact current task values; their planned tests must be created first and the defect notes below remain execution blockers. The full selector and command lists stay in the cited task blocks, so this plan does not create a second command registry.

### 1. kernel

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| C | 74 | 141 / 0 | `Makefile:292` | `task.1.0.1` |
| Go | 69 | 102 / 0 | `go.mod:3` | `task.1.21.1` |
| Python | 1 | 1 / 0 | task-defined runner environment | `task.1.97.1` |

C, `task.1.0.1`:

```sh
make -C . build/tests/kernel/codec/lxp_test_case_dispatch && ./build/tests/kernel/codec/lxp_test_case_dispatch case_dispatch_runs_named_case case_dispatch_rejects_unknown_case
```

Go, `task.1.21.1`:

```sh
CGO_ENABLED=1 go test -count=1 ./modules/lxphost/ -run '^(TestKernelArchiveSymbolsReadsLibrary)$'
```

Python, `task.1.97.1`:

```sh
python3 -m pytest layerxproof/retired/test_anchor_vectors_retired.py::test_anchor_vectors_generator_absent -q
```

### 2. ledger-auth-pay

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 159 | 281 / 2 | `go.mod:3` | `task.2.0.1` |

Go, `task.2.0.1`:

```sh
go test -count=1 ./modules/layerxledger/keeper/ ./modules/layerxauth/keeper/ -run '^(TestLedgerFromKernelCodes|TestAuthFromKernelCodes)$'
```

### 3. markets

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 140 | 183 / 7 | `go.mod:3` | `task.3.0.1` |
| JS/TS | 27 | 35 / 0 | `apps/perps/package.json:2` | `task.3.43.3` |

Go, `task.3.0.1`:

```sh
go test -count=1 ./modules/perps/types/ -run '^(TestParamsCustomTypeRoundTrip|TestMarketCustomTypeRoundTrip)$'
```

JS/TS, `task.3.43.3`:

```sh
pnpm --dir apps/perps exec node --test --import tsx --test-name-pattern '^(retiredSpotOperationsAbsent)$' tests/graphql-operations.test.ts
```

### 4. launchpad

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 136 | 173 / 15 | `go.mod:3`; `sdk/launchpad/protocol/go.mod:6`; `sdk/launchpad/shared/go.mod:8`; `services/launchpad/api/go.mod:6`; `services/launchpad/discovery/go.mod:3`; `services/launchpad/gateway/go.mod:7`; `services/launchpad/indexer/go.mod:6`; `services/launchpad/livestream/go.mod:6`; `services/launchpad/metadata/go.mod:6`; `services/launchpad/notifications/go.mod:3`; `services/launchpad/pnl-tracker/go.mod:6`; `services/launchpad/ranking-algo/go.mod:6`; `services/launchpad/social/go.mod:6`; `services/launchpad/stats-workers/go.mod:6`; `services/launchpad/trading-charts/go.mod:6`; `services/launchpad/user/go.mod:6` | `task.4.0.1` |
| JS/TS | 42 | 58 / 0 | `apps/launchpad/mobile/package.json:2`; `apps/launchpad/web/package.json:2`; `contracts/launchpad/package.json:2`; `package.json:2`; `sdk/launchpad/packages/sdk/package.json:2`; `sdk/launchpad/packages/sidiora-launchpad-v3/package.json:2` | `task.4.0.4` |
| Python | 7 | 13 / 4 | task-defined runner environment | `task.4.21.1` |

Go, `task.4.0.1`:

```sh
go test -count=1 ./precompiles/launchpad/ -run '^(TestVersionedLaunchpad)$'
```

JS/TS, `task.4.0.4`:

```sh
pnpm --dir contracts/launchpad exec hardhat test test/production/Health.test.js --grep 'NativeSurfaceIdentity'
```

Python, `task.4.21.1`:

```sh
python3 -m pytest services/launchpad/security/supplychain/test_workspace_gates.py::test_makefile_modules_match_go_work -q
```

### 5. node-runtime

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 179 | 332 / 2 | `go.mod:3` | `task.5.0.1` |
| Python | 34 | 40 / 0 | task-defined runner environment | `task.5.48.1` |
| Rust | 1 | 5 / 3 | `cmd/layerxctl/Cargo.toml:1` | `task.5.47.1` |

Go, `task.5.0.1`:

```sh
go test -count=1 ./node/ -run '^(TestSetSequencerSourceReturnsInstalledSource|TestSetSequencerRPCKeepsOptions|TestSetCommitListenerKeepsListener)$'
```

Python, `task.5.48.1`:

```sh
python3 -m pytest cmd/layerx-module-registry/test_registry_cli.py::test_read_node_is_refused cmd/layerx-module-registry/test_registry_cli.py::test_socket_and_actor_flags_are_refused -q
```

Rust, `task.5.47.1`:

```sh
cargo test --locked --manifest-path cmd/layerxctl/Cargo.toml --test files -- --exact files::rejects_symlinks_directories_empty_and_oversized_files
```

### 6. chain-modules

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 198 | 270 / 13 | `go.mod:3` | `task.6.0.1` |
| JS/TS | 1 | 1 / 0 | `contracts/package.json:2` | `task.6.81.1` |
| Python | 58 | 60 / 0 | task-defined runner environment | `task.6.51.1` |
| Solidity | 1 | 1 / 0 | `foundry.toml:3` | `task.6.82.1` |

Go, `task.6.0.1`:

```sh
go test -count=1 ./precompiles/ ./precompiles/common/ -run '^(TestMergeAddressTable|TestRetiredAtMergePlan)$'
```

JS/TS, `task.6.81.1`:

```sh
node --test --test-name-pattern '^hardhatSolcMatchesFoundryPaxeer$' contracts/test/hardhat_solc_parity.test.js
```

Python, `task.6.51.1`:

```sh
python3 -m pytest tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_reports_a_tracked_retired_file tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_reports_a_live_import_outside_the_retired_set tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_ignores_documentation_references -q
```

Solidity, `task.6.82.1`:

```sh
forge test --root . --config-path foundry.paxeer.toml --match-path contracts/test/IBankServedSelectors.t.sol --match-test '^test_IBankDeclaresOnlyServedSelectors$'
```

### 7. rpc-api

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 115 | 145 / 2 | `go.mod:3` | `task.7.0.1` |

Go, `task.7.0.1`:

```sh
go test -count=1 ./rpc/ -run '^(TestServedMethodsIncludesPromotedMethods|TestServedMethodsMatchServerDispatch)$'
```

### 8. platform

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Dotnet | 7 | 19 / 0 | `sdk/platform/dotnet/tests/LayerX.Sdk.Tests/LayerX.Sdk.Tests.csproj:3` | `task.8.43.1` |
| Go | 14 | 29 / 0 | `sdk/platform/go/go.mod:3` | `task.8.41.1` |
| JS/TS | 36 | 81 / 0 | `package.json:2`; `services/platform/examples/marketplace/package.json:2`; `services/platform/middleware/agent/package.json:2`; `services/platform/middleware/buyer/package.json:2`; `services/platform/middleware/conformance/package.json:2`; `services/platform/middleware/merchant/package.json:2`; `services/platform/middleware/seller/package.json:2` | `task.8.21.1` |
| JVM | 10 | 24 / 0 | `sdk/platform/jvm/pom.xml:2` | `task.8.42.1` |
| Python | 34 | 48 / 0 | task-defined runner environment | `task.8.14.2` |
| Rust | 67 | 137 / 8 | task-defined runner environment; `sdk/platform/generators/Cargo.toml:1`; `services/platform/hosted/agent-boundary/Cargo.toml:1`; `services/platform/hosted/authority/Cargo.toml:1`; `services/platform/hosted/core/Cargo.toml:1`; `services/platform/hosted/gateway/Cargo.toml:1`; `services/platform/hosted/indexer/Cargo.toml:1`; `services/platform/hosted/paxeer/Cargo.toml:1`; `services/platform/hosted/registry/Cargo.toml:1`; `services/platform/hosted/testnet/Cargo.toml:1`; `services/platform/hosted/webhooks/Cargo.toml:1`; `services/platform/ramps/reference/Cargo.toml:1`; `services/platform/ramps/toolkit/Cargo.toml:1`; `tools/platform/cli/Cargo.toml:1`; `tools/platform/emulator/Cargo.toml:1`; `tools/platform/release/Cargo.toml:1` | `task.8.0.1` |
| Swift | 7 | 19 / 0 | `sdk/platform/swift/Package.swift:4` | `task.8.44.1` |

Dotnet, `task.8.43.1`:

```sh
dotnet test sdk/platform/dotnet/tests/LayerX.Sdk.Tests/LayerX.Sdk.Tests.csproj --filter 'FullyQualifiedName=LayerX.Sdk.Tests.SequencerReceiptTests.SequencerReceiptV1CasesVerifyReceipted|FullyQualifiedName=LayerX.Sdk.Tests.SequencerReceiptTests.SequencerReceiptV2RevertDecodesReason|FullyQualifiedName=LayerX.Sdk.Tests.SequencerReceiptTests.SequencerReceiptV3RefusalsNameTheirCheck'
```

Go, `task.8.41.1`:

```sh
go -C sdk/platform/go test -count=1 . -run '^(TestSequencerReceiptV1CasesVerifyReceipted|TestSequencerReceiptV2RevertDecodesReason|TestSequencerReceiptV3RefusalsNameTheirCheck)$'
```

JS/TS, `task.8.21.1`:

```sh
node --test --test-name-pattern='^(encodesLedgerDrawCalldataWithDrawSelector|decodesPaymentExactLogFromLedgerPrecompile|refusesLedgerLogFromAnotherAddress)$' services/platform/middleware/seller/test/ledger.test.mjs
```

JVM, `task.8.42.1`:

```sh
mvn -q -f sdk/platform/jvm/pom.xml test -Dtest='SequencerReceiptTest#sequencerReceiptV1CasesVerifyReceipted+sequencerReceiptV2RevertDecodesReason+sequencerReceiptV3RefusalsNameTheirCheck' -Dsurefire.failIfNoSpecifiedTests=true
```

Python, `task.8.14.2`:

```sh
python3 -m pytest services/platform/hosted/tests/test_retired_kernel_scripts.py::test_paxeer_deployment_ships_no_anchor_genesis -q
```

Rust, `task.8.0.1`:

```sh
cargo test --locked --manifest-path services/platform/Cargo.toml -p layerx-platform-gateway --lib -- --exact tests::router_methods_are_sorted_unique_and_namespaced tests::router_method_refuses_every_lx_name
```

Swift, `task.8.44.1`:

```sh
swift test --package-path sdk/platform/swift --filter '^LayerXSDKTests\.SequencerReceiptTests/(testSequencerReceiptV1CasesVerifyReceipted|testSequencerReceiptV2RevertDecodesReason|testSequencerReceiptV3RefusalsNameTheirCheck)$'
```

### 9. agent

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| JS/TS | 18 | 37 / 0 | `services/agent/sdk/typescript/package.json:2` | `task.9.33.10` |
| Python | 6 | 8 / 0 | task-defined runner environment; `services/agent/sdk/python/pyproject.toml:1` | `task.9.36.1` |
| Rust | 122 | 217 / 18 | `services/agent/Cargo.toml:1`; `services/agent/crates/layerx-agent-api/Cargo.toml:1`; `services/agent/crates/layerx-agentd-host/Cargo.toml:1`; `services/agent/crates/layerx-agentd/Cargo.toml:1`; `services/agent/crates/layerx-client/Cargo.toml:1`; `services/agent/crates/layerx-crypto/Cargo.toml:1`; `services/agent/crates/layerx-mcp/Cargo.toml:1`; `services/agent/crates/layerx-proof/Cargo.toml:1`; `services/agent/crates/layerx-sdk/Cargo.toml:1`; `services/agent/crates/layerx-types/Cargo.toml:1`; `services/agent/crates/layerx-wire/Cargo.toml:1`; `services/agent/fuzz/Cargo.toml:1`; `services/agent/qualification/pay2/Cargo.toml:1`; `services/agent/tests/boundary/Cargo.toml:1`; `services/agent/tests/parity/Cargo.toml:1`; `services/agent/tests/qualify/Cargo.toml:1`; `services/agent/tools/boundary-check/Cargo.toml:1`; `services/agent/tools/sdk-gen/Cargo.toml:1` | `task.9.0.1` |

JS/TS, `task.9.33.10`:

```sh
node --test --test-name-pattern='^(placeOrder_call_carries_the_account_id|exchange_wallet_requests_are_type_2_on_chain_125)$' services/agent/sdk/typescript/dist/test/exchange-v2.test.js
```

Python, `task.9.36.1`:

```sh
python3 -m pytest services/agent/sdk/python/tests/test_px_commit.py::test_network_head_has_no_anchor services/agent/sdk/python/tests/test_px_commit.py::test_history_cursor_is_block_position -q
```

Rust, `task.9.0.1`:

```sh
cargo test --locked --manifest-path services/agent/Cargo.toml -p layerx-agent-api --test rung -- --exact rung_and_rung_status_are_exported_from_the_verify_contract legacy_level_maps_onto_rungs_by_decision
```

### 10. human

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Rust | 116 | 175 / 10 | `services/human/crates/layerx-human-kms/Cargo.toml:1`; `services/human/crates/layerx-human-service/Cargo.toml:1`; `services/human/crates/layerx-human-test-support/Cargo.toml:1`; `services/human/crates/layerx-intents/Cargo.toml:1`; `services/human/crates/layerx-network-gateway/Cargo.toml:1`; `services/human/tools/api-gen/Cargo.toml:1`; `services/human/tools/boundary-check/Cargo.toml:1`; `services/human/tools/copy-lint/Cargo.toml:1`; `services/human/tools/schema-check/Cargo.toml:1` | `task.10.0.1` |

Rust, `task.10.0.1`:

```sh
cargo test --locked --manifest-path services/human/Cargo.toml -p layerx-human-service --test frame_transport -- --exact frame_transport_round_trips_a_frame_over_uds_within_limits frame_transport_refuses_a_frame_above_max_frame_bytes
```

### 11. interop

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Python | 7 | 9 / 0 | task-defined runner environment | `task.11.0.4` |
| Rust | 74 | 222 / 2 | `services/interop/Cargo.toml:1`; `services/interop/contracts/solana-mirror/Cargo.toml:1`; `services/interop/crates/layerx-ap2/Cargo.toml:1`; `services/interop/crates/layerx-bridge-relayer/Cargo.toml:1`; `services/interop/crates/layerx-fiat/Cargo.toml:1`; `services/interop/crates/layerx-gas-station/Cargo.toml:1`; `services/interop/crates/layerx-interop-gateway/Cargo.toml:1`; `services/interop/crates/layerx-interop-service/Cargo.toml:1`; `services/interop/crates/layerx-mirror/Cargo.toml:1`; `services/interop/crates/layerx-portable/Cargo.toml:1`; `services/interop/crates/layerx-ucp/Cargo.toml:1`; `services/interop/crates/layerx-visa-tap/Cargo.toml:1`; `services/interop/crates/layerx-x402/Cargo.toml:1`; `services/interop/crates/x-websearch/Cargo.toml:1` | `task.11.0.1` |
| Solidity | 1 | 3 / 0 | `services/interop/contracts/ethereum-mirror/foundry.toml:3` | `task.11.28.1` |

Python, `task.11.0.4`:

```sh
python3 -m pytest services/interop/deploy/gateway/tests/test_env_template.py::test_template_names_every_variable_the_service_reads -q
```

Rust, `task.11.0.1`:

```sh
cargo test --locked --manifest-path services/interop/Cargo.toml -p layerx-interop-gateway --test evidence -- --exact settlement_rung_labels_are_submitted_receipted_and_committed canonical_receipt_digest_matches_the_recorded_pay_exact_vector
```

Solidity, `task.11.28.1`:

```sh
forge test --root services/interop/contracts/ethereum-mirror --match-path '**/test/LayerXMirrorArchive.t.sol' --match-test '^(testBeginRecordsCommittedHeightAndBlockHash|testBeginRejectsZeroBlockHash|testBeginRejectsDifferentBlockHashOnReopen)$'
```

### 12. wallet

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 17 | 37 / 1 | `services/wallet/attestor/go.mod:3` | `task.12.0.2` |
| JS/TS | 75 | 128 / 0 | `apps/wallet/package.json:2`; `sdk/wallet/package.json:2`; `services/wallet/demo/package.json:2`; `services/wallet/gateway/package.json:2` | `task.12.0.1` |
| Rust | 2 | 2 / 0 | `services/wallet/attestor/internal/policy/lx/testdata/kernelvectors/Cargo.toml:1` | `task.12.18.2` |

Go, `task.12.0.2`:

```sh
go -C services/wallet/attestor test -count=1 ./internal/server/ -run '^(TestEVMReviewDisclosesDecodedCall|TestEVMReviewRecordsNoSpend|TestEVMReviewRefusesWrongChain)$'
```

JS/TS, `task.12.0.1`:

```sh
pnpm --dir services/wallet/gateway exec vitest run test/env.test.ts -t '^(sequencerUrlDefaultsToFirstReadUrl|configuredSequencerUrlIsKept)$'
```

Rust, `task.12.18.2`:

```sh
cargo test --locked --manifest-path services/wallet/attestor/internal/policy/lx/testdata/kernelvectors/Cargo.toml --bin attestor-kernel-vectors -- --exact tests::generated_vectors_match_pinned_file
```

### 13. programs

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| C | 1 | 0 / 1 | `Makefile:292` | `task.13.34.1` |
| Go | 51 | 93 / 0 | `go.mod:3` | `task.13.21.1` |
| Python | 2 | 2 / 0 | task-defined runner environment | `task.13.8.1` |
| Rust | 61 | 103 / 6 | `programs/crates/layerx-programs-ai-market/Cargo.toml:1`; `programs/crates/layerx-programs-arbiter/Cargo.toml:1`; `programs/crates/layerx-programs-registry/Cargo.toml:1`; `programs/crates/layerx-programs-runtime/Cargo.toml:1`; `programs/crates/layerx-programs-sandbox/Cargo.toml:1`; `programs/fuzz/Cargo.toml:1` | `task.13.0.1` |

C, `task.13.34.1`:

```sh
make build/tests/programs_parallel_differential && ./build/tests/programs_parallel_differential
```

Go, `task.13.21.1`:

```sh
go test -count=1 ./precompiles/layerxprograms/ -run '^(TestProgramsABIEventTopics|TestProgramsABIKeepsThirtyFiveMethods)$'
```

Python, `task.13.8.1`:

```sh
python3 -m pytest programs/fixtures/pay5/test_native_roundtrip.py::test_native_roundtrip_replays_without_history_feed -q
```

Rust, `task.13.0.1`:

```sh
cargo test --locked --manifest-path programs/Cargo.toml -p layerx-programs-registry --test chain_proto -- --exact chain_proto_reader_decodes_canonical_fields chain_proto_reader_refuses_noncanonical_varint_and_order
```

### 14. web-explorer-bridge

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Elixir | 31 | 42 / 0 | `services/explorer/backend/apps/block_scout_web/mix.exs:4`; `services/explorer/backend/apps/ethereum_jsonrpc/mix.exs:4`; `services/explorer/backend/apps/explorer/mix.exs:4`; `services/explorer/backend/apps/indexer/mix.exs:4` | `task.14.21.1` |
| Go | 1 | 9 / 0 | `go.mod:3` | `task.14.53.1` |
| JS/TS | 111 | 207 / 0 | `apps/bridge/package.json:2`; `apps/explorer/package.json:2`; `apps/web/package.json:2`; `sdk/js/package.json:2` | `task.14.0.1` |
| Python | 6 | 9 / 0 | task-defined runner environment | `task.14.24.2` |
| Rust | 4 | 5 / 0 | `services/explorer/services/sig-provider/sig-provider-server/Cargo.toml:1`; `services/explorer/services/sig-provider/sig-provider/Cargo.toml:1` | `task.14.30.1` |
| Shell | 2 | 0 / 2 | `Makefile:292` | `task.14.53.4` |

Elixir, `task.14.21.1`:

```sh
services/explorer/deploy/tools/mix-in-builder.sh test apps/explorer/test/explorer/chain/paxeer_x/capabilities_retired_probe_test.exs --only 'test:test surfaces_omit_retired_precompiles' --only 'test:test address_refuses_retired_surfaces'
```

Go, `task.14.53.1`:

```sh
go test -count=1 ./services/bridge/deploy/appenv/ -run '^(TestEnvLinesFromEVMRecords|TestRefusesARecordWithAnotherChainID|TestRefusesAZeroVault)$'
```

JS/TS, `task.14.0.1`:

```sh
node --test --test-name-pattern '^(sdk_js_has_no_vendor_pin|package_entries_resolve_under_require|package_types_subpath_resolves_to_runtime_module)$' sdk/js/test/package.test.cjs
```

Python, `task.14.24.2`:

```sh
python3 -m pytest services/explorer/tests/test_paxeer_x_readme_retirements.py::test_readmes_name_no_retired_fetcher -q
```

Rust, `task.14.30.1`:

```sh
cargo test --locked --manifest-path services/explorer/services/sig-provider/Cargo.toml -p sig-provider --lib -- --exact sources::precompiles::tests::selector_resolves_to_exchange_signature sources::precompiles::tests::unknown_selector_returns_empty
```

Shell, `task.14.53.4`:

```sh
bash services/bridge/deploy/tests/workflow-check.sh
```

### 15. cosmos-fork

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Go | 50 | 65 / 0 | `go.mod:3` | `task.15.0.1` |
| JS/TS | 5 | 9 / 0 | `sdk/integrations/agents/package.json:2`; `sdk/integrations/express/package.json:2`; `sdk/integrations/next/package.json:2` | `task.15.0.4` |
| JVM | 4 | 7 / 0 | `sdk/integrations/android/pom.xml:2`; `sdk/integrations/spring/pom.xml:2` | `task.15.4.5` |
| Python | 3 | 6 / 0 | `sdk/integrations/fastapi/pyproject.toml:1` | `task.15.4.2` |
| Swift | 2 | 3 / 0 | `sdk/integrations/ios/Package.swift:4` | `task.15.4.10` |

Go, `task.15.0.1`:

```sh
go test -count=1 ./sdk/testutil/oteltest/ -run '^(TestReaderSeesInstrumentCreatedBeforeInstall|TestLegacySinkCapturesTelemetryCounter)$'
```

JS/TS, `task.15.0.4`:

```sh
node --test --test-name-pattern '^(loadConfigReadsReceiptKeysJson|integrationSettlesSignedSequencerReceipt)$' sdk/integrations/express/test/receipt-keys.test.mjs
```

JVM, `task.15.4.5`:

```sh
mvn -q -f sdk/integrations/spring/pom.xml -Dtest='ReceiptKeysConfigTest#declaredConfigBindsReceiptKeys+autoConfigurationBuildsAuthorityFromReceiptKeys' test
```

Python, `task.15.4.2`:

```sh
PYTHONPATH=sdk/integrations/fastapi:services/agent/sdk/python python3 -m pytest sdk/integrations/fastapi/tests/test_receipt_keys_config.py::test_load_config_reads_receipt_keys sdk/integrations/fastapi/tests/test_receipt_keys_config.py::test_load_config_refuses_authorized_batch_only -q
```

Swift, `task.15.4.10`:

```sh
swift test --package-path sdk/integrations/ios --filter 'LayerXMobileTests.SampleEnvironmentTests/testDecodesSignedReceiptPayload'
```

### 16. tools-ops

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| C | 4 | 4 / 1 | `Makefile:292` | `task.16.21.1` |
| Go | 18 | 23 / 1 | `go.mod:3`; `tools/loadtest/go.mod:3` | `task.16.22.1` |
| Python | 137 | 187 / 0 | task-defined runner environment | `task.16.0.1` |

C, `task.16.21.1`:

```sh
make -C . build/tests/lxp_test_harness && ./build/tests/lxp_test_harness --run harness.run-one-runs-named-case && ./build/tests/lxp_test_harness --run harness.run-one-reports-unknown-case
```

Go, `task.16.22.1`:

```sh
go test -count=1 ./tests/testutil/processblock/ -run '^(TestNewTestAppWithGenesisOverridesModuleSection|TestNewTestAppWithGenesisRejectsUnknownModule)$'
```

Python, `task.16.0.1`:

```sh
python3 -m unittest scripts.gates.test_run_gate.RunGateTest.test_records_evidence_line_for_passing_command scripts.gates.test_run_gate.RunGateTest.test_nonzero_exit_is_recorded_and_returned
```

### 17. programme

| Toolchain | Files | Planned / existing selectors | Manifest boundaries at baseline | Example task |
|---|---:|---:|---|---|
| Python | 66 | 89 / 0 | task-defined runner environment | `task.17.0.1` |

Python, `task.17.0.1`:

```sh
python3 -m pytest spec/layerx-merge/programme/tests/test_kvx.py::test_loads_workflow_sections spec/layerx-merge/programme/tests/test_kvx.py::test_refuses_duplicate_section spec/layerx-merge/programme/tests/test_evidence.py::test_evidence_line_shape spec/layerx-merge/programme/tests/test_evidence.py::test_run_records_nonzero_exit -q
```

## Fixture and service provisioning

Use the orchestration plan’s profiles to prepare isolated task environments before invoking a gate. A database, chain process, peer socket or signing cluster required by the real code path is a fixture, not an optional skip condition. Refuse a missing fixture and record the failure. Keep fixture cleanup in teardown and preserve its failure status.

- Domain 4: allocate one-validator chain fixtures for the launchpad precompile integration tasks, and real database/migration fixtures for service tests. Missing launchpad SQL or schema bundles must be supplied by their owner before the dependent tests can qualify.
- Domain 10: task `10.11.2` extends the real attestor cluster with a secp256k1 key for EIP-1559 signing. The baseline cluster fixture declares its node and signer sets at `services/human/crates/layerx-human-kms/tests/support/attestor_cluster.rs:25` and `services/human/crates/layerx-human-kms/tests/support/attestor_cluster.rs:26`. Provision that cluster and verify recovered-address equality; an ed25519-only fixture does not test the new path. Tasks `10.13.3`, `10.13.4` and `10.13.5` require real test-owned peer sockets for frame, rung, pending-nonce and transaction exchange. Keep the socket path and process lifecycle isolated per task; do not replace the peer protocol with a mocked method return.
- Domain 11: task `11.15.1` establishes the committed-proof workspace and lock. Merkle/header/commit fixtures for `11.2.2`–`11.2.4` must be recorded from the referenced chain implementation, with their source revision, before the Rust assertions can establish independent conformance. No reference-vector generation was executed during this documentation job.
- Domain 14: explorer backend cases need the builder runtime and disposable database described by its wrapper. Browser cases need their package’s declared project, browser binaries and application fixtures; a passing unit runner does not qualify a browser acceptance.
- Domains 1, 2, 3, 5, 6, 7 and 13: precompile/node Go tests that transitively link lxphost need the native dependencies above. Domain 16 owns the shared harness and release scripts; other domains request changes through wiring rather than editing those files.

## Weak-gate checks and execution evidence

The current `build.py` applies `FORBIDDEN_VERIFY` to every task command. It rejects recursive `./...` without a later `-run`, unbounded `-run .`, generic/prefix-only `-run Test` forms covered by its expressions, `|| true`, and `-skip`. It also requires nonempty tests, test files in `touches`, a test-name occurrence in the command, unique task test ownership, and the acceptance/criterion join. These are string and structure checks. They do not discover tests or prove that a selector actually ran.

The contributor contract additionally forbids whole-package/workspace task gates, source scans standing in for behavioral tests, `--no-fail-fast` as the only gate, and pipelines that conceal exit codes. Those obligations exceed the current regex list. Neither a successful J94 fragment check nor a globally valid graph establishes manifest membership, exact Rust names, correct test tags, meaningful assertions, or a nonzero executed-test count.

Before accepting a gate, compare the runner’s executed test names to the task’s selector set and require a result for every selector. Go `-json` run events support this check; task `17.4.3` plans `count_go_tests` and rejects zero-count commands. Rust requires full module-qualified exact names and a positive executed count. Vitest must match the complete title including `describe` prefixes. Node must use actual named `node:test` cases rather than a script that silently ignores the filter. ExUnit `--only test:...` filters require real matching tags; use case lines or explicit declared tags if absent. Unknown C cases must fail dispatch. Missing tools, missing fixtures, skipped cases, zero matches and nonzero exits cannot qualify an acceptance.

A passing exit code is insufficient when the wrong tests ran or the assertions were vacuous. Negative cases must reach the real rejection path; receipt-event assertions must include at least one effect that should produce an event. Retirement checks must distinguish deleted files, removed symbols in surviving files, and runtime refusal of historical addresses. Keep necessary refusal tests after implementation retirement. The replay-matrix release gate remains required; a retirement wiring cannot delete it.

Record an out-of-scope failure in `qualification.kvx` with the six-key format at `spec/layerx-merge/qualification.kvx:1` and `spec/layerx-merge/qualification.kvx:2` (`spec/workflow.kvx:64` entry_keys). Use a unique section per failure; a second finding for the same task takes a `.<n>` suffix. The ledger holds no finding at the baseline and entries are appended during the run; task `17.10.1` gates section uniqueness and the six keys, task `17.10.2` gates the header against the workflow, and task `17.3.3` plans failure serialization. Keep the observed result separate from the cause hypothesis. The following is a format template, not a finding from this job:

```kvx
[finding.<N>.<k>.<m>]
task = "<N>.<k>.<m>"
file = "<repo-relative path>:<line>"
symbol = "<exact test, function, error code, selector or key>"
observed = "At <exact candidate revision>, <complete exact command> exited <actual integer> (log <private evidence reference>): <failing test or zero executed tests>."
assumption = "<one sentence: the unconfirmed cause the task proceeds on>"
severity = "blocker"
```

Preserve the complete command, including `|` inside regex alternatives; never split records on a pipe. For a zero-test result that exits 0, record the actual exit 0 in `observed` and explicitly record that coverage failed; do not relabel it as a pass or invent a different process exit. Keep raw logs and operational inventories in access-controlled evidence storage. Publish only reviewed, sanitized technical findings. A failed task stays incomplete and its dependants stop. Record exact revision, command, status, test identities/count and private log reference for successful gates in their execution evidence; do not use an unrelated successful command to clear a failed acceptance. The release runner already records argv, exit status, log reference and digest at `tools/qualification/release_runner.py:619`.

## Aggregate gates and completion

A task runs its bounded selectors. Aggregate scripts run once per required merged candidate and retain evidence under the wave runner. Creating a script or checking `--list` does not execute its aggregate. Tasks `16.1.1`–`16.1.8` create the scripts in wave 0; `17.5.8` creates their manifest and evidence wrapper; `16.97.4` checks command lists. Tasks `17.5.1`–`17.5.7` sit in their target waves 0–6 (decision.1715) and run wave recipes 0–6 on each merged wave revision; `17.0.9` replays them on the final merged revision. Open item: the wave tasks carry no declared dependency closing every task of their wave, so the dispatcher enforces wave-wide closure before each run. Keep earlier evidence separately: it cannot satisfy the final requirement that all final wave evidence names the same candidate revision.

| Recipe | Execution task | Exact top-level entries, in order |
|---|---|---|
| `scripts/gates/wave-0.sh` | `17.5.1` | `make -n build test; make workspace-build; make workspace-lint` |
| `scripts/gates/wave-1.sh` | `17.5.2` | `make workspace-build; make workspace-test; make core-test-all; make test-sanitizer-suite` |
| `scripts/gates/wave-2.sh` | `17.5.3` | `make workspace-build; make workspace-test; make core-test-all; make agent-test` |
| `scripts/gates/wave-3.sh` | `17.5.4` | `make workspace-build; make workspace-test; make agent-test; make interop-test; make programs-test` |
| `scripts/gates/wave-4.sh` | `17.5.5` | `make workspace-build; make workspace-test; make core-qualification-environment; make beta-driver-test` |
| `scripts/gates/wave-5.sh` | `17.5.6` | `make workspace-build; make workspace-test; make human-test; tools/containers/check.sh` |
| `scripts/gates/wave-6.sh` | `17.5.7` | `make workspace-build; make workspace-test; make workspace-lint; make core-test-all; python3 tools/paxeer-x/tests/retired_callers_test.py --deleted; make programs-test` |
| `scripts/gates/wave-programme.sh` | `17.0.9` | `make workspace-ci; make beta-qualify; python3 tools/paxeer-x/tests/retired_callers_test.py --deleted; scripts/ci/beta-ledger-check.sh --revisions` |

The HEAD definitions were verified at `Makefile:3309`, `Makefile:3313`, `Makefile:3319`, `Makefile:3322`, `Makefile:3276`, `Makefile:3294`, `Makefile:3378`, `Makefile:3739`, `Makefile:3806` and `Makefile:3809`. These definitions include broad release suites and recursive dependencies. “Once” means one invocation of each declared recipe entry, not a promise that every lower-level test executes once: `workspace-test` already depends on `core-test-all`, and other entries repeat some suites. Record the actual command graph and durations; do not start aggregate recipes from each task checkout. Owner changes to retired targets must be reflected in the final script lists before execution.

Every domain completion task `N.0.9` waits for its foundation, surface, integration and client anchors plus its wave-6 tasks. For domains 1–16 the current completion test checks the retired interface set and existence of the wave-6 planned paths. It does not rerun all task tests. The table names the exact single test invocation. The planned-path check supplements, and cannot replace, the recorded behavioral gates.

`1.0.9`:

```sh
make -C . build/tests/kernel/codec/lxp_test_kernel_complete && ./build/tests/kernel/codec/lxp_test_kernel_complete kernel_completion_retired_surfaces_absent
```

`2.0.9`:

```sh
go test -count=1 ./precompiles/layerxledger/ -run '^(TestLedgerCompletionRetiredSurfacesAbsent)$'
```

`3.0.9`:

```sh
go test -count=1 ./modules/pricefeed/client/cli/ -run '^(TestMarketsCompletionRetiredSurfacesAbsent)$'
```

`4.0.9`:

```sh
pnpm --dir contracts/launchpad exec hardhat test test/production/Completion.test.js --grep 'LaunchpadCompletionRetiredSurfacesAbsent'
```

`5.0.9`:

```sh
go test -count=1 ./node/sequencer/ -run '^(TestNodeCompletionRetiredSurfacesAbsent)$'
```

`6.0.9`:

```sh
go test -count=1 ./modules/evm/client/cli/ -run '^(TestChainModulesCompletionRetiredSurfacesAbsent)$'
```

`7.0.9`:

```sh
go test -count=1 ./rpc/ -run '^(TestRpcCompletionRetiredSurfacesAbsent)$'
```

`8.0.9`:

```sh
cargo test --locked --manifest-path services/platform/Cargo.toml -p layerx-platform-gateway --test completion_gate -- --exact platform_completion_retired_surfaces_absent
```

`9.0.9`:

```sh
cargo test --locked --manifest-path services/agent/tests/parity/Cargo.toml --test completion_gate -- --exact agent_completion_retired_surfaces_absent
```

`10.0.9`:

```sh
cargo test --locked --manifest-path services/human/tools/api-gen/Cargo.toml -p layerx-human-api-gen --test completion_gate -- --exact human_completion_retired_surfaces_absent
```

`11.0.9`:

```sh
python3 -m pytest services/interop/deploy/gateway/tests/test_completion_gate.py::test_interop_completion_retired_surfaces_absent -q
```

`12.0.9`:

```sh
pnpm --dir services/wallet/demo exec vitest run src/components/Completion.test.tsx -t '^(walletCompletionRetiredSurfacesAbsent)$'
```

`13.0.9`:

```sh
cargo test --locked --manifest-path programs/Cargo.toml -p layerx-programs-registry --test completion_gate -- --exact programs_completion_retired_surfaces_absent
```

`14.0.9`:

```sh
node --test --test-name-pattern '^(web_completion_retired_surfaces_absent)$' apps/web/e2e/completion-gate.test.ts
```

`15.0.9`:

```sh
node --test --test-name-pattern '^(cosmos_fork_completion_retired_surfaces_absent)$' sdk/integrations/express/test/completion-gate.test.mjs
```

`16.0.9`:

```sh
python3 -m unittest tools.ci.tests.test_completion_gate.CompletionGateTest.test_tools_ops_completion_retired_surfaces_absent
```

Wave 7 also includes dedicated merged-candidate work:

| Task | Runs once on the final candidate |
|---|---|
| `17.1.7` | Each merge bring-up golden command and its expected exit. |
| `17.1.8` | Custody-release and anchor-retirement goldens under the adopted release decision. |
| `17.1.9` | Receipt order: sequence-gap and previous-sequence-hash mismatch refusal goldens. |
| `17.2.7` | Every retired-surface runtime refusal row, requiring nonzero test execution. |
| `17.2.8` | Retired-caller source scan as supporting retirement evidence. |
| `17.3.4` | Migration rehearsal: first sequenced block, exported sequencer state, precompile activation and anchor-store deletion. |
| `17.4.6` | Each parity-inventory row through its toolchain-specific bounded command. |
| `17.0.9` | Programme script once, then all final wave evidence with exit 0 and one identical revision; its own two tests include mixed-revision refusal. |

The programme gate requires domain completions `1.0.9`–`16.0.9` and the listed merged-candidate tasks. Retirement scanning is supporting evidence alongside runtime refusal, migration and parity tests. A script that can only be listed, or a scan that merely finds strings absent, cannot complete behavioral acceptance.

Document delivery has an additional independent gate. The Hyperfide CLI (`cg`) acceptance, `cg spec status` on spec.kvx, is detached from Python validation; delivery requires its recorded exit status to be 0, every task reported, and its result newer than the generated spec. `cg spec lint` stays owed at release. A successful `build.py check` does not waive the detached acceptance or any runtime qualification.

## Findings requiring owner resolution

Resolved in the merged candidate (decision.1724): `8.94.2` and `8.96.4` select `layerx-platform-registry`; the fourteen Rust commands of `8.93.1`, `8.94.1`, `8.94.2`, `8.94.3`, `8.96.1`, `8.96.2`, `8.96.4`, `9.92.2`, `9.94.1`, `9.95.1`, `9.96.2`, `9.97.2`, `9.97.4` and `12.95.1` carry `--locked` and `--exact` with their test names unchanged; task `9.27.6` declares `gate_kind = "existing"` because its four tests are defined at the baseline. The findings below remain open. They do not change any task command or acceptance ownership in this plan.

- The domain 1–16 completion gates describe source retirement and planned-path existence. A symbol can be retired while its containing file remains live; requiring the entire path to disappear would be wrong. Domain 2 has an empty retired-interface set, leaving only path existence. Require meaningful completion evidence from behavioral task results and distinguish file deletion from symbol removal. These gates do not alone prove domain completion.
- Some planned Vitest commands use leaf-only anchored titles, and explorer commands use `--only test:<display name>`. Establish actual full titles/tags before implementing the tests; require executed-name equality and positive counts. Treat these as selector risks until the planned test declarations exist, not as proof that every such command is currently vacuous.
- Of 98 selectors declared existing, 96 have their leaf name present in the cited file. The two C selectors `programs_parallel_differential` and `golden_replay_two_kernels_match` label whole-binary behavior rather than named functions at baseline: `programs/tests/differential/parallel.c:1322` and `tests/replay/lxp_test_golden_replay.c:101`. Task `16.98.1` explicitly adds the replay case argument while retaining its release suite. Review the existing/planned classification and whole-binary gate granularity without discarding those behavioral assertions.

Regenerate the counts and recheck the selector join after any fragment or ownership repair. The source snapshot hash above identifies exactly which candidate these tables describe.
