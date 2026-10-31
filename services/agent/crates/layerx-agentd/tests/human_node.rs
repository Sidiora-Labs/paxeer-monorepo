use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::human_node::HumanNode;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::read::{history, Cursor, HistoryLimits};
use layerx_client::read::HistoryKind;
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let handle = thread::spawn(move || {
        let mut requests = Vec::new();
        for (method, result) in exchanges {
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
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
            requests.push(request);
        }
        requests
    });
    (url, handle)
}

fn connect(url: String) -> Box<dyn HumanNode> {
    Box::new(must(NodeRpc::connect(&NodeRpcConfig {
        url,
        chain_id: 229,
        deadline: Duration::from_secs(2),
    })))
}

fn block() -> Value {
    json!({
        "number": "0x96", "hash": format!("0x{}", "11".repeat(32)),
        "parentHash": format!("0x{}", "22".repeat(32)), "timestamp": "0x1234",
        "transactionsRoot": format!("0x{}", "33".repeat(32)),
        "receiptsRoot": format!("0x{}", "44".repeat(32)), "transactions": []
    })
}

#[test]
fn human_node_head_and_receipt_keys_come_from_json_rpc() {
    let key = ed25519_dalek::SigningKey::from_bytes(&[42; 32])
        .verifying_key()
        .to_bytes();
    let other_key = ed25519_dalek::SigningKey::from_bytes(&[43; 32])
        .verifying_key()
        .to_bytes();
    let keys = json!([
        {"keyId": format!("0x{}", "01".repeat(32)), "publicKey": format!("0x{}", hex::encode(other_key)), "firstHeight": "0x0", "lastHeight": "0x95"},
        {"keyId": format!("0x{}", "02".repeat(32)), "publicKey": format!("0x{}", hex::encode(key)), "firstHeight": "0x96", "lastHeight": "0xc8"}
    ]);
    let (url, server) = serve(vec![
        ("eth_chainId", json!("0xe5")),
        ("eth_blockNumber", json!("0x96")),
        ("eth_getBlockByNumber", block()),
        ("eth_blockNumber", json!("0x96")),
        ("eth_getBlockByNumber", block()),
        ("px_getReceiptKeys", keys),
    ]);
    let node = connect(url);
    assert_eq!(must(node.chain_id()), 229);
    let head = must(node.head());
    assert_eq!(head.number, 150);
    assert_eq!(head.hash, [0x11; 32]);
    assert_eq!(head.timestamp, 0x1234);
    let receipt_key = must(node.receipt_key_at_head());
    assert_eq!(receipt_key.public_key, key);
    assert_eq!(receipt_key.key_id, [2; 32]);
    assert_eq!(
        (receipt_key.first_height, receipt_key.last_height),
        (150, 200)
    );
    let requests = must(server.join());
    assert_eq!(requests.len(), 6);
    for index in [2, 4] {
        assert_eq!(requests[index]["params"], json!(["0x96", true]));
    }
    for index in [0, 1, 3, 5] {
        assert_eq!(requests[index]["params"], json!([]));
    }
}

#[test]
fn human_history_pages_px_get_history_without_frame_transport() {
    let result = json!({"height":"150", "items":[
        {"height":"100", "txIndex":"0", "logIndex":"1", "kind":"Activity", "canonical":"0102", "proof":null},
        {"height":"150", "txIndex":"2", "logIndex":"3", "kind":"Receipt", "canonical":"0304", "proof":"0506"}
    ], "cursor":["150","2","3"]});
    let next = json!({"height":"150", "items":[], "cursor":null});
    let (url, server) = serve(vec![
        ("eth_chainId", json!("0xe5")),
        ("px_getHistory", result),
        ("px_getHistory", next),
    ]);
    let mut node = connect(url);
    let limits = HistoryLimits {
        maximum_items: 2,
        maximum_response_bytes: 4096,
    };
    let page = must(history(node.as_mut(), 100, 150, None, limits, None));
    assert_eq!(page.height, 150);
    assert_eq!(page.items.len(), 2);
    assert_eq!(page.items[0].kind, HistoryKind::Activity);
    assert_eq!(page.items[0].canonical, [1, 2]);
    assert_eq!(page.items[0].proof, None);
    assert_eq!(page.items[1].kind, HistoryKind::Receipt);
    assert_eq!(page.items[1].canonical, [3, 4]);
    assert_eq!(page.items[1].proof, Some(vec![5, 6]));
    let cursor = Cursor {
        block_height: 150,
        tx_index: 2,
        log_index: 3,
    };
    assert_eq!(page.next_cursor, Some(cursor));
    assert!(page.response_bytes > 0 && page.response_bytes <= 4096);
    let last = must(history(
        node.as_mut(),
        100,
        150,
        page.next_cursor,
        limits,
        None,
    ));
    assert!(last.items.is_empty());
    assert_eq!(last.next_cursor, None);
    let requests = must(server.join());
    assert_eq!(requests.len(), 3);
    assert_eq!(
        requests[1]["params"],
        json!([{"from_height":"100", "to_height":"150", "limit":"2", "cursor":null}])
    );
    assert_eq!(
        requests[2]["params"],
        json!([{"from_height":"100", "to_height":"150", "limit":"2", "cursor":["150","2","3"]}])
    );
    assert!(!include_str!("../src/human_runtime.rs").contains("FrameTransport"));
}
