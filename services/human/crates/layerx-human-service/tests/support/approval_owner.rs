#[path = "../../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
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
use layerx_agentd::human::{serve_one, AcceptedTransport, HumanAgentLifecycleSeed, HumanPeer};
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
    prepare_activity, AbiArgument, PrecompileCall, PrecompileTarget, PreparationDefaults,
    PrepareRequest, ProductionEvmPreparationBoundary,
};
use layerx_agentd::read::{
    checkpoint, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
    CommitValidator, CommitVote, CommittedReads, ServedCheckpoint,
};
use layerx_agentd::session::{self, OpenRequest, SessionId, SessionRegistry};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_types::clock::Clock;
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::collections::{BTreeMap, BTreeSet};
use std::fs::{self, DirBuilder};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt as _, MetadataExt as _, PermissionsExt as _};
use std::os::unix::net::UnixListener;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

const DEADLINE: Duration = Duration::from_secs(8);
const MAXIMUM: usize = 1_048_576;
const PROTOCOL_VERSION: u16 = layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION;

pub(crate) mod hex {
    pub fn encode(bytes: impl AsRef<[u8]>) -> String {
        bytes
            .as_ref()
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect()
    }
    pub fn decode(value: impl AsRef<[u8]>) -> Result<Vec<u8>, ()> {
        let input = value.as_ref();
        let value = std::str::from_utf8(input).map_err(|_| ())?;
        if value.len() % 2 != 0 {
            return Err(());
        }
        (0..value.len())
            .step_by(2)
            .map(|i| u8::from_str_radix(&value[i..i + 2], 16).map_err(|_| ()))
            .collect()
    }
    pub fn decode32(value: &str) -> Result<[u8; 32], ()> {
        decode(value)?.try_into().map_err(|_| ())
    }
}
fn committed_checkpoint(fixture: &RecordedAuthority, height: u64) -> ServedCheckpoint {
    let key = SigningKey::from_bytes(&[42; 32]);
    let public_key = key.verifying_key().to_bytes();
    let validator = CommitValidator {
        address: CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: 100,
    };
    let validators = vec![validator];
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: fixture.chain_id.to_string(),
        height,
        time: CommitTimestamp {
            seconds: 1_700_000_000,
            nanos: 0,
        },
        last_block_id: Some(BlockId {
            hash: must(hex::decode32(
                fixture.block["parentHash"]
                    .as_str()
                    .unwrap()
                    .trim_start_matches("0x"),
            )),
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: validator_set_hash(&validators),
        next_validators_hash: validator_set_hash(&validators),
        consensus_hash: [5; 32],
        app_hash: must(hex::decode32(
            fixture.block["stateRoot"]
                .as_str()
                .unwrap()
                .trim_start_matches("0x"),
        )),
        last_results_hash: Some([7; 32]),
        evidence_hash: Some([8; 32]),
        proposer_address: validator.address,
    };
    let mut commit = BlockCommit {
        height,
        round: 0,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [9; 32],
        },
        votes: Vec::new(),
    };
    commit.votes.push(CommitVote {
        validator_address: validator.address,
        voting_power: validator.voting_power,
        timestamp: header.time,
        signature: key
            .sign(&commit.vote_sign_bytes(&header.chain_id, header.time))
            .to_bytes(),
    });
    must(checkpoint(
        height,
        &header.chain_id,
        &header,
        &commit,
        &validators,
    ))
}

fn commit_response(checkpoint: &ServedCheckpoint) -> Value {
    let header = &checkpoint.header;
    let block_id = |id: BlockId| {
        json!({
            "hash": hex::encode(id.hash),
            "parts": {"total": id.part_set_total, "hash": hex::encode(id.part_set_hash)},
        })
    };
    json!({"height": checkpoint.height.to_string(), "signedHeader": {
        "header": {
            "version": {"block": header.version_block.to_string(), "app": header.version_app.to_string()},
            "chain_id": header.chain_id, "height": header.height.to_string(),
            "time": "2023-11-14T22:13:20Z",
            "last_block_id": block_id(header.last_block_id.unwrap()),
            "last_commit_hash": hex::encode(header.last_commit_hash.unwrap()),
            "data_hash": hex::encode(header.data_hash.unwrap()),
            "validators_hash": hex::encode(header.validators_hash),
            "next_validators_hash": hex::encode(header.next_validators_hash),
            "consensus_hash": hex::encode(header.consensus_hash),
            "app_hash": hex::encode(header.app_hash),
            "last_results_hash": hex::encode(header.last_results_hash.unwrap()),
            "evidence_hash": hex::encode(header.evidence_hash.unwrap()),
            "proposer_address": hex::encode(header.proposer_address),
        },
        "commit": {
            "height": checkpoint.height.to_string(), "round": checkpoint.commit.round,
            "block_id": block_id(checkpoint.commit.block_id),
            "signatures": checkpoint.commit.votes.iter().map(|vote| json!({
                "block_id_flag": 2, "validator_address": hex::encode(vote.validator_address),
                "timestamp": "2023-11-14T22:13:20Z",
                "signature": base64::engine::general_purpose::STANDARD.encode(vote.signature),
            })).collect::<Vec<_>>(),
        },
    }})
}

fn validators_response(checkpoint: &ServedCheckpoint) -> Value {
    json!({"height": checkpoint.height.to_string(), "total": checkpoint.validators.len().to_string(),
        "validators": checkpoint.validators.iter().map(|validator| json!({
            "address": hex::encode(validator.address),
            "pubKey": {"type": "tendermint/PubKeyEd25519", "value": base64::engine::general_purpose::STANDARD.encode(validator.public_key)},
            "votingPower": validator.voting_power.to_string(),
        })).collect::<Vec<_>>()})
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
    must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", bytes.len()));
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
        head: Arc<std::sync::atomic::AtomicU64>,
    ) -> (String, Self) {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}/", must(listener.local_addr()));
        let fixture: RecordedAuthority =
            must(serde_json::from_value(must(serde_json::to_value(fixture))));
        let server = Self::spawn(listener, move |mut stream| {
            let (_, body) = read_http(&mut stream);
            let mut block = fixture.block.clone();
            let checkpoint = committed_checkpoint(&fixture, head.load(Ordering::Acquire));
            block["number"] = json!(format!("{:#x}", checkpoint.height));
            block["hash"] = json!(format!("0x{}", hex::encode(checkpoint.block_hash)));
            block["timestamp"] = json!(format!("{:#x}", checkpoint.header.time.seconds));
            let request: Value = must(serde_json::from_slice(&body));
            let mut account = json!({
                "evm_address":format!("0x{}",hex::encode(fixture.actor_address)),
                "pax_address":null,"layerx_did":fixture.agent_did,
                "layerx_account":hex::encode([2;32]),"bound":true,
            });
            let result = match request["method"].as_str().expect("RPC method") {
                "px_getCommit" => {
                    assert_eq!(request["params"][0], checkpoint.height.to_string());
                    commit_response(&checkpoint)
                }
                "px_getValidators" => {
                    assert_eq!(request["params"][0], checkpoint.height.to_string());
                    validators_response(&checkpoint)
                }
                "eth_chainId" => json!(format!("{:#x}", fixture.chain_id)),
                "eth_blockNumber" => block["number"].clone(),
                "eth_getBlockByNumber" => block,
                "eth_estimateGas" => json!("0x186a0"),
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

    fn authority(
        fixture: &RecordedAuthority,
        head: Arc<std::sync::atomic::AtomicU64>,
    ) -> (String, Vec<u8>, Self) {
        let ca = include_bytes!("approval_fixture/loopback-ca.der").to_vec();
        let identity = must(native_tls::Identity::from_pkcs8(
            include_bytes!("approval_fixture/loopback.pem"),
            include_bytes!("approval_fixture/loopback-key.pem"),
        ));
        let acceptor = must(native_tls::TlsAcceptor::new(identity));
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let endpoint = format!("https://localhost:{}", must(listener.local_addr()).port());
        let registry = fixture.registry.clone();
        let recorded: RecordedAuthority =
            must(serde_json::from_value(must(serde_json::to_value(fixture))));
        let balance = fixture.balance_context.clone();
        let identity = json!({
            "canonical_core_bytes":fixture.agent_did.strip_prefix("did:layerx:").unwrap(),
            "head_sequence":fixture.budget_state["observed_head_sequence"],
            "revocation_sequence":fixture.budget_state["revocation_sequence"],
            "verification_level":"committed", "frozen":false,
            "authorities":[{"kind":"primary_key","id":fixture.agent_did.strip_prefix("did:layerx:").unwrap()}]
        });
        let server = Self::spawn(listener, move |stream| {
            let mut stream = must(acceptor.accept(stream));
            let (headers, _) = read_http(&mut stream);
            let mut budget = recorded.budget_state.clone();
            let checkpoint = committed_checkpoint(&recorded, head.load(Ordering::Acquire));
            budget["observed_head_sequence"] = json!(checkpoint.height);
            budget["checkpoint_digest"] = json!(hex::encode(checkpoint.block_hash));
            assert_eq!(
                budget["observed_head_sequence"],
                head.load(Ordering::Acquire)
            );
            assert!(headers.contains("Bearer approval-budget-loopback-authority"));
            let path = headers
                .lines()
                .next()
                .unwrap()
                .split_whitespace()
                .nth(1)
                .unwrap();
            assert!(path.contains(&format!("tenant={}", recorded.tenant)));
            assert!(path.contains(&format!("principal={}", recorded.principal)));
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

#[track_caller]
pub(crate) fn open_store(root: impl AsRef<Path>) -> Store {
    let root = root.as_ref();
    Store::open(root).unwrap_or_else(|error| {
        eprintln!(
            "approval store refused: path={} canonical={:?} uid={:?} effective_uid={} mode={:?}: {error:?}",
            root.display(),
            fs::canonicalize(root),
            fs::symlink_metadata(root).map(|metadata| metadata.uid()),
            rustix::process::geteuid().as_raw(),
            fs::symlink_metadata(root).map(|metadata| format!("{:04o}", metadata.mode() & 0o7777)),
        );
        panic!("approval budget actor store: {error:?}");
    })
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

struct AuthorityFixture {
    recorded: RecordedAuthority,
    head: Arc<std::sync::atomic::AtomicU64>,
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
        let recorded: RecordedAuthority = must(serde_json::from_str(include_str!(
            "approval_fixture/authority.json"
        )));
        let budget = must(
            layerx_agentd::budget::ProtocolBudgetState::decode_get_budget(
                recorded.budget_id,
                recorded.budget_state["observed_head_sequence"]
                    .as_u64()
                    .unwrap(),
                &recorded.budget_call,
            ),
        );
        assert_eq!(budget.record().remaining(), 77);
        assert_eq!(
            budget.record().spent_this_period,
            0,
            "no spend receipts have been persisted"
        );
        let head = Arc::new(std::sync::atomic::AtomicU64::new(
            recorded.budget_state["observed_head_sequence"]
                .as_u64()
                .unwrap(),
        ));
        let (node_rpc_url, node) = LocalServer::node(&recorded, head.clone());
        let (authority_endpoint, authority_ca_der, authority) =
            LocalServer::authority(&recorded, head.clone());
        Self {
            recorded,
            head,
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
        let root = must(fs::canonicalize(std::env::temp_dir()))
            .join(format!("lx-ha-{}", hex::encode(&nonce[..8])));
        must(DirBuilder::new().mode(0o700).create(&root));
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

pub struct Fixture {
    server: Option<AgentServer>,
    owner: Option<Arc<Mutex<UnifiedAgentOwner<RemoteHumanAuthority>>>>,
    shared: Option<Arc<Mutex<Store>>>,
    authority: AuthorityFixture,
    runtime: Runtime,
    pub tenant: TenantId,
    pub peer: HumanPeer,
    pub registry: ModuleRegistry,
    pub prepared: BTreeMap<u8, Prepared>,
    pub sequence: u64,
    pub window: u64,
    pub limit: LimitId,
    pub protocol_remaining: u128,
    pub asset: [u8; 32],
}

impl Fixture {
    pub fn new(ids: &[u8]) -> Self {
        let authority = AuthorityFixture::load();
        let runtime = Runtime::start();
        let uid = must(fs::metadata(&runtime.root)).uid();
        let peer = HumanPeer {
            uid,
            principal: authority.principal.clone(),
            tenant: authority.tenant.clone(),
            subject: None,
        };
        let tenant = must(TenantId::new(&peer.tenant));
        let mut remote = authority.connect();
        let registry = must(remote.registry(&peer));
        let state = must(remote.budget_state(&peer, authority.budget_id));
        let (_, asset, currency, _, _, _, _) = must(remote.balance_context(&peer));
        let public_key = must(hex::decode32(
            authority.agent_did.strip_prefix("did:layerx:").unwrap(),
        ));
        let now = must(runtime.clock.sample(DEADLINE)).unix_seconds();
        let expiry = now.checked_add(600).unwrap();
        let sequence = state.observed_head_sequence;
        let window = sequence + 20;
        let mut store = open_store(runtime.root.join("store"));
        let did = must(Did::new(authority.agent_did.as_bytes()));
        let identity = must(identity::register(
            &mut store,
            tenant.clone(),
            did.clone(),
            &mut AuthorityIdentity {
                authority: &mut remote,
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
                opening_client: "human-approval-repair".to_owned(),
                policy_version: "approval-actor-v1".to_owned(),
            },
            sequence,
        ));
        assert_eq!(token.credential().generation(), 1);
        let creation_receipt = verified_managed_receipt(&authority.managed_receipt);
        let seed = HumanAgentLifecycleSeed {
            agent_id: format!("agt_{}", hex::encode(authority.budget_id)),
            name: "approval actor".to_owned(),
            purpose: "durable EVM approval".to_owned(),
            currency: currency.clone(),
            monthly_limit: 1000,
            period_start: now,
            period_end: expiry,
            created_at: now,
            updated_at: now,
            verified_evidence: vec![creation_receipt],
            actor: authority.agent_did.clone(),
            primary_authority: hex::encode(public_key),
            custody_key: "approval-actor".to_owned(),
            custody_public_key: public_key,
            owner_account: format!("agent:{}:master", authority.agent_did),
            budget_account: format!("budget:{}", hex::encode(authority.budget_id)),
            budget_asset: asset,
            purpose_hash: Sha256::digest(b"durable EVM approval").into(),
            recovery_root: state.checkpoint_digest,
            recovery_threshold: 1,
            capability_id: [6; 32],
            activity_types: vec![7],
            counterparties: vec![[4; 32]],
            assets: vec![asset],
            amount_ceiling: 1000,
            rate_maximum_uses: 1,
            rate_window_sequences: 20,
            purposes: vec!["durable EVM approval".to_owned()],
            capability_expiry_sequence: window,
            session_scopes: vec!["prepare".to_owned()],
            session_expiry_unix_seconds: expiry,
            protocol_grant_id: [6; 32],
            budget_period_seconds: 600,
            budget_expiry_seconds: expiry,
            initial_funding: state.remaining,
            network_id: u32::try_from(authority.chain_id).unwrap(),
            creation_receipt_roots: vec![creation_receipt],
        };
        let agent = must(ManagedAgent::from_creation(
            &seed,
            &authority.agent_did,
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
            u32::try_from(authority.agent_did.len())
                .unwrap()
                .to_be_bytes(),
        );
        agent_digest.update(authority.agent_did.as_bytes());
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
                ceiling: 1000,
                consumed: 0,
                expiry_ms: expiry * 1000,
                revoked: false,
                mutation_key: [10; 32],
                body_digest: [11; 32],
                revoke_key: [0; 32],
            },
            CoreTimestampMs(now * 1000),
        ));
        let shared = Arc::new(Mutex::new(store));
        let peers = BTreeMap::from([(uid, (peer.principal.clone(), peer.tenant.clone()))]);
        let owner = runtime.owner(&authority, &peers, shared.clone());
        let boundary = must(ProductionEvmPreparationBoundary::new(
            &authority.node_rpc_url,
            &[],
        ));
        let mut prepared = BTreeMap::new();
        for &id in ids {
            let mut word = [0; 32];
            let amount = match id {
                1 => 25,
                2 => 26,
                3 => 24,
                _ => panic!("undeclared hold"),
            };
            word[16..].copy_from_slice(&u128::to_be_bytes(amount));
            let evm = must(prepare_activity(
                &boundary,
                PreparationDefaults {
                    max_priority_fee_per_gas: 0,
                    max_fee_per_gas: 0,
                    maximum_payload_bytes: MAXIMUM,
                },
                PrepareRequest {
                    from: authority.actor_address,
                    call: PrecompileCall {
                        target: PrecompileTarget::Ledger,
                        selector: layerx_crypto::disclosure::call::ledger_selector(
                            "mint(bytes32,bytes32,uint128)",
                        ),
                        arguments: vec![
                            AbiArgument::Word(asset),
                            AbiArgument::Word([4; 32]),
                            AbiArgument::Word(word),
                        ],
                    },
                    expected_nonce: Some(0),
                    max_priority_fee_per_gas: None,
                    max_fee_per_gas: None,
                    declared_payload_limit: MAXIMUM,
                },
            ));
            must(layerx_agentd::prepare::evm::verify_call_binding(&evm));
            assert_eq!(evm.call.chain_id, authority.chain_id);
            assert_eq!(evm.disclosure.actor, authority.actor_address);
            assert_eq!(evm.disclosure.asset, asset);
            assert_eq!(evm.disclosure.fee_limit, 0);
            assert_eq!(evm.disclosure.amounts.len(), 1);
            assert_eq!(evm.disclosure.amounts[0].value, amount);
            assert_eq!(evm.disclosure.counterparties.len(), 1);
            assert_eq!(evm.disclosure.counterparties[0].account, [4; 32]);
            let canonical = evm.call.encode_unsigned();
            assert_eq!(canonical[0], 2);
            assert_eq!(evm.signing_hash, evm.call.signing_hash());
            let digest = Sha256::digest(&canonical).into();
            let counterparty = must(AgentDid::new(hex::encode([4; 32])));
            let held = Prepared {
                preparation_ref: must(PreparationRef::new(hex::encode([id; 32]))),
                unsigned_canonical_bytes: must(CanonicalBytes::new(canonical)),
                signing_preimage: must(SigningPreimage::new(evm.signing_hash.to_vec())),
                disclosure: Disclosure {
                    canonical_digest: digest,
                    activity_type: ActivityType(evm.disclosure.activity_type.ordinal()),
                    actor: must(AgentDid::new(hex::encode(evm.disclosure.actor))),
                    authority: must(AuthorityRef::new(hex::encode(public_key))),
                    counterparties: ExplicitSet::allow(vec![counterparty.clone()]),
                    amounts: ExplicitSet::allow(vec![DisclosedAmount {
                        counterparty,
                        amount: Amount(amount),
                    }]),
                    asset: must(Asset::new(&currency)),
                    fee_limit: Amount(evm.disclosure.fee_limit),
                    expiry: TimestampSeconds(expiry),
                    idempotency_key: must(IdempotencyRef::new(hex::encode([id; 32]))),
                },
                expiry: TimestampSeconds(expiry),
            };
            let approval_expiry_sequence = if id == 3 { sequence + 4 } else { window };
            let reservation = must(reserve_until_core_time(
                &owner.budgets,
                &ReservationRequest {
                    id: [id; 32],
                    amount,
                    current_sequence: sequence,
                    expiry_sequence: approval_expiry_sequence,
                    applicable_limits: vec![limit.limit_id],
                },
                CoreTimestampMs(expiry * 1000),
                CoreTimestampMs(now * 1000),
            ));
            let ticket = must(hold_reserved_at(
                &owner.approvals,
                ApprovalContext {
                    tenant: tenant.clone(),
                    agent: must(Did::new(held.disclosure.actor.as_str().as_bytes())),
                    actor_address: authority.actor_address,
                    session: SessionId([7; 32]),
                    capability: CapabilityId([6; 32]),
                    policy_version: "approval-actor-v1".to_owned(),
                    request_id: [id; 32],
                },
                held.clone(),
                sequence,
                approval_expiry_sequence,
                &reservation,
                runtime.clock.as_ref(),
            ));
            assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
            assert_eq!(ticket.disclosure, held.disclosure);
            prepared.insert(id, held);
            let key = must(TenantKey::new(
                tenant.clone(),
                ObjectKind::PreparedActivity,
                [b"approval-hold-v1:".as_slice(), &[id; 32]].concat(),
            ));
            assert_eq!(
                must(shared.lock()).get(&key).unwrap().class(),
                StorageClass::LocalOnly
            );
        }
        must(owner.approvals.validate_registry(&tenant, &registry));
        let mut fixture = Self {
            server: None,
            owner: Some(Arc::new(Mutex::new(owner))),
            shared: Some(shared),
            authority,
            runtime,
            tenant,
            peer,
            registry,
            prepared,
            sequence,
            window,
            limit: limit.limit_id,
            protocol_remaining: state.remaining,
            asset,
        };
        fixture.reopen();
        fixture
    }

    pub fn root(&self) -> &Path {
        &self.runtime.root
    }
    pub fn clock(&self) -> Arc<dyn Clock> {
        self.runtime.clock.clone()
    }
    pub fn client(&self) -> AgentRuntime {
        self.client_at(self.root().join("human.sock"))
    }
    fn client_at(&self, socket: PathBuf) -> AgentRuntime {
        must(AgentRuntime::new_with_clock(
            socket,
            layerx_human_service::frame::Limits {
                maximum_frame_bytes: MAXIMUM,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAXIMUM,
                deadline: DEADLINE,
            },
            self.registry.clone(),
            self.clock(),
        ))
    }
    pub fn advance(&self, sequence: u64) {
        self.authority.head.store(sequence, Ordering::Release);
    }
    pub fn reopen(&mut self) {
        self.server.take();
        self.owner.take();
        self.shared.take();
        let shared = Arc::new(Mutex::new(open_store(self.root().join("store"))));
        let peers = BTreeMap::from([(
            self.peer.uid,
            (self.peer.principal.clone(), self.peer.tenant.clone()),
        )]);
        let owner = Arc::new(Mutex::new(self.runtime.owner(
            &self.authority,
            &peers,
            shared.clone(),
        )));
        self.server = Some(AgentServer::start(
            self.root().join("human.sock"),
            self.peer.clone(),
            owner.clone(),
        ));
        self.owner = Some(owner);
        self.shared = Some(shared);
    }
    pub fn assert_snapshot_recovery(&mut self) {
        let endpoint = must(SequencerEndpoint::new(&self.authority.node_rpc_url, &[]));
        let reads = must(CommittedReads::open(&endpoint, self.sequence));
        let mut pay = [0; 20];
        pay[18..].copy_from_slice(&[0x10, 0x1b]);
        let mut calldata =
            layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
        calldata.extend_from_slice(&self.authority.budget_id);
        let call = must(reads.budget_state(pay, &calldata));
        let evidence = must(
            layerx_agentd::protocol_evidence::RawStateEvidence::budget_witness(
                call,
                self.authority.budget_id,
            ),
        );
        assert_eq!(
            evidence.canonical_state(),
            must(hex::decode(
                self.authority.budget_state["canonical_core_bytes"]
                    .as_str()
                    .unwrap()
            ))
        );
        let record = must(layerx_agentd::budget::ProtocolBudgetRecord::decode(
            evidence.canonical_state(),
        ));
        assert_eq!(record.remaining(), self.protocol_remaining);
        assert_eq!(record.spent_this_period, 0);
        let verifier = must(
            layerx_agentd::protocol_evidence::EvidenceAuthority::pinned_to_handshake(
                PROTOCOL_VERSION,
                u32::try_from(self.authority.chain_id).unwrap(),
                must(hex::decode32(
                    self.authority.balance_context["sequencer_public_key"]
                        .as_str()
                        .unwrap(),
                )),
            ),
        );
        let recovery = must(layerx_agentd::human_runtime::recover_tenant_budget(
            &mut *must(self.shared.as_ref().unwrap().lock()),
            &self.tenant,
            &layerx_agentd::human_runtime::BudgetRecoveryRequest {
                budget_id: self.authority.budget_id,
                protocol_budget: layerx_agentd::budget::ProtocolBudgetState { evidence },
                verifier,
                receipts_with_evidence: &[],
                receipts_without_evidence: &[],
                ceiling_maximum: 1_000_000_000_000_000_000,
                current_sequence: self.sequence,
            },
        ));
        assert!(recovery.admission.is_ok());
        assert!(recovery.recovered.budget_accounting.reconciled);
        assert_eq!(
            recovery.recovered.budget_accounting.protocol_consumed,
            Some(0)
        );
        assert_eq!(recovery.recovered.budget_accounting.receipt_consumed, 0);
        let state = must(
            self.authority
                .connect()
                .budget_state(&self.peer, self.authority.budget_id),
        );
        let snapshot = must(self.client().agent_budget_state(self.authority.budget_id));
        let rung = layerx_types::rung::Rung::Committed;
        assert_eq!(snapshot.active_budget_id, self.authority.budget_id);
        assert_eq!(snapshot.revocation_sequence, state.revocation_sequence);
        assert_eq!(
            snapshot.observed_head_sequence,
            state.observed_head_sequence
        );
        assert_eq!(snapshot.verification, rung.wire_rank());
        assert_eq!(snapshot.remaining, state.remaining);
        assert_eq!(snapshot.asset, state.asset);
        assert_eq!(snapshot.evidence_digest, state.evidence_digest);
        assert_eq!(snapshot.receipt_digest, state.receipt_digest);
        assert_eq!(snapshot.checkpoint_digest, state.checkpoint_digest);
        assert_eq!(snapshot.age_sequences, state.age_sequences);
        assert_eq!(snapshot.maximum_age_sequences, state.maximum_age_sequences);
        let key = must(TenantKey::new(
            self.tenant.clone(),
            ObjectKind::Configuration,
            [
                b"managed-agent-observation-v1:".as_slice(),
                &[1],
                &state.evidence_digest,
            ]
            .concat(),
        ));
        let bytes = {
            let store = must(self.shared.as_ref().unwrap().lock());
            let stored = store
                .get(&key)
                .expect("daemon persisted budget observation");
            assert_eq!(stored.class(), StorageClass::LocalOnly);
            stored.bytes().to_vec()
        };
        assert_eq!(bytes, must(state.encode_snapshot(self.authority.budget_id)));
        let decoded = must(layerx_human_service::agents::decode_budget_snapshot(&bytes));
        assert_eq!(decoded, (self.authority.budget_id, state, rung));
        self.reopen();
        let store = must(self.shared.as_ref().unwrap().lock());
        let restored = store.get(&key).expect("reopened daemon budget observation");
        assert_eq!(restored.class(), StorageClass::LocalOnly);
        assert_eq!(restored.bytes(), bytes);
        assert_eq!(
            must(layerx_human_service::agents::decode_budget_snapshot(
                restored.bytes()
            )),
            decoded
        );
    }
    pub fn assert_reservations(&self, count: usize, exposure: u128, releases: usize) {
        let owner = must(self.owner.as_ref().unwrap().lock());
        assert_eq!(must(owner.budgets.held_reservations()), count);
        assert_eq!(must(owner.budgets.held_exposure(self.limit)), exposure);
        assert_eq!(
            must(owner.budgets.consumed(self.limit)),
            0,
            "approval alone does not spend"
        );
        assert_eq!(owner.approval_queue.len(), releases);
    }
    pub fn assert_release(&self, id: u8, reference: [u8; 32]) {
        let owner = must(self.owner.as_ref().unwrap().lock());
        assert_eq!(
            owner.approval_queue.prepared(reference),
            Some(self.prepared[&id].clone())
        );
        let key = must(TenantKey::new(
            self.tenant.clone(),
            ObjectKind::Outbox,
            [b"approval-released-v1:".as_slice(), &[id; 32]].concat(),
        ));
        assert!(must(self.shared.as_ref().unwrap().lock())
            .get(&key)
            .is_some());
    }
    pub fn assert_rejected_decision(&self, id: u8, sequence: u64) -> Vec<u8> {
        let owner = must(self.owner.as_ref().unwrap().lock());
        let decision = must(owner.approval_expiry.recover(
            &self.tenant,
            [id; 32],
            self.window,
            sequence,
            &owner.budgets,
        ));
        assert_eq!(
            decision.outcome,
            layerx_agentd::approval::ApprovalOutcome::Rejected
        );
        assert_eq!(decision.submission_ref, None);
        let store = must(self.shared.as_ref().unwrap().lock());
        assert_eq!(store.root(), self.root().join("store"));
        let key = must(TenantKey::new(
            self.tenant.clone(),
            ObjectKind::Idempotency,
            [b"approval-v1:".as_slice(), &[id; 32]].concat(),
        ));
        let value = store
            .get(&key)
            .expect("daemon persisted rejection decision");
        assert_eq!(value.class(), StorageClass::LocalOnly);
        let hold_key = must(TenantKey::new(
            self.tenant.clone(),
            ObjectKind::PreparedActivity,
            [b"approval-hold-v1:".as_slice(), &[id; 32]].concat(),
        ));
        eprintln!(
            "rejected approval {id}: store={} decision={:?} durable_hold_present={} registry_ticket={:?}",
            store.root().display(),
            decision.outcome,
            store.get(&hold_key).is_some(),
            must(owner.approvals.ticket([id; 32])),
        );
        value.bytes().to_vec()
    }
    pub fn assert_budget(
        &self,
        hold: &layerx_human_service::approvals::AgentApprovalRecord,
        remaining: u128,
        sequence: u64,
    ) {
        use layerx_human_service::approvals::ApprovalBoundary;
        let mut client = self.client();
        let value = must(client.verified_budget_after(hold, sequence));
        let state = must(
            self.authority
                .connect()
                .budget_state(&self.peer, self.authority.budget_id),
        );
        let amount = hold
            .held_activity
            .amounts
            .values()
            .iter()
            .fold(hold.held_activity.fee_limit.0, |total, item| {
                total + item.amount.0
            });
        let mut digest = Sha256::new();
        digest.update(b"LayerX/Human/approval-budget-after/v2\0");
        digest.update(hold.approval_id);
        digest.update(hold.canonical_bytes_digest);
        digest.update(self.authority.budget_id);
        digest.update(self.asset);
        digest.update(amount.to_be_bytes());
        digest.update(remaining.to_be_bytes());
        digest.update(state.evidence_digest);
        digest.update(state.receipt_digest);
        digest.update(state.checkpoint_digest);
        digest.update(sequence.to_be_bytes());
        assert_eq!(value.remaining, remaining);
        assert_eq!(value.rung, layerx_types::rung::Rung::Committed);
        assert_eq!(value.observed_at_sequence, sequence);
        assert_eq!(value.evidence_digest, <[u8; 32]>::from(digest.finalize()));
    }
    pub fn assert_foreign_refusals(&self, id: u8, sequence: u64) {
        let peer = HumanPeer {
            uid: self.peer.uid,
            principal: "foreign-principal".to_owned(),
            tenant: "foreign-tenant".to_owned(),
            subject: None,
        };
        let socket = self.root().join("foreign.sock");
        let _server =
            AgentServer::start(socket.clone(), peer, self.owner.as_ref().unwrap().clone());
        let mut client = self.client_at(socket);
        let page = must(client.approval_list(sequence, None, 100));
        assert!(page.approvals.is_empty());
        assert!(client.approval_get([id; 32], sequence).is_err());
        assert!(client
            .approval_decide(
                true,
                [id; 32],
                self.prepared[&id].disclosure.canonical_digest,
                "foreign-approve",
                sequence
            )
            .is_err());
        assert!(client
            .approval_decide(
                false,
                [id; 32],
                self.prepared[&id].disclosure.canonical_digest,
                "foreign-reject",
                sequence
            )
            .is_err());
    }
    pub fn lose_approval_reply(&self) -> (AgentRuntime, JoinHandle<()>) {
        let socket = self.root().join("lost.sock");
        let listener = must(UnixListener::bind(&socket));
        let backend = self.root().join("human.sock");
        let worker = thread::spawn(move || {
            use layerx_agentd::human::frame::{read_frame, write_frame};
            for opcode in [
                layerx_agentd::human::APPROVAL_GET,
                layerx_agentd::human::APPROVAL_BUDGET_AFTER_V2,
                layerx_agentd::human::APPROVAL_APPROVE,
            ] {
                let (mut browser, _) = must(listener.accept());
                must(browser.set_read_timeout(Some(DEADLINE)));
                let request = must(read_frame(&mut browser, MAXIMUM));
                let inner = untraced_frame(&request);
                assert_eq!(&inner[..8], b"LXHAGT01");
                assert_eq!(inner[8], opcode);
                let mut daemon = must(std::os::unix::net::UnixStream::connect(&backend));
                must(daemon.set_read_timeout(Some(DEADLINE)));
                must(write_frame(&mut daemon, &request, MAXIMUM));
                let response = must(read_frame(&mut daemon, MAXIMUM));
                assert_eq!(&untraced_frame(&response)[..9], b"LXHAGT01\x00");
                if opcode != layerx_agentd::human::APPROVAL_APPROVE {
                    must(write_frame(&mut browser, &response, MAXIMUM));
                }
            }
        });
        (self.client_at(socket), worker)
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        self.server.take();
        self.owner.take();
        self.shared.take();
    }
}

fn untraced_frame(frame: &[u8]) -> &[u8] {
    if frame.starts_with(b"LXHAGT02") {
        assert!(frame.len() >= 48);
        let length = u32::from_be_bytes(frame[44..48].try_into().unwrap()) as usize;
        assert_eq!(frame.len(), 48 + length);
        &frame[48..]
    } else {
        frame
    }
}

struct AgentServer {
    socket: PathBuf,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl AgentServer {
    fn start(
        socket: PathBuf,
        peer: HumanPeer,
        owner: Arc<Mutex<UnifiedAgentOwner<RemoteHumanAuthority>>>,
    ) -> Self {
        let listener = must(UnixListener::bind(&socket));
        must(fs::set_permissions(
            &socket,
            fs::Permissions::from_mode(0o600),
        ));
        must(listener.set_nonblocking(true));
        let stop = Arc::new(AtomicBool::new(false));
        let signal = stop.clone();
        let worker = thread::spawn(move || {
            while !signal.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        let credentials = must(rustix::net::sockopt::socket_peercred(&stream));
                        assert_eq!(credentials.uid.as_raw(), peer.uid);
                        let mut transport = must(AcceptedTransport::new(stream, MAXIMUM, DEADLINE));
                        must(serve_one(&mut transport, &peer, &mut *must(owner.lock())));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1))
                    }
                    Err(error) => panic!("Human listener: {error}"),
                }
            }
        });
        Self {
            socket,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for AgentServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
        must(fs::remove_file(&self.socket));
    }
}
