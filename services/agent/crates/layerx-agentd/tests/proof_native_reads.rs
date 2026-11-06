//! Shared daemon HTTP implementation for the native-read consumer proof and host.
//!
//! The daemon HTTP assertions belong to the separate proof_serve_errors target;
//! including this module never registers tests that start a node.

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

pub(crate) fn native_read_response(
    result: Result<serde_json::Value, NativeReadError>,
) -> (u16, String) {
    let (status, body) = match result {
        Ok(value) => return (200, value.to_string()),
        Err(NativeReadError::InvalidRequest | NativeReadError::CursorMismatch) => {
            (400, "{\"error\":\"invalid_read\"}")
        }
        Err(NativeReadError::ResultTooLarge) => (413, "{\"error\":\"read_too_large\"}"),
        Err(error) => {
            return (
                503,
                serde_json::json!({
                    "error": "verified_read_unavailable",
                    "detail": format!("{error:?}"),
                })
                .to_string(),
            );
        }
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
    fn native_read_unavailable_preserves_error_detail() {
        for error in [NativeReadError::Unavailable, NativeReadError::Verification] {
            let detail = format!("{error:?}");
            let (status, body) = native_read_response(Err(error));
            assert_eq!(status, 503);
            assert_eq!(
                serde_json::from_str::<serde_json::Value>(&body).expect("native refusal JSON"),
                serde_json::json!({"error": "verified_read_unavailable", "detail": detail})
            );
        }
    }

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
pub(crate) mod disposable_node {
    use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt};
    use std::path::{Path, PathBuf};
    use std::process::{Command, Stdio};
    use std::thread;
    use std::time::{SystemTime, UNIX_EPOCH};
    use zeroize::Zeroizing;

    #[track_caller]
    fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
        result.unwrap_or_else(|error| panic!("disposable native reads node: {error:?}"))
    }

    pub(crate) struct LocalNode {
        script: PathBuf,
        home: PathBuf,
        comet_url: String,
    }

    impl LocalNode {
        pub(crate) fn start() -> (Self, String, Zeroizing<String>) {
            let root = Path::new(env!("CARGO_MANIFEST_DIR"))
                .ancestors()
                .find(|path| {
                    path.join("tools/qualification/localnode/v6120-node.sh")
                        .is_file()
                })
                .expect("repository root for tools/qualification/localnode/v6120-node.sh");
            let script = root.join("tools/qualification/localnode/v6120-node.sh");
            assert!(
                script.is_file(),
                "missing producer harness: {}",
                script.display()
            );
            let directory = Path::new("/tmp/lx-localnode").join(format!(
                "native-reads-{}-{}",
                std::process::id(),
                must(SystemTime::now().duration_since(UNIX_EPOCH)).as_nanos()
            ));
            must(
                std::fs::DirBuilder::new()
                    .recursive(true)
                    .mode(0o700)
                    .create(&directory),
            );
            let environment = directory.join("localnode.env");
            let log_path = directory.join("startup.log");
            let log = must(
                std::fs::OpenOptions::new()
                    .write(true)
                    .create_new(true)
                    .mode(0o600)
                    .open(&log_path),
            );
            let status = Command::new("bash")
                .arg(&script)
                .args(["--sequencer", "--env-out"])
                .arg(&environment)
                .env("TMPDIR", "/tmp")
                .env_remove("PAXD")
                .stdout(Stdio::from(must(log.try_clone())))
                .stderr(Stdio::from(log))
                .status()
                .expect("start tools/qualification/localnode/v6120-node.sh producer");
            assert!(
                status.success(),
                "tools/qualification/localnode/v6120-node.sh failed ({status}); evidence: {}",
                log_path.display()
            );
            let text = Zeroizing::new(must(std::fs::read_to_string(&environment)));
            let field = |name: &str| {
                text.lines()
                    .filter_map(|line| line.split_once('='))
                    .find_map(|(key, value)| (key == name).then_some(value))
                    .unwrap_or_else(|| panic!("producer localnode.env is missing {name}"))
            };
            // Install cleanup before decoding the endpoint or key so assertion failures
            // in the proof also stop the producer's node and feeder processes.
            let node = Self {
                script,
                home: PathBuf::from(field("LOCAL_NODE_HOME")),
                comet_url: field("PERPS_DATA_TEST_COMET").to_owned(),
            };
            let url = field("LOCAL_NODE_SEQUENCER_RPC").to_owned();
            let key = Zeroizing::new(field("LOCAL_NODE_KEY").to_owned());
            must(std::fs::remove_file(environment));
            (node, url, key)
        }

        pub(crate) fn native_read_route(
            &self,
            url: &str,
            chain_id: u64,
        ) -> layerx_agentd::read::NativeReadRoute {
            use layerx_agentd::read::{CommitValidator, NativeReadRoute, NativeResultSource};
            use layerx_client::sequencer::SequencerEndpoint;

            let genesis: serde_json::Value = must(serde_json::from_slice(&must(std::fs::read(
                self.home.join("config/genesis.json"),
            ))));
            let validators = genesis["validators"]
                .as_array()
                .expect("disposable node genesis validator authority")
                .iter()
                .map(|validator| {
                    assert_eq!(validator["pub_key"]["type"], "tendermint/PubKeyEd25519");
                    let decoded = must(
                        Command::new("python3")
                            .args([
                                "-c",
                                "import base64,sys; sys.stdout.buffer.write(base64.b64decode(sys.argv[1],validate=True))",
                            ])
                            .arg(
                                validator["pub_key"]["value"]
                                    .as_str()
                                    .expect("genesis validator public key"),
                            )
                            .output(),
                    );
                    assert!(decoded.status.success(), "decode genesis validator public key");
                    let public_key = must(decoded.stdout.try_into());
                    let address = CommitValidator::address_for_key(&public_key);
                    assert_eq!(
                        layerx_programs::hex::encode(&address).to_ascii_uppercase(),
                        validator["address"]
                            .as_str()
                            .expect("genesis validator address")
                    );
                    CommitValidator {
                        address,
                        public_key,
                        voting_power: must(
                            validator["power"]
                                .as_str()
                                .expect("genesis validator voting power")
                                .parse(),
                        ),
                    }
                })
                .collect();
            must(
                NativeReadRoute::new(must(SequencerEndpoint::new(url, &[])), chain_id)
                    .with_checkpoint_trust(
                        genesis["chain_id"]
                            .as_str()
                            .expect("genesis consensus chain id")
                            .to_owned(),
                        validators,
                    ),
            )
            .with_result_source(must(NativeResultSource::connect(&self.comet_url, None)))
        }
    }

    impl Drop for LocalNode {
        fn drop(&mut self) {
            let status = Command::new("bash")
                .arg(&self.script)
                .arg("--stop")
                .arg(&self.home)
                .env("TMPDIR", "/tmp")
                .status();
            if !matches!(status, Ok(status) if status.success()) {
                if thread::panicking() {
                    eprintln!(
                        "failed to stop native reads producer: {}",
                        self.home.display()
                    );
                } else {
                    panic!(
                        "failed to stop native reads producer: {}",
                        self.home.display()
                    );
                }
            }
        }
    }
}
