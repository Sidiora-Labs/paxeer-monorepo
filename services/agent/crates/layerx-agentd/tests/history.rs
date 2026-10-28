use std::fs;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread;
use std::time::Duration;

use layerx_agentd::read::{history, Cursor, HistoryLimits, HistoryPage, HistoryReadError};
use layerx_client::evidence::RootSelector;
use layerx_client::head::Head;
use layerx_client::lni::framing::{read_frame, write_frame};
use layerx_client::lni::schema::{decode_envelope, encode_envelope, Envelope, Version};
use layerx_client::lni::transport::{ConnectionGate, Limits, Uds};
use layerx_client::read::{ReadContext, Requested};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

static NEXT_SOCKET: AtomicU64 = AtomicU64::new(1);

struct SocketPath(PathBuf);

impl SocketPath {
    fn new() -> Self {
        let sequence = NEXT_SOCKET.fetch_add(1, Ordering::Relaxed);
        Self(std::env::temp_dir().join(format!(
            "layerx-agentd-history-{}-{sequence}.sock",
            std::process::id()
        )))
    }
}

impl Drop for SocketPath {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.0);
    }
}

fn transport_limits() -> Limits {
    Limits {
        maximum_frame_bytes: 1024 * 1024,
        maximum_connections: 1,
        maximum_streams: 4,
        maximum_queued_bytes: 2 * 1024 * 1024,
        deadline: Duration::from_secs(2),
    }
}

pub fn history_limits(maximum_items: u16) -> HistoryLimits {
    HistoryLimits {
        maximum_items,
        maximum_response_bytes: 1_048_576,
    }
}

pub fn context() -> ReadContext {
    ReadContext {
        interface_version: Version::V1_2,
        correlation_id: 91,
        expected_protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
        expected_network_id: 77,
        requested: Requested::new(VerificationLevel::UNVERIFIED),
        head: Head {
            chain_sequence: 20,
            sealed_batch: 7,
            finalised_checkpoint: [0x71; 32],
        },
        sequencer_authorization: SequencerAuthorization::new([1; 32], [2; 32], 0, u64::MAX),
        handshake_sequencer_key: [2; 32],
        root_selector: RootSelector::Latest,
    }
}

pub fn position(block_height: u64, tx_index: u32, log_index: u32) -> Cursor {
    Cursor {
        block_height,
        tx_index,
        log_index,
    }
}

fn wire(cursor: Cursor) -> Value {
    json!([
        cursor.block_height.to_string(),
        cursor.tx_index.to_string(),
        cursor.log_index.to_string()
    ])
}

fn item(position: Cursor, kind: &str, bytes: &[u8], proof: Option<&[u8]>) -> Value {
    json!({
        "height": position.block_height.to_string(),
        "txIndex": position.tx_index.to_string(),
        "logIndex": position.log_index.to_string(),
        "kind": kind,
        "canonical": hex::encode(bytes),
        "proof": proof.map(hex::encode)
    })
}

pub fn rows() -> Vec<Value> {
    vec![
        item(
            position(10, 0, 0),
            "Activity",
            b"activity10\0\xff",
            Some(b"proof10"),
        ),
        item(
            position(10, 1, 0),
            "Receipt",
            b"receipt-10",
            Some(b"proof11"),
        ),
        item(position(11, 0, 0), "Event", b"event--011", None),
        item(
            position(12, 0, 0),
            "Activity",
            b"activity12",
            Some(b"proof12"),
        ),
        item(position(12, 0, 1), "Event", b"event--012", None),
    ]
}

pub fn result(items: Vec<Value>, cursor: Option<Cursor>) -> Value {
    json!({"items": items, "cursor": cursor.map(wire), "height": "20"})
}

pub fn page_result(cursor: Option<Cursor>, limit: u16) -> Value {
    let mut available = rows();
    if cursor.is_some() {
        available.push(item(
            position(13, 0, 0),
            "Activity",
            b"concurrent-append",
            None,
        ));
    }
    let available: Vec<_> = available
        .into_iter()
        .filter(|row| {
            let height = row["height"].as_str().unwrap().parse().unwrap();
            let position = position(
                height,
                row["txIndex"].as_str().unwrap().parse().unwrap(),
                row["logIndex"].as_str().unwrap().parse().unwrap(),
            );
            (10..=12).contains(&height) && cursor.is_none_or(|cursor| position > cursor)
        })
        .collect();
    let has_more = available.len() > usize::from(limit);
    let items: Vec<_> = available.into_iter().take(usize::from(limit)).collect();
    let next = has_more.then(|| {
        let last = items.last().unwrap();
        position(
            last["height"].as_str().unwrap().parse().unwrap(),
            last["txIndex"].as_str().unwrap().parse().unwrap(),
            last["logIndex"].as_str().unwrap().parse().unwrap(),
        )
    });
    result(items, next)
}

fn request(stream: &mut UnixStream, cursor: Option<Cursor>, limit: u16) -> u64 {
    stream
        .set_read_timeout(Some(Duration::from_secs(2)))
        .unwrap();
    stream
        .set_write_timeout(Some(Duration::from_secs(2)))
        .unwrap();
    let frame =
        read_frame(stream, 1024 * 1024).unwrap_or_else(|error| panic!("request frame: {error:?}"));
    let envelope =
        decode_envelope(&frame).unwrap_or_else(|error| panic!("request envelope: {error:?}"));
    assert_eq!(envelope.version, context().interface_version);
    assert_eq!(envelope.message_tag, 9);
    assert_eq!(envelope.correlation_id, context().correlation_id);
    assert!(envelope.proof_material.is_empty());
    let payload: Value = serde_json::from_slice(envelope.canonical_payload).unwrap();
    assert_eq!(
        payload,
        json!({
            "jsonrpc": "2.0", "id": envelope.correlation_id, "method": "px_getHistory",
            "params": [{"from_height": "10", "to_height": "12",
                "limit": limit.to_string(), "cursor": cursor.map(wire)}]
        })
    );
    envelope.correlation_id
}

fn respond(stream: &mut UnixStream, correlation: u64, result: Value) {
    let payload = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": correlation, "result": result
    }))
    .unwrap();
    let envelope = encode_envelope(Envelope {
        version: Version::V1_0,
        message_tag: 11,
        correlation_id: correlation,
        canonical_payload: &payload,
        proof_material: &[],
    })
    .unwrap_or_else(|error| panic!("response envelope: {error:?}"));
    write_frame(stream, &envelope, 1024 * 1024)
        .unwrap_or_else(|error| panic!("response frame: {error:?}"));
}

pub struct HistoryServer {
    socket: SocketPath,
    server: thread::JoinHandle<()>,
}

impl HistoryServer {
    pub fn new(pages: Vec<(Option<Cursor>, u16, Value)>) -> Self {
        let socket = SocketPath::new();
        let listener = UnixListener::bind(&socket.0).unwrap();
        let server = thread::spawn(move || {
            for (cursor, limit, page) in pages {
                let (mut stream, _) = listener.accept().unwrap();
                let correlation = request(&mut stream, cursor, limit);
                respond(&mut stream, correlation, page);
            }
        });
        Self { socket, server }
    }

    pub fn connect(&self) -> Uds {
        Uds::connect(&self.socket.0, &ConnectionGate::new(1), transport_limits())
            .unwrap_or_else(|error| panic!("connect: {error:?}"))
    }

    pub fn read(
        &self,
        cursor: Option<Cursor>,
        limits: HistoryLimits,
    ) -> Result<HistoryPage, HistoryReadError> {
        history(&mut self.connect(), 10, 12, cursor, limits, context())
    }

    pub fn page(&self, cursor: Option<Cursor>, maximum_items: u16) -> HistoryPage {
        self.read(cursor, history_limits(maximum_items))
            .unwrap_or_else(|error| panic!("history: {error:?}"))
    }

    pub fn finish(self) {
        assert!(self.server.join().is_ok(), "history node failed");
    }
}

#[test]
fn bounded_cursor_survives_restart_and_excludes_concurrent_appends() {
    let next = position(11, 0, 0);
    let server = HistoryServer::new(vec![
        (None, 5, page_result(None, 5)),
        (None, 3, page_result(None, 3)),
        (Some(next), 3, page_result(Some(next), 3)),
    ]);
    let expected = server.page(None, 5);
    assert_eq!(expected.items.len(), 5);
    assert!(expected.next_cursor.is_none());
    let first = server.page(None, 3);
    assert_eq!(first.items.len(), 3);
    assert!(first.response_bytes <= 1_048_576);
    let cursor = first
        .next_cursor
        .unwrap_or_else(|| panic!("cursor missing"));
    assert_eq!(cursor, next);
    let persisted = cursor.to_hex();
    let resumed = server.page(Some(Cursor::from_hex(&persisted).unwrap()), 3);
    assert_eq!(resumed.items.len(), 2);
    assert!(resumed.next_cursor.is_none());
    assert!(resumed
        .items
        .iter()
        .all(|item| item.position.block_height <= 12));
    let combined: Vec<_> = first.items.into_iter().chain(resumed.items).collect();
    assert_eq!(combined, expected.items);
    assert_eq!(combined[0].canonical, b"activity10\0\xff");
    for (served, original) in combined.iter().zip(&expected.items) {
        let served_hash: [u8; 32] = Sha256::digest(&served.canonical).into();
        let original_hash: [u8; 32] = Sha256::digest(&original.canonical).into();
        assert_eq!(served_hash, original_hash);
        assert_eq!(served.proof, original.proof);
    }
    server.finish();
}
