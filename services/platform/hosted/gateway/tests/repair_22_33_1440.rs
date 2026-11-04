//! Task record 22.33.1440 (acceptance 22998.1).
//! readiness-required-backends, status-permanent-degradation and readiness-test-contract:
//! readiness_and_status_require_live_dependencies_and_recover_with_both_kernel_configurations.
//! asset-list-full-scan: asset_pages_are_bounded_stable_and_continue_after_reopen and asset_metadata_shares_one_aggregate_deadline.
//! retired-http-ingress, retired-schema-route: published_hosted_transport_and_discovery_refuse_retired_aliases.
//! tenant-fixed-beta: registration_binds_two_tenants_to_real_identity_storage.
//! explorer-pages-retired: archive_pages_and_next_data_refuse_retired_urls.
//! Baseline was not run: the ordered BUILD instructions prohibit gates before implementation.
//! Wiring request: services/platform/hosted/gateway/route-locations.json and docker/roles/router (domain 8): publish hosted POST / and GET /openrpc.json, keeping node targets at /.
//! Wiring request: sdk/platform and apps/web (client owners): consume /openrpc.json and the configured registration tenant; continue px_listAssets using next_cursor in registration order.
//! Wiring request: services/platform/hosted/indexer/src (indexer owner): retain the bounded rowid keyset /v1/assets contract used here; lexical ordering would require a separately versioned index.
//! Wiring request: apps/explorer/pages/paxeer-x and apps/explorer/lib/api (explorer owner): move receipt/anchor pages and Next data to archive/, using explorer_proxy.rs archive APIs; complete frontend rendering qualification there.
//! Gotcha: the shared schema checker requires canonical decimal patterns; the published asset schema uses its supported decimal form.
//! Each case owns its environment values and listeners; only production environment access serializes, with assertions outside that guard.
//! Gotcha: indexer Readiness includes an explicit explorer field; this node-backed fixture sets it to None.
//! Gotcha: the embeddable Server does not initialize public UI origins, and the public UI transport refuses loopback targets. This gate proves production page admission/refusal; deployed frontend rendering belongs to the explorer wiring request.

use ed25519_dalek::{Signer, SigningKey};
use layerx_indexer::store::{AssetRow, Store};
use layerx_platform_gateway::evm;
use layerx_platform_gateway::http::{self, IncomingRequest, OutgoingResponse};
use layerx_platform_gateway::runtime::Server;
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::cell::RefCell;
use std::collections::BTreeMap;
use std::ffi::OsString;
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener};
use std::os::unix::fs::PermissionsExt;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

#[path = "support/repair_22_31_202.rs"]
mod support;
// The production identity journal/store is the peer; no identity implementation is copied.
#[path = "../../identity/src/store.rs"]
mod identity_store;
use support::{hex, json_response, Peer, RedisMemory};

static CONFIGURATION_ENVIRONMENT: Mutex<()> = Mutex::new(());
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

fn with_environment<T>(values: &BTreeMap<OsString, OsString>, run: impl FnOnce() -> T) -> T {
    let _configuration = CONFIGURATION_ENVIRONMENT
        .lock()
        .expect("configuration environment");
    let _restore = Environment::clear();
    for (name, value) in values {
        std::env::set_var(name, value);
    }
    run()
}

struct Fixture {
    directory: tempfile::TempDir,
    environment: RefCell<BTreeMap<OsString, OsString>>,
    tls: Arc<ServerConfig>,
    _redis: Peer,
}
impl Fixture {
    fn new() -> Self {
        let directory = tempfile::tempdir().expect("fixture directory");
        let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()])
            .expect("fixture TLS certificate");
        let tls = Arc::new(
            ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_safe_default_protocol_versions()
                .expect("TLS versions")
                .with_no_client_auth()
                .with_single_cert(
                    vec![certificate.cert.der().clone()],
                    PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
                        certificate.signing_key.serialize_der(),
                    )),
                )
                .expect("fixture TLS config"),
        );
        let memory = Arc::new(Mutex::new(RedisMemory::default()));
        let redis = RedisMemory::serve(memory, Arc::clone(&tls));
        let ca = directory.path().join("ca.der");
        fs::write(&ca, certificate.cert.der()).expect("CA file");
        let fixture = Self {
            directory,
            environment: RefCell::new(BTreeMap::new()),
            tls,
            _redis: redis,
        };
        fixture.set("LAYERX_GATEWAY_OUTBOUND_CA_DER", ca);
        for (name, value) in [
            ("LISTEN", "127.0.0.1:0"),
            ("LISTENER", "plain"),
            ("NETWORK_ID", "gateway-repair"),
            ("PROTOCOL_NETWORK_ID", "7"),
        ] {
            fixture.set(format!("LAYERX_GATEWAY_{name}"), value);
        }
        fixture.set(
            "LAYERX_GATEWAY_LXP_WIRE_VERSION",
            layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
        );
        fixture.set(
            "LAYERX_GATEWAY_REDIS_URL",
            format!("rediss://localhost:{}", fixture._redis.address.port()),
        );
        fixture.secret("REDIS_USERNAME", "gateway");
        fixture.secret("REDIS_PASSWORD", "gateway-secret");
        fixture
    }
    fn secret(&self, name: &str, value: &str) {
        let path = self.directory.path().join(name);
        fs::write(&path, value).expect("fixture secret");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600)).expect("protected secret");
        self.set(format!("LAYERX_GATEWAY_{name}_FILE"), path);
    }
    fn set(&self, name: impl AsRef<std::ffi::OsStr>, value: impl AsRef<std::ffi::OsStr>) {
        self.environment
            .borrow_mut()
            .insert(name.as_ref().into(), value.as_ref().into());
    }
    fn remove(&self, name: &str) {
        self.environment
            .borrow_mut()
            .remove(std::ffi::OsStr::new(name));
    }
    fn server(&self) -> Result<Server, String> {
        with_environment(&self.environment.borrow(), Server::from_environment)
    }
    fn gateway(&self) -> Peer {
        let server = self.server().expect("production gateway configuration");
        let environment = self.environment.borrow().clone();
        Peer::start(move |tcp| {
            with_environment(&environment, || server.serve_connection(tcp))
                .expect("production gateway connection")
        })
    }
    fn chain(&self) -> Peer {
        Peer::https(Arc::clone(&self.tls), chain_response)
    }
    fn configure_chain(&self, first: &Peer, second: &Peer) {
        self.set(
            "LAYERX_GATEWAY_PAXEER_RPC_URLS",
            json!([first.url(), second.url()]).to_string(),
        );
    }
    fn legacy(&self) {
        for name in ["COMPONENT", "AUTHORITY", "PROGRAM_REGISTRY"] {
            self.set(format!("LAYERX_GATEWAY_{name}_URL"), "https://localhost:1");
            self.secret(&format!("{name}_TOKEN"), "unused-retired-token");
        }
        self.secret("MODULE_REGISTRY", &json!({"schema_version":2,"assets":[{"asset":"02".repeat(32),"currency":"USD","decimals":6,"symbol":"$"}],"modules":[{"module":9,"ordinals":[1,2,7]}]}).to_string());
        let path = self.directory.path().join("client.p12");
        fs::write(&path, support::unhex(CLIENT_IDENTITY)).expect("committed client identity");
        self.set("LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12", path);
        self.secret("CLIENT_IDENTITY_PASSWORD", "fixture-only");
    }
}

fn exchange(gateway: &Peer, method: &str, path: &str, body: &Value) -> (u16, Value) {
    let body = if method == "GET" {
        Vec::new()
    } else {
        serde_json::to_vec(body).unwrap()
    };
    let mut stream = std::net::TcpStream::connect(gateway.address).expect("gateway connection");
    stream
        .set_read_timeout(Some(Duration::from_secs(25)))
        .expect("request deadline");
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
    stream.write_all(&body).unwrap();
    let mut response = Vec::new();
    stream.read_to_end(&mut response).unwrap();
    let split = response
        .windows(4)
        .position(|b| b == b"\r\n\r\n")
        .expect("HTTP headers");
    let status = std::str::from_utf8(&response[..split])
        .unwrap()
        .split_whitespace()
        .nth(1)
        .unwrap()
        .parse()
        .unwrap();
    (
        status,
        serde_json::from_slice(&response[split + 4..]).expect("JSON response"),
    )
}
fn rpc(gateway: &Peer, method: &str, params: Value) -> Value {
    let (status, answer) = exchange(
        gateway,
        "POST",
        "/",
        &json!({"jsonrpc":"2.0","id":9,"method":method,"params":params}),
    );
    assert_eq!(status, 200);
    assert_eq!(answer["jsonrpc"], "2.0");
    assert_eq!(answer["id"], 9);
    answer
}

fn abi_string(value: &str) -> String {
    let mut bytes = vec![0_u8; 64 + value.len().div_ceil(32) * 32];
    bytes[31] = 32;
    bytes[63] = u8::try_from(value.len()).expect("fixture string length");
    bytes[64..64 + value.len()].copy_from_slice(value.as_bytes());
    format!("0x{}", hex(&bytes))
}
fn chain_response(request: IncomingRequest) -> OutgoingResponse {
    assert_eq!(request.path, "/", "node targets use their published root");
    assert_eq!(request.method, "POST");
    let call: Value = serde_json::from_slice(&request.body).expect("node request");
    let result = match call["method"].as_str().expect("node method") {
        "eth_chainId" => json!("0x7d"),
        "eth_blockNumber" => json!("0xb"),
        "eth_call" => {
            assert_eq!(
                call["params"][0]["to"],
                evm::address_hex(&evm::BANK_PRECOMPILE)
            );
            assert_eq!(call["params"][1], "0xb");
            let data = support::unhex(call["params"][0]["data"].as_str().expect("call data"));
            let selector: [u8; 4] = data[..4].try_into().expect("selector");
            if selector == evm::SELECTOR_BANK_NAME {
                json!(abi_string("Paxeer"))
            } else if selector == evm::SELECTOR_BANK_SYMBOL {
                json!(abi_string("PAX"))
            } else {
                let number = if selector == evm::SELECTOR_BANK_DECIMALS {
                    18
                } else {
                    assert_eq!(selector, evm::SELECTOR_BANK_SUPPLY);
                    100
                };
                json!(format!("0x{number:064x}"))
            }
        }
        other => panic!("unexpected node request {other}"),
    };
    json_response(
        200,
        &json!({"jsonrpc":"2.0","id":call["id"],"result":result}),
    )
}

// A stopped listener is dropped and reopened at the same address, exercising real reconnects.
struct Restartable {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}
impl Restartable {
    fn start(
        address: &str,
        tls: Arc<ServerConfig>,
        handler: Arc<dyn Fn(IncomingRequest) -> OutgoingResponse + Send + Sync>,
    ) -> Self {
        let listener = TcpListener::bind(address).expect("dependency listener");
        let address = listener.local_addr().expect("dependency address");
        listener.set_nonblocking(true).expect("nonblocking");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let thread = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((tcp, _)) => {
                        tcp.set_read_timeout(Some(Duration::from_secs(3)))
                            .expect("dependency deadline");
                        let mut stream = StreamOwned::new(
                            ServerConnection::new(Arc::clone(&tls)).expect("dependency TLS"),
                            tcp,
                        );
                        let request =
                            http::read_request(&mut stream, 65536).expect("dependency request");
                        http::write_response(&mut stream, &handler(request))
                            .expect("dependency response");
                        stream.conn.send_close_notify();
                        stream.flush().expect("dependency flush");
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(2))
                    }
                    Err(error) => panic!("dependency accept: {error}"),
                }
            }
        });
        Self {
            address,
            stop,
            thread: Some(thread),
        }
    }
    fn url(&self) -> String {
        format!("https://localhost:{}", self.address.port())
    }
}
impl Drop for Restartable {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            thread.join().expect("dependency stopped");
        }
    }
}

fn indexer_response(store: &Store, request: IncomingRequest) -> OutgoingResponse {
    let readiness = layerx_indexer::api::Readiness {
        sources: vec!["paxeer"],
        stall_after_secs: 60,
        started_at: 0,
        explorer: None,
    };
    let answer = layerx_indexer::api::route_with(store, &readiness, &request.method, &request.path);
    json_response(
        answer.status,
        &serde_json::from_str::<Value>(&answer.body).expect("real indexer JSON"),
    )
}
fn fresh(store: &Store) {
    store
        .commit(
            &layerx_indexer::store::Unit {
                chain: "paxeer".into(),
                position: 11,
                hash: "11".repeat(32),
                link: "11".repeat(32),
                boundary: 11,
                ..Default::default()
            },
            0,
        )
        .expect("commit indexed source cursor");
    store.start_source("paxeer").expect("start indexer source");
    store
        .record_source_head("paxeer", Some(11))
        .expect("source head");
    store
        .record_source_success("paxeer", Some(11), Some(11))
        .expect("source reconciliation");
}

fn assert_contract(gateway: &Peer, ready: bool, missing: Option<&str>) -> Value {
    let (status, document) = exchange(gateway, "GET", "/readyz/core", &Value::Null);
    assert_eq!(status, if ready { 200 } else { 503 });
    assert_eq!(document["status"], if ready { "ready" } else { "degraded" });
    assert_eq!(document["scope"], "core");
    assert_eq!(document["readiness_version"], 1);
    assert_eq!(document["protocol_network_id"], 7);
    assert_eq!(document["network_id"], "gateway-repair");
    assert_eq!(document["product_routes"], Value::Null);
    assert_eq!(document["components"], json!({"durable_store":"ready"}));
    assert_eq!(
        document["valid_until_ms"].as_u64().unwrap() - document["observed_at_ms"].as_u64().unwrap(),
        30_000
    );
    let backends = document["backends"].as_object().expect("dependencies");
    assert_eq!(
        backends.keys().map(String::as_str).collect::<Vec<_>>(),
        vec!["consensus", "durable_store", "indexer", "paxeer_chain"]
    );
    assert_eq!(document["capabilities"].as_object().unwrap().len(), 2);
    assert_eq!(
        document["capabilities"]["layerx_activity"],
        json!({"state":"not_configured","reason":"not_configured"})
    );
    assert_eq!(
        document["capabilities"]["chain_serving"],
        json!({"state":backends["paxeer_chain"]["state"],"reason":backends["paxeer_chain"]["reason"],"chain_id":125})
    );
    if let Some(name) = missing {
        assert_ne!(backends[name]["state"], "ready");
    }
    let (code, status) = exchange(gateway, "GET", "/v1/status", &Value::Null);
    assert_eq!(code, 200);
    assert_eq!(
        status["services"]["hosted_gateway"],
        if ready { "available" } else { "degraded" }
    );
    let services = status["services"].as_object().unwrap();
    assert_eq!(services.len(), 5);
    for name in backends.keys() {
        assert_eq!(
            services[name] == "available",
            backends[name]["state"] == "ready"
        );
    }
    let (code, product) = exchange(gateway, "GET", "/readyz", &Value::Null);
    assert_eq!(
        code, 503,
        "unconfigured advertised product dependencies degrade only the product scope"
    );
    assert_eq!(product["status"], "degraded");
    assert_eq!(product["scope"], "product");
    assert_eq!(product["backends"], document["backends"]);
    assert_eq!(product["product_routes"]["complete"], false);
    assert!(product["product_routes"]["services"]
        .as_object()
        .unwrap()
        .values()
        .any(|v| v["ready"] == false));
    document
}

#[test]
fn readiness_and_status_require_live_dependencies_and_recover_with_both_kernel_configurations() {
    let fixture = Fixture::new();
    let store = Arc::new(Store::open_in_memory().expect("indexer store"));
    fresh(&store);
    let node_handler: Arc<dyn Fn(IncomingRequest) -> OutgoingResponse + Send + Sync> =
        Arc::new(chain_response);
    let indexed = Arc::clone(&store);
    let indexer_handler: Arc<dyn Fn(IncomingRequest) -> OutgoingResponse + Send + Sync> =
        Arc::new(move |request| indexer_response(&indexed, request));
    let consensus_handler: Arc<dyn Fn(IncomingRequest) -> OutgoingResponse + Send + Sync> =
        Arc::new(|request| {
            assert_eq!(request.path, "/");
            let call: Value = serde_json::from_slice(&request.body).unwrap();
            assert_eq!(call["method"], "status");
            json_response(
                200,
                &json!({"jsonrpc":"2.0","id":call["id"],"result":{"sync_info":{"catching_up":false}}}),
            )
        });
    let mut previous = None;
    for legacy in [false, true] {
        if legacy {
            fixture.legacy();
        }
        // A missing dependency is a failure even though its configured bit is false.
        for name in ["PAXEER_RPC_URLS", "INDEXER_URL", "CONSENSUS_RPC_URL"] {
            fixture.remove(&format!("LAYERX_GATEWAY_{name}"));
        }
        let gateway = fixture.gateway();
        let missing = assert_contract(&gateway, false, Some("paxeer_chain"));
        for name in ["paxeer_chain", "consensus", "indexer"] {
            assert_eq!(missing["backends"][name]["reason"], "not_configured");
        }
        drop(gateway);
        let mut node = Some(Restartable::start(
            "127.0.0.1:0",
            Arc::clone(&fixture.tls),
            Arc::clone(&node_handler),
        ));
        let mut indexer = Some(Restartable::start(
            "127.0.0.1:0",
            Arc::clone(&fixture.tls),
            Arc::clone(&indexer_handler),
        ));
        let mut consensus = Some(Restartable::start(
            "127.0.0.1:0",
            Arc::clone(&fixture.tls),
            Arc::clone(&consensus_handler),
        ));
        fixture.set(
            "LAYERX_GATEWAY_PAXEER_RPC_URLS",
            json!([
                node.as_ref().unwrap().url(),
                format!("{}/", node.as_ref().unwrap().url()) + "chain"
            ])
            .to_string(),
        );
        fixture.set(
            "LAYERX_GATEWAY_INDEXER_URL",
            indexer.as_ref().unwrap().url(),
        );
        fixture.set(
            "LAYERX_GATEWAY_CONSENSUS_RPC_URL",
            consensus.as_ref().unwrap().url(),
        );
        let gateway = fixture.gateway();
        let mut healthy = assert_contract(&gateway, true, None);
        healthy.as_object_mut().unwrap().remove("observed_at_ms");
        healthy.as_object_mut().unwrap().remove("valid_until_ms");
        if let Some(previous) = &previous {
            assert_eq!(
                previous, &healthy,
                "legacy configuration cannot change merged readiness"
            );
        }
        previous = Some(healthy);
        for (name, peer, handler) in [
            ("paxeer_chain", &mut node, &node_handler),
            ("indexer", &mut indexer, &indexer_handler),
            ("consensus", &mut consensus, &consensus_handler),
        ] {
            let address = peer.as_ref().unwrap().address;
            drop(peer.take());
            assert_contract(&gateway, false, Some(name));
            *peer = Some(Restartable::start(
                &address.to_string(),
                Arc::clone(&fixture.tls),
                Arc::clone(handler),
            ));
            assert_contract(&gateway, true, None);
        }
        drop(gateway);
    }
}

#[test]
fn published_hosted_transport_and_discovery_refuse_retired_aliases() {
    let fixture = Fixture::new();
    let first = fixture.chain();
    let second = fixture.chain();
    fixture.configure_chain(&first, &second);
    let gateway = fixture.gateway();
    let published = layerx_platform_gateway::unified_service_catalogue();
    assert_eq!(published["hosted_gateway"]["rpc_path"], "/");
    assert_eq!(published["node_rpc"]["rpc_path"], "/");
    let (status, schema) = exchange(&gateway, "GET", "/openrpc.json", &Value::Null);
    assert_eq!(status, 200);
    assert_eq!(
        schema,
        serde_json::from_str::<Value>(include_str!("../openrpc.json")).unwrap()
    );
    assert!(schema["info"]["description"]
        .as_str()
        .unwrap()
        .contains("GET /openrpc.json"));
    assert_eq!(
        rpc(&gateway, "px_getNetwork", json!([]))["result"]["paxeer"]["chain_id"],
        "0x7d"
    );
    for method in [
        "lx_getNetwork",
        "eth_chainId",
        "net_version",
        "web3_clientVersion",
    ] {
        assert_eq!(
            rpc(&gateway, method, json!([]))["error"]["code"],
            -32601,
            "hosted dispatcher does not own {method}"
        );
    }
    for path in [
        "/rpc",
        "/rpc/",
        "/rpc?legacy=1",
        "/rpc/schema",
        "/rpc/schema/",
        "/rpc/schema?legacy=1",
        "/rpc/evm",
    ] {
        for method in ["GET", "POST"] {
            assert_eq!(
                exchange(
                    &gateway,
                    method,
                    path,
                    &json!({"jsonrpc":"2.0","id":1,"method":"px_getNetwork","params":[]})
                )
                .0,
                404,
                "{method} {path}"
            );
        }
    }
    assert_eq!(exchange(&gateway, "GET", "/", &Value::Null).0, 405);
    assert_eq!(
        exchange(&gateway, "POST", "/openrpc.json", &json!({})).0,
        405
    );
}

#[test]
fn asset_pages_are_bounded_stable_and_continue_after_reopen() {
    let fixture = Fixture::new();
    let first = fixture.chain();
    let second = fixture.chain();
    fixture.configure_chain(&first, &second);
    let database = fixture.directory.path().join("indexer.sqlite");
    let store = Store::open(&database).expect("persistent indexer");
    let assets: Vec<_> = (0..1033)
        .map(|i| AssetRow {
            asset: format!("asset-{:04}", 1033 - i),
            chain: "paxeer".into(),
            kind: "native".into(),
            address: None,
            denom: Some(format!("denom{i}")),
            metadata: json!({}),
        })
        .collect();
    store
        .register_assets(&assets)
        .expect("populate real catalogue");
    let requests = Arc::new(Mutex::new(Vec::new()));
    let observed = Arc::clone(&requests);
    let indexer = Peer::https(Arc::clone(&fixture.tls), move |request| {
        observed.lock().unwrap().push(request.path.clone());
        indexer_response(&store, request)
    });
    fixture.set("LAYERX_GATEWAY_INDEXER_URL", indexer.url());
    let gateway = fixture.gateway();
    let page = rpc(&gateway, "px_listAssets", json!([]));
    assert!(page.get("error").is_none(), "{page}");
    let result = &page["result"];
    assert_eq!(result["assets"].as_array().unwrap().len(), 16);
    assert_eq!(result["next_cursor"], "16");
    assert_eq!(result["order"], "registration");
    assert_eq!(result["joined_limit"], 16);
    assert_eq!(result["height"], 11);
    let contract: Value = serde_json::from_str(include_str!("../openrpc.json")).unwrap();
    let schema = &contract["methods"]
        .as_array()
        .unwrap()
        .iter()
        .find(|m| m["name"] == "px_listAssets")
        .unwrap()["result"]["schema"];
    assert!(
        support::matches_schema(result, schema),
        "real response satisfies the closed published schema"
    );
    for (i, asset) in result["assets"].as_array().unwrap().iter().enumerate() {
        assert_eq!(
            asset,
            &json!({"asset_id": assets[i].asset, "kind":"native", "denom":assets[i].denom,"name":"Paxeer","symbol":"PAX","decimals":18,"supply":"100"})
        );
    }
    assert_eq!(*requests.lock().unwrap(), vec!["/v1/assets?limit=16"]);
    assert_eq!(
        rpc(&gateway, "px_listAssets", json!([])),
        page,
        "stable bounded first page"
    );
    drop(gateway);
    drop(indexer);
    let reopened = Store::open(&database).expect("reopen indexer database");
    let observed = Arc::clone(&requests);
    let indexer = Peer::https(Arc::clone(&fixture.tls), move |request| {
        observed.lock().unwrap().push(request.path.clone());
        indexer_response(&reopened, request)
    });
    fixture.set("LAYERX_GATEWAY_INDEXER_URL", indexer.url());
    let gateway = fixture.gateway();
    assert_eq!(rpc(&gateway, "px_listAssets", json!([])), page);
    let continued = rpc(&gateway, "px_listAssets", json!([result["next_cursor"]]));
    assert_eq!(continued["result"]["next_cursor"], "32");
    assert_eq!(
        continued["result"]["assets"][0]["asset_id"],
        assets[16].asset
    );
    let last = rpc(&gateway, "px_listAssets", json!(["1024"]));
    assert_eq!(last["result"]["assets"].as_array().unwrap().len(), 9);
    assert_eq!(last["result"]["next_cursor"], Value::Null);
    assert_eq!(
        rpc(&gateway, "px_listAssets", json!(["1033"]))["result"]["assets"],
        json!([])
    );
    for params in [
        json!(["0"]),
        json!(["016"]),
        json!(["-1"]),
        json!(["9223372036854775808"]),
        json!([16]),
        json!(["16", "32"]),
    ] {
        assert_eq!(
            rpc(&gateway, "px_listAssets", params)["error"]["code"],
            -32602
        );
    }
    assert_eq!(
        requests.lock().unwrap().len(),
        6,
        "one bounded indexer exchange per valid RPC"
    );
}

#[test]
fn asset_metadata_shares_one_aggregate_deadline() {
    let fixture = Fixture::new();
    let calls = Arc::new(AtomicUsize::new(0));
    let counter = Arc::clone(&calls);
    let first = Peer::https(Arc::clone(&fixture.tls), move |request| {
        counter.fetch_add(1, Ordering::AcqRel);
        std::thread::sleep(Duration::from_millis(120));
        chain_response(request)
    });
    let second = fixture.chain();
    fixture.configure_chain(&first, &second);
    let store = Store::open_in_memory().unwrap();
    let assets: Vec<_> = (0..16)
        .map(|i| AssetRow {
            asset: format!("asset{i}"),
            chain: "paxeer".into(),
            kind: "native".into(),
            address: None,
            denom: Some(format!("denom{i}")),
            metadata: json!({}),
        })
        .collect();
    store.register_assets(&assets).unwrap();
    let indexer = Peer::https(Arc::clone(&fixture.tls), move |request| {
        indexer_response(&store, request)
    });
    fixture.set("LAYERX_GATEWAY_INDEXER_URL", indexer.url());
    let gateway = fixture.gateway();
    let started = Instant::now();
    let refused = rpc(&gateway, "px_listAssets", json!([]));
    assert_eq!(
        refused["error"]["data"]["code"],
        "asset_list_deadline_exceeded"
    );
    assert!(
        refused.get("result").is_none(),
        "no truncated successful metadata page"
    );
    assert!(started.elapsed() >= Duration::from_secs(10));
    assert!(started.elapsed() < Duration::from_secs(13));
    assert!(
        calls.load(Ordering::Acquire) < 130,
        "deadline stops work before all asset reads"
    );
}

fn registration_digest(domain: &[u8], tenant: &str, key: &[u8]) -> [u8; 32] {
    let mut hash = Sha256::new();
    for part in [domain, tenant.as_bytes(), key] {
        hash.update((part.len() as u64).to_be_bytes());
        hash.update(part);
    }
    hash.finalize().into()
}

#[test]
fn registration_binds_two_tenants_to_real_identity_storage() {
    let fixture = Fixture::new();
    let directory = fixture.directory.path().join("identity");
    fs::create_dir(&directory).unwrap();
    fs::set_permissions(&directory, fs::Permissions::from_mode(0o700)).unwrap();
    let identity = Arc::new(Mutex::new(
        identity_store::Store::open(&directory).expect("real identity journal"),
    ));
    let calls = Arc::new(AtomicUsize::new(0));
    let requests = Arc::clone(&calls);
    let database = Arc::clone(&identity);
    let mismatch = Arc::new(AtomicBool::new(false));
    let diverge = Arc::clone(&mismatch);
    let peer = Peer::https(Arc::clone(&fixture.tls), move |request| {
        assert_eq!(request.method, "POST");
        assert_eq!(request.path, "/v1/principals");
        assert_eq!(
            request.headers["authorization"],
            "Bearer provisioning-token"
        );
        requests.fetch_add(1, Ordering::AcqRel);
        let value: Value = serde_json::from_slice(&request.body).unwrap();
        assert_eq!(value.as_object().unwrap().len(), 3);
        let principal = identity_store::Principal {
            tenant: value["tenant"].as_str().unwrap().into(),
            sub: value["sub"].as_str().unwrap().into(),
            allowed_signer_public_keys: serde_json::from_value(
                value["allowed_signer_public_keys"].clone(),
            )
            .unwrap(),
            account: None,
            audiences: Vec::new(),
        };
        let mut store = database.lock().unwrap();
        store
            .put_principal(principal.clone())
            .expect("production identity persistence");
        let mut response =
            serde_json::to_value(store.principal(&principal.tenant, &principal.sub).unwrap())
                .unwrap();
        if diverge.load(Ordering::Acquire) {
            response["tenant"] = json!("wrong-tenant");
        }
        json_response(200, &response)
    });
    fixture.set("LAYERX_GATEWAY_IDENTITY_URL", peer.url());
    fixture.secret("IDENTITY_TOKEN", "identity-token");
    fixture.secret("IDENTITY_PROVISIONING_TOKEN", "provisioning-token");
    assert!(
        fixture.server().is_err(),
        "tenant cannot silently default to beta"
    );
    let signing = SigningKey::from_bytes(&[7; 32]);
    let key = signing.verifying_key().to_bytes();
    let mut subjects = Vec::new();
    let mut gateways = Vec::new();
    for tenant in ["alpha", "gamma"] {
        fixture.set("LAYERX_GATEWAY_TENANT", tenant);
        let gateway = fixture.gateway();
        let signature = hex(&signing
            .sign(&registration_digest(
                b"layerx-register-binding-v1",
                tenant,
                &key,
            ))
            .to_bytes());
        let params = json!([hex(&key), signature]);
        let answer = rpc(&gateway, "px_register", params.clone());
        let subject = format!(
            "{tenant}.{}",
            &hex(&registration_digest(
                b"layerx-register-subject-v1",
                tenant,
                &key
            ))[..32]
        );
        assert_eq!(
            answer["result"],
            json!({"tenant":tenant,"sub":subject,"allowed_signer_public_keys":[hex(&key)],"account":null,"audiences":[]})
        );
        assert_eq!(
            rpc(&gateway, "px_register", params.clone()),
            answer,
            "registration is idempotent"
        );
        let count = calls.load(Ordering::Acquire);
        let cross = hex(&signing
            .sign(&registration_digest(
                b"layerx-register-binding-v1",
                "beta",
                &key,
            ))
            .to_bytes());
        let refused = rpc(&gateway, "px_register", json!([hex(&key), cross]));
        assert_eq!(refused["error"]["code"], -32002);
        assert_eq!(
            refused["error"]["data"]["code"],
            "registration_proof_invalid"
        );
        assert_eq!(
            calls.load(Ordering::Acquire),
            count,
            "cross-tenant signature never reaches identity"
        );
        let (status, forged_context) = support::request(
            gateway.address,
            "POST",
            "/",
            &[("X-Tenant", "beta"), ("X-LayerX-Tenant", "beta")],
            &json!({"jsonrpc":"2.0","id":9,"method":"px_register","params":[hex(&key),hex(&signing.sign(&registration_digest(b"layerx-register-binding-v1", "beta", &key)).to_bytes())]}),
        );
        assert_eq!(status, 200);
        assert_eq!(
            forged_context["error"]["code"], -32002,
            "untrusted request headers cannot select the signature tenant"
        );
        assert_eq!(calls.load(Ordering::Acquire), count);
        mismatch.store(true, Ordering::Release);
        assert_eq!(
            rpc(&gateway, "px_register", params.clone())["error"]["code"],
            -32603,
            "identity must confirm the same tenant"
        );
        mismatch.store(false, Ordering::Release);
        subjects.push((tenant, subject));
        gateways.push((gateway, params, answer));
    }
    assert_ne!(subjects[0].1, subjects[1].1);
    for (gateway, params, expected) in &gateways {
        assert_eq!(
            rpc(gateway, "px_register", params.clone()),
            *expected,
            "each running fixture uses its own trusted tenant context"
        );
    }
    drop(gateways);
    drop(peer);
    drop(identity);
    let recovered = identity_store::Store::open(&directory).expect("identity journal recovery");
    for (tenant, sub) in &subjects {
        assert_eq!(recovered.principal(tenant, sub).unwrap().tenant, *tenant);
        assert!(recovered
            .principal(if *tenant == "alpha" { "gamma" } else { "alpha" }, sub)
            .is_none());
    }
}

#[test]
fn archive_pages_and_next_data_refuse_retired_urls() {
    let fixture = Fixture::new();
    let gateway = fixture.gateway();
    for page in ["anchors", "receipts", "receipts/receipt-123"] {
        for path in [
            format!("/explorer/paxeer-x/archive/{page}"),
            format!("/explorer/_next/data/build-22/paxeer-x/archive/{page}.json"),
        ] {
            assert_eq!(
                exchange(&gateway, "GET", &path, &Value::Null).0,
                503,
                "supported page reaches the real proxy dependency check: {path}"
            );
        }
        for path in [
            format!("/explorer/paxeer-x/{page}"),
            format!("/explorer/_next/data/build-22/paxeer-x/{page}.json"),
        ] {
            assert_eq!(
                exchange(&gateway, "GET", &path, &Value::Null).0,
                404,
                "retired page refused before proxy transport: {path}"
            );
        }
    }
}

// Test-only client identity, generated once during implementation; no native tool runs in this gate.
const CLIENT_IDENTITY: &str = "3082043c020103308203f206092a864886f70d010701a08203e3048203df308203db3082028a06092a864886f70d010706a082027b308202770201003082027006092a864886f70d010701305f06092a864886f70d01050d3052303106092a864886f70d01050c30240410e6eb6e6b65b9f4d6286474eacd9bc6cb02020800300c06082a864886f70d02090500301d060960864801650304012a0410fc2f503807b8ab4270b0efe75c6e2f168082020020aa0ecdb3d1a4d36afb75233b47036380224e441d337a570e28ed6f36811331e0ccbd3234f2e4a09965e4a373f72229c667c94b59924eb8ecf68f58191fda845eca837e3fe8dbf8d63304b7eeb2d030d9076063a8c1cdce170d694e176a9de7df165221c61668b0378e293cf0031f6cc9dec4e69183d6f4d86b067070256d3f33d03679cc8918db0a37fe6981569a8358a59ccd2e12c92a88a80ea4aae093ac277c19d6fa08fa16bb2a8f9c4be6a304b4180f5557404fe1064c27a9873e76be48c46c4ec093fb45ada82f7af3811496aa3c5e166df6d5be579cf87ba423ef510348c15337c9e6cff42a21ba97eb4073d018ede91a9f38e0045a24685d581a984a5c1d26c78b82c1713b91f1a9f2129deccb8332026f973e75725a84ec099bcaa9bd2347181d4bc36db56d539d9aee4401d0cfecbf8081ca5c38152411376ed665bc20602f35a816ef8c810ee8a00d9305222eda65a60dcac667395e86e61ce69355131faa365429a8a3772c48438c87c798085b4a8b694121027c809c92f4786b5e1edbd2d9b0d0b1836072506213c273d3de3c193a0fcfad03f5ab7cc9bc19712e39fea601a28d8e8f0331a5074328296bb02eddb4559af99c0a72ba639e58281ab0ca130159d2702fa9edb6ee5269eaa6b28377229285e7974b2a43c258bf69c8c99034987a2a07d9ced6a44603ca3e97b7ecd6afa49fd44a7c1c12e8b7713082014906092a864886f70d010701a082013a04820136308201323082012e060b2a864886f70d010c0a0102a081f73081f4305f06092a864886f70d01050d3052303106092a864886f70d01050c30240410031cbfe2de082e293ef47af1fbf7160402020800300c06082a864886f70d02090500301d060960864801650304012a041046247aecc0936b443515f7b6292a99e30481906610fd41b1948a155db8ce4710bf801eff093398508c87a29775ffea661e8e7e13ab10ec9321fcba82f42d3d7d14fc4ae9a34253f0ed70f441830e668d8f836a121cb5fc29fbbabe071039fce3a6f873ce1bde37ab7c3d3a6e8f9e962ec3e744dc57156d518db9cd2f96f53e7469fd3b0c82a541004b587d5c42dc95ca4747f0ec6a19d4902ab7f98b16d9c997e5c4ca3125302306092a864886f70d01091531160414a301f15d2331ec05abc6c2db5064b28e2908afde30413031300d060960864801650304020105000420bc54ffa4d7a9651eb64bf9a2c37cb56660914db2adbd2602ae0faf7808acdfd40408862df6ec7665d66202020800";
