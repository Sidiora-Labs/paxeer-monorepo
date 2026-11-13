// Shared daemon HTTP implementation for the native-read consumer proof and host.
//
// The daemon HTTP assertions live beside the shared listener and parser so the
// `native-reads-http` target and the host exercise the same implementation.

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
            let worker = Self::spawn_worker(index, admissions, handle)?;
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
                            thread::sleep(SUPERVISION_INTERVAL);
                        }
                        Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                        Err(_) => return,
                    }
                })
                .map_err(|error| format!("HTTP accept thread failed: {error}"))?,
        );
        Ok(pool)
    }

    fn spawn_worker<F>(
        index: usize,
        admissions: Arc<(Mutex<HttpAdmissions>, std::sync::Condvar)>,
        handle: Arc<F>,
    ) -> Result<thread::JoinHandle<()>, String>
    where
        F: Fn(DeadlineStream) + Send + Sync + 'static,
    {
        thread::Builder::new()
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
                    if let Ok(socket) = stream.socket.try_clone() {
                        state.active[index] = Some(socket);
                    } else {
                        state.pending[index] = state.queue.pop_front();
                        state.busy[index] = state.pending[index].is_some();
                        continue;
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
            .map_err(|error| format!("HTTP worker failed: {error}"))
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
    use layerx_client::sequencer::{RpcRefusal, SequencerError};

    #[test]
    fn native_read_unavailable_has_fixed_body() {
        for error in [
            NativeReadError::Unavailable,
            NativeReadError::Verification,
            NativeReadError::Connection(SequencerError::Rpc(RpcRefusal {
                code: -32043,
                message: "private-native-read-rpc-message".into(),
                data: Some(serde_json::json!({"diagnostic": "private-native-read-rpc-data"})),
            })),
        ] {
            let detail = format!("{error:?}");
            let (status, body) = native_read_response(Err(error));
            assert_eq!(status, 503);
            assert_eq!(
                body.as_bytes(),
                b"{\"error\":\"verified_read_unavailable\"}"
            );
            assert!(!body.contains(&detail));
            for diagnostic in [
                "Unavailable",
                "Verification",
                "Connection",
                "Rpc",
                "RpcRefusal",
                "detail",
                "message",
                "data",
                "-32043",
                "private-native-read-rpc-message",
                "private-native-read-rpc-data",
            ] {
                assert!(
                    !body.contains(diagnostic),
                    "leaked diagnostic: {diagnostic}"
                );
            }
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

        let rendered: serde_json::Value = serde_json::from_str(&balance_json(&read))
            .unwrap_or_else(|error| panic!("balance response is valid JSON: {error:?}"));

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
        let rendered: serde_json::Value = serde_json::from_str(&balance_json(&read))
            .unwrap_or_else(|error| panic!("balance response is valid JSON: {error:?}"));
        assert!(rendered["accounts"][0]["frozen"].is_null());

        read.accounts[0].frozen = Some(false);
        let rendered: serde_json::Value = serde_json::from_str(&balance_json(&read))
            .unwrap_or_else(|error| panic!("balance response is valid JSON: {error:?}"));
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
                .unwrap_or_else(|| {
                    panic!("repository root for tools/qualification/localnode/v6120-node.sh")
                });
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
                .unwrap_or_else(|error| {
                    panic!("start tools/qualification/localnode/v6120-node.sh producer: {error:?}")
                });
            let evidence_path = if status.success() {
                log_path
            } else {
                let record = Path::new("/out/logs").join(
                    directory
                        .file_name()
                        .unwrap_or_else(|| panic!("unique native reads startup directory")),
                );
                must(
                    std::fs::DirBuilder::new()
                        .recursive(true)
                        .mode(0o700)
                        .create(&record),
                );
                let evidence = record.join("startup.log");
                must(std::fs::copy(&log_path, &evidence));
                evidence
            };
            assert!(
                status.success(),
                "tools/qualification/localnode/v6120-node.sh failed ({status}); evidence: {}",
                evidence_path.display()
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
                .unwrap_or_else(|| panic!("disposable node genesis validator authority"))
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
                                    .unwrap_or_else(|| panic!("genesis validator public key")),
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
                            .unwrap_or_else(|| panic!("genesis validator address"))
                    );
                    CommitValidator {
                        address,
                        public_key,
                        voting_power: must(
                            validator["power"]
                                .as_str()
                                .unwrap_or_else(|| panic!("genesis validator voting power"))
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
                            .unwrap_or_else(|| panic!("genesis consensus chain id"))
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

#[cfg(test)]
mod proof {
    use super::disposable_node;
    use super::native_read_response;
    use layerx_agentd::node_rpc::NodeRpcConfig;
    use layerx_agentd::read::{NativeReadError, ProgramBalanceReadRoute};
    use layerx_client::sequencer::{RpcRefusal, SequencerError};
    use std::io::{Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::sync::Arc;
    use std::time::Duration;

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
        fn status(self) -> u16 {
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
            .unwrap_or_else(|error| panic!("private authority directory: {error:?}"));
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
            .unwrap_or_else(|error| panic!("OpenSSL certificate generation: {error:?}"));
        assert!(
            certificate.status.success(),
            "OpenSSL certificate generation failed"
        );
        let public = Command::new("openssl")
            .args(["pkey", "-pubout", "-outform", "DER", "-in"])
            .arg(&key)
            .output()
            .unwrap_or_else(|error| panic!("OpenSSL public key export: {error:?}"));
        std::fs::remove_dir_all(&root)
            .unwrap_or_else(|error| panic!("remove ephemeral authority key: {error:?}"));
        assert!(public.status.success());
        assert_eq!(
            public.stdout.len(),
            44,
            "Ed25519 SubjectPublicKeyInfo length"
        );
        let validator = TrustedValidator::new(
            public.stdout[12..]
                .try_into()
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
            1,
        );
        let verifier =
            ProtocolDeploymentVerifier::from_chain_trust("paxeer-125", vec![validator], 1000)
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        (certificate.stdout, verifier)
    }

    struct Service {
        address: SocketAddr,
        pool: HttpPool,
        node: Option<(disposable_node::LocalNode, String)>,
    }

    impl Service {
        fn start(native_chain: Option<u64>, node_chain: u64) -> Self {
            let (node, url, _key) = disposable_node::LocalNode::start();
            let endpoint = || {
                layerx_client::sequencer::SequencerEndpoint::new(&url, &[])
                    .unwrap_or_else(|error| panic!("disposable loopback node endpoint: {error:?}"))
            };
            assert_eq!(
                endpoint()
                    .chain_id()
                    .unwrap_or_else(|error| panic!("producer chain id: {error:?}")),
                125
            );
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
            .unwrap_or_else(|error| panic!("real program reader configuration: {error:?}"));
            let native = native_chain.map(|chain| node.native_read_route(&url, chain));
            let listener = TcpListener::bind("127.0.0.1:0")
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            let address = listener
                .local_addr()
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            let pool = serve_read_routes(
                listener,
                BEARER.into(),
                ProgramBalanceReadRoute::new(reader),
                native,
                NodeRpcConfig {
                    url: url.clone(),
                    chain_id: node_chain,
                    deadline: Duration::from_secs(2),
                },
                ca,
                endpoint(),
            )
            .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            Self {
                address,
                pool,
                node: Some((node, url)),
            }
        }

        fn endpoint(&self) -> layerx_client::sequencer::SequencerEndpoint {
            layerx_client::sequencer::SequencerEndpoint::new(
                &self
                    .node
                    .as_ref()
                    .unwrap_or_else(|| panic!("service owns its disposable node"))
                    .1,
                &[],
            )
            .unwrap_or_else(|error| panic!("disposable loopback node endpoint: {error:?}"))
        }

        fn get(&self, path: &str, bearer: &str) -> Vec<u8> {
            self.pool
                .check()
                .unwrap_or_else(|error| panic!("HTTP pool failed: {error}"));
            let mut stream = TcpStream::connect(self.address)
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            stream
                .set_read_timeout(Some(Duration::from_secs(6)))
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            stream
                .set_write_timeout(Some(Duration::from_secs(6)))
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            write!(stream, "GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {bearer}\r\nConnection: close\r\n\r\n").unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            stream
                .flush()
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            let mut raw = Vec::new();
            stream
                .read_to_end(&mut raw)
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            raw
        }

        fn twice(&self, path: &str, code: Code) {
            for _ in 0..2 {
                let raw = self.get(path, BEARER);
                assert_eq!(
                    decode_response(&raw)
                        .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
                    Observed::Refused(code)
                );
                assert!(
                    decode_response(&raw[..raw.len() - 1]).is_err(),
                    "truncated producer response must be refused"
                );
            }
            assert_eq!(
                decode_response(&self.get(path, "wrong-credential"))
                    .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
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
        let path = format!("/v1/reads/availability/{}", u64::MAX);
        for _ in 0..2 {
            let raw = service.get(&path, BEARER);
            assert_eq!(
                decode_response(&raw)
                    .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
                Observed::Refused(Code::VerifiedReadUnavailable)
            );
            let first_body = raw
                .windows(4)
                .position(|w| w == b"\r\n\r\n")
                .unwrap_or_else(|| panic!("response boundary missing"))
                + 4;
            assert_eq!(
                &raw[first_body..],
                b"{\"error\":\"verified_read_unavailable\"}"
            );
            assert!(decode_response(&raw[..raw.len() - 1]).is_err());

            // Corrupt the actual response only at the consumer boundary, keeping
            // its framing valid so unknown fields reach the strict JSON decoder.
            for (field, value) in [
                ("detail", json!("private-native-read-rpc-message")),
                ("detail", Value::Null),
                (
                    "diagnostic",
                    json!({"data": "private-native-read-rpc-data"}),
                ),
            ] {
                let mut body: Value = serde_json::from_slice(&raw[first_body..])
                    .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
                body.as_object_mut()
                    .unwrap_or_else(|| panic!("response object missing"))
                    .insert(field.into(), value);
                let body = body.to_string();
                let head = std::str::from_utf8(&raw[..first_body])
                    .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"))
                    .replace(
                        &format!("Content-Length: {}\r\n", raw.len() - first_body),
                        &format!("Content-Length: {}\r\n", body.len()),
                    );
                let mut malformed = head.into_bytes();
                malformed.extend_from_slice(body.as_bytes());
                let error = decode_response(&malformed)
                    .err()
                    .unwrap_or_else(|| panic!("invalid response was accepted"));
                assert!(error.contains("unknown field"), "{error}");
            }
        }
        assert_eq!(
            decode_response(&service.get(&path, "wrong-credential"))
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
            Observed::Refused(Code::Unauthorized)
        );
    }

    #[test]
    fn native_read_refusals_redact_internal_errors() {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        let address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        let requests = Arc::new(AtomicUsize::new(0));
        let observed = Arc::clone(&requests);
        let pool = HttpPool::start(listener, move |mut stream| {
            let answer = match parse_request(&mut stream, BEARER) {
                Parsed::Admitted(path) => {
                    assert_eq!(path, "/v1/reads/checkpoint/1");
                    let index = observed.fetch_add(1, Ordering::SeqCst);
                    native_read_response(Err(NativeReadError::Connection(SequencerError::Rpc(
                        RpcRefusal {
                            code: -32043,
                            message: format!("private-native-read-rpc-message-{index}"),
                            data: Some(json!({
                                "diagnostic": format!("private-native-read-rpc-data-{index}"),
                                "request": index,
                            })),
                        },
                    ))))
                }
                Parsed::Refused(status, body) => (status, body.to_owned()),
                Parsed::Closed => return,
            };
            response(&mut stream, answer.0, &answer.1)
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        })
        .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        let service = Service {
            address,
            pool,
            node: None,
        };
        for _ in 0..2 {
            let raw = service.get("/v1/reads/checkpoint/1", BEARER);
            assert!(raw.starts_with(b"HTTP/1.1 503 "));
            assert_eq!(
                decode_response(&raw)
                    .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
                Observed::Refused(Code::VerifiedReadUnavailable)
            );
            let first_body = raw
                .windows(4)
                .position(|w| w == b"\r\n\r\n")
                .unwrap_or_else(|| panic!("response boundary missing"))
                + 4;
            assert_eq!(
                &raw[first_body..],
                b"{\"error\":\"verified_read_unavailable\"}"
            );
            let text = std::str::from_utf8(&raw[first_body..])
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
            for diagnostic in [
                "Connection",
                "Rpc",
                "RpcRefusal",
                "detail",
                "message",
                "data",
                "diagnostic",
                "-32043",
                "private-native-read-rpc-message",
                "private-native-read-rpc-data",
            ] {
                assert!(
                    !text.contains(diagnostic),
                    "leaked diagnostic: {diagnostic}"
                );
            }
            assert!(decode_response(&raw[..raw.len() - 1]).is_err());
        }
        assert_eq!(
            decode_response(&service.get("/v1/reads/checkpoint/1", "wrong-credential"))
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
            Observed::Refused(Code::Unauthorized)
        );
        assert_eq!(
            requests.load(Ordering::SeqCst),
            2,
            "independent producer invocations"
        );
    }

    #[test]
    fn read_too_large() {
        // This anchor's producer is the native-error-to-HTTP classifier. Feed its
        // actual bounded-read error type, then use the production pool, admission
        // parser and response writer. There is no JSON-RPC peer or transport double.
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        let address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
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
            response(&mut stream, answer.0, &answer.1)
                .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        })
        .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
        let service = Service {
            address,
            pool,
            node: None,
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
        let height = service
            .endpoint()
            .block_number()
            .unwrap_or_else(|error| panic!("real node head: {error:?}"));
        let path = format!("/v1/reads/checkpoint/{height}");
        let (node, url) = service
            .node
            .as_ref()
            .unwrap_or_else(|| panic!("service owns its disposable node"));
        let expected = node
            .native_read_route(url, 125)
            .read(&path)
            .unwrap_or_else(|error| panic!("real checkpoint producer: {error:?}"));
        for _ in 0..2 {
            let raw = service.get(&path, BEARER);
            assert_eq!(
                decode_response(&raw)
                    .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}")),
                Observed::Value(expected.clone())
            );
            let mut malformed = raw.clone();
            let first_body = malformed
                .windows(4)
                .position(|w| w == b"\r\n\r\n")
                .unwrap_or_else(|| panic!("response boundary missing"))
                + 4;
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
            .unwrap_or_else(|| panic!("response boundary missing"));
        malformed[offset] = b'x';
        assert!(
            decode_response(&malformed).is_err(),
            "unknown producer error code must be refused"
        );
        let mut mismatched = String::from_utf8(raw)
            .unwrap_or_else(|error| panic!("native HTTP proof failed: {error:?}"));
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
