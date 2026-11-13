use std::error::Error;
use std::fs;
use std::io::{self, BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::os::unix::net::UnixStream;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_identity_provider::{
    provision_head::validate_account_head, Policy, Server, State,
};
use layerx_types::clock::Clock;
use serde_json::{json, Value};

// Compile the actual production clock authority and operational probe in-process.
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
#[allow(dead_code)]
mod clock_server;
#[path = "../src/probe.rs"]
#[allow(dead_code)]
mod readiness;

type Result<T = ()> = std::result::Result<T, Box<dyn Error>>;
const README: &str = include_str!("../README.md");
const NOW: u64 = 1_700_000_000_000;

fn example(filename: &str) -> Result<Value> {
    let section = README
        .split_once(&format!("Example `{filename}`"))
        .ok_or("missing documented input")?
        .1;
    let block = section
        .split_once("```json\n")
        .ok_or("missing documented JSON")?
        .1
        .split_once("\n```")
        .ok_or("unterminated documented JSON")?
        .0;
    Ok(serde_json::from_str(block)?)
}

struct Worker {
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<io::Result<()>>>,
}

impl Worker {
    fn start(run: impl FnOnce(Arc<AtomicBool>) -> io::Result<()> + Send + 'static) -> Self {
        let stop = Arc::new(AtomicBool::new(false));
        let flag = Arc::clone(&stop);
        Self {
            stop,
            thread: Some(thread::spawn(move || run(flag))),
        }
    }

    fn finish(&mut self) -> Result {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.thread.take() {
            worker.join().map_err(|_| "peer panicked")??;
        }
        Ok(())
    }
}

impl Drop for Worker {
    fn drop(&mut self) {
        assert!(self.finish().is_ok(), "production peer failed");
    }
}

// Replay the committed producer's ABI/account artifact through real HTTP. The
// component being tested is the production validator and both of its RPC clients.
struct ChainTranscript {
    url: String,
    requests: Arc<Mutex<Vec<Value>>>,
    _worker: Worker,
}

impl ChainTranscript {
    fn start(record: Value) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let url = format!("http://{}/", listener.local_addr()?);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let seen = Arc::clone(&requests);
        let worker = Worker::start(move |stop| {
            while !stop.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(peer) => peer,
                    Err(error) if error.kind() == io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => return Err(error),
                };
                stream.set_read_timeout(Some(Duration::from_secs(5)))?;
                stream.set_write_timeout(Some(Duration::from_secs(5)))?;
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                reader.read_line(&mut line)?;
                assert!(line.starts_with("POST / HTTP/1.1"));
                let mut length = None;
                loop {
                    line.clear();
                    if reader.read_line(&mut line)? == 0 {
                        return Err(io::Error::other("truncated HTTP request"));
                    }
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().map_err(io::Error::other)?);
                    }
                }
                let length = length.ok_or_else(|| io::Error::other("missing request length"))?;
                assert!(length <= 1_048_576);
                let mut bytes = vec![0; length];
                reader.read_exact(&mut bytes)?;
                let request: Value = serde_json::from_slice(&bytes)?;
                assert_eq!(request["jsonrpc"], "2.0");
                seen.lock()
                    .map_err(|_| io::Error::other("requests poisoned"))?
                    .push(request.clone());
                let params = &request["params"];
                let result = match request["method"].as_str() {
                    Some("eth_chainId") => {
                        assert_eq!(params, &json!([]));
                        json!("0x7d")
                    }
                    Some("eth_blockNumber") => {
                        assert_eq!(params, &json!([]));
                        json!("0x11")
                    }
                    Some("eth_getBlockByNumber") => {
                        assert_eq!(params[0], "0x11");
                        assert!(params[1].is_boolean());
                        json!({"number":"0x11", "hash":format!("0x{:064x}",17),
                            "parentHash":format!("0x{:064x}",16), "timestamp":format!("{:#x}",NOW / 1000),
                            "stateRoot":format!("0x{:064x}",117),
                            "transactionsRoot":format!("0x{}","11".repeat(32)),
                            "receiptsRoot":format!("0x{}","22".repeat(32)), "transactions":[]})
                    }
                    Some("px_getAccount") => {
                        assert_eq!(params, &json!([record["owner_did"]]));
                        record["account"].clone()
                    }
                    Some("eth_call") => {
                        assert_eq!(params[1], "0x11");
                        record["calls"]
                            .as_array()
                            .ok_or_else(|| io::Error::other("calls"))?
                            .iter()
                            .find(|call| {
                                call["to"] == params[0]["to"] && call["data"] == params[0]["data"]
                            })
                            .ok_or_else(|| {
                                io::Error::other(format!("unrecorded request: {request}"))
                            })?["result"]
                            .clone()
                    }
                    _ => return Err(io::Error::other(format!("unrecorded request: {request}"))),
                };
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len())?;
                stream.flush()?;
            }
            Ok(())
        });
        Ok(Self {
            url,
            requests,
            _worker: worker,
        })
    }
}

fn fixture() -> Result<Value> {
    Ok(serde_json::from_str(include_str!(
        "../src/fixtures/repair_22_33_34.json"
    ))?)
}

fn observe(input: &Value, now: u64) -> io::Result<Vec<u8>> {
    let mut output = Vec::new();
    validate_account_head(input.to_string().as_bytes(), &mut output, now)?;
    Ok(output)
}

fn refuses_without_publication(input: &Value, now: u64) {
    let mut output = Vec::new();
    assert!(validate_account_head(input.to_string().as_bytes(), &mut output, now).is_err());
    assert!(
        output.is_empty(),
        "refused observation published owner limits"
    );
}

#[test]
fn identity_provision_doc_committed_observations() -> Result {
    let record = fixture()?;
    let peer = ChainTranscript::start(record.clone())?;
    let mut input = example("account-observation-request.json")?;
    for field in ["chain_id", "owner_did", "owner_account", "asset_id"] {
        assert_eq!(input[field], record[field], "documented {field}");
    }
    // Replace deployment transport inputs only; identity and policy come from README.
    input["rpc_url"] = json!(peer.url);
    input["rpc_ca_der_hex"] = json!([]);
    let result: Value = serde_json::from_slice(&observe(&input, NOW)?)?;
    assert_eq!(result.as_object().ok_or("result object")?.len(), 11);
    assert_eq!(result["schema"], "layerx.human.account-head.v1");
    assert_eq!(result["chain_id"], 125);
    assert_eq!(result["owner_did"], input["owner_did"]);
    assert_eq!(result["account"], input["owner_account"]);
    assert_eq!(result["asset_id"], input["asset_id"]);
    assert_eq!(result["committed_height"], 17);
    assert_eq!(result["block_hash"], format!("{:064x}", 17));
    assert_eq!(result["state_root"], format!("{:064x}", 117));
    assert_eq!(result["observed_at"], NOW);
    assert_eq!(result["balance"], "100");
    assert_eq!(result["consumed"], 0);
    let requests = peer.requests.lock().map_err(|_| "requests poisoned")?;
    assert_eq!(
        requests
            .iter()
            .filter(|r| r["method"] == "px_getAccount")
            .count(),
        2
    );
    assert_eq!(
        requests
            .iter()
            .filter(|r| r["method"] == "eth_blockNumber")
            .count(),
        2
    );
    for call in record["calls"].as_array().ok_or("fixture calls")? {
        assert_eq!(
            requests
                .iter()
                .filter(|r| r["method"] == "eth_call"
                    && r["params"][0]["data"] == call["data"]
                    && r["params"][0]["to"] == call["to"]
                    && r["params"][1] == "0x11")
                .count(),
            1
        );
        assert!(README.contains(
            call["signature"]
                .as_str()
                .ok_or("signature")?
                .split('(')
                .next()
                .ok_or("method")?
        ));
    }
    drop(requests);
    refuses_without_publication(&input, NOW + 60_001);
    refuses_without_publication(&input, NOW - 1);
    let mut wrong_chain = input.clone();
    wrong_chain["chain_id"] = json!(126);
    refuses_without_publication(&wrong_chain, NOW);
    for url in ["http://chain.example.test/", "https://chain.example.test/"] {
        let mut untrusted = input.clone();
        untrusted["rpc_url"] = json!(url);
        refuses_without_publication(&untrusted, NOW);
    }
    refuses_without_publication(
        &json!({"head":{"current":true},"network_id":125,"sequencer_id":"61".repeat(32),"public_key":"61".repeat(32)}),
        NOW,
    );
    drop(peer);

    for path in [
        "/calls/4/result",
        "/calls/2/result",
        "/account/account/layerx_account",
    ] {
        let mut damaged = record.clone();
        *damaged.pointer_mut(path).ok_or("fixture field")? = json!(format!("0x{:064x}", 7));
        let peer = ChainTranscript::start(damaged)?;
        input["rpc_url"] = json!(peer.url);
        refuses_without_publication(&input, NOW);
    }
    assert!(README.contains("--produce-owner-registration"));
    assert!(README.contains("--validate-owner-registration"));
    assert!(README.contains("human_account_head_observe"));
    assert!(!README.contains("Compatibility validator for a first-batch"));
    Ok(())
}

fn policy() -> Policy {
    Policy {
        root: [0x51; 32],
        threshold: 1,
        delay_seconds: 86_400,
    }
}

#[test]
fn identity_provision_doc_local_owner_survives_reopen() -> Result {
    let directory = tempfile::Builder::new()
        .permissions(fs::Permissions::from_mode(0o700))
        .tempdir()?;
    let root = directory.path().join("state");
    let input = example("owner-request.json")?;
    let mut state = State::open(&root, policy())?;
    let mut output = Vec::new();
    state.provision_owner(input.to_string().as_bytes(), &mut output)?;
    let owner: Value = serde_json::from_slice(&output)?;
    assert_eq!(owner.as_object().ok_or("owner object")?.len(), 5);
    let principal = owner["principal"].as_str().ok_or("principal")?;
    assert!(principal.starts_with("act_"));
    assert_eq!(owner["did"], format!("did:layerx:{principal}"));
    assert_eq!(owner["recovery_root"], json!(policy().root));
    assert_eq!(owner["recovery_threshold"], 1);
    assert_eq!(owner["recovery_delay_seconds"], 86_400);
    drop(state);
    let mut reopened = State::open(&root, policy())?;
    let mut retry = Vec::new();
    reopened.provision_owner(input.to_string().as_bytes(), &mut retry)?;
    assert_eq!(output, retry);
    assert_ne!(owner["did"], fixture()?["owner_did"]);
    Ok(())
}

fn start_clock(root: &Path) -> Result<(ClockBinding, Worker)> {
    let mut server = clock_server::Server::bind(root)?;
    let binding = ClockBinding {
        socket: server.socket().to_owned(),
        process: std::process::id(),
        user: rustix::process::geteuid().as_raw(),
    };
    let worker = Worker::start(move |stop| {
        while !stop.load(Ordering::Acquire) {
            server.poll()?;
            thread::sleep(Duration::from_millis(1));
        }
        Ok(())
    });
    Ok((binding, worker))
}

fn start_provider(
    socket: &Path,
    root: &Path,
    clock: Arc<RuntimeClock>,
    uid: u32,
) -> Result<Worker> {
    let server = Server::bind(
        socket,
        State::open(root, policy())?,
        uid,
        Duration::from_secs(5),
        clock,
    )?;
    Ok(Worker::start(move |stop| server.run(&stop)))
}

#[test]
fn identity_clock_doc_authenticated_startup_and_readiness() -> Result {
    for name in [
        "LAYERX_RUNTIME_CLOCK_SOCKET",
        "LAYERX_RUNTIME_CLOCK_PID",
        "LAYERX_RUNTIME_CLOCK_UID",
    ] {
        assert!(
            README.contains(&format!("| `{name}` | Required")),
            "missing required {name}"
        );
    }
    let directory = tempfile::Builder::new()
        .permissions(fs::Permissions::from_mode(0o700))
        .tempdir()?;
    let (binding, mut authority) = start_clock(directory.path())?;
    let clock = RuntimeClock::connect(binding.clone())?;
    assert!(clock.sample(Duration::from_secs(1))?.unix_milliseconds > 0);
    let mut wrong = binding.clone();
    wrong.process = wrong.process.checked_add(1).ok_or("PID overflow")?;
    assert!(RuntimeClock::connect(wrong).is_err());
    let mut wrong = binding.clone();
    wrong.process = 0;
    assert!(RuntimeClock::connect(wrong).is_err());
    let mut wrong = binding.clone();
    wrong.user = wrong.user.checked_add(1).ok_or("UID overflow")?;
    assert!(RuntimeClock::connect(wrong).is_err());
    fs::set_permissions(&binding.socket, fs::Permissions::from_mode(0o660))?;
    assert!(RuntimeClock::connect(binding.clone()).is_err());
    fs::set_permissions(&binding.socket, fs::Permissions::from_mode(0o600))?;
    let parent = binding.socket.parent().ok_or("clock parent")?;
    fs::set_permissions(parent, fs::Permissions::from_mode(0o750))?;
    assert!(RuntimeClock::connect(binding.clone()).is_err());
    fs::set_permissions(parent, fs::Permissions::from_mode(0o700))?;
    let socket = directory.path().join("identity.sock");
    let root = directory.path().join("state");
    let settings = readiness::Settings {
        socket: socket.clone(),
        deadline: Duration::from_secs(1),
    };
    assert!(readiness::probe(&settings).is_err());
    let mut provider = start_provider(&socket, &root, Arc::clone(&clock), binding.user)?;
    readiness::probe(&settings)?;

    // Queue operation 0 behind an incomplete real connection. The probe must
    // honor its own total budget, even though the server's budget is five seconds.
    let mut slow = UnixStream::connect(&socket)?;
    slow.write_all(&[0])?;
    let began = clock.sample(Duration::from_secs(1))?;
    let error = readiness::probe(&settings)
        .err()
        .ok_or("queued probe unexpectedly ready")?;
    assert_eq!(error.kind(), io::ErrorKind::TimedOut);
    assert!(clock.sample(Duration::from_secs(1))?.elapsed_since(began)? < Duration::from_secs(2));
    drop(slow);
    readiness::probe(&settings)?;
    provider.finish()?;
    assert!(readiness::probe(&settings).is_err());
    let mut reopened = start_provider(&socket, &root, Arc::clone(&clock), binding.user)?;
    readiness::probe(&settings)?;
    reopened.finish()?;
    let mut refused = start_provider(
        &socket,
        &root,
        Arc::clone(&clock),
        binding.user.checked_add(1).ok_or("UID overflow")?,
    )?;
    assert!(readiness::probe(&settings).is_err());
    refused.finish()?;
    let mut live = start_provider(&socket, &root, Arc::clone(&clock), binding.user)?;
    readiness::probe(&settings)?;
    authority.finish()?;
    assert!(RuntimeClock::connect(binding).is_err());
    assert!(clock.sample(Duration::from_secs(1)).is_err());
    assert!(readiness::probe(&settings).is_err());
    assert!(
        live.finish().is_err(),
        "clock loss must withdraw the serving provider"
    );
    Ok(())
}
