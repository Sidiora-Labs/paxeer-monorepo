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
    let tenant = lint_success(TenantId::new("tenant-a"), "required test operation");
    let mut ingestor = lint_success(
        EventIngestor::open(
            lint_success(Store::open(&root), "required test operation"),
            tenant.clone(),
            8,
            0,
        ),
        "required test operation",
    );
    let checkpoint = ingest_integration::committed_checkpoint(11);
    let mut wire_logs = vec![log(11, 0, 0), log(10, 2, 3), log(10, 1, 2), log(10, 1, 0)];
    wire_logs[0]["blockHash"] = json!(format!("0x{}", hex::encode(checkpoint.block_hash)));
    let closing_event = lint_success(
        CoreEvent::from_log(&wire_logs[0], attributes()),
        "required test operation",
    );
    assert_eq!(
        lint_success(closing_event.log(), "required test operation").block_height,
        checkpoint.height
    );
    assert_eq!(
        lint_success(closing_event.log(), "required test operation").block_hash,
        Some(checkpoint.block_hash)
    );
    assert_eq!(closing_event.rung(), Some(Rung::Receipted));
    let server = SubscriptionServer::new(json!(["logs", {"topics": []}]), wire_logs.clone());
    let mut logs = lint_success(
        subscribe(
            &server.url,
            &[],
            SubscriptionTopic::Logs {
                address: None,
                topics: vec![],
            },
        ),
        "required test operation",
    );
    for _ in &wire_logs {
        lint_success(
            ingestor.read_log(&mut logs, attributes()),
            "required test operation",
        );
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
    let mut heads = lint_success(
        subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads),
        "required test operation",
    );
    // No history request is needed for adjacent heads.
    let history = HistoryServer::new(vec![]);
    let source = history.connect();
    lint_success(
        ingestor.read_head(&mut heads, &source, history_limits(), 8, attributes()),
        "required test operation",
    );
    let (delivered, final_event) = deliver_ordered_blocks(
        &mut ingestor,
        &mut heads,
        &source,
        &checkpoint,
        heads_server,
        history,
    );
    assert_ordered_event_bytes(&ingestor, &tenant, &wire_logs, &delivered, &final_event);
    recover_ordered_stream(ingestor, &root, tenant, &final_event);
    lint_success(std::fs::remove_dir_all(root), "required test operation");
}

#[test]
fn missing_block_range_is_backfilled_from_history() {
    let root = directory();
    let tenant = lint_success(TenantId::new("tenant-a"), "required test operation");
    let mut ingestor = lint_success(
        EventIngestor::open(
            lint_success(Store::open(&root), "required test operation"),
            tenant.clone(),
            8,
            0,
        ),
        "required test operation",
    );
    let history = HistoryServer::new(vec![
        (11, json!([log(11, 0, 0), log(11, 1, 1)])),
        (12, json!([log(12, 0, 0)])),
    ]);
    let source = history.connect();
    let heads_server = SubscriptionServer::new(
        json!(["newHeads"]),
        vec![json!({"number":"0xa"}), json!({"number":"0xd"})],
    );
    let mut heads = lint_success(
        subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads),
        "required test operation",
    );
    lint_success(
        ingestor.read_head(&mut heads, &source, history_limits(), 4, attributes()),
        "required test operation",
    );
    lint_success(
        ingestor.ingest_log(&log(10, 0, 0), attributes()),
        "required test operation",
    );
    lint_success(
        ingestor.ingest_log(&log(13, 0, 0), attributes()),
        "required test operation",
    );
    // A duplicate live log and the history result identify one event.
    lint_success(
        ingestor.ingest_log(&log(11, 0, 0), attributes()),
        "required test operation",
    );
    assert!(ingestor.take_next().is_none());
    lint_success(
        ingestor.read_head(&mut heads, &source, history_limits(), 4, attributes()),
        "required test operation",
    );
    heads_server.finish();
    history.finish();
    let expected = [(10, 0, 0), (11, 0, 0), (11, 1, 1), (12, 0, 0)];
    for (height, tx, index) in expected {
        let event = lint_present(ingestor.take_next(), "required test operation");
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
            lint_success(
                serde_json::to_vec(&log(height, tx, index)),
                "required test operation"
            )
        );
        assert_eq!(
            lint_success(event.log(), "required test operation").rung,
            Rung::Receipted
        );
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
    recover_missing_range(&root, tenant);
    lint_success(std::fs::remove_dir_all(root), "required test operation");
}

fn replay_logs(ingestor: &mut EventIngestor, events: Vec<Value>) {
    let count = events.len();
    let server = SubscriptionServer::new(json!(["logs", {"topics": []}]), events);
    let mut subscription = lint_success(
        subscribe(
            &server.url,
            &[],
            SubscriptionTopic::Logs {
                address: None,
                topics: vec![],
            },
        ),
        "required test operation",
    );
    for _ in 0..count {
        lint_success(
            ingestor.read_log(&mut subscription, attributes()),
            "required test operation",
        );
    }
    server.finish();
}

fn captured_checkpoint(value: &Value) -> layerx_agentd::read::ServedCheckpoint {
    use layerx_agentd::read::{
        checkpoint, BlockCommit, CheckpointHeader, CommitTimestamp, CommitVote,
    };
    let timestamp = |text: &Value| {
        let time = &value["timestamps"][lint_present(text.as_str(), "required test operation")];
        CommitTimestamp {
            seconds: lint_present(time["seconds"].as_i64(), "required test operation"),
            nanos: lint_success(number(&time["nanos"]).try_into(), "required test operation"),
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
        chain_id: lint_present(h["chain_id"].as_str(), "required test operation").to_owned(),
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
    let validators = checkpoint_validators(value);
    let c = &value["signed_header"]["commit"];
    let votes = lint_present(c["signatures"].as_array(), "required test operation")
        .iter()
        .filter(|v| number(&v["block_id_flag"]) == 2)
        .map(|v| {
            let address = bytes(&v["validator_address"]);
            CommitVote {
                validator_address: address,
                voting_power: lint_present(
                    validators
                        .iter()
                        .find(|validator| validator.address == address),
                    "required test operation",
                )
                .voting_power,
                timestamp: timestamp(&v["timestamp"]),
                signature: lint_success(
                    lint_success(
                        openssl::base64::decode_block(lint_present(
                            v["signature"].as_str(),
                            "required test operation",
                        )),
                        "required test operation",
                    )
                    .try_into(),
                    "required test operation",
                ),
            }
        })
        .collect();
    let commit = BlockCommit {
        height: number(&c["height"]),
        round: lint_success(number(&c["round"]).try_into(), "required test operation"),
        block_id: block_id(&c["block_id"]),
        votes,
    };
    lint_success(
        checkpoint(
            header.height,
            &header.chain_id,
            &header,
            &commit,
            &validators,
        ),
        "required test operation",
    )
}

#[test]
fn receipted_logs_recover_committed_without_duplicate_delivery() {
    let fixture: Value = lint_success(
        serde_json::from_str(include_str!("fixtures/receipted-log-transition.json")),
        "required test operation",
    );
    let receipts = lint_present(fixture["receipted"].as_array(), "required test operation");
    let committed = lint_present(fixture["committed"].as_array(), "required test operation");
    let checkpoint = captured_checkpoint(&fixture["checkpoints"][0]);
    assert_receipt_fixture(&fixture, receipts, committed, &checkpoint);
    let case = ReceiptTransition {
        receipts,
        committed,
        checkpoint,
    };
    case.recover_gap();
    case.reconcile_durable();
    case.recover_pending_promotion();
}

struct ReceiptTransition<'a> {
    receipts: &'a Vec<Value>,
    committed: &'a Vec<Value>,
    checkpoint: layerx_agentd::read::ServedCheckpoint,
}

impl ReceiptTransition<'_> {
    fn limits(&self) -> layerx_agentd::read::HistoryLimits {
        layerx_agentd::read::HistoryLimits {
            maximum_items: lint_success(self.committed.len().try_into(), "required test operation"),
            maximum_response_bytes: history_limits().maximum_response_bytes,
        }
    }
    fn recover_gap(&self) {
        let receipts = self.receipts;
        let committed = self.committed;
        let checkpoint = &self.checkpoint;
        let height = checkpoint.height;
        let tenant = lint_success(
            TenantId::new("receipt-transition"),
            "required test operation",
        );
        let root = directory();
        let mut ingestor = lint_success(
            EventIngestor::open(
                lint_success(Store::open(&root), "required test operation"),
                tenant.clone(),
                8,
                0,
            ),
            "required test operation",
        );
        let empty = HistoryServer::new(vec![]);
        let source = empty.connect();
        lint_success(
            ingestor.new_head(height - 1, &source, self.limits(), 8, attributes()),
            "required test operation",
        );
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
            ingestor.commit_block(checkpoint).is_err(),
            "checkpoint alone cannot promote receipt logs"
        );
        assert_eq!(ingestor.watermark(), initial);

        self.reject_gap_history(&mut ingestor, initial);

        self.reject_invalid_logs(&mut ingestor, initial);
        let mut conflicting = committed[0].clone();
        conflicting["data"] = json!("0xff");
        let mut bad_history = committed.clone();
        bad_history[0] = conflicting.clone();
        let bad = HistoryServer::new(vec![(height, json!(bad_history))]);
        assert!(ingestor
            .new_head(height + 1, &bad.connect(), self.limits(), 8, attributes())
            .is_err());
        bad.finish();
        assert_eq!(ingestor.watermark(), initial);
        assert_eq!(ingestor.buffered_len(), 3);
        drop(ingestor);
        empty.finish();

        self.complete_gap(&root, &tenant, initial, &conflicting);
        lint_success(std::fs::remove_dir_all(root), "required test operation");
    }
    fn reconcile_durable(&self) {
        let receipts = self.receipts;
        let committed = self.committed;
        let checkpoint = &self.checkpoint;
        let height = checkpoint.height;
        let tenant = lint_success(
            TenantId::new("receipt-transition"),
            "required test operation",
        );
        let mut conflicting = self.committed[0].clone();
        conflicting["data"] = json!("0xff");
        // Reconcile already durable receipt bytes and the undelivered buffer.
        let root = directory();
        let mut ingestor = lint_success(
            EventIngestor::open(
                lint_success(Store::open(&root), "required test operation"),
                tenant.clone(),
                8,
                0,
            ),
            "required test operation",
        );
        let empty = HistoryServer::new(vec![]);
        let source = empty.connect();
        lint_success(
            ingestor.new_head(height, &source, self.limits(), 8, attributes()),
            "required test operation",
        );
        replay_logs(&mut ingestor, receipts.iter().rev().cloned().collect());
        lint_success(
            ingestor.new_head(height + 1, &source, self.limits(), 8, attributes()),
            "required test operation",
        );
        let first = lint_present(ingestor.take_next(), "required test operation");
        assert_eq!(first.rung(), Some(Rung::Receipted));
        assert_eq!(
            first.canonical_bytes,
            lint_success(serde_json::to_vec(&receipts[0]), "required test operation")
        );
        assert!(ingestor.commit_block(checkpoint).is_err());
        let progress = ingestor.watermark();
        assert!(ingestor.ingest_log(&conflicting, attributes()).is_err());
        assert_eq!(ingestor.watermark(), progress);
        replay_logs(&mut ingestor, committed.clone());
        assert_eq!(ingestor.watermark().next_expected, progress.next_expected);
        assert_eq!(
            ingestor.watermark().last_position,
            lint_success(
                CoreEvent::from_log(&committed[3], attributes()),
                "required test operation"
            )
            .position()
        );
        lint_success(ingestor.commit_block(checkpoint), "required test operation");
        for sequence in 1..4 {
            let event = lint_present(ingestor.take_next(), "required test operation");
            assert_eq!(event.global_sequence, sequence);
            assert_eq!(event.rung(), Some(Rung::Committed));
            assert_eq!(
                lint_success(event.log(), "required test operation").block_hash,
                Some(checkpoint.block_hash)
            );
        }
        assert!(ingestor.take_next().is_none());
        self.assert_preserved_receipt_bytes(&ingestor, &tenant);
        let progress = ingestor.watermark();
        drop(ingestor);
        let ingestor = self.reopen_durable(&root, &tenant, progress);
        empty.finish();
        drop(ingestor);
        lint_success(std::fs::remove_dir_all(root), "required test operation");
    }
    fn reject_gap_history(
        &self,
        ingestor: &mut EventIngestor,
        initial: layerx_agentd::events::Watermark,
    ) {
        let committed = self.committed;
        let receipts = self.receipts;
        let height = self.checkpoint.height;
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
                self.limits(),
                8,
                attributes()
            ),
            Err(IngestError::GapBlocked(_))
        ));
        uncommitted_history.finish();
        assert_eq!(ingestor.watermark(), initial);
        assert_eq!(ingestor.buffered_len(), 3);
    }
    fn reject_invalid_logs(
        &self,
        ingestor: &mut EventIngestor,
        initial: layerx_agentd::events::Watermark,
    ) {
        let receipts = self.receipts;
        let mut invalid = Vec::new();
        let mut missing = receipts[0].clone();
        lint_present(missing.as_object_mut(), "required test operation").remove("seq");
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
        let mut subscription = lint_success(
            subscribe(
                &server.url,
                &[],
                SubscriptionTopic::Logs {
                    address: None,
                    topics: vec![],
                },
            ),
            "required test operation",
        );
        for _ in 0..count {
            assert!(matches!(
                ingestor.read_log(&mut subscription, attributes()),
                Err(IngestError::InvalidLog)
            ));
            assert_eq!(ingestor.watermark(), initial);
            assert_eq!(ingestor.buffered_len(), 3);
        }
        server.finish();
    }
    fn reject_conflicting_commitment(
        &self,
        ingestor: &mut EventIngestor,
        conflicting: &Value,
        progress: layerx_agentd::events::Watermark,
    ) {
        let checkpoint = &self.checkpoint;
        let committed = self.committed;
        let mut wrong = checkpoint.clone();
        wrong.block_hash[0] ^= 1;
        assert!(ingestor.commit_block(&wrong).is_err());
        assert_eq!(ingestor.watermark(), progress);
        let mut wrong = checkpoint.clone();
        wrong.commit.votes[0].signature[0] ^= 1;
        assert!(ingestor.commit_block(&wrong).is_err());
        assert_eq!(ingestor.watermark(), progress);
        assert!(ingestor.ingest_log(conflicting, attributes()).is_err());
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
    }
    fn complete_gap(
        &self,
        root: &std::path::Path,
        tenant: &TenantId,
        initial: layerx_agentd::events::Watermark,
        conflicting: &Value,
    ) {
        let committed = self.committed;
        let height = self.checkpoint.height;
        let mut ingestor = lint_success(
            EventIngestor::open(
                lint_success(Store::open(root), "required test operation"),
                tenant.clone(),
                8,
                0,
            ),
            "required test operation",
        );
        assert_eq!(ingestor.watermark(), initial);
        assert_eq!(ingestor.buffered_len(), 3);
        let history = HistoryServer::new(vec![(height, json!(committed))]);
        lint_success(
            ingestor.new_head(
                height + 1,
                &history.connect(),
                self.limits(),
                8,
                attributes(),
            ),
            "required test operation",
        );
        history.finish();
        assert_eq!(
            ingestor.buffered_len(),
            4,
            "backfill adds only the missing log"
        );
        let mut delivered = Vec::new();
        for (sequence, expected) in committed.iter().enumerate() {
            let event = lint_present(ingestor.take_next(), "required test operation");
            assert_eq!(event.global_sequence, sequence as u64);
            assert_eq!(
                event.rung(),
                Some(Rung::Receipted),
                "a hash does not verify commitment"
            );
            assert_eq!(
                event.canonical_bytes,
                lint_success(serde_json::to_vec(expected), "required test operation")
            );
            if let Some(previous) = delivered.last() {
                assert!(event.position() > *previous);
            }
            delivered.push(event.position());
        }
        assert!(ingestor.take_next().is_none());
        let progress = ingestor.watermark();
        self.reject_conflicting_commitment(&mut ingestor, conflicting, progress);
        lint_success(
            ingestor.commit_block(&self.checkpoint),
            "required test operation",
        );
        drop(ingestor);
        self.assert_replayed_commitment(root, tenant, progress);
    }
    fn assert_replayed_commitment(
        &self,
        root: &std::path::Path,
        tenant: &TenantId,
        progress: layerx_agentd::events::Watermark,
    ) {
        let committed = self.committed;
        let receipts = self.receipts;
        let checkpoint = &self.checkpoint;
        let mut ingestor = lint_success(
            EventIngestor::open(
                lint_success(Store::open(root), "required test operation"),
                tenant.clone(),
                8,
                0,
            ),
            "required test operation",
        );
        replay_logs(&mut ingestor, committed.clone());
        replay_logs(&mut ingestor, receipts.clone());
        assert_eq!(ingestor.watermark(), progress);
        assert_eq!(ingestor.buffered_len(), 0);
        for sequence in 0..4 {
            let event = lint_success(ingestor.retained_event(sequence), "required test operation");
            assert_eq!(event.rung(), Some(Rung::Committed));
            assert_eq!(
                lint_success(event.log(), "required test operation").block_hash,
                Some(checkpoint.block_hash)
            );
        }
        assert!(ingestor.take_next().is_none());
        drop(ingestor);
    }
    fn assert_preserved_receipt_bytes(&self, ingestor: &EventIngestor, tenant: &TenantId) {
        let receipts = self.receipts;
        for (sequence, receipt) in receipts.iter().enumerate() {
            let key = lint_success(
                TenantKey::new(
                    tenant.clone(),
                    ObjectKind::Event,
                    (sequence as u64).to_be_bytes().to_vec(),
                ),
                "required test operation",
            );
            assert_eq!(
                lint_present(ingestor.store().get(&key), "required test operation").bytes(),
                lint_success(serde_json::to_vec(receipt), "required test operation")
            );
        }
    }
    fn reopen_durable(
        &self,
        root: &std::path::Path,
        tenant: &TenantId,
        progress: layerx_agentd::events::Watermark,
    ) -> EventIngestor {
        let committed = self.committed;
        let mut ingestor = lint_success(
            EventIngestor::open(
                lint_success(Store::open(root), "required test operation"),
                tenant.clone(),
                8,
                0,
            ),
            "required test operation",
        );
        replay_logs(&mut ingestor, committed.clone());
        assert_eq!(ingestor.watermark(), progress);
        assert!(ingestor.take_next().is_none());
        for sequence in 0..4 {
            assert_eq!(
                lint_success(ingestor.retained_event(sequence), "required test operation").rung(),
                Some(Rung::Committed)
            );
        }
        ingestor
    }
    fn recover_pending_promotion(&self) {
        let receipts = self.receipts;
        let committed = self.committed;
        let checkpoint = &self.checkpoint;
        let height = checkpoint.height;
        let tenant = lint_success(
            TenantId::new("receipt-transition"),
            "required test operation",
        );
        // Pending promotion and its evidence survive reopening before delivery.
        let root = directory();
        let mut ingestor = lint_success(
            EventIngestor::open(
                lint_success(Store::open(&root), "required test operation"),
                tenant.clone(),
                8,
                0,
            ),
            "required test operation",
        );
        replay_logs(&mut ingestor, receipts.clone());
        replay_logs(&mut ingestor, committed.clone());
        lint_success(ingestor.commit_block(checkpoint), "required test operation");
        drop(ingestor);
        let mut ingestor = lint_success(
            EventIngestor::open(
                lint_success(Store::open(&root), "required test operation"),
                tenant,
                8,
                0,
            ),
            "required test operation",
        );
        let empty = HistoryServer::new(vec![]);
        lint_success(
            ingestor.new_head(height + 1, &empty.connect(), self.limits(), 8, attributes()),
            "required test operation",
        );
        for sequence in 0..4 {
            let event = lint_present(ingestor.take_next(), "required test operation");
            assert_eq!(event.global_sequence, sequence);
            assert_eq!(event.rung(), Some(Rung::Committed));
            assert_eq!(
                lint_success(event.log(), "required test operation").block_hash,
                Some(checkpoint.block_hash)
            );
        }
        assert!(ingestor.take_next().is_none());
        empty.finish();
        drop(ingestor);
        lint_success(std::fs::remove_dir_all(root), "required test operation");
    }
}

fn deliver_ordered_blocks(
    ingestor: &mut EventIngestor,
    heads: &mut layerx_client::subscription::NodeSubscription,
    source: &layerx_agentd::events::gap::EthLogSource,
    checkpoint: &layerx_agentd::read::ServedCheckpoint,
    heads_server: SubscriptionServer,
    history: HistoryServer,
) -> (Vec<CoreEvent>, CoreEvent) {
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
        let event = lint_present(ingestor.take_next(), "required test operation");
        assert_eq!(event.position(), Some(*position));
        let event_log = lint_success(event.log(), "required test operation");
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
    lint_success(ingestor.commit_block(checkpoint), "required test operation");
    lint_success(
        ingestor.read_head(heads, source, history_limits(), 8, attributes()),
        "required test operation",
    );
    heads_server.finish();
    history.finish();
    let final_event = lint_present(ingestor.take_next(), "required test operation");
    assert_eq!(
        final_event.position(),
        Some(Cursor {
            block_height: 11,
            tx_index: 0,
            log_index: 0
        })
    );
    assert_eq!(
        lint_success(final_event.log(), "required test operation").rung,
        Rung::Committed
    );
    assert!(ingestor.take_next().is_none());
    (delivered, final_event)
}

fn assert_ordered_event_bytes(
    ingestor: &EventIngestor,
    tenant: &TenantId,
    wire_logs: &[Value],
    delivered: &[CoreEvent],
    final_event: &CoreEvent,
) {
    for event in delivered.iter().chain(std::iter::once(final_event)) {
        let source: Value = lint_success(
            serde_json::from_slice(&event.canonical_bytes),
            "required test operation",
        );
        assert!(wire_logs.contains(&source));
        let key = lint_success(
            TenantKey::new(
                tenant.clone(),
                ObjectKind::Event,
                event.global_sequence.to_be_bytes().to_vec(),
            ),
            "required test operation",
        );
        assert_eq!(
            lint_present(ingestor.store().get(&key), "required test operation").bytes(),
            event.canonical_bytes
        );
    }
}

fn recover_missing_range(root: &std::path::Path, tenant: TenantId) {
    let mut resumed = lint_success(
        EventIngestor::open(
            lint_success(Store::open(root), "required test operation"),
            tenant,
            8,
            0,
        ),
        "required test operation",
    );
    assert_eq!(
        lint_present(resumed.watermark().last_position, "required test operation").block_height,
        12
    );
    assert_eq!(resumed.buffered_len(), 1, "open block logs survive restart");
    let history = HistoryServer::new(vec![]);
    let source = history.connect();
    let heads_server = SubscriptionServer::new(json!(["newHeads"]), vec![json!({"number":"0xe"})]);
    let mut heads = lint_success(
        subscribe(&heads_server.url, &[], SubscriptionTopic::NewHeads),
        "required test operation",
    );
    lint_success(
        resumed.read_head(&mut heads, &source, history_limits(), 4, attributes()),
        "required test operation",
    );
    heads_server.finish();
    history.finish();
    let recovered_pending = lint_present(resumed.take_next(), "required test operation");
    assert_eq!(
        lint_present(recovered_pending.position(), "required test operation").block_height,
        13
    );
    assert_eq!(
        recovered_pending.canonical_bytes,
        lint_success(
            serde_json::to_vec(&log(13, 0, 0)),
            "required test operation"
        )
    );
    assert!(resumed.take_next().is_none());
}

fn number(value: &Value) -> u64 {
    value.as_u64().unwrap_or_else(|| {
        lint_success(
            lint_present(value.as_str(), "required test operation").parse(),
            "required test operation",
        )
    })
}
fn bytes<const N: usize>(value: &Value) -> [u8; N] {
    lint_success(
        lint_success(
            hex::decode(lint_present(value.as_str(), "required test operation")),
            "required test operation",
        )
        .try_into(),
        "required test operation",
    )
}
fn block_id(value: &Value) -> layerx_agentd::read::BlockId {
    layerx_agentd::read::BlockId {
        hash: bytes(&value["hash"]),
        part_set_total: lint_success(
            number(&value["parts"]["total"]).try_into(),
            "required test operation",
        ),
        part_set_hash: bytes(&value["parts"]["hash"]),
    }
}

fn assert_receipt_fixture(
    fixture: &Value,
    receipts: &[Value],
    committed: &[Value],
    checkpoint: &layerx_agentd::read::ServedCheckpoint,
) {
    assert_eq!(receipts.len(), 4);
    assert_eq!(committed.len(), 4);
    assert_eq!(
        lint_present(
            fixture["transactions"].as_array(),
            "required test operation"
        )
        .len(),
        2
    );
    for (receipt, copy) in receipts.iter().zip(committed) {
        assert!(receipt["blockHash"].is_null());
        assert!(receipt["seq"].is_string());
        assert_eq!(receipt["transactionHash"], copy["transactionHash"]);
        assert_eq!(receipt["logIndex"], copy["logIndex"]);
        assert!(copy.get("seq").is_none());
        let event = lint_success(
            CoreEvent::from_log_bytes(
                lint_success(serde_json::to_vec(receipt), "required test operation"),
                attributes(),
            ),
            "required test operation",
        );
        assert_eq!(
            event,
            lint_success(
                CoreEvent::from_log(receipt, attributes()),
                "required test operation"
            )
        );
        assert_eq!(
            event.canonical_bytes,
            lint_success(serde_json::to_vec(receipt), "required test operation")
        );
        assert_eq!(event.rung(), Some(Rung::Receipted));
        assert_eq!(
            lint_success(event.log(), "required test operation").block_hash,
            None
        );
        assert!(lint_success(event.log(), "required test operation")
            .transaction_hash
            .is_some());
        assert_eq!(
            lint_success(
                lint_success(
                    CoreEvent::from_log(copy, attributes()),
                    "required test operation"
                )
                .log(),
                "required test operation"
            )
            .block_hash,
            Some(checkpoint.block_hash)
        );
    }
    assert_ne!(
        receipts[0]["transactionIndex"],
        committed[0]["transactionIndex"]
    );
}

fn recover_ordered_stream(
    ingestor: EventIngestor,
    root: &std::path::Path,
    tenant: TenantId,
    final_event: &CoreEvent,
) {
    let watermark = ingestor.watermark();
    assert_eq!(watermark.last_position, final_event.position());
    let key = lint_success(
        TenantKey::new(
            tenant.clone(),
            ObjectKind::Configuration,
            b"event-ingestion-watermark".to_vec(),
        ),
        "required test operation",
    );
    let bytes = lint_present(ingestor.store().get(&key), "required test operation").bytes();
    assert_eq!(&bytes[..4], b"LXEW");
    assert_eq!(&bytes[22..30], &11_u64.to_be_bytes());
    assert_eq!(&bytes[30..38], &[0; 8]);
    drop(ingestor);
    let mut resumed = lint_success(
        EventIngestor::open(
            lint_success(Store::open(root), "required test operation"),
            tenant,
            8,
            0,
        ),
        "required test operation",
    );
    assert_eq!(resumed.watermark(), watermark);
    assert!(matches!(
        resumed.ingest_log(&log(11, 0, 0), attributes()),
        Err(IngestError::Repeated { .. })
    ));
    assert!(CoreEvent::from_log(&json!({"blockNumber":"0x01"}), attributes()).is_err());
}

fn checkpoint_validators(value: &Value) -> Vec<layerx_agentd::read::CommitValidator> {
    use layerx_agentd::read::CommitValidator;

    lint_present(value["validators"].as_array(), "required test operation")
        .iter()
        .map(|v| CommitValidator {
            address: bytes(&v["address"]),
            public_key: lint_success(
                lint_success(
                    openssl::base64::decode_block(lint_present(
                        v["pub_key"]["value"].as_str(),
                        "required test operation",
                    )),
                    "required test operation",
                )
                .try_into(),
                "required test operation",
            ),
            voting_power: number(&v["voting_power"]),
        })
        .collect()
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}
