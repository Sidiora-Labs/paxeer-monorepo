//! Task 22.33.1412: real sidecar, signed Cosmos codec, durable store and loopback peers.
#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

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
use layerx_oracle_feeder::cosmos_tx::{encode_tx, feed_account, Submission};
use layerx_oracle_feeder::{hex_decode, hex_encode, Journal, MarketRecord};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use zeroize::Zeroizing;

const NOW: u64 = 1_700_000_000_500;
const FEED: &str = "79b000887626b294a914501a4cd226b58b235983";
const ACCOUNT: &str = r#"{"response":{"code":0,"value":"ClQKIC9jb3Ntb3MuYXV0aC52MWJldGExLkJhc2VBY2NvdW50EjAKKnBheDEweGNxcHpya3k2ZWZmMmc1MnFkeWU1M3hrazlqeGt2cmEwOHludBgJIAM=","height":"811"}}"#;

type TestResult = Result<(), String>;

#[derive(Clone, Copy)]
enum Fate {
    Pending,
    Committed { code: u64, height: u64 },
    Evicted,
    LookupUnavailable,
    IncompleteMempool,
    MalformedExecutionCode,
    WrongTransaction,
    CommitDuringLookup,
}

struct Transcript {
    account_sequence: u8,
    observation_sequence: u64,
    fate: Fate,
    txs: Vec<Vec<u8>>,
    ticker_requests: usize,
    api_keys: Vec<String>,
    redirect: Option<String>,
    lose_ack: bool,
    tx_lookups: usize,
}

impl Default for Transcript {
    fn default() -> Self {
        Self {
            account_sequence: 3,
            observation_sequence: 0,
            fate: Fate::Pending,
            txs: Vec::new(),
            ticker_requests: 0,
            api_keys: Vec::new(),
            redirect: None,
            lose_ack: false,
            tx_lookups: 0,
        }
    }
}

/// A committed protocol transcript served over real HTTP sockets. Broadcast
/// replies and later lookups bind to the actual bytes produced by `encode_tx`.
/// Response shapes follow consensus/rpc/coretypes, internal/rpc/core/tx.go
/// and abci/types/ExecTxResult (including its omitted zero success code).
struct Peer {
    url: String,
    state: Arc<Mutex<Transcript>>,
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
    fn new() -> Self {
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
                    Ok((mut stream, _)) => {
                        let (header, body) = read_request(&mut stream);
                        let mut transcript =
                            shared.lock().unwrap_or_else(|error| panic!("{error:?}"));
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
                            assert!(header.starts_with("GET /api/markets/ticker/BTC "));
                            if let Some(location) = &transcript.redirect {
                                (302, format!("Location: {location}\r\n"), String::new())
                            } else {
                                (
                                    200,
                                    String::new(),
                                    include_str!("fixtures/chainflow_ticker_btc.json").to_owned(),
                                )
                            }
                        } else {
                            assert!(header.starts_with("POST / "));
                            let request: Value = serde_json::from_slice(&body)
                                .unwrap_or_else(|error| panic!("{error:?}"));
                            assert_eq!(request["jsonrpc"], "2.0");
                            let result = transcript.answer(&request);
                            if transcript.lose_ack && request["method"] == "broadcast_tx_sync" {
                                (503, String::new(), "acknowledgement lost".to_owned())
                            } else {
                                (200, String::new(), result.to_string())
                            }
                        };
                        let response = format!("HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\n{extra}Content-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len());
                        stream
                            .write_all(response.as_bytes())
                            .unwrap_or_else(|error| panic!("{error:?}"));
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

    fn rpc(&self) -> round::Rpc {
        round::Rpc::new(agent(), self.url.clone(), self.url.clone())
            .unwrap_or_else(|error| panic!("{error:?}"))
    }
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
    fn account_response(&self, request: &Value) -> Value {
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
        let height = match self.fate {
            Fate::Committed { height, .. } => height + 1,
            _ => 830,
        };
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
                    let height = match self.fate {
                        Fate::Committed { height, .. } => height + 1,
                        _ => 830,
                    };
                    assert_eq!(request["params"][1], format!("0x{height:x}"));
                    json!(format!(
                        "0x{:064x}{}",
                        self.observation_sequence,
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
                self.txs.push(tx);
                json!({"code":0,"log":"[]","codespace":"","hash":hash.to_uppercase()})
            }
            "tx" => {
                self.tx_lookups += 1;
                let tx = self
                    .txs
                    .last()
                    .unwrap_or_else(|| panic!("lookup follows real broadcast"));
                let hash = hex_encode(&Sha256::digest(tx));
                assert_eq!(request["params"]["hash"], hash.to_uppercase());
                if matches!(self.fate, Fate::LookupUnavailable) {
                    return json!({"jsonrpc":"2.0","id":1,"error":{"code":-32603,"data":"transaction indexing is disabled"}});
                }
                let committed = match self.fate {
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
                    if matches!(self.fate, Fate::MalformedExecutionCode) {
                        result["tx_result"] = json!({"code":"invalid"});
                    }
                    if matches!(self.fate, Fate::WrongTransaction) {
                        result["tx"] = json!(STANDARD.encode(b"another transaction"));
                    }
                    result
                } else {
                    return json!({"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Internal error","data":format!("tx ({}) not found, err: %!w(<nil>)",hash.to_uppercase())}});
                }
            }
            "unconfirmed_txs" => {
                assert_eq!(request["params"], json!({"page":1,"per_page":100}));
                if matches!(self.fate, Fate::Pending) {
                    json!({"total":"1","n_txs":"1","txs":[STANDARD.encode(self.txs.last().unwrap_or_else(|| panic!("required protocol value is absent")))]})
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

struct StateDir(PathBuf);
impl StateDir {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-1412-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::SeqCst)
        ));
        std::fs::create_dir(&path).unwrap_or_else(|error| panic!("{error:?}"));
        Self(path)
    }
    fn progress(&self) -> Value {
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

fn agent() -> ureq::Agent {
    ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(3)))
        .max_redirects(0)
        .build()
        .into()
}
fn config(peer: &Peer) -> Config {
    Config {
        chain_id: "test-1".to_owned(),
        validator: "22".repeat(20),
        feed_account: FEED.to_owned(),
        gas_limit: 200_000,
        cadence_ms: 1000,
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
fn start(config: &Config, peer: &Peer, dir: &StateDir) -> round::Sidecar {
    round::start(config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)
        .unwrap_or_else(|error| panic!("{error:?}"))
}
fn ticker(config: &Config) -> Ticker {
    config
        .fetch_ticker_loopback(&agent(), "BTC", "fixture-credential", || NOW)
        .unwrap_or_else(|error| panic!("{error:?}"))
}
fn submit(
    config: &Config,
    peer: &Peer,
    dir: &StateDir,
    sidecar: &mut round::Sidecar,
    metrics: &Mutex<health::Metrics>,
) -> Submission {
    let quote = ticker(config);
    let observations = sidecar
        .collect(
            &[(&config.markets[0], Ok(quote))],
            &mut health::lock(metrics),
        )
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
    let expected = encode_tx(&[1; 32], &submission).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .txs
            .last(),
        Some(&expected)
    );
    assert_eq!(dir.progress()["pending"]["tx"], STANDARD.encode(expected));
    submission
}
fn assert_unconfirmed(metrics: &Mutex<health::Metrics>) {
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

#[test]
fn finding_031_deployed_https_and_isolated_loopback_transport() {
    let peer = Peer::new();
    let mut config = config(&peer);
    assert!(config.validate().is_err());
    assert!(config
        .fetch_ticker(&agent(), "BTC", "fixture-credential", || NOW)
        .is_err());
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        0
    );
    let quote = ticker(&config);
    assert_eq!(quote.index_price, "65000500000000000000000");
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .api_keys,
        ["fixture-credential"]
    );
    for url in [
        "http://feed.example",
        "http://127.0.0.1",
        "http://[::1]",
        "ftp://feed.example",
        "https://",
        "https://user:password@feed.example",
    ] {
        config.chainflow.base_url = url.to_owned();
        assert!(
            config.validate().is_err(),
            "deployed configuration admitted {url}"
        );
    }
    for url in [
        "http://feed.example",
        "http://localhost",
        "http://127.0.0.1.example",
        "http://127.0.0.1@feed.example",
        "http://192.0.2.1",
    ] {
        config.chainflow.base_url = url.to_owned();
        assert!(config
            .fetch_ticker_loopback(&agent(), "BTC", "fixture-credential", || NOW)
            .is_err());
    }
    config.chainflow.base_url = "https://feed.example".to_owned();
    assert_eq!(config.validate(), Ok(()));
}

#[test]
fn finding_031_authenticated_quote_redirect_is_not_followed() {
    let peer = Peer::new();
    let destination = Peer::new();
    peer.state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .redirect = Some(format!("{}/api/markets/ticker/BTC", destination.url));
    let config = config(&peer);
    // Even a caller's redirect-enabled agent cannot forward the credential.
    let redirecting_agent: ureq::Agent = ureq::Agent::config_builder()
        .max_redirects(5)
        .build()
        .into();
    assert!(config
        .fetch_ticker_loopback(&redirecting_agent, "BTC", "fixture-credential", || NOW)
        .is_err());
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        1
    );
    assert_eq!(
        destination
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        0
    );
}

#[test]
fn finding_029_checktx_is_pending_and_restart_confirms_actual_height() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    let submission = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(submission.sequence, 3);
    assert_eq!(submission.observations[0].sequence, 1);
    assert_unconfirmed(&metrics);
    let text = health::lock(&metrics).metrics_text(now_ms());
    assert!(text.contains("result=\"submitted\"} 1\n"));
    assert!(text.contains("price_sidecar_proposed_target_height 831\n"));
    assert!(text.contains("price_sidecar_pending_submission{hash=\""));
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, 3);
    assert_eq!(journal.markets["BTC"].sequence, 0);
    assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let requests = peer
        .state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .ticker_requests;
    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        requests
    );
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .txs
            .len(),
        1
    );
    drop(sidecar);
    let mut sidecar = start(&config, &peer, &dir);
    let metrics = Mutex::new(health::Metrics::default());
    assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_unconfirmed(&metrics);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = Fate::Committed {
            code: 0,
            height: 840,
        };
        state.account_sequence = 4;
        state.observation_sequence = 1;
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, 4);
    assert_eq!(journal.markets["BTC"].sequence, 1);
    assert_eq!(
        journal.markets["BTC"].activity_id,
        hex_encode(&Sha256::digest(
            &peer
                .state
                .lock()
                .unwrap_or_else(|error| panic!("{error:?}"))
                .txs[0]
        ))
    );
    assert_eq!(dir.progress()["confirmed_height"], 840);
    assert!(dir.progress()["pending"].is_null());
    let confirmed_at = dir.progress()["confirmed_ms"]
        .as_u64()
        .unwrap_or_else(|| panic!("required protocol value is absent"));
    assert!(health::lock(&metrics).is_ready(confirmed_at, 1000));
    assert!(!health::lock(&metrics).is_ready(confirmed_at + 3001, 1000));
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("price_sidecar_last_accepted_height 840\n"));
    drop(sidecar);
    let mut reopened = start(&config, &peer, &dir);
    let metrics = Mutex::new(health::Metrics::default());
    reopened.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?;
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("price_sidecar_confirmed_height 840\n"));
    assert_eq!(dir.progress()["confirmed_ms"], confirmed_at);
    Ok(())
}

fn recovery_after_failure(fate: Fate, expected_account: u8, label: &str) -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    drop(sidecar);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = fate;
        state.account_sequence = expected_account;
    }
    let mut sidecar = start(&config, &peer, &dir);
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_unconfirmed(&metrics);
    assert!(dir.progress()["pending"].is_null());
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, u64::from(expected_account));
    assert_eq!(journal.markets["BTC"].sequence, 0);
    assert!(journal.markets["BTC"].activity_id.is_empty());
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains(&format!("result=\"{label}\"}} 1\n")));
    peer.state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .fate = Fate::Pending;
    let retry = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(retry.sequence, u64::from(expected_account));
    assert_eq!(retry.observations[0].sequence, 1);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = Fate::Committed {
            code: 0,
            height: 845,
        };
        state.account_sequence += 1;
        state.observation_sequence = 1;
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_eq!(dir.progress()["confirmed_height"], 845);
    assert!(health::lock(&metrics).is_ready(now_ms(), 1000));
    Ok(())
}

#[test]
fn finding_029_failed_execution_recovers_account_and_observation_sequences() -> TestResult {
    recovery_after_failure(
        Fate::Committed {
            code: 12,
            height: 840,
        },
        4,
        "failed",
    )
}
#[test]
fn finding_029_eviction_recovers_without_spending_sequences() -> TestResult {
    recovery_after_failure(Fate::Evicted, 3, "evicted")
}

#[test]
fn finding_029_unknown_fate_and_incomplete_mempool_preserve_durable_pending() {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    let pending = dir.progress();
    for fate in [
        Fate::LookupUnavailable,
        Fate::IncompleteMempool,
        Fate::MalformedExecutionCode,
        Fate::WrongTransaction,
        Fate::CommitDuringLookup,
    ] {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = fate;
        state.tx_lookups = 0;
        drop(state);
        assert!(sidecar
            .reconcile(&config, &peer.rpc(), &metrics, &dir.0)
            .is_err());
        assert_eq!(dir.progress(), pending);
        assert_unconfirmed(&metrics);
        assert_eq!(
            peer.state
                .lock()
                .unwrap_or_else(|error| panic!("{error:?}"))
                .txs
                .len(),
            1
        );
    }
}

#[test]
fn finding_029_lost_acknowledgement_retries_identical_signed_bytes_and_recovers() -> TestResult {
    let peer = Peer::new();
    peer.state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .lose_ack = true;
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    {
        let state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(state.txs.len(), 2);
        assert_eq!(state.txs[0], state.txs[1]);
    }
    assert_unconfirmed(&metrics);
    drop(sidecar);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = Fate::Committed {
            code: 0,
            height: 840,
        };
        state.account_sequence = 4;
        state.observation_sequence = 1;
    }
    let mut reopened = start(&config, &peer, &dir);
    assert!(reopened.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_eq!(dir.progress()["confirmed_height"], 840);
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .txs
            .len(),
        2
    );
    Ok(())
}

#[test]
fn finding_029_legacy_optimistic_journal_reopens_at_committed_sequences() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let mut journal = Journal {
        next_account_sequence: 99,
        ..Journal::default()
    };
    journal.markets.insert(
        "BTC".to_owned(),
        MarketRecord {
            sequence: 99,
            activity_id: "unconfirmed legacy hash".to_owned(),
            ..MarketRecord::default()
        },
    );
    journal.store(&dir.0)?;
    let mut sidecar = start(&config, &peer, &dir);
    let metrics = Mutex::new(health::Metrics::default());
    let submission = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(submission.sequence, 3);
    assert_eq!(submission.observations[0].sequence, 1);
    assert_unconfirmed(&metrics);
    Ok(())
}
