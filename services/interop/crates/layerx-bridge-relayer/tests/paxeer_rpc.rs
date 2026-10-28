use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_bridge_relayer::rpc::{JsonRpc, PaxeerRpc, RpcFault};
use layerx_bridge_relayer::transport::PaxeerEndpoint;
use serde_json::{json, Value};

const TIMEOUT: Duration = Duration::from_secs(3);

fn result(value: Value) -> Value {
    json!({"jsonrpc": "2.0", "id": 1, "result": value})
}

fn rejection(code: i64, message: &str) -> Value {
    json!({"jsonrpc": "2.0", "id": 1, "error": {"code": code, "message": message}})
}

fn request(stream: &mut TcpStream) -> Value {
    stream.set_read_timeout(Some(TIMEOUT)).unwrap();
    stream.set_write_timeout(Some(TIMEOUT)).unwrap();
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader.read_line(&mut line).unwrap();
    assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
    let mut length = None;
    loop {
        line.clear();
        assert_ne!(reader.read_line(&mut line).unwrap(), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.strip_prefix("Content-Length: ") {
            length = Some(value.trim().parse::<usize>().unwrap());
        }
    }
    let mut body = vec![0; length.unwrap()];
    reader.read_exact(&mut body).unwrap();
    let value: Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(value["jsonrpc"], "2.0");
    assert_eq!(value["id"], 1);
    value
}

fn emulator(answers: Vec<Value>) -> (PaxeerEndpoint, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = PaxeerEndpoint::new(
        format!("http://{}/rpc", listener.local_addr().unwrap()),
        TIMEOUT,
        None,
        125,
        true,
    )
    .unwrap();
    listener.set_nonblocking(true).unwrap();
    let worker = thread::spawn(move || {
        let mut requests = Vec::new();
        for answer in answers {
            let deadline = Instant::now() + TIMEOUT;
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing JSON-RPC request");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept failed: {error}"),
                }
            };
            requests.push(request(&mut stream));
            let body = answer.to_string();
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                body.len(),
                body
            )
            .unwrap();
            stream.flush().unwrap();
        }
        requests
    });
    (endpoint, worker)
}

fn quorum_call(answers: Vec<Value>, params: Value) -> Result<Value, RpcFault> {
    let (endpoints, workers): (Vec<_>, Vec<_>) = answers
        .into_iter()
        .map(|answer| emulator(vec![result(json!("0x7d")), answer]))
        .unzip();
    let rpc = PaxeerRpc::new(endpoints).unwrap();
    let outcome = rpc.call("eth_blockNumber", params.clone());
    for worker in workers {
        let requests = worker.join().unwrap();
        assert_eq!(requests.len(), 2);
        assert_eq!(requests[0]["method"], "eth_chainId");
        assert_eq!(requests[0]["params"], json!([]));
        assert_eq!(requests[1]["method"], "eth_blockNumber");
        assert_eq!(requests[1]["params"], params);
    }
    outcome
}

#[test]
fn paxeer_reads_need_a_strict_majority_of_endpoints() {
    for outlier in 0..3 {
        let mut answers = vec![result(json!("0x10")); 3];
        answers[outlier] = result(json!("0x11"));
        assert_eq!(quorum_call(answers, json!([])), Ok(json!("0x10")));
    }
    assert_eq!(
        quorum_call(
            vec![
                result(json!("0x10")),
                result(json!("0x11")),
                result(json!("0x12"))
            ],
            json!([])
        ),
        Err(RpcFault::Divergence)
    );
    assert_eq!(
        quorum_call(
            vec![result(json!("0x10")), result(json!("0x010"))],
            json!([])
        ),
        Err(RpcFault::Divergence)
    );
    assert_eq!(
        quorum_call(
            vec![
                result(json!("0x10")),
                json!({"invalid": true}),
                json!({"invalid": true})
            ],
            json!([])
        ),
        Err(RpcFault::Unavailable)
    );
    assert_eq!(
        quorum_call(
            vec![
                result(json!("0x10")),
                rejection(-32000, "refused"),
                rejection(-32000, "refused")
            ],
            json!([])
        ),
        Err(RpcFault::Rejected {
            code: -32000,
            message: "refused".to_owned()
        })
    );
    assert_eq!(
        quorum_call(
            vec![
                result(json!("0x10")),
                rejection(-32000, "first"),
                rejection(-32000, "second")
            ],
            json!([])
        ),
        Err(RpcFault::Unavailable)
    );
    let value = json!({"height": "0x10", "transactions": [null, true, {"value": 18446744073709551615_u64}]});
    assert_eq!(
        quorum_call(
            vec![result(value.clone()); 3],
            json!({"block": "latest", "full": false})
        ),
        Ok(value)
    );
    let (endpoint, worker) = emulator(vec![result(json!("0x7e"))]);
    assert_eq!(
        PaxeerRpc::new(vec![endpoint])
            .unwrap()
            .call("eth_blockNumber", json!([])),
        Err(RpcFault::Unavailable)
    );
    let requests = worker.join().unwrap();
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0]["method"], "eth_chainId");
}

#[test]
fn paxeer_rpc_refuses_mixed_chain_ids_and_more_than_eight_endpoints() {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = PaxeerEndpoint::new(
        format!("http://{}/rpc", listener.local_addr().unwrap()),
        TIMEOUT,
        None,
        125,
        true,
    )
    .unwrap();
    let mut other_chain = endpoint.clone();
    other_chain.expected_chain_id = 126;
    for endpoints in [
        vec![],
        vec![endpoint.clone(); 9],
        vec![endpoint.clone(), other_chain],
    ] {
        assert!(matches!(
            PaxeerRpc::new(endpoints),
            Err(RpcFault::Configuration)
        ));
    }
    assert!(PaxeerRpc::new(vec![endpoint.clone()]).is_ok());
    assert!(PaxeerRpc::new(vec![endpoint; 8]).is_ok());
    listener.set_nonblocking(true).unwrap();
    assert_eq!(
        listener.accept().unwrap_err().kind(),
        std::io::ErrorKind::WouldBlock
    );
}

#[test]
fn a_sequencer_receipt_read_uses_pax_sequencer_receipt() {
    let receipt = json!({"transactionHash": format!("0x{}", "ab".repeat(32)), "status": "0x1"});
    let (endpoints, workers): (Vec<_>, Vec<_>) = (0..3)
        .map(|_| {
            emulator(vec![
                result(json!("0x7d")),
                result(Value::Null),
                result(json!("0x7d")),
                result(receipt.clone()),
                result(json!("0x7d")),
                rejection(-32000, "receipt unavailable"),
            ])
        })
        .unzip();
    let rpc = PaxeerRpc::new(endpoints).unwrap();
    let rpc: &dyn JsonRpc = &rpc;
    assert_eq!(rpc.sequencer_receipt(&[0xab; 32]), Ok(None));
    assert_eq!(rpc.sequencer_receipt(&[0xab; 32]), Ok(Some(receipt)));
    assert_eq!(
        rpc.sequencer_receipt(&[0xab; 32]),
        Err(RpcFault::Rejected {
            code: -32000,
            message: "receipt unavailable".to_owned()
        })
    );
    for worker in workers {
        let requests = worker.join().unwrap();
        assert_eq!(requests.len(), 6);
        for pair in requests.chunks_exact(2) {
            assert_eq!(pair[0]["method"], "eth_chainId");
            assert_eq!(pair[0]["params"], json!([]));
            assert_eq!(pair[1]["method"], "pax_sequencerReceipt");
            assert_eq!(pair[1]["params"], json!([format!("0x{}", "ab".repeat(32))]));
        }
    }
}
