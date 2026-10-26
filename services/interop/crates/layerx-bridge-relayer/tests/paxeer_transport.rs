use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_bridge_relayer::transport::{PaxeerEndpoint, PaxeerTransport, TransportFault};
use openssl::asn1::Asn1Time;
use openssl::hash::MessageDigest;
use openssl::pkey::PKey;
use openssl::rsa::Rsa;
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};

const CHAIN_125: &str = r#"{"jsonrpc":"2.0","id":1,"result":"0x7d"}"#;
const CHAIN_126: &str = r#"{"jsonrpc":"2.0","id":1,"result":"0x7e"}"#;
const BLOCK_NUMBER: &str = r#"{"jsonrpc":"2.0","id":1,"result":"0x102030"}"#;
const RPC_ERROR: &str =
    r#"{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"execution reverted"}}"#;
const TIMEOUT: Duration = Duration::from_secs(3);

fn response(body: &str, chunked: bool) -> Vec<u8> {
    if chunked {
        let split = body.len() / 2;
        format!(
            "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n{:x}\r\n{}\r\n{:x}\r\n{}\r\n0\r\n\r\n",
            split, &body[..split], body.len() - split, &body[split..]
        ).into_bytes()
    } else {
        format!(
            "HTTP/1.1 200 OK\r\nContent-Length: {}\r\n\r\n{}",
            body.len(),
            body
        )
        .into_bytes()
    }
}

fn request(stream: &mut TcpStream) -> Value {
    stream.set_read_timeout(Some(TIMEOUT)).unwrap();
    stream.set_write_timeout(Some(TIMEOUT)).unwrap();
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader.read_line(&mut line).unwrap();
    assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
    let mut length = None;
    loop {
        line.clear();
        assert_ne!(reader.read_line(&mut line).unwrap(), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.strip_prefix("Content-Length: ") {
            length = Some(value.trim().parse::<usize>().unwrap());
        }
    }
    let mut bytes = vec![0; length.unwrap()];
    reader.read_exact(&mut bytes).unwrap();
    let value: Value = serde_json::from_slice(&bytes).unwrap();
    assert_eq!(value["jsonrpc"], "2.0");
    assert_eq!(value["id"], 1);
    value
}

fn emulator(answers: Vec<Vec<u8>>) -> (PaxeerEndpoint, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = PaxeerEndpoint::new(
        format!("http://{}/rpc", listener.local_addr().unwrap()),
        TIMEOUT,
        None,
        125,
        true,
    )
    .unwrap();
    listener.set_nonblocking(true).unwrap();
    let worker = thread::spawn(move || {
        let mut requests = Vec::new();
        for answer in answers {
            let deadline = Instant::now() + TIMEOUT;
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing JSON-RPC request");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept failed: {error}"),
                }
            };
            requests.push(request(&mut stream));
            stream.write_all(&answer).unwrap();
            stream.flush().unwrap();
        }
        // Keep the listener alive to catch a forbidden method after a mismatch.
        let deadline = Instant::now() + Duration::from_millis(100);
        while Instant::now() < deadline {
            match listener.accept() {
                Ok(_) => panic!("unexpected JSON-RPC request after the recorded answers"),
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(2))
                }
                Err(error) => panic!("accept failed: {error}"),
            }
        }
        requests
    });
    (endpoint, worker)
}

#[test]
fn a_loopback_emulator_answers_after_the_chain_id_check() {
    let (endpoint, worker) = emulator(vec![
        response(CHAIN_125, false),
        response(BLOCK_NUMBER, false),
        response(CHAIN_125, true),
        response(BLOCK_NUMBER, true),
        response(CHAIN_125, false),
        response(RPC_ERROR, true),
        response(CHAIN_125, true),
    ]);
    let params = json!(["latest", false]);
    assert_eq!(
        PaxeerTransport::call(&endpoint, "eth_getBlockByNumber", &params),
        Ok(json!("0x102030"))
    );
    assert_eq!(
        PaxeerTransport::call(&endpoint, "eth_blockNumber", &json!([])),
        Ok(json!("0x102030"))
    );
    assert_eq!(
        PaxeerTransport::call(&endpoint, "eth_call", &json!([{"to": "0x1234"}, "latest"])),
        Err(TransportFault::Rpc {
            code: -32000,
            message: "execution reverted".to_owned()
        })
    );
    assert_eq!(
        PaxeerTransport::call(&endpoint, "eth_chainId", &json!([])),
        Ok(json!("0x7d"))
    );
    let requests = worker.join().unwrap();
    assert_eq!(
        requests
            .iter()
            .map(|value| value["method"].as_str().unwrap())
            .collect::<Vec<_>>(),
        [
            "eth_chainId",
            "eth_getBlockByNumber",
            "eth_chainId",
            "eth_blockNumber",
            "eth_chainId",
            "eth_call",
            "eth_chainId"
        ]
    );
    for index in [0, 2, 4, 6] {
        assert_eq!(requests[index]["params"], json!([]));
    }
    assert_eq!(requests[1]["params"], params);
    assert_eq!(requests[5]["params"], json!([{"to": "0x1234"}, "latest"]));

    for (wire, expected) in [
        (
            b"HTTP/1.1 200 OK\r\nContent-Length: 4194305\r\n\r\n".to_vec(),
            TransportFault::ResponseTooLarge,
        ),
        (
            b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n400001\r\n".to_vec(),
            TransportFault::ResponseTooLarge,
        ),
        (
            b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n"
                .to_vec(),
            TransportFault::AmbiguousFraming,
        ),
        (
            b"HTTP/1.1 200 OK\r\n\r\n{}".to_vec(),
            TransportFault::AmbiguousFraming,
        ),
        (
            b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\nContent-Length: 0\r\n\r\n".to_vec(),
            TransportFault::AmbiguousFraming,
        ),
    ] {
        let (endpoint, worker) = emulator(vec![response(CHAIN_125, false), wire]);
        assert_eq!(
            PaxeerTransport::call(&endpoint, "eth_blockNumber", &json!([])),
            Err(expected)
        );
        assert_eq!(worker.join().unwrap().len(), 2);
    }
}

#[test]
fn a_wrong_chain_id_is_refused_before_the_method_is_sent() {
    let (endpoint, worker) = emulator(vec![response(CHAIN_126, false)]);
    assert_eq!(
        PaxeerTransport::call(&endpoint, "eth_sendRawTransaction", &json!(["0x1234"])),
        Err(TransportFault::ChainMismatch {
            expected: 125,
            actual: 126
        })
    );
    let requests = worker.join().unwrap();
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0]["method"], "eth_chainId");
    assert_eq!(requests[0]["params"], json!([]));
}

#[test]
fn a_plaintext_url_off_loopback_is_refused() {
    for url in [
        "http://192.0.2.1/rpc",
        "http://rpc.example/rpc",
        "http://localhost.example/rpc",
        "http://[2001:db8::1]/rpc",
    ] {
        assert_eq!(
            PaxeerEndpoint::new(url, TIMEOUT, None, 125, true),
            Err(TransportFault::InsecureTransport)
        );
    }
    for url in [
        "http://127.0.0.1/rpc",
        "http://localhost/rpc",
        "http://[::1]/rpc",
    ] {
        assert!(PaxeerEndpoint::new(url, TIMEOUT, None, 125, true).is_ok());
        assert_eq!(
            PaxeerEndpoint::new(url, TIMEOUT, None, 125, false),
            Err(TransportFault::InsecureTransport)
        );
    }
    for anchor in [None, Some(vec![]), Some(vec![1, 2, 3])] {
        assert_eq!(
            PaxeerEndpoint::new("https://localhost/rpc", TIMEOUT, anchor, 125, true),
            Err(TransportFault::InsecureTransport)
        );
    }
    for url in [
        "ftp://localhost/rpc",
        "http://localhost:0/rpc",
        "http://localhost/rpc\r\nInjected: yes",
        "http://user@localhost/rpc",
    ] {
        assert_eq!(
            PaxeerEndpoint::new(url, TIMEOUT, None, 125, true),
            Err(TransportFault::UnsupportedUrl)
        );
    }
    assert!(matches!(
        PaxeerEndpoint::new("http://localhost/rpc", Duration::ZERO, None, 125, true),
        Err(TransportFault::UnexpectedValue { .. })
    ));
    assert!(matches!(
        PaxeerEndpoint::new("http://localhost/rpc", TIMEOUT, None, 0, true),
        Err(TransportFault::UnexpectedValue { .. })
    ));
    let mut endpoint =
        PaxeerEndpoint::new("http://localhost/rpc", TIMEOUT, None, 125, true).unwrap();
    endpoint.url = "http://192.0.2.1/rpc".to_owned();
    assert_eq!(
        PaxeerTransport::call(&endpoint, "eth_blockNumber", &json!([])),
        Err(TransportFault::InsecureTransport)
    );
}

fn certificate() -> Vec<u8> {
    let key = PKey::from_rsa(Rsa::generate(2048).unwrap()).unwrap();
    let mut name = X509NameBuilder::new().unwrap();
    name.append_entry_by_text("CN", "localhost").unwrap();
    let name = name.build();
    let mut certificate = X509::builder().unwrap();
    certificate.set_version(2).unwrap();
    certificate.set_subject_name(&name).unwrap();
    certificate.set_issuer_name(&name).unwrap();
    certificate.set_pubkey(&key).unwrap();
    certificate
        .set_not_before(&Asn1Time::days_from_now(0).unwrap())
        .unwrap();
    certificate
        .set_not_after(&Asn1Time::days_from_now(1).unwrap())
        .unwrap();
    certificate.sign(&key, MessageDigest::sha256()).unwrap();
    certificate.build().to_der().unwrap()
}

#[test]
fn an_https_endpoint_without_a_trust_anchor_is_refused() {
    let der = certificate();
    assert!(PaxeerEndpoint::new(
        "https://localhost/rpc",
        TIMEOUT,
        Some(der.clone()),
        125,
        false
    )
    .is_ok());
    let mut trailing = der.clone();
    trailing.push(0);
    let mut two_certificates = der.clone();
    two_certificates.extend_from_slice(&der);
    let pem = X509::from_der(&der).unwrap().to_pem().unwrap();
    for anchor in [
        None,
        Some(vec![]),
        Some(vec![0x30, 0]),
        Some(trailing),
        Some(two_certificates),
        Some(pem),
    ] {
        assert_eq!(
            PaxeerEndpoint::new("https://localhost/rpc", TIMEOUT, anchor, 125, false),
            Err(TransportFault::InvalidTrustAnchor)
        );
    }
    assert_eq!(
        PaxeerEndpoint::new("https://localhost/rpc", TIMEOUT, Some(der), 125, true),
        Err(TransportFault::InsecureTransport)
    );
}
