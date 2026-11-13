use crate::option_assertion;
use crate::result_assertion;

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use serde_json::{json, Value};

fn fixture(name: &str) -> Value {
    let path = std::path::Path::new(crate::manifest_dir())
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    result_assertion::required(
        serde_json::from_slice(&result_assertion::required(
            std::fs::read(path),
            "producer recording",
        )),
        "recorded JSON",
    )
}

fn read_http(stream: &mut TcpStream) -> (String, BTreeMap<String, String>, Vec<u8>) {
    result_assertion::required(
        stream.set_read_timeout(Some(Duration::from_secs(5))),
        "timeout",
    );
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    result_assertion::required(reader.read_line(&mut line), "HTTP line");
    let first = line.trim_end().to_owned();
    let mut headers = BTreeMap::new();
    loop {
        line.clear();
        assert_ne!(
            result_assertion::required(reader.read_line(&mut line), "HTTP header"),
            0
        );
        if line == "\r\n" {
            break;
        }
        let (name, value) = option_assertion::required(line.split_once(':'), "header delimiter");
        headers.insert(name.to_ascii_lowercase(), value.trim().to_owned());
    }
    let length = headers
        .get("content-length")
        .map_or(0, |v| result_assertion::required(v.parse(), "length"));
    let mut body = vec![0; length];
    result_assertion::required(reader.read_exact(&mut body), "HTTP body");
    (first, headers, body)
}

fn respond(stream: &mut TcpStream, value: &Value) {
    let body = result_assertion::required(serde_json::to_vec(value), "response JSON");
    result_assertion::required(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()), "response header");
    result_assertion::required(stream.write_all(&body), "response body");
}

/// Replays ledger and signed receipt observations emitted by the qualified node producer.
/// The real `SequencerEndpoint` owns all request encoding and response decoding.
pub struct NodeRecording {
    pub endpoint: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl NodeRecording {
    /// # Panics
    /// Panics if the producer fixture, protocol assertion or required local resource is invalid.
    #[must_use]
    pub fn start(account_result: Option<Value>) -> Self {
        let listener =
            result_assertion::required(TcpListener::bind("127.0.0.1:0"), "node listener");
        let endpoint = format!(
            "http://{}",
            result_assertion::required(listener.local_addr(), "node address")
        );
        let calls = Arc::new(Mutex::new(Vec::new()));
        let requests = Arc::clone(&calls);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let metered = fixture("gateway/metered.json");
            let exact = fixture("gateway/exact.json");
            for stream in listener.incoming() {
                let mut stream = result_assertion::required(stream, "node connection");
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let (line, _, body) = read_http(&mut stream);
                assert_eq!(line, "POST / HTTP/1.1", "node uses the merged root");
                let request: Value =
                    result_assertion::required(serde_json::from_slice(&body), "node request");
                result_assertion::required(requests.lock(), "calls").push(request.clone());
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
                        let grant = option_assertion::required(
                            option_assertion::required(
                                metered["ledger"]["grants"].as_array(),
                                "ledger grants",
                            )
                            .iter()
                            .find(|g| {
                                option_assertion::required(g["canonical"].as_str(), "grant")
                                    .get(192..256)
                                    == Some(asset)
                            }),
                            "producer's account for asset",
                        );
                        json!(format!(
                            "0x{}",
                            &option_assertion::required(grant["canonical"].as_str(), "grant")
                                [64..128]
                        ))
                    }
                } else {
                    option_assertion::required(
                        metered["endpoints"]["gateway"]["*"].as_array(),
                        "metered RPC observations",
                    )
                    .iter()
                    .chain(option_assertion::required(
                        exact["endpoints"]["gateway"]["*"].as_array(),
                        "exact RPC observations",
                    ))
                    .find(|r| r["method"] == request["method"] && r["params"] == request["params"])
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
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for NodeRecording {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(option_assertion::required(
            self.endpoint.strip_prefix("http://"),
            "endpoint",
        ));
        result_assertion::required(
            option_assertion::required(self.worker.take(), "node worker").join(),
            "node assertions",
        );
    }
}
