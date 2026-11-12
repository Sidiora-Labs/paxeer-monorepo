use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_interop_service::config::{self, Config};
use layerx_interop_service::relay::RelayOutcome;
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
const PRIVATE_URL: &str = "http://router.railway.internal:9443";
const PAYMENT_PATH: &str = "/v1/http/visa-tap/intents/execute";
const TIMEOUT: Duration = Duration::from_secs(5);

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        write!(&mut encoded, "{byte:02x}")
            .unwrap_or_else(|error| panic!("hex encoding: {error:?}"));
    }
    encoded
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
            "repair-22-31-164-{}-{}",
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
                        stream
                            .set_read_timeout(Some(TIMEOUT))
                            .or_panic("test value");
                        stream
                            .set_write_timeout(Some(TIMEOUT))
                            .or_panic("test value");
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
        self.worker
            .take()
            .or_panic("test value")
            .join()
            .or_panic("test value");
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
    std::str::from_utf8(value).or_panic("test value")
}

#[derive(Default, Serialize, Deserialize)]
pub struct StoreState {
    strings: BTreeMap<String, Vec<u8>>,
    hashes: BTreeMap<String, BTreeMap<String, Vec<u8>>>,
    sets: BTreeMap<String, BTreeSet<Vec<u8>>>,
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
    fn eval_relay(&mut self, script: &str, keys: &[String], args: &[Vec<u8>]) -> Vec<u8> {
        let k = |i: usize| keys[i - 1].as_str();
        let a = |i: usize| args[i - 1].as_slice();
        if script.contains("string.char(1) .. ARGV[1]") {
            assert_eq!(keys.len(), 1);
            let old = self.strings.get(k(1)).cloned();
            let (status, record) = if let Some(old) = old {
                (0, old)
            } else {
                self.strings.insert(k(1).into(), a(1).to_vec());
                (1, a(1).to_vec())
            };
            return bulk(Some(&[vec![status], record].concat()));
        }
        if script.contains("local progress") {
            assert_eq!(keys.len(), 2);
            return match self.strings.get(k(1)) {
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
        assert!(script.contains("relay") || script.contains("ARGV[2]"));
        assert_eq!(keys.len(), 2);
        if self.strings.get(k(1)).map(Vec::as_slice) != Some(a(1)) {
            return bulk(None);
        }
        self.strings.insert(k(2).into(), a(2).to_vec());
        bulk(Some(a(2)))
    }
    fn eval_reserved(&mut self, keys: &[String], args: &[Vec<u8>]) -> Vec<u8> {
        let k = |i: usize| keys[i - 1].as_str();
        let a = |i: usize| args[i - 1].as_slice();
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
            text(self.value(k(2))).parse().or_panic("test value")
        };
        if used >= text(a(2)).parse().or_panic("test value") {
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
        tag("reserved")
    }
    fn eval_issued(&mut self, keys: &[String], args: &[Vec<u8>]) -> Vec<u8> {
        let k = |i: usize| keys[i - 1].as_str();
        let a = |i: usize| args[i - 1].as_slice();
        assert_eq!(keys.len(), 4);
        if self.value(k(4)) != a(12) {
            return tag("audit_retry");
        }
        if self.hashes.contains_key(k(1)) {
            return tag("conflict");
        }
        if self.sets.get(k(2)).map_or(0, BTreeSet::len)
            >= text(a(10)).parse().or_panic("test value")
        {
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
        tag("issued")
    }
    fn eval(&mut self, script: &str, keys: &[String], args: &[Vec<u8>]) -> Vec<u8> {
        let k = |i: usize| keys[i - 1].as_str();
        let a = |i: usize| args[i - 1].as_slice();
        if k(1).starts_with("interop:relay:v1:") {
            return self.eval_relay(script, keys, args);
        }
        if script.contains("return {'issued'}") {
            return self.eval_issued(keys, args);
        }
        if script.contains("return {'reserved'}") {
            return self.eval_reserved(keys, args);
        }
        if script.contains("local binding") || script.contains("return {'consumed'}") {
            assert_eq!(keys.len(), 4);
            if self.value(k(4)) != a(18) {
                return tag("audit_retry");
            }
            let fields = [
                ("principal", a(3)),
                ("key_id", a(4)),
                ("layerx_agent", a(5)),
                ("trusted_agent_id", a(6)),
                ("trusted_agent_domain", a(7)),
                ("intent", a(8)),
                ("evidence_digest", a(9)),
                ("activity_id", a(10)),
                ("signer_public_key", a(11)),
                ("target_authority", a(12)),
                ("target_path", a(13)),
                ("operation_identity", a(14)),
                ("credential_expires_at", a(15)),
            ];
            let outcome = if let Some(binding) = self.strings.get(k(1)) {
                if binding != a(2) {
                    "replay"
                } else if fields
                    .iter()
                    .all(|(field, value)| self.field(k(2), field) == *value)
                {
                    "existing"
                } else {
                    "conflict"
                }
            } else if self.hashes.contains_key(k(2)) {
                "conflict"
            } else {
                self.strings.insert(k(1).into(), a(2).to_vec());
                self.set_fields(k(2), &fields);
                self.set_fields(k(2), &[("consumed_at", a(16))]);
                "consumed"
            };
            self.strings.insert(k(4).into(), a(19).to_vec());
            return tag(outcome);
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
                let count: usize = text(&arguments[2]).parse().or_panic("test value");
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

pub struct Redis {
    listener: Listener,
    pub state: Arc<Mutex<StoreState>>,
    pub lose_claim_reply: Arc<AtomicBool>,
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
            let auth = command(&mut stream).or_panic("test value");
            assert_eq!(
                auth,
                [
                    b"AUTH".to_vec(),
                    b"relay".to_vec(),
                    b"repair-secret".to_vec()
                ]
            );
            stream.write_all(b"+OK\r\n").or_panic("test value");
            while let Ok(arguments) = command(&mut stream) {
                let reply = {
                    let mut state = shared.lock().or_panic("test value");
                    let reply = state.exchange(&arguments);
                    let mut file =
                        fs::File::create(snapshot.with_extension("tmp")).or_panic("test value");
                    file.write_all(&serde_json::to_vec(&*state).or_panic("test value"))
                        .or_panic("test value");
                    file.sync_all().or_panic("test value");
                    fs::rename(snapshot.with_extension("tmp"), &snapshot).or_panic("test value");
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
    /// Returns the stored relay record for the supplied key.
    ///
    /// # Panics
    /// Panics if the fixture state lock is poisoned.
    #[must_use]
    pub fn record(&self, key: &str) -> Option<Vec<u8>> {
        self.state
            .lock()
            .or_panic("test value")
            .strings
            .get(key)
            .cloned()
    }
}

#[derive(Default)]
pub struct NodeState {
    attempts: Vec<Vec<u8>>,
    accepted: BTreeSet<Vec<u8>>,
    methods: Vec<String>,
    connects: Vec<String>,
}
pub struct Node {
    listener: Listener,
    pub state: Arc<Mutex<NodeState>>,
    pub fault: Arc<AtomicU8>,
    pub hide_receipt: Arc<AtomicBool>,
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
            let connect = line(&mut stream).or_panic("test value");
            let connect = text(&connect).to_owned();
            assert_eq!(connect, "CONNECT router.railway.internal:9443 HTTP/1.1");
            shared.lock().or_panic("test value").connects.push(connect);
            while !line(&mut stream).or_panic("test value").is_empty() {}
            stream
                .write_all(b"HTTP/1.1 200 Connection established\r\n\r\n")
                .or_panic("test value");
            let request = http::read_request(&mut stream, 2 * 1024 * 1024).or_panic("test value");
            assert_eq!(request.method, "POST");
            assert_eq!(
                request.headers.get("host").map(String::as_str),
                Some("router.railway.internal:9443")
            );
            let rpc: Value = serde_json::from_slice(&request.body).or_panic("test value");
            let method = rpc["method"].as_str().or_panic("test value");
            let result = {
                let mut state = shared.lock().or_panic("test value");
                state.methods.push(method.into());
                match method {
                    "px_getReceiptKeys" => {
                        assert_eq!(rpc["params"], json!([]));
                        json!({"keys": [case["pinnedKey"]]})
                    }
                    "eth_sendRawTransaction" => {
                        let sent = bytes(rpc["params"][0].as_str().or_panic("test value"))
                            .or_panic("test value");
                        assert_eq!(
                            sent, raw,
                            "only the payer's exact recorded signed bytes may be sent"
                        );
                        assert_eq!(
                            format!("0x{}", hex(&transaction_hash(&sent))),
                            case["receiptJson"]["txHash"]
                        );
                        state.attempts.push(sent.clone());
                        let mode = faulted.swap(0, Ordering::SeqCst);
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
                        assert_eq!(rpc["params"], json!([case["receiptJson"]["txHash"]]));
                        if hidden.swap(false, Ordering::SeqCst) || state.accepted.is_empty() {
                            Value::Null
                        } else {
                            case["receiptJson"].clone()
                        }
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
                    .or_panic("test value"),
                },
            )
            .or_panic("test value");
        })?;
        Ok(Self {
            listener,
            state,
            fault,
            hide_receipt,
        })
    }
    pub fn reset(&self) {
        *self.state.lock().or_panic("test value") = NodeState::default();
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
    let mut manifest: Value = serde_json::from_slice(include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../deploy/gateway/config.example.json"
    )))?;
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
    Ok(Identity::from_pkcs12(&identity, "repair-password")?)
}

fn configured_environment(dir: &Path, redis: &Redis) -> Environment {
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
        ("TAP_CLOCK_SKEW_SECONDS", "60"),
    ] {
        environment.set(&format!("LAYERX_INTEROP_{name}"), value);
    }
    environment.set(
        "LAYERX_INTEROP_REDIS_URL",
        format!("rediss://localhost:{}", redis.listener.address.port()),
    );
    environment
}

fn gateway(dir: &Path, redis: &Redis, node: &Node) -> TestResult<Config> {
    let mut environment = configured_environment(dir, redis);
    environment.set("LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK", "1");
    environment.set("LAYERX_INTEROP_SEQUENCER_URL", PRIVATE_URL);
    environment.set("ALL_PROXY", format!("http://{}", node.listener.address));
    environment.set("NO_PROXY", "");
    Ok(config::load()?)
}

fn mounted(
    config: Config,
    body: &[u8],
    authorization: Option<&str>,
) -> TestResult<(Config, u16, Value)> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let worker = thread::spawn(move || -> TestResult<Config> {
        let (mut stream, _) = listener.accept()?;
        stream.set_read_timeout(Some(TIMEOUT))?;
        stream.set_write_timeout(Some(TIMEOUT))?;
        let request = http::read_request(&mut stream, 512 * 1024)?;
        let response = server::route(&config, &request);
        http::write_response(&mut stream, &response)?;
        Ok(config)
    });
    let mut stream = TcpStream::connect_timeout(&address, TIMEOUT)?;
    stream.set_read_timeout(Some(TIMEOUT))?;
    write!(stream, "POST {PAYMENT_PATH} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n", body.len())?;
    if let Some(authorization) = authorization {
        write!(stream, "Authorization: {authorization}\r\n")?;
    }
    stream.write_all(b"\r\n")?;
    stream.write_all(body)?;
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

fn visa_payment(case: &Value) -> TestResult<Vec<u8>> {
    let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
    let parameters = format!("(\"@authority\" \"@path\");created={};keyid=\"layerx-beta-tap-key-1\";alg=\"Ed25519\";expires={};nonce=\"repair-session\";tag=\"agent-payer-auth\"", now - 1, now + 300);
    let base = format!(
        "\"@authority\": shop.example\n\"@path\": /checkout\n\"@signature-params\": {parameters}"
    );
    let signature = STANDARD.encode(
        SigningKey::from_bytes(&[7; 32])
            .sign(base.as_bytes())
            .to_bytes(),
    );
    Ok(serde_json::to_vec(
        &json!({"authority":"shop.example", "path":"/checkout", "signature_input":format!("sig2={parameters}"),
        "signature":format!("sig2=:{signature}:"), "activity":case["rawTransaction"].as_str().ok_or("raw transaction")?.trim_start_matches("0x")}),
    )?)
}

/// Checks that a relay outcome carries the signed receipt and transaction hash.
///
/// # Errors
/// Returns an error when the outcome is not executed or receipt fields cannot be decoded.
///
/// # Panics
/// Panics when the signed receipt or transaction hash differs from the recorded case.
pub fn assert_executed(outcome: RelayOutcome, case: &Value) -> TestResult {
    let RelayOutcome::Executed(evidence) = outcome else {
        return Err("executed receipt required".into());
    };
    assert_eq!(
        evidence.signed_receipt,
        bytes(case["signedReceipt"].as_str().ok_or("signed receipt")?)?
    );
    assert_eq!(
        evidence.transaction_hash.as_slice(),
        bytes(case["receiptJson"]["txHash"].as_str().ok_or("hash")?)?
    );
    assert_eq!(evidence.logs.len(), 1);
    Ok(())
}

pub mod active_inputs {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/tests/support/active_inputs.rs"
    ));
}

static CONFIGURATION: Mutex<()> = Mutex::new(());

pub mod payment_fixture {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/tests/support/payment_fixture.rs"
    ));
}

fn payment_case() -> TestResult<Value> {
    payment_fixture::payment_case(None)
}

fn assert_active_environment(dir: &Path) {
    active_inputs::assert_inputs(dir, std::env::vars().map(|(key, _)| key));
}

fn receipt_node(case: &Value) -> TestResult<(Listener, Arc<AtomicU8>)> {
    let keys = json!({"keys": [case["pinnedKey"]]});
    let calls = Arc::new(AtomicU8::new(0));
    let observed = Arc::clone(&calls);
    let listener = Listener::start("127.0.0.1:0".parse()?, move |mut stream| {
        let request = http::read_request(&mut stream, 64 * 1024).or_panic("test value");
        let rpc: Value = serde_json::from_slice(&request.body).or_panic("test value");
        assert_eq!(request.method, "POST");
        assert_eq!(rpc["jsonrpc"], "2.0");
        assert_eq!(rpc["method"], "px_getReceiptKeys");
        assert_eq!(rpc["params"], json!([]));
        observed.fetch_add(1, Ordering::SeqCst);
        http::write_response(
            &mut stream,
            &OutgoingResponse {
                status: 200,
                content_type: "application/json".into(),
                headers: Vec::new(),
                retry_after: None,
                body: serde_json::to_vec(&json!({"jsonrpc":"2.0", "id":rpc["id"], "result":keys}))
                    .or_panic("test value"),
            },
        )
        .or_panic("test value");
    })?;
    Ok((listener, calls))
}

fn mounted_get(config: Config, path: &str) -> TestResult<(Config, u16, Value)> {
    let listener = TcpListener::bind(config.listen)?;
    let address = listener.local_addr()?;
    let worker = thread::spawn(move || -> TestResult<Config> {
        let (mut stream, _) = listener.accept()?;
        stream.set_read_timeout(Some(TIMEOUT))?;
        stream.set_write_timeout(Some(TIMEOUT))?;
        let request = http::read_request(&mut stream, 64 * 1024)?;
        http::write_response(&mut stream, &server::route(&config, &request))?;
        Ok(config)
    });
    let mut stream = TcpStream::connect_timeout(&address, TIMEOUT)?;
    stream.set_read_timeout(Some(TIMEOUT))?;
    stream.set_write_timeout(Some(TIMEOUT))?;
    write!(
        stream,
        "GET {path} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
    )?;
    let mut response = String::new();
    stream.read_to_string(&mut response)?;
    let (head, body) = response.split_once("\r\n\r\n").ok_or("HTTP response")?;
    let status = head.split_whitespace().nth(1).ok_or("status")?.parse()?;
    Ok((
        worker.join().map_err(|_| "gateway listener panicked")??,
        status,
        serde_json::from_str(body)?,
    ))
}

/// Checks receipt-key pinning and liveness at gateway startup.
///
/// # Errors
/// Returns an error when fixture setup, configuration, or transport fails.
///
/// # Panics
/// Panics when the observed gateway state differs from the recorded case.
pub fn minimal_node_receipt_pin_startup() -> TestResult {
    let _serial = CONFIGURATION.lock().or_panic("test value");
    let dir = Directory::new()?;
    let case = payment_case()?;
    let identity = configure(&dir.0, &case)?;
    let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let (node, calls) = receipt_node(&case)?;
    let config = {
        let mut environment = configured_environment(&dir.0, &redis);
        environment.set(
            "LAYERX_INTEROP_SEQUENCER_URL",
            format!("http://{}", node.address),
        );
        assert_active_environment(&dir.0);
        config::load()?
    };
    assert!(matches!(config.listener, config::Listener::Plain));
    assert_eq!(config.receipt_keys.len(), 1);
    assert_eq!(config.readiness_chain_id, 125);
    assert_eq!(calls.load(Ordering::SeqCst), 1);
    assert_eq!(
        config
            .sequencer
            .receipt_keys()
            .map_err(|error| format!("receipt keys: {error:?}"))?
            .len(),
        1
    );
    assert_eq!(calls.load(Ordering::SeqCst), 2);
    let (_, status, live) = mounted_get(config, "/livez")?;
    assert_eq!(status, 200, "{live}");
    assert_eq!(live["status"], "live");
    assert_eq!(live["service"], "layerx-interop-gateway");
    Ok(())
}

/// Checks that startup requires a reachable receipt-key source.
///
/// # Errors
/// Returns an error when fixture setup or transport fails.
///
/// # Panics
/// Panics when the gateway accepts an unavailable receipt-key source.
pub fn unavailable_node_refuses_minimal_startup() -> TestResult {
    let _serial = CONFIGURATION.lock().or_panic("test value");
    let dir = Directory::new()?;
    let case = payment_case()?;
    let identity = configure(&dir.0, &case)?;
    let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let reserved = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = reserved.local_addr()?;
    let mut environment = configured_environment(&dir.0, &redis);
    environment.set("LAYERX_INTEROP_SEQUENCER_URL", format!("http://{endpoint}"));
    assert_active_environment(&dir.0);
    drop(reserved);
    let Err(error) = config::load() else {
        return Err("unavailable node accepted at startup".into());
    };
    assert_eq!(error, "sequencer px_getReceiptKeys failed: Unavailable");
    Ok(())
}

/// Checks payment recovery through the mounted private-network route.
///
/// # Errors
/// Returns an error when fixture setup or transport fails.
///
/// # Panics
/// Panics when the recovered payment differs from the recorded case.
pub fn private_network_payment_recovers_through_mounted_route() -> TestResult {
    let _serial = CONFIGURATION.lock().or_panic("test value");
    let case = payment_case()?;
    let raw = bytes(case["rawTransaction"].as_str().ok_or("raw transaction")?)?;
    let (transaction, signature) = decode_signed(&raw)?;
    assert_eq!(transaction.chain_id, 125);
    let sender = recover_sender(&transaction, &signature)?;
    assert_ne!(sender, [0; 20]);
    let dir = Directory::new()?;
    let identity = configure(&dir.0, &case)?;
    let node = Node::start(&case)?;
    let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let config = gateway(&dir.0, &redis, &node)?;
    let issued = IssuedKey::generate()?;
    let record = KeyRecord {
        key_id: issued.id().into(),
        principal_digest: "33".repeat(32),
        salt: "private-route-salt".into(),
        secret_digest: gateway_digest(&[
            b"gateway-key-v1",
            b"private-route-salt",
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
        .issue_key(&record, "private route test issuance")?;
    let authorization = format!("LayerX-Key {}:{}", issued.id(), issued.secret());
    let body = visa_payment(&case)?;
    let (config, status, unauthorized) = mounted(config, &body, None)?;
    assert_eq!(status, 401, "{unauthorized}");
    assert!(node.state.lock().or_panic("test value").attempts.is_empty());
    let (config, status, response) = mounted(config, &body, Some(&authorization))?;
    assert_eq!(status, 200, "{response}");
    assert_eq!(response["result"]["state"], "receipt-verified");
    let operation = response["operation"]
        .as_str()
        .ok_or("operation")?
        .to_owned();
    let stored = config
        .store
        .operation(&operation)?
        .ok_or("completed operation")?;
    assert_eq!(stored.state, "completed");
    assert_eq!(
        stored.receipt,
        case["signedReceipt"]
            .as_str()
            .ok_or("signed receipt")?
            .trim_start_matches("0x")
    );
    let redis_address = redis.listener.address;
    drop(config);
    drop(redis);
    let redis = Redis::start(&dir.0, &identity, redis_address)?;
    let config = gateway(&dir.0, &redis, &node)?;
    let recovered = config
        .store
        .operation(&operation)?
        .ok_or("recovered operation")?;
    assert_eq!(recovered.state, stored.state);
    assert_eq!(recovered.receipt, stored.receipt);
    assert_eq!(recovered.response, stored.response);
    let (_, status, replay) = mounted(config, &body, Some(&authorization))?;
    assert_eq!(status, 200, "{replay}");
    assert_eq!(replay["operation"], response["operation"]);
    assert_eq!(replay["result"], response["result"]);
    let state = node.state.lock().or_panic("test value");
    assert_eq!(state.attempts, vec![raw.clone()]);
    assert_eq!(state.accepted, BTreeSet::from([raw]));
    assert!(state
        .methods
        .iter()
        .any(|method| method == "eth_sendRawTransaction"));
    assert!(state
        .methods
        .iter()
        .any(|method| method == "pax_sequencerReceipt"));
    assert_eq!(
        state
            .methods
            .iter()
            .filter(|method| *method == "px_getReceiptKeys")
            .count(),
        2
    );
    assert!(state
        .connects
        .iter()
        .all(|target| target == "CONNECT router.railway.internal:9443 HTTP/1.1"));
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
