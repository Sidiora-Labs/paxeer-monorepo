# Kernel fuzz and property tests

This directory holds C fuzz harnesses, deterministic property tests, and a codec seed corpus for the Paxeer X kernel embedded in `paxd`. They exercise canonical decoding, signature rejection, arithmetic, transfer conservation, metering, snapshot loading, and reproducible execution results for the network's native state transitions.

## Layout

- `corpus/` — seed inputs; `corpus/codec/seed.bin` is read by the codec smoke executable.
- `fuzz_activity.c` — activity decode determinism, canonical re-encoding, admission rejection, and extraction of activity seeds from a replay corpus.
- `fuzz_batch_header.c` — `LLVMFuzzerTestOneInput` entrypoint for batch-header decoding.
- `fuzz_gateway_json.c` — `LLVMFuzzerTestOneInput` entrypoint for payment-requirement translation and canonical output length.
- `fuzz_lxp_send.c` — `LLVMFuzzerTestOneInput` entrypoint for send decoding.
- `fuzz_meter.c` — `LLVMFuzzerTestOneInput` entrypoint for execution and storage charges, overflow atomicity, and exhaustion.
- `fuzz_signature.c` — mutations of activity signatures, keys, signing domains, network identifiers, and payloads.
- `fuzz_transfer_set.c` — transfer-set determinism, rollback on rejection, and per-asset conservation.
- `lxp_fuzz_activity.c` — activity decode and encode round trips used by protocol tests.
- `lxp_fuzz_arith.c` — byte-input adapter for arithmetic property checks.
- `lxp_fuzz_codec.c` — codec decode and round-trip checks with a standalone seeded smoke driver.
- `lxp_fuzz_signature.c` — Ed25519 signature mutations used by crypto tests.
- `lxp_prop_account_record.c` — account-record round trips, field preservation, and malformed-input rejection.
- `lxp_prop_escrow_codec.c` — escrow-record and economic-result round trips and canonical mutation checks.
- `lxp_prop_fee_params.c` — fee-parameter round trips and mutation checks across supported encoding versions.
- `lxp_prop_kernel_integrated.c` — equal receipts and state roots after permuted module-state loading and writer handoff.
- `lxp_prop_payer_grant.c` — checks that payer-grant codec and authorization symbols are absent from the linked library.
- `lxp_prop_prepare_workers.c` — compares receipts, events, program outcomes, and roots with one and four preparation workers.
- `lxp_prop_reexecution.c` — re-executes recorded activities and compares receipt bytes and state roots.
- `lxp_prop_service_codec.c` — service-record round trips and canonical byte mutations.
- `lxp_prop_snapshot_load.c` — snapshot mutations, truncations, row ordering, canonical reloads, and unchanged state on rejection.
- `lxp_prop_stream_codec.c` — stream-record and payload round trips, mutations, truncations, and extensions.

## Build and test

Run these targets from the repository root using the [root Makefile](../../Makefile). The directory has no separate build manifest. The native sources use C17; codec smoke and arithmetic property targets also require AddressSanitizer and UndefinedBehaviorSanitizer support.

| Command | Coverage |
| --- | --- |
| `make build/tests/test_fuzz_smoke` | Builds the activity, signature, and transfer-set smoke executable. |
| `make qualify-fuzz-run` | Builds and runs that executable against the replay corpus selected by `QUALIFICATION_CORPUS`; the corpus must already be readable. |
| `make qualify-fuzz` | Runs the corpus-based smoke suite and its AddressSanitizer and UndefinedBehaviorSanitizer variants. |
| `make fuzz-codec-smoke` | Builds and runs the seeded codec smoke executable with both sanitizers. |
| `make test-activity-codec` | Builds and runs protocol tests linked with the activity round-trip harness. |
| `make test-crypto-ed25519` | Builds and runs crypto tests linked with signature mutations. |
| `make test-arith-property` | Builds and runs arithmetic property tests and their sanitizer variant. |
| `make test-ledger-send` | Builds and runs ledger send tests linked with the send decoder harness. |
| `make test-metering` | Builds and runs metering tests linked with the metering harness. |
| `make test-gateway` | Builds the gateway JSON harness object and runs payment-requirement tests. |
| `make test-core-ports` | Builds and runs payer-grant symbol checks and the preparation-worker comparison, alongside core port tests. |

The root Makefile's `$(BUILD_DIR)/tests/kernel/%` rule builds individual property-test sources from `tests/%.c`. For example, `make build/tests/kernel/fuzz/lxp_prop_account_record` builds the account-record executable. Property executables take the case names declared in their source as positional arguments through [lxp_kernel_case.h](../codec/lxp_kernel_case.h).

## Configuration

The harness sources do not read environment variables or configuration files directly. The root Makefile accepts `CC`, `AR`, `BUILD_DIR`, `OPT_LEVEL`, `EXTRA_CFLAGS`, `EXTRA_LDFLAGS`, and `RUN_PREFIX` for native builds and execution. `QUALIFICATION_CORPUS` selects the replay corpus and `FUZZ_QUAL_ITERATIONS` controls the corpus-based mutation count. Sanitizer recipes set `ASAN_OPTIONS` and `UBSAN_OPTIONS` and clear `LD_PRELOAD` for AddressSanitizer runs.

The codec smoke executable reads `tests/fuzz/corpus/codec/seed.bin` relative to the repository root. The corpus-based smoke executable receives its replay-corpus path, seed-output directory, and iteration count from the Makefile recipe.

## Documentation

- [Qualification suites and replay-corpus generation](../../docs/QUALIFICATION.md)
- [Monorepo layout and native build boundaries](../../docs/MONOREPO.md)
- [Kernel execution and determinism on the documentation site](../../docs/site/paxeer-docs/public/markdown/layerx-kernel.md)
