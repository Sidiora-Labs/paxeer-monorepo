#[path = "gaps.rs"]
mod gap_integration;

use gap_integration::ingest_integration;
use gap_integration::{attributes, history_limits, log, HistoryServer};
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
    let source = history.connect();
    ingestor
        .read_head(&mut heads, &source, history_limits(), 8, attributes())
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
        .read_head(&mut heads, &source, history_limits(), 8, attributes())
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
        (11, json!([log(11, 0, 0), log(11, 1, 1)])),
        (12, json!([log(12, 0, 0)])),
    ]);
    let source = history.connect();
    let heads_server = SubscriptionServer::new(
        json!(["newHeads"]),
        vec![json!({"number":"0xa"}), json!({"number":"0xd"})],
    );
    let mut heads = subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads).unwrap();
    ingestor
        .read_head(&mut heads, &source, history_limits(), 4, attributes())
        .unwrap();
    ingestor.ingest_log(&log(10, 0, 0), attributes()).unwrap();
    ingestor.ingest_log(&log(13, 0, 0), attributes()).unwrap();
    // A duplicate live log and the history result identify one event.
    ingestor.ingest_log(&log(11, 0, 0), attributes()).unwrap();
    assert!(ingestor.take_next().is_none());
    ingestor
        .read_head(&mut heads, &source, history_limits(), 4, attributes())
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
    let mut resumed = EventIngestor::open(Store::open(&root).unwrap(), tenant, 8, 0).unwrap();
    assert_eq!(resumed.watermark().last_position.unwrap().block_height, 12);
    assert_eq!(resumed.buffered_len(), 1, "open block logs survive restart");
    let history = HistoryServer::new(vec![]);
    let source = history.connect();
    let heads_server = SubscriptionServer::new(json!(["newHeads"]), vec![json!({"number":"0xe"})]);
    let mut heads = subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads).unwrap();
    resumed
        .read_head(&mut heads, &source, history_limits(), 4, attributes())
        .unwrap();
    heads_server.finish();
    history.finish();
    let recovered_pending = resumed.take_next().unwrap();
    assert_eq!(recovered_pending.position().unwrap().block_height, 13);
    assert_eq!(
        recovered_pending.canonical_bytes,
        serde_json::to_vec(&log(13, 0, 0)).unwrap()
    );
    assert!(resumed.take_next().is_none());
    std::fs::remove_dir_all(root).unwrap();
}

fn replay_logs(ingestor: &mut EventIngestor, events: Vec<Value>) {
    let count = events.len();
    let server = SubscriptionServer::new(json!(["logs", {"topics": []}]), events);
    let mut subscription = subscribe(
        &server.url,
        &[],
        SubscriptionTopic::Logs {
            address: None,
            topics: vec![],
        },
    )
    .unwrap();
    for _ in 0..count {
        ingestor.read_log(&mut subscription, attributes()).unwrap();
    }
    server.finish();
}

fn captured_checkpoint(value: &Value) -> layerx_agentd::read::ServedCheckpoint {
    use layerx_agentd::read::{
        checkpoint, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator,
        CommitVote,
    };
    fn number(value: &Value) -> u64 {
        value
            .as_u64()
            .unwrap_or_else(|| value.as_str().unwrap().parse().unwrap())
    }
    fn bytes<const N: usize>(value: &Value) -> [u8; N] {
        hex::decode(value.as_str().unwrap())
            .unwrap()
            .try_into()
            .unwrap()
    }
    fn block_id(value: &Value) -> BlockId {
        BlockId {
            hash: bytes(&value["hash"]),
            part_set_total: number(&value["parts"]["total"]).try_into().unwrap(),
            part_set_hash: bytes(&value["parts"]["hash"]),
        }
    }
    let timestamp = |text: &Value| {
        let time = &value["timestamps"][text.as_str().unwrap()];
        CommitTimestamp {
            seconds: time["seconds"].as_i64().unwrap(),
            nanos: number(&time["nanos"]).try_into().unwrap(),
        }
    };
    let h = &value["signed_header"]["header"];
    let optional = |name: &str| {
        if h[name].is_null() || h[name] == "" {
            None
        } else {
            Some(bytes(&h[name]))
        }
    };
    let header = CheckpointHeader {
        version_block: number(&h["version"]["block"]),
        version_app: number(&h["version"]["app"]),
        chain_id: h["chain_id"].as_str().unwrap().to_owned(),
        height: number(&h["height"]),
        time: timestamp(&h["time"]),
        last_block_id: Some(block_id(&h["last_block_id"])),
        last_commit_hash: optional("last_commit_hash"),
        data_hash: optional("data_hash"),
        validators_hash: bytes(&h["validators_hash"]),
        next_validators_hash: bytes(&h["next_validators_hash"]),
        consensus_hash: bytes(&h["consensus_hash"]),
        app_hash: bytes(&h["app_hash"]),
        last_results_hash: optional("last_results_hash"),
        evidence_hash: optional("evidence_hash"),
        proposer_address: bytes(&h["proposer_address"]),
    };
    let validators: Vec<_> = value["validators"]
        .as_array()
        .unwrap()
        .iter()
        .map(|v| CommitValidator {
            address: bytes(&v["address"]),
            public_key: openssl::base64::decode_block(v["pub_key"]["value"].as_str().unwrap())
                .unwrap()
                .try_into()
                .unwrap(),
            voting_power: number(&v["voting_power"]),
        })
        .collect();
    let c = &value["signed_header"]["commit"];
    let votes = c["signatures"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|v| number(&v["block_id_flag"]) == 2)
        .map(|v| {
            let address = bytes(&v["validator_address"]);
            CommitVote {
                validator_address: address,
                voting_power: validators
                    .iter()
                    .find(|validator| validator.address == address)
                    .unwrap()
                    .voting_power,
                timestamp: timestamp(&v["timestamp"]),
                signature: openssl::base64::decode_block(v["signature"].as_str().unwrap())
                    .unwrap()
                    .try_into()
                    .unwrap(),
            }
        })
        .collect();
    let commit = BlockCommit {
        height: number(&c["height"]),
        round: number(&c["round"]).try_into().unwrap(),
        block_id: block_id(&c["block_id"]),
        votes,
    };
    checkpoint(
        header.height,
        &header.chain_id,
        &header,
        &commit,
        &validators,
    )
    .unwrap()
}

#[test]
fn receipted_logs_recover_committed_without_duplicate_delivery() {
    let fixture: Value =
        serde_json::from_str(include_str!("fixtures/receipted-log-transition.json")).unwrap();
    let receipts = fixture["receipted"].as_array().unwrap();
    let committed = fixture["committed"].as_array().unwrap();
    let fixture_limits = || layerx_agentd::read::HistoryLimits {
        maximum_items: committed.len().try_into().unwrap(),
        maximum_response_bytes: history_limits().maximum_response_bytes,
    };
    let checkpoint = captured_checkpoint(&fixture["checkpoints"][0]);
    let height = checkpoint.height;
    assert_eq!(receipts.len(), 4);
    assert_eq!(committed.len(), 4);
    assert_eq!(fixture["transactions"].as_array().unwrap().len(), 2);
    for (receipt, copy) in receipts.iter().zip(committed) {
        assert!(receipt["blockHash"].is_null());
        assert!(receipt["seq"].is_string());
        assert_eq!(receipt["transactionHash"], copy["transactionHash"]);
        assert_eq!(receipt["logIndex"], copy["logIndex"]);
        assert!(copy.get("seq").is_none());
        let event =
            CoreEvent::from_log_bytes(serde_json::to_vec(receipt).unwrap(), attributes()).unwrap();
        assert_eq!(event.rung(), Some(Rung::Receipted));
        assert_eq!(event.log().unwrap().block_hash, None);
        assert!(event.log().unwrap().transaction_hash.is_some());
        assert_eq!(
            CoreEvent::from_log(copy, attributes())
                .unwrap()
                .log()
                .unwrap()
                .block_hash,
            Some(checkpoint.block_hash)
        );
    }
    assert_ne!(
        receipts[0]["transactionIndex"],
        committed[0]["transactionIndex"]
    );

    let tenant = TenantId::new("receipt-transition").unwrap();
    let root = directory();
    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    let empty = HistoryServer::new(vec![]);
    let source = empty.connect();
    ingestor
        .new_head(height - 1, &source, fixture_limits(), 8, attributes())
        .unwrap();
    replay_logs(
        &mut ingestor,
        vec![
            receipts[3].clone(),
            receipts[0].clone(),
            receipts[2].clone(),
        ],
    );
    assert_eq!(ingestor.buffered_len(), 3);
    assert!(ingestor.take_next().is_none());
    let initial = ingestor.watermark();
    assert!(
        ingestor.commit_block(&checkpoint).is_err(),
        "checkpoint alone cannot promote receipt logs"
    );
    assert_eq!(ingestor.watermark(), initial);

    let bounded_history = HistoryServer::new(vec![(height, json!(committed))]);
    assert!(matches!(
        ingestor.new_head(
            height + 1,
            &bounded_history.connect(),
            history_limits(),
            8,
            attributes()
        ),
        Err(IngestError::GapBlocked(_))
    ));
    bounded_history.finish();
    assert_eq!(ingestor.watermark(), initial);
    assert_eq!(ingestor.buffered_len(), 3);

    let uncommitted_history = HistoryServer::new(vec![(height, json!(receipts))]);
    assert!(matches!(
        ingestor.new_head(
            height + 1,
            &uncommitted_history.connect(),
            fixture_limits(),
            8,
            attributes()
        ),
        Err(IngestError::GapBlocked(_))
    ));
    uncommitted_history.finish();
    assert_eq!(ingestor.watermark(), initial);
    assert_eq!(ingestor.buffered_len(), 3);

    let mut invalid = Vec::new();
    let mut missing = receipts[0].clone();
    missing.as_object_mut().unwrap().remove("seq");
    invalid.push(missing);
    for seq in [
        json!(null),
        json!(1),
        json!("0x"),
        json!("0x00"),
        json!("0xgg"),
        json!("0x10000000000000000"),
    ] {
        let mut value = receipts[0].clone();
        value["seq"] = seq;
        invalid.push(value);
    }
    for hash in [
        json!("0x12"),
        json!("0xzz"),
        json!(format!("0x{}", "gg".repeat(32))),
        json!(false),
        json!(format!("0x{}", "ab".repeat(33))),
    ] {
        let mut value = receipts[0].clone();
        value["blockHash"] = hash;
        invalid.push(value);
    }
    for (field, bad) in [
        ("removed", json!(true)),
        ("address", json!("0x12")),
        ("topics", json!(["0x12"])),
        ("topics", json!(vec!["0x"; 5])),
        ("data", json!("0x0")),
        ("blockNumber", json!("0x01")),
        ("transactionIndex", json!("0x100000000")),
        ("logIndex", json!("0x100000000")),
        ("transactionHash", json!("0x12")),
    ] {
        let mut value = receipts[0].clone();
        value[field] = bad;
        invalid.push(value);
    }
    let count = invalid.len();
    let server = SubscriptionServer::new(json!(["logs", {"topics": []}]), invalid);
    let mut subscription = subscribe(
        &server.url,
        &[],
        SubscriptionTopic::Logs {
            address: None,
            topics: vec![],
        },
    )
    .unwrap();
    for _ in 0..count {
        assert!(matches!(
            ingestor.read_log(&mut subscription, attributes()),
            Err(IngestError::InvalidLog)
        ));
        assert_eq!(ingestor.watermark(), initial);
        assert_eq!(ingestor.buffered_len(), 3);
    }
    server.finish();
    let mut conflicting = committed[0].clone();
    conflicting["data"] = json!("0xff");
    let mut bad_history = committed.clone();
    bad_history[0] = conflicting.clone();
    let bad = HistoryServer::new(vec![(height, json!(bad_history))]);
    assert!(ingestor
        .new_head(
            height + 1,
            &bad.connect(),
            fixture_limits(),
            8,
            attributes()
        )
        .is_err());
    bad.finish();
    assert_eq!(ingestor.watermark(), initial);
    assert_eq!(ingestor.buffered_len(), 3);
    drop(ingestor);
    empty.finish();

    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    assert_eq!(ingestor.watermark(), initial);
    assert_eq!(ingestor.buffered_len(), 3);
    let history = HistoryServer::new(vec![(height, json!(committed))]);
    ingestor
        .new_head(
            height + 1,
            &history.connect(),
            fixture_limits(),
            8,
            attributes(),
        )
        .unwrap();
    history.finish();
    assert_eq!(
        ingestor.buffered_len(),
        4,
        "backfill adds only the missing log"
    );
    let mut delivered = Vec::new();
    for (sequence, expected) in committed.iter().enumerate() {
        let event = ingestor.take_next().unwrap();
        assert_eq!(event.global_sequence, sequence as u64);
        assert_eq!(
            event.rung(),
            Some(Rung::Receipted),
            "a hash does not verify commitment"
        );
        assert_eq!(event.canonical_bytes, serde_json::to_vec(expected).unwrap());
        if let Some(previous) = delivered.last() {
            assert!(event.position() > *previous);
        }
        delivered.push(event.position());
    }
    assert!(ingestor.take_next().is_none());
    let progress = ingestor.watermark();
    let mut wrong = checkpoint.clone();
    wrong.block_hash[0] ^= 1;
    assert!(ingestor.commit_block(&wrong).is_err());
    assert_eq!(ingestor.watermark(), progress);
    let mut wrong = checkpoint.clone();
    wrong.commit.votes[0].signature[0] ^= 1;
    assert!(ingestor.commit_block(&wrong).is_err());
    assert_eq!(ingestor.watermark(), progress);
    assert!(ingestor.ingest_log(&conflicting, attributes()).is_err());
    let mut conflicting_hash = committed[0].clone();
    conflicting_hash["blockHash"] = json!(format!("0x{}", "ff".repeat(32)));
    assert!(ingestor
        .ingest_log(&conflicting_hash, attributes())
        .is_err());
    let mut conflicting_identity = committed[0].clone();
    conflicting_identity["transactionHash"] = json!(format!("0x{}", "ff".repeat(32)));
    assert!(ingestor
        .ingest_log(&conflicting_identity, attributes())
        .is_err());
    assert_eq!(ingestor.watermark(), progress);
    ingestor.commit_block(&checkpoint).unwrap();
    drop(ingestor);
    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    replay_logs(&mut ingestor, committed.clone());
    replay_logs(&mut ingestor, receipts.clone());
    assert_eq!(ingestor.watermark(), progress);
    assert_eq!(ingestor.buffered_len(), 0);
    for sequence in 0..4 {
        let event = ingestor.retained_event(sequence).unwrap();
        assert_eq!(event.rung(), Some(Rung::Committed));
        assert_eq!(event.log().unwrap().block_hash, Some(checkpoint.block_hash));
    }
    assert!(ingestor.take_next().is_none());
    drop(ingestor);
    std::fs::remove_dir_all(root).unwrap();

    // Reconcile already durable receipt bytes and the undelivered buffer.
    let root = directory();
    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    let empty = HistoryServer::new(vec![]);
    let source = empty.connect();
    ingestor
        .new_head(height, &source, fixture_limits(), 8, attributes())
        .unwrap();
    replay_logs(&mut ingestor, receipts.iter().rev().cloned().collect());
    ingestor
        .new_head(height + 1, &source, fixture_limits(), 8, attributes())
        .unwrap();
    let first = ingestor.take_next().unwrap();
    assert_eq!(first.rung(), Some(Rung::Receipted));
    assert_eq!(
        first.canonical_bytes,
        serde_json::to_vec(&receipts[0]).unwrap()
    );
    assert!(ingestor.commit_block(&checkpoint).is_err());
    let progress = ingestor.watermark();
    assert!(ingestor.ingest_log(&conflicting, attributes()).is_err());
    assert_eq!(ingestor.watermark(), progress);
    replay_logs(&mut ingestor, committed.clone());
    assert_eq!(ingestor.watermark().next_expected, progress.next_expected);
    assert_eq!(
        ingestor.watermark().last_position,
        CoreEvent::from_log(&committed[3], attributes())
            .unwrap()
            .position()
    );
    ingestor.commit_block(&checkpoint).unwrap();
    for sequence in 1..4 {
        let event = ingestor.take_next().unwrap();
        assert_eq!(event.global_sequence, sequence);
        assert_eq!(event.rung(), Some(Rung::Committed));
        assert_eq!(event.log().unwrap().block_hash, Some(checkpoint.block_hash));
    }
    assert!(ingestor.take_next().is_none());
    for (sequence, receipt) in receipts.iter().enumerate() {
        let key = TenantKey::new(
            tenant.clone(),
            ObjectKind::Event,
            (sequence as u64).to_be_bytes().to_vec(),
        )
        .unwrap();
        assert_eq!(
            ingestor.store().get(&key).unwrap().bytes(),
            serde_json::to_vec(receipt).unwrap()
        );
    }
    let progress = ingestor.watermark();
    drop(ingestor);
    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    replay_logs(&mut ingestor, committed.clone());
    assert_eq!(ingestor.watermark(), progress);
    assert!(ingestor.take_next().is_none());
    for sequence in 0..4 {
        assert_eq!(
            ingestor.retained_event(sequence).unwrap().rung(),
            Some(Rung::Committed)
        );
    }
    empty.finish();
    drop(ingestor);
    std::fs::remove_dir_all(root).unwrap();

    // Pending promotion and its evidence survive reopening before delivery.
    let root = directory();
    let mut ingestor =
        EventIngestor::open(Store::open(&root).unwrap(), tenant.clone(), 8, 0).unwrap();
    replay_logs(&mut ingestor, receipts.clone());
    replay_logs(&mut ingestor, committed.clone());
    ingestor.commit_block(&checkpoint).unwrap();
    drop(ingestor);
    let mut ingestor = EventIngestor::open(Store::open(&root).unwrap(), tenant, 8, 0).unwrap();
    let empty = HistoryServer::new(vec![]);
    ingestor
        .new_head(
            height + 1,
            &empty.connect(),
            fixture_limits(),
            8,
            attributes(),
        )
        .unwrap();
    for sequence in 0..4 {
        let event = ingestor.take_next().unwrap();
        assert_eq!(event.global_sequence, sequence);
        assert_eq!(event.rung(), Some(Rung::Committed));
        assert_eq!(event.log().unwrap().block_hash, Some(checkpoint.block_hash));
    }
    assert!(ingestor.take_next().is_none());
    empty.finish();
    drop(ingestor);
    std::fs::remove_dir_all(root).unwrap();
}
