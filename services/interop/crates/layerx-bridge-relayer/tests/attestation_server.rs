use std::io::{Read as _, Write as _};
use std::net::{Shutdown, SocketAddr, TcpListener, TcpStream};
use std::sync::{Arc, RwLock};
use std::time::Duration;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::attestation::{
    to_attestor_signature, uint256_from_u64, InboundAttestation, OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{serve, AttestationView};
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{Entry, Journal, Observation, Position};
use serde_json::{json, Value};

fn server(view: Arc<RwLock<AttestationView>>) -> SocketAddr {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind attestation server");
    let address = listener.local_addr().expect("server socket address");
    std::thread::spawn(move || serve(listener, view, Vec::new()).expect("serve attestations"));
    address
}

fn connect(address: SocketAddr) -> TcpStream {
    let stream = TcpStream::connect_timeout(&address, Duration::from_secs(5))
        .expect("connect to real attestation server");
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("bound client reads");
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .expect("bound client writes");
    stream
}

fn response(stream: &mut TcpStream) -> (u16, Value) {
    let mut bytes = Vec::new();
    stream
        .read_to_end(&mut bytes)
        .expect("read HTTP response through close");
    let text = String::from_utf8(bytes).expect("UTF-8 HTTP response");
    let (headers, body) = text.split_once("\r\n\r\n").expect("HTTP header terminator");
    let status = headers
        .lines()
        .next()
        .expect("status line")
        .split(' ')
        .nth(1)
        .expect("status code")
        .parse()
        .expect("numeric status code");
    assert!(headers
        .lines()
        .any(|line| line == "Content-Type: application/json"));
    assert!(headers.lines().any(|line| line == "Connection: close"));
    assert!(headers
        .lines()
        .any(|line| line == format!("Content-Length: {}", body.len())));
    (status, serde_json::from_str(body).expect("JSON response"))
}

fn request(address: SocketAddr, method: &str, path: &str) -> (u16, Value) {
    let mut stream = connect(address);
    write!(
        stream,
        "{method} {path} HTTP/1.1\r\nHost: localhost\r\n\r\n"
    )
    .expect("write HTTP request");
    response(&mut stream)
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

    let mut unique = [0; 16];
    openssl::rand::rand_bytes(&mut unique).expect("unique journal directory");
    let directory =
        std::env::temp_dir().join(format!("attestation-server-{}", hex::encode(&unique)));
    std::fs::create_dir(&directory).expect("create journal directory");
    let mut journal = Journal::open(&directory.join("journal.jsonl")).expect("open real journal");
    let position = Position {
        block_number: 100,
        block_hash: [0x12; 32],
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
            .expect("journal observation");
    }
    *shared.write().expect("replace shared view") = AttestationView {
        items: journal.state().items.clone(),
        ..AttestationView::default()
    };
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

    let signing_key = SigningKey::from_bytes((&[1; 32]).into()).expect("attestor key");
    let (signature, recovery) = signing_key
        .sign_prehash_recoverable(&deposit.digest())
        .expect("sign attestation");
    let mut recoverable = [0; 65];
    recoverable[..64].copy_from_slice(&signature.to_bytes());
    recoverable[64] = recovery.to_byte();
    let signature = to_attestor_signature(recoverable).expect("attestor signature");
    journal
        .append(&Entry::Signed {
            item: Observation::inbound(&deposit, position).key(),
            signature,
        })
        .expect("journal signature");
    let mut updated = AttestationView {
        items: journal.state().items.clone(),
        ..AttestationView::default()
    };
    updated.thresholds.insert(deposit.chain_id, 1);
    *shared.write().expect("publish next tick snapshot") = updated;
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
    write!(pipeline, "GET {release_path} HTTP/1.1\r\nHost: localhost\r\n\r\nGET /unknown HTTP/1.1\r\nHost: localhost\r\n\r\n").expect("pipeline two requests");
    assert_eq!(response(&mut pipeline), (200, expected_release));
    drop(journal);
    std::fs::remove_dir_all(directory).expect("remove test journal");
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
        .expect("send bounded oversized header");
    assert_eq!(response(&mut oversized).0, 413);
    let mut incomplete = connect(address);
    incomplete
        .write_all(b"GET / HTTP/1.1\r\n")
        .expect("send incomplete request");
    incomplete
        .shutdown(Shutdown::Write)
        .expect("end incomplete request");
    assert_eq!(response(&mut incomplete).0, 400);
    let mut idle = connect(address);
    assert_eq!(response(&mut idle).0, 408);
    assert_eq!(request(address, "GET", "/unknown").0, 404);
}
