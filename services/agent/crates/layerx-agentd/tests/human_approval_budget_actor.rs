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
use layerx_client::frame::transport::{ConnectionGate, FrameTransport, Limits, Uds};
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
use std::os::unix::net::UnixListener;
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

fn record_authority_budget_state(url: String, fixture: &RecordedAuthority) -> Value {
    use layerx_platform_authority::{
        budget_state, human_policy::PrincipalPolicy, BudgetStateConfig,
    };

    let config = BudgetStateConfig {
        chain_rpc_url: url,
        protocol_network_id: fixture.chain_id as u32,
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
        "account_id":hex::encode([2;32]), "asset_id":hex::encode([4;32]),
        "activities":[], "budgets":[hex::encode(fixture.budget_id)],
        "identities":[{
            "did":fixture.agent_did, "frozen":false, "authorities":[],
            "revocation_sequence":u64::from_be_bytes(fixture.budget_call[13 * 32 + 24..14 * 32].try_into().expect("canonical budget revocation sequence")),
            "evidence":evidence, "capabilities":[], "rotation":key_policy, "recovery":key_policy,
        }],
        "maximum_age_seconds":315_360_000_u64, "maximum_age_sequences":100,
    })));
    let response = must(budget_state::read(
        &config,
        &policy,
        &hex::encode(fixture.budget_id),
    ));
    assert_eq!(response.status, 200);
    must(serde_json::from_slice(&response.body))
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
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}/", must(listener.local_addr()));
        let fixture: RecordedAuthority =
            must(serde_json::from_value(must(serde_json::to_value(fixture))));
        let server = Self::spawn(listener, move |mut stream| {
            let (_, body) = read_http(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            if request["method"] == "px_getCommit" {
                write_http(
                    &mut stream,
                    &json!({"jsonrpc":"2.0","id":request["id"],"error":{"code":-32001,"message":"recorded budget facts contain no checkpoint recovery proof"}}),
                );
                return;
            }
            let mut account = json!({
                "evm_address":format!("0x{}",hex::encode(fixture.actor_address)),
                "pax_address":null,"layerx_did":fixture.agent_did,
                "layerx_account":hex::encode([2;32]),"bound":true,
            });
            let result = match request["method"].as_str().expect("RPC method") {
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
                "eth_call" => {
                    let call = &request["params"][0];
                    let data = must(hex::decode(
                        call["data"]
                            .as_str()
                            .expect("call data")
                            .trim_start_matches("0x"),
                    ));
                    let mut word = [0; 32];
                    let answer = if data
                        .starts_with(&layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX)
                    {
                        assert_eq!(
                            &data[4..],
                            &must(hex::decode(
                                fixture.agent_did.strip_prefix("did:layerx:").unwrap()
                            ))
                        );
                        word[12..].copy_from_slice(&fixture.actor_address);
                        word.to_vec()
                    } else if data.starts_with(
                        &layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")
                            [..4],
                    ) {
                        assert_eq!(&data[4..], &fixture.budget_id);
                        fixture.budget_call.clone()
                    } else if data.starts_with(
                        &layerx_crypto::evm_transaction::transaction_hash(
                            b"budgetRemaining(bytes32)",
                        )[..4],
                    ) {
                        assert_eq!(&data[4..], &fixture.budget_id);
                        word[31] = 77;
                        word.to_vec()
                    } else {
                        panic!("unexpected committed read: {request}");
                    };
                    json!(format!("0x{}", hex::encode(answer)))
                }
                "px_getReceiptKeys" => {
                    json!({"keys":[{"keyId":format!("0x{}",hex::encode([1;32])),"publicKey":format!("0x{}",fixture.balance_context["sequencer_public_key"].as_str().unwrap()),"firstHeight":1,"lastHeight":65535}]})
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
            "canonical_core_bytes":fixture.agent_did.strip_prefix("did:layerx:").unwrap(),
            "head_sequence":fixture.budget_state["observed_head_sequence"],
            "revocation_sequence":fixture.budget_state["revocation_sequence"],
            "verification_level":"checkpoint_finalised", "frozen":false,
            "authorities":[{"kind":"primary_key","id":fixture.agent_did.strip_prefix("did:layerx:").unwrap()}]
        });
        let server = Self::spawn(listener, move |stream| {
            let mut stream = must(acceptor.accept(stream));
            let (headers, _) = read_http(&mut stream);
            assert!(headers.contains("Bearer approval-budget-loopback-authority"));
            let path = headers
                .lines()
                .next()
                .unwrap()
                .split_whitespace()
                .nth(1)
                .unwrap();
            let response = match path.split('?').next().unwrap() {
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
}

impl Drop for LocalServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
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
    let bytes = must(hex::decode(fixture["receipt"].as_str().unwrap()));
    let header_bytes = must(hex::decode(fixture["header"].as_str().unwrap()));
    let header = must(layerx_wire::receipt::decode_batch_header(&header_bytes));
    let public = must(hex::decode32(
        fixture["sequencer_public_key"].as_str().unwrap(),
    ));
    let signature =
        must(must(hex::decode(fixture["header_signature"].as_str().unwrap())).try_into());
    let stored = must(hex::decode(fixture["receipt_proof"].as_str().unwrap()));
    assert_eq!(stored.len(), 9 + usize::from(stored[0]) * 32);
    let proof = must(layerx_proof::merkle::Proof::new(
        u32::from_be_bytes(stored[1..5].try_into().unwrap()),
        u32::from_be_bytes(stored[5..9].try_into().unwrap()),
        stored[9..]
            .chunks_exact(32)
            .map(|bytes| bytes.try_into().unwrap())
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
    let protocol = receipt.protocol().expect("managed protocol receipt");
    assert_eq!(protocol.module_id(), 7);
    assert_eq!(protocol.result_code(), 0);
    Sha256::digest(&bytes).into()
}

#[test]
#[ignore = "records the production Human authority response into this source fixture"]
fn record_human_approval_budget_actor_authority() {
    let vectors: Value = must(serde_json::from_slice(include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../tests/vectors/native-budget-records.json"
    ))));
    let native = must(hex::decode(
        vectors["v1"].as_str().unwrap().strip_prefix("0x").unwrap(),
    ));
    let record = must(layerx_agentd::budget::ProtocolBudgetRecord::decode(&native));
    let mut words = vec![[0; 32]; 20 + record.delegates.len()];
    words[0][31] = 32;
    for (index, value) in [
        (1, record.owner),
        (2, record.budget_account),
        (3, record.asset_id),
        (4, record.purpose_hash),
    ] {
        words[index] = value;
    }
    for (index, value) in [
        (5, record.per_period_limit),
        (6, record.configured_period_limit),
        (7, record.carry_cap),
        (8, record.spent_this_period),
        (9, record.carried),
    ] {
        words[index][16..].copy_from_slice(&value.to_be_bytes());
    }
    for (index, value) in [
        (10, record.period_length),
        (11, record.period_start),
        (12, record.expiry),
        (13, record.revocation_sequence),
    ] {
        words[index][24..].copy_from_slice(&value.to_be_bytes());
    }
    words[14][31] = record.rollover_policy;
    words[15][31] = u8::from(record.closed);
    words[16][31] = u8::from(record.revoked);
    words[17][24..].copy_from_slice(&576u64.to_be_bytes());
    words[19][24..].copy_from_slice(&(record.delegates.len() as u64).to_be_bytes());
    for (word, delegate) in words[20..].iter_mut().zip(&record.delegates) {
        *word = *delegate;
    }
    let receipts: Value = must(serde_json::from_slice(include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../platform/hosted/authority/tests/fixtures/native-module-outcomes.json"
    ))));
    let managed_receipt = receipts["receipts"]
        .as_array()
        .unwrap()
        .iter()
        .find(|receipt| receipt["module_id"] == 7 && receipt["result_code"] == 0)
        .unwrap()
        .clone();
    verified_managed_receipt(&managed_receipt);
    let header_bytes = must(hex::decode(managed_receipt["header"].as_str().unwrap()));
    let header = must(layerx_wire::receipt::decode_batch_header(&header_bytes));
    let account = must(layerx_crypto::account_derivation::derive_from_seed(
        &[42; 64], 0,
    ));
    let public = account.layerx_public_key();
    let mut fixture = RecordedAuthority {
        chain_id: 125,
        tenant: "approval-actor-tenant".to_owned(),
        principal: "approval-actor-owner".to_owned(),
        agent_did: format!("did:layerx:{}", hex::encode(public)),
        budget_id: record.budget_id,
        actor_address: account.evm_address(),
        budget_call: words.concat(),
        budget_state: Value::Null,
        registry: json!({"modules":[{"module_id":2,"activity_types":[131073,131079]}]}),
        balance_context: json!({"account_id":hex::encode(record.owner),"asset_id":hex::encode(record.asset_id),"currency":"USD","observed_at":"recorded committed block","age_seconds":0,"maximum_age_seconds":315360000,"sequencer_id":hex::encode(header.sequencer_id()),"sequencer_public_key":managed_receipt["sequencer_public_key"],"first_batch_number":1,"last_batch_number":65535}),
        block: json!({"number":"0x40","hash":format!("0x{}",hex::encode(Sha256::digest(&header_bytes))),"parentHash":format!("0x{}",hex::encode(header.previous_state_root())),"timestamp":format!("{:#x}",header.timestamp_ms()/1000),"transactionsRoot":format!("0x{}",hex::encode([17;32])),"receiptsRoot":format!("0x{}",hex::encode([18;32])),"stateRoot":format!("0x{}",hex::encode(header.resulting_state_root())),"baseFeePerGas":"0x0","transactions":[]}),
        managed_receipt,
    };
    let (url, server) = LocalServer::node(&fixture);
    fixture.budget_state = record_authority_budget_state(url, &fixture);
    assert_eq!(
        fixture.budget_state["remaining"],
        record.remaining().to_string()
    );
    assert_eq!(
        fixture.budget_state["verification"],
        layerx_types::verify::VerificationLevel::COMMITTED.wire_rank()
    );
    drop(server);
    let path = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/human_approval_budget_actor.rs");
    let source = must(fs::read_to_string(&path));
    let start = source
        .find(concat!("/* RECORDED_", "AUTHORITY\n"))
        .expect("fixture start");
    let end = source[start..]
        .find(concat!("\nEND_RECORDED_", "AUTHORITY */"))
        .unwrap()
        + start;
    let mut recorded = source[..start + "/* RECORDED_AUTHORITY\n".len()].to_owned();
    recorded.push_str(&must(serde_json::to_string_pretty(&fixture)));
    recorded.push_str(&source[end..]);
    must(fs::write(path, recorded));
}

struct AuthorityFixture {
    recorded: RecordedAuthority,
    authority_endpoint: String,
    authority_ca_der: Vec<u8>,
    node_rpc_url: String,
    _servers: Vec<LocalServer>,
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
        let start = source
            .find(concat!("/* RECORDED_", "AUTHORITY\n"))
            .expect("recorded authority start");
        let data = &source[start + "/* RECORDED_AUTHORITY\n".len()..];
        let end = data
            .find(concat!("\nEND_RECORDED_", "AUTHORITY */"))
            .expect("recorded authority end");
        let recorded: RecordedAuthority = must(serde_json::from_str(&data[..end]));
        let (node_rpc_url, node) = LocalServer::node(&recorded);
        let (authority_endpoint, authority_ca_der, authority) = LocalServer::authority(&recorded);
        Self {
            recorded,
            node_rpc_url,
            authority_endpoint,
            authority_ca_der,
            _servers: vec![node, authority],
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
                u32::try_from(fixture.chain_id).expect("protocol network"),
                must(fs::metadata(&self.root)).uid(),
            )),
        ))
    }
}

impl Drop for Runtime {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
        must(fs::remove_dir_all(&self.root));
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

fn exchange(socket: &Path, request: &[u8]) -> Vec<u8> {
    let mut transport = must(Uds::connect(
        socket,
        &ConnectionGate::new(1),
        Limits {
            maximum_frame_bytes: MAXIMUM,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: MAXIMUM,
            deadline: DEADLINE,
        },
    ));
    must(transport.send(request));
    must(transport.receive())
}

fn refusal_frame(reason: &[u8]) -> Vec<u8> {
    fn field(bytes: &mut Vec<u8>, value: &[u8]) {
        bytes.extend_from_slice(
            &u32::try_from(value.len())
                .expect("bounded field")
                .to_be_bytes(),
        );
        bytes.extend_from_slice(value);
    }
    let mut frame = b"LXHAGT01\x01".to_vec();
    field(&mut frame, reason);
    frame
}

#[test]
fn human_approval_budget_actor_serves_evm_holds_and_refuses_changed_bindings() {
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
    let public_key: [u8; 32] = must(hex::decode(
        fixture
            .agent_did
            .strip_prefix("did:layerx:")
            .expect("managed DID"),
    ))
    .try_into()
    .expect("managed identity public key");
    let address = must(endpoint.bound_address(&public_key)).expect("actual bound signing address");
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
            max_fee_per_gas: observed
                .base_fee_per_gas
                .checked_mul(2)
                .unwrap()
                .checked_add(0)
                .unwrap(),
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
    assert_eq!(evm.disclosure.actor, address);
    assert_eq!(evm.disclosure.asset, asset);
    let bytes = evm.call.encode_unsigned();
    let held_digest: [u8; 32] = Sha256::digest(&bytes).into();
    let now = must(runtime.clock.sample(DEADLINE)).unix_seconds();
    let expiry = now.checked_add(120).expect("held wall deadline");
    let window = sequence.checked_add(20).expect("held sequence deadline");
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
    let amount = evm
        .disclosure
        .fee_limit
        .checked_add(50)
        .expect("held charge");
    let ceiling = amount.checked_add(1_000).expect("daemon ceiling");
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
        network_id: u32::try_from(fixture.chain_id).expect("protocol network"),
        creation_receipt_roots: vec![creation_receipt],
    };
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
        u32::try_from(fixture.agent_did.len())
            .unwrap()
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
            expiry_ms: expiry.checked_mul(1_000).unwrap(),
            revoked: false,
            mutation_key: [10; 32],
            body_digest: [11; 32],
            revoke_key: [0; 32],
        },
        CoreTimestampMs(now.checked_mul(1_000).unwrap()),
    ));
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
        CoreTimestampMs(expiry.checked_mul(1_000).unwrap()),
        CoreTimestampMs(now.checked_mul(1_000).unwrap()),
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
        must(shared.lock())
            .get(&hold_key)
            .expect("durable held call")
            .class(),
        StorageClass::LocalOnly
    );
    drop(owner);
    drop(shared);
    let shared = Arc::new(Mutex::new(must(Store::open(&store_path))));
    let mut owner = runtime.owner(&fixture, &peers, shared);
    let restored = must(
        ApprovalService::new(&owner.approvals, &owner.budgets, &owner.approval_expiry).get(
            &tenant,
            approval_id,
            sequence,
            TimestampSeconds(must(runtime.clock.sample(DEADLINE)).unix_seconds()),
        ),
    );
    assert_eq!(restored.held_activity.actor.as_str(), actor);
    assert_eq!(restored.canonical_bytes_digest, held_digest);
    let socket = runtime.root.join("human.sock");
    let listener = must(UnixListener::bind(&socket));
    let worker = thread::spawn(move || {
        for _ in 0..3 {
            let (stream, _) = must(listener.accept());
            let credentials = must(rustix::net::sockopt::socket_peercred(&stream));
            assert_eq!(credentials.uid.as_raw(), peer.uid);
            let mut transport = must(AcceptedTransport::new(stream, MAXIMUM, DEADLINE));
            must(serve_one(&mut transport, &peer, &mut owner));
        }
    });
    let response = exchange(&socket, &operation(approval_id, held_digest, sequence));
    assert_eq!(
        response.get(..9),
        Some(b"LXHAGT01\x00".as_slice()),
        "valid EVM approval budget request"
    );
    let facts = &response[9..];
    assert_eq!(facts.len(), 155);
    assert_eq!(&facts[..2], &2u16.to_be_bytes());
    assert_eq!(&facts[2..34], &approval_id);
    assert_eq!(&facts[34..66], &held_digest);
    assert_eq!(
        u128::from_be_bytes(facts[66..82].try_into().unwrap()),
        expected_remaining
    );
    assert_eq!(
        facts[82],
        layerx_types::verify::VerificationLevel::COMMITTED.wire_rank()
    );
    assert_eq!(facts[82], state.verification);
    assert_ne!(&facts[83..115], &[0; 32]);
    assert_eq!(&facts[115..123], &sequence.to_be_bytes());
    assert_eq!(&facts[123..155], &asset);
    assert_eq!(
        exchange(
            &socket,
            &operation(approval_id, held_digest, sequence.checked_sub(1).unwrap())
        ),
        refusal_frame(b"operation.refused")
    );
    let mut changed_digest = held_digest;
    changed_digest[0] ^= 1;
    assert_eq!(
        exchange(&socket, &operation(approval_id, changed_digest, sequence)),
        refusal_frame(b"operation.refused")
    );
    must(worker.join());
}

/* RECORDED_AUTHORITY
{
  "chain_id": 125,
  "tenant": "approval-actor-tenant",
  "principal": "approval-actor-owner",
  "agent_did": "did:layerx:48443af2c3f128ded06d652381eb01d39265225ae57534432cec949a63528802",
  "budget_id": [
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    1
  ],
  "actor_address": [
    88,
    34,
    131,
    255,
    34,
    95,
    91,
    249,
    69,
    142,
    194,
    210,
    180,
    119,
    91,
    147,
    32,
    120,
    92,
    95
  ],
  "budget_call": [
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    32,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    2,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    3,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    4,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    5,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    100,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    100,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    23,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    3,
    232,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    3,
    232,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    35,
    40,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    31,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    1,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    2,
    64,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
    2,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    6,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7,
    7
  ],
  "budget_state": {
    "age_sequences": 0,
    "asset": "0404040404040404040404040404040404040404040404040404040404040404",
    "budget_id": "0101010101010101010101010101010101010101010101010101010101010101",
    "canonical_core_bytes": "000000000000000000000000000000000000000000000000000000000000002002020202020202020202020202020202020202020202020202020202020202020303030303030303030303030303030303030303030303030303030303030303040404040404040404040404040404040404040404040404040404040404040405050505050505050505050505050505050505050505050505050505050505050000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000017000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003e800000000000000000000000000000000000000000000000000000000000003e80000000000000000000000000000000000000000000000000000000000002328000000000000000000000000000000000000000000000000000000000000001f00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000206060606060606060606060606060606060606060606060606060606060606060707070707070707070707070707070707070707070707070707070707070707",
    "checkpoint_digest": "12edde9620f81e075b410c05b75918ed03739c0013b5ca4e46904de17fe08ad4",
    "closed": false,
    "evidence_digest": "d5f91cee984084f6a2ca7f0f7ca446ce281d622c4ae567454ed38344d9dfeec6",
    "maximum_age_sequences": 100,
    "observed_head_sequence": 64,
    "receipt_digest": "8395cfa873778b412da94943a1cd80ca500156b094ca0a2d24a06b5364f20282",
    "remaining": "77",
    "revocation_sequence": 31,
    "revoked": false,
    "verification": 6
  },
  "registry": {
    "modules": [
      {
        "activity_types": [
          131073,
          131079
        ],
        "module_id": 2
      }
    ]
  },
  "balance_context": {
    "account_id": "0202020202020202020202020202020202020202020202020202020202020202",
    "age_seconds": 0,
    "asset_id": "0404040404040404040404040404040404040404040404040404040404040404",
    "currency": "USD",
    "first_batch_number": 1,
    "last_batch_number": 65535,
    "maximum_age_seconds": 315360000,
    "observed_at": "recorded committed block",
    "sequencer_id": "10be7cf7e378215a7fc0a2d7933f19931b2c51893f1f458cb4b9a08031aa593d",
    "sequencer_public_key": "d759793bbc13a2819a827c76adb6fba8a49aee007f49f2d0992d99b825ad2c48"
  },
  "block": {
    "baseFeePerGas": "0x0",
    "hash": "0x12edde9620f81e075b410c05b75918ed03739c0013b5ca4e46904de17fe08ad4",
    "number": "0x40",
    "parentHash": "0x6c2f9b8b1b2960a5d9ff66d611bd2107159eb7bc0397c10dc8f43648b87560e2",
    "receiptsRoot": "0x1212121212121212121212121212121212121212121212121212121212121212",
    "stateRoot": "0x4147fa94c77b1827e32179b6b7e1d1b3f4254cbc340b7988735c057e6c23cb8c",
    "timestamp": "0x6aa78d28",
    "transactions": [],
    "transactionsRoot": "0x1111111111111111111111111111111111111111111111111111111111111111"
  },
  "managed_receipt": {
    "activity_id": "9df9ce087638436eaaf56ce3adaf3f774ac07428f74acee507678d65bc7b211e",
    "batch_number": 14,
    "header": "000317010f010003020000004d03000000000000000204000000000000000e05000000000000001b06000000000000001c07000000206c2f9b8b1b2960a5d9ff66d611bd2107159eb7bc0397c10dc8f43648b87560e208000000204147fa94c77b1827e32179b6b7e1d1b3f4254cbc340b7988735c057e6c23cb8c090000002094fa7025d8cbf5566d690f98c774aecfaf9b49edd7dd1750dd2ac6d421d412440a0000002031745aa83888a4055d6686ca921f578ffe3d690ec0193f3a7d695c26a4b710dc0b00000020e258b0264d7fbcd25faac5a2799163d318ae3c0c176cf056f704ef31366bd0f20c0000002058a219abedbda688e48a063a2dd0d776f08d41213184de66ae72b7445d35ed6a0d000000206fd4dc7d39aa9ba2abfab6ce51602faf5717905680c9420577d45a621e181dda0e000001a09e7f65070f0000002010be7cf7e378215a7fc0a2d7933f19931b2c51893f1f458cb4b9a08031aa593d",
    "header_signature": "f0e0c02a6c1f635f4490aaa65ef608b16fbed023c965cd51f72380fcffbbf8b7f273d3e7280f34332c4c9a264f36776230008e62eb9cdf5fa489d3d9f51bac01",
    "maintenance": "4c58502f62617463682d6d61696e74656e616e63652f76310000030000000000000002000000000000000e000001a09e7f6507000000000000001c00000001000002444c58502f70726f6772616d732f6f63637570616e63792d726563656970742f763200000000000000000e000000000000001c000000010000000e000000000000000100000000000000010000000000000002000000000000000400000000000000010000000000000001000000000000001cb5a32b12029f8ddfb905f90f280f664b46390de0fc62770fc197dd87b18cd8980000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005abea2797a320dfd1c22471f98d0032834210fb91b5b50292e3298e32badda85000000ac4c58502f73746f726167652d6f63637570616e63792d736574746c656d656e742f763300000000000000000e0000000e000000000000000100000000000000010000000000000002000000000000000400000000000000010000000000000001000000000000001c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000bfeb7663de4a1f003407936bba2a2b53cd0f4ed0570ac728a881711df24c5985db56bb52a1cd9468e11c079e545ccad3d55145bfe6e674ee2c7e886db08cf06c0000000000000000000000000000000000000000000000000000000000000000ff66d54f68a4add023e9bad148ebf86fed10276de762d262f5ca53d8ee7f60f34147fa94c77b1827e32179b6b7e1d1b3f4254cbc340b7988735c057e6c23cb8c000002ef4c58502f62617463682d6d61696e74656e616e63652d656666656374732f76310000030002000000010004000000000100000000000000000000000000000000000000000000000000000000000000000000480025686f6c643a810000000000000000000000000000000000000000000000000000000000000000dad346ec8191c35511a6c61ac8dd883585b055e98ea13a65cc218b4ac2a4a6a60001000001000000000000000000000000000000000000000000000000000000000000000000004a0027726573756c743a2d8a92d3f671bfb9a375ca40d7423adcf9890890cfbba27fa7560356f923dfba00d765f1ed5afc03eb1c93cb029c229879cfc5be3fa392fa83b3b2c267f7933bc00002000002013e658901ef4fb67eeb7a534663db6f505c91499b982a017738568223030dad350000000300050300000000000000000000000000000000000000000000000000000000000000000000438100000000000000000000000000000000000000000000000000000000000000000507000000000000000000000000000000000000000000000000000000000000000000030000000100010000000001000000000000000000000000000000000000000000000000000000000000000000004a00276275646765743a820000000000000000000000000000000000000000000000000000000000000000de0c0046d0d2e5d044e203021c13dc0b0682b823fb85c92b28bf7cd485e1d64c00050000000100020000000001000000000000000000000000000000000000000000000000000000000000000000004b002861677265653a3a31860000000000000000000000000000000000000000000000000000000000000000a0b81b3bf7ffb9b3e672d19ef1b7e9b40e6bf258874bf416df977a7bd3ebeef70001000e03000000000000000000000000000000000000000000000000000000000000000000003286000000000000000000000000000000000000000000000000000000000000000401000000000000001c000001a09e7f6507",
    "maintenance_proof": "01000000010000000252fc7975da88140282de8b8ea026969a7bd8c2b8a3d440306dc3e2797ac24056",
    "module_id": 7,
    "receipt": "000352010003000000209df9ce087638436eaaf56ce3adaf3f774ac07428f74acee507678d65bc7b211e000000000000001b000000206c2f9b8b1b2960a5d9ff66d611bd2107159eb7bc0397c10dc8f43648b87560e200000020ff66d54f68a4add023e9bad148ebf86fed10276de762d262f5ca53d8ee7f60f30000002094fa7025d8cbf5566d690f98c774aecfaf9b49edd7dd1750dd2ac6d421d412440000000000000001000700007109030000000020000000000000000000000000000000000000000000000000000000000000000000000020148d0d5c3c9ac703d4ab1472da0f4ab9677474fe41c482f6221fec6d5135d2df0000000000000000000000000000000000000020b2946c8a9f58921491ddc939441b0686aaf9c64f5867bbf4bca3dee65893112e000700000001000000010000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000001a09e7f6507010000004044e1cb800c888fd7665c42b6cbbf0501efa5e45ef71d6982dee77799d4b65deb9e3e03acc535c19800137d7009bd0e183b66c55fd60509f9f76c2a57a5829e0e",
    "receipt_proof": "010000000000000002011e2a7d611310b9a62256d53ae502dc18049dabcdb5befde401fa32f2d49656",
    "result_code": 0,
    "sequencer_public_key": "d759793bbc13a2819a827c76adb6fba8a49aee007f49f2d0992d99b825ad2c48"
  }
}
END_RECORDED_AUTHORITY */
