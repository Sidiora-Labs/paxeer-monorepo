use std::fmt::Debug;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_human_identity_provider::provision_head::validate_account_head;
use serde_json::{json, Value};

const NOW: u64 = 1_700_000_000_000;
const FIXTURE: &str = include_str!("../src/fixtures/repair_22_33_34.json");

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn fixture() -> Value {
    must(serde_json::from_str(FIXTURE))
}

fn text(value: &Value) -> &str {
    value
        .as_str()
        .unwrap_or_else(|| panic!("expected text: {value}"))
}

fn snapshot(height: u64) -> Value {
    let mut record = fixture();
    record["height"] = json!(height);
    record["block"] = json!({
        "number": format!("{height:#x}"), "hash": format!("0x{height:064x}"),
        "parentHash": format!("0x{:064x}", height - 1), "timestamp": format!("{:#x}", NOW / 1000),
        "stateRoot": format!("0x{:064x}", height + 100),
        "transactionsRoot": format!("0x{}", "11".repeat(32)),
        "receiptsRoot": format!("0x{}", "22".repeat(32)), "transactions": []
    });
    record["account"]["paxeer"]["height"] = json!(height);
    record
}

#[derive(Clone, Copy)]
enum Damage {
    None,
    RpcError,
    WrongResponseId,
    AdvancingHead,
    ChangedHash,
    ChangedRoot,
}

#[derive(Default)]
struct ReadCounts {
    head: u32,
    block: u32,
    root: u32,
}

fn transcript_result(
    record: &Value,
    request: &Value,
    damage: Damage,
    height: u64,
    counts: &mut ReadCounts,
) -> Value {
    let params = &request["params"];
    match text(&request["method"]) {
        "eth_chainId" => {
            assert_eq!(params, &json!([]));
            json!("0x7d")
        }
        "eth_blockNumber" => {
            assert_eq!(params, &json!([]));
            counts.head += 1;
            json!(format!(
                "{:#x}",
                height + u64::from(matches!(damage, Damage::AdvancingHead) && counts.head > 1)
            ))
        }
        "eth_getBlockByNumber" => {
            assert_eq!(params[0], format!("{height:#x}"));
            let mut block = record["block"].clone();
            if params[1] == true {
                counts.block += 1;
                if matches!(damage, Damage::ChangedHash) && counts.block > 1 {
                    block["hash"] = json!(format!("0x{}", "ff".repeat(32)));
                }
            } else {
                assert_eq!(params[1], false);
                counts.root += 1;
                if matches!(damage, Damage::ChangedRoot) && counts.root > 1 {
                    block["stateRoot"] = json!(format!("0x{}", "ff".repeat(32)));
                }
            }
            block
        }
        "px_getAccount" => {
            assert_eq!(params, &json!([record["owner_did"]]));
            record["account"].clone()
        }
        "eth_call" => {
            assert_eq!(params[1], format!("{height:#x}"));
            let calls = record["calls"]
                .as_array()
                .unwrap_or_else(|| panic!("calls"));
            calls
                .iter()
                .find(|call| call["to"] == params[0]["to"] && call["data"] == params[0]["data"])
                .unwrap_or_else(|| panic!("unrecorded call: {request}"))["result"]
                .clone()
        }
        other => panic!("unrecorded method: {other}"),
    }
}

// The peer serves the committed ABI codec artifact from a disk-backed transcript.
// The production component under repair is validate_account_head, including its
// actual SequencerEndpoint and PxEndpoint transports and decoders.
struct TranscriptPeer {
    url: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    requests: Arc<Mutex<Vec<Value>>>,
}

impl TranscriptPeer {
    fn reopen(path: &Path, damage: Damage) -> Self {
        let record: Value = must(serde_json::from_slice(&must(fs::read(path))));
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let seen = Arc::clone(&requests);
        let worker = std::thread::spawn(move || {
            let height = record["height"]
                .as_u64()
                .unwrap_or_else(|| panic!("height"));
            let mut counts = ReadCounts::default();
            while !stopped.load(Ordering::Acquire) {
                let stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                must(stream.set_write_timeout(Some(Duration::from_secs(5))));
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                assert!(line.starts_with("POST / HTTP/1.1"));
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
                let length = length.unwrap_or_else(|| panic!("content length"));
                assert!(length <= 1_048_576);
                let mut bytes = vec![0; length];
                must(reader.read_exact(&mut bytes));
                let request: Value = must(serde_json::from_slice(&bytes));
                assert_eq!(request["jsonrpc"], "2.0");
                must(seen.lock()).push(request.clone());
                let result = transcript_result(&record, &request, damage, height, &mut counts);
                let mut response = if matches!(damage, Damage::RpcError) {
                    json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32000,"message":"unsupported history"}})
                } else {
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
                };
                if matches!(damage, Damage::WrongResponseId) {
                    response["id"] = json!(999_999);
                }
                let body = response.to_string();
                let mut stream = reader.into_inner();
                must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
                must(stream.flush());
            }
        });
        Self {
            url,
            stop,
            worker: Some(worker),
            requests,
        }
    }

    fn input(&self) -> Value {
        let fixture = fixture();
        json!({"rpc_url":self.url, "rpc_ca_der_hex":[], "chain_id":fixture["chain_id"],
            "owner_did":fixture["owner_did"], "owner_account":fixture["owner_account"],
            "asset_id":fixture["asset_id"], "maximum_age_seconds":60})
    }
}

impl Drop for TranscriptPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}

fn write_snapshot(path: &Path, value: &Value) {
    must(fs::write(path, must(serde_json::to_vec(value))));
}

fn observe(input: &Value, now: u64) -> std::io::Result<Value> {
    let mut output = Vec::new();
    validate_account_head(input.to_string().as_bytes(), &mut output, now)?;
    serde_json::from_slice(&output).map_err(std::io::Error::other)
}

fn assert_refused(input: &Value, now: u64) {
    let mut output = Vec::new();
    assert!(validate_account_head(input.to_string().as_bytes(), &mut output, now).is_err());
    assert!(output.is_empty(), "refusal published a consumption result");
}

#[test]
fn identity_first_batch_provisions_after_restart_at_later_height() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    let mut first = snapshot(17);
    write_snapshot(&store, &first);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    let result = must(observe(&peer.input(), NOW));
    assert_eq!(result["committed_height"], 17);
    assert_eq!(result["consumed"], 0);
    assert_eq!(result["account"], first["owner_account"]);
    assert_eq!(result["owner_did"], first["owner_did"]);
    assert_eq!(result["balance"], "100");
    let calls = must(peer.requests.lock()).clone();
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .count(),
        5
    );
    drop(peer);
    // Stop the peer and discard its in-memory state, then reopen its retained
    // account at a later committed height. The consumer is reopened per call.
    first["height"] = json!(42);
    first["block"] = snapshot(42)["block"].clone();
    first["account"]["paxeer"]["height"] = json!(42);
    write_snapshot(&store, &first);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    let recovered = must(observe(&peer.input(), NOW));
    assert_eq!(recovered["committed_height"], 42);
    assert_ne!(recovered["block_hash"], result["block_hash"]);
    assert_ne!(recovered["state_root"], result["state_root"]);
    for field in [
        "schema",
        "chain_id",
        "owner_did",
        "account",
        "asset_id",
        "balance",
        "consumed",
    ] {
        assert_eq!(recovered[field], result[field], "recovered {field}");
    }
}

#[test]
fn identity_first_batch_refuses_recovered_spending_history() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    let mut record = snapshot(17);
    record["calls"][4]["result"] = json!(format!("0x{:064x}", 7));
    write_snapshot(&store, &record);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    assert_refused(&peer.input(), NOW);
    drop(peer);
    record["height"] = json!(42);
    record["block"] = snapshot(42)["block"].clone();
    write_snapshot(&store, &record);
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    assert_refused(&peer.input(), NOW);
    let persisted: Value = must(serde_json::from_slice(&must(fs::read(&store))));
    assert_eq!(persisted["calls"][4]["result"], format!("0x{:064x}", 7));
}

#[test]
fn identity_first_batch_preserves_identity_and_canonical_state_refusals() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    for (path, value) in [
        ("/account/account/bound", json!(false)),
        (
            "/account/account/layerx_did",
            json!(format!("did:layerx:{}", "ab".repeat(32))),
        ),
        ("/account/account/layerx_account", json!("ab".repeat(32))),
        (
            "/account/account/evm_address",
            json!(format!("0x{}", "00".repeat(20))),
        ),
        (
            "/account/paxeer/address",
            json!(format!("0x{}", "ab".repeat(20))),
        ),
        ("/calls/0/result", json!(format!("0x{}", "ab".repeat(32)))),
        ("/calls/1/result", json!(format!("0x{}", "ab".repeat(32)))),
        ("/calls/2/result", json!(format!("0x{}", "ab".repeat(32)))),
        ("/calls/3/result", json!(format!("0x{}", "ff".repeat(32)))),
        ("/calls/4/result", json!(format!("0x{}00", "00".repeat(32)))),
        ("/calls/4/result", json!("0x")),
        ("/block/hash", json!(format!("0x{}", "00".repeat(32)))),
        ("/block/stateRoot", json!(format!("0x{}", "00".repeat(32)))),
    ] {
        let mut record = snapshot(17);
        *record.pointer_mut(path).unwrap_or_else(|| panic!("{path}")) = value;
        write_snapshot(&store, &record);
        let peer = TranscriptPeer::reopen(&store, Damage::None);
        assert_refused(&peer.input(), NOW);
    }
    write_snapshot(&store, &snapshot(17));
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    for (field, value) in [
        (
            "owner_did",
            json!(format!("did:layerx:{}", "00".repeat(32))),
        ),
        (
            "owner_did",
            json!(format!("did:layerx:{}", "AB".repeat(32))),
        ),
        ("owner_account", json!("ab".repeat(32))),
        ("asset_id", json!("ab".repeat(32))),
        ("chain_id", json!(126)),
        ("maximum_age_seconds", json!(0)),
        ("maximum_age_seconds", json!(301)),
        ("head", json!({"current":true,"consumed":0})),
    ] {
        let mut input = peer.input();
        input[field] = value;
        assert_refused(&input, NOW);
    }
}

#[test]
fn identity_first_batch_preserves_authenticated_current_head_refusals() {
    let directory = must(tempfile::tempdir());
    let store = directory.path().join("committed.json");
    write_snapshot(&store, &snapshot(17));
    for damage in [
        Damage::RpcError,
        Damage::WrongResponseId,
        Damage::AdvancingHead,
        Damage::ChangedHash,
        Damage::ChangedRoot,
    ] {
        let peer = TranscriptPeer::reopen(&store, damage);
        assert_refused(&peer.input(), NOW);
    }
    let peer = TranscriptPeer::reopen(&store, Damage::None);
    assert_refused(&peer.input(), NOW + 60_001);
    assert_refused(&peer.input(), NOW - 1);
    for url in [
        "http://chain.example.test/",
        "https://chain.example.test/",
        "file:///tmp/chain",
    ] {
        let mut input = peer.input();
        input["rpc_url"] = json!(url);
        assert_refused(&input, NOW);
    }
    for input in [
        json!({}),
        json!([]),
        json!({"head":null}),
        json!({"network_id":125,"sequencer_id":"61".repeat(32),"public_key":"61".repeat(32),"head":{"current":true}}),
    ] {
        assert_refused(&input, NOW);
    }
    let mut output = Vec::new();
    assert!(validate_account_head(&vec![b' '; 1_048_577][..], &mut output, NOW).is_err());
    assert!(output.is_empty());
}

#[test]
fn identity_first_batch_hosted_job_observes_after_merged_registration() {
    let script = include_str!("../../../../platform/hosted/human/provision.sh");
    assert!(!script.contains("/v1/protocol/account-state/head"));
    assert!(script.contains("pod['containers'] = [c for c in pod['containers'] if c['name'] != 'validate-account-head']"));
    let native = script
        .split("human_native_provision() (")
        .nth(1)
        .unwrap_or_else(|| panic!("native provisioning"));
    let produce = native
        .find("--produce-owner-registration")
        .unwrap_or_else(|| panic!("produce"));
    let validate = native
        .find("--validate-owner-registration")
        .unwrap_or_else(|| panic!("validate"));
    let observe = native
        .find("human_account_head_observe\n")
        .unwrap_or_else(|| panic!("observe"));
    assert!(produce < validate && validate < observe);
    assert!(script.contains("pod['containers'] = [validator]"));
    assert!(script.contains("rpc_ca_der_hex=[ssl.PEM_cert_to_DER_cert(ca).hex()]"));
    assert!(script.contains("dict(consumed=result['consumed'])"));
    assert!(script.contains("result['account'] == request['owner_account']"));
}
