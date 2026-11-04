use ed25519_dalek::{Signer, SigningKey};
use layerx_platform_gateway::http::{self, Endpoint};
use layerx_platform_gateway::runtime::Server;
use layerx_platform_gateway::store::{RedisEndpoint, RedisStore};
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::ServerConfig;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::ffi::OsString;
use std::fs;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use tempfile::TempDir;
use zeroize::Zeroizing;

#[path = "support/repair_22_31_202.rs"]
mod support;
use support::{fixture, hex, json_response, matches_schema, unhex, Peer, RedisMemory};

static STATE_CLOCK: AtomicU64 = AtomicU64::new(0);
fn capture_clock() -> Result<u64, String> {
    Ok(STATE_CLOCK.load(Ordering::Acquire))
}

struct Environment(Vec<(OsString, OsString)>);
impl Environment {
    fn clear() -> Self {
        let previous: Vec<_> = std::env::vars_os()
            .filter(|(name, _)| name.to_string_lossy().starts_with("LAYERX_GATEWAY_"))
            .collect();
        for (name, _) in &previous {
            std::env::remove_var(name);
        }
        Self(previous)
    }
    fn set(name: &str, value: impl AsRef<std::ffi::OsStr>) {
        std::env::set_var(name, value);
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        for (name, _) in std::env::vars_os()
            .filter(|(name, _)| name.to_string_lossy().starts_with("LAYERX_GATEWAY_"))
        {
            std::env::remove_var(name);
        }
        for (name, value) in &self.0 {
            std::env::set_var(name, value);
        }
    }
}

struct Fixture {
    directory: TempDir,
    ca: native_tls::Certificate,
    redis: Peer,
    memory: Arc<Mutex<RedisMemory>>,
    identity: Peer,
    faucet: Peer,
    node: Peer,
    fallback: Peer,
    consensus: Peer,
    indexer: Peer,
    store: Arc<layerx_indexer::store::Store>,
    committed: Arc<AtomicBool>,
    funded: Arc<AtomicBool>,
    stale: Arc<AtomicBool>,
    identity_calls: Arc<Mutex<Vec<String>>>,
    node_calls: Arc<Mutex<Vec<(String, String)>>>,
    faucet_calls: Arc<AtomicU64>,
}
impl Fixture {
    fn start() -> Self {
        let directory = tempfile::tempdir().expect("fixture directory");
        let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()])
            .expect("loopback certificate");
        fs::write(directory.path().join("server.der"), certificate.cert.der()).expect("CA file");
        let ca =
            native_tls::Certificate::from_der(certificate.cert.der()).expect("trusted fixture CA");
        let tls = Arc::new(
            ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_safe_default_protocol_versions()
                .expect("fixture TLS protocol versions")
                .with_no_client_auth()
                .with_single_cert(
                    vec![certificate.cert.der().clone()],
                    PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
                        certificate.signing_key.serialize_der(),
                    )),
                )
                .expect("fixture TLS"),
        );
        let memory = Arc::new(Mutex::new(RedisMemory::default()));
        let redis = RedisMemory::serve(Arc::clone(&memory), Arc::clone(&tls));
        let identity_calls = Arc::new(Mutex::new(Vec::new()));
        let calls = Arc::clone(&identity_calls);
        let identity = Peer::https(Arc::clone(&tls), move |request| {
            assert_eq!(request.method, "POST");
            let recording = fixture("identity");
            let route = recording["routes"]
                .get(&request.path)
                .unwrap_or_else(|| panic!("unrecorded identity path {}", request.path));
            assert_eq!(
                request.headers.get("authorization").map(String::as_str),
                route["authorization"].as_str()
            );
            assert_eq!(
                serde_json::from_slice::<Value>(&request.body).expect("identity request JSON"),
                route["request"]
            );
            calls
                .lock()
                .expect("identity calls")
                .push(request.path.clone());
            json_response(
                route["status"].as_u64().expect("status") as u16,
                &route["body"],
            )
        });
        let funded = Arc::new(AtomicBool::new(false));
        let is_funded = Arc::clone(&funded);
        let faucet_calls = Arc::new(AtomicU64::new(0));
        let claims = Arc::clone(&faucet_calls);
        let redis_url = format!("localhost:{}", redis.address.port());
        let faucet_ca = ca.clone();
        let faucet = Peer::https(Arc::clone(&tls), move |request| {
            let recording = fixture("faucet");
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, recording["path"]);
            assert_eq!(
                request.headers.get("authorization").map(String::as_str),
                recording["authorization"].as_str()
            );
            assert_eq!(
                serde_json::from_slice::<Value>(&request.body).expect("claim JSON"),
                recording["request"]
            );
            let key = request
                .headers
                .get("idempotency-key")
                .expect("claim idempotency");
            let identity = fixture("identity");
            let mut hash = Sha256::new();
            for part in [
                b"layerx-faucet-claim-v1".as_slice(),
                identity["sub"].as_str().expect("sub").as_bytes(),
                b"abababababababababababababababab",
                b"starter-credit",
            ] {
                hash.update((part.len() as u64).to_be_bytes());
                hash.update(part);
            }
            assert_eq!(*key, hex(&hash.finalize()));
            // The peer's recorded claim is retained through the same TLS/RESP
            // transport, independently of gateway key and watermark records.
            persist_claim(&faucet_ca, &redis_url, key, &request.body);
            claims.fetch_add(1, Ordering::AcqRel);
            let phase = if is_funded.load(Ordering::Acquire) {
                "funded"
            } else {
                "pending"
            };
            json_response(
                recording[phase]["status"].as_u64().expect("claim status") as u16,
                &recording[phase]["body"],
            )
        });
        let committed = Arc::new(AtomicBool::new(false));
        let node_calls = Arc::new(Mutex::new(Vec::new()));
        let node = node_peer(
            Arc::clone(&tls),
            Arc::clone(&committed),
            Arc::clone(&node_calls),
        );
        let fallback = node_peer(
            Arc::clone(&tls),
            Arc::clone(&committed),
            Arc::clone(&node_calls),
        );
        let stale = Arc::new(AtomicBool::new(false));
        let stale_proof = Arc::clone(&stale);
        let consensus = Peer::https(Arc::clone(&tls), move |request| {
            assert_eq!((&*request.method, &*request.path), ("POST", "/"));
            let call: Value = serde_json::from_slice(&request.body).expect("consensus request");
            let recording = fixture("consensus");
            let result = match call["method"].as_str().expect("method") {
                "status" => {
                    assert_eq!(call["params"], json!({}));
                    recording["status"].clone()
                }
                "commit" => {
                    assert_eq!(
                        call["params"],
                        json!({"height":recording["status"]["sync_info"]["latest_block_height"]})
                    );
                    recording[if stale_proof.load(Ordering::Acquire) {
                        "stale"
                    } else {
                        "current"
                    }]
                    .clone()
                }
                other => panic!("unrecorded consensus method {other}"),
            };
            json_response(
                200,
                &json!({"jsonrpc":"2.0","id":call["id"],"result":result}),
            )
        });
        let store =
            Arc::new(layerx_indexer::store::Store::open_in_memory().expect("real indexer store"));
        let indexer_store = Arc::clone(&store);
        let indexer = Peer::start(move |mut tcp| {
            let request = http::read_request(&mut tcp, 16384).expect("indexer HTTP request");
            let answer = layerx_indexer::api::route(&indexer_store, &request.method, &request.path);
            layerx_platform_internal::http::write_response(&mut tcp, &answer)
                .expect("real indexer HTTP response");
        });
        Self {
            directory,
            ca,
            redis,
            memory,
            identity,
            faucet,
            node,
            fallback,
            consensus,
            indexer,
            store,
            committed,
            funded,
            stale,
            identity_calls,
            node_calls,
            faucet_calls,
        }
    }
    fn secret(&self, name: &str, value: &str) -> String {
        let path = self.directory.path().join(name);
        fs::write(&path, value).expect("fixture configuration");
        path.to_str().expect("UTF-8 path").into()
    }
    fn configure(&self) {
        Environment::set("LAYERX_GATEWAY_LISTEN", "127.0.0.1:0");
        Environment::set("LAYERX_GATEWAY_LISTENER", "plain");
        Environment::set(
            "LAYERX_GATEWAY_OUTBOUND_CA_DER",
            self.directory.path().join("server.der"),
        );
        Environment::set("LAYERX_GATEWAY_NETWORK_ID", "chain-reads");
        Environment::set(
            "LAYERX_GATEWAY_LXP_WIRE_VERSION",
            layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
        );
        Environment::set("LAYERX_GATEWAY_PROTOCOL_NETWORK_ID", "7");
        Environment::set(
            "LAYERX_GATEWAY_REDIS_URL",
            format!("rediss://localhost:{}", self.redis.address.port()),
        );
        for (variable, name, value) in [
            (
                "LAYERX_GATEWAY_REDIS_USERNAME_FILE",
                "redis-user",
                "gateway",
            ),
            (
                "LAYERX_GATEWAY_REDIS_PASSWORD_FILE",
                "redis-password",
                "gateway-secret",
            ),
            (
                "LAYERX_GATEWAY_IDENTITY_TOKEN_FILE",
                "identity-token",
                "identity-token",
            ),
            (
                "LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE",
                "provisioning-token",
                "provisioning-token",
            ),
            (
                "LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE",
                "faucet-token",
                "faucet-token",
            ),
        ] {
            Environment::set(variable, self.secret(name, value));
        }
        Environment::set(
            "LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE",
            self.secret("key-provisioning-key", &"12".repeat(32)),
        );
        Environment::set("LAYERX_GATEWAY_IDENTITY_URL", self.identity.url());
        Environment::set("LAYERX_GATEWAY_FAUCET_URL", self.faucet.url());
        Environment::set("LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT", "1000000");
        Environment::set("LAYERX_GATEWAY_CONSENSUS_RPC_URL", self.consensus.url());
        Environment::set(
            "LAYERX_GATEWAY_PAXEER_RPC_URLS",
            json!([self.node.url(), self.fallback.url()]).to_string(),
        );
        Environment::set(
            "LAYERX_GATEWAY_INDEXER_URL",
            format!("http://{}", self.indexer.address),
        );
        let authority = fixture("consensus")["authority"].clone();
        Environment::set(
            "LAYERX_GATEWAY_STATE_VALIDATORS_FILE",
            self.secret("state-authority", &authority.to_string()),
        );
        let receipt = fixture("node")["settlement"]["receipt"].clone();
        let keys = json!([{
            "keyId":layerx_platform_internal::base64::encode(&unhex(receipt["keyId"].as_str().expect("receipt key ID"))),
            "publicKey":layerx_platform_internal::base64::encode(&unhex(receipt["signingKey"].as_str().expect("receipt key"))),
            "firstHeight":1,"lastHeight":u64::MAX
        }]);
        Environment::set(
            "LAYERX_GATEWAY_RECEIPT_KEYS_FILE",
            self.secret("receipt-keys", &keys.to_string()),
        );
    }
    fn store(&self) -> RedisStore {
        RedisStore::new(
            RedisEndpoint::parse(&format!("rediss://localhost:{}", self.redis.address.port()))
                .expect("Redis URL"),
            self.ca.clone(),
            Zeroizing::new("gateway".into()),
            Zeroizing::new("gateway-secret".into()),
        )
    }
}

fn persist_claim(ca: &native_tls::Certificate, address: &str, key: &str, body: &[u8]) {
    use std::io::{Read, Write};
    let connector = native_tls::TlsConnector::builder()
        .add_root_certificate(ca.clone())
        .build()
        .expect("claim TLS client");
    let tcp = std::net::TcpStream::connect(address).expect("claim Redis TCP");
    tcp.set_read_timeout(Some(std::time::Duration::from_secs(3)))
        .expect("claim timeout");
    let mut stream = connector
        .connect("localhost", tcp)
        .expect("claim Redis TLS");
    for args in [
        vec![
            "AUTH".to_string(),
            "gateway".into(),
            "gateway-secret".into(),
        ],
        vec![
            "SET".into(),
            format!("faucet:claim:{key}"),
            String::from_utf8(body.to_vec()).expect("claim UTF-8"),
        ],
    ] {
        write!(stream, "*{}\r\n", args.len()).expect("RESP array");
        for arg in args {
            write!(stream, "${}\r\n{arg}\r\n", arg.len()).expect("RESP argument");
        }
        stream.flush().expect("claim write");
        let mut answer = [0; 5];
        stream.read_exact(&mut answer).expect("claim Redis reply");
        assert_eq!(&answer, b"+OK\r\n");
    }
}

fn node_peer(
    tls: Arc<ServerConfig>,
    committed: Arc<AtomicBool>,
    calls: Arc<Mutex<Vec<(String, String)>>>,
) -> Peer {
    Peer::https(tls, move |request| {
        assert_eq!(request.method, "POST");
        if !matches!(request.path.as_str(), "/" | "/chain/125") {
            return json_response(
                404,
                &json!({"error":"node accepts root or explicitly configured prefix only"}),
            );
        }
        let call: Value = serde_json::from_slice(&request.body).expect("node request JSON");
        let method = call["method"].as_str().expect("node method");
        calls
            .lock()
            .expect("node calls")
            .push((request.path.clone(), method.into()));
        let node = fixture("node");
        let blocks = fixture("blocks");
        let result = match method {
            "eth_blockNumber" => {
                assert_eq!(call["params"], json!([]));
                json!("0xb")
            }
            "eth_chainId" => json!("0x7d"),
            "eth_call" => {
                let views = fixture("views");
                let view = views["calls"]
                    .as_array()
                    .expect("view transcripts")
                    .iter()
                    .find(|v| {
                        call["params"][0]["to"] == v["to"]
                            && call["params"][0]["data"] == v["data"]
                            && call["params"][1] == v["block"]
                    })
                    .unwrap_or_else(|| panic!("unrecorded node view {}", call["params"]));
                view["result"].clone()
            }
            "eth_sendRawTransaction" => {
                assert_eq!(
                    call["params"],
                    json!([node["settlement"]["raw_transaction"]])
                );
                node["settlement"]["receipt"]["txHash"].clone()
            }
            "pax_sequencerReceipt" => {
                assert_eq!(
                    call["params"],
                    json!([node["settlement"]["receipt"]["txHash"]])
                );
                let mut receipt = node["settlement"]["receipt"].clone();
                if !committed.load(Ordering::Acquire) {
                    receipt["rung"] = json!("receipted");
                    let object = receipt.as_object_mut().expect("receipt");
                    object.remove("blockHash");
                    object.remove("blockNumber");
                }
                receipt
            }
            "eth_getTransactionReceipt" => {
                let hash = call["params"][0].as_str().expect("receipt hash");
                if hash
                    == node["settlement"]["receipt"]["txHash"]
                        .as_str()
                        .expect("payment hash")
                {
                    assert!(committed.load(Ordering::Acquire));
                    let receipt = &node["settlement"]["receipt"];
                    json!({"transactionHash":receipt["txHash"],"blockHash":receipt["blockHash"],"blockNumber":receipt["blockNumber"],"status":receipt["status"],"gasUsed":receipt["gasUsed"],"cumulativeGasUsed":receipt["cumulativeGasUsed"],"effectiveGasPrice":receipt["effectiveGasPrice"],"logs":receipt["logs"]})
                } else {
                    blocks["receipts"]
                        .get(hash)
                        .unwrap_or_else(|| panic!("unrecorded transaction {hash}"))
                        .clone()
                }
            }
            "eth_getBlockByNumber" => {
                assert_eq!(call["params"], json!(["0x2", true]));
                blocks["canonical"]["blocks"]["2"].clone()
            }
            other => panic!("unrecorded node method {other}"),
        };
        json_response(
            200,
            &json!({"jsonrpc":"2.0","id":call["id"],"result":result}),
        )
    })
}

struct Gateway(Peer);
impl Gateway {
    fn start() -> Self {
        let server = Server::from_environment()
            .expect("production chain-only configuration")
            .with_state_clock(capture_clock);
        Self(Peer::start(move |tcp| {
            server
                .serve_connection(tcp)
                .expect("real gateway HTTP server")
        }))
    }
    fn restart(self) -> Self {
        drop(self);
        Self::start()
    }
    fn request(
        &self,
        method: &str,
        path: &str,
        headers: &[(&str, &str)],
        body: &Value,
    ) -> (u16, Value) {
        support::request(self.0.address, method, path, headers, body)
    }
    fn rpc(&self, method: &str, params: Value, authorization: Option<&str>) -> Value {
        let headers = authorization
            .map(|value| vec![("Authorization", value)])
            .unwrap_or_default();
        let (status, answer) = self.request(
            "POST",
            "/",
            &headers,
            &json!({"jsonrpc":"2.0","id":"repair","method":method,"params":params}),
        );
        assert_eq!(status, 200, "{answer}");
        assert_eq!(answer["id"], "repair");
        answer
    }
}

fn schema_params(method: &str, arguments: &[(&str, Value)]) -> Value {
    let contract: Value =
        serde_json::from_str(include_str!("../openrpc.json")).expect("OpenRPC contract");
    let declaration = contract["methods"]
        .as_array()
        .expect("methods")
        .iter()
        .find(|entry| entry["name"] == method)
        .expect("declared method");
    assert_eq!(declaration["paramStructure"], "by-position");
    let mut supplied: BTreeMap<_, _> = arguments.iter().cloned().collect();
    assert_eq!(supplied.len(), arguments.len(), "duplicate parameter name");
    let mut params = Vec::new();
    let mut omitted = false;
    for parameter in declaration["params"].as_array().expect("parameters") {
        let name = parameter["name"].as_str().expect("parameter name");
        if let Some(value) = supplied.remove(name) {
            assert!(!omitted, "positional parameter {name} follows an omission");
            assert!(
                matches_schema(&value, &parameter["schema"]),
                "{method} parameter {name} does not match its schema"
            );
            params.push(value);
        } else {
            assert_ne!(
                parameter["required"], true,
                "missing {method} parameter {name}"
            );
            omitted = true;
        }
    }
    assert!(
        supplied.is_empty(),
        "undeclared {method} parameters: {supplied:?}"
    );
    Value::Array(params)
}

fn identity_config_coupling(fixture: &Fixture, gateway: &Gateway) -> (String, String) {
    let recording = support::fixture("identity");
    let private = SigningKey::from_bytes(
        &unhex(recording["signer_seed"].as_str().expect("fixture signer"))
            .try_into()
            .expect("32-byte seed"),
    );
    let public = hex(&private.verifying_key().to_bytes());
    assert_eq!(public, recording["signer"]);
    let mut binding = Sha256::new();
    for part in [
        b"layerx-register-binding-v1".as_slice(),
        b"beta",
        &private.verifying_key().to_bytes(),
    ] {
        binding.update((part.len() as u64).to_be_bytes());
        binding.update(part);
    }
    let signature = hex(&private.sign(&binding.finalize()).to_bytes());
    let registration = gateway.rpc(
        "px_register",
        schema_params(
            "px_register",
            &[
                ("signer_public_key", json!(public)),
                ("registration_signature", json!(signature)),
                (
                    "email_or_verification_token",
                    recording["routes"]["/v1/signup"]["request"]["email"].clone(),
                ),
            ],
        ),
        None,
    );
    assert!(registration.get("error").is_none(), "{registration}");
    assert_eq!(registration["result"]["pending"], true, "{registration}");
    let registered = gateway.rpc(
        "px_register",
        schema_params(
            "px_register",
            &[
                ("signer_public_key", json!(public)),
                ("registration_signature", json!(signature)),
                (
                    "email_or_verification_token",
                    recording["verification_token"].clone(),
                ),
            ],
        ),
        None,
    );
    assert!(registered.get("error").is_none(), "{registered}");
    assert_eq!(
        registered["result"], recording["routes"]["/v1/signup/verify"]["body"],
        "{registered}"
    );
    let authorization = format!(
        "Bearer {}",
        registered["result"]["token"]
            .as_str()
            .expect("identity session")
    );
    let issue = json!({"signer_public_key":public,"scopes":["receipt:read","state:read"],"quota_requests":1000,"quota_window_seconds":60});
    let (status, key) = gateway.request(
        "POST",
        "/v1/keys",
        &[
            ("Authorization", &authorization),
            ("Idempotency-Key", "repair-key"),
        ],
        &issue,
    );
    assert_eq!(status, 201, "{key}");
    let key_id = key["key"]["id"].as_str().expect("issued key");
    let old = format!(
        "LayerX-Key {key_id}:{}",
        key["key"]["secret"].as_str().expect("key secret")
    );
    let (status, list) = gateway.request(
        "GET",
        "/v1/keys",
        &[("Authorization", &authorization)],
        &Value::Null,
    );
    assert_eq!(status, 200, "{list}");
    assert_eq!(list["keys"].as_array().expect("keys").len(), 1);
    let did = format!("did:layerx:{public}");
    let claim = gateway.rpc(
        "px_requestFunds",
        json!([did, public]),
        Some(&authorization),
    );
    assert_eq!(
        claim["error"]["data"]["code"], "faucet_claim_pending",
        "{claim}"
    );
    assert_eq!(
        claim["error"]["data"]["upstream"]["state"], "still_checking",
        "{claim}"
    );
    assert!(
        fixture
            .memory
            .lock()
            .expect("claim store")
            .has_faucet_claim(),
        "faucet claim must persist across the Redis transport"
    );
    fixture.funded.store(true, Ordering::Release);
    let funded = gateway.rpc(
        "px_requestFunds",
        json!([did, public]),
        Some(&authorization),
    );
    assert_eq!(funded["result"]["funded"], true, "{funded}");
    assert_eq!(funded["result"]["amount"], "1000000");
    assert_eq!(
        funded["result"]["starter_credit"]["grant_id"],
        "ab".repeat(16)
    );
    assert_eq!(fixture.faucet_calls.load(Ordering::Acquire), 2);
    let (status, rotated) = gateway.request(
        "POST",
        &format!("/v1/keys/{key_id}/rotate"),
        &[
            ("Authorization", &authorization),
            ("Idempotency-Key", "repair-rotation"),
        ],
        &json!({}),
    );
    assert_eq!(status, 201, "{rotated}");
    let id = rotated["key"]["id"].as_str().expect("rotated key");
    let credential = format!(
        "LayerX-Key {id}:{}",
        rotated["key"]["secret"].as_str().expect("rotated secret")
    );
    assert_ne!(credential, old);
    assert_eq!(
        gateway
            .request(
                "GET",
                "/internal/v1/principal",
                &[("Authorization", &old)],
                &Value::Null
            )
            .0,
        401,
        "rotated credential must be revoked"
    );
    assert_eq!(
        gateway
            .request(
                "GET",
                "/internal/v1/principal",
                &[("Authorization", &credential)],
                &Value::Null
            )
            .0,
        200
    );
    for expected in [
        "/v1/signup",
        "/v1/signup/verify",
        "/v1/sessions/introspect",
        "/v1/starter-credits",
        "/v1/starter-credits/settle",
    ] {
        assert!(
            fixture
                .identity_calls
                .lock()
                .expect("identity calls")
                .iter()
                .any(|path| path == expected),
            "real identity transport did not carry {expected}"
        );
    }
    (credential, old)
}

fn merchant_request(receipt: &[u8]) -> Value {
    let principal = "gateway-repair-merchant";
    let request_digest = [0x73; 32];
    let mut hash = Sha256::new();
    hash.update(b"LayerX/middleware/x402/idempotency\0");
    hash.update(principal);
    hash.update(request_digest);
    json!({"principal":principal,"payload":{"x402Version":2,"accepted":{},"payload":{
        "receipt":layerx_platform_internal::base64::encode(receipt),
        "receiptDigest":hex(&layerx_proof::merkle::leaf_hash(receipt).expect("canonical receipt digest")),
        "verificationLevel":"sequencer-signed"
    }},"requirements":{},"idempotencyKey":hex(&hash.finalize()),"requestDigest":hex(&request_digest)})
}

fn node_root_and_settlement_component_gate(fixture: &Fixture, gateway: &Gateway, credential: &str) {
    for (configured, expected) in [
        ("https://node.example", "/"),
        ("https://node.example/", "/"),
        ("https://node.example/chain/125", "/chain/125"),
        ("https://node.example/chain/125/", "/chain/125"),
    ] {
        let endpoint = Endpoint::parse(configured)
            .expect("node endpoint")
            .chain_rpc();
        assert_eq!(endpoint.base_path, expected);
        assert_eq!(endpoint.chain_rpc().base_path, expected);
    }
    assert_eq!(
        gateway.rpc("eth_blockNumber", json!([]), None)["result"],
        "0xb"
    );
    let capabilities = gateway.rpc("px_getCapabilities", json!([]), None);
    for surface in [
        "exchange",
        "bridge",
        "launchpad",
        "ledger",
        "pay",
        "auth",
        "programs",
    ] {
        assert_eq!(
            capabilities["result"][surface], true,
            "{surface}: {capabilities}"
        );
    }
    let node = support::fixture("node");
    let sent = gateway.rpc(
        "eth_sendRawTransaction",
        json!([node["settlement"]["raw_transaction"]]),
        None,
    );
    assert_eq!(
        sent["result"], node["settlement"]["receipt"]["txHash"],
        "{sent}"
    );
    let pending = gateway.rpc("pax_sequencerReceipt", json!([sent["result"]]), None);
    assert_eq!(pending["result"]["rung"], "receipted");
    let canonical = unhex(
        pending["result"]["receipt"]
            .as_str()
            .expect("node canonical receipt"),
    );
    let merchant = merchant_request(&canonical);
    let (status, answer) = gateway.request(
        "POST",
        "/v1/settle",
        &[("Authorization", credential)],
        &merchant,
    );
    assert_eq!(status, 202, "{answer}");
    assert_eq!(answer["result"]["state"], "pending", "{answer}");
    fixture.committed.store(true, Ordering::Release);
    let committed = gateway.rpc("pax_sequencerReceipt", json!([sent["result"]]), None);
    assert_eq!(committed["result"], node["settlement"]["receipt"]);
    let (status, answer) = gateway.request(
        "POST",
        "/v1/settle",
        &[("Authorization", credential)],
        &merchant,
    );
    assert_eq!(status, 200, "{answer}");
    assert_eq!(answer["result"]["state"], "settled", "{answer}");
    assert_eq!(
        answer["result"]["receipt_base64"],
        layerx_platform_internal::base64::encode(&canonical)
    );
    let mut altered = canonical;
    let last = altered.len() - 1;
    altered[last] ^= 1;
    let (status, answer) = gateway.request(
        "POST",
        "/v1/settle",
        &[("Authorization", credential)],
        &merchant_request(&altered),
    );
    assert_eq!(status, 200, "{answer}");
    assert_eq!(answer["result"]["state"], "refused");
    assert_eq!(answer["result"]["reason"], "receipt_verification_failed");
    assert_eq!(
        gateway.request("POST", "/v1/settle", &[], &merchant).0,
        401,
        "settlement still authenticates"
    );
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
enum StabilitySource {
    BlockCommit,
    ExplorerApi,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
enum StabilityLevel {
    DepthStable,
    Reversible,
    ExplorerRecorded,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct HistoryStability {
    level: StabilityLevel,
    source: StabilitySource,
    finalized_boundary: Option<String>,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
enum SettlementLevel {
    Unverified,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
enum SettlementReason {
    NoVerifiedCheckpointEvidence,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct HistorySettlement {
    level: SettlementLevel,
    source: (),
    reason: SettlementReason,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct HistoryRow {
    id: String,
    height_or_seq: String,
    chain: String,
    kind: String,
    direction: String,
    account: String,
    counterparty: Value,
    asset: String,
    amount: String,
    tx_id: String,
    tx_hash: String,
    ordinal: String,
    r#final: bool,
    final_basis: StabilitySource,
    stability: HistoryStability,
    settlement: HistorySettlement,
    decoded: Value,
    asset_metadata: Value,
    #[serde(rename = "blockHeight")]
    block_height: u64,
    #[serde(rename = "txIndex")]
    tx_index_number: u64,
    #[serde(rename = "logIndex")]
    log_index_number: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    tx_index: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    log_index: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    side: Option<String>,
}
fn history_schema(fixture: &Fixture, gateway: &Gateway) {
    use layerx_indexer::paxeer::{decode_block, AttributeEncoding, BlockInputs};
    let block = gateway.rpc("eth_getBlockByNumber", json!(["0x2", true]), None);
    let mut receipts = BTreeMap::new();
    for tx in block["result"]["transactions"]
        .as_array()
        .expect("recorded node transactions")
    {
        let hash = tx["hash"].as_str().expect("transaction hash");
        let receipt = gateway.rpc("eth_getTransactionReceipt", json!([hash]), None);
        receipts.insert(hash.into(), receipt["result"].clone());
    }
    let registry = layerx_indexer::abi::AbiRegistry::default();
    let unit = decode_block(
        &block["result"],
        &BlockInputs {
            registry: &registry,
            receipts: &receipts,
            cosmos_txs: &[],
            encoding: AttributeEncoding::Base64,
            is_pointer: &|address| fixture.store.is_pointer(address),
        },
    )
    .expect("real indexer codec");
    let address = block["result"]["transactions"][0]["from"]
        .as_str()
        .expect("sender");
    assert!(unit.transfers.iter().any(|row| row.tx_id
        == block["result"]["transactions"][0]["hash"]
            .as_str()
            .expect("hash")));
    fixture
        .store
        .commit(&unit, 0)
        .expect("real indexer store commit");
    // Capture the actual store projection once, before gateway enrichment. Both
    // transports and the strict consumer below must retain these exact fields.
    let produced = fixture
        .store
        .history(address, None, 25, None)
        .expect("indexer history projection");
    let recording = serde_json::to_vec(&produced.items).expect("record indexer rows");
    let recorded: Vec<Value> =
        serde_json::from_slice(&recording).expect("decode recorded indexer rows");
    assert_eq!(recorded, produced.items);
    assert!(!recorded.is_empty());
    let contract: Value =
        serde_json::from_str(include_str!("../openrpc.json")).expect("OpenRPC contract");
    for method in ["px_getHistory", "px_getUnifiedHistory"] {
        let answer = gateway.rpc(method, json!([address, null, 25, null]), None);
        assert!(answer.get("error").is_none(), "{answer}");
        let schema = contract["methods"]
            .as_array()
            .expect("methods")
            .iter()
            .find(|m| m["name"] == method)
            .expect("history method")["result"]["schema"]
            .clone();
        assert!(
            matches_schema(&answer["result"], &schema),
            "closed {method} schema rejects {answer}"
        );
        let rows = answer["result"]["items"].as_array().expect("history rows");
        assert!(!rows.is_empty());
        assert_eq!(rows.len(), recorded.len());
        for (row, producer) in rows.iter().zip(&recorded) {
            for (field, value) in producer.as_object().expect("recorded indexer row") {
                assert_eq!(&row[field], value, "{method} changed indexer field {field}");
            }
            let consumer: HistoryRow =
                serde_json::from_value(row.clone()).expect("closed history consumer DTO");
            assert_eq!(
                consumer.tx_hash, consumer.tx_id,
                "hash aliases must be exactly equal"
            );
            assert_eq!(
                serde_json::to_value(consumer).expect("consumer re-encode"),
                *row,
                "consumer retains every field and exact decimal string"
            );
        }
        let mut old = schema.clone();
        old["properties"]["items"]["items"]["properties"]
            .as_object_mut()
            .expect("row properties")
            .remove("tx_hash");
        old["properties"]["items"]["items"]["required"]
            .as_array_mut()
            .expect("required")
            .retain(|k| k != "tx_hash");
        assert!(
            !matches_schema(&answer["result"], &old),
            "old closed schema must reject the exact nonempty answer"
        );
        for mutation in [
            "missing",
            "encoding",
            "extra",
            "missing_stability",
            "stability_level",
            "stability_boundary",
            "stability_extra",
            "settlement_level",
            "settlement_source",
            "settlement_reason",
            "settlement_extra",
        ] {
            let mut bad = answer["result"].clone();
            let row = bad["items"][0].as_object_mut().expect("row");
            match mutation {
                "missing" => {
                    row.remove("tx_hash");
                }
                "encoding" => {
                    row.insert("tx_hash".into(), json!("0x1234"));
                }
                "extra" => {
                    row.insert("undeclared".into(), json!(true));
                }
                "missing_stability" => {
                    row.remove("stability");
                }
                "stability_level" => {
                    row.get_mut("stability").expect("stability")["level"] = json!("settled");
                }
                "stability_boundary" => {
                    row.get_mut("stability").expect("stability")["finalized_boundary"] = json!(2);
                }
                "stability_extra" => {
                    row.get_mut("stability").expect("stability")["undeclared"] = json!(true);
                }
                "settlement_level" => {
                    row.get_mut("settlement").expect("settlement")["level"] = json!("verified");
                }
                "settlement_source" => {
                    row.get_mut("settlement").expect("settlement")["source"] =
                        json!("block_commit");
                }
                "settlement_reason" => {
                    row.get_mut("settlement").expect("settlement")["reason"] = json!("unknown");
                }
                "settlement_extra" => {
                    row.get_mut("settlement").expect("settlement")["undeclared"] = json!(true);
                }
                _ => unreachable!(),
            }
            assert!(
                !matches_schema(&bad, &schema),
                "closed schema must reject {mutation}"
            );
        }
    }
}

fn state_retired_producer(
    fixture: &Fixture,
    gateway: Gateway,
    credential: &str,
    old: &str,
) -> Gateway {
    let (status, before) = gateway.request(
        "GET",
        "/v1/state",
        &[("Authorization", credential)],
        &Value::Null,
    );
    assert_eq!(status, 200, "{before}");
    let proof = &before["result"]["commitment_proof"];
    assert_eq!(
        before["result"]["block_height"]
            .as_u64()
            .expect("height")
            .to_string(),
        proof["header"]["height"]
    );
    assert_eq!(
        before["result"]["state_root"]
            .as_str()
            .expect("root")
            .to_uppercase(),
        proof["header"]["app_hash"]
    );
    assert_eq!(
        before["result"]["block_hash"]
            .as_str()
            .expect("block hash")
            .to_uppercase(),
        proof["commit"]["block_id"]["hash"]
    );
    assert_eq!(
        before["result"]["verification"],
        "bft-signed-committed-header"
    );
    let gateway = gateway.restart();
    let (status, after) = gateway.request(
        "GET",
        "/v1/state",
        &[("Authorization", credential)],
        &Value::Null,
    );
    assert_eq!(status, 200, "{after}");
    assert_eq!(
        after["result"], before["result"],
        "reopened gateway recovers the durable head and key"
    );
    assert_eq!(
        gateway
            .request(
                "GET",
                "/internal/v1/principal",
                &[("Authorization", old)],
                &Value::Null
            )
            .0,
        401,
        "revocation survives restart"
    );
    fixture.stale.store(true, Ordering::Release);
    let (status, stale) = gateway.request(
        "GET",
        "/v1/state",
        &[("Authorization", credential)],
        &Value::Null,
    );
    assert_eq!(status, 502, "{stale}");
    assert_eq!(stale["error"]["code"], "state_proof_stale");
    fixture.stale.store(false, Ordering::Release);
    let captured = support::fixture("consensus")["captured_at"]
        .as_u64()
        .expect("capture epoch");
    STATE_CLOCK.store(captured + 121, Ordering::Release);
    let (status, aged) = gateway.request(
        "GET",
        "/v1/state",
        &[("Authorization", credential)],
        &Value::Null,
    );
    assert_eq!(status, 502, "{aged}");
    assert_eq!(
        aged["error"]["code"], "state_proof_stale",
        "original freshness bound remains enforced"
    );
    STATE_CLOCK.store(captured, Ordering::Release);
    let store = fixture.store();
    let height = after["result"]["block_height"].as_u64().expect("height");
    assert!(!store
        .observe_committed_state("committed-fixture", height - 1, &"11".repeat(32))
        .expect("durable rollback refusal"));
    assert!(!store
        .observe_committed_state("committed-fixture", height, &"11".repeat(32))
        .expect("durable fork refusal"));
    assert!(store
        .observe_committed_state(
            "committed-fixture",
            height,
            after["result"]["block_hash"].as_str().expect("hash")
        )
        .expect("same committed head may be replayed"));
    gateway
}

#[test]
fn repair_22_31_202_real_producer_paths() {
    let _environment = Environment::clear();
    let fixture = Fixture::start();
    fixture.configure();
    STATE_CLOCK.store(
        support::fixture("consensus")["captured_at"]
            .as_u64()
            .expect("capture epoch"),
        Ordering::Release,
    );
    for variable in [
        "LAYERX_GATEWAY_COMPONENT_URL",
        "LAYERX_GATEWAY_PUBLIC_CORE_URL",
        "LAYERX_GATEWAY_AUTHORITY_URL",
        "LAYERX_GATEWAY_PROGRAM_REGISTRY_URL",
        "LAYERX_GATEWAY_MODULE_REGISTRY_FILE",
        "LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12",
        "LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE",
    ] {
        assert!(
            std::env::var_os(variable).is_none(),
            "retired dependency must be unset: {variable}"
        );
    }
    let gateway = Gateway::start();
    let (credential, old) = identity_config_coupling(&fixture, &gateway);
    node_root_and_settlement_component_gate(&fixture, &gateway, &credential);
    history_schema(&fixture, &gateway);
    let gateway = state_retired_producer(&fixture, gateway, &credential, &old);
    let calls = fixture.node_calls.lock().expect("node calls").clone();
    for method in ["eth_blockNumber", "eth_call", "eth_sendRawTransaction"] {
        assert!(
            calls
                .iter()
                .any(|(path, name)| path == "/" && name == method),
            "bare origin did not carry {method}"
        );
    }
    assert!(
        calls.iter().all(|(path, _)| path == "/"),
        "bare origins must not gain /rpc"
    );
    drop(gateway);
    Environment::set(
        "LAYERX_GATEWAY_PAXEER_RPC_URLS",
        json!([
            format!("{}/chain/125", fixture.node.url()),
            format!("{}/chain/125", fixture.fallback.url())
        ])
        .to_string(),
    );
    let gateway = Gateway::start();
    assert_eq!(
        gateway.rpc("eth_blockNumber", json!([]), None)["result"],
        "0xb"
    );
    assert!(
        fixture
            .node_calls
            .lock()
            .expect("node calls")
            .iter()
            .any(|(path, method)| path == "/chain/125" && method == "eth_blockNumber"),
        "explicit domain path must be preserved on the wire"
    );
    assert!(fixture
        .memory
        .lock()
        .expect("store commands")
        .commands
        .iter()
        .any(|command| command == "EVAL"));
    drop(gateway);
}
