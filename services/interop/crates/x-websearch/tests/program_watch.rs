use std::error::Error;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use serde_json::{json, Value};
use x_websearch::program::ProgramRequest;
use x_websearch::watch::{
    decode_program_requested, decode_requested, hex0x, program_requested_topic, requested_topic,
    unhex0x, Canonical, EvmError, EvmRpc, RequestWatcher, MAX_PAYLOAD_BYTES,
    PROGRAM_REQUESTED_EVENT, XWEB_PRECOMPILE,
};

type Checked<T = ()> = Result<T, Box<dyn Error>>;
const CHAIN_ID: u64 = 713_714;

fn recording() -> Checked<Value> {
    Ok(serde_json::from_str(include_str!(
        "fixtures/evm/watch.json"
    ))?)
}

fn web_log() -> Checked<Value> {
    let mut log = recording()?["endpoints"]["evm"]["*"][1]["result"][0].clone();
    log["blockHash"] = json!(hex0x(&[0x31; 32]));
    Ok(log)
}

fn program_log() -> Checked<Value> {
    let mut log = web_log()?;
    log["topics"] = json!([
        hex0x(&program_requested_topic()),
        "0xa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf",
        "0x0000000000000000000000000000000000000000000000000000000000000007"
    ]);
    log["logIndex"] = json!("0x1");
    log["data"] = json!(concat!(
        "0x",
        "0000000000000000000000000000000000000000000000000000000000000001",
        "0000000000000000000000000000000000000000000000000000000000000060",
        "0000000000000000000000000000000000000000000000000000000000000029",
        "0000000000000000000000000000000000000000000000000000000000000013",
        "68747470733a2f2f7061786565722e6170702f00000000000000000000000000"
    ));
    Ok(log)
}

fn expected_request() -> ProgramRequest {
    ProgramRequest {
        program_id: std::array::from_fn(|i| 0xa0 + u8::try_from(i).unwrap()),
        request_id: 7,
        kind: 1,
        payload: b"https://paxeer.app/".to_vec(),
        sequence: 41,
    }
}

fn replace_word(log: &Value, index: usize, value: u64) -> Checked<Value> {
    let mut changed = log.clone();
    let mut data = unhex0x(log["data"].as_str().ok_or("missing data")?).ok_or("bad data")?;
    data[index * 32..(index + 1) * 32].fill(0);
    data[index * 32 + 24..(index + 1) * 32].copy_from_slice(&value.to_be_bytes());
    changed["data"] = json!(hex0x(&data));
    Ok(changed)
}

#[test]
fn a_program_request_log_decodes_to_a_program_request() -> Checked {
    assert_eq!(
        PROGRAM_REQUESTED_EVENT,
        "XWebProgramRequested(bytes32,uint64,uint8,bytes,uint64)"
    );
    let log = program_log()?;
    assert_eq!(decode_program_requested(&log)?, expected_request());
    let search = replace_word(&log, 0, 2)?;
    assert_eq!(decode_program_requested(&search)?.kind, 2);
    for kind in [0, 3, 255, 256, u64::MAX] {
        assert_eq!(
            decode_program_requested(&replace_word(&log, 0, kind)?),
            Err(EvmError::Malformed)
        );
    }
    for offset in [0, 32, 64, 128, u64::MAX] {
        assert_eq!(
            decode_program_requested(&replace_word(&log, 1, offset)?),
            Err(EvmError::Malformed)
        );
    }
    for length in [0, 33, u64::try_from(MAX_PAYLOAD_BYTES)? + 1, u64::MAX] {
        assert_eq!(
            decode_program_requested(&replace_word(&log, 3, length)?),
            Err(EvmError::Malformed)
        );
    }
    let mut largest = replace_word(&log, 3, u64::try_from(MAX_PAYLOAD_BYTES)?)?;
    let mut data = unhex0x(largest["data"].as_str().ok_or("missing data")?).ok_or("bad data")?;
    data.resize(128 + MAX_PAYLOAD_BYTES, 0x61);
    largest["data"] = json!(hex0x(&data));
    assert_eq!(
        decode_program_requested(&largest)?.payload.len(),
        MAX_PAYLOAD_BYTES
    );
    let mut padding = log.clone();
    let mut data = unhex0x(log["data"].as_str().ok_or("missing data")?).ok_or("bad data")?;
    *data.last_mut().ok_or("empty data")? = 1;
    padding["data"] = json!(hex0x(&data));
    assert_eq!(decode_program_requested(&padding), Err(EvmError::Malformed));
    data.truncate(127);
    padding["data"] = json!(hex0x(&data));
    assert_eq!(decode_program_requested(&padding), Err(EvmError::Malformed));
    let mut overflow = log.clone();
    overflow["topics"][2] = json!(hex0x(&[1; 32]));
    assert_eq!(
        decode_program_requested(&overflow),
        Err(EvmError::Malformed)
    );
    overflow = log;
    overflow["blockNumber"] = json!("0x10000000000000000");
    assert_eq!(
        decode_program_requested(&overflow),
        Err(EvmError::Malformed)
    );
    Ok(())
}

#[test]
fn a_foreign_or_removed_program_log_is_not_a_request() -> Checked {
    let log = program_log()?;
    let mut foreign = log.clone();
    foreign["address"] = json!(hex0x(&[0x11; 20]));
    assert_eq!(
        decode_program_requested(&foreign),
        Err(EvmError::ForeignLog)
    );
    foreign = log.clone();
    foreign["topics"][0] = json!(hex0x(&requested_topic()));
    assert_eq!(
        decode_program_requested(&foreign),
        Err(EvmError::ForeignLog)
    );
    foreign = log.clone();
    foreign["removed"] = json!(true);
    assert_eq!(
        decode_program_requested(&foreign),
        Err(EvmError::ForeignLog)
    );
    assert_eq!(
        decode_program_requested(&replace_word(&log, 3, 64)?),
        Err(EvmError::Malformed)
    );
    let mut malformed = log;
    malformed["topics"]
        .as_array_mut()
        .ok_or("missing topics")?
        .pop();
    assert_eq!(
        decode_program_requested(&malformed),
        Err(EvmError::Malformed)
    );
    Ok(())
}

struct Scratch(PathBuf);

impl Scratch {
    fn new() -> Checked<Self> {
        let path =
            std::env::temp_dir().join(format!("x-websearch-program-watch-{}", std::process::id()));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn read_rpc(stream: &mut TcpStream) -> Checked<Value> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut bytes = Vec::new();
    let mut chunk = [0; 4096];
    loop {
        if let Some(end) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            let head = std::str::from_utf8(&bytes[..end])?;
            let length = head
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())?
                })
                .ok_or("missing content length")?;
            if bytes.len() >= end + 4 + length {
                return Ok(serde_json::from_slice(&bytes[end + 4..end + 4 + length])?);
            }
        }
        let read = stream.read(&mut chunk)?;
        if read == 0 {
            return Err("closed RPC request".into());
        }
        bytes.extend_from_slice(&chunk[..read]);
    }
}

struct Replay {
    address: SocketAddr,
    calls: Arc<Mutex<Vec<Value>>>,
    thread: Option<thread::JoinHandle<CheckedThread>>,
}

type CheckedThread = Result<(), String>;

impl Replay {
    fn start(logs: Vec<Value>) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let calls = Arc::new(Mutex::new(Vec::new()));
        let recorded = recording()?;
        let served = Arc::clone(&calls);
        let handle = thread::spawn(move || -> CheckedThread {
            for incoming in listener.incoming() {
                let mut stream = incoming.map_err(|error| error.to_string())?;
                let request = read_rpc(&mut stream).map_err(|error| error.to_string())?;
                if request["method"] == "stop" {
                    return Ok(());
                }
                served
                    .lock()
                    .map_err(|error| error.to_string())?
                    .push(request.clone());
                let result = match request["method"].as_str() {
                    Some("eth_blockNumber") => {
                        recorded["endpoints"]["evm"]["*"][0]["result"].clone()
                    }
                    Some("eth_chainId") => json!("0xae3f2"),
                    Some("eth_getLogs") => json!(logs),
                    _ => return Err(format!("unexpected RPC: {request}")),
                };
                let body =
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).map_err(|error| error.to_string())?;
            }
            Ok(())
        });
        Ok(Self {
            address,
            calls,
            thread: Some(handle),
        })
    }

    fn rpc(&self) -> Checked<EvmRpc> {
        Ok(EvmRpc::new(&format!("http://{}/", self.address))?)
    }
}

impl Drop for Replay {
    fn drop(&mut self) {
        if let Ok(mut stream) = TcpStream::connect(self.address) {
            let body = r#"{"method":"stop"}"#;
            let _ = write!(
                stream,
                "POST / HTTP/1.1\r\nContent-Length: {}\r\n\r\n{body}",
                body.len()
            );
        }
        if let Some(handle) = self.thread.take() {
            assert!(matches!(handle.join(), Ok(Ok(()))), "RPC replay failed");
        }
    }
}

#[test]
fn the_watcher_journals_program_requests_beside_web_requests() -> Checked {
    let scratch = Scratch::new()?;
    let program = program_log()?;
    let mut other = replace_word(&program, 0, 2)?;
    other["topics"][1] = json!(hex0x(&[0xb0; 32]));
    other["logIndex"] = json!("0x2");
    let node = Replay::start(vec![program.clone(), web_log()?, other.clone()])?;
    let state = scratch.0.join("watch");
    let mut watcher =
        RequestWatcher::open(node.rpc()?, 2, &state, Some(0x19))?.with_chain_id(CHAIN_ID)?;
    let requests = watcher.poll()?;
    assert_eq!(requests, vec![decode_requested(&web_log()?)?]);
    assert_eq!(
        watcher.program_work(),
        vec![expected_request(), decode_program_requested(&other)?]
    );
    assert_eq!(watcher.next_block(), Some(0x1f));
    let journal: Value = serde_json::from_slice(&std::fs::read(state.join("pending.json"))?)?;
    assert_eq!(
        journal["entries"]
            .as_array()
            .ok_or("web entries missing")?
            .len(),
        1
    );
    assert_eq!(
        journal["program_entries"]
            .as_array()
            .ok_or("program entries missing")?
            .len(),
        2
    );
    assert_eq!(journal["program_entries"][0]["block_number"], json!(0x1c));
    assert_eq!(
        journal["program_entries"][0]["source"]["chain_id"],
        json!(CHAIN_ID)
    );
    assert_eq!(
        journal["program_entries"][0]["source"]["block_hash"],
        json!([0x31; 32].to_vec())
    );
    assert_eq!(
        journal["program_entries"][0]["source"]["transaction_hash"],
        json!([0x71; 32].to_vec())
    );
    assert_eq!(
        journal["program_entries"][0]["source"]["log_index"],
        json!(1)
    );
    assert_eq!(watcher.canonical(&requests[0])?, Canonical::Present);
    drop(watcher);
    let mut reopened =
        RequestWatcher::open(node.rpc()?, 2, &state, Some(0))?.with_chain_id(CHAIN_ID)?;
    assert_eq!(reopened.next_block(), Some(0x1f));
    assert_eq!(reopened.work(), requests);
    assert_eq!(
        reopened.program_work(),
        vec![expected_request(), decode_program_requested(&other)?]
    );
    assert!(reopened.poll()?.is_empty());
    reopened.retire_program(expected_request().program_id, 7)?;
    assert_eq!(
        reopened.program_work(),
        vec![decode_program_requested(&other)?]
    );
    reopened.retire(7)?;
    drop(reopened);
    let mut reopened =
        RequestWatcher::open(node.rpc()?, 2, &state, None)?.with_chain_id(CHAIN_ID)?;
    assert!(reopened.work().is_empty());
    assert_eq!(
        reopened.program_work(),
        vec![decode_program_requested(&other)?]
    );
    reopened.retire_program([0xb0; 32], 7)?;
    drop(reopened);
    assert!(RequestWatcher::open(node.rpc()?, 2, &state, None)?
        .program_work()
        .is_empty());
    let calls = node.calls.lock().map_err(|_| "RPC calls lock")?;
    let filters: Vec<_> = calls
        .iter()
        .filter(|call| call["method"] == "eth_getLogs")
        .collect();
    assert_eq!(filters.len(), 2);
    for call in filters {
        assert_eq!(
            call["params"][0]["topics"],
            json!([[hex0x(&requested_topic()), hex0x(&program_requested_topic())]])
        );
        assert_eq!(call["params"][0]["address"], json!(hex0x(&XWEB_PRECOMPILE)));
    }
    assert_eq!(
        calls
            .iter()
            .find(|call| call["method"] == "eth_getLogs")
            .ok_or("no logs call")?["params"][0]["fromBlock"],
        json!("0x19")
    );
    drop(calls);

    let malformed = replace_word(&program, 3, 64)?;
    let invalid_node = Replay::start(vec![web_log()?, malformed])?;
    let invalid_state = scratch.0.join("invalid");
    let mut invalid = RequestWatcher::open(invalid_node.rpc()?, 2, &invalid_state, Some(0x19))?
        .with_chain_id(CHAIN_ID)?;
    assert_eq!(invalid.poll(), Err(EvmError::Malformed));
    assert_eq!(invalid.next_block(), Some(0x19));
    assert!(invalid.work().is_empty());
    assert!(invalid.program_work().is_empty());
    assert!(!invalid_state.join("pending.json").exists());

    // Source metadata must be valid before either kind of work is committed.
    let mut missing_source = program.clone();
    missing_source
        .as_object_mut()
        .ok_or("missing log")?
        .remove("blockHash");
    let missing_node = Replay::start(vec![web_log()?, missing_source])?;
    let missing_state = scratch.0.join("missing-source");
    let mut missing = RequestWatcher::open(missing_node.rpc()?, 2, &missing_state, Some(0x19))?
        .with_chain_id(CHAIN_ID)?;
    assert_eq!(missing.poll(), Err(EvmError::Malformed));
    assert_eq!(missing.next_block(), Some(0x19));
    assert!(missing.work().is_empty());
    assert!(missing.program_work().is_empty());
    assert!(!missing_state.join("pending.json").exists());

    // Opening without a configured chain still records and preserves the
    // program request's chain identity on the first successful poll.
    let unbound_node = Replay::start(vec![program])?;
    let unbound_state = scratch.0.join("unbound");
    let mut unbound = RequestWatcher::open(unbound_node.rpc()?, 2, &unbound_state, Some(0x19))?;
    assert!(unbound.poll()?.is_empty());
    assert_eq!(unbound.program_work(), vec![expected_request()]);
    let journal: Value =
        serde_json::from_slice(&std::fs::read(unbound_state.join("pending.json"))?)?;
    assert_eq!(journal["chain_id"], json!(CHAIN_ID));
    assert_eq!(
        journal["program_entries"][0]["source"]["chain_id"],
        json!(CHAIN_ID)
    );
    drop(unbound);
    assert!(
        RequestWatcher::open(unbound_node.rpc()?, 2, &unbound_state, None)?
            .with_chain_id(CHAIN_ID + 1)
            .is_err()
    );
    let unbound = RequestWatcher::open(unbound_node.rpc()?, 2, &unbound_state, None)?
        .with_chain_id(CHAIN_ID)?;
    assert_eq!(unbound.program_work(), vec![expected_request()]);
    Ok(())
}
