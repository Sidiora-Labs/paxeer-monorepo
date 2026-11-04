//! Documentation claims qualified against production payment, HTTP and durable
//! program-exchange paths. Peers replay committed transcripts over loopback.
use std::error::Error;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::{signer_address, AttestorSet};
use x_websearch::content::ContentStore;
use x_websearch::fetch::{Fetcher, HttpClient, Url};
use x_websearch::index::WebIndex;
use x_websearch::keys::{KeyFiles, KeyRefusal, KeyRole, RECEIVER_KEY_FILE};
use x_websearch::payment::{system_clock, PaymentGate};
use x_websearch::program::{self, ProgramAttestor, ProgramExchange};
use x_websearch::search;
use x_websearch::submit::decode_program_observation;
use x_websearch::{Config, Limits, Response, RouteTable, Server};

#[path = "support/payment_repair.rs"]
mod payment_repair;

type Checked<T = ()> = Result<T, Box<dyn Error>>;
const CHAIN: u64 = 125;
const NETWORK: u32 = 9;
const README: &str = include_str!("../README.md");

struct Scratch(PathBuf);
impl Scratch {
    fn new(label: &str) -> Checked<Self> {
        let path =
            std::env::temp_dir().join(format!("repair-22-33-120-{}-{label}", std::process::id()));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}
fn key(number: u8) -> Checked<SigningKey> {
    let mut scalar = [0; 32];
    scalar[31] = number;
    Ok(SigningKey::from_slice(&scalar)?)
}
fn readme_row(name: &str) -> Checked<&'static str> {
    README
        .lines()
        .find(|line| line.starts_with(&format!("| `{name}` |")))
        .ok_or_else(|| format!("missing documented configuration row {name}").into())
}

#[test]
fn readme_metered_documents_exact_receipt_verification_and_receiver_identity() -> Checked {
    assert!(README.contains("advertise only the `exact` scheme"));
    assert!(README.contains("Payer grants and metered draws are retired at the ledger"));
    assert!(README.contains("unoffered metered payments, unverified or mismatched receipts"));
    assert!(README.contains("signed receipt, authorized signer and height range"));
    assert!(README.contains(RECEIVER_KEY_FILE));
    assert!(README.contains("Exact payment verification still requires this identity"));
    let missing = KeyFiles::from_lookup(|_| None).expect_err("receiver identity is required");
    assert_eq!(missing.role, KeyRole::Receiver);
    assert_eq!(missing.refusal, KeyRefusal::Unset);
    // The existing shared proof authenticates producer-issued exact receipts for
    // all four assets through the real seller/HTTP gate, refuses retired draws,
    // tampered receipts and redirected retries, then reopens delivery journals.
    payment_repair::supported_offers_refusals_and_recovery()
}

fn read_request(stream: &mut impl Read) -> Checked<(String, Vec<u8>)> {
    let mut bytes = Vec::new();
    let mut chunk = [0; 4096];
    loop {
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = String::from_utf8(bytes[..end].to_vec())?;
            let length = head
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())?
                })
                .unwrap_or(0);
            if bytes.len() >= end + 4 + length {
                return Ok((head, bytes[end + 4..end + 4 + length].to_vec()));
            }
        }
        if bytes.len() > 65_536 {
            return Err("oversized test request".into());
        }
        let count = stream.read(&mut chunk)?;
        if count == 0 {
            return Err("early test request EOF".into());
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
}

struct RpcTranscript {
    address: SocketAddr,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl RpcTranscript {
    fn start(handler: impl Fn(&Value) -> Result<Value, Value> + Send + 'static) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let calls = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&calls);
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            for incoming in listener.incoming() {
                if stopping.load(Ordering::Acquire) {
                    break;
                }
                let Ok(mut stream) = incoming else {
                    break;
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .unwrap();
                let (_, body) = read_request(&mut stream).unwrap();
                let request: Value = serde_json::from_slice(&body).unwrap();
                recorded.lock().unwrap().push(request.clone());
                let response = match handler(&request) {
                    Ok(result) => json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
                    Err(error) => json!({"jsonrpc":"2.0", "id":request["id"], "error":error}),
                }
                .to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()).unwrap();
            }
        });
        Ok(Self {
            address,
            calls,
            stop,
            worker: Some(worker),
        })
    }
    fn url(&self) -> String {
        format!("http://{}/rpc", self.address)
    }
}
impl Drop for RpcTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            worker.join().unwrap();
        }
    }
}
fn method_not_found() -> Value {
    json!({"code":-32601,"message":"method not found in committed transcript"})
}
fn settlement_peer(receipts: Arc<AtomicBool>, chain: Arc<AtomicU64>) -> Checked<RpcTranscript> {
    let recording: Value = serde_json::from_str(include_str!("fixtures/gateway/metered.json"))?;
    let exchanges = recording["endpoints"]["gateway"]["*"]
        .as_array()
        .ok_or("ledger exchanges")?
        .clone();
    RpcTranscript::start(move |request| match request["method"].as_str() {
        Some("eth_chainId") => Ok(json!(format!("0x{:x}", chain.load(Ordering::Acquire)))),
        Some("pax_sequencerReceipt") if receipts.load(Ordering::Acquire) => Ok(Value::Null),
        Some("eth_call") => exchanges
            .iter()
            .find(|exchange| {
                exchange["method"] == request["method"] && exchange["params"] == request["params"]
            })
            .map(|exchange| exchange["result"].clone())
            .ok_or_else(method_not_found),
        _ => Err(method_not_found()),
    })
}

#[test]
fn readme_endpoint_matches_distinct_settlement_and_watcher_authorities() -> Checked {
    let row = readme_row("gateway.endpoint")?;
    assert!(row.contains("Settlement authority shared by the payment gate and readiness"));
    assert!(row.contains("pax_sequencerReceipt"));
    assert!(
        readme_row("evm.endpoint")?.contains("Payment receipts are read from `gateway.endpoint`")
    );
    assert!(README.contains("Distinct URLs are supported"));
    assert!(README.contains("`X_WEBSEARCH_GATEWAY_ENDPOINT` must select the receipt authority"));
    let scratch = Scratch::new("settlement")?;
    let receipt_support = Arc::new(AtomicBool::new(true));
    let chain = Arc::new(AtomicU64::new(CHAIN));
    let settlement = settlement_peer(Arc::clone(&receipt_support), Arc::clone(&chain))?;
    let watcher = settlement_peer(
        Arc::new(AtomicBool::new(false)),
        Arc::new(AtomicU64::new(CHAIN)),
    )?;
    let mut config: Value = serde_json::from_str(include_str!("fixtures/config/valid.json"))?;
    config["data_dir"] = json!(scratch.0);
    config["gateway"]["endpoint"] = json!(settlement.url());
    config["evm"]["endpoint"] = json!(watcher.url());
    config["evm"]["chain_id"] = json!(CHAIN);
    let config = Config::parse(&config.to_string())?;
    let gate = Arc::new(PaymentGate::new(
        &config,
        &key(3)?,
        x_websearch::conformance_suite()?,
        system_clock,
    )?);
    assert_eq!(gate.settlement_authority().endpoint(), settlement.url());
    assert!(gate.settlement_usable());
    let index = Arc::new(WebIndex::open(&scratch.0)?);
    index.put("https://paxeer.app/", "readiness", "readiness")?;
    index.commit()?;
    let store = Arc::new(ContentStore::open(&scratch.0, &[])?);
    let mut routes = RouteTable::new();
    search::register(&mut routes, &gate, &index, &store)?;
    let readiness = Arc::clone(&gate);
    routes.set_serving_eligibility(move || readiness.settlement_usable())?;
    let ready = Arc::clone(&gate);
    routes.set_readiness(move |_| {
        if ready.settlement_usable() {
            Response::json(200, b"{}".to_vec())
        } else {
            Response::error(503, "settlement_unavailable")
        }
    })?;
    let server = Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?;
    let client = HttpClient::new(Duration::from_secs(1))?;
    let get = |path: &str| -> Checked<_> {
        Ok(client.get(
            &Url::parse(&format!("http://{}{path}", server.local_addr()))?,
            server.local_addr(),
            Instant::now() + Duration::from_secs(2),
            65_536,
            &[],
        )?)
    };
    assert_eq!(get("/readyz")?.status, 200);
    let eligible = get("/search?q=readiness")?;
    assert_eq!(eligible.status, 402);
    assert!(eligible.header("PAYMENT-REQUIRED").is_some());
    assert!(
        watcher.calls.lock().unwrap().is_empty(),
        "payment used the watcher authority"
    );
    receipt_support.store(false, Ordering::Release);
    assert_eq!(get("/readyz")?.status, 503);
    assert_eq!(get("/search?q=readiness")?.status, 503);
    receipt_support.store(true, Ordering::Release);
    chain.store(CHAIN + 1, Ordering::Release);
    assert_eq!(
        get("/search?q=readiness")?.status,
        503,
        "wrong-chain receipt authority admitted payment"
    );
    chain.store(CHAIN, Ordering::Release);
    assert_eq!(get("/search?q=readiness")?.status, 402);
    assert!(settlement
        .calls
        .lock()
        .unwrap()
        .iter()
        .any(|call| call["method"] == "eth_call"));
    assert!(settlement
        .calls
        .lock()
        .unwrap()
        .iter()
        .any(|call| call["method"] == "pax_sequencerReceipt"));
    server.shutdown()?;
    Ok(())
}

fn attestor(
    number: u8,
    index: &Arc<WebIndex>,
    store: &Arc<ContentStore>,
) -> Checked<ProgramAttestor> {
    let config = Config::parse(include_str!("fixtures/config/valid.json"))?;
    Ok(ProgramAttestor::new(
        key(number)?,
        NETWORK,
        Arc::new(Fetcher::new(config.fetch)?),
        Arc::clone(index),
        Arc::clone(store),
    ))
}
fn exchange(
    directory: &std::path::Path,
    peers: &[String],
    number: u8,
) -> Checked<Arc<ProgramExchange>> {
    let exchange = Arc::new(ProgramExchange::open(directory, peers)?);
    exchange.bind_chain(CHAIN, NETWORK, signer_address(&key(number)?))?;
    Ok(exchange)
}
fn serve_exchange(exchange: &Arc<ProgramExchange>) -> Checked<x_websearch::RunningServer> {
    let mut routes = RouteTable::new();
    program::register(&mut routes, exchange)?;
    Ok(Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?)
}

#[test]
fn readme_peer_recovery_preserves_pre_quorum_bytes_and_retirement_tombstones() -> Checked {
    for claim in [
        "program-watch/pending.json",
        "program-attest/answers.json",
        "program-attest/identity.json",
        "including before quorum",
        "local signature and collected signer",
        "Pending submission or quorum alone does not retire",
        "retirement tombstone",
        "retirement does not delete those records",
        "tests/proof_x_websearch_program_requests.rs",
    ] {
        assert!(README.contains(claim), "missing recovery guidance: {claim}");
    }
    let scratch = Scratch::new("peer-recovery")?;
    let directory = scratch.0.join("program-attest");
    let peer_dir = scratch.0.join("peer");
    let third_dir = scratch.0.join("third");
    let index = Arc::new(WebIndex::open(&scratch.0)?);
    let store = Arc::new(ContentStore::open(&scratch.0, &[])?);
    index.put(
        "https://paxeer.app/proof",
        "Paxeer network before restart",
        "paxeer network",
    )?;
    index.commit()?;
    let fixture: Value = serde_json::from_str(include_str!("fixtures/kernel/watch.json"))?;
    let event = &fixture["endpoints"]["gateway"]["*"][0]["result"]["events"][0];
    let program_id = x_websearch::payment::unhex(event["program_id"].as_str().ok_or("program id")?)
        .ok_or("program hex")?
        .try_into()
        .map_err(|_| "program width")?;
    let request = program::decode_request_record(
        program_id,
        event["sequence"].as_u64().ok_or("sequence")?,
        &x_websearch::payment::unhex(event["data"].as_str().ok_or("request data")?)
            .ok_or("request hex")?,
    )?;
    let program_key = (request.program_id, request.request_id);
    let mut signers = vec![
        signer_address(&key(1)?),
        signer_address(&key(2)?),
        signer_address(&key(4)?),
    ];
    signers.sort_unstable();
    let set = AttestorSet {
        signers,
        threshold: 3,
    };
    let local_answer = attestor(1, &index, &store)?.attest(&request)?;
    let peer_answer = attestor(2, &index, &store)?.attest(&request)?;
    let third_answer = attestor(4, &index, &store)?.attest(&request)?;
    assert_eq!(local_answer.digest, peer_answer.digest);
    assert_eq!(local_answer.digest, third_answer.digest);
    let peer = exchange(&peer_dir, &[], 2)?;
    peer.record(peer_answer.clone())?;
    let third = exchange(&third_dir, &[], 4)?;
    third.record(third_answer.clone())?;
    let peer_server = serve_exchange(&peer)?;
    let local = exchange(
        &directory,
        &[format!("http://{}", peer_server.local_addr())],
        1,
    )?;
    local.record(local_answer.clone())?;
    assert_eq!(local.collect(program_key, &set), 2);
    assert!(
        local.ready(program_key, &set).is_none(),
        "two signers cannot satisfy three-member quorum"
    );
    let before = std::fs::read(directory.join("answers.json"))?;
    let saved: Value = serde_json::from_slice(&before)?;
    assert_eq!(
        saved["answers"][0]["held"]["signatures"]
            .as_array()
            .ok_or("saved signatures")?
            .len(),
        2
    );
    peer_server.shutdown()?;
    drop(local);
    drop(peer);
    drop(third);
    drop(index);
    drop(store);

    // All exchanges and the stores are stopped before reopening. Changing the
    // real search index must not replace an already emitted signed answer.
    let index = Arc::new(WebIndex::open(&scratch.0)?);
    index.put(
        "https://paxeer.app/proof",
        "Paxeer network after restart",
        "paxeer network",
    )?;
    index.commit()?;
    let store = Arc::new(ContentStore::open(&scratch.0, &[])?);
    let peer = exchange(&peer_dir, &[], 2)?;
    let third = exchange(&third_dir, &[], 4)?;
    assert_eq!(peer.answer(program_key), Some(peer_answer));
    assert_eq!(third.answer(program_key), Some(third_answer));
    let peer_server = serve_exchange(&peer)?;
    let third_server = serve_exchange(&third)?;
    let local = exchange(
        &directory,
        &[
            format!("http://{}", peer_server.local_addr()),
            format!("http://{}", third_server.local_addr()),
        ],
        1,
    )?;
    assert_eq!(std::fs::read(directory.join("answers.json"))?, before);
    assert_eq!(local.answer(program_key), Some(local_answer.clone()));
    assert!(local.ready(program_key, &set).is_none());
    assert!(local
        .bind_chain(CHAIN + 1, NETWORK, signer_address(&key(1)?))
        .is_err());
    assert!(local
        .bind_chain(CHAIN, NETWORK, signer_address(&key(2)?))
        .is_err());
    let changed = attestor(1, &index, &store)?.attest(&request)?;
    assert_ne!(changed.response, local_answer.response);
    assert_ne!(changed.digest, local_answer.digest);
    assert!(local.record(changed).is_err());
    assert_eq!(local.collect(program_key, &set), 3);
    let ready = local.ready(program_key, &set).ok_or("recovered quorum")?;
    assert_eq!(ready.digest, local_answer.digest);
    assert_eq!(ready.response, local_answer.response);
    assert_eq!(ready.signers, set.signers);
    assert!(store.get(&ready.content_digest)?.is_some());
    let observation = program::observation_bytes(NETWORK, &request, &ready)?;
    assert_eq!(
        decode_program_observation(program_key, &observation, 3)?,
        ready
    );
    assert_eq!(
        local.pending(),
        vec![program_key],
        "quorum alone does not retire the answer"
    );
    assert_eq!(local.handle(program_key).status, 200);
    local.forget(program_key)?;
    assert!(local.pending().is_empty());
    assert!(local.answer(program_key).is_none());
    assert!(local.ready(program_key, &set).is_none());
    assert_eq!(local.handle(program_key).status, 404);
    peer_server.shutdown()?;
    third_server.shutdown()?;
    drop(local);
    drop(peer);
    drop(third);
    let local = exchange(&directory, &[], 1)?;
    assert!(local.pending().is_empty());
    assert!(local.answer(program_key).is_none());
    assert!(local.ready(program_key, &set).is_none());
    assert_eq!(local.handle(program_key).status, 404);
    assert!(
        local.record(local_answer).is_err(),
        "retirement survives reopen and prevents re-signing"
    );
    let retired: Value = serde_json::from_slice(&std::fs::read(directory.join("answers.json"))?)?;
    assert_eq!(retired["answers"][0]["retired"], true);
    assert_eq!(
        retired["retired"].as_array().ok_or("retired keys")?.len(),
        1
    );
    assert_eq!(
        retired["answers"][0]["held"]["signatures"]
            .as_array()
            .ok_or("retained signatures")?
            .len(),
        3
    );
    Ok(())
}
