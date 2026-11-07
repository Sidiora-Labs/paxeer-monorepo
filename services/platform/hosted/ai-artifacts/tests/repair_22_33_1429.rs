//! Repair record 22.33.1429: artifact-terminal-fixture-shape is proved by
//! committed_terminal_request_survives_restart_before_retention_consumption and
//! terminal_route_refuses_incomplete_uncommitted_and_mismatched_heads.
//! Both use the committed ai-market producer fixture and the Cargo-built service
//! on loopback, its durable store and retention ledger; no external tool is run.
//! No out-of-path wiring is required. Baseline verification was not run: the
//! ordered gate contract requires full implementation before building/verifying.
//! hf-done records the verified candidate revision, command, exit and private log.
#[path = "../src/retention.rs"]
#[allow(dead_code)]
mod retention;
#[path = "../src/store.rs"]
#[allow(dead_code)]
mod store;
#[path = "support/terminal_head.rs"]
mod terminal_head;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::evidence::*;
use layerx_programs_ai_market::types::{PrincipalId, PublicKey32, Signature64, Version};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpStream;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const OWNER: &str = "terminal-owner-token";
const READER: &str = "terminal-reader-token";
const OBSERVER: &str = "terminal-observer-token";
const PUBLISHER: [u8; 32] = [0x55; 32];
const PLAIN: &[u8] = b"answer: the capital of France is Berlin";

fn id(n: u8) -> String {
    hex::encode([n; 32])
}
fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

struct Fixture(PathBuf);
impl Fixture {
    fn new() -> Self {
        let nonce = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
            .join(format!("terminal-1429-{}-{nonce}", std::process::id()));
        fs::create_dir_all(&root).unwrap();
        let fixture = Self(root.canonicalize().unwrap());
        fs::write(fixture.0.join("kek.bin"), [0x81; 32]).unwrap();
        let token_hash = |token: &str| hex::encode(Sha256::digest(token.as_bytes()));
        let config = json!({
            "capacity_bytes": 1u64 << 20, "listener": "plaintext",
            "revocation_mark": fixture.0.join("revocation.mark"),
            "retention_mark": fixture.0.join("retention.mark"),
            "commit_readers": ["observer"],
            "tenants": [
                {"id": "owner", "token_sha256": token_hash(OWNER), "quota_bytes": 1u64 << 20},
                {"id": "reader", "token_sha256": token_hash(READER), "quota_bytes": 0},
                {"id": "observer", "token_sha256": token_hash(OBSERVER), "quota_bytes": 0}
            ],
            "publishers": [{"tenant": "owner", "principal": hex::encode(PUBLISHER),
                "generation": 1, "key": hex::encode(SigningKey::from_bytes(&[1; 32]).verifying_key().to_bytes())}]
        });
        fs::write(fixture.0.join("config.json"), config.to_string()).unwrap();
        fixture
    }
    fn start(&self) -> Service {
        let log = fs::File::create(self.0.join("service.log")).unwrap();
        let child = Command::new(env!("CARGO_BIN_EXE_layerx-ai-artifacts"))
            .arg("--data-dir")
            .arg(self.0.join("store"))
            .arg("--config")
            .arg(self.0.join("config.json"))
            .arg("--key-file")
            .arg(self.0.join("kek.bin"))
            .arg("--listen")
            .arg("127.0.0.1:0")
            .stdout(Stdio::piped())
            .stderr(Stdio::from(log))
            .spawn()
            .unwrap();
        let mut svc = Service {
            child,
            addr: String::new(),
        };
        let stdout = svc.child.stdout.take().unwrap();
        let (send, recv) = std::sync::mpsc::channel();
        std::thread::spawn(move || {
            let mut line = String::new();
            BufReader::new(stdout).read_line(&mut line).unwrap();
            let _ = send.send(line);
        });
        let line = recv
            .recv_timeout(Duration::from_secs(10))
            .expect("service startup deadline");
        svc.addr = line
            .trim()
            .strip_prefix("listening ")
            .unwrap_or_else(|| {
                panic!(
                    "service refused: {}",
                    fs::read_to_string(self.0.join("service.log")).unwrap()
                )
            })
            .to_string();
        svc
    }
    fn store(&self) -> store::Store {
        store::Store::open(&self.0.join("store"), &self.0.join("revocation.mark")).unwrap()
    }
    fn ledger(&self) -> retention::Ledger {
        retention::Ledger::open(&self.0.join("store"), &self.0.join("retention.mark")).unwrap()
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}
struct Service {
    child: Child,
    addr: String,
}
impl Drop for Service {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}
fn http(svc: &Service, method: &str, path: &str, token: &str, body: &[u8]) -> (u16, Vec<u8>) {
    let mut stream = TcpStream::connect(&svc.addr).unwrap();
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
    stream.write_all(body).unwrap();
    let mut response = Vec::new();
    stream.read_to_end(&mut response).unwrap();
    let split = response.windows(4).position(|w| w == b"\r\n\r\n").unwrap();
    let status = std::str::from_utf8(&response[..split])
        .unwrap()
        .split_whitespace()
        .nth(1)
        .unwrap()
        .parse()
        .unwrap();
    (status, response[split + 4..].to_vec())
}
fn post(svc: &Service, path: &str, token: &str, body: &Value) -> (u16, Value) {
    let (status, bytes) = http(svc, "POST", path, token, body.to_string().as_bytes());
    (status, serde_json::from_slice(&bytes).unwrap())
}
fn publish(svc: &Service) -> (String, String) {
    let (status, prepared) = post(
        svc,
        "/v1/prepare",
        OWNER,
        &json!({
            "request_id": id(1), "kind": ArtifactKind::Result as u8,
            "privacy": Privacy::Public as u8, "context": hex::encode(terminal_head::ctx().bytes()),
            "byte_length": PLAIN.len()
        }),
    );
    assert_eq!(status, 200, "{prepared}");
    let session = prepared["session"].as_str().unwrap();
    assert_eq!(
        http(svc, "PUT", &format!("/v1/upload/{session}/0"), OWNER, PLAIN).0,
        200
    );
    let manifest = ArtifactManifest {
        kind: ArtifactKind::Result,
        privacy: Privacy::Public,
        context: terminal_head::ctx(),
        epoch: 9,
        publisher: PrincipalId::new(PUBLISHER).unwrap(),
        subject: terminal_head::TASK,
        byte_length: PLAIN.len() as u64,
        chunk_count: 1,
        content_root: object_content_root(PLAIN, &mut [[0; 32]; 1]).unwrap(),
        parents: Items::Typed(&[]),
        declaration_root: [0; 32],
        reproduction_root: [0; 32],
        access_policy_root: [0; 32],
        not_after_height: 0,
    };
    let mut bytes = vec![0; MAX_MANIFEST_BYTES];
    let n = encode_manifest(&manifest, &mut bytes).unwrap();
    bytes.truncate(n);
    let key = SigningKey::from_bytes(&[1; 32]);
    let generation = Version::new(1).unwrap();
    let digest = publisher_digest(
        &terminal_head::ctx(),
        manifest_root(&bytes).unwrap(),
        generation,
    );
    let envelope = PublisherEnvelope {
        manifest: &bytes,
        generation,
        key: PublicKey32(key.verifying_key().to_bytes()),
        signature: Signature64(key.sign(&digest).to_bytes()),
    };
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let n = encode_envelope(&envelope, &mut encoded).unwrap();
    let (status, receipt) = post(
        svc,
        "/v1/finalize",
        OWNER,
        &json!({
            "request_id": id(2), "session": session, "envelope": hex::encode(&encoded[..n]),
        }),
    );
    assert_eq!(status, 200, "{receipt}");
    assert_eq!(receipt["signature_status"], "VALID");
    assert_eq!(receipt["protocol_receipt"], false);
    (
        receipt["root"].as_str().unwrap().to_string(),
        session.to_string(),
    )
}
fn tombstone(svc: &Service, root: &str, n: u8, reason: u8) -> (u16, Value) {
    post(
        svc,
        "/v1/tombstone",
        OWNER,
        &json!({"request_id": id(n), "root": root, "reason": reason}),
    )
}
fn assert_error(actual: (u16, Value), status: u16, error: &str) {
    assert_eq!(actual.0, status, "{}", actual.1);
    assert_eq!(actual.1["error"], error);
}

#[test]
fn committed_terminal_request_survives_restart_before_retention_consumption() {
    let fixture = Fixture::new();
    let svc = fixture.start();
    let (root, session) = publish(&svc);
    let access = json!({"root": root, "purpose": "evaluation"});
    let (status, resolved) = post(&svc, "/v1/resolve", READER, &access);
    assert_eq!(status, 200, "{resolved}");
    assert_eq!(resolved["integrity"], "ROOT_AND_SIGNATURE_VERIFIED");
    assert_eq!(resolved["attribution"]["signature_status"], "VALID");
    assert_eq!(resolved["quality"], "UNESTABLISHED");
    assert_eq!(resolved["reward_admission"], false);
    let bytes = hex::decode(resolved["envelope"].as_str().unwrap()).unwrap();
    let envelope = decode_envelope(&bytes).unwrap();
    assert_eq!(
        hex::encode(verify_publisher(&envelope).unwrap().root.bytes()),
        root
    );
    let manifest = decode_manifest(envelope.manifest).unwrap();
    let (status, content) = http(
        &svc,
        "POST",
        "/v1/content",
        READER,
        json!({"root": root, "purpose": "evaluation", "index": 0})
            .to_string()
            .as_bytes(),
    );
    assert_eq!(status, 200);
    let mut scratch = [[0; 32]; 1];
    let mut assembler = ContentAssembler::new(&manifest, &mut scratch).unwrap();
    assembler.deliver(0, &content).unwrap();
    assembler.finish().unwrap();
    assert_eq!(content, PLAIN);

    let (request, head) = terminal_head::observation(&id(3), &root, 4);
    let before = now();
    let (status, terminal) = post(&svc, "/v1/retention/terminal", OBSERVER, &request);
    assert_eq!(status, 200, "{terminal}");
    let floor = terminal["retain_until"].as_u64().unwrap();
    assert!((before + store::RETENTION_SECS..=now() + store::RETENTION_SECS).contains(&floor));
    drop(svc);
    {
        let ledger = fixture.ledger();
        let recovered = ledger.entry(&root).unwrap().terminal.as_ref().unwrap();
        assert_eq!(recovered.retain_until, floor);
        assert_eq!(recovered.height, head.block_height);
        assert_eq!(recovered.commit, hex::encode(head.block_hash.bytes()));
        assert_eq!(
            recovered.snapshot,
            hex::encode(head.snapshot_id().unwrap().bytes())
        );
    }
    let svc = fixture.start();
    assert_eq!(
        post(&svc, "/v1/retention/terminal", OBSERVER, &request),
        (200, terminal.clone())
    );
    let (status, refusal) = tombstone(&svc, &root, 4, retention::OWNER_WITHDRAWN);
    assert_eq!(status, 409, "{refusal}");
    assert_eq!(refusal["error"], "RETENTION_ACTIVE");
    assert_eq!(refusal["until"], floor);
    let (status, tombstoned) = tombstone(&svc, &root, 5, retention::POLICY_RESTRICTED);
    assert_eq!(status, 200, "{tombstoned}");
    assert_eq!(tombstoned["purge_after"], floor);
    let (status, resolved) = post(&svc, "/v1/resolve", READER, &access);
    assert_eq!(status, 410, "{resolved}");
    assert_eq!(resolved["status"], "TOMBSTONED");
    drop(svc);
    let chunks = fixture.0.join("store/staging").join(&session);
    {
        let mut store = fixture.store();
        let mut ledger = fixture.ledger();
        ledger.recover(&mut store).unwrap();
        assert_eq!(
            ledger.purge(&mut store, floor - 1),
            Ok(retention::PurgeReport::default())
        );
        assert_eq!(fs::read(chunks.join("0")).unwrap(), PLAIN);
        assert_eq!(
            ledger.purge(&mut store, floor),
            Ok(retention::PurgeReport {
                purged: vec![root.clone()],
                pending: vec![],
            })
        );
        assert!(!chunks.exists());
    }
    {
        let store = fixture.store();
        let record = &store.state.objects[&root];
        assert_eq!(record.state, store::ObjectState::Purged);
        assert_eq!(record.tombstone_reason, Some(retention::POLICY_RESTRICTED));
        assert_eq!(record.retention_until, Some(floor));
        assert!(!record.manifest.is_empty() && !record.envelope.is_empty());
        assert_eq!(
            record
                .history
                .iter()
                .map(|event| event.state)
                .collect::<Vec<_>>(),
            [
                store::ObjectState::Publishing,
                store::ObjectState::Available,
                store::ObjectState::Tombstoned,
                store::ObjectState::Purged
            ]
        );
        assert_eq!(record.history.last().unwrap().at, floor);
    }
    let svc = fixture.start();
    let (status, resolved) = post(&svc, "/v1/resolve", READER, &access);
    assert_eq!(status, 410, "{resolved}");
    assert_eq!(resolved["object_state"], "PURGED");
    assert!(resolved.get("object_key").is_none());
    let (status, usage) = http(&svc, "GET", "/v1/usage", OWNER, b"");
    assert_eq!(status, 200);
    assert_eq!(
        serde_json::from_slice::<Value>(&usage).unwrap()["quota_used"],
        0
    );
}

#[test]
fn terminal_route_refuses_incomplete_uncommitted_and_mismatched_heads() {
    let fixture = Fixture::new();
    let svc = fixture.start();
    let (root, _) = publish(&svc);
    let (request, _) = terminal_head::observation(&id(10), &root, 4);
    for (n, field) in ["block_height", "app_hash", "store_proof_digest"]
        .iter()
        .enumerate()
    {
        let mut incomplete = request.clone();
        incomplete["request_id"] = json!(id(11 + n as u8));
        incomplete.as_object_mut().unwrap().remove(*field);
        assert_error(
            post(&svc, "/v1/retention/terminal", OBSERVER, &incomplete),
            400,
            "Malformed",
        );
    }
    let (soft, _) = terminal_head::observation(&id(20), &root, 3);
    assert_error(
        post(&svc, "/v1/retention/terminal", OBSERVER, &soft),
        409,
        "FINALITY_UNAVAILABLE",
    );
    for (n, field, value) in [
        (21, "block_height", json!(1011)),
        (22, "commit", json!(hex::encode([0xee; 32]))),
        (23, "app_hash", json!(hex::encode([0xee; 32]))),
    ] {
        let mut mismatch = request.clone();
        mismatch["request_id"] = json!(id(n));
        mismatch[field] = value;
        assert_error(
            post(&svc, "/v1/retention/terminal", OBSERVER, &mismatch),
            409,
            "FINALITY_UNAVAILABLE",
        );
    }
    assert_error(
        post(&svc, "/v1/retention/terminal", OWNER, &request),
        401,
        "Unauthorized",
    );
    drop(svc);
    assert!(fixture
        .ledger()
        .entry(&root)
        .and_then(|e| e.terminal.as_ref())
        .is_none());
    let svc = fixture.start();
    assert_error(
        tombstone(&svc, &root, 30, retention::OWNER_WITHDRAWN),
        409,
        "EVIDENCE_UNAVAILABLE",
    );
    assert_eq!(
        post(&svc, "/v1/retention/terminal", OBSERVER, &request).0,
        200
    );
}
