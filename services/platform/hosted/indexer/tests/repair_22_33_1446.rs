//! Membership, archive, binding, and backfill integration cases.

#[path = "support/repair_22_33_1446.rs"]
pub mod repair_paths;

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
