#[path = "support/import_lifecycle.rs"]
mod import_lifecycle;

#[test]
fn import_restart_test_gap_acknowledged_work_is_recovered() {
    import_lifecycle::acknowledged_import_survives_restart();
}

#[tokio::test]
async fn import_restart_test_gap_upstream_refusal_is_observable() {
    import_lifecycle::upstream_refusal_is_observable().await;
}

#[tokio::test]
async fn import_restart_test_gap_single_upstream_refusal_is_observable() {
    import_lifecycle::single_upstream_refusal_is_observable().await;
}
