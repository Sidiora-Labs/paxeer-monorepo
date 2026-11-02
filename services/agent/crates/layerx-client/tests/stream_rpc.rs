use std::net::{TcpListener, TcpStream};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_client::rpc::stream::{subscribe, Cursor, EventStream, StreamItem};
use layerx_client::sequencer::SequencerError;
use layerx_client::subscription::SubscriptionTopic;
use serde_json::{json, Value};
use tungstenite::{Message, WebSocket};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

fn filter() -> SubscriptionTopic {
    SubscriptionTopic::Logs {
        address: Some([0xab; 20]),
        topics: vec![[0xcd; 32]],
    }
}

fn receive(socket: &mut WebSocket<TcpStream>) -> Value {
    match socket.read().must() {
        Message::Text(text) => serde_json::from_str(&text).must(),
        message => panic!("unexpected frame: {message:?}"),
    }
}

fn send(socket: &mut WebSocket<TcpStream>, value: Value) {
    socket.send(Message::Text(value.to_string().into())).must();
}

fn open(logs: Vec<Value>) -> (EventStream, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("ws://{}/events", listener.local_addr().must());
    let server = thread::spawn(move || {
        let (stream, _) = listener.accept().must();
        stream.set_read_timeout(Some(Duration::from_secs(5))).must();
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .must();
        let mut socket = tungstenite::accept(stream).must();
        assert_eq!(
            receive(&mut socket),
            json!({
                "jsonrpc":"2.0", "id":1, "method":"eth_subscribe",
                "params":["logs", {
                    "address":format!("0x{}", "ab".repeat(20)),
                    "topics":[format!("0x{}", "cd".repeat(32))]
                }]
            })
        );
        send(
            &mut socket,
            json!({"jsonrpc":"2.0", "id":1, "result":"0xlogs"}),
        );
        for log in logs {
            send(
                &mut socket,
                json!({
                    "jsonrpc":"2.0", "method":"eth_subscription",
                    "params":{"subscription":"0xlogs", "result":log}
                }),
            );
        }
        assert_eq!(
            receive(&mut socket),
            json!({"jsonrpc":"2.0", "id":2, "method":"eth_unsubscribe", "params":["0xlogs"]})
        );
        send(&mut socket, json!({"jsonrpc":"2.0", "id":2, "result":true}));
        assert!(matches!(socket.read().must(), Message::Close(_)));
        assert!(matches!(
            socket.flush(),
            Err(tungstenite::Error::ConnectionClosed)
        ));
    });
    (subscribe(&url, &[], filter()).must(), server)
}

fn log(block_number: u64, log_index: u64, data: &str) -> Value {
    json!({
        "address":format!("0x{}", "ab".repeat(20)),
        "topics":[format!("0x{}", "cd".repeat(32))],
        "blockNumber":format!("0x{block_number:x}"),
        "logIndex":format!("0x{log_index:x}"),
        "blockHash":format!("0x{}", "11".repeat(32)),
        "transactionHash":format!("0x{}", "22".repeat(32)),
        "transactionIndex":"0x0", "data":data, "removed":false
    })
}

#[test]
fn stream_subscribe_yields_log_events_from_eth_subscribe() {
    assert!(matches!(
        subscribe("ws://127.0.0.1:1/", &[], SubscriptionTopic::NewHeads),
        Err(SequencerError::Malformed)
    ));
    assert!(matches!(
        subscribe("wss://localhost/", &[], filter()),
        Err(SequencerError::Endpoint)
    ));

    let logs = vec![log(5, 0, "0x01"), log(5, 1, "0x02"), log(5, 0, "0x03")];
    let (mut stream, server) = open(logs.clone());
    assert_eq!(stream.cursor(), None);
    for (index, expected) in [0_u64, 1].into_iter().zip(&logs[..2]) {
        let StreamItem::Event(event) = stream.next_item().must() else {
            panic!("ordered log was reported as a gap");
        };
        let cursor = Cursor {
            block_number: 5,
            log_index: index,
        };
        assert_eq!(event.log, *expected);
        assert_eq!(event.cursor(), cursor);
        assert_eq!(stream.cursor(), Some(cursor));
    }
    let previous = Cursor {
        block_number: 5,
        log_index: 1,
    };
    let StreamItem::Gap(gap) = stream.next_item().must() else {
        panic!("backward log index was delivered as an event");
    };
    assert_eq!(gap.missing_first, 0);
    assert_eq!(gap.missing_last, 1);
    assert_eq!(gap.backfill_cursor(), previous);
    assert_eq!(stream.cursor(), Some(previous));
    assert_eq!(stream.next_item(), Err(SequencerError::Unavailable));
    stream.close().must();
    server.join().must();

    let logs = vec![log(5, 3, "0x04"), log(5, 8, "0x05"), log(6, 0, "0x06")];
    let (mut stream, server) = open(logs.clone());
    for (expected, cursor) in logs.into_iter().zip([
        Cursor {
            block_number: 5,
            log_index: 3,
        },
        Cursor {
            block_number: 5,
            log_index: 8,
        },
        Cursor {
            block_number: 6,
            log_index: 0,
        },
    ]) {
        let StreamItem::Event(event) = stream.next_item().must() else {
            panic!("filtered log or new block was reported as a gap");
        };
        assert_eq!(event.log, expected);
        assert_eq!(event.cursor(), cursor);
        assert_eq!(stream.cursor(), Some(cursor));
    }
    stream.close().must();
    assert_eq!(stream.next_item(), Err(SequencerError::Unavailable));
    server.join().must();

    for malformed in [
        json!({"logIndex":"0x0"}),
        json!({"blockNumber":"0x5"}),
        json!({"blockNumber":null, "logIndex":"0x0"}),
        json!({"blockNumber":"0x05", "logIndex":"0x0"}),
        json!({"blockNumber":"0x5", "logIndex":"0x10000000000000000"}),
        json!({"blockNumber":"0x5", "logIndex":"0x"}),
        json!({"blockNumber":"0x5", "logIndex":"1"}),
        json!({"blockNumber":"0x5", "logIndex":"0xg"}),
    ] {
        let first = log(5, 0, "0x01");
        let (mut stream, server) = open(vec![first.clone(), malformed]);
        let StreamItem::Event(event) = stream.next_item().must() else {
            panic!("first log was reported as a gap");
        };
        assert_eq!(event.log, first);
        assert_eq!(stream.next_item(), Err(SequencerError::Malformed));
        assert_eq!(stream.cursor(), Some(event.cursor()));
        assert_eq!(stream.next_item(), Err(SequencerError::Unavailable));
        stream.close().must();
        server.join().must();
    }
}
