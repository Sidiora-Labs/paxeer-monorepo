use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::process::Command;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig, NodeRpcError};
use layerx_client::sequencer::CallRequest;
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

fn config(url: String) -> NodeRpcConfig {
    NodeRpcConfig {
        url,
        chain_id: 125,
        deadline: Duration::from_secs(2),
    }
}

#[test]
fn node_rpc_refuses_a_chain_id_mismatch() {
    let (url, server) = serve(vec![("eth_chainId", json!("0x7e"))]);
    let error = must(
        NodeRpc::connect(&config(url))
            .err()
            .ok_or("mismatch was accepted"),
    );
    assert_eq!(
        error,
        NodeRpcError::ChainIdMismatch {
            expected: 125,
            observed: 126
        }
    );
    assert_eq!(error.to_string(), "node chain id mismatch");
    let requests = must(server.join());
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0]["params"], json!([]));

    let hash = format!("0x{}", "ab".repeat(32));
    let (url, server) = serve(vec![
        ("eth_chainId", json!("0x7d")),
        ("eth_getTransactionCount", json!("0x7")),
        ("eth_estimateGas", json!("0x5208")),
        ("eth_call", json!("0x1234")),
        ("eth_sendRawTransaction", json!(hash)),
        ("pax_sequencerReceipt", Value::Null),
        ("px_getReceiptKeys", json!([])),
        ("eth_getBlockByNumber", Value::Null),
    ]);
    let node = must(NodeRpc::connect(&config(url)));
    assert_eq!(node.chain_id(), 125);
    assert_eq!(must(node.pending_nonce(&[1; 20])), 7);
    assert_eq!(must(node.estimate_gas(&CallRequest::default())), 21_000);
    assert_eq!(must(node.call_at(42, &[2; 20], &[3])), vec![0x12, 0x34]);
    assert_eq!(must(node.send_raw_transaction(&[4, 5])), [0xab; 32]);
    assert_eq!(must(node.sequencer_receipt(&[0xab; 32])), None);
    assert!(must(node.receipt_keys()).is_empty());
    assert_eq!(must(node.block_by_number(42)), None);
    let requests = must(server.join());
    assert_eq!(requests.len(), 8);
    assert_eq!(
        requests[1]["params"],
        json!([format!("0x{}", "01".repeat(20)), "pending"])
    );
    assert_eq!(requests[2]["params"], json!([{"data": "0x"}, "pending"]));
    assert_eq!(
        requests[3]["params"],
        json!([{"to": format!("0x{}", "02".repeat(20)), "data": "0x03"}, "0x2a"])
    );
    assert_eq!(requests[4]["params"], json!(["0x0405"]));
    assert_eq!(requests[5]["params"], json!([hash]));
    assert_eq!(requests[6]["params"], json!([]));
    assert_eq!(requests[7]["params"], json!(["0x2a", true]));
}

#[test]
fn node_rpc_reports_unavailable_for_a_closed_port() {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    drop(listener);
    let config = config(url);
    let started = Instant::now();
    let error = must(
        NodeRpc::connect(&config)
            .err()
            .ok_or("closed port was accepted"),
    );
    assert_eq!(error, NodeRpcError::Unavailable);
    assert_eq!(error.to_string(), "node JSON-RPC is unavailable");
    assert!(started.elapsed() < config.deadline);
}

#[test]
fn node_rpc_config_requires_both_variables() {
    const CASE: &str = "LAYERX_NODE_RPC_CONFIG_TEST_CASE";
    if let Ok(case) = std::env::var(CASE) {
        match case.as_str() {
            "url" => {
                let error = must(
                    NodeRpcConfig::from_environment()
                        .err()
                        .ok_or("missing URL was accepted"),
                );
                assert_eq!(
                    error,
                    NodeRpcError::MissingVariable("LAYERX_AGENT_NODE_RPC_URL")
                );
                assert!(error.to_string().contains("LAYERX_AGENT_NODE_RPC_URL"));
            }
            "chain" => {
                let error = must(
                    NodeRpcConfig::from_environment()
                        .err()
                        .ok_or("missing chain id was accepted"),
                );
                assert_eq!(
                    error,
                    NodeRpcError::MissingVariable("LAYERX_AGENT_NODE_CHAIN_ID")
                );
                assert!(error.to_string().contains("LAYERX_AGENT_NODE_CHAIN_ID"));
            }
            "invalid" => assert_eq!(
                NodeRpcConfig::from_environment(),
                Err(NodeRpcError::InvalidVariable("LAYERX_AGENT_NODE_CHAIN_ID"))
            ),
            "valid" => {
                let config = must(NodeRpcConfig::from_environment());
                assert_eq!(config.url, "http://127.0.0.1:8545/");
                assert_eq!(config.chain_id, 125);
                assert_eq!(config.deadline, Duration::from_secs(30));
            }
            _ => panic!("unexpected case"),
        }
        return;
    }
    for (case, url, chain) in [
        ("url", None, None),
        ("url", None, Some("125")),
        ("chain", Some("http://127.0.0.1:8545/"), None),
        ("invalid", Some("http://127.0.0.1:8545/"), Some("wrong")),
        ("valid", Some("http://127.0.0.1:8545/"), Some("125")),
        ("valid", Some("http://127.0.0.1:8545/"), Some("0x7d")),
    ] {
        let mut command = Command::new(must(std::env::current_exe()));
        command
            .args([
                "--exact",
                "node_rpc_config_requires_both_variables",
                "--nocapture",
            ])
            .env(CASE, case)
            .env_remove("LAYERX_AGENT_NODE_RPC_URL")
            .env_remove("LAYERX_AGENT_NODE_CHAIN_ID");
        if let Some(url) = url {
            command.env("LAYERX_AGENT_NODE_RPC_URL", url);
        }
        if let Some(chain) = chain {
            command.env("LAYERX_AGENT_NODE_CHAIN_ID", chain);
        }
        let output = must(command.output());
        assert!(
            output.status.success(),
            "{case}: {}{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
    }
}
