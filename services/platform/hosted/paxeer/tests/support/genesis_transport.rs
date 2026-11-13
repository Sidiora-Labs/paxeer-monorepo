fn digest_hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 15)]));
    }
    text
}

const FIXTURE: &[u8] = include_bytes!("../fixtures/pacific-1.genesis.json");

struct Reply {
    path: String,
    body: Vec<u8>,
    status: u16,
    length: Option<usize>,
    chunked: bool,
    delay: Duration,
}

impl Reply {
    fn new(path: &str, body: Vec<u8>) -> Self {
        Self {
            path: path.to_owned(),
            body,
            status: 200,
            length: None,
            chunked: false,
            delay: Duration::ZERO,
        }
    }
}

fn serve(responses: Vec<Reply>) -> (NodeEndpoint, thread::JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("reserve node transport port: {error:?}"));
    let node = NodeEndpoint {
        address: listener
            .local_addr()
            .unwrap_or_else(|error| panic!("node transport address: {error:?}")),
        path: "/".to_owned(),
    };
    let worker = thread::spawn(move || {
        for reply in responses {
            let (mut stream, _) = listener
                .accept()
                .unwrap_or_else(|error| panic!("accept node request: {error:?}"));
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("read bound: {error:?}"));
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("write bound: {error:?}"));
            let request = read_http_message(&mut stream, MAX_REQUEST_BYTES, false)
                .unwrap_or_else(|error| panic!("decode real transport request: {error}"));
            assert_eq!(request.headers[""], format!("GET {} HTTP/1.1", reply.path));
            assert!(request.body.is_empty());
            thread::sleep(reply.delay);
            let mut wire = Vec::new();
            if reply.chunked {
                write!(
                    wire,
                    "HTTP/1.1 {} OK\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n",
                    reply.status
                )
                .unwrap_or_else(|error| panic!("headers: {error:?}"));
                for part in reply.body.chunks(65536) {
                    write!(wire, "{:x}\r\n", part.len())
                        .unwrap_or_else(|error| panic!("chunk header: {error:?}"));
                    wire.extend_from_slice(part);
                    wire.extend_from_slice(b"\r\n");
                }
                wire.extend_from_slice(b"0\r\n\r\n");
            } else {
                write!(
                    wire,
                    "HTTP/1.1 {} OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    reply.status,
                    reply.length.unwrap_or(reply.body.len())
                )
                .unwrap_or_else(|error| panic!("headers: {error:?}"));
                wire.extend_from_slice(&reply.body);
            }
            if let Err(error) = stream.write_all(&wire) {
                assert!(
                    matches!(
                        error.kind(),
                        std::io::ErrorKind::BrokenPipe | std::io::ErrorKind::ConnectionReset
                    ),
                    "unexpected server error: {error}"
                );
            }
        }
    });
    (node, worker)
}

fn direct(bytes: &[u8], wrapped: bool) -> Vec<u8> {
    let mut body = if wrapped {
        br#"{"jsonrpc":"2.0","id":-1,"result":{"genesis":"#.to_vec()
    } else {
        br#"{"genesis":"#.to_vec()
    };
    body.extend_from_slice(bytes);
    body.extend_from_slice(if wrapped { b"}}" } else { b"}" });
    body
}

fn check_response(body: Vec<u8>, expected: Option<&[u8]>) {
    check_transcript(vec![Reply::new("/genesis", body)], expected);
}

fn check_transcript(replies: Vec<Reply>, expected: Option<&[u8]>) {
    let (node, worker) = serve(replies);
    let response = genesis::response(&node, Instant::now() + NODE_IO_TIMEOUT);
    worker
        .join()
        .unwrap_or_else(|error| panic!("node transcript consumed: {error:?}"));
    if let Some(expected) = expected {
        assert_eq!(response.status, 200);
        assert_eq!(response.body, expected);
        let hash = rustls::crypto::ring::cipher_suite::TLS13_AES_128_GCM_SHA256
            .tls13()
            .unwrap_or_else(|| panic!("SHA256 cipher suite"))
            .common
            .hash_provider
            .hash(expected);
        let digest = digest_hex(hash.as_ref());
        assert_eq!(response.genesis_sha256.as_deref(), Some(digest.as_str()));
        let mut wire = Vec::new();
        write_response(&mut wire, &response)
            .unwrap_or_else(|error| panic!("encode consumer response: {error:?}"));
        let decoded = read_http_message(
            &mut wire.as_slice(),
            MAX_REQUEST_BYTES + expected.len(),
            false,
        )
        .unwrap_or_else(|error| panic!("decode consumer response: {error}"));
        assert_eq!(decoded.body, expected);
        assert_eq!(decoded.headers["cache-control"], "no-store");
        assert_eq!(decoded.headers["content-type"], "application/json");
        assert_eq!(decoded.headers["x-layerx-genesis-sha256"], digest);
        assert_eq!(
            decoded.headers["content-length"],
            expected.len().to_string()
        );
    } else {
        assert_eq!(response.status, 502);
        assert!(response.genesis_sha256.is_none());
        let error: Value = serde_json::from_slice(&response.body)
            .unwrap_or_else(|error| panic!("decode refusal: {error:?}"));
        assert_eq!(error["error"]["code"], "comet_response_invalid");
    }
}

pub fn byte_identity() {
    let fixture = std::str::from_utf8(FIXTURE)
        .unwrap_or_else(|error| panic!("genesis UTF-8: {error:?}"))
        .trim()
        .as_bytes();
    for wrapped in [false, true] {
        for chunked in [false, true] {
            let mut reply = Reply::new("/genesis", direct(fixture, wrapped));
            reply.chunked = chunked;
            check_transcript(vec![reply], Some(fixture));
        }
    }
}

pub fn malformed_responses() {
    for body in [
        b"bad json".as_slice(), b"[]", b"{}", b"{\"genesis\":null}",
        b"{\"genesis\":{\"chain_id\":\"\"}}",
        b"{\"jsonrpc\":\"2.0\",\"id\":-1,\"result\":{\"code\":-32603,\"message\":\"Internal error\",\"data\":\"genesis response is large, please use the genesis_chunked API instead\"}}",
        b"{\"code\":-32600,\"message\":\"Internal error\",\"data\":\"genesis response is large, please use the genesis_chunked API instead\"}",
        b"{\"jsonrpc\":\"2.0\",\"id\":-1,\"error\":{\"genesis\":{\"chain_id\":\"pacific-1\"}}}",
        b"{\"jsonrpc\":\"2.0\",\"id\":-1,\"result\":{\"genesis\":{\"chain_id\":\"pacific-1\"}}",
        b"{\"jsonrpc\":\"1.0\",\"result\":{}}",
        b"{\"result\":{}}",
        b"{\"code\":-32603,\"message\":\"Internal error\",\"data\":\"unrelated\"}",
    ] {
        check_response(body.to_vec(), None);
    }
    let mut reply = Reply::new("/genesis", chunked_error(false));
    reply.status = 500;
    check_transcript(vec![reply], None);
}

pub fn size_bound() {
    const LIMIT: usize = 32 * 1024 * 1024;
    let fixture = std::str::from_utf8(FIXTURE)
        .unwrap_or_else(|error| panic!("genesis UTF-8: {error:?}"))
        .trim()
        .as_bytes();
    for size in [LIMIT, LIMIT + 1] {
        let mut genesis = fixture[..fixture.len() - 1].to_vec();
        genesis.resize(size - 1, b' ');
        genesis.push(b'}');
        check_response(
            direct(&genesis, false),
            (size == LIMIT).then_some(genesis.as_slice()),
        );
    }
}

pub fn unreachable_and_endpoint_refusals() {
    let reserved = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("reserve unreachable endpoint: {error:?}"));
    let address = reserved
        .local_addr()
        .unwrap_or_else(|error| panic!("reserved address: {error:?}"));
    drop(reserved);
    let node = genesis::endpoint(&format!("http://{address}"))
        .unwrap_or_else(|error| panic!("valid node endpoint: {error}"));
    let response = genesis::response(&node, Instant::now() + NODE_IO_TIMEOUT);
    assert_eq!(response.status, 503);
    assert!(response.genesis_sha256.is_none());
    let error: Value = serde_json::from_slice(&response.body)
        .unwrap_or_else(|error| panic!("decode unavailable: {error:?}"));
    assert_eq!(error["error"]["code"], "comet_unavailable");
    for endpoint in [
        "",
        "https://127.0.0.1:26657",
        "http://192.0.2.1:26657",
        "http://localhost:26657",
        "http://127.0.0.1:0",
        "http://127.0.0.1:26657/path",
        "http://user@127.0.0.1:26657",
        "http://127.0.0.1:26657?x",
        "http://127.0.0.1:26657/\r\n",
    ] {
        assert!(
            genesis::endpoint(endpoint).is_err(),
            "invalid endpoint admitted: {endpoint:?}"
        );
    }
}

fn chunked_error(wrapped: bool) -> Vec<u8> {
    let error = serde_json::json!({"code": -32603, "message": "Internal error", "data": "genesis response is large, please use the genesis_chunked API instead"});
    if wrapped {
        serde_json::json!({"jsonrpc":"2.0", "id":-1, "error":error})
    } else {
        error
    }
    .to_string()
    .into_bytes()
}

fn encoded_chunk(data: &[u8], index: usize, total: usize, wrapped: bool) -> Vec<u8> {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut encoded = String::new();
    for group in data.chunks(3) {
        let a = group[0];
        let b = group.get(1).copied().unwrap_or(0);
        let c = group.get(2).copied().unwrap_or(0);
        encoded.push(char::from(ALPHABET[usize::from(a >> 2)]));
        encoded.push(char::from(ALPHABET[usize::from(((a & 3) << 4) | (b >> 4))]));
        encoded.push(if group.len() > 1 {
            char::from(ALPHABET[usize::from(((b & 15) << 2) | (c >> 6))])
        } else {
            '='
        });
        encoded.push(if group.len() > 2 {
            char::from(ALPHABET[usize::from(c & 63)])
        } else {
            '='
        });
    }
    let result =
        serde_json::json!({"chunk":index.to_string(), "total":total.to_string(), "data":encoded});
    if wrapped {
        serde_json::json!({"jsonrpc":"2.0", "id":-1, "result":result})
    } else {
        result
    }
    .to_string()
    .into_bytes()
}

pub fn ordered_chunks() {
    for wrapped in [false, true] {
        let mut genesis = b" \n".to_vec();
        genesis.extend_from_slice(
            std::str::from_utf8(FIXTURE)
                .unwrap_or_else(|error| panic!("genesis UTF-8: {error:?}"))
                .trim()
                .as_bytes(),
        );
        genesis.extend_from_slice(b"\n ");
        let parts = [&genesis[..17], &genesis[17..101], &genesis[101..]];
        let mut replies = vec![Reply::new("/genesis", chunked_error(wrapped))];
        for (index, part) in parts.iter().enumerate() {
            replies.push(Reply::new(
                &format!("/genesis_chunked?chunk={index}"),
                encoded_chunk(part, index, parts.len(), wrapped),
            ));
        }
        check_transcript(replies, Some(&genesis));
    }
}

pub fn invalid_chunk_metadata() {
    let good: Value = serde_json::from_slice(&encoded_chunk(FIXTURE, 0, 1, false))
        .unwrap_or_else(|error| panic!("chunk fixture: {error:?}"));
    for update in [
        serde_json::json!({"total":"0"}),
        serde_json::json!({"total":"33"}),
        serde_json::json!({"total":"-1"}),
        serde_json::json!({"total":1}),
        serde_json::json!({"chunk":"1"}),
        serde_json::json!({"chunk":"-1"}),
        serde_json::json!({"chunk":0}),
        serde_json::json!({"data":""}),
        serde_json::json!({"data":"!!!!"}),
        serde_json::json!({"data":"A==="}),
        serde_json::json!({"data":"AB=="}),
        serde_json::json!({"data":"AAB="}),
        serde_json::json!({"data":"AA==AAAA"}),
        serde_json::json!({"data":"AAA"}),
        serde_json::json!({"data":"AA=A"}),
    ] {
        let mut value = good.clone();
        for (key, replacement) in update
            .as_object()
            .unwrap_or_else(|| panic!("metadata update"))
        {
            value[key] = replacement.clone();
        }
        check_transcript(
            vec![
                Reply::new("/genesis", chunked_error(false)),
                Reply::new("/genesis_chunked?chunk=0", value.to_string().into_bytes()),
            ],
            None,
        );
    }
    for second in [
        encoded_chunk(&FIXTURE[10..], 1, 3, false),
        encoded_chunk(&FIXTURE[10..], 0, 2, false),
        b"{}".to_vec(),
        serde_json::json!({"jsonrpc":"2.0","id":-1,"error": serde_json::from_slice::<Value>(&encoded_chunk(&FIXTURE[10..],1,2,false)).unwrap_or_else(|error| panic!("chunk: {error:?}"))}).to_string().into_bytes(),
        encoded_chunk(b"not-json", 1, 2, false),
    ] {
        check_transcript(vec![Reply::new("/genesis", chunked_error(false)), Reply::new("/genesis_chunked?chunk=0", encoded_chunk(&FIXTURE[..10],0,2,false)), Reply::new("/genesis_chunked?chunk=1", second)], None);
    }
}

pub fn truncation_and_chunk_bounds() {
    const LIMIT: usize = 32 * 1024 * 1024;
    let mut reply = Reply::new("/genesis", Vec::new());
    reply.length = Some(LIMIT + 64 * 1024 + 1);
    check_transcript(vec![reply], None);
    let mut reply = Reply::new("/genesis", direct(FIXTURE, false));
    reply.length = Some(reply.body.len() + 1);
    check_transcript(vec![reply], None);
    let mut genesis = FIXTURE.to_vec();
    genesis.resize(LIMIT, b' ');
    for extra in [false, true] {
        let mut last = genesis[LIMIT / 2..].to_vec();
        if extra {
            last.push(b' ');
        }
        check_transcript(
            vec![
                Reply::new("/genesis", chunked_error(false)),
                Reply::new(
                    "/genesis_chunked?chunk=0",
                    encoded_chunk(&genesis[..LIMIT / 2], 0, 2, false),
                ),
                Reply::new(
                    "/genesis_chunked?chunk=1",
                    encoded_chunk(&last, 1, 2, false),
                ),
            ],
            (!extra).then_some(genesis.as_slice()),
        );
    }
}

pub fn deadline() {
    let mut reply = Reply::new("/genesis", direct(FIXTURE, false));
    reply.delay = Duration::from_secs(31);
    let (node, worker) = serve(vec![reply]);
    let started = Instant::now();
    let response = genesis::response(&node, Instant::now() + NODE_IO_TIMEOUT);
    assert_eq!(response.status, 502);
    assert!(response.genesis_sha256.is_none());
    assert!(started.elapsed() < Duration::from_secs(32));
    worker
        .join()
        .unwrap_or_else(|error| panic!("deadline peer stopped: {error:?}"));
}

pub fn route_refusals() {
    let reserved = TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("reserve disconnected transport: {error:?}"));
    let address = reserved
        .local_addr()
        .unwrap_or_else(|error| panic!("reserved address: {error:?}"));
    let node = NodeEndpoint {
        address,
        path: "/".to_owned(),
    };
    let tls =
        ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_safe_default_protocol_versions()
            .unwrap_or_else(|error| panic!("TLS versions: {error:?}"))
            .with_no_client_auth()
            .with_cert_resolver(Arc::new(rustls::server::ResolvesServerCertUsingSni::new()));
    let mut config = Config {
        listen: address,
        tls: Arc::new(tls),
        node: node.clone(),
        chain_id: 9125,
        comet: Some(node),
    };
    for (path, method) in [
        ("/genesis", "POST"),
        ("/genesis/", "GET"),
        ("/genesis_chunked", "GET"),
    ] {
        let request = Request {
            method: method.to_owned(),
            path: path.to_owned(),
            headers: BTreeMap::new(),
            body: Vec::new(),
        };
        assert_eq!(
            route(&config, &request, Instant::now() + CLIENT_IO_TIMEOUT).status,
            404
        );
    }
    let wire = b"GET /genesis?chunk=0 HTTP/1.1\r\nHost: localhost\r\nContent-Length: 0\r\n\r\n";
    assert!(parse_client_request(&mut wire.as_slice()).is_err());
    for method in [
        "eth_sendTransaction",
        "eth_sign",
        "admin_nodeInfo",
        "genesis",
        "genesis_chunked",
    ] {
        let request = Request {
            method: "POST".to_owned(),
            path: "/".to_owned(),
            headers: BTreeMap::from([("content-type".to_owned(), "application/json".to_owned())]),
            body: serde_json::json!({"jsonrpc":"2.0","id":1,"method":method,"params":[]})
                .to_string()
                .into_bytes(),
        };
        let response = route(&config, &request, Instant::now() + CLIENT_IO_TIMEOUT);
        assert_eq!(response.status, 200);
        let error: Value = serde_json::from_slice(&response.body)
            .unwrap_or_else(|error| panic!("RPC refusal: {error:?}"));
        assert_eq!(error["error"]["code"], -32601);
    }
    config.comet = None;
    let request = Request {
        method: "GET".to_owned(),
        path: "/genesis".to_owned(),
        headers: BTreeMap::new(),
        body: Vec::new(),
    };
    let response = route(&config, &request, Instant::now() + CLIENT_IO_TIMEOUT);
    assert_eq!(response.status, 404);
    assert!(response.genesis_sha256.is_none());
    assert_eq!(
        serde_json::from_slice::<Value>(&response.body)
            .unwrap_or_else(|error| panic!("missing configuration refusal: {error:?}")),
        serde_json::json!({"error":{"code":"not_found","retry":"never"}})
    );
}

pub fn native_profile_consumer() {
    let profile: Value = serde_json::from_slice(include_bytes!("../../deployment-profile.json"))
        .unwrap_or_else(|error| panic!("committed native deployment profile: {error:?}"));
    assert_eq!(profile["schema"], "paxeer/native-deployment/1");
    assert_eq!(profile["node"]["chain_id"], "hyperpax_125-1");
    assert_eq!(profile["node"]["evm_chain_id"], 125);
    let origin = profile["node"]["rpc"]
        .as_str()
        .unwrap_or_else(|| panic!("native RPC origin from the production deployment profile"));
    let node = NodeEndpoint::parse(origin).unwrap_or_else(|error| {
        panic!("maintained native profile cannot enter the restored transport: {error}")
    });
    assert_eq!(node.path, "/");
}

include!("../../src/main.rs");
