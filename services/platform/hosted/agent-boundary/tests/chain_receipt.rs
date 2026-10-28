use ed25519_dalek::{Signer as _, SigningKey};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use native_tls::{Certificate, TlsConnector};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::fmt::{Debug, Write as _};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

const HEIGHT: u64 = 150;
const KEY_ID: [u8; 32] = [1; 32];
const TX_HASH: [u8; 32] = [3; 32];

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        must(write!(text, "{byte:02x}"));
        text
    })
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

fn encoded_receipt(tx_hash: [u8; 32], success: bool, signature: Option<[u8; 64]>) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7_u64.to_be_bytes());
    sequence.extend_from_slice(&tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].fill(6);
    let mut to = [0; 32];
    to[12..].fill(7);
    let vm_error = if success { "" } else { "execution reverted" };
    let mut summary = Encoder::new(256);
    must(summary.u64(21_000));
    must(summary.u64(42_000));
    must(summary.u64(1));
    must(summary.u32(1));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&sha256(vm_error.as_bytes())));
    must(summary.u32(2));
    must(summary.u32(1));
    must(summary.u8(4));
    must(summary.fixed(b"upax"));
    let mut log = vec![8; 20];
    log.push(1);
    log.extend_from_slice(&[9; 32]);
    log.extend_from_slice(&sha256(&[10, 11]));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    must(encoder.u16(PROTOCOL_VERSION));
    must(encoder.bytes(&tx_hash, 32));
    must(encoder.u64(7));
    must(encoder.bytes(&previous, 32));
    must(encoder.bytes(&sha256(&sequence), 32));
    must(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    must(encoder.i32(i32::from(!success)));
    must(encoder.sequence_length(2, 512));
    for (ordinal, kind, body) in [(0, 1, summary.finish()), (1, 3, log)] {
        must(encoder.u16(12));
        must(encoder.u16(ordinal));
        must(encoder.u16(0));
        must(encoder.u8(kind));
        must(encoder.u8(0));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&body, 256));
    }
    must(encoder.u128(21_000));
    must(encoder.bytes(&height, 32));
    must(encoder.u16(12));
    must(encoder.u32(1));
    must(encoder.u32(0));
    must(encoder.u8(2));
    must(encoder.bytes(&sha256(b"upax"), 32));
    must(encoder.u128(1));
    must(encoder.bytes(&from, 32));
    must(encoder.u128(21_002));
    must(encoder.u128(if success { 1 } else { 2 }));
    must(encoder.u64(41));
    must(encoder.bytes(&to, 32));
    must(encoder.u128(0));
    must(encoder.u128(u128::from(success)));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&KEY_ID, 32));
    must(encoder.u64(1_000));
    must(encoder.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(encoder.bytes(&signature, 64));
    }
    encoder.finish()
}

fn signed_receipt(tx_hash: [u8; 32], signer: &SigningKey) -> (Value, Vec<u8>) {
    let success = true;
    let unsigned = encoded_receipt(tx_hash, success, None);
    let signature = signer.sign(&must(receipt_digest(&unsigned))).to_bytes();
    let bytes = encoded_receipt(tx_hash, success, Some(signature));
    let view = json!({
        "rung": "receipted", "txHash": hex(&tx_hash), "nonce": "0x29",
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": hex(&[8; 20]), "topics": [hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": "upax", "feeAmount": "0x5208", "sponsor": hex(&[0; 20]),
        "from": hex(&[6; 20]), "to": hex(&[7; 20]), "contractAddress": hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": hex(&KEY_ID), "receipt": hex(&bytes)
    });
    let mut view = view;
    view["signingKey"] = json!(hex(&signer.verifying_key().to_bytes()));
    (view, bytes)
}

fn read_message(stream: &mut impl Read) -> (String, Vec<u8>) {
    let mut reader = BufReader::new(stream);
    let mut headers = String::new();
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0, "truncated headers");
        assert!(headers.len() + line.len() < 16384);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = Some(must(value.trim().parse::<usize>()));
        }
        headers.push_str(&line);
    }
    let length = length.unwrap_or_else(|| panic!("missing content length"));
    assert!(length < 4 * MAX_MESSAGE_BYTES);
    let mut body = vec![0; length];
    must(reader.read_exact(&mut body));
    (headers, body)
}

fn chain_listener(results: Vec<Value>) -> (String, JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/rpc?chain=receipt", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let handle = thread::spawn(move || {
        for result in results {
            let deadline = Instant::now() + Duration::from_secs(15);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error)
                        if error.kind() == std::io::ErrorKind::WouldBlock
                            && Instant::now() < deadline =>
                    {
                        thread::sleep(Duration::from_millis(10))
                    }
                    Err(error) => panic!("chain accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            must(stream.set_write_timeout(Some(Duration::from_secs(5))));
            let (headers, body) = read_message(&mut stream);
            assert!(headers.starts_with("POST /rpc?chain=receipt HTTP/1.0\r\n"));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], "pax_sequencerReceipt");
            assert_eq!(request["params"], json!([hex(&TX_HASH)]));
            let body = json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            must(write!(stream, "HTTP/1.0 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
            must(stream.flush());
        }
    });
    (url, handle)
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
            .current_dir(directory)
            .args(args)
            .output(),
    );
    assert!(
        output.status.success(),
        "openssl: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

impl Boundary {
    fn start(url: &str, last_height: u64) -> Self {
        let mut unique = [0; 16];
        must(getrandom::fill(&mut unique));
        let directory = std::env::temp_dir().join(format!("boundary-receipts-{}", hex(&unique)));
        must(fs::create_dir(&directory));
        openssl(
            &directory,
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
            &directory,
            &[
                "x509", "-in", "cert.pem", "-outform", "DER", "-out", "cert.der",
            ],
        );
        openssl(
            &directory,
            &[
                "pkcs8", "-topk8", "-nocrypt", "-in", "key.pem", "-outform", "DER", "-out",
                "key.der",
            ],
        );
        for (name, token) in [
            ("gateway", "gateway-token"),
            ("registry", "registry-token"),
            ("webhook", "webhook-token"),
            ("node", "node-token"),
        ] {
            must(fs::write(directory.join(name), token));
        }
        let signer = SigningKey::from_bytes(&[8; 32]);
        let keys = json!([
            {"keyId": base64(&[2; 32]), "publicKey": base64(&SigningKey::from_bytes(&[9; 32]).verifying_key().to_bytes()), "firstHeight": 1, "lastHeight": 99},
            {"key_id": base64(&KEY_ID), "public_key": base64(&signer.verifying_key().to_bytes()), "first_height": 100, "last_height": last_height}
        ]);
        must(fs::write(directory.join("keys.json"), keys.to_string()));
        let port = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(port.local_addr());
        let stderr = must(fs::File::create(directory.join("stderr")));
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-agent-boundary"));
        command.env_clear();
        // Keep the runtime clock transport supplied by the task environment.
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
            .env(
                "LAYERX_AGENT_BOUNDARY_NODE_BEARER_TOKEN_FILE",
                directory.join("node"),
            )
            .env(
                "LAYERX_AGENT_BOUNDARY_LNI_SOCKET",
                directory.join("absent-kernel.sock"),
            )
            .env("LAYERX_AGENT_BOUNDARY_NODE_URL", "http://127.0.0.1:1")
            .env("LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL", url)
            .env(
                "LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE",
                directory.join("keys.json"),
            )
            .env("LAYERX_AGENT_BOUNDARY_STATE_DIR", directory.join("state"))
            .env("LAYERX_AGENT_BOUNDARY_PROTOCOL_NETWORK_ID", "125")
            .env("LAYERX_AGENT_BOUNDARY_NETWORK_ID", "receipt-chain")
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(stderr);
        drop(port);
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

    fn get(&self, path: &str, token: &str) -> (u16, Value) {
        let stream = must(TcpStream::connect_timeout(
            &self.address,
            Duration::from_secs(2),
        ));
        must(stream.set_read_timeout(Some(Duration::from_secs(20))));
        must(stream.set_write_timeout(Some(Duration::from_secs(5))));
        let mut stream = must(self.connector.connect("localhost", stream));
        must(write!(stream, "GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nConnection: close\r\n\r\n"));
        must(stream.flush());
        let (headers, body) = read_message(&mut stream);
        let status = must(
            headers
                .split_whitespace()
                .nth(1)
                .unwrap_or_else(|| panic!("status missing"))
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
fn receipt_route_reads_the_sequencer_receipt_of_the_transaction() {
    let signer = SigningKey::from_bytes(&[8; 32]);
    let (view, bytes) = signed_receipt(TX_HASH, &signer);
    let (mut bad_signature, _) = signed_receipt(TX_HASH, &SigningKey::from_bytes(&[9; 32]));
    bad_signature["signingKey"] = view["signingKey"].clone();
    let (wrong_transaction, _) = signed_receipt([4; 32], &signer);
    let mut corrupt = view.clone();
    let mut corrupt_bytes = bytes.clone();
    let last = corrupt_bytes.len() - 1;
    corrupt_bytes[last] ^= 1;
    corrupt["receipt"] = json!(hex(&corrupt_bytes));
    let (url, server) = chain_listener(vec![
        view.clone(),
        view.clone(),
        bad_signature,
        wrong_transaction,
        corrupt,
    ]);
    let boundary = Boundary::start(&url, 199);
    let gateway = format!("/v1/receipts/{}", hex(&TX_HASH));
    let expected = json!({"activity_id": &hex(&TX_HASH)[2..], "receipt": &hex(&bytes)[2..]});
    assert_eq!(
        boundary.get(&gateway, "gateway-token"),
        (200, json!({"result": expected.clone()}))
    );
    assert_eq!(
        boundary.get(
            &format!("/internal/v1/receipts/{}", hex(&TX_HASH)),
            "registry-token"
        ),
        (200, expected)
    );
    for _ in 0..3 {
        let (status, body) = boundary.get(&gateway, "gateway-token");
        assert_eq!(status, 503);
        assert_eq!(body["error"]["code"], "node_unavailable");
        assert!(body.get("result").is_none());
    }
    must(server.join());
    let (url, server) = chain_listener(vec![view]);
    let expired = Boundary::start(&url, 149);
    let (status, body) = expired.get(&gateway, "gateway-token");
    assert_eq!(status, 503);
    assert_eq!(body["error"]["code"], "node_unavailable");
    must(server.join());
}

#[test]
fn internal_receipt_route_refuses_an_unknown_transaction() {
    let (url, server) = chain_listener(vec![Value::Null, Value::Null]);
    let boundary = Boundary::start(&url, 199);
    let internal = format!("/internal/v1/receipts/{}", hex(&TX_HASH));
    for token in ["registry-token", "webhook-token"] {
        let (status, body) = boundary.get(&internal, token);
        assert_eq!(status, 404);
        assert_eq!(body["error"]["code"], "receipt_not_found");
        assert!(body.get("result").is_none());
    }
    must(server.join());
    let (status, body) = boundary.get(&internal, "registry-token");
    assert_eq!(status, 503);
    assert_eq!(body["error"]["code"], "node_unavailable");
    assert_eq!(body["error"]["retry_after_seconds"], 5);
}
