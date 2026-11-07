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
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .must();
    assert_eq!(
        fixture["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let receipted = fixture["cases"][0]["receiptJson"].clone();
    let hash: [u8; 32] = decode_hex(&receipted["txHash"]).try_into().must();
    let signing_key: [u8; 32] = decode_hex(&receipted["signingKey"]).try_into().must();
    assert_eq!(receipted.as_object().must().len(), 24);
    let mut missing_key = receipted.clone();
    assert_eq!(
        missing_key.as_object_mut().must().remove("signingKey"),
        Some(json!(hex(&signing_key)))
    );
    assert_eq!(missing_key.as_object().must().len(), 23);
    let mut committed = receipted.clone();
    committed["rung"] = json!("committed");
    committed["blockNumber"] = json!("0x3e8");
    committed["blockHash"] = json!(hex(&[0xcd; 32]));
    assert_eq!(committed.as_object().must().len(), 26);
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
    assert_eq!(execution.payment_denom, "uhpx");
    assert_eq!(execution.fee_denom, "uhpx");
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

fn decode_hex(value: &Value) -> Vec<u8> {
    let text = value.as_str().must().strip_prefix("0x").must();
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).must(), 16).must())
        .collect()
}

#[test]
fn fee_token_receipt_preserves_payment_and_fee_denominations() {
    use sha2::{Digest as _, Sha256};

    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .must();
    let fee_token_fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-fee-token-v1.json"
    ))
    .must();
    assert_eq!(
        fee_token_fixture["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = &fee_token_fixture["cases"][0];
    assert_eq!(case["id"], "V1-fee-token-payment");
    let mut view = fixture["cases"][0]["receiptJson"].clone();
    view["paymentDenom"] = case["paymentDenom"].clone();
    view["feeDenom"] = case["feeDenom"].clone();
    view["receipt"] = case["signedReceipt"].clone();
    let hash = decode_hex(&view["txHash"]).try_into().must();
    let (url, server) = serve("pax_sequencerReceipt", vec![view]);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let Some(SequencerReceipt::Receipted(execution)) = endpoint.sequencer_receipt(&hash).must()
    else {
        panic!("expected a receipted answer");
    };
    assert_eq!(execution.payment_denom(), "usid");
    assert_eq!(execution.fee_denom, "uhpx");
    assert_eq!(execution.receipt, decode_hex(&case["signedReceipt"]));
    assert_eq!(
        layerx_wire::receipt::decode(&execution.receipt)
            .must()
            .protocol()
            .must()
            .asset(),
        <[u8; 32]>::from(Sha256::digest(b"usid"))
    );
    assert_eq!(server.join().must(), vec![json!([hex(&hash)])]);
}

#[test]
fn payment_denom_is_required_bounded_and_matches_the_receipt_asset() {
    use sha2::{Digest as _, Sha256};

    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .must();
    let view = fixture["cases"][0]["receiptJson"].clone();
    let hash: [u8; 32] = decode_hex(&view["txHash"]).try_into().must();
    let canonical = decode_hex(&view["receipt"]);
    let asset: [u8; 32] = Sha256::digest(b"uhpx").into();
    assert_eq!(
        layerx_wire::receipt::decode(&canonical)
            .must()
            .protocol()
            .must()
            .asset(),
        asset
    );
    let mut results = vec![view.clone()];
    for rung in ["receipted", "committed"] {
        let mut valid = view.clone();
        valid["rung"] = json!(rung);
        if rung == "committed" {
            valid["blockNumber"] = json!("0x3e8");
            valid["blockHash"] = json!(hex(&[0xcd; 32]));
        }
        let mut missing = valid.clone();
        assert!(missing
            .as_object_mut()
            .must()
            .remove("paymentDenom")
            .is_some());
        results.push(missing);
        for value in [
            Value::Null,
            json!(7),
            json!(""),
            json!("x".repeat(65)),
            json!("é".repeat(33)),
            json!("usid"),
        ] {
            let mut invalid = valid.clone();
            invalid["paymentDenom"] = value;
            results.push(invalid);
        }
        let mut fee_asset = valid;
        fee_asset["feeDenom"] = json!("usid");
        let mut wrong_asset = canonical.clone();
        let offset = wrong_asset
            .windows(32)
            .position(|bytes| bytes == asset)
            .must();
        wrong_asset[offset..offset + 32].copy_from_slice(&Sha256::digest(b"usid"));
        assert_eq!(
            layerx_wire::receipt::decode(&wrong_asset)
                .must()
                .protocol()
                .must()
                .asset(),
            <[u8; 32]>::from(Sha256::digest(b"usid"))
        );
        fee_asset["receipt"] = json!(hex(&wrong_asset));
        results.push(fee_asset);
    }
    let mut submitted = json!({"rung": "submitted", "txHash": hex(&hash), "nonce": "0x0"});
    submitted["paymentDenom"] = json!("uhpx");
    results.push(submitted);
    let count = results.len();
    let (url, server) = serve("pax_sequencerReceipt", results);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let Some(SequencerReceipt::Receipted(execution)) = endpoint.sequencer_receipt(&hash).must()
    else {
        panic!("expected a receipted answer");
    };
    assert_eq!(execution.payment_denom(), "uhpx");
    assert_eq!(execution.fee_denom, "uhpx");
    assert_eq!(execution.receipt, canonical);
    for index in 1..count {
        assert_eq!(
            endpoint.sequencer_receipt(&hash),
            Err(SequencerError::Malformed),
            "response {index}"
        );
    }
    assert_eq!(server.join().must(), vec![json!([hex(&hash)]); count]);
}
