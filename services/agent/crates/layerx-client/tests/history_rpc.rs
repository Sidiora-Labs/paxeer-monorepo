#[path = "support/gateway_22_33_1546.rs"]
mod producer_repair;

use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::rpc::history::{did_accounts, history, HistoryPage, PxEndpoint};
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};
use serde_json::{json, Value};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

fn serve(exchanges: Vec<(&'static str, Value, Value)>) -> (String, JoinHandle<usize>) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("http://{}/", listener.local_addr().must());
    let handle = std::thread::spawn(move || {
        let mut seen = 0;
        for (method, params, mut response) in exchanges {
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
            assert_eq!(request["params"], params);
            response["jsonrpc"] = json!("2.0");
            response["id"] = request["id"].clone();
            let answer = response.to_string();
            let mut stream = reader.into_inner();
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                answer.len()
            ).must();
            stream.flush().must();
            seen += 1;
        }
        seen
    });
    (url, handle)
}

fn recorded_freshness() -> Value {
    let transcript: Value = serde_json::from_str(include_str!(
        "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
    ))
    .must();
    transcript["result"]["freshness"].clone()
}

#[test]
fn history_pages_px_get_history_and_carries_the_next_cursor() {
    producer_repair::history();
    let account = "0x1212121212121212121212121212121212121212";
    let next = "000000000000abcd0000001200000034";
    let items = vec![
        json!({"kind": "transfer", "amount": "123", "details": {"memo": [1, null]}}),
        json!({"kind": "call", "transactionHash": format!("0x{}", "34".repeat(32))}),
    ];
    let result = json!({
        "account": account, "chainHeight": 44_000, "freshness": recorded_freshness(),
        "items": items, "next_cursor": next, "indexedHeight": 43_981
    });
    let refusal = RpcRefusal {
        code: -32_001,
        message: "History unavailable".to_owned(),
        data: Some(json!({"code": "indexer_not_configured"})),
    };
    let mut exchanges = vec![
        (
            "px_getHistory",
            json!([account, null, 2]),
            json!({"result": result}),
        ),
        (
            "px_getHistory",
            json!([account, next, 50]),
            json!({"result": {
                "account": account, "chainHeight": 44_001, "freshness": recorded_freshness(),
                "items": [], "next_cursor": null, "indexedHeight": 43_982
            }}),
        ),
    ];
    for cursor in ["a".repeat(112), next.to_uppercase(), "g".repeat(32)] {
        let mut malformed = result.clone();
        malformed["next_cursor"] = json!(cursor);
        exchanges.push((
            "px_getHistory",
            json!([account, next, 2]),
            json!({"result": malformed}),
        ));
    }
    exchanges.push((
        "px_getHistory",
        json!([account, next, 2]),
        json!({"error": {
            "code": refusal.code, "message": refusal.message, "data": refusal.data
        }}),
    ));
    let count = exchanges.len();
    let (url, server) = serve(exchanges);
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let page: HistoryPage = history(&endpoint, account, None, Some(2)).must();
    assert_eq!(
        page,
        HistoryPage {
            account: account.to_owned(),
            freshness: recorded_freshness(),
            items,
            next_cursor: Some(next.to_owned()),
            indexed_height: Some(43_981),
            chain_height: Some(44_000),
        }
    );
    let last = history(&endpoint, account, page.next_cursor.as_deref(), None).must();
    assert_eq!(
        last,
        HistoryPage {
            account: account.to_owned(),
            freshness: recorded_freshness(),
            items: vec![],
            next_cursor: None,
            indexed_height: Some(43_982),
            chain_height: Some(44_001),
        }
    );
    for _ in 0..3 {
        assert_eq!(
            history(&endpoint, account, Some(next), Some(2)),
            Err(SequencerError::Malformed)
        );
    }
    assert_eq!(
        history(&endpoint, account, Some(next), Some(2)),
        Err(SequencerError::Rpc(refusal))
    );
    assert_eq!(server.join().must(), count);
    assert_invalid_history_cursors(account, next);
}

fn assert_invalid_history_cursors(account: &str, next: &str) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    listener.set_nonblocking(true).must();
    let endpoint =
        SequencerEndpoint::new(&format!("http://{}/", listener.local_addr().must()), &[]).must();
    for invalid in [
        "a".repeat(112),
        next.to_uppercase(),
        format!("0x{next}"),
        String::new(),
    ] {
        assert_eq!(
            history(&endpoint, account, Some(&invalid), Some(2)),
            Err(SequencerError::Endpoint)
        );
        assert_eq!(
            listener.accept().err().map(|error| error.kind()),
            Some(std::io::ErrorKind::WouldBlock)
        );
    }
}

#[test]
fn did_accounts_reads_px_get_account_for_the_did() {
    let did = format!("did:layerx:{}", "ab".repeat(32));
    let document = json!({
        "evm_address": format!("0x{}", "12".repeat(20)),
        "pax_address": "pax1boundidentity",
        "layerx_did": did,
        "layerx_account": "34".repeat(32),
        "bound": true
    });
    let result = json!({
        "account": document,
        "paxeer": {"address": document["evm_address"], "balance": "0x7b", "nonce": "0x7", "height": 42}
    });
    let mut unbound = result.clone();
    unbound["account"]["bound"] = json!(false);
    unbound["account"]["evm_address"] = Value::Null;
    unbound["account"]["pax_address"] = Value::Null;
    unbound["paxeer"] = Value::Null;
    let mut foreign = result.clone();
    foreign["account"]["layerx_did"] = json!(format!("did:layerx:{}", "cd".repeat(32)));
    let mut explicit_null = result.clone();
    explicit_null["layerx"] = Value::Null;
    let refusal = RpcRefusal {
        code: -32_001,
        message: "Read unavailable".to_owned(),
        data: Some(json!({"code": "native_read_unavailable"})),
    };
    let mut exchanges = vec![
        ("px_getAccount", json!([did]), json!({"result": result})),
        ("px_getAccount", json!([did]), json!({"result": unbound})),
        ("px_getAccount", json!([did]), json!({"result": foreign})),
        (
            "px_getAccount",
            json!([did]),
            json!({"result": explicit_null}),
        ),
    ];
    for layerx in [json!(false), json!([]), json!("invalid")] {
        let mut malformed = result.clone();
        malformed["layerx"] = layerx;
        exchanges.push(("px_getAccount", json!([did]), json!({"result": malformed})));
    }
    exchanges.push((
        "px_getAccount",
        json!([did]),
        json!({"error": {
            "code": refusal.code, "message": refusal.message, "data": refusal.data
        }}),
    ));
    let count = exchanges.len();
    let (url, server) = serve(exchanges);
    let endpoint = PxEndpoint::new(&url, &[]).must();
    assert_eq!(did_accounts(&endpoint, &did).must(), vec![document.clone()]);
    assert!(did_accounts(&endpoint, &did).must().is_empty());
    assert!(did_accounts(&endpoint, &did).must().is_empty());
    assert_eq!(did_accounts(&endpoint, &did).must(), vec![document]);
    for _ in 0..3 {
        assert_eq!(
            did_accounts(&endpoint, &did),
            Err(SequencerError::Malformed)
        );
    }
    assert_eq!(
        did_accounts(&endpoint, &did),
        Err(SequencerError::Rpc(refusal))
    );
    assert_eq!(server.join().must(), count);
}
