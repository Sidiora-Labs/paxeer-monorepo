use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::read::{Cursor, HistoryLimits, HistoryPage, HistoryReadError};
use layerx_client::read::HistoryKind;
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};
use serde_json::{json, Value};

fn serve(exchanges: Vec<(&'static str, Value, Value)>) -> (SequencerEndpoint, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let endpoint =
        SequencerEndpoint::new(&format!("http://{}/", listener.local_addr().unwrap()), &[])
            .unwrap();
    let server = thread::spawn(move || {
        for (method, params, answer) in exchanges {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(2)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(2)))
                .unwrap();
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).unwrap(), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(value.trim().parse::<usize>().unwrap());
                }
            }
            let length = length.unwrap();
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            reader.read_exact(&mut body).unwrap();
            let request: Value = serde_json::from_slice(&body).unwrap();
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let mut response = answer.as_object().unwrap().clone();
            response.insert("jsonrpc".into(), json!("2.0"));
            response.insert("id".into(), request["id"].clone());
            let response = serde_json::to_string(&response).unwrap();
            let mut stream = reader.into_inner();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()).unwrap();
            stream.flush().unwrap();
        }
        assert!(
            matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
        );
    });
    (endpoint, server)
}

fn position(block_height: u64, tx_index: u32, log_index: u32) -> Cursor {
    Cursor {
        block_height,
        tx_index,
        log_index,
    }
}

fn limits() -> HistoryLimits {
    HistoryLimits {
        maximum_items: 2,
        maximum_response_bytes: 4096,
    }
}

fn page(items: Vec<Value>, cursor: Option<Cursor>, height: Option<u64>) -> Value {
    json!({"items": items, "next_cursor": cursor.map(|cursor| cursor.to_hex()), "indexedHeight": height})
}

fn row(position: Cursor, id: &str) -> Value {
    json!({
        "id": id, "asset": "native", "tx_id": "transaction", "tx_hash": "transaction",
        "block_height": position.block_height.to_string(), "ordinal": (u64::from(position.log_index) + 1).to_string(),
        "blockHeight": position.block_height, "txIndex": position.tx_index, "logIndex": position.log_index,
        "data": {"amount": "123", "kind": "transfer"}
    })
}

#[test]
fn read_account_decodes_a_served_gateway_page() {
    let account = "0x1111111111111111111111111111111111111111";
    let positions = [
        position(100, 0, 0),
        position(100, 0, 1),
        position(101, 1, 0),
    ];
    let rows = vec![
        row(positions[0], "first"),
        row(positions[1], "second"),
        row(positions[2], "third"),
    ];
    let first = page(rows[..2].to_vec(), Some(positions[1]), Some(150));
    let continuation = page(rows[2..].to_vec(), None, Some(150));
    let empty = page(vec![], None, Some(150));
    let mut exchanges = Vec::new();
    for (cursor, result) in [
        (None, first.clone()),
        (Some(positions[1]), continuation),
        (None, empty),
    ] {
        exchanges.push((
            "px_getHistory",
            json!([account, cursor.map(|cursor| cursor.to_hex()), 2]),
            json!({"result": result}),
        ));
        exchanges.push(("eth_blockNumber", json!([]), json!({"result": "0xa0"})));
    }
    let (endpoint, server) = serve(exchanges);
    let first_page =
        HistoryPage::read_account(&endpoint, account, 100, 160, None, limits()).unwrap();
    assert_eq!(first_page.height, 150);
    assert_eq!(first_page.items.len(), 2);
    assert_eq!(first_page.next_cursor, Some(positions[1]));
    assert_eq!(
        first_page.next_cursor.unwrap().to_hex(),
        "00000000000000640000000000000001"
    );
    assert_eq!(
        first_page.response_bytes,
        serde_json::to_vec(&first).unwrap().len()
    );
    let next_page = HistoryPage::read_account(
        &endpoint,
        account,
        100,
        160,
        first_page.next_cursor,
        limits(),
    )
    .unwrap();
    assert_eq!(next_page.height, 150);
    assert_eq!(next_page.items.len(), 1);
    assert_eq!(next_page.next_cursor, None);
    let items: Vec<_> = first_page
        .items
        .into_iter()
        .chain(next_page.items)
        .collect();
    assert_eq!(
        items.iter().map(|item| item.position).collect::<Vec<_>>(),
        positions
    );
    for (item, row) in items.iter().zip(&rows) {
        assert_eq!(item.kind, HistoryKind::Event);
        assert_eq!(item.proof, None);
        assert_eq!(item.canonical, serde_json::to_vec(row).unwrap());
        assert_eq!(
            serde_json::from_slice::<Value>(&item.canonical).unwrap(),
            *row
        );
    }
    let empty_page =
        HistoryPage::read_account(&endpoint, account, 100, 160, None, limits()).unwrap();
    assert!(empty_page.items.is_empty());
    assert_eq!(empty_page.next_cursor, None);
    assert_eq!(empty_page.height, 150);
    server.join().unwrap();

    // Null is preserved as an unknown index height, rather than claiming a height.
    let (endpoint, server) = serve(vec![
        (
            "px_getHistory",
            json!([account, null, 2]),
            json!({"result": page(vec![], None, None)}),
        ),
        ("eth_blockNumber", json!([]), json!({"result": "0xa0"})),
    ]);
    let unknown = endpoint.history(account, None, 2).unwrap();
    assert_eq!(unknown.indexed_height, None);
    assert!(unknown.items.is_empty());
    assert_eq!(unknown.next_cursor, None);
    server.join().unwrap();

    let mut descending = rows[..2].to_vec();
    descending.reverse();
    let mut overflow = rows[0].clone();
    overflow["txIndex"] = json!(u64::from(u32::MAX) + 1);
    let mut string_position = rows[0].clone();
    string_position["blockHeight"] = json!("100");
    for (result, expected, head) in [
        (
            page(descending, None, Some(150)),
            HistoryReadError::Cursor,
            "0xa0",
        ),
        (
            page(vec![rows[0].clone(), rows[0].clone()], None, Some(150)),
            HistoryReadError::Cursor,
            "0xa0",
        ),
        (
            page(rows.clone(), None, Some(150)),
            HistoryReadError::Oversized,
            "0xa0",
        ),
        (
            page(vec![overflow], None, Some(150)),
            HistoryReadError::Malformed,
            "0xa0",
        ),
        (
            page(vec![string_position], None, Some(150)),
            HistoryReadError::Malformed,
            "0xa0",
        ),
        (
            page(vec![row(position(99, 0, 0), "before")], None, Some(150)),
            HistoryReadError::Cursor,
            "0xa0",
        ),
        (
            page(vec![row(position(151, 0, 0), "unindexed")], None, Some(150)),
            HistoryReadError::Cursor,
            "0xa0",
        ),
        (
            page(
                vec![row(position(101, 0, 0), "above-head")],
                None,
                Some(150),
            ),
            HistoryReadError::Cursor,
            "0x64",
        ),
        (
            page(vec![rows[0].clone()], Some(positions[1]), Some(150)),
            HistoryReadError::Cursor,
            "0xa0",
        ),
    ] {
        let (endpoint, server) = serve(vec![
            (
                "px_getHistory",
                json!([account, null, 2]),
                json!({"result": result}),
            ),
            ("eth_blockNumber", json!([]), json!({"result": head})),
        ]);
        assert_eq!(
            HistoryPage::read_account(&endpoint, account, 100, 160, None, limits()),
            Err(expected)
        );
        server.join().unwrap();
    }
}

#[test]
fn read_account_propagates_a_served_refusal() {
    let account = "0x1111111111111111111111111111111111111111";
    let refusal = RpcRefusal {
        code: -32602,
        message: "Invalid params".into(),
        data: Some(json!({"reason": "invalid_account"})),
    };
    let (endpoint, server) = serve(vec![(
        "px_getHistory",
        json!([account, null, 2]),
        json!({"error": {"code": refusal.code, "message": refusal.message, "data": refusal.data}}),
    )]);
    assert_eq!(
        HistoryPage::read_account(&endpoint, account, 100, 150, None, limits()),
        Err(HistoryReadError::Sequencer(SequencerError::Rpc(refusal)))
    );
    server.join().unwrap();
}
