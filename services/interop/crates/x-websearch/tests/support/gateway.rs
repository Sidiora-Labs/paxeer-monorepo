use serde_json::{json, Value};
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;
use x_websearch::watch::{hex0x, keccak, GET_KERNEL_NETWORK_ID_SIGNATURE, XWEB_PRECOMPILE};

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures")
}

/// A loopback JSON-RPC gateway recording every call. It answers
/// recorded EVM calls, optionally serves the settlement chain and receipt probe, and rejects
/// other methods with a method-not-found error.
pub(crate) struct Gateway {
    address: SocketAddr,
    pub(crate) calls: Arc<Mutex<Vec<(String, Value)>>>,
    stop: Arc<AtomicBool>,
    acceptor: Option<thread::JoinHandle<()>>,
}

fn read_body(stream: &mut TcpStream) -> Option<Vec<u8>> {
    stream.set_read_timeout(Some(Duration::from_secs(5))).ok()?;
    let mut bytes = Vec::new();
    let mut chunk = [0_u8; 1_024];
    loop {
        if let Some(end) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            let head = String::from_utf8_lossy(&bytes[..end]).to_ascii_lowercase();
            let length = head
                .split("\r\n")
                .find_map(|line| line.strip_prefix("content-length:"))
                .and_then(|value| value.trim().parse::<usize>().ok())
                .unwrap_or(0);
            if bytes.len() >= end + 4 + length {
                return Some(bytes[end + 4..end + 4 + length].to_vec());
            }
        }
        match stream.read(&mut chunk) {
            Ok(0) | Err(_) => return None,
            Ok(count) => bytes.extend_from_slice(&chunk[..count]),
        }
    }
}

fn answer_rpc(
    stream: &mut TcpStream,
    calls: &Mutex<Vec<(String, Value)>>,
    chain_id: Option<u64>,
    ledger_calls: &[Value],
) {
    let Some(request) =
        read_body(stream).and_then(|body| serde_json::from_slice::<Value>(&body).ok())
    else {
        return;
    };
    let method = request["method"].as_str().unwrap_or_default().to_owned();
    let params = request["params"].clone();
    let reply = if method == "eth_chainId" && chain_id.is_some() {
        json!({
            "jsonrpc": "2.0",
            "id": request["id"],
            "result": chain_id.map(|id| format!("0x{id:x}")),
        })
    } else if method == "pax_sequencerReceipt" && chain_id.is_some() {
        json!({ "jsonrpc": "2.0", "id": request["id"], "result": null })
    } else if method == "eth_call"
        && chain_id.is_some()
        && params
            == json!([{
            "to": hex0x(&XWEB_PRECOMPILE),
            "data": hex0x(&keccak(GET_KERNEL_NETWORK_ID_SIGNATURE.as_bytes())[..4]),
        }, "latest"])
    {
        let mut network_id = [0_u8; 32];
        network_id[31] = 1;
        json!({ "jsonrpc": "2.0", "id": request["id"], "result": hex0x(&network_id) })
    } else if let Some(recorded) = ledger_calls
        .iter()
        .find(|recorded| recorded["method"] == method && recorded["params"] == params)
    {
        json!({ "jsonrpc": "2.0", "id": request["id"], "result": recorded["result"] })
    } else {
        json!({
            "jsonrpc": "2.0",
            "id": request["id"],
            "error": { "code": -32_601, "message": "method not found" },
        })
    };
    if let Ok(mut calls) = calls.lock() {
        calls.push((method, params));
    }
    let body = reply.to_string();
    let _ = stream.write_all(
        format!(
            "HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{body}",
            body.len()
        )
        .as_bytes(),
    );
}

impl Gateway {
    pub(crate) fn start_with_chain_id(chain_id: Option<u64>) -> std::io::Result<Self> {
        let mut ledger_calls = if chain_id.is_some() {
            let recording: Value =
                serde_json::from_slice(&std::fs::read(fixtures().join("gateway/metered.json"))?)?;
            recording["endpoints"]["gateway"]["*"]
                .as_array()
                .ok_or_else(|| std::io::Error::other("missing recorded ledger calls"))?
                .iter()
                .filter(|call| {
                    call["method"] == "eth_call"
                        && call["params"][0]["data"].as_str().is_some_and(|data| {
                            data.starts_with("0x3f050cbc") || data.starts_with("0xd3600b24")
                        })
                })
                .cloned()
                .collect::<Vec<_>>()
        } else {
            Vec::new()
        };
        if chain_id.is_some() {
            let mut payer_calls = Vec::new();
            for call in &ledger_calls {
                let data = call["params"][0]["data"]
                    .as_str()
                    .ok_or_else(|| std::io::Error::other("missing recorded ledger calldata"))?;
                if let Some(arguments) = data.strip_prefix("0x3f050cbc") {
                    let asset = arguments
                        .get(64..)
                        .ok_or_else(|| std::io::Error::other("missing recorded ledger asset"))?;
                    let owner = format!("{:0>64}", "abababababababababababababababababababab");
                    let account = layerx_x402::model::account_identifiers(&format!(
                        "agent:did:layerx:{owner}:asset:{asset}"
                    ))
                    .map_err(std::io::Error::other)?[1];
                    let mut payer = call.clone();
                    payer["params"][0]["data"] = json!(format!("0x3f050cbc{owner}{asset}"));
                    payer["result"] = json!(hex0x(&account));
                    payer_calls.push(payer);
                }
            }
            ledger_calls.extend(payer_calls);
            for name in ["chain.json", "watch.json"] {
                let recording: Value =
                    serde_json::from_slice(&std::fs::read(fixtures().join("evm").join(name))?)?;
                ledger_calls.extend(
                    recording["endpoints"]["evm"]["*"]
                        .as_array()
                        .ok_or_else(|| std::io::Error::other("missing recorded EVM calls"))?
                        .iter()
                        .filter(|call| call["method"] != "eth_getLogs")
                        .cloned(),
                );
            }
        }
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let calls = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let (thread_calls, thread_stop) = (Arc::clone(&calls), Arc::clone(&stop));
        let acceptor = thread::spawn(move || {
            for stream in listener.incoming() {
                if thread_stop.load(Ordering::SeqCst) {
                    break;
                }
                if let Ok(mut stream) = stream {
                    answer_rpc(&mut stream, &thread_calls, chain_id, &ledger_calls);
                }
            }
        });
        Ok(Self {
            address,
            calls,
            stop,
            acceptor: Some(acceptor),
        })
    }

    pub(crate) fn url(&self) -> String {
        format!("http://{}/rpc", self.address)
    }

    pub(crate) fn params(&self, method: &str) -> Vec<Value> {
        self.calls
            .lock()
            .map(|calls| {
                calls
                    .iter()
                    .filter(|(name, _)| name == method)
                    .map(|(_, params)| params.clone())
                    .collect()
            })
            .unwrap_or_default()
    }
}

impl Drop for Gateway {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        drop(TcpStream::connect_timeout(
            &self.address,
            Duration::from_secs(1),
        ));
        if let Some(acceptor) = self.acceptor.take() {
            let _ = acceptor.join();
        }
    }
}

#[test]
fn gateway_records_rpc_parameters() -> std::io::Result<()> {
    let gateway = Gateway::start_with_chain_id(None)?;
    let mut stream = TcpStream::connect(gateway.address)?;
    let body = json!({"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}).to_string();
    write!(
        stream,
        "POST /rpc HTTP/1.1\r\nHost: localhost\r\nContent-Length: {}\r\n\r\n{body}",
        body.len()
    )?;
    let mut response = Vec::new();
    stream.read_to_end(&mut response)?;
    assert!(response.starts_with(b"HTTP/1.1 200 OK"));
    assert_eq!(gateway.params("eth_chainId"), vec![json!([])]);
    assert_eq!(
        gateway
            .calls
            .lock()
            .map_err(|_| std::io::Error::other("gateway call state"))?
            .len(),
        1
    );
    Ok(())
}
