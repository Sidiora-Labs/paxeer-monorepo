include!("repair_22_33_1471.rs");
use layerx_agentd::human::HumanPeer;
use layerx_agentd::human_runtime::{HumanAuthorityBoundary, RemoteHumanAuthority};
use layerx_types::ids::Did;
use serde_json::json;
use std::process::{Child, Command, Stdio};

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
    let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join("hm2");
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

fn signed_fixture() -> Value {
    let fixture: Value = must(serde_json::from_str(include_str!(
        "fixtures/real-program-deploy-receipt.json"
    )));
    sign_fixture(fixture)
}

fn sign_fixture(mut fixture: Value) -> Value {
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x51; 32]);
    let public_key = signer.verifying_key().to_bytes();
    let key_id: [u8; 32] = Sha256::digest(public_key).into();
    assert_ne!(key_id, [0; 32]);
    let mut receipt = must(hex::decode(fixture["receipt_hex"].as_str().unwrap()));
    let decoded = must(layerx_wire::receipt::decode(&receipt));
    let protocol = decoded.protocol().unwrap();
    let mut context = layerx_wire::encode::Encoder::new(44);
    must(context.bytes(&protocol.context_hash(), 32));
    must(context.u64(protocol.timestamp()));
    let context = context.finish();
    let offset = receipt
        .windows(context.len())
        .position(|bytes| bytes == context)
        .unwrap()
        + 4;
    receipt[offset..offset + 32].copy_from_slice(&key_id);
    let decoded = must(layerx_wire::receipt::decode(&receipt));
    assert_eq!(decoded.protocol().unwrap().context_hash(), key_id);
    let unsigned = must(layerx_wire::receipt::encode_unsigned(&decoded));
    let signature = signer.sign(&must(layerx_wire::hash::receipt_digest(&unsigned)));
    let offset = receipt.len() - 64;
    receipt[offset..].copy_from_slice(&signature.to_bytes());

    let mut header = must(hex::decode(fixture["header_hex"].as_str().unwrap()));
    let decoded = must(layerx_wire::receipt::decode_batch_header(&header));
    for (old, new) in [
        (decoded.sequencer_id(), key_id),
        (
            decoded.receipt_merkle_root(),
            must(layerx_wire::hash::merkle_leaf(&receipt)),
        ),
    ] {
        let offset = header.windows(32).position(|word| word == old).unwrap();
        header[offset..offset + 32].copy_from_slice(&new);
    }
    let signature = signer.sign(&must(layerx_wire::hash::batch_header_digest(&header)));
    fixture["receipt_hex"] = json!(hex::encode(&receipt));
    fixture["header_hex"] = json!(hex::encode(&header));
    fixture["header_signature_hex"] = json!(hex::encode(&signature.to_bytes()));
    fixture["sequencer_id_hex"] = json!(hex::encode(&key_id));
    fixture["sequencer_public_key_hex"] = json!(hex::encode(&public_key));
    fixture
}

fn seed(root: &Path) {
    seed_fixture(root, &signed_fixture());
}

fn seed_fixture(root: &Path, fixture: &Value) {
    let state = root.join("state");
    let text = |key: &str| {
        fixture[key]
            .as_str()
            .unwrap_or_else(|| panic!("fixture field {key}"))
    };
    let encode_key = |bytes: &[u8]| {
        const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
        let mut output = String::new();
        for chunk in bytes.chunks(3) {
            let a = chunk[0];
            let b = chunk.get(1).copied().unwrap_or(0);
            let c = chunk.get(2).copied().unwrap_or(0);
            output.push(char::from(ALPHABET[usize::from(a >> 2)]));
            output.push(char::from(ALPHABET[usize::from(((a & 3) << 4) | (b >> 4))]));
            output.push(if chunk.len() > 1 {
                char::from(ALPHABET[usize::from(((b & 15) << 2) | (c >> 6))])
            } else {
                '='
            });
            output.push(if chunk.len() > 2 {
                char::from(ALPHABET[usize::from(c & 63)])
            } else {
                '='
            });
        }
        output
    };
    write(
        &root.join("receipt-keys.json"),
        &must(serde_json::to_vec(&json!([{
            "keyId":encode_key(&must(hex::decode(text("sequencer_id_hex")))),
            "publicKey":encode_key(&must(hex::decode(text("sequencer_public_key_hex")))),
            "firstHeight":1,"lastHeight":100
        }]))),
    );
    let receipt = must(hex::decode(text("receipt_hex")));
    let locator = must(receipt_locator(&receipt));
    let activity = hex::encode(&locator.activity_id);
    let reference =
        json!({"activity_id": activity, "receipt_digest": hex::encode(&locator.receipt_digest)});
    let key_policy = json!({"policy_revision":1,"required_delay_seconds":10,"maximum_delay_seconds":20,"effective_sequence":1,"evidence":reference});
    let policy = json!({"principals":[{"tenant":"tenant space","principal":"principal","account_id":hex::encode(&[1;32]),"asset_id":hex::encode(&[2;32]),"activities":[activity],"budgets":[hex::encode(&[4;32])],"maximum_age_seconds":60,"maximum_age_sequences":100,"identities":[{"did":"did:layerx:test","authorities":[{"kind":"primary_key","id":hex::encode(&[5;32])}],"revocation_sequence":1,"frozen":false,"evidence":reference,"capabilities":[],"rotation":key_policy,"recovery":key_policy}]}]});
    write(
        &root.join("policy.json"),
        &must(serde_json::to_vec(&policy)),
    );
    write(
        &root.join("modules.json"),
        br#"{"schema_version":2,"assets":[{"asset":"0202020202020202020202020202020202020202020202020202020202020202","currency":"USD","decimals":6,"symbol":"$"}],"modules":[{"module":9,"ordinals":[1,7]}]}"#,
    );
    write(
        &root.join("human.token"),
        b"human-test-token-0000000000000000000000",
    );
    write(
        &root.join("service.token"),
        b"service-test-token-00000000000000000000",
    );
    write(
        &root.join("replica.token"),
        b"replica-test-token-00000000000000000000",
    );
    let mut encoder = layerx_wire::encode::Encoder::new(64);
    must(encoder.structure_header(0x4d50));
    must(encoder.u32(0));
    must(encoder.u32(1));
    must(encoder.u8(0));
    must(encoder.bytes(&[], 1024));
    let proof = encoder.finish();
    let record = json!({"receipt_hex": text("receipt_hex"), "replica_document": {
        "authority_replica_id": hex::encode(&[9;32]), "sequencer_public_key": text("sequencer_public_key_hex"),
        "batch_evidence": {"header_hex":text("header_hex"),"header_signature":text("header_signature_hex"),"receipt_proof_hex":hex::encode(&proof)}
    }});
    write(
        &state.join(format!("{activity}.json")),
        &must(serde_json::to_vec(&record)),
    );
}

fn launch(root: PathBuf) -> (Server, std::net::SocketAddr) {
    launch_at(root, "http://127.0.0.1:1")
}

fn launch_at(root: PathBuf, chain_rpc_url: &str) -> (Server, std::net::SocketAddr) {
    let state = root.join("state");
    let fixture = signed_fixture();
    let text = |key: &str| {
        fixture[key]
            .as_str()
            .unwrap_or_else(|| panic!("fixture field {key}"))
    };
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
            .env("LAYERX_AUTHORITY_REPLICA_URL", "http://127.0.0.1:1")
            .env(
                "LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE",
                root.join("replica.token"),
            )
            .env("LAYERX_AUTHORITY_REPLICA_ID", hex::encode(&[9; 32]))
            .env("LAYERX_AUTHORITY_CHAIN_RPC_URL", chain_rpc_url)
            .env(
                "LAYERX_AUTHORITY_RECEIPT_KEYS_FILE",
                root.join("receipt-keys.json"),
            )
            .env("LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID", "7332")
            .env("LAYERX_AUTHORITY_NETWORK_ID", "human-authority-test")
            .env("LAYERX_AUTHORITY_SEQUENCER_ID", text("sequencer_id_hex"))
            .env(
                "LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY",
                text("sequencer_public_key_hex"),
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
            "authority exited; see {}/server.log",
            server.root.display()
        );
        if TcpStream::connect(address).is_ok() {
            break;
        }
        assert!(Instant::now() < deadline, "authority startup timed out");
        std::thread::sleep(Duration::from_millis(20));
    }
    (server, address)
}

#[test]
fn real_tls_client_and_durable_refusals() {
    if std::env::var_os("LAYERX_HUMAN_TLS_CHILD").is_some() {
        drive_client();
        return;
    }
    let root = prepare_root();
    seed(&root);
    let (server, address) = launch(root);
    let result = must(
        Command::new(must(std::env::current_exe()))
            .args([
                "--exact",
                "real_tls_client_and_durable_refusals",
                "--nocapture",
            ])
            .env("LAYERX_HUMAN_TLS_CHILD", address.to_string())
            .env("LAYERX_HUMAN_TLS_ROOT", &server.root)
            .env("SSL_CERT_FILE", server.root.join("ca.pem"))
            .env("SSL_CERT_DIR", &server.root)
            .env_remove("HTTPS_PROXY")
            .env_remove("HTTP_PROXY")
            .env_remove("ALL_PROXY")
            .output(),
    );
    write(
        &server.root.join("client.log"),
        &[result.stdout, result.stderr].concat(),
    );
    assert!(
        result.status.success(),
        "real client failed; see {}/client.log\n{}\nserver.log (last twenty lines):\n{}",
        server.root.display(),
        must(fs::read_to_string(server.root.join("client.log"))),
        server_tail(&server.root)
    );
}

fn wire_status(root: &Path, path: &str) -> (u16, String) {
    wire_status_at(
        root,
        &must(std::env::var("LAYERX_HUMAN_TLS_CHILD")),
        path,
        "human-test-token-0000000000000000000000",
    )
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

fn server_tail(root: &Path) -> String {
    let log = fs::read_to_string(root.join("server.log"))
        .unwrap_or_else(|error| format!("server.log unavailable: {error}"));
    let lines = log.lines().collect::<Vec<_>>();
    lines[lines.len().saturating_sub(20)..].join("\n")
}

fn assert_wire_status(root: &Path, response: (u16, String), expected: u16) {
    assert_eq!(
        response.0,
        expected,
        "body: {}\nserver.log (last twenty lines):\n{}",
        response.1,
        server_tail(root)
    );
}

fn client_result<T, E: std::fmt::Debug>(
    root: &Path,
    address: &str,
    path: &str,
    result: Result<T, E>,
) -> T {
    result.unwrap_or_else(|error| {
        let (status, body) = wire_status_at(root, address, path,
            "human-test-token-0000000000000000000000");
        panic!("client refused {path}: {error:?}; status: {status}; body: {body}\nserver.log (last twenty lines):\n{}", server_tail(root));
    })
}

#[test]
fn real_server_wire_statuses_remain_distinct() {
    let root = prepare_root();
    seed(&root);
    let (server, address) = launch(root);
    let address = address.to_string();
    let status = |path: &str| {
        wire_status_at(
            &server.root,
            &address,
            path,
            "human-test-token-0000000000000000000000",
        )
    };
    let fixture = signed_fixture();
    let receipt = must(hex::decode(
        fixture["receipt_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("receipt")),
    ));
    let activity = hex::encode(&must(receipt_locator(&receipt)).activity_id);
    let base = "tenant=tenant%20space&principal=principal";
    assert_wire_status(
        &server.root,
        status(&format!("/v1/agent/registry?{base}")),
        200,
    );
    assert_wire_status(
        &server.root,
        status(&format!(
            "/v1/agent/authorized-batch?{base}&activity_id={activity}"
        )),
        200,
    );
    assert_wire_status(
        &server.root,
        status(&format!("/v1/agent/core-clock?{base}")),
        503,
    );
    assert_wire_status(
        &server.root,
        status(&format!("/v1/agent/balance-context?{base}")),
        503,
    );
    assert_wire_status(
        &server.root,
        status(&format!(
            "/v1/agent/identity?{base}&did=did%3Alayerx%3Atest"
        )),
        503,
    );
    assert_wire_status(&server.root,status(&format!("/v1/agent/capability-scope?{base}&did=did%3Alayerx%3Atest&authority={}&action_key={}&capability_id={}", hex::encode(&[5;32]), hex::encode(&[6;32]), hex::encode(&[7;32]))), 403);
    assert_wire_status(
        &server.root,
        status(&format!(
            "/v1/agent/budget-state?{base}&budget_id={}",
            hex::encode(&[4; 32])
        )),
        503,
    );
    assert_wire_status(
        &server.root,
        status(&format!(
            "/v1/agent/budget-proof?{base}&budget_id={}",
            hex::encode(&[4; 32])
        )),
        503,
    );
    assert_wire_status(
        &server.root,
        status(&format!(
            "/v1/agent/key-policy?{base}&did=did%3Alayerx%3Atest&recovery=false"
        )),
        503,
    );
    assert_wire_status(
        &server.root,
        status("/v1/agent/registry?tenant=other&principal=principal"),
        403,
    );
    assert_wire_status(
        &server.root,
        wire_status_at(
            &server.root,
            &address,
            &format!("/v1/agent/registry?{base}"),
            "service-test-token-00000000000000000000",
        ),
        401,
    );
    must(fs::rename(
        server.root.join("policy.json"),
        server.root.join("policy.hidden"),
    ));
    assert_wire_status(
        &server.root,
        status(&format!("/v1/agent/registry?{base}")),
        503,
    );
    must(fs::rename(
        server.root.join("policy.hidden"),
        server.root.join("policy.json"),
    ));
    assert_wire_status(
        &server.root,
        status(&format!("/v1/agent/registry?{base}")),
        200,
    );
    write(&server.root.join("policy.json"), br#"{"principals":[]}"#);
    assert_wire_status(
        &server.root,
        status(&format!("/v1/agent/registry?{base}")),
        503,
    );
}

fn drive_client() {
    use layerx_agentd::human::HumanOperationError;
    let endpoint = format!("https://{}", must(std::env::var("LAYERX_HUMAN_TLS_CHILD")));
    let root = PathBuf::from(must(std::env::var("LAYERX_HUMAN_TLS_ROOT")));
    let mut client = must(RemoteHumanAuthority::connect(
        &endpoint,
        "human-test-token-0000000000000000000000".to_owned(),
        Duration::from_secs(5),
        1_048_576,
        &must(fs::read(root.join("ca.der"))),
    ));
    let peer = HumanPeer {
        subject: None,
        uid: 1,
        tenant: "tenant space".to_owned(),
        principal: "principal".to_owned(),
    };
    client_result(
        &root,
        &must(std::env::var("LAYERX_HUMAN_TLS_CHILD")),
        "/v1/agent/registry?tenant=tenant%20space&principal=principal",
        client.registry(&peer),
    );
    assert_wire_status(
        &root,
        wire_status(
            &root,
            "/v1/agent/core-clock?tenant=tenant%20space&principal=principal",
        ),
        503,
    );
    assert_wire_status(
        &root,
        wire_status(&root, "/v1/agent/registry?tenant=other&principal=principal"),
        403,
    );
    assert_wire_status(&root,wire_status(&root, &format!("/v1/agent/capability-scope?tenant=tenant%20space&principal=principal&did=did%3Alayerx%3Atest&authority={}&action_key={}&capability_id={}", hex::encode(&[5;32]), hex::encode(&[6;32]), hex::encode(&[7;32]))), 403);
    let fixture = signed_fixture();
    let receipt = must(hex::decode(
        fixture["receipt_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("receipt")),
    ));
    let locator = must(receipt_locator(&receipt));
    assert_eq!(
        client_result(&root, &must(std::env::var("LAYERX_HUMAN_TLS_CHILD")),
            &format!("/v1/agent/authorized-batch?tenant=tenant%20space&principal=principal&activity_id={}", hex::encode(&locator.activity_id)),
            client.authorized_batch(&peer, locator.activity_id)).batch_id(),
        locator.batch_id
    );
    assert!(matches!(
        client.lease_attestation(&peer),
        Err(HumanOperationError::Refused)
    ));
    assert!(matches!(
        client.balance_context(&peer),
        Err(HumanOperationError::Refused)
    ));
    let did = must(Did::new(b"did:layerx:test"));
    assert!(client.core_identity(&peer, &did).is_err());
    assert!(matches!(
        client.capability_scope(&peer, &did, [5; 32], [6; 32], [7; 32]),
        Err(HumanOperationError::Refused)
    ));
    assert!(matches!(
        client.budget_state(&peer, [4; 32]),
        Err(HumanOperationError::Refused)
    ));
    assert!(matches!(
        client.key_rotation_policy(&peer, &did, false),
        Err(HumanOperationError::Refused)
    ));
    let other = HumanPeer {
        subject: None,
        tenant: "other".to_owned(),
        ..peer.clone()
    };
    assert!(client.registry(&other).is_err());
    must(fs::rename(
        root.join("policy.json"),
        root.join("policy.hidden"),
    ));
    assert!(client.registry(&peer).is_err());
    assert_wire_status(
        &root,
        wire_status(
            &root,
            "/v1/agent/registry?tenant=tenant%20space&principal=principal",
        ),
        503,
    );
    must(fs::rename(
        root.join("policy.hidden"),
        root.join("policy.json"),
    ));
    client_result(
        &root,
        &must(std::env::var("LAYERX_HUMAN_TLS_CHILD")),
        "/v1/agent/registry?tenant=tenant%20space&principal=principal",
        client.registry(&peer),
    );
    write(&root.join("policy.json"), br#"{"principals":[]}"#);
    assert!(client.registry(&peer).is_err());
}

#[test]
fn real_client_balance_context_uses_registry_and_verified_header() {
    let root = prepare_root();
    seed(&root);
    let mut policy: Value = must(serde_json::from_slice(&must(fs::read(
        root.join("policy.json"),
    ))));
    policy["principals"][0]["maximum_age_seconds"] = json!(315_360_000_u64);
    write(
        &root.join("policy.json"),
        &must(serde_json::to_vec(&policy)),
    );
    let (server, address) = launch(root);
    let mut client = must(RemoteHumanAuthority::connect(
        &format!("https://{address}"),
        "human-test-token-0000000000000000000000".to_owned(),
        Duration::from_secs(5),
        1_048_576,
        &must(fs::read(server.root.join("ca.der"))),
    ));
    let peer = HumanPeer {
        subject: None,
        uid: 1,
        tenant: "tenant space".to_owned(),
        principal: "principal".to_owned(),
    };
    let (account, asset, currency, observed_at, age, maximum_age, authorization) = client_result(
        &server.root,
        &address.to_string(),
        "/v1/agent/balance-context?tenant=tenant%20space&principal=principal",
        client.balance_context(&peer),
    );
    assert_eq!(account, [1; 32]);
    assert_eq!(asset, [2; 32]);
    assert_eq!(currency, "USD");
    assert!(age <= maximum_age);
    let fixture = signed_fixture();
    let header = must(layerx_wire::receipt::decode_batch_header(&must(
        hex::decode(
            fixture["header_hex"]
                .as_str()
                .unwrap_or_else(|| panic!("header")),
        ),
    )));
    assert_eq!(observed_at, header.timestamp_ms().to_string());
    assert_eq!(
        hex::encode(&authorization.public_key()),
        fixture["sequencer_public_key_hex"]
    );
    write(
        &server.root.join("modules.json"),
        br#"{"modules":[{"module":9,"ordinals":[1,7]}]}"#,
    );
    assert!(client.registry(&peer).is_err());
    assert!(client.balance_context(&peer).is_err());
}

#[test]
fn identity_evidence_binds_to_the_committed_anchor_checkpoint() {
    human_tls_retired_anchor();
}

#[test]
fn real_node_budget_state_and_v2_export_cross_tls_without_lni() {
    human_budget_corpus_optional();
}
