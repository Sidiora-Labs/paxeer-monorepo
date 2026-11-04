//! Audit 22.30.110. Real service handlers, TLS journal and node transport;
//! peers implement the recorded RESP/JSON-RPC boundary and persist to disk.
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_interop_service::config::{self, Config};
use layerx_interop_service::relay::{ChainRelay, RelayError, RelayOutcome};
use layerx_interop_service::server;
use layerx_platform_gateway::http::{self, OutgoingResponse};
use layerx_platform_gateway::store::KeyRecord;
use layerx_platform_gateway::{gateway_digest, IssuedKey};
use native_tls::{Identity, TlsAcceptor};
use openssl::asn1::Asn1Time;
use openssl::ec::{EcGroup, EcKey};
use openssl::hash::MessageDigest;
use openssl::nid::Nid;
use openssl::pkcs12::Pkcs12;
use openssl::pkey::PKey;
use openssl::x509::extension::{BasicConstraints, SubjectAlternativeName};
use openssl::x509::{X509NameBuilder, X509};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::collections::{BTreeMap, BTreeSet};
use std::ffi::OsString;
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

type TestResult<T = ()> = Result<T, Box<dyn std::error::Error + Send + Sync>>;

fn wire<T>(result: Result<T, layerx_wire::WireError>) -> TestResult<T> {
    result.map_err(|error| format!("wire fixture: {error:?}").into())
}
const PRIVATE_URL: &str = "http://router.railway.internal:9443";
static SERIAL: Mutex<()> = Mutex::new(());
const TIMEOUT: Duration = Duration::from_secs(5);

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn bytes(text: &str) -> TestResult<Vec<u8>> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
        .collect()
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> TestResult<Self> {
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-86-{}-{}",
            std::process::id(),
            SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos()
        ));
        fs::create_dir(&path)?;
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))?;
        Ok(Self(path))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct Listener {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl Listener {
    fn start(
        address: SocketAddr,
        handler: impl Fn(TcpStream) + Send + Sync + 'static,
    ) -> TestResult<Self> {
        let listener = TcpListener::bind(address)?;
        let address = listener.local_addr()?;
        listener.set_nonblocking(true)?;
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let handler = Arc::new(handler);
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::SeqCst) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        stream.set_read_timeout(Some(TIMEOUT)).unwrap();
                        stream.set_write_timeout(Some(TIMEOUT)).unwrap();
                        let handler = Arc::clone(&handler);
                        thread::spawn(move || handler(stream));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("fixture listener: {error}"),
                }
            }
        });
        Ok(Self {
            address,
            stop,
            worker: Some(worker),
        })
    }
}
impl Drop for Listener {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        self.worker.take().unwrap().join().unwrap();
    }
}

fn line(stream: &mut impl Read) -> TestResult<Vec<u8>> {
    let mut result = Vec::new();
    while !result.ends_with(b"\r\n") {
        assert!(result.len() < 64 * 1024);
        let mut byte = [0];
        stream.read_exact(&mut byte)?;
        result.push(byte[0]);
    }
    result.truncate(result.len() - 2);
    Ok(result)
}

fn command(stream: &mut impl Read) -> TestResult<Vec<Vec<u8>>> {
    let header = line(stream)?;
    assert_eq!(header[0], b'*');
    let count: usize = std::str::from_utf8(&header[1..])?.parse()?;
    assert!(count < 4096);
    let mut arguments = Vec::new();
    for _ in 0..count {
        let header = line(stream)?;
        assert_eq!(header[0], b'$');
        let count: usize = std::str::from_utf8(&header[1..])?.parse()?;
        assert!(count <= 2 * 1024 * 1024);
        let mut value = vec![0; count];
        stream.read_exact(&mut value)?;
        assert!(line(stream)?.is_empty());
        arguments.push(value);
    }
    Ok(arguments)
}

fn bulk(value: Option<&[u8]>) -> Vec<u8> {
    match value {
        None => b"$-1\r\n".to_vec(),
        Some(value) => [
            format!("${}\r\n", value.len()).into_bytes(),
            value.to_vec(),
            b"\r\n".to_vec(),
        ]
        .concat(),
    }
}
fn array(values: &[Vec<u8>]) -> Vec<u8> {
    let mut result = format!("*{}\r\n", values.len()).into_bytes();
    for value in values {
        result.extend(bulk(Some(value)));
    }
    result
}
fn tag(value: &str) -> Vec<u8> {
    array(&[value.as_bytes().to_vec()])
}
fn text(value: &[u8]) -> &str {
    std::str::from_utf8(value).unwrap()
}

#[derive(Default, Serialize, Deserialize)]
struct StoreState {
    strings: BTreeMap<String, Vec<u8>>,
    hashes: BTreeMap<String, BTreeMap<String, Vec<u8>>>,
    sets: BTreeMap<String, BTreeSet<Vec<u8>>>,
    expiries: BTreeMap<String, u128>,
}
impl StoreState {
    fn value(&self, key: &str) -> &[u8] {
        self.strings.get(key).map_or(b"", Vec::as_slice)
    }
    fn field(&self, key: &str, name: &str) -> Vec<u8> {
        self.hashes
            .get(key)
            .and_then(|hash| hash.get(name))
            .cloned()
            .unwrap_or_default()
    }
    fn set_fields(&mut self, key: &str, fields: &[(&str, &[u8])]) {
        let hash = self.hashes.entry(key.into()).or_default();
        for (field, value) in fields {
            hash.insert((*field).into(), value.to_vec());
        }
    }
    fn eval(&mut self, script: &str, keys: &[String], args: &[Vec<u8>]) -> Vec<u8> {
        let k = |i: usize| keys[i - 1].as_str();
        let a = |i: usize| args[i - 1].as_slice();
        if k(1) == "interop:relay:retention-cursor:v1" {
            assert!(script.contains("SCAN"));
            let keys = self
                .strings
                .keys()
                .filter(|key| {
                    key.len() == 81
                        && key.starts_with("interop:relay:v1:")
                        && !self.strings.contains_key(&format!("{key}:finalized"))
                })
                .cloned()
                .collect::<Vec<_>>();
            return bulk(Some(keys.join("\n").as_bytes()));
        }
        if k(1).starts_with("interop:relay:v1:") {
            let terminal = format!("{}:finalized", k(1));
            if script.contains("local terminal") {
                assert_eq!(keys.len(), 1);
                if let Some(hash) = self.strings.get(&terminal) {
                    assert_eq!(hash, a(1));
                    return bulk(Some(hash));
                }
                let Some(old) = self.strings.get(k(1)) else {
                    return bulk(None);
                };
                assert_eq!(&old[..32], a(1));
                self.strings.insert(terminal, a(1).to_vec());
                let deadline = unix_millis() + text(a(2)).parse::<u128>().unwrap() * 1000;
                self.expiries.insert(k(1).to_owned(), deadline);
                self.expiries
                    .insert(format!("{}:submitted", k(1)), deadline);
                return bulk(Some(a(1)));
            }
            if script.contains("string.char(1) .. ARGV[1]") {
                assert_eq!(keys.len(), 1);
                if let Some(hash) = self.strings.get(&terminal) {
                    return bulk(Some(&[vec![0], hash.clone()].concat()));
                }
                let old = self.strings.get(k(1)).cloned();
                let (status, record) = match old {
                    Some(old) => (0, old),
                    None => {
                        self.strings.insert(k(1).into(), a(1).to_vec());
                        (1, a(1).to_vec())
                    }
                };
                return bulk(Some(&[vec![status], record].concat()));
            }
            if script.contains("local progress") {
                assert_eq!(keys.len(), 2);
                let old = self.strings.get(k(1));
                if let Some(hash) = self.strings.get(&terminal) {
                    if let Some(old) = old {
                        assert_eq!(&old[..32], hash);
                    }
                    return bulk(Some(&[vec![2], old.unwrap_or(hash).clone()].concat()));
                }
                return match old {
                    None => bulk(None),
                    Some(record) => {
                        let progress = self.strings.get(k(2));
                        if let Some(progress) = progress {
                            assert_eq!(progress, &record[..32]);
                        }
                        bulk(Some(
                            &[vec![u8::from(progress.is_some())], record.clone()].concat(),
                        ))
                    }
                };
            }
            assert!(script.contains("ARGV[2]"));
            assert_eq!(keys.len(), 2);
            if self.strings.get(k(1)).map(Vec::as_slice) != Some(a(1)) {
                return bulk(None);
            }
            self.strings.insert(k(2).into(), a(2).to_vec());
            return bulk(Some(a(2)));
        }
        if script.contains("return {'issued'}") {
            assert_eq!(keys.len(), 4);
            if self.value(k(4)) != a(12) {
                return tag("audit_retry");
            }
            if self.hashes.contains_key(k(1)) {
                return tag("conflict");
            }
            if self.sets.get(k(2)).map_or(0, BTreeSet::len) >= text(a(10)).parse().unwrap() {
                return tag("limit");
            }
            self.set_fields(
                k(1),
                &[
                    ("principal", a(2)),
                    ("salt", a(3)),
                    ("secret_digest", a(4)),
                    ("signer_public_key", a(5)),
                    ("scopes", a(6)),
                    ("quota_requests", a(7)),
                    ("quota_window_seconds", a(8)),
                    ("epoch", a(9)),
                    ("disabled", b"0"),
                ],
            );
            self.sets
                .entry(k(2).into())
                .or_default()
                .insert(a(1).to_vec());
            self.strings.insert(k(4).into(), a(13).to_vec());
            return tag("issued");
        }
        if script.contains("return {'reserved'}") {
            assert_eq!(keys.len(), 8);
            if self.field(k(1), "disabled") != b"0" || self.field(k(1), "epoch") != a(1) {
                return tag("revoked");
            }
            let digest = self.field(k(3), "digest");
            if !digest.is_empty() {
                return array(&[
                    b"existing".to_vec(),
                    digest,
                    self.field(k(3), "state"),
                    self.field(k(3), "response"),
                    self.field(k(3), "receipt"),
                    self.field(k(3), "principal"),
                ]);
            }
            if self.value(k(5)) != a(8) {
                return tag("audit_retry");
            }
            if self.strings.get(k(7)).is_some_and(|owner| owner != a(10))
                || self
                    .strings
                    .get(k(8))
                    .is_some_and(|operation| operation != k(3).as_bytes())
            {
                return tag("conflict");
            }
            let used: u64 = if self.value(k(2)).is_empty() {
                0
            } else {
                text(self.value(k(2))).parse().unwrap()
            };
            if used >= text(a(2)).parse().unwrap() {
                return array(&[b"rate_limited".to_vec(), a(3).to_vec()]);
            }
            self.strings
                .insert(k(2).into(), (used + 1).to_string().into_bytes());
            self.set_fields(
                k(3),
                &[
                    ("scope", &k(3).as_bytes()[13..]),
                    ("digest", a(5)),
                    ("state", b"pending"),
                    ("started_at", a(7)),
                    ("principal", a(10)),
                    ("activity_id", &k(7).as_bytes()[17..]),
                    ("idempotency_key", a(11)),
                    (
                        "continuation_count",
                        (args.len() - 11).to_string().as_bytes(),
                    ),
                ],
            );
            for (i, value) in args[11..].iter().enumerate() {
                self.set_fields(k(3), &[(&format!("continuation_{i}"), value)]);
            }
            self.sets
                .entry(k(6).into())
                .or_default()
                .insert(k(3).as_bytes().to_vec());
            self.strings.insert(k(7).into(), a(10).to_vec());
            self.strings.insert(k(8).into(), k(3).as_bytes().to_vec());
            self.strings.insert(k(5).into(), a(9).to_vec());
            let deadline = unix_millis() + text(a(4)).parse::<u128>().unwrap() * 1000;
            for key in [k(3), k(7), k(8)] {
                self.expiries.insert(key.to_owned(), deadline);
            }
            return tag("reserved");
        }
        assert!(
            script.contains("return {'completed'}"),
            "unrecorded fixture script: {script}"
        );
        assert_eq!(keys.len(), 9);
        if self.field(k(1), "digest") != a(1) {
            return tag("conflict");
        }
        if self.field(k(1), "state") != b"pending" {
            return tag("completed");
        }
        assert!(
            a(10).is_empty(),
            "this route uses complete, not the observed-event interface"
        );
        if self.value(k(4)) != a(6) {
            return tag("audit_retry");
        }
        if a(8) == b"1" {
            if self.strings.get(k(5)).is_some_and(|owner| owner != a(9)) {
                return tag("conflict");
            }
            self.strings.insert(k(5).into(), a(9).to_vec());
        }
        self.set_fields(
            k(1),
            &[("state", a(2)), ("response", a(3)), ("receipt", a(4))],
        );
        if a(2) != b"pending" {
            self.sets
                .entry(k(2).into())
                .or_default()
                .remove(k(1).as_bytes());
        }
        self.strings.insert(k(4).into(), a(7).to_vec());
        tag("completed")
    }
    fn exchange(&mut self, arguments: &[Vec<u8>]) -> Vec<u8> {
        let expired: Vec<_> = self
            .expiries
            .iter()
            .filter(|(_, deadline)| **deadline <= unix_millis())
            .map(|(key, _)| key.clone())
            .collect();
        for key in expired {
            self.strings.remove(&key);
            self.hashes.remove(&key);
            self.expiries.remove(&key);
        }
        match text(&arguments[0]) {
            "PING" => b"+PONG\r\n".to_vec(),
            "GET" => bulk(self.strings.get(text(&arguments[1])).map(Vec::as_slice)),
            "HGET" => bulk(
                self.hashes
                    .get(text(&arguments[1]))
                    .and_then(|h| h.get(text(&arguments[2])))
                    .map(Vec::as_slice),
            ),
            "HGETALL" => {
                let values = self
                    .hashes
                    .get(text(&arguments[1]))
                    .into_iter()
                    .flat_map(|hash| hash.iter())
                    .flat_map(|(key, value)| [key.as_bytes().to_vec(), value.clone()])
                    .collect::<Vec<_>>();
                array(&values)
            }
            "EVAL" => {
                let count: usize = text(&arguments[2]).parse().unwrap();
                let keys = arguments[3..3 + count]
                    .iter()
                    .map(|key| text(key).to_owned())
                    .collect::<Vec<_>>();
                self.eval(text(&arguments[1]), &keys, &arguments[3 + count..])
            }
            other => panic!("unrecorded Redis fixture command: {other}"),
        }
    }
}

struct Redis {
    listener: Listener,
    state: Arc<Mutex<StoreState>>,
    lose_claim_reply: Arc<AtomicBool>,
}
impl Redis {
    fn start(dir: &Path, identity: &Identity, address: SocketAddr) -> TestResult<Self> {
        let snapshot = dir.join("redis.json");
        let state: StoreState = if snapshot.exists() {
            serde_json::from_slice(&fs::read(&snapshot)?)?
        } else {
            StoreState::default()
        };
        let state = Arc::new(Mutex::new(state));
        let shared = Arc::clone(&state);
        let lose_claim_reply = Arc::new(AtomicBool::new(false));
        let lose = Arc::clone(&lose_claim_reply);
        let acceptor = TlsAcceptor::new(identity.clone())?;
        let listener = Listener::start(address, move |stream| {
            let Ok(mut stream) = acceptor.accept(stream) else {
                return;
            };
            let auth = command(&mut stream).unwrap();
            assert_eq!(
                auth,
                [
                    b"AUTH".to_vec(),
                    b"relay".to_vec(),
                    b"repair-secret".to_vec()
                ]
            );
            stream.write_all(b"+OK\r\n").unwrap();
            while let Ok(arguments) = command(&mut stream) {
                let reply = {
                    let mut state = shared.lock().unwrap();
                    let reply = state.exchange(&arguments);
                    let mut file = fs::File::create(snapshot.with_extension("tmp")).unwrap();
                    file.write_all(&serde_json::to_vec(&*state).unwrap())
                        .unwrap();
                    file.sync_all().unwrap();
                    fs::rename(snapshot.with_extension("tmp"), &snapshot).unwrap();
                    reply
                };
                if arguments[0] == b"EVAL"
                    && text(&arguments[1]).contains("string.char(1) .. ARGV[1]")
                    && lose.swap(false, Ordering::SeqCst)
                {
                    return;
                }
                if stream
                    .write_all(&reply)
                    .and_then(|()| stream.flush())
                    .is_err()
                {
                    return;
                }
            }
        })?;
        Ok(Self {
            listener,
            state,
            lose_claim_reply,
        })
    }
    fn record(&self, key: &str) -> Option<Vec<u8>> {
        self.state.lock().unwrap().strings.get(key).cloned()
    }
}

#[derive(Default)]
struct NodeState {
    attempts: Vec<Vec<u8>>,
    accepted: BTreeSet<Vec<u8>>,
    methods: Vec<String>,
    connects: Vec<String>,
}
struct Node {
    listener: Listener,
    state: Arc<Mutex<NodeState>>,
    fault: Arc<AtomicU8>,
    hide_receipt: Arc<AtomicBool>,
}
impl Node {
    fn start(case: &Value) -> TestResult<Self> {
        let state = Arc::new(Mutex::new(NodeState::default()));
        let shared = Arc::clone(&state);
        let fault = Arc::new(AtomicU8::new(0));
        let faulted = Arc::clone(&fault);
        let hide_receipt = Arc::new(AtomicBool::new(false));
        let hidden = Arc::clone(&hide_receipt);
        let case = case.clone();
        let raw = bytes(case["rawTransaction"].as_str().ok_or("raw transaction")?)?;
        let listener = Listener::start("127.0.0.1:0".parse()?, move |mut stream| {
            let connect = line(&mut stream).unwrap();
            let connect = text(&connect).to_owned();
            assert_eq!(connect, "CONNECT router.railway.internal:9443 HTTP/1.1");
            shared.lock().unwrap().connects.push(connect);
            while !line(&mut stream).unwrap().is_empty() {}
            stream
                .write_all(b"HTTP/1.1 200 Connection established\r\n\r\n")
                .unwrap();
            let request = http::read_request(&mut stream, 2 * 1024 * 1024).unwrap();
            assert_eq!(request.method, "POST");
            assert_eq!(
                request.headers.get("host").map(String::as_str),
                Some("router.railway.internal:9443")
            );
            let rpc: Value = serde_json::from_slice(&request.body).unwrap();
            let method = rpc["method"].as_str().unwrap();
            let result = {
                let mut state = shared.lock().unwrap();
                state.methods.push(method.into());
                match method {
                    "px_getReceiptKeys" => {
                        assert_eq!(rpc["params"], json!([]));
                        if faulted.load(Ordering::SeqCst) == 3 {
                            json!({"keys": []})
                        } else {
                            json!({"keys": [case["pinnedKey"]]})
                        }
                    }
                    "eth_sendRawTransaction" => {
                        let sent = bytes(rpc["params"][0].as_str().unwrap()).unwrap();
                        assert_eq!(
                            sent, raw,
                            "only the payer's exact recorded signed bytes may be sent"
                        );
                        assert_eq!(
                            format!("0x{}", hex(&transaction_hash(&sent))),
                            case["receiptJson"]["txHash"]
                        );
                        state.attempts.push(sent.clone());
                        let mode = faulted.load(Ordering::SeqCst);
                        if matches!(mode, 1 | 2) {
                            faulted.store(0, Ordering::SeqCst);
                        }
                        if mode == 1 {
                            return;
                        }
                        state.accepted.insert(sent);
                        if mode == 2 {
                            return;
                        }
                        case["receiptJson"]["txHash"].clone()
                    }
                    "pax_sequencerReceipt" => {
                        if rpc["params"] != json!([case["receiptJson"]["txHash"]])
                            || hidden.swap(false, Ordering::SeqCst)
                            || state.accepted.is_empty()
                        {
                            Value::Null
                        } else if faulted.load(Ordering::SeqCst) == 4 {
                            let mut receipt = case["receiptJson"].clone();
                            receipt["rung"] = json!("receipted");
                            receipt.as_object_mut().unwrap().remove("blockNumber");
                            receipt.as_object_mut().unwrap().remove("blockHash");
                            receipt
                        } else {
                            case["receiptJson"].clone()
                        }
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(rpc["params"][0], case["receiptJson"]["blockNumber"]);
                        case["blockJson"].clone()
                    }
                    other => panic!("unrecorded node fixture method: {other}"),
                }
            };
            http::write_response(
                &mut stream,
                &OutgoingResponse {
                    status: 200,
                    content_type: "application/json".into(),
                    headers: Vec::new(),
                    retry_after: None,
                    body: serde_json::to_vec(
                        &json!({"jsonrpc":"2.0", "id":rpc["id"], "result":result}),
                    )
                    .unwrap(),
                },
            )
            .unwrap();
        })?;
        Ok(Self {
            listener,
            state,
            fault,
            hide_receipt,
        })
    }
    fn reset(&self) {
        *self.state.lock().unwrap() = NodeState::default();
    }
}

struct Environment(Vec<(OsString, Option<OsString>)>);
impl Environment {
    fn new() -> Self {
        let keys = std::env::vars_os()
            .filter(|(key, _)| {
                key.to_string_lossy().starts_with("LAYERX_INTEROP_")
                    || [
                        "ALL_PROXY",
                        "all_proxy",
                        "HTTPS_PROXY",
                        "https_proxy",
                        "HTTP_PROXY",
                        "http_proxy",
                        "NO_PROXY",
                        "no_proxy",
                    ]
                    .contains(&key.to_string_lossy().as_ref())
            })
            .map(|(key, _)| key)
            .collect::<Vec<_>>();
        let mut environment = Self(Vec::new());
        for key in keys {
            environment.save(&key);
            std::env::remove_var(key);
        }
        environment
    }
    fn save(&mut self, key: &std::ffi::OsStr) {
        if !self.0.iter().any(|(saved, _)| saved == key) {
            self.0.push((key.to_owned(), std::env::var_os(key)));
        }
    }
    fn set(&mut self, key: &str, value: impl AsRef<std::ffi::OsStr>) {
        self.save(key.as_ref());
        std::env::set_var(key, value);
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        for (key, value) in self.0.iter().rev() {
            match value {
                Some(value) => std::env::set_var(key, value),
                None => std::env::remove_var(key),
            }
        }
    }
}

fn configure(dir: &Path, case: &Value) -> TestResult<Identity> {
    let group = EcGroup::from_curve_name(Nid::X9_62_PRIME256V1)?;
    let key = PKey::from_ec_key(EcKey::generate(&group)?)?;
    let mut name = X509NameBuilder::new()?;
    name.append_entry_by_text("CN", "localhost")?;
    let name = name.build();
    let mut certificate = X509::builder()?;
    certificate.set_version(2)?;
    certificate.set_subject_name(&name)?;
    certificate.set_issuer_name(&name)?;
    certificate.set_pubkey(&key)?;
    let before = Asn1Time::days_from_now(0)?;
    let after = Asn1Time::days_from_now(1)?;
    certificate.set_not_before(&before)?;
    certificate.set_not_after(&after)?;
    certificate.append_extension(BasicConstraints::new().critical().ca().build()?)?;
    let san = SubjectAlternativeName::new()
        .dns("localhost")
        .ip("127.0.0.1")
        .build(&certificate.x509v3_context(None, None))?;
    certificate.append_extension(san)?;
    certificate.sign(&key, MessageDigest::sha256())?;
    let certificate = certificate.build();
    fs::write(dir.join("cert.der"), certificate.to_der()?)?;
    let identity = Pkcs12::builder()
        .name("relay repair")
        .pkey(&key)
        .cert(&certificate)
        .build2("repair-password")?
        .to_der()?;
    fs::write(dir.join("client.p12"), &identity)?;
    let mut manifest: Value = serde_json::from_slice(include_bytes!(
        "../../../deploy/gateway/config.example.json"
    ))?;
    let receipt_public = case["pinnedKey"]["publicKey"]
        .as_str()
        .ok_or("receipt key")?
        .trim_start_matches("0x");
    manifest["x402_supported"] = json!({"kinds":[{"x402Version":2,"scheme":"exact","network":"eip155:125"}],
        "extensions":[],"signers":{"eip155:125":[format!("did:layerx:{receipt_public}")]}});
    let ap2 = p256::ecdsa::SigningKey::from_bytes((&[9; 32]).into())?;
    for pin in manifest["ap2_keys"].as_array_mut().ok_or("AP2 pins")? {
        pin["public_key_sec1"] = json!(hex(ap2.verifying_key().to_encoded_point(false).as_bytes()));
    }
    let public = hex(SigningKey::from_bytes(&[7; 32]).verifying_key().as_bytes());
    manifest["visa_agents"][0]["public_key"] = json!(public);
    manifest["visa_agents"][0]["layerx_agent"] = json!(public);
    manifest["visa_targets"][0]["authority"] = json!("shop.example");
    manifest["fiat_providers"][0]["public_key_ed25519"] = json!(public);
    fs::write(dir.join("config.json"), serde_json::to_vec(&manifest)?)?;
    fs::write(
        dir.join("receipt-keys.json"),
        serde_json::to_vec(&json!([case["pinnedKey"]]))?,
    )?;
    fs::write(
        dir.join("registry.json"),
        br#"{"modules":[{"module":1,"ordinals":[1]}]}"#,
    )?;
    for (name, contents) in [
        ("identity-password", "repair-password"),
        ("redis-username", "relay"),
        ("redis-password", "repair-secret"),
    ] {
        fs::write(dir.join(name), contents)?;
        fs::set_permissions(dir.join(name), fs::Permissions::from_mode(0o600))?;
    }
    fs::write(
        dir.join("ap2-signers.json"),
        serde_json::to_vec(&json!({
            "payment":{"issuer":"payment-issuer","key_id":"payment-key","private_key_hex":"17".repeat(32)},
            "checkout":{"issuer":"checkout-issuer","key_id":"checkout-key","private_key_hex":"1d".repeat(32)}
        }))?,
    )?;
    fs::set_permissions(
        dir.join("ap2-signers.json"),
        fs::Permissions::from_mode(0o600),
    )?;
    Ok(Identity::from_pkcs12(&identity, "repair-password")?)
}

fn gateway(dir: &Path, redis: &Redis, node: &Node) -> TestResult<Config> {
    let mut environment = Environment::new();
    for (name, file) in [
        ("CONFIG", "config.json"),
        ("OUTBOUND_CA_DER", "cert.der"),
        ("CLIENT_IDENTITY_PKCS12", "client.p12"),
        ("CLIENT_IDENTITY_PASSWORD_FILE", "identity-password"),
        ("REDIS_USERNAME_FILE", "redis-username"),
        ("REDIS_PASSWORD_FILE", "redis-password"),
        ("RECEIPT_KEYS_FILE", "receipt-keys.json"),
        ("MODULE_REGISTRY_FILE", "registry.json"),
        ("AP2_RECEIPT_SIGNERS_FILE", "ap2-signers.json"),
    ] {
        environment.set(&format!("LAYERX_INTEROP_{name}"), dir.join(file));
    }
    for (name, value) in [
        ("LISTENER", "plain"),
        ("LISTEN", "127.0.0.1:0"),
        ("NETWORK_ID", "eip155-125"),
        ("WIRE_VERSION", "3"),
        ("PROTOCOL_NETWORK_ID", "402"),
        ("CHAIN_ID", "125"),
        ("READINESS_MAX_AGE_MS", "30000"),
        ("IDEMPOTENCY_SECONDS", "1"),
        ("TAP_CLOCK_SKEW_SECONDS", "60"),
        ("ALLOW_RAILWAY_PRIVATE_NETWORK", "1"),
        ("SEQUENCER_URL", PRIVATE_URL),
    ] {
        environment.set(&format!("LAYERX_INTEROP_{name}"), value);
    }
    environment.set(
        "LAYERX_INTEROP_REDIS_URL",
        format!("rediss://localhost:{}", redis.listener.address.port()),
    );
    environment.set("ALL_PROXY", format!("http://{}", node.listener.address));
    environment.set("NO_PROXY", "");
    Ok(config::load()?)
}

fn unix_millis() -> u128 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_millis()
}

fn fixture() -> TestResult<Value> {
    let vectors: Value = serde_json::from_slice(include_bytes!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))?;
    Ok(vectors["cases"]
        .as_array()
        .ok_or("cases")?
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .ok_or("recorded payment")?
        .clone())
}

fn mounted(
    config: Config,
    method: &str,
    path: &str,
    body: &Value,
    authorization: Option<&str>,
    idempotency: &str,
) -> TestResult<(Config, u16, Value)> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let worker = thread::spawn(move || -> TestResult<Config> {
        let (mut stream, _) = listener.accept()?;
        stream.set_read_timeout(Some(TIMEOUT))?;
        stream.set_write_timeout(Some(TIMEOUT))?;
        let request = http::read_request(&mut stream, 512 * 1024)?;
        http::write_response(&mut stream, &server::route(&config, &request))?;
        Ok(config)
    });
    let mut stream = TcpStream::connect_timeout(&address, TIMEOUT)?;
    stream.set_read_timeout(Some(TIMEOUT))?;
    let body = serde_json::to_vec(body)?;
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\nIdempotency-Key: {idempotency}\r\n", body.len())?;
    if let Some(authorization) = authorization {
        write!(stream, "Authorization: {authorization}\r\n")?;
    }
    stream.write_all(b"\r\n")?;
    stream.write_all(&body)?;
    stream.flush()?;
    let status = line(&mut stream)?;
    let status = text(&status)
        .split_whitespace()
        .nth(1)
        .ok_or("response status")?
        .parse()?;
    let mut length = None;
    loop {
        let header = line(&mut stream)?;
        if header.is_empty() {
            break;
        }
        let (name, value) = text(&header).split_once(':').ok_or("response header")?;
        if name.eq_ignore_ascii_case("content-length") {
            length = Some(value.trim().parse::<usize>()?);
        }
    }
    let mut body = vec![0; length.ok_or("response content length")?];
    stream.read_exact(&mut body)?;
    Ok((
        worker.join().map_err(|_| "gateway thread panicked")??,
        status,
        serde_json::from_slice(&body)?,
    ))
}

fn issue(config: &Config) -> TestResult<String> {
    let issued = IssuedKey::generate()?;
    let record = KeyRecord {
        key_id: issued.id().into(),
        principal_digest: "33".repeat(32),
        salt: "repair-salt".into(),
        secret_digest: gateway_digest(&[
            b"gateway-key-v1",
            b"repair-salt",
            issued.secret().as_bytes(),
        ]),
        signer_public_key: hex(SigningKey::from_bytes(&[7; 32]).verifying_key().as_bytes()),
        scopes: "interop:*".into(),
        quota_requests: 100,
        quota_window_seconds: 60,
        epoch: 1,
        disabled: false,
    };
    config
        .store
        .issue_key(&record, "repair integration issuance")?;
    Ok(format!("LayerX-Key {}:{}", issued.id(), issued.secret()))
}

fn payer_bytes(case: &Value, chain: u64, expiry: u64) -> TestResult<Vec<u8>> {
    let original = bytes(case["rawTransaction"].as_str().ok_or("raw transaction")?)?;
    let (mut transaction, _) = decode_signed(&original)?;
    assert_eq!(&transaction.data[..4], &[0x78, 0xea, 0x11, 0xdb]);
    transaction.chain_id = chain;
    transaction.data[4 + 4 * 32 + 24..4 + 5 * 32].copy_from_slice(&expiry.to_be_bytes());
    Ok(transaction.sign(&[0x3b; 32])?)
}

fn x402_verify_request(raw: &[u8]) -> TestResult<Value> {
    let (transaction, _) = decode_signed(raw)?;
    let word = |index: usize| &transaction.data[4 + index * 32..4 + (index + 1) * 32];
    let amount = u128::from_be_bytes(word(3)[16..].try_into()?);
    let account = "agent:did:layerx:payee:main";
    let payee: [u8; 32] = word(1).try_into()?;
    assert!(layerx_x402::account_identifiers(account)?.contains(&payee));
    let requirements = json!({"scheme":"exact","network":layerx_x402::model::LAYERX_NETWORK, "asset":hex(word(2)), "amount":amount.to_string(), "payTo":hex(&payee), "maxTimeoutSeconds":60, "extra":{"layerx":{"account":account,"currency":"USD"}}});
    let accepted: layerx_x402::PaymentRequirements = serde_json::from_value(requirements.clone())?;
    accepted.validate()?;
    Ok(
        json!({"x402Version":2, "paymentPayload":{"x402Version":2,"accepted":requirements,"payload":{"transaction":format!("0x{}",hex(&transaction_hash(raw))),"layerxTransaction":format!("0x{}",hex(raw)),"layerxIdempotencyKey":hex(word(5))}}, "paymentRequirements":requirements}),
    )
}

#[test]
fn obsolete_service_config_and_readiness_component_type() -> TestResult {
    let _serial = SERIAL.lock().unwrap_or_else(|error| error.into_inner());
    let case = fixture()?;
    let dir = Directory::new()?;
    let identity = configure(&dir.0, &case)?;
    let node = Node::start(&case)?;
    let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let config = gateway(&dir.0, &redis, &node)?;
    let (config, status, response) =
        mounted(config, "GET", "/readyz", &Value::Null, None, "readiness")?;
    assert_eq!(status, 200, "{response}");
    let consumed = layerx_interop_service::operations::ReadinessDocument::from_json(
        &serde_json::to_vec(&response)?,
    )?;
    assert!(consumed.is_ready());
    assert_eq!(response["components"]["sequencer"], "ready");
    node.fault.store(3, Ordering::SeqCst);
    let (_, status, response) = mounted(config, "GET", "/readyz", &Value::Null, None, "readiness")?;
    assert_eq!(status, 503, "{response}");
    let consumed = layerx_interop_service::operations::ReadinessDocument::from_json(
        &serde_json::to_vec(&response)?,
    )?;
    assert!(!consumed.is_ready());
    assert_eq!(response["components"]["sequencer"], "unavailable");
    let mut invalid = response;
    invalid["components"]["sequencer"] = json!(false);
    assert!(
        layerx_interop_service::operations::ReadinessDocument::from_json(&serde_json::to_vec(
            &invalid
        )?)
        .is_err()
    );
    Ok(())
}

#[test]
fn readiness_cli_declaration_and_concrete_proof_workflow() -> TestResult {
    use layerx_ap2::PortableLayerXEvidence;
    use layerx_interop_service::operations::{validate_startup_arguments, verify_receipted_json};
    use layerx_proof::inclusion::SequencerAuthorization;
    validate_startup_arguments(&[])?;
    assert!(validate_startup_arguments(&[
        "--verify-readiness-receipt".into(),
        "receipt.json".into()
    ])
    .is_err());
    let case = fixture()?;
    let key = &case["pinnedKey"];
    let pin = SequencerAuthorization::new(
        bytes(key["keyId"].as_str().ok_or("key id")?)?
            .try_into()
            .unwrap(),
        bytes(key["publicKey"].as_str().ok_or("public key")?)?
            .try_into()
            .unwrap(),
        key["firstHeight"].as_u64().unwrap(),
        key["lastHeight"].as_u64().unwrap(),
    );
    let raw = bytes(case["rawTransaction"].as_str().unwrap())?;
    let exported =
        PortableLayerXEvidence::from_sequencer_receipt(&case["receiptJson"], &[pin], Some(&raw))?;
    let document = exported.to_json()?;
    let digest = verify_receipted_json(&document, &[pin])?;
    assert_eq!(format!("0x{}", hex(&digest)), case["receiptDigest"]);
    assert!(verify_receipted_json(&document, &[]).is_err());
    let mut tampered: Value = serde_json::from_slice(&document)?;
    tampered["verificationLevel"] = json!("committed");
    assert!(verify_receipted_json(&serde_json::to_vec(&tampered)?, &[pin]).is_err());
    Ok(())
}

#[test]
fn payer_chain_id_through_actual_verify_handler() -> TestResult {
    let _serial = SERIAL.lock().unwrap_or_else(|error| error.into_inner());
    let case = fixture()?;
    let dir = Directory::new()?;
    let identity = configure(&dir.0, &case)?;
    let node = Node::start(&case)?;
    let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let mut config = gateway(&dir.0, &redis, &node)?;
    let authorization = issue(&config)?;
    let expiry = server::payment_now_millis()? + 60000;
    for (chain, valid) in [(1, false), (125, true)] {
        let raw = payer_bytes(&case, chain, expiry)?;
        let (tx, sig) = decode_signed(&raw)?;
        assert_eq!(tx.chain_id, chain);
        assert_eq!(
            recover_sender(&tx, &sig)?,
            recover_sender(
                &decode_signed(&payer_bytes(&case, 125, expiry)?)?.0,
                &decode_signed(&payer_bytes(&case, 125, expiry)?)?.1
            )?
        );
        let body = x402_verify_request(&raw)?;
        let (next, status, response) = mounted(
            config,
            "POST",
            "/v1/http/x402/facilitator/verify",
            &body,
            Some(&authorization),
            &format!("chain-{chain}"),
        )?;
        config = next;
        assert_eq!(status, 200, "{response}");
        assert_eq!(response["result"]["isValid"], valid, "{response}");
        if !valid {
            assert_eq!(
                response["result"]["invalidReason"],
                "transaction_chain_id_refused"
            );
        }
    }
    assert!(node.state.lock().unwrap().attempts.is_empty());
    Ok(())
}

#[test]
fn payment_expiry_units_through_mounted_real_clock_route() -> TestResult {
    let _serial = SERIAL.lock().unwrap_or_else(|error| error.into_inner());
    let case = fixture()?;
    let dir = Directory::new()?;
    let identity = configure(&dir.0, &case)?;
    let node = Node::start(&case)?;
    let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let mut config = gateway(&dir.0, &redis, &node)?;
    let authorization = issue(&config)?;
    let now = server::payment_now_millis()?;
    assert!(!server::payment_expired(now, now - 1));
    assert!(server::payment_expired(now, now));
    assert!(server::payment_expired(now, now + 1));
    assert!(now > SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs() * 999);
    for (label, expiry, valid) in [
        ("before", now + 60000, true),
        ("at", now, false),
        ("after", now - 1, false),
    ] {
        let raw = payer_bytes(&case, 125, expiry)?;
        let (next, status, response) = mounted(
            config,
            "POST",
            "/v1/http/x402/facilitator/verify",
            &x402_verify_request(&raw)?,
            Some(&authorization),
            label,
        )?;
        config = next;
        assert_eq!(status, 200, "{response}");
        assert_eq!(response["result"]["isValid"], valid, "{response}");
        if !valid {
            assert_eq!(
                response["result"]["invalidReason"],
                "activity_time_window_refused"
            );
        }
    }
    // The unchanged ledger-produced capture is now expired in real milliseconds.
    let raw = bytes(case["rawTransaction"].as_str().unwrap())?;
    let (_, status, response) = mounted(
        config,
        "POST",
        "/v1/http/x402/facilitator/verify",
        &x402_verify_request(&raw)?,
        Some(&authorization),
        "recorded-expired",
    )?;
    assert_eq!(status, 200, "{response}");
    assert_eq!(
        response["result"]["invalidReason"],
        "activity_time_window_refused"
    );
    assert!(node.state.lock().unwrap().attempts.is_empty());
    Ok(())
}

fn committed_case() -> TestResult<Value> {
    let mut case = fixture()?;
    let hash = format!("0x{}", "ab".repeat(32));
    case["receiptJson"]["rung"] = json!("committed");
    case["receiptJson"]["blockNumber"] = json!("0x3e9");
    case["receiptJson"]["blockHash"] = json!(hash);
    case["blockJson"] = json!({"number":"0x3e9","hash":hash,"parentHash":format!("0x{}","00".repeat(32)),"timestamp":"0x67a1b2c3","transactionsRoot":format!("0x{}","00".repeat(32)),"receiptsRoot":format!("0x{}","00".repeat(32)),"transactions":[]});
    Ok(case)
}

#[test]
fn relay_retention_reopens_and_late_replay_never_submits_again() -> TestResult {
    let _serial = SERIAL.lock().unwrap_or_else(|error| error.into_inner());
    let case = committed_case()?;
    let raw = bytes(case["rawTransaction"].as_str().unwrap())?;
    let (tx, sig) = decode_signed(&raw)?;
    let sender = recover_sender(&tx, &sig)?;
    let hash = transaction_hash(&raw);
    let mut unknown = tx.clone();
    unknown.nonce += 1;
    let unknown = unknown.sign(&[0x3b; 32])?;
    let (unknown_tx, unknown_sig) = decode_signed(&unknown)?;
    let unknown_sender = recover_sender(&unknown_tx, &unknown_sig)?;
    let unknown_hash = transaction_hash(&unknown);
    let dir = Directory::new()?;
    let identity = configure(&dir.0, &case)?;
    let node = Node::start(&case)?;
    let mut redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let journal_key = format!("interop:relay:v1:{:x}", Sha256::digest(b"finalized"));
    let unresolved_key = format!("interop:relay:v1:{:x}", Sha256::digest(b"unresolved"));
    let config = gateway(&dir.0, &redis, &node)?;
    let relay = ChainRelay::from_config(&config);
    redis.lose_claim_reply.store(true, Ordering::SeqCst);
    assert!(matches!(
        relay.relay("unresolved", &unknown, unknown_sender),
        Err(RelayError::Journal(_))
    ));
    node.fault.store(4, Ordering::SeqCst);
    let RelayOutcome::Executed(evidence) = relay.relay("finalized", &raw, sender)? else {
        return Err("verified receipt required".into());
    };
    assert_eq!(
        evidence.rung,
        layerx_interop_gateway::evidence::SettlementRung::Receipted
    );
    assert_eq!(redis.record(&format!("{journal_key}:finalized")), None);
    assert_eq!(relay.compact_finalized()?, 0);
    drop(relay);
    drop(config);
    thread::sleep(Duration::from_millis(1100));
    let address = redis.listener.address;
    drop(redis);
    redis = Redis::start(&dir.0, &identity, address)?;
    let config = gateway(&dir.0, &redis, &node)?;
    let relay = ChainRelay::from_config(&config);
    assert!(matches!(
        relay.recorded_evidence("finalized")?,
        Some(RelayOutcome::Executed(_))
    ));
    assert_eq!(
        redis.record(&journal_key),
        Some([hash.to_vec(), raw.clone()].concat()),
        "uncommitted receipt must not expire"
    );
    assert_eq!(
        redis.record(&unresolved_key),
        Some([unknown_hash.to_vec(), unknown.clone()].concat())
    );
    node.fault.store(0, Ordering::SeqCst);
    assert_eq!(
        relay.compact_finalized()?,
        1,
        "the production maintenance path discovers finality after reopen"
    );
    assert_eq!(
        redis.record(&format!("{journal_key}:finalized")),
        Some(hash.to_vec())
    );
    drop(relay);
    drop(config);
    thread::sleep(Duration::from_millis(1100));
    let address = redis.listener.address;
    drop(redis);
    redis = Redis::start(&dir.0, &identity, address)?;
    let config = gateway(&dir.0, &redis, &node)?;
    let relay = ChainRelay::from_config(&config);
    assert!(matches!(
        relay.relay("finalized", &raw, sender)?,
        RelayOutcome::Executed(_)
    ));
    assert_eq!(redis.record(&journal_key), None);
    assert_eq!(redis.record(&format!("{journal_key}:submitted")), None);
    assert_eq!(
        redis.record(&format!("{journal_key}:finalized")),
        Some(hash.to_vec())
    );
    assert_eq!(
        redis.record(&unresolved_key),
        Some([unknown_hash.to_vec(), unknown].concat())
    );
    node.hide_receipt.store(true, Ordering::SeqCst);
    assert_eq!(
        relay.relay("finalized", &raw, sender),
        Err(RelayError::FinalizedEvidenceUnavailable)
    );
    assert_eq!(relay.compact_finalized()?, 0);
    let state = node.state.lock().unwrap();
    assert_eq!(state.accepted.len(), 1);
    assert_eq!(state.attempts, vec![raw]);
    Ok(())
}

#[test]
fn committed_claim_without_proof_stays_receipted() -> TestResult {
    let _serial = SERIAL.lock().unwrap_or_else(|error| error.into_inner());
    for (label, block) in [
        ("absent", Value::Null),
        ("matching", committed_case()?["blockJson"].clone()),
        (
            "unrelated",
            json!({"number":"0x3e9","hash":format!("0x{}","cd".repeat(32)),"parentHash":format!("0x{}","00".repeat(32)),"timestamp":"0x67a1b2c3","transactionsRoot":format!("0x{}","00".repeat(32)),"receiptsRoot":format!("0x{}","00".repeat(32)),"transactions":[]}),
        ),
    ] {
        let mut case = committed_case()?;
        case["blockJson"] = block;
        let dir = Directory::new()?;
        let identity = configure(&dir.0, &case)?;
        let node = Node::start(&case)?;
        let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
        let config = gateway(&dir.0, &redis, &node)?;
        let raw = bytes(case["rawTransaction"].as_str().unwrap())?;
        let (tx, sig) = decode_signed(&raw)?;
        let RelayOutcome::Executed(evidence) =
            ChainRelay::from_config(&config).relay(label, &raw, recover_sender(&tx, &sig)?)?
        else {
            return Err("verified receipt required".into());
        };
        assert_eq!(
            evidence.rung,
            layerx_interop_gateway::evidence::SettlementRung::Receipted
        );
        assert_eq!(
            evidence.signed_receipt,
            bytes(case["signedReceipt"].as_str().unwrap())?
        );
    }
    Ok(())
}

fn es256_key(seed: u8) -> p256::ecdsa::SigningKey {
    p256::ecdsa::SigningKey::from_bytes((&[seed; 32]).into()).unwrap()
}

fn jws(header: Value, payload: Value, key: &p256::ecdsa::SigningKey) -> String {
    use base64::engine::general_purpose::URL_SAFE_NO_PAD;
    use p256::ecdsa::signature::Signer as _;
    let input = format!(
        "{}.{}",
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&header).unwrap()),
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&payload).unwrap())
    );
    let signature: p256::ecdsa::Signature = key.sign(input.as_bytes());
    format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
}

struct MandateKeys;
impl layerx_ap2::KeyResolver for MandateKeys {
    fn resolve(
        &self,
        usage: layerx_ap2::KeyUse,
        header: &layerx_ap2::ProtectedHeader,
    ) -> Result<p256::ecdsa::VerifyingKey, layerx_ap2::Ap2Error> {
        let expected = match usage {
            layerx_ap2::KeyUse::CheckoutMandateIssuer => "layerx-beta-checkout-mandate-key",
            layerx_ap2::KeyUse::PaymentMandateIssuer => "layerx-beta-payment-mandate-key",
            layerx_ap2::KeyUse::MerchantCheckout => "layerx-beta-merchant-checkout-key",
        };
        if header.key_id() != Some(expected) {
            return Err(layerx_ap2::Ap2Error::KeyResolution);
        }
        Ok(*es256_key(9).verifying_key())
    }
}

fn mandates(now: u64) -> (String, String) {
    use base64::engine::general_purpose::URL_SAFE_NO_PAD;
    let checkout = jws(
        json!({"alg":"ES256","typ":"JWT","kid":"layerx-beta-merchant-checkout-key"}),
        json!({
        "id":"checkout-223386","merchant":{"id":"merchant-001","name":"Test Merchant"},
        "line_items":[{"id":"line-001","item":{"id":"sku-001","title":"Node credit","price":1000000},"quantity":1,"totals":[{"type":"total","amount":1000000}]}],
        "status":"completed","currency":"USD","totals":[{"type":"total","amount":1000000}],"links":[]}),
        &es256_key(9),
    );
    let reference = URL_SAFE_NO_PAD.encode(Sha256::digest(checkout.as_bytes()));
    let root = |kid, mandate| {
        format!(
            "{}~",
            jws(
                json!({"alg":"ES256","typ":"dc+sd-jwt","kid":kid}),
                json!({"iat":now-60,"exp":now+3600,"delegate_payload":[mandate]}),
                &es256_key(9)
            )
        )
    };
    (
        root(
            "layerx-beta-checkout-mandate-key",
            json!({"vct":"mandate.checkout.1","checkout_jwt":checkout,"checkout_hash":reference,"iat":now-30,"exp":now+1800}),
        ),
        root(
            "layerx-beta-payment-mandate-key",
            json!({"vct":"mandate.payment.1","transaction_id":reference,"payee":{"id":"merchant-001","name":"Test Merchant"},"payment_amount":{"amount":1000000,"currency":"USD"},"payment_instrument":{"id":"card-001","type":"card"},"iat":now-30,"exp":now+1800}),
        ),
    )
}

// The loopback node is a fixture peer, not the component under repair. Emit a
// fresh, internally consistent transcript using the production payer signer,
// receipt Encoder/decoder and signature/log verifiers. Do not rewrite a recorded
// signature: all transaction and sequence identities are derived anew.
fn hosted_ap2_fixture() -> TestResult<(Value, Value, layerx_ap2::LayerXAssetBinding)> {
    use layerx_ap2::{
        authorize_payment, LayerXAssetBinding, MandateVerifier, Merchant, VerificationContext,
    };
    use layerx_interop_gateway::principal::PrincipalId;
    let mut case = fixture()?;
    let original = bytes(case["rawTransaction"].as_str().unwrap())?;
    let (mut transaction, _) = decode_signed(&original)?;
    let word = |index: usize| -> [u8; 32] {
        transaction.data[4 + index * 32..4 + (index + 1) * 32]
            .try_into()
            .unwrap()
    };
    let binding = LayerXAssetBinding {
        currency: "USD".into(),
        minor_unit_exponent: 2,
        atomic_units_per_minor_unit: 1,
        asset: word(2),
        payer_receipt_account: word(0),
        payee_receipt_account: word(1),
        payee_merchant: Merchant::new("merchant-001", "Test Merchant", None)?,
    };
    let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
    let pair = mandates(now);
    let verification = VerificationContext {
        now,
        clock_skew_seconds: 0,
        expected_audience: "https://merchant.example",
        expected_nonce: "nonce",
        currency_minor_exponent: 2,
        usage: None,
    };
    let verified = MandateVerifier::new(&MandateKeys).verify(&pair.0, &pair.1, &verification)?;
    let payment = authorize_payment(
        &PrincipalId::new("33".repeat(32))?,
        &verified,
        &verification,
        &binding,
    )?;
    transaction.nonce = 0;
    transaction.data[4 + 4 * 32 + 24..4 + 5 * 32]
        .copy_from_slice(&(server::payment_now_millis()? + 600000).to_be_bytes());
    transaction.data[4 + 5 * 32..4 + 6 * 32].copy_from_slice(&payment.idempotency_key());
    let raw = transaction.sign(&[0x3b; 32])?;
    let (tx, sig) = decode_signed(&raw)?;
    let sender = recover_sender(&tx, &sig)?;
    let tx_hash = transaction_hash(&raw);
    let data = [
        binding.asset.to_vec(),
        vec![0; 16],
        payment.amount().to_be_bytes().to_vec(),
    ]
    .concat();
    let log = json!({"address":case["receiptJson"]["logs"][0]["address"],"topics":[case["receiptJson"]["logs"][0]["topics"][0],format!("0x{}",hex(&payment.idempotency_key())),format!("0x{}",hex(&binding.payer_receipt_account)),format!("0x{}",hex(&binding.payee_receipt_account))],"data":format!("0x{}",hex(&data)),"logIndex":"0x0"});
    let receipt = emit_node_receipt(&raw, &log)?;
    let key = SigningKey::from_bytes(&[0x72; 32]);
    case["rawTransaction"] = json!(format!("0x{}", hex(&raw)));
    case["signedReceipt"] = json!(format!("0x{}", hex(&receipt)));
    case["receiptDigest"] = json!(format!(
        "0x{}",
        hex(&layerx_interop_gateway::evidence::canonical_receipt_digest(
            &receipt
        )?)
    ));
    case["prevSeqHash"] = json!(format!("0x{}", "00".repeat(32)));
    case["pinnedKey"]["publicKey"] = json!(format!("0x{}", hex(key.verifying_key().as_bytes())));
    let parsed = wire(layerx_wire::receipt::decode(&receipt))?;
    let protocol = parsed.protocol().ok_or("protocol receipt")?;
    let view = &mut case["receiptJson"];
    view["txHash"] = json!(format!("0x{}", hex(&tx_hash)));
    view["nonce"] = json!("0x0");
    view["seq"] = json!("0x1");
    view["index"] = json!("0x0");
    view["timestamp"] = json!(format!("{:#x}", protocol.timestamp()));
    view["receipt"] = json!(format!("0x{}", hex(&receipt)));
    view["from"] = json!(format!("0x{}", hex(&sender)));
    view["logs"] = json!([log]);
    view["signingKey"] = json!(format!("0x{}", hex(key.verifying_key().as_bytes())));
    let request = json!({"checkout_presentation":pair.0,"payment_presentation":pair.1,"nonce":"nonce","activity":hex(&raw)});
    Ok((case, request, binding))
}

fn emit_node_receipt(raw: &[u8], log: &Value) -> TestResult<Vec<u8>> {
    use layerx_wire::encode::Encoder;
    // Sequencer layout from rpc/sequencer_receipt.go and the canonical
    // ProtocolReceipt encoder in layerx-wire; this isolated fixture starts at seq 1.
    let recorded = fixture()?;
    let old = wire(layerx_wire::receipt::decode(&bytes(
        recorded["signedReceipt"].as_str().unwrap(),
    )?))?;
    let old = old.protocol().ok_or("protocol receipt")?;
    let (transaction, signature) = decode_signed(raw)?;
    let sender = recover_sender(&transaction, &signature)?;
    let tx_hash = transaction_hash(raw);
    let seq = 1_u64;
    let mut chain = Sha256::new();
    chain.update(b"PAXEER/v1/sequence\0");
    chain.update([0; 32]);
    chain.update(seq.to_be_bytes());
    chain.update(tx_hash);
    let resulting: [u8; 32] = chain.finalize().into();
    let mut summary = old.effects()[0].body().to_vec();
    summary[24..28].copy_from_slice(&0_u32.to_be_bytes());
    let mut log_body = bytes(log["address"].as_str().unwrap())?;
    log_body.push(4);
    for topic in log["topics"].as_array().unwrap() {
        log_body.extend(bytes(topic.as_str().unwrap())?);
    }
    log_body.extend(Sha256::digest(bytes(log["data"].as_str().unwrap())?));
    let mut encoder = Encoder::new(512 * 1024);
    wire(encoder.structure_header_version(0x5201, 2))?;
    wire(encoder.u16(2))?;
    wire(encoder.bytes(&tx_hash, 32))?;
    wire(encoder.u64(seq))?;
    wire(encoder.bytes(&[0; 32], 32))?;
    wire(encoder.bytes(&resulting, 32))?;
    wire(encoder.bytes(&Sha256::digest([]), 32))?;
    wire(encoder.i32(0))?;
    wire(encoder.sequence_length(2, 512))?;
    for (index, body) in [summary, log_body].into_iter().enumerate() {
        let effect = &old.effects()[index];
        wire(encoder.u16(effect.module_id()))?;
        wire(encoder.u16(index as u16))?;
        wire(encoder.u16(effect.event_type()))?;
        wire(encoder.u8(effect.kind()))?;
        wire(encoder.u8(u8::from(effect.monetary())))?;
        wire(encoder.bytes(&effect.transfer_set_root(), 32))?;
        wire(encoder.bytes(&body, 256))?;
    }
    wire(encoder.u128(old.fee_charged()))?;
    wire(encoder.bytes(&old.batch_id(), 32))?;
    wire(encoder.u16(old.module_id()))?;
    wire(encoder.u32(old.module_version()))?;
    wire(encoder.u32(old.parameter_version()))?;
    wire(encoder.u8(old.operation()))?;
    wire(encoder.bytes(&old.asset(), 32))?;
    wire(encoder.u128(old.amount()))?;
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    wire(encoder.bytes(&from, 32))?;
    wire(encoder.u128(old.debit_balance_before()))?;
    wire(encoder.u128(old.debit_balance_after()))?;
    wire(encoder.u64(transaction.nonce))?;
    wire(encoder.bytes(&old.to(), 32))?;
    wire(encoder.u128(old.credit_balance_before()))?;
    wire(encoder.u128(old.credit_balance_after()))?;
    wire(encoder.bytes(&old.transfer_set_root(), 32))?;
    wire(encoder.bytes(&old.authorization_hash(), 32))?;
    wire(encoder.bytes(&old.context_hash(), 32))?;
    wire(encoder.u64(server::payment_now_millis()?))?;
    wire(encoder.u8(0))?;
    let mut unsigned = encoder.finish();
    let mut hasher = Sha256::new();
    hasher.update(b"LXP/v1/receipt\0");
    hasher.update(&unsigned);
    let digest: [u8; 32] = hasher.finalize().into();
    assert_eq!(unsigned.pop(), Some(0));
    unsigned.push(1);
    unsigned.extend(64_u32.to_be_bytes());
    unsigned.extend(SigningKey::from_bytes(&[0x72; 32]).sign(&digest).to_bytes());
    let decoded = wire(layerx_wire::receipt::decode(&unsigned))?;
    assert_eq!(wire(layerx_wire::receipt::encode(&decoded))?, unsigned);
    assert_eq!(
        layerx_interop_gateway::evidence::canonical_receipt_digest(&unsigned)?,
        digest
    );
    Ok(unsigned)
}

fn configure_ap2(
    dir: &Path,
    case: &Value,
    binding: &layerx_ap2::LayerXAssetBinding,
) -> TestResult<Identity> {
    let identity = configure(dir, case)?;
    let mut manifest: Value = serde_json::from_slice(&fs::read(dir.join("config.json"))?)?;
    let pin = &mut manifest["ap2_assets"][0];
    pin["audience"] = json!("https://merchant.example");
    pin["currency"] = json!("USD");
    pin["minor_unit_exponent"] = json!(2);
    pin["atomic_units_per_minor_unit"] = json!("1");
    pin["asset"] = json!(hex(&binding.asset));
    pin["payer_account"] = json!(hex(&binding.payer_receipt_account));
    pin["payee_account"] = json!(hex(&binding.payee_receipt_account));
    pin["payee_merchant_id"] = json!("merchant-001");
    pin["payee_merchant_name"] = json!("Test Merchant");
    fs::write(dir.join("config.json"), serde_json::to_vec(&manifest)?)?;
    Ok(identity)
}

fn verify_receipt_jwt(jwt: &str, key: &p256::ecdsa::VerifyingKey, kid: &str) -> TestResult<Value> {
    use base64::engine::general_purpose::URL_SAFE_NO_PAD;
    use p256::ecdsa::signature::Verifier as _;
    let parts: Vec<_> = jwt.split('.').collect();
    assert_eq!(parts.len(), 3);
    let signature = p256::ecdsa::Signature::from_slice(&URL_SAFE_NO_PAD.decode(parts[2])?)?;
    key.verify(format!("{}.{}", parts[0], parts[1]).as_bytes(), &signature)?;
    let header: Value = serde_json::from_slice(&URL_SAFE_NO_PAD.decode(parts[0])?)?;
    assert_eq!(header, json!({"alg":"ES256","typ":"JWT","kid":kid}));
    Ok(serde_json::from_slice(&URL_SAFE_NO_PAD.decode(parts[1])?)?)
}

#[test]
fn hosted_ap2_signed_export_and_restarted_replay() -> TestResult {
    let _serial = SERIAL.lock().unwrap_or_else(|error| error.into_inner());
    let (case, body, binding) = hosted_ap2_fixture()?;
    let dir = Directory::new()?;
    let identity = configure_ap2(&dir.0, &case, &binding)?;
    let node = Node::start(&case)?;
    let mut redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let mut config = gateway(&dir.0, &redis, &node)?;
    // The gateway replay window may exceed the journal's independently tested 1s fixture.
    config.idempotency_seconds = 60;
    let authorization = issue(&config)?;
    let (config, status, response) = mounted(
        config,
        "POST",
        "/v1/http/ap2/execute",
        &body,
        Some(&authorization),
        "ap2",
    )?;
    assert_eq!(status, 200, "{response}");
    assert_eq!(response["result"]["verificationLevel"], "receipted");
    let result = &response["result"];
    let portable =
        layerx_ap2::PortableLayerXEvidence::from_json(&serde_json::to_vec(&result["layerx"])?)?;
    let verified = portable.verify_v2(&config.receipt_keys)?;
    assert_eq!(
        verified.raw_transaction(),
        Some(bytes(case["rawTransaction"].as_str().unwrap())?.as_slice())
    );
    assert_eq!(
        verified.activity_id(),
        transaction_hash(&bytes(case["rawTransaction"].as_str().unwrap())?)
    );
    let pair = (
        body["checkout_presentation"].as_str().unwrap(),
        body["payment_presentation"].as_str().unwrap(),
    );
    let context = layerx_ap2::VerificationContext {
        now: SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs(),
        clock_skew_seconds: 0,
        expected_audience: "https://merchant.example",
        expected_nonce: "nonce",
        currency_minor_exponent: 2,
        usage: None,
    };
    let mandates =
        layerx_ap2::MandateVerifier::new(&MandateKeys).verify(pair.0, pair.1, &context)?;
    for (field, seed, kid, reference) in [
        (
            "payment_receipt_jwt",
            23,
            "payment-key",
            mandates.payment_receipt_reference(),
        ),
        (
            "checkout_receipt_jwt",
            29,
            "checkout-key",
            mandates.checkout_receipt_reference(),
        ),
    ] {
        let payload = verify_receipt_jwt(
            result[field].as_str().ok_or("signed AP2 receipt")?,
            es256_key(seed).verifying_key(),
            kid,
        )?;
        assert_eq!(payload["reference"], reference);
        assert_eq!(payload["status"], "Success");
        assert_eq!(payload["layerx_evidence"], result["layerx"]);
    }
    let operation = response["operation"].as_str().ok_or("operation")?;
    let stored = config
        .store
        .operation(operation)?
        .ok_or("stored signed export")?;
    assert_eq!(stored.state, "completed");
    let (metadata, body_hex) = stored
        .response
        .strip_prefix("http-v1:")
        .and_then(|encoded| encoded.split_once(':'))
        .ok_or("stored HTTP response envelope")?;
    let metadata: Value = serde_json::from_slice(&bytes(metadata)?)?;
    assert_eq!(metadata["status"], 200);
    assert_eq!(metadata["content_type"], "application/json");
    assert_eq!(bytes(body_hex)?, serde_json::to_vec(&response)?);
    drop(config);
    let address = redis.listener.address;
    drop(redis);
    redis = Redis::start(&dir.0, &identity, address)?;
    let config = gateway(&dir.0, &redis, &node)?;
    let (_, status, replay) = mounted(
        config,
        "POST",
        "/v1/http/ap2/execute",
        &body,
        Some(&authorization),
        "ap2",
    )?;
    assert_eq!(status, 200, "{replay}");
    assert_eq!(replay, response);
    let state = node.state.lock().unwrap();
    assert_eq!(state.attempts.len(), 1);
    assert_eq!(state.accepted.len(), 1);
    Ok(())
}

#[test]
fn committed_claim_without_proof_hosted_ap2_response() -> TestResult {
    let _serial = SERIAL.lock().unwrap_or_else(|error| error.into_inner());
    for scenario in ["absent", "unrelated", "matching"] {
        let (mut case, body, binding) = hosted_ap2_fixture()?;
        let committed = committed_case()?;
        case["receiptJson"]["rung"] = json!("committed");
        case["receiptJson"]["blockNumber"] = committed["receiptJson"]["blockNumber"].clone();
        case["receiptJson"]["blockHash"] = committed["receiptJson"]["blockHash"].clone();
        case["blockJson"] = match scenario {
            "absent" => Value::Null,
            "unrelated" => {
                let mut block = committed["blockJson"].clone();
                block["hash"] = json!(format!("0x{}", "cd".repeat(32)));
                block
            }
            "matching" => committed["blockJson"].clone(),
            _ => unreachable!(),
        };
        let dir = Directory::new()?;
        let identity = configure_ap2(&dir.0, &case, &binding)?;
        let node = Node::start(&case)?;
        let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
        let config = gateway(&dir.0, &redis, &node)?;
        let authorization = issue(&config)?;
        let (config, status, response) = mounted(
            config,
            "POST",
            "/v1/http/ap2/execute",
            &body,
            Some(&authorization),
            scenario,
        )?;
        assert_eq!(status, 200, "{scenario}: {response}");
        let result = &response["result"];
        assert_eq!(result["verificationLevel"], "receipted");
        assert_eq!(result["layerx"]["verificationLevel"], "receipted");
        let portable =
            layerx_ap2::PortableLayerXEvidence::from_json(&serde_json::to_vec(&result["layerx"])?)?;
        portable.verify_v2(&config.receipt_keys)?;
        for (field, seed, kid) in [
            ("payment_receipt_jwt", 23, "payment-key"),
            ("checkout_receipt_jwt", 29, "checkout-key"),
        ] {
            let payload = verify_receipt_jwt(
                result[field].as_str().ok_or("signed receipt")?,
                es256_key(seed).verifying_key(),
                kid,
            )?;
            assert_eq!(payload["layerx_evidence"]["verificationLevel"], "receipted");
        }
        assert!(node
            .state
            .lock()
            .unwrap()
            .methods
            .iter()
            .any(|method| method == "eth_getBlockByNumber"));
    }
    Ok(())
}
