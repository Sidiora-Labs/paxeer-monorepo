fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::approval::ApprovalService;
use layerx_agentd::budget::{
    create_daemon_limit, daemon_limit_id, reserve_until_core_time, BudgetLimiter, CoreTimestampMs,
    DaemonLimitRecord, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::human::{
    serve_one, AcceptedTransport, HumanAgentLifecycleSeed, HumanPeer, APPROVAL_BUDGET_AFTER_V2,
};
use layerx_agentd::human_runtime::{
    HumanAuthorityBoundary, ProductionHumanOperations, RemoteHumanAuthority, UnifiedAgentOwner,
};
use layerx_agentd::identity::{
    self, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::managed_agent::{self, ManagedAgent};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::policy::approval::{hold_reserved_at, ApprovalContext, ApprovalState};
use layerx_agentd::prepare::evm::{
    prepare_bound, AbiArgument, PrecompileCall, PrecompileTarget, PreparationDefaults,
    PrepareRequest, ProductionEvmPreparationBoundary,
};
use layerx_agentd::session::{self, OpenRequest, SessionId, SessionRegistry};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_types::clock::Clock;
use layerx_types::ids::Did;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

const DEADLINE: Duration = Duration::from_secs(8);
const MAXIMUM: usize = 1_048_576;

mod hex {
    pub use ::hex::{decode, encode};

    pub fn decode32(value: &str) -> Result<[u8; 32], ()> {
        decode(value).map_err(|_| ())?.try_into().map_err(|_| ())
    }
}

#[path = "support/human_budget_native_store.rs"]
mod native_store;

fn record_authority_budget_state(url: String, fixture: &RecordedAuthority) -> Value {
    let response = must(read_authority_budget_state(url, fixture));
    assert_eq!(response.status, 200);
    must(serde_json::from_slice(&response.body))
}

fn read_authority_budget_state(
    url: String,
    fixture: &RecordedAuthority,
) -> Result<layerx_platform_authority::Response, layerx_platform_authority::Response> {
    use layerx_platform_authority::{
        budget_state, human_policy::PrincipalPolicy, BudgetStateConfig,
    };

    let config = BudgetStateConfig {
        chain_rpc_url: url,
        protocol_network_id: require_ok(u32::try_from(fixture.chain_id), "test value fits u32"),
    };
    let evidence = json!({
        "activity_id": fixture.managed_receipt["activity_id"],
        "receipt_digest": hex::encode(verified_managed_receipt(&fixture.managed_receipt)),
    });
    let key_policy = json!({
        "policy_revision":1, "required_delay_seconds":0, "maximum_delay_seconds":0,
        "effective_sequence":0, "evidence":evidence,
    });
    let policy: PrincipalPolicy = must(serde_json::from_value(json!({
        "tenant":fixture.tenant, "principal":fixture.principal,
        "account_id":hex::encode(&fixture.budget_call[32..64]), "asset_id":hex::encode([4;32]),
        "activities":[], "budgets":[hex::encode(fixture.budget_id)],
        "identities":[{
            "did":fixture.agent_did, "frozen":false, "authorities":[],
            "revocation_sequence":u64::from_be_bytes(require_ok(fixture.budget_call[13 * 32 + 24..14 * 32].try_into(), "canonical budget revocation sequence")),
            "evidence":evidence, "capabilities":[], "rotation":key_policy, "recovery":key_policy,
        }],
        "maximum_age_seconds":315_360_000_u64, "maximum_age_sequences":100,
    })));
    budget_state::read(&config, &policy, &hex::encode(fixture.budget_id))
}

struct LocalServer {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

fn read_http(stream: &mut (impl Read + Write)) -> (String, Vec<u8>) {
    let mut headers = Vec::new();
    while !headers.ends_with(b"\r\n\r\n") {
        assert!(headers.len() < 16_384);
        let mut byte = [0];
        must(stream.read_exact(&mut byte));
        headers.push(byte[0]);
    }
    let headers = must(String::from_utf8(headers));
    let length = headers
        .lines()
        .find_map(|line| {
            let (key, value) = line.split_once(':')?;
            key.eq_ignore_ascii_case("content-length")
                .then(|| must(value.trim().parse::<usize>()))
        })
        .unwrap_or(0);
    assert!(length <= MAXIMUM);
    let mut body = vec![0; length];
    must(stream.read_exact(&mut body));
    (headers, body)
}

fn write_http(stream: &mut impl Write, body: &Value) {
    let bytes = must(serde_json::to_vec(body));
    must(write!(
        stream,
        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        bytes.len()
    ));
    must(stream.write_all(&bytes));
    must(stream.flush());
}

impl LocalServer {
    fn spawn(listener: TcpListener, mut handle: impl FnMut(TcpStream) + Send + 'static) -> Self {
        must(listener.set_nonblocking(true));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((stream, address)) => {
                        assert!(address.ip().is_loopback());
                        must(stream.set_read_timeout(Some(DEADLINE)));
                        must(stream.set_write_timeout(Some(DEADLINE)));
                        handle(stream);
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("loopback accept: {error}"),
                }
            }
        });
        Self {
            stop,
            worker: Some(worker),
        }
    }

    fn node(fixture: &RecordedAuthority) -> (String, Self) {
        Self::node_with_fault(fixture, "")
    }

    fn node_with_fault(fixture: &RecordedAuthority, fault: &'static str) -> (String, Self) {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}/", must(listener.local_addr()));
        let fixture: RecordedAuthority =
            must(serde_json::from_value(must(serde_json::to_value(fixture))));
        let native = native_store::NativeStore::for_authority(&fixture);
        assert_eq!(
            fixture.block["stateRoot"],
            format!("0x{}", hex::encode(native.app_hash))
        );
        let checkpoint_reads: Vec<Value> = must(serde_json::from_str(include_str!(
            "support/human_budget_checkpoint.json"
        )));
        let height = require_ok(
            u64::from_str_radix(
                require_some(fixture.block["number"].as_str(), "required test value")
                    .trim_start_matches("0x"),
                16,
            ),
            "required test value",
        );
        let server = Self::spawn(listener, move |mut stream| {
            let (_, body) = read_http(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            let mut account = json!({
                "evm_address":format!("0x{}",hex::encode(fixture.actor_address)),
                "pax_address":null,"layerx_did":fixture.agent_did,
                "layerx_account":hex::encode(&fixture.budget_call[32..64]),"bound":true,
            });
            let result = match require_some(request["method"].as_str(), "RPC method") {
                "px_getCommit" | "px_getValidators" => {
                    assert_eq!(request["params"], json!([height.to_string()]));
                    let mut recorded = require_some(
                        checkpoint_reads.iter().find(|recorded| {
                            recorded["method"] == request["method"]
                                && recorded["params"] == request["params"]
                        }),
                        "checkpoint RPC recorded at the requested height",
                    )["result"]
                        .clone();
                    if fault == "header" && request["method"] == "px_getCommit" {
                        recorded["signedHeader"]["header"]["app_hash"] =
                            json!(hex::encode([0x99; 32]));
                    }
                    recorded
                }
                "abci_query" => {
                    assert_eq!(request["params"]["path"], "/store/lxphost/key");
                    verify_node_with_fault_1_repair_22_31_562(fault, &native, height, &request)
                }
                "eth_chainId" => json!(format!("{:#x}", fixture.chain_id)),
                "eth_blockNumber" => fixture.block["number"].clone(),
                "eth_getBlockByNumber" => fixture.block.clone(),
                "eth_getTransactionCount" => {
                    assert_eq!(
                        request["params"][0],
                        format!("0x{}", hex::encode(fixture.actor_address))
                    );
                    json!("0x0")
                }
                "px_getAccount" => {
                    json!({"account":account,"paxeer":null,"layerx":{"accounts":[account]}})
                }
                "px_getBalances" => {
                    account["layerx_account"] = request["params"][0].clone();
                    json!({"account":account,"joined_limit":100,"balances":[{
                        "asset_id":hex::encode([4;32]),"denom":null,"custody":null,"paxeer":null,
                        "layerx":{"amount":"77"}
                    }]})
                }
                "eth_call" => node_contract_call(&fixture, &request, height),
                "px_getReceiptKeys" => {
                    json!({"keys":[{"keyId":format!("0x{}",hex::encode([1;32])),"publicKey":format!("0x{}",require_some(fixture.balance_context["sequencer_public_key"].as_str(), "required test value")),"firstHeight":1,"lastHeight":65535}]})
                }
                other => panic!("unexpected RPC {other}: {request}"),
            };
            write_http(
                &mut stream,
                &json!({"jsonrpc":"2.0","id":request["id"],"result":result}),
            );
        });
        (url, server)
    }

    fn authority(
        fixture: &RecordedAuthority,
        budget_label: Arc<AtomicU8>,
    ) -> (String, Vec<u8>, Self) {
        use openssl::asn1::Asn1Time;
        use openssl::hash::MessageDigest;
        use openssl::pkey::PKey;
        use openssl::rsa::Rsa;
        use openssl::x509::extension::{BasicConstraints, KeyUsage, SubjectAlternativeName};
        use openssl::x509::{X509NameBuilder, X509};
        let key = must(PKey::from_rsa(must(Rsa::generate(2048))));
        let mut name = must(X509NameBuilder::new());
        must(name.append_entry_by_text("CN", "localhost"));
        let name = name.build();
        let mut certificate = must(X509::builder());
        must(certificate.set_version(2));
        must(certificate.set_subject_name(&name));
        must(certificate.set_issuer_name(&name));
        must(certificate.set_pubkey(&key));
        must(certificate.set_not_before(must(Asn1Time::days_from_now(0)).as_ref()));
        must(certificate.set_not_after(must(Asn1Time::days_from_now(1)).as_ref()));
        must(certificate.append_extension(must(BasicConstraints::new().critical().ca().build())));
        must(
            certificate.append_extension(must(
                KeyUsage::new()
                    .digital_signature()
                    .key_encipherment()
                    .key_cert_sign()
                    .build(),
            )),
        );
        let san = must(
            SubjectAlternativeName::new()
                .dns("localhost")
                .build(&certificate.x509v3_context(None, None)),
        );
        must(certificate.append_extension(san));
        must(certificate.sign(&key, MessageDigest::sha256()));
        let ca_certificate = certificate.build();
        let ca = must(ca_certificate.to_der());
        let leaf_key = must(PKey::from_rsa(must(Rsa::generate(2048))));
        let mut certificate = must(X509::builder());
        must(certificate.set_version(2));
        finish_authority_1_repair_22_31_562(
            FinishAuthority1Repair2231562Context {
                fixture,
                key,
                name,
                ca_certificate,
                ca,
                leaf_key,
                certificate,
            },
            budget_label,
        )
    }
}

fn verify_node_with_fault_1_repair_22_31_562(
    fault: &'static str,
    native: &native_store::NativeStore,
    height: u64,
    request: &Value,
) -> Value {
    assert_eq!(request["params"]["height"], (height - 1).to_string());
    assert_eq!(request["params"]["prove"], true);
    let key = require_some(
        request["params"]["data"]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "required test value"))
            .strip_prefix("0x"),
        "required test value",
    );
    let mut proof = require_some(native.queries.get(key), "committed native key").clone();
    proof["height"] = json!((height - 1).to_string());
    if fault == "native" {
        proof["value"] = json!("AAAAAAAAAAA=");
    }
    json!({"response":proof})
}

struct FinishAuthority1Repair2231562Context<'a> {
    fixture: &'a RecordedAuthority,
    key: openssl::pkey::PKey<openssl::pkey::Private>,
    name: openssl::x509::X509Name,
    ca_certificate: openssl::x509::X509,
    ca: Vec<u8>,
    leaf_key: openssl::pkey::PKey<openssl::pkey::Private>,
    certificate: openssl::x509::X509Builder,
}

fn finish_authority_1_repair_22_31_562(
    context: FinishAuthority1Repair2231562Context<'_>,
    budget_label: Arc<AtomicU8>,
) -> (String, Vec<u8>, LocalServer) {
    use openssl::asn1::Asn1Time;
    use openssl::hash::MessageDigest;

    use openssl::x509::extension::{BasicConstraints, KeyUsage, SubjectAlternativeName};

    let FinishAuthority1Repair2231562Context {
        fixture,
        key,
        name,
        ca_certificate,
        ca,
        leaf_key,
        mut certificate,
    } = context;

    must(certificate.set_subject_name(&name));
    must(certificate.set_issuer_name(ca_certificate.subject_name()));
    must(certificate.set_pubkey(&leaf_key));
    must(certificate.set_not_before(must(Asn1Time::days_from_now(0)).as_ref()));
    must(certificate.set_not_after(must(Asn1Time::days_from_now(1)).as_ref()));
    must(certificate.append_extension(must(BasicConstraints::new().critical().build())));
    must(
        certificate.append_extension(must(
            KeyUsage::new()
                .digital_signature()
                .key_encipherment()
                .build(),
        )),
    );
    let san = must(
        SubjectAlternativeName::new()
            .dns("localhost")
            .build(&certificate.x509v3_context(Some(&ca_certificate), None)),
    );
    must(certificate.append_extension(san));
    must(certificate.sign(&key, MessageDigest::sha256()));
    let certificate = certificate.build();
    let identity = must(native_tls::Identity::from_pkcs8(
        &must(certificate.to_pem()),
        &must(leaf_key.private_key_to_pem_pkcs8()),
    ));
    let acceptor = must(native_tls::TlsAcceptor::new(identity));
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let endpoint = format!("https://localhost:{}", must(listener.local_addr()).port());
    let registry = fixture.registry.clone();
    let budget = fixture.budget_state.clone();
    let balance = fixture.balance_context.clone();
    let identity = json!({
        "canonical_core_bytes":require_some(fixture.agent_did.strip_prefix("did:layerx:"), "required test value"),
        "head_sequence":fixture.budget_state["observed_head_sequence"],
        "revocation_sequence":fixture.budget_state["revocation_sequence"],
        "verification_level":layerx_types::verify::VerificationLevel::COMMITTED.wire_label(), "frozen":false,
        "authorities":[{"kind":"primary_key","id":require_some(fixture.agent_did.strip_prefix("did:layerx:"), "required test value")}]
    });
    let server = LocalServer::spawn(listener, move |stream| {
        let mut stream = must(acceptor.accept(stream));
        let (headers, _) = read_http(&mut stream);
        assert!(headers.contains("Bearer approval-budget-loopback-authority"));
        let path = require_some(
            headers
                .lines()
                .next()
                .unwrap_or_else(|| panic!("{}", "required test value"))
                .split_whitespace()
                .nth(1),
            "required test value",
        );
        let response = match require_some(path.split('?').next(), "required test value") {
            "/v1/agent/registry" => &registry,
            "/v1/agent/budget-state" => {
                let mut response = budget.clone();
                response["verification"] = json!(budget_label.load(Ordering::Acquire));
                write_http(&mut stream, &response);
                return;
            }
            "/v1/agent/balance-context" => &balance,
            "/v1/agent/identity" => &identity,
            other => panic!("unexpected authority route: {other}"),
        };
        write_http(&mut stream, response);
    });
    (endpoint, ca, server)
}

impl LocalServer {
    fn shutdown(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}

impl Drop for LocalServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

#[track_caller]
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("approval budget actor: {error:?}"),
    }
}

struct AuthorityIdentity<'a> {
    authority: &'a mut RemoteHumanAuthority,
    peer: &'a HumanPeer,
}

impl IdentityResolver for AuthorityIdentity<'_> {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        self.authority.core_identity(self.peer, did).map(Some)
    }
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct RecordedAuthority {
    chain_id: u64,
    tenant: String,
    principal: String,
    agent_did: String,
    budget_id: [u8; 32],
    actor_address: [u8; 20],
    budget_call: Vec<u8>,
    budget_state: Value,
    registry: Value,
    balance_context: Value,
    block: Value,
    managed_receipt: Value,
}

fn verified_managed_receipt(fixture: &Value) -> [u8; 32] {
    let bytes = must(hex::decode(require_some(
        fixture["receipt"].as_str(),
        "required test value",
    )));
    let header_bytes = must(hex::decode(require_some(
        fixture["header"].as_str(),
        "required test value",
    )));
    let header = must(layerx_wire::receipt::decode_batch_header(&header_bytes));
    let public = must(hex::decode32(require_some(
        fixture["sequencer_public_key"].as_str(),
        "required test value",
    )));
    let signature = must(
        must(hex::decode(require_some(
            fixture["header_signature"].as_str(),
            "required test value",
        )))
        .try_into(),
    );
    let stored = must(hex::decode(require_some(
        fixture["receipt_proof"].as_str(),
        "required test value",
    )));
    assert_eq!(stored.len(), 9 + usize::from(stored[0]) * 32);
    let proof = must(layerx_proof::merkle::Proof::new(
        u32::from_be_bytes(require_ok(stored[1..5].try_into(), "required test value")),
        u32::from_be_bytes(require_ok(stored[5..9].try_into(), "required test value")),
        stored[9..]
            .chunks_exact(32)
            .map(|bytes| require_ok(bytes.try_into(), "required test value"))
            .collect(),
    ));
    let authorization = layerx_proof::inclusion::SequencerAuthorization::new(
        header.sequencer_id(),
        public,
        header.batch_number(),
        header.batch_number(),
    );
    must(layerx_proof::inclusion::verify_receipt(
        &bytes,
        &proof,
        &header_bytes,
        &signature,
        &authorization,
    ));
    let receipt = must(layerx_wire::receipt::decode(&bytes));
    let protocol = require_some(receipt.protocol(), "managed protocol receipt");
    assert_eq!(protocol.module_id(), 7);
    assert_eq!(protocol.result_code(), 0);
    Sha256::digest(&bytes).into()
}

struct AuthorityFixture {
    recorded: RecordedAuthority,
    authority_endpoint: String,
    authority_ca_der: Vec<u8>,
    node_rpc_url: String,
    servers: Vec<LocalServer>,
    budget_label: Arc<AtomicU8>,
}

impl std::ops::Deref for AuthorityFixture {
    type Target = RecordedAuthority;

    fn deref(&self) -> &Self::Target {
        &self.recorded
    }
}

impl AuthorityFixture {
    fn load() -> Self {
        let source = must(fs::read_to_string(
            Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/human_approval_budget_actor.rs"),
        ));
        let start = require_some(
            source.find(concat!("/* RECORDED_", "AUTHORITY\n")),
            "recorded authority start",
        );
        let data = &source[start + "/* RECORDED_AUTHORITY\n".len()..];
        let end = require_some(
            data.find(concat!("\nEND_RECORDED_", "AUTHORITY */")),
            "recorded authority end",
        );
        let mut recorded: RecordedAuthority = must(serde_json::from_str(&data[..end]));
        native_store::prepare_authority(&mut recorded);
        let (node_rpc_url, node) = LocalServer::node(&recorded);
        let endpoint = must(SequencerEndpoint::new(&node_rpc_url, &[]));
        let checkpoint = must(
            layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
                &endpoint,
                require_some(
                    recorded.budget_state["observed_head_sequence"].as_u64(),
                    "required test value",
                ),
                None,
                &recorded.chain_id.to_string(),
            ),
        );
        assert_eq!(
            hex::encode(checkpoint.block_hash),
            recorded.budget_state["checkpoint_digest"]
        );
        let state = record_authority_budget_state(node_rpc_url.clone(), &recorded);
        assert_eq!(state["remaining"], recorded.budget_state["remaining"]);
        assert_eq!(
            state["observed_head_sequence"],
            recorded.budget_state["observed_head_sequence"]
        );
        assert_eq!(
            state["receipt_digest"],
            hex::encode(Sha256::digest(&recorded.budget_call))
        );
        assert_eq!(
            state["checkpoint_digest"],
            recorded.budget_state["checkpoint_digest"]
        );
        assert_eq!(
            state["verification"],
            layerx_types::verify::VerificationLevel::COMMITTED.wire_rank()
        );
        recorded.budget_state = state;
        let budget_label = Arc::new(AtomicU8::new(
            layerx_types::verify::VerificationLevel::COMMITTED.wire_rank(),
        ));
        let (authority_endpoint, authority_ca_der, authority) =
            LocalServer::authority(&recorded, Arc::clone(&budget_label));
        Self {
            recorded,
            node_rpc_url,
            authority_endpoint,
            authority_ca_der,
            servers: vec![node, authority],
            budget_label,
        }
    }

    fn shutdown(&mut self) {
        for server in &mut self.servers {
            server.shutdown();
        }
    }

    fn connect(&self) -> RemoteHumanAuthority {
        must(RemoteHumanAuthority::connect(
            &self.authority_endpoint,
            "approval-budget-loopback-authority".to_owned(),
            DEADLINE,
            MAXIMUM,
            &self.authority_ca_der,
        ))
    }
}

struct Runtime {
    root: PathBuf,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    clock: Arc<RuntimeClock>,
}

impl Runtime {
    fn start() -> Self {
        let mut nonce = [0; 16];
        must(getrandom::fill(&mut nonce));
        let root = std::env::temp_dir().join(format!("lx-approval-{}", hex::encode(nonce)));
        must(fs::create_dir(&root));
        must(fs::set_permissions(
            &root,
            fs::Permissions::from_mode(0o700),
        ));
        let mut server = must(clock_server::Server::bind(&root));
        let binding = ClockBinding {
            socket: server.socket().to_path_buf(),
            process: std::process::id(),
            user: must(fs::metadata(&root)).uid(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let signal = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !signal.load(Ordering::Acquire) {
                must(server.poll());
                thread::sleep(Duration::from_millis(1));
            }
        });
        let clock = must(RuntimeClock::connect(binding));
        Self {
            root,
            stop,
            worker: Some(worker),
            clock,
        }
    }

    fn owner(
        &self,
        fixture: &AuthorityFixture,
        peers: &BTreeMap<u32, (String, String)>,
        store: Arc<Mutex<Store>>,
    ) -> UnifiedAgentOwner<RemoteHumanAuthority> {
        let operations = must(ProductionHumanOperations::new(
            fixture.connect(),
            must(SequencerEndpoint::new(&fixture.node_rpc_url, &[])),
            Box::new(must(NodeRpc::connect(&NodeRpcConfig {
                url: fixture.node_rpc_url.clone(),
                chain_id: fixture.chain_id,
                deadline: DEADLINE,
            }))),
            Arc::clone(&store),
            peers,
            MAXIMUM,
            30,
            self.clock.clone(),
        ));
        let keys = self.root.join("keys");
        must(fs::create_dir_all(&keys));
        must(fs::set_permissions(
            &keys,
            fs::Permissions::from_mode(0o700),
        ));
        must(UnifiedAgentOwner::new(
            operations,
            store,
            peers,
            vec![LimitConfig {
                id: LimitId([1; 16]),
                name: "approval actor enrolment".to_owned(),
                scope: LimitScope::Tenant([2; 32]),
                ceiling: 1_000_000_000_000_000_000,
                consumed: 0,
            }],
            must(SessionKeyRegistry::open(
                keys,
                vec![3; 32],
                require_ok(u32::try_from(fixture.chain_id), "protocol network"),
                must(fs::metadata(&self.root)).uid(),
            )),
        ))
    }
}

impl Runtime {
    fn shutdown(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
        must(fs::remove_dir_all(&self.root));
    }
}

impl Drop for Runtime {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn operation(approval_id: [u8; 32], held_digest: [u8; 32], sequence: u64) -> Vec<u8> {
    let mut bytes = b"LXHAGT01".to_vec();
    bytes.push(APPROVAL_BUDGET_AFTER_V2);
    bytes.extend_from_slice(&approval_id);
    bytes.extend_from_slice(&held_digest);
    bytes.extend_from_slice(&sequence.to_be_bytes());
    bytes
}

fn refusal_frame(reason: &[u8]) -> Vec<u8> {
    fn field(bytes: &mut Vec<u8>, value: &[u8]) {
        bytes.extend_from_slice(
            &require_ok(u32::try_from(value.len()), "bounded field").to_be_bytes(),
        );
        bytes.extend_from_slice(value);
    }
    let mut frame = b"LXHAGT01\x01".to_vec();
    field(&mut frame, reason);
    frame
}

fn budget_after(label: u8) {
    let fixture = AuthorityFixture::load();
    let creation_receipt = verified_managed_receipt(&fixture.managed_receipt);
    let runtime = Runtime::start();
    let uid = must(fs::metadata(&runtime.root)).uid();
    let peer = HumanPeer {
        uid,
        principal: fixture.principal.clone(),
        tenant: fixture.tenant.clone(),
        subject: None,
    };
    let peers = BTreeMap::from([(uid, (peer.principal.clone(), peer.tenant.clone()))]);
    let tenant = must(TenantId::new(&fixture.tenant));
    let mut authority = fixture.connect();
    let registry = must(authority.registry(&peer));
    let state = must(authority.budget_state(&peer, fixture.budget_id));
    let (_, asset, currency, _, _, _, _) = must(authority.balance_context(&peer));
    assert_eq!(state.asset, asset);
    assert_eq!(
        state.verification,
        layerx_types::verify::VerificationLevel::COMMITTED.wire_rank()
    );
    let endpoint = must(SequencerEndpoint::new(&fixture.node_rpc_url, &[]));
    let public_key: [u8; 32] = require_ok(
        must(hex::decode(require_some(
            fixture.agent_did.strip_prefix("did:layerx:"),
            "managed DID",
        )))
        .try_into(),
        "managed identity public key",
    );
    let address = require_some(
        must(endpoint.bound_address(&public_key)),
        "actual bound signing address",
    );
    let actor = hex::encode(address);
    assert_eq!(actor.len(), 40);
    assert_ne!(actor, fixture.agent_did);
    let sequence = state.observed_head_sequence;
    let boundary = must(ProductionEvmPreparationBoundary::new(
        &fixture.node_rpc_url,
        &[],
    ));
    let observed = must(boundary.preparation_state(&address));
    assert_eq!(observed.head_height, sequence, "one genuine chain snapshot");
    let mut amount_word = [0; 32];
    amount_word[31] = 50;
    let evm = must(prepare_bound(
        observed,
        PreparationDefaults {
            max_priority_fee_per_gas: 0,
            max_fee_per_gas: require_some(
                observed
                    .base_fee_per_gas
                    .checked_mul(2)
                    .unwrap_or_else(|| panic!("{}", "required test value"))
                    .checked_add(0),
                "required test value",
            ),
            maximum_payload_bytes: MAXIMUM,
        },
        PrepareRequest {
            from: address,
            call: PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: layerx_crypto::disclosure::call::ledger_selector(
                    "mint(bytes32,bytes32,uint128)",
                ),
                arguments: vec![
                    AbiArgument::Word(asset),
                    AbiArgument::Word([4; 32]),
                    AbiArgument::Word(amount_word),
                ],
            },
            expected_nonce: Some(observed.pending_nonce),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: MAXIMUM,
        },
        100_000,
    ));
    finish_budget_after_5_repair_22_31_562(FinishBudgetAfter5Repair2231562Context {
        label,
        fixture,
        creation_receipt,
        runtime,
        peer,
        peers,
        tenant,
        authority,
        registry,
        state,
        asset,
        currency,
        public_key,
        address,
        actor,
        sequence,
        evm,
    });
}

struct FinishBudgetAfter5Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    creation_receipt: [u8; 32],
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    authority: RemoteHumanAuthority,
    registry: layerx_types::payload::ModuleRegistry,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    currency: String,
    public_key: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    evm: layerx_agentd::prepare::Prepared,
}

fn finish_budget_after_5_repair_22_31_562(context: FinishBudgetAfter5Repair2231562Context) {
    let FinishBudgetAfter5Repair2231562Context {
        label,
        fixture,
        creation_receipt,
        runtime,
        peer,
        peers,
        tenant,
        mut authority,
        registry,
        state,
        asset,
        currency,
        public_key,
        address,
        actor,
        sequence,
        evm,
    } = context;

    assert_eq!(evm.disclosure.actor, address);
    assert_eq!(evm.disclosure.asset, asset);
    let bytes = evm.call.encode_unsigned();
    let held_digest: [u8; 32] = Sha256::digest(&bytes).into();
    let now = must(runtime.clock.sample(DEADLINE)).unix_seconds();
    let expiry = require_some(now.checked_add(120), "held wall deadline");
    let window = require_some(sequence.checked_add(20), "held sequence deadline");
    let approval_id = [5; 32];
    let counterparty = must(AgentDid::new(hex::encode([4; 32])));
    let prepared = Prepared {
        preparation_ref: must(PreparationRef::new(hex::encode(approval_id))),
        unsigned_canonical_bytes: must(CanonicalBytes::new(bytes)),
        signing_preimage: must(SigningPreimage::new(evm.signing_hash.to_vec())),
        disclosure: Disclosure {
            canonical_digest: held_digest,
            activity_type: ActivityType(7),
            actor: must(AgentDid::new(&actor)),
            authority: must(AuthorityRef::new(hex::encode(public_key))),
            counterparties: ExplicitSet::allow(vec![counterparty.clone()]),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty,
                amount: Amount(50),
            }]),
            asset: must(Asset::new(&currency)),
            fee_limit: Amount(evm.disclosure.fee_limit),
            expiry: TimestampSeconds(expiry),
            idempotency_key: must(IdempotencyRef::new(hex::encode(approval_id))),
        },
        expiry: TimestampSeconds(expiry),
    };
    let amount = require_some(evm.disclosure.fee_limit.checked_add(50), "held charge");
    let ceiling = require_some(amount.checked_add(1_000), "daemon ceiling");
    assert!(state.remaining > amount, "genuine funded budget required");
    let expected_remaining = 1_000.min(state.remaining - amount);
    let store_path = runtime.root.join("store");
    let mut store = must(Store::open(&store_path));
    let did = must(Did::new(fixture.agent_did.as_bytes()));
    let identity = must(identity::register(
        &mut store,
        tenant.clone(),
        did.clone(),
        &mut AuthorityIdentity {
            authority: &mut authority,
            peer: &peer,
        },
    ));
    finish_budget_after_4_repair_22_31_562(FinishBudgetAfter4Repair2231562Context {
        label,
        fixture,
        creation_receipt,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        currency,
        public_key,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        ceiling,
        expected_remaining,
        store_path,
        store,
        did,
        identity,
    });
}

struct FinishBudgetAfter4Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    creation_receipt: [u8; 32],
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    registry: layerx_types::payload::ModuleRegistry,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    currency: String,
    public_key: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    held_digest: [u8; 32],
    now: u64,
    expiry: u64,
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    ceiling: u128,
    expected_remaining: u128,
    store_path: PathBuf,
    store: Store,
    did: Did,
    identity: identity::IdentityRecord,
}

fn finish_budget_after_4_repair_22_31_562(context: FinishBudgetAfter4Repair2231562Context) {
    let FinishBudgetAfter4Repair2231562Context {
        label,
        fixture,
        creation_receipt,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        currency,
        public_key,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        ceiling,
        expected_remaining,
        store_path,
        mut store,
        did,
        identity,
    } = context;

    let mut sessions = SessionRegistry::default();
    let token = must(session::open(
        &mut store,
        &mut sessions,
        &identity,
        OpenRequest {
            session_id: SessionId([7; 32]),
            token_id: [8; 32],
            tenant: tenant.clone(),
            agent: did,
            authority: ProtocolAuthority::PrimaryKey(public_key),
            permitted_activity_types: BTreeSet::from([7]),
            scopes: BTreeSet::from(["prepare".to_owned()]),
            expiry_sequence: window,
            expiry_seconds: Some(expiry),
            opening_client: "human-approval-budget-actor".to_owned(),
            policy_version: "approval-actor-v1".to_owned(),
        },
        sequence,
    ));
    assert_eq!(token.credential().generation(), 1);
    let seed = approval_budget_seed(ApprovalBudgetSeed {
        fixture: &fixture,
        creation_receipt,
        currency,
        ceiling,
        now,
        expiry,
        state: &state,
        public_key,
        asset,
        window,
    });
    verify_finish_budget_after_4_repair_22_31_562_1_repair_22_31_562(
        VerifyFinishBudgetAfter4Repair22315621Repair2231562Context {
            label,
            fixture,
            runtime,
            peer,
            peers,
            tenant,
            registry,
            state,
            asset,
            address,
            actor,
            sequence,
            held_digest,
            now,
            expiry,
            window,
            approval_id,
            prepared,
            amount,
            ceiling,
            expected_remaining,
            store_path,
            store,
            sessions,
            seed,
        },
    );
}

struct VerifyFinishBudgetAfter4Repair22315621Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    registry: layerx_types::payload::ModuleRegistry,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    held_digest: [u8; 32],
    now: u64,
    expiry: u64,
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    ceiling: u128,
    expected_remaining: u128,
    store_path: PathBuf,
    store: Store,
    sessions: SessionRegistry,
    seed: HumanAgentLifecycleSeed,
}

fn verify_finish_budget_after_4_repair_22_31_562_1_repair_22_31_562(
    context: VerifyFinishBudgetAfter4Repair22315621Repair2231562Context,
) {
    let VerifyFinishBudgetAfter4Repair22315621Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        ceiling,
        expected_remaining,
        store_path,
        store,
        sessions,
        seed,
    } = context;

    continue_finish_budget_after_4_repair_22_31_562_1_repair_22_31_562(
        ContinueFinishBudgetAfter4Repair22315621Repair2231562Context {
            label,
            fixture,
            runtime,
            peer,
            peers,
            tenant,
            registry,
            state,
            asset,
            address,
            actor,
            sequence,
            held_digest,
            now,
            expiry,
            window,
            approval_id,
            prepared,
            amount,
            ceiling,
            expected_remaining,
            store_path,
            store,
            sessions,
            seed,
        },
    );
}

struct ContinueFinishBudgetAfter4Repair22315621Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    registry: layerx_types::payload::ModuleRegistry,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    held_digest: [u8; 32],
    now: u64,
    expiry: u64,
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    ceiling: u128,
    expected_remaining: u128,
    store_path: PathBuf,
    store: Store,
    sessions: SessionRegistry,
    seed: HumanAgentLifecycleSeed,
}

fn continue_finish_budget_after_4_repair_22_31_562_1_repair_22_31_562(
    context: ContinueFinishBudgetAfter4Repair22315621Repair2231562Context,
) {
    let ContinueFinishBudgetAfter4Repair22315621Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        ceiling,
        expected_remaining,
        store_path,
        store,
        sessions,
        seed,
    } = context;

    complete_finish_budget_after_4_repair_22_31_562_1_repair_22_31_562(
        CompleteFinishBudgetAfter4Repair22315621Repair2231562Context {
            label,
            fixture,
            runtime,
            peer,
            peers,
            tenant,
            registry,
            state,
            asset,
            address,
            actor,
            sequence,
            held_digest,
            now,
            expiry,
            window,
            approval_id,
            prepared,
            amount,
            ceiling,
            expected_remaining,
            store_path,
            store,
            sessions,
            seed,
        },
    );
}

struct CompleteFinishBudgetAfter4Repair22315621Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    registry: layerx_types::payload::ModuleRegistry,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    held_digest: [u8; 32],
    now: u64,
    expiry: u64,
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    ceiling: u128,
    expected_remaining: u128,
    store_path: PathBuf,
    store: Store,
    sessions: SessionRegistry,
    seed: HumanAgentLifecycleSeed,
}

fn complete_finish_budget_after_4_repair_22_31_562_1_repair_22_31_562(
    context: CompleteFinishBudgetAfter4Repair22315621Repair2231562Context,
) {
    let CompleteFinishBudgetAfter4Repair22315621Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        ceiling,
        expected_remaining,
        store_path,
        store,
        sessions,
        seed,
    } = context;

    finish_budget_after_3_repair_22_31_562(FinishBudgetAfter3Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        ceiling,
        expected_remaining,
        store_path,
        store,
        sessions,
        seed,
    });
}

struct FinishBudgetAfter3Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    registry: layerx_types::payload::ModuleRegistry,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    held_digest: [u8; 32],
    now: u64,
    expiry: u64,
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    ceiling: u128,
    expected_remaining: u128,
    store_path: PathBuf,
    store: Store,
    sessions: SessionRegistry,
    seed: HumanAgentLifecycleSeed,
}

fn finish_budget_after_3_repair_22_31_562(context: FinishBudgetAfter3Repair2231562Context) {
    let FinishBudgetAfter3Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        ceiling,
        expected_remaining,
        store_path,
        mut store,
        sessions,
        seed,
    } = context;

    let agent = must(ManagedAgent::from_creation(
        &seed,
        &fixture.agent_did,
        [7; 32],
        [8; 32],
        1,
    ));
    must(managed_agent::publish_creation(&mut store, &tenant, &agent));
    must(managed_agent::validate_session_coordinates(
        &store, &sessions, &tenant,
    ));
    let mut agent_digest = Sha256::new();
    agent_digest.update(b"layerx-agentd/daemon-limit-agent/v1\0");
    agent_digest.update(
        require_ok(
            u32::try_from(fixture.agent_did.len()),
            "required test value",
        )
        .to_be_bytes(),
    );
    agent_digest.update(fixture.agent_did.as_bytes());
    let limit_budget = [9; 32];
    let limit = must(create_daemon_limit(
        &mut store,
        &must(BudgetLimiter::new(Vec::new())),
        DaemonLimitRecord {
            tenant: tenant.clone(),
            budget_id: limit_budget,
            limit_id: daemon_limit_id(limit_budget),
            agent_digest: agent_digest.finalize().into(),
            asset,
            ceiling,
            consumed: 0,
            expiry_ms: require_some(expiry.checked_mul(1_000), "required test value"),
            revoked: false,
            mutation_key: [10; 32],
            body_digest: [11; 32],
            revoke_key: [0; 32],
        },
        CoreTimestampMs(require_some(now.checked_mul(1_000), "required test value")),
    ));
    finish_budget_after_2_repair_22_31_562(FinishBudgetAfter2Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        expected_remaining,
        store_path,
        store,
        limit,
    });
}

struct FinishBudgetAfter2Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    registry: layerx_types::payload::ModuleRegistry,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    held_digest: [u8; 32],
    now: u64,
    expiry: u64,
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    expected_remaining: u128,
    store_path: PathBuf,
    store: Store,
    limit: DaemonLimitRecord,
}

fn finish_budget_after_2_repair_22_31_562(context: FinishBudgetAfter2Repair2231562Context) {
    let FinishBudgetAfter2Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        peers,
        tenant,
        registry,
        state,
        asset,
        address,
        actor,
        sequence,
        held_digest,
        now,
        expiry,
        window,
        approval_id,
        prepared,
        amount,
        expected_remaining,
        store_path,
        store,
        limit,
    } = context;

    let shared = Arc::new(Mutex::new(store));
    let owner = runtime.owner(&fixture, &peers, Arc::clone(&shared));
    let reservation = must(reserve_until_core_time(
        &owner.budgets,
        &ReservationRequest {
            id: approval_id,
            amount,
            current_sequence: sequence,
            expiry_sequence: window,
            applicable_limits: vec![limit.limit_id],
        },
        CoreTimestampMs(require_some(
            expiry.checked_mul(1_000),
            "required test value",
        )),
        CoreTimestampMs(require_some(now.checked_mul(1_000), "required test value")),
    ));
    let ticket = must(hold_reserved_at(
        &owner.approvals,
        ApprovalContext {
            tenant: tenant.clone(),
            agent: must(Did::new(actor.as_bytes())),
            actor_address: address,
            session: SessionId([7; 32]),
            capability: CapabilityId([6; 32]),
            policy_version: "approval-actor-v1".to_owned(),
            request_id: approval_id,
        },
        prepared,
        sequence,
        window,
        &reservation,
        runtime.clock.as_ref(),
    ));
    assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    assert_eq!(ticket.disclosure.actor.as_str(), actor);
    must(owner.approvals.validate_registry(&tenant, &registry));
    let hold_key = must(TenantKey::new(
        tenant.clone(),
        ObjectKind::PreparedActivity,
        [b"approval-hold-v1:".as_slice(), &approval_id].concat(),
    ));
    assert_eq!(
        require_some(must(shared.lock()).get(&hold_key), "durable held call").class(),
        StorageClass::LocalOnly
    );
    drop(owner);
    drop(shared);
    let shared = Arc::new(Mutex::new(must(Store::open(&store_path))));
    let owner = runtime.owner(&fixture, &peers, shared);
    let restored = must(
        ApprovalService::new(&owner.approvals, &owner.budgets, &owner.approval_expiry).get(
            &tenant,
            approval_id,
            sequence,
            TimestampSeconds(must(runtime.clock.sample(DEADLINE)).unix_seconds()),
        ),
    );
    finish_budget_after_1_repair_22_31_562(FinishBudgetAfter1Repair2231562Context {
        label,
        fixture,
        runtime,
        peer,
        state,
        asset,
        actor,
        sequence,
        held_digest,
        approval_id,
        amount,
        expected_remaining,
        owner,
        restored,
    });
}

struct FinishBudgetAfter1Repair2231562Context {
    label: u8,
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    state: layerx_agentd::human_runtime::CoreBudgetState,
    asset: [u8; 32],
    actor: String,
    sequence: u64,
    held_digest: [u8; 32],
    approval_id: [u8; 32],
    amount: u128,
    expected_remaining: u128,
    owner: UnifiedAgentOwner<RemoteHumanAuthority>,
    restored: layerx_agentd::approval::ApprovalRecord,
}

fn finish_budget_after_1_repair_22_31_562(context: FinishBudgetAfter1Repair2231562Context) {
    let FinishBudgetAfter1Repair2231562Context {
        label,
        mut fixture,
        mut runtime,
        peer,
        state,
        asset,
        actor,
        sequence,
        held_digest,
        approval_id,
        amount,
        expected_remaining,
        mut owner,
        restored,
    } = context;

    assert_eq!(restored.held_activity.actor.as_str(), actor);
    assert_eq!(restored.canonical_bytes_digest, held_digest);
    let (mut client, server) = must(UnixStream::pair());
    must(client.set_read_timeout(Some(DEADLINE)));
    must(client.set_write_timeout(Some(DEADLINE)));
    let mut transport = must(AcceptedTransport::new(server, MAXIMUM, DEADLINE));
    let request = operation(approval_id, held_digest, sequence);
    must(client.write_all(&must(u32::try_from(request.len())).to_be_bytes()));
    must(client.write_all(&request));
    fixture.budget_label.store(label, Ordering::Release);
    must(serve_one(&mut transport, &peer, &mut owner));
    let mut prefix = [0; 4];
    must(client.read_exact(&mut prefix));
    let length = u32::from_be_bytes(prefix) as usize;
    assert!(length <= MAXIMUM);
    let mut response = vec![0; length];
    must(client.read_exact(&mut response));
    if label == layerx_types::verify::VerificationLevel::COMMITTED.wire_rank() {
        assert_eq!(response.get(..9), Some(b"LXHAGT01\x00".as_slice()));
        let facts = &response[9..];
        assert_eq!(facts.len(), 155);
        assert_eq!(&facts[..2], &2_u16.to_be_bytes());
        assert_eq!(&facts[2..34], &approval_id);
        assert_eq!(&facts[34..66], &held_digest);
        assert_eq!(
            u128::from_be_bytes(require_ok(facts[66..82].try_into(), "required test value")),
            expected_remaining
        );
        assert_eq!(facts[82], 2);
        assert_eq!(
            decode_rung(facts[82]),
            Ok(layerx_types::rung::Rung::Committed)
        );
        let mut digest = Sha256::new();
        digest.update(b"LayerX/Human/approval-budget-after/v2\0");
        digest.update(approval_id);
        digest.update(held_digest);
        digest.update(fixture.budget_id);
        digest.update(state.asset);
        digest.update(amount.to_be_bytes());
        digest.update(expected_remaining.to_be_bytes());
        digest.update(state.evidence_digest);
        digest.update(state.receipt_digest);
        digest.update(state.checkpoint_digest);
        digest.update(sequence.to_be_bytes());
        assert_eq!(&facts[83..115], digest.finalize().as_slice());
        assert_eq!(&facts[115..123], &sequence.to_be_bytes());
        assert_eq!(&facts[123..155], &asset);
    } else {
        assert_eq!(
            response,
            // approval_budget_after owns the committed-state guard outside managed_agent.
            refusal_frame(b"operation.refused"),
            "label {label}"
        );
    }
    drop(owner);
    runtime.shutdown();
    fixture.shutdown();
}

fn decode_rung(value: u8) -> Result<layerx_types::rung::Rung, ()> {
    use layerx_types::rung::Rung;
    match value {
        0 => Ok(Rung::Submitted),
        1 => Ok(Rung::Receipted),
        2 => Ok(Rung::Committed),
        _ => Err(()),
    }
}

#[test]
fn committed_budget_after_decodes_as_committed_rung() {
    budget_after(layerx_types::verify::VerificationLevel::COMMITTED.wire_rank());
}

#[test]
fn budget_state_is_committed() {
    budget_after(layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED.wire_rank());
}

fn node_contract_call(fixture: &RecordedAuthority, request: &Value, height: u64) -> Value {
    let call = &request["params"][0];
    let data = must(hex::decode(
        require_some(call["data"].as_str(), "call data").trim_start_matches("0x"),
    ));
    let mut word = [0; 32];
    let answer = if data.starts_with(&layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX) {
        assert_eq!(
            &data[4..],
            &must(hex::decode(require_some(
                fixture.agent_did.strip_prefix("did:layerx:"),
                "required test value"
            )))
        );
        word[12..].copy_from_slice(&fixture.actor_address);
        word.to_vec()
    } else if data.starts_with(&[0xb1, 0xff, 0xc0, 0x3e]) {
        assert_eq!(data.len(), 68);
        assert_eq!(request["params"][1], format!("0x{:x}", height - 1));
        assert!(
            [&fixture.budget_call[32..64], &fixture.budget_call[64..96]].contains(&&data[4..36])
        );
        assert_eq!(&data[36..68], &fixture.budget_call[96..128]);
        word[31] = 77;
        word.to_vec()
    } else if data
        .starts_with(&layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4])
    {
        assert_eq!(&data[4..], &fixture.budget_id);
        fixture.budget_call.clone()
    } else if data.starts_with(
        &layerx_crypto::evm_transaction::transaction_hash(b"budgetRemaining(bytes32)")[..4],
    ) {
        assert_eq!(&data[4..], &fixture.budget_id);
        word[31] = 77;
        word.to_vec()
    } else {
        panic!("unexpected committed read: {request}");
    };
    json!(format!("0x{}", hex::encode(answer)))
}

struct ApprovalBudgetSeed<'a> {
    fixture: &'a AuthorityFixture,
    creation_receipt: [u8; 32],
    currency: String,
    ceiling: u128,
    now: u64,
    expiry: u64,
    state: &'a layerx_agentd::human_runtime::CoreBudgetState,
    public_key: [u8; 32],
    asset: [u8; 32],
    window: u64,
}
fn approval_budget_seed(input: ApprovalBudgetSeed<'_>) -> HumanAgentLifecycleSeed {
    let ApprovalBudgetSeed {
        fixture,
        creation_receipt,
        currency,
        ceiling,
        now,
        expiry,
        state,
        public_key,
        asset,
        window,
    } = input;
    HumanAgentLifecycleSeed {
        agent_id: format!("agt_{}", hex::encode(fixture.budget_id)),
        name: "approval budget actor".to_owned(),
        purpose: "durable EVM approval".to_owned(),
        currency,
        monthly_limit: ceiling,
        period_start: now,
        period_end: expiry,
        created_at: now,
        updated_at: now,
        verified_evidence: vec![creation_receipt],
        actor: fixture.agent_did.clone(),
        primary_authority: hex::encode(public_key),
        custody_key: "approval-actor".to_owned(),
        custody_public_key: public_key,
        owner_account: format!("agent:{}:master", fixture.agent_did),
        budget_account: format!("budget:{}", hex::encode(fixture.budget_id)),
        budget_asset: asset,
        purpose_hash: Sha256::digest(b"durable EVM approval").into(),
        recovery_root: state.checkpoint_digest,
        recovery_threshold: 1,
        capability_id: [6; 32],
        activity_types: vec![7],
        counterparties: vec![[4; 32]],
        assets: vec![asset],
        amount_ceiling: ceiling,
        rate_maximum_uses: 1,
        rate_window_sequences: 20,
        purposes: vec!["durable EVM approval".to_owned()],
        capability_expiry_sequence: window,
        session_scopes: vec!["prepare".to_owned()],
        session_expiry_unix_seconds: expiry,
        protocol_grant_id: [6; 32],
        budget_period_seconds: 120,
        budget_expiry_seconds: expiry,
        initial_funding: state.remaining,
        network_id: require_ok(u32::try_from(fixture.chain_id), "protocol network"),
        creation_receipt_roots: vec![creation_receipt],
    }
}
