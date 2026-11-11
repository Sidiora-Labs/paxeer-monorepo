fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

use ed25519_dalek::Signer as _;
use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::budget::{
    create_daemon_limit, daemon_limit_id, reserve_until_core_time, BudgetLimiter, CoreTimestampMs,
    DaemonLimitRecord, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::human::{
    serve_one, AcceptedTransport, HumanAgentLifecycleSeed, HumanOperationError, HumanPeer,
    HumanPrepare, HumanSubmit,
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
    prepare_bound, PrecompileCall, PreparationDefaults, PrepareRequest,
    ProductionEvmPreparationBoundary,
};
use layerx_agentd::session::{self, OpenRequest, SessionId, SessionRegistry};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use std::os::unix::net::UnixStream;
mod support;

#[derive(Default)]
struct SequencerProgress {
    calls: usize,
    raw: Option<Vec<u8>>,
    serve_receipt: bool,
    corrupt_receipt: bool,
}

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
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

const DEADLINE: Duration = Duration::from_secs(8);
const MAXIMUM: usize = 1_048_576;
const PROTOCOL_VERSION: u16 = layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION;

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

    fn node(
        fixture: &RecordedAuthority,
        progress: Arc<Mutex<SequencerProgress>>,
    ) -> (String, Self) {
        let fault = "";
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}/", must(listener.local_addr()));
        let fixture: RecordedAuthority =
            must(serde_json::from_value(must(serde_json::to_value(fixture))));
        let native = native_store::NativeStore::for_authority(&fixture);
        assert_eq!(
            fixture.block["stateRoot"],
            format!("0x{}", hex::encode(native.app_hash))
        );
        let (height, checkpoint_reads) = recorded_checkpoint_reads(&fixture);
        let server = Self::spawn(listener, move |mut stream| {
            let (_, body) = read_http(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            must(progress.lock()).calls += 1;
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
                    verify_node_3_repair_22_31_571(fault, &native, height, &request)
                }
                "eth_estimateGas" => json!("0x186a0"),
                "eth_sendRawTransaction" => {
                    let raw = must(hex::decode(
                        require_some(request["params"][0].as_str(), "required test value")
                            .trim_start_matches("0x"),
                    ));
                    verify_node_2_repair_22_31_571(&progress, &fixture, raw)
                }
                "pax_sequencerReceipt" => {
                    let state = must(progress.lock());
                    verify_node_1_repair_22_31_571(&request, &state)
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
                "px_getReceiptKeys" if must(progress.lock()).serve_receipt => {
                    let state = must(progress.lock());
                    let (_, key) = support::sequencer_receipt(
                        state
                            .raw
                            .as_ref()
                            .unwrap_or_else(|| panic!("{}", "required test value")),
                        true,
                    );
                    json!({"keys":[{"keyId":format!("0x{}",hex::encode(key.key_id)),"publicKey":format!("0x{}",hex::encode(key.public_key)),"firstHeight":key.first_height,"lastHeight":key.last_height}]})
                }
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

    fn authority(fixture: &RecordedAuthority) -> (String, Vec<u8>, Self) {
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
        must(certificate.set_subject_name(&name));
        must(certificate.set_issuer_name(ca_certificate.subject_name()));
        must(certificate.set_pubkey(&leaf_key));
        complete_authority_1_repair_22_31_571(CompleteAuthority1Repair2231571Context {
            fixture,
            key,
            ca_certificate,
            ca,
            leaf_key,
            certificate,
        })
    }
}

fn verify_node_3_repair_22_31_571(
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

fn verify_node_2_repair_22_31_571(
    progress: &Arc<Mutex<SequencerProgress>>,
    fixture: &RecordedAuthority,
    raw: Vec<u8>,
) -> Value {
    let (call, signature) = must(layerx_crypto::evm_transaction::decode_signed(&raw));
    assert_eq!(
        must(layerx_crypto::evm_transaction::recover_sender(
            &call, &signature
        )),
        fixture.actor_address
    );
    assert_eq!(call.chain_id, fixture.chain_id);
    must(layerx_crypto::disclosure::call::bind_call(
        &call,
        fixture.actor_address,
    ));
    let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
    let mut state = must(progress.lock());
    assert!(state.raw.replace(raw).is_none(), "SUBMIT broadcasts once");
    json!(format!("0x{}", hex::encode(hash)))
}

fn verify_node_1_repair_22_31_571(
    request: &Value,
    state: &std::sync::MutexGuard<'_, SequencerProgress>,
) -> Value {
    let raw = require_some(state.raw.as_ref(), "submission before lookup");
    assert_eq!(
        request["params"],
        json!([format!(
            "0x{}",
            hex::encode(layerx_crypto::evm_transaction::transaction_hash(raw))
        )])
    );
    if state.serve_receipt {
        let (mut receipt, _) = support::sequencer_receipt(raw, true);
        if state.corrupt_receipt {
            let mut bytes = must(hex::decode(
                require_some(receipt["receipt"].as_str(), "required test value")
                    .trim_start_matches("0x"),
            ));
            *require_some(bytes.last_mut(), "required test value") ^= 1;
            receipt["receipt"] = json!(format!("0x{}", hex::encode(bytes)));
        }
        receipt
    } else {
        Value::Null
    }
}

struct CompleteAuthority1Repair2231571Context<'a> {
    fixture: &'a RecordedAuthority,
    key: openssl::pkey::PKey<openssl::pkey::Private>,
    ca_certificate: openssl::x509::X509,
    ca: Vec<u8>,
    leaf_key: openssl::pkey::PKey<openssl::pkey::Private>,
    certificate: openssl::x509::X509Builder,
}

fn complete_authority_1_repair_22_31_571(
    context: CompleteAuthority1Repair2231571Context<'_>,
) -> (String, Vec<u8>, LocalServer) {
    use openssl::asn1::Asn1Time;
    use openssl::hash::MessageDigest;

    use openssl::x509::extension::{BasicConstraints, KeyUsage, SubjectAlternativeName};

    let CompleteAuthority1Repair2231571Context {
        fixture,
        key,
        ca_certificate,
        ca,
        leaf_key,
        mut certificate,
    } = context;

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
            "/v1/agent/budget-state" => &budget,
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
    progress: Arc<Mutex<SequencerProgress>>,
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
        recorded.budget_call[8 * 32..9 * 32].copy_from_slice(&[0; 32]);
        let budget = must(
            layerx_agentd::budget::ProtocolBudgetState::decode_get_budget(
                recorded.budget_id,
                require_some(
                    recorded.budget_state["observed_head_sequence"].as_u64(),
                    "required test value",
                ),
                &recorded.budget_call,
            ),
        );
        assert_eq!(budget.record().spent_this_period, 0);
        assert_eq!(budget.record().per_period_limit, 100);
        require_some(
            recorded.registry["modules"].as_array_mut(),
            "required test value",
        )
        .push(json!({"module_id":1,"activity_types":[65541]}));
        let progress = Arc::new(Mutex::new(SequencerProgress::default()));
        let (node_rpc_url, node) = LocalServer::node(&recorded, Arc::clone(&progress));
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
        let (authority_endpoint, authority_ca_der, authority) = LocalServer::authority(&recorded);
        Self {
            recorded,
            node_rpc_url,
            authority_endpoint,
            authority_ca_der,
            servers: vec![node, authority],
            progress,
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

#[test]
fn approval_release_tracks_from_grant_through_submission_receipt_and_restart() {
    let fixture = AuthorityFixture::load();
    std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &fixture.node_rpc_url);
    std::env::set_var("LAYERX_AGENT_NODE_CHAIN_ID", fixture.chain_id.to_string());
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
    finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_7_repair_22_31_571(FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart7Repair2231571Context { fixture, creation_receipt, runtime, peer, peers, tenant, authority, registry, state, asset, currency, endpoint, public_key });
}

struct FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart7Repair2231571Context {
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
    endpoint: SequencerEndpoint,
    public_key: [u8; 32],
}

fn finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_7_repair_22_31_571(
    context: FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart7Repair2231571Context,
) {
    let FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart7Repair2231571Context { fixture, creation_receipt, runtime, peer, peers, tenant, authority, registry, state, asset, currency, endpoint, public_key } = context;

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
    let now = must(runtime.clock.sample(DEADLINE)).unix_seconds();
    let expiry = now + 120;
    let account = must(layerx_crypto::account_derivation::derive_from_seed(
        &[42; 64], 0,
    ));
    assert_eq!(account.evm_address(), address);
    let debit = layerx_crypto::send::SendDebit {
        from: [3; 32],
        to: [4; 32],
        asset,
        amount: 50,
        source_sequence: 0,
        idempotency_key: [5; 32],
        expires_at: expiry,
        context_hash: layerx_crypto::send::send_context_hash(
            &[3; 32], &[4; 32], &asset, 50, &[5; 32],
        ),
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: require_ok(u32::try_from(fixture.chain_id), "test value fits u32"),
        protocol_version: PROTOCOL_VERSION,
    };
    let signing = ed25519_dalek::SigningKey::from_bytes(account.layerx_seed());
    let canonical = must(debit.authorization_message());
    let message = must(layerx_crypto::SignatureMessage::new(
        layerx_wire::hash::Domain::SignaturePreimage,
        PROTOCOL_VERSION,
        require_ok(u32::try_from(fixture.chain_id), "test value fits u32"),
        &canonical,
    ));
    let payload = must(debit.encode_signed(public_key, signing.sign(&message.digest()).to_bytes()));
    let evm = must(prepare_bound(
        observed,
        PreparationDefaults {
            max_priority_fee_per_gas: 0,
            max_fee_per_gas: observed.base_fee_per_gas * 2,
            maximum_payload_bytes: MAXIMUM,
        },
        PrepareRequest {
            from: address,
            call: PrecompileCall::ledger_transfer(payload.clone()),
            expected_nonce: Some(observed.pending_nonce),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: MAXIMUM,
        },
        100_000,
    ));
    assert_eq!(evm.disclosure.actor, address);
    assert_eq!(evm.disclosure.asset, asset);
    let bytes = evm.call.encode_unsigned();
    let held_digest: [u8; 32] = Sha256::digest(&bytes).into();

    finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_6_repair_22_31_571(FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart6Repair2231571Context { fixture, creation_receipt, runtime, peer, peers, tenant, authority, registry, state, asset, currency, endpoint, public_key, address, actor, sequence, observed, now, expiry, account, payload, evm, bytes, held_digest });
}

struct FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart6Repair2231571Context {
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
    endpoint: SequencerEndpoint,
    public_key: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    observed: layerx_agentd::prepare::EvmPreparationState,
    now: u64,
    expiry: u64,
    account: layerx_crypto::account_derivation::DerivedAccount,
    payload: Vec<u8>,
    evm: layerx_agentd::prepare::Prepared,
    bytes: Vec<u8>,
    held_digest: [u8; 32],
}

fn finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_6_repair_22_31_571(
    context: FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart6Repair2231571Context,
) {
    let FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart6Repair2231571Context { fixture, creation_receipt, runtime, peer, peers, tenant, mut authority, registry, state, asset, currency, endpoint, public_key, address, actor, sequence, observed, now, expiry, account, payload, evm, bytes, held_digest } = context;

    let window = require_some(sequence.checked_add(20), "held sequence deadline");
    let approval_id = [5; 32];
    let counterparty = must(AgentDid::new(hex::encode([4; 32])));
    let prepared = Prepared {
        preparation_ref: must(PreparationRef::new(hex::encode(approval_id))),
        unsigned_canonical_bytes: must(CanonicalBytes::new(bytes)),
        signing_preimage: must(SigningPreimage::new(evm.signing_hash.to_vec())),
        disclosure: Disclosure {
            canonical_digest: held_digest,
            activity_type: ActivityType(5),
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
            permitted_activity_types: BTreeSet::from([5]),
            scopes: BTreeSet::from(["prepare".to_owned()]),
            expiry_sequence: window,
            expiry_seconds: Some(expiry),
            opening_client: "human-approval-budget-actor".to_owned(),
            policy_version: "approval-actor-v1".to_owned(),
        },
        sequence,
    ));
    finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_5_repair_22_31_571(FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart5Repair2231571Context { fixture, creation_receipt, runtime, peer, peers, tenant, registry, state, asset, currency, endpoint, public_key, address, actor, sequence, observed, now, expiry, account, payload, evm, held_digest, window, approval_id, prepared, amount, ceiling, store_path, store, sessions, token });
}

struct FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart5Repair2231571Context {
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
    endpoint: SequencerEndpoint,
    public_key: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    observed: layerx_agentd::prepare::EvmPreparationState,
    now: u64,
    expiry: u64,
    account: layerx_crypto::account_derivation::DerivedAccount,
    payload: Vec<u8>,
    evm: layerx_agentd::prepare::Prepared,
    held_digest: [u8; 32],
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    ceiling: u128,
    store_path: PathBuf,
    store: Store,
    sessions: SessionRegistry,
    token: session::Token,
}

fn finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_5_repair_22_31_571(
    context: FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart5Repair2231571Context,
) {
    let FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart5Repair2231571Context { fixture, creation_receipt, runtime, peer, peers, tenant, registry, state, asset, currency, endpoint, public_key, address, actor, sequence, observed, now, expiry, account, payload, evm, held_digest, window, approval_id, prepared, amount, ceiling, store_path, store, sessions, token } = context;

    assert_eq!(token.credential().generation(), 1);
    let seed = HumanAgentLifecycleSeed {
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
        activity_types: vec![5],
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
    };
    let agent = must(ManagedAgent::from_creation(
        &seed,
        &fixture.agent_did,
        [7; 32],
        [8; 32],
        1,
    ));
    finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_4_repair_22_31_571(FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart4Repair2231571Context { fixture, runtime, peer, peers, tenant, registry, asset, endpoint, public_key, address, actor, sequence, observed, now, expiry, account, payload, evm, held_digest, window, approval_id, prepared, amount, ceiling, store_path, store, sessions, agent });
}

struct FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart4Repair2231571Context {
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    registry: layerx_types::payload::ModuleRegistry,
    asset: [u8; 32],
    endpoint: SequencerEndpoint,
    public_key: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    observed: layerx_agentd::prepare::EvmPreparationState,
    now: u64,
    expiry: u64,
    account: layerx_crypto::account_derivation::DerivedAccount,
    payload: Vec<u8>,
    evm: layerx_agentd::prepare::Prepared,
    held_digest: [u8; 32],
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    ceiling: u128,
    store_path: PathBuf,
    store: Store,
    sessions: SessionRegistry,
    agent: ManagedAgent,
}

fn finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_4_repair_22_31_571(
    context: FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart4Repair2231571Context,
) {
    let FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart4Repair2231571Context { fixture, runtime, peer, peers, tenant, registry, asset, endpoint, public_key, address, actor, sequence, observed, now, expiry, account, payload, evm, held_digest, window, approval_id, mut prepared, amount, ceiling, store_path, mut store, sessions, agent } = context;

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
    let shared = Arc::new(Mutex::new(store));
    let mut owner = runtime.owner(&fixture, &peers, Arc::clone(&shared));
    let core_time = require_some(
        require_some(
            must(endpoint.block_by_number(sequence)),
            "required test value",
        )
        .timestamp
        .checked_mul(1_000),
        "protocol timestamp in milliseconds",
    );
    let activity = must(layerx_types::payload::ActivityType::new(
        layerx_types::payload::ModuleId::Asset,
        5,
    ));
    let payload_hash = must(layerx_wire::hash::payload_hash_for(&must(
        layerx_types::payload::Payload::new(&registry, activity, &payload),
    )));
    let request = HumanPrepare {
        activity_type: activity.value(),
        actor: fixture.agent_did.clone(),
        authority: hex::encode(public_key),
        nonce: observed.pending_nonce,
        not_before_height: core_time,
        not_after_height: core_time + 30,
        idempotency_key: hex::encode([5; 32]),
        fee_limit: 1_000_000,
        payload,
        payload_hash,
        capability_id: None,
    };
    let response = exchange(&mut owner, &peer, &prepare_frame(&request));
    assert_eq!(&response[..9], b"LXHAGT01\0", "PREPARE: {response:?}");
    let mut decoder = layerx_wire::decode::Decoder::new(&response[9..], MAXIMUM);
    let preparation_ref = must(decoder.text(1024)).to_owned();
    prepared.preparation_ref = must(PreparationRef::new(&preparation_ref));
    finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_3_repair_22_31_571(FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart3Repair2231571Context { fixture, runtime, peer, peers, tenant, public_key, address, actor, sequence, now, expiry, account, evm, held_digest, window, approval_id, prepared, amount, store_path, limit, shared, owner, preparation_ref });
}

struct FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart3Repair2231571Context {
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    public_key: [u8; 32],
    address: [u8; 20],
    actor: String,
    sequence: u64,
    now: u64,
    expiry: u64,
    account: layerx_crypto::account_derivation::DerivedAccount,
    evm: layerx_agentd::prepare::Prepared,
    held_digest: [u8; 32],
    window: u64,
    approval_id: [u8; 32],
    prepared: Prepared,
    amount: u128,
    store_path: PathBuf,
    limit: DaemonLimitRecord,
    shared: Arc<Mutex<Store>>,
    owner: UnifiedAgentOwner<RemoteHumanAuthority>,
    preparation_ref: String,
}

fn finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_3_repair_22_31_571(
    context: FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart3Repair2231571Context,
) {
    let FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart3Repair2231571Context { fixture, runtime, peer, peers, tenant, public_key, address, actor, sequence, now, expiry, account, evm, held_digest, window, approval_id, prepared, amount, store_path, limit, shared, mut owner, preparation_ref } = context;

    let preparation_id: [u8; 32] = require_ok(
        must(hex::decode(&preparation_ref)).try_into(),
        "required test value",
    );
    let retained = {
        let store = must(shared.lock());
        must(layerx_agentd::sign::SigningPreparation::restore(
            &store,
            tenant.clone(),
            preparation_id,
        ))
    };
    assert_eq!(
        retained.transaction.call, evm.call,
        "hold the exact PREPARE output"
    );
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
        prepared.clone(),
        sequence,
        window,
        &reservation,
        runtime.clock.as_ref(),
    ));
    assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    let granted = exchange(
        &mut owner,
        &peer,
        &approve_frame(approval_id, held_digest, sequence),
    );
    assert_eq!(&granted[..11], b"LXHAGT01\0\0\x01", "APPROVE: {granted:?}");
    let reference: [u8; 32] = require_ok(granted[11..43].try_into(), "required test value");
    assert_eq!(owner.approval_queue.prepared(reference), Some(prepared));
    let calls = must(fixture.progress.lock()).calls;
    let queued = exchange(&mut owner, &peer, &track_frame(reference));
    assert_eq!(&queued[..9], b"LXHAGT01\0");
    assert_eq!(&queued[9..41], &held_digest);
    assert_ne!(&queued[9..41], &[0; 32]);
    assert_eq!(&queued[41..73], &reference);
    assert_eq!(queued[73], 2, "Queued");
    assert_eq!(queued.last(), Some(&0), "no receipt before submission");
    assert_eq!(
        must(fixture.progress.lock()).calls,
        calls,
        "queued TRACK must not read the node"
    );
    finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_2_repair_22_31_571(FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart2Repair2231571Context { fixture, runtime, peer, peers, tenant, public_key, account, evm, approval_id, store_path, shared, owner, preparation_ref, reference, calls });
}

struct FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart2Repair2231571Context {
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    tenant: TenantId,
    public_key: [u8; 32],
    account: layerx_crypto::account_derivation::DerivedAccount,
    evm: layerx_agentd::prepare::Prepared,
    approval_id: [u8; 32],
    store_path: PathBuf,
    shared: Arc<Mutex<Store>>,
    owner: UnifiedAgentOwner<RemoteHumanAuthority>,
    preparation_ref: String,
    reference: [u8; 32],
    calls: usize,
}

fn finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_2_repair_22_31_571(
    context: FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart2Repair2231571Context,
) {
    let FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart2Repair2231571Context { fixture, runtime, peer, peers, tenant, public_key, account, evm, approval_id, store_path, shared, mut owner, preparation_ref, reference, calls } = context;

    assert_refused(&exchange(&mut owner, &peer, &track_frame([0xfe; 32])));
    let other_tenant = HumanPeer {
        tenant: "another-tenant".to_owned(),
        ..peer.clone()
    };
    assert_refused(&exchange(
        &mut owner,
        &other_tenant,
        &track_frame(reference),
    ));
    assert_eq!(must(fixture.progress.lock()).calls, calls);

    let key = must(k256::ecdsa::SigningKey::from_slice(
        account
            .evm_secret()
            .unwrap_or_else(|| panic!("{}", "required test value")),
    ));
    let (signature, recovery) = must(key.sign_prehash_recoverable(&evm.signing_hash));
    let mut signature_bytes = signature.to_bytes().to_vec();
    signature_bytes.push(27 + recovery.to_byte());
    let submit = HumanSubmit {
        preparation_ref,
        signature: signature_bytes,
        signer_public_key: public_key,
        approval_release_ref: Some(reference),
    };
    let submitted = exchange(&mut owner, &peer, &submit_frame(&submit));
    assert_eq!(&submitted[..9], b"LXHAGT01\0", "SUBMIT: {submitted:?}");
    let raw = require_some(
        must(fixture.progress.lock()).raw.clone(),
        "real submitted EVM bytes",
    );
    let submission_id = layerx_crypto::evm_transaction::transaction_hash(&raw);
    let mut human = peer.clone();
    "human-observer".clone_into(&mut human.principal);
    assert_refused(&exchange(&mut owner, &human, &track_frame(submission_id)));
    let linked = exchange(&mut owner, &human, &track_frame(reference));
    let direct = exchange(&mut owner, &peer, &track_frame(submission_id));
    assert_eq!(&linked[..41], &submitted[..41]);
    assert_eq!(
        &linked[41..linked.len() - 1],
        &direct[9..],
        "linked TRACK preserves the submission status"
    );
    assert_eq!(linked[73], 3, "Submitted");
    let release_key = must(TenantKey::new(
        tenant.clone(),
        ObjectKind::Outbox,
        [b"approval-released-v1:".as_slice(), &approval_id].concat(),
    ));
    let linked_record = require_some(must(shared.lock()).get(&release_key), "required test value")
        .bytes()
        .to_vec();
    assert!(linked_record.starts_with(b"LXAPREL2"));
    drop(owner);
    drop(shared);
    let shared = Arc::new(Mutex::new(must(Store::open(&store_path))));
    let mut owner = runtime.owner(&fixture, &peers, Arc::clone(&shared));
    let direct = exchange(&mut owner, &peer, &track_frame(submission_id));
    let restored = exchange(&mut owner, &human, &track_frame(reference));
    assert_eq!(&restored[..41], &linked[..41]);
    assert_eq!(&restored[41..73], &submission_id);
    assert_eq!(restored[73], 5, "Unknown after restart without a receipt");
    finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_1_repair_22_31_571(FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart1Repair2231571Context { fixture, runtime, peer, peers, store_path, reference, other_tenant, raw, submission_id, human, release_key, linked_record, shared, owner, direct, restored });
}

struct FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart1Repair2231571Context {
    fixture: AuthorityFixture,
    runtime: Runtime,
    peer: HumanPeer,
    peers: BTreeMap<u32, (String, String)>,
    store_path: PathBuf,
    reference: [u8; 32],
    other_tenant: HumanPeer,
    raw: Vec<u8>,
    submission_id: [u8; 32],
    human: HumanPeer,
    release_key: TenantKey,
    linked_record: Vec<u8>,
    shared: Arc<Mutex<Store>>,
    owner: UnifiedAgentOwner<RemoteHumanAuthority>,
    direct: Vec<u8>,
    restored: Vec<u8>,
}

fn finish_approval_release_tracks_from_grant_through_submission_receipt_and_restart_1_repair_22_31_571(
    context: FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart1Repair2231571Context,
) {
    let FinishApprovalReleaseTracksFromGrantThroughSubmissionReceiptAndRestart1Repair2231571Context { mut fixture, mut runtime, peer, peers, store_path, reference, other_tenant, raw, submission_id, human, release_key, linked_record, shared, mut owner, direct, restored } = context;

    assert_eq!(
        &restored[41..restored.len() - 1],
        &direct[9..],
        "linked TRACK preserves the recovered submission status"
    );
    assert_eq!(restored.last(), Some(&0), "no receipt after restart");
    assert_eq!(
        require_some(must(shared.lock()).get(&release_key), "required test value").bytes(),
        linked_record
    );

    must(fixture.progress.lock()).serve_receipt = true;
    must(fixture.progress.lock()).corrupt_receipt = true;
    assert_refused(&exchange(&mut owner, &human, &track_frame(reference)));
    must(fixture.progress.lock()).corrupt_receipt = false;
    let receipted = exchange(&mut owner, &human, &track_frame(reference));
    let direct = exchange(&mut owner, &peer, &track_frame(submission_id));
    assert_eq!(&receipted[..9], b"LXHAGT01\0");
    assert_eq!(&receipted[9..41], &submission_id);
    assert_eq!(&receipted[41..73], &submission_id);
    assert_eq!(receipted[73], 6, "Executed");
    assert_eq!(&receipted[41..41 + direct.len() - 9], &direct[9..]);
    let frame = must(
        layerx_agentd::human_receipt_frame::ReceiptLookupAnswer::decode(
            &receipted[41 + direct.len() - 9..],
        ),
    );
    let layerx_agentd::human_receipt_frame::ReceiptLookupAnswer::Found(frame) = frame else {
        panic!("verified receipt required")
    };
    let (served, key) = support::sequencer_receipt(&raw, true);
    let canonical = must(hex::decode(
        require_some(served["receipt"].as_str(), "required test value").trim_start_matches("0x"),
    ));
    assert_eq!(frame.canonical_bytes, canonical);
    assert_eq!(frame.tx_hash, submission_id);
    assert_eq!(frame.public_key, key.public_key);
    assert_eq!(frame.rung, layerx_types::rung::Rung::Receipted);
    assert_eq!(
        frame.verification_level,
        layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED
    );
    must(layerx_agentd::finality::VerifiedReceipt::verify(
        submission_id,
        &frame.signed_receipt,
        &[key],
    ));
    assert_refused(&exchange(
        &mut owner,
        &other_tenant,
        &track_frame(reference),
    ));
    drop(owner);
    drop(shared);
    let shared = Arc::new(Mutex::new(must(Store::open(&store_path))));
    let mut owner = runtime.owner(&fixture, &peers, Arc::clone(&shared));
    assert_eq!(
        exchange(&mut owner, &human, &track_frame(reference)),
        receipted,
        "verified receipt survives restart"
    );
    drop(owner);
    drop(shared);
    runtime.shutdown();
    fixture.shutdown();
}

fn field(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(&must(u32::try_from(value.len())).to_be_bytes());
    out.extend_from_slice(value);
}

fn exchange(
    owner: &mut UnifiedAgentOwner<RemoteHumanAuthority>,
    peer: &HumanPeer,
    frame: &[u8],
) -> Vec<u8> {
    let (mut client, server) = must(UnixStream::pair());
    must(client.set_read_timeout(Some(DEADLINE)));
    let mut transport = must(AcceptedTransport::new(server, MAXIMUM, DEADLINE));
    must(client.write_all(&must(u32::try_from(frame.len())).to_be_bytes()));
    must(client.write_all(frame));
    must(serve_one(&mut transport, peer, owner));
    let mut size = [0; 4];
    must(client.read_exact(&mut size));
    let size = u32::from_be_bytes(size) as usize;
    assert!(size <= MAXIMUM);
    let mut response = vec![0; size];
    must(client.read_exact(&mut response));
    response
}

fn track_frame(reference: [u8; 32]) -> Vec<u8> {
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(layerx_agentd::human::TRACK);
    field(&mut frame, hex::encode(reference).as_bytes());
    frame
}

fn approve_frame(id: [u8; 32], digest: [u8; 32], sequence: u64) -> Vec<u8> {
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(layerx_agentd::human::APPROVAL_APPROVE);
    frame.extend_from_slice(&id);
    frame.extend_from_slice(&digest);
    field(&mut frame, b"grant-22-31-571");
    frame.extend_from_slice(&sequence.to_be_bytes());
    frame
}

fn prepare_frame(request: &HumanPrepare) -> Vec<u8> {
    let mut digest = Sha256::new();
    let text = |digest: &mut Sha256, value: &[u8]| {
        digest.update(must(u32::try_from(value.len())).to_be_bytes());
        digest.update(value);
    };
    digest.update(b"layerx-human-journey-prepare/v1");
    digest.update(request.activity_type.to_be_bytes());
    text(&mut digest, request.actor.as_bytes());
    text(&mut digest, request.authority.as_bytes());
    digest.update(request.nonce.to_be_bytes());
    digest.update(request.not_before_height.to_be_bytes());
    digest.update(request.not_after_height.to_be_bytes());
    text(&mut digest, request.idempotency_key.as_bytes());
    digest.update(request.fee_limit.to_be_bytes());
    digest.update(request.payload_hash);
    digest.update(Sha256::digest(&request.payload));
    let digest: [u8; 32] = digest.finalize().into();
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(layerx_agentd::human::PREPARE);
    frame.extend_from_slice(&1u64.to_be_bytes());
    frame.extend_from_slice(&[21; 32]);
    frame.extend_from_slice(&digest);
    frame.extend_from_slice(&request.activity_type.to_be_bytes());
    field(&mut frame, request.actor.as_bytes());
    field(&mut frame, request.authority.as_bytes());
    frame.extend_from_slice(&request.nonce.to_be_bytes());
    frame.extend_from_slice(&request.not_before_height.to_be_bytes());
    frame.extend_from_slice(&request.not_after_height.to_be_bytes());
    field(&mut frame, request.idempotency_key.as_bytes());
    frame.extend_from_slice(&request.fee_limit.to_be_bytes());
    field(&mut frame, &request.payload);
    frame.extend_from_slice(&request.payload_hash);
    frame
}

fn submit_frame(request: &HumanSubmit) -> Vec<u8> {
    let mut digest = Sha256::new();
    digest.update(b"layerx-human-journey-submit/v1");
    digest.update(must(u32::try_from(request.preparation_ref.len())).to_be_bytes());
    digest.update(request.preparation_ref.as_bytes());
    digest.update(Sha256::digest(&request.signature));
    digest.update(request.signer_public_key);
    digest.update([1]);
    digest.update(require_some(
        request.approval_release_ref,
        "required test value",
    ));
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(layerx_agentd::human::SUBMIT);
    frame.extend_from_slice(&2u64.to_be_bytes());
    frame.extend_from_slice(&[22; 32]);
    frame.extend_from_slice(&digest.finalize());
    field(&mut frame, request.preparation_ref.as_bytes());
    field(&mut frame, &request.signature);
    frame.extend_from_slice(&request.signer_public_key);
    frame.push(1);
    frame.extend_from_slice(&require_some(
        request.approval_release_ref,
        "required test value",
    ));
    frame
}

fn assert_refused(response: &[u8]) {
    // TRACK authorization and release lookup refuse outside managed_agent.
    assert_eq!(
        response,
        layerx_agentd::human::encode_response(Err(HumanOperationError::Refused))
    );
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

fn recorded_checkpoint_reads(fixture: &RecordedAuthority) -> (u64, Vec<Value>) {
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
    (height, checkpoint_reads)
}
