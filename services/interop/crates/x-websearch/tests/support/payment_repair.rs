mod test_network {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/../test_network.rs"));
}

use std::error::Error;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use x_websearch::content::ContentStore;
use x_websearch::index::WebIndex;
use x_websearch::payment::{
    decode_payment_exact, hex, read_payer_grant, PaymentGate, EXACT, METERED, PAYER_ADDRESS,
    PAYMENT_REQUIRED, PAYMENT_RESPONSE, PAYMENT_SIGNATURE,
};
use x_websearch::search::search_route;
use x_websearch::settlement::{decode, SettlementRpc};
use x_websearch::{Config, Limits, Route, RouteTable, RunningServer, Server};

type Outcome<T = ()> = Result<T, Box<dyn Error>>;
const PAYER: &str = "0xabababababababababababababababababababab";
const CURRENCIES: [&str; 4] = ["SID", "PAX", "USDC", "USDL"];

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures")
}

fn read_json(path: &Path) -> Outcome<Value> {
    Ok(serde_json::from_slice(&std::fs::read(path)?)?)
}

struct Scratch(PathBuf);
impl Scratch {
    fn new() -> Outcome<Self> {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        test_network::isolate()?;
        let path = std::env::temp_dir().join(format!(
            "xweb-payment-repair-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

// Only committed exact receipts, account lookups and the retired-grant refusal
// are replayed. Successful grants and draws are deliberately absent.
struct Replay {
    address: SocketAddr,
    rules: Arc<Mutex<Vec<Value>>>,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<Result<(), String>>>,
}

fn read_request(stream: &mut TcpStream) -> Outcome<Value> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut bytes = Vec::new();
    loop {
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = std::str::from_utf8(&bytes[..end])?;
            let length = head
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())?
                })
                .ok_or("request length")?;
            if bytes.len() >= end + 4 + length {
                return Ok(serde_json::from_slice(&bytes[end + 4..end + 4 + length])?);
            }
        }
        let mut chunk = [0; 4096];
        let count = stream.read(&mut chunk)?;
        if count == 0 {
            return Err("closed request".into());
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
}

impl Replay {
    fn start() -> Outcome<Self> {
        let exact = read_json(&fixtures().join("gateway/exact.json"))?;
        let ledger = read_json(&fixtures().join("gateway/metered.json"))?;
        let mut rules = exact["endpoints"]["gateway"]["*"]
            .as_array()
            .ok_or("exact rules")?
            .clone();
        rules.extend(
            ledger["endpoints"]["gateway"]["*"]
                .as_array()
                .ok_or("ledger rules")?
                .iter()
                .filter(|rule| {
                    rule["method"] == "eth_chainId"
                        || (rule["method"] == "eth_call"
                            && (rule["params"][0]["data"].as_str().is_some_and(|data| {
                                data.starts_with("0x3f050cbc") || data.starts_with("0xd3600b24")
                            }) || rule["error"]["message"]
                                == "execution reverted: LXP_NO_PAYER_GRANT"))
                })
                .cloned(),
        );
        assert_eq!(
            rules
                .iter()
                .filter(|rule| rule.get("error").is_some())
                .count(),
            1
        );
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let rules = Arc::new(Mutex::new(rules));
        let calls = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let (served_rules, served_calls, stopped) =
            (Arc::clone(&rules), Arc::clone(&calls), Arc::clone(&stop));
        let thread = thread::spawn(move || -> Result<(), String> {
            for connection in listener.incoming() {
                if stopped.load(Ordering::SeqCst) {
                    return Ok(());
                }
                let mut stream = connection.map_err(|error| error.to_string())?;
                let request = read_request(&mut stream).map_err(|error| error.to_string())?;
                served_calls
                    .lock()
                    .map_err(|error| error.to_string())?
                    .push(request.clone());
                let rules = served_rules.lock().map_err(|error| error.to_string())?;
                let rule = rules
                    .iter()
                    .find(|rule| {
                        rule["method"] == request["method"] && rule["params"] == request["params"]
                    })
                    .ok_or_else(|| format!("unrecorded request: {request}"))?;
                let mut reply = json!({"jsonrpc": "2.0", "id": request["id"]});
                let field = if rule.get("error").is_some() {
                    "error"
                } else {
                    "result"
                };
                reply[field] = rule[field].clone();
                let body = serde_json::to_vec(&reply).map_err(|error| error.to_string())?;
                stream.write_all(format!("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).as_bytes())
                    .and_then(|()| stream.write_all(&body)).map_err(|error| error.to_string())?;
            }
            Ok(())
        });
        Ok(Self {
            address,
            rules,
            calls,
            stop,
            thread: Some(thread),
        })
    }

    fn endpoint(&self) -> String {
        format!("http://{}/rpc", self.address)
    }
    fn count(&self, method: &str) -> usize {
        self.calls
            .lock()
            .or_panic("calls lock")
            .iter()
            .filter(|call| call["method"] == method)
            .count()
    }
    fn receipt(&self, hash: &Value, replacement: Option<&Value>) -> Outcome<Value> {
        let mut rules = self.rules.lock().map_err(|_| "rules lock")?;
        let rule = rules
            .iter_mut()
            .find(|rule| {
                rule["method"] == "pax_sequencerReceipt" && rule["params"] == json!([hash])
            })
            .ok_or("recorded receipt")?;
        let previous = rule["result"].clone();
        if let Some(replacement) = replacement {
            rule["result"] = replacement.clone();
        }
        Ok(previous)
    }
    fn finish(mut self) -> Outcome {
        self.stop.store(true, Ordering::SeqCst);
        drop(TcpStream::connect(self.address)?);
        self.thread
            .take()
            .ok_or("replay thread")?
            .join()
            .map_err(|_| "replay panic")?
            .map_err(Into::into)
    }
}
impl Drop for Replay {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        let _ = TcpStream::connect(self.address);
        if let Some(thread) = self.thread.take() {
            let _ = thread.join();
        }
    }
}

const fn clock() -> u64 {
    1_000_000_000_000
}

fn config(scratch: &Scratch, rpc: &Replay) -> Outcome<Config> {
    let mut value = read_json(&fixtures().join("config/valid.json"))?;
    let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
    let exact = read_json(&fixtures().join("gateway/exact.json"))?;
    let receipt = rpc.receipt(&exact["payments"]["SID"]["txHash"], None)?;
    value["data_dir"] = json!(scratch.0);
    value["evm"]["endpoint"] = json!(rpc.endpoint());
    value["evm"]["chain_id"] = json!(125);
    value["gateway"]["endpoint"] = json!(rpc.endpoint());
    value["gateway"]["receipt_keys"] = json!([{
        "key_id": receipt["keyId"].as_str().ok_or("key id")?.strip_prefix("0x").ok_or("key prefix")?,
        "public_key": buyer["sequencerPublicKey"], "first_height": 0, "last_height": u64::MAX,
    }]);
    Ok(Config::parse(&value.to_string())?)
}

fn gate(config: &Config) -> Outcome<Arc<PaymentGate>> {
    let mut secret = [0; 32];
    secret[31] = 3;
    Ok(Arc::new(PaymentGate::new(
        config,
        &SigningKey::from_slice(&secret)?,
        x_websearch::conformance_suite()?,
        clock,
    )?))
}

fn serve(
    gate: &Arc<PaymentGate>,
    config: &Config,
    served: &Arc<AtomicUsize>,
) -> Outcome<RunningServer> {
    let index = Arc::new(WebIndex::open(&config.data_dir.join("index"))?);
    let store = Arc::new(ContentStore::open(&config.data_dir, &[])?);
    assert_eq!(store.directory(), config.data_dir.join("content"));
    let mut routes = RouteTable::new();
    let counter = Arc::clone(served);
    PaymentGate::install_validated(
        gate,
        &mut routes,
        Route::Search,
        |request| match request.query_param("q") {
            Ok(Some(query)) if !query.is_empty() => Ok(()),
            _ => Err(x_websearch::Response::error(400, "missing_query")),
        },
        move |request| {
            counter.fetch_add(1, Ordering::SeqCst);
            search_route(&index, &store, request)
        },
    )?;
    Ok(Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?)
}

struct Reply {
    status: u16,
    headers: Vec<(String, String)>,
    body: Value,
    raw: Vec<u8>,
}
impl Reply {
    fn decoded(&self, name: &str) -> Outcome<Value> {
        let value = self
            .headers
            .iter()
            .find(|(key, _)| key.eq_ignore_ascii_case(name))
            .ok_or("response header")?;
        Ok(serde_json::from_slice(&STANDARD.decode(&value.1)?)?)
    }
}
fn get(
    server: &RunningServer,
    target: &str,
    payer: Option<&str>,
    signature: Option<&str>,
) -> Outcome<Reply> {
    let mut stream = TcpStream::connect(server.local_addr())?;
    stream.set_read_timeout(Some(Duration::from_secs(10)))?;
    write!(stream, "GET {target} HTTP/1.1\r\nHost: 127.0.0.1\r\n")?;
    if let Some(payer) = payer {
        write!(stream, "{PAYER_ADDRESS}: {payer}\r\n")?;
    }
    if let Some(signature) = signature {
        write!(stream, "{PAYMENT_SIGNATURE}: {signature}\r\n")?;
    }
    stream.write_all(b"\r\n")?;
    let mut raw = Vec::new();
    stream.read_to_end(&mut raw)?;
    let end = raw
        .windows(4)
        .position(|part| part == b"\r\n\r\n")
        .ok_or("HTTP header")?;
    let head = std::str::from_utf8(&raw[..end])?;
    let mut lines = head.lines();
    let status = lines
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .ok_or("HTTP status")?
        .parse()?;
    let headers = lines
        .map(|line| {
            let (name, value) = line.split_once(':').ok_or("HTTP header field")?;
            Ok((name.to_owned(), value.trim().to_owned()))
        })
        .collect::<Outcome<Vec<_>>>()?;
    let body = serde_json::from_slice(&raw[end + 4..])?;
    Ok(Reply {
        status,
        headers,
        body,
        raw,
    })
}
fn signature(offer: &Value, payload: &Value) -> Outcome<String> {
    Ok(STANDARD.encode(serde_json::to_vec(
        &json!({"x402Version": 2, "accepted": offer, "payload": payload}),
    )?))
}
fn refused(reply: &Reply, reason: &str) -> Outcome {
    assert_eq!(reply.status, 402, "{}", reply.body);
    assert_eq!(reply.body["error"], reason);
    assert!(!reply.decoded(PAYMENT_REQUIRED)?["accepts"]
        .as_array()
        .ok_or("offers")?
        .is_empty());
    Ok(())
}

pub fn supported_offers_refusals_and_recovery() -> Outcome {
    let scratch = Scratch::new()?;
    let rpc = Replay::start()?;
    let config = config(&scratch, &rpc)?;
    let mut gate = gate(&config)?;
    let served_count = Arc::new(AtomicUsize::new(0));
    let mut server = serve(&gate, &config, &served_count)?;
    let rpc_client = SettlementRpc::new(&rpc.endpoint(), 125)?;
    for _ in 0..2 {
        assert_eq!(read_payer_grant(&rpc_client, &[0xee; 32])?, None);
    }
    let exact = read_json(&fixtures().join("gateway/exact.json"))?;
    let mut replays = Vec::new();
    for currency in CURRENCIES {
        replays.push(assert_supported_offer_case(
            currency,
            &server,
            &rpc,
            &exact,
            &served_count,
        )?);
    }
    assert_eq!(served_count.load(Ordering::SeqCst), 4);
    let lookups = rpc.count("pax_sequencerReceipt");
    server.shutdown()?;
    drop(gate);
    gate = self::gate(&config)?;
    gate.recover_pending_deliveries()?;
    // Recovery reauthenticates each persisted exact receipt before admitting
    // its delivery journal. It must not release the search handler again.
    assert_eq!(
        rpc.count("pax_sequencerReceipt"),
        lookups + CURRENCIES.len()
    );
    assert_eq!(served_count.load(Ordering::SeqCst), 4);
    server = serve(&gate, &config, &served_count)?;
    for (target, signed, raw, hash, bad_signature) in replays {
        let before = rpc.count("pax_sequencerReceipt");
        let original = rpc.receipt(&hash, Some(&bad_signature))?;
        refused(
            &get(&server, &target, None, Some(&signed))?,
            "receipt_unverified",
        )?;
        assert_eq!(served_count.load(Ordering::SeqCst), 4);
        assert_eq!(rpc.count("pax_sequencerReceipt"), before + 1);
        rpc.receipt(&hash, Some(&original))?;
        let replay = get(&server, &target, None, Some(&signed))?;
        assert_eq!(replay.status, 200);
        assert_eq!(replay.raw, raw);
        assert_eq!(rpc.count("pax_sequencerReceipt"), before + 2);
        let lookups = rpc.count("pax_sequencerReceipt");
        refused(
            &get(&server, "/search?q=elsewhere", None, Some(&signed))?,
            "request_mismatch",
        )?;
        assert_eq!(rpc.count("pax_sequencerReceipt"), lookups);
    }
    assert_eq!(served_count.load(Ordering::SeqCst), 4);
    assert_eq!(
        rpc.count("pax_sequencerReceipt"),
        lookups + 3 * CURRENCIES.len()
    );
    assert_eq!(
        std::fs::read_dir(config.data_dir.join("payments/receipts"))?.count(),
        4
    );
    assert_persisted_deliveries(&config, &rpc)?;
    server.shutdown()?;
    drop(gate);
    rpc.finish()?;
    Ok(())
}

type ReplayCase = (String, String, Vec<u8>, Value, Value);

fn assert_supported_offer_case(
    currency: &str,
    server: &RunningServer,
    rpc: &Replay,
    exact: &Value,
    served_count: &Arc<AtomicUsize>,
) -> Outcome<ReplayCase> {
    let target = format!("/search?q={currency}");
    let challenge = get(server, &target, Some(PAYER), None)?;
    refused(&challenge, "payment_required")?;
    let offers = challenge.decoded(PAYMENT_REQUIRED)?;
    let offers = offers["accepts"].as_array().ok_or("offers")?;
    assert_eq!(offers.len(), 4);
    assert!(offers.iter().all(|offer| offer["scheme"] == EXACT));
    let offer = offers
        .iter()
        .find(|offer| offer["extra"]["layerx"]["currency"] == currency)
        .ok_or("currency offer")?;
    assert!(offer["extra"]["layerx"].get("purposeHash").is_none());
    assert!(offer["extra"]["layerx"].get("payer").is_none());
    let mut legacy_offer = offer.clone();
    legacy_offer["scheme"] = json!(METERED);
    let legacy = signature(
        &legacy_offer,
        &json!({"grantId": hex(&[0xee; 32]), "idempotencyKey": hex(&[0x31; 32])}),
    )?;
    let before = served_count.load(Ordering::SeqCst);
    refused(
        &get(server, &target, None, Some(&legacy))?,
        "payment_invalid",
    )?;
    assert_eq!(served_count.load(Ordering::SeqCst), before);
    let mut payload = exact["payments"][currency].clone();
    payload
        .as_object_mut()
        .ok_or("payment payload")?
        .remove("rawTransaction");
    let signed = signature(offer, &payload)?;
    let original = rpc.receipt(&payload["txHash"], None)?;
    let mut bad_signature = original.clone();
    let receipt = bad_signature["receipt"].as_str().ok_or("signed receipt")?;
    let last = if receipt.ends_with("00") { "01" } else { "00" };
    bad_signature["receipt"] = json!(format!("{}{last}", &receipt[..receipt.len() - 2]));
    rpc.receipt(&payload["txHash"], Some(&bad_signature))?;
    refused(
        &get(server, &target, None, Some(&signed))?,
        "receipt_unverified",
    )?;
    assert_eq!(served_count.load(Ordering::SeqCst), before);
    rpc.receipt(&payload["txHash"], Some(&original))?;
    let mut mismatched = payload.clone();
    mismatched["receiptDigest"] = json!(hex(&[0x11; 32]));
    refused(
        &get(server, &target, None, Some(&signature(offer, &mismatched)?))?,
        "receipt_digest_mismatch",
    )?;
    let mut unbound = payload.clone();
    unbound["idempotencyKey"] = json!(hex(&[0x12; 32]));
    refused(
        &get(server, &target, None, Some(&signature(offer, &unbound)?))?,
        "receipt_mismatch",
    )?;
    assert_eq!(served_count.load(Ordering::SeqCst), before);
    let reply = get(server, &target, None, Some(&signed))?;
    assert_eq!(reply.status, 200, "{}", reply.body);
    let payment = reply.decoded(PAYMENT_RESPONSE)?;
    assert_eq!(payment["success"], true);
    assert_eq!(payment["network"], "eip155:125");
    assert_eq!(payment["transaction"], payload["txHash"]);
    assert_eq!(
        payment["extensions"]["layerx"]["receiptDigest"],
        payload["receiptDigest"]
    );
    assert_eq!(
        payment["extensions"]["layerx"]["verificationLevel"],
        "receipted"
    );
    let receipt = decode(&original)?;
    let event = decode_payment_exact(&receipt.logs[0]).ok_or("exact event")?;
    assert_eq!(payment["payer"], hex(&event.payer));
    assert_eq!(reply.body["query"], currency);
    assert_eq!(served_count.load(Ordering::SeqCst), before + 1);
    Ok((
        target,
        signed,
        reply.raw,
        payload["txHash"].clone(),
        bad_signature,
    ))
}

fn assert_persisted_deliveries(config: &Config, rpc: &Replay) -> Outcome {
    let entries = std::fs::read_dir(config.data_dir.join("payments/requests"))?
        .collect::<Result<Vec<_>, _>>()?;
    assert_eq!(entries.len(), 4);
    assert_eq!(
        std::fs::read_dir(config.data_dir.join("payments/deliveries"))?.count(),
        4
    );
    for entry in entries {
        let record: x_websearch::payment::PaymentRecord =
            serde_json::from_slice(&std::fs::read(entry.path())?)?;
        // Resumable delivery completion lives in the delivery journal; the
        // legacy one-shot release flag must not bypass that journal.
        assert!(!record.released);
        assert!(record.receipt.is_some());
        assert!(record.receipt_digest.is_some());
        let digest = record.receipt_digest.as_deref().ok_or("receipt digest")?;
        let owner = entry.file_name();
        assert_eq!(
            std::fs::read(config.data_dir.join("payments/receipts").join(digest))?,
            owner.as_encoded_bytes()
        );
        let delivery = read_json(&config.data_dir.join("payments/deliveries").join(owner))?;
        assert_eq!(delivery["state"], "ready");
        assert_eq!(delivery["receipt_digest"], digest);
        assert_eq!(delivery["request_digest"], record.request_digest);
        assert_eq!(delivery["resource"], record.resource);
        assert_eq!(delivery["principal"], record.principal);
        assert_eq!(delivery["failures"], 0);
        assert_eq!(delivery["retry_after_ms"], 0);
        assert_eq!(delivery["response"]["status"], 200);
        let body: Vec<u8> = serde_json::from_value(delivery["response"]["body"].clone())?;
        assert_eq!(
            delivery["response"]["body_digest"],
            hex(&Sha256::digest(&body))
        );
        let body: Value = serde_json::from_slice(&body)?;
        assert_eq!(
            record.resource,
            format!("GET /search?q={}", body["query"].as_str().ok_or("query")?)
        );
    }
    for method in [
        "eth_sendRawTransaction",
        "eth_estimateGas",
        "lx_sendActivity",
        "lx_getReceipt",
    ] {
        assert_eq!(rpc.count(method), 0);
    }
    Ok(())
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
