//! Task 22.33.1538 repair record (audit 22.30.95, criterion 22998.1).
//!
//! The declared gate executes the original tests below, including their success,
//! refusal, reservation, sequence, no-mutation and restart assertions.
//!
//! Finding 22: `signing::session_key_self_signing_is_distinct_and_audited`,
//! `signing::unprovisioned_scope_exceeding_and_expired_self_signing_are_refused`,
//! `signing::durable_revocation_refuses_loaded_signers_and_survives_reopen`.
//! Finding 19: `unknown::acknowledgement_loss_is_resolved_only_by_the_existing_receipt`,
//! `unknown::restart_preserves_backoff_age_and_a_receipt_can_appear_minutes_later`,
//! `unknown::missing_or_changed_payment_denomination_never_resolves_unknown`.
//! Finding 16: `unknown_receipt_lookup::unknown_submission_resolves_to_executed_from_a_signed_receipt`,
//! `unknown_receipt_lookup::missing_or_changed_payment_denomination_refuses_authenticated_lookup_and_recovery`.
//! Finding 15: `unknown_receipt_lookup::unknown_submission_with_consumed_nonce_fails`,
//! `unknown_receipt_lookup::published_key_windows_are_enforced_after_reconnect`.
//! Finding 24: all original five `w7_core_time` tests and
//! `w7_core_time::retained_evm_call_reopens_with_exact_expiry_bytes_and_reservations`,
//! fed by the production ledger EIP-1559 preparation, call disclosure binder and
//! exact-byte signature verifier.
//!
//! Continued on baseline fd4e58297c939b75befb24cd4397def01390d2fd, including
//! prerequisite 22.31.550, with the previous implementation retained.
//! The supplied previous gate record
//! for candidate bd60dbdc23f4838427bfc6e5ea4cc674ac1e92e4 reported 14 passing and
//! 8 failing cases, including the denomination-refusal test in unknown.rs and
//! the receipt-lookup transcripts. No additional baseline execution is claimed:
//! the explicit gate order forbids a separate verify run. Valid authenticated
//! lookups now serve `pax_sequencerReceipt` followed by `eth_getTransactionReceipt`;
//! absent receipts and rejected denominations or keys stop before the second RPC.
//! The unchanged production path is `outbox::unknown::resolve_unknown` ->
//! `layerx_client::rpc::receipt::lookup_authenticated` ->
//! `SequencerEndpoint::sequencer_receipt` -> `sequencer::executed`. That decoder
//! requires paymentDenom and its digest to match the signed asset; a malformed
//! receipt leaves the resolver Unknown without recording execution evidence.
//! The bounded hf-done record supplies the verified candidate revision, exact
//! command, exit status and private evidence location.
//! First bounded verification at bcb21366d3c55d5cf303eb8e6c12d5bf0a51d09c
//! failed (exit 1; /out/logs/verify-1.log): 9 passed, 13 failed, including all
//! five time cases, four receipt-lookup cases and four unknown-resolution cases.
//! Gotcha: the asset precedes amount in the receipt codec; the retained fixtures
//! put paymentDenom's digest in `transfer_set_root` while asset still covered upax.
//! Both producers now encode and decode-check the actual signed asset, preserving
//! the missing/changed denomination assertions and the two-step lookup transcript.
//! Gotcha: retained session metadata must have canonical envelope bytes and its
//! own bound disclosure; substituting unsigned EVM bytes makes `lifecycle_facts`
//! refuse it. The compatibility metadata is derived from the verified call, with
//! exact actor, authority, scope, idempotency and effective expiry assertions.
//! The additional durable call case registers, signs, closes and reopens the real
//! call lifecycle, then proves exact recovery, expiry refusal without mutation,
//! equality admission, generation refusal and retention at the sequence horizon.
//!
//! The baseline shared producer in tests/support/mod.rs already emits paymentDenom;
//! the in-scope local producer keeps the signed payment denomination distinct from
//! feeDenom so the changed-denomination refusal cannot accidentally match the asset.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd/src/human_runtime.rs
//! must migrate its receipt fixtures to the same current node receipt field set.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd/src/session_control.rs
//! must accept `EvmPrepared` in its authorization adapter; its current retained
//! metadata API requires a separately bound envelope, as documented by 22.33.1543.
//! Current consumer contract: signingKey is mandatory in the exact receipt view
//! in layerx-client/src/sequencer.rs, so the repaired producers retain that field.

#[path = "suites/signing.rs"]
mod signing;
#[path = "suites/unknown.rs"]
mod unknown;
#[path = "unknown_receipt_lookup.rs"]
mod unknown_receipt_lookup;
#[path = "suites/w7_core_time.rs"]
mod w7_core_time;

mod support;
use support::send_authorization;
