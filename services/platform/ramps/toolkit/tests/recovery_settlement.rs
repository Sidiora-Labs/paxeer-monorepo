use std::fs;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_ramp_toolkit::clients::{
    verify_recovery_settlement, Endpoint, LayerxConfig, MutualTlsClient, MutualTlsFiles,
    PaxeerCustodyClient, PaxeerSubmission, ProviderClient, SecretFile,
};
use layerx_ramp_toolkit::journal::{Journal, Projection};
use layerx_ramp_toolkit::{
    AuthenticatedPrincipal, CreateOrder, OperatorIdentity, QuoteTerms, RampDirection, RampError,
    RampOrder,
};
use native_tls::{Identity, TlsAcceptor};
use serde_json::{json, Value};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

struct ClientFiles(PathBuf);

impl ClientFiles {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ramp-recovery-settlement-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).expect("create client credential directory");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
            .expect("protect client credential directory");
        let files = Self(path);
        files.openssl(&[
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-days",
            "1",
            "-subj",
            "/CN=localhost",
            "-addext",
            "subjectAltName=DNS:localhost",
            "-keyout",
            "key.pem",
            "-out",
            "ca.pem",
        ]);
        files.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"]);
        files.openssl(&[
            "pkcs12",
            "-export",
            "-inkey",
            "key.pem",
            "-in",
            "ca.pem",
            "-out",
            "identity.p12",
            "-passout",
            "pass:configuration-test",
        ]);
        fs::set_permissions(
            files.0.join("identity.p12"),
            fs::Permissions::from_mode(0o600),
        )
        .expect("protect client identity");
        for (name, value) in [
            ("password", b"configuration-test".as_slice()),
            ("token", b"configuration-test-token".as_slice()),
        ] {
            use std::io::Write as _;
            let mut file = fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(files.0.join(name))
                .expect("create private credential");
            file.write_all(value).expect("write private credential");
        }
        files
    }

    fn openssl(&self, args: &[&str]) {
        let output = Command::new("openssl")
            .args(args)
            .current_dir(&self.0)
            .output()
            .expect("run openssl to create real TLS credentials");
        assert!(
            output.status.success(),
            "openssl failed: {:?}",
            output.status
        );
    }

    fn http(&self) -> MutualTlsClient {
        MutualTlsClient::new(
            &MutualTlsFiles {
                ca_pem: self.0.join("ca.pem"),
                identity_pkcs12: SecretFile::new(self.0.join("identity.p12"))
                    .expect("private TLS identity"),
                identity_password: SecretFile::new(self.0.join("password"))
                    .expect("private identity password"),
            },
            Duration::from_secs(1),
        )
        .expect("construct mutual TLS client")
    }

    fn document(&self) -> Value {
        json!({
            "rpc_endpoint": "https://sequencer.example.test",
            "rpc_trust_anchor_der": self.0.join("ca.der"),
            "chain_id": 125,
            "operator_address": format!("0x{}", "12".repeat(20)),
            "receipt_keys": [{
                "keyId": format!("0x{}", "34".repeat(32)),
                "publicKey": "0xd75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
                "firstHeight": 10,
                "lastHeight": 20
            }],
            "max_fee_per_gas": 100,
            "max_priority_fee_per_gas": 2,
            "gas_limit_cap": 100000,
            "signer_endpoint": "https://signer.example.test",
            "signer_token_file": self.0.join("token"),
            "actor_did": "did:layerx:ramp"
        })
    }

    fn config(&self) -> LayerxConfig {
        serde_json::from_value(self.document()).expect("deserialize sequencer config")
    }
}

impl Drop for ClientFiles {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove client credentials");
    }
}

fn order_for(direction: RampDirection) -> RampOrder {
    RampOrder::bind(
        CreateOrder {
            order_id: "order-1".into(),
            quote_id: "quote-1".into(),
            payer_grant: (direction == RampDirection::OffRamp).then_some([9; 32]),
            payer_grant_purpose: (direction == RampDirection::OffRamp).then_some([6; 32]),
        },
        QuoteTerms {
            quote_id: "quote-1".into(),
            direction,
            layerx_asset: [1; 32],
            layerx_amount: 1_000,
            external_currency: "EUR".into(),
            external_amount_minor: 100,
            rate_numerator: 10,
            rate_denominator: 1,
            fee_minor: 2,
            maximum_slippage_bps: 25,
            context: [8; 32],
            provider_token: "product-eur".into(),
            payout_token: "beneficiary-123".into(),
            expires_at: 2_000,
        },
        AuthenticatedPrincipal {
            principal_id: "customer-1".into(),
            account: "agent:did:layerx:customer-1:main".into(),
        },
        OperatorIdentity {
            principal_id: "operator-1".into(),
            account: "agent:did:layerx:operator-1:main".into(),
            signer_key_handle: "kms.operator-1.primary".into(),
        },
        1_000,
    )
    .expect("bound order")
}

fn read_request(stream: &mut impl Read) -> (String, Value) {
    let mut bytes = Vec::new();
    let boundary = loop {
        let mut byte = [0];
        stream.read_exact(&mut byte).expect("request header");
        bytes.push(byte[0]);
        assert!(bytes.len() < 32_768, "bounded header");
        if bytes.ends_with(b"\r\n\r\n") {
            break bytes.len();
        }
    };
    let header = String::from_utf8(bytes).expect("HTTP header");
    let length = header
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length")
                .then(|| value.trim().parse::<usize>().expect("length"))
        })
        .expect("content length");
    assert!(length < 512 * 1024);
    let mut body = vec![0; length];
    stream.read_exact(&mut body).expect("request body");
    assert_eq!(boundary, header.len());
    (
        header,
        if body.is_empty() {
            Value::Null
        } else {
            serde_json::from_slice(&body).expect("request JSON")
        },
    )
}

fn respond(stream: &mut impl Write, body: &Value) {
    let bytes = serde_json::to_vec(body).expect("response JSON");
    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", bytes.len()).expect("response header");
    stream.write_all(&bytes).expect("response body");
    stream.flush().expect("flush response");
}

fn custody(files: &ClientFiles, endpoint: &str) -> PaxeerCustodyClient {
    PaxeerCustodyClient {
        http: files.http(),
        endpoint: Endpoint::parse(endpoint).expect("custody endpoint"),
        credential: "test-custody-credential".into(),
        broadcast_path: "/broadcast".into(),
        status_path: "/status".into(),
        operator_account: "operator".into(),
        wallet_address: "wallet".into(),
        vault_id: "vault".into(),
        signer_key_handle: "key".into(),
    }
}

#[test]
fn custody_hash_without_prefix_is_refused() {
    let files = ClientFiles::new();
    let listener = TcpListener::bind(("localhost", 0)).expect("custody listener");
    let endpoint = format!(
        "https://localhost:{}",
        listener.local_addr().expect("address").port()
    );
    let acceptor = TlsAcceptor::new(
        Identity::from_pkcs12(
            &fs::read(files.0.join("identity.p12")).expect("TLS identity"),
            "configuration-test",
        )
        .expect("custody identity"),
    )
    .expect("TLS custody server");
    let valid = format!("0x{}", "12".repeat(32));
    let hashes = vec![
        valid.clone(),
        "12".repeat(32),
        format!("0x{}", "12".repeat(31)),
        format!("0x{}", "12".repeat(33)),
        format!("0x{}gg", "12".repeat(31)),
        format!("0x0x{}", "12".repeat(31)),
        valid.clone(),
    ];
    let worker = thread::spawn(move || {
        for (index, hash) in hashes.into_iter().enumerate() {
            let (stream, _) = listener.accept().expect("custody connection");
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("read timeout");
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .expect("write timeout");
            let mut stream = acceptor.accept(stream).expect("custody TLS handshake");
            let (headers, request) = read_request(&mut stream);
            assert!(headers.contains("Authorization: Bearer test-custody-credential"));
            if index == 6 {
                assert!(headers.starts_with("GET /status/by-idempotency/"));
                assert_eq!(request, Value::Null);
            } else {
                assert!(headers.starts_with("POST /broadcast HTTP/1.1"));
                assert_eq!(request["amount"], 100);
                assert_eq!(request["asset"], json!([1; 32].to_vec()));
                assert_eq!(request["idempotency_key"], json!([2; 32].to_vec()));
            }
            respond(
                &mut stream,
                &serde_json::to_value(PaxeerSubmission {
                    operation_id: "custody-transfer".into(),
                    idempotency_key: [2; 32],
                    operator_account: "operator".into(),
                    wallet_address: "wallet".into(),
                    vault_id: "vault".into(),
                    asset: [1; 32],
                    amount: 100,
                    transaction_hash: hash,
                })
                .expect("custody answer"),
            );
        }
    });
    let client = custody(&files, &endpoint);
    assert_eq!(
        client
            .broadcast([1; 32], 100, [2; 32])
            .expect("prefixed hash")
            .transaction_hash,
        valid
    );
    for _ in 0..5 {
        assert_eq!(
            client.broadcast([1; 32], 100, [2; 32]),
            Err(RampError::Paxeer)
        );
    }
    assert_eq!(
        client
            .reconcile([1; 32], 100, [2; 32])
            .expect("prefixed status hash")
            .transaction_hash,
        valid
    );
    worker.join().expect("custody listener assertions");
}

struct RpcListener {
    endpoint: String,
    chain_id: Arc<AtomicU64>,
    requests: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl RpcListener {
    fn new() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback JSON-RPC listener");
        let endpoint = format!("http://{}", listener.local_addr().expect("RPC address"));
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let chain_id = Arc::new(AtomicU64::new(126));
        let chain = chain_id.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let observed = requests.clone();
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("RPC accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("RPC timeout");
                let (header, request) = read_request(&mut stream);
                assert!(header.starts_with("POST / HTTP/1.1"));
                assert_eq!(request["jsonrpc"], "2.0");
                let result = match request["method"].as_str().expect("RPC method") {
                    "eth_chainId" => {
                        assert_eq!(request["params"], json!([]));
                        json!(format!("{:#x}", chain.load(Ordering::Acquire)))
                    }
                    "px_getNetwork" => {
                        assert_eq!(request["params"], json!([]));
                        json!({"network_id": "paxeer", "paxeer": {"chain_id": 125, "latest_block": 20}, "layerx": null, "latest_committed_height": 20})
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(request["params"], json!(["0x14", true]));
                        json!({"number": "0x14", "hash": format!("0x{}", "34".repeat(32)),
                            "parentHash": format!("0x{}", "56".repeat(32)), "timestamp": "0x1",
                            "transactionsRoot": format!("0x{}", "78".repeat(32)), "transactions": []})
                    }
                    method => panic!("unexpected RPC before order validation: {method}"),
                };
                observed.lock().expect("record RPC").push(request.clone());
                respond(
                    &mut stream,
                    &json!({"jsonrpc": "2.0", "id": request["id"], "result": result}),
                );
            }
        });
        Self {
            endpoint,
            chain_id,
            requests,
            stop,
            worker: Some(worker),
        }
    }

    fn methods(&self) -> Vec<String> {
        self.requests
            .lock()
            .expect("RPC requests")
            .iter()
            .map(|request| request["method"].as_str().expect("method").to_owned())
            .collect()
    }
}

impl Drop for RpcListener {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("RPC worker")
            .join()
            .expect("RPC assertions");
    }
}

#[test]
fn recovery_refuses_chain_id_mismatch() {
    let files = ClientFiles::new();
    let rpc = RpcListener::new();
    let mut config = files.config();
    config.rpc_endpoint = rpc.endpoint.clone();
    let layerx = config.build(files.http()).expect("sequencer client");
    let provider = ProviderClient {
        http: files.http(),
        endpoint: Endpoint::parse("https://localhost:1").expect("provider endpoint"),
        credential: "test-provider-credential".into(),
        settlement_path: "/settlements".into(),
        status_path: "/status".into(),
    };
    let custody = custody(&files, "https://localhost:1");
    let order = order_for(RampDirection::OnRamp);
    let mut operator = order.operator.clone();
    operator.principal_id.push_str("-wrong");
    let mut journal = Journal::open(files.0.join("recovery.jsonl")).expect("real journal");
    journal
        .create_order(order.clone(), 1000)
        .expect("journal bound order");
    assert_eq!(
        verify_recovery_settlement(
            journal.projection(),
            &operator,
            &provider,
            &layerx,
            &custody
        ),
        Err(RampError::Paxeer)
    );
    assert_eq!(rpc.methods(), ["eth_chainId"]);
    assert_eq!(
        verify_recovery_settlement(
            &Projection::default(),
            &operator,
            &provider,
            &layerx,
            &custody
        ),
        Err(RampError::Paxeer)
    );
    assert_eq!(rpc.methods(), ["eth_chainId", "eth_chainId"]);
    rpc.chain_id.store(125, Ordering::Release);
    assert_eq!(
        verify_recovery_settlement(
            journal.projection(),
            &operator,
            &provider,
            &layerx,
            &custody
        ),
        Err(RampError::OrderBinding)
    );
    assert_eq!(
        verify_recovery_settlement(
            &Projection::default(),
            &order.operator,
            &provider,
            &layerx,
            &custody
        ),
        Ok(())
    );
    assert_eq!(
        rpc.methods(),
        [
            "eth_chainId",
            "eth_chainId",
            "eth_chainId",
            "px_getNetwork",
            "eth_getBlockByNumber",
            "eth_chainId",
            "px_getNetwork",
            "eth_getBlockByNumber"
        ]
    );
}
