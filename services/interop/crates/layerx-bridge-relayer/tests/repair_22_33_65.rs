use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_bridge_relayer::journal::{Entry, Journal};
use layerx_bridge_relayer::relayer::RelayerError;
use layerx_bridge_relayer::rpc::{PaxeerRpc, RpcFault};
use layerx_bridge_relayer::solana::journal::BackfillJournal;
use layerx_bridge_relayer::solana::observe::{
    observe_deposits, observe_deposits_pass, SolanaSettings, MAX_SCAN_PAGES, SIGNATURE_PAGE,
};
use layerx_bridge_relayer::solana::rpc::{Commitment, SolanaRpc};
use layerx_bridge_relayer::solana::{base58_encode, base58_fixed, SOLANA_CHAIN_ID};
use layerx_bridge_relayer::transport::PaxeerEndpoint;
use serde_json::{json, Value};

// The deposit transactions, instruction bytes, logs and account bytes are the
// committed custody producer transcript used by the preceding repair.
const TRANSCRIPT: &str = include_str!("../src/solana/fixtures/caught_cpi.json");
type Requests = Arc<Mutex<Vec<(String, Value)>>>;

struct FixtureServer {
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    requests: Requests,
    growth: Arc<AtomicUsize>,
    fail_transaction: Arc<AtomicBool>,
    malformed_page: Arc<AtomicBool>,
}

fn signature(index: usize) -> String {
    let mut bytes = [0xabu8; 64];
    bytes[..8].copy_from_slice(&(index as u64).to_be_bytes());
    base58_encode(&bytes)
}

fn failed_signature(index: usize, slot: u64) -> Value {
    json!({"signature": signature(index), "slot": slot, "err": {"InstructionError": [0, "InvalidArgument"]}})
}

fn settings() -> SolanaSettings {
    let fixture: Value = serde_json::from_str(TRANSCRIPT).or_panic("expected Ok value");
    SolanaSettings {
        chain_id: SOLANA_CHAIN_ID,
        vault: layerx_bridge_relayer::hex::fixed("0x334121a65b47bd45c3f6381537d9180e98e445bc")
            .or_panic("expected Ok value"),
        program_id: base58_fixed(fixture["program"].as_str().or_panic("expected Some value"))
            .or_panic("expected Ok value"),
        finality_depth: 0,
        start_slot: 1050,
        max_slot_range: 41,
        commitment: Commitment::Finalized,
    }
}

impl FixtureServer {
    fn start(large: bool) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").or_panic("expected Ok value");
        let address = listener.local_addr().or_panic("expected Ok value");
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let requests: Requests = Arc::new(Mutex::new(Vec::new()));
        let received = Arc::clone(&requests);
        let growth = Arc::new(AtomicUsize::new(0));
        let growing = Arc::clone(&growth);
        let fail_transaction = Arc::new(AtomicBool::new(false));
        let fail = Arc::clone(&fail_transaction);
        let malformed_page = Arc::new(AtomicBool::new(false));
        let malformed = Arc::clone(&malformed_page);
        let worker = thread::spawn(move || {
            let fixture: Value = serde_json::from_str(TRANSCRIPT).or_panic("expected Ok value");
            let history = signature_history(&fixture, large);
            for connection in listener.incoming() {
                let mut stream = connection.or_panic("expected Ok value");
                if stop.load(Ordering::Acquire) {
                    break;
                }
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .or_panic("expected Ok value");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .or_panic("expected Ok value");
                let mut reader = BufReader::new(stream.try_clone().or_panic("expected Ok value"));
                let mut first = String::new();
                reader.read_line(&mut first).or_panic("expected Ok value");
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).or_panic("expected Ok value"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((key, value)) = line.trim().split_once(':') {
                        if key.eq_ignore_ascii_case("content-length") {
                            assert!(length.is_none());
                            length =
                                Some(value.trim().parse::<usize>().unwrap_or_else(|error| {
                                    panic!("expected Ok value: {error:?}")
                                }));
                        }
                    }
                }
                let length = length.or_panic("expected Some value");
                assert!(length <= 16_384);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).or_panic("expected Ok value");
                let request: Value = serde_json::from_slice(&body).or_panic("expected Ok value");
                let method = request["method"].as_str().or_panic("expected Some value");
                let params = &request["params"];
                received
                    .lock()
                    .or_panic("expected Ok value")
                    .push((method.to_owned(), params.clone()));
                let result = transcript_result(
                    &fixture, &history, &growing, &malformed, &fail, method, params,
                );
                let body = serde_json::to_vec(
                    &json!({"jsonrpc": "2.0", "id": request["id"], "result": result}),
                )
                .or_panic("expected Ok value");
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).or_panic("expected Ok value");
                stream.write_all(&body).or_panic("expected Ok value");
            }
        });
        Self {
            address,
            stopped,
            worker: Some(worker),
            requests,
            growth,
            fail_transaction,
            malformed_page,
        }
    }

    fn rpc(&self) -> SolanaRpc {
        SolanaRpc::new(Box::new(
            PaxeerRpc::new(vec![PaxeerEndpoint::new(
                format!("http://{}/", self.address),
                Duration::from_secs(5),
                None,
                SOLANA_CHAIN_ID,
                true,
            )
            .or_panic("expected Ok value")])
            .or_panic("expected Ok value"),
        ))
    }
}

fn signature_history(fixture: &Value, large: bool) -> Vec<Value> {
    // Successful deposits retain their producer transcript unchanged.
    // Failed signature metadata pads newer history and a single busy
    // slot, so neither a slot bound nor slot-only continuation suffices.
    let mut history = Vec::new();
    let newer = if large {
        MAX_SCAN_PAGES * SIGNATURE_PAGE + 17
    } else {
        0
    };
    let same_slot = if large {
        MAX_SCAN_PAGES * SIGNATURE_PAGE + 19
    } else {
        0
    };
    for index in 0..newer {
        history.push(failed_signature(index, 2000));
    }
    let deposit = |index: usize| {
        json!({
            "signature": fixture["transactions"][index]["transaction"]["signatures"][0],
            "slot": fixture["transactions"][index]["slot"], "err": null,
        })
    };
    history.push(deposit(1));
    for index in 0..same_slot {
        history.push(failed_signature(newer + index, 1090));
    }
    history.push(deposit(0));
    history.push(failed_signature(newer + same_slot, 1049));
    history
}

fn transcript_result(
    fixture: &Value,
    history: &[Value],
    growing: &AtomicUsize,
    malformed: &AtomicBool,
    fail: &AtomicBool,
    method: &str,
    params: &Value,
) -> Value {
    match method {
        "eth_chainId" => json!(format!("0x{SOLANA_CHAIN_ID:x}")),
        "getSlot" => {
            assert_eq!(params, &json!([{"commitment": "finalized"}]));
            json!(3000 + growing.load(Ordering::Acquire))
        }
        "getSignaturesForAddress" => {
            assert_eq!(params[0], fixture["program"]);
            assert_eq!(params[1]["limit"], json!(SIGNATURE_PAGE));
            assert_eq!(params[1]["commitment"], "finalized");
            let added = growing.load(Ordering::Acquire);
            let mut live: Vec<Value> = (0..added)
                .map(|index| failed_signature(100_000 + index, 3000))
                .collect();
            live.extend(history.iter().cloned());
            let start = params[1].get("before").map_or(0, |before| {
                live.iter()
                    .position(|info| &info["signature"] == before)
                    .or_panic("expected Some value")
                    + 1
            });
            let mut page: Vec<_> = live.into_iter().skip(start).take(SIGNATURE_PAGE).collect();
            if malformed.load(Ordering::Acquire) && !page.is_empty() {
                if let Some(before) = params[1].get("before") {
                    page[0]["signature"] = before.clone();
                } else {
                    page[1] = page[0].clone();
                }
            }
            json!(page)
        }
        "getTransaction" => {
            assert_eq!(
                params[1],
                json!({"commitment": "finalized", "encoding": "json", "maxSupportedTransactionVersion": 0})
            );
            let tx = fixture["transactions"]
                .as_array()
                .or_panic("expected Some value")
                .iter()
                .find(|tx| tx["transaction"]["signatures"][0] == params[0])
                .or_panic("expected Some value");
            if tx["slot"] == 1090 && fail.swap(false, Ordering::AcqRel) {
                Value::Null
            } else {
                tx.clone()
            }
        }
        "getAccountInfo" => {
            assert_eq!(
                params[1],
                json!({"commitment": "finalized", "encoding": "base64"})
            );
            fixture["accounts"][params[0].as_str().or_panic("expected Some value")].clone()
        }
        _ => panic!("unexpected fixture RPC method {method}"),
    }
}

impl Drop for FixtureServer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        TcpStream::connect_timeout(&self.address, Duration::from_secs(5))
            .or_panic("expected Ok value");
        self.worker
            .take()
            .or_panic("expected Some value")
            .join()
            .or_panic("expected Ok value");
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new(name: &str) -> Self {
        let path =
            std::env::temp_dir().join(format!("repair-22-33-65-{}-{name}", std::process::id()));
        std::fs::create_dir(&path).or_panic("expected Ok value");
        Self(path)
    }
    fn open(&self) -> (Journal, BackfillJournal) {
        (
            Journal::open(&self.0.join("relay.jsonl")).or_panic("expected Ok value"),
            BackfillJournal::open(&self.0.join("backfill.json")).or_panic("expected Ok value"),
        )
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).or_panic("expected Ok value");
    }
}

#[test]
fn solana_scan_budget_paginated_catch_up_survives_restart_and_growing_head() {
    let server = FixtureServer::start(true);
    let directory = Directory::new("catch-up");
    let settings = settings();
    let stream = format!("in:{SOLANA_CHAIN_ID}");
    let mut expected_before = None;
    for pass in 0..3 {
        // Both production consumers are dropped and reopened between passes.
        let (mut journal, mut backfill) = directory.open();
        assert_eq!(
            backfill.pending().and_then(|window| window.before),
            expected_before
        );
        if pass > 0 {
            let pending = backfill.pending().or_panic("expected Some value");
            assert_eq!((pending.from, pending.to), (1050, 1090));
            assert_eq!(pending.program_id, settings.program_id);
            assert_eq!(pending.chain_id, settings.chain_id);
            assert_eq!(pending.vault, settings.vault);
            assert_eq!(pending.commitment, "finalized");
        }
        let progress = observe_deposits_pass(&server.rpc(), &settings, &mut journal, &mut backfill)
            .or_panic("expected Ok value");
        assert!(progress.pages <= MAX_SCAN_PAGES);
        assert!(progress.transactions <= MAX_SCAN_PAGES * SIGNATURE_PAGE);
        if pass < 2 {
            assert!(!progress.complete);
            assert_eq!(progress.pages, MAX_SCAN_PAGES);
            assert!(!journal.state().cursors.contains_key(&stream));
            assert_eq!(journal.state().items.len(), pass);
            let pending = backfill.pending().or_panic("expected Some value");
            expected_before = pending.before;
            assert!(expected_before.is_some());
        } else {
            assert!(progress.complete);
            assert_eq!(progress.pages, 1);
            assert!(backfill.pending().is_none());
            assert_eq!(journal.state().cursors[&stream], 1091);
            assert_eq!(journal.state().items.len(), 2);
            let mut nonces: Vec<_> = journal.state().items.values().map(|item| {
                assert!(item.refusal.is_none());
                let attestation = item.observation.inbound_attestation().or_panic("expected Some value");
                assert_eq!(attestation.vault, settings.vault);
                assert!(matches!(item.observation, layerx_bridge_relayer::journal::Observation::Inbound { position, .. }
                    if position.block_number == 1050 || position.block_number == 1090));
                attestation.log_index
            }).collect();
            nonces.sort_unstable();
            assert_eq!(nonces, vec![7, 9]);
        }
        server
            .growth
            .fetch_add(SIGNATURE_PAGE + 1, Ordering::AcqRel);
    }
    let (journal, backfill) = directory.open();
    assert!(backfill.pending().is_none());
    assert_eq!(journal.state().cursors[&stream], 1091);
    assert_eq!(journal.state().items.len(), 2);
    let requests = server.requests.lock().or_panic("expected Ok value");
    assert_eq!(
        requests
            .iter()
            .filter(|(method, _)| method == "getSlot")
            .count(),
        1
    );
    let pages: Vec<_> = requests
        .iter()
        .filter(|(method, _)| method == "getSignaturesForAddress")
        .collect();
    assert_eq!(pages.len(), 5);
    assert!(pages[0].1[1].get("before").is_none());
    assert!(pages[1..]
        .iter()
        .all(|(_, params)| params[1].get("before").is_some()));
    assert_eq!(
        requests
            .iter()
            .filter(|(method, _)| method == "getTransaction")
            .count(),
        2
    );
}

#[test]
fn solana_scan_budget_replays_partial_page_without_losing_observations() {
    let server = FixtureServer::start(false);
    let directory = Directory::new("partial-page");
    let settings = settings();
    server.fail_transaction.store(true, Ordering::Release);
    {
        let (mut journal, mut backfill) = directory.open();
        assert_eq!(
            observe_deposits_pass(&server.rpc(), &settings, &mut journal, &mut backfill),
            Err(RelayerError::Rpc(RpcFault::Unavailable))
        );
        assert_eq!(journal.state().items.len(), 1);
        assert!(journal.state().cursors.is_empty());
        let pending = backfill.pending().or_panic("expected Some value");
        assert_eq!((pending.from, pending.to), (1050, 1090));
        assert!(pending.before.is_none());
    }
    // An interrupted atomic replacement leaves the last synced checkpoint authoritative.
    std::fs::write(directory.0.join("backfill.json.tmp"), b"{torn").or_panic("expected Ok value");
    let (mut journal, mut backfill) = directory.open();
    let progress = observe_deposits_pass(&server.rpc(), &settings, &mut journal, &mut backfill)
        .or_panic("expected Ok value");
    assert!(progress.complete);
    assert_eq!(
        progress.observed, 1,
        "the already synced observation is replayed idempotently"
    );
    assert_eq!(journal.state().items.len(), 2);
    assert_eq!(
        journal.state().cursors[&format!("in:{SOLANA_CHAIN_ID}")],
        1091
    );
    let lines =
        std::fs::read_to_string(directory.0.join("relay.jsonl")).or_panic("expected Ok value");
    assert_eq!(
        lines
            .lines()
            .filter(|line| line.contains("\"kind\":\"observed\""))
            .count(),
        2
    );
    drop(journal);
    drop(backfill);
    let (journal, backfill) = directory.open();
    assert_eq!(journal.state().items.len(), 2);
    assert!(backfill.pending().is_none());
}

#[test]
fn solana_scan_budget_rejects_identity_change_and_bad_pagination() {
    let server = FixtureServer::start(true);
    let directory = Directory::new("identity");
    let settings = settings();
    let (mut journal, mut backfill) = directory.open();
    observe_deposits_pass(&server.rpc(), &settings, &mut journal, &mut backfill)
        .or_panic("expected Ok value");
    let saved = backfill.pending().cloned();
    let request_count = server.requests.lock().or_panic("expected Ok value").len();
    for changed in [
        SolanaSettings {
            program_id: [1; 32],
            ..settings
        },
        SolanaSettings {
            chain_id: settings.chain_id + 1,
            ..settings
        },
        SolanaSettings {
            vault: [1; 20],
            ..settings
        },
        SolanaSettings {
            commitment: Commitment::Confirmed,
            ..settings
        },
    ] {
        assert!(matches!(
            observe_deposits_pass(&server.rpc(), &changed, &mut journal, &mut backfill),
            Err(RelayerError::Journal(_))
        ));
    }
    assert_eq!(
        server.requests.lock().or_panic("expected Ok value").len(),
        request_count
    );
    server.malformed_page.store(true, Ordering::Release);
    assert_eq!(
        observe_deposits_pass(&server.rpc(), &settings, &mut journal, &mut backfill),
        Err(RelayerError::Rpc(RpcFault::Malformed))
    );
    assert_eq!(backfill.pending(), saved.as_ref());
    assert!(journal.state().cursors.is_empty());
    drop(journal);
    drop(backfill);
    let (mut journal, mut backfill) = directory.open();
    assert_eq!(backfill.pending(), saved.as_ref());
    // A cursor synced before checkpoint cleanup recovers without querying the head.
    journal
        .append(&Entry::Cursor {
            stream: format!("in:{SOLANA_CHAIN_ID}"),
            next_block: 1091,
        })
        .or_panic("expected Ok value");
    server.malformed_page.store(false, Ordering::Release);
    let request_count = server.requests.lock().or_panic("expected Ok value").len();
    assert!(
        observe_deposits_pass(&server.rpc(), &settings, &mut journal, &mut backfill)
            .or_panic("expected Ok value")
            .complete
    );
    assert_eq!(
        server.requests.lock().or_panic("expected Ok value").len(),
        request_count
    );
    assert!(backfill.pending().is_none());
}

#[test]
fn solana_scan_budget_legacy_scan_is_bounded_and_never_returns_partial_success() {
    let server = FixtureServer::start(true);
    let result = observe_deposits(&server.rpc(), &settings(), 1050, 1090);
    assert!(
        matches!(result, Err(RelayerError::Configuration(message)) if message.contains("budget exhausted"))
    );
    let requests = server.requests.lock().or_panic("expected Ok value");
    assert_eq!(
        requests
            .iter()
            .filter(|(method, _)| method == "getSignaturesForAddress")
            .count(),
        MAX_SCAN_PAGES
    );
    assert!(requests
        .iter()
        .all(|(method, _)| method != "getTransaction"));
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
