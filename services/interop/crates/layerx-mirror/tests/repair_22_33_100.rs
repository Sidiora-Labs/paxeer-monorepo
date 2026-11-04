// Audit finding/test mapping and wiring requests: ../src/repair_22_33_100.kvx

mod peer {
    use std::fs::{self, OpenOptions};
    use std::io::{Read, Write};
    use std::net::{SocketAddr, TcpListener, TcpStream};
    use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};
    use std::path::PathBuf;
    use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
    use std::sync::{Arc, Mutex};
    use std::thread::{self, JoinHandle};
    use std::time::Duration;

    use layerx_mirror::committed::{CommittedSourceConfig, ReceiptKeyConfig};
    use layerx_mirror::rpc::{RpcEndpointConfig, RpcQuorumConfig};
    use openssl::asn1::Asn1Time;
    use openssl::bn::BigNum;
    use openssl::hash::MessageDigest;
    use openssl::pkey::PKey;
    use openssl::rsa::Rsa;
    use openssl::ssl::{SslAcceptor, SslMethod};
    use openssl::x509::extension::{
        BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
    };
    use openssl::x509::{X509NameBuilder, X509};
    use serde_json::{json, Value};

    static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);
    const TOKEN: &str = "committed-source-loopback-test";

    pub fn fixture() -> Value {
        serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
            .unwrap_or_else(|error| panic!("committed fixture: {error}"))
    }

    pub fn bytes(text: &Value) -> Vec<u8> {
        let digits = text.as_str().unwrap_or_else(|| panic!("hex string"));
        let digits = digits
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("hex prefix"));
        (0..digits.len())
            .step_by(2)
            .map(|index| {
                u8::from_str_radix(&digits[index..index + 2], 16).unwrap_or_else(|e| panic!("{e}"))
            })
            .collect()
    }

    pub struct TestRpc {
        pub config: CommittedSourceConfig,
        pub requests: Arc<Mutex<Vec<(usize, Value)>>>,
        pub state: Arc<Mutex<PeerState>>,
        running: Arc<AtomicBool>,
        addresses: Vec<SocketAddr>,
        threads: Vec<JoinHandle<()>>,
        directory: PathBuf,
    }

    pub struct PeerState {
        pub head: u64,
        pub refuse: bool,
        pub drip: bool,
        pub handshake_delay: bool,
        pub wrong_parent: bool,
    }

    impl TestRpc {
        pub fn new() -> Self {
            let documents = vec![fixture(); 3];
            let state = Arc::new(Mutex::new(PeerState {
                head: 1000,
                refuse: false,
                drip: false,
                handshake_delay: false,
                wrong_parent: false,
            }));
            let directory = std::env::temp_dir().join(format!(
                "layerx-committed-{}-{}",
                std::process::id(),
                NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
            ));
            fs::create_dir(&directory).unwrap_or_else(|e| panic!("test directory: {e}"));
            fs::set_permissions(&directory, fs::Permissions::from_mode(0o700))
                .unwrap_or_else(|e| panic!("directory permissions: {e}"));
            let token_path = directory.join("bearer");
            let mut token = OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&token_path)
                .unwrap_or_else(|e| panic!("test bearer: {e}"));
            token
                .write_all(TOKEN.as_bytes())
                .unwrap_or_else(|e| panic!("test bearer: {e}"));

            let key = PKey::from_rsa(Rsa::generate(2048).unwrap_or_else(|e| panic!("RSA: {e}")))
                .unwrap_or_else(|e| panic!("key: {e}"));
            let mut name = X509NameBuilder::new().unwrap_or_else(|e| panic!("name: {e}"));
            name.append_entry_by_text("CN", "Committed source test CA")
                .unwrap_or_else(|e| panic!("name: {e}"));
            let name = name.build();
            let mut ca = X509::builder().unwrap_or_else(|e| panic!("CA: {e}"));
            ca.set_version(2).unwrap_or_else(|e| panic!("version: {e}"));
            let serial = BigNum::from_u32(1)
                .and_then(|n| n.to_asn1_integer())
                .unwrap_or_else(|e| panic!("serial: {e}"));
            ca.set_serial_number(&serial)
                .unwrap_or_else(|e| panic!("serial: {e}"));
            ca.set_subject_name(&name)
                .unwrap_or_else(|e| panic!("subject: {e}"));
            ca.set_issuer_name(&name)
                .unwrap_or_else(|e| panic!("issuer: {e}"));
            ca.set_pubkey(&key)
                .unwrap_or_else(|e| panic!("public key: {e}"));
            let before = Asn1Time::days_from_now(0).unwrap_or_else(|e| panic!("time: {e}"));
            let after = Asn1Time::days_from_now(1).unwrap_or_else(|e| panic!("time: {e}"));
            ca.set_not_before(&before)
                .unwrap_or_else(|e| panic!("time: {e}"));
            ca.set_not_after(&after)
                .unwrap_or_else(|e| panic!("time: {e}"));
            ca.append_extension(
                BasicConstraints::new()
                    .critical()
                    .ca()
                    .build()
                    .unwrap_or_else(|e| panic!("CA constraints: {e}")),
            )
            .unwrap_or_else(|e| panic!("extension: {e}"));
            ca.append_extension(
                KeyUsage::new()
                    .key_cert_sign()
                    .crl_sign()
                    .build()
                    .unwrap_or_else(|e| panic!("CA usage: {e}")),
            )
            .unwrap_or_else(|e| panic!("extension: {e}"));
            ca.sign(&key, MessageDigest::sha256())
                .unwrap_or_else(|e| panic!("CA signature: {e}"));
            let ca = ca.build();
            let ca_path = directory.join("ca.der");
            fs::write(&ca_path, ca.to_der().unwrap_or_else(|e| panic!("DER: {e}")))
                .unwrap_or_else(|e| panic!("CA file: {e}"));

            let mut certificate = X509::builder().unwrap_or_else(|e| panic!("certificate: {e}"));
            certificate
                .set_version(2)
                .unwrap_or_else(|e| panic!("version: {e}"));
            let serial = BigNum::from_u32(2)
                .and_then(|n| n.to_asn1_integer())
                .unwrap_or_else(|e| panic!("serial: {e}"));
            certificate
                .set_serial_number(&serial)
                .unwrap_or_else(|e| panic!("serial: {e}"));
            certificate
                .set_subject_name(&name)
                .unwrap_or_else(|e| panic!("subject: {e}"));
            certificate
                .set_issuer_name(ca.subject_name())
                .unwrap_or_else(|e| panic!("issuer: {e}"));
            certificate
                .set_pubkey(&key)
                .unwrap_or_else(|e| panic!("public key: {e}"));
            certificate
                .set_not_before(&before)
                .unwrap_or_else(|e| panic!("time: {e}"));
            certificate
                .set_not_after(&after)
                .unwrap_or_else(|e| panic!("time: {e}"));
            certificate
                .append_extension(
                    BasicConstraints::new()
                        .critical()
                        .build()
                        .unwrap_or_else(|e| panic!("constraints: {e}")),
                )
                .unwrap_or_else(|e| panic!("extension: {e}"));
            certificate
                .append_extension(
                    KeyUsage::new()
                        .digital_signature()
                        .key_encipherment()
                        .build()
                        .unwrap_or_else(|e| panic!("usage: {e}")),
                )
                .unwrap_or_else(|e| panic!("extension: {e}"));
            certificate
                .append_extension(
                    ExtendedKeyUsage::new()
                        .server_auth()
                        .build()
                        .unwrap_or_else(|e| panic!("usage: {e}")),
                )
                .unwrap_or_else(|e| panic!("extension: {e}"));
            let san = SubjectAlternativeName::new()
                .ip("127.0.0.1")
                .build(&certificate.x509v3_context(Some(&ca), None))
                .unwrap_or_else(|e| panic!("SAN: {e}"));
            certificate
                .append_extension(san)
                .unwrap_or_else(|e| panic!("extension: {e}"));
            certificate
                .sign(&key, MessageDigest::sha256())
                .unwrap_or_else(|e| panic!("signature: {e}"));
            let certificate = certificate.build();
            let mut acceptor = SslAcceptor::mozilla_intermediate(SslMethod::tls())
                .unwrap_or_else(|e| panic!("TLS: {e}"));
            acceptor
                .set_certificate(&certificate)
                .unwrap_or_else(|e| panic!("TLS certificate: {e}"));
            acceptor
                .set_private_key(&key)
                .unwrap_or_else(|e| panic!("TLS key: {e}"));
            acceptor
                .check_private_key()
                .unwrap_or_else(|e| panic!("TLS key check: {e}"));
            let acceptor = Arc::new(acceptor.build());
            let requests = Arc::new(Mutex::new(Vec::new()));
            let running = Arc::new(AtomicBool::new(true));
            let mut addresses = Vec::new();
            let mut endpoints = Vec::new();
            let mut threads = Vec::new();
            for (index, document) in documents.into_iter().enumerate() {
                let listener =
                    TcpListener::bind("127.0.0.1:0").unwrap_or_else(|e| panic!("bind: {e}"));
                let address = listener
                    .local_addr()
                    .unwrap_or_else(|e| panic!("address: {e}"));
                addresses.push(address);
                endpoints.push(RpcEndpointConfig {
                    url: format!("https://{address}/"),
                    ca_certificate_der: ca_path.clone(),
                    bearer_token_file: token_path.clone(),
                    independent_backend: format!("committed-test-{index}"),
                });
                let acceptor = Arc::clone(&acceptor);
                let requests = Arc::clone(&requests);
                let running = Arc::clone(&running);
                let state = Arc::clone(&state);
                threads.push(thread::spawn(move || {
                while running.load(Ordering::Acquire) {
                    let (tcp, _) = listener.accept().unwrap_or_else(|e| panic!("accept: {e}"));
                    if !running.load(Ordering::Acquire) { break; }
                    tcp.set_read_timeout(Some(Duration::from_secs(2))).unwrap_or_else(|e| panic!("timeout: {e}"));
                    tcp.set_write_timeout(Some(Duration::from_secs(2))).unwrap_or_else(|e| panic!("timeout: {e}"));
                    let (refuse, drip, delay) = {
                        let state = state.lock().unwrap();
                        (state.refuse, state.drip && index == 0, state.handshake_delay && index == 0)
                    };
                    if delay { thread::sleep(Duration::from_millis(700)); }
                    let Ok(mut stream) = acceptor.accept(tcp) else { continue; };
                    let mut head = Vec::new();
                    loop {
                        let mut byte = [0];
                        if stream.read_exact(&mut byte).is_err() { break; }
                        head.push(byte[0]);
                        assert!(head.len() <= 32 * 1024);
                        if head.ends_with(b"\r\n\r\n") { break; }
                    }
                    if !head.ends_with(b"\r\n\r\n") { continue; }
                    let head = String::from_utf8(head).unwrap();
                    assert!(head.starts_with("POST / HTTP/1.1\r\n"));
                    assert!(head.lines().any(|line| line == format!("Authorization: Bearer {TOKEN}")));
                    let length: usize = head.lines().find_map(|line| line.strip_prefix("Content-Length: ")).unwrap().parse().unwrap();
                    let mut body = vec![0; length];
                    if stream.read_exact(&mut body).is_err() { continue; }
                    let request: Value = serde_json::from_slice(&body).unwrap();
                    requests.lock().unwrap().push((index, request.clone()));
                    if refuse {
                        let _ = stream.write_all(b"HTTP/1.1 503 Unavailable\r\nContent-Type: application/json\r\nContent-Length: 0\r\n\r\n");
                        let _ = stream.flush();
                        continue;
                    }
                    let result = match request["method"].as_str().unwrap() {
                        "eth_chainId" => document["chainId"].clone(),
                        "eth_getBlockByNumber" => {
                            assert_eq!(request["params"][1], false);
                            if request["params"][0] == "0x0" { document["genesis"].clone() }
                            else {
                                let state = state.lock().unwrap();
                                let height = if request["params"][0] == "finalized" { state.head }
                                    else { u64::from_str_radix(request["params"][0].as_str().unwrap().strip_prefix("0x").unwrap(), 16).unwrap() };
                                if height > state.head { Value::Null }
                                else { block(&document, height, state.wrong_parent) }
                            }
                        }
                        "pax_sequencerReceipt" => document["receipts"].as_array().unwrap().iter()
                            .find(|receipt| receipt["txHash"] == request["params"][0]).unwrap().clone(),
                        _ => panic!("unexpected method"),
                    };
                    let response = serde_json::to_vec(&json!({"jsonrpc":"2.0", "id":request["id"], "result":result})).unwrap();
                    if write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).is_err() { continue; }
                    if drip {
                        for byte in &response {
                            if stream.write_all(&[*byte]).and_then(|()| stream.flush()).is_err() { break; }
                            thread::sleep(Duration::from_millis(40));
                        }
                    } else {
                        let _ = stream.write_all(&response);
                        let _ = stream.flush();
                    }
                    let _ = stream.shutdown();
                }
            }));
            }
            let document = fixture();
            let key = &document["receiptKey"];
            Self {
                config: CommittedSourceConfig {
                    rpc: RpcQuorumConfig {
                        endpoints,
                        quorum: 2,
                        connect_timeout_ms: 1000,
                        request_timeout_ms: 2000,
                        maximum_response_bytes: 1024 * 1024,
                    },
                    chain_id: 125,
                    genesis_hash_hex: document["genesis"]["hash"]
                        .as_str()
                        .unwrap_or_else(|| panic!("genesis hash"))
                        .to_owned(),
                    receipt_keys: vec![ReceiptKeyConfig {
                        sequencer_id_hex: key["keyId"]
                            .as_str()
                            .unwrap_or_else(|| panic!("key ID"))
                            .to_owned(),
                        public_key_hex: key["publicKey"]
                            .as_str()
                            .unwrap_or_else(|| panic!("public key"))
                            .to_owned(),
                        first_height: key["firstHeight"]
                            .as_u64()
                            .unwrap_or_else(|| panic!("first height")),
                        last_height: key["lastHeight"]
                            .as_u64()
                            .unwrap_or_else(|| panic!("last height")),
                    }],
                },
                requests,
                state,
                running,
                addresses,
                threads,
                directory,
            }
        }
    }

    impl Drop for TestRpc {
        fn drop(&mut self) {
            self.running.store(false, Ordering::Release);
            for address in &self.addresses {
                let _ = TcpStream::connect(address);
            }
            let failed = self
                .threads
                .drain(..)
                .map(|thread| thread.join().is_err())
                .fold(false, |failed, next| failed | next);
            fs::remove_dir_all(&self.directory).unwrap_or_else(|e| panic!("test cleanup: {e}"));
            assert!(!failed, "TLS JSON-RPC emulator failed");
        }
    }

    pub fn block(document: &Value, height: u64, wrong_parent: bool) -> Value {
        if height == 1000 {
            return document["committedBlock"].clone();
        }
        let mut block = document["committedBlock"].clone();
        block["number"] = json!(format!("0x{height:x}"));
        block["hash"] = json!(block_hash(height));
        block["parentHash"] = json!(if wrong_parent {
            block_hash(0)
        } else {
            block_hash(height - 1)
        });
        block["transactions"] = json!([]);
        block
    }
    pub fn block_hash(height: u64) -> String {
        if height == 1000 {
            return fixture()["committedBlock"]["hash"]
                .as_str()
                .unwrap()
                .to_owned();
        }
        format!("0x{height:064x}")
    }
}

use std::fs;
use std::net::{SocketAddr, ToSocketAddrs};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};
use std::time::{Duration, Instant};

use layerx_mirror::committed::{CommittedBlockSource, CommittedCoordinate, CommittedRefusal};
use layerx_mirror::rpc::{RpcCluster, RpcResolver};
use layerx_mirror::runtime::{
    lane_archives, CommittedAcquisition, RuntimeConfig, LANE_RECHECK_WINDOW, MAX_ACQUIRED_PER_POLL,
    MAX_LANE_ARCHIVES_PER_POLL,
};
use layerx_mirror::store::{ArchiveSpool, MirrorChain};
use layerx_mirror::{Archive, ArchiveData, MirrorCursor, NodeHead};
use serde_json::{json, Value};

fn directory(rpc: &peer::TestRpc) -> std::path::PathBuf {
    rpc.config.rpc.endpoints[0]
        .ca_certificate_der
        .parent()
        .unwrap()
        .to_owned()
}

fn spooled(spool: &ArchiveSpool, height: u64) -> ArchiveData {
    let entries = spool.ordered_range(height, 1).unwrap();
    assert_eq!(entries[0].0, height);
    let stored = spool.get(entries[0].1).unwrap();
    ArchiveData::decode(&stored.bytes).unwrap()
}

#[test]
fn finding_22_source_coordinates_pin_identity_and_enforce_previous_continuity() {
    let rpc = peer::TestRpc::new();
    let source = CommittedBlockSource::open(&rpc.config).unwrap();
    let head: CommittedCoordinate = source.latest_committed(None).unwrap();
    assert_eq!(head.height, 1000);
    let previous = CommittedCoordinate {
        height: 999,
        block_hash: peer::bytes(&peer::fixture()["committedBlock"]["parentHash"])
            .try_into()
            .unwrap(),
    };
    let block = source.acquire_next(previous).unwrap();
    assert_eq!(block.header.height, head.height);
    assert_eq!(block.header.block_hash, head.block_hash);
    assert_eq!(block.receipts.len(), 2);
    assert_eq!(
        source.acquire_next(CommittedCoordinate {
            block_hash: [9; 32],
            ..previous
        }),
        Err(CommittedRefusal::ParentMismatch)
    );
    assert_eq!(
        source.latest_committed(Some(CommittedCoordinate {
            height: 1001,
            ..head
        })),
        Err(CommittedRefusal::HeightRegressed)
    );
    let mut foreign = rpc.config.clone();
    foreign.chain_id = 1;
    assert!(matches!(
        CommittedBlockSource::open(&foreign),
        Err(CommittedRefusal::ChainMismatch)
    ));
    foreign = rpc.config.clone();
    foreign.genesis_hash_hex = "55".repeat(32);
    assert!(matches!(
        CommittedBlockSource::open(&foreign),
        Err(CommittedRefusal::GenesisMismatch)
    ));
}

#[test]
fn finding_23_source_archive_codec_and_receipt_consumer_preserve_all_inputs() {
    use layerx_mirror::source::{
        MirrorCanonicalPosition, MirrorObservation, MirrorSourceId, MirrorTargetIdentity,
    };
    use layerx_mirror::{MirrorEvidenceLevel, MirrorVerifier};
    use layerx_proof::inclusion::SequencerAuthorization;
    let rpc = peer::TestRpc::new();
    let source = CommittedBlockSource::open(&rpc.config).unwrap();
    let block = source.acquire(1000).unwrap();
    let head = NodeHead {
        latest_sealed_batch: 1005,
        latest_committed: Some(CommittedCoordinate {
            height: 1005,
            block_hash: [5; 32],
        }),
    };
    let archive = Archive::from_committed(&block, 125, head).unwrap();
    assert_eq!(archive.node_head(), head);
    let path = directory(&rpc).join("archive");
    let spool = ArchiveSpool::open(&path).unwrap();
    spool
        .put(archive.commitment(), archive.bytes(), archive.node_head())
        .unwrap();
    drop(spool);
    let spool = ArchiveSpool::open(&path).unwrap();
    let stored = spool.get(archive.commitment()).unwrap();
    assert_eq!(stored.node_head, head);
    assert_eq!(stored.bytes, archive.bytes());
    let decoded = ArchiveData::decode(&stored.bytes).unwrap();
    assert_eq!(decoded.network_id, 125);
    assert_eq!(decoded.batch_number, 1000);
    assert_eq!(decoded.committed.receipts, block.receipts);
    let observation = MirrorObservation::from_archive(
        MirrorSourceId::new("recorded-archive-consumer").unwrap(),
        MirrorTargetIdentity::Ethereum {
            chain_id: 1,
            genesis_hash: [1; 32],
            contract: [2; 20],
            code_hash: [3; 32],
            publisher: [4; 20],
        },
        &stored.bytes,
        125,
        1000,
        archive.commitment(),
        MirrorCanonicalPosition::Ethereum {
            block_number: 10,
            block_hash: [5; 32],
            reference_head_number: 12,
            reference_head_hash: [6; 32],
        },
    )
    .unwrap();
    let verifier = MirrorVerifier::admit_committed(&stored.bytes, observation).unwrap();
    assert_eq!(
        verifier.committed().unwrap().block_hash,
        block.header.block_hash
    );
    let key = &rpc.config.receipt_keys[0];
    let keys = [SequencerAuthorization::new(
        peer::bytes(&json!(format!(
            "0x{}",
            key.sequencer_id_hex.trim_start_matches("0x")
        )))
        .try_into()
        .unwrap(),
        peer::bytes(&json!(format!(
            "0x{}",
            key.public_key_hex.trim_start_matches("0x")
        )))
        .try_into()
        .unwrap(),
        key.first_height,
        key.last_height,
    )];
    for (index, receipt) in block.receipts.iter().enumerate() {
        let verified = verifier.committed_receipt(index, &keys).unwrap();
        assert_eq!(verified.level(), MirrorEvidenceLevel::Committed);
        assert_eq!(
            layerx_wire::receipt::encode(verified.value()).unwrap(),
            *receipt
        );
    }
    assert!(verifier.committed_receipt(0, &[]).is_err());
    assert!(Archive::from_committed(
        &block,
        125,
        NodeHead {
            latest_sealed_batch: 999,
            latest_committed: None
        }
    )
    .is_err());
    assert!(Archive::from_committed(
        &block,
        125,
        NodeHead {
            latest_sealed_batch: 1000,
            latest_committed: head.latest_committed
        }
    )
    .is_err());
}

#[test]
fn finding_13_slow_drip_and_tls_share_a_finite_budget_without_losing_quorum() {
    for delayed_tls in [false, true] {
        let mut rpc = peer::TestRpc::new();
        rpc.config.rpc.request_timeout_ms = 250;
        {
            let mut state = rpc.state.lock().unwrap();
            state.drip = !delayed_tls;
            state.handshake_delay = delayed_tls;
        }
        let cluster = RpcCluster::new(&rpc.config.rpc).unwrap();
        let start = Instant::now();
        assert_eq!(
            cluster.call("eth_chainId", json!([])).unwrap(),
            json!("0x7d")
        );
        assert!(
            start.elapsed() < Duration::from_millis(650),
            "one slow endpoint exhausted the request budget: {:?}",
            start.elapsed()
        );
        let requests = rpc.requests.lock().unwrap();
        assert!(requests.iter().any(|(index, _)| *index == 1));
        assert!(requests.iter().any(|(index, _)| *index == 2));
    }
}

struct DelayedResolution {
    port: u16,
    delay: Duration,
    delayed_lookups: AtomicUsize,
}
impl RpcResolver for DelayedResolution {
    fn resolve(&self, host: &str, port: u16) -> std::io::Result<Vec<SocketAddr>> {
        if port == self.port {
            self.delayed_lookups.fetch_add(1, Ordering::Relaxed);
            std::thread::sleep(self.delay);
        }
        (host, port)
            .to_socket_addrs()
            .map(|addresses| addresses.collect())
    }
}

fn first_port(rpc: &peer::TestRpc) -> u16 {
    rpc.config.rpc.endpoints[0]
        .url
        .trim_start_matches("https://")
        .trim_end_matches('/')
        .parse::<SocketAddr>()
        .unwrap()
        .port()
}

#[test]
fn finding_13_resolution_timeout_is_bounded_and_outstanding_lookups_are_limited() {
    let mut rpc = peer::TestRpc::new();
    rpc.config.rpc.request_timeout_ms = 150;
    let resolver = Arc::new(DelayedResolution {
        port: first_port(&rpc),
        delay: Duration::from_millis(1500),
        delayed_lookups: AtomicUsize::new(0),
    });
    let cluster = RpcCluster::with_resolver(&rpc.config.rpc, resolver.clone()).unwrap();
    for _ in 0..2 {
        let start = Instant::now();
        assert_eq!(
            cluster.call("eth_chainId", json!([])).unwrap(),
            json!("0x7d")
        );
        assert!(start.elapsed() < Duration::from_millis(450));
    }
    assert_eq!(
        resolver.delayed_lookups.load(Ordering::Relaxed),
        1,
        "timed-out lookups cannot accumulate threads"
    );
}

#[test]
fn finding_13_resolution_and_body_do_not_receive_separate_deadlines() {
    let mut rpc = peer::TestRpc::new();
    rpc.config.rpc.request_timeout_ms = 350;
    rpc.state.lock().unwrap().drip = true;
    let resolver = Arc::new(DelayedResolution {
        port: first_port(&rpc),
        delay: Duration::from_millis(230),
        delayed_lookups: AtomicUsize::new(0),
    });
    let cluster = RpcCluster::with_resolver(&rpc.config.rpc, resolver.clone()).unwrap();
    let start = Instant::now();
    assert_eq!(
        cluster.call("eth_chainId", json!([])).unwrap(),
        json!("0x7d")
    );
    assert!(
        start.elapsed() < Duration::from_millis(530),
        "DNS plus body must share the same 350ms deadline: {:?}",
        start.elapsed()
    );
    assert_eq!(resolver.delayed_lookups.load(Ordering::Relaxed), 1);
    assert!(
        rpc.requests
            .lock()
            .unwrap()
            .iter()
            .any(|(index, _)| *index == 0),
        "resolution completed and the slow body was reached"
    );
}

#[test]
fn finding_10_and_29_tip_advancement_backlog_and_refusal_have_distinct_status() {
    let rpc = peer::TestRpc::new();
    let path = directory(&rpc).join("acquisition");
    let spool = ArchiveSpool::open(&path).unwrap();
    let mut worker = CommittedAcquisition::open(&rpc.config, spool.clone(), 1000, 4).unwrap();
    assert_eq!(worker.poll().unwrap(), 1);
    let status = worker.status_json().unwrap();
    assert_eq!(status["status_revision"], 2);
    assert_eq!(status["native_network_id"], 125);
    assert_eq!(status["source_chain_id"], 125);
    assert!(status.get("native_protocol_version").is_none());
    assert!(status.get("checkpoint_freshness_budget_batches").is_none());
    assert_eq!(status["node"]["phase"], "caught_up");
    assert_eq!(status["node"]["ready"], true);
    assert_eq!(status["committed_freshness_budget_blocks"], 4);
    assert_eq!(worker.poll().unwrap(), 0);
    assert_eq!(worker.status_json().unwrap()["node"]["ready"], true);
    assert!(
        !rpc.requests
            .lock()
            .unwrap()
            .iter()
            .any(|(_, request)| request["params"][0] == "0x3e9"),
        "a steady tip must not probe an unavailable next height"
    );
    rpc.state.lock().unwrap().head = 1001;
    assert_eq!(worker.poll().unwrap(), 1);
    assert_eq!(
        worker.status_json().unwrap()["latest_committed_height"],
        1001
    );
    rpc.state.lock().unwrap().head = 1040;
    assert_eq!(worker.poll().unwrap(), MAX_ACQUIRED_PER_POLL);
    let status = worker.status_json().unwrap();
    assert_eq!(status["node"]["phase"], "backlog");
    assert_eq!(status["node"]["latest_height_mirrored"], 1017);
    assert_eq!(status["node"]["latest_height_observed"], 1040);
    assert_eq!(status["latest_committed_height"], 1040);
    assert_eq!(status["ethereum"]["latest_committed_height_verified"], 1040);
    assert_eq!(status["ethereum"]["committed_block_lag"], 1040);
    assert_eq!(status["ethereum"]["committed_within_budget"], false);
    assert!(status["ethereum"].get("checkpoint_batch_lag").is_none());
    rpc.state.lock().unwrap().refuse = true;
    assert!(worker.poll().is_err());
    let refused = worker.status_json().unwrap();
    assert_eq!(refused["node"]["ready"], false);
    assert_eq!(refused["committed_ready"], false);
    assert!(refused["committed_refusal"].is_string());
    assert_eq!(refused["latest_committed_height"], 1040);
    assert_eq!(spooled(&spool, 1017).batch_number, 1017);
    let commitment = spool.ordered_range(1017, 1).unwrap()[0].1;
    assert_eq!(
        spool.get(commitment).unwrap().node_head.latest_sealed_batch,
        1040
    );
    rpc.state.lock().unwrap().refuse = false;
    assert_eq!(worker.poll().unwrap(), MAX_ACQUIRED_PER_POLL);
    assert_eq!(worker.poll().unwrap(), 7);
    assert_eq!(worker.status_json().unwrap()["node"]["phase"], "caught_up");
    assert_eq!(worker.status_json().unwrap()["node"]["ready"], true);
}

#[test]
fn finding_11_ordered_spool_and_lane_cursors_recover_without_reloading_history_per_poll() {
    let rpc = peer::TestRpc::new();
    rpc.state.lock().unwrap().head = 1090;
    let path = directory(&rpc).join("indexed");
    let spool = ArchiveSpool::open(&path).unwrap();
    let mut worker = CommittedAcquisition::open(&rpc.config, spool.clone(), 1000, 4).unwrap();
    for _ in 0..6 {
        assert!(worker.poll().unwrap() <= MAX_ACQUIRED_PER_POLL);
    }
    assert_eq!(spool.ordered_bounds().unwrap(), Some((1000, 1090, 91)));
    let ethereum = spooled(&spool, 1080).committed_coordinate().unwrap();
    let solana = spooled(&spool, 1050).committed_coordinate().unwrap();
    spool
        .save_lane_cursor(MirrorChain::Ethereum, ethereum)
        .unwrap();
    spool.save_lane_cursor(MirrorChain::Solana, solana).unwrap();
    let expected = spool.ordered_range(1080, 1).unwrap()[0].1;
    let retained = spool.get(expected).unwrap().bytes;
    drop(worker);
    drop(spool);
    let spool = ArchiveSpool::open(&path).unwrap();
    assert_eq!(
        spool.lane_cursor(MirrorChain::Ethereum).unwrap(),
        Some(ethereum)
    );
    assert_eq!(
        spool.lane_cursor(MirrorChain::Solana).unwrap(),
        Some(solana)
    );
    assert_eq!(spool.get(expected).unwrap().bytes, retained);
    // A damaged archive outside both bounded windows must not be read by a poll.
    // Restore the retained object before reopening (startup authenticates history).
    let old = spool.ordered_range(1000, 1).unwrap()[0].1;
    let name = old
        .as_bytes()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    let old_path = path.join(format!("{name}.archive"));
    let old_bytes = fs::read(&old_path).unwrap();
    fs::write(&old_path, b"outside the active windows").unwrap();
    for (lane, coordinate) in [
        (MirrorChain::Ethereum, ethereum),
        (MirrorChain::Solana, solana),
    ] {
        let cursor = MirrorCursor {
            latest_batch: Some(coordinate.height),
            latest_committed: Some(coordinate),
        };
        let archives = lane_archives(&spool, lane, 1000, cursor)
            .into_iter()
            .collect::<Result<Vec<_>, _>>()
            .unwrap();
        assert!(archives.len() <= MAX_LANE_ARCHIVES_PER_POLL);
        assert_eq!(
            archives[0].data().batch_number,
            coordinate.height - LANE_RECHECK_WINDOW + 1
        );
        assert!(archives
            .iter()
            .any(|archive| archive.data().committed_coordinate() == Some(coordinate)));
        assert_eq!(spool.lane_cursor(lane).unwrap(), Some(coordinate));
    }
    fs::write(&old_path, &old_bytes).unwrap();
    assert_eq!(fs::read(&old_path).unwrap(), old_bytes);
    // Recovery after the archive was synced but its index entry was not written.
    fs::remove_file(path.join(".order").join(format!("{:020}", 1090))).unwrap();
    drop(spool);
    let spool = ArchiveSpool::open(&path).unwrap();
    let mut restarted = CommittedAcquisition::open(&rpc.config, spool.clone(), 1000, 4).unwrap();
    assert_eq!(restarted.poll().unwrap(), 0);
    assert_eq!(
        restarted.status_json().unwrap()["node"]["latest_height_mirrored"],
        1090
    );
    assert!(path.join(".order").join(format!("{:020}", 1090)).is_file());
    assert_eq!(
        spool.commitments().unwrap().len(),
        91,
        "history is retained"
    );
    rpc.state.lock().unwrap().head = 1091;
    rpc.state.lock().unwrap().wrong_parent = true;
    assert_eq!(restarted.poll(), Err(CommittedRefusal::ParentMismatch));
    assert_eq!(spool.ordered_bounds().unwrap(), Some((1000, 1090, 91)));
}

fn rendered_config(rpc: &peer::TestRpc) -> Value {
    let mut value: Value =
        serde_json::from_str(include_str!("../../../deploy/mirror/config.example.json")).unwrap();
    let mut endpoints = Vec::new();
    for endpoint in &rpc.config.rpc.endpoints {
        endpoints.push(json!({ "url":endpoint.url,"ca_certificate_der":endpoint.ca_certificate_der,"bearer_token_file":endpoint.bearer_token_file,"independent_backend":endpoint.independent_backend }));
    }
    value["paxeer"] = json!({
        "rpc": {"endpoints":endpoints,"quorum":2,"connect_timeout_ms":1000,"request_timeout_ms":2000,"maximum_response_bytes":1048576},
        "chain_id":rpc.config.chain_id,"genesis_hash_hex":rpc.config.genesis_hash_hex,
        "receipt_keys":rpc.config.receipt_keys.iter().map(|key|json!({"sequencer_id_hex":key.sequencer_id_hex,"public_key_hex":key.public_key_hex,"first_height":key.first_height,"last_height":key.last_height})).collect::<Vec<_>>()
    });
    value
}

#[test]
fn finding_25_and_21_rendered_native_configuration_loads_at_root_with_strict_key_windows() {
    let rpc = peer::TestRpc::new();
    let config = rendered_config(&rpc);
    let path = directory(&rpc).join("rendered.json");
    fs::write(&path, serde_json::to_vec(&config).unwrap()).unwrap();
    let loaded = RuntimeConfig::load(&path).unwrap();
    assert_eq!(loaded.paxeer, rpc.config);
    assert_eq!(loaded.committed_network_id().unwrap(), 125);
    assert!(loaded
        .paxeer
        .rpc
        .endpoints
        .iter()
        .all(|endpoint| endpoint.url.ends_with('/')));
    assert_eq!(
        CommittedBlockSource::open(&loaded.paxeer)
            .unwrap()
            .acquire(1000)
            .unwrap()
            .header
            .height,
        1000
    );
    let original: Value =
        serde_json::from_str(include_str!("../../../deploy/mirror/config.example.json")).unwrap();
    assert!(original["paxeer"]["rpc"]["endpoints"]
        .as_array()
        .unwrap()
        .iter()
        .all(|endpoint| endpoint["url"].as_str().unwrap().ends_with('/')));
    assert_eq!(config["ethereum"], original["ethereum"]);
    assert_eq!(config["solana"], original["solana"]);
    for removed in [
        "node",
        "node_endpoint",
        "expected_protocol_version",
        "sequencer_authority_source",
        "first_batch_number",
        "checkpoint_freshness_budget_batches",
    ] {
        let mut legacy = config.clone();
        legacy[removed] = json!("retired");
        fs::write(&path, serde_json::to_vec(&legacy).unwrap()).unwrap();
        assert!(
            RuntimeConfig::load(&path).is_err(),
            "removed key accepted: {removed}"
        );
    }
    let mut legacy = config.clone();
    legacy["paxeer"]["endpoints"] = legacy["paxeer"]["rpc"]["endpoints"].clone();
    fs::write(&path, serde_json::to_vec(&legacy).unwrap()).unwrap();
    assert!(RuntimeConfig::load(&path).is_err());
    let mut outside_window = loaded.paxeer.clone();
    outside_window.receipt_keys[0].first_height = 0;
    outside_window.receipt_keys[0].last_height = 999;
    assert_eq!(
        CommittedBlockSource::open(&outside_window)
            .unwrap()
            .acquire(1000),
        Err(CommittedRefusal::ReceiptSignature)
    );
    let mut inverted_window = loaded.paxeer.clone();
    inverted_window.receipt_keys[0].first_height = inverted_window.receipt_keys[0].last_height + 1;
    assert!(matches!(
        CommittedBlockSource::open(&inverted_window),
        Err(CommittedRefusal::ReceiptSignature)
    ));
}

mod publication {
    // Reuse the committed chain and signer peers: signed transactions from the
    // concrete publisher populate the peer's manifest/chunk store unchanged.
    include!("repair_22_31_166.rs");

    #[test]
    fn finding_11_real_publication_acknowledges_and_recovers_each_lane_cursor() {
        use layerx_mirror::runtime::{lane_archives, save_lane_progress};
        use layerx_mirror::store::ArchiveSpool;
        use layerx_mirror::ProgressGate;
        for lane in [MirrorChain::Ethereum, MirrorChain::Solana] {
            let fixture = Fixture::new(lane);
            let archive = archive();
            let spool_path = fixture.directory.0.join("spool");
            let spool = ArchiveSpool::open(&spool_path).unwrap();
            assert_eq!(spool.ordered_bounds().unwrap(), None);
            spool
                .put(archive.commitment(), archive.bytes(), archive.node_head())
                .unwrap();
            let journal_path = fixture.directory.0.join("lane");
            let mut client = fixture.open(&journal_path);
            let mut complete = false;
            for _ in 0..16 {
                if client.advance(&archive).phase == PublicationPhase::RetrievedVerified {
                    complete = true;
                    break;
                }
            }
            assert!(
                complete,
                "the real publisher must finish the bounded fixture"
            );
            let cursor = match &client {
                Client::Ethereum(client) => client.cursor(),
                Client::Solana(client) => client.cursor(),
            };
            let archives = lane_archives(&spool, lane, 1, cursor)
                .into_iter()
                .collect::<Result<Vec<_>, _>>()
                .unwrap();
            let mut gate = ProgressGate::new();
            let progress = gate.acknowledge(cursor, &archives, |commitment| {
                assert_eq!(commitment, archive.commitment());
                Ok::<_, ()>(Some(client.retrieve(&archive)))
            });
            assert!(progress.refusal.is_none());
            save_lane_progress(&spool, lane, &progress).unwrap();
            let acknowledged = cursor.latest_committed.unwrap();
            assert_eq!(spool.lane_cursor(lane).unwrap(), Some(acknowledged));
            drop(client);
            drop(spool);
            let spool = ArchiveSpool::open(&spool_path).unwrap();
            assert_eq!(spool.lane_cursor(lane).unwrap(), Some(acknowledged));
            let client = fixture.open(&journal_path);
            let journal = match &client {
                Client::Ethereum(client) => client.cursor(),
                Client::Solana(client) => client.cursor(),
            };
            assert_eq!(journal, cursor);
            let archives = lane_archives(&spool, lane, 1, journal)
                .into_iter()
                .collect::<Result<Vec<_>, _>>()
                .unwrap();
            let mut restarted_gate = ProgressGate::new();
            let recovered = restarted_gate.acknowledge(journal, &archives, |_| {
                Ok::<_, ()>(Some(client.retrieve(&archive)))
            });
            assert!(recovered.refusal.is_none());
            assert_eq!(recovered.mirrored, cursor);
            assert_eq!(
                spool.get(archive.commitment()).unwrap().bytes,
                client.retrieve(&archive)
            );
        }
    }
}

#[test]
fn finding_11_index_and_cursor_damage_are_refused_without_discarding_retained_bytes() {
    let rpc = peer::TestRpc::new();
    let source = CommittedBlockSource::open(&rpc.config).unwrap();
    let block = source.acquire(1000).unwrap();
    let coordinate = CommittedCoordinate {
        height: 1000,
        block_hash: block.header.block_hash,
    };
    let head = NodeHead {
        latest_sealed_batch: 1000,
        latest_committed: Some(coordinate),
    };
    let archive = Archive::from_committed(&block, 125, head).unwrap();
    let path = directory(&rpc).join("corruption");
    let spool = ArchiveSpool::open(&path).unwrap();
    spool.ordered_bounds().unwrap();
    spool
        .put(archive.commitment(), archive.bytes(), head)
        .unwrap();
    spool
        .save_lane_cursor(MirrorChain::Ethereum, coordinate)
        .unwrap();
    let mut conflicting_block = block.clone();
    conflicting_block.header.timestamp += 1;
    let conflict = Archive::from_committed(&conflicting_block, 125, head).unwrap();
    assert!(spool
        .put(conflict.commitment(), conflict.bytes(), head)
        .is_err());
    assert_eq!(spool.commitments().unwrap(), vec![archive.commitment()]);
    drop(spool);
    let cursor_path = path.join(".ethereum.cursor");
    let mut damaged = fs::read(&cursor_path).unwrap();
    damaged[10] ^= 1;
    fs::write(&cursor_path, &damaged).unwrap();
    let spool = ArchiveSpool::open(&path).unwrap();
    assert!(spool.lane_cursor(MirrorChain::Ethereum).is_err());
    assert_eq!(fs::read(&cursor_path).unwrap(), damaged);
    assert_eq!(
        spool.get(archive.commitment()).unwrap().bytes,
        archive.bytes()
    );
    drop(spool);
    let index_path = path.join(".order").join(format!("{:020}", 1000));
    let mut index_bytes = fs::read(&index_path).unwrap();
    index_bytes[0] ^= 1;
    fs::write(&index_path, &index_bytes).unwrap();
    let spool = ArchiveSpool::open(&path).unwrap();
    assert!(spool.ordered_bounds().is_err());
    assert_eq!(fs::read(&index_path).unwrap(), index_bytes);
    assert_eq!(
        spool.get(archive.commitment()).unwrap().bytes,
        archive.bytes()
    );
}
