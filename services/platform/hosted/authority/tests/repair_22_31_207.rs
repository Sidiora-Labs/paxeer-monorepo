//! Regression evidence for audit 22.30.85: real native execution, reopened
//! SDK membership proofs, loopback transport and the actual TLS authority.
use layerx_platform_authority::assertion::Must;

use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
use layerx_platform_authority::{hex, receipt_locator};
use layerx_types::payload::{ActivityType, ModuleId};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::{BufRead, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex, OnceLock};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|e| panic!("{e:?}"))
}

fn write(path: &Path, bytes: &[u8]) {
    must(fs::write(path, bytes));
    must(fs::set_permissions(path, fs::Permissions::from_mode(0o600)));
}

fn openssl(root: &Path, arguments: &[&str]) {
    let result = must(
        Command::new("openssl")
            .current_dir(root)
            .args(arguments)
            .output(),
    );
    assert!(
        result.status.success(),
        "openssl failed: {}",
        String::from_utf8_lossy(&result.stderr)
    );
}

struct Server {
    child: Child,
    root: PathBuf,
}
impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

fn prepare_root() -> PathBuf {
    let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join("repair-22-31-207");
    must(fs::create_dir_all(&root));
    let mut nonce = [0_u8; 8];
    must(getrandom::fill(&mut nonce));
    let root = must(fs::canonicalize(root)).join(format!(
        "human-tls-{}-{}",
        std::process::id(),
        hex::encode(&nonce)
    ));
    must(fs::create_dir(&root));
    must(fs::set_permissions(
        &root,
        fs::Permissions::from_mode(0o700),
    ));
    let state = root.join("state");
    must(fs::create_dir(&state));
    must(fs::set_permissions(
        &state,
        fs::Permissions::from_mode(0o700),
    ));
    prepare_tls(&root);

    root
}

fn prepare_tls(root: &Path) {
    openssl(
        root,
        &[
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "ca.key",
            "-out",
            "ca.pem",
            "-days",
            "1",
            "-subj",
            "/CN=Human authority test CA",
            "-addext",
            "basicConstraints=critical,CA:TRUE",
            "-addext",
            "keyUsage=critical,keyCertSign,cRLSign",
        ],
    );
    openssl(
        root,
        &[
            "req",
            "-new",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-keyout",
            "server.key",
            "-out",
            "server.csr",
            "-subj",
            "/CN=localhost",
        ],
    );
    write(
        &root.join("server.extensions"),
        b"basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:localhost,IP:127.0.0.1\n",
    );
    openssl(
        root,
        &[
            "x509",
            "-req",
            "-in",
            "server.csr",
            "-CA",
            "ca.pem",
            "-CAkey",
            "ca.key",
            "-CAcreateserial",
            "-days",
            "1",
            "-extfile",
            "server.extensions",
            "-out",
            "server.pem",
        ],
    );
    for certificate in ["ca", "server"] {
        openssl(
            root,
            &[
                "x509",
                "-in",
                &format!("{certificate}.pem"),
                "-outform",
                "DER",
                "-out",
                &format!("{certificate}.der"),
            ],
        );
    }
    openssl(
        root,
        &[
            "pkcs8",
            "-topk8",
            "-nocrypt",
            "-in",
            "server.key",
            "-outform",
            "DER",
            "-out",
            "key.der",
        ],
    );
}

fn launch_at(root: PathBuf, node: &Node) -> (Server, std::net::SocketAddr) {
    let state = root.join("state");
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x51; 32]);
    let public_key = signer.verifying_key().to_bytes();
    let key_id: [u8; 32] = Sha256::digest(public_key).into();
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let address = must(listener.local_addr());
    drop(listener);
    let output = must(fs::File::create(root.join("server.log")));
    let child = must(
        Command::new(env!("CARGO_BIN_EXE_layerx-receipt-authority"))
            .env_clear()
            .env("PATH", std::env::var_os("PATH").unwrap_or_default())
            .env("LAYERX_AUTHORITY_LISTEN", address.to_string())
            .env("LAYERX_AUTHORITY_TLS_CERT_DER", root.join("server.der"))
            .env("LAYERX_AUTHORITY_TLS_KEY_DER", root.join("key.der"))
            .env("LAYERX_AUTHORITY_TOKEN_FILES", root.join("service.token"))
            .env("LAYERX_AUTHORITY_REPLICA_URL", &node.base)
            .env(
                "LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE",
                root.join("replica.token"),
            )
            .env("LAYERX_AUTHORITY_REPLICA_ID", hex::encode(&[9; 32]))
            .env(
                "LAYERX_AUTHORITY_CHAIN_RPC_URL",
                format!("{}/evm", node.base),
            )
            .env(
                "LAYERX_AUTHORITY_RECEIPT_KEYS_FILE",
                root.join("receipt-keys.json"),
            )
            .env("LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID", "7")
            .env("LAYERX_AUTHORITY_NETWORK_ID", "human-authority-test")
            .env("LAYERX_AUTHORITY_SEQUENCER_ID", hex::encode(&key_id))
            .env(
                "LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY",
                hex::encode(&public_key),
            )
            .env("LAYERX_AUTHORITY_FIRST_BATCH", "1")
            .env("LAYERX_AUTHORITY_LAST_BATCH", "100")
            .env(
                "LAYERX_AUTHORITY_HUMAN_AGENT_TOKEN_FILE",
                root.join("human.token"),
            )
            .env("LAYERX_AUTHORITY_HUMAN_AGENT_TENANT", "tenant space")
            .env("LAYERX_AUTHORITY_HUMAN_AGENT_PRINCIPAL", "principal")
            .env(
                "LAYERX_AUTHORITY_PRINCIPAL_POLICY_FILE",
                root.join("policy.json"),
            )
            .env(
                "LAYERX_AUTHORITY_MODULE_REGISTRY_FILE",
                root.join("modules.json"),
            )
            .env("LAYERX_AUTHORITY_CORE_CLOCK_HORIZON", "100")
            .env("LAYERX_AUTHORITY_STATE_ROOT", &state)
            .stdout(Stdio::null())
            .stderr(output)
            .spawn(),
    );
    let mut server = Server { child, root };
    let deadline = Instant::now() + Duration::from_secs(10);
    loop {
        assert!(
            must(server.child.try_wait()).is_none(),
            "authority exited; see {}/server.log: {}",
            server.root.display(),
            must(fs::read_to_string(server.root.join("server.log")))
        );
        if TcpStream::connect(address).is_ok() {
            break;
        }
        assert!(Instant::now() < deadline, "authority startup timed out");
        std::thread::sleep(Duration::from_millis(20));
    }
    (server, address)
}
fn wire_status_at(root: &Path, address: &str, path: &str, token: &str) -> (u16, String) {
    let certificate = must(native_tls::Certificate::from_pem(&must(fs::read(
        root.join("ca.pem"),
    ))));
    let connector = must(
        native_tls::TlsConnector::builder()
            .add_root_certificate(certificate)
            .build(),
    );
    let tcp = must(TcpStream::connect(address));
    must(tcp.set_read_timeout(Some(Duration::from_secs(5))));
    let mut tls = must(connector.connect("localhost", tcp));
    must(write!(tls, "GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nConnection: close\r\n\r\n"));
    let mut response = String::new();
    must(tls.read_to_string(&mut response));
    let status = must(
        response
            .split_whitespace()
            .nth(1)
            .unwrap_or_else(|| panic!("status absent"))
            .parse(),
    );
    let body = response
        .split_once("\r\n\r\n")
        .unwrap_or_else(|| panic!("HTTP body absent: {response}"))
        .1
        .to_owned();
    (status, body)
}

fn base64(bytes: &[u8]) -> String {
    const A: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for chunk in bytes.chunks(3) {
        let value = (u32::from(chunk[0]) << 16)
            | (u32::from(chunk.get(1).copied().unwrap_or(0)) << 8)
            | u32::from(chunk.get(2).copied().unwrap_or(0));
        for (shift, present) in [
            (18, true),
            (12, true),
            (6, chunk.len() > 1),
            (0, chunk.len() > 2),
        ] {
            out.push(if present {
                char::from(A[((value >> shift) & 63) as usize])
            } else {
                '='
            });
        }
    }
    out
}

fn run(command: &mut Command, label: &str, log: &Path) {
    let output = must(command.output());
    let mut bytes = output.stdout;
    bytes.extend_from_slice(&output.stderr);
    must(fs::write(log, &bytes));
    assert!(
        output.status.success(),
        "{label} failed ({:?}); {}\n{}",
        output.status.code(),
        log.display(),
        String::from_utf8_lossy(&bytes)
    );
}

fn corpus() -> &'static Value {
    static CORPUS: OnceLock<Value> = OnceLock::new();
    CORPUS.get_or_init(|| {
        let repository = must(
            Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../..")
                .canonicalize(),
        );
        let root = prepare_root();
        // These are producer prerequisites, built inside this focused integration
        // test; no additional verification or workspace gates are invoked.
        run(
            Command::new("make")
                .current_dir(&repository)
                .args(["-s", "-j4", "build/liblayerx.a"]),
            "native kernel producer",
            &root.join("kernel-build.log"),
        );
        let target = root.join("producer-target");
        run(
            Command::new("cargo")
                .current_dir(repository.join("programs"))
                .args([
                    "build",
                    "--locked",
                    "-p",
                    "layerx-programs-sandbox",
                    "--features",
                    "host-ffi",
                ])
                .env("CARGO_TARGET_DIR", &target),
            "native host runtime",
            &root.join("runtime-build.log"),
        );
        let now: u64 = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .must()
            .as_millis()
            .try_into()
            .must();
        let public = ed25519_dalek::SigningKey::from_bytes(&[0x61; 32])
            .verifying_key()
            .to_bytes();
        let did = format!("did:layerx:{}", hex::encode(&public));
        let grant = must(issue_session_key(&SessionKeyRequest {
            grantor: must(layerx_wire::hash::did_id_for_protocol(
                &must(layerx_types::ids::Did::new(did.as_bytes())),
                3,
            )),
            session_public_key: ed25519_dalek::SigningKey::from_bytes(&[0x63; 32])
                .verifying_key()
                .to_bytes(),
            not_before: now - 1000,
            expires_at: Some(now + 3_600_000),
            permitted_activity_types: vec![must(ActivityType::new(ModuleId::Asset, 5))],
            revocation_sequence: Some(1),
            fee_budget: None,
            purpose: SessionPurpose::Activity,
        }));
        let mut producer = must(
            Command::new("go")
                .current_dir(&repository)
                .args([
                    "run",
                    "services/platform/hosted/authority/src/repair_22_31_207/producer.go",
                ])
                .env("CGO_ENABLED", "1")
                .env(
                    "CGO_LDFLAGS",
                    format!("-L{}", target.join("debug").display()),
                )
                .stdin(Stdio::piped())
                .stdout(Stdio::piped())
                .stderr(must(fs::File::create(root.join("producer.log"))))
                .spawn(),
        );
        must(serde_json::to_writer(
            producer.stdin.take().must(),
            &json!({
            "Directory":root, "Now":now, "Grant":hex::encode(&grant.registration_payload)}),
        ));
        let output = must(producer.wait_with_output());
        assert!(
            output.status.success(),
            "real SDK producer failed: {}",
            must(fs::read_to_string(root.join("producer.log")))
        );
        let corpus: Value = must(serde_json::from_slice(&must(fs::read(
            root.join("corpus.json"),
        ))));
        assert_eq!(corpus["did"], did);
        assert!(!corpus["queries"].as_object().must().is_empty());
        corpus
    })
}

#[derive(Clone)]
struct Receipt {
    bytes: Vec<u8>,
    document: Value,
    activity: String,
    digest: String,
    replica_path: String,
    key_id: [u8; 32],
    public: [u8; 32],
    height: u64,
}

fn produced_receipt(kind: &str, height: u64, seed: u8) -> Receipt {
    let signer = ed25519_dalek::SigningKey::from_bytes(&[seed; 32]);
    let public = signer.verifying_key().to_bytes();
    let key_id: [u8; 32] = Sha256::digest(public).into();
    let bytes = must(hex::decode(corpus()[kind].as_str().must()));
    let decoded = must(layerx_wire::receipt::decode(&bytes));
    let protocol = decoded.protocol().must();
    assert_eq!(protocol.global_sequence(), height);
    assert_eq!(protocol.result_code(), 0);
    assert_eq!(protocol.context_hash(), key_id);
    let document = corpus()[format!("{kind}_evidence")].clone();
    let evidence = must(layerx_platform_authority::parse_replica_evidence(
        &must(serde_json::to_vec(&document)),
        [9; 32],
        public,
    ));
    let header = must(layerx_wire::receipt::decode_batch_header(&evidence.header));
    assert_eq!(header.batch_number(), height);
    assert_eq!(
        must(layerx_wire::hash::receipt_execution_batch_id(
            protocol, &header
        )),
        protocol.batch_id()
    );
    let locator = must(receipt_locator(&bytes));
    let activity = hex::encode(&locator.activity_id);
    let digest = hex::encode(&locator.receipt_digest);
    Receipt {
        replica_path: format!(
            "/v1/batches/{}/receipt-authority?receipt_digest={digest}",
            hex::encode(&locator.batch_id)
        ),
        bytes,
        document,
        activity,
        digest,
        key_id,
        public,
        height,
    }
}

struct Node {
    base: String,
    receipts: Vec<Receipt>,
    fault: Arc<Mutex<&'static str>>,
    anchors: Arc<AtomicUsize>,
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<()>>,
}

impl Node {
    fn request_head(reader: &mut std::io::BufReader<TcpStream>) -> (String, bool, usize) {
        let mut line = String::new();
        must(reader.read_line(&mut line));
        let path = line.split_whitespace().nth(1).must().to_owned();
        let get = line.starts_with("GET ");
        let mut length = 0;
        loop {
            line.clear();
            assert_ne!(must(reader.read_line(&mut line)), 0);
            if line == "\r\n" {
                break;
            }
            if let Some(s) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = must(s.trim().parse::<usize>());
            }
        }
        assert!(length < 65536);
        (path, get, length)
    }

    fn open() -> Self {
        let corpus = corpus().clone();
        let receipts = vec![
            produced_receipt("rotation", 2, 0x51),
            produced_receipt("grant", 3, 0x52),
        ];
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let base = format!("http://{}", must(listener.local_addr()));
        let stop = Arc::new(AtomicBool::new(false));
        let fault = Arc::new(Mutex::new(""));
        let anchors = Arc::new(AtomicUsize::new(0));
        let (thread_stop, thread_fault, thread_anchors) =
            (stop.clone(), fault.clone(), anchors.clone());
        let served = receipts.clone();
        let worker = std::thread::spawn(move || {
            while !thread_stop.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(e) => panic!("node: {e}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(10))));
                let mut reader = std::io::BufReader::new(stream);
                let (path, get, length) = Self::request_head(&mut reader);
                let fault = *thread_fault.lock().must();
                let answer = if get {
                    assert_ne!(fault,"offline-replica", "replay unexpectedly fetched replica");
                    served.iter().find(|r| r.replica_path == path).must_with("real replica locator").document.clone()
                } else {
                    let mut bytes = vec![0;length]; must(reader.read_exact(&mut bytes));
                    let request: Value = must(serde_json::from_slice(&bytes));
                    let method = request["method"].as_str().must();
                    let mut result = match method {
                        "eth_chainId" => json!("0x7d"),
                        "eth_blockNumber" => json!("0x9"),
                        "eth_getBlockByNumber" => corpus["blocks"][request["params"][0].as_str().must()].clone(),
                        "px_getCommit" => { assert_eq!(request["params"],json!(["9"])); corpus["commit"].clone() },
                        "pax_sequencerReceipt" => {
                            let id = request["params"][0].as_str().must().strip_prefix("0x").must();
                            let r = served.iter().find(|r| r.activity == id).must();
                            json!({"txHash":format!("0x{id}"),"rung":"committed", "height":format!("0x{:x}",r.height),
                                "blockNumber":format!("0x{:x}",r.height),"blockHash":corpus["blocks"][format!("0x{:x}",r.height)]["hash"],
                                "receipt":format!("0x{}",hex::encode(&r.bytes)),"keyId":format!("0x{}",hex::encode(&r.key_id)),
                                "signingKey":format!("0x{}",hex::encode(&r.public))})
                        }
                        "abci_query" => {
                            assert_eq!(path,"/comet");
                            assert_eq!(request["params"]["path"],"/store/lxphost/key");
                            assert_eq!(request["params"]["height"],"8"); assert_eq!(request["params"]["prove"],true);
                            let key = request["params"]["data"].as_str().must().strip_prefix("0x").must();
                            let mut query = corpus["queries"][key].clone();
                            assert!(query.is_object(),"missing real SDK membership {key}");
                            match fault {
                                "proof" => query["proofOps"]["ops"][0]["data"] = json!(base64(&[1,2,3])),
                                "key" => query["key"] = json!(base64(&[1;35])),
                                "value" => query["value"] = json!(base64(&[1;223])),
                                "height" => query["height"] = json!("7"),
                                _ => {}
                            }
                            json!({"response":query})
                        }
                        "eth_call" => {
                            thread_anchors.fetch_add(1,Ordering::AcqRel);
                            // Upgrade-active node: the retired anchor always refuses.
                            let answer = json!({"jsonrpc":"2.0","id":request["id"],"error":{"code":-32000,"message":"ErrModuleRetired"}}).to_string();
                            let mut stream = reader.into_inner();
                            must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()));
                            continue;
                        }
                        _ => panic!("undeclared node method {method}"),
                    };
                    if matches!(fault,"header"|"apphash") && method == "px_getCommit" { result["signedHeader"]["header"]["app_hash"] = json!(hex::encode(&[1;32])); }
                    if fault == "apphash" && method == "eth_getBlockByNumber" && request["params"][0] == "0x9" { result["stateRoot"] = json!(format!("0x{}",hex::encode(&[1;32]))); }
                    if fault == "uncommitted" && method == "pax_sequencerReceipt" { result["rung"] = json!("receipted"); }
                    json!({"jsonrpc":"2.0","id":request["id"],"result":result})
                }.to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()));
            }
        });
        Self {
            base,
            receipts,
            fault,
            anchors,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().must();
        }
    }
}

fn configure(root: &Path, node: &Node) {
    let receipts = &node.receipts;
    let reference = |r: &Receipt| json!({"activity_id":r.activity,"receipt_digest":r.digest});
    let decoded = must(layerx_wire::receipt::decode(&receipts[0].bytes));
    let state = decoded
        .protocol()
        .must()
        .effects()
        .iter()
        .find(|e| e.event_type() == 0x7110)
        .must()
        .body();
    let number = |offset| u64::from_be_bytes(state[offset..offset + 8].try_into().must());
    let granted = must(layerx_wire::receipt::decode(&receipts[1].bytes));
    let summary = granted
        .protocol()
        .must()
        .effects()
        .iter()
        .find(|e| e.event_type() == 0x7145)
        .must()
        .body();
    let rotation = json!({"policy_revision":number(167),"required_delay_seconds":number(199).div_ceil(1000),
        "maximum_delay_seconds":number(207)/1000,"effective_sequence":2,"evidence":reference(&receipts[0])});
    let policy = json!({"principals":[{"tenant":"tenant space","principal":"principal","account_id":hex::encode(&[1;32]),
        "asset_id":hex::encode(&[2;32]),"activities":receipts.iter().map(|r|r.activity.clone()).collect::<Vec<_>>(),"budgets":[],
        "maximum_age_seconds":3600,"maximum_age_sequences":100,"identities":[{"did":corpus()["did"],
        "authorities":[{"kind":"primary_key","id":hex::encode(&state[37..69])}],"revocation_sequence":number(69),"frozen":false,
        "evidence":reference(&receipts[0]),"rotation":rotation,"recovery":rotation,
        "capabilities":[{"authority":hex::encode(&summary[69..101]),"action_key":hex::encode(&summary[101..133]),
            "capability_id":hex::encode(&summary[5..37]),"activity_types":[5],"counterparties":[],"assets":[],"amount_ceiling":"0",
            "expiry_sequence":10000,"enforceable_dimensions":[],"evidence":reference(&receipts[1])}]}]}]});
    write(
        &root.join("policy.json"),
        &must(serde_json::to_vec(&policy)),
    );
    write(&root.join("receipt-keys.json"),&must(serde_json::to_vec(&json!(receipts.iter().map(|r|json!({
        "keyId":base64(&r.key_id),"publicKey":base64(&r.public),"firstHeight":r.height,"lastHeight":r.height
    })).collect::<Vec<_>>()))));
    write(&root.join("modules.json"),br#"{"schema_version":2,"assets":[{"asset":"0202020202020202020202020202020202020202020202020202020202020202","currency":"USD","decimals":6,"symbol":"$"}],"modules":[{"module":7,"ordinals":[1,2,5]}]}"#);
    for (name, token) in [
        ("service", "service-test-token-000000000000000000"),
        ("replica", "replica-test-token-000000000000000000"),
        ("human", "human-test-token-0000000000000000000000"),
    ] {
        write(&root.join(format!("{name}.token")), token.as_bytes());
    }
}

fn request(server: &Server, address: std::net::SocketAddr, path: &str, expected: u16) -> Value {
    let response = wire_status_at(
        &server.root,
        &address.to_string(),
        path,
        if path.starts_with("/v1/agent/") {
            "human-test-token-0000000000000000000000"
        } else {
            "service-test-token-000000000000000000"
        },
    );
    assert_eq!(
        response.0,
        expected,
        "{}; {}",
        response.1,
        must(fs::read_to_string(server.root.join("server.log")))
    );
    must(serde_json::from_str(&response.1))
}
fn retain(server: &Server, address: std::net::SocketAddr, node: &Node) {
    for r in &node.receipts {
        let document = request(
            server,
            address,
            &format!("/v1/authorized-batches/by-activity/{}", r.activity),
            200,
        );
        assert_eq!(document["sequencer_public_key"], hex::encode(&r.public));
        let held: Value = must(serde_json::from_slice(&must(fs::read(
            server
                .root
                .join("state")
                .join(format!("{}.json", r.activity)),
        ))));
        assert_eq!(held["receipt_hex"], hex::encode(&r.bytes));
    }
}
fn human_paths() -> Vec<String> {
    let did = corpus()["did"].as_str().must();
    let grant = must(hex::decode(corpus()["grant"].as_str().must()));
    let decoded = must(layerx_wire::receipt::decode(&grant));
    let summary = decoded
        .protocol()
        .must()
        .effects()
        .iter()
        .find(|e| e.event_type() == 0x7145)
        .must()
        .body();
    let base = format!("tenant=tenant%20space&principal=principal&did={did}");
    vec![
        format!("/v1/agent/identity?{base}"),
        format!("/v1/agent/key-policy?{base}&recovery=false"),
        format!(
            "/v1/agent/capability-scope?{base}&authority={}&action_key={}&capability_id={}",
            hex::encode(&summary[69..101]),
            hex::encode(&summary[101..133]),
            hex::encode(&summary[5..37])
        ),
    ]
}

#[test]
fn retired_anchor_finality_binds_identity_capability_and_key_policy() {
    let node = Node::open();
    let root = prepare_root();
    configure(&root, &node);
    let (server, address) = launch_at(root, &node);
    retain(&server, address, &node);
    for path in human_paths() {
        let document = request(&server, address, &path, 200);
        if path.contains("/identity?") {
            assert_eq!(document["head_sequence"], 3);
            let authorities = document["authorities"].as_array().must();
            assert_eq!(authorities.len(), 2);
            assert!(authorities.iter().any(|a| a["kind"] == "session_key"));
        }
        if path.contains("/capability-scope?") {
            assert_eq!(document["activity_types"], json!([5]));
            assert_eq!(document["observed_sequence"], 3);
        }
        assert_eq!(document["finality"]["height"], 9);
        assert_eq!(document["finality"]["state_height"], 8);
        assert_eq!(
            document["finality"]["app_hash"],
            corpus()["blocks"]["0x9"]["stateRoot"]
                .as_str()
                .must()
                .trim_start_matches("0x")
        );
        assert_eq!(
            document["finality"]["block_hash"],
            corpus()["blocks"]["0x9"]["hash"]
                .as_str()
                .must()
                .trim_start_matches("0x")
        );
    }
    assert_eq!(node.anchors.load(Ordering::Acquire), 0);
    // Each altered interface coordinate or proof must fail closed on all routes.
    for fault in [
        "header",
        "apphash",
        "proof",
        "key",
        "value",
        "height",
        "uncommitted",
    ] {
        *node.fault.lock().must() = fault;
        for path in human_paths() {
            request(&server, address, &path, 503);
        }
    }
}

#[test]
fn rotated_human_receipt_key_lookup_retention_and_replay() {
    let node = Node::open();
    let root = prepare_root();
    configure(&root, &node);
    let (server, address) = launch_at(root.clone(), &node);
    retain(&server, address, &node);
    for path in human_paths() {
        request(&server, address, &path, 200);
    }
    drop(server); // Stop authority, then reload its actual durable evidence.
    *node.fault.lock().must() = "offline-replica";
    let (server, address) = launch_at(root.clone(), &node);
    let rotated = &node.receipts[1];
    let held = request(
        &server,
        address,
        &format!(
            "/v1/agent/authorized-batch?tenant=tenant%20space&principal=principal&activity_id={}",
            rotated.activity
        ),
        200,
    );
    assert_eq!(held["receipt_key_id"], hex::encode(&rotated.key_id));
    assert_eq!(held["sequencer_public_key"], hex::encode(&rotated.public));
    for path in human_paths() {
        request(&server, address, &path, 200);
    }
    drop(server);
    // Revoking applicability cannot preserve authority on restart.
    let mut pins: Value = must(serde_json::from_slice(&must(fs::read(
        root.join("receipt-keys.json"),
    ))));
    pins[1]["firstHeight"] = json!(4);
    pins[1]["lastHeight"] = json!(100);
    write(
        &root.join("receipt-keys.json"),
        &must(serde_json::to_vec(&pins)),
    );
    let (server, address) = launch_at(root.clone(), &node);
    request(&server, address, &human_paths()[0], 503);
    request(
        &server,
        address,
        &format!("/v1/authorized-batches/by-activity/{}", rotated.activity),
        502,
    );
}
