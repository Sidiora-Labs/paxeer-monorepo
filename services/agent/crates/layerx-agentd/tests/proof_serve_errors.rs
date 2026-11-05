//! Proves: route_request / native_read_response -> HttpPool / response / TCP HTTP -> proof::decode_response.
//!
//! This file is also the host's shared HTTP module: the proof compiles the same
//! listener, request parser, error producers and writer without the host-only MCP
//! dependency graph. No transport is replaced. The error anchor has no durable
//! state; each assertion makes two independent connections. Native error mapping
//! takes the real NativeReadError variants, including ResultTooLarge, rather than
//! constructing wire error messages. Malformed-result checks corrupt bytes of an
//! actual producer response at the consumer's decoding boundary.
//!
//! No runtime defect is repaired: this extracts the existing path without changing
//! its error codes or status semantics.

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use layerx_agentd::boot::readiness_gate;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::read::{
    CommittedProgramBalanceFreshness, NativeReadError, NativeReadRoute, ProgramBalanceRead,
    ProgramBalanceReadRoute,
};
use layerx_programs::{hex, ProgramId, ProgramLifecycle};

pub(crate) const HEADER_LIMIT: usize = 16 * 1024;
const BODY_LIMIT: usize = 0;
const PROGRAM_WORKERS: usize = 8;
const PROGRAM_QUEUE: usize = 16;
pub(crate) const REQUEST_DEADLINE: Duration = Duration::from_secs(5);
const IO_CEILING: Duration = Duration::from_secs(10);
pub(crate) const SUPERVISION_INTERVAL: Duration = Duration::from_millis(25);

pub(crate) struct DeadlineStream {
    pub(crate) socket: TcpStream,
    pub(crate) expires: Instant,
}

impl DeadlineStream {
    fn new(socket: TcpStream) -> Self {
        Self {
            socket,
            expires: Instant::now() + REQUEST_DEADLINE,
        }
    }

    pub(crate) fn remaining(&self) -> std::io::Result<Duration> {
        let remaining = self.expires.saturating_duration_since(Instant::now());
        if remaining.is_zero() {
            Err(std::io::Error::new(
                std::io::ErrorKind::TimedOut,
                "request deadline elapsed",
            ))
        } else {
            Ok(remaining.min(IO_CEILING))
        }
    }
}

impl Read for DeadlineStream {
    fn read(&mut self, bytes: &mut [u8]) -> std::io::Result<usize> {
        self.socket.set_read_timeout(Some(self.remaining()?))?;
        self.socket.read(bytes)
    }
}

impl Write for DeadlineStream {
    fn write(&mut self, bytes: &[u8]) -> std::io::Result<usize> {
        self.socket.set_write_timeout(Some(self.remaining()?))?;
        self.socket.write(bytes)
    }

    fn flush(&mut self) -> std::io::Result<()> {
        self.socket.set_write_timeout(Some(self.remaining()?))?;
        self.socket.flush()
    }
}

struct HttpAdmissions {
    stopping: bool,
    busy: Vec<bool>,
    pending: Vec<Option<DeadlineStream>>,
    active: Vec<Option<TcpStream>>,
    queue: std::collections::VecDeque<DeadlineStream>,
}

pub(crate) struct HttpPool {
    admissions: Arc<(Mutex<HttpAdmissions>, std::sync::Condvar)>,
    threads: Vec<thread::JoinHandle<()>>,
}

impl HttpPool {
    pub(crate) fn start<F>(listener: TcpListener, handle: F) -> Result<Self, String>
    where
        F: Fn(DeadlineStream) + Send + Sync + 'static,
    {
        listener
            .set_nonblocking(true)
            .map_err(|error| error.to_string())?;
        let handle = Arc::new(handle);
        let mut pool = Self {
            admissions: Arc::new((
                Mutex::new(HttpAdmissions {
                    stopping: false,
                    busy: vec![false; PROGRAM_WORKERS],
                    pending: (0..PROGRAM_WORKERS).map(|_| None).collect(),
                    active: (0..PROGRAM_WORKERS).map(|_| None).collect(),
                    queue: std::collections::VecDeque::with_capacity(PROGRAM_QUEUE),
                }),
                std::sync::Condvar::new(),
            )),
            threads: Vec::with_capacity(PROGRAM_WORKERS + 1),
        };
        for index in 0..PROGRAM_WORKERS {
            let admissions = Arc::clone(&pool.admissions);
            let handle = Arc::clone(&handle);
            let worker = thread::Builder::new()
                .name(format!("layerx-http-{index}"))
                .spawn(move || loop {
                    let stream = {
                        let Ok(mut state) = admissions.0.lock() else {
                            return;
                        };
                        while state.pending[index].is_none() && !state.stopping {
                            state = match admissions.1.wait(state) {
                                Ok(state) => state,
                                Err(_) => return,
                            };
                        }
                        if state.stopping {
                            return;
                        }
                        let Some(stream) = state.pending[index].take() else {
                            return;
                        };
                        match stream.socket.try_clone() {
                            Ok(socket) => state.active[index] = Some(socket),
                            Err(_) => {
                                state.pending[index] = state.queue.pop_front();
                                state.busy[index] = state.pending[index].is_some();
                                continue;
                            }
                        }
                        stream
                    };
                    if stream.remaining().is_ok() {
                        handle(stream);
                    }
                    let Ok(mut state) = admissions.0.lock() else {
                        return;
                    };
                    state.active[index] = None;
                    state.pending[index] = state.queue.pop_front();
                    state.busy[index] = state.pending[index].is_some();
                })
                .map_err(|error| format!("HTTP worker failed: {error}"))?;
            pool.threads.push(worker);
        }
        let admissions = Arc::clone(&pool.admissions);
        pool.threads.push(
            thread::Builder::new()
                .name("layerx-http-accept".to_owned())
                .spawn(move || loop {
                    {
                        let Ok(mut state) = admissions.0.lock() else {
                            return;
                        };
                        if state.stopping {
                            return;
                        }
                        state.queue.retain(|stream| stream.remaining().is_ok());
                    }
                    match listener.accept() {
                        Ok((socket, _)) => {
                            let stream = DeadlineStream::new(socket);
                            let Ok(mut state) = admissions.0.lock() else {
                                return;
                            };
                            if state.stopping {
                                return;
                            }
                            if let Some(index) = state.busy.iter().position(|busy| !busy) {
                                state.busy[index] = true;
                                state.pending[index] = Some(stream);
                                admissions.1.notify_all();
                            } else if state.queue.len() < PROGRAM_QUEUE {
                                state.queue.push_back(stream);
                            }
                        }
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(SUPERVISION_INTERVAL)
                        }
                        Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                        Err(_) => return,
                    }
                })
                .map_err(|error| format!("HTTP accept thread failed: {error}"))?,
        );
        Ok(pool)
    }

    pub(crate) fn check(&self) -> Result<(), String> {
        if self.threads.iter().any(thread::JoinHandle::is_finished) {
            Err("HTTP listener or worker terminated".to_owned())
        } else {
            Ok(())
        }
    }
}

impl Drop for HttpPool {
    fn drop(&mut self) {
        if let Ok(mut state) = self.admissions.0.lock() {
            state.stopping = true;
            state.queue.clear();
            for stream in &mut state.pending {
                *stream = None;
            }
            for socket in state.active.iter().flatten() {
                let _ = socket.shutdown(std::net::Shutdown::Both);
            }
            self.admissions.1.notify_all();
        }
    }
}

fn native_read_response(result: Result<serde_json::Value, NativeReadError>) -> (u16, String) {
    let (status, body) = match result {
        Ok(value) => return (200, value.to_string()),
        Err(NativeReadError::InvalidRequest | NativeReadError::CursorMismatch) => {
            (400, "{\"error\":\"invalid_read\"}")
        }
        Err(NativeReadError::ResultTooLarge) => (413, "{\"error\":\"read_too_large\"}"),
        Err(_) => (503, "{\"error\":\"verified_read_unavailable\"}"),
    };
    (status, body.to_owned())
}

fn lifecycle(value: ProgramLifecycle) -> &'static str {
    match value {
        ProgramLifecycle::Active => "active",
        ProgramLifecycle::Deprecated => "deprecated",
        ProgramLifecycle::Tombstoned => "tombstoned",
    }
}

fn balance_json(read: &ProgramBalanceRead<CommittedProgramBalanceFreshness>) -> String {
    let accounts: Vec<_> = read
        .accounts
        .iter()
        .map(|account| {
            serde_json::json!({
                "account": hex::encode(&account.account),
                "asset": hex::encode(&account.asset),
                "amount": account.amount.to_string(),
                "frozen": account.frozen.map_or(serde_json::Value::Null, serde_json::Value::Bool),
            })
        })
        .collect();
    serde_json::json!({
        "program": hex::encode(&read.program),
        "lifecycle": lifecycle(read.lifecycle),
        "accounts": accounts,
        "freshness": {
            "height": read.freshness.height,
            "block_hash": hex::encode(&read.freshness.block_hash),
            "timestamp": read.freshness.timestamp,
        },
    })
    .to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use layerx_agentd::read::ProgramValueBalance;

    #[test]
    fn balance_json_renders_committed_freshness() {
        let mut read = ProgramBalanceRead::<CommittedProgramBalanceFreshness> {
            program: [0x11; 32],
            lifecycle: ProgramLifecycle::Active,
            accounts: vec![ProgramValueBalance {
                account: [0x22; 32],
                asset: [0x33; 32],
                amount: u128::MAX,
                frozen: Some(true),
            }],
            freshness: CommittedProgramBalanceFreshness {
                height: 12_345,
                block_hash: [0x44; 32],
                timestamp: 1_700_000_000,
            },
        };

        let rendered: serde_json::Value =
            serde_json::from_str(&balance_json(&read)).expect("balance response is valid JSON");

        assert_eq!(
            rendered,
            serde_json::json!({
                "program": "11".repeat(32),
                "lifecycle": "active",
                "accounts": [{
                    "account": "22".repeat(32),
                    "asset": "33".repeat(32),
                    "amount": "340282366920938463463374607431768211455",
                    "frozen": true,
                }],
                "freshness": {
                    "height": 12_345,
                    "block_hash": "44".repeat(32),
                    "timestamp": 1_700_000_000,
                },
            })
        );

        read.accounts[0].frozen = None;
        let rendered: serde_json::Value =
            serde_json::from_str(&balance_json(&read)).expect("balance response is valid JSON");
        assert!(rendered["accounts"][0]["frozen"].is_null());

        read.accounts[0].frozen = Some(false);
        let rendered: serde_json::Value =
            serde_json::from_str(&balance_json(&read)).expect("balance response is valid JSON");
        assert_eq!(rendered["accounts"][0]["frozen"], false);
    }
}

pub(crate) fn response<S: Write>(stream: &mut S, status: u16, body: &str) -> Result<(), String> {
    let reason = if status < 300 { "OK" } else { "Refused" };
    let header = format!(
        "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nCache-Control: no-store\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    );
    stream
        .write_all(header.as_bytes())
        .and_then(|()| stream.write_all(body.as_bytes()))
        .map_err(|error| format!("agent response failed: {error}"))
}

fn refresh_native_receipt_keys(native: &mut Option<NativeReadRoute>) -> Result<(), String> {
    if let Some(native) = native.as_mut() {
        native
            .signed_authority()
            .map_err(|error| format!("program receipt keys unavailable: {error:?}"))?;
    }
    Ok(())
}

pub(crate) enum Parsed {
    Admitted(String),
    Refused(u16, &'static str),
    Closed,
}

fn declared_body(request: &str) -> Result<usize, ()> {
    let mut length = None;
    for header in request.split("\r\n").skip(1) {
        let (name, value) = header.split_once(':').ok_or(())?;
        if name.eq_ignore_ascii_case("transfer-encoding") {
            return Err(());
        }
        if name.eq_ignore_ascii_case("content-length") {
            let value = value.trim();
            if length.is_some()
                || value.is_empty()
                || !value.bytes().all(|byte| byte.is_ascii_digit())
            {
                return Err(());
            }
            length = Some(value.parse().map_err(|_| ())?);
        }
    }
    Ok(length.unwrap_or(BODY_LIMIT))
}

/// Reads one request head under a single absolute deadline. Every read is also capped by the
/// per-I/O ceiling, so neither a silent peer nor a peer trickling partial bytes can hold a
/// connection worker beyond `REQUEST_DEADLINE`.
pub(crate) fn parse_request(stream: &mut DeadlineStream, bearer: &str) -> Parsed {
    let deadline = stream.expires;
    let mut bytes = [0_u8; HEADER_LIMIT];
    let mut length = 0_usize;
    while length < bytes.len() && !bytes[..length].windows(4).any(|value| value == b"\r\n\r\n") {
        let remaining = deadline.saturating_duration_since(Instant::now());
        if remaining.is_zero() {
            return Parsed::Refused(408, "{\"error\":\"request_timeout\"}");
        }
        match stream.read(&mut bytes[length..]) {
            Ok(0) => return Parsed::Closed,
            Ok(count) => length += count,
            Err(error)
                if matches!(
                    error.kind(),
                    std::io::ErrorKind::WouldBlock
                        | std::io::ErrorKind::TimedOut
                        | std::io::ErrorKind::Interrupted
                ) => {}
            Err(_) => return Parsed::Closed,
        }
    }
    let Some(end) = bytes[..length]
        .windows(4)
        .position(|value| value == b"\r\n\r\n")
    else {
        return Parsed::Refused(431, "{\"error\":\"headers_too_large\"}");
    };
    let Ok(request) = std::str::from_utf8(&bytes[..end]) else {
        return Parsed::Refused(400, "{\"error\":\"invalid_request\"}");
    };
    let line = request.lines().next().unwrap_or_default();
    let mut parts = line.split_ascii_whitespace();
    let method = parts.next().unwrap_or_default();
    let path = parts.next().unwrap_or_default();
    if parts.next() != Some("HTTP/1.1") || parts.next().is_some() || method != "GET" {
        return Parsed::Refused(400, "{\"error\":\"invalid_request\"}");
    }
    match declared_body(request) {
        Ok(BODY_LIMIT) => {}
        Ok(_) => return Parsed::Refused(413, "{\"error\":\"body_too_large\"}"),
        Err(()) => return Parsed::Refused(400, "{\"error\":\"invalid_request\"}"),
    }
    if length != end + 4 {
        return Parsed::Refused(413, "{\"error\":\"body_too_large\"}");
    }
    let mut credentials = request.lines().filter_map(|header| {
        let (name, value) = header.split_once(':')?;
        name.eq_ignore_ascii_case("authorization")
            .then_some(value.trim())
    });
    let authorized = credentials
        .next()
        .and_then(|value| value.strip_prefix("Bearer "))
        == Some(bearer)
        && credentials.next().is_none();
    if !authorized {
        return Parsed::Refused(401, "{\"error\":\"unauthorized\"}");
    }
    Parsed::Admitted(path.to_owned())
}

fn route_request(
    path: &str,
    node_config: &NodeRpcConfig,
    node_ca_der: &[u8],
    endpoint: &layerx_client::sequencer::SequencerEndpoint,
    route: &mut ProgramBalanceReadRoute,
    native: &mut Option<NativeReadRoute>,
) -> (u16, String) {
    let reply = |status: u16, body: &str| (status, body.to_owned());
    if path == "/healthz" {
        let (transport_ready, ready) = match NodeRpc::connect_with_ca(node_config, node_ca_der) {
            Ok(node) => (true, readiness_gate(&node).is_ok()),
            Err(_) => (false, false),
        };
        return (
            if ready { 200 } else { 503 },
            serde_json::json!({
                "ready": ready,
                "transport_ready": transport_ready,
                "verified_reads_ready": ready,
                "tenant_writes": "unknown",
            })
            .to_string(),
        );
    }
    if path.starts_with("/v1/reads/") {
        let Some(reader) = native.as_mut() else {
            return reply(404, "{\"error\":\"not_found\"}");
        };
        return native_read_response(reader.read(path));
    }
    let Some(program_text) = path
        .strip_prefix("/v1/programs/")
        .and_then(|value| value.strip_suffix("/balances"))
    else {
        return reply(404, "{\"error\":\"not_found\"}");
    };
    let program = hex::decode_digest(program_text)
        .ok()
        .and_then(|bytes| ProgramId::new(bytes).ok());
    let Some(program) = program else {
        return reply(400, "{\"error\":\"invalid_program\"}");
    };
    if refresh_native_receipt_keys(native).is_err() {
        return reply(503, "{\"error\":\"program_state_unavailable\"}");
    }
    let read = (|| {
        let node = NodeRpc::connect_with_ca(node_config, node_ca_der).map_err(|_| ())?;
        readiness_gate(&node).map_err(|_| ())?;
        let height = endpoint.block_number().map_err(|_| ())?;
        route.read(program, height).map_err(|_| ())
    })();
    match read {
        Ok(read) => (200, balance_json(&read)),
        _ => reply(503, "{\"error\":\"program_state_unavailable\"}"),
    }
}

pub(crate) fn serve_read_routes(
    listener: TcpListener,
    bearer: String,
    route: ProgramBalanceReadRoute,
    native: Option<NativeReadRoute>,
    node_config: NodeRpcConfig,
    node_ca_der: Vec<u8>,
    endpoint: layerx_client::sequencer::SequencerEndpoint,
) -> Result<HttpPool, String> {
    let shared_reads = Arc::new(Mutex::new((route, native)));
    HttpPool::start(listener, move |mut stream| {
        let answer = match parse_request(&mut stream, &bearer) {
            Parsed::Closed => return,
            Parsed::Refused(status, body) => (status, body.to_owned()),
            Parsed::Admitted(path) => loop {
                if stream.remaining().is_err() {
                    return;
                }
                match shared_reads.try_lock() {
                    Ok(mut reads) => {
                        let (route, native) = &mut *reads;
                        break route_request(
                            &path,
                            &node_config,
                            &node_ca_der,
                            &endpoint,
                            route,
                            native,
                        );
                    }
                    Err(std::sync::TryLockError::WouldBlock) => thread::sleep(SUPERVISION_INTERVAL),
                    Err(std::sync::TryLockError::Poisoned(_)) => return,
                }
            },
        };
        let _ = response(&mut stream, answer.0, &answer.1);
    })
}

#[cfg(test)]
mod proof {
    use super::*;
    use layerx_agentd::read::{LayerxdProgramBalanceReader, ProgramAuthority};
    use layerx_programs::{
        chain_proof::validators::TrustedValidator, ProtocolDeploymentVerifier, Registry,
    };
    use serde::Deserialize;
    use serde_json::{json, Value};
    use std::net::SocketAddr;
    use std::sync::atomic::{AtomicUsize, Ordering};

    const BEARER: &str = "proof-serve-errors-authenticated-client";

    #[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq)]
    #[serde(rename_all = "snake_case")]
    enum Code {
        ProgramStateUnavailable,
        InvalidProgram,
        InvalidRead,
        ReadTooLarge,
        VerifiedReadUnavailable,
        Unauthorized,
    }

    impl Code {
        fn status(&self) -> u16 {
            match self {
                Self::InvalidProgram | Self::InvalidRead => 400,
                Self::ReadTooLarge => 413,
                Self::ProgramStateUnavailable | Self::VerifiedReadUnavailable => 503,
                Self::Unauthorized => 401,
            }
        }
    }

    #[derive(Deserialize)]
    #[serde(deny_unknown_fields)]
    struct ErrorBody {
        error: Code,
    }

    #[derive(Debug, PartialEq)]
    enum Observed {
        Value(Value),
        Refused(Code),
    }

    fn decode_response(bytes: &[u8]) -> Result<Observed, String> {
        let raw = std::str::from_utf8(bytes).map_err(|error| error.to_string())?;
        let (head, body) = raw.split_once("\r\n\r\n").ok_or("missing HTTP head")?;
        let mut lines = head.split("\r\n");
        let status_line = lines.next().ok_or("missing status")?;
        let mut parts = status_line.split_ascii_whitespace();
        if parts.next() != Some("HTTP/1.1") {
            return Err("invalid HTTP version".into());
        }
        let status: u16 = parts
            .next()
            .ok_or("missing status code")?
            .parse::<u16>()
            .map_err(|e| e.to_string())?;
        let mut length = None;
        let mut content_type = false;
        let mut no_store = false;
        let mut close = false;
        for line in lines {
            let (name, value) = line.split_once(':').ok_or("invalid header")?;
            match name.to_ascii_lowercase().as_str() {
                "content-length" => {
                    if length.is_some() {
                        return Err("duplicate length".into());
                    }
                    length = Some(value.trim().parse::<usize>().map_err(|e| e.to_string())?);
                }
                "content-type" => content_type = value.trim() == "application/json",
                "cache-control" => no_store = value.trim() == "no-store",
                "connection" => close = value.trim() == "close",
                _ => {}
            }
        }
        if length != Some(body.len()) || !content_type || !no_store || !close {
            return Err("invalid response framing or cache policy".into());
        }
        if status == 200 {
            let value: Value = serde_json::from_str(body).map_err(|e| e.to_string())?;
            if !value.is_object() || value.get("error").is_some() {
                return Err("invalid success".into());
            }
            return Ok(Observed::Value(value));
        }
        let envelope: ErrorBody = serde_json::from_str(body).map_err(|e| e.to_string())?;
        if status != envelope.error.status() {
            return Err("status and code disagree".into());
        }
        Ok(Observed::Refused(envelope.error))
    }

    fn live_endpoint() -> layerx_client::sequencer::SequencerEndpoint {
        let url =
            std::env::var("LOCAL_NODE_RPC").expect("LOCAL_NODE_RPC must name the real local node");
        layerx_client::sequencer::SequencerEndpoint::new(&url, &[]).expect("real node endpoint")
    }

    fn authority_config() -> (Vec<u8>, ProtocolDeploymentVerifier) {
        use std::os::unix::fs::DirBuilderExt;
        use std::process::Command;
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = std::env::temp_dir().join(format!(
            "serve-errors-authority-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .expect("private authority directory");
        let key = root.join("key.pem");
        let certificate = Command::new("openssl")
            .args([
                "req",
                "-x509",
                "-newkey",
                "ed25519",
                "-nodes",
                "-subj",
                "/CN=serve-errors-proof",
                "-days",
                "1",
                "-outform",
                "DER",
                "-keyout",
            ])
            .arg(&key)
            .output()
            .expect("OpenSSL certificate generation");
        assert!(
            certificate.status.success(),
            "OpenSSL certificate generation failed"
        );
        let public = Command::new("openssl")
            .args(["pkey", "-pubout", "-outform", "DER", "-in"])
            .arg(&key)
            .output()
            .expect("OpenSSL public key export");
        std::fs::remove_dir_all(&root).expect("remove ephemeral authority key");
        assert!(public.status.success());
        assert_eq!(
            public.stdout.len(),
            44,
            "Ed25519 SubjectPublicKeyInfo length"
        );
        let validator = TrustedValidator::new(public.stdout[12..].try_into().unwrap(), 1);
        let verifier =
            ProtocolDeploymentVerifier::from_chain_trust("paxeer-125", vec![validator], 1000)
                .unwrap();
        (certificate.stdout, verifier)
    }

    struct Service {
        address: SocketAddr,
        _pool: HttpPool,
    }

    impl Service {
        fn start(native_chain: Option<u64>, node_chain: u64) -> Self {
            let url = std::env::var("LOCAL_NODE_RPC").expect("real local node required");
            let (ca, verifier) = authority_config();
            let reader = LayerxdProgramBalanceReader::connect(
                &url,
                "node-proof-authorization".into(),
                ProgramAuthority {
                    endpoint: "https://localhost:1",
                    authorization: "authority-proof-authorization".into(),
                    replica_id: [1; 32],
                    ca_der: &ca,
                },
                verifier,
                Registry::new(),
            )
            .expect("real program reader configuration");
            let native = native_chain.map(|chain| NativeReadRoute::new(live_endpoint(), chain));
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            let pool = serve_read_routes(
                listener,
                BEARER.into(),
                ProgramBalanceReadRoute::new(reader),
                native,
                NodeRpcConfig {
                    url,
                    chain_id: node_chain,
                    deadline: Duration::from_secs(2),
                },
                ca,
                live_endpoint(),
            )
            .unwrap();
            Self {
                address,
                _pool: pool,
            }
        }

        fn get(&self, path: &str, bearer: &str) -> Vec<u8> {
            let mut stream = TcpStream::connect(self.address).unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(6)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(6)))
                .unwrap();
            write!(stream, "GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {bearer}\r\nConnection: close\r\n\r\n").unwrap();
            stream.flush().unwrap();
            let mut raw = Vec::new();
            stream.read_to_end(&mut raw).unwrap();
            raw
        }

        fn twice(&self, path: &str, code: Code) {
            for _ in 0..2 {
                let raw = self.get(path, BEARER);
                assert_eq!(decode_response(&raw).unwrap(), Observed::Refused(code));
                assert!(
                    decode_response(&raw[..raw.len() - 1]).is_err(),
                    "truncated producer response must be refused"
                );
            }
            assert_eq!(
                decode_response(&self.get(path, "wrong-credential")).unwrap(),
                Observed::Refused(Code::Unauthorized)
            );
        }
    }

    #[test]
    fn program_state_unavailable() {
        let path = format!("/v1/programs/{}/balances", "11".repeat(32));
        // Both actual producers: refreshing authority fails on a wrong chain,
        // and, without native reads, the node connection itself rejects that chain.
        Service::start(Some(126), 125).twice(&path, Code::ProgramStateUnavailable);
        Service::start(None, 126).twice(&path, Code::ProgramStateUnavailable);
    }

    #[test]
    fn invalid_program() {
        let service = Service::start(Some(125), 125);
        for text in [
            "unknown".to_owned(),
            "00".repeat(32),
            "gg".repeat(32),
            "11".repeat(33),
        ] {
            service.twice(
                &format!("/v1/programs/{text}/balances"),
                Code::InvalidProgram,
            );
        }
    }

    #[test]
    fn invalid_read() {
        let service = Service::start(Some(125), 125);
        for path in [
            "/v1/reads/unknown/1",
            "/v1/reads/receipt/zz",
            "/v1/reads/checkpoint/01",
            "/v1/reads/history/account?limit=1&cursor=bad",
        ] {
            service.twice(path, Code::InvalidRead);
        }
    }

    #[test]
    fn verified_read_unavailable() {
        let service = Service::start(Some(125), 125);
        service.twice(
            &format!("/v1/reads/availability/{}", u64::MAX),
            Code::VerifiedReadUnavailable,
        );
    }

    #[test]
    fn read_too_large() {
        // This anchor's producer is the native-error-to-HTTP classifier. Feed its
        // actual bounded-read error type, then use the production pool, admission
        // parser and response writer. There is no JSON-RPC peer or transport double.
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let requests = Arc::new(AtomicUsize::new(0));
        let observed = Arc::clone(&requests);
        let pool = HttpPool::start(listener, move |mut stream| {
            let answer = match parse_request(&mut stream, BEARER) {
                Parsed::Admitted(_) => {
                    observed.fetch_add(1, Ordering::SeqCst);
                    native_read_response(Err(NativeReadError::ResultTooLarge))
                }
                Parsed::Refused(status, body) => (status, body.to_owned()),
                Parsed::Closed => return,
            };
            response(&mut stream, answer.0, &answer.1).unwrap();
        })
        .unwrap();
        let service = Service {
            address,
            _pool: pool,
        };
        service.twice("/v1/reads/history/account?limit=1", Code::ReadTooLarge);
        assert_eq!(
            requests.load(Ordering::SeqCst),
            2,
            "each result must come from a fresh producer invocation"
        );
    }

    #[test]
    fn happy_path_and_malformed_producer_result() {
        let service = Service::start(Some(125), 125);
        let height = live_endpoint().block_number().expect("real node head");
        let path = format!("/v1/reads/checkpoint/{height}");
        let expected = NativeReadRoute::new(live_endpoint(), 125)
            .read(&path)
            .expect("real checkpoint producer");
        for _ in 0..2 {
            let raw = service.get(&path, BEARER);
            assert_eq!(
                decode_response(&raw).unwrap(),
                Observed::Value(expected.clone())
            );
            let mut malformed = raw.clone();
            let first_body = malformed.windows(4).position(|w| w == b"\r\n\r\n").unwrap() + 4;
            // Same byte length and valid HTTP framing: the consumer's JSON decoder
            // must reject the damaged producer payload independently of framing.
            malformed[first_body] = b'!';
            assert!(decode_response(&malformed).is_err());
        }
        let raw = service.get("/v1/reads/receipt/zz", BEARER);
        let mut malformed = raw.clone();
        let offset = malformed
            .windows(b"invalid_read".len())
            .position(|w| w == b"invalid_read")
            .unwrap();
        malformed[offset] = b'x';
        assert!(
            decode_response(&malformed).is_err(),
            "unknown producer error code must be refused"
        );
        let mut mismatched = String::from_utf8(raw).unwrap();
        mismatched = mismatched.replacen("400", "503", 1);
        assert!(
            decode_response(mismatched.as_bytes()).is_err(),
            "status/code mismatch must be refused"
        );
        assert_eq!(
            native_read_response(Ok(json!({"value": 7}))),
            (200, "{\"value\":7}".into())
        );
    }
}
