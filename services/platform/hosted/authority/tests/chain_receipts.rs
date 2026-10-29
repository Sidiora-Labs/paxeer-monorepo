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
        Self::start_with_pins(cases, std::slice::from_ref(pinned))
    }

    fn start_with_pins(cases: &[serde_json::Value], pinned: &[serde_json::Value]) -> Self {
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
        let pin = &pinned[0]["pinnedKey"];
        let keys: Vec<_> = pinned
            .iter()
            .map(|case| {
                let pin = &case["pinnedKey"];
                serde_json::json!({
                    "keyId": base64(&hex_bytes(&pin["keyId"])),
                    "publicKey": base64(&hex_bytes(&pin["publicKey"])),
                    "firstHeight": pin["firstHeight"], "lastHeight": pin["lastHeight"],
                })
            })
            .collect();
        let keys = serde_json::json!(keys);
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

struct AuthorityProcess {
    child: std::process::Child,
    address: SocketAddr,
}

impl AuthorityProcess {
    fn start(harness: &ReceiptHarness) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        drop(listener);
        let replica = must(TcpListener::bind("127.0.0.1:0"));
        let replica_address = must(replica.local_addr());
        drop(replica);
        let token = harness.directory.join("authority-token");
        let replica_token = harness.directory.join("replica-token");
        for (path, value) in [
            (&token, "chain-receipts-test-token"),
            (&replica_token, "replica-test-token"),
        ] {
            must(fs::write(path, value));
            must(fs::set_permissions(path, fs::Permissions::from_mode(0o600)));
        }
        let log = must(fs::File::create(harness.directory.join("authority.log")));
        let child =
            must(
                Command::new(env!("CARGO_BIN_EXE_layerx-receipt-authority"))
                    .env_clear()
                    .envs(env::vars_os().filter(|(name, _)| {
                        !name.to_string_lossy().starts_with("LAYERX_AUTHORITY_")
                    }))
                    .env("LAYERX_AUTHORITY_LISTEN", address.to_string())
                    .env(
                        "LAYERX_AUTHORITY_TLS_CERT_DER",
                        harness.directory.join("tls-cert.der"),
                    )
                    .env(
                        "LAYERX_AUTHORITY_TLS_KEY_DER",
                        harness.directory.join("tls-key.der"),
                    )
                    .env("LAYERX_AUTHORITY_TOKEN_FILES", token)
                    .env(
                        "LAYERX_AUTHORITY_REPLICA_URL",
                        format!("http://{replica_address}"),
                    )
                    .env("LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE", replica_token)
                    .env("LAYERX_AUTHORITY_REPLICA_ID", hex::encode(&[1; 32]))
                    .env(
                        "LAYERX_AUTHORITY_CHAIN_RPC_URL",
                        format!("http://{}/evm", harness.config.replica_address),
                    )
                    .env(
                        "LAYERX_AUTHORITY_RECEIPT_KEYS_FILE",
                        harness.directory.join("receipt-keys.json"),
                    )
                    .env("LAYERX_AUTHORITY_LNI_SOCKET", &harness.config.lni_socket)
                    .env("LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID", "125")
                    .env("LAYERX_AUTHORITY_NETWORK_ID", "chain-receipts-test")
                    .env(
                        "LAYERX_AUTHORITY_SEQUENCER_ID",
                        hex::encode(&harness.config.authorization.sequencer_id()),
                    )
                    .env(
                        "LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY",
                        hex::encode(&harness.config.sequencer_public_key),
                    )
                    .env("LAYERX_AUTHORITY_FIRST_BATCH", "1")
                    .env("LAYERX_AUTHORITY_LAST_BATCH", u64::MAX.to_string())
                    .stdout(must(log.try_clone()))
                    .stderr(log)
                    .spawn(),
            );
        let mut process = Self { child, address };
        let deadline = Instant::now() + Duration::from_secs(5);
        loop {
            if TcpStream::connect(address).is_ok() {
                return process;
            }
            assert!(
                must(process.child.try_wait()).is_none(),
                "authority exited during startup"
            );
            assert!(Instant::now() < deadline, "authority did not start");
            thread::sleep(Duration::from_millis(10));
        }
    }

    fn receipt(&self, activity: &str) -> (u16, serde_json::Value) {
        let output = must(
            Command::new("curl")
                .args([
                    "--insecure",
                    "--silent",
                    "--show-error",
                    "--max-time",
                    "3",
                    "--header",
                    "Authorization: Bearer chain-receipts-test-token",
                    "--write-out",
                    "\n%{http_code}",
                ])
                .arg(format!(
                    "https://{}/v1/authorized-batches/by-activity/{activity}",
                    self.address
                ))
                .output(),
        );
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let text = must(String::from_utf8(output.stdout));
        let (body, status) = text.rsplit_once('\n').unwrap();
        (must(status.parse()), must(serde_json::from_str(body)))
    }
}

impl Drop for AuthorityProcess {
    fn drop(&mut self) {
        must(self.child.kill());
        must(self.child.wait());
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

#[test]
fn trust_snapshot_is_the_pinned_key_list() {
    use layerx_client::handover::SequencerHistory;
    use std::os::unix::net::UnixListener;

    let document = receipt_vectors();
    let template = vector(&document, "V1-pax-transfer");
    let original_bytes = hex_bytes(&template["signedReceipt"]);
    let decoded = must(layerx_wire::receipt::decode(&original_bytes));
    let original = decoded.protocol().unwrap();
    let mut cases = Vec::new();
    let mut pins = Vec::new();
    let mut expected_keys = Vec::new();
    let mut headers = Vec::new();
    let native_fixture: serde_json::Value = must(serde_json::from_str(include_str!(
        "fixtures/real-program-deploy-receipt.json"
    )));
    for (index, seed) in [0x51, 0x52, 0x53].into_iter().enumerate() {
        let signer = ed25519_dalek::SigningKey::from_bytes(&[seed; 32]);
        let public_key = signer.verifying_key().to_bytes();
        let key_id = [seed + 0x30; 32];
        let pin = serde_json::json!({"pinnedKey": {
            "keyId": format!("0x{}", hex::encode(&key_id)),
            "publicKey": format!("0x{}", hex::encode(&public_key)),
            "firstHeight": 0, "lastHeight": u64::MAX,
        }});
        if index < 2 {
            pins.push(pin.clone());
            expected_keys.push(SequencerAuthorization::new(key_id, public_key, 0, u64::MAX));
        }
        for (height_index, height) in [0, 1, 1000, u64::MAX].into_iter().enumerate() {
            let mut bytes = original_bytes.clone();
            let mut batch = [0; 32];
            batch[24..].copy_from_slice(&height.to_be_bytes());
            let activity = [0x31 + (index * 4 + height_index) as u8; 32];
            for (old, new) in [
                (original.batch_id(), batch),
                (original.context_hash(), key_id),
                (original.activity_id(), activity),
            ] {
                let offset = bytes.windows(32).position(|value| value == old).unwrap();
                bytes[offset..offset + 32].copy_from_slice(&new);
            }
            let unsigned = must(layerx_wire::receipt::encode_unsigned(&must(
                layerx_wire::receipt::decode(&bytes),
            )));
            let signature = signer.sign(&must(layerx_wire::hash::receipt_digest(&unsigned)));
            let offset = bytes.len() - 64;
            bytes[offset..].copy_from_slice(&signature.to_bytes());
            let mut case = template.clone();
            case["pinnedKey"] = pin["pinnedKey"].clone();
            case["signedReceipt"] = serde_json::json!(format!("0x{}", hex::encode(&bytes)));
            case["receiptJson"]["receipt"] = case["signedReceipt"].clone();
            case["receiptJson"]["txHash"] =
                serde_json::json!(format!("0x{}", hex::encode(&activity)));
            case["receiptJson"]["keyId"] = pin["pinnedKey"]["keyId"].clone();
            case["receiptJson"]["height"] = serde_json::json!(format!("0x{height:x}"));
            cases.push(case);
            let mut header = must(hex::decode(native_fixture["header_hex"].as_str().unwrap()));
            let original_header = must(layerx_wire::receipt::decode_batch_header(&header));
            let offset = header
                .windows(32)
                .position(|bytes| bytes == original_header.sequencer_id())
                .unwrap();
            header[offset..offset + 32].copy_from_slice(&key_id);
            header[9..13].copy_from_slice(&125_u32.to_be_bytes());
            header[23..31].copy_from_slice(&height.to_be_bytes());
            let signature = signer
                .sign(&must(layerx_wire::hash::batch_header_digest(&header)))
                .to_bytes();
            headers.push((header, signature));
        }
    }
    let mut harness = ReceiptHarness::start_with_pins(&cases, &pins);
    let kernel = must(UnixListener::bind(&harness.config.lni_socket));
    must(kernel.set_nonblocking(true));
    let initial_key = harness.config.sequencer_public_key;
    harness.config.sequencer_public_key = [0x73; 32];
    assert!(!expected_keys
        .iter()
        .any(|key| key.public_key() == harness.config.sequencer_public_key));
    assert_eq!(harness.config.chain_receipts.pinned_keys(), expected_keys);
    let expected = must(SequencerHistory::from_pinned_keys(125, &expected_keys));
    for (index, (header, signature)) in headers.iter().enumerate() {
        if index < 8 {
            assert!(expected.verify_header(header, signature).is_ok());
            assert!(trust::authorization(&harness.config, header, signature).is_ok());
            assert!(expected.verify_header(header, &[0; 64]).is_err());
        } else {
            assert!(expected.verify_header(header, signature).is_err());
            assert!(trust::authorization(&harness.config, header, signature).is_err());
        }
    }
    let unpinned = cases.split_off(8);
    assert_eq!(
        must(trust::current(&harness.config)),
        Some(expected.clone())
    );
    for key in &expected_keys {
        for target in [0, 1, 1000, u64::MAX] {
            assert_eq!(
                must(trust::snapshot(
                    &harness.config,
                    target,
                    key.public_key(),
                    Instant::now(),
                )),
                Some(expected.clone())
            );
        }
    }
    for claimed in [[0; 32], harness.config.sequencer_public_key] {
        assert!(trust::snapshot(&harness.config, 1, claimed, Instant::now()).is_err());
    }
    for case in &cases {
        let activity = hex_bytes(&case["receiptJson"]["txHash"])
            .try_into()
            .unwrap();
        let expected = hex_bytes(&case["signedReceipt"]);
        assert!(matches!(lookup_receipt(&harness.config, activity, false),
            ReceiptSource::Found(bytes) if bytes == expected));
    }
    assert_eq!(must(harness.calls.lock()).len(), cases.len());
    for case in &unpinned {
        let activity = hex_bytes(&case["receiptJson"]["txHash"])
            .try_into()
            .unwrap();
        assert!(matches!(
            lookup_receipt(&harness.config, activity, false),
            ReceiptSource::KeyMismatch
        ));
    }
    {
        let authority = AuthorityProcess::start(&harness);
        for case in &cases {
            let activity = case["receiptJson"]["txHash"]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap();
            let (status, body) = authority.receipt(activity);
            assert_eq!(status, 503);
            assert_eq!(
                body["error"]["code"], "replica_unavailable",
                "a pinned receipt is admitted before the independent replica evidence is requested"
            );
        }
        for case in &unpinned {
            let activity = case["receiptJson"]["txHash"]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap();
            let (status, body) = authority.receipt(activity);
            assert_eq!(status, 502);
            assert_eq!(body["error"]["code"], "sequencer_key_mismatch");
        }
    }

    let ranged_keys = [
        SequencerAuthorization::new(
            expected_keys[0].sequencer_id(),
            expected_keys[0].public_key(),
            10,
            19,
        ),
        SequencerAuthorization::new(
            expected_keys[1].sequencer_id(),
            expected_keys[1].public_key(),
            20,
            29,
        ),
    ];
    let ranged = must(SequencerHistory::from_pinned_keys(125, &ranged_keys));
    for (number, key) in [
        (10, ranged_keys[0]),
        (19, ranged_keys[0]),
        (20, ranged_keys[1]),
        (29, ranged_keys[1]),
    ] {
        assert_eq!(must(ranged.authorization_for_batch(number)), key);
        assert_eq!(must(ranged.authorization_for_sequence(number)), key);
        assert_eq!(
            must(ranged.sequence_interval_end(number)),
            key.last_batch_number()
        );
    }
    for number in [0, 9, 30, u64::MAX] {
        assert!(ranged.authorization_for_batch(number).is_err());
        assert!(ranged.authorization_for_sequence(number).is_err());
        assert!(ranged.sequence_interval_end(number).is_err());
    }
    assert!(expected.authorization_for_batch(1).is_err());
    assert!(expected.authorization_for_sequence(1).is_err());

    let keys_file = harness.directory.join("receipt-keys.json");
    let rotated = serde_json::json!([{
        "keyId": base64(&[0x77; 32]), "publicKey": base64(&initial_key),
        "firstHeight": 0, "lastHeight": u64::MAX,
    }]);
    must(fs::write(&keys_file, rotated.to_string()));
    assert_eq!(
        must(trust::current(&harness.config)),
        Some(expected.clone()),
        "editing the file must not rotate the running authority"
    );
    harness.config.chain_receipts = must(ChainReceipts::load(
        &format!("http://{}/evm", harness.config.replica_address),
        &keys_file,
    ));
    assert_ne!(must(trust::current(&harness.config)), Some(expected));
    assert!(trust::snapshot(
        &harness.config,
        1,
        expected_keys[1].public_key(),
        Instant::now()
    )
    .is_err());

    assert!(SequencerHistory::from_pinned_keys(0, &expected_keys).is_err());
    assert!(SequencerHistory::from_pinned_keys(125, &[]).is_err());
    assert!(
        SequencerHistory::from_pinned_keys(125, &[expected_keys[0], expected_keys[0]]).is_err()
    );
    for public_key in [[0; 32], [0xff; 32], {
        let mut identity = [0; 32];
        identity[0] = 1;
        identity
    }] {
        assert!(SequencerHistory::from_pinned_keys(
            125,
            &[SequencerAuthorization::new(
                [1; 32],
                public_key,
                0,
                u64::MAX
            ),]
        )
        .is_err());
    }
    for key in [
        SequencerAuthorization::new([0; 32], initial_key, 0, u64::MAX),
        SequencerAuthorization::new([1; 32], initial_key, 2, 1),
    ] {
        assert!(SequencerHistory::from_pinned_keys(125, &[key]).is_err());
    }
    let artifact = include_bytes!("fixtures/real-handover-genesis.lxt");
    let genesis = must(layerx_wire::handover::decode_genesis_trust(artifact));
    let native = must(SequencerHistory::from_genesis_artifact(
        artifact,
        genesis.network_id,
        genesis.canonical_state_root,
        genesis.initial_sequencer_key,
    ));
    assert!(native.verified_head().is_none());
    assert!(native.authorization_for_batch(1).is_err());
    assert!(native.authorization_for_sequence(1).is_err());
    assert!(native.signed_authority().intervals().is_empty());
    let error = kernel.accept().unwrap_err();
    assert_eq!(
        error.kind(),
        std::io::ErrorKind::WouldBlock,
        "trust snapshots and chain receipt admission must never open the kernel socket"
    );
}
