use std::collections::BTreeMap;
use std::io::{Read as _, Write as _};
use std::net::{Shutdown, SocketAddr, TcpListener, TcpStream};
use std::sync::{Arc, RwLock};
use std::time::Duration;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::attestation::{
    to_attestor_signature, uint256_from_u64, InboundAttestation, OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{
    serve, AttestationView, AuthenticatedPolicy, PolicyDestination,
};
use layerx_bridge_relayer::config::RelayerConfig;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{Entry, Journal, Observation, Position};
use serde_json::{json, Value};

fn server(view: Arc<RwLock<AttestationView>>) -> SocketAddr {
    server_with_origins(view, Vec::new())
}

fn server_with_origins(view: Arc<RwLock<AttestationView>>, origins: Vec<String>) -> SocketAddr {
    let listener = TcpListener::bind("127.0.0.1:0").or_panic("bind attestation server");
    let address = listener.local_addr().or_panic("server socket address");
    std::thread::spawn(move || {
        serve(&listener, &view, &origins).or_panic("serve attestations");
    });
    address
}

fn connect(address: SocketAddr) -> TcpStream {
    let stream = TcpStream::connect_timeout(&address, Duration::from_secs(5))
        .or_panic("connect to real attestation server");
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .or_panic("bound client reads");
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .or_panic("bound client writes");
    stream
}

fn response(stream: &mut TcpStream) -> (u16, Value) {
    let mut bytes = Vec::new();
    stream
        .read_to_end(&mut bytes)
        .or_panic("read HTTP response through close");
    let text = String::from_utf8(bytes).or_panic("UTF-8 HTTP response");
    let (headers, body) = text
        .split_once("\r\n\r\n")
        .or_panic("HTTP header terminator");
    let status = headers
        .lines()
        .next()
        .or_panic("status line")
        .split(' ')
        .nth(1)
        .or_panic("status code")
        .parse()
        .or_panic("numeric status code");
    assert!(headers
        .lines()
        .any(|line| line == "Content-Type: application/json"));
    assert!(headers.lines().any(|line| line == "Connection: close"));
    assert!(headers
        .lines()
        .any(|line| line == format!("Content-Length: {}", body.len())));
    (status, serde_json::from_str(body).or_panic("JSON response"))
}

fn request(address: SocketAddr, method: &str, path: &str) -> (u16, Value) {
    let mut stream = connect(address);
    write!(
        stream,
        "{method} {path} HTTP/1.1\r\nHost: localhost\r\n\r\n"
    )
    .or_panic("write HTTP request");
    response(&mut stream)
}

fn origin_request(
    address: SocketAddr,
    method: &str,
    path: &str,
    origin: Option<&str>,
) -> (u16, BTreeMap<String, String>, String) {
    let mut stream = connect(address);
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\n").or_panic("request line");
    if let Some(origin) = origin {
        write!(stream, "oRiGiN: {origin}\r\n").or_panic("origin header");
    }
    if method == "OPTIONS" {
        write!(
            stream,
            "Access-Control-Request-Method: GET\r\nAccess-Control-Request-Headers: accept\r\n"
        )
        .or_panic("preflight headers");
    }
    write!(stream, "\r\n").or_panic("end request headers");
    let mut text = String::new();
    stream
        .read_to_string(&mut text)
        .or_panic("read real HTTP response");
    let (head, body) = text.split_once("\r\n\r\n").or_panic("header terminator");
    let mut lines = head.split("\r\n");
    let status = lines
        .next()
        .or_panic("status line")
        .split(' ')
        .nth(1)
        .or_panic("status code")
        .parse()
        .or_panic("numeric status");
    let mut headers = BTreeMap::new();
    for line in lines {
        let (name, value) = line.split_once(':').or_panic("response header");
        assert!(
            headers
                .insert(name.to_ascii_lowercase(), value.trim().to_owned())
                .is_none(),
            "duplicate response header {name}"
        );
    }
    assert_eq!(headers.get("connection").map(String::as_str), Some("close"));
    if status != 204 {
        assert_eq!(headers.get("content-length"), Some(&body.len().to_string()));
    }
    (status, headers, body.to_owned())
}

fn origin_config() -> RelayerConfig {
    let mut value: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../services/bridge/deploy/relayer-solana.example.json"
    )))
    .or_panic("documented relayer configuration");
    value["paxeer"]["receipt_keys"] = json!([{
        "sequencer_id": hex::prefixed(&[1; 32]),
        "public_key": hex::prefixed(&[2; 32]),
        "first_height": 0,
        "last_height": u64::MAX,
    }]);
    serde_json::from_value(value).or_panic("parse relayer configuration")
}

#[test]
fn a_configured_origin_receives_the_allow_origin_header() {
    let (config, allowed) = configured_allowed_origin();

    let deposit = InboundAttestation {
        chain_id: 1,
        vault: [1; 20],
        tx_hash: [2; 32],
        log_index: 0,
        recipient: [3; 32],
        asset: [4; 20],
        amount: uint256_from_u64(123),
    };
    let release = OutboundAttestation {
        chain_id: 1,
        vault: deposit.vault,
        paxeer_tx_hash: [5; 32],
        paxeer_nonce: 0,
        recipient: [3; 20],
        asset: deposit.asset,
        amount: uint256_from_u64(456),
    };
    let mut unique = [0; 16];
    openssl::rand::rand_bytes(&mut unique).or_panic("unique journal directory");
    let directory = std::env::temp_dir().join(format!("attestation-cors-{}", hex::encode(&unique)));
    std::fs::create_dir(&directory).or_panic("create journal directory");
    let mut journal = Journal::open(&directory.join("journal.jsonl")).or_panic("open real journal");
    let position = Position {
        block_number: 100,
        block_hash: [6; 32],
    };
    for observation in [
        Observation::inbound(&deposit, position),
        Observation::outbound(&release, position),
    ] {
        journal
            .append(&Entry::Observed {
                item: observation.key(),
                observation,
            })
            .or_panic("record real observation");
    }
    let view = Arc::new(RwLock::new(AttestationView {
        items: journal.state().items.clone(),
        ..AttestationView::default()
    }));
    let address = server_with_origins(Arc::clone(&view), config.attestation_allowed_origins);
    let unconfigured = server(view);
    assert_origin_routes(address, unconfigured, &deposit, &release, allowed);
    drop(journal);
    std::fs::remove_dir_all(directory).or_panic("remove test journal");
}

fn configured_allowed_origin() -> (RelayerConfig, &'static str) {
    let mut config = origin_config();
    assert!(config.attestation_allowed_origins.is_empty());
    assert_eq!(config.validate(), Ok(()));
    for invalid in [
        "",
        "*",
        "null",
        "bridge.example",
        "https://",
        "://bridge.example",
        "1https://bridge.example",
        "https://bridge.example/",
        "https://bridge.example/path",
        "https://bridge.example?query",
        "https://bridge.example#fragment",
        "https://user@bridge.example",
        "https://bridge.example:",
        "https://bridge.example:65536",
        "https://bridge.example:abc",
        "https://bridge.example:443:80",
        "https://bad host",
        "https://bad_host",
        "https://[invalid]",
        "https://::1",
        "https://[::1]suffix",
        "https://bridge.example\r\nX-Injected: yes",
    ] {
        config.attestation_allowed_origins =
            vec!["https://bridge.example".to_owned(), invalid.to_owned()];
        let error = config
            .validate()
            .err()
            .or_panic("invalid configured origin is refused");
        assert!(
            error.to_string().contains("attestation_allowed_origins"),
            "{invalid}: {error}"
        );
    }
    for valid in [
        "https://bridge.example",
        "http://localhost:5173",
        "http://127.0.0.1:8080",
        "https://[::1]:8443",
        "https://[::1]",
    ] {
        config.attestation_allowed_origins = vec![valid.to_owned()];
        assert_eq!(config.validate(), Ok(()), "{valid}");
    }
    let allowed = "https://bridge.example:8443";
    config.attestation_allowed_origins = vec![allowed.to_owned()];
    assert_eq!(config.validate(), Ok(()));

    (config, allowed)
}

fn assert_origin_routes(
    address: SocketAddr,
    unconfigured: SocketAddr,
    deposit: &InboundAttestation,
    release: &OutboundAttestation,
    allowed: &str,
) {
    for path in [
        format!("/v1/deposit/1/{}/0", hex::prefixed(&deposit.tx_hash)),
        format!("/v1/release/1/{}/0", hex::prefixed(&release.paxeer_tx_hash)),
    ] {
        let (status, headers, body) = origin_request(address, "GET", &path, Some(allowed));
        assert_eq!(status, 200);
        assert_eq!(
            headers
                .get("access-control-allow-origin")
                .map(String::as_str),
            Some(allowed)
        );
        assert_eq!(headers.get("vary").map(String::as_str), Some("Origin"));
        let expected: Value = serde_json::from_str(&body).or_panic("attestation JSON");
        assert_eq!(expected["status"], "observed");
        assert_eq!(expected["chainId"], "1");
        for origin in [
            None,
            Some("https://other.example"),
            Some("http://bridge.example:8443"),
            Some("https://bridge.example"),
            Some("https://bridge.example:8443.evil"),
            Some("https://BRIDGE.example:8443"),
            Some("https://bridge.example:8443/"),
            Some("https://bridge.example:8443 https://other.example"),
        ] {
            let (status, headers, body) = origin_request(address, "GET", &path, origin);
            assert_eq!(status, 200);
            assert!(
                !headers.contains_key("access-control-allow-origin"),
                "{origin:?}"
            );
            assert_eq!(headers.get("vary").map(String::as_str), Some("Origin"));
            assert_eq!(
                serde_json::from_str::<Value>(&body).or_panic("attestation JSON"),
                expected
            );
        }
        let (status, headers, _) = origin_request(unconfigured, "GET", &path, Some(allowed));
        assert_eq!(status, 200);
        assert!(!headers.contains_key("access-control-allow-origin"));
    }
    let (status, headers, _) = origin_request(
        address,
        "GET",
        &format!("/v1/deposit/2/{}/0", hex::prefixed(&deposit.tx_hash)),
        Some(allowed),
    );
    assert_eq!(status, 404);
    assert_eq!(
        headers
            .get("access-control-allow-origin")
            .map(String::as_str),
        Some(allowed)
    );
}

#[test]
fn a_preflight_from_a_configured_origin_is_answered() {
    let allowed = "https://bridge.example";
    let address = server_with_origins(
        Arc::new(RwLock::new(AttestationView::default())),
        vec![allowed.to_owned()],
    );
    for route in ["deposit", "release"] {
        let path = format!("/v1/{route}/1/{}/0", hex::prefixed(&[2; 32]));
        let (status, headers, body) = origin_request(address, "OPTIONS", &path, Some(allowed));
        assert_eq!(status, 204);
        assert!(body.is_empty());
        assert_eq!(
            headers
                .get("access-control-allow-origin")
                .map(String::as_str),
            Some(allowed)
        );
        assert_eq!(headers.get("vary").map(String::as_str), Some("Origin"));
        assert_eq!(
            headers
                .get("access-control-allow-methods")
                .map(String::as_str),
            Some("GET")
        );
        assert_eq!(
            headers
                .get("access-control-allow-headers")
                .map(String::as_str),
            Some("accept")
        );
        for origin in [
            "https://other.example",
            "http://bridge.example",
            "https://bridge.example:443",
            "null",
        ] {
            let (status, headers, body) = origin_request(address, "OPTIONS", &path, Some(origin));
            assert_eq!(status, 403, "{origin}");
            assert!(!headers.contains_key("access-control-allow-origin"));
            assert!(!headers.contains_key("access-control-allow-methods"));
            assert!(!headers.contains_key("access-control-allow-headers"));
            assert_eq!(
                serde_json::from_str::<Value>(&body).or_panic("refusal JSON")["error"],
                "origin not allowed"
            );
        }
        assert_eq!(origin_request(address, "OPTIONS", &path, None).0, 405);
    }
    assert_eq!(
        origin_request(address, "OPTIONS", "/unknown", Some(allowed)).0,
        404
    );
}

#[test]
fn the_attestation_routes_answer_over_a_socket() {
    let shared = Arc::new(RwLock::new(AttestationView::default()));
    let address = server(Arc::clone(&shared));
    let deposit = InboundAttestation {
        chain_id: u64::MAX,
        vault: [0xcd; 20],
        tx_hash: [0xef; 32],
        log_index: u64::MAX,
        recipient: [0xab; 32],
        asset: [0xbc; 20],
        amount: uint256_from_u64(123),
    };
    let release = OutboundAttestation {
        chain_id: deposit.chain_id,
        vault: deposit.vault,
        paxeer_tx_hash: [0xde; 32],
        paxeer_nonce: 0,
        recipient: [0xab; 20],
        asset: deposit.asset,
        amount: uint256_from_u64(456),
    };
    let deposit_path = format!(
        "/v1/deposit/{}/{}/{}",
        deposit.chain_id,
        hex::prefixed(&deposit.tx_hash),
        deposit.log_index
    );
    let release_path = format!(
        "/v1/release/{}/{}/{}",
        release.chain_id,
        hex::prefixed(&release.paxeer_tx_hash),
        release.paxeer_nonce
    );
    for path in [&deposit_path, &release_path] {
        assert_eq!(request(address, "GET", path).0, 404);
    }

    let (mut journal, directory, position) =
        observed_attestation_journal(&shared, &deposit, &release);
    let mut expected_deposit = json!({
        "chainId": deposit.chain_id.to_string(),
        "vault": hex::prefixed(&deposit.vault),
        "txHash": hex::prefixed(&deposit.tx_hash),
        "logIndex": deposit.log_index.to_string(),
        "recipient": hex::prefixed(&deposit.recipient),
        "asset": hex::prefixed(&deposit.asset),
        "amount": "123",
        "digest": hex::prefixed(&deposit.digest()),
        "signatures": [],
        "status": "observed",
    });
    let expected_release = json!({
        "chainId": release.chain_id.to_string(),
        "vault": hex::prefixed(&release.vault),
        "paxeerTxHash": hex::prefixed(&release.paxeer_tx_hash),
        "paxeerNonce": "0",
        "recipient": hex::prefixed(&release.recipient),
        "asset": hex::prefixed(&release.asset),
        "amount": "456",
        "digest": hex::prefixed(&release.digest()),
        "signatures": [],
        "status": "observed",
    });
    assert_eq!(
        request(address, "GET", &deposit_path),
        (200, expected_deposit.clone())
    );
    assert_eq!(
        request(address, "GET", &release_path),
        (200, expected_release.clone())
    );

    let signature = publish_signed_deposit(&mut journal, &shared, &deposit, position);
    expected_deposit["signatures"] = json!([hex::prefixed(&signature)]);
    expected_deposit["status"] = json!("ready");
    assert_eq!(
        request(address, "GET", &deposit_path),
        (200, expected_deposit)
    );
    for route in ["deposit", "release"] {
        assert_eq!(
            request(
                address,
                "GET",
                &format!("/v1/{route}/0/{}/1", hex::prefixed(&[0; 32]))
            )
            .0,
            404
        );
    }

    let mut pipeline = connect(address);
    write!(pipeline, "GET {release_path} HTTP/1.1\r\nHost: localhost\r\n\r\nGET /unknown HTTP/1.1\r\nHost: localhost\r\n\r\n").or_panic("pipeline two requests");
    assert_eq!(response(&mut pipeline), (200, expected_release));
    drop(journal);
    std::fs::remove_dir_all(directory).or_panic("remove test journal");
}

fn observed_attestation_journal(
    shared: &Arc<RwLock<AttestationView>>,
    deposit: &InboundAttestation,
    release: &OutboundAttestation,
) -> (Journal, std::path::PathBuf, Position) {
    let mut unique = [0; 16];
    openssl::rand::rand_bytes(&mut unique).or_panic("unique journal directory");
    let directory =
        std::env::temp_dir().join(format!("attestation-server-{}", hex::encode(&unique)));
    std::fs::create_dir(&directory).or_panic("create journal directory");
    let mut journal = Journal::open(&directory.join("journal.jsonl")).or_panic("open real journal");
    let position = Position {
        block_number: 100,
        block_hash: [0x12; 32],
    };
    for observation in [
        Observation::inbound(deposit, position),
        Observation::outbound(release, position),
    ] {
        journal
            .append(&Entry::Observed {
                item: observation.key(),
                observation,
            })
            .or_panic("journal observation");
    }
    *shared.write().or_panic("replace shared view") = AttestationView {
        items: journal.state().items.clone(),
        ..AttestationView::default()
    };
    (journal, directory, position)
}

fn publish_signed_deposit(
    journal: &mut Journal,
    shared: &Arc<RwLock<AttestationView>>,
    deposit: &InboundAttestation,
    position: Position,
) -> [u8; 65] {
    let signing_key = SigningKey::from_bytes((&[1; 32]).into()).or_panic("attestor key");
    let (signature, recovery) = signing_key
        .sign_prehash_recoverable(&deposit.digest())
        .or_panic("sign attestation");
    let mut recoverable = [0; 65];
    recoverable[..64].copy_from_slice(&signature.to_bytes());
    recoverable[64] = recovery.to_byte();
    let signature = to_attestor_signature(recoverable).or_panic("attestor signature");
    journal
        .append(&Entry::Signed {
            item: Observation::inbound(deposit, position).key(),
            signature,
        })
        .or_panic("journal signature");
    let destination = PolicyDestination::Inbound {
        chain_id: 125,
        bridge: layerx_bridge_relayer::abi::LAYERX_BRIDGE_PRECOMPILE,
        source_chain_id: deposit.chain_id,
        source_vault: deposit.vault,
    };
    let policy = AuthenticatedPolicy {
        destination,
        threshold: 1,
        attestors: vec![layerx_bridge_relayer::attestation::ethereum_address(
            signing_key.verifying_key(),
        )],
    };
    let updated =
        AttestationView::from_journal(journal, None, &BTreeMap::from([(destination, policy)]));
    *shared.write().or_panic("publish next tick snapshot") = updated;
    signature
}

#[test]
fn malformed_attestation_paths_answer_bad_request() {
    let address = server(Arc::new(RwLock::new(AttestationView::default())));
    let hash = hex::prefixed(&[0xab; 32]);
    for route in ["deposit", "release"] {
        for number in [
            "",
            "-1",
            "+1",
            "1.0",
            "0x1",
            "abc",
            "18446744073709551616",
            "%31",
            "１",
        ] {
            for path in [
                format!("/v1/{route}/{number}/{hash}/0"),
                format!("/v1/{route}/0/{hash}/{number}"),
            ] {
                assert_eq!(request(address, "GET", &path).0, 400, "{path}");
            }
        }
        for invalid in [
            String::new(),
            "ab".repeat(32),
            format!("0X{}", "ab".repeat(32)),
            format!("0x{}", "AB".repeat(32)),
            format!("0x{}", "ab".repeat(31)),
            format!("0x{}", "ab".repeat(33)),
            format!("0x{}", "gg".repeat(32)),
        ] {
            let path = format!("/v1/{route}/0/{invalid}/0");
            assert_eq!(request(address, "GET", &path).0, 400, "{path}");
        }
        let path = format!("/v1/{route}/0/{hash}/0");
        assert_eq!(request(address, "GET", &path).0, 404);
        for method in ["POST", "PUT", "DELETE", "HEAD", "OPTIONS"] {
            assert_eq!(request(address, method, &path).0, 405);
        }
    }
    for path in [
        "/",
        "/v2/deposit/0/hash/0",
        "/v1/other/0/hash/0",
        "/v1/deposit",
        "/v1/release/0/hash/0/extra",
    ] {
        assert_eq!(request(address, "GET", path).0, 404, "{path}");
    }
    let mut oversized = connect(address);
    let mut large_header = b"GET / HTTP/1.1\r\nX-Large: ".to_vec();
    large_header.resize(8192, b'a');
    oversized
        .write_all(&large_header)
        .or_panic("send bounded oversized header");
    assert_eq!(response(&mut oversized).0, 413);
    let mut incomplete = connect(address);
    incomplete
        .write_all(b"GET / HTTP/1.1\r\n")
        .or_panic("send incomplete request");
    incomplete
        .shutdown(Shutdown::Write)
        .or_panic("end incomplete request");
    assert_eq!(response(&mut incomplete).0, 400);
    let mut idle = connect(address);
    assert_eq!(response(&mut idle).0, 408);
    assert_eq!(request(address, "GET", "/unknown").0, 404);
}

#[allow(dead_code)]
#[path = "support/repair_22_33_68.rs"]
mod repair_22_33_68_support;

#[test]
fn repair_22_33_68_policy_restart_and_rotation() {
    repair_22_33_68_support::policy_restart_and_rotation();
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
