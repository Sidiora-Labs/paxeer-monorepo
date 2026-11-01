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

#[test]
fn history_pages_px_get_history_and_carries_the_next_cursor() {
    let account = "0x1212121212121212121212121212121212121212";
    let next = "000000000000abcd0000001200000034";
    let items = vec![
        json!({"kind": "transfer", "amount": "123", "details": {"memo": [1, null]}}),
        json!({"kind": "call", "transactionHash": format!("0x{}", "34".repeat(32))}),
    ];
    let result = json!({
        "items": items, "next_cursor": next, "indexedHeight": 43981, "chainHeight": 44000
    });
    let refusal = RpcRefusal {
        code: -32001,
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
                "items": [], "next_cursor": null, "indexedHeight": 43982, "chainHeight": 44001
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
            items,
            next_cursor: Some(next.to_owned()),
            indexed_height: 43981,
            chain_height: 44000,
        }
    );
    let last = history(&endpoint, account, page.next_cursor.as_deref(), None).must();
    assert_eq!(
        last,
        HistoryPage {
            items: vec![],
            next_cursor: None,
            indexed_height: 43982,
            chain_height: 44001,
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
    let did = "did:layerx:abc";
    let rows = vec![
        json!({"account_id": "12".repeat(32), "name": format!("agent:{did}:main"),
            "balance": "12345678901234567890", "metadata": {"tags": [true, null]}}),
        json!({"account_id": "34".repeat(32), "name": format!("agent:{did}:asset:upax"),
            "balance": "42", "locked": "7"}),
    ];
    let result = json!({
        "account": {"evm_address": null, "pax_address": null, "layerx_did": null,
            "layerx_account": null, "bound": false},
        "paxeer": null,
        "layerx": {"did": did, "accounts": rows}
    });
    let mut empty = result.clone();
    empty["layerx"]["accounts"] = json!([]);
    let mut unavailable = result.clone();
    unavailable["layerx"] = Value::Null;
    let refusal = RpcRefusal {
        code: -32001,
        message: "Read unavailable".to_owned(),
        data: Some(json!({"code": "native_read_unavailable"})),
    };
    let mut exchanges = vec![
        ("px_getAccount", json!([did]), json!({"result": result})),
        ("px_getAccount", json!([did]), json!({"result": empty})),
        (
            "px_getAccount",
            json!([did]),
            json!({"result": unavailable}),
        ),
    ];
    for layerx in [
        json!({"did": did}),
        json!({"accounts": null}),
        json!({"accounts": {}}),
    ] {
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
    assert_eq!(did_accounts(&endpoint, did).must(), rows);
    assert!(did_accounts(&endpoint, did).must().is_empty());
    assert_eq!(
        did_accounts(&endpoint, did),
        Err(SequencerError::Unavailable)
    );
    for _ in 0..3 {
        assert_eq!(did_accounts(&endpoint, did), Err(SequencerError::Malformed));
    }
    assert_eq!(
        did_accounts(&endpoint, did),
        Err(SequencerError::Rpc(refusal))
    );
    assert_eq!(server.join().must(), count);
}
