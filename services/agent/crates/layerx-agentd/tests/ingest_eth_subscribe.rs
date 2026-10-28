#[path = "gaps.rs"]
mod gap_integration;
#[path = "ingest.rs"]
mod ingest_integration;

use gap_integration::{attributes, context, history_limits, log, page, row, HistoryServer};
use ingest_integration::SubscriptionServer;
use layerx_agentd::events::{CoreEvent, EventIngestor, IngestError};
use layerx_agentd::read::Cursor;
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::subscription::{subscribe, SubscriptionTopic};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT: AtomicU64 = AtomicU64::new(1);

fn directory() -> std::path::PathBuf {
    std::env::temp_dir().join(format!(
        "eth-ingest-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    ))
}

#[test]
fn logs_are_ingested_in_block_position_order() {
    let root = directory();
    let tenant = TenantId::new("tenant-a").unwrap();
    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    let checkpoint = ingest_integration::committed_checkpoint(11);
    let mut wire_logs = vec![log(11, 0, 0), log(10, 2, 3), log(10, 1, 2), log(10, 1, 0)];
    wire_logs[0]["blockHash"] = json!(format!("0x{}", hex::encode(checkpoint.block_hash)));
    let server = SubscriptionServer::new(json!(["logs", {"topics": []}]), wire_logs.clone());
    let mut logs = subscribe(
        &server.url,
        &[],
        SubscriptionTopic::Logs {
            address: None,
            topics: vec![],
        },
    )
    .unwrap();
    for _ in &wire_logs {
        ingestor.read_log(&mut logs, attributes()).unwrap();
    }
    server.finish();
    assert_eq!(ingestor.buffered_len(), 4);
    assert!(
        ingestor.take_next().is_none(),
        "an open block must not release unordered logs"
    );
    let heads_server = SubscriptionServer::new(
        json!(["newHeads"]),
        vec![json!({"number":"0xb"}), json!({"number":"0xc"})],
    );
    let mut heads = subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads).unwrap();
    // No history request is needed for adjacent heads.
    let history = HistoryServer::new(vec![]);
    let mut transport = history.connect();
    ingestor
        .read_head(
            &mut heads,
            &mut transport,
            history_limits(),
            context(),
            8,
            attributes(),
        )
        .unwrap();
    let expected = vec![
        Cursor {
            block_height: 10,
            tx_index: 1,
            log_index: 0,
        },
        Cursor {
            block_height: 10,
            tx_index: 1,
            log_index: 2,
        },
        Cursor {
            block_height: 10,
            tx_index: 2,
            log_index: 3,
        },
    ];
    let mut delivered = Vec::new();
    for position in &expected {
        let event = ingestor.take_next().unwrap();
        assert_eq!(event.position(), Some(*position));
        let event_log = event.log().unwrap();
        assert_eq!(event_log.rung, Rung::Receipted);
        assert_eq!(event_log.address, [0x12; 20]);
        assert_eq!(event_log.topics, vec![[0x34; 32]]);
        assert_eq!(event_log.data, vec![0, 255]);
        delivered.push(event);
    }
    assert!(
        ingestor.take_next().is_none(),
        "block 11 remains open at head 11"
    );
    ingestor.commit_block(&checkpoint).unwrap();
    ingestor
        .read_head(
            &mut heads,
            &mut transport,
            history_limits(),
            context(),
            8,
            attributes(),
        )
        .unwrap();
    heads_server.finish();
    history.finish();
    let final_event = ingestor.take_next().unwrap();
    assert_eq!(
        final_event.position(),
        Some(Cursor {
            block_height: 11,
            tx_index: 0,
            log_index: 0
        })
    );
    assert_eq!(final_event.log().unwrap().rung, Rung::Committed);
    assert!(ingestor.take_next().is_none());
    for event in delivered.iter().chain(std::iter::once(&final_event)) {
        let source: Value = serde_json::from_slice(&event.canonical_bytes).unwrap();
        assert!(wire_logs.contains(&source));
        let key = TenantKey::new(
            tenant.clone(),
            ObjectKind::Event,
            event.global_sequence.to_be_bytes().to_vec(),
        )
        .unwrap();
        assert_eq!(
            ingestor.store().get(&key).unwrap().bytes(),
            event.canonical_bytes
        );
    }
    let watermark = ingestor.watermark();
    assert_eq!(watermark.last_position, final_event.position());
    let key = TenantKey::new(
        tenant.clone(),
        ObjectKind::Configuration,
        b"event-ingestion-watermark".to_vec(),
    )
    .unwrap();
    let bytes = ingestor.store().get(&key).unwrap().bytes();
    assert_eq!(&bytes[..4], b"LXEW");
    assert_eq!(&bytes[22..30], &11_u64.to_be_bytes());
    assert_eq!(&bytes[30..38], &[0; 8]);
    drop(ingestor);
    let mut resumed = EventIngestor::open(Store::open(&root).unwrap(), tenant, 8, 0).unwrap();
    assert_eq!(resumed.watermark(), watermark);
    assert!(matches!(
        resumed.ingest_log(&log(11, 0, 0), attributes()),
        Err(IngestError::Repeated { .. })
    ));
    assert!(CoreEvent::from_log(&json!({"blockNumber":"0x01"}), attributes()).is_err());
    std::fs::remove_dir_all(root).unwrap();
}

#[test]
fn missing_block_range_is_backfilled_from_history() {
    let root = directory();
    let tenant = TenantId::new("tenant-a").unwrap();
    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    let history = HistoryServer::new(vec![
        (
            Value::Null,
            2,
            page(vec![row(11, 0, 0), row(11, 1, 1)], json!(["11", "1", "1"])),
        ),
        (
            json!(["11", "1", "1"]),
            2,
            page(vec![row(12, 0, 0)], Value::Null),
        ),
    ]);
    let mut transport = history.connect();
    let heads_server = SubscriptionServer::new(
        json!(["newHeads"]),
        vec![json!({"number":"0xa"}), json!({"number":"0xd"})],
    );
    let mut heads = subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads).unwrap();
    ingestor
        .read_head(
            &mut heads,
            &mut transport,
            history_limits(),
            context(),
            4,
            attributes(),
        )
        .unwrap();
    ingestor.ingest_log(&log(10, 0, 0), attributes()).unwrap();
    ingestor.ingest_log(&log(13, 0, 0), attributes()).unwrap();
    // A duplicate live log and the history result identify one event.
    ingestor.ingest_log(&log(11, 0, 0), attributes()).unwrap();
    assert!(ingestor.take_next().is_none());
    ingestor
        .read_head(
            &mut heads,
            &mut transport,
            history_limits(),
            context(),
            4,
            attributes(),
        )
        .unwrap();
    heads_server.finish();
    history.finish();
    let expected = [(10, 0, 0), (11, 0, 0), (11, 1, 1), (12, 0, 0)];
    for (height, tx, index) in expected {
        let event = ingestor.take_next().unwrap();
        assert_eq!(
            event.position(),
            Some(Cursor {
                block_height: height,
                tx_index: tx,
                log_index: index
            })
        );
        assert_eq!(
            event.canonical_bytes,
            serde_json::to_vec(&log(height, tx, index)).unwrap()
        );
        assert_eq!(event.log().unwrap().rung, Rung::Receipted);
    }
    assert!(
        ingestor.take_next().is_none(),
        "head 13 stays pending until its block closes"
    );
    assert_eq!(ingestor.buffered_len(), 1);
    assert_eq!(
        ingestor.watermark().last_position,
        Some(Cursor {
            block_height: 12,
            tx_index: 0,
            log_index: 0
        })
    );
    assert_eq!(
        ingestor
            .store()
            .list_object_ids(&tenant, ObjectKind::Event)
            .len(),
        4
    );
    drop(ingestor);
    let resumed = EventIngestor::open(Store::open(&root).unwrap(), tenant, 8, 0).unwrap();
    assert_eq!(resumed.watermark().last_position.unwrap().block_height, 12);
    assert_eq!(resumed.buffered_len(), 1, "open block logs survive restart");
    std::fs::remove_dir_all(root).unwrap();
}
