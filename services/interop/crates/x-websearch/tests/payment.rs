use std::error::Error;
use std::fmt::Write as _;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex, PoisonError};
use std::thread;
use std::time::Duration;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::payments::{Grant, Payment};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::leaf_hash;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{receipt_digest, Domain};
use layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION as PROTOCOL_VERSION;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use x_websearch::assets::{AcceptedAsset, AcceptedAssets};
use x_websearch::canonical::{canonical_bytes, content_digest, digest_hex, ContentKind};
use x_websearch::config::{AssetSymbol, DEFAULT_DRAW_FEE_LIMIT};
use x_websearch::content::ContentStore;
use x_websearch::crawl::title_of;
use x_websearch::index::WebIndex;
use x_websearch::payment::{
    account_id, decode_grant, draw_calldata, hex, purpose_hash, read_payer_grant, wallet_account,
    GateError, LedgerTransaction, PaymentGate, DRAW_SIGNATURE, EXACT, LEDGER_PRECOMPILE, METERED,
    PAYER_DID, PAYMENT_REQUIRED, PAYMENT_RESPONSE, PAYMENT_SIGNATURE,
};
use x_websearch::search::search_route;
use x_websearch::settlement::{SettlementError, SettlementRpc};
use x_websearch::{
    Config, KeyFiles, Limits, Request, Response, Route, RouteTable, RunningServer, Server,
};

type Outcome<T = ()> = Result<T, Box<dyn Error>>;

const RECEIVER_SECRET: &str = "0000000000000000000000000000000000000000000000000000000000000003";
const NOW_MS: u64 = 1_000_000_000_000;
const CURRENCIES: [&str; 4] = ["SID", "PAX", "USDC", "USDL"];
const RECORD_VARIABLE: &str = "X_WEBSEARCH_RECORD_EXCHANGE";

const fn fixed_clock() -> u64 {
    NOW_MS
}

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures")
}

fn read_json(path: &Path) -> Outcome<Value> {
    Ok(serde_json::from_str(&std::fs::read_to_string(path)?)?)
}

fn fail(message: impl Into<String>) -> Box<dyn Error> {
    message.into().into()
}

fn text<'a>(value: &'a Value, pointer: &str) -> Outcome<&'a str> {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .ok_or_else(|| fail(format!("missing {pointer}")))
}

/// One recorded gateway answer.
struct Rule {
    method: String,
    params: Value,
    answer: Value,
    once: bool,
    used: bool,
}

/// Replays a recorded gateway exchange on the loopback interface. A request
/// no recorded rule matches is answered 503.
struct Gateway {
    address: SocketAddr,
    rules: Arc<Mutex<Vec<Rule>>>,
    calls: Arc<Mutex<Vec<String>>>,
    requests: Arc<Mutex<Vec<Value>>>,
    journal: Arc<Mutex<Option<PathBuf>>>,
    broadcasts: Arc<Mutex<Vec<Vec<x_websearch::payment::PaymentRecord>>>>,
}

fn load_rules(names: &[&str]) -> Outcome<Vec<Rule>> {
    let mut rules = Vec::new();
    for name in names {
        let recording = read_json(&fixtures().join("gateway").join(name))?;
        let list = recording
            .pointer("/endpoints/gateway/*")
            .and_then(Value::as_array)
            .ok_or_else(|| fail(format!("{name} has no gateway rules")))?;
        rules.extend(parse_rules(list)?);
    }
    if !names.contains(&"metered.json") {
        let recording = read_json(&fixtures().join("gateway/metered.json"))?;
        let ledger_rules: Vec<Value> = recording["endpoints"]["gateway"]["*"]
            .as_array()
            .ok_or("gateway rules")?
            .iter()
            .filter(|rule| {
                rule["method"]
                    .as_str()
                    .is_some_and(|name| name.starts_with("eth_"))
            })
            .cloned()
            .collect();
        rules.extend(parse_rules(&ledger_rules)?);
    }
    Ok(rules)
}

fn parse_rules(list: &[Value]) -> Outcome<Vec<Rule>> {
    list.iter()
        .map(|rule| {
            let answer = rule
                .get("result")
                .map(|result| json!({ "result": result }))
                .or_else(|| rule.get("error").map(|error| json!({ "error": error })))
                .ok_or_else(|| fail("rule without an answer"))?;
            Ok(Rule {
                method: text(rule, "/method")?.to_owned(),
                params: rule.get("params").cloned().unwrap_or(Value::Null),
                answer,
                once: rule.get("once").and_then(Value::as_bool).unwrap_or(false),
                used: false,
            })
        })
        .collect()
}

fn read_http(stream: &mut TcpStream) -> Outcome<(String, Vec<u8>)> {
    let mut bytes = Vec::new();
    let mut chunk = [0; 4_096];
    loop {
        if let Some(end) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            let head = String::from_utf8(bytes[..end].to_vec())?;
            let length = head
                .split("\r\n")
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
        let read = stream.read(&mut chunk)?;
        if read == 0 {
            return Err(fail("connection closed early"));
        }
        bytes.extend_from_slice(&chunk[..read]);
    }
}

fn answer(rules: &Mutex<Vec<Rule>>, calls: &Mutex<Vec<String>>, body: &[u8]) -> Option<Value> {
    let request: Value = serde_json::from_slice(body).ok()?;
    let method = request.get("method")?.as_str()?.to_owned();
    let params = request.get("params").cloned().unwrap_or(Value::Null);
    calls
        .lock()
        .unwrap_or_else(PoisonError::into_inner)
        .push(method.clone());
    let mut rules = rules.lock().unwrap_or_else(PoisonError::into_inner);
    let rule = rules.iter_mut().find(|rule| {
        !rule.used
            && rule.method == method
            && (rule.params == json!("$any") || rule.params == params)
    })?;
    rule.used = rule.once;
    let mut reply =
        json!({ "jsonrpc": "2.0", "id": request.get("id").cloned().unwrap_or(json!(1)) });
    if let (Some(reply), Some(answer)) = (reply.as_object_mut(), rule.answer.as_object()) {
        reply.extend(answer.clone());
    }
    Some(reply)
}

fn serve(stream: &mut TcpStream, rules: &Mutex<Vec<Rule>>, calls: &Mutex<Vec<String>>) -> Outcome {
    let (_, body) = read_http(stream)?;
    let response = answer(rules, calls, &body).map_or_else(
        || "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_owned(),
        |reply| {
            let body = reply.to_string();
            format!(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            )
        },
    );
    stream.write_all(response.as_bytes())?;
    Ok(())
}

impl Gateway {
    /// Replays the rules `list` holds, in the recording's own shape.
    fn serve_rules(list: &[Value]) -> Outcome<Self> {
        Self::listen(parse_rules(list)?)
    }

    fn start(names: &[&str]) -> Outcome<Self> {
        Self::listen(load_rules(names)?)
    }

    fn listen(rules: Vec<Rule>) -> Outcome<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let rules = Arc::new(Mutex::new(rules));
        let calls = Arc::new(Mutex::new(Vec::new()));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let journal = Arc::new(Mutex::new(None::<PathBuf>));
        let broadcasts = Arc::new(Mutex::new(Vec::new()));
        let (served_requests, served_journal, served_broadcasts) = (
            Arc::clone(&requests),
            Arc::clone(&journal),
            Arc::clone(&broadcasts),
        );
        let (served_rules, served_calls) = (Arc::clone(&rules), Arc::clone(&calls));
        thread::spawn(move || {
            for mut stream in listener.incoming().flatten() {
                let outcome = (|| -> Outcome {
                    let (_, body) = read_http(&mut stream)?;
                    let request: Value = serde_json::from_slice(&body)?;
                    if request["method"] == "eth_sendRawTransaction" {
                        let directory = served_journal
                            .lock()
                            .unwrap_or_else(PoisonError::into_inner)
                            .clone()
                            .ok_or("broadcast without journal")?;
                        let records = std::fs::read_dir(directory)?
                            .map(|entry| -> Outcome<x_websearch::payment::PaymentRecord> {
                                Ok(serde_json::from_slice(&std::fs::read(entry?.path())?)?)
                            })
                            .collect::<Outcome<Vec<_>>>()?;
                        served_broadcasts
                            .lock()
                            .unwrap_or_else(PoisonError::into_inner)
                            .push(records);
                    }
                    served_requests
                        .lock()
                        .unwrap_or_else(PoisonError::into_inner)
                        .push(request);
                    let reply = answer(&served_rules, &served_calls, &body);
                    let response = reply.map_or_else(
                        || "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_owned(),
                        |reply| { let body = reply.to_string(); format!("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()) });
                    stream.write_all(response.as_bytes())?;
                    Ok(())
                })();
                if let Err(error) = outcome {
                    panic!("gateway recorder: {error}");
                }
            }
        });
        Ok(Self {
            address,
            rules,
            calls,
            requests,
            journal,
            broadcasts,
        })
    }

    fn calls(&self, method: &str) -> usize {
        self.calls
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .iter()
            .filter(|call| *call == method)
            .count()
    }

    fn rule_count(&self) -> usize {
        self.rules
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .len()
    }
}

/// The pinned conformance suite: the recorded gateway exchange the seller is
/// qualified against, by content digest and rule count.
fn conformance(gateway: &Gateway) -> Outcome<ConformanceSuite> {
    let mut names: Vec<PathBuf> = std::fs::read_dir(fixtures().join("gateway"))?
        .map(|entry| entry.map(|entry| entry.path()))
        .collect::<Result<_, _>>()?;
    names.sort();
    let mut digest = Sha256::new();
    for name in names {
        digest.update(std::fs::read(name)?);
    }
    let count = u64::try_from(gateway.rule_count())?;
    Ok(ConformanceSuite::new(
        AdapterId::new("x402-v2")?,
        count,
        digest.finalize().into(),
    )?)
}

struct Harness {
    gate: Option<Arc<PaymentGate>>,
    server: Option<RunningServer>,
    gateway: Gateway,
    served: Arc<AtomicUsize>,
    buyer: Value,
    scratch: PathBuf,
}

impl Drop for Harness {
    fn drop(&mut self) {
        if let Some(server) = self.server.take() {
            let _ = server.shutdown();
        }
        let _ = std::fs::remove_dir_all(&self.scratch);
    }
}

fn configure_with(
    scratch: &Path,
    gateway: &Gateway,
    buyer: &Value,
    payment: Option<Value>,
) -> Outcome<Config> {
    let mut config = read_json(&fixtures().join("config/valid.json"))?;
    config["data_dir"] = json!(scratch.join("data"));
    config["gateway"]["endpoint"] = json!(format!("http://{}/rpc", gateway.address));
    config["evm"]["endpoint"] = json!(format!("http://{}/rpc", gateway.address));
    let gateway_config = config["gateway"].as_object_mut().ok_or("gateway object")?;
    gateway_config.remove("sequencer_id");
    gateway_config.remove("sequencer_public_key");
    gateway_config.insert(
        "receipt_keys".to_owned(),
        json!([{
            "key_id": hex(&receipt_key_id()),
            "public_key": text(buyer, "/sequencerPublicKey")?,
            "first_height": 0,
            "last_height": u64::MAX,
        }]),
    );
    if let Some(payment) = payment {
        config["payment"] = payment;
    }
    Ok(Config::parse(&config.to_string())?)
}

fn receiver_key(scratch: &Path) -> Outcome<k256::ecdsa::SigningKey> {
    let path = scratch.join("receiver.key");
    std::fs::write(&path, RECEIVER_SECRET)?;
    std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600))?;
    let files = KeyFiles {
        attestor: None,
        submitter: None,
        receiver: path,
    };
    Ok(files.load()?.receiver().clone())
}

fn scratch_dir(name: &str) -> Outcome<PathBuf> {
    let scratch =
        std::env::temp_dir().join(format!("x-websearch-payment-{}-{name}", std::process::id()));
    let _ = std::fs::remove_dir_all(&scratch);
    std::fs::create_dir_all(&scratch)?;
    Ok(scratch)
}

impl Harness {
    fn start(name: &str, recordings: &[&str]) -> Outcome<Self> {
        Self::launch(name, Gateway::start(recordings)?, None)
    }

    /// Starts the sidecar against `gateway` with `payment` as the
    /// configuration's `payment` object.
    fn launch(name: &str, gateway: Gateway, payment: Option<Value>) -> Outcome<Self> {
        Self::launch_with_receipt_keys(name, gateway, payment, None)
    }

    fn launch_with_receipt_keys(
        name: &str,
        gateway: Gateway,
        payment: Option<Value>,
        receipt_keys: Option<Vec<SequencerAuthorization>>,
    ) -> Outcome<Self> {
        let scratch = scratch_dir(name)?;
        let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
        let mut config = configure_with(&scratch, &gateway, &buyer, payment)?;
        if let Some(keys) = receipt_keys {
            config.gateway.receipt_keys = keys;
        }
        let receiver = receiver_key(&scratch)?;
        let gate = Arc::new(PaymentGate::new(
            &config,
            &receiver,
            conformance(&gateway)?,
            fixed_clock,
        )?);
        *gateway
            .journal
            .lock()
            .unwrap_or_else(PoisonError::into_inner) = Some(scratch.join("data/payments/requests"));
        let served = Arc::new(AtomicUsize::new(0));
        let data = scratch.join("data");
        let store = Arc::new(ContentStore::open(&data, &[])?);
        let index = Arc::new(WebIndex::open(&data)?);
        let pages = vectors()?;
        for page in &pages {
            index.put(&page.payload, &page.title(), &page.text)?;
        }
        index.commit()?;
        let mut routes = RouteTable::new();
        let (counter, search_store) = (Arc::clone(&served), Arc::clone(&store));
        PaymentGate::install(
            &gate,
            &mut routes,
            Route::Search,
            move |request: &Request| {
                counter.fetch_add(1, Ordering::SeqCst);
                search_route(&index, &search_store, request)
            },
        )?;
        let (counter, fetch_store) = (Arc::clone(&served), Arc::clone(&store));
        PaymentGate::install(
            &gate,
            &mut routes,
            Route::Fetch,
            move |request: &Request| {
                counter.fetch_add(1, Ordering::SeqCst);
                fetch_page(&pages, &fetch_store, request)
            },
        )?;
        routes.set(Route::Content, move |request: &Request| {
            store.handle(request)
        })?;
        let running = Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?;
        Ok(Self {
            server: Some(running),
            gate: Some(gate),
            gateway,
            served,
            buyer,
            scratch,
        })
    }

    fn gate(&self) -> &Arc<PaymentGate> {
        self.gate.as_ref().expect("payment gate is running")
    }

    fn close_gate(&mut self) -> Outcome {
        if let Some(server) = self.server.take() {
            server.shutdown()?;
        }
        self.gate.take();
        Ok(())
    }

    fn delivery_server(&mut self) -> Outcome {
        if let Some(server) = self.server.take() {
            server.shutdown()?;
        }
        let store = ContentStore::open(&self.scratch.join("data"), &[])?;
        let pages = vectors()?;
        let counter = Arc::clone(&self.served);
        let mut routes = RouteTable::new();
        PaymentGate::install_validated(
            self.gate(),
            &mut routes,
            Route::Fetch,
            |request| match request.query_param("url") {
                Ok(Some(_)) => Ok(()),
                _ => Err(Response::error(400, "missing_url")),
            },
            move |request| {
                counter.fetch_add(1, Ordering::SeqCst);
                fetch_page(&pages, &store, request)
            },
        )?;
        self.server =
            Some(Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?);
        Ok(())
    }

    fn restart_deliveries(&mut self) -> Outcome {
        self.close_gate()?;
        let config = configure_with(&self.scratch, &self.gateway, &self.buyer, None)?;
        let gate = Arc::new(PaymentGate::new(
            &config,
            &receiver_key(&self.scratch)?,
            conformance(&self.gateway)?,
            fixed_clock,
        )?);
        gate.recover_pending_deliveries()?;
        self.gate = Some(gate);
        self.delivery_server()
    }

    fn address(&self) -> Outcome<SocketAddr> {
        self.server
            .as_ref()
            .map(RunningServer::local_addr)
            .ok_or_else(|| fail("server stopped"))
    }

    fn served(&self) -> usize {
        self.served.load(Ordering::SeqCst)
    }

    fn payer(&self) -> Outcome<&str> {
        text(&self.buyer, "/payerDid")
    }

    fn get(&self, target: &str, headers: &[(&str, &str)]) -> Outcome<Reply> {
        let mut stream = TcpStream::connect(self.address()?)?;
        stream.set_read_timeout(Some(Duration::from_secs(30)))?;
        let mut request = format!("GET {target} HTTP/1.1\r\nHost: 127.0.0.1\r\n");
        for (name, value) in headers {
            write!(request, "{name}: {value}\r\n")?;
        }
        request.push_str("\r\n");
        stream.write_all(request.as_bytes())?;
        let mut bytes = Vec::new();
        stream.read_to_end(&mut bytes)?;
        Reply::parse(&bytes)
    }

    /// Asks for the offers, picks the one for `scheme` and `currency`, and
    /// returns the `PAYMENT-SIGNATURE` paying it with `payload`.
    fn signature(
        &self,
        target: &str,
        payer: Option<&str>,
        scheme: &str,
        currency: &str,
        payload: &Value,
    ) -> Outcome<String> {
        let headers: Vec<(&str, &str)> =
            payer.map(|payer| (PAYER_DID, payer)).into_iter().collect();
        let challenge = self.get(target, &headers)?;
        if challenge.status != 402 {
            return Err(fail(format!(
                "expected a challenge, got {}",
                challenge.status
            )));
        }
        let required = challenge.decoded(PAYMENT_REQUIRED)?;
        let offer = required
            .get("accepts")
            .and_then(Value::as_array)
            .and_then(|offers| {
                offers.iter().find(|offer| {
                    offer.get("scheme").and_then(Value::as_str) == Some(scheme)
                        && offer
                            .pointer("/extra/layerx/currency")
                            .and_then(Value::as_str)
                            == Some(currency)
                })
            })
            .ok_or_else(|| fail(format!("no {scheme} offer in {currency}")))?;
        Ok(encode(
            &json!({ "x402Version": 2, "accepted": offer, "payload": payload }),
        ))
    }

    fn pay(&self, target: &str, payer: Option<&str>, signature: &str) -> Outcome<Reply> {
        let mut headers = vec![(PAYMENT_SIGNATURE, signature)];
        if let Some(payer) = payer {
            headers.push((PAYER_DID, payer));
        }
        self.get(target, &headers)
    }

    fn metered(&self, target: &str, currency: &str, grant: &str, label: &str) -> Outcome<Reply> {
        let payer = self.payer()?;
        let signature = self.signature(
            target,
            Some(payer),
            METERED,
            currency,
            &metered_payload(grant, label),
        )?;
        self.pay(target, Some(payer), &signature)
    }

    fn exact(&self, target: &str, currency: &str, payment: &Value) -> Outcome<Reply> {
        let signature = self.signature(target, None, EXACT, currency, payment)?;
        self.pay(target, None, &signature)
    }

    fn grant(&self, pointer: &str) -> Outcome<String> {
        Ok(text(&self.buyer, pointer)?.to_owned())
    }
}

fn encode(value: &Value) -> String {
    STANDARD.encode(value.to_string())
}

fn receive_key(label: &str) -> String {
    hex(&receive_key_bytes(label))
}

fn receive_key_bytes(label: &str) -> [u8; 32] {
    labelled("x-websearch-test/receive/", label)
}

fn labelled(domain: &str, label: &str) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain.as_bytes());
    digest.update(label.as_bytes());
    digest.finalize().into()
}

/// One page of `content-vectors.json`.
#[derive(Clone)]
struct Page {
    payload: String,
    media_type: String,
    text: String,
    digest: String,
}

impl Page {
    /// The title the crawler indexes: the first line of an HTML page's text.
    fn title(&self) -> String {
        if self.media_type == "text/html" {
            title_of(&self.text)
        } else {
            String::new()
        }
    }
}

fn vectors() -> Outcome<Vec<Page>> {
    let recorded = read_json(&fixtures().join("content-vectors.json"))?;
    recorded
        .get("vectors")
        .and_then(Value::as_array)
        .ok_or_else(|| fail("content vectors"))?
        .iter()
        .map(|vector| {
            Ok(Page {
                payload: text(vector, "/payload")?.to_owned(),
                media_type: text(vector, "/media_type")?.to_owned(),
                text: text(vector, "/text")?.to_owned(),
                digest: text(vector, "/digest")?.to_owned(),
            })
        })
        .collect()
}

/// The `/fetch` resource over the committed pages: the page recorded at the
/// requested URL, or the first page's text served at it, answered in the
/// fetch route's shape after its canonical bytes are written to the store.
fn fetch_page(pages: &[Page], store: &ContentStore, request: &Request) -> Response {
    let Ok(Some(url)) = request.query_param("url") else {
        return Response::error(400, "missing_url");
    };
    let Some(page) = pages
        .iter()
        .find(|page| page.payload == url)
        .or_else(|| pages.first())
    else {
        return Response::error(500, "no_pages");
    };
    let Ok(canonical) = canonical_bytes(
        ContentKind::Fetch,
        url.as_bytes(),
        &page.media_type,
        &page.text,
    ) else {
        return Response::error(500, "canonical");
    };
    let Ok(digest) = store.put(&canonical) else {
        return Response::error(500, "content_store_error");
    };
    Response::json(
        200,
        json!({
            "url": url,
            "final_url": url,
            "media_type": page.media_type,
            "digest": digest_hex(&digest),
            "length": page.text.len(),
            "text": page.text,
        })
        .to_string()
        .into_bytes(),
    )
}

/// A query value as `encodeURIComponent` and `quote(safe='')` write it.
fn percent(text: &str) -> String {
    let mut encoded = String::with_capacity(text.len());
    for byte in text.bytes() {
        if byte.is_ascii_alphanumeric() || b"-_.~".contains(&byte) {
            encoded.push(char::from(byte));
        } else {
            let _ = write!(encoded, "%{byte:02X}");
        }
    }
    encoded
}

fn metered_payload(grant: &str, label: &str) -> Value {
    let grant_id = grant.get(..64).unwrap_or(grant);
    json!({ "grantId": grant_id, "idempotencyKey": receive_key(label) })
}

struct Reply {
    status: u16,
    headers: Vec<(String, String)>,
    body: Value,
    raw: Vec<u8>,
}

impl Reply {
    fn parse(bytes: &[u8]) -> Outcome<Self> {
        let end = bytes
            .windows(4)
            .position(|window| window == b"\r\n\r\n")
            .ok_or_else(|| fail("response without a head"))?;
        let head = std::str::from_utf8(&bytes[..end])?;
        let mut lines = head.split("\r\n");
        let status = lines
            .next()
            .and_then(|line| line.split(' ').nth(1))
            .ok_or_else(|| fail("response without a status"))?
            .parse()?;
        let headers = lines
            .filter_map(|line| line.split_once(": "))
            .map(|(name, value)| (name.to_owned(), value.to_owned()))
            .collect();
        let raw = bytes[end + 4..].to_vec();
        let body = serde_json::from_slice(&raw).unwrap_or(Value::Null);
        Ok(Self {
            status,
            headers,
            body,
            raw,
        })
    }

    fn header(&self, name: &str) -> Option<&str> {
        self.headers
            .iter()
            .find(|(candidate, _)| candidate.eq_ignore_ascii_case(name))
            .map(|(_, value)| value.as_str())
    }

    fn decoded(&self, name: &str) -> Outcome<Value> {
        let header = self
            .header(name)
            .ok_or_else(|| fail(format!("no {name} header")))?;
        Ok(serde_json::from_slice(&STANDARD.decode(header)?)?)
    }

    fn error(&self) -> Option<&str> {
        self.body.get("error").and_then(Value::as_str)
    }
}

fn assert_settled(reply: &Reply) -> Outcome {
    assert_eq!(reply.status, 200, "{:?}", reply.body);
    let response = reply.decoded(PAYMENT_RESPONSE)?;
    assert_eq!(response.get("success"), Some(&json!(true)));
    Ok(())
}

fn assert_refused(reply: &Reply, reason: &str) {
    assert_eq!(
        (reply.status, reply.error()),
        (402, Some(reason)),
        "{:?}",
        reply.body
    );
    assert!(reply.header(PAYMENT_REQUIRED).is_some());
}

fn draw_event(grant: &Grant, label: &str) -> Outcome<SequencerLog> {
    let mut amount = [0; 32];
    amount[16..].copy_from_slice(&accepted("SID")?.price.to_be_bytes());
    Ok(SequencerLog::new(
        LEDGER_PRECOMPILE,
        vec![
            x_websearch::watch::keccak(
                b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)",
            ),
            grant.id,
            grant.from,
            grant.recipient,
        ],
        [grant.asset, amount, receive_key_bytes(label), [0; 32]].concat(),
        0,
    ))
}

fn draw_receipt_answer(hash: [u8; 32], logs: &[SequencerLog], status: u64) -> Outcome<Value> {
    fn encode(
        hash: [u8; 32],
        logs: &[SequencerLog],
        status: u64,
        signature: Option<[u8; 64]>,
    ) -> Outcome<Vec<u8>> {
        let mut summary = Vec::new();
        for value in [210_000u64, 210_000, 1] {
            summary.extend_from_slice(&value.to_be_bytes());
        }
        summary.extend_from_slice(&0u32.to_be_bytes());
        summary.extend_from_slice(&[0; 40]);
        summary.extend_from_slice(&Sha256::digest(b""));
        summary.extend_from_slice(&0u32.to_be_bytes());
        summary.extend_from_slice(&u32::try_from(logs.len())?.to_be_bytes());
        summary.push(4);
        summary.extend_from_slice(b"upax");
        let mut effects = vec![(1u8, summary)];
        for log in logs {
            let mut body = log.address().to_vec();
            body.push(u8::try_from(log.topics().len())?);
            for topic in log.topics() {
                body.extend_from_slice(topic);
            }
            body.extend_from_slice(&Sha256::digest(log.data()));
            effects.push((3, body));
        }
        let mut encoder = Encoder::new(16_384);
        wire(encoder.structure_header_version(0x5201, layerx_wire::limits::PROTOCOL_VERSION))?;
        wire(encoder.u16(layerx_wire::limits::PROTOCOL_VERSION))?;
        wire(encoder.bytes(&hash, 32))?;
        wire(encoder.u64(7))?;
        wire(encoder.bytes(&[4; 32], 32))?;
        let preimage = [
            b"PAXEER/v1/sequence\0".as_slice(),
            &[4; 32],
            &7u64.to_be_bytes(),
            &hash,
        ]
        .concat();
        wire(encoder.bytes(&Sha256::digest(&preimage), 32))?;
        wire(encoder.bytes(&Sha256::digest(b""), 32))?;
        wire(encoder.i32(if status == 1 { 0 } else { 1 }))?;
        wire(encoder.sequence_length(effects.len(), layerx_wire::limits::MAX_SEQUENCER_EFFECTS))?;
        for (index, (kind, body)) in effects.iter().enumerate() {
            wire(encoder.u16(12))?;
            wire(encoder.u16(u16::try_from(index)?))?;
            wire(encoder.u16(0))?;
            wire(encoder.tag(*kind, 3))?;
            wire(encoder.u8(0))?;
            wire(encoder.bytes(&[0; 32], 32))?;
            wire(encoder.bytes(body, 256))?;
        }
        wire(encoder.u128(1))?;
        let mut batch = [0; 32];
        batch[24..].copy_from_slice(&9u64.to_be_bytes());
        wire(encoder.bytes(&batch, 32))?;
        wire(encoder.u16(12))?;
        wire(encoder.u32(1))?;
        wire(encoder.u32(0))?;
        wire(encoder.u8(2))?;
        wire(encoder.bytes(&Sha256::digest(b"upax"), 32))?;
        wire(encoder.u128(0))?;
        let mut from = [0; 32];
        from[12..].copy_from_slice(&receiver_public_key()?);
        let mut to = [0; 32];
        to[12..].copy_from_slice(&LEDGER_PRECOMPILE);
        wire(encoder.bytes(&from, 32))?;
        wire(encoder.u128(100))?;
        wire(encoder.u128(99))?;
        wire(encoder.u64(7))?;
        wire(encoder.bytes(&to, 32))?;
        wire(encoder.u128(0))?;
        wire(encoder.u128(0))?;
        wire(encoder.bytes(&[0; 32], 32))?;
        wire(encoder.bytes(&[0; 32], 32))?;
        wire(encoder.bytes(&receipt_key_id(), 32))?;
        wire(encoder.u64(NOW_MS))?;
        wire(encoder.u8(u8::from(signature.is_some())))?;
        if let Some(signature) = signature {
            wire(encoder.bytes(&signature, 64))?;
        }
        Ok(encoder.finish())
    }
    let unsigned = encode(hash, logs, status, None)?;
    let signature = test_key("sequencer")
        .sign(&wire(receipt_digest(&unsigned))?)
        .to_bytes();
    let signed = encode(hash, logs, status, Some(signature))?;
    verify_sequencer_signature(
        &signed,
        ReceiptKeys::HeightRanges(&[SequencerAuthorization::new(
            receipt_key_id(),
            test_key("sequencer").verifying_key().to_bytes(),
            0,
            u64::MAX,
        )]),
    )
    .map_err(|error| format!("{error:?}"))?;
    let logs: Vec<Value> = logs.iter().map(|log| json!({
        "address":format!("0x{}", hex(&log.address())),
        "topics":log.topics().iter().map(|topic| format!("0x{}", hex(topic))).collect::<Vec<_>>(),
        "data":format!("0x{}", hex(log.data())), "logIndex":format!("0x{:x}", log.log_index()),
    })).collect();
    Ok(
        json!({"rung":"receipted", "txHash":format!("0x{}", hex(&hash)), "nonce":"0x7",
        "seq":"0x7", "height":"0x9", "status":format!("0x{status:x}"), "logs":logs,
        "keyId":format!("0x{}", hex(&receipt_key_id())), "receipt":format!("0x{}", hex(&signed))}),
    )
}

fn recover_signature(harness: &Harness, encoded: &str, label: &str) -> Outcome<String> {
    harness.signature(
        &format!("/search?q={label}"),
        Some(harness.payer()?),
        METERED,
        "SID",
        &metered_payload(encoded, label),
    )
}

fn receipt_rule(harness: &Harness, hash: [u8; 32], answer: Value, once: bool) -> Outcome {
    harness.gateway.rules.lock().unwrap_or_else(PoisonError::into_inner).extend(parse_rules(&[
        json!({"method":"pax_sequencerReceipt", "params":[format!("0x{}", hex(&hash))], "result":answer, "once":once}),
    ])?);
    Ok(())
}

#[test]
fn a_pending_draw_recovers_from_the_sequencer_receipt_and_releases_once() -> Outcome {
    for committed in [false, true] {
        let label = if committed {
            "recover-committed"
        } else {
            "recover-receipted"
        };
        let (harness, grant, encoded) = ledger_harness_recording(label, "pending.json")?;
        let signed = broadcast_draw(&harness, &grant, &encoded, label, 7)?;
        let signature = recover_signature(&harness, &encoded, label)?;
        let target = format!("/search?q={label}");
        harness.gateway.rules.lock().unwrap_or_else(PoisonError::into_inner).extend(parse_rules(&[
            json!({"method":"eth_sendRawTransaction", "params":[format!("0x{}", hex(&signed.raw))], "result":format!("0x{}", hex(&signed.hash))}),
        ])?);
        receipt_rule(
            &harness,
            signed.hash,
            json!({"rung":"submitted", "txHash":format!("0x{}", hex(&signed.hash)), "nonce":"0x7"}),
            true,
        )?;
        let mut answer = draw_receipt_answer(signed.hash, &[draw_event(&grant, label)?], 1)?;
        if committed {
            answer["rung"] = json!("committed");
            answer["blockNumber"] = json!("0xa");
            answer["blockHash"] = json!(format!("0x{}", "ab".repeat(32)));
        }
        receipt_rule(&harness, signed.hash, answer, false)?;
        for _ in 0..2 {
            let reply = harness.pay(&target, Some(harness.payer()?), &signature)?;
            assert_eq!(
                (reply.status, reply.error()),
                (503, Some("payment_pending"))
            );
            assert_eq!(harness.served(), 0);
        }
        assert_eq!(harness.gateway.calls("eth_sendRawTransaction"), 2);
        let snapshots = harness
            .gateway
            .broadcasts
            .lock()
            .unwrap_or_else(PoisonError::into_inner);
        assert_eq!(snapshots.len(), 2);
        assert_eq!(snapshots[0], snapshots[1]);
        drop(snapshots);
        assert_settled(&harness.pay(&target, Some(harness.payer()?), &signature)?)?;
        assert_refused(
            &harness.pay(&target, Some(harness.payer()?), &signature)?,
            "receipt_consumed",
        );
        assert_eq!(harness.served(), 1);
        let requests = harness
            .gateway
            .requests
            .lock()
            .unwrap_or_else(PoisonError::into_inner);
        let lookups: Vec<_> = requests
            .iter()
            .filter(|request| request["method"] == "pax_sequencerReceipt")
            .collect();
        assert_eq!(lookups.len(), 4);
        for lookup in lookups {
            assert_eq!(
                lookup["params"],
                json!([format!("0x{}", hex(&signed.hash))])
            );
        }
        drop(requests);
        for method in [
            "lx_sendActivity",
            "lx_getReceipt",
            "lx_getActivityStatus",
            "lx_getSequence",
        ] {
            assert_eq!(harness.gateway.calls(method), 0);
        }
        let records = std::fs::read_dir(harness.scratch.join("data/payments/requests"))?
            .map(|entry| -> Outcome<x_websearch::payment::PaymentRecord> {
                Ok(serde_json::from_slice(&std::fs::read(entry?.path())?)?)
            })
            .collect::<Outcome<Vec<_>>>()?;
        assert_eq!(records.len(), 1);
        assert!(records[0].released);
        let receipt =
            x_websearch::payment::unhex(records[0].receipt.as_deref().ok_or("missing receipt")?)
                .ok_or("receipt hex")?;
        let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt)?;
        assert_eq!(records[0].receipt_digest, Some(hex(&digest)));
        let receipts = harness.scratch.join("data/payments/receipts");
        assert_eq!(std::fs::read_dir(&receipts)?.count(), 1);
        assert_eq!(
            std::fs::read_to_string(receipts.join(hex(&digest)))?,
            std::fs::read_dir(harness.scratch.join("data/payments/requests"))?
                .next()
                .ok_or("missing payment record")??
                .file_name()
                .to_str()
                .ok_or("record name")?,
        );
    }
    Ok(())
}

#[test]
fn a_reverted_draw_releases_nothing() -> Outcome {
    let label = "reverted-draw";
    let (harness, grant, encoded) = ledger_harness(label)?;
    let signed = broadcast_draw(&harness, &grant, &encoded, label, 7)?;
    receipt_rule(
        &harness,
        signed.hash,
        draw_receipt_answer(signed.hash, &[], 0)?,
        false,
    )?;
    let signature = recover_signature(&harness, &encoded, label)?;
    assert_refused(
        &harness.pay(
            &format!("/search?q={label}"),
            Some(harness.payer()?),
            &signature,
        )?,
        "payment_failed",
    );
    assert_eq!(harness.served(), 0);
    assert_eq!(harness.gateway.calls("eth_sendRawTransaction"), 1);
    let records = std::fs::read_dir(harness.scratch.join("data/payments/requests"))?
        .map(|entry| -> Outcome<x_websearch::payment::PaymentRecord> {
            Ok(serde_json::from_slice(&std::fs::read(entry?.path())?)?)
        })
        .collect::<Outcome<Vec<_>>>()?;
    assert_eq!(records.len(), 1);
    assert!(!records[0].released);
    assert!(records[0].receipt.is_none());
    assert!(records[0].receipt_digest.is_none());
    assert_eq!(
        std::fs::read_dir(harness.scratch.join("data/payments/receipts"))?.count(),
        0
    );
    Ok(())
}

#[test]
fn a_draw_receipt_without_a_matching_payment_drawn_log_is_refused() -> Outcome {
    for case in [
        "missing",
        "amount",
        "recipient",
        "key",
        "grant",
        "address",
        "unsigned-log",
        "window",
        "seq",
        "height",
        "status",
    ] {
        let label = format!("draw-mismatch-{case}");
        let keys = if case == "window" {
            Some(vec![SequencerAuthorization::new(
                receipt_key_id(),
                test_key("sequencer").verifying_key().to_bytes(),
                10,
                20,
            )])
        } else {
            None
        };
        let (harness, grant, encoded) = ledger_harness_with_keys(&label, "metered.json", keys)?;
        let signed = broadcast_draw(&harness, &grant, &encoded, &label, 7)?;
        let event = draw_event(&grant, &label)?;
        let mut address = event.address();
        let mut topics = event.topics().to_vec();
        let mut data = event.data().to_vec();
        match case {
            "amount" => data[63] ^= 1,
            "recipient" => topics[3][0] ^= 1,
            "key" => data[64] ^= 1,
            "grant" => topics[1][0] ^= 1,
            "address" => address[0] ^= 1,
            _ => {}
        }
        let logs = if case == "missing" {
            vec![]
        } else {
            vec![SequencerLog::new(address, topics, data, 0)]
        };
        let mut answer = draw_receipt_answer(signed.hash, &logs, 1)?;
        match case {
            "unsigned-log" => answer["logs"][0]["data"] = json!(format!("0x{}", "00".repeat(128))),
            "seq" => answer["seq"] = json!("0x8"),
            "height" => answer["height"] = json!("0xa"),
            "status" => answer["status"] = json!("0x0"),
            _ => (),
        };
        receipt_rule(&harness, signed.hash, answer, false)?;
        let signature = recover_signature(&harness, &encoded, &label)?;
        assert_refused(
            &harness.pay(
                &format!("/search?q={label}"),
                Some(harness.payer()?),
                &signature,
            )?,
            if case == "window" {
                "receipt_unverified"
            } else {
                "receipt_mismatch"
            },
        );
        assert_eq!(harness.served(), 0);
        for entry in std::fs::read_dir(harness.scratch.join("data/payments/requests"))? {
            let record: x_websearch::payment::PaymentRecord =
                serde_json::from_slice(&std::fs::read(entry?.path())?)?;
            assert!(!record.released);
            assert!(record.receipt.is_none());
            assert!(record.receipt_digest.is_none());
        }
    }
    Ok(())
}

#[test]
fn metered_draws_settle_in_each_asset() -> Outcome {
    let harness = Harness::start("metered", &["metered.json"])?;
    for currency in CURRENCIES {
        let grant = harness.grant(&format!("/grants/{currency}"))?;
        let reply = harness.metered(
            &format!("/search?q={currency}"),
            currency,
            &grant,
            &format!("metered-{currency}"),
        )?;
        assert_settled(&reply)?;
        let response = reply.decoded(PAYMENT_RESPONSE)?;
        assert_eq!(response.get("network"), Some(&json!("eip155:1337")));
        assert_eq!(
            response.pointer("/extensions/layerx/purposeHash"),
            Some(&json!(hex(&purpose_hash(&receiver_public_key()?))))
        );
        let asset = accepted(currency)?;
        let payer = account_id(&wallet_account(harness.payer()?, &asset))
            .ok_or_else(|| fail("payer account"))?;
        assert_eq!(response.get("payer"), Some(&json!(hex(&payer))));
        assert_eq!(reply.body.get("query"), Some(&json!(currency)));
    }
    assert_eq!(harness.served(), 4);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 4);
    Ok(())
}

#[test]
fn pax_is_paid_into_the_main_accounts_and_every_other_asset_into_its_own() -> Outcome {
    let harness = Harness::start("accounts", &["refusals.json"])?;
    let payer = harness.payer()?;
    let receiver = receiver_did(&receiver_public_key()?);
    let offers = harness
        .get("/search?q=accounts", &[(PAYER_DID, payer)])?
        .decoded(PAYMENT_REQUIRED)?;
    let offers = offers
        .get("accepts")
        .and_then(Value::as_array)
        .ok_or_else(|| fail("offers"))?;
    for offer in offers {
        let currency = text(offer, "/extra/layerx/currency")?;
        let asset = accepted(currency)?;
        let (receiver_account, payer_account) = if currency == "PAX" {
            (
                format!("agent:{receiver}:main"),
                format!("agent:{payer}:main"),
            )
        } else {
            (
                format!("agent:{receiver}:asset:{}", asset.id_hex()),
                format!("agent:{payer}:asset:{}", asset.id_hex()),
            )
        };
        assert_eq!(text(offer, "/extra/layerx/account")?, receiver_account);
        let pay_to = account_id(&receiver_account).ok_or_else(|| fail("payee"))?;
        assert_eq!(text(offer, "/payTo")?, hex(&pay_to));
        if text(offer, "/scheme")? == METERED {
            let drawn = account_id(&payer_account).ok_or_else(|| fail("payer"))?;
            assert_eq!(text(offer, "/extra/layerx/payer")?, hex(&drawn));
        }
    }
    assert_eq!(offers.len(), 8);
    Ok(())
}

fn exact_exchange() -> Outcome<Value> {
    let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
    let ledger = read_json(&fixtures().join("gateway/metered.json"))?;
    let mut rules = Vec::new();
    let mut payments = serde_json::Map::new();
    for currency in CURRENCIES {
        let asset = accepted(currency)?;
        let label = format!("pay-exact-{currency}");
        let hash = labelled("x-websearch-test/transaction/", &label);
        let payer =
            account_id(&wallet_account(text(&buyer, "/payerDid")?, &asset)).ok_or("payer")?;
        let payee = fixture_word(&ledger["ledger"], &format!("/receiverAccounts/{currency}"))?;
        let mut amount = [0; 32];
        amount[16..].copy_from_slice(&asset.price.to_be_bytes());
        let event = SequencerLog::new(
            LEDGER_PRECOMPILE,
            vec![
                x_websearch::watch::keccak(
                    b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)",
                ),
                receive_key_bytes(&label),
                payer,
                payee,
            ],
            [asset.asset_id, amount].concat(),
            0,
        );
        let answer = draw_receipt_answer(hash, &[event], 1)?;
        let receipt = x_websearch::payment::unhex(
            text(&answer, "/receipt")?
                .strip_prefix("0x")
                .ok_or("receipt prefix")?,
        )
        .ok_or("receipt hex")?;
        let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt)?;
        payments.insert(
            currency.to_owned(),
            json!({
                "txHash": format!("0x{}", hex(&hash)),
                "receiptDigest": hex(&digest), "verificationLevel": "receipted",
                "idempotencyKey": receive_key(&label),
            }),
        );
        rules.push(json!({"method": "pax_sequencerReceipt", "params": [format!("0x{}", hex(&hash))], "result": answer}));
    }
    Ok(json!({"endpoints": {"gateway": {"*": rules}}, "payments": payments}))
}

fn exact_fixture(currency: &str) -> Outcome<Value> {
    read_json(&fixtures().join("gateway/exact.json"))?["payments"]
        .get(currency)
        .cloned()
        .ok_or_else(|| fail("exact payment"))
}

#[test]
fn an_exact_payment_settles_from_its_pay_exact_transaction() -> Outcome {
    assert_eq!(
        read_json(&fixtures().join("gateway/exact.json"))?,
        exact_exchange()?
    );
    let harness = Harness::start("pay-exact-transaction", &["exact.json"])?;
    for currency in CURRENCIES {
        let payment = exact_fixture(currency)?;
        let reply = harness.exact(&format!("/fetch?url={currency}"), currency, &payment)?;
        assert_settled(&reply)?;
        let response = reply.decoded(PAYMENT_RESPONSE)?;
        assert_eq!(response["transaction"], payment["txHash"]);
        assert_eq!(
            response["extensions"]["layerx"]["verificationLevel"],
            "receipted"
        );
        assert_eq!(
            response["extensions"]["layerx"]["receiptDigest"],
            payment["receiptDigest"]
        );
    }
    assert_eq!(harness.served(), 4);
    assert_eq!(harness.gateway.calls("pax_sequencerReceipt"), 4);
    for method in ["lx_getReceipt", "lx_sendActivity", "eth_sendRawTransaction"] {
        assert_eq!(harness.gateway.calls(method), 0);
    }
    Ok(())
}

#[test]
fn an_exact_transaction_already_consumed_is_refused() -> Outcome {
    let harness = Harness::start("pay-exact-consumed", &["exact.json"])?;
    let mut payment = exact_fixture("SID")?;
    assert_settled(&harness.exact("/fetch?url=first", "SID", &payment)?)?;
    payment["idempotencyKey"] = json!(receive_key("another-exact-request"));
    assert_refused(
        &harness.exact("/fetch?url=second", "SID", &payment)?,
        "receipt_consumed",
    );
    assert_eq!(harness.served(), 1);
    assert_eq!(
        std::fs::read_dir(harness.scratch.join("data/payments/receipts"))?.count(),
        1
    );
    assert_eq!(
        std::fs::read_dir(harness.scratch.join("data/payments/requests"))?.count(),
        1
    );
    Ok(())
}

#[test]
fn delivery_recovers_the_recorded_receipted_settlement() -> Outcome {
    let mut harness = Harness::start("receipted-delivery", &["exact.json"])?;
    harness.delivery_server()?;
    let target = "/fetch?url=recorded-delivery";
    let signature = harness.signature(target, None, EXACT, "SID", &exact_fixture("SID")?)?;
    let first = harness.pay(target, None, &signature)?;
    assert_settled(&first)?;
    assert_eq!(
        first.decoded(PAYMENT_RESPONSE)?["transaction"],
        exact_fixture("SID")?["txHash"]
    );
    harness.restart_deliveries()?;
    let replay = harness.pay(target, None, &signature)?;
    assert_settled(&replay)?;
    assert_eq!(first.raw, replay.raw);
    assert_eq!(
        first.header(PAYMENT_RESPONSE),
        replay.header(PAYMENT_RESPONSE)
    );
    assert_eq!(harness.served(), 1);
    assert_eq!(harness.gateway.calls("eth_sendRawTransaction"), 0);
    assert_eq!(harness.gateway.calls("lx_getReceipt"), 0);
    Ok(())
}

fn kernel_era_delivery(harness: &Harness, released: bool) -> Outcome<(PathBuf, String)> {
    let path = std::fs::read_dir(harness.scratch.join("data/payments/requests"))?
        .next()
        .ok_or("payment record")??
        .path();
    let mut record = read_json(&path)?;
    let asset = accepted("SID")?;
    let activity_id = labelled("x-websearch-test/activity/", "kernel-era-payment");
    let receipt = signed_receipt(
        &Movement {
            label: "kernel-era-payment",
            operation: 5,
            activity_id,
            asset: &asset,
            amount: asset.price,
            from: fixture_word(&record, "/payer")?,
            to: fixture_word(&record["offer"], "/payTo")?,
            fee: RECORDED_FEE,
        },
        &test_key("sequencer"),
    )?;
    let digest = hex(&wire(leaf_hash(&receipt))?);
    let previous_digest = text(&record, "/receipt_digest")?.to_owned();
    let object = record.as_object_mut().ok_or("payment object")?;
    object.remove("tx_hash");
    object.remove("raw_transaction");
    object.insert("activity".to_owned(), Value::Null);
    object.insert("activity_id".to_owned(), json!(hex(&activity_id)));
    record["attempted"] = json!(true);
    record["released"] = json!(released);
    record["receipt"] = json!(hex(&receipt));
    record["receipt_digest"] = json!(digest);
    std::fs::write(&path, serde_json::to_vec(&record)?)?;
    let key = path.file_name().ok_or("payment key")?;
    let receipts = harness.scratch.join("data/payments/receipts");
    std::fs::rename(receipts.join(previous_digest), receipts.join(&digest))?;
    let delivery_path = harness.scratch.join("data/payments/deliveries").join(key);
    let mut delivery = read_json(&delivery_path)?;
    delivery["receipt_digest"] = json!(digest);
    let headers = delivery["response"]["headers"]
        .as_array_mut()
        .ok_or("delivery headers")?;
    let header = headers
        .iter_mut()
        .find(|header| header[0] == PAYMENT_RESPONSE)
        .ok_or("payment response")?;
    let mut settlement: Value =
        serde_json::from_slice(&STANDARD.decode(header[1].as_str().ok_or("header text")?)?)?;
    settlement["transaction"] = json!(format!("lxp:{digest}"));
    settlement["extensions"]["layerx"] = json!({"receipt": STANDARD.encode(&receipt),
        "receiptDigest": digest, "verificationLevel": "sequencer-signed"});
    settlement["extensions"]["x-websearch"]["receiptDigest"] = json!(digest);
    let encoded = encode(&settlement);
    header[1] = json!(encoded);
    std::fs::write(delivery_path, serde_json::to_vec(&delivery)?)?;
    Ok((path, encoded))
}

#[test]
fn a_released_kernel_era_record_is_still_replayed() -> Outcome {
    let mut harness = Harness::start("released-kernel-record", &["exact.json"])?;
    harness.delivery_server()?;
    let target = "/fetch?url=kernel-replay";
    let signature = harness.signature(target, None, EXACT, "SID", &exact_fixture("SID")?)?;
    let first = harness.pay(target, None, &signature)?;
    assert_settled(&first)?;
    let (path, header) = kernel_era_delivery(&harness, true)?;
    let key =
        x_websearch::payment::unhex(path.file_name().ok_or("key")?.to_str().ok_or("key text")?)
            .ok_or("key hex")?
            .try_into()
            .map_err(|_| fail("key length"))?;
    let store = x_websearch::payment::PaymentStore::open(&harness.scratch.join("data"))?;
    let record = store.load(&key)?.ok_or("loaded legacy record")?;
    assert!(record.released);
    assert_eq!(record.tx_hash, None);
    assert_eq!(
        record.receipt_digest.as_deref(),
        read_json(&path)?["receipt_digest"].as_str()
    );
    let lookups = harness.gateway.calls("pax_sequencerReceipt");
    for _ in 0..2 {
        let replay = harness.pay(target, None, &signature)?;
        assert_settled(&replay)?;
        assert_eq!(replay.raw, first.raw);
        assert_eq!(replay.header(PAYMENT_RESPONSE), Some(header.as_str()));
    }
    assert_eq!(harness.served(), 1);
    assert_eq!(harness.gateway.calls("pax_sequencerReceipt"), lookups);
    Ok(())
}

#[test]
fn an_unreleased_kernel_era_record_stops_startup() -> Outcome {
    let mut harness = Harness::start("unreleased-kernel-record", &["exact.json"])?;
    harness.delivery_server()?;
    assert_settled(&harness.exact("/fetch?url=kernel-pending", "SID", &exact_fixture("SID")?)?)?;
    let (path, _) = kernel_era_delivery(&harness, false)?;
    harness.close_gate()?;
    let config = configure_with(&harness.scratch, &harness.gateway, &harness.buyer, None)?;
    let error = match PaymentGate::new(
        &config,
        &receiver_key(&harness.scratch)?,
        conformance(&harness.gateway)?,
        fixed_clock,
    ) {
        Err(GateError::Store(error)) => error,
        Err(error) => return Err(fail(format!("unexpected startup error: {error}"))),
        Ok(_) => return Err(fail("unreleased kernel activity accepted")),
    };
    assert_eq!(error.kind(), std::io::ErrorKind::InvalidData);
    assert!(error.to_string().contains(
        path.file_name()
            .ok_or("filename")?
            .to_str()
            .ok_or("filename text")?
    ));
    assert!(error.to_string().contains("unreleased kernel activity"));
    assert_eq!(harness.gateway.calls("eth_sendRawTransaction"), 0);
    assert_eq!(harness.served(), 1);
    Ok(())
}

#[test]
fn a_kernel_era_delivery_is_still_served() -> Outcome {
    let mut harness = Harness::start("kernel-delivery-restart", &["exact.json"])?;
    harness.delivery_server()?;
    let target = "/fetch?url=kernel-delivery";
    let signature = harness.signature(target, None, EXACT, "SID", &exact_fixture("SID")?)?;
    let first = harness.pay(target, None, &signature)?;
    assert_settled(&first)?;
    let (_, header) = kernel_era_delivery(&harness, true)?;
    let lookups = harness.gateway.calls("pax_sequencerReceipt");
    harness.restart_deliveries()?;
    let replay = harness.pay(target, None, &signature)?;
    assert_settled(&replay)?;
    assert_eq!(replay.raw, first.raw);
    assert_eq!(replay.header(PAYMENT_RESPONSE), Some(header.as_str()));
    assert_eq!(
        replay.decoded(PAYMENT_RESPONSE)?["extensions"]["layerx"]["verificationLevel"],
        "sequencer-signed"
    );
    assert_eq!(harness.gateway.calls("pax_sequencerReceipt"), lookups);
    assert_eq!(harness.served(), 1);
    assert_refused(
        &harness.pay("/fetch?url=other-resource", None, &signature)?,
        "request_mismatch",
    );
    assert_eq!(harness.served(), 1);
    Ok(())
}

#[test]
fn exact_receipts_settle_in_each_asset() -> Outcome {
    let harness = Harness::start("exact", &["exact.json"])?;
    for currency in CURRENCIES {
        let payment = harness
            .buyer
            .pointer(&format!("/exact/{currency}"))
            .cloned()
            .ok_or_else(|| fail("payment"))?;
        let reply = harness.exact(&format!("/fetch?url={currency}"), currency, &payment)?;
        assert_settled(&reply)?;
        let response = reply.decoded(PAYMENT_RESPONSE)?;
        assert_eq!(response.pointer("/extensions/layerx/purposeHash"), None);
        assert_eq!(reply.body.get("url"), Some(&json!(currency)));
    }
    assert_eq!(harness.served(), 4);
    assert_eq!(harness.gateway.calls("lx_getReceipt"), 4);
    Ok(())
}

#[test]
fn a_pending_draw_recovers_the_same_activity_and_releases_once() -> Outcome {
    let harness = Harness::start("pending", &["pending.json"])?;
    let payer = harness.payer()?;
    let grant = harness.grant("/grants/SID")?;
    let target = "/search?q=pending";
    let signature = harness.signature(
        target,
        Some(payer),
        METERED,
        "SID",
        &metered_payload(&grant, "pending-SID"),
    )?;
    for _ in 0..2 {
        let reply = harness.pay(target, Some(payer), &signature)?;
        assert_eq!(
            (reply.status, reply.error()),
            (503, Some("payment_pending"))
        );
        assert_eq!(reply.header("Retry-After"), Some("1"));
        assert_eq!(harness.served(), 0);
    }
    assert_settled(&harness.pay(target, Some(payer), &signature)?)?;
    assert_eq!(harness.served(), 1);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 1);
    assert_eq!(harness.gateway.calls("lx_getActivityStatus"), 2);
    assert_eq!(harness.gateway.calls("lx_getReceipt"), 1);
    assert_eq!(harness.gateway.calls("lx_getSequence"), 2);
    assert_refused(
        &harness.pay(target, Some(payer), &signature)?,
        "receipt_consumed",
    );
    assert_eq!(harness.served(), 1);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 1);
    Ok(())
}

#[test]
fn malformed_and_unmatched_payments_are_challenged() -> Outcome {
    let harness = Harness::start("challenge", &["refusals.json"])?;
    let payer = harness.payer()?;
    let target = "/search?q=challenge";
    let challenge = harness.get(target, &[(PAYER_DID, payer)])?;
    assert_refused(&challenge, "payment_required");
    let offers = challenge.decoded(PAYMENT_REQUIRED)?;
    assert_eq!(
        offers
            .pointer("/accepts")
            .and_then(Value::as_array)
            .map(Vec::len),
        Some(8)
    );
    assert_eq!(
        harness
            .get(target, &[])?
            .decoded(PAYMENT_REQUIRED)?
            .pointer("/accepts")
            .and_then(Value::as_array)
            .map(Vec::len),
        Some(4)
    );
    assert_refused(
        &harness.pay(target, Some(payer), "not a payment")?,
        "payment_invalid",
    );
    let grant = harness.grant("/grants/SID")?;
    let signature = harness.signature(
        target,
        Some(payer),
        METERED,
        "SID",
        &metered_payload(&grant, "altered"),
    )?;
    let mut altered: Value = serde_json::from_slice(&STANDARD.decode(&signature)?)?;
    altered["accepted"]["amount"] = json!("1");
    assert_refused(
        &harness.pay(target, Some(payer), &encode(&altered))?,
        "payment_invalid",
    );
    let mut redirected: Value = serde_json::from_slice(&STANDARD.decode(&signature)?)?;
    redirected["accepted"]["payTo"] = json!("11".repeat(32));
    assert_refused(
        &harness.pay(target, Some(payer), &encode(&redirected))?,
        "payment_invalid",
    );
    assert_eq!(
        harness
            .get("/search?q=x", &[(PAYER_DID, "did:Upper")])?
            .status,
        400
    );
    assert_eq!(harness.served(), 0);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 0);
    Ok(())
}

#[test]
fn grants_that_do_not_bind_this_draw_are_refused() -> Outcome {
    let harness = Harness::start("grants", &["refusals.json"])?;
    for (grant, reason) in [
        ("recurring", "grant_not_metered"),
        ("expired", "grant_expired"),
        ("otherPurpose", "grant_purpose_mismatch"),
        ("lowLimit", "grant_limit"),
        ("otherRecipient", "grant_recipient_mismatch"),
    ] {
        let grant = harness.grant(&format!("/refusedGrants/{grant}"))?;
        let reply = harness.metered(&format!("/search?q={reason}"), "SID", &grant, reason)?;
        assert_refused(&reply, reason);
        assert_eq!(
            reply.decoded(PAYMENT_RESPONSE)?.get("success"),
            Some(&json!(false))
        );
    }
    let grant = harness.grant("/grants/SID")?;
    let bogus = "00".repeat(32);
    assert_refused(
        &harness.metered("/search?q=bogus", "SID", &bogus, "bogus")?,
        "invalid_grant",
    );
    assert_refused(
        &harness.metered("/search?q=asset", "PAX", &grant, "asset")?,
        "grant_payer_mismatch",
    );
    let other = format!("did:layerx:{}", "11".repeat(32));
    let target = "/search?q=other-payer";
    let signature = harness.signature(
        target,
        Some(&other),
        METERED,
        "SID",
        &metered_payload(&grant, "other-payer"),
    )?;
    assert_refused(
        &harness.pay(target, Some(&other), &signature)?,
        "grant_payer_mismatch",
    );
    assert_eq!(harness.served(), 0);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 0);
    Ok(())
}

#[test]
fn draws_the_gateway_does_not_settle_release_nothing() -> Outcome {
    let harness = Harness::start("draws", &["refusals.json"])?;
    let grant = harness.grant("/grants/SID")?;
    for (label, reason) in [
        ("refused-other-receipt", "receipt_mismatch"),
        ("refused-untrusted", "receipt_unverified"),
        ("refused-failed", "payment_failed"),
        ("refused-other-activity", "receipt_mismatch"),
        ("refused-amount", "receipt_mismatch"),
    ] {
        assert_refused(
            &harness.metered(&format!("/search?q={label}"), "SID", &grant, label)?,
            reason,
        );
    }
    let pax = harness.grant("/grants/PAX")?;
    assert_refused(
        &harness.metered("/search?q=conflict", "PAX", &pax, "refused-failed")?,
        "idempotency_conflict",
    );
    let payer = harness.payer()?;
    let target = "/search?q=unknown";
    let signature = harness.signature(
        target,
        Some(payer),
        METERED,
        "SID",
        &metered_payload(&grant, "refused-unknown"),
    )?;
    let reply = harness.pay(target, Some(payer), &signature)?;
    assert_eq!(
        (reply.status, reply.error()),
        (503, Some("payment_pending"))
    );
    assert_refused(
        &harness.pay("/search?q=elsewhere", Some(payer), &signature)?,
        "request_mismatch",
    );
    assert_eq!(harness.served(), 0);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 6);
    Ok(())
}

#[test]
fn exact_receipts_that_do_not_pay_this_request_are_refused() -> Outcome {
    let harness = Harness::start("exact-refusals", &["exact.json"])?;
    for (payment, reason) in [
        ("/refusedExact/unknown", "receipt_unknown"),
        ("/refusedExact/otherPayee", "receipt_mismatch"),
        ("/refusedExact/untrusted", "receipt_unverified"),
    ] {
        let payment = harness
            .buyer
            .pointer(payment)
            .cloned()
            .ok_or_else(|| fail("payment"))?;
        assert_refused(
            &harness.exact(&format!("/fetch?url={reason}"), "SID", &payment)?,
            reason,
        );
    }
    let sid = harness
        .buyer
        .pointer("/exact/SID")
        .cloned()
        .ok_or_else(|| fail("payment"))?;
    let mut mismatched = sid.clone();
    mismatched["receiptDigest"] = harness
        .buyer
        .pointer("/exact/PAX/receiptDigest")
        .cloned()
        .unwrap_or(Value::Null);
    assert_refused(
        &harness.exact("/fetch?url=digest", "SID", &mismatched)?,
        "receipt_digest_mismatch",
    );
    assert_refused(
        &harness.exact("/fetch?url=asset", "PAX", &sid)?,
        "receipt_mismatch",
    );
    let mut unleveled = sid.clone();
    unleveled["verificationLevel"] = json!("self-reported");
    assert_refused(
        &harness.exact("/fetch?url=level", "SID", &unleveled)?,
        "invalid_payment_payload",
    );
    assert_settled(&harness.exact("/fetch?url=first", "SID", &sid)?)?;
    let mut again = sid;
    again["idempotencyKey"] = json!(receive_key("again"));
    assert_refused(
        &harness.exact("/fetch?url=second", "SID", &again)?,
        "receipt_consumed",
    );
    assert_eq!(harness.served(), 1);
    Ok(())
}

#[test]
fn the_configured_payer_is_offered_and_drawn_from() -> Outcome {
    let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
    let payer = text(&buyer, "/payerDid")?.to_owned();
    let harness = Harness::launch(
        "configured-payer",
        Gateway::start(&["metered.json"])?,
        Some(json!({ "payer_did": payer.as_str() })),
    )?;
    let target = "/search?q=SID";
    let offers = harness.get(target, &[])?.decoded(PAYMENT_REQUIRED)?;
    let offers = offers
        .get("accepts")
        .and_then(Value::as_array)
        .ok_or_else(|| fail("offers"))?;
    assert_eq!(offers.len(), 8);
    for offer in offers {
        if text(offer, "/scheme")? == METERED {
            let asset = accepted(text(offer, "/extra/layerx/currency")?)?;
            let drawn = account_id(&wallet_account(&payer, &asset)).ok_or_else(|| fail("payer"))?;
            assert_eq!(text(offer, "/extra/layerx/payer")?, hex(&drawn));
        }
    }
    let other = format!("did:layerx:{}", "11".repeat(32));
    let mismatch = harness.get(target, &[(PAYER_DID, other.as_str())])?;
    assert_eq!(
        (mismatch.status, mismatch.error()),
        (400, Some("payer_mismatch"))
    );
    assert_eq!(
        harness.get(target, &[(PAYER_DID, payer.as_str())])?.status,
        402
    );
    let grant = harness.grant("/grants/SID")?;
    let signature = harness.signature(
        target,
        None,
        METERED,
        "SID",
        &metered_payload(&grant, "metered-SID"),
    )?;
    let reply = harness.pay(target, None, &signature)?;
    assert_settled(&reply)?;
    let sid = accepted("SID")?;
    let drawn = account_id(&wallet_account(&payer, &sid)).ok_or_else(|| fail("payer"))?;
    assert_eq!(
        reply.decoded(PAYMENT_RESPONSE)?.get("payer"),
        Some(&json!(hex(&drawn)))
    );
    let recording = read_json(&fixtures().join("gateway/metered.json"))?;
    let submitted = recording
        .pointer("/endpoints/gateway/*")
        .and_then(Value::as_array)
        .and_then(|rules| {
            rules
                .iter()
                .find(|rule| rule.get("method") == Some(&json!("lx_sendActivity")))
        })
        .ok_or_else(|| fail("recorded draw"))?;
    assert!(text(submitted, "/params/0")?.contains(&hex(&drawn)));
    assert_eq!(harness.served(), 1);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 1);
    Ok(())
}

#[test]
fn a_receipt_outside_every_configured_key_window_is_refused() -> Outcome {
    let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
    let sid = accepted("SID")?;
    let receiver = receiver_secret()?;
    let payee = receiver_did(&x_websearch::attest::signer_address(&receiver));
    let key = test_key("sequencer");
    let activity_id = labelled("x-websearch-test/activity/", "outside-key-windows");
    let receipt = signed_receipt(
        &Movement {
            label: "outside-key-windows",
            operation: 5,
            activity_id,
            asset: &sid,
            amount: sid.price,
            from: account_id(&wallet_account(text(&buyer, "/payerDid")?, &sid)).ok_or("payer")?,
            to: account_id(&wallet_account(&payee, &sid)).ok_or("payee")?,
            fee: RECORDED_FEE,
        },
        &key,
    )?;
    let covering = [SequencerAuthorization::new(
        receipt_key_id(),
        key.verifying_key().to_bytes(),
        9,
        9,
    )];
    let verified = wire(verify_sequencer_signature(
        &receipt,
        ReceiptKeys::HeightRanges(&covering),
    ))?;
    let facts = verified.protocol().ok_or("protocol receipt")?;
    assert_eq!(facts.activity_id(), activity_id);
    assert_eq!(&facts.batch_id()[24..], &9_u64.to_be_bytes());
    let windows = vec![
        SequencerAuthorization::new(receipt_key_id(), key.verifying_key().to_bytes(), 0, 8),
        SequencerAuthorization::new(
            labelled("x-websearch-test/receipt-key/", "next"),
            test_key("next").verifying_key().to_bytes(),
            10,
            20,
        ),
    ];
    let rules = [json!({
        "method": "lx_getReceipt", "params": [hex(&activity_id)],
        "result": completed(&activity_id, &receipt),
    })];
    let harness = Harness::launch_with_receipt_keys(
        "outside-key-windows",
        Gateway::serve_rules(&rules)?,
        None,
        Some(windows),
    )?;
    let reply = harness.exact(
        "/fetch?url=outside-key-windows",
        "SID",
        &exact_payment(&receipt)?,
    )?;
    assert_refused(&reply, "receipt_unverified");
    assert_eq!(
        reply.decoded(PAYMENT_RESPONSE)?.get("success"),
        Some(&json!(false))
    );
    assert_eq!(harness.served(), 0);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 0);
    assert_eq!(harness.gateway.calls("lx_getReceipt"), 0);
    assert_eq!(
        std::fs::read_dir(harness.scratch.join("data/payments/receipts"))?.count(),
        0
    );
    assert_eq!(
        std::fs::read_dir(harness.scratch.join("data/payments/deliveries"))?.count(),
        0
    );
    Ok(())
}

#[test]
fn a_draw_charged_above_the_configured_fee_limit_is_refused() -> Outcome {
    const LIMIT: u128 = 5;
    let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
    let receiver = receiver_secret()?;
    let receiver_did = receiver_did(&x_websearch::attest::signer_address(&receiver));
    let sid = accepted("SID")?;
    let encoded = text(&buyer, "/grants/SID")?;
    let bytes = x_websearch::payment::unhex(encoded).ok_or("grant bytes")?;
    let Payment::IssueGrant(grant) = wire(Payment::decode(
        layerx_types::payload::ModuleId::Asset,
        7,
        &bytes,
        receiver_did.as_bytes(),
    ))?
    else {
        return Err(fail("grant"));
    };
    let pay_to = account_id(&wallet_account(&receiver_did, &sid)).ok_or_else(|| fail("payee"))?;
    let from = account_id(&wallet_account(text(&buyer, "/payerDid")?, &sid))
        .ok_or_else(|| fail("payer"))?;
    let mut list = vec![
        identity_rule(&receiver_did, IDENTITY_SEQUENCE, false),
        account_rule(&pay_to),
    ];
    let recording = read_json(&fixtures().join("gateway/metered.json"))?;
    list.extend(
        recording["endpoints"]["gateway"]["*"]
            .as_array()
            .ok_or("rules")?
            .iter()
            .filter(|rule| {
                rule["method"]
                    .as_str()
                    .is_some_and(|name| name.starts_with("eth_"))
            })
            .cloned(),
    );
    for (label, fee) in [("fee-above-limit", LIMIT + 1), ("fee-at-limit", LIMIT)] {
        let signed = test_draw(
            &receiver,
            &grant,
            sid.price,
            IDENTITY_SEQUENCE,
            receive_key_bytes(label),
        )?;
        let receipt = signed_receipt(
            &Movement {
                label,
                operation: 6,
                activity_id: signed.hash,
                asset: &sid,
                amount: sid.price,
                from,
                to: pay_to,
                fee,
            },
            &test_key("sequencer"),
        )?;
        list.push(send_rule(
            &signed.raw,
            ("result", completed(&signed.hash, &receipt)),
        ));
    }
    let harness = Harness::launch(
        "fee-limit",
        Gateway::serve_rules(&list)?,
        Some(json!({ "draw_fee_limit": LIMIT.to_string() })),
    )?;
    let refused = harness.metered(
        "/search?q=fee-above-limit",
        "SID",
        encoded,
        "fee-above-limit",
    )?;
    assert_refused(&refused, "draw_fee_limit_exceeded");
    assert_eq!(
        refused.decoded(PAYMENT_RESPONSE)?.get("success"),
        Some(&json!(false))
    );
    assert_eq!(harness.served(), 0);
    assert_settled(&harness.metered("/search?q=fee-at-limit", "SID", encoded, "fee-at-limit")?)?;
    assert_eq!(harness.served(), 1);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 2);
    Ok(())
}

#[test]
fn the_gate_reads_its_payment_settings_and_checks_the_configured_suite() -> Outcome {
    let scratch = scratch_dir("settings")?;
    let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
    let gateway = Gateway::start(&["metered.json"])?;
    let receiver = receiver_key(&scratch)?;
    let defaults = PaymentGate::new(
        &configure_with(&scratch, &gateway, &buyer, None)?,
        &receiver,
        conformance(&gateway)?,
        fixed_clock,
    )?;
    assert_eq!(defaults.draw_fee_limit(), DEFAULT_DRAW_FEE_LIMIT);
    assert_eq!(defaults.payer(), None);
    drop(defaults);
    let payer = text(&buyer, "/payerDid")?;
    let configured = PaymentGate::new(
        &configure_with(
            &scratch,
            &gateway,
            &buyer,
            Some(json!({ "payer_did": payer, "draw_fee_limit": "42" })),
        )?,
        &receiver,
        conformance(&gateway)?,
        fixed_clock,
    )?;
    assert_eq!(configured.draw_fee_limit(), 42);
    assert_eq!(configured.payer(), Some(payer));
    drop(configured);
    let mut unusable = configure_with(&scratch, &gateway, &buyer, None)?;
    unusable.payment.payer_did = Some("did:Upper".to_owned());
    assert!(matches!(
        PaymentGate::new(&unusable, &receiver, conformance(&gateway)?, fixed_clock),
        Err(GateError::Payer)
    ));
    let recorded = configure_with(
        &scratch,
        &gateway,
        &buyer,
        Some(json!({ "conformance_suite": fixtures().join("gateway") })),
    )?;
    let pinned = PaymentGate::new(
        &recorded,
        &receiver,
        x_websearch::conformance_suite()?,
        fixed_clock,
    )?;
    assert_eq!(pinned.draw_fee_limit(), DEFAULT_DRAW_FEE_LIMIT);
    let other_digest = ConformanceSuite::new(
        AdapterId::new(x_websearch::CONFORMANCE_SUITE)?,
        x_websearch::CONFORMANCE_VECTOR_COUNT,
        [0x5a; 32],
    )?;
    for suite in [conformance(&gateway)?, other_digest] {
        assert!(matches!(
            PaymentGate::new(&recorded, &receiver, suite, fixed_clock),
            Err(GateError::Conformance)
        ));
    }
    for directory in [fixtures().join("config"), scratch.join("absent")] {
        let config = configure_with(
            &scratch,
            &gateway,
            &buyer,
            Some(json!({ "conformance_suite": directory })),
        )?;
        assert!(matches!(
            PaymentGate::new(
                &config,
                &receiver,
                x_websearch::conformance_suite()?,
                fixed_clock
            ),
            Err(GateError::Conformance)
        ));
    }
    std::fs::remove_dir_all(&scratch)?;
    Ok(())
}

fn receiver_secret() -> Outcome<k256::ecdsa::SigningKey> {
    let secret: [u8; 32] = x_websearch::payment::unhex(RECEIVER_SECRET)
        .and_then(|bytes| bytes.try_into().ok())
        .ok_or_else(|| fail("receiver secret"))?;
    Ok(k256::ecdsa::SigningKey::from_slice(&secret)?)
}

fn receiver_public_key() -> Outcome<[u8; 20]> {
    Ok(x_websearch::attest::signer_address(&receiver_secret()?))
}

fn accepted(currency: &str) -> Outcome<AcceptedAsset> {
    let mut value = read_json(&fixtures().join("config/valid.json"))?;
    value["gateway"] = json!({
        "endpoint": "http://127.0.0.1:8547/rpc",
        "receipt_keys": [{
            "key_id": hex(&receipt_key_id()),
            "public_key": hex(&test_key("sequencer").verifying_key().to_bytes()),
            "first_height": 0, "last_height": u64::MAX,
        }],
    });
    let config = Config::parse(&value.to_string())?;
    let assets = AcceptedAssets::new(&config.assets).map_err(|error| fail(format!("{error:?}")))?;
    assets
        .all()
        .iter()
        .find(|asset| asset.symbol.code() == currency)
        .copied()
        .ok_or_else(|| fail(format!("no {currency} asset")))
}

fn wire<T, E: std::fmt::Debug>(result: Result<T, E>) -> Outcome<T> {
    result.map_err(|error| fail(format!("{error:?}")))
}

/// The deterministic test keys the gateway recording is signed with. Only
/// their public halves and signatures reach the fixtures.
fn test_key(label: &str) -> SigningKey {
    SigningKey::from_bytes(&labelled("x-websearch-test/key/", label))
}

fn receipt_key_id() -> [u8; 32] {
    labelled("x-websearch-test/receipt-key/", "sequencer")
}

/// One asset movement a recorded receipt attests.
struct Movement<'a> {
    label: &'a str,
    operation: u8,
    activity_id: [u8; 32],
    asset: &'a AcceptedAsset,
    amount: u128,
    from: [u8; 32],
    to: [u8; 32],
    fee: u128,
}

/// The fee every recorded receipt charges.
const RECORDED_FEE: u128 = 1;

fn encode_receipt(movement: &Movement<'_>, signature: Option<[u8; 64]>) -> Outcome<Vec<u8>> {
    let root = |field: &str| {
        labelled(
            "x-websearch-test/receipt/",
            &format!("{}/{field}", movement.label),
        )
    };
    let amount = movement.amount;
    let mut encoder = Encoder::new(4_096);
    wire(encoder.structure_header_version(0x5201, PROTOCOL_VERSION))?;
    wire(encoder.u16(PROTOCOL_VERSION))?;
    wire(encoder.bytes(&movement.activity_id, 32))?;
    wire(encoder.u64(9))?;
    wire(encoder.bytes(&root("previous-state"), 32))?;
    wire(encoder.bytes(&root("resulting-state"), 32))?;
    wire(encoder.bytes(&root("activity"), 32))?;
    wire(encoder.i32(0))?;
    wire(encoder.sequence_length(0, 512))?;
    wire(encoder.u128(movement.fee))?;
    let mut batch = [0; 32];
    batch[24..].copy_from_slice(&9_u64.to_be_bytes());
    wire(encoder.bytes(&batch, 32))?;
    wire(encoder.u16(1))?;
    wire(encoder.u32(1))?;
    wire(encoder.u32(1))?;
    wire(encoder.u8(movement.operation))?;
    wire(encoder.bytes(&movement.asset.asset_id, 32))?;
    wire(encoder.u128(amount))?;
    wire(encoder.bytes(&movement.from, 32))?;
    wire(encoder.u128(amount * 100))?;
    wire(encoder.u128(amount * 99))?;
    wire(encoder.u64(1))?;
    wire(encoder.bytes(&movement.to, 32))?;
    wire(encoder.u128(amount))?;
    wire(encoder.u128(amount * 2))?;
    wire(encoder.bytes(&root("transfer-set"), 32))?;
    wire(encoder.bytes(&root("authorization"), 32))?;
    wire(encoder.bytes(&receipt_key_id(), 32))?;
    wire(encoder.u64(1_000))?;
    wire(encoder.u8(u8::from(signature.is_some())))?;
    if let Some(signature) = signature {
        wire(encoder.bytes(&signature, 64))?;
    }
    Ok(encoder.finish())
}

/// The canonical receipt `sequencer` signs for `movement`.
fn signed_receipt(movement: &Movement<'_>, sequencer: &SigningKey) -> Outcome<Vec<u8>> {
    let digest = wire(receipt_digest(&encode_receipt(movement, None)?))?;
    encode_receipt(movement, Some(sequencer.sign(&digest).to_bytes()))
}

fn exact_payment(receipt: &[u8]) -> Outcome<Value> {
    Ok(json!({
        "receipt": STANDARD.encode(receipt),
        "receiptDigest": hex(&wire(leaf_hash(receipt))?),
        "verificationLevel": "sequencer-signed",
    }))
}

/// What a payer-signed grant binds besides its signer.
struct GrantTerms {
    from: [u8; 32],
    recipient: [u8; 32],
    asset: [u8; 32],
    per_draw_maximum: u128,
    allowance: u128,
    window_length: u64,
    expiration: u64,
    purpose: [u8; 32],
}

fn signed_grant(payer: &SigningKey, terms: &GrantTerms, actor: &str) -> Outcome<(Grant, String)> {
    let recurring = terms.window_length != 0;
    let mut body = Vec::with_capacity(250);
    body.extend_from_slice(&terms.from);
    body.extend_from_slice(&terms.recipient);
    body.extend_from_slice(&terms.asset);
    body.extend_from_slice(&terms.per_draw_maximum.to_be_bytes());
    body.extend_from_slice(&terms.allowance.to_be_bytes());
    body.push(u8::from(recurring));
    body.extend_from_slice(&terms.window_length.to_be_bytes());
    body.extend_from_slice(&terms.expiration.to_be_bytes());
    body.extend_from_slice(&terms.purpose);
    body.push(0);
    body.extend_from_slice(&[0; 32]);
    body.extend_from_slice(&0_u64.to_be_bytes());
    body.extend_from_slice(&payer.verifying_key().to_bytes());
    let mut id = Sha256::new();
    id.update(Domain::AuthorityHash.tag());
    id.update(b"LXP:GRANT:v1");
    id.update(&body);
    let id: [u8; 32] = id.finalize().into();
    let grant = Grant {
        id,
        from: terms.from,
        recipient: terms.recipient,
        asset: terms.asset,
        per_draw_maximum: terms.per_draw_maximum,
        allowance: terms.allowance,
        recurring,
        window_length: terms.window_length,
        expiration: terms.expiration,
        purpose_hash: terms.purpose,
        has_reference: false,
        reference_hash: [0; 32],
        revocation_sequence: 0,
        public_key: payer.verifying_key().to_bytes(),
        signature: payer.sign(&id).to_bytes(),
    };
    let encoded = wire(Payment::IssueGrant(grant.clone()).encode(actor.as_bytes()))?;
    Ok((grant, hex(&encoded)))
}

fn rules(list: &[Value]) -> Value {
    json!({ "endpoints": { "gateway": { "*": list } } })
}

fn identity_rule(did: &str, sequence: u64, once: bool) -> Value {
    let mut rule = json!({
        "method": "lx_getSequence",
        "params": [did, "identity"],
        "result": {
            "did": did,
            "next_sequence": sequence.to_string(),
            "verification": "authenticated_node_snapshot",
        },
    });
    if once {
        rule["once"] = json!(true);
    }
    rule
}

fn account_rule(account: &[u8; 32]) -> Value {
    json!({
        "method": "lx_getSequence",
        "params": [hex(account)],
        "result": { "id": hex(account), "next_sequence": RECEIVER_SEQUENCE.to_string() },
    })
}

fn send_rule(activity: &[u8], answer: (&str, Value)) -> Value {
    let (kind, value) = answer;
    let mut rule = json!({ "method": "lx_sendActivity", "params": [hex(activity), "executed"] });
    rule[kind] = value;
    rule
}

fn completed(activity_id: &[u8; 32], receipt: &[u8]) -> Value {
    json!({ "activity_id": hex(activity_id), "receipt": hex(receipt), "state": "completed" })
}

const RECEIVER_SEQUENCE: u64 = 3;
const IDENTITY_SEQUENCE: u64 = 7;
const GRANT_EXPIRATION: u64 = NOW_MS / 1_000 + 86_400;

fn ledger_answer(canonical: &str) -> String {
    let length = canonical.len() / 2;
    let absent = canonical.bytes().all(|byte| byte == b'0');
    let counters = if absent { [0, 0, 0] } else { [17, 3, 5] };
    let mut answer = String::from("0x");
    for word in [224, counters[0], counters[1], counters[2], 0, 0, 0, length] {
        let _ = write!(answer, "{word:064x}");
    }
    answer.push_str(canonical);
    answer.push_str(&"00".repeat((32 - length % 32) % 32));
    answer
}

fn ledger_recording(buyer: &Value) -> Outcome<Value> {
    let mut grants = Vec::new();
    for category in ["grants", "refusedGrants"] {
        for (name, encoded) in buyer[category].as_object().ok_or("grants")? {
            let encoded = encoded.as_str().ok_or("canonical grant")?;
            grants.push(json!({
                "name": name, "grantId": &encoded[..64], "canonical": encoded,
                "result": ledger_answer(encoded),
            }));
        }
    }
    for (name, grant_id, canonical) in [
        ("absent", "00".repeat(32), "00".repeat(346)),
        ("empty", "ff".repeat(32), String::new()),
    ] {
        grants.push(json!({
            "name": name, "grantId": grant_id, "canonical": canonical,
            "result": ledger_answer(&canonical),
        }));
    }
    grants.push(json!({
        "name": "reverted", "grantId": "ee".repeat(32), "canonical": "",
        "error": {"code": 3, "message": "execution reverted: LXP_NO_PAYER_GRANT"},
    }));
    let mut vectors = Vec::new();
    for (name, amount, deadline) in [
        (
            "mixed",
            0x0102_0304_0506_0708_090a_0b0c_0d0e_0f10_u128,
            0x0102_0304_0506_0708_u64,
        ),
        ("zero", 0, 0),
        ("maximum", u128::MAX, u64::MAX),
    ] {
        let grant_id = "11".repeat(32);
        let recipient = "22".repeat(32);
        let key = "44".repeat(32);
        let purpose = "55".repeat(32);
        let trace = "66".repeat(32);
        vectors.push(json!({
            "name": name, "grantId": grant_id, "recipient": recipient,
            "amount": amount.to_string(), "idempotencyKey": key,
            "deadlineMs": deadline.to_string(), "purpose": purpose, "trace": trace,
            "calldata": format!("2e89490f{grant_id}{recipient}{amount:064x}{key}{deadline:064x}{purpose}{trace}"),
        }));
    }
    Ok(json!({
        "source": "precompiles/layerxledger/abi.json; independent ABI words, big-endian integers and right-padded dynamic bytes",
        "drawVectors": vectors, "grants": grants,
    }))
}

fn ledger_rules(ledger: &Value) -> Outcome<Vec<Value>> {
    let mut rules = vec![json!({ "method": "eth_chainId", "params": [], "result": "0x539" })];
    for grant in ledger["grants"].as_array().ok_or("ledger grants")? {
        let mut rule = json!({
            "method": "eth_call",
            "params": [{
                "to": "0x000000000000000000000000000000000000101a",
                "data": format!("0x423fe4b2{}", text(grant, "/grantId")?),
            }, "pending"],
        });
        let kind = if grant.get("error").is_some() {
            "error"
        } else {
            "result"
        };
        rule[kind] = grant[kind].clone();
        rules.push(rule);
    }
    Ok(rules)
}

fn fixture_word(value: &Value, pointer: &str) -> Outcome<[u8; 32]> {
    x_websearch::payment::unhex(text(value, pointer)?)
        .and_then(|bytes| bytes.try_into().ok())
        .ok_or_else(|| fail(format!("invalid fixture word {pointer}")))
}

#[test]
fn draw_calldata_matches_the_ledger_draw_selector_and_layout() -> Outcome {
    assert_eq!(
        DRAW_SIGNATURE,
        "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)"
    );
    assert_eq!(
        hex(&LEDGER_PRECOMPILE),
        "000000000000000000000000000000000000101a"
    );
    let recording = read_json(&fixtures().join("gateway/metered.json"))?;
    let vectors = recording["ledger"]["drawVectors"]
        .as_array()
        .ok_or("draw vectors")?;
    assert_eq!(vectors.len(), 3);
    for vector in vectors {
        let amount: u128 = text(vector, "/amount")?.parse()?;
        let deadline: u64 = text(vector, "/deadlineMs")?.parse()?;
        let calldata = draw_calldata(
            &fixture_word(vector, "/grantId")?,
            &fixture_word(vector, "/recipient")?,
            amount,
            &fixture_word(vector, "/idempotencyKey")?,
            deadline,
            &fixture_word(vector, "/purpose")?,
            &fixture_word(vector, "/trace")?,
        );
        assert_eq!(calldata.len(), 228);
        assert_eq!(&calldata[..4], &[0x2e, 0x89, 0x49, 0x0f]);
        assert_eq!(hex(&calldata), text(vector, "/calldata")?);
        assert_eq!(&calldata[84..100], &amount.to_be_bytes());
        assert_eq!(&calldata[156..164], &deadline.to_be_bytes());
    }
    Ok(())
}

#[test]
fn the_payer_grant_is_read_from_the_ledger_precompile() -> Outcome {
    let recording = read_json(&fixtures().join("gateway/metered.json"))?;
    let gateway = Gateway::start(&["metered.json"])?;
    let rpc = SettlementRpc::new(&format!("http://{}/rpc", gateway.address), 1337)?;
    let grants = recording["ledger"]["grants"].as_array().ok_or("grants")?;
    assert_eq!(grants.len(), 12);
    for entry in grants {
        let id = fixture_word(entry, "/grantId")?;
        let answer = read_payer_grant(&rpc, &id)?;
        let canonical =
            x_websearch::payment::unhex(text(entry, "/canonical")?).ok_or("canonical")?;
        if canonical.iter().all(|byte| *byte == 0) {
            assert_eq!(answer, None);
            continue;
        }
        let Payment::IssueGrant(expected) = wire(Payment::decode(
            layerx_types::payload::ModuleId::Asset,
            7,
            &canonical,
            b"",
        ))?
        else {
            return Err(fail("expected grant"));
        };
        assert_eq!(answer, Some(expected));
    }
    assert_eq!(gateway.calls("eth_call"), grants.len());
    assert_eq!(gateway.calls("eth_chainId"), grants.len());
    assert_eq!(gateway.calls("eth_sendRawTransaction"), 0);

    let good = x_websearch::payment::unhex(
        text(&grants[0], "/result")?
            .strip_prefix("0x")
            .ok_or("hex")?,
    )
    .ok_or("answer")?;
    let mut malformed = vec![
        good[..good.len() - 1].to_vec(),
        good[..224].to_vec(),
        Vec::new(),
    ];
    for (offset, value) in [
        (31, 0xc0),
        (32, 1),
        (96, 1),
        (191, 2),
        (223, 2),
        (224, 1),
        (good.len() - 1, 1),
        (256 + 345, 0),
    ] {
        let mut altered = good.clone();
        altered[offset] = value;
        malformed.push(altered);
    }
    for answer in malformed {
        assert_eq!(decode_grant(&answer, ""), None);
    }
    let wrong_id = [0x77; 32];
    let mismatched = Gateway::serve_rules(&[
        json!({"method": "eth_chainId", "params": [], "result": "0x539"}),
        json!({"method": "eth_call", "params": [{
            "to": "0x000000000000000000000000000000000000101a",
            "data": format!("0x423fe4b2{}", hex(&wrong_id)),
        }, "pending"], "result": grants[0]["result"]}),
    ])?;
    let rpc = SettlementRpc::new(&format!("http://{}", mismatched.address), 1337)?;
    assert_eq!(
        read_payer_grant(&rpc, &wrong_id),
        Err(SettlementError::Malformed)
    );
    Ok(())
}

#[test]
fn a_grant_the_ledger_does_not_hold_is_refused_before_any_transaction() -> Outcome {
    let harness = Harness::start("ledger-absent", &["metered.json"])?;
    for (label, grant_id) in [
        ("absent", "00".repeat(32)),
        ("empty", "ff".repeat(32)),
        ("reverted", "ee".repeat(32)),
    ] {
        let reply = harness.metered(&format!("/search?q={label}"), "SID", &grant_id, label)?;
        assert_refused(&reply, "invalid_grant");
        assert_eq!(reply.decoded(PAYMENT_RESPONSE)?["success"], json!(false));
    }
    assert_eq!(harness.gateway.calls("eth_call"), 3);
    for (name, reason) in [
        ("recurring", "grant_not_metered"),
        ("expired", "grant_expired"),
        ("otherPurpose", "grant_purpose_mismatch"),
        ("lowLimit", "grant_limit"),
        ("otherRecipient", "grant_recipient_mismatch"),
    ] {
        let grant = harness.grant(&format!("/refusedGrants/{name}"))?;
        assert_refused(
            &harness.metered(&format!("/search?q={name}"), "SID", &grant, name)?,
            reason,
        );
    }
    assert_eq!(harness.gateway.calls("eth_call"), 8);
    let canonical = harness.grant("/grants/SID")?;
    for payload in [
        json!({"grant": canonical, "idempotencyKey": receive_key("legacy")}),
        json!({"grantId": canonical, "idempotencyKey": receive_key("oversize")}),
        json!({"grantId": "00", "idempotencyKey": receive_key("short")}),
    ] {
        let target = "/search?q=invalid-grant-id";
        let payer = harness.payer()?;
        let signature = harness.signature(target, Some(payer), METERED, "SID", &payload)?;
        assert_refused(
            &harness.pay(target, Some(payer), &signature)?,
            "invalid_payment_payload",
        );
    }
    assert_eq!(harness.gateway.calls("eth_call"), 8);
    assert_eq!(harness.gateway.calls("eth_sendRawTransaction"), 0);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 0);
    assert_eq!(harness.gateway.calls("lx_getSequence"), 0);
    assert_eq!(harness.served(), 0);
    for directory in ["requests", "receive-keys", "receipts", "deliveries"] {
        assert_eq!(
            std::fs::read_dir(harness.scratch.join("data/payments").join(directory))?.count(),
            0
        );
    }
    Ok(())
}

/// The gateway recording, rebuilt from the real draw signer, grant codec and
/// receipt encoding under the deterministic test keys.
#[allow(clippy::too_many_lines)]
fn gateway_recording() -> Outcome<Vec<(&'static str, Value)>> {
    let receiver = receiver_secret()?;
    let receiver_did = receiver_did(&x_websearch::attest::signer_address(&receiver));
    let purpose = purpose_hash(&x_websearch::attest::signer_address(&receiver));
    let sequencer = test_key("sequencer");
    let untrusted = test_key("untrusted-sequencer");
    let payer = test_key("payer");
    let payer_did = format!("did:layerx:{}", hex(&payer.verifying_key().to_bytes()));
    let account = |did: &str, asset: &AcceptedAsset| {
        account_id(&wallet_account(did, asset)).ok_or_else(|| fail("account"))
    };
    let other_did = format!("did:layerx:{}", "22".repeat(32));
    let draw = |grant: &Grant, asset: &AcceptedAsset, nonce: u64, label: &str| {
        test_draw(
            &receiver,
            grant,
            asset.price,
            nonce,
            receive_key_bytes(label),
        )
    };
    let terms = |asset: &AcceptedAsset| -> Outcome<GrantTerms> {
        Ok(GrantTerms {
            from: account(&payer_did, asset)?,
            recipient: account(&receiver_did, asset)?,
            asset: asset.asset_id,
            per_draw_maximum: asset.price * 10,
            allowance: asset.price * 10_000,
            window_length: 0,
            expiration: GRANT_EXPIRATION,
            purpose,
        })
    };

    let mut grants = serde_json::Map::new();
    let mut exact = serde_json::Map::new();
    let mut metered_identity = Vec::new();
    let mut metered_accounts = Vec::new();
    let mut metered_sends = Vec::new();
    let mut exact_rules = Vec::new();
    for (position, currency) in (0_u64..).zip(CURRENCIES) {
        let asset = accepted(currency)?;
        let (grant, encoded) = signed_grant(&payer, &terms(&asset)?, &receiver_did)?;
        grants.insert(currency.to_owned(), json!(encoded));
        let label = format!("metered-{currency}");
        let signed = draw(&grant, &asset, IDENTITY_SEQUENCE + position, &label)?;
        let pay_to = account(&receiver_did, &asset)?;
        let receipt = signed_receipt(
            &Movement {
                label: &label,
                operation: 6,
                activity_id: signed.hash,
                asset: &asset,
                amount: asset.price,
                from: account(&payer_did, &asset)?,
                to: pay_to,
                fee: RECORDED_FEE,
            },
            &sequencer,
        )?;
        let mut result = completed(&signed.hash, &receipt);
        result["commitment"] = json!("executed");
        metered_identity.push(identity_rule(
            &receiver_did,
            IDENTITY_SEQUENCE + position,
            true,
        ));
        metered_accounts.push(account_rule(&pay_to));
        metered_sends.push(send_rule(&signed.raw, ("result", result)));

        let label = format!("exact-{currency}");
        let activity_id = labelled("x-websearch-test/activity/", &label);
        let receipt = signed_receipt(
            &Movement {
                label: &label,
                operation: 5,
                activity_id,
                asset: &asset,
                amount: asset.price,
                from: account(&payer_did, &asset)?,
                to: pay_to,
                fee: RECORDED_FEE,
            },
            &sequencer,
        )?;
        exact.insert(currency.to_owned(), exact_payment(&receipt)?);
        exact_rules.push(json!({
            "method": "lx_getReceipt",
            "params": [hex(&activity_id)],
            "result": completed(&activity_id, &receipt),
        }));
    }
    let mut metered_rules = Vec::new();
    for (identity, account) in metered_identity.into_iter().zip(metered_accounts) {
        metered_rules.push(identity);
        metered_rules.push(account);
    }
    metered_rules.extend(metered_sends);

    let sid = accepted("SID")?;
    let sid_receiver = account(&receiver_did, &sid)?;
    let sid_payer = account(&payer_did, &sid)?;
    let sid_exact =
        |label: &str, to: [u8; 32], signer: &SigningKey| -> Outcome<(Value, [u8; 32])> {
            let activity_id = labelled("x-websearch-test/activity/", label);
            let receipt = signed_receipt(
                &Movement {
                    label,
                    operation: 5,
                    activity_id,
                    asset: &sid,
                    amount: sid.price,
                    from: sid_payer,
                    to,
                    fee: RECORDED_FEE,
                },
                signer,
            )?;
            Ok((exact_payment(&receipt)?, activity_id))
        };
    let other_payee = account(&other_did, &sid)?;
    let (other_payment, _) = sid_exact("exact-other-payee", other_payee, &sequencer)?;
    let (unknown_payment, unknown_activity) = sid_exact("exact-unknown", sid_receiver, &sequencer)?;
    let (untrusted_payment, _) = sid_exact("exact-untrusted", sid_receiver, &untrusted)?;
    exact_rules.push(json!({
        "method": "lx_getReceipt",
        "params": [hex(&unknown_activity)],
        "error": { "code": -32004, "message": "unknown activity" },
    }));

    let refused = |edit: &dyn Fn(&mut GrantTerms)| -> Outcome<String> {
        let mut refused = terms(&sid)?;
        edit(&mut refused);
        Ok(signed_grant(&payer, &refused, &receiver_did)?.1)
    };
    let refused_grants = json!({
        "expired": refused(&|terms| terms.expiration = NOW_MS / 1_000 - 1)?,
        "lowLimit": refused(&|terms| {
            terms.per_draw_maximum = sid.price - 1;
            terms.allowance = (sid.price - 1) * 1_000;
        })?,
        "otherPurpose": refused(&|terms| {
            terms.purpose = labelled("x-websearch-test/purpose/", "other");
        })?,
        "otherRecipient": refused(&|terms| terms.recipient = other_payee)?,
        "recurring": refused(&|terms| terms.window_length = 3_600)?,
    });

    let (sid_grant, _) = signed_grant(&payer, &terms(&sid)?, &receiver_did)?;
    let sid_movement = |label: &'static str, activity_id: [u8; 32], amount: u128| Movement {
        label,
        operation: 6,
        activity_id,
        asset: &sid,
        amount,
        from: sid_payer,
        to: sid_receiver,
        fee: RECORDED_FEE,
    };
    let pending = draw(&sid_grant, &sid, IDENTITY_SEQUENCE, "pending-SID")?;
    let pending_receipt = signed_receipt(
        &sid_movement("pending-SID", pending.hash, sid.price),
        &sequencer,
    )?;
    let pending_id = hex(&pending.hash);
    let pending_rules = vec![
        identity_rule(&receiver_did, IDENTITY_SEQUENCE, false),
        account_rule(&sid_receiver),
        send_rule(
            &pending.raw,
            (
                "error",
                json!({
                    "code": -32001,
                    "data": { "activity_id": pending_id, "state": "pending" },
                    "message": "activity pending",
                }),
            ),
        ),
        json!({
            "method": "lx_getActivityStatus",
            "once": true,
            "params": [pending_id],
            "result": { "activity_id": pending_id, "state": "pending" },
        }),
        json!({
            "method": "lx_getActivityStatus",
            "params": [pending_id],
            "result": { "activity_id": pending_id, "state": "completed" },
        }),
        json!({
            "method": "lx_getReceipt",
            "params": [pending_id],
            "result": completed(&pending.hash, &pending_receipt),
        }),
    ];

    let refusal = |label: &str| draw(&sid_grant, &sid, IDENTITY_SEQUENCE, label);
    let other_receipt = refusal("refused-other-receipt")?;
    let untrusted_draw = refusal("refused-untrusted")?;
    let failed = refusal("refused-failed")?;
    let other_activity = refusal("refused-other-activity")?;
    let amount = refusal("refused-amount")?;
    let elsewhere = labelled("x-websearch-test/activity/", "elsewhere");
    let refusal_rules = vec![
        identity_rule(&receiver_did, IDENTITY_SEQUENCE, false),
        account_rule(&sid_receiver),
        send_rule(
            &other_receipt.raw,
            (
                "result",
                completed(
                    &other_receipt.hash,
                    &signed_receipt(
                        &sid_movement("refused-other-receipt", other_activity.hash, sid.price),
                        &sequencer,
                    )?,
                ),
            ),
        ),
        send_rule(
            &untrusted_draw.raw,
            (
                "result",
                completed(
                    &untrusted_draw.hash,
                    &signed_receipt(
                        &sid_movement("refused-untrusted", untrusted_draw.hash, sid.price),
                        &untrusted,
                    )?,
                ),
            ),
        ),
        send_rule(
            &failed.raw,
            (
                "result",
                json!({ "activity_id": hex(&failed.hash), "state": "failed" }),
            ),
        ),
        send_rule(
            &other_activity.raw,
            (
                "result",
                completed(
                    &elsewhere,
                    &signed_receipt(
                        &sid_movement("refused-other-activity", elsewhere, sid.price),
                        &sequencer,
                    )?,
                ),
            ),
        ),
        send_rule(
            &amount.raw,
            (
                "result",
                completed(
                    &amount.hash,
                    &signed_receipt(
                        &sid_movement("refused-amount", amount.hash, sid.price + 1),
                        &sequencer,
                    )?,
                ),
            ),
        ),
    ];

    let buyer = json!({
        "exact": exact,
        "grants": grants,
        "payerDid": payer_did,
        "refusedExact": {
            "otherPayee": other_payment,
            "unknown": unknown_payment,
            "untrusted": untrusted_payment,
        },
        "refusedGrants": refused_grants,
        "sequencerPublicKey": hex(&sequencer.verifying_key().to_bytes()),
    });
    let ledger = ledger_recording(&buyer)?;
    metered_rules.extend(ledger_rules(&ledger)?);
    let mut metered = rules(&metered_rules);
    metered["ledger"] = ledger;
    Ok(vec![
        ("buyer.json", buyer),
        ("exact.json", exact_exchange()?),
        ("metered.json", metered),
        ("pending.json", rules(&pending_rules)),
        ("refusals.json", rules(&refusal_rules)),
    ])
}

fn pretty(value: &Value) -> Outcome<String> {
    Ok(format!("{}\n", serde_json::to_string_pretty(value)?))
}

#[test]
fn the_gateway_recording_is_reproduced_by_the_real_signers() -> Outcome {
    let recording = gateway_recording()?;
    let recorded = std::env::var_os(RECORD_VARIABLE).is_some();
    for (name, value) in &recording {
        let path = fixtures().join("gateway").join(name);
        if recorded {
            std::fs::write(&path, pretty(value)?)?;
        }
        assert_eq!(std::fs::read_to_string(&path)?, pretty(value)?, "{name}");
    }
    let text =
        serde_json::to_string(&recording.iter().map(|(_, value)| value).collect::<Vec<_>>())?;
    for secret in [
        hex(&test_key("sequencer").to_bytes()),
        hex(&test_key("untrusted-sequencer").to_bytes()),
        hex(&test_key("payer").to_bytes()),
        RECEIVER_SECRET.to_owned(),
    ] {
        assert!(!text.contains(&secret));
    }
    Ok(())
}

fn exchange_entry(target: &str, headers: &[(&str, &str)], reply: &Reply) -> Outcome<Value> {
    let mut request = serde_json::Map::new();
    for (name, value) in headers {
        let value = if *name == PAYMENT_SIGNATURE {
            serde_json::from_slice(&STANDARD.decode(value)?)?
        } else {
            json!(value)
        };
        request.insert((*name).to_owned(), value);
    }
    let mut response = serde_json::Map::new();
    for name in [PAYMENT_REQUIRED, PAYMENT_RESPONSE] {
        if reply.header(name).is_some() {
            response.insert(name.to_owned(), reply.decoded(name)?);
        }
    }
    if let Some(retry) = reply.header("Retry-After") {
        response.insert("Retry-After".to_owned(), json!(retry));
    }
    let mut answer = json!({ "status": reply.status, "headers": response });
    if reply.body.is_null() {
        answer["bodyBase64"] = json!(STANDARD.encode(&reply.raw));
    } else {
        answer["body"] = reply.body.clone();
    }
    Ok(json!({
        "request": { "method": "GET", "target": target, "headers": request },
        "response": answer,
    }))
}

/// The page each asset's exact fetch is recorded for.
const FETCHED: [(&str, usize); 4] = [("SID", 0), ("PAX", 1), ("USDC", 2), ("USDL", 3)];

#[test]
fn the_client_exchange_matches_the_recording() -> Outcome {
    let harness = Harness::start("client", &["metered.json", "exact.json"])?;
    let pages = vectors()?;
    let payer = harness.payer()?.to_owned();
    let grant = harness.grant("/grants/SID")?;
    let search = "/search?q=paxeer".to_owned();
    let metered = harness.signature(
        &search,
        Some(&payer),
        METERED,
        "SID",
        &metered_payload(&grant, "metered-SID"),
    )?;
    let mut steps: Vec<(String, Vec<(&str, String)>)> = vec![
        (search.clone(), vec![(PAYER_DID, payer.clone())]),
        (
            search,
            vec![(PAYER_DID, payer.clone()), (PAYMENT_SIGNATURE, metered)],
        ),
    ];
    for (currency, page) in FETCHED {
        let target = format!("/fetch?url={}", percent(&pages[page].payload));
        let payment = harness
            .buyer
            .pointer(&format!("/exact/{currency}"))
            .cloned()
            .ok_or_else(|| fail("payment"))?;
        let signature = harness.signature(&target, None, EXACT, currency, &payment)?;
        steps.push((target.clone(), vec![]));
        steps.push((target, vec![(PAYMENT_SIGNATURE, signature)]));
    }
    steps.push((format!("/content/{}", pages[0].digest), vec![]));
    let mut exchange = Vec::new();
    for (target, headers) in &steps {
        let headers: Vec<(&str, &str)> = headers
            .iter()
            .map(|(name, value)| (*name, value.as_str()))
            .collect();
        let reply = harness.get(target, &headers)?;
        exchange.push(exchange_entry(target, &headers, &reply)?);
    }
    assert_eq!(harness.served(), 5);

    let statuses: Vec<u64> = exchange
        .iter()
        .filter_map(|entry| entry.pointer("/response/status").and_then(Value::as_u64))
        .collect();
    assert_eq!(
        statuses,
        [402, 200, 402, 200, 402, 200, 402, 200, 402, 200, 200]
    );
    let settled = &exchange[1]["response"]["headers"][PAYMENT_RESPONSE];
    assert_eq!(
        settled.pointer("/extensions/layerx/purposeHash"),
        Some(&json!(hex(&purpose_hash(&receiver_public_key()?))))
    );
    for (step, (currency, page)) in FETCHED.iter().enumerate() {
        let paid = &exchange[3 + 2 * step]["response"];
        let asset = accepted(currency)?;
        assert_eq!(paid["headers"][PAYMENT_RESPONSE]["success"], true);
        assert_eq!(
            paid["headers"][PAYMENT_RESPONSE]["amount"],
            asset.price.to_string()
        );
        assert_eq!(paid["body"]["url"], pages[*page].payload);
        assert_eq!(paid["body"]["digest"], pages[*page].digest);
    }
    let stored = STANDARD.decode(text(&exchange[10], "/response/bodyBase64")?)?;
    assert_eq!(digest_hex(&content_digest(&stored)), pages[0].digest);
    let pax = accepted("PAX")?;
    assert_eq!(pax.symbol, AssetSymbol::Pax);
    let recording_text = serde_json::to_string(&exchange)?;
    assert!(!recording_text.contains(&harness.address()?.to_string()));
    assert!(!recording_text.contains(&harness.gateway.address.to_string()));

    let recorded = json!({ "exchange": exchange });
    let path = fixtures().join("client-exchange.json");
    if std::env::var_os(RECORD_VARIABLE).is_some() {
        std::fs::write(&path, pretty(&recorded)?)?;
    }
    assert_eq!(read_json(&path)?, recorded);
    Ok(())
}

fn receiver_did(key: &[u8]) -> String {
    format!("did:layerx:{}", hex(key))
}

fn test_draw(
    receiver: &k256::ecdsa::SigningKey,
    grant: &Grant,
    amount: u128,
    nonce: u64,
    key: [u8; 32],
) -> Outcome<x_websearch::submit::SignedTransaction> {
    Ok(LedgerTransaction {
        chain_id: 1337,
        nonce,
        gas_price: 1,
        gas_limit: 210_000,
        data: draw_calldata(
            &grant.id,
            &grant.recipient,
            amount,
            &key,
            NOW_MS + x_websearch::payment::DRAW_VALIDITY_MS,
            &grant.purpose_hash,
            &[0; 32],
        ),
    }
    .sign(receiver)?)
}

fn ledger_harness(name: &str) -> Outcome<(Harness, Grant, String)> {
    ledger_harness_recording(name, "metered.json")
}

fn ledger_harness_recording(name: &str, file: &str) -> Outcome<(Harness, Grant, String)> {
    ledger_harness_with_keys(name, file, None)
}

fn ledger_harness_with_keys(
    name: &str,
    file: &str,
    keys: Option<Vec<SequencerAuthorization>>,
) -> Outcome<(Harness, Grant, String)> {
    let recording = read_json(&fixtures().join("gateway").join(file))?;
    let ledger = &recording["ledger"];
    let buyer = read_json(&fixtures().join("gateway/buyer.json"))?;
    let asset = accepted("SID")?;
    let receiver = receiver_secret()?;
    let address = x_websearch::attest::signer_address(&receiver);
    assert_eq!(
        format!("0x{}", hex(&address)),
        text(ledger, "/receiverAddress")?
    );
    let purpose = purpose_hash(&address);
    assert_eq!(hex(&purpose), text(ledger, "/receiverPurpose")?);
    let (grant, encoded) = signed_grant(
        &test_key("payer"),
        &GrantTerms {
            from: account_id(&wallet_account(text(&buyer, "/payerDid")?, &asset)).ok_or("payer")?,
            recipient: fixture_word(ledger, "/receiverAccounts/SID")?,
            asset: asset.asset_id,
            per_draw_maximum: asset.price * 10,
            allowance: asset.price * 10_000,
            window_length: 0,
            expiration: GRANT_EXPIRATION,
            purpose,
        },
        "",
    )?;
    let mut list: Vec<Value> = recording["endpoints"]["gateway"]["*"]
        .as_array()
        .ok_or("rules")?
        .iter()
        .filter(|rule| {
            rule["method"] == "eth_chainId"
                || rule["method"] == "pax_sequencerReceipt"
                || rule["params"][0]["data"]
                    .as_str()
                    .is_some_and(|data| data.starts_with("0x3f050cbc"))
        })
        .cloned()
        .collect();
    list.push(json!({"method":"eth_call", "params":[{
        "to":format!("0x{}", hex(&LEDGER_PRECOMPILE)),
        "data":format!("0x423fe4b2{}", hex(&grant.id))}, "pending"], "result":ledger_answer(&encoded)}));
    for nonce in [7, 8] {
        list.push(json!({"method":"eth_getTransactionCount", "params":[format!("0x{}", hex(&address)), "pending"], "once":true, "result":format!("0x{nonce:x}")}));
    }
    list.push(json!({"method":"eth_gasPrice", "params":[], "result":"0x1"}));
    list.push(json!({"method":"eth_estimateGas", "params":"$any", "result":"0x33450"}));
    let harness =
        Harness::launch_with_receipt_keys(name, Gateway::serve_rules(&list)?, None, keys)?;
    assert_eq!(harness.gate().receiver_address(), address);
    assert_eq!(harness.gate().purpose(), purpose);
    assert_eq!(harness.gate().payee(&asset), grant.recipient);
    Ok((harness, grant, encoded))
}

fn broadcast_draw(
    harness: &Harness,
    grant: &Grant,
    encoded: &str,
    label: &str,
    nonce: u64,
) -> Outcome<x_websearch::submit::SignedTransaction> {
    let target = format!("/search?q={label}");
    let signature = harness.signature(
        &target,
        Some(harness.payer()?),
        METERED,
        "SID",
        &metered_payload(encoded, label),
    )?;
    let payload: layerx_x402::model::PaymentPayload =
        serde_json::from_slice(&STANDARD.decode(&signature)?)?;
    let mut digest = Sha256::new();
    digest.update(b"LayerX/x402/v2/payment\0");
    digest.update(serde_json::to_vec(&payload)?);
    let digest: [u8; 32] = digest.finalize().into();
    let signed = LedgerTransaction {
        chain_id: 1337,
        nonce,
        gas_price: 1,
        gas_limit: 210_000,
        data: draw_calldata(
            &grant.id,
            &grant.recipient,
            accepted("SID")?.price,
            &receive_key_bytes(label),
            NOW_MS + x_websearch::payment::DRAW_VALIDITY_MS,
            &grant.purpose_hash,
            &digest,
        ),
    }
    .sign(&receiver_secret()?)?;
    harness.gateway.rules.lock().unwrap_or_else(PoisonError::into_inner).extend(parse_rules(&[
        json!({"method":"eth_sendRawTransaction", "params":[format!("0x{}", hex(&signed.raw))], "once":true, "result":format!("0x{}", hex(&signed.hash))}),
    ])?);
    let reply = harness.pay(&target, Some(harness.payer()?), &signature)?;
    assert_eq!(reply.status, 503, "{:?}", reply.body);
    assert_eq!(reply.error(), Some("payment_pending"));
    assert_eq!(harness.served(), 0);
    Ok(signed)
}

fn rlp_item(bytes: &[u8]) -> Outcome<(bool, &[u8], usize)> {
    let first = *bytes.first().ok_or("empty RLP")?;
    let (list, offset, length) = match first {
        0..=127 => (false, 0, 1),
        128..=183 => (false, 1, usize::from(first - 128)),
        184..=191 | 248..=255 => {
            let list = first >= 248;
            let width = usize::from(first - if list { 247 } else { 183 });
            let length = bytes
                .get(1..1 + width)
                .ok_or("RLP length")?
                .iter()
                .try_fold(0_usize, |n, byte| {
                    n.checked_mul(256)?.checked_add(usize::from(*byte))
                })
                .ok_or("RLP overflow")?;
            (list, 1 + width, length)
        }
        192..=247 => (true, 1, usize::from(first - 192)),
    };
    Ok((
        list,
        bytes.get(offset..offset + length).ok_or("truncated RLP")?,
        offset + length,
    ))
}

fn assert_ledger_transaction(
    raw: &[u8],
    expected_nonce: u64,
    grant: &Grant,
    label: &str,
) -> Outcome {
    let (list, body, consumed) = rlp_item(raw)?;
    assert!(list);
    assert_eq!(consumed, raw.len());
    let mut remaining = body;
    let mut fields = Vec::new();
    let mut signing_fields = Vec::new();
    while !remaining.is_empty() {
        let (list, field, length) = rlp_item(remaining)?;
        assert!(!list);
        if fields.len() < 6 {
            signing_fields.extend_from_slice(&remaining[..length]);
        }
        fields.push(field);
        remaining = &remaining[length..];
    }
    assert_eq!(fields.len(), 9);
    let integer = |field: &[u8]| {
        field
            .iter()
            .fold(0_u64, |value, byte| value * 256 + u64::from(*byte))
    };
    assert_eq!(integer(fields[0]), expected_nonce);
    assert_eq!(integer(fields[1]), 1);
    assert_eq!(integer(fields[2]), 210_000);
    assert_eq!(fields[3], LEDGER_PRECOMPILE);
    assert!(fields[4].is_empty());
    let data = fields[5];
    assert_eq!(data.len(), 228);
    assert_eq!(&data[..4], &[0x2e, 0x89, 0x49, 0x0f]);
    assert_eq!(&data[4..36], &grant.id);
    assert_eq!(&data[36..68], &grant.recipient);
    assert_eq!(&data[68..84], &[0; 16]);
    assert_eq!(&data[84..100], &accepted("SID")?.price.to_be_bytes());
    assert_eq!(&data[100..132], &receive_key_bytes(label));
    assert_eq!(&data[132..156], &[0; 24]);
    assert_eq!(
        &data[156..164],
        &(NOW_MS + x_websearch::payment::DRAW_VALIDITY_MS).to_be_bytes()
    );
    assert_eq!(&data[164..196], &grant.purpose_hash);
    let v = integer(fields[6]);
    assert_eq!((v - 35) / 2, 1337);
    signing_fields.extend_from_slice(&[0x82, 0x05, 0x39, 0x80, 0x80]);
    let length = u16::try_from(signing_fields.len())?;
    assert!(length >= 256);
    let unsigned = [&[0xf9][..], &length.to_be_bytes(), &signing_fields].concat();
    let mut r = [0; 32];
    let mut s = [0; 32];
    r[32 - fields[7].len()..].copy_from_slice(fields[7]);
    s[32 - fields[8].len()..].copy_from_slice(fields[8]);
    let signature = k256::ecdsa::Signature::from_scalars(r, s)?;
    assert!(signature.normalize_s().is_none());
    let recovery = k256::ecdsa::RecoveryId::try_from(((v - 35) % 2) as u8)?;
    let recovered = k256::ecdsa::VerifyingKey::recover_from_prehash(
        &x_websearch::watch::keccak(&unsigned),
        &signature,
        recovery,
    )?;
    assert_eq!(recovered, *receiver_secret()?.verifying_key());
    Ok(())
}

#[test]
fn a_metered_draw_is_broadcast_as_a_receiver_signed_ledger_transaction() -> Outcome {
    let (harness, grant, encoded) = ledger_harness("receiver-signed-ledger")?;
    let signed = broadcast_draw(&harness, &grant, &encoded, "receiver-signed", 7)?;
    let requests = harness
        .gateway
        .requests
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    let broadcast = requests
        .iter()
        .find(|request| request["method"] == "eth_sendRawTransaction")
        .ok_or("missing broadcast")?;
    let raw = broadcast["params"][0]
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .and_then(x_websearch::payment::unhex)
        .ok_or("raw transaction")?;
    assert_eq!(raw, signed.raw);
    assert_eq!(x_websearch::watch::keccak(&raw), signed.hash);
    assert_ledger_transaction(&raw, 7, &grant, "receiver-signed")?;
    let snapshots = harness
        .gateway
        .broadcasts
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    assert_eq!(snapshots.len(), 1);
    assert_eq!(snapshots[0].len(), 1);
    let record = &snapshots[0][0];
    assert_eq!(record.raw_transaction.as_deref(), Some(hex(&raw).as_str()));
    assert_eq!(record.tx_hash.as_deref(), Some(hex(&signed.hash).as_str()));
    assert_eq!(
        record.receive_key.as_deref(),
        Some(hex(&receive_key_bytes("receiver-signed")).as_str())
    );
    assert!(record.attempted);
    assert!(!record.released);
    assert!(record.receipt.is_none());
    assert_eq!(&raw[0..1], &[0xf9]);
    let account_calls: Vec<_> = requests
        .iter()
        .filter(|request| {
            request["params"][0]["data"]
                .as_str()
                .is_some_and(|data| data.starts_with("0x3f050cbc"))
        })
        .collect();
    assert_eq!(account_calls.len(), 4);
    let mut owner = [0; 32];
    owner[12..].copy_from_slice(&harness.gate().receiver_address());
    for (call, asset) in account_calls.iter().zip(harness.gate().assets().all()) {
        assert_eq!(
            call["params"],
            json!([{"to":format!("0x{}", hex(&LEDGER_PRECOMPILE)), "data":format!("0x3f050cbc{}{}", hex(&owner), hex(&asset.asset_id))}, "pending"])
        );
    }
    assert_eq!(harness.gateway.calls("eth_sendRawTransaction"), 1);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 0);
    assert_eq!(harness.gateway.calls("lx_getSequence"), 0);
    Ok(())
}

#[test]
fn draw_nonces_are_sequential_for_one_receiver() -> Outcome {
    let (harness, grant, encoded) = ledger_harness("sequential-ledger-nonces")?;
    for (nonce, label) in [(7, "nonce-seven"), (8, "nonce-eight")] {
        let signed = broadcast_draw(&harness, &grant, &encoded, label, nonce)?;
        assert_ledger_transaction(&signed.raw, nonce, &grant, label)?;
    }
    assert_eq!(harness.gateway.calls("eth_getTransactionCount"), 2);
    assert_eq!(harness.gateway.calls("eth_sendRawTransaction"), 2);
    assert_eq!(harness.gateway.calls("lx_getSequence"), 0);
    assert_eq!(harness.gateway.calls("lx_sendActivity"), 0);
    let requests = harness
        .gateway
        .requests
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    let sequence: Vec<_> = requests
        .iter()
        .filter_map(|request| match request["method"].as_str() {
            Some("eth_getTransactionCount") => {
                assert_eq!(
                    request["params"],
                    json!([
                        format!("0x{}", hex(&harness.gate().receiver_address())),
                        "pending"
                    ])
                );
                Some("nonce")
            }
            Some("eth_sendRawTransaction") => Some("broadcast"),
            _ => None,
        })
        .collect();
    assert_eq!(sequence, ["nonce", "broadcast", "nonce", "broadcast"]);
    let snapshots = harness
        .gateway
        .broadcasts
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    assert_eq!(snapshots.len(), 2);
    assert_eq!(snapshots[0].len(), 1);
    assert_eq!(snapshots[1].len(), 2);
    assert_ne!(snapshots[1][0].tx_hash, snapshots[1][1].tx_hash);
    for record in &snapshots[1] {
        assert!(record.attempted);
        let raw = record
            .raw_transaction
            .as_deref()
            .and_then(x_websearch::payment::unhex)
            .ok_or("journal raw")?;
        assert_eq!(
            record.tx_hash.as_deref(),
            Some(hex(&x_websearch::watch::keccak(&raw)).as_str())
        );
    }
    Ok(())
}

#[test]
fn exact_payment_accepts_only_native_signed_topic_order() -> Outcome {
    let permutations = [
        [1, 2, 3],
        [1, 3, 2],
        [2, 1, 3],
        [2, 3, 1],
        [3, 1, 2],
        [3, 2, 1],
    ];
    for permutation in permutations {
        let label = format!("exact-topic-order-{permutation:?}");
        let harness = Harness::start(&label, &[])?;
        let asset = accepted("SID")?;
        let payer = account_id(&wallet_account(harness.payer()?, &asset)).ok_or("payer")?;
        let payee = harness.gate.as_ref().ok_or("payment gate")?.payee(&asset);
        let reference = receive_key_bytes(&label);
        let native = [
            x_websearch::watch::keccak(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"),
            reference,
            payer,
            payee,
        ];
        assert_ne!(reference, payer);
        assert_ne!(reference, payee);
        assert_ne!(payer, payee);
        let mut amount = [0; 32];
        amount[16..].copy_from_slice(&asset.price.to_be_bytes());
        let event = SequencerLog::new(
            LEDGER_PRECOMPILE,
            vec![
                native[0],
                native[permutation[0]],
                native[permutation[1]],
                native[permutation[2]],
            ],
            [asset.asset_id, amount].concat(),
            0,
        );
        let hash = labelled("x-websearch-test/transaction/", &label);
        let answer = draw_receipt_answer(hash, &[event], 1)?;
        let receipt =
            x_websearch::payment::unhex(text(&answer, "/receipt")?.trim_start_matches("0x"))
                .ok_or("receipt bytes")?;
        let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt)?;
        let payment = json!({
            "txHash": format!("0x{}", hex(&hash)),
            "receiptDigest": hex(&digest), "verificationLevel": "receipted",
            "idempotencyKey": hex(&reference),
        });
        receipt_rule(&harness, hash, answer, false)?;
        let target = "/fetch?url=topic-order";
        let signature =
            harness.signature(target, Some(harness.payer()?), EXACT, "SID", &payment)?;
        let reply = harness.pay(target, Some(harness.payer()?), &signature)?;
        if permutation == [1, 2, 3] {
            assert_settled(&reply)?;
            assert_eq!(reply.decoded(PAYMENT_RESPONSE)?["payer"], hex(&payer));
            assert_eq!(harness.served(), 1);
        } else {
            assert_refused(&reply, "receipt_mismatch");
            assert_eq!(harness.served(), 0);
        }
    }
    Ok(())
}

#[test]
fn exact_payment_native_topics_retain_receipt_validation() -> Outcome {
    for mutation in [
        "payer",
        "payee",
        "reference",
        "asset",
        "amount",
        "amount-overflow",
        "address",
        "duplicate",
        "status",
        "signature",
        "unsigned-log",
    ] {
        let label = format!("native-exact-{mutation}");
        let harness = Harness::start(&label, &[])?;
        let asset = accepted("SID")?;
        let payer = account_id(&wallet_account(harness.payer()?, &asset)).ok_or("payer")?;
        let payee = harness.gate.as_ref().ok_or("payment gate")?.payee(&asset);
        let mut topics = vec![
            x_websearch::watch::keccak(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"),
            receive_key_bytes(&label),
            payer,
            payee,
        ];
        let mut data = [0; 64];
        data[..32].copy_from_slice(&asset.asset_id);
        data[48..].copy_from_slice(&asset.price.to_be_bytes());
        let mut address = LEDGER_PRECOMPILE;
        match mutation {
            "payer" => topics[2][0] ^= 1,
            "payee" => topics[3][0] ^= 1,
            "reference" => topics[1][0] ^= 1,
            "asset" => data[0] ^= 1,
            "amount" => data[63] ^= 1,
            "amount-overflow" => data[32] = 1,
            "address" => address[0] ^= 1,
            _ => (),
        }
        let mut logs = vec![SequencerLog::new(address, topics.clone(), data.to_vec(), 0)];
        if mutation == "duplicate" {
            logs.push(SequencerLog::new(address, topics, data.to_vec(), 1));
        }
        let hash = labelled("x-websearch-test/transaction/", &label);
        let mut answer = draw_receipt_answer(hash, &logs, u64::from(mutation != "status"))?;
        let mut receipt =
            x_websearch::payment::unhex(text(&answer, "/receipt")?.trim_start_matches("0x"))
                .ok_or("receipt bytes")?;
        if mutation == "signature" {
            let last = receipt.len() - 1;
            receipt[last] ^= 1;
            answer["receipt"] = json!(format!("0x{}", hex(&receipt)));
        }
        if mutation == "unsigned-log" {
            answer["logs"][0]["topics"][2] = json!(format!("0x{}", "00".repeat(32)));
        }
        let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt)?;
        let payment = json!({
            "txHash": format!("0x{}", hex(&hash)),
            "receiptDigest": hex(&digest), "verificationLevel": "receipted",
            "idempotencyKey": receive_key(&label),
        });
        receipt_rule(&harness, hash, answer, false)?;
        let target = "/fetch?url=receipt-validation";
        let signature =
            harness.signature(target, Some(harness.payer()?), EXACT, "SID", &payment)?;
        let reply = harness.pay(target, Some(harness.payer()?), &signature)?;
        assert_refused(
            &reply,
            match mutation {
                "status" => "payment_failed",
                "signature" => "receipt_unverified",
                _ => "receipt_mismatch",
            },
        );
        assert_eq!(harness.served(), 0, "{mutation}");
    }
    Ok(())
}
