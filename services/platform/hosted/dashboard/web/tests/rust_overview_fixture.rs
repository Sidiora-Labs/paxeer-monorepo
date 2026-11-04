use layerx_platform_dashboard::{
    DeliverySummary, KeyView, Overview, PaymentView, RequestOutcome, RequestRecord, RequestSummary,
    UsageSummary,
};
use layerx_platform_webhooks::events::{ProtocolFact, Verification};

#[test]
fn rust_overview_serialization_matches_web_fixture() -> Result<(), Box<dyn std::error::Error>> {
    let overview = Overview {
        principal: "developer-fixture".to_owned(),
        generated_at: 2,
        usage: UsageSummary {
            keys: 1,
            live_keys: 1,
            disabled_keys: 0,
            requests_allowed: 10,
            requests_used: 1,
            requests_remaining: 9,
            utilisation_per_mille: 100,
        },
        keys: vec![KeyView {
            key_id: "key-fixture".to_owned(),
            principal: "developer-fixture".to_owned(),
            disabled: false,
            requests_per_window: 10,
            window_seconds: 60,
            used_in_window: 1,
            remaining_in_window: 9,
            window_started_at: 0,
            window_resets_at: 60,
            window_lapsed: false,
            utilisation_per_mille: 100,
        }],
        requests: RequestSummary {
            records: 1,
            completed: 1,
            first_at: Some(1),
            last_at: Some(1),
            ..RequestSummary::default()
        },
        recent_requests: vec![RequestRecord {
            at: 1,
            operation: None,
            operation_digest: "operation-fixture".to_owned(),
            outcome: RequestOutcome::Completed,
            verification: Verification::Unverified,
        }],
        deliveries: DeliverySummary::default(),
        endpoints: vec![],
        dead_letters: vec![],
        payments: vec![PaymentView {
            event: "payment-fixture".to_owned(),
            subject: "subject-fixture".to_owned(),
            subject_sequence: 1,
            occurred_at: 1,
            amount: Some("10".to_owned()),
            asset: None,
            verification: Verification::Unverified,
            settlement_verification: Verification::Unverified,
            receipt_digest: None,
            settled: false,
            facts: vec![ProtocolFact::unverified("state", "pending")?],
        }],
    };
    let serialized = serde_json::to_string(&overview)?;
    let emitted: serde_json::Value = serde_json::from_str(&serialized)?;
    let fixture: serde_json::Value =
        serde_json::from_str(include_str!("fixtures/rust-overview.json"))?;
    assert_eq!(emitted, fixture);
    Ok(())
}
