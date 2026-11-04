use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::human_node::HumanNode;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::read::Cursor;
use layerx_client::sequencer::SequencerEndpoint;
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
    // Gateway chain_reads::receipt_keys wraps numeric positive ranges in keys.
    let keys = json!({"keys":[
        {"keyId": format!("0x{}", "01".repeat(32)), "publicKey": format!("0x{}", hex::encode(other_key)), "firstHeight": 1, "lastHeight": 149},
        {"keyId": format!("0x{}", "02".repeat(32)), "publicKey": format!("0x{}", hex::encode(key)), "firstHeight": 150, "lastHeight": 200}
    ]});
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
    // These rows and readiness fields were emitted by the committed indexer
    // producer used by rpc/px_node_root_history_test.go at the node root.
    let recordings: Value = must(serde_json::from_str(include_str!(
        "../../../../../rpc/testdata/px-history/responses.json"
    )));
    let recorded = &recordings[0]["body"];
    let account = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee";
    let rows = recorded["items"].as_array().expect("producer rows")[..2].to_vec();
    let cursor = Cursor {
        block_height: 11,
        tx_index: 0,
        log_index: 0,
    };
    assert_eq!(cursor.to_hex(), "000000000000000b0000000000000000");
    let result = json!({
        "account": account, "items": rows, "next_cursor": cursor.to_hex(),
        "indexedHeight": 12, "chainHeight": 150, "freshness": recorded["freshness"]
    });
    let next = json!({
        "account": account, "items": [], "next_cursor": null,
        "indexedHeight": 12, "chainHeight": 150, "freshness": recorded["freshness"]
    });
    let (url, server) = serve(vec![
        ("eth_chainId", json!("0xe5")),
        ("px_getHistory", result),
        ("px_getHistory", next),
    ]);
    let node = connect(url.clone());
    assert_eq!(must(node.chain_id()), 229);
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    let page = must(layerx_client::rpc::history::history(
        &endpoint,
        account,
        None,
        Some(2),
    ));
    assert_eq!(page.account, account);
    assert_eq!(page.indexed_height, Some(12));
    assert_eq!(page.chain_height, Some(150));
    assert_eq!(page.freshness, recorded["freshness"]);
    assert_eq!(page.items, rows);
    assert_eq!(page.items.len(), 2);
    assert_eq!(page.items[0]["id"], "3");
    assert_eq!(page.items[0]["amount"], "1000000000000000000");
    assert_eq!(page.items[0]["final"], false);
    assert_eq!(page.items[1]["id"], "6");
    assert_eq!(page.items[1]["amount"], "3");
    assert_eq!(page.items[1]["final"], false);
    assert_eq!(page.next_cursor, Some(cursor.to_hex()));
    assert_eq!(
        must(Cursor::from_hex(page.next_cursor.as_deref().unwrap())),
        cursor
    );
    let last = must(layerx_client::rpc::history::history(
        &endpoint,
        account,
        page.next_cursor.as_deref(),
        Some(2),
    ));
    assert_eq!(last.account, account);
    assert_eq!(last.indexed_height, Some(12));
    assert_eq!(last.chain_height, Some(150));
    assert_eq!(last.freshness, recorded["freshness"]);
    assert!(last.items.is_empty());
    assert_eq!(last.next_cursor, None);
    let requests = must(server.join());
    assert_eq!(requests.len(), 3);
    assert_eq!(requests[1]["params"], json!([account, null, 2]));
    assert_eq!(requests[2]["params"], json!([account, cursor.to_hex(), 2]));
    assert!(!include_str!("../src/human_runtime.rs").contains("FrameTransport"));
}

#[test]
fn receipt_keys_refuse_arrays_string_heights_and_zero_heights() {
    let valid = json!({"keyId":format!("0x{}", "02".repeat(32)),
        "publicKey":format!("0x{}", hex::encode(ed25519_dalek::SigningKey::from_bytes(&[42;32]).verifying_key().to_bytes())),
        "firstHeight":150, "lastHeight":200});
    let mut string = valid.clone();
    string["firstHeight"] = json!("0x96");
    let mut zero = valid.clone();
    zero["firstHeight"] = json!(0);
    for keys in [
        json!([valid]),
        json!({"keys":[string]}),
        json!({"keys":[zero]}),
    ] {
        let (url, server) = serve(vec![
            ("eth_chainId", json!("0xe5")),
            ("px_getReceiptKeys", keys),
        ]);
        let node = must(NodeRpc::connect(&NodeRpcConfig {
            url,
            chain_id: 229,
            deadline: Duration::from_secs(2),
        }));
        assert!(node.receipt_keys().is_err());
        assert_eq!(must(server.join()).len(), 2);
    }
}
