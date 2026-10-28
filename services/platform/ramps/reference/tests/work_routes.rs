use std::fs;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde_json::{json, Value};

struct Workspace(PathBuf);

impl Workspace {
    fn new() -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock")
            .as_nanos();
        let workspace = Self(
            std::env::temp_dir().join(format!("layerx-ramp-work-{}-{nanos}", std::process::id())),
        );
        fs::create_dir(&workspace.0).expect("create workspace");
        fs::set_permissions(&workspace.0, fs::Permissions::from_mode(0o700))
            .expect("private workspace");
        for args in [
            vec![
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-days",
                "2",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost,IP:127.0.0.1",
                "-keyout",
                "key.pem",
                "-out",
                "cert.pem",
            ],
            vec![
                "pkcs12",
                "-export",
                "-inkey",
                "key.pem",
                "-in",
                "cert.pem",
                "-passout",
                "pass:ramp-test-password",
                "-out",
                "identity.p12",
            ],
            vec![
                "x509",
                "-in",
                "cert.pem",
                "-outform",
                "DER",
                "-out",
                "anchor.der",
            ],
        ] {
            let output = Command::new("openssl")
                .args(args)
                .current_dir(&workspace.0)
                .output()
                .expect("generate client TLS material");
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
        }
        fs::set_permissions(
            workspace.path("identity.p12"),
            fs::Permissions::from_mode(0o600),
        )
        .expect("private TLS identity");
        workspace.private_file("password", b"ramp-test-password\n");
        workspace.private_file("service-token", b"work-route-service-token\n");
        workspace.private_file("operator-token", b"work-route-operator-token\n");
        workspace
    }

    fn path(&self, name: &str) -> PathBuf {
        self.0.join(name)
    }

    fn private_file(&self, name: &str, bytes: &[u8]) -> PathBuf {
        let path = self.path(name);
        fs::write(&path, bytes).expect("write protected file");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600)).expect("protect file");
        path
    }

    fn config(&self, address: SocketAddr) -> PathBuf {
        let p = |name: &str| self.path(name);
        let did = "did:layerx:ramp-work-test";
        let account = format!("agent:{did}:main");
        let config = json!({
            "listen": address.to_string(),
            "listener": "plain",
            "journal_path": p("journal.jsonl"),
            "worker_id": "ramp-work-worker",
            "lease_seconds": 60,
            "reconcile_seconds": 5,
            "operator": {
                "principal_id": "ramp-work-operator",
                "account": account,
                "signer_key_handle": "kms://ramp-work/layerx"
            },
            "quotes": [{
                "quote_id": "work-quote",
                "direction": "on_ramp",
                "layerx_asset": ([1_u8; 32]),
                "layerx_amount": 1_000_000,
                "external_currency": "EUR",
                "external_amount_minor": 100,
                "rate_numerator": 1_000_000,
                "rate_denominator": 100,
                "fee_minor": 2,
                "maximum_slippage_bps": 25,
                "context": ([8_u8; 32]),
                "provider_token": "provider-product",
                "payout_token": "beneficiary-token",
                "expires_at": 4_000_000_000_u64
            }],
            "client_tls": {
                "ca_pem": p("cert.pem"),
                "identity_pkcs12": p("identity.p12"),
                "identity_password_file": p("password"),
                "timeout_seconds": 8
            },
            "identity": {
                "endpoint": "https://identity.example.test",
                "service_token_file": p("service-token"),
                "audience": "layerx-ramp"
            },
            "compliance": {
                "endpoint": "https://compliance.example.test",
                "service_token_file": p("service-token"),
                "public_key": "22".repeat(32)
            },
            "provider": {
                "endpoint": "https://provider.example.test",
                "credential_file": p("service-token"),
                "settlement_path": "/layerx-ramp-v1/settlements",
                "status_path": "/layerx-ramp-v1/settlements"
            },
            "layerx": {
                "rpc_endpoint": "https://rpc.example.test:8545",
                "rpc_trust_anchor_der": p("anchor.der"),
                "chain_id": 125,
                "operator_address": format!("0x{}", "12".repeat(20)),
                "receipt_keys": [{
                    "keyId": "01".repeat(32),
                    "publicKey": "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
                    "firstHeight": 1,
                    "lastHeight": 10
                }],
                "max_fee_per_gas": 1000,
                "max_priority_fee_per_gas": 2,
                "gas_limit_cap": 100000,
                "signer_endpoint": "https://signer.example.test",
                "signer_token_file": p("service-token"),
                "actor_did": did
            },
            "paxeer": {
                "custody_endpoint": "https://custody.example.test",
                "custody_credential_file": p("service-token"),
                "broadcast_path": "/layerx-paxeer-v1/rebalances",
                "status_path": "/layerx-paxeer-v1/rebalances",
                "operator_account": account,
                "wallet_address": "operator-wallet",
                "vault_id": "operator-vault",
                "signer_key_handle": "kms://ramp-work/paxeer",
                "rpc_endpoints": ["https://rpc-a.example.test:8545", "https://rpc-b.example.test:8545"],
                "rpc_trust_anchor_der": p("anchor.der"),
                "rpc_chain_id": 229,
                "rpc_minimum_agreement": 2,
                "required_confirmations": 12,
                "poll_cadence_seconds": 5,
                "delayed_after_polls": 12
            },
            "provider_callback_public_key": "44".repeat(32),
            "operator_control_token_file": p("operator-token")
        });
        self.private_file(
            "config.json",
            &serde_json::to_vec(&config).expect("encode config"),
        )
    }
}

impl Drop for Workspace {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct Ramp(Child);

impl Ramp {
    fn start(config: &Path, address: SocketAddr) -> Self {
        let mut ramp = Self(
            Command::new(env!("CARGO_BIN_EXE_layerx-reference-ramp"))
                .arg(config)
                .stdout(Stdio::null())
                .stderr(Stdio::piped())
                .spawn()
                .expect("start ramp"),
        );
        let deadline = Instant::now() + Duration::from_secs(30);
        loop {
            if let Some(status) = ramp.0.try_wait().expect("poll ramp") {
                let mut stderr = String::new();
                ramp.0
                    .stderr
                    .take()
                    .expect("ramp stderr")
                    .read_to_string(&mut stderr)
                    .expect("read startup error");
                panic!("ramp exited before listening: {status}: {stderr}");
            }
            if let Ok(stream) = TcpStream::connect_timeout(&address, Duration::from_millis(100)) {
                let (status, body) = request(stream, "GET", "/livez", &[]);
                if status == 200 && body["live"] == true {
                    return ramp;
                }
            }
            assert!(Instant::now() < deadline, "ramp did not begin listening");
            thread::sleep(Duration::from_millis(50));
        }
    }
}

impl Drop for Ramp {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn request(mut stream: TcpStream, method: &str, path: &str, body: &[u8]) -> (u16, Value) {
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("read timeout");
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .expect("write timeout");
    write!(stream,
        "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer work-route-operator-token\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()).expect("write headers");
    stream.write_all(body).expect("write request body");
    let mut response = String::new();
    stream
        .read_to_string(&mut response)
        .expect("read HTTP response");
    let (headers, body) = response
        .split_once("\r\n\r\n")
        .expect("HTTP response boundary");
    let status = headers
        .split_ascii_whitespace()
        .nth(1)
        .expect("HTTP status")
        .parse()
        .expect("numeric HTTP status");
    (status, serde_json::from_str(body).expect("JSON response"))
}

#[test]
fn verify_receipt_mode_refuses_activity_hex() {
    let missing =
        std::env::temp_dir().join(format!("ramp-verifier-missing-{}", std::process::id()));
    for hash in [
        "ab".repeat(32),
        "0x1234".to_owned(),
        format!("0x{}gg", "ab".repeat(31)),
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_layerx-reference-ramp"))
            .arg("--verify-receipt")
            .arg(&missing)
            .arg(&missing)
            .arg(hash)
            .output()
            .expect("run receipt verifier");
        assert_eq!(output.status.code(), Some(1));
        assert!(output.stdout.is_empty());
        let stderr = String::from_utf8(output.stderr).expect("verifier diagnostic");
        assert!(
            stderr.contains("usage: --verify-receipt CONFIG.json ORDER.json TRANSACTION_HASH"),
            "{stderr}"
        );
        assert!(
            stderr.contains("0x-prefixed 32-byte transaction hash"),
            "{stderr}"
        );
    }
    let output = Command::new(env!("CARGO_BIN_EXE_layerx-reference-ramp"))
        .arg("--verify-receipt")
        .arg(&missing)
        .arg(&missing)
        .arg(format!("0x{}", "ab".repeat(32)))
        .output()
        .expect("run verifier with transaction hash");
    assert_eq!(output.status.code(), Some(1));
    assert!(
        String::from_utf8_lossy(&output.stderr).contains("protected verifier input unavailable")
    );
}

#[test]
fn work_route_refuses_native_payload_field() {
    let workspace = Workspace::new();
    let reservation = TcpListener::bind("127.0.0.1:0").expect("reserve ramp address");
    let address = reservation.local_addr().expect("ramp address");
    let config = workspace.config(address);
    drop(reservation);
    let _ramp = Ramp::start(&config, address);
    for action in ["submit_layerx", "compliance"] {
        for payload in [json!([1, 2, 3]), json!([]), Value::Null] {
            let mut body = json!({
                "order_digest": ([7_u8; 32]),
                "action": action,
                "canonical_receive_payload": payload
            });
            if action == "submit_layerx" {
                body["account_sequence"] = json!(1);
            }
            let (status, response) = request(
                TcpStream::connect(address).expect("connect to work route"),
                "POST",
                "/internal/v1/work",
                &serde_json::to_vec(&body).expect("encode work"),
            );
            assert_eq!(status, 400, "{response}");
            assert_eq!(response, json!({"error": "work_invalid"}));
        }
    }
    let body = json!({"order_digest": ([7_u8; 32]), "action": "submit_layerx"});
    let (status, response) = request(
        TcpStream::connect(address).expect("connect without retired field"),
        "POST",
        "/internal/v1/work",
        &serde_json::to_vec(&body).expect("encode current work request"),
    );
    assert_eq!(status, 503, "{response}");
    assert_eq!(response, json!({"error": "journal_recovery_required"}));
}
