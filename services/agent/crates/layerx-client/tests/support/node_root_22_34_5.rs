use layerx_client::px::{PxEndpoint, NODE_READ_METHODS};
use layerx_client::sequencer::{RpcRefusal, SequencerError};
use serde_json::{json, Value};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread::JoinHandle;
use std::time::Duration;

pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

pub fn recording() -> Value {
    must(serde_json::from_str(include_str!(
        "../fixtures/repair_22_34_5_node_root.json"
    )))
}

pub fn producer() -> Value {
    must(serde_json::from_str(include_str!(
        "../fixtures/repair_22_33_1546_node.json"
    )))
}

pub fn gateway_reads() -> Value {
    must(serde_json::from_str(include_str!(
        "../fixtures/repair_22_33_1546_gateway.json"
    )))
}

pub fn unified_history() -> Value {
    must(serde_json::from_str(include_str!(
        "../../../../../../rpc/testdata/px-history/gateway-recording.json"
    )))
}

pub fn hex_bytes<const N: usize>(value: &Value) -> [u8; N] {
    let text = value.as_str().expect("hex string");
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len(), 2 * N);
    std::array::from_fn(|index| must(u8::from_str_radix(&text[index * 2..index * 2 + 2], 16)))
}

pub struct Server {
    pub url: String,
    pub requests: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    join: Option<JoinHandle<()>>,
}

impl Server {
    pub fn start(
        path: &'static str,
        reply: impl Fn(&Value) -> Option<Value> + Send + 'static,
    ) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}{}", must(listener.local_addr()), path);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let seen = requests.clone();
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let join = std::thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                must(stream.set_write_timeout(Some(Duration::from_secs(3))));
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                must(reader.read_line(&mut line));
                assert_eq!(line, format!("POST {path} HTTP/1.1\r\n"));
                let mut length = None;
                loop {
                    line.clear();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
                let length = length.expect("content length");
                assert!(length <= 1_048_576);
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                assert!(request["params"].is_array());
                must(seen.lock()).push(request.clone());
                if let Some(mut response) = reply(&request) {
                    response["jsonrpc"] = json!("2.0");
                    response["id"] = request["id"].clone();
                    let body = response.to_string();
                    let mut stream = reader.into_inner();
                    must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
                    must(stream.flush());
                }
            }
        });
        Self {
            url,
            requests,
            stop,
            join: Some(join),
        }
    }

    pub fn recorded() -> Self {
        let recorded = recording();
        Self::start("/", move |request| {
            let exchange = recorded["exchanges"]
                .as_array()
                .expect("exchanges")
                .iter()
                .find(|entry| entry["request"]["method"] == request["method"])
                .expect("recorded method");
            assert_eq!(request["params"], exchange["request"]["params"]);
            Some(exchange["response"].clone())
        })
    }

    pub fn count(&self) -> usize {
        must(self.requests.lock()).len()
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(join) = self.join.take() {
            let result = join.join();
            if !std::thread::panicking() {
                must(result);
            }
        }
    }
}

pub fn assert_recorded_reads(endpoint: &PxEndpoint) {
    let recorded = recording();
    assert_eq!(recorded["chain_id"], 125);
    assert_eq!(must(endpoint.node().chain_id()), 125);
    let exchanges = recorded["exchanges"].as_array().expect("exchanges");
    assert_eq!(exchanges.len(), NODE_READ_METHODS.len() + 1);
    for method in NODE_READ_METHODS {
        let exchange = exchanges
            .iter()
            .find(|entry| entry["request"]["method"] == method)
            .expect("every served method is recorded");
        let result = endpoint.call(method, &exchange["request"]["params"]);
        let response = &exchange["response"];
        if let Some(error) = response.get("error") {
            assert_eq!(
                result,
                Err(SequencerError::Rpc(RpcRefusal {
                    code: error["code"].as_i64().expect("error code"),
                    message: error["message"].as_str().expect("error message").to_owned(),
                    data: error.get("data").cloned(),
                })),
                "{method}"
            );
        } else {
            assert_eq!(must(result), response["result"], "{method}");
        }
    }
}

pub fn storage_server() -> Server {
    let storage = producer()["storage"].clone();
    Server::start("/", move |request| {
        let height = storage["height"].as_u64().expect("height");
        let result = match request["method"].as_str().expect("method") {
            "eth_chainId" => {
                assert_eq!(request["params"], json!([]));
                json!("0x7d")
            }
            "eth_getBlockByNumber" => {
                assert_eq!(request["params"][0], format!("{height:#x}"));
                assert!(request["params"][1].is_boolean());
                let mut block = storage["block"].clone();
                block["stateRoot"] = storage["appHash"].clone();
                block
            }
            "px_getCommit" => {
                assert_eq!(request["params"], json!([height.to_string()]));
                storage["material"].clone()
            }
            "px_getValidators" => {
                assert_eq!(request["params"], json!([height.to_string()]));
                storage["validators"].clone()
            }
            "eth_getProof" => {
                assert_eq!(
                    request["params"],
                    json!([
                        storage["address"],
                        [storage["slots"][0]],
                        format!("{height:#x}")
                    ])
                );
                storage["responses"][0].clone()
            }
            method => panic!("unrecorded storage method {method}"),
        };
        Some(json!({"result": result}))
    })
}
