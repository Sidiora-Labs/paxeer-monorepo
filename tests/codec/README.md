# Codec and kernel contract tests

This directory holds C tests for canonical binary encoding and the native kernel's public contracts, plus a Unicode table generator. The tests exercise serialization, admission, authority, state, receipts and module behavior used by the kernel integrated into the Paxeer network.

## Layout

- `generate_nfc_tables.py` — regenerates the pinned NFC tables in `src/codec/lxp_codec.c` using Python's Unicode 15.0.0 database.
- `lxp_kernel_case.h` — shared assertions and dispatch of explicitly named test cases; missing or unknown case names are refused.
- `lxp_test_asset_state_root.c` — duplicate-key rejection and order-independent asset state roots.
- `lxp_test_binding_authority.c` — bound SEND authority and signature verification.
- `lxp_test_binding_authority_receive.c` — receiver authority, signature verification and binding propagation.
- `lxp_test_binding_budget_signer.c` — budget signer checks with and without bound authority.
- `lxp_test_binding_ctx_stage.c` — authority checks during issuance and account staging.
- `lxp_test_binding_custody_stage.c` — custody staging authority and signature checks.
- `lxp_test_bridge_claims_retired.c` — bridge header and exported-symbol surface checks.
- `lxp_test_bridge_credit_absent.c` — bridge-credit symbol exclusion and deposit nullifier retention.
- `lxp_test_budget_pull_retired.c` — budget-pull refusal and delegated spending behavior.
- `lxp_test_case_dispatch.c` — named-case dispatch and unknown-case rejection.
- `lxp_test_client_reads.c` — read-only module queries and unknown-identifier refusal.
- `lxp_test_codec_composite.c` — byte spans, text, sequences, tags and Unicode NFC conformance.
- `lxp_test_codec_header_version.c` — versioned header round trips and refusal cases.
- `lxp_test_codec_primitives.c` — integer round trips, null-output rejection and truncated reads.
- `lxp_test_codec_strict.c` — strict decoding and malformed-envelope admission without state changes.
- `lxp_test_codec_vectors.c` — valid and adversarial vector decoding, byte round trips and digest checks.
- `lxp_test_custody_asset_id.c` — custody asset identifiers and bounds.
- `lxp_test_custody_builder_header.c` — genesis builder header surface checks.
- `lxp_test_daemon_header_absent.c` — daemon header exclusion.
- `lxp_test_event_log_decl.c` — event-log encoding declaration checks.
- `lxp_test_fee_golden.c` — fee schedule bytes and per-activity fee tables.
- `lxp_test_fee_meter_refused.c` — host fee-meter refusal in execution and preparation.
- `lxp_test_fee_relay_retired.c` — fee-relay refusal and batch fee observation.
- `lxp_test_genesis_builder_header.c` — genesis builder declarations and descriptor checks.
- `lxp_test_global_sequence.c` — global sequence gap and reuse rejection.
- `lxp_test_grant_fee_asset.c` — governed fee-asset resolution and mismatch refusal.
- `lxp_test_handover_absent.c` — handover symbol exclusion.
- `lxp_test_handover_retired.c` — handover activity refusal and governance fee-key preservation.
- `lxp_test_idempotency_absent.c` — symbol exclusion and journal commit behavior.
- `lxp_test_idempotency_retired.c` — key reuse, sequence refusal and idempotency table behavior.
- `lxp_test_kernel_complete.c` — kernel surface and source-path checks.
- `lxp_test_kernel_entry_retired.c` — kernel entry symbol exclusion and blob admission.
- `lxp_test_kernel_writer.c` — writer ownership transfer and open-journal refusal.
- `lxp_test_ledger_bridge_credit_absent.c` — ledger bridge-credit exclusion and snapshot restoration.
- `lxp_test_ledger_journal.c` — journal rollback and state transitions.
- `lxp_test_log_kinds.c` — log-kind append and read compatibility.
- `lxp_test_migration_authorization.c` — migration authorization encoding and refusal cases.
- `lxp_test_module_bounds.c` — module key/value and staged-write bounds.
- `lxp_test_module_kv_load.c` — ordered loading, duplicate rejection and bounds.
- `lxp_test_modules_unregistered.c` — module registration and disabled read surfaces.
- `lxp_test_program_signer_binding.c` — Programs signer binding and receipt preservation on refusal.
- `lxp_test_programs_header.c` — Programs header surface checks.
- `lxp_test_programs_reads.c` — stored interface and sandbox reads.
- `lxp_test_programs_retired.c` — Programs symbol and header surface checks.
- `lxp_test_qual_header.c` — qualification corpus activity counts and header declarations.
- `lxp_test_queue_disposition_retired.c` — queue codec surface and pending receipt publication.
- `lxp_test_receipt_authority_v2.c` — authority record bytes and height validation.
- `lxp_test_receive_retired.c` — receive and grant activity refusal and symbol exclusion.
- `lxp_test_result_codes.c` — result-code names and module domains.
- `lxp_test_result_domain.c` — result registration and unknown-domain behavior.
- `lxp_test_result_registry.c` — ordered result registry enumeration and unique values.
- `lxp_test_result_retired.c` — unregistered result-code checks.
- `lxp_test_sandbox_expiry_bound.c` — sandbox expiry sweep bounds.
- `lxp_test_sandbox_lifecycle.c` — sandbox execution, funding, activation, occupancy and rollback.
- `lxp_test_sandbox_receipt.c` — escrow charge binding and monetary transfer validation.
- `lxp_test_send_store.c` — SEND store replay and refusal cases.
- `lxp_test_service_arbiter.c` — linked arbiters and dispute-window rules.
- `lxp_test_service_deliverable.c` — deliverable reference and hash validation.
- `lxp_test_service_runtime.c` — runtime availability and reader invocation.
- `lxp_test_settled_maintenance.c` — maintenance batch commitment, atomicity and publication recovery.
- `lxp_test_snapshot_golden.c` — snapshot digest and reload equivalence.
- `lxp_test_snapshot_idempotency.c` — snapshot and root treatment of idempotency state.
- `lxp_test_state_diff_retired.c` — state-diff codec surface checks.
- `lxp_test_state_feed_unbound.c` — Programs runtime and root behavior without a state feed.
- `lxp_test_state_record.c` — duplicate-account rejection in state records.
- `lxp_test_stream_accrued_read.c` — accrued-value reads for time and metered streams.
- `lxp_test_surface_frozen.c` — result registry and module identifier stability.
- `lxp_test_verify_header.c` — verification header surface checks.
- `lxp_test_web_absent.c` — web symbol exclusion.

## Build and test

Run commands from the repository root using the [root Makefile](../../Makefile). There is no separate Makefile or package manifest in this directory.

The Makefile builds the standalone codec executables with these targets:

```sh
make build/tests/lxp_test_codec_primitives
make build/tests/lxp_test_codec_composite
make build/tests/lxp_test_codec_strict
make build/tests/lxp_test_codec_vectors
```

The focused targets build their prerequisites and run the tests:

```sh
make -s test-codec test-codec-limits test-codec-vectors test-codec-version
```

`test-codec` runs primitive encoding tests and selected authority, bridge, budget, builder and fee cases. `test-codec-limits` runs composite encoding and NFC checks. `test-codec-vectors` reads the valid and adversarial vector files. `test-codec-version` runs strict decoding and malformed-envelope admission checks. These targets are also dependencies of the root `test` target.

Other kernel tests use the Makefile's `$(BUILD_DIR)/tests/kernel/%` rule for `tests/%.c`. Tests using `lxp_kernel_case.h` require case names on the command line; archive-inspection tests can also require a linked-library argument or the compile-time `LXP_TEST_LIBRARY_PATH` macro.

## Configuration

The sources in this directory read no environment variables or runtime configuration files. Tests use in-memory setup, command-line case selection and repository fixtures. Run from the repository root so fixture and header paths resolve.

- `tests/vectors/codec/valid.lxv` and `tests/vectors/codec/adversarial.lxv` supply codec cases.
- `tests/vectors/codec/NormalizationTest-15.0.0.txt` supplies NFC conformance cases.
- `tests/fixtures/public-testnet-genesis/genesis.manifest` supplies genesis data for fee and snapshot checks.

`LXP_TEST_LIBRARY_PATH` is a compile-time archive path used by symbol-inspection tests, not an environment variable. The Unicode table generator requires Python's Unicode 15.0.0 database and reads and updates `src/codec/lxp_codec.c`.

## Documentation

- [Protocol](../../docs/wiki/Protocol.md) describes the native activity envelope and its role in the merged network.
- [LayerX Kernel site page](../../docs/site/paxeer-docs/public/markdown/layerx-kernel.md) describes canonical encoding, admission, execution and receipts.
