use native_tls::{Certificate, TlsConnector};
use serde_json::{json, Value};
use std::fmt::{Debug, Write as _};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

const RETIRED_CONFIG: &[&str] = &[
    "LAYERX_AGENT_BOUNDARY_NODE_URL",
    "LAYERX_AGENT_BOUNDARY_NODE_BEARER_TOKEN_FILE",
    "LAYERX_AGENT_BOUNDARY_STATE_DIR",
    "LAYERX_AGENT_BOUNDARY_PROTOCOL_NETWORK_ID",
    "LAYERX_AGENT_BOUNDARY_MODULE_REGISTRY_FILE",
    "LAYERX_AGENT_BOUNDARY_RECEIPT_WAIT_MS",
];

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::new(), |mut text, byte| {
        must(write!(text, "{byte:02x}"));
        text
    })
}

fn decode_hex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert!(text.len().is_multiple_of(2));
    text.as_bytes()
        .chunks_exact(2)
        .map(|part| must(u8::from_str_radix(must(std::str::from_utf8(part)), 16)))
        .collect()
}

fn base64(bytes: &[u8]) -> String {
    const DIGITS: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut text = String::new();
    for part in bytes.chunks(3) {
        let a = part[0];
        let b = part.get(1).copied().unwrap_or(0);
        let c = part.get(2).copied().unwrap_or(0);
        text.push(char::from(DIGITS[usize::from(a >> 2)]));
        text.push(char::from(DIGITS[usize::from((a & 3) << 4 | b >> 4)]));
        text.push(if part.len() > 1 {
            char::from(DIGITS[usize::from((b & 15) << 2 | c >> 6)])
        } else {
            '='
        });
        text.push(if part.len() > 2 {
            char::from(DIGITS[usize::from(c & 63)])
        } else {
            '='
        });
    }
    text
}

fn producer_case() -> Value {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .unwrap_or_else(|| panic!("repository root"));
    let document: Value = must(serde_json::from_slice(&must(fs::read(
        root.join("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"),
    ))));
    assert_eq!(
        document["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    assert_eq!(document["chainId"], 125);
    document["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("producer cases"))
        .iter()
        .find(|case| case["id"] == "V1-pax-transfer")
        .unwrap_or_else(|| panic!("producer transfer case"))
        .clone()
}

fn key_document(case: &Value) -> Value {
    let pin = &case["pinnedKey"];
    json!([{
        "keyId": base64(&decode_hex(pin["keyId"].as_str().unwrap_or_else(|| panic!("key ID")))),
        "publicKey": base64(&decode_hex(pin["publicKey"].as_str().unwrap_or_else(|| panic!("public key")))),
        "firstHeight": pin["firstHeight"],
        "lastHeight": pin["lastHeight"],
    }])
}

fn read_message(stream: &mut impl Read) -> (String, Vec<u8>) {
    let mut reader = BufReader::new(stream);
    let mut headers = String::new();
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0, "truncated headers");
        assert!(headers.len() + line.len() < 16_384);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = Some(must(value.trim().parse::<usize>()));
        }
        headers.push_str(&line);
    }
    let length = length.unwrap_or_else(|| panic!("content length missing"));
    assert!(length < 4 * 1024 * 1024);
    let mut body = vec![0; length];
    must(reader.read_exact(&mut body));
    (headers, body)
}

// Replays the real Go producer's signed bytes without reconstructing its wire
// format. Both HTTP hops and the boundary's height-pinned verifier run normally.
struct RecordedProducer {
    address: SocketAddr,
    requests: Arc<AtomicUsize>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl RecordedProducer {
    fn start(case: &Value) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        must(listener.set_nonblocking(true));
        let requests = Arc::new(AtomicUsize::new(0));
        let stop = Arc::new(AtomicBool::new(false));
        let stop_signal = Arc::clone(&stop);
        let request_count = Arc::clone(&requests);
        let receipt = case["receiptJson"].clone();
        assert_eq!(receipt["receipt"], case["signedReceipt"]);
        let worker = thread::spawn(move || {
            while !stop_signal.load(Ordering::SeqCst) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(5));
                        continue;
                    }
                    Err(error) => panic!("producer transport: {error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                must(stream.set_write_timeout(Some(Duration::from_secs(5))));
                let (headers, body) = read_message(&mut stream);
                assert!(headers.starts_with("POST / HTTP/1.0\r\n"));
                assert!(!headers.to_ascii_lowercase().contains("authorization:"));
                let request: Value = must(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                assert_eq!(request["method"], "pax_sequencerReceipt");
                assert_eq!(
                    request["params"]
                        .as_array()
                        .unwrap_or_else(|| panic!("parameters"))
                        .len(),
                    1
                );
                let result = if request["params"][0] == receipt["txHash"] {
                    receipt.clone()
                } else {
                    Value::Null
                };
                let reply =
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
                request_count.fetch_add(1, Ordering::SeqCst);
                must(write!(stream, "HTTP/1.0 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{reply}", reply.len()));
                must(stream.flush());
            }
        });
        Self {
            address,
            requests,
            stop,
            worker: Some(worker),
        }
    }

    fn url(&self) -> String {
        format!("http://{}", self.address)
    }

    fn close(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}

impl Drop for RecordedProducer {
    fn drop(&mut self) {
        self.close();
    }
}

struct Boundary {
    child: Child,
    directory: PathBuf,
    address: SocketAddr,
    connector: TlsConnector,
}

fn openssl(directory: &Path, args: &[&str]) {
    let output = must(
        Command::new("openssl")
            .args(args)
            .current_dir(directory)
            .output(),
    );
    assert!(
        output.status.success(),
        "openssl: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn prepare_tls_material(directory: &Path) {
    openssl(
        directory,
        &[
            "req",
            "-x509",
            "-newkey",
            "ec",
            "-pkeyopt",
            "ec_paramgen_curve:P-256",
            "-nodes",
            "-keyout",
            "key.pem",
            "-out",
            "cert.pem",
            "-days",
            "1",
            "-subj",
            "/CN=localhost",
            "-addext",
            "subjectAltName=DNS:localhost",
        ],
    );
    openssl(
        directory,
        &[
            "x509", "-in", "cert.pem", "-outform", "DER", "-out", "cert.der",
        ],
    );
    openssl(
        directory,
        &[
            "pkcs8", "-topk8", "-nocrypt", "-in", "key.pem", "-outform", "DER", "-out", "key.der",
        ],
    );
}

impl Boundary {
    fn start(url: &str, keys: &Value) -> Self {
        let mut unique = [0; 16];
        must(getrandom::fill(&mut unique));
        let directory =
            std::env::temp_dir().join(format!("boundary-repair-22-31-191-{}", hex(&unique)));
        must(fs::create_dir(&directory));
        prepare_tls_material(&directory);
        for plane in ["gateway", "registry", "webhook"] {
            must(fs::write(directory.join(plane), format!("{plane}-token")));
        }
        must(fs::write(directory.join("keys.json"), keys.to_string()));
        let reservation = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(reservation.local_addr());
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-agent-boundary"));
        command.env_clear();
        for (name, value) in std::env::vars_os()
            .filter(|(name, _)| name.to_string_lossy().starts_with("LAYERX_RUNTIME_CLOCK"))
        {
            command.env(name, value);
        }
        command
            .env("LAYERX_AGENT_BOUNDARY_LISTEN", address.to_string())
            .env(
                "LAYERX_AGENT_BOUNDARY_TLS_CERT_DER",
                directory.join("cert.der"),
            )
            .env(
                "LAYERX_AGENT_BOUNDARY_TLS_KEY_DER",
                directory.join("key.der"),
            )
            .env(
                "LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE",
                directory.join("gateway"),
            )
            .env(
                "LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE",
                directory.join("registry"),
            )
            .env(
                "LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE",
                directory.join("webhook"),
            )
            .env("LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL", url)
            .env(
                "LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE",
                directory.join("keys.json"),
            )
            .env("LAYERX_AGENT_BOUNDARY_NETWORK_ID", "chain-125")
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(must(fs::File::create(directory.join("stderr"))));
        for name in RETIRED_CONFIG {
            assert!(
                !command.get_envs().any(|(key, _)| key == *name),
                "retired setting {name}"
            );
        }
        drop(reservation);
        let child = must(command.spawn());
        let mut builder = TlsConnector::builder();
        builder.add_root_certificate(must(Certificate::from_der(&must(fs::read(
            directory.join("cert.der"),
        )))));
        let connector = must(builder.build());
        let mut boundary = Self {
            child,
            directory,
            address,
            connector,
        };
        let deadline = Instant::now() + Duration::from_secs(10);
        loop {
            if let Some(status) = must(boundary.child.try_wait()) {
                panic!(
                    "boundary exited {status}: {}",
                    must(fs::read_to_string(boundary.directory.join("stderr")))
                );
            }
            if TcpStream::connect_timeout(&address, Duration::from_millis(50)).is_ok() {
                break;
            }
            assert!(Instant::now() < deadline, "boundary startup deadline");
            thread::sleep(Duration::from_millis(10));
        }
        boundary
    }

    fn request(&self, method: &str, path: &str, token: &str) -> (u16, Value) {
        let stream = must(TcpStream::connect_timeout(
            &self.address,
            Duration::from_secs(2),
        ));
        must(stream.set_read_timeout(Some(Duration::from_secs(20))));
        must(stream.set_write_timeout(Some(Duration::from_secs(5))));
        let mut stream = must(self.connector.connect("localhost", stream));
        must(write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"));
        must(stream.flush());
        let (headers, body) = read_message(&mut stream);
        let status = must(
            headers
                .split_whitespace()
                .nth(1)
                .unwrap_or_else(|| panic!("HTTP status"))
                .parse(),
        );
        (status, must(serde_json::from_slice(&body)))
    }
}

impl Drop for Boundary {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        let _ = fs::remove_dir_all(&self.directory);
    }
}

#[test]
fn finding_036_account_state_relays_are_retired_over_tls() {
    let case = producer_case();
    let producer = RecordedProducer::start(&case);
    let boundary = Boundary::start(&producer.url(), &key_document(&case));
    let digest = "01".repeat(32);
    for path in [
        "/v1/protocol/account-state/head".to_owned(),
        format!("/v1/receipts/{digest}/account-state"),
        "/v1/receipts/invalid/account-state".to_owned(),
    ] {
        for token in ["registry-token", "gateway-token", "webhook-token"] {
            for method in ["GET", "POST"] {
                for target in [path.clone(), format!("{path}?at=1000")] {
                    assert_eq!(
                        boundary.request(method, &target, token),
                        (
                            404,
                            json!({"error": {"code": "not_found", "retry": "never"}})
                        ),
                        "{method} {target} with {token}"
                    );
                }
            }
        }
        assert_eq!(boundary.request("GET", &path, "invalid").0, 401);
    }
    assert_eq!(
        producer.requests.load(Ordering::SeqCst),
        0,
        "retired routes reached the chain RPC"
    );
    assert_eq!(boundary.request("GET", "/livez", "").0, 200);
}

#[test]
fn finding_037_readiness_and_receipts_need_only_chain_dependencies() {
    let case = producer_case();
    let mut producer = RecordedProducer::start(&case);
    let boundary = Boundary::start(&producer.url(), &key_document(&case));
    let expected_ready = json!({"ready": true, "network_id": "chain-125", "chain_receipts": true});
    assert_eq!(
        boundary.request("GET", "/readyz", ""),
        (200, expected_ready)
    );
    let transaction = case["receiptJson"]["txHash"]
        .as_str()
        .unwrap_or_else(|| panic!("transaction hash"));
    let receipt = case["signedReceipt"]
        .as_str()
        .unwrap_or_else(|| panic!("signed receipt"))
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("receipt hex prefix"));
    let expected = json!({"activity_id": transaction.strip_prefix("0x").unwrap_or_else(|| panic!("transaction hex prefix")), "receipt": receipt});
    assert_eq!(
        boundary.request(
            "GET",
            &format!("/v1/receipts/{transaction}"),
            "gateway-token"
        ),
        (200, json!({"result": expected.clone()}))
    );
    for token in ["registry-token", "webhook-token"] {
        assert_eq!(
            boundary.request(
                "GET",
                &format!("/internal/v1/receipts/{transaction}"),
                token
            ),
            (200, expected.clone())
        );
    }
    assert_eq!(
        boundary.request(
            "GET",
            &format!("/internal/v1/deployment-proof/{transaction}"),
            "registry-token"
        ),
        (
            200,
            json!({"transaction_hash": transaction.strip_prefix("0x").unwrap_or_else(|| panic!("transaction hex prefix")), "receipt": receipt})
        )
    );
    assert_eq!(
        boundary.request(
            "GET",
            &format!("/internal/v1/receipts/{}", "00".repeat(32)),
            "registry-token"
        ),
        (
            404,
            json!({"error": {"code": "receipt_not_found", "retry": "never"}})
        )
    );
    assert_eq!(producer.requests.load(Ordering::SeqCst), 6);
    for name in ["state", "journal", "activities"] {
        assert!(
            !boundary.directory.join(name).exists(),
            "created unused {name} directory"
        );
    }
    producer.close();
    let unavailable =
        json!({"error": {"code": "node_unavailable", "retry": "after", "retry_after_seconds": 5}});
    assert_eq!(
        boundary.request("GET", "/readyz", ""),
        (503, unavailable.clone())
    );
    assert_eq!(
        boundary.request(
            "GET",
            &format!("/v1/receipts/{transaction}"),
            "gateway-token"
        ),
        (503, unavailable)
    );
    assert_eq!(boundary.request("GET", "/livez", "").0, 200);
}

#[test]
fn finding_037_receipt_reads_enforce_the_pinned_key_height() {
    let case = producer_case();
    let producer = RecordedProducer::start(&case);
    let mut keys = key_document(&case);
    // Keep the actual producer receipt intact; expire its configured trust pin.
    keys[0]["lastHeight"] = json!(999);
    keys[0]["firstHeight"] = json!(1);
    let boundary = Boundary::start(&producer.url(), &keys);
    let transaction = case["receiptJson"]["txHash"]
        .as_str()
        .unwrap_or_else(|| panic!("transaction hash"));
    assert_eq!(
        boundary.request(
            "GET",
            &format!("/internal/v1/receipts/{transaction}"),
            "registry-token"
        ),
        (
            503,
            json!({"error": {"code": "node_unavailable", "retry": "after", "retry_after_seconds": 5}})
        )
    );
}
