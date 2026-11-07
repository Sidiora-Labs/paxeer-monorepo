use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::read::{
    availability, AvailabilityRead, AvailabilityReadError, AvailabilityRequest,
};
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use serde_json::{json, Value};

pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

pub const BLOCK_HEIGHT: u64 = 42;
pub const PRUNED_HEIGHT: u64 = 41;
pub const HEAD: u64 = 43;

pub type Exchange = (&'static str, Value, Value);

pub fn block(height: u64) -> Value {
    let hash = format!("0x{}", "ab".repeat(32));
    let transactions: Vec<Value> = (0..if height == 0 { 0 } else { 2 })
        .map(|index| {
            json!({
                "blockHash": hash,
                "blockNumber": format!("{height:#x}"),
                "from": format!("0x{}", "11".repeat(20)),
                "gas": "0x5208",
                "gasPrice": "0x1",
                "hash": format!("0x{index:064x}"),
                "input": "0x",
                "nonce": format!("{index:#x}"),
                "to": format!("0x{}", "22".repeat(20)),
                "transactionIndex": format!("{index:#x}"),
                "value": "0x1",
                "type": "0x0",
                "v": "0x1b",
                "r": "0x1",
                "s": "0x2"
            })
        })
        .collect();
    json!({
        "number": format!("{height:#x}"),
        "hash": hash,
        "parentHash": format!("0x{}", "cd".repeat(32)),
        "timestamp": "0x1234",
        "transactionsRoot": format!("0x{}", "33".repeat(32)),
        "receiptsRoot": format!("0x{}", "44".repeat(32)),
        "transactions": transactions
    })
}

pub fn head_exchange() -> Exchange {
    ("eth_blockNumber", json!([]), json!(format!("{HEAD:#x}")))
}

pub fn block_exchange(height: u64, full: bool, result: Value) -> Exchange {
    let result = if full || result.is_null() {
        result
    } else {
        let mut header = result;
        let hashes = header["transactions"]
            .as_array()
            .unwrap_or_else(|| panic!("missing transactions"))
            .iter()
            .map(|transaction| transaction["hash"].clone())
            .collect::<Vec<_>>();
        header["transactions"] = json!(hashes);
        header
    };
    (
        "eth_getBlockByNumber",
        json!([format!("{height:#x}"), full]),
        result,
    )
}

pub fn available_exchanges(height: u64) -> Vec<Exchange> {
    vec![
        block_exchange(height, false, block(height)),
        head_exchange(),
        block_exchange(height, true, block(height)),
    ]
}

pub struct Node {
    pub endpoint: SequencerEndpoint,
    url: String,
    agent: ureq::Agent,
    server: JoinHandle<()>,
}

impl Node {
    pub fn connect(exchanges: Vec<Exchange>) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}/", must(listener.local_addr()));
        must(listener.set_nonblocking(true));
        let server = thread::spawn(move || {
            for (method, params, result) in exchanges {
                let expires = Instant::now() + Duration::from_secs(5);
                let stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(Instant::now() < expires, "missing {method} request");
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("accept: {error}"),
                    }
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(2))));
                must(stream.set_write_timeout(Some(Duration::from_secs(2))));
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
                let length = length.unwrap_or_else(|| panic!("content length required"));
                assert!(length <= 64 * 1024);
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                assert_eq!(request["method"], method);
                assert_eq!(request["params"], params);
                let answer = json!({
                    "jsonrpc": "2.0", "id": request["id"], "result": result
                })
                .to_string();
                let mut stream = reader.into_inner();
                must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
                must(stream.flush());
            }
        });
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let agent = ureq::Agent::config_builder()
            .timeout_global(Some(Duration::from_secs(10)))
            .max_redirects(0)
            .build()
            .into();
        Self {
            endpoint,
            url,
            agent,
            server,
        }
    }

    pub fn finish(self) {
        must(self.server.join());
    }

    pub fn rpc(&self, method: &str, params: Value) -> Value {
        let mut response = must(self.agent.post(&self.url).send_json(json!({
            "jsonrpc": "2.0", "id": 1, "method": method, "params": params
        })));
        assert_eq!(response.status().as_u16(), 200);
        let bytes = must(
            response
                .body_mut()
                .with_config()
                .limit(16 * 1_048_576)
                .read_to_vec(),
        );
        let answer: Value = must(serde_json::from_slice(&bytes));
        assert_eq!(answer["jsonrpc"], "2.0");
        assert_eq!(answer["id"], 1);
        assert!(answer.get("error").is_none(), "RPC refused: {answer}");
        answer
            .get("result")
            .unwrap_or_else(|| panic!("missing result: {answer}"))
            .clone()
    }

    pub fn head(&self) -> u64 {
        let answer = self.rpc("eth_blockNumber", json!([]));
        let text = answer
            .as_str()
            .unwrap_or_else(|| panic!("invalid head: {answer}"));
        let digits = text
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("invalid head: {text}"));
        must(u64::from_str_radix(digits, 16))
    }

    pub fn block(&self, height: u64) -> Value {
        self.rpc(
            "eth_getBlockByNumber",
            json!([format!("{height:#x}"), false]),
        )
    }
}

pub fn assert_available(node: &Node, height: u64) -> u64 {
    let block = node.block(height);
    assert!(block.is_object(), "test block must be retained: {height}");
    assert_eq!(block["number"], format!("{height:#x}"));
    let count = u64::try_from(
        block["transactions"]
            .as_array()
            .unwrap_or_else(|| panic!("missing transactions"))
            .len(),
    )
    .unwrap_or_else(|error| panic!("transaction count: {error}"));
    let outcome = must(availability(
        &node.endpoint,
        &AvailabilityRequest { height },
    ));
    assert_eq!(
        outcome,
        AvailabilityRead::Available {
            height,
            transactions_root: must(hex::decode(
                block["transactionsRoot"]
                    .as_str()
                    .unwrap_or_else(|| panic!("missing transactions root"))
                    .strip_prefix("0x")
                    .unwrap_or_else(|| panic!("invalid transactions root"))
            ))
            .try_into()
            .unwrap_or_else(|_| panic!("transactions root length")),
            receipts_root: must(hex::decode(
                block["receiptsRoot"]
                    .as_str()
                    .unwrap_or_else(|| panic!("missing receipts root"))
                    .strip_prefix("0x")
                    .unwrap_or_else(|| panic!("invalid receipts root"))
            ))
            .try_into()
            .unwrap_or_else(|_| panic!("receipts root length")),
            transaction_count: count,
        }
    );
    count
}

#[test]
fn available_block_matches_node_header() {
    let height = BLOCK_HEIGHT;
    let node = Node::connect(available_exchanges(height));
    assert!(
        assert_available(&node, height) > 0,
        "select a block containing transactions"
    );
    node.finish();
}

#[test]
fn genesis_availability_matches_node_header() {
    let node = Node::connect(available_exchanges(0));
    assert_available(&node, 0);
    node.finish();
}

#[test]
fn repeated_availability_reads_preserve_block_metadata() {
    let mut exchanges = vec![
        head_exchange(),
        block_exchange(BLOCK_HEIGHT, true, block(BLOCK_HEIGHT)),
        head_exchange(),
        block_exchange(BLOCK_HEIGHT, true, block(BLOCK_HEIGHT)),
    ];
    exchanges.extend(available_exchanges(BLOCK_HEIGHT));
    let node = Node::connect(exchanges);
    let request = AvailabilityRequest {
        height: BLOCK_HEIGHT,
    };
    let first = must(availability(&node.endpoint, &request));
    assert!(matches!(first, AvailabilityRead::Available { .. }));
    assert_eq!(must(availability(&node.endpoint, &request)), first);
    assert_available(&node, request.height);
    node.finish();
}

#[test]
fn transport_failure_is_not_reported_as_pruned() {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    drop(listener);
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    assert_eq!(
        availability(&endpoint, &AvailabilityRequest { height: 0 }),
        Err(AvailabilityReadError::Sequencer(
            SequencerError::Unavailable
        ))
    );
}
