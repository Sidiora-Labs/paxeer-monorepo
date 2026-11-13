use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::read::{Cursor, HistoryLimits, HistoryPage, HistoryReadError};
use layerx_client::rpc::history::HistoryKind;
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};
use serde_json::{json, Value};

fn serve(exchanges: Vec<(&'static str, Value, Value)>) -> (SequencerEndpoint, JoinHandle<()>) {
    let listener = assert_ok(TcpListener::bind("127.0.0.1:0"), "unexpected error");
    assert_ok(listener.set_nonblocking(true), "unexpected error");
    let endpoint = assert_ok(
        SequencerEndpoint::new(
            &format!(
                "http://{}/",
                assert_ok(listener.local_addr(), "unexpected error")
            ),
            &[],
        ),
        "unexpected error",
    );
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
            assert_ok(
                stream.set_read_timeout(Some(Duration::from_secs(2))),
                "unexpected error",
            );
            assert_ok(
                stream.set_write_timeout(Some(Duration::from_secs(2))),
                "unexpected error",
            );
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(
                    assert_ok(reader.read_line(&mut line), "unexpected error"),
                    0
                );
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(assert_ok(value.trim().parse::<usize>(), "unexpected error"));
                }
            }
            let length = assert_some(length, "expected Some value");
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            assert_ok(reader.read_exact(&mut body), "unexpected error");
            let request: Value = assert_ok(serde_json::from_slice(&body), "unexpected error");
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let mut response = assert_some(answer.as_object(), "expected Some value").clone();
            response.insert("jsonrpc".into(), json!("2.0"));
            response.insert("id".into(), request["id"].clone());
            let response = assert_ok(serde_json::to_string(&response), "unexpected error");
            let mut stream = reader.into_inner();
            assert_ok(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()), "unexpected error");
            assert_ok(stream.flush(), "unexpected error");
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

fn first_page_cursor(from_height: u64) -> String {
    position(from_height - 1, u32::MAX, u32::MAX).to_hex()
}

fn limits() -> HistoryLimits {
    HistoryLimits {
        maximum_items: 2,
        maximum_response_bytes: 4096,
    }
}

fn page(account: &str, items: Vec<Value>, cursor: Option<Cursor>, height: Option<u64>) -> Value {
    let transcript: Value = assert_ok(
        serde_json::from_str(include_str!(
            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
        )),
        "unexpected error",
    );
    json!({
        "account": account,
        "items": Value::Array(items),
        "next_cursor": cursor.map(|cursor| cursor.to_hex()),
        "indexedHeight": height,
        "chainHeight": 160,
        "freshness": transcript["result"]["freshness"]
    })
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
    let from_height = 100;
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
    assert_history_pagination(account, from_height, positions, &rows);
    // Null is preserved as an unknown index height, rather than claiming a height.
    let (endpoint, server) = serve(vec![(
        "px_getHistory",
        json!([account, null, 2]),
        json!({"result": page(account, vec![], None, None)}),
    )]);
    let unknown = assert_ok(endpoint.history(account, None, 2), "unexpected error");
    assert_eq!(unknown.indexed_height, None);
    assert!(unknown.items.is_empty());
    assert_eq!(unknown.next_cursor, None);
    assert_ok(server.join(), "unexpected error");

    for (mut result, expected, head, cursor) in invalid_history_cases(account, &rows, positions) {
        result["chainHeight"] = json!(assert_ok(
            u64::from_str_radix(&head[2..], 16),
            "unexpected error"
        ));
        let (endpoint, server) = serve(vec![(
            "px_getHistory",
            json!([
                account,
                cursor.map_or_else(|| first_page_cursor(from_height), |cursor| cursor.to_hex()),
                2
            ]),
            json!({"result": result}),
        )]);
        assert_eq!(
            HistoryPage::read_account(&endpoint, account, from_height, 160, cursor, limits()),
            Err(expected)
        );
        assert_ok(server.join(), "unexpected error");
    }
}

#[test]
fn read_account_accepts_a_self_transfer_pair_that_closes_the_page() {
    let account = "0x1111111111111111111111111111111111111111";
    let from_height = 100;
    let shared = position(100, 1, 2);
    let rows = vec![row(shared, "debit"), row(shared, "credit")];
    let result = page(account, rows.clone(), Some(shared), Some(150));
    let (endpoint, server) = serve(vec![(
        "px_getHistory",
        json!([account, first_page_cursor(from_height), 1]),
        json!({"result": result}),
    )]);
    let read = assert_ok(
        HistoryPage::read_account(
            &endpoint,
            account,
            from_height,
            160,
            None,
            HistoryLimits {
                maximum_items: 1,
                ..limits()
            },
        ),
        "unexpected error",
    );
    assert_eq!(read.items.len(), 2);
    assert_eq!(read.next_cursor, Some(shared));
    for (item, row) in read.items.iter().zip(&rows) {
        assert_eq!(item.position, shared);
        assert_eq!(
            item.canonical,
            assert_ok(serde_json::to_vec(row), "unexpected error")
        );
    }
    assert_ok(server.join(), "unexpected error");
}

#[test]
fn read_account_accepts_equal_positions_within_the_limit() {
    let account = "0x1111111111111111111111111111111111111111";
    let from_height = 100;
    let shared = position(100, 1, 2);
    let rows = vec![row(shared, "debit"), row(shared, "credit")];
    let (endpoint, server) = serve(vec![(
        "px_getHistory",
        json!([account, first_page_cursor(from_height), 2]),
        json!({"result": page(account, rows.clone(), None, Some(150))}),
    )]);
    let read = assert_ok(
        HistoryPage::read_account(&endpoint, account, from_height, 160, None, limits()),
        "unexpected error",
    );
    assert_eq!(read.items.len(), 2);
    assert_eq!(read.next_cursor, None);
    for (item, row) in read.items.iter().zip(&rows) {
        assert_eq!(item.position, shared);
        assert_eq!(
            item.canonical,
            assert_ok(serde_json::to_vec(row), "unexpected error")
        );
    }
    assert_ok(server.join(), "unexpected error");
}

#[test]
fn read_account_continues_after_a_shared_position_group() {
    let account = "0x1111111111111111111111111111111111111111";
    let from_height = 100;
    let before = position(100, 0, 0);
    let shared = position(100, 1, 2);
    let after = position(100, 1, 3);
    let rows = vec![
        row(before, "before"),
        row(shared, "first"),
        row(shared, "second"),
        row(shared, "third"),
    ];
    let later = row(after, "after");
    let (endpoint, server) = serve(vec![
        (
            "px_getHistory",
            json!([account, first_page_cursor(from_height), 2]),
            json!({"result": page(account, rows.clone(), Some(shared), Some(150))}),
        ),
        (
            "px_getHistory",
            json!([account, shared.to_hex(), 2]),
            json!({"result": page(account, vec![later.clone()], None, Some(150))}),
        ),
    ]);
    let first = assert_ok(
        HistoryPage::read_account(&endpoint, account, from_height, 160, None, limits()),
        "unexpected error",
    );
    assert_eq!(first.items.len(), rows.len());
    assert_eq!(first.next_cursor, Some(shared));
    for (item, row) in first.items.iter().zip(&rows) {
        assert_eq!(
            item.canonical,
            assert_ok(serde_json::to_vec(row), "unexpected error")
        );
    }
    assert_eq!(
        assert_some(first.items.last(), "expected Some value").position,
        shared
    );
    let next = assert_ok(
        HistoryPage::read_account(
            &endpoint,
            account,
            from_height,
            160,
            first.next_cursor,
            limits(),
        ),
        "unexpected error",
    );
    assert_eq!(next.items.len(), 1);
    assert_eq!(next.items[0].position, after);
    assert_eq!(
        next.items[0].canonical,
        assert_ok(serde_json::to_vec(&later), "unexpected error")
    );
    assert!(next.items.iter().all(|item| item.position > shared));
    assert_eq!(next.next_cursor, None);
    assert_ok(server.join(), "unexpected error");
}

#[test]
fn read_account_refuses_a_descending_row() {
    let account = "0x1111111111111111111111111111111111111111";
    let from_height = 100;
    for (earlier, later) in [
        (position(100, 0, 0), position(101, 0, 0)),
        (position(100, 0, 0), position(100, 1, 0)),
        (position(100, 1, 0), position(100, 1, 1)),
    ] {
        let (endpoint, server) = serve(vec![(
            "px_getHistory",
            json!([account, first_page_cursor(from_height), 2]),
            json!({"result": page(account, vec![row(later, "later"), row(earlier, "earlier")], None, Some(150))}),
        )]);
        assert_eq!(
            HistoryPage::read_account(&endpoint, account, from_height, 160, None, limits()),
            Err(HistoryReadError::Cursor),
        );
        assert_ok(server.join(), "unexpected error");
    }
}

#[test]
fn read_account_refuses_overflow_at_a_later_position() {
    let account = "0x1111111111111111111111111111111111111111";
    let from_height = 100;
    let before = position(100, 0, 0);
    let boundary = position(100, 1, 2);
    for later in [
        position(101, 0, 0),
        position(100, 2, 0),
        position(100, 1, 3),
    ] {
        for include_shared_overflow in [false, true] {
            let mut rows = vec![row(before, "before"), row(boundary, "boundary")];
            if include_shared_overflow {
                rows.push(row(boundary, "shared"));
            }
            rows.push(row(later, "later"));
            let (endpoint, server) = serve(vec![(
                "px_getHistory",
                json!([account, first_page_cursor(from_height), 2]),
                json!({"result": page(account, rows, Some(later), Some(150))}),
            )]);
            assert_eq!(
                HistoryPage::read_account(&endpoint, account, from_height, 160, None, limits()),
                Err(HistoryReadError::Oversized),
            );
            assert_ok(server.join(), "unexpected error");
        }
    }
}

#[test]
fn read_account_preserves_response_and_cursor_bounds() {
    let account = "0x1111111111111111111111111111111111111111";
    let from_height = 100;
    let shared = position(100, 1, 2);
    let rows = vec![row(shared, "debit"), row(shared, "credit")];
    let result = page(account, rows, Some(shared), Some(150));
    let response_bytes = assert_ok(
        serde_json::to_vec(&json!({
            "items": result["items"],
            "next_cursor": result["next_cursor"],
            "indexedHeight": result["indexedHeight"],
            "chainHeight": result["chainHeight"]
        })),
        "unexpected error",
    )
    .len();
    let mut malformed = result.clone();
    malformed["next_cursor"] = json!("not-a-cursor");
    let mut uppercase = result.clone();
    uppercase["next_cursor"] = json!("000000000000006A0000000100000002");
    for (result, byte_limit, expected) in [
        (
            result.clone(),
            response_bytes - 1,
            HistoryReadError::Oversized,
        ),
        (
            malformed,
            limits().maximum_response_bytes,
            HistoryReadError::Sequencer(SequencerError::Malformed),
        ),
        (
            uppercase,
            limits().maximum_response_bytes,
            HistoryReadError::Sequencer(SequencerError::Malformed),
        ),
    ] {
        let (endpoint, server) = serve(vec![(
            "px_getHistory",
            json!([account, first_page_cursor(from_height), 1]),
            json!({"result": result}),
        )]);
        assert_eq!(
            HistoryPage::read_account(
                &endpoint,
                account,
                from_height,
                160,
                None,
                HistoryLimits {
                    maximum_items: 1,
                    maximum_response_bytes: byte_limit
                },
            ),
            Err(expected),
        );
        assert_ok(server.join(), "unexpected error");
    }
}

#[test]
fn read_account_propagates_a_served_refusal() {
    let account = "0x1111111111111111111111111111111111111111";
    let from_height = 100;
    let refusal = RpcRefusal {
        code: -32_602,
        message: "Invalid params".into(),
        data: Some(json!({"reason": "invalid_account"})),
    };
    let (endpoint, server) = serve(vec![(
        "px_getHistory",
        json!([account, first_page_cursor(from_height), 2]),
        json!({"error": {"code": refusal.code, "message": refusal.message, "data": refusal.data}}),
    )]);
    assert_eq!(
        HistoryPage::read_account(&endpoint, account, from_height, 150, None, limits()),
        Err(HistoryReadError::Sequencer(SequencerError::Rpc(refusal)))
    );
    assert_ok(server.join(), "unexpected error");
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

#[track_caller]
fn assert_some<T>(value: Option<T>, message: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{message}"),
    }
}

fn assert_history_pagination(
    account: &str,
    from_height: u64,
    positions: [Cursor; 3],
    rows: &[Value],
) {
    let first = page(account, rows[..2].to_vec(), Some(positions[1]), Some(150));
    let continuation = page(account, rows[2..].to_vec(), None, Some(150));
    let empty = page(account, vec![], None, Some(150));
    let mut exchanges = Vec::new();
    for (cursor, result) in [
        (None, first.clone()),
        (Some(positions[1]), continuation),
        (None, empty),
    ] {
        exchanges.push((
            "px_getHistory",
            json!([
                account,
                cursor.map_or_else(|| first_page_cursor(from_height), |cursor| cursor.to_hex()),
                2
            ]),
            json!({"result": result}),
        ));
    }
    let (endpoint, server) = serve(exchanges);
    let first_page = assert_ok(
        HistoryPage::read_account(&endpoint, account, from_height, 160, None, limits()),
        "unexpected error",
    );
    assert_eq!(first_page.height, 150);
    assert_eq!(first_page.items.len(), 2);
    assert_eq!(first_page.next_cursor, Some(positions[1]));
    assert_eq!(
        assert_some(first_page.next_cursor, "expected Some value").to_hex(),
        "00000000000000640000000000000001"
    );
    assert_eq!(
        first_page.response_bytes,
        assert_ok(
            serde_json::to_vec(&json!({
                "items": first["items"],
                "next_cursor": first["next_cursor"],
                "indexedHeight": first["indexedHeight"],
                "chainHeight": first["chainHeight"]
            })),
            "unexpected error"
        )
        .len()
    );
    let next_page = assert_ok(
        HistoryPage::read_account(
            &endpoint,
            account,
            from_height,
            160,
            first_page.next_cursor,
            limits(),
        ),
        "unexpected error",
    );
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
    for (item, row) in items.iter().zip(rows) {
        assert_eq!(item.kind, HistoryKind::Event);
        assert_eq!(item.proof, None);
        assert_eq!(
            item.canonical,
            assert_ok(serde_json::to_vec(row), "unexpected error")
        );
        assert_eq!(
            assert_ok(
                serde_json::from_slice::<Value>(&item.canonical),
                "unexpected error"
            ),
            *row
        );
    }
    let empty_page = assert_ok(
        HistoryPage::read_account(&endpoint, account, from_height, 160, None, limits()),
        "unexpected error",
    );
    assert!(empty_page.items.is_empty());
    assert_eq!(empty_page.next_cursor, None);
    assert_eq!(empty_page.height, 150);
    assert_ok(server.join(), "unexpected error");
}

fn invalid_history_cases(
    account: &str,
    rows: &[Value],
    positions: [Cursor; 3],
) -> [(Value, HistoryReadError, &'static str, Option<Cursor>); 9] {
    let mut descending = rows[..2].to_vec();
    descending.reverse();
    let mut overflow = rows[0].clone();
    overflow["txIndex"] = json!(u64::from(u32::MAX) + 1);
    let mut string_position = rows[0].clone();
    string_position["blockHeight"] = json!("100");

    [
        (
            page(account, descending, None, Some(150)),
            HistoryReadError::Cursor,
            "0xa0",
            None,
        ),
        (
            page(
                account,
                vec![rows[0].clone(), rows[0].clone()],
                None,
                Some(150),
            ),
            HistoryReadError::Cursor,
            "0xa0",
            Some(positions[0]),
        ),
        (
            page(account, rows.to_vec(), None, Some(150)),
            HistoryReadError::Oversized,
            "0xa0",
            None,
        ),
        (
            page(account, vec![overflow], None, Some(150)),
            HistoryReadError::Malformed,
            "0xa0",
            None,
        ),
        (
            page(account, vec![string_position], None, Some(150)),
            HistoryReadError::Malformed,
            "0xa0",
            None,
        ),
        (
            page(
                account,
                vec![row(position(99, 0, 0), "before")],
                None,
                Some(150),
            ),
            HistoryReadError::Cursor,
            "0xa0",
            None,
        ),
        (
            page(
                account,
                vec![row(position(151, 0, 0), "unindexed")],
                None,
                Some(150),
            ),
            HistoryReadError::Cursor,
            "0xa0",
            None,
        ),
        (
            page(
                account,
                vec![row(position(101, 0, 0), "above-head")],
                None,
                Some(100),
            ),
            HistoryReadError::Cursor,
            "0x64",
            None,
        ),
        (
            page(
                account,
                vec![rows[0].clone()],
                Some(positions[1]),
                Some(150),
            ),
            HistoryReadError::Cursor,
            "0xa0",
            None,
        ),
    ]
}
