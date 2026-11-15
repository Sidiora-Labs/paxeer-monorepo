use std::fmt::Debug;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use serde_json::{json, Value};

const FIXTURE: &str = include_str!("../../src/fixtures/account_head.json");

pub(crate) fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

pub(crate) fn fixture() -> Value {
    must(serde_json::from_str(FIXTURE))
}

pub(crate) fn text(value: &Value) -> &str {
    value
        .as_str()
        .unwrap_or_else(|| panic!("expected text: {value}"))
}

pub(crate) fn snapshot_at(height: u64, now_ms: u64) -> Value {
    let mut record = fixture();
    record["height"] = json!(height);
    record["block"] = json!({
        "number": format!("{height:#x}"), "hash": format!("0x{height:064x}"),
        "parentHash": format!("0x{:064x}", height - 1), "timestamp": format!("{:#x}", now_ms / 1000),
        "stateRoot": format!("0x{:064x}", height + 100),
        "transactionsRoot": format!("0x{}", "11".repeat(32)),
        "receiptsRoot": format!("0x{}", "22".repeat(32)), "transactions": []
    });
    record["account"]["paxeer"]["height"] = json!(height);
    record
}

#[derive(Clone, Copy)]
pub(crate) enum Damage {
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

// The peer serves the committed ABI codec artifact through the production
// SequencerEndpoint and PxEndpoint transports and decoders.
pub(crate) struct TranscriptPeer {
    url: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    pub(crate) requests: Arc<Mutex<Vec<Value>>>,
}

impl TranscriptPeer {
    pub(crate) fn reopen(path: &Path, damage: Damage) -> Self {
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

    pub(crate) fn input(&self) -> Value {
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

pub(crate) fn write_snapshot(path: &Path, value: &Value) {
    must(fs::write(path, must(serde_json::to_vec(value))));
}
