use crate::{health, now_ms, round, Config, MarketConfig};

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Condvar, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_oracle_feeder::chainflow::{ChainflowConfig, Ticker};
use layerx_oracle_feeder::cosmos_tx::{feed_account, Submission};
use layerx_oracle_feeder::{hex_decode, hex_encode};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use zeroize::Zeroizing;

pub(crate) const NOW: u64 = 1_700_000_000_500;
pub(crate) const FEED: &str = "79b000887626b294a914501a4cd226b58b235983";
const ACCOUNT: &str = r#"{"response":{"code":0,"value":"ClQKIC9jb3Ntb3MuYXV0aC52MWJldGExLkJhc2VBY2NvdW50EjAKKnBheDEweGNxcHpya3k2ZWZmMmc1MnFkeWU1M3hrazlqeGt2cmEwOHludBgJIAM=","height":"811"}}"#;

pub(crate) type TestResult = Result<(), String>;

#[derive(Clone, Copy)]
pub(crate) enum Fate {
    Pending,
    Committed { code: u64, height: u64 },
    Evicted,
    LookupUnavailable,
    IncompleteMempool,
    MalformedExecutionCode,
    WrongTransaction,
    CommitDuringLookup,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum SubmissionSequenceMode {
    Any,
    CommittedOnly,
}

pub(crate) struct Transcript {
    pub(crate) account_sequence: u8,
    pub(crate) observation_sequence: u64,
    pub(crate) fate: Fate,
    pub(crate) tx_fates: Vec<Fate>,
    pub(crate) txs: Vec<Vec<u8>>,
    pub(crate) ticker_requests: usize,
    pub(crate) fresh_quotes: bool,
    pub(crate) api_keys: Vec<String>,
    pub(crate) redirect: Option<String>,
    pub(crate) lose_ack: bool,
    pub(crate) tx_lookups: usize,
    pub(crate) lookup_error: Option<Value>,
    pub(crate) committed_height: Option<u64>,
    pub(crate) advance_height_on_account_read: bool,
    pub(crate) mempool: Option<Vec<Vec<u8>>>,
    pub(crate) mempool_pages: Vec<u64>,
    pub(crate) account_reads: Vec<u64>,
    pub(crate) observation_reads: Vec<(u64, u64)>,
    pub(crate) observation_busy_reads: Vec<(u64, u64)>,
    pub(crate) observation_error: Option<Value>,
    pub(crate) observation_sequences: std::collections::BTreeMap<u64, u64>,
    pub(crate) response_delay_ms: u64,
    pub(crate) submission_sequence_mode: SubmissionSequenceMode,
    pub(crate) sequence_refusals: usize,
    pub(crate) broadcast_refusal: Option<String>,
}

impl Default for Transcript {
    fn default() -> Self {
        Self {
            account_sequence: 3,
            observation_sequence: 0,
            fate: Fate::Pending,
            tx_fates: Vec::new(),
            txs: Vec::new(),
            ticker_requests: 0,
            fresh_quotes: false,
            api_keys: Vec::new(),
            redirect: None,
            lose_ack: false,
            tx_lookups: 0,
            lookup_error: None,
            committed_height: None,
            advance_height_on_account_read: false,
            mempool: None,
            mempool_pages: Vec::new(),
            account_reads: Vec::new(),
            observation_reads: Vec::new(),
            observation_busy_reads: Vec::new(),
            observation_error: None,
            observation_sequences: std::collections::BTreeMap::new(),
            response_delay_ms: 0,
            submission_sequence_mode: SubmissionSequenceMode::Any,
            sequence_refusals: 0,
            broadcast_refusal: None,
        }
    }
}

/// A committed protocol transcript served over real HTTP sockets. Broadcast
/// replies and later lookups bind to the actual bytes produced by `encode_tx`.
/// Response shapes follow consensus/rpc/coretypes, internal/rpc/core/tx.go
/// and abci/types/ExecTxResult (including its omitted zero success code).
pub(crate) struct Peer {
    pub(crate) url: String,
    pub(crate) state: Arc<Mutex<Transcript>>,
    observation_gate: Arc<ObservationGate>,
    stop: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
}

struct ObservationGate {
    state: Mutex<ObservationGateState>,
    changed: Condvar,
}

#[derive(Default)]
struct ObservationGateState {
    expected: usize,
    in_flight: usize,
    highest_in_flight: usize,
    busy_limit: Option<usize>,
    bound_passed: bool,
}

struct ObservationPermit {
    gate: Arc<ObservationGate>,
    busy: bool,
}

impl ObservationGate {
    fn new() -> Self {
        Self {
            state: Mutex::new(ObservationGateState::default()),
            changed: Condvar::new(),
        }
    }

    fn configure(&self, expected: usize) {
        let mut state = self
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.expected = expected;
        state.in_flight = 0;
        state.highest_in_flight = 0;
        state.bound_passed = false;
        self.changed.notify_all();
    }

    fn enter(self: &Arc<Self>) -> ObservationPermit {
        let deadline = Instant::now() + Duration::from_secs(10);
        let mut state = self
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.in_flight += 1;
        state.highest_in_flight = state.highest_in_flight.max(state.in_flight);
        let busy = state
            .busy_limit
            .is_some_and(|limit| state.in_flight > limit);
        if state.expected != 0 && state.in_flight >= state.expected {
            state.bound_passed = true;
            self.changed.notify_all();
        }
        while state.expected != 0 && !state.bound_passed {
            let remaining = deadline.saturating_duration_since(Instant::now());
            if remaining.is_zero() {
                break;
            }
            let (next, result) = self
                .changed
                .wait_timeout(state, remaining)
                .unwrap_or_else(|error| panic!("{error:?}"));
            state = next;
            if result.timed_out() {
                break;
            }
        }
        ObservationPermit {
            gate: Arc::clone(self),
            busy,
        }
    }

    fn bound_passed(&self) -> bool {
        self.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .bound_passed
    }
}

impl Drop for ObservationPermit {
    fn drop(&mut self) {
        let mut state = self
            .gate
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.in_flight = state.in_flight.saturating_sub(1);
        self.gate.changed.notify_all();
    }
}

fn read_request(stream: &mut TcpStream) -> (String, Vec<u8>) {
    stream
        .set_read_timeout(Some(Duration::from_secs(3)))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut bytes = Vec::new();
    let mut buf = [0; 4096];
    loop {
        let count = stream
            .read(&mut buf)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_ne!(count, 0, "HTTP request truncated");
        bytes.extend_from_slice(&buf[..count]);
        if let Some(at) = bytes.windows(4).position(|b| b == b"\r\n\r\n") {
            let header =
                String::from_utf8(bytes[..at].to_vec()).unwrap_or_else(|error| panic!("{error:?}"));
            let size = header
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length").then(|| {
                        value
                            .trim()
                            .parse::<usize>()
                            .unwrap_or_else(|error| panic!("{error:?}"))
                    })
                })
                .unwrap_or(0);
            if bytes.len() >= at + 4 + size {
                return (header, bytes[at + 4..at + 4 + size].to_vec());
            }
        }
    }
}

impl Peer {
    pub(crate) fn new() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error:?}"));
        let url = format!(
            "http://{}",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("{error:?}"))
        );
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{error:?}"));
        let state = Arc::new(Mutex::new(Transcript::default()));
        let observation_gate = Arc::new(ObservationGate::new());
        let stop = Arc::new(AtomicBool::new(false));
        let shared = Arc::clone(&state);
        let gate = Arc::clone(&observation_gate);
        let stopped = Arc::clone(&stop);
        let handle = thread::spawn(move || {
            while !stopped.load(Ordering::SeqCst) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        let shared = Arc::clone(&shared);
                        let gate = Arc::clone(&gate);
                        thread::spawn(move || serve_connection(stream, shared, gate));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("fixture listener: {error}"),
                }
            }
        });
        Self {
            url,
            state,
            observation_gate,
            stop,
            handle: Some(handle),
        }
    }

    pub(crate) fn rpc(&self) -> round::Rpc {
        round::Rpc::new(agent(), self.url.clone(), self.url.clone())
            .unwrap_or_else(|error| panic!("{error:?}"))
    }

    pub(crate) fn hold_observation_reads_until(&self, markets: usize) {
        self.observation_gate.configure(markets);
    }

    pub(crate) fn observation_bound_passed(&self) -> bool {
        self.observation_gate.bound_passed()
    }

    pub(crate) fn limit_observation_reads(&self, limit: usize) {
        self.observation_gate.state.lock().unwrap().busy_limit = Some(limit);
    }

    pub(crate) fn highest_observation_reads_in_flight(&self) -> usize {
        self.observation_gate
            .state
            .lock()
            .unwrap()
            .highest_in_flight
    }

    pub(crate) fn serve_fresh_quotes(&self) {
        self.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .fresh_quotes = true;
    }

    pub(crate) fn disable_transaction_lookup(&self) {
        self.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .lookup_error = Some(json!({
            "code": -32603,
            "message": "Internal error",
            "data": "transaction querying is disabled",
        }));
    }

    pub(crate) fn admit_only_committed_sequence(&self) {
        self.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .submission_sequence_mode = SubmissionSequenceMode::CommittedOnly;
    }

    pub(crate) fn commit_submission(&self, index: usize, height: u64) {
        let mut state = self
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert!(index < state.txs.len(), "commit follows a broadcast");
        let previous = state.tx_fates.get(index).copied().unwrap_or(state.fate);
        if !matches!(previous, Fate::Committed { .. }) {
            state.account_sequence = state.account_sequence.saturating_add(1);
            state.observation_sequence = state.observation_sequence.saturating_add(1);
        }
        if state.tx_fates.len() <= index {
            state.tx_fates.resize(index + 1, Fate::Pending);
        }
        state.tx_fates[index] = Fate::Committed { code: 0, height };
        state.committed_height = Some(height);
    }
}

fn serve_connection(
    mut stream: TcpStream,
    shared: Arc<Mutex<Transcript>>,
    observation_gate: Arc<ObservationGate>,
) {
    let (header, body) = read_request(&mut stream);
    let mut observation_permit = None;
    let (status, extra, body) = if header.starts_with("GET ") {
        let mut transcript = shared.lock().unwrap_or_else(|error| panic!("{error:?}"));
        transcript.ticker_requests += 1;
        let key = header
            .lines()
            .find_map(|line| {
                let (name, value) = line.split_once(':')?;
                name.eq_ignore_ascii_case("x-api-key")
                    .then(|| value.trim().to_owned())
            })
            .unwrap_or_else(|| panic!("authenticated quote request"));
        transcript.api_keys.push(key);
        let path = header
            .split_whitespace()
            .nth(1)
            .unwrap_or_else(|| panic!("ticker path is absent"));
        let symbol = path
            .strip_prefix("/api/markets/ticker/")
            .unwrap_or_else(|| panic!("unexpected ticker path {path}"));
        if let Some(location) = &transcript.redirect {
            (302, format!("Location: {location}\r\n"), String::new())
        } else {
            let mut ticker: Value =
                serde_json::from_str(include_str!("../fixtures/chainflow_ticker_btc.json"))
                    .unwrap_or_else(|error| panic!("{error:?}"));
            ticker["symbol"] = json!(format!("PERP_{symbol}_USDC"));
            if transcript.fresh_quotes {
                ticker["updated_at"] = json!(now_ms());
            }
            (200, String::new(), ticker.to_string())
        }
    } else {
        assert!(header.starts_with("POST / "));
        let request: Value =
            serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(request["jsonrpc"], "2.0");
        observation_permit = (request["method"] == "eth_call"
            && request["params"][0]["data"]
                .as_str()
                .is_some_and(|data| data.starts_with("0x48f83666")))
        .then(|| observation_gate.enter());
        let mut transcript = shared.lock().unwrap_or_else(|error| panic!("{error:?}"));
        let result = transcript.answer(
            &request,
            observation_permit
                .as_ref()
                .is_some_and(|permit| permit.busy),
        );
        if transcript.lose_ack && request["method"] == "broadcast_tx_sync" {
            (503, String::new(), "acknowledgement lost".to_owned())
        } else {
            (200, String::new(), result.to_string())
        }
    };
    let response_delay_ms = shared
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .response_delay_ms;
    thread::sleep(Duration::from_millis(response_delay_ms));
    let response = format!("HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\n{extra}Content-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len());
    drop(observation_permit);
    stream
        .write_all(response.as_bytes())
        .unwrap_or_else(|error| panic!("{error:?}"));
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(handle) = self.handle.take() {
            let result = handle.join();
            if !thread::panicking() {
                result.unwrap_or_else(|error| panic!("{error:?}"));
            }
        }
    }
}

impl Transcript {
    fn height(&self) -> u64 {
        self.committed_height.unwrap_or(match self.fate {
            Fate::Committed { height, .. } => height + 1,
            _ => self
                .tx_fates
                .iter()
                .filter_map(|fate| match fate {
                    Fate::Committed { height, .. } => Some(*height + 1),
                    _ => None,
                })
                .max()
                .unwrap_or(830),
        })
    }

    fn account_response(&mut self, request: &Value) -> Value {
        assert_eq!(request["params"]["path"], round::ACCOUNT_QUERY_PATH);
        let mut account: Value =
            serde_json::from_str(ACCOUNT).unwrap_or_else(|error| panic!("{error:?}"));
        let mut bytes = STANDARD
            .decode(
                account["response"]["value"]
                    .as_str()
                    .unwrap_or_else(|| panic!("required protocol value is absent")),
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(bytes.pop(), Some(3));
        bytes.push(self.account_sequence);
        account["response"]["value"] = json!(STANDARD.encode(bytes));
        let height = self.height();
        self.account_reads.push(height);
        if self.advance_height_on_account_read {
            self.committed_height = Some(height + 1);
        }
        account["response"]["height"] = json!(height.to_string());
        account
    }

    fn answer(&mut self, request: &Value, observation_busy: bool) -> Value {
        let result = match request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("required protocol value is absent"))
        {
            "abci_query" => self.account_response(request),
            "eth_call" => {
                assert_eq!(
                    request["params"][0]["to"],
                    format!("0x{}", hex_encode(&round::PRICEFEED))
                );
                let data = request["params"][0]["data"]
                    .as_str()
                    .unwrap_or_else(|| panic!("required protocol value is absent"));
                if data.starts_with("0x007eb855") {
                    json!(format!("0x{}{}", "00".repeat(12), "22".repeat(20)))
                } else {
                    assert!(data.starts_with("0x48f83666"));
                    let height = *self
                        .account_reads
                        .last()
                        .unwrap_or_else(|| panic!("observation follows a committed account read"));
                    let market_id = u64::from_str_radix(&data[10 + 48..10 + 64], 16)
                        .unwrap_or_else(|error| panic!("{error:?}"));
                    self.observation_reads.push((market_id, height));
                    assert_eq!(request["params"][1], format!("0x{height:x}"));
                    if observation_busy {
                        self.observation_busy_reads.push((market_id, height));
                        return json!({"jsonrpc":"2.0","id":1,"error":{
                            "code": -32005, "message": "rejected due to rate limit"
                        }});
                    }
                    if let Some(error) = &self.observation_error {
                        return json!({"jsonrpc":"2.0","id":1,"error":error});
                    }
                    json!(format!(
                        "0x{:064x}{}",
                        self.observation_sequences
                            .get(&market_id)
                            .copied()
                            .unwrap_or(self.observation_sequence),
                        "00".repeat(128)
                    ))
                }
            }
            "broadcast_tx_sync" => {
                let tx = STANDARD
                    .decode(
                        request["params"]["tx"]
                            .as_str()
                            .unwrap_or_else(|| panic!("required protocol value is absent")),
                    )
                    .unwrap_or_else(|error| panic!("{error:?}"));
                assert!(!tx.is_empty());
                let hash = hex_encode(&Sha256::digest(&tx));
                let sequence = tx_sequence(&tx);
                let tx_index = self.txs.len();
                self.txs.push(tx);
                let fate = self.tx_fates.get(tx_index).copied().unwrap_or(self.fate);
                if self.broadcast_refusal.is_some()
                    || (self.submission_sequence_mode == SubmissionSequenceMode::CommittedOnly
                        && sequence != u64::from(self.account_sequence))
                {
                    self.sequence_refusals += 1;
                    return json!({
                        "jsonrpc": "2.0", "id": 1, "result": {
                        "code": 32,
                        "data": "",
                        "log": self.broadcast_refusal.clone().unwrap_or_else(|| format!(
                            "account sequence mismatch, expected {}, got {}: incorrect account sequence",
                            self.account_sequence, sequence
                        )),
                        "codespace": "sdk",
                        "hash": hash.to_uppercase(),
                        }
                    });
                }
                if matches!(fate, Fate::Committed { .. }) {
                    self.account_sequence = self.account_sequence.saturating_add(1);
                    self.observation_sequence = self.observation_sequence.saturating_add(1);
                }
                json!({"code":0,"log":"[]","codespace":"","hash":hash.to_uppercase()})
            }
            "tx" => {
                self.tx_lookups += 1;
                let (tx_index, tx) = self
                    .txs
                    .iter()
                    .enumerate()
                    .find(|(_, tx)| {
                        request["params"]["hash"] == hex_encode(&Sha256::digest(tx)).to_uppercase()
                    })
                    .unwrap_or_else(|| panic!("lookup follows real broadcast"));
                let hash = hex_encode(&Sha256::digest(tx));
                assert_eq!(request["params"]["hash"], hash.to_uppercase());
                if let Some(error) = &self.lookup_error {
                    return json!({"jsonrpc":"2.0","id":1,"error":error});
                }
                let fate = self.tx_fates.get(tx_index).copied().unwrap_or(self.fate);
                if matches!(fate, Fate::LookupUnavailable) {
                    return json!({"jsonrpc":"2.0","id":1,"error":{"code":-32603,"data":"transaction indexing is disabled"}});
                }
                let committed = match fate {
                    Fate::Committed { code, height } => Some((code, height)),
                    Fate::MalformedExecutionCode | Fate::WrongTransaction => Some((0, 820)),
                    Fate::CommitDuringLookup if self.tx_lookups >= 2 => Some((0, 820)),
                    _ => None,
                };
                if let Some((code, height)) = committed {
                    let mut result = json!({"hash":hash.to_uppercase(),"height":height.to_string(),"tx":STANDARD.encode(tx),"tx_result":{"code":code}});
                    if code == 0 {
                        result["tx_result"] = json!({"gas_used":"12000"});
                    }
                    if matches!(fate, Fate::MalformedExecutionCode) {
                        result["tx_result"] = json!({"code":"invalid"});
                    }
                    if matches!(fate, Fate::WrongTransaction) {
                        result["tx"] = json!(STANDARD.encode(b"another transaction"));
                    }
                    result
                } else {
                    return json!({"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Internal error","data":format!("tx ({}) not found, err: %!w(<nil>)",hash.to_uppercase())}});
                }
            }
            "unconfirmed_txs" => {
                let page = request["params"]["page"]
                    .as_u64()
                    .unwrap_or_else(|| panic!("mempool page is absent"));
                assert_eq!(request["params"]["per_page"], 100);
                self.mempool_pages.push(page);
                if let Some(txs) = &self.mempool {
                    let start = usize::try_from((page - 1) * 100)
                        .unwrap_or_else(|error| panic!("{error:?}"));
                    let page_txs: Vec<_> = txs
                        .iter()
                        .skip(start)
                        .take(100)
                        .map(|tx| STANDARD.encode(tx))
                        .collect();
                    return json!({"jsonrpc":"2.0","id":1,"result":{
                        "total": txs.len().to_string(),
                        "n_txs": page_txs.len().to_string(), "txs": page_txs
                    }});
                }
                assert_eq!(page, 1);
                if matches!(self.fate, Fate::Pending) {
                    let txs: Vec<_> = self.txs.iter().map(|tx| STANDARD.encode(tx)).collect();
                    json!({"total":txs.len().to_string(),"n_txs":txs.len().to_string(),"txs":txs})
                } else if matches!(self.fate, Fate::IncompleteMempool) {
                    json!({"total":"101","n_txs":"0","txs":null})
                } else {
                    json!({"total":"0","n_txs":"0","txs":null})
                }
            }
            method => panic!("unexpected protocol method {method}"),
        };
        json!({"jsonrpc":"2.0","id":1,"result":result})
    }
}

pub(crate) fn tx_sequence(tx: &[u8]) -> u64 {
    let auth_info = protobuf_bytes(tx, 2).unwrap_or_else(|| panic!("transaction auth info absent"));
    let signer_info =
        protobuf_bytes(auth_info, 1).unwrap_or_else(|| panic!("transaction signer info absent"));
    protobuf_field_varint(signer_info, 3).unwrap_or_else(|| panic!("transaction sequence absent"))
}

pub(crate) fn tx_observation_sequences(tx: &[u8]) -> Vec<(u64, u64)> {
    let body = protobuf_bytes(tx, 1).unwrap();
    let message = protobuf_bytes(body, 1).unwrap();
    let mut prices = protobuf_bytes(message, 2).unwrap();
    let mut observations = Vec::new();
    while !prices.is_empty() {
        let key = protobuf_varint(&mut prices).unwrap();
        assert_eq!(key & 7, 2);
        let length = usize::try_from(protobuf_varint(&mut prices).unwrap()).unwrap();
        let value = &prices[..length];
        prices = &prices[length..];
        if key >> 3 == 2 {
            observations.push((
                protobuf_field_varint(value, 1).unwrap_or(0),
                protobuf_field_varint(value, 2).unwrap(),
            ));
        }
    }
    observations
}

fn protobuf_bytes(mut input: &[u8], wanted: u64) -> Option<&[u8]> {
    while !input.is_empty() {
        let key = protobuf_varint(&mut input)?;
        let field = key >> 3;
        match key & 7 {
            0 => {
                protobuf_varint(&mut input)?;
                if field == wanted {
                    return None;
                }
            }
            2 => {
                let length = usize::try_from(protobuf_varint(&mut input)?).ok()?;
                if input.len() < length {
                    return None;
                }
                let value = &input[..length];
                input = &input[length..];
                if field == wanted {
                    return Some(value);
                }
            }
            _ => return None,
        }
    }
    None
}

fn protobuf_varint(input: &mut &[u8]) -> Option<u64> {
    let mut value = 0;
    for shift in (0..70).step_by(7) {
        let byte = *input.first()?;
        *input = &input[1..];
        value |= u64::from(byte & 0x7f) << shift;
        if byte & 0x80 == 0 {
            return Some(value);
        }
    }
    None
}

fn protobuf_field_varint(mut input: &[u8], wanted: u64) -> Option<u64> {
    while !input.is_empty() {
        let key = protobuf_varint(&mut input)?;
        let field = key >> 3;
        match key & 7 {
            0 => {
                let value = protobuf_varint(&mut input)?;
                if field == wanted {
                    return Some(value);
                }
            }
            2 => {
                let length = usize::try_from(protobuf_varint(&mut input)?).ok()?;
                if input.len() < length {
                    return None;
                }
                input = &input[length..];
            }
            _ => return None,
        }
    }
    None
}

pub(crate) struct StateDir(pub(crate) PathBuf);
impl StateDir {
    pub(crate) fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1412-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::SeqCst)
        ));
        std::fs::create_dir(&path).unwrap_or_else(|error| panic!("{error:?}"));
        Self(path)
    }
    pub(crate) fn progress(&self) -> Value {
        serde_json::from_slice(
            &std::fs::read(self.0.join("price-sidecar-progress.json"))
                .unwrap_or_else(|error| panic!("{error:?}")),
        )
        .unwrap_or_else(|error| panic!("{error:?}"))
    }
}
impl Drop for StateDir {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).unwrap_or_else(|error| panic!("{error:?}"));
    }
}

pub(crate) fn agent() -> ureq::Agent {
    ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(3)))
        .max_redirects(0)
        .build()
        .into()
}
pub(crate) fn config(peer: &Peer) -> Config {
    Config {
        chain_id: "test-1".to_owned(),
        validator: "22".repeat(20),
        feed_account: FEED.to_owned(),
        gas_limit: 200_000,
        cadence_ms: 1000,
        max_pending_submissions: 8,
        max_committed_reads_in_flight: 6,
        pending_submission_block_limit: round::PENDING_SUBMISSION_BLOCK_LIMIT,
        retry_backoff_ms: 1,
        retry_max_attempts: 2,
        chainflow: ChainflowConfig {
            base_url: peer.url.clone(),
            api_key_env: "UNUSED_FIXTURE_KEY".to_owned(),
            timeout_ms: 2000,
            max_quote_age_ms: 10_000,
        },
        markets: vec![MarketConfig {
            market_id: 0,
            symbol: "BTC".to_owned(),
        }],
    }
}
pub(crate) fn start(config: &Config, peer: &Peer, dir: &StateDir) -> round::Sidecar {
    round::start(config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)
        .unwrap_or_else(|error| panic!("{error:?}"))
}
pub(crate) fn ticker(config: &Config) -> Ticker {
    config
        .fetch_ticker_loopback(&agent(), "BTC", "fixture-credential", || NOW)
        .unwrap_or_else(|error| panic!("{error:?}"))
}
pub(crate) fn submit(
    config: &Config,
    peer: &Peer,
    dir: &StateDir,
    sidecar: &mut round::Sidecar,
    metrics: &Mutex<health::Metrics>,
) -> Submission {
    let quote = ticker(config);
    let quotes: Vec<_> = config
        .markets
        .iter()
        .map(|market| (market, Ok(quote.clone())))
        .collect();
    let observations = sidecar
        .collect(&quotes, &mut health::lock(metrics))
        .unwrap_or_else(|| panic!("collected quote is absent"));
    let account = round::parse_account(
        &peer
            .rpc()
            .query_account(&feed_account(
                &hex_decode(FEED)
                    .unwrap_or_else(|error| panic!("{error:?}"))
                    .try_into()
                    .unwrap_or_else(|error| panic!("{error:?}")),
            ))
            .unwrap_or_else(|error| panic!("{error:?}")),
        &feed_account(
            &hex_decode(FEED)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .try_into()
                .unwrap_or_else(|error| panic!("{error:?}")),
        ),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let submission = sidecar.next_submission(config, account, observations.clone());
    sidecar
        .submit(config, &peer.rpc(), &observations, metrics, &dir.0)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let expected = peer
        .state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .txs
        .last()
        .cloned()
        .unwrap_or_else(|| panic!("broadcast transaction is absent"));
    assert_eq!(
        dir.progress()["pending"]
            .as_array()
            .unwrap()
            .last()
            .unwrap()["tx"],
        STANDARD.encode(expected)
    );
    submission
}
pub(crate) fn assert_unconfirmed(metrics: &Mutex<health::Metrics>) {
    let metrics = health::lock(metrics);
    assert!(!metrics.is_ready(now_ms(), 1000));
    let text = metrics.metrics_text(now_ms());
    assert!(text.contains("price_sidecar_confirmed_height 0\n"));
    assert!(text.contains("price_sidecar_last_accepted_height 0\n"));
    assert!(
        health::respond(b"GET /readyz HTTP/1.1\r\n\r\n", &metrics, now_ms(), 1000)
            .starts_with("HTTP/1.1 503")
    );
}
