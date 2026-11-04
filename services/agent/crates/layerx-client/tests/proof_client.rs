//! Proves: registry_ops::apply/read_state_chunk, Eip1559Call::sign and repository-recorded RPC producers -> ObservedSnapshot, CommitFinality, FinalizedSnapshot, OperationJournal, SequencerEndpoint and PxEndpoint -> clients.md public API consumers.
//!
//! Repairs ai_market.rs::decode_snapshot_binding accepting a commit below the
//! captured execution height. Malformed bytes come from the real binding encoder.
//! The retired CheckpointFinality path is exercised as a refusal, not reintroduced.
//! RPC production is hosted in-process on a loopback TcpListener. Recorded
//! account, balance and asset bodies come unchanged from sdk/wallet/test/fixtures/
//! endpoint/available.json; the current consensus head and history bodies are
//! the responses in tests/px_reads.rs and tests/sequencer_reads.rs respectively.
//! The public HTTP/JSON-RPC transport and consumer decoders run on every read;
//! only refusal cases damage a producer result. No URL comes from the environment.
//! Inventory finding: interfaces.kvx::rs-client-ai-market-checkpointfinality-j44
//! names a removed type and ai_market_client.rs callers absent at this revision;
//! the replacement exercised here is CommitFinality, as required by clients.md.

use std::fmt;
use std::fs;
use std::future::Future;
use std::path::PathBuf;
use std::pin::pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use layerx_client::ai_market::{
    AiClientError, CommitFinality, NativeTerms, ObservedSnapshot, OperationJournal,
    OperationRecord, OperationRequest, OperationState, PreparedOperation, PROGRAM_CALL_ORDINAL,
};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::evm_transaction;
use layerx_crypto::local::LocalSigner;
use layerx_crypto::signer::Signer;
use layerx_programs_ai_market::codec::{decode_envelope, derive_market, encode_envelope, Envelope};
use layerx_programs_ai_market::dispatch;
use layerx_programs_ai_market::errors::ApplicationError;
use layerx_programs_ai_market::policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES};
use layerx_programs_ai_market::queries::{read_state_chunk, QueryError, ReadProof, FINALIZED_RANK};
use layerx_programs_ai_market::registry::{derive_rewards_account, F01_SECTION_CAP};
use layerx_programs_ai_market::registry_ops::{self, Outcome};
use layerx_programs_ai_market::rewards::{FundRequest, FUNDING_POLICY_VERSION};
use layerx_programs_ai_market::state::{self, Section, SharedState};
use layerx_programs_ai_market::types::{
    AssetId, Authentication, ChainDomain, Digest32, MarketId, Presence, PrincipalId, ProgramId,
    RequestId, RubricDigest,
};
use layerx_programs_ai_market::{MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::program_call::Resources;

const CHAIN: [u8; 32] = [10; 32];
const PROGRAM: [u8; 32] = [11; 32];
const OWNER: [u8; 32] = [12; 32];
const ASSET: [u8; 32] = [13; 32];
const REFUND: [u8; 32] = [14; 32];
const OWNER_SEED: [u8; 32] = [0x41; 32];
const ORIGIN: u64 = 1;
const NETWORK: u32 = 125;
static NEXT_JOURNAL: AtomicU64 = AtomicU64::new(1);

enum Failure {
    Application(ApplicationError),
    Query(QueryError),
    Client(AiClientError),
    Setup(String),
}

impl fmt::Debug for Failure {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Application(error) => write!(formatter, "application refusal {error:?}"),
            Self::Query(error) => write!(formatter, "query refusal {error:?}"),
            Self::Client(error) => write!(formatter, "client refusal {error:?}"),
            Self::Setup(detail) => write!(formatter, "setup failure {detail}"),
        }
    }
}

impl From<ApplicationError> for Failure {
    fn from(error: ApplicationError) -> Self {
        Self::Application(error)
    }
}

impl From<QueryError> for Failure {
    fn from(error: QueryError) -> Self {
        Self::Query(error)
    }
}

impl From<AiClientError> for Failure {
    fn from(error: AiClientError) -> Self {
        Self::Client(error)
    }
}

type Checked<T = ()> = Result<T, Failure>;

fn setup<E: fmt::Debug>(what: &'static str) -> impl FnOnce(E) -> Failure {
    move |error| Failure::Setup(format!("{what}: {error:?}"))
}

fn registry() -> Checked<ModuleRegistry> {
    let call = ActivityType::new(ModuleId::Programs, PROGRAM_CALL_ORDINAL)
        .map_err(setup("activity type"))?;
    let registration =
        ModuleRegistration::new(ModuleId::Programs, &[call]).map_err(setup("registration"))?;
    ModuleRegistry::new(&[registration]).map_err(setup("registry"))
}

fn chain() -> Checked<ChainDomain> {
    Ok(ChainDomain::new(CHAIN)?)
}

fn program() -> Checked<ProgramId> {
    Ok(ProgramId::new(PROGRAM)?)
}

fn market() -> Checked<MarketId> {
    Ok(derive_market(chain()?, program()?)?)
}

fn principal(bytes: [u8; 32]) -> Checked<PrincipalId> {
    Ok(PrincipalId::new(bytes)?)
}

fn digest(byte: u8) -> Checked<Digest32> {
    Ok(Digest32::new([byte; 32])?)
}

fn owner_signer() -> Checked<LocalSigner> {
    let account = derive_from_seed(&OWNER_SEED, 0).map_err(setup("owner derivation"))?;
    Ok(LocalSigner::from_derived_account(&account))
}

fn policy_with(rubric: u8) -> Checked<TaskPolicyV1> {
    Ok(TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: digest(1)?,
            dataset_artifact: [2; 32],
            benchmark_suite: digest(3)?,
            rubric: RubricDigest::new([rubric; 32])?,
            task_schema: digest(5)?,
            result_schema: digest(6)?,
            service_terms: digest(7)?,
        },
        100,
        1,
    )?)
}

fn encode_shared(shared: &SharedState<'_>) -> Checked<Vec<u8>> {
    let mut out = vec![0; shared.encoded_len()?];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let length = state::encode_shared_state(shared, &mut out, &mut scratch)?;
    out.truncate(length);
    Ok(out)
}

/// Real F01 CREATE at the market origin through `registry_ops::apply`.
fn created_state() -> Checked<Vec<u8>> {
    let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
    policy_with(4)?.encode(&mut encoded_policy)?;
    let rewards = derive_rewards_account(program()?, AssetId::new(ASSET)?)?;
    let mut payload = OWNER.to_vec();
    payload.extend_from_slice(&ASSET);
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&REFUND);
    payload.push(0);
    payload.extend_from_slice(&encoded_policy);
    payload.extend_from_slice(&[16; 32]);
    let envelope = Envelope {
        operation: dispatch::CREATE,
        chain: chain()?,
        program: program()?,
        market: market()?,
        actor: principal(OWNER)?,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 1_000_000,
        request: RequestId::new([1; 32])?,
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let length = encode_envelope(&envelope, &mut encoded)?;
    let validated = decode_envelope(&encoded[..length])?;
    let context = registry_ops::CallContext {
        chain: chain()?,
        program: program()?,
        principal: principal(OWNER)?,
        height: ORIGIN,
    };
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    match registry_ops::apply(&context, None, &validated, &mut section, &mut event)? {
        Outcome::Applied { state, .. } => encode_shared(&state),
        Outcome::AlreadyApplied(_) => Err(Failure::Setup("fresh create retried".to_owned())),
    }
}

fn state_chunks() -> Checked<Vec<(ReadProof, Vec<u8>)>> {
    let state = created_state()?;
    let proof = ReadProof {
        chain: chain()?,
        program: program()?,
        native_state_root: digest(21)?,
        observed_sequence: 91,
        execution_height: 9,
        block_hash: digest(22)?,
    };
    let mut chunks = Vec::new();
    let mut offset = 0_u32;
    let mut revision = 0_u64;
    let mut pinned = [0; 32];
    loop {
        let mut request = revision.to_be_bytes().to_vec();
        request.extend_from_slice(&pinned);
        request.extend_from_slice(&offset.to_be_bytes());
        request.extend_from_slice(&8192_u16.to_be_bytes());
        let mut out = vec![0; 8244];
        let length = read_state_chunk(&state, &request, &mut out)?;
        out.truncate(length);
        let chunk = layerx_programs_ai_market::codec::decode_chunk_response(&out)?;
        revision = chunk.revision;
        pinned = chunk.digest.bytes();
        let total = chunk.total_bytes;
        chunks.push((proof, out));
        offset += 8192;
        if offset >= total {
            break;
        }
    }
    Ok(chunks)
}

fn prepared_operation(registry: &ModuleRegistry) -> Checked<PreparedOperation> {
    let commit = CommitFinality::new(9, [31; 32], [32; 32], [33; 32])?;
    let snapshot = observed()?.finalize(Some(&commit), 5000)?;
    let mut payload = vec![0; 57];
    let length = FundRequest {
        amount: 25,
        refund_recipient: layerx_programs_ai_market::types::AccountId::new(REFUND)?,
        policy_version: FUNDING_POLICY_VERSION,
        consent: true,
    }
    .encode(&mut payload)?;
    payload.truncate(length);
    Ok(snapshot.prepare(
        registry,
        9,
        &OperationRequest {
            operation: dispatch::FUND,
            payload: &payload,
            actor: principal(OWNER)?,
            roster: None,
            sequence: 1,
            expiry: 200,
            request: RequestId::new([34; 32])?,
            required_rank: FINALIZED_RANK,
        },
        &NativeTerms {
            network_id: NETWORK,
            actor_did: b"did:layerx:paxai-owner",
            owner_public_key: owner_signer()?.public_key(),
            account_sequence: 1,
            idempotency_key: [34; 32],
            not_before: 0,
            not_after: 200,
            fee_limit: 1000,
            capabilities: &[0, 0],
            access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
            response_capacity: 16,
            resources: Resources([1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096]),
            gas_limit: 100_000,
            max_fee_per_gas: 30,
            max_priority_fee_per_gas: 2,
        },
    )?)
}

fn signed_operation(registry: &ModuleRegistry) -> Checked<OperationRecord> {
    let signer = owner_signer()?;
    let from = signer
        .evm_address()
        .ok_or_else(|| Failure::Setup("missing EVM key".to_owned()))?;
    let prepared = prepared_operation(registry)?;
    let call = evm_transaction::decode_unsigned(prepared.canonical_bytes())
        .map_err(setup("unsigned transaction"))?;
    let review = prepared.review_call(registry, call, from)?;
    let terms = review.terms().clone();
    let approval = review.approve(&terms, signer.public_key())?;
    Ok(ready(approval.sign(&signer, registry, 9))?)
}

fn ready<F: Future>(future: F) -> F::Output {
    struct Idle;
    impl Wake for Idle {
        fn wake(self: Arc<Self>) {}
    }
    let waker = Waker::from(Arc::new(Idle));
    let mut context = Context::from_waker(&waker);
    let mut future = pin!(future);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(output) => output,
        Poll::Pending => panic!("signing future did not complete"),
    }
}

struct JournalDirectory(PathBuf);

impl Drop for JournalDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn observed() -> Checked<ObservedSnapshot> {
    Ok(ObservedSnapshot::capture(&state_chunks()?)?)
}

#[test]
fn real_market_state_reaches_finalized_consumer_twice() -> Checked {
    use layerx_client::ai_market::{decode_snapshot_binding, FinalizedSnapshot, Freshness};
    use layerx_programs_ai_market::queries::BINDING_MAX_BYTES;
    let commit = CommitFinality::new(9, [22; 32], [32; 32], [33; 32])?;
    let mut previous = None;
    for _ in 0..2 {
        let chunks = state_chunks()?;
        assert!(!chunks.is_empty(), "the producer must emit state");
        let captured = ObservedSnapshot::capture(&chunks)?;
        assert_eq!(captured.facts().proof.execution_height, 9);
        let snapshot: FinalizedSnapshot = captured.finalize(Some(&commit), 5000)?;
        let consumer = snapshot.inspect(Some(9))?;
        assert_eq!(consumer.freshness, Freshness::Current);
        assert_eq!(consumer.binding.rank, FINALIZED_RANK);
        assert_eq!(consumer.binding.block_height, 9);
        assert_eq!(consumer.binding.block_hash, commit.block_hash());
        assert_eq!(consumer.binding.app_hash, commit.app_hash());
        assert_eq!(
            consumer.binding.store_proof_digest,
            commit.store_proof_digest()
        );
        assert_eq!(snapshot.state_bytes(), created_state()?);
        let mut wire = vec![0; BINDING_MAX_BYTES];
        let length = snapshot.binding().encode(&mut wire)?;
        wire.truncate(length);
        assert_eq!(decode_snapshot_binding(&wire)?, consumer.binding);
        assert_eq!(
            decode_snapshot_binding(&wire)?.snapshot_id()?,
            consumer.snapshot_id
        );
        if let Some(expected) = previous {
            assert_eq!(consumer, expected);
        }
        previous = Some(consumer);
        let mut malformed = *snapshot.binding();
        malformed.block_height = 8;
        let length = malformed.encode(&mut wire)?;
        assert_eq!(
            decode_snapshot_binding(&wire[..length]),
            Err(AiClientError::Query(QueryError::BindingMismatch))
        );
        let mut damaged = chunks;
        let body = &mut damaged
            .last_mut()
            .ok_or_else(|| Failure::Setup("no chunks".into()))?
            .1;
        let byte = body
            .last_mut()
            .ok_or_else(|| Failure::Setup("empty chunk".into()))?;
        *byte ^= 1;
        assert!(ObservedSnapshot::capture(&damaged).is_err());
    }
    Ok(())
}

#[test]
#[allow(non_snake_case)]
fn CheckpointFinality() -> Checked {
    assert_eq!(
        observed()?.finalize(None, 5000),
        Err(AiClientError::Query(QueryError::FinalityUnavailable))
    );
    let older = CommitFinality::new(8, [22; 32], [32; 32], [33; 32])?;
    assert_eq!(
        observed()?.finalize(Some(&older), 5000),
        Err(AiClientError::Query(QueryError::BindingMismatch))
    );
    assert_eq!(
        CommitFinality::new(9, [22; 32], [0; 32], [33; 32]),
        Err(AiClientError::Query(QueryError::FinalityUnavailable))
    );
    Ok(())
}

#[test]
fn signed_producer_survives_journal_reopen_and_refuses_corruption() -> Checked {
    let registry = registry()?;
    let prepared = prepared_operation(&registry)?;
    let signer = owner_signer()?;
    let from = signer
        .evm_address()
        .ok_or_else(|| Failure::Setup("missing sender".into()))?;
    let review = prepared.review(&registry, from)?;
    assert_eq!(
        review.clone().approve(review.terms(), [0; 32]),
        Err(AiClientError::UnauthorizedKey)
    );
    let signed = signed_operation(&registry)?;
    let id = signed.activity_id();
    let bytes = signed.signed_bytes().to_vec();
    let directory = JournalDirectory(std::env::temp_dir().join(format!(
        "proof-client-{}-{}",
        std::process::id(),
        NEXT_JOURNAL.fetch_add(1, Ordering::Relaxed)
    )));
    let journal = OperationJournal::open(&directory.0)?;
    journal.record_signed(&signed)?;
    let path = journal.record_path(&id);
    let encoded = fs::read(&path).map_err(setup("journal bytes"))?;
    assert_eq!(OperationRecord::decode(&encoded, &registry)?, signed);
    drop(journal);
    drop(signed);
    for _ in 0..2 {
        let reopened = OperationJournal::open(&directory.0)?;
        let consumer = reopened.load(&id, &registry)?;
        assert_eq!(consumer.state(), OperationState::Signed);
        assert_eq!(consumer.activity_id(), id);
        assert_eq!(consumer.signed_bytes(), bytes);
        assert_eq!(consumer.network_id(), NETWORK);
        assert_eq!(consumer.attempt(), 0);
        assert_eq!(consumer.encode()?, encoded);
    }
    let reopened = OperationJournal::open(&directory.0)?;
    let wrong_id = [0xfe; 32];
    fs::write(reopened.record_path(&wrong_id), &encoded).map_err(setup("copy producer record"))?;
    assert_eq!(
        reopened.load(&wrong_id, &registry),
        Err(AiClientError::CorruptRecord)
    );
    fs::write(&path, &encoded[..encoded.len() - 1]).map_err(setup("truncate journal"))?;
    assert!(
        reopened.load(&id, &registry).is_err(),
        "load must decode disk again, without cached state"
    );
    Ok(())
}

use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::atomic::AtomicBool;
use std::sync::Mutex;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::px::PxEndpoint;
use layerx_client::rpc::history as consumer_history;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use serde_json::{json, Value};

#[allow(dead_code)]
#[path = "../../../../platform/hosted/gateway/src/evm.rs"]
mod gateway_evm;

fn gateway_recorded_coins() -> Value {
    let vectors: Value = must(serde_json::from_str(include_str!(
        "../../../../platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
    )));
    let vector = vectors["cases"]
        .as_array()
        .expect("ABI cases")
        .iter()
        .find(|v| v["name"] == "bank.all_balances")
        .expect("bank producer");
    let raw = vector["result"]
        .as_str()
        .expect("ABI result")
        .trim_start_matches("0x");
    let bytes: Vec<_> = raw
        .as_bytes()
        .chunks_exact(2)
        .map(|p| must(u8::from_str_radix(must(std::str::from_utf8(p)), 16)))
        .collect();
    let coins = must(gateway_evm::decode_coins(&bytes));
    let result = json!(coins
        .iter()
        .map(|(denom, amount)| json!({"denom":denom,"amount":gateway_evm::uint256_decimal(amount)}))
        .collect::<Vec<_>>());
    assert_eq!(result, vector["out"]["response"]);
    result
}

fn gateway_recorded_freshness() -> Value {
    let transcript: Value = must(serde_json::from_str(include_str!(
        "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
    )));
    transcript["result"]["freshness"].clone()
}

#[derive(Clone, Debug)]
struct Exchange {
    method: String,
    params: Value,
    result: Value,
}

/// Recorded bodies are intentionally independent of the client decoders.
/// Missing methods or mismatched requests fail instead of synthesizing a result.
struct RecordedProducer {
    records: Vec<Value>,
    network: Value,
    history: Value,
    chain_id: Value,
}

impl RecordedProducer {
    fn from_repository_records() -> Self {
        let records: Vec<Value> = must(serde_json::from_str(include_str!(
            "../../../../../sdk/wallet/test/fixtures/endpoint/available.json"
        )));
        let chain: Value = must(serde_json::from_str(include_str!(
            "../../../../../sdk/wallet/test/fixtures/gateway/rpc-chain-id.json"
        )));
        Self {
            records,
            chain_id: chain["body"]["result"].clone(),
            // Exact consensus response from px_reads.rs::network_reads_the_consensus_head.
            // The older available.json network record carries a retired anchor;
            // retain it separately as a refusal rather than rewriting that record.
            network: json!({
                "network_id": "paxeer:mainnet",
                "paxeer": {"chain_id": "0x7d", "latest_block": "0x10"},
                "consensus": {
                    "latest_height": 9,
                    "latest_block_hash": "12".repeat(32),
                    "latest_app_hash": "34".repeat(32),
                    "catching_up": false
                }
            }),
            // Exact page from sequencer_reads.rs::history_sends_px_get_history_with_a_32_hex_cursor_and_refuses_other_forms.
            history: json!({
                "items": [
                    {"kind": "transfer", "amount": "123", "nested": [1, null]},
                    {"kind": "call"}
                ],
                "next_cursor": "000000000000abce0000000100000002",
                "indexedHeight": 43982, "chainHeight": 44000,
                "account": "0x1111111111111111111111111111111111111111",
                "freshness": gateway_recorded_freshness()
            }),
        }
    }

    fn result(&self, method: &str, params: &Value) -> Value {
        let account = "0x1111111111111111111111111111111111111111";
        match method {
            "eth_chainId" => {
                assert_eq!(*params, json!([]));
                self.chain_id.clone()
            }
            "px_getNetwork" => {
                assert_eq!(*params, json!([]));
                self.network.clone()
            }
            "px_getHistory" => {
                assert_eq!(*params, json!([account, null, 2]));
                self.history.clone()
            }
            "px_getBalances" => {
                assert_eq!(*params, json!([account]));
                let recorded = self
                    .records
                    .iter()
                    .find(|r| r["request"]["method"] == "px_getAccount")
                    .expect("identity transcript");
                json!({"account":recorded["response"]["result"]["account"], "height":150,
                    "balances": gateway_recorded_coins()})
            }
            "px_listAssets" => {
                assert_eq!(*params, json!([]));
                json!({"height":150,"joined_limit":16,"assets":[{
                    "asset_id":"bank:ulxp","kind":"bank","denom":"ulxp", "name":"LayerX",
                    "symbol":"LXP", "decimals":18,"supply":"250000000"}]})
            }
            _ => {
                let record = self
                    .records
                    .iter()
                    .find(|record| record["request"]["method"] == method)
                    .unwrap_or_else(|| panic!("missing recorded producer: {method}"));
                if method == "eth_getBalance" {
                    // Sequencer reads the same quantity from the pending state.
                    assert_eq!(*params, json!([account, "pending"]));
                } else {
                    assert_eq!(*params, record["request"]["params"]);
                }
                let mut result = record["response"]
                    .get("result")
                    .unwrap_or_else(|| panic!("recorded producer refused: {method}"))
                    .clone();
                if method == "px_getAccount" {
                    result["paxeer"]["height"] = json!(150);
                }
                result
            }
        }
    }
}

/// In-process producer serving repository response records over real HTTP.
/// Faults modify a result only after recording the undamaged producer body.
struct Crossing {
    url: String,
    exchanges: Arc<Mutex<Vec<Exchange>>>,
    corrupt: Arc<Mutex<Option<(String, Value)>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

fn must<T, E: fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

impl Crossing {
    fn start() -> Self {
        let producer = RecordedProducer::from_repository_records();
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let exchanges = Arc::new(Mutex::new(Vec::new()));
        let corrupt = Arc::new(Mutex::new(None::<(String, Value)>));
        let stop = Arc::new(AtomicBool::new(false));
        let (seen, fault, stopped) = (exchanges.clone(), corrupt.clone(), stop.clone());
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(5));
                        continue;
                    }
                    Err(error) => panic!("producer listener: {error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(10))));
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(size) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(size.trim().parse::<usize>()));
                    }
                }
                let mut body = vec![0; length.unwrap_or_else(|| panic!("request length"))];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                let method = request["method"]
                    .as_str()
                    .unwrap_or_else(|| panic!("RPC method"));
                assert_eq!(request["jsonrpc"], "2.0");
                let result = producer.result(method, &request["params"]);
                let mut document = json!({
                    "jsonrpc": "2.0", "id": request["id"], "result": result
                });
                must(seen.lock()).push(Exchange {
                    method: method.to_owned(),
                    params: request["params"].clone(),
                    result: document["result"].clone(),
                });
                let mut fault = must(fault.lock());
                if fault.as_ref().is_some_and(|(name, _)| name == method) {
                    // Damage a recorded producer result at the HTTP response boundary.
                    assert!(
                        document.get("error").is_none(),
                        "producer refused: {document}"
                    );
                    let (_, replacement) = fault.take().expect("armed response fault");
                    document["result"] = replacement;
                }
                drop(fault);
                let answer = must(serde_json::to_vec(&document));
                let mut stream = reader.into_inner();
                must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", answer.len()));
                must(stream.write_all(&answer));
                must(stream.flush());
            }
        });
        Self {
            url,
            exchanges,
            corrupt,
            stop,
            worker: Some(worker),
        }
    }

    fn latest(&self, method: &str) -> Exchange {
        must(self.exchanges.lock())
            .iter()
            .rev()
            .find(|exchange| exchange.method == method)
            .cloned()
            .unwrap_or_else(|| panic!("no producer exchange for {method}"))
    }

    fn corrupt_next(&self, method: &str) {
        self.replace_next(method, Value::Null);
    }

    fn replace_next(&self, method: &str, replacement: Value) {
        let mut fault = must(self.corrupt.lock());
        assert!(fault.is_none());
        *fault = Some((method.to_owned(), replacement));
    }
}

impl Drop for Crossing {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let joined = worker.join();
            if !std::thread::panicking() {
                must(joined);
            }
        }
    }
}

fn number(value: &Value) -> u64 {
    if let Some(number) = value.as_u64() {
        return number;
    }
    let text = value.as_str().unwrap_or_else(|| panic!("number: {value}"));
    if let Some(hex) = text.strip_prefix("0x") {
        must(u64::from_str_radix(hex, 16))
    } else {
        must(text.parse())
    }
}

fn word(value: &Value) -> [u8; 32] {
    let hex = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("quantity: {value}"));
    assert!(hex.len() <= 64);
    let padded = format!("{hex:0>64}");
    let mut result = [0; 32];
    for (byte, pair) in result.iter_mut().zip(padded.as_bytes().chunks_exact(2)) {
        *byte = must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16));
    }
    result
}

#[test]
#[allow(non_snake_case)]
fn px_getNetwork() {
    let crossing = Crossing::start();
    let sequencer = must(SequencerEndpoint::new(&crossing.url, &[]));
    let px = must(PxEndpoint::new(&crossing.url, &[]));
    let address = [0x11; 20];
    let account = format!("0x{}", "11".repeat(20));
    // Each iteration must reach the producer again, preserving the gateway history head.
    for iteration in 1..=2 {
        let chain = must(sequencer.chain_id());
        assert_eq!(chain, number(&crossing.latest("eth_chainId").result));
        let balance = must(sequencer.balance(&address));
        let exchange = crossing.latest("eth_getBalance");
        assert_eq!(exchange.params, json!([account, "pending"]));
        assert_eq!(balance, word(&exchange.result));
        let joined = must(px.account(&account));
        let exchange = crossing.latest("px_getAccount");
        assert_eq!(exchange.params, json!([account]));
        assert_eq!(joined.account.bound, exchange.result["account"]["bound"]);
        assert_eq!(
            joined.paxeer.is_some(),
            !exchange.result["paxeer"].is_null()
        );
        assert_eq!(joined.account.evm_address, Some(address));
        assert_eq!(joined.account.layerx_account, Some([0x7c; 32]));
        assert_eq!(
            joined.account.layerx_did.as_deref(),
            exchange.result["account"]["layerx_did"].as_str()
        );
        assert_eq!(
            joined.account.pax_address.as_deref(),
            exchange.result["account"]["pax_address"].as_str()
        );
        let paxeer = joined.paxeer.as_ref().expect("recorded Paxeer account");
        assert_eq!(paxeer.address, address);
        assert_eq!(paxeer.balance.decimal(), "1000000000000000000");
        assert_eq!(paxeer.height, exchange.result["paxeer"]["height"]);
        assert_eq!(paxeer.nonce, 3);
        assert_eq!(
            joined.layerx.as_ref(),
            exchange.result["layerx"].as_object()
        );
        let balances = must(px.balances(&account));
        let exchange = crossing.latest("px_getBalances");
        assert_eq!(exchange.params, json!([account]));
        assert_eq!(
            balances.balances.len(),
            exchange.result["balances"]
                .as_array()
                .unwrap_or_else(|| panic!("balance rows"))
                .len()
        );
        assert_eq!(balances.height, number(&exchange.result["height"]));
        assert_eq!(balances.account, joined.account);
        let row = balances.balances.first().expect("recorded balance row");
        assert_eq!(row.denom, "ulxp");
        assert_eq!(row.amount.decimal(), "123456789");
        for (row, produced) in balances
            .balances
            .iter()
            .zip(exchange.result["balances"].as_array().expect("coins"))
        {
            assert_eq!(row.denom, produced["denom"]);
            assert_eq!(row.amount.decimal(), produced["amount"]);
        }
        let assets = must(px.list_assets());
        let exchange = crossing.latest("px_listAssets");
        assert_eq!(exchange.params, json!([]));
        assert_eq!(
            assets.assets.len(),
            exchange.result["assets"]
                .as_array()
                .unwrap_or_else(|| panic!("asset rows"))
                .len()
        );
        assert_eq!(
            assets.joined_limit,
            number(&exchange.result["joined_limit"])
        );
        let asset = assets.assets.first().expect("recorded asset");
        assert_eq!(assets.height, number(&exchange.result["height"]));
        assert_eq!(asset.asset_id, "bank:ulxp");
        assert_eq!(asset.kind, "bank");
        assert_eq!(asset.denom.as_deref(), Some(row.denom.as_str()));
        assert_eq!(
            asset.name.as_deref(),
            exchange.result["assets"][0]["name"].as_str()
        );
        assert_eq!(
            asset.symbol.as_deref(),
            exchange.result["assets"][0]["symbol"].as_str()
        );
        assert_eq!(
            asset.decimals.map(u64::from),
            exchange.result["assets"][0]["decimals"].as_u64()
        );
        assert_eq!(
            asset.supply.expect("recorded supply").decimal(),
            exchange.result["assets"][0]["supply"]
        );
        assert_eq!(asset.paused, None);
        assert_eq!(asset.verification, None);
        let head = must(px.network());
        let exchange = crossing.latest("px_getNetwork");
        assert_eq!(exchange.params, json!([]));
        assert_eq!(head.paxeer.chain_id, chain);
        assert_eq!(head.network_id, exchange.result["network_id"]);
        assert_eq!(
            head.paxeer.latest_block,
            number(&exchange.result["paxeer"]["latest_block"])
        );
        assert_eq!(
            head.latest_committed_height,
            number(&exchange.result["consensus"]["latest_height"])
        );
        let history = must(consumer_history::history(
            &sequencer,
            &account,
            None,
            Some(2),
        ));
        let exchange = crossing.latest("px_getHistory");
        assert_eq!(exchange.params, json!([account, null, 2]));
        assert_eq!(
            history.items,
            *exchange.result["items"]
                .as_array()
                .unwrap_or_else(|| panic!("history rows"))
        );
        assert_eq!(
            history.next_cursor.as_deref(),
            exchange.result["next_cursor"].as_str()
        );
        assert_eq!(
            history.indexed_height,
            exchange.result["indexedHeight"].as_u64()
        );
        assert_eq!(
            history.chain_height,
            exchange.result["chainHeight"].as_u64()
        );
        assert_eq!(history.account, account);
        assert_eq!(history.freshness, exchange.result["freshness"]);
        let exchanges = must(crossing.exchanges.lock());
        assert!(exchanges
            .iter()
            .all(|exchange| exchange.method != "eth_blockNumber"));
        for method in [
            "eth_chainId",
            "eth_getBalance",
            "px_getAccount",
            "px_getBalances",
            "px_listAssets",
            "px_getNetwork",
            "px_getHistory",
        ] {
            assert_eq!(
                exchanges
                    .iter()
                    .filter(|exchange| exchange.method == method)
                    .count(),
                iteration
            );
        }
    }
    crossing.corrupt_next("eth_chainId");
    assert_eq!(sequencer.chain_id(), Err(SequencerError::Malformed));
    crossing.corrupt_next("eth_getBalance");
    assert_eq!(sequencer.balance(&address), Err(SequencerError::Malformed));
    crossing.corrupt_next("px_getAccount");
    assert_eq!(px.account(&account), Err(SequencerError::Malformed));
    crossing.corrupt_next("px_getBalances");
    assert_eq!(px.balances(&account), Err(SequencerError::Malformed));
    crossing.corrupt_next("px_listAssets");
    assert_eq!(px.list_assets(), Err(SequencerError::Malformed));
    crossing.corrupt_next("px_getNetwork");
    assert_eq!(px.network(), Err(SequencerError::Malformed));
    crossing.corrupt_next("px_getHistory");
    assert_eq!(
        consumer_history::history(&sequencer, &account, None, Some(2)),
        Err(SequencerError::Malformed)
    );
    let records = RecordedProducer::from_repository_records();
    let retired_network = records
        .records
        .iter()
        .find(|record| record["request"]["method"] == "px_getNetwork")
        .expect("recorded retired network")["response"]["result"]
        .clone();
    assert!(retired_network.get("anchor").is_some());
    crossing.replace_next("px_getNetwork", retired_network);
    assert_eq!(px.network(), Err(SequencerError::Malformed));
    let transmitted = must(crossing.exchanges.lock()).len();
    assert_eq!(
        consumer_history::history(&sequencer, &account, Some("invalid"), Some(2)),
        Err(SequencerError::Endpoint)
    );
    assert_eq!(must(crossing.exchanges.lock()).len(), transmitted);
    assert!(must(crossing.corrupt.lock()).is_none());
}
