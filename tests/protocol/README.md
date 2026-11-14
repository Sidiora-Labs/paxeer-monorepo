# Protocol tests

This directory holds native LayerX protocol and receipt tests, Go qualification checks, host migration coverage, and retained policy evidence checks. The tests exercise the deterministic kernel and its Programs, ledger, authority, receipt, genesis, and migration paths inside the single Paxeer network: the LayerX kernel inside `paxd` executes and orders each transaction and builds each block; validators check and commit it, and a receipt becomes final when the validators commit its block.

## Layout

- `cmd/` — command-line entry points for the shared protocol qualification checks.
- `hostmigration/` — Go tests for host-bound migration through the Programs precompile and host APIs.
- `internal/` — qualification packages shared by the command and Go tests.
- `lxp_test_activity_codec.c` — activity envelope encode, decode, identifier, and malformed-input checks.
- `lxp_test_arena.c` — arena alignment, reset, capacity, and canonical-allocation checks.
- `lxp_test_audit_m966.h` — kernel, transfer, savepoint, arena, and genesis audit cases included by the kernel harness.
- `lxp_test_dispatch.c` — module dispatch, fee transaction, rollback, and retired-module checks.
- `lxp_test_envelope.c` — activity version, network, signing-preimage, and payload-hash checks.
- `lxp_test_fee_gate.c` — fee calculation, fee limits, payment, version, and overflow checks.
- `lxp_test_genesis_storage_retirement.h` — expected Programs registration and storage-retirement activity types.
- `lxp_test_host_bound_migration.h` — native migration admission, authority, transport, fee, and refusal cases.
- `lxp_test_kernel.c` — module registration, epoch transitions, capacity, prepared state, migration, and audit harness entry point.
- `lxp_test_kernel_symbol_isolation.sh` — rejects kernel objects that import `getenv`.
- `lxp_test_module_ctx.c` — module context storage, parameters, effects, transfers, previews, and ledger admission checks.
- `lxp_test_owner_rotation.c` — two-key native owner rotation, account updates, preview, commit, and rollback checks.
- `lxp_test_protocol.c` — protocol versions, network identifiers, limits, and domain-tag checks.
- `lxp_test_receipts.c` — signed receipt, effect, program-artifact, and supply-binding checks.
- `lxp_test_producer_receipts.c` — producer receipt codec and node-return event fixtures.
- `lxp_test_result.c` — result names, domains, fatal classification, and uniqueness checks.
- `lxp_test_sanitizer_smoke.c` — arena allocation smoke and sanitizer-build indicator.
- `lxp_test_smoke.c` — named test-harness registration and unknown-case checks.
- `paxeer_x_rejection_policy.py` — Python entry point for retained node rejection-policy evidence.
- `lxp_check_no_clock.sh` — runs the consensus-source clock and entropy scan.
- `lxp_test_node_receipt_vectors.h` — signed receipt byte fixtures and return-data vectors.
- `clock_and_rejection_policy_test.go` — filesystem clock-scan and retained node admission, execution, commit, and replay tests.

## Build and test

Run commands from the repository root. The root [Makefile](../../Makefile) supplies the native build and test targets; `programs-build` supplies the Programs runtime library used by the kernel, module-context, dispatch, and owner-rotation targets.

- `make build` — builds the native LayerX library used by the tests.
- `make test-protocol` — builds and runs `lxp_test_protocol`.
- `make test-result` — builds and runs `lxp_test_result`.
- `make test-arena` — builds and runs the arena test and its AddressSanitizer variant.
- `make test-harness` — builds and runs the named C test harness, including `lxp_test_smoke.c`.
- `make test-activity-codec` — builds and runs the activity codec test.
- `make test-envelope` — builds and runs the envelope test.
- `make test-fee-gate` — builds and runs the fee-gate test.
- `make test-kernel` — builds and runs the kernel test, including the audit and host-bound migration cases.
- `make test-kernel-symbol-isolation` — checks the kernel object with `lxp_test_kernel_symbol_isolation.sh`.
- `make test-module-ctx` — builds and runs the module-context test.
- `make test-dispatch` — builds and runs the dispatch test.
- `make test-receipts` — builds and runs the receipt test.
- `make test-sanitizer-smoke` — builds and runs the sanitizer smoke test.
- `make test-owner-rotation` — builds and runs the owner-rotation test.
- `make programs-protocol-regression` — runs the root protocol regression group covering kernel, module-context, dispatch, receipts, state-root, and snapshot targets.
- `sh tests/protocol/lxp_check_no_clock.sh` — runs `go run ./tests/protocol/cmd/protocol-policy clock --root .` and checks the four declared consensus source roots.
- `CGO_ENABLED=1 go test -count=1 -run '^TestHostBoundMigration' ./tests/protocol/hostmigration` — runs the Go host-bound migration tests; the same command is invoked by the native migration harness.
- `make test` — runs the root native test aggregate, including result, protocol, arena, harness, activity-codec, envelope, fee-gate, kernel-symbol-isolation, and owner-rotation tests.
- `make -f chain.mk test` — runs the root Go test groups, including the Go tests in this directory; the groups build their native dependencies with `paxd-native-libs`.

## Configuration

- `PATH` — used to locate the executable `rg` for the clock and entropy scan.
- `CGO_ENABLED` — enabled for the Go host-migration test command.
- [Makefile](../../Makefile) — root build variables such as `BUILD_DIR`, `CC`, `CFLAGS`, `CPPFLAGS`, `EXTRA_CFLAGS`, `EXTRA_LDFLAGS`, `RUN_PREFIX`, and `LXP_REVISION` configure native targets.
- [go.mod](../../go.mod) — declares the Go module and dependencies for the command and Go tests.

The policy command accepts `--root` for the source tree. Its `rejection` subcommand requires `--evidence` for a JSON document using the `paxeer-node-rejection-policy/v1` schema and `--revision` for the source revision it must match.

The kernel harness reads genesis inputs from [public-testnet-genesis](../fixtures/public-testnet-genesis/) and [public-testnet-genesis-v3](../fixtures/public-testnet-genesis-v3/), including `genesis-request.lxgb`, `genesis.manifest`, and `00000000000000000000.lxs`.

## Documentation

- [Protocol](../../docs/wiki/Protocol.md) — merged-network execution, activities, authority, Programs, and receipts.
- [Programs workspace](../../docs/wiki/ProgramsWorkspace.md) — Programs build and protocol-regression recipes.
- [LayerX Kernel site page](../../docs/site/paxeer-docs/src/app/layerx-kernel/page.tsx) — the documentation-site page for the deterministic kernel.
