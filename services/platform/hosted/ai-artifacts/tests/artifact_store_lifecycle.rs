//! Repair record 22.33.1428 (audit 22.30.80).
//! artifact-finalize-live-state: finalize_checks_live_state_but_replays_history.
//! artifact-content-corruption: same_length_corruption_is_quarantined_across_reopen.
//! artifact-listener-admission: bounded_admission_and_total_deadline,
//! slow_tls_handshake_releases_admission, deadline_closes_peer_while_waiting_for_store.
//! artifact-staging-reclamation: expiration_reclaims_across_restart,
//! failed_encrypted_finalization_reclaims, duplicate_publication_reclaims_across_restart,
//! interrupted_reclamation_keeps_quota_until_synced_deletion,
//! legacy_terminal_staging_and_quota_are_reconciled.
//! missing-platform-interface-contracts: hosted_contract_drives_producer_and_consumer
//! proves the scoped hosted shapes documented in authority.rs.
//! Wiring request: domain 8 must publish contracts/platform-hosted.md and
//! contracts/platform-cli.md (or correct registry anchors), including hosted
//! identity/config/durable formats, CLI shapes and retirement dispositions;
//! those pages, registry changes and CLI proofs are outside this task's scope.
//! Baseline: not run; the ordered task contract permits gates only after the
//! entire implementation, and this test target did not exist on the base.
//! Revision, exact commands, exit codes and private logs are supplied by hf-done.
#[path = "../src/main.rs"]
#[allow(dead_code)]
mod production;
// Production authority and retention resolve their store through the crate root.
use production::{authority, crypto, retention, store};

use chacha20poly1305::aead::{Aead, KeyInit, Payload};
use chacha20poly1305::{XChaCha20Poly1305, XNonce};
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::codec::derive_market;
use layerx_programs_ai_market::evidence::{
    chunk_count, decode_envelope, decode_manifest, encode_envelope, encode_manifest, manifest_root,
    object_content_root, publisher_digest, verify_publisher, ArtifactContext, ArtifactError,
    ArtifactKind, ArtifactManifest, ContentAssembler, Items, Privacy, PublisherEnvelope,
    CHUNK_BYTES, MAX_ENVELOPE_BYTES, MAX_MANIFEST_BYTES,
};
use layerx_programs_ai_market::types::{
    ChainDomain, PolicyDigest, PrincipalId, ProgramId, PublicKey32, Signature64, Version,
};
use rustls::pki_types::ServerName;
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use store::{ObjectState, SessionState, Store, RETENTION_SECS, STAGING_SECS};

const TOKEN: &str = "repair-owner-token";
const READER: &str = "repair-reader-token";
const PRINCIPAL: [u8; 32] = [0x55; 32];

fn rid(n: u8) -> String {
    hex::encode([n; 32])
}
fn context() -> ArtifactContext {
    let chain = ChainDomain::new([0x11; 32]).unwrap();
    let program = ProgramId::new([0x22; 32]).unwrap();
    ArtifactContext {
        chain,
        program,
        market: derive_market(chain, program).unwrap(),
        policy: PolicyDigest::new([0x44; 32]).unwrap(),
    }
}
fn utc() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

struct Fixture {
    root: PathBuf,
}
impl Fixture {
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "repair-22-33-1428-{}",
            hex::encode(crypto::random::<16>().unwrap())
        ));
        fs::create_dir_all(&root).unwrap();
        fs::write(root.join("kek"), [0x73; 32]).unwrap();
        Self { root }
    }
    fn data(&self) -> PathBuf {
        self.root.join("data")
    }
    fn staging(&self, session: &str) -> PathBuf {
        self.data().join("staging").join(session)
    }
    fn open(&self) -> Store {
        Store::open(&self.data(), &self.root.join("revocations")).unwrap()
    }
    fn start(&self) -> Server {
        self.start_with(4, Duration::from_secs(3), false)
    }
    fn start_with(&self, workers: usize, deadline: Duration, tls: bool) -> Server {
        let hash = |token: &str| hex::encode(Sha256::digest(token.as_bytes()));
        let issued =
            tls.then(|| rcgen::generate_simple_self_signed(vec!["localhost".to_string()]).unwrap());
        let (listener_config, server_tls, client_tls) = if let Some(issued) = issued {
            let cert = self.root.join("cert.pem");
            let key = self.root.join("key.pem");
            fs::write(&cert, issued.cert.pem()).unwrap();
            fs::write(&key, issued.signing_key.serialize_pem()).unwrap();
            let mut roots = RootCertStore::empty();
            roots.add(issued.cert.der().clone()).unwrap();
            let client = ClientConfig::builder_with_provider(Arc::new(
                rustls::crypto::ring::default_provider(),
            ))
            .with_safe_default_protocol_versions()
            .unwrap()
            .with_root_certificates(roots)
            .with_no_client_auth();
            (
                json!({"tls": {"certificate": cert, "private_key": key}}),
                Some(production::tls_config(&cert, &key).unwrap()),
                Some(Arc::new(client)),
            )
        } else {
            (json!("plaintext"), None, None)
        };
        let config: authority::Config = serde_json::from_value(json!({
            "capacity_bytes": 16u64 << 20, "listener": listener_config,
            "revocation_mark": self.root.join("revocations"),
            "retention_mark": self.root.join("retention"),
            "tenants": [
                {"id": "owner", "token_sha256": hash(TOKEN), "quota_bytes": 16u64 << 20},
                {"id": "reader", "token_sha256": hash(READER), "quota_bytes": 0}
            ],
            "publishers": [{"tenant": "owner", "principal": hex::encode(PRINCIPAL), "generation": 1,
                "key": hex::encode(SigningKey::from_bytes(&[1; 32]).verifying_key().to_bytes())}]
        }))
        .unwrap();
        let mut store = self.open();
        let mut ledger = retention::Ledger::open(&self.data(), &config.retention_mark()).unwrap();
        ledger.recover(&mut store).unwrap();
        production::reconcile(&mut store, &mut ledger, utc()).unwrap();
        let service = Arc::new(production::Service {
            config,
            kms: Box::new(crypto::LocalFileKeyProvider::load(&self.root.join("kek")).unwrap()),
            store: Mutex::new(store),
            ledger: Mutex::new(ledger),
        });
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        listener.set_nonblocking(true).unwrap();
        let pool = Arc::new(production::ConnectionPool::new(
            Arc::clone(&service),
            server_tls,
            workers,
            deadline,
        ));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let accepting = Arc::clone(&pool);
        let thread = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((socket, _)) => accepting.admit(socket),
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1))
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            }
        });
        Server {
            addr,
            service,
            pool,
            stop,
            thread: Some(thread),
            client_tls,
        }
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}
struct Server {
    addr: SocketAddr,
    service: Arc<production::Service>,
    pool: Arc<production::ConnectionPool>,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
    client_tls: Option<Arc<ClientConfig>>,
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.thread.take().unwrap().join().unwrap();
        wait_until(|| self.pool.active() == 0, Duration::from_secs(5));
    }
}
fn wait_until(mut condition: impl FnMut() -> bool, budget: Duration) {
    let deadline = Instant::now() + budget;
    while !condition() {
        assert!(
            Instant::now() < deadline,
            "condition did not complete in {budget:?}"
        );
        std::thread::sleep(Duration::from_millis(2));
    }
}
fn socket(addr: SocketAddr) -> TcpStream {
    let socket = TcpStream::connect(addr).unwrap();
    socket
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    socket
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    socket
}
fn exchange(
    mut stream: impl Read + Write,
    method: &str,
    path: &str,
    token: &str,
    body: &[u8],
) -> (u16, Vec<u8>) {
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
    stream.write_all(body).unwrap();
    stream.flush().unwrap();
    let mut response = Vec::new();
    stream.read_to_end(&mut response).unwrap();
    let split = response
        .windows(4)
        .position(|w| w == b"\r\n\r\n")
        .expect("HTTP response");
    let status = std::str::from_utf8(&response[..split])
        .unwrap()
        .split_whitespace()
        .nth(1)
        .unwrap()
        .parse()
        .unwrap();
    (status, response[split + 4..].to_vec())
}
fn http(server: &Server, method: &str, path: &str, token: &str, body: &[u8]) -> (u16, Vec<u8>) {
    let stream = socket(server.addr);
    match &server.client_tls {
        None => exchange(stream, method, path, token, body),
        Some(config) => {
            let conn = ClientConnection::new(
                Arc::clone(config),
                ServerName::try_from("localhost").unwrap(),
            )
            .unwrap();
            exchange(StreamOwned::new(conn, stream), method, path, token, body)
        }
    }
}
fn post(server: &Server, route: &str, token: &str, body: &Value) -> (u16, Value) {
    let (status, bytes) = http(server, "POST", route, token, body.to_string().as_bytes());
    (status, serde_json::from_slice(&bytes).unwrap())
}
fn ok(server: &Server, route: &str, body: &Value) -> Value {
    let (status, value) = post(server, route, TOKEN, body);
    assert_eq!(status, 200, "{route}: {value}");
    value
}
fn prepare(server: &Server, id: u8, length: usize, privacy: Privacy) -> Value {
    ok(
        server,
        "/v1/prepare",
        &json!({"request_id": rid(id), "kind": ArtifactKind::Input as u8,
        "privacy": privacy as u8, "context": hex::encode(context().bytes()), "byte_length": length}),
    )
}
fn upload(server: &Server, session: &str, bytes: &[u8]) {
    for (index, part) in bytes.chunks(CHUNK_BYTES as usize).enumerate() {
        let (status, value) = http(
            server,
            "PUT",
            &format!("/v1/upload/{session}/{index}"),
            TOKEN,
            part,
        );
        assert_eq!(status, 200, "{}", String::from_utf8_lossy(&value));
    }
}
fn envelope(bytes: &[u8], privacy: Privacy) -> Vec<u8> {
    let count = chunk_count(bytes.len() as u64).unwrap();
    let manifest = ArtifactManifest {
        kind: ArtifactKind::Input,
        privacy,
        context: context(),
        epoch: 9,
        publisher: PrincipalId::new(PRINCIPAL).unwrap(),
        subject: [0x31; 32],
        byte_length: bytes.len() as u64,
        chunk_count: count,
        content_root: object_content_root(bytes, &mut vec![[0; 32]; count as usize]).unwrap(),
        parents: Items::Typed(&[]),
        declaration_root: [0; 32],
        reproduction_root: [0; 32],
        access_policy_root: if privacy == Privacy::Encrypted {
            [0x66; 32]
        } else {
            [0; 32]
        },
        not_after_height: 0,
    };
    let mut encoded = vec![0; MAX_MANIFEST_BYTES];
    let n = encode_manifest(&manifest, &mut encoded).unwrap();
    encoded.truncate(n);
    let signing = SigningKey::from_bytes(&[1; 32]);
    let generation = Version::new(1).unwrap();
    let digest = publisher_digest(&context(), manifest_root(&encoded).unwrap(), generation);
    let envelope = PublisherEnvelope {
        manifest: &encoded,
        generation,
        key: PublicKey32(signing.verifying_key().to_bytes()),
        signature: Signature64(signing.sign(&digest).to_bytes()),
    };
    let mut out = vec![0; MAX_ENVELOPE_BYTES];
    let n = encode_envelope(&envelope, &mut out).unwrap();
    out.truncate(n);
    out
}
fn publication(server: &Server, id: u8, bytes: &[u8]) -> (Value, Value) {
    let prepared = prepare(server, id, bytes.len(), Privacy::Public);
    let session = prepared["session"].as_str().unwrap();
    upload(server, session, bytes);
    let request = json!({"request_id": rid(id + 1), "session": session, "envelope": hex::encode(envelope(bytes, Privacy::Public))});
    let receipt = ok(server, "/v1/finalize", &request);
    (request, receipt)
}
fn usage(server: &Server) -> u64 {
    let (status, body) = http(server, "GET", "/v1/usage", TOKEN, &[]);
    assert_eq!(status, 200);
    serde_json::from_slice::<Value>(&body).unwrap()["quota_used"]
        .as_u64()
        .unwrap()
}
fn access(root: &Value, index: u32) -> Value {
    json!({"root": root, "purpose": "evaluation", "index": index})
}
fn assert_consumer(server: &Server, receipt: &Value, bytes: &[u8]) {
    let (status, resolved) = post(server, "/v1/resolve", READER, &access(&receipt["root"], 0));
    assert_eq!(status, 200, "{resolved}");
    assert_eq!(resolved["status"], "AVAILABLE_PUBLIC");
    let encoded = hex::decode(resolved["envelope"].as_str().unwrap()).unwrap();
    let envelope = decode_envelope(&encoded).unwrap();
    let verified = verify_publisher(&envelope).unwrap();
    assert_eq!(hex::encode(verified.root.bytes()), receipt["root"]);
    let manifest = decode_manifest(envelope.manifest).unwrap();
    let mut scratch = vec![[0; 32]; manifest.chunk_count as usize];
    let mut assembler = ContentAssembler::new(&manifest, &mut scratch).unwrap();
    let mut assembled = Vec::new();
    for index in 0..manifest.chunk_count {
        let (status, chunk) = http(
            server,
            "POST",
            "/v1/content",
            READER,
            access(&receipt["root"], index).to_string().as_bytes(),
        );
        assert_eq!(status, 200);
        assembler.deliver(index, &chunk).unwrap();
        assembled.extend_from_slice(&chunk);
    }
    assembler.finish().unwrap();
    assert_eq!(assembled, bytes);
}

#[test]
fn hosted_contract_drives_producer_and_consumer() {
    let fixture = Fixture::new();
    // Both production listener profiles carry the same authenticated contract.
    for tls in [false, true] {
        let server = fixture.start_with(4, Duration::from_secs(3), tls);
        let id = if tls { 11 } else { 1 };
        let (status, _) = http(&server, "GET", "/v1/usage", "wrong-token", &[]);
        assert_eq!(status, 401);
        let bytes = vec![id; CHUNK_BYTES as usize + 123];
        let (request, receipt) = publication(&server, id, &bytes);
        assert_eq!(receipt["protocol_receipt"], false);
        assert_eq!(receipt["acknowledgement"], "HISTORICAL_PUBLICATION");
        assert_eq!(receipt["availability_scope"], "AT_PUBLICATION");
        assert_eq!(receipt["authority_scope"], "AT_PUBLICATION");
        assert_eq!(receipt["state"], "AVAILABLE");
        assert_eq!(ok(&server, "/v1/finalize", &request), receipt);
        let mut conflict = request.clone();
        conflict["envelope"] = json!("00");
        let (status, refusal) = post(&server, "/v1/finalize", TOKEN, &conflict);
        assert_eq!(status, 409);
        assert_eq!(refusal["error"], "IdempotencyConflict");
        assert_consumer(&server, &receipt, &bytes);
        drop(server);
        let reopened = fixture.start_with(4, Duration::from_secs(3), tls);
        assert_consumer(&reopened, &receipt, &bytes);
    }
}

#[test]
fn finalize_checks_live_state_but_replays_history() {
    for disposition in ["tombstoned", "purged", "quarantined", "revoked"] {
        let fixture = Fixture::new();
        let server = fixture.start();
        let bytes = b"historical publication bytes";
        let (request, receipt) = publication(&server, 1, bytes);
        let expected = match disposition {
            "revoked" => {
                ok(
                    &server,
                    "/v1/publishers/revoke",
                    &json!({"request_id": rid(3), "principal": hex::encode(PRINCIPAL), "generation": 1}),
                );
                (403, "AuthorityRevoked")
            }
            "quarantined" => {
                fs::write(
                    fixture
                        .staging(request["session"].as_str().unwrap())
                        .join("0"),
                    vec![b'x'; bytes.len()],
                )
                .unwrap();
                let (status, _) =
                    post(&server, "/v1/content", READER, &access(&receipt["root"], 0));
                assert_eq!(status, 409);
                (409, "IntegrityConflict")
            }
            _ => {
                ok(
                    &server,
                    "/v1/tombstone",
                    &json!({"request_id": rid(3), "root": receipt["root"], "reason": retention::POLICY_RESTRICTED}),
                );
                if disposition == "purged" {
                    // A committed retention deadline is a store input. Advance
                    // the production purge pass beyond it without wall-clock sleep.
                    let mut store = server.service.store.lock().unwrap();
                    store
                        .state
                        .objects
                        .get_mut(receipt["root"].as_str().unwrap())
                        .unwrap()
                        .retention_until = Some(utc() + RETENTION_SECS);
                    store.persist().unwrap();
                    assert_eq!(
                        store.purge_due(u64::MAX).unwrap(),
                        vec![receipt["root"].as_str().unwrap().to_string()]
                    );
                }
                (410, "Tombstoned")
            }
        };
        assert_eq!(ok(&server, "/v1/finalize", &request), receipt);
        let mut fresh = request.clone();
        fresh["request_id"] = json!(rid(4));
        let (status, refusal) = post(&server, "/v1/finalize", TOKEN, &fresh);
        assert_eq!(status, expected.0, "{disposition}: {refusal}");
        assert_eq!(refusal["error"], expected.1);
        drop(server);
        let server = fixture.start();
        assert_eq!(ok(&server, "/v1/finalize", &request), receipt);
        fresh["request_id"] = json!(rid(5));
        let (status, refusal) = post(&server, "/v1/finalize", TOKEN, &fresh);
        assert_eq!(status, expected.0, "reopened {disposition}: {refusal}");
        assert_eq!(refusal["error"], expected.1);
        drop(server);
        // A new staging session with the same root must obey the same lifecycle.
        let reopened = fixture.start();
        let prepared = prepare(&reopened, 6, bytes.len(), Privacy::Public);
        upload(&reopened, prepared["session"].as_str().unwrap(), bytes);
        let (status, refusal) = post(
            &reopened,
            "/v1/finalize",
            TOKEN,
            &json!({"request_id": rid(7), "session": prepared["session"], "envelope": request["envelope"]}),
        );
        assert_eq!(status, expected.0, "duplicate {disposition}: {refusal}");
        assert_eq!(refusal["error"], expected.1);
    }
}

#[test]
fn same_length_corruption_is_quarantined_across_reopen() {
    for corrupt_before_reopen in [false, true] {
        let fixture = Fixture::new();
        let server = fixture.start();
        let bytes = vec![0x5a; CHUNK_BYTES as usize + 17];
        let (request, receipt) = publication(&server, 1, &bytes);
        assert_consumer(&server, &receipt, &bytes);
        let session = request["session"].as_str().unwrap();
        let corrupt = || {
            let path = fixture.staging(session).join("1");
            let mut damaged = fs::read(&path).unwrap();
            damaged[3] ^= 0xff;
            fs::write(path, damaged).unwrap();
        };
        if corrupt_before_reopen {
            corrupt();
        }
        drop(server);
        let server = fixture.start();
        if !corrupt_before_reopen {
            corrupt();
        }
        // Even a request for the undamaged chunk authenticates the whole root.
        let (status, refusal) = post(&server, "/v1/content", READER, &access(&receipt["root"], 0));
        assert_eq!(status, 409, "{refusal}");
        assert_eq!(refusal["error"], "IntegrityConflict");
        assert_eq!(
            server.service.store.lock().unwrap().state.objects[receipt["root"].as_str().unwrap()]
                .state,
            ObjectState::Quarantined
        );
        drop(server);
        let server = fixture.start();
        let (status, refusal) = post(&server, "/v1/resolve", READER, &access(&receipt["root"], 0));
        assert_eq!(status, 409, "{refusal}");
        assert_eq!(refusal["error"], "IntegrityConflict");
        // The real consumer independently rejects those same damaged bytes.
        let encoded = hex::decode(request["envelope"].as_str().unwrap()).unwrap();
        let env = decode_envelope(&encoded).unwrap();
        let manifest = decode_manifest(env.manifest).unwrap();
        let mut scratch = vec![[0; 32]; manifest.chunk_count as usize];
        let mut consumer = ContentAssembler::new(&manifest, &mut scratch).unwrap();
        for index in 0..manifest.chunk_count {
            consumer
                .deliver(
                    index,
                    &fs::read(fixture.staging(session).join(index.to_string())).unwrap(),
                )
                .unwrap();
        }
        assert!(consumer.finish().is_err());
    }
}

#[test]
fn expiration_reclaims_across_restart() {
    let fixture = Fixture::new();
    let server = fixture.start();
    let bytes = vec![0x37; 4096];
    let prepared = prepare(&server, 1, bytes.len(), Privacy::Public);
    let session = prepared["session"].as_str().unwrap();
    upload(&server, session, &bytes);
    assert_eq!(usage(&server), bytes.len() as u64);
    {
        let mut store = server.service.store.lock().unwrap();
        store.state.sessions.get_mut(session).unwrap().created_at = utc() - STAGING_SECS;
        store.persist().unwrap();
    }
    drop(server);
    let server = fixture.start();
    assert!(!fixture.staging(session).exists());
    assert_eq!(usage(&server), 0);
    assert_eq!(
        server.service.store.lock().unwrap().state.sessions[session].state,
        SessionState::Expired
    );
    production::maintain(&server.service);
    assert_eq!(usage(&server), 0);
    drop(server);
    assert_eq!(
        fixture
            .open()
            .state
            .quota_used
            .get("owner")
            .copied()
            .unwrap_or(0),
        0
    );
}

#[test]
fn failed_encrypted_finalization_reclaims() {
    let fixture = Fixture::new();
    let server = fixture.start();
    // A valid frame whose authentication tag is damaged still has a correct
    // published content commitment: it must fail the real AEAD verification.
    let plain = b"private bytes with a damaged tag";
    let length = 34 + 16 + 8 + plain.len();
    let prepared = prepare(&server, 1, length, Privacy::Encrypted);
    let session = prepared["session"].as_str().unwrap();
    let key: [u8; 32] = hex::decode(prepared["object_key"].as_str().unwrap())
        .unwrap()
        .try_into()
        .unwrap();
    let nonce = [0x39; 24];
    let mut aad = crypto::FRAME_AAD_PREFIX.to_vec();
    aad.extend_from_slice(&context().bytes());
    aad.extend_from_slice(&[0x31; 32]);
    aad.push(ArtifactKind::Input as u8);
    aad.extend_from_slice(&0u32.to_be_bytes());
    let body = [&(plain.len() as u64).to_be_bytes()[..], plain].concat();
    let sealed = XChaCha20Poly1305::new((&key).into())
        .encrypt(
            XNonce::from_slice(&nonce),
            Payload {
                msg: &body,
                aad: &aad,
            },
        )
        .unwrap();
    let mut bytes = crypto::FRAME_VERSION.to_be_bytes().to_vec();
    bytes.extend_from_slice(&0u32.to_be_bytes());
    bytes.extend_from_slice(&nonce);
    bytes.extend_from_slice(&(sealed.len() as u32).to_be_bytes());
    bytes.extend_from_slice(&sealed);
    *bytes.last_mut().unwrap() ^= 1;
    assert_eq!(bytes.len(), length);
    upload(&server, session, &bytes);
    let request = json!({"request_id": rid(2), "session": session, "envelope": hex::encode(envelope(&bytes, Privacy::Encrypted))});
    let (status, refusal) = post(&server, "/v1/finalize", TOKEN, &request);
    assert_ne!(status, 200);
    assert_eq!(refusal["error"], "IntegrityConflict");
    assert!(!fixture.staging(session).exists());
    assert_eq!(usage(&server), 0);
    drop(server);
    let server = fixture.start();
    assert!(!fixture.staging(session).exists());
    assert_eq!(usage(&server), 0);
    let store = server.service.store.lock().unwrap();
    assert_eq!(
        store.state.sessions[session].state,
        SessionState::Quarantined
    );
    assert!(store.state.sessions[session].wrapped_key.is_none());
    assert!(store.state.objects.is_empty());
    assert!(store.state.reclaiming.is_empty());
}

#[test]
fn duplicate_publication_reclaims_across_restart() {
    let fixture = Fixture::new();
    let bytes = vec![0x25; CHUNK_BYTES as usize + 29];
    let server = fixture.start();
    let (original, receipt) = publication(&server, 1, &bytes);
    drop(server);
    let server = fixture.start();
    let (duplicate, repeated) = publication(&server, 3, &bytes);
    assert_eq!(receipt, repeated);
    assert_ne!(original["session"], duplicate["session"]);
    assert!(fixture
        .staging(original["session"].as_str().unwrap())
        .exists());
    assert!(!fixture
        .staging(duplicate["session"].as_str().unwrap())
        .exists());
    assert_eq!(usage(&server), bytes.len() as u64);
    drop(server);
    let server = fixture.start();
    assert_eq!(usage(&server), bytes.len() as u64);
    assert!(!fixture
        .staging(duplicate["session"].as_str().unwrap())
        .exists());
    assert_eq!(ok(&server, "/v1/finalize", &duplicate), receipt);
    assert_consumer(&server, &receipt, &bytes);
}

#[test]
fn interrupted_reclamation_keeps_quota_until_synced_deletion() {
    let fixture = Fixture::new();
    let server = fixture.start();
    let bytes = vec![0x62; 128];
    let prepared = prepare(&server, 1, bytes.len(), Privacy::Public);
    let session = prepared["session"].as_str().unwrap();
    upload(&server, session, &bytes);
    // A filesystem fault makes remove_dir_all fail after the intent is synced.
    fs::rename(fixture.staging(session), fixture.root.join("saved-staging")).unwrap();
    fs::write(fixture.staging(session), &bytes).unwrap();
    {
        let mut store = server.service.store.lock().unwrap();
        assert_eq!(
            store.expire_staging(utc() + STAGING_SECS),
            Err(ArtifactError::StorageFailure)
        );
        assert_eq!(store.state.reclaiming[session], SessionState::Expired);
        assert_eq!(store.state.quota_used["owner"], bytes.len() as u64);
    }
    drop(server);
    assert!(matches!(
        Store::open(&fixture.data(), &fixture.root.join("revocations")),
        Err(ArtifactError::StorageFailure)
    ));
    let persisted: Value =
        serde_json::from_slice(&fs::read(fixture.data().join("state.json")).unwrap()).unwrap();
    assert_eq!(persisted["reclaiming"][session], "Expired");
    assert_eq!(persisted["quota_used"]["owner"], bytes.len() as u64);
    fs::remove_file(fixture.staging(session)).unwrap();
    fs::rename(fixture.root.join("saved-staging"), fixture.staging(session)).unwrap();
    let server = fixture.start();
    assert!(!fixture.staging(session).exists());
    assert_eq!(usage(&server), 0);
    assert!(server
        .service
        .store
        .lock()
        .unwrap()
        .state
        .reclaiming
        .is_empty());
    drop(server);
    let server = fixture.start();
    assert_eq!(usage(&server), 0);
}

#[test]
fn bounded_admission_and_total_deadline() {
    let fixture = Fixture::new();
    let budget = Duration::from_millis(500);
    let server = fixture.start_with(2, budget, false);
    let mut partial_head = socket(server.addr);
    partial_head
        .write_all(b"POST /v1/prepare HTTP/1.1\r\nX-Slow: ")
        .unwrap();
    wait_until(|| server.pool.active() == 1, Duration::from_secs(1));
    // Authenticated traffic progresses while one unauthenticated request drips.
    assert_eq!(usage(&server), 0);
    wait_until(|| server.pool.active() == 1, Duration::from_secs(1));
    let mut partial_body = socket(server.addr);
    write!(partial_body, "POST /v1/prepare HTTP/1.1\r\nAuthorization: Bearer {TOKEN}\r\nContent-Length: 1000\r\n\r\n{{").unwrap();
    wait_until(|| server.pool.active() == 2, Duration::from_secs(1));
    let mut rejected = socket(server.addr);
    let mut response = String::new();
    rejected.read_to_string(&mut response).unwrap();
    assert!(response.starts_with("HTTP/1.1 503"), "{response}");
    assert!(response.contains("Retry-After: 1"));
    let started = Instant::now();
    // Each byte arrives before any idle timeout, but cannot renew the budget.
    let drip = |mut stream: TcpStream| {
        std::thread::spawn(move || {
            for _ in 0..25 {
                std::thread::sleep(Duration::from_millis(40));
                if stream.write_all(b" ").is_err() {
                    break;
                }
            }
        })
    };
    let head = drip(partial_head);
    let body = drip(partial_body);
    wait_until(
        || server.pool.active() == 0,
        budget + Duration::from_millis(400),
    );
    assert!(started.elapsed() < budget + Duration::from_millis(400));
    assert_eq!(usage(&server), 0);
    head.join().unwrap();
    body.join().unwrap();
    // Parse/authentication failures also release their permits, repeatedly.
    for _ in 0..8 {
        let mut malformed = socket(server.addr);
        malformed.write_all(b"garbage\r\n\r\n").unwrap();
        let mut response = Vec::new();
        malformed.read_to_end(&mut response).unwrap();
        wait_until(|| server.pool.active() == 0, Duration::from_secs(1));
        let (status, _) = http(&server, "GET", "/v1/usage", "invalid", &[]);
        assert_eq!(status, 401);
    }
    let bytes = b"normal publication after saturation";
    let (_, receipt) = publication(&server, 1, bytes);
    assert_consumer(&server, &receipt, bytes);
}

#[test]
fn slow_tls_handshake_releases_admission() {
    let fixture = Fixture::new();
    let server = fixture.start_with(2, Duration::from_millis(500), true);
    let mut stalled = socket(server.addr);
    // A partial TLS record reaches the production TLS decoder and total budget.
    stalled
        .write_all(&[0x16, 0x03, 0x03, 0x00, 0x40, 0x01])
        .unwrap();
    wait_until(|| server.pool.active() == 1, Duration::from_secs(1));
    assert_eq!(usage(&server), 0);
    wait_until(|| server.pool.active() == 0, Duration::from_secs(1));
    assert_eq!(usage(&server), 0);
}

#[test]
fn deadline_closes_peer_while_waiting_for_store() {
    let fixture = Fixture::new();
    let server = fixture.start_with(2, Duration::from_millis(300), false);
    let guard = server.service.store.lock().unwrap();
    let mut blocked = socket(server.addr);
    let started = Instant::now();
    write!(
        blocked,
        "GET /v1/usage HTTP/1.1\r\nAuthorization: Bearer {TOKEN}\r\nContent-Length: 0\r\n\r\n"
    )
    .unwrap();
    wait_until(|| server.pool.active() == 1, Duration::from_secs(1));
    let mut response = Vec::new();
    blocked.read_to_end(&mut response).unwrap();
    assert!(
        response.is_empty(),
        "a timed-out request cannot advertise a response"
    );
    assert!(started.elapsed() < Duration::from_millis(800));
    drop(guard);
    wait_until(|| server.pool.active() == 0, Duration::from_secs(1));
    assert_eq!(usage(&server), 0);
}

#[test]
fn legacy_terminal_staging_and_quota_are_reconciled() {
    let fixture = Fixture::new();
    let server = fixture.start();
    let bytes = vec![0x37; 1024];
    let prepared = prepare(&server, 1, bytes.len(), Privacy::Public);
    let session = prepared["session"].as_str().unwrap();
    upload(&server, session, &bytes);
    {
        // Recorded legacy snapshot: expired payload still present, but its
        // reservation was released by the old writer. Reopen migrates it.
        let mut store = server.service.store.lock().unwrap();
        store.state.sessions.get_mut(session).unwrap().state = SessionState::Expired;
        store.release_quota("owner", bytes.len() as u64);
        store.persist().unwrap();
    }
    drop(server);
    let server = fixture.start();
    assert!(!fixture.staging(session).exists());
    assert_eq!(usage(&server), 0);
    let prepared = prepare(&server, 2, 2048, Privacy::Public);
    assert!(fixture
        .staging(prepared["session"].as_str().unwrap())
        .exists());
    drop(server);
    let mut store = fixture.open();
    // Reconcile quotas from retained reservations, not a stale reported count.
    store.state.quota_used.insert("owner".into(), 0);
    store.persist().unwrap();
    drop(store);
    let server = fixture.start();
    assert_eq!(usage(&server), 2048);
}
