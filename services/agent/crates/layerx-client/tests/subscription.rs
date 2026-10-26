use std::net::{TcpListener, TcpStream};
use std::time::Duration;

use layerx_client::sequencer::SequencerError;
use layerx_client::subscription::{subscribe, SubscriptionTopic};
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

fn accept(listener: &TcpListener) -> WebSocket<TcpStream> {
    let (stream, _) = listener.accept().must();
    stream.set_read_timeout(Some(Duration::from_secs(5))).must();
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .must();
    tungstenite::accept(stream).must()
}

fn receive(socket: &mut WebSocket<TcpStream>) -> Value {
    loop {
        match socket.read().must() {
            Message::Text(text) => return serde_json::from_str(&text).must(),
            Message::Pong(_) => {}
            message => panic!("unexpected frame: {message:?}"),
        }
    }
}

fn send(socket: &mut WebSocket<TcpStream>, value: Value) {
    socket.send(Message::Text(value.to_string().into())).must();
}

#[test]
fn logs_subscription_sends_eth_subscribe_and_yields_each_notification() {
    for url in ["ws://example.com/", "http://localhost/", "wss://localhost/"] {
        assert!(matches!(
            subscribe(url, &[], SubscriptionTopic::NewHeads),
            Err(SequencerError::Endpoint)
        ));
    }
    assert!(matches!(
        subscribe(
            "wss://localhost/",
            &[&[1, 2, 3]],
            SubscriptionTopic::NewHeads
        ),
        Err(SequencerError::Endpoint)
    ));
    assert!(matches!(
        subscribe("ws://localhost/", &[&[]], SubscriptionTopic::NewHeads),
        Err(SequencerError::Endpoint)
    ));
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("ws://{}/events", listener.local_addr().must());
    let results = [
        json!({"address":format!("0x{}", "ab".repeat(20)), "topics":[format!("0x{}", "cd".repeat(32))], "data":"0x01", "logIndex":"0x0"}),
        json!({"address":format!("0x{}", "ab".repeat(20)), "topics":[format!("0x{}", "cd".repeat(32))], "data":"0x02", "logIndex":"0x1", "removed":true}),
    ];
    let sent = results.clone();
    let server = std::thread::spawn(move || {
        let mut socket = accept(&listener);
        assert_eq!(
            receive(&mut socket),
            json!({
                "jsonrpc":"2.0", "id":1, "method":"eth_subscribe",
                "params":["logs", {"address":format!("0x{}", "ab".repeat(20)), "topics":[format!("0x{}", "cd".repeat(32)), format!("0x{}", "ef".repeat(32))]}]
            })
        );
        send(
            &mut socket,
            json!({"jsonrpc":"2.0", "id":1, "result":"0xlogs"}),
        );
        socket.send(Message::Ping(vec![1, 2, 3].into())).must();
        for result in sent {
            send(
                &mut socket,
                json!({"jsonrpc":"2.0", "method":"eth_subscription", "params":{"subscription":"0xlogs", "result":result}}),
            );
        }
        for frame in [
            json!({"jsonrpc":"2.0", "method":"eth_subscription", "params":{"subscription":"0xother", "result":{}}}),
            json!({"jsonrpc":"2.0", "method":"eth_subscription", "params":{"subscription":"0xlogs"}}),
            json!({"jsonrpc":"1.0", "method":"eth_subscription", "params":{"subscription":"0xlogs", "result":{}}}),
            json!({"jsonrpc":"2.0", "method":"lx_subscription", "params":{"subscription":"0xlogs", "result":{}}}),
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_subscription", "params":{"subscription":"0xlogs", "result":{}}}),
        ] {
            send(&mut socket, frame);
        }
        socket.send(Message::Text("invalid json".into())).must();
        socket.send(Message::Binary(vec![1, 2].into())).must();
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
    let mut subscription = subscribe(
        &url,
        &[],
        SubscriptionTopic::Logs {
            address: Some([0xab; 20]),
            topics: vec![[0xcd; 32], [0xef; 32]],
        },
    )
    .must();
    for result in results {
        assert_eq!(subscription.next().must(), result);
    }
    for _ in 0..7 {
        assert_eq!(subscription.next(), Err(SequencerError::Malformed));
    }
    subscription.close().must();
    server.join().must();
}

#[test]
fn new_heads_subscription_sends_eth_unsubscribe_on_close() {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("ws://{}/", listener.local_addr().must());
    let server = std::thread::spawn(move || {
        let mut socket = accept(&listener);
        assert_eq!(
            receive(&mut socket),
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_subscribe", "params":["newHeads"]})
        );
        send(
            &mut socket,
            json!({"jsonrpc":"2.0", "id":1, "result":"0xheads"}),
        );
        send(
            &mut socket,
            json!({"jsonrpc":"2.0", "method":"eth_subscription", "params":{"subscription":"0xheads", "result":{"number":"0x2a", "hash":"0xabcd"}}}),
        );
        assert_eq!(
            receive(&mut socket),
            json!({"jsonrpc":"2.0", "id":2, "method":"eth_unsubscribe", "params":["0xheads"]})
        );
        send(
            &mut socket,
            json!({"jsonrpc":"2.0", "method":"eth_subscription", "params":{"subscription":"0xheads", "result":{"number":"0x2b"}}}),
        );
        send(&mut socket, json!({"jsonrpc":"2.0", "id":2, "result":true}));
        assert!(matches!(socket.read().must(), Message::Close(_)));
        assert!(matches!(
            socket.flush(),
            Err(tungstenite::Error::ConnectionClosed)
        ));
    });
    let mut subscription = subscribe(&url, &[], SubscriptionTopic::NewHeads).must();
    assert_eq!(
        subscription.next().must(),
        json!({"number":"0x2a", "hash":"0xabcd"})
    );
    subscription.close().must();
    server.join().must();
}
