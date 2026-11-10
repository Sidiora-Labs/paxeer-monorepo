use std::fs::{self, OpenOptions};
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::OpenOptionsExt as _;
use std::path::PathBuf;
use std::process::{Command, Output, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::ledger_selector;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_wire::receipt::decode;
use serde_json::{json, Value};

fn hex(bytes: &[u8]) -> String {
    hex::encode(bytes)
}
fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert!(text.len().is_multiple_of(2));
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("hex text: {error:?}")),
                16,
            )
            .unwrap_or_else(|error| panic!("hex bytes: {error:?}"))
        })
        .collect()
}

struct KeyReference(PathBuf);
impl KeyReference {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "wallet-rpc-key-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path)
            .unwrap_or_else(|error| panic!("private test key reference: {error:?}"));
        file.write_all(&[0x42; 64])
            .unwrap_or_else(|error| panic!("test account seed: {error:?}"));
        Self(path)
    }
}
impl Drop for KeyReference {
    fn drop(&mut self) {
        fs::remove_file(&self.0)
            .unwrap_or_else(|error| panic!("remove test key reference: {error:?}"));
    }
}

fn configuration(endpoint: &str) -> Value {
    json!({"endpoint": endpoint, "rpc_route": "node", "protocol_version": "3", "network_id": "125",
        "native_asset": "03".repeat(32), "sequencer_id": "01".repeat(32),
        "sequencer_key": "02".repeat(32), "first_batch": "1", "last_batch": "100"})
}
fn request(value: &Value) -> (Output, Value) {
    let mut child = Command::new(env!("CARGO_BIN_EXE_layerx-wallet-rpc"))
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap_or_else(|error| panic!("wallet RPC binary: {error:?}"));
    child
        .stdin
        .take()
        .unwrap_or_else(|| panic!("stdin"))
        .write_all(
            &serde_json::to_vec(value).unwrap_or_else(|error| panic!("request JSON: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("write request: {error:?}"));
    let deadline = Instant::now() + Duration::from_secs(10);
    loop {
        if child
            .try_wait()
            .unwrap_or_else(|error| panic!("binary status: {error:?}"))
            .is_some()
        {
            break;
        }
        if Instant::now() >= deadline {
            child
                .kill()
                .unwrap_or_else(|error| panic!("kill timed out binary: {error:?}"));
            let output = child
                .wait_with_output()
                .unwrap_or_else(|error| panic!("reap timed out binary: {error:?}"));
            panic!(
                "wallet binary timed out: {}",
                String::from_utf8_lossy(&output.stderr)
            );
        }
        thread::sleep(Duration::from_millis(2));
    }
    let output = child
        .wait_with_output()
        .unwrap_or_else(|error| panic!("binary output: {error:?}"));
    let reply = serde_json::from_slice(&output.stdout)
        .unwrap_or_else(|error| panic!("exactly one JSON reply: {error:?}"));
    (output, reply)
}

fn listener(steps: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("loopback listener: {error:?}"));
    let endpoint = format!(
        "http://{}",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("address: {error:?}"))
    );
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("nonblocking listener: {error:?}"));
    let peer = thread::spawn(move || {
        let mut requests = Vec::new();
        for (method, mut result) in steps {
            let deadline = Instant::now() + Duration::from_secs(10);
            let mut socket = loop {
                match listener.accept() {
                    Ok((socket, _)) => break socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method}");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            socket
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("read timeout: {error:?}"));
            socket
                .set_write_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("write timeout: {error:?}"));
            let mut header = Vec::new();
            while !header.ends_with(b"\r\n\r\n") {
                assert!(header.len() < 8192);
                let mut byte = [0];
                socket
                    .read_exact(&mut byte)
                    .unwrap_or_else(|error| panic!("HTTP header: {error:?}"));
                header.push(byte[0]);
            }
            assert!(std::str::from_utf8(&header)
                .unwrap_or_else(|error| panic!("HTTP text: {error:?}"))
                .starts_with("POST / HTTP/1.1\r\n"));
            let length = std::str::from_utf8(&header)
                .unwrap_or_else(|error| panic!("HTTP text: {error:?}"))
                .lines()
                .find_map(|line| {
                    let (key, value) = line.split_once(':')?;
                    key.eq_ignore_ascii_case("content-length").then(|| {
                        value
                            .trim()
                            .parse::<usize>()
                            .unwrap_or_else(|error| panic!("HTTP length: {error:?}"))
                    })
                })
                .unwrap_or_else(|| panic!("content length"));
            assert!(length <= 1_048_576 + 4096);
            let mut body = vec![0; length];
            socket
                .read_exact(&mut body)
                .unwrap_or_else(|error| panic!("HTTP body: {error:?}"));
            let message: Value =
                serde_json::from_slice(&body).unwrap_or_else(|error| panic!("RPC JSON: {error:?}"));
            assert_eq!(message["method"], method);
            if method == "eth_sendRawTransaction" {
                let raw = unhex(
                    message["params"][0]
                        .as_str()
                        .unwrap_or_else(|| panic!("raw transaction")),
                );
                result = json!(format!("0x{}", hex(&transaction_hash(&raw))));
            }
            let response = serde_json::to_vec(
                &json!({"jsonrpc": "2.0", "id": message["id"], "result": result}),
            )
            .unwrap_or_else(|error| panic!("response JSON: {error:?}"));
            write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).unwrap_or_else(|error| panic!("response header: {error:?}"));
            socket
                .write_all(&response)
                .unwrap_or_else(|error| panic!("response body: {error:?}"));
            requests.push(message);
        }
        requests
    });
    (endpoint, peer)
}

#[test]
fn submit_reply_carries_the_transaction_hash_and_rung() {
    let key = KeyReference::new();
    let account = derive_from_seed(&[0x42; 64], 0)
        .unwrap_or_else(|error| panic!("derived test account: {error:?}"));
    let payload = include_str!("../../layerx-crypto/tests/fixtures/payments/native-1-5.hex").trim();
    let (endpoint, peer) = listener(vec![
        ("eth_getTransactionCount", json!("0x7")),
        ("eth_chainId", json!("0x7d")),
        ("eth_gasPrice", json!("0x2")),
        ("eth_maxPriorityFeePerGas", json!("0x1")),
        ("eth_estimateGas", json!("0x186a0")),
        (
            "px_getAccount",
            json!({"account_id": "01".repeat(32), "next_sequence": "7"}),
        ),
        ("eth_sendRawTransaction", Value::Null),
    ]);
    let mut config = configuration(&endpoint);
    config["evm_key_ref"] = json!(key.0);
    config["account_index"] = json!("0");
    let (output, reply) = request(&json!({"action": "submit", "configuration": config,
        "actor": "did:layerx:alice", "ordinal": "5", "payload": payload,
        "idempotency_key": "04".repeat(32), "fee_limit": "1000000", "not_before": "0", "not_after": "100", "timeout_ms": "0"}));
    let requests = peer
        .join()
        .unwrap_or_else(|error| panic!("loopback peer: {error:?}"));
    assert!(output.status.success(), "{reply}");
    assert_eq!(
        requests[0]["params"],
        json!([format!("0x{}", hex(&account.evm_address())), "pending"])
    );
    let raw = unhex(
        requests[6]["params"][0]
            .as_str()
            .unwrap_or_else(|| panic!("signed transaction")),
    );
    let (call, signature) = decode_signed(&raw)
        .unwrap_or_else(|error| panic!("real signed EIP-1559 transaction: {error:?}"));
    assert_eq!(
        recover_sender(&call, &signature)
            .unwrap_or_else(|error| panic!("recovered sender: {error:?}")),
        account.evm_address()
    );
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(call.gas_limit, 100_000);
    assert_eq!(&call.data[..4], &ledger_selector("send(bytes)"));
    assert_eq!(
        reply,
        json!({"ok": true, "result": {"txHash": format!("0x{}", hex(&transaction_hash(&raw))), "rung": "submitted"}})
    );
    assert!(reply["result"].get("activity_id").is_none());
}

fn signed_receipt() -> (Vec<u8>, Value, String) {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture: {error:?}"));
    let case = &fixture["cases"][0];
    let canonical = unhex(
        case["signedReceipt"]
            .as_str()
            .unwrap_or_else(|| panic!("signed sequencer receipt")),
    );
    let decoded = decode(&canonical).unwrap_or_else(|error| panic!("decoded fixture: {error:?}"));
    let protocol = decoded
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt"));
    let key = &case["pinnedKey"];
    let mut config = configuration("http://127.0.0.1:1");
    config["protocol_version"] = json!(protocol.protocol_version().to_string());
    config["sequencer_id"] = json!(hex(&unhex(
        key["keyId"]
            .as_str()
            .unwrap_or_else(|| panic!("receipt key ID"))
    )));
    config["sequencer_key"] = json!(hex(&unhex(
        key["publicKey"]
            .as_str()
            .unwrap_or_else(|| panic!("receipt public key"))
    )));
    config["first_batch"] = json!(key["firstHeight"]
        .as_u64()
        .unwrap_or_else(|| panic!("first height"))
        .to_string());
    config["last_batch"] = json!(key["lastHeight"]
        .as_u64()
        .unwrap_or_else(|| panic!("last height"))
        .to_string());
    (
        canonical,
        config,
        case["receiptJson"]["txHash"]
            .as_str()
            .unwrap_or_else(|| panic!("receipt transaction hash"))
            .to_owned(),
    )
}

fn assert_wait_deadline_and_retired_names(base: &Value, hash: &str) {
    let stalled_listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("stalled listener: {error:?}"));
    stalled_listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("nonblocking stalled listener: {error:?}"));
    let mut bounded = base.clone();
    bounded["configuration"]["endpoint"] = json!(format!(
        "http://{}",
        stalled_listener
            .local_addr()
            .unwrap_or_else(|error| panic!("stalled address: {error:?}"))
    ));
    bounded["timeout_ms"] = json!("50");
    let began = Instant::now();
    let (output, reply) = request(&bounded);
    assert!(
        began.elapsed() < Duration::from_secs(1),
        "wallet deadline must bound a stalled RPC read"
    );
    assert!(!output.status.success());
    assert_eq!(
        reply,
        json!({"ok": false, "error": {"kind": "pending", "txHash": hash, "rung": "submitted"}})
    );
    assert!(
        stalled_listener.accept().is_ok(),
        "the bounded wait must actually poll"
    );

    let refused_listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("refusal listener: {error:?}"));
    refused_listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("nonblocking refusal listener: {error:?}"));
    for old in ["executed", "batched", "finalised"] {
        for field in ["rung", "commitment"] {
            let mut invalid = base.clone();
            invalid["configuration"]["endpoint"] = json!(format!(
                "http://{}",
                refused_listener
                    .local_addr()
                    .unwrap_or_else(|error| panic!("refusal address: {error:?}"))
            ));
            invalid[field] = json!(old);
            let (output, reply) = request(&invalid);
            assert!(!output.status.success(), "{field}={old}");
            assert_eq!(
                reply,
                json!({"ok": false, "error": {"kind": "wallet_request_refused_or_unavailable"}})
            );
            assert_eq!(
                refused_listener
                    .accept()
                    .err()
                    .unwrap_or_else(|| panic!("retired names must not poll"))
                    .kind(),
                std::io::ErrorKind::WouldBlock
            );
        }
    }
}

#[test]
fn wait_accepts_rung_names_and_refuses_kernel_commitments() {
    let (canonical, mut config, hash) = signed_receipt();
    let (endpoint, peer) = listener(vec![
        (
            "pax_sequencerReceipt",
            json!({"txHash": hash, "rung": "submitted"}),
        ),
        (
            "pax_sequencerReceipt",
            json!({"txHash": hash, "rung": "receipted", "receipt": format!("0x{}", hex(&canonical))}),
        ),
    ]);
    config["endpoint"] = json!(endpoint);
    let base = json!({"action": "wait", "configuration": config, "txHash": hash, "rung": "receipted", "timeout_ms": "1000"});
    let (output, reply) = request(&base);
    let requests = peer
        .join()
        .unwrap_or_else(|error| panic!("receipt peer: {error:?}"));
    assert_eq!(requests.len(), 2);
    for message in requests {
        assert_eq!(message["params"], json!([hash]));
    }
    assert!(output.status.success(), "{reply}");
    assert_eq!(reply["ok"], true);
    assert_eq!(reply["result"]["txHash"], hash);
    assert_eq!(reply["result"]["rung"], "receipted");
    assert_eq!(reply["result"]["receipt"], hex(&canonical));

    let mut submitted = base.clone();
    submitted["rung"] = json!("submitted");
    let (output, reply) = request(&submitted);
    assert!(output.status.success(), "{reply}");
    assert_eq!(
        reply,
        json!({"ok": true, "result": {"txHash": hash, "rung": "submitted"}})
    );

    let (endpoint, peer) = listener(vec![(
        "pax_sequencerReceipt",
        json!({"txHash": hash, "rung": "committed", "receipt": format!("0x{}", hex(&canonical))}),
    )]);
    let mut committed = base.clone();
    committed["configuration"]["endpoint"] = json!(endpoint);
    committed["rung"] = json!("committed");
    let (output, reply) = request(&committed);
    assert_eq!(
        peer.join()
            .unwrap_or_else(|error| panic!("commit peer: {error:?}"))
            .len(),
        1
    );
    assert!(!output.status.success());
    assert_eq!(
        reply,
        json!({"ok": false, "error": {"kind": "wallet_request_refused_or_unavailable"}})
    );

    let mut corrupted = canonical.clone();
    let last = corrupted.len() - 1;
    corrupted[last] ^= 1;
    let (endpoint, peer) = listener(vec![(
        "pax_sequencerReceipt",
        json!({"txHash": hash, "rung": "receipted", "receipt": format!("0x{}", hex(&corrupted))}),
    )]);
    let mut invalid_receipt = base.clone();
    invalid_receipt["configuration"]["endpoint"] = json!(endpoint);
    let (output, reply) = request(&invalid_receipt);
    assert_eq!(
        peer.join()
            .unwrap_or_else(|error| panic!("invalid receipt peer: {error:?}"))
            .len(),
        1
    );
    assert!(!output.status.success());
    assert_eq!(reply["ok"], false);

    assert_wait_deadline_and_retired_names(&base, &hash);
}

#[allow(dead_code)]
#[path = "support/audit_22_30_99.rs"]
mod audit;

#[test]
fn binary_root_route_returns_the_served_balance() {
    audit::root_binary_request_and_method_response();
}
