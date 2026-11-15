use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};

use serde_json::{json, Value};
use x_websearch::watch::{EvmError, RequestWatcher};

type Checked<T = ()> = Result<T, Box<dyn std::error::Error>>;

struct Scratch(PathBuf);

impl Scratch {
    fn new(name: &str) -> Checked<Self> {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let path = std::env::temp_dir().join(format!(
            "x-websearch-watch-position-{name}-{}-{}",
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

#[derive(Clone, Copy)]
enum LogsAnswer {
    Empty,
    Pruned { earliest: u64 },
    Error { code: i64 },
}

struct Endpoint {
    address: SocketAddr,
    calls: Arc<Mutex<Vec<Value>>>,
    logs_calls: Arc<AtomicUsize>,
    thread: Option<JoinHandle<()>>,
}

fn read_request(stream: &mut TcpStream) -> Option<Value> {
    let mut bytes = Vec::new();
    let mut chunk = [0_u8; 1024];
    loop {
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = std::str::from_utf8(&bytes[..end]).ok()?;
            let length = head.lines().find_map(|line| {
                let (name, value) = line.split_once(':')?;
                name.eq_ignore_ascii_case("content-length")
                    .then(|| value.trim().parse::<usize>().ok())?
            })?;
            if bytes.len() >= end + 4 + length {
                return serde_json::from_slice(&bytes[end + 4..end + 4 + length]).ok();
            }
        }
        let count = stream.read(&mut chunk).ok()?;
        if count == 0 {
            return None;
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
}

fn write_answer(stream: &mut TcpStream, request: &Value, head: u64, answer: LogsAnswer) {
    let method = request["method"].as_str().unwrap_or_default();
    let result = match method {
        "eth_blockNumber" => Some(json!(format!("0x{head:x}"))),
        "eth_getBlockByNumber" => Some(json!({"number": match request["params"][0].as_str() {
            Some("earliest") => match answer {
                LogsAnswer::Pruned { earliest } => format!("0x{earliest:x}"),
                _ => "0x0".to_owned(),
            },
            _ => "0x0".to_owned(),
        }})),
        "eth_getLogs" => None,
        _ => Some(Value::Null),
    };
    let body = if method == "eth_getLogs" {
        match answer {
            LogsAnswer::Empty => json!({
                "jsonrpc": "2.0",
                "id": request["id"],
                "result": [],
            }),
            LogsAnswer::Pruned { .. } if request["params"][0]["fromBlock"] != json!("0x64") => {
                json!({
                    "jsonrpc": "2.0",
                    "id": request["id"],
                    "error": {"code": -32000, "message": "historical range unavailable"},
                })
            }
            LogsAnswer::Pruned { .. } => json!({
                "jsonrpc": "2.0",
                "id": request["id"],
                "result": [],
            }),
            LogsAnswer::Error { code } => json!({
                "jsonrpc": "2.0",
                "id": request["id"],
                "error": {"code": code, "message": "endpoint failure"},
            }),
        }
    } else {
        json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
    };
    let body = body.to_string();
    let _ = write!(
        stream,
        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    );
}

impl Endpoint {
    fn start(head: u64, answer: LogsAnswer) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let calls = Arc::new(Mutex::new(Vec::new()));
        let logs_calls = Arc::new(AtomicUsize::new(0));
        let thread_calls = Arc::clone(&calls);
        let thread_logs_calls = Arc::clone(&logs_calls);
        let thread = thread::spawn(move || {
            for incoming in listener.incoming() {
                let Ok(mut stream) = incoming else { break };
                let Some(request) = read_request(&mut stream) else {
                    continue;
                };
                if request["method"] == "stop" {
                    break;
                }
                if request["method"] == "eth_getLogs" {
                    thread_logs_calls.fetch_add(1, Ordering::Relaxed);
                }
                if let Ok(mut calls) = thread_calls.lock() {
                    calls.push(request.clone());
                }
                write_answer(&mut stream, &request, head, answer);
            }
        });
        Ok(Self {
            address,
            calls,
            logs_calls,
            thread: Some(thread),
        })
    }

    fn rpc(&self) -> Checked<x_websearch::watch::EvmRpc> {
        Ok(x_websearch::watch::EvmRpc::new(&format!(
            "http://{}/rpc",
            self.address
        ))?)
    }

    fn calls(&self) -> Vec<Value> {
        self.calls
            .lock()
            .map(|calls| calls.clone())
            .unwrap_or_default()
    }
}

impl Drop for Endpoint {
    fn drop(&mut self) {
        let _ = TcpStream::connect(self.address).and_then(|mut stream| {
            let body = r#"{"method":"stop"}"#;
            write!(
                stream,
                "POST / HTTP/1.1\r\nContent-Length: {}\r\n\r\n{body}",
                body.len()
            )
        });
        if let Some(thread) = self.thread.take() {
            let _ = thread.join();
        }
    }
}

fn write_cursor(state: &Path, block: u64) -> Checked {
    std::fs::create_dir_all(state)?;
    std::fs::write(state.join("cursor"), block.to_string())?;
    Ok(())
}

#[test]
fn watch_position_cursor_without_journal_continues_at_cursor() -> Checked {
    let scratch = Scratch::new("cursor")?;
    let state = scratch.0.join("watch");
    write_cursor(&state, 41)?;
    let endpoint = Endpoint::start(100, LogsAnswer::Empty)?;
    let mut watcher = RequestWatcher::open(endpoint.rpc()?, 0, &state, Some(7))?;
    assert_eq!(watcher.next_block(), Some(41));
    watcher.poll()?;
    let logs = endpoint
        .calls()
        .into_iter()
        .find(|call| call["method"] == "eth_getLogs")
        .ok_or("missing logs call")?;
    assert_eq!(logs["params"][0]["fromBlock"], json!("0x29"));
    Ok(())
}

#[test]
fn watch_position_journal_continues_at_next_block() -> Checked {
    let scratch = Scratch::new("journal")?;
    let state = scratch.0.join("watch");
    let endpoint = Endpoint::start(100, LogsAnswer::Empty)?;
    let mut watcher = RequestWatcher::open(endpoint.rpc()?, 0, &state, Some(41))?;
    watcher.poll()?;
    assert_eq!(watcher.next_block(), Some(101));
    drop(watcher);
    let reopened = RequestWatcher::open(endpoint.rpc()?, 0, &state, Some(7))?;
    assert_eq!(reopened.next_block(), Some(101));
    Ok(())
}

#[test]
fn watch_position_pruned_range_continues_at_earliest_served_block() -> Checked {
    let scratch = Scratch::new("pruned")?;
    let state = scratch.0.join("watch");
    write_cursor(&state, 40)?;
    let endpoint = Endpoint::start(100, LogsAnswer::Pruned { earliest: 100 })?;
    let mut watcher = RequestWatcher::open(endpoint.rpc()?, 0, &state, None)?;
    assert!(watcher.poll()?.is_empty());
    assert_eq!(watcher.next_block(), Some(101));
    assert_eq!(watcher.skipped_blocks(), 60);
    assert_eq!(endpoint.logs_calls.load(Ordering::Relaxed), 2);
    let logs = endpoint
        .calls()
        .into_iter()
        .filter(|call| call["method"] == "eth_getLogs")
        .collect::<Vec<_>>();
    assert_eq!(logs[0]["params"][0]["fromBlock"], json!("0x28"));
    assert_eq!(logs[1]["params"][0]["fromBlock"], json!("0x64"));
    Ok(())
}

#[test]
fn watch_position_other_endpoint_error_leaves_position_unchanged() -> Checked {
    let scratch = Scratch::new("error")?;
    let state = scratch.0.join("watch");
    write_cursor(&state, 40)?;
    let endpoint = Endpoint::start(100, LogsAnswer::Error { code: -32603 })?;
    let mut watcher = RequestWatcher::open(endpoint.rpc()?, 0, &state, None)?;
    assert_eq!(watcher.poll(), Err(EvmError::Rejected { code: -32603 }));
    assert_eq!(watcher.next_block(), Some(40));
    assert_eq!(std::fs::read_to_string(state.join("cursor"))?, "40");
    Ok(())
}
