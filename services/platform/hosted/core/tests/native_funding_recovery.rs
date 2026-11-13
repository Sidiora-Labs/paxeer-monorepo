pub mod native_funding {
    include!("../src/main.rs");
    include!("support/native_funding.rs");
}

#[test]
fn native_funding_integration_gap_lost_response_reopen_and_archive_import() {
    native_funding::native_funding_lost_response_reopen_archive_import();
}

#[test]
fn funding_intent_fixture_gap_live_producer_import_and_raw_replay() {
    native_funding::native_funding_intent_producer_import_replay_and_refusals();
}
