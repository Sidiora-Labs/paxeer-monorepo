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
    serde_json::from_str(include_str!("../fixtures/committed_blocks.json"))
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
            let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|e| panic!("bind: {e}"));
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
                            if request["params"][0] == "0x0" { document["genesis"].clone() }
                            else { assert_eq!(request["params"][0], document["committedBlock"]["number"]); document["committedBlock"].clone() }
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
