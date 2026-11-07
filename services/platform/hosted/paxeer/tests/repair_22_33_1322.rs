include!("../src/main.rs");

use rustls::pki_types::ServerName;
use rustls::{ClientConfig, ClientConnection, RootCertStore};
use std::sync::{mpsc, Mutex};

static REPAIR_1322_SERIAL: Mutex<()> = Mutex::new(());

fn repair_1322_config(node: NodeEndpoint, comet: Option<NodeEndpoint>) -> Arc<Config> {
    let tls =
        ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_safe_default_protocol_versions()
            .expect("TLS protocol versions")
            .with_no_client_auth()
            .with_single_cert(
                vec![CertificateDer::from(REPAIR_1322_SERVER.to_vec())],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(REPAIR_1322_KEY.to_vec())),
            )
            .expect("committed test certificate and key");
    Arc::new(Config {
        listen: "127.0.0.1:0".parse().unwrap(),
        tls: Arc::new(tls),
        node,
        chain_id: 125,
        comet,
    })
}

fn repair_1322_client(address: SocketAddr) -> StreamOwned<ClientConnection, TcpStream> {
    let mut roots = RootCertStore::empty();
    roots
        .add(CertificateDer::from(REPAIR_1322_CA.to_vec()))
        .unwrap();
    let config =
        ClientConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_safe_default_protocol_versions()
            .unwrap()
            .with_root_certificates(roots)
            .with_no_client_auth();
    let client =
        ClientConnection::new(Arc::new(config), ServerName::try_from("localhost").unwrap())
            .unwrap();
    let socket = TcpStream::connect_timeout(&address, Duration::from_secs(2)).unwrap();
    socket
        .set_read_timeout(Some(Duration::from_secs(2)))
        .unwrap();
    socket
        .set_write_timeout(Some(Duration::from_secs(2)))
        .unwrap();
    StreamOwned::new(client, socket)
}

fn repair_1322_start(
    config: Arc<Config>,
    budget: Duration,
) -> (
    SocketAddr,
    mpsc::Receiver<Result<(), String>>,
    thread::JoinHandle<()>,
) {
    let listener = listen_and_report(config.listen, &mut Vec::new()).unwrap();
    let address = listener.local_addr().unwrap();
    let (sender, receiver) = mpsc::channel();
    let worker = thread::spawn(move || {
        let (socket, _) = listener.accept().unwrap();
        let permit = ConnectionPermit::acquire().expect("connection slot available");
        let result = handle_connection(&config, socket, permit, Instant::now() + budget);
        sender.send(result).unwrap();
    });
    (address, receiver, worker)
}

fn repair_1322_exchange(config: Arc<Config>, method: &str, path: &str, body: &[u8]) -> Response {
    let (address, result, worker) = repair_1322_start(config, Duration::from_secs(5));
    let mut client = repair_1322_client(address);
    write!(client, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
    client.write_all(body).unwrap();
    client.flush().unwrap();
    let mut reply = read_http_message(&mut client, 40 * 1024 * 1024, false).unwrap();
    let status = reply.headers[""]
        .split_whitespace()
        .nth(1)
        .unwrap()
        .parse()
        .unwrap();
    client.conn.send_close_notify();
    client.flush().unwrap();
    client.sock.shutdown(Shutdown::Write).unwrap();
    drop(client);
    result
        .recv_timeout(Duration::from_secs(2))
        .unwrap()
        .unwrap();
    worker.join().unwrap();
    Response {
        status,
        body: std::mem::take(&mut reply.body),
        retry_after: None,
        genesis_sha256: reply.headers.remove("x-layerx-genesis-sha256"),
    }
}

type Repair1322Session = Vec<(Value, Vec<u8>)>;

fn repair_1322_peer(sessions: Vec<Repair1322Session>) -> (NodeEndpoint, thread::JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let node = NodeEndpoint {
        address: listener.local_addr().unwrap(),
        path: "/".to_owned(),
    };
    let worker = thread::spawn(move || {
        for session in sessions {
            let (mut socket, _) = listener.accept().unwrap();
            socket
                .set_read_timeout(Some(Duration::from_secs(2)))
                .unwrap();
            socket
                .set_write_timeout(Some(Duration::from_secs(2)))
                .unwrap();
            for (expected, reply) in session {
                let request = parse_client_request(&mut socket).unwrap();
                assert_eq!(request.method, "POST");
                assert_eq!(request.path, "/");
                assert_eq!(
                    serde_json::from_slice::<Value>(&request.body).unwrap(),
                    expected
                );
                write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: keep-alive\r\n\r\n", reply.len()).unwrap();
                socket.write_all(&reply).unwrap();
                socket.flush().unwrap();
            }
            let mut extra = [0];
            assert_eq!(
                socket.read(&mut extra).unwrap(),
                0,
                "unexpected request forwarded after the fixture transcript"
            );
        }
    });
    (node, worker)
}

fn repair_1322_identity() -> Value {
    serde_json::json!({"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]})
}

fn repair_1322_bytes(value: Value) -> Vec<u8> {
    serde_json::to_vec(&value).unwrap()
}

#[test]
fn repair_22_33_1322_wrong_chain_never_receives_write_and_identity_is_revalidated() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    let tx = serde_json::json!({"jsonrpc":"2.0","id":17,"method":"eth_sendRawTransaction","params":["0x010203"]});
    let accepted =
        repair_1322_bytes(serde_json::json!({"jsonrpc":"2.0","id":17,"result":"0xabcdef"}));
    let wrong = repair_1322_bytes(serde_json::json!({"jsonrpc":"2.0","id":1,"result":"0x7e"}));
    let (node, peer) = repair_1322_peer(vec![
        vec![(repair_1322_identity(), wrong.clone())],
        vec![
            (
                repair_1322_identity(),
                repair_1322_bytes(serde_json::json!({"jsonrpc":"2.0","id":1,"result":"0x7d"})),
            ),
            (tx.clone(), accepted.clone()),
        ],
        vec![(repair_1322_identity(), wrong)],
    ]);
    let config = repair_1322_config(node, None);
    let body = repair_1322_bytes(tx);
    for expected_status in [503, 200, 503] {
        let response = repair_1322_exchange(Arc::clone(&config), "POST", "/", &body);
        assert_eq!(response.status, expected_status);
        if expected_status == 200 {
            assert_eq!(response.body, accepted);
        } else {
            assert_eq!(
                serde_json::from_slice::<Value>(&response.body).unwrap()["error"]["code"],
                "chain_id_mismatch"
            );
        }
    }
    peer.join().unwrap();
}

#[test]
fn repair_22_33_1322_ambiguous_identity_never_receives_write() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    let replies = [
        r#"{"jsonrpc":"2.0","id":2,"result":"0x7d"}"#,
        r#"{"jsonrpc":"2.0","id":1,"result":"0x7d","error":{"code":-1}}"#,
        r#"{"jsonrpc":"2.0","id":1,"result":"0x7e","result":"0x7d"}"#,
        r#"{"jsonrpc":"2.0","id":1,"result":"0x"}"#,
        r#"{"jsonrpc":"2.0","id":1,"result":"0x+7d"}"#,
    ];
    let sessions = replies
        .iter()
        .map(|reply| vec![(repair_1322_identity(), reply.as_bytes().to_vec())])
        .collect();
    let (node, peer) = repair_1322_peer(sessions);
    let config = repair_1322_config(node, None);
    let body =
        br#"{"jsonrpc":"2.0","id":17,"method":"eth_sendRawTransaction","params":["0x010203"]}"#;
    for _ in replies {
        let response = repair_1322_exchange(Arc::clone(&config), "POST", "/", body);
        assert_eq!(response.status, 502);
        assert_eq!(
            serde_json::from_slice::<Value>(&response.body).unwrap()["error"]["code"],
            "node_response_invalid"
        );
    }
    peer.join().unwrap();
}

#[test]
fn repair_22_33_1322_custody_and_resource_refusals_do_not_contact_upstream() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let node = NodeEndpoint {
        address: listener.local_addr().unwrap(),
        path: "/".to_owned(),
    };
    let config = repair_1322_config(node.clone(), Some(node));
    for method in DENIED_METHODS {
        let body = repair_1322_bytes(
            serde_json::json!({"jsonrpc":"2.0","id":17,"method":method,"params":[]}),
        );
        let reply = repair_1322_exchange(Arc::clone(&config), "POST", "/", &body);
        assert_eq!(reply.status, 200);
        assert_eq!(
            serde_json::from_slice::<Value>(&reply.body).unwrap()["error"]["code"],
            -32601
        );
    }
    let body = br#"{"jsonrpc":"2.0","id":17,"method":"abci_query","params":{"path":"/store/layerxcustody/key","height":"17","data":"0x08ab","prove":true}}"#;
    let reply = repair_1322_exchange(Arc::clone(&config), "POST", "/comet", body);
    assert_eq!(reply.status, 200);
    assert_eq!(
        serde_json::from_slice::<Value>(&reply.body).unwrap()["error"]["code"],
        -32602
    );
    let reply = repair_1322_exchange(Arc::clone(&config), "POST", "/comet", &vec![b' '; 4097]);
    assert_eq!(reply.status, 413);
    let reply = repair_1322_exchange(config, "POST", "/rpc", b"{}");
    assert_eq!(reply.status, 404);
    listener.set_nonblocking(true).unwrap();
    match listener.accept() {
        Err(error) => assert_eq!(error.kind(), std::io::ErrorKind::WouldBlock),
        Ok(_) => panic!("refused request reached upstream"),
    }
}

#[test]
fn repair_22_33_1322_genesis_direct_transport_preserves_committed_fixture_bytes() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    let genesis = include_bytes!("fixtures/pacific-1.genesis.json");
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let node = NodeEndpoint {
        address: listener.local_addr().unwrap(),
        path: "/".to_owned(),
    };
    let producer = thread::spawn(move || {
        let (mut socket, _) = listener.accept().unwrap();
        socket
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        let request = parse_client_request(&mut socket).unwrap();
        assert_eq!(request.method, "GET");
        assert_eq!(request.path, "/genesis");
        let raw = std::str::from_utf8(genesis).unwrap().trim();
        let body = format!(r#"{{"jsonrpc":"2.0","id":-1,"result":{{"genesis":{raw}}}}}"#);
        write_response(&mut socket, &ok(body)).unwrap();
    });
    let config = repair_1322_config(node.clone(), Some(node));
    let reply = repair_1322_exchange(config, "GET", "/genesis", b"");
    producer.join().unwrap();
    assert_eq!(reply.status, 200);
    assert_eq!(
        reply.body,
        std::str::from_utf8(genesis).unwrap().trim().as_bytes()
    );
    let suite = rustls::crypto::ring::cipher_suite::TLS13_AES_128_GCM_SHA256
        .tls13()
        .unwrap();
    let digest = suite.common.hash_provider.hash(&reply.body);
    let expected: String = digest
        .as_ref()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    assert_eq!(reply.genesis_sha256.as_deref(), Some(expected.as_str()));
}

#[test]
fn repair_22_33_1322_status_transport_preserves_committed_transcript() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    let status: Value = serde_json::from_slice(include_bytes!("fixtures/gov/status.json")).unwrap();
    let request =
        serde_json::json!({"jsonrpc":"2.0","id":"status-fixture","method":"status","params":{}});
    let reply = repair_1322_bytes(
        serde_json::json!({"jsonrpc":"2.0","id":"status-fixture","result":status}),
    );
    let (node, peer) = repair_1322_peer(vec![vec![(request.clone(), reply.clone())]]);
    let config = repair_1322_config(node.clone(), Some(node));
    let response = repair_1322_exchange(config, "POST", "/comet", &repair_1322_bytes(request));
    assert_eq!(response.status, 200);
    assert_eq!(response.body, reply);
    peer.join().unwrap();
}

fn repair_1322_encode_base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut encoded = String::new();
    for group in bytes.chunks(3) {
        encoded.push(ALPHABET[usize::from(group[0] >> 2)] as char);
        encoded.push(
            ALPHABET[usize::from((group[0] & 3) << 4 | group.get(1).copied().unwrap_or(0) >> 4)]
                as char,
        );
        encoded.push(if group.len() > 1 {
            ALPHABET[usize::from((group[1] & 15) << 2 | group.get(2).copied().unwrap_or(0) >> 6)]
                as char
        } else {
            '='
        });
        encoded.push(if group.len() > 2 {
            ALPHABET[usize::from(group[2] & 63)] as char
        } else {
            '='
        });
    }
    encoded
}

#[test]
fn repair_22_33_1322_chunked_genesis_transport_preserves_committed_fixture_bytes() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    let genesis = include_bytes!("fixtures/pacific-1.genesis.json");
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let node = NodeEndpoint {
        address: listener.local_addr().unwrap(),
        path: "/".to_owned(),
    };
    let producer = thread::spawn(move || {
        let chunks: Vec<_> = genesis.chunks(genesis.len().div_ceil(2)).collect();
        for index in 0..=chunks.len() {
            let (mut socket, _) = listener.accept().unwrap();
            socket
                .set_read_timeout(Some(Duration::from_secs(2)))
                .unwrap();
            let request = read_http_message(&mut socket, MAX_REQUEST_BYTES, false).unwrap();
            let expected = if index == 0 {
                "/genesis".to_owned()
            } else {
                format!("/genesis_chunked?chunk={}", index - 1)
            };
            assert_eq!(request.headers[""], format!("GET {expected} HTTP/1.1"));
            let body = if index == 0 {
                serde_json::json!({"jsonrpc":"2.0","id":-1,"error":{"code":-32603,"message":"Internal error","data":"genesis response is large, please use the genesis_chunked API instead"}})
            } else {
                serde_json::json!({"jsonrpc":"2.0","id":-1,"result":{"chunk":(index - 1).to_string(),"total":chunks.len().to_string(),"data":repair_1322_encode_base64(chunks[index - 1])}})
            };
            write_response(&mut socket, &ok(body.to_string())).unwrap();
        }
    });
    let config = repair_1322_config(node.clone(), Some(node));
    let reply = repair_1322_exchange(config, "GET", "/genesis", b"");
    producer.join().unwrap();
    assert_eq!(reply.status, 200);
    assert_eq!(reply.body, genesis);
    let suite = rustls::crypto::ring::cipher_suite::TLS13_AES_128_GCM_SHA256
        .tls13()
        .unwrap();
    let digest = suite.common.hash_provider.hash(genesis);
    let expected: String = digest
        .as_ref()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    assert_eq!(reply.genesis_sha256.as_deref(), Some(expected.as_str()));
}

fn repair_1322_saturated_client(stage: &str) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let node = NodeEndpoint {
        address: listener.local_addr().unwrap(),
        path: "/".to_owned(),
    };
    let config = repair_1322_config(node, None);
    let permits: Vec<_> = (0..MAX_CONNECTIONS - 1)
        .map(|_| ConnectionPermit::acquire().unwrap())
        .collect();
    let budget = Duration::from_millis(400);
    let started = Instant::now();
    let (address, result, worker) = repair_1322_start(config, budget);
    let mut client = repair_1322_client(address);
    if stage == "handshake" {
        client.sock.write_all(b"\x16\x03\x03\x01\x00").unwrap();
    } else {
        client.conn.complete_io(&mut client.sock).unwrap();
        client.write_all(b"POST / HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n").unwrap();
        if stage == "body" {
            client.write_all(b"\r\n").unwrap();
        }
        client.flush().unwrap();
    }
    let mut progress = 0;
    let finished = loop {
        if let Ok(outcome) = result.try_recv() {
            break outcome;
        }
        assert!(
            started.elapsed() < budget + Duration::from_millis(250),
            "{stage} retained a saturated connection slot beyond its cumulative budget"
        );
        if stage == "handshake" {
            if client.sock.write_all(b"\0").is_ok() {
                progress += 1;
            }
        } else if client.write_all(b"a").and_then(|()| client.flush()).is_ok() {
            progress += 1;
        }
        if progress == 3 {
            assert!(
                ConnectionPermit::acquire().is_none(),
                "all connection slots must actually be occupied during progress"
            );
        }
        thread::sleep(Duration::from_millis(30));
    };
    assert!(
        finished.is_err(),
        "incomplete {stage} request cannot complete"
    );
    assert!(progress >= 3, "exercise repeated successful slow progress");
    let recovered = ConnectionPermit::acquire().expect("expired connection returns its slot");
    assert!(ConnectionPermit::acquire().is_none());
    drop(recovered);
    drop(client);
    worker.join().unwrap();
    drop(permits);
    listener.set_nonblocking(true).unwrap();
    assert_eq!(
        listener.accept().unwrap_err().kind(),
        std::io::ErrorKind::WouldBlock
    );
}

#[test]
fn repair_22_33_1322_slow_handshake_headers_and_body_release_saturated_slots() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    for stage in ["handshake", "headers", "body"] {
        repair_1322_saturated_client(stage);
        assert_eq!(ACTIVE_CONNECTIONS.load(Ordering::Acquire), 0);
    }
}

#[test]
fn repair_22_33_1322_slow_upstream_progress_shares_client_deadline() {
    let _serial = REPAIR_1322_SERIAL.lock().unwrap();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let node = NodeEndpoint {
        address: listener.local_addr().unwrap(),
        path: "/".to_owned(),
    };
    let (progress_sender, progress_receiver) = mpsc::channel();
    let peer = thread::spawn(move || {
        let (mut socket, _) = listener.accept().unwrap();
        socket
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        let request = parse_client_request(&mut socket).unwrap();
        assert_eq!(
            serde_json::from_slice::<Value>(&request.body).unwrap(),
            repair_1322_identity()
        );
        socket
            .write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n")
            .unwrap();
        let mut progress = 0;
        let stop = Instant::now() + Duration::from_secs(1);
        while Instant::now() < stop && socket.write_all(b" ").is_ok() {
            progress += 1;
            thread::sleep(Duration::from_millis(30));
        }
        progress_sender.send(progress).unwrap();
        let mut extra = [0];
        assert_eq!(
            socket.read(&mut extra).unwrap(),
            0,
            "write traffic must not pass an incomplete identity probe"
        );
    });
    let config = repair_1322_config(node, None);
    let permits: Vec<_> = (0..MAX_CONNECTIONS - 1)
        .map(|_| ConnectionPermit::acquire().unwrap())
        .collect();
    let budget = Duration::from_millis(400);
    let (address, result, worker) = repair_1322_start(config, budget);
    let mut client = repair_1322_client(address);
    let started = Instant::now();
    let body =
        br#"{"jsonrpc":"2.0","id":17,"method":"eth_sendRawTransaction","params":["0x010203"]}"#;
    write!(client, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n", body.len()).unwrap();
    client.write_all(body).unwrap();
    client.flush().unwrap();
    assert!(result
        .recv_timeout(budget + Duration::from_millis(250))
        .unwrap()
        .is_err());
    assert!(started.elapsed() < budget + Duration::from_millis(250));
    let recovered =
        ConnectionPermit::acquire().expect("upstream progress cannot retain the connection slot");
    assert!(ConnectionPermit::acquire().is_none());
    assert!(
        progress_receiver
            .recv_timeout(Duration::from_secs(2))
            .unwrap()
            >= 3
    );
    drop(recovered);
    drop(permits);
    drop(client);
    worker.join().unwrap();
    peer.join().unwrap();
    assert_eq!(ACTIVE_CONNECTIONS.load(Ordering::Acquire), 0);
}

const REPAIR_1322_CA: &[u8] = &[
    0x30, 0x82, 0x01, 0xa3, 0x30, 0x82, 0x01, 0x49, 0xa0, 0x03, 0x02, 0x01, 0x02, 0x02, 0x14, 0x70,
    0x8a, 0x46, 0x1e, 0x47, 0xf3, 0x7f, 0x66, 0xfe, 0x2b, 0x3b, 0x74, 0x91, 0x06, 0x49, 0x0b, 0x2a,
    0x3b, 0x18, 0xba, 0x30, 0x0a, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x04, 0x03, 0x02, 0x30,
    0x1e, 0x31, 0x1c, 0x30, 0x1a, 0x06, 0x03, 0x55, 0x04, 0x03, 0x0c, 0x13, 0x72, 0x65, 0x70, 0x61,
    0x69, 0x72, 0x2d, 0x31, 0x33, 0x32, 0x32, 0x2d, 0x74, 0x65, 0x73, 0x74, 0x2d, 0x63, 0x61, 0x30,
    0x20, 0x17, 0x0d, 0x32, 0x36, 0x31, 0x31, 0x30, 0x34, 0x31, 0x30, 0x33, 0x34, 0x31, 0x30, 0x5a,
    0x18, 0x0f, 0x32, 0x31, 0x32, 0x36, 0x31, 0x30, 0x31, 0x31, 0x31, 0x30, 0x33, 0x34, 0x31, 0x30,
    0x5a, 0x30, 0x1e, 0x31, 0x1c, 0x30, 0x1a, 0x06, 0x03, 0x55, 0x04, 0x03, 0x0c, 0x13, 0x72, 0x65,
    0x70, 0x61, 0x69, 0x72, 0x2d, 0x31, 0x33, 0x32, 0x32, 0x2d, 0x74, 0x65, 0x73, 0x74, 0x2d, 0x63,
    0x61, 0x30, 0x59, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x06, 0x08,
    0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07, 0x03, 0x42, 0x00, 0x04, 0x8b, 0xe8, 0xc5, 0xe4,
    0x9d, 0x49, 0xbe, 0x7e, 0x39, 0xa0, 0xfa, 0xfb, 0x0f, 0x27, 0xdd, 0xc1, 0x64, 0xad, 0x2c, 0xc0,
    0xee, 0xd1, 0x11, 0x17, 0xab, 0x06, 0x19, 0x9a, 0xf5, 0x93, 0xc0, 0x93, 0x9b, 0xc7, 0x40, 0x49,
    0xb0, 0x6c, 0xd8, 0xcb, 0x6e, 0x38, 0x33, 0xdd, 0x5b, 0xc0, 0xb1, 0x92, 0x48, 0xc7, 0xad, 0x1a,
    0xd5, 0xa7, 0x90, 0xc5, 0x88, 0xed, 0x23, 0x56, 0x6b, 0xde, 0x59, 0x48, 0xa3, 0x63, 0x30, 0x61,
    0x30, 0x1d, 0x06, 0x03, 0x55, 0x1d, 0x0e, 0x04, 0x16, 0x04, 0x14, 0x7c, 0xc5, 0x03, 0x24, 0xc0,
    0x92, 0x28, 0x17, 0x7d, 0xd0, 0x04, 0x7c, 0x6a, 0xd5, 0x16, 0x13, 0x76, 0xcc, 0xd2, 0x25, 0x30,
    0x1f, 0x06, 0x03, 0x55, 0x1d, 0x23, 0x04, 0x18, 0x30, 0x16, 0x80, 0x14, 0x7c, 0xc5, 0x03, 0x24,
    0xc0, 0x92, 0x28, 0x17, 0x7d, 0xd0, 0x04, 0x7c, 0x6a, 0xd5, 0x16, 0x13, 0x76, 0xcc, 0xd2, 0x25,
    0x30, 0x0f, 0x06, 0x03, 0x55, 0x1d, 0x13, 0x01, 0x01, 0xff, 0x04, 0x05, 0x30, 0x03, 0x01, 0x01,
    0xff, 0x30, 0x0e, 0x06, 0x03, 0x55, 0x1d, 0x0f, 0x01, 0x01, 0xff, 0x04, 0x04, 0x03, 0x02, 0x01,
    0x06, 0x30, 0x0a, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x04, 0x03, 0x02, 0x03, 0x48, 0x00,
    0x30, 0x45, 0x02, 0x21, 0x00, 0xa8, 0x83, 0xfe, 0x9f, 0x49, 0x3f, 0x9d, 0xfb, 0xb7, 0xb5, 0x22,
    0x02, 0x83, 0x24, 0xde, 0x06, 0x47, 0xe3, 0x8c, 0xb8, 0x44, 0x61, 0xb2, 0x97, 0x52, 0xa1, 0xab,
    0x2b, 0xfc, 0x2c, 0xe9, 0x8a, 0x02, 0x20, 0x6c, 0x9e, 0x27, 0x00, 0x98, 0x6a, 0xd8, 0x31, 0x34,
    0x4a, 0x89, 0x9f, 0xf3, 0x26, 0x46, 0x1e, 0x99, 0x3f, 0x9d, 0xdd, 0x6f, 0x8b, 0x75, 0x28, 0x5c,
    0xad, 0xf6, 0xfd, 0xea, 0xf8, 0x07, 0xbc,
];

const REPAIR_1322_SERVER: &[u8] = &[
    0x30, 0x82, 0x01, 0xc3, 0x30, 0x82, 0x01, 0x69, 0xa0, 0x03, 0x02, 0x01, 0x02, 0x02, 0x14, 0x11,
    0x28, 0xea, 0xaa, 0x49, 0x7e, 0x1e, 0x1a, 0xab, 0x67, 0x70, 0x87, 0x12, 0x30, 0xef, 0xd1, 0x29,
    0xd1, 0xdb, 0x86, 0x30, 0x0a, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x04, 0x03, 0x02, 0x30,
    0x1e, 0x31, 0x1c, 0x30, 0x1a, 0x06, 0x03, 0x55, 0x04, 0x03, 0x0c, 0x13, 0x72, 0x65, 0x70, 0x61,
    0x69, 0x72, 0x2d, 0x31, 0x33, 0x32, 0x32, 0x2d, 0x74, 0x65, 0x73, 0x74, 0x2d, 0x63, 0x61, 0x30,
    0x20, 0x17, 0x0d, 0x32, 0x36, 0x31, 0x31, 0x30, 0x34, 0x31, 0x30, 0x33, 0x34, 0x31, 0x30, 0x5a,
    0x18, 0x0f, 0x32, 0x31, 0x32, 0x36, 0x31, 0x30, 0x31, 0x31, 0x31, 0x30, 0x33, 0x34, 0x31, 0x30,
    0x5a, 0x30, 0x14, 0x31, 0x12, 0x30, 0x10, 0x06, 0x03, 0x55, 0x04, 0x03, 0x0c, 0x09, 0x6c, 0x6f,
    0x63, 0x61, 0x6c, 0x68, 0x6f, 0x73, 0x74, 0x30, 0x59, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48,
    0xce, 0x3d, 0x02, 0x01, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07, 0x03, 0x42,
    0x00, 0x04, 0x33, 0xee, 0x31, 0x12, 0x0f, 0xbc, 0xc6, 0xb7, 0xc9, 0xb7, 0xc7, 0x60, 0xb7, 0x21,
    0x4e, 0x19, 0x7d, 0x7a, 0x67, 0xfa, 0xf3, 0xe7, 0xc4, 0x42, 0x95, 0x4d, 0xd5, 0xd2, 0xc2, 0x5d,
    0x7d, 0x4c, 0xf2, 0xa6, 0x21, 0x49, 0x74, 0xa5, 0xcb, 0x59, 0x18, 0x09, 0xf3, 0xe6, 0xa4, 0x22,
    0x60, 0xb4, 0x7e, 0x49, 0x0d, 0xd8, 0xd2, 0x9e, 0x99, 0x3a, 0xfb, 0x7c, 0x38, 0x7d, 0x04, 0x1e,
    0x79, 0x28, 0xa3, 0x81, 0x8c, 0x30, 0x81, 0x89, 0x30, 0x0c, 0x06, 0x03, 0x55, 0x1d, 0x13, 0x01,
    0x01, 0xff, 0x04, 0x02, 0x30, 0x00, 0x30, 0x0e, 0x06, 0x03, 0x55, 0x1d, 0x0f, 0x01, 0x01, 0xff,
    0x04, 0x04, 0x03, 0x02, 0x07, 0x80, 0x30, 0x13, 0x06, 0x03, 0x55, 0x1d, 0x25, 0x04, 0x0c, 0x30,
    0x0a, 0x06, 0x08, 0x2b, 0x06, 0x01, 0x05, 0x05, 0x07, 0x03, 0x01, 0x30, 0x14, 0x06, 0x03, 0x55,
    0x1d, 0x11, 0x04, 0x0d, 0x30, 0x0b, 0x82, 0x09, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x68, 0x6f, 0x73,
    0x74, 0x30, 0x1d, 0x06, 0x03, 0x55, 0x1d, 0x0e, 0x04, 0x16, 0x04, 0x14, 0xc1, 0x29, 0x92, 0xce,
    0x12, 0x23, 0x60, 0x29, 0xa1, 0x2c, 0xa0, 0xcf, 0x4a, 0x95, 0xd3, 0xa8, 0xc2, 0xb6, 0x01, 0x89,
    0x30, 0x1f, 0x06, 0x03, 0x55, 0x1d, 0x23, 0x04, 0x18, 0x30, 0x16, 0x80, 0x14, 0x7c, 0xc5, 0x03,
    0x24, 0xc0, 0x92, 0x28, 0x17, 0x7d, 0xd0, 0x04, 0x7c, 0x6a, 0xd5, 0x16, 0x13, 0x76, 0xcc, 0xd2,
    0x25, 0x30, 0x0a, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x04, 0x03, 0x02, 0x03, 0x48, 0x00,
    0x30, 0x45, 0x02, 0x21, 0x00, 0xcd, 0x88, 0x63, 0xc9, 0x80, 0xf1, 0xcb, 0xb7, 0x20, 0xca, 0x21,
    0x40, 0xd4, 0x25, 0xba, 0x12, 0xdd, 0x99, 0x3e, 0xbc, 0xa9, 0xc2, 0xf5, 0x8f, 0x16, 0xd1, 0x42,
    0x88, 0x18, 0x9f, 0x8c, 0x35, 0x02, 0x20, 0x7a, 0x38, 0xc7, 0xda, 0x39, 0x7e, 0xb0, 0xb1, 0x23,
    0x0e, 0x28, 0x66, 0x2b, 0x4f, 0x87, 0xca, 0x0a, 0xd6, 0x6b, 0xe8, 0xd2, 0x40, 0xe6, 0x05, 0x7c,
    0x34, 0x0a, 0x13, 0x07, 0xa5, 0xd3, 0x06,
];

const REPAIR_1322_KEY: &[u8] = &[
    0x30, 0x81, 0x87, 0x02, 0x01, 0x00, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02,
    0x01, 0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07, 0x04, 0x6d, 0x30, 0x6b, 0x02,
    0x01, 0x01, 0x04, 0x20, 0xae, 0xe2, 0xf6, 0xf0, 0x2d, 0xf6, 0x0c, 0xbc, 0x0a, 0x5f, 0x36, 0x11,
    0x5f, 0x48, 0x69, 0xca, 0xd0, 0x4d, 0x7e, 0xd6, 0xd1, 0xe2, 0x64, 0xd8, 0x33, 0x26, 0x4b, 0x01,
    0xab, 0x09, 0x7a, 0xa9, 0xa1, 0x44, 0x03, 0x42, 0x00, 0x04, 0x33, 0xee, 0x31, 0x12, 0x0f, 0xbc,
    0xc6, 0xb7, 0xc9, 0xb7, 0xc7, 0x60, 0xb7, 0x21, 0x4e, 0x19, 0x7d, 0x7a, 0x67, 0xfa, 0xf3, 0xe7,
    0xc4, 0x42, 0x95, 0x4d, 0xd5, 0xd2, 0xc2, 0x5d, 0x7d, 0x4c, 0xf2, 0xa6, 0x21, 0x49, 0x74, 0xa5,
    0xcb, 0x59, 0x18, 0x09, 0xf3, 0xe6, 0xa4, 0x22, 0x60, 0xb4, 0x7e, 0x49, 0x0d, 0xd8, 0xd2, 0x9e,
    0x99, 0x3a, 0xfb, 0x7c, 0x38, 0x7d, 0x04, 0x1e, 0x79, 0x28,
];
