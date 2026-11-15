use encoding::{fixed_hex, hex_decode, hex_encode};
use layerx_platform_gateway::{verify_activity_operation, AuthorityFacts};
use layerx_platform_webhooks::events::{
    settled_payment, EventId, PaymentDraft, Principal, ProtocolEvent, SubjectId,
};
use serde_json::Value;

pub use layerx_platform_webhooks::{deliveries, endpoints, error, events, http, scheme, trusted};
#[path = "../src/authority_receipt_support.rs"]
mod authority_path_support;
#[path = "../src/boundary.rs"]
pub mod boundary;
#[path = "../src/chain.rs"]
pub mod chain;
#[path = "../src/encoding.rs"]
pub mod encoding;
#[path = "../src/hosted.rs"]
pub mod hosted;
#[path = "../src/routes.rs"]
mod routes;

use hosted::repair_support::{must, Harness};
use trusted::TrustedEvent;

const RECEIPTS: &str = include_str!("../../dashboard/src/fixtures/verified_payment_receipts.json");

fn principal(name: &str) -> Principal {
    must(Principal::new(name))
}

fn payment(principal: &Principal, source: usize, sequence: u64) -> ProtocolEvent {
    let sources: Value = must(serde_json::from_str(RECEIPTS));
    let source = &sources["receipts"][source];
    let field = |name: &str| {
        source[name]
            .as_str()
            .unwrap_or_else(|| panic!("missing {name}"))
    };
    let public_key = must(fixed_hex(field("sequencer_public_key")));
    let operation = must(verify_activity_operation(
        &must(hex_decode(field("receipt_hex"))),
        AuthorityFacts::new(
            must(fixed_hex(field("batch_id"))),
            must(fixed_hex(field("asset"))),
            must(fixed_hex(field("previous_state_root"))),
            must(fixed_hex(field("resulting_state_root"))),
            public_key,
        ),
        &public_key,
        Some(must(fixed_hex(field("activity_id")))),
    ));
    must(settled_payment(PaymentDraft {
        id: must(EventId::new(format!("payment-{sequence}"))),
        principal: principal.clone(),
        subject: must(SubjectId::new("payments")),
        subject_sequence: sequence,
        occurred_at: sequence,
        operation: &operation,
        amount: field("amount").to_owned(),
        asset: field("asset").to_owned(),
    }))
}

fn activity(event: &ProtocolEvent) -> String {
    event
        .facts()
        .iter()
        .find(|fact| fact.name() == "activity_id")
        .unwrap_or_else(|| panic!("activity fact"))
        .value()
        .to_owned()
}

fn trusted(event: ProtocolEvent) -> TrustedEvent {
    must(TrustedEvent::from_validated(event))
}

fn same_activity_event(event: &ProtocolEvent, id: &str, sequence: u64) -> ProtocolEvent {
    let mut value: Value = must(serde_json::to_value(event));
    value["id"] = Value::String(id.into());
    value["subject_sequence"] = Value::from(sequence);
    value["occurred_at"] = Value::from(sequence);
    must(serde_json::from_value(value))
}

fn index_key(principal: &Principal) -> String {
    format!(
        "webhooks:principal:{}:activities",
        hex_encode(&principal.audit_digest())
    )
}

fn version_key(principal: &Principal) -> String {
    format!("{}:version", index_key(principal))
}

#[test]
fn activity_index_tracks_payment_events_and_rebuilds_missing_documents() {
    let harness = Harness::new();
    let service = harness.service(
        "activity-index",
        layerx_platform_webhooks::endpoints::RetryPolicy::hosted(),
    );
    let first = principal("tenant:activity-first");
    let second = principal("tenant:activity-second");
    let event = payment(&first, 0, 1);
    let id = activity(&event);
    must(service.publish(&trusted(event.clone()), 1000));
    let second_event = payment(&second, 1, 1);
    let second_id = activity(&second_event);
    must(service.publish(&trusted(second_event), 1000));

    let stored = must(harness.store.lock());
    let first_index = stored
        .activity_indexes
        .get(&index_key(&first))
        .unwrap_or_else(|| panic!("first activity index"));
    assert_eq!(first_index.len(), 1);
    let event_json = must(serde_json::to_vec(&event));
    assert_eq!(first_index.get(&id), Some(&event_json));
    assert_eq!(
        stored.activity_versions.get(&version_key(&first)),
        Some(&"1".to_owned())
    );
    assert_eq!(
        stored
            .activity_indexes
            .get(&index_key(&second))
            .unwrap()
            .len(),
        1
    );
    assert!(stored.activity_indexes[&index_key(&second)].contains_key(&second_id));
    drop(stored);

    let duplicate = must(service.publish(&trusted(event), 1000));
    assert!(duplicate.duplicate);
    assert_eq!(
        must(harness.store.lock()).activity_indexes[&index_key(&first)].len(),
        1
    );

    let mut other_value: Value = must(serde_json::to_value(&payment(&first, 1, 2)));
    other_value["kind"] = Value::String("journey".into());
    let other: ProtocolEvent = must(serde_json::from_value(other_value));
    must(service.publish(&trusted(other), 1000));
    assert_eq!(
        must(harness.store.lock()).activity_indexes[&index_key(&first)].len(),
        1
    );

    let replacement = same_activity_event(&payment(&first, 0, 3), "payment-replacement", 3);
    let replacement_id = activity(&replacement);
    assert_eq!(replacement_id, id);
    must(service.publish(&trusted(replacement.clone()), 1000));
    assert_eq!(
        must(harness.store.lock()).activity_indexes[&index_key(&first)][&id],
        must(serde_json::to_vec(&replacement))
    );
    let later = payment(&first, 1, 4);
    must(service.publish(&trusted(later), 1000));
    must(service.prune(&first, 1));
    assert!(!must(harness.store.lock()).activity_indexes[&index_key(&first)].contains_key(&id));

    let rebuild_event = payment(&second, 0, 2);
    let rebuild_id = activity(&rebuild_event);
    must(service.publish(&trusted(rebuild_event.clone()), 1000));
    {
        let mut store = must(harness.store.lock());
        store.activity_indexes.remove(&index_key(&second));
        store.activity_versions.remove(&version_key(&second));
    }
    let report = must(service.reconcile_activity_indexes_report());
    assert_eq!(report.rebuilt, 1);
    assert_eq!(must(service.reconcile_activity_indexes_report()).rebuilt, 0);
    assert_eq!(
        must(harness.store.lock()).activity_indexes[&index_key(&second)][&rebuild_id],
        must(serde_json::to_vec(&rebuild_event))
    );
}
