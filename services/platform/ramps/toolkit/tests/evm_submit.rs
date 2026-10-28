use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_ramp_toolkit::clients::{hex, LayerxClient, LayerxSubmission};
use layerx_ramp_toolkit::{
    compile_operator_send, compile_payer_grant_draw, AuthenticatedPrincipal, CreateOrder,
    OperatorIdentity, QuoteTerms, RampDirection, RampError, RampOrder,
};
use native_tls::{Identity, TlsAcceptor};
use std::fs;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::AtomicBool;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_ramp_toolkit::clients::{LayerxConfig, MutualTlsClient, MutualTlsFiles, SecretFile};
use serde_json::{json, Value};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

struct ClientFiles(PathBuf);

impl ClientFiles {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ramp-evm-submit-{}-{}",
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

fn order() -> RampOrder {
    order_for(RampDirection::OnRamp)
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

fn call(order: &RampOrder) -> Eip1559Call {
    let compiled = match order.direction() {
        RampDirection::OnRamp => compile_operator_send(order),
        RampDirection::OffRamp => compile_payer_grant_draw(order, 9),
    }
    .expect("compile ledger call");
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 100,
        gas_limit: 50_000,
        to: compiled.to,
        data: compiled.data,
    }
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
    (header, serde_json::from_slice(&body).expect("request JSON"))
}

fn respond(stream: &mut impl Write, body: &Value) {
    let bytes = serde_json::to_vec(body).expect("response JSON");
    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", bytes.len()).expect("response header");
    stream.write_all(&bytes).expect("response body");
    stream.flush().expect("flush response");
}

struct RpcServer {
    endpoint: String,
    requests: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl RpcServer {
    fn new(tx_hash: [u8; 32]) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback JSON-RPC listener");
        let endpoint = format!("http://{}", listener.local_addr().expect("RPC address"));
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let requests = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let observed = requests.clone();
        let stopped = stop.clone();
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("accept RPC: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("read timeout");
                let (_, request) = read_request(&mut stream);
                let result = match request["method"].as_str().expect("RPC method") {
                    "eth_getTransactionCount" => {
                        assert_eq!(request["params"][1], "pending");
                        json!("0x7")
                    }
                    "eth_estimateGas" => {
                        assert_eq!(request["params"][1], "pending");
                        json!("0xc350")
                    }
                    "eth_sendRawTransaction" => json!(format!("0x{}", hex(&tx_hash))),
                    "pax_sequencerReceipt" => {
                        assert_eq!(request["params"], json!([format!("0x{}", hex(&tx_hash))]));
                        json!({"rung": "submitted", "txHash": format!("0x{}", hex(&tx_hash)), "nonce": "0x7"})
                    }
                    method => panic!("unexpected method: {method}"),
                };
                respond(
                    &mut stream,
                    &json!({"jsonrpc": "2.0", "id": request["id"], "result": result}),
                );
                observed.lock().expect("record RPC").push(request);
            }
        });
        Self {
            endpoint,
            requests,
            stop,
            worker: Some(worker),
        }
    }

    fn finish(&mut self) -> Vec<Value> {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("RPC worker")
            .join()
            .expect("RPC server");
        self.requests.lock().expect("RPC requests").clone()
    }
}

impl Drop for RpcServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().expect("RPC server");
        }
    }
}

fn base64(bytes: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for chunk in bytes.chunks(3) {
        let bits = (u32::from(chunk[0]) << 16)
            | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
            | u32::from(*chunk.get(2).unwrap_or(&0));
        out.push(TABLE[((bits >> 18) & 63) as usize] as char);
        out.push(TABLE[((bits >> 12) & 63) as usize] as char);
        out.push(if chunk.len() > 1 {
            TABLE[((bits >> 6) & 63) as usize] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            TABLE[(bits & 63) as usize] as char
        } else {
            '='
        });
    }
    out
}

fn signer(files: &ClientFiles, call: Eip1559Call, secret: [u8; 32]) -> (String, JoinHandle<()>) {
    let listener = TcpListener::bind(("localhost", 0)).expect("signer listener");
    listener
        .set_nonblocking(true)
        .expect("nonblocking signer listener");
    let endpoint = format!(
        "https://localhost:{}",
        listener.local_addr().expect("signer address").port()
    );
    let identity = Identity::from_pkcs12(
        &fs::read(files.0.join("identity.p12")).expect("TLS identity"),
        "configuration-test",
    )
    .expect("signer identity");
    let acceptor = TlsAcceptor::new(identity).expect("TLS signer");
    let worker = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(5);
        let (stream, _) = loop {
            match listener.accept() {
                Ok(connection) => break connection,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(Instant::now() < deadline, "signer connection deadline");
                    thread::sleep(Duration::from_millis(2));
                }
                Err(error) => panic!("accept signer: {error}"),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("signer timeout");
        let mut stream = acceptor.accept(stream).expect("authenticated TLS signer");
        let (header, request) = read_request(&mut stream);
        assert!(header.starts_with("POST /v1/signatures HTTP/1.1\r\n"));
        assert!(header.contains("Bearer configuration-test-token"));
        assert_eq!(request["algorithm"], "secp256k1");
        assert_eq!(request["key_handle"], "kms.operator-1.primary");
        assert_eq!(request["message"], base64(&call.signing_hash()));
        let raw = call.sign(&secret).expect("real secp256k1 signature");
        let (_, signature) = decode_signed(&raw).expect("signed call");
        respond(
            &mut stream,
            &json!({"signature": base64(signature.as_bytes())}),
        );
        stream.shutdown().expect("close TLS response");
    });
    (endpoint, worker)
}

fn client(
    files: &ClientFiles,
    rpc: &RpcServer,
    signer_endpoint: String,
    call: &Eip1559Call,
) -> LayerxClient {
    let raw = call.sign(&[1; 32]).expect("operator key");
    let (_, signature) = decode_signed(&raw).expect("operator transaction");
    let mut config = files.config();
    config.rpc_endpoint = rpc.endpoint.clone();
    config.actor_did = "did:layerx:operator-1".into();
    config.signer_endpoint = signer_endpoint;
    config.operator_address = format!(
        "0x{}",
        hex(&recover_sender(call, &signature).expect("operator address"))
    );
    let client = config
        .build(files.http())
        .unwrap_or_else(|error| panic!("client: {error}"));
    client
}

#[test]
fn signature_from_other_key_is_refused_before_submission() {
    let files = ClientFiles::new();
    let order = order();
    let call = call(&order);
    let mut rpc = RpcServer::new([0; 32]);
    let (endpoint, signer) = signer(&files, call.clone(), [2; 32]);
    let client = client(&files, &rpc, endpoint, &call);
    let registry = layerx_types::payload::ModuleRegistry::new(&[]).expect("empty EVM registry");
    let result = client.prepare_payment(&order, 9, 1_000, &registry);
    signer.join().expect("signer checked digest");
    assert_eq!(result, Err(RampError::Layerx));
    let requests = rpc.finish();
    assert_eq!(requests.len(), 2);
    assert!(requests
        .iter()
        .all(|request| request["method"] != "eth_sendRawTransaction"));
}

#[test]
fn submit_prepared_sends_raw_transaction_once() {
    let files = ClientFiles::new();
    for direction in [RampDirection::OnRamp, RampDirection::OffRamp] {
        let order = order_for(direction);
        let call = call(&order);
        let raw = call.sign(&[1; 32]).expect("signed transaction");
        let tx_hash = transaction_hash(&raw);
        let mut rpc = RpcServer::new(tx_hash);
        let (endpoint, signer) = signer(&files, call.clone(), [1; 32]);
        let client = client(&files, &rpc, endpoint, &call);
        let registry = layerx_types::payload::ModuleRegistry::new(&[]).expect("empty EVM registry");
        let prepared = match direction {
            RampDirection::OnRamp => client.prepare_payment(&order, 9, 1_000, &registry),
            RampDirection::OffRamp => {
                let mut changed = call.data.clone();
                changed[4] ^= 1;
                assert_eq!(
                    client.prepare_native_receive(&order, &changed, 9, 1_000, &registry),
                    Err(RampError::PayerGrantRequired)
                );
                client.prepare_native_receive(&order, &call.data, 9, 1_000, &registry)
            }
        }
        .expect("prepare and authenticate transaction");
        signer.join().expect("signer checked digest");
        assert_eq!(prepared.tx_hash(), tx_hash);
        assert_eq!(prepared.raw_transaction(), raw);
        assert_eq!(prepared.activity_id(), tx_hash);
        assert_eq!(prepared.canonical_activity(), raw);
        let submission = client.submit_prepared(&order, prepared);
        assert_eq!(submission.tx_hash(), tx_hash);
        assert_eq!(submission.raw_transaction(), Some(raw.as_slice()));
        assert_eq!(
            submission,
            LayerxSubmission::Pending {
                activity_id: tx_hash,
                canonical_activity: Some(raw.clone())
            }
        );
        let requests = rpc.finish();
        let sends: Vec<_> = requests
            .iter()
            .filter(|request| request["method"] == "eth_sendRawTransaction")
            .collect();
        assert_eq!(requests.len(), 3);
        assert_eq!(sends.len(), 1);
        assert_eq!(sends[0]["params"], json!([format!("0x{}", hex(&raw))]));
        assert_eq!(
            requests[0]["params"][0],
            format!("0x{}", hex(&client.operator_address))
        );
        assert_eq!(
            requests[1]["params"][0]["to"],
            format!("0x{}", hex(&call.to))
        );
        assert_eq!(
            requests[1]["params"][0]["data"],
            format!("0x{}", hex(&call.data))
        );
    }
}

#[test]
fn resolve_reports_submitted_receipt_as_pending() {
    let files = ClientFiles::new();
    let order = order();
    let call = call(&order);
    let tx_hash = transaction_hash(&call.sign(&[1; 32]).expect("signed transaction"));
    let mut rpc = RpcServer::new(tx_hash);
    let client = client(&files, &rpc, "https://localhost:1".into(), &call);
    let submission = client.resolve(&order, tx_hash).expect("submitted receipt");
    assert_eq!(submission.tx_hash(), tx_hash);
    assert_eq!(submission.raw_transaction(), None);
    assert_eq!(
        submission,
        LayerxSubmission::Pending {
            activity_id: tx_hash,
            canonical_activity: None
        }
    );
    let requests = rpc.finish();
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0]["method"], "pax_sequencerReceipt");
}
