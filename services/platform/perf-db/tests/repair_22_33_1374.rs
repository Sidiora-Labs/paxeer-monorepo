#[path = "support/repair_22_33_1374.rs"]
mod repair;

#[test]
fn event_effects_all_variants() {
    repair::all_event_variants_process_cleanly();
}

#[test]
fn event_effects_complete_snapshot() {
    repair::snapshot_preserves_all_state();
}

#[test]
fn recovery_tail_interrupted_followup() {
    repair::truncated_wal_recovers_valid_entries();
}

#[test]
fn recovery_tail_corrupt_followup() {
    repair::corrupted_crc_stops_replay_at_corruption();
}

#[test]
fn recovery_tail_interior_corruption_rejected() {
    repair::interior_crc_corruption_is_rejected();
}

#[test]
fn recovery_exposure_modification_live_and_reopen() {
    repair::wal_replay_matches_live_position_state();
}

#[test]
fn event_effects_position_opened() {
    repair::event_variant(0);
}

#[test]
fn event_effects_position_modified() {
    repair::event_variant(1);
}

#[test]
fn event_effects_position_closed() {
    repair::event_variant(2);
}

#[test]
fn event_effects_order_placed() {
    repair::event_variant(3);
}

#[test]
fn event_effects_order_executed() {
    repair::event_variant(4);
}

#[test]
fn event_effects_short_order_placed() {
    repair::event_variant(5);
}

#[test]
fn event_effects_order_cancelled() {
    repair::event_variant(6);
}

#[test]
fn event_effects_liquidation_position_opened() {
    repair::event_variant(7);
}

#[test]
fn event_effects_liquidation() {
    repair::event_variant(8);
}

#[test]
fn event_effects_adl_position_opened() {
    repair::event_variant(9);
}

#[test]
fn event_effects_adl() {
    repair::event_variant(10);
}

#[test]
fn event_effects_price_updated() {
    repair::event_variant(11);
}

#[test]
fn event_effects_funding_settled_projection() {
    repair::event_variant(12);
}

#[test]
fn event_effects_funding_rate_updated() {
    repair::event_variant(13);
}

#[test]
fn event_effects_market_created_projection() {
    repair::event_variant(14);
}

#[test]
fn event_effects_market_paused() {
    repair::event_variant(15);
}

#[test]
fn event_effects_collateral_deposited() {
    repair::event_variant(16);
}

#[test]
fn event_effects_collateral_withdrawn() {
    repair::event_variant(17);
}

#[test]
fn event_effects_vault_created() {
    repair::event_variant(18);
}

#[test]
fn event_effects_vault_deficit_projection() {
    repair::event_variant(19);
}

#[test]
fn snapshot_reopens_alerts() {
    repair::relational_snapshot_roundtrip(0);
}

#[test]
fn snapshot_reopens_api_keys() {
    repair::relational_snapshot_roundtrip(1);
}

#[test]
fn snapshot_reopens_referral_tiers() {
    repair::relational_snapshot_roundtrip(2);
}

#[test]
fn snapshot_reopens_competitions() {
    repair::relational_snapshot_roundtrip(3);
}

#[test]
fn snapshot_reopens_live_candle() {
    repair::relational_snapshot_roundtrip(4);
}

#[test]
fn exposure_long_modification_wal() {
    repair::lifecycle_recovery(true, false, 0);
}

#[test]
fn exposure_long_partial_close_wal() {
    repair::lifecycle_recovery(true, false, 1);
}

#[test]
fn exposure_long_liquidation_wal() {
    repair::lifecycle_recovery(true, false, 2);
}

#[test]
fn exposure_long_modification_snapshot() {
    repair::lifecycle_recovery(true, true, 0);
}

#[test]
fn exposure_long_partial_close_snapshot() {
    repair::lifecycle_recovery(true, true, 1);
}

#[test]
fn exposure_long_liquidation_snapshot() {
    repair::lifecycle_recovery(true, true, 2);
}

#[test]
fn exposure_short_modification_wal() {
    repair::lifecycle_recovery(false, false, 0);
}

#[test]
fn exposure_short_partial_close_wal() {
    repair::lifecycle_recovery(false, false, 1);
}

#[test]
fn exposure_short_liquidation_wal() {
    repair::lifecycle_recovery(false, false, 2);
}

#[test]
fn exposure_short_modification_snapshot() {
    repair::lifecycle_recovery(false, true, 0);
}

#[test]
fn exposure_short_partial_close_snapshot() {
    repair::lifecycle_recovery(false, true, 1);
}

#[test]
fn exposure_short_liquidation_snapshot() {
    repair::lifecycle_recovery(false, true, 2);
}

#[test]
fn event_effects_invalid_events_refused_before_cursor() {
    repair::invalid_events_refused_before_cursor();
}

#[test]
fn event_effects_recorded_producer_roundtrip() {
    repair::recorded_producer_roundtrip();
}

#[test]
fn snapshot_reopens_deleted_alert_allocator() {
    repair::snapshot_preserves_deleted_alert_allocator();
}
