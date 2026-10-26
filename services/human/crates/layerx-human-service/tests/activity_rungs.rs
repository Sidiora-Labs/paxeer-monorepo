use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_agent_api::verify::Level;
use layerx_agent_api::Sequence;
use layerx_human_service::activity::{
    ActivityKind, ActivityStatus, AgentActivity, Feed, FeedError, FilterDraft, PageRequest,
    PendingStatus, VerifiedStatus,
};
use layerx_human_service::notify::ActivityEntryId;
use layerx_human_service::store::{PrincipalScope, PrincipalStore, RowKey, Table};
use layerx_human_test_support as support;
use layerx_types::rung::Rung;

fn record_event(scope: &mut PrincipalScope<'_>, sequence: u8, level: Level) -> ActivityEntryId {
    let entry_id = ActivityEntryId::new(format!("act_rung{sequence}")).expect("entry id");
    let activity = AgentActivity::new(
        entry_id.clone(),
        ActivityKind::AgentAction,
        Some("did:layerx:agent-a".to_owned()),
        u64::from(sequence),
        PendingStatus::Processing,
        VerifiedStatus::Done,
    )
    .expect("activity");
    let delivery = EventDelivery::new(
        EventIdentity::new([sequence; 32]),
        vec![sequence],
        Cursor(Sequence(u64::from(sequence))),
        ReceiptReference::Verified {
            receipt_ref: ReceiptRef::new(format!("receipt-{sequence}")).expect("receipt ref"),
            verification_level: level,
        },
    )
    .expect("delivery");
    let entry = Feed::record_agent_event(scope, &activity, &delivery, u64::from(sequence))
        .expect("record agent event");
    assert_eq!(entry.entry_id(), &entry_id);
    assert_eq!(entry.receipts().len(), 1);
    assert_eq!(
        entry.receipts()[0].rung(),
        level.rung().expect("source rung")
    );
    entry_id
}

#[test]
fn agent_event_is_recorded_and_reread_with_its_rung() {
    let root = support::directory("activity-rungs");
    let tenancy = support::tenancy(&[("alice", "tenant-a")]);
    let retention = support::retention_uniform(10_000);
    let (mut store, digest) = support::install_and_open(&root, &tenancy, retention);
    let alice = support::principal("alice");
    let ids = {
        let mut scope = store.principal(&alice).expect("principal scope");
        let receipted = record_event(&mut scope, 1, Level::BatchIncluded);
        let committed = record_event(&mut scope, 2, Level::CheckpointFinalised);
        let row = scope
            .get(
                Table::Journeys,
                &RowKey::new("activity-feed").expect("feed key"),
            )
            .expect("stored feed");
        let state: serde_json::Value = serde_json::from_slice(row.bytes()).expect("stored JSON");
        assert_eq!(state["revisions"][0]["receipts"][0]["level"], 1);
        assert_eq!(state["revisions"][1]["receipts"][0]["level"], 2);
        [receipted, committed]
    };
    drop(store);

    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen store");
    {
        let scope = store.principal(&alice).expect("reopened scope");
        for (entry_id, expected_rung, expected_status) in [
            (&ids[0], Rung::Receipted, ActivityStatus::Done),
            (&ids[1], Rung::Committed, ActivityStatus::DoneFinalised),
        ] {
            let entry = Feed::entry(&scope, entry_id)
                .expect("reread entry")
                .expect("entry exists");
            assert_eq!(entry.receipts().len(), 1);
            assert_eq!(entry.receipts()[0].rung(), expected_rung);
            assert_eq!(entry.status(), expected_status);
        }
        let page = Feed::new(5)
            .expect("feed")
            .page(
                &scope,
                PageRequest::new(
                    10,
                    Feed::apply_filters(FilterDraft::new()).expect("filters"),
                ),
                2,
                2,
            )
            .expect("feed page");
        assert_eq!(page.entries().len(), 2);
        assert_eq!(page.entries()[0].receipts()[0].rung(), Rung::Committed);
        assert_eq!(page.entries()[1].receipts()[0].rung(), Rung::Receipted);
    }
    drop(store);
    std::fs::remove_dir_all(&root).expect("remove temp store");
}

#[test]
fn stored_verification_byte_above_committed_is_refused() {
    let root = support::directory("activity-corrupt-rung");
    let tenancy = support::tenancy(&[("alice", "tenant-a")]);
    let retention = support::retention_uniform(10_000);
    let (mut store, digest) = support::install_and_open(&root, &tenancy, retention);
    let alice = support::principal("alice");
    let entry_id = {
        let mut scope = store.principal(&alice).expect("principal scope");
        let entry_id = record_event(&mut scope, 1, Level::BatchIncluded);
        let key = RowKey::new("activity-feed").expect("feed key");
        let row = scope.get(Table::Journeys, &key).expect("stored feed");
        let mut state: serde_json::Value =
            serde_json::from_slice(row.bytes()).expect("stored JSON");
        assert_eq!(state["revisions"][0]["receipts"][0]["level"], 1);
        state["revisions"][0]["receipts"][0]["level"] = serde_json::json!(3);
        scope
            .put(
                Table::Journeys,
                key,
                2,
                serde_json::to_vec(&state).expect("encode corruption"),
            )
            .expect("persist corrupt verification byte");
        entry_id
    };
    drop(store);

    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen store");
    {
        let scope = store.principal(&alice).expect("reopened scope");
        assert!(matches!(
            Feed::entry(&scope, &entry_id),
            Err(FeedError::CorruptState)
        ));
        assert!(matches!(
            Feed::projected_agent_head(&scope),
            Err(FeedError::CorruptState)
        ));
        let page = Feed::new(5).expect("feed").page(
            &scope,
            PageRequest::new(
                10,
                Feed::apply_filters(FilterDraft::new()).expect("filters"),
            ),
            2,
            1,
        );
        assert!(matches!(page, Err(FeedError::CorruptState)));
    }
    drop(store);
    std::fs::remove_dir_all(&root).expect("remove temp store");
}
