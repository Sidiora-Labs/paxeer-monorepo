//! Task 22.33.1446 repair record, audit 22.30.82.
//! reputation-proof-tests: reputation_committed_membership_rollback_conflict_and_reopen.
//! snapshot-proof-tests: snapshot_membership_archive_transport_and_restart.
//! binding-proof-tests: binding_consumes_authenticated_store_proof_and_refuses_wrong_keys.
//! backfill-path-tests: tenant_backfill_preserves_chain_id_history_events_and_summary;
//! daemon_observation_order_allows_disabled_index_evm_fallback.
//! Baseline supplied by attempt 1: the declared verify command exited 101;
//! four cases passed and daemon_observation_order_allows_disabled_index_evm_fallback
//! failed with "Comet transaction indexing is unavailable". No new baseline run:
//! the explicit gate order requires complete implementation before qualification.
//! Positive native membership peers use the prerequisite's canonical one-leaf
//! ICS23 namespace fixture with real program codecs and signed block commits.
//! Wrong-program refusals also consume the unmodified keeper-produced transcript
//! programs/tests/vectors/ics23-market-namespaces-app-hash.json.
//! Wiring request: modules/lxphost and modules/sequencer/keeper (node domain)
//! must publish an AI-market shared-state namespace proof transcript for these
//! positive cases; the currently committed native transcript is for another program.
//! The prerequisite 22.33.1444 supplies the Comet-disabled EVM fallback; the
//! regression drives its production daemon_step callback and reopens the store.
//! Qualification evidence and finding mappings: ../src/repair_22_33_1446.kvx.
//! Archive recovery uses the production Endpoint and restore_archived consumer
//! against a durable loopback peer restarted together with the SQLite projector.

#[path = "support/repair_22_33_1446.rs"]
mod repair_paths;

#[test]
fn reputation_committed_membership_rollback_conflict_and_reopen(
) -> Result<(), Box<dyn std::error::Error>> {
    Ok(repair_paths::reputation_committed_membership_rollback_conflict_and_reopen()?)
}

#[test]
fn snapshot_membership_archive_transport_and_restart() -> Result<(), Box<dyn std::error::Error>> {
    Ok(repair_paths::snapshot_membership_archive_transport_and_restart()?)
}

#[test]
fn binding_consumes_authenticated_store_proof_and_refuses_wrong_keys(
) -> Result<(), Box<dyn std::error::Error>> {
    Ok(repair_paths::binding_consumes_authenticated_store_proof_and_refuses_wrong_keys()?)
}

#[test]
fn tenant_backfill_preserves_chain_id_history_events_and_summary(
) -> Result<(), Box<dyn std::error::Error>> {
    Ok(repair_paths::tenant_backfill_preserves_chain_id_history_events_and_summary()?)
}

#[test]
fn daemon_observation_order_allows_disabled_index_evm_fallback(
) -> Result<(), Box<dyn std::error::Error>> {
    Ok(repair_paths::daemon_observation_order_allows_disabled_index_evm_fallback()?)
}
