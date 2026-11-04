use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_agent_api::error::RequestId;
use layerx_agentd::policy::approval::{ApprovalContext, ApprovalRegistry};
use layerx_agentd::prepare::evm::{self, PrecompileCall, PrecompileTarget};
use layerx_agentd::tenant::Operation;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
use layerx_crypto::payments::Payment;
use layerx_mcp::approval::ApprovalPolicy;
use layerx_mcp::tools::web::{
    web_tool, ExactPayment, GrantTerms, WebApproval, WebCall, WebConfig, WebPayer, WebPayerError,
    WebResult,
};
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_types::payload::{ModuleId, ModuleRegistry};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

fn fixture(name: &str) -> Value {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    serde_json::from_slice(&std::fs::read(path).expect("producer recording"))
        .expect("recorded JSON")
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("hex byte"))
        .collect()
}

fn word(value: &Value) -> [u8; 32] {
    unhex(value.as_str().expect("hex word"))
        .try_into()
        .expect("32 bytes")
}

fn read_http(stream: &mut TcpStream) -> (String, BTreeMap<String, String>, Vec<u8>) {
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("timeout");
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader.read_line(&mut line).expect("HTTP line");
    let first = line.trim_end().to_owned();
    let mut headers = BTreeMap::new();
    loop {
        line.clear();
        assert_ne!(reader.read_line(&mut line).expect("HTTP header"), 0);
        if line == "\r\n" {
            break;
        }
        let (name, value) = line.split_once(':').expect("header delimiter");
        headers.insert(name.to_ascii_lowercase(), value.trim().to_owned());
    }
    let length = headers
        .get("content-length")
        .map_or(0, |v| v.parse().expect("length"));
    let mut body = vec![0; length];
    reader.read_exact(&mut body).expect("HTTP body");
    (first, headers, body)
}

fn respond(stream: &mut TcpStream, value: &Value) {
    let body = serde_json::to_vec(value).expect("response JSON");
    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("response header");
    stream.write_all(&body).expect("response body");
}

/// Replays ledger and signed receipt observations emitted by the qualified node producer.
/// The real SequencerEndpoint owns all request encoding and response decoding.
pub struct NodeRecording {
    pub endpoint: String,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl NodeRecording {
    pub fn start(account_result: Option<Value>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("node listener");
        let endpoint = format!("http://{}", listener.local_addr().expect("node address"));
        let calls = Arc::new(Mutex::new(Vec::new()));
        let requests = Arc::clone(&calls);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let metered = fixture("gateway/metered.json");
            let exact = fixture("gateway/exact.json");
            for stream in listener.incoming() {
                let mut stream = stream.expect("node connection");
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let (line, _, body) = read_http(&mut stream);
                assert_eq!(line, "POST / HTTP/1.1", "node uses the merged root");
                let request: Value = serde_json::from_slice(&body).expect("node request");
                requests.lock().expect("calls").push(request.clone());
                let data = request["params"][0]["data"].as_str().unwrap_or("");
                let result = if request["method"] == "eth_call" && data.starts_with("0x3f050cbc") {
                    assert_eq!(
                        request["params"][0]["to"],
                        "0x000000000000000000000000000000000000101a"
                    );
                    assert_eq!(request["params"][1], "pending");
                    assert_eq!(
                        &data[10..74],
                        format!("{}{}", "0".repeat(24), "ab".repeat(20))
                    );
                    if let Some(result) = &account_result {
                        result.clone()
                    } else {
                        let asset = &data[74..];
                        let grant = metered["ledger"]["grants"]
                            .as_array()
                            .expect("ledger grants")
                            .iter()
                            .find(|g| {
                                g["canonical"].as_str().expect("grant").get(192..256) == Some(asset)
                            })
                            .expect("producer's account for asset");
                        json!(format!(
                            "0x{}",
                            &grant["canonical"].as_str().expect("grant")[64..128]
                        ))
                    }
                } else {
                    metered["endpoints"]["gateway"]["*"]
                        .as_array()
                        .expect("metered RPC observations")
                        .iter()
                        .chain(
                            exact["endpoints"]["gateway"]["*"]
                                .as_array()
                                .expect("exact RPC observations"),
                        )
                        .find(|r| {
                            r["method"] == request["method"] && r["params"] == request["params"]
                        })
                        .unwrap_or_else(|| panic!("unrecorded node operation: {request}"))["result"]
                        .clone()
                };
                respond(
                    &mut stream,
                    &json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
                );
            }
        });
        Self {
            endpoint,
            calls,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for NodeRecording {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.endpoint.strip_prefix("http://").expect("endpoint"));
        self.worker
            .take()
            .expect("node worker")
            .join()
            .expect("node assertions");
    }
}
