include!("../src/main.rs");

use ed25519_dalek::Signer as _;
use std::io::{BufRead as _, BufReader};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::process::Command;
use std::sync::Mutex;

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn receipt_vectors() -> serde_json::Value {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .unwrap();
    must(serde_json::from_slice(&must(fs::read(root.join(
        "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json",
    )))))
}

fn vector(document: &serde_json::Value, id: &str) -> serde_json::Value {
    document["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|case| case["id"] == id)
        .unwrap()
        .clone()
}

fn hex_bytes(value: &serde_json::Value) -> Vec<u8> {
    must(hex::decode(
        value.as_str().unwrap().strip_prefix("0x").unwrap(),
    ))
}

fn base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut text = String::new();
    for chunk in bytes.chunks(3) {
        let a = chunk[0];
        let b = chunk.get(1).copied().unwrap_or(0);
        let c = chunk.get(2).copied().unwrap_or(0);
        text.push(char::from(ALPHABET[usize::from(a >> 2)]));
        text.push(char::from(ALPHABET[usize::from(((a & 3) << 4) | (b >> 4))]));
        text.push(if chunk.len() > 1 {
            char::from(ALPHABET[usize::from(((b & 15) << 2) | (c >> 6))])
        } else {
            '='
        });
        text.push(if chunk.len() > 2 {
            char::from(ALPHABET[usize::from(c & 63)])
        } else {
            '='
        });
    }
    text
}

struct ReceiptHarness {
    directory: PathBuf,
    config: Config,
    calls: Arc<Mutex<Vec<serde_json::Value>>>,
    stopped: Arc<AtomicBool>,
    server: Option<thread::JoinHandle<()>>,
}

impl ReceiptHarness {
    fn start(cases: &[serde_json::Value], pinned: &serde_json::Value) -> Self {
        let mut nonce = [0; 16];
        must(getrandom::fill(&mut nonce));
        let directory =
            std::env::temp_dir().join(format!("authority-chain-receipts-{}", hex::encode(&nonce)));
        must(fs::create_dir(&directory));
        must(fs::set_permissions(
            &directory,
            fs::Permissions::from_mode(0o700),
        ));
        let keys_file = directory.join("receipt-keys.json");
        let pin = &pinned["pinnedKey"];
        let keys = serde_json::json!([{
            "keyId": base64(&hex_bytes(&pin["keyId"])),
            "publicKey": base64(&hex_bytes(&pin["publicKey"])),
            "firstHeight": pin["firstHeight"], "lastHeight": pin["lastHeight"],
        }]);
        must(fs::write(&keys_file, keys.to_string()));
        must(fs::set_permissions(
            &keys_file,
            fs::Permissions::from_mode(0o600),
        ));

        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let address = must(listener.local_addr());
        let mut recorded = BTreeMap::new();
        for case in cases {
            let mut result = case["receiptJson"].clone();
            result["signingKey"] = case["pinnedKey"]["publicKey"].clone();
            recorded.insert(result["txHash"].as_str().unwrap().to_owned(), result);
        }
        let calls = Arc::new(Mutex::new(Vec::new()));
        let seen = Arc::clone(&calls);
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let server = thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("recorded chain listener: {error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(2))));
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                must(reader.read_line(&mut line));
                assert_eq!(line, "POST /evm HTTP/1.0\r\n");
                let mut length = None;
                loop {
                    line.clear();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
                let length = length.unwrap();
                assert!(length <= 1024);
                let mut bytes = vec![0; length];
                must(reader.read_exact(&mut bytes));
                let request: serde_json::Value = must(serde_json::from_slice(&bytes));
                assert_eq!(request["jsonrpc"], "2.0");
                assert_eq!(request["method"], "pax_sequencerReceipt");
                assert_eq!(request["params"].as_array().unwrap().len(), 1);
                let hash = request["params"][0].as_str().unwrap();
                let result = recorded
                    .get(hash)
                    .cloned()
                    .unwrap_or(serde_json::Value::Null);
                let answer =
                    serde_json::json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
                        .to_string();
                must(seen.lock()).push(request);
                let mut stream = reader.into_inner();
                must(write!(stream, "HTTP/1.0 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", answer.len(), answer));
            }
        });

        let key_file = directory.join("tls-key.der");
        let cert_file = directory.join("tls-cert.der");
        let output = must(
            Command::new("openssl")
                .args([
                    "req",
                    "-x509",
                    "-newkey",
                    "ed25519",
                    "-nodes",
                    "-subj",
                    "/CN=localhost",
                    "-keyout",
                    directory.join("tls-key.pem").to_str().unwrap(),
                    "-outform",
                    "DER",
                    "-out",
                    cert_file.to_str().unwrap(),
                ])
                .output(),
        );
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let output = must(
            Command::new("openssl")
                .args([
                    "pkcs8",
                    "-topk8",
                    "-nocrypt",
                    "-in",
                    directory.join("tls-key.pem").to_str().unwrap(),
                    "-outform",
                    "DER",
                    "-out",
                    key_file.to_str().unwrap(),
                ])
                .output(),
        );
        assert!(output.status.success());
        let _ = rustls::crypto::ring::default_provider().install_default();
        let tls = must(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(must(fs::read(cert_file)))],
                    must(PrivateKeyDer::try_from(must(fs::read(key_file)))),
                ),
        );
        let public_key: [u8; 32] = hex_bytes(&pin["publicKey"]).try_into().unwrap();
        let config = Config {
            listen: "127.0.0.1:0".parse().unwrap(),
            tls: Arc::new(tls),
            tokens: vec![Zeroizing::new("chain-receipts-test-token".to_owned())],
            human: None,
            replica_address: address,
            replica_host: address.to_string(),
            replica_token: Zeroizing::new("replica-test-token".to_owned()),
            evidence_read_token: None,
            replica_id: [1; 32],
            chain_receipts: must(ChainReceipts::load(
                &format!("http://{address}/evm"),
                &keys_file,
            )),
            lni_socket: directory.join("absent-lni.sock"),
            lni_gate: ConnectionGate::new(1),
            protocol_network_id: 125,
            network_id: "chain-receipts-test".to_owned(),
            wire_version: PROTOCOL_VERSION.to_string(),
            authorization: SequencerAuthorization::new(
                hex_bytes(&pin["keyId"]).try_into().unwrap(),
                public_key,
                pin["firstHeight"].as_u64().unwrap(),
                pin["lastHeight"].as_u64().unwrap(),
            ),
            sequencer_public_key: public_key,
            trust: None,
        };
        Self {
            directory,
            config,
            calls,
            stopped,
            server: Some(server),
        }
    }

    fn stop_chain(&mut self) {
        self.stopped.store(true, Ordering::Release);
        if let Some(server) = self.server.take() {
            server.join().unwrap();
        }
    }

    fn files(&self) -> Vec<PathBuf> {
        let mut files: Vec<_> = must(fs::read_dir(&self.directory))
            .map(|entry| must(entry).path())
            .collect();
        files.sort();
        files
    }
}

impl Drop for ReceiptHarness {
    fn drop(&mut self) {
        self.stop_chain();
        must(fs::remove_dir_all(&self.directory));
    }
}

#[test]
fn receipt_lookup_reads_the_sequencer_receipt() {
    let document = receipt_vectors();
    let case = vector(&document, "V1-pax-transfer");
    let mut harness = ReceiptHarness::start(&[case.clone()], &case);
    let hash: [u8; 32] = hex_bytes(&case["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    let expected = hex_bytes(&case["signedReceipt"]);
    match lookup_receipt(&harness.config, hash, false) {
        ReceiptSource::Found(bytes) => assert_eq!(bytes, expected),
        other => panic!("pinned signed receipt was not admitted: {other:?}"),
    }
    assert!(!harness.config.lni_socket.exists());
    assert!(
        matches!(lookup_receipt(&harness.config, hash, true), ReceiptSource::Found(bytes) if bytes == expected)
    );
    let missing = [0x42; 32];
    assert!(matches!(
        lookup_receipt(&harness.config, missing, false),
        ReceiptSource::Unknown
    ));
    let absent = by_activity(&harness.config, &hex::encode(&missing), false);
    assert_eq!(absent.status, 404);
    let absent: serde_json::Value = must(serde_json::from_slice(&absent.body));
    assert_eq!(absent["error"]["code"], "unknown_activity");
    let calls = must(harness.calls.lock());
    assert_eq!(calls.len(), 4);
    assert_eq!(
        calls[0]["params"],
        serde_json::json!([case["receiptJson"]["txHash"]])
    );
    drop(calls);
    harness.stop_chain();
    assert!(matches!(
        lookup_receipt(&harness.config, hash, false),
        ReceiptSource::Unavailable(_)
    ));
    let unavailable = by_activity(&harness.config, &hex::encode(&hash), false);
    assert_eq!(unavailable.status, 503);
    let unavailable: serde_json::Value = must(serde_json::from_slice(&unavailable.body));
    assert_eq!(unavailable["error"]["code"], "receipt_source_unavailable");
}

#[test]
fn receipt_signed_by_an_unpinned_key_is_refused() {
    let document = receipt_vectors();
    let pinned = vector(&document, "V1-pax-transfer");
    let mut unpinned = vector(&document, "V3-key-not-pinned");
    let outside_range = vector(&document, "V3-height-outside-key-range");
    let mut receipt = hex_bytes(&unpinned["signedReceipt"]);
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x8c; 32]);
    let unsigned = must(layerx_wire::receipt::encode_unsigned(&must(
        layerx_wire::receipt::decode(&receipt),
    )));
    let digest = must(layerx_wire::hash::receipt_digest(&unsigned));
    let signature = signer.sign(&digest).to_bytes();
    let signature_offset = receipt.len() - signature.len();
    receipt[signature_offset..].copy_from_slice(&signature);
    unpinned["receiptJson"]["receipt"] = serde_json::json!(format!("0x{}", hex::encode(&receipt)));
    unpinned["signedReceipt"] = unpinned["receiptJson"]["receipt"].clone();
    unpinned["pinnedKey"]["publicKey"] = serde_json::json!(format!(
        "0x{}",
        hex::encode(&signer.verifying_key().to_bytes())
    ));
    assert_ne!(
        unpinned["pinnedKey"]["publicKey"],
        pinned["pinnedKey"]["publicKey"]
    );
    let own_key = SequencerAuthorization::new(
        hex_bytes(&unpinned["receiptJson"]["keyId"])
            .try_into()
            .unwrap(),
        hex_bytes(&unpinned["pinnedKey"]["publicKey"])
            .try_into()
            .unwrap(),
        1,
        u64::MAX,
    );
    assert!(
        layerx_proof::receipt::verify_sequencer_signature(
            &receipt,
            layerx_proof::receipt::ReceiptKeys::HeightRanges(&[own_key]),
        )
        .is_ok(),
        "the refused receipt must carry a real valid signature"
    );
    let harness = ReceiptHarness::start(&[unpinned.clone(), outside_range.clone()], &pinned);
    let before = harness.files();
    let hash: [u8; 32] = hex_bytes(&unpinned["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    assert!(matches!(
        lookup_receipt(&harness.config, hash, false),
        ReceiptSource::KeyMismatch
    ));
    let refused = by_activity(&harness.config, &hex::encode(&hash), false);
    assert_eq!(refused.status, 502);
    let refused: serde_json::Value = must(serde_json::from_slice(&refused.body));
    assert_eq!(refused["error"]["code"], "sequencer_key_mismatch");
    assert!(matches!(
        lookup_receipt(&harness.config, hash, true),
        ReceiptSource::KeyMismatch
    ));
    let outside: [u8; 32] = hex_bytes(&outside_range["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    assert!(matches!(
        lookup_receipt(&harness.config, outside, false),
        ReceiptSource::KeyMismatch
    ));
    assert_eq!(
        harness.files(),
        before,
        "refused receipts must not be persisted"
    );
    assert_eq!(
        must(harness.calls.lock()).len(),
        4,
        "every lookup reads the chain and no refused receipt is cached"
    );
}
