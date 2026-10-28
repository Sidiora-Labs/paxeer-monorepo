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

pub struct SubscriptionServer {
    pub url: String,
    server: std::thread::JoinHandle<()>,
}

impl SubscriptionServer {
    pub fn new(topic: serde_json::Value, events: Vec<serde_json::Value>) -> Self {
        use std::io::{Read, Write};
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!("ws://{}/", listener.local_addr().unwrap());
        let server = std::thread::spawn(move || {
            let (mut socket, _) = listener.accept().unwrap();
            socket
                .set_read_timeout(Some(std::time::Duration::from_secs(3)))
                .unwrap();
            let mut request = Vec::new();
            while !request.ends_with(b"\r\n\r\n") {
                let mut byte = [0];
                socket.read_exact(&mut byte).unwrap();
                request.push(byte[0]);
            }
            let request = String::from_utf8(request).unwrap();
            let key = request
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("sec-websocket-key")
                        .then(|| value.trim())
                })
                .unwrap();
            let digest =
                openssl::sha::sha1(format!("{key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11").as_bytes());
            write!(socket, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: {}\r\n\r\n", openssl::base64::encode_block(&digest)).unwrap();
            let mut header = [0; 2];
            socket.read_exact(&mut header).unwrap();
            assert_eq!(header[0], 0x81);
            assert_ne!(header[1] & 0x80, 0);
            let length = match header[1] & 0x7f {
                126 => {
                    let mut bytes = [0; 2];
                    socket.read_exact(&mut bytes).unwrap();
                    usize::from(u16::from_be_bytes(bytes))
                }
                127 => panic!("unexpected subscription frame length"),
                value => usize::from(value),
            };
            let mut mask = [0; 4];
            socket.read_exact(&mut mask).unwrap();
            let mut payload = vec![0; length];
            socket.read_exact(&mut payload).unwrap();
            for (offset, byte) in payload.iter_mut().enumerate() {
                *byte ^= mask[offset % 4];
            }
            let request: serde_json::Value = serde_json::from_slice(&payload).unwrap();
            assert_eq!(
                request,
                serde_json::json!({"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":topic})
            );
            let send = |socket: &mut std::net::TcpStream, value: serde_json::Value| {
                let bytes = serde_json::to_vec(&value).unwrap();
                socket.write_all(&[0x81]).unwrap();
                if bytes.len() < 126 {
                    socket
                        .write_all(&[u8::try_from(bytes.len()).unwrap()])
                        .unwrap();
                } else {
                    socket.write_all(&[126]).unwrap();
                    socket
                        .write_all(&u16::try_from(bytes.len()).unwrap().to_be_bytes())
                        .unwrap();
                }
                socket.write_all(&bytes).unwrap();
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
    pub fn finish(self) {
        self.server.join().unwrap();
    }
}

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
    checkpoint(height, "events-test", &header, &commit, &[validator]).unwrap()
}
