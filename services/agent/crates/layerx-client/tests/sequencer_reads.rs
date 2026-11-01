use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::sequencer::{
    AccountProof, HistoryPage, SequencerEndpoint, SequencerError, StorageProof,
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

#[test]
fn account_proof_sends_eth_get_proof_at_the_height_and_decodes_every_node() {
    let address = [0x12; 20];
    let slots = [[0; 32], [0xab; 32]];
    let result = json!({
        "address": hex(&address),
        "balance": "0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
        "nonce": "0x100000001",
        "codeHash": hex(&[0x34; 32]),
        "storageHash": hex(&[0x56; 32]),
        "accountProof": ["0xf801", "0x00Abcd", "0x80"],
        "storageProof": [
            {"key": hex(&slots[0]), "value": "0x0", "proof": ["0x0102", "0xc0"]},
            {"key": hex(&slots[1]), "value": "0xabc", "proof": ["0xaabbcc", "0x03"]}
        ]
    });
    let mut exchanges = vec![("eth_getProof", result.clone())];
    for node in [
        json!("abcd"),
        json!("0x0"),
        json!("0xgg"),
        json!("0X00"),
        json!(7),
        Value::Null,
    ] {
        let mut account = result.clone();
        account["accountProof"][1] = node.clone();
        exchanges.push(("eth_getProof", account));
        let mut storage = result.clone();
        storage["storageProof"][1]["proof"][1] = node;
        exchanges.push(("eth_getProof", storage));
    }
    let mut malformed = result.clone();
    malformed["accountProof"] = json!("0xc0");
    exchanges.push(("eth_getProof", malformed));
    let mut malformed = result.clone();
    malformed["storageProof"][1]["proof"] = Value::Null;
    exchanges.push(("eth_getProof", malformed));
    let mut malformed = result.clone();
    malformed["codeHash"] = json!("0x01");
    exchanges.push(("eth_getProof", malformed));
    let mut malformed = result.clone();
    malformed["nonce"] = json!("0x10000000000000000");
    exchanges.push(("eth_getProof", malformed));
    let malformed_count = exchanges.len() - 1;
    let mut empty = result;
    empty["accountProof"] = json!([]);
    empty["storageProof"] = json!([]);
    empty["balance"] = json!("0x0");
    empty["nonce"] = json!("0x0");
    exchanges.push(("eth_getProof", empty));
    let (url, handle) = serve(exchanges);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let balance = [
        0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd,
        0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab,
        0xcd, 0xef,
    ];
    let mut stored = [0_u8; 32];
    stored[30..].copy_from_slice(&[0x0a, 0xbc]);
    assert_eq!(
        endpoint.account_proof(&address, &slots, 0x1234).must(),
        AccountProof {
            address,
            balance,
            nonce: 0x1_0000_0001,
            code_hash: [0x34; 32],
            storage_hash: [0x56; 32],
            account_proof: vec![vec![0xf8, 0x01], vec![0x00, 0xab, 0xcd], vec![0x80]],
            storage: vec![
                StorageProof {
                    key: slots[0],
                    value: [0; 32],
                    proof: vec![vec![1, 2], vec![0xc0]]
                },
                StorageProof {
                    key: slots[1],
                    value: stored,
                    proof: vec![vec![0xaa, 0xbb, 0xcc], vec![3]]
                },
            ],
        }
    );
    for _ in 0..malformed_count {
        assert_eq!(
            endpoint.account_proof(&address, &slots, 0x1234),
            Err(SequencerError::Malformed)
        );
    }
    let empty = endpoint.account_proof(&address, &[], 0).must();
    assert!(empty.account_proof.is_empty());
    assert!(empty.storage.is_empty());
    assert_eq!(empty.balance, [0; 32]);
    assert_eq!(empty.nonce, 0);
    let seen = handle.join().must();
    assert_eq!(seen.len(), malformed_count + 2);
    for params in &seen[..=malformed_count] {
        assert_eq!(
            params,
            &json!([hex(&address), [hex(&slots[0]), hex(&slots[1])], "0x1234"])
        );
    }
    assert_eq!(seen[malformed_count + 1], json!([hex(&address), [], "0x0"]));
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
        "items": items, "next_cursor": next, "indexedHeight": 43982, "chainHeight": 44000,
        "account": account, "freshness": {"status": "ready"}
    });
    let mut exchanges = vec![
        ("px_getHistory", result.clone()),
        (
            "px_getHistory",
            json!({
                "items": [], "next_cursor": null, "indexedHeight": 0, "chainHeight": 0
            }),
        ),
    ];
    for (field, invalid) in [
        ("items", json!({})),
        ("next_cursor", json!("a".repeat(112))),
        ("next_cursor", json!(cursor.to_uppercase())),
        ("indexedHeight", json!("0xabce")),
        ("indexedHeight", Value::Null),
        ("chainHeight", json!(-1)),
    ] {
        let mut malformed = result.clone();
        malformed[field] = invalid;
        exchanges.push(("px_getHistory", malformed));
    }
    let malformed_count = exchanges.len() - 2;
    let (url, handle) = serve(exchanges);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    assert_eq!(
        endpoint.history(account, Some(cursor), 2).must(),
        HistoryPage {
            items,
            next_cursor: Some(next.to_owned()),
            indexed_height: 43982,
            chain_height: 44000,
        }
    );
    assert_eq!(
        endpoint.history(account, None, 50).must(),
        HistoryPage {
            items: vec![],
            next_cursor: None,
            indexed_height: 0,
            chain_height: 0,
        }
    );
    for _ in 0..malformed_count {
        assert_eq!(
            endpoint.history(account, Some(cursor), 2),
            Err(SequencerError::Malformed)
        );
    }
    let seen = handle.join().must();
    assert_eq!(seen[0], json!([account, cursor, 2]));
    assert_eq!(seen[1], json!([account, null, 50]));
    for params in &seen[2..] {
        assert_eq!(params, &json!([account, cursor, 2]));
    }

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
