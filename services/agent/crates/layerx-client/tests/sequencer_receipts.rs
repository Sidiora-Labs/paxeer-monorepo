use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::sequencer::{
    SequencerEndpoint, SequencerError, SequencerLog, SequencerReceipt, TransactionReceipt,
};
use serde_json::{json, Value};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

impl<T> Must<T> for Option<T> {
    fn must(self) -> T {
        self.unwrap_or_else(|| panic!("missing value"))
    }
}

fn hex(data: &[u8]) -> String {
    use std::fmt::Write as _;
    data.iter().fold(String::from("0x"), |mut text, byte| {
        write!(text, "{byte:02x}").must();
        text
    })
}

fn serve(method: &'static str, results: Vec<Value>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("http://{}/", listener.local_addr().must());
    let handle = std::thread::spawn(move || {
        let mut seen = Vec::new();
        for result in results {
            let (stream, _) = listener.accept().must();
            stream.set_read_timeout(Some(Duration::from_secs(5))).must();
            let mut reader = BufReader::new(stream);
            let mut length = 0_usize;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).must(), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = value.trim().parse().must();
                }
            }
            let mut body = vec![0_u8; length];
            reader.read_exact(&mut body).must();
            let request: Value = serde_json::from_slice(&body).must();
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            seen.push(request["params"].clone());
            let mut stream = reader.into_inner();
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                answer.len()
            )
            .must();
            stream.flush().must();
        }
        seen
    });
    (url, handle)
}

fn receipt() -> Value {
    json!({
        "status": "0x1",
        "blockNumber": "0x100000001",
        "transactionIndex": "0x2a",
        "gasUsed": "0x5208",
        "transactionHash": hex(&[0xab; 32]),
        "blockHash": hex(&[0xcd; 32]),
        "cumulativeGasUsed": "0xa410",
        "logs": [
            {
                "address": hex(&[0x12; 20]),
                "topics": [hex(&[0x34; 32]), hex(&[0x56; 32]), hex(&[0x78; 32]), hex(&[0x9a; 32])],
                "data": "0x00aBcD",
                "logIndex": "0x100000002",
                "transactionHash": hex(&[0xab; 32]),
                "transactionIndex": "0x2a",
                "blockNumber": "0x100000001",
                "blockHash": hex(&[0xcd; 32]),
                "removed": false
            },
            {
                "address": hex(&[0xbc; 20]),
                "topics": [],
                "data": "0x",
                "logIndex": "0x100000003"
            }
        ]
    })
}

#[test]
fn transaction_receipt_decodes_status_block_gas_and_logs() {
    let hash = [0xab; 32];
    let mut reverted = receipt();
    reverted["status"] = json!("0x0");
    reverted["blockNumber"] = json!("0x0");
    reverted["transactionIndex"] = json!("0x0");
    reverted["gasUsed"] = json!("0xffffffffffffffff");
    reverted["logs"] = json!([]);
    let (url, server) = serve("eth_getTransactionReceipt", vec![receipt(), reverted]);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    assert_eq!(
        endpoint.transaction_receipt(&hash),
        Ok(Some(TransactionReceipt {
            status: true,
            block_number: 0x1_0000_0001,
            transaction_index: 42,
            gas_used: 21_000,
            logs: vec![
                SequencerLog {
                    address: [0x12; 20],
                    topics: vec![[0x34; 32], [0x56; 32], [0x78; 32], [0x9a; 32]],
                    data: vec![0, 0xab, 0xcd],
                    log_index: 0x1_0000_0002,
                },
                SequencerLog {
                    address: [0xbc; 20],
                    topics: vec![],
                    data: vec![],
                    log_index: 0x1_0000_0003,
                }
            ],
        }))
    );
    assert_eq!(
        endpoint.transaction_receipt(&hash),
        Ok(Some(TransactionReceipt {
            status: false,
            block_number: 0,
            transaction_index: 0,
            gas_used: u64::MAX,
            logs: vec![],
        }))
    );
    assert_eq!(server.join().must(), vec![json!([hex(&hash)]); 2]);
}

#[test]
fn unknown_transaction_receipt_is_none_and_a_bad_status_is_malformed() {
    let hash = [0xab; 32];
    let mut results = vec![Value::Null];
    for status in [json!("0x2"), json!("0x01"), json!(1), Value::Null] {
        let mut invalid = receipt();
        invalid["status"] = status;
        results.push(invalid);
    }
    for field in ["blockNumber", "transactionIndex", "gasUsed"] {
        for value in [
            json!("0x10000000000000000"),
            json!("0x00"),
            json!(7),
            Value::Null,
        ] {
            let mut invalid = receipt();
            invalid[field] = value;
            results.push(invalid);
        }
    }
    for (field, value) in [
        ("address", json!(hex(&[0x12; 19]))),
        ("topics", json!([hex(&[0x34; 31])])),
        ("topics", json!(vec![hex(&[0x34; 32]); 5])),
        ("topics", Value::Null),
        ("data", json!("0x0")),
        ("data", json!("0xgg")),
        ("data", json!("abcd")),
        ("logIndex", json!("0x00")),
        ("logIndex", json!("0x10000000000000000")),
    ] {
        let mut invalid = receipt();
        invalid["logs"][1][field] = value;
        results.push(invalid);
    }
    for field in ["address", "topics", "data", "logIndex"] {
        let mut invalid = receipt();
        assert!(invalid["logs"][1]
            .as_object_mut()
            .must()
            .remove(field)
            .is_some());
        results.push(invalid);
    }
    for field in [
        "status",
        "blockNumber",
        "transactionIndex",
        "gasUsed",
        "logs",
    ] {
        let mut invalid = receipt();
        assert!(invalid.as_object_mut().must().remove(field).is_some());
        results.push(invalid);
    }
    let mut invalid = receipt();
    invalid["logs"] = json!({});
    results.push(invalid);
    let mut invalid = receipt();
    invalid["logs"][1] = Value::Null;
    results.push(invalid);
    results.push(json!([]));
    let count = results.len();
    let (url, server) = serve("eth_getTransactionReceipt", results);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    assert_eq!(endpoint.transaction_receipt(&hash), Ok(None));
    for index in 1..count {
        assert_eq!(
            endpoint.transaction_receipt(&hash),
            Err(SequencerError::Malformed),
            "response {index}"
        );
    }
    assert_eq!(server.join().must(), vec![json!([hex(&hash)]); count]);
}

#[test]
fn decode_receipt_reads_the_served_signing_key() {
    let hash = [0xab; 32];
    let signing_key = ed25519_dalek::SigningKey::from_bytes(&[8; 32])
        .verifying_key()
        .to_bytes();
    let receipted = json!({
        "rung": "receipted",
        "txHash": hex(&hash),
        "nonce": "0x7",
        "seq": "0x2a",
        "height": "0x3e8",
        "index": "0x3",
        "timestamp": "0x67a1b2c3",
        "status": "0x1",
        "returnData": "0x",
        "logs": [],
        "gasUsed": "0x5208",
        "cumulativeGasUsed": "0xa410",
        "effectiveGasPrice": "0x3b9aca00",
        "feeDenom": "upax",
        "feeAmount": "0x1312d00000000",
        "sponsor": hex(&[0; 20]),
        "from": hex(&[0x12; 20]),
        "to": hex(&[0x34; 20]),
        "contractAddress": hex(&[0; 20]),
        "vmError": "",
        "keyId": hex(&[1; 32]),
        "receipt": "0x00025201000200aabbccdd",
        "signingKey": hex(&signing_key)
    });
    assert_eq!(receipted.as_object().must().len(), 23);
    let mut missing_key = receipted.clone();
    assert_eq!(
        missing_key.as_object_mut().must().remove("signingKey"),
        Some(json!(hex(&signing_key)))
    );
    assert_eq!(missing_key.as_object().must().len(), 22);
    let mut committed = receipted.clone();
    committed["rung"] = json!("committed");
    committed["blockNumber"] = json!("0x3e8");
    committed["blockHash"] = json!(hex(&[0xcd; 32]));
    assert_eq!(committed.as_object().must().len(), 25);
    let mut committed_missing_key = committed.clone();
    assert!(committed_missing_key
        .as_object_mut()
        .must()
        .remove("signingKey")
        .is_some());
    let mut short_key = receipted.clone();
    short_key["signingKey"] = json!(hex(&signing_key[..31]));
    let mut long_key = receipted.clone();
    long_key["signingKey"] = json!(hex(&[8; 33]));
    let (url, server) = serve(
        "pax_sequencerReceipt",
        vec![
            receipted,
            missing_key,
            committed,
            committed_missing_key,
            short_key,
            long_key,
        ],
    );
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let Some(SequencerReceipt::Receipted(execution)) = endpoint.sequencer_receipt(&hash).must()
    else {
        panic!("expected a receipted answer");
    };
    assert_eq!(execution.tx_hash, hash);
    assert_eq!(execution.signing_key, signing_key);
    assert_eq!(
        endpoint.sequencer_receipt(&hash),
        Err(SequencerError::Malformed)
    );
    assert_eq!(
        endpoint.sequencer_receipt(&hash),
        Ok(Some(SequencerReceipt::Committed {
            receipt: execution,
            block_number: 1000,
            block_hash: [0xcd; 32],
        }))
    );
    for _ in 0..3 {
        assert_eq!(
            endpoint.sequencer_receipt(&hash),
            Err(SequencerError::Malformed)
        );
    }
    assert_eq!(server.join().must(), vec![json!([hex(&hash)]); 6]);
}
