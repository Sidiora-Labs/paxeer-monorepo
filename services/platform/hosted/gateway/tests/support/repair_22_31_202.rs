use layerx_platform_gateway::http::{self, IncomingRequest, OutgoingResponse};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::Value;
use std::collections::{BTreeMap, BTreeSet};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

pub fn fixture(name: &str) -> Value {
    let source = match name {
        "node" => include_str!("../fixtures/repair_22_31_202/node-committed.json"),
        "consensus" => include_str!("../fixtures/repair_22_31_202/consensus.json"),
        "blocks" => include_str!("../fixtures/repair_22_31_202/indexer-blocks.json"),
        "identity" => include_str!("../fixtures/repair_22_31_202/identity.json"),
        "faucet" => include_str!("../fixtures/repair_22_31_202/faucet.json"),
        "views" => include_str!("../fixtures/repair_22_31_202/node-views.json"),
        "scripts" => include_str!("../fixtures/repair_22_31_202/redis-scripts.json"),
        _ => panic!("unknown committed transcript: {name}"),
    };
    serde_json::from_str(source).expect("committed transcript JSON")
}

pub fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}
pub fn unhex(value: &str) -> Vec<u8> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    assert!(value.is_ascii() && value.len() % 2 == 0);
    (0..value.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&value[i..i + 2], 16).expect("hex"))
        .collect()
}

pub fn json_response(status: u16, body: &Value) -> OutgoingResponse {
    OutgoingResponse {
        status,
        content_type: "application/json".into(),
        headers: Vec::new(),
        body: serde_json::to_vec(body).expect("response JSON"),
        retry_after: None,
    }
}

/// Loopback peers own their listeners for their whole lifetime, including startup.
/// Worker failures are joined and surfaced instead of disappearing in detached threads.
pub struct Peer {
    pub address: SocketAddr,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}
impl Peer {
    pub fn start(handler: impl Fn(TcpStream) + Send + Sync + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback listener");
        let address = listener.local_addr().expect("listener address");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let handler = Arc::new(handler);
        let thread = thread::spawn(move || {
            let mut workers = Vec::new();
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        tcp.set_read_timeout(Some(Duration::from_secs(3)))
                            .expect("read timeout");
                        tcp.set_write_timeout(Some(Duration::from_secs(3)))
                            .expect("write timeout");
                        tcp.set_nodelay(true).expect("TCP nodelay");
                        let handler = Arc::clone(&handler);
                        workers.push(thread::spawn(move || handler(tcp)));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2))
                    }
                    Err(error) => panic!("loopback accept: {error}"),
                }
            }
            for worker in workers {
                worker.join().expect("loopback peer request succeeded");
            }
        });
        Self {
            address,
            stop,
            thread: Some(thread),
        }
    }
    pub fn https(
        tls: Arc<ServerConfig>,
        handler: impl Fn(IncomingRequest) -> OutgoingResponse + Send + Sync + 'static,
    ) -> Self {
        Self::start(move |tcp| {
            let connection = ServerConnection::new(Arc::clone(&tls)).expect("peer TLS");
            let mut stream = StreamOwned::new(connection, tcp);
            let request = http::read_request(&mut stream, 8 * 1024 * 1024).expect("peer request");
            http::write_response(&mut stream, &handler(request)).expect("peer response");
            stream.conn.send_close_notify();
            stream.flush().expect("TLS close");
        })
    }
    pub fn url(&self) -> String {
        format!("https://localhost:{}", self.address.port())
    }
}
impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            let result = thread.join();
            if !std::thread::panicking() {
                result.expect("loopback peer succeeded");
            }
        }
    }
}

pub fn request(
    address: SocketAddr,
    method: &str,
    route: &str,
    headers: &[(&str, &str)],
    body: &Value,
) -> (u16, Value) {
    let bytes = if method == "GET" {
        Vec::new()
    } else {
        serde_json::to_vec(body).expect("request")
    };
    let mut stream = TcpStream::connect(address).expect("gateway TCP");
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("response timeout");
    write!(stream, "{method} {route} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n", bytes.len()).expect("headers");
    for (key, value) in headers {
        write!(stream, "{key}: {value}\r\n").expect("header");
    }
    stream.write_all(b"\r\n").expect("end headers");
    stream.write_all(&bytes).expect("body");
    let mut answer = Vec::new();
    stream.read_to_end(&mut answer).expect("gateway response");
    let split = answer
        .windows(4)
        .position(|s| s == b"\r\n\r\n")
        .expect("HTTP response");
    let status = std::str::from_utf8(&answer[..split])
        .expect("headers")
        .split_whitespace()
        .nth(1)
        .expect("status")
        .parse()
        .expect("status number");
    (
        status,
        serde_json::from_slice(&answer[split + 4..]).expect("gateway response JSON"),
    )
}

/// An in-memory Redis peer: the gateway still uses its production TLS/RESP
/// client, audit chaining, credential hashing, quotas and durable watermark code.
/// Only the three recorded Lua contracts and watermark command are admitted.
#[derive(Default)]
pub struct RedisMemory {
    pub hashes: BTreeMap<String, BTreeMap<String, String>>,
    strings: BTreeMap<String, String>,
    sets: BTreeMap<String, BTreeSet<String>>,
    pub commands: Vec<String>,
}

fn bulk(value: Option<&str>) -> Vec<u8> {
    value.map_or_else(
        || b"$-1\r\n".to_vec(),
        |v| format!("${}\r\n{v}\r\n", v.len()).into_bytes(),
    )
}
fn array(values: impl IntoIterator<Item = String>) -> Vec<u8> {
    let values: Vec<_> = values.into_iter().collect();
    let mut bytes = format!("*{}\r\n", values.len()).into_bytes();
    for value in values {
        bytes.extend(bulk(Some(&value)));
    }
    bytes
}
fn tag(value: &str) -> Vec<u8> {
    array([value.to_owned()])
}
fn read_command(stream: &mut impl BufRead) -> Option<Vec<String>> {
    let mut line = String::new();
    if stream.read_line(&mut line).ok()? == 0 {
        return None;
    }
    let length: usize = line.strip_prefix('*')?.trim_end().parse().ok()?;
    assert!((1..=128).contains(&length), "bounded Redis command");
    let mut command = Vec::with_capacity(length);
    for _ in 0..length {
        line.clear();
        stream.read_line(&mut line).ok()?;
        let length: usize = line.strip_prefix('$')?.trim_end().parse().ok()?;
        assert!(length <= 65536);
        let mut bytes = vec![0; length + 2];
        stream.read_exact(&mut bytes).ok()?;
        assert_eq!(&bytes[length..], b"\r\n");
        command.push(String::from_utf8(bytes[..length].to_vec()).expect("Redis UTF-8"));
    }
    Some(command)
}
impl RedisMemory {
    pub fn serve(memory: Arc<Mutex<Self>>, tls: Arc<ServerConfig>) -> Peer {
        Peer::start(move |tcp| {
            let connection = ServerConnection::new(Arc::clone(&tls)).expect("Redis TLS");
            let mut stream = BufReader::new(StreamOwned::new(connection, tcp));
            while let Some(command) = read_command(&mut stream) {
                let answer = memory.lock().expect("Redis peer store").execute(&command);
                stream.get_mut().write_all(&answer).expect("RESP response");
                stream.get_mut().flush().expect("RESP flush");
            }
        })
    }
    fn field(&self, key: &str, field: &str) -> Option<&str> {
        self.hashes.get(key)?.get(field).map(String::as_str)
    }
    fn record(&mut self, key: &str, fields: &[&str], values: &[String]) {
        assert_eq!(fields.len(), values.len());
        self.hashes.insert(
            key.into(),
            fields
                .iter()
                .zip(values)
                .map(|(k, v)| ((*k).into(), v.clone()))
                .chain([("disabled".into(), "0".into())])
                .collect(),
        );
    }
    fn audit(&mut self, key: &str, previous: &str, chain: &str, event: &str) -> bool {
        if self.strings.get(key).map(String::as_str).unwrap_or("") != previous {
            return false;
        }
        use sha2::{Digest, Sha256};
        let mut hash = Sha256::new();
        for part in [previous, event] {
            hash.update((part.len() as u64).to_be_bytes());
            hash.update(part.as_bytes());
        }
        assert_eq!(chain, hex(&hash.finalize()), "real gateway audit chain");
        self.strings.insert(key.into(), chain.into());
        true
    }
    fn execute(&mut self, command: &[String]) -> Vec<u8> {
        self.commands.push(command[0].clone());
        match command[0].as_str() {
            "AUTH" => {
                assert_eq!(command, &["AUTH", "gateway", "gateway-secret"]);
                b"+OK\r\n".to_vec()
            }
            "PING" => b"+PONG\r\n".to_vec(),
            "GET" => bulk(self.strings.get(&command[1]).map(String::as_str)),
            "HGETALL" => array(
                self.hashes
                    .get(&command[1])
                    .into_iter()
                    .flat_map(|fields| fields.iter().flat_map(|(k, v)| [k.clone(), v.clone()])),
            ),
            "SMEMBERS" => array(
                self.sets
                    .get(&command[1])
                    .into_iter()
                    .flat_map(|s| s.iter().cloned()),
            ),
            "SET" => {
                assert_eq!(command.len(), 3);
                self.strings.insert(command[1].clone(), command[2].clone());
                b"+OK\r\n".to_vec()
            }
            "EVAL" => self.eval(command),
            other => panic!("Redis peer received undeclared command {other}"),
        }
    }
    fn eval(&mut self, command: &[String]) -> Vec<u8> {
        let scripts = fixture("scripts");
        let script = command[1].as_str();
        let count: usize = command[2].parse().expect("script key count");
        let keys = &command[3..3 + count];
        let args = &command[3 + count..];
        if script == scripts["ISSUE_SCRIPT"].as_str().expect("issue script") {
            assert_eq!((count, args.len()), (4, 13));
            if self.hashes.contains_key(&keys[0]) {
                return tag("conflict");
            }
            if self.sets.get(&keys[1]).map_or(0, BTreeSet::len)
                >= args[9].parse::<usize>().expect("key limit")
            {
                return tag("limit");
            }
            if !self.audit(&keys[3], &args[11], &args[12], &args[10]) {
                return tag("audit_retry");
            }
            self.record(
                &keys[0],
                &[
                    "principal",
                    "salt",
                    "secret_digest",
                    "signer_public_key",
                    "scopes",
                    "quota_requests",
                    "quota_window_seconds",
                    "epoch",
                ],
                &args[1..9],
            );
            self.sets
                .entry(keys[1].clone())
                .or_default()
                .insert(args[0].clone());
            return tag("issued");
        }
        if script == scripts["ROTATE_SCRIPT"].as_str().expect("rotation script") {
            assert_eq!((count, args.len()), (5, 14));
            if self.field(&keys[0], "principal") != Some(&args[0])
                || self.field(&keys[0], "epoch") != Some(&args[1])
                || self.field(&keys[0], "disabled") != Some("0")
                || self.hashes.contains_key(&keys[1])
            {
                return tag("conflict");
            }
            if self.sets.get(&keys[2]).map_or(0, BTreeSet::len)
                >= args[13].parse::<usize>().expect("key limit")
            {
                return tag("limit");
            }
            if !self.audit(&keys[4], &args[11], &args[12], &args[10]) {
                return tag("audit_retry");
            }
            let old = self.hashes.get_mut(&keys[0]).expect("old key");
            old.insert("disabled".into(), "1".into());
            old.insert(
                "epoch".into(),
                (args[1].parse::<u64>().expect("epoch") + 1).to_string(),
            );
            let values: Vec<_> = [args[0].clone()]
                .into_iter()
                .chain(args[3..10].iter().cloned())
                .collect();
            self.record(
                &keys[1],
                &[
                    "principal",
                    "salt",
                    "secret_digest",
                    "signer_public_key",
                    "scopes",
                    "quota_requests",
                    "quota_window_seconds",
                    "epoch",
                ],
                &values,
            );
            self.sets
                .entry(keys[2].clone())
                .or_default()
                .insert(args[2].clone());
            return tag("rotated");
        }
        if script == scripts["CONSUME_SCRIPT"].as_str().expect("quota script") {
            assert_eq!((count, args.len()), (4, 6));
            if self.field(&keys[0], "disabled") != Some("0")
                || self.field(&keys[0], "epoch") != Some(&args[0])
            {
                return tag("revoked");
            }
            if !self.audit(&keys[3], &args[4], &args[5], &args[3]) {
                return tag("audit_retry");
            }
            let used = self
                .strings
                .get(&keys[1])
                .map_or(0, |s| s.parse::<u64>().expect("quota"));
            if used >= args[1].parse::<u64>().expect("limit") {
                return tag("rate_limited");
            }
            self.strings.insert(keys[1].clone(), (used + 1).to_string());
            return tag("consumed");
        }
        if script == scripts["OBSERVE_SCRIPT"].as_str().expect("state script") {
            assert_eq!((count, args.len()), (1, 2));
            if let Some(old) = self.field(&keys[0], "height") {
                if old > args[0].as_str()
                    || (old == args[0] && self.field(&keys[0], "hash") != Some(&args[1]))
                {
                    return b":0\r\n".to_vec();
                }
            }
            self.hashes.insert(
                keys[0].clone(),
                [
                    ("height".into(), args[0].clone()),
                    ("hash".into(), args[1].clone()),
                ]
                .into(),
            );
            return b":1\r\n".to_vec();
        }
        panic!("Redis peer received a script absent from the committed transcript");
    }
    pub fn has_faucet_claim(&self) -> bool {
        self.strings.keys().any(|k| k.starts_with("faucet:claim:"))
    }
}

/// Validate every keyword used by the two closed history schemas, failing on
/// unsupported keywords instead of silently accepting a schema extension.
pub fn matches_schema(value: &Value, schema: &Value) -> bool {
    for key in schema.as_object().expect("schema object").keys() {
        assert!(
            matches!(
                key.as_str(),
                "description"
                    | "type"
                    | "required"
                    | "properties"
                    | "additionalProperties"
                    | "items"
                    | "maxItems"
                    | "minimum"
                    | "maximum"
                    | "pattern"
                    | "enum"
                    | "const"
            ),
            "unsupported schema keyword {key}"
        );
    }
    let has_type = |kind: &str| match kind {
        "object" => value.is_object(),
        "array" => value.is_array(),
        "string" => value.is_string(),
        "integer" => value.is_i64() || value.is_u64(),
        "boolean" => value.is_boolean(),
        "null" => value.is_null(),
        _ => panic!("unknown schema type {kind}"),
    };
    if let Some(types) = schema.get("type") {
        let valid = if let Some(types) = types.as_array() {
            types.iter().any(|k| has_type(k.as_str().expect("type")))
        } else {
            has_type(types.as_str().expect("type"))
        };
        if !valid {
            return false;
        }
    }
    if schema
        .get("enum")
        .is_some_and(|v| !v.as_array().expect("enum").contains(value))
        || schema.get("const").is_some_and(|v| v != value)
    {
        return false;
    }
    if let Some(object) = value.as_object() {
        if schema.get("required").is_some_and(|v| {
            v.as_array()
                .expect("required")
                .iter()
                .any(|k| !object.contains_key(k.as_str().expect("property")))
        }) {
            return false;
        }
        for (key, value) in object {
            if let Some(property) = schema.get("properties").and_then(|p| p.get(key)) {
                if !matches_schema(value, property) {
                    return false;
                }
            } else if schema["additionalProperties"] == false {
                return false;
            }
        }
    }
    if let Some(items) = value.as_array() {
        if schema["maxItems"]
            .as_u64()
            .is_some_and(|n| items.len() as u64 > n)
        {
            return false;
        }
        if let Some(item_schema) = schema.get("items") {
            if !items.iter().all(|v| matches_schema(v, item_schema)) {
                return false;
            }
        }
    }
    if let Some(number) = value.as_f64() {
        if schema["minimum"].as_f64().is_some_and(|n| number < n)
            || schema["maximum"].as_f64().is_some_and(|n| number > n)
        {
            return false;
        }
    }
    if let (Some(text), Some(pattern)) = (value.as_str(), schema["pattern"].as_str()) {
        let hex = |s: &str, length| {
            s.len() == length
                && s.bytes()
                    .all(|c| c.is_ascii_digit() || (b'a'..=b'f').contains(&c))
        };
        let canonical = !text.is_empty()
            && text.bytes().all(|b| b.is_ascii_digit())
            && (text == "0" || !text.starts_with('0'));
        if !match pattern {
            "^0x[0-9a-f]{40}$" => text.strip_prefix("0x").is_some_and(|s| hex(s, 40)),
            "^0x[0-9a-f]{64}$" => text.strip_prefix("0x").is_some_and(|s| hex(s, 64)),
            "^[0-9a-f]{64}$" => hex(text, 64),
            "^[0-9a-f]{128}$" => hex(text, 128),
            "^(0|[1-9][0-9]*)$" => canonical,
            "^[1-9][0-9]*$" => canonical && text != "0",
            "^[a-z0-9_]{1,64}$" => {
                (1..=64).contains(&text.len())
                    && text
                        .bytes()
                        .all(|b| b.is_ascii_lowercase() || b.is_ascii_digit() || b == b'_')
            }
            _ => panic!("unsupported schema pattern {pattern}"),
        } {
            return false;
        }
    }
    true
}
