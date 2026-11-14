use crate::{health, now_ms, round, Config, MarketConfig};

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

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

pub(crate) struct Transcript {
    pub(crate) account_sequence: u8,
    pub(crate) observation_sequence: u64,
    pub(crate) fate: Fate,
    pub(crate) tx_fates: Vec<Fate>,
    pub(crate) txs: Vec<Vec<u8>>,
    pub(crate) ticker_requests: usize,
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
    pub(crate) observation_sequences: std::collections::BTreeMap<u64, u64>,
    pub(crate) response_delay_ms: u64,
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
            observation_sequences: std::collections::BTreeMap::new(),
            response_delay_ms: 0,
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
    stop: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
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
        let stop = Arc::new(AtomicBool::new(false));
        let shared = Arc::clone(&state);
        let stopped = Arc::clone(&stop);
        let handle = thread::spawn(move || {
            while !stopped.load(Ordering::SeqCst) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        let shared = Arc::clone(&shared);
                        thread::spawn(move || serve_connection(stream, shared));
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
            stop,
            handle: Some(handle),
        }
    }

    pub(crate) fn rpc(&self) -> round::Rpc {
        round::Rpc::new(agent(), self.url.clone(), self.url.clone())
            .unwrap_or_else(|error| panic!("{error:?}"))
    }
}

fn serve_connection(mut stream: TcpStream, shared: Arc<Mutex<Transcript>>) {
    let (header, body) = read_request(&mut stream);
    let mut transcript = shared.lock().unwrap_or_else(|error| panic!("{error:?}"));
    let (status, extra, body) = if header.starts_with("GET ") {
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
            (
                200,
                String::new(),
                include_str!("../fixtures/chainflow_ticker_btc.json")
                    .replace("PERP_BTC_USDC", &format!("PERP_{symbol}_USDC")),
            )
        }
    } else {
        assert!(header.starts_with("POST / "));
        let request: Value =
            serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(request["jsonrpc"], "2.0");
        let result = transcript.answer(&request);
        if transcript.lose_ack && request["method"] == "broadcast_tx_sync" {
            (503, String::new(), "acknowledgement lost".to_owned())
        } else {
            (200, String::new(), result.to_string())
        }
    };
    let response_delay_ms = transcript.response_delay_ms;
    drop(transcript);
    thread::sleep(Duration::from_millis(response_delay_ms));
    let response = format!("HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\n{extra}Content-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len());
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

    fn answer(&mut self, request: &Value) -> Value {
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
                let tx_index = self.txs.len();
                self.txs.push(tx);
                let fate = self.tx_fates.get(tx_index).copied().unwrap_or(self.fate);
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
