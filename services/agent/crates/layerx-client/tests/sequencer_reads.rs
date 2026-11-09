use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use base64::Engine as _;
use layerx_client::sequencer::{HistoryPage, SequencerEndpoint, SequencerError};
use layerx_client::storage_witness::verify_storage_witness;
use serde_json::{json, Value};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("http://{}/", listener.local_addr().must());
    let handle = std::thread::spawn(move || {
        let mut seen = Vec::new();
        for (method, result) in exchanges {
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
            ).must();
            stream.flush().must();
        }
        seen
    });
    (url, handle)
}

fn hex(data: &[u8]) -> String {
    use std::fmt::Write as _;
    data.iter().fold(String::from("0x"), |mut text, byte| {
        write!(text, "{byte:02x}").must();
        text
    })
}

fn decode<const N: usize>(value: &Value) -> [u8; N] {
    let text = value.as_str().unwrap_or_else(|| panic!("hex string"));
    assert_eq!(text.len(), N * 2);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).must(), 16).must())
        .collect::<Vec<_>>()
        .try_into()
        .must()
}

#[test]
fn storage_witness_replaces_the_account_proof() {
    let vector: Value = serde_json::from_str(include_str!(
        "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
    ))
    .must();
    assert_eq!(
        vector["source"]["generator"],
        "modules/sequencer/keeper/ics23_evm_storage_vector_test.go::TestEvmStorageAppHashVector"
    );
    let entries = vector["entries"]
        .as_array()
        .unwrap_or_else(|| panic!("entries"));
    assert!(!entries.is_empty());
    for entry in entries {
        let address = decode::<20>(&entry["address"]);
        let slot = decode::<32>(&entry["slot"]);
        let slots = [slot];
        let height = entry["height"].as_u64().unwrap_or_else(|| panic!("height"));
        let response = entry["response"].clone();
        let mut exchanges = vec![("eth_getProof", response.clone())];
        for values in [json!([]), json!([entry["value"], entry["value"]])] {
            let mut malformed = response.clone();
            malformed["hexValues"] = values;
            exchanges.push(("eth_getProof", malformed));
        }
        let mut wrong_address = response.clone();
        wrong_address["address"] = json!(hex(&[0; 20]));
        exchanges.push(("eth_getProof", wrong_address));
        let mut missing_proof = response.clone();
        missing_proof["storageProof"] = json!([]);
        exchanges.push(("eth_getProof", missing_proof));
        for node in [json!("!"), json!(7), Value::Null] {
            let mut malformed = response.clone();
            malformed["storageProof"][0]["ops"][0]["data"] = node;
            exchanges.push(("eth_getProof", malformed));
        }
        let malformed_count = exchanges.len() - 1;
        let (url, handle) = serve(exchanges);
        let endpoint = SequencerEndpoint::new(&url, &[]).must();
        let witnesses = endpoint.storage_witness(&address, &slots, height).must();
        assert_eq!(witnesses.len(), slots.len());
        for (witness, slot) in witnesses.iter().zip(slots) {
            assert_eq!(witness.address, address);
            assert_eq!(witness.slot, slot);
            assert_eq!(witness.value, decode::<32>(&entry["value"]));
            assert_eq!(witness.ops.len(), 2);
            for (op, encoded) in witness.ops.iter().zip(
                response["storageProof"][0]["ops"]
                    .as_array()
                    .unwrap_or_else(|| panic!("ops")),
            ) {
                assert_eq!(
                    op.op_type,
                    encoded["type"].as_str().unwrap_or_else(|| panic!("type"))
                );
                assert_eq!(
                    op.key,
                    base64::engine::general_purpose::STANDARD
                        .decode(encoded["key"].as_str().unwrap_or_else(|| panic!("key")))
                        .must()
                );
                assert_eq!(
                    op.data,
                    base64::engine::general_purpose::STANDARD
                        .decode(encoded["data"].as_str().unwrap_or_else(|| panic!("data")))
                        .must()
                );
            }
            assert_eq!(
                verify_storage_witness(witness, decode::<32>(&entry["app_hash"])),
                Ok(())
            );
        }
        for _ in 0..malformed_count {
            assert_eq!(
                endpoint.storage_witness(&address, &slots, height),
                Err(SequencerError::Malformed)
            );
        }
        assert_eq!(
            handle.join().must(),
            vec![json!([hex(&address), [hex(&slot)], format!("{height:#x}")]); malformed_count + 1]
        );

        let listener = TcpListener::bind("127.0.0.1:0").must();
        listener.set_nonblocking(true).must();
        let endpoint =
            SequencerEndpoint::new(&format!("http://{}/", listener.local_addr().must()), &[])
                .must();
        assert_eq!(endpoint.storage_witness(&address, &[], height), Ok(vec![]));
        assert!(
            matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
        );
    }
}

fn recorded_freshness() -> Value {
    let transcript: Value = serde_json::from_str(include_str!(
        "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
    ))
    .must();
    transcript["result"]["freshness"].clone()
}

#[test]
fn history_sends_px_get_history_with_a_32_hex_cursor_and_refuses_other_forms() {
    let account = "0x1212121212121212121212121212121212121212";
    let cursor = "000000000000abcd0000001200000034";
    let next = "000000000000abce0000000100000002";
    let items = vec![
        json!({"kind": "transfer", "amount": "123", "nested": [1, null]}),
        json!({"kind": "call"}),
    ];
    let result = json!({
        "account": account, "chainHeight": 44_000, "freshness": recorded_freshness(),
        "items": items, "next_cursor": next, "indexedHeight": 43_982
    });
    let mut exchanges = vec![
        ("px_getHistory", result.clone()),
        (
            "px_getHistory",
            json!({
                "account": account, "chainHeight": 0, "freshness": recorded_freshness(),
                "items": [], "next_cursor": null, "indexedHeight": 0
            }),
        ),
    ];
    for (field, invalid) in [
        ("items", json!({})),
        ("next_cursor", json!("a".repeat(112))),
        ("next_cursor", json!(cursor.to_uppercase())),
        ("indexedHeight", json!("0xabce")),
        ("chainHeight", json!(-1)),
    ] {
        let mut malformed = result.clone();
        malformed[field] = invalid;
        exchanges.push(("px_getHistory", malformed));
    }
    let malformed_count = exchanges.len() - 2;
    let mut unindexed = result.clone();
    unindexed["indexedHeight"] = Value::Null;
    exchanges.push(("px_getHistory", unindexed));
    let (url, handle) = serve(exchanges);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    assert_eq!(
        endpoint.history(account, Some(cursor), 2).must(),
        HistoryPage {
            account: account.to_owned(),
            freshness: recorded_freshness(),
            items: items.clone(),
            next_cursor: Some(next.to_owned()),
            indexed_height: Some(43_982),
            chain_height: Some(44_000),
        }
    );
    assert_eq!(
        endpoint.history(account, None, 50).must(),
        HistoryPage {
            account: account.to_owned(),
            freshness: recorded_freshness(),
            items: vec![],
            next_cursor: None,
            indexed_height: Some(0),
            chain_height: Some(0),
        }
    );
    for _ in 0..malformed_count {
        assert_eq!(
            endpoint.history(account, Some(cursor), 2),
            Err(SequencerError::Malformed)
        );
    }
    assert_eq!(
        endpoint.history(account, Some(cursor), 2).must(),
        HistoryPage {
            account: account.to_owned(),
            freshness: recorded_freshness(),
            items,
            next_cursor: Some(next.to_owned()),
            indexed_height: None,
            chain_height: Some(44_000),
        }
    );
    let seen = handle.join().must();
    assert_eq!(seen.len(), 3 + malformed_count);
    assert_eq!(seen[0], json!([account, cursor, 2]));
    assert_eq!(seen[1], json!([account, null, 50]));
    for params in &seen[2..] {
        assert_eq!(params, &json!([account, cursor, 2]));
    }

    assert_invalid_history_cursors(account, cursor);
}

fn assert_invalid_history_cursors(account: &str, cursor: &str) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    listener.set_nonblocking(true).must();
    let endpoint =
        SequencerEndpoint::new(&format!("http://{}/", listener.local_addr().must()), &[]).must();
    for invalid in [
        "a".repeat(112),
        String::new(),
        "0".repeat(31),
        "0".repeat(33),
        cursor.to_uppercase(),
        format!("0x{cursor}"),
        "g".repeat(32),
        "é".repeat(16),
    ] {
        assert_eq!(
            endpoint.history(account, Some(&invalid), 2),
            Err(SequencerError::Endpoint),
            "{invalid}"
        );
        assert_eq!(
            listener.accept().err().map(|error| error.kind()),
            Some(std::io::ErrorKind::WouldBlock)
        );
    }
}

#[test]
fn history_preserves_the_served_envelope_and_keeps_a_separately_sampled_head() {
    let transcripts: Value =
        serde_json::from_str(include_str!("fixtures/repair_22_33_1546_gateway.json")).must();
    assert_eq!(
        transcripts["provenance"]["source"],
        "services/platform/hosted/gateway/tests/one_source_reads.rs"
    );
    let result = transcripts["history"].clone();
    let account = result["account"]
        .as_str()
        .unwrap_or_else(|| panic!("account"));
    let mut exchanges = vec![
        ("px_getHistory", result.clone()),
        ("eth_blockNumber", json!("0x13")),
    ];
    for field in [
        "account",
        "chainHeight",
        "freshness",
        "next_cursor",
        "indexedHeight",
    ] {
        let mut malformed = result.clone();
        assert!(malformed
            .as_object_mut()
            .unwrap_or_else(|| panic!("object"))
            .remove(field)
            .is_some());
        exchanges.push(("px_getHistory", malformed));
    }
    for (field, value) in [
        ("account", json!("")),
        ("account", json!({"evm_address": account})),
        ("chainHeight", json!("12")),
        ("chainHeight", Value::Null),
        ("next_cursor", json!(42)),
        ("next_cursor", json!("0x000000000000000a0000000000000000")),
        ("indexedHeight", json!(-1)),
        ("freshness", Value::Null),
    ] {
        let mut malformed = result.clone();
        malformed[field] = value;
        exchanges.push(("px_getHistory", malformed));
    }
    for (field, value) in [
        ("version", json!(2)),
        ("database", json!("unavailable")),
        ("sources", json!({})),
        ("status", json!("ready")),
    ] {
        let mut malformed = result.clone();
        malformed["freshness"][field] = value;
        exchanges.push(("px_getHistory", malformed));
    }
    let malformed_count = exchanges.len() - 2;
    let (url, server) = serve(exchanges);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let page = endpoint.history(account, None, 50).must();
    assert_eq!(
        page,
        HistoryPage {
            account: account.to_owned(),
            freshness: result["freshness"].clone(),
            items: result["items"]
                .as_array()
                .unwrap_or_else(|| panic!("items"))
                .clone(),
            next_cursor: None,
            indexed_height: Some(12),
            chain_height: Some(12),
        }
    );
    let sampled_head = endpoint.block_number().must();
    assert_eq!(sampled_head, 19);
    assert_eq!(page.chain_height, Some(12));
    assert_eq!(page.indexed_height, Some(12));
    assert_eq!(page.items.len(), 4);
    assert_eq!(page.items[1]["asset_metadata"]["denom"], "ulxp");
    assert_eq!(page.items[3]["asset_metadata"], Value::Null);
    for index in 0..malformed_count {
        assert_eq!(
            endpoint.history(account, None, 50),
            Err(SequencerError::Malformed),
            "malformed envelope {index}"
        );
    }
    let seen = server.join().must();
    assert_eq!(seen.len(), malformed_count + 2);
    assert_eq!(seen[1], json!([]));
    for (index, params) in seen.iter().enumerate() {
        if index != 1 {
            assert_eq!(params, &json!([account, null, 50]));
        }
    }
}
