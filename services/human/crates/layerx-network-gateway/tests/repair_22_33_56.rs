// Gateway endpoint, framing, deadline, and registry cases exercise production reads.

include!("support/required.rs");

#[allow(dead_code)]
mod hosted_codec {
    include!(concat!(env!("OUT_DIR"), "/hosted_evm.rs"));
}

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{mpsc, Arc};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_network_gateway::{
    decode_hex, encode_hex, rpc_request, rpc_result, AccountIdentifier, Evidence, GatewayEndpoint,
    GatewayError, WalletObservation,
};
use serde_json::{json, Value};

fn recorded_case(name: &str) -> Value {
    let vectors: Value = serde_json::from_str(include_str!(
        "../../../../platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
    ))
    .required();
    vectors["cases"]
        .as_array()
        .required()
        .iter()
        .find(|case| case["name"] == name)
        .required()
        .clone()
}

fn balances_result() -> Value {
    let identity = decode_hex(
        recorded_case("addr.getUnifiedAccount")["result"]
            .as_str()
            .required(),
    )
    .required();
    let identity = hosted_codec::decode_unified_account(&identity).required();
    let coins = decode_hex(
        recorded_case("bank.all_balances")["result"]
            .as_str()
            .required(),
    )
    .required();
    let balances: Vec<Value> = hosted_codec::decode_coins(&coins).required().into_iter().map(|(denom, amount)| {
        json!({"denom": denom, "amount": hosted_codec::uint256_decimal(&amount)})
    }).collect();
    assert_eq!(
        json!(balances),
        recorded_case("bank.all_balances")["out"]["response"]
    );
    json!({
        "account": {
            "evm_address": hosted_codec::address_hex(&identity.evm),
            "pax_address": identity.pax_address,
            "layerx_did": format!("did:layerx:{}", encode_hex(&identity.did_public_key)),
            "layerx_account": encode_hex(&identity.layerx_main_account_id), "bound": true
        },
        "height": 16, "balances": balances
    })
}

fn rpc_answer(request: &Value) -> Vec<u8> {
    assert_eq!(request["jsonrpc"], "2.0");
    assert_eq!(request["method"], "px_getBalances");
    assert_eq!(request["params"], json!([selector().canonical_text()]));
    serde_json::to_vec(&json!({"jsonrpc": "2.0", "id": request["id"], "result": balances_result()}))
        .required()
}

fn selector() -> AccountIdentifier {
    AccountIdentifier::parse("0x102132435465768798a9bacbdcedfe0f1e2d3c4b")
        .required_with("valid account selector")
}

fn read_request(stream: &mut TcpStream) -> (String, Value) {
    let mut bytes = Vec::new();
    let mut buffer = [0; 1024];
    loop {
        let count = stream.read(&mut buffer).required();
        assert_ne!(count, 0, "client must send its complete request");
        bytes.extend_from_slice(&buffer[..count]);
        assert!(bytes.len() < 65536);
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = std::str::from_utf8(&bytes[..end]).required();
            let length: usize = head
                .split("\r\n")
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse().required())
                })
                .required();
            if bytes.len() >= end + 4 + length {
                assert_eq!(bytes.len(), end + 4 + length, "one framed request only");
                return (
                    head.to_owned(),
                    serde_json::from_slice(&bytes[end + 4..]).required(),
                );
            }
        }
    }
}

struct Peer {
    endpoint: GatewayEndpoint,
    port: u16,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Peer {
    fn start(target: &str, secure: bool, serve: impl FnOnce(TcpStream) + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").required();
        listener.set_nonblocking(true).required();
        let port = listener.local_addr().required().port();
        let scheme = if secure { "https" } else { "http" };
        let endpoint = GatewayEndpoint::parse(&format!(
            "{scheme}://{}{target}",
            listener.local_addr().required()
        ))
        .required();
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let worker = thread::spawn(move || {
            let mut accept_steps = 0..3_000;
            while !stop.load(Ordering::Acquire) {
                assert!(accept_steps.next().is_some(), "real client must connect");
                match listener.accept() {
                    Ok((stream, _)) => {
                        stream
                            .set_read_timeout(Some(Duration::from_secs(2)))
                            .required();
                        stream
                            .set_write_timeout(Some(Duration::from_secs(2)))
                            .required();
                        serve(stream);
                        return;
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            }
        });
        Self {
            endpoint,
            port,
            stopped,
            worker: Some(worker),
        }
    }

    fn finish(mut self) {
        self.worker.take().required().join().required();
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn valid_response(body: &[u8]) -> Vec<u8> {
    let mut response = format!("HTTP/1.1 200 OK\r\ncOnTeNt-LeNgTh:\t{} \t\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n", body.len()).into_bytes();
    response.extend_from_slice(body);
    response
}

#[test]
fn gateway_registry_denom_producer_transport_consumer() {
    let peer = Peer::start("/", false, |mut stream| {
        let (_, request) = read_request(&mut stream);
        stream
            .write_all(&valid_response(&rpc_answer(&request)))
            .required();
    });
    let endpoint = GatewayEndpoint::parse(&format!("http://localhost:{}/", peer.port)).required();
    let balances = endpoint.get_balances(selector()).required();
    let expected = recorded_case("bank.all_balances")["out"]["response"].clone();
    for row in expected.as_array().required() {
        let denom = row["denom"].as_str().required();
        assert_eq!(
            balances.asset(denom).required().amount.to_string(),
            row["amount"].as_str().required()
        );
    }
    assert!(balances.asset("absent").is_none());
    let observed = WalletObservation::accept(balances).required();
    assert_eq!(observed.wallet(), selector().evm_address().required());
    assert_eq!(observed.joined().height, 16);
    assert_eq!(observed.joined().evidence, Evidence::GatewayReported);
    for row in expected.as_array().required() {
        assert_eq!(
            observed
                .balance(row["denom"].as_str().required())
                .required()
                .to_string(),
            row["amount"].as_str().required()
        );
    }
    assert_eq!(observed.balance("absent"), None);
    peer.finish();
}

#[test]
fn gateway_endpoint_parser_to_request() {
    for hostile in [
        "/ok\r\nX-Injected: yes",
        "/ok\nGET / HTTP/1.1",
        "/a b",
        "/a\tb",
        "/a\0b",
        "/a\u{7f}b",
        "/%",
        "/%0",
        "/%GG",
        "/%0d%0aInjected",
        "/%00",
        "/%09",
        "/%7f",
        "/fragment#ignored",
        "/back\\slash",
        "/unicode\u{85}",
        "/query?q=\r\nInjected:yes",
        "/query?q=%zz",
    ] {
        assert_eq!(
            GatewayEndpoint::parse(&format!("http://127.0.0.1:12345{hostile}")),
            Err(GatewayError::InvalidEndpoint),
            "{hostile:?}"
        );
    }
    for hostile in [
        " http://localhost/",
        "http://localhost/\n",
        "http://localhost: +80/",
        "http://localhost:+80/",
    ] {
        assert_eq!(
            GatewayEndpoint::parse(hostile),
            Err(GatewayError::InvalidEndpoint)
        );
    }
    for (target, expected) in [
        ("/", "/"),
        (
            "/bank%2Fcoins?q=a%20b&denom=ibc%2Fabc",
            "/bank%2Fcoins?q=a%20b&denom=ibc%2Fabc",
        ),
        ("?q=coins", "/?q=coins"),
    ] {
        let expected = expected.to_owned();
        let peer = Peer::start(target, false, move |mut stream| {
            let (head, request) = read_request(&mut stream);
            assert_eq!(
                head.split("\r\n").next(),
                Some(format!("POST {expected} HTTP/1.1").as_str())
            );
            assert_eq!(head.split("\r\n").count(), 6);
            stream
                .write_all(&valid_response(&rpc_answer(&request)))
                .required();
        });
        let observed =
            WalletObservation::accept(peer.endpoint.get_balances(selector()).required()).required();
        assert_eq!(observed.balance("ulxp"), Some(123_456_789.into()));
        peer.finish();
    }
}

#[test]
fn gateway_http_framing_producer_transport_consumer() {
    for case in [
        "valid",
        "truncated",
        "excess",
        "conflict",
        "duplicate",
        "list",
        "chunked",
        "chunked-length",
        "identity",
        "missing",
        "negative",
        "plus",
        "overflow",
        "folded",
        "bad-name",
        "bare-lf",
        "bad-version",
        "bad-status",
        "refused",
    ] {
        let peer = Peer::start("/", false, move |mut stream| {
            let (_, request) = read_request(&mut stream);
            let body = rpc_answer(&request);
            let length = body.len();
            let headers = match case {
                "valid" => {
                    stream.write_all(&valid_response(&body)).required();
                    return;
                }
                "truncated" => format!("Content-Length: {}", length + 5),
                "excess" => format!("Content-Length: {}", length - 1),
                "conflict" => format!("Content-Length: {length}\r\ncontent-length: {}", length + 5),
                "duplicate" => format!("Content-Length: {length}\r\nContent-Length: {length}"),
                "list" => format!("Content-Length: {length}, {length}"),
                "chunked" => "Transfer-Encoding: chunked".to_owned(),
                "chunked-length" => {
                    format!("Transfer-Encoding: chunked\r\nContent-Length: {length}")
                }
                "identity" => format!("Transfer-Encoding: identity\r\nContent-Length: {length}"),
                "missing" => "Connection: close".to_owned(),
                "negative" => format!("Content-Length: -{length}"),
                "plus" => format!("Content-Length: +{length}"),
                "overflow" => "Content-Length: 184467440737095516160".to_owned(),
                "folded" => format!("Content-Length: {length}\r\n x-folded: yes"),
                "bad-name" => format!("Content-Length : {length}"),
                "bare-lf" => format!("Content-Length: {length}\nX-Injected: yes"),
                "bad-version" | "bad-status" | "refused" => format!("Content-Length: {length}"),
                _ => unreachable!(),
            };
            let status = match case {
                "bad-version" => "HTTP/9.9 200 OK",
                "bad-status" => "HTTP/1.1 +200 OK",
                "refused" => "HTTP/1.1 503 Unavailable",
                _ => "HTTP/1.1 200 OK",
            };
            write!(stream, "{status}\r\n{headers}\r\n\r\n").required();
            if case == "chunked" {
                write!(stream, "{length:x}\r\n").required();
                stream.write_all(&body).required();
                stream.write_all(b"\r\n0\r\n\r\n").required();
            } else {
                stream.write_all(&body).required();
            }
        });
        let answer = peer.endpoint.get_balances(selector());
        if case == "valid" {
            let observed = WalletObservation::accept(answer.required()).required();
            assert_eq!(observed.balance("ulxp"), Some(123_456_789.into()));
        } else {
            assert!(
                matches!(answer, Err(GatewayError::Transport(_))),
                "{case}: {answer:?}"
            );
        }
        peer.finish();
    }
}

fn post_with_bounded_wait(endpoint: &GatewayEndpoint, body: &str) -> Result<Vec<u8>, GatewayError> {
    let (sent, received) = mpsc::channel();
    thread::scope(|scope| {
        scope.spawn(move || {
            sent.send(endpoint.post_with_timeout(body, Duration::from_millis(250)))
                .required();
        });
        received.recv_timeout(Duration::from_millis(500)).required()
    })
}

#[test]
fn gateway_total_deadline_fragmented_response() {
    let (sent, received) = mpsc::channel();
    let peer = Peer::start("/", false, move |mut stream| {
        let (_, request) = read_request(&mut stream);
        let body = rpc_answer(&request);
        write!(
            stream,
            "HTTP/1.1 200 OK\r\nContent-Length: {}\r\n\r\n",
            body.len()
        )
        .required();
        let mut fragments = 0;
        for byte in body {
            if stream.write_all(&[byte]).is_err() {
                break;
            }
            fragments += 1;
            thread::sleep(Duration::from_millis(20));
        }
        sent.send(fragments).required();
    });
    let answer = post_with_bounded_wait(
        &peer.endpoint,
        &rpc_request(56, "px_getBalances", &[json!(selector().canonical_text())]),
    );
    assert!(
        matches!(answer, Err(GatewayError::Transport(_))),
        "{answer:?}"
    );
    assert!(
        received.recv_timeout(Duration::from_secs(2)).required() >= 3,
        "peer must deliver multiple successful fragments"
    );
    peer.finish();
}

#[test]
fn gateway_total_deadline_tls_handshake() {
    let (stop, stopped) = mpsc::channel();
    let (hello, received) = mpsc::channel();
    let peer = Peer::start("/", true, move |mut stream| {
        let mut byte = [0; 1];
        assert_eq!(stream.read(&mut byte).required(), 1);
        assert_eq!(byte[0], 22, "client must start a real TLS handshake");
        hello.send(()).required();
        stopped.recv_timeout(Duration::from_secs(2)).required();
    });
    let answer = post_with_bounded_wait(&peer.endpoint, "{}");
    received.recv_timeout(Duration::from_secs(1)).required();
    stop.send(()).required();
    assert!(
        matches!(answer, Err(GatewayError::Transport(_))),
        "{answer:?}"
    );
    peer.finish();
}

#[test]
fn gateway_expired_deadline_starts_no_exchange() {
    let listener = TcpListener::bind("127.0.0.1:0").required();
    listener.set_nonblocking(true).required();
    let endpoint =
        GatewayEndpoint::parse(&format!("http://{}/", listener.local_addr().required())).required();
    let answer = endpoint.post_with_timeout("{}", Duration::ZERO);
    assert!(matches!(answer, Err(GatewayError::Transport(_))));
    assert!(matches!(
        listener.accept(),
        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock
    ));
    let endpoint = GatewayEndpoint::parse("http://localhost/").required();
    assert!(matches!(
        endpoint.post_with_timeout("{}", Duration::ZERO),
        Err(GatewayError::Transport(_))
    ));
}

#[test]
fn gateway_response_byte_ceiling() {
    let peer = Peer::start("/", false, |mut stream| {
        read_request(&mut stream);
        // Valid JSON-RPC followed by legal JSON whitespace would otherwise be
        // accepted by rpc_result; the production transport must enforce its cap.
        let mut body = br#"{"jsonrpc":"2.0","id":56,"result":true}"#.to_vec();
        body.resize(8 * 1024 * 1024 + 1, b' ');
        assert_eq!(rpc_result(56, &body), Ok(json!(true)));
        let _ = stream.write_all(&valid_response(&body));
    });
    let answer = peer.endpoint.post(&rpc_request(
        56,
        "px_getBalances",
        &[json!(selector().canonical_text())],
    ));
    assert!(
        matches!(answer, Err(GatewayError::Transport(ref message)) if message.contains("size limit")),
        "{answer:?}"
    );
    peer.finish();
}
