mod support {
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
        running: Arc<AtomicBool>,
        addresses: Vec<SocketAddr>,
        threads: Vec<JoinHandle<()>>,
        directory: PathBuf,
    }

    impl TestRpc {
        pub fn new(documents: Vec<Value>) -> Self {
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
                    url: format!("https://{address}/rpc"),
                    ca_certificate_der: ca_path.clone(),
                    bearer_token_file: token_path.clone(),
                    independent_backend: format!("committed-test-{index}"),
                });
                let acceptor = Arc::clone(&acceptor);
                let requests = Arc::clone(&requests);
                let running = Arc::clone(&running);
                threads.push(thread::spawn(move || {
                while running.load(Ordering::Acquire) {
                    let (tcp, _) = listener.accept().unwrap_or_else(|e| panic!("accept: {e}"));
                    if !running.load(Ordering::Acquire) { break; }
                    tcp.set_read_timeout(Some(Duration::from_secs(2))).unwrap_or_else(|e| panic!("timeout: {e}"));
                    tcp.set_write_timeout(Some(Duration::from_secs(2))).unwrap_or_else(|e| panic!("timeout: {e}"));
                    let mut stream = acceptor.accept(tcp).unwrap_or_else(|e| panic!("TLS accept: {e}"));
                    let mut head = Vec::new();
                    while !head.ends_with(b"\r\n\r\n") {
                        let mut byte = [0];
                        stream.read_exact(&mut byte).unwrap_or_else(|e| panic!("HTTP header: {e}"));
                        head.push(byte[0]);
                        assert!(head.len() <= 32 * 1024);
                    }
                    let head = String::from_utf8(head).unwrap_or_else(|e| panic!("HTTP: {e}"));
                    assert!(head.starts_with("POST /rpc HTTP/1.1\r\n"));
                    assert!(head.lines().any(|line| line == format!("Authorization: Bearer {TOKEN}")));
                    let length: usize = head.lines().find_map(|line| line.strip_prefix("Content-Length: "))
                        .unwrap_or_else(|| panic!("content length")).parse().unwrap_or_else(|e| panic!("length: {e}"));
                    assert!(length <= 512 * 1024);
                    let mut body = vec![0; length];
                    stream.read_exact(&mut body).unwrap_or_else(|e| panic!("HTTP body: {e}"));
                    let request: Value = serde_json::from_slice(&body).unwrap_or_else(|e| panic!("JSON-RPC: {e}"));
                    requests.lock().unwrap_or_else(|e| panic!("requests: {e}")).push((index, request.clone()));
                    let result = match request["method"].as_str() {
                        Some("eth_chainId") => { assert_eq!(request["params"], json!([])); document["chainId"].clone() }
                        Some("eth_getBlockByNumber") => {
                            assert_eq!(request["params"][1], false);
                            match request["params"][0].as_str() {
                                Some("0x0") => document["genesis"].clone(),
                                Some("0x8") => document["block8"].clone(),
                                Some("finalized") => document["finalized"].clone(),
                                _ => { assert_eq!(request["params"][0], document["committedBlock"]["number"]); document["committedBlock"].clone() }
                            }
                        }
                        Some("pax_sequencerReceipt") => document["receipts"].as_array().unwrap_or_else(|| panic!("receipts"))
                            .iter().find(|receipt| receipt["txHash"] == request["params"][0]).unwrap_or_else(|| panic!("requested transaction")).clone(),
                        _ => panic!("unexpected method"),
                    };
                    let response = serde_json::to_vec(&json!({"jsonrpc":"2.0", "id":request["id"], "result":result})).unwrap_or_else(|e| panic!("response: {e}"));
                    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).unwrap_or_else(|e| panic!("HTTP response: {e}"));
                    stream.write_all(&response).unwrap_or_else(|e| panic!("response: {e}"));
                    stream.flush().unwrap_or_else(|e| panic!("flush: {e}"));
                    stream
                        .shutdown()
                        .unwrap_or_else(|e| panic!("TLS close_notify: {e}"));
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
}

use layerx_mirror::committed::{
    CommittedBlockSource, CommittedCoordinate, CommittedHeader, CommittedRefusal,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use serde_json::{json, Value};

fn committed_height(document: &Value) -> u64 {
    u64::from_str_radix(
        document["committedBlock"]["number"]
            .as_str()
            .unwrap_or_else(|| panic!("block number"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("hex number")),
        16,
    )
    .unwrap_or_else(|error| panic!("height: {error}"))
}

#[test]
fn the_finalized_tag_gives_the_latest_committed_coordinate() {
    let document = support::fixture();
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    let source =
        CommittedBlockSource::open(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    let expected = CommittedCoordinate {
        height: 8,
        block_hash: [0x88; 32],
    };
    assert_eq!(source.latest_committed(None), Ok(expected));
    assert_eq!(
        source.latest_committed(Some(CommittedCoordinate {
            height: 7,
            block_hash: [0x77; 32],
        })),
        Ok(expected)
    );
    assert_eq!(source.latest_committed(Some(expected)), Ok(expected));
    assert_eq!(
        source.latest_committed(Some(CommittedCoordinate {
            height: 8,
            block_hash: [0xff; 32],
        })),
        Ok(expected)
    );
    assert_eq!(
        source.latest_committed(Some(CommittedCoordinate {
            height: 9,
            block_hash: [0; 32],
        })),
        Err(CommittedRefusal::HeightRegressed)
    );
    let requests = rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"));
    for endpoint in 0..2 {
        let calls: Vec<_> = requests
            .iter()
            .filter(|(index, _)| *index == endpoint)
            .map(|(_, request)| request)
            .collect();
        assert_eq!(calls.len(), 7);
        assert_eq!(calls[0]["method"], "eth_chainId");
        assert_eq!(calls[1]["params"], json!(["0x0", false]));
        for call in &calls[2..] {
            assert_eq!(call["method"], "eth_getBlockByNumber");
            assert_eq!(call["params"], json!(["finalized", false]));
        }
    }
    drop(requests);

    let mut unavailable = document.clone();
    unavailable["finalized"] = Value::Null;
    let rpc = support::TestRpc::new(vec![unavailable.clone(), unavailable]);
    let source =
        CommittedBlockSource::open(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(
        source.latest_committed(None),
        Err(CommittedRefusal::Unavailable)
    );

    let mut different = document.clone();
    different["finalized"]["hash"] = json!(format!("0x{}", "99".repeat(32)));
    let rpc = support::TestRpc::new(vec![document, different]);
    let source =
        CommittedBlockSource::open(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(
        source.latest_committed(None),
        Err(CommittedRefusal::Divergence)
    );
}

#[test]
fn a_parent_hash_break_is_refused_as_parent_mismatch() {
    let document = support::fixture();
    let previous = CommittedCoordinate {
        height: 7,
        block_hash: [0x77; 32],
    };
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    let source =
        CommittedBlockSource::open(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    let expected = CommittedHeader::from_block_json(&document["block8"])
        .unwrap_or_else(|error| panic!("block 8: {error:?}"));
    let next = source
        .acquire_next(previous)
        .unwrap_or_else(|error| panic!("next: {error:?}"));
    assert_eq!(next.header, expected);
    assert!(next.receipts.is_empty());
    let request_count = rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"))
        .len();
    assert_eq!(
        source.acquire_next(CommittedCoordinate {
            height: u64::MAX,
            block_hash: [0x77; 32]
        }),
        Err(CommittedRefusal::Malformed)
    );
    assert_eq!(
        rpc.requests
            .lock()
            .unwrap_or_else(|error| panic!("requests: {error}"))
            .len(),
        request_count
    );

    let mut broken = document;
    broken["block8"]["parentHash"] = json!(format!("0x{}", "ff".repeat(32)));
    let rpc = support::TestRpc::new(vec![broken.clone(), broken]);
    let source =
        CommittedBlockSource::open(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(
        source.acquire_next(previous),
        Err(CommittedRefusal::ParentMismatch)
    );
    let requests = rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"));
    for endpoint in 0..2 {
        let calls: Vec<_> = requests
            .iter()
            .filter(|(index, _)| *index == endpoint)
            .map(|(_, request)| request)
            .collect();
        assert_eq!(calls.len(), 3);
        assert_eq!(calls[2]["method"], "eth_getBlockByNumber");
        assert_eq!(calls[2]["params"], json!(["0x8", false]));
    }
}

#[test]
fn a_wrong_genesis_hash_is_refused_when_the_source_opens() {
    let document = support::fixture();
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    assert!(CommittedBlockSource::open(&rpc.config).is_ok());
    let mut wrong_chain = rpc.config.clone();
    wrong_chain.chain_id += 1;
    assert!(matches!(
        CommittedBlockSource::open(&wrong_chain),
        Err(CommittedRefusal::ChainMismatch)
    ));

    let mut wrong = document.clone();
    wrong["genesis"]["hash"] = json!(format!("0x{}", "ff".repeat(32)));
    let rpc = support::TestRpc::new(vec![wrong.clone(), wrong]);
    assert!(matches!(
        CommittedBlockSource::open(&rpc.config),
        Err(CommittedRefusal::GenesisMismatch)
    ));
    let requests = rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"));
    for endpoint in 0..2 {
        let calls: Vec<_> = requests
            .iter()
            .filter(|(index, _)| *index == endpoint)
            .map(|(_, request)| request)
            .collect();
        assert_eq!(calls.len(), 2);
        assert_eq!(calls[0]["method"], "eth_chainId");
        assert_eq!(calls[0]["params"], json!([]));
        assert_eq!(calls[1]["method"], "eth_getBlockByNumber");
        assert_eq!(calls[1]["params"], json!(["0x0", false]));
    }
    drop(requests);

    let mut wrong_height = document;
    wrong_height["genesis"]["number"] = json!("0x1");
    let rpc = support::TestRpc::new(vec![wrong_height.clone(), wrong_height]);
    assert!(matches!(
        CommittedBlockSource::open(&rpc.config),
        Err(CommittedRefusal::GenesisMismatch)
    ));
}

#[test]
fn a_committed_block_is_acquired_with_every_receipt_verified() {
    let document = support::fixture();
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    let source =
        CommittedBlockSource::new(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    let height = committed_height(&document);
    let block = source
        .acquire(height)
        .unwrap_or_else(|error| panic!("acquire: {error:?}"));
    assert_eq!(
        block.header,
        CommittedHeader::from_block_json(&document["committedBlock"])
            .unwrap_or_else(|error| panic!("header: {error:?}"))
    );
    let expected = document["receipts"]
        .as_array()
        .unwrap_or_else(|| panic!("receipts"));
    assert_eq!(block.receipts.len(), expected.len());
    let key = &rpc.config.receipt_keys[0];
    let pins = [SequencerAuthorization::new(
        support::bytes(&json!(key.sequencer_id_hex))
            .try_into()
            .unwrap_or_else(|_| panic!("key ID")),
        support::bytes(&json!(key.public_key_hex))
            .try_into()
            .unwrap_or_else(|_| panic!("public key")),
        key.first_height,
        key.last_height,
    )];
    for (index, (bytes, answer)) in block.receipts.iter().zip(expected).enumerate() {
        assert_eq!(*bytes, support::bytes(&answer["receipt"]));
        let receipt = verify_sequencer_signature(bytes, ReceiptKeys::HeightRanges(&pins))
            .unwrap_or_else(|error| panic!("verified receipt: {error:?}"));
        assert_eq!(
            receipt
                .protocol()
                .unwrap_or_else(|| panic!("protocol receipt"))
                .activity_id(),
            block.header.transactions[index]
        );
    }
    let requests = rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"));
    for endpoint in 0..2 {
        let calls: Vec<_> = requests
            .iter()
            .filter(|(index, _)| *index == endpoint)
            .map(|(_, request)| request)
            .collect();
        assert_eq!(calls.len(), 3 + expected.len());
        assert_eq!(calls[0]["method"], "eth_chainId");
        assert_eq!(calls[1]["params"], json!(["0x0", false]));
        assert_eq!(calls[2]["method"], "eth_getBlockByNumber");
        assert_eq!(calls[2]["params"], json!([format!("0x{height:x}"), false]));
        for (index, transaction) in document["committedBlock"]["transactions"]
            .as_array()
            .unwrap_or_else(|| panic!("transactions"))
            .iter()
            .enumerate()
        {
            assert_eq!(calls[index + 3]["method"], "pax_sequencerReceipt");
            assert_eq!(calls[index + 3]["params"], json!([transaction]));
        }
    }
    drop(requests);
    let mut config = rpc.config.clone();
    config.chain_id += 1;
    assert!(matches!(
        CommittedBlockSource::new(&config),
        Err(CommittedRefusal::ChainMismatch)
    ));
    config = rpc.config.clone();
    config.genesis_hash_hex = format!("0x{}", "ff".repeat(32));
    assert!(matches!(
        CommittedBlockSource::new(&config),
        Err(CommittedRefusal::GenesisMismatch)
    ));

    let mut different = document.clone();
    different["committedBlock"]["timestamp"] = json!("0x1");
    let divergent = support::TestRpc::new(vec![document.clone(), different]);
    let source = CommittedBlockSource::new(&divergent.config)
        .unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(source.acquire(height), Err(CommittedRefusal::Divergence));
    let mut different = document.clone();
    different["receipts"][1]["rung"] = json!("receipted");
    let divergent = support::TestRpc::new(vec![document, different]);
    let source = CommittedBlockSource::new(&divergent.config)
        .unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(source.acquire(height), Err(CommittedRefusal::Divergence));
}

#[test]
fn a_transaction_not_yet_committed_refuses_the_block() {
    let document = support::fixture();
    let height = committed_height(&document);
    for (field, value) in [
        ("rung", json!("submitted")),
        ("rung", json!("receipted")),
        ("blockNumber", json!(format!("0x{:x}", height + 1))),
        ("blockHash", json!(format!("0x{}", "ff".repeat(32)))),
        ("blockNumber", Value::Null),
        ("blockHash", Value::Null),
    ] {
        let mut changed = document.clone();
        changed["receipts"][1][field] = value;
        let rpc = support::TestRpc::new(vec![changed.clone(), changed]);
        let source = CommittedBlockSource::new(&rpc.config)
            .unwrap_or_else(|error| panic!("source: {error:?}"));
        assert_eq!(
            source.acquire(height),
            Err(CommittedRefusal::NotCommitted),
            "{field}"
        );
        let requests = rpc
            .requests
            .lock()
            .unwrap_or_else(|error| panic!("requests: {error}"));
        assert_eq!(
            requests
                .iter()
                .filter(|(_, request)| request["method"] == "pax_sequencerReceipt")
                .count(),
            4
        );
    }
}

#[test]
fn a_receipt_outside_every_key_window_refuses_the_block() {
    let document = support::fixture();
    let height = committed_height(&document);
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    for (first, last) in [(height + 1, height + 2), (0, 999)] {
        let mut config = rpc.config.clone();
        config.receipt_keys[0].first_height = first;
        config.receipt_keys[0].last_height = last;
        let source =
            CommittedBlockSource::new(&config).unwrap_or_else(|error| panic!("source: {error:?}"));
        assert_eq!(
            source.acquire(height),
            Err(CommittedRefusal::ReceiptSignature)
        );
    }
    let mut invalid = document.clone();
    let signed = support::bytes(&invalid["receipts"][1]["receipt"]);
    let mut altered = signed;
    let last = altered
        .last_mut()
        .unwrap_or_else(|| panic!("signed receipt"));
    *last ^= 1;
    invalid["receipts"][1]["receipt"] = json!(format!(
        "0x{}",
        altered
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    ));
    let rpc = support::TestRpc::new(vec![invalid.clone(), invalid]);
    let source =
        CommittedBlockSource::new(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(
        source.acquire(height),
        Err(CommittedRefusal::ReceiptSignature)
    );
}

#[test]
fn committed_refusal_codes_are_the_published_strings() {
    const CODES: [(CommittedRefusal, &str); 9] = [
        (
            CommittedRefusal::Unavailable,
            CommittedRefusal::Unavailable.code(),
        ),
        (
            CommittedRefusal::ChainMismatch,
            CommittedRefusal::ChainMismatch.code(),
        ),
        (
            CommittedRefusal::GenesisMismatch,
            CommittedRefusal::GenesisMismatch.code(),
        ),
        (
            CommittedRefusal::Divergence,
            CommittedRefusal::Divergence.code(),
        ),
        (
            CommittedRefusal::NotCommitted,
            CommittedRefusal::NotCommitted.code(),
        ),
        (
            CommittedRefusal::ReceiptSignature,
            CommittedRefusal::ReceiptSignature.code(),
        ),
        (
            CommittedRefusal::ParentMismatch,
            CommittedRefusal::ParentMismatch.code(),
        ),
        (
            CommittedRefusal::HeightRegressed,
            CommittedRefusal::HeightRegressed.code(),
        ),
        (
            CommittedRefusal::Malformed,
            CommittedRefusal::Malformed.code(),
        ),
    ];
    let expected = [
        "committed_unavailable",
        "committed_chain_mismatch",
        "committed_genesis_mismatch",
        "committed_divergence",
        "committed_not_committed",
        "committed_receipt_signature",
        "committed_parent_mismatch",
        "committed_height_regressed",
        "committed_malformed",
    ];
    for ((refusal, code), expected) in CODES.into_iter().zip(expected) {
        assert_eq!(code, expected, "{refusal:?}");
    }
}

#[test]
fn a_recorded_block_answer_decodes_to_a_committed_header() {
    let answer: Value = serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
        .unwrap_or_else(|error| panic!("recorded block JSON: {error}"));
    let block = &answer["result"];
    let header = CommittedHeader::from_block_json(block)
        .unwrap_or_else(|error| panic!("recorded block header: {error:?}"));
    assert_eq!(
        header,
        CommittedHeader {
            height: 42,
            block_hash: [0x11; 32],
            parent_hash: [0x22; 32],
            app_hash: [0x33; 32],
            timestamp: 1_700_000_000,
            transactions: vec![[0x44; 32], [0x55; 32]],
        }
    );
    let coordinate = CommittedCoordinate {
        height: header.height,
        block_hash: header.block_hash,
    };
    let copied = coordinate;
    assert_eq!(coordinate, copied);
    assert!(
        coordinate
            < CommittedCoordinate {
                height: 43,
                block_hash: [0; 32]
            }
    );
    assert!(
        coordinate
            < CommittedCoordinate {
                height: 42,
                block_hash: [0x12; 32]
            }
    );

    for field in [
        "number",
        "hash",
        "parentHash",
        "stateRoot",
        "timestamp",
        "transactions",
    ] {
        let mut missing = block.clone();
        missing
            .as_object_mut()
            .unwrap_or_else(|| panic!("block object"))
            .remove(field);
        assert_eq!(
            CommittedHeader::from_block_json(&missing),
            Err(CommittedRefusal::Malformed),
            "missing {field}"
        );
        for invalid in [Value::Null, json!(17), json!({}), json!(false)] {
            let mut malformed = block.clone();
            malformed[field] = invalid;
            assert_eq!(
                CommittedHeader::from_block_json(&malformed),
                Err(CommittedRefusal::Malformed),
                "invalid {field}"
            );
        }
    }

    let invalid_hashes = [
        "0x".to_owned(),
        format!("0x{}", "11".repeat(31)),
        format!("0x{}", "11".repeat(33)),
        "11".repeat(32),
        format!("0x{}gg", "11".repeat(31)),
        format!("0x{}é", "11".repeat(31)),
    ];
    for invalid in invalid_hashes {
        for field in ["hash", "parentHash", "stateRoot"] {
            let mut malformed = block.clone();
            malformed[field] = json!(invalid);
            assert_eq!(
                CommittedHeader::from_block_json(&malformed),
                Err(CommittedRefusal::Malformed),
                "invalid {field}: {invalid}"
            );
        }
        let mut malformed = block.clone();
        malformed["transactions"][1] = json!(invalid);
        assert_eq!(
            CommittedHeader::from_block_json(&malformed),
            Err(CommittedRefusal::Malformed),
            "invalid transaction hash: {invalid}"
        );
    }

    for invalid in [
        json!({"hash": block["transactions"][0]}),
        Value::Null,
        json!(17),
    ] {
        let mut malformed = block.clone();
        malformed["transactions"][1] = invalid;
        assert_eq!(
            CommittedHeader::from_block_json(&malformed),
            Err(CommittedRefusal::Malformed)
        );
    }

    for field in ["number", "timestamp"] {
        for invalid in [
            "",
            "0x",
            "42",
            "0x00",
            "0x-1",
            "0x+1",
            "0xgg",
            "0x10000000000000000",
        ] {
            let mut malformed = block.clone();
            malformed[field] = json!(invalid);
            assert_eq!(
                CommittedHeader::from_block_json(&malformed),
                Err(CommittedRefusal::Malformed),
                "invalid {field}: {invalid}"
            );
        }
        for (valid, expected) in [("0x0", 0), ("0xffffffffffffffff", u64::MAX)] {
            let mut boundary = block.clone();
            boundary[field] = json!(valid);
            let decoded = CommittedHeader::from_block_json(&boundary)
                .unwrap_or_else(|error| panic!("valid {field}: {error:?}"));
            assert_eq!(
                if field == "number" {
                    decoded.height
                } else {
                    decoded.timestamp
                },
                expected
            );
        }
    }

    for invalid in [Value::Null, json!([]), json!("block")] {
        assert_eq!(
            CommittedHeader::from_block_json(&invalid),
            Err(CommittedRefusal::Malformed)
        );
    }
    let mut empty = block.clone();
    empty["transactions"] = json!([]);
    let decoded = CommittedHeader::from_block_json(&empty)
        .unwrap_or_else(|error| panic!("empty block: {error:?}"));
    assert!(decoded.transactions.is_empty());
}
