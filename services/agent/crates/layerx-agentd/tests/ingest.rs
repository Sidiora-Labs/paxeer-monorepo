use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::events::{
    ingest, CoreEvent, EventAttributes, EventIngestor, IngestError, Watermark,
};
use layerx_agentd::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};
use layerx_types::verify::VerificationLevel;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn test_directory(name: &str) -> PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-agentd-ingest-{name}-{}-{sequence}",
        std::process::id()
    ))
}

fn tenant(name: &str) -> TenantId {
    match TenantId::new(name) {
        Ok(value) => value,
        Err(error) => panic!("test tenant must be valid: {error}"),
    }
}

fn event(sequence: u64, marker: u8) -> CoreEvent {
    CoreEvent {
        global_sequence: sequence,
        canonical_bytes: vec![0x41, marker, 0x7f],
        receipt_reference: Some([marker; 32]),
        receipt_verification_level: VerificationLevel::SEQUENCER_SIGNED,
        attributes: EventAttributes {
            agent: "agent-a".to_owned(),
            account: "account-a".to_owned(),
            activity_type: 9,
            module: "asset".to_owned(),
            asset: "LXR".to_owned(),
            counterparty: "counterparty-a".to_owned(),
            result_code: 0,
        },
    }
}

fn key(tenant_id: &TenantId, kind: ObjectKind, object_id: Vec<u8>) -> TenantKey {
    match TenantKey::new(tenant_id.clone(), kind, object_id) {
        Ok(value) => value,
        Err(error) => panic!("test key must be valid: {error}"),
    }
}

#[test]
fn exact_core_events_are_durable_and_buffer_backpressure_drops_nothing() {
    let root = test_directory("ordered");
    let tenant_id = tenant("tenant-a");
    let store = match Store::open(&root) {
        Ok(value) => value,
        Err(error) => panic!("store open failed: {error}"),
    };
    let mut ingestor = match EventIngestor::open(store, tenant_id.clone(), 2, 10) {
        Ok(value) => value,
        Err(error) => panic!("ingestor open failed: {error}"),
    };

    if let Err(error) = ingest(&mut ingestor, event(10, 0x10)) {
        panic!("first ingest failed: {error}");
    }
    if let Err(error) = ingest(&mut ingestor, event(11, 0x11)) {
        panic!("second ingest failed: {error}");
    }
    assert!(matches!(
        ingest(&mut ingestor, event(12, 0x12)),
        Err(IngestError::Backpressure { capacity: 2 })
    ));
    assert_eq!(
        ingestor.watermark(),
        Watermark {
            last_ingested: Some(11),
            next_expected: 12,
            last_position: None,
        }
    );
    assert_eq!(
        ingestor
            .store()
            .list_object_ids(&tenant_id, ObjectKind::Event)
            .len(),
        2
    );

    assert_eq!(ingestor.take_next(), Some(event(10, 0x10)));
    if let Err(error) = ingest(&mut ingestor, event(12, 0x12)) {
        panic!("ingest after capacity became available failed: {error}");
    }
    assert_eq!(ingestor.take_next(), Some(event(11, 0x11)));
    assert_eq!(ingestor.take_next(), Some(event(12, 0x12)));
    assert!(ingestor.take_next().is_none());

    let event_key = key(&tenant_id, ObjectKind::Event, 10_u64.to_be_bytes().to_vec());
    let Some(stored) = ingestor.store().get(&event_key) else {
        panic!("exact event bytes were not persisted")
    };
    assert_eq!(stored.class(), StorageClass::CoreProducedCache);
    assert_eq!(stored.bytes(), event(10, 0x10).canonical_bytes);

    let mut metadata_id = b"event-evidence:".to_vec();
    metadata_id.extend_from_slice(&10_u64.to_be_bytes());
    let metadata_key = key(&tenant_id, ObjectKind::Configuration, metadata_id);
    let Some(metadata) = ingestor.store().get(&metadata_key) else {
        panic!("event receipt evidence was not persisted")
    };
    let mut expected_metadata = b"LXEM".to_vec();
    expected_metadata.extend_from_slice(&[1, 1]);
    expected_metadata.extend_from_slice(&[0x10; 32]);
    assert_eq!(metadata.class(), StorageClass::LocalOnly);
    assert_eq!(
        &metadata.bytes()[..expected_metadata.len()],
        expected_metadata
    );
    let _ = fs::remove_dir_all(root);
}

#[test]
fn restart_resumes_from_durable_watermark_and_refuses_repeat_or_gap() {
    let root = test_directory("restart");
    let tenant_id = tenant("tenant-a");
    let store = match Store::open(&root) {
        Ok(value) => value,
        Err(error) => panic!("store open failed: {error}"),
    };
    let mut ingestor = match EventIngestor::open(store, tenant_id.clone(), 4, 20) {
        Ok(value) => value,
        Err(error) => panic!("ingestor open failed: {error}"),
    };
    if let Err(error) = ingest(&mut ingestor, event(20, 0x20)) {
        panic!("initial ingest failed: {error}");
    }
    drop(ingestor);

    let restarted_store = match Store::open(&root) {
        Ok(value) => value,
        Err(error) => panic!("store reopen failed: {error}"),
    };
    let mut restarted = match EventIngestor::open(restarted_store, tenant_id.clone(), 4, 0) {
        Ok(value) => value,
        Err(error) => panic!("ingestor restart failed: {error}"),
    };
    assert_eq!(
        restarted.watermark(),
        Watermark {
            last_ingested: Some(20),
            next_expected: 21,
            last_position: None,
        }
    );
    assert!(matches!(
        ingest(&mut restarted, event(20, 0x20)),
        Err(IngestError::Repeated { sequence: 20 })
    ));
    assert!(matches!(
        ingest(&mut restarted, event(22, 0x22)),
        Err(IngestError::OutOfOrder {
            expected: 21,
            received: 22,
        })
    ));
    assert_eq!(restarted.buffered_len(), 0);
    if let Err(error) = ingest(&mut restarted, event(21, 0x21)) {
        panic!("resumed ingest failed: {error}");
    }
    assert_eq!(restarted.take_next(), Some(event(21, 0x21)));
    assert_eq!(
        restarted
            .store()
            .list_object_ids(&tenant_id, ObjectKind::Event),
        vec![20_u64.to_be_bytes().to_vec(), 21_u64.to_be_bytes().to_vec()]
    );
    let _ = fs::remove_dir_all(root);
}

#[test]
fn invalid_inputs_never_advance_or_persist_the_stream() {
    let root = test_directory("invalid");
    let tenant_id = tenant("tenant-a");
    let store = match Store::open(&root) {
        Ok(value) => value,
        Err(error) => panic!("store open failed: {error}"),
    };
    assert!(matches!(
        EventIngestor::open(store, tenant_id.clone(), 0, 7),
        Err(IngestError::InvalidCapacity)
    ));

    let store = match Store::open(&root) {
        Ok(value) => value,
        Err(error) => panic!("store reopen failed: {error}"),
    };
    let mut ingestor = match EventIngestor::open(store, tenant_id.clone(), 1, 7) {
        Ok(value) => value,
        Err(error) => panic!("ingestor open failed: {error}"),
    };
    let mut empty = event(7, 0x07);
    empty.canonical_bytes.clear();
    assert!(matches!(
        ingest(&mut ingestor, empty),
        Err(IngestError::EmptyCoreEvent)
    ));
    let mut missing_receipt = event(7, 0x07);
    missing_receipt.receipt_reference = None;
    assert!(matches!(
        ingest(&mut ingestor, missing_receipt),
        Err(IngestError::ReceiptEvidenceMismatch)
    ));
    assert_eq!(
        ingestor.watermark(),
        Watermark {
            last_ingested: None,
            next_expected: 7,
            last_position: None,
        }
    );
    assert!(ingestor
        .store()
        .list_object_ids(&tenant_id, ObjectKind::Event)
        .is_empty());
    let _ = fs::remove_dir_all(root);
}

#[test]
fn emitted_receipts_keep_exact_bytes_and_positions_through_backpressure_and_restart() {
    use serde_json::Value;

    let fixture: Value = assert_ok(
        serde_json::from_str(include_str!("fixtures/receipted-log-transition.json")),
        "unexpected error",
    );
    let receipts = assert_some(fixture["receipted"].as_array(), "expected Some value");
    let attributes = EventAttributes {
        agent: "agent-a".to_owned(),
        account: "account-a".to_owned(),
        activity_type: 9,
        module: "asset".to_owned(),
        asset: "LXR".to_owned(),
        counterparty: "counterparty-a".to_owned(),
        result_code: 0,
    };
    let root = test_directory("producer-backpressure");
    let tenant_id = tenant("producer-backpressure");
    let mut ingestor = assert_ok(
        EventIngestor::open(
            assert_ok(Store::open(&root), "unexpected error"),
            tenant_id.clone(),
            2,
            39,
        ),
        "unexpected error",
    );
    let seed_event = seed_backpressure(&mut ingestor, &root, &tenant_id);
    let initial = ingestor.watermark();
    assert_eq!(initial.next_expected, 40);
    fill_receipt_buffer(&mut ingestor, &attributes, receipts);
    assert_eq!(ingestor.buffered_len(), 2);
    assert_eq!(ingestor.watermark(), initial);
    assert!(ingestor.take_next().is_none());
    assert_eq!(
        ingestor
            .store()
            .list_object_ids(&tenant_id, ObjectKind::Event),
        vec![39_u64.to_be_bytes().to_vec()]
    );
    assert_eq!(
        assert_ok(ingestor.retained_event(39), "unexpected error"),
        seed_event
    );
    drop(ingestor);

    let mut ingestor = assert_ok(
        EventIngestor::open(
            assert_ok(Store::open(&root), "unexpected error"),
            tenant_id.clone(),
            2,
            0,
        ),
        "unexpected error",
    );
    assert_eq!(ingestor.buffered_len(), 2);
    assert_eq!(ingestor.watermark(), initial);
    assert_eq!(
        assert_ok(ingestor.retained_event(39), "unexpected error"),
        seed_event
    );
    assert!(ingestor.take_next().is_none());
    let expected: Vec<_> = receipts[..2]
        .iter()
        .map(|receipt| {
            assert_ok(
                CoreEvent::from_log(receipt, attributes.clone()),
                "unexpected error",
            )
        })
        .collect();
    let height = committed_checkpoint(
        assert_some(expected[0].position(), "expected Some value").block_height,
    )
    .height;
    close_receipt_block(&mut ingestor, height, attributes);
    assert_delivered_receipts(&mut ingestor, &tenant_id, &expected);
    assert!(ingestor.take_next().is_none());
    assert_eq!(ingestor.watermark().next_expected, 42);
    assert_eq!(ingestor.watermark().last_position, expected[1].position());
    let watermark = ingestor.watermark();
    drop(ingestor);

    assert_reopened_receipts(&root, &tenant_id, watermark, &seed_event, &expected);
    assert_ok(fs::remove_dir_all(root), "unexpected error");
}

pub struct SubscriptionServer {
    pub url: String,
    server: std::thread::JoinHandle<()>,
}

impl SubscriptionServer {
    #[must_use = "use the constructed fixture value"]
    /// # Panics
    /// Panics if the fixture data, codec, signatures, or required node exchanges fail validation.
    pub fn new(topic: serde_json::Value, events: Vec<serde_json::Value>) -> Self {
        use std::io::{Read, Write};
        let listener = assert_ok(
            std::net::TcpListener::bind("127.0.0.1:0"),
            "unexpected error",
        );
        let url = format!(
            "ws://{}/",
            assert_ok(listener.local_addr(), "unexpected error")
        );
        let server = std::thread::spawn(move || {
            let (mut socket, _) = assert_ok(listener.accept(), "unexpected error");
            assert_ok(
                socket.set_read_timeout(Some(std::time::Duration::from_secs(3))),
                "unexpected error",
            );
            let mut request = Vec::new();
            while !request.ends_with(b"\r\n\r\n") {
                let mut byte = [0];
                assert_ok(socket.read_exact(&mut byte), "unexpected error");
                request.push(byte[0]);
            }
            let request = assert_ok(String::from_utf8(request), "unexpected error");
            let key = assert_some(
                request.lines().find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("sec-websocket-key")
                        .then(|| value.trim())
                }),
                "expected Some value",
            );
            let digest =
                openssl::sha::sha1(format!("{key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11").as_bytes());
            assert_ok(write!(socket, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: {}\r\n\r\n", openssl::base64::encode_block(&digest)), "unexpected error");
            let mut header = [0; 2];
            assert_ok(socket.read_exact(&mut header), "unexpected error");
            assert_eq!(header[0], 0x81);
            assert_ne!(header[1] & 0x80, 0);
            let length = match header[1] & 0x7f {
                126 => {
                    let mut bytes = [0; 2];
                    assert_ok(socket.read_exact(&mut bytes), "unexpected error");
                    usize::from(u16::from_be_bytes(bytes))
                }
                127 => panic!("unexpected subscription frame length"),
                value => usize::from(value),
            };
            let mut mask = [0; 4];
            assert_ok(socket.read_exact(&mut mask), "unexpected error");
            let mut payload = vec![0; length];
            assert_ok(socket.read_exact(&mut payload), "unexpected error");
            for (offset, byte) in payload.iter_mut().enumerate() {
                *byte ^= mask[offset % 4];
            }
            let request: serde_json::Value =
                assert_ok(serde_json::from_slice(&payload), "unexpected error");
            assert_eq!(
                request,
                serde_json::json!({"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":topic})
            );
            let send = |socket: &mut std::net::TcpStream, value: serde_json::Value| {
                let bytes = assert_ok(serde_json::to_vec(&value), "unexpected error");
                assert_ok(socket.write_all(&[0x81]), "unexpected error");
                if bytes.len() < 126 {
                    assert_ok(
                        socket
                            .write_all(&[assert_ok(u8::try_from(bytes.len()), "unexpected error")]),
                        "unexpected error",
                    );
                } else {
                    assert_ok(socket.write_all(&[126]), "unexpected error");
                    assert_ok(
                        socket.write_all(
                            &assert_ok(u16::try_from(bytes.len()), "unexpected error")
                                .to_be_bytes(),
                        ),
                        "unexpected error",
                    );
                }
                assert_ok(socket.write_all(&bytes), "unexpected error");
            };
            send(
                &mut socket,
                serde_json::json!({"jsonrpc":"2.0","id":1,"result":"0xfeed"}),
            );
            for event in events {
                send(
                    &mut socket,
                    serde_json::json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":"0xfeed","result":event}}),
                );
            }
        });
        Self { url, server }
    }
    /// # Panics
    /// Panics if the fixture server thread panics.
    pub fn finish(self) {
        assert_ok(self.server.join(), "unexpected error");
    }
}

#[must_use = "use the constructed fixture value"]
/// # Panics
/// Panics if the signed checkpoint or its validator signatures fail verification.
pub fn committed_checkpoint(height: u64) -> layerx_agentd::read::ServedCheckpoint {
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_agentd::read::{
        checkpoint, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
        CommitValidator, CommitVote,
    };
    let key = SigningKey::from_bytes(&[7; 32]);
    let public_key = key.verifying_key().to_bytes();
    let validator = CommitValidator {
        address: CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: 100,
    };
    let timestamp = CommitTimestamp {
        seconds: 1234,
        nanos: 0,
    };
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 0,
        chain_id: "events-test".to_owned(),
        height,
        time: timestamp,
        last_block_id: None,
        last_commit_hash: None,
        data_hash: None,
        validators_hash: validator_set_hash(&[validator]),
        next_validators_hash: validator_set_hash(&[validator]),
        consensus_hash: [1; 32],
        app_hash: [2; 32],
        last_results_hash: None,
        evidence_hash: None,
        proposer_address: validator.address,
    };
    let mut commit = BlockCommit {
        height,
        round: 0,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [3; 32],
        },
        votes: vec![],
    };
    commit.votes.push(CommitVote {
        validator_address: validator.address,
        voting_power: 100,
        timestamp,
        signature: key
            .sign(&commit.vote_sign_bytes("events-test", timestamp))
            .to_bytes(),
    });
    assert_ok(
        checkpoint(height, "events-test", &header, &commit, &[validator]),
        "unexpected error",
    )
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

#[track_caller]
fn assert_some<T>(value: Option<T>, message: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{message}"),
    }
}

fn seed_backpressure(
    ingestor: &mut EventIngestor,
    root: &std::path::Path,
    tenant_id: &TenantId,
) -> CoreEvent {
    use layerx_agentd::approval::{ApprovalEventKind, ApprovalEvents, ApprovalLifecycle};
    use layerx_agentd::audit::{Coverage, Log};
    use layerx_agentd::session::SessionId;
    use layerx_types::ids::Did;
    let mut audit = assert_ok(Log::open(root.join("audit"), tenant_id), "unexpected error");
    let seed = assert_ok(
        ApprovalEvents::emit(
            ingestor,
            &mut audit,
            &mut Coverage::default(),
            &ApprovalLifecycle {
                tenant: tenant_id.clone(),
                agent: assert_ok(Did::new(b"agent-a"), "unexpected error"),
                session: SessionId([1; 32]),
                capability: [2; 32],
                policy_version: "producer-backpressure".to_owned(),
                approval_id: [3; 32],
                canonical_digest: [4; 32],
                activity_type: 9,
                asset: "LXR".to_owned(),
                kind: ApprovalEventKind::Created,
                principal: None,
                observed_at_ms: 1,
            },
        ),
        "unexpected error",
    );
    let seed_event = assert_some(ingestor.take_next(), "expected Some value");
    assert_eq!(seed_event.global_sequence, 39);
    assert_eq!(seed_event.canonical_bytes, seed.canonical_event_bytes);
    assert_eq!(
        assert_some(
            ingestor.store().get(&key(
                tenant_id,
                ObjectKind::Event,
                39_u64.to_be_bytes().to_vec()
            )),
            "expected Some value"
        )
        .class(),
        StorageClass::LocalOnly
    );
    seed_event
}

fn close_receipt_block(ingestor: &mut EventIngestor, height: u64, attributes: EventAttributes) {
    use layerx_agentd::events::gap::EthLogSource;
    use layerx_agentd::read::HistoryLimits;
    use layerx_client::subscription::{subscribe, SubscriptionTopic};
    use serde_json::json;
    let server = SubscriptionServer::new(
        json!(["newHeads"]),
        vec![json!({"number": format!("0x{:x}", height + 1)})],
    );
    let mut heads = assert_ok(
        subscribe(&server.url, &[], SubscriptionTopic::NewHeads),
        "unexpected error",
    );
    // The first head closes the pending block without requesting a history range.
    let source = assert_ok(
        EthLogSource::new(
            &server.url.replacen("ws://", "http://", 1),
            &[],
            SubscriptionTopic::Logs {
                address: None,
                topics: vec![],
            },
        ),
        "unexpected error",
    );
    assert_ok(
        ingestor.read_head(
            &mut heads,
            &source,
            HistoryLimits {
                maximum_items: 2,
                maximum_response_bytes: 1_048_576,
            },
            2,
            attributes,
        ),
        "unexpected error",
    );
    server.finish();
}

fn assert_delivered_receipts(
    ingestor: &mut EventIngestor,
    tenant_id: &TenantId,
    expected: &[CoreEvent],
) {
    use layerx_types::rung::Rung;
    for (offset, expected) in expected.iter().enumerate() {
        let mut expected = expected.clone();
        expected.global_sequence = 40 + offset as u64;
        let delivered = assert_some(ingestor.take_next(), "expected Some value");
        assert_eq!(delivered, expected);
        assert_eq!(delivered.rung(), Some(Rung::Receipted));
        let decoded = assert_ok(delivered.log(), "unexpected error");
        assert_eq!(decoded.seq, Some(0xf1));
        assert_eq!(decoded.block_hash, None);
        assert!(decoded.transaction_hash.is_some());
        let stored = assert_some(
            ingestor.store().get(&key(
                tenant_id,
                ObjectKind::Event,
                delivered.global_sequence.to_be_bytes().to_vec(),
            )),
            "expected Some value",
        );
        assert_eq!(stored.class(), StorageClass::CoreProducedCache);
        assert_eq!(stored.bytes(), delivered.canonical_bytes);
        assert_eq!(
            assert_ok(
                ingestor.retained_event(delivered.global_sequence),
                "unexpected error"
            ),
            delivered
        );
    }
}

fn assert_reopened_receipts(
    root: &std::path::Path,
    tenant_id: &TenantId,
    watermark: Watermark,
    seed_event: &CoreEvent,
    expected: &[CoreEvent],
) {
    let mut ingestor = assert_ok(
        EventIngestor::open(
            assert_ok(Store::open(root), "unexpected error"),
            tenant_id.clone(),
            2,
            0,
        ),
        "unexpected error",
    );
    assert_eq!(ingestor.watermark(), watermark);
    assert_eq!(ingestor.buffered_len(), 0);
    assert!(ingestor.take_next().is_none());
    assert_eq!(
        ingestor
            .store()
            .list_object_ids(tenant_id, ObjectKind::Event),
        vec![
            39_u64.to_be_bytes().to_vec(),
            40_u64.to_be_bytes().to_vec(),
            41_u64.to_be_bytes().to_vec()
        ]
    );
    assert_eq!(
        assert_ok(ingestor.retained_event(39), "unexpected error"),
        *seed_event
    );
    for (offset, expected) in expected.iter().enumerate() {
        let mut expected = expected.clone();
        expected.global_sequence = 40 + offset as u64;
        assert_eq!(
            assert_ok(
                ingestor.retained_event(expected.global_sequence),
                "unexpected error"
            ),
            expected
        );
    }
    drop(ingestor);
}

fn fill_receipt_buffer(
    ingestor: &mut EventIngestor,
    attributes: &EventAttributes,
    receipts: &[serde_json::Value],
) {
    use layerx_client::subscription::{subscribe, SubscriptionTopic};
    use serde_json::json;
    let server = SubscriptionServer::new(
        json!(["logs", {"topics": []}]),
        vec![
            receipts[1].clone(),
            receipts[0].clone(),
            receipts[2].clone(),
        ],
    );
    let mut logs = assert_ok(
        subscribe(
            &server.url,
            &[],
            SubscriptionTopic::Logs {
                address: None,
                topics: vec![],
            },
        ),
        "unexpected error",
    );
    assert_ok(
        ingestor.read_log(&mut logs, attributes.clone()),
        "unexpected error",
    );
    assert_ok(
        ingestor.read_log(&mut logs, attributes.clone()),
        "unexpected error",
    );
    assert!(matches!(
        ingestor.read_log(&mut logs, attributes.clone()),
        Err(IngestError::Backpressure { capacity: 2 })
    ));
    server.finish();
    drop(logs);
}
