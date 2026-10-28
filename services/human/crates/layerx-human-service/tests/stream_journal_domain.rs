#[path = "../src/lib.rs"]
mod service;
pub use service::*;

#[path = "../src/event_producer.rs"]
mod event_producer;
#[path = "../src/time.rs"]
mod time;

use layerx_human_test_support as test_support;
use layerx_proof::checkpoint::SettlementDomain;
use serde_json::{json, Value};
use service::server::stream_journal::StreamJournal;
use sha2::{Digest as _, Sha256};
use std::fs;
use std::path::Path;
use store::{
    AgentTenantId, PrincipalId, PrincipalScope, PrincipalStore, RetentionPeriod, RetentionPolicy,
    RowKey, Table, TenancyDigest, TenancyMap,
};

fn stream_journal() -> StreamJournal {
    StreamJournal::new([37; 32], SettlementDomain::new(125, [0x55; 20]))
}

fn row_key(value: &str) -> RowKey {
    RowKey::new(value).expect("journal row key")
}

fn retention() -> RetentionPolicy {
    RetentionPolicy {
        journeys: RetentionPeriod::new(10_000),
        notifications: RetentionPeriod::new(10_000),
        audit: RetentionPeriod::new(10_000),
        telemetry: RetentionPeriod::new(10_000),
        cache: RetentionPeriod::new(10_000),
    }
}

fn install_store(
    root: &Path,
    retention: RetentionPolicy,
) -> (PrincipalStore, TenancyDigest, PrincipalId) {
    let principal = PrincipalId::new("alice").expect("principal");
    let tenant = AgentTenantId::new("tenant-a").expect("tenant");
    let tenancy = TenancyMap::new([(principal.clone(), tenant)]).expect("tenancy mapping");
    let digest = tenancy.install(root).expect("install tenancy");
    let store = PrincipalStore::open(root, retention, digest).expect("open journal store");
    (store, digest, principal)
}

fn record(scope: &PrincipalScope<'_>, sequence: u64) -> Value {
    let key = row_key(&format!("stream-event-{sequence:016x}"));
    serde_json::from_slice(scope.get(Table::Stream, &key).expect("journal row").bytes())
        .expect("journal JSON")
}

fn cursor(response: &Value, field: &str) -> String {
    response[field].as_str().expect("cursor").to_owned()
}

#[test]
fn stream_journal_record_has_no_settlement_domain() {
    let root = test_support::directory("stream-journal-domain");
    let retention = retention();
    let (mut store, digest, principal) = install_store(&root, retention);
    let journal = stream_journal();
    let start;
    let payload = json!({"notification":{"notification_id":"ntf_first","read":false}});
    {
        let mut scope = store.principal(&principal).expect("principal scope");
        start = cursor(&journal.open(&scope).expect("open journal"), "cursor");
        StreamJournal::append(
            &mut scope,
            "notification:first",
            "notification",
            17,
            payload.clone(),
        )
        .expect("append journal");
        let written = record(&scope, 1);
        assert!(written.get("settlement_domain").is_none());
        assert_eq!(
            written,
            json!({
                "sequence":1,"source":"notification:first","kind":"notification",
                "observed_at":17,"payload":payload
            })
        );
    }
    drop(store);

    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen journal store");
    {
        let scope = store.principal(&principal).expect("reopened principal");
        assert!(record(&scope, 1).get("settlement_domain").is_none());
        let page = journal.next(&scope, &start).expect("read journal");
        assert_eq!(page["events"].as_array().expect("events").len(), 1);
        assert_eq!(page["events"][0]["kind"], "notification");
        assert_eq!(page["events"][0]["notification"], payload["notification"]);
        assert_eq!(page["events"][0]["observed_at"], "1970-01-01T00:00:17Z");
        let next = cursor(&page, "next_cursor");
        assert_eq!(
            cursor(&journal.open(&scope).expect("head cursor"), "cursor"),
            next
        );
        assert_eq!(
            journal.next(&scope, &next).expect("read once")["events"],
            json!([])
        );
    }
    drop(store);
    fs::remove_dir_all(root).expect("remove temporary journal");
}

#[test]
fn legacy_stream_journal_record_is_read_once() {
    let root = test_support::directory("stream-journal-legacy-domain");
    let retention = retention();
    let (mut store, digest, principal) = install_store(&root, retention);
    let journal = stream_journal();
    let start;
    let legacy_end;
    {
        let mut scope = store.principal(&principal).expect("principal scope");
        start = cursor(&journal.open(&scope).expect("open empty journal"), "cursor");
        let mut rows = Vec::new();
        for sequence in 1_u64..=2 {
            let source = format!("notification:legacy-{sequence}");
            let legacy = json!({
                "sequence":sequence,"source":source,"kind":"notification",
                "observed_at":sequence,"settlement_domain":"paxeer",
                "payload":{"notification":{"notification_id":format!("ntf_{sequence}")}}
            });
            rows.push((
                Table::Stream,
                row_key(&format!("stream-event-{sequence:016x}")),
                serde_json::to_vec(&legacy).expect("encode HEAD-format legacy record"),
            ));
            rows.push((
                Table::Stream,
                row_key(&format!(
                    "stream-source-{:x}",
                    Sha256::digest(source.as_bytes())
                )),
                sequence.to_be_bytes().to_vec(),
            ));
        }
        rows.push((
            Table::Stream,
            row_key("stream-head"),
            2_u64.to_be_bytes().to_vec(),
        ));
        scope.put_batch(9, rows).expect("persist legacy journal");
    }
    drop(store);

    let mut store = PrincipalStore::open(&root, retention, digest).expect("reopen legacy journal");
    {
        let mut scope = store.principal(&principal).expect("legacy principal");
        let page = journal.next(&scope, &start).expect("read legacy journal");
        assert_eq!(page["events"].as_array().expect("legacy events").len(), 2);
        for sequence in 1_u64..=2 {
            assert_eq!(record(&scope, sequence)["settlement_domain"], "paxeer");
            assert_eq!(
                page["events"][(sequence - 1) as usize]["notification"]["notification_id"],
                format!("ntf_{sequence}")
            );
        }
        legacy_end = cursor(&page, "next_cursor");
        assert_eq!(
            journal.next(&scope, &legacy_end).expect("legacy read once")["events"],
            json!([])
        );
        StreamJournal::append(
            &mut scope,
            "notification:current-3",
            "notification",
            20,
            json!({"notification":{"notification_id":"ntf_3"}}),
        )
        .expect("append and rewrite legacy");
        for sequence in 1_u64..=3 {
            assert!(record(&scope, sequence).get("settlement_domain").is_none());
        }
    }
    drop(store);

    let mut store =
        PrincipalStore::open(&root, retention, digest).expect("reopen migrated journal");
    {
        let mut scope = store.principal(&principal).expect("migrated principal");
        let mut migrated = Vec::new();
        for sequence in 1_u64..=2 {
            let key = row_key(&format!("stream-event-{sequence:016x}"));
            let row = scope.get(Table::Stream, &key).expect("retained legacy row");
            assert_eq!(row.written_at(), 9);
            let value = record(&scope, sequence);
            assert!(value.get("settlement_domain").is_none());
            assert_eq!(value["sequence"], sequence);
            assert_eq!(value["observed_at"], sequence);
            assert_eq!(value["source"], format!("notification:legacy-{sequence}"));
            migrated.push(row.bytes().to_vec());
        }
        let after = journal
            .next(&scope, &legacy_end)
            .expect("next appended event");
        assert_eq!(after["events"].as_array().expect("new events").len(), 1);
        assert_eq!(
            after["events"][0]["notification"]["notification_id"],
            "ntf_3"
        );
        let end = cursor(&after, "next_cursor");
        StreamJournal::append(
            &mut scope,
            "notification:legacy-1",
            "notification",
            21,
            json!({"notification":{"notification_id":"duplicate"}}),
        )
        .expect("idempotent legacy source");
        assert_eq!(
            cursor(&journal.open(&scope).expect("unchanged head"), "cursor"),
            end
        );
        StreamJournal::append(
            &mut scope,
            "notification:current-4",
            "notification",
            22,
            json!({"notification":{"notification_id":"ntf_4"}}),
        )
        .expect("append after migration");
        for (index, bytes) in migrated.iter().enumerate() {
            let key = row_key(&format!("stream-event-{:016x}", index + 1));
            let row = scope.get(Table::Stream, &key).expect("one-time migration");
            assert_eq!(row.bytes(), bytes);
            assert_eq!(row.written_at(), 9);
        }
        let all = journal
            .next(&scope, &start)
            .expect("read complete ordered journal");
        let ids: Vec<_> = all["events"]
            .as_array()
            .expect("all events")
            .iter()
            .map(|event| {
                event["notification"]["notification_id"]
                    .as_str()
                    .expect("notification id")
            })
            .collect();
        assert_eq!(ids, ["ntf_1", "ntf_2", "ntf_3", "ntf_4"]);
        assert!(record(&scope, 4).get("settlement_domain").is_none());
    }
    drop(store);
    fs::remove_dir_all(root).expect("remove temporary journal");
}
