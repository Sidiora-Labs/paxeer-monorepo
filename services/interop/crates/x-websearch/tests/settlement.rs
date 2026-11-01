use std::error::Error;
use std::io::{Read as _, Write as _};
use std::net::{TcpListener, TcpStream};
use std::sync::mpsc::{self, Sender};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use serde_json::{json, Value};
use x_websearch::settlement::{
    decode, SettlementError, SettlementLog, SettlementRpc, SettlementRung,
};

type Checked<T = ()> = Result<T, Box<dyn Error>>;
type Exchange = (Value, Value, bool);

struct Replay {
    endpoint: String,
    stop: Sender<()>,
    worker: JoinHandle<Result<Vec<Value>, String>>,
}

impl Replay {
    fn start(recording: Vec<Exchange>) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = format!("http://{}/evm", listener.local_addr()?);
        let (stop, receiver) = mpsc::channel();
        let worker = thread::spawn(move || {
            let mut observed = Vec::new();
            let mut recording = recording.into_iter();
            let deadline = Instant::now() + Duration::from_secs(15);
            loop {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        let request =
                            read_request(&mut stream).map_err(|error| error.to_string())?;
                        let (expected, answer, chunked) = recording
                            .next()
                            .ok_or_else(|| format!("unexpected extra request: {request}"))?;
                        if request != expected {
                            return Err(format!("expected {expected}, received {request}"));
                        }
                        observed.push(request);
                        let body = answer.to_string();
                        let wire = if chunked {
                            format!(
                                "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n{:x}\r\n{}\r\n0\r\n\r\n",
                                body.len(), body
                            )
                        } else {
                            format!(
                                "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                                body.len(), body
                            )
                        };
                        stream
                            .write_all(wire.as_bytes())
                            .map_err(|error| error.to_string())?;
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        if receiver.try_recv().is_ok() {
                            if recording.next().is_some() {
                                return Err("recorded requests were not sent".to_owned());
                            }
                            return Ok(observed);
                        }
                        if Instant::now() >= deadline {
                            return Err("loopback replay deadline exceeded".to_owned());
                        }
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => return Err(error.to_string()),
                }
            }
        });
        Ok(Self {
            endpoint,
            stop,
            worker,
        })
    }

    fn finish(self) -> Checked<Vec<Value>> {
        self.stop.send(())?;
        self.worker
            .join()
            .map_err(|_| "loopback replay panicked")?
            .map_err(Into::into)
    }
}

fn read_request(stream: &mut TcpStream) -> Checked<Value> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut head = Vec::new();
    while !head.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        stream.read_exact(&mut byte)?;
        head.push(byte[0]);
        if head.len() > 8192 {
            return Err("oversized HTTP header".into());
        }
    }
    let head = std::str::from_utf8(&head)?;
    assert!(head.starts_with("POST /evm HTTP/1.1\r\n"));
    assert!(!head.to_ascii_lowercase().contains("authorization:"));
    let length: usize = head
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .ok_or("missing content length")?
        .parse()?;
    let mut body = vec![0; length];
    stream.read_exact(&mut body)?;
    Ok(serde_json::from_slice(&body)?)
}

fn exchange(method: &str, params: Value, result: Value) -> Exchange {
    (
        json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}),
        json!({"jsonrpc": "2.0", "id": 1, "result": result}),
        false,
    )
}

fn chain(id: &str) -> Exchange {
    exchange("eth_chainId", json!([]), json!(id))
}

fn committed() -> Value {
    json!({
        "rung": "committed",
        "txHash": "0x1111111111111111111111111111111111111111111111111111111111111111",
        "nonce": "0x2a", "seq": "0x101", "height": "0x200", "status": "0x1",
        "logs": [{
            "address": "0x000000000000000000000000000000000000101a",
            "topics": [
                "0x2222222222222222222222222222222222222222222222222222222222222222",
                "0x3333333333333333333333333333333333333333333333333333333333333333"
            ],
            "data": "0x00abcdff", "logIndex": "0x3"
        }],
        "keyId": "0x4444444444444444444444444444444444444444444444444444444444444444",
        "receipt": "0x00020100ffabcdef",
        "blockNumber": "0x201",
        "blockHash": "0x5555555555555555555555555555555555555555555555555555555555555555"
    })
}

#[test]
fn the_settlement_client_checks_the_chain_id_before_each_call() -> Checked {
    let params = json!(["0x000000000000000000000000000000000000101a", "pending"]);
    let replay = Replay::start(vec![chain("0x7e")])?;
    let rpc = SettlementRpc::new(&replay.endpoint, 125)?;
    assert_eq!(rpc.endpoint(), replay.endpoint);
    assert_eq!(
        rpc.call("eth_getTransactionCount", &params),
        Err(SettlementError::ChainMismatch {
            expected: 125,
            actual: 126
        })
    );
    let observed = replay.finish()?;
    assert_eq!(observed.len(), 1);
    assert_eq!(observed[0]["method"], "eth_chainId");

    let replay = Replay::start(vec![
        chain("0x7d"),
        exchange("eth_getTransactionCount", params.clone(), json!("0x2a")),
        chain("0x7d"),
        exchange("eth_getTransactionCount", params.clone(), json!("0x2b")),
        chain("0x7e"),
    ])?;
    let rpc = SettlementRpc::new(&replay.endpoint, 125)?;
    assert_eq!(rpc.call("eth_getTransactionCount", &params)?, json!("0x2a"));
    assert_eq!(rpc.call("eth_getTransactionCount", &params)?, json!("0x2b"));
    assert_eq!(
        rpc.call("eth_getTransactionCount", &params),
        Err(SettlementError::ChainMismatch {
            expected: 125,
            actual: 126
        })
    );
    let methods: Vec<_> = replay
        .finish()?
        .iter()
        .map(|request| request["method"].clone())
        .collect();
    assert_eq!(
        methods,
        vec![
            json!("eth_chainId"),
            json!("eth_getTransactionCount"),
            json!("eth_chainId"),
            json!("eth_getTransactionCount"),
            json!("eth_chainId")
        ]
    );

    for answer in [
        json!({"jsonrpc":"2.0", "id":1, "error":{"code":-32000, "message":"served by sequencer"}}),
        json!({"jsonrpc":"2.0", "id":2, "result":"0x2a"}),
        json!({"jsonrpc":"2.0", "id":1, "result":"0x2a", "error":{"code":-1,"message":"bad"}}),
    ] {
        let is_rpc_error = answer.get("error").is_some() && answer.get("result").is_none();
        let (request, _, _) = exchange("eth_getTransactionCount", params.clone(), Value::Null);
        let replay = Replay::start(vec![chain("0x7d"), (request, answer, false)])?;
        let result =
            SettlementRpc::new(&replay.endpoint, 125)?.call("eth_getTransactionCount", &params);
        if is_rpc_error {
            assert_eq!(
                result,
                Err(SettlementError::Rpc {
                    code: -32000,
                    message: "served by sequencer".to_owned()
                })
            );
        } else {
            assert_eq!(result, Err(SettlementError::Malformed));
        }
        replay.finish()?;
    }
    Ok(())
}

#[test]
fn a_committed_sequencer_receipt_answer_decodes_every_field() -> Checked {
    let recording = committed();
    let receipt = decode(&recording)?;
    assert_eq!(
        receipt.rung,
        SettlementRung::Committed {
            block_number: 513,
            block_hash: [0x55; 32]
        }
    );
    assert_eq!(receipt.tx_hash, [0x11; 32]);
    assert_eq!(receipt.nonce, 42);
    assert_eq!(receipt.seq, Some(257));
    assert_eq!(receipt.height, Some(512));
    assert_eq!(receipt.status, Some(1));
    assert_eq!(receipt.key_id, Some([0x44; 32]));
    assert_eq!(receipt.receipt, Some(vec![0, 2, 1, 0, 255, 171, 205, 239]));
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    assert_eq!(
        receipt.logs,
        vec![SettlementLog {
            address,
            topics: vec![[0x22; 32], [0x33; 32]],
            data: vec![0, 171, 205, 255],
            log_index: 3,
        }]
    );
    let hash = json!(["0x1111111111111111111111111111111111111111111111111111111111111111"]);
    let mut recorded_reply = exchange("pax_sequencerReceipt", hash.clone(), recording);
    recorded_reply.2 = true;
    let replay = Replay::start(vec![
        chain("0x7d"),
        recorded_reply,
        chain("0x7d"),
        exchange("pax_sequencerReceipt", hash, Value::Null),
    ])?;
    let rpc = SettlementRpc::new(&replay.endpoint, 125)?;
    assert_eq!(rpc.sequencer_receipt(&[0x11; 32])?, Some(receipt));
    assert_eq!(rpc.sequencer_receipt(&[0x11; 32])?, None);
    assert_eq!(replay.finish()?.len(), 4);

    let replay = Replay::start(vec![
        chain("0x7d"),
        exchange(
            "eth_call",
            json!([{
            "to":"0x000000000000000000000000000000000000101a", "data":"0x123400"
        }, "pending"]),
            json!("0x00ffab"),
        ),
        chain("0x7d"),
        exchange(
            "eth_sendRawTransaction",
            json!(["0x010203"]),
            json!("0x1111111111111111111111111111111111111111111111111111111111111111"),
        ),
    ])?;
    let rpc = SettlementRpc::new(&replay.endpoint, 125)?;
    assert_eq!(rpc.eth_call(address, &[0x12, 0x34, 0])?, vec![0, 255, 171]);
    assert_eq!(rpc.send_raw_transaction(&[1, 2, 3])?, [0x11; 32]);
    assert_eq!(replay.finish()?.len(), 4);
    Ok(())
}

#[test]
fn a_submitted_answer_with_receipt_fields_is_refused() -> Checked {
    let submitted = json!({"rung":"submitted",
        "txHash":"0x1111111111111111111111111111111111111111111111111111111111111111", "nonce":"0x2a"});
    let valid = decode(&submitted)?;
    assert_eq!(valid.rung, SettlementRung::Submitted);
    assert_eq!(valid.tx_hash, [0x11; 32]);
    assert_eq!(valid.nonce, 42);
    assert_eq!(
        (
            valid.seq,
            valid.height,
            valid.status,
            valid.key_id,
            valid.receipt
        ),
        (None, None, None, None, None)
    );
    assert!(valid.logs.is_empty());
    let mut invalid = Vec::new();
    for (name, value) in [
        ("receipt", json!("0x0102")),
        ("seq", json!("0x1")),
        ("height", json!("0x1")),
        ("receipt", Value::Null),
        ("seq", Value::Null),
        ("height", Value::Null),
        ("logs", json!([])),
        ("status", json!("0x1")),
        ("keyId", json!("0x00")),
        ("blockNumber", json!("0x1")),
        ("blockHash", Value::Null),
    ] {
        let mut answer = submitted.clone();
        answer[name] = value;
        invalid.push(answer);
    }
    let mut unknown = submitted.clone();
    unknown["rung"] = json!("finalized");
    invalid.push(unknown);
    let mut receipted = committed();
    receipted["rung"] = json!("receipted");
    invalid.push(receipted.clone());
    receipted
        .as_object_mut()
        .ok_or("receipt is not an object")?
        .remove("blockNumber");
    invalid.push(receipted.clone());
    receipted
        .as_object_mut()
        .ok_or("receipt is not an object")?
        .remove("blockHash");
    assert_eq!(decode(&receipted)?.rung, SettlementRung::Receipted);
    for (name, value) in [
        ("blockNumber", json!("0x1ff")),
        ("blockHash", json!("0x00")),
        ("status", json!("0x2")),
        ("receipt", json!("0x")),
        ("height", json!("0x0200")),
        ("txHash", json!("0x11")),
        ("nonce", json!(42)),
    ] {
        let mut answer = committed();
        answer[name] = value;
        invalid.push(answer);
    }
    for name in [
        "seq",
        "height",
        "status",
        "logs",
        "keyId",
        "receipt",
        "blockNumber",
        "blockHash",
    ] {
        let mut answer = committed();
        answer
            .as_object_mut()
            .ok_or("receipt is not an object")?
            .remove(name);
        invalid.push(answer);
    }
    let mut recording = Vec::new();
    let hash = json!(["0x1111111111111111111111111111111111111111111111111111111111111111"]);
    for answer in &invalid {
        assert_eq!(decode(answer), Err(SettlementError::Malformed), "{answer}");
        recording.push(chain("0x7d"));
        recording.push(exchange(
            "pax_sequencerReceipt",
            hash.clone(),
            answer.clone(),
        ));
    }
    let expected_requests = recording.len();
    let replay = Replay::start(recording)?;
    let rpc = SettlementRpc::new(&replay.endpoint, 125)?;
    for _ in &invalid {
        assert_eq!(
            rpc.sequencer_receipt(&[0x11; 32]),
            Err(SettlementError::Malformed)
        );
    }
    assert_eq!(replay.finish()?.len(), expected_requests);
    Ok(())
}

#[test]
fn pending_nonce_reads_the_pending_transaction_count() -> Checked {
    let params = json!(["0x1212121212121212121212121212121212121212", "pending"]);
    let answers = [
        (json!("0x2a"), Ok(42)),
        (json!("0x0"), Ok(0)),
        (json!("0xffffffffffffffff"), Ok(u64::MAX)),
        (json!("0x02a"), Err(SettlementError::Malformed)),
        (json!("0x00"), Err(SettlementError::Malformed)),
        (json!("0x"), Err(SettlementError::Malformed)),
        (json!("2a"), Err(SettlementError::Malformed)),
        (
            json!("0x10000000000000000"),
            Err(SettlementError::Malformed),
        ),
        (json!("0x+1"), Err(SettlementError::Malformed)),
        (json!("0xgg"), Err(SettlementError::Malformed)),
        (json!(42), Err(SettlementError::Malformed)),
    ];
    let mut recording = Vec::new();
    for (answer, _) in &answers {
        recording.push(chain("0x7d"));
        recording.push(exchange(
            "eth_getTransactionCount",
            params.clone(),
            answer.clone(),
        ));
    }
    let replay = Replay::start(recording)?;
    let rpc = SettlementRpc::new(&replay.endpoint, 125)?;
    for (answer, expected) in &answers {
        assert_eq!(&rpc.pending_nonce([0x12; 20]), expected, "{answer}");
    }
    let requests = replay.finish()?;
    assert_eq!(requests.len(), 2 * answers.len());
    for pair in requests.chunks_exact(2) {
        assert_eq!(pair[0]["method"], "eth_chainId");
        assert_eq!(pair[1]["method"], "eth_getTransactionCount");
        assert_eq!(pair[1]["params"], params);
    }
    Ok(())
}
