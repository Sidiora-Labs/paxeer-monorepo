use layerx_human_service as service;
pub use layerx_human_service::*;

use layerx_human_test_support as test_support;
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
    StreamJournal::new([37; 32])
}

fn row_key(value: &str) -> RowKey {
    RowKey::new(value).or_fail("journal row key")
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
    let principal = PrincipalId::new("alice").or_fail("principal");
    let tenant = AgentTenantId::new("tenant-a").or_fail("tenant");
    let tenancy = TenancyMap::new([(principal.clone(), tenant)]).or_fail("tenancy mapping");
    let digest = tenancy.install(root).or_fail("install tenancy");
    let store = PrincipalStore::open(root, retention, digest).or_fail("open journal store");
    (store, digest, principal)
}

fn record(scope: &PrincipalScope<'_>, sequence: u64) -> Value {
    let key = row_key(&format!("stream-event-{sequence:016x}"));
    serde_json::from_slice(
        scope
            .get(Table::Stream, &key)
            .or_fail("journal row")
            .bytes(),
    )
    .or_fail("journal JSON")
}

fn cursor(response: &Value, field: &str) -> String {
    response[field].as_str().or_fail("cursor").to_owned()
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
        let mut scope = store.principal(&principal).or_fail("principal scope");
        start = cursor(&journal.open(&scope).or_fail("open journal"), "cursor");
        StreamJournal::append(
            &mut scope,
            "notification:first",
            "notification",
            17,
            payload.clone(),
        )
        .or_fail("append journal");
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

    let mut store = PrincipalStore::open(&root, retention, digest).or_fail("reopen journal store");
    {
        let scope = store.principal(&principal).or_fail("reopened principal");
        assert!(record(&scope, 1).get("settlement_domain").is_none());
        let page = journal.next(&scope, &start).or_fail("read journal");
        assert_eq!(page["events"].as_array().or_fail("events").len(), 1);
        assert_eq!(page["events"][0]["kind"], "notification");
        assert_eq!(page["events"][0]["notification"], payload["notification"]);
        assert_eq!(page["events"][0]["observed_at"], "1970-01-01T00:00:17Z");
        let next = cursor(&page, "next_cursor");
        assert_eq!(
            cursor(&journal.open(&scope).or_fail("head cursor"), "cursor"),
            next
        );
        assert_eq!(
            journal.next(&scope, &next).or_fail("read once")["events"],
            json!([])
        );
    }
    drop(store);
    fs::remove_dir_all(root).or_fail("remove temporary journal");
}

fn assert_migrated_journal(
    scope: &mut PrincipalScope<'_>,
    journal: &StreamJournal,
    start: &str,
    legacy_end: &str,
) {
    let mut migrated = Vec::new();
    for sequence in 1_u64..=2 {
        let key = row_key(&format!("stream-event-{sequence:016x}"));
        let row = scope
            .get(Table::Stream, &key)
            .or_fail("retained legacy row");
        assert_eq!(row.written_at(), 9);
        let value = record(scope, sequence);
        assert!(value.get("settlement_domain").is_none());
        assert_eq!(value["sequence"], sequence);
        assert_eq!(value["observed_at"], sequence);
        assert_eq!(value["source"], format!("notification:legacy-{sequence}"));
        migrated.push(row.bytes().to_vec());
    }
    let after = journal
        .next(scope, legacy_end)
        .or_fail("next appended event");
    assert_eq!(after["events"].as_array().or_fail("new events").len(), 1);
    assert_eq!(
        after["events"][0]["notification"]["notification_id"],
        "ntf_3"
    );
    let end = cursor(&after, "next_cursor");
    StreamJournal::append(
        scope,
        "notification:legacy-1",
        "notification",
        21,
        json!({"notification":{"notification_id":"duplicate"}}),
    )
    .or_fail("idempotent legacy source");
    assert_eq!(
        cursor(&journal.open(scope).or_fail("unchanged head"), "cursor"),
        end
    );
    StreamJournal::append(
        scope,
        "notification:current-4",
        "notification",
        22,
        json!({"notification":{"notification_id":"ntf_4"}}),
    )
    .or_fail("append after migration");
    for (index, bytes) in migrated.iter().enumerate() {
        let key = row_key(&format!("stream-event-{:016x}", index + 1));
        let row = scope.get(Table::Stream, &key).or_fail("one-time migration");
        assert_eq!(row.bytes(), bytes);
        assert_eq!(row.written_at(), 9);
    }
    let all = journal
        .next(scope, start)
        .or_fail("read complete ordered journal");
    let ids: Vec<_> = all["events"]
        .as_array()
        .or_fail("all events")
        .iter()
        .map(|event| {
            event["notification"]["notification_id"]
                .as_str()
                .or_fail("notification id")
        })
        .collect();
    assert_eq!(ids, ["ntf_1", "ntf_2", "ntf_3", "ntf_4"]);
    assert!(record(scope, 4).get("settlement_domain").is_none());
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
        let mut scope = store.principal(&principal).or_fail("principal scope");
        start = cursor(
            &journal.open(&scope).or_fail("open empty journal"),
            "cursor",
        );
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
                serde_json::to_vec(&legacy).unwrap_or_else(|error| {
                    panic!("{}: {error:?}", "encode HEAD-format legacy record")
                }),
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
        scope.put_batch(9, rows).or_fail("persist legacy journal");
    }
    drop(store);

    let mut store = PrincipalStore::open(&root, retention, digest).or_fail("reopen legacy journal");
    {
        let mut scope = store.principal(&principal).or_fail("legacy principal");
        let page = journal.next(&scope, &start).or_fail("read legacy journal");
        assert_eq!(page["events"].as_array().or_fail("legacy events").len(), 2);
        for sequence in 1_u64..=2 {
            assert_eq!(record(&scope, sequence)["settlement_domain"], "paxeer");
            assert_eq!(
                page["events"][usize::try_from(sequence - 1)
                    .unwrap_or_else(|_| panic!("event sequence fits usize"))]["notification"]
                    ["notification_id"],
                format!("ntf_{sequence}")
            );
        }
        legacy_end = cursor(&page, "next_cursor");
        assert_eq!(
            journal
                .next(&scope, &legacy_end)
                .or_fail("legacy read once")["events"],
            json!([])
        );
        StreamJournal::append(
            &mut scope,
            "notification:current-3",
            "notification",
            20,
            json!({"notification":{"notification_id":"ntf_3"}}),
        )
        .or_fail("append and rewrite legacy");
        for sequence in 1_u64..=3 {
            assert!(record(&scope, sequence).get("settlement_domain").is_none());
        }
    }
    drop(store);

    let mut store =
        PrincipalStore::open(&root, retention, digest).or_fail("reopen migrated journal");
    {
        let mut scope = store.principal(&principal).or_fail("migrated principal");
        assert_migrated_journal(&mut scope, &journal, &start, &legacy_end);
    }
    drop(store);
    fs::remove_dir_all(root).or_fail("remove temporary journal");
}

pub mod require_value {
    include!("support/require_value.rs");
}
use crate::require_value::RequireValue as _;
